package partner_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// Who decides whether the Partner's deposit reaches the human at all.
//
// The tool server has no conversation and cannot know whether a sitting is open;
// the host has no turn. This service holds the conversation the turn belongs to,
// so it is the one place that can say which record an entry would go to — and
// when there is none it says so on a card, where the human reads the answer,
// rather than dropping the words in silence.

const factSaid = "the mobile client renews the session differently from the web one."

// serviceDepositing is a Partner that answers in words and prepares one deposit
// of the kind named, in the tool's own fixed form.
func serviceDepositing(t *testing.T, kind, clause, text string) (*partner.Service, string) {
	t.Helper()
	root := t.TempDir()
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"},
		root, fakeacp.Open(fakeacp.Script{
			Reads: []fakeacp.Read{{
				Name:   "mcp__metasystem__" + uitools.OpDeposit,
				Title:  "deposit(" + kind + ")",
				Result: uitools.DepositedLine + "\n" + uitools.DepositHeader + kind + "\n" + clause + uitools.DepositSeparator + "\n" + text + "\n",
			}},
			Chunks: []string{"Two rulings touch this. ", "Nothing recorded says what the limit protects."},
		}))
	t.Cleanup(host.Close)
	service := partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) })
	return service, root
}

// onTheRecord is where a human pressing Start on a record's page was standing.
func onTheRecord() partner.Page {
	return partner.Page{
		Section: "Project", Path: "/project/documents/plans/designs/sessions.md",
		Kind: "record", Subject: "plans/designs/sessions.md", Title: "Session limits",
	}
}

func subjectOf() partner.Subject {
	return partner.Subject{Kind: partner.SubjectRecord, ID: "plans/designs/sessions.md", Title: "Session limits"}
}

// Starting a sitting marks the conversation and asks the opening question, in
// that order: the opening turn is already a turn of the sitting, so a deposit it
// prepares is admitted against this subject.
func TestStartingASittingMarksTheConversationAndAsksTheOpeningQuestion(t *testing.T) {
	t.Parallel()
	service, _ := serviceDepositing(t, "fact", uitools.DepositAnchor+"internal/session/session.go:212\n", factSaid)
	events, stop := service.Subscribe()
	defer stop()

	sitting, err := service.Sit(context.Background(), "Wido", subjectOf(), partner.PurposeShapeDesign, onTheRecord())
	testutil.Require(t, "the sitting opened", err, nil)
	testutil.Expect(t, "on the record", sitting.Subject.ID, "plans/designs/sessions.md")
	testutil.Expect(t, "for what the human chose", sitting.Purpose, partner.PurposeShapeDesign)
	testutil.Expect(t, "stamped when it began", sitting.StartedAt, "2026-09-26T09:00:00Z")
	drain(t, events)

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Require(t, "the snapshot carries the sitting", read.Sitting != nil, true)
	testutil.Expect(t, "with its subject", read.Sitting.Subject.ID, "plans/designs/sessions.md")
	testutil.Require(t, "the opening turn and its answer", len(read.Messages), 2)
	asked := read.Messages[0]
	testutil.Expect(t, "the question is the interface's", asked.Interface, true)
	testutil.Expect(t, "asked in the human's name", asked.Role, partner.RoleHuman)
	testutil.Expect(t, "with the fixed request", asked.Text, partner.OpeningRequest(sitting))
	testutil.Require(t, "carrying the page the human was on", asked.Page != nil, true)
	testutil.Expect(t, "which is the record's own", asked.Page.Subject, "plans/designs/sessions.md")

	// Ending it takes the mark off and leaves the transcript where it is.
	testutil.Require(t, "risen", service.Rise("Wido"), nil)
	after, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read again", err, nil)
	testutil.Expect(t, "no sitting", after.Sitting == nil, true)
	testutil.Expect(t, "and the conversation is untouched", len(after.Messages), 2)
}

// A deposit prepared during a sitting is offered, stamped with that sitting's
// subject, carried on the stream as it is admitted and kept on the answer.
func TestADepositDuringASittingIsOfferedAgainstItsSubject(t *testing.T) {
	t.Parallel()
	service, _ := serviceDepositing(t, "fact", uitools.DepositAnchor+"internal/session/session.go:212\n", factSaid)
	events, stop := service.Subscribe()
	defer stop()

	_, err := service.Sit(context.Background(), "Wido", subjectOf(), partner.PurposeShapeDesign, onTheRecord())
	testutil.Require(t, "the sitting opened", err, nil)
	beats := drain(t, events)

	offered := []partner.Event{}
	at, ended := -1, -1
	for index, beat := range beats {
		if beat.Kind == partner.EventDeposit {
			offered = append(offered, beat)
			if at < 0 {
				at = index
			}
		}
		if beat.Kind == partner.EventDone {
			ended = index
		}
	}
	testutil.Require(t, "one deposit beat", len(offered), 1)
	testutil.Require(t, "carrying the deposit", offered[0].Deposit != nil, true)
	testutil.Expect(t, "its kind", offered[0].Deposit.Kind, uitools.DepositFact)
	testutil.Expect(t, "its anchor", offered[0].Deposit.Anchor, "internal/session/session.go:212")
	testutil.Expect(t, "its words whole", offered[0].Deposit.Text, factSaid)
	testutil.Expect(t, "marked as offered", offered[0].Deposit.Offered, true)
	testutil.Expect(t, "stamped with the sitting's subject", offered[0].Deposit.Subject.ID, "plans/designs/sessions.md")
	testutil.Expect(t, "with nothing to explain", offered[0].Deposit.NotOffered, "")
	// It arrives before the answer ends, which is what puts the card on the
	// transcript and the table while the answer is still being written.
	testutil.Expect(t, "before the turn ended", at < ended, true)

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	answered := read.Messages[len(read.Messages)-1]
	testutil.Require(t, "the answer keeps it", len(answered.Deposits), 1)
	testutil.Expect(t, "whole", answered.Deposits[0].Text, factSaid)
	testutil.Expect(t, "with its subject", answered.Deposits[0].Subject.ID, "plans/designs/sessions.md")
	testutil.Expect(t, "marked offered on the answer too", answered.Deposits[0].Offered, true)
	testutil.Expect(t, "and nothing was dropped to an activity line", len(answered.Activity), 0)
	// The call is accounted for as a look as well: a deposit never arrives
	// without the call that prepared it being listed.
	testutil.Require(t, "the page and the call", len(answered.Looked), 2)
	testutil.Expect(t, "the call is named", answered.Looked[1].What, "deposit(fact)")
}

