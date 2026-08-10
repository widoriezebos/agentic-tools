package adapter

import (
	"fmt"
	"strings"
)

// MaterializeEffective writes the job's requested permission envelope out as
// the effective-permissions file the launch will be measured against. It is a
// file the handshake reads and the widening check compares, so it is written
// whole rather than as a field read.
func MaterializeEffective(recordPath, effectivePath string) error {
	requested, err := requestedPermissions(recordPath)
	if err != nil {
		return err
	}
	return atomicWriteJSON(effectivePath, requested)
}

// RewriteWriteScope pins the effective file's writeRoots to the resolved
// workspace root. Every baseline CLI makes its own workspace the OS-sandbox
// write boundary, so a request for a narrower subdirectory is recorded as the
// wider boundary that is actually enforced; the widening check then refuses it
// instead of the launch proceeding on a boundary that is falsely exact. An
// empty or absent writeRoots is left as it stands.
func RewriteWriteScope(effectivePath, workspace string) error {
	effective, err := readObject(effectivePath)
	if err != nil {
		return err
	}
	if effective == nil {
		return fmt.Errorf("effective permissions file %s is not a JSON object", effectivePath)
	}
	if roots, ok := effective["writeRoots"].([]any); ok && len(roots) > 0 {
		effective["writeRoots"] = []any{resolve(workspace)}
	}
	return atomicWriteJSON(effectivePath, effective)
}

// ComparePermissions reports the permission fields where the effective grant is
// wider than what the job requested, joined with commas and empty when the
// effective grant is a subset of the request. A launch that would run wider
// than it asked for is refused before it starts.
//
// Roots widen when the effective set is not a subset of the requested set.
// Ordinal fields widen when the effective choice ranks above the requested one:
// network and approvals run deny < ask < allow, and tools runs read-only <
// runtime-default. An effective value outside its ordinal scale counts as wider
// than any requested value; a requested value outside its scale is treated as
// the most restrictive, so any recognized effective value widens past it.
func ComparePermissions(recordPath, effectivePath string) (string, error) {
	requested, err := requestedPermissions(recordPath)
	if err != nil {
		return "", err
	}
	effective, err := readObject(effectivePath)
	if err != nil {
		return "", err
	}
	if effective == nil {
		return "", fmt.Errorf("effective permissions file %s is not a JSON object", effectivePath)
	}

	var wider []string
	for _, field := range []string{"readRoots", "writeRoots"} {
		if !stringSubset(stringSet(effective[field]), stringSet(requested[field])) {
			wider = append(wider, field)
		}
	}
	for _, ordinal := range ordinalFields {
		value, present := effective[ordinal.field]
		if !present {
			continue
		}
		if ordinal.rank(value, 999) > ordinal.rank(requested[ordinal.field], -1) {
			wider = append(wider, ordinal.field)
		}
	}
	return strings.Join(wider, ","), nil
}

type ordinalField struct {
	field string
	order map[string]int
}

// rank returns the ordinal position of value, or missing when value is not a
// string on the scale.
func (o ordinalField) rank(value any, missing int) int {
	if s, ok := value.(string); ok {
		if r, ok := o.order[s]; ok {
			return r
		}
	}
	return missing
}

var ordinalFields = []ordinalField{
	{"network", map[string]int{"deny": 0, "ask": 1, "allow": 2}},
	{"approvals", map[string]int{"deny": 0, "ask": 1, "allow": 2}},
	{"tools", map[string]int{"read-only": 0, "runtime-default": 1}},
}

// requestedPermissions returns the permissions.requested object from a job
// record.
func requestedPermissions(recordPath string) (map[string]any, error) {
	record, err := readObject(recordPath)
	if err != nil {
		return nil, err
	}
	permissions, _ := record["permissions"].(map[string]any)
	requested, ok := permissions["requested"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("job record %s has no permissions.requested object", recordPath)
	}
	return requested, nil
}

// stringSet collects the string members of a JSON array, ignoring non-strings.
func stringSet(value any) map[string]bool {
	set := map[string]bool{}
	if list, ok := value.([]any); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				set[s] = true
			}
		}
	}
	return set
}

func stringSubset(sub, super map[string]bool) bool {
	for member := range sub {
		if !super[member] {
			return false
		}
	}
	return true
}
