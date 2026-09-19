package identity

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// The kernel prober is tested against real processes: ourselves (known
// argv), a child we spawn and reap (definitively dead), and pid
// boundaries. The three-way discipline is tested with a fake prober
// because only a fake can produce Unknown on demand.

func TestProbeSelf(t *testing.T) {
	prober := KernelProber{}
	self := int64(os.Getpid())
	exact, state, err := prober.Probe(self)
	if err != nil || state != Alive {
		t.Fatalf("probing self: state=%v err=%v", state, err)
	}
	if exact.Pid != self {
		t.Fatalf("wrong pid: %d", exact.Pid)
	}
	if exact.StartedAt.After(time.Now()) || exact.StartedAt.Before(time.Now().Add(-24*time.Hour)) {
		t.Fatalf("implausible start time: %v", exact.StartedAt)
	}
	if exact.StartedAt.Nanosecond() == 0 {
		t.Log("start time has whole-second resolution — sub-second exactness not observed (legal, but worth seeing)")
	}
	if len(exact.Argv) == 0 {
		t.Fatal("self argv unreadable")
	}
	if !exact.EnvironKnown || !exact.ExeKnown || exact.Exe == "" {
		t.Fatalf("self metadata unreadable: environ=%v exe=%q", exact.EnvironKnown, exact.Exe)
	}
}

func TestProbeReapedChildIsDead(t *testing.T) {
	command := exec.Command("/usr/bin/true")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	pid := int64(command.Process.Pid)
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	// The child is reaped: its pid no longer names a process (until
	// reuse, which a fresh test run will not hit in this window).
	_, state, err := KernelProber{}.Probe(pid)
	if state != Dead || err != nil {
		t.Fatalf("reaped child not definitively dead: state=%v err=%v", state, err)
	}
}

func TestProbeZombieKeepsItsExactLiveIdentity(t *testing.T) {
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", "read value")
	command.Stdin = readPipe
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_ = readPipe.Close()
	defer func() { _ = command.Process.Kill(); _, _ = command.Process.Wait() }()
	pid := int64(command.Process.Pid)
	running, state, err := (KernelProber{}).Probe(pid)
	if err != nil || state != Alive || running.Zombie {
		t.Fatalf("running child: state=%v zombie=%v err=%v", state, running.Zombie, err)
	}
	watch, err := armExitWatch(command.Process.Pid)
	if err != nil {
		t.Fatalf("arm child exit watch: %v", err)
	}
	_ = writePipe.Close()
	if err := watch.wait(); err != nil {
		t.Fatalf("wait for child exit: %v", err)
	}
	zombie, state, err := (KernelProber{}).Probe(pid)
	if err == nil && state == Dead {
		t.Fatalf("unreaped child probed dead: state=%v", state)
	}
	if state != Alive || err != nil || !SameIdentity(zombie, running.Ref()) {
		t.Fatalf("zombie lost its exact live identity: state=%v exact=%+v err=%v", state, zombie, err)
	}
	_ = command.Wait()
	if _, state, err := (KernelProber{}).Probe(pid); err != nil || state != Dead {
		t.Fatalf("waited child: state=%v err=%v", state, err)
	}
}

func TestProbeRejectsInvalidPid(t *testing.T) {
	if _, state, err := (KernelProber{}).Probe(0); state != Unknown || err == nil {
		t.Fatal("pid 0 must be Unknown with an error, never a verdict")
	}
}

func TestAliveRefSecondsComparison(t *testing.T) {
	prober := KernelProber{}
	self := int64(os.Getpid())
	exact, _, _ := prober.Probe(self)

	if got := AliveRef(prober, exact.Ref()); got != Alive {
		t.Fatalf("self must be alive: %v", got)
	}
	wrongSecond := Ref{Pid: self, StartedAtSec: exact.StartedAt.Unix() - 61}
	if got := AliveRef(prober, wrongSecond); got != Dead {
		t.Fatalf("a start-second mismatch is a DIFFERENT process, definitively dead: %v", got)
	}
}

type fakeProber struct {
	state Liveness
	err   error
	exact Exact
}

func (f fakeProber) Probe(int64) (Exact, Liveness, error) { return f.exact, f.state, f.err }

