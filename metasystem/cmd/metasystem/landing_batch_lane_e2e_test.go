package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// TestLaneChargedProofEndToEnd (U11b fix round): nothing stubbed between the
// batch and the proof store. A batch holding one change and no goal is
// sealed, planned on the lane through the real planner, and proved by the
// real tip launcher: the enrolled engine admits the proof on the lane's
// account (the proven lane owner is this process), runs the fixture suite,
// commits the terminal under the lane's lock and writes a green result; the
// batch reaches landing, the attempt's owner is the lane, and no goal's
// budget moved. It runs in its own process with its own host home, so the
// lane it registers is seen by no other test.
func TestLaneChargedProofEndToEnd(t *testing.T) {
	const selector = "METASYSTEM_LANE_CHARGED_PROOF_E2E"
	if os.Getenv(selector) == "1" {
		runLaneChargedProofEndToEnd(t)
		return
	}
	t.Parallel()
	pid := 0
	testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{Verb: "lane-charged proof end to end",
		Resolve: func() (int, bool, error) { return pid, pid != 0, nil }}})
	arguments := append([]string{"-test.run=^" + t.Name() + "$", "-test.count=1"}, inheritedTestCoverageArguments()...)
	command := exec.Command(os.Args[0], arguments...)
	command.Env = append(os.Environ(), selector+"=1", "METASYSTEM_SUPERVISION_REGISTRY_HOME="+t.TempDir())
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	pid = command.Process.Pid
	if err := command.Wait(); err != nil {
		t.Fatalf("lane-charged proof end to end: %v\n%s", err, output.String())
	}
}

func runLaneChargedProofEndToEnd(t *testing.T) {
	fixture := newPortableProofFixtureWithSetup(t, "", func(fixture *portableProofFixture) { fixture.holderLineage = batchowner.LandingOwnerLineage })
	for _, entry := range fixture.commandEnvironment()[len(os.Environ()):] {
		name, value, _ := bytesCut(entry)
		t.Setenv(name, value)
	}
	t.Setenv("METASYSTEM_OWNER_LINEAGE", batchowner.LandingOwnerLineage)
	batchowner.BatchTreePlanExecutable = func() (string, error) { return fixture.engine, nil }
	batchowner.BatchTipProofExecutable = func() (string, error) { return fixture.engine, nil }
	now := time.Now().UTC()
	home, err := board.Home()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := lane.Register(home, fixture.root, "Wido", now); err != nil {
		t.Fatal(err)
	}
	account := lane.AccountID(fixture.root)
	// This process is the lane's owner: the fixture's checkout is held under
	// the landing owner's lineage, as the batch owner holds its lane.

	base := fixture.git("rev-parse", "HEAD")
	baseTree := fixture.git("rev-parse", "HEAD^{tree}")
	fixture.write("app/a.txt", "green a from a lane change\n", 0o644)
	fixture.git("add", "app/a.txt")
	fixture.git("commit", "-qm", "record: lane change\n\nMachine: m1e+human")
	change := fixture.git("rev-parse", "HEAD")
	fixture.git("reset", "-q", "--hard", base)

	budget := func() dispatchcore.BudgetProjection {
		data, err := os.ReadFile(filepath.Join(fixture.root, "plans", "goals", "portable.md"))
		if err != nil {
			t.Fatal(err)
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 {
			t.Fatalf("goal portable: %v", problems)
		}
		control, err := canonicalProofRoot(fixture.root)
		if err != nil {
			t.Fatal(err)
		}
		return dispatchcore.ProjectBudget(control, file, time.Now().UTC())
	}
	before := budget()

	store := batch.NewStore(fixture.root, nil)
	const id = "01j5x00000000000000000ba90"
	unit := batch.NewChangeUnit(batch.ChangeMember{Commit: change, Parent: base, AskedBy: "m1e+human", Subject: "record: lane change"},
		fixture.root, "m1e", "human", []string{"app/a.txt"}, nil)
	if _, err := batch.JoinChange(store, batch.ChangeJoin{Unit: unit, BaseTree: baseTree, NewID: id, Actor: "m1e+human", At: now}); err != nil {
		t.Fatalf("join the change: %v", err)
	}
	dependencies := batchowner.ProductionBatchProofDependencies
	dependencies.Base = func(string) (string, error) { return baseTree, nil }
	dependencies.Rearm = func(string, string) error { return nil }
	if err := batchowner.ExecuteBatchProof(fixture.root, id, "landing+owner", "nothing within reach", "token-e2e", proofrun.LoadSample{}, time.Now().UTC(), dependencies); err != nil {
		t.Fatalf("prove the batch of changes: %v", err)
	}
	record, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if record.State != batch.StateLanding || record.Proof == nil || record.Proof.Status != "green" {
		t.Fatalf("the batch did not reach landing: state=%s proof=%+v history=%+v", record.State, record.Proof, record.History)
	}
	control, err := canonicalProofRoot(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := proofrun.ReadAttempts(control)
	if err != nil {
		t.Fatal(err)
	}
	var tip *proofrun.Attempt
	for index := range attempts {
		if attempts[index].AttemptID == record.Proof.AttemptID {
			tip = &attempts[index]
		}
	}
	if tip == nil || tip.GoalID != account || tip.AccountedGoal() != account || tip.Terminal == nil || tip.Terminal.Result != proofrun.TerminalSuccess {
		t.Fatalf("tip attempt=%+v (want the lane %s, committed green)", tip, account)
	}
	after := budget()
	if after.Attempts != before.Attempts || after.ReservedJobMinutes != before.ReservedJobMinutes {
		t.Fatalf("a lane-charged proof moved goal portable's budget: before=%+v after=%+v", before, after)
	}
}

func bytesCut(entry string) (string, string, bool) {
	for index := 0; index < len(entry); index++ {
		if entry[index] == '=' {
			return entry[:index], entry[index+1:], true
		}
	}
	return entry, "", false
}
