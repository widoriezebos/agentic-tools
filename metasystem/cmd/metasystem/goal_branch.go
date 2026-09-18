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
	Delegate func(string, string, string) (string, error)
	Commit   func(branch.CommitReadRequest) (string, branch.Attestation, error)
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

func readDelegate(binary, root, brief, goalID, commit string) (string, error) {
	if binary == "" {
		return "", fmt.Errorf("delegate binary is unavailable")
	}
	command := exec.Command(binary, "delegate", "--role", "code-critic", "--reviews", "commit:"+commit,
		"--goal", goalID, "--brief", brief, "--destructive-reach", "DESIGN-BEARING")
	command.Env = append(os.Environ(), "METASYSTEM_DELEGATE_ROOT="+root)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("delegate: %s: %w", lastOutputLine(stderr.Bytes()), err)
	}
	var outcome delegateOutcome
	if jsonErr := json.Unmarshal(output, &outcome); jsonErr != nil || outcome.JobID == "" || outcome.Outcome != "WON" {
		return "", fmt.Errorf("delegate returned no started job: %s", lastOutputLine(output))
	}
	return outcome.JobID, nil
}

func runGoalBranchReadWith(args []string, dependencies goalBranchReadDependencies) int {
	flags := flag.NewFlagSet("goal branch read", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	unit := flags.String("unit", "", "Goal-Unit commit")
	collect := flags.Bool("collect", false, "collect a closed critic root into an attestation")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *goalID == "" || *unit == "" {
		fmt.Fprintln(os.Stderr, "goal branch read needs --goal and --unit")
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
	branchTip, present, err := goalBranchOriginTip(*root, endpoint, *goalID)
	if err != nil || !present {
		if err == nil {
			err = fmt.Errorf("origin has no goal/%s", *goalID)
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	commit, err := goalBranchGit(*root, "rev-parse", "--verify", *unit+"^{commit}")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	gate := dependencies.Gate
	if gate == nil {
		gate = readGate
	}
	delegate := dependencies.Delegate
	if delegate == nil {
		delegate = func(brief, goalID, commit string) (string, error) {
			return readDelegate(dependencies.Binary, *root, brief, goalID, commit)
		}
	}
	result, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: *root, Remote: endpoint.Remote,
		EndpointTip: endpointTip, BranchTip: branchTip, GoalID: *goalID, UnitCommit: commit, Collect: *collect,
		CheckClaim: goalBranchClaimCheck(*root, *goalID, endpoint), Gate: gate, Delegate: delegate, Commit: dependencies.Commit})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("state=%s root-job=%s gate-run=%s", result.State, result.RootJob, result.GateRunID)
	if result.AttestationCommit != "" {
		fmt.Printf(" attestation=%s", result.AttestationCommit)
	}
	fmt.Println()
	return 0
}

func runGoalBranchLandPush(args []string) int {
	flags := flag.NewFlagSet("goal branch land-push", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	prepared := flags.String("prepared", "", "land-prep artifact directory")
	if flags.Parse(args) != nil || *goalID == "" || *prepared == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal branch land-push needs --goal and --prepared")
		return 2
	}
	endpoint, err := goalBranchEndpoint(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result, err := branch.LandPush(branch.LandPushRequest{Repo: *root, Remote: endpoint.Remote, EndpointRef: endpoint.Branch,
		GoalID: *goalID, Prepared: *prepared, CheckClaim: goalBranchClaimCheck(*root, *goalID, endpoint)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if branch.IsLastLanding(*root, result.Landing, *goalID) {
		transport := ""
		if _, remoteErr := goalBranchGit(*root, "remote", "get-url", "transport"); remoteErr == nil {
			transport = "transport"
		}
		if _, err := branch.Sweep(branch.SweepRequest{Repo: *root, Remote: endpoint.Remote, Transport: transport,
			EndpointTip: result.Landing, GoalID: *goalID, CheckClaim: goalBranchClaimCheck(*root, *goalID, endpoint)}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	fmt.Printf("landed %s endpoint=%s branch-deleted=%s\n", result.Landing, endpoint.Branch, result.Branch)
	return 0
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
	out, err := goalBranchGit(root, "ls-remote", "--heads", endpoint.Remote, "refs/heads/goal/*", "refs/heads/landing/*")
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
	flags := flag.NewFlagSet("goal branch land-prep", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	out := flags.String("out", "", "new artifact directory")
	receipt := flags.String("test-receipt", "", "schema-3 landing test receipt")
	last := flags.Bool("last", false, "the holder's word that this is the goal's complete unit set")
	through := flags.String("through", "", "last unit commit of a human-approved partial prefix")
	if flags.Parse(args) != nil || *goalID == "" || *out == "" || *receipt == "" || *last == (*through != "") || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal branch land-prep needs --goal, --out, --test-receipt, and exactly one of --last or --through")
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
	branchTip, present, err := goalBranchOriginTip(*root, endpoint, *goalID)
	if err != nil || !present {
		if err == nil {
			err = fmt.Errorf("origin has no goal/%s", *goalID)
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	projection, err := goal.Project(endpoint, true, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	file := projection.Tree.Live[*goalID]
	if file == nil || file.Approved == nil {
		fmt.Fprintf(os.Stderr, "goal %s has no live approval\n", *goalID)
		return 1
	}
	seat, err := goal.ResolveMachine(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if dependencies.Prepare == nil {
		dependencies.Prepare = branch.PrepareLanding
	}
	result, err := dependencies.Prepare(branch.LandRequest{
		Repo: *root, Remote: endpoint.Remote, EndpointTip: endpointTip, BranchTip: branchTip, GoalID: *goalID,
		Out: *out, TestReceipt: *receipt, Last: *last, Through: *through, LandingReady: file.Landing != nil,
		GoalPage: string(goal.RenderFile(file)), ApprovedBy: file.Approved.By, Seat: seat, CheckClaim: check,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(result.FailingGroups) != 0 {
		if dependencies.LoadContract == nil {
			fmt.Fprintln(os.Stderr, "landing red classification has no testing contract reader")
			return 1
		}
		contract, err := dependencies.LoadContract(*root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		changeSet, err := branch.LandingChangeSet(*root, endpointTip, branchTip, *goalID, result.LastUnit)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		landingTree, err := goalBranchGit(*root, "rev-parse", "--verify", result.Landing+"^{tree}")
		if err != nil || landingTree != result.Candidate {
			fmt.Fprintf(os.Stderr, "local landing commit %s does not have candidate tree %s: %v\n", result.Landing, result.Candidate, err)
			return 1
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
			fmt.Fprintln(os.Stderr, redErr)
			return 1
		}
		fmt.Printf("classification=%s landing=%s candidate=%s endpoint=%s attempt=%s proof=%d\n",
			redResult.Classification, result.Landing, result.Candidate, result.Endpoint, result.Attempt, result.ProofNumber)
		return 0
	}
	fmt.Printf("landing=%s candidate=%s endpoint=%s attempt=%s proof=%d\n",
		result.Landing, result.Candidate, result.Endpoint, result.Attempt, result.ProofNumber)
	return 0
}

func runGoalBranchStatus(args []string) int {
	flags := flag.NewFlagSet("goal branch status", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	if flags.Parse(args) != nil || *goalID == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal branch status needs --goal")
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
	tip, present, err := goalBranchOriginTip(*root, endpoint, *goalID)
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
	opid, err := branchOperationID()
	if err != nil {
		return "", err
	}
	temporary := "refs/metasystem/goals/endpoint/" + opid
	defer func() {
		if _, clearErr := goalBranchGit(root, "update-ref", "-d", temporary); err == nil && clearErr != nil {
			err = clearErr
		}
	}()
	if _, err = goalBranchGit(root, "fetch", "--no-tags", "--refmap=", endpoint.Remote, "+refs/heads/main:"+temporary); err != nil {
		return "", err
	}
	return goalBranchGit(root, "rev-parse", "--verify", temporary+"^{commit}")
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
	return func() error {
		machine, err := goal.ResolveMachine(root)
		if err != nil {
			return err
		}
		holderRoot := goalBranchHolderRoot(root)
		if _, err := lease.RequireHolder(holderRoot, int64(os.Getpid()), nil); err != nil {
			return err
		}
		holder, err := lease.CurrentHolder(holderRoot)
		if err != nil {
			return err
		}
		projection, err := goal.Project(endpoint, true, time.Now().UTC())
		if err != nil {
			return err
		}
		file := projection.Tree.Live[goalID]
		if file == nil || file.Claimed == nil || file.Claimed.Machine != machine || file.Claimed.Lineage != holder.OwnerLineage {
			return fmt.Errorf("goal %s is not claimed by %s+%s", goalID, machine, holder.OwnerLineage)
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
	flags := flag.NewFlagSet("goal branch check", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	tipFlag := flags.String("tip", "", "branch tip commit")
	noFetch := flags.Bool("no-fetch", false, "use existing remote-tracking refs")
	if flags.Parse(args) != nil || *goalID == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal branch check needs --goal")
		return 2
	}
	endpoint, err := goal.ResolveEndpoint(*root)
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
		endpointTip, err = goalBranchGit(*root, "rev-parse", "--verify", "refs/remotes/"+endpoint.Remote+"/main^{commit}")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	var tip string
	if *tipFlag != "" {
		tip, err = goalBranchGit(*root, "rev-parse", "--verify", "-q", *tipFlag+"^{commit}")
	} else if !*noFetch {
		var present bool
		tip, present, err = goalBranchOriginTip(*root, endpoint, *goalID)
		if err == nil && !present {
			fmt.Println("no branch")
			return 0
		}
	} else {
		tip, err = goalBranchGit(*root, "rev-parse", "--verify", "-q", "refs/remotes/"+endpoint.Remote+"/goal/"+*goalID+"^{commit}")
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
	commits, err := branch.ValidateRange(*root, endpointTip, tip, *goalID)
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

func runGoalBranchCommit(args []string) int {
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
	endpoint, err := goalBranchEndpoint(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_, linked := linkedWorktreeMainCheckout(*root)
	if err := branch.CheckCommitCheckout(*root, *goalID, linked); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	check := goalBranchClaimCheck(*root, *goalID, endpoint)
	if err := branch.CheckCommitAccess(*goalID, check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	endpointTip, err := goalBranchEndpointTip(*root, endpoint)
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
		err = withGoalBranchCommitToken(*root, func() error {
			var commitErr error
			commit, _, commitErr = branch.CommitRead(branch.CommitReadRequest{
				Repo: *root, Remote: endpoint.Remote, EndpointTip: endpointTip, GoalID: *goalID, Units: units, OpID: operationID,
				CheckClaim: check, RootJob: *rootJob, ReaderRecord: *readerRecord, Carry: *carry,
				GateRunID: *gateRun, GateTree: *gateTree, TestsChanged: tests,
			})
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
	err = withGoalBranchCommitToken(*root, func() error {
		var commitErr error
		commit, commitErr = branch.CommitStaged(branch.CommitRequest{
			Repo: *root, Remote: endpoint.Remote, EndpointTip: endpointTip, GoalID: *goalID, Units: units, OpID: operationID,
			Kind: branch.Kind(*kindName), Amend: *amend, CheckClaim: check,
		})
		return commitErr
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(commit)
	return 0
}

func withGoalBranchCommitToken(root string, commit func() error) error {
	pid := int64(os.Getpid())
	if _, err := lease.RequireHolder(goalBranchHolderRoot(root), pid, nil); err != nil {
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
			GoalID: *goalID, CheckClaim: check,
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
