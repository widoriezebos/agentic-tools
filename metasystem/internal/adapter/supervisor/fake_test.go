package supervisor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

func assertCAS(t *testing.T, call []string, job, expect, status string) string {
	t.Helper()
	want := []string{"__record-cas", "--job", job, "--expect", expect, "--status", status, "--patch"}
	if len(call) != len(want)+1 || !reflect.DeepEqual(call[:len(want)], want) {
		t.Fatalf("record-cas call = %q, want %q PATCH", call, want)
	}
	return call[len(want)]
}

func assertPatch(t *testing.T, path string, failure any, phase string, usage bool) {
	t.Helper()
	patch := readJSON(t, path)
	if patch["error"] != failure || patch["phase"] != phase {
		t.Fatalf("patch %s = %v, want error %v phase %s", path, patch, failure, phase)
	}
	if usage != (patch["usage"] != nil) {
		t.Fatalf("patch %s usage = %v, want present=%v", path, patch["usage"], usage)
	}
}

// TestFakeCompleteValidSequence is the default round: the script's full
// happy path through the handshake, both events, the canned return, and the
// completed compare-and-swap.
func TestFakeCompleteValidSequence(t *testing.T) {
	t.Parallel()
	f := newFakeInstall(t, installOptions{})
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
	}
	if got := f.logText(); got != "fake supervisor started value=fake-tag-1\n" {
		t.Fatalf("job log = %q", got)
	}
	if got := readText(t, f.roundFile("raw.out")); got != "fake raw output\n" {
		t.Fatalf("raw = %q", got)
	}
	if got, want := readText(t, f.heartbeat()), `{"pid":`+strconv.Itoa(os.Getpid())+`,"pgid":`+strconv.Itoa(os.Getpid())+`,"instanceTag":"fake-tag-1"}`+"\n"; got != want {
		t.Fatalf("heartbeat = %q, want %q", got, want)
	}
	if got, want := readText(t, f.roundFile("build-cache.txt")), filepath.Join(f.root, "user-cache", "metasystem-delegate-go-build")+"\n"; got != want {
		t.Fatalf("build cache = %q, want the machine delegate cache %q", got, want)
	}
	if got, want := readText(t, f.roundFile("events.jsonl")),
		`{"event":"session-established","sessionId":"fake-session-fake-job-1","round":1}`+"\n"+`{"event":"turn.completed","topLevel":true}`+"\n"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
	calls := f.dispatcher.calls(t)
	wantHandshake := []string{"__handshake", "--job", f.job, "--session", "fake-session-fake-job-1", "--turn", "fake-turn-1",
		"--model", "fake-model", "--effective", f.roundFile("effective-permissions.json"), "--signal", "fake-signal"}
	if len(calls) != 2 || !reflect.DeepEqual(calls[0], wantHandshake) {
		t.Fatalf("calls = %q, want handshake %q then record-cas", calls, wantHandshake)
	}
	patch := assertCAS(t, calls[1], f.job, "running", "completed")
	if patch != f.roundFile("terminal-patch.json") {
		t.Fatalf("patch path = %s", patch)
	}
	assertPatch(t, patch, nil, "completed", true)
	if usage := readJSON(t, f.roundFile("usage.json")); usage["inputTokens"] != float64(11) {
		t.Fatalf("fake usage = %v", usage)
	}
	// The shared return normalization rewrites the round return and its
	// markdown pointer (the script left its own "# Fake return" heading).
	if got := readText(t, f.roundFile("return.md")); got != "# Agent return\n\nCanonical JSON: return.json\n" {
		t.Fatalf("return.md = %q", got)
	}
	if ret := readJSON(t, f.roundFile("return.json")); ret["sessionId"] != "fake-session-fake-job-1" || ret["mode"] != "implement" {
		t.Fatalf("return = %v", ret)
	}
	if exists(f.roundFile("protocol-violation.txt")) {
		t.Fatal("a completed round kept its violation file")
	}
	record := readJSON(t, filepath.Join(f.agents(), "jobs", f.job+".json"))
	if capability := record["launchCapability"].(map[string]any); capability["status"] != "consumed" {
		t.Fatalf("launch capability = %v, want consumed", capability)
	}
	if effective := readJSON(t, f.roundFile("effective-permissions.json")); effective["network"] != "deny" {
		t.Fatalf("effective = %v", effective)
	}
}

// TestFakeTailMarkers covers each marker the script acts on after the
// holds, one round each, plus the marker grammar: grep -Fqi (any case,
// anywhere) and the staged task direction as a second source.
func TestFakeTailMarkers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, prompt, staged string
		check                func(t *testing.T, f *fakeInstall)
	}{
		{name: "nested-agent-events", prompt: "FAKE:nested-agent-events\n", check: func(t *testing.T, f *fakeInstall) {
			events := readText(t, f.roundFile("events.jsonl"))
			if !strings.HasSuffix(events, `{"event":"agent.completed","agent":"nested","topLevel":false}`+"\n"+`{"event":"turn.completed","agent":"root","topLevel":true}`+"\n") {
				t.Fatalf("events = %q", events)
			}
		}},
		{name: "no-event-stream", prompt: "FAKE:no-event-stream\n", check: func(t *testing.T, f *fakeInstall) {
			if exists(f.roundFile("events.jsonl")) {
				t.Fatalf("events written: %q", readText(t, f.roundFile("events.jsonl")))
			}
		}},
		{name: "hook-unavailable any case", prompt: "please fake:HOOK-UNAVAILABLE now\n", check: func(t *testing.T, f *fakeInstall) {
			if !strings.HasSuffix(f.logText(), "hooks unavailable; polling fallback used\n") {
				t.Fatalf("log = %q", f.logText())
			}
		}},
		{name: "mirror-failure", prompt: "FAKE:mirror-failure\n", check: func(t *testing.T, f *fakeInstall) {
			if !exists(filepath.Join(f.agents(), f.rootJob, ".mirror-fail-once")) {
				t.Fatal("mirror fail-once marker missing")
			}
		}},
		{name: "interrupted-atomic-write", prompt: "FAKE:interrupted-atomic-write\n", check: func(t *testing.T, f *fakeInstall) {
			if got := readText(t, filepath.Join(f.agents(), "record-locks", f.job+".interrupted")); got != `{"status":"corrupt` {
				t.Fatalf("interrupted write = %q", got)
			}
		}},
		{name: "Fake-Argument is data", prompt: "Working Mode: implement\nFake-Argument: \t$(touch /tmp/never) ; rm -rf /\nFake-Argument: second\n", check: func(t *testing.T, f *fakeInstall) {
			if got := readText(t, f.roundFile("raw.out")); got != "fake raw output\nprovider argument value=$(touch /tmp/never) ; rm -rf /\n" {
				t.Fatalf("raw = %q", got)
			}
		}},
		{name: "effective-narrower", prompt: "FAKE:effective-narrower\n", check: func(t *testing.T, f *fakeInstall) {
			if effective := readJSON(t, f.roundFile("effective-permissions.json")); effective["network"] != "deny" {
				t.Fatalf("effective = %v", effective)
			}
		}},
		{name: "staged direction marker", prompt: "Working Mode: implement\nreference stanza only\n", staged: "FAKE:hook-unavailable\n", check: func(t *testing.T, f *fakeInstall) {
			if !strings.Contains(f.logText(), "hooks unavailable; polling fallback used\n") {
				t.Fatalf("log = %q", f.logText())
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeInstall(t, installOptions{prompt: tc.prompt, staged: tc.staged})
			if code := f.run(); code != 0 {
				t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
			}
			tc.check(t, f)
			if call := casCall(f.dispatcher.calls(t)); call == nil || call[6] != "completed" {
				t.Fatalf("calls = %q, want a completed compare", f.dispatcher.calls(t))
			}
		})
	}
}

