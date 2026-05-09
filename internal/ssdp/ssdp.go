// Package ssdp drives an M-SEARCH probe across one or more local
// interfaces, captures every unicast reply, and renders both a
// structured Result and a narjo-style text dump suitable for the
// ssdp-raw.txt artifact in a tutti capture bundle.
//
// Stdlib only. The whole point of tutti is to be a transparent reference
// for "what does this LAN actually advertise?" so we own the parser
// rather than wrapping a third-party SSDP library that swallows the
// quirks we care about.
package ssdp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// MulticastAddr is the SSDP IPv4 multicast group + port. Hard-coded by
// the SSDP/UPnP spec; we never reach IPv6 SSDP for tutti's scope.
const MulticastAddr = "239.255.255.250:1900"

// DefaultMX is the MX value used when callers pass mxSeconds <= 0.
// SSDP MX is the upper bound on how long a responder may delay its
// reply; 3 seconds is the sweet spot most tutti targets honor without
// dropping replies.
const DefaultMX = 3

// DefaultSTList is the search-target list used when the caller passes
// nil or empty. Mirrors narjo-eversolo-upnp/ssdp_dump.py so output is
// directly comparable.
var DefaultSTList = []string{
	"ssdp:all",
	"upnp:rootdevice",
	"urn:schemas-upnp-org:device:MediaRenderer:1",
	"urn:schemas-upnp-org:service:AVTransport:1",
}

// Response is one parsed SSDP unicast reply.
//
// Headers preserves every header exactly as parsed, with the keys
// uppercased on the way in (SSDP/HTTP header names are case-insensitive,
// devices are inconsistent). Canonical fields are convenience accessors
// pulled from Headers at parse time. RawHTTP is the original byte
// payload so the ssdp-raw.txt artifact can be re-derived without
// re-flattening through Headers.
type Response struct {
	USN          string
	ST           string
	Location     string
	Server       string
	BootID       string
	ConfigID     string
	CacheControl string
	Source       net.Addr
	Headers      map[string]string
	RawHTTP      string
}

// Result is the aggregate output of one Probe call.
//
// Responses preserves arrival order. RawDump is the formatted text
// suitable for writing verbatim to ssdp-raw.txt.
type Result struct {
	Responses []Response
	RawDump   string
}

// Probe sends an M-SEARCH for each ST in stList from each requested
// interface, listens for unicast replies for mxSeconds+2 seconds, and
// returns the aggregated result.
//
// If ifaces is nil/empty, Probe walks net.Interfaces and uses every
// interface that is up, supports multicast, and has at least one IPv4
// address. ctx cancels early; partial results are still returned.
func Probe(ctx context.Context, ifaces []string, stList []string, mxSeconds int) (*Result, error) {
	if mxSeconds <= 0 {
		mxSeconds = DefaultMX
	}
	if len(stList) == 0 {
		stList = DefaultSTList
	}

	selected, err := pickInterfaces(ifaces)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, errors.New("ssdp: no usable IPv4 multicast interfaces")
	}

	mcast, err := net.ResolveUDPAddr("udp4", MulticastAddr)
	if err != nil {
		return nil, fmt.Errorf("ssdp: resolve multicast addr: %w", err)
	}

	listenFor := time.Duration(mxSeconds+2) * time.Second
	deadline := time.Now().Add(listenFor)

	var (
		mu        sync.Mutex
		responses []Response
		wg        sync.WaitGroup
		probeErrs []error
	)

	addResp := func(r Response) {
		mu.Lock()
		responses = append(responses, r)
		mu.Unlock()
	}
	addErr := func(e error) {
		mu.Lock()
		probeErrs = append(probeErrs, e)
		mu.Unlock()
	}

	for _, ifi := range selected {
		wg.Add(1)
		go func(ifi *net.Interface) {
			defer wg.Done()
			if err := probeOne(ctx, ifi, mcast, stList, mxSeconds, deadline, addResp); err != nil {
				addErr(fmt.Errorf("ssdp: iface %s: %w", ifi.Name, err))
			}
		}(ifi)
	}

	wg.Wait()

	dump := formatDump(stList, mxSeconds, responses)
	res := &Result{Responses: responses, RawDump: dump}

	// Fail only if every interface errored AND we got no responses.
	// A noisy LAN with one busted iface should still produce a result.
	if len(responses) == 0 && len(probeErrs) > 0 && len(probeErrs) == len(selected) {
		return res, errors.Join(probeErrs...)
	}
	return res, nil
}

