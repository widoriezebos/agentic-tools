package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// A minimal shippable workspace for the tree equation: one tracked file,
// one commit, artifacts/ ignored — the same projection the runner sees.
func wallRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	writeText(t, filepath.Join(root, ".gitignore"), "artifacts/\n")
	writeText(t, filepath.Join(root, "main.go"), "package main\n")
	writeText(t, filepath.Join(root, "metasystem.conf"), "metasystem.runtimes=fake\n")
	run("add", "-A")
	run("commit", "-qm", "baseline")
	return root
}

func writeText(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// wallState is the minimal mission state a wall inspection reads: no
// acceptance entries yet, so the current sequence point is {0, 0}.
func wallState() map[string]any {
	return map[string]any{"turnLog": []any{}}
}

// wallAuthorization mints a SELF-CONSISTENT authorization for the diff
// between two trees: record + patch bytes exactly as issuance writes them,
// canonically digested AFTER any mutation — an attacker can always
// self-digest forged content, which is precisely why the wall's semantic
// checks exist beyond authentication.
func wallAuthorization(t *testing.T, root, missionID, baseTree, reviewedTree string, mutate func(map[string]any)) string {
	t.Helper()
	workspace := gittree.Workspace{Dir: root}
	patch, err := workspace.Diff(baseTree, reviewedTree)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := workspace.ChangedPaths(baseTree, reviewedTree)
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]any, 0, len(changed))
	for _, p := range changed {
		paths = append(paths, p)
	}
	patchSum := sha256.Sum256(patch)
	record := map[string]any{
		"jobId": "job-w", "rootJob": "job-w", "mission": missionID,
		"baseTree": baseTree, "reviewedTree": reviewedTree,
		"baseSequencePoint": map[string]any{"sequence": 0, "segment": 0},
		"patchDigest":       hex.EncodeToString(patchSum[:]),
		"changedPaths":      paths, "supersedes": []any{},
	}
	if mutate != nil {
		mutate(record)
	}
	digest, err := validate.AuthorizationRecordDigest(record)
	if err != nil {
		t.Fatal(err)
	}
	record["authorizationDigest"] = digest
	dir := filepath.Join(missionDirPath(root, missionID), "authorizations")
	writeText(t, filepath.Join(dir, digest+".patch"), string(patch))
	writeJSONFile(t, filepath.Join(dir, digest+".json"), record)
	return digest
}

func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	tree, err := gittree.Workspace{Dir: root}.Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestWallPassesUntouchedWorkspace(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	// Machine metadata under artifacts/ is outside the projection.
	writeText(t, filepath.Join(root, "artifacts", "agents", "x.json"), "{}\n")
	inspection, err := inspectWall(root, "demo", pre, wallState(), nil, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" || inspection.PostTree != pre {
		t.Fatalf("untouched workspace must pass: %+v", inspection)
	}
}

func TestWallPassesConsumedAuthorizedPatch(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n\nfunc F() {}\n")
	post := snapshotTree(t, root)
	digest := wallAuthorization(t, root, "demo", pre, post, nil)
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" {
		t.Fatalf("authorized integration must pass: %+v", inspection)
	}
	if inspection.ExpectedTree != post || inspection.PostTree != post {
		t.Fatalf("trees: %+v", inspection)
	}
	if len(inspection.OrderedDigests) != 1 || inspection.OrderedDigests[0] != digest {
		t.Fatalf("consumption order: %v", inspection.OrderedDigests)
	}
}

func TestWallRefusesUndeclaredHostBytes(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main // host-authored drift\n")
	inspection, err := inspectWall(root, "demo", pre, wallState(), nil, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "undeclared host-authored change: main.go") {
		t.Fatalf("undeclared drift must violate: %+v", inspection)
	}
}

func TestWallAllowsDeclaredArtifactRefusesReviewedOverwrite(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "docs", "note.md"), "design note\n")
	inspection, err := inspectWall(root, "demo", pre, wallState(), nil, map[string]bool{"docs/note.md": true}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" {
		t.Fatalf("a declared artifact must pass: %+v", inspection)
	}

	// The same path under a consumed authorization refuses:
	// a declared artifact never overwrites reviewed bytes.
	writeText(t, filepath.Join(root, "docs", "note.md"), "reviewed bytes\n")
	reviewed := snapshotTree(t, root)
	digest := wallAuthorization(t, root, "demo", pre, reviewed, nil)
	writeText(t, filepath.Join(root, "docs", "note.md"), "host overwrote the review\n")
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err = inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{"docs/note.md": true}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "overwrites bytes reviewed under authorization") {
		t.Fatalf("reviewed overwrite must violate: %+v", inspection)
	}
}

func TestWallRefusesOverlappingAuthorizations(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main // round two\n")
	post := snapshotTree(t, root)
	first := wallAuthorization(t, root, "demo", pre, post, nil)
	second := wallAuthorization(t, root, "demo", pre, post, func(r map[string]any) { r["jobId"] = "job-w2" })
	certified := []map[string]any{
		{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": first},
		{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": second},
	}
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "overlap on main.go") {
		t.Fatalf("overlapping consumptions must violate: %+v", inspection)
	}
}

func TestWallRefusesMissingPatchBytes(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	record := map[string]any{"jobId": "job-w", "rootJob": "job-w", "mission": "demo",
		"baseTree": pre, "reviewedTree": pre,
		"baseSequencePoint": map[string]any{"sequence": 0, "segment": 0},
		"changedPaths":      []any{"main.go"}, "supersedes": []any{}}
	digest, derr := validate.AuthorizationRecordDigest(record)
	if derr != nil {
		t.Fatal(derr)
	}
	record["authorizationDigest"] = digest
	writeJSONFile(t, filepath.Join(missionDirPath(root, "demo"), "authorizations", digest+".json"), record)
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "patch bytes are missing") {
		t.Fatalf("missing patch must violate: %+v", inspection)
	}
}

func TestHostArtifactDeclarationGrammar(t *testing.T) {
	cases := []struct {
		value     string
		violation string
	}{
		{"", ""},
		{"docs/note.md, receipts/r1.md", ""},
		{"/etc/passwd", "not a canonical repository-relative file"},
		{"docs/../secret", "not a canonical repository-relative file"},
		{"docs/*.md", "glob"},
		{"scripts/agents/roles/orchestrator.md", "protected path"},
		{"plans/goals.md", "protected path"},
		{"plans/goals/some-goal.md", "protected path"},
		{"plans/goals/done/old-goal.md", "protected path"},
		{"plans/known-issues.md", "protected path"},
		{"plans/mission-alpha.contract.md", "protected path"},
		{"docs/a.md,,docs/b.md", "empty path"},
	}
	for _, tc := range cases {
		_, violation := parseHostArtifacts(tc.value)
		if tc.violation == "" && violation != "" {
			t.Fatalf("%q must parse: %s", tc.value, violation)
		}
		if tc.violation != "" && !strings.Contains(violation, tc.violation) {
			t.Fatalf("%q: violation %q does not name %q", tc.value, violation, tc.violation)
		}
	}
}

// openFixtureTurn stamps the open-turn marker a bed's concluding paths
// need: the wall gate reads the anchored pre-tree from it, so any
// bed driving a conclude path must have opened its turn like the runner
// does — through the state's compare-and-write.
func openFixtureTurn(t *testing.T, root, statePath, turnID string, cycle int) {
	t.Helper()
	tree, err := wallSnapshot(gittree.Workspace{Dir: root}, filepath.Base(filepath.Dir(statePath)))
	if err != nil {
		t.Fatalf("open fixture turn: %v", err)
	}
	sequence, hash, err := mission.VerifyStateShape(statePath)
	if err != nil {
		t.Fatalf("open fixture turn: %v", err)
	}
	doc, err := readJSONDoc(statePath)
	if err != nil {
		t.Fatal(err)
	}
	taint, _ := doc["workspaceTaint"].(map[string]any)
	segment, _ := jsonInt(taint["segment"])
	missionID := filepath.Base(filepath.Dir(statePath))
	workspace := gittree.Workspace{Dir: root}
	head, _, herr := workspace.HeadCommit()
	if herr != nil {
		t.Fatalf("open fixture turn: %v", herr)
	}
	headTreeRaw, herr := workspace.TreeOf(head)
	if herr != nil {
		t.Fatalf("open fixture turn: %v", herr)
	}
	headTree, herr := workspace.FilterTree(headTreeRaw, []string{missionLedgerRel(missionID)})
	if herr != nil {
		t.Fatalf("open fixture turn: %v", herr)
	}
	refs, herr := workspace.RefMap()
	if herr != nil {
		t.Fatalf("open fixture turn: %v", herr)
	}
	// The production open anchors the open commit before the host
	// launches; the bed mirrors it so the ref fence sees the runner ref.
	if herr := workspace.AnchorCommit(missionID, "turn-open-head", head); herr != nil {
		t.Fatalf("open fixture turn: %v", herr)
	}
	doc["openTurn"] = map[string]any{
		"turnId": turnID, "cycle": cycle, "preTree": tree,
		"sequence": sequence, "segment": segment,
		"openedAt":   "2026-08-18T00:00:00Z",
		"headCommit": head, "headTree": headTree,
		"topTree": nil, "refMap": mission.RecordableRefMap(refs, missionID), "topStaged": nil,
	}
	delete(doc, "integrity")
	source := statePath + ".open-turn.src"
	if err := atomicWriteJSON(source, doc); err != nil {
		t.Fatal(err)
	}
	if err := mission.WriteState(statePath, source, hash); err != nil {
		t.Fatalf("open fixture turn: %v", err)
	}
	// Production anchors the admitted baseline at init and a real state
	// anchor at every write; the bed mirrors that so the ref fence sees
	// the runner refs it requires.
	if baseline, _ := readJSONDoc(statePath); baseline != nil {
		if b0, _ := baseline["initialBaseline"].(string); b0 != "" {
			_ = workspace.Anchor(missionID, b0)
		}
	}
	_ = workspace.Anchor(missionID, tree)
	if err := mission.AnchorNamed(statePath, root, filepath.Join(filepath.Dir(statePath), "ledger.md"), "fixture", "", ""); err != nil {
		t.Fatalf("open fixture turn cannot anchor: %v", err)
	}
}

