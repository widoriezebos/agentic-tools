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

// A clean bed passes every rule.
func TestScopeCleanBedPasses(t *testing.T) {
	bed := newScopePolicyBed(t)
	open := bed.posture("open", bed.pre)
	bed.captureOrigin(open)
	if violation := bed.judge(open, bed.expectCensus); violation != "" {
		t.Fatalf("clean bed violated: %s", violation)
	}
}

// A HEAD retreat is a violation.
func TestScopeHeadRetreatViolates(t *testing.T) {
	bed := newScopePolicyBed(t)
	bed.captureOrigin(bed.posture("second", "tree-second"))
	violation := bed.judge(bed.posture("first", bed.pre), func() {
		bed.expectGit("", "rev-list", "--first-parent", "first", "--not", "second")
	})
	if !strings.Contains(violation, "retreated or rewrote history") {
		t.Fatalf("retreat must violate: %q", violation)
	}
}

// An amend of pre-open history leaves the open commit off the
// first-parent chain.
func TestScopeAmendViolates(t *testing.T) {
	bed := newScopePolicyBed(t)
	bed.captureOrigin(bed.posture("open", bed.pre))
	violation := bed.judge(bed.posture("amended", bed.pre), func() {
		bed.expectGit("amended\n", "rev-list", "--first-parent", "amended", "--not", "open")
		bed.facts.add("Git", scopePolicyGit{code: 1}, bed.engine.Root,
			[]string{"rev-parse", "--verify", "--quiet", "amended^1"})
	})
	if !strings.Contains(violation, "retreated or rewrote history") {
		t.Fatalf("amend must violate: %q", violation)
	}
}

// A commit carrying an unaccounted tree names itself.
func TestScopeUnaccountedCommitViolates(t *testing.T) {
	bed := newScopePolicyBed(t)
	bed.captureOrigin(bed.posture("open", bed.pre))
	violation := bed.judge(bed.posture("smuggled", "tree-smuggled"), func() {
		bed.expectFirstParent("smuggled", "smuggled", "open")
		bed.expectCommitTree("smuggled", "tree-smuggled")
		bed.facts.add("ChangedPaths", []string{"smuggled.go"}, bed.pre, "tree-smuggled")
		bed.expectLedgerGuard()
		bed.expectCommitTree("smuggled", "tree-smuggled")
	})
	if !strings.Contains(violation, "advances HEAD with an unaccounted tree") ||
		!strings.Contains(violation, "commit smuggled") || !strings.Contains(violation, "smuggled.go") {
		t.Fatalf("unaccounted commit must violate: %q", violation)
	}
}

// An empty commit moves no byte and is lawful — accounting
// is by content, not ceremony. HEAD unmoved is lawful too.
func TestScopeEmptyCommitAndNoCommitLawful(t *testing.T) {
	bed := newScopePolicyBed(t)
	bed.captureOrigin(bed.posture("open", bed.pre))
	if violation := bed.judge(bed.posture("open", bed.pre), bed.expectCensus); violation != "" {
		t.Fatalf("no-commit turn violated: %q", violation)
	}
	if violation := bed.judge(bed.posture("empty", bed.pre), func() {
		bed.expectFirstParent("empty", "empty", "open")
		bed.expectCommitTree("empty", bed.pre)
		bed.expectLedgerGuard()
		bed.expectCommitTree("empty", bed.pre)
		bed.expectGit("empty open\n", "rev-list", "--parents", "-n", "1", "empty")
		bed.expectCensus()
	}); violation != "" {
		t.Fatalf("empty commit violated: %q", violation)
	}
}

// A commit of the expected composition is lawful, and a
// commit of an intermediate subset (one whole patch of two) is lawful.
func TestScopeSubsetAndFullCompositionCommitsLawful(t *testing.T) {
	bed := newScopeCompositionBed(t, "pre")
	bed.captureOrigin("origin", bed.pre, bed.refs("origin", ""))
	one := map[string]gittree.Entry{"one.go": wallPolicyFile}
	two := map[string]gittree.Entry{"two.go": wallPolicyOtherFile}
	authOne := bed.authorization("pre", "subset", strings.Repeat("d", 64), []string{"one.go"}, one)
	authTwo := bed.authorization("subset", "full", strings.Repeat("e", 64), []string{"two.go"}, two)
	auths := []scopeAuth{authOne, authTwo}

	bed.expectCapture("fullCommit", "full", "full", "full", bed.refs("fullCommit", ""), []string{})
	bed.expectAccountantAndGuard()
	bed.expectGit("fullCommit\n", "rev-list", "--first-parent", "fullCommit", "--not", "origin")
	bed.expectGit("origin\n", "rev-parse", "--verify", "--quiet", "fullCommit^1")
	bed.expectCommitTree("fullCommit", "full")
	bed.expectNoLedger()
	bed.expectCommitTree("fullCommit", "full")
	bed.expectGit("fullCommit origin\n", "rev-list", "--parents", "-n", "1", "fullCommit")
	bed.facts.add("TopLevel", bed.engine.Root)
	if violation := bed.judge(auths, "full"); violation != "" {
		t.Fatalf("full composition commit violated: %q", violation)
	}

	bed.expectCapture("restCommit", "full", "full", "full", bed.refs("restCommit", ""), []string{})
	bed.expectAccountantAndGuard()
	bed.expectGit("restCommit\nsubsetCommit\n", "rev-list", "--first-parent", "restCommit", "--not", "origin")
	bed.expectGit("origin\n", "rev-parse", "--verify", "--quiet", "subsetCommit^1")
	bed.expectCommitTree("restCommit", "full")
	bed.expectNoLedger()
	bed.expectCommitTree("subsetCommit", "subset")
	bed.facts.add("ChangedPaths", []string{"one.go"}, "pre", "subset")
	bed.expectEntries("subset", []string{"one.go"}, one)
	bed.expectGit("subsetCommit origin\n", "rev-list", "--parents", "-n", "1", "subsetCommit")
	bed.expectNoLedger()
	bed.expectCommitTree("restCommit", "full")
	bed.expectGit("restCommit subsetCommit\n", "rev-list", "--parents", "-n", "1", "restCommit")
	bed.facts.add("TopLevel", bed.engine.Root)
	if violation := bed.judge(auths, "full"); violation != "" {
		t.Fatalf("subset-state commit violated: %q", violation)
	}
}

