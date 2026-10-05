package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func writeUnitCarryFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func unitCarryIntentBed(t *testing.T, amended bool) (*workBed, intentOwners, string, *launch.UnitRunner) {
	t.Helper()
	b, owners, _ := rebaseIntentBed(t)
	brief := b.brief("carry.md", "Build the unit.\n")
	code, built, _ := b.work(append([]string{"work", "build", b.id, "u1", "--brief", brief, "--lines", "5"}, workCheck...)...)
	if code != 0 {
		t.Fatalf("build = %+v, code = %d", built, code)
	}
	run := resultData(t, built)["run"].(string)
	runner := owners.work.units(stateroot.Layout{})
	err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		subject := launch.UnitSubject{Round: review.Round.Number, Operation: "carry-op", DiffDigest: review.DiffDigest,
			Commit: "corrected-commit", Tip: "replayed-tip"}
		if amended {
			subject.Amends = "earlier-commit"
		}
		return retain(subject)
	})
	if err != nil {
		t.Fatal(err)
	}
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
		return branch.BranchReadResult{RootJob: "critic"}, nil
	}
	owners.connection.commit = func(branch.CommitRequest) (string, error) {
		t.Fatal("the retained unit was committed again")
		return "", nil
	}
	owners.connection.push = func(branch.PushRequest) (branch.PushResult, error) {
		return branch.PushResult{Tip: "published-tip"}, nil
	}
	owners.delivery = &intentDeliveryOwners{
		branchRead: func(args []string) (branch.BranchReadResult, int, error) {
			if flagValue(args, "--unit") != "corrected-commit" || slices.Contains(args, "--unit-read") {
				t.Fatalf("corrected unit's critic = %v", args)
			}
			return branch.BranchReadResult{State: "collected", AttestationCommit: "read-commit"}, 0, nil
		},
		publishRead: func(string, string, string) (branch.PublishReadResult, error) {
			return branch.PublishReadResult{State: "current"}, nil
		},
	}
	return b, owners, run, runner
}

func TestUnitReviewAmendCarriesReviews(t *testing.T) {
	t.Parallel()
	b, owners, run, _ := unitCarryIntentBed(t, true)
	carried, tokens, gates := 0, 0, 0
	inside := false
	owners.connection.commitToken = func(root string, fn func() error) error {
		if root != b.worktree {
			t.Fatalf("token repository = %s", root)
		}
		tokens++
		inside = true
		defer func() { inside = false }()
		return fn()
	}
	owners.connection.rebaseGate = func(root string) (string, error) {
		gates++
		return "checks passed for " + root, nil
	}
	owners.connection.carry = func(req branch.CarryRequest) (branch.CarryResult, error) {
		carried++
		if !inside || req.Repo != b.worktree || req.GoalID != b.id || req.Remote != "origin" || req.EndpointTip != strings.Repeat("a", 40) || req.CheckClaim == nil || req.Transport == nil {
			t.Fatalf("carry request = %+v, token = %t", req, inside)
		}
		if err := req.CheckClaim(); err != nil {
			t.Fatal(err)
		}
		if got, err := req.Gate("later-tree"); err != nil || got != "checks passed for later-tree" {
			t.Fatalf("gate = %s, error = %v", got, err)
		}
		return branch.CarryResult{NewTip: "carried-tip", Carried: []string{"u2", "u3"}}, nil
	}
	owners.connection.push = func(req branch.PushRequest) (branch.PushResult, error) {
		if carried != 1 || inside || req.Repo != b.worktree {
			t.Fatalf("push before carry = %+v, calls = %d", req, carried)
		}
		return branch.PushResult{Tip: "carried-tip"}, nil
	}
	code, result := b.runJSON(owners, "work", "review", "run:"+run)
	if code != 0 || carried != 1 || tokens != 1 || gates != 1 || !slices.Equal(resultData(t, result)["carried"].([]any), []any{"u2", "u3"}) {
		t.Fatalf("review = %+v, code = %d, carry = %d, tokens = %d", result, code, carried, tokens)
	}
	_, _, text := b.run(owners, "work", "review", "run:"+run)
	if carried != 1 || tokens != 1 {
		t.Fatalf("published review repeated carry: %s", text)
	}
}

func TestUnitReviewFirstUnitDoesNotCarryReviews(t *testing.T) {
	t.Parallel()
	b, owners, run, _ := unitCarryIntentBed(t, false)
	owners.connection.carry = func(branch.CarryRequest) (branch.CarryResult, error) {
		t.Fatal("a first unit carried reviews")
		return branch.CarryResult{}, nil
	}
	code, result := b.runJSON(owners, "work", "review", "run:"+run)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("first review = %+v, code = %d", result, code)
	}
}

func TestUnitReviewCarryFailureRetriesBeforePush(t *testing.T) {
	t.Parallel()
	b, owners, run, runner := unitCarryIntentBed(t, true)
	carried, pushed := 0, 0
	owners.connection.commitToken = func(_ string, fn func() error) error { return fn() }
	owners.connection.carry = func(branch.CarryRequest) (branch.CarryResult, error) {
		carried++
		if carried == 1 {
			return branch.CarryResult{Carried: []string{"u2"}}, errors.New("checks could not run")
		}
		return branch.CarryResult{Carried: []string{}}, nil
	}
	owners.connection.push = func(branch.PushRequest) (branch.PushResult, error) {
		pushed++
		return branch.PushResult{Tip: "carried-tip"}, nil
	}
	code, failed := b.runJSON(owners, "work", "review", "run:"+run)
	if code == 0 || failed.Outcome != intentPartial || carried != 1 || pushed != 0 || failed.Next == nil || !slices.Contains(failed.Next.Argv, "run:"+run) {
		t.Fatalf("failed carry = %+v, code = %d, pushes = %d", failed, code, pushed)
	}
	record, err := runner.Status(run)
	if err != nil || len(record.Subjects) != 1 || record.Subjects[0].Commit != "corrected-commit" || record.Subjects[0].Published != "" {
		t.Fatalf("retained subject = %+v, error = %v", record.Subjects, err)
	}
	code, retried := b.runJSON(owners, "work", "review", "run:"+run)
	if code != 0 || retried.Outcome != intentConfirmed || carried != 2 || pushed != 1 {
		t.Fatalf("retried carry = %+v, code = %d, carry = %d, pushes = %d", retried, code, carried, pushed)
	}
}

