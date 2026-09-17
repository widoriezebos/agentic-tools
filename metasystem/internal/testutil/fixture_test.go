package testutil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type recordingTB struct {
	cleanups []func()
	errs     []string
}

type fixtureProbeFunc func(int64) (identity.Exact, identity.Liveness, error)

func (f fixtureProbeFunc) Probe(pid int64) (identity.Exact, identity.Liveness, error) { return f(pid) }

func (*recordingTB) Logf(string, ...any)              {}
func (r *recordingTB) Fatalf(format string, a ...any) { panic(fmt.Sprintf(format, a...)) }
func (r *recordingTB) Cleanup(cleanup func())         { r.cleanups = append(r.cleanups, cleanup) }
func (r *recordingTB) Errorf(f string, a ...any)      { r.errs = append(r.errs, fmt.Sprintf(f, a...)) }

func TestFixtureWritesItsOwnershipRecord(t *testing.T) {
	fixture, record := Fixture(t), filepath.Join(filepath.Dir(t.TempDir()), "fixture-owner")
	data, err := os.ReadFile(record)
	failOnFixtureError(t, err)
	key, err := identity.ParseKey(string(data))
	got, gotErr := identity.EncodeKey(key)
	want, wantErr := identity.EncodeKey(fixture.Key())
	if err != nil || gotErr != nil || wantErr != nil || got != want {
		t.Fatalf("ownership record key = %+v, parse error %v, encoding errors %v, %v; want %+v", key, err, gotErr, wantErr, fixture.Key())
	}
}

func TestBinaryExitScanNamesAChildThatOutlivedItsTest(t *testing.T) {
	reader, writer, err := os.Pipe()
	failOnFixtureError(t, err)
	releaseReader, releaseWriter, err := os.Pipe()
	failOnFixtureError(t, err)
	defer func() { _ = writer.Close(); _ = releaseWriter.Close() }()
	outputReader, outputWriter, err := os.Pipe()
	failOnFixtureError(t, err)
	command := exec.Command(os.Args[0], "-test.run=^TestBinaryExitScanHelper$", "-test.count=1")
	command.Env, command.ExtraFiles, command.Stdout, command.Stderr = append(os.Environ(), "TESTUTIL_EXIT_SCAN_HELPER=1"), []*os.File{reader, releaseReader}, outputWriter, outputWriter
	failOnFixtureError(t, command.Start())
	_ = errors.Join(reader.Close(), releaseReader.Close(), outputWriter.Close())
	helper, _, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	failOnFixtureError(t, err)
	defer identity.SignalExact(identity.KernelProber{}, helper.Ref(), syscall.SIGKILL)
	_ = outputReader.SetReadDeadline(time.Now().Add(30 * time.Second))
	buffered := bufio.NewReader(outputReader)
	line, err := buffered.ReadString('\n')
	failOnFixtureError(t, err)
	pid, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64)
	failOnFixtureError(t, err)
	child, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probe helper child: state=%v err=%v", state, err)
	}
	defer identity.SignalExact(identity.KernelProber{}, child.Ref(), syscall.SIGKILL)
	failOnFixtureError(t, releaseWriter.Close())
	rest, readErr := io.ReadAll(buffered)
	if readErr != nil {
		t.Fatalf("read exit-scan helper output: %v; output=%q", readErr, line+string(rest))
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	select {
	case err = <-waited:
	case <-time.After(30 * time.Second):
		_ = identity.SignalExact(identity.KernelProber{}, helper.Ref(), syscall.SIGKILL)
		t.Fatal("exit-scan helper did not exit within 30 seconds")
	}
	if err == nil || !strings.Contains(line+string(rest), fmt.Sprintf("test=%q pid=%d ", "TestBinaryExitScanHelper", pid)) {
		t.Fatalf("helper error=%v output=%q", err, line+string(rest))
	}
	waitForFixtureExit(t, child.Ref())
}

