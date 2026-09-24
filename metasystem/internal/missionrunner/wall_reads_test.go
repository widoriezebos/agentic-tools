package missionrunner

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

type wallGitReply struct {
	root           string
	args           []string
	stdout, stderr string
	code           int
}

type wallByteReply struct {
	root    string
	content []byte
	oid     string
	err     error
}

type wallLedgerReply struct {
	root, path             string
	state                  map[string]any
	content                map[string]any
	anchored, current, oid string
	err                    error
}

// Each transcript contains answers, never a model of Git or ledger state.
type strictWallReads struct {
	t     *testing.T
	git   []wallGitReply
	bytes []wallByteReply
	truth []wallLedgerReply
	blobs []wallLedgerReply
	auth  []wallLedgerReply
}

func (f *strictWallReads) done() {
	f.t.Helper()
	if len(f.git) != 0 || len(f.bytes) != 0 || len(f.truth) != 0 || len(f.blobs) != 0 || len(f.auth) != 0 {
		f.t.Fatalf("unconsumed wall facts: git=%d bytes=%d truth=%d blobs=%d auth=%d", len(f.git), len(f.bytes), len(f.truth), len(f.blobs), len(f.auth))
	}
}

func (f *strictWallReads) Git(root string, args ...string) (string, string, int) {
	f.t.Helper()
	if len(f.git) == 0 {
		f.t.Fatalf("undeclared Git call: %q %q", root, args)
	}
	want := f.git[0]
	f.git = f.git[1:]
	if root != want.root || !reflect.DeepEqual(args, want.args) {
		f.t.Fatalf("Git call got %q %q, want %q %q", root, args, want.root, want.args)
	}
	return want.stdout, want.stderr, want.code
}

func (f *strictWallReads) BlobOID(root string, content []byte) (string, error) {
	f.t.Helper()
	if len(f.bytes) == 0 {
		f.t.Fatalf("undeclared BlobOID call at %q", root)
	}
	want := f.bytes[0]
	f.bytes = f.bytes[1:]
	if root != want.root || !reflect.DeepEqual(content, want.content) {
		f.t.Fatalf("BlobOID got root %q and bytes %q, want %q and %q", root, content, want.root, want.content)
	}
	return want.oid, want.err
}

func (f *strictWallReads) nextLedger(replies *[]wallLedgerReply, method, root string, state map[string]any, path string) wallLedgerReply {
	f.t.Helper()
	if len(*replies) == 0 {
		f.t.Fatalf("undeclared %s call: %q %q", method, root, path)
	}
	want := (*replies)[0]
	*replies = (*replies)[1:]
	if root != want.root || path != want.path || reflect.ValueOf(state).Pointer() != reflect.ValueOf(want.state).Pointer() || !reflect.DeepEqual(state, want.content) {
		f.t.Fatalf("%s got root %q path %q state %#v, want root %q path %q state %#v", method, root, path, state, want.root, want.path, want.content)
	}
	return want
}

func (f *strictWallReads) LedgerTruth(root string, state map[string]any, path string) (string, string, error) {
	want := f.nextLedger(&f.truth, "LedgerTruth", root, state, path)
	return want.anchored, want.current, want.err
}

func (f *strictWallReads) LedgerBlobOID(root string, state map[string]any, path string) (string, error) {
	want := f.nextLedger(&f.blobs, "LedgerBlobOID", root, state, path)
	return want.oid, want.err
}

func (f *strictWallReads) AuthenticateLedger(root string, state map[string]any, path string) error {
	return f.nextLedger(&f.auth, "AuthenticateLedger", root, state, path).err
}

