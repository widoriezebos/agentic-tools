package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type batchJoinRequest struct {
	SeatRoot, LandingRoot, GoalID, ChainID, Through string
	Last                                            bool
	At                                              time.Time
}

type batchJoinDependencies struct {
	binding         func(string, string, time.Time) (dispatchcore.GoalBinding, error)
	chain           func(string, string, string, uint64) (batch.CertifiedChain, error)
	base            func(string) (string, error)
	mint            func() (string, error)
	transport       func(string, batch.CertifiedChain) error
	member          func(batchJoinRequest) (batch.BranchMember, []byte, error)
	transportMember func(batchJoinRequest, batch.BranchMember) error
	assemble        func(string, string, []batch.Unit) ([]string, error)
	fixtures        func(string) ([]byte, error)
	protectedTests  func(string, string, string) error
	gate            func(string, string, []byte, []byte, *batch.Unit) error
	plan            func(string, string, string) (testpolicy.Plan, error)
	publish         func(batch.Store, string, batch.Unit, string, time.Time, func(string, string, string) (testpolicy.Plan, error), func() error) error
	handover        func(batchJoinRequest, string, batch.Claim) error
	ensure          func(string) error
	author          func(string, *goal.GoalFile) (string, string, string, error)
	prober          identity.Prober
}

var batchJoinDependenciesForCommand = productionBatchJoinDependencies
var batchJoinClock = goalCommandNow

func productionBatchJoinDependencies() batchJoinDependencies {
	return batchJoinDependencies{
		binding: dispatchcore.ResolveGoalBinding,
		chain:   batch.ReadCertifiedChain,
		base:    fetchLandingBaseTree,
		mint: func() (string, error) {
			id, err := goal.NewOperationULID()
			return strings.ToLower(id), err
		},
		transport: batch.TransportChain,
		member:    productionBatchBranchMember, transportMember: transportBatchBranchMember,
		assemble: batch.AssembleUnits,
		fixtures: func(root string) ([]byte, error) {
			return os.ReadFile(filepath.Join(root, "scripts", "agents", "fixture-bed-groups.tsv"))
		},
		protectedTests: batch.CheckProtectedTests,
		gate: func(root, tree string, patch, fixtures []byte, unit *batch.Unit) error {
			execute := productionJoinGate(root)
			return batch.RunJoinGateAt(root, tree, patch, fixtures, unit, execute)
		},
		plan: productionJoinPlan, publish: batch.PublishJoin,
		handover: productionForwardHandover, ensure: ensureBatchOwner, author: productionBatchAuthor, prober: identity.KernelProber{},
	}
}

func productionBatchBranchMember(request batchJoinRequest) (batch.BranchMember, []byte, error) {
	if _, err := goalBranchGit(request.SeatRoot, "fetch", "--quiet", "origin", "main"); err != nil {
		return batch.BranchMember{}, nil, err
	}
	endpoint, err := goalBranchGit(request.SeatRoot, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return batch.BranchMember{}, nil, err
	}
	if _, err := goalBranchGit(request.SeatRoot, "fetch", "--quiet", "origin", "refs/heads/goal/"+request.GoalID); err != nil {
		return batch.BranchMember{}, nil, err
	}
	tip, err := goalBranchGit(request.SeatRoot, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return batch.BranchMember{}, nil, err
	}
	member, err := batch.ReadGoalBranch(batch.BranchReadRequest{Repo: request.SeatRoot, EndpointTip: endpoint, BranchTip: tip, GoalID: request.GoalID, Through: request.Through, Last: request.Last})
	if err != nil {
		return batch.BranchMember{}, nil, err
	}
	patch, err := batch.BranchMemberPatch(request.SeatRoot, member)
	return member, patch, err
}