// TestFakeMissingSessionID: an empty session is refused with the code
// dispatch.sh's handshake evaluation landed for it (the shared handshake
// never records an empty session, so observe names the refusal).
func TestFakeMissingSessionID(t *testing.T) {
	t.Parallel()
	f := newFakeInstall(t, installOptions{prompt: "FAKE:missing-session-id\n"})
	if code := f.run(); code != 1 {
		t.Fatalf("exit %d, want 1 on a refused handshake", code)
	}
	calls := f.dispatcher.calls(t)
	if len(calls) != 1 {
		t.Fatalf("calls = %q, want one pending failure", calls)
	}
	assertPatch(t, assertCAS(t, calls[0], f.job, "pending", "failed"), "handshake_missing_session_id", "handshake", false)
	if exists(f.roundFile("events.jsonl")) {
		t.Fatal("a refused handshake still wrote the session event")
	}
}

// TestFakeSessions covers the session each verb and round claims, the
// follow-up resume check, and the resume-collision marker.
func TestFakeSessions(t *testing.T) {
	t.Parallel()
	t.Run("fresh dispatch above round 1", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{round: 2, parent: "fake-root-1"})
		if code := f.run(); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
		calls := f.dispatcher.calls(t)
		if got := flagValue(calls[0], "--session"); got != "fake-session-fake-root-1-fresh-2" {
			t.Fatalf("session = %q", got)
		}
		if got := flagValue(calls[0], "--turn"); got != "fake-turn-2" {
			t.Fatalf("turn = %q", got)
		}
	})
	t.Run("follow-up resumes the chain session", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{verb: "follow-up", round: 2, parent: "fake-root-1", sessionID: "fake-session-fake-root-1"})
		if code := f.run(); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
		if call := casCall(f.dispatcher.calls(t)); call[6] != "completed" {
			t.Fatalf("cas = %q", call)
		}
	})
	t.Run("follow-up on another session collides", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{verb: "follow-up", round: 2, parent: "fake-root-1", sessionID: "someone-else"})
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
		calls := f.dispatcher.calls(t)
		if got := flagValue(calls[0], "--session"); got != "someone-else" {
			t.Fatalf("handshake session = %q", got)
		}
		assertPatch(t, assertCAS(t, calls[1], f.job, "running", "failed"), "resume_collision", "resume", true)
	})
	t.Run("resume-collision marker", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{prompt: "FAKE:resume-collision\n"})
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
		calls := f.dispatcher.calls(t)
		assertPatch(t, assertCAS(t, calls[1], f.job, "running", "failed"), "resume_collision", "resume", true)
		if exists(f.roundFile("return.json")) {
			t.Fatal("a collided round wrote a return")
		}
	})
	t.Run("a failing compare still ends the round 1", func(t *testing.T) {
		// The shared layer lands refusals; the script exited with the
		// compare's own status.
		f := newFakeInstall(t, installOptions{prompt: "FAKE:resume-collision\n"})
		f.dispatcher.Statuses["__record-cas"] = 4
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
	})
}

// TestFakeEffectiveWider: the widened envelope is refused by the shared
// comparison with the code dispatch.sh's handshake evaluation landed.
func TestFakeEffectiveWider(t *testing.T) {
	t.Parallel()
	f := newFakeInstall(t, installOptions{prompt: "FAKE:effective-wider\n"})
	if code := f.run(); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if effective := readJSON(t, f.roundFile("effective-permissions.json")); effective["network"] != "allow" {
		t.Fatalf("effective = %v", effective)
	}
	calls := f.dispatcher.calls(t)
	if len(calls) != 1 {
		t.Fatalf("calls = %q", calls)
	}
	assertPatch(t, assertCAS(t, calls[0], f.job, "pending", "failed"), "permissions_mismatch:network", "handshake", false)
}

// TestFakeEffectiveUnreadableFailsClosed: an effective envelope the shared
// comparison cannot read refuses the launch, never passes it.
func TestFakeEffectiveUnreadableFailsClosed(t *testing.T) {
	t.Parallel()
	f := newFakeInstall(t, installOptions{prompt: "FAKE:effective-unreadable\n"})
	if code := f.run(); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	calls := f.dispatcher.calls(t)
	if len(calls) != 1 {
		t.Fatalf("calls = %q", calls)
	}
	assertPatch(t, assertCAS(t, calls[0], f.job, "pending", "failed"), "permissions_check_unreadable", "handshake", false)
}