func TestWallReadsConsumersUseEngineFactsWithoutGit(t *testing.T) {
	t.Parallel()
	const root = "/virtual/wall-read-consumer"
	pinArgs := []string{"config", "--local", "--type=bool", "--get", "core.fileMode"}
	for _, tc := range []struct {
		name, stdout string
		code         int
		wantErr      string
	}{
		{"normalized true", "  true\n", 0, ""},
		{"un-pinned", "false\n", 0, "wall preflight refused: core.fileMode is not pinned true in this repository; run `git config core.fileMode true` (mode-bit drift must be visible to the tree equation)"},
		{"could not run", "", -1, "wall preflight refused: core.fileMode is not pinned true in this repository; run `git config core.fileMode true` (mode-bit drift must be visible to the tree equation)"},
	} {
		t.Run("pin "+tc.name, func(t *testing.T) {
			facts := &strictWallReads{t: t, git: []wallGitReply{{root: root, args: pinArgs, stdout: tc.stdout, code: tc.code}}}
			e := &Engine{Root: root, wallReadFacts: facts}
			err := e.checkFileModePinned()
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr || exitFor(err) != 3) {
				t.Fatalf("pin error = %v, want %q", err, tc.wantErr)
			}
			facts.done()
		})
	}
	walk := []string{"rev-list", "--first-parent", "head", "--not", "origin"}
	parent := []string{"rev-parse", "--verify", "--quiet", "old^1"}
	for _, tc := range []struct {
		name      string
		replies   []wallGitReply
		chain     []string
		violation string
	}{
		{"ordered chain", []wallGitReply{{root: root, args: walk, stdout: "head\nold\n"}, {root: root, args: parent, stdout: "origin\n"}}, []string{"old", "head"}, ""},
		{"walk could not run", []wallGitReply{{root: root, args: walk, stderr: "spawn failed", code: -1}}, nil, couldNotRunSentinel + "spawn failed"},
		{"walk answered refusal", []wallGitReply{{root: root, args: walk, stderr: "bad range", code: 1}}, nil, "committed HEAD retreated or rewrote history (open origin, now head): bad range"},
		{"parent could not run", []wallGitReply{{root: root, args: walk, stdout: "old\n"}, {root: root, args: parent, stderr: "timeout", code: -1}}, nil, couldNotRunSentinel + "timeout"},
		{"parent answered mismatch", []wallGitReply{{root: root, args: walk, stdout: "old\n"}, {root: root, args: parent, stdout: "other\n"}}, nil, "committed HEAD retreated or rewrote history (open origin, now head)"},
	} {
		t.Run("first parent "+tc.name, func(t *testing.T) {
			facts := &strictWallReads{t: t, git: tc.replies}
			got, violation := (&Engine{Root: root, wallReadFacts: facts}).firstParentSegment("origin", "head")
			if !reflect.DeepEqual(got, tc.chain) || violation != tc.violation {
				t.Fatalf("chain %v, violation %q; want %v, %q", got, violation, tc.chain, tc.violation)
			}
			facts.done()
		})
	}
	parentsArgs := []string{"rev-list", "--parents", "-n", "1", "commit"}
	for _, tc := range []struct {
		name    string
		reply   wallGitReply
		parents []string
		errType string
		errText string
	}{
		{"ordered parents", wallGitReply{stdout: "commit p1 p2\n"}, []string{"p1", "p2"}, "", ""},
		{"could not run", wallGitReply{stderr: "spawn failed", code: -1}, nil, "runner", "wall inspection cannot read the parents of commit: spawn failed"},
		{"answered unreadable", wallGitReply{stderr: "missing object", code: 1}, nil, "state", "commit commit parents are unreadable: missing object"},
		{"malformed answer", wallGitReply{stdout: "other p1\n"}, nil, "runner", "wall inspection cannot read the parents of commit"},
	} {
		t.Run("commit parents "+tc.name, func(t *testing.T) {
			answer := tc.reply
			answer.root, answer.args = root, parentsArgs
			facts := &strictWallReads{t: t, git: []wallGitReply{answer}}
			got, err := (&Engine{Root: root, wallReadFacts: facts}).commitParents("commit")
			if !reflect.DeepEqual(got, tc.parents) {
				t.Fatalf("parents = %v, want %v", got, tc.parents)
			}
			if tc.errType == "" && err != nil || tc.errType != "" && (err == nil || err.Error() != tc.errText) {
				t.Fatalf("parent error = %v, want %q", err, tc.errText)
			}
			if tc.errType == "runner" && exitFor(err) != 3 {
				t.Fatalf("runner error code = %d", exitFor(err))
			}
			if tc.errType == "state" {
				if _, ok := err.(*wallStateAnswer); !ok {
					t.Fatalf("answered history error has type %T", err)
				}
			}
			facts.done()
		})
	}
}

// A Prefix-only workspace is enough for the accountant constructor. Any
// other workspace query reaches the nil embedded interface and fails the test.
type prefixOnlyWallWorkspace struct {
	wallWorkspace
	prefix string
	calls  int
}

func (w *prefixOnlyWallWorkspace) Prefix() (string, error) {
	w.calls++
	return w.prefix, nil
}

