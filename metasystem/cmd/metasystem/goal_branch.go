package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func goalBranchGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = gittree.ScrubbedEnviron()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runGoalBranch(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "goal branch needs check, status, read, commit, push, land-prep, land-push, verify, or sweep")
		return 2
	}
	switch args[0] {
	case "check":
		return runGoalBranchCheck(args[1:])
	case "status":
		return runGoalBranchStatus(args[1:])
	case "read":
		return runGoalBranchRead(args[1:])
	case "commit":
		return runGoalBranchCommit(args[1:])
	case "push":
		return runGoalBranchPush(args[1:])
	case "land-prep":
		return runGoalBranchLandPrep(args[1:])
	case "land-push":
		return runGoalBranchLandPush(args[1:])
	case "verify":
		return runGoalBranchVerify(args[1:])
	case "sweep":
		return runGoalBranchSweep(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "goal branch needs check, status, read, commit, push, land-prep, land-push, verify, or sweep")
		return 2
	}
}

type goalBranchReadDependencies struct {
	Binary   string
	Gate     func(string) (string, error)
	Delegate func(string, string, string, string, string) (string, error)
	Commit   func(branch.CommitReadRequest) (string, branch.Attestation, error)
	Raw      *goalBranchRawDependencies
}

type goalBranchRawDependencies struct {
	Config         func(string, string) (string, error)
	GoalRepository goal.Repository
	EndpointTip    func(string, goal.Endpoint) (string, error)
	OriginTip      func(string, goal.Endpoint, string) (string, bool, error)
	ResolveCommit  func(string, string) (string, error)
	HolderRoot     func(string) string
	Linked         func(string) bool
	HeadRef        func(string) ([]byte, error)
	ReadRepository branch.BranchReadRepository
	ReadInputs     *branch.ReadCommitInputs
	Transport      branch.PushTransport
}

func (d *goalBranchRawDependencies) endpoint(root string) (goal.Endpoint, error) {
	if d == nil {
		return goalBranchEndpoint(root)
	}
	if d.Config == nil || d.GoalRepository == nil || d.EndpointTip == nil || d.OriginTip == nil || d.ResolveCommit == nil || d.HolderRoot == nil || d.Linked == nil || d.ReadRepository == nil || d.ReadInputs == nil || d.Transport == nil {
		return goal.Endpoint{}, fmt.Errorf("goal branch raw inputs are incomplete")
	}
	endpoint, err := goal.ResolveEndpointWithConfig(root, d.Config)
	if err != nil {
		return endpoint, err
	}
	if endpoint.Branch != "refs/heads/main" {
		return goal.Endpoint{}, fmt.Errorf("GOAL_BRANCH_ENDPOINT_UNSUPPORTED: endpoint %s is not refs/heads/main", endpoint.Branch)
	}
	endpoint.Repository = d.GoalRepository
	return endpoint, nil
}

type goalBranchReadOption struct {
	name, value string
	seen        bool
}

func (option *goalBranchReadOption) String() string { return option.value }

func (option *goalBranchReadOption) Set(value string) error {
	if option.seen {
		return fmt.Errorf("goal branch read accepts --%s only once", option.name)
	}
	if value == "" {
		return fmt.Errorf("goal branch read --%s needs a nonempty value", option.name)
	}
	option.value, option.seen = value, true
	return nil
}

func runGoalBranchRead(args []string) int {
	binary, _ := os.Executable()
	return runGoalBranchReadWith(args, goalBranchReadDependencies{Binary: binary, Commit: branch.CommitRead})
}

func lastOutputLine(output []byte) string {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[len(lines)-1])
}

func readGate(worktree string) (string, error) {
	proof, err := os.CreateTemp("", "goal-branch-read-gate-*")
	if err != nil {
		return "", err
	}
	proofPath := proof.Name()
	proof.Close()
	defer os.Remove(proofPath)
	command := exec.Command("bash", filepath.Join(worktree, "scripts", "agents", "go-gate.sh"), "--fast", "--proof-out", proofPath)
	command.Dir = worktree
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	return lastOutputLine(output), err
}

type readDelegateOutcomeError struct {
	Outcome delegateOutcome
	Cause   error
}

func (failure *readDelegateOutcomeError) Error() string {
	if failure.Cause != nil {
		return fmt.Sprintf("delegate outcome %s: %s: %v", failure.Outcome.Outcome, failure.Outcome.Detail, failure.Cause)
	}
	return fmt.Sprintf("delegate outcome %s: %s", failure.Outcome.Outcome, failure.Outcome.Detail)
}

func (failure *readDelegateOutcomeError) Unwrap() error { return failure.Cause }

func readDelegateNeverLaunched(outcome delegateOutcome) bool {
	if outcome.JobID != "" {
		return false
	}
	switch outcome.Outcome {
	case "REFUSED-REQUEST", "BRAIN_REFUSED", "REFUSED-ROSTER":
		return true
	default:
		return false
	}
}