func TestBinaryExitScanHelper(t *testing.T) {
	if os.Getenv("TESTUTIL_EXIT_SCAN_HELPER") == "" {
		t.Skip("helper process only")
	}
	input, release := os.NewFile(3, "exit-scan-input"), os.NewFile(4, "exit-scan-release")
	var fixture *ProcessFixture
	t.Cleanup(func() {
		command := fixture.Shell("trap '' TERM\nprintf '%d\\n' $$\nread -r _")
		stdout, _ := command.StdoutPipe()
		command.Stdin, command.Stderr = input, os.Stderr
		failOnFixtureError(t, command.Start())
		_ = stdout.(*os.File).SetReadDeadline(time.Now().Add(30 * time.Second))
		line, readErr := bufio.NewReader(stdout).ReadString('\n')
		failOnFixtureError(t, readErr)
		fmt.Fprint(os.Stdout, line)
		_, readErr = io.Copy(io.Discard, release)
		failOnFixtureError(t, readErr)
	})
	fixture = Fixture(t)
}

func TestShellPrologueCarriesTheTagIntoArgv(t *testing.T) {
	for _, tagged := range []bool{true, false} {
		t.Run(fmt.Sprint("tagged=", tagged), func(t *testing.T) {
			fixture := Fixture(t)
			script := filepath.Join(t.TempDir(), "fixture.sh")
			failOnFixtureError(t, os.WriteFile(script, []byte(ShellPrologue+"printf '%s|' \"$@\"; printf '\\n'\nread -r _ || :\n"), 0o700))
			args := []string{"a", "b c"}
			if !tagged {
				args = []string{fixture.tag, "a"}
			}
			command := exec.Command("/bin/sh", append([]string{script}, args...)...)
			command.Env = fixture.Env(os.Environ())
			if !tagged {
				command.Env = slices.DeleteFunc(command.Env, func(entry string) bool { return strings.HasPrefix(entry, identity.FixtureOwnerEnv+"=") })
			}
			input, _ := command.StdinPipe()
			output, _ := command.StdoutPipe()
			failOnFixtureError(t, command.Start())
			fixture.Record(command.Process.Pid)
			ref := fixture.refs[len(fixture.refs)-1]
			killFixtureProcessAtCleanup(t, fixture, command.Process, ref)
			_ = output.(*os.File).SetReadDeadline(time.Now().Add(30 * time.Second))
			line, err := bufio.NewReader(output).ReadString('\n')
			failOnFixtureError(t, err)
			want := strings.Join(args, "|") + "|\n"
			if line != want {
				t.Fatalf("arguments = %q, want %q", line, want)
			}
			exact, state, err := (identity.KernelProber{}).Probe(ref.Pid)
			if err != nil || state != identity.Alive || !identity.SameIdentity(exact, ref) {
				t.Fatalf("probe script: state=%v err=%v", state, err)
			}
			if tagged {
				key, carrier, ok := identity.FixtureTag(exact)
				got, gotErr := identity.EncodeKey(key)
				want, wantErr := identity.EncodeKey(fixture.Key())
				if !ok || gotErr != nil || wantErr != nil || got != want || carrier != identity.FixtureCarrierArgvWord {
					t.Fatalf("fixture tag = %+v, %q, %v, encoding errors %v, %v; want %+v", key, carrier, ok, gotErr, wantErr, fixture.Key())
				}
			} else if len(exact.Argv) < 3 || !slices.Equal(exact.Argv[len(exact.Argv)-3:], append([]string{script}, args...)) {
				t.Fatalf("untagged argv = %q", exact.Argv)
			}
			_ = input.Close()
			waited := make(chan error, 1)
			go func() { waited <- command.Wait() }()
			select {
			case err = <-waited:
				failOnFixtureError(t, err)
			case <-time.After(30 * time.Second):
				t.Fatal("script did not exit within 30 seconds")
			}
		})
	}
}

