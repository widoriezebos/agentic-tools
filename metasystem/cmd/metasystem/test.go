package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/digest"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

func runTestWorkerCapabilities(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		return refuseUnknownOption(stdout, stderr, "test worker-capabilities", args[0], "it takes no options")
	}
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: metasystem internal test worker-capabilities")
		return 2
	}
	if err := testrun.WriteWorkerCapabilities(stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func runTestGroups(args []string, stdout, stderr io.Writer) int {
	return runTestGroupsWithEnvironment(args, os.Environ(), stdout, stderr)
}

func runTestGroupsWithEnvironment(args, environment []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("test groups", stdout, stderr)
	root := pathFlag(flags, "root", ".", "MetaSystem installation root")
	environmentOnly := flags.Bool("environment", false, "describe the landing environment without running groups when no ids are supplied")
	// Public options may follow the ids, as the command's usage promises.
	rootValue, rootGiven, rest := takeIntentFlag(args, "root", true)
	envValue, envGiven, ids := takeIntentFlag(rest, "environment", false)
	var options []string
	if rootGiven {
		options = append(options, "--root", rootValue)
	}
	if envGiven {
		options = append(options, "--environment="+envValue)
	}
	if flags.Parse(append(options, ids...)) != nil {
		return 2
	}
	ids = flags.Args()
	for _, id := range ids {
		if strings.HasPrefix(id, "-") {
			fmt.Fprintf(stderr, "test groups: unknown option %s\n", id)
			return 2
		}
	}
	if len(ids) == 0 && !*environmentOnly {
		fmt.Fprintln(stderr, "test groups: no testing group ids supplied")
		return 2
	}
	installation, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	// Configuration locates the installation without Git or a goal ledger.
	for {
		if _, err := os.Stat(filepath.Join(installation, "metasystem.conf")); err == nil {
			break
		}
		if config.TemplateMode(filepath.Join(installation, "metasystem")) {
			installation = filepath.Join(installation, "metasystem")
			break
		}
		parent := filepath.Dir(installation)
		if parent == installation {
			break
		}
		installation = parent
	}
	ctx := context.Background()
	description, err := proofrun.LandingEnvironment(ctx, installation, environment)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, "landing environment "+description)
	if len(ids) == 0 {
		return 0
	}
	installation, contract, _, err := testrun.LoadContract(installation)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return runNamedTestGroups(installation, contract, ids, environment, nil, stdout, stderr)
}

func runNamedTestGroups(installation string, contract testpolicy.Contract, ids, environment []string, units map[string]string, stdout, stderr io.Writer) int {
	var progress func(string, int, []proofrun.PackageExecution)
	if units != nil {
		progress = func(id string, planned int, completed []proofrun.PackageExecution) {
			if planned > 0 {
				fmt.Fprintf(stdout, "landing planned %d\n", planned)
			}
			for _, execution := range completed {
				unit := execution.Package
				if units[id] != "" {
					unit = units[id]
				}
				ms := int64(0)
				if execution.ElapsedMS != nil {
					ms = *execution.ElapsedMS
				}
				fmt.Fprintf(stdout, "landing package %s %d %s %d\n", unit, execution.Shard, execution.Status, ms)
			}
		}
	}
	results, runErr := proofrun.RunNamedGroups(context.Background(), installation, contract, ids, environment, progress)
	exit := 0
	for _, result := range results {
		fmt.Fprintf(stdout, "landing group %s %s %d\n", result.ID, result.Status, result.DurationMS)
		for _, reason := range result.Reasons {
			fmt.Fprintf(stdout, "landing group %s reason %s\n", result.ID, strings.Join(strings.Fields(reason), " "))
		}
		fmt.Fprint(stderr, result.Output)
		if result.Status != "green" {
			exit = 1
		}
	}
	if runErr != nil {
		fmt.Fprintln(stderr, runErr)
		if units != nil {
			fmt.Fprintln(stdout, "LANDING-NOT-RUN\tenvironment")
		}
		return 1
	}
	if units != nil {
		failed := 0
		for _, result := range results {
			if result.Status == "green" {
				continue
			}
			unit := result.ID
			if units[unit] != "" {
				unit = units[unit]
			}
			names := []string{}
			for _, reason := range result.Reasons {
				if name, ok := strings.CutPrefix(reason, "failed test "); ok {
					if i := strings.Index(name, ".Test"); units[result.ID] != "" && i >= 0 {
						name = name[i+1:]
					}
					names = append(names, name)
				}
			}
			fmt.Fprintf(stdout, "LANDING-FAILED\t%s\t%s\n", unit, strings.Join(names, " "))
			failed++
		}
		fmt.Fprintf(stdout, "LANDING-CHECKED\t%d\n", failed)
	}
	return exit
}

func runTestList(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("test list", stdout, stderr)
	root := pathFlag(flags, "root", "", "MetaSystem installation root")
	jsonOutput := flags.Bool("json", false, "emit structured JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 || !requireFlags(flags, stderr, "root") {
		fmt.Fprintln(stderr, "metasystem test list --help shows its forms and options")
		return 2
	}
	installation, contract, path, err := testrun.LoadContract(*root)
	if err != nil {
		page := passthroughPage(stderr, *root, false)
		if _, statErr := os.Stat(*root); errors.Is(statErr, fs.ErrNotExist) {
			page.Refusal(page.Env().Path(*root)+" does not exist; nothing was listed",
				textui.Hint{Argv: []string{"metasystem", "test", "list"}, Reason: "run it inside the checkout, or name it with --repo"})
		} else {
			page.Refusal("the testing contract cannot be read: "+err.Error(), textui.Hint{Argv: []string{"metasystem", "settings", "check"}, Reason: "names what is wrong"})
		}
		printPage(stderr, page)
		return 1
	}
	if *jsonOutput {
		writeJSONLine(stdout, stderr, map[string]any{"schemaVersion": 1, "installation": installation, "contract": path, "groups": contract.Groups})
		return 0
	}
	page := passthroughPage(stdout, installation, false)
	page.Headline(textui.Count(len(contract.Groups), "test group", "test groups")+" in the testing contract", page.Env().Path(path))
	table := page.Section("", "").Table(textui.Column{Title: "GROUP"}, textui.Column{Title: "KIND"}, textui.Column{Title: "ADAPTER", Flex: true})
	for _, group := range contract.Groups {
		table.Row(textui.Plain(group.ID), textui.Plain(group.Kind), textui.Plain(group.Adapter))
	}
	printPage(stdout, page)
	return 0
}

func runTestPlan(args []string, stdout, stderr io.Writer) int {
	return runTestPlanAs("test plan", args, stdout, stderr)
}

// runTestPlanAs is test plan answering as name: the public action, or the
// internal entrypoint a pinned engine is started with.
func runTestPlanAs(name string, args []string, stdout, stderr io.Writer) int {
	request, jsonOutput, status := parseTestingSelection(name, args, false, stdout, stderr)
	if status != 0 {
		return status
	}
	request.LandedRearm = !request.PolicyChild
	prepared, err := prepareTestingForCommand(request)
	if err != nil {
		// The refusal a person reads; --verbose adds its detail (code, cause
		// and facts), and --json the envelope a planning child's parent
		// reads its code from.
		printTestingRefusalAs(stderr, err, request, jsonOutput)
		if jsonOutput {
			_ = verbresult.Write(stdout, verbresult.FromError(name, 1, err, nil))
		}
		return 1
	}
	output := testrun.PlanOutputOf(prepared)
	if jsonOutput {
		// The plan is the envelope's data (R1).
		result := verbresult.FromError(name, 0, nil, output)
		result.Summary = "the tests to run are chosen"
		_ = verbresult.Write(stdout, result)
	} else {
		page := passthroughPage(stdout, prepared.Installation, request.Verbose)
		layTestPlan(page, output)
		printPage(stdout, page)
	}
	return 0
}

// layTestPlan is a test plan as a person reads it: the groups that would
// run on the tree and in which mode, the ones it must run, why others are
// left out (--verbose), and the inputs that match no file.
func layTestPlan(page *textui.Page, output testrun.PlanOutput) {
	plan := output.Plan
	mode := "mode " + string(plan.ExecutedMode)
	if plan.RequiredMode != "" && plan.RequiredMode != plan.ExecutedMode {
		mode += " (" + string(plan.RequiredMode) + " required)"
	}
	page.Headline(textui.Count(len(plan.SelectedGroups), "test group", "test groups")+" would run on tree "+textui.SHA(output.CandidateTree), mode)
	required := map[string]bool{}
	for _, id := range plan.RequiredGroups {
		required[id] = true
	}
	var must, also []string
	for _, id := range plan.SelectedGroups {
		if required[id] {
			must = append(must, id)
		} else {
			also = append(also, id)
		}
	}
	section := page.Section("Groups", "")
	if len(must) > 0 {
		section.KV("required", textui.Plain(strings.Join(must, ", ")))
	}
	if len(also) > 0 {
		section.KV("also", textui.Plain(strings.Join(also, ", ")))
	}
	if len(plan.AffectedSurfaces) > 0 {
		section.KV("surfaces", textui.Plain(strings.Join(plan.AffectedSurfaces, ", ")))
	}
	for _, uncertain := range plan.Uncertainty {
		section.KV("unsure", textui.Plain(uncertain))
	}
	if page.Verbose() && len(plan.Omissions) > 0 {
		left := page.Section("Left out", "")
		for _, omission := range plan.Omissions {
			left.KV(omission.Group, textui.Plain(omission.Reason))
		}
	}
	if len(output.UnmatchedInputs) > 0 {
		unmatched := page.Section("Inputs that match no file", "")
		for _, item := range output.UnmatchedInputs {
			unmatched.KV(item.Group, textui.Plain(item.Pattern))
		}
	}
}

