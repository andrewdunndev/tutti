package drive

import (
	"encoding/xml"
	"strings"
	"testing"

	"gitlab.com/dunn.dev/tutti/internal/schema"
)

// TestBuildDIDLLite_Structure feeds a known input through BuildDIDLLite
// and verifies the result parses as XML and contains the title,
// artist, album, and res protocolInfo we expect. We deliberately do
// not assert byte-for-byte equality: the element ordering inside
// <item> is not load-bearing for any UPnP renderer, only the field
// presence is.
func TestBuildDIDLLite_Structure(t *testing.T) {
	meta := schema.DIDLMetadata{
		Title:  "tutti probe: flac-96k-24",
		Artist: "tutti.dunn.dev",
		Album:  "Audio Renderer Probes (didl)",
	}
	got := BuildDIDLLite(meta, "audio/flac", "http://192.0.2.1:1234/tones/FLAC_96k_24.flac")

	// Must be parseable XML.
	var probe struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal([]byte(got), &probe); err != nil {
		t.Fatalf("unmarshal: %v\npayload: %s", err, got)
	}
	if probe.XMLName.Local != "DIDL-Lite" {
		t.Errorf("root element = %q, want DIDL-Lite", probe.XMLName.Local)
	}

	for _, want := range []string{
		"tutti probe: flac-96k-24",
		"tutti.dunn.dev",
		"Audio Renderer Probes (didl)",
		`protocolInfo="http-get:*:audio/flac:*"`,
		"http://192.0.2.1:1234/tones/FLAC_96k_24.flac",
		"object.item.audioItem.musicTrack",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in DIDL-Lite output:\n%s", want, got)
		}
	}
	// dc:creator is the dual we emit so DLNA-tier renderers can find
	// the artist; if it ever stops being emitted, that decision needs
	// to be reconsidered explicitly.
	if !strings.Contains(got, "<dc:creator>tutti.dunn.dev</dc:creator>") {
		t.Errorf("expected dc:creator dual for artist, got:\n%s", got)
	}
}

// TestBuildDIDLLite_OmitsArtURIWhenEmpty: we do not have a cover-art
// surface yet, and emitting an empty <upnp:albumArtURI/> would (a)
// clutter transcripts and (b) make some renderers report a failed
// art fetch. The element must be absent when ArtURL is "".
func TestBuildDIDLLite_OmitsArtURIWhenEmpty(t *testing.T) {
	meta := schema.DIDLMetadata{Title: "tutti probe: x", Artist: "a", Album: "b"}
	got := BuildDIDLLite(meta, "audio/flac", "http://h/x")
	if strings.Contains(got, "albumArtURI") {
		t.Errorf("albumArtURI should be omitted when empty:\n%s", got)
	}
}

// TestBuildDIDLLite_EscapesXMLSpecials guards against ampersands,
// angle brackets, and quotes in URLs (which are common with renderers
// behind Subsonic-style auth tokens).
func TestBuildDIDLLite_EscapesXMLSpecials(t *testing.T) {
	meta := schema.DIDLMetadata{
		Title:  "weird & sharp <title>",
		Artist: "a",
		Album:  "b",
		ArtURL: "http://h/art?u=x&t=y",
	}
	got := BuildDIDLLite(meta, "audio/flac", "http://h/x?a=1&b=2")
	// Raw special characters must not appear in element bodies.
	if strings.Contains(got, "weird & sharp <title>") {
		t.Errorf("title not escaped:\n%s", got)
	}
	if strings.Contains(got, "u=x&t=y") {
		t.Errorf("art URL ampersand not escaped:\n%s", got)
	}
	if strings.Contains(got, "?a=1&b=2") {
		t.Errorf("res URL ampersand not escaped:\n%s", got)
	}
}

