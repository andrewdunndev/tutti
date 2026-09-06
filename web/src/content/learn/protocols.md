---
title: "How streaming works"
eyebrow: "Protocols"
description: "UPnP/DLNA audio streaming end to end: SSDP, descriptors, SOAP, DIDL-Lite, AVTransport."
order: 1
---


*Reference for tutti maintainers. Assumes familiarity with HTTP and XML.
Covers the control-plane wire protocols; audio codec internals are out of scope.*

---

## UPnP Architecture

UPnP (Universal Plug and Play) Device Architecture (UPnP-DA) defines three participant roles:

- **Device**: an entity that exposes one or more services. A device has a type URN, a UUID-based UDN (Unique Device Name, per RFC 4122), and an XML descriptor fetched from a well-known HTTP endpoint.
- **Service**: the atomic capability unit. Each service has a type URN, a control URL (SOAP endpoint), an event URL (GENA subscription endpoint), and a SCPD URL (the service's own XML description of its actions and state variables).
- **Control Point**: a client that discovers devices via SSDP, fetches their descriptors, and issues SOAP calls against service control URLs.

<div class="cat-explainer">
<svg viewBox="0 0 720 220" role="img" aria-label="The three UPnP participant roles. A control point on the left issues three kinds of request against a device on the right: M-SEARCH to discover, GET to fetch the descriptor, and SOAP to call an action on one of the device's services (AVTransport, RenderingControl, ConnectionManager).">
  <rect x="20" y="20" width="680" height="180" fill="var(--bg)" stroke="var(--border)" stroke-width="1" rx="3"/>
  <text x="40" y="46" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="var(--text-muted)" letter-spacing="1.4">UPNP ROLES</text>

  <rect x="40" y="76" width="150" height="104" rx="3" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="115" y="100" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">ROLE</text>
  <text x="115" y="124" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="13" font-weight="700" fill="var(--text)">Control Point</text>
  <text x="115" y="146" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="var(--text-muted)">music client</text>
  <text x="115" y="162" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="var(--text-muted)">or library</text>

  <line x1="190" y1="100" x2="282" y2="100" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="282,100 272,95 272,105" fill="var(--accent)"/>
  <text x="236" y="93" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="var(--text-2)">M-SEARCH</text>

  <line x1="190" y1="128" x2="282" y2="128" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="282,128 272,123 272,133" fill="var(--accent)"/>
  <text x="236" y="122" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="var(--text-2)">GET descriptor</text>

  <line x1="190" y1="156" x2="282" y2="156" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="282,156 272,151 272,161" fill="var(--accent)"/>
  <text x="236" y="150" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="var(--text-2)">SOAP action</text>

  <rect x="282" y="76" width="398" height="104" rx="3" fill="var(--bg)" stroke="var(--accent)" stroke-width="2"/>
  <text x="296" y="94" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="var(--accent)" letter-spacing="1.2">DEVICE  ·  http://192.168.1.42/description.xml</text>
  <text x="296" y="112" font-family="IBM Plex Mono, monospace" font-size="8" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">SERVICES</text>

  <rect x="296" y="118" width="120" height="54" rx="2" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="356" y="142" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="var(--text)">AVTransport</text>
  <text x="356" y="159" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-muted)">play / stop / seek</text>

  <rect x="421" y="118" width="120" height="54" rx="2" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="481" y="142" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="var(--text)">RenderingControl</text>
  <text x="481" y="159" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-muted)">volume / mute</text>

  <rect x="546" y="118" width="120" height="54" rx="2" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="606" y="142" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="var(--text)">ConnectionManager</text>
  <text x="606" y="159" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-muted)">protocol info</text>
</svg>

Every UPnP interaction is some combination of these three calls: <em>find me devices</em>, <em>tell me about yourself</em>, <em>do this thing</em>. SSDP handles the first, HTTP the second, SOAP the third. The rest of this document is detail under each of those three.
</div>

### Hierarchical Device Model

Devices compose hierarchically. A *root device* is the top of the tree; it may contain zero or more *embedded devices*, each of which may have its own services. The root device descriptor XML enumerates all embedded devices inline. In practice, a standalone MediaRenderer (the type tutti targets) is usually a root device with no embedded devices, carrying three services directly: AVTransport, RenderingControl, and ConnectionManager.

The device type URN follows the pattern `urn:schemas-upnp-org:device:<type>:<version>`, e.g. `urn:schemas-upnp-org:device:MediaRenderer:1`. Service type URNs follow the same pattern with `service` in place of `device`.

### Service Descriptions

Every service has a *SCPD* (Service Control Protocol Description) document at its SCPDURL — a second GET, separate from the device descriptor. The SCPD lists every action the service accepts, every input/output argument, and every state variable. State variables define the type, allowed value list, and (where applicable) allowed range for each datum the service tracks. Actions reference state variables for their argument semantics.

