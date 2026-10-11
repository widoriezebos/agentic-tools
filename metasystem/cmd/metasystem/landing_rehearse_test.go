package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/rehearsal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// Real Git, the candidate build and the real proof contract are the subject;
// a synthetic Git or proof runner cannot prove the origin and depth assertions.
func TestLandingRehearsalRealRepositoryGreenAndIsolation(t *testing.T) {
	t.Parallel()
	if os.Getenv("METASYSTEM_REAL_REHEARSAL") != "1" {
		t.Skip("real-process wiring proof; run with METASYSTEM_REAL_REHEARSAL=1 and -timeout 30m")
	}
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	module = rehearsalCandidateCheckout(t, module)
	// The archived candidate owns the templates even when the calling
	// checkout edits its working copy after the candidate was frozen.
	if err := os.WriteFile(filepath.Join(module, "internal/landing/rehearsal/fixtures/green/value.go"), []byte("invalid working-copy template\n"), 0644); err != nil {
		t.Fatal(err)
	}
	untracked, err := os.CreateTemp(module, ".rehearsal-untracked-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := untracked.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(untracked.Name()) })
	host := t.TempDir()
	canary := filepath.Join(host, ".metasystem", "host", "landing-lane.json")
	if err := os.MkdirAll(filepath.Dir(canary), 0700); err != nil {
		t.Fatal(err)
	}
	hostLane := t.TempDir()
	hook := filepath.Join(hostLane, ".git/hooks/pre-push")
	if err := os.MkdirAll(filepath.Dir(hook), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte("host hook canary"), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(lane.Record{Root: hostLane, Install: filepath.Join(hostLane, "metasystem"), CustodyEpoch: 9, RegisteredBy: "host", At: "2026-10-10T12:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(canary, data, 0600); err != nil {
		t.Fatal(err)
	}
	snapshot := func(root string) string {
		t.Helper()
		var paths []string
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			paths = append(paths, fmt.Sprintf("%s %s", rel, info.Mode()))
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return strings.Join(paths, "\n")
	}
	beforeHost, beforeLane := snapshot(host), snapshot(hostLane)
	t.Cleanup(func() {
		if snapshot(host) != beforeHost || snapshot(hostLane) != beforeLane {
			t.Error("host registry or lane files changed")
		}
		if after, err := os.ReadFile(canary); err != nil || string(after) != string(data) {
			t.Errorf("host lane changed: %s: %v", after, err)
		}
		if after, err := os.ReadFile(hook); err != nil || string(after) != "host hook canary" {
			t.Errorf("host lane hook changed: %s: %v", after, err)
		}
		if _, err := os.Stat(filepath.Join(host, ".metasystem", "armed-checkouts.jsonl")); !os.IsNotExist(err) {
			t.Errorf("host registry was written: %v", err)
		}
	})
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fixture := pinProofBinaryFixture(t, module)
	cmd := fixture.command(append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1", "METASYSTEM_BIN="+executable, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+host), executable, "landing", "rehearse", "--keep", "--json", "--repo", module)
	cmd.Dir = module
	read := verbresult.Capture(cmd, "landing rehearse")
	result, err := read(boundedexec.RunContext(context.Background(), cmd, boundedexec.FixedBound(11*time.Minute, "rehearsal CLI result and cleanup"), "rehearsal CLI"))
	if err != nil {
		t.Fatal(err)
	}
	var record rehearsal.Record
	if err := json.Unmarshal(result.Data, &record); err != nil {
		t.Fatal(err)
	}
	s := &rehearsal.Scratch{Record: record, Checkout: filepath.Join(record.Scratch, "lane"), Registry: filepath.Join(record.Scratch, "registry", ".metasystem")}
	s.Module = filepath.Join(s.Checkout, "metasystem")
	s.Env = append(os.Environ(), "METASYSTEM_SUPERVISION_REGISTRY_HOME="+filepath.Dir(s.Registry), "METASYSTEM_BIN="+s.Engine, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	t.Logf("%s; scratch %s; %d steps", result.Summary, s.Scratch, len(s.Steps))
	t.Logf("breakdown (ms): %+v", record.Breakdown)
	t.Logf("rehearsal record: %s", result.Data)
	if _, err := os.Stat(s.Scratch); err != nil {
		t.Errorf("CLI exit removed retained scratch: %v", err)
	}
	t.Cleanup(func() {
		if !t.Failed() {
			_ = os.RemoveAll(filepath.Dir(filepath.Dir(s.Scratch)))
		}
	})
	if _, err := os.Stat(filepath.Join(s.Scratch, "snapshot/metasystem", filepath.Base(untracked.Name()))); !os.IsNotExist(err) {
		t.Fatalf("candidate archive included a working-tree-only file: %v", err)
	}
	if _, ok, err := lane.Read(s.Registry); err != nil || !ok {
		t.Fatalf("scratch registration missing: %v", err)
	}
	if result.Exit != 0 || record.Outcome != "passed" || record.WallMS >= (10*time.Minute).Milliseconds() {
		for _, step := range record.Steps {
			if step.Exit != 0 || step.WallMS > 1000 {
				t.Logf("step %s: %d ms, exit %d: %s", step.Command, step.WallMS, step.Exit, step.Output)
			}
		}
		t.Fatalf("rehearsal CLI exit %d, outcome %q: %s", result.Exit, record.Outcome, record.Problem)
	}
	for _, step := range record.Steps {
		if strings.Contains(step.Output, "started metasystem deploy now") {
			t.Fatalf("scratch push started a deploy: %s", step.Output)
		}
	}
	if _, err := deploy.Declared(s.Module, s.Checkout); !errors.Is(err, deploy.ErrNoContract) {
		t.Fatalf("scratch declares a deploy: %v", err)
	}
	if entries, err := os.ReadDir(filepath.Join(s.Registry, "deploy")); err == nil && len(entries) != 0 || err != nil && !os.IsNotExist(err) {
		t.Fatalf("scratch wrote deploy records: %v: %v", entries, err)
	}
	pids, err := identity.AllPids()
	if err != nil {
		t.Fatal(err)
	}
	for _, pid := range pids {
		exact, state, err := (identity.KernelProber{}).Probe(pid)
		if err != nil {
			t.Fatal(err)
		}
		if state != identity.Alive || exact.Zombie || exact.Exiting {
			continue
		}
		cwd, known := identity.ProcessCwd(pid)
		if known && (cwd == s.Scratch || strings.HasPrefix(cwd, s.Scratch+string(filepath.Separator))) {
			t.Fatalf("rehearsal returned with live child %d: %v", pid, exact.Argv)
		}
		for _, entry := range exact.Environ {
			if entry == "METASYSTEM_SUPERVISION_REGISTRY_HOME="+filepath.Dir(s.Registry) {
				t.Fatalf("rehearsal returned with live child %d: %v", pid, exact.Argv)
			}
		}
	}
	var commands []string
	for _, step := range record.Steps {
		if strings.HasPrefix(step.Command, "landing ") || strings.HasPrefix(step.Command, "work ") {
			commands = append(commands, step.Command)
		}
	}
	wantCommands := []string{
		"landing set " + s.Checkout,
		"landing prove --trunk --wait",
		"work land " + rehearsal.GreenGoal,
		"landing run",
		"landing prove --gate --wait",
		"landing prove --wait",
		"landing push",
		"landing status --json",
	}
	if strings.Join(commands, "\n") != strings.Join(wantCommands, "\n") {
		t.Fatalf("candidate command sequence = %q, want %q", commands, wantCommands)
	}
	for _, verb := range []string{"landing set " + s.Checkout, "landing run"} {
		found := false
		for _, step := range record.Steps {
			if step.Command == verb {
				found = true
				var result struct{ Exit int }
				if err := json.Unmarshal([]byte(step.Output), &result); err != nil || result.Exit != 0 {
					t.Fatalf("candidate verb %s: %s: %v", verb, step.Output, err)
				}
				if verb == "landing run" && (!strings.Contains(step.Output, `"outcome":"started"`) || !strings.Contains(step.Output, plain.WakeQueued)) {
					t.Fatalf("candidate keeper did not wake for queued work: %s", step.Output)
				}
			}
		}
		if !found {
			t.Fatalf("candidate engine did not run %s", verb)
		}
	}
	keeper, err := lane.ReadAgentState(s.Registry)
	if err != nil || keeper.Launch != rehearsal.GreenGoal || keeper.Fingerprint == "" {
		t.Fatalf("real keeper evidence missing: %+v: %v", keeper, err)
	}
	if _, err := os.Stat(filepath.Join(module, "records/rehearsals", record.Tree+".json")); !os.IsNotExist(err) {
		t.Fatalf("private CLI wrote candidate repository record: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.Scratch, "records/rehearsals", record.Tree+".json")); err != nil {
		t.Fatalf("private CLI lost scratch record: %v", err)
	}
	if record.Breakdown.Setup <= 0 || record.Breakdown.Bootstrap <= 0 || record.Breakdown.HandIn <= 0 || record.Breakdown.Gate <= 0 || record.Breakdown.Proof <= 0 || record.Breakdown.Push <= 0 || record.Breakdown.Total != record.WallMS || !strings.Contains(result.Summary, "slowest:") {
		t.Fatalf("CLI lost its timing breakdown: %+v: %s", record.Breakdown, result.Summary)
	}
	proofs, err := plain.Results(s.Module)
	if err != nil {
		t.Fatal(err)
	}
	for _, proof := range proofs {
		if proof.Trunk && proof.Scope == "full" {
			s.Harness = proof.Commit
		}
	}
	if s.Harness == "" {
		t.Fatal("the real bootstrap result is missing")
	}
	plan, err := plain.ReadImpactPlan(plain.ProveSeams{Executable: func() (string, error) { return s.Engine, nil }, Command: func(cmd *exec.Cmd) error { cmd.Env = s.Env; return cmd.Run() }}, s.Module, s.Harness)
	if err != nil {
		t.Fatal(err)
	}
	_, contract, _, err := testrun.LoadContract(s.Module)
	if err != nil {
		t.Fatal(err)
	}
	measure, err := fullPackageMeasure(s.Module, contract)
	if err != nil {
		t.Fatal(err)
	}
	_, cheap := impactDepth(plan, contract, measure, 50)
	if !cheap {
		t.Fatalf("real green plan was not cheap: %s", plan)
	}
}

func TestLandingRehearsalCommandGrammar(t *testing.T) {
	t.Parallel()
	row, registered := idempotencyRows["landing rehearse"]
	if !registered || row.kind != idemCreation || strings.TrimSpace(row.why) == "" {
		t.Fatalf("rehearsal must declare why each call creates a new attempt: %+v", row)
	}
	command, found := findIntentAction("landing", "rehearse")
	if !found {
		t.Fatal("landing rehearse is not routed")
	}
	for _, args := range [][]string{{"--keep"}, {"--engine", "/other"}, {"unexpected"}} {
		_, problem := parseIntentArgs(command, args)
		if (problem == nil) != (strings.Join(args, " ") == "--keep") {
			t.Fatalf("arguments %v: %+v", args, problem)
		}
	}
}

func TestLandingRehearsalRealContractLedgerAndPlan(t *testing.T) {
	t.Parallel()
	if os.Getenv("METASYSTEM_REAL_REHEARSAL") != "1" {
		t.Skip("real contract, ledger and plan proof; run with METASYSTEM_REAL_REHEARSAL=1 and -timeout 30m")
	}
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	module = rehearsalCandidateCheckout(t, module)
	ctx := context.Background()
	s, err := rehearsal.Prepare(ctx, module, os.Environ())
	if s != nil {
		t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(filepath.Dir(s.Scratch))) })
	}
	if err != nil {
		t.Fatal(err)
	}
	settings, err := rehearsal.LaunchSettings(s.Module)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.HarnessSetup(filepath.Join(module, "internal/landing/rehearsal/fixtures/green"), settings); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"value.go", "value_test.go"} {
		path := filepath.Join(s.Module, "internal/rehearsalgreen", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "1", "2")), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "metasystem/internal/rehearsalgreen"}, {"commit", "-m", "Change the fixture for the real impact plan"}} {
		if _, err := s.Git(s.Checkout, args...); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := config.ResolvePolicy(config.GetParams{Key: "landing.batch", ConfPath: filepath.Join(s.Module, "metasystem.conf"), Policy: &config.PolicyContext{Checkout: s.Checkout, CallingCheckout: s.Checkout}, LookupEnv: func(string) (string, bool) { return "", false }})
	if err != nil || policy.Value != "1" {
		t.Fatalf("real committed batch policy: %+v: %v", policy, err)
	}
	plan, err := plain.ReadImpactPlan(plain.ProveSeams{Executable: func() (string, error) { return s.Engine, nil }, Command: func(cmd *exec.Cmd) error {
		cmd.Env = s.Env
		return boundedexec.RunContext(ctx, cmd, boundedexec.FixedBound(10*time.Minute, "real plan test"), "real impact plan")
	}}, s.Module, s.Harness)
	if err != nil || !strings.Contains(plan, "internal/rehearsalgreen") {
		t.Fatalf("real fixture plan: %s: %v", plan, err)
	}
	_, contract, _, err := testrun.LoadContract(s.Module)
	if err != nil {
		t.Fatal(err)
	}
	measure, err := fullPackageMeasure(s.Module, contract)
	if err != nil {
		t.Fatal(err)
	}
	share, cheap := impactDepth(plan, contract, measure, 50)
	if !cheap {
		t.Fatalf("real fixture decided full: %d%%: %s", share, plan)
	}
	broken := contract
	broken.Always.Canary = nil
	brokenShare, brokenCheap := impactDepth(plan, broken, measure, 50)
	if brokenCheap {
		t.Fatalf("counting real canaries stayed cheap: %d%%: %s", brokenShare, plan)
	}
	t.Logf("real contract, validated real ledger plus fixture, actual plan: cheap %d%%; counting canaries decides full %d%%", share, brokenShare)
	// A named repeat must keep its selection and report the actual fixture red.
	s.Env = append(s.Env, "LANDING_ONLY=metasystem/internal/rehearsalgreen=TestValue")
	conf, err := os.ReadFile(filepath.Join(s.Module, "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	var command string
	for _, line := range strings.Split(string(conf), "\n") {
		if value, ok := strings.CutPrefix(line, "proof.full="); ok {
			command = value
		}
	}
	if command == "" {
		t.Fatal("harness has no committed full proof declaration")
	}
	output, err := s.Run(s.Module, "sh", "-c", command)
	if err != nil || !strings.HasSuffix(output, "LANDING-CHECKED\t0") || strings.Contains(output, "landing group fast-static-build") {
		t.Fatalf("fixture repeat did not honor LANDING_ONLY: %s: %v", output, err)
	}
	path := filepath.Join(s.Module, "internal/rehearsalgreen/value.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "2", "3")), 0644); err != nil {
		t.Fatal(err)
	}
	output, err = s.Run(s.Module, "sh", "-c", command)
	failed := plain.FailedChecks([]byte(output))
	if err == nil || len(failed) != 1 || failed[0].Unit != "metasystem/internal/rehearsalgreen" || len(failed[0].Tests) != 1 || failed[0].Tests[0] != "TestValue" || !strings.HasSuffix(output, "LANDING-CHECKED\t1") {
		t.Fatalf("fixture reporter lost its failing test protocol: %+v: %s: %v", failed, output, err)
	}
	// Reuse the real candidate build, with the same real source tree.
	binary, err := os.ReadFile(s.Engine)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(module, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(module, "bin/metasystem"), binary, 0755); err != nil {
		t.Fatal(err)
	}
	reused, err := rehearsal.Prepare(ctx, module, os.Environ())
	if reused != nil {
		t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(filepath.Dir(reused.Scratch))) })
	}
	if err != nil {
		t.Fatal(err)
	}
	copied := false
	for _, step := range reused.Steps {
		if strings.Contains(step.Command, "devgate build") {
			t.Fatal("matching real candidate engine was rebuilt")
		}
		copied = copied || step.Command == "copy candidate bin/metasystem"
	}
	if !copied || reused.Tree != s.Tree {
		t.Fatalf("real build reuse lost its tree: %s, want %s", reused.Tree, s.Tree)
	}
}

