package evidence

// Round B2-3 (scope cut) witnesses, ported from the third read's probes
// (b2-read3-probes, each failing on 3dd7f7b9f).

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// interruptedRemoval is a person's removal as a crash leaves it: tombstone
// begun, item set aside, no receipt.
func interruptedRemoval(t *testing.T, bed personBed, item, name, plan string) (diskstore.Tombstone, string) {
	t.Helper()
	files, err := diskstore.Inventory(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := diskstore.NewReceiptID(boundNow, strings.NewReader(strings.Repeat("y", 64)))
	tombstone := diskstore.Tombstone{Schema: diskstore.TombstoneSchema, Item: name, Kind: diskstore.KindChain, Segment: bed.segment.Git,
		Files: files, History: []diskstore.HistoryEntry{}, InventoryDigest: diskstore.InventoryDigest(files), Step: diskstore.StepRemove,
		Rule: diskstore.RulePerson, By: "Wido", At: boundNow, Receipt: id, State: diskstore.StateBegun, Plan: plan,
		Disposing: name + ".disposing-01STAGE"}
	data, _ := json.MarshalIndent(tombstone, "", "  ")
	if err := os.WriteFile(diskstore.RemovedTombstonePath(item), data, 0o644); err != nil {
		t.Fatal(err)
	}
	aside := filepath.Join(filepath.Dir(item), tombstone.Disposing)
	if err := os.Rename(item, aside); err != nil {
		t.Fatal(err)
	}
	return tombstone, aside
}

// Rule 3: an evidence root nested inside another root (a peer checkout's
// evidence.root is this root's parent) is never listed there: not an
// item, not a not-managed entry.
func TestANestedEvidenceRootIsNeverListedInItsParent(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	done := bed.chain(t, "done-chain", 300, 50, "g-done")
	parent := filepath.Dir(bed.root)
	other := HostCheckout{Installation: filepath.Join(parent, "other-checkout", "metasystem"),
		Facts:    diskstore.CheckoutFacts{GitRoot: filepath.Join(parent, "other-checkout"), Installation: filepath.Join(parent, "other-checkout", "metasystem")},
		Settings: diskstore.Settings{Values: map[string]string{}, EvidenceRoot: config.EvidenceRoot{Path: parent, Origin: "conf-local"}}}
	bed.env.Checkouts = append(bed.env.Checkouts, other)
	if _, err := bed.env.Resolve(context.Background(), bed.root); err == nil || !strings.Contains(err.Error(), NotAnItem) {
		t.Fatalf("a nested root is not an item: %v", err)
	}
	for _, entry := range bed.env.NotManaged(context.Background()) {
		if entry.Path == bed.root {
			t.Fatalf("a nested root is never listed as not managed: %+v", entry)
		}
	}
	if gone(done) {
		t.Fatal("nothing was removed")
	}
}

// Rule 3: a segment directory whose name is not the engine's lower-case
// hex exactly is not a segment; the upper-case spelling of this
// checkout's own segment on a case-insensitive volume is that segment and
// is never listed (a person would remove the live segment by hand).
func TestAnUpperCaseSpellingOfTheLiveSegmentIsNeverListed(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	a := bed.chain(t, "a", 300, 50, "g-done")
	lower := filepath.Join(bed.root, "agents", bed.segment.Git)
	upper := filepath.Join(bed.root, "agents", strings.ToUpper(bed.segment.Git))
	if err := os.Rename(lower, upper); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lower); err != nil {
		// A case-sensitive volume: the upper-case directory is simply not
		// a segment, listed as not managed.
		found := false
		for _, entry := range bed.env.NotManaged(context.Background()) {
			found = found || entry.Path == upper
		}
		if !found {
			t.Fatalf("on a case-sensitive volume %s is not managed", upper)
		}
		return
	}
	for _, entry := range bed.env.NotManaged(context.Background()) {
		if strings.EqualFold(entry.Path, lower) {
			t.Fatalf("the live segment is listed as not managed: %s", entry.Path)
		}
	}
	if _, err := bed.env.Resolve(context.Background(), upper); err == nil {
		t.Fatal("the segment directory is not an item")
	}
	targets, _ := bed.env.Enumerate(context.Background())
	if len(targets) != 1 || targets[0].Item.Name != "a" {
		t.Fatalf("its items still enumerate: %+v", targets)
	}
	if gone(filepath.Join(upper, "a")) {
		t.Fatalf("nothing was removed: %s", a)
	}
}

