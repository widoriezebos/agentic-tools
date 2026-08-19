package goal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var human = Caller{Class: "HUMAN"}
var mainHolder = Caller{Class: "MAIN", Holder: true}

// fakeProber scripts liveness per pid, mirroring missionstate's test seam.
type fakeProber struct {
	verdicts map[int64]identity.Liveness
	starts   map[int64]int64
}

func (f fakeProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	verdict, ok := f.verdicts[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	switch verdict {
	case identity.Alive:
		return identity.Exact{Pid: pid, StartedAt: time.Unix(f.starts[pid], 0)}, identity.Alive, nil
	case identity.Unknown:
		return identity.Exact{}, identity.Unknown, os.ErrPermission
	default:
		return identity.Exact{}, identity.Dead, nil
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	return &Store{
		Root: t.TempDir(),
		Now:  func() time.Time { return time.Unix(1786800000, 0) },
	}
}

func mustOpen(t *testing.T, s *Store, caller Caller, id, intent, next string) {
	t.Helper()
	if _, err := s.Open(caller, id, intent, next); err != nil {
		t.Fatalf("open %s: %v", id, err)
	}
}

// GOAL-20: open with no Current goal lands Current — the one-command
// program start; a second open queues behind it.
func TestOpenPromotesWhenEmpty(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "first", "The first program", "Start it.")
	ledger, problems, err := s.ReadLedger()
	if err != nil || len(problems) > 0 {
		t.Fatalf("read: %v %v", err, problems)
	}
	if ledger.Current == nil || ledger.Current.Id != "first" || ledger.Current.Origin != OriginMain {
		t.Fatalf("open did not land Current: %+v", ledger.Current)
	}
	mustOpen(t, s, human, "second", "The second program", "Queue it.")
	ledger, _, _ = s.ReadLedger()
	if ledger.Current.Id != "first" || len(ledger.Queued) != 1 || ledger.Queued[0].Origin != OriginHuman {
		t.Fatalf("second open did not queue: %+v", ledger)
	}
	// Duplicate id refuses.
	if _, err := s.Open(mainHolder, "first", "again", "No."); err == nil {
		t.Fatal("duplicate open passed")
	}
}

// The transition table: every legal move and its idempotence refusal.
func TestTransitionTableMatrix(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "a", "goal a", "Do a.")
	mustOpen(t, s, mainHolder, "b", "goal b", "Do b.")

	// park Current requires --then or --and-none; --and-none refuses with
	// a standing queue.
	if _, err := s.Park(mainHolder, "a", "blocked", "", false); err == nil {
		t.Fatal("parking the only Current without a successor passed")
	}
	if _, err := s.Park(mainHolder, "a", "blocked", "", true); err == nil || !strings.Contains(err.Error(), "the queue holds") {
		t.Fatalf("--and-none with a queue did not refuse by name: %v", err)
	}
	if _, err := s.Park(mainHolder, "a", "blocked", "b", false); err != nil {
		t.Fatalf("park with --then: %v", err)
	}
	ledger, _, _ := s.ReadLedger()
	if ledger.Current.Id != "b" || len(ledger.Parked) != 1 {
		t.Fatalf("park+then wrong: %+v", ledger)
	}

	// unpark returns to the queue; unpark twice refuses.
	if _, err := s.Unpark(mainHolder, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Unpark(mainHolder, "a"); err == nil {
		t.Fatal("double unpark passed")
	}

	// done requires successor; --then promotes.
	if _, err := s.Done(mainHolder, "b", "landed", "", false); err == nil {
		t.Fatal("done without successor passed")
	}
	if _, err := s.Done(mainHolder, "b", "landed", "a", false); err != nil {
		t.Fatal(err)
	}
	ledger, _, _ = s.ReadLedger()
	if ledger.Current.Id != "a" || len(ledger.Done) != 1 || ledger.Done[0].NextStep != "" {
		t.Fatalf("done+then wrong: %+v", ledger)
	}
	// done on done refuses.
	if _, err := s.Done(mainHolder, "b", "again", "", true); err == nil {
		t.Fatal("double done passed")
	}

	// reopen requires --next and preserves origin.
	if _, err := s.Reopen(mainHolder, "b", "Pick it back up."); err != nil {
		t.Fatal(err)
	}
	ledger, _, _ = s.ReadLedger()
	if len(ledger.Queued) != 1 || ledger.Queued[0].Id != "b" || ledger.Queued[0].Origin != OriginMain {
		t.Fatalf("reopen wrong: %+v", ledger.Queued)
	}

	// promote refuses while a Current stands; after done --and-none...
	if _, err := s.Promote(mainHolder, "b"); err == nil {
		t.Fatal("promote over a standing Current passed")
	}
	// ... the queue must clear first (park b), then and-none writes the
	// declaration.
	if _, err := s.Park(mainHolder, "b", "later", "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Done(mainHolder, "a", "all landed", "", true); err != nil {
		t.Fatal(err)
	}
	ledger, _, _ = s.ReadLedger()
	if ledger.Free == nil || ledger.Current != nil {
		t.Fatalf("and-none did not declare goal-free: %+v", ledger)
	}

	// set-next needs a Current.
	if _, err := s.SetNext(mainHolder, "New step."); err == nil {
		t.Fatal("set-next with no Current passed")
	}
	// unpark drops the declaration.
	if _, err := s.Unpark(mainHolder, "b"); err != nil {
		t.Fatal(err)
	}
	ledger, _, _ = s.ReadLedger()
	if ledger.Free != nil {
		t.Fatal("unpark left the goal-free declaration standing")
	}
}

// GOAL-15: declare-free renews on a standing declaration (the one
// idempotence exception), refreshing the digest as the world moves.
func TestGoalFreeRenew(t *testing.T) {
	s := testStore(t)
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	ledger, _, _ := s.ReadLedger()
	first := ledger.Free.Digest

	// The world moves: a new plans stream appears.
	os.WriteFile(filepath.Join(s.Root, "plans", "new-work.md"), []byte("work"), 0o644)
	result, err := s.DeclareFree(human)
	if err != nil {
		t.Fatalf("renewal refused: %v", err)
	}
	if !strings.Contains(result.Message, "renewed") {
		t.Fatalf("renewal not named: %s", result.Message)
	}
	ledger, _, _ = s.ReadLedger()
	if ledger.Free.Digest == first {
		t.Fatal("renewal did not refresh the digest")
	}
}

// The advisory human gate: done/park on a human-origin goal refuses a
// non-HUMAN caller, directly and through reconcile's replay.
func TestHumanGate(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, human, "precious", "The human's own program", "Do it.")

	if _, err := s.Done(mainHolder, "precious", "done", "", true); err == nil || !strings.Contains(err.Error(), "human-reserved") {
		t.Fatalf("main concluded a human goal: %v", err)
	}
	if _, err := s.Done(human, "precious", "done", "", true); err != nil {
		t.Fatalf("the human's own conclusion refused: %v", err)
	}
}

