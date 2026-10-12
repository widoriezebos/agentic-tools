package plain

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestRefreshShadowUnchangedLaneWritesNothing(t *testing.T) {
	t.Parallel()
	for _, recoverObservation := range []bool{false, true} {
		name := "unchanged"
		if recoverObservation {
			name = "after-recovery"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newStatusBed(t)
			seams := shadowSeams()
			if changed, err := RefreshShadow(b.home, bedNow, "landing prove", seams); err != nil || len(changed) == 0 {
				t.Fatalf("first refresh: changed=%v error=%v", changed, err)
			}
			if recoverObservation {
				if err := state.NoteObservation(Dir(b.install), bedNow, []string{"queue unavailable"}); err != nil {
					t.Fatal(err)
				}
				if _, err := RefreshShadow(b.home, bedNow.Add(time.Minute), "keeper", seams); err != nil {
					t.Fatal(err)
				}
			}
			before := shadowTreeSnapshot(t, Dir(b.install))
			changed, err := RefreshShadow(b.home, bedNow.Add(2*time.Minute), "keeper", seams)
			if err != nil || len(changed) != 0 {
				t.Fatalf("unchanged refresh: changed=%v error=%v", changed, err)
			}
			after := shadowTreeSnapshot(t, Dir(b.install))
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("unchanged refresh wrote the lane: before=%v after=%v", before, after)
			}
		})
	}
}

type shadowFileSnapshot struct {
	Bytes   string
	ModTime time.Time
}

func shadowTreeSnapshot(t *testing.T, dir string) map[string]shadowFileSnapshot {
	t.Helper()
	result := make(map[string]shadowFileSnapshot)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		file := shadowFileSnapshot{ModTime: info.ModTime()}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			file.Bytes = string(data)
		}
		result[path] = file
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
