package board

import (
	"os"
	"sync"
	"syscall"
)

// KernelWatch watches dir and its seat directories through kqueue: a vnode
// event on any of them (an entry created, renamed or removed, a write) is one
// "read again", and a seat directory created after the watch began is added
// before the signal for it is given. A pipe registered beside the vnodes
// wakes the loop to close.
func KernelWatch(dir string) (*Watcher, error) {
	kq, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	var wake [2]int
	if err := syscall.Pipe(wake[:]); err != nil {
		syscall.Close(kq)
		return nil, err
	}
	k := &kqueueWatch{kq: kq, wake: wake, fds: map[string]int{}}
	closeAll := func() {
		k.mu.Lock()
		defer k.mu.Unlock()
		for _, fd := range k.fds {
			syscall.Close(fd)
		}
		syscall.Close(kq)
		syscall.Close(wake[0])
		syscall.Close(wake[1])
	}
	stop := syscall.Kevent_t{}
	syscall.SetKevent(&stop, wake[0], syscall.EVFILT_READ, syscall.EV_ADD|syscall.EV_ENABLE)
	if _, err := syscall.Kevent(kq, []syscall.Kevent_t{stop}, nil, nil); err != nil {
		closeAll()
		return nil, err
	}
	if err := k.add(dir); err != nil {
		closeAll()
		return nil, err
	}
	k.addSeats(dir)
	watcher := newWatcher(true)
	done := make(chan struct{})
	watcher.close = func() error {
		_, err := syscall.Write(wake[1], []byte{0})
		<-done
		closeAll()
		return err
	}
	go func() {
		defer close(done)
		events := make([]syscall.Kevent_t, 16)
		for {
			n, err := syscall.Kevent(kq, nil, events, nil)
			if err == syscall.EINTR {
				continue
			}
			if err != nil {
				return
			}
			for _, event := range events[:n] {
				if int(event.Ident) == wake[0] {
					return
				}
			}
			if n > 0 {
				k.addSeats(dir)
				watcher.signal()
			}
		}
	}()
	return watcher, nil
}

type kqueueWatch struct {
	mu   sync.Mutex
	kq   int
	wake [2]int
	fds  map[string]int
}

// add registers one directory's vnode events.
func (k *kqueueWatch) add(path string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, watched := k.fds[path]; watched {
		return nil
	}
	fd, err := syscall.Open(path, syscall.O_EVTONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	event := syscall.Kevent_t{}
	syscall.SetKevent(&event, fd, syscall.EVFILT_VNODE, syscall.EV_ADD|syscall.EV_CLEAR|syscall.EV_ENABLE)
	event.Fflags = syscall.NOTE_WRITE | syscall.NOTE_DELETE | syscall.NOTE_RENAME | syscall.NOTE_EXTEND | syscall.NOTE_ATTRIB
	if _, err := syscall.Kevent(k.kq, []syscall.Kevent_t{event}, nil, nil); err != nil {
		syscall.Close(fd)
		return err
	}
	k.fds[path] = fd
	return nil
}

// addSeats adds every seat directory not yet watched and drops the ones
// that are gone.
func (k *kqueueWatch) addSeats(dir string) {
	for _, seat := range seatDirs(dir) {
		_ = k.add(seat)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	for path, fd := range k.fds {
		if _, err := os.Lstat(path); err != nil {
			syscall.Close(fd)
			delete(k.fds, path)
		}
	}
}
