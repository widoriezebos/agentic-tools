package dispatch

import (
	"fmt"
	"strings"
)

// HandshakeEval is the pending->running decision: it compares the effective
// permissions an adapter reports against the requested floor and produces the
// target status plus the record patch that carries the verdict. A grant wider
// than requested, or a missing session where the runtime promised one, fails
// the handshake; otherwise the job starts running.
//
// The result written to output is {"target": "running"|"failed", "patch": {…}}
// and the caller applies the patch through the record CAS.
func HandshakeEval(recordPath, effectivePath, session, turn, model string, signal bool, output string) error {
	record, err := readObject(recordPath)
	if err != nil {
		return fmt.Errorf("cannot read the job record: %v", err)
	}
	effective, err := readObject(effectivePath)
	if err != nil {
		return fmt.Errorf("cannot read the effective permissions: %v", err)
	}
	permissions, ok := record["permissions"].(map[string]any)
	if !ok {
		return fmt.Errorf("job record has no permissions object")
	}
	requested, ok := permissions["requested"].(map[string]any)
	if !ok {
		return fmt.Errorf("job record has no permissions.requested object")
	}
	snapshot, present := permissions["enforcementSnapshot"]
	if !present {
		return fmt.Errorf("job record has no permissions.enforcementSnapshot")
	}

	// Ordinal fields compare on their scale: an effective choice ranking above
	// the requested one is wider. A value off the scale must match the request
	// exactly. Fields the adapter did not report are not judged.
	orders := []struct {
		field string
		rank  map[string]int
	}{
		{"network", map[string]int{"deny": 0, "ask": 1, "allow": 2}},
		{"approvals", map[string]int{"deny": 0, "ask": 1, "allow": 2}},
		{"tools", map[string]int{"read-only": 0, "runtime-default": 1}},
	}
	var mismatched []string
	for _, ordinal := range orders {
		effectiveValue, present := effective[ordinal.field]
		if !present {
			continue
		}
		requestedValue, present := requested[ordinal.field]
		if !present {
			return fmt.Errorf("requested permissions have no %s field", ordinal.field)
		}
		requestedRank, requestedOK := ordinalRank(ordinal.rank, requestedValue)
		effectiveRank, effectiveOK := ordinalRank(ordinal.rank, effectiveValue)
		if requestedOK && effectiveOK {
			if effectiveRank > requestedRank {
				mismatched = append(mismatched, ordinal.field)
			}
		} else if !looseEqual(effectiveValue, requestedValue) {
			mismatched = append(mismatched, ordinal.field)
		}
	}
	for _, field := range []string{"readRoots", "writeRoots"} {
		effectiveValue, present := effective[field]
		if !present {
			continue
		}
		effectiveRoots, ok := stringMembers(effectiveValue)
		if !ok {
			return fmt.Errorf("effective permissions field %s is not an array", field)
		}
		requestedValue, present := requested[field]
		if !present {
			return fmt.Errorf("requested permissions have no %s field", field)
		}
		requestedRoots, ok := stringMembers(requestedValue)
		if !ok {
			return fmt.Errorf("requested permissions field %s is not an array", field)
		}
		allowed := map[string]bool{}
		for _, root := range requestedRoots {
			allowed[root] = true
		}
		for _, root := range effectiveRoots {
			if !allowed[root] {
				mismatched = append(mismatched, field)
				break
			}
		}
	}
	if signal && session == "" {
		mismatched = append(mismatched, "sessionId")
	}

	patch := map[string]any{
		"permissions": map[string]any{
			"requested":           requested,
			"effective":           effective,
			"enforcementSnapshot": snapshot,
		},
		"effectiveModel": model,
	}
	// The record's turnId is dispatch provenance, stamped at build and
	// immutable (host-implementer wall): the handshake must not name it.
	// The turn identifier the runtime's envelope reports is a session
	// fact, recorded beside the session id under its own key.
	if turn != "" {
		patch["envelopeTurn"] = turn
	}
	if session != "" {
		patch["sessionId"] = session
	}
	target := "running"
	if len(mismatched) > 0 {
		target = "failed"
		reason := "permissions_mismatch:" + strings.Join(mismatched, ",")
		if len(mismatched) == 1 && mismatched[0] == "sessionId" {
			reason = "handshake_missing_session_id"
		}
		patch["error"] = reason
		patch["phase"] = "handshake"
	} else {
		patch["error"] = nil
		patch["phase"] = "running"
	}
	return writeCompactJSON(output, map[string]any{"target": target, "patch": patch})
}

// ordinalRank places a value on an ordinal scale, reporting whether it is on
// the scale at all.
func ordinalRank(scale map[string]int, value any) (int, bool) {
	name, ok := value.(string)
	if !ok {
		return 0, false
	}
	rank, ok := scale[name]
	return rank, ok
}

// stringMembers reads a JSON array of strings, refusing non-arrays and
// non-string members.
func stringMembers(value any) ([]string, bool) {
	list, ok := value.([]any)
	if !ok {
		return nil, false
	}
	members := make([]string, 0, len(list))
	for _, item := range list {
		name, ok := item.(string)
		if !ok {
			return nil, false
		}
		members = append(members, name)
	}
	return members, true
}
