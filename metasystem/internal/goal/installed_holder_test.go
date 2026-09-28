package goal

import "testing"

// AnnounceHolderForTest records a process as the checkout's announced main
// through the lease owner. internal/lease imports this package, so the
// internal tests cannot call it directly; the external test package sets the
// hook (installed_holder_external_test.go) before any test runs.
var AnnounceHolderForTest func(root, session string, pid, start, startTicks int64, bootID, tag, runtime, lineage string) error

func announceTestHolder(t *testing.T, root, session string, pid, start, startTicks int64, bootID, tag, runtime, lineage string) {
	t.Helper()
	if AnnounceHolderForTest == nil {
		t.Fatal("AnnounceHolderForTest is unset; installed_holder_external_test.go wires the lease owner")
	}
	if err := AnnounceHolderForTest(root, session, pid, start, startTicks, bootID, tag, runtime, lineage); err != nil {
		t.Fatalf("announce %s holder: %v", tag, err)
	}
}
