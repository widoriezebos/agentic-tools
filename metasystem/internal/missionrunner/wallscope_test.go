package missionrunner

// Fixtures for the wall's snapshot scope: HEAD-movement
// accounting, the ref transition fence, staged accounting at both
// scopes, the worktree census, the nested repository fence, and the
// two-phase acceptance verification. Each bed is a real git repository;
// every judgment runs the production capture and rules.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// scopeBed is a minimal snapshot-scope bed: a committed repository, an
// engine bound to it, the open origin captured exactly as turn open
// records it, and the accountant over the current state.
type scopeBed struct {
	t       *testing.T
	engine  *Engine
	ws      gittree.Workspace
	preTree string
	origin  *scopeOrigin
	state   map[string]any
}

func (b *scopeBed) git(args ...string) string {
	b.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", b.engine.Root}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=host", "GIT_AUTHOR_EMAIL=host@test",
		"GIT_COMMITTER_NAME=host", "GIT_COMMITTER_EMAIL=host@test")
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newScopeBed(t *testing.T) *scopeBed {
	t.Helper()
	root := wallRepo(t)
	engine := &Engine{Root: root, Mission: "demo"}
	ws := gittree.Workspace{Dir: root}
	pre, err := wallSnapshot(ws, "demo")
	if err != nil {
		t.Fatal(err)
	}
	bed := &scopeBed{t: t, engine: engine, ws: ws, preTree: pre}
	bed.state = map[string]any{
		"branch": "main", "turnLog": []any{}, "initialBaseline": pre,
		"workspaceTaint": map[string]any{"next": 1, "segment": 0, "entries": []any{}},
	}
	bed.captureOrigin()
	return bed
}

// captureOrigin records the open-time origin the fixtures judge from.
func (b *scopeBed) captureOrigin() {
	b.t.Helper()
	capture, err := b.engine.captureWallPosture("", nil)
	if err != nil {
		b.t.Fatal(err)
	}
	refMap := map[string]string{}
	for name, oid := range capture.RefMap {
		refMap[name] = oid
	}
	b.origin = &scopeOrigin{Head: capture.Head, RefMap: refMap, OpenAnchor: ""}
	if capture.Nested {
		b.origin.TopTree = capture.TopTree
		staged := capture.TopStaged
		b.origin.TopStaged = &staged
	}
}

// judge runs the full snapshot-scope judgment with the given consumed
// authorizations, expected composition, and declared paths.
func (b *scopeBed) judge(auths []scopeAuth, expected string, declared map[string]bool) string {
	b.t.Helper()
	capture, err := b.engine.captureWallPosture(expected, declared)
	if err != nil {
		b.t.Fatal(err)
	}
	acct, err := b.engine.newWallAccountant(b.preTree, b.state, auths, declared)
	if err != nil {
		b.t.Fatal(err)
	}
	acct.noteExpected(expected)
	violation, err := b.engine.judgeScope(b.origin, capture, acct, b.state)
	if err != nil {
		b.t.Fatalf("judge: %v", err)
	}
	return violation
}

// scopeAuthFor mints the accounting facts of one reviewed change: the
// diff between two trees as a consumed authorization.
func (b *scopeBed) scopeAuthFor(base, reviewed string) scopeAuth {
	b.t.Helper()
	changed, err := b.ws.ChangedPaths(base, reviewed)
	if err != nil {
		b.t.Fatal(err)
	}
	entries, err := b.ws.Entries(reviewed, changed)
	if err != nil {
		b.t.Fatal(err)
	}
	return scopeAuth{digest: strings.Repeat("d", 64), changedPaths: changed,
		reviewedTree: reviewed, reviewedEntries: entries}
}

// stageAndSnapshot builds a tree from the CURRENT worktree without
// leaving anything staged — the reviewed-tree seed for authorizations.
func (b *scopeBed) worktreeTree() string {
	b.t.Helper()
	tree, err := wallSnapshot(b.ws, "demo")
	if err != nil {
		b.t.Fatal(err)
	}
	return tree
}

// A clean bed passes every rule.
func TestScopeCleanBedPasses(t *testing.T) {
	bed := newScopeBed(t)
	if violation := bed.judge(nil, bed.preTree, nil); violation != "" {
		t.Fatalf("clean bed violated: %s", violation)
	}
}

// A HEAD retreat is a violation.
func TestScopeHeadRetreatViolates(t *testing.T) {
	bed := newScopeBed(t)
	writeText(t, filepath.Join(bed.engine.Root, "second.go"), "package main\n")
	bed.git("add", "second.go")
	bed.git("commit", "-qm", "second")
	bed.captureOrigin()
	bed.git("reset", "-q", "--hard", "HEAD~1")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "retreated or rewrote history") {
		t.Fatalf("retreat must violate: %q", violation)
	}
}

// An amend of pre-open history leaves the open commit off the
// first-parent chain.
func TestScopeAmendViolates(t *testing.T) {
	bed := newScopeBed(t)
	bed.git("commit", "-q", "--amend", "--allow-empty", "-m", "rewritten")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "retreated or rewrote history") {
		t.Fatalf("amend must violate: %q", violation)
	}
}

// A commit carrying an unaccounted tree names itself.
func TestScopeUnaccountedCommitViolates(t *testing.T) {
	bed := newScopeBed(t)
	writeText(t, filepath.Join(bed.engine.Root, "smuggled.go"), "package main\n")
	bed.git("add", "smuggled.go")
	bed.git("commit", "-qm", "host smuggles")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "advances HEAD with an unaccounted tree") {
		t.Fatalf("unaccounted commit must violate: %q", violation)
	}
}