// seedWallEvidence writes the minimal passed wall.json a direct
// builder-call bed needs: a conclusion without evidence is an
// error, exactly as the engine paths guarantee by running the gate first.
func seedWallEvidence(t *testing.T, root, mission, turnID string) {
	t.Helper()
	tree := strings.Repeat("a", 40)
	writeJSONFile(t, filepath.Join(missionDirPath(root, mission), "turns", turnID, "wall.json"),
		map[string]any{"verdict": "passed", "preTree": tree, "expectedTree": tree,
			"postTree": tree, "orderedDigests": []any{},
			"posture": map[string]any{
				"headCommitPost": strings.Repeat("c", 40), "refMapPost": map[string]any{},
				"stagedTreePost": tree, "topTreePost": nil, "topStagedPost": nil,
				"worktreeCensusPost": []any{}, "capturedAt": "2026-01-01T00:00:00Z",
			}})
}

// The resume binding order: an unfinished open turn is
// inspected BEFORE healing or any new baseline. Drift parks with taint at
// resume; a clean workspace closes the unaccepted turn's marker so a
// fresh turn can open.
func TestResumeInspectsOpenTurnFirst(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	statePath, err := seedCrashedMissionState(t, engine)
	if err != nil {
		t.Fatal(err)
	}
	openFixtureTurn(t, engine.Root, statePath, "alpha-t1-dead", 1)
	if err := engine.anchor(statePath, filepath.Join(engine.missionDir(), "ledger.md"), "crash-seed"); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(engine.missionDir(), "turns", "alpha-t1-dead"), 0o755)
	writeJSONFile(t, filepath.Join(engine.missionDir(), "turns", "alpha-t1-dead", "turn.json"),
		map[string]any{"missionId": engine.Mission, "turnId": "alpha-t1-dead", "cycle": 1,
			"runtime": "fake", "model": "fixture", "status": "running"})

	// Crash shape with host-authored drift: resume must park on the wall.
	writeText(t, filepath.Join(engine.Root, "solo.go"), "package solo\n")
	if _, _, _, rerr := engine.resumeState(); rerr == nil ||
		!strings.Contains(rerr.Error(), "failed the wall at resume") {
		t.Fatalf("drifted crash must park at resume: %v", rerr)
	}
	state := readTestDoc(t, statePath)
	if state["parkReason"] != "wall-violation" {
		t.Fatalf("park reason: %v", state["parkReason"])
	}
	taint, _ := state["workspaceTaint"].(map[string]any)
	if entries, _ := taint["entries"].([]any); len(entries) != 1 {
		t.Fatalf("taint: %v", taint)
	}
}

func TestResumeClosesCleanUnacceptedTurn(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	statePath, err := seedCrashedMissionState(t, engine)
	if err != nil {
		t.Fatal(err)
	}
	openFixtureTurn(t, engine.Root, statePath, "alpha-t1-dead", 1)
	if err := engine.anchor(statePath, filepath.Join(engine.missionDir(), "ledger.md"), "crash-seed"); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(engine.missionDir(), "turns", "alpha-t1-dead"), 0o755)
	writeJSONFile(t, filepath.Join(engine.missionDir(), "turns", "alpha-t1-dead", "turn.json"),
		map[string]any{"missionId": engine.Mission, "turnId": "alpha-t1-dead", "cycle": 1,
			"runtime": "fake", "model": "fixture", "status": "running"})

	_, _, state, err := engine.resumeState()
	if err != nil {
		t.Fatalf("a clean crashed turn must resume: %v", err)
	}
	if state["openTurn"] != nil {
		t.Fatalf("the unaccepted turn's marker must close: %v", state["openTurn"])
	}
}

// The staleness predicate: an authorization based on an
// older E-point refuses when intervening accepted changes touch its
// paths, and consumes cleanly when they are disjoint.
func TestWallStalenessPredicate(t *testing.T) {
	root := wallRepo(t)
	writeText(t, filepath.Join(root, "other.txt"), "delegate target\n")
	base := snapshotTree(t, root) // E(k): the older accepted point
	writeText(t, filepath.Join(root, "main.go"), "package main // accepted intervening change\n")
	pre := snapshotTree(t, root) // E(j): intervening delta is main.go alone

	// The intervening acceptance took the expected tree from base to pre.
	state := map[string]any{"turnLog": []any{map[string]any{
		"turnId": "demo-t1", "consumedAuthorizations": []any{},
		"wall": map[string]any{"verdict": "passed", "preTree": base,
			"expectedTree": base, "postTree": pre, "orderedDigests": []any{},
			"sequencePoint": map[string]any{"sequence": 1, "segment": 0}},
	}}}

	// Overlapping: based at E(k), touching main.go, which changed since.
	writeText(t, filepath.Join(root, "main.go"), "package main // reviewed on the old base\n")
	overlapping := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main // accepted intervening change\n")
	stale := wallAuthorization(t, root, "demo", base, overlapping, nil)
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": stale}}
	inspection, err := inspectWall(root, "demo", pre, state, certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "is stale") {
		t.Fatalf("overlapping stale base must refuse: %+v", inspection)
	}

	// Disjoint: a delegate on the SAME old base touched only other.txt —
	// its reviewed tree is base + that edit alone, so the intervening
	// main.go change is disjoint and the authorization stays consumable.
	writeText(t, filepath.Join(root, "main.go"), "package main\n")
	writeText(t, filepath.Join(root, "other.txt"), "reviewed delegate work\n")
	disjointTree := snapshotTree(t, root)
	disjoint := wallAuthorization(t, root, "demo", base, disjointTree, nil)
	// The concluding workspace: the intervening change plus the reviewed
	// delegate bytes — exactly expected + the consumed patch.
	writeText(t, filepath.Join(root, "main.go"), "package main // accepted intervening change\n")
	certified = []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": disjoint}}
	inspection, err = inspectWall(root, "demo", pre, state, certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" {
		t.Fatalf("a disjoint delayed authorization must consume: %+v", inspection)
	}
}

// Tampered evidence refuses: a record claiming a
// reviewed tree the patch does not produce, and swapped patch bytes
// beside an intact record, both violate.
func TestWallRefusesTamperedAuthorization(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n\nfunc F() {}\n")
	post := snapshotTree(t, root)
	// A SELF-DIGESTED forgery — reviewedTree claims a tree the patch does
	// not produce — authenticates fine and dies on object-id equality.
	forged := wallAuthorization(t, root, "demo", pre, post, func(r map[string]any) { r["reviewedTree"] = pre })
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": forged}}
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "reviewed object id") {
		t.Fatalf("a forged reviewedTree must refuse: %+v", inspection)
	}

	// Swapped patch bytes beside an intact record die on the patchDigest.
	honest := wallAuthorization(t, root, "demo", pre, post, nil)
	writeText(t, filepath.Join(missionDirPath(root, "demo"), "authorizations", honest+".patch"), "not the issued bytes\n")
	certified = []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": honest}}
	inspection, err = inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "patchDigest") {
		t.Fatalf("swapped patch bytes must refuse: %+v", inspection)
	}

	// A post-mint field rewrite dies on record authentication itself.
	rewritten := wallAuthorization(t, root, "demo", pre, post, func(r map[string]any) { r["jobId"] = "job-w3" })
	patchRecord(t, root, "demo", rewritten, map[string]any{"changedPaths": []any{"free-pass.go"}})
	certified = []map[string]any{{"jobId": "job-w3", "verdict": "accepted", "authorizationDigest": rewritten}}
	inspection, err = inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "do not match their digest") {
		t.Fatalf("a rewritten record must refuse: %+v", inspection)
	}
}

// Fail-closed acceptance: no wall evidence, no conclusion.
func TestConclusionRefusesWithoutWallEvidence(t *testing.T) {
	root := t.TempDir()
	turn := testTurn()
	_, err := ConcludeTurn(root, "demo", cycleState(activeStreams()), turn, TurnConclusion{})
	if err == nil || !strings.Contains(err.Error(), "wall evidence") {
		t.Fatalf("a conclusion without evidence must refuse: %v", err)
	}
}

// seedCrashedMissionState provisions the mission exactly as a start would
// — approved contract, fences, ledger, state — without running any turn:
// the crash-shape beds then open a turn by hand and drive resume.
func seedCrashedMissionState(t *testing.T, engine *Engine) (string, error) {
	t.Helper()
	// The preflight bed pre-pins the approved contract and fences; only
	// the ledger and state are missing until a start would write them.
	statePath := filepath.Join(engine.missionDir(), "state.json")
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	if err := mission.InitLedger(ledgerPath, 5, 3); err != nil {
		return "", err
	}
	// REAL origins from the bed repository: the between-turns continuity
	// judgment walks first-parent chains from the recorded headCommit, so
	// a synthetic origin would read as rewritten history.
	origins, err := mission.CaptureAdmissionOrigins(engine.Root, engine.Mission)
	if err != nil {
		return "", err
	}
	if err := mission.InitStateWithBaseline(statePath, engine.approvedContractPath(), ledgerPath, "", "main", strings.Repeat("b", 40), origins); err != nil {
		return "", err
	}
	// initializeState anchors right after init — onto the runner-owned
	// anchor ref, which the in-turn ledger guard reads. The bed mirrors
	// that so guarded paths have their authenticated baseline.
	if err := engine.anchor(statePath, ledgerPath, engine.Mission); err != nil {
		return "", err
	}
	return statePath, nil
}

