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
	"time"
)

// The goal family (D67): the doctrine commands humans and agents type.
// Mutations classify the caller and run the same authority matrix every
// record-writer path runs (holder-only), then hand the goal package a
// distilled caller; the goal-specific gates — active mission, baseline
// discipline, the advisory human reservation — live in the package.

// goalCaller classifies the invoking process and authorizes a mutation.
// Reconcile against a root with NO accepted baseline is GENESIS: the
// control plane being seeded does not exist yet, so holder-only would
// protect nothing and refuse everything (the adopt/provisioning path;
// goal-system GOAL-14's reconcile-only initialization).
//
// The genesis rule: the caller is classified against the root being
// written — the same root every goal verb classifies against, never a
// second one the caller names — and the authority matrix admits the
// human, the root's lease holder, and any other caller whose ledger is
// adoption-shaped (goal-free, on a checkout whose history carries no
// ledger; goal.AdoptionShaped). A terminal, an announced session, a
// session whose announcement lapsed, a fixture under agent ancestry and
// the kit gate in a delegate sandbox all seed a new control plane that
// way; nobody but the holder puts intent into one that exists, and the
// store re-judges the shape under its lock.
//
// Posture, stated plainly (D93 ruled C'): this is cooperative, not
// unforgeable. --caller-pid names the ancestry for every classified verb
// in the system, a denied process table reads HUMAN for every verb, and a
// same-user actor can write the control-plane files directly; none of
// that is widened here, and none of it passes through a root this verb
// lets the caller choose.
func goalCaller(root string, callerPid int64, verb string) (goal.Caller, error) {
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

	view, err := lease.ClassifyVerb(root, callerPid)
	if err != nil {
		return goal.Caller{}, fmt.Errorf("caller classification failed: %v", err)
	}
	classification := map[string]any{"class": view.Class, "holder": view.Holder}
	var shapeErr error
	if genesis {
		mode = "genesis"
		// A probe that cannot read refuses the SHAPE, never the human:
		// the flag stays false so the matrix refuses a non-holder, while
		// the human and the holder keep today's rule and the store
		// surfaces the real read error under its lock.
		shaped := false
		ledgerBytes, readErr := os.ReadFile(goal.LedgerPath(root))
		switch {
		case readErr != nil && !os.IsNotExist(readErr):
			shapeErr = readErr
		default:
			shaped, _, shapeErr = goal.AdoptionShaped(root, ledgerBytes)
			if shapeErr != nil {
				shaped = false
			}
		}
		classification["adoptionShaped"] = shaped
	}
	if err := authority.Authorize(mode, classification, ""); err != nil {
		if shapeErr != nil {
			return goal.Caller{}, fmt.Errorf("%v (the adoption-shape probe failed: %v)", err, shapeErr)
		}
		return goal.Caller{}, err
	}
	// The Genesis flag makes the authorization MODE travel with the
	// caller: the store refuses a genesis-admitted caller every
	// non-genesis arm under its lock, and re-judges the adoption shape
	// there for a non-holder.
	return goal.Caller{Class: view.Class, Holder: view.Holder, Genesis: mode == "genesis"}, nil
}

// goalMutation is the shared verb spine: flags, classification, the
// mutation, and the result on stdout.
func goalMutation(name string, args []string, extra func(*flag.FlagSet) []*string,
	run func(*goal.Store, goal.Caller, []string) (goal.Result, error)) int {
	if code, handled := trySyncMutation(name, args); handled {
		return code
	}
	flags := flag.NewFlagSet("goal "+name, flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	callerPid := flags.Int64("caller-pid", 0, "caller pid (defaults to the parent process)")
	var extras []*string
	if extra != nil {
		extras = extra(flags)
	}
	if flags.Parse(args) != nil {
		return 2
	}
	caller, err := goalCaller(*root, *callerPid, name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// Enrollment runs AFTER authorization: it executes the target's
	// pre-commit hook (the behavioral probe), and an unauthorized
	// caller must not be able to trigger foreign hook code through a
	// refused mutation.
	if err := ensureGuardEnrolled(*root); err != nil {
		fmt.Fprintln(os.Stderr, "goal "+name+": "+err.Error())
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
	if converted(*root) {
		return listSynced(*root)
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

// converted reports the post-migration world by POSITIVE evidence:
// the legacy ledger is gone AND the synced tree is present. Absence
// of goals.md alone is not conversion — fixture sandboxes and plain
// directories never had a backlog, and routing them into the sync
// engine sends fetches at remotes that do not exist. The legacy
// file's presence keeps every pre-conversion behavior byte-identical.
func converted(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "plans", "goals.md")); err == nil {
		return false
	}
	_, err := os.Stat(filepath.Join(root, "plans", "goals", "backlog.md"))
	return err == nil
}

// listSynced prints the accepted world: the same JSON idea as the
// legacy list, grouped by state, with the projection's banners.
func listSynced(root string) int {
	e, err := goal.ResolveEndpoint(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p, err := goal.Project(e, false, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	grouped := map[string][]*goal.GoalFile{}
	for _, id := range goal.SortedGoalIds(p.Tree.Live) {
		f := p.Tree.Live[id]
		grouped[f.State] = append(grouped[f.State], f)
	}
	var done []*goal.GoalFile
	for _, id := range goal.SortedGoalIds(p.Tree.Done) {
		done = append(done, p.Tree.Done[id])
	}
	printJSON(map[string]any{
		"world": "synced", "tip": p.Tip, "banners": p.Banners,
		"queued": grouped[goal.StateQueued], "claimed": grouped[goal.StateClaimed],
		"parked": grouped[goal.StateParked], "done": done,
	})
	return 0
}

// nextSynced prints the frontier line for this machine.
func nextSynced(root string) int {
	e, err := goal.ResolveEndpoint(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p, err := goal.Project(e, false, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	v := goal.Next(p, machine)
	switch {
	case len(v.Claimed) > 0:
		fmt.Println("continue your claimed goal: " + v.Claimed[0])
	case len(v.Ready) > 0:
		fmt.Println("next ready goal: " + v.Ready[0])
	case len(v.Blocked) > 0:
		fmt.Println("all queued goals are blocked; the first is " + v.Blocked[0])
	default:
		fmt.Println("the backlog is empty; open a goal or rest")
	}
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
	if converted(*root) {
		return nextSynced(*root)
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