func admitTestingRun(request testrun.SelectionRequest, admission proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	return admitTestingRunWith(request, admission, admitProofLaunch)
}

func admitTestingRunWith(request testrun.SelectionRequest, admission proofLaunchAdmission,
	admit func(proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error),
) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	admission.ForceAttempt = request.Purpose == testpolicy.PurposeCadence || request.NoReuse || request.ForceGroups
	if admission.CandidateTree == "" {
		admission.CandidateTree = request.Tree
	}
	return admit(admission)
}

type testingCommandAdmission struct {
	now        func() time.Time
	admitRun   func(testrun.SelectionRequest, proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error)
	admitProof func(proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error)
}

func (admission testingCommandAdmission) initial(request testrun.SelectionRequest, launch proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	launch.Now = admission.now()
	return admission.admitRun(request, launch)
}

func (admission testingCommandAdmission) forced(launch proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	launch.ForceAttempt = true
	launch.ForceGroups = true
	launch.Now = admission.now()
	return admission.admitProof(launch)
}

func parseTestingSelection(name string, args []string, execution bool, stdout, stderr io.Writer) (testrun.SelectionRequest, bool, int) {
	flags := newFlagSet(name, stdout, stderr)
	// One command is one invocation: its preparations share one state.
	request := testrun.SelectionRequest{Preparation: &testrun.PreparationState{}, Notes: stderr}
	pathFlagVar(flags, &request.Root, "root", "", "MetaSystem installation root")
	flags.StringVar(&request.GoalID, "goal", "", "accepted goal owning delivery")
	flags.StringVar(&request.AuthorityGoalID, "authority", "", "claimed goal authorizing the proof reservation")
	flags.StringVar(&request.Tree, "tree", "", "exact whole-project candidate tree")
	mode := flags.String("mode", "auto", "auto, standard, deep, or diagnostic canary")
	purpose := flags.String("purpose", "delivery", "delivery, diagnostic, or cadence")
	groups := flags.String("groups", "", "comma-separated diagnostic groups")
	batchRequirements := flags.String("batch-requirements", "", "strict JSON batch delivery requirements")
	jsonOutput := flags.Bool("json", false, "emit structured JSON")
	flags.BoolVar(&request.Verbose, "verbose", false, "also print the details behind a refusal: its code and the facts it saw")
	flags.BoolVar(&request.PolicyChild, "policy-child", false, "judge policy in the pinned child without re-arming")
	flags.BoolVar(&request.BatchPrefixReceipt, "batch-prefix", false, "compose delivery evidence for a batch prefix")
	flags.BoolVar(&request.Carried, "carried", false, "compose a completed red result for carried-landing classification")
	flags.StringVar(&request.FreshEpisode, "fresh-episode", "", "retained freshness episode for a proof decision")
	flags.StringVar(&request.FreshExpiresAt, "fresh-expires-at", "", "expiry for a retained freshness episode")
	if execution {
		pathFlagVar(flags, &request.ControlRoot, "control-root", "", "durable proof control root for an internal batch proof")
		flags.BoolVar(&request.BatchTipProof, "batch-tip", false, "prove a batch tip projected into its own detached worktree")
		flags.BoolVar(&request.BatchAdmission, "batch-admission", false, "run selected batch admission checks on an exact tree")
		flags.StringVar(&request.CapMin, "cap-min", "", "reserved proof minutes")
		flags.StringVar(&request.RetryDecision, "retry-decision", "", "accountable version-1 retry decision")
		flags.StringVar(&request.ResultPath, "result", "", "atomic result projection path")
		flags.BoolVar(&request.ForceGroups, "force-groups", false, "execute every selected group regardless of retained evidence")
		flags.Uint64Var(&request.ExpectedGoalRevision, "expected-goal-revision", 0, "sealed goal revision")
		flags.Uint64Var(&request.ExpectedAccountingRevision, "expected-accounting-revision", 0, "sealed accounting revision")
		flags.BoolVar(&request.NoReuse, "no-reuse", false, "execute diagnostic groups freshly")
		flags.BoolVar(&request.RequireDiagnosticHeadroom, "require-diagnostic-headroom", false, "reserve the mandatory batch-tip diagnostic")
		flags.BoolVar(&request.AllGroups, "all-groups", false, "run every selected delivery group after a failure")
		flags.StringVar(&request.AppAddress, "app-address", "", "the address of the application run a named group is run against")
	}
	if flags.Parse(args) != nil || !requireFlags(flags, stderr, "root") || flags.NArg() != 0 || request.Root == "" {
		if _, public := publicCommand(name); public {
			fmt.Fprintf(stderr, "metasystem %s --help shows its forms and options\n", name)
		} else {
			fmt.Fprintf(stderr, "usage: metasystem internal %s --root INSTALLATION [--goal GOAL] [--authority GOAL] [--tree TREE]\n"+
				"  [--mode auto|standard|deep|canary] [--purpose delivery|diagnostic|cadence] [--groups GROUP,GROUP]\n", name)
		}
		return request, false, 2
	}
	if request.PolicyChild && (strings.TrimPrefix(name, "internal ") != "test plan" || execution) {
		fmt.Fprintln(stderr, "--policy-child is internal to the pinned test plan child")
		return request, false, 2
	}
	purposeSet := false
	flags.Visit(func(value *flag.Flag) { purposeSet = purposeSet || value.Name == "purpose" })
	if *mode == string(testpolicy.ModeCanary) && !purposeSet {
		*purpose = string(testpolicy.PurposeDiagnostic)
	}
	if *groups != "" {
		for _, id := range strings.Split(*groups, ",") {
			if strings.TrimSpace(id) == "" || id != strings.TrimSpace(id) {
				fmt.Fprintln(stderr, "diagnostic groups must be a comma-separated list of exact identifiers")
				return request, false, 2
			}
			request.Groups = append(request.Groups, id)
		}
	}
	request.Mode, request.Purpose = testpolicy.Mode(*mode), testpolicy.Purpose(*purpose)
	requirementsSet, groupsSet := false, false
	flags.Visit(func(value *flag.Flag) {
		requirementsSet = requirementsSet || value.Name == "batch-requirements"
		groupsSet = groupsSet || value.Name == "groups"
	})
	if requirementsSet {
		if !request.BatchPrefixReceipt || request.Purpose != testpolicy.PurposeDelivery || groupsSet {
			fmt.Fprintln(stderr, "--batch-requirements requires delivery --batch-prefix and cannot be combined with --groups")
			return request, false, 2
		}
		var err error
		request.BatchRequirements, err = testrun.ParseBatchRequirements(*batchRequirements)
		if err != nil {
			fmt.Fprintln(stderr, "invalid --batch-requirements:", err)
			return request, false, 2
		}
	}
	if request.BatchPrefixReceipt && groupsSet {
		fmt.Fprintln(stderr, "--groups is diagnostic-only and cannot be combined with --batch-prefix")
		return request, false, 2
	}
	if request.BatchPrefixReceipt && request.Purpose != testpolicy.PurposeDelivery {
		fmt.Fprintln(stderr, "--batch-prefix requires delivery purpose")
		return request, false, 2
	}
	if request.BatchTipProof && request.Purpose != testpolicy.PurposeDelivery {
		fmt.Fprintln(stderr, "--batch-tip requires delivery purpose")
		return request, false, 2
	}
	if request.BatchAdmission && request.Purpose != testpolicy.PurposeDelivery {
		fmt.Fprintln(stderr, "--batch-admission requires delivery purpose")
		return request, false, 2
	}
	// Both internal batch proofs execute in a detached worktree holding the
	// exact tree they name, so both direct durable writes back at the control
	// root that owns them. batchPrefixProofControlRoot is what makes that safe:
	// it admits a control root only when the execution root is a linked
	// worktree sharing its git common directory and prefix.
	if request.ControlRoot != "" && !request.BatchPrefixReceipt && !request.BatchTipProof && !request.BatchAdmission {
		fmt.Fprintln(stderr, "--control-root is only for the test runs of a batch")
		return request, false, 2
	}
	if request.NoReuse && request.Purpose != testpolicy.PurposeDiagnostic {
		fmt.Fprintln(stderr, "--no-reuse is available only for diagnostic purpose")
		return request, false, 2
	}
	if request.FreshEpisode != "" {
		if len(request.FreshEpisode) != 64 {
			fmt.Fprintln(stderr, "--fresh-episode must be a 64-digit hexadecimal identifier")
			return request, false, 2
		}
		if _, err := hex.DecodeString(request.FreshEpisode); err != nil {
			fmt.Fprintln(stderr, "--fresh-episode must be a 64-digit hexadecimal identifier")
			return request, false, 2
		}
	}
	if request.FreshExpiresAt != "" {
		if request.FreshEpisode == "" {
			fmt.Fprintln(stderr, "--fresh-expires-at requires --fresh-episode")
			return request, false, 2
		}
		if _, err := time.Parse(time.RFC3339Nano, request.FreshExpiresAt); err != nil {
			fmt.Fprintln(stderr, "--fresh-expires-at must be an RFC3339 timestamp")
			return request, false, 2
		}
	}
	if request.RequireDiagnosticHeadroom && request.Purpose != testpolicy.PurposeDelivery {
		fmt.Fprintln(stderr, "--require-diagnostic-headroom is available only for delivery purpose")
		return request, false, 2
	}
	if request.AppAddress != "" && (request.Purpose != testpolicy.PurposeDiagnostic || len(request.Groups) != 1) {
		fmt.Fprintln(stderr, "--app-address belongs to one named diagnostic group: --mode canary --groups GROUP")
		return request, false, 2
	}
	if request.AllGroups && request.Purpose != testpolicy.PurposeDelivery {
		fmt.Fprintln(stderr, "--all-groups is available only for delivery purpose")
		return request, false, 2
	}
	if (request.ExpectedGoalRevision == 0) != (request.ExpectedAccountingRevision == 0) {
		fmt.Fprintln(stderr, "expected goal and accounting revisions must be supplied together")
		return request, false, 2
	}
	return request, *jsonOutput, 0
}

