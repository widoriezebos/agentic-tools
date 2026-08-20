package main

// The steward family: the idle watchdog's decision surface. check
// reads and reports without side effects; tick folds one observation
// into the evidence and reports what the schedule glue should do;
// status shows the operator everything pending. The revive action
// itself lands with the dispatch continuation mode.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
	out, _ := json.MarshalIndent(map[string]any{
		"goal": it.Goal, "jobId": it.JobId, "runtime": it.Runtime, "model": it.Model,
		"roleDigest": it.RoleDigest, "briefDigest": it.BriefDigest, "permsDigest": it.PermsDigest,
	}, "", "  ")
	fmt.Println(string(out))
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
