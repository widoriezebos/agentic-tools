package diskstore

import (
	"os"
	"path/filepath"
	"testing"
)

// Batch 28: a process scratch owner removes its own record at its normal end
// (process.go), so another process's Inventory can list a record file that
// is gone by the time it loads it. A record removed since the listing is not
// an unreadable record: it is simply absent, and no fail-closed caller holds
// on it (the unit read's findings-store preparation refused
// TestIntentReservedGoalNames with "no such store record" in the VM suite).
// A record that is there and cannot be read still reports unreadable.
func TestInventoryOmitsARecordRemovedSinceTheListing(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	registry := MachineRegistry(filepath.Join(root, "home"))
	if err := os.MkdirAll(registry.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	kept := Record{Schema: Schema, ID: "01M3RDRNB9WHTDZY24NN47AT7A", Path: filepath.Join(root, "tmp", "kept")}
	gone := Record{Schema: Schema, ID: "01M3RDRNB9WHTDZY24NN47AT7Z", Path: filepath.Join(root, "tmp", "gone")}
	for _, record := range []Record{kept, gone} {
		if err := registry.write(record); err != nil {
			t.Fatal(err)
		}
	}
	listThenRemove := func(dir string) ([]os.DirEntry, error) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		if err := os.Remove(registry.RecordPath(gone.ID)); err != nil {
			t.Fatal(err)
		}
		return entries, nil
	}
	records, unreadable := registry.inventory(listThenRemove)
	if len(unreadable) != 0 {
		t.Fatalf("a record removed since the listing is reported unreadable: %+v", unreadable)
	}
	if len(records) != 1 || records[0].ID != kept.ID {
		t.Fatalf("records = %+v, want only %s", records, kept.ID)
	}

	if err := os.WriteFile(registry.RecordPath(gone.ID), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, unreadable := registry.Inventory(); len(unreadable) != 1 || unreadable[0].Path != registry.RecordPath(gone.ID) {
		t.Fatalf("a present unreadable record is not reported: %+v", unreadable)
	}
}
