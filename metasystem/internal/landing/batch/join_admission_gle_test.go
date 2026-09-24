package batch

import (
	"errors"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGLEBatchCrashAfterHandoverDoesNotCertifyWithoutAdmission(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	admissionTree := bed.expectJoin(joiningUnit("goal-a", "chain-a"), testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
	store.seams.publish = func(point string) error {
		if point == "handover" {
			return os.ErrProcessDone
		}
		return nil
	}
	err := PublishJoinWithAdmission(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0),
		joinPlanMode(testpolicy.ModeStandard), func() error { return nil },
		func(string, Unit) (JoinAdmission, error) {
			t.Fatal("admission ran after simulated crash")
			return JoinAdmission{}, nil
		})
	if !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("handover crash result=%v", err)
	}
	if unit := load(t, store).Units[0]; unit.State != UnitJoining || unit.Admission.Status != "pending" || unit.Admission.Tree != admissionTree {
		t.Fatalf("handover crash certified member: %+v", unit)
	}
	must(t, ReconcileJoins(store, testBatchID, bed.base, "landing+owner", time.Unix(2, 0),
		func(string, string, string, string) (Claim, error) {
			return joiningUnit("goal-a", "chain-a").Claim, nil
		}))
	if unit := load(t, store).Units[0]; unit.State != UnitJoining || unit.Admission.Status != "handed-over" {
		t.Fatalf("claim recovery promoted without admission: %+v", unit)
	}
	must(t, ResumeJoinAdmission(store, testBatchID, "goal-a", "landing+owner", time.Unix(3, 0),
		func(_ string, unit Unit) (JoinAdmission, error) {
			return JoinAdmission{Tree: unit.Admission.Tree, Status: "verified", AttemptID: "retained"}, nil
		}))
	if unit := load(t, store).Units[0]; unit.State != UnitJoined || unit.Admission.AttemptID != "retained" || unit.Admission.Tree != admissionTree {
		t.Fatalf("recovered admission was not retained: %+v", unit)
	}
}

func TestGLEBatchJoinAfterHandoverResumesOnlyWithVerifiedAdmission(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	admissionTree := bed.expectJoin(joiningUnit("goal-a", "chain-a"), testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
	crashed := errors.New("collector stopped after handover")
	called := 0
	err := PublishJoinWithAdmission(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0),
		joinPlanMode(testpolicy.ModeStandard), func() error { return nil }, func(_ string, unit Unit) (JoinAdmission, error) {
			called++
			if unit.Admission == nil || unit.Admission.Tree == "" {
				t.Fatal("joining member lacks exact admission tree")
			}
			return JoinAdmission{}, crashed
		})
	if !errors.Is(err, crashed) {
		t.Fatalf("crash result=%v", err)
	}
	must(t, ReconcileJoins(store, testBatchID, bed.base, "landing+owner", time.Unix(2, 0),
		func(string, string, string, string) (Claim, error) {
			return joiningUnit("goal-a", "chain-a").Claim, nil
		}))
	record := load(t, store)
	if record.Units[0].State != UnitJoining || record.Units[0].Admission == nil || record.Units[0].Admission.Status != "handed-over" {
		t.Fatalf("claim-only recovery promoted a member without testing evidence: %+v", record.Units[0])
	}
	must(t, ResumeJoinAdmission(store, testBatchID, "goal-a", "landing+owner", time.Unix(3, 0),
		func(_ string, unit Unit) (JoinAdmission, error) {
			called++
			return JoinAdmission{Tree: unit.Admission.Tree, Status: "verified", AttemptID: "retained-attempt", ResultPath: "retained-result"}, nil
		}))
	must(t, ResumeJoinAdmission(store, testBatchID, "goal-a", "landing+owner", time.Unix(4, 0),
		func(string, Unit) (JoinAdmission, error) {
			t.Fatal("joined member reran admission")
			return JoinAdmission{}, nil
		}))
	record = load(t, store)
	if called != 2 || record.Units[0].State != UnitJoined || record.Units[0].Admission.Tree != admissionTree ||
		record.Units[0].Admission.AttemptID != "retained-attempt" || record.Units[0].Admission.ResultPath != "retained-result" {
		t.Fatalf("admission recovery calls=%d unit=%+v", called, record.Units[0])
	}
}

func TestGLEBatchJoinRedAdmissionReturnsMemberBeforeMembership(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	bed.expectJoin(joiningUnit("goal-a", "chain-a"), testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
	err := PublishJoinWithAdmission(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0),
		joinPlanMode(testpolicy.ModeStandard), func() error { return nil },
		func(string, Unit) (JoinAdmission, error) {
			return JoinAdmission{}, &JoinAdmissionRed{Reason: "BATCH_JOIN_ADMISSION_RED: focused regression"}
		})
	var red *JoinAdmissionRed
	if !errors.As(err, &red) {
		t.Fatalf("red admission result=%v", err)
	}
	record := load(t, store)
	if record.Units[0].State != UnitReturnPending || record.Units[0].Outcome != UnitEjected || record.State != StateDissolved {
		t.Fatalf("red admission entered batch membership: state=%s unit=%+v", record.State, record.Units[0])
	}
}

func TestGLEBatchJoinAdmissionDoesNotHoldBatchLock(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	bed.expectJoin(joiningUnit("goal-a", "chain-a"), testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
	reconcileStore := store
	reconcileStore.seams.flock = func(fd, operation int) error {
		if operation == unix.LOCK_EX {
			operation |= unix.LOCK_NB
		}
		return unix.Flock(fd, operation)
	}
	called := false
	err := PublishJoinWithAdmission(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0),
		joinPlanMode(testpolicy.ModeStandard), func() error { return nil },
		func(_ string, unit Unit) (JoinAdmission, error) {
			called = true
			if err := ReconcileJoins(reconcileStore, testBatchID, bed.base, "landing+owner", time.Unix(2, 0),
				func(string, string, string, string) (Claim, error) {
					t.Fatal("handed-over admission read a claim")
					return Claim{}, nil
				}); err != nil {
				t.Fatalf("reconcile could not acquire the batch flock during admission: %v", err)
			}
			if state := load(t, store).Units[0].State; state != UnitJoining {
				t.Fatalf("pending member state=%s", state)
			}
			return JoinAdmission{Tree: unit.Admission.Tree, Status: "verified"}, nil
		})
	must(t, err)
	if !called || load(t, store).Units[0].State != UnitJoined {
		t.Fatalf("admission callback called=%t, member state=%s", called, load(t, store).Units[0].State)
	}
}
