package decisions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/dunn.dev/tutti/internal/descriptor"
	"gitlab.com/dunn.dev/tutti/internal/schema"
)

// loadEversolo reads the captured Eversolo DMP-A6 descriptor fixture and
// returns it parsed via the descriptor package, exactly as production does.
func loadEversolo(t *testing.T) (schema.DescriptorParsed, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "eversolo_descriptor.xml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	parsed, err := descriptor.Parse(raw)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return parsed, raw
}

// sonosLikeDescriptor synthesises a descriptor whose root device is NOT
// MediaRenderer:1 (it's a Sonos-style ZonePlayer:1) but whose embedded
// MediaRenderer device carries the AVTransport:1 service. This is the
// canonical case where huin/goupnp accepts and go-upnpcast accepts too,
// since go-upnpcast walks deviceList recursively. To make the divergence
// the spec asks for visible (go-upnpcast rejects, huin/goupnp accepts),
// we put AVTransport on a non-MediaRenderer embedded device.
const sonosLikeDescriptorXML = `<?xml version="1.0" encoding="UTF-8"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <friendlyName>SonosLike</friendlyName>
    <UDN>uuid:sonoslike-root</UDN>
    <serviceList/>
    <deviceList>
      <device>
        <deviceType>urn:schemas-sonos-com:device:Player:1</deviceType>
        <friendlyName>SonosLike Player</friendlyName>
        <UDN>uuid:sonoslike-player</UDN>
        <serviceList>
          <service>
            <serviceType>urn:schemas-upnp-org:service:AVTransport:1</serviceType>
            <serviceId>urn:upnp-org:serviceId:AVTransport</serviceId>
            <controlURL>/MediaRenderer/AVTransport/Control</controlURL>
            <eventSubURL>/MediaRenderer/AVTransport/Event</eventSubURL>
            <SCPDURL>/xml/AVTransport1.xml</SCPDURL>
          </service>
        </serviceList>
      </device>
    </deviceList>
  </device>
</root>`

// junkDescriptorXML has neither MediaRenderer:1 nor AVTransport:1.
const junkDescriptorXML = `<?xml version="1.0" encoding="UTF-8"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <deviceType>urn:schemas-denon-com:device:AiosDevice:1</deviceType>
    <friendlyName>Denon AIOS</friendlyName>
    <UDN>uuid:denon-aios</UDN>
    <serviceList>
      <service>
        <serviceType>urn:schemas-denon-com:service:ACT:1</serviceType>
        <serviceId>urn:denon-com:serviceId:ACT</serviceId>
        <controlURL>/control</controlURL>
        <eventSubURL>/event</eventSubURL>
        <SCPDURL>/scpd.xml</SCPDURL>
      </service>
    </serviceList>
  </device>
</root>`

// emptyControlURLDescriptorXML has MediaRenderer:1 + AVTransport but the
// AVTransport controlURL is the empty string. go-upnpcast must reject;
// huin/goupnp still accepts (it only cares that the service exists).
const emptyControlURLDescriptorXML = `<?xml version="1.0" encoding="UTF-8"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <deviceType>urn:schemas-upnp-org:device:MediaRenderer:1</deviceType>
    <friendlyName>BrokenMR</friendlyName>
    <UDN>uuid:broken-mr</UDN>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:AVTransport:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:AVTransport</serviceId>
        <controlURL></controlURL>
        <eventSubURL>/Event</eventSubURL>
        <SCPDURL>/scpd.xml</SCPDURL>
      </service>
    </serviceList>
  </device>
</root>`

func TestLibrariesOrder(t *testing.T) {
	got := Libraries()
	want := []string{LibGoUpnpcast, LibHuinGoupnp}
	if len(got) != len(want) {
		t.Fatalf("Libraries length: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Libraries[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCompare_Eversolo_BothAccept(t *testing.T) {
	parsed, raw := loadEversolo(t)
	got := Compare(context.Background(), "http://192.168.1.10:1234/dmr.xml", parsed, raw)

	if len(got) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(got))
	}

	upnpcast, ok := got[LibGoUpnpcast]
	if !ok {
		t.Fatalf("missing decision for %q", LibGoUpnpcast)
	}
	if upnpcast.Result != schema.ResultAccepted {
		t.Errorf("go-upnpcast: result=%q, want accepted (text=%q)", upnpcast.Result, upnpcast.ReasonText)
	}
	if upnpcast.ReasonCode != "mediarenderer_v1_avtransport_found" {
		t.Errorf("go-upnpcast: reason_code=%q, want mediarenderer_v1_avtransport_found", upnpcast.ReasonCode)
	}
	if upnpcast.Method != schema.MethodReimplementation {
		t.Errorf("go-upnpcast: method=%q, want reimplementation", upnpcast.Method)
	}
	if upnpcast.Confidence != schema.ConfidenceMedium {
		t.Errorf("go-upnpcast: confidence=%q, want medium", upnpcast.Confidence)
	}

	goupnp, ok := got[LibHuinGoupnp]
	if !ok {
		t.Fatalf("missing decision for %q", LibHuinGoupnp)
	}
	if goupnp.Result != schema.ResultAccepted {
		t.Errorf("huin/goupnp: result=%q, want accepted", goupnp.Result)
	}
	if goupnp.ReasonCode != "any_device_with_avtransport_service" {
		t.Errorf("huin/goupnp: reason_code=%q, want any_device_with_avtransport_service", goupnp.ReasonCode)
	}
}

