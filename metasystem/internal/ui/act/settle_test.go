package act

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
// So settle reads its own journal entry. These tests drive the three answers
// through a fixture publish: the transaction's own landed-but-unconfirmed shape
// with its error, an entry made unreadable, and the three entries that really do
// prove a non-write.

// pushedResult is what runTransaction returns when a push landed and its
// confirmation failed: an empty outcome, the tip it pushed to, and a detail that
// begins "pushed;" — with the confirming error beside it.
func pushedResult() (goal.PublishResult, error) {
	return goal.PublishResult{
			Outcome: "", Tip: "0000000000000000000000000000000000000002",
			Detail: "pushed; the confirming refetch failed and the journal entry stays pushed",
		},
		errors.New("confirming refetch failed: the remote closed the connection")
}

// journalled is one operation with a journal entry in the phase named, and the
// request that operation belongs to.
func journalled(t *testing.T, bed *ledgerBed, phase goal.Phase) (goal.VerbRequest, string) {
	t.Helper()
	request := request(t, bed)
	operation := goal.Opid(request.Ulid, request.Actor.Machine, request.Actor.Lineage)
	if _, err := goal.CreateEntry(bed.root, operation,
		request.Actor.Machine, request.Actor.Lineage,
		goal.Intent{Verb: "approve", Targets: []string{"ui-one"}}); err != nil {
		t.Fatalf("the fixture journal entry: %v", err)
	}
	switch phase {
	case goal.PhasePushed:
		if err := goal.MarkPushed(bed.root, operation, bedSeedCommit, 1, fixtureNow.Add(time.Minute)); err != nil {
			t.Fatalf("marking the entry pushed: %v", err)
		}
	case goal.PhaseTerminal:
		if err := goal.MarkTerminal(bed.root, operation, goal.OutcomeRejected, "the fixture refused it"); err != nil {
			t.Fatalf("marking the entry terminal: %v", err)
		}
	}
	return request, operation
}

func refusalOf(t *testing.T, err error) *Refusal {
	t.Helper()
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("settle answered %v, want an act.Refusal", err)
	}
	return refusal
}

// A push that landed and could not be confirmed is unresolved, in the words that
// say the act may have landed and that the next read decides.
func TestAPublishErrorWhoseEntryIsPushedIsUnresolved(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	authority := provenFor(t, bed)
	request, _ := journalled(t, bed, goal.PhasePushed)
	result, publishErr := pushedResult()

	refusal := refusalOf(t, authority.settle(request, result, publishErr, "goal approve"))
	testutil.Expect(t, "it is a failure rather than an engine refusal", refusal.Kind, KindFailed)
	testutil.Expect(t, "under its own code", refusal.Code, "pushed-unknown")
	testutil.Expect(t, "it says the push left",
		strings.HasPrefix(refusal.Message, "pushed; whether it landed is unresolved: "), true)
	testutil.Expect(t, "in the engine's own words",
		strings.Contains(refusal.Message, "confirming refetch failed: the remote closed the connection"), true)
	testutil.Expect(t, "and says what decides it",
		strings.HasSuffix(refusal.Message, "; the page's next read says what the ledger did"), true)
}

// A journal that cannot be read is unresolved too: the same fault fails
// MarkTerminal and this read alike, so nothing proves the push did not land.
func TestAPublishErrorWhoseEntryCannotBeReadIsUnresolved(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	authority := provenFor(t, bed)
	request, operation := journalled(t, bed, goal.PhasePushed)
	// The entry is there and is not an entry: what a torn write leaves, and what
	// a read fails on for a reason that is not absence.
	path := filepath.Join(bed.root, "artifacts", "agents", "goal-transactions", operation+".json")
	if err := os.WriteFile(path, []byte("{\"opid\": \"half a fi"), 0o644); err != nil {
		t.Fatalf("the unreadable entry: %v", err)
	}
	result, publishErr := pushedResult()

	refusal := refusalOf(t, authority.settle(request, result, publishErr, "goal approve"))
	testutil.Expect(t, "it is a failure", refusal.Kind, KindFailed)
	testutil.Expect(t, "under its own code", refusal.Code, "journal-unreadable")
	testutil.Expect(t, "it says nothing is proven",
		strings.HasPrefix(refusal.Message, "whether it landed is unresolved: "), true)
	testutil.Expect(t, "in the read's own words", strings.Contains(refusal.Message, "malformed"), true)
	testutil.Expect(t, "and says what to do",
		strings.HasSuffix(refusal.Message, "; check the goal before acting again"), true)
}

// The three entries that really do prove a non-write are refused, as they always
// were: the push never left, or it was refused.
func TestAPublishErrorWithNoEvidenceOfAPushIsRefused(t *testing.T) {
	t.Parallel()
	for what, phase := range map[string]goal.Phase{
		"an entry still at created": goal.PhaseCreated,
		"a terminal entry":          goal.PhaseTerminal,
	} {
		bed := ledger(t)
		authority := provenFor(t, bed)
		request, _ := journalled(t, bed, phase)
		result, publishErr := pushedResult()

		refusal := refusalOf(t, authority.settle(request, result, publishErr, "goal approve"))
		testutil.Expect(t, what+" is the engine refusing", refusal.Kind, KindEngine)
		testutil.Expect(t, what+" under the code it always had", refusal.Code, "refused")
		testutil.Expect(t, what+" in the engine's own sentence", refusal.Message,
			"confirming refetch failed: the remote closed the connection")
	}
	// And an entry that is not there at all: the push never reached the journal,
	// so it never reached the remote.
	bed := ledger(t)
	authority := provenFor(t, bed)
	result, publishErr := pushedResult()
	refusal := refusalOf(t, authority.settle(request(t, bed), result, publishErr, "goal approve"))
	testutil.Expect(t, "an absent entry is the engine refusing", refusal.Kind, KindEngine)
	testutil.Expect(t, "under the code it always had", refusal.Code, "refused")
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