func probeOne(ctx context.Context, ifi *net.Interface, mcast *net.UDPAddr, stList []string, mxSeconds int, deadline time.Time, add func(Response)) error {
	ip4, err := ifaceIPv4(ifi)
	if err != nil {
		return err
	}

	conn, err := dialBound(ip4)
	if err != nil {
		return fmt.Errorf("bind: %w", err)
	}
	defer conn.Close()

	for _, st := range stList {
		payload := buildMSearch(st, mxSeconds)
		if _, err := conn.WriteTo(payload, mcast); err != nil {
			// Soft-fail: log via returned error only if every send fails;
			// for now keep going so other STs still get a chance.
			continue
		}
	}

	// Reader goroutine bounded by listenFor.
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		buf := make([]byte, 8192)
		for {
			if err := conn.SetReadDeadline(deadline); err != nil {
				return
			}
			n, src, err := conn.ReadFrom(buf)
			if err != nil {
				// Deadline or closed — either way, we're done.
				return
			}
			if n == 0 {
				continue
			}
			raw := append([]byte(nil), buf[:n]...)
			resp, perr := ParseResponse(raw, src)
			if perr != nil {
				continue
			}
			add(resp)
		}
	}()

	select {
	case <-ctx.Done():
		_ = conn.Close()
		<-doneCh
		return nil
	case <-doneCh:
		return nil
	}
}

// dialBound binds a UDP socket to the given IPv4 address with
// SO_REUSEADDR/SO_REUSEPORT set so multiple interfaces can share the
// SSDP listening side without EADDRINUSE on multi-iface hosts.
func dialBound(ip4 net.IP) (*net.UDPConn, error) {
	lc := net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			var sockErr error
			if err := c.Control(func(fd uintptr) {
				if e := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); e != nil {
					sockErr = e
					return
				}
				// SO_REUSEPORT: not portable on every BSD/Linux combo;
				// best-effort, ignore failure.
				if e := setReusePort(int(fd)); e != nil {
					// non-fatal
					_ = e
				}
			}); err != nil {
				return err
			}
			return sockErr
		},
	}
	pc, err := lc.ListenPacket(context.Background(), "udp4", net.JoinHostPort(ip4.String(), "0"))
	if err != nil {
		return nil, err
	}
	uc, ok := pc.(*net.UDPConn)
	if !ok {
		_ = pc.Close()
		return nil, fmt.Errorf("not a UDPConn: %T", pc)
	}
	return uc, nil
}

func buildMSearch(st string, mxSeconds int) []byte {
	// Note: M-SEARCH lines must be CRLF and end with a blank line.
	return []byte(
		"M-SEARCH * HTTP/1.1\r\n" +
			"HOST: " + MulticastAddr + "\r\n" +
			"MAN: \"ssdp:discover\"\r\n" +
			fmt.Sprintf("MX: %d\r\n", mxSeconds) +
			"ST: " + st + "\r\n" +
			"\r\n",
	)
}

func pickInterfaces(names []string) ([]*net.Interface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("ssdp: enumerate interfaces: %w", err)
	}

	want := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			want[n] = true
		}
	}

	var out []*net.Interface
	for i := range all {
		ifi := &all[i]
		if len(want) > 0 {
			if !want[ifi.Name] {
				continue
			}
		} else {
			// Auto-pick: up + multicast + has IPv4.
			if ifi.Flags&net.FlagUp == 0 {
				continue
			}
			if ifi.Flags&net.FlagMulticast == 0 {
				continue
			}
		}
		if _, err := ifaceIPv4(ifi); err != nil {
			continue
		}
		out = append(out, ifi)
	}
	return out, nil
}

func ifaceIPv4(ifi *net.Interface) (net.IP, error) {
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil {
			continue
		}
		if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() && !ip4.IsUnspecified() && !ip4.IsLinkLocalUnicast() {
			return ip4, nil
		}
	}
	return nil, fmt.Errorf("no IPv4 address on %s", ifi.Name)
}

// ParseResponse turns a single SSDP unicast reply into a Response.
//
// Lenient by design: a non-trivial number of UPnP devices emit slightly
// malformed HTTP/1.1 (missing reason phrase, mixed CRLF/LF, lowercase
// header names, trailing junk). We only insist on a status line that
// starts with HTTP/1. and recover what we can from the headers.
func ParseResponse(data []byte, src net.Addr) (Response, error) {
	r := Response{
		Source:  src,
		Headers: map[string]string{},
		RawHTTP: string(data),
	}

	// Normalize line endings for parsing only; RawHTTP keeps the wire bytes.
	norm := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	scanner := bufio.NewScanner(bytes.NewReader(norm))
	scanner.Buffer(make([]byte, 0, 8192), 1<<20)

	if !scanner.Scan() {
		return r, errors.New("ssdp: empty response")
	}
	statusLine := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(statusLine, "HTTP/1.") && !strings.HasPrefix(statusLine, "NOTIFY ") {
		return r, fmt.Errorf("ssdp: not an HTTP/1.x reply: %q", trimForErr(statusLine))
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			break
		}
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToUpper(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		// Last write wins; SSDP responses don't fold/repeat in practice.
		r.Headers[key] = val
	}

	r.USN = r.Headers["USN"]
	r.ST = r.Headers["ST"]
	r.Location = r.Headers["LOCATION"]
	r.Server = r.Headers["SERVER"]
	r.CacheControl = r.Headers["CACHE-CONTROL"]
	r.BootID = r.Headers["BOOTID.UPNP.ORG"]
	r.ConfigID = r.Headers["CONFIGID.UPNP.ORG"]

	return r, nil
}

