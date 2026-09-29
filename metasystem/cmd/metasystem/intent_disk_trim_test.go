package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

// diskCleanBed is an installation with the shipped trim defaults, a
// machine user cache dir under a temp dir and the trimmer's state beside it.
type diskCleanBed struct {
	root, userCache, state string
	owners                 intentOwners
}

var diskCleanNow = time.Date(2026, 9, 28, 22, 0, 0, 0, time.UTC)

func newDiskCleanBed(t *testing.T) diskCleanBed {
	t.Helper()
	root, owners := newHomesSettingsInstallation(t)
	bed := diskCleanBed{root: root, userCache: t.TempDir(), state: filepath.Join(t.TempDir(), "cache-trim")}
	owners.disk = diskOwners{
		userCacheDir: func() (string, error) { return bed.userCache, nil },
		stateDir:     bed.state,
		now:          func() time.Time { return diskCleanNow },
	}
	bed.owners = owners
	return bed
}

// entry plants a sparse cache entry of size bytes, last used age ago.
func (bed diskCleanBed) entry(t *testing.T, cache, name string, size int64, age time.Duration) string {
	t.Helper()
	path := filepath.Join(bed.userCache, cache, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(size); err != nil {
		t.Fatal(err)
	}
	file.Close()
	when := diskCleanNow.Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	return path
}

func (bed diskCleanBed) run(t *testing.T, args ...string) (int, intentResult, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "disk clean"), append(args, "--json"), &stdout, &stderr, bed.root, bed.owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
	}
	return code, result, stdout.String()
}

