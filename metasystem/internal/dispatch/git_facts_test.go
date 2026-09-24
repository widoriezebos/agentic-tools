package dispatch

import (
	"fmt"
	"testing"
)

// strictGitFacts supplies only the repository facts a test declares. A wrong
// argument or an extra read fails the test instead of finding host Git.
type strictGitFacts struct {
	t        *testing.T
	expected []gitFactRead
	next     int
}

type gitFactRead struct {
	kind, workspace, commit, path, value string
	err                                  error
}

func declaredGitFacts(t *testing.T, reads ...gitFactRead) *strictGitFacts {
	t.Helper()
	f := &strictGitFacts{t: t, expected: reads}
	t.Cleanup(func() {
		if f.next != len(f.expected) {
			t.Errorf("repository facts: consumed %d of %d declared reads", f.next, len(f.expected))
		}
	})
	return f
}

func (f *strictGitFacts) read(kind, workspace, commit, path string) (string, error) {
	f.t.Helper()
	if f.next >= len(f.expected) {
		f.t.Errorf("undeclared repository read: %s(%q, %q, %q)", kind, workspace, commit, path)
		return "", fmt.Errorf("undeclared repository read")
	}
	want := f.expected[f.next]
	f.next++
	if want.kind != kind || want.workspace != workspace || want.commit != commit || want.path != path {
		f.t.Errorf("repository read %d: got %s(%q, %q, %q), want %s(%q, %q, %q)", f.next, kind, workspace, commit, path, want.kind, want.workspace, want.commit, want.path)
		return "", fmt.Errorf("unexpected repository read")
	}
	return want.value, want.err
}

func (f *strictGitFacts) Head(workspace string) (string, error) {
	return f.read("head", workspace, "", "")
}
func (f *strictGitFacts) Branch(workspace string) (string, error) {
	return f.read("branch", workspace, "", "")
}
func (f *strictGitFacts) BlobAt(workspace, commit, path string) (string, error) {
	return f.read("blob", workspace, commit, path)
}
func (f *strictGitFacts) AbsoluteGitDir(worktree string) (string, error) {
	return f.read("git-dir", worktree, "", "")
}
func (f *strictGitFacts) CommonGitDir(worktree string) (string, error) {
	return f.read("common-dir", worktree, "", "")
}
func (f *strictGitFacts) CurrentBranch(worktree string) (string, error) {
	return f.read("branch", worktree, "", "")
}

const testHeadID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const testBlobID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func recordFacts(t *testing.T, workspace string, records int, designPath string) *strictGitFacts {
	t.Helper()
	var reads []gitFactRead
	for i := 0; i < records; i++ {
		reads = append(reads,
			gitFactRead{kind: "head", workspace: workspace, value: testHeadID},
			gitFactRead{kind: "branch", workspace: workspace, value: "agent/record"})
		if designPath != "" {
			reads = append(reads, gitFactRead{kind: "blob", workspace: workspace, commit: testHeadID, path: designPath, value: testBlobID})
		}
	}
	return declaredGitFacts(t, reads...)
}

func worktreeFacts(t *testing.T, worktree, gitDir, commonDir, branch string) *strictGitFacts {
	t.Helper()
	return declaredGitFacts(t,
		gitFactRead{kind: "git-dir", workspace: worktree, value: gitDir},
		gitFactRead{kind: "common-dir", workspace: worktree, value: commonDir},
		gitFactRead{kind: "branch", workspace: worktree, value: branch})
}

var _ buildWorkspaceFacts = (*strictGitFacts)(nil)
var _ worktreeMetadata = (*strictGitFacts)(nil)
