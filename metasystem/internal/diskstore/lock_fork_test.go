package diskstore

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// A forked child holds a duplicate of every descriptor open at the fork
// until it execs, and a flock belongs to the open file description, so a
// descriptor merely closed stays locked through the duplicate: under load a
// store was reported "an engine verb is inside the store" by the very next
// probe (TestDiskCleanTwiceSecondIsEmpty on Linux). Every record-lock
// holder unlocks before it closes. The duplicate is made at the moment the
// lock is taken, as a fork there would.
func TestARecordLockIsFreeOnceItsHolderEndsWhateverAForkDuplicated(t *testing.T) {
	t.Parallel()
	var duplicates []int
	registry := Registry{Dir: filepath.Join(realDir(t), "stores"), lockAcquired: func(file *os.File) {
		duplicate, err := syscall.Dup(int(file.Fd()))
		if err != nil {
			t.Error(err)
			return
		}
		duplicates = append(duplicates, duplicate)
	}}
	t.Cleanup(func() {
		for _, duplicate := range duplicates {
			_ = syscall.Close(duplicate)
		}
	})
	record, err := registry.Register(plainRegistration(filepath.Join(realDir(t), "store")), testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	free := func(step string) {
		t.Helper()
		if held, err := registry.ProbeRecordLock(record.ID); err != nil || !held {
			t.Fatalf("after %s the record lock is still held (free=%v err=%v)", step, held, err)
		}
	}
	if _, err := registry.Accept(record.ID); err != nil {
		t.Fatal(err)
	}
	free("an owner's transition")
	free("a probe")
	entrant, err := registry.Enter(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := entrant.Leave(); err != nil {
		t.Fatal(err)
	}
	free("an entrant's leave")
	critical, err := registry.TryCritical(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := critical.Release(); err != nil {
		t.Fatal(err)
	}
	free("a critical section")
	if len(duplicates) < 4 {
		t.Fatalf("the lock was taken %d times through the seam, want every holder", len(duplicates))
	}
	var held *HeldError
	if _, err := registry.TryCritical(record.ID); errors.As(err, &held) {
		t.Fatal("a released critical section is still held")
	}
}
