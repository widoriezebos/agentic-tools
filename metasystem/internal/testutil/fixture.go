package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

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
}

func Fixture(t testing.TB) *ProcessFixture {
	t.Helper()
	return newProcessFixture(t, t.Name(), identity.KernelProber{}, syscall.Kill)
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
	fixture := &ProcessFixture{t: t, key: key, tag: identity.FixtureOwnerEnv + "=" + encoded, prober: prober, signal: signal}
	t.Cleanup(fixture.cleanup)
	return fixture
}

func (f *ProcessFixture) Key() identity.FixtureKey { return f.key }

func (f *ProcessFixture) Env(base []string) []string {
	prefix := identity.FixtureOwnerEnv + "="
	environment := slices.DeleteFunc(append([]string(nil), base...), func(entry string) bool {
		return strings.HasPrefix(entry, prefix)
	})
	return append(environment, f.tag)
}

func (f *ProcessFixture) Shell(script string, args ...string) *exec.Cmd {
	command := exec.Command("/bin/sh", append([]string{"-c", "shift\n" + script, "sh", f.tag}, args...)...)
	command.Env = f.Env(os.Environ())
	return command
}

func (f *ProcessFixture) Record(pid int) {
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
}

func (f *ProcessFixture) cleanup() {
	for _, ref := range f.refs {
		f.stopRunningFinishedChild(ref)
	}
	f.waitForRecordedExits()
}

func (f *ProcessFixture) stopRunningFinishedChild(ref identity.Ref) {
	exact, state, err := f.prober.Probe(ref.Pid)
	if err == nil && (state == identity.Dead || state == identity.Alive && !identity.SameIdentity(exact, ref)) {
		return
	}
	if err != nil || state != identity.Alive {
		f.t.Errorf("finished child identity unproven at teardown: ref=%+v", ref)
		return
	}
	if exact.Zombie {
		return
	}
	signalErr := identity.SignalExact(f.prober, ref, syscall.SIGKILL, f.signal)
	if signalErr == identity.ErrUninspectable {
		f.t.Errorf("finished child identity unproven at signal: ref=%+v", ref)
	} else if signalErr != nil && signalErr != identity.ErrGone {
		f.t.Errorf("finished child signal failed for ref=%+v: %v", ref, signalErr)
	}
	f.t.Errorf("finished child found running at teardown: pid=%d exe=%q argv=%q", ref.Pid, exact.Exe, exact.Argv)
}

func (f *ProcessFixture) waitForRecordedExits() {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(5 * time.Second)
	for {
		var pending []identity.Ref
		for _, ref := range f.refs {
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
				f.t.Errorf("finished child did not exit within five seconds: ref=%+v", ref)
			}
			return
		}
	}
}
