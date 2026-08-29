// Package boundedexec runs external commands under a time bound.
//
// The metasystem shells out for work it does not own — git, gates,
// fingerprint scripts — and an unbounded child can hang a supervision cycle
// or a mission turn indefinitely (go-production-grade B4). Every such call
// runs through here: the bound comes from configuration with a stated
// default, expiry kills the process GROUP so a script's children die with
// it, and the failure says which operation ran out of which bound.
package boundedexec

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// Kind selects which bound applies. Network work has unbounded latency and
// takes the shorter bound an operator may raise on a slow link; local work
// is bounded by the machine and takes a generous one that exists to stop a
// hang, not to hurry the work.
type Kind int

const (
	Local Kind = iota
	Network
)

const (
	localKey        = "exec.local-timeout-sec"
	networkKey      = "exec.network-timeout-sec"
	localDefault    = 300
	networkDefault  = 120
	killGraceWindow = 5 * time.Second
)

// ErrTimedOut marks bound expiry, so the callers for whom a timeout is an
// ANSWER (a named ceiling was exceeded) can tell it from a failure to run.
var ErrTimedOut = errors.New("timed out")

// Bound is a time limit that KNOWS which knob produced it, so expiry can
// name the key an operator would actually raise (review foundations-5: the
// old magnitude-based guess misdirected exactly when a bound was tuned).
type Bound struct {
	Limit time.Duration
	Key   string
}

// FixedBound wraps an ad-hoc limit — a fence-derived ceiling, a test value —
// with the name of whatever authored it.
func FixedBound(limit time.Duration, key string) Bound {
	return Bound{Limit: limit, Key: key}
}

// Timeout resolves a kind's bound from the checkout's metasystem.conf,
// falling back to the stated default when the key is absent or unusable —
// a malformed bound must not disable bounding (config validate names the
// malformation; the read stays soft).
func Timeout(confPath string, kind Kind) Bound {
	key, fallback := localKey, localDefault
	if kind == Network {
		key, fallback = networkKey, networkDefault
	}
	seconds := fallback
	if raw := config.ConfValue(confPath, key, ""); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			seconds = parsed
		}
	}
	return Bound{Limit: time.Duration(seconds) * time.Second, Key: key}
}

// Run executes cmd under limit. what names the operation in the failure
// text. The command runs in its own process group and expiry signals the
// GROUP, so children do not outlive the bound; the wait is additionally
// bounded so a child that ignores the signal cannot hang the caller.
//
// The returned error on expiry is a dependency failure: the metasystem
// could not finish because something it depends on did not answer.
func Run(cmd *exec.Cmd, bound Bound, what string) error {
	return runWithDeadline(cmd, bound, what, time.After)
}

func runWithDeadline(cmd *exec.Cmd, bound Bound, what string, deadline func(time.Duration) <-chan time.Time) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-deadline(bound.Limit):
	}
	// The bound expired: signal the GROUP so the command's children die with
	// it rather than outliving the bound holding pipes open. A negative pid
	// addresses the group, which Setpgid above made the child lead.
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	// Bounded wait for the reap, so a child that somehow survives the kill
	// cannot hang the caller either.
	select {
	case <-done:
	case <-deadline(killGraceWindow):
	}
	if bound.Key == "" {
		return fmt.Errorf("%s %w after %s", what, ErrTimedOut, bound.Limit)
	}
	return fmt.Errorf("%s %w after %s (raise it with %s)",
		what, ErrTimedOut, bound.Limit, bound.Key)
}
