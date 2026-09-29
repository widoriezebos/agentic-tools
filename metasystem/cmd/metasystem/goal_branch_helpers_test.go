package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The goal branch beds drive a goal's branch the way the build and the
// landing compose it in production (intentConnectionOwners,
// goalBranchReadRun, goalBranchLandPrepRun, goalBranchLandPushRun), and
// print what the retired goal branch verbs printed, so each bed reads its
// results unchanged. These are test code: the engine has no goal branch
// verb.

// goalBranchTestCommand runs one goal branch step: commit (--kind, --unit
// repeatable, --amend), push (--opid) or land-push (--prepared).
func goalBranchTestCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return 2
	}
	switch args[0] {
	case "commit":
		return goalBranchTestCommit(args[1:], stdout, stderr)
	case "push":
		return goalBranchTestPush(args[1:], stdout, stderr)
	case "land-push":
		result, endpoint, code, err := goalBranchLandPushRun(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return code
		}
		fmt.Fprintf(stdout, "landed %s endpoint=%s branch-deleted=%s\n", result.Landing, endpoint, result.Branch)
		return 0
	}
	fmt.Fprintln(stderr, "goal branch test step needs commit, push or land-push")
	return 2
}

type goalBranchTestUnits []string

func (units *goalBranchTestUnits) String() string { return fmt.Sprint([]string(*units)) }
func (units *goalBranchTestUnits) Set(value string) error {
	*units = append(*units, value)
	return nil
}

// goalBranchTestCommit commits the staged change on the goal's branch as the
// build does: the claim check, then branch.CommitStaged under the checkout's
// commit token inside its mutation section.
func goalBranchTestCommit(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("goal branch test commit", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	kind := flags.String("kind", "", "unit or plan")
	amend := flags.Bool("amend", false, "replace the named unit commit")
	var units goalBranchTestUnits
	flags.Var(&units, "unit", "unit name (repeatable)")
	if flags.Parse(args) != nil || *goalID == "" || *kind == "" || flags.NArg() != 0 {
		return 2
	}
	endpoint, err := goalBranchEndpoint(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	check := goalBranchClaimCheck(*root, *goalID, endpoint)
	if err := branch.CheckCommitAccess(*goalID, check); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	tip, err := goalBranchEndpointTip(*root, endpoint)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	operationID, err := branchOperationID()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var commit string
	err = withGoalBranchCommitTokenAt(*root, goalBranchHolderRoot(*root), func() error {
		var commitErr error
		commit, commitErr = branch.CommitStaged(branch.CommitRequest{Repo: *root, Remote: endpoint.Remote, EndpointTip: tip,
			GoalID: *goalID, Units: units, OpID: operationID, Kind: branch.Kind(*kind), Amend: *amend, CheckClaim: check})
		return commitErr
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, commit)
	return 0
}

// goalBranchTestPush pushes the goal's branch to its endpoint remote as the
// build does, the holder checked first.
func goalBranchTestPush(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("goal branch test push", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	opid := flags.String("opid", "", "operation id")
	if flags.Parse(args) != nil || *goalID == "" || flags.NArg() != 0 {
		return 2
	}
	endpoint, err := goalBranchEndpoint(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	check := goalBranchClaimCheck(*root, *goalID, endpoint)
	if err := branch.CheckHolder(check); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	tip, err := goalBranchEndpointTip(*root, endpoint)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *opid == "" {
		if *opid, err = branchOperationID(); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	result, err := branch.Push(branch.PushRequest{Repo: *root, Remote: endpoint.Remote, EndpointTip: tip, GoalID: *goalID, OpID: *opid, CheckClaim: check})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "%s %s\n", result.State, result.Tip)
	return 0
}

// runGoalBranchReadWith runs the branch read owner work review reaches and
// prints its result line.
func runGoalBranchReadWith(args []string, dependencies goalBranchReadDependencies, stdout, stderr io.Writer) int {
	result, code, err := goalBranchReadRun(args, dependencies)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return code
	}
	fmt.Fprintf(stdout, "state=%s root-job=%s gate-run=%s", result.State, result.RootJob, result.GateRunID)
	if result.AttestationCommit != "" {
		fmt.Fprintf(stdout, " attestation=%s", result.AttestationCommit)
	}
	fmt.Fprintln(stdout)
	return 0
}

// runGoalBranchLandPrepWith runs the land preparation owner work land
// reaches and prints its result line.
func runGoalBranchLandPrepWith(args []string, dependencies goalBranchLandPrepDependencies, stdout, stderr io.Writer) int {
	outcome, code, err := goalBranchLandPrepRun(args, dependencies)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return code
	}
	result := outcome.Result
	if outcome.Classification != "" {
		fmt.Fprintf(stdout, "classification=%s landing=%s candidate=%s endpoint=%s attempt=%s proof=%d\n",
			outcome.Classification, result.Landing, result.Candidate, result.Endpoint, result.Attempt, result.ProofNumber)
		return 0
	}
	fmt.Fprintf(stdout, "landing=%s candidate=%s endpoint=%s attempt=%s proof=%d\n",
		result.Landing, result.Candidate, result.Endpoint, result.Attempt, result.ProofNumber)
	return 0
}

var _ = testpolicy.Contract{}

// goalBranchTestCommitWith is goalBranchTestCommit over a raw bed's
// injected Git and ledger (goalBranchRawDependencies): the claim check the
// bed configures, then branch.CommitStagedWithInputs, the commit owner with
// the bed's facts and effects.
func goalBranchTestCommitWith(args []string, raw *goalBranchRawDependencies, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("goal branch test commit", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	kind := flags.String("kind", "", "unit or plan")
	amend := flags.Bool("amend", false, "replace the named unit commit")
	var units goalBranchTestUnits
	flags.Var(&units, "unit", "unit name (repeatable)")
	if flags.Parse(args) != nil || *goalID == "" || *kind == "" || flags.NArg() != 0 {
		return 2
	}
	endpoint, err := raw.endpoint(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	check := goalBranchClaimCheckWith(*root, *goalID, endpoint, raw.Config, raw.HolderRoot)
	if err := branch.CheckCommitAccess(*goalID, check); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	tip, err := raw.EndpointTip(*root, endpoint)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	operationID, err := branchOperationID()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var commit string
	err = withGoalBranchCommitTokenAt(*root, raw.HolderRoot(*root), func() error {
		var commitErr error
		commit, commitErr = branch.CommitStagedWithInputs(branch.CommitRequest{Repo: *root, Remote: endpoint.Remote, EndpointTip: tip,
			GoalID: *goalID, Units: units, OpID: operationID, Kind: branch.Kind(*kind), Amend: *amend, CheckClaim: check,
			Transport: raw.Transport}, raw.ReadInputs.Facts, raw.ReadInputs.Effects)
		return commitErr
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, commit)
	return 0
}
