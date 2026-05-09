package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"gitlab.com/dunn.dev/tutti/internal/audio"
	"gitlab.com/dunn.dev/tutti/internal/cm"
	"gitlab.com/dunn.dev/tutti/internal/decisions"
	"gitlab.com/dunn.dev/tutti/internal/descriptor"
	"gitlab.com/dunn.dev/tutti/internal/drive"
	"gitlab.com/dunn.dev/tutti/internal/mdns"
	"gitlab.com/dunn.dev/tutti/internal/redact"
	"gitlab.com/dunn.dev/tutti/internal/schema"
	"gitlab.com/dunn.dev/tutti/internal/ssdp"
	"gitlab.com/dunn.dev/tutti/internal/version"
)

type captureFlags struct {
	driveOn      bool
	driveForce   bool
	allowEmpty   bool
	noRedact     bool
	interfaces   strSlice
	outDir       string
	contributor  string
	mxSeconds    int
	mdnsTimeout  time.Duration
	pollDuration time.Duration
}

func runCapture(ctx context.Context, args []string) int {
	cf := &captureFlags{
		mxSeconds:    ssdp.DefaultMX,
		mdnsTimeout:  4 * time.Second,
		pollDuration: 12 * time.Second,
	}
	fs := parseFlagsOrDie("capture", args, func(fs *flag.FlagSet) {
		fs.BoolVar(&cf.driveOn, "drive", false, "run AVTransport drive against discovered renderers")
		fs.BoolVar(&cf.driveForce, "force", false, "ignore the 'device is PLAYING' precheck (with --drive)")
		fs.BoolVar(&cf.allowEmpty, "allow-empty", false, "accept a capture with zero SSDP and zero mDNS responses")
		fs.BoolVar(&cf.noRedact, "no-redact", false, "do not scrub LAN IPs / auth tokens")
		fs.Var(&cf.interfaces, "interface", "scan only the named network interface (repeatable)")
		fs.StringVar(&cf.outDir, "out", "", "output directory (default ./capture-<ts>-<host>)")
		fs.StringVar(&cf.contributor, "contributor", "", "contributor id, e.g. github:andrewdunndev or gitlab:andrew.dunn")
		fs.IntVar(&cf.mxSeconds, "ssdp-mx", ssdp.DefaultMX, "SSDP M-SEARCH MX header (1..5)")
	})
	_ = fs

	if cf.contributor != "" && !contributorPattern.MatchString(cf.contributor) {
		fmt.Fprintf(os.Stderr, "Error: --contributor must be of the form github:<handle> or gitlab:<handle>; got %q.\n", cf.contributor)
		return 2
	}

	startedAt := time.Now().UTC()
	captureID := newULID(startedAt)
	host, _ := os.Hostname()

	outDir := cf.outDir
	if outDir == "" {
		outDir = fmt.Sprintf("capture-%s-%s", startedAt.Format("20060102T150405Z"), sanitiseHost(host))
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not create output directory %s: %v.\n", outDir, err)
		return 1
	}

	red := redact.New()
	clientIP, _ := audio.PickOutboundIP(net.ParseIP("8.8.8.8"))
	if clientIP != nil {
		red.LearnClientIP(clientIP.String())
	}

	manifest := &schema.Manifest{
		SchemaVersion: schema.SchemaVersion,
		TuttiVersion:  version.Tutti,
		CaptureID:     captureID,
		CapturedAt:    startedAt.Format(time.RFC3339),
		Host: schema.Host{
			OS:         runtime.GOOS,
			Arch:       runtime.GOARCH,
			Interfaces: pickInterfaces(cf.interfaces),
		},
		ScanInfo: schema.ScanInfo{
			SsdpStList:       ssdp.DefaultSTList,
			SsdpMx:           cf.mxSeconds,
			MdnsServiceTypes: mdns.DefaultServiceTypes,
			DriveRequested:   cf.driveOn,
			DriveForce:       cf.driveForce,
			NoRedact:         cf.noRedact,
			AllowEmpty:       cf.allowEmpty,
		},
	}
	if cf.contributor != "" {
		c := cf.contributor
		manifest.Contributor = &c
	}

	// SSDP probe
	fmt.Fprintf(os.Stderr, "[tutti] SSDP M-SEARCH (MX=%ds) on %s ...\n", cf.mxSeconds, strings.Join(manifest.Host.Interfaces, ","))
	ssdpResult, ssdpErr := ssdp.Probe(ctx, cf.interfaces, ssdp.DefaultSTList, cf.mxSeconds)
	if ssdpErr != nil {
		fmt.Fprintf(os.Stderr, "[tutti] SSDP probe error: %v (continuing)\n", ssdpErr)
	}
	manifest.RunStats.SsdpResponses = 0
	if ssdpResult != nil {
		manifest.RunStats.SsdpResponses = len(ssdpResult.Responses)
		manifest.RunStats.SsdpUniqueUsns = len(ssdp.GroupByUSN(ssdpResult.Responses))
	}

	// mDNS browse
	fmt.Fprintf(os.Stderr, "[tutti] mDNS browse (%d service types, %s timeout) ...\n",
		len(mdns.DefaultServiceTypes), cf.mdnsTimeout)
	mdnsResult, mdnsErr := mdns.Browse(ctx, cf.interfaces, mdns.DefaultServiceTypes, cf.mdnsTimeout)
	if mdnsErr != nil {
		fmt.Fprintf(os.Stderr, "[tutti] mDNS browse error: %v (continuing)\n", mdnsErr)
	}
	if mdnsResult != nil {
		manifest.RunStats.MdnsRecords = len(mdnsResult.Services)
	}

	// Devices: from SSDP LOCATION URLs, descriptor + decisions + protocol_info + (optional) drive
	devices := buildDevices(ctx, ssdpResult, mdnsResult, outDir, cf, red)
	manifest.Devices = devices

	// Redactions list
	if !cf.noRedact {
		manifest.Redactions = red.Applied()
	} else {
		manifest.Redactions = []schema.Redaction{}
	}

	// Runstats
	manifest.RunStats.ElapsedSeconds = time.Since(startedAt).Seconds()
	manifest.RunStats.Exit = schema.ExitSuccess
	if ssdpErr != nil || mdnsErr != nil {
		manifest.RunStats.Exit = schema.ExitPartial
	}

	// Write artifacts
	if ssdpResult != nil {
		writeJSON(filepath.Join(outDir, "ssdp.json"), ssdpResultJSON(ssdpResult, red, cf.noRedact))
		dump := ssdpResult.RawDump
		if !cf.noRedact {
			dump = red.String(dump)
		}
		_ = os.WriteFile(filepath.Join(outDir, "ssdp-raw.txt"), []byte(dump), 0o644)
	}
	if mdnsResult != nil {
		writeJSON(filepath.Join(outDir, "mdns.json"), mdnsResultJSON(mdnsResult, red, cf.noRedact))
	}

	// Manifest
	manifestPath := filepath.Join(outDir, "manifest.json")
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	_ = os.WriteFile(manifestPath, mb, 0o644)

	// Self-validate
	v, err := schema.NewValidator()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: validator failed to compile: %v.\n", err)
		return 1
	}
	if err := v.ValidateBytes(mb); err != nil {
		fmt.Fprintf(os.Stderr, "Error: tutti wrote a manifest that fails its own schema:\n%v\n", err)
		fmt.Fprintln(os.Stderr, "  This is a tutti bug; please file at https://gitlab.com/dunn.dev/tutti/-/issues with the manifest attached.")
		return 1
	}
	if err := v.CheckVacuity(manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Error: capture is technically valid but vacuous: %v.\n", err)
		fmt.Fprintln(os.Stderr, "  Suggested next step: see the message above. Re-run with --allow-empty if intentional.")
		return 1
	}

	fmt.Printf("OK: capture written to %s\n", outDir)
	fmt.Printf("    capture_id=%s  devices=%d  ssdp_responses=%d  mdns_records=%d  elapsed=%.1fs\n",
		manifest.CaptureID, len(manifest.Devices),
		manifest.RunStats.SsdpResponses, manifest.RunStats.MdnsRecords,
		manifest.RunStats.ElapsedSeconds)
	return 0
}

