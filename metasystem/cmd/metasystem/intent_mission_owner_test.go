package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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

// TestMissionRepairHumanGateRefusesAnUnannouncedAgent is the R7 witness for
// the replaced resolve-taint child: an unannounced agent runtime with a
// controlling terminal runs `mission repair` directly, and the runner's
// human-reserved gate, called in this process, refuses it as the child did.
// The child classified from itself, whose parent was the public command; the
// call supplies the public command's process, whose parent is the agent, so
// the walk starts at the same runtime. The control supplies the command's own
// caller instead: the walk starts above the agent, and the gate's answer
// changes, so the fixture discriminates exactly the identity rule.
func TestMissionRepairHumanGateRefusesAnUnannouncedAgent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stageUnannouncedAgentParent(t, root)
	request := missionResolveRequest{root: root, mission: "demo", taint: 2, variant: "restore", tree: strings.Repeat("b", 40), by: "Wido", reason: "restored"}
	calls := defaultIntentOwnerCalls()
	var stdout, stderr bytes.Buffer
	if code := calls.missionResolveTaint(currentProcessIdentity(), &stdout, &stderr, request); code != 3 ||
		!strings.Contains(stderr.String(), "taint resolution is a human-reserved act") {
		t.Fatalf("the unannounced agent's resolution was not refused by the human gate: %d %q", code, stderr.String())
	}
	var control bytes.Buffer
	calls.missionResolveTaint(entryCallerIdentity(), io.Discard, &control, request)
	if strings.Contains(control.String(), "taint resolution is a human-reserved act") {
		t.Fatalf("control: the caller-of-caller identity was refused too, so the fixture does not discriminate: %q", control.String())
	}
}
