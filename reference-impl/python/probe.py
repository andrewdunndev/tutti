#!/usr/bin/env python3
"""SSDP M-SEARCH probe (the SSDP layer of `tutti capture`).

Sends an M-SEARCH multicast for the standard audio-renderer search targets,
listens for unicast replies, and prints them grouped by USN. Stdlib only.
"""
from __future__ import annotations

import argparse
import socket
import sys
import time

MCAST = "239.255.255.250"
PORT = 1900
DEFAULT_STS = [
    "ssdp:all",
    "upnp:rootdevice",
    "urn:schemas-upnp-org:device:MediaRenderer:1",
    "urn:schemas-upnp-org:service:AVTransport:1",
]


def msearch_packet(st: str, mx: int) -> bytes:
    return (
        "M-SEARCH * HTTP/1.1\r\n"
        f"HOST: {MCAST}:{PORT}\r\n"
        'MAN: "ssdp:discover"\r\n'
        f"MX: {mx}\r\n"
        f"ST: {st}\r\n"
        "\r\n"
    ).encode()


def parse(raw: bytes, src: tuple[str, int]) -> dict:
    """Lenient HTTP/1.1 response parser. Returns a dict of canonical fields."""
    text = raw.decode("latin-1", errors="replace")
    lines = text.replace("\r\n", "\n").split("\n")
    headers: dict[str, str] = {}
    for line in lines[1:]:
        if not line.strip() or ":" not in line:
            continue
        k, _, v = line.partition(":")
        headers[k.strip().upper()] = v.strip()
    return {
        "USN": headers.get("USN", ""),
        "ST": headers.get("ST", headers.get("NT", "")),
        "LOCATION": headers.get("LOCATION", ""),
        "SERVER": headers.get("SERVER", ""),
        "BOOTID": headers.get("BOOTID.UPNP.ORG", ""),
        "CONFIGID": headers.get("CONFIGID.UPNP.ORG", ""),
        "CACHE-CONTROL": headers.get("CACHE-CONTROL", ""),
        "FROM": f"{src[0]}:{src[1]}",
        "_raw": text,
    }


def probe(interface_ip: str | None, sts: list[str], mx: int) -> list[dict]:
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
    s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    bind_to = interface_ip or "0.0.0.0"
    s.bind((bind_to, 0))
    s.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_TTL, 4)
    if interface_ip:
        s.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_IF,
                     socket.inet_aton(interface_ip))

    for st in sts:
        s.sendto(msearch_packet(st, mx), (MCAST, PORT))

    s.settimeout(0.5)
    deadline = time.time() + mx + 2
    out: list[dict] = []
    while time.time() < deadline:
        try:
            raw, src = s.recvfrom(8192)
        except socket.timeout:
            continue
        if not raw.startswith(b"HTTP/1.1"):
            continue
        out.append(parse(raw, src))
    s.close()
    return out


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--interface", help="bind to a specific local IPv4")
    ap.add_argument("--mx", type=int, default=3, help="MX header in seconds (1-5)")
    args = ap.parse_args()

    print(f"[ssdp] M-SEARCH -> {MCAST}:{PORT}, MX={args.mx}")
    for st in DEFAULT_STS:
        print(f"[ssdp]   sent ST: {st}")
    print(f"[ssdp] listening {args.mx + 2}s ...")
    print()

    responses = probe(args.interface, DEFAULT_STS, args.mx)

    locations = sorted({r["LOCATION"] for r in responses if r["LOCATION"]})
    by_usn: dict[str, list[dict]] = {}
    for r in responses:
        by_usn.setdefault(r["USN"] or "<no-usn>", []).append(r)

    print(f"[ssdp] {len(responses)} raw response(s), "
          f"{len(by_usn)} unique USN(s), {len(locations)} unique LOCATION(s)")
    print()
    print("=" * 78)
    print("LOCATIONS (feed each into fetch.py)")
    print("=" * 78)
    for loc in locations:
        print(f"  {loc}")
    print()
    print("=" * 78)
    print("DEVICES (grouped by USN)")
    print("=" * 78)
    for usn in sorted(by_usn):
        print()
        print(f"--- {usn} ---")
        r = by_usn[usn][0]
        print(f"  FROM:     {r['FROM']}")
        print(f"  ST:       {r['ST']}")
        print(f"  LOCATION: {r['LOCATION']}")
        print(f"  SERVER:   {r['SERVER']}")
        if r["BOOTID"]:
            print(f"  BOOTID.UPNP.ORG: {r['BOOTID']}")
        if r["CONFIGID"]:
            print(f"  CONFIGID.UPNP.ORG: {r['CONFIGID']}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
