package gopackages

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// selectionFaultWorkspace answers every repository question from fields so
// one case can make exactly one fact unreadable. Revisions never carry go.mod,
// so both sides resolve through TreeOf.
type selectionFaultWorkspace struct {
	treeErr    map[string]error
	changed    []string
	changedErr error
	entriesErr map[string]error
	goMod      []byte
	goModErr   error
	snapshot   map[string]string
	openErr    map[string]error
	closeErr   error
	opened     []string
	closed     []string
}

func (w *selectionFaultWorkspace) TreeOf(revision string) (string, error) {
	if err := w.treeErr[revision]; err != nil {
		return "", err
	}
	if revision == selectionBaseRevision {
		return selectionBaseTree, nil
	}
	return selectionCandidateTree, nil
}

func (w *selectionFaultWorkspace) FileAt(tree, path string) ([]byte, bool, error) {
	if tree != selectionCandidateTree || path != "go.mod" {
		return nil, false, nil
	}
	return slices.Clone(w.goMod), w.goMod != nil, w.goModErr
}

func (w *selectionFaultWorkspace) ChangedPaths(string, string) ([]string, error) {
	return w.changed, w.changedErr
}

func (w *selectionFaultWorkspace) Entries(tree string, paths []string) (map[string]gittree.Entry, error) {
	if err := w.entriesErr[tree]; err != nil {
		return nil, err
	}
	entries := map[string]gittree.Entry{}
	for _, path := range paths {
		entries[path] = gittree.Entry{Mode: "100644"}
	}
	return entries, nil
}

func (w *selectionFaultWorkspace) openSnapshot(tree string) (string, func() error, error) {
	if err := w.openErr[tree]; err != nil {
		return "", nil, err
	}
	w.opened = append(w.opened, tree)
	return w.snapshot[tree], func() error {
		w.closed = append(w.closed, tree)
		return w.closeErr
	}, nil
}

func TestSelectionRefusesUnreadableRepositoryFacts(t *testing.T) {
	t.Parallel()
	unreadable := errors.New("fixture fact unreadable")
	module := []byte("module example.invalid/fault\n")
	cases := []struct {
		name       string
		workspace  selectionFaultWorkspace
		wantErr    string
		wantOpened []string
	}{
		{name: "base tree", workspace: selectionFaultWorkspace{treeErr: map[string]error{selectionBaseRevision: unreadable}}, wantErr: "go package base: fixture fact unreadable"},
		{name: "candidate tree", workspace: selectionFaultWorkspace{treeErr: map[string]error{selectionCandidateRevision: unreadable}}, wantErr: "go package candidate: fixture fact unreadable"},
		{name: "changed paths", workspace: selectionFaultWorkspace{changedErr: unreadable}, wantErr: "fixture fact unreadable"},
		{name: "base entries", workspace: selectionFaultWorkspace{changed: []string{"a/a.go"}, entriesErr: map[string]error{selectionBaseTree: unreadable}}, wantErr: "fixture fact unreadable"},
		{name: "candidate entries", workspace: selectionFaultWorkspace{changed: []string{"a/a.go"}, entriesErr: map[string]error{selectionCandidateTree: unreadable}}, wantErr: "fixture fact unreadable"},
		{name: "candidate manifest read", workspace: selectionFaultWorkspace{changed: []string{"a/a.go"}, goModErr: unreadable}, wantErr: "fixture fact unreadable"},
		{name: "candidate manifest absent", workspace: selectionFaultWorkspace{changed: []string{"a/a.go"}}, wantErr: "go package candidate has no go.mod"},
		{name: "module directive absent", workspace: selectionFaultWorkspace{changed: []string{"a/a.go"}, goMod: []byte("go 1.27\n")}, wantErr: "go.mod has no module directive"},
		{name: "module directive malformed", workspace: selectionFaultWorkspace{changed: []string{"a/a.go"}, goMod: []byte("module \"example.invalid/open\n")}, wantErr: "invalid syntax"},
		{name: "candidate snapshot", workspace: selectionFaultWorkspace{changed: []string{"a/a.go"}, goMod: module, openErr: map[string]error{selectionCandidateTree: unreadable}}, wantErr: "materialize Go package candidate: fixture fact unreadable"},
		{name: "base snapshot", workspace: selectionFaultWorkspace{changed: []string{"a/a.go"}, goMod: module, openErr: map[string]error{selectionBaseTree: unreadable}}, wantErr: "materialize Go package base: fixture fact unreadable", wantOpened: []string{selectionCandidateTree}},
		{name: "candidate close", workspace: selectionFaultWorkspace{changed: []string{"go.mod"}, goMod: module, closeErr: unreadable}, wantErr: "fixture fact unreadable", wantOpened: []string{selectionCandidateTree}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			workspace := tc.workspace
			workspace.snapshot = map[string]string{selectionBaseTree: t.TempDir(), selectionCandidateTree: t.TempDir()}
			owner := selector{workspace: &workspace, openSnapshot: workspace.openSnapshot}
			_, err := owner.selectPackages(selectionBaseRevision, selectionCandidateRevision, nil, nil)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("selection error = %v, want %q", err, tc.wantErr)
			}
			if !slices.Equal(workspace.opened, tc.wantOpened) || !slices.Equal(workspace.closed, tc.wantOpened) {
				t.Fatalf("snapshots opened=%v closed=%v, want each %v", workspace.opened, workspace.closed, tc.wantOpened)
			}
		})
	}
}

