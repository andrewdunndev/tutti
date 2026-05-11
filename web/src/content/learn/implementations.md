---
title: "Implementations"
eyebrow: "Catalog"
description: "Renderer-side stacks, control-point libraries, end-user clients, and bridges. Where to look for source-of-truth behavior."
order: 3
---


Reference for tutti renderer profiling: "what's the device built on / what library does this client use / where to look for source-of-truth behavior."

<div class="cat-explainer">
<svg viewBox="0 0 720 220" role="img" aria-label="The four categories of code involved in a UPnP audio stream. A renderer-side stack lives inside the playback device and accepts SOAP. A control-point library lives inside the client app and issues SOAP. An end-user client is the app the listener actually touches. A bridge or server transmutes content from a non-UPnP source into something the renderer can play.">
  <rect x="20" y="20" width="680" height="180" fill="#ffffff" stroke="#e0e0e0" stroke-width="1" rx="3"/>
  <text x="40" y="46" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="#707070" letter-spacing="1.4">FOUR CATEGORIES OF CODE IN A UPNP STREAM</text>

  <rect x="40" y="68" width="144" height="116" rx="3" fill="#ffffff" stroke="#4a6741" stroke-width="2"/>
  <text x="112" y="86" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="8" font-weight="700" fill="#4a6741" letter-spacing="1.2">CATEGORY 1</text>
  <text x="112" y="106" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="12" font-weight="700" fill="#0a0a0a">renderer stack</text>
  <text x="112" y="126" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">inside the device</text>
  <text x="112" y="140" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">answers SOAP</text>
  <text x="112" y="166" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="#707070" letter-spacing="0.6">Platinum, Sonos</text>
  <text x="112" y="178" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="#707070" letter-spacing="0.6">jUPnP, AirPlay</text>

  <rect x="198" y="68" width="144" height="116" rx="3" fill="#ffffff" stroke="#4a6741" stroke-width="2"/>
  <text x="270" y="86" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="8" font-weight="700" fill="#4a6741" letter-spacing="1.2">CATEGORY 2</text>
  <text x="270" y="106" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="12" font-weight="700" fill="#0a0a0a">control-point lib</text>
  <text x="270" y="126" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">inside the client app</text>
  <text x="270" y="140" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">issues SOAP</text>
  <text x="270" y="166" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="#707070" letter-spacing="0.6">go-upnpcast,</text>
  <text x="270" y="178" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="#707070" letter-spacing="0.6">JUPnP, Cling</text>

  <rect x="356" y="68" width="144" height="116" rx="3" fill="#ffffff" stroke="#4a6741" stroke-width="2"/>
  <text x="428" y="86" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="8" font-weight="700" fill="#4a6741" letter-spacing="1.2">CATEGORY 3</text>
  <text x="428" y="106" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="12" font-weight="700" fill="#0a0a0a">end-user client</text>
  <text x="428" y="126" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">the app the user</text>
  <text x="428" y="140" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">actually touches</text>
  <text x="428" y="166" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="#707070" letter-spacing="0.6">Symfonium,</text>
  <text x="428" y="178" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="#707070" letter-spacing="0.6">BubbleUPnP</text>

  <rect x="514" y="68" width="144" height="116" rx="3" fill="#ffffff" stroke="#4a6741" stroke-width="2"/>
  <text x="586" y="86" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="8" font-weight="700" fill="#4a6741" letter-spacing="1.2">CATEGORY 4</text>
  <text x="586" y="106" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="12" font-weight="700" fill="#0a0a0a">bridge / server</text>
  <text x="586" y="126" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">re-serves content as</text>
  <text x="586" y="140" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">a UPnP source</text>
  <text x="586" y="166" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="#707070" letter-spacing="0.6">Plex DLNA,</text>
  <text x="586" y="178" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="#707070" letter-spacing="0.6">upmpdcli, MiniDLNA</text>
</svg>

When a bug shows up <em>only</em> in a particular app, the question is which of the four categories it lives in. A descriptor that fails to parse points at category 2 (the control-point library inside the client). A device that <em>accepts</em> a stream but then plays it wrong points at category 1 (the renderer stack). Different categories, different fixes, different upstreams.
</div>

---

## Renderer-side stacks

