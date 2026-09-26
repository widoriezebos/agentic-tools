package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

type processScope struct {
	Checkout             string
	Installation         string
	InstallationExplicit bool
	Root                 string
	Binary               string
}

type processCallerClassifier func(string, string, int64) (lease.Classification, error)

// classifyVerbCaller keeps checkout-owned lease state separate from the
// installed adapters that identify runtimes. Production uses the engine that
// is executing the verb; source-tree and fixture binaries resolve the target
// checkout's installation with the same rule as stop, status, and arm.
func classifyVerbCaller(root string, callerPid int64) (lease.ClassifyResult, error) {
	return classifyVerbCallerWith(root, callerPid, stateroot.RepositoryTop)
}

func classifyVerbCallerWith(root string, callerPid int64, repositoryTop func(string) (string, error)) (lease.ClassifyResult, error) {
	installation, err := upMetasystemRoot("")
	if err != nil {
		if scope, scopeErr := resolveProcessScopeWith(root, "", repositoryTop); scopeErr == nil {
			installation = scope.Installation
		} else {
			// Package tests model a self-hosted installation directly at the
			// state root without copying a binary. In that layout root is the
			// installation, which is also ClassifyVerb's historical contract.
			installation = root
		}
	}
	return lease.ClassifyVerbAt(root, installation, callerPid)
}

