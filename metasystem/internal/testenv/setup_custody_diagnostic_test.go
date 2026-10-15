package testenv

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func requireCustodyRecord(t *testing.T, output string, e setupCustodyEvidence, what string) {
	t.Helper()
	for _, want := range []string{"testenv: still waiting", "test=" + t.Name(), "pending=" + what, e.describe()} {
		if !strings.Contains(output, want) {
			t.Errorf("diagnostic lacks %q: %s", want, output)
		}
	}
	if got := strings.Count(output, "testenv: still waiting"); got != 1 {
		t.Errorf("records=%d, want one: %s", got, output)
	}
}

func TestSetupCustodyNoDeadlineDiagnosticAndCeiling(t *testing.T) {
	t.Parallel()
	e := newSetupCustodyEvidence(t)
	recorder := &setupCustodyRecorder{name: t.Name()}
	started := time.Unix(1000, 0)
	elapsed := time.Duration(0)
	var output bytes.Buffer
	const what = "the custodian to end the setup child"
	e.wait(recorder, what, func() bool { return false }, func() {
		switch elapsed {
		case 0:
			if output.Len() != 0 {
				t.Fatal("diagnostic before two minutes")
			}
			elapsed = 2 * time.Minute
		case 2 * time.Minute, 4 * time.Minute, 6 * time.Minute:
			requireCustodyRecord(t, output.String(), e, what)
			elapsed += 2 * time.Minute
		case 8 * time.Minute:
			requireCustodyRecord(t, output.String(), e, what)
			elapsed = 10*time.Minute + time.Second
		default:
			t.Fatal("wait ignored its ten-minute ceiling")
		}
	}, func() (time.Duration, bool) { return 0, false }, func() time.Time { return started.Add(elapsed) }, &output)
	if elapsed != 10*time.Minute+time.Second || len(recorder.failures) != 1 {
		t.Fatalf("elapsed=%s failures=%q", elapsed, recorder.failures)
	}
	for _, want := range []string{what, "without a test binary deadline", e.describe()} {
		if !strings.Contains(recorder.failures[0], want) {
			t.Errorf("failure lacks %q: %s", want, recorder.failures[0])
		}
	}
	requireCustodyRecord(t, output.String(), e, what)
}

func TestSetupCustodyNoDeadlineCanSucceedBeforeCeiling(t *testing.T) {
	t.Parallel()
	for _, at := range []time.Duration{time.Minute, 9 * time.Minute} {
		t.Run(at.String(), func(t *testing.T) {
			t.Parallel()
			e := newSetupCustodyEvidence(t)
			recorder := &setupCustodyRecorder{name: t.Name()}
			started := time.Unix(1000, 0)
			elapsed := time.Duration(0)
			var output bytes.Buffer
			e.wait(recorder, "settlement", func() bool { return elapsed == at }, func() {
				if elapsed != 0 {
					t.Fatal("wait missed settlement")
				}
				elapsed = at
			}, func() (time.Duration, bool) { return 0, false }, func() time.Time { return started.Add(elapsed) }, &output)
			if elapsed != at || len(recorder.failures) != 0 || output.Len() != 0 {
				t.Fatalf("elapsed=%s failures=%q output=%q", elapsed, recorder.failures, output.String())
			}
		})
	}
}

func TestSetupCustodyDeadlineDiagnosticKeepsDeadlineBound(t *testing.T) {
	t.Parallel()
	e := newSetupCustodyEvidence(t)
	recorder := &setupCustodyRecorder{name: t.Name()}
	started := time.Unix(1000, 0)
	elapsed := time.Duration(0)
	left := awaitReserve + 20*time.Minute
	var output bytes.Buffer
	const what = "the custodian to settle the dead binary's registry home"
	e.wait(recorder, what, func() bool { return false }, func() {
		switch elapsed {
		case 0:
			elapsed = 2 * time.Minute
		case 2 * time.Minute:
			requireCustodyRecord(t, output.String(), e, what)
			elapsed = 11 * time.Minute
		case 11 * time.Minute:
			requireCustodyRecord(t, output.String(), e, what)
			left = awaitReserve
		default:
			t.Fatal("wait ignored its deadline")
		}
	}, func() (time.Duration, bool) { return left, true }, func() time.Time { return started.Add(elapsed) }, &output)
	if elapsed != 11*time.Minute || left != awaitReserve || len(recorder.failures) != 1 {
		t.Fatalf("elapsed=%s left=%s failures=%q", elapsed, left, recorder.failures)
	}
	want := "still awaiting " + what + " with 30s left before the test binary's deadline; " + e.describe()
	if recorder.failures[0] != want {
		t.Fatalf("failure=%q, want %q", recorder.failures[0], want)
	}
	requireCustodyRecord(t, output.String(), e, what)
}

