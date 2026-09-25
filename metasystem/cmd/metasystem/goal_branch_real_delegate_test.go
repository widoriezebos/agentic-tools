package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// TestGoalBranchReadRealDelegateReachesSelectedClaude drives `goal branch
// read --selected-installation` from a generated goal worktree through the
// built engine's delegate, the worktree's real dispatch.sh and claude.sh
// adapter, to an absolute fake `claude` executable. The worktree's tracked
// roster names a different model and it has no local overlay; the model the
// fake receives is the selected installation's code-critic for the frozen
// brief's working mode. Declared fakes: the fast gate (green), the `claude`
// executable, the synthetic configuration and goal state in temp repos.
func TestGoalBranchReadRealDelegateReachesSelectedClaude(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-o", engine, "./cmd/metasystem")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build engine: %v: %s", err, output)
	}
	cases := []struct {
		name, brief, mode, model string
	}{
		{"headerless", "Ordinary accepted brief: check the fixture unit.\n", "implement", "fixture-selected-implement"},
		{"explicit-review", "Working Mode: review\n\nReview-mode brief: read the fixture unit.\n", "review", "fixture-selected-review"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
			worktree, unit := realDelegateGoalWorktree(t, moduleRoot, engine)

			// The fake model records its working directory, argv and stdin.
			record := filepath.Join(t.TempDir(), "claude-record")
			fakeBin := t.TempDir()
			fake := filepath.Join(fakeBin, "claude")
			script := "#!/usr/bin/env bash\ncase \"${1:-}\" in --version) echo '2.1.0 (Claude Code)'; exit 0 ;; auth) exit 0 ;; esac\n" +
				"pwd -P >'" + record + ".cwd'\nprintf '%s\\n' \"$@\" >'" + record + ".argv'\ncat >'" + record + ".stdin' || true\n" +
				"model=; prev=; for a in \"$@\"; do [[ \"$prev\" == --model ]] && model=$a; prev=$a; done\n" +
				// Announce the session the way the Claude hook does, then end the turn.
				"printf '{\"session_id\":\"fake-session\",\"model\":\"%s\"}\\n' \"$model\" >\"$METASYSTEM_CLAUDE_SESSION_SIGNAL\"\nsleep 1\n" +
				// Its result reports the model it ran as Claude does, in modelUsage;
				// the critic payload itself stays deliberately incomplete.
				"printf '{\"type\":\"result\",\"subtype\":\"success\",\"session_id\":\"fake-session\",\"model\":\"%s\",\"modelUsage\":{\"%s\":{}},\"result\":\"fixture\"}\\n' \"$model\" \"$model\"\n" +
				": >'" + record + ".done'\n"
			if err := testexec.WriteFile(fake, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
			probe := exec.Command(filepath.Join(worktree, "scripts", "agents", "adapters", "claude.sh"), "probe")
			probe.Dir, probe.Env = worktree, append(os.Environ(), "METASYSTEM_BIN="+filepath.Join(worktree, "bin", "metasystem"))
			if output, err := probe.CombinedOutput(); err != nil {
				t.Fatalf("claude adapter probe: %v: %s", err, output)
			}

			selected := t.TempDir()
			writeTestingFixtureFile(t, filepath.Join(selected, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"+
				"role.code-critic.runtime=claude\nrole.code-critic.model.claude=fixture-selected-generic\n"+
				"runtime.claude.maximal-models=fixture-selected-implement,fixture-selected-review\n"+
				"mode.implement.role.code-critic.runtime=claude\nmode.implement.role.code-critic.model.claude=fixture-selected-implement\n"+
				"mode.review.role.code-critic.runtime=claude\nmode.review.role.code-critic.model.claude=fixture-selected-review\n"), 0o644)
			brief := filepath.Join(t.TempDir(), "brief.md")
			writeTestingFixtureFile(t, brief, []byte(c.brief), 0o644)

			code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
				return runGoalBranchReadWith([]string{"--root", worktree, "--goal", "standing-validation", "--unit", unit,
					"--brief", brief, "--selected-installation", selected}, goalBranchReadDependencies{
					Binary: filepath.Join(worktree, "bin", "metasystem"), Gate: func(string) (string, error) { return "green", nil },
				})
			})
			t.Logf("branch read: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			if code != 0 || !strings.Contains(stdout, "state=dispatched") {
				argv, _ := os.ReadFile(record + ".argv")
				t.Fatalf("branch read: code=%d stdout=%q stderr=%q; fake claude argv=%.200q", code, stdout, stderr, argv)
			}
			job := strings.Fields(strings.SplitN(stdout, "root-job=", 2)[1])[0]
			// The engine's own job waiter returns once the job records a
			// terminal status, so the fake's files and the terminal record are
			// complete before they are read and before the worktree is removed.
			// The fake's result is not a critic verdict, so any terminal outcome
			// is accepted; timeout, interruption or a missing record is not.
			wait := exec.CommandContext(t.Context(), filepath.Join(worktree, "bin", "metasystem"), "wait", "--root", worktree, "--job", job, "--timeout", "90s", "--json")
			var waitStderr strings.Builder
			wait.Stderr = &waitStderr
			waitOutput, waitErr := wait.Output()
			if _, exited := waitErr.(*exec.ExitError); waitErr != nil && !exited {
				t.Fatalf("job wait: %v", waitErr)
			}
			var waited struct {
				ExitCode      int    `json:"exitCode"`
				SourceOutcome string `json:"sourceOutcome"`
			}
			if err := json.Unmarshal(waitOutput, &waited); err != nil || waited.ExitCode > 3 || !dispatchcore.TerminalStatus(waited.SourceOutcome) {
				log, _ := os.ReadFile(filepath.Join(worktree, "artifacts", "agents", "jobs", job+".log"))
				t.Fatalf("job %s did not reach a terminal status: wait=%s stderr=%s err=%v; job log:\n%s", job, waitOutput, waitStderr.String(), err, log)
			}
			t.Logf("job %s terminal: exit=%d outcome=%s", job, waited.ExitCode, waited.SourceOutcome)
			if _, err := os.Stat(record + ".done"); err != nil {
				log, _ := os.ReadFile(filepath.Join(worktree, "artifacts", "agents", "jobs", job+".log"))
				t.Fatalf("fake claude never finished for job %s (%v); job log:\n%s", job, err, log)
			}
			argv, _ := os.ReadFile(record + ".argv")
			cwd, _ := os.ReadFile(record + ".cwd")
			stdin, _ := os.ReadFile(record + ".stdin")
			t.Logf("fake claude cwd=%s stdin=%d bytes argv=%.160q", strings.TrimSpace(string(cwd)), len(stdin), argv)
			if !strings.Contains(string(argv), "--model\n"+c.model+"\n") {
				t.Fatalf("fake claude model: want %s, argv=%s", c.model, argv)
			}
			if strings.Contains(string(argv), "fixture-worktree") {
				t.Fatalf("the worktree roster reached the model: %s", argv)
			}
			// The prompt on stdin carries the frozen brief under exactly one
			// Working Mode header, the mode dispatch resolved the roster for.
			var modes []string
			for _, line := range strings.Split(string(stdin), "\n") {
				if strings.HasPrefix(line, "Working Mode:") {
					modes = append(modes, line)
				}
			}
			body := strings.TrimSpace(strings.TrimPrefix(c.brief, "Working Mode: review\n\n"))
			t.Logf("prompt mode headers=%q brief body present=%t", modes, strings.Contains(string(stdin), body))
			if len(modes) != 1 || modes[0] != "Working Mode: "+c.mode || !strings.Contains(string(stdin), body) ||
				!strings.Contains(string(stdin), "# Supplied accepted implementation brief (frozen at dispatch)") {
				t.Fatalf("fake claude prompt: modes=%q, want the frozen brief %q under Working Mode: %s", modes, body, c.mode)
			}
			jobRecord, err := os.ReadFile(filepath.Join(worktree, "artifacts", "agents", "jobs", job+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(jobRecord, &fields); err != nil {
				t.Fatal(err)
			}
			if status, _ := fields["status"].(string); !dispatchcore.TerminalStatus(status) || status != waited.SourceOutcome {
				t.Fatalf("job record status %q is not the terminal outcome %q the waiter returned", status, waited.SourceOutcome)
			}
			t.Logf("job record %s: runtime=%v workspaceRoot=%v overridden=%v roster=%v", job, fields["runtime"], fields["workspaceRoot"], fields["overridden"], realDelegateRosterFields(fields))
			if fields["runtime"] != "claude" || fields["requestedModel"] != c.model || fields["effectiveModel"] != c.model || fields["sessionId"] != "fake-session" ||
				fields["escalationApproval"] != nil || fields["overridden"] != false {
				t.Fatalf("job record roster identity: runtime=%v requested=%v effective=%v escalation=%v overridden=%v",
					fields["runtime"], fields["requestedModel"], fields["effectiveModel"], fields["escalationApproval"], fields["overridden"])
			}
			workspace, _ := fields["workspaceRoot"].(string)
			if resolved, err := filepath.EvalSymlinks(workspace); err != nil || strings.TrimSpace(string(cwd)) != resolved ||
				!strings.HasPrefix(resolved, worktree) {
				t.Fatalf("fake claude cwd %q is not the job workspace %q inside the worktree %s (%v)", cwd, workspace, worktree, err)
			}
			for key, value := range fields {
				lower := strings.ToLower(key)
				if (strings.Contains(lower, "override") || strings.Contains(lower, "escalat")) && value != false && value != nil && value != "" {
					t.Fatalf("job record shows roster override/escalation %s=%v", key, value)
				}
			}
		})
	}
}

