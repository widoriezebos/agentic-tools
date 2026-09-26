package partner_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
)

// The conversation's own bounds (g1-s54 D1). A transcript over its size is cut
// from its head down to the floor, and the first message a human then reads says
// when the rest went.
//
// The trim is the conversation's operation and not a sweep over the file, which
// is Astra's F2: the file and the cached messages are replaced together, so the
// transcript the page is served matches the one on disk from the same instant.
func TestATranscriptOverItsSizeIsTrimmedToTheFloor(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-09-26T10:00:00Z")
	conversation, _ := transcript(t, 105, now, "a question and its answer, at some length "+strings.Repeat("x", 900))

	cut, err := conversation.Trim(partner.TrimBounds{Bytes: 100 << 10}, now)

	testutil.Require(t, "trimming", err, nil)
	testutil.Expect(t, "ten messages went, down to the floor of two hundred", cut, 10)
	held := conversation.Messages(0)
	testutil.Expect(t, "two hundred are kept, with the notice at the head", len(held), 201)
	testutil.Expect(t, "and the first message says when the rest went", held[0].Text,
		"Earlier messages were trimmed on 26 September 2026.")
	testutil.Expect(t, "the last message is still the last", held[200].Text, "answer 104")
}

// The age bound with the size bound off, which is also zero disabling one of
// them: nothing is cut for size, and everything older than the cutoff goes.
func TestATranscriptWhoseOldestMessagesArePastTheDayBoundIsTrimmed(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-09-26T10:00:00Z")
	directory := t.TempDir()
	conversation, err := partner.OpenConversation(directory, "wido")
	testutil.Require(t, "opening", err, nil)
	appendPairs(t, conversation, 0, 10, now.AddDate(0, 0, -100), "old")
	appendPairs(t, conversation, 10, 115, now, "recent")

	cut, err := conversation.Trim(partner.TrimBounds{Age: 90 * 24 * time.Hour}, now)

	testutil.Require(t, "trimming", err, nil)
	testutil.Expect(t, "the twenty messages of the ten old turns went", cut, 20)
	held := conversation.Messages(0)
	testutil.Expect(t, "the rest are kept, with the notice at the head", len(held), 211)
	oldest := ""
	for _, message := range held[1:] {
		if strings.HasPrefix(message.Text, "old") {
			oldest = message.Text
			break
		}
	}
	testutil.Expect(t, "and nothing older than the bound remains", oldest, "")
	testutil.Expect(t, "the first recent turn is now the first thing said", held[1].Text, "recent question 10")
}

// The cut falls only where the turn changes. A transcript whose only allowed cut
// would fall inside a turn is left alone: half a turn — a question with no
// answer, or an answer with no question — is never what a human is left with.
func TestATranscriptIsLeftAloneWhereTheOnlyCutWouldSplitATurn(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-09-26T10:00:00Z")
	directory := t.TempDir()
	conversation, err := partner.OpenConversation(directory, "wido")
	testutil.Require(t, "opening", err, nil)
	appendPairs(t, conversation, 0, 100, now, "kept")
	// The trailing turn: a question this server is still answering.
	testutil.Require(t, "the trailing question", conversation.Append(partner.Message{
		ID: "asking", Turn: "turn-trailing", Role: partner.RoleHuman,
		Text: "and what about this?", At: now.Format(time.RFC3339),
	}), nil)
	before := conversation.Messages(0)

	cut, err := conversation.Trim(partner.TrimBounds{Bytes: 1}, now)

	testutil.Require(t, "trimming", err, nil)
	testutil.Expect(t, "nothing went, because the one allowed cut is inside a turn", cut, 0)
	testutil.Expect(t, "the transcript is as it was", len(conversation.Messages(0)), len(before))
	testutil.Expect(t, "and the trailing question is still there",
		conversation.Messages(1)[0].Text, "and what about this?")
}

// An unfinished trailing turn is kept whole even where the bound asks for more.
// A turn with no terminal outcome is one this server is still answering, and the
// answer that lands would have nothing to answer.
func TestAnUnfinishedTrailingTurnIsKeptWhole(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-09-26T10:00:00Z")
	directory := t.TempDir()
	conversation, err := partner.OpenConversation(directory, "wido")
	testutil.Require(t, "opening", err, nil)
	appendPairs(t, conversation, 0, 20, now, "earlier")
	// A trailing turn longer than the floor, so the floor is not what keeps it
	// and the clamp is. In a served conversation a turn is a question and its
	// answer, and the floor covers it; this is the clamp itself under test.
	for at := 0; at < 210; at++ {
		testutil.Require(t, "the long trailing turn "+strconv.Itoa(at), conversation.Append(partner.Message{
			ID: "trailing-" + strconv.Itoa(at), Turn: "turn-trailing", Role: partner.RolePartner,
			Text: "still answering " + strconv.Itoa(at), At: now.Format(time.RFC3339),
		}), nil)
	}

	cut, err := conversation.Trim(partner.TrimBounds{Bytes: 1}, now)

	testutil.Require(t, "trimming", err, nil)
	testutil.Expect(t, "the forty finished messages went", cut, 40)
	held := conversation.Messages(0)
	testutil.Expect(t, "the unfinished turn is kept whole, behind the notice", len(held), 211)
	testutil.Expect(t, "the notice is at the head", held[0].Trimmed, true)
	whole := true
	for _, message := range held[1:] {
		if message.Turn != "turn-trailing" {
			whole = false
		}
	}
	testutil.Expect(t, "and everything kept is that turn", whole, true)
}

