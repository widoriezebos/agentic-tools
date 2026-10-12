package lane

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestResumeRecordsClockAndFailureClearsPause(t *testing.T) {
	t.Parallel()
	for _, blocked := range []bool{false, true} {
		t.Run(map[bool]string{false: "resumed", true: "unwritable record"}[blocked], func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			if changed, err := ClearPauseAt(home, "person", laneNow); err != nil || changed {
				t.Fatalf("absent pause: %v %v", changed, err)
			}
			if _, exists, err := ReadResume(home); err != nil || exists {
				t.Fatalf("no-op created resume: %v %v", exists, err)
			}
			if _, err := SetPause(home, "person", laneNow); err != nil {
				t.Fatal(err)
			}
			if blocked {
				path := filepath.Join(HostDir(home), "landing-lane-resumed.json")
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			now := laneNow.Add(time.Minute + 123*time.Nanosecond).In(time.FixedZone("fixture", 2*60*60))
			changed, err := ClearPauseAt(home, "the person", now)
			if blocked {
				var finding *ResumeRecordError
				if !errors.As(err, &finding) || !changed {
					t.Fatalf("resume lost its finding or did not clear pause: %v %v", changed, err)
				}
				if _, paused := ReadPause(home); paused {
					t.Fatal("resume-record failure kept pause")
				}
				return
			}
			if err != nil || !changed {
				t.Fatalf("resume: %v %v", changed, err)
			}
			resume, exists, err := ReadResume(home)
			if err != nil || !exists || resume.At != now.UTC().Format(time.RFC3339Nano) || resume.By != "the person" {
				t.Fatalf("resume record: %+v %v %v", resume, exists, err)
			}
			if _, paused := ReadPause(home); paused {
				t.Fatal("resumed lane stays paused")
			}
			if changed, err := ClearPauseAt(home, "another person", now.Add(time.Minute)); err != nil || changed {
				t.Fatalf("repeat resume: %v %v", changed, err)
			}
			again, _, _ := ReadResume(home)
			if again != resume {
				t.Fatalf("repeat rewrote resume: %+v", again)
			}
		})
	}
}

func TestResumeUnconfirmedRecordClearsPause(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if _, err := SetPause(home, "person", laneNow); err != nil {
		t.Fatal(err)
	}
	doubtWrites(t, home)
	changed, err := ClearPauseAt(home, "person", laneNow.Add(time.Minute))
	var finding *ResumeRecordError
	if !changed || !errors.As(err, &finding) || !errors.Is(err, ErrNotDurable) {
		t.Fatalf("resume: %v %v", changed, err)
	}
	if _, paused := ReadPause(home); paused {
		t.Fatal("unconfirmed resume kept pause")
	}
	if resume, present, readErr := ReadResume(home); readErr != nil || !present || resume.By != "person" {
		t.Fatalf("written resume: %+v %v %v", resume, present, readErr)
	}
}

func TestResumeReadOnlyHostAfterPauseRemovalIsFinding(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if _, err := SetPause(home, "person", laneNow); err != nil {
		t.Fatal(err)
	}
	d := doubtWrites(t, home)
	d.beforeWrite = func(path string) {
		if filepath.Base(path) != "landing-lane-resumed.json" {
			return
		}
		if _, paused := ReadPause(home); paused {
			t.Error("resume timestamp was attempted before clearing pause")
		}
		if err := os.Chmod(HostDir(home), 0500); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = os.Chmod(HostDir(home), 0700) })
	changed, err := ClearPauseAt(home, "person", laneNow.Add(time.Minute))
	var finding *ResumeRecordError
	if !changed || !errors.As(err, &finding) || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("read-only resume: %v %v", changed, err)
	}
	if _, paused := ReadPause(home); paused {
		t.Fatal("read-only shadow input restored pause")
	}
}

func TestResumeRecordRemovedWithLane(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if _, err := SetPause(home, "person", laneNow); err != nil {
		t.Fatal(err)
	}
	if _, err := ClearPauseAt(home, "person", laneNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := removeLane(home); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := ReadResume(home); err != nil || exists {
		t.Fatalf("removed lane left resume record: %v %v", exists, err)
	}
}
