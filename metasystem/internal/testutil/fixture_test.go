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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"golang.org/x/sys/unix"
)

type recordingTB struct {
	cleanups []func()
	errs     []string
}

var errRecordingFatal = errors.New("recording Fatalf")

type fixtureProbeFunc func(int64) (identity.Exact, identity.Liveness, error)

func (f fixtureProbeFunc) Probe(pid int64) (identity.Exact, identity.Liveness, error) { return f(pid) }

func (*recordingTB) Logf(string, ...any) {}
func (*recordingTB) Helper()             {}
func (r *recordingTB) Fatalf(format string, a ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, a...))
	panic(errRecordingFatal)
}
func (r *recordingTB) Cleanup(cleanup func())    { r.cleanups = append(r.cleanups, cleanup) }
func (r *recordingTB) Errorf(f string, a ...any) { r.errs = append(r.errs, fmt.Sprintf(f, a...)) }

func runningBinaryCustodian() identity.Ref { ref, _ := testenv.FixtureCustodian(); return ref }

func TestFixtureRefusesWithoutARunningCustodian(t *testing.T) {
	owner, custodian := fixtureTeardownExact(int64(os.Getpid()), 1), fixtureTeardownExact(int64(os.Getpid())+1000, 2)
	mismatch, zombie := custodian, custodian
	mismatch.StartedAt, mismatch.StartTicks, zombie.Zombie = mismatch.StartedAt.Add(time.Microsecond), mismatch.StartTicks+1, true
	for index, want := range []string{"process fixture needs a custodian: this binary's TestMain must call testenv.Main", "process fixture needs a running custodian: state=dead same-identity=false zombie=false err=<nil>", "process fixture needs a running custodian: state=alive same-identity=false zombie=false err=<nil>", "process fixture needs a running custodian: state=alive same-identity=true zombie=true err=<nil>", ""} {
		recorder := &recordingTB{}
		prober := fixtureProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
			if pid != custodian.Pid {
				return owner, identity.Alive, nil
			}
			return []identity.Exact{{}, {}, mismatch, zombie, custodian}[index], []identity.Liveness{identity.Dead, identity.Dead, identity.Alive, identity.Alive, identity.Alive}[index], nil
		})
		fixture, recovered := (*ProcessFixture)(nil), any(nil)
		func() {
			defer func() { recovered = recover() }()
			fixture = newProcessFixture(recorder, t.Name(), custodian.Ref(), index != 0, prober, syscall.Kill)
		}()
		if want != "" {
			if recovered != errRecordingFatal || !slices.Equal(recorder.errs, []string{want}) {
				t.Fatalf("panic=%v fatals=%q, want sentinel and %q", recovered, recorder.errs, want)
			}
			continue
		}
		if recovered != nil || len(recorder.errs) != 0 || fixture == nil || fixture.Key().Nonce == "" {
			t.Fatalf("alive custodian: fixture=%v panic=%v fatals=%q", fixture, recovered, recorder.errs)
		}
		fixture.closeLeash()
	}
}

func TestCustodianVariablesNeverReachFixtureChildren(t *testing.T) {
	leash := os.Stdin
	fixture := &ProcessFixture{tag: identity.FixtureOwnerEnv + "=fixture", leash: leash}
	base := []string{identity.FixtureCustodianEnv + "=1", identity.FixtureCustodianOwnerEnv + "=owner", identity.FixtureCustodianLogEnv + "=log", identity.FixtureCustodianChainEnv + "=chain", identity.FixtureCustodianRecordsEnv + "=records", identity.FixtureCustodianEnv + "_UNKNOWN=value", identity.FixtureOwnerEnv + "=inherited", "KEEP=value"}
	transforms := []func([]string) []string{testenv.WithoutInheritedControls, fixture.Env}
	wants := [][]string{{identity.FixtureOwnerEnv + "=inherited", "KEEP=value"}, {"KEEP=value", fixture.tag, fixtureLeashEnv + "=" + leash.Name()}}
	for index, transform := range transforms {
		if got := transform(base); !slices.Equal(got, wants[index]) {
			t.Fatalf("child environment %d = %v, want %v", index, got, wants[index])
		}
	}
}

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

