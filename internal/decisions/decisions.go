// Package decisions runs library accept/reject logic against a parsed
// UPnP descriptor and reports a [schema.Decision] for each known
// control-point library.
//
// For v0 the upstream Go libraries are not invoked. Their public APIs do
// SSDP discovery and descriptor parsing as one operation, which doesn't
// fit the "feed it this descriptor" shape tutti needs. Instead each
// library's accept/reject logic is reimplemented faithfully against the
// already-parsed descriptor, with method=reimplementation and
// confidence=medium so the manifest is honest about what was checked.
//
// When a real library invocation harness lands (likely a small subprocess
// that takes a descriptor URL on stdin), the library identifiers will
// switch from "@reimpl-of-X" to "@X" and method to library_call.
package decisions

import (
	"context"
	"net/url"

	"gitlab.com/dunn.dev/tutti/internal/schema"
)

// Library identifier strings, in display order. The "@reimpl-of-X" suffix
// is deliberate: tutti is mirroring the library's logic rather than
// invoking it. When real library calls land, identifiers become "@X".
const (
	LibGoUpnpcast  = "go-upnpcast@reimpl-of-v0.1.0"
	LibHuinGoupnp  = "huin/goupnp@reimpl-of-v1.3.0"
)

// UPnP service / device type strings the libraries match against.
const (
	mediaRendererV1 = "urn:schemas-upnp-org:device:MediaRenderer:1"
	avTransportV1   = "urn:schemas-upnp-org:service:AVTransport:1"
)

// Libraries returns the static list of library identifiers Compare will
// produce decisions for, in display order. Exposed so callers (manifest
// renderers, doc generators) can iterate without depending on map order.
func Libraries() []string {
	return []string{LibGoUpnpcast, LibHuinGoupnp}
}

// Compare runs each known control-point library's parser logic against
// the descriptor and returns one Decision per library. The map is keyed
// by library identifier (see [Libraries]).
//
// descriptorURL is the LOCATION URL the SSDP response pointed at, kept
// in the signature for future variants that fetch from URL; the v0
// reimplementations only use parsed.
//
// rawXML is similarly retained for future variants that re-parse the
// raw bytes; v0 uses the already-parsed canonical struct.
func Compare(ctx context.Context, descriptorURL string, parsed schema.DescriptorParsed, rawXML []byte) map[string]schema.Decision {
	_ = ctx
	_ = descriptorURL
	_ = rawXML

	return map[string]schema.Decision{
		LibGoUpnpcast: decideGoUpnpcast(parsed),
		LibHuinGoupnp: decideHuinGoupnp(parsed),
	}
}

// decideGoUpnpcast mirrors github.com/supersonic-app/go-upnpcast v0.1.0
// device.findMediaRenderer + mediaRendererFromDeviceURL:
//
//   - root device (and any nested deviceList entry) must have deviceType
//     exactly equal to "urn:schemas-upnp-org:device:MediaRenderer:1";
//   - that device must have an AVTransport service in its serviceList;
//   - the AVTransport service's controlURL must be non-empty and parse
//     as a request URI.
//
// RenderingControl and ConnectionManager are bonuses but not required.
func decideGoUpnpcast(parsed schema.DescriptorParsed) schema.Decision {
	mr := findByDeviceTypeExact(&parsed, mediaRendererV1)
	if mr == nil {
		return schema.Decision{
			Result:     schema.ResultRejected,
			ReasonCode: "no_mediarenderer_v1",
			ReasonText: "deviceType is " + quote(parsed.DeviceType) +
				", not the exact string " + mediaRendererV1 + "; library would reject",
			Method:     schema.MethodReimplementation,
			Confidence: schema.ConfidenceMedium,
		}
	}

	avt := findServiceOnDevice(mr, avTransportV1)
	if avt == nil {
		return schema.Decision{
			Result:     schema.ResultRejected,
			ReasonCode: "no_avtransport",
			ReasonText: "MediaRenderer:1 device has no AVTransport:1 service in its serviceList",
			Method:     schema.MethodReimplementation,
			Confidence: schema.ConfidenceMedium,
		}
	}

	if avt.ControlURL == "" {
		return schema.Decision{
			Result:     schema.ResultRejected,
			ReasonCode: "avtransport_control_url_invalid",
			ReasonText: "AVTransport service has empty controlURL; library would reject as wrong DMR",
			Method:     schema.MethodReimplementation,
			Confidence: schema.ConfidenceMedium,
		}
	}
	if _, err := url.ParseRequestURI(avt.ControlURL); err != nil {
		return schema.Decision{
			Result:     schema.ResultRejected,
			ReasonCode: "avtransport_control_url_invalid",
			ReasonText: "AVTransport controlURL " + quote(avt.ControlURL) + " fails url.ParseRequestURI",
			Method:     schema.MethodReimplementation,
			Confidence: schema.ConfidenceMedium,
		}
	}

	return schema.Decision{
		Result:     schema.ResultAccepted,
		ReasonCode: "mediarenderer_v1_avtransport_found",
		ReasonText: "deviceType matched MediaRenderer:1; AVTransport service present with controlURL",
		Method:     schema.MethodReimplementation,
		Confidence: schema.ConfidenceMedium,
	}
}

