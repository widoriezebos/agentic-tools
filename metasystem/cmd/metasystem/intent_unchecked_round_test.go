package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

type refusingProofStarter struct {
	launch.SupervisorStarter
	bed           *workBed
	executedFirst bool
	attempts      int
}

func (s *refusingProofStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	r, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if r.Kind != "proof" {
		return s.SupervisorStarter.StartSupervisor(id, state)
	}
	s.attempts++
	if s.executedFirst && s.attempts == 1 {
		s.bed.starter.fail["proof"] = true
		ref, err := s.SupervisorStarter.StartSupervisor(id, state)
		delete(s.bed.starter.fail, "proof")
		if err == nil {
			_, err = s.bed.manager.Store.Update(id, func(r *launch.Record) error { r.Cause = "environment"; return nil })
		}
		return ref, err
	}
	return identity.Ref{}, fmt.Errorf("fixture proof supervisor refused")
}

func writeUncheckedCandidate(t *testing.T, b *workBed, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(b.worktree, "unit.go"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// failedUncheckedBuild retains a candidate without running its proof.
func failedUncheckedBuild(t *testing.T, b *workBed) launch.UnitRunRecord {
	t.Helper()
	b.starter.fail["build"] = true
	b.manager.Supervisor = outcomeStarter{bed: b, build: func() { writeUncheckedCandidate(t, b, "new\n") }}
	code, result, _ := outcomeBuild(t, b)
	delete(b.starter.fail, "build")
	if code != 1 || resultData(t, result)["outcome"] != "build-failed" {
		t.Fatalf("first build: code=%d outcome=%v", code, resultData(t, result)["outcome"])
	}
	return onlyRun(t, b)
}

func reviseUncheckedBuild(t *testing.T, b *workBed) (int, launch.UnitRound) {
	t.Helper()
	code, _, _ := b.work("work", "revise", b.id, "--work", "outcome", "--after", "1", "--brief", b.brief("correction.md", "Correct the implementation.\n"), "--reason", "Revisit the result", "--by", "Wido")
	r := onlyRun(t, b)
	if len(r.Rounds) != 2 {
		t.Fatalf("correction was not admitted: code=%d rounds=%d", code, len(r.Rounds))
	}
	return code, r.Rounds[1]
}

func assertUncheckedCount(t *testing.T, round launch.UnitRound, lines int64) {
	t.Helper()
	if round.BuildLines != lines || round.ProductionLines == nil || *round.ProductionLines != lines || round.DeclaredLines != 1 {
		t.Fatalf("count: total=%d production=%v declared=%d, want %d and allocation 1", round.BuildLines, round.ProductionLines, round.DeclaredLines, lines)
	}
}

func assertExecutedProof(t *testing.T, b *workBed, round launch.UnitRound) {
	t.Helper()
	if len(round.Steps) < 2 {
		t.Fatal("no proof step")
	}
	r, err := b.manager.Store.Read(round.Steps[1].LaunchID)
	if err != nil || !r.Executed() {
		t.Fatalf("proof did not execute: state=%s reason=%s err=%v", r.State, r.Reason, err)
	}
}

// Regression: DONE already routes to the check, but must count inherited lines.
func TestACorrectionAfterAnUnfrozenFailureIsChecked(t *testing.T) {
	t.Parallel()
	for _, zeroCounts := range []bool{false, true} {
		t.Run(map[bool]string{false: "counted", true: "uncounted"}[zeroCounts], func(t *testing.T) {
			t.Parallel()
			b := realOutcomeBed(t)
			r := failedUncheckedBuild(t, b)
			if zeroCounts {
				r.Rounds[0].BuildLines, r.Rounds[0].DeclaredLines = 0, 0
				data, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(b.unitRoot, r.ID, "run.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			b.manager.Supervisor = doneRoundStarter{outcomeStarter{bed: b}, b}
			code, round := reviseUncheckedBuild(t, b)
			if code != 0 || round.Outcome != "green" || !slices.Equal(b.starter.launched(), []string{"build", "build", "proof"}) {
				t.Fatalf("inherited candidate: code=%d outcome=%s launches=%v", code, round.Outcome, b.starter.launched())
			}
			assertUncheckedCount(t, round, 2)
			assertExecutedProof(t, b, round)
		})
	}
}

type blockedOutcomeStarter struct{ outcomeStarter }

func (s blockedOutcomeStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	ref, err := s.outcomeStarter.StartSupervisor(id, state)
	r, readErr := s.bed.manager.Store.Read(id)
	if err == nil && readErr == nil && r.Kind == "build" {
		err = os.WriteFile(filepath.Join(state, "last-message.txt"), []byte("RESULT: BLOCKED (no host test slot)\n"), 0600)
	}
	return ref, err
}

// Regression for the unchanged candidate; preservation for a changed red one.
func TestABlockedBuilderGoesToTheCheck(t *testing.T) {
	t.Parallel()
	for _, broken := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "changed-broken"}[broken], func(t *testing.T) {
			t.Parallel()
			b := realOutcomeBed(t)
			failedUncheckedBuild(t, b)
			b.manager.Supervisor = blockedOutcomeStarter{outcomeStarter{bed: b, build: func() {
				if broken {
					writeUncheckedCandidate(t, b, "broken\n")
				}
			}}}
			b.starter.fail["proof"] = broken
			code, round := reviseUncheckedBuild(t, b)
			wantCode, wantOutcome := 0, "green"
			if broken {
				wantCode, wantOutcome = 1, "proof-red"
			}
			if code != wantCode || round.Outcome != wantOutcome || round.Steps[0].State != launch.StepPassed || round.Steps[0].Reason != "no-result-block" {
				t.Fatalf("blocked builder: code=%d outcome=%s build=%s reason=%s", code, round.Outcome, round.Steps[0].State, round.Steps[0].Reason)
			}
			assertUncheckedCount(t, round, 2)
			assertExecutedProof(t, b, round)
		})
	}
}

// Regression when both starts fail; preservation when the first proof executed.
func TestAProofThatNeverStartedIsNoCheck(t *testing.T) {
	t.Parallel()
	for _, executed := range []bool{false, true} {
		t.Run(map[bool]string{false: "never-executed", true: "executed-then-refused"}[executed], func(t *testing.T) {
			t.Parallel()
			b := realOutcomeBed(t)
			s := &refusingProofStarter{SupervisorStarter: outcomeStarter{bed: b, build: func() { writeUncheckedCandidate(t, b, "new\n") }}, bed: b, executedFirst: executed}
			b.manager.Supervisor = s
			code, _, _ := outcomeBuild(t, b)
			r := onlyRun(t, b)
			step := r.Rounds[0].Steps[1]
			if code != 1 || s.attempts != 2 || r.Rounds[0].Outcome != "proof-red" || len(step.LaunchIDs) != 2 || step.LaunchID != step.LaunchIDs[1] {
				t.Fatalf("proof starts: code=%d attempts=%d outcome=%s ids=%v current=%s", code, s.attempts, r.Rounds[0].Outcome, step.LaunchIDs, step.LaunchID)
			}
			for index, id := range step.LaunchIDs {
				record, err := b.manager.Store.Read(id)
				if err != nil || record.Executed() != (executed && index == 0) {
					t.Fatalf("attempt %d: executed=%v err=%v", index+1, record.Executed(), err)
				}
				if !record.Executed() && record.Reason != "supervisor-start: fixture proof supervisor refused" {
					t.Fatalf("unexpected refusal: %s", record.Reason)
				}
			}
			b.manager.Supervisor = outcomeStarter{bed: b}
			code, round := reviseUncheckedBuild(t, b)
			if executed {
				if code != 1 || round.Outcome != "build-gap" || !slices.Equal(b.starter.launched(), []string{"build", "proof", "build"}) {
					t.Fatalf("checked no-op: code=%d outcome=%s launches=%v", code, round.Outcome, b.starter.launched())
				}
				assertUncheckedCount(t, round, 0)
			} else {
				if code != 0 || round.Outcome != "green" || !slices.Equal(b.starter.launched(), []string{"build", "build", "proof"}) {
					t.Fatalf("unchecked candidate: code=%d outcome=%s launches=%v", code, round.Outcome, b.starter.launched())
				}
				assertUncheckedCount(t, round, 2)
				assertExecutedProof(t, b, round)
			}
		})
	}
}

// Regression: inherit the failed build, excluding work predating round 1.
func TestASizeExclusionHoldsAfterAFailedBuild(t *testing.T) {
	t.Parallel()
	b := realOutcomeBed(t)
	if err := os.WriteFile(filepath.Join(b.worktree, "other.go"), []byte(strings.Repeat("other unit\n", 40)), 0600); err != nil {
		t.Fatal(err)
	}
	failedUncheckedBuild(t, b)
	b.manager.Supervisor = outcomeStarter{bed: b}
	code, round := reviseUncheckedBuild(t, b)
	if code != 0 || round.Outcome != "green" || !slices.Equal(b.starter.launched(), []string{"build", "build", "proof"}) {
		t.Fatalf("pre-existing work: code=%d outcome=%s launches=%v", code, round.Outcome, b.starter.launched())
	}
	assertUncheckedCount(t, round, 2)
	assertExecutedProof(t, b, round)
}