<div class="cat-explainer">
<svg viewBox="0 0 720 200" role="img" aria-label="Renderer-side stack: software inside the playback device that accepts SOAP calls and drives the DAC.">
  <rect x="20" y="20" width="680" height="160" fill="#ffffff" stroke="#e0e0e0" stroke-width="1" rx="3"/>
  <text x="40" y="46" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="#707070" letter-spacing="1.4">SOMEONE'S DEVICE</text>

  <rect x="40" y="60" width="120" height="100" rx="3" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="100" y="90"  text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">network</text>
  <text x="100" y="110" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">SOAP arrives</text>
  <text x="100" y="125" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">SetAVTransportURI</text>

  <line x1="160" y1="110" x2="220" y2="110" stroke="#4a6741" stroke-width="2"/>
  <polygon points="220,110 210,105 210,115" fill="#4a6741"/>

  <rect x="220" y="60" width="220" height="100" rx="3" fill="#ffffff" stroke="#4a6741" stroke-width="2"/>
  <text x="330" y="86"  text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="#4a6741" letter-spacing="1.4">RENDERER STACK</text>
  <text x="330" y="110" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="13" font-weight="700" fill="#4a6741">this category</text>
  <text x="330" y="130" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">SOAP server · state machine</text>
  <text x="330" y="146" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">stream fetch · decoder glue</text>

  <line x1="440" y1="110" x2="500" y2="110" stroke="#707070" stroke-width="1.5" stroke-dasharray="4,3"/>
  <polygon points="500,110 490,105 490,115" fill="#707070"/>

  <rect x="500" y="60" width="180" height="100" rx="3" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="590" y="90"  text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">decoder + DAC</text>
  <text x="590" y="110" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">vendor's audio path</text>
  <text x="590" y="125" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">analog out</text>
</svg>

The compiled-into-firmware part. You don't usually see this code; it ships baked into the device. When a renderer accepts a UPnP cast, this is what answered the SOAP call.
</div>

These run on the playback device and implement the UPnP AV MediaRenderer device type. They handle incoming SetAVTransportURI/Play/Seek SOAP calls and produce audio.

### Platinum SDK (plutinosoft)

C++ UPnP SDK implementing MediaServer, MediaRenderer, and Control Point. Written by Sylvain Rebaud at Plutinosoft, LLC. Dual-licensed: GPL v2+ for open-source users, commercial license for OEMs. The SDK wraps Neptune, a BSD-licensed C++ runtime.

Kodi (XBMC) vendors Platinum directly under `lib/libUPnP/Platinum/` and uses it for both server and renderer/control-point roles. It is also the dominant embedded SDK in third-party set-top boxes, NAS appliances, and network streamers from smaller OEMs. Plex's early UPnP layers also descended from Plutinosoft work.

Known quirks: Platinum's `GetProtocolInfo` response format has historically been non-standard. Clients parsing ProtocolInfo strictly (e.g. some strict BubbleUPnP modes) encounter mismatches. The Kodi fork carries local patches that diverge from upstream, so bugs reproduced in Kodi may not reproduce in vanilla Platinum.

State: Sporadic maintenance; last public activity was 2023–2024. No regular release cadence. CLA required for contributions.

```
https://github.com/plutinosoft/Platinum
```

---

### libupnp / pupnp

The original Intel-authored "Portable SDK for UPnP Devices," now maintained as `pupnp`. Written in C. BSD license. Implements UPnP Device Architecture 1.0 including SSDP, GENA, and SOAP over HTTP. Single-threaded event loop per POSIX threads.

Used as the renderer stack base by: VLC (services discovery module `upnp.cpp`), and historically by a large number of set-top boxes. MPD's UPnP browsing and upmpdcli's predecessor dependency chain traced back here before upmpdcli migrated to libnpupnp.

Known quirks: threading model requires care; re-entrant callback issues on some platforms have generated hard-to-reproduce bugs. The 1.14.x maintenance branch is actively patched; master receives new features. Last release: 1.14.x series, March 2026.

State: Active.

```
https://github.com/pupnp/pupnp
```

---

### gUPnP (GNOME)

C/GLib object-oriented UPnP framework implementing device/control-point roles. LGPL v2.1+. Integrates with GLib main loop and libsoup for HTTP. Single-threaded async model. Part of the GNOME stack.

Used by: Rygel (the reference GNOME DLNA media server/renderer), and any GTK-based media application on Linux wanting UPnP. The gupnp-av and gupnp-dlna sub-libraries extend it for DLNA profile handling.

Known quirks: no documented renderer parser quirks. Rygel built on gUPnP is considered a reference implementation for sink-list (ProtocolInfo) formatting — if a server sends something Rygel rejects, the server is likely wrong.

