package launch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

func endFrozenTestLaunch(t *testing.T, f unitFixture, id string, state State, material int) {
	t.Helper()
	_, err := f.manager.Store.Update(id, func(r *Record) error {
		code := 0
		if state == Failed {
			code = 1
		}
		r.State, r.ExitCode, r.FinishedAt = state, &code, f.manager.Now().UTC().Format(time.RFC3339Nano)
		r.Supervisor, r.Child = nil, nil
		if r.Kind == "read" && state == Completed {
			yes := true
			r.VerdictCounts = &yes
			r.Measurement.Verdict = fmt.Sprintf("fix first (%d material findings)", material)
			stateDir, err := f.manager.Store.StateDir(id)
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(stateDir, "return.json"), []byte(structuredUnitReturn(material, "regression", "built")), 0600)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPendingReadSurvivesTheCheckRetry(t *testing.T) {
	t.Parallel()
	f, _ := newGitUnitFixture(t)
	plan, err := ReadUnitPlan(f.plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Proof[0].Name = "unit-check"
	if err := writeUnitJSON(f.plan, plan, ""); err != nil {
		t.Fatal(err)
	}
	f.starter.onStart = func(r Record) error {
		if r.Kind == "proof" || r.Kind == "read" {
			f.starter.holdKind = r.Kind
		}
		return nil
	}
	first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil || !first.Capped {
		t.Fatalf("start: cap=%v %v", first.Capped, err)
	}
	round := first.Record.Rounds[0]
	proof, read := round.Steps[1], round.Steps[2]
	writeCheckObservation(t, round.Directory, "check-first", proof.LaunchID, "2026-10-23T09:00:00Z", "deadline", "passed")
	endFrozenTestLaunch(t, f, proof.LaunchID, Failed, 0)
	retry, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil || !retry.Capped {
		t.Fatalf("retry: capped=%v %v", retry.Capped, err)
	}
	after := retry.Record.Rounds[0]
	check := after.Steps[1]
	if check.RetryBy != "machine" || len(check.LaunchIDs) != 2 || after.Steps[2].State != StepRunning || !slices.Equal(after.Steps[2].LaunchIDs, []string{read.LaunchID}) || after.ReadCheckout != round.ReadCheckout {
		t.Fatal("the machine retry replaced or detached a pending reader")
	}
	endFrozenTestLaunch(t, f, check.LaunchID, Completed, 0)
	waiting, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil || !waiting.Capped || waiting.Step != "read" || waiting.Record.State != "running" {
		t.Fatalf("passing check with pending read: %s cap=%t %v", waiting.Step, waiting.Capped, err)
	}
	endFrozenTestLaunch(t, f, read.LaunchID, Completed, 1)
	last, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil {
		t.Fatal(err)
	}
	done := last.Record.Rounds[0]
	if last.Record.State != "awaiting-judgement" || done.Outcome != "green" || done.Material != 1 || len(done.Reads) != 1 || done.Steps[2].ExecutionEndedAt <= done.Steps[1].ExecutionEndedAt || !done.CheckoutRemoved {
		t.Fatalf("late finding: state=%s outcome=%s material=%d", last.Record.State, done.Outcome, done.Material)
	}
}

func TestRecoveredOldCheckReclaimsUnrecordedCheckout(t *testing.T) {
	t.Parallel()
	f, _ := newGitUnitFixture(t)
	plan, err := ReadUnitPlan(f.plan)
	if err != nil {
		t.Fatal(err)
	}
	before := plan
	before.Read = UnitReadPlan{}
	if err := writeUnitJSON(f.plan, before, ""); err != nil {
		t.Fatal(err)
	}
	f.starter.holdKind = "proof"
	f.starter.readVerdict = "land"
	first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil || !first.Capped {
		t.Fatalf("start old check: %v", err)
	}
	round := first.Record.Rounds[0]
	check := round.Steps[1]
	// Before pairing existed, the running check had its requested read in the plan
	// and no read step, checkout or marker in the unit record.
	for _, path := range []string{first.Record.Plan, filepath.Join(round.Directory, "plan.json")} {
		if err := writeUnitJSON(path, plan, ""); err != nil {
			t.Fatal(err)
		}
	}
	store, err := diskstore.CreateTempStore("metasystem-unit-read."+first.Record.ID+"-r1", readContextClass, roundReadOwner(first.Record.ID, 1))
	if err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(store, "interrupted-provision")
	writeFile(t, stale, "partial")
	joined, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil || !joined.Capped {
		t.Fatalf("join old check: %v", err)
	}
	now := joined.Record.Rounds[0]
	if now.ReadStore != store || len(now.Steps) != 3 || now.Steps[1].LaunchID != check.LaunchID || len(now.Steps[2].LaunchIDs) != 1 {
		t.Fatal("re-entry duplicated the old check or its reader")
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the interrupted provision was reused as a checkout")
	}
	endFrozenTestLaunch(t, f, check.LaunchID, Completed, 0)
	last, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil || last.Record.Rounds[0].Outcome != "green" || len(f.starter.ids) != 3 {
		t.Fatalf("upgraded join: %v", err)
	}
	if _, err := os.Stat(store); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the recovered store survived its join")
	}
}

func TestBeforeProofMoveAndMissingReaderBrief(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"live-tree", "reader-brief"} {
		t.Run(scenario, func(t *testing.T) {
			f, repo := newGitUnitFixture(t)
			plan, err := ReadUnitPlan(f.plan)
			if err != nil {
				t.Fatal(err)
			}
			changed := false
			f.runner.AfterWrite = func(r UnitRunRecord) error {
				if !changed && len(r.Rounds) == 1 && r.Rounds[0].Result != nil && len(r.Rounds[0].Steps) == 2 {
					changed = true
					if scenario == "reader-brief" {
						return os.Remove(plan.Read.Brief)
					}
					return os.WriteFile(filepath.Join(repo, "later"), []byte("changed after freeze"), 0600)
				}
				return nil
			}
			result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil {
				t.Fatal(err)
			}
			round := result.Record.Rounds[0]
			if scenario == "live-tree" {
				if round.Outcome != "proof-wrote" || round.ReadStore != "" || !slices.Equal(f.starter.order, []string{"build"}) {
					t.Fatalf("moved before proof: %s %v", round.Outcome, f.starter.order)
				}
			} else if round.Outcome != "read-failed" || !slices.Equal(f.starter.order, []string{"build", "proof"}) || !round.CheckoutRemoved {
				t.Fatalf("reader setup lost the check: %s %v", round.Outcome, f.starter.order)
			}
		})
	}
}

func TestProofCannotChangeTheReadersDeclaredSubjectInputs(t *testing.T) {
	t.Parallel()
	f, repo := newGitUnitFixture(t)
	plan, err := ReadUnitPlan(f.plan)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(repo, "checklist")
	writeFile(t, input, "frozen checklist")
	plan.Read.Inputs = []string{input}
	plan.Read.Brief = filepath.Join(repo, "reader-instructions")
	writeFile(t, plan.Read.Brief, "Read the frozen checklist at "+input+"\n")
	if err := writeUnitJSON(f.plan, plan, ""); err != nil {
		t.Fatal(err)
	}
	f.starter.holdKind = "proof"
	f.starter.readVerdict = "fix first (1 material findings)"
	f.starter.onStart = func(r Record) error {
		switch r.Kind {
		case "proof":
			writeFile(t, input, "changed by proof")
			writeFile(t, plan.Read.Brief, "changed by proof")
		case "read":
			if len(r.Inputs) < 2 {
				t.Fatal("reader lost its declared checklist")
			}
			brief, err := os.ReadFile(r.Inputs[0].Path)
			if err != nil {
				return err
			}
			if !strings.Contains(string(brief), filepath.Join(r.WorkingDirectory, "checklist")) {
				t.Fatal("reader instructions reference the live checklist")
			}
			checklist, err := os.ReadFile(r.Inputs[1].Path)
			if err != nil {
				return err
			}
			if string(checklist) != "frozen checklist" {
				t.Fatal("the check changed the contents of the reader's declared input")
			}
		}
		return nil
	}
	first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil || !first.Capped {
		t.Fatalf("check with early findings: cap=%v err=%v", first.Capped, err)
	}
	r := first.Record.Rounds[0]
	if r.Steps[2].Verdict != "fix first (1 material findings)" || r.Steps[1].State != StepRunning {
		t.Fatal("the reader's early findings interrupted the check")
	}
	endFrozenTestLaunch(t, f, r.Steps[1].LaunchID, Completed, 0)
	last, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil {
		t.Fatal(err)
	}
	if done := last.Record.Rounds[0]; done.Outcome != "proof-wrote" || !done.Steps[2].Unattached || len(done.Reads) > 0 {
		t.Fatal("the changed subject retained the early findings")
	}
}
