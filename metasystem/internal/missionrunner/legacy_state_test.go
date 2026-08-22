package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// HIW-O11: a preserved pre-wall mission state refuses PUBLIC RESUME with
// the exact named error — before shape validation, with no migration
// path and no mutation. The bodies here are deliberately shape-invalid
// beyond their version marker: the named refusal must outrank every
// shape diagnostic, because the remedy is re-provisioning, not repair.
func TestPreWallStateRefusesResumeByName(t *testing.T) {
	cases := []struct {
		name  string
		state map[string]any
		want  string
	}{
		{"version-1", map[string]any{
			"schemaVersion": 1, "missionId": "demo",
			"someForgottenField": "pre-wall shapes are not diagnosed",
		}, "mission resume refused: state predates the host-implementer wall; re-provision the mission"},
		{"version-2", map[string]any{
			"schemaVersion": 2, "missionId": "demo",
		}, "mission resume refused: state predates the wall's snapshot scope; re-provision the mission"},
		{"version-3", map[string]any{
			"schemaVersion": 3, "missionId": "demo",
		}, "mission resume refused: state predates the wall's snapshot scope; re-provision the mission"},
		{"missing-baseline", map[string]any{
			"schemaVersion": 4, "missionId": "demo",
		}, "mission state predates the wall's baseline admission; re-provision the mission"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			engine := NewEngine(root, "demo")
			dir := engine.missionDir()
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			statePath := filepath.Join(dir, "state.json")
			writeJSONFile(t, statePath, tc.state)
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}

			rerr := engine.launch("resume", false)
			if rerr == nil || !strings.Contains(rerr.Error(), tc.want) {
				t.Fatalf("resume must refuse with the named error, got: %v", rerr)
			}

			// No migration path and no mutation: the preserved bytes are
			// byte-identical after the refusal, and no corrupt-state or
			// recovery artifact appeared beside them.
			after, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("the refused resume mutated the preserved state")
			}
			if corrupt, _ := filepath.Glob(filepath.Join(dir, "state.corrupt.*")); len(corrupt) != 0 {
				t.Fatalf("the named refusal wrote corrupt-state files: %v", corrupt)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != "state.json" {
				names := []string{}
				for _, entry := range entries {
					names = append(names, entry.Name())
				}
				t.Fatalf("the refused resume left artifacts beside the preserved state: %v", names)
			}
		})
	}
}
