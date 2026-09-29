package steward

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// The machine pass carries the launch store's retention (Part B U5d): with
// the host's disk.launch-target-mib exceeded, an ended launch past
// disk.launch-keep-days goes and a young one stays, and the class line
// counts the store.
func TestMachinePassReleasesEndedLaunchesOverTheTarget(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	if err := os.WriteFile(filepath.Join(bed.inst, "metasystem.conf"), []byte("metasystem.template=true\ndisk.floor-gib=1\ndisk.launch-target-mib=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	launches := filepath.Join(bed.home, "launch")
	for id, finished := range map[string]string{"20260801t000000-aaaaaaaaaa": "2026-08-01T01:00:00Z", "20260928t000000-bbbbbbbbbb": "2026-09-28T01:00:00Z"} {
		dir := filepath.Join(launches, id)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		record, _ := json.Marshal(map[string]any{"id": id, "kind": "build", "state": "completed", "startedAt": finished, "finishedAt": finished})
		if err := os.WriteFile(filepath.Join(dir, "record.json"), record, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "exec.log"), []byte(strings.Repeat("x", 2<<20)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := SweepDiskStores(context.Background(), bed.inst, bed.pass(diskstore.ModeApply, []string{bed.inst}, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(launches, "20260801t000000-aaaaaaaaaa")); !os.IsNotExist(err) {
		t.Errorf("the ended launch past its window is released: %v; report %+v", err, result.Machine)
	}
	if _, err := os.Stat(filepath.Join(launches, "20260928t000000-bbbbbbbbbb")); err != nil {
		t.Errorf("the young launch stays: %v", err)
	}
	var counted bool
	for _, class := range result.Machine.Classes {
		counted = counted || class.Name == "launch store" && class.Items == 2 && class.Released == 1
	}
	if !counted {
		t.Errorf("the launch store's class line: %+v", result.Machine.Classes)
	}
}