// TestParseSimpleResponse_GetPositionInfo exercises the SOAP response
// parser against a body shaped like real GetPositionInfo output. The
// inner DIDL-Lite is XML-escaped inside TrackMetaData (the wire shape
// devices actually emit) and must come back unescaped in the field
// map.
func TestParseSimpleResponse_GetPositionInfo(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
 <s:Body>
  <u:GetPositionInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
   <Track>1</Track>
   <TrackDuration>0:02:47</TrackDuration>
   <TrackMetaData>&lt;DIDL-Lite xmlns=&quot;urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/&quot; xmlns:dc=&quot;http://purl.org/dc/elements/1.1/&quot; xmlns:upnp=&quot;urn:schemas-upnp-org:metadata-1-0/upnp/&quot;&gt;&lt;item id=&quot;tutti-probe-1&quot; parentID=&quot;0&quot; restricted=&quot;1&quot;&gt;&lt;dc:title&gt;tutti probe: flac-96k-24&lt;/dc:title&gt;&lt;upnp:artist&gt;tutti.dunn.dev&lt;/upnp:artist&gt;&lt;/item&gt;&lt;/DIDL-Lite&gt;</TrackMetaData>
   <TrackURI>http://192.0.2.1:1234/tones/FLAC_96k_24.flac</TrackURI>
   <RelTime>0:00:01</RelTime>
   <AbsTime>0:00:01</AbsTime>
   <RelCount>0</RelCount>
   <AbsCount>0</AbsCount>
  </u:GetPositionInfoResponse>
 </s:Body>
</s:Envelope>`)

	fields, err := parseSimpleResponse(body)
	if err != nil {
		t.Fatalf("parseSimpleResponse: %v", err)
	}
	if got := fields["Track"]; got != "1" {
		t.Errorf("Track = %q, want 1", got)
	}
	if got := fields["TrackURI"]; got != "http://192.0.2.1:1234/tones/FLAC_96k_24.flac" {
		t.Errorf("TrackURI = %q", got)
	}
	if got := fields["TrackDuration"]; got != "0:02:47" {
		t.Errorf("TrackDuration = %q, want 0:02:47", got)
	}
	meta := fields["TrackMetaData"]
	if !strings.Contains(meta, "<DIDL-Lite") || !strings.Contains(meta, "tutti probe: flac-96k-24") {
		t.Errorf("TrackMetaData not unescaped or missing title:\n%s", meta)
	}
}

// TestParseSimpleResponse_GetTransportInfo: the smaller transport-info
// response is what we use to drive the transitions list. State must
// arrive in the expected key.
func TestParseSimpleResponse_GetTransportInfo(t *testing.T) {
	body := []byte(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
 <s:Body>
  <u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
   <CurrentTransportState>PLAYING</CurrentTransportState>
   <CurrentTransportStatus>OK</CurrentTransportStatus>
   <CurrentSpeed>1</CurrentSpeed>
  </u:GetTransportInfoResponse>
 </s:Body>
</s:Envelope>`)
	fields, err := parseSimpleResponse(body)
	if err != nil {
		t.Fatalf("parseSimpleResponse: %v", err)
	}
	if got := fields["CurrentTransportState"]; got != "PLAYING" {
		t.Errorf("CurrentTransportState = %q, want PLAYING", got)
	}
	if got := fields["CurrentSpeed"]; got != "1" {
		t.Errorf("CurrentSpeed = %q, want 1", got)
	}
}

// TestParseSimpleResponse_Fault: a SOAP fault must surface as an error
// so the drive code records result=errored with the fault text.
func TestParseSimpleResponse_Fault(t *testing.T) {
	body := []byte(`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
 <s:Body>
  <s:Fault>
   <faultcode>s:Client</faultcode>
   <faultstring>UPnPError</faultstring>
   <detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>701</errorCode><errorDescription>Transition not available</errorDescription></UPnPError></detail>
  </s:Fault>
 </s:Body>
</s:Envelope>`)
	_, err := parseSimpleResponse(body)
	if err == nil {
		t.Fatalf("expected fault error, got nil")
	}
	if !strings.Contains(err.Error(), "SOAP fault") {
		t.Errorf("error = %v, want it to mention SOAP fault", err)
	}
}

// TestParseEchoedDIDL pulls Title/Artist/Album out of a minimal
// DIDL-Lite payload of the kind devices echo back.
func TestParseEchoedDIDL(t *testing.T) {
	payload := `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"><item id="tutti-probe-1" parentID="0" restricted="1"><dc:title>tutti probe: flac-96k-24</dc:title><upnp:artist>tutti.dunn.dev</upnp:artist><upnp:album>Audio Renderer Probes (didl)</upnp:album></item></DIDL-Lite>`
	got := parseEchoedDIDL(payload)
	if got.Title != "tutti probe: flac-96k-24" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Artist != "tutti.dunn.dev" {
		t.Errorf("Artist = %q", got.Artist)
	}
	if got.Album != "Audio Renderer Probes (didl)" {
		t.Errorf("Album = %q", got.Album)
	}
}