func TestCensusFindsAnUntaggedExecutableByRecord(t *testing.T) {
	if os.Getenv("TESTUTIL_UNTAGGED_EXECUTABLE_HELPER") != "" {
		ready, release := os.NewFile(4, "fixture-copy-ready"), os.NewFile(3, "fixture-copy-release")
		custodian, _ := testenv.FixtureCustodian()
		encoded, err := identity.EncodeRef(custodian)
		logPath := fmt.Sprintf("%s.custodian-%d.log", os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), os.Getpid())
		_, writeErr := fmt.Fprintf(ready, "%s\t%s\n", encoded, logPath)
		failOnFixtureError(t, errors.Join(err, writeErr, ready.Close()))
		_, err = io.Copy(io.Discard, release)
		failOnFixtureError(t, err)
		return
	}
	root := t.TempDir()
	for _, layout := range []struct {
		name  string
		goTmp bool
	}{{"plain temporary root", false}, {"go-tmp under TMPDIR", true}} {
		t.Run(layout.name, func(t *testing.T) {
			t.Setenv("TMPDIR", root)
			if layout.goTmp {
				goTmp := filepath.Join(root, "go-tmp")
				failOnFixtureError(t, os.MkdirAll(goTmp, 0o700))
				t.Setenv("GOTMPDIR", goTmp)
			} else {
				unsetFixtureEnvironment(t, "GOTMPDIR")
			}
			fixture := Fixture(t)
			positiveDirectory := t.TempDir()
			outsideDirectory, err := os.MkdirTemp(root, "outside-")
			failOnFixtureError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(outsideDirectory) })
			wrongDirectory, err := os.MkdirTemp(root, "WrongDirectory-")
			failOnFixtureError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(wrongDirectory) })
			record, err := os.ReadFile(filepath.Join(filepath.Dir(positiveDirectory), "fixture-owner"))
			failOnFixtureError(t, err)
			failOnFixtureError(t, os.WriteFile(filepath.Join(wrongDirectory, "fixture-owner"), record, 0o600))

			positive := startUntaggedFixtureCopy(t, fixture, positiveDirectory)
			outside := startUntaggedFixtureCopy(t, fixture, outsideDirectory)
			wrong := startUntaggedFixtureCopy(t, fixture, wrongDirectory)
			got, err := identity.FixtureSurvivors(fixture.Key())
			if err != nil || len(got) != 1 {
				t.Fatalf("fixture scan = %#v, %v; want copied binary alone", got, err)
			}
			custodianRef := encodeFixtureRef(t, positive.custodian)
			if slices.ContainsFunc(got, func(survivor identity.FixtureSurvivor) bool {
				return encodeFixtureRef(t, survivor.Ref) == custodianRef
			}) {
				t.Fatalf("fixture scan = %#v; custodian %+v must be absent", got, positive.custodian)
			}
			gotKey, gotKeyErr := identity.EncodeKey(got[0].Key)
			wantKey, wantKeyErr := identity.EncodeKey(fixture.Key())
			if got[0].Ref.Pid != positive.exact.Pid || got[0].Exe != positive.exact.Exe ||
				got[0].Class != identity.FixtureSurvivorCertain || got[0].Carrier != identity.FixtureCarrierRecord ||
				gotKeyErr != nil || wantKeyErr != nil || gotKey != wantKey {
				t.Fatalf("fixture scan = %#v; key encodings %q/%q errors %v/%v; want record-backed pid %d exe %q without custodian %+v", got, gotKey, wantKey, gotKeyErr, wantKeyErr, positive.exact.Pid, positive.exact.Exe, positive.custodian)
			}
			if state := identity.AliveRef(identity.KernelProber{}, positive.custodian); state != identity.Alive {
				t.Fatalf("custodian %+v state after fixture scan = %s, want alive", positive.custodian, state)
			}
			for _, processCopy := range []*untaggedFixtureCopy{positive, outside, wrong} {
				failOnFixtureError(t, processCopy.release.Close())
				failOnFixtureError(t, processCopy.command.Wait())
			}
		})
	}
}

