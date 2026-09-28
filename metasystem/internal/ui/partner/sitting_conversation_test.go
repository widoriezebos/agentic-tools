package partner

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// A sitting is a conversation (g1-s65 D16): the store opens a sitting's
// conversation by its own key beside the human's, in the same private store and
// under the same owner, and two sittings never share a file.

func TestASittingsConversationIsOpenedByItsOwnKeyBesideTheHumans(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	at := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)

	ordinary, err := OpenConversation(directory, "Wido")
	testutil.Require(t, "the human's own", err, nil)
	review, err := OpenConversation(directory, "Wido", "metasystem/plans/reviews/review-of-g1-s64.md")
	testutil.Require(t, "a review's", err, nil)
	other, err := OpenConversation(directory, "Wido", "metasystem/plans/reviews/review-of-g1-s63.md")
	testutil.Require(t, "a second review's", err, nil)
	// Two records whose names differ only where the file name flattens them are
	// still two files: a slash and a dash are one character of a file name.
	slash, err := OpenConversation(directory, "Wido", "plans/a-b.md")
	testutil.Require(t, "one spelling", err, nil)
	dash, err := OpenConversation(directory, "Wido", "plans-a/b.md")
	testutil.Require(t, "the other spelling", err, nil)

	files := map[string]bool{}
	for _, one := range []*Conversation{ordinary, review, other, slash, dash} {
		if files[one.transcript] || files[one.state] {
			t.Fatalf("two conversations share a file: %s", one.transcript)
		}
		files[one.transcript], files[one.state] = true, true
		if !strings.HasPrefix(one.transcript, directory+string(filepath.Separator)) {
			t.Fatalf("%s is outside the store %s", one.transcript, directory)
		}
	}
	testutil.Expect(t, "the human's own file is where it always was",
		ordinary.transcript, filepath.Join(directory, "Wido.jsonl"))

	testutil.Require(t, "a message in the review", review.Append(Message{ID: "m1", Turn: "t1", Role: RoleHuman,
		Text: "what if the press dies here?", At: at.Format(time.RFC3339)}), nil)
	again, err := OpenConversation(directory, "Wido", "metasystem/plans/reviews/review-of-g1-s64.md")
	testutil.Require(t, "the review opened again", err, nil)
	testutil.Expect(t, "it holds its own words", len(again.Messages(0)), 1)
	testutil.Expect(t, "and the human's holds none of them", len(ordinary.Messages(0)), 0)
	testutil.Expect(t, "nor does the other review", len(other.Messages(0)), 0)

	// Housekeeping lists humans by their files, and a sitting's conversation is
	// not a human.
	testutil.Require(t, "a message of the human's own", ordinary.Append(Message{ID: "m2", Turn: "t2",
		Role: RoleHuman, Text: "what is in review?", At: at.Format(time.RFC3339)}), nil)
	humans, err := Humans(directory)
	testutil.Require(t, "the humans", err, nil)
	testutil.Expect(t, "one human", humans, []string{"Wido"})
	// And a carry into this directory sees a store in use, not files to move.
	entries, err := os.ReadDir(directory)
	testutil.Require(t, "the store", err, nil)
	for _, entry := range entries {
		if !entry.IsDir() && conversationFile(entry.Name()) && entry.Name() != "Wido.jsonl" && entry.Name() != "Wido.json" {
			t.Fatalf("a sitting's file sits among the humans' files: %s", entry.Name())
		}
	}
}

// The mark on a sitting's conversation carries the room's working state — the
// desk, the face and the unfinished words (D9) — and it survives a reopen.
func TestTheRoomsStateIsKeptOnTheSittingsMark(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	at := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	record := "metasystem/plans/reviews/review-of-g1-s64.md"
	conversation, err := OpenConversation(directory, "Wido", record)
	testutil.Require(t, "open", err, nil)
	testutil.Require(t, "sit", conversation.Sit(Sitting{Subject: Subject{Kind: SubjectRecord, ID: record},
		Purpose: PurposeReview, StartedAt: at.Format(time.RFC3339)}, at), nil)

	room := Room{Desk: []byte(`{"items":[{"kind":"source","path":"owner.go"}],"current":0}`),
		Face: "desk", Drafts: []byte(`{"deposit:t1#0":{"text":"the press dies","clause":"owner.go:60"}}`), Seq: 1}
	testutil.Require(t, "keep the room", conversation.Keep(room, at.Add(time.Hour)), nil)

	again, err := OpenConversation(directory, "Wido", record)
	testutil.Require(t, "open again", err, nil)
	sitting := again.Sitting()
	testutil.Require(t, "the sitting stands", sitting != nil, true)
	testutil.Require(t, "with its room", sitting.Room != nil, true)
	testutil.Expect(t, "the desk", string(sitting.Room.Desk), string(room.Desk))
	testutil.Expect(t, "the face", sitting.Room.Face, "desk")
	testutil.Expect(t, "the drafts", string(sitting.Room.Drafts), string(room.Drafts))
	testutil.Expect(t, "when it was kept", sitting.Room.At, "2026-09-28T11:00:00Z")

	oversized := Room{Face: "desk", Drafts: []byte(`"` + strings.Repeat("x", maxRoomBytes) + `"`), Seq: 9}
	testutil.Expect(t, "a room past its bound is refused", conversation.Keep(oversized, at) != nil, true)
	testutil.Require(t, "rise", conversation.Rise(at), nil)
	testutil.Expect(t, "no room without a sitting", conversation.Keep(room, at).Error(),
		"no sitting is open on this conversation, so there is no room to keep")
}

