package main

// The steward family: the idle watchdog's decision surface. check
// reads and reports without side effects; tick folds one observation
// into the evidence and reports what the schedule glue should do;
// status shows the operator everything pending. The revive action
// itself lands with the dispatch continuation mode.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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

	dispatchpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
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
func runStewardRun(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("steward run", stdout, stderr)
	repo := pathFlag(flags, "repo", "", "checkout root")
	// The arming caller's handoff. The runner keeps the value in memory and
	// reports it on this machine's presence record; nothing persists it.
	lineage := flags.String("lineage", "", "the session lineage this runner was armed under (\"no-lease\" when there was none)")
	if flags.Parse(args) != nil || !requireFlags(flags, stderr, "repo") {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(stderr, "steward run: --repo is required")
		return 2
	}
	if os.Getenv("METASYSTEM_STEWARD_RUNNER_IGNORE_TERM") != "" {
		if !fixtureauth.FixtureModeRoot(*repo) {
			fmt.Fprintf(stderr, "the steward ignores the stop signal only in a test bed, and this is not one; unset %s\n", "METASYSTEM_STEWARD_RUNNER_IGNORE_TERM")
			return 2
		}
		signal.Ignore(syscall.SIGTERM)
	}
	tickConfig, clockErr := stewardFixtureTickConfig(*repo, stewardRunClockRoot(*repo))
	if clockErr != nil {
		fmt.Fprintln(stderr, "steward run: fixture clock:", clockErr)
		return 2
	}
	tickConfig.ArmedLineage = *lineage
	tickConfig.BreachStop = delegateBreachStop(*repo)
	tickConfig.BreachStopReady = stewardRunnerCustodianReady(*repo, productionStewardCustodianFacts())
	// The steward keeps the host landing lane's owner alive (U12).
	tickConfig.KeepLandingLane = batchowner.LandingLaneKeeper(batchowner.LandingLaneHome)
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
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stderr, "steward run: %v\n", err)
		return 1
	}
	return 0
}

// stewardArmDeps are the arm verb's Git-facing and minting dependencies,
// passed per call so a test hands in its own instances.
type stewardArmDeps struct {
	repositoryTop func(string) (string, error)
	landingRefGit stewardLandingRefGit
	armSession    func(repoRoot, binaryPath string, session steward.EnrolledSession, lineage string) (string, error)
}

func runStewardArm(args []string, stdout, stderr io.Writer) int {
	return runStewardArmWith(args, stewardArmDeps{
		repositoryTop: stateroot.RepositoryTop,
		landingRefGit: realStewardLandingRefGit{},
		armSession:    steward.ArmSessionWithLineage,
	}, stdout, stderr)
}