// decideHuinGoupnp mirrors github.com/huin/goupnp v1.3.0 service-client
// discovery, which is broader than go-upnpcast: when callers ask for an
// AVTransport client, goupnp returns a client for any device whose
// serviceList contains AVTransport:1, regardless of that device's own
// deviceType. So this scan walks the entire descriptor (root device plus
// every embedded device) and accepts on first AVTransport:1 hit.
func decideHuinGoupnp(parsed schema.DescriptorParsed) schema.Decision {
	if anyServiceInTree(&parsed, avTransportV1) {
		return schema.Decision{
			Result:     schema.ResultAccepted,
			ReasonCode: "any_device_with_avtransport_service",
			ReasonText: "AVTransport:1 service found somewhere in the device tree; goupnp would expose an AVTransport client",
			Method:     schema.MethodReimplementation,
			Confidence: schema.ConfidenceMedium,
		}
	}
	return schema.Decision{
		Result:     schema.ResultRejected,
		ReasonCode: "no_avtransport_anywhere",
		ReasonText: "no AVTransport:1 service anywhere in the device tree; goupnp would not return an AVTransport client",
		Method:     schema.MethodReimplementation,
		Confidence: schema.ConfidenceMedium,
	}
}

// findByDeviceTypeExact walks root + nested devices depth-first and
// returns the first device whose DeviceType equals want exactly.
func findByDeviceTypeExact(d *schema.DescriptorParsed, want string) *schema.DescriptorParsed {
	if d == nil {
		return nil
	}
	if d.DeviceType == want {
		return d
	}
	for i := range d.EmbeddedDevices {
		if hit := findByDeviceTypeExact(&d.EmbeddedDevices[i], want); hit != nil {
			return hit
		}
	}
	return nil
}

// findServiceOnDevice returns the first service on this single device
// whose ServiceType equals want. Does not recurse into embedded devices,
// matching go-upnpcast's behaviour of only scanning the matched device.
func findServiceOnDevice(d *schema.DescriptorParsed, want string) *schema.DescriptorService {
	if d == nil {
		return nil
	}
	for i := range d.Services {
		if d.Services[i].ServiceType == want {
			return &d.Services[i]
		}
	}
	return nil
}

// anyServiceInTree reports whether want appears as a serviceType on any
// device in the tree (root or embedded), at any depth.
func anyServiceInTree(d *schema.DescriptorParsed, want string) bool {
	if d == nil {
		return false
	}
	for i := range d.Services {
		if d.Services[i].ServiceType == want {
			return true
		}
	}
	for i := range d.EmbeddedDevices {
		if anyServiceInTree(&d.EmbeddedDevices[i], want) {
			return true
		}
	}
	return false
}

// quote wraps s in single quotes so reason_text values stay readable
// when the embedded string is itself empty or contains punctuation.
func quote(s string) string {
	return "'" + s + "'"
}
