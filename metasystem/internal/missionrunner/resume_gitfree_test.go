package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

type resumeContinuityCall struct {
	method, statePath, root, ledgerPath, hash string
	sequence                                  int64
}

// The anchor answer is declared only after the actual state file passes the
// mission shape check. A clean close must verify the state it wrote.
type strictResumeContinuity struct {
	t       *testing.T
	calls   []resumeContinuityCall
	pin     string
	mission string
}

type resumeReads struct {
	t            *testing.T
	root, ledger string
	git          []wallGitReply
	blobs, auth  int
}

func (f *resumeReads) Git(root string, args ...string) (string, string, int) {
	f.t.Helper()
	if len(f.git) == 0 {
		f.t.Fatalf("undeclared Git call %q %q", root, args)
	}
	want := f.git[0]
	f.git = f.git[1:]
	if root != want.root || !reflect.DeepEqual(args, want.args) {
		f.t.Fatalf("Git call %q %q, want %q %q", root, args, want.root, want.args)
	}
	return want.stdout, want.stderr, want.code
}
func (f *resumeReads) BlobOID(string, []byte) (string, error) {
	f.t.Fatal("undeclared BlobOID call")
	return "", nil
}
func (f *resumeReads) LedgerTruth(string, map[string]any, string) (string, string, error) {
	f.t.Fatal("undeclared LedgerTruth call")
	return "", "", nil
}
func (f *resumeReads) LedgerBlobOID(root string, state map[string]any, path string) (string, error) {
	f.t.Helper()
	if f.blobs == 0 || root != f.root || path != f.ledger || state == nil {
		f.t.Fatalf("undeclared LedgerBlobOID(%q, %q)", root, path)
	}
	f.blobs--
	return "", mission.ErrNoAnchor
}
func (f *resumeReads) AuthenticateLedger(root string, state map[string]any, path string) error {
	f.t.Helper()
	if f.auth == 0 || root != f.root || path != f.ledger || state == nil {
		f.t.Fatalf("undeclared AuthenticateLedger(%q, %q)", root, path)
	}
	f.auth--
	return nil
}
func (f *resumeReads) done() {
	f.t.Helper()
	if len(f.git) != 0 || f.blobs != 0 || f.auth != 0 {
		f.t.Fatalf("unconsumed wall reads: git=%d blobs=%d auth=%d", len(f.git), f.blobs, f.auth)
	}
}

func (f *strictResumeContinuity) next(method, statePath, root, ledgerPath string) resumeContinuityCall {
	f.t.Helper()
	if len(f.calls) == 0 {
		f.t.Fatalf("undeclared continuity call %s(%q, %q, %q)", method, statePath, root, ledgerPath)
	}
	want := f.calls[0]
	f.calls = f.calls[1:]
	if want.method != method || want.statePath != statePath || want.root != root || want.ledgerPath != ledgerPath {
		f.t.Fatalf("continuity call %s(%q, %q, %q), want %+v", method, statePath, root, ledgerPath, want)
	}
	return want
}

func (f *strictResumeContinuity) Reconcile(statePath, root, ledgerPath string) (int, error) {
	f.next("Reconcile", statePath, root, ledgerPath)
	_, _, err := mission.VerifyStateShape(statePath)
	return 0, err
}

func (f *strictResumeContinuity) LedgerPin(root, missionID string) (string, error) {
	if missionID != f.mission {
		f.t.Fatalf("ledger pin mission %q, want %q", missionID, f.mission)
	}
	f.next("LedgerPin", "", root, "")
	return f.pin, nil
}

func (f *strictResumeContinuity) VerifyStateWithAnchor(statePath, root, ledgerPath string) (int64, string, error) {
	sequence, hash, err := mission.VerifyStateShape(statePath)
	if err != nil {
		f.t.Fatalf("final state shape: %v", err)
	}
	want := f.next("VerifyStateWithAnchor", statePath, root, ledgerPath)
	if sequence != want.sequence || hash != want.hash {
		f.t.Fatalf("anchored state (%d, %q), want (%d, %q)", sequence, hash, want.sequence, want.hash)
	}
	return want.sequence, want.hash, nil
}

type resumeWorkspaceFacts struct {
	wallWorkspace
	t               *testing.T
	root            string
	mission         string
	post            string
	anchored        []string
	expected        []string
	expectedAnchors []string
	beforeDecision  func()
}