func resolveProcessScopeWith(repo, installation string, repositoryTop func(string) (string, error)) (processScope, error) {
	installationExplicit := installation != ""
	checkout, err := upRepositoryScopeWith(repo, repositoryTop)
	if err != nil {
		return processScope{}, fmt.Errorf("%s is not inside a git repository", repo)
	}
	if installation != "" {
		installation, err = canonicalPath(installation)
		if err != nil {
			return processScope{}, err
		}
		if !regularFile(filepath.Join(installation, "bin", "metasystem")) {
			return processScope{}, fmt.Errorf("%s carries no engine", installation)
		}
	} else {
		for _, candidate := range []string{checkout, filepath.Join(checkout, "metasystem")} {
			if regularFile(filepath.Join(candidate, "bin", "metasystem")) {
				installation = candidate
				break
			}
		}
		if installation == "" {
			return processScope{}, fmt.Errorf("%s carries no metasystem installation", checkout)
		}
	}
	return processScope{Checkout: checkout, Installation: installation, InstallationExplicit: installationExplicit, Root: installation, Binary: filepath.Join(installation, "bin", "metasystem")}, nil
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func processTransition(scope processScope, scale int) *stoptransition.Transition {
	return &stoptransition.Transition{
		Root: scope.Root, Checkout: scope.Checkout, ScaleMilli: scale,
		Families: stoptransition.LocalFamilies(stoptransition.LocalConfig{
			Root: scope.Root, Checkout: scope.Checkout, Installation: scope.Installation,
			Binary: scope.Binary, ScaleMilli: scale,
		}),
	}
}

func processVerbRetryCommand(scope processScope, verb string) string {
	command := "metasystem " + verb + " --repo " + scope.Checkout
	if scope.InstallationExplicit {
		command += " --installation " + scope.Installation
	}
	return command
}

func runProcessStop(args []string) int {
	return runProcessStopWith(args, stateroot.RepositoryTop, lease.ClassifyAt)
}

func runProcessStopWith(args []string, repositoryTop func(string) (string, error), classify processCallerClassifier) int {
	scope, scale, code := parseProcessScopeWith("stop", args, repositoryTop)
	if code != 0 {
		return code
	}
	owners := defaultProcessOwners()
	owners.repositoryTop, owners.classify = repositoryTop, classify
	report, refusal := owners.stop(scope, scale)
	if refusal != nil {
		return refusal.print()
	}
	return printProcessReport(report)
}

// processRefusal is one process verb's refusal as its legacy call prints it:
// the sentence, the second line naming what to run, and the exit code.
type processRefusal struct {
	verb, checkout, sentence, second string
	code                             int
	// plain is printed alone by refusals that predate the two-line form.
	plain string
}

func (r *processRefusal) print() int {
	if r.plain != "" {
		fmt.Fprintln(os.Stderr, r.plain)
		return r.code
	}
	refuseProcessVerb(r.verb, r.checkout, r.sentence, r.second)
	return r.code
}

// processOwners are the checkout process owners one call uses: the
// classifier that proves a human terminal, the stop transition and the arm
// sequence. Production uses defaultProcessOwners; tests give each call its own.
type processOwners struct {
	repositoryTop func(string) (string, error)
	classify      processCallerClassifier
	transition    func(processScope, int) *stoptransition.Transition
	armSteps      func(processScope, int, processArmAuthority) ([]string, error)
}

// processArmAuthority is the human authority an arm was granted under.
type processArmAuthority struct {
	fixtureGranted          bool
	temporaryWord, reviewBy string
}

func defaultProcessOwners() processOwners {
	return processOwners{repositoryTop: stateroot.RepositoryTop, classify: lease.ClassifyAt, transition: processTransition, armSteps: armCheckoutSteps}
}

func (o processOwners) humanTerminal(scope processScope, verb, retry string) (bool, *processRefusal) {
	return humanTerminalCheck(scope.Root, scope.Installation, verb, o.repositoryTop, o.classify, retry)
}

// stop is the human-terminal checkout stop through the stop transition.
func (o processOwners) stop(scope processScope, scale int) (stoptransition.Report, *processRefusal) {
	crashStep, err := processStopCrashStep(scope.Root)
	if err != nil {
		return stoptransition.Report{}, &processRefusal{verb: "stop", checkout: scope.Checkout, sentence: err.Error(), second: "run: " + processVerbRetryCommand(scope, "stop"), code: 1}
	}
	if _, refusal := o.humanTerminal(scope, "metasystem stop", processVerbRetryCommand(scope, "stop")); refusal != nil {
		return stoptransition.Report{}, refusal
	}
	transition := o.transition(scope, scale)
	if crashStep != 0 {
		transition.AfterStep = func(step int) error {
			if step == crashStep {
				// The process exit is the crash: defers do not release the
				// transition lock or finalize its stopping fence.
				os.Exit(1)
			}
			return nil
		}
	}
	report, err := transition.Stop()
	if err != nil {
		return report, &processRefusal{verb: "stop", checkout: scope.Checkout, sentence: err.Error(), second: stopRefusalSecondLine(scope, err), code: 1}
	}
	return report, nil
}

// status reads every process family of the checkout without changing any.
func (o processOwners) status(scope processScope, scale int) (stoptransition.Report, error) {
	return o.transition(scope, scale).Status()
}

func stopRefusalSecondLine(scope processScope, err error) string {
	var unreadable *stopfence.RecordUnreadableError
	if errors.As(err, &unreadable) {
		return "run: " + processVerbRetryCommand(scope, "arm")
	}
	return "run: " + processVerbRetryCommand(scope, "status")
}

func processStopCrashStep(root string) (int, error) {
	raw := os.Getenv("METASYSTEM_STOP_CRASH_AFTER")
	if raw == "" {
		return 0, nil
	}
	if !fixtureauth.FixtureModeRoot(root) {
		return 0, fmt.Errorf("METASYSTEM_STOP_CRASH_AFTER is available only in a fixture-mode root")
	}
	step, err := strconv.Atoi(raw)
	if err != nil || step < 1 || step > 9 || strconv.Itoa(step) != raw {
		return 0, fmt.Errorf("METASYSTEM_STOP_CRASH_AFTER must name a numbered section-4 step from 1 through 9")
	}
	return step, nil
}

func runProcessStatus(args []string) int {
	return runProcessStatusWith(args, stateroot.RepositoryTop)
}

func runProcessStatusWith(args []string, repositoryTop func(string) (string, error)) int {
	scope, scale, code := parseProcessScopeWith("status", args, repositoryTop)
	if code != 0 {
		return code
	}
	report, err := defaultProcessOwners().status(scope, scale)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metasystem status: %v.\n", err)
		return 1
	}
	return printProcessReport(report)
}

