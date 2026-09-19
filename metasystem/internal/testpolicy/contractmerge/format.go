package contractmerge

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Render emits the testing contract's canonical repository format: stable
// top-level lines and one compact line for every surface and group.
func Render(contract testpolicy.Contract) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("{\n")
	fmt.Fprintf(&out, "  \"schemaVersion\": %d,\n", contract.SchemaVersion)
	if contract.Impacted != nil {
		impacted, err := json.Marshal(contract.Impacted)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&out, "  \"impacted\": %s,\n", impacted)
	}
	risk, err := renderProjectRisk(contract.ProjectRisk)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&out, "  \"projectRisk\": %s,\n", risk)
	if contract.Fallback != "" {
		fallback, _ := json.Marshal(contract.Fallback)
		fmt.Fprintf(&out, "  \"fallback\": %s,\n", fallback)
	}
	if err := renderObjectArray(&out, "surfaces", contract.Surfaces); err != nil {
		return nil, err
	}
	if err := renderObjectArray(&out, "groups", contract.Groups); err != nil {
		return nil, err
	}
	always, err := json.Marshal(contract.Always)
	if err != nil {
		return nil, err
	}
	unknown, err := json.Marshal(contract.Unknown)
	if err != nil {
		return nil, err
	}
	cadence, err := json.Marshal(contract.Cadence)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&out, "  \"always\": %s,\n", always)
	fmt.Fprintf(&out, "  \"unknown\": %s,\n", unknown)
	fmt.Fprintf(&out, "  \"cadence\": %s\n", cadence)
	out.WriteString("}\n")
	return out.Bytes(), nil
}

func renderProjectRisk(risk testpolicy.ProjectRisk) ([]byte, error) {
	reversibility, err := json.Marshal(risk.Reversibility)
	if err != nil {
		return nil, err
	}
	detection, err := json.Marshal(risk.Detection)
	if err != nil {
		return nil, err
	}
	recovery, err := json.Marshal(risk.Recovery)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`{"severity": %d, "exposure": %d, "reversibility": %s, "detection": %s, "recovery": %s}`,
		risk.Severity, risk.Exposure, reversibility, detection, recovery)), nil
}

func renderObjectArray[T any](out *bytes.Buffer, name string, values []T) error {
	fmt.Fprintf(out, "  %q: [\n", name)
	for i, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		out.WriteString("    ")
		out.Write(data)
		if i+1 < len(values) {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString("  ],\n")
	return nil
}
