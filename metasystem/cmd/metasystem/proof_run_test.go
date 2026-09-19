package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func candidateProofAdmission(request proofrun.AdmissionRequest) proofrun.AdmissionRequest {
	request.CandidateGoalID = request.GoalID
	request.CandidateRevision = request.AccountingRevision
	request.CandidateBudgetEpoch = request.BudgetEpoch
	if request.Identity.CommandClass == "testing" && request.CandidateTree == "" {
		request.CandidateTree = strings.Repeat("b", 40)
	}
	return proofrun.WithTestHostLoadSampler(request, "0")
}

func requireProofReservationNotAdmissionRefused(t *testing.T, decision proofrun.LaunchResult) {
	t.Helper()
	if decision.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("proof reservation fixture was admission-refused: %+v", decision)
	}
}

func candidateProofLaunchAdmission(request proofLaunchAdmission) proofLaunchAdmission {
	if request.CommandClass == "testing" && request.CandidateTree == "" {
		request.CandidateTree = strings.Repeat("b", 40)
	}
	if request.BeforePublish == nil {
		request.BeforePublish = func(reservation *proofrun.AdmissionRequest) {
			*reservation = proofrun.WithTestHostLoadSampler(*reservation, "0")
		}
	}
	return request
}

func admitCandidateProofLaunch(t *testing.T, request proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
	t.Helper()
	previous := request.BeforePublish
	request.BeforePublish = func(reservation *proofrun.AdmissionRequest) {
		if previous != nil {
			previous(reservation)
		}
		*reservation = proofrun.WithTestHostLoadSampler(*reservation, "0")
	}
	return admitProofLaunch(candidateProofLaunchAdmission(request))
}

func TestProofAdmissionCandidateTreeUsesProofIdentityAccessor(t *testing.T) {
	tree := strings.Repeat("2", 40)
	request := proofLaunchAdmission{CommandClass: "testing", IdentityInputs: []string{
		strings.Repeat("0", 64),
		"candidate-tree:" + tree,
	}}
	if got := proofAdmissionCandidateTree(request); got != tree {
		t.Fatalf("proof admission candidate tree = %q, want %q", got, tree)
	}
}

func addProofCandidateGoal(t *testing.T, root, id, arc string, risk *goal.RiskRecord) *goal.GoalFile {
	t.Helper()
	budget := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 480, ActiveJobLimit: 2, ReviewRoundLimit: 3}
	openedAt, approvedAt := "2026-08-30T08:10:00Z", "2026-08-30T08:11:00Z"
	openOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAV", "mac-cli", id)
	approvalOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAW", "mac-cli", id)
	file := &goal.GoalFile{
		Id: id, State: goal.StateApproved, Tier: 3, Risk: risk, Intent: "Prove candidate " + id + ".", Arc: arc,
		Origin: goal.OriginMain, NextStep: "Prove it.", OpenedAt: openedAt, Revision: 2, Budget: &budget,
		Approved: &goal.ApprovalRecord{By: "human:Wido", At: approvedAt, Revision: 2, EpisodeRevision: 2,
			Opid: approvalOpid, Authority: goal.ApprovalAuthorityProven},
		History: []goal.HistoryLine{
			{At: openedAt, Opid: openOpid, Verb: "open", Actor: "mac-cli+" + id, Targets: []string{id}, Keep: -1},
			{At: approvedAt, Opid: approvalOpid, Verb: "approve", Actor: "human:Wido", Targets: []string{id}, Keep: -1},
		},
	}
	file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, budget, risk)
	path := filepath.Join(root, "plans", "goals", id+".md")
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", filepath.ToSlash(filepath.Join("plans", "goals", id+".md")))
	goalSyncMutationGit(t, root, "commit", "-q", "-m", "add proof candidate "+id)
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	return file
}

func amendProofGoalFixture(t *testing.T, root, id, message string, mutate func(*goal.GoalFile)) *goal.GoalFile {
	t.Helper()
	path := filepath.Join(root, "plans", "goals", id+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse goal %s before amendment: %v", id, problems)
	}
	mutate(file)
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	relative := filepath.ToSlash(filepath.Join("plans", "goals", id+".md"))
	goalSyncMutationGit(t, root, "add", relative)
	goalSyncMutationGit(t, root, "commit", "-q", "-m", message)
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	return file
}

func rebudgetProofGoalFixture(t *testing.T, root, id string, now time.Time) *goal.GoalFile {
	t.Helper()
	return amendProofGoalFixture(t, root, id, "rebudget proof candidate", func(file *goal.GoalFile) {
		file.Revision++
		file.Budget.AttemptLimit++
		event := goal.HistoryLine{At: now.UTC().Format(time.RFC3339),
			Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAX", "mac-cli", id), Verb: "set-budget",
			Actor: "human:Wido", Targets: []string{id}, Keep: -1}
		file.History = append(file.History, event)
		file.Approved = &goal.ApprovalRecord{By: event.Actor, At: event.At, Revision: file.Revision,
			EpisodeRevision: file.Revision, Opid: event.Opid, Authority: goal.ApprovalAuthorityProven,
			Digest: goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)}
		file.BudgetExtension = nil
	})
}

func restoreProofAdmissionSeams(t *testing.T) {
	t.Helper()
	underLocks, beforePublish, afterPublish, lockOrder := proofAdmissionUnderLocks, proofAdmissionBeforePublish, proofAdmissionAfterPublish, proofAdmissionLockOrder
	t.Cleanup(func() {
		proofAdmissionUnderLocks, proofAdmissionBeforePublish = underLocks, beforePublish
		proofAdmissionAfterPublish, proofAdmissionLockOrder = afterPublish, lockOrder
	})
}

func TestCandidateGoalSelectsThePlanRisk(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	high := &goal.RiskRecord{Severity: 3, Novelty: 3, Exposure: 3, Accumulation: 3, Basis: "Candidate risk requires cross-cutting proof."}
	candidate := addProofCandidateGoal(t, root, "candidate-high", "", high)
	risk, revision, err := testingGoalRisk(root, candidate.Id)
	if err != nil || revision != goal.BudgetEpisodeRevision(candidate) || risk.Accumulation != 3 {
		t.Fatalf("candidate risk=%+v revision=%d err=%v", risk, revision, err)
	}
	contract := testpolicy.Contract{
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "candidate", Paths: []string{"src/**"}, Standard: []string{"standard"}, CrossCutting: []string{"cross-cutting"}}},
		Groups: []testpolicy.Group{
			{ID: "standard"},
			{ID: "cross-cutting", Obligations: []string{"cross-cutting"}},
		},
	}
	lowPlan, lowErr := testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: []string{"src/change.go"}, RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery})
	highPlan, highErr := testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: []string{"src/change.go"}, GoalRisk: risk, RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery})
	if lowErr != nil || highErr != nil {
		t.Fatalf("select low=%v high=%v", lowErr, highErr)
	}
	has := func(values []string, target string) bool {
		for _, value := range values {
			if value == target {
				return true
			}
		}
		return false
	}
	if has(lowPlan.SelectedGroups, "cross-cutting") || !has(highPlan.SelectedGroups, "cross-cutting") {
		t.Fatalf("candidate risk did not select its cross-cutting plan: low=%+v high=%+v", lowPlan, highPlan)
	}
	request, _, code := parseTestingSelection("test run", []string{"--root", root, "--goal", candidate.Id, "--authority", "standing-validation"}, true)
	if code != 0 || request.GoalID != candidate.Id || request.AuthorityGoalID != "standing-validation" {
		t.Fatalf("testing selection lost candidate or authority: request=%+v code=%d", request, code)
	}
}

func TestCandidateGoalEligibilityTable(t *testing.T) {
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2}
	approved := func(state string) *goal.GoalFile {
		return &goal.GoalFile{Id: "candidate", State: state, Revision: 3, Budget: budget,
			Approved: &goal.ApprovalRecord{Revision: 3, EpisodeRevision: 3}}
	}
	tests := []struct {
		name, wantState string
		file            *goal.GoalFile
		done            bool
		admitted        bool
	}{
		{name: "absent", wantState: "absent"},
		{name: "queued", wantState: goal.StateQueued, file: approved(goal.StateQueued)},
		{name: "no budget", wantState: "no-budget", file: func() *goal.GoalFile { f := approved(goal.StateApproved); f.Budget = nil; return f }()},
		{name: "done", wantState: goal.StateDone, file: approved(goal.StateDone), done: true},
		{name: "parked", wantState: goal.StateParked, file: approved(goal.StateParked)},
		{name: "fenced", wantState: "fenced", file: func() *goal.GoalFile {
			f := approved(goal.StateClaimed)
			f.Claimed = &goal.ClaimRecord{Machine: "m1"}
			f.StopFence = &goal.StopFence{StopID: "stop-candidate"}
			return f
		}()},
		{name: "claimed on m2", wantState: "claimed", file: func() *goal.GoalFile {
			f := approved(goal.StateClaimed)
			f.Claimed = &goal.ClaimRecord{Machine: "m2"}
			return f
		}()},
		{name: "approved unclaimed", file: approved(goal.StateApproved), admitted: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tree := &goal.TreeGoals{Live: map[string]*goal.GoalFile{}, Done: map[string]*goal.GoalFile{}}
			if test.file != nil {
				if test.done {
					tree.Done[test.file.Id] = test.file
				} else {
					tree.Live[test.file.Id] = test.file
				}
			}
			file, revision, err := candidateGoalForProof(tree, "candidate", "m1")
			if test.admitted {
				if err != nil || file == nil || revision != 3 {
					t.Fatalf("approved candidate refused: file=%+v revision=%d err=%v", file, revision, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "CANDIDATE_GOAL_REFUSED") || !strings.Contains(err.Error(), "state="+test.wantState) {
				t.Fatalf("ineligible candidate file=%+v revision=%d err=%v", file, revision, err)
			}
		})
	}
}