var prepareTestingForCommand = testrun.Prepare

// printUnmatchedInputsTo names, for a test run, the group inputs that match
// no file.
func printUnmatchedInputsTo(stderr io.Writer, values []testrun.UnmatchedInput) {
	for _, item := range values {
		fmt.Fprintf(stderr, "TEST-INPUT-NO-MATCH group=%q pattern=%q\n", item.Group, item.Pattern)
	}
}

func resolveTestingPreparationWorkerPolicy(prepared *testrun.Preparation) (proofRunLimits, error) {
	limits, err := resolveProofRunLimits(prepared.ConfPath)
	if err != nil {
		return proofRunLimits{}, err
	}
	prepared.Workers, prepared.AdmissionMaximum = limits.workers, limits.admissionMaximum
	return limits, nil
}

// testingWorkerPolicy is the proof-run limits' worker allowance as the
// testing selection reads it.
func testingWorkerPolicy(confPath string) (testrun.WorkerPolicy, error) {
	limits, err := resolveProofRunLimits(confPath)
	if err != nil {
		return testrun.WorkerPolicy{}, err
	}
	return testrun.WorkerPolicy{Workers: limits.workers, AdmissionMaximum: limits.admissionMaximum,
		Automatic: limits.automaticWorkers, AutomaticCeiling: limits.automaticWorkerCeiling}, nil
}

func runTestRun(args []string, stdout, stderr io.Writer) (exit int) {
	invocation := testRunInvocation{callerPID: int64(os.Getppid()), stdout: stdout, stderr: stderr, name: "internal test run"}
	finish := invocation.envelope(invocation.name, args)
	defer func() { finish(exit) }()
	// The entry supplies its own caller, as it always did.
	return runTestRunWith(invocation, args)
}

// testRunInvocation is the explicit context of one test run (design 6.2): the
// process its proof admission classifies and its nested-proof authentication
// starts from, and the streams its result and progress go to. A command that
// runs the test runner in its own process supplies itself, the parent the
// former child classified.
type testRunInvocation struct {
	callerPID      int64
	stdout, stderr io.Writer
	// Native waits share the invocation's cancellation and semantic clock.
	nativeContext context.Context
	nativeClock   func() time.Time
	// name is the command it answers as (default: test run).
	name string
	// outcome collects what the --json envelope reports; nil without --json.
	outcome *testRunOutcome
}

// testRunOutcome is what a test run under --json reports in its one
// envelope: the error it ended on (its code and plain reason), and its data,
// the result it would have printed.
type testRunOutcome struct {
	err     error
	data    any
	results bytes.Buffer
}

// testRunFailure is a line the run printed for a person, kept with the
// error behind it so the envelope carries that error's code.
type testRunFailure struct {
	text  string
	cause error
}

func (failure *testRunFailure) Error() string { return failure.text }
func (failure *testRunFailure) Unwrap() error { return failure.cause }

// envelope makes the invocation answer with one --json envelope when args
// ask for it: the run's own stream (banner, the suite's output, the result
// summary) goes to stderr, and stdout holds only the envelope, written by
// the returned function with the exit status. Without --json, or when an
// outer call already answers, it does nothing.
func (invocation *testRunInvocation) envelope(name string, args []string) func(int) {
	if invocation.outcome != nil || !testRunWantsJSON(args) {
		return func(int) {}
	}
	outcome := &testRunOutcome{}
	stdout := invocation.stdout
	invocation.stdout, invocation.outcome = invocation.stderr, outcome
	return func(exit int) {
		data := outcome.data
		if data == nil && json.Valid(bytes.TrimSpace(outcome.results.Bytes())) {
			data = json.RawMessage(bytes.TrimSpace(outcome.results.Bytes()))
		}
		result := verbresult.FromError(name, exit, outcome.err, data)
		if result.Summary == "" {
			result.Summary = testRunSummary(exit)
		}
		_ = verbresult.Write(stdout, result)
	}
}

// testRunWantsJSON reports whether the run's arguments ask for --json.
func testRunWantsJSON(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--json", "-json", "--json=true", "-json=true":
			return true
		case "--":
			return false
		}
	}
	return false
}

// testRunSummary is the envelope's plain line for a run that recorded no
// error of its own.
func testRunSummary(exit int) string {
	switch exit {
	case 0:
		return "the selected tests passed"
	case proofrun.ExitReusableSuccess:
		return "the selected tests passed before on this tree; that result stands"
	case proofrun.ExitLiveDuplicate:
		return "the same test run is already running"
	case 2:
		return "the test run's options were refused; the reason is on stderr"
	}
	return fmt.Sprintf("the test run ended with status %d; the reason is on stderr", exit)
}

// fail prints a line for a person on stderr and, under --json, keeps it
// (with the error it ends on) as the envelope's reason.
func (invocation testRunInvocation) fail(words ...any) {
	fmt.Fprintln(invocation.stderr, words...)
	if invocation.outcome == nil || len(words) == 0 {
		return
	}
	cause, _ := words[len(words)-1].(error)
	invocation.outcome.err = &testRunFailure{text: strings.TrimSpace(fmt.Sprintln(words...)), cause: cause}
}

// record keeps err as the envelope's reason under --json.
func (invocation testRunInvocation) record(err error) {
	if invocation.outcome != nil && err != nil {
		invocation.outcome.err = err
	}
}

// results is where the run prints its result: stdout, or under --json the
// envelope's data.
func (invocation testRunInvocation) results() io.Writer {
	if invocation.outcome != nil {
		return &invocation.outcome.results
	}
	return invocation.stdout
}

// noChildResult is where a run that started nothing prints its decision's
// PROOF-RESULT line: stdout, or under --json nowhere but the result file,
// the decision being the envelope's data (R2).
func (invocation testRunInvocation) noChildResult(decision proofrun.LaunchResult) io.Writer {
	if invocation.outcome != nil {
		invocation.outcome.data = decision
		return nil
	}
	return invocation.stdout
}

// testRunRetryRequired refuses a run whose earlier attempt on the same
// inputs failed: a retry names that attempt with a decision (R3).
func testRunRetryRequired(decision proofrun.LaunchResult) error {
	return &refusal.Coded{Code: "TEST_RETRY_REQUIRED", Facts: "prior=" + decision.PriorAttempt,
		Reason: fmt.Errorf("the same tests failed in run %s; a retry names that run with a retry decision", decision.PriorAttempt)}
}

