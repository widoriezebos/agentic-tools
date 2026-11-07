package steward

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestBreachStopAlertsEndThroughBothTickCompositions(t *testing.T) {
	t.Parallel()
	for _, composition := range []struct {
		name  string
		build func(TickConfig) tickHealthDependencies
	}{
		{"continuation", func(cfg TickConfig) tickHealthDependencies { return defaultTickContinuationDependencies(cfg).health }},
		{"config", tickHealthConfigDependencies},
	} {
		t.Run(composition.name, func(t *testing.T) {
			t.Parallel()
			b := newHealthBed(t, EnrollmentFixture, "")
			at := b.base.Add(time.Second)
			tree := &goal.TreeGoals{
				Live: map[string]*goal.GoalFile{
					"fenced":   {State: goal.StateClaimed, Claimed: &goal.ClaimRecord{}, StopFence: &goal.StopFence{}},
					"approved": {State: goal.StateApproved},
				},
				Done:      map[string]*goal.GoalFile{"done": {State: goal.StateDone}},
				Abandoned: map[string]*goal.GoalFile{"abandoned": {State: goal.StateAbandoned}},
			}
			reads := 0
			cfg := TickConfig{GoalProjection: func(root string, now time.Time) (goal.Projection, error) {
				reads++
				if reads != 1 || root != b.root || !now.Equal(at) {
					t.Fatalf("unexpected ledger read %d for %q at %s", reads, root, now)
				}
				return goal.Projection{Tree: tree}, nil
			}}
			dependencies := situationHealthDependencies(b, at, composition.build(cfg))
			seeds := make(map[string]AlertEpisode)
			for _, id := range []string{"done", "abandoned", "approved", "fenced", "unknown", "person-cleared"} {
				episode := situationStopEpisode(b, id)
				if id == "done" || id == "fenced" {
					episode.Acknowledged = true
					episode.AcknowledgedAt = b.base.Add(-time.Minute)
					episode.AcknowledgedBy = &AlertInvoker{Pid: 9001, PidStartedAt: 77, ArgvDigest: evidenceDigest("seen")}
				}
				if id == "person-cleared" {
					episode.ScopeID = "done"
					episode.Cleared, episode.ClearedAt = true, b.base.Add(-time.Minute)
					episode.ClearedBy = &AlertInvoker{Pid: 9002, PidStartedAt: 78, ArgvDigest: evidenceDigest("person clear")}
				}
				b.writeJSON(filepath.Join("artifacts", "agents", "steward", "alerts", episode.EpisodeID+".json"), episode)
				seeds[id] = episode
			}
			var result TickResult
			if err := completeTickHealthWithDependencies(b.root, &result, b.generation, b.runner, at, dependencies); err != nil {
				t.Fatal(err)
			}
			codes := map[string]string{"done": "goal-done", "abandoned": "goal-abandoned", "approved": "goal-not-stopped"}
			for id, seed := range seeds {
				episode, err := loadAlertEpisode(alertPath(b.root, seed.EpisodeID))
				if err != nil {
					t.Fatal(err)
				}
				if code := codes[id]; code != "" {
					if !episode.Resolved || !episode.Cleared {
						t.Fatalf("old stops stayed open: %s: %+v", id, episode)
					}
					seed.Resolved, seed.ResolvedAt = true, at
					seed.Cleared, seed.ClearedAt, seed.ClosedBy = true, at, code
				}
				if id == "fenced" && episode.Cleared {
					t.Fatal("a stopped goal's alert ended while it was still stopped")
				}
				if !reflect.DeepEqual(episode, seed) {
					t.Fatalf("%s: unexpected episode change\ngot  %+v\nwant %+v", id, episode, seed)
				}
			}
			if reads != 1 {
				t.Fatalf("old stops stayed open: ledger read %d times, want 1", reads)
			}
		})
	}
}