// `disk clean --go-cache` runs the steward's trimmer now (disk-lifetimes
// A12, Part B's manual entry): the engine's Go cache, Go's default cache
// every builder by hand also uses, is trimmed by last use to its cap: over
// the cap the keep window yields, oldest first, and an entry used within
// the last disk.cache-min-keep-minutes stays whatever the cap; the line
// says so plainly.
func TestDiskCleanGoCacheTrimsTheMachineCachesNow(t *testing.T) {
	t.Parallel()
	bed := newDiskCleanBed(t)
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.runtimes=claude\ndisk.go-cache-cap-gib=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const gib = int64(1) << 30
	old := bed.entry(t, "go-build", "00/old-d", 2*gib, 3*24*time.Hour)
	withinKeep := bed.entry(t, "go-build", "02/keep-d", 2*gib, 3*time.Hour)
	recent := bed.entry(t, "go-build", "01/recent-d", 2*gib, 30*time.Minute)
	delegate := bed.entry(t, "metasystem-delegate-go-build", "02/dele-d", 11*gib, 2*24*time.Hour)
	code, result, printed := bed.run(t, "--go-cache")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("disk clean = %d %s", code, printed)
	}
	for _, path := range []string{old, withinKeep, delegate} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s was not trimmed: %v", path, err)
		}
	}
	if _, err := os.Stat(recent); err != nil {
		t.Errorf("an entry used within the floor went: %v", err)
	}
	var decoded struct {
		Data struct {
			Caches []gocache.TrimReport `json:"caches"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(printed), &decoded); err != nil {
		t.Fatal(err)
	}
	said := false
	for _, report := range decoded.Data.Caches {
		if report.Cache == "engine-go-build" {
			said = strings.Contains(diskTrimLine(report), "over the cap: removing the oldest entries, keeping the last 60 min")
		}
	}
	if !said {
		t.Fatalf("the engine Go cache's line does not say the keep window yielded: %s", printed)
	}
	if !strings.Contains(printed, "engine-go-build") || !strings.Contains(printed, "delegate-go-build") {
		t.Fatalf("the report does not name the caches: %s", printed)
	}
	if _, err := os.Stat(filepath.Join(bed.state, "engine-go-build.json")); err != nil {
		t.Fatalf("no report beside the steward's: %v", err)
	}
}

// The verb refuses only input: an argument, or a settings file whose disk
// numbers are not whole numbers, names what to fix and trims nothing.
func TestDiskCleanRefusesOnlyInput(t *testing.T) {
	t.Parallel()
	bed := newDiskCleanBed(t)
	if code, result, _ := bed.run(t, "--go-cache", "extra"); code != 2 || result.Outcome != intentRefused {
		t.Fatalf("an argument = %d %+v", code, result)
	}
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.runtimes=claude\ndisk.go-cache-cap-gib=lots\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, result, _ := bed.run(t, "--go-cache")
	if code != 2 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "disk.go-cache-cap-gib") {
		t.Fatalf("a bad setting = %d %+v", code, result)
	}
}

// witnessDiskTrimRepeat runs disk clean --go-cache twice on caches within their caps:
// both succeed, and the caches are byte for byte what they were.
func witnessDiskTrimRepeat(t *testing.T) {
	bed := newDiskCleanBed(t)
	bed.entry(t, "go-build", "00/kept-d", 100, 3*24*time.Hour)
	bed.entry(t, "staticcheck", "01/kept-a", 100, 3*24*time.Hour)
	before := idemTreeDigest(t, bed.userCache)
	for run := 1; run <= 2; run++ {
		code, result, printed := bed.run(t, "--go-cache")
		if code != 0 || result.Outcome != intentConfirmed || !strings.Contains(result.Summary, "nothing removed") {
			t.Fatalf("disk clean %d = %d %s", run, code, printed)
		}
		idemSameTree(t, "a repeated disk clean", before, idemTreeDigest(t, bed.userCache))
	}
}

// The headline never contradicts the cache lines under it: "within their
// caps" only when every cache was measured whole and is under its cap; a
// cache still measuring says so, with what was counted and how it resumes.
func TestDiskTrimHeadlineMatchesItsLines(t *testing.T) {
	t.Parallel()
	const gib = int64(1) << 30
	within := func(cache string) gocache.TrimReport {
		return gocache.TrimReport{Cache: cache, EndedBy: "complete", Phase: "idle", CapBytes: 2 * gib, BytesAfter: gib}
	}
	measuring := gocache.TrimReport{Cache: "engine-go-build", EndedBy: "budget", Phase: "measure", CapBytes: 30 * gib,
		Checkpoint: gocache.TrimCheckpoint{Shard: "c0", BytesSoFar: 749 * gib / 10}}
	for _, tc := range []struct {
		name    string
		reports []gocache.TrimReport
		want    []string
		done    bool
	}{
		{name: "every cache within its cap", reports: []gocache.TrimReport{within("engine-go-build"), {Cache: "delegate-go-build", EndedBy: "absent", Phase: "idle"}},
			want: []string{"the machine caches are within their caps: nothing removed"}, done: true},
		{name: "the engine cache still measuring", reports: []gocache.TrimReport{measuring, within("engine-staticcheck")},
			want: []string{"still measuring the engine Go cache (74.9 GiB counted so far); it resumes on the next pass: run metasystem disk clean --go-cache again or let the steward continue"}},
		{name: "a cache the pass never reached", reports: []gocache.TrimReport{measuring, {Cache: "engine-staticcheck", EndedBy: "budget", Phase: "measure", CapBytes: 2 * gib, Checkpoint: gocache.TrimCheckpoint{Shard: "00"}}},
			want: []string{"still measuring the engine Go cache (74.9 GiB counted so far); the engine staticcheck cache is not measured yet; they resume on the next pass"}},
		{name: "trimmed and still evicting", reports: []gocache.TrimReport{{Cache: "delegate-go-build", EndedBy: "budget", Phase: "evict", CapBytes: 10 * gib, EntriesRemoved: 4, BytesRemoved: 3 * gib}},
			want: []string{"trimmed the machine caches: 4 entries, 3.0 GiB freed", "still trimming the delegate Go cache to its 10.0 GiB cap"}},
		{name: "over its cap inside the floor", reports: []gocache.TrimReport{{Cache: "engine-go-build", EndedBy: "complete", Phase: "idle", CapBytes: gib, BytesAfter: 3 * gib, KeepWindowBytes: 3 * gib, MinKeepBytes: 2 * gib, MinKeepMinutes: 60}},
			want: []string{"the engine Go cache stays over its 1.0 GiB cap: 2.0 GiB used within the last 60 min is never trimmed"}},
		{name: "refused and held", reports: []gocache.TrimReport{{Cache: "engine-staticcheck", EndedBy: "refused", Reason: "shard 00 is a symbolic link"}, {Cache: "delegate-staticcheck", EndedBy: "lock-held"}},
			want: []string{"the engine staticcheck cache was not trimmed: shard 00 is a symbolic link", "another steward is trimming the delegate staticcheck cache now"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			summary, done := diskTrimSummary(tc.reports)
			for _, want := range tc.want {
				if !strings.Contains(summary, want) {
					t.Errorf("summary %q does not say %q", summary, want)
				}
			}
			if done != tc.done || (!tc.done && (strings.Contains(summary, "within their caps") || strings.Contains(summary, "within its cap"))) {
				t.Errorf("summary %q done=%v; want done=%v and no claim of within the caps", summary, done, tc.done)
			}
		})
	}
}

// fakeTrimPasses is a trim that needs measuring passes before the engine Go
// cache completes; each pass advances the clock by its budget.
type fakeTrimPasses struct {
	measuring int
	calls     int
	now       time.Time
}

func (f *fakeTrimPasses) pass(_ context.Context, caches []string, _ time.Time, budget time.Duration) ([]gocache.TrimReport, error) {
	f.calls++
	f.now = f.now.Add(budget)
	if len(caches) == 0 {
		caches = gocache.MachineCaches()
	}
	var reports []gocache.TrimReport
	for _, cache := range caches {
		report := gocache.TrimReport{Cache: cache, EndedBy: "complete", Phase: "idle", CapBytes: 30 << 30, BytesAfter: 1 << 30}
		if cache == "engine-go-build" && f.calls <= f.measuring {
			report.EndedBy, report.Phase = "budget", "measure"
			report.Checkpoint = gocache.TrimCheckpoint{Shard: "c0", BytesSoFar: int64(f.calls) * 10 << 30}
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// A person running disk clean --go-cache expects it to finish the job: it
// continues pass after pass, telling each on stderr, until every cache is
// done; the person's budget bounds it, and then the headline says what is
// still measuring.
func TestDiskCleanGoCacheFinishesTheJobForAPerson(t *testing.T) {
	t.Parallel()
	bed := newDiskCleanBed(t)
	fake := &fakeTrimPasses{measuring: 2, now: diskCleanNow}
	bed.owners.disk.trimPass = fake.pass
	bed.owners.disk.now = func() time.Time { return fake.now }
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "disk clean"), []string{"--go-cache"}, &stdout, &stderr, bed.root, bed.owners)
	if code != 0 || fake.calls != 3 {
		t.Fatalf("disk clean = %d after %d passes:\n%s%s", code, fake.calls, stdout.String(), stderr.String())
	}
	progress := stderr.String()
	if !strings.Contains(progress, "pass 1: engine-go-build: budget, measuring (10.0 GiB counted so far") || !strings.Contains(progress, "pass 2:") {
		t.Fatalf("no progress lines:\n%s", progress)
	}
	if printed := stdout.String() + stderr.String(); !strings.Contains(printed, "within their caps") || strings.Contains(printed, "metasystem disk clean: still measuring") {
		t.Fatalf("the finished job is not told:\n%s", printed)
	}

	spent := newDiskCleanBed(t)
	if err := os.WriteFile(filepath.Join(spent.root, "metasystem.conf"), []byte("metasystem.runtimes=claude\ndisk.cache-trim-person-budget-sec=25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	endless := &fakeTrimPasses{measuring: 1000, now: diskCleanNow}
	spent.owners.disk.trimPass = endless.pass
	spent.owners.disk.now = func() time.Time { return endless.now }
	code, result, printed := spent.run(t, "--go-cache")
	if code != 0 || endless.calls != 3 || !strings.Contains(result.Summary, "still measuring the engine Go cache (30.0 GiB counted so far); it resumes on the next pass") {
		t.Fatalf("the person's budget: %d passes, %d %s", endless.calls, code, printed)
	}
}

// disk show tells the caches as the last trim pass left them, changing
// nothing.
func TestDiskShowTellsTheCaches(t *testing.T) {
	t.Parallel()
	bed := newDiskCleanBed(t)
	bed.entry(t, "go-build", "00/kept-d", 100, 3*24*time.Hour)
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(mustIntentCommand(t, "disk show"), nil, &stdout, &stderr, bed.root, bed.owners); code != 0 ||
		!strings.Contains(stdout.String()+stderr.String(), "caches: no trim pass has run yet") {
		t.Fatalf("show before a trim = %d:\n%s%s", code, stdout.String(), stderr.String())
	}
	if code, _, printed := bed.run(t, "--go-cache"); code != 0 {
		t.Fatalf("trim = %d %s", code, printed)
	}
	before := idemTreeDigest(t, bed.state)
	stdout.Reset()
	stderr.Reset()
	code := runIntentIn(mustIntentCommand(t, "disk show"), nil, &stdout, &stderr, bed.root, bed.owners)
	printed := stdout.String() + stderr.String()
	if code != 0 || !strings.Contains(printed, "caches, as the last trim pass left them: the machine caches are within their caps") || !strings.Contains(printed, "engine-go-build: complete") {
		t.Fatalf("show after a trim = %d:\n%s", code, printed)
	}
	idemSameTree(t, "disk show", before, idemTreeDigest(t, bed.state))
}

// A cache found over its cap says plainly that the keep window yielded:
// the oldest entries go and only the last minutes of the floor stay, both
// when the whole measurement found it and when a partial one did.
func TestDiskTrimLineSaysTheKeepWindowYieldsOverTheCap(t *testing.T) {
	t.Parallel()
	const gib = int64(1) << 30
	const plain = "over the cap: removing the oldest entries, keeping the last 60 min"
	for _, tc := range []struct {
		name   string
		report gocache.TrimReport
		says   bool
	}{
		{name: "trimmed to the cap", report: gocache.TrimReport{Cache: "engine-go-build", EndedBy: "complete", Phase: "idle", CapBytes: 30 * gib, BytesAfter: 30 * gib, EntriesRemoved: 9, BytesRemoved: 100 * gib, MinKeepMinutes: 60, OverCap: true}, says: true},
		{name: "evicting from a partial measure", report: gocache.TrimReport{Cache: "engine-go-build", EndedBy: "budget", Phase: "measure", CapBytes: 30 * gib, EntriesRemoved: 9, MinKeepMinutes: 60, OverCap: true, Checkpoint: gocache.TrimCheckpoint{Shard: "40", LastName: "x-a", BytesSoFar: 30 * gib}}, says: true},
		{name: "within the cap", report: gocache.TrimReport{Cache: "engine-go-build", EndedBy: "complete", Phase: "idle", CapBytes: 30 * gib, BytesAfter: gib, MinKeepMinutes: 60}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if line := diskTrimLine(tc.report); strings.Contains(line, plain) != tc.says {
				t.Fatalf("line %q; want it to say %q: %v", line, plain, tc.says)
			}
		})
	}
}