func TestAliveRefThreeWay(t *testing.T) {
	ref := Ref{Pid: 42, StartedAtSec: 100}
	cases := []struct {
		name string
		fake fakeProber
		want Liveness
	}{
		{"definitive absence", fakeProber{state: Dead}, Dead},
		{"read failure is unknown", fakeProber{state: Unknown, err: errors.New("eperm")}, Unknown},
		{"same second is alive", fakeProber{state: Alive, exact: Exact{Pid: 42, StartedAt: time.Unix(100, 500000)}}, Alive},
		{"other second is dead", fakeProber{state: Alive, exact: Exact{Pid: 42, StartedAt: time.Unix(101, 0)}}, Dead},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			if got := AliveRef(row.fake, ref); got != row.want {
				t.Fatalf("got %v want %v", got, row.want)
			}
		})
	}
}

func TestAliveTaggedRefBindsIdentityAndTagToOneProbe(t *testing.T) {
	ref := Ref{Pid: 41, StartedAtSec: 100, StartTicks: 700, BootID: "boot-a"}
	live := fakeProber{exact: Exact{
		Pid: 41, StartedAt: time.Unix(100, 0), StartTicks: 700, BootID: "boot-a",
		Argv: []string{"component", "instance-tag"}, ArgvKnown: true,
	}, state: Alive}
	if got := AliveTaggedRef(live, ref, "instance-tag"); got != Alive {
		t.Fatalf("matching tagged identity = %s, want alive", got)
	}
	if got := AliveTaggedRef(live, ref, "another-tag"); got != Dead {
		t.Fatalf("missing tag = %s, want dead", got)
	}
	prefixOnly := live
	prefixOnly.exact.Argv = []string{"component", "instance-tag-suffix"}
	if got := AliveTaggedRef(prefixOnly, ref, "instance-tag"); got != Dead {
		t.Fatalf("substring-only tag = %s, want dead", got)
	}
	reused := live
	reused.exact.StartTicks++
	if got := AliveTaggedRef(reused, ref, "instance-tag"); got != Dead {
		t.Fatalf("reused pid with copied tag = %s, want dead", got)
	}
	unreadable := live
	unreadable.exact.ArgvKnown = false
	if got := AliveTaggedRef(unreadable, ref, "instance-tag"); got != Unknown {
		t.Fatalf("unreadable argv = %s, want unknown", got)
	}
}

func TestAliveTaggedRefPreservesDefinitiveDeathAndProbeUncertainty(t *testing.T) {
	ref := Ref{Pid: 41, StartedAtSec: 100}
	if got := AliveTaggedRef(fakeProber{state: Dead}, ref, "component-tag"); got != Dead {
		t.Fatalf("definitive kernel absence = %s, want dead", got)
	}
	if got := AliveTaggedRef(fakeProber{state: Unknown, err: os.ErrPermission}, ref, "component-tag"); got != Unknown {
		t.Fatalf("unreadable process identity = %s, want unknown", got)
	}
}

func TestRefModeRejectsMixedAndPartialExactShapes(t *testing.T) {
	rows := []struct {
		name string
		ref  Ref
		want ComparisonMode
	}{
		{"empty", Ref{}, CompareInvalid},
		{"partial linux ticks", Ref{Pid: 1, StartedAtSec: 2, StartTicks: 3}, CompareInvalid},
		{"partial linux boot", Ref{Pid: 1, StartedAtSec: 2, BootID: "boot"}, CompareInvalid},
		{"mixed platforms", Ref{Pid: 1, StartedAtSec: 2, StartedAtUnixMicro: 2_000_001, StartTicks: 3, BootID: "boot"}, CompareInvalid},
		{"darwin exact", Ref{Pid: 1, StartedAtSec: 2, StartedAtUnixMicro: 2_000_001}, CompareDarwinMicroseconds},
		{"linux exact", Ref{Pid: 1, StartedAtSec: 2, StartTicks: 3, BootID: "boot"}, CompareLinuxTicksBootID},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			if got := row.ref.Mode(); got != row.want || row.ref.ModeName() != string(row.want) {
				t.Fatalf("mode=%s name=%q, want %s", got, row.ref.ModeName(), row.want)
			}
		})
	}
	exact := Exact{Pid: 1, StartedAt: time.UnixMicro(2_000_001)}
	if !SameIdentity(exact, rows[4].ref) {
		t.Fatal("SameIdentity rejected an equal exact identity")
	}
	if got := rows[4].ref.NativeExact(); got != (runtime.GOOS == "darwin") {
		t.Fatalf("Darwin ref NativeExact=%v on %s", got, runtime.GOOS)
	}
	if got := rows[5].ref.NativeExact(); got != (runtime.GOOS == "linux") {
		t.Fatalf("Linux ref NativeExact=%v on %s", got, runtime.GOOS)
	}
}

func TestLivenessStrings(t *testing.T) {
	if Alive.String() != "alive" || Dead.String() != "dead" || Unknown.String() != "unknown" {
		t.Fatal("liveness names drifted")
	}
}

