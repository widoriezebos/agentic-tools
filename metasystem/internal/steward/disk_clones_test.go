package steward

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// A sibling holding the checkout's root commit is reported, read-only, as
// a clone with its size and no removal command (Round B3-3); another seat's armed checkout,
// an unrelated repository, a plain directory and a linked worktree are not;
// nothing is removed.
func TestUnownedClonesAreReportedNeverRemoved(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	checkout := filepath.Join(parent, "checkout")
	for _, dir := range []string{filepath.Join(checkout, ".git"), filepath.Join(parent, "clone", ".git"), filepath.Join(parent, "seat", ".git"), filepath.Join(parent, "other", ".git"), filepath.Join(parent, "plain")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(parent, "worktree"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "worktree", ".git"), []byte("gitdir: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git := func(_ context.Context, dir string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "rev-list" && dir == checkout:
			return []byte("root1\n"), nil
		case args[0] == "cat-file" && (dir == filepath.Join(parent, "clone") || dir == filepath.Join(parent, "seat")):
			return nil, nil
		}
		return nil, errors.New("exit status 1")
	}
	class := UnownedClones{GitRoot: checkout, Git: git, Armed: []string{filepath.Join(parent, "seat")}}
	report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "checkout", Name: checkout,
		Registry: diskstore.Registry{Dir: filepath.Join(parent, "stores")}, Mode: diskstore.ModeApply, Now: attemptsNow,
		Clock: func() time.Time { return attemptsNow }, Classes: []diskstore.Class{class}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Foreign) != 1 || report.Foreign[0].Path != filepath.Join(parent, "clone") || len(report.Actions) != 0 ||
		!strings.Contains(report.Foreign[0].Verdict.Reason, "remove it yourself if you no longer need it") ||
		strings.Contains(report.Foreign[0].Verdict.Reason, "metasystem") {
		t.Fatalf("report: %+v", report)
	}
	held := UnownedClones{GitRoot: checkout, Git: git, ArmedErr: errors.New("registry unreadable")}
	if items, _ := held.Plan(context.Background(), nil); len(items) != 1 || items[0].Verdict.Decision != diskstore.Pending {
		t.Fatalf("an unreadable armed registry holds the report: %+v", items)
	}
	if _, err := os.Stat(filepath.Join(parent, "clone")); err != nil {
		t.Fatal("nothing is removed")
	}
}
