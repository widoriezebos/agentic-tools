package missionrunner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

const hostGateCommit = "1111111111111111111111111111111111111111"
const hostCandidateCommit = "cccccccccccccccccccccccccccccccccccccccc"
const hostTree = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

type hostCycleSource struct {
	t                  *testing.T
	root, contractPath string
	files              map[string][]byte
	candidateSHA       string
	candidateFiles     map[string][]byte
	signed             []byte
	created, removed   []string
	refs               map[string]string
	missingRefLookups  int
}

// This bed has no accepted goal tree or legacy goal file. The refresh still
// attempts the real goal read; the missing canonical branch is its raw answer.
type hostCycleGoals struct {
	t                  *testing.T
	mu                 sync.Mutex
	captures, accepted int
	captured           map[string]bool
}

func (r *hostCycleGoals) unexpected(op string) error {
	err := fmt.Errorf("unexpected goal repository %s", op)
	r.t.Error(err)
	return err
}
func (r *hostCycleGoals) Capture(opid string) (string, error) {
	r.mu.Lock()
	r.captures++
	if r.captured == nil {
		r.captured = make(map[string]bool)
	}
	r.captured[opid] = true
	r.mu.Unlock()
	return "", fmt.Errorf("canonical goal branch is absent")
}
func (r *hostCycleGoals) Accepted() (string, bool, error) {
	r.mu.Lock()
	r.accepted++
	r.mu.Unlock()
	return "", false, nil
}
func (r *hostCycleGoals) Files(string, ...string) (map[string][]byte, error) {
	return nil, r.unexpected("Files")
}
func (r *hostCycleGoals) Build(string, string, []goal.Change, string) (string, error) {
	return "", r.unexpected("Build")
}
func (r *hostCycleGoals) Publish(string, string) (goal.CASOutcome, error) {
	return "", r.unexpected("Publish")
}
func (r *hostCycleGoals) AcceptedCAS(string, string) error { return r.unexpected("AcceptedCAS") }
func (r *hostCycleGoals) IsAncestor(string, string) (bool, error) {
	return false, r.unexpected("IsAncestor")
}
func (r *hostCycleGoals) TrailerPresent(string, string) (bool, error) {
	return false, r.unexpected("TrailerPresent")
}
func (r *hostCycleGoals) CommitWithTrailer(string, string, string) (string, error) {
	return "", r.unexpected("CommitWithTrailer")
}
func (r *hostCycleGoals) CommitTime(string) (time.Time, error) {
	return time.Time{}, r.unexpected("CommitTime")
}

// A failed capture still owns a nonce that the read-side advance releases.
func (r *hostCycleGoals) Release(opid string) error {
	r.mu.Lock()
	captured := r.captured[opid]
	delete(r.captured, opid)
	r.mu.Unlock()
	if !captured {
		return r.unexpected("Release without Capture")
	}
	return nil
}

func writeHostCycleFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return testexec.WriteFile(path, data, mode)
}

func (f *hostCycleSource) source() *contract.Source {
	return &contract.Source{
		Repository: func(path string) (string, error) {
			if path != f.contractPath {
				f.t.Fatalf("contract repository path %q", path)
			}
			return f.root, nil
		},
		Output: f.output, Try: f.try, Fetch: func(root string) (string, error) {
			if root != f.root {
				f.t.Fatalf("origin fetch root %q", root)
			}
			return "", nil
		},
	}
}

func (f *hostCycleSource) currentCandidateSHA() string {
	if f.candidateSHA != "" {
		return f.candidateSHA
	}
	return hostCandidateCommit
}

