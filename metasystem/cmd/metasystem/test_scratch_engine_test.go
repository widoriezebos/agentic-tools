package main

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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"golang.org/x/sys/unix"
)

func seedCandidateEngineEntry(t *testing.T, entry, identity string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(entry, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(entry, "metasystem")
	if err := testexec.WriteFile(executable, []byte("#!/bin/sh\necho "+identity+"\n"), 0o500); err != nil {
		t.Fatal(err)
	}
	digest, err := fileSHA256(executable)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(candidateEngineCacheRecord{Version: 1, BuildIdentity: identity, Digest: digest})
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
	base := time.Now().Add(-time.Hour)
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
	scratch, err := proofrun.CreateScratchRun(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	ctx := proofrun.WithScratchRun(context.Background(), scratch)
	warm := identities[9]
	built, err := prepareScratchCandidateEngine(ctx, scratch, controlRoot, gittree.Workspace{}, "metasystem", "", nil,
		func() error { t.Fatal("warm identity took the cold path"); return nil }, candidateEngineIO{}, warm)
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
	if remaining != candidateEngineV2Retained {
		t.Fatalf("v2 keeps %d entries, want %d", remaining, candidateEngineV2Retained)
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
	if digest, err := fileSHA256(built.Path); err != nil || digest != built.Digest {
		t.Fatalf("private engine after eviction = %s, %v", digest, err)
	}
	if err := scratch.Cleanup(nil); err != nil {
		t.Fatal(err)
	}
}

// A6 EXDEV: a rename that crosses filesystems leaves the stage inside the
// run root as the run's private source; nothing is staged outside the root.
func TestScratchCandidateEnginePublicationSurvivesEXDEV(t *testing.T) {
	t.Parallel()
	controlRoot := t.TempDir()
	scratch, err := proofrun.CreateScratchRun(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	identity := strings.Repeat("a", 40)
	source := filepath.Join(t.TempDir(), "built")
	seedCandidateEngineEntry(t, source, identity, time.Now())
	digest, _ := fileSHA256(filepath.Join(source, "metasystem"))
	entry := filepath.Join(controlRoot, "artifacts", "agents", "candidate-engines", "v2", identity)
	used, err := publishScratchCandidateEngine(scratch.Dir("engine"), entry, identity,
		&candidateEngineBuild{Path: filepath.Join(source, "metasystem"), Digest: digest, Commit: identity},
		func(string, string) error { return &os.LinkError{Op: "rename", Err: unix.EXDEV} })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(used, scratch.Dir("engine")+string(os.PathSeparator)) || validatedCandidateEngine(used, identity) == nil {
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
}

// One run snapshots its environment once, at the first (metadata) binding;
// the post-admission and worker requests carry that exact descriptor and the
// snapshot file is never rewritten, even after the inherited file changed.
// A tampered snapshot refuses a later binding instead of being re-taken.
func TestBindTestingScratchSnapshotsOnceAndCarriesTheDescriptor(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	scratch, err := proofrun.CreateScratchRun(control)
	if err != nil {
		t.Fatal(err)
	}
	defer scratch.Cleanup(nil)
	source := filepath.Join(t.TempDir(), "go-env")
	if err := os.WriteFile(source, []byte("GOFLAGS=-count=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := proofrun.TestRunRequest{Environment: []string{"GOENV=" + source}}
	metadata, admitted, worker := base, base, base
	if err := bindTestingScratch(&metadata, scratch, nil, nil); err != nil {
		t.Fatal(err)
	}
	descriptor := metadata.ScratchEnvironment
	if descriptor == nil || descriptor.GoEnvDigest == "" {
		t.Fatalf("metadata binding prepared no Go env snapshot: %+v", descriptor)
	}
	snapshots, err := filepath.Glob(filepath.Join(scratch.Root(), "goenv", "*"))
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("snapshot files = %v, %v", snapshots, err)
	}
	before, err := os.Stat(snapshots[0])
	if err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(snapshots[0])
	if err := os.WriteFile(source, []byte("GOFLAGS=-race\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := bindTestingScratch(&admitted, scratch, nil, descriptor); err != nil {
		t.Fatal(err)
	}
	if err := bindTestingScratch(&worker, scratch, scratch.Locator(proofrun.ScratchWriterFD(nil)), descriptor); err != nil {
		t.Fatal(err)
	}
	if admitted.ScratchEnvironment != descriptor || worker.ScratchEnvironment != descriptor || worker.Scratch == nil {
		t.Fatal("a later request does not carry the first descriptor")
	}
	after, err := os.Stat(snapshots[0])
	now, _ := os.ReadFile(snapshots[0])
	if err != nil || !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) || string(now) != string(contents) {
		t.Fatalf("snapshot was rewritten: %v", err)
	}
	if err := os.Chmod(snapshots[0], 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshots[0], []byte("GOFLAGS=-race\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tampered := base
	if err := bindTestingScratch(&tampered, scratch, nil, descriptor); err == nil {
		t.Fatal("a changed snapshot was accepted")
	}
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
		io := candidateEngineIO{runGit: func(c *exec.Cmd) error {
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
		if _, err := engineProjectionTree(ctx, "/repo", strings.Repeat("a", 40), []string{"metasystem"}, io); !errors.Is(err, interrupted) {
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

// Test verify's own identity projection lands in its scratch run,
// with the writer inherited and hooks off; without a scratch run it keeps
// host temp. The Git stub interrupts the first projection command.
func TestVerifyIdentityProjectionUsesTheVerificationScratchRun(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepared := testingPreparation{ControlRoot: root, ProjectRoot: root, ConfPath: conf, CandidateTree: strings.Repeat("a", 40),
		Environment: testingEnvironment(os.Environ())}
	interrupted := errors.New("interrupted verification")
	observe := func(scratch *proofrun.ScratchRun) (string, *exec.Cmd) {
		t.Helper()
		var index string
		var command *exec.Cmd
		io := candidateEngineIO{runGit: func(c *exec.Cmd) error {
			for _, entry := range c.Env {
				if value, ok := strings.CutPrefix(entry, "GIT_INDEX_FILE="); ok {
					index = value
				}
			}
			command = c
			return interrupted
		}}
		_, err := verifyRetainedTestingPrepared(testingSelectionRequest{}, prepared, retainedTestingVerification{
			clock: time.Now, workspace: gittree.Workspace{Dir: root}, candidateIO: io, scratch: scratch})
		if !errors.Is(err, interrupted) || command == nil {
			t.Fatalf("verify did not reach the identity projection: %v", err)
		}
		return index, command
	}
	scratch, err := proofrun.CreateScratchRun(root)
	if err != nil {
		t.Fatal(err)
	}
	defer scratch.Cleanup(nil)
	index, command := observe(scratch)
	if !strings.HasPrefix(index, scratch.Dir("engine")+string(os.PathSeparator)) || len(command.ExtraFiles) != 1 ||
		command.ExtraFiles[0] != scratch.Writer() || !strings.Contains(strings.Join(command.Args, " "), "core.hooksPath="+scratch.Dir("no-hooks")) {
		t.Fatalf("verify projection = %s %v", index, command.Args)
	}
	index, command = observe(nil)
	if strings.HasPrefix(index, scratch.Root()) || len(command.ExtraFiles) != 0 {
		t.Fatalf("unmanaged verify projection = %s %v", index, command.Args)
	}
}
