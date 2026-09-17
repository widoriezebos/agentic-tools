package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func init() { compiledBatchCapabilities[waitStatusDocsAndInventory] = struct{}{} }

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
	BatchID       string                `json:"batchId"`
	State         string                `json:"state"`
	Reason        string                `json:"reason,omitempty"`
	Owner         string                `json:"owner"`
	OwnerLiveness string                `json:"ownerLiveness"`
	Lock          string                `json:"lock"`
	ProofStatus   string                `json:"proofStatus,omitempty"`
	Headroom      []batchStatusHeadroom `json:"headroom"`
	Deadline      string                `json:"deadline,omitempty"`
	Branch        string                `json:"branch,omitempty"`
	BranchTip     string                `json:"branchTip,omitempty"`
	Sample        proofrun.LoadSample   `json:"sample"`
	Units         []batchStatusUnit     `json:"units"`
}

type batchStatusHeadroom struct {
	GoalID                string `json:"goalId"`
	Status                string `json:"status"`
	AttemptsLeft          uint64 `json:"attemptsLeft"`
	ReservedMinutesLeft   uint64 `json:"reservedMinutesLeft"`
	HasDiagnosticHeadroom bool   `json:"hasDiagnosticHeadroom"`
}

func statusHeadroom(root, goalID string, now time.Time, capMinutes uint64) batchStatusHeadroom {
	view := batchStatusHeadroom{GoalID: goalID, Status: string(dispatchcore.BudgetUnknown)}
	data, err := os.ReadFile(filepath.Join(root, "plans", "goals", goalID+".md"))
	if err != nil {
		return view
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		return view
	}
	projection := dispatchcore.ProjectBudget(root, file, now)
	view.Status = string(projection.Status)
	if projection.Status != dispatchcore.BudgetKnown {
		return view
	}
	if projection.Limits.AttemptLimit >= projection.Attempts {
		view.AttemptsLeft = projection.Limits.AttemptLimit - projection.Attempts
	}
	if projection.Limits.ReservedJobMinutesLimit >= projection.ReservedJobMinutes {
		view.ReservedMinutesLeft = projection.Limits.ReservedJobMinutesLimit - projection.ReservedJobMinutes
	}
	view.HasDiagnosticHeadroom = capMinutes <= ^uint64(0)/2 && view.AttemptsLeft >= 2 && view.ReservedMinutesLeft >= 2*capMinutes
	return view
}

func batchRecordStatus(record batch.Record, settings config.BatchLanding) batchStatusView {
	view := batchStatusView{BatchID: record.BatchID, State: record.State, Lock: batchStatusLock(""), Sample: batchStatusSample(settings.Root)}
	if record.Proof != nil {
		view.Reason, view.ProofStatus = record.Proof.Failure, record.Proof.Status
	}
	capMinutes, _, _, capErr := dispatchcore.ResolveCap(filepath.Join(settings.Root, "metasystem.conf"), "proof", "main", "proof", "", "")
	if capErr != nil || capMinutes < 1 {
		capMinutes = 120
	}
	for _, unit := range record.Units {
		if _, sealed := record.Seal[unit.GoalID]; sealed {
			view.Headroom = append(view.Headroom, statusHeadroom(settings.Root, unit.GoalID, batchStatusNow().UTC(), uint64(capMinutes)))
		}
	}
	if record.Landing != nil && record.Landing.BranchTip != "" {
		view.Branch, view.BranchTip = "landing/"+record.BatchID, record.Landing.BranchTip
	}
	pid, live, err := batchStatusOwner(settings.Root)
	view.Owner, view.OwnerLiveness = fmt.Sprint(pid), fmt.Sprint(live)
	if err != nil {
		view.OwnerLiveness = "unknown: " + err.Error()
	}
	var oldest time.Time
	for _, history := range record.History {
		if history.Verb != "join" {
			continue
		}
		joined, err := time.Parse(time.RFC3339Nano, history.At)
		if err == nil && (oldest.IsZero() || joined.Before(oldest)) {
			oldest = joined
		}
	}
	if !oldest.IsZero() {
		view.Deadline = oldest.Add(settings.MaxWait).UTC().Format(time.RFC3339Nano)
	}
	for index, unit := range record.Units {
		prefix := ""
		if index < len(record.PrefixTrees) {
			prefix = record.PrefixTrees[index]
		}
		view.Units = append(view.Units, batchStatusUnit{GoalID: unit.GoalID, Chain: unit.Chain, State: unit.State,
			CommitIDs: slices.Clone(unit.CommitIDs), LastUnit: unit.LastUnit, PrefixTree: prefix,
			Outcome: unit.Outcome, ReturnDisposition: unit.ReturnDisposition, Revision: unit.Claim.Revision,
			AccountingRevision: unit.Claim.AccountingRevision, ClaimEpoch: unit.Claim.Epoch})
	}
	return view
}

func batchReadSettings(root, landingRoot string, maxWait time.Duration) (config.BatchLanding, error) {
	return resolveBatchOwnerSettings(root, landingRoot, maxWait, batchWaitClock.Now)
}

func runBatchStatus(args []string) int {
	if !batchCapabilitiesAvailable() {
		return runBatchVerbSkeleton(nil)
	}
	flags := flag.NewFlagSet("landing batch status", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	landingRoot := pathFlag(flags, "landing-root", "", "resolved dedicated landing checkout")
	maxWait := flags.Duration("max-wait", config.DefaultBatchMaxWait, "maximum wait for another unit")
	id := flags.String("batch", "", "batch id")
	goalID := flags.String("goal", "", "goal id")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *id != "" && *goalID != "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem landing batch status --root ROOT [--batch ID|--goal GOAL]")
		return 2
	}
	settings, err := batchReadSettings(*root, *landingRoot, *maxWait)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	store := batch.NewStore(settings.Root, identity.KernelProber{})
	var records []batch.Record
	if *id != "" {
		record, loadErr := store.Load(*id)
		err, records = loadErr, []batch.Record{record}
	} else if *goalID != "" {
		record, findErr := batch.FindByGoal(store, *goalID)
		err, records = findErr, []batch.Record{record}
	} else {
		records, err = store.Records()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	views := make([]batchStatusView, 0, len(records))
	for _, record := range records {
		views = append(views, batchRecordStatus(record, settings))
	}
	printJSON(views)
	return 0
}

func runBatchWait(args []string) int {
	if !batchCapabilitiesAvailable() {
		return runBatchVerbSkeleton(nil)
	}
	flags := flag.NewFlagSet("landing batch wait", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	landingRoot := pathFlag(flags, "landing-root", "", "resolved dedicated landing checkout")
	maxWait := flags.Duration("max-wait", config.DefaultBatchMaxWait, "maximum wait for another unit")
	bound := flags.Duration("bound", 6*time.Hour, "hard wait bound")
	id := flags.String("batch", "", "batch id")
	goalID := flags.String("goal", "", "goal id")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*id == "") == (*goalID == "") {
		fmt.Fprintln(os.Stderr, "usage: metasystem landing batch wait --root ROOT (--batch ID|--goal GOAL) [--bound DURATION]")
		return 2
	}
	settings, err := batchReadSettings(*root, *landingRoot, *maxWait)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	store := batch.NewStore(settings.Root, identity.KernelProber{})
	if *id == "" {
		record, findErr := batch.FindByGoal(store, *goalID)
		if findErr != nil {
			fmt.Fprintln(os.Stderr, findErr)
			return 1
		}
		*id = record.BatchID
	}
	record, err := batch.Wait(store, *id, *bound, batchWaitClock)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(batchRecordStatus(record, settings))
	return 0
}
