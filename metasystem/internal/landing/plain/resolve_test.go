package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type resolveFixture struct {
	t                                *testing.T
	home, checkout, install          string
	paths, tracked, untracked, dirty string
	writes                           [][]string
	contract                         testpolicy.Contract
	seams                            ResolveSeams
}

func newResolveFixture(t *testing.T) *resolveFixture {
	t.Helper()
	b := &resolveFixture{t: t, home: t.TempDir(), checkout: t.TempDir(),
		paths: "metasystem/out/conflict\x00", tracked: "metasystem/out/conflict\x00metasystem/out/other\x00metasystem/src/input\x00"}
	b.install = filepath.Join(b.checkout, "metasystem")
	if err := os.MkdirAll(filepath.Dir(lane.LockPath(b.home)), 0o755); err != nil {
		t.Fatal(err)
	}
	b.contract.Generated = []testpolicy.Generated{
		{Paths: []string{"out/**"}, Command: []string{"compile", "literal; $(no shell)"}, Then: []string{"finish", "second"}, Cwd: "src"},
		{Paths: []string{"unused/**"}, Command: []string{"must-not-run"}},
	}
	b.seams = ResolveSeams{Git: b.git, Now: func() time.Time { return bedNow },
		Run: func([]string, string, *os.File, func(int64) error) error {
			t.Fatal("unexpected regeneration command")
			return nil
		}}
	if _, _, err := HandIn(b.install, Line{Goal: "goal", SHA: "goal-sha", Branch: "goal/goal"}); err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *resolveFixture) git(dir string, args ...string) (string, error) {
	b.t.Helper()
	if dir != b.checkout {
		b.t.Fatalf("Git directory=%q; want %q", dir, b.checkout)
	}
	switch strings.Join(args, " ") {
	case "diff --name-only --diff-filter=U -z":
		return b.paths, nil
	case "rev-parse --verify refs/remotes/origin/main^{commit}", "rev-parse --verify HEAD^{commit}":
		return "main-sha", nil
	case "rev-parse --verify MERGE_HEAD^{commit}":
		return "goal-sha", nil
	case "diff --name-only -z AUTO_MERGE --":
		return b.dirty, nil
	case "ls-files --others --exclude-standard -z":
		return b.untracked, nil
	case "ls-files -z":
		return b.tracked, nil
	}
	if args[0] == "ls-tree" {
		return b.paths, nil
	}
	if args[0] == "ls-files" && args[1] == "--unmerged" {
		return fmt.Sprintf("100644 base 1\t%s\x00100644 main 2\t%s\x00100644 goal 3\t%s\x00", args[4], args[4], args[4]), nil
	}
	if args[0] == "diff" && args[1] == "--no-ext-diff" {
		return "@@ -2,0 +3 @@\n+insert\n", nil
	}
	if args[0] == "restore" || args[0] == "clean" || args[0] == "add" || strings.Join(args, " ") == "merge --abort" {
		b.writes = append(b.writes, append([]string(nil), args...))
		return "", nil
	}
	b.t.Fatalf("unexpected Git call %v", args)
	return "", nil
}

func (b *resolveFixture) resolve() (ResolveOutcome, error) {
	return Resolve(b.home, b.install, b.checkout, b.contract, b.seams)
}

func (b *resolveFixture) record(out ResolveOutcome) {
	b.t.Helper()
	rows, err := readLines[Regeneration](regeneratePath(b.install))
	if err != nil || len(rows) != 1 || !reflect.DeepEqual(rows[0], out.Regeneration) {
		b.t.Fatalf("regeneration rows=%+v err=%v; want one outcome %+v", rows, err, out.Regeneration)
	}
	if _, err := os.Stat(resultsPath(b.install)); !errors.Is(err, os.ErrNotExist) {
		b.t.Fatalf("regeneration wrote a proof result: %v", err)
	}
	last, err := LastRegeneration(b.install)
	if err != nil || last == nil || !reflect.DeepEqual(*last, out.Regeneration) {
		b.t.Fatalf("last regeneration=%+v err=%v", last, err)
	}
	if _, err := os.Stat(regenerationRunningPath(b.install)); !errors.Is(err, os.ErrNotExist) {
		b.t.Fatalf("completed command remains running: %v", err)
	}
}

