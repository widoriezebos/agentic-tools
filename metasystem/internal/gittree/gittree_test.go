package gittree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// One committed repository per test; every projection claim runs against
// real git, exactly as the validator and the runner invoke it.
type treeFixture struct {
	t *testing.T
	w Workspace
}

func newTreeFixture(t *testing.T) *treeFixture {
	t.Helper()
	f := &treeFixture{t: t, w: Workspace{Dir: t.TempDir()}}
	f.git("init", "-q", "-b", "main")
	f.write("README.md", "fixture\n")
	f.git("add", ".")
	f.commit("first")
	return f
}

func (f *treeFixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.w.Dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *treeFixture) commit(message string) {
	f.t.Helper()
	f.git("-c", "user.name=fixture", "-c", "user.email=fixture.invalid",
		"commit", "-q", "--allow-empty", "-m", message)
}

func (f *treeFixture) write(path, content string) {
	f.t.Helper()
	full := filepath.Join(f.w.Dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *treeFixture) snapshot() string {
	f.t.Helper()
	tree, err := f.w.Snapshot("HEAD")
	if err != nil {
		f.t.Fatalf("snapshot: %v", err)
	}
	return tree
}

func (f *treeFixture) entry(tree, path string) (Entry, bool) {
	f.t.Helper()
	entries, err := f.w.Entries(tree, []string{path})
	if err != nil {
		f.t.Fatalf("entries: %v", err)
	}
	e, ok := entries[path]
	return e, ok
}

// The snapshot must never touch the real index or the worktree.
func TestSnapshotLeavesRealIndexUntouched(t *testing.T) {
	f := newTreeFixture(t)
	f.write("staged.txt", "staged but not committed\n")
	f.git("add", "staged.txt")
	before := f.git("status", "--porcelain")
	f.snapshot()
	if after := f.git("status", "--porcelain"); after != before {
		t.Fatalf("snapshot mutated repository state:\nbefore %q\nafter  %q", before, after)
	}
}

// Deletion: a file removed from the worktree vanishes from the tree.
func TestSnapshotCapturesDeletion(t *testing.T) {
	f := newTreeFixture(t)
	f.write("doomed.txt", "bytes\n")
	f.git("add", "doomed.txt")
	f.commit("add doomed")
	if err := os.Remove(filepath.Join(f.w.Dir, "doomed.txt")); err != nil {
		t.Fatal(err)
	}
	tree := f.snapshot()
	if _, ok := f.entry(tree, "doomed.txt"); ok {
		t.Fatal("deleted file still present in snapshot tree")
	}
}

// Mode: an executable bit flip is a projection change (core.fileMode
// pinned true, so "mode" means git mode).
func TestSnapshotCapturesModeChange(t *testing.T) {
	f := newTreeFixture(t)
	f.write("tool.sh", "#!/bin/sh\n")
	f.git("add", "tool.sh")
	f.commit("add tool")
	if err := os.Chmod(filepath.Join(f.w.Dir, "tool.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	tree := f.snapshot()
	e, ok := f.entry(tree, "tool.sh")
	if !ok || e.Mode != "100755" {
		t.Fatalf("mode change not projected: %+v ok=%v", e, ok)
	}
}

// Symlink: the link itself (mode 120000, target as blob), not the target's
// content.
func TestSnapshotCapturesSymlink(t *testing.T) {
	f := newTreeFixture(t)
	if err := os.Symlink("README.md", filepath.Join(f.w.Dir, "link")); err != nil {
		t.Fatal(err)
	}
	tree := f.snapshot()
	e, ok := f.entry(tree, "link")
	if !ok || e.Mode != "120000" {
		t.Fatalf("symlink not projected as 120000: %+v ok=%v", e, ok)
	}
	target := f.git("cat-file", "blob", e.OID)
	if target != "README.md" {
		t.Fatalf("symlink target blob = %q", target)
	}
}

// Binary: non-text bytes survive the snapshot → diff → apply round trip
// exactly.
func TestBinaryDiffApplyRoundTrip(t *testing.T) {
	f := newTreeFixture(t)
	base := f.snapshot()
	blob := append([]byte{0x00, 0x01, 0xff, 0xfe, 0x0a}, []byte("binary")...)
	if err := os.WriteFile(filepath.Join(f.w.Dir, "asset.bin"), blob, 0o644); err != nil {
		t.Fatal(err)
	}
	reviewed := f.snapshot()
	patch, err := f.w.Diff(base, reviewed)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := f.w.Apply(base, patch)
	if err != nil {
		t.Fatalf("binary patch refused: %v", err)
	}
	if applied != reviewed {
		t.Fatalf("apply(diff(B,R), B) = %s, want %s", applied, reviewed)
	}
}

// Gitlink: a nested repository projects as a commit entry (160000) with the
// subproject's HEAD id; its dirty internals stay outside the projection.
func TestSnapshotCapturesGitlink(t *testing.T) {
	f := newTreeFixture(t)
	subDir := filepath.Join(f.w.Dir, "vendor", "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", subDir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(subDir, "inner.txt"), []byte("inner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("-c", "user.name=fixture", "-c", "user.email=fixture.invalid", "commit", "-q", "-m", "inner")
	subHead := strings.TrimSpace(func() string {
		out, err := exec.Command("git", "-C", subDir, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}())

	tree := f.snapshot()
	e, ok := f.entry(tree, "vendor/sub")
	if !ok || e.Mode != "160000" {
		t.Fatalf("gitlink not projected as 160000: %+v ok=%v", e, ok)
	}
	if e.OID != subHead {
		t.Fatalf("gitlink id = %s, want subproject HEAD %s", e.OID, subHead)
	}

	// Dirty content INSIDE the subproject is outside the projection.
	if err := os.WriteFile(filepath.Join(subDir, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if again := f.snapshot(); again != tree {
		t.Fatal("dirty submodule internals changed the superproject projection")
	}
}

// Ignored untracked files are outside the projection; tracked-and-ignored
// files stay inside it (add -A never drops a tracked file because a later
// ignore rule matches it — the wall's ignored-not-untracked distinction).
func TestIgnoredBoundary(t *testing.T) {
	f := newTreeFixture(t)
	f.write("kept.log", "tracked before the ignore rule\n")
	f.git("add", "kept.log")
	f.commit("track kept.log")
	f.write(".gitignore", "*.log\n")
	f.git("add", ".gitignore")
	f.commit("ignore logs")
	f.write("noise.log", "ignored untracked\n")
	tree := f.snapshot()
	if _, ok := f.entry(tree, "noise.log"); ok {
		t.Fatal("ignored untracked file leaked into the projection")
	}
	if _, ok := f.entry(tree, "kept.log"); !ok {
		t.Fatal("tracked-and-ignored file fell out of the projection")
	}
}

// Order: two authorized patches touching different files compose to the
// reviewed tree in the recorded order; the composition is the equation's
// left-to-right apply, not a merge.
func TestOrderedComposition(t *testing.T) {
	f := newTreeFixture(t)
	base := f.snapshot()

	f.write("a.txt", "from patch one\n")
	stepOne := f.snapshot()
	patchOne, err := f.w.Diff(base, stepOne)
	if err != nil {
		t.Fatal(err)
	}
	f.write("b.txt", "from patch two\n")
	stepTwo := f.snapshot()
	patchTwo, err := f.w.Diff(stepOne, stepTwo)
	if err != nil {
		t.Fatal(err)
	}

	mid, err := f.w.Apply(base, patchOne)
	if err != nil {
		t.Fatal(err)
	}
	final, err := f.w.Apply(mid, patchTwo)
	if err != nil {
		t.Fatal(err)
	}
	if final != stepTwo {
		t.Fatalf("ordered composition = %s, want %s", final, stepTwo)
	}
}

// Overlap: after the base drifts on the same file, the exact-apply rule
// refuses the stale patch rather than merging.
func TestOverlappingPatchRefused(t *testing.T) {
	f := newTreeFixture(t)
	f.write("shared.txt", "line one\nline two\nline three\n")
	f.git("add", "shared.txt")
	f.commit("shared baseline")
	base := f.snapshot()

	f.write("shared.txt", "line one\nEDIT A\nline three\n")
	editA := f.snapshot()
	patchA, err := f.w.Diff(base, editA)
	if err != nil {
		t.Fatal(err)
	}
	f.write("shared.txt", "line one\nEDIT B\nline three\n")
	editB := f.snapshot()

	if _, err := f.w.Apply(editB, patchA); err == nil {
		t.Fatal("stale overlapping patch applied; exact apply must refuse")
	}
}

// Non-application: a patch against bytes the base tree never had refuses,
// and refusal changes nothing durable.
func TestNonApplicationRefusesAndChangesNothing(t *testing.T) {
	f := newTreeFixture(t)
	base := f.snapshot()
	f.write("phantom.txt", "v1\n")
	v1 := f.snapshot()
	f.write("phantom.txt", "v2\n")
	v2 := f.snapshot()
	patch, err := f.w.Diff(v1, v2) // context expects v1, base never had it
	if err != nil {
		t.Fatal(err)
	}
	before := f.git("status", "--porcelain")
	if _, err := f.w.Apply(base, patch); err == nil {
		t.Fatal("patch with unknown context applied")
	}
	if after := f.git("status", "--porcelain"); after != before {
		t.Fatal("refused apply mutated repository state")
	}
}

// Per-entry equality (r5): the wall compares object id AND git mode per
// changed path; Entries reports both, and deletions read as absence.
func TestEntriesReportModeAndOID(t *testing.T) {
	f := newTreeFixture(t)
	f.write("plain.txt", "plain\n")
	f.write("exec.sh", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(f.w.Dir, "exec.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	tree := f.snapshot()
	entries, err := f.w.Entries(tree, []string{"plain.txt", "exec.sh", "absent.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if e := entries["plain.txt"]; e.Mode != "100644" || e.OID == "" {
		t.Fatalf("plain.txt entry: %+v", e)
	}
	if e := entries["exec.sh"]; e.Mode != "100755" {
		t.Fatalf("exec.sh entry: %+v", e)
	}
	if _, ok := entries["absent.txt"]; ok {
		t.Fatal("absent path reported as an entry")
	}
}

// Anchors: an anchored snapshot tree survives an aggressive prune; after
// DropAnchors the mission's refs are gone.
func TestAnchorKeepsTreeReachable(t *testing.T) {
	f := newTreeFixture(t)
	f.write("ephemeral.txt", "never committed\n")
	tree := f.snapshot()
	if err := f.w.Anchor("m-fixture", tree); err != nil {
		t.Fatal(err)
	}
	// Remove the worktree copy so nothing but the ref protects the objects.
	if err := os.Remove(filepath.Join(f.w.Dir, "ephemeral.txt")); err != nil {
		t.Fatal(err)
	}
	f.git("prune", "--expire=now")
	f.git("cat-file", "-e", tree) // fatal if pruned

	if err := f.w.DropAnchors("m-fixture"); err != nil {
		t.Fatal(err)
	}
	if refs := f.git("for-each-ref", "refs/metasystem/missions/m-fixture"); refs != "" {
		t.Fatalf("anchors survive DropAnchors: %q", refs)
	}
}

// roundTrip asserts Apply(Diff(base, reviewed), base) == reviewed and
// returns the applied tree — the HIW-O4 composition proof for one change
// class.
func (f *treeFixture) roundTrip(base, reviewed string) string {
	f.t.Helper()
	patch, err := f.w.Diff(base, reviewed)
	if err != nil {
		f.t.Fatal(err)
	}
	applied, err := f.w.Apply(base, patch)
	if err != nil {
		f.t.Fatalf("round trip refused: %v", err)
	}
	if applied != reviewed {
		f.t.Fatalf("apply(diff(B,R), B) = %s, want %s", applied, reviewed)
	}
	return applied
}

// Deletion, mode-only, symlink, and gitlink changes each survive the full
// Diff → Apply round trip, not just the snapshot (slice-1 critique F5).
func TestRoundTripPerChangeClass(t *testing.T) {
	f := newTreeFixture(t)
	f.write("victim.txt", "bytes\n")
	f.write("tool.sh", "#!/bin/sh\n")
	f.git("add", ".")
	f.commit("baseline")
	base := f.snapshot()

	// Deletion.
	if err := os.Remove(filepath.Join(f.w.Dir, "victim.txt")); err != nil {
		t.Fatal(err)
	}
	deleted := f.snapshot()
	f.roundTrip(base, deleted)

	// Mode-only.
	if err := os.Chmod(filepath.Join(f.w.Dir, "tool.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	moded := f.snapshot()
	f.roundTrip(deleted, moded)

	// Symlink.
	if err := os.Symlink("README.md", filepath.Join(f.w.Dir, "link")); err != nil {
		t.Fatal(err)
	}
	linked := f.snapshot()
	f.roundTrip(moded, linked)
}

// A gitlink id change survives the round trip: the wall must transport a
// superproject pointer update exactly like any other entry.
func TestRoundTripGitlink(t *testing.T) {
	f := newTreeFixture(t)
	subDir := filepath.Join(f.w.Dir, "vendor", "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sub := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", subDir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	sub("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(subDir, "inner.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub("add", ".")
	sub("-c", "user.name=fixture", "-c", "user.email=fixture.invalid", "commit", "-q", "-m", "v1")
	base := f.snapshot()

	if err := os.WriteFile(filepath.Join(subDir, "inner.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub("add", ".")
	sub("-c", "user.name=fixture", "-c", "user.email=fixture.invalid", "commit", "-q", "-m", "v2")
	moved := f.snapshot()
	if moved == base {
		t.Fatal("gitlink advance did not change the projection")
	}
	f.roundTrip(base, moved)
}

// Hostile repository configuration must not change patch bytes or apply
// exactness: prefixes stay a/ b/, whitespace fuzz stays refused, and the
// executable bit stays projected even with core.fileMode=false configured
// (slice-1 critique F1 — the pins beat the config).
func TestRoundTripUnderHostileConfig(t *testing.T) {
	f := newTreeFixture(t)
	f.git("config", "diff.noprefix", "true")
	f.git("config", "diff.mnemonicPrefix", "true")
	f.git("config", "diff.context", "0")
	f.git("config", "apply.ignoreWhitespace", "change")
	f.git("config", "core.fileMode", "false")

	f.write("code.txt", "indent matters\n")
	f.git("add", ".")
	f.commit("baseline")
	base := f.snapshot()
	f.write("code.txt", "  indent matters\n")
	if err := os.Chmod(filepath.Join(f.w.Dir, "code.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	reviewed := f.snapshot()
	f.roundTrip(base, reviewed)

	e, ok := f.entry(reviewed, "code.txt")
	if !ok || e.Mode != "100755" {
		t.Fatalf("core.fileMode=false leaked through the pin: %+v ok=%v", e, ok)
	}

	// The whitespace-only difference is a real difference: a patch whose
	// context has different leading whitespace must refuse even with
	// apply.ignoreWhitespace=change in the repository config.
	f.write("code.txt", "\tindent matters\n")
	drifted := f.snapshot()
	patch, err := f.w.Diff(base, reviewed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.w.Apply(drifted, patch); err == nil {
		t.Fatal("whitespace-drifted context applied; the exactness pin failed")
	}
}

// A filename that is also a glob pattern must resolve literally in
// Entries (slice-1 critique F2).
func TestEntriesLiteralFilename(t *testing.T) {
	f := newTreeFixture(t)
	f.write("a1.txt", "decoy\n")
	f.write("a[1].txt", "literal\n")
	tree := f.snapshot()
	entries, err := f.w.Entries(tree, []string{"a[1].txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("literal lookup returned %d entries: %v", len(entries), entries)
	}
	e, ok := entries["a[1].txt"]
	if !ok {
		t.Fatalf("literal filename missed: %v", entries)
	}
	if content := f.git("cat-file", "blob", e.OID); content != "literal" {
		t.Fatalf("glob resolved the decoy: %q", content)
	}
}

// Tracked-and-ignored files stay LIVE in the projection: a modification
// to one is a projection change, not just its continued presence.
func TestTrackedAndIgnoredModificationProjects(t *testing.T) {
	f := newTreeFixture(t)
	f.write("kept.log", "v1\n")
	f.git("add", "kept.log")
	f.commit("track kept.log")
	f.write(".gitignore", "*.log\n")
	f.git("add", ".gitignore")
	f.commit("ignore logs")
	before := f.snapshot()
	f.write("kept.log", "v2\n")
	after := f.snapshot()
	if before == after {
		t.Fatal("tracked-and-ignored modification invisible to the projection")
	}
	f.roundTrip(before, after)
}

// Arbitrary POSIX metadata is OUTSIDE the projection: touching mtimes
// changes nothing.
func TestPosixMetadataOutsideProjection(t *testing.T) {
	f := newTreeFixture(t)
	before := f.snapshot()
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(filepath.Join(f.w.Dir, "README.md"), old, old); err != nil {
		t.Fatal(err)
	}
	if after := f.snapshot(); after != before {
		t.Fatal("mtime change leaked into the projection")
	}
}

// Anchor refuses non-tree objects and foreign grammars, and creates no
// reflog even under core.logAllRefUpdates=always — a dropped anchor must
// not keep its tree GC-reachable through a log entry (slice-1 critique F4).
func TestAnchorRefusalsAndNoReflog(t *testing.T) {
	f := newTreeFixture(t)
	f.git("config", "core.logAllRefUpdates", "always")
	commit := f.git("rev-parse", "HEAD")
	if err := f.w.Anchor("m-fixture", commit); err == nil {
		t.Fatal("anchor accepted a commit object")
	}
	if err := f.w.Anchor("a/b", f.git("rev-parse", "HEAD^{tree}")); err == nil {
		t.Fatal("anchor accepted a mission id with a slash")
	}

	f.write("ephemeral.txt", "never committed\n")
	tree := f.snapshot()
	if err := f.w.Anchor("m-fixture", tree); err != nil {
		t.Fatal(err)
	}
	ref := "refs/metasystem/missions/m-fixture/" + tree
	logPath := filepath.Join(f.w.Dir, ".git", "logs", ref)
	if _, err := os.Stat(logPath); err == nil {
		t.Fatal("anchor created a reflog despite the logAllRefUpdates pin")
	}
	if err := f.w.DropAnchors("m-fixture"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.w.Dir, "ephemeral.txt")); err != nil {
		t.Fatal(err)
	}
	f.git("prune", "--expire=now")
	cmd := exec.Command("git", "-C", f.w.Dir, "cat-file", "-e", tree)
	if cmd.Run() == nil {
		t.Fatal("dropped anchor's tree survived an immediate prune")
	}
}

// The projection equals what the conformance validator historically
// computed: read-tree HEAD, add -A, write-tree — one owner, same bytes.
func TestSnapshotMatchesLegacyProjection(t *testing.T) {
	f := newTreeFixture(t)
	f.write("new.txt", "added\n")
	f.write("README.md", "modified\n")
	legacyDir := t.TempDir()
	env := append(os.Environ(), "GIT_INDEX_FILE="+filepath.Join(legacyDir, "index"))
	legacy := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", f.w.Dir}, args...)...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	legacy("read-tree", "HEAD")
	legacy("add", "-A", "--", ".")
	want := legacy("write-tree")
	if got := f.snapshot(); got != want {
		t.Fatalf("snapshot %s != legacy projection %s", got, want)
	}
}

// A nested checkout (the workspace is a subdirectory of the git toplevel —
// the supported deployment layout) speaks the SAME workspace-relative path
// space as a toplevel checkout: trees are scoped to the workspace prefix,
// lookups and exclusions match, and worktree noise outside the workspace
// is not the workspace's to project.
func TestNestedWorkspacePathSpace(t *testing.T) {
	top := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", top}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(top, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	write("outside.txt", "outer\n")
	write("metasystem/plans/contract.md", "signed\n")
	write("metasystem/truth/a.txt", "a\n")
	git("add", ".")
	git("-c", "user.name=f", "-c", "user.email=f@invalid", "commit", "-q", "-m", "first")

	w := Workspace{Dir: filepath.Join(top, "metasystem")}
	head, err := w.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := w.Entries(head, []string{"plans/contract.md"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := entries["plans/contract.md"]; !ok {
		t.Fatalf("HeadTree must speak workspace-relative paths: %v", entries)
	}

	// A clean nested workspace snapshots to HEAD's own subtree.
	clean, err := w.Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if clean != head {
		t.Fatalf("clean snapshot %s differs from the head subtree %s", clean, head)
	}

	// Worktree noise OUTSIDE the workspace is invisible to its projection.
	write("outside-noise.txt", "noise\n")
	still, err := w.Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if still != head {
		t.Fatal("toplevel noise leaked into the nested projection")
	}

	// A change INSIDE is captured under its workspace-relative path, and
	// FilterTree's exclusion matches that same path.
	write("metasystem/truth/a.txt", "changed\n")
	dirty, err := w.Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if dirty == head {
		t.Fatal("the workspace change was not captured")
	}
	dirtyFiltered, err := w.FilterTree(dirty, []string{"truth/a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	headFiltered, err := w.FilterTree(head, []string{"truth/a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if dirtyFiltered != headFiltered {
		t.Fatal("the workspace-relative exclusion did not remove the changed path")
	}

	// Diff and Apply agree in the same path space.
	patch, err := w.Diff(head, dirty)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := w.Apply(head, patch)
	if err != nil {
		t.Fatal(err)
	}
	if applied != dirty {
		t.Fatalf("apply(head, diff) = %s, want %s", applied, dirty)
	}
}
