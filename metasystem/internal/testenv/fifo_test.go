package testenv

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func fixtureFIFO(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "handoff")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHoldFIFOLetsBothEndsOpenAtOnce(t *testing.T) {
	t.Parallel()
	path := fixtureFIFO(t)
	HoldFIFO(t, path)
	writer, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Close(writer); err != nil {
		t.Fatal(err)
	}
	reader, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(reader)
	var one [1]byte
	if n, err := syscall.Read(reader, one[:]); err != syscall.EAGAIN {
		t.Fatalf("read = %d, %v; want EAGAIN while a writer holds the FIFO", n, err)
	}
}

func TestAwaitLineReturnsALineWrittenInParts(t *testing.T) {
	t.Parallel()
	path := fixtureFIFO(t)
	end := HoldFIFO(t, path)
	if _, err := end.WriteString("do"); err != nil {
		t.Fatal(err)
	}
	written := false
	line := AwaitLine(t, "the split line", path, func() (bool, string) {
		if !written {
			if _, err := end.WriteString("ne\n"); err != nil {
				t.Fatal(err)
			}
			written = true
		}
		return true, ""
	}, nil)
	if line != "done" || !written {
		t.Fatalf("line = %q, second part written = %t", line, written)
	}
}

func TestAwaitLineLeavesTheNextLineInThePipe(t *testing.T) {
	t.Parallel()
	path := fixtureFIFO(t)
	end := HoldFIFO(t, path)
	if _, err := end.WriteString("one\ntwo\n"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"one", "two"} {
		if got := AwaitLine(t, "the next line", path, nil, nil); got != want {
			t.Fatalf("line = %q, want %q", got, want)
		}
	}
}

func TestAwaitLineFailsAtOnceWhenItsProducerIsGone(t *testing.T) {
	t.Parallel()
	path := fixtureFIFO(t)
	HoldFIFO(t, path)
	recorder := &awaitRecorder{}
	line := AwaitLine(recorder, "the builder's done line", path, func() (bool, string) {
		return false, "builder 7 exited"
	}, nil)
	if line != "" || len(recorder.failures) != 1 || !strings.Contains(recorder.failures[0], "the builder's done line") || !strings.Contains(recorder.failures[0], "builder 7 exited") {
		t.Fatalf("line = %q, failures = %q", line, recorder.failures)
	}
}

func TestAwaitLineFailsAtOnceOnAnUnheldFIFO(t *testing.T) {
	t.Parallel()
	path := fixtureFIFO(t)
	recorder := &awaitRecorder{}
	line := AwaitLine(recorder, "the missing writer's line", path, func() (bool, string) { return true, "" }, nil)
	if line != "" || len(recorder.failures) != 1 || !strings.Contains(recorder.failures[0], "the missing writer's line") || !strings.Contains(recorder.failures[0], "every writer closed before a line") {
		t.Fatalf("line = %q, failures = %q", line, recorder.failures)
	}
}
