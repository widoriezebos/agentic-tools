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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

var boundNow = time.Date(2026, 12, 29, 2, 0, 0, 0, time.UTC)

const kib = 1 << 10

// boundBed is one nested installation (<git root>/metasystem) armed on the
// host, and its segment of an evidence root.
type boundBed struct {
	root, gitRoot, installation, home string
	segment                           Segment
}

func newBoundBed(t *testing.T) *boundBed {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bed := &boundBed{root: filepath.Join(base, "evidence"), gitRoot: filepath.Join(base, "checkout"), home: filepath.Join(base, "home", ".metasystem")}
	bed.installation = filepath.Join(bed.gitRoot, "metasystem")
	for _, dir := range []string{filepath.Join(bed.gitRoot, ".git"), filepath.Join(bed.installation, "artifacts", "agents", "jobs"), bed.root, bed.home} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A template-layout installation: its state root is itself, found
	// without running git.
	if err := os.WriteFile(filepath.Join(bed.installation, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	facts := diskstore.CheckoutFacts{GitRoot: bed.gitRoot, Installation: bed.installation, RootCommit: "e83c5163316f89bfbde7d9ab23ca2e25604af290", LedgerIdentity: "01J9LEDGER0000000000000000"}
	bed.segment = Segment{Root: bed.root, Git: diskstore.Segment(bed.gitRoot), Installation: diskstore.Segment(bed.installation),
		Context: &Context{Installation: bed.installation, Facts: facts}}
	return bed
}

// chain mirrors a closed chain that ended daysAgo, with payloadKiB of logs.
func (bed *boundBed) chain(t *testing.T, name string, daysAgo int, payloadKiB int, goal string) string {
	t.Helper()
	dir := filepath.Join(bed.root, "agents", bed.segment.Git, name)
	record := map[string]any{"jobId": name, "round": 1, "role": "implementer", "status": "completed", "chainClosed": true, "goalId": goal,
		"endedAt": boundNow.Add(-time.Duration(daysAgo) * 24 * time.Hour).Format(time.RFC3339)}
	data, _ := json.Marshal(record)
	returned, _ := json.Marshal(map[string]any{"claimed": map[string]any{"model": "claude-opus-5-5"}, "gaps": []any{"one"}, "whatWasDone": "built the thing\nand more"})
	files := map[string][]byte{
		"manifest.json":          []byte(`{"rootJob":"` + name + `","files":{}}`),
		"brief.md":               []byte("the brief"),
		"jobs/" + name + ".json": data,
		"jobs/" + name + ".log":  make([]byte, payloadKiB*kib),
		"rounds/1/return.json":   returned,
		"rounds/1/raw.out":       []byte(strings.Repeat("transcript ", 100)),
		"capabilities/cap.json":  []byte("{}"),
	}
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func (bed *boundBed) bound(judge Judge, locks Locks) Bound {
	if judge == nil {
		judge = func(context.Context, Segment, Item) Judgement { return Judgement{LedgerTip: "9498700a9"} }
	}
	return Bound{BoundLock: diskstore.BoundLockPath(bed.home), Now: boundNow, Entropy: rand.Reader, By: "steward m1e", Judge: judge, Locks: locks}
}

func settingsOf(capKiB int64) PassSettings {
	return PassSettings{CapBytes: capKiB * kib, AgeFloor: 90 * 24 * time.Hour, Values: map[string]string{"evidence.segment-cap-gib": "10"}}
}

func compacted(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, diskstore.CompactTombstoneName))
	return err == nil
}

func receipts(t *testing.T, segment Segment) []diskstore.DisposalReceipt {
	t.Helper()
	lines, err := diskstore.ReadReceipts(segment.Ledger())
	if err != nil {
		t.Fatal(err)
	}
	return lines
}

func TestBoundLosesNothingYoungerThanTheAgeFloor(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	var dirs []string
	for _, name := range []string{"a", "b", "c"} {
		dirs = append(dirs, bed.chain(t, name, 10, 200, "done-goal"))
	}
	position := bed.bound(nil, nil).CompactSegment(context.Background(), bed.segment, settingsOf(100))
	for _, dir := range dirs {
		if compacted(dir) {
			t.Fatalf("an item younger than the age floor is never touched: %s", dir)
		}
	}
	if !position.Over || position.YoungBytes == 0 || position.Removable != 0 {
		t.Fatalf("the segment stays over, held by the floor: %+v", position)
	}
}

func TestBoundCompactsOldestFirstAndStopsAtTheCap(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	oldest := bed.chain(t, "oldest", 300, 400, "g")
	middle := bed.chain(t, "middle", 200, 400, "g")
	newest := bed.chain(t, "newest", 100, 400, "g")
	bound := bed.bound(nil, nil)
	total, _, _ := bound.Measure(context.Background(), bed.segment)
	// A cap one chain's payload below the total: compacting the oldest is
	// enough.
	capKiB := (total - 300*kib) / kib
	position := bound.CompactSegment(context.Background(), bed.segment, settingsOf(capKiB))
	if !compacted(oldest) || compacted(middle) || compacted(newest) {
		t.Fatalf("the oldest alone must be compacted: %v %v %v %+v", compacted(oldest), compacted(middle), compacted(newest), position)
	}
	for _, kept := range []string{"manifest.json", "brief.md", "jobs/oldest.json", "rounds/1/return.json", diskstore.VerdictName} {
		if _, err := os.Stat(filepath.Join(oldest, filepath.FromSlash(kept))); err != nil {
			t.Fatalf("a compacted chain keeps %s: %v", kept, err)
		}
	}
	for _, dropped := range []string{"jobs/oldest.log", "rounds/1/raw.out", "capabilities/cap.json"} {
		if _, err := os.Stat(filepath.Join(oldest, filepath.FromSlash(dropped))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("a compacted chain drops %s: %v", dropped, err)
		}
	}
	verdict, _ := os.ReadFile(filepath.Join(oldest, diskstore.VerdictName))
	if !strings.HasPrefix(string(verdict), "1|oldest|implementer|completed|") || !strings.Contains(string(verdict), "|claude-opus-5-5|gaps=1|built the thing") {
		t.Fatalf("the verdict line: %q", verdict)
	}
	lines := receipts(t, bed.segment)
	if len(lines) != 1 || lines[0].Step != diskstore.StepCompact || lines[0].Rule != diskstore.RuleBound || lines[0].Item != "oldest" ||
		lines[0].LedgerTip != "9498700a9" || lines[0].ItemBytesAfter >= lines[0].ItemBytesBefore || lines[0].InventoryDigest == "" {
		t.Fatalf("one compaction receipt: %+v", lines)
	}
	if position.Over {
		t.Fatalf("the pass stopped at the cap: %+v", position)
	}
}

func TestBoundNeverRemovesAnItemAndNamesTheCommandPair(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	var dirs []string
	for index, name := range []string{"one", "two", "three"} {
		dirs = append(dirs, bed.chain(t, name, 100+index, 50, "g"))
		// Large briefs are kept, so compaction cannot bring the segment under.
		if err := os.WriteFile(filepath.Join(dirs[index], "brief.md"), make([]byte, 200*kib), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	position := bed.bound(nil, nil).CompactSegment(context.Background(), bed.segment, settingsOf(200))
	for _, dir := range dirs {
		if !compacted(dir) {
			t.Fatalf("every eligible item is compacted: %s", dir)
		}
		for _, kept := range []string{"brief.md", diskstore.VerdictName, diskstore.CompactTombstoneName} {
			if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
				t.Fatalf("no pass removes an item: %s lost %s", dir, kept)
			}
		}
	}
	if !position.Over || position.Removable != 3 || len(position.Commands) < 2 {
		t.Fatalf("the segment stays over and names what a person may remove: %+v", position)
	}
	if position.Commands[0] != "metasystem evidence dispose --over-bound --export DIR --preview" || position.Commands[1] != "metasystem evidence dispose --plan ID" {
		t.Fatalf("the literal command pair: %q", position.Commands)
	}
	bed.segment.Context.Settings.Values = map[string]string{"evidence.export-dir": "/Volumes/Backup/metasystem-exports"}
	again := bed.bound(nil, nil).CompactSegment(context.Background(), bed.segment, settingsOf(200))
	if again.Commands[0] != "metasystem evidence dispose --over-bound --export /Volumes/Backup/metasystem-exports --preview" {
		t.Fatalf("the directory comes from evidence.export-dir: %q", again.Commands)
	}
	if again.Compacted != 0 || len(receipts(t, bed.segment)) != 3 {
		t.Fatalf("a repeat compacts nothing and writes no second receipt: %+v", again)
	}
	var rendered []string
	rendered = append(rendered, again.Lines(false)...)
	if !strings.Contains(strings.Join(rendered, "\n"), "over the bound: "+bed.segment.Git+" of "+bed.gitRoot) {
		t.Fatalf("the report line: %v", rendered)
	}
}

// The total is a fresh stat walk under the lock before each item's cap
// test (DL4E-09): a person's removal between two of the pass's acquisitions
// is counted by the next walk, whichever side of the first walk it lands.
func TestBoundMeasuresAfreshBeforeEachItem(t *testing.T) {
	t.Parallel()
	for _, removeAt := range []int{1, 2} {
		bed := newBoundBed(t)
		first := bed.chain(t, "first", 300, 400, "g")
		second := bed.chain(t, "second", 200, 400, "g")
		big := filepath.Join(bed.root, "events", bed.segment.Installation, "events-20260101T000000Z.jsonl")
		if err := os.MkdirAll(filepath.Dir(big), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(big, make([]byte, 2000*kib), 0o644); err != nil {
			t.Fatal(err)
		}
		bound := bed.bound(nil, nil)
		total, _, _ := bound.Measure(context.Background(), bed.segment)
		calls := 0
		bound.Locks = func(string, []string, time.Duration) (func(), string, error) {
			calls++
			if calls == removeAt {
				// A receipt-less removal of 2000 KiB, as by hand.
				if err := os.Remove(big); err != nil {
					t.Fatal(err)
				}
			}
			return func() {}, "", nil
		}
		capKiB := (total - 1000*kib) / kib
		bound.CompactSegment(context.Background(), bed.segment, settingsOf(capKiB))
		switch removeAt {
		case 1:
			if compacted(first) || compacted(second) {
				t.Fatalf("a removal before the first walk leaves the segment under the cap: nothing compacts")
			}
		case 2:
			if !compacted(first) || compacted(second) {
				t.Fatalf("a removal after the first item is counted by the second's walk: %v %v", compacted(first), compacted(second))
			}
		}
	}
}

func TestBoundWalkThatDoesNotFinishCompactsNothing(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "old", 300, 400, "g")
	bound := bed.bound(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	bound.Locks = func(string, []string, time.Duration) (func(), string, error) {
		cancel() // the budget ends between the lock and the walk
		return func() {}, "", nil
	}
	position := bound.CompactSegment(ctx, bed.segment, settingsOf(1))
	if compacted(dir) {
		t.Fatal("a walk cut short compacts nothing")
	}
	if !strings.Contains(strings.Join(position.Pending, "\n"), "disk.sweep-budget-sec") {
		t.Fatalf("the report names the setting to raise: %+v", position.Pending)
	}
}

func TestBoundSkipsAChainWhoseLifecycleLockIsHeld(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "busy", 300, 400, "g")
	var asked []string
	bound := bed.bound(nil, func(installation string, jobs []string, wait time.Duration) (func(), string, error) {
		asked = append(asked, jobs...)
		if wait != 0 {
			t.Fatalf("the pass takes the lifecycle locks without waiting, got %s", wait)
		}
		return nil, "pid=4242,tag=reap", nil
	})
	position := bound.CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(dir) || len(asked) != 1 || asked[0] != "busy" {
		t.Fatalf("a held lifecycle lock makes the chain pending: compacted=%v asked=%v", compacted(dir), asked)
	}
	if !strings.Contains(strings.Join(position.Pending, "\n"), "pid=4242,tag=reap") {
		t.Fatalf("the report names the holder: %+v", position.Pending)
	}
	bound.Locks = func(string, []string, time.Duration) (func(), string, error) { return func() {}, "", nil }
	bound.CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if !compacted(dir) {
		t.Fatal("released, the next pass compacts it")
	}
}

func TestBoundHeldBoundLockLeavesTheSegmentPending(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "old", 300, 400, "g")
	lock, err := diskstore.BoundExclusive(diskstore.BoundLockPath(bed.home))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	position := bed.bound(nil, nil).CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(dir) || len(position.Pending) == 0 {
		t.Fatalf("a held bound lock is pending, never a wait: %+v", position)
	}
}

// A tombstone that is published but whose directory sync failed is not
// durable (DL4E-05): nothing proceeds to the receipt, and the next pass
// repeats the step with exactly one receipt.
func TestBoundDoesNotCommitOnAnUnconfirmedTombstone(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "old", 300, 400, "g")
	bound := bed.bound(nil, nil)
	bound.Sync = diskstore.Syncer{Dir: func(path string) error {
		if path == dir {
			return errors.New("injected: the directory entry was dropped")
		}
		return nil
	}}
	position := bound.CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if len(receipts(t, bed.segment)) != 0 || compacted(dir) {
		t.Fatalf("no receipt and no tombstone after a failed sync: %+v", position)
	}
	if _, err := os.Stat(filepath.Join(dir, "jobs", "old.log")); err != nil {
		t.Fatalf("nothing is dropped: %v", err)
	}
	bound.Sync = diskstore.Syncer{}
	bound.CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if lines := receipts(t, bed.segment); len(lines) != 1 || !compacted(dir) {
		t.Fatalf("the next pass repeats the step with one receipt: %+v", lines)
	}
}

