package partner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// Carrying a conversation out of the checkout it used to be written in.
//
// Sol's second finding: the store moved (g1-s53 D11) and nothing moved what was
// already in it, so a human who had talked to the Partner met an empty history
// while their transcript stayed inside the checkout a critic is handed. What is
// proved here is what a human would lose without it — the old messages read back
// from the new place, and no copy of them left behind.
//
// Nothing here touches the account's own registry home: Carry is given both
// directories, so both are this test's own temporary ones. The home itself is
// proved outside the checkout in grants_test.go, which writes nothing at all.

// oldPlace is a pre-upgrade conversation directory, as the state root held it:
// one human's transcript and state file, the unnamed seat's pair, the wire
// journal, and one file that is none of those.
func oldPlace(t *testing.T) string {
	t.Helper()
	was := filepath.Join(t.TempDir(), "artifacts", "agents", "ui", "partner")
	testutil.Require(t, "making the old place", os.MkdirAll(was, 0o755), nil)
	for name, body := range map[string]string{
		"Wido.jsonl": `{"id":"m1","turn":"t1","role":"human","text":"what does the limit protect?"}` + "\n",
		"Wido.json":  `{"human":"Wido","session":"s1","updatedAt":"2026-09-25T09:00:00Z"}`,
		"seat.jsonl": `{"id":"m2","turn":"t2","role":"human","text":"asked with no name"}` + "\n",
		"seat.json":  `{"human":"","session":"s2","updatedAt":"2026-09-25T09:00:00Z"}`,
		"wire.jsonl": `{"frame":"initialize"}` + "\n",
		"notes.txt":  "something a human left here",
	} {
		testutil.Require(t, "writing "+name, os.WriteFile(filepath.Join(was, name), []byte(body), 0o600), nil)
	}
	return was
}

func names(t *testing.T, directory string) []string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	testutil.Require(t, "reading "+directory, err, nil)
	found := []string{}
	for _, entry := range entries {
		found = append(found, entry.Name())
	}
	return found
}

// The whole of the finding: the conversation moves, the transcript reads back
// from the new place, and nothing of it is left in the checkout.
func TestAConversationWrittenBeforeTheMoveIsCarriedIntoTheNewPlace(t *testing.T) {
	t.Parallel()

	was := oldPlace(t)
	directory := filepath.Join(t.TempDir(), "ui", "partner", "example-abc123")

	moved, err := Carry(directory, was)

	testutil.Require(t, "carrying the conversation", err, nil)
	testutil.Expect(t, "how many files moved", moved, 5)
	testutil.Expect(t, "what is in the new place", names(t, directory),
		[]string{"Wido.json", "Wido.jsonl", "seat.json", "seat.jsonl", "wire.jsonl"})
	// Nothing private stayed in the checkout: what is left is the one file that
	// was never the conversation's.
	testutil.Expect(t, "what is left in the old place", names(t, was), []string{"notes.txt"})

	// And the history is there to be read, which is the thing the empty
	// transcript was hiding.
	conversation, err := OpenConversation(directory, "Wido")
	testutil.Require(t, "opening the carried conversation", err, nil)
	messages := conversation.Messages(10)
	testutil.Require(t, "the messages came with it", len(messages), 1)
	testutil.Expect(t, "whole", messages[0].Text, "what does the limit protect?")
	testutil.Expect(t, "and the state file beside it", conversation.Session(), "s1")
}

// The old directory goes with the last of its files, so the checkout is left
// without even the shape of a store it no longer holds.
func TestTheOldDirectoryGoesOnceNothingOfTheConversationIsLeftInIt(t *testing.T) {
	t.Parallel()

	was := filepath.Join(t.TempDir(), "artifacts", "agents", "ui", "partner")
	testutil.Require(t, "making the old place", os.MkdirAll(was, 0o755), nil)
	testutil.Require(t, "writing the transcript",
		os.WriteFile(filepath.Join(was, "Wido.jsonl"), []byte("{}\n"), 0o600), nil)
	directory := filepath.Join(t.TempDir(), "carried")

	moved, err := Carry(directory, was)

	testutil.Require(t, "carrying the conversation", err, nil)
	testutil.Expect(t, "how many files moved", moved, 1)
	_, statErr := os.Stat(was)
	testutil.Expect(t, "that the old directory is gone", os.IsNotExist(statErr), true)
}

// On this machine the files were carried by hand and the old place removed, so
// the path that must cost nothing is the one where there is nothing to carry.
func TestAnAbsentOldPlaceCarriesNothingAndIsNotAFailure(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "carried")

	moved, err := Carry(directory, filepath.Join(t.TempDir(), "never", "existed"))

	testutil.Require(t, "that an absent old place is no failure", err, nil)
	testutil.Expect(t, "how many files moved", moved, 0)
	// And it created nothing: a directory made for a carry that had nothing to
	// carry would be this function writing where it was not asked to.
	_, statErr := os.Stat(directory)
	testutil.Expect(t, "that nothing was made", os.IsNotExist(statErr), true)
}

