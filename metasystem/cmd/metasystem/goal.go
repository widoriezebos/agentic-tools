package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/metrics"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// goalCommandNow keeps the wall clock authoritative unless the target root
// explicitly authorizes fixture inputs through its fake-runtime config.
func goalCommandNow(root string) (time.Time, error) {
	return fixtureauth.GoalNow(root)
}

// goalCommandClock resolves fixture authority once for one command. A fixture
// command receives one stable semantic instant; production commands continue
// to sample the wall clock on every call.
func goalCommandClock(root string) (func() time.Time, bool, error) {
	return fixtureauth.GoalClock(root, func() time.Time { return time.Now().UTC() })
}

// goalCommandBootClock is the monotonic companion to goalCommandNow. Fixture
// values are root-authorized; production roots always use the kernel clock.
func goalCommandBootClock(root string) (string, time.Duration, error) {
	authorization, err := fixtureauth.New(root)
	if err != nil {
		return "", 0, err
	}
	if bootID, elapsed, ok, err := authorization.Clock().GoalBootClock(); err != nil {
		return "", 0, err
	} else if ok {
		return bootID, elapsed, nil
	}
	return identity.BootClock()
}

const (
	goalNowEnvironment       = "METASYSTEM_GOAL_NOW"
	goalBootIDEnvironment    = "METASYSTEM_GOAL_BOOT_ID"
	goalBootNanosEnvironment = "METASYSTEM_GOAL_BOOT_NANOS"
)

// authorizedFixtureClockEnvironment carries fixture time only after the
// target root authorizes fake-runtime fixtures. Ambient clock variables are
// removed from production environments and malformed fixture clocks fail.
func authorizedFixtureClockEnvironment(root string, environment []string) ([]string, error) {
	authorization, err := fixtureauth.New(root)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(environment)+3)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if name != goalNowEnvironment && name != goalBootIDEnvironment && name != goalBootNanosEnvironment {
			result = append(result, entry)
		}
	}
	if now, ok, err := authorization.Clock().GoalNow(); err != nil {
		return nil, err
	} else if ok {
		result = append(result, goalNowEnvironment+"="+now.UTC().Format(time.RFC3339Nano))
	}
	if bootID, elapsed, ok, err := authorization.Clock().GoalBootClock(); err != nil {
		return nil, err
	} else if ok {
		result = append(result, goalBootIDEnvironment+"="+bootID,
			goalBootNanosEnvironment+"="+strconv.FormatInt(elapsed.Nanoseconds(), 10))
	}
	return result, nil
}

// The goal family: the doctrine commands humans and agents type.
// Mutations classify the caller and run the same authority matrix every
// record-writer path runs (holder-only), then hand the goal package a
// distilled caller; the goal-specific gates — active mission, baseline
// discipline, the advisory human reservation — live in the package.

// goalCaller classifies the invoking process and authorizes a mutation.
// Reconcile against a root with NO accepted baseline is GENESIS: the
// control plane being seeded does not exist yet, so holder-only would
// protect nothing and refuse everything (the adopt/provisioning path;
// reconcile is the only initialization).
//
// The genesis rule: the caller is classified against the root being
// written — the same root every goal verb classifies against, never a
// second one the caller names — and the authority matrix admits the
// human, the root's lease holder, and any other caller whose ledger is
// adoption-shaped (goal-free, on a checkout whose history carries no
// ledger; goal.AdoptionShaped). A terminal, an announced session, a
// session whose announcement lapsed, a fixture under agent ancestry and
// the kit gate in a delegate sandbox all seed a new control plane that
// way; nobody but the holder puts intent into one that exists, and the
// store re-judges the shape under its lock.
//
// Posture, stated plainly: this is cooperative, not
// unforgeable. --caller-pid names the ancestry for every classified verb
// in the system, a denied process table reads HUMAN for every verb, and a
// same-user actor can write the control-plane files directly; none of
// that is widened here, and none of it passes through a root this verb
// lets the caller choose.
func goalCaller(root string, callerPid int64, verb string) (goal.Caller, error) {
	return goalCallerWithRepositoryTop(root, callerPid, verb, stateroot.RepositoryTop)
}

