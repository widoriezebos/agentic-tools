package batch

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const dispatchB, dispatchC = "01j5x00000000000000000ba02", "01j5x00000000000000000ba03"

func hostSample(load float64) proofrun.LoadSample {
	return proofrun.LoadSample{Sample: hostload.Sample{Cores: 16, Load1m: load, Available: true}, OverlapKnown: true}
}

// dispatchBed seals one expired single-member batch per id behind a launcher
// that blocks until the test releases that batch: the production Launch shape.
func dispatchBed(t *testing.T, state string, ids ...string) (*ownerBed, map[string]chan struct{}, chan Dispatch) {
	t.Helper()
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	var bed *ownerBed
	release, entered := map[string]chan struct{}{}, make(chan Dispatch, len(ids)+1)
	for i, id := range ids {
		record := ownerRecord(id, state, now.Add(-time.Minute))
		goal := "goal-" + string(rune('a'+i))
		record.Units[0].GoalID, record.Units[0].Chain, record.History[0].Detail = goal, "chain-"+goal, goal+" joined"
		if bed == nil {
			bed = newOwnerBed(t, record, now)
		} else {
			must(t, bed.store.Create(record))
		}
		release[id] = make(chan struct{})
	}
	bed.sample = hostSample(1)
	minted := 0
	bed.owner.mint = func() (string, error) { minted++; return fmt.Sprintf("token-%d", minted), nil }
	bed.owner.admission = func(proofrun.LoadSample) proofrun.AdmissionCap { return proofrun.AdmissionCap{Max: 2} }
	bed.owner.launch = func(request Dispatch) error {
		entered <- request
		<-release[request.ID]
		return bed.store.Update(request.ID, func(record *Record) error { record.Transition(StateLanded, now, "land", "owner", ""); return nil })
	}
	return bed, release, entered
}

// settle applies the completion of every run the owner started, as its loop
// does; a re-attached run completes at a tick, not on the channel.
func (owner *Owner) settle() {
	for slices.ContainsFunc(slices.Collect(maps.Values(owner.inflight)), func(run *proofRun) bool { return !run.attached }) {
		owner.Complete(<-owner.completions)
	}
}

func lockHeld(bed *ownerBed, id string) bool {
	_, err := os.Stat(filepath.Join(bed.lockDir, "batch-"+id, "owner"))
	return err == nil
}

func lastVerb(t *testing.T, store Store, id string) HistoryEntry {
	t.Helper()
	record, err := store.Load(id)
	must(t, err)
	return record.History[len(record.History)-1]
}

func TestOwnerOverlapsProofsThroughItsRealDispatchPath(t *testing.T) {
	t.Parallel()
	bed, release, entered := dispatchBed(t, StateSealed, testBatchID, dispatchB, dispatchC)
	waiting, stop, done := make(chan struct{}, 4), make(chan struct{}), make(chan struct{})
	bed.owner.after = func(time.Duration) <-chan time.Time { waiting <- struct{}{}; return make(chan time.Time) }
	go func() { bed.owner.Loop(time.Minute, nil, stop); close(done) }()
	first, second := <-entered, <-entered
	<-waiting
	inFlight := map[string]bool{first.ID: true, second.ID: true}
	capped, err := bed.store.Load(dispatchC)
	must(t, err)
	last := capped.History[len(capped.History)-1]
	witness(t, inFlight[testBatchID] && inFlight[dispatchB] && first.Token != "" && first.Token != second.Token,
		"in flight at once: %+v %+v", first, second)
	witness(t, capped.State == StateSealed && last.Verb == "cap" && strings.Contains(last.Detail, "load 1.00") && strings.Contains(last.Detail, "ceiling 2"),
		"third batch state=%s history=%+v", capped.State, capped.History)
	witness(t, lockHeld(bed, testBatchID) && lockHeld(bed, dispatchB) && !lockHeld(bed, dispatchC), "locks while in flight")
	close(release[testBatchID])
	third := <-entered
	<-waiting
	witness(t, third.ID == dispatchC && !lockHeld(bed, testBatchID) && lockHeld(bed, dispatchB) && lockHeld(bed, dispatchC),
		"after A's completion: third=%+v A lock=%v", third, lockHeld(bed, testBatchID))
	close(release[dispatchB])
	close(release[dispatchC])
	close(stop)
	<-done
	bed.owner.settle()
}

