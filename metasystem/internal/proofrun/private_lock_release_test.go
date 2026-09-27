package proofrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// A child forked while a private lock description is open keeps a duplicate
// of it until the child execs. The duplicate here stands in for that child:
// a released guard, census probe, record lock or writer proof must be free
// at once for the next taker, however long the copy lives.
func TestPrivateLockReleaseDoesNotWaitForAForkedCopy(t *testing.T) {
	t.Parallel()
	for _, release := range []struct {
		name string
		call func(*testing.T, *os.File)
	}{
		{"host probe", func(t *testing.T, file *os.File) {
			if err := releaseHostProbe(file); err != nil {
				t.Fatal(err)
			}
		}},
		{"scratch lock", func(_ *testing.T, file *os.File) { releaseScratchLock(file) }},
	} {
		t.Run(release.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "admission.lock")
			held, acquired, err := tryHostFile(path)
			if err != nil || !acquired {
				t.Fatalf("take the private lock: acquired=%t err=%v", acquired, err)
			}
			forkedCopy, err := unix.FcntlInt(held.Fd(), unix.F_DUPFD_CLOEXEC, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer unix.Close(forkedCopy)
			release.call(t, held)
			next, acquired, err := tryHostFile(path)
			if err != nil || !acquired {
				t.Fatalf("released lock still held through a forked copy: acquired=%t err=%v", acquired, err)
			}
			if err := releaseHostProbe(next); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// ReserveLocked holds the admission guard of its own namespace, so its slot
// census reads that namespace only. Another namespace may hold a lease marker
// between its creation and its claim (written under that namespace's
// guard); reading it made admission refuse with unknown overlap. Not
// parallel: it points the package default namespace at the foreign one.
func TestReserveCensusReadsOnlyItsOwnAdmissionNamespace(t *testing.T) {
	previous := hostAdmissionDirectoryForTest
	hostAdmissionDirectoryForTest = filepath.Join(t.TempDir(), "foreign-admission")
	t.Cleanup(func() { hostAdmissionDirectoryForTest = previous })
	foreign, err := hostAdmissionDirectory()
	if err != nil {
		t.Fatal(err)
	}
	unclaimed := filepath.Join(foreign, "lease-heavy-"+strings.Repeat("0", 32))
	if err := os.WriteFile(unclaimed, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f := newOwnershipFixture(t)
	_, decision := f.reserve("goal-census", "own-namespace", map[string]string{"check": strings.Repeat("c", 64)}, "", "", 0)
	if decision.Disposition != DispositionExecuted {
		t.Fatalf("admission read a foreign namespace's unclaimed marker: %+v", decision)
	}
}
