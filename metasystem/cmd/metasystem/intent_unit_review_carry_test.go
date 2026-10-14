package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	dispatchlib "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
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
	code, built, _ := b.work(append([]string{"work", "build", b.id, "u1", "--brief", brief, "--lines", "5", "--reason", "exercise review carry", "--by", "Wido"}, workCheck...)...)
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
	b.head = "replayed-tip"
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
		return branch.BranchReadResult{RootJob: "critic"}, nil
	}
	owners.connection.commit = func(branch.CommitRequest) (string, error) {
		t.Fatal("the retained unit was committed again")
		return "", nil
	}
	owners.connection.push = func(branch.PushRequest) (branch.PushResult, error) {
		b.head = "published-tip"
		return branch.PushResult{Tip: b.head}, nil
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
		b.head = "carried-tip"
		return branch.PushResult{Tip: b.head}, nil
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
		b.head = "carried-tip"
		return branch.PushResult{Tip: b.head}, nil
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
	for _, source := range []string{"unit-read", "critic-root"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			for _, state := range []string{"unchanged", "changed", "lost", "corrupt"} {
				t.Run(state, func(t *testing.T) {
					t.Parallel()
					witnessCanonicalReviewCarry(t, source, state)
				})
			}
		})
	}
}

// Git must replay the later unit and persist its subject journal and attestation.
func TestUnitReviewAmendDeclaredCheckOverridesCachedGateGitAdapter(t *testing.T) {
	t.Parallel()
	witnessCanonicalReviewCarry(t, "unit-read", "cached gate")
}

