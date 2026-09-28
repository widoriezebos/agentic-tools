package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// The goal beds observe the accepted ledger through these readers of the
// projection: the listing and one goal's page in the JSON shapes the
// retired goal list and goal show printed, so each bed reads its facts
// unchanged. Test code; the public goal list and goal show are the people's.

type goalListOutput struct {
	JSON, History, Done, Pretty bool
}

func runGoalListWithResolver(args []string, resolve func(string) (goal.Endpoint, error)) int {
	flags := flag.NewFlagSet("goal ledger listing", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	var output goalListOutput
	flags.BoolVar(&output.JSON, "json", false, "print goal records as JSON")
	flags.BoolVar(&output.History, "history", false, "include ledger history in JSON records")
	flags.BoolVar(&output.Done, "done", false, "include archived goals in the summary")
	flags.BoolVar(&output.Pretty, "pretty", false, "indent --json output")
	fetch := flags.Bool("fetch", false, "fetch and validate the canonical backlog before listing")
	var labels repeatedStrings
	flags.Var(&labels, "label", "label token required on every listed goal (repeatable)")
	if flags.Parse(args) != nil {
		return 2
	}
	e, err := resolve(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p, err := goal.Project(e, *fetch, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	grouped := map[string][]*goal.GoalFile{}
	open := make([]*goal.GoalFile, 0, len(p.Tree.Live))
	for _, id := range goal.OrderedOpenGoalIDs(p.Tree.Live) {
		f := p.Tree.Live[id]
		if !goal.MatchesLabels(f.Labels, labels) {
			continue
		}
		f = goalDisplayRecord(f, output.History)
		open = append(open, f)
		grouped[f.State] = append(grouped[f.State], f)
	}
	var done, abandoned []*goal.GoalFile
	for _, id := range goal.SortedGoalIds(p.Tree.Done) {
		if f := p.Tree.Done[id]; goal.MatchesLabels(f.Labels, labels) {
			done = append(done, goalDisplayRecord(f, output.History))
		}
	}
	for _, id := range goal.SortedGoalIds(p.Tree.Abandoned) {
		if f := p.Tree.Abandoned[id]; goal.MatchesLabels(f.Labels, labels) {
			abandoned = append(abandoned, goalDisplayRecord(f, output.History))
		}
	}
	grouped[goal.StateAbandoned] = abandoned
	if !output.JSON {
		grouped[goal.StateDone] = done
		fmt.Print(goalListSummary(grouped, syncedListStates, p.Tip, p.Banners, output.Done, p.Horizon, p.Tree.TrunkRed...))
		return 0
	}
	trunkRed := p.Tree.TrunkRed
	if trunkRed == nil {
		trunkRed = []goal.TrunkRedEntry{}
	}
	encoder := json.NewEncoder(os.Stdout)
	if output.Pretty {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(map[string]any{
		"root": *root, "world": "synced", "tip": p.Tip, "banners": p.Banners, "open": open,
		"queued": grouped[goal.StateQueued], "approved": grouped[goal.StateApproved], "claimed": grouped[goal.StateClaimed],
		"parked": grouped[goal.StateParked], "done": done, "abandoned": abandoned, "trunkRed": trunkRed,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runGoalShowWithResolver(args []string, resolve func(string) (goal.Endpoint, error)) int {
	flags := flag.NewFlagSet("goal ledger page", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	history := flags.Bool("history", false, "include ledger history")
	if flags.Parse(args) != nil || *id == "" {
		return 2
	}
	e, err := resolve(*root)
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
			f = p.Tree.Abandoned[*id]
		}
		if f == nil {
			fmt.Fprintf(os.Stderr, "no goal %q on the accepted tree (tip %s)\n", *id, p.Tip)
			return 1
		}
		state = "archived"
	}
	page := map[string]any{"root": *root, "world": "synced", "tip": p.Tip, "where": state, "goal": goalDisplayRecord(f, *history)}
	if blocks := goal.OpenReadItemBlocks(f); len(blocks) > 0 {
		page["openReadItems"] = blocks
	}
	printJSON(page)
	return 0
}

func runGoalList(args []string) int { return runGoalListWithResolver(args, goal.ResolveEndpoint) }

func runGoalShow(args []string) int { return runGoalShowWithResolver(args, goal.ResolveEndpoint) }

// The goal beds change the ledger through the goal owners the public goal
// actions call (the synced mutation dispatch, the approval, migration and
// carry owners), under the argument grammar the beds were written in.

func trySyncMutation(name string, args []string) (int, bool) {
	return trySyncMutationWithDependencies(name, args, goalCommandNow, defaultSyncRequestDependencies(), goalParkBranchCheck)
}

func runGoalDoneWithSync(args []string, trySync func(string, []string) (int, bool)) int {
	code, _ := trySync("done", args)
	return code
}

func runGoalApproveWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalApproveWithInputs(args, prove, goalCommandNow, defaultSyncRequestDependencies(), dispatchcore.ResolveGoalBinding)
}

func runGoalMigrate(args []string) int {
	return goalMigrateWith(defaultSyncRequestDependencies(), os.Stdout, os.Stderr, args)
}

func runGoalCarry(args []string) int {
	for _, arg := range args {
		if arg == "--to" || strings.HasPrefix(arg, "--to=") {
			return runGoalCarryAbandonedWithInputs(args, proveEnrolledGoalHumanAuthority, goalCommandNow, defaultSyncRequestDependencies())
		}
	}
	return goalCarryLandingWith(defaultSyncRequestDependencies(), os.Stdout, os.Stderr, args)
}

func goalBranchEndpointTipWithGit(root string, endpoint goal.Endpoint, git func(string, ...string) (string, error)) (string, error) {
	return branch.EndpointTipWithGit(root, endpoint, git)
}

// goalCarryHelperCommand runs the carry owner in a child of this test binary,
// whose own environment carries the bed's fixture clock.
const goalCarryHelperCommand = "test-helper-goal-carry"

func init() { testHelperCommands[goalCarryHelperCommand] = runGoalCarry }