func TestCandidateLaunchStillNeedsTheClaimHolder(t *testing.T) {
	root, now := proofExtensionGoalFixture(t)
	addProofCandidateGoal(t, root, "candidate-holder", "", &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Holder witness."})
	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe parent: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "candidate-nonholder", parent.Pid, parent.StartedAt.Unix(), parent.StartTicks, parent.BootID, "candidate-nonholder", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	leasePath := filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json")
	data, err := os.ReadFile(leasePath)
	if err != nil {
		t.Fatal(err)
	}
	var checkoutLease lease.Lease
	if err := json.Unmarshal(data, &checkoutLease); err != nil {
		t.Fatal(err)
	}
	checkoutLease.HolderMainId = "main-that-does-not-own-the-caller"
	data, err = json.Marshal(checkoutLease)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(leasePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	attempt, _, _, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"),
		GoalID: "candidate-holder", AuthorityGoalID: "standing-validation", CapMin: "1", ScopeClass: "full", CommandClass: "testing",
	})
	if err == nil || !strings.Contains(err.Error(), "active coordinator does not own the claimed goal reservation") || attempt.AttemptID != "" {
		t.Fatalf("candidate bypassed the claim holder: attempt=%+v err=%v", attempt, err)
	}
}

func TestCandidateUsesResolvedAuthority(t *testing.T) {
	root, now := proofExtensionGoalFixture(t)
	candidate := addProofCandidateGoal(t, root, "candidate-authority", "", &goal.RiskRecord{
		Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Separate candidate and authority witness.",
	})
	announceProofFixtureHolder(t, root)
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	attempt, result, noChild, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"),
		GoalID: candidate.Id, AuthorityGoalID: "standing-validation", CapMin: "1", ScopeClass: "full", CommandClass: "testing",
	})
	if err != nil || noChild || result.Disposition != proofrun.DispositionExecuted {
		t.Fatalf("cross-candidate admission failed: attempt=%+v result=%+v noChild=%t err=%v", attempt, result, noChild, err)
	}
	stored, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.GoalID != "standing-validation" || stored.AccountedGoal() != candidate.Id ||
		stored.AccountedRevision() != goal.BudgetEpisodeRevision(candidate) {
		t.Fatalf("candidate/authority tuple collapsed: attempt=%+v", stored)
	}
}

func TestCandidateGoalTransitionUnderLockIsBeforeOrAfter(t *testing.T) {
	for _, test := range []struct {
		name, seam string
		after      bool
	}{
		{name: "park before first snapshot", seam: "under-locks"},
		{name: "rebudget before publish", seam: "before-publish"},
		{name: "rebudget after publish", seam: "after-publish"},
		{name: "rebudget after admission", after: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			restoreProofAdmissionSeams(t)
			root, now := proofExtensionGoalFixture(t)
			candidate := addProofCandidateGoal(t, root, "candidate-move", "", &goal.RiskRecord{
				Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Linearization witness."})
			announceProofFixtureHolder(t, root)
			t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
			move := func() {
				if test.seam == "under-locks" {
					amendProofGoalFixture(t, root, candidate.Id, "park proof candidate", func(file *goal.GoalFile) {
						file.Revision++
						file.State = goal.StateParked
						file.Parked = &goal.ParkRecord{By: "mac-cli+m1", At: now.Format(time.RFC3339), Because: "linearization witness"}
						file.History = append(file.History, goal.HistoryLine{At: now.Format(time.RFC3339),
							Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAY", "mac-cli", candidate.Id), Verb: "park",
							Actor: "mac-cli+m1", Targets: []string{candidate.Id}, Keep: -1, Reason: "linearization witness"})
					})
					return
				}
				rebudgetProofGoalFixture(t, root, candidate.Id, now)
			}
			switch test.seam {
			case "under-locks":
				proofAdmissionUnderLocks = move
			case "before-publish":
				proofAdmissionBeforePublish = func(*proofrun.AdmissionRequest) { move() }
			case "after-publish":
				proofAdmissionAfterPublish = move
			}
			attempt, result, _, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
				ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"),
				GoalID: candidate.Id, AuthorityGoalID: "standing-validation", CapMin: "1", ScopeClass: "full", CommandClass: "testing",
			})
			if test.after {
				if err != nil || result.Disposition != proofrun.DispositionExecuted || attempt.AttemptID == "" {
					t.Fatalf("admission before later rebudget: attempt=%+v result=%+v err=%v", attempt, result, err)
				}
				moved := rebudgetProofGoalFixture(t, root, candidate.Id, now)
				stored, readErr := proofrun.ReadAttempt(root, attempt.AttemptID)
				consumption := dispatchcore.ProjectConsumption(root, moved, now)
				if readErr != nil || stored.CandidateRevision != goal.BudgetEpisodeRevision(candidate) ||
					consumption.Status != dispatchcore.BudgetKnown || consumption.Attempts != 0 {
					t.Fatalf("later episode did not leave the admitted attempt on its old episode: stored=%+v consumption=%+v err=%v", stored, consumption, readErr)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "CANDIDATE_GOAL_MOVED") || attempt.AttemptID != "" {
				t.Fatalf("movement at %s was not refused: attempt=%+v result=%+v err=%v", test.seam, attempt, result, err)
			}
			attempts, readErr := proofrun.ReadAttempts(root)
			if readErr != nil || len(attempts) != 0 {
				t.Fatalf("movement at %s retained a reservation: attempts=%+v err=%v", test.seam, attempts, readErr)
			}
		})
	}
}

func TestProofAdmissionLocksReciprocalGoalsInSortedOrder(t *testing.T) {
	restoreProofAdmissionSeams(t)
	root := t.TempDir()
	var orders [][]string
	proofAdmissionLockOrder = func(order []string) { orders = append(orders, order) }
	for _, snapshots := range []proofAdmissionSnapshots{
		{Candidate: proofAdmissionGoalSnapshot{ID: "b", LockRevision: 2}, Authority: proofAdmissionGoalSnapshot{ID: "a", LockRevision: 3}},
		{Candidate: proofAdmissionGoalSnapshot{ID: "a", LockRevision: 3}, Authority: proofAdmissionGoalSnapshot{ID: "b", LockRevision: 2}},
	} {
		held, err := acquireProofAdmissionGoalLocks(root, snapshots)
		if err != nil {
			t.Fatal(err)
		}
		releaseProofAdmissionGoalLocks(held)
	}
	if len(orders) != 2 || strings.Join(orders[0], ",") != "a,b" || strings.Join(orders[1], ",") != "a,b" {
		t.Fatalf("reciprocal acquisition orders = %v, want [a b] twice", orders)
	}
}

func TestProofAdmissionAcquireWaitZeroNamesBothGoalsAndBusyPath(t *testing.T) {
	restoreProofAdmissionSeams(t)
	root, now := proofExtensionGoalFixture(t)
	candidate := addProofCandidateGoal(t, root, "z-candidate-busy", "", &goal.RiskRecord{
		Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Busy lock witness."})
	held, err := goalrevision.Acquire(root, candidate.Id, candidate.Revision, "busy-witness")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Release() })
	previousWait := goalrevision.AcquireWait
	goalrevision.AcquireWait = 0
	t.Cleanup(func() { goalrevision.AcquireWait = previousWait })
	announceProofFixtureHolder(t, root)
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	attempt, _, _, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: candidate.Id,
		AuthorityGoalID: "standing-validation", CapMin: "1", ScopeClass: "full", CommandClass: "testing",
	})
	for _, want := range []string{candidate.Id, "standing-validation", "LOCK_BUSY", candidate.Id + "/r2"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("zero-wait refusal did not name %q: attempt=%+v err=%v", want, attempt, err)
		}
	}
}

