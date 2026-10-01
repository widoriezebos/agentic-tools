package batchowner

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// TestNoBatchOwnerLaunchedBesideALandingAgent (A-a, critique F-1): every
// path that ensures the owner (the keeper, landing start, a join) launches
// none while a landing agent launch has not ended, and still succeeds, since
// the agent holds the lane; an unreadable launch store refuses. With no
// agent the dead owner is launched. The production keeper holds the same.
func TestNoBatchOwnerLaunchedBesideALandingAgent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	launched := 0
	seams := BatchOwnerEnsureSeams{
		Inspect: func(string) (int64, identity.Liveness, error) { return 0, identity.Dead, nil },
		Wake:    func(int64) error { t.Fatal("woke a dead owner"); return nil },
		Launch:  func(string) error { launched++; return nil },
	}
	running := func() (string, bool, error) { return "landing-0011", true, nil }
	if err := ensureBatchOwnerWith(root, seams, running); err != nil || launched != 0 {
		t.Fatalf("an agent runs: err %v, launched %d; want no launch and no error", err, launched)
	}
	unreadable := func() (string, bool, error) { return "", false, errors.New("launches unreadable") }
	if err := ensureBatchOwnerWith(root, seams, unreadable); err == nil || launched != 0 {
		t.Fatalf("unreadable launches: err %v, launched %d; want refused", err, launched)
	}
	none := func() (string, bool, error) { return "", false, nil }
	if err := ensureBatchOwnerWith(root, seams, none); err != nil || launched != 1 {
		t.Fatalf("no agent: err %v, launched %d; want one launch", err, launched)
	}
	if reason, _ := landingAgentHoldWith(none); reason != "" {
		t.Fatalf("the hold holds with no agent: %q", reason)
	}
	if reason, _ := landingAgentHoldWith(running); reason == "" {
		t.Fatal("the hold does not hold while an agent runs")
	}
	if landingLaneKeeper(t.TempDir()).Hold == nil {
		t.Fatal("the production owner keeper has no landing agent hold")
	}
}

// TestSupervisedOwnerYieldsToALandingAgent (A-a, re-review 2 B-1): the
// supervised owner component takes its lease only under the ensure lock and
// only when no landing agent runs or is starting; otherwise it takes none
// and says why. Its lease clears the start mark an owner start left.
func TestSupervisedOwnerYieldsToALandingAgent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	acquired := 0
	acquire := func(string) (BatchOwnerLease, error) { acquired++; return BatchOwnerLease{}, nil }
	running := func() (string, bool, error) { return "landing-0011", true, nil }
	if _, reason, err := AcquireBatchOwnerUnlessAgent(root, running, acquire); err != nil || acquired != 0 || !strings.Contains(reason, "landing-0011") {
		t.Fatalf("an agent runs: reason %q err %v, acquired %d; want the owner to yield", reason, err, acquired)
	}
	unreadable := func() (string, bool, error) { return "", false, errors.New("launches unreadable") }
	if _, _, err := AcquireBatchOwnerUnlessAgent(root, unreadable, acquire); err == nil || acquired != 0 {
		t.Fatalf("unreadable launches: err %v, acquired %d; want refused", err, acquired)
	}
	now := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	seams := BatchOwnerEnsureSeams{
		Inspect: func(string) (int64, identity.Liveness, error) { return 0, identity.Dead, nil },
		Wake:    func(int64) error { return nil },
		Launch:  func(string) error { return nil },
		Now:     func() time.Time { return now },
	}
	none := func() (string, bool, error) { return "", false, nil }
	if err := ensureBatchOwnerWith(root, seams, none); err != nil {
		t.Fatal(err)
	}
	if !OwnerLaunchedWithin(root, now.Add(9*time.Minute), OwnerStartWindow) || OwnerLaunchedWithin(root, now.Add(OwnerStartWindow), OwnerStartWindow) {
		t.Fatal("the start mark does not last the start window on the injected clock")
	}
	if _, reason, err := AcquireBatchOwnerUnlessAgent(root, none, acquire); err != nil || reason != "" || acquired != 1 {
		t.Fatalf("no agent: reason %q err %v, acquired %d; want the lease", reason, err, acquired)
	}
	if OwnerLaunchedWithin(root, now.Add(time.Minute), OwnerStartWindow) {
		t.Fatal("the owner took its lease, yet its start mark still holds the agent")
	}
}
