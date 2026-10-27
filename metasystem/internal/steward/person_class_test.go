package steward

import (
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestOnlyActsAndDeliveriesReachAPerson(t *testing.T) {
	t.Parallel()
	fixture := newChannelFixture(t)
	root := fixture.root
	now := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	transports := 0
	deliver := func(string, string) error { transports++; return nil }
	dead := HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest("runner stale"), ShouldAlert: true,
		Roles: []RoleVerdict{{Role: RoleStewardRunner, Status: HealthDead, Reason: "runner stale"}}}
	if _, err := updateAlertEpisodesWith(root, dead, "HEALTH unhealthy — runner stale", now, noticeFunnel{}, deliver); err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenAlert(root, AlertOpening{Owner: "pattern:stuck-unit", Work: "goal/2", Since: now, Now: now, Message: "a unit stands still", Deliver: deliver}); err != nil {
		t.Fatal(err)
	}
	if err := updateSpendEpisodesWith(root, SpendObservation{Valid: true, Crossings: []SpendCrossing{crossing("day-2026-10-25", "day", "tokens", 2, 500, 250)}}, now, noticeFunnel{}, deliver); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordSeatIdleIncident(root, seatIdleFixtureIncident(3), now); err != nil {
		t.Fatal(err)
	}
	if err := updateLaneSilentEpisodes(root, LaneSilence{Known: true, Silent: true, Since: now.Add(-time.Hour), Queued: 1}, now, noticeFunnel{}); err != nil {
		t.Fatal(err)
	}
	if outcome := deliverNoticeWith(root, Notice{Source: NoticeAlert, Owner: "provider-stale", Ref: "provider-stale-p", Message: "the stale mark expired"}, deliver, noticeFunnel{}); outcome.Result != TransportWithheld {
		t.Fatalf("the provider notice was not kept: %+v", outcome)
	}
	for _, nonce := range []string{"verdict-degraded", "seat-start-a", "ledger-attention-1", "noticing-1", "engine-rearm-1", "revive-failure", "intent-1-reap", "seat-presence-m-held-1"} {
		if err := QueueNotification(root, PendingNotification{Nonce: nonce, Message: "queued " + nonce}); err != nil {
			t.Fatal(err)
		}
	}
	if delivered, err := deliverPendingWithDependencies(root, pendingDeliveryDependencies{transport: deliver}); err != nil || delivered != 0 {
		t.Fatalf("the queue delivered %d: %v", delivered, err)
	}
	if posts := fixture.posts(t); transports != 0 || len(posts) != 0 {
		t.Fatalf("a notice reached a person: %d local, %q posted", transports, posts)
	}
	var classes []string
	for _, record := range journalOf(t, root) {
		if record.Delivered || record.Withheld == "" {
			t.Fatalf("a journal line was not kept: %+v", record)
		}
		classes = append(classes, record.Withheld)
	}
	for _, class := range []string{"health", "pattern", "spend", "seat-idle", "lane-silent", "provider", "verdict", "seat-start", "ledger", "narration", "re-arm", "reap", "seat-presence"} {
		if !slices.Contains(classes, class) {
			t.Fatalf("no line kept as %s: %q", class, classes)
		}
	}
	episodes, err := AlertEpisodes(root)
	if err != nil || len(episodes) != 5 {
		t.Fatalf("episodes: %+v %v", episodes, err)
	}
	for _, episode := range episodes {
		if episode.TransportResult != TransportWithheld {
			t.Fatalf("episode %s was not kept: %+v", episode.Owner, episode)
		}
	}
	if pending, err := PendingNotifications(root); err != nil || len(pending) != 0 {
		t.Fatalf("the queue kept items: %+v %v", pending, err)
	}
}

func TestAHandoffNoticeIsKeptFromThePerson(t *testing.T) {
	t.Parallel()
	root, intent := prepareRevivalHandoff(t, "5000000000000009")
	home := testprovider.Register(t, root)
	if held, err := completeHandoffRevival(root, handoffProbe(*intent.Handoff, identity.Alive, true, nil), TickConfig{ProviderHome: home}, deadCensus(), intent.Nonce, func(Intent) error {
		t.Fatal("a live predecessor launched its successor")
		return nil
	}); err != nil || !held.Held {
		t.Fatalf("fixture did not stage its hold notice: %+v %v", held, err)
	}
	transports := 0
	if delivered, err := deliverPendingWithDependencies(root, pendingDeliveryDependencies{transport: func(string, string) error { transports++; return nil }}); err != nil || delivered != 0 || transports != 0 {
		t.Fatalf("the handoff notice reached a person: %d delivered, %d transports, %v", delivered, transports, err)
	}
	records := journalOf(t, root)
	if len(records) != 1 || records[0].Withheld != "handoff" {
		t.Fatalf("the handoff notice was not kept as handoff: %+v", records)
	}
}

func TestUnknownOwnerIsWithheld(t *testing.T) {
	t.Parallel()
	for _, notice := range []Notice{{Source: NoticeAlert, Owner: "something-new"}, {Source: NoticeSteward, Ref: "a-new-nonce"}} {
		if class, allowed := personClass(notice); allowed || class != "unknown" {
			t.Fatalf("%+v reads %s, allowed %t", notice, class, allowed)
		}
	}
}
