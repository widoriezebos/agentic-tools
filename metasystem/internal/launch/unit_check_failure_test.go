package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestUnitCheckRetainsCommandFailures(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, command, outcome string
		exit                   int
	}{
		{"exit", "printf diagnostic >&2; exit 7", "failed", 7},
		{"signal", "kill -TERM $$", "cancelled", -1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			check := UnitCheck{Cheap: row.command, Audits: "printf audited", Minutes: 15, Directory: t.TempDir(), Environment: []string{}, Declaration: &UnitDeclaration{SourceSHA256: "frozen-source", FullArgv: []string{"metasystem", "test", "run"}}}
			context := CheckContext{CheckExecution: CheckExecution{Goal: "fixture", Run: "run", Round: "round", Role: "builder"}, Parent: "build-launch", Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }, After: func(time.Duration) <-chan time.Time { return nil }}
			directory, exits, err := check.Run(check.Directory, t.TempDir(), context)
			if err == nil || len(exits) != 2 || exits[0].Exit != row.exit || exits[1].Exit != 0 || exits[1].Output != "audited" {
				t.Fatalf("failed command did not preserve its exit and run audits: exits=%+v error=%v", exits, err)
			}
			data, err := os.ReadFile(filepath.Join(directory, "observation.json"))
			var observation CheckExecution
			if err != nil || json.Unmarshal(data, &observation) != nil || len(observation.Steps) != 2 {
				t.Fatalf("observation=%s error=%v", data, err)
			}
			if observation.SourceSHA256 != "frozen-source" || observation.Steps[0].Parent != "build-launch" || observation.Steps[0].Outcome != row.outcome || observation.Steps[1].Outcome != "passed" || !reflect.DeepEqual(observation.Steps[0].FullArgv, check.Declaration.FullArgv) {
				t.Fatalf("failure changed the frozen identity or outcome: %+v", observation)
			}
			if row.name == "exit" && exits[0].Output != "diagnostic" {
				t.Fatalf("stderr was not retained: %+v", exits[0])
			}
		})
	}
}

func TestCheckEvidenceRejectsUnavailableObservation(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, data string }{
		{"mismatched_identity", `{"ExecutionID":"another-check","Steps":[{"Outcome":"deadline"}]}`},
		{"empty_steps", `{"ExecutionID":"check-current","Steps":[]}`},
		{"invalid_json", `{`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			directory := filepath.Join(root, "check-current")
			if err := os.Mkdir(directory, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "observation.json"), []byte(row.data), 0600); err != nil {
				t.Fatal(err)
			}
			records, unknown := ReadCheckExecutions(root)
			step := UnitStep{LaunchID: "launch", Cause: "unclassified", Reason: "build-failed"}
			before := step
			if len(records) != 0 || len(unknown) != 1 || checkCause(&step, root) || !reflect.DeepEqual(step, before) {
				t.Fatalf("unavailable evidence changed the failure: records=%+v unknown=%v step=%+v", records, unknown, step)
			}
		})
	}
}
