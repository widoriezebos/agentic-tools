package gittree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// headOID reads the fixture's current HEAD commit id via raw git.
func (f *treeFixture) headOID() string {
	f.t.Helper()
	return f.git("rev-parse", "HEAD")
}

// A planted replace mapping must never re-route what any tree or commit
// comparison sees: the wall judges objects, not a replacement's view.
func TestReplaceRefCannotAlterAccounting(t *testing.T) {
	f := newTreeFixture(t)
	baseTree, err := f.w.TreeOf("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	base := f.headOID()
	f.write("smuggled.txt", "other bytes\n")
	f.git("add", "smuggled.txt")
	f.commit("second")
	other := f.headOID()
	f.git("replace", base, other)
	replacedTree, err := f.w.TreeOf(base)
	if err != nil {
		t.Fatal(err)
	}
	if replacedTree != baseTree {
		t.Fatalf("replace ref altered TreeOf: %s != %s", replacedTree, baseTree)
	}
}

// An inherited GIT_DIR must not steer any probe to another repository:
// the checkout judged is the one containing the invocation directory.
func TestGitDirRedirectCannotSteerProbes(t *testing.T) {
	f := newTreeFixture(t)
	want := f.headOID()
	other := newTreeFixture(t)
	t.Setenv("GIT_DIR", filepath.Join(other.w.Dir, ".git"))
	t.Setenv("GIT_WORK_TREE", other.w.Dir)
	oid, unborn, err := f.w.HeadCommit()
	if err != nil || unborn {
		t.Fatalf("HeadCommit under GIT_DIR redirect: %v unborn=%v", err, unborn)
	}
	if oid != want {
		t.Fatalf("HeadCommit followed GIT_DIR to a foreign repository: %s != %s", oid, want)
	}
	if tree, err := f.w.Snapshot("HEAD"); err != nil {
		t.Fatal(err)
	} else if got, _ := f.w.TreeOf("HEAD"); got != tree {
		t.Fatalf("snapshot and TreeOf disagree under GIT_DIR redirect")
	}
}

// A replacement namespace relocated by GIT_REPLACE_REF_BASE is part of
// the same steering set: the scrub plus the config pin keep accounting
// on the real objects.
func TestReplaceBaseRedirectScrubbed(t *testing.T) {
	f := newTreeFixture(t)
	baseTree, _ := f.w.TreeOf("HEAD")
	base := f.headOID()
	f.write("x.txt", "x\n")
	f.git("add", "x.txt")
	f.commit("second")
	other := f.headOID()
	t.Setenv("GIT_REPLACE_REF_BASE", "refs/hidden/")
	f.git("update-ref", "refs/hidden/"+base, other)
	got, err := f.w.TreeOf(base)
	if err != nil {
		t.Fatal(err)
	}
	if got != baseTree {
		t.Fatalf("relocated replace namespace altered accounting")
	}
}

func TestHeadCommitUnbornAnswers(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "-C", dir, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	w := Workspace{Dir: dir}
	oid, unborn, err := w.HeadCommit()
	if err != nil {
		t.Fatalf("unborn HEAD must be an answer, not an error: %v", err)
	}
	if !unborn || oid != "" {
		t.Fatalf("expected unborn answer, got oid=%q unborn=%v", oid, unborn)
	}
}

func TestSymbolicHeadDetachedAnswers(t *testing.T) {
	f := newTreeFixture(t)
	ref, detached, err := f.w.SymbolicHead()
	if err != nil || detached || ref != "refs/heads/main" {
		t.Fatalf("attached head: ref=%q detached=%v err=%v", ref, detached, err)
	}
	f.git("checkout", "-q", "--detach")
	_, detached, err = f.w.SymbolicHead()
	if err != nil || !detached {
		t.Fatalf("detached head must answer detached: %v %v", detached, err)
	}
}

func TestRefMapCoversEveryNamespace(t *testing.T) {
	f := newTreeFixture(t)
	head := f.headOID()
	f.git("tag", "keeper")
	f.git("update-ref", "refs/custom/hideout", head)
	refs, err := f.w.RefMap()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"refs/heads/main", "refs/tags/keeper", "refs/custom/hideout"} {
		if refs[name] != head {
			t.Fatalf("ref map misses %s: %v", name, refs)
		}
	}
}

