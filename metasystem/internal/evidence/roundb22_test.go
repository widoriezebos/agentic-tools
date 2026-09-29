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

// V3a: a case variant of a structure directory on a case-insensitive
// volume (macOS default) resolves as an unsegmented item whose path is
// the structure directory itself; --override then removes every segment.
func TestACaseVariantOfAgentsIsNotAnItem(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	mine := bed.chain(t, "mine", 300, 50, "g-done")
	other := filepath.Join(bed.root, "agents", "0123456789ab", "someone-elses-chain")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(other, "x.log"), []byte("other segment's evidence"), 0o644)
	upper := filepath.Join(bed.root, "AGENTS")
	targets, err := bed.env.Resolve(context.Background(), upper)
	t.Logf("Resolve(%s) = %d targets err=%v", upper, len(targets), err)
	if err != nil {
		if !strings.Contains(err.Error(), "not an item") {
			t.Fatalf("refused, but not as \"not an item\": %v", err)
		}
		return
	}
	t.Logf("target item: kind=%s name=%s path=%s ", targets[0].Item.Kind, targets[0].Item.Name, targets[0].Item.Path)
	plan, err := bed.env.Preview(context.Background(), targets, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("plan item: state=%s held=%v files=%d", plan.Items[0].State, plan.Items[0].Held, plan.Items[0].Files)
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{Override: true})
	t.Logf("outcomes: %+v", outcomes)
	if gone(mine) || gone(other) {
		t.Fatalf("WRONG DELETION: naming %s removed the whole agents directory: mine gone=%v other segment gone=%v", upper, gone(mine), gone(other))
	}
	t.Fatalf("resolved %s as an item (%s) though it is the agents directory", upper, targets[0].Item.Path)
}

// V3b: an upper-case spelling of a real segment resolves as ONE legacy
// chain whose path is the whole segment directory.
func TestAnUpperCaseSegmentPathIsNotAnItem(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	a := bed.chain(t, "a", 300, 50, "g-done")
	b := bed.chain(t, "b", 300, 50, "g-open")
	upper := filepath.Join(bed.root, "agents", strings.ToUpper(bed.segment.Git))
	targets, err := bed.env.Resolve(context.Background(), upper)
	t.Logf("Resolve(%s) = %d err=%v", upper, len(targets), err)
	if err != nil {
		return
	}
	for _, target := range targets {
		t.Logf("  kind=%s name=%s path=%s ", target.Item.Kind, target.Item.Name, target.Item.Path)
	}
	plan, _ := bed.env.Preview(context.Background(), targets, nil, "")
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{Override: true})
	t.Logf("outcomes: %+v", outcomes)
	if gone(a) || gone(b) {
		t.Fatalf("WRONG DELETION: the segment path in upper case removed the whole segment as one item (a gone=%v, b (open goal) gone=%v)", gone(a), gone(b))
	}
}

// V3c: a removal tombstone (a record others count) resolves as a chain
// item and is removed by a person's plan.
func TestATombstoneIsNotAnItem(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	first := bed.chain(t, "first", 300, 50, "g-done")
	plan := bed.preview(t, "", first)
	bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	tomb := diskstore.RemovedTombstonePath(first)
	if _, err := os.Stat(tomb); err != nil {
		t.Fatalf("setup: %v", err)
	}
	targets, err := bed.env.Resolve(context.Background(), tomb)
	t.Logf("Resolve(%s) = %d err=%v", tomb, len(targets), err)
	if err != nil {
		return
	}
	t.Logf("  kind=%s name=%s", targets[0].Item.Kind, targets[0].Item.Name)
	plan2, _ := bed.env.Preview(context.Background(), targets, nil, "")
	t.Logf("plan: %+v", plan2.Items[0])
	outcomes := bed.env.Execute(context.Background(), plan2, ExecuteOptions{Override: true})
	t.Logf("outcomes: %+v", outcomes)
	if gone(tomb) {
		t.Fatalf("RECORD DELETED: the tombstone %s of a done removal was removed as an item", tomb)
	}
	t.Fatalf("a tombstone file resolved as an item: %s", tomb)
}

// V3d: a path inside a legacy (unsegmented) chain resolves as a chain
// item instead of being refused as "inside an item".
func TestAPathInsideALegacyChainIsNotAnItem(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	legacy := filepath.Join(bed.root, "agents", "legacy-chain")
	os.MkdirAll(filepath.Join(legacy, "jobs"), 0o755)
	os.WriteFile(filepath.Join(legacy, "jobs", "legacy-chain.json"), []byte(`{"jobId":"legacy-chain"}`), 0o644)
	inside := filepath.Join(legacy, "jobs")
	targets, err := bed.env.Resolve(context.Background(), inside)
	t.Logf("Resolve(%s) = %d err=%v", inside, len(targets), err)
	if err == nil {
		t.Fatalf("a path inside the legacy chain %s resolved as item kind=%s name=%s", legacy, targets[0].Item.Kind, targets[0].Item.Name)
	}
}

