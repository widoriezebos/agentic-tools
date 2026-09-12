package missionrunner

import (
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
)

// fakeGroup is one process group in the fake kernel: whether it answers
// signal 0, whether a live member with readable argv remains, how its
// ownership reads, and which signal ends it (none: it survives everything).
type fakeGroup struct {
	alive       bool
	substantive bool
	ownership   janitor.GroupOwnershipOutcome
	diesOn      syscall.Signal
	// indeterminateProbes is how many ownership probes read INDETERMINATE
	// before the recorded ownership shows (a member between fork and exec).
	indeterminateProbes int
}

// fakeKernel drives the wind-down seam: an artificial clock that advances
// by exactly what the ladder sleeps, and a process table it signals.
type fakeKernel struct {
	t        *testing.T
	now      time.Time
	groups   map[int]*fakeGroup
	signals  []string
	slept    time.Duration
	previous struct {
		windDown   windDownSeam
		stopSignal func(int, syscall.Signal) error
	}
}

type windDownSeam = struct {
	now                       func() time.Time
	sleep                     func(time.Duration)
	groupAlive                func(pgid int) bool
	groupHasSubstantiveMember func(pgid int) bool
	groupOwnership            func(pgid int, tag string, grant fixtureauth.GroupOwnershipGrant) janitor.GroupOwnershipOutcome
}

// installFakeClock puts the wind-down and stop ladders on an artificial
// clock and leaves their process probes and signals real: every wait costs
// nothing, every fact stays a fact.
func installFakeClock(t *testing.T) *fakeKernel {
	t.Helper()
	kernel := &fakeKernel{t: t, now: time.Date(2026, 9, 12, 15, 0, 0, 0, time.UTC)}
	kernel.previous.windDown = windDown
	windDown.now = func() time.Time { return kernel.now }
	windDown.sleep = func(d time.Duration) { kernel.now = kernel.now.Add(d); kernel.slept += d }
	t.Cleanup(func() { windDown = kernel.previous.windDown })
	return kernel
}

// installFakeKernel is the clock plus a process table: the probes answer
// from the groups given here, and signals end the groups they say they do.
func installFakeKernel(t *testing.T, groups map[int]*fakeGroup) *fakeKernel {
	t.Helper()
	kernel := installFakeClock(t)
	kernel.groups = groups
	kernel.previous.stopSignal = stopSignal
	windDown.groupAlive = func(pgid int) bool {
		group, ok := kernel.groups[pgid]
		return ok && group.alive
	}
	windDown.groupHasSubstantiveMember = func(pgid int) bool {
		group, ok := kernel.groups[pgid]
		return ok && group.alive && group.substantive
	}
	windDown.groupOwnership = func(pgid int, _ string, _ fixtureauth.GroupOwnershipGrant) janitor.GroupOwnershipOutcome {
		group, ok := kernel.groups[pgid]
		if !ok {
			return janitor.GroupNotOwned
		}
		if group.indeterminateProbes > 0 {
			group.indeterminateProbes--
			return janitor.GroupIndeterminate
		}
		return group.ownership
	}
	stopSignal = func(pid int, signal syscall.Signal) error {
		kernel.signals = append(kernel.signals, signal.String()+" "+itoa(pid))
		group, ok := kernel.groups[-pid]
		if ok && group.alive && group.diesOn != 0 && signal == group.diesOn {
			group.alive = false
		}
		return nil
	}
	t.Cleanup(func() { stopSignal = kernel.previous.stopSignal })
	return kernel
}

func itoa(value int) string { return strconv.Itoa(value) }

