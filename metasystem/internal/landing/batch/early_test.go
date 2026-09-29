package batch

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter/fakeadapter"
)

// earlyWitness is an owner over one waiting batch: goal-a (changes the fake's
// payments) and goal-b (changes orders) joined at 10:00 on base "base" with
// their recorded tip "tip-ab", and goal-y on m1c in judgement within reach,
// so every tick decides a wait. The early seams count their calls.
type earlyWitness struct {
	*ownerBed
	source      *scriptedBoard
	cheap       []string
	cheapResult EarlyResult
	cheapErr    error
	diagnostics []DiagnosticRequest
	base        []DiagnosticResult
	logged      []string
}

func newEarlyWitness(t *testing.T) *earlyWitness {
	t.Helper()
	bed, source := startBed(t, ten)
	w := &earlyWitness{ownerBed: bed, source: source, cheapResult: EarlyResult{Attempt: "cheap-1"}}
	must(t, bed.store.Update(testBatchID, func(record *Record) error {
		record.BaseTree, record.PrefixTrees, record.TipTree = "base", []string{"tip-a", "tip-ab"}, "tip-ab"
		record.Units[0].Closure = fakeClosure([]string{"payments"}, nil)
		record.Units = append(record.Units, Unit{GoalID: "goal-b", Chain: "chain-goal-b", Claim: Claim{Machine: "landing", Lineage: "goal-b", Epoch: 1, Revision: 2, AccountingRevision: 1},
			State: UnitJoined, Closure: fakeClosure([]string{"orders"}, nil)})
		appendUnitHistory(record, ten, "join", "seat", "goal-b", "", UnitJoined)
		return nil
	}))
	source.picture = BoardPicture{Readable: true, Cards: []board.Card{boardCardAt("goal-y", "m1c", board.StageJudgement, ten)}}
	// The survivors of a reassembly are named by their members' letters.
	bed.store.reassembly.assemble = func(_ string, units []Unit) ([]string, error) {
		tip, prefixes := "tip-", []string{}
		for _, unit := range units {
			tip += strings.TrimPrefix(unit.GoalID, "goal-")
			prefixes = append(prefixes, tip)
		}
		return prefixes, nil
	}
	bed.owner.store = bed.store
	bed.owner.early = EarlySeams{
		Cheap: func(record Record) (EarlyResult, error) {
			w.cheap = append(w.cheap, record.TipTree)
			return w.cheapResult, w.cheapErr
		},
		Adapter: fakeRedLanguage(fakeadapter.New()),
	}
	bed.owner.logWait = func(_ string, line string) { w.logged = append(w.logged, line) }
	bed.owner.runDiagnostic = func(_ string, request DiagnosticRequest, _ Claim) (DiagnosticResult, error) {
		w.diagnostics = append(w.diagnostics, request)
		if len(w.base) == 0 {
			return DiagnosticResult{AttemptID: "base-run"}, nil
		}
		result := w.base[0]
		w.base = w.base[1:]
		return result, nil
	}
	return w
}

func (w *earlyWitness) tick(t *testing.T, at time.Time) Record {
	t.Helper()
	return tickAt(t, w.ownerBed, at)
}

func (w *earlyWitness) recordBytes(t *testing.T) string {
	t.Helper()
	path, _ := w.store.recordPath(testBatchID)
	return string(contents(t, path))
}

func historyWith(record Record, verb, detail string) bool {
	return slices.ContainsFunc(record.History, func(entry HistoryEntry) bool { return entry.Verb == verb && entry.Detail == detail })
}

var loadedHost = proofrun.LoadSample{OverlapKnown: true, OverlappingHost: 1, Available: true, Cores: 18, Load1m: 14.2}

