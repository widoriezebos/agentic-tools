package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"golang.org/x/sys/unix"
)

const fixtureLeashEnv = "METASYSTEM_FIXTURE_LEASH"

const ShellPrologue = `if [ -n "${METASYSTEM_FIXTURE_OWNER-}" ]; then
  tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"
  [ "${1-}" = "$tag" ] || exec /bin/sh "$0" "$tag" "$@"
  shift
fi
`

type fixtureTB interface {
	Cleanup(func())
	Errorf(string, ...any)
	Fatalf(string, ...any)
	Logf(string, ...any)
}

type ProcessFixture struct {
	t      fixtureTB
	key    identity.FixtureKey
	tag    string
	prober identity.Prober
	signal identity.SignalFunc
	refs   []identity.Ref
	scan   func(identity.FixtureKey) ([]identity.FixtureSurvivor, error)
	held   map[identity.Ref]bool
	leash  *os.File
}

func Fixture(t testing.TB) *ProcessFixture {
	t.Helper()
	fixture := newProcessFixture(t, t.Name(), identity.KernelProber{}, syscall.Kill)
	encoded, err := identity.EncodeKey(fixture.key)
	if err != nil {
		t.Fatalf("encode process fixture ownership: %v", err)
	}
	record := filepath.Join(filepath.Dir(t.TempDir()), "fixture-owner")
	if err := os.WriteFile(record, []byte(encoded), 0o600); err != nil {
		t.Fatalf("write process fixture ownership record: %v", err)
	}
	return fixture
}

func newProcessFixture(t fixtureTB, testName string, prober identity.Prober, signal identity.SignalFunc) *ProcessFixture {
	owner, state, err := prober.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive || !owner.Ref().NativeExact() {
		t.Fatalf("create process fixture: owner identity is unproven: state=%v err=%v", state, err)
	}
	nonce := make([]byte, 4)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("create process fixture nonce: %v", err)
	}
	key := identity.FixtureKey{Owner: owner.Ref(), Test: testName, Nonce: hex.EncodeToString(nonce)}
	encoded, err := identity.EncodeKey(key)
	if err != nil {
		t.Fatalf("create process fixture for test name %q: %v", testName, err)
	}
	leash, err := openFixtureLeash()
	if err != nil {
		t.Fatalf("create process fixture leash: %v", err)
	}
	fixture := &ProcessFixture{
		t: t, key: key, tag: identity.FixtureOwnerEnv + "=" + encoded,
		prober: prober, signal: signal, scan: identity.FixtureSurvivors,
		held: make(map[identity.Ref]bool), leash: leash,
	}
	t.Cleanup(fixture.cleanup)
	testenv.RegisterFixtureKey(key)
	return fixture
}

func openFixtureLeash() (*os.File, error) {
	directory, err := os.MkdirTemp("", "metasystem-fixture-")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(directory, "leash")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		_ = os.RemoveAll(directory)
		return nil, err
	}
	reader, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err == nil {
		var writer int
		writer, err = unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		_ = unix.Close(reader)
		if err == nil {
			return os.NewFile(uintptr(writer), path), nil
		}
	}
	_ = os.RemoveAll(directory)
	return nil, err
}

func (f *ProcessFixture) Key() identity.FixtureKey { return f.key }

func (f *ProcessFixture) Env(base []string) []string {
	environment := slices.DeleteFunc(append([]string(nil), base...), func(entry string) bool {
		return strings.HasPrefix(entry, identity.FixtureOwnerEnv+"=") || strings.HasPrefix(entry, fixtureLeashEnv+"=")
	})
	return append(environment, f.tag, fixtureLeashEnv+"="+f.leash.Name())
}

func (f *ProcessFixture) Shell(script string, args ...string) *exec.Cmd {
	command := exec.Command("/bin/sh", append([]string{"-c", "shift\n" + script, "sh", f.tag}, args...)...)
	command.Env = f.Env(os.Environ())
	return command
}

func (f *ProcessFixture) Record(pid int) { f.record(pid, false) }

func (f *ProcessFixture) Hold(pid int) { f.record(pid, true) }

