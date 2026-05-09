#!/usr/bin/env python3
"""Drive a UPnP MediaRenderer end to end via SOAP.

Sends `SetAVTransportURI` with a DIDL-Lite envelope, then `Play`, then
polls `GetTransportInfo` and `GetPositionInfo` for a few seconds.
Stdlib only.
"""
from __future__ import annotations

import argparse
import sys
import time
import urllib.request
import xml.etree.ElementTree as ET

AV_NS = "urn:schemas-upnp-org:service:AVTransport:1"


def soap(control_url: str, action: str, args_xml: str, timeout: float = 10.0) -> tuple[int, bytes]:
    body = (
        '<?xml version="1.0" encoding="utf-8"?>\n'
        '<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" '
        's:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">\n'
        f'  <s:Body>\n    <u:{action} xmlns:u="{AV_NS}">{args_xml}</u:{action}>\n  </s:Body>\n'
        '</s:Envelope>\n'
    ).encode()
    req = urllib.request.Request(
        control_url, data=body,
        headers={
            "SOAPACTION": f'"{AV_NS}#{action}"',
            "CONTENT-TYPE": 'text/xml; charset="utf-8"',
            "CONTENT-LENGTH": str(len(body)),
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.getcode(), resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()


def didl_lite(title: str, artist: str, album: str, mime: str, url: str, art_url: str = "") -> str:
    art = f"<upnp:albumArtURI>{art_url}</upnp:albumArtURI>" if art_url else ""
    return (
        '<DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" '
        'xmlns:dc="http://purl.org/dc/elements/1.1/" '
        'xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/">'
        '<item id="tutti-probe-1" parentID="0" restricted="1">'
        f"<dc:title>{xml_escape(title)}</dc:title>"
        f"<upnp:artist>{xml_escape(artist)}</upnp:artist>"
        f"<upnp:album>{xml_escape(album)}</upnp:album>"
        "<upnp:class>object.item.audioItem.musicTrack</upnp:class>"
        f'<res protocolInfo="http-get:*:{mime}:*">{xml_escape(url)}</res>'
        f"{art}"
        "</item></DIDL-Lite>"
    )


def xml_escape(s: str) -> str:
    return (
        s.replace("&", "&amp;")
         .replace("<", "&lt;")
         .replace(">", "&gt;")
         .replace('"', "&quot;")
         .replace("'", "&apos;")
    )


def localname(tag: str) -> str:
    return tag.split("}", 1)[-1]


def parse_response(body: bytes, action: str) -> dict[str, str]:
    """Permissively walk SOAP body, return action's response fields."""
    fields: dict[str, str] = {}
    try:
        root = ET.fromstring(body)
    except ET.ParseError:
        return fields
    for elem in root.iter():
        name = localname(elem.tag)
        if name == f"{action}Response":
            for child in elem:
                fields[localname(child.tag)] = (child.text or "").strip()
            break
    return fields


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--control-url", required=True)
    ap.add_argument("--stream-url", required=True)
    ap.add_argument("--mime", default="audio/flac")
    ap.add_argument("--title", default="tutti probe")
    ap.add_argument("--artist", default="tutti.dunn.dev")
    ap.add_argument("--album", default="Audio Renderer Probes")
    ap.add_argument("--art-url", default="")
    ap.add_argument("--poll-seconds", type=int, default=12)
    ap.add_argument("--poll-interval", type=float, default=3.0)
    args = ap.parse_args()

    print(f"[avt] control: {args.control_url}")
    print(f"[avt] stream:  {args.stream_url}")
    print(f"[avt] track:   {args.title} [{args.mime}]")
    print()

    metadata = didl_lite(args.title, args.artist, args.album, args.mime, args.stream_url, args.art_url)

    set_args = (
        "<InstanceID>0</InstanceID>"
        f"<CurrentURI>{xml_escape(args.stream_url)}</CurrentURI>"
        f"<CurrentURIMetaData>{xml_escape(metadata)}</CurrentURIMetaData>"
    )
    code, _ = soap(args.control_url, "SetAVTransportURI", set_args)
    print(f"--- step 1: SetAVTransportURI -> HTTP {code} ---")

    code, _ = soap(args.control_url, "Play",
                   "<InstanceID>0</InstanceID><Speed>1</Speed>")
    print(f"--- step 2: Play -> HTTP {code} ---")

    polls = max(1, int(args.poll_seconds / args.poll_interval))
    print()
    print(f"[avt] polling for {polls * args.poll_interval:.0f}s every {args.poll_interval:.1f}s ...")
    for n in range(1, polls + 1):
        time.sleep(args.poll_interval)
        c, b = soap(args.control_url, "GetTransportInfo", "<InstanceID>0</InstanceID>")
        ti = parse_response(b, "GetTransportInfo")
        print(f"\n--- poll {n} transport: GetTransportInfo -> HTTP {c} ---")
        for k, v in ti.items():
            print(f"  {k}: {v}")

        c, b = soap(args.control_url, "GetPositionInfo", "<InstanceID>0</InstanceID>")
        pi = parse_response(b, "GetPositionInfo")
        print(f"\n--- poll {n} position: GetPositionInfo -> HTTP {c} ---")
        for k, v in pi.items():
            print(f"  {k}: {v[:140]}{'...' if len(v) > 140 else ''}")

    print()
    print("[avt] done.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