// TestWaitIsUsedOnlyOnSpareCapacity (R27, U10b-3): the tick that records the
// wait runs the join's cheap phase once on the tip the joins recorded; a
// loaded host runs nothing and the line says so; a sealed batch the cap
// refused waits for CPU and runs nothing; and the start comes at the same
// tick whether the early acts run or not.
func TestWaitIsUsedOnlyOnSpareCapacity(t *testing.T) {
	t.Parallel()
	w := newEarlyWitness(t)
	record := w.tick(t, ten)
	if w.launches != 0 || record.Wait == nil || !slices.Equal(w.cheap, []string{"tip-ab"}) || record.Early == nil ||
		!slices.Equal(record.Early.Shape, []string{"goal-a", "goal-b"}) || record.Early.Tree != "tip-ab" || record.Early.Cheap != earlyGreen {
		t.Fatalf("the wait's tick: launches %d cheap %v early %+v", w.launches, w.cheap, record.Early)
	}
	before := w.recordBytes(t)
	w.tick(t, ten.Add(time.Minute))
	if len(w.cheap) != 1 || w.recordBytes(t) != before {
		t.Fatalf("a repeated wait ran the cheap phase again (%v) or wrote the record", w.cheap)
	}

	// A loaded host: nothing runs, and the line says why.
	w = newEarlyWitness(t)
	w.sample = loadedHost
	record = w.tick(t, ten)
	if len(w.cheap) != 0 || record.Wait == nil || !strings.Contains(WaitLine(record, ten, time.UTC), "; meanwhile: nothing (host loaded: load 14.2 on 18 cores)") {
		t.Fatalf("loaded host: cheap %v line %q", w.cheap, WaitLine(record, ten, time.UTC))
	}
	before = w.recordBytes(t)
	w.sample.Load1m = 15.1
	w.tick(t, ten.Add(time.Minute))
	if w.recordBytes(t) != before {
		t.Fatal("a changed load alone wrote the record")
	}

	// A sealed batch the cap refused waits for CPU, not knowledge.
	w = newEarlyWitness(t)
	must(t, w.store.Update(testBatchID, func(record *Record) error { record.State = StateSealed; return nil }))
	w.source.picture = BoardPicture{Readable: true}
	w.owner.runners = func(proofrun.LoadSample, proofrun.AdmissionCap) []RunnerCapacity {
		return []RunnerCapacity{{Runner: "host", Cores: 18, LoadKnown: true, Overlapping: 3, Ceiling: 3}}
	}
	record = w.tick(t, ten)
	if w.launches != 0 || len(w.cheap) != 0 || record.Early != nil || !strings.Contains(WaitLine(record, ten, time.UTC), "first in line") {
		t.Fatalf("capped sealed batch: launches %d cheap %v early %+v line %q", w.launches, w.cheap, record.Early, WaitLine(record, ten, time.UTC))
	}

	// The start tick is the same with the early acts as without them.
	startTick := func(active bool) int {
		w := newEarlyWitness(t)
		if !active {
			w.owner.early.Cheap = func(Record) (EarlyResult, error) { return EarlyResult{}, errors.New("stubbed") }
		}
		for tick := 0; tick < 20; tick++ {
			at := ten.Add(time.Duration(tick) * time.Minute)
			if tick == 13 {
				joinUnit(t, w.ownerBed, "goal-y", at)
			}
			w.tick(t, at)
			if w.launches != 0 {
				return tick
			}
		}
		return -1
	}
	if with, without := startTick(true), startTick(false); with != 13 || without != 13 {
		t.Fatalf("the start tick moved: with the early acts %d, without %d", with, without)
	}
}

