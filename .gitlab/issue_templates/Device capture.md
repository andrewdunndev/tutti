<!--
Device capture submission via issue.

Use this when you have a tutti capture ready but aren't comfortable
opening a merge request. Attach the capture directory as a tarball
(or paste the manifest.json inline) and a maintainer will fold it
into the corpus.

If you can open an MR, the MR template is faster end to end. See
https://tutti.dunn.dev/contribute/.
-->

## Device

<!-- Vendor and product name as the manufacturer brands them
     (EVERSOLO DMP-A6 Master Gen 2, not the device's friendlyName
     which can include per-installation tags). -->

- **Vendor**:
- **Product**:
- **Firmware** (if known):

## Capture

<!-- Attach the whole capture-<ts>-<host>/ directory as a .tar.gz.
     If the bundle is too large to attach, drop it on a private
     paste site and link it; a maintainer will pull it down. -->

- Capture attached: yes / no
- `tutti validate <dir>` ran clean: yes / no / didn't run

## Network notes

<!-- Anything that might matter for reproduction. Wired vs Wi-Fi,
     multi-NIC, VLANs, IGMP snooping, IPv6, virtual network adapters.
     Skip if nothing weird. -->

## What I observed

<!-- One short paragraph: did the device discover, did the drive run
     reach PLAYING, did the metadata round-trip the way you expected. -->

## Music source(s) used with this device

<!-- Free-form: Navidrome, Subsonic, Plex, Roon, Spotify Connect, etc.
     Helps other contributors who use the same source decide whether
     this device is worth their time. -->

/label ~"device-capture"
