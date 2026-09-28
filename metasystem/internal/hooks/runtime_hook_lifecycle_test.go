package hooks

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// Lifecycle tests for every runtime and event outside SessionStart's
// outcome matrix: session arguments, runtime and delegate isolation, the
// receipt and SessionEnd events, and the worker's early refusals. Ported
// from runtime-hook-fixtures.sh and the lifecycle legs of the hook beds.

// Every runtime's lifecycle passes the session it was given, or says it had
// none; the start carries its source and SessionEnd retires.
func TestHookSessionArgs(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		event, payload string
		present        bool
	}{
		{"start", `{"session_id":"runtime-session-42","source":"resume"}`, true},
		{"stop", `{"session_id":"runtime-session-42"}`, true},
		{"end", `{"session_id":"runtime-session-42"}`, true},
		{"start", `{"source":"startup"}`, false},
		{"stop", `{}`, false},
		{"end", `{}`, false},
	} {
		t.Run(fmt.Sprintf("%s-present-%t", test.event, test.present), func(t *testing.T) {
			installation := newHookInstallation(t)
			ops := newFakeOps(t, installation)
			runHook(t, installation, ops, hookCall{runtime: "claude", event: test.event, payload: test.payload})
			if len(ops.upRequests) != 1 {
				t.Fatalf("%s made %d up calls: %s", test.event, len(ops.upRequests), ops.trace())
			}
			up := ops.upRequests[0]
			if test.present && (up.RuntimeSession != "runtime-session-42" || up.NoRuntimeSession) ||
				!test.present && (up.RuntimeSession != "" || !up.NoRuntimeSession) {
				t.Fatalf("%s up session arguments = %+v", test.event, up)
			}
			switch test.event {
			case "start":
				want := "startup"
				if test.present {
					want = "resume"
				}
				if up.StartSource != want || up.Tag != "claude:4321" {
					t.Fatalf("start up = %+v", up)
				}
			case "stop":
				if len(ops.turnVerdicts) != 1 || ops.turnVerdicts[0].SessionAbsent == test.present {
					t.Fatalf("stop turn verdict = %+v", ops.turnVerdicts)
				}
				if test.present && ops.turnVerdicts[0].Session != "runtime-session-42" {
					t.Fatalf("stop turn verdict session = %+v", ops.turnVerdicts[0])
				}
			case "end":
				if !up.Retire {
					t.Fatalf("SessionEnd did not retire: %+v", up)
				}
			}
		})
	}
}

// A Claude hook imported by a host whose process another runtime owns stays
// silent on every event and touches no coordinator or brain state.
func TestImportedClaudeHookSkipsAnotherRuntimesProcess(t *testing.T) {
	t.Parallel()
	for _, event := range []string{"start", "receipt", "stop", "end"} {
		t.Run(event, func(t *testing.T) {
			installation := newHookInstallation(t)
			ops := newFakeOps(t, installation)
			ops.identityRuntime = "devin"
			run := runHook(t, installation, ops, hookCall{runtime: "claude", event: event, payload: `{"session_id":"equal-session"}`})
			want := ""
			if event == "stop" {
				want = internalSkipResult + "\n"
			}
			if run.status != 0 || run.stdout != want {
				t.Fatalf("foreign %s = status %d stdout %q", event, run.status, run.stdout)
			}
			if !ops.called("proc find-ancestor pid=777 runtime= all-hosts=true") {
				t.Fatalf("the foreign-runtime guard did not use the all-host selector: %s", ops.trace())
			}
			assertNoBrainEffects(t, ops)
		})
	}
}

func assertNoBrainEffects(t *testing.T, ops *fakeOps) {
	t.Helper()
	for _, effect := range []string{"up ", "brain boot", "brain start-delivered", "steward hook-attempt", "report turn-verdict", "session end", "steward digest-advance"} {
		if ops.called(effect) {
			t.Fatalf("the skipped hook reached %q: %s", effect, ops.trace())
		}
	}
}

