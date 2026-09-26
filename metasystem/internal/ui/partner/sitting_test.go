package partner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The sitting, as the conversation keeps it.
//
// A sitting is not a second store and it holds no working material: the record
// is the memory. What the conversation keeps is the mark — which record, what
// for, since when — and the whole of what that mark owes a human is that it is
// still there when they come back. So it is in the file beside the transcript,
// and a write that fails refuses the act rather than reporting a sitting that
// exists only in this process's memory.

func aSitting() Sitting {
	return Sitting{
		Subject:   Subject{Kind: SubjectRecord, ID: "plans/designs/sessions.md", Title: "Session limits"},
		Purpose:   PurposeShapeDesign,
		StartedAt: "2026-09-26T09:00:00Z",
	}
}

func TestASittingSurvivesTheFileAndSoDoesEndingIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	at := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	conversation, err := OpenConversation(root, "Wido")
	testutil.Require(t, "opened", err, nil)
	testutil.Expect(t, "a fresh conversation is no sitting", conversation.Sitting() == nil, true)
	testutil.Require(t, "sat", conversation.Sit(aSitting(), at), nil)

	reopened, err := OpenConversation(root, "Wido")
	testutil.Require(t, "reopened", err, nil)
	sitting := reopened.Sitting()
	testutil.Require(t, "the sitting survived the file", sitting != nil, true)
	testutil.Expect(t, "with its subject", sitting.Subject.ID, "plans/designs/sessions.md")
	testutil.Expect(t, "addressed as a record", sitting.Subject.Kind, SubjectRecord)
	testutil.Expect(t, "titled as the page showed it", sitting.Subject.Title, "Session limits")
	testutil.Expect(t, "its purpose", sitting.Purpose, PurposeShapeDesign)
	testutil.Expect(t, "and when it began", sitting.StartedAt, "2026-09-26T09:00:00Z")
	// The answer is a copy: a caller that changed it must not change the
	// conversation's own mark.
	sitting.Subject.ID = "somewhere/else.md"
	testutil.Expect(t, "the answer is a copy", reopened.Sitting().Subject.ID, "plans/designs/sessions.md")

	testutil.Require(t, "risen", reopened.Rise(at), nil)
	after, err := OpenConversation(root, "Wido")
	testutil.Require(t, "reopened again", err, nil)
	testutil.Expect(t, "ending it survives the file too", after.Sitting() == nil, true)
}

// The session id and the sitting are written by different acts and live in one
// file, so each write carries the other: a state file that held only its own
// field would drop whichever was written first.
func TestTheSessionAndTheSittingSurviveEachOthersWrites(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	at := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	conversation, err := OpenConversation(root, "Wido")
	testutil.Require(t, "opened", err, nil)

	conversation.RecordSession("session-7", at)
	testutil.Require(t, "sat after the session was recorded", conversation.Sit(aSitting(), at), nil)
	reopened, err := OpenConversation(root, "Wido")
	testutil.Require(t, "reopened", err, nil)
	testutil.Expect(t, "the session is still there", reopened.Session(), "session-7")
	testutil.Require(t, "and so is the sitting", reopened.Sitting() != nil, true)

	reopened.RecordSession("session-8", at)
	again, err := OpenConversation(root, "Wido")
	testutil.Require(t, "reopened again", err, nil)
	testutil.Expect(t, "the newer session", again.Session(), "session-8")
	testutil.Require(t, "and the sitting survived that write", again.Sitting() != nil, true)
	testutil.Expect(t, "whole", again.Sitting().Subject.ID, "plans/designs/sessions.md")
}

// A state file that cannot be written is a sitting that is refused, and the mark
// in this process's memory goes back the way it was. A sitting only this process
// knew about would be a human whose deposits are admitted here and lost on the
// next restart.
func TestASittingThatCannotBeWrittenIsRefusedAndLeavesNoMark(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	at := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	conversation, err := OpenConversation(root, "Wido")
	testutil.Require(t, "opened", err, nil)
	// The state file's own name, taken by a directory: the write fails and
	// nothing about the failure is this package's guess.
	testutil.Require(t, "the state file's name is taken",
		os.Mkdir(filepath.Join(root, "Wido.json"), 0o700), nil)

	err = conversation.Sit(aSitting(), at)
	testutil.Expect(t, "the sitting is refused", err != nil, true)
	testutil.Expect(t, "in words that name what could not be written",
		strings.Contains(err.Error(), "conversation state could not be written"), true)
	testutil.Expect(t, "and the conversation carries no sitting", conversation.Sitting() == nil, true)
}

// The message a sitting's opening turn becomes is marked as the interface's, and
// the mark is in the file. A provenance only the live page knew would be a
// provenance a reload turns into the human's own words (g1-s53 D3).
func TestTheInterfacesOwnQuestionIsMarkedInTheTranscript(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conversation, err := OpenConversation(root, "Wido")
	testutil.Require(t, "opened", err, nil)
	testutil.Require(t, "the interface's own turn", conversation.Append(Message{
		ID: "m1", Turn: "t1", Role: RoleHuman, Text: "Open this sitting on plans/designs/sessions.md",
		Interface: true, Page: &Page{Section: "Project", Path: "/project"},
	}), nil)
	testutil.Require(t, "and one the human typed", conversation.Append(Message{
		ID: "m2", Turn: "t2", Role: RoleHuman, Text: "what does the current limit protect?",
	}), nil)

	reopened, err := OpenConversation(root, "Wido")
	testutil.Require(t, "reopened", err, nil)
	messages := reopened.Messages(0)
	testutil.Require(t, "both are there", len(messages), 2)
	testutil.Expect(t, "the opening turn is marked as the interface's", messages[0].Interface, true)
	testutil.Expect(t, "and a question the human typed is not", messages[1].Interface, false)
}

// The fixed request is fixed, and it is the one sentence this interface says in a
// human's name. What it asks for is what the records hold; what it forbids is the
// weighing, because a preparation that quietly supplies the values is the
// approval habit in conversational form.
func TestTheOpeningRequestAsksForTheRecordsAndForbidsTheWeighing(t *testing.T) {
	t.Parallel()
	request := OpeningRequest(aSitting())
	for _, said := range []string{
		"plans/designs/sessions.md",
		"whose purpose is to " + PurposeShapeDesign,
		"standing human rulings that touch it",
		"decisions this project has recorded about it",
		"the open questions on it",
		"intent and design records",
		"Facts, Proposals, Decisions, Open questions",
		"anchor every claim",
		"Weigh nothing",
		"Do not recommend, do not rank",
		"say that they conflict",
		"say that nothing is recorded",
		"offer each one with the deposit tool",
	} {
		testutil.Expect(t, "the opening request says "+said, strings.Contains(request, said), true)
	}
	testutil.Expect(t, "and it asks for no recommendation",
		strings.Contains(strings.ToLower(request), "which would you recommend"), false)
	// Shaping intent says so in its own words: the purpose travels into the
	// request, so the Partner is told what the sitting is for.
	intent := aSitting()
	intent.Purpose = PurposeShapeIntent
	testutil.Expect(t, "the purpose travels",
		strings.Contains(OpeningRequest(intent), "whose purpose is to "+PurposeShapeIntent), true)
}