// A transcript under the floor is never trimmed, whatever the bounds say: two
// hundred messages are kept even where they are over the size and past the day
// bound, which is why the numbers are retention targets and not disk ceilings.
func TestATranscriptUnderTheFloorIsNeverTrimmed(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-09-26T10:00:00Z")
	conversation, _ := transcript(t, 100, now.AddDate(0, 0, -200), strings.Repeat("y", 2000))

	cut, err := conversation.Trim(partner.TrimBounds{Bytes: 1, Age: time.Hour}, now)

	testutil.Require(t, "trimming", err, nil)
	testutil.Expect(t, "nothing went", cut, 0)
	testutil.Expect(t, "and the two hundred are all there", len(conversation.Messages(0)), 200)
}

// Zero disables a bound, and both zero is a store that keeps everything.
func TestZeroDisablesEachTranscriptBound(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-09-26T10:00:00Z")
	conversation, _ := transcript(t, 120, now.AddDate(0, 0, -200), strings.Repeat("z", 2000))

	cut, err := conversation.Trim(partner.TrimBounds{}, now)

	testutil.Require(t, "trimming with both bounds off", err, nil)
	testutil.Expect(t, "nothing went", cut, 0)
	testutil.Expect(t, "everything is kept", len(conversation.Messages(0)), 240)

	cut, err = conversation.Trim(partner.TrimBounds{Age: 90 * 24 * time.Hour}, now)
	testutil.Require(t, "trimming for age alone", err, nil)
	testutil.Expect(t, "the age bound alone still cuts", cut > 0, true)
}

// A trim leaves the file and the cached messages saying the same thing, and a
// process that reads the file afresh reads what the page was being served.
func TestTheCachedMessagesAndTheFileSayTheSameThingAfterATrim(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-09-26T10:00:00Z")
	conversation, directory := transcript(t, 105, now, strings.Repeat("w", 1000))

	_, err := conversation.Trim(partner.TrimBounds{Bytes: 100 << 10}, now)
	testutil.Require(t, "trimming", err, nil)

	reopened, err := partner.OpenConversation(directory, "wido")
	testutil.Require(t, "reopening", err, nil)
	testutil.Expect(t, "the file holds what the cache holds",
		ids(reopened.Messages(0)), ids(conversation.Messages(0)))
	testutil.Expect(t, "the file's own mode is unchanged", mode(t, filepath.Join(directory, "wido.jsonl")),
		os.FileMode(0o600))
}

// An append accepted while a trim runs is not lost, because the trim takes the
// mutex the append takes. That is the whole of F2: a read-trim-replace from
// outside the conversation could write the file back without a message it had
// already accepted, and the human who sent it would watch it disappear.
func TestAnAppendRacingATrimIsNotLost(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-09-26T10:00:00Z")
	conversation, directory := transcript(t, 105, now, strings.Repeat("v", 1000))

	const senders, each = 6, 12
	started := make(chan struct{})
	var sending sync.WaitGroup
	for sender := 0; sender < senders; sender++ {
		sending.Add(1)
		go func(sender int) {
			defer sending.Done()
			<-started
			for at := 0; at < each; at++ {
				name := "racing-" + strconv.Itoa(sender) + "-" + strconv.Itoa(at)
				_ = conversation.Append(partner.Message{
					ID: name, Turn: name, Role: partner.RoleHuman, Text: name,
					At: now.Format(time.RFC3339),
				})
			}
		}(sender)
	}
	close(started)
	_, err := conversation.Trim(partner.TrimBounds{Bytes: 100 << 10}, now)
	testutil.Require(t, "trimming under the appends", err, nil)
	sending.Wait()

	held := map[string]bool{}
	for _, message := range conversation.Messages(0) {
		held[message.ID] = true
	}
	missing := []string{}
	for sender := 0; sender < senders; sender++ {
		for at := 0; at < each; at++ {
			name := "racing-" + strconv.Itoa(sender) + "-" + strconv.Itoa(at)
			if !held[name] {
				missing = append(missing, name)
			}
		}
	}
	testutil.Expect(t, "every accepted message is still in the transcript: "+strings.Join(missing, " "),
		len(missing), 0)
	reopened, err := partner.OpenConversation(directory, "wido")
	testutil.Require(t, "reopening", err, nil)
	testutil.Expect(t, "and the file says the same as the cache",
		ids(reopened.Messages(0)), ids(conversation.Messages(0)))
}

