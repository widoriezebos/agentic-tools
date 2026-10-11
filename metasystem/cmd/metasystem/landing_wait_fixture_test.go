package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// Synthetic effects keep the child in this test process. The child still
// claims the saved admission through the public handler after Start unlocks.
func proofWaitFixtureOwners(t *testing.T, owners intentOwners, install, cwd string) intentOwners {
	t.Helper()
	launch := owners.landing.plainProve.Launch
	completed := false
	owners.landing.plainProve.Alive = func(running plain.Running) bool {
		return running.Pid == int64(os.Getpid()) && !completed
	}
	owners.landing.plainProve.Launch = func(argv []string, dir, log string) (int64, error) {
		if launch != nil {
			pid, err := launch(argv, dir, log)
			if err != nil || pid != int64(os.Getpid()) && pid != int64(os.Getppid()) {
				return pid, err
			}
		}
		return int64(os.Getpid()), nil
	}
	owners.landing.plainProve.WaitPoll = func() {
		if completed {
			t.Fatal("fixture child completed without a result belonging to its admission")
		}
		running, recorded, _, err := plain.ReadRunning(install, owners.landing.plainProve)
		if err != nil || !recorded {
			t.Fatalf("fixture child lost its admission: %+v %v", running, err)
		}
		args := []string{"--wait", "--attempt", running.Attempt, "--json"}
		for flag, enabled := range map[string]bool{"--gate": running.Gate, "--trunk": running.Trunk, "--impact": running.Impact} {
			if enabled {
				args = append(args, flag)
			}
		}
		var output bytes.Buffer
		_ = runIntentIn(mustIntentCommand(t, "landing prove"), args, &output, &output, cwd, owners)
		completed = true
		if current, ok, _, _ := plain.ReadRunning(install, owners.landing.plainProve); ok && current.Admission != nil && current.Admission.State == "launched" {
			t.Fatalf("fixture child refused to claim its admission: %s", &output)
		}
	}
	return owners
}
