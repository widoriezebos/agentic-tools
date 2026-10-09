package plain

import (
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	for _, path := range []string{regeneratePath(b.install), resultsPath(b.install), regenerationRunningPath(b.install)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			b.t.Fatalf("conflict return wrote %s: %v", path, err)
		}
	}
}

func TestResolveGeneratedAbortsAndReturnsWithoutRegeneration(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	out, err := b.resolve()
	if err != nil || out.Outcome != "returned" || out.Exit != 0 || out.Entry == nil || out.Entry.State != StateReturned || out.Held || len(out.Command) != 0 || out.Conflict.Paths[0].Class != conflict.Generated || !strings.Contains(out.Reason, "metasystem/out/conflict (generated)") || !strings.Contains(out.Reason, "work rebase goal") {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if !reflect.DeepEqual(b.writes, [][]string{{"merge", "--abort"}}) {
		t.Fatalf("Git writes=%v", b.writes)
	}
	b.record(out)
	b.paths = ""
	second, err := b.resolve()
	if err != nil || !second.Held || len(b.writes) != 1 {
		t.Fatalf("repeat=%+v err=%v writes=%v", second, err, b.writes)
	}
	b.record(second)
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

func TestResolveIgnoresObsoleteBegunRecordBeforeReturning(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	begun := resolveBegunPath(b.install)
	data := []byte("obsolete, undecodable record")
	if err := os.WriteFile(begun, data, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := b.resolve()
	if err != nil || out.Entry == nil || out.Entry.State != StateReturned {
		t.Fatalf("return=%+v err=%v", out, err)
	}
	kept, err := os.ReadFile(begun)
	if err != nil || !reflect.DeepEqual(kept, data) {
		t.Fatalf("begun record changed: %q %v", kept, err)
	}
	b.record(out)
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
		status := readStatus(b.home, lane.Record{Root: b.checkout, Install: b.install}, view, ProveSeams{Alive: func(Running) bool { return false }, Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }}, laneGit{main: func() (string, error) { return "main", nil }, contains: func(string, string) (bool, error) { return false, nil }})
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

// Conflict returns never start a generator, including when generators are paused.
func TestResolveReturnsConflictWithoutStartingPausedGenerator(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	if _, err := lane.SetPause(b.home, "Wido", bedNow); err != nil {
		t.Fatal(err)
	}
	out, err := b.resolve()
	if err != nil || out.Outcome != "returned" || out.Entry == nil || out.Entry.State != StateReturned || len(out.Command) != 0 {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	b.record(out)
}

// Real Git proves that returning a generated conflict clears its merge and index.
func TestGitAdapterResolveReturnsGeneratedConflictDeletedOnMain(t *testing.T) {
	t.Parallel()
	b := newResolveGitAdapterFixture(t, true)
	out, err := b.resolve()
	if err != nil || out.Held || out.Outcome != "returned" || out.Exit != 0 || b.runs != 0 || len(out.Command) != 0 {
		t.Fatalf("resolve=%+v runs=%d err=%v", out, b.runs, err)
	}
	if _, err := os.Stat(filepath.Join(b.checkout, "metasystem/out/conflict")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("main's deleted output remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(b.checkout, ".git/MERGE_HEAD")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("merge remains: %v", err)
	}
	if got := b.mustGit("status", "--porcelain"); got != "" {
		t.Fatalf("checkout dirty: %s", got)
	}
	b.record(out)
}

// A staged merge has no conflict to return, even if an obsolete record remains.
func TestGitAdapterResolveIgnoresBegunRecordOnStagedMerge(t *testing.T) {
	t.Parallel()
	b := newResolveGitAdapterFixture(t, false)
	begun := resolveBegunPath(b.install)
	data := []byte("obsolete record")
	if err := os.WriteFile(begun, data, 0600); err != nil {
		t.Fatal(err)
	}
	b.mustGit("restore", "--source=HEAD", "--staged", "--worktree", "--", "metasystem/out/conflict")
	index := b.mustGit("write-tree")
	for range 2 {
		out, err := b.resolve()
		if err != nil || !out.Held || b.runs != 0 || b.mustGit("write-tree") != index {
			t.Fatalf("resolve=%+v runs=%d err=%v", out, b.runs, err)
		}
		b.record(out)
	}
	kept, err := os.ReadFile(begun)
	if err != nil || !reflect.DeepEqual(kept, data) {
		t.Fatalf("begun record changed: %q %v", kept, err)
	}
}

// Refusing checkout edits preserves the merge and the obsolete record for inspection.
func TestGitAdapterResolveKeepsCheckoutEditsAndBegunRecord(t *testing.T) {
	t.Parallel()
	b := newResolveGitAdapterFixture(t, false)
	begun := resolveBegunPath(b.install)
	data := []byte("obsolete record")
	if err := os.WriteFile(begun, data, 0600); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(b.checkout, "scratch")
	if err := os.WriteFile(stray, []byte("unrelated\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := b.resolve()
	if err == nil || out.Outcome != "refused" || b.runs != 0 {
		t.Fatalf("refusal=%+v runs=%d err=%v", out, b.runs, err)
	}
	if _, err := os.Stat(filepath.Join(b.checkout, ".git/MERGE_HEAD")); err != nil {
		t.Fatalf("refusal aborted merge: %v", err)
	}
	kept, err := os.ReadFile(begun)
	if err != nil || !reflect.DeepEqual(kept, data) {
		t.Fatalf("begun record changed: %q %v", kept, err)
	}
	b.record(out)
	if err := os.Remove(stray); err != nil {
		t.Fatal(err)
	}
	out, err = b.resolve()
	if err != nil || out.Outcome != "returned" || b.runs != 0 {
		t.Fatalf("return=%+v runs=%d err=%v", out, b.runs, err)
	}
	b.record(out)
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
	b.mustGit("update-ref", "refs/remotes/origin/main", "HEAD")
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
