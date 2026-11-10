package channel_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// The one-bot bed of Decision 8: two installations (a seat and a lane) on
// one computer, each a clone of one shared goal ledger, both polling one
// fake Telegram bot with one confirmed offset.

const inboxSecret = "JBSWY3DPEHPK3PXP"
const inboxHuman = "7001"

type inboxInstallation struct {
	name, root, machine string
	provider            channel.Provider
	dest                channel.DestinationConfig
}

type inboxBed struct {
	origin, fakeDir string
	seat, lane      inboxInstallation
	now             time.Time
	parked          chan fake.WaitPoint
	replies         int
}

func inboxGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "protocol.file.allow=always"}, args...)...)
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			env = append(env, entry)
		}
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func inboxGoalFile(id string) []byte {
	opened := goal.HistoryLine{At: "2026-09-03T00:00:00Z", Opid: goal.Opid("01J5X0000000000000000000F0", "seed", "seed"), Verb: "open", Actor: "seed+seed", Targets: []string{id}, Keep: -1}
	return goal.RenderFile(&goal.GoalFile{Id: id, State: goal.StateQueued, Tier: 1, Intent: "Question target.", Origin: "main", NextStep: "Wait.", OpenedAt: "2026-09-03T00:00:00Z", Revision: 1, History: []goal.HistoryLine{opened}})
}

// inboxSetup adds configuration lines to the shared metasystem.conf and,
// when now is set, fixes the bed's and the fake bot's clock.
type inboxSetup struct {
	conf string
	now  time.Time
}

