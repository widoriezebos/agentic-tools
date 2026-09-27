package partner

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// R-131-ui: an unreadable transcript is archived and truncated to fit.
//
// A conversation file the reader cannot load because of its size is not an error
// a human meets. Astra's B-01 left the residual these tests are about:
// transcripts written past the reader's old ceiling, which took the conversation
// page and the Decisions page down on every restart. The original is kept whole
// beside the transcript, the transcript is rewritten to what fits, and the human
// is told once that it happened.

// One oversized line in the middle costs that one message and nothing else: the
// messages AFTER it are exactly what a scanner would have lost, and they are
// what the human came back for.
func TestAnUnreadableTranscriptOpensWithEveryOtherMessage(t *testing.T) {
	t.Parallel()
	directory, path, _ := archiveUnreadableBed(t)

	conversation, err := OpenConversation(directory, "Wido")

	testutil.Require(t, "the conversation opens", err, nil)
	held := conversation.Messages(0)
	testutil.Require(t, "three messages and the notice", len(held), 4)
	testutil.Expect(t, "the first is still the first", held[0].ID, "m1")
	testutil.Expect(t, "the longest line the reader admits is kept whole",
		len(held[0].Text) > maxLineBytes-200, true)
	testutil.Expect(t, "the message after the oversized one is kept", held[1].ID, "m3")
	testutil.Expect(t, "and so is the one after that", held[2].ID, "m4")
	testutil.Expect(t, "in the file's own order", held[1].Text, "the third thing said")
	notice := held[3]
	testutil.Expect(t, "the notice is the last message, which is the one the page shows first",
		notice.Trimmed, true)
	testutil.Expect(t, "it is a completed answer, which is how the drawer renders it",
		notice.Outcome, OutcomeComplete)
	testutil.Expect(t, "in the Partner's column", notice.Role, RolePartner)
	testutil.Expect(t, "it says the conversation was too large to open",
		strings.Contains(notice.Text, "too large to open"), true)
	testutil.Expect(t, "it names the archive the original is kept at",
		strings.Contains(notice.Text, filepath.Base(archiveOnlyArchive(t, directory))), true)
	testutil.Expect(t, "and says how many messages were set aside",
		strings.Contains(notice.Text, "one message was set aside"), true)
	testutil.Expect(t, "the notice carries its own turn", notice.Turn != "", true)
	testutil.Expect(t, "and its own id", notice.ID != "", true)
	testutil.Expect(t, "the transcript is where it was", fileExists(path), true)
}

// The original is kept whole, byte for byte, beside the transcript: the archive
// is what the human still has, so it is a copy and not a summary.
func TestTheUnreadableTranscriptIsArchivedByteForByte(t *testing.T) {
	t.Parallel()
	directory, path, original := archiveUnreadableBed(t)

	_, err := OpenConversation(directory, "Wido")

	testutil.Require(t, "the conversation opens", err, nil)
	archive := archiveOnlyArchive(t, directory)
	testutil.Expect(t, "the archive is beside the transcript", filepath.Dir(archive), directory)
	testutil.Expect(t, "under the transcript's own name",
		strings.HasPrefix(filepath.Base(archive), "Wido.jsonl."), true)
	testutil.Expect(t, "with the archive's extension", filepath.Ext(archive), ".archived")
	kept, err := os.ReadFile(archive)
	testutil.Require(t, "the archive reads", err, nil)
	testutil.Expect(t, "it is the original, byte for byte", bytes.Equal(kept, original), true)
	rewritten, err := os.ReadFile(path)
	testutil.Require(t, "the transcript reads", err, nil)
	testutil.Expect(t, "and the transcript is no longer the original",
		bytes.Equal(rewritten, original), false)
}

// The second open archives nothing. The rewritten file is one this reader reads,
// which is the whole point of the bound the rewrite keeps to: an archive per
// restart would be a directory of copies and a notice per open.
func TestASecondOpenOfARewrittenTranscriptArchivesNothingMore(t *testing.T) {
	t.Parallel()
	directory, path, _ := archiveUnreadableBed(t)
	first, err := OpenConversation(directory, "Wido")
	testutil.Require(t, "the first open", err, nil)
	rewritten, err := os.ReadFile(path)
	testutil.Require(t, "reading what it left", err, nil)

	second, err := OpenConversation(directory, "Wido")

	testutil.Require(t, "the second open", err, nil)
	testutil.Expect(t, "there is one archive and not two", len(archiveArchives(t, directory)), 1)
	again, err := os.ReadFile(path)
	testutil.Require(t, "reading the transcript again", err, nil)
	testutil.Expect(t, "the transcript was left alone", bytes.Equal(again, rewritten), true)
	testutil.Expect(t, "and it reads back what the first open served",
		archiveIDs(second.Messages(0)), archiveIDs(first.Messages(0)))
	testutil.Expect(t, "with one notice and not two", archiveNotices(second.Messages(0)), 1)
}