func runProcessArm(args []string) int {
	return runProcessArmWith(args, stateroot.RepositoryTop, lease.ClassifyAt)
}

func runProcessArmWith(args []string, repositoryTop func(string) (string, error), classify processCallerClassifier) int {
	var temporaryWord, reviewBy string
	scope, scale, code := parseProcessScopeWith("arm", args, repositoryTop, func(flags *flag.FlagSet) {
		flags.StringVar(&temporaryWord, "temporary-human-word", "", "verbatim remote human authorization")
		flags.StringVar(&reviewBy, "review-by", "", "human re-approval date")
	})
	if code != 0 {
		return code
	}
	owners := defaultProcessOwners()
	owners.repositoryTop, owners.classify = repositoryTop, classify
	report, refusal := owners.arm(scope, scale, temporaryWord, reviewBy)
	if refusal != nil {
		for _, line := range report.Lines {
			fmt.Println(line)
		}
		return refusal.print()
	}
	return printProcessReport(report)
}

// arm is the human-terminal arm: the transition opens the stop fence and runs
// the steward arm and recovery-only up as its arm sequence.
func (o processOwners) arm(scope processScope, scale int, temporaryWord, reviewBy string) (stoptransition.Report, *processRefusal) {
	fixtureGranted, refusal := o.humanTerminal(scope, "metasystem arm", processVerbRetryCommand(scope, "arm"))
	if refusal != nil {
		return stoptransition.Report{}, refusal
	}
	if err := humanauthority.ValidateTemporaryWordPair(temporaryWord, reviewBy); err != nil {
		return stoptransition.Report{}, &processRefusal{verb: "arm", checkout: scope.Checkout, sentence: err.Error(), plain: "metasystem arm: " + err.Error(), code: 2}
	}
	transition := o.transition(scope, scale)
	authority := processArmAuthority{fixtureGranted: fixtureGranted, temporaryWord: temporaryWord, reviewBy: reviewBy}
	transition.ArmFunc = func() ([]string, error) { return o.armSteps(scope, scale, authority) }
	report, err := transition.Arm()
	if err != nil {
		return report, &processRefusal{verb: "arm", checkout: scope.Checkout, sentence: err.Error(), second: armRefusalSecondLine(scope, err), code: 1}
	}
	return report, nil
}

// processArmEffects are the effects of the arm sequence: seeding the landing
// ref, arming the steward, and starting the missing supervision rings.
type processArmEffects struct {
	seed func(root string) (stewardLandingRefSeed, error)
	arm  func(root, binary string, authority processArmAuthority) (string, error)
	up   func(up.Options) up.Result
}

func defaultProcessArmEffects() processArmEffects {
	return processArmEffects{
		seed: seedStewardLandingRef,
		arm: func(root, binary string, authority processArmAuthority) (string, error) {
			switch {
			case authority.temporaryWord != "":
				return steward.ArmTemporary(root, binary, authority.temporaryWord, authority.reviewBy)
			case authority.fixtureGranted:
				return steward.ArmFixture(root, binary)
			}
			return steward.Arm(root, binary)
		},
		up: up.Run,
	}
}

// armCheckoutSteps is the arm sequence with its production effects.
func armCheckoutSteps(scope processScope, scale int, authority processArmAuthority) ([]string, error) {
	return defaultProcessArmEffects().steps(scope, scale, authority)
}