// The notice is not stacked and not re-dated. A sweep whose only allowed cut is
// an earlier sweep's own notice does nothing: the floor is what is stopping the
// bound, and a notice dated today over messages that went in June would be a
// lie the file told.
func TestASecondSweepDoesNotRestampTheNoticeWhenTheFloorIsWhatStops(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-09-26T10:00:00Z")
	conversation, _ := transcript(t, 115, now.AddDate(0, 0, -100), "long past")
	bounds := partner.TrimBounds{Age: 90 * 24 * time.Hour}

	first, err := conversation.Trim(bounds, now)
	testutil.Require(t, "the first sweep", err, nil)
	testutil.Expect(t, "it cut down to the floor", first, 30)
	stamped := conversation.Messages(0)[0]

	later := now.AddDate(0, 0, 5)
	second, err := conversation.Trim(bounds, later)

	testutil.Require(t, "the second sweep", err, nil)
	testutil.Expect(t, "it cut nothing", second, 0)
	held := conversation.Messages(0)
	testutil.Expect(t, "there is one notice and not two", notices(held), 1)
	testutil.Expect(t, "and it still says when the messages went", held[0].Text, stamped.Text)
}

// transcript is a conversation of n finished turns, all at one moment.
func transcript(t *testing.T, turns int, when time.Time, text string) (*partner.Conversation, string) {
	t.Helper()
	directory := t.TempDir()
	conversation, err := partner.OpenConversation(directory, "wido")
	testutil.Require(t, "opening the transcript", err, nil)
	appendPairs(t, conversation, 0, turns, when, text)
	return conversation, directory
}

// appendPairs appends one question and one finished answer per turn, which is
// what a served turn leaves behind.
func appendPairs(t *testing.T, conversation *partner.Conversation, from, turns int, when time.Time, text string) {
	t.Helper()
	stamp := when.UTC().Format(time.RFC3339)
	for turn := from; turn < turns; turn++ {
		name := "turn-" + strconv.Itoa(turn)
		if err := conversation.Append(partner.Message{
			ID: name + "-asked", Turn: name, Role: partner.RoleHuman,
			Text: text + " question " + strconv.Itoa(turn), At: stamp,
		}); err != nil {
			t.Fatalf("cannot append the question of %s: %v", name, err)
		}
		if err := conversation.Append(partner.Message{
			ID: name + "-answered", Turn: name, Role: partner.RolePartner,
			Outcome: partner.OutcomeComplete, Text: "answer " + strconv.Itoa(turn), At: stamp,
		}); err != nil {
			t.Fatalf("cannot append the answer of %s: %v", name, err)
		}
	}
}

func ids(messages []partner.Message) string {
	held := make([]string, 0, len(messages))
	for _, message := range messages {
		held = append(held, message.ID)
	}
	return strings.Join(held, ",")
}

func notices(messages []partner.Message) int {
	count := 0
	for _, message := range messages {
		if message.Trimmed {
			count++
		}
	}
	return count
}

func at(t *testing.T, stamp string) time.Time {
	t.Helper()
	when, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		t.Fatalf("cannot read the fixture's own instant %q: %v", stamp, err)
	}
	return when
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("cannot stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

// The notice is one line of the file like every other message, so a build that
// reads the transcript without this package still reads it.
func TestTheNoticeIsALineOfTheFileLikeEveryOtherMessage(t *testing.T) {
	t.Parallel()
	now := at(t, "2026-09-26T10:00:00Z")
	conversation, directory := transcript(t, 105, now, strings.Repeat("u", 1000))
	_, err := conversation.Trim(partner.TrimBounds{Bytes: 100 << 10}, now)
	testutil.Require(t, "trimming", err, nil)

	body, err := os.ReadFile(filepath.Join(directory, "wido.jsonl"))
	testutil.Require(t, "reading the file", err, nil)
	first := strings.SplitN(strings.TrimSpace(string(body)), "\n", 2)[0]
	var message partner.Message
	testutil.Require(t, "the first line is one message", json.Unmarshal([]byte(first), &message), nil)
	testutil.Expect(t, "it is marked as the trim's own", message.Trimmed, true)
	testutil.Expect(t, "it is a completed answer, which is how the drawer renders it",
		message.Outcome, partner.OutcomeComplete)
	testutil.Expect(t, "and it says when", message.Text,
		"Earlier messages were trimmed on 26 September 2026.")
}
