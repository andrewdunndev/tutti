# Capture notes

First capture of the Eversolo DMP-A6 ("LivingRoom" instance) and
first capture in the tutti corpus.

## Network

- LAN, gigabit ethernet, no special firewalling.
- macOS arm64 host (`darwin/arm64`), single en0 interface.
- Eversolo on a static lease, reachable from the host without DNS.

## Notable choices for this run

- `--drive --force` was used because at the time of this capture, drive
  did not yet emit Stop between tones, so the per-tone precheck
  refused the second run. The fix landed immediately after this
  capture (drive now emits Stop between tones); future captures need
  `--force` only if the device was already PLAYING something when the
  run started.
- The amplifier was off for the duration of this capture, so every
  PLAYING transition reached the device's display + decoder pipeline
  but not the speakers.

## Evidence highlights

- **Library decisions**: both `go-upnpcast` and `huin/goupnp`
  reimplementations accept. No discovery-side parser hazard observed.
- **ProtocolInfo**: 392 sinks, 107 audio, 0 DSD entries. Same shape as
  the narjo capture from May 2026.
- **Drive matrix**: 8 of 9 tones attempted (Opus skipped by the
  format-intersection filter; Eversolo does not announce
  `audio/opus`). All 8 reached PLAYING.
- **Metadata surface**: every run, `metadata_source = didl-lite` and
  `metadata_round_trip = matched`. Eversolo honors DIDL-Lite, ignores
  embedded ID3/Vorbis/MP4 tags.

## Open follow-ups

- Drive should issue a Stop between tones to make `--force` unnecessary
  for sequential runs.
- Per-run cover art is not yet sent; once `tutti.dunn.dev/audio/covers/`
  is up, the device's screen will show the format/rate text live.
- Re-run with --no-redact intentionally on the isolated test bench to
  confirm the un-redacted artifact set is what we expect.
