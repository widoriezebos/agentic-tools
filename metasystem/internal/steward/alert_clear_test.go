package steward

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHealthAlertAlternatingConditionsNotifyOnceEach(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sent := 0
	deliver := func(string, string) error { sent++; return nil }
	now := time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC)
	ids := make(map[string]string)
	for reading := 0; reading < 7; reading++ {
		condition := []string{"runner stale", "watcher stale"}[reading%2]
		health := HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest(condition), ShouldAlert: true}
		episode, err := updateAlertEpisodesWith(root, health, condition, now.Add(time.Duration(reading)*time.Minute), deliver)
		if err != nil || episode.Cleared || episode.Resolved || !episode.ClearedAt.IsZero() || !episode.ResolvedAt.IsZero() {
			t.Fatalf("reading %d did not open its condition: %+v %v", reading, episode, err)
		}
		if id := ids[condition]; id != "" && episode.EpisodeID != id {
			t.Errorf("reading %d opened %s instead of reusing %s", reading, episode.EpisodeID, id)
		}
		ids[condition] = episode.EpisodeID
	}
	if sent != 2 {
		t.Fatalf("seven alternating readings sent %d notices, want 2", sent)
	}
	// Recovery also retires the condition that is already closed.
	if _, err := updateAlertEpisodesWith(root, HealthVerdict{Aggregate: "healthy"}, "healthy", now.Add(7*time.Minute), deliver); err != nil {
		t.Fatal(err)
	}
	health := HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest("watcher stale"), ShouldAlert: true}
	if episode, err := updateAlertEpisodesWith(root, health, "watcher stale", now.Add(8*time.Minute), deliver); err != nil ||
		episode.EpisodeID == ids["watcher stale"] || sent != 3 {
		t.Fatalf("a recurrence after recovery is a new alert: %+v %d sent, %v", episode, sent, err)
	}
}

func TestSuppressedHealthAlertStillClosesOtherConditions(t *testing.T) {
	t.Parallel()
	f := newNotifyFixture(t)
	now := time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC)
	health := HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest("runner stale"), ShouldAlert: true}
	// The suppressed current alert sorts before the retained older condition.
	current := AlertEpisode{Schema: 1, EpisodeID: "alert-0000000000000000-1", Digest: health.FindingDigest,
		Message: "The runner is stale.", OpenedAt: now, Attempts: []AlertAttempt{}, TransportResult: TransportPending}
	if err := saveAlertEpisode(f.root, current); err != nil {
		t.Fatal(err)
	}
	current, _, err := ClearAlert(f.root, current.EpisodeID, AlertInvoker{Pid: 1, PidStartedAt: 1}, now)
	if err != nil {
		t.Fatal(err)
	}
	older := current
	older.EpisodeID, older.Digest = "alert-ffffffffffffffff-1", strings.Repeat("f", 64)
	older.Cleared, older.Suppressed, older.ClearedAt, older.ClearedBy = false, false, time.Time{}, nil
	if err := saveAlertEpisode(f.root, older); err != nil {
		t.Fatal(err)
	}
	at := now.Add(time.Minute)
	if held, err := f.alert(health, current.Message, at); err != nil || !reflect.DeepEqual(held, current) {
		t.Fatalf("the person's clear did not hold: %+v %v", held, err)
	}
	closed, err := loadAlertEpisode(alertPath(f.root, older.EpisodeID))
	if err != nil || !closed.Cleared || !closed.ClearedAt.Equal(at) || !closed.ResolvedAt.Equal(older.ResolvedAt) {
		t.Fatalf("the older alert stayed open: %+v %v", closed, err)
	}
}

func TestHealthAlertConditionChanges(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"different condition", "same condition", "resolved but open", "owned", "suppressed"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newNotifyFixture(t)
			now := time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC)
			health := HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest("runner stale")}
			first, err := f.alert(health, "The runner is stale.", now)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "resolved but open":
				first.Resolved, first.ResolvedAt = true, now.Add(10*time.Second)
			case "owned":
				first.Owner = laneSilentAlertOwner
			case "suppressed":
				first, _, err = ClearAlert(f.root, first.EpisodeID, AlertInvoker{Pid: 1, PidStartedAt: 1}, now.Add(10*time.Second))
				if err != nil {
					t.Fatal(err)
				}
				if held, err := f.alert(health, "The runner is stale.", now.Add(20*time.Second)); err != nil || !reflect.DeepEqual(held, first) {
					t.Fatalf("the person's clear did not hold: %+v %v", held, err)
				}
			}
			if err := saveAlertEpisode(f.root, first); err != nil {
				t.Fatal(err)
			}
			if name != "same condition" {
				health.FindingDigest = evidenceDigest("watcher stale")
			}
			at := now.Add(time.Minute)
			current, err := f.alert(health, "The watcher is stale.", at)
			if err != nil || current.Cleared || current.Resolved {
				t.Fatalf("current alert: %+v %v", current, err)
			}
			want := first
			switch name {
			case "different condition", "resolved but open":
				if !want.Resolved {
					want.Resolved, want.ResolvedAt = true, at
				}
				want.Cleared, want.ClearedAt = true, at
				want.ClosedBy = "condition-changed"
			case "suppressed":
				want.Suppressed = false
			}
			retained, err := loadAlertEpisode(alertPath(f.root, first.EpisodeID))
			if err != nil || !reflect.DeepEqual(retained, want) {
				t.Fatalf("retained alert: %+v, want %+v: %v", retained, want, err)
			}
			episodes, err := AlertEpisodes(f.root)
			wantCount := 2
			if name == "same condition" {
				wantCount = 1
			}
			if err != nil || len(episodes) != wantCount || (current.EpisodeID == first.EpisodeID) != (name == "same condition") {
				t.Fatalf("alert history: %+v %v", episodes, err)
			}
		})
	}
}

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
