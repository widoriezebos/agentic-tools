package launch

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// retentionFixture is a launch store under a temporary home with a unit
// store beside it, the kernel replaced by the fake prober and processes.
type retentionFixture struct {
	t         *testing.T
	manager   *Manager
	processes *fakeProcesses
	probe     *fakeProber
	root      string
	units     string
	now       time.Time
}

func newRetentionFixture(t *testing.T) *retentionFixture {
	t.Helper()
	home := t.TempDir()
	probe := &fakeProber{states: map[int64]identity.Liveness{}}
	processes := &fakeProcesses{probe: probe, groups: map[int64]bool{}}
	root := filepath.Join(home, "launch")
	units := filepath.Join(home, "unit")
	for _, dir := range []string{root, units} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	manager := &Manager{Store: Store{Root: root}, Processes: processes, Prober: probe}
	return &retentionFixture{t: t, manager: manager, processes: processes, probe: probe, root: root, units: units, now: now}
}

// launch writes one launch record whose run ended age ago, with a log of
// size bytes; pids is its supervisor and child pid (group = child).
func (f *retentionFixture) launch(id string, state State, age time.Duration, size int, pid int64) Record {
	f.t.Helper()
	supervisor, child := ref(pid), ref(pid+1)
	record := Record{ID: id, Kind: "build", State: state, Supervisor: &supervisor, Child: &child, ProcessGroup: &child,
		StartedAt: f.now.Add(-age - time.Hour).Format(time.RFC3339Nano), AdapterData: map[string]json.RawMessage{}}
	if state.Terminal() {
		record.FinishedAt = f.now.Add(-age).Format(time.RFC3339Nano)
	}
	if err := f.manager.Store.Create(record); err != nil {
		f.t.Fatal(err)
	}
	dir, _ := f.manager.Store.StateDir(id)
	if err := os.WriteFile(filepath.Join(dir, "exec.log"), []byte(strings.Repeat("x", size)), 0o600); err != nil {
		f.t.Fatal(err)
	}
	return record
}

func (f *retentionFixture) unit(id string, launches ...string) {
	f.t.Helper()
	var steps []UnitStep
	for _, launch := range launches {
		steps = append(steps, UnitStep{Name: "read", LaunchID: launch, State: StepPassed})
	}
	data, err := json.Marshal(UnitRunRecord{ID: id, State: "awaiting-judgement", Rounds: []UnitRound{{Number: 1, Steps: steps}}})
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.units, id), 0o700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.units, id, "run.json"), data, 0o600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *retentionFixture) retention(target int64) *Retention {
	return &Retention{Manager: f.manager, UnitRoot: f.units, Target: target, Keep: 14 * 24 * time.Hour}
}

func (f *retentionFixture) pass(class diskstore.Class) diskstore.Report {
	f.t.Helper()
	registry := diskstore.Registry{Dir: filepath.Join(filepath.Dir(f.root), "stores")}
	report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "machine", Name: "machine", Registry: registry,
		Mode: diskstore.ModeApply, Now: f.now, Clock: func() time.Time { return f.now }, Classes: []diskstore.Class{class}})
	if err != nil {
		f.t.Fatal(err)
	}
	return report
}

func (f *retentionFixture) exists(id string) bool {
	_, err := os.Stat(filepath.Join(f.root, id))
	return err == nil
}

const day = 24 * time.Hour

