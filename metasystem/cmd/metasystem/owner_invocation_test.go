package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// stageUnannouncedAgentParent makes this test process's parent an
// unannounced agent runtime with a controlling terminal, in the fixture
// identity table of root: the parent's argv matches an installed adapter's
// signature, and both it and this process carry a terminal.
func stageUnannouncedAgentParent(t *testing.T, root string) {
	t.Helper()
	// The parent is an agent CLI the registry recognizes: the built-in fake
	// runtime's signature matches metasystem-fake-agent.
	table := filepath.Join(t.TempDir(), "agent-parent-table.json")
	rows := fmt.Sprintf(`{"%d": {"terminal": true}, "%d": {"terminal": true, "pidStartedAt": 1700000000, "command": "metasystem-fake-agent --print"}}`, os.Getpid(), os.Getppid())
	if err := os.WriteFile(table, []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
}

// TestOwnerCallCoordinatorHumanGateRefusesAnUnannouncedAgent is the
// VOA-02-R2 witness: an unannounced agent runtime with a controlling terminal
// runs `settings coordinator --declare --by NAME` directly, and the in-process
// brain owner's human gate refuses it, as the owner child did. The child
// classified its parent, the public command; the call supplies the current
// process, the same node, so the signature walk starts at the agent runtime.
// Supplying the public command's own caller instead would start the walk
// above the agent and admit the act; the control shows the fixture
// discriminates exactly that.
func TestOwnerCallCoordinatorHumanGateRefusesAnUnannouncedAgent(t *testing.T) {
	root := migratedHumanTerminalRoot(t, "coordinator-machine", os.Getpid())
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir())
	ledger := goal.ExistingLedgerIdentity(root)
	stageUnannouncedAgentParent(t, root)

	var suppliedCallers []ownercall.Process
	calls := defaultIntentOwnerCalls()
	realBrain := calls.brain
	calls.brain = func(choice string, caller ownercall.Process, stdout, stderr io.Writer, root, by string) int {
		suppliedCallers = append(suppliedCallers, caller)
		return realBrain(choice, caller, stdout, stderr, root, by)
	}
	owners := defaultIntentOwners()
	owners.resolver = stateroot.NewResolver(fakeTop(root), noExecutable)
	owners.delivery = &intentDeliveryOwners{calls: calls}
	settings, _ := findIntentCommand("settings coordinator")
	var stdout, stderr bytes.Buffer
	code := runIntentIn(settings, []string{"--declare", "--by", "Wido", "--json"}, &stdout, &stderr, root, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("no JSON result: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	if code == 0 || result.Outcome != intentRefused || result.Summary != "brain declare is a human act; run it from an agent-free terminal" {
		t.Fatalf("an unannounced agent's direct declaration was not refused by the human gate: code=%d result=%+v", code, result)
	}
	if len(suppliedCallers) != 1 || suppliedCallers[0].Pid != int64(os.Getpid()) {
		t.Fatalf("the edge supplied %+v, want the current process %d", suppliedCallers, os.Getpid())
	}
	if state := brain.Read(root, ledger); state.State != brain.Undeclared {
		t.Fatalf("the refused act declared: %+v", state)
	}

	// Control: the same owner and fixture, supplied the public command's own
	// caller (the agent), starts the signature walk above it and admits.
	var controlOut, controlErr bytes.Buffer
	if status := brainDeclare(ownercall.EntryCaller(), &controlOut, &controlErr, root, "Wido", false); status != 0 {
		t.Fatalf("control: the caller-of-caller identity did not admit, so the fixture does not discriminate: %d %s", status, controlErr.String())
	}
	if state := brain.Read(root, ledger); state.State != brain.Declared {
		t.Fatalf("control declaration missing: %+v", state)
	}
	if !strings.Contains(controlOut.String(), `"state"`) {
		t.Fatalf("control output: %q", controlOut.String())
	}
}

// ownerRequestAuthority is the authority content of a synced mutation
// request: who acts, how the caller classified, and the epoch it may use.
type ownerRequestAuthority struct {
	Actor          goal.Actor
	CallerClass    string
	EpochAuthority string
	ClaimEpoch     int64
	Error          string
}

func authorityOf(req goal.VerbRequest, err error) ownerRequestAuthority {
	if err != nil {
		return ownerRequestAuthority{Error: err.Error()}
	}
	return ownerRequestAuthority{Actor: req.Actor, CallerClass: req.CallerClass, EpochAuthority: req.EpochAuthority, ClaimEpoch: req.ClaimEpoch}
}

// TestOwnerSyncRequestChild is the subprocess-era edge: a child of this test
// binary builds the request exactly as the former `goal edit|release|handover
// --lineage L` child did, classifying its parent.
func TestOwnerSyncRequestChild(t *testing.T) {
	verb := os.Getenv("GO_WANT_OWNER_SYNC_REQUEST_CHILD")
	if verb == "" {
		return
	}
	root, lineage := os.Getenv("OWNER_SYNC_REQUEST_ROOT"), os.Getenv("OWNER_SYNC_REQUEST_LINEAGE")
	var authority ownerRequestAuthority
	if verb == "release" {
		authority = authorityOf(syncStoppingReq(verb, root, "", lineage))
	} else {
		authority = authorityOf(syncReq(verb, root, "", lineage))
	}
	encoded, _ := json.Marshal(authority)
	fmt.Println(string(encoded))
	os.Exit(0)
}

// TestOwnerCallRequestAuthorityParityWithTheChild is the R7 witness for the
// landing path's replaced edges: on the same fixture, the in-process owner
// call and the former child produce the same actor, holder classification,
// epoch authority and claim epoch, both while this process holds the landing
// lease (the owner's own claim) and before it holds anything (the refusal
// side: no holder epoch authority).
func TestOwnerCallRequestAuthorityParityWithTheChild(t *testing.T) {
	t.Setenv("METASYSTEM_PROOF_CONTROL_ROOT", "")
	t.Setenv("METASYSTEM_PROOF_ATTEMPT", "")
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "")
	root := syncedClaimedGoalFixture(t)
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "landing-machine")
	compare := func(label string, wantHolder bool) {
		t.Helper()
		for _, verb := range []string{"edit", "release", "handover"} {
			child := exec.Command(os.Args[0], "-test.run=^TestOwnerSyncRequestChild$", "-test.count=1")
			child.Env = append(os.Environ(), "GO_WANT_OWNER_SYNC_REQUEST_CHILD="+verb,
				"OWNER_SYNC_REQUEST_ROOT="+root, "OWNER_SYNC_REQUEST_LINEAGE="+batchowner.LandingOwnerLineage)
			output, err := child.Output()
			if err != nil {
				t.Fatalf("%s %s child: %v: %s", label, verb, err, output)
			}
			line := strings.TrimSpace(string(output))
			if index := strings.LastIndex(line, "\n"); index >= 0 {
				line = line[index+1:]
			}
			var fromChild ownerRequestAuthority
			if err := json.Unmarshal([]byte(line), &fromChild); err != nil {
				t.Fatalf("%s %s child output %q: %v", label, verb, output, err)
			}
			inProcess := authorityOf(ownerSyncRequest(batchowner.LandingOwnerInvocation(), verb, root, verb == "release"))
			if fromChild != inProcess {
				t.Fatalf("%s %s: in-process authority %+v differs from the child's %+v", label, verb, inProcess, fromChild)
			}
			holder := inProcess.EpochAuthority == goal.EpochAuthorityHolder && inProcess.ClaimEpoch > 0
			if holder != wantHolder || inProcess.Actor.Lineage != batchowner.LandingOwnerLineage || inProcess.Actor.Machine != "landing-machine" {
				t.Fatalf("%s %s: authority %+v, want holder=%t under the landing lineage", label, verb, inProcess, wantHolder)
			}
		}
	}
	compare("before the lease", false)
	held, err := batchowner.AcquireBatchOwner(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := held.Retire(); err != nil {
			t.Errorf("retire owner: %v", err)
		}
	})
	compare("as the lease holder", true)
}

