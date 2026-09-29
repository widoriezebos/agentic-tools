package main

import (
	"bytes"
	"io"
	"sync"
	"testing"
)

// streamBuffer is one captured stream: a command may print from more than
// one goroutine (a progress printer, a cadence tick), so writes are
// serialised as the kernel serialised them on the pipe this replaced.
type streamBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *streamBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}

func (b *streamBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.String()
}

// runOnOwnStreams runs one command on standard output and error of its own,
// never the process's, and returns its exit code and what it printed on
// each. A parallel test's output can never reach them, and this command's
// output never reaches another test.
func runOnOwnStreams(run func(stdout, stderr io.Writer) int) (int, string, string) {
	var stdout, stderr streamBuffer
	code := run(&stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// TestRunOnOwnStreamsKeepsConcurrentCommandsApart: two commands printing at
// once each read only their own lines, however their writes interleave.
func TestRunOnOwnStreamsKeepsConcurrentCommandsApart(t *testing.T) {
	t.Parallel()
	type captured struct {
		code           int
		stdout, stderr string
	}
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan captured, 2)
	for _, name := range []string{"first", "second"} {
		go func() {
			code, stdout, stderr := runOnOwnStreams(func(stdout, stderr io.Writer) int {
				ready <- struct{}{}
				<-release
				for range 100 {
					_, _ = io.WriteString(stdout, name+"\n")
					_, _ = io.WriteString(stderr, name+"!\n")
				}
				return len(name)
			})
			results <- captured{code, stdout, stderr}
		}()
	}
	<-ready
	<-ready
	close(release)
	for range 2 {
		result := <-results
		name := map[int]string{5: "first", 6: "second"}[result.code]
		if name == "" || result.stdout != string(bytes.Repeat([]byte(name+"\n"), 100)) || result.stderr != string(bytes.Repeat([]byte(name+"!\n"), 100)) {
			t.Fatalf("a command read another's output: %+v", result)
		}
	}
}

// TestStreamBufferTakesConcurrentWrites: one command's goroutines may print
// on the same stream at once; every write arrives whole.
func TestStreamBufferTakesConcurrentWrites(t *testing.T) {
	t.Parallel()
	var buffer streamBuffer
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 1000 {
				_, _ = buffer.Write([]byte("line\n"))
			}
		}()
	}
	group.Wait()
	if got := buffer.String(); got != string(bytes.Repeat([]byte("line\n"), 8000)) {
		t.Fatalf("concurrent writes were lost or torn: %d bytes", len(got))
	}
}

// dispatchOn runs one command line as the binary does, on the caller's
// streams.
func dispatchOn(args []string, stdout, stderr io.Writer) int {
	return dispatchWithFamilies(args, stdout, stderr, families())
}

// withStreams is dependencies printing on the caller's streams.
func withStreams(dependencies syncRequestDependencies, stdout, stderr io.Writer) syncRequestDependencies {
	dependencies.stdout, dependencies.stderr = stdout, stderr
	return dependencies
}