func TestResolveGeneratedRunsOrderedArgvAndStagesDeclaredOutputs(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	var commands [][]string
	b.seams.Run = func(argv []string, dir string, log *os.File, started func(int64) error) error {
		commands = append(commands, append([]string(nil), argv...))
		if dir != filepath.Join(b.install, "src") {
			t.Fatalf("cwd=%s", dir)
		}
		if err := started(0); err != nil {
			t.Fatal(err)
		}
		if _, err := log.WriteString("rebuilt\n"); err != nil {
			t.Fatal(err)
		}
		b.untracked = "metasystem/out/new\x00"
		return nil
	}
	out, err := b.resolve()
	wantCommands := [][]string{b.contract.Generated[0].Command, b.contract.Generated[0].Then}
	if err != nil || out.Outcome != "resolved" || out.Exit != 0 || out.Entry != nil || out.Held || !reflect.DeepEqual(commands, wantCommands) || !reflect.DeepEqual(out.Command, wantCommands) {
		t.Fatalf("out=%+v commands=%v err=%v", out, commands, err)
	}
	wantWrites := [][]string{
		{"restore", "--source=HEAD", "--staged", "--worktree", "--", "metasystem/out/conflict"},
		{"add", "-A", "--", "metasystem/out/conflict", "metasystem/out/other", "metasystem/out/new"},
	}
	if !reflect.DeepEqual(b.writes, wantWrites) {
		t.Fatalf("Git writes=%v; want %v", b.writes, wantWrites)
	}
	b.record(out)
	entry, _, err := Latest(b.install, "goal")
	if err != nil || entry.State != StateWaiting {
		t.Fatalf("queue=%+v err=%v", entry, err)
	}
	data, err := os.ReadFile(out.Log)
	if err != nil || string(data) != "rebuilt\nrebuilt\n" {
		t.Fatalf("log=%q err=%v", data, err)
	}
	// The second invocation observes the staged tree and must append nothing.
	before, err := os.ReadFile(regeneratePath(b.install))
	if err != nil {
		t.Fatal(err)
	}
	b.paths = ""
	second, err := b.resolve()
	after, readErr := os.ReadFile(regeneratePath(b.install))
	if err != nil || readErr != nil || !second.Held || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(b.writes, wantWrites) {
		t.Fatalf("repeat=%+v err=%v read=%v writes=%v", second, err, readErr, b.writes)
	}
}

func TestResolveSourceAbortsAndReturnsStructuredConflictWithoutEditing(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	b.paths += "metasystem/src/list.go\x00"
	out, err := b.resolve()
	want := &conflict.Return{Main: "main-sha", Paths: []conflict.Path{
		{Path: "metasystem/out/conflict", Class: conflict.Generated},
		{Path: "metasystem/src/list.go", Class: conflict.Builder, Resolution: "keep both, main's lines then the goal's"},
	}}
	if err != nil || out.Outcome != "returned" || out.Entry == nil || out.Entry.State != StateReturned || !reflect.DeepEqual(out.Conflict, want) {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if !reflect.DeepEqual(b.writes, [][]string{{"merge", "--abort"}}) || len(out.Command) != 0 {
		t.Fatalf("source conflict edited or regenerated: writes=%v commands=%v", b.writes, out.Command)
	}
	entry, _, err := Latest(b.install, "goal")
	if err != nil || !reflect.DeepEqual(entry.Conflict, want) {
		t.Fatalf("queue conflict=%+v err=%v", entry.Conflict, err)
	}
	b.record(out)
}

// A non-zero regeneration returns only when its commands pass before the merge.
func TestResolveFailedCommandRestoresAbortsAndReturnsOwnWithBaselineAndLog(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	runs := 0
	b.seams.Run = func(_ []string, _ string, log *os.File, started func(int64) error) error {
		runs++
		if runs > 1 {
			return nil
		}
		if err := started(0); err != nil {
			return err
		}
		if _, err := log.WriteString("compile failed\n"); err != nil {
			return err
		}
		b.untracked = "metasystem/out/new\x00metasystem/unrelated\x00"
		return exec.Command("/usr/bin/false").Run()
	}
	out, err := b.resolve()
	if err == nil || out.Outcome != "returned" || out.Exit != 1 || out.Entry == nil || out.Entry.State != StateReturned || !strings.Contains(out.Entry.Reason, "exited 1; log: "+out.Log) || len(out.Command) != 1 || runs != 3 || out.Entry.Cause.Kind != "own" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	want := [][]string{
		{"restore", "--source=HEAD", "--staged", "--worktree", "--", "metasystem/out/conflict"},
		{"clean", "-f", "--", "metasystem/out/new"},
		{"restore", "--source=AUTO_MERGE", "--worktree", "--", "metasystem/out/conflict", "metasystem/out/other", "metasystem/out/new"},
		{"merge", "--abort"},
		{"clean", "-f", "--", "metasystem/out/new"},
		{"restore", "--source=HEAD", "--staged", "--worktree", "--", "metasystem/out/conflict", "metasystem/out/other", "metasystem/out/new"},
	}
	if !reflect.DeepEqual(b.writes, want) {
		t.Fatalf("failure cleanup=%v; want %v", b.writes, want)
	}
	b.record(out)
	data, readErr := os.ReadFile(out.Log)
	if readErr != nil || string(data) != "compile failed\n\nRegeneration on the tree before the merge:\n" {
		t.Fatalf("failure log=%q err=%v", data, readErr)
	}
}

func TestResolveRefusesCheckoutEditsWithoutChangingTheMerge(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"tracked", "untracked", "unknown merge snapshot", "unknown hand-in"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newResolveFixture(t)
			switch kind {
			case "tracked":
				b.dirty = "metasystem/src/input\x00"
			case "untracked":
				b.untracked = "metasystem/scratch\x00"
			case "unknown merge snapshot":
				git := b.seams.Git
				b.seams.Git = func(dir string, args ...string) (string, error) {
					if reflect.DeepEqual(args, []string{"diff", "--name-only", "-z", "AUTO_MERGE", "--"}) {
						return "", errors.New("no AUTO_MERGE")
					}
					return git(dir, args...)
				}
			case "unknown hand-in":
				if err := os.Remove(queuePath(b.install)); err != nil {
					t.Fatal(err)
				}
			}
			out, err := b.resolve()
			if err == nil || len(b.writes) != 0 || out.Entry != nil {
				t.Fatalf("refusal=%+v err=%v writes=%v", out, err, b.writes)
			}
			if kind == "tracked" || kind == "untracked" {
				if out.Outcome != "refused" || !strings.Contains(err.Error(), "changes outside the recorded merge") {
					t.Fatalf("out=%+v err=%v", out, err)
				}
				b.record(out)
			}
		})
	}
}

