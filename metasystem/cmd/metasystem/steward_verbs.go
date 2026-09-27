package main

// The steward family: the idle watchdog's decision surface. check
// reads and reports without side effects; tick folds one observation
// into the evidence and reports what the schedule glue should do;
// status shows the operator everything pending. The revive action
// itself lands with the dispatch continuation mode.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	channelphase "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	dispatchpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func stewardCensusFor(repo string) steward.WorkerCensus {
	return steward.RuntimeWorkerCensus{
		MetasystemRoot: repo,
		ProcessFile:    os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE"),
	}
}

var stewardHealthNow = time.Now
var stewardPreviewHealthAt = steward.PreviewHealthAt
var stewardObserveHealth = steward.ObserveHealth

func stewardClockRoot(explicit, fallback string) string {
	if explicit != "" {
		return explicit
	}
	if installed, err := upMetasystemRoot(""); err == nil {
		return installed
	}
	return fallback
}

func stewardFixtureNow(root string) (time.Time, bool, error) {
	if !fixtureauth.FixtureModeRoot(root) {
		return time.Time{}, false, nil
	}
	authorization, err := fixtureauth.New(root)
	if err != nil {
		return time.Time{}, false, err
	}
	return authorization.Clock().GoalNow()
}

func stewardFixtureTickConfig(repo, root string) (steward.TickConfig, error) {
	now, ok, err := stewardFixtureNow(stewardClockRoot(root, repo))
	if err != nil || !ok {
		return steward.TickConfig{}, err
	}
	return steward.TickConfig{Now: now}, nil
}

func stewardRunClockRoot(repo string) string {
	root := stewardClockRoot("", repo)
	top, err := canonicalPath(repo)
	if err != nil {
		return root
	}
	installed, err := steward.VerifyIdentity(steward.RepoIdentityPath(top), top)
	if err != nil {
		return root
	}
	return filepath.Dir(filepath.Dir(installed.InstallPath))
}

// runStewardHealth prints every role on one line and returns the aggregate
// health code: zero healthy, one when any role is dead, two when unknown is
// the worst result.
func runStewardHealth(args []string) int {
	flags := flag.NewFlagSet("health", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	metasystemRoot := flags.String("metasystem-root", "", "installed metasystem root (defaults to checkout root)")
	hookPreview := flags.Bool("hook-preview", false, "render current hook facts without advancing the tick-owned alert breaker (internal)")
	format := flags.String("format", "text", "health output format: text or json")
	if flags.Parse(args) != nil {
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintln(os.Stderr, "health: --format must be text or json")
		return 2
	}
	if *format == "json" && !*hookPreview {
		fmt.Fprintln(os.Stderr, "health: --format=json requires --hook-preview")
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "health: --repo is required")
		return 2
	}
	if *hookPreview {
		return writeHookHealthPreview(*repo, *metasystemRoot, *format == "json", os.Stdout, os.Stderr)
	}
	clockRoot := stewardClockRoot(*metasystemRoot, *repo)
	if *metasystemRoot == "" {
		*metasystemRoot = *repo
	}
	now := stewardHealthNow().UTC()
	if fixtureNow, ok, err := stewardFixtureNow(clockRoot); err != nil {
		fmt.Fprintln(os.Stderr, "health: fixture clock:", err)
		return 2
	} else if ok {
		now = fixtureNow
	}
	verdict, err := stewardObserveHealth(*repo, now, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "health: health evidence is unknown: %v\n", err)
		return 2
	}
	alertView := verdict
	alertView.ShouldAlert = false
	if _, err := steward.UpdateAlertEpisodes(*repo, alertView, verdict.Line(), now); err != nil {
		fmt.Fprintf(os.Stderr, "health: alert episode state is unknown: %v\n", err)
		return 2
	}
	fmt.Println(verdict.Line())
	code := verdict.ExitCode()
	prober, processes, _, enumErr := census.FixtureSurvivorSource(*metasystemRoot)
	if enumErr != nil {
		fmt.Printf("fixture-survivors: process table is unreadable: %v\n", enumErr)
		return code
	}
	lines, certain, scanErr := census.FixtureSurvivorLines(prober, processes)
	if scanErr != nil {
		fmt.Printf("fixture-survivors: process table is unreadable: %v\n", scanErr)
		return code
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	if code == 0 && certain {
		return 1
	}
	return code
}

