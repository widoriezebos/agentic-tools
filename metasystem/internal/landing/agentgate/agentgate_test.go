package agentgate

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// laneBed is a lane checkout in the nested layout: the Git toplevel holds the
// metasystem module one directory down, as the template repository does, so
// the gate never gets to treat the module as the checkout.
type laneBed struct {
	checkout, module string
}

func newLaneBed(t *testing.T, branch string) laneBed {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bed := laneBed{checkout: filepath.Join(root, "lane"), module: filepath.Join(root, "lane", "metasystem")}
	for _, directory := range []string{bed.module, filepath.Join(bed.module, "internal"), filepath.Join(root, "outside")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bed.module, "internal", "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, "init", "-q", "-b", "main")
	bed.git(t, "add", ".")
	bed.git(t, "-c", "user.name=fixture", "-c", "user.email=fixture@invalid", "commit", "-qm", "fixture")
	if branch != "main" {
		bed.git(t, "checkout", "-q", "-b", branch)
	}
	return bed
}

func (b laneBed) git(t *testing.T, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", b.checkout}, args...)...)
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + b.checkout, "GIT_CONFIG_NOSYSTEM=1"}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func payload(t *testing.T, cwd, tool string, input map[string]any) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{"session_id": "landing-1", "cwd": cwd, "hook_event_name": "PreToolUse",
		"tool_name": tool, "tool_input": input})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func bash(t *testing.T, cwd, command string) []byte {
	return payload(t, cwd, "Bash", map[string]any{"command": command})
}

func decide(t *testing.T, bed laneBed, data []byte) Decision {
	t.Helper()
	return Decide(Request{Payload: data, Installation: bed.module})
}