// patchRecord edits fields of an authorization record in place — the
// tamper and staleness beds adjust what issuance wrote.
func patchRecord(t *testing.T, root, missionID, digest string, fields map[string]any) {
	t.Helper()
	path := filepath.Join(missionDirPath(root, missionID), "authorizations", digest+".json")
	record, err := readJSONDoc(path)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range fields {
		record[k] = v
	}
	writeJSONFile(t, path, record)
}

// The changed-then-reverted ambiguity: a path the
// mission changed and later reverted vanishes from the endpoint diff, but
// the turn-by-turn union still names it — an old authorization touching
// it refuses instead of landing bytes whose review context is gone.
func TestWallStalenessCatchesChangedThenReverted(t *testing.T) {
	root := wallRepo(t)
	base := snapshotTree(t, root) // E0
	writeText(t, filepath.Join(root, "main.go"), "package main // changed in turn one\n")
	writeText(t, filepath.Join(root, "helper.txt"), "kept\n")
	mid := snapshotTree(t, root) // E1: main.go and helper.txt changed
	writeText(t, filepath.Join(root, "main.go"), "package main\n")
	preJ := snapshotTree(t, root) // E2: main.go reverted; endpoint diff hides it

	writeText(t, filepath.Join(root, "main.go"), "package main // reviewed on E0\n")
	reviewed := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n")
	digest := wallAuthorization(t, root, "demo", base, reviewed, nil)

	state := map[string]any{"turnLog": []any{
		map[string]any{"turnId": "demo-t1", "consumedAuthorizations": []any{},
			"wall": map[string]any{"verdict": "passed", "preTree": base,
				"expectedTree": base, "postTree": mid, "orderedDigests": []any{},
				"sequencePoint": map[string]any{"sequence": 1, "segment": 0}}},
		map[string]any{"turnId": "demo-t2", "consumedAuthorizations": []any{},
			"wall": map[string]any{"verdict": "passed", "preTree": mid,
				"expectedTree": mid, "postTree": preJ, "orderedDigests": []any{},
				"sequencePoint": map[string]any{"sequence": 2, "segment": 0}}},
	}}
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err := inspectWall(root, "demo", preJ, state, certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "is stale") {
		t.Fatalf("a changed-then-reverted path must refuse the old authorization: %+v", inspection)
	}
}

// A declared artifact path beneath a symlinked ancestor
// refuses: the write escaped the repository, so the tree shows nothing.
func TestWallRefusesSymlinkedArtifactAncestry(t *testing.T) {
	root := wallRepo(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	pre := snapshotTree(t, root)
	inspection, err := inspectWall(root, "demo", pre, wallState(), nil, map[string]bool{"docs/note.md": true}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "traverses the symlink docs") {
		t.Fatalf("a symlinked ancestor must refuse: %+v", inspection)
	}
}

// The ledger-ahead violation window: the crashed
// turn's cycle is already booked, so the violation ramp must not append
// it twice — the taint and park still land.
func TestResumeWallParkSurvivesLedgerAhead(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	statePath, err := seedCrashedMissionState(t, engine)
	if err != nil {
		t.Fatal(err)
	}
	openFixtureTurn(t, engine.Root, statePath, "alpha-t1-dead", 1)
	// The crash happened AFTER the ledger append: reserve and book cycle
	// 1 and align the state's ledger count, exactly the ledger-ahead shape.
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	if err := mission.ReserveCycle(engine.Root, engine.Mission); err != nil {
		t.Fatal(err)
	}
	if _, err := mission.AppendCycle(ledgerPath, 1, "no-progress", strings.Repeat("a", 40), "score=0", ""); err != nil {
		t.Fatal(err)
	}
	doc, _ := readJSONDoc(statePath)
	_, hash, _ := mission.VerifyStateShape(statePath)
	doc["ledger"].(map[string]any)["cycles"] = 1
	delete(doc, "integrity")
	source := statePath + ".ahead.src"
	if err := atomicWriteJSON(source, doc); err != nil {
		t.Fatal(err)
	}
	if err := mission.WriteState(statePath, source, hash); err != nil {
		t.Fatal(err)
	}
	if err := engine.anchor(statePath, ledgerPath, "crash-seed"); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(engine.missionDir(), "turns", "alpha-t1-dead"), 0o755)
	writeJSONFile(t, filepath.Join(engine.missionDir(), "turns", "alpha-t1-dead", "turn.json"),
		map[string]any{"missionId": engine.Mission, "turnId": "alpha-t1-dead", "cycle": 1,
			"runtime": "fake", "model": "fixture", "status": "running"})

	writeText(t, filepath.Join(engine.Root, "solo.go"), "package solo\n")
	if _, _, _, rerr := engine.resumeState(); rerr == nil ||
		!strings.Contains(rerr.Error(), "failed the wall at resume") {
		t.Fatalf("ledger-ahead drift must still park at resume: %v", rerr)
	}
	state := readTestDoc(t, statePath)
	if state["parkReason"] != "wall-violation" {
		t.Fatalf("park reason: %v", state["parkReason"])
	}
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("the taint must land despite the booked cycle: %v", taint)
	}
	if reason, _ := entries[0].(map[string]any)["reason"].(string); !strings.Contains(reason, "solo.go") {
		t.Fatalf("the taint must name the host drift, not runner bookkeeping: %v", reason)
	}
}

// A CLEAN ledger-ahead crash resumes without taint:
// the runner's own append between the anchor and the crash is the
// legitimate shape the guard tolerates at resume, and the filtered tree
// identity never sees it.
func TestResumeCleanLedgerAheadHeals(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	statePath, err := seedCrashedMissionState(t, engine)
	if err != nil {
		t.Fatal(err)
	}
	openFixtureTurn(t, engine.Root, statePath, "alpha-t1-dead", 1)
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	if err := engine.anchor(statePath, ledgerPath, "crash-seed"); err != nil {
		t.Fatal(err)
	}
	// The TRUE production crash window: the runner RESERVED
	// cycle 1, opened the turn, appended its block, then died before the
	// state write — reservation and the open marker answer for the block.
	if err := mission.ReserveCycle(engine.Root, engine.Mission); err != nil {
		t.Fatal(err)
	}
	if _, err := mission.AppendCycle(ledgerPath, 1, "no-progress", strings.Repeat("a", 40), "score=0", ""); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(engine.missionDir(), "turns", "alpha-t1-dead"), 0o755)
	writeJSONFile(t, filepath.Join(engine.missionDir(), "turns", "alpha-t1-dead", "turn.json"),
		map[string]any{"missionId": engine.Mission, "turnId": "alpha-t1-dead", "cycle": 1,
			"runtime": "fake", "model": "fixture", "status": "running"})

	_, _, state, rerr := engine.resumeState()
	if rerr != nil {
		t.Fatalf("a clean ledger-ahead crash must resume: %v", rerr)
	}
	if state["openTurn"] != nil {
		t.Fatalf("the unaccepted marker must close: %v", state["openTurn"])
	}
	taint, _ := state["workspaceTaint"].(map[string]any)
	if entries, _ := taint["entries"].([]any); len(entries) != 0 {
		t.Fatalf("a clean crash must not taint: %v", taint)
	}
}

// The filtered identity is append-stable: tracked
// ledger appends between snapshots never change the wall's trees, so
// E(i+1) == pre(i+1) holds exactly across turn boundaries.
func TestWallIdentityStableAcrossLedgerAppends(t *testing.T) {
	root := wallRepo(t)
	rel := missionLedgerRel("demo")
	writeText(t, filepath.Join(root, rel), "# ledger\n")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "-f", "--", rel)
	run("commit", "-qm", "anchor")
	workspace := gittree.Workspace{Dir: root}
	before, err := wallSnapshot(workspace, "demo")
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(root, rel), "# ledger\n\n## Cycle 1\nappended\n")
	after, err := wallSnapshot(workspace, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("a ledger append changed the wall's identity: %s vs %s", before, after)
	}
}

// The resolution segment fence: the live taint
// segment advances at resolution, before any new-segment acceptance, and
// an old-segment authorization refuses even on a repeated tree.
// The resolution's E-transition is
// PATH-SENSITIVE: a delayed authorization overlapping the resolution's own
// delta refuses, and one disjoint from it consumes — the old blanket
// segment fence refused work the resolution never touched.
func TestWallResolutionDeltaStalenessOverlap(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n\nfunc F() {}\n")
	reviewed := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n")
	digest := wallAuthorization(t, root, "demo", pre, reviewed, nil)

	// The adopted disputed tree TOUCHES main.go — the very path the
	// delayed authorization changes.
	writeText(t, filepath.Join(root, "main.go"), "package main\n// disputed\n")
	adopted := snapshotTree(t, root)

	state := resolvedFixtureState(pre, adopted)
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err := inspectWall(root, "demo", adopted, state, certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "a workspace resolution since its base touch main.go") {
		t.Fatalf("an authorization overlapping the resolution delta must refuse: %+v", inspection)
	}
}

func TestWallResolutionDeltaStalenessDisjoint(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n\nfunc F() {}\n")
	reviewed := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n")
	digest := wallAuthorization(t, root, "demo", pre, reviewed, nil)

	// The adopted tree touches ONLY notes.md; the delayed main.go
	// authorization is disjoint from the ruling and stays fresh.
	writeText(t, filepath.Join(root, "notes.md"), "adopted by ruling\n")
	adopted := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n\nfunc F() {}\n")

	state := resolvedFixtureState(pre, adopted)
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err := inspectWall(root, "demo", adopted, state, certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" {
		t.Fatalf("an authorization disjoint from the resolution delta must consume: %+v", inspection)
	}
	if len(inspection.OrderedDigests) != 1 {
		t.Fatalf("consumption: %v", inspection.OrderedDigests)
	}
}

