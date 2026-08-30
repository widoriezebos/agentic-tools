package main

// The steward family: the idle watchdog's decision surface. check
// reads and reports without side effects; tick folds one observation
// into the evidence and reports what the schedule glue should do;
// status shows the operator everything pending. The revive action
// itself lands with the dispatch continuation mode.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	dispatchpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func stewardCensusFor(repo string) steward.WorkerCensus {
	return steward.RuntimeWorkerCensus{
		MetasystemRoot: repo,
		ProcessFile:    os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE"),
	}
}

// runStewardHealth prints every role on one line and returns the aggregate
// health code: zero healthy, one when any role is dead, two when unknown is
// the worst result.
func runStewardHealth(args []string) int {
	flags := flag.NewFlagSet("health", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	metasystemRoot := flags.String("metasystem-root", "", "installed metasystem root (defaults to checkout root)")
	hookPreview := flags.Bool("hook-preview", false, "render current hook facts without advancing the tick-owned alert breaker (internal)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "health: --repo is required")
		return 2
	}
	if *metasystemRoot == "" {
		*metasystemRoot = *repo
	}
	if *hookPreview {
		verdict := steward.PreviewHealthAt(*repo, *metasystemRoot, time.Now(), nil)
		fmt.Println(verdict.Line())
		return verdict.ExitCode()
	}
	verdict, err := steward.ObserveHealth(*repo, time.Now(), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "health: health evidence is unknown: %v\n", err)
		return 2
	}
	alertView := verdict
	alertView.ShouldAlert = false
	if _, err := steward.UpdateAlertEpisodes(*repo, alertView, verdict.Line(), time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "health: alert episode state is unknown: %v\n", err)
		return 2
	}
	fmt.Println(verdict.Line())
	return verdict.ExitCode()
}

func runHealthAcknowledgeAlert(args []string) int {
	flags := flag.NewFlagSet("health acknowledge-alert", flag.ContinueOnError)
	episodeID := flags.String("episode", "", "alert episode id")
	repo := flags.String("repo", ".", "checkout root")
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
	repo := flags.String("repo", "", "checkout root")
	pid := flags.Int64("pid", 0, "hook process pid")
	turnKey := flags.String("turn-key", "", "current turn key")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *pid < 1 || *turnKey == "" {
		fmt.Fprintln(os.Stderr, "steward hook-attempt: --repo, --pid, and --turn-key are required")
		return 2
	}
	exact, state, err := (identity.KernelProber{}).Probe(*pid)
	if err != nil || state != identity.Alive {
		fmt.Fprintln(os.Stderr, "steward hook-attempt: the hook process identity is unavailable")
		return 1
	}
	record, err := steward.BeginHookAttempt(*repo, exact.Ref(), *turnKey, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward hook-attempt: %v\n", err)
		return 1
	}
	data, _ := json.Marshal(map[string]any{"generation": record.Generation, "attemptSeq": record.AttemptSeq})
	fmt.Println(string(data))
	return 0
}

func runStewardHookComplete(args []string) int {
	flags := flag.NewFlagSet("steward hook-complete", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	generation := flags.Int("generation", 0, "hook turn generation")
	attempt := flags.Int64("attempt", 0, "hook attempt sequence")
	result := flags.String("result", "", "OK | ERROR | INDETERMINATE")
	outcome := flags.String("outcome", "", "completion outcome")
	healthLine := flags.String("health-line", "", "health verdict carried by the payload")
	payloadFile := flags.String("payload-file", "", "file containing the emitted payload")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *generation < 1 || *attempt < 1 || *result == "" || *outcome == "" {
		fmt.Fprintln(os.Stderr, "steward hook-complete: repo and exact completion flags are required")
		return 2
	}
	payload := []byte{}
	if *payloadFile != "" {
		var err error
		payload, err = os.ReadFile(*payloadFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "steward hook-complete: %v\n", err)
			return 1
		}
	}
	if *result == string(steward.ComponentOK) && (*healthLine == "" || *payloadFile == "") {
		fmt.Fprintln(os.Stderr, "steward hook-complete: OK requires --health-line and --payload-file")
		return 2
	}
	if _, err := steward.CompleteHookAttempt(*repo, *generation, *attempt, steward.ComponentResult(*result),
		*outcome, *healthLine, string(payload), time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "steward hook-complete: %v\n", err)
		return 1
	}
	return 0
}

func runStewardDigestPending(args []string) int {
	flags := flag.NewFlagSet("steward digest-pending", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward digest-pending: --repo is required")
		return 2
	}
	pending, err := narratordigest.Pending(*repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward digest-pending: %v\n", err)
		return 1
	}
	data, _ := json.Marshal(pending)
	fmt.Println(string(data))
	return 0
}