func (f *resumeWorkspaceFacts) next(method string) {
	f.t.Helper()
	if len(f.expected) == 0 {
		f.t.Fatalf("undeclared workspace call %s", method)
	}
	want := f.expected[0]
	f.expected = f.expected[1:]
	if method != want {
		f.t.Fatalf("workspace call %s, want %s", method, want)
	}
}
func (f *resumeWorkspaceFacts) done() {
	f.t.Helper()
	if len(f.expected) != 0 || len(f.expectedAnchors) != 0 {
		f.t.Fatalf("unconsumed workspace calls: %v; anchor trees: %v", f.expected, f.expectedAnchors)
	}
}
func (f *resumeWorkspaceFacts) HistorySteeringFiles() ([]string, error) {
	f.next("HistorySteeringFiles")
	return nil, nil
}
func (f *resumeWorkspaceFacts) HeadCommit() (string, bool, error) {
	f.next("HeadCommit")
	return strings.Repeat("c", 40), false, nil
}
func (f *resumeWorkspaceFacts) TreeOf(rev string) (string, error) {
	f.next("TreeOf")
	if rev != strings.Repeat("c", 40) {
		f.t.Fatalf("undeclared TreeOf(%q)", rev)
	}
	return strings.Repeat("e", 40), nil
}
func (f *resumeWorkspaceFacts) FilterTree(tree string, paths []string) (string, error) {
	f.next("FilterTree")
	if !reflect.DeepEqual(paths, []string{missionLedgerRel(f.mission)}) {
		f.t.Fatalf("undeclared filter paths %v", paths)
	}
	switch tree {
	case strings.Repeat("e", 40), strings.Repeat("f", 40):
		return strings.Repeat("b", 40), nil
	case strings.Repeat("d", 40):
		return f.post, nil
	default:
		f.t.Fatalf("undeclared FilterTree(%q)", tree)
		return "", nil
	}
}
func (f *resumeWorkspaceFacts) SymbolicHead() (string, bool, error) {
	f.next("SymbolicHead")
	return "refs/heads/main", false, nil
}
func (f *resumeWorkspaceFacts) RefMap() (map[string]string, error) {
	f.next("RefMap")
	ns := mission.MissionRefNamespace(f.mission)
	return map[string]string{
		"refs/heads/main":            strings.Repeat("c", 40),
		ns + "state-anchors":         strings.Repeat("d", 40),
		ns + "turn-open-head":        strings.Repeat("c", 40),
		ns + strings.Repeat("b", 40): strings.Repeat("b", 40),
	}, nil
}
func (f *resumeWorkspaceFacts) WorktreeCensus() ([]gittree.WorktreeRecord, error) {
	f.next("WorktreeCensus")
	return nil, nil
}
func (f *resumeWorkspaceFacts) StagedTree() (string, error) {
	f.next("StagedTree")
	return strings.Repeat("f", 40), nil
}
func (f *resumeWorkspaceFacts) Prefix() (string, error) {
	f.next("Prefix")
	return "", nil
}
func (f *resumeWorkspaceFacts) TopLevel() (string, error) {
	f.next("TopLevel")
	return f.root, nil
}
func (f *resumeWorkspaceFacts) SnapshotSeeded(seed, expected string, paths []string) (string, error) {
	f.next("SnapshotSeeded")
	if f.beforeDecision != nil {
		f.beforeDecision()
		f.beforeDecision = nil
	}
	if seed != strings.Repeat("c", 40) || expected != strings.Repeat("b", 40) || len(paths) != 0 {
		f.t.Fatalf("undeclared snapshot (%q, %q, %v)", seed, expected, paths)
	}
	return strings.Repeat("d", 40), nil
}
func (f *resumeWorkspaceFacts) ChangedPaths(from, to string) ([]string, error) {
	f.next("ChangedPaths")
	if from != strings.Repeat("b", 40) || to != f.post || from == to {
		f.t.Fatalf("undeclared changed paths (%q, %q)", from, to)
	}
	return []string{"solo.go"}, nil
}
func (f *resumeWorkspaceFacts) Anchor(missionID, tree string) error {
	f.next("Anchor")
	if missionID != f.mission {
		f.t.Fatalf("anchor mission %q", missionID)
	}
	if len(f.expectedAnchors) == 0 || tree != f.expectedAnchors[0] {
		f.t.Fatalf("anchor tree %q, want next of %v", tree, f.expectedAnchors)
	}
	f.expectedAnchors = f.expectedAnchors[1:]
	f.anchored = append(f.anchored, tree)
	return nil
}