// A FIRST-turn violation resolved before anything was
// accepted: turnLog is empty, so E0 must derive from the
// resolution's previousTree — a delayed {0,0} authorization disjoint
// from the ruling's delta still names its base and consumes.
func TestWallFirstTurnResolutionKeepsE0(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n\nfunc F() {}\n")
	reviewed := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n")
	digest := wallAuthorization(t, root, "demo", pre, reviewed, nil)

	writeText(t, filepath.Join(root, "notes.md"), "adopted by ruling\n")
	adopted := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n\nfunc F() {}\n")

	state := resolvedFixtureState(pre, adopted)
	state["turnLog"] = []any{}
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err := inspectWall(root, "demo", adopted, state, certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" {
		t.Fatalf("a delayed E0 authorization disjoint from the ruling must consume: %+v", inspection)
	}
	if len(inspection.OrderedDigests) != 1 {
		t.Fatalf("consumption: %v", inspection.OrderedDigests)
	}
}

// resolvedFixtureState is a post-resolution mission state: one prior
// no-change acceptance (so E0 and the base point are named) and one
// adopt-disputed-tree resolution whose delta is pre -> adopted.
func resolvedFixtureState(pre, adopted string) map[string]any {
	return map[string]any{
		"turnLog": []any{map[string]any{
			"turnId": "demo-t0",
			"wall": map[string]any{"preTree": pre, "postTree": pre,
				"sequencePoint": map[string]any{"sequence": 1, "segment": 0}},
			"consumedAuthorizations": []any{},
		}},
		"workspaceTaint": map[string]any{"next": 2, "segment": 1, "entries": []any{
			map[string]any{"taintId": 1, "turnId": "demo-t1", "reason": "drift",
				"setAt": "2026-08-18T00:00:00Z", "resolution": map[string]any{
					"variant": "adopt-disputed-tree", "treeId": adopted, "previousTree": pre,
					"sequencePoint": map[string]any{"sequence": 2, "segment": 1},
					"resolvedAt":    "2026-08-18T00:00:00Z", "resolvedBy": "Wido",
					"reason": "ruled", "waivedClaims": []any{"main.go authorship"}}},
		}},
	}
}

// The in-turn ledger guard: the baseline is the
// AUTHENTICATED anchor ref, so a host that edits the ledger mid-turn —
// even one that commits its edit on the mission branch to become "the
// last commit touching the path" — violates.
func TestWallCatchesMidTurnLedgerTamper(t *testing.T) {
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

	// The host injects a vocal stop-loss reset line and commits its own
	// alteration on the branch, hoping to become the guard's baseline.
	tampered, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, ledgerPath, string(tampered)+"- Stop-loss reset: ask=forged\n")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", engine.Root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=h", "GIT_AUTHOR_EMAIL=h@h",
			"GIT_COMMITTER_NAME=h", "GIT_COMMITTER_EMAIL=h@h")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("laundering git %v must succeed for the proof: %v\n%s", args, err, out)
		}
	}
	run("add", "-f", "--", "artifacts/agents/missions/alpha/ledger.md")
	run("commit", "-qm", "host launders its ledger edit")

	_, final, violated, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !violated {
		t.Fatalf("a mid-turn ledger edit must violate: %v", final)
	}
	state := readTestDoc(t, statePath)
	if state["parkReason"] != "wall-violation" {
		t.Fatalf("park reason: %v", state["parkReason"])
	}
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("taint: %v", taint)
	}
	if reason, _ := entries[0].(map[string]any)["reason"].(string); !strings.Contains(reason, "ledger") {
		t.Fatalf("the taint must name the ledger: %v", reason)
	}
}

// Answering a superseded ask refuses BY NAME toward its
// successor: the refusal must be the superseded guard, not a
// coincidental later refusal, so the bed is a real provisioned mission
// and the stderr text is captured.
func TestAnswerRefusesSupersededAsk(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	if _, err := seedCrashedMissionState(t, engine); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(asksDirPath(engine.Root, engine.Mission), "ask-1-1.json"),
		map[string]any{"askId": "ask-1-1", "streamId": "build", "reasonClass": "reserved-decision",
			"question": "old wording", "answeredAt": nil, "supersededBy": "ask-2-1"})

	saved := os.Stderr
	read, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = wr
	code := engine.Answer("ask-1-1", "approve: too late")
	wr.Close()
	os.Stderr = saved
	captured, _ := io.ReadAll(read)

	if code == 0 {
		t.Fatal("answering a superseded ask must refuse")
	}
	if !strings.Contains(string(captured), "superseded; answer ask-2-1 instead") {
		t.Fatalf("the refusal must name the successor: %q", captured)
	}
}

// The two typed resolutions, end to end from a host-authored-product
// wall-violation park. The
// bed classifies HUMAN (a test process has no recognized ancestry), which
// is exactly the human-reserved gate's happy path.
func TestResolveTaintRestore(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state := readTestDoc(t, statePath)
	preTree := state["openTurn"].(map[string]any)["preTree"].(string)

	// A generic answer NEVER clears taint: the wall-violation ask
	// refuses toward the resolution verb by name.
	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation*.json"))
	if len(asks) == 0 {
		t.Fatal("the park must have raised a wall-violation ask")
	}
	askDoc := readTestDoc(t, asks[0])
	if code := engine.Answer(askDoc["askId"].(string), "approve: just continue"); code == 0 {
		t.Fatal("a free-text answer must never clear taint")
	}

	// Whitespace never satisfies the identity and reason gates.
	if code := engine.ResolveTaint(1, "restore", preTree, "  ", " ", nil); code == 0 {
		t.Fatal("whitespace identity and reason must refuse")
	}

	// Restore refuses while the drift is still on disk.
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restoring the recorded safe tree", nil); code == 0 {
		t.Fatal("restore must refuse while the workspace still differs from the safe tree")
	}
	// The human restores the file, then the runner proves equality.
	if err := os.Remove(filepath.Join(engine.Root, "solo.go")); err != nil {
		t.Fatal(err)
	}
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restoring the recorded safe tree", nil); code != 0 {
		t.Fatalf("restore must succeed once the workspace equals the safe tree: %d", code)
	}

	after := readTestDoc(t, statePath)
	taint := after["workspaceTaint"].(map[string]any)
	if seg, _ := jsonInt(taint["segment"]); seg != 1 {
		t.Fatalf("a resolution starts a new segment: %v", taint["segment"])
	}
	entry := taint["entries"].([]any)[0].(map[string]any)
	resolution, _ := entry["resolution"].(map[string]any)
	if resolution == nil || resolution["variant"] != "restore" || resolution["treeId"] != preTree ||
		resolution["resolvedBy"] != "Wido" {
		t.Fatalf("resolution record: %v", entry)
	}
	if after["openTurn"] != nil || after["status"] != "running" || after["parkReason"] != nil {
		t.Fatalf("the resolution must close the violated turn and unpark: %v %v %v",
			after["openTurn"], after["status"], after["parkReason"])
	}
	if unresolvedTaint(after) != "" {
		t.Fatal("the taint STOP must lift")
	}
	askAfter := readTestDoc(t, asks[0])
	if askAfter["answeredAt"] == nil || askAfter["answer"] == nil {
		t.Fatalf("the resolution must answer the wall-violation ask: %v", askAfter)
	}
	// Double resolution refuses.
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "again", nil); code == 0 {
		t.Fatal("a resolved taint must refuse a second resolution")
	}

	// The mission MOVES again — and the wall keeps watching: this bed's
	// host is the solo-build offender, so the resumed turn re-offends
	// and the wall parks it as a SECOND taint under the same segment.
	// That is the full lifecycle: stop, resolve, reopen, re-protect.
	signal := filepath.Join(t.TempDir(), "resume.json")
	engine.internalRun("resume", "metasystem-mission-runner-alpha-fixture-r", signal)
	turns, _ := filepath.Glob(filepath.Join(engine.missionDir(), "turns", "*", "turn.json"))
	if len(turns) < 2 {
		t.Fatalf("the resolved mission must open a fresh turn: %v", turns)
	}
	final := readTestDoc(t, statePath)
	finalTaint := final["workspaceTaint"].(map[string]any)
	finalEntries, _ := finalTaint["entries"].([]any)
	if len(finalEntries) != 2 {
		t.Fatalf("the re-offense must taint separately: %v", finalTaint)
	}
	second := finalEntries[1].(map[string]any)
	if second["resolution"] != nil {
		t.Fatalf("the new taint must be unresolved: %v", second)
	}
	if seg, _ := jsonInt(finalTaint["segment"]); seg != 1 {
		t.Fatalf("only resolutions advance the segment: %v", finalTaint["segment"])
	}
}

