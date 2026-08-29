package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func diskCheckout(t *testing.T) (*DiskCheckout, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents", "supervision", "lock.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	checkout := &DiskCheckout{
		Root:        root,
		Self:        identity.Ref{Pid: 41, StartedAtSec: 100},
		SelfTag:     "owner-tag",
		IntervalSec: 5,
		Fingerprint: "f",
		WatcherCap:  150,
	}
	return checkout, root
}

func writeOwner(t *testing.T, root string, pid int64, tag string) {
	t.Helper()
	record, _ := json.Marshal(ownerRecord{Pid: pid, PidStartedAt: 100, InstanceTag: tag})
	path := filepath.Join(root, "artifacts", "agents", "supervision", "lock.d", "owner.json")
	if err := os.WriteFile(path, record, 0o644); err != nil {
		t.Fatal(err)
	}
}

// D-1's decision table needs honest three-way inputs from disk.
func TestDiskThreeWayReads(t *testing.T) {
	checkout, root := diskCheckout(t)
	if checkout.RootState() != Present {
		t.Fatal("existing root must read present")
	}
	if checkout.StateFileState() != Absent {
		t.Fatal("missing state must read definitively absent")
	}
	if checkout.Currency() != NoLock {
		t.Fatal("missing owner file must read no-lock")
	}
	writeOwner(t, root, 41, "owner-tag")
	if checkout.Currency() != NamesSelf {
		t.Fatal("own identity must read names-self")
	}
	writeOwner(t, root, 99, "other-tag")
	if checkout.Currency() != NamesOther {
		t.Fatal("another identity must read names-other")
	}
	garbage := filepath.Join(root, "artifacts", "agents", "supervision", "lock.d", "owner.json")
	if err := os.WriteFile(garbage, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if checkout.Currency() != Unreadable {
		t.Fatal("garbage owner file is uninspectable, never a verdict")
	}
	checkout.Root = filepath.Join(root, "deleted")
	if checkout.RootState() != Absent {
		t.Fatal("missing root must read definitively absent")
	}
}

// Publication is fenced — a lock that stopped naming this
// owner aborts the atomic rename.
func TestDiskPublicationFence(t *testing.T) {
	checkout, root := diskCheckout(t)
	writeOwner(t, root, 41, "owner-tag")
	held := []Held{{Component: Watcher, Tag: "w1", Generation: 1,
		Identity: identity.Ref{Pid: 50, StartedAtSec: 200}}}
	if err := checkout.PublishState(held); err != nil {
		t.Fatalf("publication under own lock failed: %v", err)
	}
	namesSelf, err := checkout.StateNamesSelf()
	if err != nil || !namesSelf {
		t.Fatalf("published state must name self: %v %v", namesSelf, err)
	}
	content, _ := os.ReadFile(checkout.statePath())
	var document stateDocument
	if json.Unmarshal(content, &document) != nil {
		t.Fatal("state unparseable")
	}
	if document.Engine != "go" || document.Components["watcher"].Pid != 50 {
		t.Fatalf("state is not self-describing: %+v", document)
	}

	// The fence: takeover between build and rename aborts.
	writeOwner(t, root, 99, "successor")
	if err := checkout.PublishState(held); err == nil {
		t.Fatal("publication under a successor's lock must abort")
	}
}

func TestDiskGenerationReadsFailClosedAndExcludeRetiredComponents(t *testing.T) {
	checkout, root := diskCheckout(t)
	fixed := time.Date(2026, 8, 29, 16, 0, 0, 0, time.UTC)
	checkout.clock = func() time.Time { return fixed }
	if generation := checkout.PriorGeneration(); generation != 0 {
		t.Fatalf("a missing state seeded generation %d", generation)
	}
	if err := os.WriteFile(checkout.statePath(), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if generation := checkout.PriorGeneration(); generation != 0 {
		t.Fatalf("a malformed state seeded generation %d", generation)
	}
	writeOwner(t, root, 41, "owner-tag")
	held := []Held{
		{Component: Watcher, Tag: "retired-watcher", Generation: 4, Identity: identity.Ref{Pid: 50, StartedAtSec: 200}},
		{Component: Watcher, Tag: "current-watcher", Generation: 5, Identity: identity.Ref{Pid: 51, StartedAtSec: 201}},
	}
	if err := checkout.PublishState(held); err != nil {
		t.Fatal(err)
	}
	if generation := checkout.PriorGeneration(); generation != 5 {
		t.Fatalf("published generation = %d, want 5", generation)
	}
	data, err := os.ReadFile(checkout.statePath())
	if err != nil {
		t.Fatal(err)
	}
	var document stateDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if watcher := document.Components[string(Watcher)]; watcher.Pid != 51 || watcher.InstanceTag != "current-watcher" {
		t.Fatalf("retired generation leaked into publication: %+v", watcher)
	}
	if document.StartedAt != fixed.Format(isoSecond) {
		t.Fatalf("publication ignored its supplied clock: %q", document.StartedAt)
	}
}

func TestDiskStateOwnershipReportsMissingMalformedAndForeignState(t *testing.T) {
	checkout, _ := diskCheckout(t)
	if _, err := checkout.StateNamesSelf(); err == nil {
		t.Fatal("missing state was reported as owned")
	}
	if err := os.WriteFile(checkout.statePath(), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := checkout.StateNamesSelf(); err == nil {
		t.Fatal("malformed state was reported as owned")
	}
	if err := os.WriteFile(checkout.statePath(), []byte(`{"owner":{"pid":99,"pidStartedAt":100,"instanceTag":"other"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	owned, err := checkout.StateNamesSelf()
	if err != nil || owned {
		t.Fatalf("foreign state was reported as owned: owned=%v err=%v", owned, err)
	}
}

func TestDiskCurrencyTreatsUnreadableOwnerAsIndeterminate(t *testing.T) {
	checkout, _ := diskCheckout(t)
	if err := os.MkdirAll(checkout.ownerFile(), 0o755); err != nil {
		t.Fatal(err)
	}
	if checkout.Currency() != Unreadable {
		t.Fatal("an unreadable owner was treated as absent or authoritative")
	}
}

func TestDiskIntentUsesTheCurrentClockWhenNoTestClockIsSupplied(t *testing.T) {
	root := t.TempDir()
	lockDir := filepath.Join(root, "artifacts", "agents", "supervision", "lock.d")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	record, _ := json.Marshal(intentRecord{
		TargetPid: 41, TargetPidStartedAt: 100, TargetInstanceTag: "owner-tag",
		Requester: "test", WrittenAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err := os.WriteFile(filepath.Join(lockDir, "shutdown-intent.json"), record, 0o600); err != nil {
		t.Fatal(err)
	}
	intents := &DiskIntents{Root: root, Self: identity.Ref{Pid: 41, StartedAtSec: 100}, SelfTag: "owner-tag", LatchWindow: time.Minute}
	if !intents.LatchShutdown() {
		t.Fatal("a fresh intent was rejected when using the current clock")
	}
}

func TestDiskIntentLatch(t *testing.T) {
	root := t.TempDir()
	lockDir := filepath.Join(root, "artifacts", "agents", "supervision", "lock.d")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	intents := &DiskIntents{
		Root: root, Self: identity.Ref{Pid: 41, StartedAtSec: 100}, SelfTag: "owner-tag",
		LatchWindow: 20 * time.Second, clock: func() time.Time { return now },
	}
	write := func(pid int64, tag string, age time.Duration) {
		record, _ := json.Marshal(intentRecord{
			TargetPid: pid, TargetPidStartedAt: 100, TargetInstanceTag: tag,
			Requester: "test", WrittenAt: now.Add(-age).Format(time.RFC3339),
		})
		if err := os.WriteFile(filepath.Join(lockDir, "shutdown-intent.json"), record, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if intents.LatchShutdown() {
		t.Fatal("no intent must latch nothing")
	}
	write(41, "owner-tag", 5*time.Second)
	if !intents.LatchShutdown() {
		t.Fatal("a fresh intent naming this owner must latch")
	}
	if _, err := os.Stat(filepath.Join(lockDir, "shutdown-intent.json")); !os.IsNotExist(err) {
		t.Fatal("the intent must be consumed on latch")
	}
	write(41, "owner-tag", time.Minute)
	if intents.LatchShutdown() {
		t.Fatal("a stale intent is reported, never honored (SLC-R8-005)")
	}
	write(99, "other", time.Second)
	if intents.LatchShutdown() {
		t.Fatal("an intent naming another identity is ignored (and consumed)")
	}
	if _, err := os.Stat(filepath.Join(lockDir, "shutdown-intent.json")); !os.IsNotExist(err) {
		t.Fatal("foreign intents are consumed too")
	}
	if err := os.WriteFile(filepath.Join(lockDir, "shutdown-intent.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if intents.LatchShutdown() {
		t.Fatal("a malformed shutdown intent was honored")
	}
	record, _ := json.Marshal(intentRecord{
		TargetPid: 41, TargetPidStartedAt: 100, TargetInstanceTag: "owner-tag",
		Requester: "test", WrittenAt: "not-a-time",
	})
	if err := os.WriteFile(filepath.Join(lockDir, "shutdown-intent.json"), record, 0o600); err != nil {
		t.Fatal(err)
	}
	if intents.LatchShutdown() {
		t.Fatal("a shutdown intent without a valid timestamp was honored")
	}
}

// The trace narrates the full decision basis (the observability
// ruling): drive one purpose-gone cycle and read the story back.
func TestCycleTraceNarratesDecisions(t *testing.T) {
	world := newWorld()
	owner := newOwner(world)
	var traces []CycleTrace
	owner.Narrate = func(trace CycleTrace) { traces = append(traces, trace) }
	establish(t, owner, world)
	world.root = Absent
	if exit := owner.Cycle(time.Now()); exit == nil {
		t.Fatal("expected the purpose-gone exit")
	}
	last := traces[len(traces)-1]
	if last.Root != "absent" || last.Verdict != "purpose-gone" || last.Exit != "purpose-gone" {
		t.Fatalf("the trace does not narrate the decision basis: %+v", last)
	}
	first := traces[0]
	if len(first.Actions) == 0 || first.Actions[0] != "establish" {
		t.Fatalf("establishment was not narrated: %+v", first)
	}
}
