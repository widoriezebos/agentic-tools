package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestLandingProveFreshExecutesAfterGreenAndRecordsRequest(t *testing.T) {
	t.Parallel()
	b := newReplayVerbBed(t)
	b.prepareBatch(t)
	calls := 0
	b.fail = func(_ *exec.Cmd, only string) (string, error) {
		if only == "" {
			calls++
		}
		return "landing environment fixture\nLANDING-CHECKED\t0\n", nil
	}
	priorAttempt := ""
	for index, args := range [][]string{{"prove", "--wait", "--json"}, {"prove", "--fresh", "--wait", "--json"}} {
		code, out := b.run(t, b.root, args...)
		var result struct{ Data plain.Result }
		if err := json.Unmarshal([]byte(out), &result); err != nil || code != 0 || result.Data.Result != plain.Green || result.Data.Fresh != (index == 1) {
			t.Fatalf("proof %d exit=%d result=%+v decode=%v\n%s", index, code, result, err, out)
		}
		if result.Data.Attempt == "" || result.Data.Attempt == priorAttempt {
			t.Fatalf("proof %d returned an existing attempt %q", index, result.Data.Attempt)
		}
		priorAttempt = result.Data.Attempt
		stored, ok, err := plain.LastResult(b.install)
		if err != nil || !ok || stored.Fresh != result.Data.Fresh {
			t.Fatalf("stored fresh=%t err=%v", stored.Fresh, err)
		}
	}
	if calls != 2 {
		t.Fatalf("fresh reused a prior green: executions=%d", calls)
	}
}

func TestLandingProveFreshLaunchCarriesRequest(t *testing.T) {
	t.Parallel()
	b := newReplayVerbBed(t)
	b.prepareBatch(t)
	b.fail = func(_ *exec.Cmd, _ string) (string, error) {
		return "landing environment fixture\nLANDING-CHECKED\t0\n", nil
	}
	if code, out := b.run(t, b.root, "prove", "--wait", "--json"); code != 0 {
		t.Fatalf("initial green exit=%d: %s", code, out)
	}
	launches := 0
	b.owners.landing.plainProve.Launch = func(argv []string, _, _ string) (int64, error) {
		launches++
		if !slices.Contains(argv, "--fresh") {
			t.Fatalf("detached command lost fresh: %v", argv)
		}
		return int64(os.Getpid()), nil
	}
	code, out := b.run(t, b.root, "prove", "--fresh", "--json")
	var result struct{ Data plain.Running }
	if err := json.Unmarshal([]byte(out), &result); err != nil || code != 0 || !result.Data.Fresh {
		t.Fatalf("exit=%d result=%+v decode=%v\n%s", code, result, err, out)
	}
	running, ok, _, err := plain.ReadRunning(b.install, b.owners.landing.plainProve)
	if err != nil || !ok || !running.Fresh {
		t.Fatalf("recorded fresh=%t ok=%t err=%v", running.Fresh, ok, err)
	}
	code, out = b.run(t, b.root, "prove", "--fresh", "--json")
	if code != 0 || !strings.Contains(out, "already proving") || launches != 1 {
		t.Fatalf("repeated fresh started another proof: exit=%d launches=%d\n%s", code, launches, out)
	}
	code, out = b.run(t, b.root, "prove", "--wait", "--attempt", running.Attempt, "--json")
	var finished struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(out), &finished); err != nil || code != 0 || !finished.Data.Fresh || finished.Data.Result != plain.Green {
		t.Fatalf("detached result exit=%d result=%+v decode=%v\n%s", code, finished, err, out)
	}
}

func TestLandingGateFreshWithRealGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	marker := filepath.Join(t.TempDir(), "executions")
	impactWrite(t, root, "go.mod", "module example.com/fresh-gate\n\ngo 1.23\n")
	impactWrite(t, root, "owner/owner_test.go", fmt.Sprintf(`package owner
import ("os"; "testing")
func TestSelected(t *testing.T) {
 t.Parallel()
 f, err := os.OpenFile(%q, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
 if err != nil { t.Fatal(err) }
 defer f.Close()
 if _, err := f.WriteString("executed\n"); err != nil { t.Fatal(err) }
}
`, marker))
	tools := t.TempDir()
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(goPath, filepath.Join(tools, "go")); err != nil {
		t.Fatal(err)
	}
	environment := proofrun.TestingEnvironment(os.Environ(), map[string]string{"PATH": tools, "GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off"})
	contract := testpolicy.Contract{SchemaVersion: 2, Groups: []testpolicy.Group{{ID: "unit/owner", Adapter: "go", CWD: ".", Packages: []string{"owner"}, Tests: json.RawMessage(`"all"`)}}}
	b := newMergeGateBed(t)
	b.fail = func(_ *exec.Cmd, _ string) (string, error) {
		var out, problem bytes.Buffer
		code := runNamedTestGroups(root, contract, []string{"unit/owner"}, environment, map[string]string{"unit/owner": "owner"}, &out, &problem)
		if code != 0 {
			t.Fatalf("real Go exit=%d\n%s\n%s", code, &out, &problem)
		}
		return out.String(), nil
	}
	gateResult(t, b, 0)
	gateResult(t, b, 0)
	before, err := os.ReadFile(marker)
	if err != nil || strings.Count(string(before), "executed\n") != 2 {
		t.Fatalf("ordinary gate did not retain baseline and tip: %q %v", before, err)
	}
	code, out := b.run(t, b.root, "prove", "--gate", "--fresh", "--wait", "--json")
	var result struct{ Data plain.Result }
	after, readErr := os.ReadFile(marker)
	if err := json.Unmarshal([]byte(out), &result); err != nil || code != 0 || !result.Data.Fresh || result.Data.Result != plain.Green || readErr != nil || strings.Count(string(after), "executed\n") != 4 {
		t.Fatalf("fresh gate must execute baseline and tip again: exit=%d result=%+v executions=%q read=%v decode=%v\n%s", code, result, after, readErr, err, out)
	}
}

