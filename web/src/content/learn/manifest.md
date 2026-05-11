---
title: "What's in a capture"
eyebrow: "Reference"
description: "The shape of tutti's output: directory layout, what each artifact carries, and the manifest JSON that ties it all together."
order: 4
---


A capture is a directory. `manifest.json` at the root is the index every consumer reads first; everything else is referenced from it by relative path. This page walks the shape end to end: what tutti writes per device, how the manifest stitches it together, and the controlled vocabularies a renderer or downstream tool can rely on.

The wire-level theory behind these artifacts (SSDP, mDNS, descriptor parse, AVTransport drive) lives in [protocols](/learn/protocols/). The failure modes that motivated each capture surface live in [why it goes wrong](/learn/why-finicky/). This document is about *what tutti hands you*.

<div class="cat-explainer">
<svg viewBox="0 0 720 240" role="img" aria-label="The capture pipeline. tutti listens on the LAN for SSDP and mDNS announcements, fetches each device's descriptor and protocol info, optionally drives a real cast, and writes everything as JSON plus raw wire artifacts under a single capture directory. The site at tutti.dunn.dev reads that directory back to render device pages.">
  <rect x="20" y="20" width="680" height="200" fill="#ffffff" stroke="#e0e0e0" stroke-width="1" rx="3"/>
  <text x="40" y="46" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="#707070" letter-spacing="1.4">CAPTURE PIPELINE</text>

  <rect x="40" y="74" width="160" height="120" rx="3" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="120" y="94" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="#707070" letter-spacing="1.2">LAN</text>
  <text x="120" y="116" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">audio devices</text>
  <text x="120" y="138" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#707070">SSDP / mDNS</text>
  <text x="120" y="152" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#707070">descriptor.xml</text>
  <text x="120" y="166" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#707070">GetProtocolInfo</text>
  <text x="120" y="180" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#707070">AVTransport</text>

  <line x1="200" y1="134" x2="280" y2="134" stroke="#4a6741" stroke-width="2"/>
  <polygon points="280,134 270,129 270,139" fill="#4a6741"/>
  <text x="240" y="127" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="#404040">listen + probe</text>

  <rect x="280" y="74" width="160" height="120" rx="3" fill="#ffffff" stroke="#4a6741" stroke-width="2"/>
  <text x="360" y="94" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="#4a6741" letter-spacing="1.2">TUTTI</text>
  <text x="360" y="116" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="700" fill="#4a6741">capture binary</text>
  <text x="360" y="138" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#404040">discover</text>
  <text x="360" y="152" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#404040">analyse (library decisions)</text>
  <text x="360" y="166" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#404040">drive (--drive only)</text>
  <text x="360" y="180" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#404040">redact + write</text>

  <line x1="440" y1="134" x2="520" y2="134" stroke="#4a6741" stroke-width="2"/>
  <polygon points="520,134 510,129 510,139" fill="#4a6741"/>
  <text x="480" y="127" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" fill="#404040">write to disk</text>

  <rect x="520" y="74" width="160" height="120" rx="3" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="600" y="94" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="#707070" letter-spacing="1.2">CAPTURE DIR</text>
  <text x="600" y="116" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="11" font-weight="600" fill="#0a0a0a">manifest.json</text>
  <text x="600" y="138" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#707070">ssdp.json + raw.txt</text>
  <text x="600" y="152" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#707070">mdns.json</text>
  <text x="600" y="166" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#707070">devices/&lt;slug&gt;/...</text>
  <text x="600" y="180" text-anchor="middle" font-family="IBM Plex Sans, sans-serif" font-size="9" fill="#707070">notes.md (yours)</text>

  <line x1="600" y1="194" x2="600" y2="210" stroke="#707070" stroke-width="1.5" stroke-dasharray="3,2"/>
  <text x="600" y="216" text-anchor="middle" font-family="IBM Plex Mono, monospace" font-size="8" fill="#707070">consumed by tutti.dunn.dev</text>
</svg>

Three phases, one directory. <em>Discover</em> is passive listening plus M-SEARCH bursts. <em>Analyse</em> is library-decision reimplementation against each fetched descriptor. <em>Drive</em> is opt-in: tutti only sends real AVTransport actions when you pass <code>--drive</code>. Everything is written to disk before the binary exits — you can review the bundle, edit <code>notes.md</code>, and decide whether to submit.
</div>

---

## What tutti captures, per source

