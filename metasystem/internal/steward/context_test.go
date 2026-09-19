package steward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
	"golang.org/x/sys/unix"
)

func TestRoleContextRendersBoundCeilingAndUnknowns(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		tokens int64
		status HealthStatus
		reason string
	}{
		{104000, HealthAlive, "104 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250"},
		{105000, HealthAlive, "105 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250"},
		{105001, HealthAlive, "105 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the trigger"},
		{120000, HealthAlive, "120 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the trigger"},
		{200000, HealthAlive, "200 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the trigger"},
		{200001, HealthDead, "200 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the proof maximum"},
		{210000, HealthDead, "210 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the proof maximum"},
	} {
		for _, diagnostic := range []bool{true, false} {
			name := fmt.Sprintf("tokens-%d", test.tokens)
			if !diagnostic {
				name += "-live"
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				options := ContextOptions{Runtime: "claude", Session: "session", Toplevel: root}
				if diagnostic {
					options.Transcript = writeContextClaudeTranscript(t, root, test.tokens, false)
				} else {
					// A live read is the one a seat is judged on, and only it
					// carries the handoff instruction. Without this branch a
					// regression could tell an over-ceiling coordinator to run
					// context status, which changes nothing.
					home := t.TempDir()
					options.Home = home
					writeDiscoveredContextTranscript(t, home, root, "claude", "session", test.tokens, false)
				}
				role, reading, err := ContextBudgetLine(root, root, now, options)
				if err != nil || role.Status != test.status || !strings.Contains(role.Reason, test.reason) ||
					reading.Latest == nil || reading.Latest.PromptTokens != test.tokens {
					t.Fatalf("tokens %d diagnostic=%t = role %+v reading %+v err %v", test.tokens, diagnostic, role, reading, err)
				}
				handoff := "metasystem context handoff --root " + root + " --note <configured-note-path> --no-delegates"
				switch {
				case test.status == HealthDead && diagnostic:
					if !role.NoAutomaticRemedy || !strings.Contains(role.Remedy, "context status") || strings.Contains(role.Remedy, "transcript") {
						t.Fatalf("diagnostic ceiling breach has a lawful automatic remedy: %+v", role)
					}
				case test.status == HealthDead:
					if !role.NoAutomaticRemedy || role.Remedy != handoff {
						t.Fatalf("live ceiling breach must name the handoff: %+v", role)
					}
				case test.tokens > config.DefaultContextCeilingTokens-config.DefaultContextHandoffMarginTokens && !diagnostic:
					if !strings.Contains(role.Reason, "; over the trigger: run "+handoff) {
						t.Fatalf("live over-trigger reading must name the handoff: %+v", role)
					}
				}
			})
		}
	}

	t.Run("no call yet", func(t *testing.T) {
		root := t.TempDir()
		transcript := filepath.Join(root, "empty.jsonl")
		if err := os.WriteFile(transcript, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{Runtime: "claude", Session: "session", Transcript: transcript, Toplevel: root})
		if err != nil || role.Status != HealthAlive || role.Reason != "diagnostic transcript override; unknown (no call recorded yet)" {
			t.Fatalf("empty transcript = %+v err=%v", role, err)
		}
	})

	t.Run("missing source", func(t *testing.T) {
		root := t.TempDir()
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{Runtime: "claude", Session: "missing", Home: root, Toplevel: root})
		if err != nil || role.Status != HealthUnknown || !strings.HasPrefix(role.Reason, "unknown (no transcript at ") || role.Remedy == "" {
			t.Fatalf("missing source = %+v err=%v", role, err)
		}
	})

	t.Run("unreadable Codex discovery", func(t *testing.T) {
		root := t.TempDir()
		sessions := filepath.Join(root, ".codex", "sessions")
		if err := os.MkdirAll(filepath.Dir(sessions), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sessions, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{Runtime: "codex", Session: "session", Home: root, Toplevel: root})
		if err != nil || role.Status != HealthUnknown || !strings.Contains(role.Reason, "cannot read rollout directory "+sessions) {
			t.Fatalf("unreadable rollout discovery = %+v err=%v", role, err)
		}
	})

	t.Run("legacy rollout", func(t *testing.T) {
		root := t.TempDir()
		transcript := filepath.Join(root, "legacy.jsonl")
		line := `{"type":"event_msg","payload":{"type":"token_count"}}` + "\n"
		if err := os.WriteFile(transcript, []byte(line), 0o600); err != nil {
			t.Fatal(err)
		}
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{Runtime: "codex", Session: "legacy", Transcript: transcript, Toplevel: root})
		if err != nil || role.Status != HealthAlive || role.Reason != "diagnostic transcript override; unknown (rollout carries no token_usage_record (codex CLI before 0.153))" {
			t.Fatalf("legacy rollout = %+v err=%v", role, err)
		}
	})

	t.Run("sidechain only", func(t *testing.T) {
		root := t.TempDir()
		transcript := writeContextClaudeTranscript(t, root, 120000, true)
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{Runtime: "claude", Session: "session", Transcript: transcript, Toplevel: root})
		if err != nil || role.Status != HealthAlive || role.Reason != "diagnostic transcript override; unknown (only sidechain records so far)" {
			t.Fatalf("sidechain transcript = %+v err=%v", role, err)
		}
	})

	for _, test := range []struct {
		runtime    string
		reason     string
		capability usagepkg.Capability
	}{
		{"devin", "unknown (per-invocation usage only)", usagepkg.PerInvocation},
		{"fake", "unknown (no per-call stream)", usagepkg.NoStream},
	} {
		t.Run(test.runtime, func(t *testing.T) {
			root := t.TempDir()
			role, _, err := ContextBudgetLine(root, root, now, ContextOptions{Runtime: test.runtime, Session: "session", Toplevel: root})
			if err != nil || role.Status != HealthAlive || role.Reason != test.reason {
				t.Fatalf("%s capability = %+v err=%v", test.runtime, role, err)
			}
		})
		for _, state := range []identity.Liveness{identity.Dead, identity.Unknown} {
			t.Run(test.runtime+" inferred "+state.String(), func(t *testing.T) {
				root := t.TempDir()
				writeContextHolder(t, root, test.runtime, "stale")
				probe := healthProbe{123: {state: state}}
				role, reading, err := contextBudgetLineWithProber(root, root, now, ContextOptions{}, probe)
				if err != nil || role.Status != HealthAlive || role.Reason != test.reason || reading.Capability != test.capability {
					t.Fatalf("%s inferred %s capability = role %+v reading %+v err=%v", test.runtime, state, role, reading, err)
				}
				if _, statErr := os.Stat(filepath.Join(root, "artifacts", "agents", "context")); !os.IsNotExist(statErr) {
					t.Fatalf("%s inferred %s capability wrote live evidence: %v", test.runtime, state, statErr)
				}
			})
		}
	}

	t.Run("no holder", func(t *testing.T) {
		root := t.TempDir()
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{})
		if err != nil || role.Status != HealthAlive || !strings.Contains(role.Reason, "no announced holder") || !strings.Contains(role.Reason, "worktree-lease.json") {
			t.Fatalf("missing holder = %+v err=%v", role, err)
		}
	})

	t.Run("empty holder", func(t *testing.T) {
		root := t.TempDir()
		writeStewardRecord(t, filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json"), map[string]any{"holderMainId": ""})
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{})
		if err != nil || role.Status != HealthAlive || !strings.Contains(role.Reason, "no announced holder") || !strings.Contains(role.Reason, "no holderMainId") {
			t.Fatalf("empty holder = %+v err=%v", role, err)
		}
	})

	t.Run("holder without announcement", func(t *testing.T) {
		root := t.TempDir()
		writeStewardRecord(t, filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json"), map[string]any{"holderMainId": "main-absent"})
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{})
		if err != nil || role.Status != HealthAlive || !strings.Contains(role.Reason, "no announced holder") || !strings.Contains(role.Reason, "main-absent") {
			t.Fatalf("holder without announcement = %+v err=%v", role, err)
		}
	})

	t.Run("session id is the legacy main id", func(t *testing.T) {
		root := t.TempDir()
		directory := filepath.Join(root, "artifacts", "agents", "mains")
		pid, started := contextTestProcess(t)
		writeStewardRecord(t, filepath.Join(directory, "worktree-lease.json"), map[string]any{"holderMainId": "legacy-session"})
		writeStewardRecord(t, filepath.Join(directory, "legacy-session-123.json"), map[string]any{
			"sessionId": "legacy-session", "runtime": "devin", "pid": pid, "pidStartedAt": started,
		})
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{})
		if err != nil || role.Status != HealthAlive || role.Reason != "unknown (per-invocation usage only)" {
			t.Fatalf("legacy main id fallback = %+v err=%v", role, err)
		}
	})

	t.Run("malformed lease", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("null"), 0o600); err != nil {
			t.Fatal(err)
		}
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{})
		if err == nil || role.Status != HealthUnknown || !strings.Contains(role.Reason, path) || role.Remedy == "" {
			t.Fatalf("malformed lease = %+v err=%v", role, err)
		}
	})

	t.Run("unregistered holder", func(t *testing.T) {
		root := t.TempDir()
		writeLiveContextHolder(t, root, "other-runtime", "session")
		role, _, err := ContextBudgetLine(root, root, now, ContextOptions{})
		if err != nil || role.Status != HealthAlive || role.Reason != "unknown (runtime other-runtime is not registered)" {
			t.Fatalf("unregistered holder = %+v err=%v", role, err)
		}
	})
}

