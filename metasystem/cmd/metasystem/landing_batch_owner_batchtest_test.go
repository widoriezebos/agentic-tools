//go:build batchtest

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestBatchGoalHandoverChild(t *testing.T) {
	raw := os.Getenv("GO_WANT_BATCH_GOAL_HANDOVER_CHILD")
	if raw == "" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		fmt.Println(err)
		os.Exit(25)
	}
	os.Exit(runGoalHandoverMutation(args))
}

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

func TestBatchOwnerInspectionUsesInjectedProberAndKeepsReadErrorsUnknown(t *testing.T) {
	holder := lease.CurrentHolderView{MainId: "main-a", OwnerLineage: landingOwnerLineage, ClaimEpoch: 1, Pid: 41}
	announcements := func(string, int64) []lease.Announcement {
		return []lease.Announcement{{MainId: "main-a", Pid: 41, PidStartedAt: 2}}
	}
	read := func(string) (lease.CurrentHolderView, error) { return holder, nil }
	exact := identity.Exact{Pid: 41, StartedAt: time.Unix(2, 0)}
	pid, state, err := inspectBatchOwnerWith("root", processRefProber{exact: exact, state: identity.Alive}, read, announcements)
	if err != nil || pid != 41 || state != identity.Alive {
		t.Fatalf("alive inspection pid=%d state=%s error=%v", pid, state, err)
	}
	_, state, err = inspectBatchOwnerWith("root", processRefProber{exact: exact, state: identity.Dead}, read, announcements)
	if err != nil || state != identity.Dead {
		t.Fatalf("dead inspection state=%s error=%v", state, err)
	}
	readErr := errors.New("transient lease read")
	_, state, err = inspectBatchOwnerWith("root", processRefProber{}, func(string) (lease.CurrentHolderView, error) {
		return lease.CurrentHolderView{}, readErr
	}, announcements)
	if !errors.Is(err, readErr) || state != identity.Unknown {
		t.Fatalf("read failure state=%s error=%v", state, err)
	}
}

func TestBatchOwnerHoldsTheLease(t *testing.T) {
	if mode := os.Getenv("GO_WANT_BATCH_OWNER_LEASE_HELPER"); mode != "" {
		root := os.Getenv("BATCH_OWNER_TEST_ROOT")
		if mode == "hold-foreign" {
			exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
			if err != nil || state != identity.Alive {
				fmt.Printf("predecessor identity state=%s err=%v\n", state, err)
				os.Exit(22)
			}
			if _, err := lease.AnnounceWithPair(root, fmt.Sprintf("landing-owner-predecessor-%d", os.Getpid()), int64(os.Getpid()),
				exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "landing-owner", "metasystem", "retired-landing-lineage"); err != nil {
				fmt.Println(err)
				os.Exit(23)
			}
			holder, err := lease.CurrentHolder(root)
			if err != nil || holder.ClaimEpoch != 1 {
				fmt.Printf("predecessor holder=%+v err=%v\n", holder, err)
				os.Exit(24)
			}
			fmt.Println("READY")
			_, _ = os.Stdin.Read(make([]byte, 1))
			return
		}
		held, err := acquireBatchOwner(root)
		if err != nil {
			fmt.Println(err)
			os.Exit(23)
		}
		if mode != "hold-dead" {
			defer held.retire()
		}
		holder, err := lease.RequireHolder(root, int64(os.Getpid()), nil)
		if err != nil || !holder.Holder || holder.ClaimEpoch == nil || *holder.ClaimEpoch < 1 {
			fmt.Printf("holder=%+v err=%v\n", holder, err)
			os.Exit(24)
		}
		fmt.Println("READY")
		if mode == "hold" || mode == "hold-dead" {
			_, _ = os.Stdin.Read(make([]byte, 1))
		}
		return
	}
	root := t.TempDir()
	if err := exec.Command("git", "init", "-q", root).Run(); err != nil {
		t.Fatal(err)
	}
	first := exec.Command(os.Args[0], "-test.run=^TestBatchOwnerHoldsTheLease$")
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
	second := exec.Command(os.Args[0], "-test.run=^TestBatchOwnerHoldsTheLease$")
	second.Env = append(os.Environ(), "GO_WANT_BATCH_OWNER_LEASE_HELPER=once", "BATCH_OWNER_TEST_ROOT="+root)
	output, secondErr := second.CombinedOutput()
	if secondErr == nil || !strings.Contains(string(output), "BATCH_OWNER_OWNED_ELSEWHERE") {
		t.Fatalf("second owner err=%v output=%q", secondErr, output)
	}
}

