package boundedexec

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTimeoutResolution(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	// Absent file: the stated defaults.
	if got := Timeout(conf, Local); got.Limit != 300*time.Second || got.Key != "exec.local-timeout-sec" {
		t.Fatalf("local default: %v", got)
	}
	if got := Timeout(conf, Network); got.Limit != 120*time.Second || got.Key != "exec.network-timeout-sec" {
		t.Fatalf("network default: %v", got)
	}
	os.WriteFile(conf, []byte("exec.local-timeout-sec=42\nexec.network-timeout-sec=7\n"), 0o644)
	if got := Timeout(conf, Local); got.Limit != 42*time.Second {
		t.Fatalf("configured local: %v", got)
	}
	if got := Timeout(conf, Network); got.Limit != 7*time.Second {
		t.Fatalf("configured network: %v", got)
	}
	// A malformed or non-positive bound must not disable bounding.
	os.WriteFile(conf, []byte("exec.local-timeout-sec=nonsense\n"), 0o644)
	if got := Timeout(conf, Local); got.Limit != 300*time.Second {
		t.Fatalf("malformed bound did not fall back: %v", got)
	}
	os.WriteFile(conf, []byte("exec.local-timeout-sec=0\n"), 0o644)
	if got := Timeout(conf, Local); got.Limit != 300*time.Second {
		t.Fatalf("zero bound did not fall back: %v", got)
	}
}

func TestRunReturnsPromptlyOnSuccess(t *testing.T) {
	if err := Run(exec.Command("true"), FixedBound(5*time.Second, "exec.local-timeout-sec"), "the true command"); err != nil {
		t.Fatalf("a fast command failed: %v", err)
	}
}

func TestRunPropagatesCommandFailure(t *testing.T) {
	err := Run(exec.Command("false"), FixedBound(5*time.Second, "exec.local-timeout-sec"), "the false command")
	if err == nil {
		t.Fatal("a failing command reported success")
	}
	if strings.Contains(err.Error(), "timed out") {
		t.Fatalf("a failure was misreported as a timeout: %v", err)
	}
}

