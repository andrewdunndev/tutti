package mdns

import (
	"context"
	"net"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/grandcat/zeroconf"
)

func TestEntryToService_Full(t *testing.T) {
	t.Parallel()

	e := &zeroconf.ServiceEntry{
		HostName: "Living-Room.local.",
		Port:     7000,
		Text:     []string{"deviceid=AA:BB:CC:DD:EE:FF", "model=AppleTV5,3", "flags=0x4", "noeq"},
		AddrIPv4: []net.IP{net.ParseIP("192.0.2.10")},
		AddrIPv6: []net.IP{net.ParseIP("fe80::1")},
	}
	e.Instance = "Living Room"
	e.Service = "_airplay._tcp."
	e.Domain = "local."

	got := entryToService("_airplay._tcp", e)

	if got.ServiceType != "_airplay._tcp" {
		t.Errorf("ServiceType: got %q, want %q", got.ServiceType, "_airplay._tcp")
	}
	if got.Instance != "Living Room._airplay._tcp.local" {
		t.Errorf("Instance: got %q, want %q", got.Instance, "Living Room._airplay._tcp.local")
	}
	if got.Hostname != "Living-Room.local" {
		t.Errorf("Hostname: got %q, want %q", got.Hostname, "Living-Room.local")
	}
	if got.Port != 7000 {
		t.Errorf("Port: got %d, want 7000", got.Port)
	}

	wantAddrs := []string{"192.0.2.10", "fe80::1"}
	if !reflect.DeepEqual(got.Addrs, wantAddrs) {
		t.Errorf("Addrs: got %v, want %v", got.Addrs, wantAddrs)
	}

	wantTXT := map[string]string{
		"deviceid": "AA:BB:CC:DD:EE:FF",
		"model":    "AppleTV5,3",
		"flags":    "0x4",
		"noeq":     "",
	}
	if !reflect.DeepEqual(got.TXT, wantTXT) {
		t.Errorf("TXT: got %v, want %v", got.TXT, wantTXT)
	}
}

func TestEntryToService_MinimalNoText(t *testing.T) {
	t.Parallel()

	e := &zeroconf.ServiceEntry{
		HostName: "speaker.local",
		Port:     5353,
	}
	e.Instance = "Speaker"
	e.Service = "_raop._tcp."
	e.Domain = "local."

	got := entryToService("_raop._tcp", e)

	if got.Instance != "Speaker._raop._tcp.local" {
		t.Errorf("Instance: got %q", got.Instance)
	}
	if got.Hostname != "speaker.local" {
		t.Errorf("Hostname: got %q", got.Hostname)
	}
	if got.Addrs != nil {
		t.Errorf("Addrs: got %v, want nil", got.Addrs)
	}
	if got.TXT != nil {
		t.Errorf("TXT: got %v, want nil", got.TXT)
	}
}

func TestEntryToService_InstanceFallback(t *testing.T) {
	t.Parallel()

	e := &zeroconf.ServiceEntry{HostName: "x.local."}
	e.Instance = "BareInstance"
	// Empty Service and Domain triggers the fallback to bare e.Instance.
	got := entryToService("_googlecast._tcp", e)
	if got.Instance != "BareInstance" {
		t.Errorf("Instance fallback: got %q, want %q", got.Instance, "BareInstance")
	}
	if got.Hostname != "x.local" {
		t.Errorf("Hostname trim: got %q", got.Hostname)
	}
}

func TestEntriesToServices_Order(t *testing.T) {
	t.Parallel()

	mk := func(name, host string, port int) *zeroconf.ServiceEntry {
		e := &zeroconf.ServiceEntry{HostName: host + ".", Port: port}
		e.Instance = name
		e.Service = "_googlecast._tcp."
		e.Domain = "local."
		return e
	}

	in := []*zeroconf.ServiceEntry{
		mk("Kitchen", "kitchen", 8009),
		nil, // skipped
		mk("Office", "office", 8009),
	}

	out := entriesToServices("_googlecast._tcp", in)

	if len(out) != 2 {
		t.Fatalf("len: got %d, want 2", len(out))
	}
	gotNames := []string{out[0].Instance, out[1].Instance}
	wantNames := []string{
		"Kitchen._googlecast._tcp.local",
		"Office._googlecast._tcp.local",
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Errorf("names: got %v, want %v", gotNames, wantNames)
	}
}

func TestParseTXT(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []string
		want map[string]string
	}{
		{"nil", nil, nil},
		{"empty entries skipped", []string{"", "k=v"}, map[string]string{"k": "v"}},
		{"no equals", []string{"flag"}, map[string]string{"flag": ""}},
		{"value contains equals", []string{"jwt=abc=def=="}, map[string]string{"jwt": "abc=def=="}},
		{"empty value", []string{"k="}, map[string]string{"k": ""}},
	}
	for _, tc := range cases {
		got := parseTXT(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCollectAddrs(t *testing.T) {
	t.Parallel()

	e := &zeroconf.ServiceEntry{
		AddrIPv4: []net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("10.0.0.2")},
		AddrIPv6: []net.IP{net.ParseIP("fe80::abcd")},
	}
	got := collectAddrs(e)
	want := []string{"10.0.0.1", "10.0.0.2", "fe80::abcd"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if collectAddrs(&zeroconf.ServiceEntry{}) != nil {
		t.Error("expected nil for empty entry")
	}
}

func TestDefaultServiceTypes_NoSMB(t *testing.T) {
	t.Parallel()

	want := []string{
		"_airplay._tcp",
		"_googlecast._tcp",
		"_raop._tcp",
		"_spotify-connect._tcp",
		"_sonos._tcp",
		"_daap._tcp",
		"_dacp._tcp",
	}
	if !reflect.DeepEqual(DefaultServiceTypes, want) {
		t.Errorf("DefaultServiceTypes: got %v, want %v", DefaultServiceTypes, want)
	}

	for _, st := range DefaultServiceTypes {
		if st == "_smb._tcp" {
			t.Error("DefaultServiceTypes must not contain _smb._tcp")
		}
	}
}

func TestBrowse_EmptyServiceTypes(t *testing.T) {
	t.Parallel()

	res, err := Browse(context.Background(), nil, nil, time.Millisecond)
	if err != nil {
		t.Fatalf("Browse: %v", err)
	}
	if res == nil {
		t.Fatal("Browse: nil result")
	}
	if len(res.Services) != 0 {
		t.Errorf("Browse: got %d services, want 0", len(res.Services))
	}
}

func TestResolveInterfaces_EmptyReturnsNil(t *testing.T) {
	t.Parallel()

	out, err := resolveInterfaces(nil)
	if err != nil {
		t.Fatalf("resolveInterfaces: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil for empty input, got %v", out)
	}
}

func TestResolveInterfaces_BogusName(t *testing.T) {
	t.Parallel()

	_, err := resolveInterfaces([]string{"definitely-not-a-real-iface-zzz"})
	if err == nil {
		t.Error("expected error for bogus interface")
	}
}

// Compile-time guard that the package's exported result type is what callers expect.
func TestResultShape(t *testing.T) {
	t.Parallel()

	r := &Result{}
	r.Services = append(r.Services, entryToService("_dacp._tcp", &zeroconf.ServiceEntry{
		HostName: "ctl.local.",
		Port:     3689,
	}))
	if len(r.Services) != 1 {
		t.Fatalf("Services len: got %d", len(r.Services))
	}
	// Sort smoke test: schema.MDNSService is a comparable-by-fields struct.
	sort.SliceStable(r.Services, func(i, j int) bool {
		return r.Services[i].ServiceType < r.Services[j].ServiceType
	})
}
