package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestCarryAndFullSuitePublicAdmission(t *testing.T) {
	t.Parallel()
	t.Run("build-revise", func(t *testing.T) {
		t.Parallel()
		b := declaredCheckBed(t, "proof.cheap=printf full\nproof.audits=true\nproof.deadline=15\n")
		old := b.workOwnersHook
		full := "printf full"
		b.workOwnersHook = func(w *intentWorkOwners) {
			old(w)
			git := w.git
			w.git = func(root string, args ...string) ([]byte, error) {
				data, err := git(root, args...)
				if err == nil && args[0] == "show" {
					data = []byte(strings.ReplaceAll(string(data), "proof.full=true", "proof.full="+full))
				}
				return data, err
			}
		}
		owners := b.workOwners()
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		brief := b.brief("full.md", "Build.\n")
		args := []string{"work", "build", b.id, "--work", "full", "--brief", brief, "--lines", "5"}
		code, held := checkActBuild(t, b, owners, args...)
		act := processAct(t, held)
		if code != 1 || act.Class != "full-suite-exception" || act.BeforeDeclaration.Values != act.AfterDeclaration.Values || len(act.FullArgv) != 3 || !slices.Equal(act.FullArgv, act.SelectedArgv) || len(b.starter.launched()) != 0 {
			t.Fatalf("unchanged full selection launched: %d %+v", code, held)
		}
		owners.prove = fixedFixtureGoalAuthority
		if code, result := checkActBuild(t, b, owners, held.Next.Argv[1:]...); code != 1 || len(b.starter.launched()) != 0 {
			t.Fatalf("agent exception: %d %+v", code, result)
		}
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		code, built := checkActBuild(t, b, owners, held.Next.Argv[1:]...)
		if code != 0 {
			t.Fatalf("full remedy: %d %+v", code, built)
		}
		checkActLaunches(t, b, built, act.ID)
		count := len(b.starter.launched())
		if code, replay := checkActBuild(t, b, owners, held.Next.Argv[1:]...); code != 0 || len(b.starter.launched()) != count {
			t.Fatalf("full replay: %d %+v", code, replay)
		}
		q, err := channel.ReadQuestion(b.root(), act.Question)
		if err != nil || q.State != "closed" {
			t.Fatalf("exception closure: %+v %v", q, err)
		}
		checkNextWorktree(t, b)
		owners = b.workOwners()
		other := slices.Clone(args)
		other[4] = "other"
		code, otherHeld := checkActBuild(t, b, owners, other...)
		if code != 1 || processAct(t, otherHeld).ID == act.ID || len(b.starter.launched()) != count {
			t.Fatalf("another unit reused exception: %d %+v", code, otherHeld)
		}
		wrong := append(slices.Clone(other), "--act", act.ID)
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		beforeWrong := len(b.starter.launched())
		if code, wrongResult := checkActBuild(t, b, owners, wrong...); code != 1 || len(b.starter.launched()) != beforeWrong {
			t.Fatalf("wrong subject: %d %+v", code, wrongResult)
		}
		// A person's explicit command is the exception, without a second approval.
		checkNextWorktree(t, b)
		manual := []string{"work", "build", b.id, "--work", "manual", "--brief", brief, "--lines", "5", "--reason", "Run full suite once", "--by", "Wido", "--check", "printf", "full"}
		code, manualResult := checkActBuild(t, b, owners, manual...)
		if code != 0 {
			t.Fatalf("direct exception: %d %+v", code, manualResult)
		}
		plan, err := launch.ReadUnitPlan(resultData(t, manualResult)["plan"].(string))
		if err != nil || plan.Check.ProcessAct == "" {
			t.Fatalf("manual consumed act: %+v %v", plan, err)
		}
		acts := checkActs(t, b)
		if !slices.ContainsFunc(acts, func(a processchange.ProcessAct) bool {
			return a.ID == plan.Check.ProcessAct && a.Class == "full-suite-exception" && a.Status == "applied"
		}) {
			t.Fatalf("manual exception absent: %+v", acts)
		}
		// Distinct ordered shell arguments remain an ordinary declaration.
		checkNextWorktree(t, b)
		full = "printf full-different"
		owners = b.workOwners()
		code, distinct := checkActBuild(t, b, owners, "work", "build", b.id, "--work", "distinct", "--brief", brief, "--lines", "5")
		if code != 0 {
			t.Fatalf("name classified full: %d %+v", code, distinct)
		}
		// The next round needs its own exception despite an unchanged tuple.
		record, _ := (&launch.UnitRunner{Root: b.unitRoot}).Status(resultData(t, built)["run"].(string))
		b.worktree = record.Worktree
		run := resultData(t, built)["run"].(string)
		full = "printf full"
		owners = b.workOwners()
		code, revised := checkActBuild(t, b, owners, "work", "revise", "run:"+run, "--brief", b.brief("next.md", "Repair.\n"), "--reason", "Next round", "--by", "Wido")
		if code != 1 || processAct(t, revised).Class != "full-suite-exception" {
			t.Fatalf("revision full selection was not held: %d %+v", code, revised)
		}
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		code, revised = checkActBuild(t, b, owners, revised.Next.Argv[1:]...)
		if code != 0 {
			t.Fatalf("revision remedy: %d %+v", code, revised)
		}
		next, err := launch.ReadUnitPlan(resultData(t, revised)["plan"].(string))
		if err != nil || next.Check.ProcessAct == act.ID || next.Check.ProcessAct == "" {
			t.Fatalf("revision reused exception: %+v %v", next, err)
		}
	})
	t.Run("carry-subject", func(t *testing.T) {
		t.Parallel()
		b, owners, _ := rebaseIntentBed(t)
		initialGoal := b.id
		script := filepath.Join(t.TempDir(), "check")
		if err := testexec.WriteFile(script, []byte("#!/bin/sh\nprintf 'subject checked'\n"), 0700); err != nil {
			t.Fatal(err)
		}
		mainContent := "proof.cheap=printf seed\nproof.audits=true\nproof.deadline=15\nproof.full=printf full\n"
		cheap, full, unavailable := script, script, false
		content := func() string {
			return "proof.cheap=" + cheap + "\nproof.audits=true\nproof.deadline=15\nproof.full=" + full + "\n"
		}
		subject := branch.AttestationSubject{Commit: strings.Repeat("c", 40), Tree: strings.Repeat("d", 40), Parent: "seed"}
		owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
		original := owners.work.git
		owners.work.git = func(root string, args ...string) ([]byte, error) {
			switch {
			case len(args) == 4 && args[0] == "show" && args[1] == "-s":
				id := initialGoal
				if args[3] == strings.Repeat("b", 40) {
					id = "carry-b"
				}
				return []byte("Goal-Unit: " + id + "/u1"), nil
			case slices.Equal(args, []string{"rev-parse", "HEAD"}):
				return []byte(subject.Commit), nil
			case slices.Equal(args, []string{"rev-parse", "origin/main"}):
				return []byte("seed"), nil
			case args[0] == "merge-base":
				return []byte("seed"), nil
			case args[0] == "rev-parse" && strings.HasSuffix(args[1], "^{tree}"):
				return []byte(subject.Tree), nil
			case args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf"):
				if unavailable {
					return nil, fmt.Errorf("committed classification unavailable")
				}
				if strings.HasPrefix(args[1], "seed:") {
					return []byte(mainContent), nil
				}
				return []byte(content()), nil
			case args[0] == "status":
				return nil, nil
			}
			return original(root, args...)
		}
		// The branch boundary supplies the real subject gate and the same
		// declaration-unavailable result that carry retains as NeedsReview.
		owners.connection.rebase = func(req branch.RebaseRequest) (branch.RebaseResult, error) {
			gate, err := branch.ResolveReadGate(branch.ReadGateRequest{Repo: b.worktree, GoalID: b.id, UnitCommit: subject.Commit, SubjectCheck: req.SubjectCheck, Repository: owners.connection.readRepository})
			result := branch.RebaseResult{State: "held", NeedsReview: []string{}, Carried: []string{}}
			if err != nil {
				var unavailable *branch.DeclarationUnavailableError
				if !errors.As(err, &unavailable) {
					return result, err
				}
				result.NeedsReview = []string{"u1"}
				result.ReviewReasons = map[string]string{"u1": err.Error()}
			} else {
				if gate.RunID == "" {
					t.Fatal("no actual execution")
				}
				result.Carried = []string{"u1"}
			}
			return result, nil
		}
		code, held := b.runJSON(owners, "work", "rebase", b.id)
		acts := checkActs(t, b)
		if code != 0 || len(acts) != 1 || acts[0].Status != "proposed" || acts[0].Class != "full-suite-exception" {
			t.Fatalf("carry hold: %d %+v acts=%+v", code, held, acts)
		}
		paths, _ := filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", "*", "check-*"))
		if len(paths) != 0 {
			t.Fatalf("carry launched before approval: %v", paths)
		}
		q, err := channel.ReadQuestion(b.root(), acts[0].Question)
		if err != nil {
			t.Fatal(err)
		}
		// The retained public remedy fixes this commit, never a mutable unit name.
		remedy := []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--repo", b.root(), "--check-only", "--act", acts[0].ID}
		if q.Wants != shellCommand(append([]string{"metasystem"}, remedy...)) {
			t.Fatalf("retained remedy: %q", q.Wants)
		}
		if code, result := checkActBuild(t, b, owners, remedy...); code != 1 {
			t.Fatalf("agent used carry act: %d %+v", code, result)
		}
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		if err := os.MkdirAll(acts[0].Operation, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(acts[0].Operation, 0500); err != nil {
			t.Fatal(err)
		}
		if code, result := checkActBuild(t, b, owners, remedy...); code != 1 {
			t.Fatalf("interrupted carry publication: %d %+v", code, result)
		}
		q, _ = channel.ReadQuestion(b.root(), acts[0].Question)
		if q.State != "open" {
			t.Fatal("question closed before subject publication")
		}
		if err := os.Chmod(acts[0].Operation, 0700); err != nil {
			t.Fatal(err)
		}
		if code, result := checkActBuild(t, b, owners, remedy...); code != 0 {
			t.Fatalf("carry remedy: %d %+v", code, result)
		}
		q, err = channel.ReadQuestion(b.root(), acts[0].Question)
		if err != nil || q.State != "closed" {
			t.Fatalf("carry question: %+v %v acts=%+v", q, err, checkActs(t, b))
		}
		gatePath := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", subject.Commit, "gate.json")
		if err := os.Remove(gatePath); err != nil {
			t.Fatal(err)
		}
		if code, result := b.runJSON(owners, "work", "rebase", b.id); code != 0 {
			t.Fatalf("carry replay: %d %+v", code, result)
		}
		paths, _ = filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", "*", "check-*", "result.json"))
		if len(paths) != 1 {
			t.Fatalf("duplicate carry check: %v", paths)
		}
		raw, _ := os.ReadFile(paths[0])
		var executed struct {
			Check launch.UnitCheck
			Exits []launch.CheckExit
		}
		if json.Unmarshal(raw, &executed) != nil || executed.Check.ProcessAct != acts[0].ID || executed.Check.SourceTree != subject.Tree || executed.Exits[0].Output != "subject checked" {
			t.Fatalf("carry consumed identity: %s", raw)
		}
		originalGoal := b.id
		second := b.goalFile(b.id)
		second.Id = "carry-b"
		goalPath := "plans/goals/carry-b.md"
		body := goal.RenderFile(second)
		if err := os.WriteFile(filepath.Join(b.root(), goalPath), body, 0600); err != nil {
			t.Fatal(err)
		}
		for id, commit := range b.repo.commits {
			commit.files[goalPath] = body
			b.repo.commits[id] = commit
		}
		for index, id := range []string{originalGoal, "carry-b", originalGoal, "carry-b"} {
			b.id = id
			cheap, full = script+" "+id, "printf full"
			subject.Commit = strings.Repeat("a", 40)
			if id == "carry-b" {
				subject.Commit = strings.Repeat("b", 40)
			}
			owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
			owners.prove = fixedFixtureGoalAuthority
			code, result := b.runJSON(owners, "work", "rebase", b.id)
			if code != 0 {
				t.Fatalf("alternating carry: %d %+v", code, result)
			}
			current := checkActs(t, b)
			pending := slices.DeleteFunc(slices.Clone(current), func(a processchange.ProcessAct) bool { return a.Status != "proposed" })
			if index < 2 {
				if len(pending) != 1 || pending[0].Class != "declaration" || pending[0].BeforeDeclaration.Values[0] != map[bool]string{true: script, false: "printf seed"}[id == originalGoal] {
					t.Fatalf("carry used another goal's history: %+v", pending)
				}
				owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
				remedy := []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--repo", b.root(), "--check-only", "--act", pending[0].ID}
				if code, result := checkActBuild(t, b, owners, remedy...); code != 0 {
					t.Fatalf("alternating remedy: %d %+v", code, result)
				}
			} else if len(pending) != 0 {
				t.Fatalf("unchanged alternating carry raised an act: %+v", pending)
			}
		}
		b.id = originalGoal
		subject.Commit = strings.Repeat("c", 40)
		owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
		// A frozen subject resumes even if the advisory goal reference is damaged.
		baseline := filepath.Join(b.root(), "process", fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte(b.id))))
		if err := os.WriteFile(baseline, []byte("damaged"), 0600); err != nil {
			t.Fatal(err)
		}
		if code, result := checkActBuild(t, b, owners, remedy...); code != 0 {
			t.Fatalf("damaged carry resume: %d %+v", code, result)
		}
		snapshot := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", subject.Commit, "subject.json")
		if err := os.WriteFile(snapshot, []byte("damaged snapshot"), 0600); err != nil {
			t.Fatal(err)
		}
		unavailable = true
		owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
		repair := []string{"work", "review", "--commit", subject.Commit, "--goal", b.id, "--repo", b.root(), "--check-only", "--reason", "Repair missing classification", "--by", "Wido", "--check", script}
		if code, result := checkActBuild(t, b, owners, repair...); code != 0 {
			t.Fatalf("carry explicit repair: %d %+v", code, result)
		}
		if !slices.ContainsFunc(checkActs(t, b), func(a processchange.ProcessAct) bool {
			return a.Measure == "full-suite classification unknown" && a.Status == "applied"
		}) {
			t.Fatal("repair invented a classification")
		}
		preserved, _ := filepath.Glob(snapshot + ".damaged-*")
		if len(preserved) != 1 {
			t.Fatalf("damaged snapshot not preserved: %v", preserved)
		}

	})
}

