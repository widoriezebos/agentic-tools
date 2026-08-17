package janitor

import (
	"os"
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

// Only ENOENT ascends to a parent; every other failure to establish
// the requested path REFUSES — an ancestor's answer is not a
// measurement of the path that was asked about (the opus-window
// review's finding 1: permission-denied and overlong paths returned
// exit 0 while reporting the ancestor).
func TestHeadroomRefusesUnestablishablePath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission refusal is unobservable as root")
	}
	dir := t.TempDir()
	sealed := filepath.Join(dir, "sealed")
	if err := os.Mkdir(sealed, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sealed, 0o755) })
	if _, err := Headroom([]string{filepath.Join(sealed, "inside")}, 1); err == nil {
		t.Fatal("a permission-denied component must refuse, not measure the ancestor")
	}
	// A merely-absent tail still ascends and measures (the not-yet-
	// created target case the walk exists for).
	if _, err := Headroom([]string{filepath.Join(dir, "absent", "tail")}, 1); err != nil {
		t.Fatalf("an absent tail must still measure its existing ancestor: %v", err)
	}
}

// The ascent never cleans: an absent component followed by dot
// components must refuse, not lexically collapse into some directory
// the requested path never named (verification round 2 finding 3 —
// filepath.Dir internally Cleans, so 'absent/../x' ascended into '.').
func TestHeadroomRefusesDotComponentAscent(t *testing.T) {
	dir := t.TempDir()
	// Raw concatenation on purpose: filepath.Join CLEANS, which would
	// collapse the dot components before the guard ever saw them.
	cases := []string{
		dir + "/absent/../x",
		dir + "/absent/../.",
		"README.md/../.",
	}
	for _, path := range cases {
		if _, err := Headroom([]string{path}, 1); err == nil {
			t.Fatalf("%s must refuse, not measure a cleaned ancestor", path)
		}
	}
}

// rawParent's own table: strip-one-component semantics without Clean.
func TestRawParent(t *testing.T) {
	cases := []struct {
		in     string
		parent string
		ok     bool
	}{
		{"/a/b/c", "/a/b", true},
		{"/a/b/", "/a", true},
		{"/a", "/", true},
		{"/", "", false},
		{"", "", false},
		{"rel", "", false},
		{"/a/..", "", false},
		{"/a/.", "", false},
		{"/a/../b", "", false},
	}
	for _, tc := range cases {
		parent, ok := rawParent(tc.in)
		if ok != tc.ok || (ok && parent != tc.parent) {
			t.Fatalf("rawParent(%q) = (%q, %v), want (%q, %v)", tc.in, parent, ok, tc.parent, tc.ok)
		}
	}
}
