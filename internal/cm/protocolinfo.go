// Package cm implements ConnectionManager:1 SOAP helpers used by tutti.
//
// The only call tutti needs from ConnectionManager today is GetProtocolInfo:
// the renderer's announced Source/Sink lists, from which we derive
// audio-sink counts and per-format-family matches. The drive package will
// later issue AVTransport SOAP calls in the same style; this file is the
// reference shape for those.
package cm

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.com/dunn.dev/tutti/internal/schema"
)

// soapEnvelopeRequest is the GetProtocolInfo request body. Hand-rolling
// the envelope as a string template keeps us off any SOAP library and
// matches the byte-for-byte shape Platinum/-style stacks expect.
const soapEnvelopeRequest = `<?xml version="1.0" encoding="utf-8"?>` +
	`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" ` +
	`s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
	`<s:Body>` +
	`<u:GetProtocolInfo xmlns:u="urn:schemas-upnp-org:service:ConnectionManager:1"/>` +
	`</s:Body>` +
	`</s:Envelope>`

// soapAction is the SOAPACTION header value for GetProtocolInfo. The
// quotes are required by the UPnP Device Architecture spec; some stacks
// silently 500 if they are stripped.
const soapAction = `"urn:schemas-upnp-org:service:ConnectionManager:1#GetProtocolInfo"`

// formatFamily binds a family name to the substrings (in the third,
// MIME field of a protocolInfo entry) that count as a hit. All matches
// are done lowercased.
type formatFamily struct {
	name      string
	substrs   []string
	detectable bool
}

// formatFamilies is the canonical detection table. Order is preserved
// in the resulting FormatMatches map iteration only if a caller sorts;
// the values themselves are deterministic.
var formatFamilies = []formatFamily{
	{name: "flac", substrs: []string{"audio/flac"}, detectable: true},
	{name: "alac", substrs: []string{"audio/alac", "audio/x-m4a"}, detectable: true},
	{name: "mp3", substrs: []string{"audio/mpeg", "audio/mp3"}, detectable: true},
	{name: "aac", substrs: []string{"audio/aac", "audio/mp4", "audio/x-aac"}, detectable: true},
	{name: "opus", substrs: []string{"audio/opus"}, detectable: true},
	{name: "vorbis", substrs: []string{"audio/vorbis", "audio/ogg"}, detectable: true},
	{name: "wav", substrs: []string{"audio/wav", "audio/x-wav", "audio/x-pcm"}, detectable: true},
	{name: "dsd", substrs: []string{"audio/dsd", "audio/dsf", "audio/x-dsd", "audio/x-dsf"}, detectable: true},
	// MQA cannot be reliably read off a MIME string. We record the family
	// but with a nil count so consumers can distinguish "we did not detect"
	// from "the device announced zero".
	{name: "mqa", detectable: false},
}

// soapEnvelopeResponse and friends are the response decode shape. We do
// not attempt to validate the SOAP envelope structure beyond locating
// Source and Sink under whatever element tutti happens to find in the
// body, since some implementations omit the s: namespace prefix.
type soapEnvelopeResponse struct {
	XMLName xml.Name        `xml:"Envelope"`
	Body    soapBodyResponse `xml:"Body"`
}

type soapBodyResponse struct {
	GetProtocolInfoResponse getProtocolInfoResponse `xml:"GetProtocolInfoResponse"`
	Fault                   *soapFault              `xml:"Fault,omitempty"`
}

type getProtocolInfoResponse struct {
	Source string `xml:"Source"`
	Sink   string `xml:"Sink"`
}

type soapFault struct {
	FaultCode   string `xml:"faultcode"`
	FaultString string `xml:"faultstring"`
}

// GetProtocolInfo SOAP-calls ConnectionManager.GetProtocolInfo at the
// given control URL and returns the parsed ProtocolInfo plus a raw text
// dump suitable for writing to protocol-info.txt. The RawFile field on
// the returned ProtocolInfo is left empty: the orchestrator picks the
// per-device path.
func GetProtocolInfo(ctx context.Context, controlURL string) (schema.ProtocolInfo, []byte, error) {
	if controlURL == "" {
		return schema.ProtocolInfo{}, nil, errors.New("cm: empty control URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, controlURL, strings.NewReader(soapEnvelopeRequest))
	if err != nil {
		return schema.ProtocolInfo{}, nil, fmt.Errorf("cm: build request: %w", err)
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", soapAction)
	req.Header.Set("User-Agent", "tutti/0 (UPnP/1.0 ConnectionManager probe)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return schema.ProtocolInfo{}, nil, fmt.Errorf("cm: do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return schema.ProtocolInfo{}, nil, fmt.Errorf("cm: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Even a 500 carries a SOAP fault we may want to surface verbatim.
		return schema.ProtocolInfo{}, nil, fmt.Errorf("cm: http %d: %s", resp.StatusCode, truncate(string(body), 256))
	}

	source, sink, err := decodeProtocolInfoResponse(body)
	if err != nil {
		return schema.ProtocolInfo{}, nil, err
	}

	pi := ParseProtocolInfo(source, sink)
	dump := renderRawDump(controlURL, source, sink, pi, time.Now().UTC())
	return pi, dump, nil
}

// decodeProtocolInfoResponse extracts <Source> and <Sink> from a
// GetProtocolInfoResponse body, tolerating both namespaced and bare
// envelope elements.
func decodeProtocolInfoResponse(body []byte) (source, sink string, err error) {
	var env soapEnvelopeResponse
	if e := xml.Unmarshal(body, &env); e != nil {
		return "", "", fmt.Errorf("cm: decode envelope: %w", e)
	}
	if env.Body.Fault != nil {
		return "", "", fmt.Errorf("cm: SOAP fault: %s: %s",
			env.Body.Fault.FaultCode, env.Body.Fault.FaultString)
	}
	return env.Body.GetProtocolInfoResponse.Source,
		env.Body.GetProtocolInfoResponse.Sink, nil
}

// ParseProtocolInfo takes raw Source/Sink strings (each a comma-separated
// list of "schema:network:mime:additional-info" entries) and returns a
// ProtocolInfo with derived counts. RawFile is left empty.
func ParseProtocolInfo(source, sink string) schema.ProtocolInfo {
	srcEntries := splitProtocolInfo(source)
	sinkEntries := splitProtocolInfo(sink)

	pi := schema.ProtocolInfo{
		SourceCount:    len(srcEntries),
		SinkCount:      len(sinkEntries),
		AudioSinkCount: 0,
		FormatMatches:  make(map[string]*int, len(formatFamilies)),
	}

	// Initialise every family. Detectable families start at zero so the
	// JSON payload distinguishes "0 hits" (integer) from "not measured"
	// (null). MQA is the standing example of the latter.
	for _, fam := range formatFamilies {
		if fam.detectable {
			zero := 0
			pi.FormatMatches[fam.name] = &zero
		} else {
			pi.FormatMatches[fam.name] = nil
		}
	}

	for _, entry := range sinkEntries {
		mime := mimeField(entry)
		if mime == "" {
			continue
		}
		if !strings.HasPrefix(mime, "audio/") {
			continue
		}
		pi.AudioSinkCount++

		for _, fam := range formatFamilies {
			if !fam.detectable {
				continue
			}
			for _, needle := range fam.substrs {
				if strings.Contains(mime, needle) {
					*pi.FormatMatches[fam.name]++
					break
				}
			}
		}
	}

	return pi
}

// splitProtocolInfo splits a Source/Sink list on commas, trims each
// entry, and drops empties. Entries themselves contain colons but never
// raw commas in any UPnP corpus we have on hand, so a plain comma split
// is correct.
func splitProtocolInfo(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// mimeField returns the third (MIME) field of a protocolInfo entry,
// lowercased. Entries are "schema:network:mime:additional-info"; the
// MIME field itself can contain semicolons (e.g. audio/L16;rate=...).
func mimeField(entry string) string {
	// SplitN with n=4 keeps the additional-info field intact even if it
	// itself contains colons (it almost always does).
	parts := strings.SplitN(entry, ":", 4)
	if len(parts) < 3 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parts[2]))
}

// renderRawDump produces the human-readable protocol-info.txt content
// described in the schema docs. The orchestrator writes this verbatim
// alongside the manifest.
func renderRawDump(controlURL, source, sink string, pi schema.ProtocolInfo, now time.Time) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# tutti -- ConnectionManager:GetProtocolInfo\n")
	fmt.Fprintf(&b, "# captured: %s\n", now.Format(time.RFC3339))
	fmt.Fprintf(&b, "# control_url: %s\n", controlURL)
	fmt.Fprintf(&b, "# total source entries: %d\n", pi.SourceCount)
	fmt.Fprintf(&b, "# total sink entries: %d\n", pi.SinkCount)
	fmt.Fprintf(&b, "# audio sink entries: %d\n", pi.AudioSinkCount)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "==============================================================================")
	fmt.Fprintln(&b, "SOURCE")
	fmt.Fprintln(&b, "==============================================================================")
	for _, e := range splitProtocolInfo(source) {
		fmt.Fprintln(&b, e)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "==============================================================================")
	fmt.Fprintln(&b, "SINK")
	fmt.Fprintln(&b, "==============================================================================")
	for _, e := range splitProtocolInfo(sink) {
		fmt.Fprintln(&b, e)
	}
	return b.Bytes()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
