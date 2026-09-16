package batch

import "time"

const StateOpen, StateSealed, StateProving, StateDiagnosing, StateLanding, StateLanded, StateHeldTrunkRed, StateDissolved = "open", "sealed", "proving", "diagnosing", "landing", "landed", "held-trunk-red", "dissolved"

const UnitJoining, UnitJoined, UnitWithdrawnBudget, UnitEjected, UnitLanded = "joining", "joined", "withdrawn-budget", "ejected", "landed"

type Claim struct {
	Machine            string `json:"machine"`
	Lineage            string `json:"lineage"`
	Epoch              uint64 `json:"epoch"`
	Revision           uint64 `json:"revision"`
	AccountingRevision uint64 `json:"accountingRevision"`
}
type Unit struct {
	GoalID string `json:"goalId"`
	Chain  string `json:"chain"`
	Claim  Claim  `json:"claim"`
	State  string `json:"state"`
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
	Schema  int            `json:"schema"`
	BatchID string         `json:"batchId"`
	TipTree string         `json:"tipTree"`
	State   string         `json:"state"`
	Units   []Unit         `json:"units"`
	History []HistoryEntry `json:"history"`
}

func (record *Record) Transition(to string, at time.Time, verb, actor, detail string) {
	entry := HistoryEntry{At: at.UTC().Format(time.RFC3339Nano), Verb: verb, From: record.State, To: to, Actor: actor, Detail: detail}
	record.History = append(record.History, entry)
	record.State = to
}
