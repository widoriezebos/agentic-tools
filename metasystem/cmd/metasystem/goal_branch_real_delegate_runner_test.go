package main

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// The real-delegate fixture's cleanup joins the disarmed steward runner by
// its exact identity (testutil.AwaitExactExit, which counts a zombie as
// exited) and then probes the recorded pid once more. A runner that has
// exited but whose new parent has not yet reaped it is still a kernel entry
// with its old identity: that zombie has not outlived cleanup, and neither
// has a process the kernel is already tearing down. Only the same identity,
// alive and running, has.
func TestStewardRunnerOutlivedCountsOnlyARunningRunner(t *testing.T) {
	t.Parallel()
	started := time.Unix(1790593402, 0)
	runner := steward.RunnerRecord{Pid: 4242, StartTicks: 858970, BootID: "boot", PidStartedAt: started.Unix()}
	same := identity.Exact{Pid: 4242, StartedAt: started, StartTicks: 858970, BootID: "boot"}
	zombie, exiting, other := same, same, same
	zombie.Zombie, exiting.Exiting, other.StartTicks = true, true, 1
	secondsOnly := steward.RunnerRecord{Pid: 4242, PidStartedAt: started.Unix()}
	cases := []struct {
		name   string
		runner steward.RunnerRecord
		live   identity.Exact
		state  identity.Liveness
		want   bool
	}{
		{"running runner", runner, same, identity.Alive, true},
		{"running runner by seconds identity", secondsOnly, identity.Exact{Pid: 4242, StartedAt: started}, identity.Alive, true},
		{"unreaped zombie", runner, zombie, identity.Alive, false},
		{"exiting", runner, exiting, identity.Alive, false},
		{"pid reused", runner, other, identity.Alive, false},
		{"gone", runner, identity.Exact{}, identity.Dead, false},
	}
	for _, c := range cases {
		if got := stewardRunnerOutlived(c.runner, c.live, c.state); got != c.want {
			t.Errorf("%s: outlived=%t, want %t", c.name, got, c.want)
		}
	}
}