// The public review uses the real carry and push owners to observe the Git records they publish.
func TestUnitReviewAmendCarriesReviewsGitAdapter(t *testing.T) {
	t.Parallel()
	b, owners, run, runner := unitCarryIntentBed(t, true)
	repo := b.worktree
	connectionGit(t, repo, "init", "-q", "-b", "main")
	connectionGit(t, repo, "config", "user.name", "fixture")
	connectionGit(t, repo, "config", "user.email", "fixture@example.invalid")
	connectionGit(t, repo, "commit", "-qm", "base", "--allow-empty")
	base := connectionGit(t, repo, "rev-parse", "HEAD")
	remote := filepath.Join(t.TempDir(), "origin.git")
	connectionGit(t, filepath.Dir(remote), "init", "-q", "--bare", remote)
	connectionGit(t, repo, "remote", "add", "origin", remote)
	var first, later, previous string
	for _, unit := range []string{"u1", "u2"} {
		writeUnitCarryFile(t, filepath.Join(repo, unit+".go"), "one\n")
		connectionGit(t, repo, "add", unit+".go")
		commit, err := branch.CommitStaged(branch.CommitRequest{Repo: repo, Remote: "origin", EndpointTip: base,
			GoalID: b.id, Unit: unit, OpID: "commit-" + unit, Kind: branch.Unit, CheckClaim: func() error { return nil }})
		if err != nil {
			t.Fatal(err)
		}
		if unit == "u1" {
			first = commit
		}
		change, err := branch.UnitDigest(repo, commit)
		if err != nil {
			t.Fatal(err)
		}
		path := "metasystem/records/misc/" + unit + "-read.md"
		writeUnitCarryFile(t, filepath.Join(repo, path), "Reviewed commit "+commit+".\nChange "+change+".\nFound it clean.\n")
		read, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: repo, Remote: "origin", EndpointTip: base,
			GoalID: b.id, Unit: unit, OpID: "read-" + unit, ReaderRecord: path, CheckClaim: func() error { return nil },
			GateRunID: "first-gate", GateTree: connectionGit(t, repo, "rev-parse", commit+"^{tree}")})
		if err != nil {
			t.Fatal(err)
		}
		later, previous = commit, read
	}
	writeUnitCarryFile(t, filepath.Join(repo, "u1.go"), "corrected\n")
	connectionGit(t, repo, "add", "u1.go")
	tip, err := branch.CommitStaged(branch.CommitRequest{Repo: repo, Remote: "origin", EndpointTip: base,
		GoalID: b.id, Unit: "u1", OpID: "correct-u1", Kind: branch.Unit, Amend: true, CheckClaim: func() error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	commits, err := branch.ValidateRange(repo, base, tip, b.id)
	if err != nil {
		t.Fatal(err)
	}
	corrected := commits[0].ID
	if err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		subject := *review.Subject
		subject.Commit, subject.Tip, subject.Amends = corrected, tip, first
		return retain(subject)
	}); err != nil {
		t.Fatal(err)
	}
	owners.connection.carry, owners.connection.push = nil, nil
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return base, nil }
	owners.connection.commitToken = func(_ string, fn func() error) error { return fn() }
	owners.connection.rebaseGate = func(string) (string, error) { return "checks passed", nil }
	critics := 0
	owners.delivery.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		critics++
		if flagValue(args, "--unit") != corrected || slices.Contains(args, "--unit-read") {
			t.Fatalf("corrected unit's critic = %v", args)
		}
		return branch.BranchReadResult{}, 1, &branch.ReadNeverLaunchedError{Err: errors.New("critic requested")}
	}
	_, stdout, stderr := b.run(owners, "work", "review", "run:"+run)
	if !strings.Contains(stdout+stderr, "review carried: u2") || critics != 1 {
		t.Fatalf("review output = %s%s, critics = %d", stdout, stderr, critics)
	}
	published := connectionGit(t, repo, "rev-parse", "HEAD")
	status, err := branch.InspectStatus(repo, base, published, b.id)
	if err != nil || len(status.Units) != 2 || status.Units[0].ReadState == "read clean" || status.Units[1].ReadState != "read clean" {
		t.Fatalf("hand-in status = %+v, error = %v", status, err)
	}
	att, err := branch.ValidateAttestationAt(repo, published, base, b.id, "u2", status.Units[1].Commit)
	if err != nil || att.Carry == nil || att.Carry.FromCommit != later || att.Carry.ToCommit != status.Units[1].Commit {
		t.Fatalf("carried review = %+v, error = %v", att, err)
	}
	if got := connectionGit(t, remote, "rev-parse", "refs/metasystem/goals/before/"+b.id+"/"+previous); got != previous {
		t.Fatalf("published kept tip = %s, want %s", got, previous)
	}
}
