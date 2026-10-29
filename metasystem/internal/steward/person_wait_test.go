package steward

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

type personWaitBed struct {
	t *testing.T
	*attentionPolicyBed
	seat    *seatBed
	channel channelFixture
}

// The poll publishes its inbox into a temporary Git repository. The tick's
// accepted ledger reads use the strict repository fixture.
func newPersonWaitBed(t *testing.T, continuation bool) *personWaitBed {
	t.Helper()
	publication := testgoal.NewPublicationFixture(t)
	publication.Git("config", "metasystem.goal.machine", "ui")
	bed := newAttentionPolicyBed(t)
	bed.root, bed.now = publication.Root, publication.Now
	file := seatClaimedGoal("held-goal", SeatLineage)
	bed.world("base", map[string]*goal.GoalFile{file.Id: file})
	fixture := channelFixture{root: bed.root, dir: t.TempDir()}
	conf := "metasystem.template=true\nmetasystem.runtimes=claude\nlaunch.seat.runtime=claude\nchannel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir=" + fixture.dir + "\nchannel.human.answer-code=off\n"
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ready, done := make(chan string, 1), make(chan error, 1)
	channelNow := bed.now
	go func() {
		done <- fake.ServeWithHooks(ctx, fixture.dir, fake.ServeHooks{Ready: ready, Now: func() time.Time { return channelNow }})
	}()
	select {
	case <-ready:
	case err := <-done:
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	testprovider.Register(t, bed.root)
	seat := &seatBed{t: t, root: bed.root, now: bed.now, goals: map[string]*goal.GoalFile{}, launcher: &fakeSeatLauncher{}, tips: map[string]string{}}
	seat.put(file)
	if continuation {
		nonce := endedContinuationFixture(t, bed.root, false)
		writeStewardRecord(t, filepath.Join(consumedDir(bed.root), nonce+".json"), map[string]any{"nonce": nonce, "goal": file.Id, "jobId": "steward-" + nonce, "reason": "seatHandoff"})
		decision := &decisionTickRepository{t: t, root: bed.root, head: strings.Repeat("a", 40), files: bed.worlds["base"]}
		if err := SaveEvidence(bed.root, EvidencePath(bed.root), Evidence{Marks: decision.currentMarks(), CurrentContinuation: nonce}); err != nil {
			t.Fatal(err)
		}
	}
	return &personWaitBed{t: t, attentionPolicyBed: bed, seat: seat, channel: fixture}
}

func (b *personWaitBed) pass() TickResult {
	b.t.Helper()
	f := newTickContinuationFixture(b.t, b.attentionPolicyBed, false)
	f.cfg.ProviderHome = testprovider.Home(b.root)
	f.census = fakeCensus{workers: deadWorkers}
	f.dependencies.openWork.Seat = b.seat.seatDependencies()
	f.dependencies.resolveMachine = func(string) (string, error) { return "ui", nil }
	result, completed, err := f.run(b.t)
	f.checkWork()
	if err != nil || !completed {
		b.t.Fatalf("tick: %+v completed=%t err=%v", result, completed, err)
	}
	b.now = b.now.Add(time.Minute)
	b.seat.now = b.now
	return result
}

func (b *personWaitBed) questions() []channel.Question {
	b.t.Helper()
	qs, unreadable := channel.WalkQuestions(b.root)
	if len(unreadable) > 0 {
		b.t.Fatal(unreadable)
	}
	return qs
}

func (b *personWaitBed) poll() {
	b.t.Helper()
	loaded := loadSignalChannel(b.root).loaded
	_, err := channel.Poll(context.Background(), channel.PollConfig{RepoRoot: b.root, Destination: "fleet", ProviderName: "fake", HumanUserID: "UWIDO", Machine: "ui", Lineage: "steward", Provider: loaded.Provider, DestinationConfig: loaded.Destination, Now: b.now})
	if err != nil {
		b.t.Fatal(err)
	}
}

func TestAHoldAsksOnceInTheContract(t *testing.T) {
	t.Parallel()
	b := newPersonWaitBed(t, true)
	result := b.pass()
	qs := b.questions()
	cause, _, _ := strings.Cut(strings.Split(result.Decision.Reason, "\n")[0], personRemedyMark)
	want := channel.Decision{Kind: "steward-hold", Subject: "held-goal", State: cause}
	if len(qs) != 1 || !qs[0].Contract || qs[0].Decision == nil || *qs[0].Decision != want || qs[0].Goal != "" || qs[0].Lineage != "" || qs[0].Wants != "" || qs[0].Recommendation != "Revive it" || len(qs[0].Options) != 2 || !reflect.DeepEqual(qs[0].Options[0].Act, []string{"metasystem", "machine", "revive", "ui"}) || qs[0].Options[1].Label != "Leave it" || len(qs[0].Options[1].Act) != 0 {
		t.Fatalf("hold contract: %+v", qs)
	}
	b.poll()
	if _, err := deliverPendingWithDependencies(b.root, pendingDeliveryDependencies{transport: func(string, string) error { t.Fatal("a verdict reached the provider"); return nil }, funnel: noticeFunnel{clock: func() time.Time { return b.now }}}); err != nil {
		t.Fatal(err)
	}
	b.pass()
	b.poll()
	posts := b.channel.posts(t)
	if len(b.questions()) != 1 || len(posts) != 1 || !strings.Contains(posts[0], "★ Revive it") || !strings.Contains(posts[0], "Reply:") {
		t.Fatalf("hold posted more than once or without the contract: %q", posts)
	}
}

func TestAHoldIsAskedOnceAfterAChoice(t *testing.T) {
	t.Parallel()
	b := newPersonWaitBed(t, true)
	result := b.pass()
	b.poll()
	q := b.questions()[0]
	row, _ := json.Marshal(map[string]any{"thread_ts": q.Thread.ThreadID, "user": "UWIDO", "text": "leave it"})
	if err := os.WriteFile(filepath.Join(b.channel.dir, "replies.jsonl"), append(row, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	b.poll()
	cause, _, _ := strings.Cut(strings.Split(result.Decision.Reason, "\n")[0], personRemedyMark)
	want := channel.Decision{Kind: "steward-hold", Subject: "held-goal", State: cause}
	check := func() {
		q = b.questions()[0]
		if q.Decision == nil || *q.Decision != want || q.State != "open" || len(q.Attempts) != 1 || q.Attempts[0].Label != "Leave it" || q.Attempts[0].Outcome != "chosen" || !q.RetiredAt.IsZero() {
			t.Fatalf("choice lost its decision or open question: %+v", q)
		}
	}
	check()
	b.pass()
	b.poll()
	check()
	posts := b.channel.posts(t)
	if len(b.questions()) != 1 || len(posts) != 2 || !strings.Contains(posts[1], "Nothing runs") {
		t.Fatalf("leave it asked again: %q", posts)
	}
}

func TestAHoldQuestionRetiresWhenTheHoldClears(t *testing.T) {
	t.Parallel()
	b := newPersonWaitBed(t, true)
	b.pass()
	first := b.questions()[0]
	work, err := b.seat.dependencies().ReadClaimableBudgetedWork(b.root, b.now)
	if err != nil {
		t.Fatal(err)
	}
	ev, err := LoadEvidence(EvidencePath(b.root))
	if err != nil {
		t.Fatal(err)
	}
	for _, verdict := range []Verdict{VerdictDegraded, VerdictUnknown} {
		if err := askPersonWaits(b.root, "ui", Decision{Verdict: verdict}, Evidence{}, work, b.now); err != nil {
			t.Fatal(err)
		}
		if q, _ := channel.ReadQuestion(b.root, first.ID); !q.RetiredAt.IsZero() {
			t.Fatal("uncertain tick retired the hold")
		}
	}
	if _, err := testprovider.Record(b.root, "overloaded", "API Error: 529", "test", b.now); err != nil {
		t.Fatal(err)
	}
	b.pass()
	if q, _ := channel.ReadQuestion(b.root, first.ID); !q.RetiredAt.IsZero() || !reflect.DeepEqual(q.Decision, first.Decision) {
		t.Fatal("provider wait retired the underlying hold")
	}
	if err := outage.Clear(testprovider.Home(b.root), "anthropic", b.now); err != nil {
		t.Fatal(err)
	}
	b.pass()
	if len(b.questions()) != 1 {
		t.Fatal("the same hold asked after the provider recovered")
	}
	if err := SaveEvidence(b.root, EvidencePath(b.root), Evidence{}); err != nil {
		t.Fatal(err)
	}
	b.pass()
	if q, _ := channel.ReadQuestion(b.root, first.ID); q.RetiredAt.IsZero() || q.State != "closed" {
		t.Fatal("cleared hold was not retired")
	}
	if err := SaveEvidence(b.root, EvidencePath(b.root), ev); err != nil {
		t.Fatal(err)
	}
	b.pass()
	if len(b.questions()) != 2 {
		t.Fatal("a later hold did not ask anew")
	}
}

func TestTheRestartCapAsksForApproval(t *testing.T) {
	t.Parallel()
	b := newPersonWaitBed(t, false)
	capSeats(t, b.seat, "held-goal", 3)
	b.pass()
	b.poll()
	qs := b.questions()
	if len(qs) != 1 || !qs[0].Contract || qs[0].Decision == nil || *qs[0].Decision != (channel.Decision{Kind: "steward-hold", Subject: "held-goal", State: "restart cap"}) || qs[0].Recommendation != "Approve it again" || !reflect.DeepEqual(qs[0].Options[0].Act, []string{"metasystem", "goal", "approve", "held-goal"}) {
		t.Fatalf("cap contract: %+v", qs)
	}
	first := qs[0]
	b.pass()
	if _, err := testprovider.Record(b.root, "overloaded", "API Error: 529", "test", b.now); err != nil {
		t.Fatal(err)
	}
	b.pass()
	b.poll()
	if q, _ := channel.ReadQuestion(b.root, first.ID); !q.RetiredAt.IsZero() || !reflect.DeepEqual(q.Decision, first.Decision) || len(b.questions()) != 1 || len(b.channel.posts(t)) != 1 {
		t.Fatal("provider wait retired or repeated the cap")
	}
	if err := outage.Clear(testprovider.Home(b.root), "anthropic", b.now); err != nil {
		t.Fatal(err)
	}
	b.pass()
	b.poll()
	if q, _ := channel.ReadQuestion(b.root, first.ID); !q.RetiredAt.IsZero() || !reflect.DeepEqual(q.Decision, first.Decision) || len(b.questions()) != 1 || len(b.channel.posts(t)) != 1 {
		t.Fatal("the same cap asked again after provider recovery")
	}
	b.seat.goals["held-goal"].Approved.Opid = goal.Opid("01J5X000000000000000000002", seatBedMachine, "steward")
	file := b.seat.goals["held-goal"]
	file.History[file.Approved.Revision-1].Opid = file.Approved.Opid
	b.world("base", map[string]*goal.GoalFile{file.Id: file})
	if err := outage.Clear(testprovider.Home(b.root), "anthropic", b.now); err != nil {
		t.Fatal(err)
	}
	b.pass()
	if q, _ := channel.ReadQuestion(b.root, first.ID); q.RetiredAt.IsZero() {
		t.Fatal("fresh approval did not retire the cap")
	}
}

func TestTheHoldNamesItsGoal(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"continuation", "seat", "cap", "fallback"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			root, now := t.TempDir(), time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
			ev := Evidence{}
			d := Decision{Action: ActNotify, Reason: "the seat is held" + personRemedyMark + "metasystem machine revive ui"}
			want := "held-goal"
			switch source {
			case "continuation":
				ev.CurrentContinuation = "held"
				writeStewardRecord(t, filepath.Join(consumedDir(root), "held.json"), map[string]any{"nonce": "held", "goal": want})
			case "seat":
				ev.CurrentSeat = "matching"
				for _, r := range []SeatRecord{{LaunchID: "matching", Goal: want, StartedAt: "2026-10-14T12:00:00Z"}, {LaunchID: "later", Goal: "other-goal", StartedAt: "2026-10-15T12:00:00Z"}} {
					if err := writeSeatRecord(root, r); err != nil {
						t.Fatal(err)
					}
				}
			case "cap":
				d = seatCapped(VerdictStalledDead, want, 3)
			case "fallback":
				want = "seat:ui"
			}
			if err := askPersonWaits(root, "ui", d, ev, goal.ClaimableBudgetedWork{}, now); err != nil {
				t.Fatal(err)
			}
			qs, _ := channel.WalkQuestions(root)
			if len(qs) != 1 || qs[0].Goal != "" || qs[0].Decision == nil || qs[0].Decision.Subject != want {
				t.Fatalf("%s subject: %+v", source, qs)
			}
		})
	}
}

