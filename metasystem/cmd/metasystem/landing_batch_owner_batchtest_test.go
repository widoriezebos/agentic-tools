//go:build batchtest

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestBatchJoinSpawnsOneOwner(t *testing.T) {
	original := batchOwnerEnsure
	t.Cleanup(func() { batchOwnerEnsure = original })
	state, pid := identity.Dead, int64(0)
	launches, wakes := 0, 0
	batchOwnerEnsure = batchOwnerEnsureSeams{
		inspect: func(string) (int64, identity.Liveness, error) { return pid, state, nil },
		wake: func(got int64) error {
			wakes++
			if got != pid {
				t.Fatalf("woke pid %d, want %d", got, pid)
			}
			return nil
		},
		launch: func(string) error {
			launches++
			pid, state = int64(40+launches), identity.Alive
			return nil
		},
	}
	root := t.TempDir()
	if err := ensureBatchOwner(root); err != nil {
		t.Fatal(err)
	}
	if err := ensureBatchOwner(root); err != nil {
		t.Fatal(err)
	}
	state = identity.Dead
	if err := ensureBatchOwner(root); err != nil {
		t.Fatal(err)
	}
	if launches != 2 || wakes != 1 {
		t.Fatalf("owner launches=%d wakes=%d, want one initial launch, one wake, one dead replacement", launches, wakes)
	}
}

func TestBatchOwnerHoldsTheLease(t *testing.T) {
	if mode := os.Getenv("GO_WANT_BATCH_OWNER_LEASE_HELPER"); mode != "" {
		held, err := acquireBatchOwner(os.Getenv("BATCH_OWNER_TEST_ROOT"))
		if err != nil {
			fmt.Println(err)
			os.Exit(23)
		}
		defer held.retire()
		holder, err := lease.CurrentHolder(os.Getenv("BATCH_OWNER_TEST_ROOT"))
		if err != nil || holder.OwnerLineage != landingOwnerLineage || holder.ClaimEpoch < 1 {
			fmt.Printf("holder=%+v err=%v\n", holder, err)
			os.Exit(24)
		}
		fmt.Println("READY")
		if mode == "hold" {
			_, _ = os.Stdin.Read(make([]byte, 1))
		}
		return
	}
	root := t.TempDir()
	if err := exec.Command("git", "init", "-q", root).Run(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBatchOwnerHoldsTheLease$")
	first.Env = append(os.Environ(), "GO_WANT_BATCH_OWNER_LEASE_HELPER=hold", "BATCH_OWNER_TEST_ROOT="+root)
	stdin, err := first.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := first.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = first.Wait() })
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "READY" {
		t.Fatalf("first owner did not acquire the lease: %q (%v)", scanner.Text(), scanner.Err())
	}
	second := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBatchOwnerHoldsTheLease$")
	second.Env = append(os.Environ(), "GO_WANT_BATCH_OWNER_LEASE_HELPER=once", "BATCH_OWNER_TEST_ROOT="+root)
	output, secondErr := second.CombinedOutput()
	if secondErr == nil || !strings.Contains(string(output), "BATCH_OWNER_OWNED_ELSEWHERE") {
		t.Fatalf("second owner err=%v output=%q", secondErr, output)
	}
}

