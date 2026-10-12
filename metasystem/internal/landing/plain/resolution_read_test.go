package plain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolutionReadBoundReturnsOnlyLaneOwnedRead(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		owned                bool
		minutes              int
		bound                string
		stopped              bool
		stopErr              error
		pushErr              error
		policy               string
		priorReturns         int
		wantStop, wantReturn bool
	}{
		{name: "lane over", owned: true, minutes: 21, stopped: true, wantStop: true, wantReturn: true},
		{name: "push fails", owned: true, minutes: 21, stopped: true, pushErr: errors.New("offline"), wantStop: true, wantReturn: true},
		{name: "person policy", owned: true, minutes: 21, policy: "person"},
		{name: "two returns", owned: true, minutes: 21, priorReturns: 2},
		{name: "non-shrinking design", owned: true, minutes: 21, priorReturns: 1},
		{name: "person over", minutes: 21},
		{name: "at bound", owned: true, minutes: 20},
		{name: "under", owned: true, minutes: 19},
		{name: "override", owned: true, minutes: 21, bound: "25"},
		{name: "stop fails", owned: true, minutes: 21, stopErr: errors.New("process still runs"), wantStop: true},
		{name: "already passed", owned: true, minutes: 21, wantStop: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if tc.bound != "" {
				if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("landing.read-minutes="+tc.bound+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			fix := &Fix{Attempt: "resolution", Goal: "goal", Tip: "tip", Commit: "merge-sha", Job: "build", Units: []string{"lane-merge-1"}, State: "reviewing", Read: "read", ReadLaneOwned: tc.owned, ReadStartedAt: bedNow.UTC().Format(time.RFC3339Nano)}
			if err := WriteFix(root, fix); err != nil {
				t.Fatal(err)
			}
			for n := 0; n < tc.priorReturns; n++ {
				sha := fmt.Sprintf("old-%d", n)
				if err := appendLine(queuePath(root), Line{Goal: "goal", SHA: sha}); err != nil {
					t.Fatal(err)
				}
				if err := appendLine(queuePath(root), Line{Goal: "goal", SHA: sha, Outcome: StateReturned, ReturnOrigin: &ReturnOrigin{Class: "design", Failures: []string{"previous-merge"}}}); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := HandIn(root, Line{Goal: "goal", SHA: "tip", Branch: "goal/goal"}); err != nil {
				t.Fatal(err)
			}
			called := false
			seams := ProveSeams{Now: func() time.Time { return bedNow.Add(time.Duration(tc.minutes) * time.Minute) }, BudgetHome: t.TempDir()}
			if tc.policy != "" {
				seams.Policy = func(string) (PolicyValue, error) { return PolicyValue{Value: tc.policy, Source: "fixture"}, nil }
			}
			pushes := 0
			seams.Git = func(dir string, args ...string) (string, error) {
				pushes++
				if dir != root || strings.Join(args, " ") != "push origin merge-sha:refs/heads/lane-merges/goal" || !called {
					t.Fatalf("publication dir=%s args=%v stopped=%v", dir, args, called)
				}
				latest, ok, err := Latest(root, "goal")
				if err != nil || !ok || latest.State != StateWaiting {
					t.Fatalf("published after returning the hand-in: %+v %v", latest, err)
				}
				return "", tc.pushErr
			}
			entry, err := ExpireResolutionRead(root, func(ref string) (bool, error) {
				called = true
				if ref != "read" {
					t.Fatalf("stopped another read: %q", ref)
				}
				return tc.stopped, tc.stopErr
			}, seams)
			if called != tc.wantStop || (entry != nil) != tc.wantReturn || !errors.Is(err, tc.stopErr) {
				t.Fatalf("stop=%v entry=%+v err=%v", called, entry, err)
			}
			latest, _, err := Latest(root, "goal")
			if err != nil {
				t.Fatal(err)
			}
			retained, err := FixForAttempt(root, fix.Attempt)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantReturn {
				location := "the automatic merge and the resolution are published at origin refs/heads/lane-merges/goal (merge-sha)"
				if tc.pushErr != nil {
					location = "the merge could not be published: offline; the saved resolution patch is fixes/resolution.resolution.patch"
				}
				want := "the resolution read of goal ran 1 min past the bound of 20 min; " + location + "; review and hand in again"
				if pushes != 1 {
					t.Fatalf("publication calls=%d", pushes)
				}
				if entry.Reason != want || latest.State != StateReturned || retained.State != "closed" || retained.Commit != "merge-sha" || retained.ReadMinutes != 21 || retained.Reason != want {
					t.Fatalf("return=%+v retained=%+v", entry, retained)
				}
				if again, err := ExpireResolutionRead(root, func(string) (bool, error) { t.Fatal("repeat stopped a closed read"); return false, nil }, seams); err != nil || again != nil {
					t.Fatalf("repeat=%+v %v", again, err)
				}
			} else if latest.State != StateWaiting || retained.State != "reviewing" || retained.ReadLaneOwned != tc.owned || !strings.EqualFold(retained.ReadStartedAt, fix.ReadStartedAt) {
				t.Fatalf("untouched read changed: %+v %+v", latest, retained)
			}
			if !tc.wantReturn && pushes != 0 {
				t.Fatalf("published without a return: %d", pushes)
			}
			if tc.policy != "" || tc.priorReturns > 0 {
				if !strings.Contains(retained.Reason, "merge read ran 21 min, bound 20") || !strings.Contains(retained.Reason, "landing give-back goal") {
					t.Fatalf("refused read has no finding or person action: %+v", retained)
				}
			}
		})
	}
}
