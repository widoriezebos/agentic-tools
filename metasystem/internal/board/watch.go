package board

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Watcher tells its reader that the board may have changed. A signal carries
// nothing and means "read again", the rule of the interface's fleet watch:
// one pending signal and two pending signals ask for the same single read
// (batch-lane design D14-r2, R25).
type Watcher struct {
	events chan struct{}
	kernel bool
	stop   chan struct{}
	once   sync.Once
	close  func() error
}

func newWatcher(kernel bool) *Watcher {
	return &Watcher{events: make(chan struct{}, 1), kernel: kernel, stop: make(chan struct{})}
}

// Events is the channel of "read again" signals.
func (w *Watcher) Events() <-chan struct{} { return w.events }

// Kernel reports whether the kernel's file events drive the watch, rather
// than the bounded poll.
func (w *Watcher) Kernel() bool { return w.kernel }

// Close ends the watch.
func (w *Watcher) Close() error {
	var err error
	w.once.Do(func() {
		close(w.stop)
		if w.close != nil {
			err = w.close()
		}
	})
	return err
}

// signal offers one "read again"; an undrained one is not doubled.
func (w *Watcher) signal() {
	select {
	case w.events <- struct{}{}:
	default:
	}
}

// Watch watches the board directory and every seat directory in it with the
// kernel's file events (kqueue on macOS, inotify on Linux) and, where no
// kernel watch can be established (a board on a filesystem without events),
// polls every poll instead.
func Watch(dir string, poll time.Duration) *Watcher {
	if watcher, err := KernelWatch(dir); err == nil {
		return watcher
	}
	ticker := time.NewTicker(poll)
	watcher := PollWatch(ticker.C)
	watcher.close = func() error { ticker.Stop(); return nil }
	return watcher
}

// seatDirs are the directories a watch covers beside the board itself: the
// seat directories whose names pass the safe-name rule, and the mailboxes'
// directories (a seat's mailbox, the goal namespace and each goal's
// mailbox), where they exist. A marker for a message already offered lands
// one level below a watched directory; the bridge's tick finds it.
func seatDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() || !SafeName(entry.Name()) {
			continue
		}
		seat := filepath.Join(dir, entry.Name())
		dirs = append(dirs, seat)
		mailboxes := []string{filepath.Join(seat, "mailbox")}
		if entry.Name() == GoalNamespace {
			mailboxes = nil
			goals, _ := os.ReadDir(seat)
			for _, goal := range goals {
				if goal.IsDir() && SafeName(goal.Name()) {
					dirs = append(dirs, filepath.Join(seat, goal.Name()))
					mailboxes = append(mailboxes, filepath.Join(seat, goal.Name(), "mailbox"))
				}
			}
		}
		for _, mailbox := range mailboxes {
			for _, sub := range []string{mailbox, filepath.Join(mailbox, "messages"), filepath.Join(mailbox, "delivered")} {
				if info, err := os.Lstat(sub); err == nil && info.IsDir() {
					dirs = append(dirs, sub)
				}
			}
		}
	}
	return dirs
}
