package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// helmLedgerRepo is a real repository on main with a migrated local goal
// ledger, the production pre-commit guard, metasystem.runtimes=fake (the
// suite's own agent ancestry is no agent here) and wido enrolled at a
// terminal no test process has.
func helmLedgerRepo(t *testing.T, machine string) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "helm fixture")
	runReceiptGit(t, root, "config", "user.email", "helm@example.invalid")
	runReceiptGit(t, root, "config", "metasystem.goal.machine", machine)
	runReceiptGit(t, root, "config", "goal.sync-remote", "local")
	runReceiptGit(t, root, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	for _, directory := range []string{"bin", "scripts/agents", "plans"} {
		helmMust(t, os.MkdirAll(filepath.Join(root, directory), 0o755))
	}
	stub := "#!/usr/bin/env bash\nset -euo pipefail\n" + preCommitEngineDispatch(t) + "exit 1\n"
	helmMust(t, testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte(stub), 0o755))
	legacy := "# Goals\n\n## Goal-free: declared 2026-09-09T00:00:00Z by human over " + strings.Repeat("ab", 32) + "\n"
	digest := bytesSHA256([]byte(legacy))
	baseline, err := json.Marshal(map[string]any{"schemaVersion": 1, "ledger": legacy, "sha256": digest})
	helmMust(t, err)
	writeReceiptFixture(t, root, "plans/goals.md", legacy)
	writeReceiptFixture(t, root, "plans/goals-accepted.json", string(baseline))
	writeReceiptFixture(t, root, "metasystem.conf", "metasystem.runtimes=fake\n")
	writeReceiptFixture(t, root, "scripts/agents/.gitkeep", "")
	runReceiptGit(t, root, "add", ".")
	runReceiptGit(t, root, "commit", "-qm", "initialization")
	var stdout, stderr bytes.Buffer
	if code := goalMigrateWith(defaultSyncRequestDependencies(), &stdout, &stderr, []string{"--root", root, "--source-digest", digest,
		"--sync-mode", "local", "--identity", "01J5XM00000000000000000000", "--by", "wido"}); code != 0 {
		t.Fatalf("migrate: %d %s %s", code, stdout.String(), stderr.String())
	}
	materializeLocalLedger(t, root)
	writeFixtureEnrollment(t, root, "wido")
	return root
}

// helmEngine runs the real engine from dir under a shell in its own session:
// the caller has no controlling terminal and names no agent session, as a
// tool beside the person does.
func helmEngine(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	return helmEngineIn(t, dir, nil, args...)
}

// helmEngineIn is helmEngine with the engine's standard input.
func helmEngineIn(t *testing.T, dir string, stdin io.Reader, args ...string) (int, string) {
	t.Helper()
	words := []string{shellQuote(intentTestEngine(t))}
	for _, arg := range args {
		words = append(words, shellQuote(arg))
	}
	command := exec.Command("/bin/sh", "-c", strings.Join(words, " ")+"; exit $?")
	command.Dir = dir
	var env []string
	for _, entry := range testenv.WithoutInheritedControls(gittree.ScrubbedEnviron()) {
		if name, _, _ := strings.Cut(entry, "="); name != "METASYSTEM_OWNER_LINEAGE" && name != "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE" {
			env = append(env, entry)
		}
	}
	command.Env = env
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.Stdin = stdin
	out, err := command.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), string(out)
	}
	if err != nil {
		t.Fatalf("engine %v: %v", args, err)
	}
	return 0, string(out)
}

func helmOpenGoal(t *testing.T, root, id string) {
	t.Helper()
	if code, out := helmEngine(t, root, "goal", "open", id, "--intent", "Goal "+id+".", "--next", "Continue.",
		"--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "fixture risk",
		"--origin", "human", "--by", "wido", "--fixture-human-authority", "--lineage", "fixture-lineage"); code != 0 {
		t.Fatalf("open %s: %d\n%s", id, code, out)
	}
}

func helmLedgerRecord(t *testing.T, root, path string) string {
	t.Helper()
	return runReceiptGit(t, root, "show", goal.LocalLedgerBranch+":"+path)
}

