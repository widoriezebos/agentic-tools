package main

// The board's Refresh classifies this clone's journal before it advances.
//
// A push that landed and failed its confirmation leaves a journal entry at
// pushed, and the engine mutates nothing in this clone until somebody
// classifies it. Refresh used to advance the accepted ref and answer a current
// board while every act stayed refused — a successful ledger read standing in
// for journal recovery. It now runs the engine's own recovery rule first, and
// where recovery may not touch what it finds, the entry is left and the answer
// says which entry stands and why.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

func TestTheRefreshAdvanceClassifiesTheJournalFirst(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	at := time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
	record := &goal.RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1",
		SyncMode: goal.SyncRemote, MigrationEpoch: "2026-09-01T00:00:00Z",
		ManifestDigest: strings.Repeat("ab", 32), MigrationMode: "manifest", Revision: 1,
	}
	endpoint := goal.Endpoint{
		Root: root, Remote: "origin", Branch: "refs/heads/main",
		Repository: testgoal.New(map[string][]byte{"plans/goals/backlog.md": goal.RenderRoot(record)},
			at, strings.Repeat("0", 39)+"1"),
	}
	// A pushed entry a live process that is not this one owns. Recovery never
	// takes one of those, so the clone stays excluded and Refresh must say so
	// rather than answer as if nothing were outstanding.
	stranded := goal.Entry{
		Opid: "01J5X0000000000000000000ZZ-mac-ui-1a2b3c4d", Machine: "mac-ui", Lineage: "seat-1",
		Owner: goal.OwnerIdentity{Pid: 1}, Phase: goal.PhasePushed, CreatedAt: "2026-09-20T08:00:00Z",
		Intent: goal.Intent{Verb: "park", Targets: []string{"ui-wedged"}},
	}
	encoded, err := json.Marshal(stranded)
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(root, "artifacts", "agents", "goal-transactions")
	if err := os.MkdirAll(journal, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(journal, stranded.Opid+".json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = uiAdvance(root, endpoint)

	if err == nil {
		t.Fatal("Refresh advanced without classifying the journal it is excluded by")
	}
	if !strings.Contains(err.Error(), stranded.Opid) {
		t.Fatalf("the answer does not name the entry that stands: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "a live owner's entry") {
		t.Fatalf("the answer does not say why it stands, in the engine's words: %q", err.Error())
	}
}