func TestContextVerdictReadsConfiguredBounds(t *testing.T) {
	t.Setenv("METASYSTEM_CONTEXT_CEILING_TOKENS", "")
	_ = os.Unsetenv("METASYSTEM_CONTEXT_CEILING_TOKENS")
	t.Setenv("METASYSTEM_CONTEXT_HANDOFF_MARGIN_TOKENS", "")
	_ = os.Unsetenv("METASYSTEM_CONTEXT_HANDOFF_MARGIN_TOKENS")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("context.ceiling.tokens=200000\ncontext.handoff.margin.tokens=120000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	transcript := writeContextClaudeTranscript(t, root, 90000, false)
	role, _, err := ContextBudgetLine(root, root, time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC), ContextOptions{Runtime: "claude", Session: "session", Transcript: transcript, Toplevel: root})
	want := "diagnostic transcript override; 90 thousand tokens this call, trigger 80, proof line 150, proof maximum 200, ceiling 200; over the trigger"
	if err != nil || role.Status != HealthAlive || role.Reason != want {
		t.Fatalf("configured verdict = %+v, want reason %q", role, want)
	}

	invalidRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalidRoot, "metasystem.conf"), []byte("context.ceiling.tokens=invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	role, reading, err := contextBudgetLineWithProber(invalidRoot, invalidRoot, time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC), ContextOptions{}, healthProbe{})
	if err == nil || role.Status != HealthUnknown || role.Reason != err.Error() || !strings.Contains(role.Reason, "CONTEXT_CONFIG_INVALID key=context.ceiling.tokens") ||
		role.Remedy != "metasystem config validate --conf "+filepath.Join(invalidRoot, "metasystem.conf") || !reflect.DeepEqual(reading, usagepkg.Reading{}) {
		t.Fatalf("invalid budget = role %+v reading %+v err %v", role, reading, err)
	}
}

