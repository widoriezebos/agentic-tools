// Package covenant reads an app's covenant: the versioned, app-owned
// declaration that binds intent to proofs — each requirement to an
// executable check, green to one battery with its threshold, budgets to
// bounded metrics, standing guards to per-cycle floors, and the
// guardrail net to declared paths. The covenant is the interface
// between the generic builder and the specific app: its content lives
// with the app (never overwritten by a metasystem update; at worst
// migrated), and the mission contract's own gate and guardrail
// declarations must agree with it at preflight.
package covenant

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// Covenant is the parsed, validated covenant document.
type Covenant struct {
	Identity     Identity
	Requirements []Requirement
	Battery      Battery
	Budgets      []Budget
	Guards       []Guard
	// Guardrails is the app's declared net in the shared guardrail
	// grammar — the app-side home the contract's wall.guardrails
	// declaration derives from.
	Guardrails    []string
	GuardrailSet  *mission.GuardrailClass
	SchemaVersion int64
}

// Identity names the app the covenant governs.
type Identity struct {
	Name        string
	EntryPoint  string
	SourcePaths []string
}

// Requirement binds one statement of intent to its executable proof.
// The adequacy discipline reads these rows: a requirement whose proof
// is missing or never runs is intent that floats, not intent that is
// guaranteed.
type Requirement struct {
	ID    string
	Ref   string
	Proof string
}

// Battery is the one command that earns green, with its threshold.
type Battery struct {
	Command   string
	Metric    string
	Direction string
	Threshold string
}

// Budget is a bounded metric: the ratchet's subject.
type Budget struct {
	Metric    string
	Bound     float64
	Direction string
}

// Guard is a standing floor checked every cadence cycles, deliberately
// orthogonal to the battery: a build can stay green while a guard
// catches what the battery cannot see.
type Guard struct {
	Name    string
	Command string
	Cadence int64
	Floor   float64
}

// Filename is the covenant's one location at an adopted app's root.
const Filename = "covenant.json"

func fail(format string, args ...any) error {
	return fmt.Errorf("covenant refused: "+format, args...)
}

func exactKeys(doc map[string]any, label string, keys ...string) error {
	if len(doc) != len(keys) {
		return fail("%s must contain exactly %v", label, keys)
	}
	for _, key := range keys {
		if _, ok := doc[key]; !ok {
			return fail("%s must contain exactly %v", label, keys)
		}
	}
	return nil
}

func requiredString(doc map[string]any, label, key string) (string, error) {
	value, _ := doc[key].(string)
	if value == "" {
		return "", fail("%s.%s must be a non-empty string", label, key)
	}
	return value, nil
}

func stringList(doc map[string]any, label, key string) ([]string, error) {
	raw, ok := doc[key].([]any)
	if !ok {
		return nil, fail("%s.%s must be a list of strings", label, key)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		value, ok := item.(string)
		if !ok || value == "" {
			return nil, fail("%s.%s must be a list of non-empty strings", label, key)
		}
		out = append(out, value)
	}
	return out, nil
}

func direction(doc map[string]any, label string) (string, error) {
	value, _ := doc["direction"].(string)
	if value != "max" && value != "min" {
		return "", fail("%s.direction must be max or min", label)
	}
	return value, nil
}

