// Group captures by device. The corpus is keyed by directory:
//   evidence/<vendor-model>/captures/<ts>-<contributor>/manifest.json
// The first path segment is the device key. Captures sort newest-first
// by captured_at; the page renders the latest as the headline state and
// older captures as a history strip.

import type { CollectionEntry } from 'astro:content';

export type CaptureEntry = CollectionEntry<'captures'>;

export interface DeviceCorpus {
  slug: string;
  vendor: string | undefined;
  model: string | undefined;
  latest: CaptureEntry;
  history: CaptureEntry[];
  // Headline state pulled from the latest capture's first device entry.
  // (Most captures so far have one device per manifest; if more, we use [0].)
  headlineDevice: NonNullable<CaptureEntry['data']['devices'][number]>;
}

/**
 * deviceSlugFromEntry pulls the device key from the manifest path,
 * which lives under ../evidence/<slug>/captures/.../manifest.json.
 */
export function deviceSlugFromEntry(entry: CaptureEntry): string {
  // entry.id is a relative path like
  // "eversolo-dmp-a6/captures/20260509T133946Z-andrewdunndev/manifest"
  const parts = entry.id.split('/');
  return parts[0] ?? 'unknown';
}

/**
 * groupCaptures returns a Map keyed by device slug. Each value carries
 * the latest capture, the full history (newest-first), and a headline
 * device record lifted from the latest manifest's first devices[] entry.
 *
 * Captures whose first devices[] entry slug differs from the path slug
 * still group under the path slug: the path is the canonical anchor.
 */
export function groupCaptures(entries: CaptureEntry[]): Map<string, DeviceCorpus> {
  const byDevice = new Map<string, CaptureEntry[]>();
  for (const e of entries) {
    const slug = deviceSlugFromEntry(e);
    const list = byDevice.get(slug) ?? [];
    list.push(e);
    byDevice.set(slug, list);
  }

  const out = new Map<string, DeviceCorpus>();
  for (const [slug, list] of byDevice) {
    list.sort((a, b) => b.data.captured_at.localeCompare(a.data.captured_at));
    const latest = list[0];
    const headline = latest?.data.devices[0];
    if (!latest || !headline) continue;
    out.set(slug, {
      slug,
      vendor: headline.vendor,
      model: headline.model ?? headline.descriptor?.parsed.friendly_name,
      latest,
      history: list,
      headlineDevice: headline,
    });
  }
  return out;
}

/**
 * decisionSummary collapses the per-library decision map into a single
 * label: "all-accepted", "all-rejected", "split", "none". Used in the
 * device list to give an at-a-glance verdict without consuming a column
 * per library.
 */
export function decisionSummary(device: DeviceCorpus['headlineDevice']): {
  label: string;
  tone: 'ok' | 'fail' | 'warn' | 'mute';
} {
  const decisions = device.decisions;
  if (!decisions || Object.keys(decisions).length === 0) {
    return { label: 'no decisions', tone: 'mute' };
  }
  const results = Object.values(decisions).map(d => d.result);
  const accepted = results.filter(r => r === 'accepted').length;
  const rejected = results.filter(r => r === 'rejected').length;
  if (accepted === results.length) return { label: 'all accepted', tone: 'ok' };
  if (rejected === results.length) return { label: 'all rejected', tone: 'fail' };
  return { label: 'split', tone: 'warn' };
}

/**
 * driveSummary describes the drive_test outcome at-a-glance.
 */
export function driveSummary(device: DeviceCorpus['headlineDevice']): {
  label: string;
  tone: 'ok' | 'fail' | 'warn' | 'mute';
} {
  const dt = device.drive_test;
  if (!dt) return { label: 'not run', tone: 'mute' };
  if (!dt.performed) return { label: dt.skipped_reason ?? 'skipped', tone: 'mute' };
  const runs = dt.runs ?? [];
  if (runs.length === 0) return { label: 'no runs', tone: 'mute' };
  const playing = runs.filter(r => r.result === 'playing').length;
  if (playing === runs.length) return { label: `${playing}/${runs.length} playing`, tone: 'ok' };
  if (playing === 0) return { label: `0/${runs.length} playing`, tone: 'fail' };
  return { label: `${playing}/${runs.length} playing`, tone: 'warn' };
}

/**
 * formatDate pulls a short YYYY-MM-DD label from an ISO captured_at.
 */
export function formatDate(iso: string): string {
  return iso.slice(0, 10);
}
