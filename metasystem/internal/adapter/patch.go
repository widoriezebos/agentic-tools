package adapter

import (
	"fmt"
	"os"
)

// These writers emit the small JSON patch files the record lifecycle applies
// under its own lock. Each is handed to `dispatch record-cas --patch`, which
// compare-and-swaps the named fields into the job record; writing a patch file
// keeps the single-writer lock where it belongs instead of duplicating it here.

// WriteModelPatch writes a patch that records the model the runtime actually
// ran under.
func WriteModelPatch(outputPath, model string) error {
	if model == "" {
		return fmt.Errorf("effective model is required")
	}
	return atomicWriteJSON(outputPath, map[string]any{"effectiveModel": model})
}

// WriteTransportPatch pins the job's transport into its record —
// a chain never switches transports (D82 fix-forward), and the
// pin is what lets both branches refuse a switch.
func WriteTransportPatch(outputPath, transport string) error {
	if transport != "acp" && transport != "legacy" {
		return fmt.Errorf("transport must be acp or legacy")
	}
	return atomicWriteJSON(outputPath, map[string]any{"transport": transport})
}

// WriteRepairsPatch writes a patch that records how many return-repair turns a
// round needed, so a chain that got its return right only after a repair never
// reads as one that got it right the first time.
func WriteRepairsPatch(outputPath string, count int) error {
	return atomicWriteJSON(outputPath, map[string]any{"returnRepairs": count})
}

// WriteResultPatch writes the terminal patch for a round: the failure code (or
// null on success), the phase it settled in, and the typed usage read from
// usagePath. An empty usagePath, or one that is not a regular file, records a
// null usage; a present file must hold valid JSON.
func WriteResultPatch(outputPath, failure, phase, usagePath string) error {
	patch := map[string]any{
		"error": errorValue(failure),
		"phase": phase,
		"usage": nil,
	}
	if usagePath != "" {
		if info, err := os.Stat(usagePath); err == nil && info.Mode().IsRegular() {
			usage, err := decodeJSON(usagePath)
			if err != nil {
				return fmt.Errorf("cannot read usage file %s: %w", usagePath, err)
			}
			patch["usage"] = usage
		}
	}
	return atomicWriteJSON(outputPath, patch)
}

// errorValue maps the literal "null" to a JSON null and any other string to
// itself, so a success ("null") and a failure code write the same field.
func errorValue(failure string) any {
	if failure == "null" {
		return nil
	}
	return failure
}
