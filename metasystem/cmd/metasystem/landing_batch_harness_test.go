package main

// runLandingBatch is the test harness over the landing batch owners the
// build, land and supervised landing owner call.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
)

var landingBatchVerbs = map[string]command{
	"join": runBatchJoin, "status": runBatchStatus,
	"withdraw": runBatchWithdraw, "owner": runBatchOwner,
	"tick": runBatchTick, "wait": runBatchWait,
}

func runLandingBatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: metasystem internal landing batch <join|status|withdraw|owner|tick|wait>")
		return 2
	}
	run, ok := landingBatchVerbs[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "metasystem internal landing batch: unknown verb %q\n", args[0])
		return 2
	}
	return run(args[1:], stdout, stderr)
}

func runBatchJoin(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("landing batch join", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	goalID := flags.String("goal", "", "claimed goal id")
	chainID := flags.String("chain", "", "closed implementation chain root")
	last := flags.Bool("last", false, "join the whole land-ready goal")
	through := flags.String("through", "", "join through one land-ready goal commit")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *goalID == "" || ((*chainID != "") == (*last || *through != "")) || (*last && *through != "") {
		fmt.Fprintln(stderr, "usage: metasystem internal landing batch join --root ROOT --goal GOAL (--chain CHAIN | --last | --through COMMIT)")
		return 2
	}
	now, err := batchowner.BatchJoinClock(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	settings, err := config.ResolveBatchLanding(filepath.Join(*root, "metasystem.conf"), *root, func() time.Time { return now })
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	record, err := batchowner.ExecuteBatchJoin(batchowner.BatchJoinRequest{SeatRoot: *root, LandingRoot: settings.Root, GoalID: *goalID, ChainID: *chainID, Last: *last, Through: *through, At: now}, batchowner.BatchJoinDependenciesForCommand())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	writeJSONLine(stdout, stderr, map[string]any{"batchId": record.BatchID, "goalId": *goalID, "state": batch.UnitJoined})
	return 0
}

func batchOwnerSignals() (<-chan struct{}, <-chan struct{}, func()) {
	wakeSignal, stopSignal := make(chan os.Signal, 1), make(chan os.Signal, 1)
	wake, stop := make(chan struct{}, 1), make(chan struct{})
	signal.Notify(wakeSignal, syscall.SIGUSR1)
	signal.Notify(stopSignal, syscall.SIGTERM, os.Interrupt)
	go func() {
		for range wakeSignal {
			select {
			case wake <- struct{}{}:
			default:
			}
		}
	}()
	go func() { <-stopSignal; close(stop) }()
	return wake, stop, func() {
		signal.Stop(wakeSignal)
		signal.Stop(stopSignal)
		close(wakeSignal)
	}
}

func batchOwnerSourceNow(source *batchowner.BatchOwnerSource, root string) (time.Time, error) {
	if source == nil {
		return goalCommandNow(root)
	}
	return source.CommandNow(root)
}

func loopBatchOwner(out io.Writer, owner *batch.Owner, held batchowner.BatchOwnerLease, root string, clock func() time.Time, interval time.Duration, wake <-chan struct{}, stop <-chan struct{}) error {
	return loopBatchOwnerWithCadence(out, owner, held, root, clock, interval, wake, stop, batchowner.NewBatchOwnerCadence())
}

func loopBatchOwnerWithCadence(out io.Writer, owner *batch.Owner, held batchowner.BatchOwnerLease, root string, clock func() time.Time, interval time.Duration, wake <-chan struct{}, stop <-chan struct{}, cadence *batchowner.BatchOwnerCadence) error {
	defer cadence.Stop()
	for {
		if err := batchowner.BatchOwnerRequire(held); err != nil {
			return err
		}
		batchowner.RunBatchOwnerPass(out, owner, held, root, clock, cadence)
		// The harness's loop advances on events only: a wake, a completion
		// or the stop; the interval's passing is never a test's clock (the
		// production owner loop is the supervised component's).
		select {
		case <-wake:
		case done := <-owner.Completions():
			owner.Complete(done)
		case <-stop:
			return nil
		}
	}
}

func parseBatchOwnerWithSource(args []string, verb string, clock func() time.Time, source *batchowner.BatchOwnerSource) (config.BatchLanding, time.Duration, error) {
	flags := flag.NewFlagSet("landing batch "+verb, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	landingRoot := pathFlag(flags, "landing-root", "", "resolved dedicated landing checkout")
	maxWait := flags.Duration("max-wait", config.DefaultBatchMaxWait, "maximum wait for another unit")
	interval := flags.Duration("interval", time.Minute, "owner tick interval")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *interval <= 0 {
		return config.BatchLanding{}, 0, fmt.Errorf("usage: metasystem internal landing batch %s --root ROOT [--landing-root ROOT --max-wait DURATION]", verb)
	}
	if err := source.Validate(); err != nil {
		return config.BatchLanding{}, 0, err
	}
	if _, err := batchOwnerSourceNow(source, *root); err != nil {
		return config.BatchLanding{}, 0, err
	}
	settings, err := resolveBatchOwnerSettingsWithSource(*root, *landingRoot, *maxWait, clock, source)
	return settings, *interval, err
}

func resolveBatchOwnerSettings(seatRoot, landingRoot string, maxWait time.Duration, now func() time.Time) (config.BatchLanding, error) {
	return resolveBatchOwnerSettingsWithSource(seatRoot, landingRoot, maxWait, now, nil)
}

func resolveBatchOwnerSettingsWithSource(seatRoot, landingRoot string, maxWait time.Duration, now func() time.Time, source *batchowner.BatchOwnerSource) (config.BatchLanding, error) {
	if err := source.Validate(); err != nil {
		return config.BatchLanding{}, err
	}
	if landingRoot != "" {
		if source != nil {
			return config.ResolveExplicitBatchLandingWithRunner(landingRoot, seatRoot, maxWait, now, source.LandingGit)
		}
		return config.ResolveExplicitBatchLanding(landingRoot, seatRoot, maxWait, now)
	}
	if source != nil {
		return config.BatchLanding{}, fmt.Errorf("batch owner raw inputs require an explicit landing root")
	}
	return config.ResolveBatchLanding(filepath.Join(seatRoot, "metasystem.conf"), seatRoot, now)
}

func runBatchOwner(args []string, stdout, stderr io.Writer) (code int) {
	return runBatchOwnerWithSource(args, nil, stdout, stderr)
}

func runBatchOwnerWithSource(args []string, source *batchowner.BatchOwnerSource, stdout, stderr io.Writer) (code int) {
	clock := batchowner.CadenceProductionClock
	settings, interval, err := parseBatchOwnerWithSource(args, "owner", clock, source)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	inputs, err := batchowner.ResolveProductionBatchOwnerInputsWithSource(settings.Root, source)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	inputs.Log = stderr
	held, err := batchowner.BatchOwnerAcquire(settings.Root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() {
		if retireErr := held.Retire(); retireErr != nil {
			fmt.Fprintln(stderr, retireErr)
			code = 1
		}
	}()
	if err := parkBatchOwnerForFixture(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	owner, err := batchowner.BatchOwnerConstruct(settings, held, inputs, clock)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := batchowner.BatchOwnerSweepSources(settings.Root); err != nil {
		line, _ := json.Marshal(map[string]any{"component": "landing-owner", "sweep": "retained-sources", "error": err.Error()})
		fmt.Fprintln(stderr, string(line))
	}
	wake, stop, cleanup := batchOwnerSignals()
	defer cleanup()
	if err := loopBatchOwner(stderr, owner, held, settings.Root, clock, interval, wake, stop); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runBatchTick(args []string, stdout, stderr io.Writer) (code int) {
	return runBatchTickWithSource(args, nil, stdout, stderr)
}

func runBatchTickWithSource(args []string, source *batchowner.BatchOwnerSource, stdout, stderr io.Writer) (code int) {
	flags := flag.NewFlagSet("landing batch tick", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	landingRoot := pathFlag(flags, "landing-root", "", "resolved dedicated landing checkout")
	maxWait := flags.Duration("max-wait", config.DefaultBatchMaxWait, "maximum wait for another unit")
	id := flags.String("batch", "", "batch id")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *id == "" {
		fmt.Fprintln(stderr, "usage: metasystem internal landing batch tick --root ROOT --batch ULID")
		return 2
	}
	if err := source.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	now, err := batchOwnerSourceNow(source, *root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	settings, err := resolveBatchOwnerSettingsWithSource(*root, *landingRoot, *maxWait, func() time.Time { return now }, source)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	inputs, err := batchowner.ResolveProductionBatchOwnerInputsWithSource(settings.Root, source)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	inputs.Log = stderr
	held, err := batchowner.BatchOwnerAcquire(settings.Root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() {
		if retireErr := held.Retire(); retireErr != nil {
			fmt.Fprintln(stderr, retireErr)
			code = 1
		}
	}()
	owner, err := batchowner.BatchOwnerConstruct(settings, held, inputs, func() time.Time { return now })
	if err == nil {
		err = batchowner.BatchOwnerRequire(held)
	}
	if err == nil {
		err = batchowner.BatchOwnerTick(owner, *id)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func batchCadenceStatusWithEndpoint(endpoint goal.Endpoint, now time.Time) (batchowner.BatchCadenceStatusView, error) {
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return batchowner.BatchCadenceStatusView{}, err
	}
	return classifyBatchCadenceStatus(projection.Tree.Cadence, now)
}

func batchReadSettings(root, landingRoot string, maxWait time.Duration) (config.BatchLanding, error) {
	return resolveBatchOwnerSettings(root, landingRoot, maxWait, batchowner.BatchWaitClock.Now)
}

func batchRecordStatus(record batch.Record, settings config.BatchLanding, configuredLockDir ...string) batchowner.BatchStatusView {
	controlRoot := batch.ModuleRoot(settings.Root)
	lockDir := batch.DefaultProofLockDir
	if len(configuredLockDir) != 0 && configuredLockDir[0] != "" {
		lockDir = configuredLockDir[0]
	}
	view := batchowner.BatchStatusView{BatchID: record.BatchID, State: record.State, Lock: batchowner.BatchStatusLock(lockDir), Sample: batchowner.BatchStatusSample(controlRoot)}
	if record.Proof != nil {
		view.Reason, view.ProofStatus = record.Proof.Failure, record.Proof.Status
	}
	capMinutes, _, _, capErr := dispatchcore.ResolveCap(filepath.Join(controlRoot, "metasystem.conf"), "proof", "main", "proof", "", "")
	if capErr != nil || capMinutes < 1 {
		capMinutes = 120
	}
	for _, unit := range record.Units {
		if _, sealed := record.Seal[unit.GoalID]; sealed {
			view.Headroom = append(view.Headroom, statusHeadroom(controlRoot, unit.GoalID, batchowner.BatchStatusNow().UTC(), uint64(capMinutes)))
		}
	}
	view.LiveHeadroom = slices.Clone(view.Headroom)
	if record.CostForecast != nil {
		view.CostForecast = record.CostForecast
		view.CostSnapshotStale = !record.CostForecast.Matches(record)
	}
	if record.Landing != nil && record.Landing.BranchTip != "" {
		view.Branch, view.BranchTip = "landing/"+record.BatchID, record.Landing.BranchTip
	}
	pid, live, err := batchowner.BatchStatusOwner(settings.Root)
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
		view.Units = append(view.Units, batchowner.BatchStatusUnit{GoalID: unit.GoalID, Chain: unit.Chain, State: unit.State,
			CommitIDs: slices.Clone(unit.CommitIDs), LastUnit: unit.LastUnit, PrefixTree: prefix,
			Outcome: unit.Outcome, ReturnDisposition: unit.ReturnDisposition, Revision: unit.Claim.Revision,
			AccountingRevision: unit.Claim.AccountingRevision, ClaimEpoch: unit.Claim.Epoch})
	}
	return view
}

func classifyBatchCadenceStatus(status *goal.CadenceStatus, now time.Time) (batchowner.BatchCadenceStatusView, error) {
	if status == nil {
		return batchowner.BatchCadenceStatusView{State: "none-recorded"}, nil
	}
	view := batchowner.BatchCadenceStatusView{State: "green", TrunkCommit: status.TrunkCommit, TrunkTree: status.TrunkTree, EndedAt: status.EndedAt}
	window, parseErr := time.Parse(time.RFC3339, status.ForcedWindowStart)
	if parseErr != nil {
		return batchowner.BatchCadenceStatusView{}, parseErr
	}
	if !now.UTC().Before(window.Add(gaterun.CadenceForcedInterval + time.Minute)) {
		view.State = "overdue"
	} else if !status.Green() {
		view.State = "non-green"
	}
	return view, nil
}

func runBatchStatus(args []string, stdout, stderr io.Writer) int {
	return runBatchStatusWithSource(args, nil, goal.ResolveEndpoint, stdout, stderr)
}

func runBatchStatusWithOutput(args []string, source *batchowner.BatchOwnerSource, resolveEndpoint func(string) (goal.Endpoint, error), stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("landing batch status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	landingRoot := pathFlag(flags, "landing-root", "", "resolved dedicated landing checkout")
	maxWait := flags.Duration("max-wait", config.DefaultBatchMaxWait, "maximum wait for another unit")
	lockDir := flags.String("lock-dir", batch.DefaultProofLockDir, "configured proof lock directory")
	id := flags.String("batch", "", "batch id")
	goalID := flags.String("goal", "", "goal id")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *id != "" && *goalID != "" {
		fmt.Fprintln(stderr, "usage: metasystem internal landing batch status --root ROOT [--batch ID|--goal GOAL]")
		return 2
	}
	settings, err := resolveBatchOwnerSettingsWithSource(*root, *landingRoot, *maxWait, batchowner.BatchWaitClock.Now, source)
	if err != nil {
		fmt.Fprintln(stderr, err)
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
		fmt.Fprintln(stderr, err)
		return 1
	}
	endpoint, err := resolveEndpoint(batch.ModuleRoot(settings.Root))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cadence, err := batchCadenceStatusWithEndpoint(endpoint, batchowner.BatchStatusNow().UTC())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	views := make([]batchowner.BatchStatusView, 0, len(records))
	for _, record := range records {
		views = append(views, batchRecordStatus(record, settings, *lockDir))
	}
	encoded, err := json.Marshal(batchowner.BatchStatusOutput{Cadence: cadence, Batches: views})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if _, err := fmt.Fprintln(stdout, string(encoded)); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runBatchStatusWithSource(args []string, source *batchowner.BatchOwnerSource, resolveEndpoint func(string) (goal.Endpoint, error), stdout, stderr io.Writer) int {
	return runBatchStatusWithOutput(args, source, resolveEndpoint, stdout, stderr)
}

func runBatchWait(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("landing batch wait", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	landingRoot := pathFlag(flags, "landing-root", "", "resolved dedicated landing checkout")
	maxWait := flags.Duration("max-wait", config.DefaultBatchMaxWait, "maximum wait for another unit")
	lockDir := flags.String("lock-dir", batch.DefaultProofLockDir, "configured proof lock directory")
	bound := flags.Duration("bound", 6*time.Hour, "hard wait bound")
	id := flags.String("batch", "", "batch id")
	goalID := flags.String("goal", "", "goal id")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*id == "") == (*goalID == "") {
		fmt.Fprintln(stderr, "usage: metasystem internal landing batch wait --root ROOT (--batch ID|--goal GOAL) [--bound DURATION]")
		return 2
	}
	settings, err := batchReadSettings(*root, *landingRoot, *maxWait)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	store := batch.NewStore(settings.Root, identity.KernelProber{})
	if *id == "" {
		record, findErr := batch.FindByGoal(store, *goalID)
		if findErr != nil {
			fmt.Fprintln(stderr, findErr)
			return 1
		}
		*id = record.BatchID
	}
	record, err := batch.Wait(store, *id, *bound, batchowner.BatchWaitClock)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	writeJSONLine(stdout, stderr, batchRecordStatus(record, settings, *lockDir))
	return 0
}

func statusHeadroom(root, goalID string, now time.Time, capMinutes uint64) batchowner.BatchStatusHeadroom {
	view := batchowner.BatchStatusHeadroom{GoalID: goalID, Status: string(dispatchcore.BudgetUnknown)}
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

func executeBatchWithdraw(request batchowner.BatchWithdrawRequest, dependencies batchowner.BatchWithdrawDependencies) (batch.Record, error) {
	machine, err := dependencies.Machine(request.SeatRoot)
	if err != nil {
		return batch.Record{}, err
	}
	lineage := dependencies.Lineage()
	if lineage == "" {
		return batch.Record{}, fmt.Errorf("BATCH_WITHDRAW_REFUSED: export METASYSTEM_OWNER_LINEAGE for the recorded joiner")
	}
	record, err := dependencies.Withdraw(batch.NewStore(request.LandingRoot, nil), request.GoalID, machine, lineage, request.SeatRoot, machine+"+"+lineage, request.At)
	if err != nil {
		return batch.Record{}, err
	}
	if err := dependencies.Ensure(request.LandingRoot); err != nil {
		return batch.Record{}, err
	}
	return record, nil
}

func runBatchWithdraw(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("landing batch withdraw", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	goalID := flags.String("goal", "", "joined goal id")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *goalID == "" {
		fmt.Fprintln(stderr, "usage: metasystem internal landing batch withdraw --root ROOT --goal GOAL")
		return 2
	}
	now, err := batchowner.BatchWithdrawClock(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	settings, err := config.ResolveBatchLanding(filepath.Join(*root, "metasystem.conf"), *root, func() time.Time { return now })
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	record, err := executeBatchWithdraw(batchowner.BatchWithdrawRequest{SeatRoot: *root, LandingRoot: settings.Root, GoalID: *goalID, At: now}, batchowner.BatchWithdrawDependenciesForCommand())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	writeJSONLine(stdout, stderr, map[string]any{"batchId": record.BatchID, "goalId": *goalID, "state": batch.UnitReturnPending, "outcome": batch.UnitWithdrawn})
	return 0
}
