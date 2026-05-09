package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gitlab.com/dunn.dev/tutti/internal/schema"
)

func runValidate(_ context.Context, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: tutti validate requires a capture directory.")
		fmt.Fprintln(os.Stderr, "  Suggested next step: pass the path to a capture-* directory produced by `tutti capture`.")
		return 2
	}

	dir := args[0]
	manifestPath := filepath.Join(dir, "manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not read %s: %v.\n", manifestPath, err)
		fmt.Fprintln(os.Stderr, "  Suggested next step: confirm the directory contains a manifest.json.")
		return 1
	}

	v, err := schema.NewValidator()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: embedded schema is invalid: %v.\n", err)
		fmt.Fprintln(os.Stderr, "  This is a tutti bug; please file at https://gitlab.com/dunn.dev/tutti/-/issues.")
		return 1
	}

	if err := v.ValidateBytes(raw); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s does not pass schema v%d.\n", manifestPath, schema.SchemaVersion)
		fmt.Fprintln(os.Stderr, indent(err.Error(), "  "))
		fmt.Fprintln(os.Stderr, "  Suggested next step: re-run `tutti capture` to regenerate. If the failure reproduces, file a bug with the manifest attached.")
		return 1
	}

	var m schema.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		fmt.Fprintf(os.Stderr, "Error: manifest passes schema-shape but cannot be unmarshalled into the Go type: %v.\n", err)
		fmt.Fprintln(os.Stderr, "  This indicates a schema/types drift; tutti bug.")
		return 1
	}

	if err := v.CheckVacuity(&m); err != nil {
		var ve *schema.VacuityError
		if errors.As(err, &ve) {
			fmt.Fprintf(os.Stderr, "Error: capture is technically valid but vacuous: %s\n", ve.Rule)
			fmt.Fprintf(os.Stderr, "  Suggested next step: %s\n", ve.Hint)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return 1
	}

	fmt.Printf("OK: %s passes schema v%d (capture_id=%s, %d device(s)).\n",
		manifestPath, schema.SchemaVersion, m.CaptureID, len(m.Devices))
	return 0
}

func indent(s, prefix string) string {
	out := ""
	for _, line := range splitLines(s) {
		out += prefix + line + "\n"
	}
	return out
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
