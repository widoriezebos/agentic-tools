package batch

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// scriptedBoard is a pipeline source whose picture a test sets: the board
// the owner re-reads on every decision.
type scriptedBoard struct{ picture BoardPicture }

func (source *scriptedBoard) Board(time.Time) BoardPicture { return source.picture }

func boardCardAt(goal, seat string, stage board.Stage, since time.Time) board.Card {
	return board.Card{Seat: board.Seat{Machine: seat, Installation: "/c/" + seat + "/metasystem"}, Goal: goal, Stage: stage, Since: since, LastProgressAt: since}
}

// startBed is an owner over one open batch with goal-a joined at 10:00, a
// scripted board and an artificial clock.
func startBed(t *testing.T, now time.Time) (*ownerBed, *scriptedBoard) {
	t.Helper()
	bed := newOwnerBed(t, ownerRecord(testBatchID, StateOpen, ten), now)
	source := &scriptedBoard{picture: BoardPicture{Readable: true}}
	bed.owner.pipeline = source
	return bed, source
}

// joinUnit joins goal to the bed's batch at at, as a join admission would.
func joinUnit(t *testing.T, bed *ownerBed, goal string, at time.Time) {
	t.Helper()
	must(t, bed.store.Update(testBatchID, func(record *Record) error {
		record.Units = append(record.Units, Unit{GoalID: goal, Chain: "chain-" + goal, Claim: Claim{Machine: "landing", Lineage: goal, Epoch: 1, Revision: 2, AccountingRevision: 1}, State: UnitJoined})
		appendUnitHistory(record, at, "join", "seat", goal, "", UnitJoined)
		return nil
	}))
}

func tickAt(t *testing.T, bed *ownerBed, at time.Time) Record {
	t.Helper()
	bed.now = at
	must(t, bed.owner.Tick(testBatchID))
	bed.owner.settle()
	return load(t, bed.store)
}

func waitEntries(record Record) int {
	count := 0
	for _, entry := range record.History {
		if entry.Verb == "wait" {
			count++
		}
	}
	return count
}

