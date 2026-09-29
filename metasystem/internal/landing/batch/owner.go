package batch

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

type ownerSeams struct {
	now                 func() time.Time
	fetchTree           func() (string, error)
	readClaim           func(string, string, string, string) (Claim, error)
	resumeJoinAdmission JoinAdmissionRun
	returns             ReturnSeams
	rebind              func(string, string) error
	mint                func() (string, error)
	logRed              func(string, TrunkRedRecordOutcome)
	baseCommit          func(string) (string, error)
	runDiagnostic       func(string, DiagnosticRequest, Claim) (DiagnosticResult, error)
	descendsFrom        func(string, string) (bool, error)
	sample              func() proofrun.LoadSample
	admission           func(proofrun.LoadSample) proofrun.AdmissionCap
	launch              func(Dispatch) error
	probeRun            func(string, Record) (RunProbe, error)
	runners             func(proofrun.LoadSample, proofrun.AdmissionCap) []RunnerCapacity
	lock                func(string) *proofLock
	after               func(time.Duration) <-chan time.Time
	report              func(string, error)
	pipeline            PipelineSource
	logWait             func(id, line string)
	location            *time.Location
	glob                func(string) ([]string, error)
	helmActive          func(string) bool
	baseMove            func(string, string) (BaseMove, error)
	early               EarlySeams
	earlyRuns           map[string]string
	earlyDone           chan EarlyCompletion
	locks               map[string]*proofLock
	held                map[string]HeldBatch
	inflight            map[string]*proofRun
	completions         chan Completion
	standing            bool
	facts               PipelineFacts
	factsAt             time.Time
	decided             map[string]decidedAt
}

// decidedAt is the last start decision of a batch and when it was made.
type decidedAt struct {
	at       time.Time
	decision Decision
}

// HeldBatch is one batch the owner held on its last pass because a unit's
// seat is at the helm: the seat, the queue registration withdrawn for it
// (base name, empty when there was none), and whether the hold began on that
// pass, so the caller prints once per hold.
type HeldBatch struct {
	ID, Seat, Entry string
	New             bool
}

// OwnerOptions names every authority and side effect used by the durable
// batch owner. Callers must provide an injected clock; tests can substitute
// every external edge without weakening the production constructor.
type OwnerOptions struct {
	Store               Store
	Settings            config.BatchLanding
	Actor               string
	PID                 int64
	LockDir             string
	QueueDir            string
	Now                 func() time.Time
	FetchTree           func() (string, error)
	ReadClaim           func(string, string, string, string) (Claim, error)
	ResumeJoinAdmission JoinAdmissionRun
	Returns             ReturnSeams
	Rebind              func(string, string) error
	Mint                func() (string, error)
	LogRed              func(string, TrunkRedRecordOutcome)
	BaseCommit          func(string) (string, error)
	RunDiagnostic       func(string, DiagnosticRequest, Claim) (DiagnosticResult, error)
	DescendsFrom        func(string, string) (bool, error)
	Sample              func() proofrun.LoadSample
	Admission           func(proofrun.LoadSample) proofrun.AdmissionCap
	Launch              func(Dispatch) error
	ProbeRun            func(string, Record) (RunProbe, error) // a planned proof's launcher, read from the proof store after a restart
	After               func(time.Duration) <-chan time.Time
	Report              func(string, error)
	// Pipeline reads the host board a start is decided from (D14, R22).
	Pipeline PipelineSource
	// LogWait receives a batch's wait or start line once per change; nil
	// logs nothing.
	LogWait func(id, line string)
	// Location renders times for people; nil is the local zone.
	Location *time.Location
	Glob     func(string) ([]string, error)
	// HelmActive reports whether a unit's seat is at the helm; nil holds nothing.
	HelmActive func(root string) bool
	// BaseMove reads what moved main between two base trees; nil leaves a
	// batch on the base it was sealed on until its landing decides.
	BaseMove func(fromTree, toTree string) (BaseMove, error)
	// Early are the acts that use a wait (D14, R27).
	Early EarlySeams
}

type Owner struct {
	store    Store
	settings config.BatchLanding
	actor    string
	ownerSeams
}

