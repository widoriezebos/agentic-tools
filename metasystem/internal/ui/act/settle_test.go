package act

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// What a publication error means, and why it cannot be assumed.
//
// A push can LAND and then fail its confirmation: the refetch fails, the opid is
// not visible on the refetched tip, or a confirmed follow-on effect fails. The
// transaction then returns an empty outcome whose detail begins "pushed;" with an
// error beside it, and the journal entry stays at pushed. Calling that refused
// told a human that an approval had not been made when it had, and offered them
// the press that would make a second one — and a second approve is a second
// approval record, not a no-op.
//
// So settle reads its own journal entry. The three answers below are driven
// through a REAL publication: the act is an ordinary Approve, and the bed's own
// ledger seam is scripted to fail at one moment of it, so the result and the
// error settle classifies are the transaction's own rather than a test's idea of
// them. What remains of the journal's classification that no publication can
// produce is read straight off settle, at the end.

// scripted is the bed's own ledger with the two calls a publication's fate turns
// on — the capture of the tip and the compare-and-set — under a test's hand.
// Everything else is the fixture's, so a scripted act runs the whole transaction
// and parts from a confirming one at exactly one moment.
type scripted struct {
	goal.Repository
	capture func(plain goal.Repository, opid string) (string, error)
	publish func(plain goal.Repository, parent, commit string) (goal.CASOutcome, error)
}

func (s *scripted) Capture(opid string) (string, error) {
	if s.capture != nil {
		return s.capture(s.Repository, opid)
	}
	return s.Repository.Capture(opid)
}

func (s *scripted) Publish(parent, commit string) (goal.CASOutcome, error) {
	if s.publish != nil {
		return s.publish(s.Repository, parent, commit)
	}
	return s.Repository.Publish(parent, commit)
}

// scripting hands the bed a scripted ledger, and belongs AFTER the fixture's own
// setup verbs: those open the goal against the ledger that simply works, so the
// only publication a script meets is the act under test.
func scripting(bed *ledgerBed) *scripted {
	seam := &scripted{Repository: bed.repository}
	bed.repository = seam
	return seam
}

// goalAt reads one goal on one commit without moving any ref: what a push
// actually wrote, as against what the accepted tree the page reads shows.
func goalAt(t *testing.T, bed *ledgerBed, commit, id string) *goal.GoalFile {
	t.Helper()
	projection, err := goal.ProjectAtEndpoint(bed.endpoint(), commit, fixtureNow)
	if err != nil {
		t.Fatalf("reading the ledger at %s: %v", commit, err)
	}
	file := projection.Tree.Live[id]
	if file == nil {
		t.Fatalf("goal %s is not live at %s", id, commit)
	}
	return file
}

// createdEntry is one operation whose journal entry exists and has not been
// pushed, and the request that operation belongs to.
func createdEntry(t *testing.T, bed *ledgerBed) goal.VerbRequest {
	t.Helper()
	asked := request(t, bed)
	operation := goal.Opid(asked.Ulid, asked.Actor.Machine, asked.Actor.Lineage)
	if _, err := goal.CreateEntry(bed.root, operation,
		asked.Actor.Machine, asked.Actor.Lineage,
		goal.Intent{Verb: "approve", Targets: []string{"ui-one"}}); err != nil {
		t.Fatalf("the fixture journal entry: %v", err)
	}
	return asked
}

func refusalOf(t *testing.T, err error) *Refusal {
	t.Helper()
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("the act answered %v, want an act.Refusal", err)
	}
	return refusal
}

// A push that landed and could not be confirmed is unresolved, in the words that
// say the act may have landed and that the next read decides. The publication is
// real: the compare-and-set lands on the canonical branch, and the refetch the
// transaction takes afterwards to find its own opid there fails.
func TestAPushThatLandsAndCannotBeConfirmedIsUnresolved(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-settle")
	authority := provenFor(t, bed)
	seam := scripting(bed)
	landed := ""
	seam.publish = func(plain goal.Repository, parent, commit string) (goal.CASOutcome, error) {
		landed = commit
		return plain.Publish(parent, commit)
	}
	// Every capture the transaction needs to build its commit answers; the one it
	// takes after the push has left does not.
	seam.capture = func(plain goal.Repository, opid string) (string, error) {
		if landed == "" {
			return plain.Capture(opid)
		}
		return "", errors.New("the remote closed the connection")
	}

	refusal := refusalOf(t, authority.Approve("ui-settle", box()))
	testutil.Expect(t, "it is a failure rather than an engine refusal", refusal.Kind, KindFailed)
	testutil.Expect(t, "under its own code", refusal.Code, "pushed-unknown")
	testutil.Expect(t, "it says the push left",
		strings.HasPrefix(refusal.Message, "pushed; whether it landed is unresolved: "), true)
	testutil.Expect(t, "in the engine's own words",
		strings.Contains(refusal.Message, "confirming refetch failed: the remote closed the connection"), true)
	testutil.Expect(t, "and says what decides it",
		strings.HasSuffix(refusal.Message, "; the page's next read says what the ledger did"), true)

	// And those words are true of this ledger. The approval IS on the commit the
	// push landed on, and the accepted tree the page reads has not advanced onto it:
	// which is why the answer sends the human to the next read rather than to the
	// one press that would write a second approval.
	if landed == "" {
		t.Fatal("the act never reached the compare-and-set, so no push was left unresolved")
	}
	testutil.Expect(t, "the approval is on the pushed commit", goalAt(t, bed, landed, "ui-settle").State, goal.StateApproved)
	testutil.Expect(t, "and the page's own read still shows it queued",
		readGoal(t, bed, "ui-settle").State, goal.StateQueued)
}