func TestBatchOwnerWiringBound(t *testing.T) {
	if os.Getenv("GO_WANT_BATCH_OWNER_SIGNAL_HELPER") != "" {
		root := os.Getenv("BATCH_OWNER_TEST_ROOT")
		batchOwnerAcquire = func(string) (batchOwnerLease, error) {
			return batchOwnerLease{root: root, pid: int64(os.Getpid()), epoch: 1}, nil
		}
		batchOwnerConstruct = func(settings config.BatchLanding, held batchOwnerLease, now func() time.Time) (*batch.Owner, error) {
			calls := 0
			return batch.NewOwner(batch.OwnerOptions{
				Store: batch.NewStore(root, nil), Settings: settings, Actor: "fixture", PID: held.pid, Now: now,
				FetchTree: func() (string, error) { return "tree", nil },
				ReadClaim: func(string, string, string, string) (batch.Claim, error) { return batch.Claim{}, nil },
				Rebind:    func(string, string) error { return nil }, Mint: func() (string, error) { return "opid", nil },
				LogRed: func(string, batch.TrunkRedRecordOutcome) {}, BaseCommit: func(string) (string, error) { return "commit", nil },
				RunDiagnostic: func(string, batch.DiagnosticRequest, batch.Claim) (batch.DiagnosticResult, error) {
					return batch.DiagnosticResult{}, nil
				},
				DescendsFrom: func(string, string) (bool, error) { return false, nil }, Sample: func() proofrun.LoadSample { return proofrun.LoadSample{} },
				Admission: func(proofrun.LoadSample) proofrun.AdmissionCap { return proofrun.AdmissionCap{} },
				Launch:    func(string, proofrun.LoadSample, string) error { return nil },
				After:     func(time.Duration) <-chan time.Time { return make(chan time.Time) },
				Report:    func(string, error) {}, Glob: func(string) ([]string, error) {
					calls++
					if calls == 1 {
						fmt.Println("READY")
					} else {
						fmt.Println("TICK")
					}
					return nil, nil
				},
			})
		}
		seat := os.Getenv("BATCH_OWNER_TEST_SEAT")
		os.Exit(runBatchOwner([]string{"--root", seat, "--landing-root", root, "--max-wait", "1m", "--interval", "1m"}))
	}
	root, seat := t.TempDir(), t.TempDir()
	for _, repo := range []string{root, seat} {
		if err := exec.Command("git", "init", "-q", repo).Run(); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(os.Args[0], "-test.run=^TestBatchOwnerWiringBound$")
	command.Env = append(os.Environ(), "GO_WANT_BATCH_OWNER_SIGNAL_HELPER=1", "BATCH_OWNER_TEST_ROOT="+root,
		"BATCH_OWNER_TEST_SEAT="+seat, "METASYSTEM_GOAL_NOW=2026-09-17T10:00:00Z")
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
	// Wait for the fact (the second Resume's TICK) on the package's wiring bound, never on an
	// instant or a count of steps a loaded machine can outrun.
	lines := make(chan string)
	go func() {
		defer close(lines)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	bound := time.NewTimer(wiringBound)
	defer bound.Stop()
	for observed := ""; observed != "TICK"; {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("SIGUSR1 wiring ended before the owner loop ticked: %v", scanner.Err())
			}
			observed = line
		case <-bound.C:
			_ = command.Process.Kill()
			_ = command.Wait()
			t.Fatalf("SIGUSR1 did not reach the owner loop within %s", wiringBound)
		}
	}
	if err := syscall.Kill(command.Process.Pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
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
		seal: func(_, _, _, _ string, _ time.Time) error {
			events = append(events, "seal")
			return store.Update(batchID, func(record *batch.Record) error {
				record.Seal = map[string]batch.Claim{"goal-a": claim, "goal-b": claim}
				record.Transition(batch.StateSealed, time.Unix(7, 0), "seal", "landing-owner", "")
				return nil
			})
		},
		plan: func(_, goalID, _ string, mode testpolicy.Mode) (testpolicy.Plan, error) {
			events = append(events, "plan:"+goalID+":"+string(mode))
			groups := []string{"common"}
			if goalID == "goal-a" {
				groups = append(groups, "member-only-crosscut")
			}
			return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard, SelectedGroups: groups}, nil
		},
		launch: func(request batchProofLaunch) (proofrun.TestResult, error) {
			events = append(events, "launch:"+string(request.Mode))
			admitted, err := store.Load(batchID)
			if err != nil || admitted.State != batch.StateProving || admitted.Proof == nil || admitted.Proof.Status != "planned" {
				t.Fatalf("runner charged without durable admission: record=%+v err=%v", admitted, err)
			}
			return proofrun.TestResult{AttemptID: "attempt-tip", LaunchCounts: proofrun.LaunchCounts{Test: 1, Build: 1}, Delivery: proofrun.DeliveryJudgment{Sufficient: true}, Groups: []proofrun.GroupResult{
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
	if strings.Join(events, ",") != "rearm:base,seal,plan:goal-a:auto,plan:goal-b:auto,launch:standard" || finished.State != batch.StateLanding ||
		finished.Proof == nil || finished.Proof.AttemptID != "attempt-tip" || finished.Proof.Launchers != 2 ||
		!slices.Equal(finished.Proof.Executions, []string{"common"}) || finished.Proof.Reuse["member-only-crosscut"] != "attempt-member" {
		t.Fatalf("events=%v proof=%+v state=%s", events, finished.Proof, finished.State)
	}
}

func batchProofRefusalBed(t *testing.T) (string, string, batch.Store, batch.Record) {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main", root}, {"-C", root, "config", "user.name", "Fixture"}, {"-C", root, "config", "user.email", "fixture@example.com"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("base-b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", root, "add", "."}, {"-C", root, "commit", "-qm", "base"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	base := strings.TrimSpace(string(mustOutput(t, exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}"))))
	for _, item := range []struct{ chain, file, body string }{{"chain-a", "a.txt", "unit-a\n"}, {"chain-b", "b.txt", "unit-b\n"}} {
		if err := os.WriteFile(filepath.Join(root, item.file), []byte(item.body), 0o644); err != nil {
			t.Fatal(err)
		}
		patch := mustOutput(t, exec.Command("git", "-C", root, "diff", "--binary", "HEAD", "--", item.file))
		dir := filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", item.chain)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "diff.patch"), patch, 0o644); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command("git", "-C", root, "checkout", "--", item.file).CombinedOutput(); err != nil {
			t.Fatalf("restore %s: %v %s", item.file, err, output)
		}
	}
	claimA := batch.Claim{Machine: "seat", Lineage: "a", Epoch: 1, Revision: 7, AccountingRevision: 5}
	claimB := batch.Claim{Machine: "seat", Lineage: "b", Epoch: 1, Revision: 8, AccountingRevision: 6}
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba12", State: batch.StateSealed, BaseTree: base,
		SelectedGroups: []string{"required"}, Seal: map[string]batch.Claim{"goal-a": claimA, "goal-b": claimB}, Units: []batch.Unit{
			{GoalID: "goal-a", Chain: "chain-a", Claim: claimA, State: batch.UnitJoined},
			{GoalID: "goal-b", Chain: "chain-b", Claim: claimB, State: batch.UnitJoined},
		}}
	prefixes, err := batch.AssembleUnits(root, base, record.Units)
	if err != nil {
		t.Fatal(err)
	}
	record.PrefixTrees, record.TipTree = prefixes, prefixes[len(prefixes)-1]
	store := batch.NewStore(root, nil)
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	return root, record.BatchID, store, record
}

