package steward

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type replacementStopProbe func(int64) (identity.Exact, identity.Liveness, error)

func (p replacementStopProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return p(pid)
}

func TestReplacementStopWaitsForExecutionToEnd(t *testing.T) {
	t.Parallel()
	for _, ticks := range []bool{false, true} {
		for _, tc := range []struct {
			name      string
			zombieOn  syscall.Signal
			reused    bool
			wantKill  bool
			wantError bool
			wantWait  time.Duration
		}{
			{name: "zombie after TERM", zombieOn: syscall.SIGTERM},
			{name: "zombie after KILL", zombieOn: syscall.SIGKILL, wantKill: true, wantWait: runnerReplacementKillWait},
			{name: "still executing", wantKill: true, wantError: true, wantWait: 2 * runnerReplacementKillWait},
			{name: "pid reused by zombie", zombieOn: syscall.SIGTERM, reused: true},
			{name: "pid reused by live process", reused: true},
		} {
			t.Run(fmt.Sprintf("ticks=%t/%s", ticks, tc.name), func(t *testing.T) {
				root := t.TempDir()
				if err := os.MkdirAll(runnerDir(root), 0o755); err != nil {
					t.Fatal(err)
				}
				start := time.Unix(100, 0)
				runner := RunnerRecord{Pid: 4242, PidStartedAt: start.Unix()}
				exact := identity.Exact{Pid: runner.Pid, StartedAt: start}
				if ticks {
					runner.StartTicks, runner.BootID = 123, "fixture-boot"
					exact.StartTicks, exact.BootID = runner.StartTicks, runner.BootID
				}
				if err := writeJSONAtomic(runnerRecordPath(root), runner); err != nil {
					t.Fatal(err)
				}
				probe := replacementStopProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
					if pid != runner.Pid {
						t.Fatalf("probe pid = %d, want recorded runner", pid)
					}
					return exact, identity.Alive, nil
				})
				var signals []syscall.Signal
				signal := func(pid int, sig syscall.Signal) error {
					if int64(pid) != runner.Pid {
						t.Fatalf("signal pid = %d, want recorded runner", pid)
					}
					marker, err := os.ReadFile(runnerStopPath(root))
					if err != nil || string(marker) != "restart\n" {
						t.Fatal("restart marker must precede signals")
					}
					signals = append(signals, sig)
					if sig == tc.zombieOn {
						exact.Zombie = true
					}
					if sig == syscall.SIGTERM && tc.reused {
						if ticks {
							exact.StartTicks++
						} else {
							exact.StartedAt = exact.StartedAt.Add(time.Second)
						}
					}
					return nil
				}
				now := start
				err := stopRunnerForReplacementWithClock(root, runner, signal,
					func() time.Time { return now }, func(d time.Duration) { now = now.Add(d) }, probe)
				if tc.wantError {
					want := fmt.Sprintf("runner pid %d remained alive after replacement stop", runner.Pid)
					if err == nil || err.Error() != want {
						t.Fatalf("error = %v, want %q", err, want)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				wantSignals := []syscall.Signal{syscall.SIGTERM, syscall.SIGCONT}
				if tc.wantKill {
					wantSignals = append(wantSignals, syscall.SIGKILL)
				}
				if !reflect.DeepEqual(signals, wantSignals) || now.Sub(start) != tc.wantWait {
					t.Fatalf("signals = %v, wait = %s; want %v, %s", signals, now.Sub(start), wantSignals, tc.wantWait)
				}
				if _, alive := liveRunnerWithProber(root, probe); alive == tc.reused {
					t.Fatal("ordinary runner liveness must retain its identity and zombie semantics")
				}
			})
		}
	}
}

func TestReplacementStopAcceptsUnreapedChild(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(runnerDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	child := exec.Command("/bin/sh", "-c", "exit 0")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Wait()
	prober := identity.KernelProber{}
	deadline := time.Now().Add(5 * time.Second)
	var exact identity.Exact
	for {
		var state identity.Liveness
		var err error
		exact, state, err = prober.Probe(int64(child.Process.Pid))
		if err == nil && state == identity.Alive && exact.Zombie {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not become an observable zombie before collection")
		}
		time.Sleep(time.Millisecond)
	}
	runner := RunnerRecord{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), StartTicks: exact.StartTicks, BootID: exact.BootID}
	if err := writeJSONAtomic(runnerRecordPath(root), runner); err != nil {
		t.Fatal(err)
	}
	if err := stopRunnerForReplacement(root, runner, nil); err != nil {
		t.Fatal(err)
	}
	if _, alive := liveRunnerWithProber(root, prober); !alive {
		t.Fatal("child must remain observable until its parent collects it")
	}
}
