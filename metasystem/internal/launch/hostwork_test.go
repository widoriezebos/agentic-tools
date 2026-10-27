package launch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostWorkCountsBuildsAndChecks(t *testing.T) {
	t.Parallel()
	m, _, _, _ := manager(t)
	seed(t, m, "build", Running)
	for _, state := range []State{Running, Completed} {
		id := "check-" + string(state)
		seed(t, m, id, state)
		if _, err := m.Store.Update(id, func(r *Record) error { r.Kind = "proof"; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() map[string]string {
		t.Helper()
		files := map[string]string{}
		if err := filepath.WalkDir(m.Store.Root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				files[path] = string(data)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := snapshot()
	builds, checks, err := m.HostWork()
	if err != nil || builds != 1 || checks != 1 {
		t.Fatalf("host work=%d,%d err=%v", builds, checks, err)
	}
	capacity, err := m.CapacityBuilds()
	if err != nil || len(capacity) != 1 || capacity[0].ID != "build" {
		t.Fatalf("capacity changed: %+v %v", capacity, err)
	}
	after := snapshot()
	if len(before) != len(after) {
		t.Fatal("reader changed record files")
	}
	for path, bytes := range before {
		if after[path] != bytes {
			t.Fatalf("reader changed %s", path)
		}
	}
	m.Store.Root = filepath.Join(t.TempDir(), "missing")
	if _, _, err := m.HostWork(); err == nil {
		t.Fatal("missing store read as empty")
	}
	if capacity, err := m.CapacityBuilds(); err != nil || capacity != nil {
		t.Fatalf("missing capacity behavior changed: %+v %v", capacity, err)
	}
}