func TestFixtureCleanupReadsOwnershipRecordBeforeTempDirRemoval(t *testing.T) {
	recorder := &recordingTB{}
	perTestDirectory, err := os.MkdirTemp("/tmp", t.Name())
	failOnFixtureError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(perTestDirectory) })
	tempDirectory := filepath.Join(perTestDirectory, "001")
	newRecordedProcessFixture(recorder, t.Name(), func() string {
		failOnFixtureError(t, os.MkdirAll(tempDirectory, 0o700))
		recorder.Cleanup(func() { _ = os.RemoveAll(perTestDirectory) })
		return tempDirectory
	}, runningBinaryCustodian(), true, identity.KernelProber{}, syscall.Kill)
	processCopy := startUntaggedFixtureCopy(t, nil, tempDirectory)
	for index := len(recorder.cleanups) - 1; index >= 0; index-- {
		recorder.cleanups[index]()
	}
	_ = processCopy.release.Close()
	_, _ = processCopy.command.Process.Wait()
	failures := strings.Join(recorder.errs, "\n")
	for _, want := range []string{"unrecorded fixture child found running at teardown", fmt.Sprintf("pid=%d", processCopy.exact.Pid), fmt.Sprintf("exe=%q", processCopy.exact.Exe)} {
		if !strings.Contains(failures, want) {
			t.Fatalf("cleanup failures %q do not contain %q", failures, want)
		}
	}
}

type untaggedFixtureCopy struct {
	command   *exec.Cmd
	exact     identity.Exact
	custodian identity.Ref
	logPath   string
	release   *os.File
}

