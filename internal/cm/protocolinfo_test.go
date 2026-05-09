package cm

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// synthSink builds a comma-separated Sink string out of a fixed set of
// entries that lets the test cover every detectable family at least once
// plus a handful of non-audio decoys.
func synthSink() string {
	entries := []string{
		"http-get:*:audio/flac:*",
		"http-get:*:audio/x-flac:*",
		"http-get:*:audio/x-m4a:*",            // alac
		"http-get:*:audio/mpeg:*",             // mp3
		"http-get:*:audio/mp3:*",              // mp3
		"http-get:*:audio/aac:*",              // aac
		"http-get:*:audio/x-aac:*",            // aac
		"http-get:*:audio/mp4:*",              // aac
		"http-get:*:audio/opus:*",             // opus
		"http-get:*:audio/ogg:*",              // vorbis
		"http-get:*:audio/vorbis:*",           // vorbis
		"http-get:*:audio/wav:*",              // wav
		"http-get:*:audio/x-wav:*",            // wav
		"http-get:*:audio/L16;rate=44100;channels=2:DLNA.ORG_PN=LPCM",
		"http-get:*:audio/unknown:*",
		"http-get:*:image/jpeg:*",
		"http-get:*:video/mp4:*",
	}
	return strings.Join(entries, ",")
}

func TestParseProtocolInfo_Synthetic(t *testing.T) {
	sink := synthSink()
	pi := ParseProtocolInfo("", sink)

	if pi.SourceCount != 0 {
		t.Errorf("SourceCount = %d, want 0", pi.SourceCount)
	}
	wantSink := 17
	if pi.SinkCount != wantSink {
		t.Errorf("SinkCount = %d, want %d", pi.SinkCount, wantSink)
	}
	// 17 total - 1 image - 1 video = 15 audio entries.
	if pi.AudioSinkCount != 15 {
		t.Errorf("AudioSinkCount = %d, want 15", pi.AudioSinkCount)
	}

	// Per spec: "flac" matches the substring "audio/flac" only, so
	// "audio/x-flac" does NOT count; "alac" matches "audio/alac" or
	// "audio/x-m4a"; "wav" matches "audio/wav", "audio/x-wav", or
	// "audio/x-pcm".
	checks := map[string]int{
		"flac":   1, // audio/flac only (audio/x-flac not in the family list)
		"alac":   1, // audio/x-m4a
		"mp3":    2, // audio/mpeg, audio/mp3
		"aac":    3, // audio/aac, audio/x-aac, audio/mp4
		"opus":   1,
		"vorbis": 2, // audio/vorbis, audio/ogg
		"wav":    2, // audio/wav, audio/x-wav
		"dsd":    0, // none in the synthetic
	}
	for fam, want := range checks {
		got := pi.FormatMatches[fam]
		if got == nil {
			t.Errorf("family %q: want detectable count %d, got nil", fam, want)
			continue
		}
		if *got != want {
			t.Errorf("family %q: got %d, want %d", fam, *got, want)
		}
	}

	// MQA is undetectable; it must be present and nil.
	mqa, ok := pi.FormatMatches["mqa"]
	if !ok {
		t.Fatalf("FormatMatches missing mqa key")
	}
	if mqa != nil {
		t.Errorf("FormatMatches[mqa] = %d, want nil", *mqa)
	}
}