func resumeFileBed(t *testing.T, drift bool) (*Engine, string, string, *strictResumeContinuity, *resumeReads, *resumeWorkspaceFacts, string) {
	t.Helper()
	root := t.TempDir()
	e := &Engine{Root: root, Mission: "alpha"}
	statePath := filepath.Join(e.missionDir(), "state.json")
	ledgerPath := filepath.Join(e.missionDir(), "ledger.md")
	contractPath := e.approvedContractPath()
	contract := "```mission\ncandidate.branch=main\nstream.solo=Do solo\n```\n```mission-seal\ncandidate.branch=main\n```\n"
	writeText(t, contractPath, contract)
	sum := sha256.Sum256([]byte(contract))
	writeJSONFile(t, e.fencesPath(), map[string]any{"cycles": 0, "approvedContractSha256": hex.EncodeToString(sum[:])})
	if err := mission.InitLedger(ledgerPath, 5, 3); err != nil {
		t.Fatal(err)
	}
	origins := map[string]any{
		"headCommit": strings.Repeat("c", 40), "topTree": nil, "topStaged": nil,
		"refMap":         map[string]any{"refs/heads/main": strings.Repeat("c", 40)},
		"worktreeCensus": []any{}, "capturedAt": "2026-01-01T00:00:00Z",
	}
	if err := mission.InitStateWithBaseline(statePath, contractPath, ledgerPath, "", "", strings.Repeat("b", 40), origins); err != nil {
		t.Fatal(err)
	}
	state := readTestDoc(t, statePath)
	state["openTurn"] = map[string]any{
		"turnId": "alpha-t1-dead", "cycle": 1, "preTree": strings.Repeat("b", 40),
		"sequence": 0, "segment": 0, "openedAt": "2026-01-01T00:00:00Z",
		"headCommit": strings.Repeat("c", 40), "headTree": strings.Repeat("b", 40),
		"topTree": nil, "refMap": origins["refMap"], "topStaged": nil,
	}
	if _, err := e.writeState(statePath, state); err != nil {
		t.Fatal(err)
	}
	turnDir := filepath.Join(e.missionDir(), "turns", "alpha-t1-dead")
	writeJSONFile(t, filepath.Join(turnDir, "turn.json"), map[string]any{
		"missionId": e.Mission, "turnId": "alpha-t1-dead", "cycle": 1,
		"runtime": "fake", "model": "fixture", "status": "running",
	})
	writeJSONFile(t, e.birthRecordPath(), map[string]any{"missionId": e.Mission, "bornAt": "2026-01-01T00:00:00Z"})
	continuity := &strictResumeContinuity{t: t, mission: e.Mission, pin: strings.Repeat("a", 64),
		calls: []resumeContinuityCall{{method: "Reconcile", statePath: statePath, root: root, ledgerPath: ledgerPath}}}
	e.continuityFacts = continuity
	post := strings.Repeat("b", 40)
	if drift {
		post = strings.Repeat("a", 40)
	}
	workspace := &resumeWorkspaceFacts{t: t, root: root, mission: e.Mission, post: post}
	e.wallWorkspaceFactory = func(got string) wallWorkspace {
		if got != root {
			t.Fatalf("undeclared workspace root %q", got)
		}
		return workspace
	}
	reads := &resumeReads{t: t, root: root, ledger: ledgerPath, git: []wallGitReply{{root: root, args: []string{"config", "--local", "--type=bool", "--get", "core.fileMode"}, stdout: "true\n"}}}
	e.wallReadFacts = reads
	gitLog := filepath.Join(root, "git-invocations.log")
	gitBin := filepath.Join(root, "deny-bin")
	writeText(t, filepath.Join(gitBin, "git"), "#!/bin/sh\nprintf 'unexpected git\\n' >> '"+gitLog+"'\nexit 93\n")
	if err := os.Chmod(filepath.Join(gitBin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", gitBin)
	t.Cleanup(func() { assertNoGitInvocations(t, gitLog) })
	workspace.beforeDecision = func() {
		state := readTestDoc(t, statePath)
		if state["openTurn"] == nil {
			t.Fatal("turn closed before the wall decision")
		}
		_, _, cycles, err := mission.ParseLedger(ledgerPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(cycles) != 0 {
			t.Fatalf("cycle healed before the wall decision: %d ledger cycles", len(cycles))
		}
		turns, err := os.ReadDir(filepath.Join(e.missionDir(), "turns"))
		if err != nil {
			t.Fatal(err)
		}
		if len(turns) != 1 || turns[0].Name() != "alpha-t1-dead" {
			t.Fatalf("a new turn opened before the wall decision: %v", turns)
		}
	}
	return e, statePath, ledgerPath, continuity, reads, workspace, gitLog
}

func assertNoGitInvocations(t *testing.T, log string) {
	t.Helper()
	if body, err := os.ReadFile(log); err == nil {
		t.Fatalf("real Git dependency invoked: %s", body)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestGitRevParseUsesWallReads(t *testing.T) {
	t.Parallel()
	const root = "/virtual/resume-candidate"
	for _, tc := range []struct {
		name, stdout, stderr string
		code                 int
		wantSHA, wantErr     string
	}{
		{name: "trimmed answer", stdout: "  abc123\n", wantSHA: "abc123"},
		{name: "answered error", stdout: "fallback", stderr: "missing ref\n", code: 1, wantErr: "cannot resolve candidate sha: missing ref"},
		{name: "could not run", stderr: "spawn failed", code: -1, wantErr: "cannot resolve candidate sha: spawn failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := &strictWallReads{t: t, git: []wallGitReply{{
				root: root, args: []string{"-C", root, "rev-parse", "main"},
				stdout: tc.stdout, stderr: tc.stderr, code: tc.code,
			}}}
			sha, err := (&Engine{Root: root, wallReadFacts: facts}).gitRevParse("main")
			if sha != tc.wantSHA {
				t.Fatalf("SHA %q, want %q", sha, tc.wantSHA)
			}
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr || exitFor(err) != 3) {
				t.Fatalf("error %v, want %q with code 3", err, tc.wantErr)
			}
			facts.done()
		})
	}
}