func runTestRunWith(invocation testRunInvocation, args []string) (exit int) {
	commandStarted := time.Now().UTC()
	name := invocation.name
	if name == "" {
		name = "test run"
	}
	finish := invocation.envelope(name, args)
	defer func() { finish(exit) }()
	request, _, status := parseTestingSelection(name, args, true, invocation.stdout, invocation.stderr)
	if status != 0 {
		return status
	}
	request.LandedRearm = true
	request.RequireWorkerCapabilities = true
	request.CallerPID = invocation.callerPID
	prepared, err := prepareTestingForCommand(request)
	if err != nil {
		printTestingRefusal(invocation.stderr, err, request)
		invocation.record(err)
		if errors.Is(err, testrun.ErrWorkerPolicyUnsupported) {
			return proofrun.ExitAdmissionRefused
		}
		return 1
	}
	printUnmatchedInputsTo(invocation.stderr, prepared.UnmatchedInputs)
	controlRoot := prepared.ProofControlRoot()
	commandClock, fixtureClock, err := goalCommandClock(controlRoot)
	if err != nil {
		invocation.fail("metasystem test run:", err)
		return proofrun.ExitAdmissionRefused
	}
	// Every disposable byte of this invocation lands in one recorded root,
	// removed after its writers provably drained; earlier crashed roots are
	// recovered first, never this run's own.
	scratch, err := proofrun.CreateScratchRun(controlRoot)
	if err != nil {
		invocation.fail("metasystem test run: scratch root:", err)
		return 1
	}
	defer func() { exit = finishTestingScratch(invocation.stderr, scratch, exit) }()
	for _, outcome := range proofrun.ReconcileScratch(controlRoot, proofrun.ScratchOptions{Self: scratch.ID()}) {
		if outcome.Action != proofrun.ReconcileScratchPending {
			fmt.Fprintf(invocation.stderr, "metasystem test run: %s: %s: %s\n", outcome.AttemptID, outcome.Action, outcome.Reason)
		}
	}
	scratchContext := proofrun.WithScratchRun(context.Background(), scratch)
	semanticCommandStarted := commandClock()
	commandAdmission := testingCommandAdmission{now: commandClock, admitRun: admitTestingRun, admitProof: admitProofLaunch}
	limits, err := resolveTestingPreparationWorkerPolicy(&prepared)
	if err != nil {
		invocation.fail("metasystem test run:", err)
		return 1
	}
	engine, err := os.Executable()
	if err != nil {
		invocation.fail("metasystem test run:", err)
		return 1
	}
	workerEngine := engine
	if !prepared.FirstTestingTransition {
		workerEngine = prepared.PolicyEngine
	}
	if !prepared.WorkerCapabilitiesChecked {
		capabilityContext, cancelCapabilities := context.WithCancel(context.Background())
		workerCapabilities, capabilityErr := testrun.ReadWorkerCapabilities(capabilityContext, prepared, engine)
		cancelCapabilities()
		if capabilityErr != nil {
			invocation.fail("metasystem test run:", capabilityErr)
			return proofrun.ExitAdmissionRefused
		}
		prepared.WorkerCapabilitiesChecked, prepared.WorkerScratchPolicies = true, workerCapabilities.ScratchEnvironmentPolicies
		prepared.WorkerResultSchemas = workerCapabilities.TestResultSchemaVersions
	}
	unmark, markErr := proofrun.MarkManagedProofProcess()
	if markErr != nil {
		invocation.fail("metasystem test run: mark host admission process:", markErr)
		return proofrun.ExitAdmissionRefused
	}
	defer unmark()
	buildContext, cancelBuild := context.WithCancel(scratchContext)
	engineIO := candidateengine.Native()
	engineIO.Notes = invocation.stderr
	candidateEngine, err := candidateengine.Prepare(buildContext, controlRoot, gittree.Workspace{Dir: prepared.ProjectRoot}, prepared.Prefix,
		prepared.CandidateTree, testrun.InheritedEnvironment(prepared.Environment, os.Environ()), func() error {
			return refuseKnownColdBuildBudget(prepared, request)
		}, engineIO)
	cancelBuild()
	if err != nil {
		invocation.fail("metasystem test run:", err)
		var budgetRefusal *coldBuildBudgetRefusal
		if errors.As(err, &budgetRefusal) {
			return proofrun.ExitAdmissionRefused
		}
		return 1
	}
	defer candidateEngine.Close()
	planDigest := proofrun.TestPlanDigest(prepared.EffectiveContract, prepared.Plan, prepared.CandidateTree)
	manifestDigest, err := testrun.CandidateManifest(gittree.Workspace{Dir: prepared.ProjectRoot}, prepared.CandidateTree, scratch)
	if err != nil {
		invocation.fail("metasystem test run: capture candidate manifest:", err)
		return proofrun.ExitAdmissionRefused
	}
	preRequest := testrun.RunRequest(prepared, "", "", candidateEngine.Path, candidateEngine.Digest, candidateEngine.Commit)
	if err := testrun.PrepareScratch(context.Background(), &preRequest, scratch, prepared); err != nil {
		invocation.fail("metasystem test run: scratch environment:", err)
		return proofrun.ExitAdmissionRefused
	}
	scratchEnvironment := preRequest.ScratchEnvironment
	metadataContext, cancelMetadata := context.WithCancel(scratchContext)
	metadataLease, leaseErr := proofrun.AcquireHostResources(metadataContext, controlRoot, prepared.ConfPath, "heavy", nil)
	if leaseErr != nil {
		cancelMetadata()
		invocation.fail("metasystem test run: admit testing metadata preparation:", leaseErr)
		return proofrun.ExitAdmissionRefused
	}
	queueDurationMS := candidateEngine.QueueDurationMS + metadataLease.Waited().Milliseconds()
	metadataStarted := time.Now()
	identities, preparedGroups, preparationLaunches, identityErr := proofrun.PrepareGroupExecutionIdentities(
		proofrun.WithHostResourceLease(metadataContext, metadataLease), preRequest)
	closeLeaseErr := metadataLease.Close()
	cancelMetadata()
	if identityErr == nil {
		identityErr = closeLeaseErr
	}
	preparationDuration := time.Since(metadataStarted).Milliseconds()
	preRequest.PreparedGroups, preRequest.PreparationLaunches = preparedGroups, preparationLaunches
	preRequest.PreparationDurationMS = preparationDuration
	preRequest.QueueDurationMS = queueDurationMS
	preRequest.CommandStartedAt = commandStarted.Format(time.RFC3339Nano)
	var identityInputs []string
	if identityErr == nil {
		for _, id := range prepared.Plan.SelectedGroups {
			identityInputs = append(identityInputs, "group:"+id+":"+identities[id])
		}
	} else {
		identityInputs = append(identityInputs, "group-identity-unavailable:"+identityErr.Error())
	}
	if identityErr != nil {
		invocation.fail("metasystem test run: bounded testing metadata preparation:", identityErr)
		return proofrun.ExitAdmissionRefused
	}
	freshGroups, maxFreshAge := testrun.FreshGroups(prepared, request)
	if request.FreshEpisode == "" && len(freshGroups) != 0 {
		request.FreshEpisode, err = testrun.NewFreshEpisode()
		if err != nil {
			invocation.fail("metasystem test run: create freshness episode:", err)
			return proofrun.ExitAdmissionRefused
		}
		if maxFreshAge > 0 && request.FreshExpiresAt == "" {
			request.FreshExpiresAt = semanticCommandStarted.Add(maxFreshAge).Format(time.RFC3339Nano)
		}
	}
	if maxFreshAge > 0 && request.FreshExpiresAt == "" {
		invocation.fail("metasystem test run: selected fresh group requires --fresh-expires-at")
		return proofrun.ExitAdmissionRefused
	}
	preRequest.FreshnessEpisode, preRequest.FreshnessExpiresAt = request.FreshEpisode, request.FreshExpiresAt
	preRequest.FreshGroups = freshGroups
	if err := testrun.BindFreshnessProjection(&preRequest, prepared.Installation); err != nil {
		invocation.fail("metasystem test run: project freshness candidate:", err)
		return proofrun.ExitAdmissionRefused
	}
	freshnessProjection := preRequest.FreshnessCandidateProjection
	freshBinding := testrun.FreshnessBinding(preRequest, identities, request.FreshEpisode)
	preRequest.FreshnessBinding = freshBinding
	admission := proofLaunchAdmission{ControlRoot: controlRoot,
		ExecutionRoot: prepared.ProjectRoot, ConfPath: prepared.ConfPath, GoalID: request.GoalID, AuthorityGoalID: request.AuthorityGoalID,
		CandidateRevision: prepared.AccountingRevision, RetryDecision: request.RetryDecision,
		CapMin: request.CapMin, ExpectedGoalRevision: request.ExpectedGoalRevision,
		ExpectedAccountingRevision: request.ExpectedAccountingRevision,
		ScopeClass:                 "selected", CommandClass: "testing", CandidateTree: prepared.CandidateTree, IdentityInputs: append([]string{prepared.ContractDigest,
			prepared.BaseContractDigest, prepared.PolicyEngineDigest, candidateEngine.Digest, prepared.BehaviorPolicyDigest, planDigest}, identityInputs...), Environment: prepared.Environment,
		SharedEngine: engine, SharedManifestDigest: manifestDigest, ComponentIdentities: identities,
		FreshnessEpisode: request.FreshEpisode, FreshnessBinding: freshBinding, FreshnessExpiresAt: request.FreshExpiresAt,
		FreshGroups:     freshGroups,
		ForceGroups:     request.ForceGroups,
		ManagedCapacity: true,
		// A cadence attempt is the fresh sweep: it never inherits a
		// reusable-success answer from an earlier run of its goal.
		RequireDiagnosticHeadroom: request.RequireDiagnosticHeadroom,
		CallerPID:                 invocation.callerPID}
	attempt, decision, joined, err := commandAdmission.initial(request, admission)
	if err != nil {
		invocation.fail("metasystem test run:", err)
		return proofrun.ExitAdmissionRefused
	}
	if decision.Disposition == proofrun.DispositionReusableSuccess {
		attempts, readErr := proofrun.ReadAttempts(controlRoot)
		if readErr != nil || identityErr != nil {
			invocation.fail("metasystem test run: reusable component evidence is unreadable")
			return 1
		}
		template := proofrun.NewTestResultAt(preRequest, commandClock())
		projection, exact := proofrun.ExactReusableTestResult(template, attempts, identities, prepared.GoalID, prepared.AccountingRevision)
		if !exact {
			projection = proofrun.ReusedTestResult(template, attempts, identities, prepared.EffectiveContract)
		}
		if projection.Delivery.Sufficient {
			if err := publishTestingResultTo(invocation.results(), invocation.stderr, controlRoot, request.ResultPath, projection); err != nil {
				invocation.fail("metasystem test run:", err)
				return 1
			}
			return decision.ExitStatus
		}
		// The goal-scoped admission saw only this goal's successes; the seat's
		// newest observations say otherwise (a newer failure or a live plan
		// under another goal). Run afresh instead of stranding the caller.
		attempt, decision, joined, err = commandAdmission.forced(admission)
		if err != nil {
			invocation.fail("metasystem test run:", err)
			return proofrun.ExitAdmissionRefused
		}
	}
	if decision.Disposition != proofrun.DispositionExecuted {
		if decision.Reason != "" {
			invocation.fail(decision.Reason)
		}
		if decision.Disposition == proofrun.DispositionRetryRequired {
			invocation.record(testRunRetryRequired(decision))
		}
		// Under --json the decision is the envelope's data and PROOF-RESULT
		// goes only to the result file, never stdout (R2).
		if err := proofrun.EncodeResult(invocation.noChildResult(decision), request.ResultPath, decision); err != nil {
			invocation.fail("metasystem test run: publish no-child result:", err)
			return 1
		}
		return decision.ExitStatus
	}
	prepared.GoalID, prepared.AccountingRevision = attempt.AccountedGoal(), attempt.AccountedRevision()
	if err := scratch.RecordAttempt(attempt.AttemptID); err != nil {
		invocation.fail("metasystem test run: record scratch attempt:", err)
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
	}
	preRequest = testrun.RunRequest(prepared, "", "", candidateEngine.Path, candidateEngine.Digest, candidateEngine.Commit)
	if err := testrun.BindScratch(&preRequest, scratch, nil, scratchEnvironment); err != nil {
		invocation.fail("metasystem test run: scratch environment:", err)
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
	}
	preRequest.FreshnessEpisode, preRequest.FreshnessBinding, preRequest.FreshnessExpiresAt = request.FreshEpisode, freshBinding, request.FreshExpiresAt
	preRequest.FreshGroups = freshGroups
	preRequest.FreshnessCandidateProjection = freshnessProjection
	preRequest.PreparedGroups, preRequest.PreparationLaunches = preparedGroups, preparationLaunches
	preRequest.PreparationDurationMS, preRequest.CommandStartedAt = preparationDuration, commandStarted.Format(time.RFC3339Nano)
	preRequest.QueueDurationMS = queueDurationMS
	reusedGroups := map[string]proofrun.GroupResult{}
	if identityErr == nil {
		attempts, readErr := proofrun.ReadAttempts(controlRoot)
		if readErr != nil {
			invocation.fail("metasystem test run: read reusable component evidence:", readErr)
			return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
		}
		reused := proofrun.ReusedTestResultExcludingWithPolicy(proofrun.NewTestResultAt(preRequest, commandClock()), attempts, identities,
			prepared.EffectiveContract, attempt.AttemptID, proofrun.ReusePolicy{ForceGroups: request.ForceGroups})
		for _, group := range reused.Groups {
			if group.Status == "reused" {
				reusedGroups[group.ID] = group
			}
		}
	}
	pathsRoot := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", attempt.AttemptID, "testing", planDigest)
	if err := os.MkdirAll(pathsRoot, 0o700); err != nil {
		invocation.fail("metasystem test run:", err)
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
	}
	runRequest := testrun.RunRequest(prepared, attempt.AttemptID, filepath.Join(pathsRoot, "groups"), candidateEngine.Path, candidateEngine.Digest, candidateEngine.Commit)
	runRequest.FreshnessEpisode, runRequest.FreshnessBinding, runRequest.FreshnessExpiresAt = request.FreshEpisode, freshBinding, request.FreshExpiresAt
	runRequest.FreshGroups = freshGroups
	runRequest.FreshnessCandidateProjection = freshnessProjection
	runRequest.ProgressPath = filepath.Join(pathsRoot, "progress.jsonl")
	runRequest.Reused, runRequest.ComponentIdentities = reusedGroups, identities
	runRequest.PreparedGroups, runRequest.PreparationLaunches = preparedGroups, preparationLaunches
	runRequest.PreparationDurationMS, runRequest.CommandStartedAt = preparationDuration, preRequest.CommandStartedAt
	runRequest.QueueDurationMS = queueDurationMS
	runRequest.EvidenceTimeoutMS, runRequest.EvidenceMaxBytes = limits.evidenceTimeout.Milliseconds(), limits.evidenceMax
	runRequest.Concurrency = limits.concurrency
	packetPath, workerResultPath := filepath.Join(pathsRoot, "request.json"), filepath.Join(pathsRoot, "result.json")
	nativeContext := invocation.nativeContext
	if nativeContext == nil {
		nativeContext = context.Background()
	}
	nativeContext, cancelNative := context.WithCancel(nativeContext)
	defer cancelNative()
	if invocation.nativeClock != nil {
		commandClock, fixtureClock = invocation.nativeClock, true
	}
	checkAdmission := func() error {
		if err := nativeContext.Err(); err != nil {
			return err
		}
		current, err := proofrun.ReadAttempt(controlRoot, attempt.AttemptID)
		if err != nil {
			return err
		}
		if current.Terminal != nil || current.CancellationIntent != "" {
			return fmt.Errorf("the test run ended or was cancelled before its command started")
		}
		binding, err := dispatchcore.ResolveGoalBinding(controlRoot, attempt.GoalID, commandClock())
		if err != nil {
			return err
		}
		accountingRevision := binding.File.Claimed.AccountingRevision
		if accountingRevision == 0 {
			accountingRevision = binding.Revision
		}
		if binding.Revision != attempt.GoalRevision || accountingRevision != attempt.AccountingRevision || binding.Fence != nil {
			return fmt.Errorf("the goal changed before the test run's command started")
		}
		roles, err := resolveProofGoalRoles(controlRoot, attempt.AccountedGoal(), attempt.GoalID, commandClock())
		if err != nil {
			return err
		}
		if roles.CandidateRevision != attempt.AccountedRevision() || roles.Authority.Id != attempt.GoalID {
			return fmt.Errorf("the goals this test run is charged to changed before its command started")
		}
		return nil
	}
	checkWait := func() error {
		// A nested wait shares its owner's absolute execution limit.
		if attempt.ReservationOwner != nil {
			if _, _, err := proofDeadline(attempt.Deadline, commandClock); err != nil {
				return err
			}
		}
		return checkAdmission()
	}
	var retained *proofrun.TestResult
	workerEnvironment, err := testingWorkerEnvironment(prepared.Environment)
	if err != nil {
		invocation.fail("metasystem test run: export run owner:", err)
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
	}
	workerEnvironment, err = authorizedFixtureClockEnvironment(controlRoot, workerEnvironment)
	if err != nil {
		invocation.fail("metasystem test run: export fixture clock:", err)
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
	}
	workerEnvironment = resolvedTestWorkerEnvironment(workerEnvironment, limits.workers)
	producerWaitStarted := time.Now()
	for _, id := range prepared.Plan.SelectedGroups {
		if attempt.TestWaits[id] == "" {
			continue
		}
		if _, err := proofrun.WaitForTestProducerWithWaitCheck(nativeContext, controlRoot, attempt, id, checkWait); err != nil {
			invocation.fail("metasystem test run: await shared producer:", err)
			return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
		}
	}
	runRequest.QueueDurationMS += time.Since(producerWaitStarted).Milliseconds()
	resourceClass, exclusive := testrun.OwnedResources(prepared, attempt)
	var nativeLease *proofrun.HostResourceLease
	if resourceClass != "" {
		nativeLease, err = proofrun.AcquireHostResourcesWithWaitCheck(nativeContext, controlRoot, prepared.ConfPath, resourceClass, exclusive, checkWait)
		if err != nil {
			invocation.fail("metasystem test run: admit native testing:", err)
			return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
		}
		defer nativeLease.Close()
		runRequest.QueueDurationMS += nativeLease.Waited().Milliseconds()
	}
	// The admitted execution allowance starts after producer and capacity waits.
	// A nested run still belongs to its reservation owner's absolute horizon.
	admittedDeadline, deadlineErr := time.Parse(time.RFC3339Nano, attempt.Deadline)
	admittedStart, startErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
	if deadlineErr != nil || startErr != nil || !admittedDeadline.After(admittedStart) {
		invocation.fail("metasystem test run: the admitted execution timing is unreadable")
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, proofrun.ExitAdmissionRefused)
	}
	if attempt.ReservationOwner == nil {
		admittedDeadline = commandClock().Add(admittedDeadline.Sub(admittedStart))
	}
	deadline, deadlineCheck, err := proofDeadline(admittedDeadline.Format(time.RFC3339Nano), commandClock)
	if err != nil {
		invocation.fail("metasystem test run: admit native testing:", err)
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, proofrun.ExitAdmissionRefused)
	}
	executionContext, cancelExecution := proofDeadlineContext(nativeContext, deadline, fixtureClock)
	defer cancelExecution()
	// The worker sees the writer after the host lease files (LaunchSuite).
	if err := testrun.BindScratch(&runRequest, scratch, scratch.Locator(proofrun.ScratchWriterFD(nativeLease.Files())), scratchEnvironment); err != nil {
		invocation.fail("metasystem test run: scratch environment:", err)
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
	}
	if err := writePrivateJSON(packetPath, runRequest); err != nil {
		invocation.fail("metasystem test run:", err)
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
	}
	packetDigest, err := fileSHA256(packetPath)
	if err != nil {
		invocation.fail("metasystem test run:", err)
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, 1)
	}
	if err := errors.Join(executionContext.Err(), checkAdmission(), deadlineCheck()); err != nil {
		invocation.fail("metasystem test run: admit native testing:", err)
		return retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, proofrun.ExitAdmissionRefused)
	}
	launchStatus := proofrun.LaunchSuite(proofrun.LaunchOptions{Suite: "testing", Root: prepared.ProjectRoot,
		ControlRoot: controlRoot, AttemptID: attempt.AttemptID, JoinedAttempt: joined, Deadline: deadline, ConfPath: prepared.ConfPath,
		ProgressPath: runRequest.ProgressPath, LogPath: filepath.Join(pathsRoot, "launcher.log"),
		Banner: "TESTING-CONTRACT plan=" + planDigest, Silence: limits.silence, SectionCap: limits.sectionCap,
		EvidenceTimeout: limits.evidenceTimeout, EvidenceMax: limits.evidenceMax, Poll: time.Second, TermGrace: 5 * time.Second,
		KillGrace: time.Second, Command: []string{workerEngine, "test", "worker", "--packet", packetPath, "--packet-sha256", packetDigest, "--result", workerResultPath},
		Environment: workerEnvironment, HostResourceFiles: nativeLease.Files(), ScratchWriter: scratch.Writer(), RequireCustody: true, Output: invocation.stdout, ErrorOutput: invocation.stderr,
		Now: commandClock,
		PrepareSuccess: func(completion proofrun.CompletionContext) (json.RawMessage, error) {
			result, readErr := testrun.ReadWorkerResult(workerResultPath)
			if readErr != nil {
				return nil, readErr
			}
			retained = &result
			if request.BatchPrefixReceipt || request.BatchAdmission || !testingReceiptWanted(joined, prepared.Plan.Purpose, result.Delivery.Sufficient) {
				return nil, nil
			}
			receipt, payload, prepareErr := prepareTestingRunReceipt(prepared, result, completion.CompletedAt)
			if prepareErr == nil && receipt.Testing != nil {
				updated := *receipt.Testing
				retained = &updated
			}
			return payload, prepareErr
		},
		CommitTerminal: testingTerminalCommit(workerResultPath, &retained)})
	launchStatus = retainIncompleteProofAttempt(invocation.stderr, controlRoot, attempt.AttemptID, joined, launchStatus)
	if retained == nil {
		if result, readErr := testrun.ReadWorkerResult(workerResultPath); readErr == nil {
			retained = &result
		}
	}
	if retained != nil {
		if joined {
			if _, err := proofrun.RecordTestResultAt(controlRoot, attempt.AttemptID, *retained, commandClock()); err != nil {
				invocation.fail("metasystem test run: retain joined result:", err)
				return 1
			}
		}
		if err := publishTestingResultTo(invocation.results(), invocation.stderr, controlRoot, request.ResultPath, *retained); err != nil {
			invocation.fail("metasystem test run:", err)
			return 1
		}
	}
	return launchStatus
}