---

## SSDP Discovery Layer

SSDP (Simple Service Discovery Protocol) is UPnP's device-advertisement and search mechanism. It runs over UDP using HTTP-like framing (sometimes called HTTPU — HTTP over UDP), described in an IETF internet-draft (draft-cai-ssdp-v1). It is not HTTP/1.1 proper: no persistent connection, no TCP, no Content-Length guarantee, and the request-URI is always `*`.

- **Multicast group**: `239.255.255.250` (IPv4 link-local scope administrative)
- **Port**: 1900/UDP

### M-SEARCH

A control point sends an M-SEARCH to discover devices. The packet is sent to the multicast group. Each matching device replies *unicast* back to the sender's ephemeral port:

```
M-SEARCH * HTTP/1.1\r\n
HOST: 239.255.255.250:1900\r\n
MAN: "ssdp:discover"\r\n
MX: 3\r\n
ST: urn:schemas-upnp-org:device:MediaRenderer:1\r\n
\r\n
```

Key headers:

| Header | Meaning |
|--------|---------|
| `MAN` | Must be `"ssdp:discover"` (quoted). Identifies this as an SSDP extension. |
| `MX` | Max wait seconds before reply. Devices randomize their reply within `[0, MX]` to spread UDP bursts. |
| `ST` | Search target. `ssdp:all` hits everything; `upnp:rootdevice` hits only root devices; a URN hits that specific type. |

The unicast response from a device carries:

```
HTTP/1.1 200 OK\r\n
CACHE-CONTROL: max-age=1800\r\n
LOCATION: http://192.168.1.42:49152/description.xml\r\n
SERVER: Linux/5.15 UPnP/1.0 SomeRenderer/2.1\r\n
ST: urn:schemas-upnp-org:device:MediaRenderer:1\r\n
USN: uuid:12345678-1234-1234-1234-123456789abc::urn:schemas-upnp-org:device:MediaRenderer:1\r\n
EXT:\r\n
BOOTID.UPNP.ORG: 1\r\n
CONFIGID.UPNP.ORG: 42\r\n
\r\n
```

`LOCATION` is the URL for the device descriptor. `USN` (Unique Service Name) is the stable per-(device, role) identifier: a UUID plus `::` plus the ST URN. `EXT` is a required empty header in responses to M-SEARCH.

<div class="cat-explainer">
<svg viewBox="0 0 720 260" role="img" aria-label="SSDP M-SEARCH wire flow. The control point sends one M-SEARCH packet to the multicast group 239.255.255.250 on UDP port 1900. Every device on the LAN that matches the search target replies unicast back to the control point's ephemeral source port.">
  <rect x="20" y="20" width="680" height="220" fill="var(--bg)" stroke="var(--border)" stroke-width="1" rx="3"/>
  <text x="40" y="46" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="var(--text-muted)" letter-spacing="1.4">SSDP M-SEARCH FLOW</text>

  <rect x="280" y="64" width="160" height="50" rx="3" fill="var(--bg)" stroke="var(--accent)" stroke-width="2"/>
  <text x="360" y="84" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="var(--accent)" letter-spacing="1.2">CONTROL POINT</text>
  <text x="360" y="104" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" fill="var(--text)">music client</text>

  <line x1="360" y1="114" x2="360" y2="148" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="360,148 355,138 365,138" fill="var(--accent)"/>
  <text x="372" y="135" font-family="IBM Plex Mono, monospace" font-size="9" fill="var(--text-2)">M-SEARCH</text>

  <line x1="60" y1="158" x2="660" y2="158" stroke="var(--text-muted)" stroke-width="1" stroke-dasharray="4,3"/>

  <rect x="60" y="184" width="150" height="48" rx="3" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="135" y="202" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="8" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">DEVICE</text>
  <text x="135" y="220" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="var(--text)">MediaRenderer</text>

  <rect x="285" y="184" width="150" height="48" rx="3" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="360" y="202" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="8" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">DEVICE</text>
  <text x="360" y="220" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="var(--text)">MediaServer</text>

  <rect x="510" y="184" width="150" height="48" rx="3" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="585" y="202" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="8" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">DEVICE</text>
  <text x="585" y="220" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="var(--text)">Printer</text>

  <path d="M 135 184 V 152 A 6 6 0 0 1 141 146 H 294 A 6 6 0 0 0 300 140 V 124" fill="none" stroke="var(--text-muted)" stroke-width="1.5" stroke-dasharray="3,2"/>
  <polygon points="300,114 295,124 305,124" fill="var(--text-muted)"/>

  <line x1="330" y1="184" x2="330" y2="124" stroke="var(--text-muted)" stroke-width="1.5" stroke-dasharray="3,2"/>
  <polygon points="330,114 325,124 335,124" fill="var(--text-muted)"/>

  <rect x="163" y="122" width="110" height="16" fill="var(--bg)"/>
  <text x="218" y="134" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="8" fill="var(--text-muted)">unicast 200 OK replies</text>
  <text x="660" y="173" text-anchor="end" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="var(--text-muted)" letter-spacing="1.2">MULTICAST  239.255.255.250 : UDP 1900</text>
