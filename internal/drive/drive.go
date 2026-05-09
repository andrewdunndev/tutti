// Package drive performs the AVTransport SOAP "drive" probe against a
// UPnP MediaRenderer: SetAVTransportURI -> Play -> poll
// GetTransportInfo / GetPositionInfo. One run is recorded per tone in
// the intersected matrix; per-run transcripts mirror the shape of the
// narjo proof_of_chain.log so a human reading either format finds the
// same landmarks.
//
// The package is stdlib-only apart from the local audio package, which
// supplies the test-tone matrix and the transient HTTP server that
// hands those tones to the renderer. SOAP envelopes are hand-rolled as
// strings, matching the cm package's house style.
package drive

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/dunn.dev/tutti/internal/audio"
	"gitlab.com/dunn.dev/tutti/internal/schema"
)

// avtNS is the AVTransport:1 service namespace. Used in SOAPACTION
// headers and as the xmlns:u attribute on action elements.
const avtNS = "urn:schemas-upnp-org:service:AVTransport:1"

// soapHeaderContentType matches what cm and the narjo proof use: some
// stacks 500 if the charset attribute is missing.
const soapHeaderContentType = `text/xml; charset="utf-8"`

// userAgent is the probe identification string. Drives are explicit
// human-initiated tests, so we name the project rather than pretending
// to be a stock UPnP control point.
const userAgent = "tutti/0 (UPnP/1.0 AVTransport drive)"

// titlePrefixDIDL marks tracks whose title was authored by tutti's
// drive package and shipped to the renderer through DIDL-Lite. The
// renderer echoing this prefix back in TrackMetaData is the signal
// that DIDL-Lite is the metadata source.
const titlePrefixDIDL = "tutti probe:"

// titlePrefixEmbedded matches the prefix scripts/gen-tones.sh writes
// into the tone files via ffmpeg. The renderer echoing this prefix
// back is the signal that the device is reading metadata out of the
// container rather than from DIDL-Lite.
const titlePrefixEmbedded = "tutti embed:"

// Default polling cadence. A 12s window with 3s ticks gives four
// snapshots, which mirrors what proof_of_chain.log captures per run
// and is enough to see TRANSITIONING -> PLAYING on every device tutti
// has been pointed at so far.
const (
	defaultPollInterval = 3 * time.Second
	defaultPollDuration = 12 * time.Second
	soapTimeout         = 10 * time.Second
	stopSettle          = 500 * time.Millisecond
)

// Options configures a single Run. AVTransportControlURL and Tones are
// required; everything else has a usable zero value.
type Options struct {
	AVTransportControlURL string
	Tones                 []audio.Tone
	AudioServer           *audio.Server
	PollInterval          time.Duration
	PollDuration          time.Duration
	Force                 bool
	TranscriptDir         string
}

