package act

// Two hands on one ledger, in one process.
//
// The first is the second press. Two tabs read one proposal and both press
// Apply; nothing in the ledger stops the second one, because every request
// gets its own operation identifier. Held at the first act's publication, the
// second press used to run the whole way through and land a second record —
// here, an identical edit, which is the verb that has no no-op to fall back on
// (Astra A-01).
//
// The second is recovery. An act whose push has landed and whose confirmation
// has not returned leaves its journal entry at pushed, and that is precisely
// what the engine's recovery rule classifies. A Refresh in another tab ran it
// beside the publication and terminalized the entry; the request that HAD
// landed the act then answered refused and recorded no authority proof beside
// it (Astra D-01). The live-owner rule did not help: a live owner is another
// process, and this is another request of this one.
//
// Both are closed by the act layer owning what it is executing: a registry
// keyed by the goal and the act, and one lock over this clone's publications
// and recoveries. The concurrency below is driven through the bed's own ledger
// seam with channels, so every ordering here is stated rather than waited for.

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func intent(text string) *string { return &text }

// errClosed is the one remote fault these tests script: the confirming
// refetch that never comes back.
var errClosed = errors.New("the remote closed the connection")

// proofRecorded says whether the authority proof this clone writes beside an
// act is there — the record a request told it was refused never makes.
func proofRecorded(t *testing.T, bed *ledgerBed, id string) bool {
	t.Helper()
	file := readGoal(t, bed, id)
	if file.Approved == nil {
		t.Fatalf("goal %s carries no approval, so nothing could be recorded beside it", id)
	}
	_, err := os.Stat(filepath.Join(bed.root, "artifacts", "agents", "authority", "proofs", file.Approved.Opid+".json"))
	return err == nil
}

// The second press, while the first is still in flight: refused at once, in
// the words the page shows, having read nothing and published nothing. The
// first press lands exactly as it would have alone.
func TestASecondPressOnAnActAlreadyRunningIsRefusedBeforeItReadsAnything(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-race")
	authority := sessionFor(t, bed)
	// Every reading an act takes, counted, so the refusal can be shown to
	// have taken none of them. The bed's own readings are kept and called
	// through, so the first press still runs the whole publication path.
	var readings atomic.Int64
	taken := authority.reads
	authority.reads.fence = func(installation stateroot.Installation) error {
		readings.Add(1)
		return taken.fence(installation)
	}
	seam := scripting(bed)
	// The first press is held before its push, where the journal carries
	// nothing for the second press's own recovery to find: the registry is
	// what refuses it, and nothing else.
	building, release, once := make(chan struct{}), make(chan struct{}), sync.Once{}
	seam.capture = func(plain goal.Repository, opid string) (string, error) {
		once.Do(func() {
			close(building)
			<-release
		})
		return plain.Capture(opid)
	}
	answered := make(chan error, 1)
	go func() { answered <- authority.Edit("ui-race", Edited{Intent: intent("A rewritten intent.")}) }()

	<-building
	before := readings.Load()
	refusal := refusalOf(t, authority.Edit("ui-race", Edited{Intent: intent("A rewritten intent.")}))

	testutil.Expect(t, "the second press is refused", refusal.Code, "in-flight")
	testutil.Expect(t, "in the words the page shows", refusal.Message,
		"this act on this goal is being applied by another press; the next read says what happened")
	testutil.Expect(t, "under the kind the route answers 409 for", refusal.Kind, KindEngine)
	testutil.Expect(t, "and it read nothing at all", readings.Load(), before)

	close(release)
	testutil.Expect(t, "the first press landed", <-answered, nil)
	file := readGoal(t, bed, "ui-race")
	testutil.Expect(t, "the intent is the one the human sent", file.Intent, "A rewritten intent.")
	testutil.Expect(t, "and one edit was recorded, not two", len(file.History), 2)
}

// The registry is the goal and the act together: a second press on the same
// goal for a DIFFERENT act, and on the same act for a different goal, is not
// this refusal, and the first press's hold is over the moment it answers.
func TestTheHoldIsOneGoalsOneActAndEndsWhenItAnswers(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-held")
	authority := sessionFor(t, bed)
	held := ownerOf(bed.root)

	clear, err := held.begin("ui-held", "goal approve")
	if err != nil {
		t.Fatalf("the first press: %v", err)
	}
	if _, again := held.begin("ui-held", "goal approve"); again == nil {
		t.Fatal("a second press on the act in flight was admitted")
	}
	for name, key := range map[string]flight{
		"another act on the same goal": {goal: "ui-held", act: "goal park"},
		"the same act on another goal": {goal: "ui-other", act: "goal approve"},
	} {
		other, otherErr := held.begin(key.goal, key.act)
		if otherErr != nil {
			t.Fatalf("%s was refused as in flight: %v", name, otherErr)
		}
		other()
	}
	clear()

	// And the act it held is pressable again, for real.
	if err := authority.Approve("ui-held", box()); err != nil {
		t.Fatalf("the press after the hold ended: %v", err)
	}
	testutil.Expect(t, "the goal is approved", readGoal(t, bed, "ui-held").State, goal.StateApproved)
}