State: Active. Canonical source on GNOME GitLab; GitHub is a read-only mirror.

```
https://gitlab.gnome.org/GNOME/gupnp
```

---

### CyberLink for Java (CyberGarage)

Pure-Java UPnP stack authored by Satoshi Konno at CyberGarage. Last release on SourceForge: 2007. Implements UPnP DA 1.0 for both device and control-point. Historical interest only: it appeared in early Java-based DLNA renderers and was embedded in some NAS firmware when Java ME was viable.

No active development. The cybergarage-upnp GitHub repo is a mirror but receives no commits. Maven artifact `org.cybergarage.cyberlink` exists but no modern projects should depend on it.

State: Abandoned.

```
https://github.com/cybergarage/cybergarage-upnp
```

---

### jUPnP (as renderer)

Java/OSGi UPnP library. Apache License 2.0. Active fork of 4thline/cling (see Control Points section). Requires Java 11+. When used renderer-side (rare), it provides the same AVTransport service scaffolding as on the control-point side. More commonly seen embedded in smart home hubs (openHAB) acting as both renderer and control point within the same JVM.

State: Active. Latest release in the 3.x series (2024–2025). Maintained by the jupnp GitHub organization with openHAB as the primary driver.

```
https://github.com/jupnp/jupnp
```

---

### Sonos proprietary stack

Closed source. Sonos devices expose a standard UPnP/SSDP announcement on port 1900 and a SOAP service on port 1400. The SOAP service implements a superset of AVTransport and RenderingControl with Sonos-specific extensions. Community reverse-engineering is documented at `sonos.svrooij.io` (scraped from device XML).

Key differences from standard UPnP: SetAVTransportURI accepts Sonos-specific metadata extensions for grouping and gapless playback. GetProtocolInfo returns a Sonos-specific ProtocolInfo list. Devices operate as a zone mesh; sending to one device routes to the zone coordinator. Direct UPnP control of a non-coordinator yields unexpected behavior.

Quirks: `GetProtocolInfo` response includes non-standard MIME types. Clients that rely on strict AVTransport compliance without Sonos-specific awareness (like vanilla go-upnpcast) may have partial success — transport commands work, but gapless and multi-room fail.

State: Closed, proprietary. Actively developed.

```
https://github.com/svrooij/sonos-api-docs   (community docs)
https://docs.sonos.com/docs/soap-requests-and-responses   (official)
```

---

### Yamaha MusicCast

Closed source. Yamaha network receivers (RX-A/R-N series) expose both a UPnP AVTransport renderer (Digital Media Renderer, DMR) and a proprietary Yamaha Extended Control (YXC) protocol over HTTP. DLNA conformance is certified but limited.

Known quirks: No seek-within-track via AVTransport in many firmware versions — the DLNA renderer implements playback but not position seek. Symfonium's support forum documents a "track progress not updated" issue specific to MusicCast devices when driven over UPnP. Renderer discovery uses nested device descriptions that caused go-upnpcast pre-0.1.0 to miss these devices (fixed in the "Fix MediaRenderer discovery for nested devices" release).

State: Closed, firmware maintained by Yamaha.

---

### Denon HEOS

Closed source. Denon/Marantz AVRs with HEOS expose UPnP AVTransport on the network and support SSDP discovery. Additionally, HEOS has a proprietary CLI protocol accessible via TCP on port 1255, which is the primary control path for the HEOS app.

Known quirks: `GetProtocolInfo` response is non-conformant on some firmware versions — some bridge software (AirConnect, swyh-rs) documents that HEOS does not respond correctly to GetProtocolInfo. Discovery uses nested device descriptions, which caused go-upnpcast pre-0.1.0 to miss HEOS renderers (same fix as MusicCast above).

State: Closed, maintained by Denon/Marantz (Sound United / Masimo).

```
https://assets.denon.com/documentmaster/us/heos_cli_protocol_specification_290616.pdf
```

---

### Roon Ready (RAAT)

Not UPnP. Included for context because tutti profiles renderers that may support both paths.

RAAT (Roon Advanced Audio Transport) is a proprietary binary protocol. Devices must be Roon Ready certified. Transport is opaque. No SSDP; discovery is through Roon's own service. Roon can also drive UPnP/DLNA renderers via its built-in DLNA output, but that path is distinct from RAAT.

Source: closed. Certification spec is NDA.

---

### AirPlay / AirPlay 2

Not UPnP. Included for context.

