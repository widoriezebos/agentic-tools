package steward

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/spend"
)

var spendFenceNow = time.Date(2026, 9, 2, 20, 0, 0, 0, time.UTC)

func fixtureSpendLedger() spend.Ledger {
	return spend.Ledger{
		SchemaVersion: 1,
		ObservedAt:    spendFenceNow,
		Day:           "2026-09-02",
		Machine:       "bed-m1",
		Currency:      "USD",
		Settings: config.SpendSettings{
			Mode: "alert", Currency: "USD",
			DayTokenCeiling: 250000000, DayMoneyCeiling: 750,
			GoalTokenCeiling: 125000000, GoalMoneyCeiling: 300,
		},
		DayScope: spend.ScopeSummary{
			ID: "day-2026-09-02", Machine: "bed-m1", Day: "2026-09-02",
			Tokens: 173523756, Money: 67.91, Unpriced: 8, Unmeasured: 2,
		},
		Seat: spend.SeatSummary{
			DayTokens: 118425925, LifetimeTokens: 118425925, Files: 1,
		},
		ClaimedGoals: []string{"dispatch-cap-necessity"},
		GoalScopes: map[string]spend.ScopeSummary{
			"dispatch-cap-necessity": {
				ID: "goal-dispatch-cap-necessity", Goal: "dispatch-cap-necessity", Machine: "bed-m1",
				Tokens: 33922917, Money: 38.34, Unpriced: 4,
			},
		},
	}
}

func withSpendMeasurement(t *testing.T, measure func(string, string, time.Time) (spend.Ledger, error)) {
	t.Helper()
	prior := measureSpend
	measureSpend = measure
	t.Cleanup(func() { measureSpend = prior })
}

func TestSpendFenceHealthLineBytes(t *testing.T) {
	ledger := fixtureSpendLedger()
	withSpendMeasurement(t, func(string, string, time.Time) (spend.Ledger, error) { return ledger, nil })
	role, observation := checkSpendFence(t.TempDir(), spendFenceNow)
	line := (HealthVerdict{Aggregate: "healthy", Roles: []RoleVerdict{role}}).Line()
	want := "HEALTH healthy — spend-fence=alive (mode=alert day=2026-09-02 tokens=173523756/250000000 money=USD67.91/750.00 unpriced=8 unmeasured=2 unreadable=0 inflight=0; seat tokens=118425925 lifetime=118425925 files=1 aged=0 unmeasured requests=0; goal=dispatch-cap-necessity tokens=33922917/125000000 money=USD38.34/300.00 unpriced=4 unmeasured=0 unreadable=0)"
	if line != want || role.Status != HealthAlive || !observation.Valid || len(observation.Crossings) != 0 {
		t.Fatalf("below-ceiling health bytes changed:\n got: %s\nwant: %s\nrole=%+v observation=%+v", line, want, role, observation)
	}

	ledger.DayScope.Tokens = 500000000
	role, observation = checkSpendFence(t.TempDir(), spendFenceNow)
	line = (HealthVerdict{Aggregate: "healthy", Roles: []RoleVerdict{role}}).Line()
	if role.Status != HealthAlive || !strings.Contains(line, "spend-fence=alive (CROSSED day-2026-09-02.tokensx2 mode=alert") ||
		!strings.Contains(line, "; remedy: raise spend.ceiling.<scope>.<ceiling> in metasystem.conf on Wido's recorded word (R-60-m1); alert mode refuses nothing; see artifacts/agents/steward/spend/2026-09-02.json)") ||
		len(observation.Crossings) != 1 || observation.Crossings[0].Multiple != 2 {
		t.Fatalf("a crossing must stay healthy while carrying exact typed alert facts: %s role=%+v observation=%+v", line, role, observation)
	}
}

func crossing(scopeID, scope, ceiling string, multiple int, value, limit float64) SpendCrossing {
	return SpendCrossing{
		ScopeID: scopeID, Scope: scope, Ceiling: ceiling, Multiple: multiple,
		Machine: "bed-m1", Spend: value, Limit: limit, Day: "2026-09-02",
	}
}

func deliveryLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func spendDeliveryLines(t *testing.T, path string) []string {
	t.Helper()
	var result []string
	for _, line := range deliveryLines(t, path) {
		if strings.HasPrefix(line, "SPEND CROSSED ") {
			result = append(result, line)
		}
	}
	return result
}

func spendOwned(episodes []AlertEpisode) []AlertEpisode {
	var result []AlertEpisode
	for _, episode := range episodes {
		if episode.Owner == string(RoleSpendFence) {
			result = append(result, episode)
		}
	}
	return result
}

