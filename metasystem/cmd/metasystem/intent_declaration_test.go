package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestDeclaredChangePublicAdmission(t *testing.T) {
	t.Parallel()
	b := newWorkBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		file.Budget.AttemptLimit = 30
		file.Budget.ReservedJobMinutesLimit = 2000
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	b.manager.Supervisor = &stopReadStarter{bed: b, reads: [][]readsubject.Finding{{stopFinding("regression", "a.go")}}}
	aID, bID := b.id, "goal-b"
	second := b.goalFile(aID)
	second.Id = bID
	path := "plans/goals/" + bID + ".md"
	body := goal.RenderFile(second)
	if err := os.WriteFile(filepath.Join(b.root(), path), body, 0600); err != nil {
		t.Fatal(err)
	}
	for id, commit := range b.repo.commits {
		commit.files[path] = body
		b.repo.commits[id] = commit
	}
	aTree := b.worktree
	checkNextWorktree(t, b)
	bTree := b.worktree
	trees := map[string]string{aID: aTree, bID: bTree}
	script := filepath.Join(t.TempDir(), "check")
	if err := testexec.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' \"$1\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	declarations := func(name string) string {
		return "proof.cheap=" + shellCommand([]string{script, name}) + "\nproof.audits=true\nproof.deadline=15\nproof.full=true\n"
	}
	contents := map[string]string{"seed": declarations("seed"), "a": declarations("A"), "b": declarations("B"), "a2": declarations("A2"), "merged": declarations("A"), "formatted": "# formatting only\n" + declarations("A")}
	main, mergeBase := "seed", "seed"
	moveOnRead, wrongBranch, missingMain := false, false, false
	b.head = "seed"
	b.workOwnersHook = func(work *intentWorkOwners) {
		original := work.git
		work.git = func(root string, args ...string) ([]byte, error) {
			switch {
			case slices.Equal(args, []string{"rev-parse", "origin/main"}):
				if missingMain {
					return nil, fmt.Errorf("origin/main is unavailable")
				}
				return []byte(main), nil
			case slices.Equal(args, []string{"symbolic-ref", "--short", "HEAD"}):
				if wrongBranch {
					return []byte("goal/another"), nil
				}
				return []byte("goal/" + b.id), nil
			case len(args) == 3 && args[0] == "merge-base":
				return []byte(mergeBase), nil
			case len(args) == 2 && args[0] == "rev-parse" && strings.HasSuffix(args[1], "^{tree}"):
				return []byte(args[1]), nil
			case len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf"):
				commit := strings.TrimSuffix(args[1], ":metasystem.conf")
				if content, ok := contents[commit]; ok {
					if moveOnRead && commit == "a2" {
						b.head = "b"
						moveOnRead = false
					}
					return []byte(content), nil
				}
			}
			return original(root, args...)
		}
	}
	person := enrolledPersonProver(t, b.root(), b.manager.Now())
	selectGoal := func(id, head string) intentOwners {
		b.id, b.head = id, head
		b.worktree = trees[id]
		owners := b.workOwners()
		owners.lookupEnv = func(key string) (string, bool) {
			if strings.HasPrefix(key, "METASYSTEM_PROOF_") {
				return "false", true
			}
			return "", false
		}
		owners.binding = func(_ string, id string, _ time.Time) (dispatchcore.GoalBinding, error) {
			binding := *b.initialBinding
			binding.GoalID = id
			binding.File = b.goalFile(id)
			return binding, nil
		}
		return owners
	}
	brief := b.brief("declarations.md", "Read each round: yes\nBuild the unit.\n")
	buildArgs := func(unit string) []string {
		return []string{"work", "build", b.id, "--work", unit, "--brief", brief, "--lines", "5"}
	}
	build := func(id, head, unit string) intentResult {
		owners := selectGoal(id, head)
		checkNextWorktree(t, b)
		trees[id] = b.worktree
		code, result := checkActBuild(t, b, owners, buildArgs(unit)...)
		if code != 0 {
			t.Fatalf("%s/%s: %d %+v", id, unit, code, result)
		}
		return result
	}
	build(bID, "seed", "initial-b")
	approve := func(id, head, unit string) (intentResult, string) {
		owners := selectGoal(id, head)
		checkNextWorktree(t, b)
		trees[id] = b.worktree
		before := len(b.starter.launched())
		code, held := checkActBuild(t, b, owners, buildArgs(unit)...)
		act := processAct(t, held)
		if code != 1 || act.Class != "declaration" || act.Actor != "unknown" || act.BeforeDeclaration.Values[0] != shellCommand([]string{script, "seed"}) || len(b.starter.launched()) != before {
			t.Fatalf("proposal: %d %+v", code, held)
		}
		_, duplicate := checkActBuild(t, b, owners, buildArgs(unit)...)
		if repeated := processAct(t, duplicate); repeated.ID != act.ID || repeated.Question != act.Question {
			t.Fatalf("duplicate: %+v", repeated)
		}
		if code, result := checkActBuild(t, b, owners, held.Next.Argv[1:]...); code != 1 || len(b.starter.launched()) != before {
			t.Fatalf("agent used act: %d %+v", code, result)
		}
		owners.prove = person
		if id == bID {
			if err := os.Chmod(act.Operation, 0500); err != nil {
				t.Fatal(err)
			}
			code, interrupted := checkActBuild(t, b, owners, held.Next.Argv[1:]...)
			if err := os.Chmod(act.Operation, 0700); err != nil {
				t.Fatal(err)
			}
			q, _ := channel.ReadQuestion(b.root(), act.Question)
			if code != 1 || len(b.starter.launched()) != before || q.State == "closed" {
				t.Fatalf("interrupted publication: %d %s", code, interrupted.Summary)
			}
			owners.prove = fixedFixtureGoalAuthority
		}
		code, built := checkActBuild(t, b, owners, held.Next.Argv[1:]...)
		wantCode := 0
		if code != wantCode {
			t.Fatalf("person remedy: %d %+v", code, built)
		}
		checkActLaunches(t, b, built, act.ID)
		question, err := channel.ReadQuestion(b.root(), act.Question)
		if err != nil || question.State != "closed" {
			t.Fatalf("question: %+v %v", question, err)
		}
		count := len(b.starter.launched())
		if code, result := checkActBuild(t, b, owners, held.Next.Argv[1:]...); code != wantCode || len(b.starter.launched()) != count {
			t.Fatalf("replay: %d %+v", code, result)
		}
		return built, act.ID
	}
	a, aAct := approve(aID, "a", "own-a")
	ownA := trees[aID]
	_, bAct := approve(bID, "b", "own-b")
	for i, id := range []string{aID, bID, aID, bID} {
		build(id, map[string]string{aID: "a", bID: "b"}[id], fmt.Sprintf("alternating-%d", i))
	}
	for _, id := range []string{aID, bID} {
		selectGoal(id, map[string]string{aID: "a", bID: "b"}[id])
		if len(checkActs(t, b)) != 1 {
			t.Fatal("alternation created a false act")
		}
	}
	// Merge the landed A declaration into B; the common ancestor carries A's value.
	main, mergeBase = "a", "a"
	inherited := build(bID, "merged", "inherited-main")
	plan, err := launch.ReadUnitPlan(resultData(t, inherited)["plan"].(string))
	if err != nil || plan.Check.Cheap != shellCommand([]string{script, "A"}) || len(checkActs(t, b)) != 1 {
		t.Fatalf("inherited delta: %+v %v", plan.Check, err)
	}
	// First-main seeds remain, including after new units and moved main.
	for _, id := range []string{aID, bID} {
		raw, err := os.ReadFile(filepath.Join(b.root(), "process", fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte(id)))))
		var reference struct{ Seed string }
		if err != nil || json.Unmarshal(raw, &reference) != nil || reference.Seed != "seed" {
			t.Fatalf("main reset seed: %s %v", raw, err)
		}
	}
	build(aID, "formatted", "formatting")
	// A new round freezes only after its own changed declaration is approved.
	owners := selectGoal(aID, "a2")
	count := len(b.starter.launched())
	run := resultData(t, a)["run"].(string)
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil || len(record.Rounds[0].Reads) != 1 || record.Rounds[0].Stop.Decision != "continue" {
		t.Fatalf("revision prerequisite: %v", err)
	}
	follow := b.brief("follow.md", fmt.Sprintf("Repair the next round.\n\n## Decisions on round 1\n\n| Finding | Decision | Evidence |\n| --- | --- | --- |\n| %s | fixed | a.go:12 |\n", record.Rounds[0].Reads[0].Findings[0].ID))
	b.worktree = ownA
	trees[aID] = ownA
	revision := []string{"work", "revise", "run:" + run, "--brief", follow}
	code, held := checkActBuild(t, b, owners, revision...)
	act := processAct(t, held)
	if code != 1 || act.Class != "declaration" || act.BeforeDeclaration.Values[0] != shellCommand([]string{script, "A"}) || act.Predecessor != aAct || len(b.starter.launched()) != count {
		t.Fatalf("revision hold: %d %+v", code, held)
	}
	oldPath := resultData(t, a)["plan"].(string)
	oldBytes, _ := os.ReadFile(oldPath)
	owners.prove = person
	code, revised := checkActBuild(t, b, owners, held.Next.Argv[1:]...)
	if code != 0 {
		t.Fatalf("revision remedy: %d %+v", code, revised)
	}
	next, err := launch.ReadUnitPlan(resultData(t, revised)["plan"].(string))
	oldAfter, _ := os.ReadFile(oldPath)
	if err != nil || next.Check.Cheap != shellCommand([]string{script, "A2"}) || !bytes.Equal(oldBytes, oldAfter) {
		t.Fatalf("round freeze: %+v %v", next.Check, err)
	}
	// The public executor consumes the frozen tuple even after the branch moves.
	selectGoal(aID, "b")
	checkCode, executed := declaredCheckAt(t, b, ownA, "", "test", "run", "--unit-run", run)
	if checkCode != 0 {
		t.Fatalf("frozen executor: %d %+v", checkCode, executed)
	}
	exits := resultData(t, executed)["exits"].([]any)
	if exits[0].(map[string]any)["output"] != "A2" {
		t.Fatalf("executor re-resolved: %+v", executed)
	}
	selectGoal(bID, "merged")
	if acts := checkActs(t, b); len(acts) != 1 || acts[0].ID != bAct {
		t.Fatalf("A advanced B: %+v", acts)
	}
	owners = selectGoal(aID, "a2")
	b.worktree = ownA
	files, _ := filepath.Glob(filepath.Join(b.root(), "process", "declaration-*.json"))
	baselineBytes := map[string][]byte{}
	for _, path := range files {
		baselineBytes[path], _ = os.ReadFile(path)
	}
	if code, resumed := checkActBuild(t, b, owners, "work", "build", "run:"+run); code != 0 {
		t.Fatalf("old resume: %d %s", code, resumed.Summary)
	}
	for path, before := range baselineBytes {
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("resume rolled back declaration reference")
		}
	}
	owners = selectGoal(bID, "merged")
	// Unknown history holds ordinary agent admission; explicit person repair remains usable.
	baseline := filepath.Join(b.root(), "process", fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte(bID))))
	if err := os.WriteFile(baseline, []byte("damaged evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	owners = selectGoal(bID, "merged")
	checkNextWorktree(t, b)
	trees[bID] = b.worktree
	code, held = checkActBuild(t, b, owners, buildArgs("unknown")...)
	if code != 1 || !strings.Contains(held.Summary, "baseline is unavailable") {
		t.Fatalf("unknown baseline: %d %+v", code, held)
	}
	repair := slices.Clone(held.Next.Argv[1:])
	for i, value := range repair {
		if value == "NAME" {
			repair[i] = "Wido"
		}
		if value == "COMMAND" {
			repair[i] = "true"
		}
	}
	owners.prove = person
	if code, result := checkActBuild(t, b, owners, repair...); code != 0 {
		t.Fatalf("explicit repair: %d %+v", code, result)
	}
	preserved, _ := os.ReadFile(baseline)
	if string(preserved) != "damaged evidence" {
		t.Fatal("repair fabricated a declaration baseline")
	}
	// A changed tree or goal branch never consumes a retained proposal.
	owners = selectGoal(aID, "a2")
	checkNextWorktree(t, b)
	trees[aID] = b.worktree
	wrongBranch = true
	if code, result := checkActBuild(t, b, owners, buildArgs("wrong-branch")...); code != 1 {
		t.Fatalf("wrong branch: %d %+v", code, result)
	}
	wrongBranch = false
	checkNextWorktree(t, b)
	trees[aID] = b.worktree
	moveOnRead = true
	if code, result := checkActBuild(t, b, owners, buildArgs("moving-tree")...); code != 1 || !strings.Contains(result.Summary, "moved") {
		t.Fatalf("tree movement: %d %+v", code, result)
	}
	checkNextWorktree(t, b)
	trees[aID] = b.worktree
	missingMain = true
	if code, result := checkActBuild(t, b, owners, buildArgs("unknown-main")...); code != 1 {
		t.Fatalf("unknown main: %d %+v", code, result)
	}
	t.Log("public build/revise: separate goal histories, inherited main, exact person acts, immutable rounds and explicit damaged-history repair")
}

func TestProcessCheckActImpact(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.declaredCheap = "true"
	owners := b.workOwners()
	args := checkBuildArgs(b, "impact", b.brief("impact.md", "Build.\n"), []string{"false"})
	_, held := checkActBuild(t, b, owners, args...)
	act := processAct(t, held)
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	command, rest, _ := resolveIntentArgv(held.Next.Argv[1:])
	var out, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &out, &stderr, b.root(), owners)
	if code != 0 || !strings.Contains(stderr.String(), "no audits and a 15-minute deadline") {
		t.Fatalf("act impact not shown: %d %s %s", code, out.String(), stderr.String())
	}
	files, _ := filepath.Glob(filepath.Join(b.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
	if len(files) != 1 {
		t.Fatalf("act impact not recorded: %v", files)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil || !strings.Contains(string(raw), "no audits and a 15-minute deadline") || !strings.Contains(string(raw), "Apply the retained check selection") {
		t.Fatalf("impact record: %s %v", raw, err)
	}
	question, _ := channel.ReadQuestion(b.root(), act.Question)
	if question.State != "closed" {
		t.Fatal("approved check did not close its own question")
	}
}

func TestDeclaredChangePrivateEnvironment(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.declaredCheap = "printf '%s' \"$DECLARATION_CONTEXT\""
	plan := launch.UnitPlan{Goal: b.id, Unit: "context", Worktree: b.worktree, Base: b.head,
		Build: launch.UnitBuildPlan{Brief: filepath.Join(b.root(), b.brief("context.md", "Build.\n")), UnitsPage: filepath.Join(b.root(), b.brief("context-units.md", "## Units\n\n| Unit | Changed lines |\n| --- | --- |\n| context | 5 |\n")), Inputs: []string{}, Outputs: []string{}, Units: []string{"context"}},
		Proof: []launch.ProofCommand{{Name: "check", Dir: b.worktree, Argv: []string{"true"}, Env: []string{"DECLARATION_CONTEXT=originating-agent"}}}}
	plan.Check = &launch.UnitCheck{Cheap: "printf forged", Audits: "true", Minutes: 15, Directory: b.worktree, Environment: plan.Proof[0].Env, Declaration: &launch.UnitDeclaration{}}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	code, built := checkActBuild(t, b, b.workOwners(), "work", "build", b.id, "--plan", path)
	if code != 0 {
		t.Fatalf("private environment build: %d %s", code, built.Summary)
	}
	code, executed := declaredCheckAt(t, b, b.worktree, "", "test", "run", "--unit-run", resultData(t, built)["run"].(string))
	if code != 0 {
		t.Fatalf("private environment execution: %d %s", code, executed.Summary)
	}
	exits := resultData(t, executed)["exits"].([]any)
	if exits[0].(map[string]any)["output"] != "originating-agent" {
		t.Fatal("the committed check or its retained private environment was replaced")
	}
	frozen, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
	if err != nil || !slices.Equal(frozen.Check.Environment, plan.Proof[0].Env) {
		t.Fatalf("private environment was not frozen: %v", err)
	}
}