func readDelegate(binary, root, brief, goalID, commit, runtime, model string, environment ...string) (string, error) {
	if binary == "" {
		return "", fmt.Errorf("delegate binary is unavailable")
	}
	args := []string{"delegate", "--role", "code-critic", "--reviews", "commit:" + commit,
		"--goal", goalID, "--brief", brief, "--destructive-reach", "DESIGN-BEARING"}
	if runtime != "" {
		args = append(args, "--runtime", runtime)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	command := exec.Command(binary, args...)
	command.Env = append(append(os.Environ(), environment...), "METASYSTEM_DELEGATE_ROOT="+root)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	var outcome delegateOutcome
	if jsonErr := json.Unmarshal(bytes.TrimSpace(output), &outcome); jsonErr == nil && outcome.Outcome != "" {
		if err == nil && outcome.Outcome == "WON" && outcome.JobID != "" {
			return outcome.JobID, nil
		}
		failure := &readDelegateOutcomeError{Outcome: outcome, Cause: err}
		if err != nil && readDelegateNeverLaunched(outcome) {
			return "", &branch.ReadNeverLaunchedError{Err: failure}
		}
		return "", failure
	}
	if err != nil {
		return "", fmt.Errorf("delegate: %s: %w", lastOutputLine(stderr.Bytes()), err)
	}
	return "", fmt.Errorf("delegate returned no started job: %s", lastOutputLine(output))
}

func runGoalBranchReadWith(args []string, dependencies goalBranchReadDependencies) int {
	result, code, err := goalBranchReadRun(args, dependencies)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return code
	}
	fmt.Printf("state=%s root-job=%s gate-run=%s", result.State, result.RootJob, result.GateRunID)
	if result.AttestationCommit != "" {
		fmt.Printf(" attestation=%s", result.AttestationCommit)
	}
	fmt.Println()
	return 0
}

// goalBranchReadRun is the branch read owner with its typed result; the
// exit code accompanies any error.
func goalBranchReadRun(args []string, dependencies goalBranchReadDependencies) (branch.BranchReadResult, int, error) {
	flags := flag.NewFlagSet("goal branch read", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	unit := flags.String("unit", "", "Goal-Unit commit")
	brief, runtime, model := &goalBranchReadOption{name: "brief"}, &goalBranchReadOption{name: "runtime"}, &goalBranchReadOption{name: "model"}
	flags.Var(brief, "brief", "accepted implementation brief to freeze into the critic dispatch")
	flags.Var(runtime, "runtime", "requested critic runtime (subject to roster authorization)")
	flags.Var(model, "model", "requested critic model (subject to roster authorization)")
	collect := flags.Bool("collect", false, "collect a closed critic root into an attestation")
	selected := flags.String("selected-installation", "", "installation whose configured code-critic roster the critic dispatch resolves (a generated goal worktree's selected installation)")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *goalID == "" || *unit == "" {
		return branch.BranchReadResult{}, 2, fmt.Errorf("goal branch read needs --goal and --unit")
	}
	endpoint, err := dependencies.Raw.endpoint(*root)
	if err != nil {
		return branch.BranchReadResult{}, 1, err
	}
	endpointTipReader := goalBranchEndpointTip
	originTipReader := goalBranchOriginTip
	resolveCommit := func(root, unit string) (string, error) {
		return goalBranchGit(root, "rev-parse", "--verify", unit+"^{commit}")
	}
	holderRoot := goalBranchHolderRoot
	config := func(root, key string) (string, error) { return goal.ResolveMachine(root) }
	if dependencies.Raw != nil {
		endpointTipReader, originTipReader, resolveCommit = dependencies.Raw.EndpointTip, dependencies.Raw.OriginTip, dependencies.Raw.ResolveCommit
		holderRoot, config = dependencies.Raw.HolderRoot, dependencies.Raw.Config
	}
	endpointTip, err := endpointTipReader(*root, endpoint)
	if err != nil {
		return branch.BranchReadResult{}, 1, err
	}
	branchTip, present, err := originTipReader(*root, endpoint, *goalID)
	if err != nil || !present {
		if err == nil {
			err = fmt.Errorf("origin has no goal/%s", *goalID)
		}
		return branch.BranchReadResult{}, 1, err
	}
	commit, err := resolveCommit(*root, *unit)
	if err != nil {
		return branch.BranchReadResult{}, 1, err
	}
	gate := dependencies.Gate
	if gate == nil {
		gate = readGate
	}
	delegate := dependencies.Delegate
	if delegate == nil {
		delegate = func(brief, goalID, commit, runtime, model string) (string, error) {
			environment, err := criticDelegateEnvironment(*selected, *root, brief)
			if err != nil {
				return "", &branch.ReadNeverLaunchedError{Err: err}
			}
			return readDelegate(dependencies.Binary, *root, brief, goalID, commit, runtime, model, environment...)
		}
	}
	var readRepository branch.BranchReadRepository
	commitRead := dependencies.Commit
	if dependencies.Raw != nil {
		readRepository = dependencies.Raw.ReadRepository
		commitRead = func(request branch.CommitReadRequest) (string, branch.Attestation, error) {
			request.Inputs, request.GateRepository, request.Transport = dependencies.Raw.ReadInputs, readRepository, dependencies.Raw.Transport
			return branch.CommitRead(request)
		}
	}
	result, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: *root, Remote: endpoint.Remote,
		EndpointTip: endpointTip, BranchTip: branchTip, GoalID: *goalID, UnitCommit: commit, Collect: *collect,
		BriefPath: brief.value, Runtime: runtime.value, Model: model.value,
		CheckClaim: goalBranchClaimCheckWith(*root, *goalID, endpoint, config, holderRoot), Gate: gate, Delegate: delegate, Commit: commitRead, Repository: readRepository})
	if err != nil {
		return branch.BranchReadResult{}, 1, err
	}
	return result, 0, nil
}

func runGoalBranchLandPush(args []string) int {
	result, endpoint, code, err := goalBranchLandPushRun(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return code
	}
	fmt.Printf("landed %s endpoint=%s branch-deleted=%s\n", result.Landing, endpoint, result.Branch)
	return 0
}