var contributorPattern = regexp.MustCompile(`^(github|gitlab):[A-Za-z0-9._-]+$`)

func newULID(t time.Time) string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}

func sanitiseHost(h string) string {
	if h == "" {
		return "host"
	}
	out := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, h)
	if len(out) > 32 {
		out = out[:32]
	}
	return strings.ToLower(out)
}

func pickInterfaces(requested strSlice) []string {
	if len(requested) > 0 {
		return []string(requested)
	}
	ifs, err := net.Interfaces()
	if err != nil {
		return []string{"unknown"}
	}
	var out []string
	for _, iface := range ifs {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				if ip4 := ipnet.IP.To4(); ip4 != nil && !ip4.IsLinkLocalUnicast() {
					out = append(out, iface.Name)
					break
				}
			}
		}
	}
	if len(out) == 0 {
		out = []string{"none"}
	}
	return out
}

func buildDevices(ctx context.Context, ssdpResult *ssdp.Result, mdnsResult *mdns.Result, outDir string, cf *captureFlags, red *redact.Redactor) []schema.Device {
	devicesByLoc := map[string]*schema.Device{}

	// Group SSDP by LOCATION URL; each unique LOCATION is one device.
	if ssdpResult != nil {
		usnsByLoc := map[string]map[string]bool{}
		for _, r := range ssdpResult.Responses {
			if r.Location == "" {
				continue
			}
			if _, ok := usnsByLoc[r.Location]; !ok {
				usnsByLoc[r.Location] = map[string]bool{}
			}
			usnsByLoc[r.Location][r.USN] = true
		}

		for loc, usns := range usnsByLoc {
			d := &schema.Device{}
			for u := range usns {
				d.Discovery.SsdpUSNs = append(d.Discovery.SsdpUSNs, u)
			}
			sort.Strings(d.Discovery.SsdpUSNs)

			// Fetch and parse descriptor.
			parsed, raw, err := descriptor.Fetch(ctx, loc)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[tutti] descriptor fetch failed for %s: %v\n", redactURL(loc, red, cf.noRedact), err)
				d.Slug = slugFromUSNs(d.Discovery.SsdpUSNs)
				devicesByLoc[loc] = d
				continue
			}

			// Learn the device's IP for redaction.
			if u, perr := url.Parse(loc); perr == nil {
				if h, _, _ := net.SplitHostPort(u.Host); h != "" {
					red.LearnDeviceIP(h)
				}
			}

			d.Vendor = parsed.Manufacturer
			d.Model = derivedModel(parsed)
			d.UDN = parsed.UDN
			d.Slug = deriveSlug(parsed, d.Discovery.SsdpUSNs)
			d.Tags = []string{}

			// Per-device subdir.
			devDir := filepath.Join(outDir, "devices", d.Slug)
			_ = os.MkdirAll(devDir, 0o755)

			// Library decisions (use raw bytes + pristine parsed; the
			// decisions package does not mutate the descriptor).
			decs := decisions.Compare(ctx, loc, parsed, raw)
			d.Decisions = decs
			writeJSON(filepath.Join(devDir, "decisions.json"), decs)

			// ConnectionManager:GetProtocolInfo if present (uses pristine
			// parsed.Services control URLs).
			cmCtl := serviceControlURL(parsed, "urn:schemas-upnp-org:service:ConnectionManager:1")
			if cmCtl != "" {
				pi, rawTxt, err := cm.GetProtocolInfo(ctx, cmCtl)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[tutti] GetProtocolInfo failed for %s: %v\n", d.Slug, err)
				} else {
					rawOutTxt := rawTxt
					if !cf.noRedact {
						rawOutTxt = red.Bytes(rawTxt)
					}
					_ = os.WriteFile(filepath.Join(devDir, "protocol-info.txt"), rawOutTxt, 0o644)
					pi.RawFile = filepath.Join("devices", d.Slug, "protocol-info.txt")
					d.ProtocolInfo = &pi
					writeJSON(filepath.Join(devDir, "protocol-info.json"), pi)
				}
			}

			// Drive (if requested) -- uses pristine parsed.Services control URLs.
			if cf.driveOn {
				avt := serviceControlURL(parsed, "urn:schemas-upnp-org:service:AVTransport:1")
				if avt == "" {
					skip := schema.DriveTest{Performed: false, SkippedReason: "no AVTransport service"}
					d.DriveTest = &skip
				} else {
					d.DriveTest = runDrive(ctx, d, parsed, avt, devDir, cf, red)
				}
			}

			// Now redact and persist descriptor. Done last so all live
			// network calls above used the pristine URLs.
			rawOut := raw
			if !cf.noRedact {
				rawOut = red.Bytes(raw)
			}
			_ = os.WriteFile(filepath.Join(devDir, "descriptor.xml"), rawOut, 0o644)
			descParsed := redactDescriptor(parsed, red, cf.noRedact)
			d.Descriptor = &schema.Descriptor{
				URLRedacted: redactURL(loc, red, cf.noRedact),
				RawFile:     filepath.Join("devices", d.Slug, "descriptor.xml"),
				Parsed:      descParsed,
			}
			writeJSON(filepath.Join(devDir, "descriptor.json"), descParsed)

			// Tags
			d.Tags = append(d.Tags, "ssdp")
			if d.DriveTest != nil && d.DriveTest.Performed {
				d.Tags = append(d.Tags, "drive-attempted")
				gotPlaying := false
				for _, run := range d.DriveTest.Runs {
					if run.Result == schema.DriveResultPlaying {
						gotPlaying = true
						break
					}
				}
				if gotPlaying {
					d.Tags = append(d.Tags, "drive-ok")
				} else {
					d.Tags = append(d.Tags, "drive-fail")
				}
			}
			devicesByLoc[loc] = d
		}
	}

	// Attach mDNS services to existing devices when host overlaps; otherwise
	// create discovery-only entries.
	if mdnsResult != nil {
		for _, svc := range mdnsResult.Services {
			matched := false
			for _, d := range devicesByLoc {
				if d.Descriptor == nil {
					continue
				}
				if hostMatches(d.Descriptor.URLRedacted, svc) {
					d.Discovery.MdnsServices = append(d.Discovery.MdnsServices, svc)
					matched = true
					break
				}
			}
			if matched {
				continue
			}
			// Discovery-only device: announces on mDNS but no UPnP descriptor.
			slug := slugFromMDNS(svc)
			key := "mdns:" + svc.Instance
			if devicesByLoc[key] == nil {
				devicesByLoc[key] = &schema.Device{Slug: slug}
			}
			devicesByLoc[key].Discovery.MdnsServices = append(devicesByLoc[key].Discovery.MdnsServices, svc)
		}
	}

	out := make([]schema.Device, 0, len(devicesByLoc))
	for _, d := range devicesByLoc {
		// Schema requires at least one of decisions/descriptor/protocol_info/drive_test
		// per device. mDNS-only devices lack these; for v0 we drop them rather
		// than fabricate a decision. Future work: add an mdns-only decision shape.
		if d.Descriptor == nil && d.Decisions == nil && d.ProtocolInfo == nil && d.DriveTest == nil {
			continue
		}
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

func runDrive(ctx context.Context, d *schema.Device, parsed schema.DescriptorParsed, avtURL, devDir string, cf *captureFlags, red *redact.Redactor) *schema.DriveTest {
	// Pick outbound IP toward the device so the URL we build is reachable.
	devHost := ""
	if u, err := url.Parse(avtURL); err == nil {
		devHost, _, _ = net.SplitHostPort(u.Host)
	}
	devIP := net.ParseIP(devHost)
	bindIP, _ := audio.PickOutboundIP(devIP)
	srv := audio.NewServer(bindIP)
	if _, err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[tutti] audio server failed to start: %v\n", err)
		skip := schema.DriveTest{Performed: false, SkippedReason: "audio server start failed: " + err.Error()}
		return &skip
	}
	defer func() { _ = srv.Stop() }()

	// Intersect tones with the device's announced sink list when known.
	tones := audio.Matrix
	if d.ProtocolInfo != nil && d.ProtocolInfo.AudioSinkCount > 0 {
		mimes := mimesFromProtocolInfo(filepath.Join(devDir, "protocol-info.txt"))
		if len(mimes) > 0 {
			tones = audio.FilterByMIMEs(mimes)
		}
	}
	if len(tones) == 0 {
		skip := schema.DriveTest{Performed: false, SkippedReason: "no tones intersect with device sink list"}
		return &skip
	}

	transcriptDir := filepath.Join(devDir, "drive")
	_ = os.MkdirAll(transcriptDir, 0o755)

	dt, err := drive.Run(ctx, drive.Options{
		AVTransportControlURL: avtURL,
		Tones:                 tones,
		AudioServer:           srv,
		PollInterval:          3 * time.Second,
		PollDuration:          cf.pollDuration,
		Force:                 cf.driveForce,
		TranscriptDir:         transcriptDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[tutti] drive failed for %s: %v\n", d.Slug, err)
		skip := schema.DriveTest{Performed: false, SkippedReason: err.Error()}
		return &skip
	}

	// Redact transcript files in place.
	if !cf.noRedact {
		for _, run := range dt.Runs {
			p := run.TranscriptFile
			if !filepath.IsAbs(p) {
				p = filepath.Join(transcriptDir, filepath.Base(p))
			}
			if data, err := os.ReadFile(p); err == nil {
				_ = os.WriteFile(p, red.Bytes(data), 0o644)
			}
		}
	}
	// Normalise transcript paths in the manifest to be relative to capture root.
	for i := range dt.Runs {
		dt.Runs[i].TranscriptFile = filepath.Join("devices", d.Slug, "drive", filepath.Base(dt.Runs[i].TranscriptFile))
	}
	return &dt
}

func mimesFromProtocolInfo(rawPath string) []string {
	data, err := os.ReadFile(rawPath)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "=") || strings.HasPrefix(line, "SOURCE") || strings.HasPrefix(line, "SINK") {
			continue
		}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 3 {
			continue
		}
		mime := strings.SplitN(parts[2], ";", 2)[0]
		mime = strings.TrimSpace(mime)
		if strings.HasPrefix(mime, "audio/") {
			out = append(out, mime)
		}
	}
	return out
}