// Custodian-log assertions belong with the cleanup path that owns and retains
// that log; this witness proves the fixture's direct recorded-child cleanup.
func TestHelperFailingBeforeItsKillPointLeavesNoChild(t *testing.T) {
	for index, helperFailure := range []string{"helper write failed", "helper ownership mismatched", "helper did not observe exit"} {
		t.Run(helperFailure, func(t *testing.T) {
			prober, recorder := identity.KernelProber{}, &recordingTB{}
			fixture := newProcessFixture(recorder, t.Name(), prober, syscall.Kill)
			fixture.scan = noFixtureSurvivors
			command := fixture.Shell("trap '' TERM\nkill -STOP $$")
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = command.Process.Kill() }()
			var status syscall.WaitStatus
			if _, err := syscall.Wait4(command.Process.Pid, &status, syscall.WUNTRACED, nil); err != nil || status&0xff != 0x7f {
				t.Fatalf("child did not stop: status=%#x err=%v", status, err)
			}
			stopped, _, err := prober.Probe(int64(command.Process.Pid))
			if err != nil {
				t.Fatalf("probe stopped child: %v", err)
			}
			fixture.Record(command.Process.Pid)
			ref := fixture.refs[0]
			recorder.Errorf("%s", helperFailure)
			switch index {
			case 1:
				mismatch := ref
				if mismatch.StartTicks != 0 {
					mismatch.StartTicks++
				} else {
					mismatch.StartedAtUnixMicro++
				}
				if err := identity.SignalExact(prober, mismatch, syscall.SIGKILL); !errors.Is(err, identity.ErrGone) {
					t.Fatalf("helper mismatch check: %v", err)
				}
			case 2:
				if err := command.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				waitForFixtureZombie(t, prober, ref)
			}
			recorder.cleanups[0]()
			waited := make(chan error, 1)
			go func() { waited <- command.Wait() }()
			select {
			case <-waited:
			case <-time.After(30 * time.Second):
				_ = command.Process.Kill()
				<-waited
				t.Fatal("fixture child remained after cleanup")
			}
			failures := strings.Join(recorder.errs, "\n")
			if index == 2 {
				if failures != helperFailure {
					t.Fatalf("zombie failures = %q, want only %q", failures, helperFailure)
				}
				return
			}
			for _, want := range []string{"finished child found running at teardown", fmt.Sprintf("pid=%d", command.Process.Pid), fmt.Sprintf("exe=%q", stopped.Exe), `argv=["/bin/sh"`} {
				if !strings.Contains(failures, want) {
					t.Fatalf("failure text %q does not contain %q", failures, want)
				}
			}
		})
	}
}

func TestRecordedChildIsReprovedBeforeKill(t *testing.T) {
	owner, _, _ := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	first := owner
	first.Pid = 500
	second := first
	if second.StartTicks != 0 {
		second.StartTicks++
	} else {
		second.StartedAt = second.StartedAt.Add(time.Microsecond)
	}
	for _, test := range []struct {
		held                               bool
		sameProbes, wantSent, wantFailures int
	}{
		{false, 1, 0, 0}, {true, 1, 0, 0}, {false, 2, 0, 1}, {true, 2, 0, 0}, {false, 0, 1, 2},
	} {
		probes := 0
		recorder, sent := &recordingTB{}, 0
		prober := fixtureProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
			if pid == int64(os.Getpid()) {
				return owner, identity.Alive, nil
			}
			probes++
			if test.sameProbes > 0 && probes > test.sameProbes {
				return second, identity.Alive, nil
			}
			return first, identity.Alive, nil
		})
		fixture := newProcessFixture(recorder, t.Name(), prober, func(int, syscall.Signal) error { sent++; return nil })
		fixture.scan = func(identity.FixtureKey) ([]identity.FixtureSurvivor, error) { return nil, nil }
		if test.held {
			fixture.Hold(500)
		} else {
			fixture.Record(500)
		}
		recorder.cleanups[0]()
		failures := strings.Join(recorder.errs, "\n")
		if sent != test.wantSent || len(recorder.errs) != test.wantFailures ||
			test.wantFailures > 0 && !strings.Contains(failures, "finished child found running at teardown") ||
			test.wantFailures > 1 && !strings.Contains(failures, "child did not exit within five seconds") {
			t.Fatalf("case=%+v: sent=%d failures=%v", test, sent, recorder.errs)
		}
	}
	recorder, sent := &recordingTB{}, 0
	fixture := newProcessFixture(recorder, t.Name(), identity.KernelProber{}, func(int, syscall.Signal) error { sent++; return nil })
	fixture.scan = func(identity.FixtureKey) ([]identity.FixtureSurvivor, error) { return nil, nil }
	command := fixture.Shell("read -r _\nexit 0")
	stdin, _ := command.StdinPipe()
	failOnFixtureError(t, command.Start())
	fixture.Record(command.Process.Pid)
	_ = stdin.Close()
	failOnFixtureError(t, command.Wait())
	recorder.cleanups[0]()
	if sent != 0 || len(recorder.errs) != 0 {
		t.Fatalf("exited child: sent=%d failures=%v", sent, recorder.errs)
	}
}

