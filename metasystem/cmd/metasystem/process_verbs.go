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

var classifyProcessVerbCaller = lease.ClassifyAt

// classifyVerbCaller keeps checkout-owned lease state separate from the
// installed adapters that identify runtimes. Production uses the engine that
// is executing the verb; source-tree and fixture binaries resolve the target
// checkout's installation with the same rule as stop, status, and arm.
func classifyVerbCaller(root string, callerPid int64) (lease.ClassifyResult, error) {
	installation, err := upMetasystemRoot("")
	if err != nil {
		if scope, scopeErr := resolveProcessScope(root, ""); scopeErr == nil {
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

func resolveProcessScope(repo, installation string) (processScope, error) {
	installationExplicit := installation != ""
	checkout, err := upRepositoryScope(repo)
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
	root, err := stateroot.RootForInstallation(installation)
	if err != nil {
		return processScope{}, err
	}
	return processScope{Checkout: checkout, Installation: installation, InstallationExplicit: installationExplicit, Root: root, Binary: filepath.Join(installation, "bin", "metasystem")}, nil
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
	scope, scale, code := parseProcessScope("stop", args)
	if code != 0 {
		return code
	}
	crashStep, err := processStopCrashStep(scope.Root)
	if err != nil {
		return refuseProcessVerb("stop", scope.Checkout, err.Error(), "run: "+processVerbRetryCommand(scope, "stop"))
	}
	if _, authorized := requireHumanTerminalAt(scope.Root, scope.Installation, "metasystem stop", processVerbRetryCommand(scope, "stop")); !authorized {
		return 1
	}
	transition := processTransition(scope, scale)
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
		return refuseProcessVerb("stop", scope.Checkout, err.Error(), stopRefusalSecondLine(scope, err))
	}
	return printProcessReport(report)
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
	scope, scale, code := parseProcessScope("status", args)
	if code != 0 {
		return code
	}
	report, err := processTransition(scope, scale).Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "metasystem status: %v.\n", err)
		return 1
	}
	return printProcessReport(report)
}

func runProcessArm(args []string) int {
	var temporaryWord, reviewBy string
	scope, scale, code := parseProcessScope("arm", args, func(flags *flag.FlagSet) {
		flags.StringVar(&temporaryWord, "temporary-human-word", "", "verbatim remote human authorization")
		flags.StringVar(&reviewBy, "review-by", "", "human re-approval date")
	})
	if code != 0 {
		return code
	}
	fixtureGranted, authorized := requireHumanTerminalAt(scope.Root, scope.Installation, "metasystem arm", processVerbRetryCommand(scope, "arm"))
	if !authorized {
		return 1
	}
	if err := humanauthority.ValidateTemporaryWordPair(temporaryWord, reviewBy); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem arm:", err)
		return 2
	}
	transition := processTransition(scope, scale)
	transition.ArmFunc = func() ([]string, error) {
		if seed, seedErr := seedStewardLandingRef(scope.Root); seedErr != nil {
			return nil, seedErr
		} else if seed.Ref != "" {
			// The durable git configuration is the result; arm's report does
			// not need a second provenance line for the seeding mechanism.
		}
		var message string
		var armErr error
		switch {
		case temporaryWord != "":
			message, armErr = steward.ArmTemporary(scope.Root, scope.Binary, temporaryWord, reviewBy)
		case fixtureGranted:
			message, armErr = steward.ArmFixture(scope.Root, scope.Binary)
		default:
			message, armErr = steward.Arm(scope.Root, scope.Binary)
		}
		lines := []string{}
		if message != "" {
			lines = append(lines, message)
		}
		if armErr != nil {
			return lines, armErr
		}
		upResult := up.Run(up.Options{
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
	report, err := transition.Arm()
	if err != nil {
		for _, line := range report.Lines {
			fmt.Println(line)
		}
		return refuseProcessVerb("arm", scope.Checkout, err.Error(), armRefusalSecondLine(scope, err))
	}
	return printProcessReport(report)
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

func parseProcessScope(verb string, args []string, register ...func(*flag.FlagSet)) (processScope, int, int) {
	flags := flag.NewFlagSet("metasystem "+verb, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository or path inside it")
	installation := flags.String("installation", "", "metasystem installation for this checkout")
	all := flags.Bool("all", false, "every checkout on this host")
	for _, add := range register {
		add(flags)
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return processScope{}, 0, 2
	}
	scope, err := resolveProcessScope(*repo, *installation)
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
	retryCommand := ""
	if strings.HasPrefix(verb, "metasystem ") {
		checkout, _ := upRepositoryScope(repo)
		retryCommand = verb + " --repo " + checkout
		if len(retryCommands) > 0 && retryCommands[0] != "" {
			retryCommand = retryCommands[0]
		}
	}
	classification, err := classifyProcessVerbCaller(repo, metasystemRoot, int64(os.Getppid()))
	if err != nil {
		if strings.HasPrefix(verb, "metasystem ") {
			checkout, _ := upRepositoryScope(repo)
			if refuseClassificationData(strings.TrimPrefix(verb, "metasystem "), checkout, retryCommand, err) {
				return false, false
			}
			refuseProcessVerb(strings.TrimPrefix(verb, "metasystem "), checkout, "the caller's ancestry could not be read: "+err.Error(), "at an agent-free terminal, run: "+retryCommand)
		} else {
			fmt.Fprintf(os.Stderr, "%s: human ancestry proof failed: %v\n", verb, err)
		}
		return false, false
	}
	if classification.Class != lease.ClassHuman {
		if strings.HasPrefix(verb, "metasystem ") {
			checkout, _ := upRepositoryScope(repo)
			name := strings.TrimPrefix(verb, "metasystem ")
			refuseProcessVerb(name, checkout, name+" is a human act at a terminal; this caller is "+classification.Class, "at an agent-free terminal, run: "+retryCommand)
		} else {
			fmt.Fprintf(os.Stderr, "%s: explicit engine enrollment requires an agent-free terminal; caller classified %s\n", verb, classification.Class)
		}
		return false, false
	}
	return classification.FixtureGranted, true
}

func refuseClassificationData(verb, checkout, retryCommand string, err error) bool {
	var failure *lease.ClassificationFailure
	if !errors.As(err, &failure) || failure.Kind != lease.ClassificationSupportingData {
		return false
	}
	input := failure.Source
	if failure.Path != "" {
		input += " " + failure.Path
	}
	second := "repair " + failure.Path + ", then at an agent-free terminal, run: " + retryCommand
	refuseProcessVerb(verb, checkout, "caller classification is blocked by "+input+": "+failure.Reason(), second)
	return true
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
	record, err := stopfence.Read(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mission "+mode+":", err)
		return 0, 1
	}
	if record.State == stopfence.StateOpen {
		return record.Generation, 0
	}
	scope, scopeErr := resolveProcessScope(root, "")
	if scopeErr != nil {
		fmt.Fprintln(os.Stderr, "mission "+mode+":", scopeErr)
		return 0, 1
	}
	retryCommand := fmt.Sprintf("metasystem mission %s --root %s --mission <id>", mode, scope.Checkout)
	classification, classifyErr := classifyProcessVerbCaller(scope.Root, scope.Installation, int64(os.Getpid()))
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