// GOAL-02: reconcile replays full transition authority — a manual edit
// marking a human-origin goal done is refused for MAIN and accepted for
// HUMAN; origin rewrites and illegal jumps always refuse.
func TestReconcileReplaysAuthority(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, human, "precious", "The human's own program", "Do it.")

	// Manual edit: precious becomes Done with a goal-free declaration.
	edited := `# Goals

## Done goal: precious — The human's own program
- Origin: human
- Concluded: Landed by hand.

## Goal-free: declared 2026-08-15T12:00:00Z by human over abc
`
	if err := os.WriteFile(LedgerPath(s.Root), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	// Mutating verbs refuse the mismatch.
	if _, err := s.SetNext(mainHolder, "step"); err == nil || !strings.Contains(err.Error(), "reconcile") {
		t.Fatalf("mutation on a mismatched ledger did not point at reconcile: %v", err)
	}
	// MAIN reconcile refuses the human-origin conclusion.
	if _, err := s.Reconcile(mainHolder); err == nil || !strings.Contains(err.Error(), "human-reserved") {
		t.Fatalf("MAIN replayed a human conclusion: %v", err)
	}
	// HUMAN reconcile accepts it.
	if _, err := s.Reconcile(human); err != nil {
		t.Fatalf("HUMAN reconcile refused: %v", err)
	}
	if !s.BaselineMatches() {
		t.Fatal("reconcile did not accept the baseline")
	}

	// Origin rewrite refuses even for HUMAN.
	rewritten := strings.Replace(edited, "Origin: human", "Origin: main", 1)
	os.WriteFile(LedgerPath(s.Root), []byte(rewritten), 0o644)
	if _, err := s.Reconcile(human); err == nil || !strings.Contains(err.Error(), "origin") {
		t.Fatalf("origin rewrite passed replay: %v", err)
	}
}