func newInboxBed(t *testing.T, setups ...inboxSetup) *inboxBed {
	t.Helper()
	var setup inboxSetup
	if len(setups) > 0 {
		setup = setups[0]
	}
	base := t.TempDir()
	bed := &inboxBed{origin: filepath.Join(base, "origin.git"), now: time.Now().UTC().Truncate(time.Second), parked: make(chan fake.WaitPoint, 16)}
	hooks := fake.ServeHooks{Parked: bed.parked}
	if !setup.now.IsZero() {
		bed.now = setup.now
		hooks.Now = func() time.Time { return setup.now }
	}
	inboxGit(t, base, "init", "-q", "--bare", "-b", "main", bed.origin)
	seed := filepath.Join(base, "seed")
	inboxGit(t, base, "clone", "-q", bed.origin, seed)
	root := &goal.RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1", SyncMode: goal.SyncRemote,
		MigrationEpoch: "2026-08-20T00:00:00Z", ManifestDigest: strings.Repeat("ab", 32), MigrationMode: "manifest", Revision: 1,
		History: []goal.HistoryLine{{At: "2026-08-20T09:00:00Z", Opid: "01J5X0000000000000000000A0-mac-a-1a2b3c4d", Verb: "migrate", Actor: "mac-a+lin-1", Keep: -1}},
	}
	files := map[string][]byte{
		"plans/goals/backlog.md":   goal.RenderRoot(root),
		"plans/goals/seat-goal.md": inboxGoalFile("seat-goal"),
		"plans/goals/lane-goal.md": inboxGoalFile("lane-goal"),
		"metasystem.conf":          []byte("metasystem.runtimes=fake\nmetasystem.budget.tier-1=1h/3/360m/1/0\nmetasystem.budget.review-round-max=3\n" + setup.conf),
	}
	for path, body := range files {
		full := filepath.Join(seed, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inboxGit(t, seed, "add", ".")
	inboxGit(t, seed, "commit", "-qm", "seed shared ledger")
	inboxGit(t, seed, "push", "-q", "origin", "main")

	bed.fakeDir = filepath.Join(base, "bot")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	ready := make(chan string, 1)
	hooks.Ready = ready
	go func() {
		done <- fake.ServeWithHooks(ctx, bed.fakeDir, hooks)
	}()
	select {
	case <-ready:
	case err := <-done:
		cancel()
		t.Fatalf("fake bot did not start: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	for _, inst := range []*inboxInstallation{{name: "seat", machine: "m1e"}, {name: "lane", machine: "landing"}} {
		inst.root = filepath.Join(base, inst.name)
		inboxGit(t, base, "clone", "-q", bed.origin, inst.root)
		inboxGit(t, inst.root, "config", "metasystem.goal.machine", inst.machine)
		inboxGit(t, inst.root, "config", "goal.sync-remote", "origin")
		inboxGit(t, inst.root, "config", "goal.sync-branch", "refs/heads/main")
		inboxGit(t, inst.root, "update-ref", goal.AcceptedRef, "origin/main")
		p, d, err := fake.TelegramProvider(bed.fakeDir, inst.name)
		if err != nil {
			t.Fatal(err)
		}
		inst.provider, inst.dest = p, d
		if inst.name == "seat" {
			bed.seat = *inst
		} else {
			bed.lane = *inst
		}
	}
	return bed
}

func (b *inboxBed) config(inst inboxInstallation) channel.PollConfig {
	return channel.PollConfig{RepoRoot: inst.root, Destination: "fleet", ProviderName: "telegram", HumanUserID: inboxHuman, TOTPSecret: inboxSecret,
		Machine: inst.machine, Lineage: "steward", Provider: inst.provider, DestinationConfig: inst.dest, Now: b.now}
}

func (b *inboxBed) poll(t *testing.T, inst inboxInstallation) channel.PollResult {
	t.Helper()
	r, err := channel.Poll(context.Background(), b.config(inst))
	if err != nil {
		t.Fatalf("%s poll: %v", inst.name, err)
	}
	return r
}

func (b *inboxBed) ask(t *testing.T, inst inboxInstallation, goalID, fact string) channel.Question {
	t.Helper()
	q, err := channel.Ask(channel.AskRequest{RepoRoot: inst.root, Goal: goalID, Kind: "other", Machine: inst.machine, Lineage: inst.name, Facts: []string{fact}, Now: b.now.Add(-10 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func (b *inboxBed) question(t *testing.T, inst inboxInstallation, id string) channel.Question {
	t.Helper()
	q, err := channel.ReadQuestion(inst.root, id)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

// reply has the human send text in reply to message replyTo at sentAt.
func (b *inboxBed) reply(t *testing.T, replyTo, text string, sentAt time.Time) {
	t.Helper()
	var parent int64
	if _, err := fmt.Sscanf(replyTo, "%d", &parent); err != nil {
		t.Fatal(err)
	}
	date := sentAt.Unix()
	row, err := json.Marshal(map[string]any{"face": "telegram", "user": 7001, "text": text, "reply_to": parent, "date": date})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(b.fakeDir, "replies.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(append(row, '\n')); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	b.replies++
}

func code(t *testing.T, at time.Time) string {
	t.Helper()
	c, err := channel.TOTPCode(inboxSecret, at)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type inboxRecord struct {
	Path        string
	Provider    string  `json:"provider"`
	Destination string  `json:"destination"`
	MessageID   string  `json:"messageId"`
	ReplyTo     *string `json:"replyTo"`
	UserID      string  `json:"userId"`
	Text        string  `json:"text"`
	Step        *int64  `json:"step"`
	Outcome     string  `json:"outcome"`
	Question    string  `json:"question"`
	Opid        string  `json:"opid"`
	ReceivedBy  string  `json:"receivedBy"`
}

func (b *inboxBed) inbox(t *testing.T) []inboxRecord {
	t.Helper()
	listing := inboxGit(t, b.origin, "ls-tree", "-r", "--name-only", "main", "--", "plans/channel/inbox/")
	var out []inboxRecord
	for _, path := range strings.Fields(listing) {
		var r inboxRecord
		if err := json.Unmarshal([]byte(inboxGit(t, b.origin, "show", "main:"+path)), &r); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		r.Path = path
		out = append(out, r)
	}
	return out
}

func (b *inboxBed) recordFor(t *testing.T, text string) inboxRecord {
	t.Helper()
	var found []inboxRecord
	for _, r := range b.inbox(t) {
		if r.Text == text {
			found = append(found, r)
		}
	}
	if len(found) != 1 {
		t.Fatalf("inbox records with text %q: %+v (whole inbox %+v)", text, found, b.inbox(t))
	}
	return found[0]
}

type journalRow struct {
	Method   string         `json:"method"`
	Listener string         `json:"listener"`
	Form     map[string]any `json:"form"`
}

func (b *inboxBed) journal(t *testing.T) []journalRow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(b.fakeDir, "journal.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []journalRow
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var row journalRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

func (b *inboxBed) sent(t *testing.T) []journalRow {
	t.Helper()
	var out []journalRow
	for _, row := range b.journal(t) {
		if row.Method == "sendMessage" {
			out = append(out, row)
		}
	}
	return out
}

func (b *inboxBed) confirms(t *testing.T, listener string) int {
	t.Helper()
	n := 0
	for _, row := range b.journal(t) {
		if row.Method == "confirm" && row.Listener == listener {
			n++
		}
	}
	return n
}

func (b *inboxBed) control(t *testing.T, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(b.fakeDir, "control.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func replyParent(row journalRow) string {
	parameters, _ := row.Form["reply_parameters"].(map[string]any)
	id, _ := parameters["message_id"].(float64)
	if id == 0 {
		return ""
	}
	return fmt.Sprintf("%d", int64(id))
}

func notices(rows []journalRow) []journalRow {
	var out []journalRow
	for _, row := range rows {
		if text, _ := row.Form["text"].(string); strings.HasPrefix(text, "not recorded:") {
			out = append(out, row)
		}
	}
	return out
}

func TestOneBotTwoInstallationsCommitToTheLedgerInbox(t *testing.T) {
	t.Parallel()
	bed := newInboxBed(t)
	seatQ := bed.ask(t, bed.seat, "seat-goal", "Seat needs a decision")
	laneQ := bed.ask(t, bed.lane, "lane-goal", "Lane needs a decision")

	// Each installation posts its own question; nothing is pending yet.
	bed.poll(t, bed.seat)
	bed.poll(t, bed.lane)
	seatThread := bed.question(t, bed.seat, seatQ.ID).Thread
	laneThread := bed.question(t, bed.lane, laneQ.ID).Thread
	if seatThread == nil || laneThread == nil {
		t.Fatalf("questions were not posted: seat=%v lane=%v", seatThread, laneThread)
	}

	// The human answers both questions; both stewards poll at once and the
	// bot answers the lane's poll with a 409.
	sentA := bed.now.Add(-120 * time.Second)
	sentB := bed.now.Add(-90 * time.Second)
	bed.reply(t, laneThread.ID, "lane go ahead "+code(t, sentA), sentA)
	bed.reply(t, seatThread.ID, "seat go ahead "+code(t, sentB), sentB)
	bed.control(t, map[string]any{"conflict": []map[string]any{{"listener": "lane", "remaining": 1, "description": "Conflict: terminated by other getUpdates request; make sure that only one bot instance is running"}}})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, inst := range []inboxInstallation{bed.seat, bed.lane} {
		wg.Add(1)
		go func(i int, inst inboxInstallation) {
			defer wg.Done()
			_, errs[i] = channel.Poll(context.Background(), bed.config(inst))
		}(i, inst)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("a 409 failed a poll: seat=%v lane=%v", errs[0], errs[1])
	}
	bed.control(t, map[string]any{})
	// The lane polls again and finds its answer in the ledger inbox.
	bed.poll(t, bed.lane)
	bed.poll(t, bed.seat)
	if got := bed.question(t, bed.seat, seatQ.ID); got.State != "closed" || got.Answer == nil || got.Answer.Text != "seat go ahead" {
		t.Fatalf("the seat's reply was not recorded in the seat: %+v", got)
	}
	if got := bed.question(t, bed.lane, laneQ.ID); got.State != "closed" || got.Answer == nil || got.Answer.Text != "lane go ahead" {
		t.Fatalf("the lane's reply was not recorded in the lane: %+v", got)
	}
	recordA, recordB := bed.recordFor(t, "lane go ahead"), bed.recordFor(t, "seat go ahead")
	for _, r := range []inboxRecord{recordA, recordB} {
		if r.Outcome != "verified" || r.Step == nil || r.Question != "unmatched" || r.UserID != inboxHuman || r.Destination != "fleet" || r.Provider != "telegram" ||
			r.Path != "plans/channel/inbox/fleet/telegram-"+r.MessageID+".json" {
			t.Fatalf("inbox record: %+v", r)
		}
	}
	if recordA.ReplyTo == nil || *recordA.ReplyTo != laneThread.ID || recordB.ReplyTo == nil || *recordB.ReplyTo != seatThread.ID {
		t.Fatalf("inbox records lost their thread: %+v %+v", recordA, recordB)
	}
	if posts := bed.sent(t); len(posts) != 2 {
		t.Fatalf("an answered question posted a receipt or a notice: %+v", posts)
	}

	// A second seat question; the human reuses an earlier code on a new
	// message. The seat commits it and pauses before its Confirm; the lane
	// receives the same unconfirmed update, loses the commit race and confirms.
	seatQ2 := bed.ask(t, bed.seat, "seat-goal", "Seat needs a second decision")
	bed.poll(t, bed.seat)
	seat2Thread := bed.question(t, bed.seat, seatQ2.ID).Thread
	if seat2Thread == nil {
		t.Fatal("second seat question was not posted")
	}
	bed.reply(t, seat2Thread.ID, "reused code "+code(t, sentA), sentA)
	release := filepath.Join(t.TempDir(), "release-seat-confirm")
	bed.control(t, map[string]any{"pauseBefore": []map[string]any{{"listener": "seat", "method": "confirm", "until": release}}})
	seatDone := make(chan error, 1)
	go func() {
		_, err := channel.Poll(context.Background(), bed.config(bed.seat))
		seatDone <- err
	}()
	select {
	case point := <-bed.parked:
		if point != fake.PauseWait {
			t.Fatalf("seat parked at %q", point)
		}
	case err := <-seatDone:
		t.Fatalf("the seat finished before confirming: %v", err)
	}
	laneConfirms := bed.confirms(t, "lane")
	bed.poll(t, bed.lane)
	if bed.confirms(t, "lane") <= laneConfirms {
		t.Fatal("the loser of the commit race did not confirm")
	}
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := <-seatDone; err != nil {
		t.Fatalf("seat poll: %v", err)
	}
	bed.control(t, map[string]any{})
	replayed := bed.recordFor(t, "reused code")
	if replayed.Outcome != "replayed" || replayed.Step == nil || *replayed.Step != *recordA.Step || replayed.ReceivedBy != "m1e" {
		t.Fatalf("a reused code was not committed replayed by the winner: %+v", replayed)
	}
	replayNotices := notices(bed.sent(t))
	if len(replayNotices) != 1 || replayNotices[0].Listener != "seat" || replyParent(replayNotices[0]) != seat2Thread.ID ||
		replayNotices[0].Form["text"] != "not recorded: replayed code. Reply to the question above with your answer and your code" {
		t.Fatalf("replay notice: %+v", replayNotices)
	}

	// A bad code to the seat's question is received first by the lane, which
	// is killed after its commit and before Confirm. The update comes again,
	// loses to the lane's own record, and is noticed exactly once, by the lane.
	sentD := bed.now.Add(-60 * time.Second)
	near := code(t, sentD.Add(-30*time.Second)) + code(t, sentD) + code(t, sentD.Add(30*time.Second))
	bad := ""
	for _, candidate := range []string{"000000", "111111", "222222", "333333"} {
		if bad == "" && !strings.Contains(near, candidate) {
			bad = candidate
		}
	}
	bed.reply(t, seat2Thread.ID, "bad try "+bad, sentD)
	killed := bed.config(bed.lane)
	killed.FailurePoint = func(point string) error {
		if point == "inbox-committed" {
			return errors.New("lane killed after its commit")
		}
		return nil
	}
	if _, err := channel.Poll(context.Background(), killed); err == nil {
		t.Fatal("the lane was not killed after its commit")
	}
	badRecord := bed.recordFor(t, "bad try")
	if badRecord.Outcome != "bad-code" || badRecord.Step != nil || badRecord.ReceivedBy != "landing" {
		t.Fatalf("bad code record: %+v", badRecord)
	}
	bed.poll(t, bed.lane)
	bed.recordFor(t, "bad try")
	bed.poll(t, bed.seat)
	if got := bed.question(t, bed.seat, seatQ2.ID); got.State != "open" || got.Answer != nil {
		t.Fatalf("a rejected reply touched the seat's question: %+v", got)
	}
	badNotices := notices(bed.sent(t))[1:]
	if len(badNotices) != 1 || badNotices[0].Listener != "lane" || replyParent(badNotices[0]) != seat2Thread.ID ||
		badNotices[0].Form["text"] != "not recorded: bad code. Reply to the question above with your answer and your code" {
		t.Fatalf("bad code notice: %+v", badNotices)
	}

	// The human replies to the original question with a good code; the lane
	// receives it first, and the seat matches it from the inbox.
	sentE := bed.now.Add(-30 * time.Second)
	bed.reply(t, seat2Thread.ID, "second answer "+code(t, sentE), sentE)
	bed.poll(t, bed.lane)
	if got := bed.question(t, bed.seat, seatQ2.ID); got.State != "open" {
		t.Fatalf("the seat's question changed before the seat polled: %+v", got)
	}
	bed.poll(t, bed.seat)
	got := bed.question(t, bed.seat, seatQ2.ID)
	if got.State != "closed" || got.Answer == nil || got.Answer.Text != "second answer" {
		t.Fatalf("the corrected reply was not matched by the seat: %+v", got)
	}
	if rec := bed.recordFor(t, "second answer"); rec.ReceivedBy != "landing" || rec.Outcome != "verified" {
		t.Fatalf("corrected reply record: %+v", rec)
	}
	if n := len(bed.inbox(t)); n != bed.replies {
		t.Fatalf("inbox holds %d records for %d replies", n, bed.replies)
	}
	if posts := bed.sent(t); len(posts) != 5 {
		t.Fatalf("posts: three questions and two notices, no receipts; got %+v", posts)
	}
}

// R2-1: the seat commits a rejected reply and is killed before posting its
// notice; the lane receives the redelivery, loses to the seat's record and
// posts nothing; the seat's queued notice is posted on its next poll.
func TestKilledCommitterStillNoticesWhenTheOtherInstallationConfirms(t *testing.T) {
	t.Parallel()
	bed := newInboxBed(t)
	q := bed.ask(t, bed.seat, "seat-goal", "Seat needs a decision")
	bed.poll(t, bed.seat)
	thread := bed.question(t, bed.seat, q.ID).Thread
	if thread == nil {
		t.Fatal("question was not posted")
	}
	bed.reply(t, thread.ID, "no code here", bed.now.Add(-30*time.Second))
	killed := bed.config(bed.seat)
	killed.FailurePoint = func(point string) error {
		if point == "inbox-published" {
			return errors.New("seat killed after its commit")
		}
		return nil
	}
	if _, err := channel.Poll(context.Background(), killed); err == nil {
		t.Fatal("the seat was not killed after its commit")
	}
	if rec := bed.recordFor(t, "no code here"); rec.Outcome != "no-code" || rec.ReceivedBy != "m1e" {
		t.Fatalf("record=%+v", rec)
	}
	bed.poll(t, bed.lane)
	if got := notices(bed.sent(t)); len(got) != 0 {
		t.Fatalf("the losing lane posted: %+v", got)
	}
	bed.poll(t, bed.seat)
	bed.poll(t, bed.seat)
	got := notices(bed.sent(t))
	if len(got) != 1 || got[0].Listener != "seat" || replyParent(got[0]) != thread.ID {
		t.Fatalf("notices=%+v", got)
	}
	if n := len(bed.inbox(t)); n != 1 {
		t.Fatalf("inbox holds %d records", n)
	}
}

func config(b *inboxBed, inst inboxInstallation, at time.Time) channel.PollConfig {
	c := b.config(inst)
	c.Now = at
	c.HostSeats = func() ([]string, error) { return []string{b.seat.root, b.lane.root}, nil }
	return c
}
func tick(t *testing.T, b *inboxBed, inst inboxInstallation, at time.Time) {
	t.Helper()
	c := config(b, inst, at)
	if _, err := channel.SettleReplies(context.Background(), c, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.Poll(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}
func unmatched(t *testing.T, b *inboxBed) []journalRow {
	t.Helper()
	var rows []journalRow
	for _, row := range b.sent(t) {
		if text, _ := row.Form["text"].(string); strings.HasPrefix(text, "I could not match") {
			rows = append(rows, row)
		}
	}
	return rows
}
func handled(t *testing.T, inst inboxInstallation, record string) bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(inst.root, "artifacts/agents/channel/fleet/inbox-handled.json"))
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	var paths []string
	if err != nil || json.Unmarshal(data, &paths) != nil {
		t.Fatalf("handled replies: %s %v", data, err)
	}
	return slices.Contains(paths, record)
}

func TestAReplyNoSeatTookIsToldOnce(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 10, 14, 9, 13, 0, 0, time.Local)
	waiting := func(t *testing.T, b *inboxBed) []struct{ Record string } {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(b.seat.root, "artifacts/agents/channel/fleet/notices.json"))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		var ledger struct{ Awaiting []struct{ Record string } }
		if err != nil || json.Unmarshal(data, &ledger) != nil {
			t.Fatalf("waiting replies: %s %v", data, err)
		}
		return ledger.Awaiting
	}
	newBed := func(t *testing.T) *inboxBed {
		t.Helper()
		return newInboxBed(t, inboxSetup{now: t0, conf: "channel.human.answer-code=off\n"})
	}
	t.Run("another seat matched it", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		q := b.ask(t, b.lane, "lane-goal", "Lane needs a decision")
		tick(t, b, b.lane, t0.Add(-5*time.Minute))
		q = b.question(t, b.lane, q.ID)
		b.reply(t, q.Thread.ID, "yes", t0)
		tick(t, b, b.seat, t0)
		tick(t, b, b.seat, t0.Add(time.Minute))
		tick(t, b, b.lane, t0.Add(2*time.Minute))
		tick(t, b, b.seat, t0.Add(16*time.Minute))
		tick(t, b, b.seat, t0.Add(40*time.Minute))
		q = b.question(t, b.lane, q.ID)
		if q.State != "closed" || q.Answer == nil || q.Answer.Text != "yes" || !handled(t, b.lane, b.recordFor(t, "yes").Path) || len(unmatched(t, b)) != 0 {
			t.Fatalf("answer: %+v; notices: %+v", q, unmatched(t, b))
		}
	})
	t.Run("an action request keeps it", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		stopPath := filepath.Join(plain.Dir(b.lane.root), "stops.jsonl")
		if err := os.MkdirAll(filepath.Dir(stopPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stopPath, []byte(`{"loop":"lane-return","subject":"lane","attempt":2,"decision":"stop","handoff":"ask lane","at":"2026-10-14T08:00:00Z","tree":"candidate","proof-attempt":"attempt"}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := plain.SyncPolicyQuestion(b.lane.root, func(string) (string, error) { return b.lane.machine, nil }, t0.Add(-10*time.Minute)); err != nil {
			t.Fatal(err)
		}
		questions, unreadable := channel.WalkOpenQuestions(b.lane.root)
		if len(questions) != 1 || len(unreadable) != 0 || channel.LaneStopCommand(questions[0]) == "" {
			t.Fatalf("landing request: %+v %v", questions, unreadable)
		}
		q := questions[0]
		tick(t, b, b.lane, t0.Add(-5*time.Minute))
		q = b.question(t, b.lane, q.ID)
		snapshot := func() map[string]string {
			files := map[string]string{}
			if err := filepath.WalkDir(plain.Dir(b.lane.root), func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() {
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					files[path] = string(data)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			return files
		}
		before := snapshot()
		b.reply(t, q.Thread.ID, "yes", t0)
		tick(t, b, b.seat, t0)
		tick(t, b, b.lane, t0.Add(2*time.Minute))
		tick(t, b, b.seat, t0.Add(16*time.Minute))
		q = b.question(t, b.lane, q.ID)
		if q.State != "open" || q.Answer == nil || q.Answer.Phase != "recorded" || q.Answer.Text != "yes" || len(unmatched(t, b)) != 0 || !reflect.DeepEqual(before, snapshot()) {
			t.Fatalf("request: %+v; notices: %+v; landing files before=%v after=%v", q, unmatched(t, b), before, snapshot())
		}
	})
	t.Run("nobody matched it", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		b.ask(t, b.seat, "seat-goal", "Seat needs a decision")
		q, err := channel.Ask(channel.AskRequest{RepoRoot: b.lane.root, About: "lane", Kind: "other", Machine: b.lane.machine, Lineage: b.lane.name, Facts: []string{"Lane needs a decision"}, Now: t0.Add(-12 * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		tick(t, b, b.seat, t0.Add(-5*time.Minute))
		tick(t, b, b.lane, t0.Add(-5*time.Minute))
		q = b.question(t, b.lane, q.ID)
		b.reply(t, q.Thread.ID, "yes", t0)
		tick(t, b, b.seat, t0)
		tick(t, b, b.seat, t0.Add(14*time.Minute))
		if got := unmatched(t, b); len(got) != 0 {
			t.Fatalf("notice before the wait: %+v", got)
		}
		tick(t, b, b.seat, t0.Add(16*time.Minute))
		if got := unmatched(t, b); len(got) != 1 {
			t.Fatalf("notice was not sent in the due pass: %+v", got)
		}
		tick(t, b, b.seat, t0.Add(40*time.Minute))
		got := unmatched(t, b)
		want := "I could not match your reply 'yes' (09:13) to an open question.\nThe questions open now:\n- the landing lane, asked 09:01\n- goal seat goal, asked 09:03\nTo answer one, reply in its own thread."
		if len(got) != 1 || got[0].Listener != "seat" || replyParent(got[0]) != q.Thread.ID || got[0].Form["text"] != want {
			t.Fatalf("notice: %+v; want %q", got, want)
		}
		for _, word := range []string{"m1e", "on landing", "unmatched", "verified", "inbox", "plans/"} {
			if strings.Contains(got[0].Form["text"].(string), word) {
				t.Fatalf("notice contains %q", word)
			}
		}
	})
	t.Run("an older reply is never told", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		q := b.ask(t, b.lane, "lane-goal", "Lane needs a decision")
		tick(t, b, b.lane, t0.Add(-65*time.Minute))
		q = b.question(t, b.lane, q.ID)
		ulid, err := goal.NewOperationULID()
		if err != nil {
			t.Fatal(err)
		}
		opid, step := goal.Opid(ulid, "m1e", "steward"), int64(42)
		in := &goal.ChannelInbound{Provider: "telegram", Destination: "fleet", MessageID: "old-reply", ReplyTo: &q.Thread.ID, UserID: inboxHuman, SentAt: t0.Add(-time.Hour).UTC().Format(time.RFC3339), Text: "yes", Step: &step, Outcome: "verified", Question: "unmatched", Opid: opid, ReceivedBy: "m1e", ReceivedAt: t0.Add(-time.Hour).UTC().Format(time.RFC3339)}
		data, err := goal.RenderChannelInbound(in)
		if err != nil {
			t.Fatal(err)
		}
		ep, err := project.PublicationEndpoint(b.seat.root)
		if err != nil {
			t.Fatal(err)
		}
		result, err := goal.Publish(ep, goal.PublishRequest{Opid: opid, Machine: "m1e", Lineage: "steward", Intent: goal.Intent{Verb: "inbox"}, Message: "channel inbox telegram-old-reply", Mutate: func(string) ([]goal.Change, error) {
			return []goal.Change{{Path: goal.ChannelInboxPath("fleet", in), Content: data}}, nil
		}})
		if err != nil || result.Outcome != goal.OutcomeConfirmed {
			t.Fatalf("old reply: %+v %v", result, err)
		}
		q.State, q.ClosedAt = "closed", t0.Add(-time.Hour)
		q.Answer = &channel.Answer{Text: "yes", Phase: "closed", At: t0.Add(-time.Hour), Ref: channel.MessageRef{ID: in.MessageID, ThreadID: q.Thread.ID}}
		data, err = json.Marshal(q)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(b.lane.root, "artifacts/agents/channel/questions", q.ID+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, at := range []time.Time{t0, t0.Add(16 * time.Minute), t0.Add(2 * time.Hour)} {
			tick(t, b, b.seat, at)
		}
		if len(unmatched(t, b)) != 0 || len(waiting(t, b)) != 0 || handled(t, b.lane, goal.ChannelInboxPath("fleet", in)) || handled(t, b.seat, goal.ChannelInboxPath("fleet", in)) {
			t.Fatalf("old reply was noticed or marked: %+v %+v", unmatched(t, b), waiting(t, b))
		}
	})
	t.Run("a status owner at the helm keeps its reply", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		ref, err := b.lane.provider.Post(context.Background(), b.lane.dest, "Lane status", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := channel.SaveStatusState(b.lane.root, channel.StatusState{LastPost: t0.Add(-10 * time.Minute), Ref: ref, GoalID: "lane-goal"}); err != nil {
			t.Fatal(err)
		}
		b.reply(t, ref.ID, "yes", t0)
		tick(t, b, b.seat, t0)
		for _, minute := range []int{2, 12, 16, 22, 32, 40} {
			at := t0.Add(time.Duration(minute) * time.Minute)
			if minute == 16 || minute == 40 {
				tick(t, b, b.seat, at)
			} else if _, err := channel.SettleReplies(context.Background(), config(b, b.lane, at), nil); err != nil {
				t.Fatal(err)
			}
		}
		record := b.recordFor(t, "yes").Path
		if got := waiting(t, b); len(got) != 1 || got[0].Record != record || handled(t, b.lane, record) || len(unmatched(t, b)) != 0 {
			t.Fatalf("reply while owner at helm: waiting=%+v notices=%+v", got, unmatched(t, b))
		}
		tick(t, b, b.lane, t0.Add(41*time.Minute))
		if !handled(t, b.lane, record) {
			t.Fatal("status reply was not marked handled")
		}
		posts := b.sent(t)
		if len(posts) != 2 || posts[1].Listener != "lane" || replyParent(posts[1]) != ref.ID {
			t.Fatalf("status reply was not answered in its thread: %+v", posts)
		}
		tick(t, b, b.seat, t0.Add(56*time.Minute))
		if len(unmatched(t, b)) != 0 || len(waiting(t, b)) != 0 {
			t.Fatalf("handled status reply: waiting=%+v notices=%+v", waiting(t, b), unmatched(t, b))
		}
	})
}

// threadReadingProvider receives only replies in the threads the caller supplies.
type threadReadingProvider struct{ channel.Provider }

func (p threadReadingProvider) Receive(ctx context.Context, d channel.DestinationConfig, threads []channel.MessageRef, cursor channel.Cursor) ([]channel.Inbound, channel.Cursor, error) {
	rows, next, err := p.Provider.Receive(ctx, d, threads, cursor)
	return slices.DeleteFunc(rows, func(in channel.Inbound) bool {
		return !slices.ContainsFunc(threads, func(ref channel.MessageRef) bool { return in.ThreadID == ref.ID || in.ThreadID == ref.ThreadID })
	}), next, err
}

func TestAReplyToAClosedQuestionIsToldOnce(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 10, 14, 9, 13, 0, 0, time.Local)
	newBed := func(t *testing.T) *inboxBed {
		t.Helper()
		return newInboxBed(t, inboxSetup{now: t0, conf: "channel.human.answer-code=off\n"})
	}
	act := channel.PersonAct{Verb: "work build", Goal: "seat-goal", Work: "U"}
	const command = "metasystem work build seat-goal --work U --brief brief.md"
	bound := func(t *testing.T, b *inboxBed) channel.Question {
		t.Helper()
		q, err := channel.Ask(channel.AskRequest{RepoRoot: b.seat.root, Goal: "seat-goal", Kind: "other", Machine: b.seat.machine, Lineage: b.seat.name, Facts: []string{"Start the build?"}, Plain: &channel.Plain{Decision: "Start the build?", Act: command}, PersonAct: &act, Now: t0.Add(-10 * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	settled := func(t *testing.T, b *inboxBed) []journalRow {
		t.Helper()
		var rows []journalRow
		for _, row := range b.sent(t) {
			if text, _ := row.Form["text"].(string); strings.HasPrefix(text, "This was settled") {
				rows = append(rows, row)
			}
		}
		return rows
	}
	wantSettled := func(t *testing.T, b *inboxBed, q channel.Question, text string) {
		t.Helper()
		rows := settled(t, b)
		if len(rows) != 1 || rows[0].Form["text"] != text || replyParent(rows[0]) != q.Thread.ID || len(unmatched(t, b)) != 0 {
			t.Fatalf("settled=%+v unmatched=%+v; want %q", rows, unmatched(t, b), text)
		}
		if !handled(t, b.seat, b.recordFor(t, "yes").Path) {
			t.Fatal("settled reply was not marked handled")
		}
	}
	t.Run("a reply leaves a bound question open", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		q := bound(t, b)
		tick(t, b, b.seat, t0.Add(-5*time.Minute))
		q = b.question(t, b.seat, q.ID)
		if got := channel.ReplyInstructions(q); got != "Run "+command+"; its success closes this question. A text answer leaves it open." {
			t.Fatal(got)
		}
		b.reply(t, q.Thread.ID, "yes", t0)
		tick(t, b, b.seat, t0)
		tick(t, b, b.seat, t0.Add(16*time.Minute))
		q = b.question(t, b.seat, q.ID)
		if q.State != "open" || q.Answer == nil || q.Answer.Text != "yes" || q.Answer.Phase != "recorded" || channel.AnswerSummary(q) != "replied 'yes' at 09:13; the act is still needed: "+command || !handled(t, b.seat, b.recordFor(t, "yes").Path) || len(settled(t, b)) != 0 || len(unmatched(t, b)) != 0 {
			t.Fatalf("question=%+v answer=%+v", q, q.Answer)
		}
	})
	t.Run("the act closed it before the match", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		q := bound(t, b)
		tick(t, b, b.seat, t0.Add(-5*time.Minute))
		q = b.question(t, b.seat, q.ID)
		b.reply(t, q.Thread.ID, "yes", t0)
		tick(t, b, b.lane, t0)
		if err := channel.ClosePersonActs(b.seat.root, act, t0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		tick(t, b, b.seat, t0.Add(2*time.Minute))
		wantSettled(t, b, q, "This was settled at 09:14: the act it asked for ran (metasystem work build).")
		if got := b.question(t, b.seat, q.ID); got.Answer != nil {
			t.Fatalf("closed reply applied: %+v", got)
		}
		tick(t, b, b.lane, t0.Add(16*time.Minute))
		tick(t, b, b.seat, t0.Add(20*time.Minute))
		wantSettled(t, b, q, "This was settled at 09:14: the act it asked for ran (metasystem work build).")
		data, err := os.ReadFile(filepath.Join(b.lane.root, "artifacts/agents/channel/fleet/notices.json"))
		var ledger struct{ Awaiting []any }
		if err != nil || json.Unmarshal(data, &ledger) != nil || len(ledger.Awaiting) != 0 {
			t.Fatalf("waiting: %s %v", data, err)
		}
	})
	t.Run("a later reply to an answered question", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		q := b.ask(t, b.seat, "seat-goal", "Choose the next step")
		tick(t, b, b.seat, t0.Add(-5*time.Minute))
		q = b.question(t, b.seat, q.ID)
		b.reply(t, q.Thread.ID, "yes", t0.Add(-3*time.Minute))
		tick(t, b, b.seat, t0.Add(-2*time.Minute))
		b.reply(t, q.Thread.ID, "no", t0)
		tick(t, b, b.seat, t0.Add(time.Minute))
		tick(t, b, b.seat, t0.Add(16*time.Minute))
		wantSettled(t, b, q, "This was settled at 09:11.")
		if got := b.question(t, b.seat, q.ID); got.Answer == nil || got.Answer.Text != "yes" || !handled(t, b.seat, b.recordFor(t, "no").Path) {
			t.Fatalf("answer changed: %+v", got)
		}
	})
	t.Run("an older reply is never told", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		var records []string
		for index, age := range []time.Duration{10 * time.Minute, 25 * time.Hour} {
			q, err := channel.Ask(channel.AskRequest{RepoRoot: b.seat.root, Goal: "seat-goal", Kind: "other", Machine: b.seat.machine, Lineage: b.seat.name, Facts: []string{fmt.Sprintf("Decision %d", index)}, Now: t0.Add(-27 * time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			tick(t, b, b.seat, t0.Add(-26*time.Hour))
			q = b.question(t, b.seat, q.ID)
			if _, err := channel.Withdraw(b.seat.root, q.ID, "done", nil, channel.DestinationConfig{}, t0.Add(-age)); err != nil {
				t.Fatal(err)
			}
			at := t0.Add(-30 * time.Minute)
			if index == 1 {
				at = t0.Add(-time.Hour)
			}
			ulid, err := goal.NewOperationULID()
			if err != nil {
				t.Fatal(err)
			}
			opid, step := goal.Opid(ulid, "m1e", "steward"), int64(42+index)
			in := &goal.ChannelInbound{Provider: "telegram", Destination: "fleet", MessageID: fmt.Sprintf("old-%d", index), ReplyTo: &q.Thread.ID, UserID: inboxHuman, SentAt: at.UTC().Format(time.RFC3339), Text: "yes", Step: &step, Outcome: "verified", Question: "unmatched", Opid: opid, ReceivedBy: "m1e", ReceivedAt: at.UTC().Format(time.RFC3339)}
			data, err := goal.RenderChannelInbound(in)
			if err != nil {
				t.Fatal(err)
			}
			ep, err := project.PublicationEndpoint(b.seat.root)
			if err != nil {
				t.Fatal(err)
			}
			path := goal.ChannelInboxPath("fleet", in)
			result, err := goal.Publish(ep, goal.PublishRequest{Opid: opid, Machine: "m1e", Lineage: "steward", Intent: goal.Intent{Verb: "inbox"}, Message: "channel inbox " + in.MessageID, Mutate: func(string) ([]goal.Change, error) { return []goal.Change{{Path: path, Content: data}}, nil }})
			if err != nil || result.Outcome != goal.OutcomeConfirmed {
				t.Fatalf("old reply: %+v %v", result, err)
			}
			records = append(records, path)
		}
		tick(t, b, b.seat, t0)
		tick(t, b, b.seat, t0.Add(20*time.Minute))
		if len(settled(t, b)) != 0 {
			t.Fatal(settled(t, b))
		}
		for _, path := range records {
			if handled(t, b.seat, path) {
				t.Fatal("old reply marked handled", path)
			}
		}
	})
	t.Run("a provider reads only the supplied threads", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		b.seat.provider = threadReadingProvider{b.seat.provider}
		q := bound(t, b)
		tick(t, b, b.seat, t0.Add(-5*time.Minute))
		q = b.question(t, b.seat, q.ID)
		if err := channel.ClosePersonActs(b.seat.root, act, t0.Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
		b.reply(t, q.Thread.ID, "yes", t0)
		tick(t, b, b.seat, t0.Add(time.Minute))
		tick(t, b, b.seat, t0.Add(16*time.Minute))
		wantSettled(t, b, q, "This was settled at 09:12: the act it asked for ran (metasystem work build).")
		if len(b.inbox(t)) != 1 {
			t.Fatal("reply not committed")
		}
	})
	t.Run("another closer's reason is not shown", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		q, err := channel.Ask(channel.AskRequest{RepoRoot: b.seat.root, About: "machine", Kind: "other", Machine: b.seat.machine, Lineage: b.seat.name, Facts: []string{"Recover the seat"}, Recovery: &channel.RecoverySubject{Seat: "m1e", Launch: "failed-seat"}, Wants: "metasystem machine revive m1e", Now: t0.Add(-10 * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		tick(t, b, b.seat, t0.Add(-5*time.Minute))
		q = b.question(t, b.seat, q.ID)
		if _, err := channel.Withdraw(b.seat.root, q.ID, "matching recovery launch launch-7f3", nil, channel.DestinationConfig{}, t0.Add(-time.Minute)); err != nil {
			t.Fatal(err)
		}
		b.reply(t, q.Thread.ID, "yes", t0)
		tick(t, b, b.seat, t0.Add(time.Minute))
		tick(t, b, b.seat, t0.Add(16*time.Minute))
		wantSettled(t, b, q, "This was settled at 09:12.")
		for _, row := range b.sent(t) {
			if text, _ := row.Form["text"].(string); strings.Contains(text, "launch") {
				t.Fatal(text)
			}
		}
	})
}