// TestFakeHandshakeFailure fails the pending round before any effective
// permissions exist.
func TestFakeHandshakeFailure(t *testing.T) {
	t.Parallel()
	f := newFakeInstall(t, installOptions{prompt: "FAKE:handshake-failure\n"})
	f.dispatcher.Statuses["__record-cas"] = 5
	if code := f.run(); code != 1 {
		t.Fatalf("exit %d, want 1 whatever the compare says", code)
	}
	calls := f.dispatcher.calls(t)
	if len(calls) != 1 {
		t.Fatalf("calls = %q", calls)
	}
	patch := assertCAS(t, calls[0], f.job, "pending", "failed")
	if patch != f.roundFile("pending-failure.json") {
		t.Fatalf("patch = %s", patch)
	}
	assertPatch(t, patch, "authentication_failed", "handshake", false)
}

// TestFakeReferenceVerification covers a mismatched reference, an
// unreadable composition, and the tamper hook.
func TestFakeReferenceVerification(t *testing.T) {
	t.Parallel()
	reference := func(f *fakeInstall, digest string, size int) {
		mustJSON(t, f.roundFile("composition.json"), map[string]any{"references": []any{map[string]any{
			"slot": "task-direction", "purpose": "brief", "path": "staged/task-direction.md",
			"openPath": f.roundFile("staged/task-direction.md"), "digest": digest, "bytes": size, "lifetime": "round",
		}}})
	}
	t.Run("mismatch", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{staged: "direction\n"})
		reference(f, strings.Repeat("0", 64), 10)
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
		line := "REFERENCE_MISMATCH path=staged/task-direction.md open=" + f.roundFile("staged/task-direction.md") + " expected=" + strings.Repeat("0", 64) + ":10 found="
		if !strings.Contains(f.logText(), line) || !strings.Contains(f.stderr.String(), line) {
			t.Fatalf("log %q / stderr %q lack %q", f.logText(), f.stderr.String(), line)
		}
		calls := f.dispatcher.calls(t)
		if len(calls) != 1 {
			t.Fatalf("calls = %q", calls)
		}
		patch := assertCAS(t, calls[0], f.job, "pending", "failed")
		if patch != f.roundFile("pending-failure.json") {
			t.Fatalf("patch = %s", patch)
		}
		assertPatch(t, patch, "reference_mismatch:staged/task-direction.md", "launch", false)
	})
	t.Run("unreadable composition", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{})
		os.Remove(f.roundFile("composition.json"))
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
		if !strings.Contains(f.logText(), "read composition record") {
			t.Fatalf("log = %q", f.logText())
		}
		assertPatch(t, f.roundFile("pending-failure.json"), "reference_mismatch:composition", "launch", false)
	})
	t.Run("tamper", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{staged: "direction\n"})
		sum := sha256.Sum256([]byte("direction\n"))
		reference(f, hex.EncodeToString(sum[:]), len("direction\n"))
		f.env["METASYSTEM_FAKE_TAMPER_REFERENCE"] = "1"
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
		if got := readText(t, f.roundFile("staged/task-direction.md")); got != "direction\ntampered\n" {
			t.Fatalf("staged = %q", got)
		}
		assertPatch(t, f.roundFile("pending-failure.json"), "reference_mismatch:staged/task-direction.md", "launch", false)
	})
	t.Run("verified reference launches", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{staged: "direction\n"})
		sum := sha256.Sum256([]byte("direction\n"))
		reference(f, hex.EncodeToString(sum[:]), len("direction\n"))
		if code := f.run(); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
	})
}

// TestFakeCustodialCritique holds a registered child until the release
// file appears, bounded by the hold cap on the supervisor's clock.
func TestFakeCustodialCritique(t *testing.T) {
	t.Parallel()
	t.Run("released", func(t *testing.T) {
		release := filepath.Join(t.TempDir(), "release")
		f := newFakeInstall(t, installOptions{prompt: "  FAKE:custodial-critique=" + release + "\n"})
		f.env["METASYSTEM_HEARTBEAT_INTERVAL_MS"] = "20"
		ended := make(chan struct{})
		defer close(ended)
		go func() {
			// The hold records its argv before the release stops it; a run
			// that ended without one stops this wait with it.
			for len(f.holds()) != 1 {
				select {
				case <-ended:
					return
				default:
					time.Sleep(10 * time.Millisecond)
				}
			}
			os.WriteFile(release, nil, 0o644)
		}()
		if code := f.run(); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
		calls := f.dispatcher.calls(t)
		if got := callNames(calls); !reflect.DeepEqual(got, []string{"__handshake", "__register-custody", "__record-cas"}) {
			t.Fatalf("calls = %q", calls)
		}
		pid := strings.TrimSpace(readText(t, f.roundFile("custody-child.pid")))
		if flagValue(calls[1], "--pid") != pid || flagValue(calls[1], "--job") != f.job {
			t.Fatalf("custody call %q, child %s", calls[1], pid)
		}
		holdPid, _ := strconv.Atoi(pid)
		if argv := f.holds()[holdPid]; !reflect.DeepEqual(argv, []string{"--tag", f.tag}) {
			t.Fatalf("hold argv = %q", argv)
		}
		if processAlive(holdPid) {
			t.Fatal("released hold child still alive")
		}
	})
	t.Run("cap expires", func(t *testing.T) {
		release := filepath.Join(t.TempDir(), "never")
		f := newFakeInstall(t, installOptions{prompt: "FAKE:custodial-critique=" + release + "\n"})
		f.env["METASYSTEM_FAKE_CRITIQUE_HOLD_CAP_SEC"] = "2"
		// The supervisor's clock: the cap is read on it, never on the
		// wall (the custodian's own waits share it, so no sleep count is
		// asserted).
		f.clock = &stepClock{now: time.Unix(1_000_000, 0), onSleep: func(int) { time.Sleep(time.Millisecond) }}
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
		calls := f.dispatcher.calls(t)
		assertPatch(t, assertCAS(t, calls[2], f.job, "running", "failed"), "fixture_release_timeout", "execute", true)
		pid, _ := strconv.Atoi(strings.TrimSpace(readText(t, f.roundFile("custody-child.pid"))))
		if processAlive(pid) {
			t.Fatal("timed-out hold child still alive")
		}
	})
	for _, tc := range []struct{ name, release, cap string }{
		{"relative release", "relative/release", ""},
		{"invalid cap", "/abs/release", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeInstall(t, installOptions{prompt: "FAKE:custodial-critique=" + tc.release + "\n"})
			if tc.cap != "" {
				f.env["METASYSTEM_FAKE_CRITIQUE_HOLD_CAP_SEC"] = tc.cap
			}
			if code := f.run(); code != 1 {
				t.Fatalf("exit %d, want 1", code)
			}
			calls := f.dispatcher.calls(t)
			assertPatch(t, assertCAS(t, calls[1], f.job, "running", "failed"), "invalid_fixture_control", "execute", true)
			if len(f.holds()) != 0 {
				t.Fatal("an invalid control started a hold")
			}
		})
	}
	t.Run("custody refused", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{prompt: "FAKE:custodial-critique=/abs/release\n"})
		f.dispatcher.Statuses["__register-custody"] = 1
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
		calls := f.dispatcher.calls(t)
		assertPatch(t, assertCAS(t, calls[2], f.job, "running", "failed"), "custody_registration", "execute", true)
		for pid := range f.holds() {
			if processAlive(pid) {
				t.Fatal("refused custody left the hold alive")
			}
		}
	})
	t.Run("first value line wins per source", func(t *testing.T) {
		release := filepath.Join(t.TempDir(), "release")
		mustWrite(t, release, "")
		// The prompt's first marker line is empty, so the staged
		// direction's value is used.
		f := newFakeInstall(t, installOptions{
			prompt: "FAKE:custodial-critique=\nFAKE:custodial-critique=relative\n",
			staged: "FAKE:custodial-critique=" + release + "\n",
		})
		if code := f.run(); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
	})
}

