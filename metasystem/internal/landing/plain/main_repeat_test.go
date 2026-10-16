package plain

import (
	"os/exec"
	"testing"
)

func TestMainRepeatRetainsItsOneExecution(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"pending", "red", "green"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			cause := "flake"
			if outcome == "red" {
				cause = "unclassified"
			}
			prior := Result{Attempt: "main-replay-1", Tree: "main-tree", Result: Red, Repeat: "started",
				Cause: &Cause{Kind: cause}, Failed: []FailedUnit{{Unit: "u/a", Tests: []string{"TestA"}}}}
			if outcome == "green" {
				prior.Result, prior.FlakePublished = Green, true
				prior.Cause = nil
			}
			if err := withLock(b.install, func() error { return appendLine(resultsPath(b.install), prior) }); err != nil {
				t.Fatal(err)
			}
			b.seams.Command = func(*exec.Cmd) error { t.Fatal("a retained main repeat ran again"); return nil }
			red := Result{Attempt: prior.Attempt, Tree: prior.Tree, Result: Red, Cause: &Cause{Kind: "unclassified"}, Failed: prior.Failed}
			got := repeatRed(b.seams, b.install, b.install, "fixture", b.install, Running{Attempt: prior.Attempt, Trunk: true}, scopeDecision{}, red, true)
			if got.Result != prior.Result || got.FlakePublished != prior.FlakePublished || (got.Cause == nil) != (prior.Cause == nil) || got.Cause != nil && got.Cause.Kind != prior.Cause.Kind {
				t.Fatalf("retained outcome lost: %+v cause=%+v", got, got.Cause)
			}
			if len(b.records) != 0 {
				t.Fatal("retained flake republished")
			}
		})
	}
}
