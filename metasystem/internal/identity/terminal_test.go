package identity

import (
	"os"
	"testing"
)

type stubProbe struct {
	entry FixtureEntry
	has   bool
}

func (s stubProbe) FixtureEntry(int64) (FixtureEntry, bool) { return s.entry, s.has }

func TestFixtureTerminalFactDecides(t *testing.T) {
	has, ok := ControllingTerminal(1, stubProbe{entry: FixtureEntry{Terminal: true, HasTerminal: true}, has: true})
	if !ok || !has {
		t.Fatalf("a staged true fact decides: has=%v ok=%v", has, ok)
	}
	has, ok = ControllingTerminal(1, stubProbe{entry: FixtureEntry{Terminal: false, HasTerminal: true}, has: true})
	if !ok || has {
		t.Fatalf("a staged false fact decides over any kernel truth: has=%v ok=%v", has, ok)
	}
}

func TestKernelAnswersForALiveProcess(t *testing.T) {
	// The value depends on where the suite runs (a desk has a
	// terminal, a runner does not); the contract is that the kernel
	// ANSWERS for a live pid either way.
	if _, ok := ControllingTerminal(int64(os.Getpid()), nil); !ok {
		t.Fatal("the kernel must answer for our own live process")
	}
}

func TestKernelExecutablePathForSelf(t *testing.T) {
	if executable, ok := ExecutablePath(int64(os.Getpid())); !ok || executable == "" {
		t.Fatalf("the kernel did not return this process's executable: %q ok=%v", executable, ok)
	}
}

func TestDeadPidHasNoAnswer(t *testing.T) {
	if _, ok := ControllingTerminal(1<<30, nil); ok {
		t.Fatal("a nonexistent pid answers nothing")
	}
}
