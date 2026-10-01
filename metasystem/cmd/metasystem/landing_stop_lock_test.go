package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// syncWriter is an output a test reads while the verb still writes it; seen
// closes once the output holds want.
type syncWriter struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	want string
	seen chan struct{}
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	if w.seen != nil && strings.Contains(w.buf.String(), w.want) {
		close(w.seen)
		w.seen = nil
	}
	return n, err
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// N-3: a landing stop that finds the host flock held (another lane step
// runs) says so in one plain line before it waits, then stops the lane
// once the flock is free.
func TestLandingStopSaysWhyItWaitsForTheHostLock(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	bed.alive = true
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("set = %d %q", code, stderr)
	}
	held, err := lock.File(lane.LockPath(bed.home), 0o600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	command, _ := findIntentAction("landing", "stop")
	seen := make(chan struct{})
	var stdout syncWriter
	stderr := syncWriter{want: "another landing step holds the lane; the stop takes effect when it finishes", seen: seen}
	done := make(chan int, 1)
	go func() {
		done <- runIntentIn(command, []string{"--by", "Wido"}, &stdout, &stderr, bed.cwd, bed.owners())
	}()
	select {
	case code := <-done:
		t.Fatalf("stop finished (%d) while the host lock was held, or waited silently: %q %q", code, stdout.String(), stderr.String())
	case <-seen:
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != 0 {
		t.Fatalf("stop after the lock was freed = %d %q", code, stderr.String())
	}
	if _, paused := lane.ReadPause(bed.home); !paused {
		t.Fatal("the lane is not paused after the stop")
	}
}