// writeHookHealthPreview renders the hook's health preview without
// advancing the tick-owned alert breaker: the Stop hook's health facts.
func writeHookHealthPreview(repo, metasystemRoot string, asJSON bool, stdout, stderr io.Writer) int {
	clockRoot := stewardClockRoot(metasystemRoot, repo)
	if metasystemRoot == "" {
		metasystemRoot = repo
	}
	now := stewardHealthNow().UTC()
	if fixtureNow, ok, err := stewardFixtureNow(clockRoot); err != nil {
		fmt.Fprintln(stderr, "health: fixture clock:", err)
		return 2
	} else if ok {
		now = fixtureNow
	}
	verdict := stewardPreviewHealthAt(repo, metasystemRoot, now, nil)
	if asJSON {
		if err := json.NewEncoder(stdout).Encode(steward.NewHookHealthPreview(verdict)); err != nil {
			fmt.Fprintln(stderr, "health: encode hook preview:", err)
			return 2
		}
	} else {
		fmt.Fprintln(stdout, verdict.Line())
	}
	return verdict.ExitCode()
}

func runHealthAcknowledgeAlert(args []string) int {
	flags := flag.NewFlagSet("health acknowledge-alert", flag.ContinueOnError)
	episodeID := flags.String("episode", "", "alert episode id")
	repo := pathFlag(flags, "repo", ".", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *episodeID == "" {
		fmt.Fprintln(os.Stderr, "health acknowledge-alert: --episode is required")
		return 2
	}
	// L8 will replace this observed invoker record with enrolled-terminal
	// ancestry enforcement. Until then this records the immediate caller
	// exactly and makes no claim that it was an agent-free terminal.
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		fmt.Fprintln(os.Stderr, "health acknowledge-alert: the immediate caller identity is unavailable")
		return 1
	}
	argvDigest := ""
	if exact.ArgvKnown {
		argv := sha256.Sum256([]byte(strings.Join(exact.Argv, "\x00")))
		argvDigest = hex.EncodeToString(argv[:])
	}
	invoker := steward.AlertInvoker{
		Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), PidStartTicks: exact.StartTicks,
		BootID: exact.BootID, UID: os.Getuid(), ArgvDigest: argvDigest,
	}
	episode, err := steward.AcknowledgeAlert(*repo, *episodeID, invoker, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "health acknowledge-alert: %v\n", err)
		return 1
	}
	fmt.Printf("acknowledged alert %s\n", episode.EpisodeID)
	return 0
}

func runStewardHookAttempt(args []string) int {
	flags := flag.NewFlagSet("steward hook-attempt", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	pid := flags.Int64("pid", 0, "hook process pid")
	turnKey := flags.String("turn-key", "", "current turn key")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *pid < 1 || *turnKey == "" {
		fmt.Fprintln(os.Stderr, "steward hook-attempt: --repo, --pid, and --turn-key are required")
		return 2
	}
	line, err := beginHookAttempt(*repo, *pid, *turnKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "steward hook-attempt:", err)
		return 1
	}
	fmt.Println(line)
	return 0
}

// beginHookAttempt records a Stop attempt for the exact hook process and
// returns its generation and attempt sequence as one JSON line.
func beginHookAttempt(repo string, pid int64, turnKey string) (string, error) {
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		return "", fmt.Errorf("the hook process identity is unavailable")
	}
	record, err := steward.BeginHookAttempt(repo, exact.Ref(), turnKey, time.Now())
	if err != nil {
		return "", err
	}
	data, _ := json.Marshal(map[string]any{"generation": record.Generation, "attemptSeq": record.AttemptSeq})
	return string(data), nil
}

