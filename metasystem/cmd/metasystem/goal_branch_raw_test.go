package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

type branchRawFixture struct {
	t                                             *testing.T
	project, installation, base, unit, tree, read string
	unitPath, record, tip, trailer                string
	ledger                                        *obligationRepository
	generated                                     map[string][]byte
	responses                                     map[string][]byte
	used                                          map[string]int
	staged, published                             bool
}

func branchRawID(c string) string { return strings.Repeat(c, 40) }
func branchRawEntry(path string) []byte {
	return []byte(":100644 100644 " + branchRawID("1") + " " + branchRawID("2") + " M\x00" + path + "\x00")
}
func branchRawKey(args ...string) string { return strings.Join(args, "\x00") }

func newBranchRawFixture(t *testing.T, nested, reader bool) *branchRawFixture {
	return newBranchRawFixtureWithReadReplies(t, nested, reader, true)
}

func newBranchRawFixtureWithReadReplies(t *testing.T, nested, reader, readReplies bool) *branchRawFixture {
	t.Helper()
	deny, err := filepath.Abs(filepath.Join("..", "..", "internal", "testgit", "testdata", "deny-bin"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", deny+string(os.PathListSeparator)+os.Getenv("PATH"))
	deniedLog := filepath.Join(t.TempDir(), "git-denied.log")
	t.Setenv("METASYSTEM_TEST_GIT_DENIED_LOG", deniedLog)
	t.Cleanup(func() {
		data, err := os.ReadFile(deniedLog)
		if err != nil && !os.IsNotExist(err) {
			t.Errorf("read denied Git log: %v", err)
		} else if len(data) != 0 {
			t.Errorf("denied Git calls:\n%s", data)
		}
	})
	source := newObligationCommandFixture(t)
	project, installation := source.root(), source.root()
	if nested {
		installation = filepath.Join(project, "metasystem")
	}
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(project, "plans", "goals", "backlog.md"))
	if err != nil {
		t.Fatal(err)
	}
	root, problems := goal.ParseRoot(data)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	root.SyncMode = goal.SyncRemote
	rendered := goal.RenderRoot(root)
	if err := os.WriteFile(filepath.Join(project, "plans", "goals", "backlog.md"), rendered, 0644); err != nil {
		t.Fatal(err)
	}
	for id, commit := range source.repo.commits {
		commit.files["plans/goals/backlog.md"] = rendered
		source.repo.commits[id] = commit
	}
	f := &branchRawFixture{t: t, project: project, installation: installation, ledger: source.repo, base: branchRawID("a"), unit: branchRawID("b"), tree: branchRawID("c"), read: branchRawID("d"), tip: branchRawID("b"), responses: map[string][]byte{}, used: map[string]int{}}
	f.unitPath = "metasystem/code.go"
	if reader {
		f.unitPath = "metasystem/read.go"
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(project, f.unitPath)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, f.unitPath), []byte("package fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("identity %s %v", state, err)
	}
	if _, err = lease.AnnounceWithPair(installation, "goal-branch-raw", int64(os.Getpid()), exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "fixture", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	if readReplies {
		f.add(f.base+"\n", "merge-base", f.base, f.unit)
		f.add(f.unit+" "+f.base+"\n", "rev-list", "--first-parent", "--reverse", "--parents", f.base+".."+f.unit)
		f.add("Goal-Unit: standing-validation/u1\n", "show", "-s", "--format=%(trailers:only,unfold=true)", f.unit)
		f.add(f.unit+" "+f.base+"\n", "rev-list", "--parents", "-n", "1", f.unit)
		f.responses[branchRawKey("diff-tree", "-r", "-z", "--no-renames", "--full-index", f.unit+"^", f.unit)] = branchRawEntry(f.unitPath)
	}
	t.Cleanup(func() {
		for key := range f.responses {
			if f.used[key] == 0 {
				t.Errorf("unconsumed raw fact %q", strings.Split(key, "\x00"))
			}
		}
	})
	return f
}
func (f *branchRawFixture) add(value string, args ...string) {
	f.responses[branchRawKey(args...)] = []byte(value)
}
func (f *branchRawFixture) raw(repo string, args ...string) ([]byte, error) {
	f.t.Helper()
	if repo != f.installation && !strings.HasPrefix(repo, f.project+string(os.PathSeparator)) {
		f.t.Fatalf("raw repo %s", repo)
	}
	key := branchRawKey(args...)
	value, ok := f.responses[key]
	if !ok {
		f.t.Fatalf("unexpected raw read %q", args)
	}
	f.used[key]++
	return bytes.Clone(value), nil
}
func (f *branchRawFixture) readTranscript() {
	f.add(f.base+"\n", "merge-base", f.base, f.read)
	f.add(f.unit+" "+f.base+"\n"+f.read+" "+f.unit+"\n", "rev-list", "--first-parent", "--reverse", "--parents", f.base+".."+f.read)
	f.add(f.trailer+"\n", "show", "-s", "--format=%(trailers:only,unfold=true)", f.read)
	f.add(f.read+" "+f.unit+"\n", "rev-list", "--parents", "-n", "1", f.read)
	paths := make([]string, 0, len(f.generated)+1)
	for path := range f.generated {
		paths = append(paths, path)
	}
	if f.record != "" {
		paths = append(paths, f.record)
	}
	sort.Strings(paths)
	var entries []byte
	for _, path := range paths {
		entries = append(entries, branchRawEntry(path)...)
	}
	f.responses[branchRawKey("diff-tree", "-r", "-z", "--no-renames", "--full-index", f.read+"^", f.read)] = entries
}
func (f *branchRawFixture) Range(r, e, tip, g string) ([]branch.Commit, error) {
	return branch.ValidateRangeWithGit(r, e, tip, g, f.raw)
}
func (f *branchRawFixture) Subject(r, c string) (branch.AttestationSubject, error) {
	return branch.ComputeSubjectWithReads(f, r, c)
}
func (f *branchRawFixture) CommonDir(repo string) (string, error) {
	if repo != f.installation {
		f.t.Fatalf("common dir repo %s", repo)
	}
	return filepath.Join(f.project, ".git"), nil
}
func (f *branchRawFixture) Entries(r, c string) ([]branch.Entry, error) {
	return branch.RawEntriesWithRaw(r, c, f.raw)
}
func (f *branchRawFixture) Detached(repo, commit string) (string, func() error, error) {
	if repo != f.installation || commit != f.unit {
		f.t.Fatalf("detached %s %s", repo, commit)
	}
	dir, err := os.MkdirTemp(f.project, "gate-")
	return dir, func() error { return os.RemoveAll(dir) }, err
}
func (f *branchRawFixture) ReadSubject(r, c string) (readsubject.ReadSubject, error) {
	value, present, err := dispatchcore.ComputeReadSubjectWithFacts(dispatchcore.ReadSubjectRequest{RepoRoot: r, Role: "code-critic", Reviews: "commit:" + c}, f)
	if !present && err == nil {
		err = fmt.Errorf("missing subject")
	}
	return value, err
}
func (f *branchRawFixture) RawEntries(r, c string) ([]byte, error) {
	return f.raw(r, "diff-tree", "-r", "-z", "--no-renames", "--full-index", c+"^", c)
}
func (f *branchRawFixture) SnapshotFile(string, string, string) ([]byte, error) {
	f.t.Fatal("unexpected snapshot file")
	return nil, nil
}
func (f *branchRawFixture) TopLevel(repo string) (string, error) {
	if repo != f.installation {
		f.t.Fatalf("top level repo %s", repo)
	}
	return f.project, nil
}
func (f *branchRawFixture) CommitExists(_, c string) error {
	if c != f.unit {
		f.t.Fatal(c)
	}
	return nil
}
func (f *branchRawFixture) Kind(r, c, g string) (branch.KindInfo, error) {
	return branch.KindOfWithRaw(r, c, g, f.raw)
}
func (f *branchRawFixture) Prefix(string) (string, error) {
	if f.installation != f.project {
		return "metasystem/", nil
	}
	return "", nil
}
func (f *branchRawFixture) Transition(string, string, string) ([]byte, error) {
	f.t.Fatal("unexpected transition")
	return nil, nil
}
func (f *branchRawFixture) TreeEntry(string, string, string) (string, error) {
	f.t.Fatal("unexpected tree entry")
	return "", nil
}
func (f *branchRawFixture) CommitParent(_, c string) (string, error) {
	if c != f.unit {
		f.t.Fatal(c)
	}
	return f.base, nil
}
func (f *branchRawFixture) CommitTree(_, c string) (string, error) {
	if c != f.unit {
		f.t.Fatal(c)
	}
	return f.tree, nil
}
func (f *branchRawFixture) CommitDiff(_, parent, c string) ([]byte, error) {
	if parent != f.unit+"^" || c != f.unit {
		f.t.Fatal(parent, c)
	}
	return []byte("unit patch\n"), nil
}
func (f *branchRawFixture) WorkspaceHead(string) (string, error) {
	f.t.Fatal("unexpected workspace head")
	return "", nil
}
func (f *branchRawFixture) LiveWorkspaceTree(string, string) (string, error) {
	f.t.Fatal("unexpected workspace tree")
	return "", nil
}
func (f *branchRawFixture) readerRecord() string {
	f.record = "metasystem/records/misc/command-read.md"
	path := filepath.Join(f.project, filepath.FromSlash(f.record))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		f.t.Fatal(err)
	}
	digest, err := branch.UnitDigestWithRaw(f.installation, f.unit, f.raw)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(f.unit+" "+digest+"\n"), 0644); err != nil {
		f.t.Fatal(err)
	}
	return f.record
}

