package evidence

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

type personBed struct {
	*boundBed
	env    Env
	fake   *ledgers
	export string
}

func newPersonBed(t *testing.T) personBed {
	t.Helper()
	bed := newBoundBed(t)
	userHome := filepath.Join(filepath.Dir(bed.root), "user")
	fake := &ledgers{
		accepted: map[string]LedgerView{bed.installation: view(identityA, map[string]string{"g-open": GoalOpen, "g-done": GoalDone})},
		fetched:  map[string]LedgerView{bed.installation: view(identityA, map[string]string{"g-open": GoalOpen, "g-done": GoalDone})},
	}
	this := HostCheckout{Installation: bed.installation, Facts: bed.segment.Context.Facts,
		Settings: diskstore.Settings{Values: map[string]string{}, EvidenceRoot: config.EvidenceRoot{Path: bed.root, Origin: "conf-local"}}}
	citations := &Citations{Dir: filepath.Join(bed.home, "stores", "citations"), Now: boundNow, Roots: func() ([]string, error) { return nil, nil }}
	if _, err := citations.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordSegmentIndex(bed.root, bed.segment.Context.Facts, boundNow, diskstore.Syncer{}); err != nil {
		t.Fatal(err)
	}
	env := Env{UserHome: userHome, HomeStateRoot: bed.home, Checkouts: []HostCheckout{this}, This: this, Now: boundNow, Entropy: rand.Reader,
		Observe: fake.observe, Citations: citations, By: "Wido", Blobs: diskstore.BlobStore{Dir: diskstore.BlobStoreDir(userHome)},
		Locks:           func(string, []string, time.Duration) (func(), string, error) { return func() {}, "", nil },
		SegmentSettings: func(Segment) (PassSettings, error) { return settingsOf(1 << 20), nil }}
	return personBed{boundBed: bed, env: env, fake: fake, export: filepath.Join(filepath.Dir(bed.root), "backup", "exports")}
}

func (bed personBed) preview(t *testing.T, exportDir string, paths ...string) DisposePlan {
	t.Helper()
	var targets []Target
	for _, path := range paths {
		target, err := bed.env.Locate(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		targets = append(targets, target)
	}
	plan, err := bed.env.Preview(context.Background(), targets, nil, exportDir)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func gone(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, os.ErrNotExist)
}

// Removal is a person's act from a previewed plan; the preview fetches
// nothing and writes only its plan (R15, DL4E-13), and the execution
// observes afresh and removes a clear item with rule person, whatever its
// age.
func TestAPersonRemovesFromAPreviewedPlan(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	young := bed.chain(t, "ten-days-old", 10, 50, "g-done")
	before := snapshot(t, filepath.Dir(bed.root))
	plan := bed.preview(t, "", young)
	after := snapshot(t, filepath.Dir(bed.root))
	delete(after, strings.TrimPrefix(PlanPath(bed.home, plan.ID), filepath.Dir(bed.root)))
	if !equalSnapshots(before, after) || bed.fake.fetches != 0 {
		t.Fatalf("a preview changes nothing but its plan and fetches nothing: fetches=%d", bed.fake.fetches)
	}
	if len(plan.Items) != 1 || plan.Items[0].State != "clear" || plan.Items[0].Files == 0 || !strings.Contains(plan.Ledger, "judged on the accepted ledger") {
		t.Fatalf("the plan: %+v", plan)
	}
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{Reason: "space"})
	if len(outcomes) != 1 || !outcomes[0].Done || !gone(young) || bed.fake.fetches == 0 {
		t.Fatalf("the execution observes and removes: %+v", outcomes)
	}
	lines := receipts(t, bed.segment)
	if len(lines) != 1 || lines[0].Rule != diskstore.RulePerson || lines[0].Step != diskstore.StepRemove || lines[0].Plan != plan.ID || lines[0].By != "Wido" {
		t.Fatalf("one person's receipt: %+v", lines)
	}
	if again := bed.env.Execute(context.Background(), plan, ExecuteOptions{}); !again[0].Already || len(receipts(t, bed.segment)) != 1 {
		t.Fatalf("a repeat writes nothing: %+v", again)
	}
}