// TestHelmPersonProofEndToEnd drives the real engine with no controlling
// terminal: goal done refuses before the take, is the holder's act at the
// helm (with and without --by, no lineage), and still refuses from a linked
// worktree, under an adapter supervisor (HB-01) and on another seat (HB-02).
func TestHelmPersonProofEndToEnd(t *testing.T) {
	t.Parallel()
	root := helmLedgerRepo(t, "helm-machine")
	for _, id := range []string{"helm-done", "helm-by", "helm-linked", "helm-machinery", "helm-approve"} {
		helmOpenGoal(t, root, id)
	}
	yields := filepath.Join(root, ".git", "metasystem", "helm-yields.log")

	code, out := helmEngine(t, root, "goal", "done", "helm-done", "--reason", "x")
	if code != 1 || !strings.Contains(out, "this terminal isn't enrolled") || strings.Contains(out, "HUMAN AT THE HELM") {
		t.Fatalf("done before the take: %d\n%s", code, out)
	}

	seat, err := helm.Locate(root)
	helmMust(t, err)
	_, err = helm.Write(root, helm.Record{By: "wido", At: time.Now().UTC().Format(time.RFC3339), Reason: "e2e", Checkout: seat.Checkout})
	helmMust(t, err)

	code, out = helmEngine(t, root, "goal", "done", "helm-done", "--reason", "landed at the helm")
	if code != 0 || strings.Count(out, "HUMAN AT THE HELM (wido): goal done helm-done runs as wido's act") != 1 {
		t.Fatalf("done at the helm: %d\n%s", code, out)
	}
	if record := helmLedgerRecord(t, root, "records/goals/helm-done.md"); !strings.Contains(record, " done actor=human:wido ") {
		t.Fatalf("the archive does not name the holder:\n%s", record)
	}
	lines := yieldLines(t, yields)
	var y helm.Yield
	if len(lines) != 1 || json.Unmarshal([]byte(lines[0]), &y) != nil || y.Boundary != "person-proof" || y.Gate != "human-proof" ||
		y.By != "wido" || !strings.Contains(y.Subject, "verb=goal done helm-done class=UNTRUSTED cwd="+root) {
		t.Fatalf("yields %q", lines)
	}

	// The real done owner with --by the holder and no lineage (HB-03).
	if code, out := helmEngine(t, root, "goal", "done", "helm-by", "--reason", "x", "--by", "wido"); code != 0 ||
		!strings.Contains(helmLedgerRecord(t, root, "records/goals/helm-by.md"), " done actor=human:wido ") {
		t.Fatalf("done --by at the helm: %d\n%s", code, out)
	}

	code, out = helmEngine(t, root, "goal", "approve", "helm-approve", "--budget", "1d/10/720m/1/3")
	if code != 0 {
		t.Fatalf("approve at the helm: %d\n%s", code, out)
	}
	proofs, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "authority", "proofs", "*.json"))
	helmApproval := false
	for _, path := range proofs {
		var record struct {
			Action string
			Proof  humanauthority.Proof
		}
		data, err := os.ReadFile(path)
		helmMust(t, err, json.Unmarshal(data, &record))
		helmApproval = helmApproval || record.Action == "goal approve" && record.Proof.Helm != nil && record.Proof.Helm.Class == "UNTRUSTED"
	}
	if !helmApproval {
		t.Fatalf("no approval proof record holds the helm block: %v", proofs)
	}
	admitted := len(yieldLines(t, yields))
	if admitted != 3 {
		t.Fatalf("three admitted acts left %d yields", admitted)
	}

	refused := func(leg, dir string, args ...string) {
		t.Helper()
		code, out := helmEngine(t, dir, args...)
		if code != 1 || !strings.Contains(out, "this terminal isn't enrolled") || strings.Contains(out, "HUMAN AT THE HELM") || len(yieldLines(t, yields)) != admitted {
			t.Fatalf("%s: %d\n%s", leg, code, out)
		}
	}
	linked := filepath.Join(t.TempDir(), "linked")
	runReceiptGit(t, root, "worktree", "add", "-q", "-b", "feature", linked)
	refused("a linked worktree of the seat", linked, "goal", "done", "helm-linked", "--reason", "x", "--repo", root)

	other := helmLedgerRepo(t, "other-machine")
	helmOpenGoal(t, other, "other-goal")
	refused("--repo naming another seat (HB-02)", root, "goal", "done", "other-goal", "--reason", "x", "--repo", other)

	start, ok := lease.StartedAt(int64(os.Getpid()), nil)
	if !ok {
		t.Fatal("the test process's start is unreadable")
	}
	job := filepath.Join(root, "artifacts", "agents", "jobs", "helm-e2e.json")
	helmMust(t, os.MkdirAll(filepath.Dir(job), 0o755),
		os.WriteFile(job, []byte(fmt.Sprintf(`{"jobId":"helm-e2e","pid":%d,"pidStartedAt":%d}`, os.Getpid(), start)), 0o644))
	refused("a caller under an adapter supervisor (HB-01)", root, "goal", "done", "helm-machinery", "--reason", "x")
	helmMust(t, os.Remove(job))
}

// helmCommitAt commits one file in root under a shell in its own session,
// as GitHub Desktop's git runs the enrolled hook.
func helmCommitAt(t *testing.T, root, file, subject string) {
	t.Helper()
	writeReceiptFixture(t, root, file, file+"\n")
	runReceiptGit(t, root, "add", file)
	command := exec.Command("git", "-C", root, "commit", "-qm", subject)
	command.Env = testenv.WithoutInheritedControls(gittree.ScrubbedEnviron())
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if out, err := command.CombinedOutput(); err != nil || !strings.Contains(string(out), "HUMAN AT THE HELM (wido): the wrapper-fence yields") {
		t.Fatalf("commit %s at the helm: %v\n%s", subject, err, out)
	}
}