// Two unresolved taints, one resolution each:
// resolving the first answers ONLY its own ask and leaves the mission
// parked with the violated turn's marker intact; the last resolution
// unparks.
func TestResolveTaintMultiTaintDiscipline(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	state := readTestDoc(t, statePath)
	preTree := state["openTurn"].(map[string]any)["preTree"].(string)

	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation*.json"))
	if len(asks) != 1 {
		t.Fatalf("the park must have raised exactly one wall-violation ask: %v", asks)
	}
	firstAsk := readTestDoc(t, asks[0])
	if id, ok := jsonInt(firstAsk["taintId"]); !ok || id != 1 {
		t.Fatalf("the park must bind its ask to the taint it books: %v", firstAsk)
	}

	// A second unresolved taint with its own bound ask, as a second
	// violation would book it.
	secondAsk := deepCopyDoc(firstAsk)
	secondAsk["askId"] = firstAsk["askId"].(string) + "-second"
	secondAsk["taintId"] = 2
	secondAskPath := filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation-second.json")
	writeJSONFile(t, secondAskPath, secondAsk)
	proposed := deepCopyDoc(state)
	taint := proposed["workspaceTaint"].(map[string]any)
	taint["entries"] = append(taint["entries"].([]any), map[string]any{
		"taintId": 2, "turnId": "alpha-t1-second", "reason": "second drift",
		"setAt": "2026-08-18T00:00:00Z", "resolution": nil})
	taint["next"] = 3
	proposed["waitingList"] = openAskIDs(asksDirPath(engine.Root, engine.Mission))
	if _, err := engine.writeState(statePath, proposed); err != nil {
		t.Fatal(err)
	}
	if err := engine.anchor(statePath, ledgerPath, "fixture-second-taint"); err != nil {
		t.Fatal(err)
	}
	// Anchoring reclaimed the checkout as MAIN; the human resolves from
	// their own shell, so clear the dead run's announcement again.
	announcements, _ := filepath.Glob(filepath.Join(engine.Root, "artifacts", "agents", "mains", "*.json"))
	for _, path := range announcements {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.Remove(filepath.Join(engine.Root, "solo.go")); err != nil {
		t.Fatal(err)
	}
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restoring the recorded safe tree", nil); code != 0 {
		t.Fatalf("the first resolution must succeed: %d", code)
	}

	mid := readTestDoc(t, statePath)
	if mid["openTurn"] == nil || mid["parkReason"] != "wall-violation" {
		t.Fatalf("an unresolved sibling taint must keep the mission parked: %v %v",
			mid["openTurn"], mid["parkReason"])
	}
	midTaint := mid["workspaceTaint"].(map[string]any)
	if seg, _ := jsonInt(midTaint["segment"]); seg != 1 {
		t.Fatalf("each resolution advances the segment once: %v", midTaint["segment"])
	}
	if unresolvedTaint(mid) == "" {
		t.Fatal("the sibling taint must still STOP the mission")
	}
	if doc := readTestDoc(t, asks[0]); doc["answeredAt"] == nil {
		t.Fatal("the first resolution must answer its own ask")
	}
	if doc := readTestDoc(t, secondAskPath); doc["answeredAt"] != nil {
		t.Fatal("the first resolution must NOT answer the sibling taint's ask")
	}

	// The first resolution's anchor announced THIS process as MAIN; a
	// real second ruling is a fresh shell process, so clear it again.
	announcements, _ = filepath.Glob(filepath.Join(engine.Root, "artifacts", "agents", "mains", "*.json"))
	for _, path := range announcements {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}

	if code := engine.ResolveTaint(2, "adopt-disputed-tree", "", "Wido", "keeping the disputed work",
		[]string{"authorship of the second drift"}); code != 0 {
		t.Fatalf("the last resolution must succeed: %d", code)
	}
	final := readTestDoc(t, statePath)
	if final["openTurn"] != nil || final["status"] != "running" || final["parkReason"] != nil {
		t.Fatalf("the LAST resolution unparks: %v %v %v",
			final["openTurn"], final["status"], final["parkReason"])
	}
	finalTaint := final["workspaceTaint"].(map[string]any)
	if seg, _ := jsonInt(finalTaint["segment"]); seg != 2 {
		t.Fatalf("two resolutions, two segment advances: %v", finalTaint["segment"])
	}
	if unresolvedTaint(final) != "" {
		t.Fatal("the taint STOP must lift after the last resolution")
	}
	if doc := readTestDoc(t, secondAskPath); doc["answeredAt"] == nil {
		t.Fatal("the last resolution must answer its own ask")
	}
}

// A ledger-domain taint cannot be RESTORED:
// the ledger sits outside the tree projection restore proves against;
// adoption — which re-baselines the anchored truth — is the lawful path.
func TestResolveTaintLedgerDomainRefusesRestore(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	state := readTestDoc(t, statePath)
	preTree := state["openTurn"].(map[string]any)["preTree"].(string)

	proposed := deepCopyDoc(state)
	taint := proposed["workspaceTaint"].(map[string]any)
	taint["entries"] = append(taint["entries"].([]any), map[string]any{
		"taintId": 2, "turnId": "alpha-t1-ledger", "reason": ledgerViolationPrefix + " bytes were modified during the turn",
		"setAt": "2026-08-18T00:00:00Z", "resolution": nil})
	taint["next"] = 3
	if _, err := engine.writeState(statePath, proposed); err != nil {
		t.Fatal(err)
	}
	if err := engine.anchor(statePath, ledgerPath, "fixture-ledger-taint"); err != nil {
		t.Fatal(err)
	}
	announcements, _ := filepath.Glob(filepath.Join(engine.Root, "artifacts", "agents", "mains", "*.json"))
	for _, path := range announcements {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}

	if code := engine.ResolveTaint(2, "restore", preTree, "Wido", "put it back", nil); code != 3 {
		t.Fatalf("restore of a ledger-domain taint must refuse: %d", code)
	}
	if code := engine.ResolveTaint(2, "adopt-disputed-tree", "", "Wido", "re-baselining the ledger",
		[]string{"ledger narrative integrity for turn alpha-t1-ledger"}); code != 0 {
		t.Fatalf("adoption must close a ledger-domain taint: %d", code)
	}
}

// The resolution tail is COMPLETABLE: a crash
// after the durable state write but before the ask answers is repaired by
// re-running resolve-taint, which answers from the RECORDED ruling; an
// ask without a parseable bound taint id is never touched.
func TestResolveTaintTailCompletion(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state := readTestDoc(t, statePath)
	preTree := state["openTurn"].(map[string]any)["preTree"].(string)

	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation*.json"))
	if len(asks) != 1 {
		t.Fatalf("expected one bound ask: %v", asks)
	}
	// An UNBOUND wall-violation ask (no taintId): fail-closed — no
	// resolution may ever answer it.
	unbound := deepCopyDoc(readTestDoc(t, asks[0]))
	unbound["askId"] = unbound["askId"].(string) + "-unbound"
	delete(unbound, "taintId")
	unboundPath := filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation-unbound.json")
	writeJSONFile(t, unboundPath, unbound)

	if err := os.Remove(filepath.Join(engine.Root, "solo.go")); err != nil {
		t.Fatal(err)
	}
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restoring the recorded safe tree", nil); code != 0 {
		t.Fatalf("restore must succeed: %d", code)
	}
	if doc := readTestDoc(t, unboundPath); doc["answeredAt"] != nil {
		t.Fatal("an unbound ask must never be answered by a resolution (fail closed)")
	}

	// Simulate the crash window: the bound ask reverts to unanswered
	// while state already records the ruling.
	crashed := deepCopyDoc(readTestDoc(t, asks[0]))
	crashed["answeredAt"] = nil
	crashed["answer"] = nil
	writeJSONFile(t, asks[0], crashed)
	announcements, _ := filepath.Glob(filepath.Join(engine.Root, "artifacts", "agents", "mains", "*.json"))
	for _, path := range announcements {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	// The retry may name ANY variant/args — the tail completes from the
	// RECORDED resolution, never from this call.
	if code := engine.ResolveTaint(1, "adopt-disputed-tree", "", "Someone Else", "different args", []string{"x"}); code != 0 {
		t.Fatalf("the tail completion must succeed: %d", code)
	}
	completed := readTestDoc(t, asks[0])
	answer, _ := completed["answer"].(string)
	if completed["answeredAt"] == nil || !strings.Contains(answer, "resolved by Wido") || !strings.Contains(answer, "restore") {
		t.Fatalf("the completed answer must carry the RECORDED ruling: %v", completed)
	}
	// With the tail complete, a further resolve refuses.
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "again", nil); code != 3 {
		t.Fatalf("a completed resolution must refuse: %d", code)
	}
}

// E-continuity at reservation: drift landing
// between a resolution and the next turn parks as a NEW wall violation
// instead of becoming the silently grandfathered baseline.
func TestReservationParksOnDriftAfterResolution(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")

	if code := engine.ResolveTaint(1, "adopt-disputed-tree", "", "Wido", "keeping the disputed work",
		[]string{"authorship of solo.go"}); code != 0 {
		t.Fatalf("adoption must succeed: %d", code)
	}
	// Out-of-band drift AFTER the ruling, BEFORE the next reservation.
	writeText(t, filepath.Join(engine.Root, "drift.txt"), "unruled bytes\n")

	signal := filepath.Join(t.TempDir(), "resume.json")
	engine.internalRun("resume", "metasystem-mission-runner-alpha-fixture-d", signal)

	final := readTestDoc(t, statePath)
	if final["parkReason"] != "wall-violation" {
		t.Fatalf("drift at reservation must park as a wall violation: %v", final["parkReason"])
	}
	taint := final["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("the drift must book its own taint: %v", entries)
	}
	second := entries[1].(map[string]any)
	reason, _ := second["reason"].(string)
	if !strings.Contains(reason, "workspace drifted between turns") {
		t.Fatalf("the taint must name the reservation drift: %v", reason)
	}
	if second["resolution"] != nil {
		t.Fatal("the drift taint must be unresolved")
	}
	// The violation wrote its wall.json evidence...
	turnID, _ := second["turnId"].(string)
	evidence := readTestDoc(t, filepath.Join(engine.missionDir(), "turns", turnID, "wall.json"))
	if v, _ := evidence["violation"].(string); !strings.Contains(v, "workspace drifted between turns") {
		t.Fatalf("the drift park must record wall evidence: %v", evidence)
	}
	// ...and booked NO ledger block: the reserved
	// cycle heals as a lost turn, so every crash window reconciles.
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	_, _, cycles, err := mission.ParseLedger(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	stateLedger, _ := final["ledger"].(map[string]any)
	stateCycles, _ := jsonInt(stateLedger["cycles"])
	if int64(len(cycles)) != stateCycles {
		t.Fatalf("the reservation park must not book the ledger: ledger=%d state=%d", len(cycles), stateCycles)
	}
	if _, _, err := mission.VerifyStateWithAnchor(statePath, engine.Root, ledgerPath); err != nil {
		t.Fatalf("the parked position must verify with its anchor: %v", err)
	}
}

// Late ledger bytes never ride a
// resolution: tamper between the park and the ruling fails the anchor-verified
// entry, and reconciliation refuses to bless it.
func TestResolveTaintRefusesLedgerDrift(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	state := readTestDoc(t, statePath)
	preTree := state["openTurn"].(map[string]any)["preTree"].(string)

	tampered, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, ledgerPath, string(tampered)+"- Stop-loss reset: ask=forged-mid-resolution\n")

	if err := os.Remove(filepath.Join(engine.Root, "solo.go")); err != nil {
		t.Fatal(err)
	}
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restoring", nil); code == 0 {
		t.Fatal("a resolution over drifted ledger bytes must refuse")
	}
}