// An empty commit moves no byte and is lawful — accounting
// is by content, not ceremony. HEAD unmoved is lawful too.
func TestScopeEmptyCommitAndNoCommitLawful(t *testing.T) {
	bed := newScopeBed(t)
	if violation := bed.judge(nil, bed.preTree, nil); violation != "" {
		t.Fatalf("no-commit turn violated: %q", violation)
	}
	bed.git("commit", "-q", "--allow-empty", "-m", "empty")
	if violation := bed.judge(nil, bed.preTree, nil); violation != "" {
		t.Fatalf("empty commit violated: %q", violation)
	}
}

// A commit of the expected composition is lawful, and a
// commit of an intermediate subset (one whole patch of two) is lawful.
func TestScopeSubsetAndFullCompositionCommitsLawful(t *testing.T) {
	bed := newScopeBed(t)
	root := bed.engine.Root
	writeText(t, filepath.Join(root, "one.go"), "package one\n")
	bed.git("add", "one.go")
	subset := bed.worktreeTree()
	authOne := bed.scopeAuthFor(bed.preTree, subset)
	writeText(t, filepath.Join(root, "two.go"), "package two\n")
	bed.git("add", "two.go")
	full := bed.worktreeTree()
	authTwo := bed.scopeAuthFor(subset, full)
	authTwo.digest = strings.Repeat("e", 64)
	bed.git("commit", "-qm", "subset then full")
	// One commit carrying the full composition: accounted.
	if violation := bed.judge([]scopeAuth{authOne, authTwo}, full, nil); violation != "" {
		t.Fatalf("full composition commit violated: %q", violation)
	}
	// Rewind and commit only the first patch: still accounted (whole
	// patches, order-free membership).
	bed.git("reset", "-q", "--hard", bed.origin.Head)
	writeText(t, filepath.Join(root, "one.go"), "package one\n")
	bed.git("add", "one.go")
	bed.git("commit", "-qm", "subset only")
	writeText(t, filepath.Join(root, "two.go"), "package two\n")
	bed.git("add", "two.go")
	bed.git("commit", "-qm", "rest")
	if violation := bed.judge([]scopeAuth{authOne, authTwo}, full, nil); violation != "" {
		t.Fatalf("subset-state commit violated: %q", violation)
	}
}

// A partially-carried patch (one hunk of a reviewed change)
// violates the whole-patch rule.
func TestScopePartialPatchCommitViolates(t *testing.T) {
	bed := newScopeBed(t)
	root := bed.engine.Root
	writeText(t, filepath.Join(root, "a.go"), "package a\n")
	writeText(t, filepath.Join(root, "b.go"), "package b\n")
	bed.git("add", "a.go", "b.go")
	full := bed.worktreeTree()
	auth := bed.scopeAuthFor(bed.preTree, full)
	bed.git("reset", "-q", "main", "--", "b.go")
	os.Remove(filepath.Join(root, "b.go"))
	bed.git("commit", "-qm", "half the patch")
	violation := bed.judge([]scopeAuth{auth}, full, nil)
	if !strings.Contains(violation, "advances HEAD with an unaccounted tree") {
		t.Fatalf("partial patch must violate: %q", violation)
	}
}

// The side-tip lane: a reviewed side branch integrated with --no-ff is
// lawful; the same branch fast-forwarded puts intermediate trees on the
// first-parent chain and the violation names the remedy.
func TestScopeMergeIntegrationLawfulAndFastForwardNamesRemedy(t *testing.T) {
	bed := newScopeBed(t)
	root := bed.engine.Root
	base := bed.origin.Head
	// The dispatch record admits the implementer branch to its free lane.
	jobsDir := jobsDirPath(root)
	os.MkdirAll(jobsDir, 0o755)
	writeJSONFile(t, filepath.Join(jobsDir, "job-x.json"),
		map[string]any{"jobId": "job-x", "branch": "agent/job-x", "mission": "demo", "role": "implementer"})
	// The implementer branch: two commits, the tip being the reviewed tree.
	bed.git("checkout", "-q", "-b", "agent/job-x")
	writeText(t, filepath.Join(root, "draft.go"), "package draft\n")
	bed.git("add", "draft.go")
	bed.git("commit", "-qm", "intermediate")
	writeText(t, filepath.Join(root, "draft.go"), "package final\n")
	bed.git("add", "draft.go")
	bed.git("commit", "-qm", "final")
	reviewed := bed.worktreeTree()
	auth := bed.scopeAuthFor(bed.preTree, reviewed)
	bed.git("checkout", "-q", "main")
	// Lawful: --no-ff merge, side tip is the reviewed tree.
	bed.git("merge", "-q", "--no-ff", "-m", "integrate", "agent/job-x")
	if violation := bed.judge([]scopeAuth{auth}, reviewed, nil); violation != "" {
		t.Fatalf("no-ff integration violated: %q", violation)
	}
	// Unlawful: rewind and fast-forward the same branch.
	bed.git("reset", "-q", "--hard", base)
	bed.git("merge", "-q", "--ff-only", "agent/job-x")
	violation := bed.judge([]scopeAuth{auth}, reviewed, nil)
	if !strings.Contains(violation, "--no-ff") {
		t.Fatalf("fast-forward must name the remedy: %q", violation)
	}
}

