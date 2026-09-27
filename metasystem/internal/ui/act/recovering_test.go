package act

// Journal recovery from the browser.
//
// A push can LAND and fail its confirmation. The journal entry then stays at
// pushed with no outcome, and the engine's exclusion rule is that this clone
// mutates NOTHING until somebody classifies it: every later act from the page
// met "this clone mutates nothing until it is classified", and the page had no
// way to reach the classification. Refresh advanced the accepted ref and
// answered a current board, which is not the same thing at all — a successful
// ledger read is not journal recovery.
//
// So a human's press classifies the journal first, through the engine's own
// recovery rule — the same call `bin/metasystem goal recover` makes, with the
// policy this hand can carry — and then publishes. What recovery may not touch
// is left exactly where it is, and the act says so in the engine's words.

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The act the finding names: an approve whose confirming refetch failed leaves
// the entry at pushed, and the next act from the page classifies it — confirmed
// on the canonical tip — and then publishes its own.
func TestTheNextActClassifiesAPushNobodyCouldConfirm(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-unconfirmed")
	openGoal(t, bed, "ui-after")
	authority := sessionFor(t, bed)
	seam := scripting(bed)
	landed, operation := "", ""
	seam.publish = func(plain goal.Repository, parent, commit string) (goal.CASOutcome, error) {
		landed = commit
		return plain.Publish(parent, commit)
	}
	seam.capture = func(plain goal.Repository, opid string) (string, error) {
		if landed == "" {
			operation = opid
			return plain.Capture(opid)
		}
		return "", errors.New("the remote closed the connection")
	}

	refusal := refusalOf(t, authority.Approve("ui-unconfirmed", box()))
	testutil.Expect(t, "the push is unresolved", refusal.Code, "pushed-unknown")
	entry, err := goal.ReadEntry(bed.root, operation)
	if err != nil {
		t.Fatalf("reading the stranded entry: %v", err)
	}
	testutil.Expect(t, "the entry stands at pushed", entry.Phase, goal.PhasePushed)
	testutil.Expect(t, "with no outcome", string(entry.Outcome), "")

	// Connectivity returns, and the human presses something else.
	seam.capture = nil
	if err := authority.Approve("ui-after", box()); err != nil {
		t.Fatalf("the next act after an unclassified push: %v", err)
	}

	entry, err = goal.ReadEntry(bed.root, operation)
	if err != nil {
		t.Fatalf("reading the recovered entry: %v", err)
	}
	testutil.Expect(t, "the stranded entry is closed", entry.Phase, goal.PhaseTerminal)
	testutil.Expect(t, "under the outcome its opid proves", entry.Outcome, goal.OutcomeConfirmed)
	testutil.Expect(t, "the approval that landed is on the page's own read",
		readGoal(t, bed, "ui-unconfirmed").State, goal.StateApproved)
	testutil.Expect(t, "and the act the human just made landed too",
		readGoal(t, bed, "ui-after").State, goal.StateApproved)
}

// What recovery may not touch. A pushed entry another live process owns is
// never taken from it, so the wedge stands — and the act says which entry
// stands and why, rather than publishing over it or claiming a refusal the
// ledger did not make.
func TestAPushedEntryARecoveryMayNotTouchLeavesTheActUnresolved(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-wedged")
	authority := sessionFor(t, bed)
	// A live process that is not this one: pid 1 is alive on every host this
	// runs on, and it is nobody's descendant, so recovery leaves its entry.
	stranded := goal.Entry{
		Opid: "01J5X0000000000000000000ZZ-mac-ui-1a2b3c4d", Machine: "mac-ui", Lineage: "seat-1",
		Owner: goal.OwnerIdentity{Pid: 1}, Phase: goal.PhasePushed, CreatedAt: "2026-09-20T08:00:00Z",
		Intent: goal.Intent{Verb: "park", Targets: []string{"ui-wedged"}},
	}
	encoded, err := json.Marshal(stranded)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(bed.root, "artifacts", "agents", "goal-transactions", stranded.Opid+".json"),
		string(encoded), 0o644)

	refusal := refusalOf(t, authority.Approve("ui-wedged", box()))

	testutil.Expect(t, "it is unresolved rather than a refusal of this act", refusal.Kind, KindFailed)
	testutil.Expect(t, "under the code that says a push is outstanding", refusal.Code, "pushed-unknown")
	if !strings.Contains(refusal.Message, stranded.Opid) {
		t.Fatalf("the refusal does not name the entry that stands: %q", refusal.Message)
	}
	if !strings.Contains(refusal.Message, "a live owner's entry") {
		t.Fatalf("the refusal does not say why it stands, in the engine's words: %q", refusal.Message)
	}
	testutil.Expect(t, "nothing was published", readGoal(t, bed, "ui-wedged").State, goal.StateQueued)
	after, err := goal.ReadEntry(bed.root, stranded.Opid)
	if err != nil {
		t.Fatalf("reading the entry recovery left: %v", err)
	}
	testutil.Expect(t, "and the entry is exactly where it was", after.Phase, goal.PhasePushed)
}