// prepareTestingRunReceipt builds the receipt of a passing run. Its
// attempt records are read at the proof control root, where the run wrote
// them: a lane proof's lie in the lane checkout, not in the detached worktree
// it ran in; a seat proof's control root is its own installation.
func prepareTestingRunReceipt(prepared testrun.Preparation, result proofrun.TestResult, completedAt time.Time) (landing.TestReceipt, json.RawMessage, error) {
	return landing.PrepareTestingReceiptPayloadFrom(prepared.Installation, prepared.ProofControlRoot(), prepared.CandidateTree, result, completedAt)
}

func testingWorkerEnvironment(environment []string) ([]string, error) {
	return identity.ExportRunOwner(environment)
}

func runTestWorker(args []string, stdout, stderr io.Writer) int {
	return runTestWorkerWithCandidateOpener(args, nil, stdout, stderr)
}

func runTestWorkerWithCandidateOpener(args []string, opener func(string, string) (proofrun.CandidateWorkspace, error), stdout, stderr io.Writer) int {
	flags := newFlagSet("test worker", stdout, stderr)
	packet := flags.String("packet", "", "private testing request")
	packetDigest := flags.String("packet-sha256", "", "SHA-256 identity of the immutable testing request")
	resultPath := flags.String("result", "", "private testing result")
	if flags.Parse(args) != nil || !requireFlags(flags, stderr, "packet", "packet-sha256", "result") || flags.NArg() != 0 || *packet == "" || *packetDigest == "" || *resultPath == "" {
		return 2
	}
	actualPacketDigest, err := fileSHA256(*packet)
	if err != nil || actualPacketDigest != *packetDigest {
		fmt.Fprintln(stderr, "test worker: immutable request identity mismatch")
		return 3
	}
	var request proofrun.TestRunRequest
	if err := strictjson.Read(*packet, &request); err != nil {
		fmt.Fprintln(stderr, "test worker:", err)
		return 2
	}
	legacyPolicyProbe := os.Getenv(policyProbeWorkerEnvironment) == "1"
	if legacyPolicyProbe {
		refusal := frozenPolicyProbeRefusal(request, *resultPath)
		if refusal != "" {
			fmt.Fprintln(stderr, "test worker: unrecognized frozen policy probe:", refusal)
			return 3
		}
		request.SyntheticProbe = true
	}
	if request.CandidateEngine == "" || request.CandidateEngineDigest == "" ||
		(request.CandidateEngineBuildIdentity == "" && !legacyPolicyProbe) {
		fmt.Fprintln(stderr, "test worker: input-bound candidate engine is absent")
		return 3
	}
	policyDigest, policyDigestErr := fileSHA256(request.PolicyEngine)
	if request.PolicyEngine == "" || policyDigestErr != nil || policyDigest != request.PolicyEngineDigest {
		fmt.Fprintln(stderr, "test worker: input-bound policy engine changed")
		return 3
	}
	engineInfo, statErr := os.Stat(request.CandidateEngine)
	engineDigest, digestErr := fileSHA256(request.CandidateEngine)
	if statErr != nil || !engineInfo.Mode().IsRegular() || engineInfo.Mode()&0o111 == 0 || digestErr != nil || engineDigest != request.CandidateEngineDigest {
		fmt.Fprintln(stderr, "test worker: input-bound candidate engine changed")
		return 3
	}
	controlRoot, attemptID := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	canonicalControl, err := canonicalProofRoot(controlRoot)
	if err != nil || canonicalControl == "" || attemptID == "" || attemptID != request.AttemptID {
		fmt.Fprintln(stderr, "test worker: the test run named in its settings is not the one it was started for")
		return 3
	}
	if err := proofrun.AuthenticateWorker(canonicalControl, attemptID, os.Getenv("METASYSTEM_PROOF_RECORD_KEY"),
		os.Getenv("METASYSTEM_PROOF_CREATION_CLAIM"), int64(os.Getppid())); err != nil {
		fmt.Fprintln(stderr, "test worker:", err)
		return 3
	}
	attempt, err := proofrun.ReadAttempt(canonicalControl, attemptID)
	if err != nil {
		fmt.Fprintln(stderr, "test worker:", err)
		return 3
	}
	packetControl, controlErr := canonicalProofRoot(request.ControlRoot)
	packetProject, projectErr := canonicalProofRoot(request.ProjectRoot)
	admittedControl, admittedControlErr := canonicalProofRoot(attempt.ControlRoot)
	admittedProject, admittedProjectErr := canonicalProofRoot(attempt.ExecutionRoot)
	if controlErr != nil || projectErr != nil || admittedControlErr != nil || admittedProjectErr != nil ||
		packetControl != canonicalControl || admittedControl != canonicalControl ||
		(!legacyPolicyProbe && packetProject != admittedProject) {
		fmt.Fprintln(stderr, "test worker: authenticated request roots do not match the worker packet")
		return 3
	}
	request.ControlRoot = canonicalControl
	if legacyPolicyProbe {
		request.ProjectRoot = packetProject
	} else {
		request.ProjectRoot = admittedProject
	}
	if _, err := time.Parse(time.RFC3339Nano, attempt.Deadline); err != nil {
		fmt.Fprintln(stderr, "test worker: admitted deadline is invalid")
		return 3
	}
	request.Environment = testrun.InheritedEnvironment(request.Environment, os.Environ())
	request.Environment = proofrun.TestingEnvironment(request.Environment, map[string]string{
		proofWitnessExecutionRootEnv: attempt.ExecutionRoot,
	})
	workerBase := context.Background()
	if request.Scratch != nil {
		// A managed run: authenticate the inherited writer and the run
		// against this attempt before any byte lands, then bind it so every
		// command, custodian and Git child inherits the writer.
		scratch, err := proofrun.OpenScratchRun(canonicalControl, attemptID, *request.Scratch)
		if err != nil {
			fmt.Fprintln(stderr, "test worker:", err)
			return 3
		}
		request.BindScratch(scratch, request.Scratch)
		if err := proofrun.ValidateScratchEnvironment(request, scratch); err != nil {
			fmt.Fprintln(stderr, "test worker:", err)
			return 3
		}
		workerBase = proofrun.WithScratchRun(workerBase, scratch)
	}
	// The worker's context carries no deadline: the reservation is a
	// figure, not a kill rule (proof-groups-detect-hangs-by-progress-not-
	// the-clock, decision 3). A recorded cancellation intent cancels it.
	workerContext, cancel := context.WithCancel(workerBase)
	defer cancel()
	go cancelOnRecordedIntent(stderr, workerContext, cancel, canonicalControl, attemptID)
	if err := runFrozenPolicyProtectionCorpus(workerContext, request); err != nil {
		fmt.Fprintln(stderr, "test worker:", err)
		return 1
	}
	if opener != nil {
		request.WithCandidateOpener(opener)
	}
	result, status, runErr := proofrun.RunTestPlan(workerContext, request)
	response := result
	if runErr == nil && request.SyntheticProbe {
		response, err = frozenNegativeProbeResponse(request, result)
		if err != nil {
			fmt.Fprintln(stderr, "test worker:", err)
			return 1
		}
	}
	if runErr != nil {
		if err := proofrun.ValidateTestResult(response); err != nil {
			fmt.Fprintln(stderr, "test worker:", runErr)
			fmt.Fprintln(stderr, "test worker: operational result was not retained:", err)
			return 1
		}
	}
	if err := writePrivateJSON(*resultPath, response); err != nil {
		if runErr != nil {
			fmt.Fprintln(stderr, "test worker:", runErr)
		}
		fmt.Fprintln(stderr, "test worker:", err)
		return 1
	}
	printTestingSummaryTo(stdout, result)
	if runErr != nil {
		fmt.Fprintln(stderr, "test worker:", runErr)
		return 1
	}
	return status
}

