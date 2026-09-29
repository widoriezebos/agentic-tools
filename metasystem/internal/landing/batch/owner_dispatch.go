package batch

import (
	"errors"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// Dispatch is one run the owner starts: the batch, the window and sample it
// was admitted under, the plan token its completion must carry, and the
// runner whose capacity admitted it.
type Dispatch struct {
	ID, Window, Token, Runner string
	Sample                    proofrun.LoadSample
}

// Completion is a finished run as it returns to the owner's loop.
type Completion struct {
	ID, Token string
	Err       error
}

// What the proof store says about the launcher of a planned proof the owner
// finds without a run of its own (after a restart).
const (
	RunLive     = "live"
	RunTerminal = "terminal"
	RunDead     = "dead"
)

// RunProbe is that answer: a terminal run carries its result file's result.
type RunProbe struct {
	State, Detail string
	Result        proofrun.TestResult
	Err           error
}

type proofRun struct {
	token, runner string
	started       time.Time
	lock          *proofLock
	attached      bool
}

// RunnerCapacity is one proof runner's own capacity: the CPUs it gives
// proofs, its sampled load, the proofs its census sees that are not this
// owner's, and its CPU-derived ceiling. The host runner reports the local
// cores and load; any other runner reports its own, so no core counts twice.
type RunnerCapacity struct {
	Runner      string
	Cores       int
	Load        float64
	LoadKnown   bool
	Overlapping int
	Ceiling     int
}

// hostRunner reads the host from the owner's own census sample. The census
// already sees the slots this owner's running proofs hold (OwnHost); admits
// counts those runs itself, so they are taken out of the overlap here and
// each own run counts once.
func hostRunner(sample proofrun.LoadSample, admission proofrun.AdmissionCap) RunnerCapacity {
	return RunnerCapacity{Runner: "host", Cores: sample.Cores, Load: sample.Load1m, LoadKnown: sample.Available,
		Overlapping: max(sample.OverlappingHost-sample.OwnHost, 0), Ceiling: admission.Max}
}

// admits reports whether one more proof fits beside own runs of this owner on
// the runner. The first proof on an idle runner always fits; a further one
// fits only under the ceiling and when the load leaves a proof's share of the
// cores free. A proof's share is cores over the ceiling; the committed load is
// at least the running proofs' shares, since a minute's load average lags a
// launch that just started.
func (runner RunnerCapacity) admits(own int) bool {
	running := own + runner.Overlapping
	if runner.Ceiling > 0 && running >= runner.Ceiling {
		return false
	}
	if running == 0 {
		return true
	}
	if !runner.LoadKnown || runner.Cores <= 0 {
		return false
	}
	ceiling := runner.Ceiling
	if ceiling <= 0 {
		derived, _ := proofrun.ResolveAdmissionCap("", runner.Cores)
		ceiling = derived.Max
	}
	share := float64(runner.Cores) / float64(ceiling)
	return max(runner.Load, float64(running)*share)+share <= float64(runner.Cores)
}

func (owner *Owner) ownRuns(runner string) (count int) {
	for _, run := range owner.inflight {
		if run.runner == runner {
			count++
		}
	}
	return count
}

func (owner *Owner) chooseRunner(sample proofrun.LoadSample, admission proofrun.AdmissionCap) (string, bool) {
	for _, runner := range owner.runners(sample, admission) {
		if runner.admits(owner.ownRuns(runner.Runner)) {
			return runner.Runner, true
		}
	}
	return "", false
}

// recordCap leaves a batch no runner has room for where it is, with the
// sample, once per wait; the next tick retries.
func (owner *Owner) recordCap(id string, sample proofrun.LoadSample, admission proofrun.AdmissionCap, at time.Time) error {
	return owner.store.Update(id, func(current *Record) error {
		if n := len(current.History); n > 0 && current.History[n-1].Verb == "cap" {
			return nil
		}
		current.Transition(current.State, at, "cap", owner.actor,
			fmt.Sprintf("%d own run(s) in flight, ceiling %d: %s", len(owner.inflight), admission.Max, sample.Describe()))
		return nil
	})
}

func (owner *Owner) batchLock(id string) *proofLock {
	lock := owner.locks[id]
	if lock == nil {
		lock = owner.lock(id)
		owner.locks[id] = lock
	}
	return lock
}

// dispatch starts a run that holds the batch's lock until its completion is
// applied; the tick that decided it returns at once.
func (owner *Owner) dispatch(lock *proofLock, request Dispatch) {
	owner.inflight[request.ID] = &proofRun{token: request.Token, runner: request.Runner, started: owner.now(), lock: lock}
	go func() {
		owner.completions <- Completion{ID: request.ID, Token: request.Token, Err: owner.launch(request)}
	}()
}

// Completions is where the owner's runs finish; a loop selecting on it hands
// each to Complete.
func (owner *Owner) Completions() <-chan Completion {
	if owner == nil {
		return nil
	}
	return owner.completions
}

// Complete applies one finished run: its lock is released and its error, a
// stale completion's refusal among them, is reported once.
func (owner *Owner) Complete(done Completion) {
	if err := owner.complete(done); err != nil {
		owner.report(done.ID, err)
	}
}

func (owner *Owner) complete(done Completion) error {
	run := owner.inflight[done.ID]
	if run == nil || run.attached || run.token != done.Token {
		return errors.Join(done.Err, fmt.Errorf("BATCH_PROOF_STALE_COMPLETION: batch %s has no run for this completion; discarded", done.ID))
	}
	delete(owner.inflight, done.ID)
	return errors.Join(done.Err, owner.stampRunner(done.ID, done.Token, run.runner), run.lock.release())
}

// stampRunner records on the batch's proof the runner that ran it, once,
// when the proof belongs to the completed run.
func (owner *Owner) stampRunner(id, token, runner string) error {
	record, err := owner.store.Load(id)
	if err != nil || record.Proof == nil || record.Proof.Token != token || record.Proof.Runner != "" || runner == "" {
		return nil
	}
	return owner.store.Update(id, func(current *Record) error {
		if current.Proof != nil && current.Proof.Token == token && current.Proof.Runner == "" {
			current.Proof.Runner = runner
		}
		return nil
	})
}

func (owner *Owner) drain() {
	for {
		select {
		case done := <-owner.completions:
			owner.Complete(done)
		default:
			return
		}
	}
}

// restart binds a planned proof found without a run of this owner to its
// launcher: a live one is re-attached and completes from its result file at
// a later tick; a dead one is refused back to sealed for the start rule.
func (owner *Owner) restart(id string) error {
	record, err := owner.store.Load(id)
	if err != nil {
		return err
	}
	run := owner.inflight[id]
	if record.State != StateProving || record.Proof == nil || record.Proof.Status != "planned" {
		if run == nil {
			return nil
		}
		delete(owner.inflight, id)
		return run.lock.release()
	}
	probe, err := owner.probeRun(id, record)
	if err != nil {
		return err
	}
	token, at := record.Proof.Token, owner.now()
	switch probe.State {
	case RunLive:
		if run != nil {
			return nil
		}
		lock := owner.batchLock(id)
		if polled, err := lock.poll(); err != nil || polled != lockAcquired {
			return err
		}
		owner.inflight[id] = &proofRun{token: token, runner: "host", started: at, lock: lock, attached: true}
		return nil
	case RunTerminal:
		err = FinishProof(owner.store, id, owner.actor, token, probe.Result, probe.Err, at)
	default:
		err = RefuseProofAdmission(owner.store, id, owner.actor, token, "launcher-died", "launcher-died: "+probe.Detail, at)
	}
	if run != nil {
		delete(owner.inflight, id)
		err = errors.Join(err, run.lock.release())
	}
	return err
}