// The runner repairs a resolution's crash tail at
// start: a RESOLVED taint's still-open ask is answered from
// the recorded ruling, and the NEXT violation books its own bound ask —
// a stale tail never suppresses or strands it.
func TestRunnerRepairsResolutionTailAtResume(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state := readTestDoc(t, statePath)
	preTree := state["openTurn"].(map[string]any)["preTree"].(string)

	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation*.json"))
	if len(asks) != 1 {
		t.Fatalf("expected one bound ask: %v", asks)
	}
	if err := os.Remove(filepath.Join(engine.Root, "solo.go")); err != nil {
		t.Fatal(err)
	}
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restoring the recorded safe tree", nil); code != 0 {
		t.Fatalf("restore must succeed: %d", code)
	}
	// Crash tail: the answer never landed.
	crashed := deepCopyDoc(readTestDoc(t, asks[0]))
	crashed["answeredAt"] = nil
	crashed["answer"] = nil
	writeJSONFile(t, asks[0], crashed)

	// Resume: the runner repairs the tail, then the still-offending
	// solo-build host violates again — and taint 2 must get its OWN ask.
	signal := filepath.Join(t.TempDir(), "resume.json")
	engine.internalRun("resume", "metasystem-mission-runner-alpha-fixture-t", signal)

	repaired := readTestDoc(t, asks[0])
	answer, _ := repaired["answer"].(string)
	if repaired["answeredAt"] == nil || !strings.Contains(answer, "resolved by Wido") {
		t.Fatalf("the runner must repair the crash tail from the recorded ruling: %v", repaired)
	}
	final := readTestDoc(t, statePath)
	entries, _ := final["workspaceTaint"].(map[string]any)["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("the re-offense must book taint 2: %v", entries)
	}
	boundToSecond := 0
	askPaths, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "*.json"))
	for _, path := range askPaths {
		doc := readTestDoc(t, path)
		if doc["reasonClass"] != "wall-violation" || doc["answeredAt"] != nil {
			continue
		}
		if id, ok := jsonInt(doc["taintId"]); ok && id == 2 {
			boundToSecond++
		}
	}
	if boundToSecond != 1 {
		t.Fatalf("taint 2 must get exactly one bound open ask, got %d", boundToSecond)
	}
}

// Violated evidence with NO marker and NO taint — the reservation
// park's evidence-to-park crash
// window — re-executes the park at the next reservation, even when
// the workspace was cleaned up in between.
func TestReservationReparksOrphanedEvidence(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	if code := engine.ResolveTaint(1, "adopt-disputed-tree", "", "Wido", "keeping the disputed work",
		[]string{"authorship of solo.go"}); code != 0 {
		t.Fatalf("adoption must succeed: %d", code)
	}
	// The crash artifact: violated evidence for a turn no ledger knows.
	orphanDir := filepath.Join(engine.missionDir(), "turns", "alpha-t9-orphan")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(orphanDir, "turn.json"),
		map[string]any{"missionId": engine.Mission, "turnId": "alpha-t9-orphan", "cycle": 9,
			"runtime": "fake", "model": "fixture", "status": "pending"})
	evidence := &wallInspection{PreTree: strings.Repeat("a", 40), ExpectedTree: strings.Repeat("b", 40),
		PostTree: strings.Repeat("a", 40), Violation: "workspace drifted between turns: fixture orphan",
		Unaccounted: []string{"drift.txt"}}
	writeJSONFile(t, filepath.Join(orphanDir, "wall.json"), evidence.document())

	signal := filepath.Join(t.TempDir(), "resume.json")
	engine.internalRun("resume", "metasystem-mission-runner-alpha-fixture-o", signal)

	final := readTestDoc(t, statePath)
	if final["parkReason"] != "wall-violation" {
		t.Fatalf("orphaned violated evidence must re-park: %v", final["parkReason"])
	}
	entries, _ := final["workspaceTaint"].(map[string]any)["entries"].([]any)
	last, _ := entries[len(entries)-1].(map[string]any)
	if turn, _ := last["turnId"].(string); turn != "alpha-t9-orphan" {
		t.Fatalf("the re-park must book the orphaned turn: %v", last)
	}
	if reason, _ := last["reason"].(string); !strings.Contains(reason, "fixture orphan") {
		t.Fatalf("the recorded violation must carry over verbatim: %v", reason)
	}
	if last["resolution"] != nil {
		t.Fatal("the re-parked taint must be unresolved")
	}
}

// Both typed resolutions through the REAL human
// entrypoint: the wrapper resolves the bed root from
// its own location and forwards to the bed's binary — this is the
// invocation a human actually types, wrapper to binary to engine.
func TestResolveTaintThroughWrapper(t *testing.T) {
	restoreBed := parkedSoloBuildMission(t)
	statePath := filepath.Join(restoreBed.missionDir(), "state.json")
	state := readTestDoc(t, statePath)
	preTree := state["openTurn"].(map[string]any)["preTree"].(string)
	if err := os.Remove(filepath.Join(restoreBed.Root, "solo.go")); err != nil {
		t.Fatal(err)
	}
	runWrapper := func(bed *Engine, args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(filepath.Join(bed.Root, "scripts", "agents", "mission-runner.sh"), args...)
		cmd.Dir = bed.Root
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	out, err := runWrapper(restoreBed, "resolve-taint", "--mission", restoreBed.Mission,
		"--taint", "1", "--restore", preTree, "--by", "Wido", "--reason", "restored through the entrypoint")
	if err != nil {
		t.Fatalf("the wrapper restore must succeed: %v\n%s", err, out)
	}
	after := readTestDoc(t, statePath)
	if after["parkReason"] != nil || unresolvedTaint(after) != "" {
		t.Fatalf("the wrapper restore must unpark and lift the STOP: %v", after["parkReason"])
	}

	adoptBed := parkedSoloBuildMission(t)
	adoptState := filepath.Join(adoptBed.missionDir(), "state.json")
	out, err = runWrapper(adoptBed, "resolve-taint", "--mission", adoptBed.Mission,
		"--taint", "1", "--adopt", "--by", "Wido", "--reason", "adopted through the entrypoint",
		"--waives", "authorship of solo.go")
	if err != nil {
		t.Fatalf("the wrapper adoption must succeed: %v\n%s", err, out)
	}
	adopted := readTestDoc(t, adoptState)
	entry := adopted["workspaceTaint"].(map[string]any)["entries"].([]any)[0].(map[string]any)
	resolution, _ := entry["resolution"].(map[string]any)
	if resolution == nil || resolution["variant"] != "adopt-disputed-tree" {
		t.Fatalf("the wrapper adoption must record its typed resolution: %v", entry)
	}

	// Malformed shapes refuse at the wrapper too, exit 2, no state read.
	if _, err := runWrapper(adoptBed, "resolve-taint", "--mission", adoptBed.Mission,
		"--taint", "1", "--restore", "not-a-tree", "--by", "Wido", "--reason", "r"); err == nil {
		t.Fatal("a malformed tree id must refuse through the wrapper")
	}
}

// The FINAL-CYCLE orphan: with
// the cycle budget exhausted, no new reservation exists to run the
// reservation-time sweep — ONLY the resume-time sweep can turn the
// orphaned evidence into a taint before the fence buries it.
func TestResumeSweepOutranksTheCycleFence(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	if code := engine.ResolveTaint(1, "adopt-disputed-tree", "", "Wido", "keeping the disputed work",
		[]string{"authorship of solo.go"}); code != 0 {
		t.Fatalf("adoption must succeed: %d", code)
	}
	// Orphaned violated evidence, crash artifact shape.
	orphanDir := filepath.Join(engine.missionDir(), "turns", "alpha-t8-lastcycle")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(orphanDir, "turn.json"),
		map[string]any{"missionId": engine.Mission, "turnId": "alpha-t8-lastcycle", "cycle": 8,
			"runtime": "fake", "model": "fixture", "status": "pending"})
	evidence := &wallInspection{PreTree: strings.Repeat("a", 40), ExpectedTree: strings.Repeat("b", 40),
		PostTree: strings.Repeat("a", 40), Violation: "workspace drifted between turns: final-cycle orphan",
		Unaccounted: []string{"drift.txt"}}
	writeJSONFile(t, filepath.Join(orphanDir, "wall.json"), evidence.document())
	// Exhaust the cycle budget: the fence refuses any NEW reservation,
	// so the reservation-time sweep can never run.
	_, _, budget, err := mission.ParseLedger(filepath.Join(engine.missionDir(), "ledger.md"))
	_ = budget
	if err != nil {
		t.Fatal(err)
	}
	fences := readTestDoc(t, engine.fencesPath())
	cycleBudget, _ := jsonInt(fences["cycles"])
	contractBudget, _ := jsonInt(readTestDoc(t, engine.fencesPath())["cycles"])
	_ = contractBudget
	// Push the spent counter to a huge value so ReserveCycle's fence
	// refuses outright.
	fences["cycles"] = cycleBudget + 1000
	writeJSONFile(t, engine.fencesPath(), fences)

	signal := filepath.Join(t.TempDir(), "resume.json")
	engine.internalRun("resume", "metasystem-mission-runner-alpha-fixture-f", signal)

	final := readTestDoc(t, statePath)
	if final["parkReason"] != "wall-violation" {
		t.Fatalf("the resume sweep must outrank the fence: %v", final["parkReason"])
	}
	entries, _ := final["workspaceTaint"].(map[string]any)["entries"].([]any)
	last, _ := entries[len(entries)-1].(map[string]any)
	if reason, _ := last["reason"].(string); !strings.Contains(reason, "final-cycle orphan") {
		t.Fatalf("the orphaned violation must be the booked taint: %v", reason)
	}
}

