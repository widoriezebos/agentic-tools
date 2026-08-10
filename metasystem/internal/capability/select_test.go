package capability

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const hash64 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func init() {
	// Deterministic clock so snapshot staleness is exercised, not the wall.
	now = func() time.Time { return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC) }
}

type env struct {
	root, role, runtime, version, configHash, envelopePath, outputPath string
}

func newEnv(t *testing.T) env {
	t.Helper()
	root := t.TempDir()
	e := env{
		root: root, role: "implementer", runtime: "codex", version: "0.146.0",
		configHash:   "abc123",
		envelopePath: filepath.Join(root, "envelope.json"),
		outputPath:   filepath.Join(root, "selected.json"),
	}
	mustMkdir(t, filepath.Join(root, "artifacts/agents/capabilities"))
	mustMkdir(t, filepath.Join(root, "scripts/agents/roles"))
	return e
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func (e env) writeSnapshot(t *testing.T, name string, snap map[string]any) {
	t.Helper()
	body, _ := json.Marshal(snap)
	path := filepath.Join(e.root, "artifacts/agents/capabilities", name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e env) writeRequirements(t *testing.T, req map[string]any) {
	t.Helper()
	body, _ := json.Marshal(req)
	if err := os.WriteFile(filepath.Join(e.root, "scripts/agents/roles", e.role+".requirements.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e env) writeEnvelope(t *testing.T, envelope map[string]any) {
	t.Helper()
	body, _ := json.Marshal(envelope)
	if err := os.WriteFile(e.envelopePath, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e env) identity(configHash string) string {
	return `{"runtime":"codex","cliVersion":"0.146.0","configHash":"` + configHash +
		`","configKeyHashes":{"model":"` + hash64 + `"}}`
}

func baseSnapshot(configHash, captured string) map[string]any {
	return map[string]any{
		"runtime": "codex", "cliVersion": "0.146.0", "configHash": configHash,
		"configKeyHashes": map[string]any{"model": hash64},
		"capturedAt":      captured,
		"capabilities":    map[string]any{"sessionEstablishedTimeoutSec": 2},
		"permissions":     map[string]any{"unverified": []any{}},
		"envelopeEnforcement": map[string]any{
			"writeRoots": "mapped", "readRoots": "notEnforced", "network": "mapped",
		},
	}
}

func TestSelectMatchesFreshSnapshot(t *testing.T) {
	e := newEnv(t)
	e.writeSnapshot(t, "codex-0.146.0-abc123-20260810-001.json", baseSnapshot("abc123", "2026-08-10T00:00:00Z"))
	e.writeRequirements(t, map[string]any{"required": []any{}, "optional": map[string]any{}, "waivers": map[string]any{}})
	e.writeEnvelope(t, map[string]any{"readRoots": []any{}, "writeRoots": []any{}, "network": "deny"})

	if err := Select(e.root, e.runtime, e.role, e.identity("abc123"), 30, e.envelopePath, e.outputPath); err != nil {
		t.Fatalf("a fresh matching snapshot should select: %v", err)
	}
	out, _ := os.ReadFile(e.outputPath)
	var result map[string]any
	_ = json.Unmarshal(out, &result)
	if !strings.Contains(result["path"].(string), "codex-0.146.0-abc123") {
		t.Fatalf("wrong snapshot selected: %v", result["path"])
	}
	if result["sessionEstablishedTimeoutSec"].(float64) != 2 {
		t.Fatalf("timeout not carried: %v", result)
	}
}

func TestSelectNoMatchNamesChangedKeys(t *testing.T) {
	e := newEnv(t)
	// A snapshot with a different configHash and a different model key hash.
	snap := baseSnapshot("other", "2026-08-10T00:00:00Z")
	snap["configKeyHashes"] = map[string]any{"model": strings.Repeat("b", 64)}
	e.writeSnapshot(t, "codex-0.146.0-other-20260810-001.json", snap)
	e.writeRequirements(t, map[string]any{"required": []any{}, "optional": map[string]any{}, "waivers": map[string]any{}})
	e.writeEnvelope(t, map[string]any{"network": "allow"})

	err := Select(e.root, e.runtime, e.role, e.identity("abc123"), 30, e.envelopePath, e.outputPath)
	if err == nil || !strings.Contains(err.Error(), "changed configuration keys: model") {
		t.Fatalf("no match should name the changed key, got %v", err)
	}
}

func TestSelectStaleSnapshot(t *testing.T) {
	e := newEnv(t)
	e.writeSnapshot(t, "codex-0.146.0-abc123-20260101-001.json", baseSnapshot("abc123", "2026-01-01T00:00:00Z"))
	e.writeRequirements(t, map[string]any{"required": []any{}, "optional": map[string]any{}, "waivers": map[string]any{}})
	e.writeEnvelope(t, map[string]any{"network": "allow"})
	if err := Select(e.root, e.runtime, e.role, e.identity("abc123"), 30, e.envelopePath, e.outputPath); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("an old snapshot should be stale, got %v", err)
	}
}

func TestSelectMissingRequiredCapability(t *testing.T) {
	e := newEnv(t)
	e.writeSnapshot(t, "codex-0.146.0-abc123-20260810-001.json", baseSnapshot("abc123", "2026-08-10T00:00:00Z"))
	e.writeRequirements(t, map[string]any{"required": []any{"resume"}, "optional": map[string]any{}, "waivers": map[string]any{}})
	e.writeEnvelope(t, map[string]any{"network": "allow"})
	if err := Select(e.root, e.runtime, e.role, e.identity("abc123"), 30, e.envelopePath, e.outputPath); err == nil || !strings.Contains(err.Error(), "required runtime capabilities are absent") {
		t.Fatalf("a missing required capability should refuse, got %v", err)
	}
}

func TestSelectRestrictiveFieldRefusedThenWaived(t *testing.T) {
	e := newEnv(t)
	snap := baseSnapshot("abc123", "2026-08-10T00:00:00Z")
	snap["permissions"] = map[string]any{"unverified": []any{"network"}}
	e.writeSnapshot(t, "codex-0.146.0-abc123-20260810-001.json", snap)
	e.writeEnvelope(t, map[string]any{"network": "deny"}) // deny != allow -> restrictive

	// No waiver: refused.
	e.writeRequirements(t, map[string]any{"required": []any{}, "optional": map[string]any{}, "waivers": map[string]any{}})
	if err := Select(e.root, e.runtime, e.role, e.identity("abc123"), 30, e.envelopePath, e.outputPath); err == nil ||
		!strings.Contains(err.Error(), "cannot enforce restrictive permission field network") {
		t.Fatalf("an unenforceable restriction should refuse, got %v", err)
	}
	// With a waiver for codex: allowed.
	e.writeRequirements(t, map[string]any{"required": []any{}, "optional": map[string]any{}, "waivers": map[string]any{"network": []any{"codex"}}})
	if err := Select(e.root, e.runtime, e.role, e.identity("abc123"), 30, e.envelopePath, e.outputPath); err != nil {
		t.Fatalf("a waived restriction should be allowed: %v", err)
	}
}
