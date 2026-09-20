package proofrun

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Cancellation must drain an ordinary descendant before either the host slot
// or its exclusive named resource can be handed to another command. The child
// deliberately keeps neither stdio nor lease descriptors open.
func TestGLEResourceCommandCancellationDrainsClosedFDDescendantBeforeRelease(t *testing.T) {
	engine := buildResourceCustodyEngine(t)
	directory, conf := isolatedHostResources(t)
	first, err := AcquireHostResources(context.Background(), directory, conf, "heavy", []string{"fixture-db"})
	if err != nil {
		t.Fatal(err)
	}
	firstClosed := false
	t.Cleanup(func() {
		if !firstClosed {
			_ = first.Close()
		}
	})

	pidPath := filepath.Join(directory, "descendant.pid")
	readyPath := filepath.Join(directory, "descendant.ready")
	script := `(exec 3>&- 4>&- 5>&- 6>&- 7>&- 8>&- 9>&-; exec </dev/null >/dev/null 2>&1; trap '' TERM; : > "$2"; exec sleep 60) & echo $! > "$1"; wait`
	command := exec.Command("sh", "-c", script, "sh", pidPath, readyPath)
	runCtx, cancelRun := context.WithCancel(WithResourceCustodyExecutable(context.Background(), engine))
	defer cancelRun()
	runDone := make(chan error, 1)
	go func() { runDone <- RunResourceCommand(runCtx, command, first) }()

	prober := identity.KernelProber{}
	waitCustodyFile(t, readyPath, 8*time.Second)
	child := waitCustodyRef(t, prober, pidPath, 3*time.Second)
	workerGroup, err := syscall.Getpgid(command.Process.Pid)
	if err != nil {
		t.Fatalf("read native worker process group: %v", err)
	}
	childGroup, err := syscall.Getpgid(int(child.Pid))
	if err != nil || childGroup != workerGroup {
		t.Fatalf("native descendant escaped worker custody group: worker=%d child=%d err=%v", workerGroup, childGroup, err)
	}
	t.Cleanup(func() {
		// Failure cleanup only: the assertion below observes death first.
		if liveCustodyRef(prober, child) {
			_ = identity.SignalExact(prober, child, syscall.SIGKILL)
		}
	})
	if err := identity.SignalExact(prober, child, syscall.SIGTERM); err != nil {
		t.Fatalf("send TERM to exact descendant: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if !liveCustodyRef(prober, child) {
		t.Fatal("fixture descendant did not ignore TERM before cancellation")
	}

	contenderCtx, cancelContender := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancelContender()
	type contender struct {
		name   string
		done   chan *HostResourceLease
		errors chan error
	}
	startContender := func(name, class string, exclusive []string) contender {
		waiting := contender{name: name, done: make(chan *HostResourceLease), errors: make(chan error, 1)}
		go func() {
			next, acquireErr := AcquireHostResources(contenderCtx, directory, conf, class, exclusive)
			if acquireErr != nil {
				waiting.errors <- acquireErr
				return
			}
			select {
			case waiting.done <- next:
			case <-contenderCtx.Done():
				_ = next.Close()
			}
		}()
		return waiting
	}
	capacity := startContender("capacity", "heavy", nil)
	named := startContender("named resource", "cheap", []string{"fixture-db"})
	for _, waiting := range []contender{capacity, named} {
		select {
		case next := <-waiting.done:
			_ = next.Close()
			t.Fatalf("%s acquired while original descendant was alive", waiting.name)
		case acquireErr := <-waiting.errors:
			t.Fatalf("%s contender refused instead of waiting for custody: %v", waiting.name, acquireErr)
		case <-time.After(150 * time.Millisecond):
		}
	}

	cancelRun()
	select {
	case runErr := <-runDone:
		if runErr == nil || !errors.Is(runErr, context.Canceled) {
			t.Fatalf("cancelled native command must remain non-green: %v", runErr)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("resource command did not settle after cancellation")
	}
	if liveCustodyRef(prober, child) {
		t.Fatalf("exact descendant %d survived cancelled command custody", child.Pid)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	firstClosed = true
	for _, waiting := range []contender{capacity, named} {
		select {
		case next := <-waiting.done:
			if liveCustodyRef(prober, child) {
				_ = next.Close()
				t.Fatalf("%s acquired while exact descendant %d was alive", waiting.name, child.Pid)
			}
			if err := MarkHostResourcesClean(next.Files()); err != nil {
				_ = next.Close()
				t.Fatal(err)
			}
			if err := next.Close(); err != nil {
				t.Fatal(err)
			}
		case acquireErr := <-waiting.errors:
			t.Fatalf("%s contender failed after exact descendant died: %v", waiting.name, acquireErr)
		case <-contenderCtx.Done():
			t.Fatalf("%s remained unavailable after exact descendant died: %v", waiting.name, contenderCtx.Err())
		}
	}
}
