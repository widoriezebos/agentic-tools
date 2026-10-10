package plain

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestKeeperFetchesMissingHandInBeforeReadingIt(t *testing.T) {
	t.Parallel()
	for _, onOrigin := range []bool{true, false} {
		t.Run(map[bool]string{true: "only on origin", false: "nowhere"}[onOrigin], func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("proof.trunk-every=1h\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := HandIn(install, Line{Goal: "next", SHA: "tip"}); err != nil {
				t.Fatal(err)
			}
			fetched, fetches := false, 0
			notAncestor := exec.Command("/usr/bin/false").Run()
			seams := ProveSeams{Now: func() time.Time { return bedNow }, Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }, Git: func(_ string, args ...string) (string, error) {
				switch args[0] {
				case "fetch":
					if slices.Contains(args, "+refs/heads/main:refs/remotes/origin/main") {
						return "", nil
					}
					if !slices.Contains(args, "refs/heads/goal/next") {
						t.Fatalf("wrong branch fetch: %v", args)
					}
					fetches++
					fetched = onOrigin
					return "", nil
				case "cat-file":
					if args[2] == "tip^{commit}" && !fetched {
						return "", errors.New("missing")
					}
					return "", nil
				case "rev-parse":
					return "main", nil
				case "merge-base":
					return "", notAncestor
				}
				t.Fatalf("unexpected git: %v", args)
				return "", nil
			}}
			_, selectionErr := SelectBatch(install, install, lane.Record{Root: install, Install: install}, seams)
			reasons, err := WakeReasons(install, install, time.Time{}, bedNow, seams)
			if fetches != 1 {
				t.Fatalf("fetches=%d want 1; reasons=%v err=%v", fetches, reasons, err)
			}
			if onOrigin {
				if selectionErr != nil || err != nil || !slices.Contains(reasons, WakeQueued) {
					t.Fatalf("hand-in did not wake agent: %v %v", reasons, err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "hand-in next at tip is not on origin") {
					t.Fatalf("missing commit hidden: %v", err)
				}
				if !slices.Contains(reasons, WakeFullDue) || !slices.Contains(reasons, WakeMissingHandIn) {
					t.Fatalf("missing hand-in erased wake reasons: %v", reasons)
				}
				root := install
				status := ReadStatus(t.TempDir(), lane.Record{Root: install, Install: install}, lane.View{Root: &root, Wake: &lane.Wake{Unread: []string{err.Error()}}, Summary: "idle; its agent starts within one tick"}, seams)
				if !strings.Contains(status.Summary, "hand-in next at tip is not on origin") {
					t.Fatalf("status hides missing hand-in: %s", status.Summary)
				}
			}
			for range 2 {
				_, _ = WakeReasons(install, install, time.Time{}, bedNow, seams)
				_, _ = SelectBatch(install, install, lane.Record{Root: install, Install: install}, seams)
			}
			if fetches != 1 {
				t.Fatalf("repeated reads fetched %d times", fetches)
			}
		})
	}
}

