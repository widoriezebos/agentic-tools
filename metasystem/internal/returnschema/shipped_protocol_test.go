package returnschema

// The machine-readable half of the agent-protocol-fixtures section of the
// retired validate-metasystem.sh (verbs-object-action U7b): the shipped role
// schemas, permission presets and capability declarations keep the protocol's
// exact shapes, and the return checker accepts the canonical return of every
// role while naming each single drift. The checks read the shipped
// installation at ../.. and write only to temp dirs.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var protocolRoles = []string{"design-critic", "implementer", "code-critic", "verifier", "investigator", "behavior-judge"}

var protocolCommonFields = []string{"jobId", "round", "runtime", "sessionId", "model", "evidence", "gaps", "mode"}

var protocolOwnedFields = map[string][]string{
	"design-critic":  {"reviewedCommit", "findings", "verdictMaterialCount"},
	"implementer":    {"riskiestPart", "diffBoundary", "whatWasDone"},
	"code-critic":    {"reviewedTree", "findings", "verdictMaterialCount"},
	"verifier":       {"riskiestPart", "whatWasDone"},
	"investigator":   {"frozenFrame", "theories", "classifications", "stopLoss"},
	"behavior-judge": {"dimensions", "reliabilityWatch"},
}

func shippedAgents(t *testing.T, parts ...string) string {
	t.Helper()
	return filepath.Join(append([]string{"..", "..", "scripts", "agents"}, parts...)...)
}

func readShippedJSON(t *testing.T, parts ...string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(shippedAgents(t, parts...))
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("%s is not a JSON object: %v", filepath.Join(parts...), err)
	}
	return value
}

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

// closedFieldSet returns the property names of a closed object schema, or a
// reason it is not closed: properties, required and the roster must be the
// same set, and additionalProperties exactly false. An explicit null member
// is malformed, not empty.
func closedFieldSet(node map[string]any) ([]string, string) {
	properties := map[string]any{}
	if raw, ok := node["properties"]; ok {
		object, isObject := raw.(map[string]any)
		if !isObject {
			return nil, "properties is not an object"
		}
		properties = object
	}
	var required []string
	if raw, ok := node["required"]; ok {
		list, isList := raw.([]any)
		if !isList {
			return nil, "required is not an array"
		}
		for _, item := range list {
			name, isString := item.(string)
			if !isString {
				return nil, "required names a non-string"
			}
			required = append(required, name)
		}
	}
	var keys []string
	for key := range properties {
		keys = append(keys, key)
	}
	keys, required = sortedUnique(keys), sortedUnique(required)
	if additional, ok := node["additionalProperties"]; !ok || additional != false {
		return keys, "additionalProperties is not false"
	}
	if !reflect.DeepEqual(keys, required) {
		return keys, "required does not name every property"
	}
	return keys, ""
}

func TestShippedRoleSchemasHoldTheProtocolFieldSets(t *testing.T) {
	t.Parallel()
	for _, role := range protocolRoles {
		schema := readShippedJSON(t, "schemas", role+".schema.json")
		want := sortedUnique(append(append([]string(nil), protocolCommonFields...), protocolOwnedFields[role]...))
		got, reason := closedFieldSet(schema)
		if reason != "" || !reflect.DeepEqual(got, want) {
			t.Errorf("%s schema property set drifted from the protocol: %v (%s), want %v", role, got, reason, want)
		}
	}
	orchestrator := readShippedJSON(t, "schemas", "orchestrator.schema.json")
	want := sortedUnique([]string{"turnId", "missionId", "cycle", "dispatched", "certified", "streamUpdatesRequested", "askCandidates", "factsForLedger", "gaps", "identity"})
	if got, reason := closedFieldSet(orchestrator); reason != "" || !reflect.DeepEqual(got, want) {
		t.Errorf("orchestrator schema property set drifted from the host-turn protocol: %v (%s), want %v", got, reason, want)
	}
}

// Every object node of the orchestrator schema, at every nesting level and
// through array item schemas, is fully enumerated.
func TestShippedOrchestratorSchemaIsClosedAtEveryLevel(t *testing.T) {
	t.Parallel()
	var walk func(node map[string]any, path string)
	walk = func(node map[string]any, path string) {
		switch node["type"] {
		case "object":
			if _, reason := closedFieldSet(node); reason != "" {
				t.Errorf("orchestrator schema object is not fully enumerated: %s (%s)", path, reason)
				return
			}
			properties, _ := node["properties"].(map[string]any)
			for name, child := range properties {
				childNode, ok := child.(map[string]any)
				if !ok {
					t.Errorf("orchestrator schema property is not a schema: %s.%s", path, name)
					continue
				}
				walk(childNode, path+"."+name)
			}
		case "array":
			items, ok := node["items"].(map[string]any)
			if !ok {
				t.Errorf("orchestrator schema array has no item schema: %s", path)
				return
			}
			walk(items, path+"[]")
		}
	}
	walk(readShippedJSON(t, "schemas", "orchestrator.schema.json"), "$")
}