func runStewardHookComplete(args []string) int {
	flags := flag.NewFlagSet("steward hook-complete", flag.ContinueOnError)
	var request hooks.HookCompletion
	pathFlagVar(flags, &request.Repo, "repo", "", "checkout root")
	generation := flags.Int("generation", 0, "hook turn generation")
	attempt := flags.Int64("attempt", 0, "hook attempt sequence")
	flags.StringVar(&request.Result, "result", "", "OK | ERROR | INDETERMINATE")
	flags.StringVar(&request.Outcome, "outcome", "", "completion outcome")
	flags.StringVar(&request.HealthLine, "health-line", "", "health verdict carried by the payload")
	flags.StringVar(&request.PayloadFile, "payload-file", "", "file containing the emitted payload")
	flags.StringVar(&request.ReportPath, "report-path", "", "exact immutable Stop report path")
	flags.StringVar(&request.ReportID, "report-id", "", "exact immutable Stop report id")
	flags.StringVar(&request.ReportAlias, "report-alias", "", "exact short Stop report alias used by the payload")
	flags.StringVar(&request.ReportSHA256, "report-sha256", "", "sha256 of the immutable Stop report")
	flags.StringVar(&request.Installation, "installation", "", "installation bound to the report lookup")
	flags.Int64Var(&request.ElapsedSec, "elapsed-sec", 0, "whole seconds elapsed since the Stop deadline parent started")
	if flags.Parse(args) != nil {
		return 2
	}
	flags.Visit(func(parsed *flag.Flag) {
		if parsed.Name == "elapsed-sec" {
			request.HasElapsed = true
		}
	})
	request.Generation = strconv.Itoa(*generation)
	request.Attempt = strconv.FormatInt(*attempt, 10)
	return completeHookAttempt(request, os.Stderr)
}

// completeHookAttempt records one Stop attempt's completion: the payload it
// emitted, the health line it carried and the report it delivered.
func completeHookAttempt(request hooks.HookCompletion, stderr io.Writer) int {
	if request.HasElapsed && request.ElapsedSec < 0 {
		fmt.Fprintln(stderr, "steward hook-complete: --elapsed-sec must be non-negative")
		return 2
	}
	generation, generationErr := strconv.Atoi(request.Generation)
	attempt, attemptErr := strconv.ParseInt(request.Attempt, 10, 64)
	if request.Repo == "" || generationErr != nil || generation < 1 || attemptErr != nil || attempt < 1 || request.Result == "" || request.Outcome == "" {
		fmt.Fprintln(stderr, "steward hook-complete: repo and exact completion flags are required")
		return 2
	}
	payload := []byte{}
	if request.PayloadFile != "" {
		var err error
		payload, err = os.ReadFile(request.PayloadFile)
		if err != nil {
			fmt.Fprintf(stderr, "steward hook-complete: %v\n", err)
			return 1
		}
	}
	if request.Result == string(steward.ComponentOK) && (request.HealthLine == "" || request.PayloadFile == "") {
		fmt.Fprintln(stderr, "steward hook-complete: OK requires health and payload flags")
		return 2
	}
	var stopElapsedSec *int64
	if request.HasElapsed {
		elapsed := request.ElapsedSec
		stopElapsedSec = &elapsed
	}
	_, err := steward.CompleteHookAttemptWithDelivery(request.Repo, generation, attempt, steward.ComponentResult(request.Result),
		request.Outcome, request.HealthLine, string(payload), steward.HookDeliveryReference{Installation: request.Installation, ID: request.ReportID, Alias: request.ReportAlias, Path: request.ReportPath, SHA256: request.ReportSHA256}, stopElapsedSec, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "steward hook-complete: %v\n", err)
		return 1
	}
	return 0
}