// NewOwner constructs the only process allowed to advance batch records.
func NewOwner(options OwnerOptions) (*Owner, error) {
	if options.Now == nil || options.FetchTree == nil || options.ReadClaim == nil || options.Rebind == nil || options.Mint == nil || options.LogRed == nil ||
		options.BaseCommit == nil || options.RunDiagnostic == nil || options.DescendsFrom == nil ||
		options.Sample == nil || options.Admission == nil || options.Launch == nil || options.ProbeRun == nil || options.After == nil || options.Report == nil ||
		options.Pipeline == nil || options.Early.Cheap == nil || options.Early.Prove == nil || options.Early.Budget == nil || options.Early.Adapter == nil {
		return nil, fmt.Errorf("construct batch owner: every owner seam is required")
	}
	if options.PID < 1 {
		return nil, fmt.Errorf("construct batch owner: pid must be positive")
	}
	if options.Glob == nil {
		options.Glob = filepath.Glob
	}
	owner := &Owner{store: options.Store, settings: options.Settings, actor: options.Actor}
	owner.ownerSeams = ownerSeams{
		now: options.Now, fetchTree: options.FetchTree, readClaim: options.ReadClaim, resumeJoinAdmission: options.ResumeJoinAdmission,
		returns: options.Returns, rebind: options.Rebind, mint: options.Mint, logRed: options.LogRed,
		baseCommit: options.BaseCommit, runDiagnostic: options.RunDiagnostic, descendsFrom: options.DescendsFrom, sample: options.Sample,
		admission: options.Admission, launch: options.Launch, probeRun: options.ProbeRun, after: options.After,
		report: options.Report, glob: options.Glob, pipeline: options.Pipeline, logWait: options.LogWait, location: options.Location, helmActive: options.HelmActive, baseMove: options.BaseMove, early: options.Early, locks: map[string]*proofLock{}, held: map[string]HeldBatch{},
		inflight: map[string]*proofRun{}, decided: map[string]decidedAt{}, completions: make(chan Completion, 64),
		earlyRuns: map[string]string{}, earlyDone: make(chan EarlyCompletion, 64),
		runners: func(sample proofrun.LoadSample, admission proofrun.AdmissionCap) []RunnerCapacity {
			return []RunnerCapacity{hostRunner(sample, admission)}
		},
	}
	owner.lock = func(id string) *proofLock {
		return newBatchProofLock(options.Store, options.LockDir, options.QueueDir, id, options.PID, options.Now)
	}
	return owner, nil
}