// Freeze the worktree through the repository's isolated-index projection.
// Only this private clone receives a commit; the source index is untouched.
func rehearsalCandidateCheckout(t *testing.T, module string) string {
	t.Helper()
	workspace := gittree.Workspace{Dir: filepath.Dir(module)}
	tree, err := workspace.Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	patch, err := workspace.Diff("HEAD^{tree}", tree)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "candidate")
	git := func(dir string, input []byte, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		cmd.Stdin = strings.NewReader(string(input))
		data, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, data, err)
		}
		return strings.TrimSpace(string(data))
	}
	git(filepath.Dir(module), nil, "clone", "--no-local", filepath.Dir(module), root)
	git(root, nil, "config", "core.hooksPath", "/dev/null")
	if len(patch) > 0 {
		git(root, patch, "apply", "--index", "--binary")
		git(root, nil, "-c", "user.name=rehearsal", "-c", "user.email=rehearsal@example.invalid", "commit", "-m", "Freeze the candidate worktree for rehearsal")
	}
	if actual := git(root, nil, "rev-parse", "HEAD^{tree}"); actual != tree {
		t.Fatalf("private candidate tree %s differs from worktree %s", actual, tree)
	}
	t.Logf("candidate worktree tree: %s", tree)
	return filepath.Join(root, "metasystem")
}
