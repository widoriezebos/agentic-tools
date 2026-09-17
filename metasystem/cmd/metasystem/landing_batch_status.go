package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func init() { compiledBatchCapabilities[waitStatusDocsAndInventory] = struct{}{} }

var batchWaitClock = batch.WaitClock{Now: time.Now, After: time.After}
var batchStatusOwner = inspectBatchOwner
var batchStatusLock = func() string {
	data, err := os.ReadFile("/tmp/metasystem-testrun-lock/owner")
	if err != nil {
		return "free"
	}
	return strings.TrimSpace(string(data))
}
var batchStatusSample = func(root string) proofrun.LoadSample {
	return proofrun.SampleLoad(root, "", int64(os.Getpid()), time.Now())
}

type batchStatusUnit struct {
	GoalID             string `json:"goalId"`
	Chain              string `json:"chain"`
	State              string `json:"state"`
	Outcome            string `json:"outcome,omitempty"`
	ReturnDisposition  string `json:"returnDisposition,omitempty"`
	Revision           uint64 `json:"revision"`
	AccountingRevision uint64 `json:"accountingRevision"`
	ClaimEpoch         uint64 `json:"claimEpoch"`
}

type batchStatusView struct {
	BatchID       string              `json:"batchId"`
	State         string              `json:"state"`
	Reason        string              `json:"reason,omitempty"`
	Owner         string              `json:"owner"`
	OwnerLiveness string              `json:"ownerLiveness"`
	Lock          string              `json:"lock"`
	Headroom      string              `json:"headroom"`
	Deadline      string              `json:"deadline,omitempty"`
	Sample        proofrun.LoadSample `json:"sample"`
	Units         []batchStatusUnit   `json:"units"`
}

func batchRecordStatus(record batch.Record, settings config.BatchLanding) batchStatusView {
	view := batchStatusView{BatchID: record.BatchID, State: record.State, Lock: batchStatusLock(), Sample: batchStatusSample(settings.Root), Headroom: "required-at-P2"}
	if record.Proof != nil {
		view.Reason, view.Headroom = record.Proof.Failure, record.Proof.Status
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
	for _, unit := range record.Units {
		view.Units = append(view.Units, batchStatusUnit{GoalID: unit.GoalID, Chain: unit.Chain, State: unit.State,
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

var _ = filepath.Separator