// Tick advances one batch. A batch with a run in flight returns at once; a
// decided run is dispatched and completes on the owner's loop.
func (owner *Owner) Tick(id string) error {
	if run := owner.inflight[id]; run != nil && !run.attached {
		tree, err := owner.fetchTree()
		if err == nil {
			_, err = owner.followMovedBase(id, tree, true)
		}
		return err
	}
	if err := owner.restart(id); err != nil {
		return err
	}
	tree, err := owner.fetchTree()
	if err != nil {
		return err
	}
	at := owner.now()
	if err = ReconcileJoins(owner.store, id, tree, owner.actor, at, owner.readClaim); err != nil {
		return err
	}
	if owner.resumeJoinAdmission != nil {
		record, loadErr := owner.store.Load(id)
		if loadErr != nil {
			return loadErr
		}
		for _, unit := range record.Units {
			if unit.State == UnitJoining && unit.Admission != nil {
				if err := ResumeJoinAdmission(owner.store, id, unit.GoalID, owner.actor, at, owner.resumeJoinAdmission); err != nil {
					return err
				}
			}
		}
	}
	if err = ReturnUnits(owner.store, id, tree, owner.actor, at, owner.returns); err != nil {
		return err
	}
	if err = owner.rebind(id, tree); err != nil {
		return err
	}
	if moved, err := owner.followMovedBase(id, tree, false); err != nil || moved {
		return errors.Join(err, owner.release(id))
	}
	record, err := owner.store.Load(id)
	if err != nil {
		return err
	}
	if record.State == StateLanding || record.State == StateLanded {
		open, clearErr := owner.clearGreenTipTrunkRed(id, record, at)
		if clearErr != nil {
			owner.report(id, fmt.Errorf("clear green tip trunk-red entries: %w", clearErr))
		} else if record.State == StateLanding && len(open) != 0 {
			opid, mintErr := owner.mint()
			if mintErr != nil {
				return mintErr
			}
			baseCommit := ""
			if record.Proof != nil {
				baseCommit = record.Proof.BaseCommit
			}
			if baseCommit == "" {
				baseCommit, err = owner.baseCommit(record.BaseTree)
				if err != nil {
					return err
				}
			}
			if err := owner.store.HoldRegisteredTrunkRed(id, open, baseCommit, record.BaseTree, opid, at, owner.actor); err != nil {
				return err
			}
			return owner.release(id)
		}
		if record.State == StateLanded {
			return owner.release(id)
		}
	}
	if record.State == StateDiagnosing || record.State == StateLanding {
		lock := owner.batchLock(id)
		polled, pollErr := lock.poll()
		if pollErr != nil || polled != lockAcquired {
			return pollErr
		}
		token := ""
		if record.Proof != nil {
			token = record.Proof.Token
		}
		owner.dispatch(lock, Dispatch{ID: id, Window: "resume", Token: token, Runner: "host"})
		return nil
	}
	if record.State == StateHeldTrunkRed && record.TrunkRed != nil && len(record.TrunkRed.Entries) == 0 {
		if err := owner.release(id); err != nil {
			return err
		}
		outcome, recordErr := owner.store.EnsureTrunkRedRecorded(id, owner.mint, at, owner.actor)
		if outcome != "" && owner.logRed != nil {
			owner.logRed(id, outcome)
		}
		if errors.Is(recordErr, ErrTrunkRedRecordPending) {
			return nil
		}
		return recordErr
	}
	if record.State == StateHeldTrunkRed && record.TrunkRed != nil && len(record.TrunkRed.Entries) != 0 {
		if tree == record.BaseTree || tree == record.TrunkRed.Red.BaseTree || tree == record.TrunkRed.CheckedTree {
			return owner.release(id)
		}
		baseCommit, err := owner.baseCommit(tree)
		if err != nil {
			return err
		}
		return reopenHeldAfterDiagnostic(owner.store, id, tree, baseCommit, owner.actor, at, trunkRedClearSeams{
			run: func(request DiagnosticRequest, claim Claim) (DiagnosticResult, error) {
				return owner.runDiagnostic(id, request, claim)
			}, mint: owner.mint, ledger: owner.store.LedgerOwner(), descendsFrom: owner.descendsFrom,
		})
	}
	if record.State != StateOpen && record.State != StateSealed {
		return owner.release(id)
	}
	sample := owner.sample()
	start, _, err := owner.start(&record, sample, at)
	if err != nil {
		return err
	}
	if !start {
		return owner.release(id)
	}
	lock := owner.batchLock(id)
	polled, err := lock.poll()
	if err != nil || polled != lockAcquired {
		return err
	}
	sample = owner.sample()
	start, window, err := owner.start(&record, sample, at)
	if err != nil || !start {
		if err != nil {
			_ = lock.release()
			return err
		}
		return lock.release()
	}
	admission := owner.admission(sample)
	runner, room := owner.chooseRunner(sample, admission)
	if !room {
		return errors.Join(owner.recordCap(id, sample, admission, at), lock.release())
	}
	token, err := owner.mint()
	if err != nil {
		return errors.Join(err, lock.release())
	}
	owner.dispatch(lock, Dispatch{ID: id, Window: window, Token: token, Runner: runner, Sample: sample})
	return nil
}

// followMovedBase runs the moved-base decision (D2, R6) for a batch whose
// base another landing moved: a proving batch, or a sealed batch or an open
// one without a join in flight when no run of this owner is sealing it. A
// reopen puts it on the new base; a rebase keeps it, and its landing
// rebases the series. A landing batch decides in its landing run.
func (owner *Owner) followMovedBase(id, tree string, inflight bool) (bool, error) {
	record, err := owner.store.Load(id)
	if err != nil || owner.baseMove == nil || tree == "" || record.BaseTree == "" || tree == record.BaseTree {
		return false, err
	}
	joining := slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State == UnitJoining })
	settled := !inflight && (record.State == StateSealed || record.State == StateOpen && !joining)
	if record.State != StateProving && !settled || len(joinedUnits(record.Units)) == 0 {
		return false, nil
	}
	move, err := owner.baseMove(record.BaseTree, tree)
	if err != nil || !DecideMovedBase(record, move.Changed, move.Prefix).Reopen {
		return false, err
	}
	return true, ReopenMovedBase(owner.store, id, tree, move.LandedBy, owner.actor, owner.now())
}