// Run executes the drive probe. It always returns DriveTest with the
// outcome encoded in Performed/SkippedReason/Runs; the error return is
// reserved for "we could not even reach the precheck stage" failures.
//
// Per-tone outcome:
//  1. Pre-check via GetTransportInfo. If state is PLAYING and !Force,
//     emit a single rejected run and return.
//  2. Build DIDL-Lite, SetAVTransportURI, Play.
//  3. Poll GetTransportInfo + GetPositionInfo every PollInterval for
//     PollDuration, capturing transitions and the echoed TrackMetaData.
//  4. Derive metadata_source by matching the echoed Title against the
//     "tutti probe:" / "tutti embed:" prefixes.
//  5. Write the per-run transcript under TranscriptDir.
func Run(ctx context.Context, opts Options) (schema.DriveTest, error) {
	if opts.AVTransportControlURL == "" {
		return skipped("no AVTransport control URL on device")
	}
	if opts.AudioServer == nil {
		return skipped("audio server not running")
	}
	if len(opts.Tones) == 0 {
		return skipped("no tones intersect the device's announced sink list")
	}

	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	pollDuration := opts.PollDuration
	if pollDuration <= 0 {
		pollDuration = defaultPollDuration
	}

	if opts.TranscriptDir != "" {
		if err := os.MkdirAll(opts.TranscriptDir, 0o755); err != nil {
			return schema.DriveTest{Performed: false, SkippedReason: fmt.Sprintf("create transcript dir: %v", err)}, nil
		}
	}

	test := schema.DriveTest{Performed: true}
	client := &http.Client{Timeout: soapTimeout}

	// One-shot precheck at the start. If the device is already PLAYING
	// (something the user is listening to) and --force was not set, we
	// refuse the entire run with a single rejected entry. After this
	// point every driveOne assumes the device is OK to interrupt; we
	// emit Stop between tones so the per-tone state is always STOPPED
	// at the next SetAVTransportURI.
	if rejected, run, err := initialPrecheck(ctx, client, opts); rejected {
		test.Runs = append(test.Runs, run)
		return test, nil
	} else if err != nil {
		// Network-level failure on the first SOAP call: surface as a
		// "did not run" rather than a per-tone error, since we never
		// got far enough to attempt a tone.
		return schema.DriveTest{Performed: false, SkippedReason: fmt.Sprintf("precheck GetTransportInfo: %v", err)}, nil
	}

	// Always Stop on the way out, so we never leave the device parked
	// playing one of our tones if the loop is cut short.
	defer emitStop(context.Background(), client, opts.AVTransportControlURL)

	for i, tone := range opts.Tones {
		idx := i + 1
		run := driveOne(ctx, client, opts, tone, idx, pollInterval, pollDuration)
		test.Runs = append(test.Runs, run)

		// Hand the device back to STOPPED before the next tone so the
		// next SetAVTransportURI starts from a clean state. Most
		// renderers tolerate "URI swap mid-playback" but a few choke;
		// emitting Stop is the conservative choice and matches what a
		// real control point does between tracks.
		emitStop(ctx, client, opts.AVTransportControlURL)
	}
	return test, nil
}

// initialPrecheck calls GetTransportInfo once before any tones run. It
// returns rejected=true with a single populated DriveRun when the device
// is PLAYING and Force was not set.
func initialPrecheck(ctx context.Context, client *http.Client, opts Options) (bool, schema.DriveRun, error) {
	status, body, err := soapCall(ctx, client, opts.AVTransportControlURL, "GetTransportInfo", argsInstanceOnly())
	if err != nil {
		return false, schema.DriveRun{}, err
	}
	if status < 200 || status >= 300 {
		return false, schema.DriveRun{}, fmt.Errorf("HTTP %d", status)
	}
	fields, _ := parseSimpleResponse(body)
	state := strings.ToUpper(strings.TrimSpace(fields["CurrentTransportState"]))
	if state != "PLAYING" || opts.Force {
		return false, schema.DriveRun{}, nil
	}
	rejected := schema.DriveRun{
		Scenario:       "precheck",
		MIME:           "",
		Result:         schema.DriveResultRejected,
		ResultReason:   "device was PLAYING; rerun with --force",
		Transitions:    []string{"PLAYING"},
		TranscriptFile: "",
	}
	return true, rejected, nil
}

// emitStop sends a best-effort Stop. Errors are intentionally swallowed:
// a Stop failure is a per-device quirk we record in the next tone's
// transcript (because that tone's SetAVTransportURI will see whatever
// state the device is actually in), not a fatal condition.
func emitStop(ctx context.Context, client *http.Client, controlURL string) {
	_, _, _ = soapCall(ctx, client, controlURL, "Stop", "<InstanceID>0</InstanceID>")
	// Brief settle time so the next GetTransportInfo will reflect STOPPED.
	// This matches what real control points do between tracks.
	time.Sleep(stopSettle)
}

// skipped is the canonical "did not run anything" return. It is a
// helper so the early-exit branches all agree on the schema shape.
func skipped(reason string) (schema.DriveTest, error) {
	return schema.DriveTest{Performed: false, SkippedReason: reason}, nil
}

