package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// TestMissionOwnersRunInThisProcess is the U9a witness that the public
// mission actions reach the mission runner in this process (design 6.2): no
// engine child starts for status, resume or repair, the owner's own report
// comes back as the child's output did, and the supplied caller identity is
// this process, the parent the child used to classify.
func TestMissionOwnersRunInThisProcess(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	parkedHostFailureMission(t, b.root())
	owners := b.owners()
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
		process: func(process intentProcess) intentProcessResult {
			t.Errorf("an engine child ran: %v", process.argv)
			return intentProcessResult{code: 1}
		}}
	var suppliedLaunch, suppliedResolve []processIdentity
	calls := defaultIntentOwnerCalls()
	realLaunch, realResolve := calls.missionLaunch, calls.missionResolveTaint
	calls.missionLaunch = func(caller processIdentity, stdout, stderr io.Writer, root, mission, mode string) int {
		suppliedLaunch = append(suppliedLaunch, caller)
		return realLaunch(caller, stdout, stderr, root, mission, mode)
	}
	calls.missionResolveTaint = func(caller processIdentity, stdout, stderr io.Writer, request missionResolveRequest) int {
		suppliedResolve = append(suppliedResolve, caller)
		return realResolve(caller, stdout, stderr, request)
	}
	owners.delivery.calls = calls

	code, result := b.runJSON(owners, "mission", "status", "demo")
	if code != 0 || !strings.Contains(result.Summary, "mission=demo status=parked reason=host-failure") {
		t.Fatalf("mission status = %d %+v", code, result)
	}
	code, result = b.runJSON(owners, "mission", "resume", "demo")
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "mission is parked; answer its park reason before resume") {
		t.Fatalf("mission resume of a parked mission = %d %+v", code, result)
	}
	code, result = b.runJSON(owners, "mission", "repair", "demo", "--problem", "2", "--confirm-restored", strings.Repeat("b", 40), "--by", "Wido", "--reason", "restored")
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "resolve refused") {
		t.Fatalf("mission repair = %d %+v", code, result)
	}
	for _, supplied := range append(suppliedLaunch, suppliedResolve...) {
		if supplied.pid != int64(os.Getpid()) {
			t.Fatalf("an owner call supplied caller %+v, want this process %d", supplied, os.Getpid())
		}
	}
	if len(suppliedLaunch) != 1 || len(suppliedResolve) != 1 {
		t.Fatalf("owner calls: launch %d, resolve %d", len(suppliedLaunch), len(suppliedResolve))
	}
}
