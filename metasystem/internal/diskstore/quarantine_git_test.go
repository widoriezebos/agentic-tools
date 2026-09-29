package diskstore

// U5f (engine-owns-disk-lifetimes Part B, 3.7, R12, DL2-18, DL3B-09): the
// real-git claim of the quarantine absorb. Git's own object formats are the
// claim: loose objects inflated and hashed, packs imported by index-pack
// --strict and read back by verify-pack; no stub can prove them.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// quarantineBed is a common repository with a linked delegate worktree
// whose objects live in a quarantine the common store borrows through its
// alternates: one commit packed, one loose.
type quarantineBed struct {
	repo, worktree, common, quarantine string
	packed, loose                      string
}

func newQuarantineBed(t *testing.T) quarantineBed {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	ctx := context.Background()
	root := realDir(t)
	bed := quarantineBed{repo: filepath.Join(root, "repo"), worktree: filepath.Join(root, "repo", "artifacts", "agents", "worktrees", "j1")}
	must := func(env []string, dir string, args ...string) string {
		t.Helper()
		command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
		command.Env = append(append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com"), env...)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if err := os.MkdirAll(bed.repo, 0o700); err != nil {
		t.Fatal(err)
	}
	must(nil, bed.repo, "init", "-q", "-b", "main")
	writeBedFile(t, filepath.Join(bed.repo, "README"), []byte("base\n"))
	must(nil, bed.repo, "add", "README")
	must(nil, bed.repo, "commit", "-q", "-m", "base")
	must(nil, bed.repo, "worktree", "add", "-q", "-b", "agent/j1", bed.worktree, "HEAD")
	gitdir := must(nil, bed.worktree, "rev-parse", "--absolute-git-dir")
	bed.common = filepath.Join(bed.repo, ".git", "objects")
	bed.quarantine = filepath.Join(gitdir, QuarantineName)
	if err := os.MkdirAll(bed.quarantine, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := AddAlternate(bed.common, bed.quarantine); err != nil {
		t.Fatal(err)
	}
	env := []string{"GIT_OBJECT_DIRECTORY=" + bed.quarantine, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + bed.common}
	writeBedFile(t, filepath.Join(bed.worktree, "one.txt"), []byte("the delegate's first change\n"))
	must(env, bed.worktree, "add", "one.txt")
	must(env, bed.worktree, "commit", "-q", "-m", "one")
	bed.packed = must(env, bed.worktree, "rev-parse", "HEAD")
	must(env, bed.worktree, "repack", "-a", "-d", "-l", "-q")
	writeBedFile(t, filepath.Join(bed.worktree, "two.txt"), []byte("the delegate's second change\n"))
	must(env, bed.worktree, "add", "two.txt")
	must(env, bed.worktree, "commit", "-q", "-m", "two")
	bed.loose = must(env, bed.worktree, "rev-parse", "HEAD")
	return bed
}

func (bed quarantineBed) request(stage string) AbsorbRequest {
	return AbsorbRequest{Git: realWorkspaceGit, GitRoot: bed.repo, CommonObjects: bed.common, Quarantine: bed.quarantine, Stage: stage}
}

// commonHolds asks the common store, with the alternates file set aside,
// whether it holds commit and its whole closure.
func (bed quarantineBed) commonHolds(t *testing.T, commit string) bool {
	t.Helper()
	command := exec.Command("git", "-C", bed.repo, "rev-list", "--objects", commit)
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	return command.Run() == nil
}

func (bed quarantineBed) alternates(t *testing.T) []string {
	t.Helper()
	lines, err := readAlternates(bed.common)
	if err != nil {
		t.Fatal(err)
	}
	return lines
}

func TestTheAbsorbPublishesEveryQuarantineObjectVerifiedByContent(t *testing.T) {
	t.Parallel()
	bed := newQuarantineBed(t)
	result, err := AbsorbQuarantine(context.Background(), bed.request("01K2Z7Q3M8XW1V0P9D4J6S5R2T"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Loose == 0 || result.Packed == 0 {
		t.Fatalf("both a loose and a packed object were absorbed: %+v", result)
	}
	if lines := bed.alternates(t); len(lines) != 0 {
		t.Fatalf("the quarantine's alternates line goes after the absorb: %q", lines)
	}
	// The quarantine goes; every object stays readable from the common store.
	if err := RemoveTree(context.Background(), bed.quarantine); err != nil {
		t.Fatal(err)
	}
	for _, commit := range []string{bed.packed, bed.loose} {
		if !bed.commonHolds(t, commit) {
			t.Fatalf("%s is readable from the common store with the quarantine gone", commit)
		}
	}
	// A repeat is success and changes nothing.
	if _, err := AbsorbQuarantine(context.Background(), bed.request("01K2Z7Q3M8XW1V0P9D4J6S5R2V")); err != nil {
		t.Fatal(err)
	}
}

func TestAnAbsorbThatOmitsAnObjectFailsWhileTheAlternatesStay(t *testing.T) {
	t.Parallel()
	for name, omit := range map[string]func(id string) bool{
		"a loose object": func(id string) bool { return len(id) == 40 || len(id) == 64 },
		"a pack":         func(id string) bool { return strings.HasSuffix(id, ".idx") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newQuarantineBed(t)
			request := bed.request("01K2Z7Q3M8XW1V0P9D4J6S5R2T")
			omitted := false
			request.skip = func(id string) bool {
				if !omitted && omit(id) {
					omitted = true
					return true
				}
				return false
			}
			if _, err := AbsorbQuarantine(context.Background(), request); err == nil {
				t.Fatal("verification finds the omitted object")
			}
			if lines := bed.alternates(t); len(lines) != 1 || lines[0] != bed.quarantine {
				t.Fatalf("the alternates line stays: %q", lines)
			}
		})
	}
}

func TestAWrongLooseDestinationIsNeverTakenForTheObject(t *testing.T) {
	t.Parallel()
	bed := newQuarantineBed(t)
	objects, err := filepath.Glob(filepath.Join(bed.quarantine, "[0-9a-f][0-9a-f]", "*"))
	if err != nil || len(objects) == 0 {
		t.Fatalf("the bed has loose objects: %v", err)
	}
	source := objects[0]
	destination := filepath.Join(bed.common, filepath.Base(filepath.Dir(source)), filepath.Base(source))
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	wrong := make([]byte, len(data))
	copy(wrong, data)
	wrong[len(wrong)-1] ^= 0xff
	writeBedFile(t, destination, wrong)
	if _, err := AbsorbQuarantine(context.Background(), bed.request("01K2Z7Q3M8XW1V0P9D4J6S5R2T")); err == nil {
		t.Fatal("a same-name, same-size destination with other bytes fails")
	}
	if got, _ := os.ReadFile(destination); string(got) != string(wrong) {
		t.Fatal("the differing destination is kept, never overwritten")
	}
	if lines := bed.alternates(t); len(lines) != 1 {
		t.Fatalf("the alternates line stays: %q", lines)
	}
}

func TestABrokenDestinationPackIsReimported(t *testing.T) {
	t.Parallel()
	for name, breakPair := range map[string]func(t *testing.T, pack, index string){
		"an index whose pack is missing": func(t *testing.T, pack, index string) {
			if err := os.Remove(pack); err != nil {
				t.Fatal(err)
			}
		},
		"an index whose pack is truncated": func(t *testing.T, pack, index string) {
			if err := os.Truncate(pack, 20); err != nil {
				t.Fatal(err)
			}
		},
		"a leftover partial": func(t *testing.T, pack, index string) {
			writeBedFile(t, filepath.Join(filepath.Dir(pack), "tmp_"+strings.TrimSuffix(filepath.Base(pack), ".pack")+".partial-01K2Z7Q3M8XW1V0P9D4J6S5R2A.pack"), []byte("junk"))
			if err := os.Remove(pack); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newQuarantineBed(t)
			indexes, _ := filepath.Glob(filepath.Join(bed.quarantine, "pack", "*.idx"))
			if len(indexes) != 1 {
				t.Fatalf("the bed has one pack: %q", indexes)
			}
			index := filepath.Join(bed.common, "pack", filepath.Base(indexes[0]))
			pack := strings.TrimSuffix(index, ".idx") + ".pack"
			for _, pair := range [][2]string{{indexes[0], index}, {strings.TrimSuffix(indexes[0], ".idx") + ".pack", pack}} {
				data, err := os.ReadFile(pair[0])
				if err != nil {
					t.Fatal(err)
				}
				writeBedFile(t, pair[1], data)
			}
			breakPair(t, pack, index)
			ids, err := indexIDs(index, 20)
			if err != nil {
				t.Fatal(err)
			}
			if covered(context.Background(), realWorkspaceGit, bed.repo, index, ids) == nil {
				t.Fatal("a broken destination pair never verifies")
			}
			if _, err := AbsorbQuarantine(context.Background(), bed.request("01K2Z7Q3M8XW1V0P9D4J6S5R2T")); err != nil {
				t.Fatalf("the pair is imported afresh and passes: %v", err)
			}
			if covered(context.Background(), realWorkspaceGit, bed.repo, index, ids) != nil {
				t.Fatal("the re-imported pair verifies")
			}
		})
	}
}

func TestConcurrentAlternatesEditsKeepBothLines(t *testing.T) {
	t.Parallel()
	objects := filepath.Join(realDir(t), "objects")
	if err := AddAlternate(objects, "/q/one"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	go func() { done <- AddAlternate(objects, "/q/two") }()
	go func() { done <- RemoveAlternate(objects, "/q/one") }()
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	lines, err := readAlternates(objects)
	if err != nil || len(lines) != 1 || lines[0] != "/q/two" {
		t.Fatalf("an add and a remove racing leave exactly the added line: %q %v", lines, err)
	}
}
