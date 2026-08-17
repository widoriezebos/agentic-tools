package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

// The goal family (D67): the doctrine commands humans and agents type.
// Mutations classify the caller and run the same authority matrix every
// record-writer path runs (holder-only), then hand the goal package a
// distilled caller; the goal-specific gates — active mission, baseline
// discipline, the advisory human reservation — live in the package.

// goalCaller classifies the invoking process and authorizes a mutation.
// Reconcile against a root with NO accepted baseline is GENESIS: the
// control plane being seeded does not exist yet, so the human or a
// main agent may seed it without holding a lease nobody could hold
// (the adopt/provisioning path; goal-system GOAL-14's reconcile-only
// initialization).
//
// The genesis rule (D84, restated honestly per D93's C' ruling and
// the verification rounds): the effective class is the SOURCE view
// when it authenticates as an announced MAIN, else the TARGET view.
// This closes the accidental cases (a missing or malformed source
// cannot raise; a real delegate against a signature-carrying target
// reads DELEGATE and is refused) and is explicitly COOPERATIVE, not
// unforgeable: a deliberately crafted source root with a copied live
// announcement can read MAIN, and a machinery caller against a
// signature-free virgin target can read HUMAN — both remain open by
// the C' decision (unforgeable local genesis is not a product
// contract; a notEnforced delegate writes the baseline directly
// regardless).
func goalCaller(root string, callerPid int64, verb, genesisFrom string) (goal.Caller, error) {
	if callerPid == 0 {
		callerPid = int64(os.Getppid())
	}
	mode := "holder-only"
	genesis := verb == "reconcile"
	if genesis {
		if _, statErr := os.Stat(filepath.Join(root, "plans", "goals-accepted.json")); statErr == nil {
			genesis = false // an initialized project: holder-only, unchanged
		}
	}

	targetView, err := lease.ClassifyVerb(root, callerPid)
	if err != nil {
		return goal.Caller{}, fmt.Errorf("caller classification failed: %v", err)
	}
	effective := targetView
	if genesis {
		mode = "genesis"
		if genesisFrom != "" {
			fromView, fromErr := lease.ClassifyVerb(genesisFrom, callerPid)
			if fromErr != nil {
				return goal.Caller{}, fmt.Errorf("genesis-from classification failed: %v", fromErr)
			}
			effective, err = genesisEffective(targetView, fromView)
			if err != nil {
				return goal.Caller{}, err
			}
		}
	}

	classification := map[string]any{"class": effective.Class, "holder": effective.Holder}
	if err := authority.Authorize(mode, classification, ""); err != nil {
		return goal.Caller{}, err
	}
	// The store's genesis branch re-checks under the lock that a
	// non-holder only seeds a goal-free ledger (review F2/F3), so the
	// holder bit carried here is the target's, not the source's. The
	// Genesis flag makes the authorization MODE travel with the caller:
	// the store refuses a genesis-admitted caller every non-genesis arm
	// under its lock (the pre-lock race, re-review finding 4).
	return goal.Caller{Class: effective.Class, Holder: targetView.Holder, Genesis: mode == "genesis"}, nil
}

// genesisEffective decides the effective genesis class from the two
// classifications — pure, so the rule lives in table tests. A
// positively-announced main against the source is the only way
// --genesis-from RAISES the effective class; any other source result
// (HUMAN, or a machinery class) falls through to the TARGET's own
// view, which decides.
//
// The cooperative posture, recorded plainly (D93 ruled C': unforgeable
// genesis is NOT a product contract): a machinery-refusal was tried
// here and REVERTED — it broke legitimate flows. WHO passes genesis is
// decided by the classifier's nearest-agent-ancestor rule, per node
// announcements-before-signatures:
//
//   - a HUMAN terminal (no agent ancestor) → HUMAN → admitted;
//   - a command whose nearest agent ancestor IS the announced main
//     (the orchestrating session running fixtures or the kit gate,
//     with --genesis-from/METASYSTEM_GENESIS_AUTHORITY_ROOT naming a
//     root that carries the announcement) → MAIN → admitted;
//   - a TRUE delegate (its nearest agent ancestor is a delegate CLI,
//     nearer than any announced main) → DELEGATE at the source, and
//     the target view decides — which refuses wherever the target
//     classifies it as machinery. A delegate seeding a control plane
//     is refused BY DESIGN; the kit gate inside a delegate sandbox
//     therefore cannot perform genesis itself and relies on the
//     provisioner having seeded the baseline first.
//
// Two cooperative holes REMAIN OPEN by decision, not oversight: a
// crafted source root carrying a copied live main announcement
// classifies MAIN and raises, and a machinery caller against a
// signature-free virgin target can read HUMAN there. Both require
// deliberate construction — outside the accidental scope C' keeps —
// and a notEnforced delegate could write the baseline directly
// regardless, which is why D86/D92 concluded this boundary cannot be
// load-bearing.
func genesisEffective(targetView, fromView lease.ClassifyResult) (lease.ClassifyResult, error) {
	if fromView.Class == "MAIN" {
		return fromView, nil
	}
	return targetView, nil
}

