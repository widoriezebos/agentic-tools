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
	started     chan string
	proveResult EarlyResult
	proveErr    error
	gate        chan struct{}
	budgetWhy   string
}

// spare gives the host runner room for the batch proof beside an early one.
func spare(ceiling int) func(proofrun.LoadSample, proofrun.AdmissionCap) []RunnerCapacity {
	return func(proofrun.LoadSample, proofrun.AdmissionCap) []RunnerCapacity {
		return []RunnerCapacity{{Runner: "host", Cores: 18, Load: 1, LoadKnown: true, Ceiling: ceiling}}
	}
}

// early is the number of early proofs of this owner in flight: an early
// proof is launched exactly when one is recorded, in the tick.
func (w *earlyWitness) early() int { return len(w.owner.earlyRuns) }

// finishEarly lets the early proof in flight end and applies its completion.
func (w *earlyWitness) finishEarly(t *testing.T) {
	t.Helper()
	w.gate <- struct{}{}
	w.owner.CompleteEarly(<-w.owner.EarlyCompletions())
}

func newEarlyWitness(t *testing.T) *earlyWitness {
	t.Helper()
	bed, source := startBed(t, ten)
	w := &earlyWitness{ownerBed: bed, source: source, cheapResult: EarlyResult{Attempt: "cheap-1"}, proveResult: EarlyResult{Attempt: "early-1"},
		gate: make(chan struct{}, 1), started: make(chan string, 8)}
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
		Prove: func(record Record) (EarlyResult, error) {
			w.started <- record.TipTree
			<-w.gate
			return w.proveResult, w.proveErr
		},
		Budget:  func(Record) (bool, string) { return w.budgetWhy == "", w.budgetWhy },
		Adapter: fakeRedLanguage(fakeadapter.New()),
	}
	// By default the host has no room beside the batch proof: a witness of
	// the early proof gives it some.
	bed.owner.runners = spare(1)
	t.Cleanup(func() { close(w.gate) })
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