// Each keep carries the page's sequence, and the mark takes only a keep numbered
// above the one it holds: a request that set out earlier and arrives later
// changes nothing, so older words never overwrite newer ones.
func TestAnOlderKeepArrivingLateLeavesTheMarkAsItWas(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	at := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	record := "metasystem/plans/reviews/review-of-g1-s64.md"
	conversation, err := OpenConversation(directory, "Wido", record)
	testutil.Require(t, "open", err, nil)
	testutil.Require(t, "sit", conversation.Sit(Sitting{Subject: Subject{Kind: SubjectRecord, ID: record},
		Purpose: PurposeReview, StartedAt: at.Format(time.RFC3339)}, at), nil)

	words := func(text string, seq int64) Room {
		return Room{Face: "desk", Drafts: []byte(`{"local-1":{"text":"` + text + `"}}`), Seq: seq}
	}
	testutil.Require(t, "the newer keep", conversation.Keep(words("half a finding", 2), at.Add(time.Minute)), nil)
	testutil.Require(t, "the older keep, late", conversation.Keep(words("hal", 1), at.Add(2*time.Minute)), nil)
	testutil.Require(t, "a replay of the newer", conversation.Keep(words("hal", 2), at.Add(3*time.Minute)), nil)

	again, err := OpenConversation(directory, "Wido", record)
	testutil.Require(t, "open again", err, nil)
	held := again.Sitting().Room
	testutil.Expect(t, "the newer words stand", string(held.Drafts), `{"local-1":{"text":"half a finding"}}`)
	testutil.Expect(t, "under the newer sequence", held.Seq, int64(2))
	testutil.Expect(t, "kept when the newer words were", held.At, "2026-09-28T10:01:00Z")

	testutil.Require(t, "a later keep", again.Keep(words("half a finding, and why", 3), at.Add(4*time.Minute)), nil)
	testutil.Expect(t, "replaces it", string(again.Sitting().Room.Drafts), `{"local-1":{"text":"half a finding, and why"}}`)
	testutil.Expect(t, "with its sequence", again.Sitting().Room.Seq, int64(3))
}

// Two keeps admitted in order publish the mark in that order: the older keep's
// write is held back while the newer one tries to complete, and the file read
// back carries the newer room and sequence.
func TestTwoKeepsAdmittedInOrderPublishTheNewerLast(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	at := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	record := "metasystem/plans/reviews/review-of-g1-s64.md"
	conversation, err := OpenConversation(directory, "Wido", record)
	testutil.Require(t, "open", err, nil)
	testutil.Require(t, "sit", conversation.Sit(Sitting{Subject: Subject{Kind: SubjectRecord, ID: record},
		Purpose: PurposeReview, StartedAt: at.Format(time.RFC3339)}, at), nil)

	words := func(text string, seq int64) Room {
		return Room{Face: "desk", Drafts: []byte(`{"local-1":{"text":"` + text + `"}}`), Seq: seq}
	}
	newerDone := make(chan error, 1)
	conversation.publish = func(path string, held stateFile) error {
		if held.Sitting != nil && held.Sitting.Room != nil && held.Sitting.Room.Seq == 1 {
			// The older keep is writing. The newer one sets out now; where the
			// conversation's lock is free it can finish first, and it is let.
			go func() { newerDone <- conversation.Keep(words("half a finding", 2), at.Add(2*time.Minute)) }()
			if conversation.mu.TryLock() {
				conversation.mu.Unlock()
				testutil.Require(t, "the newer keep", <-newerDone, nil)
				newerDone <- nil
			}
		}
		return publishState(path, held)
	}
	testutil.Require(t, "the older keep", conversation.Keep(words("hal", 1), at.Add(time.Minute)), nil)
	testutil.Require(t, "the newer keep, done", <-newerDone, nil)

	again, err := OpenConversation(directory, "Wido", record)
	testutil.Require(t, "open again", err, nil)
	held := again.Sitting().Room
	testutil.Expect(t, "the newer words are on disk", string(held.Drafts), `{"local-1":{"text":"half a finding"}}`)
	testutil.Expect(t, "under the newer sequence", held.Seq, int64(2))
}