// V4: a person's removal interrupted after its set-aside (rename done,
// receipt not written). The machine reports the person's command; does
// that command finish or roll it back?
func TestAPersonsRemovalInterruptedAfterAsideIsSettledByThePlan(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	item := bed.chain(t, "aside-chain", 300, 50, "g-done")
	plan := bed.preview(t, "", item)
	files, _ := diskstore.Inventory(context.Background(), item)
	id, _ := diskstore.NewReceiptID(boundNow, strings.NewReader(strings.Repeat("x", 64)))
	tombstone := diskstore.Tombstone{Schema: diskstore.TombstoneSchema, Item: "aside-chain", Kind: diskstore.KindChain, Segment: bed.segment.Git,
		Files: files, History: []diskstore.HistoryEntry{}, InventoryDigest: diskstore.InventoryDigest(files), Step: diskstore.StepRemove,
		Rule: diskstore.RulePerson, By: "Wido", At: boundNow, Receipt: id, State: diskstore.StateBegun, Plan: plan.ID,
		Disposing: "aside-chain.disposing-01STAGE"}
	data, _ := json.MarshalIndent(tombstone, "", "  ")
	os.WriteFile(diskstore.RemovedTombstonePath(item), data, 0o644)
	aside := filepath.Join(filepath.Dir(item), tombstone.Disposing)
	if err := os.Rename(item, aside); err != nil {
		t.Fatal(err)
	}
	t.Logf("open: %v", bed.segment.OpenPersonDisposals())
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	t.Logf("outcomes: %+v", outcomes)
	_, statAside := os.Stat(aside)
	_, statItem := os.Stat(item)
	tomb, _ := diskstore.ReadTombstone(diskstore.RemovedTombstonePath(item))
	t.Logf("aside present=%v item present=%v tombstone state=%s", statAside == nil, statItem == nil, tomb.State)
	if statAside == nil && tomb.State == diskstore.StateBegun {
		t.Fatalf("STUCK: the reported command (evidence dispose --plan %s) neither finished nor rolled back the removal: %s", plan.ID, outcomes[0].Line)
	}
}

// V7: local mode. A goal reopened in a PEER clone between two items (this
// installation's tip unmoved) is not seen by the second item's judgement.
func TestAPeerReopenBetweenItemsIsSeen(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	here := view(identityA, map[string]string{"g-x": GoalDone})
	here.Local = true
	peer := "/elsewhere/clone/metasystem"
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: here},
		accepted: map[string]LedgerView{peer: view(identityA, map[string]string{"g-x": GoalDone})}}
	exclusions := bed.exclusions(fake)
	exclusions.Peers = []Context{{Installation: peer, Facts: diskstore.CheckoutFacts{LedgerIdentity: identityA}}}
	exclusions.Tip = func(context.Context, string) (string, error) { return "9498700a9", nil }
	first := exclusions.Judge(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "a", Goal: "g-x"})
	t.Logf("first=%+v", first)
	fake.accepted[peer] = LedgerView{Tip: "peer-moved", Identity: identityA, States: map[string]string{"g-x": GoalOpen}}
	second := exclusions.Judge(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "b", Goal: "g-x"})
	t.Logf("second=%+v", second)
	if blocking(second) == "" {
		t.Fatalf("EXCLUSION FAILS OPEN: g-x reopened in the local-mode peer before item b's judgement; b still judged clear")
	}
}

// V3a': the case variant of the disposals directory: every receipt ledger.
func TestACaseVariantOfDisposalsIsNotAnItem(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	first := bed.chain(t, "first", 300, 50, "g-done")
	bed.env.Execute(context.Background(), bed.preview(t, "", first), ExecuteOptions{})
	before := receipts(t, bed.segment)
	upper := filepath.Join(bed.root, "Disposals")
	targets, err := bed.env.Resolve(context.Background(), upper)
	if err != nil {
		t.Logf("refused: %v", err)
		return
	}
	plan, _ := bed.env.Preview(context.Background(), targets, nil, "")
	t.Logf("plan: state=%s held=%v", plan.Items[0].State, plan.Items[0].Held)
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{Override: true})
	t.Logf("outcomes: %+v", outcomes)
	after, _ := diskstore.ReadReceipts(bed.segment.Ledger())
	t.Fatalf("RECORDS DELETED: %s resolved as an item; segment receipts before=%d after=%d", upper, len(before), len(after))
}