</svg>

The M-SEARCH goes out once. The replies come back <em>unicast</em>, each device speaking to the control point's ephemeral source port directly. A printer is on the bus and hears the same packet; if the ST didn't match its type, it stays silent. Devices randomise their reply within <code>[0, MX]</code> seconds so a hundred-device LAN doesn't burst-reply in the same microsecond.
</div>

### NOTIFY

Devices also *push* announcements unsolicited via NOTIFY on the multicast group. Two subtypes:

**ssdp:alive** — device is up:
```
NOTIFY * HTTP/1.1\r\n
HOST: 239.255.255.250:1900\r\n
NT: urn:schemas-upnp-org:device:MediaRenderer:1\r\n
NTS: ssdp:alive\r\n
USN: uuid:12345678-1234-1234-1234-123456789abc::urn:schemas-upnp-org:device:MediaRenderer:1\r\n
LOCATION: http://192.168.1.42:49152/description.xml\r\n
CACHE-CONTROL: max-age=1800\r\n
SERVER: Linux/5.15 UPnP/1.0 SomeRenderer/2.1\r\n
BOOTID.UPNP.ORG: 1\r\n
CONFIGID.UPNP.ORG: 42\r\n
\r\n
```

**ssdp:byebye** — device is shutting down:
```
NOTIFY * HTTP/1.1\r\n
HOST: 239.255.255.250:1900\r\n
NT: urn:schemas-upnp-org:device:MediaRenderer:1\r\n
NTS: ssdp:byebye\r\n
USN: uuid:12345678-1234-1234-1234-123456789abc::urn:schemas-upnp-org:device:MediaRenderer:1\r\n
BOOTID.UPNP.ORG: 1\r\n
\r\n
```

No one sends a response to a NOTIFY. Caches expire entries per `CACHE-CONTROL: max-age`.

### BOOTID and CONFIGID

`BOOTID.UPNP.ORG` increments each time the device reboots. A control point that sees a changed BOOTID knows the device's service URLs may have changed and must re-fetch the descriptor. `CONFIGID.UPNP.ORG` increments when the device's configuration changes without a reboot. These headers were introduced in UPnP-DA 1.1.

### HTTPU Framing

HTTPU uses the same line-oriented text format as HTTP/1.1 — method line, CRLF-terminated headers, blank line — but over a single UDP datagram with no connection state, no chunked encoding, and no persistent session. Implementations must tolerate devices that use LF instead of CRLF, lowercase header names, or missing reason phrases on the status line. tutti's parser is deliberately lenient on all three.

---

## Device Descriptor Fetch

Once a LOCATION URL is in hand, the control point issues a plain HTTP GET:

```
GET /description.xml HTTP/1.1
Host: 192.168.1.42:49152
```

The response body is an XML document in the `urn:schemas-upnp-org:device-1-0` namespace. A minimal MediaRenderer descriptor looks like:

```xml
<?xml version="1.0" encoding="utf-8"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <deviceType>urn:schemas-upnp-org:device:MediaRenderer:1</deviceType>
    <friendlyName>Living Room Renderer</friendlyName>
    <manufacturer>ACME</manufacturer>
    <manufacturerURL>http://acme.example/</manufacturerURL>
    <modelDescription>Hi-Res Renderer</modelDescription>
    <modelName>ACME-9000</modelName>
    <UDN>uuid:12345678-1234-1234-1234-123456789abc</UDN>
    <X_DLNADOC xmlns="urn:schemas-dlna-org:device-1-0">DMR-1.50</X_DLNADOC>
    <presentationURL>http://192.168.1.42/</presentationURL>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:AVTransport:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:AVTransport</serviceId>
        <SCPDURL>/avt/scpd.xml</SCPDURL>
        <controlURL>/avt/control</controlURL>
        <eventSubURL>/avt/event</eventSubURL>
      </service>
      <service>
        <serviceType>urn:schemas-upnp-org:service:RenderingControl:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:RenderingControl</serviceId>
        <SCPDURL>/rc/scpd.xml</SCPDURL>
        <controlURL>/rc/control</controlURL>
        <eventSubURL>/rc/event</eventSubURL>
      </service>
      <service>
        <serviceType>urn:schemas-upnp-org:service:ConnectionManager:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:ConnectionManager</serviceId>
        <SCPDURL>/cm/scpd.xml</SCPDURL>
        <controlURL>/cm/control</controlURL>
        <eventSubURL>/cm/event</eventSubURL>
      </service>
    </serviceList>
  </device>
</root>
```

