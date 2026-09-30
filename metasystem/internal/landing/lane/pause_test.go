package lane

import (
	"os"
	"path/filepath"
	"testing"
)

// The pause fails closed (design r10 K2, r6 P1): a pause record that
// cannot be read, whatever the reason, reads as paused, and only a clear
// removes it.
func TestCorruptPauseReadsPaused(t *testing.T) {
	t.Parallel()
	for name, write := range map[string]func(path string) error{
		"garbage":   func(path string) error { return os.WriteFile(path, []byte("{not json"), 0o600) },
		"empty":     func(path string) error { return os.WriteFile(path, nil, 0o600) },
		"directory": func(path string) error { return os.Mkdir(path, 0o700) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			if err := os.MkdirAll(HostDir(home), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := write(pausePath(home)); err != nil {
				t.Fatal(err)
			}
			if _, paused := ReadPause(home); !paused {
				t.Fatalf("an unreadable pause record (%s) reads as not paused", name)
			}
			if changed, err := SetPause(home, "Wido", laneNow); err != nil || changed {
				t.Fatalf("SetPause over an unreadable pause = %v %v; want already paused", changed, err)
			}
			if name == "directory" {
				return
			}
			if changed, err := ClearPause(home); err != nil || !changed {
				t.Fatalf("ClearPause over an unreadable pause = %v %v; want it cleared", changed, err)
			}
			if _, paused := ReadPause(home); paused {
				t.Fatalf("cleared pause still reads paused")
			}
		})
	}
	if _, paused := ReadPause(filepath.Join(t.TempDir(), "no-home")); paused {
		t.Fatalf("no pause record reads as paused")
	}
}
