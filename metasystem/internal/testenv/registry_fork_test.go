package testenv

import (
	"bytes"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// A forked child holds a duplicate of every descriptor open at the fork
// until it execs, and a flock belongs to the open file description, so a
// registry lock that is merely closed stays held through the duplicate:
// under load the owner's own cleanup left its home looking live and the
// next sweep kept it ("survived cleanup",
// TestRegistrySidecarCleanupKeepsRecordsWhenLiveCustodianLogIsUnlinked).
// Every registry and custodian-log lock holder unlocks before it closes.

// forkDuplicate stands in for a concurrent fork: a close-on-exec duplicate
// of file's open file description, made atomically so no real fork copies it.
func forkDuplicate(t *testing.T, file *os.File) {
	t.Helper()
	fd, err := unix.FcntlInt(file.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Close(fd) })
}

// The owner's lease: a duplicate made while the owner holds its lock does not
// keep the home live once the owner's cleanup has ended.
func TestARegistryHomeIsDeadOnceItsOwnerEndsWhateverAForkDuplicated(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	owner, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	records := owner.path + ".fixture-refs-4242"
	checkTestenv(t, os.WriteFile(records, []byte("recorded fixture\n"), 0o600))
	forkDuplicate(t, owner.lock)

	// Unsettled records keep the home through the owner's cleanup.
	var report bytes.Buffer
	checkTestenv(t, owner.cleanupReporting(&report))
	requireSidecars(t, true, owner.path, records)

	checkTestenv(t, os.Remove(records))
	if !HomeSettled(owner.path) {
		t.Fatal("the ended owner's home reads live through a fork's duplicate")
	}
	removeDeadRegistryHomes(root, &report)
	requireSidecars(t, false, owner.path)
	if report.Len() != 0 {
		t.Fatalf("unexpected report: %q", report.String())
	}
}

// A sweep's own probe: a duplicate made when a sweep or an observation takes
// a home's owner lock, or a custodian log's lock, does not make the next
// sweep read that lock held. registryLockTaken is package state, so this test
// is serial.
func TestARegistrySweepLeavesNoLockAForkDuplicated(t *testing.T) {
	previous := registryLockTaken
	t.Cleanup(func() { registryLockTaken = previous })
	registryLockTaken = func(file *os.File) { forkDuplicate(t, file) }

	root := t.TempDir()
	owner, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	records, log := owner.path+".fixture-refs-4242", owner.path+".custodian-4242.log"
	checkTestenv(t, os.WriteFile(records, []byte("recorded fixture\n"), 0o600))
	checkTestenv(t, os.WriteFile(log, nil, 0o600))
	var report bytes.Buffer
	checkTestenv(t, owner.cleanupReporting(&report))

	// Each pass takes the owner lock and the log lock and keeps the home:
	// the records are unsettled.
	if HomeSettled(owner.path) {
		t.Fatal("a home with unsettled records read as settled")
	}
	removeDeadRegistryHomes(root, &report)
	requireSidecars(t, true, owner.path, records, log)

	checkTestenv(t, os.Remove(records))
	if !HomeSettled(owner.path) {
		t.Fatal("an earlier probe's duplicate kept a lock held")
	}
	removeDeadRegistryHomes(root, &report)
	requireSidecars(t, false, owner.path, log)
}