func TestBatchOwnerWiringBound(t *testing.T) {
	if os.Getenv("GO_WANT_BATCH_OWNER_SIGNAL_HELPER") != "" {
		wake, _, cleanup := batchOwnerSignals()
		defer cleanup()
		fmt.Println("READY")
		<-wake
		fmt.Println("WOKE")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBatchOwnerWiringBound$")
	command.Env = append(os.Environ(), "GO_WANT_BATCH_OWNER_SIGNAL_HELPER=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "READY" {
		t.Fatalf("signal helper not ready: %q (%v)", scanner.Text(), scanner.Err())
	}
	if err := syscall.Kill(command.Process.Pid, syscall.SIGUSR1); err != nil {
		t.Fatal(err)
	}
	if !scanner.Scan() || scanner.Text() != "WOKE" {
		t.Fatalf("SIGUSR1 did not wake owner: %q (%v)", scanner.Text(), scanner.Err())
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestBatchProductionReturnTargetClassifiesCustody(t *testing.T) {
	original := batchReturnTargetSeams
	t.Cleanup(func() { batchReturnTargetSeams = original })
	files := []*goal.GoalFile{}
	holder := lease.CurrentHolderView{MainId: "main-a", OwnerLineage: "seat-lineage", ClaimEpoch: 9, Pid: 41}
	announcements := []lease.Announcement{{MainId: "main-a", OwnerLineage: "seat-lineage", Pid: 41, PidStartedAt: 2}}
	batchReturnTargetSeams.goals = func(string, string) ([]*goal.GoalFile, error) { return files, nil }
	batchReturnTargetSeams.holder = func(string) (lease.CurrentHolderView, error) { return holder, nil }
	batchReturnTargetSeams.announcements = func(string, string) []lease.Announcement { return announcements }
	unit := batch.Unit{GoalID: "goal-a", SeatRoot: t.TempDir(), Claim: batch.Claim{Machine: "seat", Lineage: "seat-lineage"}}
	exact := identity.Exact{Pid: 41, StartedAt: time.Unix(2, 0)}
	if got := productionReturnTarget("landing", "tree", processRefProber{exact: exact, state: identity.Alive}, unit); got.State != batch.ReturnTargetLive || got.Epoch != 9 {
		t.Fatalf("live target=%+v", got)
	}
	holder.MainId, holder.OwnerLineage = "main-b", "new-lineage"
	if got := productionReturnTarget("landing", "tree", processRefProber{exact: exact, state: identity.Alive}, unit); got.State != batch.ReturnTargetRestarted {
		t.Fatalf("restarted target=%+v", got)
	}
	holder = lease.CurrentHolderView{}
	if got := productionReturnTarget("landing", "tree", processRefProber{exact: exact, state: identity.Dead}, unit); got.State != batch.ReturnTargetDead {
		t.Fatalf("dead target=%+v", got)
	}
	files = []*goal.GoalFile{{Id: "goal-b", Claimed: &goal.ClaimRecord{Machine: "seat"}}}
	if got := productionReturnTarget("landing", "tree", processRefProber{exact: exact, state: identity.Unknown}, unit); got.State != batch.ReturnTargetOccupied || !strings.Contains(got.Reason, "goal-b") {
		t.Fatalf("occupied target=%+v", got)
	}
}

func TestBatchProofCoversTheUnion(t *testing.T) {
	const batchID = "01j5x00000000000000000ba11"
	root := t.TempDir()
	store := batch.NewStore(root, nil)
	claim := batch.Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 4, Revision: 6, AccountingRevision: 5}
	record := batch.Record{Schema: 1, BatchID: batchID, State: batch.StateOpen, BaseTree: "base", TipTree: "tip", Units: []batch.Unit{
		{GoalID: "goal-a", Chain: "chain-a", Claim: claim, State: batch.UnitJoined},
		{GoalID: "goal-b", Chain: "chain-b", Claim: claim, State: batch.UnitJoined},
	}}
	record.SelectedGroups = []string{"common", "member-only-crosscut"}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	events := []string{}
	deps := batchProofDependencies{
		rearm: func(_ string, tree string) error {
			events = append(events, "rearm:"+tree)
			return nil
		},
		plan: func(_, _, _ string, mode testpolicy.Mode) (testpolicy.Plan, error) {
			events = append(events, "plan:"+string(mode))
			if mode == testpolicy.ModeAuto {
				return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard, SelectedGroups: []string{"common"}}, nil
			}
			return testpolicy.Plan{RequiredMode: testpolicy.ModeDeep, ExecutedMode: testpolicy.ModeDeep,
				SelectedGroups: []string{"common", "member-only-crosscut"}}, nil
		},
		launch: func(request batchProofLaunch) (proofrun.TestResult, error) {
			events = append(events, "launch:"+string(request.Mode))
			admitted, err := store.Load(batchID)
			if err != nil || admitted.State != batch.StateProving || admitted.Proof == nil || admitted.Proof.Status != "planned" {
				t.Fatalf("runner charged without durable admission: record=%+v err=%v", admitted, err)
			}
			return proofrun.TestResult{AttemptID: "attempt-tip", Delivery: proofrun.DeliveryJudgment{Sufficient: true}, Groups: []proofrun.GroupResult{
				{ID: "common", NativeLaunched: true}, {ID: "member-only-crosscut", ReuseAttempt: "attempt-member"},
			}}, nil
		},
	}
	sample := proofrun.LoadSample{OverlappingHost: 2, OverlapKnown: true}
	if err := executeBatchProof(root, batchID, "landing-owner", "full", sample, time.Unix(8, 0), deps); err != nil {
		t.Fatal(err)
	}
	finished, err := store.Load(batchID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(events, ",") != "rearm:base,plan:auto,plan:deep,launch:deep" || finished.State != batch.StateLanding ||
		finished.Proof == nil || finished.Proof.AttemptID != "attempt-tip" || finished.Proof.Launchers != 2 ||
		!slices.Equal(finished.Proof.Executions, []string{"common"}) || finished.Proof.Reuse["member-only-crosscut"] != "attempt-member" {
		t.Fatalf("events=%v proof=%+v state=%s", events, finished.Proof, finished.State)
	}
}

func TestBatchProofAcceptsReusableSuccess(t *testing.T) {
	if !batchProofExitAccepted(proofrun.ExitReusableSuccess, proofrun.TestResult{Delivery: proofrun.DeliveryJudgment{Sufficient: true}}) {
		t.Fatal("reusable sufficient proof was classified red")
	}
	if batchProofExitAccepted(proofrun.ExitReusableSuccess, proofrun.TestResult{}) || batchProofExitAccepted(1, proofrun.TestResult{Delivery: proofrun.DeliveryJudgment{Sufficient: true}}) {
		t.Fatal("insufficient reuse or an ordinary failure was classified green")
	}
}

func TestBatchSupervisorTakeoverRebindsJoinedClaims(t *testing.T) {
	const batchID = "01j5x00000000000000000ba01"
	root := t.TempDir()
	store := batch.NewStore(root, nil)
	claim := batch.Claim{Machine: "landing", Lineage: landingOwnerLineage, Epoch: 1, Revision: 3, AccountingRevision: 2}
	if err := store.Create(batch.Record{Schema: 1, BatchID: batchID, State: batch.StateOpen, Units: []batch.Unit{
		{GoalID: "goal-a", Chain: "chain-a", Claim: claim, State: batch.UnitJoined},
		{GoalID: "goal-b", Chain: "chain-b", Claim: claim, State: batch.UnitJoining},
	}}); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	run := func(gotRoot string, args ...string) error {
		if gotRoot != root {
			t.Fatalf("rebind root=%q, want %q", gotRoot, root)
		}
		calls = append(calls, append([]string(nil), args...))
		return nil
	}
	if err := rebindBatchClaims(root, batchID, "landing-machine", 2, run); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("rebind calls=%v", calls)
	}
	joined := strings.Join(calls[0], " ")
	if !strings.Contains(joined, "--id goal-a") || strings.Contains(joined, "goal-b") ||
		!strings.Contains(joined, "--target-claim-epoch 2") || !strings.Contains(joined, "--target-lineage "+landingOwnerLineage) {
		t.Fatalf("rebind calls=%v", calls)
	}
}

func TestGoalHandoverTargetRootFlagFlows(t *testing.T) {
	flags := flag.NewFlagSet("goal handover", flag.ContinueOnError)
	target := pathFlag(flags, "target-root", "", "return target checkout")
	want := filepath.Join(t.TempDir(), "seat")
	if err := flags.Parse([]string{"--target-root", want}); err != nil {
		t.Fatal(err)
	}
	request := goal.VerbRequest{}
	bindHandoverTargetRoot(&request, *target)
	if request.HandoverTargetRoot != want {
		t.Fatalf("handover request target root=%q, want %q", request.HandoverTargetRoot, want)
	}
}

func TestLandingBatchJoinVerbPublishesOutsideFlock(t *testing.T) {
	originalDependencies, originalClock := batchJoinDependenciesForCommand, batchJoinClock
	t.Cleanup(func() { batchJoinDependenciesForCommand, batchJoinClock = originalDependencies, originalClock })
	now := time.Unix(30, 0).UTC()
	batchJoinClock = func(string) (time.Time, error) { return now, nil }
	seat, landing := t.TempDir(), t.TempDir()
	for _, root := range []string{seat, landing} {
		if err := exec.Command("git", "init", "-q", root).Run(); err != nil {
			t.Fatal(err)
		}
		if err := exec.Command("git", "-C", root, "config", "user.email", "fixture@example.com").Run(); err != nil {
			t.Fatal(err)
		}
		if err := exec.Command("git", "-C", root, "config", "user.name", "Fixture").Run(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "base.txt"), []byte("base\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := exec.Command("git", "-C", root, "add", "base.txt").Run(); err != nil {
			t.Fatal(err)
		}
		if err := exec.Command("git", "-C", root, "commit", "-qm", "base").Run(); err != nil {
			t.Fatal(err)
		}
	}
	conf := config.BatchRootKey + "=" + landing + "\n" + config.BatchMaxWaitKey + "=1m\n"
	if err := os.WriteFile(filepath.Join(seat, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(mustOutput(t, exec.Command("git", "-C", landing, "rev-parse", "HEAD^{tree}"))))
	const batchID = "01j5x00000000000000000ba01"
	store := batch.NewStore(landing, nil)
	if err := store.Create(batch.Record{Schema: 1, BatchID: batchID, BaseTree: base, TipTree: base, State: batch.StateOpen}); err != nil {
		t.Fatal(err)
	}
	patch := []byte("diff --git a/joined.txt b/joined.txt\nnew file mode 100644\nindex 0000000..257cc56\n--- /dev/null\n+++ b/joined.txt\n@@ -0,0 +1 @@\n+joined\n")
	lockPath := filepath.Join(landing, "artifacts", "agents", "locks", "landing-batches.lock")
	flockFree := func(stage string) {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			t.Fatalf("batch flock held during %s: %v", stage, err)
		}
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
	}
	events := []string{}
	dependencies := productionBatchJoinDependencies()
	dependencies.binding = func(string, string, time.Time) (dispatchcore.GoalBinding, error) {
		file := &goal.GoalFile{Claimed: &goal.ClaimRecord{AccountingRevision: 4}}
		return dispatchcore.GoalBinding{Revision: 5, Machine: "seat-machine", Lineage: "seat-lineage", File: file,
			Capability: goal.StopCapability{ClaimEpoch: 7}}, nil
	}
	dependencies.chain = func(string, string, string, uint64) (batch.CertifiedChain, error) {
		return batch.CertifiedChain{ID: "chain-a", Patch: patch}, nil
	}
	dependencies.base = func(string) (string, error) { return base, nil }
	dependencies.mint = func() (string, error) { return "01j5x00000000000000000ba99", nil }
	dependencies.transport = func(root string, chain batch.CertifiedChain) error {
		flockFree("transport")
		events = append(events, "transport")
		return batch.TransportChain(root, chain)
	}
	dependencies.gate = func(_ string, tree string, gotPatch, fixtures []byte, unit *batch.Unit) error {
		flockFree("gate")
		events = append(events, "gate")
		return batch.RunJoinGate(tree, gotPatch, fixtures, unit, func(string, batch.GateStep) batch.GateStepResult {
			return batch.GateStepResult{RunID: "join-gate"}
		})
	}
	dependencies.fixtures = func(string) ([]byte, error) { return nil, nil }
	dependencies.plan = func(string, string, string) (testpolicy.Plan, error) {
		return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard}, nil
	}
	dependencies.handover = func(request batchJoinRequest, gotBatch string, source batch.Claim) error {
		events = append(events, "forward-handover")
		if request.SeatRoot != seat || gotBatch != batchID || source.Lineage != "seat-lineage" {
			t.Fatalf("forward handover request=%+v batch=%s source=%+v", request, gotBatch, source)
		}
		file, err := os.OpenFile(lockPath, os.O_RDWR, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
			t.Fatal("forward handover ran outside PublishJoin's flock")
		}
		return nil
	}
	dependencies.ensure = func(root string) error {
		flockFree("ensureOwner")
		events = append(events, "ensure-owner")
		record, err := batch.NewStore(root, nil).Load(batchID)
		if err != nil || len(record.Units) != 1 || record.Units[0].State != batch.UnitJoined {
			t.Fatalf("ensureOwner preceded joined publication: record=%+v err=%v", record, err)
		}
		return nil
	}
	batchJoinDependenciesForCommand = func() batchJoinDependencies { return dependencies }
	if code := runBatchJoin([]string{"--root", seat, "--goal", "goal-a", "--chain", "chain-a"}); code != 0 {
		t.Fatalf("join exited %d", code)
	}
	record, err := store.Load(batchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Units) != 1 || record.Units[0].SeatRoot != seat || record.Units[0].State != batch.UnitJoined ||
		strings.Join(events, ",") != "transport,gate,forward-handover,ensure-owner" {
		t.Fatalf("joined record=%+v events=%v", record, events)
	}
}

func mustOutput(t *testing.T, command *exec.Cmd) []byte {
	t.Helper()
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return output
}
