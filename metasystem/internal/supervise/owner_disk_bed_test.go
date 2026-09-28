package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// Ports of supervision-go-fixtures.sh (verbs-object-action U7c): the owner
// loop over the production disk adapters, not a fake world. DiskCheckout
// (root, lock currency, fenced state publication), DiskIntents and
// RegistryLedger run against a scratch checkout and a scratch registry; only
// the component processes and the clock are stood in for. The shell bed
// drove the same adapters through the built binary with one-second
// intervals; here every cycle is driven, so nothing waits on time.

// diskBedComponents stands in for the watcher, reaper and landing owner:
// launches mint identities, stops are proven, and the observation is the
// bed's to choose (a crash-on-start component never beats: Failing).
type diskBedComponents struct {
	nextPid     int64
	observation Observation
	launched    []Held
	stopped     map[int64]bool
}

func (c *diskBedComponents) Launch(component Component, tag string, generation int64) (identity.Ref, error) {
	c.nextPid++
	ref := identity.Ref{Pid: c.nextPid, StartedAtSec: 1000 + c.nextPid}
	c.launched = append(c.launched, Held{Component: component, Tag: tag, Identity: ref, Generation: generation})
	return ref, nil
}

func (c *diskBedComponents) Observe(Held) Observation { return c.observation }

func (c *diskBedComponents) GroupCount(held []Held) (int, error) {
	live := 0
	for _, member := range held {
		if !c.stopped[member.Identity.Pid] {
			live++
		}
	}
	return live, nil
}

func (c *diskBedComponents) Stop(held Held) bool {
	c.stopped[held.Identity.Pid] = true
	return true
}

type diskBed struct {
	t          *testing.T
	repo       string
	registry   string
	tag        string
	self       identity.Ref
	components *diskBedComponents
	traces     []CycleTrace
	owner      *Owner
	clock      time.Time
}

func newDiskBed(t *testing.T, tag string) *diskBed {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(base, "checkout")
	exact, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe the owner's own identity: %v %v", state, err)
	}
	bed := &diskBed{
		t: t, repo: repo, registry: filepath.Join(base, "registry.jsonl"), tag: tag, self: exact.Ref(),
		components: &diskBedComponents{nextPid: 70000, observation: Healthy, stopped: map[int64]bool{}},
		clock:      time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC),
	}
	// The armer publishes the lock naming the owner before the owner runs.
	lockDir := filepath.Join(repo, "artifacts", "agents", "supervision", "lock.d")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bed.writeLockOwner(bed.self.Pid, bed.self.StartedAtSec, tag)
	now := func() time.Time { return bed.clock }
	bed.owner = &Owner{
		Checkout: &DiskCheckout{
			Root: repo, Self: bed.self, SelfTag: tag, IntervalSec: 1,
			Fingerprint: "disk-bed-fingerprint", WatcherCap: 60, ComponentStopCeiling: 5 * time.Second, clock: now,
		},
		Components: bed.components,
		Ledger: &RegistryLedger{CheckoutPath: repo, OwnerTag: tag, now: now, Append: func(record map[string]any) error {
			payload, err := EncodeRecord(record)
			if err != nil {
				return err
			}
			return registry.AppendFrame(bed.registry, payload)
		}},
		Intents:        &DiskIntents{Root: repo, Self: bed.self, SelfTag: tag, LatchWindow: 20 * time.Second, clock: now},
		WatcherRepairs: &DiskWatcherRepairs{Root: repo},
		// The production owner's numbers (cmd/metasystem supervise_owner.go).
		BaseInterval:  time.Second,
		Ceiling:       12,
		Breaker:       Breaker{GiveUpAt: 5, BaseInterval: time.Second, BackoffCap: 10 * time.Minute},
		Establishment: Establishment{Deadline: 5},
		TagPrefix:     tag,
		Narrate:       func(trace CycleTrace) { bed.traces = append(bed.traces, trace) },
	}
	cycles := 0
	bed.owner.Sleep = func(interval time.Duration) {
		cycles++
		if cycles > 100 {
			t.Fatalf("the owner ran 100 cycles without an exit")
		}
		bed.clock = bed.clock.Add(interval)
	}
	return bed
}

func (bed *diskBed) writeLockOwner(pid, startedAt int64, tag string) {
	bed.t.Helper()
	record, _ := json.Marshal(map[string]any{"pid": pid, "pidStartedAt": startedAt, "instanceTag": tag})
	if err := os.WriteFile(filepath.Join(bed.repo, "artifacts", "agents", "supervision", "lock.d", "owner.json"), append(record, '\n'), 0o644); err != nil {
		bed.t.Fatal(err)
	}
}

func (bed *diskBed) cycle() {
	bed.t.Helper()
	if exit := bed.owner.Cycle(bed.clock); exit != nil {
		bed.t.Fatalf("an ordinary cycle exited: %+v", exit)
	}
	bed.clock = bed.clock.Add(bed.owner.BaseInterval)
}