// A local delegate proven by exact custody skips before runtime discovery.
func TestUnhintedLocalDelegateSkipsBeforeBrainEffects(t *testing.T) {
	t.Parallel()
	for _, event := range []string{"start", "receipt", "stop", "end"} {
		t.Run(event, func(t *testing.T) {
			installation := newHookInstallation(t)
			ops := newFakeOps(t, installation)
			ops.hookDelegate = func(root, _, job string, _ int) (string, int) {
				if job == "" {
					return `{"delegate":true,"jobId":"job-unhinted-local","matchedPid":1,"comparisonMode":"legacy-seconds"}` + "\n", 0
				}
				return "", 3
			}
			run := runHook(t, installation, ops, hookCall{runtime: "claude", event: event, payload: `{"session_id":"equal-session"}`})
			if run.status != 0 || strings.Trim(run.stdout, "\n") != map[bool]string{true: internalSkipResult}[event == "stop"] {
				t.Fatalf("local delegate %s = status %d stdout %q stderr %q", event, run.status, run.stdout, run.stderr)
			}
			if ops.called("proc find-ancestor") {
				t.Fatalf("a local delegate reached runtime discovery: %s", ops.trace())
			}
			assertNoBrainEffects(t, ops)
		})
	}
}

// Hinted delegate children of every runtime are silent on every event once
// the launcher's custody authenticates them, even concurrently; a forged or
// incomplete hint refuses instead of skipping.
func TestHintedDelegateChildrenAndForgedHints(t *testing.T) {
	t.Parallel()
	for _, runtime := range []string{"claude", "codex", "devin"} {
		for _, event := range []string{"start", "receipt", "stop", "end"} {
			t.Run(runtime+"-"+event, func(t *testing.T) {
				installation := newHookInstallation(t)
				ops := newFakeOps(t, installation)
				ops.hookDelegate = func(root, _, job string, _ int) (string, int) {
					if job == "job-"+runtime {
						return `{"delegate":true,"jobId":"job-` + runtime + `","matchedPid":1,"comparisonMode":"legacy-seconds"}` + "\n", 0
					}
					return "", 1
				}
				env := map[string]string{
					"METASYSTEM_HOOK_DELEGATE_STATE_ROOT": installation.root, "METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT": installation.root,
					"METASYSTEM_HOOK_DELEGATE_JOB": "job-" + runtime,
				}
				run := runHook(t, installation, ops, hookCall{runtime: runtime, event: event, payload: `{"session_id":"equal-session"}`, env: env})
				if run.status != 0 || strings.Trim(run.stdout, "\n") != map[bool]string{true: internalSkipResult}[event == "stop"] {
					t.Fatalf("hinted %s %s = status %d stdout %q", runtime, event, run.status, run.stdout)
				}
				assertNoBrainEffects(t, ops)

				forged := newFakeOps(t, installation)
				env["METASYSTEM_HOOK_DELEGATE_JOB"] = "job-forged"
				forgedRun := runHook(t, installation, forged, hookCall{runtime: runtime, event: event, payload: `{}`, env: env})
				if forgedRun.status != 1 {
					t.Fatalf("a forged hint for %s %s = status %d stdout %q", runtime, event, forgedRun.status, forgedRun.stdout)
				}
				if event != "start" && forgedRun.stderr != "supervision hook refused: supplied delegate context did not authenticate this process ancestry\n" {
					t.Fatalf("forged hint stderr = %q", forgedRun.stderr)
				}
				assertNoBrainEffects(t, forged)
			})
		}
	}
	t.Run("incomplete hint", func(t *testing.T) {
		installation := newHookInstallation(t)
		ops := newFakeOps(t, installation)
		run := runHook(t, installation, ops, hookCall{runtime: "codex", event: "stop", payload: `{}`,
			env: map[string]string{"METASYSTEM_HOOK_DELEGATE_JOB": "job-x"}})
		if run.status != 1 || run.stderr != "supervision hook refused: delegate context hint is incomplete or its engine is unavailable\n" || ops.called("lease hook-delegate") {
			t.Fatalf("incomplete hint = status %d stderr %q trace %s", run.status, run.stderr, ops.trace())
		}
	})
}

// Cold and mission hosts that no delegate custody names reach enrollment.
func TestColdHostsAreNotSuppressedByDelegateClassification(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: `{"session_id":"equal-session"}`})
	if !ops.called("up runtime=claude") {
		t.Fatalf("a cold host was suppressed instead of reaching enrollment: %s", ops.trace())
	}
}

// Unreadable local custody evidence refuses rather than guessing.
func TestLocalCustodyEvidenceUnreadableRefuses(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	ops.hookDelegate = func(string, string, string, int) (string, int) { return "", 1 }
	run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "end", payload: `{}`})
	if run.status != 1 || run.stderr != "supervision hook refused: local delegate custody evidence was unreadable\n" {
		t.Fatalf("unreadable custody = status %d stderr %q", run.status, run.stderr)
	}
}

