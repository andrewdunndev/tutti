# reference-impl/python

Stdlib-only Python implementation of the three wire-level operations the
`tutti` Go binary performs. These scripts exist as a legibility anchor:
anyone wary of running an unsigned Go binary can read these in a few
minutes and verify the binary does the same thing on the wire.

Independent reimplementations from first principles, vocabulary-aligned
with `tutti`. The original narjo Python scripts (at
`gitlab.com/dunn.dev/narjo-eversolo-upnp`) were the reference.

Python 3.10+, stdlib only, no `pip install` required.

## Scripts

```sh
# 1. SSDP M-SEARCH multicast, group responses by USN.
python3 probe.py [--interface IP] [--mx 3]

# 2. GET a UPnP device descriptor and parse it.
python3 fetch.py http://<DEVICE_IP>:<PORT>/description.xml

# 3. Drive a UPnP MediaRenderer end to end via SOAP.
python3 drive.py \
    --control-url 'http://<DEVICE_IP>:<PORT>/AVTransport/<udn>/control.xml' \
    --stream-url  '<HTTP audio URL>' \
    --mime audio/flac \
    --title 'tutti probe: FLAC 96k/24'
```

These produce human-readable text on stdout. The `tutti` Go binary
produces structured JSON manifests; that is the difference.
