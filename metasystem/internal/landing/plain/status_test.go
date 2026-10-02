package plain

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// ReadStatus is the lane as landing status --json and /api/board's lane both
// carry it (goal fleet-card-can-land-now: one reader for the terminal and
// the card): the view, whether the lane is paused and its agent alive, the
// queue with its states derived against origin's main, the running proof,
// the last proof and the last push.
func TestReadStatusIsTheLaneTheTerminalAndTheCardRead(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	before := b.originMain()
	shaA := b.seat("seat-a", "goal-a")
	b.handIn("m1e", "goal-a", shaA)
	head := b.merge("goal-a")
	b.prove(b.greenScript)
	if _, err := b.push(); err != nil {
		t.Fatal(err)
	}
	shaB := b.seat("seat-b", "goal-b")
	b.handIn("ui", "goal-b", shaB)
	b.git(b.checkout, "fetch", "--quiet", "origin")
	home := t.TempDir()
	record := lane.Record{Root: b.checkout, Install: b.install}
	view := lane.View{Root: &b.checkout, Owner: lane.OwnerView{State: lane.OwnerRunning}}

	status := ReadStatus(home, record, view, ProveSeams{Now: func() time.Time { return bedNow.Add(time.Hour) }})

	if status.Paused || !status.AgentAlive || status.Root == nil || *status.Root != b.checkout {
		t.Fatalf("paused %v alive %v root %v; want not paused, alive, the view's root", status.Paused, status.AgentAlive, status.Root)
	}
	states := map[string]string{}
	for _, entry := range status.Queue {
		states[entry.Goal] = entry.State
	}
	if len(status.Queue) != 2 || states["goal-a"] != StateLanded || states["goal-b"] != StateWaiting {
		t.Fatalf("queue %+v; want goal-a landed and goal-b waiting", status.Queue)
	}
	// The landing time is the lane checkout's own answer to which commits
	// the push brought to main.
	if status.Queue[0].LandedAt != bedNow.Format(time.RFC3339) || status.Queue[1].LandedAt != "" || len(status.Problems) != 0 {
		t.Fatalf("queue %+v, problems %q; want goal-a landed at the push's time", status.Queue, status.Problems)
	}
	if status.RunningProof != nil {
		t.Fatalf("running proof %+v; want none", status.RunningProof)
	}
	if status.LastProof == nil || status.LastProof.Result != Green || status.LastProof.Commit != head {
		t.Fatalf("last proof %+v; want the green proof of %s", status.LastProof, head)
	}
	if status.LastPush == nil || status.LastPush.Old != before || status.LastPush.Commit != head {
		t.Fatalf("last push %+v; want %s to %s", status.LastPush, before, head)
	}

	if _, err := lane.SetPause(home, "Wido", bedNow); err != nil {
		t.Fatal(err)
	}
	idle := lane.View{Root: &b.checkout, Owner: lane.OwnerView{State: lane.OwnerStopped}}
	if paused := ReadStatus(home, record, idle, ProveSeams{}); !paused.Paused || paused.AgentAlive {
		t.Fatalf("paused %v alive %v; want paused and no agent", paused.Paused, paused.AgentAlive)
	}
}

// A lane with no root reads as its view and an empty queue, never null.
func TestReadStatusOfNoLaneIsAnEmptyQueue(t *testing.T) {
	t.Parallel()
	status := ReadStatus(t.TempDir(), lane.Record{}, lane.View{}, ProveSeams{})
	if status.Queue == nil || len(status.Queue) != 0 || status.LastProof != nil || status.LastPush != nil || status.RunningProof != nil {
		t.Fatalf("status of no lane = %+v", status)
	}
	if status.Problems == nil || len(status.Problems) != 0 {
		t.Fatalf("problems of no lane = %#v; want an empty list, never null", status.Problems)
	}
}

// statusBed is a registered lane whose records are written by hand and whose
// Git is answered by the test: no repository is needed to read its status.
type statusBed struct {
	t        *testing.T
	home     string
	record   lane.Record
	view     lane.View
	install  string
	checkout string
}

