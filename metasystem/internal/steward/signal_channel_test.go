package steward

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"

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
	fixture := newUnservedChannelFixture(t)
	fixture.serve(t)
	return fixture
}

// newUnservedChannelFixture names the fake channel before it serves: a post
// fails until serve starts it.
func newUnservedChannelFixture(t *testing.T) channelFixture {
	t.Helper()
	fixture := channelFixture{root: t.TempDir(), dir: t.TempDir()}
	conf := "channel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir=" + fixture.dir + "\n"
	if err := os.WriteFile(filepath.Join(fixture.root, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f channelFixture) serve(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- fake.ServeReady(ctx, f.dir, ready) }()
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

func TestSeatIdleThirdRefusalIsKeptFromThePerson(t *testing.T) {
	t.Parallel()
	fixture := newChannelFixture(t)
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	first, err := RecordSeatIdleIncident(fixture.root, seatIdleFixtureIncident(3), now)
	if err != nil {
		t.Fatal(err)
	}
	if posts := fixture.posts(t); len(posts) != 0 || first.TransportResult != TransportWithheld {
		t.Fatalf("the third idle refusal reached the channel: %q %+v", posts, first)
	}
	again, err := RecordSeatIdleIncident(fixture.root, seatIdleFixtureIncident(4), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if again.EpisodeID != first.EpisodeID || again.TransportResult != TransportWithheld {
		t.Fatalf("the repeated refusal did not stay in the one kept episode: %+v", again)
	}
	if posts := fixture.posts(t); len(posts) != 0 {
		t.Fatalf("a repeated idle refusal posted: %q", posts)
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

func TestSpendCrossingIsKeptFromThePerson(t *testing.T) {
	t.Parallel()
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
	if posts := fixture.posts(t); len(posts) != 0 {
		t.Fatalf("a spend crossing reached the channel: %q", posts)
	}
	owned := episodesOwnedBy(t, fixture.root, string(RoleSpendFence))
	if len(owned) != 1 || owned[0].TransportResult != TransportWithheld {
		t.Fatalf("the spend episode is not one kept episode: %+v", owned)
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

// breachBox is the stopped goal's box in the breach tests.
var breachBox = goal.Budget{ElapsedLimit: "5d", AttemptLimit: 12, ReservedJobMinutesLimit: 2400, ActiveJobLimit: 1, ReviewRoundLimit: 20}

func TestBreachStopAsksOncePerGoalRevision(t *testing.T) {
	t.Parallel()
	fixture := newChannelFixture(t)
	stops := 0
	cfg := breachFixtureConfig(&stops)
	routes := []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 4, Reason: "elapsed-limit", Budget: &breachBox, Elapsed: 61 * time.Hour}}
	scanner := func(string, time.Time) ([]dispatch.StopRoute, error) { return routes, nil }
	if reports := custodialBreachStops(fixture.root, cfg, scanner); len(reports) != 1 || reports[0].State != "COMPLETE" {
		t.Fatalf("fixture stop did not complete: %+v", reports)
	}
	asked, _ := channel.WalkOpenQuestions(fixture.root)
	if len(asked) != 1 || len(fixture.posts(t)) != 0 {
		t.Fatalf("a completed breach stop asked %d questions and posted %q", len(asked), fixture.posts(t))
	}
	q := asked[0]
	decision := "Budget goal: it used 7d5h of its 5d box and stopped at " + cfg.Now.Local().Format("15:04") + "."
	if !q.Contract || q.Goal != "budget-goal" || q.Recommendation != "One more day" || !strings.HasPrefix(q.Text, decision+"\n\n★ One more day — its time box becomes the time used so far plus one working day (8 hours), counted when you choose; the other limits stay. Undo: goal budget back to today's box (it stops again).\nLeave it stopped — nothing runs and nothing is spent;") ||
		!strings.Contains(q.Text, "If you do nothing: it stays stopped; nothing resumes it.") ||
		!slices.Equal(q.Facts, []string{"stopped at revision 4: elapsed-limit", "undo after One more day: metasystem goal budget budget-goal 5d/12/2400m/1/20"}) {
		t.Fatalf("the box question:\n%s\nfacts: %q", q.Text, q.Facts)
	}
	if !slices.Equal(q.Options[0].Act, []string{"metasystem", "goal", "budget", "budget-goal", "one-more-day"}) || q.Options[1].Act != nil {
		t.Fatalf("the options' acts: %+v", q.Options)
	}
	// The next tick reports the same revision with its stop id: nothing more.
	routes[0].StopID = "stop-1"
	custodialBreachStops(fixture.root, cfg, scanner)
	if all, _ := channel.WalkQuestions(fixture.root); len(all) != 1 {
		t.Fatalf("the same stopped revision asked again: %+v", all)
	}
	// A closed question still holds its revision.
	if err := channel.Close(fixture.root, q.ID, "budget at 10:00", nil, channel.DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	custodialBreachStops(fixture.root, cfg, scanner)
	if all, _ := channel.WalkQuestions(fixture.root); len(all) != 1 {
		t.Fatalf("a closed question's revision asked again: %+v", all)
	}
	// A new revision stopped asks again.
	routes = []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 5, Reason: "elapsed-limit", Budget: &breachBox}}
	custodialBreachStops(fixture.root, cfg, scanner)
	asked, _ = channel.WalkOpenQuestions(fixture.root)
	if len(asked) != 1 || !strings.HasPrefix(asked[0].Plain.Decision, "budget goal: it used its 5d box and stopped at") || stops != 4 {
		t.Fatalf("a newly stopped revision: %+v, %d stops", asked, stops)
	}
}

func TestBreachStopAskFitsALongGoalId(t *testing.T) {
	t.Parallel()
	fixture := newChannelFixture(t)
	stops := 0
	long := strings.Repeat("stopped-", 7) + "goal"
	if utf8.RuneCountInString(long) != 60 {
		t.Fatalf("fixture goal id is %d runes", utf8.RuneCountInString(long))
	}
	routes := []dispatch.StopRoute{{GoalID: long, Revision: 2, Reason: "elapsed-limit", Budget: &breachBox, Elapsed: 41 * time.Hour}}
	scanner := func(string, time.Time) ([]dispatch.StopRoute, error) { return routes, nil }
	reports := custodialBreachStops(fixture.root, breachFixtureConfig(&stops), scanner)
	if len(reports) != 1 || reports[0].State != "COMPLETE" {
		t.Fatalf("fixture stop did not complete: %+v", reports)
	}
	if _, err := noticeBreachStops(fixture.root, reports, time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("the long goal id's ask failed: %v", err)
	}
	asked, _ := channel.WalkOpenQuestions(fixture.root)
	if len(asked) != 1 {
		t.Fatalf("the long goal id asked %d questions", len(asked))
	}
	q := asked[0]
	if n := utf8.RuneCountInString(q.Options[0].Consequence); n > 200 {
		t.Fatalf("One more day's consequence is %d runes", n)
	}
	if !slices.Contains(q.Facts, "undo after One more day: metasystem goal budget "+long+" 5d/12/2400m/1/20") {
		t.Fatalf("the facts carry no undo line: %q", q.Facts)
	}
}

func TestBreachStopNoticeOnlyForACompletedStop(t *testing.T) {
	t.Parallel()
	fixture := newChannelFixture(t)
	cfg := TickConfig{Now: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)}
	scanner := func(string, time.Time) ([]dispatch.StopRoute, error) {
		return []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 4, Budget: &breachBox}}, nil
	}
	// No lifecycle wired: the report is FAILED, and nothing was stopped.
	reports := custodialBreachStops(fixture.root, cfg, scanner)
	if len(reports) != 1 || reports[0].State != "FAILED" {
		t.Fatalf("fixture stop did not fail: %+v", reports)
	}
	if asked, _ := channel.WalkQuestions(fixture.root); len(asked) != 0 {
		t.Fatalf("a failed stop asked: %+v", asked)
	}
}

func TestBreachStopWithoutChannelKeepsTheReportOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stops := 0
	scanner := func(string, time.Time) ([]dispatch.StopRoute, error) {
		return []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 4, Budget: &breachBox}}, nil
	}
	reports := custodialBreachStops(root, breachFixtureConfig(&stops), scanner)
	if len(reports) != 1 || reports[0].State != "COMPLETE" {
		t.Fatalf("fixture stop did not complete: %+v", reports)
	}
	if asked, _ := channel.WalkQuestions(root); len(asked) != 0 {
		t.Fatalf("with no channel the stop asked: %+v", asked)
	}
	if _, err := os.Stat(alertDir(root)); !os.IsNotExist(err) {
		t.Fatalf("with no channel the stop opened an alert episode: %v", err)
	}
}