// A partially-carried patch (one hunk of a reviewed change)
// violates the whole-patch rule.
func TestScopePartialPatchCommitViolates(t *testing.T) {
	bed := newScopeCompositionBed(t, "pre")
	bed.captureOrigin("origin", bed.pre, bed.refs("origin", ""))
	auth := bed.authorization("pre", "full", strings.Repeat("d", 64), []string{"a.go", "b.go"},
		map[string]gittree.Entry{"a.go": wallPolicyFile, "b.go": wallPolicyOtherFile})
	bed.expectCapture("halfCommit", "half", "half", "full", bed.refs("halfCommit", ""), []string{})
	bed.expectAccountantAndGuard()
	bed.expectGit("halfCommit\n", "rev-list", "--first-parent", "halfCommit", "--not", "origin")
	bed.expectGit("origin\n", "rev-parse", "--verify", "--quiet", "halfCommit^1")
	bed.expectCommitTree("halfCommit", "half")
	bed.facts.add("ChangedPaths", []string{"a.go"}, "pre", "half")
	bed.expectEntries("half", []string{"a.go", "b.go"}, map[string]gittree.Entry{"a.go": wallPolicyFile})
	bed.expectNoLedger()
	bed.expectCommitTree("halfCommit", "half")
	violation := bed.judge([]scopeAuth{auth}, "full")
	if !strings.Contains(violation, "advances HEAD with an unaccounted tree") {
		t.Fatalf("partial patch must violate: %q", violation)
	}
}

// The side-tip lane: a reviewed side branch integrated with --no-ff is
// lawful; the same branch fast-forwarded puts intermediate trees on the
// first-parent chain and the violation names the remedy.
func TestScopeMergeIntegrationLawfulAndFastForwardNamesRemedy(t *testing.T) {
	bed := newScopeCompositionBed(t, "pre")
	bed.captureOrigin("origin", bed.pre, bed.refs("origin", ""))
	jobsDir := jobsDirPath(bed.engine.Root)
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(jobsDir, "job-x.json"),
		map[string]any{"jobId": "job-x", "branch": "agent/job-x", "mission": "demo", "role": "implementer"})
	auth := bed.authorization("pre", "reviewed", strings.Repeat("d", 64), []string{"draft.go"},
		map[string]gittree.Entry{"draft.go": wallPolicyFile})
	sideRefs := bed.refs("mergeCommit", "sideTip")

	bed.expectCapture("mergeCommit", "reviewed", "reviewed", "reviewed", sideRefs, []string{})
	bed.expectAccountantAndGuard()
	bed.expectGit("mergeCommit\n", "rev-list", "--first-parent", "mergeCommit", "--not", "origin")
	bed.expectGit("origin\n", "rev-parse", "--verify", "--quiet", "mergeCommit^1")
	bed.expectCommitTree("mergeCommit", "reviewed")
	bed.expectNoLedger()
	bed.expectCommitTree("mergeCommit", "reviewed")
	bed.expectGit("mergeCommit origin sideTip\n", "rev-list", "--parents", "-n", "1", "mergeCommit")
	bed.expectCommitTree("sideTip", "reviewed")
	bed.facts.add("TopLevel", bed.engine.Root)
	if violation := bed.judge([]scopeAuth{auth}, "reviewed"); violation != "" {
		t.Fatalf("no-ff integration violated: %q", violation)
	}

	bed.expectCapture("sideTip", "reviewed", "reviewed", "reviewed", bed.refs("sideTip", "sideTip"), []string{})
	bed.expectAccountantAndGuard()
	bed.expectGit("sideTip\nintermediate\n", "rev-list", "--first-parent", "sideTip", "--not", "origin")
	bed.expectGit("origin\n", "rev-parse", "--verify", "--quiet", "intermediate^1")
	bed.expectCommitTree("sideTip", "reviewed")
	bed.expectNoLedger()
	bed.expectCommitTree("intermediate", "intermediateTree")
	bed.facts.add("ChangedPaths", []string{"draft.go"}, "pre", "intermediateTree")
	bed.expectEntries("intermediateTree", []string{"draft.go"},
		map[string]gittree.Entry{"draft.go": wallPolicyOtherFile})
	violation := bed.judge([]scopeAuth{auth}, "reviewed")
	if !strings.Contains(violation, "--no-ff") {
		t.Fatalf("fast-forward must name the remedy: %q", violation)
	}
}

// An ours-merge burying an illicit side tip fails by its tip.
func TestScopeOursMergeBuriedCommitViolates(t *testing.T) {
	bed := newScopeCompositionBed(t, "pre")
	bed.captureOrigin("origin", bed.pre, bed.refs("origin", ""))
	bed.expectCapture("oursMerge", bed.pre, bed.pre, bed.pre, bed.refs("oursMerge", ""), []string{})
	bed.expectAccountantAndGuard()
	bed.expectGit("oursMerge\n", "rev-list", "--first-parent", "oursMerge", "--not", "origin")
	bed.expectGit("origin\n", "rev-parse", "--verify", "--quiet", "oursMerge^1")
	bed.expectCommitTree("oursMerge", bed.pre)
	bed.expectNoLedger()
	bed.expectCommitTree("oursMerge", bed.pre)
	bed.expectGit("oursMerge origin illicit\n", "rev-list", "--parents", "-n", "1", "oursMerge")
	bed.expectCommitTree("illicit", "illicitTree")
	bed.facts.add("ChangedPaths", []string{"payload.go"}, "pre", "illicitTree")
	violation := bed.judge(nil, bed.pre)
	if !strings.Contains(violation, "merge side tip") {
		t.Fatalf("ours-merge burial must violate by its tip: %q", violation)
	}
}

// Tag, custom-namespace, and remote-namespace retention all
// violate the exact transition fence.
func TestScopeRefRetentionViolates(t *testing.T) {
	for _, ref := range []string{"refs/tags/keeper", "refs/custom/hideout", "refs/remotes/origin/x"} {
		t.Run(ref, func(t *testing.T) {
			bed := newScopePolicyBed(t)
			open := bed.posture("open", bed.pre)
			bed.captureOrigin(open)
			live := bed.posture("open", bed.pre)
			live.refs[ref] = "open"
			violation := bed.judge(live, nil)
			if !strings.Contains(violation, ref) || !strings.Contains(violation, "created") {
				t.Fatalf("retention under %s must violate: %q", ref, violation)
			}
		})
	}
}

