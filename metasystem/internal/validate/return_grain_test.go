package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requireReturnGrain(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf(format, args...)
	}
}

func TestMechanicalRigorRequiresBehaviourAndFixture(t *testing.T) {
	result := map[string]any{
		"findings": []any{map[string]any{"id": "F-1", "material": true}},
		"rigor":    []any{map[string]any{"findingId": "F-1", "grain": "mechanical", "behaviour": "observed behavior", "reopeningTrigger": "change"}},
	}
	check := func() string {
		checker := &returnChecker{}
		checker.checkRigorRows(result, 5)
		return strings.Join(checker.violations, "\n")
	}
	requireReturnGrain(t, strings.Contains(check(), "$.rigor[0].fixture"), "missing fixture violation")
	result["rigor"].([]any)[0].(map[string]any)["fixture"] = "TestFixture"
	requireReturnGrain(t, check() == "", "complete mechanical row refused: %s", check())
	result["rigor"].([]any)[0] = map[string]any{"findingId": "F-1", "grain": "invariant", "reopeningTrigger": "change"}
	requireReturnGrain(t, check() == "", "invariant row required mechanical proof: %s", check())
}

func TestReturnVersionFiveRoleGateAndNoClosePolicy(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "scripts", "agents", "schemas")
	requireReturnGrain(t, os.MkdirAll(dir, 0o755) == nil, "create schema directory")
	schema := []byte(`{"title":"test","type":"object","additionalProperties":false,"required":["findings","materialCount","model"],"properties":{"findings":{"type":"array","items":{"type":"object"}},"materialCount":{"type":"integer"},"model":{"type":"object","additionalProperties":false,"required":[],"properties":{}}}}`)
	for _, role := range []string{"design-critic", "implementer"} {
		requireReturnGrain(t, os.WriteFile(filepath.Join(dir, role+".schema.json"), schema, 0o644) == nil, "write role schema")
	}
	write := func(name string, body []byte) string {
		path := filepath.Join(root, name)
		requireReturnGrain(t, os.WriteFile(path, body, 0o644) == nil, "write return")
		return path
	}
	valid := []byte(`{"schemaVersion":5,"claimed":{"sessionId":null,"model":null},"findings":[],"materialCount":0,"model":{},"rigor":[]}`)
	violations := ReturnCompleteRole(root, "design-critic", write("v5.json", valid))
	requireReturnGrain(t, len(violations) == 0, "version 5 critic refused: %v", violations)
	for role, version := range map[string]int{"implementer": 5, "design-critic": 6} {
		candidate := []byte(fmt.Sprintf(`{"schemaVersion":%d}`, version))
		got := strings.Join(ReturnCompleteRole(root, role, write(role+".json", candidate)), "\n")
		requireReturnGrain(t, strings.Contains(got, "unknown return schema version"), "%s version %d was not refused by name: %s", role, version, got)
	}
	data, err := os.ReadFile(filepath.Join("..", "dispatch", "finding_register.go"))
	requireReturnGrain(t, err == nil, "read finding_register.go: %v", err)
	source := string(data)
	requireReturnGrain(t, strings.Contains(source, "root[findingRegisterRoundField] = round\n\t\t\tmaterialHistory, historyErr := appendMaterialRound") && strings.Contains(source, "root[materialByRoundField] = materialHistory"), "materialByRound is not published beside the folded round")
	closeSource := strings.Split(strings.Split(source, "func CritiqueRegisterClose")[1], "func cleanClosure")[0]
	requireReturnGrain(t, !strings.Contains(closeSource, "materialByRound") && !strings.Contains(closeSource, ".Grain"), "CritiqueRegisterClose applies policy from trajectory or grain")
}
