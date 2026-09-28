package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

// brainActDependencies are the reads the brain declaration owners make
// outside the declaration record: caller classification, the ledger identity
// and machine (Git configuration), the goal projection, the in-flight scan,
// the registry home, and the clock. Production uses
// defaultBrainActDependencies; tests supply per-test instances.
type brainActDependencies struct {
	classify       func(root string, callerPid int64) (lease.ClassifyResult, error)
	ledgerIdentity func(string) string
	machine        func(string) (string, error)
	project        func(root string) (goal.Projection, error)
	scan           func(root string) goal.ScanResult
	registryHome   func() (string, error)
	now            func() time.Time
}

func defaultBrainActDependencies() brainActDependencies {
	return brainActDependencies{
		classify: func(root string, callerPid int64) (lease.ClassifyResult, error) {
			return classifyVerbCaller(root, callerPid)
		},
		ledgerIdentity: goal.ExistingLedgerIdentity,
		machine:        goal.ResolveMachine,
		project: func(root string) (goal.Projection, error) {
			endpoint, err := goal.ResolveEndpoint(root)
			if err != nil {
				return goal.Projection{}, err
			}
			return goal.Project(endpoint, false, time.Now().UTC())
		},
		scan:         report.ScanForBrainDeclaration,
		registryHome: brain.RegistryHome,
		now:          time.Now,
	}
}

func brainHumanAct(caller processIdentity, root, verb string, fixture bool, classify func(string, int64) (lease.ClassifyResult, error)) error {
	if fixture {
		authorization, err := fixtureauth.New(root)
		if err != nil {
			return err
		}
		if authorization.GoalHumanAuthority().Allows(root) {
			return nil
		}
	}
	callerPid, err := caller.classifiablePid(identity.KernelProber{})
	if err != nil {
		return fmt.Errorf("brain %s could not classify its caller: %w", verb, err)
	}
	classification, err := classify(root, callerPid)
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
	root := pathFlag(flags, "root", ".", "checkout state root")
	by := flags.String("by", "", "human making the declaration")
	fixture := flags.Bool("fixture-human-authority", false, "fixture-only authority under an exact fake-runtime root")
	if flags.Parse(args) != nil {
		return 2
	}
	return brainDeclare(entryCallerIdentity(), os.Stdout, os.Stderr, *root, *by, *fixture)
}

// brainDeclare is the declaration owner: the human gate classifies the
// supplied caller identity (owner_invocation.go), and the outcome goes to the
// caller's streams.
func brainDeclare(caller processIdentity, stdout, stderr io.Writer, root, by string, fixture bool) int {
	return brainDeclareWith(caller, stdout, stderr, root, by, fixture, defaultBrainActDependencies())
}