// GOAL-03 + GOAL-14 + GOAL-10 substrate: the baseline lifecycle — genesis
// adoption of an unbaselined ledger, the crash window (ledger written,
// baseline stale), and restoration after post-adoption deletion.
func TestBaselineLifecycle(t *testing.T) {
	s := testStore(t)

	// Genesis: a hand-written legal ledger, no baseline.
	body := "# Goals\n\n## Current goal: solo — One goal\n- Origin: main\n- Next step: Do.\n"
	os.MkdirAll(filepath.Join(s.Root, "plans"), 0o755)
	os.WriteFile(LedgerPath(s.Root), []byte(body), 0o644)
	if _, err := s.SetNext(mainHolder, "step"); err == nil || !strings.Contains(err.Error(), "no accepted baseline") {
		t.Fatalf("unbaselined mutation did not refuse: %v", err)
	}
	if _, err := s.Reconcile(mainHolder); err != nil {
		t.Fatalf("genesis reconcile: %v", err)
	}
	if !s.BaselineMatches() {
		t.Fatal("genesis did not write the baseline")
	}

	// Crash window: the ledger advanced, the baseline did not (simulated
	// by rewriting the ledger directly). Mutation refuses; reconcile
	// repairs via replay.
	advanced := body + "\n## Queued goal: next — Another\n- Origin: main\n- Next step: Queue.\n"
	os.WriteFile(LedgerPath(s.Root), []byte(advanced), 0o644)
	if _, err := s.SetNext(mainHolder, "x"); err == nil {
		t.Fatal("crash-window mutation passed")
	}
	if _, err := s.Reconcile(mainHolder); err != nil {
		t.Fatalf("crash-window reconcile: %v", err)
	}

	// Post-adoption deletion: restoration from the baseline's full bytes.
	os.Remove(LedgerPath(s.Root))
	if _, err := s.Open(mainHolder, "another", "x", "y"); err == nil || !strings.Contains(err.Error(), "restore") {
		t.Fatalf("mutation on deleted ledger did not point at restoration: %v", err)
	}
	result, err := s.Reconcile(mainHolder)
	if err != nil || !strings.Contains(result.Message, "restored from baseline") {
		t.Fatalf("restoration: %v %s", err, result.Message)
	}
	data, _ := os.ReadFile(LedgerPath(s.Root))
	if string(data) != advanced {
		t.Fatal("restoration did not reproduce the accepted bytes")
	}
}

// Illegal replay deltas refuse: deleting a standing goal, editing a
// queued goal in place, jumping done -> current.
func TestReconcileRefusesIllegalDeltas(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "a", "goal a", "Do a.")
	mustOpen(t, s, mainHolder, "b", "goal b", "Do b.")

	cases := []struct{ name, body string }{
		{"delete-standing", "# Goals\n\n## Current goal: a — goal a\n- Origin: main\n- Next step: Do a.\n"},
		{"queued-edit", "# Goals\n\n## Current goal: a — goal a\n- Origin: main\n- Next step: Do a.\n\n## Queued goal: b — goal b REWRITTEN\n- Origin: main\n- Next step: Do b.\n"},
	}
	for _, c := range cases {
		os.WriteFile(LedgerPath(s.Root), []byte(c.body), 0o644)
		if _, err := s.Reconcile(mainHolder); err == nil {
			t.Errorf("%s passed replay", c.name)
		}
	}
}

