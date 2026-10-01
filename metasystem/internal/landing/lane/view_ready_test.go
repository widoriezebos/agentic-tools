package lane

import (
	"strings"
	"testing"
)

// A lane that cannot run (its checkout's supervision is not armed) shows
// no landing agent started, why, and the one command a person runs.
func TestViewOfALaneThatCannotStartNamesTheFix(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	register(t, home, root)
	sources := viewSources(home, false, nil)
	sources.Ready = func(root string) error { return UnarmedRefusal(root) }
	view := BuildView(sources)
	fix := "metasystem system start --repo " + resolved(root)
	if view.Owner.State != OwnerNotStarted || view.Owner.LastExit == nil || !strings.Contains(*view.Owner.LastExit, "supervision is not armed") ||
		view.Owner.RetryHint == nil || !strings.Contains(*view.Owner.RetryHint, fix) {
		t.Fatalf("owner = %+v", view.Owner)
	}
	if strings.Join(view.Owner.Fix, " ") != fix {
		t.Fatalf("fix argv = %q, want %q", view.Owner.Fix, fix)
	}
	for _, want := range []string{"landing agent not-started", "supervision is not armed", fix} {
		if !strings.Contains(view.Summary, want) {
			t.Errorf("summary %q lacks %q", view.Summary, want)
		}
	}
	// A running agent is never second-guessed.
	sources = viewSources(home, true, nil)
	sources.Ready = func(string) error { t.Fatal("asked whether a lane whose agent runs can start"); return nil }
	if view := BuildView(sources); view.Owner.State != OwnerRunning {
		t.Fatalf("running agent = %+v", view.Owner)
	}
}
