package plain

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLaunchResolutionRefusesAnUnreadableRecord(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	fix := &Fix{Attempt: "resolution"}
	path := filepath.Join(Dir(b.install), "fixes", fix.Attempt+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"attempt":"resolution","job":"j1","units":"x"}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	existing, starts := 0, 0
	err := LaunchResolution(b.install, fix, bedNow, LaunchRecord{By: "landing"}, func() (string, bool, error) {
		existing++
		return "", false, nil
	}, func() (string, error) {
		starts++
		return "j2", nil
	})
	if err == nil || existing != 0 || starts != 0 {
		t.Fatalf("unreadable record: error=%v, lookups=%d, starts=%d", err, existing, starts)
	}
	saved, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(saved, data) {
		t.Fatalf("unreadable record changed: %q: %v", saved, err)
	}
}
