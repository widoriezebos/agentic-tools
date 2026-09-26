package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
)

// The private store's housekeeping (g1-s54 D1): at every tick it asks both
// owners, and the tick is the caller's.
//
// The cadence is the fleet fetch loop's, for the reason that loop has it: a
// daily sweep proven by waiting a day is a suite that takes a day, so the server
// passes a real ticker and this passes a channel it sends on.
func TestUIStoreHousekeepingAsksBothOwnersOnEveryTick(t *testing.T) {
	t.Parallel()
	journal := partner.NewJournal(filepath.Join(t.TempDir(), "wire.jsonl"))
	testutil.Require(t, "opening the journal", journal.Open(), nil)
	t.Cleanup(func() { _ = journal.Close() })
	_, err := journal.Write([]byte("a frame of one runtime's life\n"))
	testutil.Require(t, "journalling a frame", err, nil)
	asked := 0
	said := []string{}
	keeper := &storeKeeper{
		journal: journal, wire: 1,
		trim: func() (int, error) { asked++; return asked, nil },
		say:  func(line string) { said = append(said, line) },
	}

	// At start, which is what the server does before it listens.
	keeper.Consider()
	testutil.Expect(t, "the conversation owner was asked once", asked, 1)
	testutil.Expect(t, "the journal rotated", len(lines(t, journal.Previous())), 1)

	// And then on every tick.
	ticks := make(chan time.Time)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		keeper.Run(context.Background(), ticks)
	}()
	ticks <- time.Time{}
	ticks <- time.Time{}
	close(ticks)
	<-stopped

	testutil.Expect(t, "the conversation owner was asked once per tick", asked, 3)
	testutil.Expect(t, "and every sweep that did something said so", len(said) >= 3, true)
	testutil.Expect(t, "the rotation is named in the seat's own words",
		strings.Contains(said[0], "the Partner's wire journal passed its bound and was rotated"), true)
	testutil.Expect(t, "and the trim says where what went is still kept",
		strings.Contains(said[1], "what they said is in the records"), true)
}

// The loop ends with the context, so a server that is shutting down is not
// trimming a transcript while the next one takes the checkout.
func TestUIStoreHousekeepingEndsWithTheContext(t *testing.T) {
	t.Parallel()
	asked := 0
	keeper := &storeKeeper{trim: func() (int, error) { asked++; return 0, nil }}
	ctx, stop := context.WithCancel(context.Background())
	ticks := make(chan time.Time)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		keeper.Run(ctx, ticks)
	}()

	ticks <- time.Time{}
	stop()
	<-stopped

	testutil.Expect(t, "it swept once and then ended", asked, 1)
}

// A refusal from one owner does not stop the other, and neither is said twice:
// they hold different files, and the store is smaller for either one.
func TestUIStoreHousekeepingAsksTheSecondOwnerAfterTheFirstRefuses(t *testing.T) {
	t.Parallel()
	gone := filepath.Join(t.TempDir(), "gone")
	journal := partner.NewJournal(filepath.Join(gone, "wire.jsonl"))
	testutil.Require(t, "making the directory", os.MkdirAll(gone, 0o700), nil)
	testutil.Require(t, "opening the journal", journal.Open(), nil)
	t.Cleanup(func() { _ = journal.Close() })
	_, err := journal.Write([]byte("frames\n"))
	testutil.Require(t, "journalling", err, nil)
	// The directory the journal lives in goes, so the rename cannot land: the
	// rotation refuses, and the refusal must not cost the other owner its sweep.
	testutil.Require(t, "taking the directory away", os.RemoveAll(gone), nil)
	asked := 0
	said := []string{}
	keeper := &storeKeeper{
		journal: journal, wire: 1,
		trim: func() (int, error) { asked++; return 0, errors.New("one transcript could not be read") },
		say:  func(line string) { said = append(said, line) },
	}

	keeper.Consider()

	testutil.Require(t, "both owners said something", len(said), 2)
	testutil.Expect(t, "the journal's refusal is the seat's own line",
		strings.Contains(said[0], "could not be rotated"), true)
	testutil.Expect(t, "the conversation owner was still asked", asked, 1)
	testutil.Expect(t, "and its refusal too", said[1], "one transcript could not be read")
}