Per device on your LAN, a capture records:

- **SSDP**: the M-SEARCH inputs (target, MX, ST list), every response (USN, ST, LOCATION, SERVER, BOOTID, CONFIGID, source `IP:port`), and per-USN grouping. Raw wire dump and parsed JSON sit side by side.
- **mDNS / DNS-SD**: every audio-renderer service type announced (`_airplay._tcp`, `_googlecast._tcp`, `_raop._tcp`, `_spotify-connect._tcp`, `_sonos._tcp`, and a handful of vendor-specific types), with full TXT records.
- **Device descriptor**: raw XML as fetched, and a parsed canonical form (deviceType, friendlyName, manufacturer, modelName, UDN, X_DLNADOC, icons, services list with SCPD/control/event URLs).
- **Library decisions**: for each Go UPnP control-point library tutti knows about (`go-upnpcast`, `huin/goupnp`), the accept/reject result with a machine-parseable `reason_code`. v0 ships *reimplementations* of each library's parser (see `internal/decisions`), keyed as `<lib>@reimpl-of-<version>`. Real library invocation lands later; the schema field `decision.method` distinguishes `reimplementation` (today) from `library_call` (future).
- **GetProtocolInfo**: the device's Sink list (every MIME type it claims to accept), with derived counts per format class (DSD, MQA, FLAC, etc.). Drives discovery of the kind of bug that hides behind a device that *plays* a format but doesn't *announce* it.
- **Drive test** (opt-in via `--drive`): a real `SetAVTransportURI` + `Play` against test tones, with polled `GetTransportInfo` / `GetPositionInfo` capture. Format matrix is intersected with the device's `protocol_info` Sink list, so tutti only attempts formats the device claims to accept (PCM 44.1/48/96/192 kHz, FLAC at common rates, MP3, AAC, Opus). Tones are math-generated 5-second sine waves built by `scripts/gen-tones.sh`, embedded in the binary, served from a transient local HTTP server tutti spins up on a random port.
- **Metadata on two surfaces** (per drive run):
  - **DIDL-Lite envelope** that tutti sends with `SetAVTransportURI`: title `"tutti probe: <format>@<rate>"`, artist `tutti.dunn.dev`, album `Audio Renderer Probes`. v0 omits `<upnp:albumArtURI>` (no cover art site yet); when `tutti.dunn.dev/audio/covers/` exists, each scenario will reference a 300×300 PNG with `PROBE` and the format/rate overlaid.
  - **Embedded tags inside each test file** (ID3v2 for MP3, Vorbis comments for FLAC/Ogg, MP4 atoms for AAC/ALAC): title `"tutti embed: <format>@<rate>"`, artist `tutti.dunn.dev`, album `Audio Renderer Probes (embedded)`. Embedded artwork is a v1+ addition.

The two surfaces use *deliberately different* title prefixes (`probe` vs `embed`) so tutti can read `GetPositionInfo`'s `TrackMetaData` and derive which surface the device honored: `didl-lite`, `embedded`, `mixed`, or `none`. When cover art lands, devices that render it will show whichever card the device preferred on their display, adding a visible confirmation channel.

---

## Directory layout

```
capture-2026-05-09T143022Z-andrewdunndev/
├── manifest.json                 # schema-versioned index, the renderer reads this
├── ssdp.json                     # parsed SSDP responses
├── ssdp-raw.txt                  # original wire dump
├── mdns.json                     # parsed mDNS records, all audio service types
├── devices/
│   └── eversolo-dmp-a6/          # one dir per device
│       ├── descriptor.xml        # raw, fetched
│       ├── descriptor.json       # parsed, canonical fields
│       ├── decisions.json        # per-library accept/reject + reason
│       ├── protocol-info.json    # parsed Sink list with derived counts
│       ├── protocol-info.txt     # raw GetProtocolInfo response
│       └── drive/                # only if --drive
│           ├── run-1-opus.log
│           ├── run-2-dsd64.log
│           └── run-3-dsd256.log
└── notes.md                      # writable by you, prose context
```

The slug under `devices/` is canonical and stable: `<vendor>-<model>` lowercased, hyphenated. It's what `evidence/` uses, what the device page URL is keyed on, and what `manifest.json` references.

---

## manifest.json (schema v1, illustrative)