// driveOne runs the full sequence for a single tone. The caller is
// responsible for ensuring the device is in a state ready to accept a
// new SetAVTransportURI (the orchestrator emits Stop between tones to
// guarantee this).
func driveOne(
	ctx context.Context,
	client *http.Client,
	opts Options,
	tone audio.Tone,
	idx int,
	pollInterval, pollDuration time.Duration,
) schema.DriveRun {
	scenario := tone.Scenario()
	fileURL := opts.AudioServer.URLFor(tone)

	transcriptPath := ""
	if opts.TranscriptDir != "" {
		// NN-scenario.log: zero-padded 2-digit index keeps the per-tone
		// transcripts sorted in directory listings without a separate
		// run-order metadata file.
		transcriptPath = filepath.Join(opts.TranscriptDir, fmt.Sprintf("%02d-%s.log", idx, scenario))
	}

	tr := newTranscript(scenario, tone.MIME, opts.AVTransportControlURL, fileURL)

	run := schema.DriveRun{
		Scenario:       scenario,
		MIME:           tone.MIME,
		RateHz:         tone.RateHz,
		Bits:           tone.Bits,
		Channels:       tone.Channels,
		Result:         schema.DriveResultErrored, // pessimistic default
		Transitions:    []string{},
		TranscriptFile: transcriptPath,
	}

	tStart := time.Now()
	step := 0

	// Build DIDL-Lite to ship with SetAVTransportURI. The cover-art
	// URL is left empty: tutti has no internal cover-art surface yet,
	// so the placeholder gap is documented in BuildDIDLLite rather
	// than papered over with a fake URL.
	sentMeta := schema.DIDLMetadata{
		Title:  fmt.Sprintf("tutti probe: %s", scenario),
		Artist: tone.Artist,
		Album:  tone.Album,
		ArtURL: "", // see BuildDIDLLite docstring
	}
	embeddedMeta := schema.DIDLMetadata{
		Title:  tone.Title,
		Artist: tone.Artist,
		Album:  tone.Album,
	}
	didl := BuildDIDLLite(sentMeta, tone.MIME, fileURL)

	run.MetadataDIDLLiteSent = &sentMeta
	run.MetadataEmbedded = &embeddedMeta

	// SetAVTransportURI.
	step++
	setArgs := "<InstanceID>0</InstanceID>" +
		"<CurrentURI>" + xmlEscape(fileURL) + "</CurrentURI>" +
		"<CurrentURIMetaData>" + xmlEscape(didl) + "</CurrentURIMetaData>"
	setStatus, setBody, err := soapCall(ctx, client, opts.AVTransportControlURL, "SetAVTransportURI", setArgs)
	tr.step(step, "SetAVTransportURI", setStatus, setBody, err)
	if err != nil || setStatus < 200 || setStatus >= 300 {
		run.Result = schema.DriveResultErrored
		if err != nil {
			run.ResultReason = fmt.Sprintf("SetAVTransportURI: %v", err)
		} else {
			run.ResultReason = fmt.Sprintf("SetAVTransportURI returned HTTP %d", setStatus)
		}
		run.ElapsedSeconds = time.Since(tStart).Seconds()
		writeTranscript(transcriptPath, tr.finish(run))
		return run
	}

	// Play.
	step++
	playStatus, playBody, err := soapCall(ctx, client, opts.AVTransportControlURL, "Play", "<InstanceID>0</InstanceID><Speed>1</Speed>")
	tr.step(step, "Play", playStatus, playBody, err)
	if err != nil || playStatus < 200 || playStatus >= 300 {
		run.Result = schema.DriveResultErrored
		if err != nil {
			run.ResultReason = fmt.Sprintf("Play: %v", err)
		} else {
			run.ResultReason = fmt.Sprintf("Play returned HTTP %d", playStatus)
		}
		run.ElapsedSeconds = time.Since(tStart).Seconds()
		writeTranscript(transcriptPath, tr.finish(run))
		return run
	}

	// Poll loop. We sample both GetTransportInfo and GetPositionInfo
	// each tick so the transcript records both the state machine and
	// the metadata round-trip in lockstep.
	deadline := time.Now().Add(pollDuration)
	tick := 0
	sawDIDL := false
	sawEmbedded := false
	var lastEchoedMeta *schema.DIDLMetadata
	var lastEchoedRaw string
	for {
		if time.Now().After(deadline) {
			break
		}
		tick++

		ts, tb, terr := soapCall(ctx, client, opts.AVTransportControlURL, "GetTransportInfo", argsInstanceOnly())
		tFields, _ := parseSimpleResponse(tb)
		tr.poll(tick, "transport", "GetTransportInfo", ts, tb, terr, tFields)
		if terr == nil && ts >= 200 && ts < 300 {
			state := strings.ToUpper(strings.TrimSpace(tFields["CurrentTransportState"]))
			if state != "" {
				run.Transitions = appendUnique(run.Transitions, state)
			}
		}

		ps, pb, perr := soapCall(ctx, client, opts.AVTransportControlURL, "GetPositionInfo", argsInstanceOnly())
		pFields, _ := parseSimpleResponse(pb)
		tr.poll(tick, "position", "GetPositionInfo", ps, pb, perr, pFields)
		if perr == nil && ps >= 200 && ps < 300 {
			meta := pFields["TrackMetaData"]
			if meta != "" {
				lastEchoedRaw = meta
				echoed := parseEchoedDIDL(meta)
				lastEchoedMeta = &echoed
				if strings.HasPrefix(echoed.Title, titlePrefixDIDL) {
					sawDIDL = true
				}
				if strings.HasPrefix(echoed.Title, titlePrefixEmbedded) {
					sawEmbedded = true
				}
			}
		}

		// Sleep the interval, but bail out if the context is dead so
		// callers can shut us down promptly.
		select {
		case <-ctx.Done():
			break
		case <-time.After(pollInterval):
		}
		if ctx.Err() != nil {
			break
		}
	}

	run.MetadataEchoed = lastEchoedMeta
	run.MetadataSource, run.MetadataRoundTrip = deriveMetadata(sentMeta, lastEchoedMeta, sawDIDL, sawEmbedded)
	run.Result = classifyResult(run.Transitions)
	run.ElapsedSeconds = time.Since(tStart).Seconds()

	tr.lastEchoedRaw = lastEchoedRaw
	writeTranscript(transcriptPath, tr.finish(run))
	return run
}