func witnessCanonicalReviewCarry(t *testing.T, source, state string) {
	t.Helper()
	b, owners, run, runner := unitCarryIntentBed(t, true)
	originalGit := owners.work.git
	owners.work.git = func(root string, args ...string) ([]byte, error) {
		if root == b.root() {
			return originalGit(root, args...)
		}
		out, err := goalBranchGit(root, args...)
		return []byte(out), err
	}
	repo := b.worktree
	connectionGit(t, repo, "init", "-q", "-b", "main")
	connectionGit(t, repo, "config", "user.name", "fixture")
	connectionGit(t, repo, "config", "user.email", "fixture@example.invalid")
	declaration := "proof.cheap=true\nproof.audits=true\nproof.deadline=15\nproof.full=printf full-suite\n"
	if state == "cached gate" {
		declaration = "proof.cheap=printf 'cheap subject check\\n'; test -n \"$LANDING_PROOF_BASE\"; test \"$(cat u1.go)\" = corrected\nproof.audits=printf 'audit subject check\\n'; test -f u2.go\nproof.deadline=15\nproof.full=printf full-suite\n"
	}
	writeUnitCarryFile(t, filepath.Join(repo, "metasystem.conf"), declaration)
	connectionGit(t, repo, "add", "metasystem.conf")
	connectionGit(t, repo, "commit", "-qm", "base")
	base := connectionGit(t, repo, "rev-parse", "HEAD")
	remote := filepath.Join(t.TempDir(), "origin.git")
	connectionGit(t, filepath.Dir(remote), "init", "-q", "--bare", remote)
	connectionGit(t, repo, "remote", "add", "origin", remote)
	connectionGit(t, repo, "push", "-q", "origin", base+":refs/heads/main")
	var first, later, previous string
	var original branch.Attestation
	var originalBundle []byte
	shared := ""
	for i := 1; i <= 20; i++ {
		shared += fmt.Sprintf("line %d\n", i)
	}
	for _, unit := range []string{"u1", "u2"} {
		path, body := unit+".go", "one\n"
		if state == "changed" {
			path, body = "shared.go", shared
			if unit == "u2" {
				body = strings.Replace(body, "line 15\n", "second unit\n", 1)
			}
		}
		writeUnitCarryFile(t, filepath.Join(repo, path), body)
		connectionGit(t, repo, "add", path)
		if state == "cached gate" && unit == "u2" {
			declaration = "proof.cheap=printf 'cheap subject check\\n'; test -n \"$LANDING_PROOF_BASE\"; test \"$(cat u1.go)\" = corrected\nproof.audits=printf 'audit subject check\\n'; test -f u2.go\nproof.deadline=15\nproof.full=printf full-suite\n"
			writeUnitCarryFile(t, filepath.Join(repo, "metasystem.conf"), declaration)
			connectionGit(t, repo, "add", "metasystem.conf")
		}
		commit, err := branch.CommitStaged(branch.CommitRequest{Repo: repo, Remote: "origin", EndpointTip: base,
			GoalID: b.id, Unit: unit, OpID: "commit-" + unit, Kind: branch.Unit, CheckClaim: func() error { return nil }})
		if err != nil {
			t.Fatal(err)
		}
		if unit == "u1" {
			first = commit
		}
		subject, present, err := dispatchlib.ComputeReadSubject(dispatchlib.ReadSubjectRequest{RepoRoot: repo, Role: "code-critic", Reviews: "commit:" + commit})
		if err != nil || !present {
			t.Fatalf("subject: %+v %v", subject, err)
		}
		canonical, digest := (readsubject.Read{ID: "read-" + unit, Subject: subject, Engine: "engine-at-examination", Model: "reader-model", Findings: []readsubject.Finding{}, Output: "immutable-report"}).Canonical()
		req := branch.CommitReadRequest{Repo: repo, Remote: "origin", EndpointTip: base,
			GoalID: b.id, Unit: unit, OpID: "read-" + unit, CheckClaim: func() error { return nil },
			GateRunID: "first-gate", GateTree: subject.Tree}
		encode := func(value any) string {
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
		if source == "unit-read" {
			req.UnitRead = []byte(encode(branch.UnitReadBundle{SchemaVersion: 1, Goal: b.id, Commit: commit,
				UnitRun: "run-" + unit, Round: 1, ReadLaunch: "read-" + unit, ReadModel: "reader-model", BuildModel: "builder-model",
				ExaminedTree: subject.Tree, ExaminedBase: subject.Parent, VerdictLine: "VERDICT: land", Report: "VERDICT: land\n",
				LaunchRecord: `{"state":"completed","kind":"read","verdictCounts":true}`, CanonicalRead: canonical, ReadDigest: digest}))
		} else {
			req.RootJob = "critic-" + unit
			writeUnitCarryFile(t, filepath.Join(repo, "artifacts/agents/jobs", req.RootJob+".json"), encode(map[string]any{
				"jobId": req.RootJob, "role": "code-critic", "round": 1, "status": "completed", "reviews": "commit:" + commit,
				"goalId": b.id, "goalRevision": 1, "findingRegister": []any{}, "findingRegisterRound": 1,
				"findingRegisterSubjectDigest": subject.Digest(), "chainClosed": true,
				"closure": readsubject.Closure{CriticRoot: req.RootJob, Round: 1, Subject: subject, Mechanism: "clean"}, "read": json.RawMessage(canonical), "readDigest": digest}))
			writeUnitCarryFile(t, filepath.Join(repo, "artifacts/agents", req.RootJob, "rounds/1/subject.json"), encode(subject))
			writeUnitCarryFile(t, filepath.Join(repo, "artifacts/agents", req.RootJob, "rounds/1/return.json"), encode(map[string]any{"jobId": req.RootJob, "round": 1, "reviewedTree": subject.Tree}))
		}
		read, att, err := branch.CommitRead(req)
		if err != nil {
			t.Fatal(err)
		}
		later, previous = commit, read
		original = att
		originalBundle, err = os.ReadFile(filepath.Join(repo, "metasystem/records/reads", b.id, commit+".closure.json"))
		if err != nil {
			t.Fatal(err)
		}
	}
	path, correction := "u1.go", "corrected\n"
	if state == "changed" {
		path = "shared.go"
		correction = strings.Replace(strings.Replace(shared, "line 15\n", "second unit\n", 1), "line 13\n", "corrected\n", 1)
	}
	writeUnitCarryFile(t, filepath.Join(repo, path), correction)
	connectionGit(t, repo, "add", path)
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
	var carriedSubject, carriedTree, journalPath string
	if state == "cached gate" {
		for _, commit := range commits {
			if commit.Kind == branch.Unit && slices.Contains(commit.Units, "u2") {
				carriedSubject = commit.ID
			}
		}
		if carriedSubject == "" || carriedSubject == later {
			t.Fatal("the amendment did not replay the later unit")
		}
		carriedTree = connectionGit(t, repo, "rev-parse", carriedSubject+"^{tree}")
		journalPath = filepath.Join(repo, ".git", "metasystem", "goal-reads", b.id, carriedSubject+".json")
		writeUnitCarryFile(t, journalPath, fmt.Sprintf(`{"schemaVersion":1,"goal":%q,"unitCommit":%q,"tree":%q,"gateRunId":"cached-static-gate"}`, b.id, carriedSubject, carriedTree))
	}
	if err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		subject := *review.Subject
		subject.Commit, subject.Tip, subject.Amends = corrected, tip, first
		return retain(subject)
	}); err != nil {
		t.Fatal(err)
	}
	b.head = tip
	owners.connection.carry, owners.connection.push = nil, nil
	var carried branch.CarryResult
	owners.connection.carry = func(req branch.CarryRequest) (branch.CarryResult, error) {
		path := filepath.Join(repo, "metasystem/records/reads", b.id, later+".closure.json")
		if state == "lost" {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		} else if state == "corrupt" {
			writeUnitCarryFile(t, path, "corrupt predecessor")
		}
		var err error
		carried, err = branch.CarryReviews(req)
		b.head = connectionGit(t, repo, "rev-parse", "HEAD")
		return carried, err
	}
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return base, nil }
	owners.connection.commitToken = func(_ string, fn func() error) error { return fn() }
	owners.connection.rebaseGate = func(string) (string, error) { return "checks passed", nil }
	if state == "cached gate" {
		owners.connection.rebaseGate = func(string) (string, error) {
			t.Fatal("correction carry ran the static gate")
			return "", nil
		}
	}
	critics := 0
	owners.delivery.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		critics++
		if flagValue(args, "--unit") != corrected || slices.Contains(args, "--unit-read") {
			t.Fatalf("corrected unit's critic = %v", args)
		}
		return branch.BranchReadResult{}, 1, &branch.ReadNeverLaunchedError{Err: errors.New("critic requested")}
	}
	_, stdout, stderr := b.run(owners, "work", "review", "run:"+run)
	unchanged := state == "unchanged" || state == "cached gate"
	if critics != 1 || unchanged && !strings.Contains(stdout+stderr, "review carried: u2") {
		t.Fatalf("review output = %s%s, critics = %d, carry = %+v", stdout, stderr, critics, carried)
	}
	published := connectionGit(t, repo, "rev-parse", "HEAD")
	status, err := branch.InspectStatus(repo, base, published, b.id)
	if !unchanged {
		if err != nil || len(status.Units) != 2 || status.Units[1].ReadState == "read clean" || !slices.Contains(carried.NeedsReview, "u2") || len(carried.Carried) != 0 || !strings.Contains(stdout+stderr, "work review "+b.id+" --work u2") {
			t.Fatalf("unread work was carried or lacks its remedy: status=%+v carry=%+v err=%v output=%s%s", status, carried, err, stdout, stderr)
		}
		if (state == "lost" || state == "corrupt") && !slices.Equal(carried.Unknown, []string{"u2"}) {
			t.Fatalf("unknown evidence relabelled: %+v", carried)
		}
		return
	}
	if err != nil || len(status.Units) != 2 || status.Units[0].ReadState == "read clean" || status.Units[1].ReadState != "read clean" {
		_, proofErr := branch.ValidateAttestationAt(repo, published, base, b.id, "u2", status.Units[1].Commit)
		t.Fatalf("hand-in status = %+v, error = %v, proof = %v", status, err, proofErr)
	}
	att, err := branch.ValidateAttestationAt(repo, published, base, b.id, "u2", status.Units[1].Commit)
	if err != nil || att.Carry == nil || att.Carry.FromCommit != later || att.Carry.ToCommit != status.Units[1].Commit {
		t.Fatalf("carried review = %+v, error = %v", att, err)
	}
	if state == "cached gate" {
		if att.Subject.Commit != carriedSubject || att.Gate.Kind != "unit-check" || att.Gate.RunID == "cached-static-gate" || att.Gate.Tree != carriedTree {
			t.Fatalf("carried attestation did not use its subject's declared check: subject=%+v gate=%+v", att.Subject, att.Gate)
		}
		executions, err := filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", carriedSubject, "check-*", "result.json"))
		if err != nil || len(executions) != 1 {
			t.Fatalf("subject check executions=%v error=%v", executions, err)
		}
		var execution struct {
			ExecutionID string
			Check       launch.UnitCheck
			Exits       []launch.CheckExit
		}
		data, err := os.ReadFile(executions[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &execution); err != nil {
			t.Fatal(err)
		}
		if execution.ExecutionID != att.Gate.RunID || execution.Check.Base != att.Subject.Parent || execution.Check.SourceTree != carriedTree || len(execution.Exits) != 2 || execution.Exits[0].Exit != 0 || execution.Exits[1].Exit != 0 || !strings.Contains(execution.Exits[0].Output, "cheap subject check") || !strings.Contains(execution.Exits[1].Output, "audit subject check") {
			t.Fatalf("attestation does not bind a passing subject execution: %+v", execution)
		}
		var committed struct {
			ExecutionID string
			Check       launch.UnitCheck
			Exits       []launch.CheckExit
		}
		if err := json.Unmarshal([]byte(att.Gate.Evidence), &committed); err != nil || committed.ExecutionID != execution.ExecutionID || committed.Check.Base != att.Subject.Parent || committed.Check.SourceTree != carriedTree || committed.Check.Cheap != execution.Check.Cheap || committed.Check.Audits != execution.Check.Audits || !slices.Equal(committed.Exits, execution.Exits) {
			t.Fatalf("committed subject execution=%+v error=%v", committed, err)
		}
		data, err = os.ReadFile(journalPath)
		var journal struct{ GateRunID string }
		if err != nil || json.Unmarshal(data, &journal) != nil || journal.GateRunID != execution.ExecutionID {
			t.Fatalf("subject journal retained its cached gate: %s error=%v", data, err)
		}
	}
	var before, after readsubject.Read
	if json.Unmarshal(original.CanonicalRead, &before) != nil || json.Unmarshal(att.CanonicalRead, &after) != nil || after.ID == before.ID || after.CarriedFrom != before.ID || after.Subject.Commit != att.Subject.Commit || after.Subject.Tree != att.Subject.Tree || after.Engine != before.Engine || after.Model != before.Model || after.Output != before.Output || att.Source != original.Source {
		t.Fatalf("canonical carry: before=%+v after=%+v source=%+v", before, after, att.Source)
	}
	for _, commit := range []string{later, att.Subject.Commit} {
		bundle, err := os.ReadFile(filepath.Join(repo, "metasystem/records/reads", b.id, commit+".closure.json"))
		if err != nil || !bytes.Equal(bundle, originalBundle) {
			t.Fatalf("source bundle changed: %s %v", commit, err)
		}
	}
	if got := connectionGit(t, remote, "rev-parse", "refs/metasystem/goals/before/"+b.id+"/"+previous); got != previous {
		t.Fatalf("published kept tip = %s, want %s", got, previous)
	}
}