// TestFakeWorktreeFile covers the round-1 guarded write and its refusals.
func TestFakeWorktreeFile(t *testing.T) {
	t.Parallel()
	t.Run("writes in round 1", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{prompt: "FAKE:worktree-file=pkg/sub/work.txt\n"})
		if code := f.run(); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
		if got := readText(t, filepath.Join(f.workspace, "pkg", "sub", "work.txt")); got != "fake envelope write probe\n" {
			t.Fatalf("worktree file = %q", got)
		}
	})
	t.Run("ignored above round 1", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{round: 2, parent: "fake-root-1", prompt: "FAKE:worktree-file=later.txt\n"})
		if code := f.run(); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
		if exists(filepath.Join(f.workspace, "later.txt")) {
			t.Fatal("a continuation round recreated its predecessor's file")
		}
	})
	for _, path := range []string{"/abs.txt", "../escape.txt", "a/../b.txt", "a/./b.txt", "."} {
		t.Run("path "+path, func(t *testing.T) {
			f := newFakeInstall(t, installOptions{prompt: "FAKE:worktree-file=" + path + "\n"})
			if code := f.run(); code != 1 {
				t.Fatalf("exit %d, want 1", code)
			}
			calls := f.dispatcher.calls(t)
			assertPatch(t, assertCAS(t, calls[1], f.job, "running", "failed"), "worktree_file_path", "execute", true)
		})
	}
	t.Run("refused by the envelope", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{prompt: "FAKE:worktree-file=out.txt\n", writeRoots: []string{}})
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1", code)
		}
		calls := f.dispatcher.calls(t)
		assertPatch(t, assertCAS(t, calls[1], f.job, "running", "failed"), "worktree_write_refused", "execute", true)
		if exists(filepath.Join(f.workspace, "out.txt")) {
			t.Fatal("a refused write landed")
		}
	})
}

// TestFakeReturnValidation covers the malformed-return marker and a canned
// return the validator refuses.
func TestFakeReturnValidation(t *testing.T) {
	t.Parallel()
	t.Run("malformed-return", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{prompt: "FAKE:malformed-return\n"})
		// The shared round lands the protocol error and exits 1 (the
		// script exited 0 after it).
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1 after the protocol error", code)
		}
		if got := readText(t, f.roundFile("return.json")); got != "{malformed\n" {
			t.Fatalf("return = %q", got)
		}
		violation := readText(t, f.roundFile("protocol-violation.txt"))
		// The shared adjudication normalizes before validating, so an
		// unparseable return is a normalization violation.
		if violation != "return normalization failed: no JSON return object found in runtime output\n" {
			t.Fatalf("violation file = %q", violation)
		}
		if !strings.Contains(f.logText(), "malformed return\n"+violation) {
			t.Fatalf("log = %q", f.logText())
		}
		calls := f.dispatcher.calls(t)
		want := []string{"__protocol-error", "--job", f.job, "--expect", "running", "--violation-file", f.roundFile("protocol-violation.txt")}
		if len(calls) != 2 || !reflect.DeepEqual(calls[1], want) {
			t.Fatalf("calls = %q", calls)
		}
		if exists(f.roundFile("return.md")) {
			t.Fatal("malformed return wrote return.md")
		}
	})
	t.Run("malformed-return protocol-error status", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{prompt: "FAKE:malformed-return\n"})
		f.dispatcher.Statuses["__protocol-error"] = 3
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d, want 1 (the script exited with the callback's 3)", code)
		}
	})
	t.Run("canned return refused", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{})
		f.dispatcher.KeepSession = true // the record's sessionId stays null
		if code := f.run(); code != 1 {
			t.Fatalf("exit %d", code)
		}
		violation := readText(t, f.roundFile("protocol-violation.txt"))
		if !strings.Contains(violation, "violation: $.sessionId identity mismatch") {
			t.Fatalf("violation = %q", violation)
		}
		if !strings.HasSuffix(f.logText(), violation) {
			t.Fatalf("log = %q", f.logText())
		}
		if got := callNames(f.dispatcher.calls(t)); !reflect.DeepEqual(got, []string{"__handshake", "__protocol-error"}) {
			t.Fatalf("calls = %q", got)
		}
	})
	t.Run("lost compare is success", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{})
		f.dispatcher.Statuses["__record-cas"] = 3
		if code := f.run(); code != 0 {
			t.Fatalf("exit %d, want 0", code)
		}
	})
}