Key fields:

- `UDN`: The device's stable UUID-based identifier, matching the UUID portion of every USN this device emits. Format: `uuid:<RFC-4122-UUID>`.
- `X_DLNADOC`: Vendor-extension element in the `urn:schemas-dlna-org:device-1-0` namespace declaring the DLNA device class and version. Discussed in the DLNA section below.
- `presentationURL`: A browseable UI URL, if the device provides one.
- `controlURL` / `eventSubURL` / `SCPDURL`: All may be relative or absolute URLs. Relative URLs are resolved against the LOCATION base.

For a root device with embedded devices, each `<device>` block appears inside a `<deviceList>` child element; the structure recurses. tutti's scope is flat MediaRenderers, so embedded device parsing is not exercised.

---

## Service Descriptions

Each service's SCPDURL returns an SCPD document. This is another XML namespace (`urn:schemas-upnp-org:service-1-0`) listing:

- `<actionList>`: Each `<action>` has a name and zero or more `<argument>` elements, each with a name, direction (`in`/`out`), and `relatedStateVariable`.
- `<serviceStateTable>`: Each `<stateVariable>` has a name, data type, optional allowed value list or range, and a `sendEvents` attribute (whether GENA publishes changes to this variable).

Action arguments get their type and constraints from their related state variable, not directly. This indirection means a control point must cross-reference the action list against the state table to know what values are legal for a given argument.

SCPDs are fetched once and cached. In practice tutti does not exercise SCPD fetching for audio-streaming decisions — it goes directly to SOAP calls against AVTransport and ConnectionManager — but the spec requires SCPD availability.

---

## SOAP Control

