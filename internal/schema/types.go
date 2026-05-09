// Package schema holds the in-memory shape of a tutti capture manifest
// (schema v1). The single source of truth is schema/manifest.v1.json at
// the repo root; these types must round-trip with that schema.
package schema

import _ "embed"

// SchemaVersion is the only valid value for Manifest.SchemaVersion in v1.
const SchemaVersion = 1

// SchemaJSON is the raw bytes of the v1 JSON Schema document. Bundled
// with the binary so tutti validate works without a network round trip.
// The canonical source file is schema/manifest.v1.json at the repo root;
// this copy is kept in sync by `make sync-schema` (and a CI guard once
// the pipeline lands).
//
//go:embed manifest.v1.json
var SchemaJSON []byte

// Redaction is the controlled vocabulary for the manifest.redactions array.
type Redaction string

const (
	RedactionClientIP         Redaction = "client-ip"
	RedactionDeviceIP         Redaction = "device-ip"
	RedactionAuthTokens       Redaction = "auth-tokens"
	RedactionSalt             Redaction = "salt"
	RedactionUsername         Redaction = "username"
	RedactionLanTopology      Redaction = "lan-topology"
	RedactionInternalHostname Redaction = "internal-hostname"
)

// Result is the outcome of a single library decision.
type Result string

const (
	ResultAccepted Result = "accepted"
	ResultRejected Result = "rejected"
	ResultErrored  Result = "errored"
)

// Method describes how a decision was reached.
type Method string

const (
	MethodLibraryCall     Method = "library_call"
	MethodReimplementation Method = "reimplementation"
)

// Confidence is a coarse, tutti-author-set indication of how reliable a
// decision is. Library calls are typically high; heuristic reimplementations
// may be medium.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// MetadataSource is derived during a drive run by comparing what the device
// echoed in GetPositionInfo against the two metadata surfaces tutti set.
type MetadataSource string

const (
	MetadataSourceDIDLLite MetadataSource = "didl-lite"
	MetadataSourceEmbedded MetadataSource = "embedded"
	MetadataSourceMixed    MetadataSource = "mixed"
	MetadataSourceNone     MetadataSource = "none"
)

// MetadataRoundTrip records whether the device echoed back what tutti sent
// in DIDL-Lite (or embedded tags, depending on metadata_source).
type MetadataRoundTrip string

const (
	MetadataRoundTripMatched    MetadataRoundTrip = "matched"
	MetadataRoundTripMismatched MetadataRoundTrip = "mismatched"
	MetadataRoundTripAbsent     MetadataRoundTrip = "absent"
)

// DriveResult is the outcome classification of one drive run.
type DriveResult string

const (
	DriveResultPlaying       DriveResult = "playing"
	DriveResultTransitioning DriveResult = "transitioning"
	DriveResultStopped       DriveResult = "stopped"
	DriveResultErrored       DriveResult = "errored"
	DriveResultRejected      DriveResult = "rejected"
)

// Exit summarises the run outcome at the top level of runstats.
type Exit string

const (
	ExitSuccess Exit = "success"
	ExitPartial Exit = "partial"
	ExitError   Exit = "error"
)

// Manifest is the top-level shape of capture/manifest.json.
type Manifest struct {
	SchemaVersion int        `json:"schema_version"`
	TuttiVersion  string     `json:"tutti_version"`
	CaptureID     string     `json:"capture_id"`
	CapturedAt    string     `json:"captured_at"`
	Contributor   *string    `json:"contributor,omitempty"`
	Host          Host       `json:"host"`
	ScanInfo      ScanInfo   `json:"scaninfo"`
	RunStats      RunStats   `json:"runstats"`
	Redactions    []Redaction `json:"redactions"`
	Devices       []Device    `json:"devices"`
}

type Host struct {
	OS         string   `json:"os"`
	Arch       string   `json:"arch"`
	Interfaces []string `json:"interfaces"`
}

type ScanInfo struct {
	SsdpStList       []string `json:"ssdp_st_list"`
	SsdpMx           int      `json:"ssdp_mx"`
	MdnsServiceTypes []string `json:"mdns_service_types"`
	DriveRequested   bool     `json:"drive_requested,omitempty"`
	DriveForce       bool     `json:"drive_force,omitempty"`
	NoRedact         bool     `json:"no_redact,omitempty"`
	AllowEmpty       bool     `json:"allow_empty,omitempty"`
}

type RunStats struct {
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	SsdpResponses  int     `json:"ssdp_responses"`
	SsdpUniqueUsns int     `json:"ssdp_unique_usns"`
	MdnsRecords    int     `json:"mdns_records"`
	Exit           Exit    `json:"exit"`
}

