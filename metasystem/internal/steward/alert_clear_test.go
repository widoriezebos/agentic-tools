package steward

import (
	"testing"
	"time"
)

// A person's clear of a live health alert holds: the next ticks with the
// same finding open nothing and notify nobody; once health has read healthy
// the same finding is new again.
func TestManualClearSuppressesLiveHealthAlert(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sent := 0
	deliverTo := func(string, string) error { sent++; return nil }
	now := time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC)
	unhealthy := HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest("runner stale"), ShouldAlert: true,
		Roles: []RoleVerdict{{Role: RoleStewardRunner, Status: HealthDead, Reason: "runner stale"}}}
	episode, err := updateAlertEpisodesWith(root, unhealthy, "HEALTH unhealthy — runner stale", now, deliverTo)
	if err != nil || sent != 1 {
		t.Fatalf("first alert: %v, %d sent", err, sent)
	}
	if _, changed, err := ClearAlert(root, episode.EpisodeID, AlertInvoker{Pid: 1, PidStartedAt: 1}, now.Add(time.Minute)); err != nil || !changed {
		t.Fatalf("clear: %v %v", changed, err)
	}
	for minute := 2; minute < 6; minute++ {
		if _, err := updateAlertEpisodesWith(root, unhealthy, "HEALTH unhealthy — runner stale", now.Add(time.Duration(minute)*time.Minute), deliverTo); err != nil {
			t.Fatal(err)
		}
	}
	episodes, err := AlertEpisodes(root)
	if err != nil || len(episodes) != 1 || sent != 1 {
		t.Fatalf("the cleared alert reopened: %d episodes, %d sent, %v", len(episodes), sent, err)
	}
	healthy := HealthVerdict{Aggregate: "healthy", FindingDigest: evidenceDigest("")}
	if _, err := updateAlertEpisodesWith(root, healthy, "HEALTH healthy", now.Add(6*time.Minute), deliverTo); err != nil {
		t.Fatal(err)
	}
	if _, err := updateAlertEpisodesWith(root, unhealthy, "HEALTH unhealthy — runner stale", now.Add(7*time.Minute), deliverTo); err != nil {
		t.Fatal(err)
	}
	if episodes, _ := AlertEpisodes(root); len(episodes) != 2 || sent != 2 {
		t.Fatalf("after a healthy reading the finding is new: %d episodes, %d sent", len(episodes), sent)
	}
}

// A person's clear of a standing spend crossing holds until the crossing is
// gone once.
func TestManualClearSuppressesLiveSpendAlert(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sent := 0
	deliverTo := func(string, string) error { sent++; return nil }
	now := time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC)
	crossing := SpendObservation{Valid: true, Crossings: []SpendCrossing{{ScopeID: "machine.m1e", Scope: "machine", Ceiling: "tokens", Multiple: 1, Machine: "m1e", Spend: 2, Limit: 1, Day: "2026-09-30"}}}
	if err := updateSpendEpisodesWith(root, crossing, now, deliverTo); err != nil || sent != 1 {
		t.Fatalf("first crossing: %v %d", err, sent)
	}
	episodes, _ := AlertEpisodes(root)
	if _, _, err := ClearAlert(root, episodes[0].EpisodeID, AlertInvoker{Pid: 1, PidStartedAt: 1}, now); err != nil {
		t.Fatal(err)
	}
	for minute := 1; minute < 4; minute++ {
		if err := updateSpendEpisodesWith(root, crossing, now.Add(time.Duration(minute)*time.Minute), deliverTo); err != nil {
			t.Fatal(err)
		}
	}
	if episodes, _ := AlertEpisodes(root); len(episodes) != 1 || sent != 1 {
		t.Fatalf("the cleared crossing reopened: %d episodes, %d sent", len(episodes), sent)
	}
	if err := updateSpendEpisodesWith(root, SpendObservation{Valid: true}, now.Add(5*time.Minute), deliverTo); err != nil {
		t.Fatal(err)
	}
	if err := updateSpendEpisodesWith(root, crossing, now.Add(6*time.Minute), deliverTo); err != nil {
		t.Fatal(err)
	}
	if episodes, _ := AlertEpisodes(root); len(episodes) != 2 || sent != 2 {
		t.Fatalf("after the crossing went away it is new: %d episodes, %d sent", len(episodes), sent)
	}
}