// TestWaitIsUsedOnlyOnSpareCapacity (R27, U3-01, U10b-3, artificial clock,
// scripted board, scripted sample, the real Owner.Tick with counting
// seams): with the runner admitting two more proofs, the tick that records
// the wait runs the join's cheap phase on the recorded tip and launches
// exactly one early proof of it; admitting exactly one more, the cheap phase
// and no early proof; a loaded host, nothing; a second waiting batch of the
// same owner, no second early proof; a head budget that could not keep the
// batch proof's headroom, no early proof, and the batch proof still starts;
// a sealed batch the cap refused, nothing; and the start comes at the same
// tick with the early acts as without them.
func TestWaitIsUsedOnlyOnSpareCapacity(t *testing.T) {
	t.Parallel()
	w := newEarlyWitness(t)
	w.owner.runners = spare(3)
	record := w.tick(t, ten)
	if w.launches != 0 || record.Wait == nil || !slices.Equal(w.cheap, []string{"tip-ab"}) || w.early() != 1 || <-w.started != "tip-ab" ||
		record.Early == nil || !slices.Equal(record.Early.Shape, []string{"goal-a", "goal-b"}) || record.Early.Tree != "tip-ab" ||
		record.Early.Cheap != earlyGreen || record.Early.Proof != earlyRunning || lockHeld(w.ownerBed, testBatchID) {
		t.Fatalf("the wait's tick: launches %d cheap %v early proofs %d early %+v", w.launches, w.cheap, w.early(), record.Early)
	}
	before := w.recordBytes(t)
	w.tick(t, ten.Add(time.Minute))
	if len(w.cheap) != 1 || w.early() != 1 || w.recordBytes(t) != before {
		t.Fatalf("a repeated wait ran again (cheap %v, early proofs %d) or wrote the record", w.cheap, w.early())
	}
	w.finishEarly(t)
	record = load(t, w.store)
	if record.Early.Proof != earlyGreen || record.Early.Attempt != "early-1" || record.Proof != nil {
		t.Fatalf("the early proof's end: early %+v proof %+v", record.Early, record.Proof)
	}
	w.tick(t, ten.Add(2*time.Minute))
	if w.early() != 0 {
		t.Fatal("a second early proof of the same tip started")
	}

	// A real proof that arrives while an early proof holds a slot queues as
	// any run queues: the early proof counts as one of this owner's runs.
	w = newEarlyWitness(t)
	w.owner.runners = spare(3)
	w.tick(t, ten)
	w.owner.runners = func(proofrun.LoadSample, proofrun.AdmissionCap) []RunnerCapacity {
		return []RunnerCapacity{{Runner: "host", Cores: 18, Load: 1, LoadKnown: true, Overlapping: 1, Ceiling: 2}}
	}
	joinUnit(t, w.ownerBed, "goal-y", ten.Add(13*time.Minute))
	record = w.tick(t, ten.Add(13*time.Minute))
	if w.launches != 0 || lastHistory(record).Verb != "cap" || !strings.Contains(WaitLine(record, ten, time.UTC), "first in line") {
		t.Fatalf("a real proof beside the early one's slot: launches %d last %+v", w.launches, lastHistory(record))
	}
	w.finishEarly(t)
	if w.tick(t, ten.Add(14*time.Minute)); w.launches != 1 {
		t.Fatalf("the real proof did not start once the early one ended: launches %d", w.launches)
	}

	// Room for exactly one more real proof: the cheap phase, no early proof.
	w = newEarlyWitness(t)
	record = w.tick(t, ten)
	if len(w.cheap) != 1 || w.early() != 0 || !strings.HasSuffix(WaitLine(record, ten, time.UTC), "; meanwhile: goal-a+goal-b: cheap checks green; no spare proof slot") {
		t.Fatalf("one slot: cheap %v early proofs %d line %q", w.cheap, w.early(), WaitLine(record, ten, time.UTC))
	}

	// A loaded host: nothing runs, and the line says why.
	w = newEarlyWitness(t)
	w.owner.runners = spare(3)
	w.sample = loadedHost
	record = w.tick(t, ten)
	if len(w.cheap) != 0 || w.early() != 0 || record.Wait == nil ||
		!strings.HasSuffix(WaitLine(record, ten, time.UTC), "; meanwhile: nothing (host loaded: load 14.2 on 18 cores)") {
		t.Fatalf("loaded host: cheap %v early proofs %d line %q", w.cheap, w.early(), WaitLine(record, ten, time.UTC))
	}
	before = w.recordBytes(t)
	w.sample.Load1m = 15.1
	w.tick(t, ten.Add(time.Minute))
	if w.recordBytes(t) != before {
		t.Fatal("a changed load alone wrote the record")
	}

	// A second waiting batch of the same owner: one early proof at a time.
	w = newEarlyWitness(t)
	w.owner.runners = spare(3)
	const second = "01j5x00000000000000000ba02"
	other := load(t, w.store)
	other.BatchID, other.History, other.Early = second, other.History[:0:0], nil
	for _, unit := range other.Units {
		appendUnitHistory(&other, ten, "join", "seat", unit.GoalID, "", UnitJoined)
	}
	must(t, w.store.Create(other))
	w.tick(t, ten)
	w.now = ten
	must(t, w.owner.Tick(second))
	if otherRecord, err := w.store.Load(second); err != nil || w.early() != 1 || len(w.cheap) != 2 || otherRecord.Early == nil ||
		otherRecord.Early.Proof != "" || otherRecord.Early.Idle != noSpareSlot {
		t.Fatalf("second batch: early proofs %d cheap %v early %+v err %v", w.early(), w.cheap, otherRecord.Early, err)
	}

	// The head's budget could not keep the batch proof's headroom (U3-01):
	// no early proof, the line says why, and the batch proof still starts.
	w = newEarlyWitness(t)
	w.owner.runners = spare(3)
	w.budgetWhy = "no early proof: goal-b has 2 attempts and 900 reserved minutes left, kept for the batch proof"
	record = w.tick(t, ten)
	if w.early() != 0 || !strings.HasSuffix(WaitLine(record, ten, time.UTC), "; meanwhile: goal-a+goal-b: cheap checks green; "+w.budgetWhy) {
		t.Fatalf("budget: early proofs %d line %q", w.early(), WaitLine(record, ten, time.UTC))
	}
	joinUnit(t, w.ownerBed, "goal-y", ten.Add(13*time.Minute))
	if w.tick(t, ten.Add(13*time.Minute)); w.launches != 1 {
		t.Fatalf("the batch proof did not start after the early proof was skipped: launches %d", w.launches)
	}

	// A sealed batch the cap refused waits for CPU, not knowledge.
	w = newEarlyWitness(t)
	must(t, w.store.Update(testBatchID, func(record *Record) error { record.State = StateSealed; return nil }))
	w.source.picture = BoardPicture{Readable: true}
	w.owner.runners = func(proofrun.LoadSample, proofrun.AdmissionCap) []RunnerCapacity {
		return []RunnerCapacity{{Runner: "host", Cores: 18, LoadKnown: true, Overlapping: 3, Ceiling: 3}}
	}
	record = w.tick(t, ten)
	if w.launches != 0 || len(w.cheap) != 0 || w.early() != 0 || record.Early != nil || !strings.Contains(WaitLine(record, ten, time.UTC), "first in line") {
		t.Fatalf("capped sealed batch: launches %d cheap %v early %+v line %q", w.launches, w.cheap, record.Early, WaitLine(record, ten, time.UTC))
	}

	// The start tick is the same with the early acts, an early proof still
	// in flight, as without them.
	startTick := func(active bool) int {
		w := newEarlyWitness(t)
		w.owner.runners = spare(3)
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
				if active && w.early() != 1 {
					t.Fatalf("no early proof was in flight at the start: %d", w.early())
				}
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
	// (Checked on the grown batch below, after its own acts.)
	joinUnit(t, w.ownerBed, "goal-y", ten.Add(13*time.Minute))
	record = w.tick(t, ten.Add(13*time.Minute))
	if w.launches != 1 || record.Early == nil || record.Early.Ended != "batch started" || record.Early.Cheap != earlyGreen ||
		!historyWith(record, "early-end", "batch started") || len(w.cheap) != 2 {
		t.Fatalf("start: launches %d early %+v cheap %v", w.launches, record.Early, w.cheap)
	}
}

// TestEarlyProofIsAbandonedNotCancelled (R27, U10b-3): an early proof in
// flight when the base moves is forgotten with the move and ends on its own,
// and its end writes nothing; one whose batch grows by a join keeps running,
// its tree the grown series' prefix, no second early proof starts until it
// ends, and the acts then start again over the grown tip; the start line
// names how many of the batch proof's groups came from it.
func TestEarlyProofIsAbandonedNotCancelled(t *testing.T) {
	t.Parallel()
	w := newEarlyWitness(t)
	w.owner.runners = spare(3)
	w.tick(t, ten)
	must(t, ReopenMovedBase(w.store, testBatchID, "base-2", "", "owner", ten.Add(time.Minute)))
	record := load(t, w.store)
	if record.Early != nil || !historyWith(record, "early-forget", "base moved to base-2") || w.early() != 1 {
		t.Fatalf("base moved under an early proof: early %+v in flight %d", record.Early, w.early())
	}
	before := w.recordBytes(t)
	w.finishEarly(t)
	if w.recordBytes(t) != before || w.early() != 0 {
		t.Fatal("an abandoned early proof's end wrote the record")
	}

	w = newEarlyWitness(t)
	w.owner.runners = spare(3)
	w.tick(t, ten)
	<-w.started
	joinUnit(t, w.ownerBed, "goal-c", ten.Add(2*time.Minute))
	must(t, w.store.Update(testBatchID, func(record *Record) error {
		record.PrefixTrees, record.TipTree = append(record.PrefixTrees, "tip-abc"), "tip-abc"
		return nil
	}))
	record = w.tick(t, ten.Add(2*time.Minute))
	if w.early() != 1 || len(w.cheap) != 1 || record.Early == nil || record.Early.Tree != record.PrefixTrees[1] || record.Early.Proof != earlyRunning {
		t.Fatalf("a join under an early proof: in flight %d cheap %v early %+v", w.early(), w.cheap, record.Early)
	}
	w.finishEarly(t)
	if record = load(t, w.store); record.Early.Attempt != "early-1" || record.Early.Proof != earlyGreen {
		t.Fatalf("the prefix's early proof ended unrecorded: %+v", record.Early)
	}
	w.proveResult = EarlyResult{Attempt: "early-2"}
	record = w.tick(t, ten.Add(3*time.Minute))
	if !slices.Equal(w.cheap, []string{"tip-ab", "tip-abc"}) || w.early() != 1 || <-w.started != "tip-abc" ||
		!slices.Equal(record.Early.Shape, []string{"goal-a", "goal-b", "goal-c"}) {
		t.Fatalf("the acts did not start again over the grown tip: cheap %v early %+v", w.cheap, record.Early)
	}

	// A red that ends after the batch started (its proof queued for a slot)
	// is judged no more and is kept for the batch proof (U3-03).
	w = newEarlyWitness(t)
	w.owner.runners = spare(3)
	w.proveResult = EarlyResult{Attempt: "early-1", Failing: []RedGroup{{ID: "fake/pay", Failures: []Failure{{Classname: "com.example.PaymentTest", Name: "testCharge"}}}}}
	w.tick(t, ten)
	w.owner.runners = func(proofrun.LoadSample, proofrun.AdmissionCap) []RunnerCapacity {
		return []RunnerCapacity{{Runner: "host", Cores: 18, Load: 1, LoadKnown: true, Overlapping: 1, Ceiling: 2}}
	}
	joinUnit(t, w.ownerBed, "goal-y", ten.Add(13*time.Minute))
	w.tick(t, ten.Add(13*time.Minute))
	w.finishEarly(t)
	record = load(t, w.store)
	if len(w.diagnostics) != 0 || slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) ||
		record.Early.Finding == nil || record.Early.Finding.Attempt != "early-1" || record.Early.Ended != "batch started" {
		t.Fatalf("a red after the start: diagnostics %d units %+v early %+v", len(w.diagnostics), record.Units, record.Early)
	}

	// The start line: the batch proof's sources name the early attempt.
	record = Record{BatchID: testBatchID, StartReason: "goal-y on m1c joined, the last unit waited for",
		Early: &Early{Attempt: "early-2", Ended: "batch started"},
		Proof: &Proof{AttemptID: "tip-1", SelectedGroups: []string{"g1", "g2", "g3", "g4"}, Sources: map[string]Source{
			"g1": {Kind: SourceReused, Attempt: "early-2"}, "g2": {Kind: SourceReused, Attempt: "early-2"}, "g3": {Kind: SourceExecuted, Attempt: "tip-1"},
			"g4": {Kind: SourceReused, Attempt: "older-1"}}}}
	if line := WaitLine(record, ten, time.UTC); line != "batch "+testBatchID+" started: goal-y on m1c joined, the last unit waited for; reusing 2 of 4 groups from the early proof" {
		t.Fatalf("start line: %q", line)
	}
}

