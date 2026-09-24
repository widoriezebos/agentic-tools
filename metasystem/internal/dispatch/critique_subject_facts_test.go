package dispatch

import (
	"fmt"
	"testing"
)

type critiqueFactCall struct {
	kind, root, commit, tree, path string
	paths                          []string
	value                          string
	absent                         bool
	err                            error
}

type strictCritiqueFacts struct {
	t     *testing.T
	calls []critiqueFactCall
	next  int
}

func declaredPrefix(root, prefix string) critiqueFactCall {
	return critiqueFactCall{kind: "install prefix", root: root, value: prefix}
}

func declaredPaths(root, commit string, paths ...string) critiqueFactCall {
	return critiqueFactCall{kind: "changed paths", root: root, commit: commit, paths: paths}
}

func declaredTree(root, commit, tree string) critiqueFactCall {
	return critiqueFactCall{kind: "commit tree", root: root, commit: commit, value: tree}
}

func declaredAbsence(root, tree, path string, absent bool) critiqueFactCall {
	return critiqueFactCall{kind: "artifact absent", root: root, tree: tree, path: path, absent: absent}
}

func newStrictCritiqueFacts(t *testing.T, calls ...critiqueFactCall) *strictCritiqueFacts {
	t.Helper()
	return &strictCritiqueFacts{t: t, calls: calls}
}

func (f *strictCritiqueFacts) take(kind, root, commit, tree, path string) critiqueFactCall {
	f.t.Helper()
	if f.next >= len(f.calls) {
		f.t.Fatalf("unexpected repository subject request %s root=%q commit=%q tree=%q path=%q", kind, root, commit, tree, path)
	}
	call := f.calls[f.next]
	f.next++
	if call.kind != kind || call.root != root || call.commit != commit || call.tree != tree || call.path != path {
		f.t.Fatalf("repository subject request %d = %s root=%q commit=%q tree=%q path=%q; want %+v", f.next, kind, root, commit, tree, path, call)
	}
	return call
}

func (f *strictCritiqueFacts) ChangedPaths(root, commit string) ([]string, error) {
	call := f.take("changed paths", root, commit, "", "")
	return call.paths, call.err
}

func (f *strictCritiqueFacts) CommitTree(root, commit string) (string, error) {
	call := f.take("commit tree", root, commit, "", "")
	return call.value, call.err
}

func (f *strictCritiqueFacts) InstallPrefix(root string) (string, error) {
	call := f.take("install prefix", root, "", "", "")
	return call.value, call.err
}

func (f *strictCritiqueFacts) ArtifactAbsent(root, tree, path string) bool {
	return f.take("artifact absent", root, "", tree, path).absent
}

func (f *strictCritiqueFacts) assertConsumed() {
	f.t.Helper()
	if f.next != len(f.calls) {
		f.t.Fatalf("repository subject facts consumed %d of %d calls; next = %s", f.next, len(f.calls), fmt.Sprint(f.calls[f.next]))
	}
}

func advanceWithFacts(t *testing.T, repo, root, round string, calls ...critiqueFactCall) (string, error) {
	t.Helper()
	facts := newStrictCritiqueFacts(t, calls...)
	outcome, err := critiqueRegisterAdvance(repo, root, round, facts)
	facts.assertConsumed()
	return outcome, err
}

func advanceWithPrefix(t *testing.T, repo, root, round string) (string, error) {
	t.Helper()
	return advanceWithFacts(t, repo, root, round, declaredPrefix(repo, ""))
}