func deriveSlug(parsed schema.DescriptorParsed, usns []string) string {
	v := strings.ToLower(parsed.Manufacturer)
	n := strings.ToLower(parsed.FriendlyName)
	if n == "" {
		n = strings.ToLower(parsed.ModelName)
	}
	combined := strings.TrimSpace(v + " " + n)
	if combined == "" {
		return slugFromUSNs(usns)
	}
	return slugify(combined)
}

func derivedModel(p schema.DescriptorParsed) string {
	if p.ModelName != "" && !strings.EqualFold(p.ModelName, "AV Renderer Device") {
		return p.ModelName
	}
	if p.FriendlyName != "" {
		return p.FriendlyName
	}
	return p.ModelName
}

var slugSep = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugSep.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "device"
	}
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

func slugFromUSNs(usns []string) string {
	for _, u := range usns {
		if strings.HasPrefix(u, "uuid:") {
			rest := strings.TrimPrefix(u, "uuid:")
			rest = strings.SplitN(rest, "::", 2)[0]
			return "device-" + slugify(rest)
		}
	}
	return "device-unknown"
}

func slugFromMDNS(svc schema.MDNSService) string {
	host := svc.Hostname
	if host == "" {
		host = svc.Instance
	}
	return slugify(strings.TrimSuffix(host, ".local"))
}