func TestBreachStopAlertsReadTheLedgerOnlyWhileOneIsOpen(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		stop bool
		err  error
	}{
		{name: "only-health-and-pattern"},
		{name: "ledger-error", stop: true, err: errors.New("ledger unavailable")},
		{name: "nil-tree", stop: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newHealthBed(t, EnrollmentFixture, "")
			at := b.base.Add(time.Second)
			reads := 0
			cfg := TickConfig{GoalProjection: func(root string, now time.Time) (goal.Projection, error) {
				reads++
				if !tc.stop || reads != 1 || root != b.root || !now.Equal(at) {
					t.Fatalf("unexpected ledger read %d for %q at %s", reads, root, now)
				}
				return goal.Projection{}, tc.err
			}}
			dependencies := situationHealthDependencies(b, at, tickHealthConfigDependencies(cfg))
			var seeds []AlertEpisode
			if tc.stop {
				seeds = append(seeds, situationStopEpisode(b, "unknown"))
			} else {
				for _, other := range []struct{ id, owner string }{{"health", ""}, {"pattern", "pattern:health-standing-red"}} {
					episode := situationStopEpisode(b, other.id)
					episode.Owner = other.owner
					seeds = append(seeds, episode)
				}
				// A person's clear also removes the need to read the ledger.
				episode := situationStopEpisode(b, "person-cleared")
				episode.Cleared, episode.ClearedAt = true, b.base
				episode.ClearedBy = &AlertInvoker{Pid: 9002, PidStartedAt: 78}
				seeds = append(seeds, episode)
			}
			for _, episode := range seeds {
				b.writeJSON(filepath.Join("artifacts", "agents", "steward", "alerts", episode.EpisodeID+".json"), episode)
			}
			var result TickResult
			if err := completeTickHealthWithDependencies(b.root, &result, b.generation, b.runner, at, dependencies); err != nil {
				t.Fatalf("an unreadable ledger must not fail the health pass: %v", err)
			}
			wantReads := 0
			if tc.stop {
				wantReads = 1
			}
			if reads != wantReads {
				t.Fatalf("ledger read %d times, want %d", reads, wantReads)
			}
			for _, seed := range seeds {
				if seed.Owner != breachStopAlertOwner {
					continue
				}
				episode, err := loadAlertEpisode(alertPath(b.root, seed.EpisodeID))
				if err != nil || !reflect.DeepEqual(episode, seed) {
					t.Fatalf("stop changed without a readable goal: %+v, %v", episode, err)
				}
			}
		})
	}
}

func situationHealthDependencies(b *healthBed, at time.Time, dependencies tickHealthDependencies) tickHealthDependencies {
	dependencies.evaluate, dependencies.deliver = b.evaluate, b.deliver
	dependencies.now = func() time.Time { return at }
	dependencies.lane, dependencies.ask = nil, nil
	dependencies.prober, dependencies.probeRuntime, dependencies.examineLedger = nil, nil, nil
	return dependencies
}

func situationStopEpisode(b *healthBed, id string) AlertEpisode {
	return AlertEpisode{
		Schema: 1, EpisodeID: "alert-old-" + id, Digest: evidenceDigest(id),
		Owner: breachStopAlertOwner, ScopeID: id, Message: "Goal " + id + " was stopped: it spent its budget.",
		OpenedAt: b.base.Add(-time.Hour), Attempts: []AlertAttempt{}, TransportResult: TransportSubmitted,
	}
}

func TestStandingHealthAlertSaysItsFindingInWords(t *testing.T) {
	t.Parallel()
	b := newHealthBed(t, EnrollmentFixture, "")
	b.lookPath = func(string) (string, error) { return "/fixture/runtime", nil }
	b.writeFile("metasystem.conf", []byte("metasystem.runtimes=codex\ncapability.snapshot-max-age-days=30\n"))
	// Notification starts when the existing automatic repair allowance ends.
	for observation := 1; observation < healthFailureLimit; observation++ {
		b.health(b.base.Add(time.Duration(observation) * time.Millisecond))
	}
	first := b.tick(b.base.Add(5 * time.Millisecond))
	b.writeFile("metasystem.conf", []byte("metasystem.runtimes=claude\ncapability.snapshot-max-age-days=30\n"))
	at := b.base.Add(6 * time.Millisecond)
	second := b.tick(at)
	if first.FindingDigest != second.FindingDigest || !strings.Contains(first.Line(), "missing or stale capability snapshots: codex") ||
		!strings.Contains(second.Line(), "missing or stale capability snapshots: claude") {
		t.Fatalf("ticks must keep the finding but change its reason: first=%s second=%s", first.Line(), second.Line())
	}
	episodes := b.episodes()
	if len(episodes) != 1 {
		t.Fatalf("standing finding has %d episodes, want 1", len(episodes))
	}
	episode := episodes[0]
	if episode.Message != "Runtime capability checks are missing or outdated" || strings.ContainsAny(episode.Message, "=\n") {
		t.Fatalf("health alert does not say its finding in words: %q", episode.Message)
	}
	if len(episode.Evidence) != 1 || !strings.Contains(episode.Evidence[0].Fact, "capability-snapshots=dead") ||
		!strings.Contains(episode.Evidence[0].Fact, "missing or stale capability snapshots: claude") ||
		episode.Evidence[0].Record != HealthRecordPath(b.root) || episode.Evidence[0].At != at.Format(time.RFC3339Nano) {
		t.Fatalf("health evidence did not refresh on the second pass: %+v", episode.Evidence)
	}
	if len(episode.Attempts) != 1 {
		t.Fatalf("unchanged finding submitted %d attempts, want 1", len(episode.Attempts))
	}
}
