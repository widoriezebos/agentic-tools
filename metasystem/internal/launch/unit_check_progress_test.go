package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type checkTimerRequest struct {
	duration time.Duration
	fire     chan time.Time
}
type progressCheckResult struct {
	directory string
	exits     []CheckExit
	err       error
}
type progressCheckBed struct {
	t       *testing.T
	clockMu sync.Mutex
	elapsed time.Duration
	fifo    string
	ready   *os.File
	timers  chan checkTimerRequest
	ended   chan progressCheckResult
}

func newProgressCheckBed(t *testing.T, write string) *progressCheckBed {
	t.Helper()
	return newProgressCheckBedWithExpiry(t, write, false)
}

func newProgressCheckBedWithExpiry(t *testing.T, write string, independent bool) *progressCheckBed {
	t.Helper()
	b := &progressCheckBed{t: t, fifo: filepath.Join(t.TempDir(), "input"), timers: make(chan checkTimerRequest, 100), ended: make(chan progressCheckResult, 1)}
	if err := syscall.Mkfifo(b.fifo, 0600); err != nil {
		t.Fatal(err)
	}
	readyPath := filepath.Join(t.TempDir(), "ready")
	if err := syscall.Mkfifo(readyPath, 0600); err != nil {
		t.Fatal(err)
	}
	readyFile, err := os.OpenFile(readyPath, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	b.ready = readyFile
	t.Cleanup(func() { readyFile.Close() })
	if independent {
		write += fmt.Sprintf("; printf r > '%s'", readyPath)
	}
	ready := "printf 'ready\\n';"
	if write == "" {
		ready, write = "", ":"
	}
	command := fmt.Sprintf("exec 3<> '%s'; printf r > '%s'; %s while IFS= read -r line <&3; do [ \"$line\" = end ] && exit 0; %s; done", b.fifo, readyPath, ready, write)
	check := UnitCheck{Cheap: command, Audits: "true", Minutes: 15, Directory: t.TempDir(), Environment: []string{}}
	records := t.TempDir()
	var mu sync.Mutex
	var all []chan time.Time
	abort := false
	stopping := make(chan struct{})
	completed := make(chan struct{})
	t.Cleanup(func() {
		mu.Lock()
		abort = true
		close(stopping)
		for _, timer := range all {
			close(timer)
		}
		mu.Unlock()
		<-completed
	})
	go func() {
		directory, exits, err := check.Run(check.Directory, records, CheckContext{Now: func() time.Time {
			b.clockMu.Lock()
			defer b.clockMu.Unlock()
			return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(b.elapsed)
		}, After: func(d time.Duration) <-chan time.Time {
			capacity := 1
			if independent {
				capacity = 0
			}
			fire := make(chan time.Time, capacity)
			bounded := d == 15*time.Minute || d == 120*time.Minute
			mu.Lock()
			cancelled := abort
			if cancelled || !bounded {
				close(fire)
			} else {
				all = append(all, fire)
			}
			mu.Unlock()
			if cancelled || !bounded {
				return fire
			}
			select {
			case b.timers <- checkTimerRequest{d, fire}:
			case <-stopping:
			}
			return fire
		}})
		b.ended <- progressCheckResult{directory, exits, err}
		close(completed)
	}()
	var readyByte [1]byte
	if _, err := readyFile.Read(readyByte[:]); err != nil {
		t.Fatal(err)
	}
	return b
}
func (b *progressCheckBed) timer(duration time.Duration) chan time.Time {
	b.t.Helper()
	select {
	case request := <-b.timers:
		if request.duration != duration {
			b.t.Fatalf("timer=%s, want %s", request.duration, duration)
		}
		return request.fire
	case result := <-b.ended:
		b.t.Fatalf("check ended before timer: %+v", result)
	}
	return nil
}
func (b *progressCheckBed) write(line string) {
	b.t.Helper()
	if line != "end" {
		b.clockMu.Lock()
		b.elapsed += time.Minute
		b.clockMu.Unlock()
	}
	file, err := os.OpenFile(b.fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		b.t.Fatal(err)
	}
	_, err = file.WriteString(line + "\n")
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		b.t.Fatalf("FIFO write=%v close=%v", err, closeErr)
	}
}
func (b *progressCheckBed) at(minute int) {
	b.clockMu.Lock()
	defer b.clockMu.Unlock()
	b.elapsed = time.Duration(minute) * time.Minute
}
func (b *progressCheckBed) finish() progressCheckResult {
	b.t.Helper()
	for {
		select {
		case <-b.timers: // The audits command has its own timers.
		case result := <-b.ended:
			return result
		}
	}
}
func assertProgressOutcome(t *testing.T, result progressCheckResult, outcome, message string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(result.directory, "observation.json"))
	var observation CheckExecution
	if err != nil || json.Unmarshal(data, &observation) != nil || len(observation.Steps) != 2 {
		t.Fatalf("observation=%s err=%v", data, err)
	}
	if observation.Steps[0].Outcome != outcome || observation.Steps[1].Outcome != "passed" {
		t.Fatalf("steps=%+v", observation.Steps)
	}
	if outcome == "passed" {
		if result.err != nil || result.exits[0].Exit != 0 {
			t.Fatalf("result=%+v", result)
		}
	} else if result.err == nil || !strings.Contains(result.exits[0].Error, message) {
		t.Fatalf("result=%+v, want %q", result, message)
	}
}
func TestUnitCheckProgressRestartsIdleTimer(t *testing.T) {
	t.Parallel()
	// An unbuffered progress receive orders expiry after the executor receives it.
	// Both timers fire, so retaining the old timer fails without waiting for time.
	progress := make(chan struct{})
	requests := make(chan chan time.Time)
	done := make(chan error, 1)
	go func() {
		command := exec.Command("/bin/sh", "-c", "read line")
		input, holder, err := os.Pipe()
		if err != nil {
			done <- err
			return
		}
		defer input.Close()
		defer holder.Close()
		command.Stdin = input
		done <- boundedexec.RunWithDeadline(command, boundedexec.Bound{Limit: 15 * time.Minute, Progress: progress}, "check", func(d time.Duration) <-chan time.Time {
			timer := make(chan time.Time, 1)
			if d == 15*time.Minute {
				requests <- timer
			} else {
				timer <- time.Time{}
			}
			return timer
		})
	}()
	old := <-requests
	select {
	case progress <- struct{}{}:
	case err := <-done:
		t.Fatalf("check ended before progress: %v", err)
	}
	old <- time.Time{}
	var fresh chan time.Time
	select {
	case fresh = <-requests:
	case err := <-done:
		t.Fatalf("progress did not restart the idle timer: %v", err)
	}
	fresh <- time.Time{}
	if err := <-done; !errors.Is(err, boundedexec.ErrTimedOut) || len(old) != 1 {
		t.Fatalf("the old timer remained active: old pending=%d error=%v", len(old), err)
	}
	// Silence also proves the check connected its writer to the bound.
	silent := newProgressCheckBed(t, "")
	expiry := silent.timer(15 * time.Minute)
	silent.timer(120 * time.Minute)
	silent.at(15)
	expiry <- time.Time{}
	assertProgressOutcome(t, silent.finish(), "deadline", "no output for 15m0s (proof.deadline)")
	b := newProgressCheckBed(t, "printf x")
	old = b.timer(15 * time.Minute)
	b.timer(120 * time.Minute)
	current := b.timer(15 * time.Minute) // The ready output proves the FIFO reader opened.
	var stale []chan time.Time
	for minute := 1; minute <= 20; minute++ {
		b.write("write")
		next := b.timer(15 * time.Minute)
		stale = append(stale, current)
		if minute == 15 {
			old <- time.Time{}
		}
		if minute >= 15 {
			stale[minute-15] <- time.Time{}
		}
		current = next
	}
	b.write("end")
	result := b.finish()
	assertProgressOutcome(t, result, "passed", "")
	if result.exits[0].Minutes != 20 {
		t.Fatalf("check ran for %g virtual minutes, want 20", result.exits[0].Minutes)
	}
}
func TestUnitCheckProgressSilenceStops(t *testing.T) {
	t.Parallel()
	b := newProgressCheckBed(t, "")
	idle := b.timer(15 * time.Minute)
	b.timer(120 * time.Minute)
	b.at(15)
	idle <- time.Time{}
	assertProgressOutcome(t, b.finish(), "deadline", "no output for 15m0s (proof.deadline)")
}
func TestUnitCheckProgressCountsOutputBytes(t *testing.T) {
	t.Parallel()
	writer := &outputProgress{wrote: make(chan struct{}, 1)}
	if n, err := writer.Write([]byte("x")); err != nil || n != 1 || len(writer.wrote) != 1 {
		t.Fatalf("bytes without a newline did not signal progress: n=%d err=%v", n, err)
	}
	<-writer.wrote
	if _, err := writer.Write(nil); err != nil || len(writer.wrote) != 0 {
		t.Fatalf("empty write signaled progress: %v", err)
	}
	check := UnitCheck{Cheap: "printf stdout", Audits: "printf stderr >&2", Minutes: 15, Directory: t.TempDir(), Environment: []string{}}
	_, exits, err := check.Run(check.Directory, t.TempDir(), CheckContext{Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }, After: func(time.Duration) <-chan time.Time { return make(chan time.Time) }})
	if err != nil || len(exits) != 2 || exits[0].Output != "stdout" || exits[1].Output != "stderr" {
		t.Fatalf("stdout or stderr lost: exits=%+v err=%v", exits, err)
	}
	for _, row := range []struct{ name, write string }{{"stdout", "printf 'x\\n'"}, {"stderr", "printf 'x\\n' >&2"}, {"no_newline", "printf x"}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := newProgressCheckBed(t, row.write)
			b.timer(15 * time.Minute)
			b.timer(120 * time.Minute)
			b.timer(15 * time.Minute)
			b.write("write")
			fresh := b.timer(15 * time.Minute)
			b.at(16)
			fresh <- time.Time{}
			assertProgressOutcome(t, b.finish(), "deadline", "no output for 15m0s (proof.deadline)")
		})
	}
}
func TestUnitCheckProgressCeilingDoesNotRestart(t *testing.T) {
	t.Parallel()
	// Initial idle expiry completes even if a ceiling was never scheduled.
	ceilings := 0
	check := UnitCheck{Cheap: "true", Audits: "true", Minutes: 15, Directory: t.TempDir(), Environment: []string{}}
	_, _, _ = check.Run(check.Directory, t.TempDir(), CheckContext{Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }, After: func(d time.Duration) <-chan time.Time {
		if d == 120*time.Minute {
			ceilings++
		}
		fired := make(chan time.Time, 1)
		fired <- time.Time{}
		return fired
	}})
	if ceilings != 2 {
		t.Fatalf("scheduled %d ceilings, want one per command", ceilings)
	}
	b := newProgressCheckBed(t, "yes x")
	b.timer(15 * time.Minute)
	ceiling := b.timer(120 * time.Minute)
	b.timer(15 * time.Minute)
	b.write("write")
	for range 20 {
		b.timer(15 * time.Minute)
	}
	b.at(120)
	ceiling <- time.Time{}
	assertProgressOutcome(t, b.finish(), "deadline", "still running after 2h0m0s (proof.deadline)")
}