// Network is granted by default: the container or VM is the isolation
// boundary; a repository narrows it with dispatch.permissions.network=deny.
func TestShippedPermissionPresetsEqualTheirEnvelopes(t *testing.T) {
	t.Parallel()
	for preset, want := range map[string]string{
		"none":      `{"readRoots": ["."], "writeRoots": [], "network": "allow", "approvals": "deny", "tools": "read-only"}`,
		"workspace": `{"readRoots": ["."], "writeRoots": ["<worktree>"], "network": "allow", "approvals": "deny", "tools": "runtime-default"}`,
	} {
		var expected map[string]any
		if err := json.Unmarshal([]byte(want), &expected); err != nil {
			t.Fatal(err)
		}
		if got := readShippedJSON(t, "permissions", preset+".json"); !reflect.DeepEqual(got, expected) {
			t.Errorf("%s permission preset drifted from its envelope: %v", preset, got)
		}
	}
}

// A capability declaration names only required, optional and waivers; it
// never repeats adapter-guaranteed baseline capabilities; only the
// implementer declares a variable capability (resume, with its embed
// fallback); and waivers map a field to runtime-name strings.
func TestShippedRoleCapabilityDeclarationsKeepTheirShape(t *testing.T) {
	t.Parallel()
	for _, role := range protocolRoles {
		requirement := readShippedJSON(t, "roles", role+".requirements.json")
		_, hasRequired := requirement["required"]
		_, hasOptional := requirement["optional"]
		shapeOK := hasRequired && hasOptional
		for key := range requirement {
			if key != "required" && key != "optional" && key != "waivers" {
				shapeOK = false
			}
		}
		if !shapeOK {
			t.Errorf("%s capability declaration has unknown top-level fields: %v", role, requirement)
			continue
		}
		if raw, ok := requirement["waivers"]; ok {
			waivers, isObject := raw.(map[string]any)
			valid := isObject
			for _, runtimes := range waivers {
				list, isList := runtimes.([]any)
				if !isList {
					valid = false
					break
				}
				for _, runtime := range list {
					if _, isString := runtime.(string); !isString {
						valid = false
					}
				}
			}
			if !valid {
				t.Errorf("%s capability waivers have an invalid shape: %v", role, raw)
			}
		}
		if required, ok := requirement["required"].([]any); !ok || len(required) != 0 {
			t.Errorf("%s incorrectly repeats adapter-guaranteed baseline capabilities: %v", role, requirement["required"])
		}
		optional, isObject := requirement["optional"].(map[string]any)
		if role != "implementer" {
			if !isObject || len(optional) != 0 {
				t.Errorf("%s declares a variable capability it does not need: %v", role, requirement["optional"])
			}
			continue
		}
		resume, _ := optional["resume"].(map[string]any)
		fallback, _ := resume["fallback"].(string)
		if !isObject || len(optional) != 1 || resume == nil || strings.TrimSpace(fallback) == "" {
			t.Errorf("implementer resume capability lacks its embed fallback: %v", requirement["optional"])
		}
	}
}