func trimForErr(s string) string {
	if len(s) > 64 {
		return s[:64] + "..."
	}
	return s
}

// GroupByUSN buckets responses by USN. One device emits the same
// LOCATION for multiple ST values (root device, embedded devices, each
// service); the USN is the stable per-(device, role) key.
//
// Within each bucket we de-dupe on (ST, LOCATION) so that re-sends from
// the same role don't pollute the dump.
func GroupByUSN(rs []Response) map[string][]Response {
	out := map[string][]Response{}
	type key struct{ st, loc string }
	seen := map[string]map[key]bool{}
	for _, r := range rs {
		usn := r.USN
		if usn == "" {
			usn = "<no-usn>"
		}
		k := key{r.ST, r.Location}
		if seen[usn] == nil {
			seen[usn] = map[key]bool{}
		}
		if seen[usn][k] {
			continue
		}
		seen[usn][k] = true
		out[usn] = append(out[usn], r)
	}
	return out
}

// UniqueLocations returns each distinct LOCATION URL once, in arrival
// order. Empty LOCATIONs are skipped.
func UniqueLocations(rs []Response) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rs {
		if r.Location == "" {
			continue
		}
		if seen[r.Location] {
			continue
		}
		seen[r.Location] = true
		out = append(out, r.Location)
	}
	return out
}

// formatDump renders the narjo-style dump that gets written to
// ssdp-raw.txt. The shape is mechanically the same as
// narjo-eversolo-upnp/ssdp_dump.txt so a human can diff a tutti capture
// against the reference dump.
func formatDump(stList []string, mxSeconds int, responses []Response) string {
	var b strings.Builder

	fmt.Fprintf(&b, "[ssdp] M-SEARCH -> %s, MX=%d\n", MulticastAddr, mxSeconds)
	for _, st := range stList {
		fmt.Fprintf(&b, "[ssdp]   sent ST: %s\n", st)
	}
	b.WriteString("\n")

	groups := GroupByUSN(responses)
	locs := UniqueLocations(responses)

	fmt.Fprintf(&b, "[ssdp] %d raw response(s), %d unique USN(s), %d unique LOCATION(s)\n",
		len(responses), len(groups), len(locs))

	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", 78) + "\n")
	b.WriteString("LOCATIONS (feed each into descriptor_fetch)\n")
	b.WriteString(strings.Repeat("=", 78) + "\n")

	sortedLocs := append([]string(nil), locs...)
	sort.Strings(sortedLocs)
	for _, l := range sortedLocs {
		fmt.Fprintf(&b, "  %s\n", l)
	}

	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", 78) + "\n")
	b.WriteString("DEVICES (grouped by USN)\n")
	b.WriteString(strings.Repeat("=", 78) + "\n")

	skip := map[string]bool{
		"ST": true, "LOCATION": true, "SERVER": true, "USN": true,
		"CACHE-CONTROL": true, "EXT": true, "DATE": true, "HOST": true,
	}

	usns := make([]string, 0, len(groups))
	for u := range groups {
		usns = append(usns, u)
	}
	sort.Strings(usns)

	for _, usn := range usns {
		fmt.Fprintf(&b, "\n--- %s ---\n", usn)
		for _, h := range groups[usn] {
			from := ""
			if h.Source != nil {
				from = h.Source.String()
			}
			fmt.Fprintf(&b, "  FROM:     %s\n", from)
			fmt.Fprintf(&b, "  ST:       %s\n", h.ST)
			fmt.Fprintf(&b, "  LOCATION: %s\n", h.Location)
			fmt.Fprintf(&b, "  SERVER:   %s\n", h.Server)

			extra := make([]string, 0, len(h.Headers))
			for k := range h.Headers {
				if skip[k] {
					continue
				}
				extra = append(extra, k)
			}
			sort.Strings(extra)
			for _, k := range extra {
				fmt.Fprintf(&b, "  %s: %s\n", k, h.Headers[k])
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}
