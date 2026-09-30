package cadence

import (
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

var validateTestNow = time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC)

// startOrphanedGroup starts a shell leading its own process group that
// leaves a sleeping child in the group and exits: the group outlives its
// leader, as a run's workers outlive the run a run store has already
// terminalized. It returns the dead leader's identity and the group id;
// cleanup ends the group by its literal id.
func startOrphanedGroup(t *testing.T) (identity.Ref, int) {
	t.Helper()
	command := exec.Command("sh", "-c", "sleep 120 & echo started")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	group := command.Process.Pid
	t.Cleanup(func() { _ = syscall.Kill(-group, syscall.SIGKILL) })
	exact, state, err := (identity.KernelProber{}).Probe(int64(group))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe the group leader: %v %v", state, err)
	}
	buffer := make([]byte, 16)
	if n, _ := stdout.Read(buffer); !strings.HasPrefix(string(buffer[:n]), "started") {
		t.Fatalf("the group leader said %q", buffer[:n])
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	return exact.Ref(), group
}

// endGroup ends a group this test started and waits, bounded, until the
// kernel reports it empty.
func endGroup(t *testing.T, group int) {
	t.Helper()
	_ = syscall.Kill(-group, syscall.SIGKILL)
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if members, err := custody.GroupMembers(int64(group)); err == nil && !members {
			return
		}
	}
	t.Fatalf("process group %d did not empty", group)
}

// TestTerminalRunWithLiveGroupIsUnsettled (K9): the run store already
// reads the validation run as ended green (what run/conclude.go does to a
// draining run at its wind-down, members or not), but a process of its
// group still runs. The reservation is not finalized, nothing is
// published, and the agent's wake does not call it pending; once the group
// is empty the same call finalizes it.
func TestTerminalRunWithLiveGroupIsUnsettled(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "home")
	leader, group := startOrphanedGroup(t)
	record, err := custody.Open(home, custody.KindValidate, custodySubject("cadence-run-9"), validateTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := custody.BindChild(home, record.ID, leader); err != nil {
		t.Fatal(err)
	}
	tree := strings.Repeat("1", 40)
	reservation := gaterun.Validation{Key: goal.CadenceClaimKey{TrunkTree: tree, WeightGeneration: 3, ForcedWindowStart: "2026-09-30T18:00:00Z"},
		RunID: "cadence-run-9", Custody: record.ID, Trunk: gaterun.CadenceTrunk{Commit: strings.Repeat("2", 40), Tree: tree},
		Trigger: goal.CadenceTriggerIdentityChanged, Authority: gaterun.CadenceAuthority{GoalID: AuthorityGoal, ObligationRevision: 2},
		DeepOnly: []string{"deep"}, Probes: []proofrun.GroupResult{{ID: "deep", Status: "passed", ExecutionIdentity: strings.Repeat("a", 64)}},
		ReservedAt: validateTestNow.Add(-time.Hour).Format(time.RFC3339)}
	if err := Reserve(home, reservation); err != nil {
		t.Fatal(err)
	}
	var published []string
	terminalGreen := gaterun.RunOutcome{Usable: true, Result: proofrun.TestResult{AttemptID: "attempt-9",
		Groups: []proofrun.GroupResult{{ID: "deep", Status: "passed", CollectionComplete: true, ExecutionIdentity: strings.Repeat("a", 64)}}}}
	unused := func(what string) func() error {
		return func() error { t.Fatalf("a reserved run reached %s", what); return nil }
	}
	seams := gaterun.ValidateSeams{
		Clock:       func() time.Time { return validateTestNow },
		Reservation: func() (*gaterun.Validation, error) { return ReadReservation(home) },
		Reserve:     func(v gaterun.Validation) error { return Reserve(home, v) },
		Record:      func(v gaterun.Validation) error { return RecordReservation(home, v) },
		Clear:       func(runID string) error { return ClearReservation(home, runID) },
		Custody: func(v gaterun.Validation) (string, string) {
			return ReservationCustody(home, v, custody.Probes{})
		},
		Barrier: func() ([]string, []string, error) {
			settlement, err := custody.Settle(home, custody.Probes{})
			return settlement.Live, settlement.Unknown, err
		},
		// The attached call's wait is over before the group empties.
		Wait:    func(gaterun.Validation) error { return nil },
		Outcome: func(gaterun.Validation) gaterun.RunOutcome { return terminalGreen },
		Gap:     unused("the authority check"),
		Publish: func(_ goal.CadenceClaimKey, status goal.CadenceStatus, _ []goal.TrunkRedRecordGroup) error {
			published = append(published, status.RunID)
			return nil
		},
	}
	outcome, err := gaterun.Validate(false, seams)
	if err != nil || outcome.Result != gaterun.ValidateWaiting || !outcome.Attached || len(published) != 0 {
		t.Fatalf("a terminal run whose group still runs: outcome %+v %v, published %q; want it attached and unsettled", outcome, err, published)
	}
	if kept, err := ReadReservation(home); err != nil || kept == nil || kept.RunID != "cadence-run-9" {
		t.Fatalf("reservation after the attach = %+v %v; want it kept", kept, err)
	}
	if pending, err := FinalizationPending(home); err != nil || pending {
		t.Fatalf("finalization pending while the group runs = %v %v; want not pending", pending, err)
	}
	endGroup(t, group)
	if pending, err := FinalizationPending(home); err != nil || !pending {
		t.Fatalf("finalization pending once the group ended = %v %v; want pending", pending, err)
	}
	outcome, err = gaterun.Validate(false, seams)
	if err != nil || outcome.Result != gaterun.ValidateFinalized || len(published) != 1 || published[0] != "cadence-run-9" {
		t.Fatalf("after the group ended: outcome %+v %v, published %q; want it finalized once", outcome, err, published)
	}
	if kept, err := ReadReservation(home); err != nil || kept != nil {
		t.Fatalf("reservation after finalizing = %+v %v; want it cleared", kept, err)
	}
	if pending, err := FinalizationPending(home); err != nil || pending {
		t.Fatalf("finalization pending after finalizing = %v %v", pending, err)
	}
}