func TestSecondProofQueuesUnderLoadAndDispatchesWhenIdle(t *testing.T) {
	t.Parallel()
	t.Run("host load", func(t *testing.T) {
		t.Parallel()
		bed, release, entered := dispatchBed(t, StateSealed, testBatchID, dispatchB)
		must(t, bed.owner.Tick(testBatchID))
		<-entered
		bed.sample = hostSample(14)
		must(t, bed.owner.Tick(dispatchB))
		last := lastVerb(t, bed.store, dispatchB)
		witness(t, bed.owner.inflight[dispatchB] == nil && last.Verb == "cap" && strings.Contains(last.Detail, "load 14.00"),
			"second proof under load: inflight=%v history=%+v", bed.owner.inflight[dispatchB] != nil, last)
		bed.sample = hostSample(1)
		must(t, bed.owner.Tick(dispatchB))
		witness(t, bed.owner.inflight[dispatchB] != nil, "second proof not dispatched on an idle host")
		idle := <-entered
		witness(t, idle.ID == dispatchB && idle.Runner == "host", "second proof when idle: %+v", idle)
		close(release[testBatchID])
		close(release[dispatchB])
		bed.owner.settle()
	})
	t.Run("a separate runner is its own capacity", func(t *testing.T) {
		t.Parallel()
		bed, release, entered := dispatchBed(t, StateSealed, testBatchID, dispatchB, dispatchC)
		bed.sample = hostSample(14)
		bed.owner.runners = func(sample proofrun.LoadSample, admission proofrun.AdmissionCap) []RunnerCapacity {
			return []RunnerCapacity{hostRunner(sample, admission), {Runner: "vm", Cores: 8, LoadKnown: true, Ceiling: 1}}
		}
		must(t, bed.owner.Tick(testBatchID))
		onHost := <-entered
		must(t, bed.owner.Tick(dispatchB))
		witness(t, bed.owner.inflight[dispatchB] != nil, "the vm's own capacity did not admit B beside the busy host")
		onVM := <-entered
		must(t, bed.owner.Tick(dispatchC))
		witness(t, onHost.Runner == "host" && onVM.ID == dispatchB && onVM.Runner == "vm" && bed.owner.inflight[dispatchC] == nil &&
			lastVerb(t, bed.store, dispatchC).Verb == "cap", "host=%+v vm=%+v third in flight=%v", onHost, onVM, bed.owner.inflight[dispatchC] != nil)
		close(release[testBatchID])
		bed.owner.Complete(<-bed.owner.completions)
		must(t, bed.owner.Tick(dispatchC))
		witness(t, bed.owner.inflight[dispatchC] != nil, "the vm's run was counted against the host")
		third := <-entered
		witness(t, third.ID == dispatchC && third.Runner == "host", "the vm's run counted against the host: %+v", third)
		close(release[dispatchB])
		close(release[dispatchC])
		bed.owner.settle()
	})
}

