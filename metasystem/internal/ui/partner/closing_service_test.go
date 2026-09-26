package partner_test

import (
	"context"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The two turns this interface asks in a human's name besides the opening one,
// and the one decision that owns all three (g1-s55 D2, D3).
//
// Both are the server's, and that is the point of them. A browser that could
// submit a turn in a human's name could submit one per tab, per reload and per
// remount, and the transcript would carry questions nobody typed and nobody can
// count. So the close is a route that asks one turn, the resume is a decision
// taken where the page reads the conversation, and neither is reachable as
// anything but a turn marked as the interface's.

const outcomeSaid = "The limit counts from last activity.\n\nOpen questions: what the twelve hours protected."

// Ending a sitting asks one more turn with the fixed closing request, marked as
// the interface's, and leaves the sitting standing — so the outcome the turn
// offers is admitted against the record the sitting is on.
func TestClosingASittingAsksTheFixedRequestAndLeavesTheSittingStanding(t *testing.T) {
	t.Parallel()
	service, _ := serviceDepositing(t, "outcome", "", outcomeSaid)
	events, stop := service.Subscribe()
	defer stop()

	sitting, err := service.Sit(context.Background(), "Wido", subjectOf(), partner.PurposeShapeDesign, onTheRecord())
	testutil.Require(t, "the sitting opened", err, nil)
	drain(t, events)

	closing, err := service.Closing(context.Background(), "Wido", onTheRecord())
	testutil.Require(t, "the close was asked", err, nil)
	testutil.Expect(t, "about the record the sitting is on", closing.Subject.ID, sitting.Subject.ID)
	beats := drain(t, events)

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Require(t, "the two turns and their answers", len(read.Messages), 4)
	asked := read.Messages[2]
	testutil.Expect(t, "the closing question is the interface's", asked.Interface, true)
	testutil.Expect(t, "asked in the human's name", asked.Role, partner.RoleHuman)
	testutil.Expect(t, "with the fixed request", asked.Text, partner.ClosingRequest(sitting))

	// The sitting is still standing, which is what makes the card recordable: a
	// deposit admitted against no sitting is not offered at all.
	testutil.Require(t, "the sitting still stands", read.Sitting != nil, true)
	testutil.Expect(t, "on the same record", read.Sitting.Subject.ID, sitting.Subject.ID)
	offered := 0
	for _, beat := range beats {
		if beat.Kind == partner.EventDeposit && beat.Deposit != nil {
			offered++
			testutil.Expect(t, "the outcome is offered", beat.Deposit.Offered, true)
			testutil.Expect(t, "of the closing kind", beat.Deposit.Kind, uitools.DepositOutcome)
			testutil.Expect(t, "against the sitting's record", beat.Deposit.Subject.ID, sitting.Subject.ID)
			testutil.Expect(t, "carrying the draft whole", beat.Deposit.Text, outcomeSaid)
		}
	}
	testutil.Expect(t, "one outcome and no more", offered, 1)
}

// The closing request asks for one deposit of one kind, from what the sitting
// holds, and forbids the weighing — because the close is the draft that becomes
// the record's result, which is where a preparation that supplied the values
// would do the most damage.
func TestTheClosingRequestAsksForOneOutcomeAndForbidsTheWeighing(t *testing.T) {
	t.Parallel()
	sitting := partner.Sitting{Subject: subjectOf(), Purpose: partner.PurposeShapeDesign}
	said := partner.ClosingRequest(sitting)
	for _, wanted := range []string{
		"Close this sitting on plans/designs/sessions.md",
		"one deposit of kind outcome",
		"the outcome as decided",
		"the constraints",
		"the open questions",
		"Facts, Proposals, Decisions, Open questions",
		"Weigh nothing",
		"presses Record it",
	} {
		testutil.Expect(t, "the closing request says "+wanted, strings.Contains(said, wanted), true)
	}
}

// Ending a sitting nobody is in is refused in words, and asks nothing.
func TestClosingWithNoSittingIsRefused(t *testing.T) {
	t.Parallel()
	service, _ := serviceDepositing(t, "outcome", "", outcomeSaid)
	_, err := service.Closing(context.Background(), "Wido", onTheRecord())
	testutil.Require(t, "it is refused", err != nil, true)
	testutil.Expect(t, "saying there is nothing to close",
		strings.Contains(err.Error(), "nothing to close"), true)
	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Expect(t, "and nothing was asked", len(read.Messages), 0)
}

// A standing sitting whose session has ended asks the opening question again,
// said as a resuming, so the first words a human reads say what is on the table.
func TestResumeAsksTheOpeningQuestionAgainWhenTheSessionHasEnded(t *testing.T) {
	t.Parallel()
	service, _ := serviceDepositing(t, "fact", uitools.DepositAnchor+"internal/session/session.go:212\n", factSaid)
	events, stop := service.Subscribe()
	defer stop()

	sitting, err := service.Sit(context.Background(), "Wido", subjectOf(), partner.PurposeShapeDesign, onTheRecord())
	testutil.Require(t, "the sitting opened", err, nil)
	drain(t, events)

	// The live session is alive, so a page reading the conversation asks nothing:
	// the Partner remembers this sitting, and a second opening turn would be the
	// interface asking a question nobody needs.
	asked, err := service.Resume(context.Background(), "Wido")
	testutil.Require(t, "the live session was asked about", err, nil)
	testutil.Expect(t, "a live session resumes nothing", asked, false)

	// The session ends — the process was torn down, or this server restarted.
	service.Close()

	resumed, err := service.Resume(context.Background(), "Wido")
	testutil.Require(t, "the resume was asked", err, nil)
	testutil.Expect(t, "a fresh session resumes", resumed, true)
	drain(t, events)

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Require(t, "the opening turn, its answer, and the resuming turn with its own", len(read.Messages), 4)
	again := read.Messages[2]
	testutil.Expect(t, "the resuming question is the interface's", again.Interface, true)
	testutil.Expect(t, "with the resuming request", again.Text, partner.ResumingRequest(*read.Sitting))
	testutil.Expect(t, "which says it is resuming", strings.Contains(again.Text, "Resuming this sitting"), true)
	testutil.Expect(t, "and asks for the records again",
		strings.Contains(again.Text, partner.OpeningRequest(*read.Sitting)), true)
	testutil.Expect(t, "the sitting is still the one it was", read.Sitting.Subject.ID, sitting.Subject.ID)
}

// A conversation nobody is sitting on resumes nothing, whatever the session is
// doing: there is no sitting to bring what the records hold about.
func TestResumeAsksNothingWhereNoSittingStands(t *testing.T) {
	t.Parallel()
	service, _ := serviceDepositing(t, "fact", uitools.DepositAnchor+"sessions.go:212\n", factSaid)
	asked, err := service.Resume(context.Background(), "Wido")
	testutil.Require(t, "no failure", err, nil)
	testutil.Expect(t, "nothing was asked", asked, false)
	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Expect(t, "and the transcript is empty", len(read.Messages), 0)
}

// The sitting mark is what a resume is decided from, and it is on the
// conversation rather than in this process's memory — so the answer survives a
// restart, which is the only moment a resume happens at all.
func TestTheSittingOneHumanIsInIsReadableOnItsOwn(t *testing.T) {
	t.Parallel()
	service, _ := serviceDepositing(t, "fact", uitools.DepositAnchor+"sessions.go:212\n", factSaid)
	events, stop := service.Subscribe()
	defer stop()

	none, err := service.Sitting("Wido")
	testutil.Require(t, "reading it before one stands", err, nil)
	testutil.Expect(t, "nobody is sitting", none == nil, true)

	_, err = service.Sit(context.Background(), "Wido", subjectOf(), partner.PurposeShapeDesign, onTheRecord())
	testutil.Require(t, "the sitting opened", err, nil)
	drain(t, events)

	standing, err := service.Sitting("Wido")
	testutil.Require(t, "reading it while one stands", err, nil)
	testutil.Require(t, "a sitting stands", standing != nil, true)
	testutil.Expect(t, "on the record", standing.Subject.ID, "plans/designs/sessions.md")

	testutil.Require(t, "risen", service.Rise("Wido"), nil)
	after, err := service.Sitting("Wido")
	testutil.Require(t, "reading it after the rise", err, nil)
	testutil.Expect(t, "and none stands afterwards", after == nil, true)
}
