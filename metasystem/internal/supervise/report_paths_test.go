package supervise

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestShutdownReportLinesRenderEveryOutcomeShape(t *testing.T) {
	report := ShutdownReport{
		Outcomes: []ComponentOutcome{
			{Component: "watcher", Identity: identity.Ref{Pid: 11}, Result: ShutdownAlreadyGone},
			{Component: "reaper", Identity: identity.Ref{Pid: 12}, Result: ShutdownAlreadyGone, Reason: "exited before the signal"},
			{Component: "supervision-owner", Identity: identity.Ref{Pid: 13}, Tag: "tag-a", Generation: 7, Result: ShutdownStopped, Signal: ShutdownSignalKill, Reason: "shutdown-escalated"},
			{Component: "watcher", Identity: identity.Ref{Pid: 14}, Result: ShutdownStopped, Signal: ShutdownSignalKill},
			{Component: "supervision-owner", Identity: identity.Ref{Pid: 15}, Tag: "tag-b", Generation: 8, Result: ShutdownStopped, Signal: ShutdownSignalTerm, Reason: "shutdown"},
			{Component: "reaper", Identity: identity.Ref{Pid: 16}, Result: ShutdownStopped, Signal: ShutdownSignalTerm},
			{Component: "watcher", Identity: identity.Ref{Pid: 17, StartedAtSec: 1700000000}, Result: ShutdownNotStopped, Reason: "TERM and KILL both refused"},
		},
		Failures: []ShutdownFailure{{Component: "supervision-lock", Path: "/lock.d", Reason: "busy", Did: "left the lock"}},
	}
	want := []string{
		"watcher pid 11: already gone",
		"reaper pid 12: already gone (exited before the signal)",
		"supervision-owner pid 13 tag tag-a generation 7: killed (TERM ignored; reaped reason=shutdown-escalated)",
		"watcher pid 14: killed (TERM ignored)",
		"supervision-owner pid 15 tag tag-b generation 8: stopped (TERM, exited reason=shutdown)",
		"reaper pid 16: stopped (TERM)",
		"NOT STOPPED watcher pid 17 started 1700000000: TERM and KILL both refused; did: left it listed in the fence record",
		"NOT STOPPED supervision-lock /lock.d: busy; did: left the lock",
	}
	got := report.Lines()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lines =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if report.Complete() {
		t.Fatal("a report with a not-stopped outcome and a failure read as complete")
	}
}

func TestMergeTakeoverComponentsOrdersAndRefusesConflicts(t *testing.T) {
	recorded := []Held{
		{Component: Reaper, Tag: "t", Identity: identity.Ref{Pid: 30, StartedAtSec: 5}, Generation: 2},
		{Component: Watcher, Tag: "t", Identity: identity.Ref{Pid: 40, StartedAtSec: 5}, Generation: 3},
	}
	discovered := []Held{
		{Component: Watcher, Tag: "t", Identity: identity.Ref{Pid: 20, StartedAtSec: 5}, Generation: 2},
		{Component: Reaper, Tag: "t", Identity: identity.Ref{Pid: 30, StartedAtSec: 5}, Generation: 2},
		{Component: Reaper, Tag: "t", Identity: identity.Ref{Pid: 10, StartedAtSec: 5}, Generation: 2},
	}
	held, err := mergeTakeoverComponents(recorded, discovered)
	if err != nil {
		t.Fatal(err)
	}
	var order []int64
	for _, member := range held {
		order = append(order, member.Identity.Pid)
	}
	// Watchers first by generation, then the rest by generation and pid; the
	// duplicate pid 30 counts once.
	if len(order) != 4 || order[0] != 20 || order[1] != 40 || order[2] != 10 || order[3] != 30 {
		t.Fatalf("merged order = %v", order)
	}
	conflicting := []Held{{Component: Watcher, Tag: "other", Identity: identity.Ref{Pid: 30, StartedAtSec: 5}, Generation: 2}}
	if _, err := mergeTakeoverComponents(recorded, conflicting); err == nil || !strings.Contains(err.Error(), "pid 30 has conflicting") {
		t.Fatalf("conflict = %v", err)
	}
}

type startProbe struct {
	entries map[int64]identity.FixtureEntry
}

func (p startProbe) FixtureEntry(pid int64) (identity.FixtureEntry, bool) {
	entry, ok := p.entries[pid]
	return entry, ok
}

func TestExactStartMicroPrefersTheFixtureDeclarationOfALiveProcess(t *testing.T) {
	self := int64(os.Getpid())
	declared := startProbe{entries: map[int64]identity.FixtureEntry{self: {StartedAt: 1_700_000_000, HasStartedAt: true}}}
	if micro, ok := exactStartMicro(self, declared); !ok || micro != 1_700_000_000_000_000 {
		t.Fatalf("declared start = %d %v", micro, ok)
	}
	undeclared := startProbe{entries: map[int64]identity.FixtureEntry{self: {HasStartTicks: true, StartTicks: 9}}}
	kernel, ok := exactStartMicro(self, undeclared)
	if !ok || kernel <= 0 {
		t.Fatalf("kernel start = %d %v", kernel, ok)
	}
	if bare, ok := exactStartMicro(self, nil); !ok || bare != kernel {
		t.Fatalf("nil probe start = %d %v, want the kernel's %d", bare, ok, kernel)
	}
	if _, ok := exactStartMicro(int64(1<<30), declared); ok {
		t.Fatal("an absent pid proved a start token")
	}
}

func TestReadInventoryWithoutAnOwnerRecordReadsTheTagFromStateAndRefusesAnUninspectableOwner(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(SupervisionDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	// No owner record and no state: an empty inventory, no error.
	if items, err := ReadInventory(root, nil); err != nil || len(items) != 0 {
		t.Fatalf("empty checkout inventory = %+v, %v", items, err)
	}
	// No owner record, but a published state names the tag: the takeover
	// census runs under that tag (nothing to find in an empty snapshot).
	document := stateDocument{Owner: stateIdentity{Pid: 41, PidStartedAt: 100, InstanceTag: "owner-tag-from-state"}, Generation: 3}
	if err := writeArmingHelperJSON(filepath.Join(SupervisionDir(root), "state.json"), document); err != nil {
		t.Fatal(err)
	}
	if items, err := ReadInventory(root, nil); err != nil || len(items) != 0 {
		t.Fatalf("state-tagged inventory = %+v, %v", items, err)
	}
	// An owner record whose identity cannot be inspected refuses the read.
	if err := WriteArmingOwner(root, ArmingOwner{Pid: 41, PidStartedAt: 100, InstanceTag: "owner-tag", FenceGeneration: 12}); err != nil {
		t.Fatal(err)
	}
	prior := armingOwnerLiveness
	armingOwnerLiveness = func(ArmingOwner) identity.Liveness { return identity.Unknown }
	t.Cleanup(func() { armingOwnerLiveness = prior })
	if _, err := ReadInventory(root, nil); err == nil || !strings.Contains(err.Error(), "uninspectable") {
		t.Fatalf("uninspectable owner = %v", err)
	}
	// A dead owner is simply absent from the inventory.
	armingOwnerLiveness = func(ArmingOwner) identity.Liveness { return identity.Dead }
	if items, err := ReadInventory(root, nil); err != nil || len(items) != 0 {
		t.Fatalf("dead owner inventory = %+v, %v", items, err)
	}
}