func TestHandInFetchFailureIsRetainedWithoutStatusRetries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, output, want string }{
		{"unreachable", "fatal: unable to access origin\nextra detail", "origin unreachable: fatal: unable to access origin"},
		{"absent branch", "fatal: couldn't find remote ref refs/heads/goal/next\nextra detail", "is not on origin: fatal: couldn't find remote ref refs/heads/goal/next"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("proof.trunk-every=1h\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := HandIn(install, Line{Goal: "next", SHA: "tip"}); err != nil {
				t.Fatal(err)
			}
			fetches := 0
			seams := ProveSeams{Git: func(_ string, args ...string) (string, error) {
				switch args[0] {
				case "cat-file":
					return "", errors.New("missing")
				case "fetch":
					fetches++
					return tc.output, errors.New("exit status 128")
				case "rev-parse":
					return "", errors.New("no main")
				}
				t.Fatalf("unexpected git: %v", args)
				return "", nil
			}}
			root := install
			record := lane.Record{Root: install, Install: install}
			for range 3 {
				if _, err := SelectBatch(install, install, record, seams); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("selection error: %v", err)
				}
				wake := lane.ReadWake(record, lane.WakeSources{Reasons: func(string) ([]string, error) { return WakeReasons(install, install, time.Time{}, bedNow, seams) }})
				if !slices.Contains(wake.Reasons, WakeFullDue) || !slices.Contains(wake.Reasons, WakeMissingHandIn) {
					t.Fatalf("wake reasons lost: %+v", wake)
				}
				status := ReadStatus(t.TempDir(), record, lane.View{Root: &root, Wake: &wake}, seams)
				if !strings.Contains(status.Summary, tc.want) || strings.Contains(status.Summary, "extra detail") {
					t.Fatalf("headline: %q", status.Summary)
				}
			}
			if fetches != 1 {
				t.Fatalf("same wake fetched %d times", fetches)
			}
			if _, _, err := HandIn(install, Line{Goal: "next", SHA: "replacement"}); err != nil {
				t.Fatal(err)
			}
			_, _ = SelectBatch(install, install, record, seams)
			if fetches != 2 {
				t.Fatalf("new hand-in did not get its fetch: %d", fetches)
			}
		})
	}
}

func TestHandInFetchSkipsLocalCommitsAndFixClaims(t *testing.T) {
	t.Parallel()
	for _, fix := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "missing fix claim"}[fix], func(t *testing.T) {
			t.Parallel()
			entry := Entry{Goal: "next", SHA: "tip", State: StateWaiting}
			if fix {
				entry.Fix = "incident"
			}
			seams := ProveSeams{Git: func(_ string, args ...string) (string, error) {
				if args[0] != "cat-file" {
					t.Fatalf("must not fetch: %v", args)
				}
				if fix {
					return "", errors.New("missing")
				}
				return "", nil
			}}
			install := t.TempDir()
			if err := handInCommits(install, "checkout", []Entry{entry}, seams, true); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(Dir(install)); !os.IsNotExist(err) {
				t.Fatalf("local commits or fix claims wrote fetch state: %v", err)
			}
		})
	}
}

func TestSelectionFetchesOriginOnlyHandInWithRealGit(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	sha := b.seat("seat", "next")
	if _, err := Git(b.checkout, "cat-file", "-e", sha+"^{commit}"); err == nil {
		t.Fatal("hand-in already local")
	}
	b.handIn("seat", "next", sha)
	record := lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}
	selected, err := SelectBatch(b.install, b.checkout, record, ProveSeams{})
	if err != nil || selected == nil || len(selected.Members) != 1 || selected.Members[0].SHA != sha {
		t.Fatalf("selection: %+v %v", selected, err)
	}
	if _, err := Git(b.checkout, "cat-file", "-e", sha+"^{commit}"); err != nil {
		t.Fatalf("fetch did not supply hand-in: %v", err)
	}
}

func TestConcurrentSelectionFetchesMissingHandInOnce(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.MkdirAll(Dir(install), 0755); err != nil {
		t.Fatal(err)
	}
	entry := Entry{Goal: "next", SHA: "tip", State: StateWaiting}
	var fetches atomic.Int32
	seams := ProveSeams{Git: func(_ string, args ...string) (string, error) {
		if args[0] == "fetch" {
			fetches.Add(1)
			return "", nil
		}
		return "", errors.New("missing")
	}}
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() { <-start; results <- handInCommits(install, "checkout", []Entry{entry}, seams, true) }()
	}
	close(start)
	for range 2 {
		if err := <-results; err == nil {
			t.Fatal("missing hand-in hidden")
		}
	}
	if fetches.Load() != 1 {
		t.Fatalf("concurrent selections fetched %d times", fetches.Load())
	}
}
