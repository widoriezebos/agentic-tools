package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/agentgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The landing agent's tool gate (goal landing-lane-runtime-redesign, design
// section 3, unit A-b) through the real runtime hook: the settings commands
// run through a shell in a nested checkout (toplevel != module) whose engine
// is this test binary serving `internal hook claude tool`, with the session's
// lineage in the environment exactly as the landing launch sets it.

const landingLineage = agentgate.LineageEnv + "=" + agentgate.Lineage

// landingBed is a direct engine bed on a lane branch.
func newLandingBed(t *testing.T) directBed {
	t.Helper()
	bed := newDirectEngineBed(t)
	bed.git(t, bed.repo, "checkout", "-q", "-b", "lane/b1")
	return bed
}

// landingRoutes are the two hook commands a landing session runs: the
// checkout's project settings, and the settings the landing launch passes.
func landingRoutes(t *testing.T, bed directBed) map[string]string {
	t.Helper()
	landing, err := agentgate.HookCommand(filepath.Join(bed.installation, "bin", "metasystem"), bed.repo)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{"project": renderedCommands(t, claudeShipped, "claude", "metasystem")["tool"], "landing": landing}
}

func landingCall(t *testing.T, cwd, tool string, input map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{"session_id": "landing-1", "cwd": cwd, "hook_event_name": "PreToolUse",
		"tool_name": tool, "tool_input": input})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// denial reads a hook's answer as a denial: the runtime's deny object on
// stdout, or the blocking exit status 2 with the reason on stderr. The
// reason is two lines: the situation, then the one command to run.
func denial(status int, stdout, stderr string) (string, bool) {
	reason := ""
	switch {
	case status == 2:
		reason = strings.TrimSuffix(stderr, "\n")
	case status == 0 && stdout != "":
		var answer struct {
			HookSpecificOutput struct {
				PermissionDecision       string `json:"permissionDecision"`
				PermissionDecisionReason string `json:"permissionDecisionReason"`
			} `json:"hookSpecificOutput"`
		}
		if json.Unmarshal([]byte(stdout), &answer) != nil || answer.HookSpecificOutput.PermissionDecision != "deny" {
			return "", false
		}
		reason = answer.HookSpecificOutput.PermissionDecisionReason
	default:
		return "", false
	}
	lines := strings.Split(reason, "\n")
	if len(lines) != 2 || lines[0] == "" || !strings.HasPrefix(lines[1], "run: ") || len(lines[1]) <= len("run: ") {
		return "", false
	}
	return reason, true
}

// sampleCall is one representative call for an allowlist entry, or false
// when the witness does not know the entry (a new entry must get a sample).
func sampleCall(t *testing.T, bed directBed, entry agentgate.Entry) (string, bool) {
	t.Helper()
	inside := filepath.Join(bed.installation, "README")
	command := ""
	switch entry.Kind {
	case "tool":
		switch entry.Name {
		case "Read", "Grep", "Glob", "LS", "TodoWrite", "Skill":
			return landingCall(t, bed.repo, entry.Name, map[string]any{"file_path": inside, "pattern": "x"}), true
		case "Edit", "MultiEdit", "Write":
			return landingCall(t, bed.repo, entry.Name, map[string]any{"file_path": inside, "old_string": "a", "new_string": "b", "content": "c"}), true
		case "NotebookEdit":
			return landingCall(t, bed.repo, entry.Name, map[string]any{"notebook_path": filepath.Join(bed.installation, "n.ipynb"), "new_source": "x"}), true
		case "Bash":
			command = "git status"
		default:
			return "", false
		}
	case "verb":
		command = "metasystem " + entry.Name
	case "git":
		command = "git " + entry.Name
		if entry.Name == "branch" {
			command += " --list"
		}
	case "git-lane":
		command = map[string]string{"fetch": "git fetch origin", "checkout": "git checkout -b lane/b2", "add": "git add metasystem/README",
			"cherry-pick": "git cherry-pick --abort", "rebase": "git rebase --abort", "commit": "git commit -m 'Lane-Integration: seam'"}[entry.Name]
	case "shell":
		command = entry.Name
	}
	if command == "" {
		return "", false
	}
	return landingCall(t, bed.repo, "Bash", map[string]any{"command": command}), true
}