// An ours-merge burying an illicit side tip fails by its tip.
func TestScopeOursMergeBuriedCommitViolates(t *testing.T) {
	bed := newScopeBed(t)
	root := bed.engine.Root
	bed.git("checkout", "-q", "-b", "smuggle")
	writeText(t, filepath.Join(root, "payload.go"), "package payload\n")
	bed.git("add", "payload.go")
	bed.git("commit", "-qm", "illicit")
	bed.git("checkout", "-q", "main")
	bed.git("merge", "-q", "-s", "ours", "-m", "bury", "smuggle")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "merge side tip") {
		t.Fatalf("ours-merge burial must violate by its tip: %q", violation)
	}
}

// Tag, custom-namespace, and remote-namespace retention all
// violate the exact transition fence.
func TestScopeRefRetentionViolates(t *testing.T) {
	for _, ref := range []string{"refs/tags/keeper", "refs/custom/hideout", "refs/remotes/origin/x"} {
		t.Run(ref, func(t *testing.T) {
			bed := newScopeBed(t)
			bed.git("update-ref", ref, bed.origin.Head)
			violation := bed.judge(nil, bed.preTree, nil)
			if !strings.Contains(violation, ref) || !strings.Contains(violation, "created") {
				t.Fatalf("retention under %s must violate: %q", ref, violation)
			}
		})
	}
}

// A same-tip detach violates — the active branch must BE the
// checkout.
func TestScopeSameTipDetachViolates(t *testing.T) {
	bed := newScopeBed(t)
	bed.git("checkout", "-q", "--detach")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "left the mission branch") {
		t.Fatalf("same-tip detach must violate: %q", violation)
	}
}

// An implementer branch moves freely while unconsumed and holds
// still from consumption on.
func TestScopeAgentBranchFreeThenHeld(t *testing.T) {
	bed := newScopeBed(t)
	root := bed.engine.Root
	jobsDir := jobsDirPath(root)
	os.MkdirAll(jobsDir, 0o755)
	writeJSONFile(t, filepath.Join(jobsDir, "job-1.json"),
		map[string]any{"jobId": "job-1", "branch": "agent/job-1", "mission": "demo", "role": "implementer"})
	bed.git("branch", "-q", "agent/job-1", bed.origin.Head)
	bed.captureOrigin()
	delegate := bed.git("commit-tree", bed.origin.Head+"^{tree}", "-p", bed.origin.Head, "-m", "delegate")
	bed.git("update-ref", "refs/heads/agent/job-1", delegate)
	capture, err := bed.engine.captureWallPosture(bed.preTree, nil)
	if err != nil {
		t.Fatal(err)
	}
	acct, err := bed.engine.newWallAccountant(bed.preTree, bed.state, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Unconsumed: free.
	if violation, err := bed.engine.judgeRefFence(bed.origin, capture, bed.state, map[string]bool{}, mustAccountant(t, bed)); err != nil || violation != "" {
		t.Fatalf("unconsumed branch motion must be free: %q %v", violation, err)
	}
	// Consumed: stationary.
	violation, err := bed.engine.judgeRefFence(bed.origin, capture, bed.state, map[string]bool{"job-1": true}, mustAccountant(t, bed))
	if err != nil || !strings.Contains(violation, "moved after consumption") {
		t.Fatalf("consumed branch motion must violate: %q %v", violation, err)
	}
	_ = acct
}

// Pseudoref retention — an unaccounted commit parked in
// REBASE_HEAD violates; ORIG_HEAD at an accounted commit is lawful
// (exactly what a lawful --no-ff integration leaves).
func TestScopePseudorefRetention(t *testing.T) {
	bed := newScopeBed(t)
	gitDir := bed.git("rev-parse", "--absolute-git-dir")
	// Lawful: ORIG_HEAD at the accounted open commit.
	if err := os.WriteFile(filepath.Join(gitDir, "ORIG_HEAD"), []byte(bed.origin.Head+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if violation := bed.judge(nil, bed.preTree, nil); violation != "" {
		t.Fatalf("accounted ORIG_HEAD violated: %q", violation)
	}
	// Violation: an unaccounted commit parked in REBASE_HEAD.
	writeText(t, filepath.Join(bed.engine.Root, "hidden.go"), "package hidden\n")
	bed.git("add", "hidden.go")
	smuggled := bed.git("commit-tree", bed.git("write-tree"), "-p", bed.origin.Head, "-m", "hidden")
	bed.git("reset", "-q", "main", "--", "hidden.go")
	os.Remove(filepath.Join(bed.engine.Root, "hidden.go"))
	if err := os.WriteFile(filepath.Join(gitDir, "REBASE_HEAD"), []byte(smuggled+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "REBASE_HEAD") {
		t.Fatalf("REBASE_HEAD retention must violate: %q", violation)
	}
}

// An unrecorded worktree is a private carrier and violates
// outright; a runner-recorded measurement worktree at its recorded tip
// is lawful.
func TestScopeWorktreeCensus(t *testing.T) {
	bed := newScopeBed(t)
	linked := filepath.Join(t.TempDir(), "private")
	bed.git("worktree", "add", "--detach", "-q", linked, bed.origin.Head)
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "unrecorded worktree") {
		t.Fatalf("unrecorded worktree must violate: %q", violation)
	}
	// Record it as a measurement worktree at its tip: lawful.
	registry := measureWorktreeRecordsPath(bed.engine.Root)
	os.MkdirAll(filepath.Dir(registry), 0o755)
	writeText(t, registry, `{"path":"`+linked+`","sha":"`+bed.origin.Head+`"}`+"\n")
	if violation := bed.judge(nil, bed.preTree, nil); violation != "" {
		t.Fatalf("recorded measurement worktree violated: %q", violation)
	}
	// A recorded worktree off its recorded tip violates.
	other := bed.git("commit-tree", bed.origin.Head+"^{tree}", "-p", bed.origin.Head, "-m", "moved")
	cmd := exec.Command("git", "-C", linked, "checkout", "-q", "--detach", other)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout in linked worktree: %v %s", err, out)
	}
	violation = bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "left its recorded tip") {
		t.Fatalf("moved measurement worktree must violate: %q", violation)
	}
}

// Staged smuggling violates; a staged lawful subset passes; a
// conflicted workspace index refuses toward the wall.
func TestScopeStagedAccounting(t *testing.T) {
	bed := newScopeBed(t)
	root := bed.engine.Root
	// Smuggle: stage bytes never reviewed.
	writeText(t, filepath.Join(root, "staged.go"), "package staged\n")
	bed.git("add", "staged.go")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "staged bytes unaccounted") {
		t.Fatalf("staged smuggle must violate: %q", violation)
	}
	// The same staged state under a consumed authorization is lawful.
	full := bed.worktreeTree()
	auth := bed.scopeAuthFor(bed.preTree, full)
	if violation := bed.judge([]scopeAuth{auth}, full, nil); violation != "" {
		t.Fatalf("lawful staged subset violated: %q", violation)
	}
	// A revert of the staged bytes (smuggle-then-revert) passes with no
	// auth: the projection equals the pre-tree again.
	bed.git("reset", "-q", "main", "--", "staged.go")
	bed.git("rm", "-q", "--cached", "--ignore-unmatch", "staged.go")
	os.Remove(filepath.Join(root, "staged.go"))
	if violation := bed.judge(nil, bed.preTree, nil); violation != "" {
		t.Fatalf("reverted staged state violated: %q", violation)
	}
}

