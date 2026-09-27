package launch

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// secondSessionBed is a primary checkout with a nested harness, git stubbed:
// the toplevel query answers the checkout and `worktree add` lays the new
// worktree's harness down, so the real isolation audit runs on real files.
type secondSessionBed struct {
	checkout, harness string
	gitCalls          [][]string
	armed             []string
	armedHarness      string
	options           SecondSessionOptions
}

func newSecondSessionBed(t *testing.T) *secondSessionBed {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bed := &secondSessionBed{checkout: filepath.Join(parent, "source")}
	bed.harness = filepath.Join(bed.checkout, "metasystem")
	for _, dir := range []string{filepath.Join(bed.harness, "scripts", "agents"), filepath.Join(bed.checkout, ".claude")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bed.checkout, ".claude", "settings.local.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.options = SecondSessionOptions{
		HarnessRoot: bed.harness, Pid: 4242,
		Git: func(args ...string) (string, error) {
			bed.gitCalls = append(bed.gitCalls, args)
			switch {
			case len(args) > 2 && args[2] == "rev-parse":
				return bed.checkout + "\n", nil
			case len(args) > 3 && args[2] == "worktree" && args[3] == "add":
				return "", os.MkdirAll(filepath.Join(args[7], "metasystem"), 0o755)
			}
			return "", errors.New("unexpected git call")
		},
		StartedAt: func(pid int64) (int64, error) { return 1786104000, nil },
		Token:     func() (string, error) { return "beef", nil },
		Now:       func() time.Time { return time.Date(2026, 9, 27, 1, 2, 3, 0, time.UTC) },
		ArmSupervision: func(newHarness string, args []string) error {
			bed.armedHarness, bed.armed = newHarness, args
			return nil
		},
	}
	return bed
}

// Ported from second-session-fixtures.sh human-shell-bootstrap (WC-8): the
// worktree is created on its own branch beside the checkout, the adapters'
// local configuration is copied into it, and the new harness is armed for the
// new checkout with this process's identity and recorded start time.
func TestSecondSessionCreatesAnIsolatedArmedWorktree(t *testing.T) {
	t.Parallel()
	bed := newSecondSessionBed(t)
	bed.options.Name = "human-session"
	destination, err := SecondSession(bed.options)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(bed.checkout), "human-session")
	if destination != want {
		t.Fatalf("destination = %s, want %s", destination, want)
	}
	wantAdd := []string{"-C", bed.checkout, "worktree", "add", "-q", "-b", "session/human-session", want, "HEAD"}
	if len(bed.gitCalls) != 2 || !reflect.DeepEqual(bed.gitCalls[1], wantAdd) {
		t.Fatalf("git calls = %q", bed.gitCalls)
	}
	if data, err := os.ReadFile(filepath.Join(want, ".claude", "settings.local.json")); err != nil || string(data) != "{}\n" {
		t.Fatalf("local configuration was not copied: %q %v", data, err)
	}
	if bed.armedHarness != filepath.Join(want, "metasystem") {
		t.Fatalf("armed harness = %s", bed.armedHarness)
	}
	wantArm := []string{"--repo", want, "--session", "second-session-bootstrap-human-session-4242", "--pid", "4242",
		"--start-time", "1786104000", "--tag", "metasystem-main-bootstrap-human-session-4242"}
	if !reflect.DeepEqual(bed.armed, wantArm) {
		t.Fatalf("arming = %q\nwant %q", bed.armed, wantArm)
	}
}

func TestSecondSessionMintsANameAndRefusesUnlawfulOnes(t *testing.T) {
	t.Parallel()
	bed := newSecondSessionBed(t)
	destination, err := SecondSession(bed.options)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(destination) != "source-session-20260927t010203z-beef" {
		t.Fatalf("minted name = %s", filepath.Base(destination))
	}

	for _, name := range []string{"bad name", "../escape", "a/b"} {
		bed := newSecondSessionBed(t)
		bed.options.Name = name
		_, err := SecondSession(bed.options)
		var refusal *SecondSessionError
		if !errors.As(err, &refusal) || refusal.Code != 2 || !strings.Contains(refusal.Detail, "only letters, numbers") {
			t.Fatalf("name %q: %v", name, err)
		}
		if len(bed.gitCalls) != 1 || bed.armed != nil {
			t.Fatalf("name %q reached the worktree: %q", name, bed.gitCalls)
		}
	}

	bed = newSecondSessionBed(t)
	bed.options.Name = "taken"
	if err := os.Mkdir(filepath.Join(filepath.Dir(bed.checkout), "taken"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = SecondSession(bed.options)
	var refusal *SecondSessionError
	if !errors.As(err, &refusal) || refusal.Code != 1 || !strings.Contains(refusal.Detail, "destination already exists") {
		t.Fatalf("existing destination: %v", err)
	}
}

func TestSecondSessionStopsWhenArmingFails(t *testing.T) {
	t.Parallel()
	bed := newSecondSessionBed(t)
	bed.options.Name = "unarmed"
	bed.options.ArmSupervision = func(string, []string) error { return errors.New("up refused") }
	if _, err := SecondSession(bed.options); err == nil || !strings.Contains(err.Error(), "up refused") {
		t.Fatalf("arming failure: %v", err)
	}
}
