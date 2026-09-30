package batchowner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"context"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

type BatchJoinRequest struct {
	SeatRoot, LandingRoot, GoalID, ChainID, Through string
	// ChainHead is the commit a certified chain publishes, the head of its
	// candidate branch: the tip the human's word on the chain is bound to,
	// which the batch's publication gate reads again (g1-s70 D2).
	ChainHead string
	Last      bool
	At        time.Time
}

type BatchJoinDependencies struct {
	Binding          func(string, string, time.Time) (dispatchcore.GoalBinding, error)
	Chain            func(string, string, string, uint64) (batch.CertifiedChain, error)
	Base             func(string) (string, error)
	Mint             func() (string, error)
	Transport        func(string, batch.CertifiedChain) error
	member           func(BatchJoinRequest) (batch.BranchMember, []byte, error)
	transportMember  func(BatchJoinRequest, batch.BranchMember) error
	Assemble         func(string, string, []batch.Unit) ([]string, error)
	ProtectedTests   func(string, string, string) error
	AdmissionRun     func(string, string, batch.Unit) (batch.JoinAdmission, error)
	Plan             func(string, string, string) (testpolicy.Plan, error)
	PublishAdmission func(batch.Store, string, batch.Unit, string, time.Time, func(string, string, string) (testpolicy.Plan, error), func() error, batch.JoinAdmissionRun) error
	CostForecast     func(string, batch.Record, batch.Unit, time.Time, func(string, string, string) (testpolicy.Plan, error), func(string, string, []batch.Unit) ([]string, error)) (batch.Unit, batch.CostForecast, error)
	PublishForecast  func(batch.Store, string, batch.Unit, string, time.Time, func(string, string, string) (testpolicy.Plan, error), func() error, batch.JoinAdmissionRun, batch.CostForecast) error
	Handover         func(BatchJoinRequest, string, batch.Claim) error
	Ensure           func(string) error
	Author           func(string, *goal.GoalFile) (string, string, string, error)
	Prober           identity.Prober
	// ReleaseSet selects, in the seat checkout, the goal's workspaces whose
	// work the joined tip contains (disk-lifetimes Part B 3.6): recorded in
	// the member at join, released at P6. Nil records none.
	ReleaseSet func(seatRoot, goalID, tip string) (*diskstore.ReleaseSet, error)
}

var BatchJoinDependenciesForCommand = ProductionBatchJoinDependencies
var BatchJoinClock = fixtureauth.GoalNow
var BatchTreePlanExecutable = os.Executable

func ProductionBatchJoinDependencies() BatchJoinDependencies {
	return BatchJoinDependencies{
		ReleaseSet: SelectMemberReleaseSet,
		Binding:    dispatchcore.ResolveGoalBinding,
		Chain:      batch.ReadCertifiedChain,
		Base:       fetchLandingBaseTree,
		Mint: func() (string, error) {
			id, err := goal.NewOperationULID()
			return strings.ToLower(id), err
		},
		Transport: batch.TransportChain,
		member:    ProductionBatchBranchMember, transportMember: transportBatchBranchMember,
		Assemble:       batch.AssembleUnits,
		ProtectedTests: ProductionBatchProtectedTests,
		AdmissionRun:   productionJoinAdmission,
		Plan:           productionJoinPlan, PublishAdmission: batch.PublishJoinWithAdmission,
		CostForecast: prepareProspectiveBatchCost, PublishForecast: batch.PublishJoinWithAdmissionForecast,
		Handover: productionForwardHandover, Ensure: EnsureBatchOwner, Author: ProductionBatchAuthor, Prober: identity.KernelProber{},
	}
}