func startUntaggedFixtureCopy(t *testing.T, fixture *ProcessFixture, directory string) *untaggedFixtureCopy {
	t.Helper()
	binary := filepath.Join(directory, "fixture-copy")
	data, err := os.ReadFile(os.Args[0])
	failOnFixtureError(t, err)
	failOnFixtureError(t, os.WriteFile(binary, data, 0o700))
	releaseReader, releaseWriter, err := os.Pipe()
	failOnFixtureError(t, err)
	readyReader, readyWriter, err := os.Pipe()
	failOnFixtureError(t, err)
	command := exec.Command(binary, "-test.run=^TestCensusFindsAnUntaggedExecutableByRecord$", "-test.count=1")
	command.Env = append(slices.DeleteFunc(os.Environ(), func(entry string) bool {
		return strings.HasPrefix(entry, identity.FixtureOwnerEnv+"=") || strings.HasPrefix(entry, "TESTUTIL_UNTAGGED_EXECUTABLE_HELPER=")
	}), "TESTUTIL_UNTAGGED_EXECUTABLE_HELPER=1")
	command.ExtraFiles = []*os.File{releaseReader, readyWriter}
	failOnFixtureError(t, command.Start())
	_ = errors.Join(releaseReader.Close(), readyWriter.Close())
	processCopy := &untaggedFixtureCopy{command: command, release: releaseWriter}
	t.Cleanup(func() {
		if processCopy.exact.Ref().NativeExact() {
			_ = identity.SignalExact(identity.KernelProber{}, processCopy.exact.Ref(), syscall.SIGKILL)
		} else {
			_ = command.Process.Kill()
		}
		_, _ = command.Process.Wait()
		processCopy.finishCustodianLog(t)
	})
	ready, err := bufio.NewReader(readyReader).ReadString('\n')
	_ = readyReader.Close()
	failOnFixtureError(t, err)
	encodedCustodian, logPath, found := strings.Cut(strings.TrimSpace(ready), "\t")
	if !found {
		t.Fatalf("fixture copy readiness %q has no custodian log path", ready)
	}
	processCopy.custodian, err = identity.ParseRef(encodedCustodian)
	failOnFixtureError(t, err)
	processCopy.logPath = logPath
	var state identity.Liveness
	processCopy.exact, state, err = (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != identity.Alive || !processCopy.exact.Ref().NativeExact() || !processCopy.exact.ExeKnown {
		t.Fatalf("probe fixture copy: state=%v exact=%+v err=%v", state, processCopy.exact, err)
	}
	if fixture != nil {
		fixture.Hold(int(processCopy.exact.Pid))
		fixture.Hold(int(processCopy.custodian.Pid))
		custodianRef := encodeFixtureRef(t, processCopy.custodian)
		if !slices.ContainsFunc(fixture.refs, func(ref identity.Ref) bool { return encodeFixtureRef(t, ref) == custodianRef }) {
			t.Fatalf("fixture records %v do not contain custodian %+v", fixture.refs, processCopy.custodian)
		}
	}
	return processCopy
}

func (p *untaggedFixtureCopy) finishCustodianLog(t *testing.T) {
	if p.logPath == "" {
		return
	}
	fd, err := unix.Open(p.logPath, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if os.IsNotExist(err) {
		return
	}
	failOnFixtureError(t, err)
	log := os.NewFile(uintptr(fd), p.logPath)
	defer log.Close()
	failOnFixtureError(t, unix.Flock(fd, unix.LOCK_EX))
	contents, err := io.ReadAll(log)
	failOnFixtureError(t, err)
	if string(contents) != "" && string(contents) != identity.FixtureCustodianCompletionLine(p.exact.Ref()) {
		return
	}
	logInfo, logErr := log.Stat()
	pathInfo, pathErr := os.Lstat(p.logPath)
	if logErr == nil && pathErr == nil && pathInfo.Mode().IsRegular() && os.SameFile(logInfo, pathInfo) {
		failOnFixtureError(t, os.Remove(p.logPath))
	}
}

func encodeFixtureRef(t *testing.T, ref identity.Ref) string {
	t.Helper()
	encoded, err := identity.EncodeRef(ref)
	failOnFixtureError(t, err)
	return encoded
}

func unsetFixtureEnvironment(t *testing.T, name string) {
	t.Helper()
	value, present := os.LookupEnv(name)
	failOnFixtureError(t, os.Unsetenv(name))
	t.Cleanup(func() {
		if present {
			_ = os.Setenv(name, value)
		} else {
			_ = os.Unsetenv(name)
		}
	})
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
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	failOnFixtureError(t, command.Start())
	_ = errors.Join(reader.Close(), releaseReader.Close(), outputWriter.Close())
	helper, _, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	failOnFixtureError(t, err)
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = identity.SignalExact(identity.KernelProber{}, helper.Ref(), syscall.SIGKILL)
			_, _ = command.Process.Wait()
		}
	})
	helperGroup, err := syscall.Getpgid(command.Process.Pid)
	failOnFixtureError(t, err)
	if helperGroup != command.Process.Pid {
		t.Fatalf("exit-scan helper group = %d, want leader %d", helperGroup, command.Process.Pid)
	}
	buffered := bufio.NewReader(outputReader)
	line, err := buffered.ReadString('\n')
	failOnFixtureError(t, err)
	pidText, childEnv, _ := strings.Cut(strings.TrimSpace(line), "|")
	pid, err := strconv.ParseInt(pidText, 10, 64)
	failOnFixtureError(t, err)
	child, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("probe helper child: state=%v err=%v", state, err)
	}
	t.Cleanup(func() {
		_ = identity.SignalExact(identity.KernelProber{}, child.Ref(), syscall.SIGKILL)
		waitForFixtureExit(t, child.Ref(), fixtureProcessWaitBound(t))
	})
	if childEnv != "" {
		t.Fatalf("custodian variable reached live fixture child: %s", childEnv)
	}
	failOnFixtureError(t, writer.Close())
	failOnFixtureError(t, releaseWriter.Close())
	rest, readErr := io.ReadAll(buffered)
	if readErr != nil {
		t.Fatalf("read exit-scan helper output: %v; output=%q", readErr, line+string(rest))
	}
	err = command.Wait()
	if err != nil {
		t.Fatalf("helper error=%v output=%q", err, line+string(rest))
	}
	waitForFixtureExit(t, child.Ref(), fixtureProcessWaitBound(t))
	if err := syscall.Kill(-helperGroup, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("helper process group %d still exists: %v", helperGroup, err)
	}
}

