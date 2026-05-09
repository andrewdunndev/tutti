# Eversolo DMP-A6 Master Gen 2

The first device in the corpus. A Plutinosoft Platinum 1.0.5.13 stack
running UPnP 1.1 / DLNADOC 1.50, advertised as `MediaRenderer:1` with
the standard three services (AVTransport, RenderingControl,
ConnectionManager).

## Headline findings

- **Discovers cleanly under both Go control-point libraries that tutti
  knows about.** Both `go-upnpcast` (the supersonic-app fork) and
  `huin/goupnp` accept the descriptor. The narjo iOS investigation
  about a non-RFC-4122 UDN does not bite either Go library; their
  parsers do not validate UDN shape.
- **GetProtocolInfo announces 392 sinks, 107 audio, zero DSD entries.**
  The device is fed DSD streams via Subsonic `format=raw` passthrough
  (audibly verified during narjo's investigation), but does not
  declare DSD support in its Sink list. A control point that
  intersects against the announced list will not attempt DSD. Same
  pattern for Opus.
- **DIDL-Lite metadata is honored, embedded tags are not.** Across
  every drive run that reached PLAYING, the device echoed back the
  DIDL-Lite title tutti sent (`tutti probe: ...`), never the embedded
  tag (`tutti embed: ...`). `metadata_source = didl-lite,
  metadata_round_trip = matched` in every run.
- **Plays every announced audio format end-to-end.** WAV PCM
  44.1/48/96/192 kHz, FLAC 44.1/96 kHz, MP3 320, AAC 256: all reached
  PLAYING within a few seconds of `Play`. Opus is not in the device's
  Sink list and was therefore skipped by tutti's filter; narjo's
  evidence shows Opus does in fact play when the path is right.

## Why this device is the corpus's anchor

It surfaces three classes of evidence that any future device's tutti
capture can be measured against:

1. *Discovery*: a device that should "just work" with mainstream Go
   libraries actually does — useful as a positive baseline.
2. *Announce vs play divergence*: the GetProtocolInfo Sink list
   under-reports what the device actually accepts. Catching this in
   the corpus tells future contributors what to expect on
   Plutinosoft-derived stacks.
3. *Metadata surface*: the DIDL-Lite-vs-embedded test fires cleanly,
   and the device falls firmly on the DIDL-Lite side. Future devices
   that fall on the embedded side (or are mixed) will be visible in
   the corpus at a glance.

## Reference

- Vendor URL: https://www.eversolo.com/Product/details/14.html
- Renderer SDK: Plutinosoft Platinum (`https://github.com/plutinosoft/Platinum`)
- DLNA cert: `DMR-1.50`
- Sibling investigation: [narjo-eversolo-upnp][narjo] (Python toolkit,
  the seed of tutti).

[narjo]: https://gitlab.com/dunn.dev/narjo-eversolo-upnp
