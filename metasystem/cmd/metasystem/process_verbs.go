package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
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
			Binary: scope.Binary, ScaleMilli: scale, CancelJob: delegateCancel(scope.Installation),
		}),
	}
}

// publicProcessVerb is the public object-action form of a process verb, the
// spelling every label and retry line names.
func publicProcessVerb(verb string) string {
	switch verb {
	case "stop":
		return "system stop"
	case "arm":
		return "system start"
	case "status":
		return "system status"
	}
	return verb
}

func processVerbRetryCommand(scope processScope, verb string) string {
	command := "metasystem " + publicProcessVerb(verb) + " --repo " + scope.Checkout
	if scope.InstallationExplicit {
		command += " --installation " + scope.Installation
	}
	return command
}

// processRefusal is one process verb's refusal as its legacy call prints it:
// the sentence, the second line naming what to run, and the exit code.
type processRefusal struct {
	verb, checkout, sentence, second string
	code                             int
	// plain is printed alone by refusals that predate the two-line form.
	plain string
	// next is the command that resolves the refusal and nextReason what
	// goes with it; details are what only --verbose shows.
	next       []string
	nextReason string
	details    []string
}

// printTo writes the refusal to w.
func (r *processRefusal) printTo(w io.Writer) int {
	if r.plain != "" {
		fmt.Fprintln(w, r.plain)
		return r.code
	}
	refuseProcessVerbTo(w, r.verb, r.checkout, r.sentence, r.second)
	return r.code
}

// processOwners are the checkout process owners one call uses: the
// classifier that proves a human terminal, the stop transition and the arm
// sequence. Production uses defaultProcessOwners; tests give each call its own.
type processOwners struct {
	repositoryTop func(string) (string, error)
	classify      processCallerClassifier
	transition    func(processScope, int) *stoptransition.Transition
	armSteps      func(processScope, int, processArmAuthority) (processArmResult, error)
	// evidenceRoot resolves the evidence root a start says; nil is the
	// owner over the process environment.
	evidenceRoot func(conf string) (config.EvidenceRoot, error)
}

// processArmResult is the arm sequence's report: its lines, and whether it
// found everything already running (the steward's same live runner kept and
// no helper started), with that runner's pid.
type processArmResult struct {
	lines     []string
	unchanged bool
	runnerPid int64
}

// processArmAuthority is the human authority an arm was granted under.
type processArmAuthority struct {
	fixtureGranted          bool
	temporaryWord, reviewBy string
}

func defaultProcessOwners() processOwners {
	return processOwners{repositoryTop: stateroot.RepositoryTop, classify: personClassifyAt, transition: processTransition, armSteps: armCheckoutSteps}
}

func (o processOwners) humanTerminal(scope processScope, verb, retry string) (bool, *processRefusal) {
	return humanTerminalCheck(scope.Root, scope.Installation, verb, o.repositoryTop, o.classify, retry)
}