// An unfinished turn reaches the wall before healing or opening another
// turn. A drifted workspace parks the mission with one taint entry.
func TestResumeInspectsOpenTurnFirst(t *testing.T) {
	e, statePath, ledger, continuity, reads, workspace, gitLog := resumeFileBed(t, true)
	root := e.Root
	workspace.expected = []string{
		"HistorySteeringFiles", "HeadCommit", "TreeOf", "FilterTree", "SymbolicHead", "RefMap",
		"WorktreeCensus", "StagedTree", "FilterTree", "Prefix", "SnapshotSeeded", "FilterTree",
		"ChangedPaths",
		"HistorySteeringFiles", "HeadCommit", "TreeOf", "FilterTree", "SymbolicHead", "RefMap",
		"WorktreeCensus", "StagedTree", "FilterTree", "Prefix", "SnapshotSeeded", "FilterTree",
		"Anchor", "ChangedPaths",
	}
	workspace.expectedAnchors = []string{strings.Repeat("a", 40)}
	reads.git = append(reads.git,
		wallGitReply{root: root, args: []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}, stdout: strings.Repeat("c", 40)},
		wallGitReply{root: root, args: []string{"-C", root, "rev-parse", "main"}, stdout: "  " + strings.Repeat("c", 40) + "\n"})
	e.anchorFn = func(gotState, gotLedger, identity string) error {
		if gotState != statePath || gotLedger != ledger || identity != "alpha-t1-dead" {
			t.Fatalf("park anchor %q %q %q", gotState, gotLedger, identity)
		}
		return nil
	}
	_, _, _, err := e.resumeState()
	if !reflect.DeepEqual(workspace.anchored, []string{strings.Repeat("a", 40)}) {
		t.Fatalf("workspace anchor effects: %v", workspace.anchored)
	}
	if err == nil || !strings.Contains(err.Error(), "failed the wall at resume") {
		t.Fatalf("resume drift: %v", err)
	}
	state := readTestDoc(t, statePath)
	if state["parkReason"] != "wall-violation" {
		t.Fatalf("park reason: %v", state["parkReason"])
	}
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("wall taint entries: %v", entries)
	}
	entry, ok := entries[0].(map[string]any)
	if !ok || entry["turnId"] != "alpha-t1-dead" || entry["resolution"] != nil {
		t.Fatalf("wall taint does not identify the unresolved original turn: %v", entries[0])
	}
	if reason, _ := entry["reason"].(string); !strings.Contains(reason, "solo.go") {
		t.Fatalf("wall taint reason does not name solo.go: %v", entry["reason"])
	}
	wall := readTestDoc(t, filepath.Join(e.missionDir(), "turns", "alpha-t1-dead", "wall.json"))
	if wall["verdict"] != "violated" {
		t.Fatalf("wall verdict: %v", wall["verdict"])
	}
	if violation, _ := wall["violation"].(string); !strings.Contains(violation, "solo.go") {
		t.Fatalf("wall violation does not name solo.go: %v", wall["violation"])
	}
	if !reflect.DeepEqual(wall["unaccounted"], []any{"solo.go"}) {
		t.Fatalf("wall unaccounted paths: %v", wall["unaccounted"])
	}
	askPaths, err := filepath.Glob(filepath.Join(asksDirPath(root, e.Mission), "*.json"))
	if err != nil || len(askPaths) != 1 {
		t.Fatalf("wall asks: %v, glob error: %v", askPaths, err)
	}
	ask := readTestDoc(t, askPaths[0])
	askID, ok := ask["askId"].(string)
	if !ok || askID == "" || filepath.Base(askPaths[0]) != askID+".json" ||
		ask["answeredAt"] != nil || ask["reasonClass"] != "wall-violation" {
		t.Fatalf("open wall-violation ask: %v", ask)
	}
	entryID, entryIDOK := jsonInt(entry["taintId"])
	askTaintID, askTaintIDOK := jsonInt(ask["taintId"])
	if !entryIDOK || !askTaintIDOK || entryID != askTaintID {
		t.Fatalf("ask does not bind to the original turn's taint: ask=%v, entry=%v", ask, entry)
	}
	if !reflect.DeepEqual(state["waitingList"], []any{askID}) {
		t.Fatalf("state waiting list does not name the bound ask: %v", state["waitingList"])
	}
	turn := readTestDoc(t, filepath.Join(e.missionDir(), "turns", "alpha-t1-dead", "turn.json"))
	if turn["turnId"] != "alpha-t1-dead" || turn["status"] != "failed" ||
		turn["outcome"] != "wall-violation" || turn["error"] != "wall-violation" {
		t.Fatalf("original turn wall failure: %v", turn)
	}
	if detail, _ := turn["detail"].(string); !strings.Contains(detail, "solo.go") {
		t.Fatalf("original turn failure does not name solo.go: %v", turn["detail"])
	}
	if workspace.beforeDecision != nil {
		t.Fatal("the drift wall decision did not run")
	}
	workspace.done()
	if len(continuity.calls) != 0 {
		t.Fatalf("unused continuity declarations: %+v", continuity.calls)
	}
	reads.done()
	assertNoGitInvocations(t, gitLog)
}