// A state file is published whole: written beside itself under a name no
// reader counts, then put over the old one, so a reader that opened the file
// before the publish reads the old state whole rather than a truncated one, and
// nothing but the state file is left behind (SOL-S67-05).
func TestAStateFileIsPublishedWholeAndNothingIsLeftBeside(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	at := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	record := "metasystem/plans/reviews/review-of-g1-s64.md"
	conversation, err := OpenConversation(directory, "Wido", record)
	testutil.Require(t, "open", err, nil)
	testutil.Require(t, "sit", conversation.Sit(Sitting{Subject: Subject{Kind: SubjectRecord, ID: record},
		Purpose: PurposeReview, StartedAt: at.Format(time.RFC3339)}, at), nil)
	before, err := os.ReadFile(conversation.state)
	testutil.Require(t, "the state as it stands", err, nil)

	reading, err := os.Open(conversation.state)
	testutil.Require(t, "a reader opens the state", err, nil)
	defer reading.Close()
	testutil.Require(t, "keep", conversation.Keep(Room{Face: "desk", Seq: 1}, at.Add(time.Minute)), nil)
	read, err := io.ReadAll(reading)
	testutil.Require(t, "the reader reads", err, nil)
	testutil.Expect(t, "the reader reads the state it opened, whole", string(read), string(before))

	after, err := os.ReadFile(conversation.state)
	testutil.Require(t, "the published state", err, nil)
	var published stateFile
	testutil.Require(t, "it parses whole", json.Unmarshal(after, &published), nil)
	testutil.Expect(t, "with the kept room", published.Sitting.Room.Seq, int64(1))
	entries, err := os.ReadDir(filepath.Dir(conversation.state))
	testutil.Require(t, "the sittings directory", err, nil)
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	testutil.Expect(t, "exactly the state file beside the transcript, no temporary",
		names, []string{filepath.Base(conversation.state)})
}

// A temporary left by a publish that never finished is neither a sitting to
// count nor a state to read.
func TestALeftoverTemporaryIsNeitherCountedNorRead(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	record := "metasystem/plans/reviews/review-of-g1-s64.md"
	other := "metasystem/plans/reviews/review-of-g1-s63.md"
	conversation, err := OpenConversation(directory, "Wido", record)
	testutil.Require(t, "open", err, nil)
	stray, err := json.Marshal(stateFile{Session: "stray", Sitting: &Sitting{
		Subject: Subject{Kind: SubjectRecord, ID: other}, Purpose: PurposeReview}})
	testutil.Require(t, "a stray state", err, nil)
	leftover := filepath.Join(filepath.Dir(conversation.state), filepath.Base(conversation.state)+".crashed.tmp")
	testutil.Require(t, "a leftover temporary", os.WriteFile(leftover, stray, 0o600), nil)

	records, err := sittingsOn(directory, "Wido")
	testutil.Require(t, "the sittings", err, nil)
	testutil.Expect(t, "the leftover is not counted", records, []string{})
	again, err := OpenConversation(directory, "Wido", record)
	testutil.Require(t, "open again", err, nil)
	testutil.Expect(t, "nor read as the state", again.Sitting() == nil, true)
	testutil.Expect(t, "nor its session", again.session, "")
}

// A state file that is there but does not parse is refused in words naming it,
// rather than taken for a conversation with no sitting (SOL-S67-05).
func TestAStateFileThatDoesNotParseIsRefusedByName(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	conversation, err := OpenConversation(directory, "Wido", "metasystem/plans/reviews/review-of-g1-s64.md")
	testutil.Require(t, "open", err, nil)
	testutil.Require(t, "a half-written state", os.WriteFile(conversation.state, []byte(`{"sitting":{"subj`), 0o600), nil)

	_, err = sittingsOn(directory, "Wido")
	testutil.Require(t, "refused", err != nil, true)
	testutil.Expect(t, "naming the file", strings.Contains(err.Error(), conversation.state), true)
}