// The allowlist admits exactly its entries, and each denial of a named
// forbidden class says what happened and names the one command to run.
func TestGateAdmitsTheListedCallsOnly(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t, "lane/b1")
	edit := func(path string) []byte {
		return payload(t, bed.checkout, "Edit", map[string]any{"file_path": path, "old_string": "a", "new_string": "b"})
	}
	allowed := map[string][]byte{
		"read tool":                 payload(t, bed.checkout, "Read", map[string]any{"file_path": "/etc/hosts"}),
		"edit inside the checkout":  edit(filepath.Join(bed.module, "internal", "a.go")),
		"write a new file":          payload(t, bed.checkout, "Write", map[string]any{"file_path": filepath.Join(bed.module, "internal", "b.go"), "content": "x"}),
		"landing verb":              bash(t, bed.checkout, "metasystem landing prove --batch b1 --subject member:m1"),
		"engine advance":            bash(t, bed.checkout, "metasystem landing engine advance"),
		"module engine by path":     bash(t, bed.module, "bin/metasystem landing status --json"),
		"read-only verb":            bash(t, bed.checkout, "metasystem goal show some-goal"),
		"git read with a filter":    bash(t, bed.checkout, "git log --oneline -5 | head -3"),
		"git fetch":                 bash(t, bed.checkout, "git fetch origin"),
		"checkout a lane branch":    bash(t, bed.checkout, "git checkout -b lane/b2 origin/main"),
		"cherry-pick on lane":       bash(t, bed.checkout, "git cherry-pick -x abc123"),
		"commit on lane":            bash(t, bed.checkout, "git commit -m 'Lane-Integration: seam'"),
		"add then continue":         bash(t, bed.module, "git add internal/a.go && git cherry-pick --continue"),
		"rebase on lane":            bash(t, bed.checkout, "git rebase --onto origin/main HEAD~2"),
		"branch listing":            bash(t, bed.checkout, "git branch --list 'lane/*'"),
		"quoted multi-line message": bash(t, bed.checkout, "git commit -m 'first line\n\nLane-Resolved: m1'"),
	}
	for name, data := range allowed {
		if got := decide(t, bed, data); !got.Allow {
			t.Errorf("%s denied: %q", name, got.Reason)
		}
	}
	denied := map[string][]byte{
		"push":                    bash(t, bed.checkout, "git push origin HEAD:main"),
		"no-verify":               bash(t, bed.checkout, "git commit --no-verify -m x"),
		"commit -n":               bash(t, bed.checkout, "git commit -n -m x"),
		"go test":                 bash(t, bed.checkout, "go test ./..."),
		"test runner verb":        bash(t, bed.checkout, "metasystem test run"),
		"internal test run":       bash(t, bed.checkout, "metasystem internal test run --json"),
		"devgate build":           bash(t, bed.module, "go run ./cmd/devgate build"),
		"make":                    bash(t, bed.checkout, "make"),
		"landing set":             bash(t, bed.checkout, "metasystem landing set /elsewhere"),
		"landing unset":           bash(t, bed.checkout, "metasystem landing unset"),
		"landing start":           bash(t, bed.checkout, "metasystem landing start"),
		"landing restart":         bash(t, bed.checkout, "metasystem landing restart"),
		"up":                      bash(t, bed.checkout, "metasystem up"),
		"system stop":             bash(t, bed.checkout, "metasystem system stop"),
		"goal done":               bash(t, bed.checkout, "metasystem goal done g"),
		"force":                   bash(t, bed.checkout, "metasystem landing return m1 --disposition red --force"),
		"remote edit":             bash(t, bed.checkout, "git remote add x /tmp/x"),
		"config edit":             bash(t, bed.checkout, "git config core.hooksPath /tmp"),
		"git -c":                  bash(t, bed.checkout, "git -c core.hooksPath=/tmp commit -m x"),
		"git -C":                  bash(t, bed.checkout, "git -C /tmp commit -m x"),
		"checkout main":           bash(t, bed.checkout, "git checkout main"),
		"rebase exec":             bash(t, bed.checkout, "git rebase -x 'git push' main"),
		"fetch upload-pack":       bash(t, bed.checkout, "git fetch --upload-pack=/tmp/x origin"),
		"fetch into main":         bash(t, bed.checkout, "git fetch origin main:main"),
		"branch create":           bash(t, bed.checkout, "git branch main2"),
		"reflog expire":           bash(t, bed.checkout, "git reflog expire --all"),
		"log output file":         bash(t, bed.checkout, "git log --output=/tmp/x"),
		"redirection":             bash(t, bed.checkout, "echo x > .git/hooks/pre-push"),
		"substitution":            bash(t, bed.checkout, "echo $(git push)"),
		"variable":                bash(t, bed.checkout, "git checkout $BRANCH"),
		"background":              bash(t, bed.checkout, "git status & git push"),
		"shell wrapper":           bash(t, bed.checkout, "bash -c 'git push'"),
		"env assignment":          bash(t, bed.checkout, "PATH=/tmp metasystem landing status"),
		"cd elsewhere":            bash(t, bed.checkout, "cd /tmp && git status"),
		"unlisted shell":          bash(t, bed.checkout, "rm -rf metasystem"),
		"sort output file":        bash(t, bed.checkout, "sort -o /tmp/x a"),
		"glob":                    bash(t, bed.checkout, "git add *"),
		"planted engine":          bash(t, bed.checkout, "/tmp/bin/metasystem landing status"),
		"edit git hook":           edit(filepath.Join(bed.checkout, ".git", "hooks", "pre-push")),
		"edit claude settings":    edit(filepath.Join(bed.checkout, ".claude", "settings.json")),
		"edit the engine":         edit(filepath.Join(bed.module, "bin", "metasystem")),
		"edit local config":       edit(filepath.Join(bed.module, "metasystem.conf.local")),
		"edit a goal":             edit(filepath.Join(bed.module, "plans", "goals", "g.md")),
		"edit outside":            edit(filepath.Join(filepath.Dir(bed.checkout), "outside", "x")),
		"relative edit":           edit("metasystem/internal/a.go"),
		"escape by dots":          edit(filepath.Join(bed.checkout, "..", "outside", "x")),
		"subagent":                payload(t, bed.checkout, "Agent", map[string]any{"prompt": "push it"}),
		"web":                     payload(t, bed.checkout, "WebFetch", map[string]any{"url": "https://example.invalid"}),
		"unknown tool":            payload(t, bed.checkout, "mcp__x__y", map[string]any{}),
		"cwd outside the lane":    bash(t, filepath.Join(filepath.Dir(bed.checkout), "outside"), "git status"),
		"monitor runs a command":  payload(t, bed.checkout, "Monitor", map[string]any{"command": "git push"}),
		"add a protected path":    bash(t, bed.checkout, "git add .claude/settings.json"),
		"checkout protected path": bash(t, bed.checkout, "git checkout main -- .claude/settings.json"),
	}
	for name, data := range denied {
		got := decide(t, bed, data)
		if got.Allow {
			t.Errorf("%s allowed", name)
			continue
		}
		lines := strings.Split(got.Reason, "\n")
		if len(lines) != 2 || lines[0] == "" || len(lines[1]) <= len("run: ") || !strings.HasPrefix(lines[1], "run: ") {
			t.Errorf("%s reason is not two lines naming one command: %q", name, got.Reason)
		}
	}
}