// The hang test (B4's proof): a command that never returns is killed at the
// bound and named, instead of hanging its caller forever.
func TestRunKillsAHangingCommand(t *testing.T) {
	bound := FixedBound(300*time.Millisecond, "exec.local-timeout-sec")
	expiry := make(chan time.Time, 1)
	expiry <- time.Time{}
	var waits []time.Duration
	fixture := newPipeHeldCommand(t, "cat <&3 >/dev/null")
	err := fixture.runWithDeadline(bound, "the sleeping command", func(wait time.Duration) <-chan time.Time {
		waits = append(waits, wait)
		return expiry
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("a hanging command was not bounded: %v", err)
	}
	if !strings.Contains(err.Error(), "the sleeping command") {
		t.Fatalf("the failure does not name the operation: %v", err)
	}
	if fixture.command.ProcessState == nil || fixture.command.ProcessState.Success() {
		t.Fatalf("the expired command was not reaped after being killed: %v", fixture.command.ProcessState)
	}
	assertDeadlineWaits(t, waits, bound.Limit)
}

// A script's children must die with it: the group is signalled, not just the
// direct child.
func TestRunKillsTheWholeProcessGroup(t *testing.T) {
	t.Run("start failure releases readiness pipe", func(t *testing.T) {
		fixture := newPipeHeldCommand(t, "cat <&3 & echo ready; wait")
		reader := fixture.captureOutput(t)
		fixture.command.Path = filepath.Join(t.TempDir(), "missing-command")
		deadline := make(chan time.Time)
		done := fixture.startWithDeadline(FixedBound(300*time.Millisecond, "exec.local-timeout-sec"), "the missing command", func(time.Duration) <-chan time.Time {
			return deadline
		})
		ready, err := fixture.readReady(reader)
		if ready != "" || !errors.Is(err, io.EOF) {
			t.Fatalf("readiness after failed start = %q, %v; want EOF", ready, err)
		}
		startErr := fixture.waitForStart()
		if !errors.Is(startErr, os.ErrNotExist) {
			t.Fatalf("start error = %v, want nonexistent executable", startErr)
		}
		select {
		case <-fixture.completed:
		default:
			t.Fatal("failed-start readiness cleanup returned before joining the runner")
		}
		if runErr := <-done; !errors.Is(runErr, os.ErrNotExist) {
			t.Fatalf("failed-start result = %v, want nonexistent executable", runErr)
		}
	})

	t.Run("early readiness failure joins cleanup", func(t *testing.T) {
		fixture := newPipeHeldCommand(t, "cat <&3 & echo ready; wait")
		deadline := make(chan time.Time)
		done := fixture.startWithDeadline(FixedBound(300*time.Millisecond, "exec.local-timeout-sec"), "the spawning script", func(time.Duration) <-chan time.Time {
			return deadline
		})
		readinessErr := errors.New("injected readiness failure")
		_, err := fixture.readReady(bufio.NewReader(errorReader{err: readinessErr}))
		if !errors.Is(err, readinessErr) {
			t.Fatalf("readiness error = %v, want %v", err, readinessErr)
		}
		select {
		case <-fixture.completed:
		default:
			t.Fatal("early readiness cleanup returned before joining the runner")
		}
		if runErr := <-done; errors.Is(runErr, ErrTimedOut) {
			t.Fatalf("early readiness cleanup result = %v, want release before timeout", runErr)
		}
	})

	deadline := make(chan time.Time)
	fixture := newPipeHeldCommand(t, "cat <&3 & echo ready; wait")
	reader := fixture.captureOutput(t)
	var waits []time.Duration
	done := fixture.startWithDeadline(FixedBound(300*time.Millisecond, "exec.local-timeout-sec"), "the spawning script", func(duration time.Duration) <-chan time.Time {
		waits = append(waits, duration)
		return deadline
	})
	ready, err := fixture.readReady(reader)
	if err != nil || ready != "ready\n" {
		t.Fatalf("the spawning script did not report its child ready: line %q, error %v", ready, err)
	}
	// Pipe readiness is an OS event; this channel also proves Cmd.Start has
	// returned before the runner can close the parent's readiness writer.
	if err := fixture.waitForStart(); err != nil {
		t.Fatalf("the spawning script did not start: %v", err)
	}
	deadline <- time.Time{}
	remaining, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read process-group completion signal: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("unexpected process-group output after ready: %q", remaining)
	}
	err = <-done
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("the spawning script was not bounded: %v", err)
	}
	assertDeadlineWaits(t, waits, 300*time.Millisecond)
}

// Callers for whom a timeout is an ANSWER (a ceiling verdict, not a failure
// to run) rely on the sentinel surviving the wrap.
func TestRunTimeoutMatchesTheSentinel(t *testing.T) {
	bound := FixedBound(300*time.Millisecond, "exec.local-timeout-sec")
	expiry := make(chan time.Time, 1)
	expiry <- time.Time{}
	var waits []time.Duration
	err := newPipeHeldCommand(t, "cat <&3 >/dev/null").runWithDeadline(bound, "the held command", func(wait time.Duration) <-chan time.Time {
		waits = append(waits, wait)
		return expiry
	})
	if !errors.Is(err, ErrTimedOut) {
		t.Fatalf("expiry must match ErrTimedOut: %v", err)
	}
	assertDeadlineWaits(t, waits, bound.Limit)
	exit := exec.Command("false")
	if failure := Run(exit, FixedBound(time.Minute, "exec.local-timeout-sec"), "false"); errors.Is(failure, ErrTimedOut) {
		t.Fatalf("a plain failure must not match ErrTimedOut: %v", failure)
	}
}

