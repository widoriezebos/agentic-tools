package gocache_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

// One machine pass trims the four machine caches, each to its own cap:
// the engine's Go cache (Go's default, which every builder by hand also
// uses), the engine's staticcheck cache, and the delegate pair. It never
// reads GOCACHE.
func TestTrimMachineTrimsTheFourCachesEachToItsCap(t *testing.T) {
	t.Parallel()
	userCache := t.TempDir()
	state := filepath.Join(t.TempDir(), "cache-trim")
	old := trimNow.Add(-5 * 24 * time.Hour)
	for _, dir := range []string{"go-build", "staticcheck", "metasystem-delegate-go-build", "metasystem-delegate-staticcheck"} {
		for index, name := range []string{"00/aaaa-d", "01/bbbb-d"} {
			path := filepath.Join(userCache, dir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, make([]byte, 100), 0o644); err != nil {
				t.Fatal(err)
			}
			when := old.Add(time.Duration(index) * time.Hour)
			_ = os.Chtimes(path, when, when)
		}
	}
	reports, err := gocache.TrimMachine(context.Background(), gocache.MachineTrim{
		UserCacheDir: func() (string, error) { return userCache, nil }, StateDir: state,
		EngineGoCapBytes: 150, DelegateGoCapBytes: 250, StaticcheckCapBytes: 150,
		Keep: 12 * time.Hour, Now: trimNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int64{
		"engine-go-build": {200, 100}, "engine-staticcheck": {200, 100},
		"delegate-go-build": {200, 200}, "delegate-staticcheck": {200, 100},
	}
	if len(reports) != 4 {
		t.Fatalf("reports = %+v", reports)
	}
	for _, report := range reports {
		if got := [2]int64{report.BytesBefore, report.BytesAfter}; got != want[report.Cache] || report.EndedBy != "complete" {
			t.Errorf("%s: before/after %v ended by %s, want %v", report.Cache, got, report.EndedBy, want[report.Cache])
		}
		if _, err := os.Stat(filepath.Join(state, report.Cache+".json")); err != nil {
			t.Errorf("%s: no persisted report: %v", report.Cache, err)
		}
	}
	if _, err := os.Stat(filepath.Join(userCache, "go-build", "01", "bbbb-d")); err != nil {
		t.Fatalf("the newer engine entry went: %v", err)
	}

	// A cancelled machine pass stops at once: every cache reports it.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reports, err = gocache.TrimMachine(ctx, gocache.MachineTrim{
		UserCacheDir: func() (string, error) { return userCache, nil }, StateDir: state,
		EngineGoCapBytes: 1, DelegateGoCapBytes: 1, StaticcheckCapBytes: 1, Keep: 12 * time.Hour, Now: trimNow,
	})
	if err != nil || len(reports) != 4 {
		t.Fatalf("cancelled: %+v %v", reports, err)
	}
	for _, report := range reports {
		if report.EndedBy != "cancelled" {
			t.Errorf("%s ended by %s", report.Cache, report.EndedBy)
		}
	}
}