// D-01. A publication holds this clone's one lock from the moment its request
// is assembled until its settlement has answered, so the recovery a Refresh
// runs cannot take its entry away: the Refresh waits, and then finds the entry
// terminal and touches nothing.
//
// The instant it is held at is the dangerous one — the push has landed on the
// canonical branch and nothing has confirmed it — which is exactly the entry
// recovery would classify.
func TestARecoveryCannotTakeTheEntryOfAPublicationStillRunning(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-confirming")
	authority := sessionFor(t, bed)
	seam := scripting(bed)
	operation, pushed, release := "", make(chan struct{}), make(chan struct{})
	seam.capture = func(plain goal.Repository, opid string) (string, error) {
		if operation == "" {
			operation = opid
		}
		return plain.Capture(opid)
	}
	seam.publish = func(plain goal.Repository, parent, commit string) (goal.CASOutcome, error) {
		outcome, err := plain.Publish(parent, commit)
		close(pushed)
		<-release
		return outcome, err
	}
	answered := make(chan error, 1)
	go func() { answered <- authority.Approve("ui-confirming", box()) }()

	<-pushed
	if ownerOf(bed.root).publications.TryLock() {
		ownerOf(bed.root).publications.Unlock()
		t.Fatal("the publication holds no lock, so a recovery could classify the entry it is still executing")
	}
	reconciled := make(chan error, 1)
	go func() { reconciled <- Reconcile(bed.root, bed.endpoint()) }()
	close(release)

	testutil.Expect(t, "the act that landed answers as one that landed", <-answered, nil)
	testutil.Expect(t, "and the recovery beside it answers too", <-reconciled, nil)
	entry, err := goal.ReadEntry(bed.root, operation)
	if err != nil {
		t.Fatalf("reading the act's own journal entry: %v", err)
	}
	testutil.Expect(t, "the entry is closed by the request that owned it", entry.Phase, goal.PhaseTerminal)
	testutil.Expect(t, "under the outcome its own confirmation wrote", entry.Outcome, goal.OutcomeConfirmed)
	testutil.Expect(t, "the approval stands", readGoal(t, bed, "ui-confirming").State, goal.StateApproved)
	// The proof beside the act is the half D-01 took away: a request told it
	// was refused records none.
	testutil.Expect(t, "and its authority proof was recorded", proofRecorded(t, bed, "ui-confirming"), true)
}

// The other half of the exclusion: a recovery classifies under the same lock,
// asked from inside the recovery's own reading of the ledger. A publication
// that started beside it would have to wait, which is what keeps the two from
// ever overlapping in either order.
func TestARecoveryClassifiesUnderTheLockAPublicationTakes(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-stranded")
	authority := sessionFor(t, bed)
	seam := scripting(bed)
	landed := ""
	seam.publish = func(plain goal.Repository, parent, commit string) (goal.CASOutcome, error) {
		landed = commit
		return plain.Publish(parent, commit)
	}
	// An act whose push lands and whose confirmation fails: the entry stays
	// at pushed, which is the work the recovery below has to do.
	seam.capture = func(plain goal.Repository, opid string) (string, error) {
		if landed != "" {
			return "", errClosed
		}
		return plain.Capture(opid)
	}
	testutil.Expect(t, "the push is unresolved",
		refusalOf(t, authority.Approve("ui-stranded", box())).Code, "pushed-unknown")

	free := false
	seam.capture = func(plain goal.Repository, opid string) (string, error) {
		if ownerOf(bed.root).publications.TryLock() {
			ownerOf(bed.root).publications.Unlock()
			free = true
		}
		return plain.Capture(opid)
	}
	if err := Reconcile(bed.root, bed.endpoint()); err != nil {
		t.Fatalf("the recovery: %v", err)
	}
	if free {
		t.Fatal("the recovery read the ledger without the lock a publication takes, so one could have started under it")
	}
}