// The reservation is durable and fenced by its run id: a second run can't
// be reserved over it, and only its own run id clears it.
func TestValidationReservationIsFencedByRunID(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "home")
	first := gaterun.Validation{Key: goal.CadenceClaimKey{TrunkTree: strings.Repeat("1", 40)}, RunID: "cadence-run-1"}
	if err := Reserve(home, first); err != nil {
		t.Fatal(err)
	}
	if err := Reserve(home, gaterun.Validation{Key: first.Key, RunID: "cadence-run-2"}); err == nil {
		t.Fatal("a second run was reserved over the first")
	}
	if err := ClearReservation(home, "cadence-run-2"); err == nil {
		t.Fatal("another run id cleared the reservation")
	}
	first.Custody = "validate-1"
	if err := RecordReservation(home, first); err != nil {
		t.Fatal(err)
	}
	if kept, err := ReadReservation(home); err != nil || kept == nil || kept.Custody != "validate-1" {
		t.Fatalf("reservation = %+v %v", kept, err)
	}
	if err := ClearReservation(home, "cadence-run-1"); err != nil {
		t.Fatal(err)
	}
	if kept, err := ReadReservation(home); err != nil || kept != nil {
		t.Fatalf("reservation after clear = %+v %v", kept, err)
	}
}

// A reservation whose launch never recorded a custody record: nothing was
// opened for its run (nothing started) reads settled; a record opened for
// it is found by its run id.
func TestReservationCustodyFindsTheRunsRecord(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "home")
	reservation := gaterun.Validation{RunID: "cadence-run-3"}
	if state, why := ReservationCustody(home, reservation, custody.Probes{}); state != gaterun.CustodyDead {
		t.Fatalf("nothing opened: %s %s", state, why)
	}
	if _, err := custody.Open(home, custody.KindValidate, custodySubject("cadence-run-3"), validateTestNow); err != nil {
		t.Fatal(err)
	}
	// Opened by this (live) process and not yet bound: it is starting.
	if state, why := ReservationCustody(home, reservation, custody.Probes{}); state != gaterun.CustodyLive {
		t.Fatalf("opened, not bound: %s %s", state, why)
	}
	// No reservation is written in this bed: nothing is pending.
	if pending, err := FinalizationPending(home); err != nil || pending {
		t.Fatalf("finalization pending without a reservation = %v %v", pending, err)
	}
}
