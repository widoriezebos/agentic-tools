package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The four raw readers below moved into internal/goal/branch, where the
// interface's own park reaches them too; these are the command edge's
// one-line names for them.
func goalBranchGit(root string, args ...string) (string, error) {
	return branch.ScrubbedGit(root, args...)
}

type goalBranchReadDependencies struct {
	// Delegator is the delegate boundary the read calls in its own process
	// (design 6.2); nil is the production boundary.
	Delegator delegateCaller
	Gate      func(string) (string, error)
	Delegate  func(string, string, string, string, string) (string, error)
	Commit    func(branch.CommitReadRequest) (string, branch.Attestation, error)
	Raw       *goalBranchRawDependencies
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
		return branch.MainEndpoint(root)
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
	argv := goalBranchStaticArgv(proofPath)
	command := exec.Command(argv[0], argv[1:]...)
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

// delegateCaller is one call of the delegate boundary on the caller's
// streams, returning its status.
type delegateCaller func(request delegateRequest, stdout, stderr io.Writer) int

// callDelegate runs one delegate request in this process as the former
// `internal delegate` child ran: the installation is root, the script gets
// no input, and the extra environment reaches the script. It returns the
// child's stdout and stderr and, for a nonzero status, the error its exit
// was.
func callDelegate(delegate delegateCaller, root string, args []string, environment []string) ([]byte, []byte, error) {
	if delegate == nil {
		delegate = runDelegateWith
	}
	var stdout, stderr bytes.Buffer
	status := delegate(delegateRequest{rootOverride: root, args: args, environment: environment}, &stdout, &stderr)
	if status != 0 {
		return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("delegate exited with status %d", status)
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func readDelegate(delegate delegateCaller, root, brief, goalID, commit, runtime, model string, environment ...string) (string, error) {
	args := []string{"--role", "code-critic", "--reviews", "commit:" + commit,
		"--goal", goalID, "--brief", brief, "--destructive-reach", "DESIGN-BEARING"}
	if runtime != "" {
		args = append(args, "--runtime", runtime)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	output, stderr, err := callDelegate(delegate, root, args, environment)
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
		return "", fmt.Errorf("delegate: %s: %w", lastOutputLine(stderr), err)
	}
	return "", fmt.Errorf("delegate returned no started job: %s", lastOutputLine(output))
}

// readFollowUp starts one more round of a critic chain through the delegate
// follow-up and returns the round's job id.
func readFollowUp(delegate delegateCaller, root, rootJob, brief string, environment ...string) (string, error) {
	output, stderr, err := callDelegate(delegate, root, []string{"--follow-up", rootJob, "--brief", brief}, environment)
	var outcome delegateOutcome
	if jsonErr := json.Unmarshal(bytes.TrimSpace(output), &outcome); jsonErr == nil && outcome.Outcome == "WON" && outcome.JobID != "" && err == nil {
		return outcome.JobID, nil
	}
	if err != nil {
		return "", fmt.Errorf("delegate follow-up: %s: %w", lastOutputLine(stderr), err)
	}
	return "", fmt.Errorf("delegate follow-up returned no started round: %s", lastOutputLine(output))
}

// goalBranchReadRun is the branch read owner with its typed result; the
// exit code accompanies any error. It prints nothing: a mistake in the
// words its caller built is the returned error.
func goalBranchReadRun(args []string, dependencies goalBranchReadDependencies) (branch.BranchReadResult, int, error) {
	// A mistake in the words is returned, never printed.
	var parseProblem strings.Builder
	flags := newFlagSet("goal branch read", io.Discard, &parseProblem)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	unit := flags.String("unit", "", "Goal-Unit commit")
	brief, runtime, model := &goalBranchReadOption{name: "brief"}, &goalBranchReadOption{name: "runtime"}, &goalBranchReadOption{name: "model"}
	flags.Var(brief, "brief", "accepted implementation brief to freeze into the critic dispatch")
	flags.Var(runtime, "runtime", "requested critic runtime (subject to roster authorization)")
	flags.Var(model, "model", "requested critic model (subject to roster authorization)")
	collect := flags.Bool("collect", false, "collect a closed critic root into an attestation")
	retry := flags.Int64("retry", 0, "examine the critic chain's failed round N once more, in the same chain")
	selected := flags.String("selected-installation", "", "installation whose configured code-critic roster the critic dispatch resolves (a generated goal worktree's selected installation)")
	parseErr := flags.Parse(args)
	if parseErr != nil && parseProblem.Len() > 0 {
		return branch.BranchReadResult{}, 2, errors.New(strings.TrimSpace(parseProblem.String()))
	}
	if parseErr != nil || flags.NArg() != 0 || *goalID == "" || *unit == "" {
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
			return readDelegate(dependencies.Delegator, *root, brief, goalID, commit, runtime, model, environment...)
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
		CheckClaim: goalBranchClaimCheckWith(*root, *goalID, endpoint, config, holderRoot), Gate: gate, Delegate: delegate, Commit: commitRead, Repository: readRepository,
		Retry: *retry, FollowUp: func(rootJob, brief string) (string, error) {
			environment, err := criticDelegateEnvironment(*selected, *root, brief)
			if err != nil {
				return "", err
			}
			return readFollowUp(dependencies.Delegator, *root, rootJob, brief, environment...)
		}})
	if err != nil {
		return branch.BranchReadResult{}, 1, err
	}
	return result, 0, nil
}

// goalBranchLandPushRun pushes one prepared landing and sweeps a goal's last
// landing, returning the pushed landing and the endpoint branch it moved.
func goalBranchLandPushRun(args []string) (branch.PreparedLanding, string, int, error) {
	// A mistake in the words is returned, never printed.
	var parseProblem strings.Builder
	flags := newFlagSet("goal branch land-push", io.Discard, &parseProblem)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	prepared := flags.String("prepared", "", "land-prep artifact directory")
	parseErr := flags.Parse(args)
	if parseErr != nil && parseProblem.Len() > 0 {
		return branch.PreparedLanding{}, "", 2, errors.New(strings.TrimSpace(parseProblem.String()))
	}
	if parseErr != nil || *goalID == "" || *prepared == "" || flags.NArg() != 0 {
		return branch.PreparedLanding{}, "", 2, fmt.Errorf("goal branch land-push needs --goal and --prepared")
	}
	endpoint, err := branch.MainEndpoint(*root)
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

// goalBranchLandPrepOutcome is one land-prep: the prepared landing and, for a
// red receipt, the owner's classification of that red.
type goalBranchLandPrepOutcome struct {
	Result         branch.LandResult
	Classification string
}

// goalBranchLandPrepRun prepares one hand landing through its owner and
// returns the typed outcome; the exit code accompanies any error.
func goalBranchLandPrepRun(args []string, dependencies goalBranchLandPrepDependencies) (goalBranchLandPrepOutcome, int, error) {
	// A mistake in the words is returned, never printed.
	var parseProblem strings.Builder
	flags := newFlagSet("goal branch land-prep", io.Discard, &parseProblem)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	out := flags.String("out", "", "new artifact directory")
	receipt := flags.String("test-receipt", "", "schema-3 landing test receipt")
	last := flags.Bool("last", false, "the holder's word that this is the goal's complete unit set")
	through := flags.String("through", "", "last unit commit of a human-approved partial prefix")
	parseErr := flags.Parse(args)
	if parseErr != nil && parseProblem.Len() > 0 {
		return goalBranchLandPrepOutcome{}, 2, errors.New(strings.TrimSpace(parseProblem.String()))
	}
	if parseErr != nil || *goalID == "" || !dependencies.CandidateOnly && (*out == "" || *receipt == "") || *last == (*through != "") || flags.NArg() != 0 {
		return goalBranchLandPrepOutcome{}, 2, fmt.Errorf("goal branch land-prep needs --goal, --out, --test-receipt, and exactly one of --last or --through")
	}
	endpoint, err := branch.MainEndpoint(*root)
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

func goalBranchEndpointTip(root string, endpoint goal.Endpoint) (tip string, err error) {
	return branch.EndpointTip(root, endpoint)
}

func goalBranchOriginTip(root string, endpoint goal.Endpoint, goalID string) (tip string, present bool, err error) {
	return branch.OriginTip(root, endpoint, goalID)
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
			return fmt.Errorf("goal %s is not claimed by %s+%s, and writing its branch from here would race the session that holds it; the holding session writes it, or a person takes the goal over with metasystem goal claim %s --take-over --reason TEXT", goalID, machine, current.OwnerLineage, goalID)
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

func withGoalBranchCommitTokenAt(root, holderRoot string, commit func() error) error {
	return goalBranchCheckoutSection(root, holderRoot, func(withToken func(func() error) error) error {
		return withToken(commit)
	})
}

// goalBranchCheckoutSection runs body inside the checkout's mutation
// section: the holder is established first, the bounded checkout mutation
// lock is then taken and the holder checked again inside it. withToken mints
// the commit identity token and runs one commit under it; it takes no lock,
// so staging and its commit share one section without nested acquisition.
// The token is the hook's identity proof; the lock is the exclusion.
func goalBranchCheckoutSection(root, holderRoot string, body func(withToken func(func() error) error) error) error {
	pid := int64(os.Getpid())
	if _, err := lease.RequireHolder(holderRoot, pid, nil); err != nil {
		return err
	}
	release, err := lease.LockBounded(lease.LockPath(holderRoot), "goal branch commit")
	if err != nil {
		return err
	}
	defer release()
	if _, err := lease.RequireHolder(holderRoot, pid, nil); err != nil {
		return err
	}
	return body(func(commit func() error) error { return goalBranchCommitToken(root, pid, commit) })
}

func goalBranchCommitToken(root string, pid int64, commit func() error) error {
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

func branchOperationID() (string, error) { return branch.OperationID() }

// goalBranchSweepLanded sweeps the merged goal branch after its last landing
// was pushed: the sweep land-push runs, repeatable after it failed.
func goalBranchSweepLanded(root, goalID, landing string) error {
	endpoint, err := branch.MainEndpoint(root)
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
	endpoint, err := branch.MainEndpoint(root)
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

// goalBranchStaticArgv runs the worktree's own static gate, trimmed.
func goalBranchStaticArgv(proofPath string) []string {
	return []string{"go", "run", "-trimpath", "./cmd/devgate", "static", "--proof-out", proofPath}
}
