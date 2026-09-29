package supervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// delegateCacheRound is one delegate round of a runtime in a shared
// checkout (no job worktree), with the user cache dir behind the seam.
type delegateCacheRound struct {
	turn  *Turn
	cache string
}

func newDelegateCacheRound(t *testing.T) delegateCacheRound {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(root, "user-cache")
	workspace := filepath.Join(root, "checkout")
	dir := filepath.Join(root, "round")
	for _, path := range []string{workspace, dir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	record := filepath.Join(root, "job.json")
	mustWrite(t, record, `{"role":"implementer","instanceTag":"tag-1","workspaceRoot":"`+workspace+`","permissions":{"requested":{"readRoots":["`+workspace+`"],"writeRoots":["`+workspace+`"],"network":"deny","tools":"runtime-default","approvals":"deny"}}}`)
	effective := filepath.Join(dir, "effective.json")
	mustWrite(t, effective, `{"writeRoots":["`+workspace+`"]}`)
	schema := filepath.Join(root, "schema.json")
	mustWrite(t, schema, `{"type":"object"}`)
	d := Deps{
		Root: root, Engine: "/opt/bin/metasystem", Environ: []string{"TMPDIR=" + filepath.Join(root, "tmp")},
		Getenv:       func(name string) string { return map[string]string{"TMPDIR": filepath.Join(root, "tmp")}[name] },
		Git:          fakeGit(map[string]map[string]string{}),
		UserCacheDir: func() (string, error) { return cache, nil },
	}
	turn := &Turn{d: d, Role: RoleDelegate, Verb: runtimes.SupervisorDispatch, Root: root, Workspace: workspace, Dir: dir,
		Record: record, Job: "job-1", Tag: "tag-1", Round: "1", RootJob: "job-1", Prompt: filepath.Join(root, "prompt.md"),
		Schema: schema, Model: "m", Effective: effective, Requested: record, Events: filepath.Join(dir, "events.jsonl"), Log: &syncBuffer{}}
	return delegateCacheRound{turn: turn, cache: cache}
}

func (r delegateCacheRound) delegatePair() (string, string) {
	return filepath.Join(r.cache, "metasystem-delegate-go-build"), filepath.Join(r.cache, "metasystem-delegate-staticcheck")
}

// A Claude round in a shared checkout builds in the machine delegate cache
// (disk-lifetimes A7): no per-round go-cache under its scratch, TMPDIR and
// GOTMPDIR stay per round, the sandbox grants both delegate directories and
// denies both engine directories, and the round records the delegate cache.
func TestClaudeRoundBuildsInTheDelegateCache(t *testing.T) {
	t.Parallel()
	round := newDelegateCacheRound(t)
	launch, err := claudeOps{}.Prepare(round.turn)
	if err != nil || launch.Refusal != nil || launch.SetupError != nil {
		t.Fatalf("prepare: %v %+v %v", err, launch.Refusal, launch.SetupError)
	}
	goCache, staticcheck := round.delegatePair()
	scratch := filepath.Join(round.turn.d.Getenv("TMPDIR"), "metasystem-claude", "job-1-1")
	for name, want := range map[string]string{"GOCACHE": goCache, "STATICCHECK_CACHE": staticcheck, "TMPDIR": scratch, "GOTMPDIR": filepath.Join(scratch, "go-tmp")} {
		if got := envValue(launch.Env, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(scratch, "go-cache")); !os.IsNotExist(err) {
		t.Errorf("a per-round go-cache was made: %v", err)
	}
	for _, dir := range []string{goCache, staticcheck} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("delegate cache %s not made: %v", dir, err)
		}
	}
	var settings struct {
		Sandbox struct {
			Filesystem struct {
				AllowWrite []string `json:"allowWrite"`
				DenyWrite  []string `json:"denyWrite"`
			} `json:"filesystem"`
		} `json:"sandbox"`
	}
	data, _ := os.ReadFile(filepath.Join(round.turn.Dir, "claude-settings.json"))
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	fs := settings.Sandbox.Filesystem
	if !slices.Contains(fs.AllowWrite, goCache) || !slices.Contains(fs.AllowWrite, staticcheck) {
		t.Errorf("allowWrite lacks the delegate cache: %v", fs.AllowWrite)
	}
	if !slices.Contains(fs.DenyWrite, filepath.Join(round.cache, "go-build")) || !slices.Contains(fs.DenyWrite, filepath.Join(round.cache, "staticcheck")) {
		t.Errorf("denyWrite lacks the engine cache: %v", fs.DenyWrite)
	}
	if got := readText(t, filepath.Join(round.turn.Dir, "build-cache.txt")); got != goCache+"\n" {
		t.Errorf("recorded cache = %q", got)
	}
}

// A Codex round's sandbox gets both delegate directories as extra write
// roots, and its environment carries them.
func TestCodexRoundBuildsInTheDelegateCache(t *testing.T) {
	t.Parallel()
	round := newDelegateCacheRound(t)
	round.turn.Env = nil
	launch, err := codexOps{}.Prepare(round.turn)
	if err != nil || launch.Refusal != nil {
		t.Fatalf("prepare: %v %+v", err, launch.Refusal)
	}
	goCache, staticcheck := round.delegatePair()
	argv := strings.Join(launch.Argv, " ")
	for _, dir := range []string{goCache, staticcheck} {
		if !strings.Contains(argv, "--add-dir "+dir) {
			t.Errorf("argv lacks --add-dir %s: %s", dir, argv)
		}
	}
	if envValue(launch.Env, "GOCACHE") != goCache || envValue(launch.Env, "STATICCHECK_CACHE") != staticcheck {
		t.Errorf("env = %v", launch.Env)
	}
	if got := readText(t, filepath.Join(round.turn.Dir, "build-cache.txt")); got != goCache+"\n" {
		t.Errorf("recorded cache = %q", got)
	}
}

// Devin has no OS sandbox to grant; the export alone puts a cooperating
// Devin's builds in the delegate cache, at every launch form.
func TestDevinLaunchFormsExportTheDelegateCache(t *testing.T) {
	t.Parallel()
	round := newDelegateCacheRound(t)
	goCache, staticcheck := round.delegatePair()
	env := delegateRoundEnv(round.turn.d, round.turn.Workspace)
	if envValue(env, "GOCACHE") != goCache || envValue(env, "STATICCHECK_CACHE") != staticcheck {
		t.Fatalf("delegate round env = %v", env)
	}
	source, err := os.ReadFile("devin.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(source), "jobGitQuarantineEnv(") {
		t.Fatal("a Devin launch form exports the quarantine environment without the delegate cache; use delegateRoundEnv")
	}
	if count := strings.Count(string(source), "delegateRoundEnv(d, t.Workspace)"); count != 3 {
		t.Fatalf("Devin launch forms using delegateRoundEnv = %d, want 3", count)
	}
}