func TestWallReadsLedgerGuardPreservesTypedErrorsWithoutGit(t *testing.T) {
	t.Parallel()
	const root = "/virtual/wall-ledger"
	path := filepath.Join(missionDirPath(root, "demo"), "ledger.md")
	for _, tc := range []struct {
		name, anchored, current string
		readErr                 error
		wantViolation, wantErr  string
	}{
		{"equal", "same", "same", nil, "", ""},
		{"changed", "before", "after", nil, "mission ledger bytes were modified during the turn", ""},
		{"no anchor", "", "", mission.ErrNoAnchor, "", ""},
		{"could not run", "", "", &gittree.RunFailure{Op: "show", Err: errors.New("spawn failed")}, "", "mission ledger guard could not run: git show could not run: spawn failed"},
		{"anchor disagreement", "", "", errors.New("anchor mismatch"), "mission ledger disagrees with the anchored truth: anchor mismatch", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := map[string]any{"integrity": "known", "cycle": 7}
			facts := &strictWallReads{t: t, truth: []wallLedgerReply{{root: root, path: path, state: state, content: map[string]any{"integrity": "known", "cycle": 7}, anchored: tc.anchored, current: tc.current, err: tc.readErr}}}
			got, err := (&Engine{Root: root, Mission: "demo", wallReadFacts: facts}).guardLedgerInTurn(state, path)
			if got != tc.wantViolation || tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr || exitFor(err) != 3) {
				t.Fatalf("guard = (%q, %v), want (%q, %q)", got, err, tc.wantViolation, tc.wantErr)
			}
			facts.done()
		})
	}
	state := map[string]any{"integrity": map[string]any{"hash": "signed"}}
	content := map[string]any{"integrity": map[string]any{"hash": "signed"}}
	facts := &strictWallReads{t: t,
		blobs: []wallLedgerReply{{root: root, path: path, state: state, content: content, oid: "ledger-blob"}},
		auth:  []wallLedgerReply{{root: root, path: path, state: state, content: content}},
	}
	workspace := &prefixOnlyWallWorkspace{prefix: "nested/"}
	e := &Engine{Root: root, Mission: "demo", wallReadFacts: facts}
	e.wallWorkspaceFactory = func(got string) wallWorkspace {
		if got != root {
			t.Fatalf("workspace root = %q, want %q", got, root)
		}
		return workspace
	}
	acct, err := e.newWallAccountant("pre", state, nil, map[string]bool{})
	if err != nil || acct.anchoredLedger != "ledger-blob" || workspace.calls != 1 {
		t.Fatalf("accountant = %+v, err %v, prefix calls %d", acct, err, workspace.calls)
	}
	ref := mission.MissionRefNamespace("demo") + "state-anchors"
	violation, err := e.judgeMissionNamespace(map[string]string{ref: "anchor"}, "", state)
	if violation != "" || err != nil {
		t.Fatalf("namespace = (%q, %v)", violation, err)
	}
	facts.done()
	for _, tc := range []struct {
		name                 string
		authErr              error
		violation, runnerErr string
	}{
		{"could not run", &gittree.RunFailure{Op: "show", Err: errors.New("spawn failed")}, "", "mission namespace authentication could not run: git show could not run: spawn failed"},
		{"anchor disagreement", errors.New("anchor mismatch"), "runner ref " + ref + " does not authenticate against the anchored truth: anchor mismatch", ""},
	} {
		t.Run("namespace "+tc.name, func(t *testing.T) {
			reads := &strictWallReads{t: t, auth: []wallLedgerReply{{root: root, path: path, state: state, content: content, err: tc.authErr}}}
			answer, err := (&Engine{Root: root, Mission: "demo", wallReadFacts: reads}).judgeMissionNamespace(map[string]string{ref: "anchor"}, "", state)
			if answer != tc.violation || tc.runnerErr == "" && err != nil || tc.runnerErr != "" && (err == nil || err.Error() != tc.runnerErr || exitFor(err) != 3) {
				t.Fatalf("namespace = (%q, %v), want (%q, %q)", answer, err, tc.violation, tc.runnerErr)
			}
			reads.done()
		})
	}
}

type baselineQuery struct {
	method, tree string
	paths        []string
	result       string
	entries      map[string]gittree.Entry
}

type strictBaselineWorkspace struct {
	wallWorkspace
	t       *testing.T
	queries []baselineQuery
}