// TestFakeNoSessionSignal holds a util-hold child to its exit, then
// continues the round.
func TestFakeNoSessionSignal(t *testing.T) {
	t.Parallel()
	f := newFakeInstall(t, installOptions{prompt: "FAKE:no-session-signal\n"})
	handshake := make(chan struct{}, 1)
	f.dispatcher.OnRun = func(args []string) {
		if args[0] != "__handshake" {
			return
		}
		for pid := range f.holds() {
			if !exists(filepath.Join(f.holdDir, ".exited."+strconv.Itoa(pid))) {
				t.Errorf("a handshake ran before hold %d reported its exit", pid)
			}
		}
		handshake <- struct{}{}
	}
	done := make(chan int, 1)
	go func() { done <- f.run() }()
	var pid int
	waitFor(t, "the hold child", func() bool {
		for holdPid := range f.holds() {
			pid = holdPid
			return true
		}
		return false
	})
	if argv := f.holds()[pid]; !reflect.DeepEqual(argv, []string{"--tag", f.tag}) {
		t.Fatalf("hold argv = %q", argv)
	}
	select {
	case code := <-done:
		t.Fatalf("supervisor exited %d while its hold lived", code)
	default:
	}
	if !strings.Contains(f.logText(), "ordinary output without a session-established event\n") {
		t.Fatalf("log = %q", f.logText())
	}
	if len(f.dispatcher.calls(t)) != 0 {
		t.Fatal("a handshake ran before the hold ended")
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != 0 {
		t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
	}
	select {
	case <-handshake:
	default:
		t.Fatal("no handshake followed the hold's exit")
	}
}

// TestFakeProcessLossBehaviors run in their own process: the supervisor
// kills itself without a terminal compare-and-swap.
func TestFakeProcessLossBehaviors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		marker     string
		wantCalls  []string
		wantChild  bool
		wantReturn bool
	}{
		{marker: "pending-process-loss"},
		{marker: "process-loss", wantCalls: []string{"__handshake"}, wantChild: true},
		{marker: "return-then-process-loss", wantCalls: []string{"__handshake"}, wantReturn: true},
	} {
		t.Run(tc.marker, func(t *testing.T) {
			f := newFakeInstall(t, installOptions{prompt: "FAKE:" + tc.marker + "\n"})
			command := f.startSubprocess()
			err := command.Wait()
			if signalOfExit(err) != syscall.SIGKILL {
				t.Fatalf("supervisor ended %v, want SIGKILL; output %s", err, f.subprocessOutput())
			}
			if got := readText(t, f.heartbeat()); got != "{\"lost\":true}\n" {
				t.Fatalf("heartbeat = %q", got)
			}
			if got := callNames(f.dispatcher.calls(t)); !reflect.DeepEqual(got, tc.wantCalls) {
				t.Fatalf("calls = %q, want %q", got, tc.wantCalls)
			}
			if tc.wantChild {
				// The hold was started and recorded before the supervisor
				// died. Its argv is TestFakeHoldBehaviors' (the same
				// recorded-hold path); whether the orphan survives is not
				// asserted: the test harness's fixture custodian reaps every
				// descendant of a dead test process, racing the hold's own
				// argv record.
				if pid, err := strconv.Atoi(strings.TrimSpace(readText(t, f.roundFile("child.pid")))); err != nil || pid < 2 {
					t.Fatalf("child.pid = %q", readText(t, f.roundFile("child.pid")))
				}
			} else if exists(f.roundFile("child.pid")) {
				t.Fatal("unexpected child.pid")
			}
			if tc.wantReturn != exists(f.roundFile("return.json")) {
				t.Fatalf("return.json present = %v, want %v", !tc.wantReturn, tc.wantReturn)
			}
			if tc.wantReturn && readText(t, f.roundFile("return.md")) == "" {
				t.Fatal("return.md missing")
			}
		})
	}
}

// TestFakeHoldBehaviors: timeout, concurrent-turn, and cap-hold-round hold
// with a recorded child and a fresh heartbeat until stopped.
func TestFakeHoldBehaviors(t *testing.T) {
	t.Parallel()
	for _, prompt := range []string{"FAKE:timeout\n", "FAKE:concurrent-turn\n", "FAKE:cap-hold-round=1\n"} {
		t.Run(strings.TrimSpace(prompt), func(t *testing.T) {
			f := newFakeInstall(t, installOptions{prompt: prompt})
			f.env["METASYSTEM_HEARTBEAT_INTERVAL_MS"] = "20"
			command := f.startSubprocess()
			pid := f.waitPidFile("child.pid")
			waitFor(t, "the hold's argv record", func() bool { return len(f.holds()[pid]) > 0 })
			if argv := f.holds()[pid]; !reflect.DeepEqual(argv, []string{"--tag", f.tag, "--stopped-file", f.roundFile("child.stopped")}) {
				t.Fatalf("hold argv = %q", argv)
			}
			info, err := os.Stat(f.heartbeat())
			if err != nil {
				t.Fatal(err)
			}
			first := info.ModTime()
			waitFor(t, "a heartbeat refresh", func() bool {
				info, err := os.Stat(f.heartbeat())
				return err == nil && info.ModTime().After(first)
			})
			if strings.Contains(readText(t, f.heartbeat()), "lost") {
				t.Fatal("a holding round reported itself lost")
			}
			if got := callNames(f.dispatcher.calls(t)); !reflect.DeepEqual(got, []string{"__handshake"}) {
				t.Fatalf("calls = %q", got)
			}
			syscall.Kill(command.Process.Pid, syscall.SIGTERM)
			if err := command.Wait(); signalOfExit(err) != syscall.SIGTERM {
				t.Fatalf("holding supervisor ended %v, want the default SIGTERM death", err)
			}
		})
	}
	t.Run("cap-hold-round of another round completes", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{prompt: "FAKE:cap-hold-round=2\n"})
		if code := f.run(); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
		if len(f.holds()) != 0 {
			t.Fatal("round 1 held for round 2's control")
		}
	})
}

