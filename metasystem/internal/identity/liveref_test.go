package identity

import (
	"os/exec"
	"testing"
	"time"
)

// LiveRef answers whether a recorded process still holds what it held. A
// zombie has exited and only awaits its parent's reap: it holds no lock,
// lease or file a liveness check protects, so it reads Dead there, while
// AliveRef keeps reporting it Alive for the callers that wait on the reap
// itself (a launcher's watchdog). Everything else classifies as AliveRef does.
func TestLiveRefClassifiesAZombieAsDead(t *testing.T) {
	ref := Ref{Pid: 42, StartedAtSec: 100}
	for _, test := range []struct {
		name string
		fake fakeProber
		want Liveness
	}{
		{"running", fakeProber{exact: Exact{Pid: 42, StartedAt: time.Unix(100, 0)}, state: Alive}, Alive},
		{"zombie", fakeProber{exact: Exact{Pid: 42, StartedAt: time.Unix(100, 0), Zombie: true, Exiting: true}, state: Alive}, Dead},
		{"reused pid", fakeProber{exact: Exact{Pid: 42, StartedAt: time.Unix(160, 0)}, state: Alive}, Dead},
		{"gone", fakeProber{state: Dead}, Dead},
		{"uninspectable", fakeProber{state: Unknown}, Unknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := LiveRef(test.fake, ref); got != test.want {
				t.Fatalf("LiveRef = %s, want %s", got, test.want)
			}
		})
	}
	zombie := fakeProber{exact: Exact{Pid: 42, StartedAt: time.Unix(100, 0), Zombie: true}, state: Alive}
	if got := AliveRef(zombie, ref); got != Alive {
		t.Fatalf("AliveRef of a zombie = %s, want alive (unchanged)", got)
	}
}

// The same classification against the kernel: an exited child its parent
// has not reaped reads Dead through LiveRef on this host's prober.
func TestLiveRefReadsAnUnreapedChildAsDead(t *testing.T) {
	t.Parallel()
	command := exec.Command("/bin/cat")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	reaped := false
	t.Cleanup(func() {
		_ = stdin.Close()
		if !reaped {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	running, state, err := (KernelProber{}).ReadStart(int64(command.Process.Pid))
	if err != nil || state != Alive {
		t.Fatalf("running child probe state=%s err=%v", state, err)
	}
	ref := running.Ref()
	if got := LiveRef(KernelProber{}, ref); got != Alive {
		t.Fatalf("LiveRef of the running child = %s, want alive", got)
	}
	watch, err := armExitWatch(command.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		watch.close()
		t.Fatal(err)
	}
	if err := watch.wait(); err != nil {
		watch.close()
		t.Fatal(err)
	}
	watch.close()
	if got := LiveRef(KernelProber{}, ref); got != Dead {
		exact, state, _ := (KernelProber{}).ReadStart(int64(command.Process.Pid))
		t.Fatalf("LiveRef of the unreaped child = %s, want dead (probe state=%s zombie=%t)", got, state, exact.Zombie)
	}
	_ = command.Wait()
	reaped = true
}