// nestedScopeBed builds the nested-checkout shape: the workspace one
// level under the toplevel, a sibling beside it.
func newNestedScopeBed(t *testing.T) *scopeBed {
	t.Helper()
	top := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", top}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	writeText(t, filepath.Join(top, "ws", ".gitignore"), "artifacts/\n")
	writeText(t, filepath.Join(top, "ws", "main.go"), "package main\n")
	writeText(t, filepath.Join(top, "ws", "metasystem.conf"), "metasystem.runtimes=fake\n")
	writeText(t, filepath.Join(top, "sibling", "lib.go"), "package lib\n")
	run("add", "-A")
	run("commit", "-qm", "nested baseline")
	root := filepath.Join(top, "ws")
	engine := &Engine{Root: root, Mission: "demo"}
	ws := gittree.Workspace{Dir: root}
	pre, err := wallSnapshot(ws, "demo")
	if err != nil {
		t.Fatal(err)
	}
	bed := &scopeBed{t: t, engine: engine, ws: ws, preTree: pre}
	bed.state = map[string]any{
		"branch": "main", "turnLog": []any{}, "initialBaseline": pre,
		"workspaceTaint": map[string]any{"next": 1, "segment": 0, "entries": []any{}},
	}
	bed.captureOrigin()
	return bed
}

// A sibling edit mid-turn violates with the paths named; a
// workspace edit does not trip the toplevel fence.
func TestScopeSiblingEditViolates(t *testing.T) {
	bed := newNestedScopeBed(t)
	top, err := bed.ws.TopLevel()
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(top, "sibling", "lib.go"), "package altered\n")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "sibling paths changed") || !strings.Contains(violation, "sibling/lib.go") {
		t.Fatalf("sibling edit must violate with the path named: %q", violation)
	}
}

// A host commit touching a sibling path violates at repository
// scope even when its workspace subtree is clean.
func TestScopeSiblingCommitViolates(t *testing.T) {
	bed := newNestedScopeBed(t)
	top, err := bed.ws.TopLevel()
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(top, "sibling", "lib.go"), "package smuggled\n")
	bed.git("-C", top, "add", "sibling/lib.go")
	bed.git("-C", top, "commit", "-qm", "sibling smuggle")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "sibling paths changed") {
		t.Fatalf("sibling commit must violate: %q", violation)
	}
}

// A sibling payload buried in an interior side commit under an
// empty accounted tip is caught by the ACCUMULATED side-chain scope.
func TestScopeSiblingPayloadBuriedUnderAccountedTipViolates(t *testing.T) {
	bed := newNestedScopeBed(t)
	top, err := bed.ws.TopLevel()
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		bed.t.Helper()
		return bed.git(append([]string{"-C", top}, args...)...)
	}
	git("checkout", "-q", "-b", "cover")
	writeText(t, filepath.Join(top, "sibling", "payload.go"), "package payload\n")
	git("add", "sibling/payload.go")
	git("commit", "-qm", "interior payload")
	git("commit", "-q", "--allow-empty", "-m", "cover tip: empty immediate delta, empty workspace delta")
	git("checkout", "-q", "main")
	git("merge", "-q", "-s", "ours", "-m", "bury", "cover")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "sibling") {
		t.Fatalf("buried sibling payload must violate: %q", violation)
	}
}

// Nested: staged sibling motion violates; a preexisting sibling
// conflict refuses nothing.
func TestScopeToplevelStagedMotion(t *testing.T) {
	bed := newNestedScopeBed(t)
	top, err := bed.ws.TopLevel()
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(top, "sibling", "staged.go"), "package staged\n")
	bed.git("-C", top, "add", "sibling/staged.go")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "staged bytes unaccounted") || !strings.Contains(violation, "sibling") {
		t.Fatalf("staged sibling motion must violate: %q", violation)
	}
}

