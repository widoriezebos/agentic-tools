package capability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Code critique finding 9: the SHIPPED role files' waiver decisions are
// golden-pinned against the live selector — a role-policy typo cannot
// silently change a live decision. Every shipped role waives devin's
// two declared residuals; devin's undeclared network fails closed; an
// identifier under the wrong field never matches; a malformed waiver
// entry is ignored, not honored.
func TestShippedRoleWaiverMatrixGolden(t *testing.T) {
	roleDir := "../../scripts/agents/roles"
	entries, err := filepath.Glob(filepath.Join(roleDir, "*.requirements.json"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("no shipped role files found: %v", err)
	}

	devinIdentity := `{"runtime":"devin","cliVersion":"1.0.0","configHash":"abc123","configKeyHashes":{"model":"` + hash64 + `"}}`
	stage := func(t *testing.T, roleFile string, unverified []any, envelope map[string]any) error {
		e := newEnv(t)
		snap := baseSnapshot("abc123", "2026-08-10T00:00:00Z")
		snap["runtime"] = "devin"
		snap["cliVersion"] = "1.0.0"
		snap["permissions"] = map[string]any{"unverified": unverified}
		e.writeSnapshot(t, "devin-1.0.0-abc123-20260810-001.json", snap)
		e.writeEnvelope(t, envelope)
		source, err := os.ReadFile(roleFile)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(e.root, "scripts", "agents", "roles", "golden-role.requirements.json")
		os.MkdirAll(filepath.Dir(target), 0o755)
		os.WriteFile(target, source, 0o644)
		return Select(e.root, "devin", "golden-role", devinIdentity, 30, e.envelopePath, e.outputPath)
	}

	restrictiveRoots := map[string]any{"readRoots": []any{"src"}, "writeRoots": []any{"src"}}
	for _, roleFile := range entries {
		role := strings.TrimSuffix(filepath.Base(roleFile), ".requirements.json")
		t.Run(role+"/roots-allowed", func(t *testing.T) {
			if err := stage(t, roleFile, []any{"readRoots", "writeRoots"}, restrictiveRoots); err != nil {
				t.Fatalf("shipped %s waivers no longer allow devin restrictive roots: %v", role, err)
			}
		})
		t.Run(role+"/network-fails-closed", func(t *testing.T) {
			err := stage(t, roleFile, []any{"network"}, map[string]any{"network": "deny"})
			if err == nil || !strings.Contains(err.Error(), "declares no residual") {
				t.Fatalf("shipped %s allowed devin's undeclared network residual: %v", role, err)
			}
		})
	}

	// Negatives against a synthetic role file.
	writeRole := func(t *testing.T, e env, body string) {
		target := filepath.Join(e.root, "scripts", "agents", "roles", "golden-role.requirements.json")
		os.MkdirAll(filepath.Dir(target), 0o755)
		os.WriteFile(target, []byte(body), 0o644)
	}
	negative := func(t *testing.T, body, wantErr string) {
		e := newEnv(t)
		snap := baseSnapshot("abc123", "2026-08-10T00:00:00Z")
		snap["runtime"] = "devin"
		snap["cliVersion"] = "1.0.0"
		snap["permissions"] = map[string]any{"unverified": []any{"writeRoots"}}
		e.writeSnapshot(t, "devin-1.0.0-abc123-20260810-001.json", snap)
		e.writeEnvelope(t, map[string]any{"writeRoots": []any{"src"}})
		writeRole(t, e, body)
		err := Select(e.root, "devin", "golden-role", devinIdentity, 30, e.envelopePath, e.outputPath)
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("want refusal %q, got %v", wantErr, err)
		}
	}
	t.Run("wrong-field-identifier", func(t *testing.T) {
		negative(t, `{"required":[],"optional":{},"waivers":{"readRoots":["devin-write-roots-unenforced"]}}`,
			"devin-write-roots-unenforced")
	})
	t.Run("wrong-runtime-identifier", func(t *testing.T) {
		negative(t, `{"required":[],"optional":{},"waivers":{"writeRoots":["fake-network-unverified"]}}`,
			"devin-write-roots-unenforced")
	})
	t.Run("runtime-name-not-identifier", func(t *testing.T) {
		negative(t, `{"required":[],"optional":{},"waivers":{"writeRoots":["devin"]}}`,
			"devin-write-roots-unenforced")
	})
	t.Run("malformed-entry-ignored", func(t *testing.T) {
		negative(t, `{"required":[],"optional":{},"waivers":{"writeRoots":[42]}}`,
			"devin-write-roots-unenforced")
	})
}