func TestBinaryExitScanHelper(t *testing.T) {
	if os.Getenv("TESTUTIL_EXIT_SCAN_HELPER") == "" {
		return
	}
	input, release := os.NewFile(3, "exit-scan-input"), os.NewFile(4, "exit-scan-release")
	var fixture *ProcessFixture
	t.Cleanup(func() {
		defer fixture.closeLeash()
		command := fixture.Shell("trap '' TERM\nprintf '%d|' $$\nenv | sed -n 's/^\\(METASYSTEM_FIXTURE_CUSTODIAN[^=]*\\)=.*/\\1/p'\nprintf '\\n'\nread -r _\nexec /usr/bin/tail -f /dev/null")
		stdout, _ := command.StdoutPipe()
		command.Stdin, command.Stderr = input, os.Stderr
		failOnFixtureError(t, command.Start())
		line, readErr := bufio.NewReader(stdout).ReadString('\n')
		failOnFixtureError(t, readErr)
		fmt.Fprint(os.Stdout, line)
		_, readErr = io.Copy(io.Discard, release)
		failOnFixtureError(t, readErr)
		failOnFixtureError(t, input.Close())
		child, state, probeErr := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
		if probeErr != nil || state != identity.Alive {
			t.Fatalf("probe exit-scan child for cleanup: state=%s err=%v", state, probeErr)
		}
		signalErr := identity.SignalExact(identity.KernelProber{}, child.Ref(), syscall.SIGKILL)
		if signalErr != nil && signalErr != identity.ErrGone {
			t.Fatalf("signal exit-scan child: %v", signalErr)
		}
		waitForFixtureExit(t, child.Ref(), fixture.waitBound)
		if waitErr := command.Wait(); waitErr == nil {
			t.Fatal("exit-scan child ignored SIGKILL")
		}
	})
	custodian, present := testenv.FixtureCustodian()
	fixture = makeProcessFixture(t, t.Name(), custodian, present, identity.KernelProber{}, syscall.Kill)
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
			failOnFixtureError(t, command.Wait())
		})
	}
	t.Run("proof attempt tag", func(t *testing.T) {
		t.Parallel()

		fixture := Fixture(t)
		base := append(os.Environ(), identity.FixtureAttemptEnv+"=attempt-a")
		command := fixture.shell(base, "printf '%s|' \"$@\"; printf '\\n'; read -r _ || :", "a", "b c")
		input, _ := command.StdinPipe()
		output, _ := command.StdoutPipe()
		failOnFixtureError(t, command.Start())
		fixture.Record(command.Process.Pid)
		ref := fixture.refs[len(fixture.refs)-1]
		killFixtureProcessAtCleanup(t, fixture, command.Process, ref)
		line, err := bufio.NewReader(output).ReadString('\n')
		failOnFixtureError(t, err)
		if line != "a|b c|\n" {
			t.Fatalf("arguments = %q, want user arguments only", line)
		}
		exact, state, err := (identity.KernelProber{}).Probe(ref.Pid)
		if err != nil || state != identity.Alive || !slices.Contains(exact.Argv, identity.FixtureAttemptEnv+"=attempt-a") {
			t.Fatalf("proof attempt carrier = argv %q state=%s err=%v", exact.Argv, state, err)
		}
		failOnFixtureError(t, input.Close())
		failOnFixtureError(t, command.Wait())
	})
}

