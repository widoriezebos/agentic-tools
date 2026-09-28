package goal

// An idempotent repeat leaves no record (R-129-ui, U-idem): the effect the
// act asks for already stands, so the operation is success, builds no commit,
// pushes nothing, and discards the journal entry it opened.

import (
	"os"
	"strings"
	"testing"
)

func journalEntryCount(t *testing.T, root string) int {
	t.Helper()
	entries, err := Entries(root)
	if err != nil && !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") {
		t.Fatal(err)
	}
	return len(entries)
}

func TestAnAlreadyHoldingActLeavesNoRecord(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	before := acceptedTipForEndpoint(t, endpoint)
	entries := journalEntryCount(t, endpoint.Root)
	request := verbReqFor(endpoint, "01J5X000000000000000000AH1", "mac-a")
	result, err := Publish(endpoint, PublishRequest{
		Opid: request.opid(), Machine: "mac-a", Lineage: "m1",
		Intent:  Intent{Verb: "park", Targets: []string{"g"}},
		Message: "goal park g",
		Mutate: func(string) ([]Change, error) {
			return nil, AlreadyHolds{Reason: "goal g is already paused"}
		},
	})
	if err != nil || result.Outcome != OutcomeAbandoned || !result.Unchanged || result.Detail != "goal g is already paused" || result.Commit != "" {
		t.Fatalf("an already-holding act was not an unchanged success: %+v %v", result, err)
	}
	if after := acceptedTipForEndpoint(t, endpoint); after != before {
		t.Fatalf("an already-holding act moved the ledger: %s to %s", before, after)
	}
	if count := journalEntryCount(t, endpoint.Root); count != entries {
		t.Fatalf("an already-holding act left a journal entry: %d before, %d after", entries, count)
	}
	if _, readErr := ReadEntry(endpoint.Root, request.opid()); readErr == nil {
		t.Fatal("the repeat's journal entry survived")
	}
}

func TestDiscardUnpushedRefusesABuiltEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := CreateEntry(root, "op-built", "mac-a", "m1", Intent{Verb: "park"}); err != nil {
		t.Fatal(err)
	}
	if err := RecordSteps(root, "op-built", "", "abc123"); err != nil {
		t.Fatal(err)
	}
	if err := DiscardUnpushed(root, "op-built"); err == nil {
		t.Fatal("a built entry was discarded")
	}
	if err := DiscardUnpushed(root, "../escape"); err == nil {
		t.Fatal("a path-shaped opid was accepted")
	}
	if _, err := CreateEntry(root, "op-fresh", "mac-a", "m1", Intent{Verb: "park"}); err != nil {
		t.Fatal(err)
	}
	if err := DiscardUnpushed(root, "op-fresh"); err != nil {
		t.Fatalf("a fresh entry was not discarded: %v", err)
	}
	if _, err := ReadEntry(root, "op-fresh"); err == nil {
		t.Fatal("the fresh entry survived its discard")
	}
}
