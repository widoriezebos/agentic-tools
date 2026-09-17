package batch

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

type ownerSeams struct {
	now       func() time.Time
	fetchTree func() (string, error)
	readClaim func(string, string, string, string) (Claim, error)
	returns   ReturnSeams
	rebind    func(string) error
	sample    func() proofrun.LoadSample
	admission func(proofrun.LoadSample) proofrun.AdmissionCap
	launch    func(string, proofrun.LoadSample, string) error
	lock      func(string) *proofLock
	after     func(time.Duration) <-chan time.Time
	report    func(string, error)
	locks     map[string]*proofLock
}

type Owner struct {
	store    Store
	settings config.BatchLanding
	actor    string
	ownerSeams
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
	if err = owner.rebind(id); err != nil {
		return err
	}
	record, err := owner.store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateOpen {
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
	paths, _ := filepath.Glob(filepath.Join(owner.store.root, "artifacts", "agents", "landing-batches", "*.json"))
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
