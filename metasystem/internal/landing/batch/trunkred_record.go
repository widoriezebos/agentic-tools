package batch

import (
	"fmt"
)

// ErrTrunkRedRecordPending means the ledger transaction is still in progress.
var ErrTrunkRedRecordPending = fmt.Errorf("%s: ledger transaction is still in progress", codeTrunkRedRecordPending)

// TrunkRedRecordOutcome says whether this call stored references or found that
// the batch no longer needed the write.
type TrunkRedRecordOutcome string

// TrunkRedRecordFailed describes a ledger transaction that was not confirmed.
type TrunkRedRecordFailed struct {
	Outcome  string
	Evidence string
}