// TestFakeCancelRace: SIGTERM during the hold completes the round valid and
// exits 0.
//
// The hold outliving the supervisor is the product contract, so nothing in
// the supervisor's own process may reap it. A fixture custodian there (the
// test binary's TestMain run in full inside the subprocess) observes its
// owner's descendants every poll and kills them when the owner exits; under
// suite load the subprocess lived past the default 250ms poll and the hold
// died after this assertion's reading had become racy. The poll is set to
// 1ms so any such custodian observes the hold at once, and its exit is
// awaited before the hold is judged.
func TestFakeCancelRace(t *testing.T) {
	t.Parallel()
	f := newFakeInstall(t, installOptions{prompt: "FAKE:cancel-race\n"})
	f.env["METASYSTEM_HEARTBEAT_INTERVAL_MS"] = "20"
	f.env[identity.FixtureCustodianPollEnv] = "1ms"
	command := f.startSubprocess()
	f.waitPidFile("child.pid")
	custodians := fixtureCustodiansOf(t, command.Process.Pid)
	syscall.Kill(command.Process.Pid, syscall.SIGTERM)
	if err := command.Wait(); err != nil {
		t.Fatalf("cancel-race supervisor ended %v, want exit 0; output %s", err, f.subprocessOutput())
	}
	for _, custodian := range custodians {
		waitFor(t, "the supervisor's fixture custodian to finish", func() bool {
			return identity.AliveRef(identity.KernelProber{}, custodian) != identity.Alive
		})
	}
	if len(custodians) != 0 {
		t.Errorf("the supervisor subprocess ran %d fixture custodian(s); they reap its children when it exits, which the product supervisor never does", len(custodians))
	}
	calls := f.dispatcher.calls(t)
	if got := callNames(calls); !reflect.DeepEqual(got, []string{"__handshake", "__record-cas"}) {
		t.Fatalf("calls = %q", got)
	}
	assertPatch(t, assertCAS(t, calls[1], f.job, "running", "completed"), nil, "completed", true)
	pid, _ := strconv.Atoi(strings.TrimSpace(readText(t, f.roundFile("child.pid"))))
	if !processAlive(pid) {
		t.Fatal("the trap stopped the hold child; the script left it to the dispatcher")
	}
}

// TestFakeSuperviseRefusals covers the launch refusals before the round.
func TestFakeSuperviseRefusals(t *testing.T) {
	t.Parallel()
	t.Run("missing flag", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{})
		args := f.superviseArgs()
		if code := f.run(args[:len(args)-2]...); code != 2 {
			t.Fatalf("exit %d, want 2", code)
		}
		if !strings.Contains(f.stderr.String(), "probe --root ROOT [--profile current|old|unverified-network] [--age-days N]") {
			t.Fatalf("usage = %q", f.stderr.String())
		}
	})
	t.Run("capability spent twice", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{})
		if code := f.run(); code != 0 {
			t.Fatalf("first exit %d", code)
		}
		editRecord(filepath.Join(f.agents(), "jobs", f.job+".json"), func(record map[string]any) { record["status"] = "pending" })
		// The shared preparation answers every launch refusal with its
		// usage and 2 (the script exited 1).
		if code := f.run(); code != 2 {
			t.Fatalf("replay exit %d, want 2", code)
		}
		if !strings.Contains(f.stderr.String(), "already consumed") {
			t.Fatalf("stderr = %q", f.stderr.String())
		}
	})
	// The poll interval is the shared preparation's (usage, 2); the
	// heartbeat interval is the fake's prepare's (an error, 1; the script
	// exited 2 for both).
	for name, want := range map[string]int{"METASYSTEM_HEARTBEAT_INTERVAL_MS": 1, "METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS": 2} {
		t.Run("invalid "+name, func(t *testing.T) {
			f := newFakeInstall(t, installOptions{})
			f.env[name] = "0"
			if code := f.run(); code != want {
				t.Fatalf("exit %d, want %d", code, want)
			}
			if !strings.Contains(f.stderr.String(), "fake adapter interval must be a positive integer in milliseconds") {
				t.Fatalf("stderr = %q", f.stderr.String())
			}
		})
	}
	t.Run("gate never opens", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{})
		os.Remove(f.gate)
		f.env["METASYSTEM_HOST_START_GATE_TIMEOUT_SEC"] = "1"
		if code := f.run(); code != 2 {
			t.Fatalf("exit %d, want 2", code)
		}
		if !strings.Contains(f.stderr.String(), "start gate never opened: "+f.gate) {
			t.Fatalf("stderr = %q", f.stderr.String())
		}
		if exists(filepath.Join(f.agents(), "jobs", f.job+".log")) {
			t.Fatal("a round started behind a closed gate")
		}
	})
}

// TestFakeRecordBuildCachePath: the fake records the path a real runtime's
// round records (disk-lifetimes A7), the one machine delegate cache for a
// job worktree and a shared checkout alike, and makes no per-chain cache.
func TestFakeRecordBuildCachePath(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root, _ := filepath.EvalSymlinks(t.TempDir())
	agents := filepath.Join(root, "artifacts", "agents")
	repo := filepath.Join(root, "repo")
	worktree := filepath.Join(agents, "worktrees", "job-1")
	for _, args := range [][]string{
		{"init", "-q", repo},
		{"-C", repo, "-c", "user.email=f@example.invalid", "-c", "user.name=f", "commit", "-q", "--allow-empty", "-m", "init"},
		{"-C", repo, "worktree", "add", "-q", "--detach", worktree},
	} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	caches := gocache.Paths{GoCache: filepath.Join(root, "cache", "metasystem-delegate-go-build"), StaticcheckCache: filepath.Join(root, "cache", "metasystem-delegate-staticcheck")}
	roundDir := t.TempDir()
	for _, workspace := range []string{worktree, repo} {
		fakeRecordBuildCachePath(runGit, agents, workspace, roundDir, caches)
		if got := readText(t, filepath.Join(roundDir, "build-cache.txt")); got != caches.GoCache+"\n" {
			t.Fatalf("%s: build cache = %q", workspace, got)
		}
	}
	gitdir := filepath.Join(repo, ".git", "worktrees", "job-1")
	if !exists(filepath.Join(gitdir, "metasystem-go-tmp")) || exists(filepath.Join(gitdir, "metasystem-build-cache")) {
		t.Fatal("the fake makes the worktree's GOTMPDIR and no per-chain cache")
	}
}