UPnP action invocations use SOAP 1.1 (via HTTP POST to the service's `controlURL`). The `SOAPAction` header specifies the service type URN and action name, separated by `#`, enclosed in double quotes:

```
POST /avt/control HTTP/1.1
Host: 192.168.1.42:49152
Content-Type: text/xml; charset="utf-8"
SOAPAction: "urn:schemas-upnp-org:service:AVTransport:1#Play"
Content-Length: ...
```

The body is a SOAP 1.1 envelope. The action element lives inside `s:Body` in the service's namespace:

```xml
<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
            s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:Play xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
      <Speed>1</Speed>
    </u:Play>
  </s:Body>
</s:Envelope>
```

The response mirrors the shape with a `<u:PlayResponse>` element (action name plus `Response` suffix):

```xml
<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
            s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:PlayResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/>
  </s:Body>
</s:Envelope>
```

### Fault Structure

On error the device returns HTTP 500 with a SOAP fault:

```xml
<s:Body>
  <s:Fault>
    <faultcode>s:Client</faultcode>
    <faultstring>UPnPError</faultstring>
    <detail>
      <UPnPError xmlns="urn:schemas-upnp-org:control-1-0">
        <errorCode>701</errorCode>
        <errorDescription>Transition not available</errorDescription>
      </UPnPError>
    </detail>
  </s:Fault>
</s:Body>
```

UPnP-DA defines error codes 400–499 (client argument errors) and 500–599 (device-side failures). Each service spec adds its own action-specific error codes above 600. Error code 701 (`TransitionNotImplemented`) from AVTransport is the common case when you issue Play on a device that has no URI loaded.

Implementation note: the double-quoted `SOAPAction` header is mandatory per UPnP-DA. Some stacks silently 500 without the quotes. tutti always emits them.

---

## GENA Eventing

GENA (General Event Notification Architecture) is UPnP's push-event mechanism. A control point subscribes to a service's event URL with an HTTP SUBSCRIBE request:

```
SUBSCRIBE /avt/event HTTP/1.1
Host: 192.168.1.42:49152
NT: upnp:event
Callback: <http://192.168.1.100:54321/notify>
Timeout: Second-1800
```

The device replies with:

```
HTTP/1.1 200 OK
SID: uuid:subscription-uuid-here
Timeout: Second-1800
```

`SID` is a subscription ID the control point uses in subsequent RENEW requests. The device then POSTs NOTIFY messages to the `Callback` URL whenever a state variable with `sendEvents="yes"` changes:

```
NOTIFY /notify HTTP/1.1
Host: 192.168.1.100:54321
NT: upnp:event
NTS: upnp:propchange
SID: uuid:subscription-uuid-here
SEQ: 0
Content-Type: text/xml; charset="utf-8"
Content-Length: ...

<?xml version="1.0" encoding="utf-8"?>
<e:propertyset xmlns:e="urn:schemas-upnp-org:event-1-0">
  <e:property>
    <TransportState>PLAYING</TransportState>
  </e:property>
</e:propertyset>
```

`SEQ` is a monotonically increasing sequence number starting at 0 for the initial event sent on subscription, allowing the control point to detect out-of-order or missing NOTIFY deliveries.

**tutti does not use GENA.** Instead it polls GetTransportInfo and GetPositionInfo on a 3-second tick. Polling avoids the need for a reachable callback HTTP server on the control point — which is a real constraint for a LAN probe tool that might run from a laptop behind a firewall or network namespace boundary.

---

## DIDL-Lite Metadata

When setting a URI on a renderer via SetAVTransportURI, the control point passes a `CurrentURIMetaData` argument containing a DIDL-Lite document (Digital Item Declaration Language Lite, defined in the UPnP AV Media architecture). DIDL-Lite is itself an XML document — embedded as escaped XML inside a SOAP argument, which is itself embedded inside a SOAP envelope. Three levels of XML nesting.

A typical DIDL-Lite blob:

```xml
<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"
           xmlns:dc="http://purl.org/dc/elements/1.1/"
           xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">
  <item id="1" parentID="0" restricted="1">
    <dc:title>tutti probe: flac-44100-16-stereo</dc:title>
    <upnp:artist>tutti</upnp:artist>
    <dc:creator>tutti</dc:creator>
    <upnp:album>LAN Probe</upnp:album>
    <upnp:class>object.item.audioItem.musicTrack</upnp:class>
    <res protocolInfo="http-get:*:audio/flac:*">
      http://192.168.1.100:8765/probe.flac
    </res>
    <upnp:albumArtURI>http://192.168.1.100:8765/art.jpg</upnp:albumArtURI>
  </item>
</DIDL-Lite>
```

When this blob is placed in the SOAP argument, every `<`, `>`, and `&` inside it must be XML-escaped — turning `<DIDL-Lite` into `&lt;DIDL-Lite`, and so on. If the DIDL-Lite itself contains URLs with `&` query parameters, those `&` characters get double-escaped: `&amp;` in the DIDL-Lite becomes `&amp;amp;` in the SOAP envelope. This is correct XML but produces spectacular-looking raw SOAP bodies.

Key metadata elements:

| Element | Namespace | Purpose |
|---------|-----------|---------|
| `dc:title` | Dublin Core | Track title |
| `dc:creator` | Dublin Core | Artist (legacy field; many renderers prefer this) |
| `upnp:artist` | UPnP AV | Artist (modern field) |
| `upnp:album` | UPnP AV | Album name |
| `upnp:class` | UPnP AV | Object class hierarchy; `object.item.audioItem.musicTrack` is the standard leaf for music |
| `upnp:albumArtURI` | UPnP AV | URL of cover art; renderer fetches it independently |
| `res` | DIDL-Lite | Resource element: the actual media URL, with `protocolInfo` attribute |

The `res` element's `protocolInfo` attribute uses the four-field format described in the ConnectionManager section. The `item` element's `id` and `parentID` are container-model identifiers from the UPnP MediaServer content directory; for a control point passing a bare URL they are arbitrary strings. `restricted="1"` means the item cannot be modified via the ContentDirectory service.

---

## AVTransport Service

AVTransport (defined in the UPnP AV AVTransport service specification) is the service that controls playback. Every call includes `InstanceID` — nominally supporting multiple concurrent streams per renderer, but virtually every device only implements instance 0.

### State Machine

The transport state variable `TransportState` progresses through:

<div class="cat-explainer">
<svg viewBox="0 0 720 240" role="img" aria-label="AVTransport state machine. Five states: NO_MEDIA_PRESENT, STOPPED, TRANSITIONING, PLAYING, PAUSED_PLAYBACK. Setting a URI moves a device out of NO_MEDIA_PRESENT to STOPPED. Play moves it to TRANSITIONING while buffering, then PLAYING. Stop or end-of-stream return it to STOPPED. Pause toggles between PLAYING and PAUSED_PLAYBACK.">
  <rect x="20" y="20" width="680" height="200" fill="var(--bg)" stroke="var(--border)" stroke-width="1" rx="3"/>
  <text x="40" y="46" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="var(--text-muted)" letter-spacing="1.4">AVTRANSPORT STATE MACHINE</text>

  <rect x="40" y="74" width="120" height="56" rx="3" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="100" y="96" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="var(--text)">NO_MEDIA</text>
  <text x="100" y="110" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="var(--text)">_PRESENT</text>
  <text x="100" y="124" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-muted)">no URI set</text>

  <line x1="160" y1="102" x2="220" y2="102" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="220,102 210,97 210,107" fill="var(--accent)"/>
  <text x="190" y="95" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="var(--text-2)">SetURI</text>

  <rect x="220" y="74" width="120" height="56" rx="3" fill="var(--bg)" stroke="var(--accent)" stroke-width="2"/>
  <text x="280" y="100" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="11" font-weight="700" fill="var(--accent)">STOPPED</text>
  <text x="280" y="118" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-muted)">safe to set URI</text>

  <line x1="340" y1="102" x2="400" y2="102" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="400,102 390,97 390,107" fill="var(--accent)"/>
  <text x="370" y="95" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="var(--text-2)">Play</text>

  <rect x="400" y="74" width="120" height="56" rx="3" fill="var(--bg)" stroke="var(--accent)" stroke-width="2"/>
  <text x="460" y="98" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="11" font-weight="700" fill="var(--accent)">TRANSI-</text>
  <text x="460" y="112" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="11" font-weight="700" fill="var(--accent)">TIONING</text>
  <text x="460" y="124" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-muted)">buffering</text>

  <line x1="520" y1="102" x2="580" y2="102" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="580,102 570,97 570,107" fill="var(--accent)"/>
  <text x="550" y="95" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="var(--text-2)">buffered</text>

  <rect x="580" y="74" width="100" height="56" rx="3" fill="var(--bg)" stroke="var(--accent)" stroke-width="2"/>
  <text x="630" y="100" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="11" font-weight="700" fill="var(--accent)">PLAYING</text>
  <text x="630" y="118" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="var(--text-muted)">active</text>

  <rect x="580" y="160" width="100" height="40" rx="3" fill="var(--surface)" stroke="var(--border)" stroke-width="1"/>
  <text x="630" y="178" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="var(--text)">PAUSED_</text>
  <text x="630" y="192" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="var(--text)">PLAYBACK</text>

  <line x1="618" y1="130" x2="618" y2="158" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="618,158 613,148 623,148" fill="var(--accent)"/>
  <text x="608" y="148" text-anchor="end" font-family="IBM Plex Mono, monospace" font-size="9" fill="var(--text-2)">Pause</text>

  <line x1="642" y1="160" x2="642" y2="132" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="642,132 637,142 647,142" fill="var(--accent)"/>
  <text x="652" y="148" font-family="IBM Plex Mono, monospace" font-size="9" fill="var(--text-2)">Play</text>

  <path d="M 680 112 H 684 A 6 6 0 0 1 690 118 V 206 A 6 6 0 0 1 684 212 H 326 A 6 6 0 0 1 320 206 V 140" fill="none" stroke="var(--accent)" stroke-width="2"/>
  <polygon points="320,130 315,140 325,140" fill="var(--accent)"/>
  <rect x="400" y="190" width="120" height="14" fill="var(--bg)"/>
  <text x="460" y="200" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="var(--text-2)">Stop · end of stream</text>