func frozenPolicyProbeRefusal(request proofrun.TestRunRequest, resultPath string) string {
	switch {
	case request.Contract.SchemaVersion != 1:
		return fmt.Sprintf("its contract has schema %d, not 1", request.Contract.SchemaVersion)
	case len(request.Contract.Groups) != 1:
		return fmt.Sprintf("its contract has %d groups, not 1", len(request.Contract.Groups))
	case request.Contract.Groups[0].ID != "literal":
		return fmt.Sprintf("its contract's group is %q, not literal", request.Contract.Groups[0].ID)
	case len(request.Plan.SelectedGroups) != 1:
		return fmt.Sprintf("its plan selects %d groups, not 1", len(request.Plan.SelectedGroups))
	case request.Plan.SelectedGroups[0] != "literal":
		return fmt.Sprintf("its plan selects %q, not literal", request.Plan.SelectedGroups[0])
	case !strings.HasPrefix(filepath.Base(request.ProjectRoot), "metasystem-policy-probe."):
		return fmt.Sprintf("its project directory %q is not named metasystem-policy-probe.*", filepath.Base(request.ProjectRoot))
	case filepath.Dir(resultPath) != request.ProjectRoot:
		return fmt.Sprintf("its result goes to %q, outside its project directory %q", filepath.Dir(resultPath), request.ProjectRoot)
	default:
		return ""
	}
}