// steps seeds the landing ref, arms the steward under the granted authority,
// and starts the missing supervision rings. Every step works in the
// checkout's state root; the installation supplies the engine.
func (effects processArmEffects) steps(scope processScope, scale int, authority processArmAuthority) ([]string, error) {
	if seed, seedErr := effects.seed(scope.Root); seedErr != nil {
		return nil, seedErr
	} else if seed.Ref != "" {
		// The durable git configuration is the result; arm's report does
		// not need a second provenance line for the seeding mechanism.
	}
	message, armErr := effects.arm(scope.Root, scope.Binary, authority)
	lines := []string{}
	if message != "" {
		lines = append(lines, message)
	}
	if armErr != nil {
		return lines, armErr
	}
	upResult := effects.up(up.Options{
		Root: scope.Root, MetasystemRoot: scope.Installation, Scope: scope.Checkout,
		Binary: scope.Binary, RecoverOnly: true, IfDown: true, WaitScaleMilli: scale,
		CallerPid: int64(os.Getpid()),
	})
	lines = append(lines, upResult.Lines()...)
	if upResult.ExitCode() != 0 {
		return lines, fmt.Errorf("up ended with outcome %s", upResult.Outcome)
	}
	return lines, nil
}

func armRefusalSecondLine(scope processScope, err error) string {
	armCommand := processVerbRetryCommand(scope, "arm")
	statusCommand := processVerbRetryCommand(scope, "status")
	stopCommand := processVerbRetryCommand(scope, "stop")
	second := "run: " + armCommand
	var stopInProgress *stoptransition.StopInProgressError
	var localSurvivor *stoptransition.LocalSurvivorError
	var unprobeableLocal *stoptransition.UnprobeableLocalSurvivorError
	var remoteJob *stoptransition.RemoteJobSurvivorError
	var remoteEvidence *stoptransition.RemoteJobEvidenceError
	var creatorClaim *stoptransition.CreatorClaimSurvivorError
	var unreadableFamily *stoptransition.UnreadableFamilySurvivorError
	var recordedReason *stoptransition.RecordedReasonSurvivorError
	var fencePublication *stoptransition.FencePublicationError
	switch {
	case errors.As(err, &stopInProgress):
		return "run: " + statusCommand
	case errors.As(err, &localSurvivor):
		return fmt.Sprintf("run: %s; if it survives a second stop, end pid %d yourself; it is listed with its start time", stopCommand, localSurvivor.Survivor.Pid)
	case errors.As(err, &unprobeableLocal):
		return "run: " + stopCommand
	case errors.As(err, &remoteJob):
		return remoteJob.Remedy() + "; then run: " + armCommand
	case errors.As(err, &remoteEvidence):
		return "run: " + stopCommand
	case errors.As(err, &creatorClaim):
		return "run: " + stopCommand
	case errors.As(err, &unreadableFamily), errors.As(err, &recordedReason), errors.As(err, &fencePublication):
		return "run: " + stopCommand
	}
	return second
}

func parseProcessScopeWith(verb string, args []string, repositoryTop func(string) (string, error), register ...func(*flag.FlagSet)) (processScope, int, int) {
	flags := flag.NewFlagSet("metasystem "+verb, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := pathFlag(flags, "repo", ".", "repository or path inside it")
	installation := flags.String("installation", "", "metasystem installation for this checkout")
	all := flags.Bool("all", false, "every checkout on this host")
	for _, add := range register {
		add(flags)
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return processScope{}, 0, 2
	}
	scope, err := resolveProcessScopeWith(*repo, *installation, repositoryTop)
	if err != nil {
		return processScope{}, 0, processScopeRefusal(verb, *repo, *installation, err)
	}
	if *all {
		return processScope{}, 0, refuseProcessVerb(verb, scope.Checkout, "the fleet form is not built yet", "run: "+processVerbRetryCommand(scope, verb))
	}
	scale := upWaitScale()
	if scale < 1 {
		fmt.Fprintf(os.Stderr, "metasystem %s: METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer\n", verb)
		return processScope{}, 0, 2
	}
	return scope, scale, 0
}

func processScopeRefusal(verb, repo, installation string, err error) int {
	sentence := err.Error()
	scope := processScope{Checkout: "<a path inside the checkout>", Installation: installation, InstallationExplicit: installation != ""}
	if strings.Contains(sentence, "carries no metasystem installation") {
		scope.Checkout = strings.TrimSuffix(sentence, " carries no metasystem installation")
		scope.Installation = "<dir>, where <dir> holds this checkout's bin/metasystem"
		scope.InstallationExplicit = true
	} else if strings.Contains(sentence, "carries no engine") {
		scope.Checkout = repo
		scope.Installation = "<dir>, where <dir> holds this checkout's bin/metasystem"
		scope.InstallationExplicit = true
	}
	return refuseProcessVerb(verb, repo, sentence, "run: "+processVerbRetryCommand(scope, verb))
}

func refuseProcessVerb(verb, checkout, sentence, second string) int {
	sentence = strings.TrimSuffix(strings.TrimSpace(sentence), ".")
	fmt.Fprintf(os.Stderr, "metasystem %s: %s.\n", verb, sentence)
	fmt.Fprintln(os.Stderr, second)
	return 1
}

func printProcessReport(report stoptransition.Report) int {
	for _, line := range report.Lines {
		fmt.Println(line)
	}
	return report.ExitCode
}

// requireHumanTerminal is the common classifier gate for process stopping and
// steward enrollment. Fixture-granted HUMAN classifications are explicit
// authority, while every other class is refused.
func requireHumanTerminal(repo, verb string, retryCommands ...string) (fixtureGranted, authorized bool) {
	metasystemRoot, err := upMetasystemRoot("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot resolve the installed engine: %v\n", verb, err)
		return false, false
	}
	return requireHumanTerminalAt(repo, metasystemRoot, verb, retryCommands...)
}

