package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

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
	if err := testrun.BindScratch(&metadata, scratch, nil, nil); err != nil {
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
	if err := testrun.BindScratch(&admitted, scratch, nil, descriptor); err != nil {
		t.Fatal(err)
	}
	if err := testrun.BindScratch(&worker, scratch, scratch.Locator(proofrun.ScratchWriterFD(nil)), descriptor); err != nil {
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
	if err := testrun.BindScratch(&tampered, scratch, nil, descriptor); err == nil {
		t.Fatal("a changed snapshot was accepted")
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
	prepared := testrun.Preparation{ControlRoot: root, ProjectRoot: root, ConfPath: conf, CandidateTree: strings.Repeat("a", 40),
		Environment: testrun.Environment(os.Environ())}
	interrupted := errors.New("interrupted verification")
	observe := func(scratch *proofrun.ScratchRun) (string, *exec.Cmd) {
		t.Helper()
		var index string
		var command *exec.Cmd
		io := candidateengine.IO{RunGit: func(c *exec.Cmd) error {
			for _, entry := range c.Env {
				if value, ok := strings.CutPrefix(entry, "GIT_INDEX_FILE="); ok {
					index = value
				}
			}
			command = c
			return interrupted
		}}
		_, err := testrun.VerifyPrepared(testrun.SelectionRequest{}, prepared, testrun.Verification{
			Clock: time.Now, Workspace: gittree.Workspace{Dir: root}, CandidateIO: io, Scratch: scratch, WorkerPolicy: testingWorkerPolicy})
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