// TestBatchProofCancellationSparesTheResidentOwner is the VOA-15 witness:
// the landing owner's proofs stay child processes, so cancelling one proof —
// goal-stop signals the proof's recorded launcher, the child — ends that proof
// alone while the owner, proving another batch at the same time, survives and
// completes it.
func TestBatchProofCancellationSparesTheResidentOwner(t *testing.T) {
	state := t.TempDir()
	proof := filepath.Join(state, "proof")
	script := "#!/usr/bin/env bash\nset -euo pipefail\ngoal=; result=\n" +
		"while (( $# )); do case $1 in --goal) goal=$2; shift 2;; --result) result=$2; shift 2;; *) shift;; esac; done\n" +
		"printf '%s\\n' $$ >\"$PROOF_STATE/$goal.pid\"\n" +
		"if [[ $goal == cancelled ]]; then exec sleep 600; fi\n" +
		"while [[ ! -e \"$PROOF_STATE/release\" ]]; do sleep 0.05; done\n" +
		"printf '{}\\n' >\"$result\"\nexit 0\n"
	script = withTestRunEnvelope(t, script)
	if err := testexec.WriteFile(proof, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROOF_STATE", state)
	dependencies := batchowner.BatchExecutionDependencies{
		Executable: func() (string, error) { return proof, nil },
		Checkout: func(string, string) (string, func() error, error) {
			return t.TempDir(), func() error { return nil }, nil
		},
		TopLevel: func(root string) (string, error) { return root, nil },
		ReadGit:  func(string, ...string) (string, error) { return "", fmt.Errorf("no git in this witness") },
	}
	type outcome struct {
		goal string
		err  error
	}
	done := make(chan outcome, 2)
	// Each proof child reports its pid through a FIFO: the read blocks until
	// the child writes, so the witness waits on the event, not on a clock.
	for _, goalID := range []string{"cancelled", "completed"} {
		if err := syscall.Mkfifo(filepath.Join(state, goalID+".pid"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, goalID := range []string{"cancelled", "completed"} {
		goalID := goalID
		go func() {
			_, err := batchowner.LaunchBatchTipProofWithDependencies(batchowner.BatchProofLaunch{Root: t.TempDir(), BatchID: "batch-" + goalID, GoalID: goalID,
				Tree: strings.Repeat("a", 40), Mode: "auto", ResultPath: filepath.Join(state, goalID+".json")}, dependencies)
			done <- outcome{goalID, err}
		}()
	}
	pidOf := func(goalID string) int {
		t.Helper()
		type report struct {
			data []byte
			err  error
		}
		reported := make(chan report, 1)
		go func() {
			data, err := os.ReadFile(filepath.Join(state, goalID+".pid"))
			reported <- report{data, err}
		}()
		select {
		case <-t.Context().Done():
			t.Fatalf("proof %s never started", goalID)
		case got := <-reported:
			if got.err != nil {
				t.Fatal(got.err)
			}
			var pid int
			if _, err := fmt.Sscan(string(got.data), &pid); err != nil {
				t.Fatal(err)
			}
			return pid
		}
		return 0
	}
	cancelled, completed := pidOf("cancelled"), pidOf("completed")
	if cancelled == os.Getpid() || completed == os.Getpid() || cancelled == completed {
		t.Fatalf("proof launchers are not their own processes: owner=%d cancelled=%d completed=%d", os.Getpid(), cancelled, completed)
	}
	process, err := os.FindProcess(cancelled)
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Signal(syscallTERM); err != nil {
		t.Fatal(err)
	}
	first := <-done
	if first.goal != "cancelled" || first.err == nil {
		t.Fatalf("the cancelled proof did not end first with its error: %+v", first)
	}
	if err := os.WriteFile(filepath.Join(state, "release"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	second := <-done
	if second.goal != "completed" || second.err != nil {
		t.Fatalf("the owner did not complete the other batch's proof after the cancellation: %+v", second)
	}
}

var syscallTERM = syscall.SIGTERM