func TestAHeldItemIsSkippedUnlessOverriddenAndTheOverrideIsRecorded(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	held := bed.chain(t, "open-goal", 300, 50, "g-open")
	clear := bed.chain(t, "done-goal", 300, 50, "g-done")
	plan := bed.preview(t, "", held, clear)
	if plan.Items[0].State != "held" || plan.Items[1].State != "clear" {
		t.Fatalf("the preview says held or clear: %+v", plan.Items)
	}
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	if outcomes[0].Done || !strings.Contains(outcomes[0].Line, "held: goal g-open open; --override takes it anyway") || !outcomes[1].Done {
		t.Fatalf("the held item is skipped with the override line; the rest completes: %+v", outcomes)
	}
	outcomes = bed.env.Execute(context.Background(), plan, ExecuteOptions{Override: true})
	if !outcomes[0].Done || !gone(held) {
		t.Fatalf("--override takes it: %+v", outcomes)
	}
	tombstone, err := diskstore.ReadTombstone(diskstore.RemovedTombstonePath(held))
	if err != nil || len(tombstone.Overrides) != 1 || tombstone.Overrides[0] != "goal g-open open" {
		t.Fatalf("the tombstone names the override: %+v %v", tombstone.Overrides, err)
	}
}

func TestAFailedObservationIsAnExclusionAPersonMayOverride(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	dir := bed.chain(t, "offline", 300, 50, "g-done")
	bed.fake.err = errors.New("offline")
	plan := bed.preview(t, "", dir)
	if plan.Items[0].State != "held" || !strings.Contains(strings.Join(plan.Items[0].Held, " "), "ledger not observed") {
		t.Fatalf("a failed observation holds the item in the preview: %+v", plan.Items)
	}
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{Override: true})
	if !outcomes[0].Done {
		t.Fatalf("a person may take it anyway: %+v", outcomes)
	}
	lines := receipts(t, bed.segment)
	if len(lines) != 1 || !strings.Contains(strings.Join(lines[0].Overrides, " "), "ledger-not-observed") {
		t.Fatalf("the override is on the receipt: %+v", lines)
	}
}

func TestDisposeExportsFirstAndThePointerNamesTheArchive(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	dir := bed.chain(t, "exported", 300, 50, "g-done")
	plan := bed.preview(t, bed.export, dir)
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	if !outcomes[0].Done || outcomes[0].Export == "" || !gone(dir) {
		t.Fatalf("exported, then removed: %+v", outcomes)
	}
	lines := receipts(t, bed.segment)
	if lines[0].Export == nil || lines[0].Export.Archive != outcomes[0].Export {
		t.Fatalf("the receipt names the archive: %+v", lines[0].Export)
	}
	answer := Pointer(context.Background(), filepath.Join(dir, "jobs", "exported.log"))
	if answer.State != "removed" || !strings.Contains(answer.Line, "exported to "+outcomes[0].Export) {
		t.Fatalf("the pointer names the copy: %+v", answer)
	}
}

func TestAnItemWhoseExportFailsIsKeptAndTheRestCompletes(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	first := bed.chain(t, "first", 300, 50, "g-done")
	second := bed.chain(t, "second", 300, 50, "g-done")
	// The destination is a file: every export into it fails.
	if err := os.MkdirAll(filepath.Dir(bed.export), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bed.export, []byte("full"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := bed.preview(t, bed.export, first)
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	if outcomes[0].Done || gone(first) || !strings.Contains(outcomes[0].Line, "kept: not exported") {
		t.Fatalf("a failed export keeps the item: %+v", outcomes)
	}
	plan = bed.preview(t, "", second)
	if outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{}); !outcomes[0].Done {
		t.Fatalf("the rest completes: %+v", outcomes)
	}
}

func TestAnItemChangedSinceThePreviewIsSkipped(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	dir := bed.chain(t, "changing", 300, 50, "g-done")
	plan := bed.preview(t, "", dir)
	if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte("a re-landed record"), 0o644); err != nil {
		t.Fatal(err)
	}
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	if outcomes[0].Done || gone(dir) || !strings.Contains(outcomes[0].Line, "changed since the preview; run --preview again") {
		t.Fatalf("%+v", outcomes)
	}
}

