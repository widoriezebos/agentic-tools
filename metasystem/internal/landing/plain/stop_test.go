package plain

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

func TestStopClosurePreservesOriginalIdentity(t *testing.T) {
	t.Parallel()
	for _, at := range []string{"", "2026-10-06T10:00:00Z"} {
		t.Run("opened="+at, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			now := time.Date(2026, 10, 6, 11, 0, 0, 0, time.UTC)
			first := Stop{Loop: "lane-proof", Subject: "goal", Attempt: 2, Tree: "first-tree", ProofAttempt: "first", Decision: "stop", At: at}
			second := first
			second.Tree, second.ProofAttempt = "second-tree", "second"
			if err := withLock(install, func() error {
				if err := appendLine(stopsPath(install), first); err != nil {
					return err
				}
				if err := appendLine(stopsPath(install), second); err != nil {
					return err
				}
				return closeMatchingStopsLocked(install, "matching check admitted", now, func(stop Stop) bool { return stop.Tree == second.Tree })
			}); err != nil {
				t.Fatal(err)
			}
			if stop, err := NewestStop(install); err != nil || stop == nil || stop.Tree != first.Tree {
				t.Fatalf("closure did not isolate the original stop: %+v %v", stop, err)
			}
			if err := withLock(install, func() error {
				return closeMatchingStopsLocked(install, "matching check admitted", now, func(stop Stop) bool { return stop.Tree == first.Tree })
			}); err != nil {
				t.Fatal(err)
			}
			if stop, err := NewestStop(install); err != nil || stop != nil {
				t.Fatalf("closed stop reappeared: %+v %v", stop, err)
			}
			lines, err := readLines[Stop](stopsPath(install))
			if err != nil || len(lines) != 4 || lines[0].Decision != "stop" || lines[1].Decision != "stop" {
				t.Fatalf("closure erased history: %+v %v", lines, err)
			}
		})
	}
}

func TestLegacyStopClosureStillClosesEveryReader(t *testing.T) {
	t.Parallel()
	for _, loop := range []string{"lane-proof", "lane-return"} {
		t.Run(loop, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			now := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
			stop := Stop{Loop: loop, Subject: "lane", Attempt: 2, Decision: "stop", Handoff: "ask lane", At: now.Add(-time.Hour).Format(time.RFC3339), Tree: "old-tree", ProofAttempt: "old-attempt"}
			machine := func(string) (string, error) { return "fixture", nil }
			if err := withLock(install, func() error { return appendLine(stopsPath(install), stop) }); err != nil {
				t.Fatal(err)
			}
			if err := SyncPolicyQuestion(install, machine, now); err != nil {
				t.Fatal(err)
			}
			questions, unread := channel.WalkOpenQuestions(install)
			if len(questions) != 1 || len(unread) != 0 {
				t.Fatalf("open request: %+v %v", questions, unread)
			}
			close := stop
			close.Decision, close.At = "close", now.Format(time.RFC3339)
			close.Tree, close.ProofAttempt = "", ""
			if err := withLock(install, func() error { return appendLine(stopsPath(install), close) }); err != nil {
				t.Fatal(err)
			}
			if got, err := NewestStop(install); err != nil || got != nil {
				t.Fatalf("legacy stop reopened: %+v %v", got, err)
			}
			if err := SyncPolicyQuestion(install, machine, now); err != nil {
				t.Fatal(err)
			}
			if current, err := channel.ReadQuestion(install, questions[0].ID); err != nil || current.State != "closed" {
				t.Fatalf("legacy question reopened: %+v %v", current, err)
			}
			if err := withLock(install, func() error { return closeSubjectStopsLocked(install, loop, "lane", "again", now) }); err != nil {
				t.Fatal(err)
			}
			lines, err := readLines[Stop](stopsPath(install))
			if err != nil || len(lines) != 2 {
				t.Fatalf("legacy closure duplicated: %+v %v", lines, err)
			}
		})
	}
}
