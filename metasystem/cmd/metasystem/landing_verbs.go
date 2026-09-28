package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func runLandingObserve(args []string) int {
	flags := flag.NewFlagSet("landing observe", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "project checkout root")
	tree := flags.String("tree", "", "prospective project tree")
	chain := flags.String("chain", "", "closed implementation chain root")
	attested := flags.String("attested", "", "critic-attested goal branch commit")
	attestedSnapshot := flags.String("attested-snapshot", "", "goal branch snapshot containing the attestation")
	attestedBase := flags.String("attested-base", "", "endpoint commit below the goal branch")
	directFix := flags.String("direct-fix", "", "typed direct-fix class; register-carriage may accompany --chain")
	revertOf := flags.String("revert-of", "", "commit inverted by exact-revert")
	goal := flags.String("goal", "", "goal item carried by the landing")
	actor := flags.String("actor", "", "wrapper actor as machine+lineage")
	rootJob := flags.String("root-job", "", "tier-1 root implementer job")
	testReceipt := flags.String("test-receipt", "", "candidate test receipt path")
	recertification := flags.String("recertification", "", "canonical recertification record path")
	carried := flags.String("carried", "", "human carry word operation id")
	projectTree := flags.String("project-tree", "", "whole-project tree used for the carry workspace projection")
	ledgerTip := flags.String("ledger-tip", "", "frozen accepted goal-ledger tip")
	judge := flags.String("judge", "", "carried evaluation engine: live or base")
	liveFailure := flags.String("live-failure", "", "live evaluator refusal code or exit status used by the base judge")
	carriedBy := flags.String("carried-by", "", "human actor stamped in the carried commit")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	if *carried != "" && (*judge != "live" && *judge != "base" || *judge == "base" && *liveFailure == "") {
		fmt.Fprintln(os.Stderr, "landing observe --carried requires --judge live, or --judge base with --live-failure")
		return 2
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		return recordExit(err)
	}
	params := landing.ObserveParams{
		RepoRoot: *root, CandidateTree: *tree, Chain: *chain,
		Attested: *attested, AttestedSnapshot: *attestedSnapshot, AttestedBase: *attestedBase,
		DirectFix: *directFix, RevertOf: *revertOf, Goal: *goal, Actor: *actor,
		RootJob: *rootJob, TestReceipt: *testReceipt, Recertification: *recertification,
		Carried: *carried, ProjectTree: *projectTree, LedgerTip: *ledgerTip, Judge: *judge, LiveFailure: *liveFailure, CarriedBy: *carriedBy, Now: now,
	}
	params.BindAttested = func(commit, snapshot, base, goal, beforeTree, afterTree string) (landing.AttestedUnit, error) {
		bound, err := goalbranch.BindLandedUnit(*root, snapshot, base, goal, commit, beforeTree, afterTree)
		return landing.AttestedUnit{Goal: bound.Goal, Unit: bound.Unit, Digest: bound.Digest, CriticRoot: bound.CriticRoot, GateRunID: bound.GateRunID,
			Round: bound.Round, GoalRevision: bound.GoalRevision, FoldPaths: bound.FoldPaths, ChangedPaths: bound.ChangedPaths,
			HasPlan: bound.HasPlan, Destructive: bound.Destructive}, err
	}
	if (*testReceipt != "" || *carried != "") && *recertification == "" {
		params.VerifyTesting = func() (proofrun.TestResult, error) {
			return verifyRetainedTesting(testingSelectionRequest{Root: *root, GoalID: *goal,
				Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, Carried: *carried != ""})
		}
	}
	observation := landing.Observe(params)
	printJSON(observation)
	return 0
}

func runLandingWorkspace(args []string) int {
	flags := flag.NewFlagSet("landing workspace", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "MetaSystem installation root")
	tree := flags.String("tree", "", "whole-project tree")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *root == "" || *tree == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal landing workspace --root INSTALLATION --tree TREE")
		return 2
	}
	workspace, err := landing.ProjectWorkspaceTree(*root, *tree)
	if err != nil {
		return recordExit(err)
	}
	fmt.Println(workspace)
	return 0
}

