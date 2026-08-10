package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeProcessTree is a fixed ancestry: parents maps child to parent,
// starts maps pid to its start second.
type fakeProcessTree struct {
	parents map[int64]int64
	starts  map[int64]int64
}

func (tree fakeProcessTree) ParentPid(pid int64) (int64, bool) {
	parent, ok := tree.parents[pid]
	return parent, ok
}

func (tree fakeProcessTree) StartedAtSec(pid int64) (int64, bool) {
	start, ok := tree.starts[pid]
	return start, ok
}

func writeToken(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "worktree-commit-token.json")
	writeFile(t, path, content)
	return path
}

const validNonce = "0123456789abcdef0123456789abcdef"

func TestWrapperTokenAcceptsLiveAncestor(t *testing.T) {
	token := writeToken(t, `{"wrapperPid":50,"wrapperPidStartedAt":1000,"nonce":"`+validNonce+`"}`)
	tree := fakeProcessTree{
		parents: map[int64]int64{100: 70, 70: 50, 50: 1},
		starts:  map[int64]int64{50: 1000},
	}
	if !WrapperToken(token, 100, tree) {
		t.Fatal("a live wrapper in the caller's ancestry must prove the token")
	}
}

func TestWrapperTokenRejects(t *testing.T) {
	tree := fakeProcessTree{
		parents: map[int64]int64{100: 70, 70: 50, 50: 1},
		starts:  map[int64]int64{50: 2000},
	}
	valid := `{"wrapperPid":50,"wrapperPidStartedAt":1000,"nonce":"` + validNonce + `"}`

	// The pid at the wrapper's slot started at a different second: a
	// recycled pid, not the recorded wrapper.
	if WrapperToken(writeToken(t, valid), 100, tree) {
		t.Fatal("a start-time mismatch must fail the proof")
	}

	// The wrapper pid is not among the caller's ancestors.
	tree.starts[50] = 1000
	if WrapperToken(writeToken(t, valid), 200, fakeProcessTree{
		parents: map[int64]int64{200: 199, 199: 1},
		starts:  tree.starts,
	}) {
		t.Fatal("a wrapper outside the ancestry must fail the proof")
	}

	// Malformed tokens: short nonce, fractional pid, missing file.
	if WrapperToken(writeToken(t, strings.Replace(valid, validNonce, "short", 1)), 100, tree) {
		t.Fatal("a short nonce must fail the proof")
	}
	if WrapperToken(writeToken(t, `{"wrapperPid":50.5,"wrapperPidStartedAt":1000,"nonce":"`+validNonce+`"}`), 100, tree) {
		t.Fatal("a fractional wrapperPid must fail the proof")
	}
	if WrapperToken(filepath.Join(t.TempDir(), "absent.json"), 100, tree) {
		t.Fatal("a missing token must fail the proof")
	}
}

// The kernel-backed tree must prove a token naming a live ancestor of
// this very process: the walk starts at the caller, so the caller's own
// pid with its own kernel start time is the tightest live fixture.
func TestWrapperTokenKernelTree(t *testing.T) {
	tree := KernelProcessTree{}
	self := int64(os.Getpid())
	start, ok := tree.StartedAtSec(self)
	if !ok {
		t.Fatal("could not read our own start time from the kernel")
	}
	if parent, ok := tree.ParentPid(self); !ok || parent != int64(os.Getppid()) {
		t.Fatalf("ParentPid(self) = %d,%v, want %d", parent, ok, os.Getppid())
	}
	token := writeToken(t, fmt.Sprintf(
		`{"wrapperPid":%d,"wrapperPidStartedAt":%d,"nonce":"%s"}`, self, start, validNonce))
	if !WrapperToken(token, self, tree) {
		t.Fatal("a live caller must prove its own token against the kernel")
	}
	stale := writeToken(t, fmt.Sprintf(
		`{"wrapperPid":%d,"wrapperPidStartedAt":%d,"nonce":"%s"}`, self, start-1, validNonce))
	if WrapperToken(stale, self, tree) {
		t.Fatal("a stale start time must fail against the kernel")
	}
}

func TestWrapperTokenStopsOnAncestryCycle(t *testing.T) {
	token := writeToken(t, `{"wrapperPid":50,"wrapperPidStartedAt":1000,"nonce":"`+validNonce+`"}`)
	tree := fakeProcessTree{
		parents: map[int64]int64{100: 70, 70: 100},
		starts:  map[int64]int64{50: 1000},
	}
	if WrapperToken(token, 100, tree) {
		t.Fatal("a cyclic ancestry that never reaches the wrapper must fail the proof")
	}
}