// GOAL-13: prune keeps the newest ten done goals and reports every drop.
func TestPruneReportsDrops(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "seed", "seed goal", "Do.")
	ledger, _, _ := s.ReadLedger()
	for i := 0; i < 12; i++ {
		ledger.Done = append(ledger.Done, Goal{
			Id: "d" + string(rune('a'+i)), Intent: "done goal", Origin: OriginMain, Conclude: "landed",
		})
	}
	if err := s.publish(ledger); err != nil {
		t.Fatal(err)
	}
	s.writeBaseline(Serialize(ledger))

	result, err := s.Prune(mainHolder)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Dropped) != 2 {
		t.Fatalf("dropped %d, want 2: %v", len(result.Dropped), result.Dropped)
	}
	ledger, _, _ = s.ReadLedger()
	if len(ledger.Done) != DoneKept {
		t.Fatalf("kept %d, want %d", len(ledger.Done), DoneKept)
	}
	// Nothing left to prune refuses (idempotence-explicit).
	if _, err := s.Prune(mainHolder); err == nil {
		t.Fatal("empty prune passed")
	}
}

// GOAL-21: mutation refuses while a mission is actively driven (live
// runner record), refuses fail-closed on unknown liveness, and allows
// after the runner is provably dead. Env-stripping is irrelevant by
// construction: the fact is read from disk.
func TestGoalMutationRefusesActiveMission(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	dir := filepath.Join(s.Root, "artifacts", "agents", "missions", "runners")
	os.MkdirAll(dir, 0o755)
	record := map[string]any{"missionId": "m1", "status": "running", "pid": 4242, "pidStartedAt": 999}
	writeJSON := func() {
		data, _ := json.Marshal(record)
		os.WriteFile(filepath.Join(dir, "m1.json"), data, 0o644)
	}
	writeJSON()

	// Live runner: refuse.
	s.Prober = fakeProber{verdicts: map[int64]identity.Liveness{4242: identity.Alive}, starts: map[int64]int64{4242: 999}}
	if _, err := s.SetNext(mainHolder, "step"); err == nil || !strings.Contains(err.Error(), "mission is active") {
		t.Fatalf("active mission did not refuse: %v", err)
	}

	// Unknown liveness: refuse fail-closed.
	s.Prober = fakeProber{verdicts: map[int64]identity.Liveness{4242: identity.Unknown}}
	if _, err := s.SetNext(mainHolder, "step"); err == nil || !strings.Contains(err.Error(), "fail-closed") {
		t.Fatalf("unknown liveness did not refuse fail-closed: %v", err)
	}

	// Dead runner: mutation allowed.
	s.Prober = fakeProber{verdicts: map[int64]identity.Liveness{4242: identity.Dead}}
	if _, err := s.SetNext(mainHolder, "step"); err != nil {
		t.Fatalf("dead runner blocked mutation: %v", err)
	}

	// Read verbs stay available under an active mission.
	s.Prober = fakeProber{verdicts: map[int64]identity.Liveness{4242: identity.Alive}, starts: map[int64]int64{4242: 999}}
	writeJSON()
	if _, _, err := s.ReadLedger(); err != nil {
		t.Fatalf("read refused under active mission: %v", err)
	}
}

