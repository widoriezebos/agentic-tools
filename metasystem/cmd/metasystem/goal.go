package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	dispatchpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// goalCommandNow keeps the wall clock authoritative unless the target root
// explicitly authorizes fixture inputs through its fake-runtime config.
func goalCommandNow(root string) (time.Time, error) {
	authorization, err := fixtureauth.New(root)
	if err != nil {
		return time.Time{}, err
	}
	if fixtureNow, ok, err := authorization.Clock().GoalNow(); err != nil {
		return time.Time{}, err
	} else if ok {
		return fixtureNow, nil
	}
	return time.Now().UTC(), nil
}

// The goal family: the doctrine commands humans and agents type.
// Mutations classify the caller and run the same authority matrix every
// record-writer path runs (holder-only), then hand the goal package a
// distilled caller; the goal-specific gates — active mission, baseline
// discipline, the advisory human reservation — live in the package.

// goalCaller classifies the invoking process and authorizes a mutation.
// Reconcile against a root with NO accepted baseline is GENESIS: the
// control plane being seeded does not exist yet, so holder-only would
// protect nothing and refuse everything (the adopt/provisioning path;
// reconcile is the only initialization).
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
// Posture, stated plainly: this is cooperative, not
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

	view, err := classifyVerbCaller(root, callerPid)
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
	if name == "done" && len(values) > 0 {
		return reportAfterConfirmedDone(0, *root, values[0], os.Stderr)
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
	pretty := flags.Bool("pretty", false, "a human table instead of JSON")
	fetch := flags.Bool("fetch", false, "fetch and validate the canonical backlog before listing")
	var labels repeatedStrings
	flags.Var(&labels, "label", "label token required on every listed goal (repeatable)")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := goal.ValidateLabels(labels); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if converted(*root) {
		return listSynced(*root, *pretty, *fetch, labels...)
	}
	if len(labels) > 0 || *fetch {
		fmt.Fprintln(os.Stderr, "goal list --label and --fetch read the synced backlog; this checkout still carries the legacy ledger and must migrate first")
		return 1
	}
	store := &goal.Store{Root: *root}
	ledger, problems, err := store.ReadLedger()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	out := map[string]any{
		"root":            *root,
		"world":           "legacy",
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

// runGoalTierProbe prints the backlog's tier spread: recorded and derived
// tiers over the open goals with a risk record, the tier-3 share of each,
// and the goals whose recorded tier exceeds their derivation.
func runGoalTierProbe(args []string) int {
	flags := flag.NewFlagSet("goal tier-probe", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	pretty := flags.Bool("pretty", false, "print lines instead of JSON")
	fetchFirst := flags.Bool("fetch", false, "fetch the canonical tip before reading")
	if flags.Parse(args) != nil {
		return 2
	}
	e, err := goal.ResolveEndpoint(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p, err := goal.Project(e, *fetchFirst, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	probe := goal.ProbeTiers(p.Tree)
	recorded, derived := probe.Tier3Share()
	if !*pretty {
		printJSON(map[string]any{"open": probe.Open, "recorded": probe.Recorded, "derived": probe.Derived,
			"tier3ShareRecorded": recorded, "tier3ShareDerived": derived, "lowerable": probe.Lowerable, "tip": p.Tip})
		return 0
	}
	fmt.Printf("open goals with a risk record: %d\n", probe.Open)
	fmt.Printf("recorded tiers: 1=%d 2=%d 3=%d (tier 3: %d%%)\n", probe.Recorded[1], probe.Recorded[2], probe.Recorded[3], recorded)
	fmt.Printf("derived tiers:  1=%d 2=%d 3=%d (tier 3: %d%%)\n", probe.Derived[1], probe.Derived[2], probe.Derived[3], derived)
	for _, lower := range probe.Lowerable {
		fmt.Printf("lowerable: %s %s recorded=%d derived=%d\n", lower.ID, lower.State, lower.Recorded, lower.Derived)
	}
	return 0
}

// listSynced prints the accepted world: the same JSON idea as the
// legacy list, grouped by state, with the projection's banners.
func listSynced(root string, pretty, fetchFirst bool, requiredLabels ...string) int {
	e, err := goal.ResolveEndpoint(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p, err := goal.Project(e, fetchFirst, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	grouped := map[string][]*goal.GoalFile{}
	open := make([]*goal.GoalFile, 0, len(p.Tree.Live))
	for _, id := range goal.OrderedOpenGoalIDs(p.Tree.Live) {
		f := p.Tree.Live[id]
		if !goal.MatchesLabels(f.Labels, requiredLabels) {
			continue
		}
		open = append(open, f)
		grouped[f.State] = append(grouped[f.State], f)
	}
	var done []*goal.GoalFile
	for _, id := range goal.SortedGoalIds(p.Tree.Done) {
		if f := p.Tree.Done[id]; goal.MatchesLabels(f.Labels, requiredLabels) {
			done = append(done, f)
		}
	}
	if pretty {
		for _, banner := range p.Banners {
			fmt.Println("! " + banner)
		}
		writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(writer, "PRIORITY\tSEQUENCE\tSTATE\tPIN\tGOAL")
		for _, f := range open {
			priority, sequence, pin := "-", "-", "-"
			if f.Priority != 0 {
				priority = fmt.Sprintf("%d", f.Priority)
			}
			if f.Sequence != 0 {
				sequence = fmt.Sprintf("%d", f.Sequence)
			}
			if f.Pinned != "" {
				pin = f.Pinned
			}
			fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", priority, sequence, f.State, pin, f.Id)
			var details []string
			if f.Intent != "" {
				details = append(details, f.Intent)
			}
			if f.Claimed != nil {
				details = append(details, "claimed by "+f.Claimed.Machine+"+"+f.Claimed.Lineage)
			}
			if f.Parked != nil && f.Parked.Because != "" {
				details = append(details, "parked: "+f.Parked.Because)
			}
			if f.Approved != nil && f.Approved.Authority == goal.ApprovalAuthorityRelayed {
				if expired, why := f.ApprovalExpired(p.Horizon); expired {
					details = append(details, "relayed, EXPIRED: "+why)
				} else {
					details = append(details, "relayed, review by "+f.Approved.ReviewBy)
				}
			}
			if len(details) > 0 {
				fmt.Fprintf(writer, "\t\t\t\t  %s\n", strings.Join(details, "; "))
			}
		}
		_ = writer.Flush()
		fmt.Printf("done: %d archived\n", len(done))
		return 0
	}
	printJSON(map[string]any{
		"root": root, "world": "synced", "tip": p.Tip, "banners": p.Banners,
		"open":   open,
		"queued": grouped[goal.StateQueued], "approved": grouped[goal.StateApproved], "claimed": grouped[goal.StateClaimed],
		"parked": grouped[goal.StateParked], "done": done,
	})
	return 0
}

// runGoalShow addresses ONE goal: fields, claim, park, and history,
// from the accepted tree.
func runGoalShow(args []string) int {
	flags := flag.NewFlagSet("goal show", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	if flags.Parse(args) != nil || *id == "" {
		fmt.Fprintln(os.Stderr, "goal show needs --id")
		return 2
	}
	if !converted(*root) {
		fmt.Fprintln(os.Stderr, "goal show reads the synced backlog; this checkout still carries the legacy ledger (goal list shows it whole)")
		return 1
	}
	e, err := goal.ResolveEndpoint(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p, err := goal.Project(e, false, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	f, live := p.Tree.Live[*id]
	state := "live"
	if !live {
		if f = p.Tree.Done[*id]; f == nil {
			fmt.Fprintf(os.Stderr, "no goal %q on the accepted tree (tip %s)\n", *id, p.Tip)
			return 1
		}
		state = "archived"
	}
	printJSON(map[string]any{
		"root": *root, "world": "synced", "tip": p.Tip, "where": state, "goal": f,
	})
	return 0
}

// nextSynced prints the ordered frontier line for one machine.
func nextSynced(root, machine string, fetchFirst bool, requiredLabels ...string) int {
	e, err := goal.ResolveEndpoint(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	now, err := goalCommandNow(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p, err := goal.Project(e, fetchFirst, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// Projection notices such as stale or single-machine state print before
	// orientation so the caller sees the limits on the answer.
	for _, banner := range p.Banners {
		fmt.Println(banner)
	}
	frontier, frontierErr := goal.Next(p, machine, requiredLabels...)
	if frontierErr != nil {
		fmt.Fprintln(os.Stderr, "goal next could not answer: "+frontierErr.Error())
		return 1
	}
	fenced := make([]*goal.GoalFile, 0, len(frontier.Fenced))
	for _, id := range frontier.Fenced {
		fenced = append(fenced, p.Tree.Live[id])
	}
	for _, line := range goal.FencedClaimLines(fenced) {
		fmt.Println(line)
	}
	selection := goal.SelectNext(frontier)
	switch selection.Kind {
	case goal.NextSelectionContinue:
		fmt.Println("continue your claimed goal: " + selection.GoalID)
	case goal.NextSelectionReady:
		fmt.Println("next ready goal: " + selection.GoalID)
	default:
		if len(requiredLabels) > 0 {
			matched := false
			for _, file := range p.Tree.Live {
				if goal.MatchesLabels(file.Labels, requiredLabels) {
					matched = true
					break
				}
			}
			if !matched {
				fmt.Println("no goal matches --label " + strings.Join(requiredLabels, " --label "))
				return 0
			}
		}
		line := "no claimable goal for machine " + machine
		switch {
		case len(frontier.Refused) > 0:
			line += fmt.Sprintf("; claim would refuse %d (first: %s): %s", len(frontier.Refused), frontier.Refused[0].GoalID, frontier.Refused[0].Cause)
		case len(frontier.Blocked) > 0:
			line += "; first blocked goal: " + frontier.Blocked[0]
		case len(frontier.Awaiting) > 0:
			line += fmt.Sprintf("; %d await the human's approval (first: %s)", len(frontier.Awaiting), frontier.Awaiting[0])
		case len(p.Tree.Live) == 0:
			line += "; the backlog is empty"
		default:
			line += "; no matching eligible work"
		}
		fmt.Println(line)
	}
	return 0
}

// runGoalNext prints the one orientation line any runtime's main can read
// by instruction — the universal fallback transport.
func runGoalNext(args []string) int {
	flags := flag.NewFlagSet("goal next", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	machineFlag := flags.String("machine", "", "machine nickname whose ordered frontier to inspect")
	fetch := flags.Bool("fetch", false, "fetch and validate the canonical backlog before selecting")
	var labels repeatedStrings
	flags.Var(&labels, "label", "label token required on recommendation candidates (repeatable)")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal next accepts flags only")
		return 2
	}
	if err := goal.ValidateLabels(labels); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	machineProvided := false
	flags.Visit(func(flag *flag.Flag) {
		if flag.Name == "machine" {
			machineProvided = true
		}
	})
	if converted(*root) {
		machine := *machineFlag
		if machineProvided {
			if err := goal.ValidateMachineNickname(machine); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		} else {
			var err error
			machine, err = goal.ResolveMachine(*root)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
		return nextSynced(*root, machine, *fetch, labels...)
	}
	if len(labels) > 0 || machineProvided || *fetch {
		fmt.Fprintln(os.Stderr, "goal next --label, --machine, and --fetch read the synced backlog; this checkout still carries the legacy ledger and must migrate first")
		return 1
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
	stopHookActive := flags.Bool("stop-hook-active", false, "the runtime is repeating a Stop hook that previously blocked")
	if flags.Parse(args) != nil {
		return 2
	}
	scan := report.Scan(*root)
	now, err := goalCommandNow(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(scan.Busy) == 0 {
		var warning string
		scan.Open, warning, err = report.MarkOpenWorkSeen(*root, scan.Open, now)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if warning != "" {
			scan.OpenWorkWarnings = append(scan.OpenWorkWarnings, warning)
		}
	}
	store := &goal.Store{Root: *root, Now: func() time.Time { return now }}
	options := goal.TurnVerdictOptions{StopHookActive: *stopHookActive}
	stateRoot, rootErr := goal.ResolveStateRoot(*root)
	if rootErr != nil {
		options.SeatActorProblem = "the seat state root could not be resolved: " + rootErr.Error()
	} else {
		store.PrepareIdleContinuation = prepareSeatIdleContinuation(stateRoot)
		store.RecordIdleIncident = recordSeatIdleIncident(stateRoot, now)
		store.RaiseIdleAlarm = raiseSeatIdleAlarm(stateRoot)
		store.ResolveIdleSeat = resolveSeatIdleActor(stateRoot, *mainId)
	}
	verdict, err := store.TurnVerdict(scan, *session, *watchdog, *mainId, options)
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

func resolveSeatIdleActor(root, mainID string) func() (goal.Actor, int64, error) {
	return func() (goal.Actor, int64, error) {
		machine, err := goal.ResolveMachine(root)
		if err != nil {
			return goal.Actor{}, 0, fmt.Errorf("the seat machine could not be resolved: %w", err)
		}
		holder, err := lease.CurrentHolder(root)
		if err != nil {
			return goal.Actor{}, 0, fmt.Errorf("the announced checkout holder could not be resolved: %w", err)
		}
		if mainID == "" || holder.MainId != mainID {
			return goal.Actor{}, 0, fmt.Errorf("the Stop main %q does not match the announced checkout holder %q", mainID, holder.MainId)
		}
		if holder.SessionId == "" || holder.OwnerLineage == "" {
			return goal.Actor{}, 0, fmt.Errorf("the checkout holder has no readable main announcement and lineage")
		}
		return goal.Actor{Machine: machine, Lineage: holder.OwnerLineage}, holder.ClaimEpoch, nil
	}
}

func prepareSeatIdleContinuation(root string) func(goal.IdleEscalationEvent) (string, error) {
	return func(event goal.IdleEscalationEvent) (string, error) {
		roster, err := dispatchpkg.ResolveRoster(dispatchpkg.RosterParams{
			ConfPath: filepath.Join(root, "metasystem.conf"),
			Role:     "steward-continuation", Mode: "build",
		})
		if err != nil {
			return "", fmt.Errorf("steward continuation roster could not be resolved: %w", err)
		}
		raw := make([]byte, 8)
		if _, err := rand.Read(raw); err != nil {
			return "", fmt.Errorf("steward continuation nonce could not be minted: %w", err)
		}
		nonce := hex.EncodeToString(raw)
		intent, err := steward.StageIntent(root, nonce, event.GoalID, "steward-"+nonce,
			roster.Runtime, roster.Model, "seatIdle")
		if err != nil {
			return "", err
		}
		intent.ClaimNeeded = event.ClaimNeeded
		intent.SeatActor = &steward.SeatActor{
			Machine: event.ClaimActor.Machine,
			Lineage: event.ClaimActor.Lineage,
		}
		intent.SeatClaimEpoch = event.SeatClaimEpoch
		if err := steward.PrepareIntent(root, filepath.Join(root, "memory", "receipts.log"), intent); err != nil {
			return "", err
		}
		return nonce, nil
	}
}

func recordSeatIdleIncident(root string, now time.Time) func(goal.IdleEscalationEvent) (string, error) {
	return func(event goal.IdleEscalationEvent) (string, error) {
		actor := ""
		if event.ClaimActor.Machine != "" || event.ClaimActor.Lineage != "" {
			actor = event.ClaimActor.Machine + "+" + event.ClaimActor.Lineage
		}
		episode, err := steward.RecordSeatIdleIncident(root, steward.SeatIdleIncident{
			SessionID: event.SessionID, MainID: event.MainID, GoalID: event.GoalID,
			BacklogDigest: event.BacklogDigest, Refusal: event.Refusal,
			StopHookActive: event.StopHookActive, ClaimActor: actor,
			ClaimNeeded: event.ClaimNeeded, SeatClaimEpoch: event.SeatClaimEpoch,
			ClaimMade: event.ClaimMade, ClaimDetail: event.ClaimDetail,
			IntentID: event.IntentID, IntentPrepared: event.IntentPrepared, IntentDetail: event.IntentDetail,
		}, now)
		if err != nil {
			return "", err
		}
		return episode.EpisodeID, nil
	}
}

func raiseSeatIdleAlarm(root string) func(goal.IdleEscalationEvent) error {
	return func(event goal.IdleEscalationEvent) error {
		verdict := steward.VerdictIdleBacklogDead
		return steward.QueueNotification(root, steward.PendingNotification{
			Nonce:   "verdict-" + string(verdict),
			Message: fmt.Sprintf("steward: %s — seat %s reached idle refusal %d, but no steward continuation intent could be prepared: %s; %s", verdict, event.SessionID, event.Refusal, event.ClaimDetail, event.IntentDetail),
		})
	}
}
