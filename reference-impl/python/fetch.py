#!/usr/bin/env python3
"""Fetch and parse a UPnP device descriptor.

Pass a LOCATION URL (typically from probe.py's output). Prints a parsed
summary plus the raw XML. Stdlib only.
"""
from __future__ import annotations

import argparse
import sys
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET

NS = {"d": "urn:schemas-upnp-org:device-1-0"}


def localname(tag: str) -> str:
    """Strip XML namespace prefix from a tag name."""
    return tag.split("}", 1)[-1]


def fetch(url: str, timeout: float = 10.0) -> bytes:
    req = urllib.request.Request(url, headers={"Connection": "close"})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return resp.read()


def text_of(elem: ET.Element | None, tag: str) -> str:
    if elem is None:
        return ""
    for child in elem:
        if localname(child.tag) == tag and child.text:
            return child.text.strip()
    return ""


def find_device(root: ET.Element) -> ET.Element | None:
    for child in root:
        if localname(child.tag) == "device":
            return child
    return None


def parse_services(device: ET.Element, base: str) -> list[dict]:
    out: list[dict] = []
    for child in device:
        if localname(child.tag) != "serviceList":
            continue
        for svc in child:
            if localname(svc.tag) != "service":
                continue
            entry = {
                "serviceType": text_of(svc, "serviceType"),
                "serviceId": text_of(svc, "serviceId"),
                "SCPDURL": urllib.parse.urljoin(base, text_of(svc, "SCPDURL")),
                "controlURL": urllib.parse.urljoin(base, text_of(svc, "controlURL")),
                "eventSubURL": urllib.parse.urljoin(base, text_of(svc, "eventSubURL")),
            }
            out.append(entry)
    return out


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("url", help="LOCATION URL of the UPnP device descriptor")
    args = ap.parse_args()

    print(f"[fetch] GET {args.url}")
    raw = fetch(args.url)
    print(f"[fetch] {len(raw)} bytes")
    root = ET.fromstring(raw)
    device = find_device(root)
    if device is None:
        print("[fetch] no <device> element in descriptor", file=sys.stderr)
        return 1

    services = parse_services(device, args.url)

    print()
    print("DEVICE")
    for field in (
        "deviceType", "friendlyName", "manufacturer", "manufacturerURL",
        "modelDescription", "modelName", "modelURL", "modelNumber",
        "serialNumber", "UDN", "presentationURL", "X_DLNADOC",
    ):
        v = text_of(device, field)
        if v:
            print(f"  {field}: {v}")

    if services:
        print("  services:")
        for s in services:
            print(f"    - type:    {s['serviceType']}")
            print(f"      id:      {s['serviceId']}")
            print(f"      SCPD:    {s['SCPDURL']}")
            print(f"      control: {s['controlURL']}")
            print(f"      event:   {s['eventSubURL']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