func TestReceiptEventSurfacesTheRetroCadence(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		status int
		want   string
	}{
		{0, ""},
		{1, `{"systemMessage":"Metasystem retro due: run metasystem receipt status for details, then skills/retro."}` + "\n"},
		{2, `{"systemMessage":"Metasystem receipt check errored; run metasystem receipt status to see why."}` + "\n"},
	} {
		installation := newHookInstallation(t)
		ops := newFakeOps(t, installation)
		ops.receipt = func() (string, string, int) { return "", "", test.status }
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "receipt", payload: `{}`})
		if run.status != 0 || run.stdout != test.want || ops.called("up ") {
			t.Fatalf("receipt status %d = hook status %d stdout %q trace %s", test.status, run.status, run.stdout, ops.trace())
		}
	}
}

// SessionEnd retires the unused Stop authorization before the announcement,
// names every failure, and never arms an unidentified process.
func TestSessionEndRetiresAuthorizationThenAnnouncement(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	ops.stewardPending = func(string) (string, int) { return "2 undelivered; newest: incident\n", 0 }
	ops.sessionEnd = func() int { return 1 }
	run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "end", payload: `{"session_id":"template-human"}`})
	want := `{"systemMessage":"Steward incidents pending: 2 undelivered; newest: incident"}` + "\n" +
		`{"systemMessage":"Metasystem could not durably retire this session's unused stop authorization; later stops must treat it as unsafe."}` + "\n"
	if run.status != 0 || run.stdout != want {
		t.Fatalf("SessionEnd = status %d stdout %q", run.status, run.stdout)
	}
	trace := ops.trace()
	if strings.Index(trace, "session end template-human") > strings.Index(trace, "up runtime=claude") || !ops.upRequests[0].Retire ||
		ops.upRequests[0].Tag != "metasystem-main-claude-template-human" {
		t.Fatalf("SessionEnd order or retirement = %s %+v", trace, ops.upRequests)
	}

	unidentified := newFakeOps(t, installation)
	unidentified.findAncestor = func(string, int, string, bool) (string, int) { return "", 1 }
	unidentified.classify = func(string, string, int) (string, int) { return `{"class":"HUMAN"}`, 0 }
	run = runHook(t, installation, unidentified, hookCall{runtime: "codex", event: "end", payload: `{}`})
	if run.stdout != `{"systemMessage":"Metasystem supervision could not identify the immediate codex agent process; arming was refused."}`+"\n" || unidentified.called("up ") {
		t.Fatalf("unidentified SessionEnd = stdout %q trace %s", run.stdout, unidentified.trace())
	}
}

// The worker's early boundary: an invalid invocation exits 2 without an
// owner; an unregistered runtime or a failed registry refuses; a missing,
// unvalidated or skewed engine answers Stop with its fixed allowance.
func TestLifecycleEarlyBoundary(t *testing.T) {
	t.Parallel()
	stopForm := func(cause string) string { return mustForm(t, "allowed", cause) + "\n" }
	tests := []struct {
		name, runtime, event, payload string
		env                           map[string]string
		configure                     func(hookInstallation, *fakeOps)
		status                        int
		stdout, stderr                string
	}{
		{name: "invalid runtime", runtime: "Bad", event: "stop", status: 2},
		{name: "unknown event", runtime: "claude", event: "bogus", status: 2},
		{name: "tool for another runtime", runtime: "codex", event: "tool", status: 2},
		{name: "unregistered runtime", runtime: "ghost", event: "stop", status: 2, stderr: "supervision hook refused: runtime 'ghost' is not registered\n"},
		{name: "registry failure", runtime: "claude", event: "end", configure: func(_ hookInstallation, ops *fakeOps) {
			ops.runtimeNames = func() (string, int) { return "", 41 }
		}, status: 41, stderr: "supervision hook refused: runtime registry query failed (exit 41)\n"},
		{name: "missing engine", runtime: "claude", event: "stop", configure: func(i hookInstallation, _ *fakeOps) {
			_ = os.Remove(filepath.Join(i.root, "bin", "metasystem"))
		}, stdout: stopForm("engine-missing")},
		{name: "override waives no provenance", runtime: "claude", event: "stop", env: map[string]string{"METASYSTEM_BIN": "/metasystem-missing"}, stdout: stopForm("engine-missing")},
		{name: "unvalidated installation", runtime: "claude", event: "stop", configure: func(_ hookInstallation, ops *fakeOps) {
			ops.stateRoot = func(string) (string, int) { return "", 1 }
		}},
		{name: "engine skew", runtime: "claude", event: "stop", configure: func(_ hookInstallation, ops *fakeOps) {
			ops.stateRoot = func(string) (string, int) { return "", 2 }
		}, stdout: stopForm("engine-skew")},
		{name: "malformed state root", runtime: "claude", event: "stop", configure: func(i hookInstallation, ops *fakeOps) {
			ops.stateRoot = func(string) (string, int) { return i.root + "\nsecond-root-line\n", 0 }
		}, stdout: stopForm("engine-skew")},
		{name: "unreadable Stop payload", runtime: "claude", event: "stop", payload: "not json", status: 1},
		{name: "unidentified checkout", runtime: "claude", event: "stop", configure: func(_ hookInstallation, ops *fakeOps) {
			ops.git = func(...string) (string, error) { return "", fmt.Errorf("exit status 128") }
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installation := newHookInstallation(t)
			ops := newFakeOps(t, installation)
			if test.configure != nil {
				test.configure(installation, ops)
			}
			payload := test.payload
			if payload == "" {
				payload = `{"session_id":"early"}`
			}
			run := runHook(t, installation, ops, hookCall{runtime: test.runtime, event: test.event, payload: payload, env: test.env})
			if run.status != test.status || run.stdout != test.stdout || run.stderr != test.stderr {
				t.Fatalf("%s = status %d stdout %q stderr %q", test.name, run.status, run.stdout, run.stderr)
			}
			if ops.called("up ") || ops.called("report turn-verdict") {
				t.Fatalf("%s reached the turn: %s", test.name, ops.trace())
			}
			if _, err := os.Stat(filepath.Join(installation.root, "artifacts", "agents", "supervision")); !os.IsNotExist(err) {
				t.Fatalf("%s wrote supervision evidence: %v", test.name, err)
			}
		})
	}
}

