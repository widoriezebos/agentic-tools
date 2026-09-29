package steward

// U5f/U5g (engine-owns-disk-lifetimes Part B, 3.10, DL2-14): a job worktree
// that predates registration is adopted at apply when its chain's job
// record is present, and is a stray, never released or wedged, when the
// record is gone; a preview adopts nothing.

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

func delegateWorktree(t *testing.T, top, job string, record bool) string {
	t.Helper()
	worktree := filepath.Join(top, "artifacts", "agents", "worktrees", job)
	gitdir := filepath.Join(top, ".git", "worktrees", job)
	for _, dir := range []string{worktree, gitdir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if record {
		path := filepath.Join(top, "artifacts", "agents", "jobs", job+".json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"jobId":"`+job+`","status":"completed"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return worktree
}

func delegatePass(top string, mode diskstore.Mode) diskstore.PassOptions {
	registry := diskstore.CheckoutRegistry(top)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	return diskstore.PassOptions{Kind: "checkout", Name: top, Registry: registry, LockPath: filepath.Join(registry.Dir, ".sweep.flock"),
		ReportPath: diskstore.CheckoutReportPath(top), PlanDir: filepath.Join(top, "plans-dir"), Mode: mode, Now: now,
		Clock: func() time.Time { return now }, Entropy: rand.Reader, Classes: []diskstore.Class{DelegateWorktrees{Top: top}}}
}

func TestDelegateWorktreesAreAdoptedOrReportedAsStrays(t *testing.T) {
	t.Parallel()
	top := canonicalPath(t.TempDir())
	adoptable := delegateWorktree(t, top, "chain-with-records", true)
	stray := delegateWorktree(t, top, "chain-records-pruned", false)
	if _, err := diskstore.RunPass(context.Background(), delegatePass(top, diskstore.ModePreview)); err != nil {
		t.Fatal(err)
	}
	if records, _ := diskstore.CheckoutRegistry(top).Inventory(); len(records) != 0 {
		t.Fatalf("a preview adopts nothing: %+v", records)
	}
	for range 2 {
		report, err := diskstore.RunPass(context.Background(), delegatePass(top, diskstore.ModeApply))
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Strays) != 1 || report.Strays[0].Path != stray {
			t.Fatalf("the worktree whose records were pruned is a stray: %+v", report.Strays)
		}
	}
	records, unreadable := diskstore.CheckoutRegistry(top).Inventory()
	if len(unreadable) != 0 || len(records) != 1 {
		t.Fatalf("exactly one adoption, and a repeat adopts nothing again: %+v %+v", records, unreadable)
	}
	record := records[0]
	if record.Path != adoptable || !record.Adopted || record.State != diskstore.StateAccepted || record.Class != diskstore.DelegateClass ||
		record.Owner != (diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: "chain-with-records"}) || record.Identity.Gitdir == "" {
		t.Fatalf("the adopted record: %+v", record)
	}
	for _, path := range []string{adoptable, stray} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("nothing is removed: %v", err)
		}
	}
}

// Rule 1: a registry that cannot be read holds the class; nothing is adopted.
func TestDelegateWorktreesHoldOnAnUnreadableRegistry(t *testing.T) {
	t.Parallel()
	top := canonicalPath(t.TempDir())
	delegateWorktree(t, top, "chain-with-records", true)
	registry := diskstore.CheckoutRegistry(top)
	if err := os.MkdirAll(registry.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registry.RecordPath("01K2Z7Q3M8XW1V0P9D4J6S5R2T"), []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := diskstore.RunPass(context.Background(), delegatePass(top, diskstore.ModeApply))
	if err != nil {
		t.Fatal(err)
	}
	if records, _ := registry.Inventory(); len(records) != 0 || len(report.Pending) == 0 {
		t.Fatalf("nothing is adopted and the class is pending: %+v %+v", records, report.Pending)
	}
}
