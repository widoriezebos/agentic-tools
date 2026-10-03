package main

import (
	"bytes"
	"fmt"

	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
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
	var suppliedLaunch, suppliedResolve []ownercall.Process
	calls := defaultIntentOwnerCalls()
	realLaunch, realResolve := calls.missionLaunch, calls.missionResolveTaint
	calls.missionLaunch = func(caller ownercall.Process, stdout, stderr io.Writer, root, mission, mode string, wait bool, top func(string) (string, error)) int {
		suppliedLaunch = append(suppliedLaunch, caller)
		return realLaunch(caller, stdout, stderr, root, mission, mode, wait, top)
	}
	calls.missionResolveTaint = func(caller ownercall.Process, stdout, stderr io.Writer, request missionResolveRequest) int {
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
		if supplied.Pid != int64(os.Getpid()) {
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
	if code := calls.missionResolveTaint(ownercall.CurrentProcess(), &stdout, &stderr, request); code != 3 ||
		!strings.Contains(stderr.String(), "taint resolution is a human-reserved act") {
		t.Fatalf("the unannounced agent's resolution was not refused by the human gate: %d %q", code, stderr.String())
	}
	var control bytes.Buffer
	calls.missionResolveTaint(ownercall.EntryCaller(), io.Discard, &control, request)
	if strings.Contains(control.String(), "taint resolution is a human-reserved act") {
		t.Fatalf("control: the caller-of-caller identity was refused too, so the fixture does not discriminate: %q", control.String())
	}
}

// missionSealBed is a process bed whose mission seal owner is recorded:
// each call appends the contract path, and calls after the first answer as a
// sealed contract does.
func missionSealBed(t *testing.T) (*processBed, intentOwners, *[]string) {
	t.Helper()
	b := newProcessBed(t)
	owners := b.owners()
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
		process: func(process intentProcess) intentProcessResult {
			t.Errorf("an engine child ran: %v", process.argv)
			return intentProcessResult{code: 1}
		}}
	var sealed []string
	calls := defaultIntentOwnerCalls()
	calls.missionSeal = func(path string) (string, []string, error) {
		sealed = append(sealed, path)
		if len(sealed) > 1 {
			return "", nil, contract.ErrAlreadySealed
		}
		return strings.Repeat("a", 64), []string{"ledger.no-gain-budget=3 does not exceed the critique cadence"}, nil
	}
	owners.delivery.calls = calls
	return b, owners, &sealed
}

// TestMissionSealChecksAndSealsAContractThenLeavesItSealed is the witness of
// the public home of the former contract-validate and contract-seal verbs: a
// mission id names plans/mission-M.contract.md, the result carries the
// digest the approval line signs and the sizing warnings, and a repeat on a
// sealed contract is unchanged with exit 0.
func TestMissionSealChecksAndSealsAContractThenLeavesItSealed(t *testing.T) {
	t.Parallel()
	witnessMissionSeal(t)
}

// witnessMissionSeal is also mission seal's idempotency witness.
func witnessMissionSeal(t *testing.T) {
	b, owners, sealed := missionSealBed(t)
	code, result := b.runJSON(owners, "mission", "seal", "demo")
	root, err := filepath.EvalSymlinks(b.root())
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "plans", "mission-demo.contract.md")
	data, _ := result.Data.(map[string]any)
	if code != 0 || result.Outcome != intentConfirmed || len(*sealed) != 1 || (*sealed)[0] != want ||
		!strings.Contains(result.Summary, strings.Repeat("a", 64)) || !strings.Contains(fmt.Sprint(data["warnings"]), "no-gain-budget") {
		t.Fatalf("mission seal = %d %+v (sealed %v, want %s)", code, result, *sealed, want)
	}
	code, result = b.runJSON(owners, "mission", "seal", "demo")
	if code != 0 || result.Outcome != intentUnchanged || len(*sealed) != 2 {
		t.Fatalf("second mission seal = %d %+v", code, result)
	}
}

// TestMissionStartWaitRunsTheMissionToItsEnd is the witness that --wait
// asks the runner for the mission's whole run in this process (the former
// internal start --foreground), and that without it the loop is detached.
func TestMissionStartWaitRunsTheMissionToItsEnd(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil }}
	var waits []bool
	calls := defaultIntentOwnerCalls()
	calls.missionLaunch = func(caller ownercall.Process, stdout, stderr io.Writer, root, mission, mode string, wait bool, _ func(string) (string, error)) int {
		waits = append(waits, wait)
		return 0
	}
	owners.delivery.calls = calls
	if code, result := b.runJSON(owners, "mission", "start", "demo", "--wait"); code != 0 {
		t.Fatalf("mission start --wait = %d %+v", code, result)
	}
	if code, result := b.runJSON(owners, "mission", "resume", "demo"); code != 0 {
		t.Fatalf("mission resume = %d %+v", code, result)
	}
	if len(waits) != 2 || !waits[0] || waits[1] {
		t.Fatalf("launch waits = %v, want [true false]", waits)
	}
}

// TestMissionStatusOfAnUnknownMissionIsRefused is EM-08: a mission with no
// state in this repository is not a status line with exit 0 (a false
// success a script reads as a live record); it is refused, exit 1, naming
// the mission. A never-started mission with a contract says so and names
// the start. A known mission's status is still a confirmed read.
func TestMissionStatusOfAnUnknownMissionIsRefused(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	parkedHostFailureMission(t, b.root())
	owners := b.owners()
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
		process: func(process intentProcess) intentProcessResult {
			t.Errorf("an engine child ran: %v", process.argv)
			return intentProcessResult{code: 1}
		}, calls: defaultIntentOwnerCalls()}
	code, result := b.runJSON(owners, "mission", "status", "nosuch")
	if code != 1 || result.Outcome != intentRefused || result.Summary != "no mission nosuch in this repository; nothing was read" {
		t.Fatalf("unknown mission status = %d %+v", code, result)
	}
	contractPath := filepath.Join(b.root(), "plans", "mission-drafted.contract.md")
	if err := os.MkdirAll(filepath.Dir(contractPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contractPath, []byte("# drafted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, result = b.runJSON(owners, "mission", "status", "drafted")
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "mission drafted has a contract but was never started") ||
		result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "mission start drafted") {
		t.Fatalf("never-started mission status = %d %+v", code, result)
	}
	if code, result = b.runJSON(owners, "mission", "status", "demo"); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("known mission status = %d %+v", code, result)
	}
}

// The status verb decides that a mission has no state from the state
// itself, never from the words of the owner's status line: an owner that
// prints anything else for a missing state still gets the refusal.
func TestMissionStatusWithoutStateIsJudgedFromTheState(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	calls := defaultIntentOwnerCalls()
	calls.missionStatus = func(stdout, stderr io.Writer, root, mission string) int {
		fmt.Fprintln(stdout, "no state here, in other words")
		return 7
	}
	owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
		process: func(process intentProcess) intentProcessResult {
			t.Errorf("an engine child ran: %v", process.argv)
			return intentProcessResult{code: 1}
		}, calls: calls}
	code, result := b.runJSON(owners, "mission", "status", "nosuch")
	if code != 1 || result.Outcome != intentRefused || result.Summary != "no mission nosuch in this repository; nothing was read" {
		t.Fatalf("mission status without state = %d %+v", code, result)
	}
}
