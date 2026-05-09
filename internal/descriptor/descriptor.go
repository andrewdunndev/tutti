// Package descriptor fetches and parses UPnP device descriptor XML.
//
// A UPnP device announces a LOCATION URL via SSDP; an HTTP GET on that
// URL returns an XML document describing the root device, its services,
// and any embedded devices. This package converts that XML into the
// schema.DescriptorParsed canonical struct used by the rest of tutti.
//
// The parser is namespace-tolerant: encoding/xml's default behaviour is
// to ignore namespace prefixes when struct tags do not request them, so
// elements like <dlna:X_DLNADOC> match the same struct field as plain
// <X_DLNADOC>. Service URLs in the source XML are typically host-relative
// (e.g. "/AVTransport/.../control.xml"); Fetch resolves them against the
// LOCATION URL so callers receive absolute URLs ready to use.
package descriptor

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/dunn.dev/tutti/internal/schema"
)

// httpTimeout is the per-attempt HTTP timeout.
const httpTimeout = 10 * time.Second

// retryDelays defines the exponential backoff schedule. Length determines
// the total attempt count: len(retryDelays)+1.
var retryDelays = []time.Duration{
	100 * time.Millisecond,
	400 * time.Millisecond,
	1 * time.Second,
}

// xmlRoot mirrors <root> with its single <device> child. We only care
// about <device>; specVersion, URLBase, and configId are ignored.
type xmlRoot struct {
	XMLName xml.Name  `xml:"root"`
	Device  xmlDevice `xml:"device"`
}

// xmlDevice mirrors a UPnP <device> element. The XDlnaDoc tag has no
// namespace prefix because encoding/xml strips namespace prefixes when
// struct tags don't request them, so <dlna:X_DLNADOC> matches.
type xmlDevice struct {
	DeviceType       string       `xml:"deviceType"`
	FriendlyName     string       `xml:"friendlyName"`
	Manufacturer     string       `xml:"manufacturer"`
	ManufacturerURL  string       `xml:"manufacturerURL"`
	ModelDescription string       `xml:"modelDescription"`
	ModelName        string       `xml:"modelName"`
	ModelURL         string       `xml:"modelURL"`
	ModelNumber      string       `xml:"modelNumber"`
	SerialNumber     string       `xml:"serialNumber"`
	UDN              string       `xml:"UDN"`
	PresentationURL  string       `xml:"presentationURL"`
	XDlnaDoc         string       `xml:"X_DLNADOC"`
	IconList         xmlIconList  `xml:"iconList"`
	ServiceList      xmlServiceList `xml:"serviceList"`
	DeviceList       xmlDeviceList  `xml:"deviceList"`
}

type xmlIconList struct {
	Icons []xmlIcon `xml:"icon"`
}

type xmlIcon struct {
	MIME   string `xml:"mimetype"`
	Width  int    `xml:"width"`
	Height int    `xml:"height"`
	Depth  int    `xml:"depth"`
	URL    string `xml:"url"`
}

type xmlServiceList struct {
	Services []xmlService `xml:"service"`
}

type xmlService struct {
	ServiceType string `xml:"serviceType"`
	ServiceID   string `xml:"serviceId"`
	SCPDURL     string `xml:"SCPDURL"`
	ControlURL  string `xml:"controlURL"`
	EventSubURL string `xml:"eventSubURL"`
}

type xmlDeviceList struct {
	Devices []xmlDevice `xml:"device"`
}