// The canonical positive return of each role, as a JSON object: the eight
// shared fields plus the role's own. Negatives are derived by one edit.
func canonicalReturns() map[string]map[string]any {
	zeroSHA := strings.Repeat("0", 40)
	common := func(mode string) map[string]any {
		return map[string]any{
			"jobId": "fixture-job", "round": 1, "runtime": "fake", "sessionId": "session-1",
			"model":    map[string]any{"requested": "fake-model", "effective": "fake-model"},
			"evidence": []any{map[string]any{"command": "go test ./...", "observed": "fixture output", "level": "ran"}},
			"gaps":     []any{}, "mode": mode,
		}
	}
	with := func(base map[string]any, fields map[string]any) map[string]any {
		for key, value := range fields {
			base[key] = value
		}
		return base
	}
	ledgerAnchor := []any{map[string]any{"file": "artifacts/agents/missions/fixture/ledger.md", "line": 1}}
	return map[string]map[string]any{
		"orchestrator": {
			"turnId": "turn-3", "missionId": "fixture-mission", "cycle": 3,
			"dispatched": []any{map[string]any{"jobId": "fixture-job", "role": "implementer", "stream": "stream-a"}},
			"certified": []any{map[string]any{"jobId": "prior-job", "verdict": "accepted", "evidence": "focused checks passed",
				"authorizationDigest": strings.Repeat("1", 64)}},
			"streamUpdatesRequested": []any{map[string]any{"streamId": "stream-a", "requestedState": "active", "reason": "work remains"}},
			"askCandidates": []any{map[string]any{"streamId": "stream-b", "reasonClass": "reserved-decision",
				"question": "Approve the contract change?", "supersedes": nil}},
			"factsForLedger": []any{"focused check exposed one new fact"},
			"gaps":           []any{},
			"identity":       map[string]any{"runtime": "fake", "model": "fake-model", "sessionId": nil},
		},
		"design-critic": with(common("design"), map[string]any{"reviewedCommit": zeroSHA,
			"findings":             []any{map[string]any{"id": "F-1", "severity": "high", "material": true, "claim": "contract gap", "evidence": "read design"}},
			"verdictMaterialCount": 1}),
		"code-critic": with(common("implement"), map[string]any{"reviewedTree": zeroSHA, "findings": []any{}, "verdictMaterialCount": 0}),
		"implementer": with(common("implement"), map[string]any{"riskiestPart": "schema boundary",
			"diffBoundary": []any{"metasystem/internal/example.go"}, "whatWasDone": "implemented the brief"}),
		"verifier": with(common("verify"), map[string]any{"riskiestPart": "failure path", "whatWasDone": "drove the runnable surface"}),
		"investigator": with(common("take-a-step-back"), map[string]any{"frozenFrame": "symptom and boundary frozen",
			"theories":        []any{map[string]any{"statement": "owner lost state", "evidenceFor": "trace", "evidenceAgainst": "focused check"}},
			"classifications": []any{"falsified-continue"},
			"stopLoss":        map[string]any{"triggered": false, "trigger": nil}}),
		"behavior-judge": with(common("verify"), map[string]any{
			"dimensions": behaviorDimensions(ledgerAnchor, []any{map[string]any{"id": "BQ-1", "claim": "fixture finding", "evidence": "fixture evidence",
				"anchors": []any{map[string]any{"file": "artifacts/agents/fixture/rounds/1/prompt.md", "line": 10}}}}),
			"reliabilityWatch": []any{map[string]any{"dimension": "proportionality", "mechanicalMetric": "fence-economy", "agreement": "agrees",
				"explanation": "fixture agreement", "anchors": []any{map[string]any{"file": "artifacts/agents/missions/fixture/state.json", "line": 5}}}},
		}),
	}
}

// The eight judged dimensions; only the first carries findings and only its
// anchors ever vary.
func behaviorDimensions(firstAnchors, firstFindings []any) []any {
	ledgerAnchor := []any{map[string]any{"file": "artifacts/agents/missions/fixture/ledger.md", "line": 1}}
	dimensions := []any{map[string]any{"id": "brief-quality", "score": 4, "rationale": "fixture judgment", "anchors": firstAnchors, "findings": firstFindings}}
	for _, id := range []string{"adjudication-quality", "delegation-discipline", "gap-handling", "spec-fidelity", "repeated-work", "proportionality", "evidence-honesty"} {
		dimensions = append(dimensions, map[string]any{"id": id, "score": 4, "rationale": "fixture judgment", "anchors": ledgerAnchor, "findings": []any{}})
	}
	return dimensions
}

func shippedSchemaRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	schemas, err := filepath.Abs(shippedAgents(t, "schemas"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(schemas, filepath.Join(root, "scripts", "agents", "schemas")); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeReturn(t *testing.T, dir, name string, value map[string]any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func clone(value map[string]any) map[string]any {
	data, _ := json.Marshal(value)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}

func TestShippedSchemasAcceptEachRoleReturnAndNameEachDrift(t *testing.T) {
	t.Parallel()
	root := shippedSchemaRoot(t)
	dir := t.TempDir()
	returns := canonicalReturns()
	for _, role := range append([]string{"orchestrator"}, protocolRoles...) {
		if violations := ReturnCompleteRole(root, role, writeReturn(t, dir, role+"-positive", returns[role])); len(violations) != 0 {
			t.Errorf("return checker refused the canonical %s return: %v", role, violations)
		}
	}
	unanchored := []any{map[string]any{"id": "BQ-1", "claim": "fixture finding", "evidence": "fixture evidence", "anchors": []any{}}}
	anchored := returns["behavior-judge"]["dimensions"].([]any)[0].(map[string]any)["findings"].([]any)
	for _, negative := range []struct {
		name, role string
		edit       func(map[string]any)
		want       string
	}{
		{"orchestrator", "orchestrator", func(r map[string]any) { delete(r, "factsForLedger") }, "$.factsForLedger is required"},
		{"design-critic", "design-critic", func(r map[string]any) { delete(r, "findings"); delete(r, "verdictMaterialCount") }, "$.findings is required"},
		{"code-critic", "code-critic", func(r map[string]any) { r["whatWasDone"] = "critics do not own this section" }, "$.whatWasDone is not allowed"},
		{"implementer", "implementer", func(r map[string]any) { delete(r, "diffBoundary") }, "$.diffBoundary is required"},
		{"verifier", "verifier", func(r map[string]any) { r["diffBoundary"] = []any{"not verifier-owned"} }, "$.diffBoundary is not allowed"},
		{"investigator", "investigator", func(r map[string]any) { delete(r, "frozenFrame"); delete(r, "theories") }, "$.frozenFrame is required"},
		{"behavior-judge", "behavior-judge", func(r map[string]any) {
			r["dimensions"] = behaviorDimensions([]any{map[string]any{"file": "artifacts/agents/missions/fixture/ledger.md", "line": 1}}, unanchored)
		}, "$.dimensions[0].findings[0].anchors must contain at least one file-and-line anchor"},
		{"behavior-judge-empty-dimensions", "behavior-judge", func(r map[string]any) { r["dimensions"] = []any{} },
			"$.dimensions must contain at least one requested judged dimension with no duplicate ids"},
		{"behavior-judge-invalid-anchor", "behavior-judge", func(r map[string]any) {
			r["dimensions"] = behaviorDimensions([]any{map[string]any{"file": "artifacts/agents/missions/fixture/ledger.md", "line": 0}}, anchored)
		}, "$.dimensions[0].anchors[0].line must be a positive one-based line number"},
		{"critic-missing-verdict", "design-critic", func(r map[string]any) { delete(r, "verdictMaterialCount") }, "$.verdictMaterialCount is required"},
		{"critic-miscount", "design-critic", func(r map[string]any) { r["verdictMaterialCount"] = 0 },
			"$.verdictMaterialCount must equal the count of findings with material=true"},
	} {
		value := clone(returns[negative.role])
		negative.edit(value)
		violations := ReturnCompleteRole(root, negative.role, writeReturn(t, dir, negative.name+"-negative", value))
		if !strings.Contains(strings.Join(violations, "\n"), negative.want) {
			t.Errorf("return checker did not name the %s violation %q: %v", negative.name, negative.want, violations)
		}
	}
}

// Job mode derives the schema and return path from the job record and checks
// the four identity fields, one at a time, against a schema-valid return.
func TestShippedSchemasJobModeChecksReturnIdentity(t *testing.T) {
	t.Parallel()
	root := shippedSchemaRoot(t)
	implementer := canonicalReturns()["implementer"]
	write := func(rel string, value any) {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("artifacts/agents/jobs/fixture-job.json", map[string]any{"jobId": "fixture-job", "role": "implementer", "round": 1,
		"parentJob": nil, "runtime": "fake", "sessionId": "session-1"})
	write("artifacts/agents/fixture-job/rounds/1/return.json", implementer)
	if violations := ReturnCompleteJob(root, "fixture-job"); len(violations) != 0 {
		t.Fatalf("job-aware return checker refused round one: %v", violations)
	}
	write("artifacts/agents/jobs/fixture-job-r2.json", map[string]any{"jobId": "fixture-job-r2", "role": "implementer", "round": 2,
		"parentJob": "fixture-job", "runtime": "fake", "sessionId": "session-2"})
	followUp := clone(implementer)
	followUp["jobId"], followUp["round"], followUp["sessionId"] = "fixture-job-r2", 2, "session-2"
	write("artifacts/agents/fixture-job/rounds/2/return.json", followUp)
	if violations := ReturnCompleteJob(root, "fixture-job-r2"); len(violations) != 0 {
		t.Fatalf("job-aware return checker refused the round-two follow-up: %v", violations)
	}
	for field, value := range map[string]any{"jobId": "other-job", "round": 2, "runtime": "other-runtime", "sessionId": "other-session"} {
		mismatched := clone(implementer)
		mismatched[field] = value
		write("artifacts/agents/fixture-job/rounds/1/return.json", mismatched)
		violations := ReturnCompleteJob(root, "fixture-job")
		if !strings.Contains(strings.Join(violations, "\n"), "$."+field+" identity mismatch") {
			t.Errorf("job-aware return checker did not name the %s mismatch: %v", field, violations)
		}
	}
}
