package proofrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// Heap measurements belong to a single test process.
// The parent remains parallel with the package's other tests.
func isolatedRetentionTest(t *testing.T) bool {
	t.Helper()
	const child = "METASYSTEM_RETENTION_MEASUREMENT_CHILD"
	if os.Getenv(child) == t.Name() {
		return true
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v", "-test.timeout=30m")
	command.Env = append(os.Environ(), child+"="+t.Name())
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	return false
}

// Each record has a megabyte of payload after its retention facts. No payload
// object escapes this helper into the heap measurement's baseline.
func largeRetentionBed(t *testing.T) *retentionBed {
	t.Helper()
	b := newRetentionBed(t)
	payload := json.RawMessage(`"` + strings.Repeat("x", 1<<20) + `"`)
	for i := 0; i < 300; i++ {
		id := fmt.Sprintf("proof-large-%016x", i)
		b.attempt(id, 60*retentionDay, true, func(a *Attempt) { a.DeliveryReceipt = payload })
		b.write(b.path(id+".json"), Record{Suite: "testing", AttemptID: id})
	}
	return b
}

func TestAttemptRetentionAtOrBelowTargetReadsNoRecords(t *testing.T) {
	t.Parallel()
	if !isolatedRetentionTest(t) {
		return
	}
	b := largeRetentionBed(t)
	for _, path := range []string{
		b.path("attempts", "proof-large-0000000000000000.json"),
		b.path("processes", "proof-large-0000000000000000-launch-1.json"),
		b.path("proof-large-0000000000000000.json"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '{'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var reads int
	readRecord := func(path string) ([]byte, error) {
		reads++
		return os.ReadFile(path)
	}
	measured, _, complete := diskstore.Measure(t.Context(), b.path())
	if !complete {
		t.Fatal("fixture measurement incomplete")
	}
	for _, extra := range []int64{0, 1 << 20} {
		retention := b.retention(measured + extra)
		retention.readRecord = readRecord
		items, err := retention.Plan(t.Context(), &diskstore.Pass{Now: retentionNow})
		if err != nil || len(items) != 0 || reads != 0 {
			t.Fatalf("target extra %d: items=%+v err=%v reads=%d", extra, items, err, reads)
		}
		if count, bytes := retention.Totals(); count != 300 || bytes != measured {
			t.Fatalf("totals=(%d, %d), want (300, %d)", count, bytes, measured)
		}
		t.Logf("target extra %d: 300 attempts, %d bytes, zero record reads", extra, measured)
	}
}

func TestAttemptRetentionOverTargetRetainsOnlyFacts(t *testing.T) {
	t.Parallel()
	if !isolatedRetentionTest(t) {
		return
	}
	b := largeRetentionBed(t)
	retention := b.retention(1)
	reads := map[string]int{}
	retention.readRecord = func(path string) ([]byte, error) {
		reads[filepath.Dir(path)]++
		return os.ReadFile(path)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	items, err := retention.Plan(context.Background(), &diskstore.Pass{Now: retentionNow})
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	delta := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	runtime.KeepAlive(retention)
	if delta >= 16<<20 {
		t.Fatalf("retained heap grew by %d bytes, want under 16 MiB", delta)
	}
	if len(retention.units) != 300 {
		t.Fatalf("retained %d units, want 300", len(retention.units))
	}
	for _, dir := range []string{b.path("attempts"), b.path("processes"), b.path()} {
		if reads[dir] < 300 {
			t.Errorf("read seam saw %d records in %s, want at least 300", reads[dir], dir)
		}
	}
	var releases int
	for _, item := range items {
		if item.Verdict.Decision == diskstore.Release {
			releases++
		}
	}
	if releases != 300 {
		t.Fatalf("releases=%d, want 300: %+v", releases, items)
	}
	t.Logf("300 decoded attempts: retained heap delta %d bytes", delta)
}

func TestAttemptRetentionFreshnessBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		expiry  string
		release bool
	}{
		{"", true}, {"invalid", false},
		{retentionNow.Add(retentionDay).Format(time.RFC3339Nano), false},
		{retentionNow.Format(time.RFC3339Nano), true},
		{retentionNow.Add(-retentionDay).Format(time.RFC3339Nano), true},
	} {
		b := newRetentionBed(t)
		id := "proof-fresh-0000000000000001"
		b.attempt(id, 70*retentionDay, true, func(a *Attempt) { a.FreshnessExpiresAt = tc.expiry })
		items, err := b.retention(1).Plan(t.Context(), &diskstore.Pass{Now: retentionNow})
		if err != nil {
			t.Fatal(err)
		}
		var released bool
		for _, item := range items {
			released = released || item.Key == id && item.Verdict.Decision == diskstore.Release
		}
		if released != tc.release {
			t.Errorf("expiry %q: released=%v, want %v", tc.expiry, released, tc.release)
		}
	}
}