func ProductionBatchBranchMember(request BatchJoinRequest) (batch.BranchMember, []byte, error) {
	if _, err := branch.ScrubbedGit(request.SeatRoot, "fetch", "--quiet", "origin", "main"); err != nil {
		return batch.BranchMember{}, nil, err
	}
	endpoint, err := branch.ScrubbedGit(request.SeatRoot, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return batch.BranchMember{}, nil, err
	}
	if _, err := branch.ScrubbedGit(request.SeatRoot, "fetch", "--quiet", "origin", "refs/heads/goal/"+request.GoalID); err != nil {
		return batch.BranchMember{}, nil, err
	}
	tip, err := branch.ScrubbedGit(request.SeatRoot, "rev-parse", "FETCH_HEAD")
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

func transportBatchBranchMember(request BatchJoinRequest, _ batch.BranchMember) error {
	command := exec.Command("git", "-C", request.LandingRoot, "fetch", "--quiet", "origin", "refs/heads/goal/"+request.GoalID)
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("transport goal branch: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func ProductionBatchAuthor(root string, file *goal.GoalFile) (string, string, string, error) {
	if file == nil || file.Approved == nil || !strings.HasPrefix(file.Approved.By, "human:") {
		return "", "", "", fmt.Errorf("%s: goal has no human approver", codeJoinAuthorUnbound)
	}
	approver := strings.TrimPrefix(file.Approved.By, "human:")
	key := "goal.human." + strings.ToLower(approver)
	identity, _, err := config.Get(config.GetParams{Key: key, ConfPath: filepath.Join(root, "metasystem.conf")})
	left, right := strings.LastIndex(identity, "<"), strings.LastIndex(identity, ">")
	if err != nil || left < 1 || right != len(identity)-1 || left >= right-1 {
		return "", "", "", fmt.Errorf("%s: %s has no complete %s identity", codeJoinAuthorUnbound, file.Approved.By, key)
	}
	name, email := strings.TrimSpace(identity[:left]), strings.TrimSpace(identity[left+1:right])
	if name == "" || email == "" || !strings.Contains(email, "@") {
		return "", "", "", fmt.Errorf("%s: %s has no complete %s identity", codeJoinAuthorUnbound, file.Approved.By, key)
	}
	return approver, name, email, nil
}

func ExecuteBatchJoin(request BatchJoinRequest, dependencies BatchJoinDependencies) (batch.Record, error) {
	binding, err := dependencies.Binding(request.SeatRoot, request.GoalID, request.At)
	if err != nil {
		return batch.Record{}, err
	}
	var chain batch.CertifiedChain
	var member batch.BranchMember
	var patch []byte
	if request.Last || request.Through != "" {
		if dependencies.member == nil {
			return batch.Record{}, fmt.Errorf("%s: goal branch reader is unavailable", codeJoinUnread)
		}
		member, patch, err = dependencies.member(request)
	} else {
		chain, err = dependencies.Chain(request.SeatRoot, request.GoalID, request.ChainID, binding.Revision)
		patch = chain.Patch
	}
	if err != nil {
		return batch.Record{}, err
	}
	baseTree, err := dependencies.Base(request.LandingRoot)
	if err != nil {
		return batch.Record{}, err
	}
	id, err := dependencies.Mint()
	if err != nil {
		return batch.Record{}, err
	}
	actor := binding.Machine + "+" + binding.Lineage
	store := batch.NewStore(request.LandingRoot, dependencies.Prober)
	record := batch.Record{BatchID: id, BaseTree: baseTree, TipTree: baseTree, State: batch.StateOpen}
	// A join addresses an open batch (the newest on the current base, else the
	// newest on any base), else a new batch on the current base; new joins open
	// beside batches already sealed or proving, while their proofs run one at a
	// time on the host, each child holding the host's proving flock (U12).
	records, err := store.Records()
	if err != nil {
		return batch.Record{}, err
	}
	open, foundOpen := batch.JoinableOpen(records, baseTree)
	if foundOpen {
		record = open
	}
	if request.Last || request.Through != "" {
		if dependencies.transportMember == nil {
			return batch.Record{}, fmt.Errorf("%s: goal branch transport is unavailable", codeJoinUnread)
		}
		err = dependencies.transportMember(request, member)
	} else {
		err = dependencies.Transport(request.LandingRoot, chain)
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
	if dependencies.Author != nil {
		unit.Approver, unit.AuthorName, unit.AuthorEmail, err = dependencies.Author(request.SeatRoot, binding.File)
		if err != nil {
			return batch.Record{}, err
		}
	}
	if len(member.Builds) != 0 {
		unit = batch.BindBranchMember(unit, member)
	} else {
		// A chain member has no builds; its tip is the commit the chain
		// publishes, which the publication gate binds the human's word to.
		unit.BranchTip = request.ChainHead
	}
	// The goal's workspaces whose work this member lands (its tip), recorded
	// in the member now and released by the batch's P6 step (disk-lifetimes
	// Part B 3.6).
	if dependencies.ReleaseSet != nil && unit.BranchTip != "" {
		if unit.ReleaseSet, err = dependencies.ReleaseSet(request.SeatRoot, request.GoalID, unit.BranchTip); err != nil {
			return batch.Record{}, fmt.Errorf("the goal's workspaces cannot be judged for this landing's release set: %w", err)
		}
	}
	for path := range batch.ChangedPaths(patch) {
		unit.ChangedPaths = append(unit.ChangedPaths, path)
	}
	slices.Sort(unit.ChangedPaths)
	if unit.Claim.AccountingRevision == 0 {
		return batch.Record{}, fmt.Errorf("%s: goal %s has no accounting revision", codeJoinRevisionMoved, request.GoalID)
	}
	// The unit is prepared on the addressed batch's base. When the batch it
	// finally joins has another base (the addressed one was sealed meanwhile and
	// a batch on a newer base took its place), the unit is reassembled there.
	prepared := record.BaseTree
	if err := prepareJoinUnit(request.LandingRoot, prepared, unit, dependencies); err != nil {
		return batch.Record{}, err
	}
	materialize := func() error {
		if record, err = batch.FindOrCreateOpen(store, baseTree, id, actor, request.At); err != nil || record.BaseTree == prepared {
			return err
		}
		prepared = record.BaseTree
		return prepareJoinUnit(request.LandingRoot, prepared, unit, dependencies)
	}
	if foundOpen {
		if err := materialize(); err != nil {
			return batch.Record{}, err
		}
	}
	handover := func() error { return dependencies.Handover(request, record.BatchID, unit.Claim) }
	if dependencies.PublishAdmission == nil || dependencies.AdmissionRun == nil {
		return batch.Record{}, fmt.Errorf("%s: shared admission owner is unavailable", codeJoinAdmissionUnavailable)
	}
	var cost *batch.CostForecast
	if dependencies.CostForecast != nil {
		costNow, nowErr := fixtureauth.GoalNow(batch.ModuleRoot(request.LandingRoot))
		if nowErr != nil {
			return batch.Record{}, nowErr
		}
		var forecast batch.CostForecast
		var forecastErr error
		unit, forecast, forecastErr = dependencies.CostForecast(request.LandingRoot, record, unit, costNow, dependencies.Plan, dependencies.Assemble)
		if forecastErr != nil {
			return batch.Record{}, forecastErr
		}
		cost = &forecast
		if refused := ForecastCostRefusal(forecast); refused != nil {
			if len(record.Units) != 0 {
				if err := batch.CloseAdmissionForCost(store, record.BatchID, actor, costNow, forecast); err != nil {
					return batch.Record{}, err
				}
				if dependencies.Ensure != nil {
					refused = errors.Join(refused, dependencies.Ensure(request.LandingRoot))
				}
			}
			return batch.Record{}, refused
		}
	}
	// A refused first member must leave no empty batch. Once its outside-lock
	// forecast fits, materialize the open record and let the publication CAS
	// reject any intervening membership or base change before handover.
	if !foundOpen {
		if err := materialize(); err != nil {
			return batch.Record{}, err
		}
	}
	runAdmission := func(batchID string, joined batch.Unit) (batch.JoinAdmission, error) {
		return dependencies.AdmissionRun(request.LandingRoot, batchID, joined)
	}
	var publishErr error
	if cost != nil {
		if dependencies.PublishForecast == nil {
			return batch.Record{}, fmt.Errorf("%s: cost-bound publication is unavailable", codeJoinAdmissionUnavailable)
		}
		publishErr = dependencies.PublishForecast(store, record.BatchID, unit, actor, request.At, dependencies.Plan, handover, runAdmission, *cost)
	} else {
		publishErr = dependencies.PublishAdmission(store, record.BatchID, unit, actor, request.At, dependencies.Plan, handover, runAdmission)
	}
	if publishErr != nil {
		// A failed admission may already have handed over the goal and
		// requested its return. The durable owner must still settle custody.
		if dependencies.Ensure != nil {
			publishErr = errors.Join(publishErr, dependencies.Ensure(request.LandingRoot))
		}
		return batch.Record{}, publishErr
	}
	if err := dependencies.Ensure(request.LandingRoot); err != nil {
		return batch.Record{}, err
	}
	return store.Load(record.BatchID)
}

// prepareJoinUnit assembles the unit alone on base and checks that the
// candidate keeps every protected test.
func prepareJoinUnit(root, base string, unit batch.Unit, dependencies BatchJoinDependencies) error {
	prefixes, err := dependencies.Assemble(root, base, []batch.Unit{unit})
	if err != nil || len(prefixes) != 1 {
		return fmt.Errorf("prepare join unit tree: prefixes=%d: %w", len(prefixes), err)
	}
	if dependencies.ProtectedTests == nil {
		return fmt.Errorf("%s: protected test gate is unavailable", codeJoinTestDropped)
	}
	return dependencies.ProtectedTests(root, base, prefixes[0])
}

func fetchLandingBaseTree(root string) (string, error) {
	command := exec.Command("git", "-C", root, "fetch", "--quiet", "origin", "main")
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("fetch landing base: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return (gittree.Workspace{Dir: root}).TreeOf("FETCH_HEAD")
}

// ProductionBatchProtectedTests asks the testing owner to check base-listed
// Go tests before join hands the member to the batch owner. The installed
// contract path and each group's cwd are independent of the repository root.
func ProductionBatchProtectedTests(root, baseTree, candidateTree string) error {
	return ProductionBatchProtectedTestsWithRawSource(root, baseTree, candidateTree, nil)
}

func ProductionBatchProtectedTestsWithRawSource(root, baseTree, candidateTree string, raw func(gittree.RawRequest) gittree.RawResult) error {
	installationRoot := batch.ModuleRoot(root)
	installation := gittree.Workspace{Dir: installationRoot, RawSource: raw}
	projectRoot, err := installation.TopLevel()
	if err != nil {
		return err
	}
	prefix, err := installation.Prefix()
	if err != nil {
		return err
	}
	confPath := filepath.Join(installationRoot, "metasystem.conf")
	contractRel, present, err := config.CommittedLookup(confPath, "testing.contract")
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("testing.contract is required in committed metasystem.conf")
	}
	if filepath.IsAbs(contractRel) || filepath.ToSlash(filepath.Clean(contractRel)) != contractRel || strings.HasPrefix(contractRel, "../") {
		return fmt.Errorf("testing.contract must be a relative normalized path")
	}
	contractPath := filepath.ToSlash(filepath.Join(strings.TrimSuffix(prefix, "/"), contractRel))
	workspace := gittree.Workspace{Dir: projectRoot, RawSource: raw}
	baseConfigPath := filepath.ToSlash(filepath.Join(strings.TrimSuffix(prefix, "/"), "metasystem.conf"))
	baseConfig, present, err := workspace.FileAt(baseTree, baseConfigPath)
	if err != nil {
		return err
	}
	liveConfig, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}
	if !present || !bytes.Equal(baseConfig, liveConfig) {
		return fmt.Errorf("base metasystem.conf differs from the installed configuration")
	}
	data, present, err := workspace.FileAt(baseTree, contractPath)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("base testing contract %s is absent from tree %s", contractPath, baseTree)
	}
	contract, err := testpolicy.Decode(data)
	if err != nil {
		return fmt.Errorf("decode base testing contract: %w", err)
	}
	if err := proofrun.CheckProtectedGoTests(workspace, baseTree, candidateTree, contract); err != nil {
		var missing *proofrun.ProtectedGoTestMissing
		if errors.As(err, &missing) {
			return fmt.Errorf("%s: %w", codeJoinTestDropped, missing)
		}
		return err
	}
	return nil
}

func productionJoinPlan(root, goalID, tree string) (testpolicy.Plan, error) {
	return ProductionBatchTreePlan(root, goalID, tree, testpolicy.ModeAuto)
}

func ProductionBatchTreePlan(root, goalID, tree string, mode testpolicy.Mode) (_ testpolicy.Plan, err error) {
	planned, err := productionBatchTreePlanOutput(root, goalID, tree, mode)
	return planned.Plan, err
}

func productionBatchTreePlanOutput(root, goalID, tree string, mode testpolicy.Mode) (_ testrun.PlanOutput, err error) {
	return productionBatchTreePlanOutputWithGroups(root, goalID, tree, mode, nil)
}

func productionBatchTreePlanOutputWithGroups(root, goalID, tree string, mode testpolicy.Mode, groups []string) (_ testrun.PlanOutput, err error) {
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(tree)
	if err != nil {
		return testrun.PlanOutput{}, fmt.Errorf("plan batch tree: %w", err)
	}
	defer func() { err = errors.Join(err, detached.Close()) }()
	planningRoot := detached.Workspace().Dir
	binary, err := BatchTreePlanExecutable()
	if err != nil {
		return testrun.PlanOutput{}, err
	}
	command := BatchTreePlanCommand(binary, planningRoot, goalID, tree, mode)
	if len(groups) != 0 {
		command.Args = append(command.Args, "--batch-prefix", "--batch-requirements", testrun.BatchRequirementsArgument(groups))
	}
	verb := testPlanVerb
	if lane.IsAccount(goalID) {
		verb = laneTestPlanVerb
	}
	child, err := verbresult.Run(command, verb)
	if err != nil {
		return testrun.PlanOutput{}, fmt.Errorf("plan joined unit: %w", err)
	}
	if child.Outcome != verbresult.Confirmed {
		// The error keeps the child's code, which a caller matches with
		// errors.As (laneHold), never in these words.
		return testrun.PlanOutput{}, fmt.Errorf("plan joined unit: %w", child.Err())
	}
	var planned testrun.PlanOutput
	if err := child.DecodeData(&planned); err != nil {
		return testrun.PlanOutput{}, fmt.Errorf("plan joined unit: the plan could not be read: %w", err)
	}
	return planned, nil
}

func BatchTreePlanCommand(binary, planningRoot, goalID, tree string, mode testpolicy.Mode) *exec.Cmd {
	controlRoot := batch.ModuleRoot(planningRoot)
	account := []string{"test", "plan", "--root", controlRoot, "--goal", goalID}
	if lane.IsAccount(goalID) {
		// A batch of changes is planned on the lane's account (U11b).
		account = []string{"internal", "test", "plan", "--root", controlRoot, "--lane", goalID}
	}
	command := exec.Command(binary, append(account, "--tree", tree, "--mode", string(mode), "--purpose", "delivery", "--json")...)
	command.Dir, command.Env = controlRoot, gittree.ScrubbedEnviron()
	return command
}

func productionForwardHandover(request BatchJoinRequest, batchID string, source batch.Claim) error {
	holder, err := lease.CurrentHolder(request.LandingRoot)
	if err != nil {
		return fmt.Errorf("the landing lane does not hold its checkout: %w", err)
	}
	if holder.OwnerLineage != LandingOwnerLineage || holder.ClaimEpoch < 1 {
		return fmt.Errorf("the landing lane does not hold its checkout (held by session %s, claim %d)", holder.OwnerLineage, holder.ClaimEpoch)
	}
	machine, err := goal.ResolveMachine(request.LandingRoot)
	if err != nil {
		return err
	}
	// The joining seat's own process is the supplied identity, as the
	// handover child's parent was, and the request carries the seat's lineage.
	return BatchOwnerCalls.Handover(ownercall.FromThisProcess(source.Lineage), ownercall.HandoverRequest{Root: request.SeatRoot,
		GoalID: request.GoalID, TargetMachine: machine, TargetLineage: LandingOwnerLineage,
		TargetEpoch: holder.ClaimEpoch, Batch: batchID})
}

// SelectMemberReleaseSet selects, in the member's seat checkout, the goal's
// accepted copies whose every tip the joined tip contains and whose tree is
// clean: the release set the batch's P6 step runs (disk-lifetimes Part B
// 3.6, Round B3 ruling). It reads only.
func SelectMemberReleaseSet(seatRoot, goalID, tip string) (*diskstore.ReleaseSet, error) {
	layout, err := stateroot.ResolveLayout(seatRoot)
	if err != nil {
		return nil, err
	}
	set, err := diskstore.SelectReleaseSet(context.Background(), diskstore.CheckoutRegistry(steward.StoreControl(layout.InstallationRoot)), layout.GitRoot, goalID, tip, steward.ExecWorkspaceGit)
	if err != nil {
		return nil, err
	}
	return &set, nil
}

// ReleaseMemberSet releases a landed member's recorded set in its seat
// checkout through diskstore.ReleaseWorkspace, each workspace's tips
// archived first and judged against the landed tip again; an unreadable
// seat leaves every entry pending for the next recovery.
func ReleaseMemberSet(batchID string, at time.Time) func(batch.Unit, *diskstore.ReleaseSet) {
	return func(unit batch.Unit, set *diskstore.ReleaseSet) {
		layout, err := stateroot.ResolveLayout(unit.SeatRoot)
		if err != nil {
			return
		}
		request := diskstore.WorkspaceReleaseRequest{Registry: diskstore.CheckoutRegistry(steward.StoreControl(layout.InstallationRoot)), GitRoot: layout.GitRoot,
			Git: steward.ExecWorkspaceGit, By: "landing batch " + batchID, Now: at.UTC(),
			TakeCensus: func() *diskstore.UseCensus {
				home, _ := steward.HomeStateRoot()
				census := diskstore.TakeUseCensus(context.Background(), *steward.KernelCensusReader(home, append(steward.ArmedCheckouts(), unit.SeatRoot)))
				return &census
			}}
		diskstore.RunReleaseSet(context.Background(), request, set)
	}
}

// The steward's checkout pass retries its own seat's batch members'
// unfinished release sets (Round D3 N6).
func init() {
	steward.RegisterBatchReleaseRetry(steward.BatchReleaseRetry{Unfinished: UnfinishedSeatReleaseSets, Retry: RetrySeatReleaseSets})
}

// UnfinishedSeatReleaseSets lists the batches in lane holding a landed
// member of seat whose release set is unfinished. It reads only; an absent
// lane has none.
func UnfinishedSeatReleaseSets(lane, seat string) ([]string, error) {
	records, err := batch.NewStore(lane, nil).Records()
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, record := range records {
		for _, unit := range record.Units {
			if unit.P6Done && unit.ReleaseSet != nil && !unit.ReleaseSet.Finished() && sameSeat(unit.SeatRoot, seat) {
				ids = append(ids, record.BatchID)
				break
			}
		}
	}
	return ids, nil
}

// RetrySeatReleaseSets retries seat's landed members' unfinished sets in
// batch id, leaving every other seat's members to their own checkout.
func RetrySeatReleaseSets(_ context.Context, lane, seat, id string, at time.Time) error {
	release := ReleaseMemberSet(id, at)
	return batch.RetryReleaseSets(batch.NewStore(lane, nil), id, func(unit batch.Unit, set *diskstore.ReleaseSet) {
		if sameSeat(unit.SeatRoot, seat) {
			release(unit, set)
		}
	})
}

// sameSeat compares two installation paths by their resolved spelling.
func sameSeat(left, right string) bool {
	resolve := func(path string) string {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			return resolved
		}
		return filepath.Clean(path)
	}
	return left != "" && resolve(left) == resolve(right)
}
