//go:build linux

package identity

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

type linuxFileEvent struct {
	descriptor int
	watch      int32
	name       string
}

func armWitnessFileEvent(t *testing.T, path string) witnessEventSource {
	t.Helper()
	descriptor, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if err != nil {
		t.Fatalf("create inotify for %s: %v", path, err)
	}
	event := &linuxFileEvent{descriptor: descriptor, watch: -1, name: filepath.Base(path)}
	watch, err := unix.InotifyAddWatch(descriptor, filepath.Dir(path),
		unix.IN_CREATE|unix.IN_MOVED_TO|unix.IN_MODIFY|unix.IN_CLOSE_WRITE|
			unix.IN_DELETE|unix.IN_MOVED_FROM|unix.IN_DELETE_SELF|unix.IN_MOVE_SELF)
	if err != nil {
		event.close()
		t.Fatalf("register parent directory for %s: %v", path, err)
	}
	event.watch = int32(watch)
	t.Cleanup(event.close)
	return event
}

func (event *linuxFileEvent) wait() error {
	buffer := make([]byte, 4096)
	for {
		count, err := unix.Read(event.descriptor, buffer)
		// EINTR reissues the system call; it does not retry an assertion.
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("wait for inotify event: %w", err)
		}
		if count == 0 {
			return errors.New("inotify returned no event bytes")
		}
		for offset := 0; offset+unix.SizeofInotifyEvent <= count; {
			wd := int32(binary.NativeEndian.Uint32(buffer[offset : offset+4]))
			mask := binary.NativeEndian.Uint32(buffer[offset+4 : offset+8])
			nameLength := int(binary.NativeEndian.Uint32(buffer[offset+12 : offset+16]))
			next := offset + unix.SizeofInotifyEvent + nameLength
			if next > count {
				return fmt.Errorf("short inotify event: offset=%d length=%d bytes=%d", offset, nameLength, count)
			}
			nameBytes := buffer[offset+unix.SizeofInotifyEvent : next]
			for len(nameBytes) > 0 && nameBytes[len(nameBytes)-1] == 0 {
				nameBytes = nameBytes[:len(nameBytes)-1]
			}
			if mask&unix.IN_Q_OVERFLOW != 0 {
				return errors.New("inotify queue overflow")
			}
			if wd == event.watch && mask&unix.IN_IGNORED != 0 {
				return errors.New("inotify parent watch was lost")
			}
			if wd == event.watch && mask&(unix.IN_DELETE_SELF|unix.IN_MOVE_SELF) != 0 {
				return fmt.Errorf("inotify parent directory was removed or moved: mask=%#x", mask)
			}
			if wd == event.watch && (string(nameBytes) == event.name || len(nameBytes) == 0) {
				return nil
			}
			offset = next
		}
	}
}

func (event *linuxFileEvent) close() {
	if event.descriptor >= 0 {
		_ = unix.Close(event.descriptor)
		event.descriptor = -1
	}
}

type linuxProcessEvent struct {
	descriptor int
	delivered  bool
}

func armWitnessDeathEvent(t *testing.T, ref Ref) witnessEventSource {
	t.Helper()
	descriptor, err := unix.PidfdOpen(int(ref.Pid), 0)
	if err != nil {
		if errors.Is(err, unix.ESRCH) {
			if released, _ := witnessIdentityReleased(ref); released {
				return witnessEventFunc(func() error { return errors.New("death event requested after proven death") })
			}
		}
		t.Fatalf("pidfd_open for pid %d: %v", ref.Pid, err)
	}
	event := &linuxProcessEvent{descriptor: descriptor}
	t.Cleanup(event.close)
	return event
}

func (event *linuxProcessEvent) wait() error {
	if event.delivered {
		return errors.New("process exit event was already delivered")
	}
	descriptors := []unix.PollFd{{Fd: int32(event.descriptor), Events: unix.POLLIN}}
	for {
		count, err := unix.Poll(descriptors, -1)
		// EINTR reissues the system call; it does not retry an assertion.
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("wait for pidfd: %w", err)
		}
		if count != 1 {
			return fmt.Errorf("wait for pidfd returned %d descriptors", count)
		}
		if descriptors[0].Revents&unix.POLLIN == 0 {
			return fmt.Errorf("pidfd readiness omitted POLLIN: events=%#x", descriptors[0].Revents)
		}
		event.delivered = true
		return nil
	}
}

func (event *linuxProcessEvent) close() {
	if event.descriptor >= 0 {
		_ = unix.Close(event.descriptor)
		event.descriptor = -1
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