// TestFakeProbe covers the profiles, the fault hook, the envelope result,
// and the scaled handshake window.
func TestFakeProbe(t *testing.T) {
	t.Parallel()
	probe := func(f *fakeInstall, args ...string) int {
		return f.run(append([]string{"fake", "probe", "--root", f.root}, args...)...)
	}
	snapshot := func(t *testing.T, f *fakeInstall) map[string]any {
		t.Helper()
		path := strings.TrimSpace(f.stdout.String())
		if filepath.Dir(path) != filepath.Join(f.agents(), "capabilities") {
			t.Fatalf("snapshot path %q", path)
		}
		return readJSON(t, path)
	}
	t.Run("current default", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{})
		result := filepath.Join(f.root, "envelope.json")
		f.env["METASYSTEM_FAKE_ENVELOPE_PROBE_RESULT"] = result
		if code := probe(f); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
		value := snapshot(t, f)
		if value["profile"] != "current" || value["capabilities"].(map[string]any)["sessionEstablishedTimeoutSec"] != float64(16) {
			t.Fatalf("snapshot = %v", value)
		}
		if got := readText(t, result); got != `{"network":{"exitStatus":77,"observed":"denied"},"writeRoots":{"exitStatus":77,"observed":"denied"}}`+"\n" {
			t.Fatalf("envelope result = %q", got)
		}
	})
	t.Run("old aged", func(t *testing.T) {
		f := newFakeInstall(t, installOptions{})
		before := time.Now()
		if code := probe(f, "--profile", "old", "--age-days", "3"); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
		after := time.Now()
		value := snapshot(t, f)
		captured, _ := time.Parse(time.RFC3339, value["capturedAt"].(string))
		// Three days before the probe's own reading of the clock, which lies
		// between the two readings around it (capturedAt keeps seconds).
		earliest, latest := before.Add(-72*time.Hour).Truncate(time.Second), after.Add(-72*time.Hour)
		if value["profile"] != "old" || captured.Before(earliest) || captured.After(latest) {
			t.Fatalf("snapshot = %v, want capturedAt between %s and %s", value, earliest, latest)
		}
	})
	for _, tc := range []struct {
		name  string
		env   map[string]string
		want  float64
		fails string
	}{
		{name: "calibration floor", env: map[string]string{}, want: 16},
		{name: "milli 1500", env: map[string]string{"METASYSTEM_FIXTURE_CAP_SCALE_MILLI": "1500"}, want: 3},
		{name: "milli 3000", env: map[string]string{"METASYSTEM_FIXTURE_CAP_SCALE_MILLI": "3000"}, want: 6},
		{name: "capped at 60", env: map[string]string{"METASYSTEM_FIXTURE_CAP_SCALE_MILLI": "48000"}, want: 60},
		{name: "milli invalid", env: map[string]string{"METASYSTEM_FIXTURE_CAP_SCALE_MILLI": "0"}, fails: "the test time scale METASYSTEM_FIXTURE_CAP_SCALE_MILLI is not a whole number above zero"},
		{name: "scripted failure", env: map[string]string{"METASYSTEM_FAKE_PROBE_FAIL": "1"}, fails: "scripted probe failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeInstall(t, installOptions{})
			for name, value := range tc.env {
				f.env[name] = value
			}
			code := probe(f, "--profile", "unverified-network")
			if tc.fails != "" {
				if code != 1 || !strings.Contains(f.stderr.String(), tc.fails) {
					t.Fatalf("exit %d stderr %q, want 1 with %q", code, f.stderr.String(), tc.fails)
				}
				return
			}
			if code != 0 {
				t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
			}
			value := snapshot(t, f)
			if got := value["capabilities"].(map[string]any)["sessionEstablishedTimeoutSec"]; got != tc.want {
				t.Fatalf("handshake = %v, want %v", got, tc.want)
			}
			if value["envelopeEnforcement"].(map[string]any)["network"] != "notEnforced" {
				t.Fatalf("snapshot = %v", value)
			}
		})
	}
	for _, args := range [][]string{{"--profile", "new"}, {"--age-days", "-1"}, {"--age-days"}, {"--bogus", "1"}} {
		t.Run("usage "+strings.Join(args, " "), func(t *testing.T) {
			f := newFakeInstall(t, installOptions{})
			if code := probe(f, args...); code != 2 {
				t.Fatalf("exit %d, want 2", code)
			}
		})
	}
}

// TestFakeSmallVerbs covers identity, config-identity, contract,
// output-stream, local-config-paths, signature, wait-delivery, and cancel.
func TestFakeSmallVerbs(t *testing.T) {
	t.Parallel()
	f := newFakeInstall(t, installOptions{})
	run := func(args ...string) (int, string) {
		f.stdout.mu.Lock()
		f.stdout.buf.Reset()
		f.stdout.mu.Unlock()
		code := f.run(append([]string{"fake", args[0], "--root", f.root}, args[1:]...)...)
		return code, f.stdout.String()
	}
	if code, out := run("identity"); code != 0 || out != "fake-1 fake-config-v1\n" {
		t.Fatalf("identity = %d %q", code, out)
	}
	if code, out := run("config-identity"); code != 0 || out != fakeConfigIdentity+"\n" {
		t.Fatalf("config-identity = %d %q", code, out)
	}
	if code, out := run("output-stream", "--round-dir", "/x/rounds/1/"); code != 0 || out != "/x/rounds/1/events.jsonl\n" {
		t.Fatalf("output-stream = %d %q", code, out)
	}
	if code, _ := run("output-stream", "--round-dir", "relative"); code != 2 {
		t.Fatalf("relative output-stream = %d", code)
	}
	if code, out := run("local-config-paths"); code != 0 || out != "" {
		t.Fatalf("local-config-paths = %d %q", code, out)
	}
	if code, out := run("signature"); code != 0 || !strings.HasPrefix(out, "match (^|[[:space:]/-])metasystem-fake-agent([[:space:]]|$)\n") {
		t.Fatalf("signature = %d %q", code, out)
	}
	if code, out := run("wait-delivery", "--wait-id", "w", "--nonce", "n", "--deadline", "2026-09-27T10:00:00Z", "--session", "s"); code != 0 || out != "blocking\n" {
		t.Fatalf("wait-delivery = %d %q", code, out)
	}
	code, out := run("contract")
	if code != 0 {
		t.Fatalf("contract exit %d", code)
	}
	var contract map[string]any
	if err := json.Unmarshal([]byte(out), &contract); err != nil {
		t.Fatalf("contract %q: %v", out, err)
	}
	if contract["profile"] != "current" || contract["capabilities"].(map[string]any)["sessionEstablishedTimeoutSec"] != float64(1) {
		t.Fatalf("contract = %v", contract)
	}
	if exists(filepath.Join(f.agents(), "capabilities")) {
		t.Fatal("contract wrote into the installation's capabilities")
	}
	f.dispatcher.Statuses["__cancel-owned"] = 7
	if code, _ := run("cancel", "--job", f.job); code != 7 {
		t.Fatalf("cancel = %d, want the callback's status", code)
	}
	if calls := f.dispatcher.calls(t); !reflect.DeepEqual(calls, [][]string{{"__cancel-owned", "--job", f.job}}) {
		t.Fatalf("calls = %q", calls)
	}
}