// TestLaunchRetentionReleasesOnlyProvenEndedLaunchesPastTheWindow is the
// launch proof of 3.1 (U5d): past disk.launch-keep-days and over
// disk.launch-target-mib, a terminal launch whose processes are proven
// ended, whose outputs are not unproven and that no unit record names is
// removed, oldest first, until the store is under its target; every other
// launch is kept, and a repeat changes nothing.
func TestLaunchRetentionReleasesOnlyProvenEndedLaunchesPastTheWindow(t *testing.T) {
	t.Parallel()
	f := newRetentionFixture(t)
	f.launch("old-a", Completed, 40*day, 64<<10, 100)
	f.launch("old-b", Failed, 30*day, 64<<10, 110)
	f.launch("old-c", Completed, 20*day, 64<<10, 120)
	f.launch("young", Completed, 2*day, 64<<10, 130)
	f.launch("running", Running, 30*day, 64<<10, 140)
	f.launch("named", Completed, 50*day, 64<<10, 150)
	f.unit("unit-1", "named")
	unproven := f.launch("unproven", Failed, 50*day, 64<<10, 160)
	if _, err := f.manager.Store.Update(unproven.ID, func(r *Record) error {
		r.OutputOwnerUnproven, r.Reason = true, "child-start: boom; process-group-unproven"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.launch("live", Completed, 45*day, 64<<10, 170)
	f.processes.groups[171] = true
	f.probe.states[140], f.probe.states[141] = identity.Alive, identity.Alive
	f.processes.groups[141] = true

	// Everything but one old launch's worth: two of the three removable
	// launches go, the oldest two.
	var total int64
	entries, _ := os.ReadDir(f.root)
	for _, entry := range entries {
		bytes, _, _ := diskstore.Measure(context.Background(), filepath.Join(f.root, entry.Name()))
		total += bytes
	}
	one, _, _ := diskstore.Measure(context.Background(), filepath.Join(f.root, "old-c"))
	report := f.pass(f.retention(total - 2*one))
	for id, want := range map[string]bool{"old-a": false, "old-b": false, "old-c": true, "young": true, "running": true,
		"named": true, "unproven": true, "live": true} {
		if got := f.exists(id); got != want {
			t.Errorf("launch %s present = %v, want %v; report %+v", id, got, want, report)
		}
	}
	var keptLive bool
	for _, line := range report.Kept {
		if strings.Contains(line.Path, "live") && strings.Contains(line.Command, "metasystem work stop j1:live") {
			keptLive = true
		}
	}
	if !keptLive {
		t.Errorf("a live launch past the window is kept with its public stop command: %+v", report.Kept)
	}
	if left, _ := os.ReadDir(f.root); len(left) != 6 {
		t.Errorf("no partial or renamed directory stays behind: %d entries", len(left))
	}

	again := f.pass(f.retention(total - 2*one))
	if len(again.Actions) != 0 {
		t.Errorf("a repeat under the target releases nothing: %+v", again.Actions)
	}
}

// TestLaunchRetentionUnderTargetKeepsEverything: the window alone never
// removes a launch; only a store over its target does.
func TestLaunchRetentionUnderTargetKeepsEverything(t *testing.T) {
	t.Parallel()
	f := newRetentionFixture(t)
	f.launch("old-a", Completed, 40*day, 4<<10, 100)
	report := f.pass(f.retention(512 << 20))
	if !f.exists("old-a") || len(report.Actions) != 0 {
		t.Fatalf("a store under its target keeps every launch: %+v", report)
	}
	if len(report.Classes) != 1 || report.Classes[0].Items != 1 || report.Classes[0].Bytes == 0 {
		t.Errorf("the class line counts every launch and its bytes: %+v", report.Classes)
	}
}

// TestLaunchRetentionWaitsForAnUnreadableUnitRecord: which launches a unit
// names is unknown while one unit record is unreadable, so nothing goes.
func TestLaunchRetentionWaitsForAnUnreadableUnitRecord(t *testing.T) {
	t.Parallel()
	f := newRetentionFixture(t)
	f.launch("old-a", Completed, 40*day, 4<<10, 100)
	if err := os.MkdirAll(filepath.Join(f.units, "broken"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.units, "broken", "run.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := f.pass(f.retention(1))
	if !f.exists("old-a") || len(report.Pending) == 0 {
		t.Fatalf("an unreadable unit record keeps every launch and is pending: %+v", report)
	}
}

// TestLaunchRetentionHeldRecordLockIsPending: a launch whose own record
// lock a verb holds is pending, never removed, and goes on the next pass.
func TestLaunchRetentionHeldRecordLockIsPending(t *testing.T) {
	t.Parallel()
	f := newRetentionFixture(t)
	f.launch("old-a", Completed, 40*day, 4<<10, 100)
	held, err := lock.File(filepath.Join(f.root, "old-a", ".lock"), 0o600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	report := f.pass(f.retention(1))
	if !f.exists("old-a") || len(report.Pending) != 1 {
		t.Fatalf("a held launch lock is pending: %+v", report)
	}
	_ = held.Release()
	f.pass(f.retention(1))
	if f.exists("old-a") {
		t.Fatal("the next pass releases the launch once its lock is free")
	}
}

// TestLaunchRetentionFinishesAnInterruptedRemoval: a removal cut short
// leaves the renamed directory, which no launch listing reads and the next
// pass removes.
func TestLaunchRetentionFinishesAnInterruptedRemoval(t *testing.T) {
	t.Parallel()
	f := newRetentionFixture(t)
	partial := filepath.Join(f.root, releasedPrefix+"old-a")
	if err := os.MkdirAll(filepath.Join(partial, "outputs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if records, err := f.manager.Store.List(); err != nil || len(records) != 0 {
		t.Fatalf("a renamed launch is never listed: %v %v", records, err)
	}
	f.pass(f.retention(512 << 20))
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatalf("the interrupted removal is finished: %v", err)
	}
}

// TestLaunchUpdateNeverRecreatesARemovedLaunch: an update of a launch whose
// directory is gone fails and leaves no empty directory for List to trip on.
func TestLaunchUpdateNeverRecreatesARemovedLaunch(t *testing.T) {
	t.Parallel()
	f := newRetentionFixture(t)
	if _, err := f.manager.Store.Update("gone", func(*Record) error { return nil }); err == nil {
		t.Fatal("updating an absent launch fails")
	}
	if f.exists("gone") {
		t.Fatal("an update of an absent launch recreated its directory")
	}
}

// TestSuperviseCompressesItsLogAtCompletion: a launch's log at or above
// disk.compress-above-mib is gzipped when the launch ends, verified before
// the original goes; a smaller log stays as written (3.2 "Launch").
func TestSuperviseCompressesItsLogAtCompletion(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		size       int
		compressed bool
	}{{4096, true}, {100, false}} {
		m, p, _, _ := manager(t)
		m.CompressAbove = 1024
		p.onStart = func(command Command) {
			if err := os.WriteFile(command.LogPath, []byte(strings.Repeat("log line\n", row.size/9+1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		id := fmt.Sprintf("log-%d", row.size)
		seed(t, m, id, Starting)
		got, err := m.Supervise(id)
		if err != nil || !got.State.Terminal() {
			t.Fatalf("supervise: %+v %v", got, err)
		}
		dir, _ := m.Store.StateDir(id)
		_, plainErr := os.Stat(filepath.Join(dir, "log"))
		zipped, zipErr := os.Open(filepath.Join(dir, "log.gz"))
		if row.compressed {
			if !os.IsNotExist(plainErr) || zipErr != nil {
				t.Fatalf("a %d-byte log is replaced by its gzip: plain=%v gz=%v", row.size, plainErr, zipErr)
			}
			reader, err := gzip.NewReader(zipped)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(reader)
			if !strings.HasPrefix(string(data), "log line\n") || len(data) < row.size {
				t.Errorf("the gzip holds the whole log: %d bytes", len(data))
			}
			_ = zipped.Close()
		} else if plainErr != nil || zipErr == nil {
			t.Fatalf("a %d-byte log stays as written: plain=%v gz=%v", row.size, plainErr, zipErr)
		}
	}
}

// The backstop behind the typed readers: a launch whose id any JSON file
// under the unit or design stores mentions, in any field, is kept; a file
// there that cannot be read holds the class (Round B3-4).
func TestLaunchRetentionKeepsALaunchAnyRecordMentions(t *testing.T) {
	t.Parallel()
	f := newRetentionFixture(t)
	f.launch("mentioned", Completed, 40*day, 4<<10, 100)
	f.launch("free", Completed, 40*day, 4<<10, 110)
	if err := os.MkdirAll(filepath.Join(f.units, "somewhere"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.units, "somewhere", "note.json"), []byte(`{"see":"logs of launch mentioned, retry later"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	f.pass(f.retention(1))
	if !f.exists("mentioned") || f.exists("free") {
		t.Fatalf("mentioned kept=%v free kept=%v", f.exists("mentioned"), f.exists("free"))
	}
	f.launch("free2", Completed, 40*day, 4<<10, 120)
	unreadable := filepath.Join(f.units, "somewhere", "locked.json")
	if err := os.WriteFile(unreadable, []byte("{}"), 0o000); err != nil {
		t.Fatal(err)
	}
	if report := f.pass(f.retention(1)); !f.exists("free2") || len(report.Pending) == 0 {
		t.Fatalf("an unreadable record there holds the class: %+v", report)
	}
}

// A launch whose process group was never proven ended keeps its log as
// written: it is never compressed (Round B3-4, item 7).
func TestUnprovenLaunchLogIsNeverCompressed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	log := filepath.Join(dir, "exec.log")
	if err := os.WriteFile(log, make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	compressFinishedLog(Record{State: Failed, Reason: "child-start: x; process-group-unproven"}, log, 1024)
	compressFinishedLog(Record{State: Failed, OutputOwnerUnproven: true}, log, 1024)
	if _, err := os.Stat(log); err != nil {
		t.Fatalf("an unproven launch's log stays: %v", err)
	}
	compressFinishedLog(Record{State: Completed}, log, 1024)
	if _, err := os.Stat(log + ".gz"); err != nil {
		t.Fatalf("a proven launch's log is compressed: %v", err)
	}
}
