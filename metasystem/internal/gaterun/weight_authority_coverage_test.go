package gaterun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func addWeightLanding(t *testing.T, root string) {
	t.Helper()
	if _, _, err := WeightAdd(root, "landing", []byte("1\t0\tdirect.go\n"), "", 0); err != nil {
		t.Fatal(err)
	}
}

func completeWeightProof(t *testing.T, root, id string, now *time.Time, exitCode int64, mutate func(*run.GovernedAdmissionResult)) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	prober := &proofProber{alive: true, started: *now}
	store := &run.Store{Root: root, Now: func() time.Time { return *now }, Prober: prober,
		Getpgid: func(pid int64) (int64, error) { return pid, nil }, AllPids: func() ([]int64, error) { return nil, nil }}
	store.AdmitGoverned = func(request run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
		admission, err := dispatch.EvaluateGovernedRunAdmission(root, request, *now)
		if err == nil && mutate != nil {
			mutate(&admission)
		}
		return admission, err
	}
	store.ObserveGoverned = func(record *run.Record, ended time.Time) run.AssumptionObservation {
		return dispatch.ObserveGovernedRun(root, record, ended)
	}
	nonce, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: id, Kind: "suite",
		Display: "weight authority proof", Log: filepath.Join("artifacts", id+".log"), GoalId: "bounded",
		ObligationRevision: 3, StandingShared: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind(id, nonce, 4343, 4343); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	if err := store.WriteSidecar(id, 1, nonce, exitCode); err != nil {
		t.Fatal(err)
	}
	prober.alive = false
	result, err := store.Assess(id)
	want := run.StatusGreen
	if exitCode != 0 {
		want = run.StatusRed
	}
	if err != nil || !result.Transitioned || result.To != want {
		t.Fatalf("proof did not terminalize through real run files: %+v %v", result, err)
	}
}

func weightAuthorityBed(t *testing.T) (string, *time.Time) {
	t.Helper()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	root, _ := governedWeightBed(t, now)
	priorNow := weightNow
	weightNow = func() time.Time { return now }
	t.Cleanup(func() { weightNow = priorNow })
	addWeightLanding(t, root)
	return root, &now
}

func TestWeightAddAndCheckUsePersistedThresholdState(t *testing.T) {
	root := t.TempDir()
	state, reached, err := WeightCheck(root, 1)
	if err != nil || reached || state.Generation != 0 || state.Accumulated != 0 {
		t.Fatalf("empty weight check was not a clean zero state: %+v reached=%t err=%v", state, reached, err)
	}
	state, reached, err = WeightAdd(root, "landing-one", []byte("1\t0\tdirect.go\n"), "", 0)
	if err != nil || reached || state.Generation != 1 || state.Accumulated != 3 || state.Landings != 1 {
		t.Fatalf("weight add did not persist its landing: %+v reached=%t err=%v", state, reached, err)
	}
	checked, reached, err := WeightCheck(root, 3)
	if err != nil || !reached || checked.Generation != state.Generation || checked.Accumulated != state.Accumulated {
		t.Fatalf("weight check did not read the persisted threshold: %+v reached=%t err=%v", checked, reached, err)
	}
	if _, _, err := WeightAdd(root, "bad-landing", []byte("not-a-count\t0\tbroken.go\n"), "", 1); err == nil || !strings.Contains(err.Error(), "invalid numstat count") {
		t.Fatalf("malformed landing weight was accepted: %v", err)
	}
	if err := os.WriteFile(weightPath(root), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WeightCheck(root, 3); err == nil {
		t.Fatal("an unreadable persisted weight state passed check")
	}
}

func TestWeightDischargeRefusesWrongRevisionAndPolicy(t *testing.T) {
	t.Run("obligation revision", func(t *testing.T) {
		root, _ := weightAuthorityBed(t)
		if _, err := WeightDischarge(root, "bounded", 4, "wrong-revision"); err == nil ||
			!strings.Contains(err.Error(), "accepted obligation revision 4") {
			t.Fatalf("wrong obligation revision did not refuse: %v", err)
		}
	})

	t.Run("correlation policy", func(t *testing.T) {
		root, now := weightAuthorityBed(t)
		completeWeightProof(t, root, "policy-proof", now, 0, nil)
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=B\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := WeightDischarge(root, "bounded", 3, "policy-proof"); err == nil ||
			!strings.Contains(err.Error(), "current recorded authority and policy") {
			t.Fatalf("policy mismatch did not refuse discharge: %v", err)
		}
	})
}

func TestWeightDischargeRefusesNonGreenAndStaleBudgetEpoch(t *testing.T) {
	t.Run("non-green proof", func(t *testing.T) {
		root, now := weightAuthorityBed(t)
		completeWeightProof(t, root, "red-proof", now, 1, nil)
		if _, err := WeightDischarge(root, "bounded", 3, "red-proof"); err == nil ||
			!strings.Contains(err.Error(), "not an exact green governed proof") {
			t.Fatalf("non-green proof did not refuse discharge: %v", err)
		}
	})

	t.Run("budget epoch", func(t *testing.T) {
		root, now := weightAuthorityBed(t)
		completeWeightProof(t, root, "epoch-reset-proof", now, 0, nil)
		if result, err := WeightDischarge(root, "bounded", 3, "epoch-reset-proof"); err != nil || !result.Decision.Applied {
			t.Fatalf("fixture could not establish the next budget epoch: %+v %v", result, err)
		}
		completeWeightProof(t, root, "wrong-epoch-proof", now, 0, func(admission *run.GovernedAdmissionResult) {
			admission.Attempt.BudgetEpoch = nil
			admission.Attempt.AttemptOrdinal = 2
		})
		if _, err := WeightDischarge(root, "bounded", 3, "wrong-epoch-proof"); err == nil ||
			!strings.Contains(err.Error(), "not bound to the current obligation budget epoch") {
			t.Fatalf("stale budget epoch did not receive typed refusal: %v", err)
		}
	})
}

func TestWeightDischargeRefusesWhenRetroObligationCannotBeRaised(t *testing.T) {
	root, now := weightAuthorityBed(t)
	completeWeightProof(t, root, "retro-failure-proof", now, 0, nil)
	before, err := loadWeight(root, *now)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(retrodebt.Path(root), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := WeightDischarge(root, "bounded", 3, "retro-failure-proof")
	if err == nil || !strings.Contains(err.Error(), "retro obligation could not be raised") {
		t.Fatalf("failed retro publication did not refuse discharge: %+v %v", result, err)
	}
	after, loadErr := loadWeight(root, *now)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if result.Decision.Applied || after.Generation != before.Generation || after.Accumulated != before.Accumulated || after.Landings != before.Landings {
		t.Fatalf("failed retro publication changed weight authority: before=%+v after=%+v result=%+v", before, after, result)
	}
}