// runStewardTick is one scheduled observation: decide, persist the
// aging, and print the decision as JSON for the tick script.
func runStewardTick(args []string) int {
	flags := flag.NewFlagSet("steward tick", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	staleTicks := flags.Int("stale-ticks", 0, "live-idle noise threshold in ticks (default 5)")
	maxRevivals := flags.Int("max-revivals", 0, "dry revivals before notify-only (default 3)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward tick: --repo is required")
		return 2
	}
	tickConfig, err := stewardFixtureTickConfig(*repo, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "steward tick: fixture clock:", err)
		return 2
	}
	tickConfig.StaleTicks, tickConfig.MaxRevivals = *staleTicks, *maxRevivals
	tickConfig.BreachStop = delegateBreachStop(*repo)
	result, err := steward.RunTick(*repo, tickConfig, stewardCensusFor(*repo))
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward tick: %v\n", err)
		if _, deliverErr := steward.DeliverPending(*repo); deliverErr != nil {
			fmt.Fprintf(os.Stderr, "steward tick: notifications pending: %v\n", deliverErr)
		}
		return 1
	}
	// The tick is a functional seam: an external ticker gets the runner's
	// whole pass. Recovery precedes notification, so a condition the machinery
	// heals never reaches the operator as an alert.
	revived := false
	resume := result.Decision.Action == steward.ActRevive
	if !resume {
		// A prepared intent that never launched resumes here too; the
		// external-ticker seam must not strand what the resident runner
		// would have completed.
		if _, ok, resumeErr := steward.ResumableIntent(*repo); resumeErr == nil && ok {
			resume = true
		}
	}
	if resume {
		if out, err := stewardReviveOwner(*repo); err != nil {
			fmt.Fprintf(os.Stderr, "steward tick: revive: %v (%s)\n", err, strings.TrimSpace(string(out)))
			// Durable, not just printed: a failed revival must reach
			// the operator even when nobody reads this output.
			if qErr := steward.QueueNotification(*repo, steward.PendingNotification{
				Nonce:   "revive-failure",
				Message: "steward: revival failed — " + strings.TrimSpace(string(out)),
			}); qErr != nil {
				fmt.Fprintf(os.Stderr, "steward tick: revive-failure incident could not queue: %v\n", qErr)
			}
		} else {
			revived = strings.Contains(string(out), "launched=true")
		}
	}
	delivered, deliverErr := steward.DeliverPending(*repo)
	channelContext, cancelChannel := context.WithTimeout(context.Background(), 15*time.Second)
	channelUndelivered, channelErr := channelphase.Run(channelContext, *repo)
	cancelChannel()
	if channelErr != nil {
		fmt.Fprintf(os.Stderr, "steward tick: channel pending: %d undelivered: %v\n", channelUndelivered, channelErr)
	}
	report := map[string]any{
		"verdict":            result.Decision.Verdict,
		"action":             result.Decision.Action,
		"reason":             result.Decision.Reason,
		"openWork":           result.OpenWork,
		"evidence":           result.Evidence,
		"health":             result.Health,
		"reaped":             result.Reaped,
		"goalStops":          result.GoalStops,
		"delivered":          delivered,
		"channelUndelivered": channelUndelivered,
		"revived":            revived,
	}
	if deliverErr != nil {
		report["deliveryProblem"] = deliverErr.Error()
	}
	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))
	return 0
}

// runStewardAuthorizeDispatch is the dispatcher's gate for the
// unattended continuation: the caller must classify STEWARD, the
// consumed intent must exist unstamped, and the staged tuple is
// printed for dispatch to USE — nothing in this mode is
// caller-selectable.
func runStewardAuthorizeDispatch(args []string) int {
	flags := flag.NewFlagSet("steward authorize-dispatch", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	callerPid := flags.Int64("caller-pid", 0, "the dispatching process")
	nonce := flags.String("intent", "", "the consumed intent's nonce")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *callerPid == 0 || *nonce == "" {
		fmt.Fprintln(os.Stderr, "steward authorize-dispatch: --repo, --caller-pid, and --intent are required")
		return 2
	}
	classification, err := lease.Classify(*repo, *callerPid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: %v\n", err)
		return 1
	}
	if classification.Class != lease.ClassSteward {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: caller is %s, not the steward; the continuation mode admits exactly one caller\n", classification.Class)
		return 1
	}
	authorization, err := steward.AuthorizeDispatch(*repo, *nonce)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: %v\n", err)
		return 1
	}
	out, _ := json.MarshalIndent(map[string]any{
		"goal": authorization.Goal, "jobId": authorization.JobId, "runtime": authorization.Runtime, "model": authorization.Model,
		"role": authorization.Role, "permissions": authorization.Permissions, "brief": authorization.Brief,
	}, "", "  ")
	fmt.Println(string(out))
	return 0
}

