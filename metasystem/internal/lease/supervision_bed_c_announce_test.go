package lease

// Ported from scripts/agents/supervision-fixtures.sh, scenario
// census-lifecycle: double arming by the same session collapses into one
// announcement, and a second live session announces without displacing the
// checkout holder (the advisor outcome's owner-level half). The two live
// sessions are this test process and its parent; nothing is spawned.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSupCDuplicateStartsCollapseAndASecondSessionNeverDisplacesTheHolder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	self := int64(os.Getpid())
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := Announce(root, "duplicate", self, selfStart(t), "fixture-main", "fake", ""); err != nil {
			t.Fatalf("announce attempt %d: %v", attempt, err)
		}
	}
	matches, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "mains", "duplicate-*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("duplicate session starts did not collapse: %v %v", matches, err)
	}
	holderBefore, err := CurrentHolder(root)
	if err != nil {
		t.Fatal(err)
	}

	parent := int64(os.Getppid())
	parentStart, ok := StartedAt(parent, nil)
	if !ok {
		t.Fatal("could not read the parent's start")
	}
	if _, err := Announce(root, "advisor-session", parent, parentStart, "advisor-session", "fake", ""); err != nil {
		t.Fatalf("the second live session could not announce: %v", err)
	}
	holderAfter, err := CurrentHolder(root)
	if err != nil {
		t.Fatal(err)
	}
	if holderAfter.MainId != holderBefore.MainId || holderAfter.SessionId != "duplicate" || holderAfter.Pid != self {
		t.Fatalf("a second session displaced the live checkout holder: before=%+v after=%+v", holderBefore, holderAfter)
	}
	view, err := ClassifyVerb(root, parent)
	if err != nil {
		t.Fatal(err)
	}
	if view.Class != ClassMain || view.Holder {
		t.Fatalf("the second session is not a non-holding main: %+v", view)
	}
}