// A seat with no Partner has neither owner, and a sweep is then nothing at all
// rather than a refusal.
func TestUIStoreHousekeepingWithNoOwnersDoesNothing(t *testing.T) {
	t.Parallel()
	said := []string{}
	keeper := &storeKeeper{say: func(line string) { said = append(said, line) }}
	keeper.Consider()
	testutil.Expect(t, "nothing was said", len(said), 0)
}

// D2: housekeeping touches only what is under the account's home. Nothing in a
// checkout and nothing under the state root changes, byte for byte, when a sweep
// rotates a journal and trims a transcript to its floor.
func TestUIStoreHousekeepingChangesNothingOutsideTheStore(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	checkout := filepath.Join(outside, "repository")
	plantOutside(t, filepath.Join(checkout, "plans", "goals"), "backlog.md", "the project's own records")
	plantOutside(t, filepath.Join(checkout, "metasystem", "artifacts", "agents"), "ui.json", "the state root's own")
	before := tree(t, outside)

	store := t.TempDir()
	journal := partner.NewJournal(filepath.Join(store, "wire.jsonl"))
	testutil.Require(t, "opening the journal", journal.Open(), nil)
	t.Cleanup(func() { _ = journal.Close() })
	_, err := journal.Write([]byte("frames\n"))
	testutil.Require(t, "journalling", err, nil)
	conversation, err := partner.OpenConversation(store, "wido")
	testutil.Require(t, "opening the conversation", err, nil)
	now := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	for turn := 0; turn < 105; turn++ {
		name := "turn-" + strconv.Itoa(turn)
		testutil.Require(t, "appending "+name+" question", conversation.Append(partner.Message{
			ID: name + "-asked", Turn: name, Role: partner.RoleHuman,
			Text: strings.Repeat("q", 1000), At: now.Format(time.RFC3339),
		}), nil)
		testutil.Require(t, "appending "+name+" answer", conversation.Append(partner.Message{
			ID: name + "-answered", Turn: name, Role: partner.RolePartner,
			Outcome: partner.OutcomeComplete, Text: "answered", At: now.Format(time.RFC3339),
		}), nil)
	}
	keeper := &storeKeeper{
		journal: journal, wire: 1,
		trim: func() (int, error) {
			return conversation.Trim(partner.TrimBounds{Bytes: 100 << 10}, now)
		},
	}

	keeper.Consider()

	testutil.Expect(t, "the journal rotated", len(lines(t, journal.Previous())), 1)
	testutil.Expect(t, "the transcript was trimmed to its floor", len(conversation.Messages(0)), 201)
	testutil.Expect(t, "and nothing outside the store changed", tree(t, outside), before)
}

// Housekeeping reaches a transcript nobody has opened this run, which is what a
// sweep at start has to do: the conversations the service holds are the ones
// somebody spoke into, and at start there are none.
func TestUIStoreHousekeepingFindsTheStoresOwnTranscripts(t *testing.T) {
	t.Parallel()
	store := t.TempDir()
	for _, name := range []string{"wido.jsonl", "seat.jsonl", "wire.jsonl", "wire.1.jsonl"} {
		plantOutside(t, store, name, "{}\n")
	}
	plantOutside(t, store, "wido.json", "{}\n")

	humans, err := partner.Humans(store)

	testutil.Require(t, "reading the store", err, nil)
	testutil.Expect(t, "every human's transcript and neither wire journal", humans,
		[]string{"seat", "wido"})
}

func plantOutside(t *testing.T, directory, name, body string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("cannot make %s: %v", directory, err)
	}
	if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0o600); err != nil {
		t.Fatalf("cannot write %s: %v", name, err)
	}
}

// tree is every file under a directory with what it holds, so "nothing changed"
// is a comparison and not a claim.
func tree(t *testing.T, root string) []string {
	t.Helper()
	held := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		held = append(held, relative+" = "+string(body))
		return nil
	})
	if err != nil {
		t.Fatalf("cannot read %s: %v", root, err)
	}
	sort.Strings(held)
	return held
}

// lines is one file's whole lines, and nothing where there is no file.
func lines(t *testing.T, path string) []string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("cannot read %s: %v", path, err)
	}
	trimmed := strings.TrimSuffix(string(body), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
