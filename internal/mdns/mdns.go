// Package mdns runs DNS-SD service browsing for the audio-renderer-relevant
// service types and converts the results into schema.MDNSService values.
//
// Implementation uses github.com/grandcat/zeroconf, the de-facto Go DNS-SD
// library. Each service-type browse runs in its own goroutine with a per-type
// timeout budget; the parent context cancels everything early.
package mdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"

	"gitlab.com/dunn.dev/tutti/internal/schema"
)

// DefaultServiceTypes is the audio-renderer-relevant default set of DNS-SD
// service types tutti browses for. Order is informational; the result set is
// flat.
var DefaultServiceTypes = []string{
	"_airplay._tcp",
	"_googlecast._tcp",
	"_raop._tcp",
	"_spotify-connect._tcp",
	"_sonos._tcp",
	"_daap._tcp", // iTunes Music Sharing
	"_dacp._tcp", // Apple DACP, AirPlay control
}

// Result is the aggregated outcome of a Browse call.
type Result struct {
	Services []schema.MDNSService
}

// Browse runs DNS-SD service browsing for each entry in serviceTypes. For
// every zeroconf.ServiceEntry returned, it converts to a schema.MDNSService
// and appends it to Result.Services.
//
// ifaces is an optional list of network-interface names to restrict browsing
// to; an empty slice means "all interfaces" (zeroconf default). ctx cancels
// the entire operation early. Each per-type browse is bounded by
// perTypeTimeout.
//
// A non-nil error is returned only if every service-type browse failed; a
// partial result with a nil error is returned if at least one type succeeded.
func Browse(ctx context.Context, ifaces []string, serviceTypes []string, perTypeTimeout time.Duration) (*Result, error) {
	res := &Result{}
	if len(serviceTypes) == 0 {
		return res, nil
	}

	netIfaces, err := resolveInterfaces(ifaces)
	if err != nil {
		return nil, fmt.Errorf("mdns: resolve interfaces: %w", err)
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		errs     = make([]error, 0, len(serviceTypes))
		successN int
	)

	for _, st := range serviceTypes {
		st := st
		wg.Add(1)
		go func() {
			defer wg.Done()
			services, browseErr := browseOne(ctx, netIfaces, st, perTypeTimeout)
			mu.Lock()
			defer mu.Unlock()
			if browseErr != nil {
				errs = append(errs, fmt.Errorf("%s: %w", st, browseErr))
				return
			}
			successN++
			res.Services = append(res.Services, services...)
		}()
	}

	wg.Wait()

	// All browses failed: surface a joined error. Otherwise return whatever
	// succeeded; partial-success is normal on home networks.
	if successN == 0 && len(errs) > 0 {
		return res, errors.Join(errs...)
	}
	return res, nil
}

// browseOne performs a single service-type browse, collects every entry that
// arrives within the per-type budget, and returns them converted to
// schema.MDNSService.
func browseOne(ctx context.Context, ifaces []net.Interface, serviceType string, perTypeTimeout time.Duration) ([]schema.MDNSService, error) {
	opts := []zeroconf.ClientOption{}
	if len(ifaces) > 0 {
		opts = append(opts, zeroconf.SelectIfaces(ifaces))
	}
	resolver, err := zeroconf.NewResolver(opts...)
	if err != nil {
		return nil, fmt.Errorf("new resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 32)

	browseCtx, cancel := context.WithTimeout(ctx, perTypeTimeout)
	defer cancel()

	if err := resolver.Browse(browseCtx, serviceType, "local.", entries); err != nil {
		return nil, fmt.Errorf("browse: %w", err)
	}

	var collected []*zeroconf.ServiceEntry
	for {
		select {
		case <-browseCtx.Done():
			return entriesToServices(serviceType, collected), nil
		case e, ok := <-entries:
			if !ok {
				// Channel closed by zeroconf when its internal context expires.
				return entriesToServices(serviceType, collected), nil
			}
			if e != nil {
				collected = append(collected, e)
			}
		}
	}
}

// entriesToServices converts a slice of *zeroconf.ServiceEntry to
// []schema.MDNSService, preserving the input order.
func entriesToServices(serviceType string, entries []*zeroconf.ServiceEntry) []schema.MDNSService {
	out := make([]schema.MDNSService, 0, len(entries))
	for _, e := range entries {
		if e == nil {
			continue
		}
		out = append(out, entryToService(serviceType, e))
	}
	return out
}

// entryToService maps a single zeroconf.ServiceEntry to schema.MDNSService.
//
// ServiceType is the type passed to Browse, not e.Service, because zeroconf
// fills e.Service with the trailing-dot form ("_airplay._tcp.") and we want
// the canonical no-trailing-dot form on the manifest.
func entryToService(serviceType string, e *zeroconf.ServiceEntry) schema.MDNSService {
	svc := schema.MDNSService{
		ServiceType: serviceType,
		Instance:    composeInstance(e),
		Hostname:    strings.TrimSuffix(e.HostName, "."),
		Port:        e.Port,
		Addrs:       collectAddrs(e),
		TXT:         parseTXT(e.Text),
	}
	return svc
}

// composeInstance returns the fully-qualified instance name in the form
// "<instance>.<service>.<domain>", trimming a trailing dot. If e.Service or
// e.Domain are empty, falls back to e.Instance alone.
func composeInstance(e *zeroconf.ServiceEntry) string {
	if e.Instance == "" {
		return ""
	}
	if e.Service == "" || e.Domain == "" {
		return e.Instance
	}
	service := strings.TrimSuffix(e.Service, ".")
	domain := strings.TrimSuffix(e.Domain, ".")
	if service == "" || domain == "" {
		return e.Instance
	}
	return strings.TrimSuffix(fmt.Sprintf("%s.%s.%s", e.Instance, service, domain), ".")
}

// collectAddrs returns a flat []string of stringified IPv4 then IPv6
// addresses from the ServiceEntry. Returns nil if none.
func collectAddrs(e *zeroconf.ServiceEntry) []string {
	if len(e.AddrIPv4) == 0 && len(e.AddrIPv6) == 0 {
		return nil
	}
	out := make([]string, 0, len(e.AddrIPv4)+len(e.AddrIPv6))
	for _, ip := range e.AddrIPv4 {
		out = append(out, ip.String())
	}
	for _, ip := range e.AddrIPv6 {
		out = append(out, ip.String())
	}
	return out
}

// parseTXT splits each "key=value" entry of e.Text into a map. Entries with
// no "=" are kept as keys with an empty-string value. Returns nil if the
// input is empty so that omitempty in MDNSService.TXT keeps the manifest
// tidy.
func parseTXT(text []string) map[string]string {
	if len(text) == 0 {
		return nil
	}
	out := make(map[string]string, len(text))
	for _, kv := range text {
		if kv == "" {
			continue
		}
		if i := strings.IndexByte(kv, '='); i >= 0 {
			out[kv[:i]] = kv[i+1:]
		} else {
			out[kv] = ""
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// resolveInterfaces converts a list of interface names to []net.Interface
// suitable for zeroconf.SelectIfaces. An empty input returns a nil slice
// (meaning: zeroconf default of "all interfaces").
func resolveInterfaces(names []string) ([]net.Interface, error) {
	if len(names) == 0 {
		return nil, nil
	}
	out := make([]net.Interface, 0, len(names))
	for _, name := range names {
		ifi, err := net.InterfaceByName(name)
		if err != nil {
			return nil, fmt.Errorf("interface %q: %w", name, err)
		}
		out = append(out, *ifi)
	}
	return out, nil
}