func (f *ProcessFixture) record(pid int, held bool) {
	exact, state, err := f.prober.Probe(int64(pid))
	if err == nil && state == identity.Dead {
		f.t.Logf("process fixture: pid %d exited before it could be recorded", pid)
		return
	}
	if err != nil || state != identity.Alive || !exact.Ref().NativeExact() {
		f.t.Errorf("process fixture: pid %d identity is unproven: state=%v err=%v", pid, state, err)
		return
	}
	f.refs = append(f.refs, exact.Ref())
	f.held[exact.Ref()] = held
}

func (f *ProcessFixture) cleanup() {
	defer f.closeLeash()
	for _, ref := range f.refs {
		f.stopRecordedChild(ref, f.held[ref])
	}
	f.waitForExits(f.refs)
	f.reapKeySurvivors()
}

func (f *ProcessFixture) stopRecordedChild(ref identity.Ref, held bool) {
	exact, state, err := f.prober.Probe(ref.Pid)
	if err == nil && (state == identity.Dead || state == identity.Alive && !identity.SameIdentity(exact, ref)) {
		return
	}
	if err != nil || state != identity.Alive {
		f.t.Errorf("child identity unproven at teardown: ref=%+v", ref)
		return
	}
	if exact.Zombie {
		return
	}
	signalErr := identity.SignalExact(f.prober, ref, syscall.SIGKILL, f.signal)
	if signalErr == identity.ErrUninspectable {
		f.t.Errorf("child identity unproven at signal: ref=%+v", ref)
	} else if signalErr != nil && signalErr != identity.ErrGone {
		f.t.Errorf("child signal failed for ref=%+v: %v", ref, signalErr)
	}
	if !held {
		f.t.Errorf("finished child found running at teardown: pid=%d exe=%q argv=%q", ref.Pid, exact.Exe, exact.Argv)
	}
}

func (f *ProcessFixture) waitForExits(refs []identity.Ref) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(5 * time.Second)
	for {
		var pending []identity.Ref
		for _, ref := range refs {
			exact, state, err := f.prober.Probe(ref.Pid)
			if err != nil || state == identity.Unknown || state == identity.Alive && identity.SameIdentity(exact, ref) && !exact.Zombie {
				pending = append(pending, ref)
			}
		}
		if len(pending) == 0 {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline:
			for _, ref := range pending {
				f.t.Errorf("child did not exit within five seconds: ref=%+v", ref)
			}
			return
		}
	}
}

func (f *ProcessFixture) reapKeySurvivors() {
	survivors, err := f.scan(f.key)
	if err != nil {
		f.t.Errorf("scan process fixture survivors: %v", err)
		return
	}
	var reaped []identity.Ref
	for _, survivor := range survivors {
		exact, state, probeErr := f.prober.Probe(survivor.Ref.Pid)
		if probeErr == nil && state == identity.Alive && identity.SameIdentity(exact, survivor.Ref) && exact.Zombie {
			continue
		}
		if survivor.Class != identity.FixtureSurvivorCertain {
			f.t.Errorf("fixture survivor ownership unproven at teardown: pid=%d exe=%q argv=%q", survivor.Ref.Pid, survivor.Exe, survivor.Argv)
			continue
		}
		signalErr := identity.SignalExact(f.prober, survivor.Ref, syscall.SIGKILL, f.signal)
		if signalErr == identity.ErrUninspectable {
			f.t.Errorf("fixture survivor identity unproven at signal: ref=%+v", survivor.Ref)
		} else if signalErr != nil && signalErr != identity.ErrGone {
			f.t.Errorf("fixture survivor signal failed for ref=%+v: %v", survivor.Ref, signalErr)
		}
		f.t.Errorf("unrecorded fixture child found running at teardown: pid=%d exe=%q argv=%q", survivor.Ref.Pid, survivor.Exe, survivor.Argv)
		reaped = append(reaped, survivor.Ref)
	}
	f.waitForExits(reaped)
}

func (f *ProcessFixture) closeLeash() {
	_ = f.leash.Close()
	_ = os.RemoveAll(filepath.Dir(f.leash.Name()))
}