func TestWindDownLadderOnAFakeKernel(t *testing.T) {
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "1000")
	engine := &Engine{Mission: "mr-fake", Root: t.TempDir()}
	tag := "metasystem-job-mr-owned-fake"

	t.Run("a dead group is already gone and never signalled", func(t *testing.T) {
		kernel := installFakeKernel(t, map[int]*fakeGroup{})
		result, err := engine.terminateGroup(4100, tag, false)
		if err != nil || result != TerminationAlreadyGone || len(kernel.signals) != 0 {
			t.Fatalf("result = %q, %v, signals %v", result, err, kernel.signals)
		}
	})
	t.Run("a live group that is not provably ours is left to the census", func(t *testing.T) {
		kernel := installFakeKernel(t, map[int]*fakeGroup{4200: {alive: true, substantive: true, ownership: janitor.GroupNotOwned, diesOn: syscall.SIGTERM}})
		result, err := engine.terminateGroup(4200, tag, false)
		if err != nil || result != TerminationAlreadyGone || len(kernel.signals) != 0 || !kernel.groups[4200].alive {
			t.Fatalf("result = %q, %v, signals %v, alive %v", result, err, kernel.signals, kernel.groups[4200].alive)
		}
	})
	t.Run("an owned group that honours TERM ends on TERM alone", func(t *testing.T) {
		kernel := installFakeKernel(t, map[int]*fakeGroup{4300: {alive: true, substantive: true, ownership: janitor.GroupOwned, diesOn: syscall.SIGTERM}})
		result, err := engine.terminateGroup(4300, tag, false)
		if err != nil || result != TerminationTerm || len(kernel.signals) != 1 || !strings.HasPrefix(kernel.signals[0], "terminated") {
			t.Fatalf("result = %q, %v, signals %v", result, err, kernel.signals)
		}
		if kernel.slept > 0 {
			t.Fatalf("a group that died on TERM still cost %s of waiting", kernel.slept)
		}
	})
	t.Run("a TERM-immune owned group dies through KILL after the grace", func(t *testing.T) {
		kernel := installFakeKernel(t, map[int]*fakeGroup{4400: {alive: true, substantive: true, ownership: janitor.GroupOwned, diesOn: syscall.SIGKILL}})
		result, err := engine.terminateGroup(4400, tag, false)
		if err != nil || result != TerminationKill || len(kernel.signals) != 2 || !strings.HasPrefix(kernel.signals[1], "killed") {
			t.Fatalf("result = %q, %v, signals %v", result, err, kernel.signals)
		}
		if kernel.slept < 2*time.Second {
			t.Fatalf("KILL was sent before the TERM grace: waited %s", kernel.slept)
		}
	})
	t.Run("a group that reads indeterminate while a member execs is probed again, then ended", func(t *testing.T) {
		kernel := installFakeKernel(t, map[int]*fakeGroup{4500: {alive: true, substantive: true, ownership: janitor.GroupOwned, diesOn: syscall.SIGTERM, indeterminateProbes: 2}})
		result, err := engine.terminateGroup(4500, tag, false)
		if err != nil || result != TerminationTerm || len(kernel.signals) != 1 {
			t.Fatalf("result = %q, %v, signals %v", result, err, kernel.signals)
		}
	})
	t.Run("a group that stays indeterminate is never signalled", func(t *testing.T) {
		kernel := installFakeKernel(t, map[int]*fakeGroup{4550: {alive: true, substantive: true, ownership: janitor.GroupOwned, diesOn: syscall.SIGTERM, indeterminateProbes: 99}})
		result, err := engine.terminateGroup(4550, tag, false)
		if err != nil || result != TerminationAlreadyGone || len(kernel.signals) != 0 {
			t.Fatalf("result = %q, %v, signals %v", result, err, kernel.signals)
		}
	})
	t.Run("a pgid recycled by a foreign process after TERM is not killed", func(t *testing.T) {
		// The owned group ignores TERM; by the time the grace has passed the
		// ownership probe reads a foreign process on the same pgid, and the
		// kill is skipped rather than aimed at a stranger.
		kernel := installFakeKernel(t, map[int]*fakeGroup{4600: {alive: true, substantive: true, ownership: janitor.GroupOwned}})
		termed := false
		previous := stopSignal
		stopSignal = func(pid int, signal syscall.Signal) error {
			if signal == syscall.SIGTERM {
				termed = true
			}
			if signal == syscall.SIGKILL {
				t.Fatal("a provably foreign group was killed")
			}
			return previous(pid, signal)
		}
		windDown.groupOwnership = func(int, string, fixtureauth.GroupOwnershipGrant) janitor.GroupOwnershipOutcome {
			if termed {
				return janitor.GroupNotOwned
			}
			return janitor.GroupOwned
		}
		result, err := engine.terminateGroup(4600, tag, false)
		if err != nil || result != TerminationTerm || len(kernel.signals) != 1 {
			t.Fatalf("result = %q, %v, signals %v", result, err, kernel.signals)
		}
	})
	t.Run("a group down to zombies after KILL is finished work", func(t *testing.T) {
		kernel := installFakeKernel(t, map[int]*fakeGroup{4700: {alive: true, substantive: true, ownership: janitor.GroupOwned}})
		previous := stopSignal
		stopSignal = func(pid int, signal syscall.Signal) error {
			if signal == syscall.SIGKILL {
				kernel.groups[4700].substantive = false // the shell is a zombie now; the pgid still answers
			}
			return previous(pid, signal)
		}
		result, err := engine.terminateGroup(4700, tag, false)
		if err != nil || result != TerminationKill {
			t.Fatalf("result = %q, %v", result, err)
		}
	})
	t.Run("a group that survives KILL with running work is a loud error", func(t *testing.T) {
		kernel := installFakeKernel(t, map[int]*fakeGroup{4800: {alive: true, substantive: true, ownership: janitor.GroupOwned}})
		result, err := engine.terminateGroup(4800, tag, false)
		if err == nil || result != TerminationKill || !strings.Contains(err.Error(), "survived the kill-through window") {
			t.Fatalf("result = %q, %v", result, err)
		}
		if len(kernel.signals) != 2 {
			t.Fatalf("signals = %v", kernel.signals)
		}
	})
}