// TestGateEntriesThroughRealHook: every allowlist entry is admitted and the
// forbidden classes are denied, through both hook routes a landing session
// runs; a seat's session is not governed by the landing gate at all.
func TestGateEntriesThroughRealHook(t *testing.T) {
	t.Parallel()
	bed := newLandingBed(t)
	entries, err := agentgate.Entries()
	if err != nil || len(entries) == 0 {
		t.Fatalf("entries: %v", err)
	}
	engine := directTestEngineEnv + "=allow"
	denied := map[string]string{
		"push":             landingCall(t, bed.repo, "Bash", map[string]any{"command": "git push origin HEAD:main"}),
		"no-verify":        landingCall(t, bed.repo, "Bash", map[string]any{"command": "git commit --no-verify -m x"}),
		"test runner":      landingCall(t, bed.repo, "Bash", map[string]any{"command": "go test ./..."}),
		"test verb":        landingCall(t, bed.repo, "Bash", map[string]any{"command": "metasystem test run"}),
		"internal test":    landingCall(t, bed.repo, "Bash", map[string]any{"command": "metasystem internal test run --json"}),
		"build":            landingCall(t, bed.installation, "Bash", map[string]any{"command": "go run ./cmd/devgate build"}),
		"landing set":      landingCall(t, bed.repo, "Bash", map[string]any{"command": "metasystem landing set /elsewhere"}),
		"landing unset":    landingCall(t, bed.repo, "Bash", map[string]any{"command": "metasystem landing unset"}),
		"landing start":    landingCall(t, bed.repo, "Bash", map[string]any{"command": "metasystem landing start"}),
		"landing restart":  landingCall(t, bed.repo, "Bash", map[string]any{"command": "metasystem landing restart"}),
		"arm":              landingCall(t, bed.repo, "Bash", map[string]any{"command": "metasystem up"}),
		"goal mutation":    landingCall(t, bed.repo, "Bash", map[string]any{"command": "metasystem goal done g"}),
		"config edit":      landingCall(t, bed.repo, "Bash", map[string]any{"command": "git config core.hooksPath /tmp"}),
		"hook edit":        landingCall(t, bed.repo, "Edit", map[string]any{"file_path": filepath.Join(bed.repo, ".git", "hooks", "pre-push"), "old_string": "a", "new_string": "b"}),
		"settings edit":    landingCall(t, bed.repo, "Write", map[string]any{"file_path": filepath.Join(bed.repo, ".claude", "settings.json"), "content": "{}"}),
		"subagent":         landingCall(t, bed.repo, "Agent", map[string]any{"prompt": "push"}),
		"redirection":      landingCall(t, bed.repo, "Bash", map[string]any{"command": "echo x > .git/hooks/pre-push"}),
		"main branch work": landingCall(t, bed.repo, "Bash", map[string]any{"command": "git checkout main"}),
	}
	for route, command := range landingRoutes(t, bed) {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			for _, entry := range entries {
				call, known := sampleCall(t, bed, entry)
				if !known {
					t.Fatalf("the witness has no sample call for allowlist entry %s %q", entry.Kind, entry.Name)
				}
				status, stdout, stderr := bed.run(t, bed.repo, command, call, landingLineage, engine, "TMPDIR="+t.TempDir())
				if status != 0 || stdout != "" {
					t.Errorf("%s %q was not admitted: status %d stdout %q stderr %q", entry.Kind, entry.Name, status, stdout, stderr)
				}
			}
			for name, call := range denied {
				status, stdout, stderr := bed.run(t, bed.repo, command, call, landingLineage, engine, "TMPDIR="+t.TempDir())
				if _, ok := denial(status, stdout, stderr); !ok {
					t.Errorf("%s was not denied: status %d stdout %q stderr %q", name, status, stdout, stderr)
				}
			}
		})
	}
	// A seat is not the landing agent: its push reaches the context gate's
	// own path (here, with no recorded gate, silence), never this denial.
	project := landingRoutes(t, bed)["project"]
	status, stdout, stderr := bed.run(t, bed.repo, project, denied["push"], agentgate.LineageEnv+"=steward-seat", engine, "TMPDIR="+t.TempDir())
	if status != 0 || stdout != "" {
		t.Fatalf("a seat's call met the landing gate: status %d stdout %q stderr %q", status, stdout, stderr)
	}
}

// The landing launch's hook command carries the lineage itself: a session
// whose environment lost it is still gated on that route.
func TestLandingHookCommandGovernsWithoutTheEnvironment(t *testing.T) {
	t.Parallel()
	bed := newLandingBed(t)
	status, stdout, stderr := bed.run(t, bed.repo, landingRoutes(t, bed)["landing"],
		landingCall(t, bed.repo, "Bash", map[string]any{"command": "git push origin HEAD:main"}), directTestEngineEnv+"=allow", "TMPDIR="+t.TempDir())
	if _, ok := denial(status, stdout, stderr); !ok {
		t.Fatalf("a push without the lineage in the environment passed the landing route: status %d stdout %q stderr %q", status, stdout, stderr)
	}
}

