package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Two writers of one entry must never share a temporary name: with a fixed
// path+".tmp" one writer renames the other's half-written temporary away and
// the loser's rename fails.
func TestJournalConcurrentWritersOfOneEntryNeverCollide(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(journalDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	const writers, rounds = 8, 40
	var group sync.WaitGroup
	errs := make(chan error, writers*rounds)
	for writer := 0; writer < writers; writer++ {
		group.Add(1)
		go func(writer int) {
			defer group.Done()
			for round := 0; round < rounds; round++ {
				entry := Entry{Opid: "op-shared", Machine: fmt.Sprintf("writer-%d-%d", writer, round), Phase: PhaseCreated}
				if err := writeEntry(root, entry); err != nil {
					errs <- err
				}
			}
		}(writer)
	}
	group.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("a concurrent journal write failed: %v", err)
	}
	if _, err := ReadEntry(root, "op-shared"); err != nil {
		t.Fatalf("the entry is not a whole record after concurrent writes: %v", err)
	}
	info, err := os.Stat(entryPath(root, "op-shared"))
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("the entry must keep mode 0644: %v %v", info, err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(journalDir(root), "*.tmp"))
	if len(leftovers) != 0 {
		t.Fatalf("temporaries left behind: %v", leftovers)
	}
}
