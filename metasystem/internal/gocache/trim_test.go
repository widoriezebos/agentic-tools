package gocache_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

var trimNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

const trimKeep = 12 * time.Hour

// syntheticCache is a Go-layout cache under a temp dir: README and trim.txt
// at the root, entries in two-hex-digit shards.
type syntheticCache struct {
	t     *testing.T
	root  string
	state string
}

func newSyntheticCache(t *testing.T) *syntheticCache {
	t.Helper()
	base := t.TempDir()
	c := &syntheticCache{t: t, root: filepath.Join(base, "go-build"), state: filepath.Join(base, "state")}
	if err := os.MkdirAll(c.root, 0o755); err != nil {
		t.Fatal(err)
	}
	c.write("README", 10, 30*24*time.Hour)
	c.write("trim.txt", 10, 30*24*time.Hour)
	return c
}

func (c *syntheticCache) path(parts ...string) string {
	return filepath.Join(append([]string{c.root}, parts...)...)
}

// write makes a file of size bytes whose mtime is age before trimNow.
func (c *syntheticCache) write(relative string, size int, age time.Duration) {
	c.t.Helper()
	path := c.path(relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		c.t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		c.t.Fatal(err)
	}
	c.age(relative, age)
}

func (c *syntheticCache) age(relative string, age time.Duration) {
	c.t.Helper()
	when := trimNow.Add(-age)
	if err := os.Chtimes(c.path(relative), when, when); err != nil {
		c.t.Fatal(err)
	}
}

// executable makes a directory-form -d entry holding the given children
// (name to size), aged after its children are written.
func (c *syntheticCache) executable(relative string, age time.Duration, children map[string]int) {
	c.t.Helper()
	if err := os.MkdirAll(c.path(relative), 0o755); err != nil {
		c.t.Fatal(err)
	}
	for name, size := range children {
		if err := os.WriteFile(c.path(relative, name), make([]byte, size), 0o644); err != nil {
			c.t.Fatal(err)
		}
	}
	c.age(relative, age)
}

func (c *syntheticCache) exists(relative string) bool {
	_, err := os.Lstat(c.path(relative))
	return err == nil
}

func (c *syntheticCache) config(capBytes int64) gocache.TrimConfig {
	return gocache.TrimConfig{Name: "engine-go-build", Root: c.root, CapBytes: capBytes, Keep: trimKeep,
		StateDir: c.state, Now: trimNow, Clock: func() time.Time { return trimNow }}
}

func (c *syntheticCache) trim(cfg gocache.TrimConfig) gocache.TrimReport {
	c.t.Helper()
	report, err := gocache.Trim(context.Background(), cfg)
	if err != nil {
		c.t.Fatalf("trim: %v", err)
	}
	return report
}

func (c *syntheticCache) persisted() gocache.TrimReport {
	c.t.Helper()
	var report gocache.TrimReport
	data, err := os.ReadFile(filepath.Join(c.state, "engine-go-build.json"))
	if err != nil {
		c.t.Fatal(err)
	}
	if err := json.Unmarshal(data, &report); err != nil {
		c.t.Fatalf("report %s: %v", data, err)
	}
	return report
}

func requirePresent(t *testing.T, c *syntheticCache, names ...string) {
	t.Helper()
	for _, name := range names {
		if !c.exists(name) {
			t.Errorf("%s was removed", name)
		}
	}
}

func requireAbsent(t *testing.T, c *syntheticCache, names ...string) {
	t.Helper()
	for _, name := range names {
		if c.exists(name) {
			t.Errorf("%s survived", name)
		}
	}
}

// Entries inside the keep window survive whatever the cap; the oldest go
// first until the total is at or below the cap; keep-1min survives and
// keep+1min goes (DL2-22's boundary).
func TestTrimEvictsOldestFirstToTheCapNeverInsideTheKeepWindow(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	c.write("00/aaaa-a", 100, 5*24*time.Hour)
	c.write("00/aaaa-d", 1000, 4*24*time.Hour)
	c.write("01/bbbb-d", 1000, 2*24*time.Hour)
	c.write("02/cccc-d", 1000, trimKeep+time.Minute)
	c.write("03/dddd-d", 1000, trimKeep-time.Minute)
	c.write("04/eeee-d", 1000, time.Hour)
	c.write("04/ffff-a", 50, 0)
	report := c.trim(c.config(2100))
	requireAbsent(t, c, "00/aaaa-a", "00/aaaa-d", "01/bbbb-d", "02/cccc-d")
	requirePresent(t, c, "03/dddd-d", "04/eeee-d", "04/ffff-a", "README", "trim.txt")
	if report.BytesBefore != 5150 || report.BytesAfter != 2050 || report.EntriesRemoved != 4 || report.BytesRemoved != 3100 || report.EndedBy != "complete" {
		t.Fatalf("report = %+v", report)
	}

	// The keep window holds over any cap: a cap of zero leaves everything
	// used within the window and says so.
	c2 := newSyntheticCache(t)
	c2.write("00/aaaa-d", 1000, trimKeep-time.Minute)
	c2.write("01/bbbb-d", 1000, time.Minute)
	report = c2.trim(c2.config(1))
	requirePresent(t, c2, "00/aaaa-d", "01/bbbb-d")
	if report.BytesAfter != 2000 || report.KeepWindowEntries != 2 || report.KeepWindowBytes != 2000 || report.EntriesRemoved != 0 {
		t.Fatalf("keep-window report = %+v", report)
	}
}