func TestCandidateCannotUseAuthorityEarnedExtension(t *testing.T) {
	root, now := proofExtensionGoalFixture(t)
	addProofCandidateGoal(t, root, "candidate-no-extension", "", &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Extension authority witness."})
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, jobs, "authority-spent.json", map[string]any{
		"jobId": "authority-spent", "operationId": "authority-spent", "goalId": "standing-validation", "goalRevision": 2,
		"capMin": 1, "status": "completed", "startedAt": "2026-08-30T08:20:00Z", "endedAt": "2026-08-30T08:21:00Z",
	})
	announceProofFixtureHolder(t, root)
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	attempt, decision, _, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"),
		GoalID: "candidate-no-extension", AuthorityGoalID: "standing-validation", CapMin: "1", ScopeClass: "full", CommandClass: "testing",
	})
	if err != nil || decision.Disposition != proofrun.DispositionExecuted || attempt.AttemptID == "" {
		t.Fatalf("authority consumption incorrectly closed the candidate lens: attempt=%+v decision=%+v err=%v", attempt, decision, err)
	}
	data, readErr := os.ReadFile(filepath.Join(root, "plans", "goals", "standing-validation.md"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 || file.BudgetExtension != nil {
		t.Fatalf("cross-candidate admission used the authority's extension: extension=%+v problems=%v", file.BudgetExtension, problems)
	}
}

func TestOneLiveChargedAttemptCountsForBothLenses(t *testing.T) {
	t.Parallel()
	root, now := proofExtensionGoalFixture(t)
	amendSyncedGoalFixture(t, root, "bound authority concurrency", func(file *goal.GoalFile) {
		file.Budget.AttemptLimit = 6
		file.Budget.ActiveJobLimit = 1
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	candidate := addProofCandidateGoal(t, root, "candidate-two-lens", "", &goal.RiskRecord{
		Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Two-lens witness."})
	candidate = amendProofGoalFixture(t, root, candidate.Id, "bound candidate attempts", func(file *goal.GoalFile) {
		file.Budget.AttemptLimit = 1
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	other := addProofCandidateGoal(t, root, "candidate-other", "", &goal.RiskRecord{
		Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Authority concurrency witness."})
	announceProofFixtureHolder(t, root)
	beforePublish := false
	launch := func(goalID, tree string) (proofrun.Attempt, error) {
		attempt, _, _, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
			ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"),
			GoalID: goalID, AuthorityGoalID: "standing-validation", CapMin: "1", ScopeClass: "full",
			CommandClass: "testing", CandidateTree: tree, Now: now,
			BeforePublish: func(*proofrun.AdmissionRequest) { beforePublish = true },
		})
		return attempt, err
	}
	first, err := launch(candidate.Id, strings.Repeat("b", 40))
	if err != nil || !beforePublish || first.AttemptID == "" || first.GoalID != "standing-validation" || first.CandidateGoalID != candidate.Id {
		t.Fatalf("first two-lens launch: attempt=%+v err=%v", first, err)
	}
	binding, err := dispatchcore.ResolveGoalBinding(root, "standing-validation", now)
	if err != nil {
		t.Fatal(err)
	}
	authority := dispatchcore.ProjectBudget(root, binding.File, now)
	if authority.ActiveJobs != 1 || authority.Attempts != 0 {
		t.Fatalf("live candidate charge did not split authority capacity from consumption: %+v", authority)
	}
	if attempt, err := launch(other.Id, strings.Repeat("c", 40)); err == nil || attempt.AttemptID != "" ||
		!strings.Contains(err.Error(), "standing-validation") || !strings.Contains(err.Error(), "activeJobLimit") {
		t.Fatalf("second authority job was not refused by the authority lens: attempt=%+v err=%v", attempt, err)
	}
	if _, err := proofrun.FinalizeAttempt(root, first.AttemptID, proofrun.TerminalFailed, 1, "two-lens witness", nil, now); err != nil {
		t.Fatal(err)
	}
	if attempt, err := launch(candidate.Id, strings.Repeat("d", 40)); err == nil || attempt.AttemptID != "" ||
		!strings.Contains(err.Error(), candidate.Id) || !strings.Contains(err.Error(), "attemptLimit") {
		t.Fatalf("second candidate attempt was not refused by the candidate lens: attempt=%+v err=%v", attempt, err)
	}
}

func TestCandidateCannotEscapeAuthorityElapsedLimit(t *testing.T) {
	t.Parallel()
	root, now := proofExtensionGoalFixture(t)
	amendSyncedGoalFixture(t, root, "expire authority elapsed budget", func(file *goal.GoalFile) {
		file.Budget.ElapsedLimit = "1m"
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	candidate := addProofCandidateGoal(t, root, "candidate-elapsed", "", &goal.RiskRecord{
		Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Elapsed authority witness."})
	announceProofFixtureHolder(t, root)
	attempt, _, _, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: candidate.Id,
		AuthorityGoalID: "standing-validation", CapMin: "1", ScopeClass: "full", CommandClass: "testing", Now: now,
	})
	if err == nil || attempt.AttemptID != "" || !strings.Contains(err.Error(), "standing-validation") || !strings.Contains(err.Error(), "elapsedLimit") {
		t.Fatalf("candidate escaped the authority clock: attempt=%+v err=%v", attempt, err)
	}
}

func TestCandidateExtensionIsRefusedUntilCandidateBecomesAuthority(t *testing.T) {
	t.Parallel()
	root, now := proofExtensionGoalFixture(t)
	candidate := addProofCandidateGoal(t, root, "candidate-extension", "", &goal.RiskRecord{
		Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Candidate extension witness."})
	candidate = amendProofGoalFixture(t, root, candidate.Id, "bound candidate extension", func(file *goal.GoalFile) {
		file.Budget.AttemptLimit = 1
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	receiptPath := filepath.Join(root, "memory", "receipts.log")
	receipts, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	receiptAt := now.Add(-30 * time.Minute)
	receipts = append(receipts, []byte(fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=shipped|goal=%s|note=candidate extension witness\n",
		receiptAt.Unix(), receiptAt.Format(time.RFC3339), candidate.Id))...)
	if err := os.WriteFile(receiptPath, receipts, 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "memory/receipts.log")
	goalSyncMutationGit(t, root, "commit", "-q", "-m", "candidate extension receipt")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	announceProofFixtureHolder(t, root)
	first, _, _, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: candidate.Id,
		AuthorityGoalID: "standing-validation", CapMin: "1", ScopeClass: "full", CommandClass: "testing", CandidateTree: strings.Repeat("b", 40), Now: now,
	})
	if err != nil || first.AttemptID == "" {
		t.Fatalf("first candidate charge: attempt=%+v err=%v", first, err)
	}
	if _, err := proofrun.FinalizeAttempt(root, first.AttemptID, proofrun.TerminalFailed, 1, "candidate extension witness", nil, now); err != nil {
		t.Fatal(err)
	}
	request := proofLaunchAdmission{ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: candidate.Id,
		AuthorityGoalID: "standing-validation", CapMin: "1", ScopeClass: "full", CommandClass: "testing", CandidateTree: strings.Repeat("c", 40), Now: now}
	attempt, _, _, err := admitCandidateProofLaunch(t, request)
	for _, want := range []string{"CANDIDATE_EXTENSION_REFUSED", candidate.Id, "attemptLimit", "claim " + candidate.Id + " as authority", "goal set-budget"} {
		if err == nil || !strings.Contains(err.Error(), want) || attempt.AttemptID != "" {
			t.Fatalf("candidate extension refusal did not name %q: attempt=%+v err=%v", want, attempt, err)
		}
	}
	amendSyncedGoalFixture(t, root, "release old proof authority", func(file *goal.GoalFile) {
		file.Revision++
		file.State, file.Claimed, file.StopCapability = goal.StateApproved, nil, nil
		file.History = append(file.History, goal.HistoryLine{At: now.Format(time.RFC3339),
			Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAT", "mac-cli", "m1"), Verb: "release",
			Actor: "mac-cli+m1", Targets: []string{"standing-validation"}, Keep: -1})
	})
	unchanged := amendProofGoalFixture(t, root, candidate.Id, "claim extension candidate", func(file *goal.GoalFile) {
		if file.BudgetExtension != nil {
			t.Fatalf("candidate refusal wrote an extension: %+v", file.BudgetExtension)
		}
		file.Revision++
		claimedAt := now.Add(-time.Minute).Format(time.RFC3339)
		event := goal.HistoryLine{At: claimedAt, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAZ", "mac-cli", candidate.Id),
			Verb: "claim", Actor: "mac-cli+m1", Targets: []string{candidate.Id}, Keep: -1}
		file.History = append(file.History, event)
		file.State = goal.StateClaimed
		file.Claimed = &goal.ClaimRecord{Machine: "mac-cli", Lineage: "m1", At: claimedAt, Revision: file.Revision, AccountingRevision: file.Revision}
		file.StopCapability = &goal.StopCapability{Generation: file.Revision, Revision: file.Revision, Machine: "mac-cli", ClaimEpoch: 1}
	})
	request.AuthorityGoalID = ""
	attempt, decision, _, err := admitCandidateProofLaunch(t, request)
	if err != nil || decision.Disposition != proofrun.DispositionExecuted || attempt.AttemptID == "" {
		t.Fatalf("candidate as authority did not receive its own earned extension: goal=%+v attempt=%+v decision=%+v err=%v", unchanged, attempt, decision, err)
	}
	tip := goalSyncMutationGit(t, root, "rev-parse", goal.AcceptedRef)
	data := goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/"+candidate.Id+".md")
	file, problems := goal.ParseFile([]byte(data))
	if len(problems) != 0 || file.BudgetExtension == nil {
		t.Fatalf("candidate authority extension was not persisted: extension=%+v problems=%v", file.BudgetExtension, problems)
	}
}

func TestBoundProofContextsRequireTheirOwnAuthority(t *testing.T) {
	for _, test := range []struct {
		name, kind, candidate, authority string
	}{
		{name: "governed candidate differs", kind: "governed run", candidate: "candidate"},
		{name: "governed authority flag", kind: "governed run", authority: "run-goal"},
		{name: "delegate candidate differs", kind: "native delegate", candidate: "candidate"},
		{name: "delegate authority flag", kind: "native delegate", authority: "run-goal"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := enforceBoundProofGoals(test.kind, "run-goal", test.candidate, test.authority)
			if err == nil || !strings.Contains(err.Error(), "PROOF_AUTHORITY_REQUIRED") || !strings.Contains(err.Error(), "run-goal") {
				t.Fatalf("bound context accepted candidate=%q authority=%q: %v", test.candidate, test.authority, err)
			}
		})
	}
}

func TestArcMateAuthorityIsRefused(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	amendSyncedGoalFixture(t, root, "put authority in arc-a", func(file *goal.GoalFile) { file.Arc = "arc-a" })
	addProofCandidateGoal(t, root, "candidate-arc", "arc-a", &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Arc witness."})
	addProofCandidateGoal(t, root, "candidate-outside", "arc-b", &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "Outside arc witness."})
	for _, authority := range []string{"standing-validation", ""} {
		roles, err := resolveProofGoalRoles(root, "candidate-arc", authority, now)
		if err == nil || !strings.Contains(err.Error(), "PROOF_AUTHORITY_ARC_MATE_REFUSED") ||
			!strings.Contains(err.Error(), "candidate-arc") || !strings.Contains(err.Error(), "standing-validation") ||
			!strings.Contains(err.Error(), "arc-a") || roles.Authority != nil {
			t.Fatalf("arc mate authority=%q roles=%+v err=%v", authority, roles, err)
		}
	}
	roles, err := resolveProofGoalRoles(root, "candidate-outside", "standing-validation", now)
	if err != nil || roles.Candidate.Id != "candidate-outside" || roles.Authority.Id != "standing-validation" {
		t.Fatalf("outside-arc authority refused: roles=%+v err=%v", roles, err)
	}
}

func TestProofRunWitnessStateUsesProbeAndFrozenEligibility(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o700); err != nil {
		t.Fatal(err)
	}
	tracked := filepath.Join(root, "internal", "tracked")
	if err := os.WriteFile(tracked, []byte("clean\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "add", "internal/tracked")
	runGit(t, root, "-c", "user.name=metasystem", "-c", "user.email=metasystem@example.invalid", "commit", "-qm", "initial")
	for name, value := range map[string]string{
		"METASYSTEM_GATE_WITNESS": "", "METASYSTEM_GATE_WITNESS_EXPORT": "",
		"METASYSTEM_COVERAGE_RATCHET_SEED": "0", "METASYSTEM_GATE_FORCE": "0",
		"METASYSTEM_DELIVERY_CONTRACT": "0", "GOFLAGS": "",
	} {
		t.Setenv(name, value)
	}
	if state := proofRunWitnessState(root); state != "unarmed" {
		t.Fatalf("clean state = %q", state)
	}
	if err := os.WriteFile(tracked, []byte("dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if state := proofRunWitnessState(root); state != "frozen" {
		t.Fatalf("eligible dirty state = %q", state)
	}
	t.Setenv("METASYSTEM_GATE_FORCE", "1")
	if state := proofRunWitnessState(root); state != "unarmed" {
		t.Fatalf("forced dirty state = %q", state)
	}
	t.Setenv("METASYSTEM_GATE_FORCE", "0")
	t.Setenv("METASYSTEM_GATE_WITNESS", "unusable")
	if state := proofRunWitnessState(root); state != "unarmed" {
		t.Fatalf("unusable witness state = %q", state)
	}

	script := filepath.Join(root, "scripts", "agents", "go-gate.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\n[[ \"$1\" == --witness-check-only && \"$METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE\" == ENGINE && \"$METASYSTEM_GATE_WITNESS\" == usable ]]\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_GATE_WITNESS", "usable")
	if state := proofRunWitnessState(root); state != "armed" {
		t.Fatalf("usable witness state = %q", state)
	}
	export := filepath.Join(root, "export")
	t.Setenv("METASYSTEM_GATE_WITNESS_EXPORT", export)
	if state := proofRunWitnessState(root); state != "unarmed" {
		t.Fatalf("missing exported witness state = %q", state)
	}
	if err := os.Mkdir(export, 0o700); err != nil {
		t.Fatal(err)
	}
	if state := proofRunWitnessState(root); state != "frozen" {
		t.Fatalf("usable exported witness state = %q", state)
	}
}

func runGit(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}

func TestProofRunLimitsDefaultSilentlyWhenOperationalKnobsAreAbsent(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	limits, err := resolveProofRunLimits(conf)
	if err != nil {
		t.Fatal(err)
	}
	if limits.silence != 30*time.Minute || limits.sectionCap != 45*time.Minute ||
		limits.evidenceTimeout != 60*time.Second || limits.evidenceMax != 512*1024*1024 {
		t.Fatalf("default proof-run limits = %+v", limits)
	}
}

func workerAuthorizedAttemptFixture(t *testing.T) (string, string, proofrun.Attempt) {
	t.Helper()
	controlRoot, _ := proofExtensionGoalFixture(t)
	controlRoot, err := canonicalProofRoot(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	executionRoot := t.TempDir()
	proofIdentity, err := proofrun.BuildProofIdentity(controlRoot, filepath.Join(controlRoot, "metasystem.conf"), "full", "worker-root", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.ProcessIdentityForPID(int64(os.Getppid()), nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt, decision, err := proofrun.ReserveLocked(proofrun.WithTestHostLoadSampler(proofrun.AdmissionRequest{
		ControlRoot: controlRoot, ExecutionRoot: executionRoot, GoalID: "standing-validation", GoalRevision: 2,
		AccountingRevision: 2, CandidateGoalID: "standing-validation", CandidateRevision: 2,
		ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher,
		Now: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC),
	}, "0"))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("worker-authorized attempt fixture was admission-refused: %+v", decision)
	}
	t.Setenv("METASYSTEM_PROOF_CONTROL_ROOT", controlRoot)
	t.Setenv("METASYSTEM_PROOF_ATTEMPT", attempt.AttemptID)
	t.Setenv("METASYSTEM_PROOF_RECORD_KEY", "")
	t.Setenv("METASYSTEM_PROOF_CREATION_CLAIM", "")
	t.Setenv(proofWitnessExecutionRootEnv, "")
	t.Setenv("METASYSTEM_GATE_WITNESS_WRITE", "")
	return controlRoot, executionRoot, attempt
}

func TestWorkerAuthorizedAcceptsTheAdmittedRoot(t *testing.T) {
	_, root, _ := workerAuthorizedAttemptFixture(t)
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runProofRunWorkerAuthorized([]string{"--root", root})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("admitted execution root was not authorized: code=%d stderr=%q", code, stderr)
	}
}

func TestWorkerAuthorizedAcceptsAttemptWitnessSnapshot(t *testing.T) {
	_, root, _ := workerAuthorizedAttemptFixture(t)
	snapshot := t.TempDir()
	t.Chdir(snapshot)
	t.Setenv(proofWitnessExecutionRootEnv, root)
	t.Setenv("METASYSTEM_GATE_WITNESS_WRITE", filepath.Join(t.TempDir(), "witness.json"))
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runProofRunWorkerAuthorized([]string{"--root", snapshot})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("attempt witness snapshot was not authorized: code=%d stderr=%q", code, stderr)
	}
}

func TestWorkerAuthorizedRefusesAForeignRoot(t *testing.T) {
	foreign, root, attempt := workerAuthorizedAttemptFixture(t)
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runProofRunWorkerAuthorized([]string{"--root", foreign})
	})
	if code != 3 || !strings.Contains(stderr, foreign) || !strings.Contains(stderr, attempt.ExecutionRoot) {
		t.Fatalf("foreign root refusal = code=%d stderr=%q; want both %q and %q", code, stderr, foreign, root)
	}
}

func TestSuiteProgressPrinterSurfacesDeepestLiveSection(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "artifacts", "agents", "supervision", "suite-progress.jsonl")
	if err := proofrun.AppendProgressHeader(progress, proofrun.ProgressHeader{LogPaths: []string{"suite.log"}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, event := range []proofrun.SectionEvent{
		{Suite: "outer", Section: "parent", Event: "start", At: now, Depth: 0},
		{Suite: "inner", Section: "child", Event: "start", At: now, Depth: 1},
	} {
		if err := proofrun.AppendSectionEvent(progress, event); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	stop := startSuiteProgressPrinter(root, time.Hour, &output)
	stop()
	if got := strings.TrimSpace(output.String()); got != "inner:child since 0min" {
		t.Fatalf("progress note = %q", got)
	}
}

func TestSelectedSectionsReadsTwiceConsultedDataFromSelector(t *testing.T) {
	selector := filepath.Join(t.TempDir(), "selector.sh")
	script := `#!/usr/bin/env bash
case "$1" in
  list) printf 'first\tfirst section\nrepeat\trepeated section\n' ;;
  twice) printf 'repeat\n' ;;
  *) exit 2 ;;
esac
`
	if err := os.WriteFile(selector, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	sections, repeated, err := selectedSections(selector, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(sections, ",") != "first,repeat" || len(repeated) != 1 || !repeated["repeat"] {
		t.Fatalf("selector data = %v, %v", sections, repeated)
	}
	// A selected run drives one call site, so even a declared-twice section
	// expects a single interval there.
	sections, repeated, err = selectedSections(selector, "repeat", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0] != "repeat" || len(repeated) != 0 {
		t.Fatalf("selected selector data = %v, %v", sections, repeated)
	}
	sections, repeated, err = selectedSections(selector, "first", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0] != "first" || len(repeated) != 0 {
		t.Fatalf("non-repeated selected selector data = %v, %v", sections, repeated)
	}
	sections, repeated, err = selectedSections(selector, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(sections, ",") != "first,repeat" || len(repeated) != 0 {
		t.Fatalf("enumerated selector data = %v, %v", sections, repeated)
	}
}

func TestProofRunLimitsRejectOutOfRangeLocalOverride(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("suite.section-cap-min=601\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := resolveProofRunLimits(conf)
	if err == nil || !strings.Contains(err.Error(), "suite.section-cap-min must be an integer from 1 through 600") {
		t.Fatalf("error = %v", err)
	}

	for _, test := range []struct {
		line string
		want string
	}{
		{"suite.progress-silence-min=0\n", "suite.progress-silence-min must be an integer from 1 through 600"},
		{"suite.section-cap-min=601\n", "suite.section-cap-min must be an integer from 1 through 600"},
		{"suite.evidence-copy-timeout-sec=0\n", "suite.evidence-copy-timeout-sec must be an integer from 1 through 600"},
		{"suite.evidence-copy-max-mb=10241\n", "suite.evidence-copy-max-mb must be an integer from 1 through 10240"},
	} {
		if err := os.WriteFile(conf, []byte(test.line), 0o600); err != nil {
			t.Fatal(err)
		}
		problems, err := proofRunConfigProblems(conf)
		if err != nil || len(problems) != 1 || !strings.Contains(problems[0], test.want) {
			t.Fatalf("config validation problems for %q = %v, %v", test.line, problems, err)
		}
	}
}

func TestProofRunLimitsRejectEffectiveLocalAndEnvironmentOverlays(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		envName  string
		wantText string
	}{
		{"suite.section-cap-min", "601", "METASYSTEM_SUITE_SECTION_CAP_MIN", "suite.section-cap-min"},
		{"suite.evidence-copy-timeout-sec", "0", "METASYSTEM_SUITE_EVIDENCE_COPY_TIMEOUT_SEC", "suite.evidence-copy-timeout-sec"},
		{"suite.evidence-copy-max-mb", "10241", "METASYSTEM_SUITE_EVIDENCE_COPY_MAX_MB", "suite.evidence-copy-max-mb"},
	}
	for _, test := range tests {
		t.Run(test.key+" local", func(t *testing.T) {
			conf := filepath.Join(t.TempDir(), "metasystem.conf")
			if err := os.WriteFile(conf, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(conf+".local", []byte(test.key+"="+test.value+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := resolveProofRunLimits(conf); err == nil || !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("effective local error = %v", err)
			}
		})
		t.Run(test.key+" environment", func(t *testing.T) {
			conf := filepath.Join(t.TempDir(), "metasystem.conf")
			if err := os.WriteFile(conf, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(test.envName, test.value)
			if _, err := resolveProofRunLimits(conf); err == nil || !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("effective environment error = %v", err)
			}
		})
	}
}

// The command's terminal commit refuses a success when the goal-revision
// authority it needs is absent (this fixture's goal carries no stop
// capability): the launcher reports it, exits nonzero, and no terminal is
// written. This test once claimed to recheck the deadline after
// preparation; it never reached it, the authority refusal came first, and
// decision 3 of the hang-detection design removed the recheck anyway (the
// deadline is a reservation horizon, proven in proofrun's launcher test).
func TestCommitProofTerminalRefusesWithoutGoalRevisionAuthority(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	proofIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "deadline-finalization", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	attempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root,
		GoalID: "standing-validation", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 30,
		Identity: proofIdentity, Launcher: launcher, Now: started}))

	if err != nil || decision.Disposition != proofrun.DispositionExecuted {
		t.Fatalf("reserve = %+v, %+v, %v", attempt, decision, err)
	}
	deadline, err := time.Parse(time.RFC3339Nano, attempt.Deadline)
	if err != nil {
		t.Fatal(err)
	}
	artifactDir := filepath.Join(root, "artifacts", "deadline-finalization")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatal(err)
	}
	watchdog := filepath.Join(artifactDir, "watchdog.sh")
	if err := os.WriteFile(watchdog, []byte(`#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
while [[ ! -e "$done_path" ]]; do sleep 0.005; done
`), 0o755); err != nil {
		t.Fatal(err)
	}
	var launcherErrors bytes.Buffer
	result := proofrun.LaunchSuite(proofrun.LaunchOptions{Suite: "deadline-finalization", Root: root, ControlRoot: root, ErrorOutput: &launcherErrors,
		AttemptID: attempt.AttemptID, Deadline: deadline, ConfPath: filepath.Join(root, "metasystem.conf"),
		ProgressPath: filepath.Join(artifactDir, "progress.jsonl"), LogPath: filepath.Join(artifactDir, "proof.log"),
		Banner: "deadline finalization fixture", Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second,
		EvidenceMax: 1024, Poll: 5 * time.Millisecond, TermGrace: time.Second, KillGrace: time.Second,
		WatchdogExecutable: watchdog, Command: []string{"true"},
		PrepareSuccess: func(proofrun.CompletionContext) (json.RawMessage, error) {
			return json.RawMessage(`{"preparedAt":"before-terminal-locks"}`), nil
		}, CommitTerminal: commitProofTerminal})
	if result == 0 || !strings.Contains(launcherErrors.String(), "lost goal-revision authority") {
		t.Fatalf("a terminal commit without goal-revision authority was not refused by name: result %d\n%s", result, launcherErrors.String())
	}
	// The launcher's fallback retains the attempt as incomplete once the
	// commit is refused; what must never appear is a success.
	stored, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || (stored.Terminal != nil && stored.Terminal.Result == proofrun.TerminalSuccess) || len(stored.DeliveryReceipt) != 0 {
		t.Fatalf("a refused terminal commit still published a success: attempt=%+v err=%v", stored, err)
	}
	if _, err := proofrun.FinalizeAttempt(root, attempt.AttemptID, proofrun.TerminalFailed, 1, "deadline canary cleanup", nil, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

func proofExtensionGoalFixture(t *testing.T) (string, time.Time) {
	t.Helper()
	root := syncedClaimedGoalFixture(t)
	now := time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\nmetasystem.budget.tier-3=8h/1/1200m/1/3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pinProofBinaryFixture(t, root)
	amendSyncedGoalFixture(t, root, "proof extension fixture", func(file *goal.GoalFile) {
		file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
		file.Budget.AttemptLimit = 1
		file.Budget.ReservedJobMinutesLimit = 10000
		file.Budget.ActiveJobLimit = 10
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	if err := os.MkdirAll(filepath.Join(root, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	receiptAt := now.Add(-time.Hour)
	receipt := fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=shipped|goal=standing-validation|note=proof fixture\n",
		receiptAt.Unix(), receiptAt.Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(root, "memory", "receipts.log"), []byte(receipt), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "memory/receipts.log", "metasystem.conf")
	goalSyncMutationGit(t, root, "commit", "-q", "-m", "proof extension receipt")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	return root, now
}

func TestProofAdmissionExtendsRejudgesAndReserves(t *testing.T) {
	root, now := proofExtensionGoalFixture(t)

	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, jobs, "proof-spent.json", map[string]any{
		"jobId": "proof-spent", "operationId": "proof-spent", "goalId": "standing-validation", "goalRevision": 2,
		"capMin": 1, "status": "completed", "startedAt": "2026-08-30T08:20:00Z", "endedAt": "2026-08-30T08:21:00Z",
	})

	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe proof caller: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "proof-extension-main", parent.Pid, parent.StartedAt.Unix(),
		parent.StartTicks, parent.BootID, "proof-extension-main", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	attempt, decision, joined, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: "standing-validation",
		CapMin: "1", ScopeClass: "full", CommandClass: "testing",
	})

	if err != nil || joined || decision.Disposition != proofrun.DispositionExecuted || attempt.AttemptID == "" ||
		attempt.SchemaVersion != proofrun.CandidateAttemptSchemaVersion || attempt.CandidateGoalID != attempt.GoalID ||
		attempt.CandidateRevision != attempt.AccountingRevision || attempt.CandidateTree != strings.Repeat("b", 40) {
		t.Fatalf("proof admission did not extend and reserve: attempt=%+v decision=%+v joined=%v err=%v", attempt, decision, joined, err)
	}
	tip := goalSyncMutationGit(t, root, "rev-parse", goal.AcceptedRef)
	record := goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/standing-validation.md")
	if !strings.Contains(record, "- BudgetExtension: ") || !strings.Contains(record, "attemptLimit=1->2") ||
		!strings.Contains(record, "- Claimed: machine=mac-cli lineage=m1") {
		t.Fatalf("proof admission did not preserve the claim and marker: %s", record)
	}
}

func TestNativeDelegateProofAdmissionExtendsItsClaimPairBudget(t *testing.T) {
	root, now := proofExtensionGoalFixture(t)
	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe native delegate parent: state=%s err=%v", state, err)
	}
	ref := parent.Ref()
	record := map[string]any{
		"jobId": "native-proof", "operationId": "native-proof", "goalId": "standing-validation", "goalRevision": 2,
		"machineId": "mac-cli", "claimEpoch": 1, "capMin": 1, "status": "running",
		"pid": parent.Pid, "pidStartedAt": ref.StartedAtSec,
	}
	if ref.StartedAtUnixMicro > 0 {
		record["pidStartedAtExactMicro"] = ref.StartedAtUnixMicro
	}
	if ref.StartTicks > 0 {
		record["pidStartTicks"] = ref.StartTicks
		record["bootId"] = ref.BootID
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, jobs, "native-proof.json", record)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_STATE_ROOT", root)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT", root)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_JOB", "native-proof")
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	attempt, decision, joined, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: "standing-validation",
		CapMin: "1", ScopeClass: "full", CommandClass: "testing",
	})

	if err != nil || joined || decision.Disposition != proofrun.DispositionExecuted || attempt.AttemptID == "" {
		t.Fatalf("native delegate proof did not extend and reserve: attempt=%+v decision=%+v joined=%v err=%v", attempt, decision, joined, err)
	}
	tip := goalSyncMutationGit(t, root, "rev-parse", goal.AcceptedRef)
	goalRecord := goalSyncMutationGit(t, root, "cat-file", "-p", tip+":plans/goals/standing-validation.md")
	if !strings.Contains(goalRecord, "- BudgetExtension: ") || !strings.Contains(goalRecord, "attemptLimit=1->2") {
		t.Fatalf("native delegate proof did not persist the extension: %s", goalRecord)
	}
}

func TestSupervisorTakeoverRefusesStaleEpochProof(t *testing.T) {
	root, now := proofExtensionGoalFixture(t)
	amendSyncedGoalFixture(t, root, "landing owner epoch two", func(file *goal.GoalFile) {
		file.StopCapability.ClaimEpoch = 2
	})
	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe stale delegate parent: state=%s err=%v", state, err)
	}
	ref := parent.Ref()
	record := map[string]any{
		"jobId": "stale-proof", "operationId": "stale-proof", "goalId": "standing-validation", "goalRevision": 2,
		"machineId": "mac-cli", "claimEpoch": 1, "capMin": 1, "status": "running",
		"pid": parent.Pid, "pidStartedAt": ref.StartedAtSec,
	}
	if ref.StartedAtUnixMicro > 0 {
		record["pidStartedAtExactMicro"] = ref.StartedAtUnixMicro
	}
	if ref.StartTicks > 0 {
		record["pidStartTicks"], record["bootId"] = ref.StartTicks, ref.BootID
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, jobs, "stale-proof.json", record)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_STATE_ROOT", root)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT", root)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_JOB", "stale-proof")
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	attempt, _, _, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: "standing-validation",
		CapMin: "1", ScopeClass: "full", CommandClass: "testing",
	})

	if err == nil || !strings.Contains(err.Error(), "native delegate proof custody changed before reservation") || attempt.AttemptID != "" {
		t.Fatalf("stale epoch attempt=%+v err=%v", attempt, err)
	}
}

func TestProofGateAdmitsAfterTheStopCapabilityIsRestamped(t *testing.T) {
	root, now := proofExtensionGoalFixture(t)
	announceProofFixtureHolder(t, root)
	leasePath := filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json")
	leaseBytes, err := os.ReadFile(leasePath)
	if err != nil {
		t.Fatal(err)
	}
	var current lease.Lease
	if err := json.Unmarshal(leaseBytes, &current); err != nil {
		t.Fatal(err)
	}
	current.ClaimEpoch = 5
	current.Revision++
	leaseBytes, err = json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(leasePath, leaseBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	admission := proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: "standing-validation",
		CapMin: "1", ScopeClass: "full", CommandClass: "testing",
	}
	attempt, _, _, err := admitCandidateProofLaunch(t, admission)
	wantStart := "active coordinator does not own the claimed goal reservation"
	if err == nil || !strings.HasPrefix(err.Error(), wantStart) || !strings.Contains(err.Error(), "lease claim epoch 5") ||
		!strings.Contains(err.Error(), "stop capability claim epoch 1") ||
		!strings.Contains(err.Error(), "metasystem goal restamp --id standing-validation") || attempt.AttemptID != "" {
		t.Fatalf("stale capability refusal: attempt=%+v err=%v", attempt, err)
	}

	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := goal.Restamp(goal.VerbRequest{
		Endpoint: endpoint, Actor: goal.Actor{Machine: "mac-cli", Lineage: "m1"},
		Ulid: "01J5X00000000000000000CP10", Now: now, ClaimEpoch: 5, CallerClass: lease.ClassMain,
		EpochAuthority: goal.EpochAuthorityHolder,
	}, "standing-validation")
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("restamp: %+v %v", result, err)
	}
	attempt, decision, joined, err := admitCandidateProofLaunch(t, admission)
	if err != nil || joined || decision.Disposition != proofrun.DispositionExecuted || attempt.AttemptID == "" {
		t.Fatalf("proof admission after restamp: attempt=%+v decision=%+v joined=%v err=%v", attempt, decision, joined, err)
	}
}

func announceProofFixtureHolder(t *testing.T, root string) {
	t.Helper()
	parent, state, err := (identity.KernelProber{}).Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe proof caller: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "batch-proof-main", parent.Pid, parent.StartedAt.Unix(),
		parent.StartTicks, parent.BootID, "batch-proof-main", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
}

func TestBatchRevisionBoundAdmissionRefusesBeforeRunnerOrCharge(t *testing.T) {
	for _, test := range []struct {
		name                string
		goalRev, accountRev uint64
	}{{"goal moved", 1, 2}, {"accounting moved", 2, 1}} {
		t.Run(test.name, func(t *testing.T) {
			root, now := proofExtensionGoalFixture(t)
			announceProofFixtureHolder(t, root)
			t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
			binding, err := dispatchcore.ResolveGoalBinding(root, "standing-validation", now)
			if err != nil {
				t.Fatal(err)
			}
			before := dispatchcore.ProjectBudget(root, binding.File, now)
			attempt, _, _, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
				ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: "standing-validation",
				CapMin: "1", ScopeClass: "selected", CommandClass: "testing",
				ExpectedGoalRevision: test.goalRev, ExpectedAccountingRevision: test.accountRev,
			})

			after := dispatchcore.ProjectBudget(root, binding.File, now)
			if err == nil || !strings.Contains(err.Error(), "GOAL_REVISION_MOVED") || attempt.AttemptID != "" ||
				after.Attempts != before.Attempts || after.ReservedJobMinutes != before.ReservedJobMinutes {
				t.Fatalf("attempt=%+v err=%v budget=%d/%d -> %d/%d", attempt, err,
					before.Attempts, before.ReservedJobMinutes, after.Attempts, after.ReservedJobMinutes)
			}
		})
	}
}

func TestTestingSelectionExpectedRevisionsArePaired(t *testing.T) {
	root := t.TempDir()
	if _, _, code := parseTestingSelection("test run", []string{"--root", root, "--expected-goal-revision", "2"}, true); code != 2 {
		t.Fatalf("unpaired expected revision exited %d", code)
	}
	request, _, code := parseTestingSelection("test run", []string{"--root", root,
		"--expected-goal-revision", "2", "--expected-accounting-revision", "1"}, true)
	if code != 0 || request.ExpectedGoalRevision != 2 || request.ExpectedAccountingRevision != 1 {
		t.Fatalf("paired expected revisions request=%+v code=%d", request, code)
	}
}

func TestBatchP2RequiresDiagnosticHeadroom(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*goal.GoalFile)
	}{
		{"one attempt remains", func(*goal.GoalFile) {}},
		{"less than twice P2 minutes remain", func(file *goal.GoalFile) {
			file.Budget.AttemptLimit = 10
			file.Budget.ReservedJobMinutesLimit = 1
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, now := proofExtensionGoalFixture(t)
			if test.name != "one attempt remains" {
				amendSyncedGoalFixture(t, root, test.name, test.mutate)
			}
			announceProofFixtureHolder(t, root)
			t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
			attempt, _, _, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
				ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: "standing-validation",
				CapMin: "1", ScopeClass: "selected", CommandClass: "testing",
				ExpectedGoalRevision: 2, ExpectedAccountingRevision: 2,
				RequireDiagnosticHeadroom: true,
			})

			if err == nil || !strings.Contains(err.Error(), "BATCH_MEMBER_BUDGET_REFUSED") || attempt.AttemptID != "" {
				t.Fatalf("headroom attempt=%+v err=%v", attempt, err)
			}
		})
	}
}

