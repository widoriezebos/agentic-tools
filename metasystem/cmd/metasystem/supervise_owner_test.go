package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The restricted-procfs refusal wiring, proven end to end: with a
// restrictive mounts table, supervision arming
// refuses before it waits on the start gate — the verb returns promptly
// with a non-zero code instead of blocking, which is exactly the refusal
// contract (a hung test here would itself be the failure).
func TestSuperviseOwnerRefusesRestrictedProcfs(t *testing.T) {
	dir := t.TempDir()
	mounts := filepath.Join(dir, "mounts")
	os.WriteFile(mounts, []byte("proc /proc proc rw,hidepid=2 0 0\n"), 0o644)
	saved := procfsMounts
	procfsMounts = mounts
	defer func() { procfsMounts = saved }()
	code := runSuperviseOwnerLoop([]string{"--repo", dir, "--tag", "b2-fixture"})
	if code != 1 {
		t.Fatalf("arming under hidepid=2 must refuse with 1, got %d", code)
	}
}