func brainDeclareWith(caller processIdentity, stdout, stderr io.Writer, root, by string, fixture bool, deps brainActDependencies) int {
	if by == "" {
		fmt.Fprintln(stderr, "brain declare needs --by")
		return 2
	}
	ledgerIdentity := deps.ledgerIdentity(root)
	// A declaration of a checkout already declared is a repeat whose effect
	// holds (R-129-ui): success at every authority, and the declaration it
	// keeps. It reads what the unguarded coordinator read shows and changes
	// nothing, so no person's proof is needed to answer it.
	if state := brain.Read(root, ledgerIdentity); ledgerIdentity != "" && state.State == brain.Declared {
		writeJSONLine(stdout, stderr, map[string]any{"state": brain.Declared, "record": state.Record, "unchanged": true,
			"summary": fmt.Sprintf("this checkout is already the coordinator of ledger %s, declared by %s at %s", state.Record.Ledger, state.Record.DeclaredBy, state.Record.DeclaredAt)})
		return 0
	}
	if err := brainHumanAct(caller, root, "declare", fixture, deps.classify); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if ledgerIdentity == "" {
		fmt.Fprintln(stderr, "brain declare needs a migrated ledger identity; run metasystem internal goal migrate first")
		return 2
	}
	machine, err := deps.machine(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	stamp := deps.now().UTC().Format(time.RFC3339)
	if err := brain.ValidateDeclaration(ledgerIdentity, machine, by, stamp); err != nil {
		fmt.Fprintln(stderr, "brain declare refused:", err)
		return 2
	}
	state := brain.Read(root, ledgerIdentity)
	if state.State == brain.Declared {
		fmt.Fprintf(stderr, "this checkout is already the brain of ledger %s, declared by %s at %s; withdraw it first: metasystem internal brain withdraw --root %s --by <name>\n", state.Record.Ledger, state.Record.DeclaredBy, state.Record.DeclaredAt, root)
		return 2
	}
	if state.State == brain.Corrupt {
		fmt.Fprintln(stderr, brain.RemedialRefusal(state.Reason, root))
		return 2
	}
	if obstacles, err := brainDeclarationObstacles(root, machine, deps); err != nil {
		fmt.Fprintln(stderr, "brain declare could not prove quiescence:", err)
		return 2
	} else if len(obstacles) > 0 {
		for _, obstacle := range obstacles {
			fmt.Fprintln(stderr, obstacle)
		}
		return 2
	}
	registryHome, err := deps.registryHome()
	if err != nil {
		fmt.Fprintln(stderr, "brain declare:", err)
		return 2
	}
	record, err := brain.Declare(brain.DeclareOptions{StateRoot: root, RegistryHome: registryHome, LedgerIdentity: ledgerIdentity, Machine: machine, DeclaredBy: by, Now: deps.now().UTC()})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	writeJSONLine(stdout, stderr, map[string]any{"state": brain.Declared, "record": record})
	return 0
}

func brainDeclarationObstacles(root, machine string, deps brainActDependencies) ([]string, error) {
	projection, err := deps.project(root)
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
	scan := deps.scan(root)
	if len(scan.Unreadable) > 0 || len(scan.RunUnreadable) > 0 {
		return nil, fmt.Errorf("scanner inputs are unreadable: %s", strings.Join(append(scan.Unreadable, scan.RunUnreadable...), "; "))
	}
	for _, item := range scan.Busy {
		switch item.Kind {
		case "job":
			obstacles = append(obstacles, fmt.Sprintf("job %s is in flight; stop it first: metasystem work stop j2:%s", item.Id, item.Id))
		case "run":
			obstacles = append(obstacles, fmt.Sprintf("run %s is live; wait for it or conclude it: metasystem internal run watch --root %s --id %s", item.Id, root, item.Id))
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

func runBrainWithdraw(args []string) int {
	flags := flag.NewFlagSet("brain withdraw", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout state root")
	by := flags.String("by", "", "human withdrawing the declaration")
	fixture := flags.Bool("fixture-human-authority", false, "fixture-only authority under an exact fake-runtime root")
	if flags.Parse(args) != nil {
		return 2
	}
	return brainWithdraw(entryCallerIdentity(), os.Stdout, os.Stderr, *root, *by, *fixture)
}

// brainWithdraw is the withdrawal owner under a supplied caller identity.
func brainWithdraw(caller processIdentity, stdout, stderr io.Writer, root, by string, fixture bool) int {
	return brainWithdrawWith(caller, stdout, stderr, root, by, fixture, defaultBrainActDependencies())
}

func brainWithdrawWith(caller processIdentity, stdout, stderr io.Writer, root, by string, fixture bool, deps brainActDependencies) int {
	if by == "" {
		fmt.Fprintln(stderr, "brain withdraw needs --by")
		return 2
	}
	// A withdrawal where nothing is declared is a repeat whose effect holds
	// (R-129-ui): success at every authority, and nothing is touched.
	if brain.Read(root, deps.ledgerIdentity(root)).State == brain.Undeclared {
		writeJSONLine(stdout, stderr, map[string]any{"state": brain.Undeclared, "unchanged": true,
			"summary": "no coordinator is declared for this checkout; there was nothing to withdraw"})
		return 0
	}
	if err := brainHumanAct(caller, root, "withdraw", fixture, deps.classify); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	registryHome, err := deps.registryHome()
	if err != nil {
		fmt.Fprintln(stderr, "brain withdraw:", err)
		return 2
	}
	removed, err := brain.Withdraw(root, registryHome, deps.ledgerIdentity(root))
	if err != nil {
		fmt.Fprintln(stderr, "brain withdraw:", err)
		return 2
	}
	if !removed {
		// Nothing was declared: the withdrawal's effect already holds
		// (R-129-ui).
		writeJSONLine(stdout, stderr, map[string]any{"state": brain.Undeclared, "unchanged": true,
			"summary": "no coordinator is declared for this checkout; there was nothing to withdraw"})
		return 0
	}
	writeJSONLine(stdout, stderr, map[string]any{"state": brain.Undeclared})
	return 0
}

func runBrainBoot(args []string) int {
	return runBrainBootCommand(args)
}
