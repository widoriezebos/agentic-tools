package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// kernelBed is a host whose registered landing lane is a real nested
// checkout (Git toplevel != installation root).
type kernelBed struct {
	*laneVerbBed
	checkout, installation string
}

func newKernelBed(t *testing.T) *kernelBed {
	t.Helper()
	bed := &kernelBed{laneVerbBed: newLaneVerbBed(t)}
	bed.checkout = filepath.Join(filepath.Dir(bed.home), "lane")
	bed.installation = filepath.Join(bed.checkout, "metasystem")
	if err := os.MkdirAll(bed.installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "--quiet", "-b", "main", bed.checkout).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(bed.installation, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	registerLane(t, bed.home, bed.checkout, "Wido", laneTestNow)
	return bed
}

func (bed *kernelBed) owners() intentOwners {
	owners := bed.laneVerbBed.owners()
	owners.resolver = stateroot.NewResolver(stateroot.RepositoryTop, os.Executable)
	return owners
}

func (bed *kernelBed) git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=lane", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// landMain gives the lane checkout a bare origin whose main is its first
// commit, and names that commit.
func (bed *kernelBed) landMain(t *testing.T) string {
	t.Helper()
	origin := filepath.Join(filepath.Dir(bed.checkout), "origin.git")
	bed.git(t, filepath.Dir(bed.checkout), "init", "--quiet", "--bare", "-b", "main", origin)
	if err := os.WriteFile(filepath.Join(bed.checkout, ".gitignore"), []byte("/artifacts/\nmetasystem/bin/\nmetasystem/artifacts/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, bed.checkout, "add", "-A")
	bed.git(t, bed.checkout, "commit", "--quiet", "-m", "landed")
	bed.git(t, bed.checkout, "remote", "add", "origin", "file://"+origin)
	bed.git(t, bed.checkout, "push", "--quiet", "origin", "main")
	return bed.git(t, bed.checkout, "rev-parse", "HEAD")
}

// proveGreen keeps a green result for the lane checkout's HEAD tree, as
// landing prove does.
func (bed *kernelBed) proveGreen(t *testing.T) {
	t.Helper()
	tree := bed.git(t, bed.checkout, "rev-parse", "HEAD^{tree}")
	data, err := json.Marshal(kernel.TreeProof{Tree: tree, Attempt: "a1", Status: batch.AttemptGreen, ResultPath: "/result.json", StartedAt: "2026-10-01T09:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(bed.checkout, "artifacts", "agents", "landing-proofs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, tree+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (bed *kernelBed) runWith(t *testing.T, owners intentOwners, words ...string) (int, string) {
	t.Helper()
	command, ok := findIntentAction(words[0], words[1])
	if !ok {
		t.Fatalf("no public command %s %s", words[0], words[1])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, words[2:], &stdout, &stderr, bed.cwd, owners)
	return code, stdout.String() + stderr.String()
}

func init() {
	registerIdempotency("landing push", idemStateful, "main already is the proven HEAD: success, nothing pushed", witnessLandingPushRepeat)
}

// witnessLandingPushRepeat pushes the lane checkout's proven HEAD twice
// with the production kernel: the second is success, pushes nothing and
// leaves the host home as it was.
func witnessLandingPushRepeat(t *testing.T) {
	bed := newKernelBed(t)
	main := bed.landMain(t)
	if err := os.WriteFile(filepath.Join(bed.installation, "change.txt"), []byte("a change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, bed.checkout, "add", "-A")
	bed.git(t, bed.checkout, "commit", "--quiet", "-m", "a merged change")
	head := bed.git(t, bed.checkout, "rev-parse", "HEAD")
	bed.setCommand(t, "true")
	if code, text := bed.runWith(t, bed.owners(), "landing", "prove", "--wait"); code != 0 {
		t.Fatalf("prove = %d\n%s", code, text)
	}
	origin := func() string { return bed.git(t, bed.checkout, "ls-remote", "origin", "refs/heads/main") }
	if code, text := bed.runWith(t, bed.owners(), "landing", "push"); code != 0 || !strings.Contains(text, "pushed "+shortLandingID(head)+" to main (from "+shortLandingID(main)+")") {
		t.Fatalf("first push = %d\n%s", code, text)
	}
	pushed, home := origin(), idemTreeDigest(t, bed.home)
	if code, text := bed.runWith(t, bed.owners(), "landing", "push"); code != 0 || !strings.Contains(text, "main already is "+shortLandingID(head)) {
		t.Fatalf("repeated push = %d\n%s", code, text)
	}
	if origin() != pushed {
		t.Fatalf("a repeated push moved main")
	}
	idemSameTree(t, "a repeated landing push (home)", home, idemTreeDigest(t, bed.home))
}

// The lane verbs' layout goldens join G1b through the group hook.
var _ = func() bool {
	layoutGroupCases = append(layoutGroupCases, func() []layoutCase {
		return []layoutCase{
			{name: "landing-prove-unset", args: []string{"landing", "prove", "--wait"}, bed: landingKernelLayoutBed()},
			{name: "landing-push-refusal", args: []string{"landing", "push"}, bed: landingKernelLayoutBed()},
		}
	})
	return true
}()

// landingKernelLayoutBed is the running lane's bed whose lane has no proof
// command (prove refuses) and whose HEAD was never proven (push refuses).
func landingKernelLayoutBed() func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		return landingLayoutBed(landingLayoutRunning)(t)
	}
}

// syncWriter is an output a test reads while the verb still writes it; seen
// closes once the output holds want.
type syncWriter struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	want string
	seen chan struct{}
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	if w.seen != nil && strings.Contains(w.buf.String(), w.want) {
		close(w.seen)
		w.seen = nil
	}
	return n, err
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// N-3: a landing stop that finds the host flock held (another lane step
// runs) says so in one plain line before it waits, then stops the lane
// once the flock is free.
func TestLandingStopSaysWhyItWaitsForTheHostLock(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.alive = true
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	held, err := lock.File(lane.LockPath(bed.home), 0o600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	command, _ := findIntentAction("landing", "stop")
	seen := make(chan struct{})
	var stdout syncWriter
	stderr := syncWriter{want: "another landing step holds the lane; the stop takes effect when it finishes", seen: seen}
	done := make(chan int, 1)
	go func() {
		done <- runIntentIn(command, []string{"--by", "Wido"}, &stdout, &stderr, bed.cwd, bed.owners())
	}()
	select {
	case code := <-done:
		t.Fatalf("stop finished (%d) while the host lock was held, or waited silently: %q %q", code, stdout.String(), stderr.String())
	case <-seen:
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != 0 {
		t.Fatalf("stop after the lock was freed = %d %q", code, stderr.String())
	}
	if _, paused := lane.ReadPause(bed.home); !paused {
		t.Fatal("the lane is not paused after the stop")
	}
}