func TestContextVerdictOverTriggerIsAliveWithTheRemedy(t *testing.T) {
	role := contextVerdict(usagepkg.Reading{Latest: &usagepkg.CallSample{PromptTokens: 105001}}, config.Budget{Ceiling: 250000, Margin: 145000, Trigger: 105000}, "/root", false)
	if role.Status != HealthAlive || !strings.HasSuffix(role.Reason, "; over the trigger: run metasystem context handoff --root /root --note <configured-note-path> --no-delegates") {
		t.Fatalf("over-trigger verdict = %+v", role)
	}
}

func TestContextVerdictOverProofMaximumIsDead(t *testing.T) {
	role := contextVerdict(usagepkg.Reading{Latest: &usagepkg.CallSample{PromptTokens: ProofMaxTokens + 1}}, config.Budget{Ceiling: 250000, Margin: 145000, Trigger: 105000}, "/root", false)
	if role.Status != HealthDead || !role.NoAutomaticRemedy || role.Remedy != "metasystem context handoff --root /root --note <configured-note-path> --no-delegates" || !strings.HasSuffix(role.Reason, "; over the proof maximum") {
		t.Fatalf("over-proof-maximum verdict = %+v", role)
	}
}

func TestHealthLineCarriesContextBudget(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	writeContextHolder(t, root, "codex", "session")
	rollout := filepath.Join(home, ".codex", "sessions", "2026", "09", "13", "rollout-fixture-session.jsonl")
	if err := os.MkdirAll(filepath.Dir(rollout), 0o755); err != nil {
		t.Fatal(err)
	}
	row := `{"type":"token_usage_record","timestamp":"2026-09-13T11:00:00Z","ordinal":1,"payload":{"response_id":"response","usage":{"input_tokens":200001,"cached_input_tokens":0,"cache_write_input_tokens":0}}}` + "\n"
	if err := os.WriteFile(rollout, []byte(row), 0o600); err != nil {
		t.Fatal(err)
	}

	probe := healthProbe{123: {
		exact: identity.Exact{Pid: 123, StartedAt: time.Unix(456, 0)},
		state: identity.Alive,
	}}
	roles, _ := evaluateHealthRoles(root, root, now, probe, true)
	contextIndex := -1
	for index, candidate := range roles {
		if candidate.Role == RoleContext {
			contextIndex = index
			if index == 0 || roles[index-1].Role != RoleStopHookDuration || candidate.DurationMillis < 1 || candidate.Status != HealthDead {
				t.Fatalf("timed context check has wrong integration result: index=%d role=%+v", index, candidate)
			}
			break
		}
	}
	if contextIndex < 0 {
		t.Fatal("health evaluation omitted context-budget")
	}

	verdict := applyHealthObservation(root, HealthObservationState{}, roles, now)
	contextRole := verdict.Roles[contextIndex]
	if contextRole.Role != RoleContext {
		t.Fatalf("context role moved from health order: index=%d role=%+v", contextIndex, contextRole)
	}
	if !strings.Contains(verdict.Line(), "context-budget=dead (200 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the proof maximum") ||
		!verdict.ShouldAlert || contextRole.FailureEscalation != NoLawfulRemedy {
		t.Fatalf("health line did not carry immediate context escalation: %+v line=%s", verdict, verdict.Line())
	}
}

