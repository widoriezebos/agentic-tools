package batch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// FindOrCreateOpen selects the one open batch, or creates it while holding the
// batch flock. Preparation is deliberately not part of this critical section.
func FindOrCreateOpen(store Store, baseTree, id, actor string, at time.Time) (Record, error) {
	var selected Record
	err := store.locked(func() error {
		paths, err := filepath.Glob(filepath.Join(store.root, "artifacts", "agents", "landing-batches", "*.json"))
		if err != nil {
			return err
		}
		sort.Strings(paths)
		for _, path := range paths {
			recordID := strings.TrimSuffix(filepath.Base(path), ".json")
			record, loadErr := store.Load(recordID)
			if loadErr != nil {
				return loadErr
			}
			if record.State == StateOpen && record.ClosedReason == "" {
				if selected.BatchID != "" {
					return fmt.Errorf("more than one open landing batch exists")
				}
				selected = record
			}
		}
		if selected.BatchID != "" {
			return nil
		}
		selected = Record{Schema: 1, BatchID: id, BaseTree: baseTree, TipTree: baseTree}
		selected.Transition(StateOpen, at, "open", actor, "")
		return store.write(selected)
	})
	return selected, err
}

type batchSeams struct {
	prober      identity.Prober
	flock       func(int, int) error
	publish     func(string) error
	updated     func(Record)
	ledgerOwner LedgerOwner
}
type Store struct {
	root  string
	seams batchSeams
}

func NewStore(root string, prober identity.Prober) Store {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	return Store{root: root, seams: batchSeams{prober: prober, flock: unix.Flock, publish: func(string) error { return nil }, updated: func(Record) {}}}
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
		for _, unit := range record.Units {
			if terminalUnitState(unit.State) {
				return fmt.Errorf("new batch unit cannot start in terminal state %s", unit.State)
			}
		}
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
	before, err := json.Marshal(record)
	if err != nil {
		return err
	}
	prior := record
	prior.Units, prior.History = slices.Clone(record.Units), slices.Clone(record.History)
	if err := mutate(&record); err != nil {
		return err
	}
	sameUnit := func(next, old Unit) bool {
		return next.GoalID == old.GoalID && next.Chain == old.Chain && next.SeatRoot == old.SeatRoot && next.Claim == old.Claim &&
			next.Approver == old.Approver && next.AuthorName == old.AuthorName && next.AuthorEmail == old.AuthorEmail &&
			next.LastUnit == old.LastUnit && next.GoalLast == old.GoalLast && next.BranchTip == old.BranchTip && slices.Equal(next.CommitIDs, old.CommitIDs) &&
			reflect.DeepEqual(next.Builds, old.Builds) && slices.Equal(next.ChangedPaths, old.ChangedPaths) && slices.Equal(next.SelectedGroups, old.SelectedGroups)
	}
	unitsImmutable := len(record.Units) >= len(prior.Units) && slices.EqualFunc(record.Units[:len(prior.Units)], prior.Units, sameUnit)
	if record.BatchID != id || record.Schema != prior.Schema || !unitsImmutable {
		return fmt.Errorf("batch identity and existing units are immutable")
	}
	for index, unit := range record.Units {
		if index >= len(prior.Units) && terminalUnitState(unit.State) {
			return fmt.Errorf("new batch unit cannot start in terminal state %s", unit.State)
		}
		if index < len(prior.Units) && unit.State != prior.Units[index].State && terminalUnitState(unit.State) && (prior.Units[index].State != UnitReturnPending || !returnDisposition(unit.ReturnDisposition) || unit.Outcome != unit.State) {
			return fmt.Errorf("batch unit enters terminal state only from return-pending with its return disposition")
		}
	}
	if len(record.History) < len(prior.History) || !slices.Equal(record.History[:len(prior.History)], prior.History) {
		return fmt.Errorf("batch history is append-only")
	}
	after, err := json.Marshal(record)
	if err != nil || bytes.Equal(before, after) {
		return err
	}
	if err := store.write(record); err != nil {
		return err
	}
	store.seams.updated(record)
	return nil
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
	if record.Schema != 1 || len(record.BatchID) != 26 || strings.Trim(record.BatchID, "0123456789abcdefghjkmnpqrstvwxyz") != "" || !strings.Contains("|open|sealed|proving|diagnosing|landing|landed|held-trunk-red|held-unclassified|dissolved|", "|"+record.State+"|") {
		return fmt.Errorf("batch record is incomplete")
	}
	if record.State == StateHeldTrunkRed && (record.TrunkRed == nil || record.TrunkRed.Opid == "" || len(record.TrunkRed.Opids) == 0 || record.TrunkRed.Opids[len(record.TrunkRed.Opids)-1] != record.TrunkRed.Opid) {
		return fmt.Errorf("held trunk-red record is incomplete")
	}
	for _, unit := range record.Units {
		if unit.GoalID == "" || unit.Chain == "" || unit.Claim.Machine == "" || unit.Claim.Lineage == "" || unit.Claim.Epoch == 0 || unit.Claim.Revision == 0 || unit.Claim.AccountingRevision == 0 || !strings.Contains("|joining|joined|return-pending|withdrawn-budget|ejected|landed|", "|"+unit.State+"|") {
			return fmt.Errorf("batch unit identity, revisions, or state are incomplete")
		}
		if unit.State == UnitReturnPending && (!terminalUnitState(unit.Outcome) || strings.TrimSpace(unit.Failure) == "" || unit.ReturnDisposition != "") {
			return fmt.Errorf("return-pending batch unit needs a terminal outcome and reason without a disposition")
		}
	}
	return nil
}

func terminalUnitState(state string) bool {
	return state == UnitEjected || state == UnitWithdrawnBudget || state == UnitLanded
}

func returnDisposition(disposition string) bool {
	return disposition == ReturnHandedBack || disposition == ReturnReleased || disposition == ReturnAlreadyReturned
}