func newStatusBed(t *testing.T) *statusBed {
	t.Helper()
	checkout := t.TempDir()
	install := filepath.Join(checkout, "metasystem")
	root := checkout
	return &statusBed{t: t, home: t.TempDir(), checkout: checkout, install: install,
		record: lane.Record{Root: checkout, Install: install},
		view:   lane.View{Root: &root, Owner: lane.OwnerView{State: lane.OwnerIdle}}}
}

// lines writes one record file of the lane, a JSON line per value.
func (b *statusBed) lines(name string, values ...any) {
	b.t.Helper()
	var text strings.Builder
	for _, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			b.t.Fatal(err)
		}
		text.Write(append(data, '\n'))
	}
	if err := os.MkdirAll(Dir(b.install), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(b.install), name), []byte(text.String()), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// unreadable puts a folder where a record file is, so reading it fails.
func (b *statusBed) unreadable(name string) {
	b.t.Helper()
	if err := os.MkdirAll(filepath.Join(Dir(b.install), name), 0o755); err != nil {
		b.t.Fatal(err)
	}
}

// git answers the lane checkout's three questions from fixed facts: main is
// "main", which contains the commits named, and each push brought the
// commits listed under its range.
func (b *statusBed) git(contained []string, brought map[string][]string) laneGit {
	inside := map[string]bool{}
	for _, sha := range contained {
		inside[sha] = true
	}
	return laneGit{
		main:     func() (string, error) { return "main", nil },
		contains: func(_, sha string) (bool, error) { return inside[sha], nil },
		brought: func(old, commit string) ([]string, error) {
			return brought[old+".."+commit], nil
		},
	}
}

// A landed hand-in carries when it landed: the time of the push that brought
// its commit to main, read from pushes.jsonl. Nothing is stamped anew, and a
// hand-in main contains that no push of the lane brought carries no time.
func TestReadStatusSaysWhenEachHandInLanded(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.lines("queue.jsonl",
		Line{Goal: "goal-a", SHA: "sha-a", Seat: "m1e", At: "2026-10-01T20:00:00Z", Delivered: "Stuck agents now ask you on Telegram."},
		Line{Goal: "goal-b", SHA: "sha-b", Seat: "ui", At: "2026-10-01T20:30:00Z"},
		Line{Goal: "goal-c", SHA: "sha-c", Seat: "m1f", At: "2026-10-01T21:00:00Z"},
		Line{Goal: "goal-d", SHA: "sha-d", Seat: "m1g", At: "2026-10-01T21:10:00Z"})
	b.lines("pushes.jsonl",
		Pushed{Old: "m0", Commit: "m1", At: "2026-10-01T21:20:00Z"},
		Pushed{Old: "m1", Commit: "m2", At: "2026-10-01T21:54:00Z"})
	git := b.git([]string{"sha-a", "sha-b", "sha-d"}, map[string][]string{
		"m0..m1": {"m1", "sha-a"},
		"m1..m2": {"m2", "sha-b", "x1"},
	})

	status := readStatus(b.home, b.record, b.view, ProveSeams{Now: func() time.Time { return bedNow }}, git)

	got := map[string]Entry{}
	for _, entry := range status.Queue {
		got[entry.Goal] = entry
	}
	if got["goal-a"].State != StateLanded || got["goal-a"].LandedAt != "2026-10-01T21:20:00Z" {
		t.Fatalf("goal-a = %+v; want landed by the first push, at its time", got["goal-a"])
	}
	if got["goal-a"].Delivered != "Stuck agents now ask you on Telegram." {
		t.Fatalf("goal-a delivered %q; want the hand-in's sentence", got["goal-a"].Delivered)
	}
	if got["goal-b"].State != StateLanded || got["goal-b"].LandedAt != "2026-10-01T21:54:00Z" {
		t.Fatalf("goal-b = %+v; want landed by the second push, at its time", got["goal-b"])
	}
	if got["goal-c"].State != StateWaiting || got["goal-c"].LandedAt != "" {
		t.Fatalf("goal-c = %+v; want waiting, with no landing time", got["goal-c"])
	}
	if got["goal-d"].State != StateLanded || got["goal-d"].LandedAt != "" {
		t.Fatalf("goal-d = %+v; want landed with no time: no push of the lane brought it", got["goal-d"])
	}
	if len(status.Problems) != 0 {
		t.Fatalf("problems %v; want none", status.Problems)
	}
}