```json
{
  "schema_version": 1,
  "tutti_version": "0.1.0",
  "capture_id": "01HX7Z2FJK4Q3P9V8R5N6M2W3T",
  "captured_at": "2026-05-09T14:30:22Z",
  "contributor": "github:andrewdunndev",
  "host": { "os": "darwin", "arch": "arm64", "interfaces": ["en0"] },

  "scaninfo": {
    "ssdp_st_list": ["ssdp:all", "upnp:rootdevice",
                     "urn:schemas-upnp-org:device:MediaRenderer:1"],
    "ssdp_mx": 3,
    "mdns_service_types": ["_airplay._tcp", "_googlecast._tcp",
                           "_raop._tcp", "_spotify-connect._tcp"]
  },

  "runstats": {
    "elapsed_seconds": 12.4,
    "ssdp_responses": 9,
    "ssdp_unique_usns": 6,
    "mdns_records": 14,
    "exit": "success"
  },

  "redactions": ["client-ip", "device-ip", "auth-tokens",
                 "salt", "username", "lan-topology"],

  "devices": [
    {
      "slug": "eversolo-dmp-a6",
      "vendor": "EVERSOLO",
      "model": "DMP-A6 Master Gen 2",
      "firmware": "5.6.46-100",
      "udn": "uuid:800a805eef11-dmr",
      "tags": ["dlna", "discovery-ok", "drive-ok"],

      "discovery": {
        "ssdp_usns": ["uuid:800a805eef11-dmr",
                      "uuid:800a805eef11-dmr::upnp:rootdevice"],
        "mdns_services": []
      },

      "decisions": {
        "go-upnpcast@reimpl-of-v0.1.0": {
          "result": "accepted",
          "reason_code": "mediarenderer_v1_avtransport_found",
          "reason_text": "deviceType matched MediaRenderer:1; AVTransport, RenderingControl, ConnectionManager all parsed",
          "method": "library_call",
          "confidence": "high"
        },
        "huin/goupnp@reimpl-of-v1.3.0": {
          "result": "accepted",
          "reason_code": "device_with_av_transport",
          "reason_text": "...",
          "method": "library_call",
          "confidence": "high"
        }
      },

      "protocol_info": {
        "sink_count": 392,
        "audio_sink_count": 107,
        "format_matches": { "dsd": 0, "flac": 4, "mqa": null }
      },

      "drive_test": {
        "performed": true,
        "runs": [
          {
            "scenario": "flac-96k-24",
            "mime": "audio/flac",
            "rate_hz": 96000,
            "bits": 24,
            "metadata_didl_lite_sent": {
              "title": "tutti probe: FLAC 96k/24",
              "artist": "tutti.dunn.dev",
              "album": "Audio Renderer Probes",
              "art_url": "https://tutti.dunn.dev/audio/covers/probe-flac-96000-24.png"
            },
            "metadata_embedded": {
              "title": "tutti embed: FLAC 96k/24",
              "artist": "tutti.dunn.dev",
              "album": "Audio Renderer Probes (embedded)",
              "art_card": "embed-flac-96000-24.png"
            },
            "metadata_echoed": {
              "title": "tutti probe: FLAC 96k/24",
              "art_url": "https://tutti.dunn.dev/audio/covers/probe-flac-96000-24.png"
            },
            "metadata_source": "didl-lite",
            "metadata_round_trip": "matched",
            "art_fetch_observed": true,
            "result": "playing",
            "transitions": ["TRANSITIONING", "PLAYING"],
            "transcript_file": "devices/eversolo-dmp-a6/drive/run-1-flac-96k-24.log"
          }
        ]
      }
    }
  ]
}
```

---

## Field notes

`reason_code` is a controlled vocabulary; the renderer filters and colors on it. `reason_text` is for humans reading the bug report. `capture_id` is a ULID; cite it in upstream issues so the maintainer can find the source-of-truth bundle.

A device entry must always have `discovery` (the SSDP/mDNS announcements that surfaced it). Everything else (`decisions`, `protocol_info`, `drive_test`, `descriptor`) is optional but at least one must be present. This lets tutti record evidence for devices that announce only on mDNS (AirPlay-only, Spotify-Connect-only) or whose descriptor URL is unreachable, without dropping them from the manifest.

The full JSON Schema lives at [`schema/manifest.v1.json`](https://gitlab.com/dunn.dev/tutti/-/blob/main/schema/manifest.v1.json) in the source tree. That file is the source of truth. Everything downstream is derived from it: tutti's Go validator, this site's TypeScript types, and the CI gate on contribution MRs.