// The seeded projection follows the comparison target — a
// committed ignored declared artifact stays projected instead of
// reading as drift.
func TestScopeSeededCaptureFollowsExpected(t *testing.T) {
	bed := newScopeBed(t)
	root := bed.engine.Root
	writeText(t, filepath.Join(root, ".gitignore"), "artifacts/\nout.bin\n")
	bed.git("add", ".gitignore")
	bed.git("commit", "-qm", "ignore out.bin")
	bed.captureOrigin()
	pre, err := wallSnapshot(bed.ws, "demo")
	if err != nil {
		t.Fatal(err)
	}
	bed.preTree = pre
	writeText(t, filepath.Join(root, "out.bin"), "artifact bytes\n")
	declared := map[string]bool{"out.bin": true}
	capture, err := bed.engine.captureWallPosture(pre, declared)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := bed.ws.Entries(capture.Post, []string{"out.bin"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := entries["out.bin"]; !ok {
		t.Fatalf("declared ignored artifact vanished from the seeded projection")
	}
}

// The post-publication verification concludes a clean turn and
// parks over motion between the acceptance write and its verification.
func TestScopeAcceptanceVerification(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	statePath, err := seedCrashedMissionState(t, engine)
	if err != nil {
		t.Fatal(err)
	}
	openFixtureTurn(t, engine.Root, statePath, "alpha-t1-live", 1)
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	if err := engine.anchor(statePath, ledgerPath, "open"); err != nil {
		t.Fatal(err)
	}
	turnDir := filepath.Join(engine.missionDir(), "turns", "alpha-t1-live")
	os.MkdirAll(turnDir, 0o755)
	writeJSONFile(t, filepath.Join(turnDir, "turn.json"),
		map[string]any{"missionId": engine.Mission, "turnId": "alpha-t1-live", "cycle": 1,
			"runtime": "fake", "model": "fixture", "status": "running"})
	// Append the acceptance entry with the REAL captured posture — the
	// state a crash between the two writes leaves.
	state := readTestDoc(t, statePath)
	openTurn, _ := state["openTurn"].(map[string]any)
	pre, _ := openTurn["preTree"].(string)
	capture, err := engine.captureWallPosture(pre, nil)
	if err != nil {
		t.Fatal(err)
	}
	integrity, _ := state["integrity"].(map[string]any)
	sequence, _ := jsonInt(integrity["sequence"])
	hash, _ := integrity["hash"].(string)
	wall := map[string]any{
		"verdict": "passed", "preTree": pre, "expectedTree": pre, "postTree": capture.Post,
		"orderedDigests": []any{},
		"sequencePoint":  map[string]any{"sequence": sequence + 1, "segment": 0},
	}
	for field, value := range capture.postureDoc(engine.Mission) {
		wall[field] = value
	}
	proposed := readTestDoc(t, statePath)
	turnLog, _ := proposed["turnLog"].([]any)
	proposed["turnLog"] = append(turnLog, map[string]any{
		"turnId": "alpha-t1-live", "cycle": 1, "outcome": "completed",
		"detail": "host return accepted", "sessionId": nil, "measurement": nil,
		"accepted": []any{}, "rejected": []any{}, "certified": []any{},
		"factsForLedger": []any{}, "gaps": []any{},
		"wall": wall, "consumedAuthorizations": []any{}, "gatePassed": false,
	})
	if _, err := mission.AppendCycle(ledgerPath, 1, "no-progress", strings.Repeat("a", 40), "observed=unmeasurable:test", ""); err != nil {
		t.Fatal(err)
	}
	ledgerBlock, _ := proposed["ledger"].(map[string]any)
	ledgerBlock["cycles"] = 1
	fences, _ := proposed["fences"].(map[string]any)
	fences["cycles"] = 1
	delete(proposed, "integrity")
	source := statePath + ".acc.src"
	writeJSONFile(t, source, proposed)
	if err := mission.WriteState(statePath, source, hash); err != nil {
		t.Fatalf("acceptance write: %v", err)
	}
	if err := engine.anchor(statePath, ledgerPath, "acc"); err != nil {
		t.Fatal(err)
	}
	// The state is consumed-but-unconcluded.
	if pending := mission.UnverifiedAcceptance(readTestDoc(t, statePath)); pending != "alpha-t1-live" {
		t.Fatalf("expected the defined interval state, got %q", pending)
	}
	// Clean verification concludes: verification entry, marker closed.
	final, parked, err := engine.verifyAcceptance(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil)
	if err != nil || parked {
		t.Fatalf("clean verification: parked=%v err=%v", parked, err)
	}
	if final["openTurn"] != nil {
		t.Fatalf("verification must conclude the open turn")
	}
	if pending := mission.UnverifiedAcceptance(final); pending != "" {
		t.Fatalf("verification entry missing: %q", pending)
	}
}

// Motion after the acceptance write parks over the acceptance.
func TestScopeAcceptanceVerificationCatchesMotion(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	statePath, err := seedCrashedMissionState(t, engine)
	if err != nil {
		t.Fatal(err)
	}
	openFixtureTurn(t, engine.Root, statePath, "alpha-t1-live", 1)
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	if err := engine.anchor(statePath, ledgerPath, "open"); err != nil {
		t.Fatal(err)
	}
	turnDir := filepath.Join(engine.missionDir(), "turns", "alpha-t1-live")
	os.MkdirAll(turnDir, 0o755)
	writeJSONFile(t, filepath.Join(turnDir, "turn.json"),
		map[string]any{"missionId": engine.Mission, "turnId": "alpha-t1-live", "cycle": 1,
			"runtime": "fake", "model": "fixture", "status": "running"})
	state := readTestDoc(t, statePath)
	openTurn, _ := state["openTurn"].(map[string]any)
	pre, _ := openTurn["preTree"].(string)
	capture, err := engine.captureWallPosture(pre, nil)
	if err != nil {
		t.Fatal(err)
	}
	integrity, _ := state["integrity"].(map[string]any)
	sequence, _ := jsonInt(integrity["sequence"])
	hash, _ := integrity["hash"].(string)
	wall := map[string]any{
		"verdict": "passed", "preTree": pre, "expectedTree": pre, "postTree": capture.Post,
		"orderedDigests": []any{},
		"sequencePoint":  map[string]any{"sequence": sequence + 1, "segment": 0},
	}
	for field, value := range capture.postureDoc(engine.Mission) {
		wall[field] = value
	}
	proposed := readTestDoc(t, statePath)
	turnLog, _ := proposed["turnLog"].([]any)
	proposed["turnLog"] = append(turnLog, map[string]any{
		"turnId": "alpha-t1-live", "cycle": 1, "outcome": "completed",
		"detail": "host return accepted", "sessionId": nil, "measurement": nil,
		"accepted": []any{}, "rejected": []any{}, "certified": []any{},
		"factsForLedger": []any{}, "gaps": []any{},
		"wall": wall, "consumedAuthorizations": []any{}, "gatePassed": false,
	})
	if _, err := mission.AppendCycle(ledgerPath, 1, "no-progress", strings.Repeat("a", 40), "observed=unmeasurable:test", ""); err != nil {
		t.Fatal(err)
	}
	ledgerBlock, _ := proposed["ledger"].(map[string]any)
	ledgerBlock["cycles"] = 1
	fences, _ := proposed["fences"].(map[string]any)
	fences["cycles"] = 1
	delete(proposed, "integrity")
	source := statePath + ".acc.src"
	writeJSONFile(t, source, proposed)
	if err := mission.WriteState(statePath, source, hash); err != nil {
		t.Fatalf("acceptance write: %v", err)
	}
	if err := engine.anchor(statePath, ledgerPath, "acc"); err != nil {
		t.Fatal(err)
	}
	// The gate mutates the repository after the acceptance write.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", engine.Root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=g", "GIT_AUTHOR_EMAIL=g@g",
			"GIT_COMMITTER_NAME=g", "GIT_COMMITTER_EMAIL=g@g")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("commit", "-q", "--allow-empty", "-m", "post-acceptance motion")
	// The park proposal re-projects the fence counters from disk; the
	// bed's fence file must carry the reserved cycle like a real run's.
	fenceCounters := readTestDoc(t, engine.fencesPath())
	fenceCounters["cycles"] = 1
	writeJSONFile(t, engine.fencesPath(), fenceCounters)
	final, parked, err := engine.verifyAcceptance(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !parked {
		t.Fatalf("post-acceptance motion must park: %v", final["status"])
	}
	if final["parkReason"] != "wall-violation" {
		t.Fatalf("park reason: %v", final["parkReason"])
	}
	taintReason := unresolvedTaint(final)
	if !strings.Contains(taintReason, "post-verification") {
		t.Fatalf("the taint must name the verification window: %q", taintReason)
	}
}

// Admission REFUSES a nonempty replacement namespace — the real
// preflight path, not merely the ref's presence.
func TestScopeAdmissionRefusesReplaceNamespace(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	base := gittree.Workspace{Dir: engine.Root}
	head, _, err := base.HeadCommit()
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", engine.Root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, rerr := cmd.CombinedOutput(); rerr != nil {
			t.Fatalf("git %v: %v %s", args, rerr, out)
		}
	}
	run("commit", "-q", "--allow-empty", "-m", "other")
	other, _, _ := base.HeadCommit()
	run("reset", "-q", "--hard", head)
	run("update-ref", "refs/replace/"+head, other)
	approvedText, values, _, err := engine.parseContract(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, aerr := engine.admittedBaseline(values, []byte(approvedText)); aerr == nil ||
		!strings.Contains(aerr.Error(), "replacement namespace is not empty") {
		t.Fatalf("admission must refuse a nonempty replacement namespace, got %v", aerr)
	}
}

// RESTORE verifies every carrier — a staged remnant or an
// unaccounted commit refuses the restore even when the worktree equals
// the named safe tree — and the recorded resolution carries the full
// carrier posture as the next accounting origin.
func TestScopeRestoreVerifiesCarriers(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state := readTestDoc(t, statePath)
	openTurn := state["openTurn"].(map[string]any)
	preTree := openTurn["preTree"].(string)
	openHead := openTurn["headCommit"].(string)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", engine.Root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=h", "GIT_AUTHOR_EMAIL=h@h",
			"GIT_COMMITTER_NAME=h", "GIT_COMMITTER_EMAIL=h@h")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	// The human restores the worktree but leaves smuggled STAGED bytes.
	if err := os.Remove(filepath.Join(engine.Root, "solo.go")); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(engine.Root, "staged-extra.go"), "package staged\n")
	git("add", "staged-extra.go")
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restore", nil); code == 0 {
		t.Fatal("restore must refuse while staged bytes differ from the named tree")
	}
	git("reset", "-q", "HEAD", "--", "staged-extra.go")
	os.Remove(filepath.Join(engine.Root, "staged-extra.go"))
	// An unaccounted commit is a carrier the worktree restore cannot
	// un-ship: adoption or human git surgery first.
	writeText(t, filepath.Join(engine.Root, "committed-extra.go"), "package committed\n")
	git("add", "committed-extra.go")
	git("commit", "-qm", "unaccounted")
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restore", nil); code == 0 {
		t.Fatal("restore must refuse while committed HEAD carries unaccounted commits")
	}
	git("reset", "-q", "--hard", openHead)
	// The reset itself parked the smuggled commit in ORIG_HEAD — a
	// retention carrier in its own right; the surgery clears it too.
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restore", nil); code == 0 {
		t.Fatal("restore must refuse while ORIG_HEAD retains the unaccounted commit")
	}
	gitDirOut, err := exec.Command("git", "-C", engine.Root, "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(strings.TrimSpace(string(gitDirOut)), "ORIG_HEAD")); err != nil {
		t.Fatal(err)
	}
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restore", nil); code != 0 {
		t.Fatalf("restore must succeed once every carrier equals the ruled state: %d", code)
	}
	after := readTestDoc(t, statePath)
	taint := after["workspaceTaint"].(map[string]any)
	entry := taint["entries"].([]any)[0].(map[string]any)
	resolution, _ := entry["resolution"].(map[string]any)
	posture, _ := resolution["posture"].(map[string]any)
	if posture == nil {
		t.Fatalf("the resolution must record the carrier posture: %v", resolution)
	}
	if head, _ := posture["headCommitPost"].(string); head != openHead {
		t.Fatalf("recorded posture head %s != %s", head, openHead)
	}
}