// A same-tip detach violates — the active branch must BE the
// checkout.
func TestScopeSameTipDetachViolates(t *testing.T) {
	bed := newScopePolicyBed(t)
	bed.captureOrigin(bed.posture("open", bed.pre))
	live := bed.posture("open", bed.pre)
	live.detached, live.branch = true, ""
	violation := bed.judge(live, nil)
	if !strings.Contains(violation, "left the mission branch") {
		t.Fatalf("same-tip detach must violate: %q", violation)
	}
}

// An implementer branch moves freely while unconsumed and holds
// still from consumption on.
func TestScopeAgentBranchFreeThenHeld(t *testing.T) {
	bed := newScopePolicyBed(t)
	root := bed.engine.Root
	jobsDir := jobsDirPath(root)
	os.MkdirAll(jobsDir, 0o755)
	writeJSONFile(t, filepath.Join(jobsDir, "job-1.json"),
		map[string]any{"jobId": "job-1", "branch": "agent/job-1", "mission": "demo", "role": "implementer"})
	open := bed.posture("open", bed.pre)
	open.refs["refs/heads/agent/job-1"] = "open"
	bed.captureOrigin(open)
	live := bed.posture("open", bed.pre)
	live.refs["refs/heads/agent/job-1"] = "delegate"
	bed.expectCapture(live, bed.pre)
	capture, err := bed.engine.captureWallPosture(bed.pre, nil)
	if err != nil {
		t.Fatal(err)
	}
	bed.expectAccountant()
	acct, err := bed.engine.newWallAccountant(bed.pre, bed.state, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Unconsumed: free.
	if violation, err := bed.engine.judgeRefFence(bed.origin, capture, bed.state, map[string]bool{}, acct); err != nil || violation != "" {
		t.Fatalf("unconsumed branch motion must be free: %q %v", violation, err)
	}
	// Consumed: stationary.
	violation, err := bed.engine.judgeRefFence(bed.origin, capture, bed.state, map[string]bool{"job-1": true}, acct)
	if err != nil || !strings.Contains(violation, "moved after consumption") {
		t.Fatalf("consumed branch motion must violate: %q %v", violation, err)
	}
}

// Pseudoref retention — an unaccounted commit parked in
// REBASE_HEAD violates; ORIG_HEAD at an accounted commit is lawful
// (exactly what a lawful --no-ff integration leaves).
func TestScopePseudorefRetention(t *testing.T) {
	bed := newScopeCarrierBed(t)
	bed.captureOrigin()
	// Lawful: ORIG_HEAD at the accounted open commit.
	orig := gittree.Pseudoref{Name: "ORIG_HEAD", OIDs: []string{carrierOpen}, Parseable: true}
	posture := scopeCarrierPosture{staged: carrierPreRaw, post: carrierPre,
		census: []gittree.WorktreeRecord{bed.main(orig)}}
	if violation := bed.judge(posture, nil, carrierPre, func() {
		bed.facts.add("TopLevel", bed.root)
		bed.facts.add("Git", scopePolicyGit{stdout: "commit\n"}, bed.root, []string{"cat-file", "-t", carrierOpen})
		bed.facts.add("TreeOf", carrierPreRaw, carrierOpen)
		bed.facts.add("FilterTree", carrierPre, carrierPreRaw, []string{missionLedgerRel("demo")})
	}); violation != "" {
		t.Fatalf("accounted ORIG_HEAD violated: %q", violation)
	}
	// Violation: an unaccounted commit parked in REBASE_HEAD.
	const child = "hidden-child-of-open"
	const hiddenRaw = "raw-hidden-tree"
	const hidden = "hidden-tree"
	posture.census = []gittree.WorktreeRecord{bed.main(orig,
		gittree.Pseudoref{Name: "REBASE_HEAD", OIDs: []string{child}, Parseable: true})}
	violation := bed.judge(posture, nil, carrierPre, func() {
		bed.facts.add("TopLevel", bed.root)
		bed.facts.add("Git", scopePolicyGit{stdout: "commit\n"}, bed.root, []string{"cat-file", "-t", carrierOpen})
		bed.facts.add("TreeOf", carrierPreRaw, carrierOpen)
		bed.facts.add("FilterTree", carrierPre, carrierPreRaw, []string{missionLedgerRel("demo")})
		bed.facts.add("Git", scopePolicyGit{stdout: "commit\n"}, bed.root, []string{"cat-file", "-t", child})
		bed.facts.add("TreeOf", hiddenRaw, child)
		bed.facts.add("FilterTree", hidden, hiddenRaw, []string{missionLedgerRel("demo")})
		bed.facts.add("ChangedPaths", []string{"hidden.go"}, carrierPre, hidden)
	})
	if !strings.Contains(violation, "REBASE_HEAD") {
		t.Fatalf("REBASE_HEAD retention must violate: %q", violation)
	}
}

// An unrecorded worktree is a private carrier and violates
// outright; a runner-recorded measurement worktree at its recorded tip
// is lawful.
func TestScopeWorktreeCensus(t *testing.T) {
	bed := newScopeCarrierBed(t)
	bed.captureOrigin()
	linked := filepath.Join(bed.root, "private")
	if err := os.MkdirAll(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	linkedRecord := gittree.WorktreeRecord{Path: linked, HeadOID: carrierOpen, Detached: true,
		PostureReadable: true, Pseudorefs: []gittree.Pseudoref{{Name: "ORIG_HEAD", OIDs: []string{carrierOpen}, Parseable: true}},
		Staged: gittree.StagedPosture{Tree: carrierPreRaw}}
	posture := scopeCarrierPosture{staged: carrierPreRaw, post: carrierPre,
		census: []gittree.WorktreeRecord{bed.main(), linkedRecord}}
	violation := bed.judge(posture, nil, carrierPre, func() {
		bed.facts.add("TopLevel", bed.root)
	})
	if !strings.Contains(violation, "unrecorded worktree") {
		t.Fatalf("unrecorded worktree must violate: %q", violation)
	}
	// Record it as a measurement worktree at its tip: lawful.
	registry := measureWorktreeRecordsPath(bed.root)
	if err := os.MkdirAll(filepath.Dir(registry), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, registry, `{"path":"`+linked+`","sha":"`+carrierOpen+`"}`+"\n")
	if violation := bed.judge(posture, nil, carrierPre, func() {
		bed.facts.add("TopLevel", bed.root)
		bed.facts.add("Git", scopePolicyGit{}, bed.root,
			[]string{"merge-base", "--is-ancestor", carrierOpen, carrierOpen})
		bed.facts.add("Git", scopePolicyGit{stdout: carrierPreRaw + "\n"}, bed.root,
			[]string{"rev-parse", carrierOpen + "^{tree}"})
		bed.facts.add("ChangedPaths", []string{}, carrierPreRaw, carrierPreRaw)
	}); violation != "" {
		t.Fatalf("recorded measurement worktree violated: %q", violation)
	}
	// A recorded worktree off its recorded tip violates.
	linkedRecord.HeadOID = "same-tree-child-of-open"
	posture.census = []gittree.WorktreeRecord{bed.main(), linkedRecord}
	violation = bed.judge(posture, nil, carrierPre, func() {
		bed.facts.add("TopLevel", bed.root)
	})
	if !strings.Contains(violation, "left its recorded tip") {
		t.Fatalf("moved measurement worktree must violate: %q", violation)
	}
}

// Staged smuggling violates; a staged lawful subset and a revert pass.
func TestScopeStagedAccounting(t *testing.T) {
	bed := newScopeCarrierBed(t)
	bed.captureOrigin()
	root := bed.root
	// Smuggle: stage bytes never reviewed.
	writeText(t, filepath.Join(root, "staged.go"), "package staged\n")
	stagedMain := bed.main()
	stagedMain.Staged.Tree = carrierStagedRaw
	posture := scopeCarrierPosture{staged: carrierStagedRaw, post: carrierStagedTree,
		census: []gittree.WorktreeRecord{stagedMain}}
	violation := bed.judge(posture, nil, carrierPre, func() {
		bed.facts.add("TopLevel", bed.root)
		bed.facts.add("ChangedPaths", []string{"staged.go"}, carrierPre, carrierStagedTree)
	})
	if !strings.Contains(violation, "staged bytes unaccounted") {
		t.Fatalf("staged smuggle must violate: %q", violation)
	}
	// The same staged state under a consumed authorization is lawful.
	patch := []byte("diff --git a/staged.go b/staged.go\nnew file mode 100644\nindex 0000000..98a5a99\n--- /dev/null\n+++ b/staged.go\n@@ -0,0 +1 @@\n+package staged\n")
	digest := bed.authorization(carrierStagedTree, patch)
	entry := gittree.Entry{Mode: "100644", OID: "98a5a992ed2bc4b17d078d396ba034c8064079b4"}
	paths := []string{"staged.go"}
	entries := map[string]gittree.Entry{"staged.go": entry}
	bed.facts.add("Apply", carrierStagedTree, carrierPre, patch)
	bed.facts.add("Entries", entries, carrierStagedTree, paths)
	bed.facts.add("Entries", entries, carrierStagedTree, paths)
	bed.facts.add("Snapshot", carrierStagedRaw, "HEAD")
	bed.facts.add("FilterTree", carrierStagedTree, carrierStagedRaw, []string{missionLedgerRel("demo")})
	inspection, err := inspectWallWithWorkspace(bed.engine.wallWorkspace(root), root, "demo", carrierPre,
		bed.state, []map[string]any{{"verdict": "accepted", "authorizationDigest": digest}}, nil, nil, "",
		func(expected string) (string, error) {
			if expected != carrierStagedTree {
				t.Fatalf("inspection expected %q, want %q", expected, carrierStagedTree)
			}
			return wallSnapshotWithWorkspace(bed.engine.wallWorkspace(root), "demo")
		})
	if err != nil || inspection.Violation != "" || inspection.ExpectedTree != carrierStagedTree ||
		len(inspection.OrderedDigests) != 1 || inspection.OrderedDigests[0] != digest ||
		len(inspection.Auths) != 1 || inspection.Auths[0].reviewedEntries["staged.go"] != entry {
		t.Fatalf("authorization consumption: inspection=%+v, err=%v", inspection, err)
	}
	authDir := filepath.Join(missionDirPath(root, "demo"), "authorizations")
	t.Logf("consumed authorization record %s; patch %s",
		filepath.Join(authDir, digest+".json"), filepath.Join(authDir, digest+".patch"))
	if violation := bed.judge(posture, inspection.Auths, inspection.ExpectedTree, func() {
		bed.facts.add("TopLevel", bed.root)
		bed.facts.add("ChangedPaths", paths, carrierPre, carrierStagedTree)
		bed.facts.add("Entries", entries, carrierStagedTree, paths)
	}); violation != "" {
		t.Fatalf("lawful staged subset violated: %q", violation)
	}
	// A revert of the staged bytes (smuggle-then-revert) passes with no
	// auth: the projection equals the pre-tree again.
	if err := os.Remove(filepath.Join(root, "staged.go")); err != nil {
		t.Fatal(err)
	}
	posture.staged, posture.post = carrierPreRaw, carrierPre
	posture.census[0].Staged.Tree = carrierPreRaw
	if violation := bed.judge(posture, nil, carrierPre, func() {
		bed.facts.add("TopLevel", bed.root)
	}); violation != "" {
		t.Fatalf("reverted staged state violated: %q", violation)
	}
}

// A sibling edit mid-turn violates with the paths named; a
// workspace edit does not trip the toplevel fence.
func TestScopeSiblingEditViolates(t *testing.T) {
	bed := newStrictNestedScopeBed(t)
	open := nestedScopePosture{head: "origin", workspaceTree: bed.pre, topTree: "top-origin",
		topStaged: gittree.StagedPosture{Tree: "top-staged-origin"}}
	bed.captureOrigin(open)
	live := open
	live.topTree = "top-edited"
	bed.expectJudgeStart(live, "top-origin")
	bed.facts.add(bed.root, "TopLevel", bed.top) // first-parent scope
	bed.facts.add(bed.root, "TopLevel", bed.top) // census
	bed.facts.add(bed.root, "TopLevel", bed.top) // toplevel fence
	bed.facts.add(bed.top, "ChangedPaths", []string{"sibling/lib.go"}, open.topTree, live.topTree)
	violation := bed.judge(live)
	if !strings.Contains(violation, "sibling paths changed") || !strings.Contains(violation, "sibling/lib.go") {
		t.Fatalf("sibling edit must violate with the path named: %q", violation)
	}
}

// A host commit touching a sibling path violates at repository
// scope even when its workspace subtree is clean.
func TestScopeSiblingCommitViolates(t *testing.T) {
	bed := newStrictNestedScopeBed(t)
	open := nestedScopePosture{head: "origin", workspaceTree: bed.pre, topTree: "top-origin",
		topStaged: gittree.StagedPosture{Tree: "top-staged-origin"}}
	bed.captureOrigin(open)
	live := open
	live.head, live.topTree = "sibling-commit", "top-sibling-commit"
	bed.expectJudgeStart(live, "top-sibling-commit")
	bed.expectGit(live.head+"\n", "rev-list", "--first-parent", live.head, "--not", open.head)
	bed.expectGit(open.head+"\n", "rev-parse", "--verify", "--quiet", live.head+"^1")
	bed.facts.add(bed.root, "TopLevel", bed.top)                 // first-parent scope
	bed.facts.add(bed.root, "TreeOf", "raw-"+bed.pre, live.head) // tip accounting
	bed.facts.add(bed.root, "FilterTree", bed.pre, "raw-"+bed.pre, bed.ledgerPaths())
	bed.expectLedgerGuard() // commit ledger carrier
	bed.facts.add(bed.root, "TreeOf", "raw-"+bed.pre, live.head)
	bed.facts.add(bed.root, "FilterTree", bed.pre, "raw-"+bed.pre, bed.ledgerPaths())
	bed.expectGit(live.head+" "+open.head+"\n", "rev-list", "--parents", "-n", "1", live.head)
	bed.facts.add(bed.top, "ChangedPaths", []string{"sibling/lib.go"}, open.head+"^{tree}", live.head+"^{tree}")
	violation := bed.judge(live)
	if !strings.Contains(violation, "sibling paths changed") {
		t.Fatalf("sibling commit must violate: %q", violation)
	}
}

// A sibling payload buried in an interior side commit under an
// empty accounted tip is caught by the ACCUMULATED side-chain scope.
func TestScopeSiblingPayloadBuriedUnderAccountedTipViolates(t *testing.T) {
	bed := newStrictNestedScopeBed(t)
	open := nestedScopePosture{head: "pre-branch-base", workspaceTree: bed.pre, topTree: "top-base",
		topStaged: gittree.StagedPosture{Tree: "top-staged-base"}}
	bed.captureOrigin(open)
	// The side chain is base -> payload -> empty cover tip. The cover's
	// whole top tree still contains the payload, while its workspace tree
	// and immediate top delta are empty. An ours merge keeps the base tree.
	mainParent, merge := open.head, "ours-merge"
	payload := nestedScopeSideCommit{oid: "side-payload", parent: mainParent,
		topTree: "top-side-payload", workspaceTree: bed.pre}
	cover := nestedScopeSideCommit{oid: "empty-cover", parent: payload.oid,
		topTree: payload.topTree, workspaceTree: payload.workspaceTree}
	live := open
	live.head = merge
	bed.expectJudgeStart(live, "top-base")
	bed.expectGit(merge+"\n", "rev-list", "--first-parent", merge, "--not", open.head)
	bed.expectGit(mainParent+"\n", "rev-parse", "--verify", "--quiet", merge+"^1")
	bed.facts.add(bed.root, "TopLevel", bed.top)             // first-parent scope
	bed.facts.add(bed.root, "TreeOf", "raw-"+bed.pre, merge) // tip accounting
	bed.facts.add(bed.root, "FilterTree", bed.pre, "raw-"+bed.pre, bed.ledgerPaths())
	bed.expectLedgerGuard() // merge ledger carrier
	bed.facts.add(bed.root, "TreeOf", "raw-"+bed.pre, merge)
	bed.facts.add(bed.root, "FilterTree", bed.pre, "raw-"+bed.pre, bed.ledgerPaths())
	bed.expectGit(merge+" "+mainParent+" "+cover.oid+"\n", "rev-list", "--parents", "-n", "1", merge)
	bed.facts.add(bed.top, "ChangedPaths", []string{}, mainParent+"^{tree}", merge+"^{tree}")
	bed.facts.add(bed.root, "TreeOf", "raw-"+cover.workspaceTree, cover.oid)
	bed.facts.add(bed.root, "FilterTree", bed.pre, "raw-"+bed.pre, bed.ledgerPaths())
	bed.expectGit(payload.parent+"\n", "merge-base", mainParent, cover.oid)
	bed.facts.add(bed.top, "ChangedPaths", []string{"sibling/payload.go"}, mainParent+"^{tree}", cover.oid+"^{tree}")
	violation := bed.judge(live)
	if !strings.Contains(violation, "sibling") {
		t.Fatalf("buried sibling payload must violate: %q", violation)
	}
}

// Nested: staged sibling motion violates; a preexisting sibling
// conflict refuses nothing.
func TestScopeToplevelStagedMotion(t *testing.T) {
	bed := newStrictNestedScopeBed(t)
	open := nestedScopePosture{head: "origin", workspaceTree: bed.pre, topTree: "top-origin",
		topStaged: gittree.StagedPosture{Tree: "top-staged-origin"}}
	bed.captureOrigin(open)
	live := open
	live.topStaged = gittree.StagedPosture{Tree: "top-staged-sibling"}
	bed.expectJudgeStart(live, "top-origin")
	bed.facts.add(bed.root, "TopLevel", bed.top) // first-parent scope
	bed.facts.add(bed.root, "TopLevel", bed.top) // census
	bed.facts.add(bed.root, "TopLevel", bed.top) // staged fence
	bed.facts.add(bed.top, "ChangedPaths", []string{"sibling/staged.go"}, open.topStaged.Tree, live.topStaged.Tree)
	violation := bed.judge(live)
	if !strings.Contains(violation, "staged bytes unaccounted") || !strings.Contains(violation, "sibling") {
		t.Fatalf("staged sibling motion must violate: %q", violation)
	}
}

// The seeded projection follows the comparison target — a
// committed ignored declared artifact stays projected instead of
// reading as drift.
func TestScopeSeededCaptureFollowsExpected(t *testing.T) {
	bed := newScopeCompositionBed(t, "pre")
	bed.captureOrigin("ignoreCommit", bed.pre, bed.refs("ignoreCommit", ""))
	declared := map[string]bool{"out.bin": true}
	bed.expectCapture("ignoreCommit", bed.pre, "postWithOut", bed.pre, bed.refs("ignoreCommit", ""),
		[]string{"out.bin"})
	bed.expectEntries("postWithOut", []string{"out.bin"},
		map[string]gittree.Entry{"out.bin": wallPolicyFile})
	capture, err := bed.engine.captureWallPosture(bed.pre, declared)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := bed.engine.wallWorkspace(bed.engine.Root).Entries(capture.Post, []string{"out.bin"})
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
	bed := newVerificationFileBed(t)
	bed.accept()
	bed.expectCapture(recoveryHead)
	bed.expectLedgerCarrier()
	bed.expectConclusion()
	final, parked, err := bed.e.verifyAcceptance(bed.state, bed.ledger, "alpha-t1-live", bed.turnDir, 1, nil)
	if err != nil || parked {
		t.Fatalf("clean verification: parked=%v err=%v", parked, err)
	}
	if final["openTurn"] != nil {
		t.Fatalf("verification must conclude the open turn")
	}
	if pending := mission.UnverifiedAcceptance(final); pending != "" {
		t.Fatalf("verification entry missing: %q", pending)
	}
	bed.checkLedgerBytes()
	bed.checkAnchors(2)
}

// Motion after the acceptance write parks over the acceptance.
func TestScopeAcceptanceVerificationCatchesMotion(t *testing.T) {
	bed := newVerificationFileBed(t)
	bed.accept()
	accepted := bed.facts.stateDoc()
	entries := accepted["turnLog"].([]any)
	passedWall := entries[len(entries)-1].(map[string]any)["wall"].(map[string]any)
	writeJSONFile(t, filepath.Join(bed.turnDir, "wall.json"), passedWall)
	bed.expectCapture(recoveryPost)
	bed.expectLedgerCarrier()
	bed.expectPark()
	final, parked, err := bed.e.verifyAcceptance(bed.state, bed.ledger, "alpha-t1-live", bed.turnDir, 1, nil)
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
	evidence := readTestDoc(t, filepath.Join(bed.turnDir, "wall.json"))
	if evidence["verdict"] != "violated" {
		t.Fatalf("motion must publish a violated wall verdict: %v", evidence["verdict"])
	}
	if violation, _ := evidence["violation"].(string); !strings.Contains(violation, "post-verification") {
		t.Fatalf("wall evidence must name the verification window: %q", violation)
	}
	posture, _ := evidence["postureAtVerification"].(map[string]any)
	refs, _ := posture["refMapPost"].(map[string]any)
	if evidence["headCommitPost"] != recoveryHead || posture["headCommitPost"] != recoveryPost || refs["refs/heads/main"] != recoveryPost {
		t.Fatalf("wall evidence must retain accepted and changed postures: %v", evidence)
	}
	bed.checkAnchors(2)
}

// Admission REFUSES a nonempty replacement namespace — the real
// preflight path, not merely the ref's presence.
func TestScopeAdmissionRefusesReplaceNamespace(t *testing.T) {
	bed := newVerificationFileBed(t)
	approvedText, values, _, err := bed.e.parseContract(true)
	if err != nil {
		t.Fatal(err)
	}
	f := bed.facts
	root := bed.e.Root
	contract := bed.contractPath()
	exclude := []string{missionLedgerRel(bed.e.Mission), contract}
	entry := gittree.Entry{OID: recoveryPost, Mode: "100644"}
	f.add("Git", scopePolicyGit{stdout: "true\n"}, root,
		[]string{"config", "--local", "--type=bool", "--get", "core.fileMode"})
	f.add("Snapshot", recoveryPre, "HEAD")
	f.add("FilterTree", recoveryPre, recoveryPre, exclude)
	f.add("HeadTree", recoveryPre)
	f.add("FilterTree", recoveryPre, recoveryPre, exclude)
	f.add("Entries", map[string]gittree.Entry{contract: entry}, recoveryPre, []string{contract})
	f.add("Entries", map[string]gittree.Entry{contract: entry}, recoveryPre, []string{contract})
	f.add("BlobOID", recoveryPost, root, []byte(approvedText))
	f.add("RefMap", map[string]string{"refs/heads/main": recoveryHead,
		"refs/replace/" + recoveryHead: recoveryPost})
	if _, aerr := bed.e.admittedBaseline(values, []byte(approvedText)); aerr == nil ||
		!strings.Contains(aerr.Error(), "replacement namespace is not empty") {
		t.Fatalf("admission must refuse a nonempty replacement namespace, got %v", aerr)
	}
	bed.checkAnchors(0)
}

// RESTORE verifies every carrier — a staged remnant or an
// unaccounted commit refuses the restore even when the worktree equals
// the named safe tree — and the recorded resolution carries the full
// carrier posture as the next accounting origin.
func TestScopeRestoreVerifiesCarriers(t *testing.T) {
	bed, facts := newCarrierResolutionBed(t)
	engine := bed.e
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state := readTestDoc(t, statePath)
	openTurn := state["openTurn"].(map[string]any)
	preTree := openTurn["preTree"].(string)
	openHead := openTurn["headCommit"].(string)
	// The human restores the worktree but leaves smuggled STAGED bytes.
	if err := os.Remove(filepath.Join(engine.Root, "solo.go")); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(engine.Root, "staged-extra.go"), "package staged\n")
	if err := os.Remove(filepath.Join(engine.Root, "staged-extra.go")); err != nil {
		t.Fatal(err)
	}
	bed.snapshot(preTree)
	facts.add("Anchor", nil, engine.Mission, preTree)
	bed.snapshot(preTree)
	facts.expectCapture(openHead, recoveryPre, recoveryPre, carrierPolicyStagedRaw, carrierPolicyStagedRaw, nil)
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restore", nil); code == 0 {
		t.Fatal("restore must refuse while staged bytes differ from the named tree")
	}
	// An unaccounted commit is a carrier the worktree restore cannot
	// un-ship: adoption or human git surgery first.
	writeText(t, filepath.Join(engine.Root, "committed-extra.go"), "package committed\n")
	if err := os.Remove(filepath.Join(engine.Root, "committed-extra.go")); err != nil {
		t.Fatal(err)
	}
	bed.snapshot(preTree)
	facts.add("Anchor", nil, engine.Mission, preTree)
	bed.snapshot(preTree)
	facts.expectCapture(carrierCommit, carrierCommitRaw, carrierCommitTree, recoveryPre, recoveryPre, nil)
	facts.expectAccountant()
	facts.expectLedgerGuard(carrierCommit, carrierCommitRaw, recoveryPre)
	facts.add("Git", scopePolicyGit{stdout: carrierCommit + "\n"}, engine.Root,
		[]string{"rev-list", "--first-parent", carrierCommit, "--not", openHead})
	facts.add("Git", scopePolicyGit{stdout: openHead + "\n"}, engine.Root,
		[]string{"rev-parse", "--verify", "--quiet", carrierCommit + "^1"})
	facts.add("TreeOf", carrierCommitRaw, carrierCommit)
	facts.expectRawLedger(carrierCommitRaw)
	facts.add("FilterTree", carrierCommitTree, carrierCommitRaw, []string{missionLedgerRel(engine.Mission)})
	facts.add("ChangedPaths", []string{"committed-extra.go"}, preTree, carrierCommitTree)
	facts.expectOID()
	facts.add("TreeOf", carrierCommitRaw, carrierCommit)
	facts.expectRawLedger(carrierCommitRaw)
	facts.add("TreeOf", carrierCommitRaw, carrierCommit)
	facts.expectRawLedger(carrierCommitRaw)
	facts.add("FilterTree", carrierCommitTree, carrierCommitRaw, []string{missionLedgerRel(engine.Mission)})
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restore", nil); code == 0 {
		t.Fatal("restore must refuse while committed HEAD carries unaccounted commits")
	}
	// The reset itself parked the smuggled commit in ORIG_HEAD — a
	// retention carrier in its own right; the surgery clears it too.
	retained := []gittree.Pseudoref{{Name: "ORIG_HEAD", OIDs: []string{carrierCommit}, Parseable: true}}
	bed.snapshot(preTree)
	facts.add("Anchor", nil, engine.Mission, preTree)
	bed.snapshot(preTree)
	facts.expectCapture(openHead, recoveryPre, recoveryPre, recoveryPre, recoveryPre, retained)
	facts.expectAccountant()
	facts.expectLedgerGuard(openHead, recoveryPre, recoveryPre)
	facts.expectNamespace()
	facts.add("TopLevel", engine.Root)
	facts.add("Git", scopePolicyGit{stdout: "commit\n"}, engine.Root, []string{"cat-file", "-t", carrierCommit})
	facts.add("TreeOf", carrierCommitRaw, carrierCommit)
	facts.expectRawLedger(carrierCommitRaw)
	facts.add("FilterTree", carrierCommitTree, carrierCommitRaw, []string{missionLedgerRel(engine.Mission)})
	facts.add("ChangedPaths", []string{"committed-extra.go"}, preTree, carrierCommitTree)
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restore", nil); code == 0 {
		t.Fatal("restore must refuse while ORIG_HEAD retains the unaccounted commit")
	}
	bed.snapshot(preTree)
	facts.add("Anchor", nil, engine.Mission, preTree)
	bed.snapshot(preTree)
	facts.expectCapture(openHead, recoveryPre, recoveryPre, recoveryPre, recoveryPre, nil)
	facts.expectAccountant()
	facts.expectSafeJudge()
	bed.ledgerTruth()
	facts.expectCapture(openHead, recoveryPre, recoveryPre, recoveryPre, recoveryPre, nil)
	facts.expectLedgerGuard(openHead, recoveryPre, recoveryPre)
	facts.expectNamespace()
	bed.dropOpenHead()
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
	refs, _ := posture["refMapPost"].(map[string]any)
	census, _ := posture["worktreeCensusPost"].([]any)
	if len(census) != 1 {
		t.Fatalf("resolution census must name the restored checkout: %v", census)
	}
	checkout, _ := census[0].(map[string]any)
	staged, _ := checkout["staged"].(map[string]any)
	pseudorefs, _ := checkout["pseudorefs"].([]any)
	if len(posture) != 7 || len(refs) != 1 || posture["stagedTreePost"] != preTree ||
		refs["refs/heads/main"] != openHead || posture["capturedAt"] == "" ||
		posture["topTreePost"] != nil || posture["topStagedPost"] != nil ||
		checkout["path"] != engine.Root || checkout["headOid"] != openHead ||
		checkout["branch"] != "refs/heads/main" || checkout["postureReadable"] != true ||
		staged["tree"] != preTree || len(pseudorefs) != 0 || bed.pins != 1 {
		t.Fatalf("resolution did not pin the full safe carrier posture: %v", posture)
	}
}