// README, trim.txt and every name without the -a/-d suffix survive, in the
// root and in a shard, and are not counted.
func TestTrimNeverTouchesANameWithoutTheEntrySuffix(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	c.write("00/notes.txt", 5000, 40*24*time.Hour)
	c.write("00/aaaa-x", 5000, 40*24*time.Hour)
	c.write("stray-a", 5000, 40*24*time.Hour)
	c.write("0a/aaaa-a", 10, 40*24*time.Hour)
	report := c.trim(c.config(1))
	requirePresent(t, c, "README", "trim.txt", "00/notes.txt", "00/aaaa-x", "stray-a")
	requireAbsent(t, c, "0a/aaaa-a")
	if report.BytesBefore != 10 {
		t.Fatalf("a non-entry name was measured: %+v", report)
	}
}

// A well-formed directory entry (one regular child) is measured by its
// child's bytes and goes whole through the aside rename (DL4A-08); one with
// two children, zero children, a symlink child or a nested directory is
// Unknown and its name is unchanged afterwards (DL4A-07).
func TestTrimClassifiesADirectoryEntryWholeBeforeTouchingIt(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	c.executable("10/good-d", 3*24*time.Hour, map[string]int{"metasystem": 4096})
	c.executable("11/two-d", 3*24*time.Hour, map[string]int{"a": 10, "b": 10})
	c.executable("12/empty-d", 3*24*time.Hour, nil)
	c.executable("13/link-d", 3*24*time.Hour, nil)
	if err := os.Symlink("/etc/hosts", c.path("13/link-d", "tool")); err != nil {
		t.Fatal(err)
	}
	c.age("13/link-d", 3*24*time.Hour)
	c.executable("14/nest-d", 3*24*time.Hour, nil)
	if err := os.Mkdir(c.path("14/nest-d", "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	c.age("14/nest-d", 3*24*time.Hour)

	// Measured by the child's bytes, never the directory's stat size.
	report := c.trim(c.config(1 << 40))
	if report.BytesBefore != 4096 {
		t.Fatalf("a directory entry was measured by %d bytes, want its child's 4096: %+v", report.BytesBefore, report)
	}
	unknown := map[string]string{}
	for _, entry := range report.Unknown {
		unknown[entry.Name] = entry.Reason
	}
	for _, name := range []string{"11/two-d", "12/empty-d", "13/link-d", "14/nest-d"} {
		if unknown[name] == "" {
			t.Errorf("%s is not reported Unknown: %v", name, report.Unknown)
		}
	}

	report = c.trim(c.config(1))
	requireAbsent(t, c, "10/good-d", "10/good-d.trim")
	requirePresent(t, c, "11/two-d", "11/two-d/a", "11/two-d/b", "12/empty-d", "13/link-d", "13/link-d/tool", "14/nest-d/inner")
	for _, name := range []string{"11/two-d.trim", "12/empty-d.trim", "13/link-d.trim", "14/nest-d.trim"} {
		if c.exists(name) {
			t.Errorf("an Unknown entry was renamed to %s", name)
		}
	}
	if report.EntriesRemoved != 1 || report.BytesRemoved != 4096 {
		t.Fatalf("report = %+v", report)
	}
}

// An aside left by a killed pass is finished by the next pass and counted
// removed; an aside that fails classification is reported by name on every
// pass and never touched.
func TestTrimFinishesAnInterruptedAsideAndReportsABadOneEveryPass(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	c.executable("20/killed-d.trim", time.Minute, map[string]int{"tool": 700})
	c.executable("21/emptied-d.trim", time.Minute, nil)
	c.executable("22/foreign-d.trim", time.Minute, map[string]int{"a": 1, "b": 1})
	for pass := 1; pass <= 2; pass++ {
		report := c.trim(c.config(1 << 40))
		if pass == 1 && report.AsidesFinished != 2 {
			t.Fatalf("pass 1 finished %d asides: %+v", report.AsidesFinished, report)
		}
		found := false
		for _, entry := range report.Unknown {
			found = found || entry.Name == "22/foreign-d.trim"
		}
		if !found {
			t.Fatalf("pass %d does not report the unclassifiable aside: %+v", pass, report.Unknown)
		}
	}
	requireAbsent(t, c, "20/killed-d.trim", "21/emptied-d.trim")
	requirePresent(t, c, "22/foreign-d.trim/a", "22/foreign-d.trim/b")
}

// A stat error skips the entry as Unknown; a symlink entry is skipped; a
// symlink at a shard position or at the root refuses that cache with
// nothing touched (DL3A-09).
func TestTrimRefusesSymlinkedShardsAndSkipsWhatItCannotStat(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	c.write("30/old-d", 100, 40*24*time.Hour)
	c.write("31/vanish-d", 100, 40*24*time.Hour)
	if err := os.Symlink("/etc/hosts", c.path("30/link-a")); err != nil {
		t.Fatal(err)
	}
	cfg := gocache.WithTrimHooks(c.config(1), gocache.TrimHooks{BeforeStat: func(shard, name string) {
		if name == "vanish-d" {
			_ = os.Rename(c.path("31/vanish-d"), c.path("31/moved-away"))
		}
	}})
	report := c.trim(cfg)
	requireAbsent(t, c, "30/old-d")
	requirePresent(t, c, "30/link-a", "31/moved-away")
	reasons := map[string]string{}
	for _, entry := range report.Unknown {
		reasons[entry.Name] = entry.Reason
	}
	if reasons["30/link-a"] == "" || reasons["31/vanish-d"] == "" {
		t.Fatalf("unknown = %v", report.Unknown)
	}

	shard := newSyntheticCache(t)
	shard.write("00/old-d", 100, 40*24*time.Hour)
	elsewhere := t.TempDir()
	if err := os.WriteFile(filepath.Join(elsewhere, "victim-d"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	old := trimNow.Add(-40 * 24 * time.Hour)
	_ = os.Chtimes(filepath.Join(elsewhere, "victim-d"), old, old)
	if err := os.Symlink(elsewhere, shard.path("ff")); err != nil {
		t.Fatal(err)
	}
	report = shard.trim(shard.config(1))
	if report.EndedBy != "refused" || !strings.Contains(report.Reason, "ff") {
		t.Fatalf("a symlinked shard did not refuse the cache: %+v", report)
	}
	requirePresent(t, shard, "00/old-d")
	if _, err := os.Stat(filepath.Join(elsewhere, "victim-d")); err != nil {
		t.Fatalf("the trimmer followed a shard symlink: %v", err)
	}

	root := t.TempDir()
	if err := os.Symlink(shard.root, filepath.Join(root, "go-build")); err != nil {
		t.Fatal(err)
	}
	cfg = shard.config(1)
	cfg.Root = filepath.Join(root, "go-build")
	report, err := gocache.Trim(context.Background(), cfg)
	if err != nil || report.EndedBy != "refused" {
		t.Fatalf("a symlinked root: %+v %v", report, err)
	}
	requirePresent(t, shard, "00/old-d")
}

// A shard of 10,000 entries under a budget that allows three batches
// checkpoints within the shard, and the next pass resumes at the
// checkpointed name (DL3A-10).
func TestTrimCheckpointsWithinAShardAndResumes(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	names := make([]string, 10000)
	for index := range names {
		names[index] = fmt.Sprintf("%064x-a", index)
		c.write("5a/"+names[index], 1, time.Minute)
	}
	slices.Sort(names)
	batches := 0
	cfg := c.config(1 << 40)
	cfg.Stopped = func() bool { batches++; return batches >= 3 }
	report := c.trim(cfg)
	if report.EndedBy != "cancelled" || report.Checkpoint.Shard != "5a" || report.Checkpoint.LastName != names[3*256-1] || report.Checkpoint.BytesSoFar != 3*256 {
		t.Fatalf("checkpoint after three batches = %+v (ended by %s)", report.Checkpoint, report.EndedBy)
	}
	if persisted := c.persisted(); persisted.Checkpoint != report.Checkpoint {
		t.Fatalf("the checkpoint was not persisted: %+v", persisted.Checkpoint)
	}
	var first string
	cfg = gocache.WithTrimHooks(c.config(1<<40), gocache.TrimHooks{BeforeStat: func(shard, name string) {
		if first == "" {
			first = name
		}
	}})
	report = c.trim(cfg)
	if first != names[3*256] || report.EndedBy != "complete" || report.BytesBefore != 10000 {
		t.Fatalf("resumed at %q, want %q; report %+v", first, names[3*256], report)
	}
}

// The persisted candidates let a pass evict without re-measuring, and an
// entry whose mtime moved between plan and re-stat is skipped.
func TestTrimEvictsFromPersistedCandidatesAndSkipsAMovedEntry(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	c.write("00/one-d", 100, 5*24*time.Hour)
	c.write("01/two-d", 100, 4*24*time.Hour)
	c.write("02/three-d", 100, 3*24*time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	report, err := gocache.Trim(ctx, gocache.WithTrimHooks(c.config(200), gocache.TrimHooks{AfterMeasure: cancel}))
	if err != nil || report.EndedBy != "cancelled" || report.Phase != "evict" {
		t.Fatalf("first pass: %+v %v", report, err)
	}
	requirePresent(t, c, "00/one-d", "01/two-d", "02/three-d")
	c.age("00/one-d", 2*24*time.Hour) // used again: its mtime moved
	stats := 0
	report = c.trim(gocache.WithTrimHooks(c.config(200), gocache.TrimHooks{BeforeStat: func(string, string) { stats++ }}))
	if stats != 0 {
		t.Fatalf("the evict pass re-measured %d entries", stats)
	}
	requirePresent(t, c, "00/one-d", "02/three-d")
	requireAbsent(t, c, "01/two-d")
	if report.MovedSkips != 1 || report.EntriesRemoved != 1 {
		t.Fatalf("report = %+v", report)
	}
}

// A held flock skips the cache with the reason; a cancelled context
// returns promptly with a consistent report.
func TestTrimSkipsAHeldCacheAndStopsOnCancellation(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	c.write("00/one-d", 100, 5*24*time.Hour)
	if err := os.MkdirAll(c.state, 0o755); err != nil {
		t.Fatal(err)
	}
	held, err := os.OpenFile(filepath.Join(c.state, "engine-go-build.flock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Flock(int(held.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	report := c.trim(c.config(1))
	if report.EndedBy != "lock-held" || report.Reason != "another steward trims" {
		t.Fatalf("held flock: %+v", report)
	}
	requirePresent(t, c, "00/one-d")
	_ = held.Close()

	for index := 0; index < 3000; index++ {
		c.write(fmt.Sprintf("%02x/%04d-a", index%256, index), 1, time.Minute)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Promptly is counted in work, not wall time: a pass cancelled before
	// it starts stats and removes nothing of the 3000 entries, whatever
	// the host's load.
	stats, removals := 0, 0
	report, err = gocache.Trim(ctx, gocache.WithTrimHooks(c.config(1), gocache.TrimHooks{
		BeforeStat:   func(string, string) { stats++ },
		BeforeRemove: func(string, string) { removals++ },
	}))
	if stats != 0 || removals != 0 {
		t.Fatalf("a cancelled pass statted %d and removed %d entries", stats, removals)
	}
	if err != nil || report.EndedBy != "cancelled" {
		t.Fatalf("cancelled pass: %+v %v", report, err)
	}
	if persisted := c.persisted(); persisted.EndedBy != "cancelled" || persisted.Checkpoint != report.Checkpoint {
		t.Fatalf("persisted report is not the pass's: %+v", persisted)
	}
}

// A repeat with nothing over cap changes nothing but the report's
// timestamp (R-129).
func TestTrimRepeatChangesOnlyTheTimestamp(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	c.write("00/one-d", 100, 5*24*time.Hour)
	c.write("01/two-d", 100, time.Hour)
	c.trim(c.config(150))
	second := c.trim(c.config(150))
	later := c.config(150)
	later.Clock = func() time.Time { return trimNow.Add(time.Minute) }
	third := c.trim(later)
	if third.PassAt == second.PassAt {
		t.Fatalf("the timestamp did not move: %s", third.PassAt)
	}
	second.PassAt, third.PassAt = "", ""
	if fmt.Sprintf("%+v", second) != fmt.Sprintf("%+v", third) {
		t.Fatalf("a repeat changed the report:\n%+v\n%+v", second, third)
	}
	requirePresent(t, c, "01/two-d")
}

// The accepted failure (DL3A-13): a reader holds an entry's path, the
// trimmer evicts it, the reader's open fails with ENOENT, and nothing else
// in the cache changed.
func TestTrimEvictionUnderAReaderFailsItsOpenAndNothingElse(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	c.write("00/held-d", 100, 5*24*time.Hour)
	c.write("01/other-d", 100, time.Hour)
	held := c.path("00/held-d")
	c.trim(c.config(100))
	_, err := os.Open(held)
	if !errors.Is(err, syscall.ENOENT) {
		t.Fatalf("the reader's open = %v, want ENOENT", err)
	}
	requirePresent(t, c, "01/other-d", "README", "trim.txt")
}

// An absent cache is nothing to do.
func TestTrimOfAnAbsentCacheIsNothingToDo(t *testing.T) {
	t.Parallel()
	c := newSyntheticCache(t)
	cfg := c.config(1)
	cfg.Root = filepath.Join(c.root, "missing")
	report := c.trim(cfg)
	if report.EndedBy != "absent" {
		t.Fatalf("report = %+v", report)
	}
}
