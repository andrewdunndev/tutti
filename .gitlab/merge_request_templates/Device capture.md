<!--
Device capture submission via MR.

If you can open an MR, this is the faster path: your capture lands
in the corpus directly on merge and the site rebuilds within a few
minutes. See https://tutti.dunn.dev/contribute/.

If you don't want to open an MR, use the issue template instead.
-->

## Device

- **Vendor**:
- **Product**:
- **Firmware** (if known):
- **device.json authored**: yes / no (paste it inline if no)

## Capture

- Path added under `evidence/<vendor>-<model>/captures/<ts>-<contributor>/`
- `tutti validate <dir>` passes locally
- Captured with tutti version: `vX.Y.Z`

## Pre-submit checklist

- [ ] `tutti validate` passes against the capture directory
- [ ] `notes.md` filled in with anything worth knowing (network
      topology, firmware quirks, what surprised me)
- [ ] No raw LAN IPs / auth tokens in the bundle (the binary
      auto-redacts; this is a final visual check)
- [ ] `device.json` exists at `evidence/<slug>/device.json` for this
      device (vendor, product_name, manufacturer URL); add one if
      this is the first capture for the slug

## What this capture shows

<!-- One short paragraph: discovery verdict, drive results, anything
     unusual. -->

/label ~"device-capture"