func TestBreachStopQuestionPostsThroughThePoll(t *testing.T) {
	t.Parallel()
	fixture := newUnservedChannelFixture(t)
	stops := 0
	scanner := func(string, time.Time) ([]dispatch.StopRoute, error) {
		return []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 4, Budget: &breachBox}}, nil
	}
	custodialBreachStops(fixture.root, breachFixtureConfig(&stops), scanner)
	asked, _ := channel.WalkOpenQuestions(fixture.root)
	if len(asked) != 1 || asked[0].Thread != nil {
		t.Fatalf("the asked question was posted by the steward: %+v", asked)
	}
	poll := func() {
		// The poll's later inbox read needs a ledger this bed lacks; only
		// its posting is read here.
		// The fake that does not serve yet has no provider: one that
		// cannot post stands in for it.
		loaded := loadSignalChannel(fixture.root).loaded
		provider := channel.Provider(downChannel{})
		if loaded.Provider != nil {
			provider = loaded.Provider
		}
		_, _ = channel.Poll(context.Background(), channel.PollConfig{RepoRoot: fixture.root, Destination: "fleet", ProviderName: "fake", Provider: provider, DestinationConfig: loaded.Destination, Now: breachFixtureConfig(&stops).Now})
	}
	poll()
	if q, _ := channel.ReadQuestion(fixture.root, asked[0].ID); q.Thread != nil || q.Undelivered != 1 {
		t.Fatalf("a failed post was not counted: %+v", q)
	}
	fixture.serve(t)
	poll()
	poll()
	if q, _ := channel.ReadQuestion(fixture.root, asked[0].ID); q.Thread == nil || len(fixture.posts(t)) != 1 {
		t.Fatalf("the poll did not post the question once: %+v %q", q, fixture.posts(t))
	}
}

// downChannel is a channel that cannot be reached.
type downChannel struct{}

func (downChannel) Post(context.Context, channel.DestinationConfig, string, *channel.MessageRef) (channel.MessageRef, error) {
	return channel.MessageRef{}, errors.New("the channel is down")
}
func (downChannel) Receive(context.Context, channel.DestinationConfig, []channel.MessageRef, channel.Cursor) ([]channel.Inbound, channel.Cursor, error) {
	return nil, "", errors.New("the channel is down")
}
func (downChannel) Confirm(context.Context, channel.DestinationConfig, channel.Cursor) error {
	return errors.New("the channel is down")
}
func (downChannel) Credential(context.Context, channel.DestinationConfig) (channel.CredentialIdentity, error) {
	return channel.CredentialIdentity{}, errors.New("the channel is down")
}
