---
title: "Why it goes wrong"
eyebrow: "Failure modes"
description: "Parser variance, vendor SDK fingerprints, ProtocolInfo pathologies, network plumbing — with linked real-world bug reports."
order: 2
---


This document explains the failure modes behind the most common user complaint: "my renderer shows up in app X but not app Y" or "format X plays in Sonos but not in Subsonic." It assumes familiarity with UPnP/DLNA protocol basics (see `01-protocols.md`).

<div class="cat-explainer">
<svg viewBox="0 0 720 240" role="img" aria-label="One device descriptor handed to three different control-point libraries can produce three different verdicts: one library accepts the device, two reject it, each for a different reason. This is the failure mode behind 'works in app X, not in app Y'.">
  <rect x="20" y="20" width="680" height="200" fill="var(--bg)" stroke="var(--border)" stroke-width="1" rx="3"/>
  <text x="40" y="46" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="var(--text-muted)" letter-spacing="1.4">PARSER VARIANCE</text>

  <rect x="40" y="92" width="150" height="68" rx="3" fill="var(--bg)" stroke="var(--accent)" stroke-width="2"/>
  <text x="115" y="112" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="var(--accent)" letter-spacing="1.2">ONE DESCRIPTOR</text>
  <text x="115" y="132" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="var(--text)">device.xml</text>
  <text x="115" y="148" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-muted)">same bytes, every time</text>

  <line x1="190" y1="100" x2="260" y2="80" stroke="var(--text-muted)" stroke-width="2"/>
  <polygon points="260,80 252,88 249,78" fill="var(--text-muted)"/>
  <line x1="190" y1="126" x2="260" y2="126" stroke="var(--text-muted)" stroke-width="2"/>
  <polygon points="260,126 250,121 250,131" fill="var(--text-muted)"/>
  <line x1="190" y1="152" x2="260" y2="172" stroke="var(--text-muted)" stroke-width="2"/>
  <polygon points="260,172 249,174 252,164" fill="var(--text-muted)"/>

  <rect x="260" y="64" width="180" height="36" rx="2" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="276" y="80" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">LIBRARY A</text>
  <text x="276" y="93" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="var(--text-2)">substring match on deviceType</text>

  <rect x="260" y="110" width="180" height="36" rx="2" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="276" y="126" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">LIBRARY B</text>
  <text x="276" y="139" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="var(--text-2)">strict XML, namespace-aware</text>

  <rect x="260" y="156" width="180" height="36" rx="2" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="276" y="172" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">LIBRARY C</text>
  <text x="276" y="185" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="var(--text-2)">requires exact deviceType URN</text>

  <line x1="440" y1="82" x2="500" y2="82" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="500,82 490,77 490,87" fill="var(--accent)"/>
  <line x1="440" y1="128" x2="500" y2="128" stroke="var(--text-muted)" stroke-width="2"/>
  <polygon points="500,128 490,123 490,133" fill="var(--text-muted)"/>
  <line x1="440" y1="174" x2="500" y2="174" stroke="var(--text-muted)" stroke-width="2"/>
  <polygon points="500,174 490,169 490,179" fill="var(--text-muted)"/>

  <rect x="500" y="64" width="160" height="36" rx="2" fill="var(--bg)" stroke="var(--accent)" stroke-width="2"/>
  <text x="580" y="79" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="var(--accent)" letter-spacing="1.2">ACCEPTED</text>
  <text x="580" y="93" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-muted)">deviceType match found</text>

  <rect x="500" y="110" width="160" height="36" rx="2" fill="var(--bg)" stroke="var(--text-muted)" stroke-width="1.5" stroke-dasharray="4,2"/>
  <text x="580" y="125" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">REJECTED</text>
  <text x="580" y="139" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-2)">missing xmlns on child element</text>

  <rect x="500" y="156" width="160" height="36" rx="2" fill="var(--bg)" stroke="var(--text-muted)" stroke-width="1.5" stroke-dasharray="4,2"/>
  <text x="580" y="171" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">REJECTED</text>
  <text x="580" y="185" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-2)">non-standard deviceType suffix</text>
</svg>

