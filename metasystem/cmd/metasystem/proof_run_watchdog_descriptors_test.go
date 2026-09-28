package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"golang.org/x/sys/unix"
)

// TestProofRunWatchdogReleasesAnInheritedLockDescriptor is the witness for
// the lock an orphaned watchdog held on 2026-09-28. The watchdog inherits a
// descriptor holding an exclusive flock, as the Lima suite runner's lock
// reached it through flock(1), go test and the test binary. The watchdog is
// started with no ExtraFiles of its own, so it must close that descriptor at
// startup: once it has opened its progress journal (a FIFO, so the open is a
// synchronization point and nothing here waits on a clock), an independent
// description takes the lock without blocking.
func TestProofRunWatchdogReleasesAnInheritedLockDescriptor(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(root, "suite.lock")
	inherited, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer inherited.Close()
	if err := unix.Flock(int(inherited.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	progress := filepath.Join(root, "progress.jsonl")
	if err := unix.Mkfifo(progress, 0o600); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	done := filepath.Join(root, "done")
	suite, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe the watched process: %v (%s)", err, state)
	}
	ref := suite.Ref()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fixture := testutil.Fixture(t)
	proof := pinProofBinaryFixture(t, root)
	watchdog := proof.command(fixture.Env(os.Environ()), executable, "proof-run", "watchdog",
		"--suite", "fixture", "--root", root, "--conf", conf, "--progress", progress, "--done", done,
		"--suite-pid", strconv.FormatInt(ref.Pid, 10), "--suite-started-at", strconv.FormatInt(ref.StartedAtSec, 10),
		"--suite-start-ticks", strconv.FormatInt(ref.StartTicks, 10), "--suite-boot-id", ref.BootID,
		"--silence-ms", "60000", "--section-cap-ms", "60000", "--evidence-timeout-ms", "1000", "--evidence-max-bytes", "1024",
		"--poll-ms", "1", "--term-grace-ms", "1", "--kill-grace-ms", "1", "--log", filepath.Join(root, "suite.log"))
	watchdog.ExtraFiles = []*os.File{inherited}
	var output bytes.Buffer
	watchdog.Stdout, watchdog.Stderr = &output, &output
	if err := watchdog.Start(); err != nil {
		t.Fatal(err)
	}
	fixture.Hold(watchdog.Process.Pid)
	waited := false
	t.Cleanup(func() {
		if !waited {
			_ = watchdog.Process.Kill()
			_ = watchdog.Wait()
		}
	})
	// From here only the watchdog's inherited copy holds the lock.
	if err := inherited.Close(); err != nil {
		t.Fatal(err)
	}
	// Blocks until the watchdog opens the journal for its first read, which
	// is after its startup.
	writer, err := os.OpenFile(progress, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	probe, err := os.OpenFile(lockPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	lockErr := unix.Flock(int(probe.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	_ = probe.Close()
	// Release the watchdog the way its launcher would: done lands before the
	// journal read ends, so the next poll returns without reopening it.
	if err := os.WriteFile(done, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	waitErr := watchdog.Wait()
	waited = true
	if errors.Is(lockErr, unix.EWOULDBLOCK) {
		t.Fatalf("the running watchdog still holds the inherited lock descriptor; output:\n%s", output.String())
	}
	if lockErr != nil {
		t.Fatalf("take the lock after watchdog startup: %v", lockErr)
	}
	if waitErr != nil {
		t.Fatalf("watchdog ended with %v; output:\n%s", waitErr, output.String())
	}
}