// Every removal of an item leaves an inventory, a tombstone and a receipt
// in its segment's ledger; an entry outside every segment is not managed
// and refused (Round B2-3, rule 3).
func TestEveryRemovalLeavesATombstoneAndAnUnsegmentedEntryIsRefused(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	events := filepath.Join(bed.root, "events", bed.segment.Installation, "events-20260101T000000Z.jsonl")
	cache := filepath.Join(bed.root, "gocache-x")
	for path, data := range map[string]string{events: "{}\n", filepath.Join(cache, "unique.bin"): "the one unique file"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := bed.env.Locate(context.Background(), cache); err == nil || !strings.Contains(err.Error(), NotManagedLine) {
		t.Fatalf("an unsegmented entry is not managed and refused: %v", err)
	}
	plan := bed.preview(t, "", events)
	if outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{}); !outcomes[0].Done || !gone(events) || gone(cache) {
		t.Fatalf("the events archive is removed, the entry outside every segment untouched: %+v", outcomes)
	}
	tombstone, err := diskstore.ReadTombstone(diskstore.RemovedTombstonePath(events))
	if err != nil || len(tombstone.Files) != 1 || tombstone.Files[0].Path != filepath.Base(events) {
		t.Fatalf("the inventory names the archive: %+v %v", tombstone, err)
	}
	if receipts, _ := diskstore.ReadReceipts(filepath.Join(bed.root, "disposals", bed.segment.Installation+".jsonl")); len(receipts) != 1 {
		t.Fatalf("a receipt in the events segment's ledger: %+v", receipts)
	}
}

// A chain that is not closed is declined naming the one action that
// changes its state, and a registered, unreleased workspace store of the
// chain declines it with that store's remedy (DL4E-11).
func TestAnUnsettledChainIsDeclinedWithTheActionThatSettlesIt(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	dir := bed.chain(t, "unsettled", 300, 50, "g-done")
	jobs := filepath.Join(bed.installation, "artifacts", "agents", "jobs")
	payload := filepath.Join(bed.installation, "artifacts", "agents", "unsettled")
	if err := os.MkdirAll(payload, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ record, want string }{
		{`{"jobId":"unsettled","status":"running"}`, "metasystem work stop j2:unsettled"},
		{`{"jobId":"unsettled","status":"completed"}`, "metasystem work review j2:unsettled"},
		{`{"jobId":"unsettled","status":"completed","chainClosed":true}`, "workspace store"},
	}
	registry := diskstore.CheckoutRegistry(bed.installation)
	for index, test := range cases {
		if err := os.WriteFile(filepath.Join(jobs, "unsettled.json"), []byte(test.record), 0o644); err != nil {
			t.Fatal(err)
		}
		if index == 2 {
			store := filepath.Join(bed.installation, "artifacts", "agents", "worktrees", "unsettled")
			if err := os.MkdirAll(store, 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := registry.Register(diskstore.Registration{Path: store, Class: "delegate-workspace", Owner: diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: "unsettled"},
				Lifetime: diskstore.LifetimeOwner, CapKind: diskstore.CapTarget}, boundNow, rand.Reader); err != nil {
				t.Fatal(err)
			}
		}
		plan := bed.preview(t, "", dir)
		outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
		if outcomes[0].Done || gone(dir) || !strings.Contains(outcomes[0].Line, test.want) {
			t.Fatalf("case %d: %+v", index, outcomes)
		}
	}
}

// A mirror in its lifecycle lock makes dispose wait the reaper's bound and
// then decline naming the holder (DL4E-02).
func TestDisposeDeclinesWhileAMirrorHoldsTheLifecycleLock(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	dir := bed.chain(t, "mirroring", 300, 50, "g-done")
	var waited time.Duration
	bed.env.Locks = func(_ string, _ []string, wait time.Duration) (func(), string, error) {
		waited = wait
		return nil, "job mirroring's lifecycle lock is held by pid=4242,tag=reap", nil
	}
	plan := bed.preview(t, "", dir)
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	if outcomes[0].Done || gone(dir) || waited != ReaperBound || !strings.Contains(outcomes[0].Line, "pid=4242,tag=reap") {
		t.Fatalf("waits the reaper's bound, then declines naming the holder: waited %s %+v", waited, outcomes)
	}
}