// Rule 2: a committed removal cut short, with a new directory at the item
// path since, removes only the set-aside copy the tombstone records.
func TestSettlingACommittedRemovalKeepsWhatNowStandsAtThePath(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	item := bed.chain(t, "reborn", 300, 50, "g-done")
	plan := bed.preview(t, "", item)
	tombstone, aside := interruptedRemoval(t, bed, item, "reborn", plan.ID)
	if err := diskstore.AppendReceipt(bed.segment.Ledger(), diskstore.DisposalReceipt{Schema: diskstore.ReceiptSchema, ID: tombstone.Receipt, Item: "reborn",
		Kind: diskstore.KindChain, Step: diskstore.StepRemove, Rule: diskstore.RulePerson}, diskstore.Syncer{}); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(item, "jobs", "reborn-2.log")
	if err := os.MkdirAll(filepath.Dir(fresh), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fresh, []byte("new evidence written after the crash"), 0o644); err != nil {
		t.Fatal(err)
	}
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	if gone(fresh) || !gone(aside) || !strings.Contains(outcomes[0].Line, "set-aside copy is now removed") {
		t.Fatalf("only the set-aside copy goes: fresh gone=%v aside gone=%v %+v", gone(fresh), gone(aside), outcomes)
	}
}

// Rule 2, N3-3: a rollback with both the set-aside copy and an entry at
// the item path keeps the tombstone and reports; the copy stays listed.
func TestARollbackWithBothCopiesKeepsTheTombstoneAndReports(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	item := bed.chain(t, "twice", 300, 50, "g-done")
	plan := bed.preview(t, "", item)
	_, aside := interruptedRemoval(t, bed, item, "twice", plan.ID)
	if err := os.MkdirAll(filepath.Join(item, "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(item, "jobs", "placeholder.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	if outcomes[0].Done || !strings.Contains(outcomes[0].Line, "both") {
		t.Fatalf("both present is reported: %+v", outcomes)
	}
	if gone(aside) || gone(diskstore.RemovedTombstonePath(item)) || len(bed.segment.OpenPersonDisposals()) != 1 {
		t.Fatalf("the copy and its tombstone stay, listed as open: aside gone=%v open=%v", gone(aside), bed.segment.OpenPersonDisposals())
	}
}

// Rule 2: a removal cut short is never continued, whatever the person
// passes: under --override the item is rolled back, no receipt is
// written, and the person previews again.
func TestARemovalCutShortIsNeverContinuedUnderOverride(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	item := bed.chain(t, "held-later", 300, 50, "g-open")
	plan := bed.preview(t, "", item)
	_, aside := interruptedRemoval(t, bed, item, "held-later", plan.ID)
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{Override: true, Reason: "space"})
	if outcomes[0].Done || gone(item) || !gone(aside) || len(receipts(t, bed.segment)) != 0 {
		t.Fatalf("rolled back, nothing removed, no receipt: %+v", outcomes)
	}
}

// Rule 2: a set-aside chain whose goal reopened after the crash is rolled
// back, never finished.
func TestASetAsideChainOfAReopenedGoalIsRolledBack(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	item := bed.chain(t, "reopened", 300, 50, "g-z")
	bed.fake.accepted[bed.installation].States["g-z"] = GoalDone
	bed.fake.fetched[bed.installation].States["g-z"] = GoalDone
	plan := bed.preview(t, "", item)
	_, aside := interruptedRemoval(t, bed, item, "reopened", plan.ID)
	bed.fake.accepted[bed.installation].States["g-z"] = GoalOpen
	bed.fake.fetched[bed.installation].States["g-z"] = GoalOpen
	bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	if gone(item) || !gone(aside) {
		t.Fatalf("the chain is back: item gone=%v aside gone=%v", gone(item), gone(aside))
	}
}

// Rule 4: a local-mode peer whose identity read failed (empty, no error)
// makes the whole union Unknown.
func TestAPeerWithAnEmptyIdentityMakesTheUnionUnknown(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	here := view(identityA, map[string]string{"g-x": GoalDone})
	here.Local = true
	peer := "/elsewhere/clone/metasystem"
	fake := &ledgers{fetched: map[string]LedgerView{bed.installation: here},
		accepted: map[string]LedgerView{bed.installation: here, peer: view(identityA, map[string]string{"g-x": GoalOpen})}}
	exclusions := bed.exclusions(fake)
	exclusions.Peers = []Context{{Installation: peer, Facts: diskstore.CheckoutFacts{GitRoot: "/elsewhere/clone", Installation: peer}}}
	judgement := exclusions.Judge(context.Background(), bed.segment, Item{Kind: diskstore.KindChain, Name: "a", Goal: "g-x"})
	if !strings.Contains(judgement.SegmentUnknown, "ledger identity of "+peer+" cannot be read") {
		t.Fatalf("the union is Unknown: %+v", judgement)
	}
}

// N3-7: the default plan is this session's newest preview only while it
// is younger than a day.
func TestTheDefaultPlanIsYoungerThanADay(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	item := bed.chain(t, "old-plan", 300, 50, "g-done")
	for _, check := range []struct {
		age  time.Duration
		want bool
	}{{40 * 24 * time.Hour, false}, {25 * time.Hour, false}, {time.Hour, true}} {
		env := bed.env
		env.Session = "4242-" + check.age.String()
		env.Now = boundNow.Add(-check.age)
		target, err := env.Locate(context.Background(), item)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := env.Preview(context.Background(), []Target{target}, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := NewestDisposePlan(bed.home, env.Session, boundNow) == plan.ID; got != check.want {
			t.Fatalf("a plan %s old is the default = %v", check.age, got)
		}
	}
}
