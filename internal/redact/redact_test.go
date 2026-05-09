package redact

import (
	"reflect"
	"strings"
	"testing"

	"gitlab.com/dunn.dev/tutti/internal/schema"
)

// sampleTranscript mimics narjo-eversolo-upnp/proof_of_chain.log style:
// SOAP control URL on the device, Subsonic stream URL with u/t/s
// params, plus a synthetic Authorization header line that a future
// HTTP transcript dump might emit.
const sampleTranscript = `[avt] control: http://10.0.0.42:1054/AVTransport/800a805eef11-dmr/control.xml
[avt] stream:  https://navidrome.example.net/rest/stream?id=ihfBXWSqnMasIo1gEuckd1&u=alice&t=abcdef0123456789&s=cafebabe&v=1.16.1&c=narjo-debug&format=raw
[avt] art:     https://navidrome.example.net/rest/getCoverArt?id=mf-ihfBXWSqnMasIo1gEuckd1&u=alice&t=abcdef0123456789&s=cafebabe&v=1.16.1&c=narjo-debug&size=300
> GET /rest/ping HTTP/1.1
> Host: navidrome.example.net
> Authorization: Bearer secret-token-xyz
> X-Client-IP: 192.168.1.7
`

func TestStringRedactsTranscript(t *testing.T) {
	r := New()
	r.LearnDeviceIP("10.0.0.42")
	r.LearnClientIP("192.168.1.7")

	out := r.String(sampleTranscript)

	for _, leak := range []string{
		"alice",                // Subsonic u=
		"abcdef0123456789",     // Subsonic t=
		"cafebabe",             // Subsonic s=
		"Bearer secret-token-xyz",
		"secret-token-xyz",
		"10.0.0.42",
		"192.168.1.7",
	} {
		if strings.Contains(out, leak) {
			t.Errorf("output still contains %q\n--- output ---\n%s", leak, out)
		}
	}

	for _, want := range []string{
		"u=" + placeholderUser,
		"t=" + placeholderToken,
		"s=" + placeholderSalt,
		"Authorization: " + placeholderToken,
		placeholderDeviceIP,
		placeholderClientIP,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing expected %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestAppliedCategories(t *testing.T) {
	r := New()
	r.LearnDeviceIP("10.0.0.42")
	r.LearnClientIP("192.168.1.7")
	_ = r.String(sampleTranscript)

	got := r.Applied()
	want := []schema.Redaction{
		schema.RedactionAuthTokens,
		schema.RedactionClientIP,
		schema.RedactionDeviceIP,
		schema.RedactionSalt,
		schema.RedactionUsername,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Applied() = %v, want %v", got, want)
	}
}

func TestRFC1918AutoDetect(t *testing.T) {
	// IP not learned: redactor must still scrub via RFC1918 detection
	// and account it under device-ip.
	r := New()
	in := "control URL is http://172.20.5.6:1400/foo and peer is 192.168.99.1"
	out := r.String(in)
	if strings.Contains(out, "172.20.5.6") {
		t.Errorf("172.20.5.6 not scrubbed: %q", out)
	}
	if strings.Contains(out, "192.168.99.1") {
		t.Errorf("192.168.99.1 not scrubbed: %q", out)
	}
	if !strings.Contains(out, placeholderRFC1918IP) {
		t.Errorf("expected %s in output, got %q", placeholderRFC1918IP, out)
	}

	got := r.Applied()
	if len(got) != 1 || got[0] != schema.RedactionDeviceIP {
		t.Errorf("Applied() = %v, want [device-ip]", got)
	}
}

func TestPublicIPNotScrubbed(t *testing.T) {
	// 8.8.8.8 is not RFC1918; we must not touch it.
	r := New()
	in := "upstream resolver is 8.8.8.8"
	out := r.String(in)
	if out != in {
		t.Errorf("public IP modified: in=%q out=%q", in, out)
	}
	if got := r.Applied(); len(got) != 0 {
		t.Errorf("Applied() = %v, want []", got)
	}
}

func TestBenignStringRoundTrips(t *testing.T) {
	r := New()
	in := "no secrets here, just a friendly_name and a UDN: uuid:abcdef"
	out := r.String(in)
	if out != in {
		t.Errorf("benign string changed: in=%q out=%q", in, out)
	}
	if got := r.Applied(); len(got) != 0 {
		t.Errorf("Applied() should be empty, got %v", got)
	}
}

func TestBytesMatchesString(t *testing.T) {
	r1 := New()
	r1.LearnDeviceIP("10.0.0.42")
	gotS := r1.String(sampleTranscript)

	r2 := New()
	r2.LearnDeviceIP("10.0.0.42")
	gotB := r2.Bytes([]byte(sampleTranscript))

	if string(gotB) != gotS {
		t.Errorf("Bytes and String diverge:\n--- bytes ---\n%s\n--- string ---\n%s", gotB, gotS)
	}
}

func TestBytesEmpty(t *testing.T) {
	r := New()
	out := r.Bytes(nil)
	if out == nil {
		t.Errorf("Bytes(nil) returned nil; want non-nil empty slice")
	}
	if len(out) != 0 {
		t.Errorf("Bytes(nil) returned %d bytes; want 0", len(out))
	}
}

func TestAuthorizationHeaderCaseInsensitive(t *testing.T) {
	r := New()
	in := "AUTHORIZATION: Bearer xyz\nauthorization: Basic abcd\n"
	out := r.String(in)
	if strings.Contains(out, "xyz") || strings.Contains(out, "abcd") {
		t.Errorf("auth header values not scrubbed: %q", out)
	}
	if !strings.Contains(out, "AUTHORIZATION: "+placeholderToken) {
		t.Errorf("expected uppercase header preserved: %q", out)
	}
	if !strings.Contains(out, "authorization: "+placeholderToken) {
		t.Errorf("expected lowercase header preserved: %q", out)
	}
}

func TestSubsonicParamsAtURLStart(t *testing.T) {
	// Param at start (?u=) and middle (&u=) must both be caught.
	r := New()
	in := "https://nav.example.com/rest/ping?u=bob&t=abc&s=def&extra=keep"
	out := r.String(in)
	if strings.Contains(out, "bob") || strings.Contains(out, "abc") || strings.Contains(out, "def") {
		t.Errorf("Subsonic params leaked: %q", out)
	}
	if !strings.Contains(out, "extra=keep") {
		t.Errorf("non-auth param 'extra=keep' was modified: %q", out)
	}
}

func TestLearnEmptyIPNoOp(t *testing.T) {
	r := New()
	r.LearnDeviceIP("")
	r.LearnClientIP("")
	in := "nothing sensitive"
	if out := r.String(in); out != in {
		t.Errorf("benign string changed after empty Learn: in=%q out=%q", in, out)
	}
	if got := r.Applied(); len(got) != 0 {
		t.Errorf("Applied() = %v, want []", got)
	}
}

func TestLearnedIPOutsideRFC1918(t *testing.T) {
	// Caller can declare a non-RFC1918 IP as a device IP; the explicit
	// learn must scrub it even though the auto-detector would not.
	r := New()
	r.LearnDeviceIP("203.0.113.5")
	in := "device at 203.0.113.5:1400"
	out := r.String(in)
	if strings.Contains(out, "203.0.113.5") {
		t.Errorf("learned device IP not scrubbed: %q", out)
	}
	if !strings.Contains(out, placeholderDeviceIP) {
		t.Errorf("expected %s placeholder: %q", placeholderDeviceIP, out)
	}
}