// TestBatchStartsAtOnceWhenNothingIsWithinReach (R22, U10b-2): one unit
// joined at 10:00 on an empty readable board starts at the first tick; a
// unit expected later than a separate proof, a stalled one, an Unknown one
// and a landing one are named and not waited for; a proof re-ending one
// section is waited for until its stamp crosses the stall bound, then the
// batch starts naming the stall; a quiet host changes nothing.
func TestBatchStartsAtOnceWhenNothingIsWithinReach(t *testing.T) {
	t.Parallel()
	bed, _ := startBed(t, ten)
	record := tickAt(t, bed, ten)
	if bed.launches != 1 || bed.window != WindowNothingNear || record.StartReason != "nothing within reach" {
		t.Fatalf("empty board: launches %d window %q reason %q", bed.launches, bed.window, record.StartReason)
	}

	bed, source := startBed(t, ten)
	source.picture.Cards = []board.Card{boardCardAt("goal-x", "m1b", board.StageBuild, ten.Add(4*time.Minute))}
	source.picture.Cards[0].Job = &board.Job{ID: "j", Phase: "handshake"}
	// A handshake build at 10:00: 41 min to join, later than 40.
	record = tickAt(t, bed, ten)
	if bed.launches != 1 || !strings.Contains(record.StartReason, "goal-x on m1b expected in ~41 min, later than a separate proof (~40 min)") {
		t.Fatalf("out of reach: launches %d reason %q", bed.launches, record.StartReason)
	}

	bed, source = startBed(t, ten)
	stalled := boardCardAt("goal-s", "m1b", board.StageUnitProof, ten.Add(-30*time.Minute))
	source.picture.Unknown = []board.Unknown{
		{Seat: stalled.Seat, Goal: "goal-s", Reason: board.ReasonStalled, Card: &stalled},
		{Seat: board.Seat{Machine: "m1c"}, Goal: "goal-u", Reason: "no card"},
	}
	landing := boardCardAt("goal-l", "m1e", board.StageLanding, ten)
	source.picture.Cards = []board.Card{landing}
	record = tickAt(t, bed, ten)
	if bed.launches != 1 || !strings.Contains(record.StartReason, "goal-s on m1b stalled since") || !strings.Contains(record.StartReason, "goal-u on m1c unknown (no card)") ||
		strings.Contains(record.StartReason, "goal-l") {
		t.Fatalf("stalled, unknown and landing: launches %d reason %q", bed.launches, record.StartReason)
	}

	// A proof re-ending one section every minute: its card keeps done and
	// its first end, so it is waited for until the stall bound, then named.
	bed, source = startBed(t, ten)
	stuck := boardCardAt("goal-p", "m1b", board.StageUnitProof, ten)
	stuck.Proof = &board.Proof{Attempt: "a", Done: 1, Planned: 189}
	stuck.Owner = &board.Owner{Pid: 41}
	source.picture.Cards = []board.Card{stuck}
	record = tickAt(t, bed, ten.Add(time.Minute))
	if bed.launches != 0 || record.Wait == nil || record.Wait.For[0].Goal != "goal-p" {
		t.Fatalf("a proof in progress is waited for: launches %d wait %+v", bed.launches, record.Wait)
	}
	stuckStalled := stuck
	source.picture.Cards = nil
	source.picture.Unknown = []board.Unknown{{Seat: stuck.Seat, Goal: "goal-p", Reason: board.ReasonStalled, Card: &stuckStalled}}
	record = tickAt(t, bed, ten.Add(21*time.Minute))
	if bed.launches != 1 || bed.window != WindowWaited || !strings.Contains(record.StartReason, "goal-p on m1b stalled since") {
		t.Fatalf("a stalled proof ends the wait: launches %d window %q reason %q", bed.launches, bed.window, record.StartReason)
	}

	// A quiet host with one unit changes nothing: the board decides.
	bed, source = startBed(t, ten)
	source.picture.Cards = []board.Card{boardCardAt("goal-r", "m1b", board.StageLandReady, ten)}
	bed.sample = proofrun.LoadSample{OverlapKnown: true}
	record = tickAt(t, bed, ten)
	if bed.launches != 0 || record.Wait == nil {
		t.Fatalf("a quiet host started a batch the board says to wait for: launches %d", bed.launches)
	}
}