func goalCallerWithRepositoryTop(root string, callerPid int64, verb string, repositoryTop func(string) (string, error)) (goal.Caller, error) {
	if callerPid == 0 {
		callerPid = int64(os.Getppid())
	}
	mode := "holder-only"
	genesis := verb == "reconcile"
	if genesis {
		if _, statErr := os.Stat(filepath.Join(root, "plans", "goals-accepted.json")); statErr == nil {
			genesis = false // an initialized project: holder-only, unchanged
		}
	}

	view, err := classifyVerbCallerWith(root, callerPid, repositoryTop)
	if err != nil {
		return goal.Caller{}, fmt.Errorf("who started this command couldn't be determined: %v", err)
	}
	classification := map[string]any{"class": view.Class, "holder": view.Holder}
	var shapeErr error
	if genesis {
		mode = "genesis"
		// A probe that cannot read refuses the SHAPE, never the human:
		// the flag stays false so the matrix refuses a non-holder, while
		// the human and the holder keep today's rule and the store
		// surfaces the real read error under its lock.
		shaped := false
		ledgerBytes, readErr := os.ReadFile(goal.LedgerPath(root))
		switch {
		case readErr != nil && !os.IsNotExist(readErr):
			shapeErr = readErr
		default:
			shaped, _, shapeErr = goal.AdoptionShaped(root, ledgerBytes)
			if shapeErr != nil {
				shaped = false
			}
		}
		classification["adoptionShaped"] = shaped
	}
	if err := authority.Authorize(mode, classification, ""); err != nil {
		if shapeErr != nil {
			return goal.Caller{}, fmt.Errorf("%v (the adoption-shape probe failed: %v)", err, shapeErr)
		}
		return goal.Caller{}, err
	}
	// The Genesis flag makes the authorization MODE travel with the
	// caller: the store refuses a genesis-admitted caller every
	// non-genesis arm under its lock, and re-judges the adoption shape
	// there for a non-holder.
	return goal.Caller{Class: view.Class, Holder: view.Holder, Genesis: mode == "genesis"}, nil
}

type legacyMutationInputs struct {
	repositoryTop func(string) (string, error)
	ensureGuard   func(string) error
	reporter      func(metrics.Options) (metrics.Result, error)
	// stdout and stderr are the invocation's streams the verb reports on.
	// caller, when set, is the supplied identity classification starts
	// from (owner_invocation.go) where no --caller-pid names one.
	stdout, stderr io.Writer
	caller         ownercall.Process
}

func goalMutationWithInputs(name string, args []string, extra func(*flag.FlagSet) []*string,
	run func(*goal.Store, goal.Caller, []string) (goal.Result, error),
	trySync func(string, []string) (int, bool), inputs legacyMutationInputs) int {
	stdout, stderr := inputs.stdout, inputs.stderr
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "Shared synced-goal options follow; each verb may accept fewer flags.")
		_, err := parseSyncFlagValuesWithOutput(name, args, stdout, stdout)
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintln(stderr, "goal "+name+" help:", err)
		return 2
	}
	if inputs.repositoryTop == nil {
		inputs.repositoryTop = stateroot.RepositoryTop
	}
	if inputs.ensureGuard == nil {
		inputs.ensureGuard = ensureGuardEnrolled
	}
	if inputs.reporter == nil {
		inputs.reporter = generateMetricsReport
	}
	if code, handled := trySync(name, args); handled {
		return code
	}
	flags := newFlagSet("goal "+name, stdout, stderr)
	root := pathFlag(flags, "root", ".", "checkout root")
	callerPid := flags.Int64("caller-pid", 0, "caller pid (defaults to the parent process)")
	var extras []*string
	if extra != nil {
		extras = extra(flags)
	}
	if flags.Parse(args) != nil {
		return 2
	}
	if *callerPid == 0 {
		*callerPid = inputs.caller.Pid
	}
	caller, err := goalCallerWithRepositoryTop(*root, *callerPid, name, inputs.repositoryTop)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	// Enrollment runs AFTER authorization: it executes the target's
	// pre-commit hook (the behavioral probe), and an unauthorized
	// caller must not be able to trigger foreign hook code through a
	// refused mutation.
	if err := inputs.ensureGuard(*root); err != nil {
		fmt.Fprintln(stderr, "goal "+name+": "+err.Error())
		return 1
	}
	values := make([]string, len(extras))
	for i, ptr := range extras {
		values[i] = *ptr
	}
	store := &goal.Store{Root: *root}
	result, err := run(store, caller, values)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, result.Message)
	for _, dropped := range result.Dropped {
		fmt.Fprintln(stdout, "dropped: "+dropped)
	}
	if name == "done" && len(values) > 0 {
		return reportAfterConfirmedDoneWithReporter(0, *root, values[0], stderr, inputs.reporter)
	}
	return 0
}