// EVERY unparsable-ledger tamper shape resolves through ONE
// flow: the park lands with its
// anchor DEFERRED (the anchor refuses unparsable bytes — a baseline
// nothing can drive), resolution refuses with the named
// byte-restoration path, and after restoration the one-step lag-heal
// bridges and adoption completes on a mission the machinery can still
// read. The shapes cover the budget line, the heading grammar, and the
// cross-product (broken budget beside a valid contiguous
// block).
func TestUnparsableLedgerTamperResolvesAfterRestoration(t *testing.T) {
	shapes := map[string]func(string) string{
		"broken-budget": func(pristine string) string {
			return strings.Replace(pristine, "- Cycle budget:", "-x Cycle budget:", 1)
		},
		"broken-heading": func(pristine string) string {
			return pristine + "\n### Cycle 5\n- Classification: no-progress; candidate-sha=abc; observed=x\n"
		},
		"broken-budget-valid-block": func(pristine string) string {
			return strings.Replace(pristine, "- Cycle budget:", "-x Cycle budget:", 1) +
				"\n### Cycle 1\n- Classification: no-progress; candidate-sha=abc; observed=x\n"
		},
	}
	for name, tamper := range shapes {
		t.Run(name, func(t *testing.T) {
			var pristineBytes string
			engine, statePath, ledgerPath, taintID := ledgerTamperPark(t, func(pristine string) string {
				pristineBytes = pristine
				return tamper(pristine)
			})
			if code := engine.ResolveTaint(taintID, "adopt-disputed-tree", "", "Wido", "ruling on the tamper",
				[]string{"ledger narrative integrity"}); code != 3 {
				t.Fatalf("resolution over unparsable bytes must refuse: %d", code)
			}
			// The NATURAL intervening resume: the
			// taint STOP refuses on a raw read BEFORE any reconciliation
			// can write, so the anchor gap stays exactly one step.
			signal := filepath.Join(t.TempDir(), "resume.json")
			engine.internalRun("resume", "metasystem-mission-runner-alpha-fixture-u", signal)
			midState := readTestDoc(t, statePath)
			midIntegrity, _ := midState["integrity"].(map[string]any)
			if midState["parkReason"] != "wall-violation" {
				t.Fatalf("an intervening resume must not repark the taint: %v", midState["parkReason"])
			}
			if hash, _ := midIntegrity["hash"].(string); hash == "" {
				t.Fatal("state must stay readable after the refused resume")
			}
			writeText(t, ledgerPath, pristineBytes)
			if code := engine.ResolveTaint(taintID, "adopt-disputed-tree", "", "Wido", "ruling on the tamper",
				[]string{"ledger narrative integrity"}); code != 0 {
				t.Fatalf("resolution after byte restoration must succeed: %d", code)
			}
			// The resolved mission is DRIVEABLE: its ledger parses.
			if _, _, _, err := mission.ParseLedger(ledgerPath); err != nil {
				t.Fatalf("the resolved mission must stay readable: %v", err)
			}
			_ = statePath
		})
	}
}

// The TAIL CALL SITE faces moved ledger bytes in its write-to-anchor
// window: the pinned anchor must refuse — a caller
// reverted to a post-write reread would pin the moved bytes and anchor
// them successfully, failing this test.
func TestTailAnchorRefusesMovedLedger(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	state := readTestDoc(t, statePath)
	preTree := state["openTurn"].(map[string]any)["preTree"].(string)
	if err := os.Remove(filepath.Join(engine.Root, "solo.go")); err != nil {
		t.Fatal(err)
	}
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restoring the recorded safe tree", nil); code != 0 {
		t.Fatalf("restore must succeed: %d", code)
	}
	asks, _ := filepath.Glob(filepath.Join(asksDirPath(engine.Root, engine.Mission), "wall-violation*.json"))
	crashed := deepCopyDoc(readTestDoc(t, asks[0]))
	crashed["answeredAt"] = nil
	crashed["answer"] = nil
	writeJSONFile(t, asks[0], crashed)
	announcements, _ := filepath.Glob(filepath.Join(engine.Root, "artifacts", "agents", "mains", "*.json"))
	for _, path := range announcements {
		os.Remove(path)
	}
	pristine, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	engine.preAnchorHook = func() {
		writeText(t, ledgerPath, string(pristine)+"\n")
	}
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "tail retry", nil); code == 0 {
		t.Fatal("the tail anchor must refuse ledger bytes moved past the verified pin")
	}
	// The refused anchor left the tail DURABLE (asks answered, waiting
	// list written) with the anchor one lawful step behind. The NATURAL
	// retry with the bytes STILL MOVED must refuse WITHOUT WIDENING the
	// gap: no state-integrity park may land on the
	// recoverable shape.
	engine.preAnchorHook = nil
	beforeRetry := readTestDoc(t, statePath)
	beforeIntegrity, _ := beforeRetry["integrity"].(map[string]any)
	beforeHash, _ := beforeIntegrity["hash"].(string)
	if code, _ := mission.Reconcile(statePath, engine.Root, ledgerPath); code == 0 {
		t.Fatal("reconciliation over moved bytes must refuse")
	}
	afterRetry := readTestDoc(t, statePath)
	afterIntegrity, _ := afterRetry["integrity"].(map[string]any)
	if afterHash, _ := afterIntegrity["hash"].(string); afterHash != beforeHash {
		t.Fatal("the refusal must not write — the gap must stay one step")
	}
	// The HUMAN VERB surfaces the actionable
	// repair: resolve-taint over the moved bytes must print
	// the restore instruction, not a generic verification failure.
	announcements2, _ := filepath.Glob(filepath.Join(engine.Root, "artifacts", "agents", "mains", "*.json"))
	for _, path := range announcements2 {
		os.Remove(path)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	savedStderr := os.Stderr
	os.Stderr = stderrW
	code := engine.ResolveTaint(1, "restore", preTree, "Wido", "retry over moved bytes", nil)
	os.Stderr = savedStderr
	stderrW.Close()
	captured, _ := io.ReadAll(stderrR)
	if code == 0 {
		t.Fatal("resolve over moved bytes must refuse")
	}
	if !strings.Contains(string(captured), "restore the ledger bytes") {
		t.Fatalf("the refusal must carry the actionable repair, got: %s", captured)
	}
	// Restoring the bytes lets reconciliation bridge it — the same heal
	// every crash window uses.
	writeText(t, ledgerPath, string(pristine))
	if code, rerr := mission.Reconcile(statePath, engine.Root, ledgerPath); rerr != nil || code != 0 {
		t.Fatalf("reconciliation must heal the refused-anchor tail: code=%d err=%v", code, rerr)
	}
	if _, err := engine.verifyState(statePath, true); err != nil {
		t.Fatalf("the healed mission must verify clean: %v", err)
	}
}

// The RESUME CLOSE-MARKER call site faces the same moved-bytes window
// and must refuse rather than rebind.
func TestResumeCloseAnchorRefusesMovedLedger(t *testing.T) {
	engine := buildFullCycleRoot(t, "FAKEHOST:close-stream")
	statePath, err := seedCrashedMissionState(t, engine)
	if err != nil {
		t.Fatal(err)
	}
	openFixtureTurn(t, engine.Root, statePath, "alpha-t1-clean", 1)
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	if err := engine.anchor(statePath, ledgerPath, "open"); err != nil {
		t.Fatal(err)
	}
	pristine, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	engine.preAnchorHook = func() {
		writeText(t, ledgerPath, string(pristine)+"\n")
	}
	if _, _, _, rerr := engine.resumeState(); rerr == nil {
		t.Fatal("the close-marker anchor must refuse ledger bytes moved past the verified pin")
	}
	// The natural retry with bytes still moved refuses WITHOUT widening;
	// restoration heals.
	engine.preAnchorHook = nil
	statePathClean := filepath.Join(engine.missionDir(), "state.json")
	before := readTestDoc(t, statePathClean)
	beforeIntegrity, _ := before["integrity"].(map[string]any)
	beforeHash, _ := beforeIntegrity["hash"].(string)
	if code, _ := mission.Reconcile(statePathClean, engine.Root, ledgerPath); code == 0 {
		t.Fatal("reconciliation over moved bytes must refuse")
	}
	after := readTestDoc(t, statePathClean)
	afterIntegrity, _ := after["integrity"].(map[string]any)
	if afterHash, _ := afterIntegrity["hash"].(string); afterHash != beforeHash {
		t.Fatal("the refusal must not write — the gap must stay one step")
	}
	writeText(t, ledgerPath, string(pristine))
	if code, rerr := mission.Reconcile(statePathClean, engine.Root, ledgerPath); rerr != nil || code != 0 {
		t.Fatalf("restoration must heal the close-marker gap: code=%d err=%v", code, rerr)
	}
}