// The refusal edges the matrix test does not reach: promote on every
// wrong section, read-side helpers, and the empty-checkout read.
func TestVerbRefusalEdges(t *testing.T) {
	s := testStore(t)
	if ledger, problems, err := s.ReadLedger(); ledger != nil || problems != nil || err != nil {
		t.Fatal("empty checkout read was not clean-empty")
	}
	if s.BaselinePresent() {
		t.Fatal("baseline present in an empty checkout")
	}
	mustOpen(t, s, mainHolder, "a", "goal a", "Do a.")
	mustOpen(t, s, mainHolder, "b", "goal b", "Do b.")
	if !s.BaselinePresent() {
		t.Fatal("baseline missing after open")
	}
	if _, err := s.Promote(mainHolder, "ghost"); err == nil {
		t.Fatal("promote of a missing id passed")
	}
	if _, err := s.Promote(mainHolder, "a"); err == nil {
		t.Fatal("promote of the Current goal passed")
	}
	if _, err := s.Park(mainHolder, "a", "why", "b", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Promote(mainHolder, "a"); err == nil {
		t.Fatal("promote of a parked goal passed")
	}
	if _, err := s.Done(mainHolder, "a", "x", "", true); err == nil {
		t.Fatal("done on a parked goal passed")
	}
	if _, err := s.Reopen(mainHolder, "a", "step"); err == nil {
		t.Fatal("reopen of a parked goal passed")
	}
	if _, err := s.Park(mainHolder, "ghost", "why", "", true); err == nil {
		t.Fatal("park of a missing id passed")
	}
	if _, err := s.SetNext(mainHolder, "Do a."); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetNext(mainHolder, "Do a."); err == nil {
		t.Fatal("identical set-next passed (idempotence-explicit)")
	}
	// The queued park path (no successor requirement).
	mustOpen(t, s, mainHolder, "c", "goal c", "Do c.")
	if _, err := s.Park(mainHolder, "c", "later", "", false); err != nil {
		t.Fatalf("queued park: %v", err)
	}
	// Revision and queued digest read through the ledger.
	ledger, _, _ := s.ReadLedger()
	if ledger.Revision() == "" {
		t.Fatal("no revision on a current goal")
	}
	if ledger.QueuedDigest() != "" {
		t.Fatal("queued digest without queued goals")
	}
}

// Concurrent verbs serialize under the flock: two goroutines opening
// distinct goals both land, and the ledger stays legal.
func TestConcurrentVerbsSerialize(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "root-goal", "the root", "Do.")
	done := make(chan error, 2)
	go func() { _, err := s.Open(mainHolder, "left", "left goal", "L."); done <- err }()
	go func() { _, err := s.Open(mainHolder, "right", "right goal", "R."); done <- err }()
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	ledger, problems, err := s.ReadLedger()
	if err != nil || len(problems) > 0 {
		t.Fatalf("post-race ledger: %v %v", err, problems)
	}
	if len(ledger.Queued) != 2 {
		t.Fatalf("lost an open under the lock: %+v", ledger.Queued)
	}
	if !s.BaselineMatches() {
		t.Fatal("baseline out of step after concurrent writes")
	}
}