func TestResolveAlreadyResolvedHoldsWithoutRecords(t *testing.T) {
	t.Parallel()
	install := filepath.Join(t.TempDir(), "metasystem")
	out, err := Resolve(t.TempDir(), install, filepath.Dir(install), testpolicy.Contract{}, ResolveSeams{Git: func(_ string, args ...string) (string, error) {
		if !reflect.DeepEqual(args, []string{"diff", "--name-only", "--diff-filter=U", "-z"}) {
			t.Fatalf("repeat queried %v", args)
		}
		return "", nil
	}})
	if err != nil || !out.Held {
		t.Fatalf("repeat=%+v err=%v", out, err)
	}
	for _, path := range []string{regeneratePath(install), resultsPath(install), queuePath(install)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("repeat wrote %s: %v", path, err)
		}
	}
}

func TestRegenerationStatusShowsCommandLogGrowthAndDiedState(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	b.seams.Run = func(argv []string, _ string, log *os.File, started func(int64) error) error {
		if err := started(0); err != nil {
			t.Fatal(err)
		}
		for _, content := range []string{"first", "second"} {
			if _, err := log.WriteString(content); err != nil {
				t.Fatal(err)
			}
			running, err := ReadRunningRegeneration(b.install, ProveSeams{Alive: func(Running) bool { return true }})
			info, statErr := log.Stat()
			if err != nil || statErr != nil || running == nil || running.State != "running" || !reflect.DeepEqual(running.Command, argv) || running.LogBytes != info.Size() || running.LogBytes == 0 {
				t.Fatalf("running=%+v err=%v stat=%v", running, err, statErr)
			}
		}
		view := lane.View{Root: &b.checkout}
		status := readStatus(b.home, lane.Record{Root: b.checkout, Install: b.install}, view, ProveSeams{Alive: func(Running) bool { return false }}, laneGit{main: func() (string, error) { return "main", nil }, contains: func(string, string) (bool, error) { return false, nil }})
		if status.RunningRegeneration == nil || status.RunningRegeneration.State != "died" || len(status.Problems) != 0 {
			t.Fatalf("status=%+v", status)
		}
		return nil
	}
	out, err := b.resolve()
	if err != nil {
		t.Fatal(err)
	}
	b.record(out)
}

// A stopped generator leaves the hand-in waiting with an environment cause.
func TestRegenerationRefusesPausedLaneBeforeStartingCommand(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	if _, err := lane.SetPause(b.home, "Wido", bedNow); err != nil {
		t.Fatal(err)
	}
	out, err := b.resolve()
	if err == nil || !strings.Contains(err.Error(), "landing lane is stopped") || out.Entry == nil || out.Entry.State != StateWaiting || out.Outcome != "held" || out.Cause.Kind != "environment" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	b.record(out)
}

