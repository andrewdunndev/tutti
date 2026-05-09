// Content collection backed by ../evidence/*/captures/*/manifest.json.
// Astro's `glob` loader (5.x) walks the tree at build time. Each
// manifest is a typed entry; the device list page groups by slug,
// the device detail page resolves the latest capture per slug.

import { defineCollection, z } from 'astro:content';
import { glob } from 'astro/loaders';

const didlMetadata = z.object({
  title: z.string().optional(),
  artist: z.string().optional(),
  album: z.string().optional(),
  art_url: z.string().optional(),
  art_card: z.string().optional(),
}).optional();

const decision = z.object({
  result: z.enum(['accepted', 'rejected', 'errored']),
  reason_code: z.string(),
  reason_text: z.string().optional(),
  method: z.enum(['library_call', 'reimplementation']),
  confidence: z.enum(['low', 'medium', 'high']).optional(),
});

const driveRun = z.object({
  scenario: z.string(),
  mime: z.string(),
  rate_hz: z.number().optional(),
  bits: z.number().optional(),
  channels: z.number().optional(),
  metadata_didl_lite_sent: didlMetadata,
  metadata_embedded: didlMetadata,
  metadata_echoed: didlMetadata,
  metadata_source: z.enum(['didl-lite', 'embedded', 'mixed', 'none']).optional(),
  metadata_round_trip: z.enum(['matched', 'mismatched', 'absent']).optional(),
  art_fetch_observed: z.boolean().optional(),
  result: z.enum(['playing', 'transitioning', 'stopped', 'errored', 'rejected']),
  result_reason: z.string().optional(),
  transitions: z.array(z.string()),
  transcript_file: z.string(),
  elapsed_seconds: z.number().optional(),
});

const protocolInfo = z.object({
  source_count: z.number().optional(),
  sink_count: z.number(),
  audio_sink_count: z.number(),
  format_matches: z.record(z.union([z.number(), z.null()])),
  raw_file: z.string(),
});

// descriptorParsed is recursive (a device's embedded_devices are
// themselves descriptors). Zod's z.lazy needs an explicit type
// annotation in TS strict mode for the inferred return type.
type DescriptorParsedShape = {
  device_type: string;
  friendly_name: string;
  manufacturer?: string;
  manufacturer_url?: string;
  model_description?: string;
  model_name?: string;
  model_url?: string;
  model_number?: string;
  serial_number?: string;
  udn: string;
  presentation_url?: string;
  x_dlna_doc?: string;
  icons?: Array<{ mime?: string; width?: number; height?: number; depth?: number; url?: string }>;
  services: Array<{ service_type: string; service_id?: string; scpd_url?: string; control_url?: string; event_sub_url?: string }>;
  embedded_devices?: DescriptorParsedShape[];
};

const descriptorParsed: z.ZodType<DescriptorParsedShape> = z.object({
  device_type: z.string(),
  friendly_name: z.string(),
  manufacturer: z.string().optional(),
  manufacturer_url: z.string().optional(),
  model_description: z.string().optional(),
  model_name: z.string().optional(),
  model_url: z.string().optional(),
  model_number: z.string().optional(),
  serial_number: z.string().optional(),
  udn: z.string(),
  presentation_url: z.string().optional(),
  x_dlna_doc: z.string().optional(),
  icons: z.array(z.object({
    mime: z.string().optional(),
    width: z.number().optional(),
    height: z.number().optional(),
    depth: z.number().optional(),
    url: z.string().optional(),
  })).optional(),
  services: z.array(z.object({
    service_type: z.string(),
    service_id: z.string().optional(),
    scpd_url: z.string().optional(),
    control_url: z.string().optional(),
    event_sub_url: z.string().optional(),
  })),
  embedded_devices: z.array(z.lazy(() => descriptorParsed)).optional(),
});

const mdnsService = z.object({
  service_type: z.string(),
  instance: z.string(),
  hostname: z.string().optional(),
  port: z.number().optional(),
  addrs: z.array(z.string()).optional(),
  txt: z.record(z.string()).optional(),
});

const device = z.object({
  slug: z.string(),
  vendor: z.string().optional(),
  model: z.string().optional(),
  firmware: z.string().optional(),
  udn: z.string().optional(),
  tags: z.array(z.string()).optional(),
  discovery: z.object({
    ssdp_usns: z.array(z.string()).optional(),
    mdns_services: z.array(mdnsService).optional(),
  }),
  descriptor: z.object({
    url_redacted: z.string(),
    raw_file: z.string(),
    parsed: descriptorParsed,
  }).optional(),
  decisions: z.record(decision).optional(),
  protocol_info: protocolInfo.optional(),
  drive_test: z.object({
    performed: z.boolean(),
    skipped_reason: z.string().optional(),
    runs: z.array(driveRun).optional(),
  }).optional(),
});

const manifestSchema = z.object({
  schema_version: z.literal(1),
  tutti_version: z.string(),
  capture_id: z.string(),
  captured_at: z.string(),
  contributor: z.string().nullable().optional(),
  host: z.object({
    os: z.string(),
    arch: z.string(),
    interfaces: z.array(z.string()),
  }),
  scaninfo: z.object({
    ssdp_st_list: z.array(z.string()),
    ssdp_mx: z.number(),
    mdns_service_types: z.array(z.string()),
    drive_requested: z.boolean().optional(),
    drive_force: z.boolean().optional(),
    no_redact: z.boolean().optional(),
    allow_empty: z.boolean().optional(),
  }),
  runstats: z.object({
    elapsed_seconds: z.number(),
    ssdp_responses: z.number(),
    ssdp_unique_usns: z.number(),
    mdns_records: z.number(),
    exit: z.enum(['success', 'partial', 'error']),
  }),
  redactions: z.array(z.string()),
  devices: z.array(device),
});

// captures: every manifest under ../evidence/*/captures/*/manifest.json
// gets its own entry. Path relative to this config file.
const captures = defineCollection({
  loader: glob({
    pattern: '*/captures/*/manifest.json',
    base: '../evidence',
  }),
  schema: manifestSchema,
});

// devices: per-device authored metadata at ../evidence/<slug>/device.json.
// Mirrors schema/device.v1.json. Optional per device; the site falls back
// to capture-derived identity when device.json is absent.
const deviceMetaSchema = z.object({
  schema_version: z.literal(1),
  vendor: z.string().min(1),
  product_name: z.string().min(1),
  tagline: z.string().optional(),
  discontinued: z.boolean().optional(),
  links: z.object({
    manufacturer: z.string().url().optional(),
    support: z.string().url().optional(),
    firmware: z.string().url().optional(),
    manual: z.string().url().optional(),
    purchase: z.string().url().optional(),
  }).optional(),
});

const devices = defineCollection({
  loader: glob({
    pattern: '*/device.json',
    base: '../evidence',
  }),
  schema: deviceMetaSchema,
});

export const collections = { captures, devices };
export type Manifest = z.infer<typeof manifestSchema>;
export type CaptureDevice = z.infer<typeof device>;
export type CaptureDecision = z.infer<typeof decision>;
export type CaptureDriveRun = z.infer<typeof driveRun>;
export type DeviceMeta = z.infer<typeof deviceMetaSchema>;
