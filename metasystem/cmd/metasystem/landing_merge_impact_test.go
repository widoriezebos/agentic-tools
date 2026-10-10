package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
)

// Git topology and the real impact command must agree on the pending merge's first parent.
func TestLaneMergeRealImpactCheckGitAdapter(t *testing.T) {
	t.Parallel()
	for _, green := range []bool{true, false} {
		t.Run(map[bool]string{true: "green", false: "red"}[green], func(t *testing.T) {
			t.Parallel()
			root, _ := impactGitAdapterBed(t)
			git := func(args ...string) string { return strings.TrimSpace(testingFixtureGit(t, root, args...)) }
			git("checkout", "-qb", "goal")
			impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = false\n")
			git("add", ".")
			git("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "goal")
			tip := git("rev-parse", "HEAD")
			git("checkout", "main")
			impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = true // main intent\n")
			git("add", ".")
			git("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "main")
			base := git("rev-parse", "HEAD")
			if _, err := plain.Git(root, "merge", "--no-commit", "goal"); err == nil {
				t.Fatal("fixture must conflict")
			}
			fix := &plain.Fix{Goal: "goal", Units: []string{"lane-merge-1"}, Commit: base, Tip: tip, State: "resolving", Attempt: "attempt", Paths: []conflict.Path{{Path: "internal/launch/value.go", Class: conflict.Builder}}}
			if err := plain.WriteFix(root, fix); err != nil {
				t.Fatal(err)
			}
			if _, _, err := plain.HandIn(root, plain.Line{Goal: "goal", SHA: tip}); err != nil {
				t.Fatal(err)
			}
			value := "true"
			if !green {
				value = "false"
			}
			impactWrite(t, root, "internal/launch/value.go", "package launch\nconst Value = "+value+" // resolved intent\n")
			git("add", "internal/launch/value.go")
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			// Only this child dispatches as the engine; the check uses its real command runner.
			check := "GO_WANT_BATCH_E2E_COMMAND=1 " + shellquote.Token(executable) + " test impact"
			seams := plain.ResolveSeams{Git: func(dir string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "commit" {
					args = append([]string{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid"}, args...)
				}
				return plain.Git(dir, args...)
			}}
			home := t.TempDir()
			if err := os.MkdirAll(filepath.Join(home, "host"), 0o755); err != nil {
				t.Fatal(err)
			}
			err = plain.CompleteMerge(home, root, root, fix, check, seams)
			logs, globErr := filepath.Glob(filepath.Join(plain.Dir(root), "merge-check-*.log"))
			if globErr != nil || len(logs) != 1 {
				t.Fatalf("logs=%v err=%v", logs, globErr)
			}
			data, readErr := os.ReadFile(logs[0])
			if readErr != nil {
				t.Fatal(readErr)
			}
			t.Logf("check log:\n%s", data)
			if !strings.Contains(string(data), "plan: base "+base) {
				t.Fatalf("first parent did not reach the real check: %v\n%s", err, data)
			}
			if green {
				if err != nil || fix.State != "reviewing" || !strings.Contains(string(data), "landing group unit/internal/launch green") {
					t.Fatalf("check=%v state=%s\n%s", err, fix.State, data)
				}
				if parents := git("show", "-s", "--format=%P", "HEAD"); parents != base+" "+tip {
					t.Fatalf("parents=%s", parents)
				}
			} else if err == nil || git("rev-parse", "HEAD") != base || git("rev-parse", "MERGE_HEAD") != tip || !strings.Contains(string(data), "TestValue") {
				t.Fatalf("red check committed or lost the merge: %v\n%s", err, data)
			}
		})
	}
}