func TestProbeLiveChildArgv(t *testing.T) {
	command := startReadyShell(t, nil)
	exact, state, err := KernelProber{}.Probe(int64(command.Process.Pid))
	if err != nil || state != Alive || len(exact.Argv) != 3 {
		t.Fatalf("child argv misread: %v (state %v err %v)", exact.Argv, state, err)
	}
	if exact.Argv[1] != "-c" {
		t.Fatalf("child argv misread: %v", exact.Argv)
	}
	if got := AliveRef(KernelProber{}, exact.Ref()); got != Alive {
		t.Fatalf("round-trip ref not alive: %v", got)
	}
}

func TestProbeRestrictedShellDoesNotClaimKnownEmptyEnvironment(t *testing.T) {
	command := startReadyShell(t, append(os.Environ(), "METASYSTEM_IDENTITY_TEST=known"))
	exact, state, err := (KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != Alive || !exact.ArgvKnown || len(exact.Argv) != 3 {
		t.Fatalf("restricted child argv unreadable: state=%v err=%v", state, err)
	}
	if exact.EnvironKnown && len(exact.Environ) == 0 {
		t.Fatal("restricted child environment reported as known empty")
	}
}

func startReadyShell(t *testing.T, environment []string) *exec.Cmd {
	t.Helper()
	readyReader, readyWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		_ = readyReader.Close()
		_ = readyWriter.Close()
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", "printf R >&3; read value; true")
	command.Stdin = stdinReader
	command.ExtraFiles = []*os.File{readyWriter}
	if environment != nil {
		command.Env = environment
	}
	if err := command.Start(); err != nil {
		_ = readyReader.Close()
		_ = readyWriter.Close()
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		t.Fatal(err)
	}
	_ = readyWriter.Close()
	_ = stdinReader.Close()
	t.Cleanup(func() {
		_ = stdinWriter.Close()
		_ = readyReader.Close()
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})
	// The shell writes the byte after its own exec.
	var ready [1]byte
	if _, err := io.ReadFull(readyReader, ready[:]); err != nil {
		t.Fatalf("read shell readiness byte: %v", err)
	}
	if ready[0] != 'R' {
		t.Fatalf("shell readiness byte=%q, want R", ready[0])
	}
	return command
}

// btimeShiftProber replays the btime-step kernel behavior: the same live
// process, constant ticks and boot id, but a start SECOND that walks as
// the realtime clock is stepped.
type btimeShiftProber struct{ shift int64 }

func (p btimeShiftProber) Probe(pid int64) (Exact, Liveness, error) {
	return Exact{
		Pid:        pid,
		StartedAt:  time.Unix(1786991670+p.shift, 0),
		StartTicks: 707949,
		BootID:     "boot-aaaa",
	}, Alive, nil
}

// A pair-bearing record survives clock drift; a legacy seconds-only
// record keeps the old (drift-exposed) semantics; and a genuinely
// different process — new boot or different ticks — still reads Dead.
func TestAliveRefClockStepImmunity(t *testing.T) {
	paired := Ref{Pid: 40723, StartedAtSec: 1786991670, StartTicks: 707949, BootID: "boot-aaaa"}
	legacy := Ref{Pid: 40723, StartedAtSec: 1786991670}
	for _, shift := range []int64{-4, -1, 0, 1, 4} {
		if got := AliveRef(btimeShiftProber{shift}, paired); got != Alive {
			t.Fatalf("paired ref at shift %d: want Alive, got %v", shift, got)
		}
	}
	if got := AliveRef(btimeShiftProber{0}, legacy); got != Alive {
		t.Fatalf("legacy ref, no drift: want Alive, got %v", got)
	}
	if got := AliveRef(btimeShiftProber{3}, legacy); got != Dead {
		t.Fatalf("legacy ref under drift keeps old semantics: want Dead, got %v", got)
	}
	rebooted := Ref{Pid: 40723, StartedAtSec: 1786991670, StartTicks: 707949, BootID: "boot-bbbb"}
	if got := AliveRef(btimeShiftProber{0}, rebooted); got != Dead {
		t.Fatalf("same ticks across a reboot: want Dead, got %v", got)
	}
	recycled := Ref{Pid: 40723, StartedAtSec: 1786991670, StartTicks: 1, BootID: "boot-aaaa"}
	if got := AliveRef(btimeShiftProber{0}, recycled); got != Dead {
		t.Fatalf("different ticks same boot: want Dead, got %v", got)
	}
}