func (f *hostCycleSource) output(root string, args ...string) (string, error) {
	f.t.Helper()
	if root == f.root {
		switch {
		case reflect.DeepEqual(args, []string{"rev-parse", "instruments^{commit}"}):
			if f.refs != nil && f.refs["refs/tags/instruments"] == "" {
				f.missingRefLookups++
				return "", fmt.Errorf("unknown revision instruments")
			}
			return hostGateCommit + "\n", nil
		case reflect.DeepEqual(args, []string{"rev-parse", "main^{commit}"}):
			return f.currentCandidateSHA() + "\n", nil
		case reflect.DeepEqual(args, []string{"branch", "--show-current"}):
			return "main\n", nil
		case reflect.DeepEqual(args, []string{"symbolic-ref", "refs/remotes/origin/HEAD"}):
			return "refs/remotes/origin/main\n", nil
		case reflect.DeepEqual(args, []string{"ls-tree", "-r", "--name-only", "-z", hostGateCommit}):
			paths := make([]string, 0, len(f.files))
			for path := range f.files {
				paths = append(paths, path)
			}
			sort.Strings(paths)
			return strings.Join(paths, "\x00") + "\x00", nil
		case len(args) == 2 && args[0] == "show" && strings.HasPrefix(args[1], hostGateCommit+":"):
			path := strings.TrimPrefix(args[1], hostGateCommit+":")
			if data, ok := f.files[path]; ok {
				return string(data), nil
			}
		case len(args) == 6 && reflect.DeepEqual(args[:4], []string{"worktree", "add", "--detach", "--quiet"}) && args[5] == f.currentCandidateSHA():
			f.checkRegistry(args[4])
			if err := f.materialize(args[4]); err != nil {
				return "", err
			}
			f.created = append(f.created, args[4])
			return "", nil
		}
	}
	if len(args) == 6 && root == f.lastCreated() && reflect.DeepEqual(args, []string{"checkout", "--quiet", hostGateCommit, "--", "scripts/gate.sh", "truth/reference.txt"}) {
		f.checkRegistry(root)
		for _, path := range args[4:] {
			mode := os.FileMode(0o644)
			if path == "scripts/gate.sh" {
				mode = 0o755
			}
			if err := writeHostCycleFile(filepath.Join(root, path), f.files[path], mode); err != nil {
				return "", err
			}
		}
		return "", nil
	}
	f.t.Fatalf("undeclared contract output %q %q", root, args)
	return "", nil
}

func (f *hostCycleSource) lastCreated() string {
	if len(f.created) == 0 {
		return ""
	}
	return f.created[len(f.created)-1]
}

func (f *hostCycleSource) materialize(worktree string) error {
	if f.candidateFiles != nil {
		for path, data := range f.candidateFiles {
			mode := os.FileMode(0o644)
			if strings.HasSuffix(path, ".sh") {
				mode = 0o755
			}
			if err := writeHostCycleFile(filepath.Join(worktree, path), data, mode); err != nil {
				return err
			}
		}
		return nil
	}
	for _, path := range []string{"scripts/gate.sh", "truth/reference.txt"} {
		mode := os.FileMode(0o644)
		if path == "scripts/gate.sh" {
			mode = 0o755
		}
		if err := writeHostCycleFile(filepath.Join(worktree, path), f.files[path], mode); err != nil {
			return err
		}
	}
	return nil
}

func (f *hostCycleSource) checkRegistry(worktree string) {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, "artifacts", "agents", "measure-worktrees.jsonl"))
	if err != nil {
		f.t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		var row struct{ Path, SHA, GateRef string }
		if json.Unmarshal([]byte(line), &row) == nil && row.Path == worktree && row.SHA == f.currentCandidateSHA() && row.GateRef == hostGateCommit {
			return
		}
	}
	f.t.Fatalf("worktree %q was materialized before registration: %s", worktree, data)
}

func (f *hostCycleSource) try(root string, args ...string) (string, int) {
	f.t.Helper()
	contractRel, err := filepath.Rel(f.root, f.contractPath)
	if err != nil {
		f.t.Fatal(err)
	}
	if root == f.root && len(args) == 2 && args[0] == "show" && args[1] == "refs/remotes/origin/main:"+filepath.ToSlash(contractRel) {
		if f.signed == nil {
			f.t.Fatal("origin requested before signed bytes were published")
		}
		return string(f.signed), 0
	}
	if root == f.root && reflect.DeepEqual(args, []string{"worktree", "remove", "--force", f.lastCreated()}) {
		f.removed = append(f.removed, args[3])
		return "", 0
	}
	f.t.Fatalf("undeclared contract try %q %q", root, args)
	return "", 1
}

