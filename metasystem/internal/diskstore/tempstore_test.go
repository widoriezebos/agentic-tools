package diskstore

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var readOwner = Owner{Kind: OwnerAttempt, Ref: "read:r1-a1"}

func newTempStoreBed(t *testing.T) scratchBed {
	t.Helper()
	return newScratchBed(t)
}

func (b scratchBed) createTemp(t *testing.T, owner Owner) (string, error) {
	t.Helper()
	return createTempStore(b.temp, b.registry, "metasystem-read-context.r1-a1", "read-context", owner, rand.Reader)
}

func (b scratchBed) releaseTemp(path string, owner Owner) error {
	return releaseTempStore(context.Background(), b.registry, path, "read-context", owner)
}

// An owner-released temporary store is registered before it holds a byte,
// lies under TMPDIR/metasystem where no stray recognizer reaches, and its
// owner's release removes it and keeps the record as history (fail-closed
// rule 3).
func TestTempStoreIsRegisteredAndReleasedByItsOwner(t *testing.T) {
	t.Parallel()
	bed := newTempStoreBed(t)
	path, err := bed.createTemp(t, readOwner)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != filepath.Join(mustEval(t, bed.temp), "metasystem") {
		t.Fatalf("store %s is not under TMPDIR/metasystem", path)
	}
	record := bed.onlyRecord(t)
	if record.Path != path || record.State != StateAccepted || record.Owner != readOwner || record.Identity.RootInode == 0 {
		t.Fatalf("record = %+v", record)
	}
	beside := filepath.Join(filepath.Dir(path), "beside")
	if err := os.WriteFile(beside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "payload"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := bed.releaseTemp(path, readOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the released store stayed: %v", err)
	}
	if released := bed.onlyRecord(t); released.State != StateReleased || released.ReleasedBy != "owner" {
		t.Fatalf("record after release = %+v; it stays as history", released)
	}
	if _, err := os.Lstat(beside); err != nil {
		t.Fatalf("an entry beside the store went: %v", err)
	}
}

// A path the request did not make is refused and kept; so is a path
// another owner's record names. An interrupted earlier try of the same
// owner is replaced through its own record.
func TestTempStoreClaimsOnlyWhatItMade(t *testing.T) {
	t.Parallel()
	bed := newTempStoreBed(t)
	foreign := filepath.Join(bed.temp, "metasystem", "metasystem-read-context.r1-a1")
	if err := os.MkdirAll(foreign, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := bed.createTemp(t, readOwner); err == nil || !strings.Contains(err.Error(), "metasystem disk show") {
		t.Fatalf("an unregistered path was claimed: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(foreign, "keep")); err != nil {
		t.Fatalf("the unregistered path lost its content: %v", err)
	}

	other := newTempStoreBed(t)
	if _, err := other.createTemp(t, readOwner); err != nil {
		t.Fatal(err)
	}
	if _, err := other.createTemp(t, Owner{Kind: OwnerAttempt, Ref: "read:r2-a1"}); err == nil {
		t.Fatal("a path another owner's record names was claimed")
	}
	path, err := other.createTemp(t, readOwner)
	if err != nil {
		t.Fatalf("the same owner's interrupted try was not replaced: %v", err)
	}
	records, _ := other.registry.Inventory()
	states := map[State]int{}
	for _, record := range records {
		states[record.State]++
		if record.State == StateAccepted && record.Path != path {
			t.Fatalf("the accepted record names %s, not %s", record.Path, path)
		}
	}
	if len(records) != 2 || states[StateReleased] != 1 || states[StateAccepted] != 1 {
		t.Fatalf("records after the replacement = %+v", records)
	}
}

// A temporary root inside a registered store (a process scratch TMPDIR)
// is refused: the outer store's release would take the inner one with it.
func TestTempStoreRefusesATemporaryRootInsideAStore(t *testing.T) {
	t.Parallel()
	bed := newTempStoreBed(t)
	created, err := newProcessScratch(bed.temp, bed.registry, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unlockAndClose(created.writer) })
	if _, err := createTempStore(created.record.Path, bed.registry, "metasystem-read-context.r1-a1", "read-context", readOwner, rand.Reader); err == nil {
		t.Fatal("a store was made inside a process scratch root")
	}
}

// Fail-closed rule 1: an unreadable record in the registry holds the
// release; rule 2: a directory put in the store's place with a copy of its
// marker is not the store. Both keep every byte.
func TestTempStoreReleaseFailsClosed(t *testing.T) {
	t.Parallel()
	t.Run("unreadable record", func(t *testing.T) {
		t.Parallel()
		bed := newTempStoreBed(t)
		path, err := bed.createTemp(t, readOwner)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "payload"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(bed.registry.RecordPath("01ZZZZZZZZZZZZZZZZZZZZZZZZ"), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := bed.releaseTemp(path, readOwner); err == nil {
			t.Fatal("a release with an unreadable record in the registry went ahead")
		}
		if _, err := os.Lstat(filepath.Join(path, "payload")); err != nil {
			t.Fatalf("the store lost its content: %v", err)
		}
	})
	t.Run("replaced directory", func(t *testing.T) {
		t.Parallel()
		bed := newTempStoreBed(t)
		path, err := bed.createTemp(t, readOwner)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "payload"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		moved := path + ".moved"
		if err := os.Rename(path, moved); err != nil {
			t.Fatal(err)
		}
		if err := CopyTree(context.Background(), moved, path); err != nil {
			t.Fatal(err)
		}
		if err := bed.releaseTemp(path, readOwner); err == nil {
			t.Fatal("a directory with a copied marker was released by its path string")
		}
		for _, dir := range []string{path, moved} {
			if _, err := os.Lstat(filepath.Join(dir, "payload")); err != nil {
				t.Fatalf("%s lost its content: %v", dir, err)
			}
		}
	})
}
