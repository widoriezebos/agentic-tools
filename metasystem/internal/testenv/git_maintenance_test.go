package testenv

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// These adapter integrations need Git itself: a stub cannot prove config
// precedence or whether fetch launches a detached maintenance subprocess.
func TestGitFixtureEffectiveMaintenanceConfiguration(t *testing.T) {
	t.Parallel()
	probe := t.TempDir()
	checkTestenv(t, writeFixtureGitConfig(filepath.Join(probe, ".gitconfig")))
	for name, environment := range map[string][]string{
		"probe":            gitProbeEnvironment(os.Environ(), probe),
		"inherited":        os.Environ(),
		"product scrubbed": gittree.ScrubbedEnviron(),
	} {
		t.Run(name, func(t *testing.T) {
			for key, want := range map[string]string{
				"maintenance.auto": "false", "gc.auto": "0",
				"gc.autoDetach": "false", "fetch.writeCommitGraph": "false",
			} {
				command := exec.Command("git", "config", "--get", key)
				command.Dir = t.TempDir()
				command.Env = environment
				output, err := command.CombinedOutput()
				if err != nil || strings.TrimSpace(string(output)) != want {
					t.Errorf("effective %s = %q (%v), want %s", key, output, err, want)
				}
			}
		})
	}
}

func TestGitFixtureFetchStartsNoMaintenance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source, clone := filepath.Join(root, "source"), filepath.Join(root, "clone")
	run := func(input string, args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		command.Stdin = strings.NewReader(input)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	run("", "init", "--bare", source)
	run("commit refs/heads/main\ncommitter Fixture <fixture@example.invalid> 1000000000 +0000\ndata 5\nfirst\n\ndone\n", "-C", source, "fast-import", "--quiet")
	run("", "clone", "--branch=main", source, clone)
	run("commit refs/heads/main\ncommitter Fixture <fixture@example.invalid> 1000000001 +0000\ndata 6\nsecond\nfrom refs/heads/main^0\n\ndone\n", "-C", source, "fast-import", "--quiet")
	trace := filepath.Join(root, "fetch-trace.json")
	command := exec.Command("git", "-C", clone, "fetch", "origin")
	command.Env = append(os.Environ(), "GIT_TRACE2_EVENT="+trace)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fetch: %v\n%s", err, output)
	}
	data, err := os.ReadFile(trace)
	checkTestenv(t, err)
	starts := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var event struct {
			Event string   `json:"event"`
			Argv  []string `json:"argv"`
		}
		checkTestenv(t, json.Unmarshal([]byte(line), &event))
		if event.Event != "child_start" {
			continue
		}
		starts++
		for _, arg := range event.Argv {
			if arg == "maintenance" || arg == "gc" {
				t.Errorf("fetch started maintenance: %v", event.Argv)
			}
		}
	}
	if starts == 0 {
		t.Fatal("fetch trace recorded no child processes")
	}
	for _, name := range []string{"commit-graph", "commit-graphs"} {
		if _, err := os.Stat(filepath.Join(clone, ".git", "objects", "info", name)); !os.IsNotExist(err) {
			t.Errorf("fetch wrote %s: %v", name, err)
		}
	}
}