// Only the last day's pushes are asked what they brought: the page lists
// what landed today, and the push record grows with every landing.
func TestLandingTimesAskOnlyTheLastDaysPushes(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		{Goal: "old", SHA: "sha-old", State: StateLanded},
		{Goal: "new", SHA: "sha-new", State: StateLanded},
		{Goal: "back", SHA: "sha-back", State: StateReturned},
	}
	pushes := []Pushed{
		{Old: "m0", Commit: "m1", At: "2026-09-29T09:00:00Z"},
		{Old: "m3", Commit: "m4", At: "2026-10-01T21:00:00Z"},
	}
	var asked []string
	brought := func(old, commit string) ([]string, error) {
		asked = append(asked, old+".."+commit)
		return []string{"sha-old", "sha-new", "sha-back"}, nil
	}

	timed, err := LandingTimes(entries, pushes, bedNow.Add(-24*time.Hour), brought)

	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(asked, " ") != "m3..m4" {
		t.Fatalf("asked %v; want only the day's push", asked)
	}
	if timed[0].LandedAt != "2026-10-01T21:00:00Z" || timed[1].LandedAt != "2026-10-01T21:00:00Z" {
		t.Fatalf("landed entries %+v; want the day's push's time", timed[:2])
	}
	if timed[2].LandedAt != "" {
		t.Fatalf("returned entry %+v; want no landing time", timed[2])
	}
}

// A push whose commits can't be listed is named, and the other pushes still
// give their entries a time.
func TestLandingTimesNameAPushTheyCannotRead(t *testing.T) {
	t.Parallel()
	entries := []Entry{{Goal: "a", SHA: "sha-a", State: StateLanded}, {Goal: "b", SHA: "sha-b", State: StateLanded}}
	pushes := []Pushed{{Old: "m0", Commit: "m1", At: "2026-10-01T20:00:00Z"}, {Old: "m1", Commit: "m2", At: "2026-10-01T21:00:00Z"}}
	brought := func(old, _ string) ([]string, error) {
		if old == "m0" {
			return nil, errors.New("bad object m0")
		}
		return []string{"sha-b"}, nil
	}

	timed, err := LandingTimes(entries, pushes, bedNow.Add(-24*time.Hour), brought)

	if err == nil || !strings.Contains(err.Error(), "bad object m0") {
		t.Fatalf("err %v; want the unreadable push named", err)
	}
	if timed[0].LandedAt != "" || timed[1].LandedAt != "2026-10-01T21:00:00Z" {
		t.Fatalf("entries %+v; want b timed by the readable push", timed)
	}
}

// What the status could not read is said, record by record, instead of
// reading as an empty queue, no proof or no push: the queue, whether main
// contains the queued work, the running proof, the proof results and the
// push record.
func TestReadStatusSaysWhatItCouldNotRead(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.lines("queue.jsonl", Line{Goal: "goal-a", SHA: "sha-a", Seat: "m1e", At: "2026-10-01T20:00:00Z"})
	b.unreadable("results.jsonl")
	b.unreadable("pushes.jsonl")
	b.unreadable("running.json")
	git := b.git(nil, nil)
	git.main = func() (string, error) { return "", errors.New("origin's main is not fetched") }

	status := readStatus(b.home, b.record, b.view, ProveSeams{Now: func() time.Time { return bedNow }}, git)

	if len(status.Queue) != 1 || status.Queue[0].State != StateWaiting {
		t.Fatalf("queue %+v; want the hand-in as recorded", status.Queue)
	}
	want := []string{
		"whether main holds the queued work can't be read: origin's main is not fetched",
		"the running proof can't be read: ",
		"the proof results can't be read: ",
		"the push record can't be read: ",
	}
	if len(status.Problems) != len(want) {
		t.Fatalf("problems %q; want %d", status.Problems, len(want))
	}
	for index, prefix := range want {
		if !strings.HasPrefix(status.Problems[index], prefix) {
			t.Fatalf("problem %d = %q; want it to start %q", index, status.Problems[index], prefix)
		}
	}

	torn := newStatusBed(t)
	torn.unreadable("queue.jsonl")
	contains := 0
	git = torn.git(nil, nil)
	git.contains = func(string, string) (bool, error) { contains++; return false, nil }
	unread := readStatus(torn.home, torn.record, torn.view, ProveSeams{}, git)
	if len(unread.Problems) != 1 || !strings.HasPrefix(unread.Problems[0], "the queue can't be read: ") {
		t.Fatalf("problems %q; want the queue named", unread.Problems)
	}
	if unread.Queue == nil || len(unread.Queue) != 0 || contains != 0 {
		t.Fatalf("queue %+v, %d containment reads; want an empty list and nothing asked", unread.Queue, contains)
	}
}