// A journal that cannot be read is unresolved too: the same fault fails
// MarkTerminal and this read alike, so nothing proves the push did not land. Here
// the push really has landed, which is what makes the assumption dangerous rather
// than merely unproven.
func TestAPublicationWhoseJournalCannotBeReadIsUnresolved(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-settle")
	authority := provenFor(t, bed)
	seam := scripting(bed)
	landed, operation := "", ""
	seam.publish = func(plain goal.Repository, parent, commit string) (goal.CASOutcome, error) {
		landed = commit
		return plain.Publish(parent, commit)
	}
	seam.capture = func(plain goal.Repository, opid string) (string, error) {
		if landed == "" {
			// The captures before the push are the transaction's own, taken under
			// the operation's id: the entry this act will be judged by.
			operation = opid
			return plain.Capture(opid)
		}
		// One fault takes the refetch and the journal together. The entry is
		// there and is not an entry: what a torn write leaves, and what a read
		// fails on for a reason that is not absence.
		write(t, filepath.Join(bed.root, "artifacts", "agents", "goal-transactions", operation+".json"),
			"{\"opid\": \"half a fi", 0o644)
		return "", errors.New("the remote closed the connection")
	}

	refusal := refusalOf(t, authority.Approve("ui-settle", box()))
	testutil.Expect(t, "it is a failure", refusal.Kind, KindFailed)
	testutil.Expect(t, "under its own code", refusal.Code, "journal-unreadable")
	testutil.Expect(t, "it says nothing is proven",
		strings.HasPrefix(refusal.Message, "whether it landed is unknown: "), true)
	testutil.Expect(t, "in the read's own words", strings.Contains(refusal.Message, "is damaged"), true)
	testutil.Expect(t, "and says what to do",
		strings.HasSuffix(refusal.Message, "; check the goal before acting again"), true)

	testutil.Expect(t, "the approval is on the pushed commit", goalAt(t, bed, landed, "ui-settle").State, goal.StateApproved)
	testutil.Expect(t, "and the page's own read still shows it queued",
		readGoal(t, bed, "ui-settle").State, goal.StateQueued)
}

// A compare-and-set the ledger refuses is the engine refusing, under the code it
// always had. A refusal is not an ending on its own: the transaction rebuilds on
// the branch that moved and tries again inside its deadline, so the seam fails
// the rebuild's own capture — a refusal scripted to repeat forever would spin
// against a publication deadline this seat cannot shorten. The sentence the human
// reads is therefore the transaction's last word rather than the lease's, and it
// is the journal entry, closed with an outcome, that proves no push is
// outstanding.
func TestARefusedCompareAndSetIsTheEnginesRefusal(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-settle")
	authority := provenFor(t, bed)
	seam := scripting(bed)
	refused, operation := false, ""
	seam.publish = func(plain goal.Repository, parent, commit string) (goal.CASOutcome, error) {
		refused = true
		return goal.CASRefused, errors.New("the branch moved under the lease")
	}
	seam.capture = func(plain goal.Repository, opid string) (string, error) {
		if !refused {
			operation = opid
			return plain.Capture(opid)
		}
		return "", errors.New("the rebuild's fetch failed")
	}

	refusal := refusalOf(t, authority.Approve("ui-settle", box()))
	testutil.Expect(t, "the engine refused it", refusal.Kind, KindEngine)
	testutil.Expect(t, "under the code it always had", refusal.Code, "refused")
	testutil.Expect(t, "in the engine's own sentence", refusal.Message, "the rebuild's fetch failed")

	entry, err := goal.ReadEntry(bed.root, operation)
	if err != nil {
		t.Fatalf("reading the act's own journal entry: %v", err)
	}
	testutil.Expect(t, "the entry is closed", entry.Phase, goal.PhaseTerminal)
	testutil.Expect(t, "under the outcome that says nothing landed", entry.Outcome, goal.OutcomeAbandoned)
	testutil.Expect(t, "and the goal is untouched", readGoal(t, bed, "ui-settle").State, goal.StateQueued)
}

// The two journal shapes left over are refused as they always were, and they are
// read straight off settle because no publication produces them: the entry is
// created before anything happens and pushed before the push leaves this process,
// so a transaction that reached a remote at all leaves neither. They are still the
// classification's own branches, and an absent entry is the strongest non-write
// there is.
func TestAJournalThatProvesNoPushIsRefused(t *testing.T) {
	t.Parallel()
	failure := errors.New("the push was refused")

	bed := ledger(t)
	authority := provenFor(t, bed)
	refusal := refusalOf(t, authority.settle(createdEntry(t, bed), goal.PublishResult{}, failure, "goal approve"))
	testutil.Expect(t, "an entry still at created is the engine refusing", refusal.Kind, KindEngine)
	testutil.Expect(t, "under the code it always had", refusal.Code, "refused")
	testutil.Expect(t, "in the engine's own sentence", refusal.Message, failure.Error())

	// An entry that is not there at all: the push never reached the journal, so
	// it never reached the remote.
	other := ledger(t)
	elsewhere := provenFor(t, other)
	refusal = refusalOf(t, elsewhere.settle(request(t, other), goal.PublishResult{}, failure, "goal approve"))
	testutil.Expect(t, "an absent entry is the engine refusing", refusal.Kind, KindEngine)
	testutil.Expect(t, "under that same code", refusal.Code, "refused")
}

// A confirmed publication is untouched by any of this: the proof is recorded and
// the route answers with the backlog.
func TestAConfirmedPublicationStillSettlesWithoutReadingTheJournal(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-settle")
	authority := provenFor(t, bed)
	testutil.Expect(t, "the act lands", authority.Approve("ui-settle", box()), nil)
	file := readGoal(t, bed, "ui-settle")
	testutil.Expect(t, "and the goal is approved", file.State, goal.StateApproved)
}
