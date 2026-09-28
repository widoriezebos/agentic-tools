package main

import (
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

var batchWaitClock = batch.WaitClock{Now: time.Now, After: time.After}
var batchStatusOwner = inspectBatchOwner
var batchStatusLock = batch.ProofLockOwner
var batchStatusNow = time.Now
var batchStatusSample = func(root string) proofrun.LoadSample {
	return proofrun.SampleLoad(root, "", int64(os.Getpid()), time.Now())
}

type batchStatusUnit struct {
	GoalID             string   `json:"goalId"`
	Chain              string   `json:"chain"`
	CommitIDs          []string `json:"commitIds,omitempty"`
	LastUnit           string   `json:"lastUnit,omitempty"`
	PrefixTree         string   `json:"prefixTree,omitempty"`
	State              string   `json:"state"`
	Outcome            string   `json:"outcome,omitempty"`
	ReturnDisposition  string   `json:"returnDisposition,omitempty"`
	Revision           uint64   `json:"revision"`
	AccountingRevision uint64   `json:"accountingRevision"`
	ClaimEpoch         uint64   `json:"claimEpoch"`
}

type batchStatusView struct {
	BatchID           string                `json:"batchId"`
	State             string                `json:"state"`
	Reason            string                `json:"reason,omitempty"`
	Owner             string                `json:"owner"`
	OwnerLiveness     string                `json:"ownerLiveness"`
	Lock              string                `json:"lock"`
	ProofStatus       string                `json:"proofStatus,omitempty"`
	Headroom          []batchStatusHeadroom `json:"headroom"`
	LiveHeadroom      []batchStatusHeadroom `json:"liveHeadroom"`
	CostForecast      *batch.CostForecast   `json:"costForecast,omitempty"`
	CostSnapshotStale bool                  `json:"costSnapshotStale,omitempty"`
	Deadline          string                `json:"deadline,omitempty"`
	Branch            string                `json:"branch,omitempty"`
	BranchTip         string                `json:"branchTip,omitempty"`
	Sample            proofrun.LoadSample   `json:"sample"`
	Units             []batchStatusUnit     `json:"units"`
}

type batchCadenceStatusView struct {
	State       string `json:"state"`
	TrunkCommit string `json:"trunkCommit,omitempty"`
	TrunkTree   string `json:"trunkTree,omitempty"`
	EndedAt     string `json:"endedAt,omitempty"`
}

type batchStatusOutput struct {
	Cadence batchCadenceStatusView `json:"cadence"`
	Batches []batchStatusView      `json:"batches"`
}

type batchStatusHeadroom struct {
	GoalID                string `json:"goalId"`
	Status                string `json:"status"`
	AttemptsLeft          uint64 `json:"attemptsLeft"`
	ReservedMinutesLeft   uint64 `json:"reservedMinutesLeft"`
	HasDiagnosticHeadroom bool   `json:"hasDiagnosticHeadroom"`
}