// This witness certifies the shipped contract and ledger, so it reads their live files.
func TestLandingFreshRealContractLedgerAndPlan(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	contract, err := testpolicy.Load(filepath.Join(root, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "plans", "goals", "*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("ledger pages=%d err=%v", len(paths), err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	record, problems := goal.ParseFile(data)
	if record == nil || len(problems) != 0 {
		t.Fatalf("real ledger record: %v", problems)
	}
	if record.Risk == nil {
		t.Fatal("real goal has no risk record")
	}
	changed, err := filepath.Rel(root, paths[0])
	if err != nil {
		t.Fatal(err)
	}
	plan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{RequestedMode: testpolicy.ModeDeep, Purpose: testpolicy.PurposeDiagnostic, ChangedPaths: []string{filepath.ToSlash(changed)}, GoalRisk: testpolicy.GoalRisk{Severity: int(record.Risk.Severity), Novelty: int(record.Risk.Novelty), Exposure: int(record.Risk.Exposure), Accumulation: int(record.Risk.Accumulation)}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(plan)
	if err != nil || len(plan.SelectedGroups) == 0 {
		t.Fatalf("real plan=%s err=%v", encoded, err)
	}
	for _, id := range plan.SelectedGroups {
		if !slices.ContainsFunc(contract.Groups, func(g testpolicy.Group) bool { return g.ID == id }) {
			t.Fatalf("plan names missing group %s", id)
		}
	}
	b := newReplayVerbBed(t)
	b.prepareBatch(t)
	b.owners.landing.plainProve.GoalTier = func(string, string) (uint8, error) { return record.Tier, nil }
	b.fail = func(command *exec.Cmd, _ string) (string, error) {
		if commandEnv(command, "LANDING_PROOF_SCOPE") != "full" || commandEnv(command, "LANDING_PROOF_GROUPS") != "" {
			t.Fatalf("fresh narrowed the real contract's full selection: %v", command.Env)
		}
		return "landing environment fixture\nLANDING-CHECKED\t0\n", nil
	}
	code, out := b.run(t, b.root, "prove", "--fresh", "--wait", "--json")
	var result struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(out), &result); err != nil || code != 0 || !result.Data.Fresh || result.Data.Scope != "full" {
		t.Fatalf("fresh real-contract scope: exit=%d result=%+v decode=%v\n%s", code, result, err, out)
	}
	t.Logf("live ledger %s: goal=%s tier=%d; real plan=%s", paths[0], record.Id, record.Tier, encoded)
}

func TestLandingProveFreshDoesNotJoinOrdinaryProof(t *testing.T) {
	t.Parallel()
	b := newReplayVerbBed(t)
	b.prepareBatch(t)
	launches := 0
	b.owners.landing.plainProve.Launch = func([]string, string, string) (int64, error) {
		launches++
		return int64(os.Getpid()), nil
	}
	if code, out := b.run(t, b.root, "prove", "--json"); code != 0 {
		t.Fatalf("ordinary launch exit=%d: %s", code, out)
	}
	code, out := b.run(t, b.root, "prove", "--fresh", "--json")
	if code == 0 || launches != 1 || !strings.Contains(out, `"outcome": "refused"`) {
		t.Fatalf("fresh joined an ordinary proof: exit=%d launches=%d\n%s", code, launches, out)
	}
}

func TestLandingProveFreshRetryKeepsRequest(t *testing.T) {
	t.Parallel()
	inv := &intentInvocation{input: intentInput{values: map[string][]string{"fresh": {"true"}, "gate": {"true"}}}}
	argv := proofRetryArgv(inv)
	if !slices.Contains(argv, "--fresh") || !slices.Contains(argv, "--gate") {
		t.Fatalf("retry lost the requested check: %v", argv)
	}
}