</svg>

The state machine is the same on every conforming renderer, but the <em>timing</em> isn't: <code>TRANSITIONING</code> can be a millisecond on a Sonos with a small MP3 or several seconds on a slow streamer fetching a large FLAC header. tutti polls <code>GetTransportInfo</code> through this transition so the capture records whatever the device did, however long it took.
</div>

States:
- `STOPPED`: No active playback. Safe to issue SetAVTransportURI.
- `TRANSITIONING`: Device has accepted the Play command and is buffering/connecting to the stream. Duration is device-dependent and network-dependent; can be milliseconds or several seconds for a slow device fetching a large FLAC header.
- `PLAYING`: Active playback.
- `PAUSED_PLAYBACK`: Paused (not all devices implement Pause).
- `NO_MEDIA_PRESENT`: No URI set. Some devices emit this rather than STOPPED before SetAVTransportURI.

### Key Actions

**SetAVTransportURI**: Sets the media URI and its metadata. Must be called before Play. Arguments: `InstanceID`, `CurrentURI`, `CurrentURIMetaData` (DIDL-Lite blob, XML-escaped).

**Play**: Starts playback. Arguments: `InstanceID`, `Speed` (always `"1"` in practice; other values are spec-allowed but no renderer honors them usefully).

**Stop**: Stops playback and returns to STOPPED. Arguments: `InstanceID`.

**Pause**: Pauses playback. Not all renderers implement this; check the SCPD action list. Arguments: `InstanceID`.

**Seek**: Seeks to a position. Arguments: `InstanceID`, `Unit` (e.g. `REL_TIME`, `ABS_TIME`, `TRACK_NR`), `Target` (position string matching the unit). Renderers frequently implement Seek only for `REL_TIME` and return a fault for other modes.

**GetTransportInfo**: Returns current state. Response fields: `CurrentTransportState`, `CurrentTransportStatus` (OK/ERROR_OCCURRED), `CurrentSpeed`.

**GetPositionInfo**: Returns playback position and track metadata. Response fields include `Track`, `TrackDuration`, `TrackMetaData` (a DIDL-Lite blob the device echoes back, possibly rewritten from its own container parse), `TrackURI`, `RelTime`, `AbsTime`, `RelCount`, `AbsCount`. `TrackMetaData` is the field tutti watches to determine whether the renderer honored the DIDL-Lite metadata it received or replaced it with its own container-parsed version.

