package supervise

// Ported from scripts/agents/supervision-fixtures.sh, scenario
// census-lifecycle, S4-5: the published supervision state names exactly the
// owner's component set, each with its full identity and heartbeat path, and
// a second census writer is refused while the first is live, by name.

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestSupCPublishedStateNamesExactlyTheComponentSet(t *testing.T) {
	t.Parallel()
	checkout, root := diskCheckout(t)
	writeOwner(t, root, 41, "owner-tag")
	held := []Held{
		{Component: Watcher, Tag: "w-3", Generation: 3, Identity: identity.Ref{Pid: 50, StartedAtSec: 200}},
		{Component: Reaper, Tag: "r-3", Generation: 3, Identity: identity.Ref{Pid: 51, StartedAtSec: 201}},
		{Component: LandingOwner, Tag: "l-3", Generation: 3, Identity: identity.Ref{Pid: 52, StartedAtSec: 202}},
	}
	if err := checkout.PublishState(held); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(checkout.statePath())
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Generation int64                                 `json:"generation"`
		Components map[string]map[string]json.RawMessage `json:"components"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	var names []string
	for name := range wire.Components {
		names = append(names, name)
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"landing-owner", "reaper", "watcher"}) {
		t.Fatalf("S4-5: state components = %v", names)
	}
	for _, name := range names {
		for _, key := range []string{"pid", "pidStartedAt", "instanceTag", "heartbeat"} {
			if _, ok := wire.Components[name][key]; !ok {
				t.Fatalf("S4-5: component %s is missing %s: %s", name, key, data)
			}
		}
	}
	if wire.Generation != 3 {
		t.Fatalf("S4-5: published generation = %d", wire.Generation)
	}
}

func TestSupCSecondCensusWriterRefusalNamesTheLiveOwner(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedLockOwner(t, dir, 200, 7, "watcher-incumbent")
	contender := &CensusWriterLock{
		Dir:    dir,
		Self:   identity.Ref{Pid: 300, StartedAtSec: 8},
		Tag:    "second-writer",
		Prober: livenessProber{alive: map[int64]int64{200: 7, 300: 8}},
	}
	err := contender.Claim()
	if err == nil || !strings.Contains(err.Error(), "live census writer already owns") {
		t.Fatalf("S4-5: a second live census writer = %v", err)
	}
	if owner := readLockOwner(t, dir); owner.Pid != 200 || owner.InstanceTag != "watcher-incumbent" {
		t.Fatalf("the refused writer disturbed the live owner: %+v", owner)
	}
}

// foreign-owner (S4-11): a checkout must never stop a supervisor another
// checkout armed. The lock names a running process (this test process, so no
// process is spawned and a signal would fail the run loudly) under another
// checkout's owner tag; shutdown refuses by naming the cause, whatever the
// owner's liveness, and the lock is untouched.
func TestSupCShutdownRefusesAForeignOwnerLock(t *testing.T) {
	isolatedArmingRegistry(t)
	root := t.TempDir()
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	self, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe this process: state=%s err=%v", state, err)
	}
	foreign := ArmingOwner{
		Pid: self.Pid, PidStartedAt: self.StartedAt.Unix(), PidStartTicks: self.StartTicks, BootID: self.BootID,
		InstanceTag: "metasystem-supervision-owner-some-other-checkout-1-2", AcquiredAt: "1970-01-01T00:00:00Z",
	}
	if err := WriteArmingOwner(root, foreign); err != nil {
		t.Fatal(err)
	}
	_, err = ShutdownAt(root, root, root, "metasystem-supervision-owner-test-", 1)
	if err == nil || !strings.Contains(err.Error(), "another repository") {
		t.Fatalf("shutdown accepted a lock armed for another repository: %v", err)
	}
	read, err := ReadArmingOwner(root)
	if err != nil || !sameArmingOwner(read, foreign) {
		t.Fatalf("the refused shutdown changed the foreign owner lock: owner=%+v err=%v", read, err)
	}
}

// S4-3/S4-4: the end-of-turn watchdog surfaces fingerprint drift together
// with a dead supervision owner in one SUPERVISION DOWN line, never hiding
// the owner behind the drift.
func TestSupCWatchdogSurfacesDriftAndADeadOwnerTogether(t *testing.T) {
	repo := watchdogRepo(t)
	writeSupervisionFile(t, repo, "last-census.json", healthyCensus(watchdogNow-10))
	state := strings.Replace(healthyState(), `"fingerprint":"fp-1"`, `"fingerprint":"fp-OLD"`, 1)
	state = strings.Replace(state, `"owner":{"pid":`+itoaTest(int64(os.Getpid())), `"owner":{"pid":999999`, 1)
	writeSupervisionFile(t, repo, "state.json", state)
	lines := WatchdogReport(repo, time.Unix(watchdogNow, 0))
	if len(lines) != 1 || !strings.Contains(lines[0], "SUPERVISION DOWN") ||
		!strings.Contains(lines[0], "code changed since arming") || !strings.Contains(lines[0], "owner not running") {
		t.Fatalf("the watchdog hid drift or the dead owner: %q", lines)
	}
}