// TestPartialRedIsJudgedByTheLanesRules (R27, R1, R2, U10b-3, with the fake
// adapter): a red of the partial tip is judged by D1's first two steps
// against the members of its own shape; the classification never runs.
func TestPartialRedIsJudgedByTheLanesRules(t *testing.T) {
	t.Parallel()
	paymentRed := EarlyResult{Attempt: "cheap-1", Failing: []RedGroup{{ID: "fake/pay", LogPath: "/logs/pay.log",
		Failures: []Failure{{Classname: "com.example.PaymentTest", Name: "testCharge"}}}}}

	// Base green, the failure owned by payments, which goal-a alone changed:
	// goal-a is ejected now, the survivors reassemble, the wait goes on and
	// the acts start again over them.
	w := newEarlyWitness(t)
	w.cheapResult = paymentRed
	record := w.tick(t, ten)
	if len(w.diagnostics) != 1 || w.diagnostics[0].Tree != "base" || !slices.Equal(w.diagnostics[0].Groups, []string{"fake/pay"}) || !w.diagnostics[0].NeverReuse {
		t.Fatalf("the base check: %+v", w.diagnostics)
	}
	a := record.Units[0]
	want := "EJECTED from landing batch " + testBatchID + " before its proof: testCharge (fake/pay) failed on the batch's partial tip (goal-a, goal-b) and passes on main; log /logs/pay.log. Fix it, then metasystem work land goal-a."
	if a.GoalID != "goal-a" || a.State != UnitReturnPending || a.Outcome != UnitEjected || a.Failure != want || record.Units[1].State != UnitJoined ||
		record.State != StateOpen || record.TipTree != "tip-b" || record.Early != nil || !historyWith(record, "early-forget", "goal-a ejected") {
		t.Fatalf("named member: state %s tip %s early %+v units %+v", record.State, record.TipTree, record.Early, record.Units)
	}
	w.cheapResult = EarlyResult{Attempt: "cheap-2"}
	record = w.tick(t, ten.Add(time.Minute))
	if w.launches != 0 || !slices.Equal(w.cheap, []string{"tip-ab", "tip-b"}) || record.Early == nil || !slices.Equal(record.Early.Shape, []string{"goal-b"}) {
		t.Fatalf("the acts did not start again over the survivors: cheap %v early %+v", w.cheap, record.Early)
	}

	// Base red twice: a trunk red, held and registered as today.
	w = newEarlyWitness(t)
	w.cheapResult = paymentRed
	red := DiagnosticResult{AttemptID: "base-red", Groups: []RedGroup{{ID: "fake/pay", Status: "failed"}}}
	w.base = []DiagnosticResult{red, red}
	var registered []string
	w.store = w.store.WithLedgerOwner(recordOwner(func(opid string, red TrunkRed) ([]EntryRef, error) {
		registered = append(registered, opid)
		return []EntryRef{{ID: "entry-1", Group: "fake/pay"}}, nil
	}))
	w.owner.store = w.store
	record = w.tick(t, ten)
	if record.State != StateHeldTrunkRed || record.TrunkRed == nil || len(record.TrunkRed.Entries) != 1 || len(registered) != 1 ||
		len(w.diagnostics) != 2 || !slices.Equal(w.diagnostics[1].Fresh, []string{"--rerun-fake"}) {
		t.Fatalf("base red twice: state %s trunk %+v registered %v diagnostics %+v", record.State, record.TrunkRed, registered, w.diagnostics)
	}

	// Base green, nobody named: a finding on the record and in the line;
	// every member stays joined, the batch open, no classification runs.
	w = newEarlyWitness(t)
	w.cheapResult = EarlyResult{Attempt: "cheap-1", Failing: []RedGroup{{ID: "fake/other", LogPath: "/logs/other.log",
		Failures: []Failure{{Classname: "com.example.OtherTest", Name: "testOther"}}}}}
	record = w.tick(t, ten)
	if record.State != StateOpen || record.Units[0].State != UnitJoined || record.Units[1].State != UnitJoined || len(w.diagnostics) != 1 ||
		record.Early == nil || record.Early.Finding == nil || *record.Early.Finding != (EarlyFinding{Group: "fake/other", Attempt: "cheap-1", Log: "/logs/other.log"}) {
		t.Fatalf("nobody named: state %s units %+v diagnostics %d early %+v", record.State, record.Units, len(w.diagnostics), record.Early)
	}

	// Red then green on main: nobody named, and nothing enters the register.
	w = newEarlyWitness(t)
	w.cheapResult = paymentRed
	w.base = []DiagnosticResult{red, {AttemptID: "base-green"}}
	registered = nil
	w.store = w.store.WithLedgerOwner(recordOwner(func(opid string, _ TrunkRed) ([]EntryRef, error) {
		registered = append(registered, opid)
		return nil, nil
	}))
	w.owner.store = w.store
	record = w.tick(t, ten)
	if record.State != StateOpen || record.Early == nil || record.Early.Finding == nil || len(registered) != 0 || record.Units[0].State != UnitJoined {
		t.Fatalf("red then green: state %s early %+v registered %v", record.State, record.Early, registered)
	}
}