// TickOnce waits for the run its tick started and releases a queue
// registration when a caller will not poll again.
func (owner *Owner) TickOnce(id string) (err error) {
	defer func() {
		if run := owner.inflight[id]; run != nil && run.attached {
			delete(owner.inflight, id)
		}
		err = errors.Join(err, owner.release(id))
	}()
	err = owner.Tick(id)
	for run := owner.inflight[id]; run != nil && !run.attached; run = owner.inflight[id] {
		done := <-owner.completions
		if done.ID == id {
			err = errors.Join(err, owner.complete(done))
		} else {
			owner.Complete(done)
		}
	}
	return err
}

func greenTipClearDetail(proof *Proof) string {
	if proof == nil {
		return ""
	}
	return "attempt=" + proof.AttemptID + " base=" + proof.BaseCommit
}

func greenTipClearStatus(record Record) (pending, complete bool) {
	detail := greenTipClearDetail(record.Proof)
	if detail == "" || record.Proof.Status != "green" || record.Proof.BaseCommit == "" {
		return false, true
	}
	for _, entry := range record.History {
		if entry.Detail != detail {
			continue
		}
		switch entry.Verb {
		case "trunk-red-clear-pending":
			pending = true
		case "trunk-red-clear-complete":
			complete = true
		}
	}
	return pending, complete
}