// A removal with a verified export, cut short before its receipt, is
// rolled back and never resumed (Round B2-3, rule 2): the item is back,
// the tombstone gone, no receipt written, and the person previews again.
func TestARemovalCutShortIsRolledBackNeverResumed(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	dir := bed.chain(t, "crashed", 300, 50, "g-done")
	exported, err := diskstore.Export(context.Background(), diskstore.ExportRequest{Item: dir, Dir: bed.export, Segment: bed.segment.Git, Kind: diskstore.KindChain,
		Blobs: bed.env.Blobs, Now: boundNow, Stage: "01E"})
	if err != nil {
		t.Fatal(err)
	}
	plan := bed.preview(t, "", dir)
	tombstone, aside := interruptedRemoval(t, bed, dir, "crashed", plan.ID)
	tombstone.Export = exported.Ref(bed.export)
	writeJSON(t, diskstore.RemovedTombstonePath(dir), tombstone)
	outcomes := bed.env.Execute(context.Background(), plan, ExecuteOptions{Override: true})
	if outcomes[0].Done || gone(dir) || !gone(aside) || !strings.Contains(outcomes[0].Line, "rolled back, the item is back; run --preview again") {
		t.Fatalf("%+v", outcomes)
	}
	if !gone(diskstore.RemovedTombstonePath(dir)) || len(receipts(t, bed.segment)) != 0 {
		t.Fatal("the rollback removes the begun tombstone and writes no receipt")
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOverBoundSelectsItemsPastTheAgeFloorOldestFirst(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	bed.chain(t, "oldest", 400, 50, "g-done")
	bed.chain(t, "held", 350, 50, "g-open")
	bed.chain(t, "young", 10, 400, "g-done")
	events := filepath.Join(bed.root, "events", bed.segment.Installation, "events-20260101T000000Z.jsonl")
	if err := os.MkdirAll(filepath.Dir(events), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(events, make([]byte, 100*kib), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.env.SegmentSettings = func(Segment) (PassSettings, error) { return settingsOf(1), nil }
	exclusions := &Exclusions{Observe: bed.fake.observe, Fetch: false, Citations: bed.env.Citations}
	targets, stillOver := bed.env.OverBound(context.Background(), func(segment Segment, item Item) Judgement {
		return exclusions.Judge(context.Background(), segment, item)
	})
	var names []string
	for _, target := range targets {
		names = append(names, target.Item.Name)
	}
	if strings.Join(names, ",") != "oldest,events-20260101T000000Z.jsonl,held" {
		t.Fatalf("items past the age floor, oldest first, the held one listed: %v", names)
	}
	if len(stillOver) != 1 || !strings.Contains(stillOver[0], "younger than the age floor") {
		t.Fatalf("the still-over line says what the plan cannot select: %v", stillOver)
	}
}

func TestThePointerAnswersEveryPathThatEverLayInAnItem(t *testing.T) {
	t.Parallel()
	bed := newPersonBed(t)
	dir := bed.chain(t, "pointed", 300, 50, "g-done")
	live := Pointer(context.Background(), filepath.Join(dir, "brief.md"))
	if live.State != "live" {
		t.Fatalf("%+v", live)
	}
	plan := bed.preview(t, "", dir)
	bed.env.Execute(context.Background(), plan, ExecuteOptions{})
	removed := Pointer(context.Background(), filepath.Join(dir, "rounds", "1", "raw.out"))
	if removed.State != "removed" || !strings.Contains(removed.Line, "removed on 2026-12-29 under rule person") {
		t.Fatalf("a file of a removed item answers through its tombstone: %+v", removed)
	}
	if unknown := Pointer(context.Background(), filepath.Join(bed.root, "nothing-here")); unknown.State != "absent" || !strings.HasSuffix(unknown.Line, "no such evidence") {
		t.Fatalf("%+v", unknown)
	}
}
