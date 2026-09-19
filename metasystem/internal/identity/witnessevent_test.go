package identity

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type witnessEventSource interface {
	wait() error
}

type witnessEventFunc func() error

func (event witnessEventFunc) wait() error { return event() }

func waitForWitnessEvent(
	want string,
	arm func() witnessEventSource,
	observe func() (bool, string),
	elapsed func() time.Duration,
	snapshot func(string, []string, time.Duration),
	fail func(string),
) {
	event := arm()
	var observations []string
	lastObservation := ""
	haveObservation := false
	for {
		done, observation := observe()
		changed := !haveObservation || observation != lastObservation
		if changed {
			observations = append(observations, observation)
			lastObservation = observation
			haveObservation = true
			snapshot(want, observations, elapsed())
		}
		if done {
			return
		}
		if !changed {
			snapshot(want, observations, elapsed())
		}
		if err := event.wait(); err != nil {
			observations = append(observations, "event error: "+err.Error())
			snapshot(want, observations, elapsed())
			fail(fmt.Sprintf("waiting for %s: %v", want, err))
			return
		}
	}
}

func TestWitnessEventPreexistingFactDoesNotBlock(t *testing.T) {
	t.Parallel()

	blocks := 0
	waitForWitnessEvent("preexisting fact", func() witnessEventSource {
		return witnessEventFunc(func() error {
			blocks++
			return errors.New("unexpected block")
		})
	}, func() (bool, string) {
		return true, "published"
	}, func() time.Duration { return 0 }, func(string, []string, time.Duration) {}, func(message string) {
		t.Fatal(message)
	})
	if blocks != 0 {
		t.Fatalf("preexisting fact blocked %d times", blocks)
	}
}

func TestWitnessEventPublicationBetweenCheckAndBlockIsSeen(t *testing.T) {
	t.Parallel()

	published := false
	blocks := 0
	waitForWitnessEvent("published fact", func() witnessEventSource {
		return witnessEventFunc(func() error {
			blocks++
			published = true
			return nil
		})
	}, func() (bool, string) {
		return published, fmt.Sprintf("published=%t", published)
	}, func() time.Duration { return 0 }, func(string, []string, time.Duration) {}, func(message string) {
		t.Fatal(message)
	})
	if blocks != 1 {
		t.Fatalf("publication required %d blocks, want 1", blocks)
	}
}

func TestWitnessEventSnapshotPrecedesFirstBlock(t *testing.T) {
	t.Parallel()

	var order []string
	fact := false
	waitForWitnessEvent("ordered fact", func() witnessEventSource {
		return witnessEventFunc(func() error {
			order = append(order, "block")
			fact = true
			return nil
		})
	}, func() (bool, string) {
		order = append(order, "check")
		return fact, fmt.Sprintf("fact=%t", fact)
	}, func() time.Duration { return 7 * time.Second }, func(string, []string, time.Duration) {
		order = append(order, "snapshot")
	}, func(message string) {
		t.Fatal(message)
	})
	want := []string{"check", "snapshot", "block", "check", "snapshot"}
	if fmt.Sprint(order) != fmt.Sprint(want) {
		t.Fatalf("event loop order=%v, want %v", order, want)
	}
}

func TestWitnessEventTerminalErrorFailsWithText(t *testing.T) {
	t.Parallel()

	terminal := errors.New("watch was lost")
	var failure string
	waitForWitnessEvent("terminal fact", func() witnessEventSource {
		return witnessEventFunc(func() error {
			return terminal
		})
	}, func() (bool, string) {
		return false, "not yet"
	}, func() time.Duration { return 0 }, func(string, []string, time.Duration) {}, func(message string) {
		failure = message
	})
	if !strings.Contains(failure, terminal.Error()) {
		t.Fatalf("failure %q omits terminal event error %q", failure, terminal)
	}
}

func TestWitnessFileEventSeesContentAppend(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "custodian.log")
	if err := os.WriteFile(path, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	kernelEvent := armWitnessFileEvent(t, path)
	appended := false
	event := witnessEventFunc(func() error {
		if !appended {
			appended = true
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
			if err != nil {
				return err
			}
			_, writeErr := file.WriteString(" complete")
			if err := errors.Join(writeErr, file.Close()); err != nil {
				return err
			}
		}
		return kernelEvent.wait()
	})
	var failure string
	waitForWitnessEvent("complete log", func() witnessEventSource { return event }, func() (bool, string) {
		contents, err := os.ReadFile(path)
		return strings.Contains(string(contents), "complete"), fmt.Sprintf("read=%v content=%q", err, contents)
	}, func() time.Duration { return 0 }, func(string, []string, time.Duration) {}, func(message string) {
		failure = message
	})
	if failure != "" {
		t.Fatal(failure)
	}
}