// TestParseProtocolInfo_EversoloCorpus replays the audio sink list from
// the narjo dump and asserts the counts the docs claim (107 audio
// entries, 0 DSD-matching). We do not have the full 392-entry sink
// string in source form, so this verifies the audio detection plane
// against the dump's recorded "FULL AUDIO SINK LIST" section.
func TestParseProtocolInfo_EversoloCorpus(t *testing.T) {
	// 107 entries, copied from the narjo eversolo-getprotocolinfo.txt
	// "FULL AUDIO SINK LIST" section.
	entries := []string{
		"http-get:*:audio/ape:*",
		"http-get:*:audio/x-asf-pf:*",
		"http-get:*:audio/wma:*",
		"http-get:*:audio/x-wav:*",
		"http-get:*:audio/vorbis:*",
		"http-get:*:audio/x-scpls:*",
		"http-get:*:audio/x-ra:*",
		"http-get:*:audio/ra:*",
		"http-get:*:audio/x-realaudio:*",
		"http-get:*:audio/x-pn-realaudio-plugin:*",
		"http-get:*:audio/x-pn-realaudio:*",
		"http-get:*:audio/x-ms-wmv:*",
		"http-get:*:audio/x-ms-wma:*",
		"http-get:*:audio/x-ms-wax:*",
		"http-get:*:audio/x-mpeg-url:*",
		"http-get:*:audio/x-mpeg3:*",
		"http-get:*:audio/x-mp3:*",
		"http-get:*:audio/x-midi:*",
		"http-get:*:audio/x-m4a:*",
		"http-get:*:audio/x-flac:*",
		"http-get:*:audio/flac:*",
		"http-get:*:audio/x-ac3:*",
		"http-get:*:audio/ac3:*",
		"http-get:*:audio/x-aac:*",
		"http-get:*:audio/aac:*",
		"http-get:*:audio/x-aiff:*",
		"http-get:*:audio/wave:*",
		"http-get:*:audio/wav:*",
		"http-get:*:audio/vnd.rn-realaudio:*",
		"http-get:*:audio/vnd.qcelp:*",
		"http-get:*:audio/vnd.dlna.adts:*",
		"http-get:*:audio/unknown:*",
		"http-get:*:audio/playlist:*",
		"http-get:*:audio/x-ogg:*",
		"http-get:*:audio/ogg:*",
		"http-get:*:audio/mpg:*",
		"http-get:*:audio/mpeg-url:*",
		"http-get:*:audio/mpeg3:*",
		"http-get:*:audio/mpeg2:*",
		"http-get:*:audio/x-mpeg:*",
		"http-get:*:audio/mpeg:*",
		"http-get:*:audio/mp4a-latm:*",
		"http-get:*:audio/mp4:*",
		"http-get:*:audio/mp3:*",
		"http-get:*:audio/mp2:*",
		"http-get:*:audio/mp1:*",
		"http-get:*:audio/x-dts:*",
		"http-get:*:audio/midi:*",
		"http-get:*:audio/mid:*",
		"http-get:*:audio/m4a:*",
		"http-get:*:audio/x-atrac3:*",
		"http-get:*:audio/basic:*",
		"http-get:*:audio/asf:*",
		"http-get:*:audio/aiff:*",
		"http-get:*:audio/L16:*",
		"http-get:*:audio/L8:*",
		"http-get:*:audio/L16:DLNA.ORG_PN=LPCM",
		"http-get:*:audio/L16:DLNA.ORG_PN=LPCM_low",
		"http-get:*:audio/L16;rate=44100;channels=1:DLNA.ORG_PN=LPCM",
		"http-get:*:audio/L16;rate=44100;channels=2:DLNA.ORG_PN=LPCM",
		"http-get:*:audio/L16;rate=48000;channels=2:DLNA.ORG_PN=LPCM",
		"http-get:*:audio/mpeg:DLNA.ORG_PN=MP3",
		"http-get:*:audio/mpeg:DLNA.ORG_PN=MP3X",
		"http-get:*:audio/vnd.dlna.adts:DLNA.ORG_PN=AAC_ADTS",
		"http-get:*:audio/vnd.dlna.adts:DLNA.ORG_PN=AAC_ADTS_192",
		"http-get:*:audio/vnd.dlna.adts:DLNA.ORG_PN=AAC_ADTS_320",
		"http-get:*:audio/vnd.dlna.adts:DLNA.ORG_PN=AAC_MULT5_ADTS",
		"http-get:*:audio/mp4:DLNA.ORG_PN=AAC_ISO",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=AAC_ISO",
		"http-get:*:audio/mp4:DLNA.ORG_PN=AAC_ISO_192",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=AAC_ISO_192",
		"http-get:*:audio/mp4:DLNA.ORG_PN=AAC_ISO_320",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=AAC_ISO_320",
		"http-get:*:audio/mp4:DLNA.ORG_PN=AAC_MULT5_ISO",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=AAC_MULT5_ISO",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAACv2_L2",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAACv2_L2",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAACv2_L2_128",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAACv2_L2_128",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAACv2_L2_320",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAACv2_L2_320",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAACv2_L3",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAACv2_L3",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAACv2_L4",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAACv2_L4",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAACv2_MULT5",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAACv2_MULT5",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAAC_L2_ISO",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAAC_L2_ISO",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAAC_L2_ISO_128",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAAC_L2_ISO_128",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAAC_L2_ISO_320",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAAC_L2_ISO_320",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAAC_L3_ISO",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAAC_L3_ISO",
		"http-get:*:audio/mp4:DLNA.ORG_PN=HEAAC_MULT5_ISO",
		"http-get:*:audio/3gpp:DLNA.ORG_PN=HEAAC_MULT5_ISO",
		"http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMAFULL",
		"http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMABASE",
		"http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMAPRO",
		"http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMALSL",
		"http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMALSL_MULT5",
		"http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMDRM_WMABASE",
		"http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMDRM_WMAFULL",
		"http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMDRM_WMALSL",
		"http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMDRM_WMALSL_MULT5",
		"http-get:*:audio/x-ms-wma:DLNA.ORG_PN=WMDRM_WMAPRO",
	}
	if len(entries) != 107 {
		t.Fatalf("test corpus mis-copied: got %d entries, want 107", len(entries))
	}

	pi := ParseProtocolInfo("", strings.Join(entries, ","))

	if pi.SinkCount != 107 {
		t.Errorf("SinkCount = %d, want 107", pi.SinkCount)
	}
	if pi.AudioSinkCount != 107 {
		t.Errorf("AudioSinkCount = %d, want 107", pi.AudioSinkCount)
	}

	// THE Eversolo evidence: zero DSD-matching entries.
	dsd := pi.FormatMatches["dsd"]
	if dsd == nil {
		t.Fatalf("dsd: want detectable (zero-valued), got nil")
	}
	if *dsd != 0 {
		t.Errorf("dsd matches = %d, want 0 (Eversolo evidence)", *dsd)
	}

	// FLAC must hit exactly once: only "audio/flac:*" matches the family
	// substring "audio/flac"; "audio/x-flac:*" does not.
	if v := pi.FormatMatches["flac"]; v == nil || *v != 1 {
		t.Errorf("flac matches: got %v, want 1", v)
	}
}