func requireHumanTerminalAt(repo, metasystemRoot, verb string, retryCommands ...string) (fixtureGranted, authorized bool) {
	return requireHumanTerminalAtWith(repo, metasystemRoot, verb, stateroot.RepositoryTop, lease.ClassifyAt, retryCommands...)
}

func requireHumanTerminalAtWith(repo, metasystemRoot, verb string, repositoryTop func(string) (string, error), classify processCallerClassifier, retryCommands ...string) (fixtureGranted, authorized bool) {
	fixtureGranted, refusal := humanTerminalCheck(repo, metasystemRoot, verb, repositoryTop, classify, retryCommands...)
	if refusal != nil {
		refusal.print()
		return false, false
	}
	return fixtureGranted, true
}

// humanTerminalCheck classifies the caller's terminal and returns the
// refusal an agent, unreadable ancestry or blocking supporting data earns.
func humanTerminalCheck(repo, metasystemRoot, verb string, repositoryTop func(string) (string, error), classify processCallerClassifier, retryCommands ...string) (bool, *processRefusal) {
	retryCommand := ""
	checkout := ""
	if strings.HasPrefix(verb, "metasystem ") {
		checkout, _ = upRepositoryScopeWith(repo, repositoryTop)
		retryCommand = verb + " --repo " + checkout
		if len(retryCommands) > 0 && retryCommands[0] != "" {
			retryCommand = retryCommands[0]
		}
	}
	name := strings.TrimPrefix(verb, "metasystem ")
	classification, err := classify(repo, metasystemRoot, int64(os.Getppid()))
	if err != nil {
		if strings.HasPrefix(verb, "metasystem ") {
			if refusal := classificationDataRefusal(name, checkout, retryCommand, err); refusal != nil {
				return false, refusal
			}
			return false, &processRefusal{verb: name, checkout: checkout, sentence: "the caller's ancestry could not be read: " + err.Error(), second: "at an agent-free terminal, run: " + retryCommand, code: 1}
		}
		return false, &processRefusal{verb: name, plain: fmt.Sprintf("%s: human ancestry proof failed: %v", verb, err), code: 1}
	}
	if classification.Class != lease.ClassHuman {
		if strings.HasPrefix(verb, "metasystem ") {
			return false, &processRefusal{verb: name, checkout: checkout, sentence: name + " is a human act at a terminal; this caller is " + classification.Class, second: "at an agent-free terminal, run: " + retryCommand, code: 1}
		}
		return false, &processRefusal{verb: name, plain: fmt.Sprintf("%s: explicit engine enrollment requires an agent-free terminal; caller classified %s", verb, classification.Class), code: 1}
	}
	return classification.FixtureGranted, nil
}

