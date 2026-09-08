package stopfence

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type claimProber struct{ state identity.Liveness }

func (p claimProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(20, 0)}, p.state, nil
}

func TestFenceRecordAndCreationClaims(t *testing.T) {
	root := t.TempDir()
	closed, record, err := Closed(root)
	if err != nil || closed || record.Generation != 0 {
		t.Fatalf("implicit fence = closed %v generation %d error %v", closed, record.Generation, err)
	}
	record = Record{State: StateClosed, Phase: PhaseStopping, Generation: 1, ChangedAt: "2026-09-07T00:00:00Z", Checkout: root, By: Actor{Verb: "stop", Process: Process{Pid: 10, PidStartedAt: 20}}}
	if err := Write(root, record); err != nil {
		t.Fatal(err)
	}
	if closed, got, err := Closed(root); err != nil || !closed || got.Phase != PhaseStopping {
		t.Fatalf("written fence = closed %v phase %q error %v", closed, got.Phase, err)
	}
	claim, err := Creating(root, "run-launch", 0, identity.Ref{Pid: 10, StartedAtSec: 20})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := Claims(root, 1)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claims = %d, %v", len(claims), err)
	}
	if removed, err := RemoveStale(claims[0], claimProber{state: identity.Alive}); err != nil || removed {
		t.Fatalf("live claim removed = %v, %v", removed, err)
	}
	if err := claim.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claim.Path()); !os.IsNotExist(err) {
		t.Fatalf("closed claim still exists: %v", err)
	}
}

func TestCloseClaimRejectsArbitraryPath(t *testing.T) {
	path := t.TempDir() + "/victim.json"
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CloseClaim(path); err == nil {
		t.Fatal("arbitrary path was accepted")
	}
}

func TestFenceLockContentionAndDeadHolderTakeover(t *testing.T) {
	root := t.TempDir()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("self identity: %v %s", err, state)
	}
	held, err := Acquire(root, "stop", exact.Ref(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(root, "arm", exact.Ref(), 1); err == nil {
		t.Fatal("a live lock holder did not refuse its contender")
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}

	dead := identity.Ref{Pid: 2_000_000_000, StartedAtSec: 1}
	stale, err := Acquire(root, "stop", dead, 1)
	if err != nil {
		t.Fatal(err)
	}
	taken, err := Acquire(root, "arm", exact.Ref(), 1)
	if err != nil {
		t.Fatalf("dead holder was not taken over: %v", err)
	}
	if err := stale.Release(); err == nil {
		t.Fatal("the stale handle released its successor's lock")
	}
	if err := taken.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedDescriptionsNameCompletedIncompleteAndUnfinishedRemedies(t *testing.T) {
	base := Record{State: StateClosed, Generation: 3, ChangedAt: "2026-09-08T02:00:00Z", Checkout: "/checkout", By: Actor{Verb: "stop", Process: Process{Pid: 72}}}
	completed := base
	completed.Phase = PhaseStopped
	if got, err := ClosedDescription(completed, "/fallback"); err != nil || got != "the metasystem is stopped for /checkout since 2026-09-08T02:00:00Z, by stop pid 72" {
		t.Fatalf("completed description = %q, %v", got, err)
	}
	assertClosedRendering(t, completed, "metasystem arm --repo /checkout", "HEALTH STOPPED ")
	incomplete := base
	incomplete.Phase = PhaseStopIncomplete
	incomplete.NotStopped = []Survivor{{Component: "run", ID: "one", Reason: "survived"}}
	if got, err := ClosedDescription(incomplete, "/fallback"); err != nil || got != "stop incomplete for /checkout since 2026-09-08T02:00:00Z by stop pid 72; 1 unresolved entries from the last stop" {
		t.Fatalf("incomplete description = %q, %v", got, err)
	}
	assertClosedRendering(t, incomplete, "metasystem stop --repo /checkout", "HEALTH STOP INCOMPLETE ")
	unfinished := base
	unfinished.Phase = PhaseStopping
	if got, err := ClosedDescription(unfinished, "/fallback"); err != nil || got != "stop unfinished for /checkout since 2026-09-08T02:00:00Z by stop pid 72" {
		t.Fatalf("unfinished description = %q, %v", got, err)
	}
	assertClosedRendering(t, unfinished, "metasystem stop --repo /checkout", "HEALTH STOP UNFINISHED ")

	inconsistent := completed
	inconsistent.NotStopped = []Survivor{{Component: "run", ID: "one", Reason: "survived"}}
	assertClosedRendering(t, inconsistent, "metasystem stop --repo /checkout", "HEALTH STOP INCOMPLETE ")
}

func assertClosedRendering(t *testing.T, record Record, wantCommand, wantHealth string) {
	t.Helper()
	command, commandErr := ClosedCommand(record, "/fallback")
	health, healthErr := HealthPrefix(record)
	if commandErr != nil || healthErr != nil || command != wantCommand || health != wantHealth {
		t.Fatalf("closed rendering command=%q error=%v health=%q error=%v, want %q and %q", command, commandErr, health, healthErr, wantCommand, wantHealth)
	}
}

func TestClosedFenceRenderersRefuseAnOpenRecord(t *testing.T) {
	open := Record{State: StateOpen, Phase: PhaseArmed, Generation: 9}
	if description, err := ClosedDescription(open, "/checkout"); err == nil || description != "" || !strings.Contains(err.Error(), "requires a closed record") {
		t.Fatalf("open description = %q, %v", description, err)
	}
	if command, err := ClosedCommand(open, "/checkout"); err == nil || command != "" || !strings.Contains(err.Error(), "requires a closed record") {
		t.Fatalf("open command = %q, %v", command, err)
	}
	if health, err := HealthPrefix(open); err == nil || health != "" || !strings.Contains(err.Error(), "requires a closed record") {
		t.Fatalf("open health prefix = %q, %v", health, err)
	}
}