func TestSpendFenceHigherMultipleRearmsWhileLowerMultipleRemainsCrossed(t *testing.T) {
	root, sink := notifyRepo(t, "")
	x1 := crossing("day-2026-09-02", "day", "tokens", 1, 125, 100)
	x2 := crossing("day-2026-09-02", "day", "tokens", 2, 225, 100)
	for index, observation := range []SpendObservation{
		{Valid: true, Crossings: []SpendCrossing{x1}},
		{Valid: true, Crossings: []SpendCrossing{x2}},
		{Valid: true, Crossings: []SpendCrossing{x1}},
		{Valid: true, Crossings: []SpendCrossing{x2}},
	} {
		if err := UpdateSpendEpisodes(root, observation, spendFenceNow.Add(time.Duration(index)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	episodes, err := AlertEpisodes(root)
	if err != nil {
		t.Fatal(err)
	}
	owned := spendOwned(episodes)
	if len(owned) != 3 || len(deliveryLines(t, sink)) != 3 {
		t.Fatalf("x1 to x2 to x1 to x2 did not produce three submitted episodes: %+v", owned)
	}
	var activeX1, clearedX2, activeX2 int
	var firstX2ID, secondX2ID string
	for _, episode := range owned {
		if episode.ScopeID != x1.ScopeID || episode.Ceiling != "tokens" {
			t.Fatalf("spend episode lost its structured identity: %+v", episode)
		}
		switch {
		case episode.Multiple == 1 && !episode.Cleared:
			activeX1++
		case episode.Multiple == 2 && episode.Cleared:
			clearedX2++
			firstX2ID = episode.EpisodeID
		case episode.Multiple == 2 && !episode.Cleared:
			activeX2++
			secondX2ID = episode.EpisodeID
		}
	}
	if activeX1 != 1 || clearedX2 != 1 || activeX2 != 1 || firstX2ID == secondX2ID {
		t.Fatalf("the higher multiple did not clear and rearm independently while x1 stayed crossed: %+v", owned)
	}
}

func TestSpendFenceCrossingsHaveIndependentEpisodesAndRearmWhileOtherRoleDead(t *testing.T) {
	root, sink := notifyRepo(t, "")
	retroDigest := evidenceDigest("retro debt remains dead")
	dead := HealthVerdict{Aggregate: "unhealthy", FindingDigest: retroDigest,
		Roles: []RoleVerdict{{Role: RoleRetroDebt, Status: HealthDead}}}
	if _, err := UpdateAlertEpisodes(root, dead, "HEALTH unhealthy — retro debt", spendFenceNow); err != nil {
		t.Fatal(err)
	}
	tokens1 := crossing("day-2026-09-02", "day", "tokens", 1, 260000000, 250000000)
	money1 := crossing("day-2026-09-02", "day", "money", 1, 760, 750)
	first := SpendObservation{Valid: true, Crossings: []SpendCrossing{tokens1, money1}}
	if err := UpdateSpendEpisodes(root, first, spendFenceNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := UpdateSpendEpisodes(root, first, spendFenceNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	tokens2 := crossing("day-2026-09-02", "day", "tokens", 2, 510000000, 250000000)
	if err := UpdateSpendEpisodes(root, SpendObservation{Valid: true, Crossings: []SpendCrossing{tokens2, money1}}, spendFenceNow.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateAlertEpisodes(root, dead, "HEALTH unhealthy — retro debt", spendFenceNow.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := UpdateSpendEpisodes(root, SpendObservation{Valid: true, Crossings: []SpendCrossing{money1}}, spendFenceNow.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := UpdateSpendEpisodes(root, first, spendFenceNow.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}

	episodes, err := AlertEpisodes(root)
	if err != nil {
		t.Fatal(err)
	}
	owned := spendOwned(episodes)
	if len(owned) != 4 || len(deliveryLines(t, sink)) != 4 {
		t.Fatalf("crossing episodes or their one-time submissions were merged: owned=%+v deliveries=%v", owned, deliveryLines(t, sink))
	}
	var clearedToken, activeToken, activeMoney int
	for _, episode := range owned {
		for _, fact := range []string{episode.ScopeID + "." + episode.Ceiling, "machine=bed-m1", "spend=", "ceiling=", "ledger=artifacts/agents/steward/spend/2026-09-02.json", "raise: spend.ceiling.day." + episode.Ceiling} {
			if !strings.Contains(episode.Message, fact) {
				t.Fatalf("crossing message lost fact %q: %s", fact, episode.Message)
			}
		}
		if episode.Ceiling == "tokens" && episode.Cleared {
			clearedToken++
		}
		if episode.Ceiling == "tokens" && !episode.Cleared {
			activeToken++
		}
		if episode.Ceiling == "money" && !episode.Cleared {
			activeMoney++
		}
	}
	if clearedToken != 2 || activeToken != 1 || activeMoney != 1 {
		t.Fatalf("token recurrence did not clear independently while retro debt and money stayed active: %+v", owned)
	}
	for _, episode := range episodes {
		if episode.Owner == "" && (episode.Resolved || episode.Cleared) {
			t.Fatalf("the still-dead ordinary health episode was changed by spend updates: %+v", episode)
		}
	}
}

func TestTickCarriesSpendObservationAndUnknownDoesNotClearEpisodes(t *testing.T) {
	root, sink := notifyRepo(t, "")
	ledger := fixtureSpendLedger()
	ledger.DayScope.Tokens = 250000000
	calls := 0
	unknown := false
	withSpendMeasurement(t, func(string, string, time.Time) (spend.Ledger, error) {
		calls++
		if unknown {
			return spend.Ledger{}, errors.New("jobs directory cannot be listed: fixture")
		}
		return ledger, nil
	})
	result := TickResult{}
	process := identity.Ref{Pid: 1, StartedAtSec: 1}
	if err := completeTickHealth(root, &result, 1, process); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !result.Health.Spend.Valid || len(result.Health.Spend.Crossings) != 1 || len(spendDeliveryLines(t, sink)) != 1 {
		t.Fatalf("the tick did not carry checkSpendFence's one typed observation exactly once: calls=%d health=%+v deliveries=%v", calls, result.Health, deliveryLines(t, sink))
	}
	unknown = true
	if err := completeTickHealth(root, &result, 1, process); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || result.Health.Spend.Valid {
		t.Fatalf("the second tick did not carry the invalid measurement: calls=%d health=%+v", calls, result.Health)
	}
	episodes, err := AlertEpisodes(root)
	if err != nil {
		t.Fatal(err)
	}
	owned := spendOwned(episodes)
	if len(owned) != 1 || owned[0].Cleared || owned[0].Resolved || len(spendDeliveryLines(t, sink)) != 1 {
		t.Fatalf("an unknown tick cleared or resubmitted a spend episode: %+v", owned)
	}
}