func TestBatchProofRefusalTransitions(t *testing.T) {
	plan := func(groups ...string) func(string, string, string, testpolicy.Mode) (testpolicy.Plan, error) {
		return func(string, string, string, testpolicy.Mode) (testpolicy.Plan, error) {
			return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard, SelectedGroups: groups}, nil
		}
	}
	reseal := func(store batch.Store, claims map[string]batch.Claim) func(string, string, string, string, time.Time) error {
		return func(_, id, actor, _ string, at time.Time) error {
			return store.Update(id, func(record *batch.Record) error {
				record.Seal = claims
				record.SelectedGroups = []string{"required"}
				record.Transition(batch.StateSealed, at, "seal", actor, "")
				return nil
			})
		}
	}
	t.Run("union uncovered is durable and never launches", func(t *testing.T) {
		root, id, store, _ := batchProofRefusalBed(t)
		launches := 0
		err := executeBatchProof(root, id, "owner", "full", proofrun.LoadSample{}, time.Unix(10, 0), batchProofDependencies{
			rearm: func(string, string) error { return nil }, plan: plan("other"), launch: func(batchProofLaunch) (proofrun.TestResult, error) {
				launches++
				return proofrun.TestResult{}, nil
			},
		})
		record, loadErr := store.Load(id)
		refusals := 0
		for _, event := range record.History {
			if event.Verb == "prove-refused" {
				refusals++
			}
		}
		if err == nil || !strings.Contains(err.Error(), "BATCH_PROOF_UNION_UNCOVERED") || loadErr != nil || launches != 0 ||
			record.Proof == nil || record.Proof.Status != "union-uncovered" || refusals != 1 {
			t.Fatalf("error=%v load=%v launches=%d refusals=%d record=%+v", err, loadErr, launches, refusals, record)
		}
	})
	t.Run("capacity refusal returns to sealed without diagnosis", func(t *testing.T) {
		root, id, store, _ := batchProofRefusalBed(t)
		err := executeBatchProof(root, id, "owner", "full", proofrun.LoadSample{}, time.Unix(10, 0), batchProofDependencies{
			rearm: func(string, string) error { return nil }, plan: plan("required"), launch: func(batchProofLaunch) (proofrun.TestResult, error) {
				return proofrun.TestResult{}, &batchProofAdmissionRefusal{kind: "capacity", reason: "ADMISSION_REFUSED capacity"}
			},
		})
		record, loadErr := store.Load(id)
		if err != nil || loadErr != nil || record.State != batch.StateSealed || record.Proof == nil || record.Proof.Status != "admission-refused" {
			t.Fatalf("error=%v load=%v record=%+v", err, loadErr, record)
		}
	})
	t.Run("revision refusal returns head and excludes its patch", func(t *testing.T) {
		root, id, store, original := batchProofRefusalBed(t)
		err := executeBatchProof(root, id, "owner", "full", proofrun.LoadSample{}, time.Unix(10, 0), batchProofDependencies{
			rearm: func(string, string) error { return nil }, plan: plan("required"), launch: func(batchProofLaunch) (proofrun.TestResult, error) {
				return proofrun.TestResult{}, &batchProofAdmissionRefusal{kind: "revision", reason: "GOAL_REVISION_MOVED"}
			},
		})
		record, loadErr := store.Load(id)
		bAtTip, showErr := exec.Command("git", "-C", root, "show", record.TipTree+":b.txt").Output()
		if err != nil || loadErr != nil || showErr != nil || record.Units[1].State != batch.UnitReturnPending || record.State != batch.StateOpen ||
			record.TipTree == original.TipTree || strings.TrimSpace(string(bAtTip)) != "base-b" {
			t.Fatalf("error=%v load=%v show=%v b=%q record=%+v", err, loadErr, showErr, bAtTip, record)
		}
	})
	t.Run("budget refusal withdraws head and next launch uses survivor tip", func(t *testing.T) {
		root, id, store, original := batchProofRefusalBed(t)
		var launches []batchProofLaunch
		deps := batchProofDependencies{rearm: func(string, string) error { return nil }, plan: plan("required"), launch: func(request batchProofLaunch) (proofrun.TestResult, error) {
			launches = append(launches, request)
			if len(launches) == 1 {
				return proofrun.TestResult{}, &batchProofAdmissionRefusal{kind: "budget", reason: "BATCH_MEMBER_BUDGET_REFUSED"}
			}
			return proofrun.TestResult{AttemptID: "survivor", Delivery: proofrun.DeliveryJudgment{Sufficient: true}}, nil
		}}
		if err := executeBatchProof(root, id, "owner", "full", proofrun.LoadSample{}, time.Unix(10, 0), deps); err != nil {
			t.Fatal(err)
		}
		withdrawn, err := store.Load(id)
		if err != nil || withdrawn.Units[1].State != batch.UnitReturnPending || withdrawn.Units[1].Outcome != batch.UnitWithdrawnBudget ||
			withdrawn.TipTree == original.TipTree || len(launches) != 1 {
			t.Fatalf("withdrawn=%+v launches=%+v err=%v", withdrawn, launches, err)
		}
		deps.seal = reseal(store, map[string]batch.Claim{"goal-a": withdrawn.Units[0].Claim})
		if err := executeBatchProof(root, id, "owner", "full", proofrun.LoadSample{}, time.Unix(11, 0), deps); err != nil {
			t.Fatal(err)
		}
		if len(launches) != 2 || launches[1].GoalID != "goal-a" || launches[1].Tree != withdrawn.TipTree || launches[1].Tree == original.TipTree {
			t.Fatalf("launches=%+v withdrawn=%+v", launches, withdrawn)
		}
	})
}