// stop is the human-terminal checkout stop through the stop transition.
func (o processOwners) stop(scope processScope, scale int) (stoptransition.Report, *processRefusal) {
	crashStep, err := processStopCrashStep(scope.Root)
	if err != nil {
		return stoptransition.Report{}, &processRefusal{verb: "stop", checkout: scope.Checkout, sentence: err.Error() + ", so nothing was stopped",
			second: "run: env -u " + processStopCrashVariable + " " + processVerbRetryCommand(scope, "stop"), code: 1}
	}
	if _, refusal := o.humanTerminal(scope, "metasystem system stop", processVerbRetryCommand(scope, "stop")); refusal != nil {
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

// processStopCrashVariable is the test setting that crashes a stop after
// one of its numbered steps (section 4 of the stop design).
const processStopCrashVariable = "METASYSTEM_STOP_CRASH_AFTER"

func processStopCrashStep(root string) (int, error) {
	raw := os.Getenv(processStopCrashVariable)
	if raw == "" {
		return 0, nil
	}
	if !fixtureauth.FixtureModeRoot(root) {
		return 0, errors.New("a test setting that crashes the stop is set, and works only in a test installation")
	}
	step, err := strconv.Atoi(raw)
	if err != nil || step < 1 || step > 9 || strconv.Itoa(step) != raw {
		return 0, errors.New("a test setting that crashes the stop names no stop step (1 through 9)")
	}
	return step, nil
}

// arm is the human-terminal arm: the transition opens the stop fence and runs
// the steward arm and recovery-only up as its arm sequence.
func (o processOwners) arm(scope processScope, scale int, temporaryWord, reviewBy string) (stoptransition.Report, *processRefusal) {
	fixtureGranted, refusal := o.humanTerminal(scope, "metasystem system start", processVerbRetryCommand(scope, "arm"))
	if refusal != nil {
		return stoptransition.Report{}, refusal
	}
	if err := humanauthority.ValidateTemporaryWordPair(temporaryWord, reviewBy); err != nil {
		return stoptransition.Report{}, &processRefusal{verb: "arm", checkout: scope.Checkout, sentence: err.Error(), plain: "metasystem system start: " + err.Error(), code: 2}
	}
	resolveEvidence := o.evidenceRoot
	if resolveEvidence == nil {
		resolveEvidence = func(conf string) (config.EvidenceRoot, error) {
			return config.ResolveEvidenceRoot(config.EvidenceRootParams{ConfPath: conf})
		}
	}
	evidence, err := resolveEvidence(filepath.Join(scope.Installation, "metasystem.conf"))
	if err != nil {
		return stoptransition.Report{}, &processRefusal{verb: "arm", checkout: scope.Checkout, sentence: err.Error(), plain: "metasystem system start: " + err.Error(), code: 1}
	}
	transition := o.transition(scope, scale)
	authority := processArmAuthority{fixtureGranted: fixtureGranted, temporaryWord: temporaryWord, reviewBy: reviewBy}
	var steps processArmResult
	transition.ArmFunc = func() ([]string, error) {
		var err error
		steps, err = o.armSteps(scope, scale, authority)
		return steps.lines, err
	}
	report, err := transition.Arm()
	if err != nil {
		return report, &processRefusal{verb: "arm", checkout: scope.Checkout, sentence: err.Error(), second: armRefusalSecondLine(scope, err), code: 1}
	}
	report.Lines = append([]string{evidence.Line()}, report.Lines...)
	// The start is a repeat only when the fence was already open and the arm
	// sequence found everything running; otherwise it started something.
	report.Unchanged = report.Unchanged && steps.unchanged
	if report.Unchanged {
		report.Lines = append(report.Lines, alreadyRunsSentence(steps.runnerPid, report.Since))
	}
	return report, nil
}

// alreadyRunsSentence is a repeated start's answer: what already runs.
func alreadyRunsSentence(pid int64, since string) string {
	return fmt.Sprintf("MetaSystem already runs for this checkout (pid %d since %s)", pid, since)
}

// processArmEffects are the effects of the arm sequence: seeding the landing
// ref, arming the steward, and starting the missing supervision rings.
type processArmEffects struct {
	seed func(root string) (stewardLandingRefSeed, error)
	arm  func(root, binary string, authority processArmAuthority) (string, error)
	up   func(up.Options) up.Result
	// runner is the steward's live runner pid, read before and after the
	// arm: the same live runner on both sides is an arm that changed
	// nothing. Nil reads as changed.
	runner func(root string) (int64, bool)
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
		runner: func(root string) (int64, bool) {
			record, alive := steward.LiveRunner(root)
			return record.Pid, alive
		},
	}
}

// armCheckoutSteps is the arm sequence with its production effects.
func armCheckoutSteps(scope processScope, scale int, authority processArmAuthority) (processArmResult, error) {
	return defaultProcessArmEffects().steps(scope, scale, authority)
}

// steps seeds the landing ref, arms the steward under the granted authority,
// and starts the missing supervision rings. Every step works in the
// checkout's state root; the installation supplies the engine.
func (effects processArmEffects) steps(scope processScope, scale int, authority processArmAuthority) (processArmResult, error) {
	seed, seedErr := effects.seed(scope.Root)
	if seedErr != nil {
		return processArmResult{}, seedErr
	}
	// The durable git configuration is the seed's result; arm's report does
	// not need a second provenance line for the seeding mechanism.
	beforePid, beforeLive := int64(0), false
	if effects.runner != nil {
		beforePid, beforeLive = effects.runner(scope.Root)
	}
	message, armErr := effects.arm(scope.Root, scope.Binary, authority)
	lines := []string{}
	if message != "" {
		lines = append(lines, message)
	}
	if armErr != nil {
		return processArmResult{lines: lines}, armErr
	}
	afterPid, afterLive := int64(0), false
	if effects.runner != nil {
		afterPid, afterLive = effects.runner(scope.Root)
	}
	stewardKept := effects.runner != nil && seed.Ref == "" && beforeLive && afterLive && beforePid == afterPid
	upResult := effects.up(up.Options{
		Root: scope.Root, MetasystemRoot: scope.Installation, Scope: scope.Checkout,
		Binary: scope.Binary, RecoverOnly: true, IfDown: true, WaitScaleMilli: scale,
		CallerPid: int64(os.Getpid()),
	})
	lines = append(lines, upResult.Lines()...)
	if upResult.ExitCode() != 0 {
		return processArmResult{lines: lines}, fmt.Errorf("up ended with outcome %s", upResult.Outcome)
	}
	return processArmResult{lines: lines, unchanged: stewardKept && upResult.Outcome == "recovery-not-needed", runnerPid: afterPid}, nil
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

func refuseProcessVerbTo(w io.Writer, verb, checkout, sentence, second string) int {
	sentence = strings.TrimSuffix(strings.TrimSpace(sentence), ".")
	fmt.Fprintf(w, "metasystem %s: %s.\n", publicProcessVerb(verb), sentence)
	fmt.Fprintln(w, second)
	return 1
}

// requireHumanTerminal is the common classifier gate for process stopping and
// steward enrollment. Fixture-granted HUMAN classifications are explicit
// authority, while every other class is refused.
func requireHumanTerminal(stderr io.Writer, repo, verb string, retryCommands ...string) (fixtureGranted, authorized bool) {
	metasystemRoot, err := upMetasystemRoot("")
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot resolve the installed engine: %v\n", verb, err)
		return false, false
	}
	return requireHumanTerminalAtWith(stderr, repo, metasystemRoot, verb, stateroot.RepositoryTop, personClassifyAt, retryCommands...)
}

// requireHumanTerminalAtWith prints a refusal on stderr, the invocation's.
func requireHumanTerminalAtWith(stderr io.Writer, repo, metasystemRoot, verb string, repositoryTop func(string) (string, error), classify processCallerClassifier, retryCommands ...string) (fixtureGranted, authorized bool) {
	fixtureGranted, refusal := humanTerminalCheck(repo, metasystemRoot, verb, repositoryTop, classify, retryCommands...)
	if refusal != nil {
		refusal.printTo(stderr)
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
			return false, &processRefusal{verb: name, checkout: checkout, sentence: "the processes behind this shell couldn't be read",
				second: "in a terminal you opened yourself, run: " + retryCommand, next: shellWords(retryCommand), nextReason: "try again, in a terminal you opened yourself",
				details: []string{"refused because: " + err.Error()}, code: 1}
		}
		return false, &processRefusal{verb: name, plain: fmt.Sprintf("%s: the processes behind this shell couldn't be read (%v); run it in a terminal you opened yourself", verb, err), code: 1}
	}
	if classification.Class != lease.ClassHuman {
		origin := shellOrigin(classification.Class)
		if strings.HasPrefix(verb, "metasystem ") {
			return false, &processRefusal{verb: name, checkout: checkout, sentence: origin,
				second: "in a terminal you opened yourself, run: " + retryCommand, next: shellWords(retryCommand), nextReason: "in a terminal you opened yourself",
				details: []string{name + " is a person's act; this shell's class is " + classification.Class}, code: 1}
		}
		return false, &processRefusal{verb: name, plain: fmt.Sprintf("%s: %s; run it in a terminal you opened yourself", verb, origin), code: 1}
	}
	return classification.FixtureGranted, nil
}