// A grafts file forges the first-parent walk; its presence is
// a violation before any accounting runs.
func TestScopeGraftFileViolates(t *testing.T) {
	bed := newScopePolicyBed(t)
	bed.captureOrigin(bed.posture("open", bed.pre))
	live := bed.posture("open", bed.pre)
	live.steering = []string{"info/grafts"}
	violation := bed.judge(live, nil)
	if !strings.Contains(violation, "history-steering") {
		t.Fatalf("a grafts file must violate: %q", violation)
	}
}

// A commit parked under its own id in the mission's anchor
// namespace is not an anchor — anchors are trees.
func TestScopeCommitInAnchorNamespaceViolates(t *testing.T) {
	bed := newScopePolicyBed(t)
	bed.captureOrigin(bed.posture("open", bed.pre))
	live := bed.posture("open", bed.pre)
	smuggled := strings.Repeat("a", 40)
	live.refs["refs/metasystem/missions/demo/"+smuggled] = smuggled
	violation := bed.judge(live, func() { bed.expectGit("commit\n", "cat-file", "-t", smuggled) })
	if !strings.Contains(violation, "anchors are trees") {
		t.Fatalf("a commit in the anchor namespace must violate: %q", violation)
	}
}

// A raw reviewed tree from an OLD base is not globally
// accounted — committing it on the first-parent chain reverts later
// state and refuses — while the same tree stays lawful as a merge side
// tip (the side-tip lane).
func TestScopeReviewedTreeIsNotGloballyAccounted(t *testing.T) {
	const originTree = "origin-y-v1"
	const oldBaseTree = "old-base-y-v0"
	const reviewed = "reviewed-old-base-plus-x"
	bed := newScopeCompositionBed(t, originTree)
	bed.captureOrigin("origin", originTree, bed.refs("origin", ""))
	// The authorization changes only x on the old base. The open origin
	// has y=v1, so using the raw reviewed tree would revert y to v0.
	auth := bed.authorization(oldBaseTree, reviewed, strings.Repeat("d", 64),
		[]string{"x.go"}, map[string]gittree.Entry{"x.go": wallPolicyFile})
	bed.expectCapture("burned", reviewed, reviewed, originTree, bed.refs("burned", ""), []string{})
	bed.expectAccountantAndGuard()
	bed.expectGit("burned\n", "rev-list", "--first-parent", "burned", "--not", "origin")
	bed.expectGit("origin\n", "rev-parse", "--verify", "--quiet", "burned^1")
	bed.expectCommitTree("burned", reviewed)
	bed.facts.add("ChangedPaths", []string{"x.go", "y.go"}, originTree, reviewed)
	bed.expectNoLedger()
	bed.expectCommitTree("burned", reviewed)
	violation := bed.judge([]scopeAuth{auth}, originTree)
	if !strings.Contains(violation, "unaccounted tree") {
		t.Fatalf("a raw reviewed tree on the first-parent chain must refuse: %q", violation)
	}

	// The same reviewed tree remains lawful on a merge side tip. Only
	// first-parent commits must account for the full open-origin tree.
	writeJSONFile(t, filepath.Join(jobsDirPath(bed.engine.Root), "job-x.json"),
		map[string]any{"jobId": "job-x", "branch": "agent/job-x", "mission": "demo", "role": "implementer"})
	bed.expectCapture("merge", originTree, originTree, originTree, bed.refs("merge", "sideTip"), []string{})
	bed.expectAccountantAndGuard()
	bed.expectGit("merge\n", "rev-list", "--first-parent", "merge", "--not", "origin")
	bed.expectGit("origin\n", "rev-parse", "--verify", "--quiet", "merge^1")
	bed.expectCommitTree("merge", originTree)
	bed.expectNoLedger()
	bed.expectCommitTree("merge", originTree)
	bed.expectGit("merge origin sideTip\n", "rev-list", "--parents", "-n", "1", "merge")
	bed.expectCommitTree("sideTip", reviewed)
	bed.facts.add("TopLevel", bed.engine.Root)
	if violation := bed.judge([]scopeAuth{auth}, originTree); violation != "" {
		t.Fatalf("the reviewed old-base tree must remain lawful as a merge side tip: %q", violation)
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
	bed := newRecoveryFileBed(t)
	engine := bed.e
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state := readTestDoc(t, statePath)
	// The birth record is gone and the namespace input has no state-anchors
	// ref. The surviving state hash chain still proves prior publication.
	writeJSONFile(t, engine.birthRecordPath(), map[string]any{"missionId": engine.Mission})
	if err := os.Remove(engine.birthRecordPath()); err != nil {
		t.Fatalf("remove the birth record: %v", err)
	}
	if _, err := os.Stat(engine.birthRecordPath()); !os.IsNotExist(err) {
		t.Fatalf("birth record remains after deletion: %v", err)
	}
	if _, hash, err := mission.VerifyStateShape(statePath); err != nil || hash == "" {
		t.Fatalf("state must remain hash-chained after birth-record removal: %s %v", hash, err)
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
	bed, facts := newCarrierLedgerBed(t)
	engine := bed.e
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state := readTestDoc(t, statePath)
	preTree := state["openTurn"].(map[string]any)["preTree"].(string)
	if _, _, _, err := mission.ParseLedger(bed.ledger); err != nil {
		t.Fatal(err)
	}
	facts.expectCapture(recoveryHead, recoveryPre, recoveryPre, recoveryPre)
	capture, err := engine.captureWallPosture(preTree, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Baseline: the clean bed passes the carrier check.
	facts.expectLedgerOID()
	facts.add("TreeOf", recoveryPre, recoveryHead)
	facts.add("StagedTree", recoveryPre)
	for i := 0; i < 2; i++ {
		facts.add("Entries", map[string]gittree.Entry{}, recoveryPre, []string{missionLedgerRel(engine.Mission)})
	}
	if violation, err := engine.judgeLedgerCarriers(capture, state); err != nil || violation != "" {
		t.Fatalf("clean bed ledger carriers violated: %q %v", violation, err)
	}
	// The raw staged tree carries a foreign regular-file ledger entry while
	// the ordinary ledger file keeps its authenticated bytes.
	ledgerRel := missionLedgerRel(engine.Mission)
	foreign := []byte("arbitrary unreviewed bytes\n")
	writeText(t, filepath.Join(t.TempDir(), "foreign-ledger"), string(foreign))
	facts.foreignOID = carrierBlobOID(foreign)
	if facts.foreignOID == facts.anchorOID {
		t.Fatal("foreign ledger blob equals anchor")
	}
	facts.foreignTree = carrierLedgerRaw
	facts.expectCapture(recoveryHead, recoveryPre, carrierLedgerRaw, recoveryPre)
	capture2, err := engine.captureWallPosture(preTree, nil)
	if err != nil {
		t.Fatal(err)
	}
	facts.expectLedgerOID()
	facts.add("TreeOf", recoveryPre, recoveryHead)
	facts.add("StagedTree", carrierLedgerRaw)
	facts.add("CarrierLedgerEntries", nil, recoveryPre, carrierLedgerRaw, []string{ledgerRel})
	violation, err := engine.judgeLedgerCarriers(capture2, state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(violation, "unauthorized mission-ledger entry") {
		t.Fatalf("staged ledger smuggle must violate: %q", violation)
	}
	if !facts.stageProbe || !strings.Contains(violation, "the index") {
		t.Fatalf("staged ledger entry was not judged: %q", violation)
	}
	facts.foreignTree = ""
	// And a COMMITTED smuggle on the first-parent chain is caught
	// per-commit, even if a later commit removes it from HEAD.
	smuggleCommit := carrierLedgerCommit
	foreignEntry := map[string]gittree.Entry{ledgerRel: {OID: facts.foreignOID, Mode: "100644"}}
	facts.expectLedgerOID()
	facts.add("TreeOf", carrierLedgerRaw, smuggleCommit)
	facts.add("Entries", foreignEntry, carrierLedgerRaw, []string{ledgerRel})
	if v, err := engine.judgeCommitLedgerCarrier(smuggleCommit, state); err != nil || !strings.Contains(v, "unauthorized mission-ledger entry") {
		t.Fatalf("committed ledger smuggle must violate per-commit: %q %v", v, err)
	}
	// The accounted/reviewed lanes judge the RAW ledger entry
	// before the workspace filter — a side tip or pseudoref cargo commit
	// carrying the foreign blob refuses even though the filtered tree
	// would read as accounted.
	facts.add("Prefix", "")
	facts.expectLedgerOID()
	acct, err := engine.newWallAccountant(preTree, state, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	facts.add("TreeOf", carrierLedgerRaw, smuggleCommit)
	facts.add("Entries", foreignEntry, carrierLedgerRaw, []string{ledgerRel})
	detail, err := acct.accountedOrReviewedCommit(smuggleCommit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(detail, "unauthorized mission-ledger entry") {
		t.Fatalf("side-tip ledger smuggle must violate before the filter: %q", detail)
	}
	if _, _, _, err := mission.ParseLedger(bed.ledger); err != nil {
		t.Fatalf("unchanged ledger stopped parsing: %v", err)
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
