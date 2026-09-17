package batch

import "time"

const StateOpen, StateSealed, StateProving, StateDiagnosing, StateLanding, StateLanded, StateHeldTrunkRed, StateDissolved = "open", "sealed", "proving", "diagnosing", "landing", "landed", "held-trunk-red", "dissolved"

const UnitJoining, UnitJoined, UnitReturnPending, UnitWithdrawnBudget, UnitEjected, UnitLanded = "joining", "joined", "return-pending", "withdrawn-budget", "ejected", "landed"

const ReturnHandedBack, ReturnReleased, ReturnAlreadyReturned = "handed-back", "released", "already-returned"

type Claim struct {
	Machine            string `json:"machine"`
	Lineage            string `json:"lineage"`
	Epoch              uint64 `json:"epoch"`
	Revision           uint64 `json:"revision"`
	AccountingRevision uint64 `json:"accountingRevision"`
}
type runIDs []string
type unitRecordFields struct {
	Outcome           string `json:"outcome,omitempty"`
	Failure           string `json:"failure,omitempty"`
	ReturnDisposition string `json:"returnDisposition,omitempty"`
}
type Unit struct {
	GoalID string `json:"goalId"`
	Chain  string `json:"chain"`
	Claim  Claim  `json:"claim"`
	State  string `json:"state"`
	Gate   runIDs `json:"gate,omitempty"`

	unitRecordFields
}
type HistoryEntry struct {
	At     string `json:"at"`
	Verb   string `json:"verb"`
	From   string `json:"from"`
	To     string `json:"to"`
	Actor  string `json:"actor"`
	Detail string `json:"detail,omitempty"`
}
type Record struct {
	Schema   int            `json:"schema"`
	BatchID  string         `json:"batchId"`
	TipTree  string         `json:"tipTree"`
	State    string         `json:"state"`
	Units    []Unit         `json:"units"`
	History  []HistoryEntry `json:"history"`
	TrunkRed *TrunkRedHold  `json:"trunkRed,omitempty"`

	CensusHold bool `json:"censusHold,omitempty"`
	batchRecordFields
}

func (record *Record) Transition(to string, at time.Time, verb, actor, detail string) {
	entry := HistoryEntry{At: at.UTC().Format(time.RFC3339Nano), Verb: verb, From: record.State, To: to, Actor: actor, Detail: detail}
	record.History = append(record.History, entry)
	record.State = to
}
