package batch

import (
	"strings"
	"testing"
	"time"
)

// hostProving is the host's proving flock as the owner sees it: held by the
// proof child that runs, never by the owner, which only probes it.
type hostProving struct {
	held   bool
	probes int
}

func (p *hostProving) probe() (string, bool, error) {
	p.probes++
	if p.held {
		return "pid 99", true, nil
	}
	return "", false, nil
}

func provingWaits(record Record) int {
	waits := 0
	for _, entry := range record.History {
		if entry.Verb == ProvingWaitVerb {
			waits++
		}
	}
	return waits
}

// One batch proves at a time on the host (U12): with A's proof child
// holding the flock, B's start is held without a dispatch; B stays open,
// takes a late join and records its wait once; when A's child ends and the
// flock is free, B's next tick starts it with both members. The owner never
// holds the flock itself.
func TestOwnerSecondBatchWaitsForProvingLock(t *testing.T) {
	t.Parallel()
	bed, release, entered := dispatchBed(t, StateSealed, testBatchID)
	open := ownerRecord(dispatchB, StateOpen, bed.now.Add(-2*time.Minute))
	open.Units[0].GoalID, open.Units[0].Chain, open.History[0].Detail = "goal-b", "chain-goal-b", "goal-b joined"
	must(t, bed.store.Create(open))
	release[dispatchB] = make(chan struct{})
	proving := &hostProving{}
	bed.owner.proving = proving.probe

	must(t, bed.owner.Tick(testBatchID))
	first := <-entered
	witness(t, first.ID == testBatchID && proving.probes > 0 && !proving.held, "A dispatched=%+v probes=%d; the owner took the flock itself", first, proving.probes)
	proving.held = true // A's proof child takes the flock for its life

	must(t, bed.owner.Tick(dispatchB))
	// A late join lands in the waiting batch; the wait is still one wait.
	must(t, bed.store.Update(dispatchB, func(record *Record) error {
		unit := record.Units[0]
		unit.GoalID, unit.Chain = "goal-c", "chain-goal-c"
		record.Units = append(record.Units, unit)
		record.History = append(record.History, HistoryEntry{At: bed.now.Format(time.RFC3339Nano), Verb: "join", From: StateOpen, To: StateOpen, Detail: "goal-c joined"})
		return nil
	}))
	must(t, bed.owner.Tick(dispatchB))
	waited, err := bed.store.Load(dispatchB)
	must(t, err)
	entry, waiting := ProvingWait(waited)
	witness(t, bed.owner.inflight[dispatchB] == nil && waited.State == StateOpen && provingWaits(waited) == 1 && waiting && strings.Contains(entry.Detail, "pid 99") && !lockHeld(bed, dispatchB),
		"B while A proves: inflight=%v state=%s waits=%d waiting=%v entry=%+v lock=%v", bed.owner.inflight[dispatchB] != nil, waited.State, provingWaits(waited), waiting, entry, lockHeld(bed, dispatchB))

	close(release[testBatchID])
	bed.owner.Complete(<-bed.owner.completions)
	proving.held = false // A's child ended; the kernel released its flock
	must(t, bed.owner.Tick(dispatchB))
	second := <-entered
	started, err := bed.store.Load(dispatchB)
	must(t, err)
	witness(t, second.ID == dispatchB && len(started.Units) == 2, "B after A: %+v units=%d", second, len(started.Units))
	close(release[dispatchB])
	bed.owner.settle()
}

// F-2: an owner restarted while a proof child still runs holds nothing and
// remembers nothing, yet the older batch that is ready waits: the running
// child holds the flock, not the dead owner.
func TestRestartedOwnerWaitsForTheRunningProof(t *testing.T) {
	t.Parallel()
	bed, release, entered := dispatchBed(t, StateSealed, testBatchID)
	proving := &hostProving{held: true} // another batch's proof child, launched by the previous owner
	bed.owner.proving = proving.probe
	must(t, bed.owner.Tick(testBatchID))
	witness(t, bed.owner.inflight[testBatchID] == nil && lastVerb(t, bed.store, testBatchID).Verb == ProvingWaitVerb, "the older batch started beside a running proof")
	proving.held = false
	must(t, bed.owner.Tick(testBatchID))
	started := <-entered
	witness(t, started.ID == testBatchID, "the older batch after the proof ended: %+v", started)
	close(release[testBatchID])
	bed.owner.settle()
}

// A diagnosis is a proof too: it waits while a proof holds the flock; a
// landing run does not wait behind another batch's proof.
func TestOwnerDiagnosisTakesProvingLockLandingDoesNot(t *testing.T) {
	t.Parallel()
	bed, release, entered := dispatchBed(t, StateDiagnosing, testBatchID)
	landing := ownerRecord(dispatchB, StateLanding, bed.now.Add(-2*time.Minute))
	must(t, bed.store.Create(landing))
	release[dispatchB] = make(chan struct{})
	proving := &hostProving{held: true}
	bed.owner.proving = proving.probe

	must(t, bed.owner.Tick(testBatchID))
	witness(t, bed.owner.inflight[testBatchID] == nil && lastVerb(t, bed.store, testBatchID).Verb == ProvingWaitVerb, "diagnosis dispatched beside a held flock")
	must(t, bed.owner.Tick(dispatchB))
	landed := <-entered
	witness(t, landed.ID == dispatchB, "landing run waited behind the flock: %+v", landed)
	close(release[dispatchB])
	bed.owner.settle()

	proving.held = false
	must(t, bed.owner.Tick(testBatchID))
	diagnosed := <-entered
	witness(t, diagnosed.ID == testBatchID, "diagnosis after the flock freed: %+v", diagnosed)
	close(release[testBatchID])
	bed.owner.settle()
}

// F-3: the held-trunk-red clearing diagnostic runs in the owner's tick; it
// never starts while another batch's proof holds the flock.
func TestHeldTrunkRedClearingWaitsForTheRunningProof(t *testing.T) {
	t.Parallel()
	now := time.Unix(10, 0)
	record := ownerRecord(testBatchID, StateHeldTrunkRed, now.Add(-time.Minute))
	record.BaseTree = "old"
	record.TrunkRed.Red = TrunkRed{BaseTree: "old", Groups: []RedGroup{{ID: "fast"}}}
	record.TrunkRed.Entries = []EntryRef{{ID: "entry", Group: "fast"}}
	bed := newOwnerBed(t, record, now)
	bed.tree = "moved"
	diagnostics := 0
	bed.owner.runDiagnostic = func(string, DiagnosticRequest, Claim) (DiagnosticResult, error) {
		diagnostics++
		return DiagnosticResult{AttemptID: "diagnostic"}, nil
	}
	proving := &hostProving{held: true}
	bed.owner.proving = proving.probe
	must(t, bed.owner.Tick(testBatchID))
	held, err := bed.store.Load(testBatchID)
	must(t, err)
	_, waiting := ProvingWait(held)
	witness(t, diagnostics == 0 && waiting, "clearing diagnostic beside a running proof: diagnostics=%d waiting=%v", diagnostics, waiting)
}
