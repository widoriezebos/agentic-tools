package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
// every builder by hand also uses, is trimmed by last use to its cap, and
// an entry used inside the keep window stays whatever the cap.
func TestDiskCleanGoCacheTrimsTheMachineCachesNow(t *testing.T) {
	t.Parallel()
	bed := newDiskCleanBed(t)
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.runtimes=claude\ndisk.go-cache-cap-gib=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const gib = int64(1) << 30
	old := bed.entry(t, "go-build", "00/old-d", 2*gib, 3*24*time.Hour)
	recent := bed.entry(t, "go-build", "01/recent-d", 2*gib, time.Hour)
	delegate := bed.entry(t, "metasystem-delegate-go-build", "02/dele-d", 11*gib, 2*24*time.Hour)
	code, result, printed := bed.run(t, "--go-cache")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("disk clean = %d %s", code, printed)
	}
	for _, path := range []string{old, delegate} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s was not trimmed: %v", path, err)
		}
	}
	if _, err := os.Stat(recent); err != nil {
		t.Errorf("an entry inside the keep window went: %v", err)
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

func init() {
	registerIdempotency("disk clean", idemStateful, "every cache already within its cap: success, nothing removed; only the report's timestamp moves", witnessDiskCleanRepeat)
}

// witnessDiskCleanRepeat runs disk clean twice on caches within their caps:
// both succeed, and the caches are byte for byte what they were.
func witnessDiskCleanRepeat(t *testing.T) {
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
