package janitor

import (
	"path/filepath"
	"testing"
)

// The guard measures a real filesystem, dedups paths on the same
// device, and refuses an unmeasurable path rather than passing it.
func TestHeadroom(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b", "deep", "not-created-yet")
	results, err := Headroom([]string{a, b}, 1)
	if err != nil {
		t.Fatal(err)
	}
	// a and b are on the same tmpdir filesystem → one result.
	if len(results) != 1 {
		t.Fatalf("same-device paths must dedup: %d results", len(results))
	}
	if results[0].FreeBytes <= 0 {
		t.Fatalf("a real filesystem reports positive free bytes: %d", results[0].FreeBytes)
	}
	if results[0].BelowFloor() || results[0].Deficit() != 0 {
		t.Fatal("a 1-byte floor must be met by any real filesystem")
	}

	// An impossible floor trips the below-floor signal with a deficit.
	high, err := Headroom([]string{dir}, 1<<62)
	if err != nil {
		t.Fatal(err)
	}
	if !high[0].BelowFloor() || high[0].Deficit() <= 0 {
		t.Fatalf("an impossible floor must report below with a deficit: %+v", high[0])
	}
}