// TestHelmReturnEndToEnd: two commits admitted at the helm and pushed to a
// local bare remote; helm return through the real engine with stdin a pipe
// reads both back on origin/main, asks nothing, prints the two commands and
// logs the return.
func TestHelmReturnEndToEnd(t *testing.T) {
	t.Parallel()
	root := helmLedgerRepo(t, "return-machine")
	bare := filepath.Join(t.TempDir(), "origin.git")
	runReceiptGit(t, root, "init", "-q", "--bare", bare)
	runReceiptGit(t, root, "remote", "add", "origin", bare)
	helmMust(t, testexec.WriteFile(filepath.Join(root, ".git", "hooks", "pre-commit"),
		[]byte("#!/bin/sh\nexec \""+filepath.Join(root, "bin", "metasystem")+"\" internal pre-commit --root \""+root+"\"\n"), 0o755))
	// A seat holds this checkout: an agent commit here is fenced.
	helmMust(t, os.MkdirAll(filepath.Join(root, "artifacts", "agents", "mains"), 0o755))
	seat, err := helm.Locate(root)
	helmMust(t, err)
	take := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	_, err = helm.Write(root, helm.Record{By: "wido", At: take.Format(time.RFC3339), Reason: "e2e", Checkout: seat.Checkout})
	helmMust(t, err)
	helmCommitAt(t, root, "one.txt", "first at the helm")
	helmCommitAt(t, root, "two.txt", "second at the helm")
	runReceiptGit(t, root, "push", "-q", "origin", "main")
	first := runReceiptGit(t, root, "rev-parse", "HEAD~1")
	second := runReceiptGit(t, root, "rev-parse", "HEAD")
	// Without an engine at bin/metasystem supervision is not recovered from
	// the test: the return says so in one line and still succeeds.
	helmMust(t, os.Remove(filepath.Join(root, "bin", "metasystem")))

	code, out := helmEngineIn(t, root, strings.NewReader(""), "helm", "return")
	want := fmt.Sprintf("commits at the helm on main: %s first at the helm (on origin/main), %s second at the helm (on origin/main)", first[:7], second[:7])
	if code != 0 || !strings.Contains(out, want) || !strings.Contains(out, "every goal stays open; to conclude one: metasystem goal done G --reason ") ||
		!strings.Contains(out, "to ask independent readers for feedback: metasystem work review --patch ") ||
		strings.Contains(out, "[y/N]") || !strings.HasSuffix(out, "the machinery is at the helm again\n") {
		t.Fatalf("return: %d\n%s", code, out)
	}
	if helm.Active(root).Active {
		t.Fatal("the return left the helm taken")
	}
	if log, err := os.ReadFile(seat.Log); err != nil || !strings.Contains(string(log), `"action":"return","by":"wido"`) {
		t.Fatalf("helm.log: %v\n%s", err, log)
	}
}

// TestHelmReturnConcludesThroughTheRealDoneOwner: after the signature is
// removed, the answer yes runs the real goal done owner in this process with
// the helm proof built from the removed record, no lineage and --by the
// holder (HB-03).
func TestHelmReturnConcludesThroughTheRealDoneOwner(t *testing.T) {
	t.Parallel()
	root := helmLedgerRepo(t, "conclude-machine")
	helmOpenGoal(t, root, "helm-return")
	seat, err := helm.Locate(root)
	helmMust(t, err)
	_, err = helm.Write(root, helm.Record{By: "wido", At: time.Now().UTC().Format(time.RFC3339), Reason: "e2e", Checkout: seat.Checkout})
	helmMust(t, err)
	owners := defaultIntentOwners()
	owners.dependencies.ownerLineage = func() string { return "" }
	// The person answers at the terminal: the goal, Enter for the offered
	// conclusion; the questions are asked on the invocation's own writer.
	owners.helm = helmOwners{
		stdin:         strings.NewReader("helm-return\n\n"),
		stdinTerminal: func() bool { return true },
		recover:       func(processScope) string { return "supervision: recovered" },
	}
	command, rest, _ := resolveIntentArgv([]string{"helm", "return"})
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(command, rest, &stdout, &stderr, root, owners); code != 0 || !strings.Contains(stdout.String(), "goal done helm-return: ") ||
		!strings.Contains(stdout.String(), "Conclude a goal with these commits? [goal id / Enter keeps every goal open] Conclusion [Enter: landed at the helm by wido] ") {
		t.Fatalf("return: %d\n%s\n%s", code, stdout.String(), stderr.String())
	}
	if record := helmLedgerRecord(t, root, "records/goals/helm-return.md"); !strings.Contains(record, " done actor=human:wido ") ||
		!strings.Contains(record, "landed at the helm by wido") {
		t.Fatalf("the conclusion is not the holder's:\n%s\n%s", record, stdout.String())
	}
}
