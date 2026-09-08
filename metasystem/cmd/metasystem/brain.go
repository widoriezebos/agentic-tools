package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

func brainHumanAct(root, verb string, fixture bool) error {
	if fixture {
		authorization, err := fixtureauth.New(root)
		if err != nil {
			return err
		}
		if authorization.GoalHumanAuthority().Allows(root) {
			return nil
		}
	}
	classification, err := classifyVerbCaller(root, int64(os.Getppid()))
	if err != nil {
		return fmt.Errorf("brain %s could not classify its caller: %w", verb, err)
	}
	if classification.Class != lease.ClassHuman {
		return fmt.Errorf("brain %s is a human act; run it from an agent-free terminal", verb)
	}
	return nil
}

func runBrainDeclare(args []string) int {
	flags := flag.NewFlagSet("brain declare", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout state root")
	by := flags.String("by", "", "human making the declaration")
	fixture := flags.Bool("fixture-human-authority", false, "fixture-only authority under an exact fake-runtime root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *by == "" {
		fmt.Fprintln(os.Stderr, "brain declare needs --by")
		return 2
	}
	if err := brainHumanAct(*root, "declare", *fixture); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ledgerIdentity := goal.ExistingLedgerIdentity(*root)
	if ledgerIdentity == "" {
		fmt.Fprintln(os.Stderr, "brain declare needs a migrated ledger identity; run metasystem goal migrate first")
		return 2
	}
	machine, err := goal.ResolveMachine(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	if err := brain.ValidateDeclaration(ledgerIdentity, machine, *by, stamp); err != nil {
		fmt.Fprintln(os.Stderr, "brain declare refused:", err)
		return 2
	}
	state := brain.Read(*root, ledgerIdentity)
	if state.State == brain.Declared {
		fmt.Fprintf(os.Stderr, "this checkout is already the brain of ledger %s, declared by %s at %s; withdraw it first: metasystem brain withdraw --root %s --by <name>\n", state.Record.Ledger, state.Record.DeclaredBy, state.Record.DeclaredAt, *root)
		return 2
	}
	if state.State == brain.Corrupt {
		fmt.Fprintln(os.Stderr, brain.RemedialRefusal(state.Reason, *root))
		return 2
	}
	if obstacles, err := brainDeclarationObstacles(*root, machine); err != nil {
		fmt.Fprintln(os.Stderr, "brain declare could not prove quiescence:", err)
		return 2
	} else if len(obstacles) > 0 {
		for _, obstacle := range obstacles {
			fmt.Fprintln(os.Stderr, obstacle)
		}
		return 2
	}
	registryHome, err := brain.RegistryHome()
	if err != nil {
		fmt.Fprintln(os.Stderr, "brain declare:", err)
		return 2
	}
	record, err := brain.Declare(brain.DeclareOptions{StateRoot: *root, RegistryHome: registryHome, LedgerIdentity: ledgerIdentity, Machine: machine, DeclaredBy: *by, Now: time.Now().UTC()})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	printJSON(map[string]any{"state": brain.Declared, "record": record})
	return 0
}

func brainDeclarationObstacles(root, machine string) ([]string, error) {
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return nil, err
	}
	projection, err := goal.Project(endpoint, false, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	var obstacles []string
	for _, id := range sortedGoalIDs(projection.Tree.Live) {
		item := projection.Tree.Live[id]
		if item.Claimed == nil || item.Claimed.Machine != machine {
			continue
		}
		command := fmt.Sprintf("metasystem goal release --root %s --id %s", root, id)
		if item.Arc != "" {
			command += " --arc " + item.Arc
		}
		obstacles = append(obstacles, fmt.Sprintf("goal %s is claimed here; release it to a node: %s", id, command))
	}
	scan := report.ScanForBrainDeclaration(root)
	if len(scan.Unreadable) > 0 || len(scan.RunUnreadable) > 0 {
		return nil, fmt.Errorf("scanner inputs are unreadable: %s", strings.Join(append(scan.Unreadable, scan.RunUnreadable...), "; "))
	}
	for _, item := range scan.Busy {
		switch item.Kind {
		case "job":
			obstacles = append(obstacles, fmt.Sprintf("job %s is in flight; stop it first: metasystem delegate --cancel %s", item.Id, item.Id))
		case "run":
			obstacles = append(obstacles, fmt.Sprintf("run %s is live; wait for it or conclude it: metasystem run watch --root %s --id %s", item.Id, root, item.Id))
		case "mission":
			obstacles = append(obstacles, fmt.Sprintf("mission %s is active; wait for mission %s to finish or park; metasystem mission status --root %s --mission %s", item.Id, item.Id, root, item.Id))
		}
	}
	return obstacles, nil
}

func sortedGoalIDs(items map[string]*goal.GoalFile) []string {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func runBrainShow(args []string) int {
	flags := flag.NewFlagSet("brain show", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout state root")
	if flags.Parse(args) != nil {
		return 2
	}
	result := brain.Read(*root, goal.ExistingLedgerIdentity(*root))
	object := map[string]any{"state": result.State}
	if result.Record != nil {
		object["record"] = result.Record
	}
	if result.Reason != "" {
		object["reason"] = result.Reason
		object["remedy"] = brain.RemedialRefusal(result.Reason, *root)
	}
	printJSON(object)
	switch result.State {
	case brain.Declared:
		return 0
	case brain.Undeclared:
		return 3
	default:
		fmt.Fprintln(os.Stderr, brain.RemedialRefusal(result.Reason, *root))
		return 1
	}
}

func runBrainWithdraw(args []string) int {
	flags := flag.NewFlagSet("brain withdraw", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout state root")
	by := flags.String("by", "", "human withdrawing the declaration")
	fixture := flags.Bool("fixture-human-authority", false, "fixture-only authority under an exact fake-runtime root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *by == "" {
		fmt.Fprintln(os.Stderr, "brain withdraw needs --by")
		return 2
	}
	if err := brainHumanAct(*root, "withdraw", *fixture); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	registryHome, err := brain.RegistryHome()
	if err != nil {
		fmt.Fprintln(os.Stderr, "brain withdraw:", err)
		return 2
	}
	removed, err := brain.Withdraw(*root, registryHome, goal.ExistingLedgerIdentity(*root))
	if err != nil {
		fmt.Fprintln(os.Stderr, "brain withdraw:", err)
		return 2
	}
	if !removed {
		return 3
	}
	printJSON(map[string]any{"state": brain.Undeclared})
	return 0
}

func runBrainFence(args []string) int {
	flags := flag.NewFlagSet("brain fence", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout state root")
	act := flags.String("act", "", "guarded act")
	if flags.Parse(args) != nil || *act == "" {
		fmt.Fprintln(os.Stderr, "brain fence needs --act")
		return 2
	}
	ledgerIdentity := goal.ExistingLedgerIdentity(*root)
	state := brain.Read(*root, ledgerIdentity)
	detail := brain.Fence(*root, *act, ledgerIdentity)
	printJSON(map[string]any{"state": state.State, "fenced": detail != "", "detail": detail})
	if detail != "" {
		return 2
	}
	return 0
}

func runBrainBoot(args []string) int {
	return runBrainBootCommand(args)
}
