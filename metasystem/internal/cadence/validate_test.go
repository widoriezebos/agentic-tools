package cadence

import (
	"errors"
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

// startOrphanedGroup starts a process leading its own process group and a
// second process in that group, then ends the leader: the group outlives
// its leader, as a run's workers outlive the run a run store has already
// terminalized. It returns the dead leader's identity and the member,
// which endGroup ends; cleanup ends both by their literal pids.
func startOrphanedGroup(t *testing.T) (identity.Ref, *exec.Cmd) {
	t.Helper()
	leader := exec.Command("sleep", "120")
	leader.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := leader.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = leader.Process.Kill(); _ = leader.Wait() })
	member := exec.Command("sleep", "120")
	member.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: leader.Process.Pid}
	if err := member.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = member.Process.Kill(); _ = member.Wait() })
	exact, state, err := (identity.KernelProber{}).Probe(int64(leader.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe the group leader: %v %v", state, err)
	}
	_ = leader.Process.Kill()
	_ = leader.Wait()
	return exact.Ref(), member
}

// endGroup ends the group's last member and reaps it: the group is empty.
func endGroup(t *testing.T, member *exec.Cmd) {
	t.Helper()
	_ = member.Process.Kill()
	_ = member.Wait()
	if members, err := custody.GroupMembers(int64(member.SysProcAttr.Pgid)); err != nil || members {
		t.Fatalf("process group %d after its last member was reaped: members %v, %v", member.SysProcAttr.Pgid, members, err)
	}
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
	leader, member := startOrphanedGroup(t)
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
	if _, err := StartValidation(home, reservation, passingStart(record.ID)); err != nil {
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
		Clear:       func(v gaterun.Validation) error { return ClearReservation(home, v) },
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
	endGroup(t, member)
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

// passingStart starts a reservation whose barrier is clear, whose authority
// is claimed and whose launch recorded custody id.
func passingStart(id string) StartSteps {
	return StartSteps{Gate: func(start func() error) error { return start() }, Barrier: func() error { return nil },
		Claim: func(time.Time) (gaterun.CadenceAuthority, error) {
			return gaterun.CadenceAuthority{GoalID: AuthorityGoal, ObligationRevision: 2}, nil
		},
		Launch: func(gaterun.Validation) (string, error) { return id, nil },
		Now:    func() time.Time { return validateTestNow }}
}

// The reservation is durable and fenced: a second run can't be reserved
// over it, and a clear of a reservation that has moved since it was read
// is refused.
func TestValidationReservationIsFencedByRunID(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "home")
	first := gaterun.Validation{Key: goal.CadenceClaimKey{TrunkTree: strings.Repeat("1", 40)}, RunID: "cadence-run-1"}
	started, err := StartValidation(home, first, passingStart("validate-1"))
	if err != nil || started.Custody != "validate-1" || started.Authority.GoalID != AuthorityGoal {
		t.Fatalf("start = %+v %v", started, err)
	}
	if _, err := StartValidation(home, gaterun.Validation{Key: first.Key, RunID: "cadence-run-2"}, passingStart("validate-2")); err == nil {
		t.Fatal("a second run was reserved over the first")
	}
	if err := ClearReservation(home, first); !errors.Is(err, gaterun.ErrReservationMoved) {
		t.Fatalf("a clear of the reservation as it was before its custody was recorded = %v; want moved", err)
	}
	if kept, err := ReadReservation(home); err != nil || kept == nil || kept.Custody != "validate-1" {
		t.Fatalf("reservation = %+v %v", kept, err)
	}
	if err := ClearReservation(home, started); err != nil {
		t.Fatal(err)
	}
	if kept, err := ReadReservation(home); err != nil || kept != nil {
		t.Fatalf("reservation after clear = %+v %v", kept, err)
	}
}

// F-4: a second landing validate that reads the reservation between its
// reserve and its custody record (nothing opened yet, so the run reads as
// never started) can't clear it: the reservation is held from the reserve
// to the recorded custody id, and the clear is refused once the id is
// there. The second call waits; the first run keeps its reservation.
func TestValidateCannotClearARunBeingStarted(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "home")
	tree := strings.Repeat("1", 40)
	launching, release := make(chan struct{}), make(chan struct{})
	steps := passingStart("")
	var opened string
	steps.Launch = func(v gaterun.Validation) (string, error) {
		close(launching)
		<-release
		record, err := custody.Open(home, custody.KindValidate, custodySubject(v.RunID), validateTestNow)
		opened = record.ID
		return record.ID, err
	}
	first := make(chan error, 1)
	go func() {
		_, err := StartValidation(home, gaterun.Validation{Key: goal.CadenceClaimKey{TrunkTree: tree}, RunID: "cadence-run-1",
			Trunk: gaterun.CadenceTrunk{Commit: strings.Repeat("2", 40), Tree: tree}}, steps)
		first <- err
	}()
	<-launching
	clearing := make(chan struct{})
	second := make(chan gaterun.ValidateOutcome, 1)
	go func() {
		outcome, err := gaterun.Validate(false, gaterun.ValidateSeams{
			Clock:       func() time.Time { return validateTestNow },
			Reservation: func() (*gaterun.Validation, error) { return ReadReservation(home) },
			Custody: func(v gaterun.Validation) (string, string) {
				return ReservationCustody(home, v, custody.Probes{})
			},
			Barrier: func() ([]string, []string, error) { return nil, nil, nil },
			Outcome: func(gaterun.Validation) gaterun.RunOutcome { return gaterun.RunOutcome{Why: "no run record yet"} },
			Clear: func(v gaterun.Validation) error {
				close(clearing)
				return ClearReservation(home, v)
			},
			// A second call that cleared the first run would go on to plan.
			Gap: func() error { return nil },
			Plan: func() (gaterun.ValidationPlan, error) {
				return gaterun.ValidationPlan{}, errors.New("the second call cleared the first run")
			},
		})
		if err != nil {
			t.Error(err)
		}
		second <- outcome
	}()
	<-clearing
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	outcome := <-second
	if outcome.Result != gaterun.ValidateWaiting {
		t.Fatalf("the second call = %+v; want it waiting", outcome)
	}
	kept, err := ReadReservation(home)
	if err != nil || kept == nil || kept.RunID != "cadence-run-1" || kept.Custody != opened {
		t.Fatalf("reservation = %+v %v; want the first run's with its custody id %s", kept, err, opened)
	}
}

