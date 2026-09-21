package batch

import "time"

const StateOpen, StateSealed, StateProving, StateDiagnosing, StateLanding, StateLanded, StateHeldTrunkRed, StateHeldUnclassified, StateDissolved = "open", "sealed", "proving", "diagnosing", "landing", "landed", "held-trunk-red", "held-unclassified", "dissolved"

const UnitJoining, UnitJoined, UnitReturnPending, UnitWithdrawn, UnitWithdrawnBudget, UnitEjected, UnitLanded = "joining", "joined", "return-pending", "withdrawn", "withdrawn-budget", "ejected", "landed"

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
	GoalID         string         `json:"goalId"`
	Chain          string         `json:"chain"`
	SeatRoot       string         `json:"seatRoot,omitempty"`
	Claim          Claim          `json:"claim"`
	State          string         `json:"state"`
	Gate           runIDs         `json:"gate,omitempty"`
	Admission      *JoinAdmission `json:"admission,omitempty"`
	ChangedPaths   []string       `json:"changedPaths,omitempty"`
	SelectedGroups []string       `json:"selectedGroups,omitempty"`
	Approver       string         `json:"approver,omitempty"`
	AuthorName     string         `json:"authorName,omitempty"`
	AuthorEmail    string         `json:"authorEmail,omitempty"`
	LandedCommit   string         `json:"landedCommit,omitempty"`
	P6Done         bool           `json:"p6Done,omitempty"`
	CommitIDs      []string       `json:"commitIds,omitempty"`
	LastUnit       string         `json:"lastUnit,omitempty"`
	GoalLast       bool           `json:"goalLast,omitempty"`
	BranchTip      string         `json:"branchTip,omitempty"`
	Builds         []BranchBuild  `json:"builds,omitempty"`

	unitRecordFields
}

// JoinAdmission is the durable gap between claim handover and membership.
// A joining unit cannot be promoted until its exact-tree admission is proved.
type JoinAdmission struct {
	Tree           string `json:"tree"`
	Status         string `json:"status"`
	DecisionID     string `json:"decisionId,omitempty"`
	AttemptID      string `json:"attemptId,omitempty"`
	ResultPath     string `json:"resultPath,omitempty"`
	FreshEpisode   string `json:"freshEpisode,omitempty"`
	FreshExpiresAt string `json:"freshExpiresAt,omitempty"`
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
	Schema   int                      `json:"schema"`
	BatchID  string                   `json:"batchId"`
	TipTree  string                   `json:"tipTree"`
	State    string                   `json:"state"`
	Units    []Unit                   `json:"units"`
	History  []HistoryEntry           `json:"history"`
	TrunkRed *TrunkRedHold            `json:"trunkRed,omitempty"`
	Proof    *Proof                   `json:"proof,omitempty"`
	Receipts map[string]PrefixReceipt `json:"receipts,omitempty"`
	Landing  *LandingProgress         `json:"landing,omitempty"`

	CensusHold bool `json:"censusHold,omitempty"`
	batchRecordFields
}

func (record *Record) Transition(to string, at time.Time, verb, actor, detail string) {
	entry := HistoryEntry{At: at.UTC().Format(time.RFC3339Nano), Verb: verb, From: record.State, To: to, Actor: actor, Detail: detail}
	record.History = append(record.History, entry)
	record.State = to
}
