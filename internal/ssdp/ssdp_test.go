package ssdp

import (
	"net"
	"strings"
	"testing"
)

// fakeAddr lets us exercise Source without opening a real socket.
type fakeAddr struct{ s string }

func (f fakeAddr) Network() string { return "udp" }
func (f fakeAddr) String() string  { return f.s }

// A representative well-formed reply, modelled on the Eversolo sample
// in narjo-eversolo-upnp/ssdp_dump.txt.
const sampleEversolo = "HTTP/1.1 200 OK\r\n" +
	"CACHE-CONTROL: max-age=1800\r\n" +
	"DATE: Thu, 08 May 2025 23:55:21 GMT\r\n" +
	"EXT:\r\n" +
	"LOCATION: http://192.0.2.42:1054/description.xml\r\n" +
	"SERVER: UPnP/1.0 DLNADOC/1.50 Platinum/1.0.5.13\r\n" +
	"ST: urn:schemas-upnp-org:device:MediaRenderer:1\r\n" +
	"USN: uuid:800a805eef11-dmr::urn:schemas-upnp-org:device:MediaRenderer:1\r\n" +
	"BOOTID.UPNP.ORG: 1778188637\r\n" +
	"CONFIGID.UPNP.ORG: 9435121\r\n" +
	"\r\n"

func TestParseResponse_Canonical(t *testing.T) {
	src := fakeAddr{s: "192.0.2.42:45803"}
	r, err := ParseResponse([]byte(sampleEversolo), src)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}

	cases := map[string]struct {
		got, want string
	}{
		"USN":          {r.USN, "uuid:800a805eef11-dmr::urn:schemas-upnp-org:device:MediaRenderer:1"},
		"ST":           {r.ST, "urn:schemas-upnp-org:device:MediaRenderer:1"},
		"Location":     {r.Location, "http://192.0.2.42:1054/description.xml"},
		"Server":       {r.Server, "UPnP/1.0 DLNADOC/1.50 Platinum/1.0.5.13"},
		"BootID":       {r.BootID, "1778188637"},
		"ConfigID":     {r.ConfigID, "9435121"},
		"CacheControl": {r.CacheControl, "max-age=1800"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}

	if r.Source == nil || r.Source.String() != "192.0.2.42:45803" {
		t.Errorf("Source = %v, want 192.0.2.42:45803", r.Source)
	}
	if r.RawHTTP != sampleEversolo {
		t.Errorf("RawHTTP not preserved verbatim")
	}
	// Header keys are uppercased on the way in.
	if v, ok := r.Headers["BOOTID.UPNP.ORG"]; !ok || v != "1778188637" {
		t.Errorf("Headers[BOOTID.UPNP.ORG] = %q, ok=%v", v, ok)
	}
	if _, ok := r.Headers["bootid.upnp.org"]; ok {
		t.Errorf("Headers should not contain lowercase keys")
	}
}

// Lenient parser: missing reason phrase, mixed CRLF/LF, lowercase
// header names, trailing junk. All of these have been observed on real
// devices that tutti is supposed to characterize, not reject.
func TestParseResponse_Lenient(t *testing.T) {
	raw := "HTTP/1.1 200\n" +
		"location: http://192.0.2.7:8080/desc.xml\n" +
		"st: upnp:rootdevice\r\n" +
		"USN:   uuid:abc::upnp:rootdevice  \r\n" +
		"Server: Linux/3.4 UPnP/1.0\n" +
		"\r\n"
	r, err := ParseResponse([]byte(raw), fakeAddr{s: "192.0.2.7:1900"})
	if err != nil {
		t.Fatalf("ParseResponse(lenient): %v", err)
	}
	if r.Location != "http://192.0.2.7:8080/desc.xml" {
		t.Errorf("Location = %q", r.Location)
	}
	if r.ST != "upnp:rootdevice" {
		t.Errorf("ST = %q", r.ST)
	}
	if r.USN != "uuid:abc::upnp:rootdevice" {
		t.Errorf("USN = %q (whitespace not trimmed)", r.USN)
	}
	if r.Server != "Linux/3.4 UPnP/1.0" {
		t.Errorf("Server = %q", r.Server)
	}
}

