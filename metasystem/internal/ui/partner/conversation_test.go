package partner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestTheConversationIsKeptInItsOwnDirectoryAndReadBack(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conversation, err := OpenConversation(root, "Wido")
	testutil.Require(t, "opened", err, nil)
	testutil.Require(t, "appended", conversation.Append(Message{
		ID: "m1", Turn: "t1", Role: RoleHuman, Text: "why is this goal waiting?",
		Key: "k1", Page: &Page{Section: "Backlog", Kind: KindGoal, Subject: "g1"},
	}), nil)
	testutil.Require(t, "answered", conversation.Append(Message{
		ID: "m2", Turn: "t1", Role: RolePartner, Text: "It is blocked.", Outcome: OutcomeComplete,
	}), nil)

	reopened, err := OpenConversation(root, "Wido")
	testutil.Require(t, "reopened", err, nil)
	messages := reopened.Messages(0)
	testutil.Require(t, "both lines are there", len(messages), 2)
	testutil.Expect(t, "the human's page context survives the file", messages[0].Page.Subject, "g1")
	testutil.Expect(t, "the outcome survives the file", messages[1].Outcome, OutcomeComplete)
	testutil.Expect(t, "the file is where the design says",
		fileExists(filepath.Join(root, "Wido.jsonl")), true)
}

// A retry after a lost answer is the same turn, because the key is in the
// file and not only in this process's memory.
func TestARetriedKeyNamesTheSameTurnAfterARestart(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conversation, err := OpenConversation(root, "Wido")
	testutil.Require(t, "opened", err, nil)
	testutil.Require(t, "appended", conversation.Append(Message{
		ID: "m1", Turn: "t1", Role: RoleHuman, Text: "hello", Key: "k1"}), nil)
	reopened, err := OpenConversation(root, "Wido")
	testutil.Require(t, "reopened", err, nil)
	turn, known := reopened.TurnFor("k1")
	testutil.Expect(t, "the key is known", known, true)
	testutil.Expect(t, "and names its turn", turn, "t1")
	_, unknown := reopened.TurnFor("k2")
	testutil.Expect(t, "another key is not", unknown, false)
}

// A handle with a separator in it names a file in this directory and never a
// path out of it.
func TestAHumansNameNeverLeavesTheConversationDirectory(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "a path becomes one name", fileName("../../etc/passwd"), "------etc-passwd")
	testutil.Expect(t, "a space becomes a dash", fileName("Jane Doe"), "Jane-Doe")
	testutil.Expect(t, "nobody is the seat", fileName("  "), "seat")
}

// The recovery block is the last twenty messages, each with its speaker and,
// for the human's, the page it was asked from, so an earlier "this goal" keeps
// the goal it meant.
func TestTheRecoveryBlockKeepsSpeakersAndPageContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conversation, err := OpenConversation(root, "Wido")
	testutil.Require(t, "opened", err, nil)
	for at := 0; at < 30; at++ {
		testutil.Require(t, "append "+strconv.Itoa(at), conversation.Append(Message{
			ID: "m" + strconv.Itoa(at), Turn: "t" + strconv.Itoa(at), Role: RoleHuman,
			Text: "question " + strconv.Itoa(at),
			Page: &Page{Section: "Backlog", Title: "Sign-in goal"},
		}), nil)
	}
	block, given := conversation.History()
	testutil.Expect(t, "twenty messages at most", given, recoveryMessages)
	testutil.Expect(t, "it says what it is", strings.HasPrefix(block, "Conversation so far\n"), true)
	testutil.Expect(t, "the speaker is named", strings.Contains(block, "- The human (asked from Backlog · Sign-in goal): question 29"), true)
	testutil.Expect(t, "the oldest kept is the twentieth back", strings.Contains(block, "question 10"), true)
	testutil.Expect(t, "and nothing older", strings.Contains(block, "question 9:"), false)
}

