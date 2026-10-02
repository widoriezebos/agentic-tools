package candidateengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/digest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// lockedScratchRun runs body with forks excluded; body must not fork or
// write through testexec (both take ForkLock).
func lockedScratchRun(t *testing.T, body func()) {
	t.Helper()
	if err := testexec.Locked(func() error { body(); return nil }); err != nil {
		t.Fatal(err)
	}
}

func seedCandidateEngineEntry(t *testing.T, entry, identity string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(entry, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(entry, "metasystem")
	if err := testexec.WriteFile(executable, []byte("#!/bin/sh\necho "+identity+"\n"), 0o500); err != nil {
		t.Fatal(err)
	}
	sum, err := digest.FileSHA256(executable)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(cacheRecord{Version: 1, BuildIdentity: identity, Digest: sum})
	record := filepath.Join(entry, "record.json")
	if err := os.WriteFile(record, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(record, at, at); err != nil {
		t.Fatal(err)
	}
}

// A6: the v2 namespace keeps eight entries, never evicts a locked entry or a
// legacy one, and every consumer runs a validated private copy in its root.
func TestScratchCandidateEngineV2KeepsEightAndCopiesOut(t *testing.T) {
	t.Parallel()
	controlRoot := t.TempDir()
	cacheRoot := filepath.Join(controlRoot, "artifacts", "agents", "candidate-engines")
	v2 := filepath.Join(cacheRoot, "v2")
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	identities := make([]string, 10)
	for index := range identities {
		identities[index] = fmt.Sprintf("%040d", index)
		seedCandidateEngineEntry(t, filepath.Join(v2, identities[index]), identities[index], base.Add(time.Duration(index)*time.Minute))
	}
	legacy := filepath.Join(cacheRoot, identities[0])
	seedCandidateEngineEntry(t, legacy, identities[0], base.Add(-time.Hour))
	held, err := os.OpenFile(filepath.Join(v2, identities[0]+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := unix.Flock(int(held.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	// Cleanup proves the writer free through a fresh description; a parallel
	// test's fork would copy the open writer and hold the flock until it
	// execs, so forks are excluded from creation to Cleanup.
	lockedScratchRun(t, func() {
		scratch, err := proofrun.CreateScratchRun(controlRoot)
		if err != nil {
			t.Fatal(err)
		}
		ctx := proofrun.WithScratchRun(context.Background(), scratch)
		warm := identities[9]
		built, err := prepareScratch(ctx, scratch, controlRoot, gittree.Workspace{}, "metasystem", "", nil,
			func() error { t.Fatal("warm identity took the cold path"); return nil }, IO{}, warm)
		if err != nil {
			t.Fatal(err)
		}
		if built.Path != filepath.Join(scratch.Root(), "engine", "metasystem") {
			t.Fatalf("consumer engine %s is not the run's private copy", built.Path)
		}
		for _, evicted := range identities[1:3] {
			if _, err := os.Lstat(filepath.Join(v2, evicted)); !os.IsNotExist(err) {
				t.Fatalf("oldest unlocked v2 entry %s survived: %v", evicted, err)
			}
		}
		remaining := 0
		if entries, err := os.ReadDir(v2); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					remaining++
				}
			}
		}
		if remaining != v2Retained {
			t.Fatalf("v2 keeps %d entries, want %d", remaining, v2Retained)
		}
		for _, kept := range []string{filepath.Join(v2, identities[0]), legacy, filepath.Join(v2, identities[1]+".lock"), filepath.Join(v2, warm)} {
			if _, err := os.Stat(kept); err != nil {
				t.Fatalf("%s: %v", kept, err)
			}
		}
		// The consumer keeps its copy when its entry is later evicted.
		if err := os.RemoveAll(filepath.Join(v2, warm)); err != nil {
			t.Fatal(err)
		}
		if sum, err := digest.FileSHA256(built.Path); err != nil || sum != built.Digest {
			t.Fatalf("private engine after eviction = %s, %v", sum, err)
		}
		if err := scratch.Cleanup(nil); err != nil {
			t.Fatal(err)
		}
	})
}

// A6 EXDEV: a rename that crosses filesystems leaves the stage inside the
// run root as the run's private source; nothing is staged outside the root.
func TestScratchCandidateEnginePublicationSurvivesEXDEV(t *testing.T) {
	t.Parallel()
	controlRoot := t.TempDir()
	identity := strings.Repeat("a", 40)
	source := filepath.Join(t.TempDir(), "built")
	seedCandidateEngineEntry(t, source, identity, time.Now())
	sum, _ := digest.FileSHA256(filepath.Join(source, "metasystem"))
	// Cleanup proves the writer free through a fresh description; a parallel
	// test's fork would copy the open writer and hold the flock until it
	// execs, so forks are excluded from creation to Cleanup.
	lockedScratchRun(t, func() {
		scratch, err := proofrun.CreateScratchRun(controlRoot)
		if err != nil {
			t.Fatal(err)
		}
		entry := filepath.Join(controlRoot, "artifacts", "agents", "candidate-engines", "v2", identity)
		used, err := publishScratch(scratch.Dir("engine"), entry, identity,
			&Engine{Path: filepath.Join(source, "metasystem"), Digest: sum, Commit: identity},
			func(string, string) error { return &os.LinkError{Op: "rename", Err: unix.EXDEV} }, t.Output())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(used, scratch.Dir("engine")+string(os.PathSeparator)) || validated(used, identity) == nil {
			t.Fatalf("unpublished engine source = %s", used)
		}
		if _, err := os.Lstat(entry); !os.IsNotExist(err) {
			t.Fatalf("EXDEV published an entry: %v", err)
		}
		if err := scratch.Cleanup(nil); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(used); !os.IsNotExist(err) {
			t.Fatalf("stage outlived the run: %v", err)
		}
	})
}

// C1: in a managed run the engine identity projection keeps its indexes in
// the run's engine directory and its Git children inherit the writer with
// hooks off; unmanaged callers keep host temp and the plain command.
func TestEngineProjectionIndexesStayInTheOwnedRoot(t *testing.T) {
	t.Parallel()
	scratch, err := proofrun.CreateScratchRun(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer scratch.Cleanup(nil)
	interrupted := errors.New("interrupted preflight")
	observe := func(ctx context.Context) (index string, command *exec.Cmd) {
		t.Helper()
		io := IO{RunGit: func(c *exec.Cmd) error {
			for _, entry := range c.Env {
				if value, ok := strings.CutPrefix(entry, "GIT_INDEX_FILE="); ok {
					index = value
				}
			}
			if _, err := os.Stat(filepath.Dir(index)); err != nil {
				t.Fatalf("projection directory absent while Git runs: %v", err)
			}
			command = c
			return interrupted
		}}
		if _, err := projectionTree(ctx, "/repo", strings.Repeat("a", 40), []string{"metasystem"}, io); !errors.Is(err, interrupted) {
			t.Fatalf("projection error = %v", err)
		}
		return index, command
	}
	index, command := observe(proofrun.WithScratchRun(context.Background(), scratch))
	if !strings.HasPrefix(index, scratch.Dir("engine")+string(os.PathSeparator)) {
		t.Fatalf("managed projection index %s is outside the run root", index)
	}
	if len(command.ExtraFiles) != 1 || command.ExtraFiles[0] != scratch.Writer() ||
		!strings.Contains(strings.Join(command.Args, " "), "core.hooksPath="+scratch.Dir("no-hooks")) {
		t.Fatalf("managed projection Git child = %v, extra %v", command.Args, command.ExtraFiles)
	}
	if !strings.Contains(strings.Join(command.Args, " "), "-C /repo -c core.fileMode=true -c core.useReplaceRefs=false read-tree "+strings.Repeat("a", 40)) {
		t.Fatalf("projection arguments changed: %v", command.Args)
	}
	index, command = observe(context.Background())
	if strings.HasPrefix(index, scratch.Root()) || len(command.ExtraFiles) != 0 || strings.Contains(strings.Join(command.Args, " "), "hooksPath") {
		t.Fatalf("unmanaged projection changed: %s %v", index, command.Args)
	}
}