// A deposit prepared with no sitting open is not offered, and the refusal is
// carried as the deposit it was, with its reason, so the drawer shows it as a
// card where the human reads the answer. It is never stamped with a subject:
// nothing may record it anywhere.
func TestADepositWithNoSittingOpenIsNotOffered(t *testing.T) {
	t.Parallel()
	service, _ := serviceDepositing(t, "decision",
		uitools.DepositReason+"it counts from last activity\n", "the limit counts from last activity.")
	events, stop := service.Subscribe()
	defer stop()

	_, err := service.Submit(context.Background(), "Wido", "key-1", "so the limit counts from last activity",
		partner.Page{Section: "Project", Path: "/project"})
	testutil.Require(t, "admitted", err, nil)
	drain(t, events)

	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", err, nil)
	testutil.Expect(t, "there was no sitting", read.Sitting == nil, true)
	answered := read.Messages[len(read.Messages)-1]
	testutil.Require(t, "the refusal is kept as the deposit it was", len(answered.Deposits), 1)
	testutil.Expect(t, "not offered", answered.Deposits[0].Offered, false)
	testutil.Expect(t, "saying why", answered.Deposits[0].NotOffered,
		"no sitting is open, so there is no record to offer this to; "+
			"start a sitting from the record you are working on")
	testutil.Expect(t, "belonging to no record", answered.Deposits[0].Subject.ID, "")
	testutil.Expect(t, "with the words it would have offered",
		answered.Deposits[0].Text, "the limit counts from last activity.")
	testutil.Expect(t, "and its reason kept for the card", answered.Deposits[0].Reason,
		"it counts from last activity")
	testutil.Expect(t, "no activity line stands in for it", len(answered.Activity), 0)
}

// A sitting this build does not offer is refused before anything is written: the
// conversation carries no mark and the transcript stays empty.
func TestASittingThisBuildDoesNotOfferIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		what    string
		subject partner.Subject
		purpose string
		says    string
	}{
		{"a purpose this build has no moves for", subjectOf(), "review",
			"review and learning sittings are not in this build"},
		{"no purpose at all", subjectOf(), "  ", "a sitting is for"},
		{"a subject that is not a record", partner.Subject{Kind: "goal", ID: "g1-s53"},
			partner.PurposeShapeDesign, "about one record of this project"},
		{"a record with no name", partner.Subject{Kind: partner.SubjectRecord, ID: "  "},
			partner.PurposeShapeDesign, "says which one, by its path"},
	} {
		service, _ := serviceDepositing(t, "fact", "", factSaid)
		_, err := service.Sit(context.Background(), "Wido", probe.subject, probe.purpose, onTheRecord())
		testutil.Expect(t, probe.what+" is refused", err != nil, true)
		testutil.Expect(t, probe.what+" says why", strings.Contains(err.Error(), probe.says), true)

		read, readErr := service.Snapshot("Wido", 100)
		testutil.Require(t, "read back "+probe.what, readErr, nil)
		testutil.Expect(t, probe.what+" leaves no sitting", read.Sitting == nil, true)
		testutil.Expect(t, probe.what+" appends nothing", len(read.Messages), 0)
	}
}

// A runtime that cannot start refuses the sitting, and refuses it before
// anything is appended: the transcript is untouched and no mark is left behind
// for a human to come back to (g1-s53 D3).
func TestASittingIsRefusedBeforeAnythingIsAppendedWhenTheRuntimeCannotStart(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"},
		root, fakeacp.Open(fakeacp.Script{InitError: "this runtime is not signed in"}))
	t.Cleanup(host.Close)
	service := partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) })

	_, err := service.Sit(context.Background(), "Wido", subjectOf(), partner.PurposeShapeDesign, onTheRecord())
	testutil.Expect(t, "the sitting is refused", err != nil, true)

	read, readErr := service.Snapshot("Wido", 100)
	testutil.Require(t, "read back", readErr, nil)
	testutil.Expect(t, "nothing was appended", len(read.Messages), 0)
	testutil.Expect(t, "and no sitting was left on the conversation", read.Sitting == nil, true)
}