---

## RenderingControl Service

RenderingControl manages audio output parameters. The key actions:

- **GetVolume** / **SetVolume**: Volume is in the range `[0, 100]` for channel `Master`. Some devices expose separate `LF`/`RF` channels.
- **GetMute** / **SetMute**: Boolean mute state per channel.
- **ListPresets** / **SelectPreset**: Named presets (e.g. `FactoryDefaults`). Most renderers list one preset and SelectPreset just resets volume/mute.

RenderingControl is outside tutti's primary measurement path. tutti does not issue RenderingControl calls during a probe run — the goal is passively observing what the device accepts, not altering its output state.

---

## ConnectionManager Service

ConnectionManager (defined in the UPnP AV ConnectionManager service specification) is the protocol-negotiation layer. Its primary action for a control point is:

**GetProtocolInfo**: Returns two comma-separated lists:
- `Source`: what this device can push (typically empty for a renderer)
- `Sink`: what this device can receive and play

Each entry in both lists is a four-field colon-separated string:

```
<schema>:<network>:<mime-type>:<additional-info>
```

For an HTTP-served audio stream:
```
http-get:*:audio/flac:*
http-get:*:audio/mpeg:DLNA.ORG_PN=MP3;DLNA.ORG_OP=01;DLNA.ORG_FLAGS=01700000000000000000000000000000
http-get:*:audio/L16;rate=44100;channels=2:*
```

Fields:
- `schema`: transport scheme. `http-get` for HTTP GET (the standard case). `rtsp-rtp-udp`, `internal`, etc. exist but are rarely seen in practice.
- `network`: network interface; always `*` in practice (any network).
- `mime-type`: MIME type, possibly with parameters (semicolons are significant here and are not field separators).
- `additional-info`: DLNA profile string or `*` if DLNA extensions are not declared.

### ConnectionIDs

ConnectionManager also defines PrepareForConnection / ConnectionComplete for formal connection negotiation, returning a `ConnectionID` used to associate a connection with an AVTransport instance. In practice, MediaRenderers that support only a single stream (instance 0) omit PrepareForConnection from their SCPD or return an error when called. Control points targeting such renderers use ConnectionID 0 and InstanceID 0 unconditionally.

---

## DLNA Layered on UPnP

DLNA (Digital Living Network Alliance) is a certification layer on top of UPnP AV. DLNA-certified devices declare a device class in `X_DLNADOC`:

| Class | Role |
|-------|------|
| `DMS-1.50` | Digital Media Server |
| `DMR-1.50` | Digital Media Renderer |
| `DMP-1.50` | Digital Media Player (combined server+renderer) |
| `M-DMR` | Mobile DMR variant |

`X_DLNADOC` appears as a vendor-extension element in the device descriptor (namespace `urn:schemas-dlna-org:device-1-0`). A device may declare multiple classes by including multiple `X_DLNADOC` elements.

### protocolInfo Extensions

DLNA's most consequential addition is a structured vocabulary for the `additional-info` field of protocolInfo entries. Four DLNA parameters appear there, semicolon-separated:

**DLNA.ORG_PN** (Profile Name): A token identifying the exact media profile, encoding parameters, and container. Examples: `MP3`, `LPCM`, `FLAC`, `AAC_ISO_320`. Profile names are normative in the DLNA Guidelines and specify exact constraints (bitrate, sampling rate, container). If present, the device is declaring it knows specifically how to render that profile, not just the MIME type.

**DLNA.ORG_OP** (Operations): A two-hex-digit bitmask of supported seek operations for this format. Bit 1 (value `01`) = time-seek supported (Range header with byte offsets derived from duration); bit 0 (value `10`) = byte-range supported (standard HTTP Range). `DLNA.ORG_OP=01` means time-seek, `DLNA.ORG_OP=11` means both. `00` means neither. Affects whether a control point can seek mid-stream on HTTP.

**DLNA.ORG_CI** (Conversion Indicator): `0` = content is native (no transcoding), `1` = content is transcoded. Allows a media server to advertise that a format entry represents a real-time transcode path.

**DLNA.ORG_FLAGS**: A 32-hex-digit bitmask (128 bits, padded to 32 chars). The leading bits govern stream mode:

| Bit position (of leading byte) | Meaning |
|--------------------------------|---------|
| Bit 31 | Sender-paced (background transfer) |
| Bit 30 | Limited Operations — time seek supported |
| Bit 29 | Limited Operations — byte seek supported |
| Bit 28 | Play container |
| Bit 27 | S0 increasing (DLNA content features) |
| Bit 26 | SN increasing |
| Bit 25 | RTSP pause supported |
| Bit 24 | Streaming transfer mode supported |
| Bit 23 | Interactive transfer mode supported |
| Bit 22 | Background transfer mode supported |