// runStewardRevive drives one revival end to end: stage the exact
// launch bytes, mint the intent under the lock, then complete through the
// critical section with the real dispatcher as the launch. Recovery heals
// before alerting; a consumed intent refuses on replay.
func runStewardRevive(args []string) int {
	flags := flag.NewFlagSet("steward revive", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward revive: --repo is required")
		return 2
	}
	return stewardRevive(*repo, os.Stdout, os.Stderr)
}

// stewardReviveOwner is one revival called in the caller's process (the
// steward runner or tick), with its output returned as the former child's
// combined output was.
func stewardReviveOwner(repo string) (string, error) {
	var output strings.Builder
	if status := stewardRevive(cleanOwnerRoot(repo), &output, &output); status != 0 {
		return output.String(), fmt.Errorf("steward revive exited %d", status)
	}
	return output.String(), nil
}

// stewardRevive is the revival owner on the caller's streams. The revival's
// dispatch stays its own process in its own session (delegate --revive), so
// the delegate chain classifies STEWARD from the steward process that calls
// this, as it did from the revive child.
func stewardRevive(repo string, stdout, stderr io.Writer) int {

	// Resume an already-minted intent before minting another: the
	// one-active-continuation guard would otherwise refuse forever.
	live, err := steward.LiveIntents(repo)
	if err != nil {
		fmt.Fprintf(stderr, "steward revive: %v\n", err)
		return 1
	}
	var nonce string
	if len(live) > 0 {
		nonce = live[0].Nonce
	} else {
		work, reason, err := steward.LegacyOpenWork(repo)
		if err != nil || work != steward.WorkOwned {
			fmt.Fprintf(stderr, "steward revive: no owned open work (%s)\n", reason)
			return 1
		}
		goalName := strings.TrimPrefix(reason, "current goal: ")
		roster, err := dispatchpkg.ResolveRoster(dispatchpkg.RosterParams{
			ConfPath: filepath.Join(repo, "metasystem.conf"),
			Role:     "steward-continuation", Mode: "build",
		})
		if err != nil {
			fmt.Fprintf(stderr, "steward revive: roster: %v\n", err)
			return 1
		}
		raw := make([]byte, 8)
		if _, err := rand.Read(raw); err != nil {
			fmt.Fprintf(stderr, "steward revive: %v\n", err)
			return 1
		}
		nonce = hex.EncodeToString(raw)
		it, err := steward.StageIntent(repo, nonce, goalName, "steward-"+nonce,
			roster.Runtime, roster.Model, "worker provably dead with open work")
		if err != nil {
			fmt.Fprintf(stderr, "steward revive: %v\n", err)
			return 1
		}
		receiptRoot, rootErr := stateroot.StateRoot(stateroot.Receipts)
		if rootErr != nil {
			fmt.Fprintf(stderr, "steward revive: %v\n", rootErr)
			return 1
		}
		if err := steward.PrepareIntent(repo, filepath.Join(receiptRoot, "receipts.log"), it); err != nil {
			fmt.Fprintf(stderr, "steward revive: %v\n", err)
			return 1
		}
	}

	outcome, err := steward.CompleteRevival(repo, steward.TickConfig{}, stewardCensusFor(repo), nonce,
		func(it steward.Intent) error {
			binary, binaryErr := os.Executable()
			if binaryErr != nil {
				return binaryErr
			}
			cmd := exec.Command(binary, "internal", "delegate", "--revive", it.Nonce)
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), "METASYSTEM_DELEGATE_ROOT="+repo)
			// Its own session, no controlling terminal: the chain
			// classifies STEWARD identically whether the tick came
			// from the runner, a cron, or an operator's shell.
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("delegate revival: %v (%s)", err, strings.TrimSpace(string(out)))
			}
			return nil
		})
	if err != nil {
		fmt.Fprintf(stderr, "steward revive: %v\n", err)
		return 1
	}
	// A concurrent runner may consume, launch, and stamp this exact intent
	// before this caller enters the critical section. Report that completed
	// handoff as the successful launch it was; an unstamped consumption still
	// remains an unknown outcome and is not promoted.
	if !outcome.Launched && strings.Contains(outcome.Reason, "intent is not live") {
		if consumed, consumedErr := steward.ConsumedIntent(repo, nonce); consumedErr == nil && consumed.LaunchStamped {
			outcome.Launched = true
			outcome.Reason = "intent was launched by a concurrent reviver"
		}
	}
	fmt.Fprintf(stdout, "launched=%v reason=%s\n", outcome.Launched, outcome.Reason)
	if !outcome.Launched {
		if outcome.Escalate {
			return 3
		}
		return 0
	}
	return 0
}