// TestHookErrorDenies: for the landing agent, every way the hook can fail
// denies the call instead of letting it through.
func TestHookErrorDenies(t *testing.T) {
	t.Parallel()
	engine := directTestEngineEnv + "=allow"
	push := func(bed directBed) string {
		return landingCall(t, bed.repo, "Bash", map[string]any{"command": "git push origin HEAD:main"})
	}
	t.Run("engine missing", func(t *testing.T) {
		t.Parallel()
		bed := newLandingBed(t)
		command, err := agentgate.HookCommand(filepath.Join(bed.installation, "bin", "gone"), bed.repo)
		if err != nil {
			t.Fatal(err)
		}
		if status, stdout, stderr := bed.run(t, bed.repo, command, landingCall(t, bed.repo, "Read", map[string]any{"file_path": "/x"}), landingLineage); status != 2 {
			t.Fatalf("a missing engine let the call through: status %d stdout %q stderr %q", status, stdout, stderr)
		} else if _, ok := denial(status, stdout, stderr); !ok {
			t.Fatalf("a missing engine's denial is not two lines: %q", stderr)
		}
	})
	t.Run("engine crashes after output", func(t *testing.T) {
		t.Parallel()
		bed := newDirectBed(t, "#!/bin/sh\nprintf '{\"half\n'\nexit 3\n")
		command, err := agentgate.HookCommand(filepath.Join(bed.installation, "bin", "metasystem"), bed.repo)
		if err != nil {
			t.Fatal(err)
		}
		status, stdout, stderr := bed.run(t, bed.repo, command, push(bed), landingLineage)
		if _, ok := denial(status, stdout, stderr); !ok || status != 2 {
			t.Fatalf("a crashed engine let the call through: status %d stdout %q stderr %q", status, stdout, stderr)
		}
	})
	t.Run("checkout gone", func(t *testing.T) {
		t.Parallel()
		bed := newLandingBed(t)
		command, err := agentgate.HookCommand(filepath.Join(bed.installation, "bin", "metasystem"), filepath.Join(bed.repo, "gone"))
		if err != nil {
			t.Fatal(err)
		}
		status, stdout, stderr := bed.run(t, bed.repo, command, push(bed), landingLineage, engine)
		if _, ok := denial(status, stdout, stderr); !ok {
			t.Fatalf("an unreachable checkout let the call through: status %d stdout %q stderr %q", status, stdout, stderr)
		}
	})
	// The engine runs but cannot decide: each case went through the hook's
	// fail-open paths before (delegate job, no recorded gate, unreadable
	// input); for the landing agent every one denies, through both routes.
	for name, call := range map[string]func(directBed) (string, []string){
		"unreadable call": func(bed directBed) (string, []string) { return "{not json", nil },
		"no call":         func(bed directBed) (string, []string) { return "", nil },
		"session cwd gone": func(bed directBed) (string, []string) {
			return landingCall(t, filepath.Join(bed.repo, "gone"), "Bash", map[string]any{"command": "git status"}), nil
		},
		"delegate job env": func(bed directBed) (string, []string) {
			return push(bed), []string{"METASYSTEM_HOOK_DELEGATE_JOB=job-1"}
		},
		"no recorded gate": func(bed directBed) (string, []string) { return push(bed), nil },
		"subagent call": func(bed directBed) (string, []string) {
			encoded, err := json.Marshal(map[string]any{"session_id": "s", "agent_id": "sub-1", "cwd": bed.repo,
				"tool_name": "Bash", "tool_input": map[string]any{"command": "git push"}})
			if err != nil {
				t.Fatal(err)
			}
			return string(encoded), nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newLandingBed(t)
			// A recorded gate that would allow everything: the landing
			// agent's decision never defers to it.
			if name != "no recorded gate" {
				allowAll := filepath.Join(bed.installation, "allow-all")
				if err := testexec.WriteFile(allowAll, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
					t.Fatal(err)
				}
				context := filepath.Join(bed.installation, "artifacts", "agents", "context")
				if err := os.MkdirAll(context, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(context, "engine-path"), []byte(allowAll+"\n"+bed.installation+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			input, extra := call(bed)
			for route, command := range landingRoutes(t, bed) {
				env := append([]string{landingLineage, engine, "TMPDIR=" + t.TempDir()}, extra...)
				status, stdout, stderr := bed.run(t, bed.repo, command, input, env...)
				if _, ok := denial(status, stdout, stderr); !ok {
					t.Errorf("%s route let the call through: status %d stdout %q stderr %q", route, status, stdout, stderr)
				}
			}
		})
	}
}