func TestParseResponse_RejectsNonHTTP(t *testing.T) {
	for _, raw := range []string{
		"",
		"hello world\r\n\r\n",
		"GET / HTTP/1.1\r\n\r\n",
	} {
		if _, err := ParseResponse([]byte(raw), nil); err == nil {
			t.Errorf("expected error for %q", raw)
		}
	}
}

func TestGroupByUSN_DedupesAndBuckets(t *testing.T) {
	rs := []Response{
		{USN: "uuid:A", ST: "st1", Location: "http://x/desc"},
		{USN: "uuid:A", ST: "st1", Location: "http://x/desc"}, // dup
		{USN: "uuid:A", ST: "st2", Location: "http://x/desc"},
		{USN: "uuid:B", ST: "st1", Location: "http://y/desc"},
		{USN: "", ST: "st1", Location: "http://z/desc"},
	}
	g := GroupByUSN(rs)
	if len(g["uuid:A"]) != 2 {
		t.Errorf("uuid:A bucket = %d, want 2 (dedup of dup)", len(g["uuid:A"]))
	}
	if len(g["uuid:B"]) != 1 {
		t.Errorf("uuid:B bucket = %d, want 1", len(g["uuid:B"]))
	}
	if _, ok := g["<no-usn>"]; !ok {
		t.Errorf("missing-USN responses should bucket under <no-usn>")
	}
}

func TestUniqueLocations_OrderPreserved(t *testing.T) {
	rs := []Response{
		{Location: "http://a"},
		{Location: "http://b"},
		{Location: "http://a"},
		{Location: ""},
		{Location: "http://c"},
	}
	got := UniqueLocations(rs)
	want := []string{"http://a", "http://b", "http://c"}
	if len(got) != len(want) {
		t.Fatalf("UniqueLocations = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("UniqueLocations[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFormatDump_HeaderAndSummary(t *testing.T) {
	src := fakeAddr{s: "192.0.2.42:45803"}
	r, err := ParseResponse([]byte(sampleEversolo), src)
	if err != nil {
		t.Fatal(err)
	}
	dump := formatDump(DefaultSTList, DefaultMX, []Response{r})

	for _, want := range []string{
		"[ssdp] M-SEARCH -> 239.255.255.250:1900, MX=3",
		"[ssdp]   sent ST: ssdp:all",
		"[ssdp]   sent ST: urn:schemas-upnp-org:device:MediaRenderer:1",
		"[ssdp] 1 raw response(s), 1 unique USN(s), 1 unique LOCATION(s)",
		"LOCATIONS",
		"DEVICES (grouped by USN)",
		"http://192.0.2.42:1054/description.xml",
		"FROM:     192.0.2.42:45803",
		"SERVER:   UPnP/1.0 DLNADOC/1.50 Platinum/1.0.5.13",
		"BOOTID.UPNP.ORG: 1778188637",
	} {
		if !strings.Contains(dump, want) {
			t.Errorf("dump missing substring %q\n--- dump ---\n%s", want, dump)
		}
	}
}

func TestBuildMSearch_Wellformed(t *testing.T) {
	got := string(buildMSearch("upnp:rootdevice", 3))
	for _, want := range []string{
		"M-SEARCH * HTTP/1.1\r\n",
		"HOST: 239.255.255.250:1900\r\n",
		"MAN: \"ssdp:discover\"\r\n",
		"MX: 3\r\n",
		"ST: upnp:rootdevice\r\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("M-SEARCH missing %q\nfull:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, "\r\n\r\n") {
		t.Errorf("M-SEARCH must end with blank line, got %q", got[len(got)-4:])
	}
}

// Sanity: make sure ParseResponse doesn't choke on a minimal but valid
// reply with only the bare canonical fields.
func TestParseResponse_Minimal(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\nUSN: uuid:1\r\nST: ssdp:all\r\nLOCATION: http://x/d\r\n\r\n"
	r, err := ParseResponse([]byte(raw), nil)
	if err != nil {
		t.Fatalf("ParseResponse(minimal): %v", err)
	}
	if r.USN != "uuid:1" || r.ST != "ssdp:all" || r.Location != "http://x/d" {
		t.Errorf("minimal parse: %+v", r)
	}
	// Ensure compile-time assertion that net.Addr is the type used.
	var _ net.Addr = fakeAddr{}
}
