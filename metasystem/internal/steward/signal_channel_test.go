package steward

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/spend"
)

// channelFixture is a steward root whose metasystem.conf names the fake
// channel adapter, served by the fake provider on a port of its own. Every
// post the steward makes lands in the fake's journal.
type channelFixture struct {
	root string
	dir  string
}

func newChannelFixture(t *testing.T) channelFixture {
	t.Helper()
	fixture := channelFixture{root: t.TempDir(), dir: t.TempDir()}
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- fake.ServeReady(ctx, fixture.dir, ready) }()
	select {
	case <-ready:
	case err := <-done:
		cancel()
		t.Fatalf("the fake channel did not start: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("the fake channel did not stop cleanly: %v", err)
		}
	})
	conf := "channel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir=" + fixture.dir + "\n"
	if err := os.WriteFile(filepath.Join(fixture.root, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	return fixture
}

// posts are the texts the fake channel received, oldest first.
func (f channelFixture) posts(t *testing.T) []string {
	t.Helper()
	return channelPosts(t, f.dir)
}

func channelPosts(t *testing.T, dir string) []string {
	t.Helper()
	file, err := os.Open(filepath.Join(dir, "journal.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var texts []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row struct {
			Method string              `json:"method"`
			Form   map[string][]string `json:"form"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("fake journal line is not JSON: %q", scanner.Text())
		}
		if row.Method == "chat.postMessage" && len(row.Form["text"]) == 1 {
			texts = append(texts, row.Form["text"][0])
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return texts
}

// assertTwoLineNotice checks the plain shape: line 1 the situation, line 2
// the one command.
func assertTwoLineNotice(t *testing.T, text, situation, command string) {
	t.Helper()
	lines := strings.Split(text, "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], situation) || lines[1] != command {
		t.Fatalf("notice is not line 1 %q and line 2 %q:\n%s", situation, command, text)
	}
}

func episodesOwnedBy(t *testing.T, root, owner string) []AlertEpisode {
	t.Helper()
	episodes, err := AlertEpisodes(root)
	if err != nil {
		t.Fatal(err)
	}
	var owned []AlertEpisode
	for _, episode := range episodes {
		if episode.Owner == owner {
			owned = append(owned, episode)
		}
	}
	return owned
}

func seatIdleFixtureIncident(refusal int) SeatIdleIncident {
	return SeatIdleIncident{
		SessionID: "seat-session", MainID: "main-1", GoalID: "next-goal",
		BacklogDigest: evidenceDigest("unchanged backlog"), Refusal: refusal,
		ClaimDetail: "claimed", IntentDetail: "no intent",
	}
}

func TestSeatIdleThirdRefusalPostsOnceToTheChannel(t *testing.T) {
	t.Parallel()
	fixture := newChannelFixture(t)
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	first, err := RecordSeatIdleIncident(fixture.root, seatIdleFixtureIncident(3), now)
	if err != nil {
		t.Fatal(err)
	}
	posts := fixture.posts(t)
	if len(posts) != 1 {
		t.Fatalf("the third idle refusal posted %d times, want once: %q", len(posts), posts)
	}
	assertTwoLineNotice(t, posts[0], "seat-session", "metasystem status")
	if !strings.Contains(posts[0], "next-goal") {
		t.Fatalf("the idle notice does not name the ready goal: %q", posts[0])
	}
	again, err := RecordSeatIdleIncident(fixture.root, seatIdleFixtureIncident(4), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if again.EpisodeID != first.EpisodeID || again.TransportResult != TransportSubmitted {
		t.Fatalf("the repeated refusal did not stay in the one submitted episode: %+v", again)
	}
	if posts := fixture.posts(t); len(posts) != 1 {
		t.Fatalf("a repeated idle refusal posted again: %q", posts)
	}
	if pending, err := PendingNotifications(fixture.root); err != nil || len(pending) != 0 {
		t.Fatalf("the channel post also queued a local alarm: %+v %v", pending, err)
	}
}

func TestSeatIdleWithoutChannelKeepsTheSilentEpisode(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	episode, err := RecordSeatIdleIncident(root, seatIdleFixtureIncident(3), time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if episode.TransportResult != TransportPending || len(episode.Attempts) != 0 {
		t.Fatalf("with no channel the idle episode was submitted: %+v", episode)
	}
}

func TestSpendCrossingPostsOnceToTheChannel(t *testing.T) {
	fixture := newChannelFixture(t)
	ledger := fixtureSpendLedger()
	ledger.DayScope.Tokens = 250000000
	measure := func(string, string, time.Time) (spend.Ledger, error) { return ledger, nil }
	local := func(string, string) error {
		t.Fatal("with a channel configured the spend crossing went to the local notifier")
		return nil
	}
	dependencies := tickHealthDependencies{evaluate: tickHealthRoles(t, fixture.root, ledger.Machine, measure), now: time.Now, deliver: local}
	process := identity.Ref{Pid: 1, StartedAtSec: 1}
	for tick := 0; tick < 2; tick++ {
		result := TickResult{}
		if err := completeTickHealthWithDependencies(fixture.root, &result, 1, process, spendFenceNow.Add(time.Duration(tick)*time.Minute), dependencies); err != nil {
			t.Fatal(err)
		}
	}
	var spendPosts []string
	for _, text := range fixture.posts(t) {
		if strings.Contains(text, "spend.ceiling.") {
			spendPosts = append(spendPosts, text)
		}
	}
	if len(spendPosts) != 1 {
		t.Fatalf("one spend crossing over two ticks posted %d times: %q", len(spendPosts), fixture.posts(t))
	}
	assertTwoLineNotice(t, spendPosts[0], "day-2026-09-02", "metasystem settings set spend.ceiling.day.tokens NEW-LIMIT")
	owned := episodesOwnedBy(t, fixture.root, string(RoleSpendFence))
	if len(owned) != 1 || owned[0].TransportResult != TransportSubmitted || len(owned[0].Attempts) != 1 {
		t.Fatalf("the spend episode does not record its one channel submission: %+v", owned)
	}
}

func breachFixtureConfig(stops *int) TickConfig {
	return TickConfig{
		Now: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
		BreachStop: func(string, uint64) (string, error) {
			*stops++
			return "stopped", nil
		},
	}
}

func TestBreachStopNoticeOnNewStop(t *testing.T) {
	t.Parallel()
	fixture := newChannelFixture(t)
	stops := 0
	cfg := breachFixtureConfig(&stops)
	routes := []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 4}}
	scanner := func(string, time.Time) ([]dispatch.StopRoute, error) { return routes, nil }

	// The first report of a stop has no stop id; it still posts.
	reports := custodialBreachStops(fixture.root, cfg, scanner)
	if len(reports) != 1 || reports[0].State != "COMPLETE" || reports[0].StopID != "" {
		t.Fatalf("fixture stop did not complete: %+v", reports)
	}
	posts := fixture.posts(t)
	if len(posts) != 1 {
		t.Fatalf("a completed breach stop posted %d times, want once: %q", len(posts), posts)
	}
	assertTwoLineNotice(t, posts[0], "Goal budget-goal was stopped: it spent its budget.", "metasystem goal resume budget-goal")

	// The next tick reports the same revision, now with its stop id: no
	// duplicate.
	routes = []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 4, StopID: "stop-1"}}
	custodialBreachStops(fixture.root, cfg, scanner)
	custodialBreachStops(fixture.root, cfg, scanner)
	if posts := fixture.posts(t); len(posts) != 1 {
		t.Fatalf("the same stopped revision posted again: %q", posts)
	}

	// A new revision stopped is a new stop: one more notice.
	routes = []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 5}}
	custodialBreachStops(fixture.root, cfg, scanner)
	if posts := fixture.posts(t); len(posts) != 2 {
		t.Fatalf("a newly stopped revision did not post once: %q", posts)
	}
	if stops != 4 {
		t.Fatalf("the notice changed the stop producer: %d stops", stops)
	}
	if owned := episodesOwnedBy(t, fixture.root, "breach-stop"); len(owned) != 2 {
		t.Fatalf("want one breach-stop episode per stopped revision, got %+v", owned)
	}
}

func TestBreachStopNoticeOnlyForACompletedStop(t *testing.T) {
	t.Parallel()
	fixture := newChannelFixture(t)
	cfg := TickConfig{Now: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)}
	scanner := func(string, time.Time) ([]dispatch.StopRoute, error) {
		return []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 4}}, nil
	}
	// No lifecycle wired: the report is FAILED, and nothing was stopped.
	reports := custodialBreachStops(fixture.root, cfg, scanner)
	if len(reports) != 1 || reports[0].State != "FAILED" {
		t.Fatalf("fixture stop did not fail: %+v", reports)
	}
	if posts := fixture.posts(t); len(posts) != 0 {
		t.Fatalf("a failed stop posted a stop notice: %q", posts)
	}
}

func TestBreachStopWithoutChannelKeepsTheReportOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stops := 0
	scanner := func(string, time.Time) ([]dispatch.StopRoute, error) {
		return []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 4}}, nil
	}
	reports := custodialBreachStops(root, breachFixtureConfig(&stops), scanner)
	if len(reports) != 1 || reports[0].State != "COMPLETE" {
		t.Fatalf("fixture stop did not complete: %+v", reports)
	}
	if _, err := os.Stat(alertDir(root)); !os.IsNotExist(err) {
		t.Fatalf("with no channel the stop opened an alert episode: %v", err)
	}
}