// The pin ORIGIN is distinguishing: when the live
// ledger bytes have moved past the anchored truth, the verified pin is
// the TIP's sha — a reverted post-write reread would return the moved
// bytes' sha and this test would fail.
func TestVerifiedLedgerPinIsTheAnchorTip(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	tipSHA, err := engine.verifiedLedgerPin()
	if err != nil {
		t.Fatal(err)
	}
	moved, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, ledgerPath, string(moved)+"\n")
	movedSHA := sha256Hex(string(moved) + "\n")
	pin, err := engine.verifiedLedgerPin()
	if err != nil {
		t.Fatal(err)
	}
	if pin != tipSHA {
		t.Fatalf("the pin must stay the anchored position: %s vs %s", pin, tipSHA)
	}
	if pin == movedSHA {
		t.Fatal("the pin must never be the moved live bytes")
	}
}

// ledgerTamperPark drives a mid-turn ledger tamper through the wall gate
// and returns the parked bed with the booked taint id.
func ledgerTamperPark(t *testing.T, tamper func(string) string) (*Engine, string, string, int64) {
	t.Helper()
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
	pristine, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, ledgerPath, tamper(string(pristine)))

	_, final, violated, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !violated {
		t.Fatalf("the ledger tamper must violate: %v", final)
	}
	state := readTestDoc(t, statePath)
	if unresolvedTaint(state) == "" {
		t.Fatal("the violation must be a recorded, resolvable taint")
	}
	announcements, _ := filepath.Glob(filepath.Join(engine.Root, "artifacts", "agents", "mains", "*.json"))
	for _, path := range announcements {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	stageHumanShell(t)
	entries, _ := state["workspaceTaint"].(map[string]any)["entries"].([]any)
	lastEntry, _ := entries[len(entries)-1].(map[string]any)
	taintID, _ := jsonInt(lastEntry["taintId"])
	return engine, statePath, ledgerPath, taintID
}

// The resolution's crash window: state written,
// anchor missing. The reconciliation anchor-lag heal recovers it without
// human surgery.
func TestResolveTaintCrashHealsAtReconcile(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")
	ledgerPath := filepath.Join(engine.missionDir(), "ledger.md")
	state := readTestDoc(t, statePath)
	preTree := state["openTurn"].(map[string]any)["preTree"].(string)
	if err := os.Remove(filepath.Join(engine.Root, "solo.go")); err != nil {
		t.Fatal(err)
	}
	if code := engine.ResolveTaint(1, "restore", preTree, "Wido", "restoring the recorded safe tree", nil); code != 0 {
		t.Fatalf("restore must succeed: %d", code)
	}

	// Simulate the crash between the state write and the anchor: rewind
	// the runner-owned anchor ref to its parent, leaving state exactly
	// one write ahead.
	ref := "refs/metasystem/missions/" + engine.Mission + "/state-anchors"
	rev := exec.Command("git", "-C", engine.Root, "rev-parse", ref+"^")
	parent, err := rev.Output()
	if err != nil {
		t.Fatalf("the resolution anchor must have a parent: %v", err)
	}
	rewind := exec.Command("git", "-C", engine.Root, "update-ref", ref, strings.TrimSpace(string(parent)))
	if out, err := rewind.CombinedOutput(); err != nil {
		t.Fatalf("rewind: %v\n%s", err, out)
	}

	code, err := mission.Reconcile(statePath, engine.Root, ledgerPath)
	if err != nil || code != 0 {
		t.Fatalf("the anchor-lag heal must recover the resolution write: code=%d err=%v", code, err)
	}
	if _, err := engine.verifyState(statePath, false); err != nil {
		t.Fatalf("the healed mission must verify clean: %v", err)
	}
}

func TestResolveTaintAdoptDisputedTree(t *testing.T) {
	engine := parkedSoloBuildMission(t)
	statePath := filepath.Join(engine.missionDir(), "state.json")

	// Adoption without named waived claims refuses.
	if code := engine.ResolveTaint(1, "adopt-disputed-tree", "", "Wido", "keeping the work", nil); code == 0 {
		t.Fatal("adoption must name the waived attribution claims")
	}
	if code := engine.ResolveTaint(1, "adopt-disputed-tree", "", "Wido", "keeping the disputed work",
		[]string{"authorship of solo.go"}); code != 0 {
		t.Fatal("adoption with named claims must succeed")
	}
	after := readTestDoc(t, statePath)
	taint := after["workspaceTaint"].(map[string]any)
	entry := taint["entries"].([]any)[0].(map[string]any)
	resolution, _ := entry["resolution"].(map[string]any)
	if resolution["variant"] != "adopt-disputed-tree" {
		t.Fatalf("resolution record: %v", entry)
	}
	adopted, _ := resolution["treeId"].(string)
	// The adopted tree IS the observed workspace — solo.go included.
	workspaceNow, err := wallSnapshot(gittree.Workspace{Dir: engine.Root}, engine.Mission)
	if err != nil {
		t.Fatal(err)
	}
	if adopted != workspaceNow {
		t.Fatalf("adoption must bind the observed tree: %s vs %s", adopted, workspaceNow)
	}
	claims, _ := resolution["waivedClaims"].([]any)
	if len(claims) != 1 || claims[0] != "authorship of solo.go" {
		t.Fatalf("waived claims: %v", claims)
	}
	// The violation record survives adoption — cleared operationally,
	// never erased.
	if entry["reason"] == nil || entry["setAt"] == nil {
		t.Fatalf("the violation record must survive adoption: %v", entry)
	}
	if seg, _ := jsonInt(taint["segment"]); seg != 1 {
		t.Fatalf("segment: %v", taint["segment"])
	}
	if unresolvedTaint(after) != "" {
		t.Fatal("the taint STOP must lift")
	}
}

// parkedSoloBuildMission drives a host that authors its own product file
// to its wall-violation park
// and returns the engine, ready for resolution.
func parkedSoloBuildMission(t *testing.T) *Engine {
	t.Helper()
	engine := buildFullCycleRoot(t, "FAKEHOST:solo-build")
	signal := filepath.Join(t.TempDir(), "start.json")
	engine.internalRun("start", "metasystem-mission-runner-alpha-fixture", signal)
	state := readTestDoc(t, filepath.Join(engine.missionDir(), "state.json"))
	if state["parkReason"] != "wall-violation" {
		t.Fatalf("the bed must park on the wall: %v", state["parkReason"])
	}
	// The in-process run announced THIS test pid as the checkout main; a
	// real resolution runs from the human's own shell with no announced
	// ancestry. Clearing the dead run's announcements — and pinning the
	// person-at-a-terminal fact, now that HUMAN is positive-only —
	// simulates exactly that, so the human-reserved gate sees ClassHuman.
	announcements, _ := filepath.Glob(filepath.Join(engine.Root, "artifacts", "agents", "mains", "*.json"))
	for _, path := range announcements {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	stageHumanShell(t)
	return engine
}

// stageHumanShell pins this test process's person-at-a-terminal fact:
// HUMAN is positive-only, and the human-reserved resolution gates must
// decide identically at a desk and on a headless runner. The fixture
// repositories declare fake runtimes in their committed baseline, which
// authorizes the staged table.
func stageHumanShell(t *testing.T) {
	t.Helper()
	table := filepath.Join(t.TempDir(), "terminal-table.json")
	if err := os.WriteFile(table, []byte(fmt.Sprintf(`{"%d": {"terminal": true}}`, os.Getpid())), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
}

// The tree-equation positive after a resolution: FRESH work issued at the new
// segment consumes cleanly — the fence blocks only the old segment.
func TestWallConsumesFreshWorkAfterResolution(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n\nfunc Fresh() {}\n")
	reviewed := snapshotTree(t, root)
	digest := wallAuthorization(t, root, "demo", pre, reviewed, func(r map[string]any) {
		r["baseSequencePoint"] = map[string]any{"sequence": 1, "segment": 1}
	})
	state := map[string]any{
		"turnLog": []any{},
		"workspaceTaint": map[string]any{"next": 2, "segment": 1, "entries": []any{
			map[string]any{"taintId": 1, "turnId": "demo-t1", "reason": "drift",
				"setAt": "2026-08-18T00:00:00Z", "resolution": map[string]any{
					"variant": "restore", "treeId": pre, "previousTree": pre,
					"sequencePoint": map[string]any{"sequence": 1, "segment": 1},
					"resolvedAt":    "2026-08-18T00:00:00Z",
					"resolvedBy":    "Wido", "reason": "restored"}},
		}},
	}
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err := inspectWall(root, "demo", pre, state, certified, map[string]bool{}, "", legacySnapshot(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" {
		t.Fatalf("fresh new-segment work must consume: %+v", inspection)
	}
	if len(inspection.OrderedDigests) != 1 {
		t.Fatalf("consumption: %v", inspection.OrderedDigests)
	}
}

// testAdmissionOrigins is a minimal valid admission-origins record for
// states born outside a live repository.
func testAdmissionOrigins() map[string]any {
	return map[string]any{
		"headCommit": strings.Repeat("c", 40), "topTree": nil, "topStaged": nil,
		"refMap": map[string]any{}, "worktreeCensus": []any{},
		"capturedAt": "2026-01-01T00:00:00Z",
	}
}

// legacySnapshot is the HEAD-seeded projection the tree-equation tests
// exercise rule 7 with; the seeded-projection behavior has its own
// engine-level tests.
func legacySnapshot(root, missionID string) func(string) (string, error) {
	return func(string) (string, error) {
		return wallSnapshot(gittree.Workspace{Dir: root}, missionID)
	}
}