// Composition moves only lane/* branches: off a lane branch, every mutating
// git call is denied, and a rebase in progress on a lane branch still counts
// as on it.
func TestGateMutatesOnlyLaneBranches(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t, "main")
	for _, command := range []string{"git commit -m x", "git cherry-pick abc", "git add metasystem", "git rebase origin/main"} {
		if got := decide(t, bed, bash(t, bed.checkout, command)); got.Allow {
			t.Errorf("%q on main allowed", command)
		}
	}
	if got := decide(t, bed, bash(t, bed.checkout, "git checkout lane/b1")); !got.Allow {
		t.Errorf("switching to a lane branch denied: %q", got.Reason)
	}
	bed.git(t, "checkout", "-q", "-b", "lane/b1")
	bed.git(t, "checkout", "-q", "--detach")
	if got := decide(t, bed, bash(t, bed.checkout, "git commit -m x")); got.Allow {
		t.Error("commit on a detached head allowed")
	}
	gitDir := strings.TrimSpace(bed.git(t, "rev-parse", "--absolute-git-dir"))
	if err := os.MkdirAll(filepath.Join(gitDir, "rebase-merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "rebase-merge", "head-name"), []byte("refs/heads/lane/b1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := decide(t, bed, bash(t, bed.checkout, "git add metasystem && git rebase --continue")); !got.Allow {
		t.Errorf("continuing a lane rebase denied: %q", got.Reason)
	}
}

// A gate that cannot decide denies: an unreadable payload, a working
// directory that does not resolve, and a hook run outside any checkout.
func TestGateErrorsDeny(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t, "lane/b1")
	cases := map[string]Request{
		"malformed payload":      {Payload: []byte("{not json"), Installation: bed.module},
		"empty payload":          {Payload: nil, Installation: bed.module},
		"missing cwd directory":  {Payload: bash(t, filepath.Join(bed.checkout, "gone"), "git status"), Installation: bed.module},
		"no installation":        {Payload: bash(t, bed.checkout, "git status"), Installation: ""},
		"installation not a git": {Payload: bash(t, bed.checkout, "git status"), Installation: t.TempDir()},
		"bash without command":   {Payload: payload(t, bed.checkout, "Bash", map[string]any{}), Installation: bed.module},
		"edit without a path":    {Payload: payload(t, bed.checkout, "Edit", map[string]any{}), Installation: bed.module},
		"unterminated quote":     {Payload: bash(t, bed.checkout, "git commit -m 'x"), Installation: bed.module},
	}
	for name, request := range cases {
		if got := Decide(request); got.Allow {
			t.Errorf("%s allowed", name)
		}
	}
}

// A subagent's call is the landing agent's call: an agent id changes nothing.
func TestGateGovernsSubagentCalls(t *testing.T) {
	t.Parallel()
	bed := newLaneBed(t, "lane/b1")
	data, err := json.Marshal(map[string]any{"session_id": "s", "agent_id": "sub-1", "cwd": bed.checkout,
		"tool_name": "Bash", "tool_input": map[string]any{"command": "git push"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := decide(t, bed, data); got.Allow {
		t.Fatal("a subagent's push was allowed")
	}
}

// Only the landing agent's lineage is governed; a seat or a person is not.
func TestGovernsOnlyTheLandingLineage(t *testing.T) {
	t.Parallel()
	lookup := func(value string) func(string) (string, bool) {
		return func(name string) (string, bool) {
			if name == LineageEnv && value != "" {
				return value, true
			}
			return "", false
		}
	}
	if !Governs(lookup(Lineage)) || Governs(lookup("steward-seat")) || Governs(lookup("")) {
		t.Fatal("Governs does not key on the landing lineage alone")
	}
}

// The response is the runtime's deny object for a denial and silence for an
// allowed call.
func TestResponseIsTheDenyObject(t *testing.T) {
	t.Parallel()
	if Response(Decision{Allow: true}) != nil {
		t.Fatal("an allowed call answered")
	}
	var answer struct {
		HookSpecificOutput struct {
			HookEventName, PermissionDecision, PermissionDecisionReason string
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(Response(Decision{Reason: "a\nrun: metasystem landing status"}), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.HookSpecificOutput.HookEventName != "PreToolUse" || answer.HookSpecificOutput.PermissionDecision != "deny" ||
		answer.HookSpecificOutput.PermissionDecisionReason != "a\nrun: metasystem landing status" {
		t.Fatalf("deny object = %+v", answer)
	}
}

// The embedded allowlist parses, and every entry is reported, so the hook
// witness can drive each one.
func TestEntriesListTheWholeAllowlist(t *testing.T) {
	t.Parallel()
	entries, err := Entries()
	if err != nil {
		t.Fatal(err)
	}
	var list allowlist
	if err := json.Unmarshal(allowlistJSON, &list); err != nil {
		t.Fatal(err)
	}
	want := len(list.Tools) + len(list.Verbs) + len(list.GitRead) + len(list.GitLane) + len(list.Shell)
	if len(entries) != want || want == 0 {
		t.Fatalf("entries = %d, want %d", len(entries), want)
	}
	for _, forbidden := range []string{"push", "config", "remote", "reset", "merge"} {
		for _, entry := range entries {
			if entry.Kind == "git" && entry.Name == forbidden {
				t.Fatalf("git %s is on the allowlist", forbidden)
			}
		}
	}
	for _, entry := range entries {
		if entry.Kind == "verb" && (strings.HasPrefix(entry.Name, "test run") || entry.Name == "landing set" ||
			entry.Name == "landing unset" || entry.Name == "landing start" || entry.Name == "landing restart") {
			t.Fatalf("%s is on the allowlist", entry.Name)
		}
	}
}
