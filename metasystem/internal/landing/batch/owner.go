package batch

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

type ownerSeams struct {
	now           func() time.Time
	fetchTree     func() (string, error)
	readClaim     func(string, string, string, string) (Claim, error)
	returns       ReturnSeams
	rebind        func(string, string) error
	mint          func() (string, error)
	logRed        func(string, TrunkRedRecordOutcome)
	baseCommit    func(string) (string, error)
	runDiagnostic func(string, DiagnosticRequest, Claim) (DiagnosticResult, error)
	descendsFrom  func(string, string) (bool, error)
	sample        func() proofrun.LoadSample
	admission     func(proofrun.LoadSample) proofrun.AdmissionCap
	launch        func(string, proofrun.LoadSample, string) error
	lock          func(string) *proofLock
	after         func(time.Duration) <-chan time.Time
	report        func(string, error)
	glob          func(string) ([]string, error)
	locks         map[string]*proofLock
}

// OwnerOptions names every authority and side effect used by the durable
// batch owner. Callers must provide an injected clock; tests can substitute
// every external edge without weakening the production constructor.
type OwnerOptions struct {
	Store         Store
	Settings      config.BatchLanding
	Actor         string
	PID           int64
	LockDir       string
	QueueDir      string
	Now           func() time.Time
	FetchTree     func() (string, error)
	ReadClaim     func(string, string, string, string) (Claim, error)
	Returns       ReturnSeams
	Rebind        func(string, string) error
	Mint          func() (string, error)
	LogRed        func(string, TrunkRedRecordOutcome)
	BaseCommit    func(string) (string, error)
	RunDiagnostic func(string, DiagnosticRequest, Claim) (DiagnosticResult, error)
	DescendsFrom  func(string, string) (bool, error)
	Sample        func() proofrun.LoadSample
	Admission     func(proofrun.LoadSample) proofrun.AdmissionCap
	Launch        func(string, proofrun.LoadSample, string) error
	After         func(time.Duration) <-chan time.Time
	Report        func(string, error)
	Glob          func(string) ([]string, error)
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
		options.Sample == nil || options.Admission == nil || options.Launch == nil || options.After == nil || options.Report == nil {
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
		now: options.Now, fetchTree: options.FetchTree, readClaim: options.ReadClaim,
		returns: options.Returns, rebind: options.Rebind, mint: options.Mint, logRed: options.LogRed,
		baseCommit: options.BaseCommit, runDiagnostic: options.RunDiagnostic, descendsFrom: options.DescendsFrom, sample: options.Sample,
		admission: options.Admission, launch: options.Launch, after: options.After,
		report: options.Report, glob: options.Glob, locks: map[string]*proofLock{},
	}
	owner.lock = func(id string) *proofLock {
		return newProofLock(options.Store, options.LockDir, options.QueueDir, "batch:"+id, options.PID, options.Now)
	}
	return owner, nil
}

func (owner *Owner) Tick(id string) error {
	tree, err := owner.fetchTree()
	if err != nil {
		return err
	}
	at := owner.now()
	if err = ReconcileJoins(owner.store, id, tree, owner.actor, at, owner.readClaim); err != nil {
		return err
	}
	if err = ReturnUnits(owner.store, id, tree, owner.actor, at, owner.returns); err != nil {
		return err
	}
	if err = owner.rebind(id, tree); err != nil {
		return err
	}
	record, err := owner.store.Load(id)
	if err != nil {
		return err
	}
	if record.State == StateLanding {
		if _, unbound := owner.store.LedgerOwner().(UnboundLedgerOwner); !unbound {
			if err := clearGreenTipEntries(record.Proof, trunkRedClearSeams{mint: owner.mint, ledger: owner.store.LedgerOwner(), descendsFrom: owner.descendsFrom}); err != nil {
				return err
			}
		}
	}
	if record.State == StateDiagnosing || record.State == StateLanding {
		lock := owner.locks[id]
		if lock == nil {
			lock = owner.lock(id)
			owner.locks[id] = lock
		}
		polled, pollErr := lock.poll()
		if pollErr != nil || polled != lockAcquired {
			return pollErr
		}
		return lock.whileHeld(func() error { return owner.launch(id, proofrun.LoadSample{}, "resume") })
	}
	if record.State == StateHeldTrunkRed && record.TrunkRed != nil && len(record.TrunkRed.Entries) == 0 {
		if err := owner.release(id); err != nil {
			return err
		}
		outcome, recordErr := owner.store.EnsureTrunkRedRecorded(id, owner.mint, at, owner.actor)
		if outcome != "" && owner.logRed != nil {
			owner.logRed(id, outcome)
		}
		return recordErr
	}
	if record.State == StateHeldTrunkRed && record.TrunkRed != nil && len(record.TrunkRed.Entries) != 0 {
		if tree == record.BaseTree || tree == record.TrunkRed.Red.BaseTree {
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
	start, window, err := owner.start(&record, sample, at)
	if err != nil {
		return err
	}
	if !start {
		return owner.release(id)
	}
	lock := owner.locks[id]
	if lock == nil {
		lock = owner.lock(id)
		owner.locks[id] = lock
	}
	polled, err := lock.poll()
	if err != nil || polled != lockAcquired {
		return err
	}
	sample = owner.sample()
	start, window, err = owner.start(&record, sample, at)
	if err != nil || !start {
		if err != nil {
			_ = lock.release()
			return err
		}
		return lock.release()
	}
	return lock.whileHeld(func() error { return owner.launch(id, sample, window) })
}

func (owner *Owner) release(id string) error {
	if lock := owner.locks[id]; lock != nil {
		return lock.release()
	}
	return nil
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
	start, window, _ := batchStartRule(true, joined, oldest, owner.settings, sample, owner.admission(sample))
	return start, window, nil
}

func (owner *Owner) Resume() {
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
		if record.State == StateLanded || record.State == StateDissolved {
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
		case <-stop:
			return
		}
	}
}
