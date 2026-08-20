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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	dispatchpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func stewardCensusFor(repo string) steward.WorkerCensus {
	return steward.LiveWorkerCensus{MetasystemRoot: repo}
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
		return 1
	}
	out, _ := json.MarshalIndent(map[string]any{
		"verdict":  result.Decision.Verdict,
		"action":   result.Decision.Action,
		"reason":   result.Decision.Reason,
		"openWork": result.OpenWork,
		"evidence": result.Evidence,
	}, "", "  ")
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
	if !it.Notified {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: intent %s was never delivered to the operator\n", *nonce)
		return 1
	}
	if err := steward.VerifyStagedDigests(*repo, it); err != nil {
		fmt.Fprintf(os.Stderr, "steward authorize-dispatch: %v\n", err)
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
// launch bytes, mint the intent under the lock, deliver the
// notification, then complete through the critical section with the
// real dispatcher as the launch. Safe to re-run: an undelivered
// intent resumes at its gate, a consumed one refuses.
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
		if err := steward.PrepareIntent(*repo, it); err != nil {
			fmt.Fprintf(os.Stderr, "steward revive: %v\n", err)
			return 1
		}
	}

	outcome, err := steward.CompleteRevival(*repo, steward.TickConfig{}, stewardCensusFor(*repo), nonce,
		func(it steward.Intent) error {
			cmd := exec.Command(filepath.Join(*repo, "scripts", "agents", "dispatch.sh"),
				"--steward-intent", it.Nonce)
			cmd.Dir = *repo
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("dispatch: %v (%s)", err, strings.TrimSpace(string(out)))
			}
			return nil
		})
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward revive: %v\n", err)
		return 1
	}
	fmt.Printf("launched=%v reason=%s\n", outcome.Launched, outcome.Reason)
	if !outcome.Launched {
		return 3
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
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "steward arm: --repo is required")
		return 2
	}
	bin, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward arm: %v\n", err)
		return 1
	}
	msg, err := steward.Arm(*repo, bin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "steward arm: %v\n", err)
		return 1
	}
	fmt.Println(msg)
	return 0
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
