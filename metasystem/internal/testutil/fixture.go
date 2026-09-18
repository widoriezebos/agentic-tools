package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
  attempt_tag=
  [ -z "${METASYSTEM_FIXTURE_ATTEMPT-}" ] || attempt_tag="METASYSTEM_FIXTURE_ATTEMPT=$METASYSTEM_FIXTURE_ATTEMPT"
  if [ "${1-}" != "$tag" ]; then
    [ -z "$attempt_tag" ] || exec /bin/sh "$0" "$tag" "$attempt_tag" "$@"
    exec /bin/sh "$0" "$tag" "$@"
  fi
  if [ -n "$attempt_tag" ] && [ "${2-}" != "$attempt_tag" ]; then
    shift
    exec /bin/sh "$0" "$tag" "$attempt_tag" "$@"
  fi
  shift
  [ -z "$attempt_tag" ] || shift
fi
`

type fixtureTB interface {
	Cleanup(func())
	Errorf(string, ...any)
	Fatalf(string, ...any)
	Logf(string, ...any)
}

type ProcessFixture struct {
	t         fixtureTB
	key       identity.FixtureKey
	tag       string
	prober    identity.Prober
	signal    identity.SignalFunc
	refs      []identity.Ref
	scan      func(identity.FixtureKey) ([]identity.FixtureSurvivor, error)
	held      map[identity.Ref]bool
	released  map[identity.Ref]bool
	records   string
	leash     *os.File
	waitBound time.Duration
}

func Fixture(t testing.TB) *ProcessFixture {
	t.Helper()
	custodian, present := testenv.FixtureCustodian()
	fixture := newRecordedProcessFixture(t, t.Name(), t.TempDir, custodian, present, identity.KernelProber{}, syscall.Kill)
	fixture.records, _ = testenv.FixtureCustodianRecords()
	return fixture
}

func newRecordedProcessFixture(t fixtureTB, testName string, tempDir func() string, custodian identity.Ref, custodianPresent bool, prober identity.Prober, signal identity.SignalFunc) *ProcessFixture {
	fixture := makeProcessFixture(t, testName, custodian, custodianPresent, prober, signal)
	registered := false
	defer func() {
		if !registered {
			fixture.closeLeash()
		}
	}()
	encoded, err := identity.EncodeKey(fixture.key)
	if err != nil {
		t.Fatalf("encode process fixture ownership: %v", err)
	}
	record := filepath.Join(filepath.Dir(tempDir()), "fixture-owner")
	if err := os.WriteFile(record, []byte(encoded), 0o600); err != nil {
		t.Fatalf("write process fixture ownership record: %v", err)
	}
	registerProcessFixture(t, fixture)
	registered = true
	return fixture
}

func newProcessFixture(t fixtureTB, testName string, custodian identity.Ref, custodianPresent bool, prober identity.Prober, signal identity.SignalFunc) *ProcessFixture {
	fixture := makeProcessFixture(t, testName, custodian, custodianPresent, prober, signal)
	registerProcessFixture(t, fixture)
	return fixture
}

func makeProcessFixture(t fixtureTB, testName string, custodian identity.Ref, custodianPresent bool, prober identity.Prober, signal identity.SignalFunc) *ProcessFixture {
	if !custodianPresent {
		t.Fatalf("process fixture needs a custodian: this binary's TestMain must call testenv.Main")
	}
	custodianExact, custodianState, custodianErr := prober.Probe(custodian.Pid)
	custodianMatches := custodianState == identity.Alive && identity.SameIdentity(custodianExact, custodian)
	if custodianErr != nil || !custodianMatches || custodianExact.Zombie {
		t.Fatalf("process fixture needs a running custodian: state=%s same-identity=%t zombie=%t err=%v", custodianState, custodianMatches, custodianExact.Zombie, custodianErr)
	}
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
		held: make(map[identity.Ref]bool), released: make(map[identity.Ref]bool), leash: leash, waitBound: 5 * time.Second,
	}
	return fixture
}

func registerProcessFixture(t fixtureTB, fixture *ProcessFixture) {
	t.Cleanup(fixture.cleanup)
	testenv.RegisterFixtureKey(fixture.key)
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
		return strings.HasPrefix(entry, identity.FixtureOwnerEnv+"=") || strings.HasPrefix(entry, fixtureLeashEnv+"=") || strings.HasPrefix(entry, identity.FixtureCustodianEnv)
	})
	return append(environment, f.tag, fixtureLeashEnv+"="+f.leash.Name())
}

func (f *ProcessFixture) Shell(script string, args ...string) *exec.Cmd {
	return f.shell(os.Environ(), script, args...)
}

func (f *ProcessFixture) shell(base []string, script string, args ...string) *exec.Cmd {
	environment := f.Env(base)
	carriers := []string{f.tag}
	for _, entry := range environment {
		if strings.HasPrefix(entry, identity.FixtureAttemptEnv+"=") {
			carriers = append(carriers, entry)
			break
		}
	}
	commandArgs := append([]string{"-c", fmt.Sprintf("shift %d\n", len(carriers)) + script, "sh"}, carriers...)
	command := exec.Command("/bin/sh", append(commandArgs, args...)...)
	command.Env = environment
	return command
}

func (f *ProcessFixture) Record(pid int) { f.record(pid, false) }

func (f *ProcessFixture) Hold(pid int) { f.record(pid, true) }

// WaitForNoUnrecordedChildren waits until the fixture key names only children recorded by this fixture.
func (f *ProcessFixture) WaitForNoUnrecordedChildren(ctx context.Context) error {
	recorded := make(map[string]bool, len(f.refs))
	for _, ref := range f.refs {
		encoded, err := identity.EncodeRef(ref)
		if err != nil {
			return fmt.Errorf("encode recorded fixture child: %w", err)
		}
		recorded[encoded] = true
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		survivors, err := f.scan(f.key)
		if err != nil {
			return fmt.Errorf("scan process fixture survivors: %w", err)
		}
		unrecorded := make([]identity.FixtureSurvivor, 0, len(survivors))
		for _, survivor := range survivors {
			encoded, encodeErr := identity.EncodeRef(survivor.Ref)
			if encodeErr == nil && recorded[encoded] {
				continue
			}
			exact, state, probeErr := f.prober.Probe(survivor.Ref.Pid)
			if probeErr == nil && (state == identity.Dead || state == identity.Alive && (!identity.SameIdentity(exact, survivor.Ref) || exact.Zombie)) {
				continue
			}
			unrecorded = append(unrecorded, survivor)
		}
		if len(unrecorded) == 0 {
			return nil
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			parts := make([]string, 0, len(unrecorded))
			for _, survivor := range unrecorded {
				parts = append(parts, fmt.Sprintf("pid=%d exe=%q argv=%q", survivor.Ref.Pid, survivor.Exe, survivor.Argv))
			}
			return fmt.Errorf("unrecorded fixture children still running: %s: %w", strings.Join(parts, "; "), ctx.Err())
		}
	}
}

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
	f.appendRecord('+', exact.Ref())
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
		f.release(ref)
		return
	}
	if err != nil || state != identity.Alive {
		f.t.Errorf("child identity unproven at teardown: ref=%+v", ref)
		return
	}
	if exact.Zombie {
		f.release(ref)
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
	deadline := time.After(f.waitBound)
	for {
		var pending []identity.Ref
		for _, ref := range refs {
			exact, state, err := f.prober.Probe(ref.Pid)
			if err != nil || state == identity.Unknown || state == identity.Alive && identity.SameIdentity(exact, ref) && !exact.Zombie {
				pending = append(pending, ref)
			} else {
				f.release(ref)
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

func (f *ProcessFixture) release(ref identity.Ref) {
	if f.released[ref] {
		return
	}
	f.released[ref] = true
	f.appendRecord('-', ref)
}

func (f *ProcessFixture) appendRecord(operation byte, ref identity.Ref) {
	if f.records == "" {
		return
	}
	encoded, err := identity.EncodeRef(ref)
	if err != nil {
		f.t.Errorf("write process fixture custodian record: %v", err)
		return
	}
	file, err := os.OpenFile(f.records, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		f.t.Errorf("write process fixture custodian record: %v", err)
		return
	}
	line := string(operation) + encoded + "\n"
	written, writeErr := file.Write([]byte(line))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil || written != len(line) {
		f.t.Errorf("write process fixture custodian record: wrote %d of %d bytes: write=%v close=%v", written, len(line), writeErr, closeErr)
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
