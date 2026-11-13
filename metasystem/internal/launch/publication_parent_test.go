package launch

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPublicationRerunUsesTheActualParent(t *testing.T) {
	t.Parallel()
	f, run, _, need, _ := publicationFixture(t, "base")
	record, err := f.runner.Status(run)
	if err != nil {
		t.Fatal(err)
	}
	round := record.Rounds[0].Directory
	planPath := filepath.Join(round, "plan.json")
	plan, err := ReadUnitPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	plan.Check.Base = "frozen-parent"
	if err := writeUnitJSON(planPath, plan, f.runner.Root); err != nil {
		t.Fatal(err)
	}
	evidencePath := filepath.Join(round, "check-source", "evidence.json")
	beforePlan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeEvidence, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	launches := 0
	f.starter.onStart = func(record Record) error {
		command, err := (PlainExec{}).Command(record, f.manager.Store.Root)
		if err != nil {
			return err
		}
		if !slices.Contains(command.Environment, "METASYSTEM_PUBLICATION_BASE="+need.Base) {
			t.Fatalf("publication command lost parent: %+v", command)
		}
		launches++
		return nil
	}
	provePublication(t, f.runner, run, f.worktree, need.Base)
	afterPlan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	afterEvidence, err := os.ReadFile(evidencePath)
	if err != nil || !bytes.Equal(beforePlan, afterPlan) || !bytes.Equal(beforeEvidence, afterEvidence) || launches != 1 {
		t.Fatalf("publication rewrote frozen evidence or skipped rerun: launches=%d err=%v", launches, err)
	}
}