func runTestVerify(args []string, stdout, stderr io.Writer) int {
	return runTestVerifyAs("test verify", args, stdout, stderr)
}

// runTestVerifyAs is test verify answering as name: the internal entrypoint
// or the public test status.
func runTestVerifyAs(name string, args []string, stdout, stderr io.Writer) int {
	return runTestVerifyAsWithStatus(name, args, stdout, stderr, testStatusInputs{})
}

func runTestVerifyAsWithStatus(name string, args []string, stdout, stderr io.Writer, inputs testStatusInputs) int {
	request, jsonOutput, status := parseTestingSelection(name, args, false, stdout, stderr)
	if status != 0 {
		return status
	}
	if request.Tree == "" {
		page := passthroughPage(stderr, request.Root, request.Verbose)
		page.Refusal(commandLabel(name)+" needs the tree to check; nothing was read",
			textui.Hint{Argv: []string{"metasystem", "test", "status", "--tree", "TREE"}, Reason: "TREE is the exact tree, as git write-tree prints it"})
		printPage(stderr, page)
		return 2
	}
	_, public := publicCommand(name)
	if public {
		return testStatusTo(stdout, stderr, request, jsonOutput, inputs)
	}
	if !public && jsonOutput {
		return testVerifyEnvelope(stdout, stderr, request)
	}
	return testVerifyTo(stdout, stderr, request, jsonOutput)
}

// testVerifyEnvelope is the internal verify a base judge calls: one
// envelope on stdout whose data is the delivery verdict, the words on
// stderr.
func testVerifyEnvelope(stdout, stderr io.Writer, request testrun.SelectionRequest) int {
	var verdict bytes.Buffer
	status := testVerifyTo(&verdict, stderr, request, true)
	var data any
	if payload := bytes.TrimSpace(verdict.Bytes()); len(payload) > 0 && json.Valid(payload) {
		data = json.RawMessage(payload)
	}
	result := verbresult.FromError(testVerifyVerb, status, nil, data)
	switch {
	case data == nil:
		result.Summary = "the retained test runs could not be verified"
	case status == 0:
		result.Summary = "passing test runs cover this tree"
	default:
		result.Summary = "no passing test run covers every group on this tree"
	}
	_ = verbresult.Write(stdout, result)
	return status
}

// testVerifyTo verifies retained delivery proof for request.Tree and prints
// the verdict on the caller's streams; it launches nothing.
func testVerifyTo(stdout, stderr io.Writer, request testrun.SelectionRequest, jsonOutput bool) int {
	result, err := verifyRetainedTesting(request)
	if err != nil {
		printMovedProofInputsWithoutCandidateEngine(stderr, request)
		// The internal verify and the landing path read these two lines as
		// they always were.
		printTestingRefusalAs(stderr, err, request, true)
		return 1
	}
	if jsonOutput {
		writeJSONLine(stdout, stderr, result)
	} else {
		printTestingSummaryTo(stdout, result)
	}
	if !result.Delivery.Sufficient {
		printMovedProofInputs(stderr, request, result)
		fmt.Fprintf(stderr, "no passing test run covers groups %s on this tree\nrun: metasystem test run --root %s --goal %s --tree %s --mode auto\n",
			strings.Join(result.Delivery.MissingGroups, ","), request.Root, request.GoalID, result.CandidateTree)
		return 1
	}
	return 0
}

// verifyRetainedTesting composes the retained proof that covers request.Tree
// (the index when empty) for the request's goal and accounting revision. It
// launches nothing and creates no attempt. The result's per-group
// ExecutionIdentity is the identity on the current tree.
func verifyRetainedTesting(request testrun.SelectionRequest) (proofrun.TestResult, error) {
	prepared, err := prepareTestingForCommand(request)
	if err != nil {
		return proofrun.TestResult{}, err
	}
	commandClock, _, err := goalCommandClock(prepared.ProofControlRoot())
	if err != nil {
		return proofrun.TestResult{}, err
	}
	// Revalidation materializes the candidate like a run does, so it owns a
	// fresh scratch run of its own for exactly that long.
	scratch, err := proofrun.CreateScratchRun(prepared.ProofControlRoot())
	if err != nil {
		return proofrun.TestResult{}, fmt.Errorf("scratch root: %w", err)
	}
	result, verifyErr := testrun.VerifyPrepared(request, prepared, testrun.Verification{
		Clock: commandClock, Revalidate: proofrun.RevalidateRetainedGroupExecutionIdentities,
		Workspace: gittree.Workspace{Dir: prepared.ProjectRoot}, CandidateIO: candidateengine.Native(), Scratch: scratch,
		WorkerPolicy: testingWorkerPolicy,
	})
	if cleanupErr := scratch.Cleanup(nil); cleanupErr != nil {
		return proofrun.TestResult{}, errors.Join(verifyErr, cleanupErr)
	}
	return result, verifyErr
}

