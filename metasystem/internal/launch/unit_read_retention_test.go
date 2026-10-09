package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func TestCollectExaminationStoppedPatchKeepsLaunchRead(t *testing.T) {
	t.Parallel()
	manager, _, _, _ := manager(t)
	runner := &UnitRunner{Manager: manager, Root: t.TempDir(),
		ExaminationRead: func(string, string) (readsubject.Read, error) {
			return readsubject.Read{}, os.ErrNotExist
		}}
	path := filepath.Join(t.TempDir(), "return.json")
	data := structuredUnitReturn(1, "regression", "source.go")
	writeFile(t, path, data)
	read, err := readsubject.Collect("patch-reader", readsubject.ReadSubject{}, "fixture-engine", "fixture-reader", path, []byte(data), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Store.Create(Record{ID: read.ID, Kind: "read", State: Completed, Read: &read}); err != nil {
		t.Fatal(err)
	}
	budget := 0
	record := UnitRunRecord{ID: "stopped-patch", Goal: "goal", Unit: "unit", CorrectionBudget: &budget,
		Rounds: []UnitRound{{Number: 1, Directory: filepath.Join(runner.Root, "stopped-patch", "round-1"),
			Steps: []UnitStep{{Name: "read", LaunchID: read.ID, State: StepPassed}}}}}
	round := &record.Rounds[0]
	if err := runner.collectRoundRead(&record, round); err != nil {
		t.Fatal(err)
	}
	if round.Stop == nil || round.Stop.Decision != "stop" || round.Stop.Class != "correction allowance spent" || len(round.Reads) != 1 {
		t.Fatalf("the patch must start with a real stopped read: %+v", round)
	}
	wantStop := *round.Stop
	wantReads := append([]readsubject.Read(nil), round.Reads...)
	subject := UnitSubject{Round: 1, Commit: "", Examination: read.ID, ExaminationRound: 1, ExaminationReturnPath: path}
	if err := runner.CollectExamination(&record, round, subject); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(round.Reads, wantReads) || !reflect.DeepEqual(round.Stop, &wantStop) {
		t.Fatalf("stopped patch lost its launch read or real stop: reads=%+v stop=%+v; want reads=%+v stop=%+v", round.Reads, round.Stop, wantReads, wantStop)
	}
	retained, err := runner.Status(record.ID)
	if err != nil || !reflect.DeepEqual(retained.Rounds[0].Reads, wantReads) || !reflect.DeepEqual(retained.Rounds[0].Stop, &wantStop) {
		t.Fatalf("stopped patch decision was not retained: %+v %v", retained, err)
	}
}

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
