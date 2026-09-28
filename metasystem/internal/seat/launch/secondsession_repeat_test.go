package launch

// A repeated named second session whose worktree already exists (R-129-ui),
// and the refusals on the way to a new one.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withWorktreeList answers `git worktree list --porcelain` with listed (or
// fails it with listErr) and leaves every other call to the bed's stub.
func withWorktreeList(bed *secondSessionBed, listed string, listErr error) {
	base := bed.options.Git
	bed.options.Git = func(args ...string) (string, error) {
		if len(args) > 3 && args[2] == "worktree" && args[3] == "list" {
			bed.gitCalls = append(bed.gitCalls, args)
			return listed, listErr
		}
		return base(args...)
	}
}

func TestSecondSessionRepeatAnswersTheExistingIsolatedWorktree(t *testing.T) {
	t.Parallel()
	bed := newSecondSessionBed(t)
	bed.options.Name = "human-session"
	destination := filepath.Join(filepath.Dir(bed.checkout), "human-session")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	withWorktreeList(bed, "worktree "+bed.checkout+"\nHEAD abc\nbranch refs/heads/main\n\n"+
		"worktree "+destination+"\nHEAD def\nbranch refs/heads/session/human-session\n", nil)

	path, err := SecondSession(bed.options)
	var isolated *SecondSessionIsolated
	if !errors.As(err, &isolated) {
		t.Fatalf("repeat error = %v, want SecondSessionIsolated", err)
	}
	if path != destination || isolated.Path != destination || isolated.Branch != "session/human-session" {
		t.Fatalf("repeat = %q, %+v", path, isolated)
	}
	want := "second-session: " + destination + " is already this checkout's isolated worktree on session/human-session"
	if isolated.Error() != want {
		t.Fatalf("message = %q, want %q", isolated.Error(), want)
	}
	for _, call := range bed.gitCalls {
		if len(call) > 3 && call[3] == "add" {
			t.Fatalf("a repeat created a worktree: %q", bed.gitCalls)
		}
	}
	if bed.armed != nil {
		t.Fatalf("a repeat armed supervision again: %q", bed.armed)
	}
}

func TestSecondSessionExistingDestinationThatIsNotTheSessionWorktreeIsRefused(t *testing.T) {
	t.Parallel()
	cases := map[string]func(destination string) (string, error){
		"list fails": func(string) (string, error) { return "", errors.New("git exploded") },
		"not listed": func(string) (string, error) {
			return "worktree /elsewhere\nbranch refs/heads/session/human-session\n", nil
		},
		"listed on another branch": func(destination string) (string, error) {
			return "worktree " + destination + "\nbranch refs/heads/feature\n", nil
		},
		"listed detached": func(destination string) (string, error) {
			return "worktree " + destination + "\nHEAD abc\ndetached\n", nil
		},
	}
	for name, list := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newSecondSessionBed(t)
			bed.options.Name = "human-session"
			destination := filepath.Join(filepath.Dir(bed.checkout), "human-session")
			if err := os.Mkdir(destination, 0o755); err != nil {
				t.Fatal(err)
			}
			listed, listErr := list(destination)
			withWorktreeList(bed, listed, listErr)

			path, err := SecondSession(bed.options)
			var refusal *SecondSessionError
			if !errors.As(err, &refusal) || refusal.Code != 1 || path != "" {
				t.Fatalf("SecondSession = %q, %v", path, err)
			}
			if refusal.Error() != "second-session destination already exists: "+destination {
				t.Fatalf("message = %q", refusal.Error())
			}
		})
	}
}

// A minted name is never a repeat: the existing destination is refused
// without asking git which worktrees it has.
func TestSecondSessionMintedNameCollisionIsNotARepeat(t *testing.T) {
	t.Parallel()
	bed := newSecondSessionBed(t)
	if err := os.Mkdir(filepath.Join(filepath.Dir(bed.checkout), "source-session-20260927t010203z-beef"), 0o755); err != nil {
		t.Fatal(err)
	}
	withWorktreeList(bed, "", errors.New("must not be asked"))
	_, err := SecondSession(bed.options)
	var refusal *SecondSessionError
	if !errors.As(err, &refusal) || refusal.Code != 1 {
		t.Fatalf("minted collision = %v", err)
	}
	for _, call := range bed.gitCalls {
		if len(call) > 3 && call[3] == "list" {
			t.Fatalf("a minted name asked for the worktree list: %q", bed.gitCalls)
		}
	}
}

func TestSecondSessionCarriesEachSeamsFailure(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		change func(bed *secondSessionBed)
		want   string
	}{
		"not a checkout": {func(bed *secondSessionBed) {
			bed.options.Git = func(...string) (string, error) { return "", errors.New("not a git repository") }
		}, "is not inside a git checkout"},
		"token": {func(bed *secondSessionBed) {
			bed.options.Name = ""
			bed.options.Token = func() (string, error) { return "", errors.New("no entropy") }
		}, "no entropy"},
		"worktree add": {func(bed *secondSessionBed) {
			base := bed.options.Git
			bed.options.Git = func(args ...string) (string, error) {
				if len(args) > 3 && args[3] == "add" {
					return "", errors.New("branch exists")
				}
				return base(args...)
			}
		}, "git worktree add failed: branch exists"},
		"isolation": {func(bed *secondSessionBed) {
			bed.options.Isolate = func(string, string, string, string) (string, error) {
				return "", errors.New("isolation audit refused")
			}
		}, "isolation audit refused"},
		"start time": {func(bed *secondSessionBed) {
			bed.options.Isolate = func(_, destination, _, _ string) (string, error) {
				return filepath.Join(destination, "metasystem"), nil
			}
			bed.options.StartedAt = func(int64) (int64, error) { return 0, errors.New("no such process") }
		}, "start time is unreadable: no such process"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newSecondSessionBed(t)
			bed.options.Name = "session-" + strings.ReplaceAll(name, " ", "-")
			test.change(bed)
			path, err := SecondSession(bed.options)
			if err == nil || !strings.Contains(err.Error(), test.want) || path != "" {
				t.Fatalf("SecondSession = %q, %v; want %q", path, err, test.want)
			}
			if bed.armed != nil {
				t.Fatalf("armed after a failure: %q", bed.armed)
			}
		})
	}
}
