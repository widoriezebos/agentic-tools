package partner

import (
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