func TestSetupCustodySharedEvidenceHasIndependentWaits(t *testing.T) {
	t.Parallel()
	e := newSetupCustodyEvidence(t)
	var output bytes.Buffer
	clock := time.Unix(1000, 0)
	for _, what := range []string{"the custodian to end the setup child", "the custodian to settle the dead binary's registry home"} {
		recorder := &setupCustodyRecorder{name: t.Name()}
		start := clock
		before := output.Len()
		e.wait(recorder, what, func() bool { return clock.Sub(start) >= 3*time.Minute }, func() {
			elapsed := clock.Sub(start)
			if elapsed == 0 {
				if output.Len() != before {
					t.Fatal("second wait reused the first wait's start time")
				}
				clock = clock.Add(2 * time.Minute)
			} else if elapsed == 2*time.Minute {
				requireCustodyRecord(t, output.String()[before:], e, what)
				clock = clock.Add(time.Minute)
			} else {
				t.Fatal("wait missed event")
			}
		}, func() (time.Duration, bool) { return 0, false }, func() time.Time { return clock }, &output)
		if len(recorder.failures) != 0 {
			t.Fatal(recorder.failures)
		}
		requireCustodyRecord(t, output.String()[before:], e, what)
	}
	if strings.Count(output.String(), "testenv: still waiting") != 2 {
		t.Fatal(output.String())
	}
}

// The child signals on stdout only after the diagnostic write has completed,
// then stays inside the wait until the parent releases stdin. Reading the pipe
// at that barrier proves the default writer did not retain the record in memory.
func TestSetupCustodyDefaultDiagnosticReachesStderrBeforeReturn(t *testing.T) {
	t.Parallel()
	const helper = "TESTENV_CUSTODY_DIAGNOSTIC_HELPER"
	if os.Getenv(helper) == "1" {
		e := newSetupCustodyEvidence(t)
		clock := time.Unix(1000, 0)
		start := clock
		released := false
		e.wait(t, "the custodian to end the setup child", func() bool { return released }, func() {
			if clock == start {
				clock = clock.Add(2 * time.Minute)
				return
			}
			if _, err := io.WriteString(os.Stdout, "diagnostic write completed\n"); err != nil {
				t.Fatal(err)
			}
			var b [1]byte
			if _, err := io.ReadFull(os.Stdin, b[:]); err != nil {
				t.Fatal(err)
			}
			released = true
		}, func() (time.Duration, bool) { return 0, false }, func() time.Time { return clock }, nil)
		return
	}
	command := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.timeout=30m")
	command.Env = append(os.Environ(), helper+"=1")
	stdin, err := command.StdinPipe()
	checkTestenv(t, err)
	stdout, err := command.StdoutPipe()
	checkTestenv(t, err)
	read, write, err := os.Pipe()
	checkTestenv(t, err)
	defer read.Close()
	defer write.Close()
	command.Stderr = write
	checkTestenv(t, command.Start())
	defer func() { _ = stdin.Close(); _ = command.Process.Kill(); _ = command.Wait() }()
	lines := bufio.NewScanner(stdout)
	if !lines.Scan() || lines.Text() != "diagnostic write completed" {
		t.Fatalf("child did not reach the blocked wait: %q %v", lines.Text(), lines.Err())
	}
	// Use the descriptor directly: os.File's poller would wait for future writes.
	fd := int(read.Fd())
	checkTestenv(t, unix.SetNonblock(fd, true))
	var output bytes.Buffer
	for {
		var block [4096]byte
		n, err := unix.Read(fd, block[:])
		if n > 0 {
			output.Write(block[:n])
		}
		if err == unix.EAGAIN {
			break
		}
		checkTestenv(t, err)
		if n == 0 {
			break
		}
	}
	got := output.String()
	for _, want := range []string{"testenv: still waiting test=" + t.Name(), "pending=the custodian to end the setup child", "condition=custodian records remain", "synthetic custodian: cleanup failed", "synthetic setup-child record\n\n\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr lacks %q while child is blocked: %s", want, got)
		}
	}
	if strings.Count(got, "testenv: still waiting") != 1 {
		t.Errorf("expected one complete stderr record: %s", got)
	}
	_, err = stdin.Write([]byte{1})
	checkTestenv(t, err)
	checkTestenv(t, command.Wait())
}