// shellOrigin says in plain words who started a shell that is not a
// person's, from its lease class.
func shellOrigin(class string) string {
	switch class {
	case lease.ClassDelegate, lease.ClassMain:
		return "an agent started this shell"
	case lease.ClassSupervision, lease.ClassSteward:
		return "MetaSystem's own machinery started this shell"
	}
	return "this shell can't be traced to a terminal a person opened"
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
	second := "repair " + failure.Path + ", then in a terminal you opened yourself, run: " + retryCommand
	return &processRefusal{verb: verb, checkout: checkout, sentence: "who started this shell can't be told: " + input + " is damaged (" + failure.Reason() + ")", second: second, code: 1}
}

// missionFenceBeforeArmFor is the fence check with its caller and report
// stream explicit: the process a human classification starts from (the
// launching command supplies itself) and where refusals are written.
func missionFenceBeforeArmFor(caller ownercall.Process, stderr io.Writer, root, mode string, repositoryTop func(string) (string, error), classify processCallerClassifier) (int64, int) {
	record, err := stopfence.Read(root)
	if err != nil {
		fmt.Fprintln(stderr, "mission "+mode+":", err)
		return 0, 1
	}
	if record.State == stopfence.StateOpen {
		return record.Generation, 0
	}
	scope, scopeErr := resolveProcessScopeWith(root, "", repositoryTop)
	if scopeErr != nil {
		fmt.Fprintln(stderr, "mission "+mode+":", scopeErr)
		return 0, 1
	}
	retryCommand := fmt.Sprintf("metasystem mission %s --root %s --mission <id>", mode, scope.Checkout)
	// Mission state and its stop fence remain application-owned. Only the
	// process-control verbs move their supervision/accounting root to the
	// authenticated installation.
	classification, classifyErr := classify(scope.Checkout, scope.Installation, caller.Pid)
	if classifyErr != nil {
		if refusal := classificationDataRefusal("mission "+mode, scope.Checkout, retryCommand, classifyErr); refusal != nil {
			refusal.printTo(stderr)
			return 0, 1
		}
	}
	if classifyErr == nil && classification.Class == lease.ClassHuman {
		generation, openErr := processTransition(scope, upWaitScale()).OpenFence("mission-" + mode)
		if openErr != nil {
			fmt.Fprintln(stderr, "mission "+mode+":", openErr)
			fmt.Fprintln(stderr, armRefusalSecondLine(scope, openErr))
			return 0, 1
		}
		return generation, 0
	}
	description, descriptionErr := stopfence.ClosedDescription(record, record.Checkout)
	if descriptionErr != nil {
		fmt.Fprintln(stderr, "mission "+mode+":", descriptionErr)
		return 0, 1
	}
	fmt.Fprintln(stderr, description)
	if stopfence.Completed(record) {
		fmt.Fprintln(stderr, "at an agent-free terminal, run: "+retryCommand)
	} else {
		command, commandErr := stopfence.ClosedCommand(record, record.Checkout)
		if commandErr != nil {
			fmt.Fprintln(stderr, "mission "+mode+":", commandErr)
			return 0, 1
		}
		fmt.Fprintln(stderr, "at an agent-free terminal, run: "+command)
	}
	return 0, 1
}
