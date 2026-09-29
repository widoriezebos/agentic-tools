package evidence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// Round B2, F-3 as ruled in B2-2 (R1): only items the inventory
// enumerates are accepted; a segment, the root's structure directories and
// paths inside items are refused, and each item is judged.
func TestOnlyEnumeratedItemsResolveAndEachIsJudged(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	open := bed.chain(t, "open-goal-chain", 300, 50, "g-open")
	done := bed.chain(t, "done-chain", 300, 50, "g-done")
	for _, structure := range []string{filepath.Join(bed.root, "agents"), filepath.Join(bed.root, "agents", bed.segment.Git), filepath.Join(bed.root, "disposals"),
		filepath.Join(bed.root, "segments"), bed.root, filepath.Join(done, "jobs", "done-chain.json")} {
		if _, err := bed.env.Resolve(context.Background(), structure); err == nil || !strings.Contains(err.Error(), "not an item") {
			t.Fatalf("%s must never resolve to an item: %v", structure, err)
		}
	}
	plan := bed.preview(t, "", open, done)
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	if gone(open) || !gone(done) {
		t.Fatalf("the open goal's chain is held, the concluded one removed: %+v", outcomes)
	}
}

// Round B2, F-4 (Wido's option B; Round B2-3): the machine pass never
// removes an item and never settles a person's removal; it reports it with
// the command the person runs.
func TestTheMachinePassNeverFinishesAPersonsRemoval(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	item := bed.chain(t, "old-chain", 300, 400, "g-done")
	files, err := diskstore.Inventory(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	tombstone := diskstore.Tombstone{Schema: diskstore.TombstoneSchema, Item: "old-chain", Kind: diskstore.KindChain, Segment: bed.segment.Git,
		Files: files, History: []diskstore.HistoryEntry{}, InventoryDigest: diskstore.InventoryDigest(files), Step: diskstore.StepRemove,
		Rule: diskstore.RulePerson, By: "Wido", At: boundNow, Receipt: "01PERSONRECEIPT", State: diskstore.StateBegun,
		Export:    &diskstore.ExportRef{Dir: "/Volumes/Gone", Archive: "/Volumes/Gone/x.tar.gz", ArchiveSHA256: "00"},
		Disposing: "old-chain.disposing-01STAGE"}
	data, _ := json.MarshalIndent(tombstone, "", "  ")
	if err := os.WriteFile(diskstore.RemovedTombstonePath(item), data, 0o644); err != nil {
		t.Fatal(err)
	}
	position := bed.bound().ReportSegment(context.Background(), bed.segment, settingsOf(1))
	t.Logf("position: %+v", position)
	lines := receipts(t, bed.segment)
	t.Logf("receipts: %+v", lines)
	if gone(item) || len(lines) != 0 {
		t.Fatalf("MACHINE REMOVAL: the pass touched a person's removal of %s: %+v", item, lines)
	}
	if !strings.Contains(strings.Join(position.Pending, "\n"), "metasystem evidence dispose "+item+" --preview") {
		t.Fatalf("the report names the person's command: %+v", position.Pending)
	}
}

// Round B2, F-7: every item is judged at its own critical section; a goal
// reopened between two items is seen by the second item's judgement.
func TestAReopenBetweenItemsIsSeenByTheNextJudgement(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view(identityA, map[string]string{"g-x": GoalDone})}}
	exclusions := bed.exclusions(fake)
	first := Item{Kind: diskstore.KindChain, Name: "a", Goal: "g-x"}
	_ = exclusions.Judge(context.Background(), bed.segment, first)
	// goal reopen publishes (under the bound lock shared, between items)
	fake.fetched[bed.installation] = view(identityA, map[string]string{"g-x": GoalOpen})
	second := exclusions.Judge(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "b", Goal: "g-x"})
	t.Logf("fetches=%d second=%+v", fake.fetches, second)
	if blocking(second) == "" {
		t.Fatalf("EXCLUSION FAILS OPEN: goal g-x reopened before item b's judgement, which still judged it clear")
	}
}