// TestParseEchoedDIDL_FallbackCreator: dc:creator is the artist field
// older renderers prefer to round-trip; ensure we accept it.
func TestParseEchoedDIDL_FallbackCreator(t *testing.T) {
	payload := `<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/"><item><dc:title>x</dc:title><dc:creator>fallback artist</dc:creator></item></DIDL-Lite>`
	got := parseEchoedDIDL(payload)
	if got.Artist != "fallback artist" {
		t.Errorf("Artist = %q, want fallback artist", got.Artist)
	}
}

// TestClassifyResult walks the documented dominance order: PLAYING
// beats TRANSITIONING beats STOPPED, and an empty list -> errored.
func TestClassifyResult(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want schema.DriveResult
	}{
		{"playing wins", []string{"STOPPED", "TRANSITIONING", "PLAYING"}, schema.DriveResultPlaying},
		{"transitioning over stopped", []string{"STOPPED", "TRANSITIONING"}, schema.DriveResultTransitioning},
		{"stopped only", []string{"STOPPED"}, schema.DriveResultStopped},
		{"empty -> errored", nil, schema.DriveResultErrored},
		{"unknown only -> errored", []string{"NO_MEDIA_PRESENT"}, schema.DriveResultErrored},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyResult(tc.in); got != tc.want {
				t.Errorf("classifyResult(%v) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

// TestDeriveMetadata exercises the (sent, echoed, sawDIDL, sawEmbedded)
// truth table.
func TestDeriveMetadata(t *testing.T) {
	sent := schema.DIDLMetadata{Title: "tutti probe: x"}
	matched := &schema.DIDLMetadata{Title: "tutti probe: x"}
	mismatched := &schema.DIDLMetadata{Title: "tutti probe: y"}
	embedded := &schema.DIDLMetadata{Title: "tutti embed: x"}

	cases := []struct {
		name      string
		echoed    *schema.DIDLMetadata
		sawDIDL   bool
		sawEmbed  bool
		wantSrc   schema.MetadataSource
		wantTrip  schema.MetadataRoundTrip
	}{
		{"none echoed", nil, false, false, schema.MetadataSourceNone, schema.MetadataRoundTripAbsent},
		{"didl matched", matched, true, false, schema.MetadataSourceDIDLLite, schema.MetadataRoundTripMatched},
		{"didl mismatched", mismatched, true, false, schema.MetadataSourceDIDLLite, schema.MetadataRoundTripMismatched},
		{"embedded only", embedded, false, true, schema.MetadataSourceEmbedded, schema.MetadataRoundTripMismatched},
		{"both -> mixed", embedded, true, true, schema.MetadataSourceMixed, schema.MetadataRoundTripMismatched},
		{"echoed but unrecognised", &schema.DIDLMetadata{Title: "Some Other Title"}, false, false, schema.MetadataSourceNone, schema.MetadataRoundTripAbsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotSrc, gotTrip := deriveMetadata(sent, tc.echoed, tc.sawDIDL, tc.sawEmbed)
			if gotSrc != tc.wantSrc {
				t.Errorf("source = %s, want %s", gotSrc, tc.wantSrc)
			}
			if gotTrip != tc.wantTrip {
				t.Errorf("round_trip = %s, want %s", gotTrip, tc.wantTrip)
			}
		})
	}
}

// TestAppendUnique: the transitions list dedupes adjacent duplicates
// but preserves a STOPPED -> PLAYING -> STOPPED loop.
func TestAppendUnique(t *testing.T) {
	got := []string{}
	for _, s := range []string{"STOPPED", "TRANSITIONING", "TRANSITIONING", "PLAYING", "PLAYING", "STOPPED"} {
		got = appendUnique(got, s)
	}
	want := []string{"STOPPED", "TRANSITIONING", "PLAYING", "STOPPED"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

// TestRun_NoTones: with an empty matrix Run reports Performed=false
// and a SkippedReason rather than producing an empty Runs slice that
// would fail manifest validation (drive_test requires runs when
// performed=true).
func TestRun_NoTones(t *testing.T) {
	got, err := Run(nil, Options{
		AVTransportControlURL: "http://example/control",
		Tones:                 nil,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Performed {
		t.Errorf("Performed = true, want false when no tones")
	}
	if got.SkippedReason == "" {
		t.Errorf("SkippedReason should not be empty")
	}
}

// TestRun_NoControlURL: same path, different reason.
func TestRun_NoControlURL(t *testing.T) {
	got, err := Run(nil, Options{
		AVTransportControlURL: "",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Performed {
		t.Errorf("Performed = true, want false")
	}
}
