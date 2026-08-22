package adapter

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// The naming contract, pinned behaviorally — by generated names, never
// by grepping this package's source, which would fail on any reflow
// with zero behavior change. The grammar the readers
// depend on is <runtime>-<version>-<configHash>-<yyyymmdd>-<seq %03d>.json,
// and only generated names can prove it.
func TestSnapshotNameGrammar(t *testing.T) {
	restore := now
	now = func() time.Time { return time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC) }
	defer func() { now = restore }()
	dir := t.TempDir()
	grammar := regexp.MustCompile(`^fake-1\.2\.3-abcd1234-20260814-\d{3}\.json$`)
	for want := 1; want <= 2; want++ {
		path, err := WriteCapabilitySnapshot(dir, "fake", "1.2.3", "abcd1234",
			`["cli"]`, `{}`, `{}`, `{"writeRoots":"mapped","readRoots":"mapped","network":"mapped"}`, `{}`)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		if !grammar.MatchString(name) {
			t.Fatalf("generated name %q breaks the grammar", name)
		}
		if name[len(name)-8:] != "00"+string(rune('0'+want))+".json" {
			t.Fatalf("sequence did not advance: %q at write %d", name, want)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}