func (w *strictBaselineWorkspace) next(method, tree string, paths []string) baselineQuery {
	w.t.Helper()
	if len(w.queries) == 0 {
		w.t.Fatalf("undeclared workspace call: %s %q %q", method, tree, paths)
	}
	want := w.queries[0]
	w.queries = w.queries[1:]
	if method != want.method || tree != want.tree || !reflect.DeepEqual(paths, want.paths) {
		w.t.Fatalf("workspace call %s %q %q, want %s %q %q", method, tree, paths, want.method, want.tree, want.paths)
	}
	return want
}

func (w *strictBaselineWorkspace) Snapshot(tree string) (string, error) {
	return w.next("Snapshot", tree, nil).result, nil
}
func (w *strictBaselineWorkspace) FilterTree(tree string, paths []string) (string, error) {
	return w.next("FilterTree", tree, paths).result, nil
}
func (w *strictBaselineWorkspace) HeadTree() (string, error) {
	return w.next("HeadTree", "", nil).result, nil
}
func (w *strictBaselineWorkspace) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	return w.next("Entries", tree, paths).entries, nil
}
func (w *strictBaselineWorkspace) RefMap() (map[string]string, error) {
	w.next("RefMap", "", nil)
	return map[string]string{}, nil
}
func (w *strictBaselineWorkspace) StagedTree() (string, error) {
	return w.next("StagedTree", "", nil).result, nil
}

func TestWallReadsAdmittedBaselineUsesApprovedBytesAndIsolatesEngines(t *testing.T) {
	t.Parallel()
	const missionID = "demo"
	contract := filepath.Join("plans", "mission-"+missionID+".contract.md")
	ledger := missionLedgerRel(missionID)
	for _, tc := range []struct {
		id, liveOID, approvedOID, wantTree, wantErr string
	}{
		{"alpha", "oid-alpha", "oid-alpha", "record-alpha", ""},
		{"beta", "oid-unapproved", "oid-beta", "", "wall preflight refused: the workspace contract does not match the approved contract bytes; the mission must run exactly the contract that was pinned"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			root := "/virtual/baseline-" + tc.id
			approved := []byte("approved contract for " + tc.id + "\n")
			entry := map[string]gittree.Entry{contract: {Mode: "100644", OID: tc.liveOID}}
			ws := &strictBaselineWorkspace{t: t, queries: []baselineQuery{
				{method: "Snapshot", tree: "HEAD", result: "raw-" + tc.id},
				{method: "FilterTree", tree: "raw-" + tc.id, paths: []string{ledger, contract}, result: "clean-" + tc.id},
				{method: "HeadTree", result: "head-" + tc.id},
				{method: "FilterTree", tree: "head-" + tc.id, paths: []string{ledger, contract}, result: "clean-" + tc.id},
				{method: "Entries", tree: "raw-" + tc.id, paths: []string{contract}, entries: entry},
				{method: "Entries", tree: "head-" + tc.id, paths: []string{contract}, entries: entry},
			}}
			if tc.wantErr == "" {
				ws.queries = append(ws.queries,
					baselineQuery{method: "RefMap"},
					baselineQuery{method: "FilterTree", tree: "raw-" + tc.id, paths: []string{ledger}, result: "record-" + tc.id},
					baselineQuery{method: "StagedTree", result: "staged-" + tc.id},
					baselineQuery{method: "FilterTree", tree: "staged-" + tc.id, paths: []string{ledger}, result: "identity-" + tc.id},
					baselineQuery{method: "FilterTree", tree: "head-" + tc.id, paths: []string{ledger}, result: "identity-" + tc.id},
				)
			}
			facts := &strictWallReads{t: t,
				git:   []wallGitReply{{root: root, args: []string{"config", "--local", "--type=bool", "--get", "core.fileMode"}, stdout: "true\n"}},
				bytes: []wallByteReply{{root: root, content: approved, oid: tc.approvedOID}},
			}
			e := &Engine{Root: root, Mission: missionID, wallReadFacts: facts}
			e.wallWorkspaceFactory = func(got string) wallWorkspace {
				if got != root {
					t.Fatalf("workspace requested root %q, want %q", got, root)
				}
				return ws
			}
			got, err := e.admittedBaseline(nil, approved)
			if got != tc.wantTree || tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr || exitFor(err) != 3) {
				t.Fatalf("baseline = (%q, %v), want (%q, %q)", got, err, tc.wantTree, tc.wantErr)
			}
			facts.done()
			if len(ws.queries) != 0 {
				t.Fatalf("unconsumed workspace queries: %+v", ws.queries)
			}
		})
	}
}