// goalBranchLandPushRun pushes one prepared landing and sweeps a goal's last
// landing, returning the pushed landing and the endpoint branch it moved.
func goalBranchLandPushRun(args []string) (branch.PreparedLanding, string, int, error) {
	flags := flag.NewFlagSet("goal branch land-push", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	prepared := flags.String("prepared", "", "land-prep artifact directory")
	if flags.Parse(args) != nil || *goalID == "" || *prepared == "" || flags.NArg() != 0 {
		return branch.PreparedLanding{}, "", 2, fmt.Errorf("goal branch land-push needs --goal and --prepared")
	}
	endpoint, err := goalBranchEndpoint(*root)
	if err != nil {
		return branch.PreparedLanding{}, "", 1, err
	}
	result, err := branch.LandPush(branch.LandPushRequest{Repo: *root, Remote: endpoint.Remote, EndpointRef: endpoint.Branch,
		GoalID: *goalID, Prepared: *prepared, CheckClaim: goalBranchClaimCheck(*root, *goalID, endpoint)})
	if err != nil {
		return branch.PreparedLanding{}, "", 1, err
	}
	if branch.IsLastLanding(*root, result.Landing, *goalID) {
		if err := goalBranchSweepLandedAt(*root, *goalID, result.Landing, endpoint); err != nil {
			return result, endpoint.Branch, 1, err
		}
	}
	return result, endpoint.Branch, 0, nil
}

func runGoalBranchSweep(args []string) int {
	flags := flag.NewFlagSet("goal branch sweep", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id to delete")
	abandoned := flags.Bool("abandoned", false, "the word to delete abandoned goal work or an orphan landing")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *abandoned && *goalID == "" {
		fmt.Fprintln(os.Stderr, "goal branch sweep takes optional --goal and --abandoned")
		return 2
	}
	endpoint, err := goalBranchEndpoint(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	endpointTip, err := goalBranchEndpointTip(*root, endpoint)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	projection, err := goal.Project(endpoint, true, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *goalID == "" {
		return listGoalBranchSweep(*root, endpoint, projection.Tree)
	}
	if _, err := lease.RequireHolder(goalBranchHolderRoot(*root), int64(os.Getpid()), nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	file, state := projection.Tree.Live[*goalID], "live"
	if file == nil {
		file, state = projection.Tree.Done[*goalID], "done"
	}
	if file == nil {
		file, state = projection.Tree.Abandoned[*goalID], "abandoned"
	}
	if *abandoned && file == nil {
		_, present, remoteErr := (branch.GitPushTransport{}).RemoteTip(*root, endpoint.Remote, "refs/heads/landing/"+*goalID)
		if remoteErr != nil {
			fmt.Fprintln(os.Stderr, remoteErr)
			return 1
		}
		if present {
			if err := branch.DeleteLanding(branch.DeleteLandingRequest{Repo: *root, Remote: endpoint.Remote, GoalID: *goalID, CheckClaim: func() error { return nil }}); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			fmt.Printf("deleted orphan landing/%s\n", *goalID)
			return 0
		}
	}
	if file == nil {
		fmt.Fprintf(os.Stderr, "goal %s is unknown\n", *goalID)
		return 1
	}
	if err := goalBranchSweepState(state, *abandoned); err != nil {
		fmt.Fprintf(os.Stderr, "goal %s %v\n", *goalID, err)
		return 1
	}
	check := func() error { return nil }
	if state == "live" && file.Claimed != nil {
		check = goalBranchClaimCheck(*root, *goalID, endpoint)
	}
	transport := ""
	if _, remoteErr := goalBranchGit(*root, "remote", "get-url", "transport"); remoteErr == nil {
		transport = "transport"
	}
	result, err := branch.Sweep(branch.SweepRequest{Repo: *root, Remote: endpoint.Remote, Transport: transport,
		EndpointTip: endpointTip, GoalID: *goalID, Dropped: file.NextStep, Abandoned: *abandoned, CheckClaim: check})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if result.Deleted {
		fmt.Printf("deleted goal/%s at %s\n", *goalID, result.Tip)
	} else {
		fmt.Printf("no branch goal/%s\n", *goalID)
	}
	return 0
}

func goalBranchSweepState(state string, abandoned bool) error {
	if state == "abandoned" && !abandoned {
		return fmt.Errorf("is abandoned; repeat with --abandoned to delete its branch")
	}
	if abandoned && state != "abandoned" {
		return fmt.Errorf("is %s, not abandoned", state)
	}
	return nil
}

func listGoalBranchSweep(root string, endpoint goal.Endpoint, tree *goal.TreeGoals) int {
	return listGoalBranchSweepWithGit(root, endpoint, tree, goalBranchGit)
}

func listGoalBranchSweepWithGit(root string, endpoint goal.Endpoint, tree *goal.TreeGoals, gitRead func(string, ...string) (string, error)) int {
	out, err := gitRead(root, "ls-remote", "--heads", endpoint.Remote, "refs/heads/goal/*", "refs/heads/landing/*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	branches := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			branches[fields[1]] = true
		}
	}
	liveIDs := make([]string, 0, len(tree.Live))
	for id := range tree.Live {
		liveIDs = append(liveIDs, id)
	}
	sort.Strings(liveIDs)
	for _, id := range liveIDs {
		file := tree.Live[id]
		ref := "refs/heads/goal/" + id
		if file.State == goal.StateParked && !branches[ref] {
			fmt.Printf("parked-missing goal/%s\n", id)
		}
	}
	refs := make([]string, 0, len(branches))
	for ref := range branches {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		if id, ok := strings.CutPrefix(ref, "refs/heads/goal/"); ok {
			if tree.Done[id] != nil {
				fmt.Printf("done goal/%s\n", id)
			} else if tree.Abandoned[id] != nil {
				fmt.Printf("abandoned goal/%s\n", id)
			}
			continue
		}
		id, ok := strings.CutPrefix(ref, "refs/heads/landing/")
		if !ok {
			continue
		}
		file := tree.Live[id]
		if file == nil || file.Landing == nil {
			fmt.Printf("orphan landing/%s\n", id)
		}
	}
	return 0
}

func runGoalBranchVerify(args []string) int {
	flags := flag.NewFlagSet("goal branch verify", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	landed := flags.String("landed", "", "last landed commit")
	if flags.Parse(args) != nil || *landed == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal branch verify needs --landed")
		return 2
	}
	commit, err := goalBranchGit(*root, "rev-parse", "--verify", *landed+"^{commit}")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	results, err := branch.VerifyLandedSeries(*root, commit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, result := range results {
		fmt.Printf("%s equal goal=%s units=%s digest=%s\n", result.Commit, result.Goal, result.Units, result.Actual)
	}
	return 0
}

type goalBranchLandPrepDependencies struct {
	// CandidateOnly asks the owner for the landing candidate a receipt must
	// prove; --out and --test-receipt are then not read.
	CandidateOnly   bool
	Prepare         func(branch.LandRequest) (branch.LandResult, error)
	LoadContract    func(string) (testpolicy.Contract, error)
	AdmitDiagnostic func() error
	Runner          branch.RedRunner
	TrunkRed        branch.TrunkRedRecorder
	Progress        branch.LandingProgressRecorder
}

func runGoalBranchLandPrep(args []string) int {
	return runGoalBranchLandPrepWith(args, goalBranchLandPrepDependencies{
		Prepare: branch.PrepareLanding,
		LoadContract: func(root string) (testpolicy.Contract, error) {
			_, contract, _, err := loadPhysicalTestingContract(root)
			return contract, err
		},
	})
}

func runGoalBranchLandPrepWith(args []string, dependencies goalBranchLandPrepDependencies) int {
	outcome, code, err := goalBranchLandPrepRun(args, dependencies)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return code
	}
	result := outcome.Result
	if outcome.Classification != "" {
		fmt.Printf("classification=%s landing=%s candidate=%s endpoint=%s attempt=%s proof=%d\n",
			outcome.Classification, result.Landing, result.Candidate, result.Endpoint, result.Attempt, result.ProofNumber)
		return 0
	}
	fmt.Printf("landing=%s candidate=%s endpoint=%s attempt=%s proof=%d\n",
		result.Landing, result.Candidate, result.Endpoint, result.Attempt, result.ProofNumber)
	return 0
}

// goalBranchLandPrepOutcome is one land-prep: the prepared landing and, for a
// red receipt, the owner's classification of that red.
type goalBranchLandPrepOutcome struct {
	Result         branch.LandResult
	Classification string
}

// goalBranchLandPrepRun prepares one hand landing through its owner and
// returns the typed outcome; the exit code accompanies any error.
func goalBranchLandPrepRun(args []string, dependencies goalBranchLandPrepDependencies) (goalBranchLandPrepOutcome, int, error) {
	flags := flag.NewFlagSet("goal branch land-prep", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	out := flags.String("out", "", "new artifact directory")
	receipt := flags.String("test-receipt", "", "schema-3 landing test receipt")
	last := flags.Bool("last", false, "the holder's word that this is the goal's complete unit set")
	through := flags.String("through", "", "last unit commit of a human-approved partial prefix")
	if flags.Parse(args) != nil || *goalID == "" || !dependencies.CandidateOnly && (*out == "" || *receipt == "") || *last == (*through != "") || flags.NArg() != 0 {
		return goalBranchLandPrepOutcome{}, 2, fmt.Errorf("goal branch land-prep needs --goal, --out, --test-receipt, and exactly one of --last or --through")
	}
	endpoint, err := goalBranchEndpoint(*root)
	if err != nil {
		return goalBranchLandPrepOutcome{}, 1, err
	}
	check := goalBranchClaimCheck(*root, *goalID, endpoint)
	if err := branch.CheckHolder(check); err != nil {
		return goalBranchLandPrepOutcome{}, 1, err
	}
	endpointTip, err := goalBranchEndpointTip(*root, endpoint)
	if err != nil {
		return goalBranchLandPrepOutcome{}, 1, err
	}
	branchTip, present, err := goalBranchOriginTip(*root, endpoint, *goalID)
	if err != nil || !present {
		if err == nil {
			err = fmt.Errorf("origin has no goal/%s", *goalID)
		}
		return goalBranchLandPrepOutcome{}, 1, err
	}
	projection, err := goal.Project(endpoint, true, time.Now().UTC())
	if err != nil {
		return goalBranchLandPrepOutcome{}, 1, err
	}
	file := projection.Tree.Live[*goalID]
	if file == nil || file.Approved == nil {
		return goalBranchLandPrepOutcome{}, 1, fmt.Errorf("goal %s has no live approval", *goalID)
	}
	seat, err := goal.ResolveMachine(*root)
	if err != nil {
		return goalBranchLandPrepOutcome{}, 1, err
	}
	if dependencies.Prepare == nil {
		dependencies.Prepare = branch.PrepareLanding
	}
	result, err := dependencies.Prepare(branch.LandRequest{
		Repo: *root, Remote: endpoint.Remote, EndpointTip: endpointTip, BranchTip: branchTip, GoalID: *goalID,
		Out: *out, TestReceipt: *receipt, Last: *last, Through: *through, LandingReady: file.Landing != nil,
		GoalPage: string(goal.RenderFile(file)), ApprovedBy: file.Approved.By, Seat: seat, CheckClaim: check,
		CandidateOnly: dependencies.CandidateOnly,
	})
	if err != nil {
		return goalBranchLandPrepOutcome{}, 1, err
	}
	if len(result.FailingGroups) != 0 {
		if dependencies.LoadContract == nil {
			return goalBranchLandPrepOutcome{Result: result}, 1, fmt.Errorf("landing red classification has no testing contract reader")
		}
		contract, err := dependencies.LoadContract(*root)
		if err != nil {
			return goalBranchLandPrepOutcome{Result: result}, 1, err
		}
		changeSet, err := branch.LandingChangeSet(*root, endpointTip, branchTip, *goalID, result.LastUnit)
		if err != nil {
			return goalBranchLandPrepOutcome{Result: result}, 1, err
		}
		landingTree, err := goalBranchGit(*root, "rev-parse", "--verify", result.Landing+"^{tree}")
		if err != nil || landingTree != result.Candidate {
			return goalBranchLandPrepOutcome{Result: result}, 1, fmt.Errorf("local landing commit %s does not have candidate tree %s: %v", result.Landing, result.Candidate, err)
		}
		admit := dependencies.AdmitDiagnostic
		if admit == nil {
			admit = func() error { return branch.CheckHolder(check) }
		}
		redResult, redErr := branch.HandleLandingRed(branch.RedRequest{
			Goal: *goalID, Endpoint: result.Endpoint, Branch: "goal/" + *goalID, BranchTip: branchTip,
			LastUnit: result.LastUnit, Proof: branch.LandingProof{Number: result.ProofNumber,
				Endpoint: result.Endpoint, Candidate: result.Candidate, Landing: result.Landing, RetryIdentity: result.RetryIdentity, Attempt: result.Attempt},
			FailingGroups: result.FailingGroups, ChangeSet: changeSet, Contract: contract,
			AdmitDiagnostic: admit, Runner: dependencies.Runner, TrunkRed: dependencies.TrunkRed, Progress: dependencies.Progress,
		})
		if redErr != nil {
			return goalBranchLandPrepOutcome{Result: result}, 1, redErr
		}
		return goalBranchLandPrepOutcome{Result: result, Classification: redResult.Classification}, 0, nil
	}
	return goalBranchLandPrepOutcome{Result: result}, 0, nil
}

func runGoalBranchStatus(args []string) int { return runGoalBranchStatusWithRaw(args, nil) }

func runGoalBranchStatusWithRaw(args []string, raw *goalBranchRawDependencies) int {
	flags := flag.NewFlagSet("goal branch status", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	if flags.Parse(args) != nil || *goalID == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal branch status needs --goal")
		return 2
	}
	endpoint, err := raw.endpoint(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	endpointTipReader, originTipReader := goalBranchEndpointTip, goalBranchOriginTip
	if raw != nil {
		endpointTipReader, originTipReader = raw.EndpointTip, raw.OriginTip
	}
	endpointTip, err := endpointTipReader(*root, endpoint)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	tip, present, err := originTipReader(*root, endpoint, *goalID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !present {
		fmt.Println("no branch")
		return 0
	}
	status, err := branch.InspectStatus(*root, endpointTip, tip, *goalID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, commit := range status.Commits {
		fmt.Printf("%.12s %s %s %s\n", commit.ID, commit.Kind, commit.Unit, commit.Digest)
	}
	for _, unit := range status.Units {
		fmt.Printf("unit %s %s %s %s\n", unit.Unit, unit.Commit, unit.Digest, unit.ReadState)
	}
	last := "none"
	if status.Prefix > 0 {
		last = status.Units[status.Prefix-1].Commit
	}
	fmt.Printf("land-ready prefix %d/%d through %s\n", status.Prefix, len(status.Units), last)
	return 0
}

func goalBranchEndpoint(root string) (goal.Endpoint, error) {
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return goal.Endpoint{}, err
	}
	if endpoint.Branch != "refs/heads/main" {
		return goal.Endpoint{}, fmt.Errorf("GOAL_BRANCH_ENDPOINT_UNSUPPORTED: endpoint %s is not refs/heads/main", endpoint.Branch)
	}
	return endpoint, nil
}

func goalBranchEndpointTip(root string, endpoint goal.Endpoint) (tip string, err error) {
	return goalBranchEndpointTipWithGit(root, endpoint, goalBranchGit)
}

func goalBranchEndpointTipWithGit(root string, endpoint goal.Endpoint, git func(string, ...string) (string, error)) (tip string, err error) {
	opid, err := branchOperationID()
	if err != nil {
		return "", err
	}
	temporary := "refs/metasystem/goals/endpoint/" + opid
	defer func() {
		if _, clearErr := git(root, "update-ref", "-d", temporary); err == nil && clearErr != nil {
			err = clearErr
		}
	}()
	if _, err = git(root, "fetch", "--no-tags", "--refmap=", endpoint.Remote, "+refs/heads/main:"+temporary); err != nil {
		return "", err
	}
	return git(root, "rev-parse", "--verify", temporary+"^{commit}")
}

func goalBranchOriginTip(root string, endpoint goal.Endpoint, goalID string) (tip string, present bool, err error) {
	transport := branch.GitPushTransport{}
	opid, err := branchOperationID()
	if err != nil {
		return "", false, err
	}
	temporary := "refs/metasystem/goals/check/" + opid
	defer func() {
		if _, clearErr := goalBranchGit(root, "update-ref", "-d", temporary); err == nil && clearErr != nil {
			err = clearErr
		}
	}()
	goalRef := "refs/heads/goal/" + goalID
	if fetchErr := transport.Fetch(root, endpoint.Remote, goalRef, temporary); fetchErr != nil {
		_, present, err = transport.RemoteTip(root, endpoint.Remote, goalRef)
		if err != nil || !present {
			return "", present, err
		}
		return "", false, fetchErr
	}
	tip, err = goalBranchGit(root, "rev-parse", "--verify", temporary+"^{commit}")
	return tip, true, err
}

func goalBranchClaimCheck(root, goalID string, endpoint goal.Endpoint) func() error {
	return goalBranchClaimCheckWith(root, goalID, endpoint, func(root, _ string) (string, error) { return goal.ResolveMachine(root) }, goalBranchHolderRoot)
}

func goalBranchClaimCheckWith(root, goalID string, endpoint goal.Endpoint, config func(string, string) (string, error), holderRoot func(string) string) func() error {
	return func() error {
		machine, err := goal.ResolveMachineWithConfig(root, config)
		if err != nil {
			return err
		}
		holder := holderRoot(root)
		if _, err := lease.RequireHolder(holder, int64(os.Getpid()), nil); err != nil {
			return err
		}
		current, err := lease.CurrentHolder(holder)
		if err != nil {
			return err
		}
		projection, err := goal.Project(endpoint, true, time.Now().UTC())
		if err != nil {
			return err
		}
		file := projection.Tree.Live[goalID]
		if file == nil || file.Claimed == nil || file.Claimed.Machine != machine || file.Claimed.Lineage != current.OwnerLineage {
			return fmt.Errorf("goal %s is not claimed by %s+%s", goalID, machine, current.OwnerLineage)
		}
		return nil
	}
}

func goalBranchHolderRoot(root string) string {
	if main, linked := linkedWorktreeMainCheckout(root); linked {
		top, topErr := goalBranchGit(root, "rev-parse", "--show-toplevel")
		installation, rootErr := filepath.Abs(root)
		if rootErr == nil {
			installation, rootErr = filepath.EvalSymlinks(installation)
		}
		relative, relErr := filepath.Rel(top, installation)
		if topErr == nil && rootErr == nil && relErr == nil {
			return filepath.Join(main, relative)
		}
		return main
	}
	return root
}

func runGoalBranchCheck(args []string) int {
	return runGoalBranchCheckWith(args, nil, nil, nil)
}

func runGoalBranchCheckWith(args []string, configLookup func(string, string) (string, error), cliGitRead func(string, ...string) (string, error), rangeGitRead func(string, ...string) ([]byte, error)) int {
	if cliGitRead == nil {
		cliGitRead = goalBranchGit
	}
	flags := flag.NewFlagSet("goal branch check", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	tipFlag := flags.String("tip", "", "branch tip commit")
	noFetch := flags.Bool("no-fetch", false, "use existing remote-tracking refs")
	if flags.Parse(args) != nil || *goalID == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal branch check needs --goal")
		return 2
	}
	var endpoint goal.Endpoint
	var err error
	if configLookup == nil {
		endpoint, err = goal.ResolveEndpoint(*root)
	} else {
		endpoint, err = goal.ResolveEndpointWithConfig(*root, configLookup)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if endpoint.Branch != "refs/heads/main" {
		fmt.Fprintf(os.Stderr, "GOAL_BRANCH_ENDPOINT_UNSUPPORTED: endpoint %s is not refs/heads/main\n", endpoint.Branch)
		return 1
	}
	var endpointTip string
	if !*noFetch {
		if endpointTip, err = goalBranchEndpointTip(*root, endpoint); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	} else {
		endpointTip, err = cliGitRead(*root, "rev-parse", "--verify", "refs/remotes/"+endpoint.Remote+"/main^{commit}")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	var tip string
	if *tipFlag != "" {
		tip, err = cliGitRead(*root, "rev-parse", "--verify", "-q", *tipFlag+"^{commit}")
	} else if !*noFetch {
		var present bool
		tip, present, err = goalBranchOriginTip(*root, endpoint, *goalID)
		if err == nil && !present {
			fmt.Println("no branch")
			return 0
		}
	} else {
		tip, err = cliGitRead(*root, "rev-parse", "--verify", "-q", "refs/remotes/"+endpoint.Remote+"/goal/"+*goalID+"^{commit}")
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			fmt.Println("no branch")
			return 0
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var commits []branch.Commit
	if rangeGitRead == nil {
		commits, err = branch.ValidateRange(*root, endpointTip, tip, *goalID)
	} else {
		commits, err = branch.ValidateRangeWithGit(*root, endpointTip, tip, *goalID, rangeGitRead)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, commit := range commits {
		fmt.Printf("%.12s %s %s %s\n", commit.ID, commit.Kind, commit.Unit, commit.Digest)
	}
	return 0
}

type goalBranchTestChanges []branch.TestChange

func (v *goalBranchTestChanges) String() string { return fmt.Sprint([]branch.TestChange(*v)) }
func (v *goalBranchTestChanges) Set(value string) error {
	path, word, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(path) == "" || strings.TrimSpace(word) == "" {
		return fmt.Errorf("test change must be path=reader-word")
	}
	*v = append(*v, branch.TestChange{Path: path, ReaderWord: word})
	return nil
}

type goalBranchCommitDependencies struct {
	Gate  func(string) (string, error)
	NewID func(string) (string, error)
	Raw   *goalBranchRawDependencies
}

func runGoalBranchCommit(args []string) int {
	return runGoalBranchCommitWith(args, goalBranchCommitDependencies{Gate: readGate})
}

func runGoalBranchCommitWith(args []string, dependencies goalBranchCommitDependencies) int {
	flags := flag.NewFlagSet("goal branch commit", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	kindName := flags.String("kind", "", "unit, plan, or read")
	var units repeatedStringFlag
	flags.Var(&units, "unit", "unit name (repeat for every unit in this build)")
	amend := flags.Bool("amend", false, "replace the named unit commit")
	rootJob := flags.String("root-job", "", "closed code-critic root")
	readerRecord := flags.String("reader-record", "", "prose read record")
	carry := flags.String("carry", "", "old unit commit whose clean read carries")
	gateRun := flags.String("gate-run", "", "fast-gate run id")
	gateTree := flags.String("gate-tree", "", "tree observed by the fast gate")
	var tests goalBranchTestChanges
	flags.Var(&tests, "test-change", "changed test and reader word (path=word, repeatable)")
	if flags.Parse(args) != nil || *goalID == "" || *kindName == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal branch commit needs --goal and --kind")
		return 2
	}
	endpoint, err := dependencies.Raw.endpoint(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	linked := false
	if dependencies.Raw != nil {
		linked = dependencies.Raw.Linked(*root)
	} else {
		_, linked = linkedWorktreeMainCheckout(*root)
	}
	checkCheckout := branch.CheckCommitCheckout
	if dependencies.Raw != nil {
		checkCheckout = func(repo, goalID string, linked bool) error {
			return branch.CheckCommitCheckoutWithHeadRef(repo, goalID, linked, dependencies.Raw.HeadRef)
		}
	}
	if err := checkCheckout(*root, *goalID, linked); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	check := goalBranchClaimCheck(*root, *goalID, endpoint)
	holderRoot := goalBranchHolderRoot
	if dependencies.Raw != nil {
		check = goalBranchClaimCheckWith(*root, *goalID, endpoint, dependencies.Raw.Config, dependencies.Raw.HolderRoot)
		holderRoot = dependencies.Raw.HolderRoot
	}
	if err := branch.CheckCommitAccess(*goalID, check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	endpointTipReader := goalBranchEndpointTip
	if dependencies.Raw != nil {
		endpointTipReader = dependencies.Raw.EndpointTip
	}
	endpointTip, err := endpointTipReader(*root, endpoint)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	operationID, err := branchOperationID()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if branch.Kind(*kindName) == branch.Read {
		var commit string
		err = withGoalBranchCommitTokenAt(*root, holderRoot(*root), func() error {
			request := branch.CommitReadRequest{
				Repo: *root, Remote: endpoint.Remote, EndpointTip: endpointTip, GoalID: *goalID, Units: units, OpID: operationID,
				CheckClaim: check, RootJob: *rootJob, ReaderRecord: *readerRecord, Carry: *carry,
				TestsChanged: tests,
			}
			if dependencies.Raw != nil {
				request.Inputs, request.GateRepository, request.Transport = dependencies.Raw.ReadInputs, dependencies.Raw.ReadRepository, dependencies.Raw.Transport
			}
			observation, observeErr := branch.ResolveCommitReadGate(request, dependencies.Gate, dependencies.NewID)
			if observeErr != nil {
				return observeErr
			}
			if (*gateRun != "" && *gateRun != observation.RunID) || (*gateTree != "" && *gateTree != observation.Tree) {
				return fmt.Errorf("%s: recorded fast-gate observation is run %s on tree %s", branch.ReadUngatedCode, observation.RunID, observation.Tree)
			}
			request.GateRunID, request.GateTree = observation.RunID, observation.Tree
			var commitErr error
			commit, _, commitErr = branch.CommitRead(request)
			return commitErr
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(commit)
		return 0
	}
	if *rootJob != "" || *readerRecord != "" || *carry != "" || *gateRun != "" || *gateTree != "" || len(tests) != 0 {
		fmt.Fprintln(os.Stderr, "read source, gate, carry, and test flags require --kind read")
		return 2
	}
	var commit string
	err = withGoalBranchCommitTokenAt(*root, holderRoot(*root), func() error {
		var commitErr error
		request := branch.CommitRequest{
			Repo: *root, Remote: endpoint.Remote, EndpointTip: endpointTip, GoalID: *goalID, Units: units, OpID: operationID,
			Kind: branch.Kind(*kindName), Amend: *amend, CheckClaim: check,
		}
		if dependencies.Raw != nil {
			request.Transport = dependencies.Raw.Transport
			commit, commitErr = branch.CommitStagedWithInputs(request, dependencies.Raw.ReadInputs.Facts, dependencies.Raw.ReadInputs.Effects)
		} else {
			commit, commitErr = branch.CommitStaged(request)
		}
		return commitErr
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(commit)
	return 0
}

func withGoalBranchCommitTokenAt(root, holderRoot string, commit func() error) error {
	pid := int64(os.Getpid())
	if _, err := lease.RequireHolder(holderRoot, pid, nil); err != nil {
		return err
	}
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		return fmt.Errorf("goal branch commit: caller process start time is unreadable")
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := filepath.Join(root, "artifacts", "agents", "mains", "worktree-commit-token.json")
	if err := writeIdentityJSON(token, map[string]any{
		"wrapperPid": pid, "wrapperPidStartedAt": exact.StartedAt.Unix(),
		"nonce": hex.EncodeToString(raw), "createdAt": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}); err != nil {
		return err
	}
	defer os.Remove(token)
	return commit()
}

func branchOperationID() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "branch-" + hex.EncodeToString(raw), nil
}

func runGoalBranchPush(args []string) int {
	flags := flag.NewFlagSet("goal branch push", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	opid := flags.String("opid", "", "operation id")
	mirror := flags.Bool("transport", false, "mirror origin's goal branch to the transport remote")
	if flags.Parse(args) != nil || *goalID == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal branch push needs --goal")
		return 2
	}
	endpoint, err := goalBranchEndpoint(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	check := goalBranchClaimCheck(*root, *goalID, endpoint)
	if err := branch.CheckHolder(check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	endpointTip, err := goalBranchEndpointTip(*root, endpoint)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *opid == "" {
		*opid, err = branchOperationID()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	if *mirror {
		result, err := branch.Mirror(branch.MirrorRequest{
			Repo: *root, Origin: endpoint.Remote, Transport: "transport", EndpointTip: endpointTip,
			GoalID: *goalID, OpID: *opid, CheckClaim: check,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("%s %s\n", result.State, result.Tip)
		return 0
	}
	result, err := branch.Push(branch.PushRequest{
		Repo: *root, Remote: endpoint.Remote, EndpointTip: endpointTip,
		GoalID: *goalID, OpID: *opid, CheckClaim: check,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("%s %s\n", result.State, result.Tip)
	return 0
}

// goalBranchSweepLanded sweeps the merged goal branch after its last landing
// was pushed: the sweep land-push runs, repeatable after it failed.
func goalBranchSweepLanded(root, goalID, landing string) error {
	endpoint, err := goalBranchEndpoint(root)
	if err != nil {
		return err
	}
	if !branch.IsLastLanding(root, landing, goalID) {
		return nil
	}
	return goalBranchSweepLandedAt(root, goalID, landing, endpoint)
}

func goalBranchSweepLandedAt(root, goalID, landing string, endpoint goal.Endpoint) error {
	transport := ""
	if _, remoteErr := goalBranchGit(root, "remote", "get-url", "transport"); remoteErr == nil {
		transport = "transport"
	}
	_, err := branch.Sweep(branch.SweepRequest{Repo: root, Remote: endpoint.Remote, Transport: transport,
		EndpointTip: landing, GoalID: goalID, CheckClaim: goalBranchClaimCheck(root, goalID, endpoint)})
	return err
}

// goalBranchPublishRead publishes a collected read's attestation through the
// goal-branch push owner and returns the remote tip that now contains it.
func goalBranchPublishRead(root, goalID, unit string) (branch.PublishReadResult, error) {
	endpoint, err := goalBranchEndpoint(root)
	if err != nil {
		return branch.PublishReadResult{}, err
	}
	endpointTip, err := goalBranchEndpointTip(root, endpoint)
	if err != nil {
		return branch.PublishReadResult{}, err
	}
	commit, err := goalBranchGit(root, "rev-parse", "--verify", unit+"^{commit}")
	if err != nil {
		return branch.PublishReadResult{}, err
	}
	return branch.PublishCollectedRead(branch.PublishReadRequest{Repo: root, Remote: endpoint.Remote, EndpointTip: endpointTip,
		GoalID: goalID, UnitCommit: commit, CheckClaim: goalBranchClaimCheck(root, goalID, endpoint)})
}

// criticDelegateEnvironment carries the selected installation's configured
// code-critic roster to a critic dispatched from another installation (a
// generated goal worktree, whose tracked roster may be a template and which
// never has the selected checkout's metasystem.conf.local). The roster is
// resolved by the roster owner in the selected installation for the working
// mode dispatch reads from the same frozen brief, and is passed as the
// configuration owner's per-key process settings. Those outrank every file
// entry, mode-scoped or not, so dispatch resolving that mode in the worktree
// obtains exactly this pair as its configured default rather than an
// override: an explicit --runtime/--model still escalates by the unchanged
// roster policy. The selected installation's maximal-model mapping for that
// runtime is carried as its exact value (empty when it has none), so the
// worktree's hazard check admits or refuses the selected model by the
// selected installation's authorization. An unreadable brief mode or a
// selected roster that does not resolve is refused before any dispatch.
func criticDelegateEnvironment(selected, root, brief string) ([]string, error) {
	if selected == "" || filepath.Clean(selected) == filepath.Clean(root) {
		return nil, nil
	}
	mode, err := dispatchcore.BriefModeOnly(brief)
	if err != nil {
		return nil, fmt.Errorf("the critic brief %s has no readable working mode: %w", brief, err)
	}
	conf := filepath.Join(selected, "metasystem.conf")
	resolution, err := dispatchcore.ResolveRoster(dispatchcore.RosterParams{ConfPath: conf, Role: "code-critic", Mode: mode})
	if err != nil {
		return nil, fmt.Errorf("the selected installation's code-critic roster for mode %s does not resolve: %w", mode, err)
	}
	runtimes, _, err := config.Get(config.GetParams{Key: "metasystem.runtimes", ConfPath: conf})
	if err != nil {
		return nil, fmt.Errorf("the selected installation's metasystem.runtimes does not resolve: %w", err)
	}
	maximalKey := "runtime." + resolution.RosterRuntime + ".maximal-models"
	maximal, _, err := config.Get(config.GetParams{Key: maximalKey, ConfPath: conf, Default: "", DefaultSet: true})
	if err != nil {
		return nil, fmt.Errorf("the selected installation's %s does not resolve: %w", maximalKey, err)
	}
	return []string{
		config.EnvName("metasystem.runtimes") + "=" + runtimes,
		config.EnvName("role.code-critic.runtime") + "=" + resolution.RosterRuntime,
		config.EnvName("role.code-critic.model."+resolution.RosterRuntime) + "=" + resolution.RosterModel,
		config.EnvName(maximalKey) + "=" + maximal,
	}, nil
}