// Session hygiene happens once at the boundary: an unsafe session string is
// replaced by its sha256 everywhere downstream.
func TestLifecycleHashesUnsafeSessions(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"../../evil"}`})
	want := sha256Text("../../evil")
	if len(ops.turnVerdicts) != 1 || ops.turnVerdicts[0].Session != want || ops.upRequests[0].Session != want {
		t.Fatalf("unsafe session reached the owners: %s", ops.trace())
	}
}

// The payload's cwd never redirects evidence: the state world is the
// engine-validated installation.
func TestStopEvidenceStaysInTheResolvedWorld(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	elsewhere := t.TempDir()
	ops := newFakeOps(t, installation)
	run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop",
		payload: `{"session_id":"nested-freshness","cwd":"` + elsewhere + `","hook_event_name":"Stop"}`})
	if run.status != 0 || readHookLog(t, installation) == "" {
		t.Fatalf("stop = status %d, no evidence in the installation", run.status)
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("payload cwd received evidence: %v", entries)
	}
}

// A Stop from a nested installation or a linked worktree runs the same turn
// against the resolved world.
func TestStopFromALinkedWorktreeCompletesInThePrimary(t *testing.T) {
	t.Parallel()
	primary := newHookInstallation(t)
	worktreeTop, _ := filepath.EvalSymlinks(t.TempDir())
	harness := filepath.Join(worktreeTop, "metasystem")
	if err := os.MkdirAll(harness, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(primary.root, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(primary.root, "metasystem", "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(primary.root, "metasystem", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(primary.root, "metasystem", "bin", "metasystem"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	worktree := hookInstallation{root: harness}
	ops := newFakeOps(t, primary)
	ops.git = func(args ...string) (string, error) {
		switch strings.Join(args[2:], " ") {
		case "rev-parse --path-format=absolute --git-dir --git-common-dir":
			return primary.root + "/.git/worktrees/wt\n" + primary.root + "/.git\n", nil
		case "rev-parse --show-toplevel":
			return worktreeTop + "\n", nil
		case "config --get metasystem.goal.machine":
			return "", fmt.Errorf("unset")
		}
		return "", fmt.Errorf("unexpected %v", args)
	}
	run := runHook(t, worktree, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"nested-wt-parent"}`})
	if run.status != 0 || strings.Contains(run.stdout, "stop-hook-output-was-unreadable") || !strings.Contains(run.stdout, "presented:") {
		t.Fatalf("worktree stop = status %d stdout %q", run.status, run.stdout)
	}
	if !ops.called("path state-root " + primary.root + "/metasystem") {
		t.Fatalf("the worktree hook did not validate the primary installation: %s", ops.trace())
	}
	if _, err := os.Stat(filepath.Join(harness, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("the worktree hook materialized state in the worktree: %v", err)
	}
	_ = io.Discard
}