// Git's delete/change index and its open merge after restore determine whether
// resolution can continue; a stub that accepts restore cannot prove either.
func TestGitAdapterResolveRemovesGeneratedPathDeletedOnMain(t *testing.T) {
	t.Parallel()
	b := newResolveGitAdapterFixture(t, true)
	out, err := b.resolve()
	if err != nil || out.Held || out.Outcome != "resolved" || out.Exit != 0 || b.runs != 1 || !reflect.DeepEqual(out.Command, [][]string{b.contract.Generated[0].Command}) {
		t.Fatalf("resolve=%+v runs=%d err=%v", out, b.runs, err)
	}
	if _, err := os.Stat(filepath.Join(b.checkout, "metasystem/out/conflict")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("main's deleted output remains: %v", err)
	}
	if got := b.mustGit("show", ":metasystem/out/rebuilt"); got != "regenerated" {
		t.Fatalf("staged output=%q", got)
	}
	if got := b.mustGit("ls-files", "--unmerged"); got != "" {
		t.Fatalf("unmerged paths=%q", got)
	}
	b.record(out)
}

// A real restore clears every unmerged index stage while keeping MERGE_HEAD;
// a stub cannot prove that a restart regenerates this apparently resolved tree.
func TestGitAdapterResolveResumesAfterTakingMainBeforeStaging(t *testing.T) {
	t.Parallel()
	b := newResolveGitAdapterFixture(t, false)
	sha := b.mustGit("rev-parse", "--verify", "MERGE_HEAD^{commit}")
	data, err := json.Marshal(struct {
		SHA   string   `json:"sha"`
		Paths []string `json:"paths"`
	}{sha, []string{"metasystem/out/conflict"}})
	if err != nil {
		t.Fatal(err)
	}
	// Recreate a resolver that wrote its begun record, took main, then died.
	if err := os.WriteFile(filepath.Join(Dir(b.install), "resolve-begun.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	b.mustGit("restore", "--source=HEAD", "--staged", "--worktree", "--", "metasystem/out/conflict")
	if got := b.mustGit("ls-files", "--unmerged"); got != "" {
		t.Fatalf("restore left unmerged paths=%q", got)
	}
	out, err := b.resolve()
	if err != nil || out.Held || out.Outcome != "resolved" || b.runs != 1 {
		t.Fatalf("restart=%+v runs=%d err=%v", out, b.runs, err)
	}
	if got := b.mustGit("show", ":metasystem/out/conflict"); got != "regenerated" {
		t.Fatalf("staged output=%q; want regenerated", got)
	}
	b.record(out)
	if _, err := os.Stat(filepath.Join(Dir(b.install), "resolve-begun.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completed resolve keeps its begun record: %v", err)
	}
	before, err := os.ReadFile(regeneratePath(b.install))
	if err != nil {
		t.Fatal(err)
	}
	index := b.mustGit("write-tree")
	repeat, err := b.resolve()
	after, readErr := os.ReadFile(regeneratePath(b.install))
	if err != nil || readErr != nil || !repeat.Held || b.runs != 1 || !reflect.DeepEqual(before, after) || b.mustGit("write-tree") != index {
		t.Fatalf("repeat=%+v runs=%d err=%v read=%v", repeat, b.runs, err, readErr)
	}
}

// Git's restore clears the unmerged stages while leaving an open merge; a stub
// cannot prove that a refused resume still needs regeneration on its next run.
func TestGitAdapterResolveRetainsBegunRecordAfterRefusedResume(t *testing.T) {
	t.Parallel()
	b := newResolveGitAdapterFixture(t, false)
	data, err := json.Marshal(begunResolve{
		SHA: b.mustGit("rev-parse", "--verify", "MERGE_HEAD^{commit}"), Paths: []string{"metasystem/out/conflict"},
	})
	if err != nil {
		t.Fatal(err)
	}
	begun := resolveBegunPath(b.install)
	if err := os.WriteFile(begun, data, 0o600); err != nil {
		t.Fatal(err)
	}
	b.mustGit("restore", "--source=HEAD", "--staged", "--worktree", "--", "metasystem/out/conflict")
	if got := b.mustGit("ls-files", "--unmerged"); got != "" {
		t.Fatalf("restore left unmerged paths=%q", got)
	}
	stray := filepath.Join(b.checkout, "scratch")
	if err := os.WriteFile(stray, []byte("unrelated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refused, err := b.resolve()
	if err == nil || !strings.Contains(err.Error(), "changes outside the recorded merge") || refused.Outcome != "refused" || refused.Held || b.runs != 0 {
		t.Fatalf("refusal=%+v runs=%d err=%v", refused, b.runs, err)
	}
	b.record(refused)
	kept, err := os.ReadFile(begun)
	if err != nil || !reflect.DeepEqual(kept, data) {
		t.Fatalf("refused resume lost or changed its begun record: %v", err)
	}
	if err := os.Remove(stray); err != nil {
		t.Fatal(err)
	}
	out, err := b.resolve()
	if err != nil || out.Held || out.Outcome != "resolved" || b.runs != 1 {
		t.Fatalf("restart=%+v runs=%d err=%v", out, b.runs, err)
	}
	if got := b.mustGit("show", ":metasystem/out/conflict"); got != "regenerated" {
		t.Fatalf("staged output=%q; want regenerated", got)
	}
	if _, err := os.Stat(begun); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completed resolve keeps its begun record: %v", err)
	}
	rows, err := readLines[Regeneration](regeneratePath(b.install))
	if err != nil || len(rows) != 2 || !reflect.DeepEqual(rows[1], out.Regeneration) {
		t.Fatalf("regeneration rows=%+v err=%v", rows, err)
	}
	before, err := os.ReadFile(regeneratePath(b.install))
	if err != nil {
		t.Fatal(err)
	}
	index := b.mustGit("write-tree")
	repeat, err := b.resolve()
	after, readErr := os.ReadFile(regeneratePath(b.install))
	if err != nil || readErr != nil || !repeat.Held || b.runs != 1 || !reflect.DeepEqual(before, after) || b.mustGit("write-tree") != index {
		t.Fatalf("repeat=%+v runs=%d err=%v read=%v", repeat, b.runs, err, readErr)
	}
}

type resolveGitAdapterFixture struct {
	*resolveFixture
	runs int
}

func newResolveGitAdapterFixture(t *testing.T, deleted bool) *resolveGitAdapterFixture {
	t.Helper()
	b := &resolveGitAdapterFixture{resolveFixture: newResolveFixture(t)}
	b.seams.Git = func(dir string, args ...string) (string, error) {
		command := exec.Command("/usr/bin/git", append([]string{"-C", dir, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		output, err := command.CombinedOutput()
		return strings.TrimSuffix(string(output), "\n"), err
	}
	b.mustGit("init", "--quiet", "-b", "main")
	output := filepath.Join(b.install, "out", "conflict")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.checkout, ".gitignore"), []byte("metasystem/artifacts/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(data string) {
		t.Helper()
		if err := os.WriteFile(output, []byte(data+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("base")
	b.mustGit("add", ".")
	b.mustGit("commit", "--quiet", "-m", "base")
	b.mustGit("checkout", "--quiet", "-b", "goal")
	write("goal")
	b.mustGit("commit", "--quiet", "-am", "goal")
	sha := b.mustGit("rev-parse", "HEAD")
	b.mustGit("checkout", "--quiet", "main")
	if deleted {
		b.mustGit("rm", "--", "metasystem/out/conflict")
	} else {
		write("main")
	}
	b.mustGit("commit", "--quiet", "-am", "main")
	if output, err := b.seams.Git(b.checkout, "merge", "--no-commit", "goal"); err == nil {
		t.Fatalf("expected a conflicted merge: %s", output)
	}
	if _, _, err := HandIn(b.install, Line{Goal: "goal", SHA: sha, Branch: "goal/goal"}); err != nil {
		t.Fatal(err)
	}
	b.contract.Generated = []testpolicy.Generated{{Paths: []string{"out/**"}, Command: []string{"compile"}}}
	b.seams.Run = func(argv []string, dir string, log *os.File, started func(int64) error) error {
		if !reflect.DeepEqual(argv, b.contract.Generated[0].Command) || dir != b.install {
			t.Fatalf("command=%v cwd=%s", argv, dir)
		}
		b.runs++
		if err := started(0); err != nil {
			return err
		}
		if deleted {
			if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("regeneration started before removing main's deleted path: %v", err)
			}
			output = filepath.Join(b.install, "out", "rebuilt")
		}
		return os.WriteFile(output, []byte("regenerated\n"), 0o644)
	}
	return b
}

func (b *resolveGitAdapterFixture) mustGit(args ...string) string {
	b.t.Helper()
	out, err := b.seams.Git(b.checkout, args...)
	if err != nil {
		b.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return out
}
