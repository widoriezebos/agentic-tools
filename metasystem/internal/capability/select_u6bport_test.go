package capability

import (
	"encoding/json"
	"strings"
	"testing"
)

// Ported from dispatch-fixtures.sh (empty-write-envelope, lines 4459-4497):
// an EMPTY write scope is still restrictive on a runtime whose write boundary
// is notEnforced, so the role is refused without the runtime's declared
// writeRoots residual waived and admitted with it; an unregistered runtime
// declares no residual and fails closed even under a runtime-name waiver.
func TestSelectEmptyWriteRootsOnAnUnenforcedBoundaryIsRestrictive(t *testing.T) {
	t.Parallel()
	envelope := map[string]any{"readRoots": []any{}, "writeRoots": []any{}, "network": "allow", "approvals": "deny", "tools": "read-only"}
	stage := func(t *testing.T, runtime, waivers string) error {
		e := newEnv(t)
		e.role = "design-critic"
		snap := baseSnapshot("cfg1", "2026-08-10T00:00:00Z")
		snap["runtime"] = runtime
		snap["cliVersion"] = "1.0"
		snap["configKeyHashes"] = map[string]any{}
		snap["permissions"] = map[string]any{"unverified": []any{"readRoots", "writeRoots", "network"}}
		snap["envelopeEnforcement"] = map[string]any{"writeRoots": "notEnforced", "readRoots": "notEnforced", "network": "notEnforced"}
		e.writeSnapshot(t, runtime+"-1.0-cfg1-20260810-001.json", snap)
		e.writeEnvelope(t, envelope)
		e.writeRequirements(t, map[string]any{"required": []any{}, "optional": map[string]any{}, "waivers": decode(t, waivers)})
		identity := `{"runtime":"` + runtime + `","cliVersion":"1.0","configHash":"cfg1","configKeyHashes":{}}`
		return Select(e.root, runtime, e.role, identity, 30, e.envelopePath, e.outputPath)
	}
	if err := stage(t, "devin", `{}`); err == nil || !strings.Contains(err.Error(), "permission field writeRoots") ||
		!strings.Contains(err.Error(), "devin-write-roots-unenforced") {
		t.Fatalf("empty writeRoots on a notEnforced boundary ran without a waiver: %v", err)
	}
	if err := stage(t, "devin", `{"writeRoots":["devin-write-roots-unenforced"]}`); err != nil {
		t.Fatalf("the declared writeRoots residual waiver did not admit the role: %v", err)
	}
	if err := stage(t, "ghostrt", `{}`); err == nil || !strings.Contains(err.Error(), "writeRoots") {
		t.Fatalf("an unregistered runtime's empty write scope was not refused by field: %v", err)
	}
	if err := stage(t, "ghostrt", `{"writeRoots":["ghostrt"]}`); err == nil || !strings.Contains(err.Error(), "declares no residual") {
		t.Fatalf("an unregistered runtime's name waiver bypassed the residual rule: %v", err)
	}
}

func decode(t *testing.T, text string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