type carryFullRepository struct {
	root, common string
	subject      branch.AttestationSubject
}

func (r carryFullRepository) Range(_, _, _, _ string) ([]branch.Commit, error) {
	return nil, fmt.Errorf("range unavailable")
}
func (r carryFullRepository) Subject(repo, commit string) (branch.AttestationSubject, error) {
	if repo != r.root {
		return branch.AttestationSubject{}, fmt.Errorf("the subject was requested from another checkout")
	}
	if commit != r.subject.Commit {
		return branch.AttestationSubject{}, fmt.Errorf("different subject")
	}
	return r.subject, nil
}
func (r carryFullRepository) CommonDir(_ string) (string, error) { return r.common, nil }
func (r carryFullRepository) Entries(_, _ string) ([]branch.Entry, error) {
	return nil, fmt.Errorf("entries unavailable")
}
func (r carryFullRepository) Detached(_, _ string) (string, func() error, error) {
	return r.root, func() error { return nil }, nil
}

func TestDeclaredResumeAndRepairPublicBuild(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.starter.hold = "build"
	owners := b.workOwners()
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	args := []string{"work", "build", b.id, "--work", "resume", "--brief", b.brief("resume.md", "Build.\n"), "--lines", "5"}
	code, built := checkActBuild(t, b, owners, args...)
	if code != 3 {
		t.Fatalf("incomplete admitted round: %d %+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	baseline := filepath.Join(b.root(), "process", fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte(b.id))))
	if err := os.WriteFile(baseline, []byte("damaged evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	owners = b.workOwners()
	count := len(b.starter.launched())
	if code, resumed := checkActBuild(t, b, owners, "work", "build", "run:"+run); code != 3 || len(b.starter.launched()) != count {
		t.Fatalf("damaged admitted resume: %d %+v", code, resumed)
	}
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	code, repaired := checkActBuild(t, b, owners, "work", "build", "run:"+run, "--reason", "Repair unavailable declarations", "--by", "Wido", "--check", "false")
	if code != 3 || len(b.starter.launched()) != count {
		t.Fatalf("person retained repair: %d %+v", code, repaired)
	}
	plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
	if err != nil || plan.Check.Cheap != "false" || plan.Check.Audits != "true" || plan.Check.ProcessAct == "" {
		t.Fatalf("repair didn't replace check: %+v %v", plan.Check, err)
	}
	raw, _ := os.ReadFile(baseline)
	if string(raw) != "damaged evidence" {
		t.Fatal("manual repair fabricated declaration baseline")
	}
}