// Device is one entry in manifest.devices. Discovery is always present;
// at least one of decisions / protocol_info / drive_test / descriptor must
// be present.
type Device struct {
	Slug         string             `json:"slug"`
	Vendor       string             `json:"vendor,omitempty"`
	Model        string             `json:"model,omitempty"`
	Firmware     string             `json:"firmware,omitempty"`
	UDN          string             `json:"udn,omitempty"`
	Tags         []string           `json:"tags,omitempty"`
	Discovery    Discovery          `json:"discovery"`
	Descriptor   *Descriptor        `json:"descriptor,omitempty"`
	Decisions    map[string]Decision `json:"decisions,omitempty"`
	ProtocolInfo *ProtocolInfo      `json:"protocol_info,omitempty"`
	DriveTest    *DriveTest         `json:"drive_test,omitempty"`
}

type Discovery struct {
	SsdpUSNs     []string       `json:"ssdp_usns,omitempty"`
	MdnsServices []MDNSService  `json:"mdns_services,omitempty"`
}

type MDNSService struct {
	ServiceType string            `json:"service_type"`
	Instance    string            `json:"instance"`
	Hostname    string            `json:"hostname,omitempty"`
	Port        int               `json:"port,omitempty"`
	Addrs       []string          `json:"addrs,omitempty"`
	TXT         map[string]string `json:"txt,omitempty"`
}

type Descriptor struct {
	URLRedacted string                  `json:"url_redacted"`
	RawFile     string                  `json:"raw_file"`
	Parsed      DescriptorParsed        `json:"parsed"`
}

type DescriptorParsed struct {
	DeviceType       string             `json:"device_type"`
	FriendlyName     string             `json:"friendly_name"`
	Manufacturer     string             `json:"manufacturer,omitempty"`
	ManufacturerURL  string             `json:"manufacturer_url,omitempty"`
	ModelDescription string             `json:"model_description,omitempty"`
	ModelName        string             `json:"model_name,omitempty"`
	ModelURL         string             `json:"model_url,omitempty"`
	ModelNumber      string             `json:"model_number,omitempty"`
	SerialNumber     string             `json:"serial_number,omitempty"`
	UDN              string             `json:"udn"`
	PresentationURL  string             `json:"presentation_url,omitempty"`
	XDlnaDoc         string             `json:"x_dlna_doc,omitempty"`
	Icons            []DescriptorIcon   `json:"icons,omitempty"`
	Services         []DescriptorService `json:"services"`
	EmbeddedDevices  []DescriptorParsed `json:"embedded_devices,omitempty"`
}

type DescriptorIcon struct {
	MIME   string `json:"mime,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Depth  int    `json:"depth,omitempty"`
	URL    string `json:"url,omitempty"`
}

type DescriptorService struct {
	ServiceType string `json:"service_type"`
	ServiceID   string `json:"service_id,omitempty"`
	ScpdURL     string `json:"scpd_url,omitempty"`
	ControlURL  string `json:"control_url,omitempty"`
	EventSubURL string `json:"event_sub_url,omitempty"`
}

type Decision struct {
	Result     Result     `json:"result"`
	ReasonCode string     `json:"reason_code"`
	ReasonText string     `json:"reason_text,omitempty"`
	Method     Method     `json:"method"`
	Confidence Confidence `json:"confidence,omitempty"`
}

type ProtocolInfo struct {
	SourceCount    int            `json:"source_count,omitempty"`
	SinkCount      int            `json:"sink_count"`
	AudioSinkCount int            `json:"audio_sink_count"`
	FormatMatches  map[string]*int `json:"format_matches"`
	RawFile        string         `json:"raw_file"`
}

type DriveTest struct {
	Performed     bool       `json:"performed"`
	SkippedReason string     `json:"skipped_reason,omitempty"`
	Runs          []DriveRun `json:"runs,omitempty"`
}

type DriveRun struct {
	Scenario               string            `json:"scenario"`
	MIME                   string            `json:"mime"`
	RateHz                 int               `json:"rate_hz,omitempty"`
	Bits                   int               `json:"bits,omitempty"`
	Channels               int               `json:"channels,omitempty"`
	MetadataDIDLLiteSent   *DIDLMetadata     `json:"metadata_didl_lite_sent,omitempty"`
	MetadataEmbedded       *DIDLMetadata     `json:"metadata_embedded,omitempty"`
	MetadataEchoed         *DIDLMetadata     `json:"metadata_echoed,omitempty"`
	MetadataSource         MetadataSource    `json:"metadata_source,omitempty"`
	MetadataRoundTrip      MetadataRoundTrip `json:"metadata_round_trip,omitempty"`
	ArtFetchObserved       bool              `json:"art_fetch_observed,omitempty"`
	Result                 DriveResult       `json:"result"`
	ResultReason           string            `json:"result_reason,omitempty"`
	Transitions            []string          `json:"transitions"`
	TranscriptFile         string            `json:"transcript_file"`
	ElapsedSeconds         float64           `json:"elapsed_seconds,omitempty"`
}

type DIDLMetadata struct {
	Title   string `json:"title,omitempty"`
	Artist  string `json:"artist,omitempty"`
	Album   string `json:"album,omitempty"`
	ArtURL  string `json:"art_url,omitempty"`
	ArtCard string `json:"art_card,omitempty"`
}
