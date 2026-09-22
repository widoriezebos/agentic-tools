package main

import (
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type fixtureLifetimeProber func(int64) (identity.Exact, identity.Liveness, error)

func (probe fixtureLifetimeProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return probe(pid)
}

func exactFixtureLifetimeOwner(t *testing.T) string {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe fixture lifetime owner: state=%s error=%v", state, err)
	}
	key, err := identity.EncodeKey(identity.FixtureKey{Owner: exact.Ref(), Test: "bounded-lifetime", Nonce: "1234abcd"})
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func fixtureLifetimeTestDependencies(environment map[string]string, after func(time.Duration) <-chan time.Time, open func(string) (io.ReadCloser, error)) fixtureLifetimeDependencies {
	return fixtureLifetimeDependencies{
		getenv: func(name string) string { return environment[name] },
		prober: identity.KernelProber{}, after: after, openLeash: open, ready: func() {},
	}
}

func TestRunUtilHoldRequiresTag(t *testing.T) {
	if code := runUtilHold(nil); code != 2 {
		t.Fatalf("a missing tag must be a usage error, got %d", code)
	}
}

func TestRunUtilHoldRequiresLeashOrExpiry(t *testing.T) {
	t.Parallel()
	deps := fixtureLifetimeTestDependencies(nil, nil, func(string) (io.ReadCloser, error) {
		t.Fatal("unbounded hold tried to open a leash")
		return nil, nil
	})
	if code := runUtilHoldWithDependencies([]string{"--tag", "unbounded"}, deps); code != 2 {
		t.Fatalf("an unbounded hold must be refused, got %d", code)
	}
}

func TestRunUtilHoldEndsAtInjectedExpiry(t *testing.T) {
	t.Parallel()
	expiry := make(chan time.Time, 1)
	deps := fixtureLifetimeTestDependencies(nil, func(time.Duration) <-chan time.Time { return expiry }, func(string) (io.ReadCloser, error) {
		t.Fatal("expiry-bounded hold tried to open a leash")
		return nil, nil
	})
	done := make(chan int, 1)
	go func() {
		done <- runUtilHoldWithDependencies([]string{"--tag", "expiry", "--max-seconds", "1"}, deps)
	}()
	expiry <- time.Unix(1, 0)
	if code := <-done; code != 0 {
		t.Fatalf("injected expiry hold exit = %d, want 0", code)
	}
}

func TestRunUtilHoldEndsWhenExactOwnersLeashCloses(t *testing.T) {
	t.Parallel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	owner := exactFixtureLifetimeOwner(t)
	deps := fixtureLifetimeTestDependencies(map[string]string{
		identity.FixtureOwnerEnv: owner,
		fixtureLeashEnvironment:  "fixture-leash",
	}, nil, func(path string) (io.ReadCloser, error) {
		if path != "fixture-leash" {
			t.Fatalf("opened leash %q", path)
		}
		return reader, nil
	})
	done := make(chan int, 1)
	go func() { done <- runUtilHoldWithDependencies([]string{"--tag", "leashed"}, deps) }()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if code := <-done; code != 0 {
		t.Fatalf("leash-bounded hold exit = %d, want 0", code)
	}
}

func TestRunUtilHoldRefusesMismatchedExactOwner(t *testing.T) {
	t.Parallel()
	owner := exactFixtureLifetimeOwner(t)
	deps := fixtureLifetimeTestDependencies(map[string]string{
		identity.FixtureOwnerEnv: owner,
		fixtureLeashEnvironment:  "fixture-leash",
	}, nil, func(string) (io.ReadCloser, error) {
		t.Fatal("mismatched owner reached its leash")
		return nil, nil
	})
	deps.prober = fixtureLifetimeProber(func(int64) (identity.Exact, identity.Liveness, error) {
		return identity.Exact{}, identity.Dead, nil
	})
	if code := runUtilHoldWithDependencies([]string{"--tag", "mismatch"}, deps); code != 2 {
		t.Fatalf("mismatched owner hold exit = %d, want 2", code)
	}
}

// The verb must survive until its termination signal, then acknowledge the
// orderly stop in the stopped file and exit 0.
func TestRunUtilHoldWritesStoppedFileOnTerm(t *testing.T) {
	// Registering our own handler first keeps an early SIGTERM from killing
	// the test process before the verb has installed its handler.
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, syscall.SIGTERM)
	defer signal.Stop(guard)

	stopped := filepath.Join(t.TempDir(), "child.stopped")
	ready := make(chan struct{})
	expiry := make(chan time.Time, 1)
	deps := fixtureLifetimeTestDependencies(nil, func(time.Duration) <-chan time.Time { return expiry }, nil)
	deps.ready = func() { close(ready) }
	done := make(chan int, 1)
	finished := false
	go func() {
		done <- runUtilHoldWithDependencies([]string{"--tag", "metasystem-job-hold-test", "--stopped-file", stopped, "--max-seconds", "1"}, deps)
	}()
	t.Cleanup(func() {
		if !finished {
			expiry <- time.Unix(1, 0)
			<-done
		}
	})
	select {
	case <-ready:
	case code := <-done:
		finished = true
		t.Fatalf("hold exited with %d before acknowledging signal readiness", code)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	code := <-done
	finished = true
	if code != 0 {
		t.Fatalf("hold must exit 0 on SIGTERM, got %d", code)
	}
	data, err := os.ReadFile(stopped)
	if err != nil || string(data) != "stopped\n" {
		t.Fatalf("stopped file wrong: %q err=%v", data, err)
	}
}
