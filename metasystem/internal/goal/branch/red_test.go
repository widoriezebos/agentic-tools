package branch_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type fakeRedRunner struct {
	result branch.DiagnosticResult
	runs   []branch.DiagnosticRun
}

func (f *fakeRedRunner) Run(run branch.DiagnosticRun) (branch.DiagnosticResult, error) {
	f.runs = append(f.runs, run)
	return f.result, nil
}

type fakeTrunkRed struct{ entries []branch.TrunkRedEntry }

func (f *fakeTrunkRed) RecordTrunkRed(entry branch.TrunkRedEntry) error {
	f.entries = append(f.entries, entry)
	return nil
}

type fakeLandingProgress struct{ lines, next []string }

func (f *fakeLandingProgress) RecordLandingProgress(line, next string) error {
	f.lines = append(f.lines, line)
	f.next = append(f.next, next)
	return nil
}

func redRequest(runner branch.RedRunner, ledger branch.TrunkRedRecorder, progress branch.LandingProgressRecorder) branch.RedRequest {
	hex := strings.Repeat("a", 40)
	return branch.RedRequest{Goal: "goal-a", Endpoint: hex, Branch: "landing/goal-a", BranchTip: strings.Repeat("b", 40),
		LastUnit: "u3", Proof: branch.LandingProof{Number: 2, Endpoint: hex, Candidate: strings.Repeat("c", 40),
			Landing: strings.Repeat("d", 40), Attempt: "deep-two"},
		FailingGroups: []string{"external"}, ChangeSet: []string{"metasystem/internal/goal/branch/red.go"},
		Contract: testpolicy.Contract{Groups: []testpolicy.Group{
			{ID: "owned", Inputs: []string{"metasystem/internal/goal/**"}},
			{ID: "external", Inputs: []string{"metasystem/internal/contract/**"}},
		}}, AdmitDiagnostic: func() error { return nil }, Runner: runner, TrunkRed: ledger, Progress: progress}
}

func TestLandingRedLoopClassificationAndBudget(t *testing.T) {
	t.Run("manifest-owned failure", func(t *testing.T) {
		runner := &fakeRedRunner{result: branch.DiagnosticResult{AttemptID: "must-not-run", Green: true}}
		progress := &fakeLandingProgress{}
		req := redRequest(runner, &fakeTrunkRed{}, progress)
		req.FailingGroups = []string{"owned"}
		result, err := branch.HandleLandingRed(req)
		if err != nil || result.Classification != "goal-red" || len(runner.runs) != 0 || len(progress.lines) != 1 {
			t.Fatalf("owned result=%+v err=%v runs=%v progress=%v", result, err, runner.runs, progress.lines)
		}
	})

	t.Run("endpoint green means integration red", func(t *testing.T) {
		runner := &fakeRedRunner{result: branch.DiagnosticResult{AttemptID: "endpoint-green", Green: true}}
		progress := &fakeLandingProgress{}
		result, err := branch.HandleLandingRed(redRequest(runner, &fakeTrunkRed{}, progress))
		if err != nil || result.Classification != "goal-red" || result.EndpointAttempt != "endpoint-green" ||
			len(runner.runs) != 1 || runner.runs[0].Tree != strings.Repeat("a", 40) ||
			runner.runs[0].Purpose != "diagnostic" || runner.runs[0].Mode != "canary" || !runner.runs[0].NoReuse {
			t.Fatalf("green endpoint result=%+v err=%v runs=%+v", result, err, runner.runs)
		}
		if len(progress.lines) != 1 || !strings.Contains(progress.next[0], "proof 2") {
			t.Fatalf("progress lines=%v next=%v", progress.lines, progress.next)
		}
	})

	t.Run("endpoint red opens register and holds", func(t *testing.T) {
		runner := &fakeRedRunner{result: branch.DiagnosticResult{AttemptID: "endpoint-red"}}
		ledger, progress := &fakeTrunkRed{}, &fakeLandingProgress{}
		result, err := branch.HandleLandingRed(redRequest(runner, ledger, progress))
		var refusal *branch.OpError
		if !errors.As(err, &refusal) || refusal.Code != branch.LandTrunkRedCode || result.Classification != "trunk-red" ||
			len(ledger.entries) != 1 || ledger.entries[0].Branch != "landing/goal-a" ||
			ledger.entries[0].Commit != strings.Repeat("b", 40) || ledger.entries[0].Attempt != "endpoint-red" ||
			!ledger.entries[0].Open || len(progress.lines) != 1 {
			t.Fatalf("trunk result=%+v err=%v entries=%+v progress=%v", result, err, ledger.entries, progress.lines)
		}
	})

	t.Run("claim admission precedes runner", func(t *testing.T) {
		runner := &fakeRedRunner{result: branch.DiagnosticResult{AttemptID: "must-not-run", Green: true}}
		req := redRequest(runner, &fakeTrunkRed{}, &fakeLandingProgress{})
		req.AdmitDiagnostic = func() error { return fmt.Errorf("attempt box exhausted") }
		_, err := branch.HandleLandingRed(req)
		if err == nil || !strings.Contains(err.Error(), "attempt box exhausted") || len(runner.runs) != 0 {
			t.Fatalf("admission err=%v runs=%v", err, runner.runs)
		}
	})

	t.Run("proof record appends red and green counts", func(t *testing.T) {
		progress := &fakeLandingProgress{}
		req := redRequest(&fakeRedRunner{}, &fakeTrunkRed{}, progress)
		req.FailingGroups = []string{"owned"}
		if _, err := branch.HandleLandingRed(req); err != nil {
			t.Fatal(err)
		}
		green := req.Proof
		green.Number = 3
		if err := branch.RecordLandingGreen(progress, req.Goal, req.LastUnit, green); err != nil {
			t.Fatal(err)
		}
		if len(progress.lines) != 2 || !strings.Contains(progress.lines[0], "verdict=red") ||
			!strings.Contains(progress.lines[1], "verdict=green") ||
			!strings.Contains(progress.next[0], "proof 2") || progress.next[1] != "LANDED goal-a through u3 after 3 proofs" {
			t.Fatalf("lines=%v next=%v", progress.lines, progress.next)
		}
	})
}