// The staged projection reconstructs from logical entries: the staged
// content appears, the real index and worktree stay untouched.
func TestStagedTreeReconstructsWithoutMutatingIndex(t *testing.T) {
	f := newTreeFixture(t)
	f.write("staged.txt", "staged\n")
	f.git("add", "staged.txt")
	before := f.git("status", "--porcelain")
	tree, err := f.w.StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	if entry, ok := f.entry(tree, "staged.txt"); !ok || entry.Mode != "100644" {
		t.Fatalf("staged projection misses staged.txt: %v %v", entry, ok)
	}
	if after := f.git("status", "--porcelain"); after != before {
		t.Fatalf("staged projection mutated the real index:\n%s\n---\n%s", before, after)
	}
}

// A clean index projects exactly HEAD's tree.
func TestStagedTreeCleanEqualsHead(t *testing.T) {
	f := newTreeFixture(t)
	tree, err := f.w.StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	head, _ := f.w.HeadTree()
	if tree != head {
		t.Fatalf("clean staged projection %s != HEAD tree %s", tree, head)
	}
}

// conflictedIndexEntry plants an unmerged entry directly in the real
// index — the logical shape a merge conflict leaves.
func conflictedIndexEntry(f *treeFixture, dir, path string) {
	f.t.Helper()
	cmd := exec.Command("git", "-C", dir, "update-index", "--index-info")
	oid := f.git("rev-parse", "HEAD:README.md")
	cmd.Stdin = strings.NewReader(
		"100644 " + oid + " 1\t" + path + "\n" +
			"100644 " + oid + " 2\t" + path + "\n" +
			"100644 " + oid + " 3\t" + path + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		f.t.Fatalf("update-index: %v %s", err, out)
	}
}

func TestStagedTreeConflictedWorkspaceRefuses(t *testing.T) {
	f := newTreeFixture(t)
	conflictedIndexEntry(f, f.w.Dir, "clash.txt")
	if _, err := f.w.StagedTree(); !errors.Is(err, ErrUnmergedWorkspaceIndex) {
		t.Fatalf("conflicted workspace entry must refuse toward the wall, got %v", err)
	}
}

// nestedFixture is a toplevel repository with the workspace one level
// down — the supported nested checkout shape.
func nestedFixture(t *testing.T) (*treeFixture, Workspace) {
	t.Helper()
	f := newTreeFixture(t)
	f.write("ws/app.txt", "app\n")
	f.write("sibling/lib.txt", "lib\n")
	f.git("add", ".")
	f.commit("nested layout")
	return f, Workspace{Dir: filepath.Join(f.w.Dir, "ws")}
}

// A preexisting sibling conflict never enters the workspace projection
// and never refuses it.
func TestStagedTreeSiblingConflictNotBlamed(t *testing.T) {
	f, ws := nestedFixture(t)
	conflictedIndexEntry(f, f.w.Dir, "sibling/clash.txt")
	tree, err := ws.StagedTree()
	if err != nil {
		t.Fatalf("sibling conflict refused the workspace projection: %v", err)
	}
	want, err := ws.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	if tree != want {
		t.Fatalf("workspace staged projection drifted: %s != %s", tree, want)
	}
}

// The toplevel posture represents sibling conflicts instead of refusing,
// and touching a file's stat cache never changes it.
func TestTopStagedPostureRepresentsConflictsAndIgnoresStat(t *testing.T) {
	f, ws := nestedFixture(t)
	conflictedIndexEntry(f, f.w.Dir, "sibling/clash.txt")
	posture, err := ws.TopStagedPosture()
	if err != nil {
		t.Fatal(err)
	}
	if len(posture.Unmerged) != 3 {
		t.Fatalf("expected 3 unmerged stage entries, got %v", posture.Unmerged)
	}
	// Churn the stat cache without changing any staged byte.
	path := filepath.Join(f.w.Dir, "ws", "app.txt")
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	f.git("status", "--porcelain")
	again, err := ws.TopStagedPosture()
	if err != nil {
		t.Fatal(err)
	}
	if !posture.Equal(again) {
		t.Fatalf("stat refresh changed the staged posture")
	}
}

// Seeding from the comparison target: a tracked-and-ignored path present
// in the expected tree projects even when HEAD does not carry it yet.
func TestSnapshotSeededFollowsExpectedMembership(t *testing.T) {
	f := newTreeFixture(t)
	f.write(".gitignore", "generated.txt\n")
	f.git("add", ".gitignore")
	f.commit("ignore generated")
	head := f.headOID()
	// The expected tree carries the ignored-but-tracked file; HEAD does
	// not. Build it via an isolated snapshot with the file force-added.
	f.write("generated.txt", "expected bytes\n")
	f.git("add", "-f", "generated.txt")
	expected, err := f.w.StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	f.git("reset", "-q", "generated.txt")
	// A HEAD-seeded snapshot cannot see the ignored file — the frozen
	// seed false rejection this primitive kills.
	plain, err := f.w.Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.entry(plain, "generated.txt"); ok {
		t.Fatalf("HEAD-seeded snapshot unexpectedly projects the ignored file")
	}
	seeded, err := f.w.SnapshotSeeded(head, expected, nil)
	if err != nil {
		t.Fatal(err)
	}
	if seeded != expected {
		t.Fatalf("seeded snapshot %s does not reach the expected tree %s", seeded, expected)
	}
}