// Round B2, F-1: every disposal mints its own receipt id; a begun
// tombstone is committed only by a ledger line with its id and its item.
func TestEveryDisposalHasItsOwnReceiptID(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	first := bed.chain(t, "first", 300, 50, "g-done")
	plan := bed.preview(t, "", first)
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	lines := receipts(t, bed.segment)
	if len(lines) != 1 || lines[0].ID == "" {
		t.Fatalf("the first disposal's receipt has an id: %+v %+v", outcomes, lines)
	}
	if tombstone, err := diskstore.ReadTombstone(diskstore.RemovedTombstonePath(first)); err != nil || tombstone.Receipt != lines[0].ID {
		t.Fatalf("its tombstone names the same id: %+v %v", tombstone, err)
	}
	// A second removal (of an OPEN goal's chain, overridden by the person)
	// crashes after its begun tombstone, before its rename and receipt.
	held := bed.chain(t, "held", 300, 400, "g-open")
	files, _ := diskstore.Inventory(context.Background(), held)
	tombstone := diskstore.Tombstone{Schema: diskstore.TombstoneSchema, Item: "held", Kind: diskstore.KindChain, Segment: bed.segment.Git,
		Files: files, History: []diskstore.HistoryEntry{}, InventoryDigest: diskstore.InventoryDigest(files), Step: diskstore.StepRemove,
		Rule: diskstore.RulePerson, By: "Wido", At: boundNow, Receipt: lines[0].ID, State: diskstore.StateBegun, Disposing: "held.disposing-01X"}
	data, _ := json.MarshalIndent(tombstone, "", "  ")
	if err := os.WriteFile(diskstore.RemovedTombstonePath(held), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if committed, _ := diskstore.ReceiptCommitted(bed.segment.Ledger(), tombstone.Receipt, tombstone.Item); committed {
		t.Fatal("another item's receipt never commits this removal")
	}
	if committed, _ := diskstore.ReceiptCommitted(bed.segment.Ledger(), "", "first"); committed {
		t.Fatal("an empty id is never committed")
	}
	settled, err := diskstore.SettlePersonDisposal(context.Background(), held, bed.segment.Ledger(), diskstore.Syncer{}, "01S")
	if err != nil || settled.Finished || gone(held) {
		t.Fatalf("UNCOMMITTED REMOVAL FINISHED: %s was removed with no receipt of its own: %+v %v", held, settled, err)
	}
}

// Round B2, F-11: an installation whose state root cannot be resolved
// makes the receipt clause Unknown for its segment; no guessed ledger.
func TestAnUnresolvableStateRootHoldsTheSegment(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "old", 300, 400, "")
	// No template conf and no repository: the state root is unknown.
	if err := os.Remove(filepath.Join(bed.installation, "metasystem.conf")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(bed.gitRoot, ".git")); err != nil {
		t.Fatal(err)
	}
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view(identityA, map[string]string{})}}
	pass := bed.judge(bed.exclusions(fake))
	if pass.clear[dir] || !strings.Contains(pass.unknown, "state root") {
		t.Fatalf("an unresolvable state root holds every item: %+v", pass)
	}
}

// Round B2, F-13: a held item is never settled: its checkout's payload is
// not re-mirrored or collected before the person's --override.
func TestAHeldItemIsNotSettledBeforeTheOverride(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	dir := bed.chain(t, "held-payload", 300, 50, "g-open")
	jobs := filepath.Join(bed.installation, "artifacts", "agents", "jobs")
	payload := filepath.Join(bed.installation, "artifacts", "agents", "held-payload")
	if err := os.MkdirAll(payload, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, "held-payload.json"), []byte(`{"jobId":"held-payload","status":"completed","chainClosed":true,"goalId":"g-open"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := bed.preview(t, "", dir)
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	if outcomes[0].Done || !strings.Contains(outcomes[0].Line, "held: goal g-open open") {
		t.Fatalf("%+v", outcomes)
	}
	if _, err := os.Stat(payload); err != nil {
		t.Fatalf("the payload of a held chain is left alone: %v", err)
	}
}

// Round B2, F-7: with the tip seam, an unmoved accepted tip reuses the
// judgement's view and a moved one is projected afresh without a fetch.
func TestAMovedAcceptedTipIsProjectedAfresh(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: view(identityA, map[string]string{"g-x": GoalDone})},
		accepted: map[string]LedgerView{bed.installation: view(identityA, map[string]string{"g-x": GoalOpen})}}
	exclusions := bed.exclusions(fake)
	tip := "9498700a9"
	exclusions.Tip = func(context.Context, string) (string, error) { return tip, nil }
	if blocking(exclusions.Judge(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "a", Goal: "g-x"})) != "" {
		t.Fatal("the fetched view has g-x done")
	}
	if blocking(exclusions.Judge(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "b", Goal: "g-x"})) != "" || fake.fetches != 1 {
		t.Fatalf("an unmoved tip reuses the view without a fetch: fetches=%d", fake.fetches)
	}
	fake.accepted[bed.installation] = LedgerView{Tip: "a-reopen", Identity: identityA, States: map[string]string{"g-x": GoalOpen}}
	tip = "a-reopen"
	if blocking(exclusions.Judge(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "c", Goal: "g-x"})) == "" || fake.fetches != 1 {
		t.Fatalf("a moved tip is projected afresh from the accepted ledger: fetches=%d", fake.fetches)
	}
}

// Round B2, F-7: a citation written after one item's judgement is seen by
// the next item's, not only by the next pass.
func TestACitationWrittenBetweenItemsIsSeen(t *testing.T) {
	t.Parallel()
	bed := newCitationBed(t)
	bed.complete(t)
	index := bed.fresh()
	if files, _, _ := index.Cited(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "first"}); len(files) != 0 {
		t.Fatalf("%v", files)
	}
	bed.write(t, "late.md", []byte("see "+bed.segment.Git+"/second"))
	if files, unknown, pending := index.Cited(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "second"}); len(files) != 1 {
		t.Fatalf("the citation written after the first item is seen by the second: %v %q %q", files, unknown, pending)
	}
}

// Round B2, F-10: in local mode a peer whose ledger facts cannot be read
// makes the union unknown: every item of that identity is held.
func TestALocalModePeerWithUnreadableFactsMakesTheUnionUnknown(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "local", 300, 400, "g")
	here := view(identityA, map[string]string{"g": GoalDone})
	here.Local = true
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: here}}
	exclusions := bed.exclusions(fake)
	exclusions.Unreadable = []string{"/elsewhere/clone/metasystem"}
	pass := bed.judge(exclusions)
	if pass.clear[dir] || !strings.Contains(pass.unknown, "/elsewhere/clone/metasystem") {
		t.Fatalf("an unreadable peer holds the identity: %+v", pass)
	}
}