The spec lets a device emit XML that's technically conforming but that strict parsers will reject. Different libraries enforce different rules, so the <em>same</em> wire bytes produce <em>different</em> verdicts. That's why "this device shows up in BubbleUPnP but not in Symfonium" is almost always a parser disagreement, not a network issue. tutti's job is to reproduce each library's verdict and name the rule it tripped over.
</div>

---

## The Spec Is Permissive Enough to Bite People

The UPnP Device Architecture specification is explicit that control points "must ignore" unrecognized elements and attributes in device descriptors. Section 2.3 of UDA 1.1 states that XML elements not defined in the schema are permitted as vendor extensions, and that the schema itself is non-normative. No schema validation is required at descriptor parse time.

The practical consequences:

- **Namespace declarations on inner elements**: Platinum SDK (used in Kodi, Plex, and dozens of embedded streamers) repeats namespace declarations on child elements inside `<device>`. The spec allows this. Strict XML parsers reject it. Cling (used in BubbleUPnP and JUPnP-based renderers) is strict by default; a ReadyMedia bug ([sourceforge.net/p/minidlna/bugs/316](https://sourceforge.net/p/minidlna/bugs/316/)) involved missing `xmlns:sec` on Samsung-namespace elements that Cling refused to parse. The fix was to add the declaration. WiiM firmware pre-5.0.613551 emitted `<song:rate_hz>` elements without a corresponding `xmlns:song` declaration in SOAP responses ([forum.wiimhome.com thread](https://forum.wiimhome.com/threads/bug-invalid-xml-returned-from-at-least-upnp-getmediainfo-invocation-after-playing-track-from-dlna-server.3133/)); strict parsers failed on every GetPositionInfo call.

- **Optional fields that aren't really optional**: `<friendlyName>`, `<manufacturer>`, and `<modelName>` are listed as required in the schema but many parsers ignore missing values. `<UDN>` is required and must be globally unique. Some parsers will reject a descriptor without a UDN; others silently continue, which causes a different failure mode: two devices collide in the device table.

- **Vendor extensions are allowed anywhere**: The Denon AVR-X2700H descriptor includes `<DMH:X_Audyssey>`, `<DMH:X_AudysseyPort>`, `<DMH:X_WebAPIPort>`, and `<qq:X_QPlay_SoftwareCapability>` (Tencent QPlay) as sibling elements to `<deviceType>`. Any parser that does schema-validated deserialization will fail. Parsers that do element-by-element extraction will work. The spec says to ignore unknown elements; not all parsers do.

---

## Parser-Side Variance

### Exact-string vs. substring matching on deviceType

The spec defines `deviceType` as a URN: `urn:schemas-upnp-org:device:MediaRenderer:1`. Control points are expected to match on this exact string (or on a prefix for version tolerance). In practice:

- **Sonos** root devices advertise `urn:schemas-upnp-org:device:ZonePlayer:1`. The `MediaRenderer:1` is a *sub-device* nested inside a `<deviceList>`. Control points that scan only the root `<deviceType>` for `MediaRenderer:1` never find Sonos. Supersonic's go-upnpcast library `findMediaRenderer()` recurses into `<deviceList>`, which is correct — but the initial release did not, causing issue [#594](https://github.com/dweymouth/supersonic/issues/594): "Sonos doesn't appear in the cast list." The fix required walking the full device tree.

- **Denon** (and all Aios-platform AVRs) advertise `urn:schemas-denon-com:device:AiosDevice:1` as the root, again with `MediaRenderer:1` nested one level down in a `<deviceList>`. The exact Denon descriptor from issue [#614](https://github.com/dweymouth/supersonic/issues/614) shows `<deviceType>urn:schemas-denon-com:device:AiosDevice:1</deviceType>` at the root, with the standard MediaRenderer URN on the nested child.

- **KEF Wireless 2** and other Platinum-based embedded streamers use `urn:schemas-upnp-org:device:MediaRenderer:1` at the root — so they work with naive parsers. But they may also have a sibling root device for the control surface with a vendor URN; parsers that expect exactly one root device get confused.

### UDN format: RFC 4122 vs. non-conforming patterns

The spec requires `uuid:` prefix followed by an RFC 4122 UUID (8-4-4-4-12 hex groups). Platinum SDK generates UDNs using a MAC address-seeded algorithm that can produce strings like `uuid:00-11-22-33-44-55-dmr` — syntactically a UUID prefix with a non-standard suffix. The spec (UDA 1.1, section 1.1.4) says control points "must be able to accept UUIDs that have not been formatted according to those rules," but many do not. Home Assistant's IGD integration logged "wanted UPnP/IGD device with UDN `uuid: xxxx` not found, aborting" ([community thread](https://community.home-assistant.io/t/wanted-upnp-igd-device-with-udn-uuid-xxxx-not-found-aborting/170861)) when the UDN had a trailing space after `uuid:` — a whitespace sensitivity bug in the matching code. Platinum's generated UDNs have caused similar failures in parsers that do exact-string key lookups on the UDN.

### serviceList / controlURL: relative vs. absolute URLs and URLBase

The `<controlURL>` element can be either absolute (`http://192.168.1.5:1400/MediaRenderer/AVTransport/Control`) or relative (`/MediaRenderer/AVTransport/Control`). The spec says relative URLs are resolved against the device description URL. The `<URLBase>` element, deprecated in UDA 1.1, provided an explicit base override. Parsers that implement URLBase resolution and parsers that don't behave differently on the same descriptor.

Concretely: Jellyfin bug [#2377](https://github.com/jellyfin/jellyfin/issues/2377) — when Jellyfin was configured with a BaseURL path prefix (e.g., `/jellyfin/`), UPnP clients including VLC and gssdp-discover could no longer discover the server. The descriptor's `<controlURL>` values were relative, but the base path was not factored in when constructing the description URL. The workaround was to strip the path prefix at the reverse proxy. Root cause: Jellyfin's UPnP code built the description URL from host only, not from the full configured base URL.

go-upnpcast's unmarshal.go normalizes relative control URLs by prepending `/` if absent and then concatenating `scheme://host`. This works for most devices but breaks on devices that use truly relative paths without a leading slash (e.g., `upnp/control/AVTransport`).

### Element ordering

The UPnP spec does not mandate element order within `<device>` or `<serviceList>`. Most parsers use a DOM-based approach and are order-independent. A small number of line-by-line SAX-style parsers written for embedded use assume `<deviceType>` appears before `<UDN>`, or that `<serviceList>` is the last child. When a device reorders elements (common in firmware updates), these parsers silently fail to populate fields.

### XML namespace declarations on inner elements

Platinum SDK declares its vendor namespace (`xmlns:plt="urn:schemas-plutinosoft-com:metadata-1-0/"`) on the `<root>` element and then repeats it on individual child elements. The XML spec permits this; it is redundant but valid. Parsers that use namespace-aware processing and cache the namespace table can handle this. Parsers that treat each `xmlns:` attribute as a new registration may error on the redeclaration. This is an edge case, but enough DLNA clients are built on minimalist XML libraries that it surfaces.

---

## Vendor SDK Fingerprints

### Plutinosoft Platinum (DMP-A6, KEF Wireless 2, Kodi, Plex, most embedded streamers)

Platinum is the dominant embedded C++ UPnP SDK. Recognizable by:
- Repeated namespace declarations on inner elements
- Vendor URN `urn:schemas-plutinosoft-com:metadata-1-0/` in the descriptor
- MAC address-seeded UDNs
- ProtocolInfo sink lists that are static and often incomplete (see below)
- `X-AV-Client-Info` header on SOAP requests

KEF Wireless 2, Bluesound NODE, and most Cambridge Audio/NAD streamers are Platinum-based. DMP-A6 (Denon portable) is Platinum; the full AVR line is Aios (different SDK).

### Sonos

Sonos uses its own proprietary stack. Key fingerprints:
- Root `deviceType`: `urn:schemas-upnp-org:device:ZonePlayer:1`
- Custom services: `AudioIn`, `GroupManagement`, `MusicServices`, `ZoneGroupTopology`
- MediaRenderer sub-device nested inside `<deviceList>`, not at root
- AVTransport control URL: `/MediaRenderer/AVTransport/Control`
- SSDP `ST` response: `urn:schemas-upnp-org:device:ZonePlayer:1` and `urn:rinconnetworks-com:device:ZonePlayer:1`
- Control points that do a SSDP `M-SEARCH` for `urn:schemas-upnp-org:device:MediaRenderer:1` will not get a direct Sonos response; they get `ZonePlayer:1` and must then fetch the descriptor and walk the device tree.

Post-S2 app update, Sonos removed the UPnP/DLNA library browsing UI. The hardware still responds to AVTransport commands from third-party control points, but Sonos's own apps no longer act as UPnP control points.

### Denon/Marantz Aios

Aios platform (all current Denon/Marantz network AVRs): root `urn:schemas-denon-com:device:AiosDevice:1`, MediaRenderer and AiosServices sub-devices in `<deviceList>`. Extra services: QPlay (`urn:schemas-tencent-com:service:QPlay:1`), ErrorHandler (`urn:schemas-denon-com:service:ErrorHandler:1`). The QPlay namespace declaration is present on the root element; parsers that don't handle multiple non-UPnP namespaces can trip here.

### CyberLink SDK (Cyberlink CMS, older Samsung devices)

CyberLink's SDK is recognizable by its SOAP fault format and by `<X_hardwareVersion>` / `<X_softwareVersion>` in the descriptor. Some CyberLink devices send `HTTP/1.0` for SOAP responses where the spec implies `HTTP/1.1`. HTTP/1.0 parsers that expect chunked transfer encoding (HTTP/1.1) will fail to read the response body.

### Intel UPnP / libupnp / pupnp

The libupnp (pupnp) stack powers ReadyMedia (MiniDLNA), many NAS UPnP servers, and HDHomeRun. Known quirks: SSDP `M-SEARCH` responses bind to a local ephemeral port; stateful firewalls see the response from a LAN device as unsolicited traffic and drop it. The go2tv issue [#72](https://github.com/alexballas/go2tv/issues/72) traced intermittent device disappearance to exactly this: the NixOS default firewall drops replies from `239.255.255.250` arriving on an unexpected source port.

### JUPnP (Cling successor) on Android / embedded

JUPnP is strict about XML namespace declarations and schema conformance. As noted above, the ReadyMedia missing `xmlns:sec` bug ([sourceforge.net/p/minidlna/bugs/316](https://sourceforge.net/p/minidlna/bugs/316/)) was caught by Cling/JUPnP. JUPnP-based control points (BubbleUPnP, some Linn apps) will reject descriptors that namespace-lenient parsers accept without complaint.

---

## ProtocolInfo Sink List Pathologies

The `GetProtocolInfo` response from a renderer's ConnectionManager service lists the `<Sink>` protocols the device can play. Format per entry:

```
http-get:*:audio/mpeg:DLNA.ORG_PN=MP3;DLNA.ORG_FLAGS=01700000000000000000000000000000
```

### DLNA.ORG_PN profile names that don't match registered profiles

`DLNA.ORG_PN` is a registered profile name from the DLNA Guidelines. The registered names are case-sensitive and version-specific: `LPCM`, `MP3`, `AAC_ISO`, `AAC_ISO_320`, `FLAC`, `DSF`, etc. Devices that use non-registered strings (e.g., `LPCM_HI_RES`, `MP3_EXTENDED`) cause control points that do PN-matching for format selection to fail silently — the profile is not in the control point's known-profile table, so the format is treated as unsupported even if the MIME type matches.

PS3 Media Server forum discussions ([ps3mediaserver.org](http://www.ps3mediaserver.org/forum/viewtopic.php?p=30719)) document the case where a device's PN string for PCM audio was missing entirely, causing PMS to refuse to send LPCM even though the device played it fine.

### Sink lists that omit formats the device actually plays

Platinum SDK ships a static default sink list compiled into the library. Devices that don't override it at runtime will advertise only the profiles in that static list. DSD (DSF/DSDIFF) is not in Platinum's default sink list. KEF Wireless 2 and other Platinum-based DSD-capable streamers therefore advertise no DSD support. Control points that rely on the sink list for format selection will never attempt DSD, even though the device plays it over HTTP without issue.

The upmpdcli issue [#43](https://www.lesbonscomptes.com/upmpdcli/github-issues/upmpdcli-html/issue-43.html) shows the inverse: the media server (MinimServer) failed to include `protocolInfo` in the resource descriptor, so upmpdcli reported "resource has no protocolInfo" and refused to add DSD files to a playlist. The workaround was `checkcontentformat = 0` in upmpdcli config, which disables sink-list matching entirely.

### Sink lists that announce formats the device chokes on

Some devices advertise `audio/x-flac` or `audio/flac` (MIME type varies by implementation) at high sample rates (192kHz/24-bit) but hard-stop or skip tracks above 96kHz. The sink list has no mechanism to express sample rate limits within a MIME type entry; the `DLNA.ORG_PN` profile implies the limits, but only if the device uses a real registered profile name.

### The `*` wildcard and what it means

`http-get:*:*:*` in a sink list means "I'll accept any HTTP-delivered content." Some devices advertise this as a catch-all. Control points cannot use a wildcard sink to infer anything about actual format support. A device that advertises `*:*:*` and then rejects FLAC is not violating the spec — the wildcard is a declaration of transport capability, not codec capability. Naim streamers historically shipped `*` sinks and relied on the control point to know from documentation which formats were actually supported.

---

## DIDL-Lite Metadata Pathologies

DIDL-Lite is sent as the `CurrentURIMetaData` argument in `SetAVTransportURI`. It is an XML document, but it is sent as a *string* inside a SOAP envelope — the DIDL-Lite XML must be entity-escaped (`<` becomes `&lt;`, `&` becomes `&amp;`, etc.) before insertion into the SOAP body.

### Double-escaping

Libraries that compose the SOAP envelope by string concatenation after already receiving an escaped DIDL-Lite string will double-escape: `&lt;` becomes `&amp;lt;`. The device receives `&amp;lt;item` and either fails to parse it or ignores the metadata entirely and shows no title/artist. The Kodi forum thread on SOAP XML parsing errors ([forum.kodi.tv, tid=358054](https://forum.kodi.tv/showthread.php?tid=358054)) documents this pattern: the device logs "bad request" and the control point gets no error indication, only silent metadata loss.

### Devices that read embedded tags instead of DIDL-Lite

Some renderers (common in budget Android-based streamers) ignore `CurrentURIMetaData` entirely and instead fetch the media URL, open it with a local decoder, and extract ID3/Vorbis/FLAC tags from the stream. This means: (1) metadata depends on what's embedded in the file, not what the server sends; (2) cover art from DIDL-Lite `<upnp:albumArtURI>` is never displayed — only embedded art; (3) the display updates only after initial buffering, not immediately on `SetAVTransportURI`. Control points cannot distinguish "metadata from tags" from "metadata from DIDL-Lite" — both look like the renderer is working.

### Multiple `<upnp:albumArtURI>` elements

The DIDL-Lite spec allows multiple `<upnp:albumArtURI>` elements with different `dlna:profileID` attributes (e.g., one `JPEG_TN` thumbnail and one `JPEG_MED` full-size). The renderer is supposed to pick the appropriate size. Many renderers take the *last* element. Some take the *first*. Others take whichever URI resolves fastest.

Kodi issue [#17587](https://github.com/xbmc/xbmc/issues/17587) is a concrete regression: Kodi 19's UPnP server started placing the video file URI in the `albumArtURI` field instead of the poster image URI. LG webOS TVs showed no thumbnail because the URI returned a video stream, not an image. The bug was in how Kodi constructed the DIDL-Lite metadata after a code refactor — the wrong field was mapped.

Airsonic issue [#1905](https://github.com/airsonic/airsonic/issues/1905) shows a different flavor: cover art was "jumbled" — albums showed art belonging to other albums — because the server was using a non-deterministic internal ID to generate the art URI, and the ID did not correspond 1:1 with album identity in the DIDL-Lite context.

---

## Network Plumbing

### Wi-Fi AP isolation

Consumer access points ship with "AP isolation" or "client isolation" enabled by default on guest networks and sometimes on main networks. Isolation prevents Wi-Fi clients from sending unicast packets to other Wi-Fi clients. SSDP discovery uses multicast to `239.255.255.250`; with AP isolation, the renderer's SSDP `alive` messages and M-SEARCH responses are blocked at the AP before they reach the control point. The result is a completely empty device list with no error message. Disabling isolation (or enabling "multicast/DLNA passthrough" if the AP offers it) fixes this.

### IGMP snooping misconfiguration

SSDP uses multicast. Managed switches use IGMP snooping to prune multicast traffic to only ports that have registered interest. If IGMP snooping is enabled but no IGMP querier is present on the network, group memberships expire after ~30 seconds (the default IGMPv2 timeout). The renderer disappears from the device list within a minute of discovery. The symptom: device appears briefly after powering on, then vanishes. Disabling IGMP snooping or enabling an IGMP querier (often a setting on the router) fixes this. TP-Link community thread [community.tp-link.com/us/home/forum/topic/244290](https://community.tp-link.com/us/home/forum/topic/244290) documents this pattern on TL-SG105E switches.

### Multi-NIC hosts

A control point running on a host with multiple network interfaces (wired + wireless, or multiple wired NICs) must bind its SSDP socket to the interface that can reach the renderer. Most SSDP implementations bind to `0.0.0.0`, which causes the OS to send the multicast on the default route interface. If the renderer is on a different subnet or interface, discovery fails. tutti's `--interface` flag addresses this directly: it binds the SSDP socket to a specific interface. Without it, a host with both an active VPN tun0 and a LAN eth0 will typically send SSDP on tun0, which is wrong.

### VPN tunnels and Docker bridges

WireGuard and OpenVPN tun interfaces claim multicast capability (`MULTICAST` flag in `ip link show`) but do not actually deliver multicast to the VPN peer. Docker bridges (`docker0`, custom networks) also have the multicast flag set but do not bridge to the physical LAN. Software on a VPN or inside a Docker container that tries SSDP discovery without explicit interface binding will bind to the tun or bridge interface and receive nothing. The fix is explicit interface selection or using the host network namespace.

### WSL2 networking

WSL2 runs inside a Hyper-V VM with a NAT'd network. Under the default NAT mode, WSL2 has its own IP address not reachable from the LAN, and multicast packets sent from WSL2 do not reach LAN devices. The WSL GitHub discussion [#10614](https://github.com/microsoft/WSL/discussions/10614) documents that even in mirrored network mode (Windows 11 22H2+, `networkingMode=mirrored` in `.wslconfig`), multicast send works but receive is unreliable. Sending SSDP M-SEARCH from WSL2 typically produces no responses from LAN renderers regardless of mode.

### macOS firewall and Windows Firewall first-run prompts

macOS's application-level firewall blocks incoming UDP on first run unless the user approves. SSDP M-SEARCH responses arrive on an ephemeral UDP port; macOS can drop them as "unsolicited." The go2tv issue [#614 comment](https://github.com/dweymouth/supersonic/issues/614) traced a WiiM device failure to Lulu (a third-party macOS firewall) blocking the response packets. Windows Firewall presents a first-run dialog; if dismissed without allowing, SSDP responses are silently dropped. Neither firewall logs a user-visible error in the application.

### IPv6 in mixed networks

Devices that support both IPv4 and IPv6 may announce SSDP on both. IPv6 multicast for SSDP uses `ff02::c` (link-local all-routers) or `ff05::c` (site-local). Control points that bind only to IPv4 will miss IPv6 announcements. Some devices announce a different `LOCATION` header value for IPv6 vs. IPv4 descriptions. If the control point follows the IPv6 LOCATION but its HTTP client is IPv4-only, the descriptor fetch fails. The failure mode is "SSDP response received, descriptor fetch failed" — the device briefly flickers in and visible device lists but drops out before the descriptor is parsed.

---

## Real-World Bug Examples

### dweymouth/supersonic#594 — Sonos not visible

**URL**: https://github.com/dweymouth/supersonic/issues/594

Sonos speakers don't appear in Supersonic's cast list. Root cause: the original go-upnpcast device discovery checked the root `<deviceType>` for `MediaRenderer:1`. Sonos's root is `ZonePlayer:1`; the `MediaRenderer:1` sub-device is nested in `<deviceList>`. Without recursive descent into the device tree, the MediaRenderer was never found. The fix was to walk `<deviceList>` recursively.

### dweymouth/supersonic#614 — WiiM Pro Plus not detected (macOS)

**URL**: https://github.com/dweymouth/supersonic/issues/614

WiiM Pro Plus not visible in cast list on macOS ARM64. The device was detected by Symfonium (Android) without issue. Investigation showed the Fyne tooltip error in the logs was a red herring; the real cause was a third-party firewall (Lulu) blocking incoming SSDP response UDP packets, reported by a different commenter. A separate commenter also noted that Windows Firewall prompts at first run block the same path.

### ReadyMedia / MiniDLNA bugs#316 — undeclared namespace breaks Cling

**URL**: https://sourceforge.net/p/minidlna/bugs/316/

ReadyMedia emitted `<sec:dcmInfo>` elements using the Samsung DLNA namespace without declaring `xmlns:sec`. Strict XML parsers (Cling, used in JUPnP-based control points) rejected the DIDL-Lite as invalid XML. Lenient parsers (most others) silently ignored the undeclared prefix. The fix was trivial — add the namespace declaration — but the bug survived for years because the majority of UPnP parsers are not strict.

### WiiM — undeclared `song:` namespace in SOAP responses

**URL**: https://forum.wiimhome.com/threads/bug-invalid-xml-returned-from-at-least-upnp-getmediainfo-invocation-after-playing-track-from-dlna-server.3133/

WiiM Pro devices emitted `<song:rate_hz>` and `<song:bitrate>` elements in GetMediaInfo/GetPositionInfo/GetInfoEx responses without declaring the `song` namespace. Any strict XML parser in the control point threw an exception on every position poll. The bug persisted until firmware 5.0.613551 (April 2024). Control points built on Python's `xml.etree` with namespace-strict mode, or on Cling, failed silently on WiiM until the firmware was updated.

### Kodi/xbmc#17587 — albumArtURI points to video file

**URL**: https://github.com/xbmc/xbmc/issues/17587

Kodi 19 regression: the DIDL-Lite `<upnp:albumArtURI>` field was populated with the media file URI (the video itself) instead of the poster image URI. LG webOS TVs displayed no thumbnail. A Wireshark comparison between Kodi 18 and 19 made the bug obvious: the wrong variable was referenced after a refactor. The `dlna:profileID="JPEG_TN"` attribute was present on the `albumArtURI` tag, so the TV knew to expect a JPEG thumbnail, but the URI returned video bytes. Result: black thumbnail, no error, silent failure.

### Airsonic#1905 — cover art jumbled across albums via DLNA

**URL**: https://github.com/airsonic/airsonic/issues/1905

Album cover art shown on DLNA clients (BubbleUPnP, Yamaha MusicCast) was incorrect: albums displayed art from other albums. The Airsonic web interface showed correct art. The bug was in how Airsonic generated the `<upnp:albumArtURI>` — the URI was derived from a non-stable internal ID that didn't map cleanly to album identity in the DIDL-Lite context. DLNA clients cached cover art by URI; when URIs collided or rotated, the wrong art was cached and displayed.

---

## "Works in App X, Not App Y" — Why Both Are Usually Right

This is the most common user-reported framing and the one most likely to produce an unhelpful bug report: "app X is broken because Y works."

The reason both apps are usually correct is that UPnP/DLNA has no canonical test suite and no normative conformance validator. Each app has made different choices about:

- **How strict to be with descriptor parsing**: a parser that accepts a non-RFC-4122 UDN discovers more devices; one that rejects it avoids ambiguous device identity.
- **Which device types to accept**: an app that accepts only `MediaRenderer:1` at the root misses Sonos and Denon; one that recurses into `<deviceList>` finds them but may also pick up non-renderer sub-devices from multi-function devices.
- **Whether to use the sink list for format negotiation**: an app that checks the sink list before sending a format avoids sending unsupported content; one that ignores the sink list discovers empirically what works, reaching Platinum-based DSD devices that don't advertise DSD.
- **How to resolve relative URLs**: an app that prepends the description URL base handles most devices; one that uses `URLBase` explicitly breaks when `URLBase` is absent; one that ignores `URLBase` breaks when a device intends it as an override.
- **How to handle IGMP-dropped discovery**: an app that issues M-SEARCH and waits finds devices that have already announced; one that waits for passive `alive` announcements depends on IGMP not dropping the multicast.

When a device "works in Symfonium but not Supersonic," Symfonium is making a more permissive choice somewhere. When a device "works in Supersonic but not BubbleUPnP," BubbleUPnP's Cling-based parser is rejecting something that Supersonic's Go XML parser silently accepts. Neither app has a bug in the sense of violating the spec; both are operating within the spec's enormous tolerance for implementation variance.

The tutti renderer probe exists to surface exactly this: what does the device's descriptor actually contain, what does its sink list claim, and what SOAP responses does it produce — so the question of "which app's interpretation is closer to what the device expects" can be answered from evidence rather than from guessing.
