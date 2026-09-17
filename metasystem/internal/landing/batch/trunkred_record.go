package batch

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrTrunkRedRecordPending means the ledger transaction is still in progress.
var ErrTrunkRedRecordPending = errors.New("TRUNK_RED_RECORD_PENDING: ledger transaction is still in progress")

// TrunkRedRecordOutcome says whether this call stored references or found that
// the batch no longer needed the write.
type TrunkRedRecordOutcome string

const (
	TrunkRedRecordRecorded TrunkRedRecordOutcome = "recorded"
	TrunkRedRecordAlready  TrunkRedRecordOutcome = "already"
	TrunkRedRecordMovedOn  TrunkRedRecordOutcome = "moved-on"
)

var (
	errTrunkRedRecordAlready = errors.New("the batch already stores trunk-red references")
	errTrunkRedRecordMovedOn = errors.New("the batch moved before trunk-red references were stored")
)

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

// IsTrunkRedClosed reports whether a ledger transaction refused a clear
// because another writer had already closed the entry.
func IsTrunkRedClosed(err error) bool {
	var failed *TrunkRedRecordFailed
	return errors.As(err, &failed) && failed != nil && strings.HasPrefix(failed.Evidence, "TRUNK_RED_CLOSED:")
}

// EnsureTrunkRedRecorded records an unrecorded held trunk red through the ledger owner.
func (store Store) EnsureTrunkRedRecorded(id string, mint func() (string, error), at time.Time, actor string) (TrunkRedRecordOutcome, error) {
	record, err := store.Load(id)
	if err != nil {
		return "", err
	}
	if record.State != StateHeldTrunkRed || record.TrunkRed == nil {
		return TrunkRedRecordMovedOn, nil
	}
	if len(record.TrunkRed.Entries) != 0 {
		return TrunkRedRecordAlready, nil
	}
	used := record.TrunkRed.Opid
	entries, err := store.LedgerOwner().Record(used, record.TrunkRed.Red)
	if err == nil && len(entries) != 0 {
		updateErr := store.Update(id, func(record *Record) error {
			if record.State != StateHeldTrunkRed || record.TrunkRed == nil || record.TrunkRed.Opid != used {
				return errTrunkRedRecordMovedOn
			}
			if len(record.TrunkRed.Entries) != 0 {
				return errTrunkRedRecordAlready
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
		if errors.Is(updateErr, errTrunkRedRecordAlready) {
			return TrunkRedRecordAlready, nil
		}
		if errors.Is(updateErr, errTrunkRedRecordMovedOn) {
			return TrunkRedRecordMovedOn, nil
		}
		if updateErr != nil {
			return "", updateErr
		}
		return TrunkRedRecordRecorded, nil
	}
	if err == nil {
		err = errors.New("ledger owner returned no entries")
	}
	if errors.Is(err, ErrTrunkRedRecordPending) {
		return "", err
	}
	var failed *TrunkRedRecordFailed
	if errors.As(err, &failed) && failed != nil {
		current, loadErr := store.Load(id)
		if loadErr != nil {
			return "", errors.Join(err, loadErr)
		}
		if current.State != StateHeldTrunkRed || current.TrunkRed == nil || current.TrunkRed.Opid != used || len(current.TrunkRed.Entries) != 0 {
			return TrunkRedRecordMovedOn, nil
		}
		next, mintErr := mint()
		if mintErr != nil {
			return "", errors.Join(err, mintErr)
		}
		updateErr := store.Update(id, func(record *Record) error {
			if record.State != StateHeldTrunkRed || record.TrunkRed == nil || record.TrunkRed.Opid != used || len(record.TrunkRed.Entries) != 0 {
				return errTrunkRedRecordMovedOn
			}
			record.TrunkRed.Opids = append(record.TrunkRed.Opids, next)
			record.TrunkRed.Opid = next
			detail := fmt.Sprintf("opid=%s outcome=%s new-opid=%s", used, failed.Outcome, next)
			record.Transition(StateHeldTrunkRed, at, "trunk-red-record-failed", actor, detail)
			return nil
		})
		if errors.Is(updateErr, errTrunkRedRecordMovedOn) {
			return TrunkRedRecordMovedOn, nil
		}
		if updateErr != nil {
			return "", errors.Join(err, updateErr)
		}
		return "", err
	}
	detail := "trunk-red-record refused: " + err.Error()
	updateErr := store.Update(id, func(record *Record) error {
		if record.State != StateHeldTrunkRed || record.TrunkRed == nil || record.TrunkRed.Opid != used {
			return errTrunkRedRecordMovedOn
		}
		if len(record.TrunkRed.Entries) != 0 {
			return errTrunkRedRecordAlready
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
	if errors.Is(updateErr, errTrunkRedRecordAlready) {
		return TrunkRedRecordAlready, nil
	}
	if errors.Is(updateErr, errTrunkRedRecordMovedOn) {
		return TrunkRedRecordMovedOn, nil
	}
	if updateErr != nil {
		return "", errors.Join(err, updateErr)
	}
	return "", err
}