// An old place holding nothing of a conversation is the same no-op, so a
// directory somebody left behind empty does not make a store.
func TestAnOldPlaceWithNoConversationInItCarriesNothing(t *testing.T) {
	t.Parallel()

	was := t.TempDir()
	testutil.Require(t, "writing a file that is not the conversation's",
		os.WriteFile(filepath.Join(was, "notes.txt"), []byte("x"), 0o600), nil)
	directory := filepath.Join(t.TempDir(), "carried")

	moved, err := Carry(directory, was)

	testutil.Require(t, "that it is no failure", err, nil)
	testutil.Expect(t, "how many files moved", moved, 0)
	_, statErr := os.Stat(directory)
	testutil.Expect(t, "that nothing was made", os.IsNotExist(statErr), true)
}

// The carry is for the FIRST open of a workspace's directory. A directory that
// already holds a conversation is a store in use, and carrying into it would be
// this function deciding which of two conversations is the real one.
func TestADirectoryThatAlreadyHoldsAConversationIsLeftAlone(t *testing.T) {
	t.Parallel()

	for _, already := range []string{"Someone.jsonl", "Someone.json", "wire.jsonl"} {
		t.Run(already, func(t *testing.T) {
			was := oldPlace(t)
			directory := filepath.Join(t.TempDir(), "carried")
			testutil.Require(t, "making the new place", os.MkdirAll(directory, 0o700), nil)
			testutil.Require(t, "writing the file already there",
				os.WriteFile(filepath.Join(directory, already), []byte("{}\n"), 0o600), nil)

			moved, err := Carry(directory, was)

			testutil.Require(t, "that it is no failure", err, nil)
			testutil.Expect(t, "how many files moved", moved, 0)
			testutil.Expect(t, "what is in the new place", names(t, directory), []string{already})
			// And the old place is untouched, so nothing was half-carried.
			testutil.Expect(t, "how many files are still in the old place", len(names(t, was)), 6)
		})
	}
}

// A directory holding something that is not a conversation is a first open all
// the same, so the carry runs — and the file that is not a conversation's stays.
func TestADirectoryHoldingSomethingElseIsStillAFirstOpen(t *testing.T) {
	t.Parallel()

	was := oldPlace(t)
	directory := filepath.Join(t.TempDir(), "carried")
	testutil.Require(t, "making the new place", os.MkdirAll(directory, 0o700), nil)
	testutil.Require(t, "writing a file that is not the conversation's",
		os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("x"), 0o600), nil)

	moved, err := Carry(directory, was)

	testutil.Require(t, "carrying the conversation", err, nil)
	testutil.Expect(t, "how many files moved", moved, 5)
	testutil.Expect(t, "what is in the new place", names(t, directory),
		[]string{"Wido.json", "Wido.jsonl", "notes.txt", "seat.json", "seat.jsonl", "wire.jsonl"})
}

// A name the new place has already taken refuses the whole carry, before
// anything has moved, and says which name it was.
//
// The gate above means no conversation FILE can be in the way, so what reaches
// this is something else standing at one of those names — here a directory,
// which is what a build that once kept a name for something else would leave.
// Whatever it is, it is not written over: the refusal is the Partner not being
// served, and both conversations are still whole.
func TestANameTheNewPlaceHasTakenRefusesTheCarryAndMovesNothing(t *testing.T) {
	t.Parallel()

	was := oldPlace(t)
	directory := filepath.Join(t.TempDir(), "carried")
	testutil.Require(t, "making the new place", os.MkdirAll(filepath.Join(directory, "seat.json"), 0o700), nil)

	moved, err := Carry(directory, was)

	if err == nil {
		t.Fatalf("a carry over a name the new place had taken was accepted")
	}
	testutil.Expect(t, "how many files moved", moved, 0)
	testutil.Expect(t, "that it says which name", strings.Contains(err.Error(), "seat.json"), true)
	testutil.Expect(t, "that it says it will not write over it",
		strings.Contains(err.Error(), "will not write over it"), true)
	// Nothing moved: the whole conversation is still where it was, so the next
	// open can try again once whatever is in the way has gone.
	testutil.Expect(t, "how many files are still in the old place", len(names(t, was)), 6)
	testutil.Expect(t, "what is in the new place", names(t, directory), []string{"seat.json"})
}

// The old place, spelled as the state root spelled it. It is read from the
// constant rather than repeated, so a build that moved it again would move this.
func TestTheOldPlaceIsTheStateRootsOwnAgentsDirectory(t *testing.T) {
	t.Parallel()

	testutil.Expect(t, "where a checkout kept its conversations", LegacyRelative,
		"artifacts/agents/ui/partner")
	testutil.Expect(t, "that it is not where they go now",
		strings.Contains(Directory("/home/someone/.metasystem", "/tmp/example"), LegacyRelative), false)
}