// The block is bounded in characters as well as in messages, and what does not
// fit is dropped from the oldest end.
func TestTheRecoveryBlockIsBoundedInCharacters(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conversation, err := OpenConversation(root, "Wido")
	testutil.Require(t, "opened", err, nil)
	long := strings.Repeat("x", 3000)
	for at := 0; at < 6; at++ {
		testutil.Require(t, "append "+strconv.Itoa(at), conversation.Append(Message{
			ID: "m" + strconv.Itoa(at), Role: RolePartner, Text: long + strconv.Itoa(at)}), nil)
	}
	block, given := conversation.History()
	testutil.Expect(t, "fewer than six fit", given < 6, true)
	testutil.Expect(t, "within the bound", len(block) <= recoveryBytes+len("Conversation so far\n"), true)
	testutil.Expect(t, "the newest is kept", strings.Contains(block, long+"5"), true)
}

func TestAnEmptyConversationGivesNoHistory(t *testing.T) {
	t.Parallel()
	conversation, err := OpenConversation(t.TempDir(), "Wido")
	testutil.Require(t, "opened", err, nil)
	block, given := conversation.History()
	testutil.Expect(t, "nothing to give", given, 0)
	testutil.Expect(t, "and no block", block, "")
}

// The live session's id is kept beside the transcript, for a build that learns
// to load sessions natively.
func TestTheLiveSessionIsRecordedBesideTheTranscript(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conversation, err := OpenConversation(root, "Wido")
	testutil.Require(t, "opened", err, nil)
	conversation.RecordSession("session-1", time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC))
	reopened, err := OpenConversation(root, "Wido")
	testutil.Require(t, "reopened", err, nil)
	testutil.Expect(t, "the session is remembered", reopened.Session(), "session-1")
}

// A message longer than the transcript keeps is kept to the bound and says so,
// rather than being dropped or written whole.
func TestAnOversizeAnswerIsBoundedAndSaysSo(t *testing.T) {
	t.Parallel()
	conversation, err := OpenConversation(t.TempDir(), "Wido")
	testutil.Require(t, "opened", err, nil)
	testutil.Require(t, "appended", conversation.Append(Message{
		ID: "m1", Role: RolePartner, Text: strings.Repeat("y", maxMessageBytes+10)}), nil)
	kept := conversation.Messages(0)[0].Text
	testutil.Expect(t, "it says it was cut", strings.HasSuffix(kept, "longer than the transcript keeps]"), true)
	testutil.Expect(t, "and is bounded", len(kept) < maxMessageBytes+200, true)
}

// A message written at the bound the writer holds one to reads back, whatever
// JSON spends on spelling it and whatever bounded proposals it carries.
//
// The reader took a line of the message bound plus a kilobyte, which is less than
// the text bound alone can cost: JSON spends six bytes on one `<`, so an answer of
// them — or a plain answer carrying one proposal — wrote a line the reader then
// refused. And it refused THE WHOLE FILE for it, which is the opposite of this
// store's own rule that a bad line costs one message: the transcript failed to
// open on every restart, and the Decisions page, which reads the same transcript,
// failed with it (Astra B-01). What the writer can write, the reader reads.
func TestAMessageAtTheBoundReadsBackWhateverJSONSpendsOnIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conversation, err := OpenConversation(root, "Wido")
	testutil.Require(t, "opened", err, nil)
	testutil.Require(t, "appended", conversation.Append(Message{
		ID: "m1", Turn: "t1", Role: RolePartner, Outcome: OutcomeComplete,
		// Every one of these costs six bytes in JSON, which is the most one byte
		// of an answer can cost.
		Text: strings.Repeat("<", maxMessageBytes),
		Proposals: []Proposal{{
			Index: 0, Verb: "edit-goal", Goal: strings.Repeat("g", 64),
			Title: strings.Repeat("t", 160), State: ProposalWaiting, Version: 1,
			Fields: map[string]string{
				"intent":   strings.Repeat("i", 2000),
				"nextStep": strings.Repeat("n", 2000),
				"labels":   strings.Repeat("l", 25*32),
			},
			Why: strings.Repeat("w", 2000),
		}},
	}), nil)

	reopened, err := OpenConversation(root, "Wido")
	testutil.Require(t, "reopened", err, nil)
	messages := reopened.Messages(0)
	testutil.Require(t, "the line reads back", len(messages), 1)
	testutil.Expect(t, "with its whole text", len(messages[0].Text), maxMessageBytes)
	testutil.Require(t, "and its proposal", len(messages[0].Proposals), 1)
	testutil.Expect(t, "whole", len(messages[0].Proposals[0].Why), 2000)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
