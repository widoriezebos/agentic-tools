package census

import "testing"

// fakeTree is a fixed process tree keyed by pid.
type fakeTree map[int64]ProcInfo

func (f fakeTree) Info(pid int64) (ProcInfo, bool) {
	info, ok := f[pid]
	return info, ok
}

func TestFindAncestorMatchesFirstAgentUp(t *testing.T) {
	sigs := testSignatures(t)
	// 500 (delegate shell) -> 400 (claude session) -> 300 (login shell)
	tree := fakeTree{
		500: {PPID: 400, PGID: 500, Command: "bash -c dispatch"},
		400: {PPID: 300, PGID: 400, Command: "/usr/local/bin/claude serve"},
		300: {PPID: 1, PGID: 300, Command: "-zsh"},
	}
	got, err := FindAncestor(tree, 500, sigs)
	if err != nil {
		t.Fatal(err)
	}
	if got.Pid != 400 || got.Runtime != "claude" || got.PGID != 400 {
		t.Fatalf("wrong ancestor: %+v", got)
	}
}

func TestFindAncestorStartsAtSelf(t *testing.T) {
	sigs := testSignatures(t)
	// The pid itself is the agent.
	tree := fakeTree{700: {PPID: 300, PGID: 700, Command: "claude --serve"}}
	got, err := FindAncestor(tree, 700, sigs)
	if err != nil || got.Pid != 700 {
		t.Fatalf("self-match failed: %+v %v", got, err)
	}
}

func TestFindAncestorNoMatch(t *testing.T) {
	sigs := testSignatures(t)
	tree := fakeTree{
		500: {PPID: 300, PGID: 500, Command: "bash build.sh"},
		300: {PPID: 1, PGID: 300, Command: "-zsh"},
	}
	if _, err := FindAncestor(tree, 500, sigs); err == nil {
		t.Fatal("no agent ancestor must error")
	}
}

func TestFindAncestorStopsAtGonePid(t *testing.T) {
	sigs := testSignatures(t)
	// 500's parent 999 is not in the tree (gone): the walk stops, no match.
	tree := fakeTree{500: {PPID: 999, PGID: 500, Command: "bash x"}}
	if _, err := FindAncestor(tree, 500, sigs); err == nil {
		t.Fatal("walk into a gone pid must end unmatched")
	}
}

func TestFindAncestorLoopSafe(t *testing.T) {
	sigs := testSignatures(t)
	// A cycle 500 <-> 600 with no agent: must terminate, not spin.
	tree := fakeTree{
		500: {PPID: 600, PGID: 500, Command: "bash a"},
		600: {PPID: 500, PGID: 600, Command: "bash b"},
	}
	if _, err := FindAncestor(tree, 500, sigs); err == nil {
		t.Fatal("a cyclic tree with no agent must error, not loop")
	}
}