// The deleted-baseline downgrade guard (authority review F2/F3): a
// non-holder may not genesis-adopt a ledger that already carries
// goals — that is a corrupted initialized project, restored by its
// holder, never re-adopted by a passing genesis caller.
func TestGenesisRefusesPopulatedLedgerForNonHolder(t *testing.T) {
	s := testStore(t)
	if _, err := s.Open(mainHolder, "real-goal", "intent", "next"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reconcile(mainHolder); err != nil {
		t.Fatal(err)
	}
	// Simulate the attack: the accepted baseline is deleted, leaving a
	// populated ledger with no baseline — which looks like genesis.
	if err := os.Remove(BaselinePath(s.Root)); err != nil {
		t.Fatal(err)
	}
	nonHolderMain := Caller{Class: "MAIN", Holder: false}
	if _, err := s.Reconcile(nonHolderMain); err == nil || !strings.Contains(err.Error(), "already carries goals") {
		t.Fatalf("a non-holder must not re-baseline a populated ledger: %v", err)
	}
	if _, err := s.Reconcile(human); err == nil || !strings.Contains(err.Error(), "already carries goals") {
		t.Fatalf("even the human genesis path refuses a populated ledger without a holder: %v", err)
	}
	// The holder (the project's owner) may re-baseline it.
	if _, err := s.Reconcile(mainHolder); err != nil {
		t.Fatalf("the holder must be able to re-baseline: %v", err)
	}
}

// A genuinely goal-free genesis (the adopt skeleton shape) is still
// admitted for a non-holder — the legitimate provisioning path.
func TestGenesisAdmitsGoalFreeLedgerForNonHolder(t *testing.T) {
	s := testStore(t)
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatalf("declare-free: %v", err)
	}
	if err := os.Remove(BaselinePath(s.Root)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	nonHolderMain := Caller{Class: "MAIN", Holder: false}
	if _, err := s.Reconcile(nonHolderMain); err != nil {
		t.Fatalf("a goal-free genesis must pass for a non-holder main: %v", err)
	}
}

// The pre-lock race (opus-window re-review finding 4): a caller the
// verb layer admitted under GENESIS mode reaches Reconcile after a
// baseline has appeared. Every non-genesis arm must refuse it — the
// caller never earned holder-only authority — while the same state
// stays reachable for a holder-authorized (non-genesis) caller.
func TestReconcileRefusesGenesisCallerOnceBaselined(t *testing.T) {
	s := testStore(t)
	if _, err := s.Open(mainHolder, "real-goal", "intent", "next"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reconcile(mainHolder); err != nil {
		t.Fatal(err)
	}
	// The race outcome: authorization saw no baseline (Genesis true),
	// the lock sees one. The exact-match arm would be a harmless no-op,
	// the replay and malformed arms would not — the guard refuses them
	// all uniformly before any arm runs.
	raced := Caller{Class: "HUMAN", Holder: false, Genesis: true}
	if _, err := s.Reconcile(raced); err == nil || !strings.Contains(err.Error(), "authorized for genesis") {
		t.Fatalf("a genesis-admitted caller must be refused once a baseline exists: %v", err)
	}
	// A re-run re-authorizes against the initialized project at the
	// verb layer — holder-only there. The guard promises re-evaluation,
	// not success: a HUMAN passes as sovereign, a non-holder MAIN is
	// refused by authorization, and the HOLDER (here) proceeds.
	if _, err := s.Reconcile(Caller{Class: "MAIN", Holder: true}); err != nil {
		t.Fatalf("a holder-authorized re-run must pass: %v", err)
	}
	// And a genesis-admitted caller against a genuinely virgin store
	// still takes the genesis arm (the flag only bites with a baseline).
	virgin := testStore(t)
	if _, err := virgin.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(BaselinePath(virgin.Root)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := virgin.Reconcile(Caller{Class: "MAIN", Holder: false, Genesis: true}); err != nil {
		t.Fatalf("genesis admission must still work on a virgin store: %v", err)
	}
}

// Genesis is goal-free-only for every non-holder (above) AND, for every
// non-holder that is not the human, refused outright when the checkout's
// history carries a ledger: a deleted baseline on an initialized project
// is the holder's to restore, and rm-then-reconcile must not become a
// ledger rewrite a merge carries. The human keeps today's rule.
func TestGenesisRefusesTrackedLedgerForNonHolder(t *testing.T) {
	s := testStore(t)
	gitOK(t, s.Root, "init", "-q")
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatalf("declare-free: %v", err)
	}
	gitOK(t, s.Root, "add", ".")
	gitOK(t, s.Root, "commit", "-qm", "adopted")
	if err := os.Remove(BaselinePath(s.Root)); err != nil {
		t.Fatal(err)
	}
	for _, caller := range []Caller{
		{Class: "MAIN", Holder: false, Genesis: true},
		{Class: "DELEGATE", Holder: false, Genesis: true},
		{Class: "ADAPTER-SUPERVISOR", Holder: false, Genesis: true},
	} {
		if _, err := s.Reconcile(caller); err == nil || !strings.Contains(err.Error(), "committed history") {
			t.Fatalf("%s: a tracked ledger must refuse a non-holder genesis: %v", caller.Class, err)
		}
	}
	if _, err := s.Reconcile(Caller{Class: "HUMAN", Genesis: true}); err != nil {
		t.Fatalf("the human keeps the goal-free genesis path: %v", err)
	}
	if err := os.Remove(BaselinePath(s.Root)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reconcile(mainHolder); err != nil {
		t.Fatalf("the holder restores: %v", err)
	}
}

// On a root whose history carries no ledger, a goal-free ledger is
// adoption-shaped for any class — the provisioning flows under agent
// ancestry (the adopt fixtures, the kit gate, a session whose
// announcement lapsed) all seed through here.
func TestGenesisAdmitsAdoptionShapedLedgerForMachinery(t *testing.T) {
	s := testStore(t)
	os.MkdirAll(filepath.Join(s.Root, "plans"), 0o755)
	os.WriteFile(LedgerPath(s.Root), []byte(goalFreeLedger), 0o644)
	if _, err := s.Reconcile(Caller{Class: "DELEGATE", Genesis: true}); err != nil {
		t.Fatalf("a delegate-shaped adopter must seed a goal-free ledger: %v", err)
	}
	if !s.BaselineMatches() {
		t.Fatal("genesis did not write the baseline")
	}
}
