package board

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPollWatchSignalsOnEveryTick (R25, U10c-1): where no kernel watch can
// be established the bridge polls: every tick of its bounded poll is one
// "read again", carrying nothing, and a signal nobody drained yet is not
// doubled.
func TestPollWatchSignalsOnEveryTick(t *testing.T) {
	t.Parallel()
	ticks := make(chan time.Time)
	watcher := PollWatch(ticks)
	defer watcher.Close()
	ticks <- t0
	<-watcher.Events()
	ticks <- t0
	ticks <- t0
	<-watcher.Events()
	select {
	case <-watcher.Events():
		t.Fatal("two undrained ticks gave two signals")
	default:
	}
	if watcher.Kernel() {
		t.Fatal("a poll is not a kernel watch")
	}
}

// TestKernelWatchSignalsEveryCardWrite (R25, U10c-1): the kernel watch
// (kqueue on macOS, inotify on Linux) signals a card written under an
// existing seat, and a card written under a seat directory created after
// the watch began.
func TestKernelWatchSignalsEveryCardWrite(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	put(t, home, Card{Seat: seatOf("m1b"), Goal: "goal-a", Stage: StageClaimedIdle, Since: t0, LastProgressAt: t0})
	watcher, err := KernelWatch(Dir(home))
	if err != nil {
		t.Fatalf("no kernel watch on this platform's temporary directory: %v", err)
	}
	defer watcher.Close()
	if !watcher.Kernel() {
		t.Fatal("a kernel watch says it is one")
	}
	put(t, home, Card{Seat: seatOf("m1b"), Goal: "goal-b", Stage: StageClaimedIdle, Since: t0, LastProgressAt: t0})
	<-watcher.Events()
	if err := os.Mkdir(filepath.Join(Dir(home), "m1c"), 0o700); err != nil {
		t.Fatal(err)
	}
	<-watcher.Events()
	// The watch adds the new seat's directory on the signal it gave for it;
	// a write under it is then a signal of its own.
	drain(watcher)
	put(t, home, Card{Seat: seatOf("m1c"), Goal: "goal-c", Stage: StageClaimedIdle, Since: t0, LastProgressAt: t0})
	<-watcher.Events()
}

func drain(watcher *Watcher) {
	for {
		select {
		case <-watcher.Events():
		default:
			return
		}
	}
}
