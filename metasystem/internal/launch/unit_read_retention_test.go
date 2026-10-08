package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCollectedUnitReadUsesRetainedStructuredOutput(t *testing.T) {
	t.Parallel()
	for _, variation := range []string{"shared output replaced", "shared output removed", "retained output corrupt"} {
		t.Run(variation, func(t *testing.T) {
			t.Parallel()
			state, shared := t.TempDir(), t.TempDir()
			live := filepath.Join(shared, "read-findings.md.json")
			retained := filepath.Join(state, "outputs", filepath.Base(live))
			diff := filepath.Join(state, "read.diff")
			if err := os.MkdirAll(filepath.Dir(retained), 0700); err != nil {
				t.Fatal(err)
			}
			writeFile(t, live, structuredUnitReturn(2, "scope", "later.go"))
			writeFile(t, retained, structuredUnitReturn(1, "regression", "source.go"))
			writeFile(t, diff, "the immutable examined change\n")
			record := Record{ID: "reader", Outputs: []Output{{Path: live}, {Path: retained}}, Measurement: Measurement{Verdict: "VERDICT: material=1"}}
			record.AdapterData = map[string]json.RawMessage{}
			setStrings(record.AdapterData, "declaredOutputs", []string{filepath.Join(shared, "read-findings.md"), live})
			setString(record.AdapterData, "readDiff", diff)
			setString(record.AdapterData, "engine", "fixture-engine")
			setString(record.AdapterData, "model", "fixture-reader")
			if variation == "shared output removed" {
				if err := os.Remove(live); err != nil {
					t.Fatal(err)
				}
			} else if variation == "retained output corrupt" {
				writeFile(t, retained, "not a structured read")
			}
			read, err := collectLaunchRead(record, state)
			if variation == "retained output corrupt" {
				if err == nil {
					t.Fatal("corrupt retained evidence fell back to another writer's output")
				}
				return
			}
			if err != nil || read.Material != 1 || read.Output != retained || read.Findings[0].ID != "reader:1" {
				t.Fatalf("collection used mutable evidence or rejected its declared filename: %+v %v", read, err)
			}
		})
	}
}
