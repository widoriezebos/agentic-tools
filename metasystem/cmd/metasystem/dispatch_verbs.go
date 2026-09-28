package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The dispatch family is the job-record lifecycle surface (internal/dispatch):
// the single writer that creates a job's record, completes its setup, stamps a
// protocol error, and compare-and-swaps its status. Each write holds the
// exclusive per-record lock and lands atomically.

// recordExit maps a lifecycle error to a process exit code, printing any
// message the refusal carries to stderr. A nil error is exit 0.
func recordExit(err error) int {
	return recordExitTo(os.Stderr, err)
}

// recordExitTo writes an owner error's refusal to stderr and returns its
// exit.
func recordExitTo(stderr io.Writer, err error) int {
	if err == nil {
		return 0
	}
	var op *dispatchcore.OpError
	if errors.As(err, &op) {
		if op.Message != "" || op.Reason != "" {
			fmt.Fprintln(stderr, op.Error())
		}
		return op.Code
	}
	fmt.Fprintln(stderr, err)
	return 1
}

func breachStopOrderingHumanWith(root string, caller lease.ClassifyResult, by string, now time.Time,
	enrolledName func(string, time.Time) (string, error)) (string, error) {
	typed := strings.TrimPrefix(by, "human:")
	if caller.Class != lease.ClassHuman {
		if typed != "" {
			return "", fmt.Errorf("breach stop: --by names the person ordering the stop; a %s caller records the custodian and takes no --by", caller.Class)
		}
		return "", nil
	}
	name, err := enrolledName(root, now)
	if err != nil {
		return "", fmt.Errorf("breach stop: the stop is admitted for a person and records who ordered it, and no enrolled person was proven here (%v); run it at the enrolled terminal, or enroll this one with metasystem system enroll --name NAME", err)
	}
	if typed != "" && typed != name {
		return "", fmt.Errorf("breach stop: --by %s is not the person enrolled at this terminal (%s); nothing was done", typed, name)
	}
	return name, nil
}