// TestBatchWaitsForExactlyTheReachableUnitsAndStartsWhenTheLastJoins (R22,
// U10b-2).
func TestBatchWaitsForExactlyTheReachableUnitsAndStartsWhenTheLastJoins(t *testing.T) {
	t.Parallel()
	three := 3
	picture := func(xStage board.Stage, xSince time.Time, ySeat string) BoardPicture {
		x := boardCardAt("goal-x", "m1b", xStage, xSince)
		x.Round = &board.Round{N: 2, Max: &three}
		y := boardCardAt("goal-y", ySeat, board.StageJudgement, ten.Add(5*time.Minute))
		z := boardCardAt("goal-z", "m1e", board.StageBuild, ten)
		z.Job = &board.Job{ID: "j", Phase: "handshake"}
		return BoardPicture{Readable: true, Cards: []board.Card{x, y, z}}
	}
	// x in review since 10:00 (10:26); y in judgement since 10:05 (10:13);
	// z a build still in its handshake (10:41), out of reach of 10:40.
	bed, source := startBed(t, ten)
	var logged []string
	bed.owner.logWait = func(id, line string) { logged = append(logged, line) }
	source.picture = picture(board.StageReview, ten, "m1c")
	record := tickAt(t, bed, ten)
	if bed.launches != 0 || record.State != StateOpen || record.Wait == nil || len(record.Wait.For) != 2 ||
		record.Wait.For[0].Goal == "goal-z" || record.Wait.For[1].Goal == "goal-z" || waitEntries(record) != 1 {
		t.Fatalf("wait %+v history %d", record.Wait, waitEntries(record))
	}
	// The same picture ten minutes on writes nothing.
	path, _ := bed.store.recordPath(testBatchID)
	before := string(contents(t, path))
	tickAt(t, bed, ten.Add(10*time.Minute))
	if string(contents(t, path)) != before {
		t.Fatal("a repeated decision with the same reason wrote the record")
	}
	if len(logged) != 1 || !strings.HasPrefix(logged[0], "batch "+testBatchID+" waits for goal-") {
		t.Fatalf("the owner logs the line once per change: %q", logged)
	}
	// y joins: the wait names x alone.
	joinUnit(t, bed, "goal-y", ten.Add(20*time.Minute))
	source.picture.Cards = source.picture.Cards[:1:1]
	source.picture.Cards = append(source.picture.Cards, picture(board.StageReview, ten, "m1c").Cards[2])
	record = tickAt(t, bed, ten.Add(20*time.Minute))
	if bed.launches != 0 || len(record.Wait.For) != 1 || record.Wait.For[0].Goal != "goal-x" {
		t.Fatalf("after y joined: %+v", record.Wait)
	}
	// x joins: the next decision starts, naming it.
	joinUnit(t, bed, "goal-x", ten.Add(26*time.Minute))
	// x's own card still lags at land-ready: the record's join decides.
	lagging := picture(board.StageLandReady, ten.Add(25*time.Minute), "m1c").Cards[0]
	source.picture.Cards = append(source.picture.Cards[1:], lagging)
	record = tickAt(t, bed, ten.Add(26*time.Minute))
	if bed.launches != 1 || bed.window != WindowWaited || record.StartReason != "goal-x on m1b joined, the last unit waited for" || record.Wait != nil {
		t.Fatalf("after x joined: launches %d reason %q wait %+v", bed.launches, record.StartReason, record.Wait)
	}

	// x returned at 10:24 without joining: the decision starts then.
	bed, source = startBed(t, ten)
	source.picture = picture(board.StageReview, ten, "m1c")
	tickAt(t, bed, ten)
	joinUnit(t, bed, "goal-y", ten.Add(20*time.Minute))
	source.picture.Cards = []board.Card{boardCardAt("goal-x", "m1b", board.StageReturned, ten.Add(24*time.Minute))}
	record = tickAt(t, bed, ten.Add(24*time.Minute))
	if bed.launches != 1 || record.StartReason != "goal-x on m1b left the pipeline without joining; nothing else within reach" {
		t.Fatalf("x returned: launches %d reason %q", bed.launches, record.StartReason)
	}

	// x's estimate moves to 10:50 at 10:15: it leaves the list and, y
	// joined, the batch starts naming it.
	bed, source = startBed(t, ten)
	source.picture = picture(board.StageReview, ten, "m1c")
	tickAt(t, bed, ten)
	joinUnit(t, bed, "goal-y", ten.Add(12*time.Minute))
	source.picture.Cards = []board.Card{picture(board.StageBuild, ten.Add(15*time.Minute), "m1c").Cards[0]}
	source.picture.Cards[0].Job = &board.Job{ID: "j", Phase: "handshake"}
	record = tickAt(t, bed, ten.Add(15*time.Minute))
	if bed.launches != 1 || !strings.Contains(record.StartReason, "goal-x on m1b now expected in ~41 min, later than a separate proof (~40 min)") {
		t.Fatalf("x out of reach: launches %d reason %q", bed.launches, record.StartReason)
	}

	// x handed over to m1c at 10:15: the source card is released, the
	// target claimed-idle; the wait follows the target's card when it moves.
	bed, source = startBed(t, ten)
	source.picture = picture(board.StageReview, ten, "m1c")
	tickAt(t, bed, ten)
	released := boardCardAt("goal-x", "m1b", board.StageReleased, ten.Add(15*time.Minute))
	target := boardCardAt("goal-x", "m1c", board.StageLandReady, ten.Add(15*time.Minute))
	y := picture(board.StageReview, ten, "m1c").Cards[1]
	source.picture.Cards = []board.Card{released, target, y}
	record = tickAt(t, bed, ten.Add(15*time.Minute))
	var seats []string
	for _, waited := range record.Wait.For {
		seats = append(seats, waited.Goal+"@"+waited.Seat)
	}
	if bed.launches != 0 || strings.Join(seats, ",") != "goal-x@m1c,goal-y@m1c" {
		t.Fatalf("after the handover the wait names %v", seats)
	}
}

