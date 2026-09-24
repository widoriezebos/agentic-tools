package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestStopVerdictWaitingLinesFromGateOnly(t *testing.T) {
	acceptedRoot := t.TempDir()
	acceptedSession, acceptedMain := pendingWaitVerdictCommandFixture(t, acceptedRoot, "fake")
	code, output, problem := captureChannelOutput(t, func() int {
		return runReportTurnVerdict([]string{"--root", acceptedRoot, "--session", acceptedSession, "--main-id", acceptedMain})
	})
	var accepted struct {
		Display string `json:"display"`
	}
	if code != 0 || problem != "" || json.Unmarshal([]byte(output), &accepted) != nil ||
		strings.Count(accepted.Display, "WAITING: registered wait") != 1 {
		t.Fatalf("accepted Stop wait lines: code=%d stderr=%q output=%q display=%q", code, problem, output, accepted.Display)
	}

	refusedRoot := t.TempDir()
	refusedSession, refusedMain := pendingWaitVerdictCommandFixture(t, refusedRoot, "fake")
	paths, err := filepath.Glob(filepath.Join(refusedRoot, "artifacts", "agents", "waiters", "*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("find refused waiter row: paths=%v err=%v", paths, err)
	}
	var row map[string]any
	data, err := os.ReadFile(paths[0])
	if err != nil || json.Unmarshal(data, &row) != nil {
		t.Fatalf("read refused waiter row: %v", err)
	}
	row["runtimeSession"] = "forged-session"
	data, _ = json.Marshal(row)
	if err := os.WriteFile(paths[0], append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	code, output, problem = captureChannelOutput(t, func() int {
		return runReportTurnVerdict([]string{"--root", refusedRoot, "--session", refusedSession, "--main-id", refusedMain})
	})
	var refused struct {
		Display string `json:"display"`
	}
	if code != 0 || problem != "" || json.Unmarshal([]byte(output), &refused) != nil || strings.Contains(refused.Display, "WAITING") {
		t.Fatalf("refused Stop inherited recovery orientation: code=%d stderr=%q output=%q display=%q", code, problem, output, refused.Display)
	}
	for name, invoke := range map[string]func() int{
		"session start": func() int { return runSessionStart([]string{"--root", refusedRoot, "--session", refusedSession}) },
		"goal next":     func() int { return runGoalNext([]string{"--root", refusedRoot}) },
	} {
		orientationCode, orientation, orientationProblem := captureChannelOutput(t, invoke)
		if orientationCode != 0 || orientationProblem != "" || !strings.Contains(orientation, "WAITING") {
			t.Fatalf("%s lost recovery orientation: code=%d stderr=%q output=%q", name, orientationCode, orientationProblem, orientation)
		}
	}
}

// The command-layer genesis boundary: goalCaller classifies against the
// ONE root it is writing (never a caller-named second root — a root
// the caller chooses is an authority-laundering hole) and applies the
// matrix. A DELEGATE-shaped caller — every
// provisioning flow under agent ancestry looks like one to a virgin
// target — is admitted to genesis exactly when the ledger is
// adoption-shaped, and is refused every holder-only verb; the human
// passes both.

// delegateRoot builds a root whose adapter signatures match this test
// process's own command, so classifying a child of ours there reads
// DELEGATE (the lease package's own fixture pattern).
func delegateRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	command, ok := lease.ProcessCommand(int64(os.Getpid()), nil)
	if !ok {
		t.Skip("cannot read our own command to build a matching signature")
	}
	adapterDir := filepath.Join(root, "scripts/agents/adapters")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := "match " + regexp.QuoteMeta(command)
	script := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' '" + line + "'\n"
	if err := testexec.WriteFile(filepath.Join(adapterDir, "fake.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// childPid spawns a child whose parent is this process, so its ancestry
// carries our (signature-matched or not) command.
func childPid(t *testing.T) int64 {
	t.Helper()
	held := testutil.StartHeldProcess(t, exec.Command("/bin/sh", "-c", "printf x >&3; IFS= read -r _ || :"))
	return int64(held.Command.Process.Pid)
}

func writeLedger(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stageHumanTerminal pins the caller's controlling-terminal fact:
// HUMAN is positive-only, and a headless test runner must not decide
// these legs differently from a person's desk.
func stageHumanTerminal(t *testing.T, root string, pid int64) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	table := filepath.Join(t.TempDir(), "terminal-table.json")
	if err := os.WriteFile(table, []byte(fmt.Sprintf(`{"%d": {"terminal": true}}`, pid)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
}

func TestGoalCallerGenesisBoundary(t *testing.T) {
	goalFree := "# Goals\n\n## Goal-free: declared 2026-08-15T12:00:00Z by human over abc\n"
	withGoals := "# Goals\n\n## Current goal: solo — One goal\n- Origin: main\n- Next step: Do.\n"

	t.Run("delegate reconcile on an adoption-shaped root is genesis-admitted", func(t *testing.T) {
		root := delegateRoot(t)
		writeLedger(t, root, goalFree)
		caller, err := goalCaller(root, childPid(t), "reconcile")
		if err != nil {
			t.Fatalf("adoption-shaped genesis must admit a delegate: %v", err)
		}
		if caller.Class != "DELEGATE" || !caller.Genesis || caller.Holder {
			t.Fatalf("want a non-holder DELEGATE genesis caller, got %+v", caller)
		}
	})

	t.Run("delegate reconcile on a goal-bearing root is refused", func(t *testing.T) {
		root := delegateRoot(t)
		writeLedger(t, root, withGoals)
		if _, err := goalCaller(root, childPid(t), "reconcile"); err == nil ||
			!strings.Contains(err.Error(), "genesis admits a non-holder") {
			t.Fatalf("a goal-bearing ledger must refuse a delegate genesis: %v", err)
		}
	})

	t.Run("delegate open is holder-only refused", func(t *testing.T) {
		root := delegateRoot(t)
		if _, err := goalCaller(root, childPid(t), "open"); err == nil ||
			!strings.Contains(err.Error(), "lease holder") {
			t.Fatalf("open must stay holder-only for a delegate: %v", err)
		}
	})

	t.Run("human passes genesis and holder-only alike", func(t *testing.T) {
		root := t.TempDir()
		writeLedger(t, root, goalFree)
		pid := childPid(t)
		stageHumanTerminal(t, root, pid)
		caller, err := goalCaller(root, pid, "reconcile")
		if err != nil {
			t.Fatalf("human genesis: %v", err)
		}
		if caller.Class != "HUMAN" || !caller.Genesis {
			t.Fatalf("want a HUMAN genesis caller, got %+v", caller)
		}
		if _, err := goalCaller(root, pid, "open"); err != nil {
			t.Fatalf("human open: %v", err)
		}
	})

	t.Run("a broken shape probe refuses the shape, never the human", func(t *testing.T) {
		root := t.TempDir()
		writeLedger(t, root, goalFree)
		pid := childPid(t)
		stageHumanTerminal(t, root, pid)
		t.Setenv("PATH", t.TempDir()) // no git anywhere
		caller, err := goalCaller(root, pid, "reconcile")
		if err != nil {
			t.Fatalf("the human must keep genesis when the probe cannot run: %v", err)
		}
		if caller.Class != "HUMAN" || !caller.Genesis {
			t.Fatalf("want a HUMAN genesis caller, got %+v", caller)
		}
	})

	t.Run("a broken shape probe names itself in a machinery refusal", func(t *testing.T) {
		root := delegateRoot(t)
		writeLedger(t, root, goalFree)
		t.Setenv("PATH", t.TempDir())
		_, err := goalCaller(root, childPid(t), "reconcile")
		if err == nil || !strings.Contains(err.Error(), "adoption-shape probe failed") {
			t.Fatalf("a delegate refused on a broken probe must see the probe error: %v", err)
		}
	})

	t.Run("an accepted baseline ends genesis mode", func(t *testing.T) {
		root := delegateRoot(t)
		writeLedger(t, root, goalFree)
		if err := os.WriteFile(filepath.Join(root, "plans", "goals-accepted.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := goalCaller(root, childPid(t), "reconcile"); err == nil ||
			!strings.Contains(err.Error(), "lease holder") {
			t.Fatalf("an initialized root must be holder-only even for reconcile: %v", err)
		}
	})
}

func TestFreshInitializationUsesHumanGitCommitThenRealMigration(t *testing.T) {
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "fresh human fixture")
	runReceiptGit(t, root, "config", "user.email", "fresh-human@example.invalid")
	runReceiptGit(t, root, "config", "metasystem.goal.machine", "fresh-machine")
	runReceiptGit(t, root, "config", "goal.sync-remote", "local")
	runReceiptGit(t, root, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	for _, directory := range []string{"bin", "scripts/agents", "plans"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	guardBytes, err := os.ReadFile("../../scripts/agents/pre-commit-guard.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "scripts", "agents", "pre-commit-guard.sh"), guardBytes, 0o755); err != nil {
		t.Fatal(err)
	}
	// Only the terminal fact is simulated. The installed production guard,
	// ordinary Git commit and goal migration all execute their real owners.
	stub := `#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} == lease && ${2:-} == classify ]]; then printf '%s\n' '{"class":"HUMAN"}'; exit 0; fi
if [[ ${1:-} == json && ${2:-} == get ]]; then printf '%s\n' HUMAN; exit 0; fi
exit 1
`
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, ".git", "hooks", "pre-commit"), []byte("#!/bin/sh\nexec \""+filepath.Join(root, "scripts", "agents", "pre-commit-guard.sh")+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "# Goals\n\n## Goal-free: declared 2026-09-09T00:00:00Z by human over " + strings.Repeat("ab", 32) + "\n"
	digest := bytesSHA256([]byte(legacy))
	baseline, err := json.Marshal(map[string]any{"schemaVersion": 1, "ledger": legacy, "sha256": digest})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals.md"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals-accepted.json"), baseline, 0o644); err != nil {
		t.Fatal(err)
	}
	runReceiptGit(t, root, "add", ".")
	runReceiptGit(t, root, "commit", "-qm", "human initialization")
	initial := runReceiptGit(t, root, "rev-parse", "HEAD")

	stageHumanTerminal(t, root, int64(os.Getppid()))
	if stderr, code := captureStderr(t, func() int {
		return runGoalMigrate([]string{"--root", root, "--source-digest", digest, "--sync-mode", "local",
			"--identity", "01J5XM00000000000000000000", "--by", "fixture-human"})
	}); code != 0 {
		t.Fatalf("real migration after the human initialization commit failed: code=%d stderr=%s", code, stderr)
	}
	tip := runReceiptGit(t, root, "rev-parse", goal.LocalLedgerBranch)
	if tip == initial {
		t.Fatal("migration did not advance the local accepted baseline")
	}
	if err := exec.Command("git", "-C", root, "cat-file", "-e", tip+":plans/goals.md").Run(); err == nil {
		t.Fatal("migration retained the legacy ledger in the accepted baseline")
	}
	if err := exec.Command("git", "-C", root, "cat-file", "-e", tip+":plans/goals/backlog.md").Run(); err != nil {
		contents, _ := exec.Command("git", "-C", root, "ls-tree", "-r", "--name-only", tip).CombinedOutput()
		kind, _ := exec.Command("git", "-C", root, "cat-file", "-t", tip).CombinedOutput()
		t.Fatalf("migration did not publish the native root record: %v object=%s tree=%s", err, kind, contents)
	}
}

func TestGoalCommandClockOverrideIsFixtureOnly(t *testing.T) {
	production := t.TempDir()
	if err := os.WriteFile(filepath.Join(production, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_GOAL_NOW", "not-a-time")
	t.Setenv("METASYSTEM_GOAL_BOOT_ID", "hostile-boot")
	t.Setenv("METASYSTEM_GOAL_BOOT_NANOS", "not-a-duration")
	before := time.Now().UTC()
	observed, err := goalCommandNow(production)
	after := time.Now().UTC()
	if err != nil || observed.Before(before) || observed.After(after) {
		t.Fatalf("a production root must ignore the fixture clock completely: observed=%s err=%v", observed, err)
	}
	if bootID, elapsed, err := goalCommandBootClock(production); err != nil || bootID == "" || elapsed < 0 || bootID == "hostile-boot" {
		t.Fatalf("a production root accepted the fixture boot clock: boot=%q elapsed=%s err=%v", bootID, elapsed, err)
	}

	fixture := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-28T09:45:00Z")
	t.Setenv("METASYSTEM_GOAL_BOOT_ID", "fixture-boot")
	t.Setenv("METASYSTEM_GOAL_BOOT_NANOS", strconv.FormatInt((3*time.Hour).Nanoseconds(), 10))
	observed, err = goalCommandNow(fixture)
	if err != nil || observed.Format(time.RFC3339) != "2026-08-28T09:45:00Z" {
		t.Fatalf("a fake root must receive its explicit fixture instant: observed=%s err=%v", observed, err)
	}
	if bootID, elapsed, err := goalCommandBootClock(fixture); err != nil || bootID != "fixture-boot" || elapsed != 3*time.Hour {
		t.Fatalf("a fake root did not receive its indivisible boot clock: boot=%q elapsed=%s err=%v", bootID, elapsed, err)
	}
	t.Setenv("METASYSTEM_GOAL_NOW", "not-a-time")
	if _, err := goalCommandNow(fixture); err == nil {
		t.Fatal("a malformed clock fixture must fail loudly in a fake root")
	}
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-28T09:45:00Z")
	t.Setenv("METASYSTEM_GOAL_BOOT_NANOS", "")
	if _, _, err := goalCommandBootClock(fixture); err == nil {
		t.Fatal("a partial boot-clock fixture must fail loudly in a fake root")
	}
	t.Run("stop proof cancellation keeps the frozen instant fixture-only", testStopProofCancelCommandKeepsFrozenInstantFixtureOnly)
}