func TestSelectionWithoutChangedPathsOpensNoSnapshot(t *testing.T) {
	t.Parallel()
	workspace := selectionFaultWorkspace{}
	owner := selector{workspace: &workspace, openSnapshot: workspace.openSnapshot}
	selected, err := owner.selectPackages(selectionBaseRevision, selectionCandidateRevision, nil, nil)
	if err != nil || selected.Tree != selectionCandidateTree || selected.Packages != nil || selected.Changed != nil || len(workspace.opened) != 0 {
		t.Fatalf("unchanged selection = %+v err=%v opened=%v", selected, err, workspace.opened)
	}
}

func TestSelectionAcceptsQuotedModuleDirective(t *testing.T) {
	t.Parallel()
	candidate := t.TempDir()
	writeSelectionFile(t, candidate, "go.mod", "module \"example.invalid/quoted\"\n")
	writeSelectionFile(t, candidate, "a/a.go", "package a\n")
	workspace := selectionFaultWorkspace{changed: []string{"a/a.go"}, goMod: []byte("module \"example.invalid/quoted\"\n"),
		snapshot: map[string]string{selectionCandidateTree: candidate}}
	owner := selector{workspace: &workspace, openSnapshot: workspace.openSnapshot}
	selected, err := owner.selectPackages(selectionBaseRevision, selectionCandidateRevision, nil, nil)
	if err != nil || selected.ModulePath != "example.invalid/quoted" || !slices.Equal(selected.Packages, []string{"./a"}) {
		t.Fatalf("quoted module selection = %+v err=%v", selected, err)
	}
}

func TestInventoryRefusesUnresolvableGoEnvironment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ entry, wantErr string }{
		{"CGO_ENABLED=maybe", `cannot resolve CGO_ENABLED="maybe"`},
		{"GOEXPERIMENT=arenas", `cannot resolve GOEXPERIMENT="arenas"`},
		{"GOFLAGS=-tags='feature'", "cannot resolve quoted GOFLAGS tags"},
		{"GOFLAGS=-modfile=alt.mod", "cannot resolve GOFLAGS -modfile=alt.mod"},
		{"GOFLAGS=-tags", "cannot resolve GOFLAGS tags"},
		{"GOFLAGS=-tags=feature,bad-tag", `cannot resolve GOFLAGS tag "bad-tag"`},
	} {
		t.Run(tc.entry, func(t *testing.T) {
			t.Parallel()
			_, err := runnablePackageInventory(t.TempDir(), nil, []string{tc.entry})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("inventory error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