func realDelegateRosterFields(fields map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range fields {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "roster") || strings.Contains(lower, "override") || strings.Contains(lower, "escalat") || strings.Contains(lower, "model") {
			out[key] = value
		}
	}
	return out
}

// realDelegateGoalWorktree builds the claimed, approved goal fixture with a
// breach-stop capability, the repository's real scripts and a placeholder
// worktree roster, then creates goal/standing-validation with one pushed unit
// commit and the built engine installed and enrolled in the worktree.
func realDelegateGoalWorktree(t *testing.T, moduleRoot, engine string) (string, string) {
	t.Helper()
	main, _, _ := goalBranchMainCLIFixtureBelow(t, "m1", ".")
	guard := filepath.Join(main, "scripts", "agents", "pre-commit-guard.sh")
	stub, err := os.ReadFile(guard)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"scripts", "docs", "skills"} {
		if output, err := exec.Command("cp", "-R", filepath.Join(moduleRoot, dir), main).CombinedOutput(); err != nil {
			t.Fatalf("install %s: %v: %s", dir, err, output)
		}
	}
	writeTestingFixtureFile(t, guard, stub, 0o755)
	conf := filepath.Join(main, "metasystem.conf")
	existing, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	roster := "metasystem.runtimes=claude\nrole.code-critic.runtime=claude\nrole.code-critic.model.claude=fixture-worktree-generic\n" +
		// The worktree authorizes only its own critic as maximal; the selected
		// critic models are authorized by the selected installation alone.
		"runtime.claude.maximal-models=fixture-worktree-generic\n" +
		"evidence.root=" + filepath.Join(t.TempDir(), "evidence") + "\n"
	writeTestingFixtureFile(t, conf, []byte(strings.Replace(string(existing), "metasystem.runtimes=fake\n", roster, 1)), 0o644)
	goalPath := filepath.Join(main, "plans", "goals", "standing-validation.md")
	data, err := os.ReadFile(goalPath)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal: %v", problems)
	}
	// The goal is opened, claimed and approved within its elapsed budget at
	// wall-clock time, since the real supervisor measures elapsed against now.
	now := time.Now().UTC()
	file.OpenedAt = now.Add(-10 * time.Minute).Format(time.RFC3339)
	file.Claimed.At = now.Add(-9 * time.Minute).Format(time.RFC3339)
	file.Approved.At = now.Add(-8 * time.Minute).Format(time.RFC3339)
	for i, at := range []string{file.OpenedAt, file.Claimed.At, file.Approved.At} {
		file.History[i].At = at
	}
	file.StopCapability = &goal.StopCapability{Generation: 2, Revision: file.Claimed.Revision, Machine: "mac-cli", ClaimEpoch: 1}
	writeTestingFixtureFile(t, goalPath, goal.RenderFile(file), 0o644)
	goalSyncMutationGit(t, main, "add", "-A")
	goalSyncMutationGit(t, main, "commit", "-qm", "real scripts, worktree roster, stop capability")
	goalSyncMutationGit(t, main, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, main, "update-ref", goal.AcceptedRef, "HEAD")
	goalSyncMutationGit(t, main, "push", "-q", "upstream", "HEAD:main")
	base := goalSyncMutationGit(t, main, "rev-parse", "HEAD")

	worktree := filepath.Join(t.TempDir(), "goal-worktree")
	goalSyncMutationGit(t, main, "worktree", "add", "-q", "-b", "goal/standing-validation", worktree, base)
	worktree, err = filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe holder: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(worktree, "goal-branch-real-delegate", exact.Pid, exact.StartedAt.Unix(),
		exact.StartTicks, exact.BootID, "fixture", "claude", "m1"); err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(worktree, "metasystem", "code.go"), []byte("package fixture\n"), 0o644)
	goalSyncMutationGit(t, worktree, "add", "metasystem/code.go")
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runGoalBranch([]string{"commit", "--goal", "standing-validation", "--kind", "unit", "--unit", "u1", "--root", worktree})
	})
	unit := strings.TrimSpace(stdout)
	if code != 0 || len(unit) != 40 {
		t.Fatalf("unit commit: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stderr = captureCommandOutput(t, true, true, func() int {
		return runGoalBranch([]string{"push", "--goal", "standing-validation", "--root", worktree, "--opid", "real-delegate-push"})
	})
	if code != 0 {
		t.Fatalf("goal push: code=%d stderr=%q", code, stderr)
	}
	installed := filepath.Join(worktree, "bin", "metasystem")
	body, err := os.ReadFile(engine)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(installed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(installed, body, 0o755); err != nil {
		t.Fatal(err)
	}
	digest, err := exec.Command(installed, "util", "sha256", "--file", installed).Output()
	if err != nil {
		t.Fatalf("engine digest: %v", err)
	}
	writeTestingFixtureFile(t, filepath.Join(worktree, "artifacts", "agents", "steward", "identity.json"), []byte(
		`{"repoIdentity":"`+worktree+`","generation":1,"installPath":"`+installed+`","installDigest":"sha256:`+strings.TrimSpace(string(digest))+
			`","mintedAt":"1970-01-01T00:00:00Z","enrollment":"fixture"}`+"\n"), 0o600)
	// Supervision is armed the way the dispatch fixture bed arms it and shut
	// down when the case ends.
	arm := filepath.Join(worktree, "scripts", "agents", "arm-supervision.sh")
	armEnv := append(os.Environ(), "METASYSTEM_BIN="+installed, "METASYSTEM_AGENT_RUNTIME=claude")
	t.Cleanup(func() {
		shutdown := exec.Command(arm, "--repo", worktree, "--shutdown")
		shutdown.Env = armEnv
		if output, err := shutdown.CombinedOutput(); err != nil {
			t.Errorf("supervision shutdown: %v: %s", err, output)
		}
	})
	started, err := exec.Command(installed, "proc", "started-at", "--pid", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		t.Fatalf("started-at: %v", err)
	}
	armCommand := exec.Command(arm, "--repo", worktree, "--session", "real-delegate", "--pid", strconv.Itoa(os.Getpid()),
		"--start-time", strings.TrimSpace(string(started)), "--tag", "real-delegate-fixture")
	armCommand.Env = armEnv
	if output, err := armCommand.CombinedOutput(); err != nil {
		t.Fatalf("arm supervision: %v: %s", err, output)
	}
	return worktree, unit
}