// F-5: the standing authority is claimed only after the custody barrier
// passed, inside the lane's gate: a run the barrier holds writes nothing to
// main and leaves no reservation.
func TestValidateClaimsOnlyPastTheBarrier(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "home")
	claims, launches := 0, 0
	steps := passingStart("validate-1")
	steps.Barrier = func() error { return &custody.Held{Unknown: []string{"prove b1: its launcher died"}} }
	steps.Claim = func(time.Time) (gaterun.CadenceAuthority, error) {
		claims++
		return gaterun.CadenceAuthority{GoalID: AuthorityGoal, ObligationRevision: 2}, nil
	}
	steps.Launch = func(gaterun.Validation) (string, error) { launches++; return "validate-1", nil }
	_, err := StartValidation(home, gaterun.Validation{Key: goal.CadenceClaimKey{TrunkTree: strings.Repeat("1", 40)}, RunID: "cadence-run-1"}, steps)
	var held *gaterun.StartHeld
	if !errors.As(err, &held) || len(held.Unknown) != 1 || claims != 0 || launches != 0 {
		t.Fatalf("start past a holding barrier = %v, claims %d, launches %d; want held with no claim", err, claims, launches)
	}
	if kept, err := ReadReservation(home); err != nil || kept != nil {
		t.Fatalf("reservation = %+v %v; want none", kept, err)
	}
	// A lane that refuses the launch (its pause) refuses before anything.
	steps.Barrier = func() error { return nil }
	steps.Gate = func(func() error) error { return errors.New("the landing lane is stopped") }
	_, err = StartValidation(home, gaterun.Validation{Key: goal.CadenceClaimKey{TrunkTree: strings.Repeat("1", 40)}, RunID: "cadence-run-2"}, steps)
	var blocked *gaterun.StartBlocked
	if !errors.As(err, &blocked) || claims != 0 || launches != 0 {
		t.Fatalf("start on a stopped lane = %v, claims %d; want blocked", err, claims)
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