// A transcript that reads is untouched: no archive, no notice, and the file as
// it was. Nothing about the load changes for the conversations that are fine.
func TestATranscriptThatReadsIsNotArchived(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "Wido.jsonl")
	archiveWrite(t, path, []string{
		archiveLine(t, Message{ID: "m1", Turn: "t1", Role: RoleHuman, Text: "the first thing said"}),
		archiveLine(t, Message{ID: "m2", Turn: "t1", Role: RolePartner,
			Outcome: OutcomeComplete, Text: "the answer"}),
	})
	before, err := os.ReadFile(path)
	testutil.Require(t, "reading the file", err, nil)

	conversation, err := OpenConversation(directory, "Wido")

	testutil.Require(t, "the conversation opens", err, nil)
	testutil.Expect(t, "both messages are there", archiveIDs(conversation.Messages(0)), "m1,m2")
	testutil.Expect(t, "nothing was archived", len(archiveArchives(t, directory)), 0)
	testutil.Expect(t, "no notice was written", archiveNotices(conversation.Messages(0)), 0)
	after, err := os.ReadFile(path)
	testutil.Require(t, "reading the file again", err, nil)
	testutil.Expect(t, "and the file is as it was", bytes.Equal(after, before), true)
}

// archiveUnreadableBed is a transcript with one oversized line in the middle:
// two messages the reader admits after it, and one line no scanner of this store
// can return.
func archiveUnreadableBed(t *testing.T) (directory, path string, original []byte) {
	t.Helper()
	directory = t.TempDir()
	path = filepath.Join(directory, "Wido.jsonl")
	archiveWrite(t, path, []string{
		// The longest line this store's reader can return, exactly: the line the
		// rewrite must KEEP. A threshold off by one would set it aside, and the
		// open after that would archive the file a second time.
		archiveLineOfExactly(t, Message{ID: "m1", Turn: "t1", Role: RoleHuman}, maxLineBytes-1),
		archiveLine(t, Message{ID: "m2", Turn: "t1", Role: RolePartner, Outcome: OutcomeComplete,
			Text: strings.Repeat("x", maxLineBytes)}),
		archiveLine(t, Message{ID: "m3", Turn: "t2", Role: RoleHuman, Text: "the third thing said"}),
		archiveLine(t, Message{ID: "m4", Turn: "t2", Role: RolePartner, Outcome: OutcomeComplete,
			Text: "the fourth thing said"}),
	})
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the fixture's own transcript: %v", err)
	}
	return directory, path, body
}

// archiveLineOfExactly is one message written to exactly n bytes, padded in its
// own text, so a fixture can sit a line on the reader's bound.
func archiveLineOfExactly(t *testing.T, message Message, n int) string {
	t.Helper()
	bare := archiveLine(t, message)
	message.Text = strings.Repeat("x", n-len(bare))
	line := archiveLine(t, message)
	if len(line) != n {
		t.Fatalf("the fixture's line is %d bytes, wanted %d", len(line), n)
	}
	return line
}

// archiveLine is one message as this store writes it.
func archiveLine(t *testing.T, message Message) string {
	t.Helper()
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("cannot write the fixture's message %s: %v", message.ID, err)
	}
	return string(body)
}

func archiveWrite(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("cannot write the fixture's transcript: %v", err)
	}
}

// archiveArchives names every archive one directory holds.
func archiveArchives(t *testing.T, directory string) []string {
	t.Helper()
	held, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("cannot read %s: %v", directory, err)
	}
	names := []string{}
	for _, entry := range held {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".archived" {
			names = append(names, filepath.Join(directory, entry.Name()))
		}
	}
	return names
}

// archiveOnlyArchive is the one archive a directory is expected to hold.
func archiveOnlyArchive(t *testing.T, directory string) string {
	t.Helper()
	names := archiveArchives(t, directory)
	if len(names) != 1 {
		t.Fatalf("expected one archive in %s, found %d: %v", directory, len(names), names)
	}
	return names[0]
}

func archiveIDs(messages []Message) string {
	held := make([]string, 0, len(messages))
	for _, message := range messages {
		held = append(held, message.ID)
	}
	return strings.Join(held, ",")
}

func archiveNotices(messages []Message) int {
	count := 0
	for _, message := range messages {
		if message.Trimmed {
			count++
		}
	}
	return count
}