func TestParseProtocolInfo_Empty(t *testing.T) {
	pi := ParseProtocolInfo("", "")
	if pi.SinkCount != 0 || pi.SourceCount != 0 || pi.AudioSinkCount != 0 {
		t.Errorf("empty input: counts = (%d,%d,%d), want all zero",
			pi.SourceCount, pi.SinkCount, pi.AudioSinkCount)
	}
	if _, ok := pi.FormatMatches["mqa"]; !ok {
		t.Errorf("FormatMatches missing mqa key on empty input")
	}
}

// TestRequestEnvelopeWellFormed ensures the SOAP body we POST parses
// as XML and matches the expected structure.
func TestRequestEnvelopeWellFormed(t *testing.T) {
	type envelope struct {
		XMLName xml.Name
		Body    struct {
			Inner struct {
				XMLName xml.Name
			} `xml:",any"`
		} `xml:"Body"`
	}
	var env envelope
	if err := xml.Unmarshal([]byte(soapEnvelopeRequest), &env); err != nil {
		t.Fatalf("envelope is not well-formed XML: %v", err)
	}
	if env.XMLName.Local != "Envelope" {
		t.Errorf("root = %q, want Envelope", env.XMLName.Local)
	}
	if env.Body.Inner.XMLName.Local != "GetProtocolInfo" {
		t.Errorf("body action = %q, want GetProtocolInfo", env.Body.Inner.XMLName.Local)
	}
}

// TestGetProtocolInfo_HTTPRoundTrip stands up an httptest server that
// behaves like a UPnP control endpoint and verifies the client sends the
// right headers and parses a Source+Sink response correctly.
func TestGetProtocolInfo_HTTPRoundTrip(t *testing.T) {
	wantSink := "http-get:*:audio/flac:*,http-get:*:audio/mpeg:*,http-get:*:image/jpeg:*"
	respBody := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
 <s:Body>
  <u:GetProtocolInfoResponse xmlns:u="urn:schemas-upnp-org:service:ConnectionManager:1">
   <Source></Source>
   <Sink>%s</Sink>
  </u:GetProtocolInfoResponse>
 </s:Body>
</s:Envelope>`, wantSink)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("SOAPAction"); got != soapAction {
			t.Errorf("SOAPAction = %q, want %q", got, soapAction)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(strings.ToLower(ct), "text/xml") {
			t.Errorf("Content-Type = %q, want text/xml", ct)
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "GetProtocolInfo") {
			t.Errorf("request body missing GetProtocolInfo: %s", b)
		}
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		_, _ = io.WriteString(w, respBody)
	}))
	defer srv.Close()

	pi, dump, err := GetProtocolInfo(context.Background(), srv.URL+"/control")
	if err != nil {
		t.Fatalf("GetProtocolInfo: %v", err)
	}
	if pi.SinkCount != 3 {
		t.Errorf("SinkCount = %d, want 3", pi.SinkCount)
	}
	if pi.AudioSinkCount != 2 {
		t.Errorf("AudioSinkCount = %d, want 2", pi.AudioSinkCount)
	}
	if v := pi.FormatMatches["flac"]; v == nil || *v != 1 {
		t.Errorf("flac = %v, want 1", v)
	}
	if v := pi.FormatMatches["mp3"]; v == nil || *v != 1 {
		t.Errorf("mp3 = %v, want 1", v)
	}
	if !strings.Contains(string(dump), "ConnectionManager:GetProtocolInfo") {
		t.Errorf("dump missing header banner")
	}
	if !strings.Contains(string(dump), "audio/flac") {
		t.Errorf("dump missing sink entries")
	}
	if !strings.Contains(string(dump), srv.URL+"/control") {
		t.Errorf("dump missing control_url")
	}
}

func TestGetProtocolInfo_SOAPFault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
 <s:Body>
  <s:Fault>
   <faultcode>s:Client</faultcode>
   <faultstring>UPnPError</faultstring>
  </s:Fault>
 </s:Body>
</s:Envelope>`)
	}))
	defer srv.Close()

	_, _, err := GetProtocolInfo(context.Background(), srv.URL+"/control")
	if err == nil {
		t.Fatalf("expected error on 500 response")
	}
}

func TestMimeField(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"http-get:*:audio/flac:*", "audio/flac"},
		{"http-get:*:audio/L16;rate=44100;channels=2:DLNA.ORG_PN=LPCM", "audio/l16;rate=44100;channels=2"},
		{"http-get:*:audio/mpeg:DLNA.ORG_PN=MP3", "audio/mpeg"},
		{"too:few", ""},
	}
	for _, c := range cases {
		got := mimeField(c.in)
		if got != c.want {
			t.Errorf("mimeField(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