type branchRawTransport struct{ f *branchRawFixture }

func (x branchRawTransport) RemoteTip(repo, remote, ref string) (string, bool, error) {
	if repo != x.f.installation || remote != "upstream" || ref != "refs/heads/goal/standing-validation" {
		x.f.t.Fatal(repo, remote, ref)
	}
	return "", false, nil
}
func (x branchRawTransport) Fetch(string, string, string, string) error {
	x.f.t.Fatal("unexpected fetch")
	return nil
}
func (x branchRawTransport) Push(string, string, string, string, string) (branch.CASOutcome, error) {
	x.f.t.Fatal("unexpected push")
	return "", nil
}
func (f *branchRawFixture) inputs() *branch.ReadCommitInputs {
	fail := func() { f.t.Helper(); f.t.Fatal("unexpected commit callback") }
	facts := branch.CommitFacts{
		Tip: func(_, ref string) (string, bool, error) {
			if ref == "refs/heads/goal/standing-validation" {
				return f.tip, true, nil
			}
			if ref == "refs/metasystem/goals/origin/standing-validation" {
				return "", false, nil
			}
			fail()
			return "", false, nil
		},
		Head: func(repo string) (string, error) {
			if repo == f.installation {
				return f.tip, nil
			}
			if strings.HasPrefix(repo, f.project+string(os.PathSeparator)) {
				return f.read, nil
			}
			fail()
			return "", nil
		},
		Tree: func(string) (string, error) { fail(); return "", nil }, Index: func(string) (string, error) { return f.tree, nil }, HeadRef: func(string) string { return "refs/heads/goal/standing-validation" },
		Staged: func(string) ([]string, error) { return nil, nil }, Unstaged: func(string) ([]string, error) { return nil, nil }, Patch: func(string) ([]byte, error) { fail(); return nil, nil },
		Range: f.Range, Ancestor: func(string, string, string) (bool, error) { fail(); return false, nil }, Suffix: func(string, string, string) ([]string, error) { fail(); return nil, nil }, Kind: f.Kind, Entries: f.Entries,
		Changes: func(string, string, string) ([]string, error) { return nil, nil }, Worktrees: func(string) ([]branch.CommitWorktree, error) { fail(); return nil, nil },
	}
	effects := branch.CommitEffects{
		ClearFetch: func(string, string) error { fail(); return nil }, Open: func(string, string, bool) (string, func(), error) {
			dir, err := os.MkdirTemp(f.project, "build-")
			return dir, func() { os.RemoveAll(dir) }, err
		},
		Apply: func(_ string, patch []byte) error {
			if !bytes.Equal(patch, []byte("read patch\n")) {
				f.t.Fatalf("patch %q", patch)
			}
			return nil
		},
		Commit: func(_, subject, trailer string, amend bool) error {
			if subject != "goal standing-validation read u1" || trailer != "Goal-Read: standing-validation/u1 "+f.unit || amend {
				f.t.Fatalf("read commit %q %q", subject, trailer)
			}
			f.trailer = trailer
			f.readTranscript()
			return nil
		},
		Replay: func(string, string) error { fail(); return nil }, WithoutPaths: func(string, string, []string) (string, error) { fail(); return "", nil },
		Checkout: func(_, before, after string) error {
			if before != f.tree || after != f.read {
				f.t.Fatalf("checkout %s %s", before, after)
			}
			return nil
		},
		Attach: func(string, string) error { fail(); return nil }, Restore: func(string, string, string) error { fail(); return nil },
		Publish: func(_, goal, old, next, origin string) error {
			if goal != "standing-validation" || old != f.unit || next != f.read || origin != "" {
				f.t.Fatalf("publish %s %s %s %s", goal, old, next, origin)
			}
			f.tip = next
			f.published = true
			return nil
		},
	}
	return &branch.ReadCommitInputs{Reads: f, Facts: facts, Effects: effects,
		Patch: func(_ string, generated map[string][]byte, reader string) ([]byte, error) {
			f.generated = map[string][]byte{}
			for path, data := range generated {
				f.generated[path] = bytes.Clone(data)
			}
			if reader != f.record {
				f.t.Fatalf("reader %s", reader)
			}
			return []byte("read patch\n"), nil
		},
		RestoreIndex: func(string, string) error { fail(); return nil },
		Stage: func(_ string, paths []string) error {
			for _, path := range paths {
				if _, err := os.Stat(filepath.Join(f.project, path)); err != nil {
					f.t.Fatalf("stage %s: %v", path, err)
				}
			}
			f.staged = true
			return nil
		},
	}
}
func (f *branchRawFixture) dependencies() *goalBranchRawDependencies {
	return &goalBranchRawDependencies{
		Config: func(_, key string) (string, error) {
			switch key {
			case "goal.sync-remote":
				return "upstream", nil
			case "goal.sync-branch":
				return "refs/heads/main", nil
			case "metasystem.goal.machine":
				return "mac-cli", nil
			}
			f.t.Fatalf("config key %s", key)
			return "", nil
		},
		GoalRepository: f.ledger, EndpointTip: func(repo string, endpoint goal.Endpoint) (string, error) {
			if repo != f.installation || endpoint.Remote != "upstream" || endpoint.Branch != "refs/heads/main" {
				f.t.Fatalf("endpoint %s %+v", repo, endpoint)
			}
			return f.base, nil
		},
		OriginTip: func(repo string, endpoint goal.Endpoint, goalID string) (string, bool, error) {
			if repo != f.installation || endpoint.Remote != "upstream" || goalID != "standing-validation" {
				f.t.Fatalf("origin tip %s %s", repo, goalID)
			}
			return f.unit, true, nil
		}, ResolveCommit: func(_, commit string) (string, error) {
			if commit != f.unit {
				f.t.Fatal(commit)
			}
			return commit, nil
		},
		HolderRoot: func(string) string { return f.installation }, Linked: func(string) bool { return true },
		HeadRef: func(repo string) ([]byte, error) {
			if repo != f.installation {
				f.t.Fatalf("head ref repo %s", repo)
			}
			return []byte("refs/heads/main\n"), nil
		},
		ReadRepository: f, ReadInputs: f.inputs(), Transport: branchRawTransport{f},
	}
}