// argsInstanceOnly is the SOAP argument body for actions whose only
// argument is InstanceID=0 (GetTransportInfo, GetPositionInfo, Stop).
func argsInstanceOnly() string {
	return "<InstanceID>0</InstanceID>"
}

// classifyResult collapses an ordered list of observed transport
// states into the single drive_run.result enum value. Order matters:
// PLAYING dominates TRANSITIONING dominates STOPPED, and an empty list
// becomes "errored" because the SOAP layer should already have given
// us at least one state observation.
func classifyResult(transitions []string) schema.DriveResult {
	sawPlaying := false
	sawTrans := false
	sawStopped := false
	for _, s := range transitions {
		switch strings.ToUpper(s) {
		case "PLAYING":
			sawPlaying = true
		case "TRANSITIONING":
			sawTrans = true
		case "STOPPED":
			sawStopped = true
		}
	}
	switch {
	case sawPlaying:
		return schema.DriveResultPlaying
	case sawTrans:
		return schema.DriveResultTransitioning
	case sawStopped:
		return schema.DriveResultStopped
	default:
		return schema.DriveResultErrored
	}
}

// deriveMetadata maps the (sent, echoed, sawDIDL, sawEmbedded) tuple
// to the metadata_source / metadata_round_trip enum pair documented in
// the schema. The "mixed" case fires when the device echoed both
// prefixes at different polls (commonly: an initial DIDL-Lite echo
// followed by the device replacing it with whatever it parsed out of
// the container itself).
func deriveMetadata(sent schema.DIDLMetadata, echoed *schema.DIDLMetadata, sawDIDL, sawEmbedded bool) (schema.MetadataSource, schema.MetadataRoundTrip) {
	if echoed == nil {
		return schema.MetadataSourceNone, schema.MetadataRoundTripAbsent
	}
	if sawDIDL && sawEmbedded {
		return schema.MetadataSourceMixed, schema.MetadataRoundTripMismatched
	}
	if sawDIDL {
		if echoed.Title == sent.Title {
			return schema.MetadataSourceDIDLLite, schema.MetadataRoundTripMatched
		}
		return schema.MetadataSourceDIDLLite, schema.MetadataRoundTripMismatched
	}
	if sawEmbedded {
		// We did not "send" the embedded title via DIDL-Lite, so the
		// matched/mismatched axis is ambiguous; the cleanest signal is
		// "mismatched against what we sent in DIDL-Lite," which is
		// what a downstream renderer-vendor reviewer cares about.
		return schema.MetadataSourceEmbedded, schema.MetadataRoundTripMismatched
	}
	// Echoed metadata exists but does not carry either prefix: the
	// renderer has rewritten the title entirely. Treat as "none" with
	// the echoed payload still recorded for the human reader.
	return schema.MetadataSourceNone, schema.MetadataRoundTripAbsent
}

