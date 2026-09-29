package main

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// The walkthrough owns its checkout (disk-lifetimes A11, DL3A-07, DL4A-06):
// it is removed after a completed shutdown with joined writers and a closed
// Partner, on every exit path, and kept, named, when a handler may still run.

var walkthroughBinary struct {
	once sync.Once
	path string
	err  error
}

// buildWalkthrough builds this command once for the package's tests.
func buildWalkthrough(t *testing.T) string {
	t.Helper()
	walkthroughBinary.once.Do(func() {
		directory, err := os.MkdirTemp("", "walkthrough-binary-")
		if err != nil {
			walkthroughBinary.err = err
			return
		}
		walkthroughBinary.path = filepath.Join(directory, "walkthrough")
		output, err := exec.Command("go", "build", "-trimpath", "-o", walkthroughBinary.path, ".").CombinedOutput()
		if err != nil {
			walkthroughBinary.err = errors.New(string(output))
		}
	})
	if walkthroughBinary.err != nil {
		t.Fatalf("build the walkthrough: %v", walkthroughBinary.err)
	}
	return walkthroughBinary.path
}

// walkthroughRun is one started walkthrough with its own temp root.
type walkthroughRun struct {
	command  *exec.Cmd
	temp     string
	checkout string
	address  string
	stderr   strings.Builder
}

func startWalkthrough(t *testing.T, args ...string) *walkthroughRun {
	t.Helper()
	run := &walkthroughRun{temp: t.TempDir()}
	run.command = exec.Command(buildWalkthrough(t), append([]string{"-listen", "127.0.0.1:0"}, args...)...)
	run.command.Env = append(os.Environ(), "TMPDIR="+run.temp)
	run.command.Stderr = &run.stderr
	stdout, err := run.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := run.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = run.command.Process.Kill(); _ = run.command.Wait() })
	lines := bufio.NewScanner(stdout)
	for lines.Scan() {
		line := lines.Text()
		if value, found := strings.CutPrefix(line, "checkout "); found {
			run.checkout = value
		}
		if value, found := strings.CutPrefix(line, "ready http://"); found {
			run.address = value
			break
		}
	}
	go func() { _, _ = io.Copy(io.Discard, stdout) }()
	if run.checkout == "" || run.address == "" {
		t.Fatalf("the walkthrough did not start: checkout=%q address=%q stderr=%s", run.checkout, run.address, run.stderr.String())
	}
	return run
}

// terminate sends SIGTERM and returns the exit code.
func (run *walkthroughRun) terminate(t *testing.T) int {
	t.Helper()
	if err := run.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	err := run.command.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0
}

// openStream holds the notification event stream open: it reads the
// preamble and returns the body, which it never closes itself.
func (run *walkthroughRun) openStream(t *testing.T) io.ReadCloser {
	t.Helper()
	response, err := http.Get("http://" + run.address + "/api/notifications/stream")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream = %s %s", response.Status, response.Header.Get("Content-Type"))
	}
	preamble := make([]byte, 8)
	if _, err := io.ReadFull(response.Body, preamble); err != nil {
		t.Fatalf("read the stream preamble: %v", err)
	}
	return response.Body
}

func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s survived: %v", path, err)
	}
}

// SIGTERM with both writers live removes the checkout and its notepad home,
// and exits 0; nothing recreates the journal after the exit.
func TestWalkthroughSIGTERMRemovesItsCheckout(t *testing.T) {
	t.Parallel()
	run := startWalkthrough(t, "-notify-every", "50ms", "-fleet-every", "50ms")
	response, err := http.Get("http://" + run.address + "/")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if code := run.terminate(t); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, run.stderr.String())
	}
	requireAbsent(t, run.checkout)
	requireAbsent(t, run.checkout+"-notepad")
	if entries, err := os.ReadDir(run.temp); err != nil || len(entries) != 0 {
		t.Fatalf("the temp root holds %v after the exit (%v)", entries, err)
	}
}

// A connected event stream is ended by the server: the handler's context is
// cancelled, the connection closes within the bound, the exit is 0 and the
// checkout is gone.
func TestWalkthroughEndsAConnectedStreamAndRemovesItsCheckout(t *testing.T) {
	t.Parallel()
	run := startWalkthrough(t, "-notify-every", "50ms")
	stream := run.openStream(t)
	defer stream.Close()
	ended := make(chan error, 1)
	go func() { _, err := io.Copy(io.Discard, stream); ended <- err }()
	if code := run.terminate(t); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, run.stderr.String())
	}
	if err := <-ended; err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("the stream did not end cleanly: %v", err)
	}
	requireAbsent(t, run.checkout)
}

// With the handlers' cancellation disabled (a test seam), Shutdown runs out
// its bound under the held stream: exit 1, the line naming the kept path,
// and the checkout present, so a live reader never reads a deleted journal.
func TestWalkthroughKeepsItsCheckoutWhenShutdownDoesNotComplete(t *testing.T) {
	t.Parallel()
	run := startWalkthrough(t, "-test-uncancelled-handlers", "-test-shutdown-bound", "300ms")
	stream := run.openStream(t)
	defer stream.Close()
	if code := run.terminate(t); code != 1 {
		t.Fatalf("exit %d, stderr %s", code, run.stderr.String())
	}
	if want := "walkthrough: checkout kept at " + run.checkout + ": shutdown did not complete"; !strings.Contains(run.stderr.String(), want) {
		t.Fatalf("stderr %q lacks %q", run.stderr.String(), want)
	}
	if _, err := os.Stat(filepath.Join(run.checkout, "metasystem.conf")); err != nil {
		t.Fatalf("the kept checkout is gone: %v", err)
	}
}

// A flag error exits non-zero without making a checkout; a listen failure
// removes the checkout it already made.
func TestWalkthroughFailuresLeaveNoCheckout(t *testing.T) {
	t.Parallel()
	binary := buildWalkthrough(t)
	temp := t.TempDir()
	command := exec.Command(binary, "-register", "bogus")
	command.Env = append(os.Environ(), "TMPDIR="+temp)
	if err := command.Run(); err == nil {
		t.Fatal("a flag error exited 0")
	}
	if entries, _ := os.ReadDir(temp); len(entries) != 0 {
		t.Fatalf("a flag error made %v", entries)
	}

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	command = exec.Command(binary, "-listen", occupied.Addr().String())
	command.Env = append(os.Environ(), "TMPDIR="+temp)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("a listen failure exited 0: %s", output)
	}
	if entries, _ := os.ReadDir(temp); len(entries) != 0 {
		t.Fatalf("a listen failure left %v", entries)
	}
}
