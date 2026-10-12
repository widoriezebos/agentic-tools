package plain

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestRestartCancelledTreeMayBeProvenAgain(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, ended := restartBed(t, 12, false)
	if _, err := SelectBatch(install, install, batch.Lane, seams); err != nil || *ended != 1 {
		t.Fatalf("restart did not happen: %v %d", err, *ended)
	}
	seams.Person = nil
	_, found, err := seams.checkBound(install, running.Tree)
	var nr *NoRepeat
	t.Logf("checkBound(old tree): found=%v err=%v", found, err)
	if errors.As(err, &nr) {
		t.Fatalf("REPRODUCED F-1: %v", err)
	}
}

func liveRef(t *testing.T, args ...string) (*exec.Cmd, string) {
	c := exec.Command(args[0], args[1:]...)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	exact, _, err := identity.KernelProber{}.Probe(int64(c.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	return c, ref
}

func TestRestartFinishedCommandNeverWedgesTheLane(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, ended := restartBed(t, 12, false)
	wrapper, wref := liveRef(t, "/bin/sleep", "600")
	t.Cleanup(func() { _ = wrapper.Process.Kill(); _, _ = wrapper.Process.Wait() })
	cmd, cref := liveRef(t, "/bin/sleep", "600")
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	running.Pid, running.Process, running.ProofProcess = int64(wrapper.Process.Pid), wref, cref
	if err := writeRunning(install, running); err != nil {
		t.Fatal(err)
	}
	seams.EndProof = nil
	seams.Alive = func(Running) bool { return true }
	_, err := SelectBatch(install, install, batch.Lane, seams)
	c, _ := readCancel(install, running)
	t.Logf("tick 1 (wrapper alive, command exited): err=%v cancel=%v ended=%d", err, c != nil, *ended)
	if err != nil || c != nil {
		t.Fatalf("REPRODUCED F-2: tick wrote a decision against a finishing proof")
	}
}

// A person's --classify act holds a running record of its own; a newcomer must not cancel it.
func TestRestartNeverEndsAPersonsClassification(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, ended := restartBed(t, 12, false)
	if err := appendLine(resultsPath(install), Result{Attempt: running.Attempt, Tree: running.Tree, Result: Red, BatchID: batch.ID}); err != nil {
		t.Fatal(err)
	}
	classify := Running{Attempt: running.Attempt + "-classify-x", Commit: running.Commit, Tree: running.Tree, BatchID: running.BatchID, BatchMembers: running.BatchMembers, Since: running.Since, Pid: 4243, Process: "person-classifier", Person: &ActProvenance{Person: "Wido"}}
	if err := writeRunning(install, classify); err != nil {
		t.Fatal(err)
	}
	seams.EndProof = func(r Running) error {
		*ended++
		t.Logf("EndProof called on %s (pid %d, person %v)", r.Attempt, r.Pid, r.Person != nil)
		return nil
	}
	selected, err := SelectBatch(install, install, batch.Lane, seams)
	t.Logf("err=%v ended=%d restarts=%d waiting=%v", err, *ended, len(selected.Restarts), selected.Waiting)
	if *ended > 0 {
		t.Fatalf("REPRODUCED: a newcomer ended a person's classification act and reopened the batch")
	}
}
