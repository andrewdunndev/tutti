package descriptor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"gitlab.com/dunn.dev/tutti/internal/schema"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return b
}

func TestParseEversolo(t *testing.T) {
	raw := loadFixture(t, "eversolo.xml")
	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got, want := parsed.DeviceType, "urn:schemas-upnp-org:device:MediaRenderer:1"; got != want {
		t.Errorf("DeviceType = %q, want %q", got, want)
	}
	if got, want := parsed.FriendlyName, "DMP-A6"; got != want {
		t.Errorf("FriendlyName = %q, want %q", got, want)
	}
	if got, want := parsed.Manufacturer, "EVERSOLO"; got != want {
		t.Errorf("Manufacturer = %q, want %q", got, want)
	}
	if got, want := parsed.ModelName, "AV Renderer Device"; got != want {
		t.Errorf("ModelName = %q, want %q", got, want)
	}
	if got, want := parsed.UDN, "uuid:800a805eef11-dmr"; got != want {
		t.Errorf("UDN = %q, want %q", got, want)
	}
	if got, want := parsed.XDlnaDoc, "DMR-1.50"; got != want {
		t.Errorf("XDlnaDoc = %q, want %q", got, want)
	}

	if len(parsed.Services) != 3 {
		t.Fatalf("Services count = %d, want 3", len(parsed.Services))
	}

	wantTypes := map[string]bool{
		"urn:schemas-upnp-org:service:AVTransport:1":       false,
		"urn:schemas-upnp-org:service:ConnectionManager:1": false,
		"urn:schemas-upnp-org:service:RenderingControl:1":  false,
	}
	for _, svc := range parsed.Services {
		if _, ok := wantTypes[svc.ServiceType]; ok {
			wantTypes[svc.ServiceType] = true
		}
		if svc.ScpdURL == "" {
			t.Errorf("service %q: empty ScpdURL", svc.ServiceType)
		}
		if svc.ControlURL == "" {
			t.Errorf("service %q: empty ControlURL", svc.ServiceType)
		}
		if svc.EventSubURL == "" {
			t.Errorf("service %q: empty EventSubURL", svc.ServiceType)
		}
	}
	for typ, seen := range wantTypes {
		if !seen {
			t.Errorf("missing service type %q", typ)
		}
	}

	// Bare Parse should not resolve URLs: they remain host-relative.
	for _, svc := range parsed.Services {
		if !strings.HasPrefix(svc.ControlURL, "/") {
			t.Errorf("Parse should leave relative ControlURL unresolved, got %q", svc.ControlURL)
		}
	}

	if len(parsed.Icons) != 1 {
		t.Fatalf("Icons count = %d, want 1", len(parsed.Icons))
	}
	ic := parsed.Icons[0]
	if ic.MIME != "image/png" || ic.Width != 64 || ic.Height != 64 || ic.Depth != 16 {
		t.Errorf("icon = %+v, want image/png 64x64 depth 16", ic)
	}
}

func TestSelectMediaRenderer(t *testing.T) {
	raw := loadFixture(t, "eversolo.xml")
	parsed, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	hit := SelectMediaRenderer(parsed)
	if hit == nil {
		t.Fatal("SelectMediaRenderer returned nil for a MediaRenderer descriptor")
	}
	if hit.FriendlyName != "DMP-A6" {
		t.Errorf("hit.FriendlyName = %q, want DMP-A6", hit.FriendlyName)
	}

	// Negative case: synthesize a non-renderer root with an embedded renderer.
	wrapper := schema.DescriptorParsed{
		DeviceType:   "urn:schemas-upnp-org:device:Basic:1",
		FriendlyName: "Wrapper",
		UDN:          "uuid:wrapper",
		Services:     []schema.DescriptorService{},
		EmbeddedDevices: []schema.DescriptorParsed{
			parsed,
		},
	}
	hit2 := SelectMediaRenderer(wrapper)
	if hit2 == nil {
		t.Fatal("SelectMediaRenderer should descend into embedded devices")
	}
	if hit2.FriendlyName != "DMP-A6" {
		t.Errorf("embedded hit.FriendlyName = %q, want DMP-A6", hit2.FriendlyName)
	}

	// Absent case.
	none := schema.DescriptorParsed{
		DeviceType:   "urn:schemas-upnp-org:device:Basic:1",
		FriendlyName: "Plain",
		UDN:          "uuid:plain",
		Services:     []schema.DescriptorService{},
	}
	if SelectMediaRenderer(none) != nil {
		t.Error("SelectMediaRenderer should return nil when no MediaRenderer present")
	}
}

func TestFetchResolvesURLs(t *testing.T) {
	raw := loadFixture(t, "eversolo.xml")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	loc := srv.URL + "/description.xml"
	parsed, gotRaw, err := Fetch(context.Background(), loc)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(gotRaw) != len(raw) {
		t.Errorf("raw bytes length = %d, want %d", len(gotRaw), len(raw))
	}

	for _, svc := range parsed.Services {
		if !strings.HasPrefix(svc.ControlURL, srv.URL+"/") {
			t.Errorf("ControlURL not resolved against base: %q", svc.ControlURL)
		}
		if !strings.HasPrefix(svc.ScpdURL, srv.URL+"/") {
			t.Errorf("ScpdURL not resolved against base: %q", svc.ScpdURL)
		}
		if !strings.HasPrefix(svc.EventSubURL, srv.URL+"/") {
			t.Errorf("EventSubURL not resolved against base: %q", svc.EventSubURL)
		}
	}
	for _, ic := range parsed.Icons {
		if !strings.HasPrefix(ic.URL, srv.URL+"/") {
			t.Errorf("icon URL not resolved against base: %q", ic.URL)
		}
	}
	if parsed.PresentationURL != srv.URL+"/" {
		t.Errorf("PresentationURL = %q, want %q", parsed.PresentationURL, srv.URL+"/")
	}
}

func TestFetchRetriesThenSucceeds(t *testing.T) {
	raw := loadFixture(t, "eversolo.xml")
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	parsed, _, err := Fetch(context.Background(), srv.URL+"/description.xml")
	if err != nil {
		t.Fatalf("Fetch after retries: %v", err)
	}
	if parsed.FriendlyName != "DMP-A6" {
		t.Errorf("FriendlyName = %q, want DMP-A6", parsed.FriendlyName)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("hits = %d, want 3 (two failures then a success)", got)
	}
}

func TestFetchExhaustsRetries(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, _, err := Fetch(context.Background(), srv.URL+"/description.xml")
	if err == nil {
		t.Fatal("Fetch should fail after exhausting retries")
	}
	if got := atomic.LoadInt32(&hits); got != 4 {
		t.Errorf("attempt count = %d, want 4 (1 + 3 retries)", got)
	}
}

func TestFetchEmptyURL(t *testing.T) {
	_, _, err := Fetch(context.Background(), "")
	if err == nil {
		t.Fatal("Fetch(\"\") should error")
	}
}