func TestKeyScanNamesAnUnrecordedTaggedGrandchild(t *testing.T) {
	prober, recorder := identity.KernelProber{}, &recordingTB{}
	fixture := newProcessFixture(recorder, t.Name(), prober, syscall.Kill)
	reader, writer, err := os.Pipe()
	failOnFixtureError(t, err)
	releaseReader, releaseWriter, err := os.Pipe()
	failOnFixtureError(t, err)
	defer func() { _ = writer.Close(); _ = releaseReader.Close(); _ = releaseWriter.Close() }()
	ready := t.TempDir() + "/ready"
	command := fixture.Shell(`tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"
exec 4<&0
/bin/sh -c 'exec 3<&- 4<&-; trap "" TERM HUP; : >"$2"; read -r _' sh "$tag" "$1" <&4 >/dev/null 2>&1 &
printf '%d\n' "$!"
read -r _ <&3 || :`, ready)
	command.Stdin, command.ExtraFiles = reader, []*os.File{releaseReader}
	stdout, _ := command.StdoutPipe()
	failOnFixtureError(t, command.Start())
	_ = reader.Close()
	_ = releaseReader.Close()
	fixture.Record(command.Process.Pid)
	ref := fixture.refs[0]
	killFixtureProcessAtCleanup(t, fixture, command.Process, ref)
	var grandPID int
	_, err = fmt.Fscan(stdout, &grandPID)
	failOnFixtureError(t, err)
	waitForFixtureCondition(t, "fixture ready file creation", func() bool { _, err := os.Stat(ready); return err == nil })
	grand, state, err := prober.Probe(int64(grandPID))
	if err != nil || state != identity.Alive {
		t.Fatalf("grandchild is not alive: state=%v err=%v", state, err)
	}
	defer identity.SignalExact(prober, grand.Ref(), syscall.SIGKILL)
	failOnFixtureError(t, releaseWriter.Close())
	waitForFixtureExit(t, ref)
	failOnFixtureError(t, command.Wait())
	recorder.cleanups[0]()
	if !strings.Contains(strings.Join(recorder.errs, "\n"), fmt.Sprintf("pid=%d ", grandPID)) {
		t.Fatalf("cleanup failures do not name grandchild %d: %v", grandPID, recorder.errs)
	}
	waitForFixtureExit(t, grand.Ref())
}

func TestLeashedShellExitsWhenItsOwnerLetsGo(t *testing.T) {
	recorder, sent := &recordingTB{}, 0
	fixture := newProcessFixture(recorder, t.Name(), identity.KernelProber{}, func(pid int, sig syscall.Signal) error { sent++; return syscall.Kill(pid, sig) })
	fixture.scan = noFixtureSurvivors
	command, ref, release := startLeashedShell(t, fixture, true)
	// Close the leash before releasing the shell; on macOS, a reader entering read as the last writer closes can miss EOF.
	_ = fixture.leash.Close()
	_ = release.Close()
	waitForFixtureExit(t, ref)
	_, _ = command.Wait()
	recorder.cleanups[0]()
	if sent != 0 || len(recorder.errs) != 0 {
		t.Fatalf("sent=%d failures=%v", sent, recorder.errs)
	}
}