func TestBatchProofAcceptsReusableSuccess(t *testing.T) {
	if !batchProofExitAccepted(proofrun.ExitReusableSuccess, proofrun.TestResult{Delivery: proofrun.DeliveryJudgment{Sufficient: true}}) {
		t.Fatal("reusable sufficient proof was classified red")
	}
	if batchProofExitAccepted(proofrun.ExitReusableSuccess, proofrun.TestResult{}) || batchProofExitAccepted(1, proofrun.TestResult{Delivery: proofrun.DeliveryJudgment{Sufficient: true}}) {
		t.Fatal("insufficient reuse or an ordinary failure was classified green")
	}
}

func TestBatchProofUnionUsesRealMemberRiskSelection(t *testing.T) {
	contract := testpolicy.Contract{
		Surfaces: []testpolicy.Surface{{ID: "app", Paths: []string{"pkg/**"}, Standard: []string{"base"}, CrossCutting: []string{"cross"}}},
		Groups:   []testpolicy.Group{{ID: "base"}, {ID: "cross", Obligations: []string{"cross"}}},
	}
	units := []batch.Unit{{GoalID: "risk-a"}, {GoalID: "tip-b"}}
	plan, err := planBatchMemberUnion("root", "tree", units, testpolicy.ModeAuto, func(_, goalID, _ string, mode testpolicy.Mode) (testpolicy.Plan, error) {
		risk := testpolicy.GoalRisk{}
		if goalID == "risk-a" {
			risk.Accumulation = 2
		}
		return testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: []string{"pkg/value.go"}, GoalRisk: risk, RequestedMode: mode, Purpose: testpolicy.PurposeDelivery})
	})
	if err != nil || !slices.Contains(plan.SelectedGroups, "cross") {
		t.Fatalf("member-risk union plan=%+v error=%v", plan, err)
	}
}