// Load reads and validates a covenant document. Every refusal names the
// section and the rule, because the covenant is written by humans at
// inception and retrofit — a bare "invalid" teaches nothing.
func Load(path string) (*Covenant, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fail("cannot read %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fail("%s is not valid JSON: %v", path, err)
	}
	if err := exactKeys(doc, "the covenant",
		"schemaVersion", "identity", "requirements", "battery", "budgets", "guards", "guardrails"); err != nil {
		return nil, err
	}
	version, ok := doc["schemaVersion"].(float64)
	if !ok || version != 1 {
		return nil, fail("schemaVersion must be 1")
	}
	c := &Covenant{SchemaVersion: int64(version)}

	identity, ok := doc["identity"].(map[string]any)
	if ok {
		err = exactKeys(identity, "identity", "name", "entryPoint", "sourcePaths")
	} else {
		err = fail("identity must be an object")
	}
	if err != nil {
		return nil, err
	}
	if c.Identity.Name, err = requiredString(identity, "identity", "name"); err != nil {
		return nil, err
	}
	if c.Identity.EntryPoint, err = requiredString(identity, "identity", "entryPoint"); err != nil {
		return nil, err
	}
	if c.Identity.SourcePaths, err = stringList(identity, "identity", "sourcePaths"); err != nil {
		return nil, err
	}

	rows, ok := doc["requirements"].([]any)
	if !ok || len(rows) == 0 {
		return nil, fail("requirements must be a non-empty list: a covenant with no requirements guarantees nothing")
	}
	seen := map[string]bool{}
	for index, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fail("requirements[%d] must be an object", index)
		}
		if err := exactKeys(row, fmt.Sprintf("requirements[%d]", index), "id", "ref", "proof"); err != nil {
			return nil, err
		}
		var r Requirement
		if r.ID, err = requiredString(row, fmt.Sprintf("requirements[%d]", index), "id"); err != nil {
			return nil, err
		}
		if seen[r.ID] {
			return nil, fail("requirement id %q appears twice; every row is one distinct promise", r.ID)
		}
		seen[r.ID] = true
		if r.Ref, err = requiredString(row, fmt.Sprintf("requirements[%d]", index), "ref"); err != nil {
			return nil, err
		}
		if r.Proof, err = requiredString(row, fmt.Sprintf("requirements[%d]", index), "proof"); err != nil {
			return nil, err
		}
		c.Requirements = append(c.Requirements, r)
	}

	battery, ok := doc["battery"].(map[string]any)
	if ok {
		err = exactKeys(battery, "battery", "command", "metric", "direction", "threshold")
	} else {
		err = fail("battery must be an object")
	}
	if err != nil {
		return nil, err
	}
	if c.Battery.Command, err = requiredString(battery, "battery", "command"); err != nil {
		return nil, err
	}
	if c.Battery.Metric, err = requiredString(battery, "battery", "metric"); err != nil {
		return nil, err
	}
	if c.Battery.Direction, err = direction(battery, "battery"); err != nil {
		return nil, err
	}
	if c.Battery.Threshold, err = requiredString(battery, "battery", "threshold"); err != nil {
		return nil, err
	}

	budgets, ok := doc["budgets"].([]any)
	if !ok {
		return nil, fail("budgets must be a list (empty is lawful: budgets grow by the ratchet, never by pretense)")
	}
	for index, raw := range budgets {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fail("budgets[%d] must be an object", index)
		}
		if err := exactKeys(row, fmt.Sprintf("budgets[%d]", index), "metric", "bound", "direction"); err != nil {
			return nil, err
		}
		var b Budget
		if b.Metric, err = requiredString(row, fmt.Sprintf("budgets[%d]", index), "metric"); err != nil {
			return nil, err
		}
		bound, ok := row["bound"].(float64)
		if !ok {
			return nil, fail("budgets[%d].bound must be a number", index)
		}
		b.Bound = bound
		if b.Direction, err = direction(row, fmt.Sprintf("budgets[%d]", index)); err != nil {
			return nil, err
		}
		c.Budgets = append(c.Budgets, b)
	}

	guards, ok := doc["guards"].([]any)
	if !ok {
		return nil, fail("guards must be a list (empty is lawful)")
	}
	for index, raw := range guards {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fail("guards[%d] must be an object", index)
		}
		if err := exactKeys(row, fmt.Sprintf("guards[%d]", index), "name", "command", "cadence", "floor"); err != nil {
			return nil, err
		}
		var g Guard
		if g.Name, err = requiredString(row, fmt.Sprintf("guards[%d]", index), "name"); err != nil {
			return nil, err
		}
		if g.Command, err = requiredString(row, fmt.Sprintf("guards[%d]", index), "command"); err != nil {
			return nil, err
		}
		cadence, ok := row["cadence"].(float64)
		if !ok || cadence < 1 || cadence != float64(int64(cadence)) {
			return nil, fail("guards[%d].cadence must be a positive whole number of cycles", index)
		}
		g.Cadence = int64(cadence)
		floor, ok := row["floor"].(float64)
		if !ok {
			return nil, fail("guards[%d].floor must be a number", index)
		}
		g.Floor = floor
		c.Guards = append(c.Guards, g)
	}

	if c.Guardrails, err = stringList(doc, "the covenant", "guardrails"); err != nil {
		return nil, err
	}
	joined := ""
	for index, path := range c.Guardrails {
		if index > 0 {
			joined += ","
		}
		joined += path
	}
	class, violation := mission.ParseGuardrails(joined, nil)
	if violation != "" {
		return nil, fail("guardrails: %s", violation)
	}
	c.GuardrailSet = class
	return c, nil
}