func runStewardArmWith(args []string, deps stewardArmDeps, stdout, stderr io.Writer) int {
	flags := newFlagSet("steward arm", stdout, stderr)
	repo := pathFlag(flags, "repo", "", "checkout root")
	temporaryWord := flags.String("temporary-human-word", "", "verbatim remote human authorization; enrolls TEMPORARILY with the word recorded on the identity until a terminal re-arm")
	reviewBy := flags.String("review-by", "", "the human's own re-approval date (required with --temporary-human-word)")
	launchRecord := pathFlag(flags, "launch-record", "", "a launch record the interface stamped with a signed-in browser session's enrollment; enrolls as that human (g1-s72)")
	if flags.Parse(args) != nil || !requireFlags(flags, stderr, "repo") {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(stderr, "steward arm: --repo is required")
		return 2
	}
	if *launchRecord != "" && (*temporaryWord != "" || *reviewBy != "") {
		fmt.Fprintln(stderr, "--launch-record carries its person's approval, so it takes no --temporary-human-word or --review-by")
		return 2
	}
	if refused, err := refuseStewardIfStopped(*repo, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "steward arm:", err)
		return 1
	} else if refused {
		return 1
	}
	if err := humanauthority.ValidateTemporaryWordPair(*temporaryWord, *reviewBy); err != nil {
		fmt.Fprintln(stderr, "steward arm:", err)
		return 2
	}
	fixtureEnrollment := false
	var session steward.EnrolledSession
	if *launchRecord != "" {
		var err error
		if session, err = sessionEnrollmentFromRecord(*repo, *launchRecord, deps.repositoryTop); err != nil {
			fmt.Fprintln(stderr, "steward arm:", err)
			return 1
		}
		metasystemRoot, err := upMetasystemRoot("")
		if err != nil {
			fmt.Fprintf(stderr, "steward arm: cannot resolve the installed engine: %v\n", err)
			return 1
		}
		if err := sessionCallerCheck(*repo, metasystemRoot, personClassifyAt); err != nil {
			fmt.Fprintln(stderr, "steward arm:", err)
			return 1
		}
	} else if *temporaryWord == "" {
		var authorized bool
		fixtureEnrollment, authorized = requireHumanTerminal(stderr, *repo, "steward arm")
		if !authorized {
			return 1
		}
	} else {
		// The agent-free-terminal law stands; this is its one recorded
		// exception: the human is away from the machine and authorized a
		// temporary enrollment in their own words, which ride the
		// identity record until they re-arm at a terminal. Loud by
		// construction — the word and the review date are durable.
		fmt.Fprintf(stderr, "the steward runs on a temporary approval given remotely; a person approves it again by %s\n", *reviewBy)
	}
	if seed, err := seedStewardLandingRefWithGit(*repo, deps.landingRefGit); err != nil {
		fmt.Fprintf(stderr, "steward arm: %v\n", err)
		return 1
	} else if seed.Ref != "" {
		fmt.Fprintf(stderr, "steward arm: seeded metasystem.steward.landing-ref=%s from the checked-out branch's upstream\n", seed.Ref)
	} else if seed.NotSeeded != "" {
		fmt.Fprintf(stderr, "steward arm: landing ref was not seeded: %s\n", seed.NotSeeded)
	}
	bin, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "steward arm: %v\n", err)
		return 1
	}
	var msg string
	lineage := armingLineage(*repo)
	if *launchRecord != "" {
		msg, err = deps.armSession(*repo, bin, session, lineage)
	} else if *temporaryWord != "" {
		msg, err = steward.ArmTemporaryWithLineage(*repo, bin, *temporaryWord, *reviewBy, lineage)
	} else if fixtureEnrollment {
		msg, err = steward.ArmFixtureWithLineage(*repo, bin, lineage)
	} else {
		msg, err = steward.ArmWithLineage(*repo, bin, lineage)
	}
	if err != nil {
		var stopped *steward.StoppedError
		if errors.As(err, &stopped) {
			printStewardStopped(stopped.Checkout, stopped.Record, stdout, stderr)
			return 1
		}
		fmt.Fprintf(stderr, "steward arm: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, msg)
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

func refuseStewardIfStopped(repo string, stdout, stderr io.Writer) (bool, error) {
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
	printStewardStopped(top, record, stdout, stderr)
	return true, nil
}

func printStewardStopped(checkout string, record stopfence.Record, stdout, stderr io.Writer) {
	description, descriptionErr := stopfence.ClosedDescription(record, checkout)
	command, commandErr := stopfence.ClosedCommand(record, checkout)
	if descriptionErr != nil || commandErr != nil {
		fmt.Fprintln(stderr, "steward: cannot render stopped refusal:", errors.Join(descriptionErr, commandErr))
		return
	}
	fmt.Fprintln(stderr, description)
	fmt.Fprintln(stderr, "run: "+command)
}

// runStewardPending prints one line naming undelivered incidents —
// empty output means none, so shell callers can gate on it.
// runStewardStatus is the operator's view: the last evidence state,
// live intents, and pending notifications — the second visibility
// channel the design pins.
func runStewardStatus(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("steward status", stdout, stderr)
	repo := pathFlag(flags, "repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(stderr, "steward status: --repo is required")
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
	fmt.Fprintln(stdout, string(out))
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

// sessionEnrollmentFromRecord reads the verdict a signed-in browser launch
// left on its record and binds it to the clone being armed (g1-s72 D2).
//
// Threat model (S72-01): a same-user adversary is out of scope repo-wide
// (internal/steward/identity.go). The record's ownership, owner-only mode and
// destination binding defend against accident and against one record arming
// a second clone; the caller classification below refuses an agent runtime,
// a delegate, the steward and supervision by name. The residual, stated: a
// same-user process that has escaped its ancestry, or another checkout's
// steward (recognition is target-repository local), could forge a 0600
// record and mint a human-session identity; nothing here proves the record
// was the interface's. The temporary word pair has the same exposure.
func sessionEnrollmentFromRecord(repo, recordPath string, repositoryTop func(string) (string, error)) (steward.EnrolledSession, error) {
	path, err := canonicalPath(recordPath)
	if err != nil {
		return steward.EnrolledSession{}, fmt.Errorf("launch record %s: %w", recordPath, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return steward.EnrolledSession{}, fmt.Errorf("launch record absent: %w", err)
	}
	if !info.Mode().IsRegular() {
		return steward.EnrolledSession{}, fmt.Errorf("launch record %s is not a regular file", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return steward.EnrolledSession{}, fmt.Errorf("launch record %s is not owner-only (mode %o)", path, info.Mode().Perm())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return steward.EnrolledSession{}, fmt.Errorf("launch record %s is owned by uid %d, not the calling user", path, st.Uid)
	}
	// The launching checkout is where the record lives: the interface writes
	// it under <checkout>/artifacts/agents/ui/launches/<launch>.json.
	from := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(path)))))
	id, isJSON := strings.CutSuffix(filepath.Base(path), ".json")
	if !isJSON || !launch.ValidLaunchID(id) || launch.Dir(from) != filepath.Dir(path) {
		return steward.EnrolledSession{}, fmt.Errorf("launch record %s is not a launch record under a checkout's artifacts/agents/ui/launches", path)
	}
	record, err := launch.LoadAt(path)
	if err != nil {
		return steward.EnrolledSession{}, err
	}
	if record.Launch != id {
		return steward.EnrolledSession{}, fmt.Errorf("launch record %s names launch %s, not the %s its file is named for", path, record.Launch, id)
	}
	enrollment := record.Enrollment
	if enrollment == nil {
		return steward.EnrolledSession{}, fmt.Errorf("launch record %s carries no signed-in session enrollment", path)
	}
	if enrollment.Kind != launch.EnrollmentHumanSession {
		return steward.EnrolledSession{}, fmt.Errorf("launch record %s carries enrollment kind %q, not %q", path, enrollment.Kind, launch.EnrollmentHumanSession)
	}
	for _, field := range []struct{ name, value string }{
		{"provider", enrollment.Provider}, {"human", enrollment.Human}, {"session", enrollment.Session},
	} {
		if strings.TrimSpace(field.value) == "" {
			return steward.EnrolledSession{}, fmt.Errorf("launch record %s: its enrollment has no %s", path, field.name)
		}
	}
	top, err := upRepositoryScopeWith(repo, repositoryTop)
	if err != nil {
		return steward.EnrolledSession{}, err
	}
	destination, err := canonicalPath(record.Destination)
	if err != nil || record.Destination == "" || destination != top {
		return steward.EnrolledSession{}, fmt.Errorf("launch record %s names destination %s, not this repository %s", path, record.Destination, top)
	}
	return steward.EnrolledSession{
		Provider: enrollment.Provider, Human: enrollment.Human, Reference: enrollment.Session,
		Launch: record.Launch, From: from,
	}, nil
}

// sessionCallerCheck classifies the arm's caller as the terminal path does,
// with the same injectable classifier. A caller with no recognised ancestor
// passes as well as a human's: the browser's detached launch has no terminal
// and the record is what vouches. Every recognised actor is refused by name.
func sessionCallerCheck(repo, metasystemRoot string, classify processCallerClassifier) error {
	classification, err := classify(repo, metasystemRoot, int64(os.Getppid()))
	if err != nil {
		return fmt.Errorf("who is launching this session cannot be told: %w", err)
	}
	switch classification.Class {
	case lease.ClassHuman, lease.ClassUntrusted:
		return nil
	}
	return errors.New("a session is enrolled only from a person's own launch, and this is not one")
}