func goalReconcileLegacy(s *goal.Store, c goal.Caller, _ []string) (goal.Result, error) {
	return s.Reconcile(c)
}

// goalReconcileWith is goal reconcile under explicit request dependencies:
// the synced reconcile's request and proof start from their supplied caller
// and carry their lineage, the legacy one classifies from that caller, and
// both report on the caller's streams.
func goalReconcileWith(dependencies syncRequestDependencies, stdout, stderr io.Writer, args []string) int {
	dependencies.stdout, dependencies.stderr = stdout, stderr
	trySync := func(name string, args []string) (int, bool) {
		return trySyncMutationWithDependencies(name, args, goalCommandNow, dependencies, goalParkBranchCheck)
	}
	return goalMutationWithInputs("reconcile", args, nil, goalReconcileLegacy, trySync,
		legacyMutationInputs{stdout: stdout, stderr: stderr, caller: dependencies.authorityFacts.caller})
}

// converted reports the post-migration world by POSITIVE evidence:
// the legacy ledger is gone AND the synced tree is present. Absence
// of goals.md alone is not conversion — fixture sandboxes and plain
// directories never had a backlog, and routing them into the sync
// engine sends fetches at remotes that do not exist. The legacy
// file's presence keeps reads on the legacy store until migration.
func converted(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "plans", "goals.md")); err == nil {
		return false
	}
	_, err := os.Stat(filepath.Join(root, "plans", "goals", "backlog.md"))
	return err == nil
}