// runStewardRun is the runner's body — normally spawned by arm,
// callable directly by any external ticker the operator provides.
func runStewardRun(args []string) int {
	flags := flag.NewFlagSet("steward run", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	// The arming caller's handoff. The runner keeps the value in memory and
	// reports it on this machine's presence record; nothing persists it.
	lineage := flags.String("lineage", "", "the session lineage this runner was armed under (\"no-lease\" when there was none)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward run: --repo is required")
		return 2
	}
	if os.Getenv("METASYSTEM_STEWARD_RUNNER_IGNORE_TERM") != "" {
		if !fixtureauth.FixtureModeRoot(*repo) {
			fmt.Fprintln(os.Stderr, "steward run: METASYSTEM_STEWARD_RUNNER_IGNORE_TERM is fixture-only")
			return 2
		}
		signal.Ignore(syscall.SIGTERM)
	}
	tickConfig, clockErr := stewardFixtureTickConfig(*repo, stewardRunClockRoot(*repo))
	if clockErr != nil {
		fmt.Fprintln(os.Stderr, "steward run: fixture clock:", clockErr)
		return 2
	}
	tickConfig.ArmedLineage = *lineage
	tickConfig.BreachStop = delegateBreachStop(*repo)
	tickConfig.BreachStopReady = stewardRunnerCustodianReady(*repo, productionStewardCustodianFacts())
	interval := time.Duration(steward.TickSeconds(*repo)) * time.Second
	err := steward.RunLoop(*repo, stewardCensusFor(*repo), func() error {
		out, err := stewardReviveOwner(*repo)
		if err != nil {
			return fmt.Errorf("%v (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}, interval, tickConfig)
	if err != nil {
		var stopped *steward.StoppedError
		if errors.As(err, &stopped) {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "steward run: %v\n", err)
		return 1
	}
	return 0
}

func runStewardArm(args []string) int {
	flags := flag.NewFlagSet("steward arm", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	temporaryWord := flags.String("temporary-human-word", "", "verbatim remote human authorization; enrolls TEMPORARILY with the word recorded on the identity until a terminal re-arm")
	reviewBy := flags.String("review-by", "", "the human's own re-approval date (required with --temporary-human-word)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward arm: --repo is required")
		return 2
	}
	if refused, err := refuseStewardIfStopped(*repo); err != nil {
		fmt.Fprintln(os.Stderr, "steward arm:", err)
		return 1
	} else if refused {
		return 1
	}
	if err := humanauthority.ValidateTemporaryWordPair(*temporaryWord, *reviewBy); err != nil {
		fmt.Fprintln(os.Stderr, "steward arm:", err)
		return 2
	}
	fixtureEnrollment := false
	if *temporaryWord == "" {
		var authorized bool
		fixtureEnrollment, authorized = requireHumanTerminal(*repo, "steward arm")
		if !authorized {
			return 1
		}
	} else {
		// The agent-free-terminal law stands; this is its one recorded
		// exception: the human is away from the machine and authorized a
		// temporary enrollment in their own words, which ride the
		// identity record until they re-arm at a terminal. Loud by
		// construction — the word and the review date are durable.
		fmt.Fprintf(os.Stderr, "steward arm: TEMPORARY enrollment under a recorded remote human word; re-approval due %s at an agent-free terminal\n", *reviewBy)
	}
	if seed, err := seedStewardLandingRef(*repo); err != nil {
		fmt.Fprintf(os.Stderr, "steward arm: %v\n", err)
		return 1
	} else if seed.Ref != "" {
		fmt.Fprintf(os.Stderr, "steward arm: seeded metasystem.steward.landing-ref=%s from the checked-out branch's upstream\n", seed.Ref)
	} else if seed.NotSeeded != "" {
		fmt.Fprintf(os.Stderr, "steward arm: landing ref was not seeded: %s\n", seed.NotSeeded)
	}
	bin, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward arm: %v\n", err)
		return 1
	}
	var msg string
	lineage := armingLineage(*repo)
	if *temporaryWord != "" {
		msg, err = steward.ArmTemporaryWithLineage(*repo, bin, *temporaryWord, *reviewBy, lineage)
	} else if fixtureEnrollment {
		msg, err = steward.ArmFixtureWithLineage(*repo, bin, lineage)
	} else {
		msg, err = steward.ArmWithLineage(*repo, bin, lineage)
	}
	if err != nil {
		var stopped *steward.StoppedError
		if errors.As(err, &stopped) {
			printStewardStopped(stopped.Checkout, stopped.Record)
			return 1
		}
		fmt.Fprintf(os.Stderr, "steward arm: %v\n", err)
		return 1
	}
	fmt.Println(msg)
	return 0
}

func runStewardRestart(args []string) int {
	flags := flag.NewFlagSet("steward restart", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward restart: --repo is required")
		return 2
	}
	if refused, err := refuseStewardIfStopped(*repo); err != nil {
		fmt.Fprintln(os.Stderr, "steward restart:", err)
		return 1
	} else if refused {
		return 1
	}
	fixtureEnrollment, authorized := requireHumanTerminal(*repo, "steward restart")
	if !authorized {
		return 1
	}
	if seed, err := seedStewardLandingRef(*repo); err != nil {
		fmt.Fprintf(os.Stderr, "steward restart: %v\n", err)
		return 1
	} else if seed.Ref != "" {
		fmt.Fprintf(os.Stderr, "steward restart: seeded metasystem.steward.landing-ref=%s from the checked-out branch's upstream\n", seed.Ref)
	} else if seed.NotSeeded != "" {
		fmt.Fprintf(os.Stderr, "steward restart: landing ref was not seeded: %s\n", seed.NotSeeded)
	}
	bin, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward restart: %v\n", err)
		return 1
	}
	var msg string
	if fixtureEnrollment {
		msg, err = steward.RestartFixture(*repo, bin)
	} else {
		msg, err = steward.Restart(*repo, bin)
	}
	if err != nil {
		var stopped *steward.StoppedError
		if errors.As(err, &stopped) {
			printStewardStopped(stopped.Checkout, stopped.Record)
			return 1
		}
		fmt.Fprintf(os.Stderr, "steward restart: %v\n", err)
		return 1
	}
	fmt.Println(msg)
	return 0
}

type stewardLandingRefSeed struct {
	Ref       string
	NotSeeded string
}

type stewardLandingRefGit interface {
	Output(repo string, args ...string) ([]byte, error)
	CombinedOutput(repo string, args ...string) ([]byte, error)
}

type realStewardLandingRefGit struct{}

func (realStewardLandingRefGit) Output(repo string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
}

func (realStewardLandingRefGit) CombinedOutput(repo string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
}

func seedStewardLandingRef(repo string) (stewardLandingRefSeed, error) {
	return seedStewardLandingRefWithGit(repo, realStewardLandingRefGit{})
}

func seedStewardLandingRefWithGit(repo string, git stewardLandingRefGit) (stewardLandingRefSeed, error) {
	if out, err := git.Output(repo, "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref"); err == nil && strings.TrimSpace(string(out)) != "" {
		return stewardLandingRefSeed{}, nil
	}
	out, err := git.Output(repo, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return stewardLandingRefSeed{NotSeeded: "the checkout is detached; automatic machine re-arm remains disabled until the key is configured"}, nil
	}
	branch := strings.TrimSpace(string(out))
	out, err = git.CombinedOutput(repo, "rev-parse", "--symbolic-full-name", "@{upstream}")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return stewardLandingRefSeed{NotSeeded: fmt.Sprintf("the checked-out branch %s has no upstream; automatic machine re-arm remains disabled until the key is configured", branch)}, nil
	}
	landingRef := strings.TrimSpace(string(out))
	tail := strings.TrimPrefix(landingRef, "refs/remotes/")
	remote, upstreamBranch, qualified := strings.Cut(tail, "/")
	if tail == landingRef || !qualified || remote == "" || upstreamBranch == "" {
		return stewardLandingRefSeed{NotSeeded: fmt.Sprintf("the checked-out branch %s has upstream %s, not a remote-tracking ref shaped refs/remotes/<remote>/<branch>; automatic machine re-arm remains disabled until the key is configured", branch, landingRef)}, nil
	}
	if out, err := git.CombinedOutput(repo, "config", "--local", "metasystem.steward.landing-ref", landingRef); err != nil {
		return stewardLandingRefSeed{}, fmt.Errorf("seed metasystem.steward.landing-ref: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return stewardLandingRefSeed{Ref: landingRef}, nil
}

func refuseStewardIfStopped(repo string) (bool, error) {
	top, err := canonicalPath(repo)
	if err != nil {
		return false, err
	}
	closed, record, err := stopfence.Closed(top)
	if err != nil {
		return false, fmt.Errorf("read process-creation fence: %w", err)
	}
	if !closed {
		return false, nil
	}
	printStewardStopped(top, record)
	return true, nil
}

func printStewardStopped(checkout string, record stopfence.Record) {
	description, descriptionErr := stopfence.ClosedDescription(record, checkout)
	command, commandErr := stopfence.ClosedCommand(record, checkout)
	if descriptionErr != nil || commandErr != nil {
		fmt.Fprintln(os.Stderr, "steward: cannot render stopped refusal:", errors.Join(descriptionErr, commandErr))
		return
	}
	fmt.Fprintln(os.Stderr, description)
	fmt.Fprintln(os.Stderr, "run: "+command)
}

func runStewardDisarm(args []string) int {
	flags := flag.NewFlagSet("steward disarm", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward disarm: --repo is required")
		return 2
	}
	outcome, err := steward.Disarm(*repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward disarm: %v\n", err)
		return 1
	}
	fmt.Println(outcome.LongForm())
	return 0
}

// runStewardPending prints one line naming undelivered incidents —
// empty output means none, so shell callers can gate on it.
// runStewardStatus is the operator's view: the last evidence state,
// live intents, and pending notifications — the second visibility
// channel the design pins.
func runStewardStatus(args []string) int {
	flags := flag.NewFlagSet("steward status", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward status: --repo is required")
		return 2
	}
	evidence, evErr := steward.LoadEvidence(steward.EvidencePath(*repo))
	intents, intErr := steward.LiveIntents(*repo)
	pending, pendErr := steward.PendingNotifications(*repo)
	alerts, alertErr := steward.AlertEpisodes(*repo)
	report := map[string]any{"evidence": evidence, "liveIntents": intents, "pendingNotifications": pending, "alertEpisodes": alerts}
	var problems []string
	for _, err := range []error{evErr, intErr, pendErr, alertErr} {
		if err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		report["problems"] = problems
	}
	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))
	if len(problems) > 0 {
		return 1
	}
	return 0
}

// armingLineage reads the lineage of the session that holds this checkout,
// which the arming caller hands to the runner it starts. A checkout with no
// lease is the literal no-lease: a fresh machine armed before its first
// session is an ordinary state, not a fault.
func armingLineage(repo string) string {
	holder, err := lease.CurrentHolder(repo)
	if err != nil || holder.OwnerLineage == "" {
		return seat.NoLease
	}
	return holder.OwnerLineage
}
