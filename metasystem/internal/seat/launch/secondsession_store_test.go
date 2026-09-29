package launch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// A second session's worktree is registered, reserved with its bootstrap,
// before git writes a byte of it (Part B 3.2 "Seat", U5e); a failed
// creation leaves the reservation as history, never pending for ever.
func TestSecondSessionRegistersItsWorktreeBeforeGitMakesIt(t *testing.T) {
	t.Parallel()
	bed := newSecondSessionBed(t)
	bed.options.Name = "registered"
	registry := diskstore.CheckoutRegistry(bed.harness)
	git := bed.options.Git
	var atAdd []diskstore.Record
	bed.options.Git = func(args ...string) (string, error) {
		if len(args) > 3 && args[2] == "worktree" && args[3] == "add" {
			atAdd, _ = registry.Inventory()
		}
		return git(args...)
	}
	if _, err := SecondSession(bed.options); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(bed.checkout), "registered")
	if len(atAdd) != 1 || atAdd[0].Path != want || atAdd[0].Class != diskstore.SessionWorktreeClass || atAdd[0].State != diskstore.StateReserved ||
		atAdd[0].Owner != (diskstore.Owner{Kind: diskstore.OwnerSession, Ref: "registered"}) || atAdd[0].Bootstrap != diskstore.BootstrapRef(4242, 1786104000) {
		t.Fatalf("the record when git ran = %+v", atAdd)
	}

	failing := newSecondSessionBed(t)
	failing.options.Name = "failed"
	failing.options.Git = func(args ...string) (string, error) {
		if len(args) > 2 && args[2] == "rev-parse" {
			return failing.checkout + "\n", nil
		}
		return "", os.ErrPermission
	}
	if _, err := SecondSession(failing.options); err == nil {
		t.Fatal("a failed worktree add succeeded")
	}
	records, _ := diskstore.CheckoutRegistry(failing.harness).Inventory()
	if len(records) != 1 || records[0].State != diskstore.StateReleased {
		t.Fatalf("a failed creation's record = %+v", records)
	}
}