func nextSyncedWithInputs(stdout, stderr io.Writer, root, machine string, fetchFirst bool, resolve func(string) (goal.Endpoint, error), commandNow func(string) (time.Time, error), project func(goal.Endpoint, bool, time.Time) (goal.Projection, error), presence func(string, goal.Endpoint) (seat.Copy, error), requiredLabels ...string) int {
	e, err := resolve(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	now, err := commandNow(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	p, err := project(e, fetchFirst, now)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	// Projection notices such as stale or single-machine state print before
	// orientation so the caller sees the limits on the answer.
	for _, banner := range p.Banners {
		fmt.Fprintln(stdout, banner)
	}
	// What the other machines are, before what this one can take: a goal
	// nobody is moving because its holder has gone quiet is context for the
	// frontier and never part of it. Standard error, so the orientation line
	// an agent reads is exactly the line it read before.
	for _, line := range silentHolderLines(root, e, p.Tree, machine, now, presence) {
		fmt.Fprintln(stderr, line)
	}
	frontier, frontierErr := goal.Next(p, machine, requiredLabels...)
	if frontierErr != nil {
		fmt.Fprintln(stderr, "goal next could not answer: "+frontierErr.Error())
		return 1
	}
	for _, entry := range frontier.TrunkRedOwned {
		fmt.Fprintln(stdout, trunkRedOwnedLine(entry))
	}
	fenced := make([]*goal.GoalFile, 0, len(frontier.Fenced))
	for _, id := range frontier.Fenced {
		fenced = append(fenced, p.Tree.Live[id])
	}
	for _, line := range goal.FencedClaimLines(fenced) {
		fmt.Fprintln(stdout, line)
	}
	landing := make([]*goal.GoalFile, 0, len(frontier.Landing))
	for _, id := range frontier.Landing {
		landing = append(landing, p.Tree.Live[id])
	}
	for _, line := range goal.LandingClaimLines(landing, now) {
		fmt.Fprintln(stdout, line)
	}
	selection := goal.SelectNext(frontier)
	switch selection.Kind {
	case goal.NextSelectionContinue:
		fmt.Fprintln(stdout, "continue your claimed goal: "+selection.GoalID)
		printOpenReadItemBlocks(stdout, p.Tree.Live[selection.GoalID])
	case goal.NextSelectionReady:
		fmt.Fprintln(stdout, "next ready goal: "+selection.GoalID)
		printOpenReadItemBlocks(stdout, p.Tree.Live[selection.GoalID])
	default:
		if len(requiredLabels) > 0 {
			matched := false
			for _, file := range p.Tree.Live {
				if goal.MatchesLabels(file.Labels, requiredLabels) {
					matched = true
					break
				}
			}
			if !matched {
				fmt.Fprintln(stdout, "no goal matches --label "+strings.Join(requiredLabels, " --label "))
				break
			}
		}
		line := "no claimable goal for machine " + machine
		switch {
		case len(frontier.Refused) > 0:
			line += fmt.Sprintf("; claim would refuse %d (first: %s): %s", len(frontier.Refused), frontier.Refused[0].GoalID, frontier.Refused[0].Cause)
		case len(frontier.Blocked) > 0:
			line += "; first blocked goal: " + frontier.Blocked[0]
		case len(frontier.Awaiting) > 0:
			line += fmt.Sprintf("; %d await the human's approval (first: %s)", len(frontier.Awaiting), frontier.Awaiting[0])
		case len(p.Tree.Live) == 0:
			line += "; the backlog is empty"
		default:
			line += "; no matching eligible work"
		}
		fmt.Fprintln(stdout, line)
	}
	for _, entry := range frontier.TrunkRedElsewhere {
		owner := entry.Owner.Machine
		if owner == "" {
			owner = "nobody"
		}
		fmt.Fprintf(stdout, "trunk red %s owned by %s since %s\n", entry.ID, owner, entry.Owner.Since)
	}
	return 0
}

func printOpenReadItemBlocks(stdout io.Writer, file *goal.GoalFile) {
	for _, block := range goal.OpenReadItemBlocks(file) {
		fmt.Fprintln(stdout, block.Heading)
		for _, item := range block.Items {
			fmt.Fprintf(stdout, "- %s: %s\n", item.ID, item.Text)
		}
	}
}

func trunkRedOwnedLine(entry goal.TrunkRedEntry) string {
	observed := entry.Status
	if len(entry.Failures) > 0 {
		failure := entry.Failures[0]
		observed = failure.Report + "/" + failure.Classname + "/" + failure.Name
	}
	sighting := goal.TrunkRedSighting{}
	if len(entry.Sightings) > 0 {
		sighting = entry.Sightings[len(entry.Sightings)-1]
	}
	fix := entry.FixGoal
	if fix == "" {
		fix = "take it first"
	}
	line := fmt.Sprintf("trunk red %s: %s %s on %s, holds %d batches, since %s; fix it under %s", entry.ID, entry.Group, observed, sighting.BaseCommit, len(entry.Holds), entry.Owner.Since, fix)
	if entry.FixBranch.Name != "" {
		line += fmt.Sprintf(" on branch %s@%s (%s)", entry.FixBranch.Name, entry.FixBranch.Commit, entry.FixBranch.State)
	}
	return line
}

// runGoalNext prints the one orientation line any runtime's main can read
// by instruction — the universal fallback transport.
func runGoalNext(args []string, stdout, stderr io.Writer) int {
	return runGoalNextWithInputs(args, defaultSyncRequestDependencies(), goalCommandNow, stdout, stderr)
}

func runGoalNextWithInputs(args []string, dependencies syncRequestDependencies, commandNow func(string) (time.Time, error), stdout, stderr io.Writer) int {
	flags := newFlagSet("goal next", stdout, stderr)
	root := pathFlag(flags, "root", ".", "checkout root")
	machineFlag := flags.String("machine", "", "machine nickname whose ordered frontier to inspect")
	fetch := flags.Bool("fetch", false, "fetch and validate the canonical backlog before selecting")
	var labels repeatedStrings
	flags.Var(&labels, "label", "label token required on recommendation candidates (repeatable)")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "goal next accepts flags only")
		return 2
	}
	if err := goal.ValidateLabels(labels); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if stateRoot, rootErr := goal.ResolveStateRoot(*root); rootErr == nil {
		if waiting, waitErr := report.CurrentWaitingLines(stateRoot); waitErr == nil {
			for _, line := range waiting {
				fmt.Fprintln(stdout, line)
			}
		} else {
			fmt.Fprintln(stderr, "durable wait recovery rows could not be read:", waitErr)
		}
	} else {
		fmt.Fprintln(stderr, "durable wait recovery rows could not be read:", rootErr)
	}
	machineProvided := false
	flags.Visit(func(flag *flag.Flag) {
		if flag.Name == "machine" {
			machineProvided = true
		}
	})
	if converted(*root) {
		machine := *machineFlag
		if machineProvided {
			if err := goal.ValidateMachineNickname(machine); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		} else {
			var err error
			machine, err = dependencies.machine(*root)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}
		return nextSyncedWithInputs(stdout, stderr, *root, machine, *fetch, dependencies.endpoint, commandNow, goal.Project, dependencies.presence, labels...)
	}
	if len(labels) > 0 || machineProvided || *fetch {
		fmt.Fprintln(stderr, "--label, --machine and --fetch need the upgraded goal list, and this checkout has the old one\nrun: metasystem goal sync --upgrade")
		return 1
	}
	store := &goal.Store{Root: *root}
	ledger, problems, err := store.ReadLedger()
	switch {
	case err != nil:
		fmt.Fprintln(stderr, err)
		return 1
	case ledger == nil && store.BaselinePresent():
		fmt.Fprintln(stdout, "goals.md was deleted after it was set up\nrun: metasystem goal sync")
	case ledger == nil:
		fmt.Fprintln(stdout, "no goals yet; metasystem goal open starts one")
	case len(problems) > 0:
		fmt.Fprintln(stdout, "the goal list has a problem: "+string(problems[0]))
	case ledger.Current != nil:
		fmt.Fprintf(stdout, "%s — %s; next: %s\n", ledger.Current.Id, ledger.Current.Intent, ledger.Current.NextStep)
	case ledger.Free != nil:
		fmt.Fprintln(stdout, "goal-free declared "+ledger.Free.Declared)
	case len(ledger.Queued) > 0:
		fmt.Fprintf(stdout, "no current goal; %s is next in the queue\nrun: metasystem goal sync --upgrade  (to upgrade this old goal list)\n", ledger.Queued[0].Id)
	default:
		fmt.Fprintln(stdout, "no current goal")
	}
	return 0
}