// Custodian-log assertions belong with the cleanup path that owns and retains
// that log; this witness proves the fixture's direct recorded-child cleanup.
func TestHelperFailingBeforeItsKillPointLeavesNoChild(t *testing.T) {
	for index, helperFailure := range []string{"helper write failed", "helper ownership mismatched", "helper did not observe exit"} {
		t.Run(helperFailure, func(t *testing.T) {
			prober, recorder := identity.KernelProber{}, &recordingTB{}
			fixture := newProcessFixture(recorder, t.Name(), runningBinaryCustodian(), true, prober, syscall.Kill)
			fixture.scan = noFixtureSurvivors
			command := fixture.Shell("trap '' TERM\nkill -STOP $$")
			output, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
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
				waitForFixtureZombie(t, output, prober, ref, fixture.waitBound)
			}
			recorder.cleanups[0]()
			_ = command.Wait()
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
		fixture := newProcessFixture(recorder, t.Name(), owner.Ref(), true, prober, func(int, syscall.Signal) error { sent++; return nil })
		fixture.scan = func(identity.FixtureKey) ([]identity.FixtureSurvivor, error) { return nil, nil }
		fixture.waitBound = time.Millisecond
		if test.held {
			fixture.Hold(500)
		} else {
			fixture.Record(500)
		}
		recorder.cleanups[0]()
		failures := strings.Join(recorder.errs, "\n")
		if sent != test.wantSent || len(recorder.errs) != test.wantFailures ||
			test.wantFailures > 0 && !strings.Contains(failures, "finished child found running at teardown") ||
			test.wantFailures > 1 && !strings.Contains(failures, "child did not exit after") {
			t.Fatalf("case=%+v: sent=%d failures=%v", test, sent, recorder.errs)
		}
	}
	recorder, sent := &recordingTB{}, 0
	fixture := newProcessFixture(recorder, t.Name(), runningBinaryCustodian(), true, identity.KernelProber{}, func(int, syscall.Signal) error { sent++; return nil })
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

func TestFixtureRecordsAndReleasesEachChild(t *testing.T) {
	owner := fixtureTeardownExact(int64(os.Getpid()), 1)
	child := fixtureTeardownExact(500, 2)
	encoded, err := identity.EncodeRef(child.Ref())
	failOnFixtureError(t, err)
	for _, test := range []struct {
		name        string
		hold, stays bool
		wantAfter   string
	}{
		{"recorded child already gone", false, false, "+" + encoded + "\n-" + encoded + "\n"},
		{"held child killed", true, false, "+" + encoded + "\n-" + encoded + "\n"},
		{"child remains live", true, true, "+" + encoded + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			childProbes, childAlive := 0, true
			prober := fixtureProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
				if pid == owner.Pid {
					return owner, identity.Alive, nil
				}
				childProbes++
				if !childAlive || !test.hold && childProbes > 1 {
					return identity.Exact{}, identity.Dead, nil
				}
				return child, identity.Alive, nil
			})
			recorder := &recordingTB{}
			fixture := makeProcessFixture(recorder, t.Name(), owner.Ref(), true, prober, func(int, syscall.Signal) error {
				if !test.stays {
					childAlive = false
				}
				return nil
			})
			recorder.Cleanup(fixture.cleanup)
			fixture.scan = noFixtureSurvivors
			fixture.waitBound = time.Millisecond
			fixture.records = filepath.Join(t.TempDir(), "records")
			failOnFixtureError(t, os.WriteFile(fixture.records, nil, 0o600))
			if test.hold {
				fixture.Hold(500)
			} else {
				fixture.Record(500)
			}
			contents, err := os.ReadFile(fixture.records)
			failOnFixtureError(t, err)
			if want := "+" + encoded + "\n"; string(contents) != want {
				t.Fatalf("record before cleanup = %q, want %q", contents, want)
			}
			recorder.cleanups[0]()
			contents, err = os.ReadFile(fixture.records)
			failOnFixtureError(t, err)
			if string(contents) != test.wantAfter {
				t.Fatalf("record after cleanup = %q, want %q; failures=%v", contents, test.wantAfter, recorder.errs)
			}
		})
	}
}
func TestKeyScanNamesAnUnrecordedTaggedGrandchild(t *testing.T) {
	prober, recorder := identity.KernelProber{}, &recordingTB{}
	fixture := newProcessFixture(recorder, t.Name(), runningBinaryCustodian(), true, prober, syscall.Kill)
	reader, writer, err := os.Pipe()
	failOnFixtureError(t, err)
	releaseReader, releaseWriter, err := os.Pipe()
	failOnFixtureError(t, err)
	defer func() { _ = writer.Close(); _ = releaseReader.Close(); _ = releaseWriter.Close() }()
	ready := t.TempDir() + "/ready"
	readyReader, readyWriter, err := os.Pipe()
	failOnFixtureError(t, err)
	defer readyReader.Close()
	command := fixture.Shell(`tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"
exec 5<&0
/bin/sh -c 'exec 3<&- 5<&-; trap "" TERM HUP; : >"$2"; printf "ready\n" >&4; read -r _' sh "$tag" "$1" <&5 >/dev/null 2>&1 &
grand=$!
exec 4>&- 5<&-
printf '%d\n' "$grand"
read -r _ <&3 || :`, ready)
	command.Stdin, command.ExtraFiles = reader, []*os.File{releaseReader, readyWriter}
	stdout, _ := command.StdoutPipe()
	failOnFixtureError(t, command.Start())
	_ = reader.Close()
	_ = releaseReader.Close()
	_ = readyWriter.Close()
	fixture.Record(command.Process.Pid)
	ref := fixture.refs[0]
	killFixtureProcessAtCleanup(t, fixture, command.Process, ref)
	var grandPID int
	_, err = fmt.Fscan(stdout, &grandPID)
	failOnFixtureError(t, err)
	readyLine, err := bufio.NewReader(readyReader).ReadString('\n')
	failOnFixtureError(t, err)
	if readyLine != "ready\n" {
		t.Fatalf("grandchild readiness = %q, want ready", readyLine)
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatalf("grandchild ready file: %v", err)
	}
	grand, state, err := prober.Probe(int64(grandPID))
	if err != nil || state != identity.Alive {
		t.Fatalf("grandchild is not alive: state=%v err=%v", state, err)
	}
	defer identity.SignalExact(prober, grand.Ref(), syscall.SIGKILL)
	failOnFixtureError(t, releaseWriter.Close())
	waitForFixtureExit(t, ref, fixture.waitBound)
	failOnFixtureError(t, command.Wait())
	recorder.cleanups[0]()
	if !strings.Contains(strings.Join(recorder.errs, "\n"), fmt.Sprintf("pid=%d ", grandPID)) {
		t.Fatalf("cleanup failures do not name grandchild %d: %v", grandPID, recorder.errs)
	}
	waitForFixtureExit(t, grand.Ref(), fixture.waitBound)
}

