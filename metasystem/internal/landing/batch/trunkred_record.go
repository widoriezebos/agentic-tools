package batch

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrTrunkRedRecordPending means the ledger transaction is still in progress.
var ErrTrunkRedRecordPending = errors.New("TRUNK_RED_RECORD_PENDING: ledger transaction is still in progress")

// TrunkRedRecordFailed describes a ledger transaction that was not confirmed.
type TrunkRedRecordFailed struct {
	Outcome  string
	Evidence string
}

// Error names the transaction outcome and its evidence.
func (failure *TrunkRedRecordFailed) Error() string {
	if failure == nil {
		return "TRUNK_RED_RECORD_FAILED : "
	}
	return fmt.Sprintf("TRUNK_RED_RECORD_FAILED %s: %s", failure.Outcome, failure.Evidence)
}

// EnsureTrunkRedRecorded records an unrecorded held trunk red through the ledger owner.
func (store Store) EnsureTrunkRedRecorded(id string, mint func() (string, error), at time.Time, actor string) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateHeldTrunkRed || record.TrunkRed == nil || len(record.TrunkRed.Entries) != 0 {
		return nil
	}
	used := record.TrunkRed.Opid
	entries, err := store.LedgerOwner().Record(used, record.TrunkRed.Red)
	if err == nil && len(entries) != 0 {
		return store.Update(id, func(record *Record) error {
			if record.State != StateHeldTrunkRed || record.TrunkRed == nil || len(record.TrunkRed.Entries) != 0 || record.TrunkRed.Opid != used {
				return nil
			}
			record.TrunkRed.Entries = entries
			record.TrunkRed.RecordedAt = at.UTC().Format(time.RFC3339Nano)
			ids := make([]string, len(entries))
			for index, entry := range entries {
				ids[index] = entry.ID
			}
			record.Transition(StateHeldTrunkRed, at, "trunk-red-recorded", actor, "entries="+strings.Join(ids, ","))
			return nil
		})
	}
	if err == nil {
		err = errors.New("ledger owner returned no entries")
	}
	if errors.Is(err, ErrTrunkRedRecordPending) {
		return err
	}
	var failed *TrunkRedRecordFailed
	if errors.As(err, &failed) && failed != nil {
		current, loadErr := store.Load(id)
		if loadErr != nil {
			return errors.Join(err, loadErr)
		}
		if current.State != StateHeldTrunkRed || current.TrunkRed == nil || current.TrunkRed.Opid != used || len(current.TrunkRed.Entries) != 0 {
			return err
		}
		next, mintErr := mint()
		if mintErr != nil {
			return errors.Join(err, mintErr)
		}
		updateErr := store.Update(id, func(record *Record) error {
			if record.State != StateHeldTrunkRed || record.TrunkRed == nil || record.TrunkRed.Opid != used || len(record.TrunkRed.Entries) != 0 {
				return nil
			}
			record.TrunkRed.Opids = append(record.TrunkRed.Opids, next)
			record.TrunkRed.Opid = next
			detail := fmt.Sprintf("opid=%s outcome=%s new-opid=%s", used, failed.Outcome, next)
			record.Transition(StateHeldTrunkRed, at, "trunk-red-record-failed", actor, detail)
			return nil
		})
		if updateErr != nil {
			return errors.Join(err, updateErr)
		}
		return err
	}
	detail := "trunk-red-record refused: " + err.Error()
	updateErr := store.Update(id, func(record *Record) error {
		if record.State != StateHeldTrunkRed || record.TrunkRed == nil || len(record.TrunkRed.Entries) != 0 || record.TrunkRed.Opid != used {
			return nil
		}
		if len(record.History) != 0 {
			newest := record.History[len(record.History)-1]
			if newest.Verb == "trunk-red-record-refused" && newest.Detail == detail {
				return nil
			}
		}
		record.Transition(StateHeldTrunkRed, at, "trunk-red-record-refused", actor, detail)
		return nil
	})
	if updateErr != nil {
		return errors.Join(err, updateErr)
	}
	return err
}