func serviceControlURL(p schema.DescriptorParsed, serviceType string) string {
	for _, svc := range p.Services {
		if svc.ServiceType == serviceType {
			return svc.ControlURL
		}
	}
	for _, child := range p.EmbeddedDevices {
		if u := serviceControlURL(child, serviceType); u != "" {
			return u
		}
	}
	return ""
}

func hostMatches(urlRedacted string, svc schema.MDNSService) bool {
	if urlRedacted == "" {
		return false
	}
	u, err := url.Parse(urlRedacted)
	if err != nil {
		return false
	}
	h, _, _ := net.SplitHostPort(u.Host)
	if h == "" {
		h = u.Host
	}
	for _, addr := range svc.Addrs {
		if addr == h {
			return true
		}
	}
	return strings.EqualFold(svc.Hostname, h)
}

func writeJSON(path string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func ssdpResultJSON(r *ssdp.Result, red *redact.Redactor, noRedact bool) any {
	type respJSON struct {
		USN, ST, Location, Server string
		BootID, ConfigID          string
		CacheControl              string
		Source                    string
		Headers                   map[string]string
	}
	out := make([]respJSON, 0, len(r.Responses))
	for _, rr := range r.Responses {
		j := respJSON{
			USN: rr.USN, ST: rr.ST, Location: rr.Location, Server: rr.Server,
			BootID: rr.BootID, ConfigID: rr.ConfigID, CacheControl: rr.CacheControl,
			Headers: rr.Headers,
		}
		if rr.Source != nil {
			j.Source = rr.Source.String()
		}
		if !noRedact {
			j.Location = red.String(j.Location)
			j.Source = red.String(j.Source)
			redacted := map[string]string{}
			for k, v := range j.Headers {
				redacted[k] = red.String(v)
			}
			j.Headers = redacted
		}
		out = append(out, j)
	}
	return out
}

func mdnsResultJSON(r *mdns.Result, red *redact.Redactor, noRedact bool) any {
	if noRedact {
		return r.Services
	}
	out := make([]schema.MDNSService, len(r.Services))
	for i, s := range r.Services {
		out[i] = s
		out[i].Hostname = red.String(s.Hostname)
		addrs := make([]string, len(s.Addrs))
		for j, a := range s.Addrs {
			addrs[j] = red.String(a)
		}
		out[i].Addrs = addrs
		txt := map[string]string{}
		for k, v := range s.TXT {
			txt[k] = red.String(v)
		}
		out[i].TXT = txt
	}
	return out
}

func redactURL(s string, red *redact.Redactor, noRedact bool) string {
	if noRedact {
		return s
	}
	return red.String(s)
}

func redactDescriptor(p schema.DescriptorParsed, red *redact.Redactor, noRedact bool) schema.DescriptorParsed {
	if noRedact {
		return p
	}
	p.PresentationURL = red.String(p.PresentationURL)
	p.ManufacturerURL = red.String(p.ManufacturerURL)
	p.ModelURL = red.String(p.ModelURL)
	for i, ic := range p.Icons {
		ic.URL = red.String(ic.URL)
		p.Icons[i] = ic
	}
	for i, svc := range p.Services {
		svc.ScpdURL = red.String(svc.ScpdURL)
		svc.ControlURL = red.String(svc.ControlURL)
		svc.EventSubURL = red.String(svc.EventSubURL)
		p.Services[i] = svc
	}
	for i, child := range p.EmbeddedDevices {
		p.EmbeddedDevices[i] = redactDescriptor(child, red, false)
	}
	return p
}

func runDiffLibs(_ context.Context, _ []string) int {
	fmt.Fprintln(os.Stderr, "Error: tutti diff-libs is not yet implemented in this build.")
	fmt.Fprintln(os.Stderr, "  Suggested next step: run `tutti capture` and inspect devices/<slug>/decisions.json.")
	return 1
}
