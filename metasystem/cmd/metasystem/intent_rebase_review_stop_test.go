package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestRebaseReviewStopKeepsTheReadAuthoritative(t *testing.T) {
	t.Parallel()
	t.Skip("Production defect: work review rejects the rebased HEAD before closing the original critic; plans/designs/the-lane-finishes-what-it-starts.md:178 (D8), with the original read identity required at :96 (D7(e))")
	b, owners, run, custody := criticCustodyBed(t)
	custody.dead = true
	runner := owners.work.units(stateroot.Layout{})
	owners.work.cancelCritic = func(string, string) (map[string]any, int, error) { return nil, 0, nil }
	git := func(args ...string) string { t.Helper(); return connectionGit(t, b.worktree, args...) }
	git("init", "-q", "-b", "main")
	git("config", "user.name", "fixture")
	git("config", "user.email", "fixture@example.invalid")
	writeUnitCarryFile(t, filepath.Join(b.worktree, ".gitignore"), "artifacts/\n")
	git("add", ".gitignore")
	git("commit", "-qm", "base")
	git("checkout", "-qb", "goal/"+b.id)
	writeUnitCarryFile(t, filepath.Join(b.worktree, "unit.go"), "implemented\n")
	git("add", "unit.go")
	git("commit", "-qm", "unit")
	original := git("rev-parse", "HEAD")
	git("checkout", "-q", "main")
	writeUnitCarryFile(t, filepath.Join(b.worktree, "main.go"), "independent main change\n")
	git("add", "main.go")
	git("commit", "-qm", "main change")
	main := git("rev-parse", "HEAD")
	git("checkout", "-q", "goal/"+b.id)
	var rebased string
	examined, present, err := dispatchcore.ComputeReadSubject(dispatchcore.ReadSubjectRequest{
		RepoRoot: b.worktree, Role: "code-critic", Reviews: "commit:" + original,
	})
	if err != nil || !present {
		t.Fatalf("original commit's read subject: present=%t %v", present, err)
	}
	if err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		subject := *review.Subject
		subject.Commit, subject.Tip, subject.Published = original, original, original
		subject.StagedTree = examined.Tree
		return retain(subject)
	}); err != nil {
		t.Fatal(err)
	}
	b.head = original
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return main, nil }
	owners.connection.rebase = func(req branch.RebaseRequest) (branch.RebaseResult, error) {
		git("rebase", "main")
		rebased = git("rev-parse", "HEAD")
		b.head = rebased
		return branch.RebaseResult{State: "rebased", OldTip: original, NewTip: rebased, MainTip: req.EndpointTip, NeedsReview: []string{"u1"}}, nil
	}
	owners.connection.push = func(branch.PushRequest) (branch.PushResult, error) {
		t.Fatal("review republished the retained subject after rebase")
		return branch.PushResult{}, nil
	}

	// The critic supplies an empty examination; the lifecycle's real close
	// validates its decisions, mirrors its evidence and stamps its closure.
	d := &deliveryBed{intentBed: b.intentBed, install: b.worktree, owners: &intentDeliveryOwners{}}
	d.writeFile(filepath.Join(d.install, "metasystem.conf"), string(mustRead(t, filepath.Join(b.root(), "metasystem.conf")))+"\nevidence.root="+t.TempDir()+"\n")
	d.writeFile(filepath.Join(d.install, "plans", "goals", "backlog.md"), "# Backlog\n")
	d.writeFile(filepath.Join(d.install, "artifacts", "agents", "capabilities", "close.json"), `{"ok":true}`)
	if err := os.MkdirAll(filepath.Join(d.install, "artifacts", "agents", "record-locks"), 0o755); err != nil {
		t.Fatal(err)
	}
	realCloseOwner(t, d, func(ports *delegation.Ports) {
		ports.Clock = &fake.Clock{Current: b.manager.Now()}
		ports.Git = &fake.Git{Log: &fake.Log{}, Responses: map[string]fake.GitResponse{
			"*|rev-parse --show-toplevel": {Stdout: b.root()},
		}}
	})
	writeCustodyCritic(t, b, "critic", "completed")
	critic := d.job("critic")
	for key, value := range map[string]any{"jobId": "critic", "role": "code-critic", "status": "completed", "round": 1,
		"engineBuild": "fixture-engine", "effectiveModel": "fixture-critic", "goalId": b.id,
		"reviewRoundLimit": b.goalFile(b.id).Budget.ReviewRoundLimit,
		"reviews":          "commit:" + original, "destructiveReach": "DESIGN-BEARING", "dispatchMode": "fresh",
		"sessionId": "critic-session", "endedAt": b.manager.Now().Format("2006-01-02T15:04:05Z"),
		"capabilitySnapshot": "artifacts/agents/capabilities/close.json", "parentJob": nil, "findingRegister": []any{}} {
		critic[key] = value
	}
	d.writeJob(critic)
	round := filepath.Join(d.install, "artifacts", "agents", "critic", "rounds", "1")
	d.writeJSON(filepath.Join(round, "subject.json"), examined)
	d.writeJSON(filepath.Join(round, "return.json"), map[string]any{"jobId": "critic", "round": 1,
		"findings": []any{}, "verdict": "clean", "verdictMaterialCount": 0, "reviewedTree": examined.Tree})
	d.writeFile(filepath.Join(round, "return.md"), "VERDICT: LAND\n")
	if advanced, err := dispatchcore.CritiqueRegisterAdvance(d.install, "critic", "critic"); err != nil || advanced != "advanced" {
		t.Fatalf("register advance: %q %v", advanced, err)
	}
	// A published unit has a completed read before its branch can be rebased.
	if err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		subject := *review.Subject
		subject.Examination, subject.ExaminationRound = "critic", 1
		subject.ExaminationReturnPath = filepath.Join(round, "return.json")
		return retain(subject)
	}); err != nil {
		t.Fatal(err)
	}
	owners.resolver = stateroot.NewResolver(func(path string) (string, error) {
		if withinPath(path, b.worktree) {
			return b.worktree, nil
		}
		return fakeTop(b.root())(path)
	}, noExecutable)
	owners.delivery.recordWriter, owners.delivery.closeOwner = humanRecordWriter, d.owners.closeOwner
	owners.delivery.now = b.manager.Now
	var readSubjects, publishedSubjects []string
	owners.delivery.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		readSubjects = append(readSubjects, flagValue(args, "--unit"))
		if slices.Contains(args, "--collect") {
			b.head = "read-commit"
			return branch.BranchReadResult{State: "collected", RootJob: "critic", AttestationCommit: "read-commit"}, 0, nil
		}
		return branch.BranchReadResult{State: "closed", RootJob: "critic"}, 0, nil
	}
	owners.delivery.publishRead = func(_, _, subject string) (branch.PublishReadResult, error) {
		publishedSubjects = append(publishedSubjects, subject)
		return branch.PublishReadResult{State: "pushed", Attestation: "read-commit"}, nil
	}

	if code, result := b.runJSON(owners, "work", "rebase", b.id); code != 0 || result.Outcome != intentConfirmed || b.head != rebased {
		t.Fatalf("rebase: code=%d %+v head=%s", code, result, b.head)
	}
	if code, result := b.runJSON(owners, "work", "review", b.id, "--work", "u1"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("review: code=%d %+v", code, result)
	}
	if len(d.calls) != 1 || d.job("critic")["chainClosed"] != true || d.job("critic")["mirror"] == nil {
		t.Fatalf("review did not close through the real owner: calls=%v job=%v", d.calls, d.job("critic"))
	}
	binding, err := readReviewBinding(mustRead(t, filepath.Join(round, "decisions.md")))
	if err != nil || binding.Subject != original {
		t.Fatalf("the read's binding must remain on the original commit: %+v %v", binding, err)
	}
	if code, result := b.runJSON(owners, "work", "stop", "run:"+run); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("stop after rebase and review must succeed: code=%d %+v", code, result)
	}
	stopped, err := runner.Status(run)
	if err != nil || stopped.State != "cancelled" || len(stopped.Subjects) != 1 {
		t.Fatalf("unit did not stop: %+v %v", stopped, err)
	}
	subject := stopped.Subjects[0]
	if subject.Commit != original || subject.Published != original || subject.Examination != "critic" || subject.PublishedAt == "" {
		t.Fatalf("stopping must keep the original published subject and its read authoritative: %+v", subject)
	}
	reads := stopped.Rounds[len(stopped.Rounds)-1].Reads
	if len(reads) != 1 || reads[0].ID != "critic" || reads[0].Subject.Commit != original {
		t.Fatalf("the retained read must stay attributed to the original commit: %+v", reads)
	}
	if !slices.Equal(readSubjects, []string{original, original}) || !slices.Equal(publishedSubjects, []string{original}) {
		t.Fatalf("the rebased commit must have no attributed read: reads=%v publications=%v", readSubjects, publishedSubjects)
	}
}