func TestUnitCheckStderrProgressHasIndependentExpiry(t *testing.T) {
	t.Parallel()
	// Draining sixteen MiB requires the process pipe to deliver stderr before
	// the child acknowledges the write through the separate readiness FIFO.
	b := newProgressCheckBedWithExpiry(t, `i=0; while [ "$i" -lt 16384 ]; do printf '%01024d' 0 >&2; i=$((i+1)); done`, true)
	b.timer(15 * time.Minute)
	b.timer(120 * time.Minute)
	idle := b.timer(15 * time.Minute)
	b.write("write")
	var acknowledged [1]byte
	if _, err := b.ready.Read(acknowledged[:]); err != nil {
		t.Fatal(err)
	}
	b.at(15)
	progress, expired := false, false
	for !progress && !expired {
		select {
		case request := <-b.timers:
			if request.duration != 15*time.Minute {
				t.Fatalf("stderr changed the fixed ceiling: %s", request.duration)
			}
			progress = true
		case result := <-b.ended:
			t.Fatalf("check ended before the independently driven expiry: %v", result.err)
		default:
		}
		if !progress {
			// A nonblocking send to an unbuffered timer cannot race buffered
			// progress: it succeeds only when the executor is already waiting.
			select {
			case idle <- time.Time{}:
				expired = true
			default:
				runtime.Gosched()
			}
		}
	}
	if expired {
		result := b.finish()
		t.Fatalf("stderr did not restart the idle timer before its injected expiry: %v", result.err)
	}
	b.write("end")
	assertProgressOutcome(t, b.finish(), "passed", "")
}