func TestCleanupIsScopedToTheFixtureKey(t *testing.T) {
	recorders := []*recordingTB{{}, {}}
	fixtures := []*ProcessFixture{newProcessFixture(recorders[0], t.Name(), identity.KernelProber{}, syscall.Kill), newProcessFixture(recorders[1], t.Name(), identity.KernelProber{}, syscall.Kill)}
	first, firstRef, _ := startLeashedShell(t, fixtures[0], false)
	second, secondRef, _ := startLeashedShell(t, fixtures[1], false)
	recorders[0].cleanups[0]()
	waitForFixtureExit(t, firstRef)
	_, _ = first.Wait()
	exact, state, err := (identity.KernelProber{}).Probe(secondRef.Pid)
	if err != nil || state != identity.Alive || !identity.SameIdentity(exact, secondRef) || exact.Zombie {
		t.Fatalf("second fixture child changed after first cleanup: state=%v err=%v", state, err)
	}
	recorders[1].cleanups[0]()
	waitForFixtureExit(t, secondRef)
	_, _ = second.Wait()
	if len(recorders[0].errs)+len(recorders[1].errs) != 0 {
		t.Fatalf("cleanup failures: %v %v", recorders[0].errs, recorders[1].errs)
	}
}

func startLeashedShell(t *testing.T, fixture *ProcessFixture, holdInput bool) (*os.Process, identity.Ref, io.WriteCloser) {
	t.Helper()
	command := fixture.Shell("trap '' TERM\nexec 3<\"${METASYSTEM_FIXTURE_LEASH:?}\"\nprintf 'ready\\n'\nread -r _\nread -r _ <&3")
	var release io.WriteCloser
	if holdInput {
		release, _ = command.StdinPipe()
	}
	stdout, _ := command.StdoutPipe()
	failOnFixtureError(t, command.Start())
	fixture.Hold(command.Process.Pid)
	ref := fixture.refs[len(fixture.refs)-1]
	killFixtureProcessAtCleanup(t, fixture, command.Process, ref)
	var ready string
	if _, err := fmt.Fscan(stdout, &ready); err != nil || ready != "ready" {
		t.Fatalf("leashed shell readiness = %q, %v", ready, err)
	}
	return command.Process, ref, release
}
func killFixtureProcessAtCleanup(t *testing.T, fixture *ProcessFixture, process *os.Process, ref identity.Ref) {
	t.Cleanup(fixture.closeLeash)
	t.Cleanup(func() { _ = identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL); _, _ = process.Wait() })
}
func waitForFixtureExit(t *testing.T, ref identity.Ref) {
	waitForFixtureCondition(t, fmt.Sprintf("fixture process %d exit", ref.Pid), func() bool {
		exact, state, _ := (identity.KernelProber{}).Probe(ref.Pid)
		return state == identity.Dead || state == identity.Alive && identity.SameIdentity(exact, ref) && exact.Zombie
	})
}

func waitForFixtureCondition(t *testing.T, outcome string, done func() bool) {
	t.Helper()
	ticker, deadline := time.NewTicker(10*time.Millisecond), time.After(30*time.Second)
	defer ticker.Stop()
	for !done() {
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("%s did not happen within 30 seconds", outcome)
		}
	}
}
func failOnFixtureError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func noFixtureSurvivors(identity.FixtureKey) ([]identity.FixtureSurvivor, error) { return nil, nil }
func waitForFixtureZombie(t *testing.T, prober identity.Prober, ref identity.Ref) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		exact, state, err := prober.Probe(ref.Pid)
		if err == nil && state == identity.Alive && identity.SameIdentity(exact, ref) && exact.Zombie {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("killed child did not become a zombie: state=%v err=%v", state, err)
		}
		<-time.After(10 * time.Millisecond)
	}
}