// TestFakeSelftest drives the full sequence against the engine stand-in.
func TestFakeSelftest(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) *fakeInstall {
		f := newFakeInstall(t, installOptions{})
		f.env["TMPDIR"] = t.TempDir()
		f.clock = &stepClock{now: time.Date(2026, 9, 27, 10, 11, 12, 0, time.UTC)}
		return f
	}
	t.Run("pass", func(t *testing.T) {
		f := setup(t)
		f.env["METASYSTEM_DELEGATE_SELFTEST_INTERNAL"] = ""
		if code := f.run("fake", "selftest", "--root", f.root); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
		if f.stdout.String() != "fake adapter selftest passed: full protocol sequence and denied-envelope mechanism probes\n" {
			t.Fatalf("stdout = %q", f.stdout.String())
		}
		id := "fake-selftest-20260927t101112z-" + strconv.Itoa(os.Getpid())
		dirs, _ := filepath.Glob(filepath.Join(f.env["TMPDIR"], "metasystem-fake-selftest.*"))
		if len(dirs) != 1 {
			t.Fatalf("selftest dirs = %q", dirs)
		}
		dir := dirs[0]
		lines := strings.Split(strings.TrimSpace(readText(t, f.delegates)), "\n")
		want := []string{
			"--adapter-selftest fake --brief " + dir + "/brief.md --workspace " + f.root + " --op " + id + " --wait|root=" + f.root + "|internal=1",
			"--follow-up " + id + " --brief " + dir + "/follow.md --wait|root=" + f.root + "|internal=",
			"--adapter-selftest fake --brief " + dir + "/cancel.md --workspace " + f.root + " --op " + id + "-cancel|root=" + f.root + "|internal=1",
			"--cancel " + id + "-cancel|root=" + f.root + "|internal=",
		}
		if !reflect.DeepEqual(lines, want) {
			t.Fatalf("delegate calls =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
		}
		template, err := protocol.Template("brief.md")
		if err != nil {
			t.Fatal(err)
		}
		designed := strings.Replace(string(template), "Working Mode: <working mode, lower case, e.g. implement>", "Working Mode: design", 1)
		if !strings.HasPrefix(string(template), "Working Mode: <working mode, lower case, e.g. implement>\n") || strings.Count(designed, "Working Mode:") != 1 {
			t.Fatalf("the engine's brief template changed its working-mode header: %q", template[:min(len(template), 80)])
		}
		if got := readText(t, filepath.Join(dir, "brief.md")); got != designed {
			t.Fatalf("brief = %q", got)
		}
		if got := readText(t, filepath.Join(dir, "cancel.md")); got != designed+"\nFAKE:timeout\n" {
			t.Fatalf("cancel brief = %q", got)
		}
		if followUp, err := protocol.Template("follow-up.md"); err != nil || readText(t, filepath.Join(dir, "follow.md")) != string(followUp) {
			got := readText(t, filepath.Join(dir, "follow.md"))
			t.Fatalf("follow brief = %q", got)
		}
		record := readJSON(t, filepath.Join(f.agents(), "selftests", id+".json"))
		if record["job"] != id || record["runtime"] != "fake" {
			t.Fatalf("selftest record = %v", record)
		}
		if snapshots, _ := filepath.Glob(filepath.Join(f.agents(), "capabilities", "fake-*.json")); len(snapshots) != 1 {
			t.Fatalf("the selftest's probe wrote %q", snapshots)
		}
	})
	// Without a TMPDIR the fake allocates in the process's scratch root,
	// never in a hard-coded /tmp outside every owner (disk-lifetimes R1).
	t.Run("without TMPDIR it allocates in process scratch", func(t *testing.T) {
		f := setup(t)
		f.env["TMPDIR"] = ""
		f.env["METASYSTEM_DELEGATE_SELFTEST_INTERNAL"] = ""
		if code := f.run("fake", "selftest", "--root", f.root); code != 0 {
			t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
		}
		scratch, err := diskstore.ProcessScratch()
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(strings.SplitN(readText(t, f.delegates), "\n", 2)[0])
		if len(fields) < 4 || fields[2] != "--brief" {
			t.Fatalf("first delegate call = %q", fields)
		}
		selftestDir := filepath.Dir(fields[3])
		if filepath.Dir(selftestDir) != scratch || !strings.HasPrefix(filepath.Base(selftestDir), "metasystem-fake-selftest.") {
			t.Fatalf("selftest dir %s, want a metasystem-fake-selftest. directory in process scratch %s", selftestDir, scratch)
		}
	})
	t.Run("a failing delegate ends it with its status", func(t *testing.T) {
		f := setup(t)
		f.env["FAKE_DELEGATE_STATUS"] = "5"
		if code := f.run("fake", "selftest", "--root", f.root); code != 5 {
			t.Fatalf("exit %d, want 5", code)
		}
		if lines := strings.Split(strings.TrimSpace(readText(t, f.delegates)), "\n"); len(lines) != 1 {
			t.Fatalf("delegate calls after the failure: %q", lines)
		}
		if exists(filepath.Join(f.agents(), "selftests")) {
			t.Fatal("a failed selftest recorded a pass")
		}
	})
}

func TestFakeHostMarkerParser(t *testing.T) {
	t.Parallel()
	got := fakeHostBehaviorsIn([]byte("a FAKEHOST:x-y b\nFAKEHOST:\nFAKEHOST:b FAKEHOST:\nFAKEHOST:a\n"))
	if want := []string{"a", "b", "x-y"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("behaviors = %q, want %q", got, want)
	}
}