// Whether main contains one hand-in can't be read: that hand-in stays as
// recorded and the status says so; the others are still derived.
func TestReadStatusSaysWhichContainmentItCouldNotRead(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.lines("queue.jsonl",
		Line{Goal: "goal-a", SHA: "sha-a", At: "2026-10-01T20:00:00Z"},
		Line{Goal: "goal-b", SHA: "sha-b", At: "2026-10-01T20:00:00Z"})
	git := b.git([]string{"sha-b"}, nil)
	inside := git.contains
	git.contains = func(main, sha string) (bool, error) {
		if sha == "sha-a" {
			return false, errors.New("merge-base failed")
		}
		return inside(main, sha)
	}

	status := readStatus(b.home, b.record, b.view, ProveSeams{}, git)

	if status.Queue[0].State != StateWaiting || status.Queue[1].State != StateLanded {
		t.Fatalf("queue %+v; want goal-a as recorded and goal-b landed", status.Queue)
	}
	if len(status.Problems) != 1 || !strings.Contains(status.Problems[0], "goal-a: merge-base failed") {
		t.Fatalf("problems %q; want goal-a's containment named", status.Problems)
	}
}

// The running proof reads as running while its process runs and as died once
// it is gone without a result; with none recorded there is none.
func TestReadStatusSaysWhetherTheRunningProofStillRuns(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	if ReadRunningProof(b.install, ProveSeams{}) != nil {
		t.Fatal("a lane with no running.json has a running proof")
	}
	b.lines("running.json", Running{Attempt: "a-7", Tree: "t1", Commit: "c1", Since: "2026-10-01T21:00:00Z", Log: "/l"})
	for alive, state := range map[bool]string{true: "running", false: "died"} {
		seams := ProveSeams{Alive: func(Running) bool { return alive }}
		status := readStatus(b.home, b.record, b.view, seams, b.git(nil, nil))
		if status.RunningProof == nil || status.RunningProof.State != state || status.RunningProof.Attempt != "a-7" || status.RunningProof.Log != "/l" {
			t.Fatalf("alive %v: running proof %+v; want %s", alive, status.RunningProof, state)
		}
		if read := ReadRunningProof(b.install, seams); read == nil || read.State != state {
			t.Fatalf("alive %v: ReadRunningProof %+v; want %s", alive, read, state)
		}
	}
}

// A lane record whose roots are not a layout is said, not read as a lane
// with nothing in it.
func TestReadStatusSaysALaneRecordItCannotPlace(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.record.Install = "/elsewhere"

	status := readStatus(b.home, b.record, b.view, ProveSeams{}, b.git(nil, nil))

	if len(status.Problems) != 1 || !strings.HasPrefix(status.Problems[0], "the lane's record can't be placed: ") {
		t.Fatalf("problems %q; want the record named", status.Problems)
	}
}

