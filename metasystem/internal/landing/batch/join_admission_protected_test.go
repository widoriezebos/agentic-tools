package batch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Admission must use the selected exact-tree work after claim handover.
func TestBatchJoinRunsItsOwnGate(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	unit := joiningUnit("goal-a", "chain-a")
	unit.Gate = []string{"caller-supplied"}
	admissionTree := bed.expectJoin(unit, testpolicy.Plan{SelectedGroups: []string{"required-a"}, RequiredGroups: []string{"required-a"}})
	handedOver, calls := false, 0
	err := PublishJoinWithAdmission(store, testBatchID, unit, "seat+goal-a", time.Unix(1, 0),
		func(_, _, _ string) (testpolicy.Plan, error) {
			return testpolicy.Plan{SelectedGroups: []string{"required-a"}, RequiredGroups: []string{"required-a"}}, nil
		}, func() error { handedOver = true; return nil },
		func(_ string, joining Unit) (JoinAdmission, error) {
			calls++
			if !handedOver || joining.State != UnitJoining || joining.Admission == nil || joining.Admission.Status != "handed-over" ||
				!slices.Equal(joining.SelectedGroups, []string{"required-a"}) {
				t.Fatalf("shared admission was not called after handover with selected work: %+v", joining)
			}
			return JoinAdmission{Tree: joining.Admission.Tree, Status: "verified", AttemptID: "own-attempt"}, nil
		})
	must(t, err)
	joined := load(t, store).Units[0]
	if calls != 1 || joined.State != UnitJoined || joined.Admission.Tree != admissionTree || joined.Admission.AttemptID != "own-attempt" ||
		!slices.Contains(joined.Gate, "own-attempt") {
		t.Fatalf("shared admission did not certify exact member: calls=%d unit=%+v", calls, joined)
	}
}

func TestBatchJoinRefusesRedStep(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	bed.expectJoin(joiningUnit("goal-a", "chain-a"), testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
	red := &JoinAdmissionRed{Reason: "BATCH_JOIN_ADMISSION_RED: required group failed"}
	err := PublishJoinWithAdmission(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0),
		joinPlanMode(testpolicy.ModeStandard), func() error { return nil },
		func(_ string, joining Unit) (JoinAdmission, error) {
			if joining.Admission == nil || joining.Admission.Status != "handed-over" {
				t.Fatalf("red step ran before handover: %+v", joining)
			}
			return JoinAdmission{}, red
		})
	if !errors.Is(err, red) {
		t.Fatalf("red step result=%v", err)
	}
	record := load(t, store)
	if record.Units[0].State != UnitReturnPending || record.Units[0].Outcome != UnitEjected || record.State != StateDissolved {
		t.Fatalf("red required step entered membership: %+v", record)
	}
}

func TestBatchJoinIgnoresSuppliedResults(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	prior := filepath.Join(bed.root, "artifacts", "agents", "proof-runs", "supplied.json")
	must(t, os.MkdirAll(filepath.Dir(prior), 0o755))
	must(t, os.WriteFile(prior, []byte(`{"runId":"supplied","status":"passed"}`), 0o644))
	unit := joiningUnit("goal-a", "chain-a")
	unit.Gate = []string{"supplied"}
	bed.expectJoin(unit, testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
	called := false
	err := PublishJoinWithAdmission(store, testBatchID, unit, "seat+goal-a", time.Unix(1, 0),
		joinPlanMode(testpolicy.ModeStandard), func() error { return nil },
		func(_ string, joining Unit) (JoinAdmission, error) {
			called = true
			if !slices.Equal(joining.Gate, []string{"supplied"}) {
				t.Fatalf("historical run IDs were rewritten before admission: %v", joining.Gate)
			}
			return JoinAdmission{}, &JoinAdmissionRed{Reason: "BATCH_JOIN_ADMISSION_RED: own attempt failed"}
		})
	if !called || err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_ADMISSION_RED") {
		t.Fatalf("caller-supplied result skipped own admission: called=%v err=%v", called, err)
	}
	record := load(t, store)
	if record.Units[0].State != UnitReturnPending || record.Units[0].Outcome != UnitEjected {
		t.Fatalf("supplied result certified a red member: %+v", record.Units[0])
	}
}

func TestBatchJoinRefusesUnmappedFixtureBed(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	contract := testpolicy.Contract{
		Surfaces: []testpolicy.Surface{{ID: "known", Paths: []string{"scripts/agents/known-fixtures.sh"}, Standard: []string{"known"}}},
		Groups:   []testpolicy.Group{{ID: "known"}, {ID: "unknown"}},
		Unknown:  []string{"unknown"},
	}
	plan := func(_, _, _ string) (testpolicy.Plan, error) {
		return testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: []string{"scripts/agents/unmapped-fixtures.sh"}, Purpose: testpolicy.PurposeDelivery})
	}
	wantPlan, planErr := plan("", "", "")
	must(t, planErr)
	bed.expectJoin(joiningUnit("goal-a", "chain-a"), wantPlan)
	called := false
	err := PublishJoinWithAdmission(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0), plan,
		func() error { return nil }, func(_ string, joining Unit) (JoinAdmission, error) {
			called = true
			if !slices.Equal(joining.SelectedGroups, []string{"unknown"}) {
				t.Fatalf("unmapped fixture skipped unknown fallback: %v", joining.SelectedGroups)
			}
			return JoinAdmission{}, &JoinAdmissionRed{Reason: "BATCH_JOIN_ADMISSION_RED: unknown-path fallback failed"}
		})
	if !called || err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_ADMISSION_RED") {
		t.Fatalf("unmapped fixture was not rejected by fallback: called=%v err=%v", called, err)
	}
	record := load(t, store)
	if record.Units[0].State != UnitReturnPending || record.Units[0].Outcome != UnitEjected {
		t.Fatalf("unmapped fixture entered membership: %+v", record.Units[0])
	}
}
