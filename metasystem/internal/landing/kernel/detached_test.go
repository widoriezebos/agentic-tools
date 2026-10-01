package kernel

import (
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// The detached-prove helper's environment: its mode (start, the caller of
// landing prove; prove, the detached landing prove --wait it starts), and
// the bed it acts on.
const (
	helperMode     = "METASYSTEM_TEST_DETACHED_PROVE_MODE"
	helperHome     = "METASYSTEM_TEST_DETACHED_PROVE_HOME"
	helperCheckout = "METASYSTEM_TEST_DETACHED_PROVE_CHECKOUT"
	helperEngine   = "METASYSTEM_TEST_DETACHED_PROVE_ENGINE"
	helperChild    = "METASYSTEM_TEST_DETACHED_PROVE_CHILD"
)

// TestDetachedProveHelper is not a test: it is the engine's two processes
// for TestStartProofOutlivesItsCaller. In start mode it is the caller of
// landing prove, which returns at once; in prove mode it is the detached
// landing prove --wait --tree T --attempt A that start launched, running the
// real Prove over a stand-in test run child.
func TestDetachedProveHelper(t *testing.T) {
	mode := os.Getenv(helperMode)
	if mode == "" {
		t.Skip("the detached-prove helper runs only as its own process")
	}
	layout, err := lane.NewLayout(os.Getenv(helperCheckout))
	if err != nil {
		t.Fatal(err)
	}
	request := ProveRequest{Home: os.Getenv(helperHome), Layout: layout, Actor: kernelActor}
	switch mode {
	case "start":
		seams := ProductionStartSeams()
		seams.Executable = func() (string, error) { return os.Getenv(helperEngine), nil }
		started, err := StartProof(request, seams)
		if err != nil || started.Already {
			t.Fatalf("start = %+v, %v; want a new detached proof", started, err)
		}
	case "prove":
		argv := flag.Args()
		if len(argv) != 7 || argv[0] != "landing" || argv[1] != "prove" || argv[2] != "--wait" {
			t.Fatalf("the detached proof ran %q; want landing prove --wait --tree T --attempt A", argv)
		}
		request.Tree, request.Attempt = argValue(argv, "--tree"), argValue(argv, "--attempt")
		seams := ProductionProveSeams()
		seams.Executable = func() (string, error) { return os.Getenv(helperChild), nil }
		if _, err := Prove(request, seams); err != nil {
			t.Fatal(err)
		}
	}
}

// landing prove starts the proof as a detached job and returns at once; the
// proof is recorded running with its process while it runs, survives the
// end of its caller's whole process group (a headless agent session that
// exits), and records its result exactly as a blocking prove does. A repeat
// while the same tree's proof runs starts nothing.
func TestStartProofOutlivesItsCaller(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	head := bed.merge(bed.first, bed.second)
	tree := bed.tree(head)
	green, _ := bed.fakeChild("green", passed("app-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	release := filepath.Join(bed.dir, "release")
	child := filepath.Join(bed.dir, "children", "gated")
	// The stand-in test run waits for the test's word, bounded, so the
	// proof is certainly running when its caller has gone.
	if err := testexec.WriteFile(child, []byte("#!/bin/sh\ni=0\nwhile [ ! -e '"+release+"' ] && [ \"$i\" -lt 1200 ]; do sleep 0.1; i=$((i+1)); done\nexec '"+green+"' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(bed.dir, "engine", "metasystem")
	if err := os.MkdirAll(filepath.Dir(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(engine, []byte("#!/bin/sh\n"+helperMode+"=prove exec '"+self+"' -test.run='^TestDetachedProveHelper$' -- \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	caller := exec.Command(self, "-test.run=^TestDetachedProveHelper$")
	caller.Env = append(os.Environ(), helperMode+"=start", helperHome+"="+bed.home, helperCheckout+"="+string(bed.layout.Checkout),
		helperEngine+"="+engine, helperChild+"="+child)
	caller.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if out, err := caller.CombinedOutput(); err != nil {
		t.Fatalf("the caller of landing prove: %v\n%s", err, out)
	}
	// The caller's session ends: its whole process group is killed, as a
	// headless agent's end does. This is the group this test started.
	_ = syscall.Kill(-caller.Process.Pid, syscall.SIGKILL)

	running, live, ok, err := ReadRunningProof(bed.layout, identity.KernelProber{})
	if err != nil || !ok || live != identity.Alive || running.Tree != tree || running.Commit != head || running.Status != batch.AttemptRunning || running.Process == "" {
		t.Fatalf("after its caller ended the proof is %+v %v %v %v; want tree %s running in a live process", running, live, ok, err, tree)
	}
	if _, proven, _ := ReadTreeProof(bed.layout, tree); !proven {
		t.Fatalf("the running proof is not kept for its tree")
	}
	var refusal *Refusal
	if _, err := bed.push(); !errors.As(err, &refusal) || refusal.Code != CodePushUnproven {
		t.Fatalf("push while the tree is being proven = %v; want %s", err, CodePushUnproven)
	}

	// A repeat while it runs starts nothing.
	repeat := ProductionStartSeams()
	repeat.Launch = func([]string, string, string) (int64, error) {
		t.Fatalf("a repeat while the tree's proof runs launched a second proof")
		return 0, nil
	}
	again, err := StartProof(ProveRequest{Home: bed.home, Layout: bed.layout, Actor: kernelActor}, repeat)
	if err != nil || !again.Already || again.Proof.Attempt != running.Attempt {
		t.Fatalf("a repeat while proving = %+v, %v; want the running attempt %s, already", again, err, running.Attempt)
	}

	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Minute)
	var kept TreeProof
	for {
		kept, _, err = ReadTreeProof(bed.layout, tree)
		if err == nil && kept.Status != batch.AttemptRunning && kept.Status != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the detached proof did not end within the bound: %+v %v", kept, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if kept.Status != batch.AttemptGreen || kept.Attempt != running.Attempt || kept.EndedAt == "" {
		t.Fatalf("the detached proof ended %+v; want green for attempt %s", kept, running.Attempt)
	}
	for _, id := range []string{firstBatch, secondBatch} {
		record, err := bed.store().Load(id)
		if err != nil || len(record.Attempts) != 1 || record.Attempts[0].ID != running.Attempt || record.Attempts[0].Status != batch.AttemptGreen {
			t.Fatalf("batch %s attempts = %+v %v; want the detached attempt green", id, record.Attempts, err)
		}
	}
	if _, _, ok, err := ReadRunningProof(bed.layout, identity.KernelProber{}); ok || err != nil {
		t.Fatalf("a proof still reads running after it ended (%v)", err)
	}
}

// A proof whose process died without a result reads died, never running,
// and blocks nothing: the next prove records it unavailable and proves
// again.
func TestDiedProofBlocksNothing(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	head := bed.merge(bed.first)
	tree := bed.tree(head)
	// A process identity that is no longer the process at its pid.
	exact, live, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || live != identity.Alive {
		t.Fatalf("probe self: %v %v", live, err)
	}
	gone := exact.Ref()
	if gone.StartTicks != 0 {
		gone.StartTicks = 1
	} else {
		gone.StartedAtUnixMicro, gone.StartedAtSec = 1_000_000, 1
	}
	process, err := identity.EncodeRef(gone)
	if err != nil {
		t.Fatal(err)
	}
	if err := batch.StartAttempt(bed.store(), firstBatch, batch.ProofAttempt{ID: "01j5x00000000000000000kdie", Subject: batch.SubjectBatch,
		Covers: []string{bed.firstID}, Tree: tree, Purpose: "delivery", Actor: kernelActor, StartedAt: kernelAt.Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	died := TreeProof{Tree: tree, Commit: head, Attempt: "01j5x00000000000000000kdie", Status: batch.AttemptRunning, Process: process,
		Batches: []string{firstBatch}, StartedAt: kernelAt.Format(time.RFC3339Nano)}
	if err := keepTreeProof(bed.layout, died); err != nil {
		t.Fatal(err)
	}
	if got, state, ok, err := ReadRunningProof(bed.layout, identity.KernelProber{}); err != nil || !ok || state != identity.Dead || got.Attempt != died.Attempt {
		t.Fatalf("a died proof reads %+v %v %v %v; want it, dead", got, state, ok, err)
	}
	executable, _ := bed.fakeChild("green", passed("app-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	proof, err := bed.prove("", executable)
	if err != nil || proof.Status != batch.AttemptGreen || proof.Attempt == died.Attempt {
		t.Fatalf("prove after a died proof = %+v, %v; want a new green attempt", proof, err)
	}
	record, err := bed.store().Load(firstBatch)
	if err != nil {
		t.Fatal(err)
	}
	attempt, ok := record.Attempt(died.Attempt)
	if !ok || attempt.Status != batch.AttemptUnavailable || attempt.Reason == "" {
		t.Fatalf("the died attempt is recorded %+v; want it unavailable with its reason", attempt)
	}
}