// A clean crashed turn closes its marker and verifies the written state.
func TestResumeClosesCleanUnacceptedTurn(t *testing.T) {
	e, statePath, ledger, continuity, reads, workspace, gitLog := resumeFileBed(t, false)
	root := e.Root
	workspace.expected = []string{
		"HistorySteeringFiles", "HeadCommit", "TreeOf", "FilterTree", "SymbolicHead", "RefMap",
		"WorktreeCensus", "StagedTree", "FilterTree", "Prefix", "SnapshotSeeded", "FilterTree",
		"Prefix", "TopLevel",
		"HistorySteeringFiles", "HeadCommit", "TreeOf", "FilterTree", "SymbolicHead", "RefMap",
		"WorktreeCensus", "StagedTree", "FilterTree", "Prefix", "SnapshotSeeded", "FilterTree",
	}
	continuity.calls = append(continuity.calls,
		resumeContinuityCall{method: "LedgerPin", root: root},
		resumeContinuityCall{method: "VerifyStateWithAnchor", statePath: statePath, root: root, ledgerPath: ledger})
	reads.blobs = 3
	for i := 0; i < 2; i++ {
		reads.auth++
		reads.git = append(reads.git, wallGitReply{root: root, args: []string{"cat-file", "-t", strings.Repeat("b", 40)}, stdout: "tree\n"})
	}
	reads.git = append(reads.git, wallGitReply{root: root, args: []string{"update-ref", "-d", mission.MissionRefNamespace(e.Mission) + "turn-open-head"}})
	e.anchorFn = func(gotState, gotLedger, identity string) error {
		if gotState != statePath || gotLedger != ledger || identity != "alpha-t1-dead" {
			t.Fatalf("close anchor %q %q %q", gotState, gotLedger, identity)
		}
		sequence, hash, err := mission.VerifyStateShape(statePath)
		if err != nil {
			t.Fatal(err)
		}
		continuity.calls[len(continuity.calls)-1].sequence = sequence
		continuity.calls[len(continuity.calls)-1].hash = hash
		return nil
	}
	_, _, state, err := e.resumeState()
	if err != nil {
		t.Fatalf("clean resume: %v", err)
	}
	if state["openTurn"] != nil {
		t.Fatalf("open turn survived: %v", state["openTurn"])
	}
	if workspace.beforeDecision != nil {
		t.Fatal("the clean wall decision did not run")
	}
	workspace.done()
	if len(continuity.calls) != 0 {
		t.Fatalf("unused continuity declarations: %+v", continuity.calls)
	}
	reads.done()
	assertNoGitInvocations(t, gitLog)
}
