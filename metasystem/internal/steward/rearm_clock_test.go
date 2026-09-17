package steward

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
)

type manualRearmClock struct {
	mu      sync.Mutex
	now     time.Time
	timers  []chan time.Time
	created chan chan time.Time
}

func newManualRearmClock() *manualRearmClock {
	return &manualRearmClock{now: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC), created: make(chan chan time.Time, 256)}
}

func (clock *manualRearmClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *manualRearmClock) After(time.Duration) <-chan time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	timer := make(chan time.Time, 1)
	clock.timers = append(clock.timers, timer)
	select {
	case clock.created <- timer:
	default:
	}
	return timer
}

func (clock *manualRearmClock) advance(duration time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(duration)
	clock.mu.Unlock()
}

func (clock *manualRearmClock) snapshot() []chan time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return append([]chan time.Time(nil), clock.timers...)
}

func waitForRearmTimer(clock *manualRearmClock, count int) chan time.Time {
	return <-clock.created
}

func TestWitnessResolverStallsOnlyWhenAStepIsSilent(t *testing.T) {
	clock := newManualRearmClock()
	permit := make(chan struct{})
	progressed := make(chan struct{})
	finish := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- RunRearmStep(context.Background(), clock, 20*time.Second, "digest", func(_ context.Context, progress func()) error {
			for {
				select {
				case <-permit:
					progress()
					progressed <- struct{}{}
				case <-finish:
					return nil
				}
			}
		})
	}()
	current := waitForRearmTimer(clock, 1)
	for index := 0; index < 18; index++ {
		clock.advance(5 * time.Second)
		permit <- struct{}{}
		<-progressed
		current <- clock.Now()
		select {
		case current = <-clock.created:
		case err := <-result:
			t.Fatalf("progressing step stopped after %d seconds: %v", (index+1)*5, err)
		}
		select {
		case err := <-result:
			t.Fatalf("progressing step stopped after %d seconds: %v", (index+1)*5, err)
		default:
		}
	}
	close(finish)
	if err := <-result; err != nil {
		t.Fatalf("progressing 90-second step failed: %v", err)
	}

	silentClock := newManualRearmClock()
	silent := make(chan error, 1)
	go func() {
		silent <- RunRearmStep(context.Background(), silentClock, 20*time.Second, "digest", func(ctx context.Context, _ func()) error {
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	silentTimer := waitForRearmTimer(silentClock, 1)
	silentClock.advance(21 * time.Second)
	silentTimer <- silentClock.Now()
	if err := <-silent; err == nil || !errors.Is(err, ErrJudgmentStalled) || !strings.Contains(err.Error(), "digest exceeded the configured 20-second bound") {
		t.Fatalf("silent step did not name its stall: %v", err)
	}

	sequenceClock := newManualRearmClock()
	for index := 0; index < 5; index++ {
		sequenceClock.advance(15 * time.Second)
		if err := RunRearmStep(context.Background(), sequenceClock, 20*time.Second, "step", func(context.Context, func()) error { return nil }); err != nil {
			t.Fatalf("step %d inherited a total deadline: %v", index+1, err)
		}
	}
}

func TestWitnessExtractionCountsStdoutAsProgress(t *testing.T) {
	root := initRearmRepo(t)
	commitRearmTree(t, root, "archive")
	bin := t.TempDir()
	startedPath := filepath.Join(t.TempDir(), "started")
	readyPath := filepath.Join(t.TempDir(), "ready")
	releasePath := filepath.Join(t.TempDir(), "release")
	if err := syscall.Mkfifo(startedPath, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(readyPath, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(releasePath, 0o600); err != nil {
		t.Fatal(err)
	}
	release, err := os.OpenFile(releasePath, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = release.Close() })
	script := "#!/bin/sh\nprintf started >\"$TAR_STARTED_FIFO\"\nIFS= read -r answer <\"$TAR_RELEASE_FIFO\"\ni=0\nwhile [ \"$i\" -lt 8192 ]; do printf 'entry-%s\\n' \"$i\"; i=$((i+1)); done\nprintf ready >\"$TAR_READY_FIFO\"\nIFS= read -r answer <\"$TAR_RELEASE_FIFO\"\n"
	tar := filepath.Join(bin, "tar")
	writeRearmFile(t, tar, script)
	if err := os.Chmod(tar, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TAR_STARTED_FIFO", startedPath)
	t.Setenv("TAR_READY_FIFO", readyPath)
	t.Setenv("TAR_RELEASE_FIFO", releasePath)
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	clock := newManualRearmClock()
	result := make(chan error, 1)
	go func() {
		_, digestErr := digestArchivedTree(context.Background(), root, "HEAD", policy, clock, 20)
		result <- digestErr
	}()
	started, err := os.Open(startedPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(started); err != nil {
		t.Fatal(err)
	}
	_ = started.Close()
	clock.advance(15 * time.Second)
	if _, err := release.WriteString("write stdout\n"); err != nil {
		t.Fatal(err)
	}
	ready, err := os.Open(readyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(ready); err != nil {
		t.Fatal(err)
	}
	_ = ready.Close()
	timers := clock.snapshot()
	if len(timers) != 2 {
		t.Fatalf("archive and extraction did not each acquire one silence timer: timers=%d", len(timers))
	}
	for range timers {
		<-clock.created
	}
	clock.advance(15 * time.Second)
	timers[1] <- clock.Now()
	select {
	case <-clock.created:
	case err := <-result:
		t.Fatalf("extractor treated stdout-only entries as silence: %v", err)
	}
	if _, err := release.WriteString("finish\n"); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatalf("extractor stalled despite stdout entry progress: %v", err)
	}
}