// TestUnreadableBoardFallsBackToTheMaxWait (R22, U10b-2): with the registry
// unreadable the timer decides exactly as before and the line names the
// reason and the max wait; with a readable empty board nothing waits.
func TestUnreadableBoardFallsBackToTheMaxWait(t *testing.T) {
	t.Parallel()
	bed, source := startBed(t, ten)
	source.picture = BoardPicture{Reason: "registry: permission denied"}
	record := tickAt(t, bed, ten.Add(30*time.Second))
	if bed.launches != 0 || record.Wait == nil || record.Wait.Fallback != "registry: permission denied" {
		t.Fatalf("before the max wait: launches %d wait %+v", bed.launches, record.Wait)
	}
	if line := WaitLine(record, ten.Add(30*time.Second), time.UTC); line != "batch "+testBatchID+" waits: board unreadable (registry: permission denied); starts at the max wait, 10:01" {
		t.Fatalf("fallback wait line %q", line)
	}
	record = tickAt(t, bed, ten.Add(time.Minute))
	if bed.launches != 1 || bed.window != "expired" || record.StartReason != "board unreadable (registry: permission denied); waited the max wait (1 min)" {
		t.Fatalf("at the max wait: launches %d window %q reason %q", bed.launches, bed.window, record.StartReason)
	}
	if line := WaitLine(record, ten.Add(time.Minute), time.UTC); line != "batch "+testBatchID+" started: board unreadable (registry: permission denied); waited the max wait (1 min)" {
		t.Fatalf("started line %q", line)
	}
	// A readable empty board never waits for the timer.
	readable, _ := startBed(t, ten)
	if tickAt(t, readable, ten.Add(30*time.Second)); readable.launches != 1 {
		t.Fatalf("a readable empty board waited: launches %d", readable.launches)
	}
}

// TestWaitLineIsOneLineEverywhereAndNamesSeats (R23, U10b-2's half): one
// line, in local time, naming each waited unit's seat, stage, round of
// limit and expected time, the proof cost and its basis.
func TestWaitLineIsOneLineEverywhereAndNamesSeats(t *testing.T) {
	t.Parallel()
	three := 3
	record := Record{BatchID: "B", State: StateOpen, Wait: &WaitState{Reason: "r", ProofCost: 40 * time.Minute, Basis: "measured, n=8", For: []Waited{
		{Goal: "goal-x", Seat: "m1b", Stage: board.StageReview, Round: &board.Round{N: 2, Max: &three}, ExpectedAt: ten.Add(8 * time.Minute)},
		{Goal: "goal-y", Seat: "m1c", Stage: board.StageUnitProof, Proof: &board.Proof{Done: 120, Planned: 189}, ExpectedAt: ten.Add(6 * time.Minute)},
	}}}
	want := "batch B waits for goal-x on m1b (review round 2 of 3, ~8 min) and goal-y on m1c (unit proof 120 of 189, ~6 min); a separate proof costs ~40 min (measured, n=8)"
	if line := WaitLine(record, ten, time.UTC); line != want || strings.Contains(line, "\n") {
		t.Fatalf("wait line\n%q\nwant\n%q", line, want)
	}
	cest := time.FixedZone("CEST", 2*60*60)
	fallback := Record{BatchID: "B", State: StateOpen, Wait: &WaitState{Reason: "f", Fallback: "registry: gone", Until: ten}}
	if line := WaitLine(fallback, ten, cest); !strings.HasSuffix(line, "starts at the max wait, 12:00") {
		t.Fatalf("times are local: %q", line)
	}
	started := Record{BatchID: "B", State: StateProving, StartReason: "goal-y on m1c joined, the last unit waited for"}
	if line := WaitLine(started, ten, time.UTC); line != "batch B started: goal-y on m1c joined, the last unit waited for" {
		t.Fatalf("started line %q", line)
	}
}

