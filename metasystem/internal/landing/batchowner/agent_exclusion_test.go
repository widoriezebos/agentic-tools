package batchowner

import (
	"errors"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// TestNoBatchOwnerLaunchedBesideALandingAgent (A-a, critique F-1): every
// path that ensures the owner (the keeper, landing start, a join) launches
// none while a landing agent launch has not ended, and still succeeds, since
// the agent holds the lane; an unreadable launch store refuses. With no
// agent the dead owner is launched.
func TestNoBatchOwnerLaunchedBesideALandingAgent(t *testing.T) {
	root := t.TempDir()
	originalEnsure, originalLive := BatchOwnerEnsure, LandingAgentLive
	t.Cleanup(func() { BatchOwnerEnsure, LandingAgentLive = originalEnsure, originalLive })
	launched := 0
	BatchOwnerEnsure = BatchOwnerEnsureSeams{
		Inspect: func(string) (int64, identity.Liveness, error) { return 0, identity.Dead, nil },
		Wake:    func(int64) error { t.Fatal("woke a dead owner"); return nil },
		Launch:  func(string) error { launched++; return nil },
	}
	LandingAgentLive = func() (string, bool, error) { return "landing-0011", true, nil }
	if err := EnsureBatchOwner(root); err != nil || launched != 0 {
		t.Fatalf("an agent runs: err %v, launched %d; want no launch and no error", err, launched)
	}
	LandingAgentLive = func() (string, bool, error) { return "", false, errors.New("launches unreadable") }
	if err := EnsureBatchOwner(root); err == nil || launched != 0 {
		t.Fatalf("unreadable launches: err %v, launched %d; want refused", err, launched)
	}
	LandingAgentLive = func() (string, bool, error) { return "", false, nil }
	if err := EnsureBatchOwner(root); err != nil || launched != 1 {
		t.Fatalf("no agent: err %v, launched %d; want one launch", err, launched)
	}
	if reason, _ := landingLaneKeeper(t.TempDir()).Hold(root); reason != "" {
		t.Fatalf("the keeper holds with no agent: %q", reason)
	}
	LandingAgentLive = func() (string, bool, error) { return "landing-0011", true, nil }
	if reason, _ := landingLaneKeeper(t.TempDir()).Hold(root); reason == "" {
		t.Fatal("the production keeper does not hold while an agent runs")
	}
}