func TestLeashedShellExitsWhenItsOwnerLetsGo(t *testing.T) {
	recorder, sent := &recordingTB{}, 0
	fixture := newProcessFixture(recorder, t.Name(), runningBinaryCustodian(), true, identity.KernelProber{}, func(pid int, sig syscall.Signal) error { sent++; return syscall.Kill(pid, sig) })
	fixture.scan = noFixtureSurvivors
	command, ref, release := startLeashedShell(t, fixture, true)
	// Close the leash before releasing the shell; on macOS, a reader entering read as the last writer closes can miss EOF.
	_ = fixture.leash.Close()
	_ = release.Close()
	waitForFixtureExit(t, ref, fixture.waitBound)
	_, _ = command.Wait()
	recorder.cleanups[0]()
	if sent != 0 || len(recorder.errs) != 0 {
		t.Fatalf("sent=%d failures=%v", sent, recorder.errs)
	}
}

func TestCleanupIsScopedToTheFixtureKey(t *testing.T) {
	recorders := []*recordingTB{{}, {}}
	custodian := runningBinaryCustodian()
	fixtures := []*ProcessFixture{newProcessFixture(recorders[0], t.Name(), custodian, true, identity.KernelProber{}, syscall.Kill), newProcessFixture(recorders[1], t.Name(), custodian, true, identity.KernelProber{}, syscall.Kill)}
	first, firstRef, _ := startLeashedShell(t, fixtures[0], false)
	second, secondRef, _ := startLeashedShell(t, fixtures[1], false)
	recorders[0].cleanups[0]()
	waitForFixtureExit(t, firstRef, fixtures[0].waitBound)
	_, _ = first.Wait()
	exact, state, err := (identity.KernelProber{}).Probe(secondRef.Pid)
	if err != nil || state != identity.Alive || !identity.SameIdentity(exact, secondRef) || exact.Zombie {
		t.Fatalf("second fixture child changed after first cleanup: state=%v err=%v", state, err)
	}
	recorders[1].cleanups[0]()
	waitForFixtureExit(t, secondRef, fixtures[1].waitBound)
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
func waitForFixtureExit(t *testing.T, ref identity.Ref, processWaitBound time.Duration) {
	t.Helper()
	bound := fixtureExitObservationBound(processWaitBound)
	ticker, deadline := time.NewTicker(10*time.Millisecond), time.After(bound)
	defer ticker.Stop()
	for {
		exact, state, err := (identity.KernelProber{}).Probe(ref.Pid)
		if err == nil && (state == identity.Dead || state == identity.Alive && identity.SameIdentity(exact, ref) && exact.Zombie) {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("fixture process %d did not exit after %s: state=%s same-identity=%t zombie=%t probe=%v",
				ref.Pid, bound, state, identity.SameIdentity(exact, ref), exact.Zombie, err)
		}
	}
}

func fixtureProcessWaitBound(t *testing.T) time.Duration {
	t.Helper()
	bound, err := testenv.FixtureExitWaitBound()
	failOnFixtureError(t, err)
	return bound
}

func fixtureExitObservationBound(processWaitBound time.Duration) time.Duration {
	return 10 * processWaitBound
}

func TestFixtureExitObservationBoundDerivesFromProcessFixture(t *testing.T) {
	t.Parallel()

	for _, processBound := range []time.Duration{50 * time.Second, 12500 * time.Millisecond} {
		if got, want := fixtureExitObservationBound(processBound), 10*processBound; got != want {
			t.Errorf("fixture exit observation bound for %s = %s, want %s", processBound, got, want)
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

type fixtureWaitTB interface {
	Helper()
	Fatalf(string, ...any)
}

func waitForFixtureZombie(t fixtureWaitTB, output io.Reader, prober identity.Prober, ref identity.Ref, processWaitBound time.Duration) {
	t.Helper()
	if _, err := io.Copy(io.Discard, output); err != nil {
		t.Fatalf("read killed child stdout to EOF: %v", err)
	}
	bound := fixtureExitObservationBound(processWaitBound)
	started := time.Now()
	ticker, deadline := time.NewTicker(10*time.Millisecond), time.After(bound)
	defer ticker.Stop()
	for {
		exact, state, err := prober.Probe(ref.Pid)
		if err == nil && state == identity.Alive && identity.SameIdentity(exact, ref) && exact.Zombie {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("killed child stdout reached EOF without a matching zombie: bound=%s elapsed=%s state=%s same-identity=%t zombie=%t probe=%v",
				bound, time.Since(started), state, identity.SameIdentity(exact, ref), exact.Zombie, err)
		}
	}
}

func TestWaitForFixtureZombieWaitsForTheKernelFactAfterEOF(t *testing.T) {
	t.Parallel()

	exact := fixtureTeardownExact(500, 2)
	ref := exact.Ref()
	probes := 0
	prober := fixtureProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
		if pid != ref.Pid {
			t.Fatalf("probe pid = %d, want %d", pid, ref.Pid)
		}
		probes++
		observed := exact
		observed.Zombie = probes >= 4
		return observed, identity.Alive, nil
	})
	recorder := &recordingTB{}
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		waitForFixtureZombie(recorder, strings.NewReader(""), prober, ref, fixtureExitObservationBound(time.Millisecond))
	}()
	if probes < 4 {
		t.Fatalf("wait made %d probes, want at least 4", probes)
	}
	if recovered != nil || len(recorder.errs) != 0 {
		t.Fatalf("successful wait panic=%v failures=%q", recovered, recorder.errs)
	}

	recorder = &recordingTB{}
	recovered = nil
	func() {
		defer func() { recovered = recover() }()
		waitForFixtureZombie(recorder, strings.NewReader(""), fixtureProbeFunc(func(int64) (identity.Exact, identity.Liveness, error) {
			return exact, identity.Alive, nil
		}), ref, time.Millisecond)
	}()
	failure := strings.Join(recorder.errs, "\n")
	if recovered != errRecordingFatal {
		t.Fatalf("non-zombie wait panic = %v, want fatal sentinel; failure=%q", recovered, failure)
	}
	for _, want := range []string{"bound=10ms", "elapsed=", "state=", "same-identity=", "zombie=false"} {
		if !strings.Contains(failure, want) {
			t.Fatalf("non-zombie wait failure %q does not contain %q", failure, want)
		}
	}
}