func TestAPersonQuestionFailureIsToldNotFatal(t *testing.T) {
	t.Parallel()
	b := newPersonWaitBed(t, false)
	file := seatReadyGoal("held-goal", "Build it.")
	b.seat.goals[file.Id] = file
	b.world("base", map[string]*goal.GoalFile{file.Id: file})
	capSeats(t, b.seat, "held-goal", 3)
	writeStewardRecord(t, filepath.Join(b.root, "artifacts", "agents", "channel", "questions", "broken.json"), map[string]any{"invalid": true})
	result := b.pass()
	if result.Decision.Action != ActNotify {
		t.Fatalf("hold did not notify: %+v", result)
	}
	pending, err := PendingNotifications(b.root)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) < 2 || pending[0].Nonce != "person-question" || !strings.HasPrefix(pending[1].Nonce, "verdict-") {
		t.Fatalf("fail-safe order: %+v", pending)
	}
	if _, err := deliverPendingWithDependencies(b.root, pendingDeliveryDependencies{transport: func(string, string) error { t.Fatal("withheld notice was sent"); return nil }, funnel: noticeFunnel{clock: func() time.Time { return b.now }}}); err != nil {
		t.Fatal(err)
	}
	journal, err := os.ReadFile(filepath.Join(runnerDir(canonicalPath(b.root)), NotificationJournalName))
	if err != nil || !strings.Contains(string(journal), `"withheld":"unknown"`) || !strings.Contains(string(journal), `"withheld":"verdict"`) {
		t.Fatalf("fail-safe not journaled withheld: %s %v", journal, err)
	}
}