// goalMutation is the shared verb spine: flags, classification, the
// mutation, and the result on stdout.
func goalMutation(name string, args []string, extra func(*flag.FlagSet) []*string,
	run func(*goal.Store, goal.Caller, []string) (goal.Result, error)) int {
	flags := flag.NewFlagSet("goal "+name, flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	callerPid := flags.Int64("caller-pid", 0, "caller pid (defaults to the parent process)")
	genesisFrom := flags.String("genesis-from", "", "source root whose announcements authenticate a genesis reconcile")
	var extras []*string
	if extra != nil {
		extras = extra(flags)
	}
	if flags.Parse(args) != nil {
		return 2
	}
	caller, err := goalCaller(*root, *callerPid, name, *genesisFrom)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	values := make([]string, len(extras))
	for i, ptr := range extras {
		values[i] = *ptr
	}
	store := &goal.Store{Root: *root}
	result, err := run(store, caller, values)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(result.Message)
	for _, dropped := range result.Dropped {
		fmt.Println("dropped: " + dropped)
	}
	return 0
}

func runGoalOpen(args []string) int {
	return goalMutation("open", args, func(f *flag.FlagSet) []*string {
		return []*string{
			f.String("id", "", "kebab-case goal id"),
			f.String("intent", "", "one-line intent"),
			f.String("next", "", "the first next step"),
		}
	}, func(s *goal.Store, c goal.Caller, v []string) (goal.Result, error) {
		return s.Open(c, v[0], v[1], v[2])
	})
}

func runGoalSetNext(args []string) int {
	return goalMutation("set-next", args, func(f *flag.FlagSet) []*string {
		return []*string{f.String("next", "", "the rewritten step")}
	}, func(s *goal.Store, c goal.Caller, v []string) (goal.Result, error) {
		return s.SetNext(c, v[0])
	})
}

func runGoalPromote(args []string) int {
	return goalMutation("promote", args, func(f *flag.FlagSet) []*string {
		return []*string{f.String("id", "", "queued goal id")}
	}, func(s *goal.Store, c goal.Caller, v []string) (goal.Result, error) {
		return s.Promote(c, v[0])
	})
}

func runGoalPark(args []string) int {
	return goalMutation("park", args, func(f *flag.FlagSet) []*string {
		return []*string{
			f.String("id", "", "goal id"),
			f.String("because", "", "why it parks"),
			f.String("then", "", "queued id to promote in the same write"),
			boolAsString(f, "and-none"),
		}
	}, func(s *goal.Store, c goal.Caller, v []string) (goal.Result, error) {
		return s.Park(c, v[0], v[1], v[2], v[3] == "true")
	})
}

func runGoalUnpark(args []string) int {
	return goalMutation("unpark", args, func(f *flag.FlagSet) []*string {
		return []*string{f.String("id", "", "parked goal id")}
	}, func(s *goal.Store, c goal.Caller, v []string) (goal.Result, error) {
		return s.Unpark(c, v[0])
	})
}

func runGoalDone(args []string) int {
	return goalMutation("done", args, func(f *flag.FlagSet) []*string {
		return []*string{
			f.String("id", "", "the Current goal's id"),
			f.String("concluded", "", "one concluding sentence"),
			f.String("then", "", "queued id to promote in the same write"),
			boolAsString(f, "and-none"),
		}
	}, func(s *goal.Store, c goal.Caller, v []string) (goal.Result, error) {
		return s.Done(c, v[0], v[1], v[2], v[3] == "true")
	})
}

func runGoalReopen(args []string) int {
	return goalMutation("reopen", args, func(f *flag.FlagSet) []*string {
		return []*string{
			f.String("id", "", "done goal id"),
			f.String("next", "", "the reopened next step"),
		}
	}, func(s *goal.Store, c goal.Caller, v []string) (goal.Result, error) {
		return s.Reopen(c, v[0], v[1])
	})
}

func runGoalDeclareFree(args []string) int {
	return goalMutation("declare-free", args, nil,
		func(s *goal.Store, c goal.Caller, v []string) (goal.Result, error) {
			return s.DeclareFree(c)
		})
}

func runGoalPrune(args []string) int {
	return goalMutation("prune", args, nil,
		func(s *goal.Store, c goal.Caller, v []string) (goal.Result, error) {
			return s.Prune(c)
		})
}

func runGoalReconcile(args []string) int {
	return goalMutation("reconcile", args, nil,
		func(s *goal.Store, c goal.Caller, v []string) (goal.Result, error) {
			return s.Reconcile(c)
		})
}

// boolAsString adapts a boolean flag into the shared string plumbing.
func boolAsString(f *flag.FlagSet, name string) *string {
	value := "false"
	f.BoolFunc(name, name, func(string) error {
		value = "true"
		return nil
	})
	return &value
}

// runGoalList prints the parsed ledger as JSON — read-only, never
// mutating, with problems carried as degraded facts.
func runGoalList(args []string) int {
	flags := flag.NewFlagSet("goal list", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	store := &goal.Store{Root: *root}
	ledger, problems, err := store.ReadLedger()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	out := map[string]any{
		"problems":        problems,
		"baselinePresent": store.BaselinePresent(),
		// The read-only pair fact (bytes AND digest match the accepted
		// baseline) — what a second reconcile's "already reconciled"
		// proves, exposed without a mutating verb so consistency checks
		// need no write authority (and no python, kill-python doctrine).
		"baselineMatches": store.BaselineMatches(),
	}
	if ledger != nil {
		out["current"] = ledger.Current
		out["queued"] = ledger.Queued
		out["parked"] = ledger.Parked
		out["done"] = ledger.Done
		out["goalFree"] = ledger.Free
	}
	printJSON(out)
	return 0
}

// runGoalNext prints the one orientation line any runtime's main can read
// by instruction — the universal fallback transport.
func runGoalNext(args []string) int {
	flags := flag.NewFlagSet("goal next", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	store := &goal.Store{Root: *root}
	ledger, problems, err := store.ReadLedger()
	switch {
	case err != nil:
		fmt.Fprintln(os.Stderr, err)
		return 1
	case ledger == nil && store.BaselinePresent():
		fmt.Println("goal ledger degraded: goals.md was deleted after adoption; run `goal reconcile`")
	case ledger == nil:
		fmt.Println("no goal ledger; `goal open` starts one")
	case len(problems) > 0:
		fmt.Println("goal ledger degraded: " + string(problems[0]))
	case ledger.Current != nil:
		fmt.Printf("%s — %s; next: %s\n", ledger.Current.Id, ledger.Current.Intent, ledger.Current.NextStep)
	case ledger.Free != nil:
		fmt.Println("goal-free declared " + ledger.Free.Declared)
	case len(ledger.Queued) > 0:
		fmt.Printf("no current goal; the queue holds %s: `goal promote %s` or park it\n", ledger.Queued[0].Id, ledger.Queued[0].Id)
	default:
		fmt.Println("no current goal")
	}
	return 0
}

// runReportTurnVerdict is the Stop hook's one verb: the scanner fills the
// verdict's input contract and the decision returns as JSON on stdout.
// Every representable state is exit 0; nonzero means I/O failure and the
// hook's own fixed degraded message takes over.
func runReportTurnVerdict(args []string) int {
	flags := flag.NewFlagSet("report turn-verdict", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	session := flags.String("session", "", "normalized session id")
	watchdog := flags.String("watchdog-surfaced", "", "sha256 of this turn's watchdog report (empty clears)")
	mainId := flags.String("main-id", "", "the caller main identity for the unwatched-work rule")
	if flags.Parse(args) != nil {
		return 2
	}
	scan := report.Scan(*root)
	verdict, err := (&goal.Store{Root: *root}).TurnVerdict(scan, *session, *watchdog, *mainId)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data, err := json.Marshal(verdict)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(string(data))
	return 0
}
