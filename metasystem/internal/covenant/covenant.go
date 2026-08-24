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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
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

// Filename is the covenant's one location at an adopted app's root —
// the mission package's constant, aliased so the guardrail class and
// this reader can never disagree about the one home.
const Filename = mission.CovenantFilename

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
	if strings.TrimSpace(value) == "" {
		return "", fail("%s.%s must be a non-empty string", label, key)
	}
	return value, nil
}

// duplicateKey walks the raw JSON tokens and reports the first object
// member name that repeats inside one object: encoding/json silently
// keeps the last duplicate, which would let a second "battery" shadow
// the one a reviewer read.
func duplicateKey(data []byte) string {
	decoder := json.NewDecoder(bytes.NewReader(data))
	type frame struct {
		object bool
		seen   map[string]bool
		isKey  bool
	}
	var stack []*frame
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				stack = append(stack, &frame{object: true, seen: map[string]bool{}, isKey: true})
			case '[':
				stack = append(stack, &frame{})
			case '}', ']':
				stack = stack[:len(stack)-1]
				if len(stack) > 0 && stack[len(stack)-1].object {
					stack[len(stack)-1].isKey = true
				}
			}
			continue
		}
		if len(stack) == 0 {
			continue
		}
		top := stack[len(stack)-1]
		if !top.object {
			continue
		}
		if top.isKey {
			if name, ok := token.(string); ok {
				if top.seen[name] {
					return name
				}
				top.seen[name] = true
			}
			top.isKey = false
		} else {
			top.isKey = true
		}
	}
}

func stringList(doc map[string]any, label, key string) ([]string, error) {
	raw, ok := doc[key].([]any)
	if !ok {
		return nil, fail("%s.%s must be a list of strings", label, key)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		value, ok := item.(string)
		if !ok || strings.TrimSpace(value) == "" {
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
// inception and retrofit — a bare "invalid" teaches nothing. The read
// is ONE open with no-follow and the shape judged on the held handle:
// a check-then-read pair would let a swap serve different bytes than
// the shape that passed.
func Load(path string) (*Covenant, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, fail("%s is a symlink, and a symlink is never a covenant — the one home holds the bytes themselves", path)
		}
		return nil, fail("cannot read %s: %v", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fail("cannot read %s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fail("%s must be a regular file, not %s", path, info.Mode().Type())
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fail("cannot read %s: %v", path, err)
	}
	return Parse(data, path)
}

// Parse validates covenant bytes wherever they came from — the one
// home on disk, or a tree the wall reads them back out of. The label
// names the source where the syntax and duplicate-member refusals
// speak; the section refusals name their rows, which need no path.
func Parse(data []byte, label string) (*Covenant, error) {
	path := label
	var err error
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fail("%s is not valid JSON: %v", path, err)
	}
	if name := duplicateKey(data); name != "" {
		return nil, fail("%s repeats the member %q; a duplicate would silently shadow what a reviewer read", path, name)
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
	if !contract.ValidThreshold(c.Battery.Threshold) {
		return nil, fail("battery.threshold %q is not in the contract's threshold grammar (a comparator then a number, no spaces); a mission contract could never carry it", c.Battery.Threshold)
	}
	if !contract.ValidMetricID(c.Battery.Metric) {
		return nil, fail("battery.metric %q is not in the contract's metric grammar (a lowercase letter or digit first, then lowercase letters, digits, or hyphens); the gate.threshold key it forms could never parse", c.Battery.Metric)
	}
	// The command must be carryable as a contract value: one line, no
	// NUL, no padding the contract's key/value parser would strip or
	// refuse — an uncarryable command would load here and refuse at
	// every preflight forever.
	switch {
	case strings.ContainsAny(c.Battery.Command, "\n\r"):
		return nil, fail("battery.command must be a single line; a mission contract value could never carry it")
	case strings.ContainsRune(c.Battery.Command, 0):
		return nil, fail("battery.command must not contain NUL; a mission contract value could never carry it")
	case strings.TrimSpace(c.Battery.Command) != c.Battery.Command:
		return nil, fail("battery.command must not carry leading or trailing whitespace; the contract's value parser rejects padded values")
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
	for index, entry := range c.Guardrails {
		if strings.Contains(entry, ",") {
			return nil, fail("guardrails[%d] %q contains a comma, which the guardrail grammar reserves as its separator", index, entry)
		}
		if index > 0 {
			joined += ","
		}
		joined += entry
	}
	// The same protected-path predicate the contract side applies: a
	// covenant declaring a wall-custodied path would pass here and
	// refuse at every preflight forever — better named at load.
	class, violation := mission.ParseGuardrails("covenant guardrails", joined, mission.ProtectedArtifactPath)
	if violation != "" {
		return nil, fail("guardrails: %s", violation)
	}
	c.GuardrailSet = class
	return c, nil
}