AirPlay uses Bonjour (mDNS) for discovery and RTSP over TCP for transport. AES-encrypted. AirPlay 1 is capped at 16-bit/44.1 kHz; AirPlay 2 supports 24-bit/48 kHz. Multi-room sync is native. No relation to UPnP AVTransport. Devices that support both AirPlay and UPnP are independent stacks co-existing on the same hardware.

```
https://github.com/openairplay/openairplay   (protocol reverse engineering)
```

---

## Control-point libraries

<div class="cat-explainer">
<svg viewBox="0 0 720 200" role="img" aria-label="Control-point library: SSDP and SOAP plumbing that lives inside a music app.">
  <rect x="20" y="20" width="500" height="160" fill="#ffffff" stroke="#e0e0e0" stroke-width="1" rx="3"/>
  <text x="40" y="46" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="#707070" letter-spacing="1.4">A MUSIC APP</text>

  <rect x="40" y="60" width="180" height="100" rx="3" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="130" y="90"  text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">UI + library</text>
  <text x="130" y="110" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">cast button, playlist</text>
  <text x="130" y="125" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">device picker</text>

  <line x1="220" y1="110" x2="280" y2="110" stroke="#4a6741" stroke-width="2"/>
  <polygon points="280,110 270,105 270,115" fill="#4a6741"/>

  <rect x="280" y="60" width="220" height="100" rx="3" fill="#ffffff" stroke="#4a6741" stroke-width="2"/>
  <text x="390" y="86"  text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="#4a6741" letter-spacing="1.4">CONTROL-POINT LIBRARY</text>
  <text x="390" y="110" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="13" font-weight="700" fill="#4a6741">this category</text>
  <text x="390" y="130" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">SSDP discovery · descriptor parse</text>
  <text x="390" y="146" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">SOAP envelopes · DIDL-Lite</text>

  <line x1="540" y1="110" x2="600" y2="110" stroke="#4a6741" stroke-width="2"/>
  <polygon points="600,110 590,105 590,115" fill="#4a6741"/>

  <rect x="580" y="60" width="120" height="100" rx="3" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="640" y="90"  text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">network</text>
  <text x="640" y="110" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">to renderer</text>
</svg>

A library, not an app. It's what an app uses to <em>talk to</em> renderers — the part that knows how to open a UDP socket for SSDP, parse a descriptor, and assemble a SOAP envelope. tutti's library-decision matrix asks: which library would your device's descriptor pass?
</div>

These run in the application that discovers and drives renderers.

### huin/goupnp

Go. MIT license. SSDP client plus auto-generated SOAP service stubs for AVTransport, RenderingControl, MediaServer (av1 profile) and Internet Gateway Device profiles. Used by projects that need low-level UPnP control-point plumbing. go-upnpcast (see below) depends on goupnp for SSDP discovery.

No documented DLNA-specific quirks. The library is a thin client; quirks in the field are almost always protocol-level, not library bugs.

State: Actively maintained. Regular releases on GitHub.

```
https://github.com/huin/goupnp
```

---

### supersonic-app/go-upnpcast

Go. MIT license. Extracted and refactored from go2tv (alexballas) for use as a standalone library. Handles MediaRenderer discovery (SSDP), AVTransport SOAP calls, and playback lifecycle. Not API stable.

Key fix in v0.1.0: "Fix MediaRenderer discovery for nested devices (Denon, etc)" — nested UPnP device descriptions (where the MediaRenderer service is under a sub-device node) were not being walked. This affected Denon HEOS and Yamaha MusicCast. Tutti's UPnP control path runs through this library.

State: Active. Maintained within the supersonic-app GitHub org.

```
https://github.com/supersonic-app/go-upnpcast
```

---

### alexballas/go2tv

Go. MIT license. Desktop casting tool (not a library) that exposes the UPnP control-point logic go-upnpcast was extracted from. Supports M-POST fallback for renderers that don't accept POST, and RFC-compliant relative URL resolution. Still active as an application; go-upnpcast is the library extraction.

State: Active.

```
https://github.com/alexballas/go2tv
```

---

### jupnp/jupnp

Java. Apache License 2.0. Active fork of 4thline/cling. Maintained by the jupnp org with openHAB as primary consumer. OSGi bundles available. Requires Java 11+. Implements full UPnP DA 1.0: SSDP, GENA event subscription, SOAP, and SCPD description parsing.

Used by: openHAB (home automation hub), and any Java application that previously used Cling and has migrated. Version 3.x series is the current line.

State: Active.