func classificationDataRefusal(verb, checkout, retryCommand string, err error) *processRefusal {
	var failure *lease.ClassificationFailure
	if !errors.As(err, &failure) || failure.Kind != lease.ClassificationSupportingData {
		return nil
	}
	input := failure.Source
	if failure.Path != "" {
		input += " " + failure.Path
	}
	second := "repair " + failure.Path + ", then at an agent-free terminal, run: " + retryCommand
	return &processRefusal{verb: verb, checkout: checkout, sentence: "caller classification is blocked by " + input + ": " + failure.Reason(), second: second, code: 1}
}

func runStopFenceCreatingClose(args []string) int {
	flags := flag.NewFlagSet("stopfence creating-close", flag.ContinueOnError)
	claim := flags.String("claim", "", "creation claim path")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *claim == "" {
		fmt.Fprintln(os.Stderr, "stopfence creating-close: --claim is required")
		return 2
	}
	if err := stopfence.CloseClaim(*claim); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "stopfence creating-close:", err)
		return 1
	}
	return 0
}

// missionFenceBeforeArm gives a closed checkout its stopped answer before the
// mission launcher can reach supervision arming or any gate that depends on
// live supervision.
func missionFenceBeforeArm(root, mode string) (int64, int) {
	return missionFenceBeforeArmWith(root, mode, stateroot.RepositoryTop, lease.ClassifyAt)
}

func missionFenceBeforeArmWith(root, mode string, repositoryTop func(string) (string, error), classify processCallerClassifier) (int64, int) {
	record, err := stopfence.Read(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mission "+mode+":", err)
		return 0, 1
	}
	if record.State == stopfence.StateOpen {
		return record.Generation, 0
	}
	scope, scopeErr := resolveProcessScopeWith(root, "", repositoryTop)
	if scopeErr != nil {
		fmt.Fprintln(os.Stderr, "mission "+mode+":", scopeErr)
		return 0, 1
	}
	retryCommand := fmt.Sprintf("metasystem mission %s --root %s --mission <id>", mode, scope.Checkout)
	// Mission state and its stop fence remain application-owned. Only the
	// process-control verbs move their supervision/accounting root to the
	// authenticated installation.
	classification, classifyErr := classify(scope.Checkout, scope.Installation, int64(os.Getpid()))
	if classifyErr != nil {
		if refuseClassificationData("mission "+mode, scope.Checkout, retryCommand, classifyErr) {
			return 0, 1
		}
	}
	if classifyErr == nil && classification.Class == lease.ClassHuman {
		generation, openErr := processTransition(scope, upWaitScale()).OpenFence("mission-" + mode)
		if openErr != nil {
			fmt.Fprintln(os.Stderr, "mission "+mode+":", openErr)
			fmt.Fprintln(os.Stderr, armRefusalSecondLine(scope, openErr))
			return 0, 1
		}
		return generation, 0
	}
	description, descriptionErr := stopfence.ClosedDescription(record, record.Checkout)
	if descriptionErr != nil {
		fmt.Fprintln(os.Stderr, "mission "+mode+":", descriptionErr)
		return 0, 1
	}
	fmt.Fprintln(os.Stderr, description)
	if stopfence.Completed(record) {
		fmt.Fprintln(os.Stderr, "at an agent-free terminal, run: "+retryCommand)
	} else {
		command, commandErr := stopfence.ClosedCommand(record, record.Checkout)
		if commandErr != nil {
			fmt.Fprintln(os.Stderr, "mission "+mode+":", commandErr)
			return 0, 1
		}
		fmt.Fprintln(os.Stderr, "at an agent-free terminal, run: "+command)
	}
	return 0, 1
}

func refuseClassificationData(verb, checkout, retryCommand string, err error) bool {
	refusal := classificationDataRefusal(verb, checkout, retryCommand, err)
	if refusal == nil {
		return false
	}
	refusal.print()
	return true
}
