package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

func ownEngineFixture(t *testing.T) (intentOwners, string, string, string) {
	t.Helper()
	root := t.TempDir()
	installation := filepath.Join(root, "goal tree", "vendor", "tool")
	executable := filepath.Join(installation, "bin", "metasystem")
	checkout := filepath.Join(root, "primary tree")
	engine := filepath.Join(checkout, "vendor", "tool", "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{filepath.Join(installation, "metasystem.conf"): "", executable: "build"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return intentOwners{ownEngine: ownEngineOwners{
		executable: func() (string, error) { return executable, nil },
		serving: func(root string) (string, string) {
			if root != realpath.Resolve(installation) {
				t.Fatalf("installation = %q, want %q", root, installation)
			}
			return filepath.Dir(filepath.Dir(engine)), checkout
		},
		lookupEnv: func(string) (string, bool) { return "", false },
	}}, executable, checkout, engine
}

func TestAnAgentRunningAGoalWorktreesOwnEngineIsRefused(t *testing.T) {
	t.Parallel()
	for _, class := range []string{lease.ClassMain, lease.ClassDelegate} {
		t.Run(class, func(t *testing.T) {
			owners, executable, checkout, engine := ownEngineFixture(t)
			calls, ran := 0, false
			owners.agent.caller = func(_ *intentInvocation, root string) string {
				calls++
				if root != checkout {
					t.Fatalf("caller checkout = %q, want %q", root, checkout)
				}
				return class
			}
			command, _ := findIntentCommand("goal show")
			command.run = func(*intentInvocation) int { ran = true; return 7 }
			raw := []string{"a-goal", "--history", "--repo=other tree", "--json"}
			var stdout, stderr bytes.Buffer
			code := runIntentIn(command, raw, &stdout, &stderr, t.TempDir(), owners)
			var result intentResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			want := append([]string{engine, "goal", "show"}, raw...)
			if code != 1 || ran || calls != 1 || stderr.Len() != 0 || result.Outcome != intentRefused || result.Next == nil || !slices.Equal(result.Next.Argv, want) || result.Summary != "this engine is the goal worktree's own build, so nothing was done" || len(result.Details) != 1 || !strings.Contains(result.Details[0], "METASYSTEM_BIN") || !strings.Contains(result.Details[0], executable) {
				t.Fatalf("code=%d ran=%t calls=%d stderr=%q result=%+v", code, ran, calls, stderr.String(), result)
			}
		})
	}
}

func TestAPersonRunningAGoalWorktreesOwnEngineIsToldWhichEngineRuns(t *testing.T) {
	t.Parallel()
	// The caller seam returns an empty class when classification fails.
	for _, class := range []string{lease.ClassHuman, "", lease.ClassUntrusted} {
		t.Run("class="+class, func(t *testing.T) {
			owners, executable, checkout, engine := ownEngineFixture(t)
			calls, ran := 0, false
			owners.agent.caller = func(_ *intentInvocation, root string) string {
				calls++
				if root != checkout {
					t.Fatalf("caller checkout = %q, want %q", root, checkout)
				}
				return class
			}
			command, _ := findIntentCommand("goal show")
			command.run = func(*intentInvocation) int { ran = true; return 7 }
			var stdout, stderr bytes.Buffer
			code := runIntentIn(command, []string{"a-goal"}, &stdout, &stderr, t.TempDir(), owners)
			line := stderr.String()
			if code != 7 || !ran || calls != 1 || stdout.Len() != 0 || strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") || !strings.Contains(line, executable) || !strings.Contains(line, engine) {
				t.Fatalf("code=%d ran=%t calls=%d stdout=%q stderr=%q", code, ran, calls, stdout.String(), line)
			}
		})
	}
}

func TestTheOwnEngineCheckLeavesOtherEnginesAlone(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"explicit build through symlink", "serves itself", "help", "argument error"} {
		t.Run(name, func(t *testing.T) {
			owners, executable, _, _ := ownEngineFixture(t)
			owners.agent.caller = func(*intentInvocation, string) string { t.Fatal("caller classified"); return "" }
			raw, want, ran := []string{"a-goal"}, 7, false
			switch name {
			case "explicit build through symlink":
				link := filepath.Join(t.TempDir(), "engine")
				if err := os.Symlink(executable, link); err != nil {
					t.Fatal(err)
				}
				owners.ownEngine.lookupEnv = func(string) (string, bool) { return link, true }
			case "serves itself":
				owners.ownEngine.serving = func(root string) (string, string) { return root, "" }
			case "help", "argument error":
				owners.ownEngine.executable = func() (string, error) { t.Fatal("engine checked before input"); return "", nil }
				if name == "help" {
					raw, want = []string{"--help"}, 0
				} else {
					raw, want = []string{"--unknown"}, 2
				}
			}
			command, _ := findIntentCommand("goal show")
			command.run = func(*intentInvocation) int { ran = true; return 7 }
			var stdout, stderr bytes.Buffer
			code := runIntentIn(command, raw, &stdout, &stderr, t.TempDir(), owners)
			if code != want || ran != (want == 7) || want == 7 && (stdout.Len() != 0 || stderr.Len() != 0) {
				t.Fatalf("code=%d ran=%t stdout=%q stderr=%q", code, ran, stdout.String(), stderr.String())
			}
		})
	}
}