// A lane whose registration can't be read is a lane the status could not
// read, not a lane that is not there: it carries the read error as a problem.
func TestReadStatusSaysARegistrationItCannotRead(t *testing.T) {
	t.Parallel()
	view := lane.View{Owner: lane.OwnerView{State: lane.OwnerUnready}, Unreadable: "invalid character 'n' looking for beginning of object key string"}

	status := readStatus(t.TempDir(), lane.Record{}, view, ProveSeams{}, laneGit{})

	if len(status.Problems) != 1 || status.Problems[0] != "the lane's registration can't be read: invalid character 'n' looking for beginning of object key string" {
		t.Fatalf("problems %q; want the registration named", status.Problems)
	}
	if none := readStatus(t.TempDir(), lane.Record{}, lane.View{}, ProveSeams{}, laneGit{}); len(none.Problems) != 0 {
		t.Fatalf("a lane never registered has problems %q", none.Problems)
	}
}

// The running proof names the waiting hand-ins its commit holds, by the same
// containment check that derives landed; a died proof names none, and one
// whose containment can't be read is said.
func TestReadStatusNamesWhatTheRunningProofHolds(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.lines("queue.jsonl",
		Line{Goal: "landed-before", SHA: "sha-l", At: "2026-10-01T19:00:00Z"},
		Line{Goal: "seat-path", SHA: "sha-a", At: "2026-10-01T20:00:00Z"},
		Line{Goal: "plain-lane", SHA: "sha-b", At: "2026-10-01T20:10:00Z"},
		Line{Goal: "not-merged", SHA: "sha-c", At: "2026-10-01T20:20:00Z"})
	b.lines("running.json", Running{Attempt: "a-9", Tree: "t9", Commit: "proof-head", Since: "2026-10-01T21:50:00Z"})
	git := b.git([]string{"sha-l"}, nil)
	git.contains = func(commit, sha string) (bool, error) {
		if commit == "proof-head" {
			return sha == "sha-a" || sha == "sha-b" || sha == "sha-l", nil
		}
		return sha == "sha-l", nil
	}
	alive := ProveSeams{Alive: func(Running) bool { return true }}

	status := readStatus(b.home, b.record, b.view, alive, git)

	if status.RunningProof == nil || strings.Join(status.RunningProof.Goals, " ") != "seat-path plain-lane" || len(status.Problems) != 0 {
		t.Fatalf("running proof %+v, problems %q; want the two waiting hand-ins it holds", status.RunningProof, status.Problems)
	}
	died := readStatus(b.home, b.record, b.view, ProveSeams{Alive: func(Running) bool { return false }}, git)
	if died.RunningProof == nil || len(died.RunningProof.Goals) != 0 {
		t.Fatalf("died proof %+v; want no goals named", died.RunningProof)
	}
	git.contains = func(commit, sha string) (bool, error) {
		if commit == "proof-head" {
			return false, errors.New("bad object proof-head")
		}
		return sha == "sha-l", nil
	}
	torn := readStatus(b.home, b.record, b.view, alive, git)
	if len(torn.Problems) != 1 || !strings.HasPrefix(torn.Problems[0], "what the running proof holds can't be read: seat-path: bad object proof-head") {
		t.Fatalf("problems %q; want the proof's containment named", torn.Problems)
	}
}