// A grafts file forges the first-parent walk; its presence is
// a violation before any accounting runs.
func TestScopeGraftFileViolates(t *testing.T) {
	bed := newScopeBed(t)
	gitDir := bed.git("rev-parse", "--absolute-git-dir")
	if err := os.MkdirAll(filepath.Join(gitDir, "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(gitDir, "info", "grafts"), bed.origin.Head+"\n")
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "history-steering") {
		t.Fatalf("a grafts file must violate: %q", violation)
	}
}

// A commit parked under its own id in the mission's anchor
// namespace is not an anchor — anchors are trees.
func TestScopeCommitInAnchorNamespaceViolates(t *testing.T) {
	bed := newScopeBed(t)
	smuggled := bed.git("commit-tree", bed.origin.Head+"^{tree}", "-p", bed.origin.Head, "-m", "parked")
	bed.git("update-ref", "refs/metasystem/missions/demo/"+smuggled, smuggled)
	violation := bed.judge(nil, bed.preTree, nil)
	if !strings.Contains(violation, "anchors are trees") {
		t.Fatalf("a commit in the anchor namespace must violate: %q", violation)
	}
}

// A raw reviewed tree from an OLD base is not globally
// accounted — committing it on the first-parent chain reverts later
// state and refuses — while the same tree stays lawful as a merge side
// tip (the side-tip lane).
func TestScopeReviewedTreeIsNotGloballyAccounted(t *testing.T) {
	bed := newScopeBed(t)
	root := bed.engine.Root
	// History: v0 of y, then v1 (the open origin).
	writeText(t, filepath.Join(root, "y.go"), "package v0\n")
	bed.git("add", "y.go")
	bed.git("commit", "-qm", "y v0")
	oldBase := bed.git("rev-parse", "HEAD")
	writeText(t, filepath.Join(root, "y.go"), "package v1\n")
	bed.git("add", "y.go")
	bed.git("commit", "-qm", "y v1")
	bed.captureOrigin()
	pre, err := wallSnapshot(bed.ws, "demo")
	if err != nil {
		t.Fatal(err)
	}
	bed.preTree = pre
	// The reviewed tree: OLD base (y=v0) plus a patch to x only.
	bed.git("checkout", "-q", "--detach", oldBase)
	writeText(t, filepath.Join(root, "x.go"), "package x\n")
	bed.git("add", "x.go")
	bed.git("commit", "-qm", "reviewed work on the old base")
	reviewedCommit := bed.git("rev-parse", "HEAD")
	reviewed, err := bed.ws.TreeOf(reviewedCommit)
	if err != nil {
		t.Fatal(err)
	}
	oldBaseTree, err := bed.ws.TreeOf(oldBase)
	if err != nil {
		t.Fatal(err)
	}
	auth := bed.scopeAuthFor(oldBaseTree, reviewed)
	bed.git("checkout", "-q", "main")
	bed.git("reset", "-q", "--hard", bed.origin.Head)
	// Committing the raw reviewed tree reverts y to v0: refused.
	burned := bed.git("commit-tree", reviewed, "-p", bed.origin.Head, "-m", "raw reviewed tree")
	bed.git("update-ref", "refs/heads/main", burned)
	violation := bed.judge([]scopeAuth{auth}, bed.preTree, nil)
	if !strings.Contains(violation, "unaccounted tree") {
		t.Fatalf("a raw reviewed tree on the first-parent chain must refuse: %q", violation)
	}
}

// The launch context skips post-verification entries: a concluded turn
// announces no reconciliation and keeps its session.
func TestPriorContextSkipsVerificationEntries(t *testing.T) {
	log := []any{
		map[string]any{"turnId": "t1", "outcome": "completed", "sessionId": "s-1"},
		map[string]any{"turnId": "t1", "kind": "wall-verification", "capturedAt": "2026-01-01T00:00:00Z", "verdict": "clean"},
	}
	session, reconciliation, failures := PriorContext(log)
	if session != "s-1" || reconciliation || failures != 0 {
		t.Fatalf("verification entries poisoned the launch context: %v %v %d", session, reconciliation, failures)
	}
}

// Deleting the state-anchors ref during a live (chained)
// mission is a runner-ref deletion — a violation.
func TestScopeStateAnchorDeletionViolates(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state := readTestDoc(t, statePath)
	// Delete BOTH the birth record and the state-anchors ref: the
	// discriminator is the tamper-resistant hash chain, not a deletable
	// file, so the deletion still violates.
	_ = os.Remove(engine.birthRecordPath())
	if _, code := runIn(t, engine.Root, "update-ref", "-d", "refs/metasystem/missions/"+engine.Mission+"/state-anchors"); code != 0 {
		t.Fatal("could not delete the state-anchors ref for the probe")
	}
	violation, err := engine.judgeMissionNamespace(map[string]string{}, "", state)
	if err != nil || !strings.Contains(violation, "state-anchors was deleted") {
		t.Fatalf("state-anchors deletion must violate: %q %v", violation, err)
	}
}

// runIn is a raw git helper reporting the exit code.
func runIn(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return string(out), exit.ExitCode()
		}
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out), 0
}

