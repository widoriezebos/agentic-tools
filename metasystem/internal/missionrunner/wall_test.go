package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
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
	inspection, err := inspectWall(root, "demo", pre, wallState(), nil, map[string]bool{}, "")
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
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "")
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
	inspection, err := inspectWall(root, "demo", pre, wallState(), nil, map[string]bool{}, "")
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
	inspection, err := inspectWall(root, "demo", pre, wallState(), nil, map[string]bool{"docs/note.md": true}, "")
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" {
		t.Fatalf("a declared artifact must pass: %+v", inspection)
	}

	// The same path under a consumed authorization refuses (HIW-R4-05):
	// a declared artifact never overwrites reviewed bytes.
	writeText(t, filepath.Join(root, "docs", "note.md"), "reviewed bytes\n")
	reviewed := snapshotTree(t, root)
	digest := wallAuthorization(t, root, "demo", pre, reviewed, nil)
	writeText(t, filepath.Join(root, "docs", "note.md"), "host overwrote the review\n")
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err = inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{"docs/note.md": true}, "")
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
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "")
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
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "")
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
// need (HIW-O1): the wall gate reads the anchored pre-tree from it, so any
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
	doc["openTurn"] = map[string]any{
		"turnId": turnID, "cycle": cycle, "preTree": tree,
		"sequence": sequence, "segment": segment,
		"openedAt": "2026-08-18T00:00:00Z",
	}
	delete(doc, "integrity")
	source := statePath + ".open-turn.src"
	if err := atomicWriteJSON(source, doc); err != nil {
		t.Fatal(err)
	}
	if err := mission.WriteState(statePath, source, hash); err != nil {
		t.Fatalf("open fixture turn: %v", err)
	}
}

// seedWallEvidence writes the minimal passed wall.json a direct
// builder-call bed needs: since F-4 a conclusion without evidence is an
// error, exactly as the engine paths guarantee by running the gate first.
func seedWallEvidence(t *testing.T, root, mission, turnID string) {
	t.Helper()
	tree := strings.Repeat("a", 40)
	writeJSONFile(t, filepath.Join(missionDirPath(root, mission), "turns", turnID, "wall.json"),
		map[string]any{"verdict": "passed", "preTree": tree, "expectedTree": tree,
			"postTree": tree, "orderedDigests": []any{}})
}

// The resume binding order (critique F-1): an unfinished open turn is
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

// The staleness predicate (critique F-2): an authorization based on an
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
	inspection, err := inspectWall(root, "demo", pre, state, certified, map[string]bool{}, "")
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
	inspection, err = inspectWall(root, "demo", pre, state, certified, map[string]bool{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Violation != "" {
		t.Fatalf("a disjoint delayed authorization must consume: %+v", inspection)
	}
}

// Tampered evidence refuses (critiques F-2/F-3): a record claiming a
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
	inspection, err := inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "")
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
	inspection, err = inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "")
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
	inspection, err = inspectWall(root, "demo", pre, wallState(), certified, map[string]bool{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "do not match their digest") {
		t.Fatalf("a rewritten record must refuse: %+v", inspection)
	}
}

// Fail-closed acceptance (critique F-4): no wall evidence, no conclusion.
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
	if err := mission.InitState(statePath, engine.approvedContractPath(), ledgerPath, "", "main"); err != nil {
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

// The changed-then-reverted ambiguity (round-2 finding 4): a path the
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
	inspection, err := inspectWall(root, "demo", preJ, state, certified, map[string]bool{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "is stale") {
		t.Fatalf("a changed-then-reverted path must refuse the old authorization: %+v", inspection)
	}
}

// A declared artifact path beneath a symlinked ancestor refuses (round-2
// finding 6): the write escaped the repository, so the tree shows nothing.
func TestWallRefusesSymlinkedArtifactAncestry(t *testing.T) {
	root := wallRepo(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	pre := snapshotTree(t, root)
	inspection, err := inspectWall(root, "demo", pre, wallState(), nil, map[string]bool{"docs/note.md": true}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "traverses the symlink docs") {
		t.Fatalf("a symlinked ancestor must refuse: %+v", inspection)
	}
}

// The ledger-ahead violation window (round-2 finding 3): the crashed
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
	if err := mission.AppendCycle(ledgerPath, 1, "no-progress", strings.Repeat("a", 40), "score=0", ""); err != nil {
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

// A CLEAN ledger-ahead crash resumes without taint (round-3 finding 1):
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
	// The TRUE production crash window (round-5/6): the runner RESERVED
	// cycle 1, opened the turn, appended its block, then died before the
	// state write — reservation and the open marker answer for the block.
	if err := mission.ReserveCycle(engine.Root, engine.Mission); err != nil {
		t.Fatal(err)
	}
	if err := mission.AppendCycle(ledgerPath, 1, "no-progress", strings.Repeat("a", 40), "score=0", ""); err != nil {
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

// The filtered identity is append-stable (round-3 finding 2): tracked
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

// The resolution segment fence (round-3 finding 3): the live taint
// segment advances at resolution, before any new-segment acceptance, and
// an old-segment authorization refuses even on a repeated tree.
func TestWallSegmentFenceAfterResolution(t *testing.T) {
	root := wallRepo(t)
	pre := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n\nfunc F() {}\n")
	post := snapshotTree(t, root)
	writeText(t, filepath.Join(root, "main.go"), "package main\n")
	digest := wallAuthorization(t, root, "demo", pre, post, nil)
	writeText(t, filepath.Join(root, "main.go"), "package main\n\nfunc F() {}\n")

	state := map[string]any{
		"turnLog": []any{},
		"workspaceTaint": map[string]any{"next": 2, "segment": 1, "entries": []any{
			map[string]any{"taintId": 1, "turnId": "demo-t1", "reason": "drift",
				"setAt": "2026-08-18T00:00:00Z", "resolution": map[string]any{
					"variant": "restore", "treeId": pre, "resolvedAt": "2026-08-18T00:00:00Z",
					"resolvedBy": "Wido", "reason": "restored"}},
		}},
	}
	certified := []map[string]any{{"jobId": "job-w", "verdict": "accepted", "authorizationDigest": digest}}
	inspection, err := inspectWall(root, "demo", pre, state, certified, map[string]bool{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection.Violation, "predates a workspace resolution") {
		t.Fatalf("an old-segment authorization must refuse after a resolution: %+v", inspection)
	}
}

// The in-turn ledger guard (round-4 finding 2): the baseline is the
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

	final, violated, err := engine.wallGate(statePath, ledgerPath, "alpha-t1-live", turnDir, 1, nil, false)
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

// Answering a superseded ask refuses BY NAME toward its successor
// (issue #11): the refusal must be the superseded guard, not a
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