// landingHeld re-checks the goal binding of every commit a push introduces,
// in the caller's process; a refusal or an unreadable verdict is the error,
// carrying the lines the verb prints.
func landingHeld(root, base, commit, remote, ref string) error {
	var output strings.Builder
	if status := landingHeldTo(&output, &output, cleanOwnerRoot(root), base, commit, remote, ref); status != 0 {
		return fmt.Errorf("%s: landing held exited %d", strings.TrimSpace(output.String()), status)
	}
	return nil
}

func landingHeldTo(stdout, stderr io.Writer, root, base, commit, remote, ref string) int {
	verdict, err := landing.Held(root, base, commit, remote, ref)
	if err != nil {
		fmt.Fprintln(stderr, "held: unreadable:", err)
		return 2
	}
	for _, warning := range verdict.Warnings {
		fmt.Fprintln(stderr, warning)
	}
	switch verdict.Outcome {
	case "nothing-to-push":
		fmt.Fprintln(stdout, "held: nothing to push")
	case "goal-free":
		fmt.Fprintln(stdout, "held: goal-free ledger")
	case "ok":
		fmt.Fprintf(stdout, "held: ok %d commit(s) above %s\n", verdict.Commits, shortLandingID(verdict.Base))
	case "refused":
		if verdict.Refusal != nil {
			fmt.Fprintf(stderr, "held refused: %s: %s: %s\n", verdict.Refusal.Code, verdict.Refusal.Commit, verdict.Refusal.Detail)
		}
	case "unreadable":
		// Held has already supplied the precise unreadable line in Warnings.
	default:
		fmt.Fprintln(stderr, "held: unreadable: unknown verdict")
		return 2
	}
	return verdict.ExitCode
}

func shortLandingID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

var landingReceiptTestRun = runTestRun

func runLandingTestReceipt(args []string) (status int) {
	return runLandingTestReceiptWithContext(context.Background(), goalCommandClock, args)
}

func runLandingTestReceiptWithContext(parent context.Context, resolveClock func(string) (func() time.Time, bool, error), args []string) (status int) {
	return runLandingTestReceiptWithDependencies(parent, resolveClock, nil, landingReceiptTestRun, args)
}

func runLandingTestReceiptWithDependencies(parent context.Context, resolveClock func(string) (func() time.Time, bool, error), raw func(gittree.RawRequest) gittree.RawResult, testRun func([]string) int, args []string) (status int) {
	return runLandingTestReceiptWithInputs(parent, resolveClock, raw, testRun, args,
		landing.PrepareTestReceipt, admitProofLaunch, landing.PublishCommittedReceiptAt, commitProofTerminal)
}