func printMovedProofInputs(stderr io.Writer, request testrun.SelectionRequest, current proofrun.TestResult) {
	installation, err := canonicalProofRoot(request.Root)
	if err != nil {
		return
	}
	goalID, err := testrun.ResolveGoal(installation, request.GoalID)
	if err != nil {
		return
	}
	_, accountingRevision, err := testrun.GoalRisk(installation, goalID)
	if err != nil {
		return
	}
	attempts, err := proofrun.ReadAttempts(installation)
	if err != nil {
		return
	}
	currentGroups := map[string]proofrun.GroupResult{}
	for _, group := range current.Groups {
		currentGroups[group.ID] = group
	}
	workspace := gittree.Workspace{Dir: current.ProjectRoot}
	for _, id := range current.Delivery.MissingGroups {
		var source *proofrun.GroupResult
		var sourceTree string
		var newest time.Time
		for _, attempt := range attempts {
			if !testrun.AttemptAccountsFor(attempt, goalID, accountingRevision) ||
				attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess || attempt.TestResult == nil {
				continue
			}
			started, startedErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
			if startedErr != nil || (!newest.IsZero() && !started.After(newest)) {
				continue
			}
			for groupIndex := range attempt.TestResult.Groups {
				group := &attempt.TestResult.Groups[groupIndex]
				if group.ID == id && group.CollectionComplete && (group.Status == "passed" || group.Status == "reused") &&
					group.ExecutionIdentity != currentGroups[id].ExecutionIdentity {
					copyGroup := *group
					source, sourceTree, newest = &copyGroup, attempt.TestResult.CandidateTree, started
				}
			}
		}
		if source == nil {
			continue
		}
		changed, err := workspace.ChangedPaths(sourceTree, current.CandidateTree)
		if err != nil {
			continue
		}
		var moved []string
		for _, changedPath := range changed {
			if testrun.InputManifestContains(source.InputManifest, changedPath) {
				moved = append(moved, changedPath)
			}
		}
		if len(moved) == 0 {
			fmt.Fprintf(stderr, "group %s passed on tree %s, but its environment or a tool changed since\n", id, sourceTree)
			printMovedInputsCode(stderr, request.Verbose)
			continue
		}
		fmt.Fprintf(stderr, "group %s passed on tree %s, but files it depends on changed since: %s\n", id, sourceTree, strings.Join(moved, ","))
		printMovedInputsCode(stderr, request.Verbose)
	}
}

// movedInputsCode is the register code of a group whose passing run no
// longer covers the tree; --verbose shows it.
const movedInputsCode = "proof-input-moved-after-receipt"

func printMovedInputsCode(stderr io.Writer, verbose bool) {
	if verbose {
		fmt.Fprintln(stderr, "  "+movedInputsCode)
	}
}

// printTestingRefusal prints a testing refusal as a person reads it, and
// with --verbose the detail behind it: the code, the cause and its facts.
// When the pinned engine that chooses the tests failed, its remedy is that
// engine's internal command; a person runs the public test plan with
// --verbose instead, which shows the engine's own words and names the
// internal command.
func printTestingRefusal(stderr io.Writer, err error, request testrun.SelectionRequest) {
	printTestingRefusalAs(stderr, err, request, false)
}

// printTestingRefusalAs is printTestingRefusal; with plain it is the two
// lines --json has always printed beside its JSON line.
func printTestingRefusalAs(stderr io.Writer, err error, request testrun.SelectionRequest, plain bool) {
	text, detail := err.Error(), refusal.Detail(err)
	var engine *enginecause.Refusal
	if errors.As(err, &engine) && (engine.Token == "child-failed" || engine.Token == "child-output") {
		// An engine that refused in its own two lines already names what
		// resolves it; otherwise the public plan shows why.
		text = engine.Reason
		if !strings.Contains(text, "\nrun: ") {
			retry := []string{"metasystem", "test", "plan", "--verbose"}
			if request.GoalID != "" {
				retry = append(retry, "--goal", request.GoalID)
			}
			if request.Tree != "" {
				retry = append(retry, "--tree", request.Tree)
			}
			text += "\nrun: " + shellCommand(retry)
		}
		// The detail names the engine and the command it ran.
		detail = engine.Detail()
	}
	if plain {
		fmt.Fprintln(stderr, text)
		if request.Verbose && detail != "" {
			fmt.Fprintln(stderr, "  "+detail)
		}
		return
	}
	reason, remedy, _ := strings.Cut(text, "\nrun: ")
	page := passthroughPage(stderr, request.Root, request.Verbose)
	page.Refusal(reason, textui.Hint{Reason: remedy})
	if request.Verbose && detail != "" {
		page.Section("Details", "").Text(detail)
	}
	printPage(stderr, page)
}

func printMovedProofInputsWithoutCandidateEngine(stderr io.Writer, request testrun.SelectionRequest) {
	prepared, err := testrun.Prepare(request)
	if err != nil {
		return
	}
	attempts, err := proofrun.ReadAttempts(prepared.Installation)
	if err != nil {
		return
	}
	workspace := gittree.Workspace{Dir: prepared.ProjectRoot}
	for _, id := range prepared.Plan.RequiredGroups {
		var source *proofrun.GroupResult
		var sourceTree string
		var newest time.Time
		for _, attempt := range attempts {
			if !testrun.AttemptAccountsFor(attempt, prepared.GoalID, prepared.AccountingRevision) ||
				attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess || attempt.TestResult == nil {
				continue
			}
			started, startedErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
			if startedErr != nil || (!newest.IsZero() && !started.After(newest)) {
				continue
			}
			for groupIndex := range attempt.TestResult.Groups {
				group := &attempt.TestResult.Groups[groupIndex]
				if group.ID == id && group.CollectionComplete && (group.Status == "passed" || group.Status == "reused") {
					copyGroup := *group
					source, sourceTree, newest = &copyGroup, attempt.TestResult.CandidateTree, started
				}
			}
		}
		if source == nil {
			continue
		}
		changed, changedErr := workspace.ChangedPaths(sourceTree, prepared.CandidateTree)
		if changedErr != nil {
			continue
		}
		var moved []string
		for _, changedPath := range changed {
			if testrun.InputManifestContains(source.InputManifest, changedPath) {
				moved = append(moved, changedPath)
			}
		}
		if len(moved) > 0 {
			fmt.Fprintf(stderr, "group %s passed on tree %s, but files it depends on changed since: %s\n", id, sourceTree, strings.Join(moved, ","))
			printMovedInputsCode(stderr, request.Verbose)
		}
	}
}

func writePrivateJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicfile.WriteVolatile(path, string(data)+"\n")
}

// publishTestingResultTo writes a test result to its path, or prints it (or
// its spill reference) on stdout.
func publishTestingResultTo(stdout, stderr io.Writer, root, path string, result proofrun.TestResult) error {
	if path != "" {
		return writeIdentityJSON(path, result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if len(encoded) <= output.MaxInlineBytes {
		fmt.Fprintln(stdout, string(encoded))
		return nil
	}
	reference, err := output.Spill(root, "test-run", "json", append(encoded, '\n'), time.Now().UTC())
	if err != nil {
		return err
	}
	writeJSONLine(stdout, stderr, reference)
	return nil
}

func printTestingSummaryTo(stdout io.Writer, result proofrun.TestResult) {
	admission := "unlimited"
	if result.AdmissionMaximum != nil && *result.AdmissionMaximum > 0 {
		admission = strconv.Itoa(*result.AdmissionMaximum)
	}
	fmt.Fprintf(stdout, "TEST-RESULT sufficient=%t tree=%s selected=%s workers=%d admissionMaximum=%s\n",
		result.Delivery.Sufficient, result.CandidateTree, strings.Join(result.SelectedGroups, ","), result.Workers, admission)
	for _, group := range result.Groups {
		if group.Status != "passed" && group.Status != "reused" {
			fmt.Fprintf(stdout, "TEST-GROUP %s status=%s reason=%s log=%s\n", group.ID, group.Status, group.NotRunReason, group.LogPath)
		}
	}
}

func bytesSHA256(data []byte) string { return digest.SHA256(data) }

func fileSHA256(path string) (string, error) { return digest.FileSHA256(path) }

// finishTestingScratch removes this run's scratch root; a writer that has not
// drained keeps the root and its record for recovery and fails the command.
func finishTestingScratch(stderr io.Writer, scratch *proofrun.ScratchRun, status int) int {
	if err := scratch.Cleanup(nil); err != nil {
		fmt.Fprintln(stderr, "metasystem test run:", err)
		if status == 0 || status == proofrun.ExitReusableSuccess {
			return 1
		}
	}
	return status
}

func testingTerminalCommit(workerResultPath string, retained **proofrun.TestResult) func(proofrun.CompletionContext, json.RawMessage) error {
	return testingTerminalCommitWithReads(workerResultPath, retained, nil)
}

func testingTerminalCommitWithReads(workerResultPath string, retained **proofrun.TestResult, reads *dispatchcore.ProofAdmissionReads) func(proofrun.CompletionContext, json.RawMessage) error {
	return func(completion proofrun.CompletionContext, receipt json.RawMessage) error {
		if *retained == nil {
			result, readErr := testrun.ReadWorkerResult(workerResultPath)
			if readErr != nil {
				if completion.ExitStatus == 0 {
					return readErr
				}
				return commitProofTerminalWithReasonAndReads(completion, receipt, nil,
					"proof launcher completed; the worker left no usable result: "+readErr.Error(), reads)
			}
			*retained = &result
		}
		return commitProofTerminalWithReasonAndReads(completion, receipt, *retained, "proof launcher completed", reads)
	}
}

func testingReceiptWanted(joined bool, purpose testpolicy.Purpose, sufficient bool) bool {
	return !joined && sufficient && purpose != testpolicy.PurposeDiagnostic
}

// cancelOnRecordedIntent cancels the worker's context once a cancellation
// intent is recorded on its attempt; it reads the record at a slow pace
// and ends with the context.
func cancelOnRecordedIntent(stderr io.Writer, ctx context.Context, cancel context.CancelFunc, controlRoot, attemptID string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if attempt, err := proofrun.ReadAttempt(controlRoot, attemptID); err == nil && attempt.CancellationIntent != "" {
				fmt.Fprintf(stderr, "test worker: cancellation intent recorded: %s\n", attempt.CancellationIntent)
				cancel()
				return
			}
		}
	}
}