// appendUnique appends s to xs only if it is not already the most
// recent entry. Keeping order means a STOPPED -> TRANSITIONING ->
// PLAYING sequence is preserved; deduplication keeps the list short
// when a device sits in PLAYING for the whole poll window.
func appendUnique(xs []string, s string) []string {
	if len(xs) > 0 && xs[len(xs)-1] == s {
		return xs
	}
	return append(xs, s)
}

// BuildDIDLLite returns the CurrentURIMetaData payload tutti ships in
// SetAVTransportURI. The returned string is the inner XML (a
// <DIDL-Lite>...</DIDL-Lite> element); the caller is expected to
// XML-escape it once when wrapping it in the SOAP envelope.
//
// Cover art: ArtURL is rendered as <upnp:albumArtURI> when non-empty.
// Today tutti has no internal cover-art surface, so callers pass "".
// The element is omitted in that case rather than emitting an empty
// URL that would (a) clutter the transcript and (b) potentially make
// some renderers mark art-fetch as failed when it was simply absent.
func BuildDIDLLite(meta schema.DIDLMetadata, mime string, fileURL string) string {
	var b strings.Builder
	b.WriteString(`<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" `)
	b.WriteString(`xmlns:dc="http://purl.org/dc/elements/1.1/" `)
	b.WriteString(`xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">`)
	b.WriteString(`<item id="tutti-probe-1" parentID="0" restricted="1">`)
	b.WriteString(`<dc:title>`)
	b.WriteString(xmlEscape(meta.Title))
	b.WriteString(`</dc:title>`)
	if meta.Artist != "" {
		b.WriteString(`<upnp:artist>`)
		b.WriteString(xmlEscape(meta.Artist))
		b.WriteString(`</upnp:artist>`)
		// dc:creator is the field DLNA-tier renderers actually look at;
		// upnp:artist is the modern field. Emitting both maximizes the
		// odds the device will pick up an artist string at all.
		b.WriteString(`<dc:creator>`)
		b.WriteString(xmlEscape(meta.Artist))
		b.WriteString(`</dc:creator>`)
	}
	if meta.Album != "" {
		b.WriteString(`<upnp:album>`)
		b.WriteString(xmlEscape(meta.Album))
		b.WriteString(`</upnp:album>`)
	}
	b.WriteString(`<upnp:class>object.item.audioItem.musicTrack</upnp:class>`)
	b.WriteString(`<res protocolInfo="http-get:*:`)
	b.WriteString(xmlEscape(mime))
	b.WriteString(`:*">`)
	b.WriteString(xmlEscape(fileURL))
	b.WriteString(`</res>`)
	if meta.ArtURL != "" {
		b.WriteString(`<upnp:albumArtURI>`)
		b.WriteString(xmlEscape(meta.ArtURL))
		b.WriteString(`</upnp:albumArtURI>`)
	}
	b.WriteString(`</item>`)
	b.WriteString(`</DIDL-Lite>`)
	return b.String()
}

