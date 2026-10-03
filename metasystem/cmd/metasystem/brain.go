package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
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
			// Exact fake-runtime roots may supply their terminal through the
			// existing identity fixture. Grants never answer this classifier.
			if authorization, err := fixtureauth.New(root); err == nil {
				if entry, present := authorization.Identity().FixtureEntry(callerPid); present && entry.HasTerminal && entry.Terminal {
					return lease.ClassifyVerb(root, callerPid)
				}
			}
			return coordinatorDirectCaller(root, callerPid, time.Now(), humanauthority.Prove)
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

// coordinatorDirectCaller never lets the helm or a grant answer a declaration
// change. Its classifier seam represents this direct proof, not person class.
func coordinatorDirectCaller(root string, pid int64, now time.Time, prove func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error)) (lease.ClassifyResult, error) {
	proof, err := prove(root, pid, nil, now)
	if err != nil {
		return lease.ClassifyResult{}, err
	}
	if proof.Helm != nil || !proof.EnrolledTerminalFor(root) {
		return lease.ClassifyResult{}, fmt.Errorf("the enrolled terminal's direct proof is missing")
	}
	return lease.ClassifyResult{Class: lease.ClassHuman}, nil
}

func brainHumanAct(caller ownercall.Process, root, verb string, fixture bool, classify func(string, int64) (lease.ClassifyResult, error)) error {
	if fixture {
		authorization, err := fixtureauth.New(root)
		if err != nil {
			return err
		}
		if authorization.GoalHumanAuthority().Allows(root) {
			return nil
		}
	}
	callerPid, err := caller.ClassifiablePid(identity.KernelProber{})
	if err != nil {
		return fmt.Errorf("coordinator %s: who is running this cannot be told: %w", verb, err)
	}
	classification, err := classify(root, callerPid)
	if err != nil {
		return fmt.Errorf("%s\n%s", coordinatorOwnActRefusal(verb), err)
	}
	if classification.Class != lease.ClassHuman {
		return fmt.Errorf("%s\nbrain %s is a human act; run it from an agent-free terminal", coordinatorOwnActRefusal(verb), verb)
	}
	return nil
}

func coordinatorOwnActRefusal(verb string) string {
	if verb != "withdraw" {
		return "brain declare is a human act; run it from an agent-free terminal"
	}
	return "withdrawing the coordinator declaration is the person's own act, at their own terminal; a grant never stands in for it; nothing was done"
}

// brainDeclare is the declaration owner: the human gate classifies the
// supplied caller identity (owner_invocation.go), and the outcome goes to the
// caller's streams.
func brainDeclare(caller ownercall.Process, stdout, stderr io.Writer, root, by string, fixture bool) int {
	return brainDeclareWith(caller, stdout, stderr, root, by, fixture, defaultBrainActDependencies())
}

func brainDeclareWith(caller ownercall.Process, stdout, stderr io.Writer, root, by string, fixture bool, deps brainActDependencies) int {
	return brainDeclareRoleWith(caller, stdout, stderr, root, by, "", fixture, deps)
}

func brainDeclareRoleWith(caller ownercall.Process, stdout, stderr io.Writer, root, by, role string, fixture bool, deps brainActDependencies) int {
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
		if state.Record.Role != role {
			fmt.Fprintf(stderr, "this checkout already has a coordinator with another role; withdraw it first\nrun: metasystem settings coordinator --withdraw --by %s --repo %s\n", state.Record.DeclaredBy, root)
			return 2
		}
		writeJSONLine(stdout, stderr, map[string]any{"state": brain.Declared, "record": state.Record, "unchanged": true,
			"summary": fmt.Sprintf("this checkout is already the coordinator of ledger %s, declared by %s at %s", state.Record.Ledger, state.Record.DeclaredBy, state.Record.DeclaredAt)})
		return 0
	}
	if err := brainHumanAct(caller, root, "declare", fixture, deps.classify); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if ledgerIdentity == "" {
		fmt.Fprintln(stderr, "this checkout cannot be the coordinator yet: its goal list has not been upgraded\nrun: metasystem goal sync --upgrade --by <your name>")
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
		fmt.Fprintf(stderr, "this checkout is already the coordinator, set by %s at %s\nto undo it, run: metasystem settings coordinator --withdraw --by %s --repo %s\n", state.Record.DeclaredBy, state.Record.DeclaredAt, state.Record.DeclaredBy, root)
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
	record, err := brain.Declare(brain.DeclareOptions{StateRoot: root, Role: role, RegistryHome: registryHome, LedgerIdentity: ledgerIdentity, Machine: machine, DeclaredBy: by, Now: deps.now().UTC()})
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
			obstacles = append(obstacles, fmt.Sprintf("run %s is live; wait for it: metasystem work wait --run %s --exit-code --repo %s", item.Id, item.Id, root))
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

// brainWithdraw is the withdrawal owner under a supplied caller identity.
func brainWithdraw(caller ownercall.Process, stdout, stderr io.Writer, root, by string, fixture bool) int {
	return brainWithdrawWith(caller, stdout, stderr, root, by, fixture, defaultBrainActDependencies())
}

func brainWithdrawWith(caller ownercall.Process, stdout, stderr io.Writer, root, by string, fixture bool, deps brainActDependencies) int {
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