func TestStaleCompletionCannotFinishAReopenedProof(t *testing.T) {
	t.Parallel()
	bed, release, entered := dispatchBed(t, StateSealed, testBatchID)
	reported := []string{}
	bed.owner.report = func(id string, err error) { reported = append(reported, id+": "+err.Error()) }
	plan := testpolicy.Plan{SelectedGroups: []string{"group"}}
	bed.owner.launch = func(request Dispatch) error {
		if _, err := RequireProofPlan(bed.store, request.ID, "owner", request.Window, request.Token, request.Sample, plan, bed.now); err != nil {
			return err
		}
		entered <- request
		<-release[request.ID]
		return FinishProof(bed.store, request.ID, "owner", request.Token, proofrun.TestResult{AttemptID: "stale", Delivery: proofrun.DeliveryJudgment{Sufficient: true}}, nil, bed.now)
	}
	must(t, bed.owner.Tick(testBatchID))
	old := <-entered
	reassembly := bed.store.WithReassembly(func(string, []Unit) ([]string, error) { return []string{"tip-2"}, nil }, nil, nil)
	must(t, ReassembleSurvivors(reassembly, testBatchID, "owner", bed.now))
	must(t, bed.store.Update(testBatchID, func(record *Record) error {
		record.Transition(StateSealed, bed.now, "seal", "owner", "")
		return nil
	}))
	_, err := RequireProofPlan(bed.store, testBatchID, "owner", "expired", "token-new", proofrun.LoadSample{}, plan, bed.now)
	must(t, err)
	close(release[testBatchID])
	bed.owner.settle()
	record, err := bed.store.Load(testBatchID)
	must(t, err)
	witness(t, old.Token != "token-new" && record.State == StateProving && record.Proof.Status == "planned" && record.Proof.Token == "token-new" &&
		record.Proof.AttemptID == "" && record.TipTree == "tip-2", "replacement plan touched: %+v", record.Proof)
	witness(t, len(reported) == 1 && strings.Contains(reported[0], "BATCH_PROOF_STALE_COMPLETION"), "reports=%q", reported)
	witness(t, FinishProof(bed.store, testBatchID, "owner", "", proofrun.TestResult{}, nil, bed.now) != nil, "an untokened completion finished a tokened plan")
}

func TestOwnerRestartReattachesOrRefusesAPlannedProof(t *testing.T) {
	t.Parallel()
	bed, release, entered := dispatchBed(t, StateProving, testBatchID, dispatchB)
	for _, id := range []string{testBatchID, dispatchB} {
		must(t, bed.store.Update(id, func(record *Record) error {
			record.Proof = &Proof{Status: "planned", Tree: "tip", Token: "token-" + id}
			return nil
		}))
	}
	probes := map[string]RunProbe{testBatchID: {State: RunLive}, dispatchB: {State: RunDead, Detail: "launcher pid 41 dead"}}
	bed.owner.probeRun = func(id string, record Record) (RunProbe, error) { return probes[id], nil }
	bed.owner.Resume()
	attached := bed.owner.inflight[testBatchID]
	live, err := bed.store.Load(testBatchID)
	must(t, err)
	witness(t, attached != nil && attached.token == "token-"+testBatchID && live.State == StateProving && lockHeld(bed, testBatchID),
		"live launcher: run=%+v state=%s", attached, live.State)
	relaunched := <-entered
	dead, err := bed.store.Load(dispatchB)
	must(t, err)
	refused := false
	for _, entry := range dead.History {
		refused = refused || entry.Verb == "prove-refused" && strings.Contains(entry.Detail, "launcher-died")
	}
	witness(t, relaunched.ID == dispatchB && refused && dead.State == StateSealed && dead.Proof.Status == "launcher-died",
		"dead launcher: dispatch=%+v record=%+v", relaunched, dead)
	close(release[dispatchB])
	bed.owner.Complete(<-bed.owner.completions)
	probes[testBatchID] = RunProbe{State: RunTerminal, Result: proofrun.TestResult{AttemptID: "attempt-restart", Delivery: proofrun.DeliveryJudgment{Sufficient: true}}}
	must(t, bed.owner.Tick(testBatchID))
	finished, err := bed.store.Load(testBatchID)
	must(t, err)
	witness(t, finished.Proof.Status == "green" && finished.Proof.AttemptID == "attempt-restart" && finished.State == StateLanding,
		"re-attached run did not complete from its result: %+v", finished.Proof)
	close(release[testBatchID])
	bed.owner.settle()
}