// A host that force-commits arbitrary bytes into the filtered
// ledger path smuggles them onto the branch; the ledger-carrier check
// refuses it even though the tree identity filters the path.
func TestScopeLedgerCarrierSmugglingViolates(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state := readTestDoc(t, statePath)
	capture, err := engine.captureWallPosture(state["openTurn"].(map[string]any)["preTree"].(string), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Baseline: the clean bed passes the carrier check.
	if violation, err := engine.judgeLedgerCarriers(capture, state); err != nil || violation != "" {
		t.Fatalf("clean bed ledger carriers violated: %q %v", violation, err)
	}
	// Force an arbitrary ledger blob into the index and commit it.
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", engine.Root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=h", "GIT_AUTHOR_EMAIL=h@h",
			"GIT_COMMITTER_NAME=h", "GIT_COMMITTER_EMAIL=h@h")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	ledgerRel := missionLedgerRel(engine.Mission)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(engine.Root, ledgerRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(engine.Root, ledgerRel+".smuggle"), "arbitrary unreviewed bytes\n")
	blob := func() string {
		cmd := exec.Command("git", "-C", engine.Root, "hash-object", "-w", filepath.Join(engine.Root, ledgerRel+".smuggle"))
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}()
	git("update-index", "--add", "--cacheinfo", "100644,"+blob+","+ledgerRel)
	capture2, err := engine.captureWallPosture(state["openTurn"].(map[string]any)["preTree"].(string), nil)
	if err != nil {
		t.Fatal(err)
	}
	violation, err := engine.judgeLedgerCarriers(capture2, state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(violation, "unauthorized mission-ledger entry") {
		t.Fatalf("staged ledger smuggle must violate: %q", violation)
	}
	// And a COMMITTED smuggle on the first-parent chain is caught
	// per-commit, even if a later commit removes it from HEAD.
	git("-c", "user.name=h", "-c", "user.email=h@h", "commit", "-qm", "smuggle the ledger blob")
	smuggleCommit := strings.TrimSpace(func() string {
		out, _ := exec.Command("git", "-C", engine.Root, "rev-parse", "HEAD").Output()
		return string(out)
	}())
	if v, err := engine.judgeCommitLedgerCarrier(smuggleCommit, state); err != nil || !strings.Contains(v, "unauthorized mission-ledger entry") {
		t.Fatalf("committed ledger smuggle must violate per-commit: %q %v", v, err)
	}
	// The accounted/reviewed lanes judge the RAW ledger entry
	// before the workspace filter — a side tip or pseudoref cargo commit
	// carrying the foreign blob refuses even though the filtered tree
	// would read as accounted.
	preTree, _ := state["openTurn"].(map[string]any)["preTree"].(string)
	acct, err := engine.newWallAccountant(preTree, state, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := acct.accountedOrReviewedCommit(smuggleCommit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(detail, "unauthorized mission-ledger entry") {
		t.Fatalf("side-tip ledger smuggle must violate before the filter: %q", detail)
	}
}

// requiredAnchorTrees names the OPEN turn's topTree and
// topStaged.tree — ExpectedTreePoints excludes the open turn, so without
// this their anchors' deletion would go unseen.
func TestScopeRequiredAnchorTreesIncludesOpenTurn(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "demo"}
	topTree := strings.Repeat("1", 40)
	stagedTree := strings.Repeat("2", 40)
	state := map[string]any{
		"initialBaseline": strings.Repeat("0", 40),
		"turnLog":         []any{},
		"workspaceTaint":  map[string]any{"next": 1, "segment": 0, "entries": []any{}},
		"openTurn": map[string]any{
			"topTree":   topTree,
			"topStaged": map[string]any{"tree": stagedTree, "unmerged": []any{}},
		},
	}
	trees := engine.requiredAnchorTrees(state)
	set := map[string]bool{}
	for _, tree := range trees {
		set[tree] = true
	}
	if !set[topTree] || !set[stagedTree] {
		t.Fatalf("open-turn top trees missing from required set: %v", trees)
	}
}

// mustAccountant builds the snapshot-scope accountant for a bed's judge
// calls that need it (the consumed-carrier accounting lane).
func mustAccountant(t *testing.T, bed *scopeBed) *wallAccountant {
	t.Helper()
	acct, err := bed.engine.newWallAccountant(bed.preTree, bed.state, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return acct
}