// raw writes one record file of the lane as the text given.
func (b *statusBed) raw(name, text string) {
	b.t.Helper()
	if err := os.MkdirAll(Dir(b.install), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(b.install), name), []byte(text), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// jsonLine is value as one JSON line.
func jsonLine(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}

// Fix round 4, F-1: a line of the queue, the proof results or the push
// record that does not decode is said, counted for its file, instead of
// being left out unsaid; the lines that decode are still read, and a blank
// line, which holds no record, is not counted.
func TestReadStatusSaysLinesItCouldNotDecode(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.raw("queue.jsonl", jsonLine(t, Line{Goal: "goal-a", SHA: "sha-a", At: "2026-10-01T20:00:00Z"})+`{"goal":"goal-b","sh`+"\n\n"+`{"goal":`+"\n")
	b.raw("results.jsonl", jsonLine(t, Result{Tree: "t1", Commit: "c1", Result: Green, At: "2026-10-01T20:30:00Z"})+`{"tree":"t2","res`+"\n")
	b.raw("pushes.jsonl", jsonLine(t, Pushed{Old: "m0", Commit: "m1", Tree: "t1", At: "2026-10-01T20:40:00Z"})+`{"old":"m1","com`+"\n")

	status := readStatus(b.home, b.record, b.view, ProveSeams{Now: func() time.Time { return bedNow }}, b.git(nil, nil))

	dir := Dir(b.install)
	want := []string{
		"the queue has 2 lines that can't be read (" + filepath.Join(dir, "queue.jsonl") + ")",
		"the proof results have 1 line that can't be read (" + filepath.Join(dir, "results.jsonl") + ")",
		"the push record has 1 line that can't be read (" + filepath.Join(dir, "pushes.jsonl") + ")",
	}
	if strings.Join(status.Problems, "\n") != strings.Join(want, "\n") {
		t.Fatalf("problems %q; want %q", status.Problems, want)
	}
	if len(status.Queue) != 1 || status.Queue[0].Goal != "goal-a" {
		t.Fatalf("queue %+v; want goal-a still read", status.Queue)
	}
	if status.LastProof == nil || status.LastProof.Tree != "t1" || status.LastPush == nil || status.LastPush.Commit != "m1" {
		t.Fatalf("last proof %+v, last push %+v; want the lines that decode still read", status.LastProof, status.LastPush)
	}
	// The lane's own readers still skip the line: a partial append never
	// stops the keeper or a verb.
	if entries, err := Entries(b.install); err != nil || len(entries) != 1 {
		t.Fatalf("Entries = %+v, %v; want the one line that decodes and no error", entries, err)
	}
}

// Fix round 4, F-2: what the view itself could not read reaches the status's
// problems: the wake's unread list (an unreadable keeper record, on which
// the keeper starts nothing) as one plain line each, never twice.
func TestReadStatusCarriesWhatTheViewCouldNotRead(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	keeper := lane.UnreadableAgentRecord(b.home)
	b.view.Wake = &lane.Wake{Reasons: []string{}, Unread: []string{"unexpected end of JSON input", keeper, "unexpected end of JSON input"}}

	status := readStatus(b.home, b.record, b.view, ProveSeams{}, b.git(nil, nil))

	want := []string{"unexpected end of JSON input", strings.ReplaceAll(keeper, "\n", "; ")}
	if strings.Join(status.Problems, "\n") != strings.Join(want, "\n") {
		t.Fatalf("problems %q; want %q", status.Problems, want)
	}
}

// Fix round 4, sweep: a pause record that can't be read reads as stopped (it
// fails closed), and the status says it could not read it.
func TestReadStatusSaysAPauseItCouldNotRead(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	if err := os.MkdirAll(lane.HostDir(b.home), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-lane-paused.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	status := readStatus(b.home, b.record, b.view, ProveSeams{}, b.git(nil, nil))

	if !status.Paused || len(status.Problems) != 1 || status.Problems[0] != "the lane's pause record can't be read, so the lane reads as stopped" {
		t.Fatalf("paused %v, problems %q; want stopped and the pause record named", status.Paused, status.Problems)
	}
}

// Fix round 4, sweep: whether the landing agent runs, or whether the lane can
// run, that the view could not read reaches the problems as the view said it.
func TestReadStatusSaysAnAgentItCouldNotCheck(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	unknown := "whether the landing agent runs is unknown: permission denied"
	b.view.Owner = lane.OwnerView{State: lane.OwnerUnready, LastExit: &unknown, Unread: unknown}

	status := readStatus(b.home, b.record, b.view, ProveSeams{}, b.git(nil, nil))

	if len(status.Problems) != 1 || status.Problems[0] != unknown {
		t.Fatalf("problems %q; want %q", status.Problems, unknown)
	}
}

// Fix round 4, sweep: a push line that names no old main, no commit or no
// readable time can't say what it brought: it is named, and the other pushes
// still time their entries.
func TestLandingTimesNameAPushLineTheyCannotRead(t *testing.T) {
	t.Parallel()
	entries := []Entry{{Goal: "a", SHA: "sha-a", State: StateLanded}}
	pushes := []Pushed{
		{Old: "m1", Commit: "m2", At: "not a time"},
		{Old: "", Commit: "m3", At: "2026-10-01T20:00:00Z"},
		{Old: "m3", Commit: "m4", At: "2026-10-01T21:00:00Z"},
	}
	brought := func(old, commit string) ([]string, error) { return []string{"sha-a"}, nil }

	timed, err := LandingTimes(entries, pushes, bedNow.Add(-24*time.Hour), brought)

	if err == nil || !strings.Contains(err.Error(), `a push line can't be read (old "m1", commit "m2", at "not a time")`) ||
		!strings.Contains(err.Error(), `a push line can't be read (old "", commit "m3", at "2026-10-01T20:00:00Z")`) {
		t.Fatalf("err %v; want both damaged push lines named", err)
	}
	if timed[0].LandedAt != "2026-10-01T21:00:00Z" {
		t.Fatalf("entries %+v; want a timed by the readable push", timed)
	}
}

// A goal handed in again after its earlier hand-in landed keeps that
// landing in today's list; a hand-in superseded before the last day is not
// asked again.
func TestReadStatusKeepsALandingItsGoalHandedInAgain(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	old := bedNow.Add(-72 * time.Hour).Format(time.RFC3339)
	older := bedNow.Add(-73 * time.Hour).Format(time.RFC3339)
	b.lines("queue.jsonl",
		Line{Goal: "goal-z", SHA: "sha-z1", Seat: "ui", At: older},
		Line{Goal: "goal-z", SHA: "sha-z2", Seat: "ui", At: old},
		Line{Goal: "goal-a", SHA: "sha-a1", Seat: "ui", At: "2026-10-01T20:00:00Z", Delivered: "Step one."},
		Line{Goal: "goal-a", SHA: "sha-a2", Seat: "ui", At: "2026-10-01T22:00:00Z"})
	b.lines("pushes.jsonl", Pushed{Old: "m0", Commit: "m1", At: "2026-10-01T21:00:00Z"})
	git := b.git([]string{"sha-a1", "sha-z1", "sha-z2"}, map[string][]string{"m0..m1": {"m1", "sha-a1"}})

	status := readStatus(b.home, b.record, b.view, ProveSeams{Now: func() time.Time { return bedNow }}, git)

	got := map[string]Entry{}
	for _, entry := range status.Queue {
		got[entry.SHA] = entry
	}
	if got["sha-a1"].State != StateLanded || got["sha-a1"].LandedAt != "2026-10-01T21:00:00Z" || got["sha-a1"].Delivered != "Step one." {
		t.Fatalf("sha-a1 = %+v; want landed at its push, with its sentence", got["sha-a1"])
	}
	if got["sha-a2"].State != StateWaiting {
		t.Fatalf("sha-a2 = %+v; want the new hand-in waiting", got["sha-a2"])
	}
	if got["sha-z1"].State != StateSuperseded {
		t.Fatalf("sha-z1 = %+v; want superseded: its goal was handed in again before the last day", got["sha-z1"])
	}
	if len(status.Problems) != 0 {
		t.Fatalf("problems %v; want none", status.Problems)
	}
}

// A damaged push line is named even when no landed entry waits for a time.
func TestLandingTimesNameADamagedPushWithNothingToTime(t *testing.T) {
	t.Parallel()
	pushes := []Pushed{{Old: "", Commit: "m3", At: "2026-10-01T20:00:00Z"}}
	brought := func(old, commit string) ([]string, error) { t.Fatal("nothing to time; brought asked"); return nil, nil }

	_, err := LandingTimes([]Entry{{Goal: "a", SHA: "sha-a", State: StateWaiting}}, pushes, bedNow.Add(-24*time.Hour), brought)

	if err == nil || !strings.Contains(err.Error(), `a push line can't be read (old "", commit "m3", at "2026-10-01T20:00:00Z")`) {
		t.Fatalf("err %v; want the damaged push line named", err)
	}
}