func TestBoundRollsBackWhenTheCommitCheckFails(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	dir := bed.chain(t, "old", 300, 400, "g")
	before := snapshot(t, dir)
	bound := bed.bound(func(context.Context, Segment, Item) Judgement {
		return Judgement{Commit: func() error { return errors.New("receipt ledger changed during the judgement") }}
	}, nil)
	position := bound.CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if len(receipts(t, bed.segment)) != 0 || !strings.Contains(strings.Join(position.Pending, "\n"), "receipt ledger changed during the judgement") {
		t.Fatalf("a failed commit check rolls back and says why: %+v", position)
	}
	if after := snapshot(t, dir); !equalSnapshots(before, after) {
		t.Fatalf("the rolled-back chain is whole")
	}
}

func TestBoundJudgesTheSettledCheckUnderTheInstallation(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	withPayload := bed.chain(t, "payload", 300, 400, "g")
	recordsOnly := bed.chain(t, "records", 300, 400, "g")
	if err := os.MkdirAll(filepath.Join(bed.installation, "artifacts", "agents", "payload", "rounds"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.installation, "artifacts", "agents", "jobs", "records.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var locked [][]string
	bound := bed.bound(nil, func(installation string, jobs []string, _ time.Duration) (func(), string, error) {
		if installation != bed.installation {
			t.Fatalf("the lifecycle locks live in the installation, got %s", installation)
		}
		locked = append(locked, jobs)
		return func() {}, "", nil
	})
	bound.CompactSegment(context.Background(), bed.segment, settingsOf(1))
	if compacted(withPayload) || !compacted(recordsOnly) {
		t.Fatalf("a chain with local payload is never a candidate; one with local records only is: %v %v", compacted(withPayload), compacted(recordsOnly))
	}
	missing := bed.segment
	missing.Context = nil
	if ok, reason := bound.Candidate(missing, chainItem(withPayload), 0); ok || !strings.Contains(reason, "not armed") {
		t.Fatalf("a missing installation is Unknown, never settled: %v %s", ok, reason)
	}
}

func TestSegmentIndexRevalidates(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	facts := bed.segment.Context.Facts
	index, err := RecordSegmentIndex(bed.root, facts, boundNow, diskstore.Syncer{})
	if err != nil || index.GitSegment != bed.segment.Git || index.InstallationSegment != bed.segment.Installation {
		t.Fatalf("index=%+v err=%v", index, err)
	}
	if reason := index.Revalidate(facts); reason != "" {
		t.Fatalf("the same checkout revalidates: %s", reason)
	}
	reused := facts
	reused.RootCommit = "0000000000000000000000000000000000000000"
	if reason := index.Revalidate(reused); !strings.Contains(reason, "checkout path reused") {
		t.Fatalf("a reused path: %q", reason)
	}
	readopted := facts
	readopted.LedgerIdentity = "01KOTHER000000000000000000"
	if reason := index.Revalidate(readopted); !strings.Contains(reason, "ledger identity changed") {
		t.Fatalf("a re-adopted ledger: %q", reason)
	}
	again, err := RecordSegmentIndex(bed.root, readopted, boundNow.Add(time.Hour), diskstore.Syncer{})
	if err != nil || again.LedgerIdentity != facts.LedgerIdentity || !again.FirstSeen.Equal(index.FirstSeen) {
		t.Fatalf("an existing index is kept as it is: %+v %v", again, err)
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			data, _ := os.ReadFile(path)
			files[strings.TrimPrefix(path, root)] = string(data)
		}
		return nil
	})
	return files
}

func equalSnapshots(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}