func runLandingTestReceiptWithInputs(parent context.Context, resolveClock func(string) (func() time.Time, bool, error), raw func(gittree.RawRequest) gittree.RawResult, testRun func([]string) int, args []string,
	prepare func(string, string, string) (*landing.ReceiptPreparation, error),
	admit func(proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error),
	publish func(string, string, string, time.Time) (landing.TestReceipt, error),
	terminal func(proofrun.CompletionContext, json.RawMessage) error) (status int) {
	flags := flag.NewFlagSet("landing test-receipt", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "project checkout root")
	tree := flags.String("tree", "", "candidate project tree")
	command := flags.String("command", "", "test command to run from the isolated candidate workspace")
	mode := flags.String("mode", "", "shared testing mode: auto, standard, or deep")
	goalID := flags.String("goal", "", "accepted goal owning the receipt proof")
	capMin := flags.String("cap-min", "", "reserved proof minutes")
	retryDecision := flags.String("retry-decision", "", "accountable retry decision")
	resultPath := flags.String("result", "", "atomic structured launch result path")
	expectedGoalRevision := flags.Uint64("expected-goal-revision", 0, "sealed goal revision")
	expectedAccountingRevision := flags.Uint64("expected-accounting-revision", 0, "sealed accounting revision")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	if (*expectedGoalRevision == 0) != (*expectedAccountingRevision == 0) {
		fmt.Fprintln(os.Stderr, "landing test-receipt expected goal and accounting revisions must be supplied together")
		return 2
	}
	if (*mode == "") == (*command == "") {
		fmt.Fprintln(os.Stderr, "landing test-receipt requires exactly one of --mode or legacy --command")
		return 2
	}
	controlRoot, err := canonicalProofRoot(*root)
	if err != nil {
		return recordExit(err)
	}
	commandClock, fixtureClock, err := resolveClock(controlRoot)
	if err != nil {
		return recordExit(err)
	}
	if *mode != "" {
		if *mode != "auto" && *mode != "standard" && *mode != "deep" {
			fmt.Fprintln(os.Stderr, "landing test-receipt --mode must be auto, standard, or deep")
			return 2
		}
		controlWorkspace := gittree.Workspace{Dir: controlRoot, RawSource: raw}
		projectRoot, err := controlWorkspace.TopLevel()
		if err != nil {
			return recordExit(err)
		}
		workspace := gittree.Workspace{Dir: projectRoot, RawSource: raw}
		acceptedIndexTree := *tree
		if acceptedIndexTree == "" {
			acceptedIndexTree, err = workspace.StagedTree()
		} else {
			acceptedIndexTree, err = workspace.ResolveTree(acceptedIndexTree)
		}
		if err != nil {
			return recordExit(err)
		}
		resultPath := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "delivery", "testing-result-"+*tree+".json")
		testArgs := []string{"--root", controlRoot, "--tree", acceptedIndexTree, "--mode", *mode, "--purpose", "delivery", "--result", resultPath}
		if *goalID != "" {
			testArgs = append(testArgs, "--goal", *goalID)
		}
		if *capMin != "" {
			testArgs = append(testArgs, "--cap-min", *capMin)
		}
		if *retryDecision != "" {
			testArgs = append(testArgs, "--retry-decision", *retryDecision)
		}
		if *expectedGoalRevision != 0 {
			testArgs = append(testArgs, "--expected-goal-revision", fmt.Sprint(*expectedGoalRevision),
				"--expected-accounting-revision", fmt.Sprint(*expectedAccountingRevision))
		}
		status := testRun(testArgs)
		if status != 0 && status != proofrun.ExitReusableSuccess {
			return status
		}
		var result proofrun.TestResult
		if err := readStrictJSON(resultPath, &result); err != nil {
			return recordExit(fmt.Errorf("read shared testing result: %w", err))
		}
		var receipt landing.TestReceipt
		if result.AttemptID != "" {
			receipt, err = landing.PublishCommittedReceiptAt(controlRoot, result.AttemptID, acceptedIndexTree, commandClock())
		} else {
			// A verifier may lawfully compose unchanged successful groups from
			// several older outer attempts. No new execution or synthetic
			// attempt is created for that schema-2 projection.
			receipt, err = landing.CreateTestingReceiptAt(controlRoot, result.CandidateTree, result, commandClock())
		}
		if err != nil {
			return recordExit(err)
		}
		printJSON(receipt)
		return 0
	}
	preparation, err := prepare(controlRoot, *tree, *command)
	if err != nil {
		return recordExit(err)
	}
	defer func() {
		if closeErr := preparation.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "landing test-receipt: preserve detached suite-failure evidence:", closeErr)
			if status == 0 {
				status = 1
			}
		}
	}()
	confPath := filepath.Join(preparation.ExecutionRoot(), "metasystem.conf")
	executionEnvironment := []string(nil)
	if *command == landing.CanonicalValidatorCommand {
		if proofRunAlternateGoInputs() {
			return recordExit(fmt.Errorf("canonical validator refuses GOFLAGS containing -modfile or -overlay"))
		}
		executionEnvironment = canonicalValidatorEnvironment()
	}
	limits, err := resolveProofRunLimits(confPath)
	if err != nil {
		return recordExit(err)
	}
	executionEnvironment = resolvedTestWorkerEnvironment(executionEnvironment, limits.workers)
	proofDir := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "delivery")
	if err := os.MkdirAll(proofDir, 0o700); err != nil {
		return recordExit(err)
	}
	attempt, decision, joined, err := admit(proofLaunchAdmission{ControlRoot: controlRoot,
		ExecutionRoot: preparation.ExecutionRoot(), ConfPath: confPath, GoalID: *goalID, CapMin: *capMin,
		RetryDecision: *retryDecision, ScopeClass: "full", CommandClass: "landing-test-receipt", Sections: nil,
		ExpectedGoalRevision: *expectedGoalRevision, ExpectedAccountingRevision: *expectedAccountingRevision,
		Environment: executionEnvironment, Now: commandClock()})
	if err != nil {
		decision = proofrun.LaunchResult{SchemaVersion: 1, Disposition: proofrun.DispositionAdmissionRefused, ExitStatus: proofrun.ExitAdmissionRefused}
		_ = proofrun.EncodeResult(os.Stderr, *resultPath, decision)
		fmt.Fprintln(os.Stderr, "landing test-receipt:", err)
		return proofrun.ExitAdmissionRefused
	}
	if decision.Disposition != proofrun.DispositionExecuted {
		if decision.Disposition == proofrun.DispositionReusableSuccess {
			if _, err := publish(controlRoot, decision.AttemptID, preparation.AcceptedIndexTree(), commandClock()); err != nil {
				return recordExit(err)
			}
		}
		if err := proofrun.EncodeResult(os.Stderr, *resultPath, decision); err != nil {
			return recordExit(err)
		}
		return decision.ExitStatus
	}
	deadline, deadlineCheck, err := proofDeadline(attempt.Deadline, commandClock)
	if err != nil {
		fmt.Fprintln(os.Stderr, "landing test-receipt: admit native proof:", err)
		status = retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, proofrun.ExitAdmissionRefused)
		decision.ExitStatus, decision.Disposition, decision.Reason = status, proofrun.DispositionAdmissionRefused, err.Error()
		_ = proofrun.EncodeResult(os.Stderr, *resultPath, decision)
		return status
	}
	resourceContext, cancelResource := proofDeadlineContext(parent, deadline, fixtureClock)
	defer cancelResource()
	lease, release, leaseErr := acquireManagedProofLaunchWithWaitCheck(resourceContext, controlRoot, confPath, deadlineCheck)
	if leaseErr != nil {
		fmt.Fprintln(os.Stderr, "landing test-receipt: admit native proof:", leaseErr)
		status = retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, proofrun.ExitAdmissionRefused)
		decision.ExitStatus, decision.Disposition, decision.Reason = status, proofrun.DispositionAdmissionRefused, leaseErr.Error()
		_ = proofrun.EncodeResult(os.Stderr, *resultPath, decision)
		return status
	}
	defer release()
	status = proofrun.LaunchSuite(proofrun.LaunchOptions{Suite: "landing-receipt", Root: preparation.ExecutionRoot(),
		ControlRoot: controlRoot, AttemptID: attempt.AttemptID, JoinedAttempt: joined, Deadline: deadline, ConfPath: confPath,
		ProgressPath: filepath.Join(proofDir, attempt.AttemptID+".progress.jsonl"), LogPath: filepath.Join(proofDir, attempt.AttemptID+".log"),
		Banner:  "EXPENSIVE SUITE landing-receipt: one admitted execution; retained result controls repeats",
		Silence: limits.silence, SectionCap: limits.sectionCap,
		EvidenceTimeout: limits.evidenceTimeout, EvidenceMax: limits.evidenceMax, Poll: time.Second,
		TermGrace: 5 * time.Second, KillGrace: time.Second, Command: []string{"bash", "-c", *command}, HostResourceFiles: lease.Files(), RequireCustody: true, Output: os.Stdout, ErrorOutput: os.Stderr,
		Environment: executionEnvironment, Now: commandClock,
		PrepareSuccess: func(completion proofrun.CompletionContext) (json.RawMessage, error) {
			current, err := proofrun.ReadAttempt(controlRoot, attempt.AttemptID)
			if err != nil {
				return nil, err
			}
			receipt, err := preparation.Complete(current, completion.CompletedAt)
			if err != nil {
				return nil, err
			}
			if *command == landing.CanonicalValidatorCommand && receipt.Coverage == nil {
				return nil, fmt.Errorf("canonical validator produced no authenticated coverage component")
			}
			return json.Marshal(receipt)
		}, CommitTerminal: terminal})
	status = retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, status)
	decision.ExitStatus = status
	if status != 0 {
		decision.Disposition = proofrun.DispositionFailed
	} else {
		receipt, publishErr := publish(controlRoot, attempt.AttemptID, preparation.AcceptedIndexTree(), commandClock())
		if publishErr != nil {
			fmt.Fprintln(os.Stderr, publishErr)
			status = 1
			decision.ExitStatus = status
			decision.Disposition = proofrun.DispositionFailed
		} else {
			printJSON(receipt)
		}
	}
	if err := proofrun.EncodeResult(os.Stderr, *resultPath, decision); err != nil {
		return recordExit(err)
	}
	return status
}

