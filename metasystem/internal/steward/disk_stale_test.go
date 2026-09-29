package steward

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

var staleNow = time.Date(2026, 9, 29, 6, 0, 0, 0, time.UTC)

// staleBed is one armed checkout in the template layout, a home state root
// with its own host registry, and a temporary root, all under one temp dir.
type staleBed struct {
	root, inst, home, registry, tmp string
}

func newStaleBed(t *testing.T) staleBed {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bed := staleBed{root: root, inst: filepath.Join(root, "repo", "metasystem"), home: filepath.Join(root, "home", ".metasystem"), tmp: filepath.Join(root, "tmp")}
	bed.registry = filepath.Join(bed.home, "armed-checkouts.jsonl")
	for _, dir := range []string{filepath.Join(root, "repo", ".git"), bed.inst, bed.home, bed.tmp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bed.inst, "metasystem.conf"), []byte("metasystem.template=true\ndisk.floor-gib=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return bed
}

func (bed staleBed) pass(mode diskstore.Mode, checkouts []string, forget bool) DiskPass {
	return DiskPass{Mode: mode, Now: staleNow, Clock: func() time.Time { return staleNow }, Home: bed.home,
		TempRoots: []string{bed.tmp}, Volumes: []string{bed.root}, Checkouts: checkouts, Registry: bed.registry, ForgetRemoved: forget,
		Proofs: map[diskstore.OwnerKind]diskstore.OwnerProof{}}
}

// row appends one registry record to the bed's host registry.
func (bed staleBed) row(t *testing.T, event, checkout, tag string, fields map[string]any) {
	t.Helper()
	record := map[string]any{"schemaVersion": 1, "event": event, "checkoutPath": checkout, "ownerTag": tag, "at": staleNow.Add(-time.Hour).Format(time.RFC3339)}
	for key, value := range fields {
		record[key] = value
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.AppendFrame(bed.registry, payload); err != nil {
		t.Fatal(err)
	}
}

func (bed staleBed) owner(t *testing.T, checkout, tag string) {
	bed.row(t, registry.EventRelaunched, checkout, tag, map[string]any{"generation": 1, "watcherTag": tag + "-w", "reaperTag": tag + "-r", "retiredThrough": 0})
}

func (bed staleBed) open(t *testing.T) map[string]bool {
	t.Helper()
	open := map[string]bool{}
	for _, path := range armedCheckoutsAt(bed.registry) {
		open[path] = true
	}
	return open
}

func hasNote(report diskstore.Report, want string) bool {
	for _, note := range report.Notes {
		if strings.Contains(note, want) {
			return true
		}
	}
	return false
}

// A registration whose checkout directory no longer exists is a stale
// registration, not an unreadable participant: it stays out of the host
// settings (R24 would make them Unknown and the machine pass would act on
// nothing), and the report counts the stale ones once. An existing
// checkout whose settings cannot be read stays Unknown as designed.
func TestMachinePassLeavesRemovedCheckoutsOutOfTheHostSettings(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	removed := []string{filepath.Join(bed.root, "gone-a", "repo"), filepath.Join(bed.root, "gone-b", "repo")}
	result, err := SweepDiskStores(context.Background(), bed.inst, bed.pass(diskstore.ModeReport, append([]string{bed.inst}, removed...), false))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Machine.HostUnknown) != 0 || result.Machine.Health.Status != diskstore.HealthOK {
		t.Fatalf("removed checkouts made the host settings unknown: %q, %+v", result.Machine.HostUnknown, result.Machine.Health)
	}
	if !hasNote(result.Machine, "stale registrations of 2 removed checkouts; metasystem disk clean forgets them") {
		t.Fatalf("the stale registrations are not counted once: %q", result.Machine.Notes)
	}
	for _, line := range result.Machine.Lines() {
		if strings.Contains(line, "gone-b") && strings.Contains(line, "unknown") {
			t.Fatalf("a removed checkout is reported unreadable: %q", line)
		}
	}

	unreadable := filepath.Join(bed.root, "exists-without-settings")
	if err := os.MkdirAll(unreadable, 0o700); err != nil {
		t.Fatal(err)
	}
	result, err = SweepDiskStores(context.Background(), bed.inst, bed.pass(diskstore.ModeReport, []string{bed.inst, unreadable}, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Machine.HostUnknown) != 1 || !strings.Contains(result.Machine.HostUnknown[0], unreadable) {
		t.Fatalf("an existing checkout with unreadable settings is not Unknown: %q", result.Machine.HostUnknown)
	}
}

// disk clean forgets the registration of a removed checkout: one reaped
// record with reason checkout-gone closes it, a registration whose recorded
// process still runs is kept, a live checkout is untouched, the steward's
// own pass only counts, and a repeat finds nothing and writes nothing.
func TestDiskCleanForgetsTheRegistrationsOfRemovedCheckouts(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	gone := filepath.Join(bed.root, "gone", "repo")
	goneRunning := filepath.Join(bed.root, "gone-running", "repo")
	bed.owner(t, gone, "owner-gone")
	bed.owner(t, bed.inst, "owner-live")
	exact, state, err := identity.KernelProber{}.ReadStart(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("read this process's start: %v %v", state, err)
	}
	bed.row(t, registry.EventArming, goneRunning, "claim-running", nil)
	bed.row(t, registry.EventArmed, goneRunning, "claim-running", map[string]any{"ownerPid": os.Getpid(), "ownerPidStartedAt": exact.StartedAt.Unix(), "generation": 0})

	steward, err := SweepDiskStores(context.Background(), bed.inst, bed.pass(diskstore.ModeApply, nil, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(steward.Forgotten) != 0 || !bed.open(t)[gone] || !hasNote(steward.Machine, "stale registrations of 2 removed checkouts; metasystem disk clean forgets them") {
		t.Fatalf("the steward's pass forgot or did not count: %v %q", steward.Forgotten, steward.Machine.Notes)
	}

	result, err := SweepDiskStores(context.Background(), bed.inst, bed.pass(diskstore.ModeApply, nil, true))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Forgotten) != 1 || result.Forgotten[0] != gone {
		t.Fatalf("forgotten = %v; want %s", result.Forgotten, gone)
	}
	open := bed.open(t)
	if open[gone] || !open[goneRunning] || !open[bed.inst] {
		t.Fatalf("open registrations after disk clean = %v", open)
	}
	if !hasNote(result.Machine, "forgot the registrations of 1 removed checkout") || !hasNote(result.Machine, "the registrations of 1 removed checkout still name a running process") {
		t.Fatalf("notes = %q", result.Machine.Notes)
	}
	frames, err := registry.ReadFrames(bed.registry)
	if err != nil {
		t.Fatal(err)
	}
	last := frames[len(frames)-1].Record
	if last["event"] != registry.EventReaped || last["reason"] != "checkout-gone" || last["ownerTag"] != "owner-gone" || last["sweepPending"] != false {
		t.Fatalf("the forgetting record = %v", last)
	}

	before, err := os.ReadFile(bed.registry)
	if err != nil {
		t.Fatal(err)
	}
	again, err := SweepDiskStores(context.Background(), bed.inst, bed.pass(diskstore.ModeApply, nil, true))
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(bed.registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Forgotten) != 0 || !bytes.Equal(before, after) {
		t.Fatalf("a repeat forgot %v or changed the registry", again.Forgotten)
	}
	if hasNote(again.Machine, "forgot ") {
		t.Fatalf("a repeat reports forgetting: %q", again.Machine.Notes)
	}
}
