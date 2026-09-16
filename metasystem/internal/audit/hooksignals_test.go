package audit

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
)

var (
	hookSignalGuard     = make(chan os.Signal, 3)
	hookSignalGuardOnce sync.Once
)

// armHookSignalGuard gives hook subprocesses dispositions that Bash can trap.
// Bash cannot trap signals ignored on entry, so a test binary launched by nohup
// must own HUP, INT, and TERM before it execs a hook. No test in this package runs
// in parallel, making the process-wide guard safe. The drain restores and re-raises
// signals sent to the test binary so the guard cannot swallow launcher cancellation.
func armHookSignalGuard() {
	hookSignalGuardOnce.Do(func() {
		go func() {
			for sig := range hookSignalGuard {
				signal.Reset(sig)
				_ = syscall.Kill(os.Getpid(), sig.(syscall.Signal))
				signal.Notify(hookSignalGuard, sig)
			}
		}()
	})
	signal.Notify(hookSignalGuard, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
}

func TestHookStartSignalsSurviveALauncherThatIgnoresSIGHUP(t *testing.T) {
	signal.Ignore(syscall.SIGHUP)
	t.Cleanup(armHookSignalGuard)
	if !signal.Ignored(syscall.SIGHUP) {
		t.Fatal("SIGHUP disposition is not ignored")
	}
	armHookSignalGuard()

	hook := mutatedStartHook(t, "  kill -HUP $$\n")
	stdout, _, status := runHookStart(t, hook, nil, false)
	if status != 129 || stdout != hookStartInterrupted+"\n" {
		t.Fatalf("signal = status %d stdout %q", status, stdout)
	}
}
