package batch

import (
	"strings"
	"testing"
	"time"
)

// hostProving is the host's proving flock as the owner sees it: one holder.
type hostProving struct{ held bool }

func (p *hostProving) take() (func() error, string, error) {
	if p.held {
		return nil, "pid 99", nil
	}
	p.held = true
	return func() error { p.held = false; return nil }, "", nil
}

// One batch proves at a time on the host (U12): with A proving, B's start is
// held without a dispatch; B stays open and takes a late join; when A's run
// completes and frees the flock, B's next tick starts it with both members.
func TestOwnerSecondBatchWaitsForProvingLock(t *testing.T) {
	t.Parallel()
	bed, release, entered := dispatchBed(t, StateSealed, testBatchID)
	open := ownerRecord(dispatchB, StateOpen, bed.now.Add(-2*time.Minute))
	open.Units[0].GoalID, open.Units[0].Chain, open.History[0].Detail = "goal-b", "chain-goal-b", "goal-b joined"
	must(t, bed.store.Create(open))
	release[dispatchB] = make(chan struct{})
	proving := &hostProving{}
	bed.owner.proving = proving.take

	must(t, bed.owner.Tick(testBatchID))
	first := <-entered
	witness(t, first.ID == testBatchID && proving.held, "A dispatched=%+v flock held=%v", first, proving.held)

	must(t, bed.owner.Tick(dispatchB))
	must(t, bed.owner.Tick(dispatchB))
	waited, err := bed.store.Load(dispatchB)
	must(t, err)
	waits := 0
	for _, entry := range waited.History {
		if entry.Verb == ProvingWaitVerb {
			waits++
		}
	}
	last := waited.History[len(waited.History)-1]
	witness(t, bed.owner.inflight[dispatchB] == nil && waited.State == StateOpen && waits == 1 && strings.Contains(last.Detail, "pid 99") && !lockHeld(bed, dispatchB),
		"B while A proves: inflight=%v state=%s waits=%d last=%+v lock=%v", bed.owner.inflight[dispatchB] != nil, waited.State, waits, last, lockHeld(bed, dispatchB))

	// A late join lands in the waiting batch.
	must(t, bed.store.Update(dispatchB, func(record *Record) error {
		unit := record.Units[0]
		unit.GoalID, unit.Chain = "goal-c", "chain-goal-c"
		record.Units = append(record.Units, unit)
		record.History = append(record.History, HistoryEntry{At: bed.now.Format(time.RFC3339Nano), Verb: "join", From: StateOpen, To: StateOpen, Detail: "goal-c joined"})
		return nil
	}))

	close(release[testBatchID])
	bed.owner.Complete(<-bed.owner.completions)
	witness(t, !proving.held, "A's completion did not free the flock")
	must(t, bed.owner.Tick(dispatchB))
	second := <-entered
	started, err := bed.store.Load(dispatchB)
	must(t, err)
	witness(t, second.ID == dispatchB && proving.held && len(started.Units) == 2, "B after A: %+v held=%v units=%d", second, proving.held, len(started.Units))
	close(release[dispatchB])
	bed.owner.settle()
	witness(t, !proving.held, "B's completion did not free the flock")
}

// A diagnosis is a proof too: it waits for the flock and holds it; a landing
// run does not wait behind another batch's proof.
func TestOwnerDiagnosisTakesProvingLockLandingDoesNot(t *testing.T) {
	t.Parallel()
	bed, release, entered := dispatchBed(t, StateDiagnosing, testBatchID)
	landing := ownerRecord(dispatchB, StateLanding, bed.now.Add(-2*time.Minute))
	must(t, bed.store.Create(landing))
	release[dispatchB] = make(chan struct{})
	proving := &hostProving{held: true}
	bed.owner.proving = proving.take

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
	witness(t, diagnosed.ID == testBatchID && proving.held, "diagnosis after the flock freed: %+v held=%v", diagnosed, proving.held)
	close(release[testBatchID])
	bed.owner.settle()
	witness(t, !proving.held, "the diagnosis's completion did not free the flock")
}
