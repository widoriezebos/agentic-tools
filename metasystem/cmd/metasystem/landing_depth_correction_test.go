package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

func TestLandingPushRejectsImpactWhenBatchDecidedFull(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.prepareBatch(t)
	depthGoals(t, b, 3, 3)
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if len(cmd.Args) > 1 && cmd.Args[1] == "test" {
			writeImpactPlanResult(t, cmd, "plan: base main\nselection: internal/a\n")
			return nil
		}
		fmt.Fprint(cmd.Stdout, "landing environment fixture\nLANDING-CHECKED\t0\n")
		return nil
	}
	code, out := b.run(t, b.root, "prove", "--impact", "--wait", "--json")
	var report struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(out), &report); err != nil || code != 0 || report.Data.Scope != "impact" || report.Data.Result != plain.Green {
		t.Fatalf("impact proof: %d %s (%v)", code, out, err)
	}
	git := b.owners.landing.plainProve.Git
	pushed := false
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --verify refs/remotes/origin/main^{commit}" {
			return "main", nil
		}
		if args[0] == "push" {
			pushed = true
			return "", nil
		}
		return git(dir, args...)
	}
	code, out = b.run(t, b.root, "push")
	if code != 1 || pushed || !strings.Contains(out, "decided full depth") || !strings.Contains(out, "metasystem landing prove") || strings.Contains(out, "--impact") {
		t.Fatalf("full decision bypassed: %d %s pushed=%v", code, out, pushed)
	}
	if len(report.Data.Executions) != 1 || !strings.Contains(report.Data.Executions[0].Reason, "does not satisfy the push") {
		t.Fatalf("explicit impact did not record its limitation: %+v", report.Data.Executions)
	}
	t.Log(out)
}

func TestLandingDepthTierlessGoalRequiresFull(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.prepareBatch(t)
	depthGoals(t, b, 3, 2)
	path := filepath.Join(b.install, "plans", "goals", "a.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	file.Tier = 0
	data = goal.RenderFile(file)
	if strings.Contains(string(data), "- Tier:") {
		t.Fatalf("fixture still has Tier: %s", data)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	// Give the lower-tier path a cheap plan, so a zero-tier regression selects impact.
	fixture := impactAdapterBed(t)
	contract, err := os.ReadFile(filepath.Join(fixture, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		value, err := git(dir, args...)
		if len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
			install := filepath.Join(args[3], "metasystem")
			if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(install, "testing.json"), contract, 0600); err != nil {
				t.Fatal(err)
			}
		}
		return value, err
	}
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		writeImpactPlanResult(t, cmd, "plan: base main\nselection: internal/a=TestA\n")
		return nil
	}
	impact, reason := batchDepth(b.install, b.root, b.head, b.owners.landing.plainProve)
	if impact || reason != "tier 3 (unset): full" {
		t.Fatalf("tierless depth: impact=%v reason=%q", impact, reason)
	}
	t.Log(reason)
}

func TestLandingProveBackgroundHonoursScopedAdmissionWithoutDepthDecision(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.prepareBatch(t)
	// The selected goals have no files, so depth selection falls back to the recent full proof.
	path := "metasystem/plans/page.md"
	contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"}, Surfaces: []testpolicy.Surface{{ID: "app", Paths: []string{"metasystem/**"}, Standard: []string{"cover"}}}, Unknown: []string{"cover"}}
	for _, id := range []string{"cover", "other"} {
		input := path
		if id == "other" {
			input = "other/**"
		}
		contract.Groups = append(contract.Groups, testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{input}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"tests"}, Format: "exit-status"})
	}
	encoded, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testpolicy.Decode(encoded); err != nil {
		t.Fatal(err)
	}
	impactWrite(t, b.install, "metasystem.conf", "testing.contract=testing.json\n")
	at := laneTestNow.Add(-10 * time.Minute).Format(time.RFC3339)
	writeCauseProof(t, b.install, "results.jsonl", plain.Result{Trunk: true, Scope: "full", Result: plain.Green, At: at}, plain.Result{Commit: "merge-b", Tree: "merge-b-tree", Result: plain.Green, Scope: "full", Attempt: "base", At: at, FullAt: at, FullTree: "merge-b-tree", Environment: "fixture"})
	b.head = "merge-b-moved"
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/testing.json") {
			return string(encoded), nil
		}
		if args[0] == "rev-list" && !(len(args) == 5 && args[1] == "--first-parent") {
			return "sha-a\nsha-b", nil
		}
		if args[0] == "diff" {
			return path, nil
		}
		return git(dir, args...)
	}
	b.owners.landing.plainProve.Closure = func(string, string, string) (adapter.Closure, error) { return adapter.Closure{}, nil }
	b.owners.landing.plainProve.Executable = func() (string, error) { return "/fixture/engine", nil }
	b.owners.landing.plainProve.Launch = func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil }
	b.owners.landing.plainProve.Alive = func(plain.Running) bool { return true }
	calls := 0
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		calls++
		if commandEnv(cmd, "LANDING_PROOF_SCOPE") != "scoped" || commandEnv(cmd, "LANDING_PROOF_GROUPS") != "cover" {
			t.Fatalf("admitted scope lost: scope=%q groups=%q", commandEnv(cmd, "LANDING_PROOF_SCOPE"), commandEnv(cmd, "LANDING_PROOF_GROUPS"))
		}
		fmt.Fprint(cmd.Stdout, "landing environment fixture\nlanding group cover passed 1\nLANDING-CHECKED\t0\n")
		return nil
	}
	code, out := b.run(t, b.root, "prove", "--json")
	if code != 0 {
		t.Fatalf("start: %d %s", code, out)
	}
	running, recorded, _, err := plain.ReadRunning(b.install, b.owners.landing.plainProve)
	if err != nil || !recorded || running.Admission == nil || running.Admission.Scope != "scoped" || running.Admission.DepthScope != "" {
		t.Fatalf("admission: %+v (%v)", running, err)
	}
	// The detached command writes to its regular log; a pipe has no proof environment.
	log, err := os.OpenFile(running.Log, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	child := b.owners.landing.plainProve
	child.DepthReason = running.Admission.Reason
	result, err := plain.Run(b.install, b.root, "full-fixture", running.Attempt, log, child)
	if err != nil || result.Result != plain.Green || result.Scope != "scoped" || calls != 1 || result.FullAt != at {
		t.Fatalf("child: %+v calls=%d (%v)", result, calls, err)
	}
	t.Logf("background admitted scoped; child ended %s at %s scope", result.Result, result.Scope)
}