// A declared ignored artifact the expected tree cannot name joins by
// forced membership.
func TestSnapshotSeededForcesDeclaredPaths(t *testing.T) {
	f := newTreeFixture(t)
	f.write(".gitignore", "artifact.bin\n")
	f.git("add", ".gitignore")
	f.commit("ignore artifact")
	head := f.headOID()
	expected, err := f.w.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	f.write("artifact.bin", "produced\n")
	without, err := f.w.SnapshotSeeded(head, expected, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.entry(without, "artifact.bin"); ok {
		t.Fatalf("undeclared ignored artifact must stay outside the projection")
	}
	with, err := f.w.SnapshotSeeded(head, expected, []string{"artifact.bin"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.entry(with, "artifact.bin"); !ok {
		t.Fatalf("declared ignored artifact vanished from the projection")
	}
	// An absent declared path is simply no delta, never an error.
	if _, err := f.w.SnapshotSeeded(head, expected, []string{"never-produced.bin"}); err != nil {
		t.Fatalf("absent declared path errored: %v", err)
	}
}

// The nested graft: the projection is judged in workspace path space
// against the expected workspace tree, with sibling entries seeded from
// the resolved commit.
func TestSnapshotSeededNestedGraft(t *testing.T) {
	f, ws := nestedFixture(t)
	head := f.git("rev-parse", "HEAD")
	expected, err := ws.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	seeded, err := ws.SnapshotSeeded(head, expected, nil)
	if err != nil {
		t.Fatal(err)
	}
	if seeded != expected {
		t.Fatalf("clean nested seeded snapshot %s != expected %s", seeded, expected)
	}
	f.write("ws/app.txt", "changed\n")
	moved, err := ws.SnapshotSeeded(head, expected, nil)
	if err != nil {
		t.Fatal(err)
	}
	if moved == expected {
		t.Fatalf("a workspace edit must move the seeded projection")
	}
}

func TestAnchorCommitCASAndGrammar(t *testing.T) {
	f := newTreeFixture(t)
	first := f.headOID()
	if err := f.w.AnchorCommit("m1", "turn-open-head", first); err != nil {
		t.Fatal(err)
	}
	f.write("next.txt", "next\n")
	f.git("add", "next.txt")
	f.commit("second")
	second := f.headOID()
	if err := f.w.AnchorCommit("m1", "turn-open-head", second); err != nil {
		t.Fatalf("CAS update to the new open commit: %v", err)
	}
	if got := f.git("rev-parse", "refs/metasystem/missions/m1/turn-open-head"); got != second {
		t.Fatalf("anchor ref %s != %s", got, second)
	}
	tree, _ := f.w.HeadTree()
	if err := f.w.AnchorCommit("m1", "turn-open-head", tree); err == nil {
		t.Fatalf("a tree object must refuse the commit anchor")
	}
	if err := f.w.AnchorCommit("m1", "Bad/Name", second); err == nil {
		t.Fatalf("a slashed name must refuse")
	}
	if err := f.w.DropAnchors("m1"); err != nil {
		t.Fatal(err)
	}
	if _, code := runIn(t, f.w.Dir, "rev-parse", "--verify", "-q", "refs/metasystem/missions/m1/turn-open-head"); code == 0 {
		t.Fatalf("DropAnchors left the commit anchor behind")
	}
}

// runIn is a raw git helper that reports the exit code.
func runIn(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return string(out), exit.ExitCode()
		}
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out), 0
}

func TestPseudorefCensusParsesTheFamily(t *testing.T) {
	f := newTreeFixture(t)
	head := f.headOID()
	gitDir := strings.TrimSpace(f.git("rev-parse", "--absolute-git-dir"))
	writeFile := func(name, content string) {
		if err := os.WriteFile(filepath.Join(gitDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("ORIG_HEAD", head+"\n")
	writeFile("FETCH_HEAD", head+"\t\tbranch 'main' of example\n"+head+"\tnot-for-merge\tbranch 'dev' of example\n")
	writeFile("REBASE_HEAD", "not an oid\n")
	census, err := f.w.PseudorefCensus()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Pseudoref{}
	for _, entry := range census {
		byName[entry.Name] = entry
	}
	if got := byName["ORIG_HEAD"]; len(got.OIDs) != 1 || got.OIDs[0] != head || !got.Parseable {
		t.Fatalf("ORIG_HEAD census: %+v", got)
	}
	if got := byName["FETCH_HEAD"]; len(got.OIDs) != 2 || !got.Parseable {
		t.Fatalf("FETCH_HEAD multi-OID census: %+v", got)
	}
	if got := byName["REBASE_HEAD"]; got.Parseable {
		t.Fatalf("unparseable pseudoref must answer Parseable=false: %+v", got)
	}
}

// A linked worktree is a carrier with its own private pseudorefs and
// staged posture, distinct from the main checkout's.
func TestWorktreeCensusRecordsPrivatePosture(t *testing.T) {
	f := newTreeFixture(t)
	head := f.headOID()
	linked := filepath.Join(t.TempDir(), "linked")
	f.git("worktree", "add", "--detach", "-q", linked, head)
	// Private ORIG_HEAD in the LINKED worktree only.
	linkedGitDir := strings.TrimSpace(func() string {
		out, code := runIn(t, linked, "rev-parse", "--absolute-git-dir")
		if code != 0 {
			t.Fatalf("linked git dir: %s", out)
		}
		return out
	}())
	if err := os.WriteFile(filepath.Join(linkedGitDir, "ORIG_HEAD"), []byte(head+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	census, err := f.w.WorktreeCensus()
	if err != nil {
		t.Fatal(err)
	}
	if len(census) != 2 {
		t.Fatalf("expected 2 worktrees, got %+v", census)
	}
	resolvedLinked, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	var main, link *WorktreeRecord
	for i := range census {
		if resolved, rerr := filepath.EvalSymlinks(census[i].Path); rerr == nil && resolved == resolvedLinked {
			link = &census[i]
		} else {
			main = &census[i]
		}
	}
	if link == nil || main == nil {
		t.Fatalf("census misses a worktree: %+v", census)
	}
	if !link.Detached || link.HeadOID != head || !link.PostureReadable {
		t.Fatalf("linked worktree posture: %+v", link)
	}
	linkHasOrig := false
	for _, ref := range link.Pseudorefs {
		if ref.Name == "ORIG_HEAD" {
			linkHasOrig = true
		}
	}
	if !linkHasOrig {
		t.Fatalf("linked worktree's private ORIG_HEAD missed: %+v", link.Pseudorefs)
	}
	for _, ref := range main.Pseudorefs {
		if ref.Name == "ORIG_HEAD" {
			t.Fatalf("main checkout inherited the linked worktree's private ORIG_HEAD")
		}
	}
	headTree := f.git("rev-parse", "HEAD^{tree}")
	if main.Staged.Tree != headTree || link.Staged.Tree != headTree {
		t.Fatalf("clean staged postures must equal HEAD's tree: main=%s link=%s want=%s",
			main.Staged.Tree, link.Staged.Tree, headTree)
	}
}

// WSS-10 / WSS-I5-4: ScrubbedEnviron strips git's config-injection
// channels so an inherited GIT_CONFIG_PARAMETERS / GIT_CONFIG_COUNT
// cannot re-enable object replacement past the runner's own pin.
func TestScrubbedEnvironStripsConfigInjection(t *testing.T) {
	t.Setenv("GIT_CONFIG_PARAMETERS", "'core.useReplaceRefs=true'")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.useReplaceRefs")
	t.Setenv("GIT_CONFIG_VALUE_0", "true")
	t.Setenv("GIT_DIR", "/somewhere/else/.git")
	env := ScrubbedEnviron("GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.useReplaceRefs", "GIT_CONFIG_VALUE_0=false")
	seen := map[string]string{}
	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")
		seen[name] = value
	}
	if _, ok := seen["GIT_CONFIG_PARAMETERS"]; ok {
		t.Fatal("GIT_CONFIG_PARAMETERS must be stripped")
	}
	if _, ok := seen["GIT_DIR"]; ok {
		t.Fatal("GIT_DIR must be stripped")
	}
	// Only the runner's own pin survives (appended after the scrub).
	if seen["GIT_CONFIG_VALUE_0"] != "false" {
		t.Fatalf("the runner pin must win: GIT_CONFIG_VALUE_0=%q", seen["GIT_CONFIG_VALUE_0"])
	}
}