func TestProofRunCommandTopLevelRetryAcrossRenamedRoots(t *testing.T) {
	controlRoot := syncedClaimedGoalFixture(t)
	proofFixture := pinProofBinaryFixture(t, controlRoot)
	controlRoot, err := filepath.EvalSymlinks(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	goalPath := filepath.Join(controlRoot, "plans", "goals", "standing-validation.md")
	goalBytes, err := os.ReadFile(goalPath)
	if err != nil {
		t.Fatal(err)
	}
	goalFile, problems := goal.ParseFile(goalBytes)
	if len(problems) != 0 {
		t.Fatalf("parse command canary goal: %v", problems)
	}
	goalFile.StopCapability = &goal.StopCapability{Generation: 2, Revision: goalFile.Claimed.Revision,
		Machine: goalFile.Claimed.Machine, ClaimEpoch: 1}
	goalFile.Budget.ElapsedLimit = "10000h"
	goalFile.Approved.Digest = goal.ApprovalDigest(goalFile.Intent, goalFile.Tier, *goalFile.Budget, goalFile.Risk)
	if err := os.WriteFile(goalPath, goal.RenderFile(goalFile), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, controlRoot, "add", "plans/goals/standing-validation.md")
	goalSyncMutationGit(t, controlRoot, "commit", "-qm", "bind command canary stop capability")
	goalSyncMutationGit(t, controlRoot, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, controlRoot, "update-ref", goal.AcceptedRef, "HEAD")
	makeExecutionRoot := func() string {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		conf, err := os.ReadFile(filepath.Join(controlRoot, "metasystem.conf"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), conf, 0o600); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
			if err := os.WriteFile(filepath.Join(root, "scripts", "agents", name), []byte(`{"floors":{"internal/proofrun":1},"exempt":{}}`), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
	firstRoot, renamedRoot := makeExecutionRoot(), makeExecutionRoot()
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-o", engine, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build command canary engine: %v\n%s", err, output)
	}
	identities := filepath.Join(t.TempDir(), "process-identities.json")
	if err := os.WriteFile(identities, []byte(fmt.Sprintf(`{"%d":{"terminal":true}}`, os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	count := filepath.Join(t.TempDir(), "child-launches")
	body := `count=0; test ! -f "$1" || count=$(cat "$1"); count=$((count+1)); printf '%d\n' "$count" >"$1"; test "$count" -gt 1`
	environment := append(receiptCanaryEnvironment(), "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identities)
	run := func(root, retry, result string) (int, proofrun.LaunchResult, string) {
		args := []string{"proof-run", "launch", "--suite", "command-retry", "--root", root, "--control-root", controlRoot,
			"--goal", "standing-validation", "--cap-min", "1", "--command-class", "command-retry", "--conf", filepath.Join(root, "metasystem.conf"),
			"--progress", result + ".progress.jsonl", "--log", result + ".log",
			"--banner", "command retry canary", "--result", result}
		if retry != "" {
			args = append(args, "--retry-decision", retry)
		}
		args = append(args, "--", "bash", "-c", body, "fixture", count)
		command := proofFixture.command(environment, engine, args...)
		output, err := command.CombinedOutput()
		status := 0
		if exit, ok := err.(*exec.ExitError); ok {
			status = exit.ExitCode()
		} else if err != nil {
			t.Fatalf("launch command failed outside a child status: %v\n%s", err, output)
		}
		var resultRecord proofrun.LaunchResult
		data, readErr := os.ReadFile(result)
		if readErr != nil || json.Unmarshal(data, &resultRecord) != nil {
			t.Fatalf("read launch result: err=%v bytes=%s", readErr, data)
		}
		return status, resultRecord, string(output)
	}
	firstResult := filepath.Join(t.TempDir(), "first.json")
	status, first, output := run(firstRoot, "", firstResult)
	if status != 1 || first.Disposition != proofrun.DispositionFailed || first.AttemptID == "" {
		t.Fatalf("first diagnosed failure status=%d result=%+v output=%s", status, first, output)
	}
	secondResult := filepath.Join(t.TempDir(), "second.json")
	status, second, _ := run(renamedRoot, "", secondResult)
	if status != proofrun.ExitRetryRequired || second.Disposition != proofrun.DispositionRetryRequired || second.PriorAttempt != first.AttemptID {
		t.Fatalf("undiagnosed repeat status=%d result=%+v", status, second)
	}
	evidence := filepath.Join(t.TempDir(), "failure.log")
	if err := os.WriteFile(evidence, []byte("controlled first child failure\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	retryPath := filepath.Join(t.TempDir(), "retry.json")
	retryBytes, _ := json.Marshal(proofrun.RetryDecision{SchemaVersion: 1, PriorAttempt: first.AttemptID,
		Cause: "controlled child refusal", EvidencePath: evidence, Rationale: "the helper succeeds on its diagnosed second execution"})
	if err := os.WriteFile(retryPath, retryBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	thirdResult := filepath.Join(t.TempDir(), "third.json")
	status, third, output := run(renamedRoot, retryPath, thirdResult)
	if status != 0 || third.Disposition != proofrun.DispositionExecuted || third.PriorAttempt != first.AttemptID {
		t.Fatalf("diagnosed retry status=%d result=%+v output=%s", status, third, output)
	}
	launches, err := os.ReadFile(count)
	if err != nil || strings.TrimSpace(string(launches)) != "2" {
		t.Fatalf("no-child decision launched work or retry did not execute: launches=%q err=%v", launches, err)
	}
	attempts, err := proofrun.ReadAttempts(controlRoot)
	if err != nil || len(attempts) != 2 || attempts[1].Retry == nil || attempts[1].PreviousAttempt != first.AttemptID ||
		attempts[0].ExecutionRoot == attempts[1].ExecutionRoot {
		t.Fatalf("command accounting after renamed-root retry: attempts=%+v err=%v", attempts, err)
	}
}

func TestProofRunCommandGovernedParentSharesOneCharge(t *testing.T) {
	if os.Getenv("GO_WANT_GOVERNED_COMMAND_PARENT") == "1" {
		ready := os.NewFile(3, "governed-command-ready")
		if ready == nil {
			os.Exit(97)
		}
		if _, err := fmt.Fprintln(ready, "ready"); err != nil || ready.Close() != nil {
			os.Exit(97)
		}
		release := os.NewFile(4, "governed-command-release")
		if release == nil {
			os.Exit(97)
		}
		var signal [1]byte
		if _, err := io.ReadFull(release, signal[:]); err != nil {
			os.Exit(97)
		}
		_ = release.Close()
		root := os.Getenv("GOVERNED_COMMAND_ROOT")
		proofFixture := pinProofBinaryFixture(t, root)
		command := proofFixture.command(os.Environ(), os.Getenv("GOVERNED_COMMAND_ENGINE"), "proof-run", "launch",
			"--suite", "governed-command", "--root", root, "--control-root", root,
			"--conf", filepath.Join(root, "metasystem.conf"), "--progress", filepath.Join(root, "artifacts", "governed.progress.jsonl"),
			"--log", filepath.Join(root, "artifacts", "governed.log"), "--banner", "governed command canary", "--", "true")
		// The governed locators travel under the fixture's own names: the
		// package's TestMain clears every METASYSTEM_PROOF_* control before
		// a test runs, so the engine receives them here, not by inheritance.
		command.Env = append(command.Env, "METASYSTEM_PROOF_RUN_ROOT="+os.Getenv("GOVERNED_COMMAND_PROOF_RUN_ROOT"),
			"METASYSTEM_PROOF_RUN_ID="+os.Getenv("GOVERNED_COMMAND_PROOF_RUN_ID"))
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				os.Exit(exit.ExitCode())
			}
			os.Exit(97)
		}
		os.Exit(0)
	}
	root := syncedClaimedGoalFixture(t)
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	goalPath := filepath.Join(root, "plans", "goals", "standing-validation.md")
	goalBytes, err := os.ReadFile(goalPath)
	if err != nil {
		t.Fatal(err)
	}
	governedGoal, problems := goal.ParseFile(goalBytes)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	governedGoal.StopCapability = &goal.StopCapability{Generation: 2, Revision: governedGoal.Claimed.Revision,
		Machine: governedGoal.Claimed.Machine, ClaimEpoch: 1}
	governedGoal.Budget.ElapsedLimit = "10000h"
	governedGoal.Approved.Digest = goal.ApprovalDigest(governedGoal.Intent, governedGoal.Tier, *governedGoal.Budget, governedGoal.Risk)
	if err := os.WriteFile(goalPath, goal.RenderFile(governedGoal), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "plans/goals/standing-validation.md")
	goalSyncMutationGit(t, root, "commit", "-qm", "bind governed command authority")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		path := filepath.Join(root, "scripts", "agents", name)
		if err := os.WriteFile(path, []byte(`{"floors":{"internal/proofrun":1},"exempt":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-o", engine, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build governed canary engine: %v\n%s", err, output)
	}
	now := time.Now().UTC()
	weight := uint64(0)
	store := &runpkg.Store{Root: root, Now: func() time.Time { return now }}
	store.AdmitGoverned = func(runpkg.GovernedAdmissionRequest) (runpkg.GovernedAdmissionResult, error) {
		return runpkg.GovernedAdmissionResult{Attempt: runpkg.GovernedAttempt{GoalRevision: 2, ObligationRevision: 7,
			WeightGeneration: &weight, Recurrence: governance.StandingSharedProcess, ExecutionCostMinutes: 2, AttemptOrdinal: 1,
			Budget:          goalbudget.Budget{ElapsedLimit: "10000h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2},
			BudgetStartedAt: now.Add(-time.Hour).Format(time.RFC3339), CorrelationPolicy: "exact-run-generation",
			ExpectedAssumptions: governance.ObligationAssumptions{Recurrence: governance.StandingSharedProcess,
				Platform: "fixture/os", ToolchainIdentity: "fixture-go", SurfaceDigest: "fixture-surface", MaxActiveJobs: 1,
				TimingEnvelopeSeconds: 120, ObservationSource: "run-terminal-record"},
			AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: runpkg.BreakerClosed}}, nil
	}
	nonce, err := store.Launch(runpkg.Caller{Class: "MAIN", MainId: "main-fixture", OwnerLineage: "main-fixture"}, runpkg.LaunchParams{
		Id: "governed-command", Kind: "suite", Display: "governed command owner", Log: "artifacts/governed-parent.log",
		GoalId: "standing-validation", ObligationRevision: 7, StandingShared: true,
		Expect: runpkg.Expect{Green: "green", Red: "red", Hung: "hung", Unknown: "unknown"}})
	if err != nil {
		t.Fatal(err)
	}
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		_ = releaseRead.Close()
		_ = releaseWrite.Close()
	})
	child := exec.Command(os.Args[0], "-test.run=^TestProofRunCommandGovernedParentSharesOneCharge$")
	child.Env = append(receiptCanaryEnvironment(), "GO_WANT_GOVERNED_COMMAND_PARENT=1", "GOVERNED_COMMAND_ROOT="+root, "GOVERNED_COMMAND_ENGINE="+engine,
		"GOVERNED_COMMAND_PROOF_RUN_ROOT="+root, "GOVERNED_COMMAND_PROOF_RUN_ID=governed-command")
	child.ExtraFiles = []*os.File{readyWrite, releaseRead}
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	child.Stdout, child.Stderr = &output, &output
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	_ = readyWrite.Close()
	_ = releaseRead.Close()
	finished := false
	t.Cleanup(func() {
		if !finished {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	line, err := bufio.NewReader(readyRead).ReadString('\n')
	if err != nil || line != "ready\n" {
		t.Fatalf("governed parent readiness = %q, %v: %s", line, err, output.String())
	}
	_ = readyRead.Close()
	pgid, err := syscall.Getpgid(child.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind("governed-command", nonce, int64(child.Process.Pid), int64(pgid)); err != nil {
		t.Fatal(err)
	}
	if _, err := releaseWrite.Write([]byte{'x'}); err != nil {
		t.Fatal(err)
	}
	_ = releaseWrite.Close()
	if err := child.Wait(); err != nil {
		t.Fatalf("governed command failed: %v\n%s", err, output.String())
	}
	finished = true
	attempts, err := proofrun.ReadAttempts(root)
	if err != nil || len(attempts) != 1 || attempts[0].ReservationOwner == nil || attempts[0].ReservationOwner.RunID != "governed-command" ||
		attempts[0].Terminal == nil || attempts[0].Terminal.Result != proofrun.TerminalSuccess {
		t.Fatalf("governed command attempt=%+v err=%v output=%s", attempts, err, output.String())
	}
	goalBytes, err = os.ReadFile(filepath.Join(root, "plans", "goals", "standing-validation.md"))
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(goalBytes)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	projection := dispatchcore.ProjectBudget(root, file, now.Add(time.Second))
	if projection.Status != dispatchcore.BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 2 {
		t.Fatalf("governed parent and proof were not one accounting charge: %+v", projection)
	}
}

// terminalCommitFixture reserves an attempt on a claimed goal that carries
// stop capability, so the commit's authority checks pass and the scripted
// lock seam alone decides the outcome. The launch it returns runs a command
// under a watchdog stub and commits through the given terminal commit. The
// fixture's 2026-08-30 goal dates are inert here: the commit's binding reads
// committed fields only, and nothing on this path reads METASYSTEM_GOAL_NOW.
func terminalCommitFixture(t *testing.T) (string, proofrun.Attempt, func([]string, func(proofrun.CompletionContext, json.RawMessage) error) (int, string)) {
	t.Helper()
	root, _ := proofExtensionGoalFixture(t)
	proofIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "terminal-commit", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root,
		GoalID: "standing-validation", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 30,
		Identity: proofIdentity, Launcher: launcher, Now: time.Now().UTC()}))

	if err != nil || decision.Disposition != proofrun.DispositionExecuted {
		t.Fatalf("reserve = %+v, %+v, %v", attempt, decision, err)
	}
	deadline, err := time.Parse(time.RFC3339Nano, attempt.Deadline)
	if err != nil {
		t.Fatal(err)
	}
	artifactDir := filepath.Join(root, "artifacts", "terminal-commit")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatal(err)
	}
	watchdog := filepath.Join(artifactDir, "watchdog.sh")
	if err := os.WriteFile(watchdog, []byte(`#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
while [[ ! -e "$done_path" ]]; do sleep 0.005; done
`), 0o755); err != nil {
		t.Fatal(err)
	}
	launch := func(command []string, commit func(proofrun.CompletionContext, json.RawMessage) error) (int, string) {
		var launcherErrors bytes.Buffer
		result := proofrun.LaunchSuite(proofrun.LaunchOptions{Suite: "terminal-commit", Root: root, ControlRoot: root, ErrorOutput: &launcherErrors,
			AttemptID: attempt.AttemptID, Deadline: deadline, ConfPath: filepath.Join(root, "metasystem.conf"),
			ProgressPath: filepath.Join(artifactDir, "progress.jsonl"), LogPath: filepath.Join(artifactDir, "proof.log"),
			Banner: "terminal commit fixture", Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second,
			EvidenceMax: 1024, Poll: 5 * time.Millisecond, TermGrace: time.Second, KillGrace: time.Second,
			WatchdogExecutable: watchdog, Command: command,
			PrepareSuccess: func(proofrun.CompletionContext) (json.RawMessage, error) {
				return json.RawMessage(`{"prepared":true}`), nil
			}, CommitTerminal: commit})
		return result, launcherErrors.String()
	}
	return root, attempt, launch
}

// scriptTerminalLocks installs a lock seam for one test; an unset acquirer
// keeps the real one, the pause records its durations instead of sleeping,
// and the notes written without a launcher stream are captured.
func scriptTerminalLocks(t *testing.T, seam terminalLockSeam) (notes *bytes.Buffer, pauses *[]time.Duration) {
	t.Helper()
	previous := terminalLocks
	if seam.stopFence == nil {
		seam.stopFence = previous.stopFence
	}
	if seam.goalRevision == nil {
		seam.goalRevision = previous.goalRevision
	}
	notes = &bytes.Buffer{}
	pauses = &[]time.Duration{}
	seam.notes = notes
	seam.pause = func(d time.Duration) { *pauses = append(*pauses, d) }
	terminalLocks = seam
	t.Cleanup(func() { terminalLocks = previous })
	return notes, pauses
}

func TestCommitProofTerminalTriesARefusedLockAgainAndNamesTheHolder(t *testing.T) {
	root, attempt, launch := terminalCommitFixture(t)
	fenceCalls, goalCalls := 0, 0
	notes, pauses := scriptTerminalLocks(t, terminalLockSeam{
		stopFence: func(root, verb string, ref identity.Ref, scaleMilli int) (*lock.Lock, error) {
			fenceCalls++
			if fenceCalls <= 2 {
				return nil, &lock.HolderError{Path: "fence", Holder: lock.Identity{Pid: 4242, PidStartedAt: 7}, State: lock.Alive}
			}
			return stopfence.Acquire(root, verb, ref, scaleMilli)
		},
		goalRevision: func(root, goalID string, revision uint64, tag string) (*goalrevision.Held, error) {
			goalCalls++
			if goalCalls == 1 {
				return nil, &goalrevision.Busy{Key: goalID + "/r2", Holder: "pid=4343,tag=goal-resume"}
			}
			return goalrevision.Acquire(root, goalID, revision, tag)
		},
	})
	result, launcherErrors := launch([]string{"true"}, commitProofTerminal)
	if result != 0 {
		t.Fatalf("a commit whose locks were refused and then granted did not succeed: result %d\n%s", result, launcherErrors)
	}
	stored, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalSuccess {
		t.Fatalf("the green proof was not committed: attempt=%+v err=%v", stored, err)
	}
	if fenceCalls != 4 || goalCalls != 2 {
		t.Fatalf("the commit did not try the locks again from the top: fence tries %d, goal-revision tries %d", fenceCalls, goalCalls)
	}
	// The notes go to the launcher's error stream, which the launcher tees
	// into launcher.log, the record the evidence is mined from.
	for _, want := range []string{
		"try 1 of 6 waits behind lock fence is held by pid 4242 (started 7) alive",
		"try 2 of 6 waits behind lock fence is held by pid 4242",
		"try 3 of 6 waits behind LOCK_BUSY rank=goal-revision key=standing-validation/r2 holder=pid=4343,tag=goal-resume",
	} {
		if !strings.Contains(launcherErrors, want) {
			t.Fatalf("the refused try did not name its holder on the launcher's stream: want %q in\n%s", want, launcherErrors)
		}
	}
	if strings.Count(launcherErrors, "waits behind") != 3 || notes.Len() != 0 {
		t.Fatalf("a granted try was noted as refused, or a note bypassed the launcher's stream:\n%s\n%s", launcherErrors, notes.String())
	}
	// Every refused try is followed by one pause of the named length, so a
	// waiter polling the released fence can see it free.
	if len(*pauses) != 3 || (*pauses)[0] != terminalCommitPause || (*pauses)[2] != terminalCommitPause {
		t.Fatalf("the tries were not paused apart: %v", *pauses)
	}
	// The stop fence taken on the refused goal-revision try was released
	// before the next try (the real acquisition on try 4 succeeded within
	// its bound), and nothing is left held after the commit.
	if _, err := os.Stat(stopfence.LockPath(root)); !os.IsNotExist(err) {
		t.Fatalf("the stop fence is still held after the commit: %v", err)
	}
}

func TestCommitProofTerminalGivesUpAfterNamedTriesWithTheHolderInTheRecord(t *testing.T) {
	root, attempt, launch := terminalCommitFixture(t)
	calls := 0
	// The shape the lock produces for an unreadable owner file: no holder
	// identity, unproven liveness, the read error as the cause.
	_, pauses := scriptTerminalLocks(t, terminalLockSeam{
		stopFence: func(string, string, identity.Ref, int) (*lock.Lock, error) {
			calls++
			return nil, &lock.HolderError{Path: "fence", Holder: lock.Identity{}, State: lock.Unknown,
				Cause: errors.New("owner.json: permission denied")}
		},
	})
	result, launcherErrors := launch([]string{"true"}, commitProofTerminal)
	if result == 0 || !strings.Contains(launcherErrors,
		"proof terminal commit refused 6 times; the last holder: lock fence is held by pid 0 (started 0) of unproven liveness (uninspectable is alive); owner file: owner.json: permission denied") {
		t.Fatalf("the last refusal was not reported by name with its cause: result %d\n%s", result, launcherErrors)
	}
	if calls != terminalCommitTries || strings.Count(launcherErrors, "waits behind") != terminalCommitTries-1 || len(*pauses) != terminalCommitTries-1 {
		t.Fatalf("the commit did not try the named number of times a pause apart: %d tries, %d pauses\n%s", calls, len(*pauses), launcherErrors)
	}
	stored, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalUnknown || stored.Terminal.ExitStatus != 1 ||
		!strings.Contains(stored.Terminal.Reason, "refused 6 times") || !strings.Contains(stored.Terminal.Reason, "owner file: owner.json: permission denied") {
		t.Fatalf("the refused attempt was not retained with the holder named: attempt=%+v err=%v", stored, err)
	}

	// An attempt a stop batch asked to cancel while the commit was being
	// refused is retained as cancelled, as the commit itself would have
	// recorded it. The intent lands during the first refused try, after the
	// launch (which refuses an attempt already cancelled) and before the
	// retained terminal.
	root, attempt, launch = terminalCommitFixture(t)
	cancelledRoot, cancelledAttempt := root, attempt.AttemptID
	scriptTerminalLocks(t, terminalLockSeam{
		stopFence: func(string, string, identity.Ref, int) (*lock.Lock, error) {
			if err := proofrun.RequestCancellation(cancelledRoot, cancelledAttempt, "stop batch"); err != nil {
				t.Error(err)
			}
			return nil, &lock.HolderError{Path: "fence", Holder: lock.Identity{Pid: 4242, PidStartedAt: 7}, State: lock.Alive}
		},
	})
	if result, _ := launch([]string{"true"}, commitProofTerminal); result == 0 {
		t.Fatal("a refused commit on a cancelled attempt reported success")
	}
	stored, err = proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalCancelled || !strings.Contains(stored.Terminal.Reason, "refused 6 times") {
		t.Fatalf("the refused cancelled attempt was not retained as cancelled: attempt=%+v err=%v", stored, err)
	}
}

func TestATestingWorkerThatWroteNoResultEndsItsAttemptFailedWithTheFileNamed(t *testing.T) {
	root, attempt, launch := terminalCommitFixture(t)
	missing := filepath.Join(root, "artifacts", "terminal-commit", "worker-result.json")
	var retained *proofrun.TestResult
	result, launcherErrors := launch([]string{"false"}, testingTerminalCommit(missing, &retained))
	if result != 1 || strings.Contains(launcherErrors, "commit terminal proof result") {
		t.Fatalf("a failed worker without a result did not commit its terminal: result %d\n%s", result, launcherErrors)
	}
	stored, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalFailed || stored.Terminal.ExitStatus != 1 ||
		!strings.Contains(stored.Terminal.Reason, "the worker left no usable result: open "+missing) || stored.TestResult != nil || retained != nil {
		t.Fatalf("the missing result was not named on a failed terminal: attempt=%+v err=%v", stored, err)
	}

	// A success without its result is a contradiction and is refused. In a
	// real launch PrepareSuccess reads the result first and a missing one
	// already fails the exit; this fixture's PrepareSuccess returns a canned
	// payload, which is what reaches the commit's own guard.
	root, attempt, launch = terminalCommitFixture(t)
	missing = filepath.Join(root, "artifacts", "terminal-commit", "worker-result.json")
	result, launcherErrors = launch([]string{"true"}, testingTerminalCommit(missing, &retained))
	if result == 0 || !strings.Contains(launcherErrors, "commit terminal proof result: open "+missing) {
		t.Fatalf("a success without its result was committed: result %d\n%s", result, launcherErrors)
	}
	stored, err = proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || stored.Terminal != nil {
		t.Fatalf("a refused success commit still wrote a terminal: attempt=%+v err=%v", stored, err)
	}
}