// Fetch GETs a UPnP device descriptor from the LOCATION URL, parses it,
// and returns the canonical struct + raw XML bytes. Network errors are
// retried up to 3 times with exponential backoff (100ms, 400ms, 1s);
// if all attempts fail, returns an error.
//
// Service URLs in the returned struct are resolved against the LOCATION
// URL so callers receive absolute URLs.
func Fetch(ctx context.Context, locationURL string) (schema.DescriptorParsed, []byte, error) {
	if locationURL == "" {
		return schema.DescriptorParsed{}, nil, errors.New("descriptor: empty location URL")
	}

	loc, err := url.Parse(locationURL)
	if err != nil {
		return schema.DescriptorParsed{}, nil, fmt.Errorf("descriptor: parse location: %w", err)
	}

	client := &http.Client{Timeout: httpTimeout}

	var raw []byte
	var lastErr error
	totalAttempts := len(retryDelays) + 1
	for attempt := 0; attempt < totalAttempts; attempt++ {
		if attempt > 0 {
			delay := retryDelays[attempt-1]
			select {
			case <-ctx.Done():
				return schema.DescriptorParsed{}, nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		raw, lastErr = fetchOnce(ctx, client, locationURL)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return schema.DescriptorParsed{}, nil, fmt.Errorf("descriptor: fetch %s: %w", locationURL, lastErr)
	}

	parsed, err := parseAndResolve(raw, loc)
	if err != nil {
		return schema.DescriptorParsed{}, raw, err
	}
	return parsed, raw, nil
}

// fetchOnce performs a single HTTP GET attempt.
func fetchOnce(ctx context.Context, client *http.Client, locationURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, locationURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "tutti/descriptor")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Drain a little so the connection can be reused, then surface status.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// Parse parses raw XML to schema.DescriptorParsed. Service URLs are left
// in the form they appeared in the document (typically host-relative).
// Used in tests and when the caller already has the bytes.
func Parse(raw []byte) (schema.DescriptorParsed, error) {
	return parseAndResolve(raw, nil)
}

// parseAndResolve parses raw XML and, if base is non-nil, resolves every
// service / icon / presentation URL against it.
func parseAndResolve(raw []byte, base *url.URL) (schema.DescriptorParsed, error) {
	var root xmlRoot
	if err := xml.Unmarshal(raw, &root); err != nil {
		return schema.DescriptorParsed{}, fmt.Errorf("descriptor: parse xml: %w", err)
	}
	return convertDevice(root.Device, base), nil
}

// convertDevice maps an xmlDevice (and its nested embedded devices) to
// schema.DescriptorParsed, optionally resolving relative URLs against base.
func convertDevice(d xmlDevice, base *url.URL) schema.DescriptorParsed {
	out := schema.DescriptorParsed{
		DeviceType:       strings.TrimSpace(d.DeviceType),
		FriendlyName:     strings.TrimSpace(d.FriendlyName),
		Manufacturer:     strings.TrimSpace(d.Manufacturer),
		ManufacturerURL:  strings.TrimSpace(d.ManufacturerURL),
		ModelDescription: strings.TrimSpace(d.ModelDescription),
		ModelName:        strings.TrimSpace(d.ModelName),
		ModelURL:         strings.TrimSpace(d.ModelURL),
		ModelNumber:      strings.TrimSpace(d.ModelNumber),
		SerialNumber:     strings.TrimSpace(d.SerialNumber),
		UDN:              strings.TrimSpace(d.UDN),
		PresentationURL:  resolveURL(strings.TrimSpace(d.PresentationURL), base),
		XDlnaDoc:         strings.TrimSpace(d.XDlnaDoc),
	}

	if n := len(d.IconList.Icons); n > 0 {
		out.Icons = make([]schema.DescriptorIcon, 0, n)
		for _, ic := range d.IconList.Icons {
			out.Icons = append(out.Icons, schema.DescriptorIcon{
				MIME:   strings.TrimSpace(ic.MIME),
				Width:  ic.Width,
				Height: ic.Height,
				Depth:  ic.Depth,
				URL:    resolveURL(strings.TrimSpace(ic.URL), base),
			})
		}
	}

	// Services is required by schema; emit at least an empty slice so that
	// a Marshal of the returned struct always serialises the field.
	out.Services = make([]schema.DescriptorService, 0, len(d.ServiceList.Services))
	for _, s := range d.ServiceList.Services {
		out.Services = append(out.Services, schema.DescriptorService{
			ServiceType: strings.TrimSpace(s.ServiceType),
			ServiceID:   strings.TrimSpace(s.ServiceID),
			ScpdURL:     resolveURL(strings.TrimSpace(s.SCPDURL), base),
			ControlURL:  resolveURL(strings.TrimSpace(s.ControlURL), base),
			EventSubURL: resolveURL(strings.TrimSpace(s.EventSubURL), base),
		})
	}

	if n := len(d.DeviceList.Devices); n > 0 {
		out.EmbeddedDevices = make([]schema.DescriptorParsed, 0, n)
		for _, sub := range d.DeviceList.Devices {
			out.EmbeddedDevices = append(out.EmbeddedDevices, convertDevice(sub, base))
		}
	}

	return out
}

// resolveURL resolves ref against base, returning ref unchanged when base
// is nil or ref is empty / unparseable. Already-absolute URLs pass through.
func resolveURL(ref string, base *url.URL) string {
	if ref == "" || base == nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(r).String()
}

// SelectMediaRenderer walks the descriptor's device tree (root device and
// embedded devices, depth-first) looking for a node whose deviceType matches
// "urn:schemas-upnp-org:device:MediaRenderer:1". Returns nil if absent.
func SelectMediaRenderer(d schema.DescriptorParsed) *schema.DescriptorParsed {
	const want = "urn:schemas-upnp-org:device:MediaRenderer:1"
	return findByDeviceType(&d, want)
}

func findByDeviceType(d *schema.DescriptorParsed, want string) *schema.DescriptorParsed {
	if d == nil {
		return nil
	}
	if d.DeviceType == want {
		return d
	}
	for i := range d.EmbeddedDevices {
		if hit := findByDeviceType(&d.EmbeddedDevices[i], want); hit != nil {
			return hit
		}
	}
	return nil
}
