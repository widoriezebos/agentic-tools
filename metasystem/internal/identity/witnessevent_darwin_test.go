//go:build darwin

package identity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

type darwinFileEvent struct {
	queue     int
	directory int
	target    int
	path      string
}

func armWitnessFileEvent(t *testing.T, path string) witnessEventSource {
	t.Helper()
	event := &darwinFileEvent{queue: -1, directory: -1, target: -1, path: path}
	if err := event.arm(); err != nil {
		event.close()
		t.Fatalf("arm file event for %s: %v", path, err)
	}
	t.Cleanup(event.close)
	return event
}

func (event *darwinFileEvent) arm() error {
	queue, err := unix.Kqueue()
	if err != nil {
		return fmt.Errorf("create kqueue: %w", err)
	}
	event.queue = queue
	directory, err := unix.Open(filepath.Dir(event.path), unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open parent directory: %w", err)
	}
	event.directory = directory
	change := []unix.Kevent_t{{
		Ident:  uint64(directory),
		Filter: unix.EVFILT_VNODE,
		Flags:  unix.EV_ADD | unix.EV_CLEAR,
		Fflags: unix.NOTE_WRITE | unix.NOTE_RENAME | unix.NOTE_DELETE | unix.NOTE_REVOKE,
	}}
	if _, err := keventNoInterrupt(queue, change, nil); err != nil {
		return fmt.Errorf("register parent directory: %w", err)
	}
	return event.refreshTarget()
}

func (event *darwinFileEvent) refreshTarget() error {
	var pathState unix.Stat_t
	if err := unix.Stat(event.path, &pathState); err != nil {
		if errors.Is(err, unix.ENOENT) {
			event.closeTarget()
			return nil
		}
		return fmt.Errorf("stat target: %w", err)
	}
	if event.target >= 0 {
		var watchedState unix.Stat_t
		if err := unix.Fstat(event.target, &watchedState); err != nil {
			return fmt.Errorf("stat watched target: %w", err)
		}
		if pathState.Dev == watchedState.Dev && pathState.Ino == watchedState.Ino {
			return nil
		}
		event.closeTarget()
	}
	target, err := unix.Open(event.path, unix.O_EVTONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return fmt.Errorf("open target: %w", err)
	}
	change := []unix.Kevent_t{{
		Ident:  uint64(target),
		Filter: unix.EVFILT_VNODE,
		Flags:  unix.EV_ADD | unix.EV_CLEAR,
		Fflags: unix.NOTE_WRITE | unix.NOTE_EXTEND | unix.NOTE_RENAME | unix.NOTE_DELETE | unix.NOTE_REVOKE,
	}}
	if _, err := keventNoInterrupt(event.queue, change, nil); err != nil {
		_ = unix.Close(target)
		return fmt.Errorf("register target: %w", err)
	}
	event.target = target
	return nil
}

func (event *darwinFileEvent) wait() error {
	events := make([]unix.Kevent_t, 2)
	count, err := keventNoInterrupt(event.queue, nil, events)
	if err != nil {
		return fmt.Errorf("wait for file event: %w", err)
	}
	if count < 1 {
		return fmt.Errorf("wait for file event returned %d events", count)
	}
	for _, current := range events[:count] {
		if current.Flags&unix.EV_ERROR != 0 {
			return fmt.Errorf("file event error: %s", unix.Errno(current.Data))
		}
		if int(current.Ident) == event.directory && current.Fflags&(unix.NOTE_DELETE|unix.NOTE_RENAME|unix.NOTE_REVOKE) != 0 {
			return fmt.Errorf("parent directory watch was lost: flags=%#x", current.Fflags)
		}
		if int(current.Ident) == event.target && current.Fflags&unix.NOTE_REVOKE != 0 {
			return fmt.Errorf("target watch was revoked: flags=%#x", current.Fflags)
		}
	}
	if err := event.refreshTarget(); err != nil {
		return err
	}
	return nil
}

func (event *darwinFileEvent) closeTarget() {
	if event.target >= 0 {
		_ = unix.Close(event.target)
		event.target = -1
	}
}

func (event *darwinFileEvent) close() {
	event.closeTarget()
	if event.directory >= 0 {
		_ = unix.Close(event.directory)
		event.directory = -1
	}
	if event.queue >= 0 {
		_ = unix.Close(event.queue)
		event.queue = -1
	}
}

type darwinProcessEvent struct {
	queue     int
	delivered bool
}

func armWitnessDeathEvent(t *testing.T, ref Ref) witnessEventSource {
	t.Helper()
	queue, err := unix.Kqueue()
	if err != nil {
		t.Fatalf("create process kqueue for pid %d: %v", ref.Pid, err)
	}
	event := &darwinProcessEvent{queue: queue}
	change := []unix.Kevent_t{{
		Ident:  uint64(ref.Pid),
		Filter: unix.EVFILT_PROC,
		Flags:  unix.EV_ADD | unix.EV_CLEAR,
		Fflags: unix.NOTE_EXIT,
	}}
	if _, err := keventNoInterrupt(queue, change, nil); err != nil {
		event.close()
		if errors.Is(err, unix.ESRCH) {
			if released, _ := witnessIdentityReleased(ref); released {
				return witnessEventFunc(func() error { return errors.New("death event requested after proven death") })
			}
		}
		t.Fatalf("register process exit for pid %d: %v", ref.Pid, err)
	}
	t.Cleanup(event.close)
	return event
}

func (event *darwinProcessEvent) wait() error {
	if event.delivered {
		return errors.New("process exit event was already delivered")
	}
	events := make([]unix.Kevent_t, 1)
	count, err := keventNoInterrupt(event.queue, nil, events)
	if err != nil {
		return fmt.Errorf("wait for process exit: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("wait for process exit returned %d events", count)
	}
	if events[0].Flags&unix.EV_ERROR != 0 {
		return fmt.Errorf("process exit event error: %s", unix.Errno(events[0].Data))
	}
	if events[0].Fflags&unix.NOTE_EXIT == 0 {
		return fmt.Errorf("process event omitted NOTE_EXIT: flags=%#x", events[0].Fflags)
	}
	event.delivered = true
	return nil
}

func (event *darwinProcessEvent) close() {
	if event.queue >= 0 {
		_ = unix.Close(event.queue)
		event.queue = -1
	}
}

func publishWitnessFile(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
