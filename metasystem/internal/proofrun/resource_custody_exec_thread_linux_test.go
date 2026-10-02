//go:build linux

package proofrun

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The custodian decides its bound worker ended from the worker's
// /proc/<pid>/stat. custody-exec execs the command, so that stat must never
// read zombie or exiting while the exec is in progress. It did when the exec
// ran on a non-leader thread: the kernel shows the leader exiting, then
// zombie, until the execing thread takes over the pid, and the custodian
// killed the live command (a VM probe saw it in 99 of 400 execs under load).
// The engine locks its main goroutine to the leader thread (cmd/metasystem
// exec_thread.go), so every read through the exec sees a running leader.
func TestCustodyExecNeverShowsAnEndedLeaderWhileItExecs(t *testing.T) {
	t.Parallel()
	engine := buildResourceCustodyEngine(t)
	catPath, err := exec.LookPath("cat")
	if err != nil {
		t.Fatal(err)
	}
	prober := identity.KernelProber{}
	const execs = 200
	for run := 0; run < execs; run++ {
		if ended := custodyExecEndedReads(t, prober, engine, catPath); ended != 0 {
			t.Fatalf("exec %d: %d stat reads showed the live worker's leader zombie or exiting during its exec", run, ended)
		}
	}
}

// custodyExecEndedReads releases one custody-exec worker into cat and reads
// its stat until /proc/<pid>/exe names cat. cat blocks on a pipe this test
// holds, so the worker cannot genuinely end before the exec is observed.
func custodyExecEndedReads(t *testing.T, prober identity.Prober, engine, catPath string) int {
	t.Helper()
	input, feed, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer feed.Close()
	command := exec.Command(catPath)
	command.Stdin = input
	barrier, err := prepareCustodyExec(command, engine)
	if err != nil {
		input.Close()
		t.Fatal(err)
	}
	defer barrier.close()
	if err := command.Start(); err != nil {
		input.Close()
		t.Fatal(err)
	}
	input.Close()
	for _, file := range command.ExtraFiles[len(command.ExtraFiles)-2:] {
		_ = file.Close()
	}
	defer func() {
		_ = feed.Close()
		_ = command.Wait()
	}()
	if err := barrier.await(t.Context().Done()); err != nil {
		t.Fatal(err)
	}
	pid := int64(command.Process.Pid)
	exePath := fmt.Sprintf("/proc/%d/exe", pid)
	if err := barrier.releaseWork(); err != nil {
		t.Fatal(err)
	}
	ended := 0
	for {
		exact, state, err := prober.Probe(pid)
		if err != nil || state != identity.Alive {
			t.Fatalf("worker %d ended before cat ran: state=%s err=%v", pid, state, err)
		}
		if exact.Zombie || exact.Exiting {
			ended++
		}
		if exe, err := os.Readlink(exePath); err == nil && exe == catPath {
			return ended
		}
		if t.Context().Err() != nil {
			t.Fatalf("worker %d never became cat: %v", pid, t.Context().Err())
		}
		runtime.Gosched()
	}
}
