package batch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type batchSeams struct {
	prober  identity.Prober
	flock   func(int, int) error
	publish func(string) error
}
type Store struct {
	root  string
	seams batchSeams
}

func NewStore(root string, prober identity.Prober) Store {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	return Store{root: root, seams: batchSeams{prober: prober, flock: unix.Flock, publish: func(string) error { return nil }}}
}
func (s Store) Liveness(p identity.Ref) identity.Liveness {
	return identity.AliveRef(s.seams.prober, p)
}
func (store Store) Load(id string) (Record, error) {
	path, err := store.recordPath(id)
	if err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, err
	}
	return record, validateRecord(record)
}
func (store Store) Create(record Record) error {
	return store.locked(func() error {
		path, err := store.recordPath(record.BatchID)
		if err != nil {
			return err
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return fmt.Errorf("batch %s already exists", record.BatchID)
		}
		return store.write(record)
	})
}
func (store Store) Update(id string, mutate func(*Record) error) error {
	return store.locked(func() error {
		return store.updateLocked(id, mutate)
	})
}
func (store Store) updateLocked(id string, mutate func(*Record) error) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	prior := record
	prior.Units, prior.History = slices.Clone(record.Units), slices.Clone(record.History)
	if err := mutate(&record); err != nil {
		return err
	}
	sameUnit := func(next, old Unit) bool {
		return next.GoalID == old.GoalID && next.Chain == old.Chain && next.Claim == old.Claim
	}
	unitsImmutable := len(record.Units) >= len(prior.Units) && slices.EqualFunc(record.Units[:len(prior.Units)], prior.Units, sameUnit)
	if record.BatchID != id || record.Schema != prior.Schema || !unitsImmutable {
		return fmt.Errorf("batch identity and existing units are immutable")
	}
	if len(record.History) < len(prior.History) || !slices.Equal(record.History[:len(prior.History)], prior.History) {
		return fmt.Errorf("batch history is append-only")
	}
	return store.write(record)
}
func (store Store) locked(change func() error) error {
	path := filepath.Join(store.root, "artifacts", "agents", "locks", "landing-batches.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := store.seams.flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer store.seams.flock(int(file.Fd()), unix.LOCK_UN)
	return change()
}
func (store Store) write(record Record) error {
	if err := validateRecord(record); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	path, _ := store.recordPath(record.BatchID)
	durable, err := atomicfile.WriteText(path, string(data)+"\n", store.root)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("batch %s was published with durability unknown", record.BatchID)
	}
	return nil
}
func (store Store) recordPath(id string) (string, error) {
	if len(id) != 26 || strings.Trim(id, "0123456789abcdefghjkmnpqrstvwxyz") != "" {
		return "", fmt.Errorf("batch id %q is not a lowercase ULID", id)
	}
	return filepath.Join(store.root, "artifacts", "agents", "landing-batches", id+".json"), nil
}
func validateRecord(record Record) error {
	if record.Schema != 1 || len(record.BatchID) != 26 || strings.Trim(record.BatchID, "0123456789abcdefghjkmnpqrstvwxyz") != "" || !strings.Contains("|open|sealed|proving|diagnosing|landing|landed|held-trunk-red|dissolved|", "|"+record.State+"|") {
		return fmt.Errorf("batch record is incomplete")
	}
	for _, unit := range record.Units {
		if unit.GoalID == "" || unit.Chain == "" || unit.Claim.Machine == "" || unit.Claim.Lineage == "" || unit.Claim.Epoch == 0 || unit.Claim.Revision == 0 || unit.Claim.AccountingRevision == 0 || !strings.Contains("|joining|joined|withdrawn-budget|ejected|landed|", "|"+unit.State+"|") {
			return fmt.Errorf("batch unit identity, revisions, or state are incomplete")
		}
	}
	return nil
}
