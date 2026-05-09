// Group captures by device. The corpus is keyed by directory:
//   evidence/<vendor-model>/captures/<ts>-<contributor>/manifest.json
// The first path segment is the device key. Captures sort newest-first
// by captured_at; the page renders the latest as the headline state and
// older captures as a history strip.

import type { CollectionEntry } from 'astro:content';

export type CaptureEntry = CollectionEntry<'captures'>;
export type DeviceMetaEntry = CollectionEntry<'devices'>;

export interface DeviceCorpus {
  slug: string;
  // Canonical identity: prefer authored device.json; fall back to
  // capture-derived (descriptor manufacturer + friendlyName/modelName)
  // when the slug has no device.json yet.
  vendor: string | undefined;
  model: string | undefined;
  // tagline + links are authored-only; absent when device.json is missing.
  meta: DeviceMetaEntry['data'] | undefined;
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
 * deviceMetas is the authored device.json collection, indexed by slug.
 * When a slug has authored metadata, vendor + model come from there
 * (canonical product identity); otherwise we fall back to descriptor's
 * manufacturer + modelName, with a final fallback to the friendlyName
 * (which can leak per-installation tags like "livingroom" — used only
 * when nothing better exists).
 */
export function groupCaptures(
  entries: CaptureEntry[],
  deviceMetas: DeviceMetaEntry[] = [],
): Map<string, DeviceCorpus> {
  const metaBySlug = new Map<string, DeviceMetaEntry>();
  for (const m of deviceMetas) {
    // device.json entry id is "<slug>/device" under the base.
    const slug = m.id.split('/')[0] ?? m.id;
    metaBySlug.set(slug, m);
  }

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
    const meta = metaBySlug.get(slug);
    const vendor = meta?.data.vendor ?? headline.vendor;
    const model =
      meta?.data.product_name
      ?? (headline.descriptor?.parsed.model_name && headline.descriptor.parsed.model_name !== 'AV Renderer Device'
        ? headline.descriptor.parsed.model_name
        : undefined)
      ?? headline.descriptor?.parsed.friendly_name
      ?? slug;
    out.set(slug, {
      slug,
      vendor,
      model,
      meta: meta?.data,
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