func TestBatchProofRearmsBaseBeforePlanningEvenWhenTreeMatches(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main", root}, {"-C", root, "config", "user.name", "Fixture"}, {"-C", root, "config", "user.email", "fixture@example.com"}} {
		if err := exec.Command("git", args...).Run(); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "value"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mustRun("add", "value")
	mustRun("commit", "-qm", "old")
	old := strings.TrimSpace(string(mustOutput(t, exec.Command("git", "-C", root, "rev-parse", "HEAD"))))
	if err := os.WriteFile(filepath.Join(root, "value"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun("add", "value")
	mustRun("commit", "-qm", "base")
	baseTree := strings.TrimSpace(string(mustOutput(t, exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}"))))
	mustRun("update-ref", "refs/remotes/origin/main", "HEAD")
	mustRun("checkout", "-q", "--detach", old)
	original := batchBaseRearm
	t.Cleanup(func() { batchBaseRearm = original })
	var events []string
	batchBaseRearm.fastForward = func(context.Context, string, string) error { events = append(events, "fast-forward"); return nil }
	batchBaseRearm.rebuild = func(context.Context, string) error { events = append(events, "rebuild"); return nil }
	batchBaseRearm.up = func(context.Context, string, string) (upOutcome, error) {
		events = append(events, "up")
		return upOutcome{}, nil
	}
	if err := rearmBatchBase(root, baseTree); err != nil || strings.Join(events, ",") != "fast-forward,rebuild,up" {
		t.Fatalf("moved rearm events=%v error=%v", events, err)
	}
	events = nil
	mustRun("checkout", "-q", "refs/remotes/origin/main")
	if err := rearmBatchBase(root, baseTree); err != nil || strings.Join(events, ",") != "rebuild,up" {
		t.Fatalf("equal-tree rearm events=%v error=%v", events, err)
	}
}

func TestBatchSupervisorTakeoverRebindsJoinedClaims(t *testing.T) {
	const batchID = "01j5x00000000000000000ba01"
	if !slices.Contains(supervise.ProductionComponents(), supervise.LandingOwner) {
		t.Fatal("production supervisor arming omitted the landing owner")
	}
	root := syncedClaimedGoalFixture(t)
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "landing-machine")
	amendSyncedGoalFixture(t, root, "hand standing validation to the landing owner", func(file *goal.GoalFile) {
		file.Claimed.Machine, file.Claimed.Lineage = "landing-machine", landingOwnerLineage
		file.Claimed.HandedOver = goal.HandedOver{FromMachine: "seat-machine", FromLineage: "seat-lineage", FromEpoch: 1, Batch: batchID}
		file.StopCapability = &goal.StopCapability{Generation: 1, Revision: file.Claimed.Revision, Machine: "landing-machine", ClaimEpoch: 1}
	})
	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe owner caller: state=%s err=%v", state, err)
	}
	first := exec.Command(os.Args[0], "-test.run=^TestBatchOwnerHoldsTheLease$")
	first.Env = append(os.Environ(), "GO_WANT_BATCH_OWNER_LEASE_HELPER=hold-foreign", "BATCH_OWNER_TEST_ROOT="+root)
	firstInput, err := first.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	firstOutput, err := first.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	firstReleased := false
	t.Cleanup(func() {
		if !firstReleased {
			_ = firstInput.Close()
			_ = first.Wait()
		}
	})
	firstScanner := bufio.NewScanner(firstOutput)
	if !firstScanner.Scan() || firstScanner.Text() != "READY" {
		t.Fatalf("first batch owner did not acquire the lease: %q (%v)", firstScanner.Text(), firstScanner.Err())
	}
	t.Setenv("METASYSTEM_OWNER_LINEAGE", landingOwnerLineage)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-30T08:07:00Z")
	store := batch.NewStore(root, nil)
	claim := batch.Claim{Machine: "landing", Lineage: landingOwnerLineage, Epoch: 1, Revision: 3, AccountingRevision: 2}
	if err := store.Create(batch.Record{Schema: 1, BatchID: batchID, State: batch.StateOpen, Units: []batch.Unit{
		{GoalID: "standing-validation", Chain: "chain-a", Claim: claim, State: batch.UnitJoined},
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
		if len(args) < 2 || args[0] != "goal" || args[1] != "handover" {
			return fmt.Errorf("unexpected rebind command %v", args)
		}
		childArgs, err := json.Marshal(args[2:])
		if err != nil {
			return err
		}
		command := exec.Command(os.Args[0], "-test.run=^TestBatchGoalHandoverChild$")
		command.Env = append(os.Environ(), "GO_WANT_BATCH_GOAL_HANDOVER_CHILD="+string(childArgs))
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("real goal handover: %w: %s", err, output)
		}
		return nil
	}
	acceptedTree := func() string {
		return strings.TrimSpace(goalSyncMutationGit(t, root, "rev-parse", goal.AcceptedRef+"^{tree}"))
	}
	if err := rebindBatchClaims(root, batchID, acceptedTree(), "landing-machine", 1, batch.ReadReturnLedgerGoal, run); err != nil || len(calls) != 0 {
		t.Fatalf("equal-epoch tick rebind calls=%v error=%v", calls, err)
	}
	if err := firstInput.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Wait(); err != nil {
		t.Fatal(err)
	}
	firstReleased = true
	replacement, err := acquireBatchOwner(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(replacement.retire)
	if replacement.epoch != 2 {
		t.Fatalf("replacement owner epoch=%d, want real lease takeover epoch 2", replacement.epoch)
	}
	if err := rebindBatchClaims(root, batchID, acceptedTree(), "landing-machine", replacement.epoch, batch.ReadReturnLedgerGoal, run); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("rebind calls=%v", calls)
	}
	joined := strings.Join(calls[0], " ")
	if !strings.Contains(joined, "--id standing-validation") || strings.Contains(joined, "goal-b") ||
		!strings.Contains(joined, "--target-claim-epoch 2") || !strings.Contains(joined, "--target-lineage "+landingOwnerLineage) {
		t.Fatalf("rebind calls=%v", calls)
	}
	ledger, err := batch.ReadReturnLedgerGoal(root, acceptedTree(), "standing-validation")
	if err != nil || ledger.ClaimEpoch != 2 {
		t.Fatalf("real rebound ledger=%+v error=%v", ledger, err)
	}
	if err := rebindBatchClaims(root, batchID, acceptedTree(), "landing-machine", 2, batch.ReadReturnLedgerGoal, run); err != nil || len(calls) != 1 {
		t.Fatalf("settled takeover rebind calls=%v error=%v", calls, err)
	}
	ref := parent.Ref()
	job := map[string]any{
		"jobId": "stale-batch-proof", "operationId": "stale-batch-proof", "goalId": "standing-validation", "goalRevision": 2,
		"machineId": "landing-machine", "claimEpoch": 1, "capMin": 1, "status": "running",
		"pid": parent.Pid, "pidStartedAt": ref.StartedAtSec,
	}
	if ref.StartedAtUnixMicro > 0 {
		job["pidStartedAtExactMicro"] = ref.StartedAtUnixMicro
	}
	if ref.StartTicks > 0 {
		job["pidStartTicks"], job["bootId"] = ref.StartTicks, ref.BootID
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, jobs, "stale-batch-proof.json", job)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_STATE_ROOT", root)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT", root)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_JOB", "stale-batch-proof")
	attempt, _, _, err := admitProofLaunch(proofLaunchAdmission{ControlRoot: root, ExecutionRoot: root,
		ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: "standing-validation", CapMin: "1", ScopeClass: "full", CommandClass: "testing"})
	if err == nil || !strings.Contains(err.Error(), "custody changed before reservation") || attempt.AttemptID != "" {
		t.Fatalf("stale takeover proof attempt=%+v error=%v", attempt, err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		path := filepath.Join(root, "scripts", "agents", name)
		if err := os.WriteFile(path, []byte(`{"floors":{"cmd/metasystem":1},"exempt":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	proofIdentity, err := proofrun.BuildProofIdentity(canonicalRoot, filepath.Join(canonicalRoot, "metasystem.conf"), "full", "testing", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	parentIdentity := proofrun.ProcessIdentity{Pid: parent.Pid, PidStartedAt: ref.StartedAtSec, PidStartedAtMicro: ref.StartedAtUnixMicro, PidStartTicks: ref.StartTicks, BootID: ref.BootID}
	parentAttempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{ControlRoot: canonicalRoot, ExecutionRoot: canonicalRoot, GoalID: "standing-validation",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 1, Identity: proofIdentity, Launcher: parentIdentity, Now: time.Unix(2, 0)})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_HOOK_DELEGATE_JOB", "")
	t.Setenv("METASYSTEM_PROOF_CONTROL_ROOT", canonicalRoot)
	t.Setenv("METASYSTEM_PROOF_ATTEMPT", parentAttempt.AttemptID)
	before, err := proofrun.ReadAttempts(canonicalRoot)
	if err != nil {
		t.Fatal(err)
	}
	attempt, _, _, err = admitProofLaunch(proofLaunchAdmission{ControlRoot: canonicalRoot, ExecutionRoot: canonicalRoot, ConfPath: filepath.Join(canonicalRoot, "metasystem.conf"),
		GoalID: "standing-validation", CapMin: "1", ScopeClass: "selected", CommandClass: "testing", ExpectedGoalRevision: 3, ExpectedAccountingRevision: 2})
	after, readErr := proofrun.ReadAttempts(canonicalRoot)
	if err == nil || !strings.Contains(err.Error(), "GOAL_REVISION_MOVED") || attempt.AttemptID != "" || readErr != nil || len(after) != len(before) {
		t.Fatalf("parent revision admission attempt=%+v error=%v attempts=%d->%d read=%v", attempt, err, len(before), len(after), readErr)
	}
}

func TestGoalHandoverTargetRootFlagFlows(t *testing.T) {
	const batchID = "01j5x00000000000000000ba01"
	root := syncedClaimedGoalFixture(t)
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "landing-machine")
	amendSyncedGoalFixture(t, root, "hand standing validation to landing", func(file *goal.GoalFile) {
		file.Claimed.Machine, file.Claimed.Lineage = "landing-machine", landingOwnerLineage
		file.Claimed.HandedOver = goal.HandedOver{FromMachine: "seat-machine", FromLineage: "seat-lineage", FromEpoch: 1, Batch: batchID}
		file.StopCapability = &goal.StopCapability{Generation: 1, Revision: file.Claimed.Revision, Machine: "landing-machine", ClaimEpoch: 1}
	})
	seat := t.TempDir()
	goalSyncMutationGit(t, seat, "init", "-q", "-b", "main")
	goalSyncMutationGit(t, seat, "config", "metasystem.goal.machine", "seat-machine")
	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe hand-back caller: state=%s err=%v", state, err)
	}
	for _, fixture := range []struct{ checkout, session, lineage string }{
		{root, "landing-fixture", landingOwnerLineage},
		{seat, "seat-fixture", "seat-lineage"},
	} {
		if _, err := lease.AnnounceWithPair(fixture.checkout, fixture.session, parent.Pid, parent.StartedAt.Unix(), parent.StartTicks, parent.BootID, "fixture", "fake", fixture.lineage); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("METASYSTEM_OWNER_LINEAGE", landingOwnerLineage)
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-08-30T08:07:00Z")
	code := runGoalHandoverMutation([]string{"--root", root, "--id", "standing-validation", "--lineage", landingOwnerLineage,
		"--target-machine", "seat-machine", "--target-lineage", "seat-lineage", "--target-claim-epoch", "1", "--batch", batchID, "--target-root", seat})
	if code != 0 {
		t.Fatalf("real target-root return exited %d", code)
	}
	tip := goalSyncMutationGit(t, root, "rev-parse", goal.AcceptedRef)
	data := goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/standing-validation.md")
	returned, problems := goal.ParseFile([]byte(data))
	if len(problems) != 0 || returned.Claimed.Machine != "seat-machine" || returned.Claimed.Lineage != "seat-lineage" || returned.Claimed.HandedOver != (goal.HandedOver{}) {
		t.Fatalf("target-root return claim=%+v problems=%v", returned.Claimed, problems)
	}
}

func TestBatchProductionReturnAndForwardHandoverArguments(t *testing.T) {
	const batchID = "01j5x00000000000000000ba01"
	root, seat := t.TempDir(), filepath.Join(t.TempDir(), "seat")
	store := batch.NewStore(root, nil)
	claim := batch.Claim{Machine: "seat-machine", Lineage: "seat-lineage", Epoch: 3, Revision: 4, AccountingRevision: 4}
	if err := store.Create(batch.Record{Schema: 1, BatchID: batchID, State: batch.StateOpen, Units: []batch.Unit{{
		GoalID: "goal-a", Chain: "chain-a", SeatRoot: seat, Claim: claim, State: batch.UnitReturnPending,
	}}}); err == nil {
		t.Fatal("invalid return-pending fixture unexpectedly passed")
	}
	if err := store.Create(batch.Record{Schema: 1, BatchID: batchID, State: batch.StateOpen, Units: []batch.Unit{{
		GoalID: "goal-a", Chain: "chain-a", SeatRoot: seat, Claim: claim, State: batch.UnitJoined,
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := batch.RequestReturn(store, batchID, "goal-a", batch.UnitEjected, "red", "owner", time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	original, originalRead := batchChildRunner, batchReturnLedgerGoal
	t.Cleanup(func() { batchChildRunner, batchReturnLedgerGoal = original, originalRead })
	batchReturnLedgerGoal = func(string, string, string) (batch.ReturnLedgerGoal, error) {
		return batch.ReturnLedgerGoal{Claimed: true, Batch: batchID}, nil
	}
	var calls [][]string
	batchChildRunner = func(gotRoot, lineage string, args ...string) error {
		calls = append(calls, append([]string{gotRoot, lineage}, args...))
		return nil
	}
	seams := productionReturnSeams(root, func() string { return "tree" })
	if err := seams.HandBack("goal-a", claim, 8); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls[0], " ")
	if !strings.Contains(joined, "--target-root "+seat) || !strings.Contains(joined, "--target-claim-epoch 8") || !strings.Contains(joined, "--batch "+batchID) {
		t.Fatalf("hand-back argv=%v", calls[0])
	}
	calls = nil
	if err := seams.Release("goal-a", "failure first"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || !slices.Contains(calls[0], "edit") || !slices.Contains(calls[1], "release") {
		t.Fatalf("release order=%v", calls)
	}
}

func TestLandingBatchJoinVerbPublishesOutsideFlock(t *testing.T) {
	t.Run("failed preparation does not create an empty batch", func(t *testing.T) {
		landing := t.TempDir()
		dependencies := batchJoinDependencies{
			binding: func(string, string, time.Time) (dispatchcore.GoalBinding, error) {
				return dispatchcore.GoalBinding{Revision: 2, Machine: "seat", Lineage: "lineage", File: &goal.GoalFile{Claimed: &goal.ClaimRecord{AccountingRevision: 1}}, Capability: goal.StopCapability{ClaimEpoch: 1}}, nil
			},
			chain: func(string, string, string, uint64) (batch.CertifiedChain, error) {
				return batch.CertifiedChain{ID: "chain", Patch: []byte("patch")}, nil
			},
			base: func(string) (string, error) { return "base", nil }, mint: func() (string, error) { return "01j5x00000000000000000ba98", nil },
			transport: func(string, batch.CertifiedChain) error { return errors.New("transport refused") },
		}
		if _, err := executeBatchJoin(batchJoinRequest{SeatRoot: t.TempDir(), LandingRoot: landing, GoalID: "goal", ChainID: "chain", At: time.Unix(1, 0)}, dependencies); err == nil {
			t.Fatal("failed transport unexpectedly joined")
		}
		records, err := batch.NewStore(landing, nil).Records()
		if err != nil || len(records) != 0 {
			t.Fatalf("failed preparation left records=%+v err=%v", records, err)
		}
	})
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
	fakeRef := identity.Ref{Pid: 99123, StartedAtSec: 55}
	dependencies.prober = processRefProber{exact: identity.Exact{Pid: fakeRef.Pid, StartedAt: time.Unix(fakeRef.StartedAtSec, 0)}, state: identity.Alive}
	dependencies.binding = func(string, string, time.Time) (dispatchcore.GoalBinding, error) {
		file := &goal.GoalFile{Claimed: &goal.ClaimRecord{AccountingRevision: 4}}
		return dispatchcore.GoalBinding{Revision: 5, Machine: "seat-machine", Lineage: "seat-lineage", File: file,
			Capability: goal.StopCapability{ClaimEpoch: 7}}, nil
	}
	dependencies.chain = func(string, string, string, uint64) (batch.CertifiedChain, error) {
		return batch.CertifiedChain{ID: "chain-a", Patch: patch}, nil
	}
	// Trunk moved after the open batch took its base: the join still joins that batch.
	const movedTrunkTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	dependencies.base = func(string) (string, error) { return movedTrunkTree, nil }
	dependencies.mint = func() (string, error) { return "01j5x00000000000000000ba99", nil }
	dependencies.author = func(string, *goal.GoalFile) (string, string, string, error) {
		return "Fixture", "Fixture", "fixture@example.com", nil
	}
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
	publish := dependencies.publish
	dependencies.publish = func(store batch.Store, id string, unit batch.Unit, actor string, at time.Time,
		plan func(string, string, string) (testpolicy.Plan, error), handover func() error) error {
		if got := store.Liveness(fakeRef); got != identity.Alive {
			t.Fatalf("join store ignored injected prober: %s", got)
		}
		return publish(store, id, unit, actor, at, plan, handover)
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
	if len(record.Units) != 1 || record.Units[0].SeatRoot != seat || record.Units[0].State != batch.UnitJoined || record.BaseTree != base ||
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
