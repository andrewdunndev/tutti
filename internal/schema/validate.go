package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator wraps a compiled JSON Schema for the bundled manifest.v1.
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator compiles the embedded v1 schema. Returns an error if the
// schema itself is invalid (unreachable in shipped binaries; a unit test
// guards against regression).
func NewValidator() (*Validator, error) {
	c := jsonschema.NewCompiler()
	resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(SchemaJSON))
	if err != nil {
		return nil, fmt.Errorf("parse embedded schema: %w", err)
	}
	if err := c.AddResource("manifest.v1.json", resource); err != nil {
		return nil, fmt.Errorf("register embedded schema: %w", err)
	}
	sch, err := c.Compile("manifest.v1.json")
	if err != nil {
		return nil, fmt.Errorf("compile embedded schema: %w", err)
	}
	return &Validator{schema: sch}, nil
}

// ValidateBytes checks raw manifest JSON against schema v1.
func (v *Validator) ValidateBytes(b []byte) error {
	var inst any
	if err := json.Unmarshal(b, &inst); err != nil {
		return fmt.Errorf("manifest is not valid JSON: %w", err)
	}
	if err := v.schema.Validate(inst); err != nil {
		return shapeError(err)
	}
	return nil
}

// ValidateManifest validates an in-memory Manifest by round-tripping through
// JSON. Slower than ValidateBytes but useful in tests and capture self-check.
func (v *Validator) ValidateManifest(m *Manifest) error {
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return v.ValidateBytes(b)
}

// VacuityError is returned when a manifest is well-formed against the
// schema but fails the project's vacuity rules. Documented in the README.
type VacuityError struct {
	Rule string
	Hint string
}

func (e *VacuityError) Error() string {
	return fmt.Sprintf("vacuity rule failed: %s. %s", e.Rule, e.Hint)
}

// CheckVacuity applies the rules from the README's "Validation rules
// beyond schema-shape" section. Returns nil if the manifest carries
// non-trivial evidence; returns *VacuityError otherwise.
func (v *Validator) CheckVacuity(m *Manifest) error {
	if !m.ScanInfo.AllowEmpty {
		if m.RunStats.SsdpResponses == 0 && m.RunStats.MdnsRecords == 0 {
			return &VacuityError{
				Rule: "no SSDP and no mDNS records captured",
				Hint: "If your LAN really has no audio renderers announcing on either protocol, " +
					"re-run with --allow-empty. Otherwise check firewall (UDP/1900, UDP/5353) " +
					"and that you scanned the right interface.",
			}
		}
	}
	if !m.ScanInfo.NoRedact && len(m.Redactions) == 0 {
		return &VacuityError{
			Rule: "redactions array is empty without --no-redact",
			Hint: "tutti must record which redactions it applied. If you used --no-redact, " +
				"the manifest's scaninfo.no_redact must be true and the array can stay empty.",
		}
	}
	for i, dev := range m.Devices {
		if dev.Decisions != nil && len(dev.Decisions) == 0 {
			return &VacuityError{
				Rule: fmt.Sprintf("devices[%d] (%s) has empty decisions map", i, dev.Slug),
				Hint: "An empty decisions map means tutti recorded the field but no library was " +
					"actually invoked. This is a tutti bug; please re-run.",
			}
		}
		if dev.DriveTest != nil && dev.DriveTest.Performed && len(dev.DriveTest.Runs) == 0 {
			return &VacuityError{
				Rule: fmt.Sprintf("devices[%d] (%s) has drive_test.performed=true but no runs", i, dev.Slug),
				Hint: "If the drive test was attempted but every scenario failed, set " +
					"drive_test.performed=false and drive_test.skipped_reason; otherwise capture at " +
					"least one run with result=errored.",
			}
		}
	}
	return nil
}

// shapeError flattens jsonschema.Validate's error tree into a single
// human-readable error per the project's "prescriptive failure" principle.
func shapeError(err error) error {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return err
	}
	var b strings.Builder
	b.WriteString("manifest fails schema v1:\n")
	walk(&b, ve, "  ")
	return errors.New(strings.TrimRight(b.String(), "\n"))
}

func walk(b *strings.Builder, ve *jsonschema.ValidationError, indent string) {
	loc := "/" + strings.Join(ve.InstanceLocation, "/")
	if len(ve.InstanceLocation) == 0 {
		loc = "(root)"
	}
	fmt.Fprintf(b, "%sat %s: %s\n", indent, loc, ve.ErrorKind)
	for _, c := range ve.Causes {
		walk(b, c, indent+"  ")
	}
}