func canonicalValidatorEnvironment() []string {
	owned := map[string]bool{"GOFLAGS": true, "METASYSTEM_GATE_FROZEN_TOOLCHAIN": true, "GOCACHE": true, "STATICCHECK_CACHE": true}
	caches, _ := gocache.Resolve(os.Environ())
	environment := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !owned[name] {
			environment = append(environment, entry)
		}
	}
	environment = append(environment, gocache.Environment(caches)...)
	// GOFLAGS must equal cmd/devgate's ownedGoFlags: the gate's frozen-tree
	// check accepts exactly that value (disk-lifetimes A4).
	return append(environment, "GOFLAGS=-mod=readonly -trimpath", "METASYSTEM_GATE_FROZEN_TOOLCHAIN=1")
}

func runLandingAdoptionRulings(args []string) int {
	flags := flag.NewFlagSet("landing adoption-rulings", flag.ContinueOnError)
	source := flags.String("source", "", "staged template installation")
	target := flags.String("target", "", "application installation")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *source == "" || *target == "" {
		return 2
	}
	data, err := landing.AdoptionRulings(*source, *target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runLandingPark durably records one stopped recertified landing attempt
// (`landing park`); the landing path parks through the same owner.
func runLandingPark(args []string) int {
	flags := flag.NewFlagSet("landing park", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "integration project root")
	chain := flags.String("chain", "", "root implementation chain")
	target := flags.String("target", "", "frozen target commit")
	reason := flags.String("reason", "", "original landing refusal code")
	detail := flags.String("detail", "", "specific refusal explanation")
	recertification := flags.String("recertification", "", "diagnostic recertification record path")
	candidate := flags.String("candidate-commit", "", "retained local landing commit")
	var refs landingRepeatedStrings
	flags.Var(&refs, "recovery-ref", "retained source/result anchor ref (repeatable)")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	result, err := landing.Park(landing.ParkParams{
		Root: *root, Chain: *chain, TargetCommit: *target, Reason: *reason, Detail: *detail,
		Recertification: *recertification, CandidateCommit: *candidate, RecoveryRefs: refs,
		CallerPID: int64(os.Getppid()),
	})
	if err != nil {
		var failure *landing.ParkFailure
		if errors.As(err, &failure) {
			fmt.Fprintf(os.Stderr, "reason=chain-recertification-park-failed cause=%s error=%v\n", failure.Cause, failure.Err)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	fmt.Printf("state=%s\nreason=%s\nparkRecord=%s\n", result.State, result.Reason, result.ParkRecord)
	return 0
}

type landingRepeatedStrings []string

func (values *landingRepeatedStrings) String() string { return fmt.Sprint([]string(*values)) }
func (values *landingRepeatedStrings) Set(value string) error {
	*values = append(*values, value)
	return nil
}