// TestEarlyProofIsAbandonedAndReusedOnlyByIdentity (R27, R10, U10b-3): the
// early work is forgotten, with one history line naming the cause, when the
// base moves or a member leaves; a join grows the shape and the acts start
// again over it; the batch starting ends the acts and keeps what they did.
func TestEarlyProofIsAbandonedAndReusedOnlyByIdentity(t *testing.T) {
	t.Parallel()
	w := newEarlyWitness(t)
	w.tick(t, ten)
	must(t, ReopenMovedBase(w.store, testBatchID, "base-2", "", "owner", ten.Add(time.Minute)))
	record := load(t, w.store)
	if record.Early != nil || !historyWith(record, "early-forget", "base moved to base-2") {
		t.Fatalf("base moved: early %+v", record.Early)
	}

	w = newEarlyWitness(t)
	w.tick(t, ten)
	must(t, ReassembleSurvivorsWithReturns(w.store, testBatchID, "owner", ten.Add(time.Minute), []ReturnDecision{{GoalID: "goal-b", Outcome: UnitEjected, Reason: "red"}}))
	if record = load(t, w.store); record.Early != nil || !historyWith(record, "early-forget", "goal-b ejected") {
		t.Fatalf("member ejected: early %+v", record.Early)
	}

	w = newEarlyWitness(t)
	w.tick(t, ten)
	must(t, RequestReturn(w.store, testBatchID, "goal-b", UnitWithdrawnBudget, "budget", "owner", ten.Add(time.Minute)))
	if record = load(t, w.store); record.Early != nil || !historyWith(record, "early-forget", "goal-b withdrawn") {
		t.Fatalf("member withdrawn: early %+v", record.Early)
	}

	// goal-c joins: the tip grows and the acts start again over it.
	w = newEarlyWitness(t)
	w.tick(t, ten)
	joinUnit(t, w.ownerBed, "goal-c", ten.Add(2*time.Minute))
	must(t, w.store.Update(testBatchID, func(record *Record) error {
		record.PrefixTrees, record.TipTree = append(record.PrefixTrees, "tip-abc"), "tip-abc"
		return nil
	}))
	record = w.tick(t, ten.Add(2*time.Minute))
	if !slices.Equal(w.cheap, []string{"tip-ab", "tip-abc"}) || record.Early == nil || !slices.Equal(record.Early.Shape, []string{"goal-a", "goal-b", "goal-c"}) {
		t.Fatalf("grown shape: cheap %v early %+v", w.cheap, record.Early)
	}

	// The batch starts: the acts end and the record keeps what they did.
	joinUnit(t, w.ownerBed, "goal-y", ten.Add(13*time.Minute))
	record = w.tick(t, ten.Add(13*time.Minute))
	if w.launches != 1 || record.Early == nil || record.Early.Ended != "batch started" || record.Early.Cheap != earlyGreen ||
		!historyWith(record, "early-end", "batch started") || len(w.cheap) != 2 {
		t.Fatalf("start: launches %d early %+v cheap %v", w.launches, record.Early, w.cheap)
	}
}

// TestWaitLineSaysWhatTheWaitIsUsedFor (R27, R23, U10b-3): the meanwhile
// clause in each of its shapes, logged once per change; a repeated tick with
// unchanged state writes nothing.
func TestWaitLineSaysWhatTheWaitIsUsedFor(t *testing.T) {
	t.Parallel()
	w := newEarlyWitness(t)
	record := w.tick(t, ten)
	line := WaitLine(record, ten, time.UTC)
	if !strings.HasSuffix(line, "; meanwhile: goal-a+goal-b: cheap checks green") || len(w.logged) != 2 || w.logged[1] != line ||
		!historyWith(record, "meanwhile", line) {
		t.Fatalf("cheap green: line %q logged %q", line, w.logged)
	}
	w.tick(t, ten.Add(time.Minute))
	if len(w.logged) != 2 {
		t.Fatalf("an unchanged tick logged again: %q", w.logged)
	}

	w = newEarlyWitness(t)
	w.cheapResult = EarlyResult{Attempt: "cheap-1", Failing: []RedGroup{{ID: "fake/other", Failures: []Failure{{Classname: "com.example.OtherTest", Name: "testOther"}}}}}
	record = w.tick(t, ten)
	if line = WaitLine(record, ten, time.UTC); !strings.HasSuffix(line, "; meanwhile: partial red: fake/other on goal-a+goal-b, nobody named; decided at the batch proof") {
		t.Fatalf("finding: %q", line)
	}

	w = newEarlyWitness(t)
	w.cheapErr = errors.New("engine build refused")
	record = w.tick(t, ten)
	if line = WaitLine(record, ten, time.UTC); !strings.HasSuffix(line, "; meanwhile: cheap checks unavailable: engine build refused") {
		t.Fatalf("unavailable: %q", line)
	}
}