// foundations-5: expiry names the key that actually produced the bound —
// never a magnitude-based guess that misdirects once a bound is tuned.
func TestTimeoutErrorNamesItsOwnKey(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	// A LOCAL bound tuned down to 1s sits below the network default — the
	// old guess would have named the network key.
	if err := os.WriteFile(conf, []byte("exec.local-timeout-sec=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bound := Timeout(conf, Local)
	if bound.Key != "exec.local-timeout-sec" || bound.Limit != time.Second {
		t.Fatalf("bound = %+v", bound)
	}
	expiry := make(chan time.Time, 1)
	expiry <- time.Time{}
	var waits []time.Duration
	err := newPipeHeldCommand(t, "cat <&3 >/dev/null").runWithDeadline(bound, "the tuned command", func(wait time.Duration) <-chan time.Time {
		waits = append(waits, wait)
		return expiry
	})
	if err == nil || !strings.Contains(err.Error(), "exec.local-timeout-sec") {
		t.Fatalf("expiry must name the key that produced the bound: %v", err)
	}
	if strings.Contains(err.Error(), "network") {
		t.Fatalf("the old magnitude guess resurfaced: %v", err)
	}
	assertDeadlineWaits(t, waits, bound.Limit)
}

func assertDeadlineWaits(t *testing.T, got []time.Duration, bound time.Duration) {
	t.Helper()
	if len(got) != 2 || got[0] != bound || got[1] != killGraceWindow {
		t.Fatalf("deadline waits = %v, want [%s %s]", got, bound, killGraceWindow)
	}
}

func TestPipeHeldCommandCleanupBeforeRunnerLaunch(t *testing.T) {
	t.Parallel()
	fixture := newPipeHeldCommand(t, "cat <&3 >/dev/null")

	// Setup can fail after the held-input pipe is allocated but before the
	// runner is launched. Exercise the same cleanup registered by the fixture.
	fixture.cleanup()
	if _, err := fixture.holder.Write([]byte("released")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write to cleaned-up holder = %v, want closed descriptor", err)
	}
	if _, err := fixture.reader.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("stat cleaned-up reader = %v, want closed descriptor", err)
	}
}

type pipeHeldCommand struct {
	command        *exec.Cmd
	reader         *os.File
	holder         *os.File
	startOrFailure chan struct{}
	completed      chan struct{}
	result         chan error
	outputWriter   *os.File
	startErr       error
	runnerLaunched bool
	cleanupOnce    sync.Once
}

func newPipeHeldCommand(t *testing.T, script string) *pipeHeldCommand {
	t.Helper()
	reader, holder, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", "-c", script)
	command.ExtraFiles = []*os.File{reader}
	fixture := &pipeHeldCommand{
		command: command, reader: reader, holder: holder,
		startOrFailure: make(chan struct{}), completed: make(chan struct{}), result: make(chan error, 1),
	}
	t.Cleanup(fixture.cleanup)
	return fixture
}

func (fixture *pipeHeldCommand) captureOutput(t *testing.T) *bufio.Reader {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fixture.command.Stdout = writer
	fixture.outputWriter = writer
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	return bufio.NewReader(reader)
}

func (fixture *pipeHeldCommand) startWithDeadline(bound Bound, what string, deadline func(time.Duration) <-chan time.Time) <-chan error {
	fixture.runnerLaunched = true
	go func() {
		acknowledged := false
		result := RunWithDeadline(fixture.command, bound, what, func(wait time.Duration) <-chan time.Time {
			if !acknowledged {
				acknowledged = true
				close(fixture.startOrFailure)
			}
			return deadline(wait)
		})
		if !acknowledged {
			fixture.startErr = result
			close(fixture.startOrFailure)
		}
		if fixture.outputWriter != nil {
			_ = fixture.outputWriter.Close()
		}
		fixture.result <- result
		close(fixture.completed)
	}()
	return fixture.result
}

func (fixture *pipeHeldCommand) runWithDeadline(bound Bound, what string, deadline func(time.Duration) <-chan time.Time) error {
	return <-fixture.startWithDeadline(bound, what, deadline)
}

func (fixture *pipeHeldCommand) waitForStart() error {
	<-fixture.startOrFailure
	return fixture.startErr
}

func (fixture *pipeHeldCommand) readReady(reader *bufio.Reader) (string, error) {
	ready, err := reader.ReadString('\n')
	if err != nil {
		fixture.cleanup()
	}
	return ready, err
}

func (fixture *pipeHeldCommand) cleanup() {
	fixture.cleanupOnce.Do(func() {
		if !fixture.runnerLaunched {
			_ = fixture.holder.Close()
			_ = fixture.reader.Close()
			return
		}
		<-fixture.startOrFailure
		_ = fixture.holder.Close()
		<-fixture.completed
		_ = fixture.reader.Close()
	})
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}