// TestPartialRedJudgesOnlyTheMembersItsShapeHeld (U3-02): an early proof of
// goal-a and goal-b red on a test that goal-c's changes own, goal-c joining
// while it ran: goal-c is not named, nobody is ejected, and it is a finding.
func TestPartialRedJudgesOnlyTheMembersItsShapeHeld(t *testing.T) {
	t.Parallel()
	w := newEarlyWitness(t)
	w.owner.runners = spare(3)
	w.proveResult = EarlyResult{Attempt: "early-1", Failing: []RedGroup{{ID: "fake/ledger", LogPath: "/logs/ledger.log",
		Failures: []Failure{{Classname: "com.example.LedgerTest", Name: "testPost"}}}}}
	w.tick(t, ten)
	joinUnit(t, w.ownerBed, "goal-c", ten.Add(2*time.Minute))
	must(t, w.store.Update(testBatchID, func(record *Record) error {
		record.Units[2].Closure = fakeClosure([]string{"ledger"}, nil)
		record.PrefixTrees, record.TipTree = append(record.PrefixTrees, "tip-abc"), "tip-abc"
		return nil
	}))
	w.finishEarly(t)
	record := load(t, w.store)
	if len(w.diagnostics) != 1 || slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) ||
		record.Early == nil || record.Early.Finding == nil || record.Early.Finding.Attempt != "early-1" {
		t.Fatalf("a later member was judged: diagnostics %d units %+v early %+v", len(w.diagnostics), record.Units, record.Early)
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
	if !strings.HasSuffix(line, "; meanwhile: goal-a+goal-b: cheap checks green; no spare proof slot") || len(w.logged) != 2 || w.logged[1] != line ||
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
	w.owner.runners = spare(3)
	record = w.tick(t, ten)
	if line = WaitLine(record, ten, time.UTC); !strings.HasSuffix(line, "; meanwhile: goal-a+goal-b: cheap checks green, early proof running") {
		t.Fatalf("running: %q", line)
	}
	w.finishEarly(t)
	if line = WaitLine(load(t, w.store), ten, time.UTC); !strings.HasSuffix(line, "; meanwhile: goal-a+goal-b: cheap checks green, early proof green") {
		t.Fatalf("green: %q", line)
	}

	w = newEarlyWitness(t)
	w.owner.runners = spare(3)
	w.proveErr = errors.New("admission refused")
	w.tick(t, ten)
	w.finishEarly(t)
	if line = WaitLine(load(t, w.store), ten, time.UTC); !strings.HasSuffix(line, "; meanwhile: goal-a+goal-b: cheap checks green; early proof unavailable: admission refused") {
		t.Fatalf("early proof unavailable: %q", line)
	}

	w = newEarlyWitness(t)
	w.cheapErr = errors.New("engine build refused")
	record = w.tick(t, ten)
	if line = WaitLine(record, ten, time.UTC); !strings.HasSuffix(line, "; meanwhile: cheap checks unavailable: engine build refused") {
		t.Fatalf("unavailable: %q", line)
	}
}