A typical value seen from a real renderer: `01700000000000000000000000000000`, which breaks down as: byte 0 = `01` (bit 24 set: streaming mode), byte 1 = `70` (bits 30, 29, 28 set: time-seek + byte-seek + play-container).

A complete protocolInfo sink entry from a DLNA renderer:
```
http-get:*:audio/mpeg:DLNA.ORG_PN=MP3;DLNA.ORG_OP=01;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=01700000000000000000000000000000
```

### Transfer Mode

DLNA adds a `transferMode.dlna.org` HTTP request header for the actual media GET:
- `Streaming` — the renderer expects the server to stream the content (normal audio playback)
- `Interactive` — short requests for UI thumbnails etc.
- `Background` — large background downloads

A DLNA server checks this header and adjusts buffering. A control point that supplies a media URL should be prepared for the renderer to include this header on its GET to the content server.

### Conformance Testing

The DLNA Conformance Test (CTT) verifies interoperability against the DLNA Guidelines. In practice, many consumer devices ship with quirks that a strict CTT would flag — misformatted protocolInfo, missing `EXT:` header in SSDP responses, non-standard state machine behavior on format rejection — while still interoperating with popular control points that are equally lenient.

---

## Control Point: End-to-End Flow

Putting the layers together, here is the flow tutti executes for a probe run:

### 1. Discovery (SSDP)

Send M-SEARCH for `ssdp:all`, `upnp:rootdevice`, `urn:schemas-upnp-org:device:MediaRenderer:1`, and `urn:schemas-upnp-org:service:AVTransport:1`. Collect unicast responses for MX+2 seconds. Deduplicate by (USN, LOCATION). Extract the set of unique LOCATION URLs.

### 2. Describe (Device Descriptor + ConnectionManager)

For each LOCATION: GET the descriptor XML. Parse out `friendlyName`, `UDN`, `X_DLNADOC`, and the service list. For each service, record `serviceType`, `controlURL`, and `eventSubURL`.

If the device has a ConnectionManager service, SOAP-call GetProtocolInfo. Parse the Sink list into a structured set of `(schema, mime, dlna-params)` tuples. This is the *announced capability* — what the device says it can play. It is not always accurate: devices mislist formats, list DLNA profiles they cannot actually decode, or omit formats they accept without declaring.

### 3. Decide (Library Analysis)

Cross-reference the Sink list against the local test-tone corpus (or the user's library). Build a matrix of `(tone, format)` pairs where the format appears in the Sink list. Rank by quality tier. This is where tutti's decision layer lives — it is not a protocol artifact but a local policy.

### 4. Command (AVTransport Drive)

Pre-check: call GetTransportInfo. If `CurrentTransportState` is `PLAYING` and `--force` is not set, abort to avoid interrupting active listening.

For each candidate tone:
1. Serve the tone file from an ephemeral local HTTP server.
2. Build DIDL-Lite with a sentinel title (`tutti probe: <scenario>`).
3. Call SetAVTransportURI with the tone URL and DIDL-Lite payload.
4. Call Play (InstanceID 0, Speed 1).
5. Poll GetTransportInfo and GetPositionInfo every 3 seconds for 12 seconds. Record `CurrentTransportState` transitions.
6. Inspect `TrackMetaData` in GetPositionInfo responses. If it echoes the sentinel title prefix, the renderer honored the DIDL-Lite. If it echoes the embedded file metadata (a different sentinel), the renderer read the container directly. If neither, the renderer dropped or rewrote the metadata.
7. Emit Stop. Settle 500ms.

### 5. Poll and Record

At the end of each tone run, classify the result as `playing`, `transitioning`, `stopped`, or `errored` based on observed state transitions. Write a structured transcript (SOAP bodies, state sequence, echoed metadata) alongside a JSON manifest. The manifest is the paste-ready evidence artifact.

### Where Each Protocol Layer Fits

| Phase | Protocol |
|-------|---------|
| Discovery | SSDP over UDP/1900 multicast |
| Descriptor fetch | HTTP GET to LOCATION URL (UPnP device XML namespace) |
| Capability query | SOAP over HTTP (ConnectionManager:GetProtocolInfo) |
| Media command | SOAP over HTTP (AVTransport:SetAVTransportURI, Play, Stop) |
| State polling | SOAP over HTTP (AVTransport:GetTransportInfo, GetPositionInfo) |
| Media delivery | Plain HTTP GET from the renderer to tutti's ephemeral server |
| Metadata round-trip | DIDL-Lite inside SOAP, echoed back in GetPositionInfo:TrackMetaData |
| Eventing | GENA (not used by tutti; polling substitutes) |