func (f *branchRawFixture) unchangedFiles(paths ...string) func() {
	f.t.Helper()
	before := make(map[string][]byte, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(f.project, filepath.FromSlash(path)))
		if err != nil {
			f.t.Fatal(err)
		}
		before[path] = data
	}
	return func() {
		f.t.Helper()
		for path, want := range before {
			data, err := os.ReadFile(filepath.Join(f.project, filepath.FromSlash(path)))
			if err != nil || !bytes.Equal(data, want) {
				f.t.Errorf("physical file %s changed: err=%v", path, err)
			}
		}
	}
}

func (f *branchRawFixture) assertNoPreflightEffects() {
	f.t.Helper()
	if f.ledger.canonical != strings.Repeat("0", 39)+"1" || f.ledger.accepted != strings.Repeat("0", 39)+"1" ||
		f.ledger.captures != 0 || f.ledger.builds != 0 || f.ledger.publications != 0 || f.ledger.advances != 0 || f.ledger.releases != 0 ||
		len(f.ledger.commits) != 1 || f.tip != f.unit || f.staged || f.published || f.generated != nil || len(f.responses) != 0 {
		f.t.Fatalf("early refusal changed raw state: ledger=%+v tip=%s staged=%t published=%t generated=%v replies=%d",
			f.ledger, f.tip, f.staged, f.published, f.generated, len(f.responses))
	}
}