func (f *hostCycleSource) done() {
	f.t.Helper()
	if !reflect.DeepEqual(f.created, f.removed) {
		f.t.Errorf("measurement worktrees created %q, removed %q", f.created, f.removed)
	}
	for _, worktree := range f.created {
		if _, err := os.Stat(filepath.Dir(worktree)); !os.IsNotExist(err) {
			f.t.Errorf("measurement scratch remains: %q (%v)", worktree, err)
		}
	}
}

// newGitFreePreflightBed prepares an authored, signed contract and live
// supervision facts without pinning them to a mission.
func newGitFreePreflightBed(t *testing.T, behavior string) (*Engine, *hostCycleSource) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{Root: root, Mission: "alpha"}
	f := &hostCycleSource{t: t, root: root, contractPath: e.contractPath(), files: map[string][]byte{}}
	f.files["scripts/gate.sh"] = []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf 'metric=score=1\\nmetric=audit=1\\n'\n")
	f.files["truth/reference.txt"] = []byte("certified truth\n")
	f.files["scripts/agents/arm-supervision.sh"] = []byte("#!/usr/bin/env bash\nset -euo pipefail\nif [[ ${1:-} == fingerprint ]]; then printf 'fixture-fingerprint\\n'; exit 0; fi\nprintf 'up outcome=armed authority=writer\\n'\n")
	rules, err := os.ReadFile(filepath.Join("..", "..", "docs", "project-rules.md"))
	if err != nil {
		t.Fatal(err)
	}
	f.files["docs/project-rules.md"] = rules
	for path, data := range f.files {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") {
			mode = 0o755
		}
		if err := writeHostCycleFile(filepath.Join(root, path), data, mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := testexec.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nrole.default.runtime=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	document := fixtureContract
	if behavior == "FAKEHOST:park-request" {
		// A passing gate completes the only stream before its accepted park
		// request can conclude the mission.
		document = strings.Replace(document, "gate.threshold.score=>=1", "gate.threshold.score=>=2", 1)
	}
	if behavior != "" {
		document = strings.Replace(document, "stream.primary=Reach the acceptance score.", "stream.primary=Reach the acceptance score. "+behavior, 1)
	}
	if err := writeHostCycleFile(f.contractPath, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	e.contractSource = f.source()
	sha, err := contract.SealWithSource(f.contractPath, e.contractSource)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	approval := "\nApproval: name=Fixture Human; date=2026-08-12; contract-sha256=" + sha + "\n"
	handle, err := os.OpenFile(f.contractPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.WriteString(approval); err != nil {
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	f.signed, err = os.ReadFile(f.contractPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFreshSupervision(t, e)
	installHostCycleRepository(t, e, f)
	return e, f
}

func buildGitFreeHostCycle(t *testing.T, behavior string) *Engine {
	t.Helper()
	e, f := newGitFreePreflightBed(t, behavior)
	equipFullCycleFiles(t, e)
	goalRepository := &hostCycleGoals{t: t}
	e.goalSource = &mission.GoalSource{Endpoint: goal.Endpoint{
		Root: e.Root, Remote: "origin", Branch: "refs/heads/main", Repository: goalRepository,
	}, Machine: "fixture-machine"}
	t.Cleanup(func() {
		goalRepository.mu.Lock()
		captures, accepted, unreleased := goalRepository.captures, goalRepository.accepted, len(goalRepository.captured)
		goalRepository.mu.Unlock()
		if captures == 0 || accepted == 0 || unreleased != 0 {
			t.Errorf("serving goal source use/cleanup: captures=%d accepted=%d unreleased=%d", captures, accepted, unreleased)
		}
	})
	if err := e.armAndPreflight("start"); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	t.Cleanup(f.done)
	return e
}

type hostCycleWorkspace struct {
	t           *testing.T
	root        string
	mission     string
	contractOID string
	refs        map[string]string
}

func (w *hostCycleWorkspace) missionID() string {
	if w.mission == "" {
		return "alpha"
	}
	return w.mission
}
func (w *hostCycleWorkspace) treeRef() string {
	return mission.MissionRefNamespace(w.missionID()) + hostTree
}
func (w *hostCycleWorkspace) stateRef() string {
	return mission.MissionRefNamespace(w.missionID()) + "state-anchors"
}
func (w *hostCycleWorkspace) openRef() string {
	return mission.MissionRefNamespace(w.missionID()) + "turn-open-head"
}

func hostCycleTreeRef() string { return "refs/metasystem/missions/alpha/" + hostTree }

const hostCycleStateRef = "refs/metasystem/missions/alpha/state-anchors"

func (w *hostCycleWorkspace) unexpected(name string, args ...any) {
	w.t.Fatalf("undeclared wall %s %v", name, args)
}
func (w *hostCycleWorkspace) Snapshot(base string) (string, error) {
	if base != "HEAD" {
		w.unexpected("Snapshot", base)
	}
	return hostTree, nil
}
func (w *hostCycleWorkspace) FilterTree(tree string, paths []string) (string, error) {
	ledger := missionLedgerRel(w.missionID())
	if tree != hostTree ||
		!reflect.DeepEqual(paths, []string{ledger}) &&
			!reflect.DeepEqual(paths, []string{ledger, "plans/mission-" + w.missionID() + ".contract.md"}) {
		w.unexpected("FilterTree", tree, paths)
	}
	return hostTree, nil
}
func (w *hostCycleWorkspace) HeadTree() (string, error) { return hostTree, nil }
func (w *hostCycleWorkspace) HeadCommit() (string, bool, error) {
	return hostCandidateCommit, false, nil
}
func (w *hostCycleWorkspace) TreeOf(rev string) (string, error) {
	if rev != hostCandidateCommit {
		w.unexpected("TreeOf", rev)
	}
	return hostTree, nil
}
func (w *hostCycleWorkspace) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	if tree == hostTree && reflect.DeepEqual(paths, []string{missionLedgerRel(w.missionID())}) {
		return map[string]gittree.Entry{}, nil
	}
	if tree != hostTree || !reflect.DeepEqual(paths, []string{"plans/mission-" + w.missionID() + ".contract.md"}) {
		w.unexpected("Entries", tree, paths)
	}
	return map[string]gittree.Entry{paths[0]: {Mode: "100644", OID: w.contractOID}}, nil
}
func (w *hostCycleWorkspace) StagedTree() (string, error) { return hostTree, nil }
func (w *hostCycleWorkspace) SnapshotSeeded(seed, expected string, paths []string) (string, error) {
	if seed != hostCandidateCommit || expected != hostTree || !reflect.DeepEqual(paths, []string{}) {
		w.unexpected("SnapshotSeeded", seed, expected, paths)
	}
	return hostTree, nil
}
func (w *hostCycleWorkspace) RefMap() (map[string]string, error) {
	refs := make(map[string]string, len(w.refs))
	for ref, oid := range w.refs {
		refs[ref] = oid
	}
	return refs, nil
}
func (w *hostCycleWorkspace) SymbolicHead() (string, bool, error) {
	return "refs/heads/main", false, nil
}
func (w *hostCycleWorkspace) WorktreeCensus() ([]gittree.WorktreeRecord, error) {
	return []gittree.WorktreeRecord{}, nil
}
func (w *hostCycleWorkspace) Prefix() (string, error)   { return "", nil }
func (w *hostCycleWorkspace) TopLevel() (string, error) { return w.root, nil }
func (w *hostCycleWorkspace) TopStagedPosture() (gittree.StagedPosture, error) {
	w.unexpected("TopStagedPosture")
	return gittree.StagedPosture{}, nil
}
func (w *hostCycleWorkspace) HistorySteeringFiles() ([]string, error) { return nil, nil }
func (w *hostCycleWorkspace) ChangedPaths(from, to string) ([]string, error) {
	if from != hostTree || to != hostTree {
		w.unexpected("ChangedPaths", from, to)
	}
	return nil, nil
}
func (w *hostCycleWorkspace) Anchor(missionID, tree string) error {
	if missionID != w.missionID() || tree != hostTree {
		w.unexpected("Anchor", missionID, tree)
	}
	w.refs[w.treeRef()] = tree
	return nil
}
func (w *hostCycleWorkspace) AnchorCommit(missionID, name, commit string) error {
	if missionID != w.missionID() || name != "turn-open-head" || commit != hostCandidateCommit {
		w.unexpected("AnchorCommit", missionID, name, commit)
	}
	if w.refs[w.stateRef()] == "" {
		w.t.Fatal("turn-open anchor preceded state anchor")
	}
	w.refs[w.openRef()] = commit
	return nil
}
func (w *hostCycleWorkspace) FileAt(tree, path string) ([]byte, bool, error) {
	w.unexpected("FileAt", tree, path)
	return nil, false, nil
}
func (w *hostCycleWorkspace) Apply(tree string, patch []byte) (string, error) {
	w.unexpected("Apply", tree)
	return "", nil
}
func (w *hostCycleWorkspace) MaterializePaths(tree string, paths []string) error {
	w.unexpected("MaterializePaths", tree, paths)
	return nil
}

type hostCycleReads struct {
	t         *testing.T
	root      string
	workspace *hostCycleWorkspace
}

func (r *hostCycleReads) Git(root string, args ...string) (string, string, int) {
	if root == r.root && reflect.DeepEqual(args, []string{"update-ref", "-d", r.workspace.openRef()}) {
		if r.workspace.refs[r.workspace.openRef()] == "" {
			r.t.Fatal("turn-open ref was removed before its anchor effect")
		}
		delete(r.workspace.refs, r.workspace.openRef())
		return "", "", 0
	}
	if root == r.root && reflect.DeepEqual(args, []string{"-C", r.root, "rev-parse", "main"}) {
		return hostCandidateCommit + "\n", "", 0
	}
	if root == r.root && reflect.DeepEqual(args, []string{"cat-file", "-t", hostTree}) {
		return "tree\n", "", 0
	}
	if root == r.root && reflect.DeepEqual(args, []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}) {
		return hostCandidateCommit + "\n", "", 0
	}
	if root == r.root && reflect.DeepEqual(args, []string{"config", "--local", "--type=bool", "--get", "core.fileMode"}) {
		return "true\n", "", 0
	}
	if root == r.root && reflect.DeepEqual(args, []string{"for-each-ref", "--format=%(refname)", mission.MissionRefNamespace(r.workspace.missionID())}) {
		var refs []string
		for ref := range r.workspace.refs {
			if strings.HasPrefix(ref, mission.MissionRefNamespace(r.workspace.missionID())) {
				refs = append(refs, ref)
			}
		}
		sort.Strings(refs)
		if len(refs) > 0 {
			return strings.Join(refs, "\n") + "\n", "", 0
		}
		return "", "", 0
	}
	r.t.Fatalf("undeclared wall Git %q %q", root, args)
	return "", "", 1
}
func (r *hostCycleReads) BlobOID(root string, content []byte) (string, error) {
	if root != r.root {
		r.t.Fatal("blob root", root)
	}
	return birthBlobOID(content), nil
}
func (r *hostCycleReads) LedgerTruth(root string, state map[string]any, path string) (string, string, error) {
	if root != r.root {
		r.t.Fatal("ledger root", root)
	}
	return "", "", mission.ErrNoAnchor
}
func (r *hostCycleReads) LedgerBlobOID(root string, state map[string]any, path string) (string, error) {
	if root != r.root {
		r.t.Fatal("ledger root", root)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return birthBlobOID(data), nil
}
func (r *hostCycleReads) AuthenticateLedger(root string, state map[string]any, path string) error {
	if root != r.root {
		r.t.Fatal("ledger root", root)
	}
	return nil
}

type hostCycleBirth struct {
	t         *testing.T
	root      string
	workspace *hostCycleWorkspace
}

func (b *hostCycleBirth) CaptureAdmissionOrigins(root, missionID string) (map[string]any, error) {
	if root != b.root || missionID != "alpha" {
		b.t.Fatal("birth origins", root, missionID)
	}
	origins := testAdmissionOrigins()
	refs, _ := b.workspace.RefMap()
	for ref := range refs {
		if strings.HasPrefix(ref, mission.MissionRefNamespace(missionID)) {
			b.t.Fatalf("birth origins include preexisting mission ref %s", ref)
		}
	}
	origins["refMap"] = mission.RecordableRefMap(refs, missionID)
	return origins, nil
}
func (b *hostCycleBirth) AnchorInitial(root, missionID, tree string) error {
	if root != b.root || missionID != "alpha" || tree != hostTree {
		b.t.Fatal("birth anchor", root, missionID, tree)
	}
	b.workspace.refs[hostCycleTreeRef()] = tree
	return nil
}
func (b *hostCycleBirth) DropAnchors(root, missionID string) error {
	if root != b.root || missionID != "alpha" {
		b.t.Fatal("birth drop", root, missionID)
	}
	for ref := range b.workspace.refs {
		if strings.HasPrefix(ref, mission.MissionRefNamespace(missionID)) {
			delete(b.workspace.refs, ref)
		}
	}
	return nil
}

type hostCycleContinuity struct {
	t    *testing.T
	root string
}

func (c *hostCycleContinuity) Reconcile(state, root, ledger string) (int, error) {
	if root != c.root {
		c.t.Fatal("reconcile root", root)
	}
	_, _, err := mission.VerifyStateShape(state)
	return 0, err
}
func (c *hostCycleContinuity) LedgerPin(root, missionID string) (string, error) {
	c.t.Fatal("unexpected ledger pin")
	return "", nil
}
func (c *hostCycleContinuity) VerifyStateWithAnchor(state, root, ledger string) (int64, string, error) {
	if root != c.root {
		c.t.Fatal("verify root", root)
	}
	return mission.VerifyStateShape(state)
}

func installHostCycleRepository(t *testing.T, e *Engine, f *hostCycleSource) {
	t.Helper()
	w := &hostCycleWorkspace{t: t, root: e.Root, contractOID: birthBlobOID(f.signed), refs: map[string]string{"refs/heads/main": hostCandidateCommit}}
	e.wallWorkspaceFactory = func(root string) wallWorkspace {
		if root != e.Root {
			t.Fatalf("workspace root %q", root)
		}
		return w
	}
	e.wallReadFacts = &hostCycleReads{t: t, root: e.Root, workspace: w}
	e.birthEffects = &hostCycleBirth{t: t, root: e.Root, workspace: w}
	e.continuityFacts = &hostCycleContinuity{t: t, root: e.Root}
	e.anchorFn = func(state, ledger, name string) error {
		if name == "" || state != filepath.Join(e.missionDir(), "state.json") || ledger != filepath.Join(e.missionDir(), "ledger.md") {
			return fmt.Errorf("unexpected state anchor: %q %q %q", state, ledger, name)
		}
		_, _, err := mission.VerifyStateShape(state)
		if err == nil {
			if w.refs[hostCycleTreeRef()] != hostTree {
				return fmt.Errorf("state anchor preceded birth tree anchor")
			}
			w.refs[hostCycleStateRef] = hostCandidateCommit
		}
		return err
	}
}
