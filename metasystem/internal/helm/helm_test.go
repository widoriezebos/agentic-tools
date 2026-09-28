package helm

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

var since = time.Date(2026, 9, 28, 19, 14, 3, 0, time.UTC)

// fakeSeat lays a checkout out as git does, with no subprocess: the primary,
// then a linked and a job-style worktree whose .git files name
// worktrees/NAME with a relative commondir. The last root is the installation.
func fakeSeat(t *testing.T) []string {
	base := t.TempDir()
	roots := []string{filepath.Join(base, "checkout")}
	must(t, os.MkdirAll(filepath.Join(roots[0], ".git"), 0o755), os.MkdirAll(filepath.Join(roots[0], "metasystem"), 0o755))
	for _, name := range []string{"linked", "job-j1"} {
		gitdir, tree := filepath.Join(roots[0], ".git", "worktrees", name), filepath.Join(base, "trees", name)
		must(t, os.MkdirAll(gitdir, 0o755), os.MkdirAll(tree, 0o755), os.WriteFile(filepath.Join(gitdir, "commondir"), []byte("../..\n"), 0o644),
			os.WriteFile(filepath.Join(tree, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644))
		roots = append(roots, tree)
	}
	return append(roots, filepath.Join(roots[0], "metasystem"))
}

func must(t *testing.T, errs ...error) {
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func take(t *testing.T, root string, at time.Time) Seat {
	seat, err := Write(root, Record{By: "Wido", At: at.Format(time.RFC3339), Reason: "by hand"})
	must(t, err)
	return seat
}

func TestActiveSameAcrossLinkedWorktrees(t *testing.T) {
	t.Parallel()
	t.Run("HM-4", func(t *testing.T) {
		roots := fakeSeat(t)
		seat := take(t, roots[1], since)
		for _, root := range roots {
			if state := Active(root); !state.Active || state.By != "Wido" || !state.Since.Equal(since) {
				t.Fatalf("Active(%s) = %+v, want Wido since %s", root, state, since)
			}
		}
		if info, err := os.Stat(filepath.Join(roots[0], ".git", "metasystem", "helm.json")); err != nil || info.Mode().Perm() != 0o600 || filepath.Base(seat.Checkout) != "checkout" {
			t.Fatalf("signature not in the common dir with mode 0600 (%v), checkout %s", err, seat.Checkout)
		}
	})
}

func TestActiveFromInstallationPath(t *testing.T) {
	t.Parallel()
	t.Run("HM-3", func(t *testing.T) {
		roots := fakeSeat(t)
		if take(t, roots[0], since); !Active(roots[3]).Active {
			t.Fatalf("installation path reads %+v", Active(roots[3]))
		}
	})
}

func TestActiveFalseInOtherCheckout(t *testing.T) {
	t.Parallel()
	t.Run("HM-4", func(t *testing.T) {
		if take(t, fakeSeat(t)[0], since); Active(fakeSeat(t)[2]).Active {
			t.Fatal("another checkout reads the helm")
		}
	})
}

func TestActiveOutsideRepositoryIsFalse(t *testing.T) {
	t.Parallel()
	t.Run("HM-3", func(t *testing.T) {
		if state := Active(t.TempDir()); state.Active || state.Diagnostic == "" {
			t.Fatalf("outside a repository: %+v, want inactive with a diagnostic", state)
		}
	})
}

func TestActiveMalformedIsActive(t *testing.T) {
	t.Parallel()
	t.Run("HM-3", func(t *testing.T) {
		roots := fakeSeat(t)
		seat := take(t, roots[0], since)
		for _, body := range []string{`{"schema":1,"by":"Wi`, `{"schema":2,"by":"Wido","at":"2026-09-28T19:14:03Z"}`, ""} {
			must(t, os.WriteFile(seat.Signature, []byte(body), 0o600))
			if state := Active(roots[2]); !state.Active || state.Malformed == "" || state.By != "unknown" {
				t.Fatalf("malformed %q reads %+v, want active and malformed", body, state)
			}
		}
	})
}

func TestActiveNeverErrors(t *testing.T) {
	t.Parallel()
	t.Run("HM-3", func(t *testing.T) {
		roots := fakeSeat(t)
		must(t, os.MkdirAll(filepath.Join(roots[0], ".git", "metasystem", "helm.json"), 0o700), os.WriteFile(filepath.Join(roots[1], ".git"), []byte("x"), 0o644))
		if state := Active(roots[0]); !state.Active || state.Malformed == "" {
			t.Fatalf("a directory at the signature reads %+v, want active and unreadable", state)
		}
		if state := Active(roots[1]); state.Active || state.Diagnostic == "" {
			t.Fatalf("a broken .git file reads %+v, want inactive with a diagnostic", state)
		}
		_, _ = Active(""), Active("\x00")
	})
}

// TestActiveIgnoresGitSteering is serial: it sets the process environment.
func TestActiveIgnoresGitSteering(t *testing.T) {
	t.Run("HM-3", func(t *testing.T) {
		roots, other := fakeSeat(t), fakeSeat(t)[0]
		take(t, other, since)
		t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
		t.Setenv("GIT_COMMON_DIR", filepath.Join(other, ".git"))
		if state := Active(roots[1]); state.Active {
			t.Fatalf("steering variables leaked another seat's helm: %+v", state)
		}
	})
}

func TestActiveAfterSimulatedRestart(t *testing.T) {
	t.Parallel()
	t.Run("HM-10", func(t *testing.T) {
		root := fakeSeat(t)[0]
		take(t, root, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
		must(t, os.Rename(root, root+"-moved"), os.Rename(root+"-moved", root))
		if state := Active(root); !state.Active || state.By != "Wido" {
			t.Fatalf("a years-old helm after a restart reads %+v", state)
		}
	})
}