func runStewardDigestAdvance(args []string) int {
	flags := flag.NewFlagSet("steward digest-advance", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	cursor := flags.Int64("cursor", -1, "emitted digest byte cursor")
	prefix := flags.String("prefix-sha256", "", "emitted digest prefix digest")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *cursor < 0 || *prefix == "" {
		fmt.Fprintln(os.Stderr, "steward digest-advance: --repo, --cursor, and --prefix-sha256 are required")
		return 2
	}
	if err := narratordigest.Advance(*repo, *cursor, *prefix); err != nil {
		fmt.Fprintf(os.Stderr, "steward digest-advance: %v\n", err)
		return 1
	}
	return 0
}

// runStewardTick is one scheduled observation: decide, persist the
// aging, and print the decision as JSON for the tick script.
func runStewardTick(args []string) int {
	flags := flag.NewFlagSet("steward tick", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	staleTicks := flags.Int("stale-ticks", 0, "live-idle noise threshold in ticks (default 5)")
	maxRevivals := flags.Int("max-revivals", 0, "dry revivals before notify-only (default 3)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward tick: --repo is required")
		return 2
	}
	result, err := steward.RunTick(*repo, steward.TickConfig{
		StaleTicks: *staleTicks, MaxRevivals: *maxRevivals,
	}, stewardCensusFor(*repo))
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
		cmd := exec.Command(os.Args[0], "steward", "revive", "--repo", *repo)
		if out, err := cmd.CombinedOutput(); err != nil {
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
	report := map[string]any{
		"verdict":   result.Decision.Verdict,
		"action":    result.Decision.Action,
		"reason":    result.Decision.Reason,
		"openWork":  result.OpenWork,
		"evidence":  result.Evidence,
		"health":    result.Health,
		"reaped":    result.Reaped,
		"goalStops": result.GoalStops,
		"delivered": delivered,
		"revived":   revived,
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
	repo := flags.String("repo", "", "checkout root")
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
	it, err := steward.ConsumedIntent(*repo, *nonce)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: %v\n", err)
		return 1
	}
	if it.LaunchStamped {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: intent %s already launched; a replay authorizes nothing\n", *nonce)
		return 1
	}
	if err := steward.VerifyStagedDigests(*repo, it); err != nil {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: %v\n", err)
		return 1
	}
	top, absErr := filepath.Abs(*repo)
	if absErr != nil {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: %v\n", absErr)
		return 1
	}
	installed, idErr := steward.VerifyIdentity(steward.RepoIdentityPath(top), top)
	if idErr != nil {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: %v\n", idErr)
		return 1
	}
	if it.RepoIdentity != installed.RepoIdentity || it.InstallGen != installed.Generation {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: the authorization was minted under installation generation %d of %q; the current installation is generation %d of %q — a superseded authorization launches nothing\n",
			it.InstallGen, it.RepoIdentity, installed.Generation, installed.RepoIdentity)
		return 1
	}
	out, _ := json.MarshalIndent(map[string]any{
		"goal": it.Goal, "jobId": it.JobId, "runtime": it.Runtime, "model": it.Model,
		"role": it.Role, "permissions": it.Permissions, "brief": steward.BriefPath(*repo, it.Nonce),
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
	repo := flags.String("repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward revive: --repo is required")
		return 2
	}

	// Resume an already-minted intent before minting another: the
	// one-active-continuation guard would otherwise refuse forever.
	live, err := steward.LiveIntents(*repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward revive: %v\n", err)
		return 1
	}
	var nonce string
	if len(live) > 0 {
		nonce = live[0].Nonce
	} else {
		work, reason, err := steward.LegacyOpenWork(*repo)
		if err != nil || work != steward.WorkOwned {
			fmt.Fprintf(os.Stderr, "steward revive: no owned open work (%s)\n", reason)
			return 1
		}
		goalName := strings.TrimPrefix(reason, "current goal: ")
		roster, err := dispatchpkg.ResolveRoster(dispatchpkg.RosterParams{
			ConfPath: filepath.Join(*repo, "metasystem.conf"),
			Role:     "steward-continuation", Mode: "build",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "steward revive: roster: %v\n", err)
			return 1
		}
		raw := make([]byte, 8)
		if _, err := rand.Read(raw); err != nil {
			fmt.Fprintf(os.Stderr, "steward revive: %v\n", err)
			return 1
		}
		nonce = hex.EncodeToString(raw)
		it, err := steward.StageIntent(*repo, nonce, goalName, "steward-"+nonce,
			roster.Runtime, roster.Model, "worker provably dead with open work")
		if err != nil {
			fmt.Fprintf(os.Stderr, "steward revive: %v\n", err)
			return 1
		}
		receiptRoot, rootErr := stateroot.StateRoot(stateroot.Receipts)
		if rootErr != nil {
			fmt.Fprintf(os.Stderr, "steward revive: %v\n", rootErr)
			return 1
		}
		if err := steward.PrepareIntent(*repo, filepath.Join(receiptRoot, "receipts.log"), it); err != nil {
			fmt.Fprintf(os.Stderr, "steward revive: %v\n", err)
			return 1
		}
	}

	outcome, err := steward.CompleteRevival(*repo, steward.TickConfig{}, stewardCensusFor(*repo), nonce,
		func(it steward.Intent) error {
			binary, binaryErr := os.Executable()
			if binaryErr != nil {
				return binaryErr
			}
			cmd := exec.Command(binary, "delegate", "--revive", it.Nonce)
			cmd.Dir = *repo
			cmd.Env = append(os.Environ(), "METASYSTEM_DELEGATE_ROOT="+*repo)
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
		fmt.Fprintf(os.Stderr, "steward revive: %v\n", err)
		return 1
	}
	// A concurrent runner may consume, launch, and stamp this exact intent
	// before this caller enters the critical section. Report that completed
	// handoff as the successful launch it was; an unstamped consumption still
	// remains an unknown outcome and is not promoted.
	if !outcome.Launched && strings.Contains(outcome.Reason, "intent is not live") {
		if consumed, consumedErr := steward.ConsumedIntent(*repo, nonce); consumedErr == nil && consumed.LaunchStamped {
			outcome.Launched = true
			outcome.Reason = "intent was launched by a concurrent reviver"
		}
	}
	fmt.Printf("launched=%v reason=%s\n", outcome.Launched, outcome.Reason)
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
	repo := flags.String("repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward run: --repo is required")
		return 2
	}
	interval := time.Duration(steward.TickSeconds(*repo)) * time.Second
	err := steward.RunLoop(*repo, stewardCensusFor(*repo), func() error {
		cmd := exec.Command(os.Args[0], "steward", "revive", "--repo", *repo)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}, interval)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward run: %v\n", err)
		return 1
	}
	return 0
}

func runStewardArm(args []string) int {
	flags := flag.NewFlagSet("steward arm", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	temporaryWord := flags.String("temporary-human-word", "", "verbatim remote human authorization; enrolls TEMPORARILY with the word recorded on the identity until a terminal re-arm")
	reviewBy := flags.String("review-by", "", "the human's own re-approval date (required with --temporary-human-word)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward arm: --repo is required")
		return 2
	}
	if (*temporaryWord == "") != (*reviewBy == "") {
		fmt.Fprintln(os.Stderr, "steward arm: --temporary-human-word and --review-by travel together")
		return 2
	}
	if *temporaryWord == "" {
		if !requireHumanStewardEnrollment(*repo, "steward arm") {
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
	bin, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward arm: %v\n", err)
		return 1
	}
	var msg string
	if *temporaryWord != "" {
		msg, err = steward.ArmTemporary(*repo, bin, *temporaryWord, *reviewBy)
	} else {
		msg, err = steward.Arm(*repo, bin)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward arm: %v\n", err)
		return 1
	}
	fmt.Println(msg)
	return 0
}

func runStewardRestart(args []string) int {
	flags := flag.NewFlagSet("steward restart", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward restart: --repo is required")
		return 2
	}
	if !requireHumanStewardEnrollment(*repo, "steward restart") {
		return 1
	}
	bin, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward restart: %v\n", err)
		return 1
	}
	msg, err := steward.Restart(*repo, bin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward restart: %v\n", err)
		return 1
	}
	fmt.Println(msg)
	return 0
}

func requireHumanStewardEnrollment(repo, verb string) bool {
	metasystemRoot, err := upMetasystemRoot("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot resolve the installed engine: %v\n", verb, err)
		return false
	}
	classification, err := lease.ClassifyAt(repo, metasystemRoot, int64(os.Getppid()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: human ancestry proof failed: %v\n", verb, err)
		return false
	}
	if classification.Class != lease.ClassHuman {
		fmt.Fprintf(os.Stderr, "%s: explicit engine enrollment requires an agent-free terminal; caller classified %s\n", verb, classification.Class)
		return false
	}
	return true
}

func runStewardDisarm(args []string) int {
	flags := flag.NewFlagSet("steward disarm", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward disarm: --repo is required")
		return 2
	}
	msg, err := steward.Disarm(*repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward disarm: %v\n", err)
		return 1
	}
	fmt.Println(msg)
	return 0
}

// runStewardPending prints one line naming undelivered incidents —
// empty output means none, so shell callers can gate on it.
func runStewardPending(args []string) int {
	flags := flag.NewFlagSet("steward pending", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward pending: --repo is required")
		return 2
	}
	pending, err := steward.PendingNotifications(*repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward pending: %v\n", err)
		return 1
	}
	if len(pending) == 0 {
		return 0
	}
	fmt.Printf("%d undelivered; newest: %s\n", len(pending), pending[len(pending)-1].Message)
	return 0
}

// runStewardStatus is the operator's view: the last evidence state,
// live intents, and pending notifications — the second visibility
// channel the design pins.
func runStewardStatus(args []string) int {
	flags := flag.NewFlagSet("steward status", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
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
	report := map[string]any{"evidence": evidence, "liveIntents": intents, "pendingNotifications": pending}
	var problems []string
	for _, err := range []error{evErr, intErr, pendErr} {
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