// openLedger is a ledger owner whose open register entries a test sets.
type openLedger []OpenEntry

func (openLedger) Record(string, TrunkRed) ([]EntryRef, error) { return nil, nil }
func (openLedger) Clear(string, EntryRef, Green) error         { return nil }
func (ledger openLedger) Open() ([]OpenEntry, error)           { return ledger, nil }

// TestRedOnMainStartsAtOnce (R22, U10b-2): a joined unit that is the fix goal
// of an open trunk-red entry starts its batch at the first tick although a
// unit on m1b is 3 min away; a known-flake entry naming the goal does not;
// an entry whose fix branch is the goal's but whose fix goal is another
// takes the ordinary path; with the cap refusing, the record carries cap
// and the line says the batch is first in line.
func TestRedOnMainStartsAtOnce(t *testing.T) {
	t.Parallel()
	near := BoardPicture{Readable: true, Cards: []board.Card{boardCardAt("goal-n", "m1b", board.StageLandReady, ten)}}
	for _, row := range []struct {
		name   string
		entry  OpenEntry
		starts bool
	}{
		{"trunk red fixed by the joined goal", OpenEntry{ID: "E1", Class: ClassTrunkRed, FixGoal: "goal-a"}, true},
		{"a known flake naming the goal", OpenEntry{ID: "E2", Class: ClassKnownFlake, FixGoal: "goal-a"}, false},
		{"another goal's fix on this goal's branch", OpenEntry{ID: "E3", Class: ClassTrunkRed, FixGoal: "goal-h"}, false},
	} {
		bed, source := startBed(t, ten)
		source.picture = near
		bed.owner.store = bed.owner.store.WithLedgerOwner(openLedger{row.entry})
		record := tickAt(t, bed, ten)
		if row.starts != (bed.launches == 1) {
			t.Fatalf("%s: launches %d reason %q wait %+v", row.name, bed.launches, record.StartReason, record.Wait)
		}
		if row.starts && (bed.window != WindowRedOnMain || record.StartReason != "red on main (goal-a fixes incident E1)") {
			t.Fatalf("%s: window %q reason %q", row.name, bed.window, record.StartReason)
		}
		if !row.starts && (record.Wait == nil || record.Wait.For[0].Goal != "goal-n") {
			t.Fatalf("%s: the ordinary path waits for goal-n: %+v", row.name, record.Wait)
		}
	}
	bed, source := startBed(t, ten)
	source.picture = near
	bed.owner.store = bed.owner.store.WithLedgerOwner(openLedger{{ID: "E1", Class: ClassTrunkRed, FixGoal: "goal-a"}})
	bed.owner.runners = func(proofrun.LoadSample, proofrun.AdmissionCap) []RunnerCapacity {
		return []RunnerCapacity{{Runner: "host", Ceiling: 1, Overlapping: 1}}
	}
	record := tickAt(t, bed, ten)
	if bed.launches != 0 || lastHistory(record).Verb != "cap" {
		t.Fatalf("the cap refused: launches %d history %+v", bed.launches, lastHistory(record))
	}
	if line := WaitLine(record, ten, time.UTC); line != "batch "+testBatchID+" started: red on main (goal-a fixes incident E1); first in line for a slot" {
		t.Fatalf("cap line %q", line)
	}
}