func TestWitnessNonChildDeathEvent(t *testing.T) {
	t.Parallel()

	command := exec.Command("/bin/sh", "-c", `/bin/sh -c 'exec tail -f /dev/null' & printf '%s\n' "$!"; wait`)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_, _ = command.Process.Wait()
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read grandchild pid: %v", err)
	}
	pid, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64)
	if err != nil {
		t.Fatalf("parse grandchild pid %q: %v", line, err)
	}
	exact, state, err := (KernelProber{}).Probe(pid)
	if err != nil || state != Alive {
		t.Fatalf("probe grandchild pid %d: state=%s err=%v", pid, state, err)
	}
	ref := exact.Ref()
	armedEvent := armWitnessDeathEvent(t, ref)
	eventCalls := 0
	event := witnessEventFunc(func() error {
		eventCalls++
		return armedEvent.wait()
	})
	killed := false
	var failure string
	waitForWitnessEvent("grandchild death", func() witnessEventSource { return event }, func() (bool, string) {
		return witnessIdentityReleased(ref)
	}, func() time.Duration { return 0 }, func(string, []string, time.Duration) {
		// Kill after the first observation so it sees a live process and the
		// wait must consume the exit event.
		if killed {
			return
		}
		killed = true
		if err := SignalExact(KernelProber{}, ref, syscall.SIGKILL); err != nil {
			t.Fatalf("kill grandchild pid %d: %v", pid, err)
		}
	}, func(message string) {
		failure = message
	})
	if failure != "" {
		t.Fatal(failure)
	}
	if eventCalls != 1 {
		t.Fatalf("grandchild death event calls=%d, want 1", eventCalls)
	}
}

func TestWitnessDeathOfANonChildWhoseParentNeverReaps(t *testing.T) {
	t.Parallel()

	command := exec.Command("/bin/sh", "-c", `/bin/sh -c 'exec tail -f /dev/null' & printf '%s\n' "$!"; exec tail -f /dev/null`)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_, _ = command.Process.Wait()
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read grandchild pid: %v", err)
	}
	pid, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64)
	if err != nil {
		t.Fatalf("parse grandchild pid %q: %v", line, err)
	}
	t.Logf("non-reaping parent pid=%d grandchild pid=%d", command.Process.Pid, pid)
	exact, state, err := (KernelProber{}).Probe(pid)
	if err != nil || state != Alive {
		t.Fatalf("probe grandchild pid %d: state=%s err=%v", pid, state, err)
	}
	ref := exact.Ref()
	armedEvent := armWitnessDeathEvent(t, ref)
	eventCalls := 0
	event := witnessEventFunc(func() error {
		eventCalls++
		return armedEvent.wait()
	})
	killed := false
	var failure string
	waitForWitnessEvent("grandchild death without reap", func() witnessEventSource { return event }, func() (bool, string) {
		return witnessIdentityReleased(ref)
	}, func() time.Duration { return 0 }, func(string, []string, time.Duration) {
		// Kill after the first observation so it sees a live process and the
		// wait must consume the exit event.
		if killed {
			return
		}
		killed = true
		if err := SignalExact(KernelProber{}, ref, syscall.SIGKILL); err != nil {
			t.Fatalf("kill grandchild pid %d: %v", pid, err)
		}
	}, func(message string) {
		failure = message
	})
	if failure != "" {
		t.Fatal(failure)
	}
	if eventCalls != 1 {
		t.Fatalf("grandchild death event calls=%d, want 1", eventCalls)
	}
	zombie, state, err := (KernelProber{}).Probe(pid)
	if err != nil || state != Alive || !SameIdentity(zombie, ref) || !zombie.Zombie {
		t.Fatalf("unreaped grandchild pid %d: state=%s same=%t zombie=%t err=%v",
			pid, state, SameIdentity(zombie, ref), zombie.Zombie, err)
	}
}