```
https://github.com/jupnp/jupnp
```

---

### 4thline/cling

Java. LGPL v2.1. The original Java UPnP library by Christian Bauer. Archived: last commit February 2020, no releases since. Still widely embedded in older Android apps and home automation projects that have not migrated to jupnp.

BubbleUPnP historically used Cling as its Android UPnP library (documented in Cling's own README as a notable user). Current BubbleUPnP versions may carry a private fork or use jupnp — the app is closed source.

Known quirks: Android transport layer requires custom wiring (`AndroidUpnpService`). The library's HTTP stack predates modern Android networking APIs and requires workarounds on API 28+ (cleartext restrictions).

State: Archived. Use jupnp instead.

```
https://github.com/4thline/cling
```

---

### GUPnP (C, GLib — control-point side)

Same library as the renderer-side entry. gUPnP is symmetric: the same C/GLib library is used for both device (renderer) and control-point roles. LGPL v2.1+.

Used by: Rygel (control-point features), gnome-user-share, and any GLib-based Linux application acting as a controller. The gupnp-tools package includes `gssdp-discover` and `gupnp-universal-cp` — standard debugging tools for UPnP on Linux.

```
https://gitlab.gnome.org/GNOME/gupnp
```

---

### libupnp / pupnp (control-point side)

Same C library as renderer side. pupnp's control-point API is lower-level than gUPnP — callers do their own XML parsing of device descriptions and build SOAP payloads manually. VLC uses it at this layer to drive renderer discovery and AVTransport control.

```
https://github.com/pupnp/pupnp
```

---

### flyte/upnpclient (Python)

Python 3. MIT license. SSDP discovery plus SOAP action invocation against arbitrary UPnP services. No AV-specific helpers — callers address services by type string and action name. Descended from Ferry Boender's blog-post example code.

Used for scripting and testing. async_upnp_client (StevenLooman) is the asyncio-native alternative used in Home Assistant for DLNA DMR control.

State: Low activity; functional. PyPI: `upnpclient`.

```
https://github.com/flyte/upnpclient
```

---

### node-upnp-utils (Node.js)

Node.js. MIT license. SSDP client for device discovery plus device-description XML fetch. No built-in AVTransport helpers — discovery-only. Does not work out of the box on Windows due to conflict with the Windows SSDP Discovery service.

State: Low activity. npm: `node-upnp-utils`.

```
https://github.com/futomi/node-upnp-utils
```

---

### upnpx / fkuehne fork (iOS/Objective-C)

Objective-C. Originally by Bruno Keymolen (brunokeymolen/upnpx), maintained fork by Felix Küehne (fkuehne/upnpx). Static library for iOS/macOS. Control-point only. Written in Cocoa + C++ (SSDP layer).

Superseded for new projects by: SwiftUPnP (katoemba/SwiftUPnP — pure Swift, supports OpenHome and UPnP AV, iOS/macOS/tvOS) and CocoaUPnP (arcam/CocoaUPnP — Obj-C successor, block-based).

State: upnpx original — abandoned. fkuehne fork — low activity. SwiftUPnP is the recommended replacement.

```
https://github.com/fkuehne/upnpx
https://github.com/katoemba/SwiftUPnP
```

---

## End-user clients

<div class="cat-explainer">
<svg viewBox="0 0 720 200" role="img" aria-label="End-user client: a complete music app with UI plus an embedded control-point library.">
  <rect x="20" y="60" width="60" height="100" rx="30" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="50" y="115" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">user</text>

  <line x1="80" y1="110" x2="140" y2="110" stroke="#4a6741" stroke-width="2"/>
  <polygon points="140,110 130,105 130,115" fill="#4a6741"/>

  <rect x="140" y="40" width="380" height="140" rx="3" fill="#ffffff" stroke="#4a6741" stroke-width="2"/>
  <text x="160" y="62" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="#4a6741" letter-spacing="1.4">END-USER CLIENT</text>
  <text x="330" y="98" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="13" font-weight="700" fill="#4a6741">the whole app</text>

  <rect x="160" y="115" width="160" height="50" rx="2" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="240" y="138" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">UI + playlist</text>
  <text x="240" y="155" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">supersonic, Feishin, etc.</text>

  <rect x="340" y="115" width="160" height="50" rx="2" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="420" y="138" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">CP library</text>
  <text x="420" y="155" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">go-upnpcast, jupnp, etc.</text>

  <line x1="520" y1="110" x2="580" y2="110" stroke="#4a6741" stroke-width="2"/>
  <polygon points="580,110 570,105 570,115" fill="#4a6741"/>

  <rect x="580" y="60" width="120" height="100" rx="3" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="640" y="105" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">renderers</text>
  <text x="640" y="122" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">on the LAN</text>
</svg>

A library wrapped in a UI. The user clicks "cast to," the UI calls into the embedded control-point library, the library does the SSDP/SOAP work. When tutti finds a parser hazard, the failure shows up to the user as "device missing from picker" inside one of these apps.
</div>

### supersonic (dweymouth)

Cross-platform desktop client for Subsonic/OpenSubsonic and Jellyfin servers. Go + Fyne UI toolkit + MPV audio. UPnP casting via `supersonic-app/go-upnpcast`. Open source, MIT license.

UPnP role: control point, drives external renderers while Supersonic proxies or serves the audio URL. The go-upnpcast library is maintained within the supersonic-app org specifically to support this use case.

```
https://github.com/dweymouth/supersonic
```

---

### Feishin (jeffvli)

Cross-platform desktop client for Navidrome, Jellyfin, and Subsonic. Electron/React/TypeScript. GPL v3. No confirmed native UPnP/DLNA casting implementation — the app focuses on local playback and server-side features. Not a UPnP control point.

State: Active.

```
https://github.com/jeffvli/feishin
```

---

### BubbleUPnP (Android)

Closed source, paid (freemium). The de-facto reference behavior oracle for UPnP/DLNA interoperability on Android. Acts as both control point and software renderer. Originally built on Cling; current internals unknown (app is not open source). Used widely to establish "correct" behavior when debugging renderer bugs — if BubbleUPnP plays it and your client doesn't, the problem is your client.

```
https://bubblesoftapps.com/bubbleupnp/
```

---

### mconnect Player (iOS/Android)

Closed source. UPnP/DLNA and Google Cast control point. Used as a sanity-check oracle alongside BubbleUPnP. If both BubbleUPnP and mconnect exhibit the same behavior toward a renderer, it is almost certainly the renderer's behavior (not a client bug). No source available.

---

### Symfonium (Android)

Closed source, paid. Android music player for Navidrome, Jellyfin, Plex, Subsonic, and local files. UPnP/DLNA casting supported including gapless playback on compatible renderers. Includes proxy mode for renderers that cannot reach the server URL directly. Symfonium's support forum is a useful source of device-specific compatibility reports (Yamaha MusicCast, Denon HEOS quirks documented there).

State: Active (v14.1.0, April 2026).

```
https://symfonium.app/
```

---

### BubbleUPnP Server

Closed source, paid. Java server application that runs on a PC or NAS. Not a media server (does not index files). Key functions: UPnP proxy to improve renderer/server discovery across network segments, OpenHome renderer wrapping for standard AVTransport renderers, Chromecast-to-UPnP bridging, and optional transcoding via ffmpeg. Exposes Subsonic/OpenSubsonic API to enable BubbleUPnP (Android) to connect to media servers. This is the server-side component paired with the BubbleUPnP Android client.

```
https://www.bubblesoftapps.com/bubbleupnpserver2/
```

---

### Kazoo / Kinsky (Linn)

Linn's control-point applications for their DS streaming products. Kinsky (predecessor) was open source and implemented standard UPnP A/V. Kazoo (successor) is closed source and primarily implements OpenHome (a Linn-defined extension of UPnP). Both work with UPnP/AV media servers but prefer the OpenHome OHProduct/OHPlaylist/OHInfo service stack when present.

Relevant to tutti: upmpdcli supports both OpenHome and UPnP AV on the renderer side, and Kazoo/Kinsky are cited on the upmpdcli control-point compatibility page.

```
https://docs.linn.co.uk/wiki/index.php/Kazoo_FAQ
```

---

### VLC UPnP plugin

Open source (GPL). VLC's UPnP services discovery module (`modules/services_discovery/upnp.cpp`) uses libupnp (pupnp) to discover both MediaServer and MediaRenderer devices. VLC can act as a control point driving external renderers. The plugin predates libupnp's 1.14.x series; VLC carries bundled or distro-linked libupnp.

State: Active (maintained within VideoLAN/VLC).

```
https://github.com/videolan/vlc/blob/master/modules/services_discovery/upnp.cpp
```

---

### foobar2000 UPnP plugin (foo_upnp / foo_out_upnp)

Windows only. Closed source, free. Two components: `foo_upnp` (full UPnP/DLNA renderer + server + control point, by Bubblesoft), and `foo_out_upnp` (UPnP MediaRenderer output only). `foo_upnp` can stream virtually any foobar2000-decodable format, including formats unsupported by most standalone servers (CUE sheets, game audio, archives). Per-device transcoding profiles.

Notable: `foo_upnp` is by the same author as BubbleUPnP, so its ProtocolInfo and metadata behavior tracks BubbleUPnP closely.

```
https://www.foobar2000.org/components/view/foo_upnp
```

---

## Bridges and servers

<div class="cat-explainer">
<svg viewBox="0 0 720 200" role="img" aria-label="Bridge / server: turns a media library into HTTP URLs the renderer can fetch.">
  <rect x="20" y="60" width="120" height="100" rx="3" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="80" y="90"  text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">media library</text>
  <text x="80" y="110" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">your files,</text>
  <text x="80" y="124" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">streaming source,</text>
  <text x="80" y="138" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">music playlist</text>

  <line x1="140" y1="110" x2="200" y2="110" stroke="#4a6741" stroke-width="2"/>
  <polygon points="200,110 190,105 190,115" fill="#4a6741"/>

  <rect x="200" y="60" width="240" height="100" rx="3" fill="#ffffff" stroke="#4a6741" stroke-width="2"/>
  <text x="320" y="86"  text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="#4a6741" letter-spacing="1.4">BRIDGE / SERVER</text>
  <text x="320" y="110" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="13" font-weight="700" fill="#4a6741">this category</text>
  <text x="320" y="130" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">indexes media · serves HTTP</text>
  <text x="320" y="146" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">DLNA discovery · stream URLs</text>

  <line x1="440" y1="110" x2="500" y2="110" stroke="#707070" stroke-width="1.5" stroke-dasharray="4,3"/>
  <polygon points="500,110 490,105 490,115" fill="#707070"/>

  <rect x="500" y="60" width="200" height="100" rx="3" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="600" y="90"  text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">renderer fetches</text>
  <text x="600" y="110" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">HTTP GET on the URL</text>
  <text x="600" y="124" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9"  fill="#707070">the bridge handed it</text>
</svg>

The source side of the conversation. The control-point doesn't send audio — it sends the URL where the audio lives. The bridge or server is what serves that URL when the renderer asks for it. Subsonic/Navidrome/Plex aren't UPnP themselves but slot into the same role for clients that speak their APIs.
</div>

### upmpdcli

UPnP MediaRenderer front-end to MPD (Music Player Daemon). C++. GPL. By Jean-François Dockes (lesbonscomptes). Implements both UPnP AV AVTransport and the OpenHome OHProduct/OHPlaylist/OHInfo/OHRadio service set. Uses libnpupnp (a C++ rewrite of pupnp) and libupnpp (C++ UPnP wrapper). Source on Framagit.

This is the reference UPnP renderer implementation for Linux audio setups. Correct behavior here defines the "right" way to interpret SOAP calls. Control-point developers use upmpdcli as a test target for strict compliance.

Known quirks documented in release notes: v1.2.9 fixed incorrect ProtocolInfo format sent to control points. Seeking after gapless transition in UPnP/AV mode was broken in earlier versions. The GetProtocolInfo response format and the set of supported MIME types are configurable via `upmpdcli.conf` — the defaults reflect what MPD can actually play.

State: Actively maintained.

```
https://www.lesbonscomptes.com/upmpdcli/
```

---

### Plex Media Server (DLNA)

Closed source (server), client open source varies. Plex includes a built-in DLNA server exposing the MediaServer device type. ProtocolInfo strings and container/codec support vary by Plex Pass tier. Server-side DLNA is enabled in Settings > DLNA. SSDP announcement lease time defaults to 1800 s.

Relevant to tutti: Navidrome/Subsonic APIs are the sources; Plex is included because some tutti test flows use Plex as the media source for UPnP casting tests.

State: Active.

```
https://support.plex.tv/articles/200350536-dlna/
```

---

### Jellyfin DLNA server

Open source, GPL. Jellyfin's DLNA support was split out into `jellyfin-plugin-dlna` in a later release cycle. Implements UPnP MediaServer (content browsing) and ContentDirectory service. SSDP on port 1900 UDP. The DLNA plugin exposes Jellyfin's library to any DLNA/UPnP control point.

State: Active. Plugin repo separate from main Jellyfin repo.

```
https://github.com/jellyfin/jellyfin-plugin-dlna
```

---

### Subsonic / Navidrome / Airsonic

Not UPnP servers. They expose the Subsonic (OpenSubsonic) REST API over HTTP. They do not implement SSDP, AVTransport, ContentDirectory, or any UPnP service. They appear in tutti's evidence flow because UPnP clients (supersonic, Symfonium, BubbleUPnP) fetch audio URLs from Navidrome/Subsonic via REST, then push those URLs to UPnP renderers via SetAVTransportURI. The Subsonic server is the A-side audio source, not the UPnP peer.

```
https://www.navidrome.org/
https://www.subsonic.org/
```

---

### Roon (server, briefly)

Not UPnP natively. Roon Server drives Roon Ready devices over RAAT. Roon also includes a built-in DLNA output that can push audio to UPnP AVTransport renderers, but this is a secondary path. Included for completeness: if a device reports working with Roon RAAT but failing with UPnP, the fault is in the UPnP stack on the device, not the renderer hardware.

---

### ReadyDLNA / MiniDLNA

C. GPL. Originally developed by NETGEAR for ReadyNAS appliances. Lightweight DLNA/UPnP MediaServer. Implements ContentDirectory and ConnectionManager services. No renderer. Uses SQLite for the media index.

Now packaged as `minidlna` or `readymedia` in Linux distributions. Primary SourceForge repo; the azatoth/minidlna GitHub mirror receives no recent commits (last meaningful activity 2021–2022). Maintained distributionally via distro patches.

State: Slow-moving; effectively in maintenance mode. Widely deployed despite infrequent upstream releases.

```
https://sourceforge.net/projects/minidlna/
```

---

## Quick reference matrix

| Name | Role | Language | License | State |
|---|---|---|---|---|
| Platinum SDK (plutinosoft) | renderer / server / CP library | C++ | GPL v2 / Commercial | Sporadic |
| libupnp / pupnp | renderer / CP library | C | BSD | Active |
| gUPnP | renderer / CP library | C (GLib) | LGPL v2.1 | Active |
| CyberLink for Java | renderer / CP library | Java | LGPL | Abandoned |
| jUPnP | renderer / CP library | Java | Apache 2.0 | Active |
| Sonos stack | renderer | C++ (closed) | Proprietary | Active |
| Yamaha MusicCast | renderer | closed | Proprietary | Active |
| Denon HEOS | renderer | closed | Proprietary | Active |
| Roon Ready (RAAT) | renderer | closed | Proprietary | Active |
| AirPlay 2 | renderer | closed | Proprietary | Active |
| huin/goupnp | CP library | Go | MIT | Active |
| supersonic-app/go-upnpcast | CP library | Go | MIT | Active |
| alexballas/go2tv | client / CP | Go | MIT | Active |
| jupnp/jupnp | CP library | Java | Apache 2.0 | Active |
| 4thline/cling | CP library | Java | LGPL v2.1 | Archived |
| GUPnP (control-point) | CP library | C (GLib) | LGPL v2.1 | Active |
| libupnp / pupnp (CP side) | CP library | C | BSD | Active |
| flyte/upnpclient | CP library | Python | MIT | Low activity |
| node-upnp-utils | CP library | Node.js | MIT | Low activity |
| upnpx / fkuehne fork | CP library | Obj-C | MIT | Low activity |
| SwiftUPnP (katoemba) | CP library | Swift | MIT | Active |
| supersonic (dweymouth) | client | Go | MIT | Active |
| Feishin (jeffvli) | client | TypeScript | GPL v3 | Active |
| BubbleUPnP | client | Java/Android (closed) | Proprietary | Active |
| mconnect Player | client | closed | Proprietary | Active |
| Symfonium | client | Android (closed) | Proprietary | Active |
| BubbleUPnP Server | server / bridge | Java (closed) | Proprietary | Active |
| Kazoo / Kinsky (Linn) | client / CP | closed | Proprietary | Active |
| VLC UPnP plugin | client / CP | C | GPL | Active |
| foobar2000 foo_upnp | client / CP | Windows (closed) | Freeware | Active |
| upmpdcli | server / bridge | C++ | GPL | Active |
| Plex Media Server DLNA | server | closed | Proprietary | Active |
| Jellyfin DLNA plugin | server | C# | GPL | Active |
| Navidrome / Subsonic | source (not UPnP) | Go / Java | GPL / AGPL | Active |
| Roon | source / bridge | closed | Proprietary | Active |
| MiniDLNA / ReadyMedia | server | C | GPL | Maintenance |
