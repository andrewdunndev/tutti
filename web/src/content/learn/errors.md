---
title: "Error messages name the next action"
eyebrow: "Design"
description: "Every tutti error is written to tell you what to do next, not just what failed. The retry policy, the message shape, and three exemplars."
order: 5
---


tutti errors are written to name the next action, not just the problem. A user hitting an error should be able to act on it without leaving the terminal: re-run with a different flag, check a firewall, file a bug at the right URL, or open a `tutti capture --debug-network` to see the raw socket setup. Three exemplars below; they're representative of the shape, not exhaustive.

<div class="cat-explainer">
<svg viewBox="0 0 720 240" role="img" aria-label="Anatomy of a tutti error message. Three parts: an Error line that names exactly what failed, a Suggested next step line that proposes the action the user can take to recover, and an optional debug-toggle line pointing at a tutti flag that exposes more state.">
  <rect x="20" y="20" width="680" height="200" fill="#ffffff" stroke="#e0e0e0" stroke-width="1" rx="3"/>
  <text x="40" y="46" font-family="IBM Plex Mono, monospace" font-size="10" font-weight="700" fill="#707070" letter-spacing="1.4">ERROR ANATOMY</text>

  <rect x="40" y="64" width="420" height="140" rx="3" fill="#f4f4f4" stroke="#e0e0e0" stroke-width="1"/>
  <text x="56" y="86" font-family="IBM Plex Mono, monospace" font-size="11" font-weight="700" fill="#0a0a0a">Error: no SSDP responses received</text>
  <text x="56" y="102" font-family="IBM Plex Mono, monospace" font-size="11" fill="#0a0a0a">on any interface (en0, en1).</text>
  <text x="56" y="132" font-family="IBM Plex Mono, monospace" font-size="11" fill="#404040">Suggested next step: re-run with</text>
  <text x="56" y="148" font-family="IBM Plex Mono, monospace" font-size="11" fill="#404040">`--interface &lt;name&gt;` or check firewall.</text>
  <text x="56" y="184" font-family="IBM Plex Mono, monospace" font-size="11" fill="#707070">Run `tutti capture --debug-network`.</text>

  <line x1="466" y1="92" x2="500" y2="92" stroke="#4a6741" stroke-width="1.5"/>
  <text x="510" y="86" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="#4a6741" letter-spacing="1.2">WHAT FAILED</text>
  <text x="510" y="100" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">precise, with context</text>
  <text x="510" y="114" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">(interfaces that were tried)</text>

  <line x1="466" y1="140" x2="500" y2="140" stroke="#4a6741" stroke-width="1.5"/>
  <text x="510" y="134" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="#4a6741" letter-spacing="1.2">NEXT ACTION</text>
  <text x="510" y="148" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">concrete, runnable</text>
  <text x="510" y="162" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">commands the user can paste</text>

  <line x1="466" y1="190" x2="500" y2="190" stroke="#707070" stroke-width="1.5" stroke-dasharray="3,2"/>
  <text x="510" y="184" font-family="IBM Plex Mono, monospace" font-size="9" font-weight="700" fill="#707070" letter-spacing="1.2">DEBUG ESCAPE</text>
  <text x="510" y="198" font-family="IBM Plex Sans, sans-serif" font-size="10" fill="#404040">flag for "show me more"</text>
</svg>

Every tutti error has the first two parts. The debug-escape line is added when there's a useful flag to expose more state. <em>What failed</em> is the diagnostic. <em>Next action</em> is the recovery. The user shouldn't have to read documentation, file an issue, or guess what to try first.
</div>

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