func (owner *Owner) clearGreenTipTrunkRed(id string, record Record, at time.Time) ([]OpenEntry, error) {
	if _, unbound := owner.store.LedgerOwner().(UnboundLedgerOwner); unbound {
		return nil, nil
	}
	// Only a trunk red holds a landing or clears on a green tip; a flake, hang
	// or quality entry closes by a proven fix or a person, never here.
	ledger := holdingLedger{owner.store.LedgerOwner()}
	pending, complete := greenTipClearStatus(record)
	if complete {
		return ledger.Open()
	}
	detail := greenTipClearDetail(record.Proof)
	if !pending {
		if err := owner.store.Update(id, func(current *Record) error {
			if current.Proof == nil || greenTipClearDetail(current.Proof) != detail || (current.State != StateLanding && current.State != StateLanded) {
				return fmt.Errorf("batch %s moved before trunk-red clearing was recorded", id)
			}
			current.Transition(current.State, at, "trunk-red-clear-pending", owner.actor, detail)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	open, err := clearGreenTipEntries(record.Proof, trunkRedClearSeams{mint: owner.mint, ledger: ledger, descendsFrom: owner.descendsFrom})
	if err != nil {
		return nil, err
	}
	err = owner.store.Update(id, func(current *Record) error {
		if current.Proof == nil || greenTipClearDetail(current.Proof) != detail || (current.State != StateLanding && current.State != StateLanded) {
			return fmt.Errorf("batch %s moved before trunk-red clearing completed", id)
		}
		_, alreadyComplete := greenTipClearStatus(*current)
		if !alreadyComplete {
			current.Transition(current.State, at, "trunk-red-clear-complete", owner.actor, detail)
		}
		return nil
	})
	return open, err
}

// holdingLedger narrows a ledger to the entries that hold a landing.
type holdingLedger struct{ LedgerOwner }

func (ledger holdingLedger) Open() ([]OpenEntry, error) {
	open, err := ledger.LedgerOwner.Open()
	return slices.DeleteFunc(open, func(entry OpenEntry) bool { return !entry.HoldsLanding() }), err
}

// release gives up a batch's queue registration or lock; a run in flight keeps
// its lock until its completion is applied.
func (owner *Owner) release(id string) error {
	if lock := owner.locks[id]; lock != nil && owner.inflight[id] == nil {
		return lock.release()
	}
	return nil
}

// retire forgets an ended batch: its lock is released, its own empty queue
// directory removed and its lock map entry dropped, so neither the queue root
// nor the owner's memory grows with every batch. A run still in flight keeps
// both until its completion is applied.
func (owner *Owner) retire(id string) error {
	if owner.inflight[id] != nil {
		return nil
	}
	lock := owner.locks[id]
	if lock == nil {
		lock = owner.lock(id)
	}
	delete(owner.locks, id)
	return lock.retire()
}

func joinedFacts(record Record) (int, time.Time) {
	joined := map[string]bool{}
	count := 0
	for _, unit := range record.Units {
		if unit.State == UnitJoined {
			joined[unit.GoalID] = true
			count++
		}
	}
	var oldest time.Time
	for _, entry := range record.History {
		fields := strings.Fields(entry.Detail)
		at, err := time.Parse(time.RFC3339Nano, entry.At)
		if entry.Verb == "join" && len(fields) > 0 && joined[fields[0]] && err == nil && (oldest.IsZero() || at.Before(oldest)) {
			oldest = at
		}
	}
	return count, oldest
}

func (owner *Owner) start(record *Record, sample proofrun.LoadSample, at time.Time) (bool, string, error) {
	joined, oldest := joinedFacts(*record)
	want, verb := record.CensusHold, ""
	if sample.OverlapKnown && want {
		want, verb = false, "unhold"
	}
	if !sample.OverlapKnown && joined > 0 && !oldest.IsZero() && owner.settings.MaxWaitElapsed(oldest) && !want {
		want, verb = true, "hold"
	}
	if verb != "" {
		detail := "census-unknown"
		if verb == "unhold" {
			detail = "census-known"
		}
		err := owner.store.Update(record.BatchID, func(current *Record) error {
			current.CensusHold = want
			current.Transition(current.State, at, verb, owner.actor, detail)
			return nil
		})
		if err != nil {
			return false, "", err
		}
		record.CensusHold = want
	}
	if oldest.IsZero() {
		return false, "", nil
	}
	var open []OpenEntry
	if _, unbound := owner.store.LedgerOwner().(UnboundLedgerOwner); !unbound {
		open, _ = owner.store.LedgerOwner().Open()
	}
	decision := pipelineStartRule(startInputs{record: *record, joined: joined, oldest: oldest, facts: owner.pipelineFacts(sample, at),
		open: open, settings: owner.settings, sample: sample, admission: owner.admission(sample), now: at, location: owner.location})
	// A tick decides twice, around its lock, at one time: the second call
	// keeps the first start's reason rather than re-deciding from the record
	// the first wrote.
	if previous, ok := owner.decided[record.BatchID]; ok && previous.at.Equal(at) && previous.decision.Start && decision.Start {
		decision = previous.decision
	}
	owner.decided[record.BatchID] = decidedAt{at: at, decision: decision}
	if err := owner.recordDecision(record, decision, at); err != nil {
		return false, "", err
	}
	if !decision.Start && decision.Wait != nil {
		if err := owner.useWait(*record, sample, at); err != nil {
			owner.report(record.BatchID, fmt.Errorf("use the wait: %w", err))
		}
	}
	return decision.Start, decision.Window, nil
}

// pipelineFacts reads the host board and the lane's retained records once
// per decision time: a tick decides twice, around its lock, from one read.
func (owner *Owner) pipelineFacts(sample proofrun.LoadSample, at time.Time) PipelineFacts {
	if owner.factsAt.Equal(at) && !at.IsZero() {
		return owner.facts
	}
	var records []Record
	if paths, err := owner.glob(filepath.Join(owner.store.root, "artifacts", "agents", "landing-batches", "*.json")); err == nil {
		for _, path := range paths {
			if record, loadErr := owner.store.Load(strings.TrimSuffix(filepath.Base(path), ".json")); loadErr == nil {
				records = append(records, record)
			}
		}
	}
	runner := ""
	if runners := owner.runners(sample, owner.admission(sample)); len(runners) > 0 {
		runner = runners[0].Runner
	}
	owner.facts = Facts(owner.pipeline.Board(at), records, owner.settings.Pipeline, runner, at)
	owner.factsAt = at
	return owner.facts
}

// recordDecision writes a decision onto the record only when its reason
// changed (R-129): a new wait records its line once, with one history
// entry; a start clears the wait and keeps its reason for the proof.
func (owner *Owner) recordDecision(record *Record, decision Decision, at time.Time) error {
	var changed bool
	switch {
	case decision.Start:
		changed = record.Wait != nil || record.StartReason != decision.Reason || record.Early != nil && record.Early.Ended == ""
	case decision.Wait != nil:
		changed = record.Wait == nil || record.Wait.Reason != decision.Wait.Reason
	}
	if !changed {
		return nil
	}
	var line string
	err := owner.store.Update(record.BatchID, func(current *Record) error {
		if decision.Start {
			current.Wait, current.StartReason = nil, decision.Reason
			endEarly(current, at, owner.actor)
		} else {
			current.Wait, current.StartReason = decision.Wait, ""
		}
		line = WaitLine(*current, at, owner.location)
		if !decision.Start {
			current.Transition(current.State, at, "wait", owner.actor, line)
		}
		return nil
	})
	if err != nil {
		return err
	}
	loaded, err := owner.store.Load(record.BatchID)
	if err != nil {
		return err
	}
	*record = loaded
	if owner.logWait != nil {
		owner.logWait(record.BatchID, line)
	}
	return nil
}

// helmSeat names the first unit's seat that is at the helm. A unit without a
// SeatRoot (joined before the owner learned to hold) is never held.
func (owner *Owner) helmSeat(record Record) (string, bool) {
	for _, unit := range record.Units {
		if owner.helmActive != nil && unit.SeatRoot != "" && owner.helmActive(unit.SeatRoot) {
			return unit.SeatRoot, true
		}
	}
	return "", false
}

// hold stands a batch down whole: its Tick is not called, so its record is
// untouched, and its dormant queue registration is withdrawn so no other batch
// waits behind it. release removes only this batch's entry; the lock
// directory only when its owner file names this pid, which outside a launch it
// never does.
func (owner *Owner) hold(id, seat string, previous map[string]HeldBatch) {
	entry := ""
	if lock := owner.locks[id]; lock != nil && lock.entry != "" {
		entry = filepath.Base(lock.entry)
	}
	if err := owner.release(id); err != nil {
		owner.report(id, fmt.Errorf("withdraw held batch queue registration: %w", err))
	}
	_, before := previous[id]
	owner.held[id] = HeldBatch{ID: id, Seat: seat, Entry: entry, New: !before}
}

// Held names the batches held on the last pass.
func (owner *Owner) Held() []HeldBatch {
	held := make([]HeldBatch, 0, len(owner.held))
	for _, batch := range owner.held {
		held = append(held, batch)
	}
	slices.SortFunc(held, func(a, b HeldBatch) int { return strings.Compare(a.ID, b.ID) })
	return held
}

// Withdraw stands the whole owner down (the landing seat's own helm): it
// releases every queue registration the owner holds. first is true on the
// transition; the next Resume ends the stand-down and registers again.
func (owner *Owner) Withdraw() (first bool, withdrawn []string, err error) {
	first, owner.standing = !owner.standing, true
	for id, lock := range owner.locks {
		if lock.entry != "" {
			withdrawn = append(withdrawn, filepath.Base(lock.entry))
		}
		err = errors.Join(err, owner.release(id))
	}
	slices.Sort(withdrawn)
	return first, withdrawn, err
}

func (owner *Owner) Resume() {
	owner.drain()
	owner.standing = false
	previous := owner.held
	owner.held = map[string]HeldBatch{}
	paths, err := owner.glob(filepath.Join(owner.store.root, "artifacts", "agents", "landing-batches", "*.json"))
	if err != nil {
		owner.report("", fmt.Errorf("enumerate landing batches: %w", err))
		return
	}
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		record, loadErr := owner.store.Load(id)
		if loadErr != nil {
			owner.report(id, loadErr)
			continue
		}
		_, clearComplete := greenTipClearStatus(record)
		if record.State == StateDissolved || record.State == StateLanded && clearComplete {
			if err := owner.retire(id); err != nil {
				owner.report(id, fmt.Errorf("retire ended batch lock: %w", err))
			}
			continue
		}
		if seat, held := owner.helmSeat(record); held {
			owner.hold(id, seat, previous)
			continue
		}
		if tickErr := owner.Tick(id); tickErr != nil {
			owner.report(id, tickErr)
		}
	}
}

func (owner *Owner) Loop(interval time.Duration, wake <-chan struct{}, stop <-chan struct{}) {
	for {
		owner.Resume()
		select {
		case <-stop:
			return
		default:
		}
		timer := owner.after(interval)
		select {
		case <-wake:
			continue
		default:
		}
		select {
		case <-wake:
		case <-timer:
		case done := <-owner.completions:
			owner.Complete(done)
		case done := <-owner.earlyDone:
			owner.CompleteEarly(done)
		case <-stop:
			return
		}
	}
}