func (bed *diskBed) registryRecords(event string) []map[string]any {
	bed.t.Helper()
	frames, err := registry.ReadFrames(bed.registry)
	if err != nil {
		bed.t.Fatal(err)
	}
	var records []map[string]any
	for _, frame := range frames {
		if frame.Record != nil && frame.Record["event"] == event {
			records = append(records, frame.Record)
		}
	}
	return records
}

func (bed *diskBed) requireTerminal(reason string) {
	bed.t.Helper()
	exited := bed.registryRecords("exited")
	if len(exited) != 1 || exited[0]["reason"] != reason || exited[0]["teardownComplete"] != true || exited[0]["ownerTag"] != bed.tag {
		bed.t.Fatalf("registry terminal = %+v, want one %s exit with complete teardown", exited, reason)
	}
	for _, launched := range bed.components.launched {
		if !bed.components.stopped[launched.Identity.Pid] {
			bed.t.Fatalf("component %s (%d) survived the %s teardown", launched.Tag, launched.Identity.Pid, reason)
		}
	}
}

// Establish and publish: the owner publishes engine-stamped state naming
// exactly the watcher, reaper and landing owner at generation one and stays
// there (no churn), and every verdict-bearing cycle narrates its basis.
func TestDiskOwnerEstablishesAndPublishesAStableGenerationOne(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t, "gofix-establish")
	for range 4 {
		bed.cycle()
	}
	data, err := os.ReadFile(filepath.Join(bed.repo, "artifacts", "agents", "supervision", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state stateDocument
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	var names []string
	for name := range state.Components {
		names = append(names, name)
	}
	sort.Strings(names)
	if state.Engine != "go" || state.Generation != 1 || len(names) != 3 || names[0] != "landing-owner" || names[1] != "reaper" || names[2] != "watcher" {
		t.Fatalf("published state = engine %q generation %d components %v", state.Engine, state.Generation, names)
	}
	if state.Owner.Pid != bed.self.Pid || state.Owner.InstanceTag != bed.tag {
		t.Fatalf("published state names owner %+v, not this owner", state.Owner)
	}
	relaunched := bed.registryRecords("relaunched")
	if len(relaunched) != 1 || relaunched[0]["generation"] != float64(1) {
		t.Fatalf("the owner churned generations: %+v", relaunched)
	}
	if launched := bed.registryRecords("launched"); len(launched) != 3 {
		t.Fatalf("launched records = %d, want 3", len(launched))
	}

	// Observability: every verdict-bearing trace names the root and currency
	// reads, and there is at least one; each encodes as one JSON line.
	verdicts := 0
	for _, trace := range bed.traces {
		encoded, err := json.Marshal(trace)
		if err != nil || !json.Valid(encoded) {
			t.Fatalf("a cycle trace does not encode: %v", err)
		}
		if trace.Verdict == "" {
			continue
		}
		if trace.Root == "" || trace.Currency == "" {
			t.Fatalf("cycle trace does not narrate the decision basis: %+v", trace)
		}
		verdicts++
	}
	if verdicts == 0 {
		t.Fatalf("no cycle trace carried a verdict: %+v", bed.traces)
	}
}

// Purpose gone: the checkout root vanishes; the owner exits purpose-gone
// with a complete teardown and an honest terminal (KI-32 designed away).
func TestDiskOwnerExitsPurposeGoneWhenTheCheckoutVanishes(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t, "gofix-purpose")
	bed.cycle()
	if err := os.Rename(bed.repo, bed.repo+".gone"); err != nil {
		t.Fatal(err)
	}
	exit := bed.owner.Run()
	if exit.Reason != "purpose-gone" || !exit.TeardownComplete {
		t.Fatalf("exit = %+v, want purpose-gone with complete teardown", exit)
	}
	bed.requireTerminal("purpose-gone")
}

// Superseded: another identity takes the lock while the checkout persists;
// the owner leaves voluntarily (SLC-R3-003) with complete teardown.
func TestDiskOwnerLeavesWhenTheLockNamesASuccessor(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t, "gofix-super")
	bed.cycle()
	bed.writeLockOwner(999999, 1, "a-successor")
	exit := bed.owner.Run()
	if exit.Reason != "superseded" || !exit.TeardownComplete {
		t.Fatalf("exit = %+v, want superseded with complete teardown", exit)
	}
	bed.requireTerminal("superseded")
}

// Crash-loop breaker (D-2): components that never beat fail every
// observation; at five the owner gives up with a complete teardown and an
// honest terminal, relaunching between observations as the backoff allows.
func TestDiskOwnerGivesUpOnACrashLoop(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t, "gofix-breaker")
	bed.components.observation = Failing
	exit := bed.owner.Run()
	if exit.Reason != "giving-up" || !exit.TeardownComplete {
		t.Fatalf("exit = %+v, want giving-up with complete teardown", exit)
	}
	bed.requireTerminal("giving-up")
	if relaunched := bed.registryRecords("relaunched"); len(relaunched) < 2 {
		t.Fatalf("the breaker never relaunched the failing set: %d relaunched rows", len(relaunched))
	}
}