// soapCall posts a SOAP request and returns the (status, body, error)
// triple. The body is always returned (even on non-2xx) so the caller
// can record SOAP faults verbatim in the transcript.
func soapCall(ctx context.Context, client *http.Client, controlURL, action, argsXML string) (int, []byte, error) {
	envelope := `<?xml version="1.0" encoding="utf-8"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" ` +
		`s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
		`<s:Body>` +
		`<u:` + action + ` xmlns:u="` + avtNS + `">` +
		argsXML +
		`</u:` + action + `>` +
		`</s:Body>` +
		`</s:Envelope>`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, controlURL, strings.NewReader(envelope))
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", soapHeaderContentType)
	req.Header.Set("SOAPAction", `"`+avtNS+`#`+action+`"`)
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, body, nil
}

// parseSimpleResponse pulls every element with non-empty text content
// out of a SOAP response body, keyed by local tag name. We deliberately
// flatten across the envelope (Envelope/Body/<Action>Response/<field>)
// so callers can read fields like CurrentTransportState or
// TrackMetaData with one map lookup. SOAP faults are caught and
// surfaced as an error.
func parseSimpleResponse(body []byte) (map[string]string, error) {
	out := map[string]string{}
	if len(body) == 0 {
		return out, nil
	}
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity

	var stack []string
	var faultParts []string
	inFault := false
	var sb strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, fmt.Errorf("decode response: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			stack = append(stack, t.Name.Local)
			if t.Name.Local == "Fault" {
				inFault = true
			}
			sb.Reset()
		case xml.EndElement:
			text := strings.TrimSpace(sb.String())
			sb.Reset()
			top := ""
			if len(stack) > 0 {
				top = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			if top != t.Name.Local {
				// Mismatched tags: with Strict=false we may see this on
				// minor whitespace oddities. Best-effort recovery.
			}
			if inFault {
				if t.Name.Local == "Fault" {
					inFault = false
				} else if text != "" {
					faultParts = append(faultParts, t.Name.Local+"="+text)
				}
				continue
			}
			if text != "" {
				switch t.Name.Local {
				case "Envelope", "Body":
					// skip wrappers
				default:
					if !strings.HasSuffix(t.Name.Local, "Response") {
						out[t.Name.Local] = text
					}
				}
			}
		case xml.CharData:
			sb.Write([]byte(t))
		}
	}
	if len(faultParts) > 0 {
		return out, fmt.Errorf("SOAP fault: %s", strings.Join(faultParts, "; "))
	}
	return out, nil
}

// parseEchoedDIDL reads a TrackMetaData payload as the device echoed
// it. The payload is itself a DIDL-Lite document (which the device may
// have rewritten); we only need a few fields, so we walk it the same
// way we walk SOAP responses.
//
// This is the most fragile parser in the package: devices have been
// observed truncating, namespace-stripping, or replacing fields here.
// We pull what we can, leave the rest empty, and let the manifest
// surface "metadata_source=none" when the result is unrecognisable.
func parseEchoedDIDL(payload string) schema.DIDLMetadata {
	out := schema.DIDLMetadata{}
	if strings.TrimSpace(payload) == "" {
		return out
	}
	dec := xml.NewDecoder(strings.NewReader(payload))
	dec.Strict = false
	dec.Entity = xml.HTMLEntity
	dec.AutoClose = xml.HTMLAutoClose

	var sb strings.Builder
	var current string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			current = t.Name.Local
			sb.Reset()
		case xml.EndElement:
			text := strings.TrimSpace(sb.String())
			sb.Reset()
			if text == "" {
				current = ""
				continue
			}
			switch t.Name.Local {
			case "title":
				if out.Title == "" {
					out.Title = text
				}
			case "artist":
				if out.Artist == "" {
					out.Artist = text
				}
			case "creator":
				if out.Artist == "" {
					out.Artist = text
				}
			case "album":
				if out.Album == "" {
					out.Album = text
				}
			case "albumArtURI":
				if out.ArtURL == "" {
					out.ArtURL = text
				}
			}
			current = ""
		case xml.CharData:
			if current != "" {
				sb.Write([]byte(t))
			}
		}
	}
	return out
}

// xmlEscape wraps the standard library helper so the rest of the file
// can stay readable. We use the text-content escaper because the SOAP
// arguments live as element text, never inside attributes (those are
// all literals).
func xmlEscape(s string) string {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		// xml.EscapeText cannot fail for an in-memory buffer, but if
		// it ever does we would rather drop the content than panic
		// inside a probe.
		return ""
	}
	return b.String()
}

// transcript builds the per-run .log file contents. The shape mirrors
// the narjo proof_of_chain.log: a header block, "--- step N ---" / "--- poll N ---"
// dividers, then a footer summary. The format is line-oriented so a
// human can grep it; the schema-level structure lives in manifest.json
// alongside.
type transcript struct {
	scenario      string
	mime          string
	controlURL    string
	fileURL       string
	startedAt     time.Time
	body          bytes.Buffer
	lastEchoedRaw string
}

func newTranscript(scenario, mime, controlURL, fileURL string) *transcript {
	t := &transcript{
		scenario:   scenario,
		mime:       mime,
		controlURL: controlURL,
		fileURL:    fileURL,
		startedAt:  time.Now().UTC(),
	}
	fmt.Fprintf(&t.body, "[avt] scenario: %s\n", scenario)
	fmt.Fprintf(&t.body, "[avt] mime:     %s\n", mime)
	fmt.Fprintf(&t.body, "[avt] control:  %s\n", controlURL)
	fmt.Fprintf(&t.body, "[avt] tone:     %s\n", fileURL)
	fmt.Fprintf(&t.body, "[avt] started:  %s\n", t.startedAt.Format(time.RFC3339))
	return t
}

// step records a SOAP step in the transcript. err is recorded verbatim
// when non-nil; otherwise the parsed response fields are dumped one per
// line, with TrackMetaData truncated the same way as the narjo log.
func (t *transcript) step(n int, action string, status int, body []byte, err error) {
	fmt.Fprintf(&t.body, "\n--- step %d: %s -> ", n, action)
	if err != nil {
		fmt.Fprintf(&t.body, "ERROR ---\n  %v\n", err)
		return
	}
	fmt.Fprintf(&t.body, "HTTP %d ---\n", status)
	t.dumpFields(body)
}

// poll records one half of a poll tick (transport or position). The
// "kind" string is the same vocabulary the narjo log uses ("transport"
// / "position") so a reader can scan for a tick and see both halves.
func (t *transcript) poll(n int, kind, action string, status int, body []byte, err error, fields map[string]string) {
	fmt.Fprintf(&t.body, "\n--- poll %d %s: %s -> ", n, kind, action)
	if err != nil {
		fmt.Fprintf(&t.body, "ERROR ---\n  %v\n", err)
		return
	}
	fmt.Fprintf(&t.body, "HTTP %d ---\n", status)
	if len(fields) == 0 {
		fmt.Fprintf(&t.body, "  (no fields parsed; raw %d bytes)\n", len(body))
		return
	}
	for _, k := range stableKeys(fields) {
		v := fields[k]
		if len(v) > 200 {
			v = v[:200] + " ...[truncated]"
		}
		fmt.Fprintf(&t.body, "  %s: %s\n", k, v)
	}
}

// dumpFields parses a body for human display. Used for SOAP step
// responses that we did not pre-parse, e.g. SetAVTransportURI's empty
// body which still has a useful byte count.
func (t *transcript) dumpFields(body []byte) {
	fields, err := parseSimpleResponse(body)
	if err != nil {
		fmt.Fprintf(&t.body, "  PARSE ERROR: %v\n", err)
		fmt.Fprintf(&t.body, "  RAW: %s\n", truncate(string(body), 256))
		return
	}
	if len(fields) == 0 {
		fmt.Fprintf(&t.body, "  (no fields in response, raw %d bytes)\n", len(body))
		return
	}
	for _, k := range stableKeys(fields) {
		v := fields[k]
		if len(v) > 200 {
			v = v[:200] + " ...[truncated]"
		}
		fmt.Fprintf(&t.body, "  %s: %s\n", k, v)
	}
}

// finish emits the closing summary block and returns the full
// transcript bytes ready to be written to disk.
func (t *transcript) finish(run schema.DriveRun) []byte {
	finalState := ""
	if len(run.Transitions) > 0 {
		finalState = run.Transitions[len(run.Transitions)-1]
	}
	fmt.Fprintf(&t.body, "\n[avt] elapsed: %.2fs\n", run.ElapsedSeconds)
	fmt.Fprintf(&t.body, "[avt] result:  %s\n", run.Result)
	if run.ResultReason != "" {
		fmt.Fprintf(&t.body, "[avt] reason:  %s\n", run.ResultReason)
	}
	fmt.Fprintf(&t.body, "[avt] final:   %s\n", finalState)
	fmt.Fprintf(&t.body, "[avt] transitions: %s\n", strings.Join(run.Transitions, " -> "))
	if run.MetadataSource != "" {
		fmt.Fprintf(&t.body, "[avt] metadata_source: %s\n", run.MetadataSource)
	}
	if run.MetadataRoundTrip != "" {
		fmt.Fprintf(&t.body, "[avt] metadata_round_trip: %s\n", run.MetadataRoundTrip)
	}
	return t.body.Bytes()
}

// writeTranscript writes the transcript bytes to disk if a path was
// configured. Errors are silently dropped: the manifest already
// carries the structured outcome, and the caller cannot do anything
// useful with a "could not write debug log" error mid-probe.
func writeTranscript(path string, content []byte) {
	if path == "" {
		return
	}
	_ = os.WriteFile(path, content, 0o644)
}

// stableKeys returns the keys of m in lexicographic order. Used so the
// transcript output is deterministic for review and (eventually)
// golden-file tests.
func stableKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// simple insertion sort: maps here are tiny (single-digit keys)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ErrNoControlURL is returned by Run when the device has no
// AVTransport control URL. Exposed so the orchestrator can distinguish
// "we deliberately skipped" from "everything blew up."
var ErrNoControlURL = errors.New("drive: no AVTransport control URL")
