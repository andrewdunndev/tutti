---
title: "Error messages name the next action"
eyebrow: "Design"
description: "Every tutti error is written to tell you what to do next, not just what failed. The retry policy, the message shape, and three exemplars."
order: 5
---


tutti errors are written to name the next action, not just the problem. A user hitting an error should be able to act on it without leaving the terminal: re-run with a different flag, check a firewall, file a bug at the right URL, or open a `tutti capture --debug-network` to see the raw socket setup. Three exemplars below; they're representative of the shape, not exhaustive.

---

## Exemplar: no SSDP responses

```
$ tutti capture
Error: no SSDP responses received on any interface (en0, en1).
Suggested next step: re-run with `--interface <name>` to scan a
specific interface, or check that your firewall permits inbound
multicast on UDP/1900. Run `tutti capture --debug-network` to see the
raw socket setup.
```

Names which interfaces were tried so you know the binding happened. Two concrete next actions (scope the scan or open the firewall) and a debug toggle for the case where neither is the issue. tutti does not infer which to suggest first; the user knows their network better than tutti does.

---

## Exemplar: missing required manifest field

```
$ tutti validate ./capture-2026-05-09T143022Z-myhost
Error: manifest.json is missing required field `devices[0].decisions`.
Suggested next step: this is a tutti bug, not a capture you can
hand-fix. Re-run `tutti capture` to regenerate. If it reproduces, file
the failing capture-id at https://gitlab.com/dunn.dev/tutti/-/issues.
```

Distinguishes "you can fix this" from "tutti can fix this." A user with a capture from a long expedition does not want to be told to re-run if hand-editing `notes.md` is sufficient. Here the missing field can't come from anything but a tutti code path, so the next action is upstream.

---

## Exemplar: descriptor fetch failure under `--drive`

```
$ tutti capture --drive
Error: descriptor fetch failed for eversolo-dmp-a6 after 3 attempts
(http://<DEVICE_IP>:1054/description.xml: connection refused).
Suggested next step: confirm the device is on (its UDN was advertised
2 seconds ago) and re-run. If the device is reachable but tutti
can't reach it, check the firewall on the host running tutti, or
pass `--interface <name>` to scan a specific NIC.
```

Carries the contradictory evidence in the message: the device advertised itself 2 seconds ago, but its descriptor port is closed. That contradiction is the diagnostic value. The next-action list is ordered by probability.

---

## Retry policy

Network operations retry up to three times with exponential backoff and then bail with a prescriptive error. Library decision calls do not retry; they are deterministic against a fetched descriptor.