// reportTurnVerdict is the one structured turn-end decision: it scans the
// checkout, judges the turn, writes the frozen facts and completion
// observation a Stop presentation reads, and prints the verdict JSON.
func reportTurnVerdict(request hooks.TurnVerdictRequest, stdout, stderr io.Writer, resolve func(string) (goal.Endpoint, error), resolveMachine func(string) (string, error)) int {
	root, session, watchdog, mainId := &request.Root, &request.Session, &request.Watchdog, &request.MainID
	stopHookActive, sessionAbsent := &request.StopHookActive, &request.SessionAbsent
	transcript, runtimeName := &request.Transcript, &request.Runtime
	factsFile, completionFile := &request.FactsFile, &request.CompletionFile
	if *root == "" {
		*root = "."
	}
	for _, output := range []struct {
		name string
		path string
	}{{"--facts-file", *factsFile}, {"--completion-file", *completionFile}} {
		if output.path == "" {
			continue
		}
		if !filepath.IsAbs(output.path) {
			fmt.Fprintf(stderr, "report turn-verdict: %s must be absolute\n", output.name)
			return 2
		}
		if _, err := os.Lstat(output.path); err == nil {
			fmt.Fprintf(stderr, "report turn-verdict: %s must not already exist\n", output.name)
			return 2
		} else if !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "report turn-verdict: cannot inspect %s: %v\n", output.name, err)
			return 2
		}
	}
	if *factsFile != "" && filepath.Clean(*factsFile) == filepath.Clean(*completionFile) {
		fmt.Fprintln(stderr, "report turn-verdict: --facts-file and --completion-file must differ")
		return 2
	}
	var endpoint goal.Endpoint
	var machine string
	var err error
	if resolve != nil {
		endpoint, err = resolve(*root)
		if err == nil {
			machine, err = resolveMachine(*root)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	var scan goal.ScanResult
	var completionCapture report.StopCompletionCapture
	if resolve == nil {
		scan, completionCapture = report.ScanWithCompletion(*root)
	} else {
		scan, completionCapture = report.ScanWithCompletionAtEndpoint(*root, endpoint, machine)
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(scan.Busy) == 0 {
		var warning string
		scan.Open, warning, err = report.MarkOpenWorkSeen(*root, scan.Open, now)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if warning != "" {
			scan.OpenWorkWarnings = append(scan.OpenWorkWarnings, warning)
		}
	}
	store := &goal.Store{
		Root: *root,
		Now:  func() time.Time { return now },
		BootClock: func() (string, time.Duration, error) {
			return goalCommandBootClock(*root)
		},
	}
	options := goal.TurnVerdictOptions{StopHookActive: *stopHookActive, SessionAbsent: *sessionAbsent}
	stateRoot, rootErr := goal.ResolveStateRoot(*root)
	options.ContextLine = turnVerdictContextLine(*root, stateRoot, *runtimeName, *session, *transcript, now, rootErr)
	if rootErr != nil {
		options.SeatActorProblem = "the seat state root could not be resolved: " + rootErr.Error()
	} else {
		options.HandoffRecorded = func(session string) (string, bool, error) {
			return steward.LiveHandoffForSession(stateRoot, session)
		}
		store.PrepareIdleContinuation = prepareSeatIdleContinuation(stateRoot)
		store.RecordIdleIncident = recordSeatIdleIncident(stateRoot, now)
		store.RaiseIdleAlarm = raiseSeatIdleAlarm(stateRoot)
		store.ResolveIdleSeat = resolveSeatIdleActorWithMachine(stateRoot, *mainId, resolveMachine)
		// The holder takes its due landing or revision on this Stop through
		// the public command, from the session's checkout (g1-s70 D3).
		if checkout, absErr := filepath.Abs(*root); absErr == nil {
			store.TakeHolderStep = holderStepTaker(checkout, defaultIntentOwners())
		}
	}
	var verdict goal.Verdict
	if resolve == nil {
		verdict, err = store.TurnVerdict(scan, *session, *watchdog, *mainId, options)
	} else {
		verdict, err = store.TurnVerdictAtEndpoint(endpoint, machine, scan, *session, *watchdog, *mainId, options)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if verdict.Facts != nil {
		verdict.Facts.Verdict = verdict
	}
	data, err := json.Marshal(verdict)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *factsFile != "" {
		if verdict.Facts == nil {
			fmt.Fprintln(stderr, "report turn-verdict: frozen facts are unavailable for this judgment")
		} else if facts, marshalErr := json.Marshal(verdict.Facts); marshalErr != nil {
			fmt.Fprintln(stderr, "report turn-verdict: frozen facts could not be rendered:", marshalErr)
		} else if durable, writeErr := turnVerdictFactsWriter(*factsFile, string(facts)+"\n", ""); writeErr != nil || !durable {
			_ = os.Remove(*factsFile)
			if writeErr != nil {
				fmt.Fprintln(stderr, "report turn-verdict: frozen facts could not be written:", writeErr)
			} else {
				fmt.Fprintln(stderr, "report turn-verdict: frozen facts could not be written: crash durability is unknown")
			}
		}
	}
	if *completionFile != "" {
		completionRoot := *root
		completionSession := goal.NormalizeSession(*session)
		if verdict.Facts != nil {
			completionRoot = verdict.Facts.Identity.Installation
			completionSession = verdict.Facts.Identity.Session
		} else if absolute, resolveErr := filepath.Abs(completionRoot); resolveErr == nil {
			completionRoot = filepath.Clean(absolute)
			if physical, physicalErr := filepath.EvalSymlinks(completionRoot); physicalErr == nil {
				completionRoot = physical
			}
		}
		observation := report.BindStopCompletion(completionCapture, completionRoot, completionSession, *mainId, now)
		if completion, marshalErr := json.Marshal(observation); marshalErr != nil {
			fmt.Fprintln(stderr, "report turn-verdict: completion observation could not be rendered:", marshalErr)
		} else if durable, writeErr := turnVerdictCompletionWriter(*completionFile, string(completion)+"\n", ""); writeErr != nil || !durable {
			_ = os.Remove(*completionFile)
			if writeErr != nil {
				fmt.Fprintln(stderr, "report turn-verdict: completion observation could not be written:", writeErr)
			} else {
				fmt.Fprintln(stderr, "report turn-verdict: completion observation could not be written: crash durability is unknown")
			}
		}
	}
	fmt.Fprintln(stdout, string(data))
	return 0
}

func turnVerdictContextLine(root, stateRoot, runtimeName, session, transcript string, now time.Time, rootErr error) string {
	const contextPrefix = "CONTEXT: "
	unknown := func(reason string) string {
		reason = strings.ReplaceAll(strings.ReplaceAll(reason, "\r", " "), "\n", " ")
		reasonRunes := []rune(reason)
		if limit := goal.TurnVerdictDisplayRuneLimit / 8; len(reasonRunes) > limit {
			reason = string(reasonRunes[:limit/2]) + "..." + string(reasonRunes[len(reasonRunes)-limit/2:])
		}
		return contextPrefix + "unknown (" + reason + ")"
	}
	if transcript == "" || runtimeName == "" {
		return unknown("--transcript and --runtime are required")
	}
	if rootErr != nil {
		return unknown(rootErr.Error())
	}
	budget, err := config.ContextBudget(root)
	if err != nil {
		return unknown(err.Error())
	}
	reading, err := usagepkg.LatestCall(stateRoot, runtimeName, session, usagepkg.ReadOptions{
		Capability: usagepkg.PerCall, Transcript: transcript, Installation: root, Now: now, NonBlocking: true,
	})
	if err != nil {
		return unknown(err.Error())
	}
	if reading.Latest == nil {
		return unknown(reading.Reason)
	}
	return fmt.Sprintf(contextPrefix+"%dK of trigger %dK (proof line %dK, maximum %dK, ceiling %dK)",
		turnContextThousands(reading.Latest.PromptTokens), budget.Trigger/1000,
		steward.ProofP95Tokens/1000, steward.ProofMaxTokens/1000, budget.Ceiling/1000)
}

func turnContextThousands(tokens int64) int64 {
	thousands := tokens / 1000
	if tokens%1000 >= 500 {
		thousands++
	}
	return thousands
}

var turnVerdictFactsWriter = atomicfile.WriteText
var turnVerdictCompletionWriter = atomicfile.WriteText

func resolveSeatIdleActorWithMachine(root, mainID string, resolveMachine func(string) (string, error)) func() (goal.Actor, int64, error) {
	if resolveMachine == nil {
		resolveMachine = goal.ResolveMachine
	}
	return func() (goal.Actor, int64, error) {
		machine, err := resolveMachine(root)
		if err != nil {
			return goal.Actor{}, 0, fmt.Errorf("the seat machine could not be resolved: %w", err)
		}
		holder, err := lease.CurrentHolder(root)
		if err != nil {
			return goal.Actor{}, 0, fmt.Errorf("the announced checkout holder could not be resolved: %w", err)
		}
		if mainID == "" || holder.MainId != mainID {
			return goal.Actor{}, 0, fmt.Errorf("the Stop main %q does not match the announced checkout holder %q", mainID, holder.MainId)
		}
		if holder.SessionId == "" || holder.OwnerLineage == "" {
			return goal.Actor{}, 0, errors.New("the session holding this checkout hasn't announced itself; start it with metasystem session start")
		}
		return goal.Actor{Machine: machine, Lineage: holder.OwnerLineage}, holder.ClaimEpoch, nil
	}
}

func prepareSeatIdleContinuation(root string) func(goal.IdleEscalationEvent) (string, error) {
	return func(event goal.IdleEscalationEvent) (string, error) {
		roster, err := dispatchpkg.ResolveRoster(dispatchpkg.RosterParams{
			ConfPath: filepath.Join(root, "metasystem.conf"),
			Role:     "steward-continuation", Mode: "build",
		})
		if err != nil {
			return "", fmt.Errorf("steward continuation roster could not be resolved: %w", err)
		}
		raw := make([]byte, 8)
		if _, err := rand.Read(raw); err != nil {
			return "", fmt.Errorf("steward continuation nonce could not be minted: %w", err)
		}
		nonce := hex.EncodeToString(raw)
		intent, err := steward.StageIntent(root, nonce, event.GoalID, "steward-"+nonce,
			roster.Runtime, roster.Model, "seatIdle")
		if err != nil {
			return "", err
		}
		intent.ClaimNeeded = event.ClaimNeeded
		intent.SeatActor = &steward.SeatActor{
			Machine: event.ClaimActor.Machine,
			Lineage: event.ClaimActor.Lineage,
		}
		intent.SeatClaimEpoch = event.SeatClaimEpoch
		if err := steward.PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), intent); err != nil {
			return "", err
		}
		return nonce, nil
	}
}

func recordSeatIdleIncident(root string, now time.Time) func(goal.IdleEscalationEvent) (string, error) {
	return func(event goal.IdleEscalationEvent) (string, error) {
		actor := ""
		if event.ClaimActor.Machine != "" || event.ClaimActor.Lineage != "" {
			actor = event.ClaimActor.Machine + "+" + event.ClaimActor.Lineage
		}
		episode, err := steward.RecordSeatIdleIncident(root, steward.SeatIdleIncident{
			SessionID: event.SessionID, MainID: event.MainID, GoalID: event.GoalID,
			BacklogDigest: event.BacklogDigest, Refusal: event.Refusal,
			StopHookActive: event.StopHookActive, ClaimActor: actor,
			ClaimNeeded: event.ClaimNeeded, SeatClaimEpoch: event.SeatClaimEpoch,
			ClaimMade: event.ClaimMade, ClaimDetail: event.ClaimDetail,
			IntentID: event.IntentID, IntentPrepared: event.IntentPrepared, IntentDetail: event.IntentDetail,
		}, now)
		if err != nil {
			return "", err
		}
		return episode.EpisodeID, nil
	}
}

func raiseSeatIdleAlarm(root string) func(goal.IdleEscalationEvent) error {
	return func(event goal.IdleEscalationEvent) error {
		verdict := steward.VerdictIdleBacklogDead
		return steward.QueueNotification(root, steward.PendingNotification{
			Nonce:   "verdict-" + string(verdict),
			Message: fmt.Sprintf("steward: %s — seat %s reached idle refusal %d, but no steward continuation intent could be prepared: %s; %s", verdict, event.SessionID, event.Refusal, event.ClaimDetail, event.IntentDetail),
		})
	}
}