func transportBatchBranchMember(request batchJoinRequest, _ batch.BranchMember) error {
	command := exec.Command("git", "-C", request.LandingRoot, "fetch", "--quiet", "origin", "refs/heads/goal/"+request.GoalID)
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("transport goal branch: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func productionBatchAuthor(root string, file *goal.GoalFile) (string, string, string, error) {
	if file == nil || file.Approved == nil || !strings.HasPrefix(file.Approved.By, "human:") {
		return "", "", "", fmt.Errorf("BATCH_JOIN_AUTHOR_UNBOUND: goal has no human approver")
	}
	approver := strings.TrimPrefix(file.Approved.By, "human:")
	key := "goal.human." + strings.ToLower(approver)
	identity, _, err := config.Get(config.GetParams{Key: key, ConfPath: filepath.Join(root, "metasystem.conf")})
	left, right := strings.LastIndex(identity, "<"), strings.LastIndex(identity, ">")
	if err != nil || left < 1 || right != len(identity)-1 || left >= right-1 {
		return "", "", "", fmt.Errorf("BATCH_JOIN_AUTHOR_UNBOUND: %s has no complete %s identity", file.Approved.By, key)
	}
	name, email := strings.TrimSpace(identity[:left]), strings.TrimSpace(identity[left+1:right])
	if name == "" || email == "" || !strings.Contains(email, "@") {
		return "", "", "", fmt.Errorf("BATCH_JOIN_AUTHOR_UNBOUND: %s has no complete %s identity", file.Approved.By, key)
	}
	return approver, name, email, nil
}

func executeBatchJoin(request batchJoinRequest, dependencies batchJoinDependencies) (batch.Record, error) {
	binding, err := dependencies.binding(request.SeatRoot, request.GoalID, request.At)
	if err != nil {
		return batch.Record{}, err
	}
	var chain batch.CertifiedChain
	var member batch.BranchMember
	var patch []byte
	if request.Last || request.Through != "" {
		if dependencies.member == nil {
			return batch.Record{}, fmt.Errorf("BATCH_JOIN_UNREAD: goal branch reader is unavailable")
		}
		member, patch, err = dependencies.member(request)
	} else {
		chain, err = dependencies.chain(request.SeatRoot, request.GoalID, request.ChainID, binding.Revision)
		patch = chain.Patch
	}
	if err != nil {
		return batch.Record{}, err
	}
	baseTree, err := dependencies.base(request.LandingRoot)
	if err != nil {
		return batch.Record{}, err
	}
	id, err := dependencies.mint()
	if err != nil {
		return batch.Record{}, err
	}
	actor := binding.Machine + "+" + binding.Lineage
	store := batch.NewStore(request.LandingRoot, dependencies.prober)
	record := batch.Record{BatchID: id, BaseTree: baseTree, TipTree: baseTree, State: batch.StateOpen}
	records, err := store.Records()
	if err != nil {
		return batch.Record{}, err
	}
	foundOpen := false
	for _, candidate := range records {
		if candidate.State == batch.StateOpen && candidate.ClosedReason == "" {
			if foundOpen {
				return batch.Record{}, fmt.Errorf("more than one open landing batch exists")
			}
			record, foundOpen = candidate, true
		}
	}
	if request.Last || request.Through != "" {
		if dependencies.transportMember == nil {
			return batch.Record{}, fmt.Errorf("BATCH_JOIN_UNREAD: goal branch transport is unavailable")
		}
		err = dependencies.transportMember(request, member)
	} else {
		err = dependencies.transport(request.LandingRoot, chain)
	}
	if err != nil {
		return batch.Record{}, err
	}
	chainID := request.ChainID
	if chainID == "" {
		chainID = member.Tip
	}
	unit := batch.Unit{GoalID: request.GoalID, Chain: chainID, SeatRoot: request.SeatRoot, State: batch.UnitJoining,
		Claim: batch.Claim{Machine: binding.Machine, Lineage: binding.Lineage, Epoch: uint64(binding.Capability.ClaimEpoch),
			Revision: binding.Revision, AccountingRevision: binding.File.Claimed.AccountingRevision}}
	if dependencies.author != nil {
		unit.Approver, unit.AuthorName, unit.AuthorEmail, err = dependencies.author(request.SeatRoot, binding.File)
		if err != nil {
			return batch.Record{}, err
		}
	}
	if len(member.Builds) != 0 {
		unit = batch.BindBranchMember(unit, member)
	}
	for path := range batch.ChangedPaths(patch) {
		unit.ChangedPaths = append(unit.ChangedPaths, path)
	}
	slices.Sort(unit.ChangedPaths)
	if unit.Claim.AccountingRevision == 0 {
		return batch.Record{}, fmt.Errorf("BATCH_JOIN_REVISION_MOVED: goal %s has no accounting revision", request.GoalID)
	}
	prefixes, err := dependencies.assemble(request.LandingRoot, record.BaseTree, []batch.Unit{unit})
	if err != nil || len(prefixes) != 1 {
		return batch.Record{}, fmt.Errorf("prepare join unit tree: prefixes=%d: %w", len(prefixes), err)
	}
	fixtureMap, err := dependencies.fixtures(request.SeatRoot)
	if err != nil {
		return batch.Record{}, err
	}
	if err := dependencies.gate(request.LandingRoot, prefixes[0], patch, fixtureMap, &unit); err != nil {
		return batch.Record{}, err
	}
	if dependencies.protectedTests == nil {
		return batch.Record{}, fmt.Errorf("BATCH_JOIN_TEST_DROPPED: protected test gate is unavailable")
	}
	if err := dependencies.protectedTests(request.LandingRoot, record.BaseTree, prefixes[0]); err != nil {
		return batch.Record{}, err
	}
	// The candidate was prepared on the open batch's own base, which trunk may have
	// passed since; only a batch that changed under the preparation refuses.
	prepared := record
	record, err = batch.FindOrCreateOpen(store, baseTree, id, actor, request.At)
	if err != nil {
		return batch.Record{}, err
	}
	if record.BaseTree != prepared.BaseTree {
		return batch.Record{}, fmt.Errorf("BATCH_JOIN_BASE_MOVED: open batch base changed during preparation")
	}
	handover := func() error { return dependencies.handover(request, record.BatchID, unit.Claim) }
	if err := dependencies.publish(store, record.BatchID, unit, actor, request.At, dependencies.plan, handover); err != nil {
		return batch.Record{}, err
	}
	if err := dependencies.ensure(request.LandingRoot); err != nil {
		return batch.Record{}, err
	}
	return store.Load(record.BatchID)
}

func fetchLandingBaseTree(root string) (string, error) {
	command := exec.Command("git", "-C", root, "fetch", "--quiet", "origin", "main")
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("fetch landing base: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return (gittree.Workspace{Dir: root}).TreeOf("FETCH_HEAD")
}

func productionJoinGate(root string) batch.GateExecutor {
	return func(tree string, step batch.GateStep) batch.GateStepResult {
		detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(tree)
		if err != nil {
			return batch.GateStepResult{ExitCode: 1}
		}
		defer detached.Close()
		args := append([]string(nil), step.Args...)
		if len(args) == 0 {
			return batch.GateStepResult{ExitCode: 1}
		}
		if args[0] == "bin/metasystem" {
			binary, executableErr := os.Executable()
			if executableErr != nil {
				return batch.GateStepResult{ExitCode: 1}
			}
			args[0] = binary
		}
		command := exec.Command(args[0], args[1:]...)
		command.Dir, command.Env = batchModuleRoot(detached.Workspace().Dir), gittree.ScrubbedEnviron()
		output, commandErr := command.CombinedOutput()
		if commandErr != nil {
			fmt.Fprintf(os.Stderr, "landing batch join gate %s: %s\n", step.Name, strings.TrimSpace(string(output)))
			code := 1
			if command.ProcessState != nil {
				code = command.ProcessState.ExitCode()
			}
			return joinGateFailure(code, output)
		}
		digest := sha256.Sum256([]byte(tree + "\x00" + step.Name + "\x00" + strings.Join(step.Args, "\x00")))
		return batch.GateStepResult{RunID: "join-" + hex.EncodeToString(digest[:8])}
	}
}

func joinGateFailure(exitCode int, output []byte) batch.GateStepResult {
	return batch.GateStepResult{ExitCode: exitCode, Detail: strings.TrimSpace(string(output))}
}

func batchModuleRoot(checkout string) string {
	nested := filepath.Join(checkout, "metasystem")
	if info, err := os.Stat(filepath.Join(nested, "go.mod")); err == nil && !info.IsDir() {
		return nested
	}
	return checkout
}

func productionJoinPlan(root, goalID, tree string) (testpolicy.Plan, error) {
	return productionBatchTreePlan(root, goalID, tree, testpolicy.ModeAuto)
}

func productionBatchTreePlan(root, goalID, tree string, mode testpolicy.Mode) (_ testpolicy.Plan, err error) {
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(tree)
	if err != nil {
		return testpolicy.Plan{}, fmt.Errorf("plan batch tree: %w", err)
	}
	defer func() { err = errors.Join(err, detached.Close()) }()
	planningRoot := detached.Workspace().Dir
	binary, err := os.Executable()
	if err != nil {
		return testpolicy.Plan{}, err
	}
	command := exec.Command(binary, "test", "plan", "--root", planningRoot, "--goal", goalID, "--tree", tree, "--mode", string(mode), "--purpose", "delivery", "--json")
	command.Dir, command.Env = planningRoot, gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		return testpolicy.Plan{}, fmt.Errorf("plan joined unit: %s: %w", strings.TrimSpace(string(output)), err)
	}
	var planned testingPlanOutput
	if err := json.Unmarshal(output, &planned); err != nil {
		return testpolicy.Plan{}, err
	}
	return planned.Plan, nil
}

func productionForwardHandover(request batchJoinRequest, batchID string, source batch.Claim) error {
	holder, err := lease.CurrentHolder(request.LandingRoot)
	if err != nil {
		return fmt.Errorf("landing owner is not a proven holder: %w", err)
	}
	if holder.OwnerLineage != landingOwnerLineage || holder.ClaimEpoch < 1 {
		return fmt.Errorf("landing owner is not a proven holder: lineage=%s epoch=%d", holder.OwnerLineage, holder.ClaimEpoch)
	}
	machine, err := goal.ResolveMachine(request.LandingRoot)
	if err != nil {
		return err
	}
	return batchChildRunner(request.SeatRoot, source.Lineage, "goal", "handover", "--root", request.SeatRoot,
		"--id", request.GoalID, "--lineage", source.Lineage, "--target-machine", machine,
		"--target-lineage", landingOwnerLineage, "--target-claim-epoch", fmt.Sprint(holder.ClaimEpoch), "--batch", batchID)
}

func runBatchJoin(args []string) int {
	flags := flag.NewFlagSet("landing batch join", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	goalID := flags.String("goal", "", "claimed goal id")
	chainID := flags.String("chain", "", "closed implementation chain root")
	last := flags.Bool("last", false, "join the whole land-ready goal")
	through := flags.String("through", "", "join through one land-ready goal commit")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *goalID == "" || ((*chainID != "") == (*last || *through != "")) || (*last && *through != "") {
		fmt.Fprintln(os.Stderr, "usage: metasystem landing batch join --root ROOT --goal GOAL (--chain CHAIN | --last | --through COMMIT)")
		return 2
	}
	now, err := batchJoinClock(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	settings, err := config.ResolveBatchLanding(filepath.Join(*root, "metasystem.conf"), *root, func() time.Time { return now })
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	record, err := executeBatchJoin(batchJoinRequest{SeatRoot: *root, LandingRoot: settings.Root, GoalID: *goalID, ChainID: *chainID, Last: *last, Through: *through, At: now}, batchJoinDependenciesForCommand())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(map[string]any{"batchId": record.BatchID, "goalId": *goalID, "state": batch.UnitJoined})
	return 0
}
