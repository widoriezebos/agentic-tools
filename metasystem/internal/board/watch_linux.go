package board

import (
	"os"
	"sync"
	"syscall"
)

const inotifyMask = syscall.IN_CREATE | syscall.IN_MOVED_TO | syscall.IN_MOVED_FROM | syscall.IN_DELETE |
	syscall.IN_CLOSE_WRITE | syscall.IN_ATTRIB | syscall.IN_DELETE_SELF

// KernelWatch watches dir and its seat directories through inotify: any
// event on them is one "read again", and a seat directory created after the
// watch began is added before the signal for it is given. The inotify
// descriptor is nonblocking and read through the runtime's poller, so
// closing it ends the loop.
func KernelWatch(dir string) (*Watcher, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "inotify")
	k := &inotifyWatch{fd: fd, watched: map[string]bool{}}
	if err := k.add(dir); err != nil {
		file.Close()
		return nil, err
	}
	k.addSeats(dir)
	watcher := newWatcher(true)
	done := make(chan struct{})
	watcher.close = func() error {
		err := file.Close()
		<-done
		return err
	}
	go func() {
		defer close(done)
		buffer := make([]byte, 64*(syscall.SizeofInotifyEvent+syscall.NAME_MAX+1))
		for {
			n, err := file.Read(buffer)
			if err != nil {
				return
			}
			if n > 0 {
				k.addSeats(dir)
				watcher.signal()
			}
		}
	}()
	return watcher, nil
}

type inotifyWatch struct {
	mu      sync.Mutex
	fd      int
	watched map[string]bool
}

func (k *inotifyWatch) add(path string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.watched[path] {
		return nil
	}
	if _, err := syscall.InotifyAddWatch(k.fd, path, inotifyMask); err != nil {
		return err
	}
	k.watched[path] = true
	return nil
}

// addSeats adds every seat directory not yet watched; the kernel drops the
// watch of a directory that is removed.
func (k *inotifyWatch) addSeats(dir string) {
	for _, seat := range seatDirs(dir) {
		_ = k.add(seat)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	for path := range k.watched {
		if _, err := os.Lstat(path); err != nil {
			delete(k.watched, path)
		}
	}
}
