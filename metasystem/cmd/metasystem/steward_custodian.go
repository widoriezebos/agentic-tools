package main

import (
	"os"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// stewardCustodianFacts are the process facts the resident runner's
// custodian readiness reads; production reads the kernel and the real
// classifier, tests supply them.
type stewardCustodianFacts struct {
	// pid is the runner's own process.
	pid int64
	// sessionLeader reports whether the runner leads its own session.
	sessionLeader func() bool
	// classify is lease.Classify: who the supplied caller is.
	classify func(root string, pid int64) (lease.Classification, error)
}

func productionStewardCustodianFacts() stewardCustodianFacts {
	pid := int64(os.Getpid())
	return stewardCustodianFacts{
		pid: pid,
		sessionLeader: func() bool {
			sid, err := unix.Getsid(0)
			return err == nil && int64(sid) == pid
		},
		classify: lease.Classify,
	}
}

// stewardRunnerCustodianReady is the resident runner's BreachStopReady: the
// runner acts as the stop custodian once it leads its own session and the
// real classifier names the runner itself STEWARD, which it does only when
// no ancestor is recognized (the arming process has exited and the runner
// was reparented) and it has no controlling terminal. Until then the stop
// custodian gate would judge the arming ancestor instead, so the pass is
// deferred rather than reported FAILED. Readiness latches: the runner's
// standing does not revert once its arming parent is gone.
func stewardRunnerCustodianReady(root string, facts stewardCustodianFacts) func() bool {
	ready := false
	return func() bool {
		if ready {
			return true
		}
		if !facts.sessionLeader() {
			return false
		}
		classification, err := facts.classify(root, facts.pid)
		if err != nil || classification.Class != lease.ClassSteward || classification.Pid != facts.pid {
			return false
		}
		ready = true
		return true
	}
}