func TestContextBudgetDoesNotAttributeUsageToAStaleHolder(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	for _, state := range []identity.Liveness{identity.Dead, identity.Unknown} {
		t.Run(state.String(), func(t *testing.T) {
			root := t.TempDir()
			transcript := writeContextClaudeTranscript(t, root, 210000, false)
			writeContextHolder(t, root, "claude", "stale")
			probe := healthProbe{123: {state: state}}

			role, reading, err := contextBudgetLineWithProber(root, root, now, ContextOptions{
				Transcript: transcript, Toplevel: root,
			}, probe)
			if err != nil || role.Status != HealthUnknown || role.NoAutomaticRemedy || reading.Latest != nil ||
				!strings.Contains(role.Reason, "holder main-context") || !strings.Contains(role.Reason, state.String()) {
				t.Fatalf("%s holder = role %+v reading %+v err %v", state, role, reading, err)
			}
			if _, statErr := os.Stat(usagepkg.CursorPath(root, "claude", "stale")); !os.IsNotExist(statErr) {
				t.Fatalf("%s holder still produced a context cursor: %v", state, statErr)
			}
		})
	}
}

func TestContextHolderResolutionSkipsForeignMalformedAnnouncements(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "artifacts", "agents", "mains")
	writeStewardRecord(t, filepath.Join(directory, "worktree-lease.json"), map[string]any{"holderMainId": "main-context"})
	if err := os.WriteFile(filepath.Join(directory, "a-foreign-broken.json"), []byte("{not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeStewardRecord(t, filepath.Join(directory, "b-foreign-incomplete.json"), map[string]any{"mainId": "another-main"})
	writeStewardRecord(t, filepath.Join(directory, "c-holder-incomplete.json"), map[string]any{"mainId": "main-context"})
	writeStewardRecord(t, filepath.Join(directory, "d-holder-valid.json"), map[string]any{
		"sessionId": "holder-session", "mainId": "main-context", "runtime": "claude",
		"pid": int64(123), "pidStartedAt": int64(456),
	})
	transcript := writeContextClaudeTranscript(t, root, 120000, false)
	probe := healthProbe{123: {
		exact: identity.Exact{Pid: 123, StartedAt: time.Unix(456, 0)},
		state: identity.Alive,
	}}

	role, reading, err := contextBudgetLineWithProber(root, root, time.Now().UTC(), ContextOptions{
		Transcript: transcript, Toplevel: root,
	}, probe)
	if err != nil || role.Status != HealthAlive || reading.Latest == nil || reading.Latest.PromptTokens != 120000 {
		t.Fatalf("holder resolution = role %+v reading %+v err %v", role, reading, err)
	}
}

func TestContextBudgetReturnsBusyUnknownWithoutWaiting(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeContextHolder(t, root, "codex", "busy")
	rollout := filepath.Join(home, ".codex", "sessions", "2026", "09", "13", "rollout-fixture-busy.jsonl")
	if err := os.MkdirAll(filepath.Dir(rollout), 0o700); err != nil {
		t.Fatal(err)
	}
	row := `{"type":"token_usage_record","timestamp":"2026-09-13T11:00:00Z","ordinal":1,"payload":{"response_id":"response","usage":{"input_tokens":120000}}}` + "\n"
	if err := os.WriteFile(rollout, []byte(row), 0o600); err != nil {
		t.Fatal(err)
	}
	lockPath := usagepkg.CursorPath(root, "codex", "busy") + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	locked := true
	release := func() {
		if locked {
			_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
			locked = false
		}
	}
	defer release()
	probe := healthProbe{123: {
		exact: identity.Exact{Pid: 123, StartedAt: time.Unix(456, 0)},
		state: identity.Alive,
	}}
	done := make(chan RoleVerdict, 1)
	go func() {
		done <- checkContextBudget(root, root, time.Now().UTC(), probe)
	}()
	role := <-done
	if role.Status != HealthUnknown || !strings.Contains(role.Reason, "cursor is busy") || role.Remedy == "" {
		t.Fatalf("contended cursor = %+v", role)
	}
}

func TestContextBudgetReturnsRegistryBusyUnknownWithoutWaiting(t *testing.T) {
	root := t.TempDir()
	writeContextHolder(t, root, "claude", "busy-registry")
	lockPath := filepath.Join(root, "artifacts", "agents", "context", "sessions.jsonl.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	locked := true
	release := func() {
		if locked {
			_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
			locked = false
		}
	}
	defer release()
	probe := healthProbe{123: {
		exact: identity.Exact{Pid: 123, StartedAt: time.Unix(456, 0)},
		state: identity.Alive,
	}}
	done := make(chan RoleVerdict, 1)
	go func() {
		done <- checkContextBudget(root, root, time.Now().UTC(), probe)
	}()
	role := <-done
	if role.Status != HealthUnknown || !strings.Contains(role.Reason, "session registry is busy") ||
		!strings.Contains(role.Reason, lockPath) || role.Remedy == "" {
		t.Fatalf("contended session registry = %+v", role)
	}
}

func TestRoleContextNamesTheNewestSpill(t *testing.T) {
	root := t.TempDir()
	firstAt := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	home := t.TempDir()
	writeDiscoveredContextTranscript(t, home, root, "claude", "session", 120000, false)
	opts := ContextOptions{Runtime: "claude", Session: "session", Home: home, Toplevel: root}
	if _, _, err := ContextBudgetLine(root, root, firstAt, opts); err != nil {
		t.Fatal(err)
	}
	spill := filepath.Join(root, filepath.FromSlash(output.Dir), "goal-list.txt")
	if err := os.MkdirAll(filepath.Dir(spill), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spill, []byte("seven!"), 0o600); err != nil {
		t.Fatal(err)
	}
	spillAt := firstAt.Add(time.Minute)
	if err := os.Chtimes(spill, spillAt, spillAt); err != nil {
		t.Fatal(err)
	}
	role, reading, err := ContextBudgetLine(root, root, firstAt.Add(2*time.Minute), opts)
	if err != nil || !reading.PreviousReadAt.Equal(firstAt) || !strings.Contains(role.Reason, "; newest spill: goal-list.txt (6 bytes)") {
		t.Fatalf("newest spill role = %+v reading=%+v err=%v", role, reading, err)
	}
}

func TestContextTranscriptOverrideUsesPrivateEvidence(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	probe := healthProbe{123: {
		exact: identity.Exact{Pid: 123, StartedAt: time.Unix(456, 0)},
		state: identity.Alive,
	}}
	for _, runtimeName := range []string{"claude", "codex"} {
		for _, identityCase := range []string{"inferred", "explicit-holder", "another-explicit", "real-path"} {
			t.Run(runtimeName+"/"+identityCase, func(t *testing.T) {
				root := t.TempDir()
				home := t.TempDir()
				const holderSession = "holder"
				realPath := writeDiscoveredContextTranscript(t, home, root, runtimeName, holderSession, 111000, false)
				if err := usagepkg.RegisterSession(root, runtimeName, holderSession, 123, 456); err != nil {
					t.Fatal(err)
				}
				if _, err := usagepkg.LatestCall(root, runtimeName, holderSession, usagepkg.ReadOptions{
					Capability: usagepkg.PerCall, Home: home, Toplevel: root, Installation: root, Now: now,
				}); err != nil {
					t.Fatal(err)
				}
				writeContextHolder(t, root, runtimeName, holderSession)
				override := writeContextRuntimeTranscript(t, t.TempDir(), runtimeName, "override", 130000, false)
				opts := ContextOptions{Transcript: override, Home: home, Toplevel: root}
				expectedTokens := int64(130000)
				expectedSession := holderSession
				switch identityCase {
				case "explicit-holder":
					opts.Runtime, opts.Session = runtimeName, holderSession
				case "another-explicit":
					opts.Runtime, opts.Session = runtimeName, "another"
					expectedSession = "another"
				case "real-path":
					opts.Runtime, opts.Session, opts.Transcript = runtimeName, holderSession, realPath
					expectedTokens = 111000
				}
				before := snapshotContextEvidence(t, root)
				role, reading, err := contextBudgetLineWithProber(root, root, now.Add(time.Minute), opts, probe)
				if err != nil || role.Status != HealthAlive || !strings.HasPrefix(role.Reason, "diagnostic transcript override; ") ||
					reading.Latest == nil || reading.Latest.Runtime != runtimeName || reading.Latest.Session != expectedSession ||
					reading.Latest.PromptTokens != expectedTokens || !reading.PreviousReadAt.IsZero() || reading.NewSamples != 1 {
					t.Fatalf("diagnostic result = role %+v reading %+v err %v", role, reading, err)
				}
				assertContextEvidence(t, root, before)
			})
		}

		t.Run(runtimeName+"/fresh-inferred", func(t *testing.T) {
			root := t.TempDir()
			writeContextHolder(t, root, runtimeName, "fresh")
			override := writeContextRuntimeTranscript(t, t.TempDir(), runtimeName, "fresh", 120000, false)
			role, reading, err := contextBudgetLineWithProber(root, root, now, ContextOptions{Transcript: override}, probe)
			if err != nil || role.Status != HealthAlive || reading.Latest == nil || reading.Latest.PromptTokens != 120000 {
				t.Fatalf("fresh inferred diagnostic = role %+v reading %+v err %v", role, reading, err)
			}
			contextDirectory := filepath.Join(root, "artifacts", "agents", "context")
			if _, statErr := os.Stat(contextDirectory); !os.IsNotExist(statErr) {
				t.Fatalf("fresh inferred diagnostic created live evidence at %s: %v", contextDirectory, statErr)
			}
		})
	}
}

func TestContextTranscriptOverridePreservesTheNextHealthRead(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	const session = "holder"
	realPath := writeDiscoveredContextTranscript(t, home, root, "claude", session, 210001, false)
	writeContextHolder(t, root, "claude", session)
	probe := healthProbe{123: {
		exact: identity.Exact{Pid: 123, StartedAt: time.Unix(456, 0)},
		state: identity.Alive,
	}}
	liveOpts := ContextOptions{Home: home, Toplevel: root}
	role, reading, err := contextBudgetLineWithProber(root, root, now, liveOpts, probe)
	if err != nil || role.Status != HealthDead || reading.Latest == nil || reading.Latest.PromptTokens != 210001 || reading.NewSamples != 1 {
		t.Fatalf("seed health read = role %+v reading %+v err %v", role, reading, err)
	}
	original := snapshotContextEvidence(t, root)

	empty := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	realBytes, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(t.TempDir(), "copy.jsonl")
	if err := os.WriteFile(copyPath, realBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	distinct := writeContextRuntimeTranscript(t, t.TempDir(), "claude", "distinct", 180000, true)
	for _, diagnostic := range []struct {
		path       string
		newSamples int
		newMarkers int
	}{{empty, 0, 0}, {copyPath, 1, 0}, {distinct, 1, 1}} {
		role, diagnosticReading, err := contextBudgetLineWithProber(root, root, now.Add(time.Minute), ContextOptions{
			Transcript: diagnostic.path, Home: home, Toplevel: root,
		}, probe)
		if err != nil || !strings.HasPrefix(role.Reason, "diagnostic transcript override; ") ||
			diagnosticReading.NewSamples != diagnostic.newSamples || diagnosticReading.NewMarkers != diagnostic.newMarkers {
			t.Fatalf("override %s = role %+v reading %+v err %v", diagnostic.path, role, diagnosticReading, err)
		}
		assertContextEvidence(t, root, original)

		role, reading, err = contextBudgetLineWithProber(root, root, now.Add(2*time.Minute), liveOpts, probe)
		if err != nil || role.Status != HealthDead || reading.Latest == nil || reading.Latest.PromptTokens != 210001 ||
			reading.NewSamples != 0 || reading.NewMarkers != 0 {
			t.Fatalf("health after override %s = role %+v reading %+v err %v", diagnostic.path, role, reading, err)
		}
		assertContextEvidence(t, root, original)
	}

	file, err := os.OpenFile(realPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString(contextClaudeLine("real-next", 210001))
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		t.Fatal(err)
	}
	role, reading, err = contextBudgetLineWithProber(root, root, now.Add(3*time.Minute), liveOpts, probe)
	if err != nil || role.Status != HealthDead || reading.NewSamples != 1 || reading.NewMarkers != 0 {
		t.Fatalf("appended health read = role %+v reading %+v err %v", role, reading, err)
	}
	samples, markers, err := usagepkg.Calls(root, "claude", session, time.Time{})
	if err != nil || len(samples) != 2 || len(markers) != 0 {
		t.Fatalf("committed live evidence after append = samples %d markers %d err %v", len(samples), len(markers), err)
	}
}

func TestContextTranscriptOverrideIgnoresLiveStoreFailures(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	probe := healthProbe{123: {
		exact: identity.Exact{Pid: 123, StartedAt: time.Unix(456, 0)},
		state: identity.Alive,
	}}
	for _, failure := range []string{"uncommitted-suffix", "corrupt-cursor", "short-log", "busy-paths"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			const session = "holder"
			writeContextHolder(t, root, "claude", session)
			if failure != "busy-paths" {
				liveTranscript := writeContextRuntimeTranscript(t, t.TempDir(), "claude", "live", 210001, false)
				if err := usagepkg.RegisterSession(root, "claude", session, 123, 456); err != nil {
					t.Fatal(err)
				}
				if _, err := usagepkg.LatestCall(root, "claude", session, usagepkg.ReadOptions{
					Capability: usagepkg.PerCall, Transcript: liveTranscript, Now: now,
				}); err != nil {
					t.Fatal(err)
				}
				cursorPath := usagepkg.CursorPath(root, "claude", session)
				samplesPath := usagepkg.SamplesPath(root, "claude", session)
				switch failure {
				case "uncommitted-suffix":
					file, err := os.OpenFile(samplesPath, os.O_APPEND|os.O_WRONLY, 0)
					if err != nil {
						t.Fatal(err)
					}
					_, writeErr := file.WriteString("{\"kind\":\"sample\",\"uncommitted\":true}\n")
					closeErr := file.Close()
					if err := errors.Join(writeErr, closeErr); err != nil {
						t.Fatal(err)
					}
				case "corrupt-cursor":
					if err := os.WriteFile(cursorPath, []byte("{not-json\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				case "short-log":
					contents, err := os.ReadFile(samplesPath)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(samplesPath, contents[:len(contents)-1], 0o600); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				cursorLock := usagepkg.CursorPath(root, "claude", session) + ".lock"
				registryLock := filepath.Join(root, "artifacts", "agents", "context", "sessions.jsonl.lock")
				if err := os.MkdirAll(cursorLock, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(registryLock, 0o700); err != nil {
					t.Fatal(err)
				}
			}

			override := writeContextRuntimeTranscript(t, t.TempDir(), "claude", "diagnostic", 120000, false)
			before := snapshotContextEvidence(t, root)
			role, reading, err := contextBudgetLineWithProber(root, root, now.Add(time.Minute), ContextOptions{Transcript: override}, probe)
			if err != nil || role.Status != HealthAlive || reading.Latest == nil || reading.Latest.PromptTokens != 120000 {
				t.Fatalf("diagnostic beside %s = role %+v reading %+v err %v", failure, role, reading, err)
			}
			assertContextEvidence(t, root, before)
		})
	}
}

func TestContextTranscriptOverrideDisposesPrivateCursor(t *testing.T) {
	originalMake := makeContextDiagnosticRoot
	originalRemove := removeContextDiagnosticRoot
	defer func() {
		makeContextDiagnosticRoot = originalMake
		removeContextDiagnosticRoot = originalRemove
	}()

	parent := t.TempDir()
	var roots []string
	var removed []string
	makeContextDiagnosticRoot = func(_ string, pattern string) (string, error) {
		root, err := os.MkdirTemp(parent, pattern)
		if err == nil {
			roots = append(roots, root)
		}
		return root, err
	}
	removeContextDiagnosticRoot = func(root string) error {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("private root was absent before cleanup: %s: %v", root, err)
		}
		removed = append(removed, root)
		return os.RemoveAll(root)
	}
	sample := writeContextRuntimeTranscript(t, t.TempDir(), "claude", "sample", 120000, false)
	empty := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	nonregular := t.TempDir()
	for index, source := range []string{sample, empty, nonregular} {
		reading, err := readContextTranscriptOverride("claude", fmt.Sprintf("case-%d", index), usagepkg.ReadOptions{
			Capability: usagepkg.PerCall, Transcript: source, Now: time.Now().UTC(),
		})
		if index < 2 && err != nil {
			t.Fatalf("private read %d failed: %v", index, err)
		}
		if index == 0 && (reading.Latest == nil || reading.Latest.PromptTokens != 120000) {
			t.Fatalf("private sample read = %+v", reading)
		}
		if index == 1 && reading.Latest != nil {
			t.Fatalf("private empty read = %+v", reading)
		}
		if index == 2 && (err == nil || !strings.Contains(err.Error(), "not a regular file")) {
			t.Fatalf("private nonregular read err = %v", err)
		}
	}
	if len(roots) != 3 || len(removed) != 3 || roots[0] == roots[1] || roots[0] == roots[2] || roots[1] == roots[2] {
		t.Fatalf("private roots were not unique and disposed: roots=%v removed=%v", roots, removed)
	}
	for _, root := range roots {
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatalf("private root remains after cleanup: %s: %v", root, err)
		}
	}

	allocationFailure := errors.New("diagnostic allocation failed")
	makeContextDiagnosticRoot = func(string, string) (string, error) { return "", allocationFailure }
	removeContextDiagnosticRoot = originalRemove
	liveRoot := t.TempDir()
	before := snapshotContextEvidence(t, liveRoot)
	role, _, err := ContextBudgetLine(liveRoot, liveRoot, time.Now().UTC(), ContextOptions{
		Runtime: "claude", Session: "allocation", Transcript: sample,
	})
	if !errors.Is(err, allocationFailure) || role.Status != HealthUnknown || !strings.HasPrefix(role.Reason, "diagnostic transcript override; ") {
		t.Fatalf("allocation failure = role %+v err %v", role, err)
	}
	assertContextEvidence(t, liveRoot, before)

	cleanupFailure := errors.New("diagnostic cleanup failed")
	var leaked []string
	makeContextDiagnosticRoot = func(_ string, pattern string) (string, error) {
		root, err := os.MkdirTemp(parent, pattern)
		if err == nil {
			leaked = append(leaked, root)
		}
		return root, err
	}
	removeContextDiagnosticRoot = func(string) error { return cleanupFailure }
	reading, err := readContextTranscriptOverride("claude", "cleanup", usagepkg.ReadOptions{
		Capability: usagepkg.PerCall, Transcript: sample, Now: time.Now().UTC(),
	})
	if !errors.Is(err, cleanupFailure) || reading.Latest == nil || reading.Latest.PromptTokens != 120000 {
		t.Fatalf("cleanup failure lost reading: reading %+v err %v", reading, err)
	}
	role, cleanupReading, err := ContextBudgetLine(liveRoot, liveRoot, time.Now().UTC(), ContextOptions{
		Runtime: "claude", Session: "cleanup-role", Transcript: sample,
	})
	if !errors.Is(err, cleanupFailure) || role.Status != HealthUnknown || !strings.HasPrefix(role.Reason, "diagnostic transcript override; ") ||
		cleanupReading.Latest == nil || cleanupReading.Latest.PromptTokens != 120000 {
		t.Fatalf("cleanup failure role = role %+v reading %+v err %v", role, cleanupReading, err)
	}
	_, err = readContextTranscriptOverride("claude", "read-and-cleanup", usagepkg.ReadOptions{
		Capability: usagepkg.PerCall, Transcript: nonregular, Now: time.Now().UTC(),
	})
	if !errors.Is(err, cleanupFailure) || !strings.Contains(err.Error(), "not a regular file") || !strings.Contains(err.Error(), cleanupFailure.Error()) {
		t.Fatalf("joined read and cleanup errors = %v", err)
	}
	for _, root := range leaked {
		if cleanupErr := os.RemoveAll(root); cleanupErr != nil {
			t.Fatal(cleanupErr)
		}
	}
}

type contextEvidenceEntry struct {
	Mode os.FileMode
	Data string
}

func snapshotContextEvidence(t *testing.T, root string) map[string]contextEvidenceEntry {
	t.Helper()
	directory := filepath.Join(root, "artifacts", "agents", "context")
	snapshot := map[string]contextEvidenceEntry{}
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		value := contextEvidenceEntry{Mode: info.Mode()}
		if info.Mode().IsRegular() {
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value.Data = string(contents)
		}
		snapshot[relative] = value
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return snapshot
	}
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func assertContextEvidence(t *testing.T, root string, want map[string]contextEvidenceEntry) {
	t.Helper()
	if got := snapshotContextEvidence(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("live context evidence changed:\nwant: %#v\n got: %#v", want, got)
	}
}

func writeDiscoveredContextTranscript(t *testing.T, home, toplevel, runtimeName, session string, tokens int64, marker bool) string {
	t.Helper()
	var path string
	switch runtimeName {
	case "claude":
		path = filepath.Join(home, ".claude", "projects", contextClaudeSlugForTest(toplevel), session+".jsonl")
	case "codex":
		path = filepath.Join(home, ".codex", "sessions", "2026", "09", "13", "rollout-fixture-"+session+".jsonl")
	default:
		t.Fatalf("unsupported context fixture runtime %s", runtimeName)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	writeContextRuntimeTranscriptAt(t, path, runtimeName, "real", tokens, marker)
	return path
}

func writeContextRuntimeTranscript(t *testing.T, directory, runtimeName, id string, tokens int64, marker bool) string {
	t.Helper()
	path := filepath.Join(directory, id+".jsonl")
	writeContextRuntimeTranscriptAt(t, path, runtimeName, id, tokens, marker)
	return path
}

func writeContextRuntimeTranscriptAt(t *testing.T, path, runtimeName, id string, tokens int64, marker bool) {
	t.Helper()
	var contents string
	switch runtimeName {
	case "claude":
		contents = contextClaudeLine(id, tokens)
		if marker {
			contents += `{"type":"system","subtype":"compact_boundary","timestamp":"2026-09-13T11:01:00Z","compactMetadata":{"trigger":"auto","preTokens":170000}}` + "\n"
		}
	case "codex":
		contents = fmt.Sprintf(`{"type":"token_usage_record","timestamp":"2026-09-13T11:00:00Z","ordinal":1,"payload":{"response_id":%q,"usage":{"input_tokens":%d,"cached_input_tokens":0,"cache_write_input_tokens":0}}}`+"\n", id, tokens)
		if marker {
			contents += `{"type":"compacted","timestamp":"2026-09-13T11:01:00Z","ordinal":2}` + "\n"
		}
	default:
		t.Fatalf("unsupported context fixture runtime %s", runtimeName)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func contextClaudeLine(id string, tokens int64) string {
	return fmt.Sprintf(`{"type":"assistant","requestId":%q,"timestamp":"2026-09-13T11:00:00Z","message":{"usage":{"input_tokens":%d,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`+"\n", id, tokens)
}

func contextClaudeSlugForTest(path string) string {
	bytes := []byte(path)
	for index, value := range bytes {
		if (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z') || (value >= '0' && value <= '9') {
			continue
		}
		bytes[index] = '-'
	}
	return string(bytes)
}

func writeContextClaudeTranscript(t *testing.T, root string, tokens int64, sidechain bool) string {
	t.Helper()
	path := filepath.Join(root, "claude.jsonl")
	line := fmt.Sprintf(`{"type":"assistant","requestId":"request-%d","timestamp":"2026-09-13T11:00:00Z","isSidechain":%t,"message":{"usage":{"input_tokens":%d,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`+"\n", tokens, sidechain, tokens)
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeContextHolder(t *testing.T, root, runtimeName, session string) {
	t.Helper()
	directory := filepath.Join(root, "artifacts", "agents", "mains")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeStewardRecord(t, filepath.Join(directory, "worktree-lease.json"), map[string]any{"holderMainId": "main-context"})
	writeStewardRecord(t, filepath.Join(directory, session+"-123.json"), map[string]any{
		"sessionId": session, "mainId": "main-context", "runtime": runtimeName,
		"pid": int64(123), "pidStartedAt": int64(456),
	})
}

func writeLiveContextHolder(t *testing.T, root, runtimeName, session string) {
	t.Helper()
	pid, started := contextTestProcess(t)
	directory := filepath.Join(root, "artifacts", "agents", "mains")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeStewardRecord(t, filepath.Join(directory, "worktree-lease.json"), map[string]any{"holderMainId": "main-context"})
	writeStewardRecord(t, filepath.Join(directory, session+"-live.json"), map[string]any{
		"sessionId": session, "mainId": "main-context", "runtime": runtimeName,
		"pid": pid, "pidStartedAt": started,
	})
}

func contextTestProcess(t *testing.T) (int64, int64) {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("cannot probe the context test process: state=%s err=%v", state, err)
	}
	return exact.Pid, exact.StartedAt.Unix()
}