func TestCompare_SonosLike_DivergentDecisions(t *testing.T) {
	raw := []byte(sonosLikeDescriptorXML)
	parsed, err := descriptor.Parse(raw)
	if err != nil {
		t.Fatalf("parse synthesised xml: %v", err)
	}
	got := Compare(context.Background(), "http://192.168.1.50/xml/device_description.xml", parsed, raw)

	upnpcast := got[LibGoUpnpcast]
	if upnpcast.Result != schema.ResultRejected {
		t.Errorf("go-upnpcast: result=%q, want rejected (text=%q)", upnpcast.Result, upnpcast.ReasonText)
	}
	if upnpcast.ReasonCode != "no_mediarenderer_v1" {
		t.Errorf("go-upnpcast: reason_code=%q, want no_mediarenderer_v1", upnpcast.ReasonCode)
	}

	goupnp := got[LibHuinGoupnp]
	if goupnp.Result != schema.ResultAccepted {
		t.Errorf("huin/goupnp: result=%q, want accepted (text=%q)", goupnp.Result, goupnp.ReasonText)
	}
	if goupnp.ReasonCode != "any_device_with_avtransport_service" {
		t.Errorf("huin/goupnp: reason_code=%q, want any_device_with_avtransport_service", goupnp.ReasonCode)
	}
}

func TestCompare_Junk_BothReject(t *testing.T) {
	raw := []byte(junkDescriptorXML)
	parsed, err := descriptor.Parse(raw)
	if err != nil {
		t.Fatalf("parse junk xml: %v", err)
	}
	got := Compare(context.Background(), "http://192.168.1.99/desc.xml", parsed, raw)

	upnpcast := got[LibGoUpnpcast]
	if upnpcast.Result != schema.ResultRejected {
		t.Errorf("go-upnpcast: result=%q, want rejected", upnpcast.Result)
	}
	if upnpcast.ReasonCode != "no_mediarenderer_v1" {
		t.Errorf("go-upnpcast: reason_code=%q, want no_mediarenderer_v1", upnpcast.ReasonCode)
	}

	goupnp := got[LibHuinGoupnp]
	if goupnp.Result != schema.ResultRejected {
		t.Errorf("huin/goupnp: result=%q, want rejected", goupnp.Result)
	}
	if goupnp.ReasonCode != "no_avtransport_anywhere" {
		t.Errorf("huin/goupnp: reason_code=%q, want no_avtransport_anywhere", goupnp.ReasonCode)
	}
}

func TestCompare_EmptyControlURL_GoUpnpcastRejects(t *testing.T) {
	raw := []byte(emptyControlURLDescriptorXML)
	parsed, err := descriptor.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := Compare(context.Background(), "http://192.168.1.77/desc.xml", parsed, raw)

	upnpcast := got[LibGoUpnpcast]
	if upnpcast.Result != schema.ResultRejected {
		t.Errorf("go-upnpcast: result=%q, want rejected (text=%q)", upnpcast.Result, upnpcast.ReasonText)
	}
	if upnpcast.ReasonCode != "avtransport_control_url_invalid" {
		t.Errorf("go-upnpcast: reason_code=%q, want avtransport_control_url_invalid", upnpcast.ReasonCode)
	}

	// huin/goupnp doesn't care about controlURL validity at the discovery
	// stage; the service is present, so it accepts.
	goupnp := got[LibHuinGoupnp]
	if goupnp.Result != schema.ResultAccepted {
		t.Errorf("huin/goupnp: result=%q, want accepted", goupnp.Result)
	}
}

// TestCompare_NoAVTransportOnMR ensures the second go-upnpcast rejection
// branch (MediaRenderer found, but no AVTransport service) is exercised.
func TestCompare_NoAVTransportOnMR(t *testing.T) {
	const xml = `<?xml version="1.0" encoding="UTF-8"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <deviceType>urn:schemas-upnp-org:device:MediaRenderer:1</deviceType>
    <friendlyName>SilentMR</friendlyName>
    <UDN>uuid:silent-mr</UDN>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:RenderingControl:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:RenderingControl</serviceId>
        <controlURL>/RC/Control</controlURL>
        <eventSubURL>/RC/Event</eventSubURL>
        <SCPDURL>/RC/scpd.xml</SCPDURL>
      </service>
    </serviceList>
  </device>
</root>`
	raw := []byte(xml)
	parsed, err := descriptor.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := Compare(context.Background(), "http://192.168.1.78/desc.xml", parsed, raw)

	upnpcast := got[LibGoUpnpcast]
	if upnpcast.Result != schema.ResultRejected {
		t.Errorf("go-upnpcast: result=%q, want rejected", upnpcast.Result)
	}
	if upnpcast.ReasonCode != "no_avtransport" {
		t.Errorf("go-upnpcast: reason_code=%q, want no_avtransport", upnpcast.ReasonCode)
	}

	goupnp := got[LibHuinGoupnp]
	if goupnp.Result != schema.ResultRejected {
		t.Errorf("huin/goupnp: result=%q, want rejected", goupnp.Result)
	}
	if goupnp.ReasonCode != "no_avtransport_anywhere" {
		t.Errorf("huin/goupnp: reason_code=%q, want no_avtransport_anywhere", goupnp.ReasonCode)
	}
}
