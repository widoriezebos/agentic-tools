package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestCoverageReuseVerbRefusesChangedParentProjectInput(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	root := filepath.Join(project, "tools", "metasystem")
	writeTestingFixtureFile(t, filepath.Join(project, ".gitattributes"), []byte("testing.json merge=metasystem-testing\n"), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\ndispatch.cap-max=120\n"), 0o600)
	writeTestingFixtureFile(t, filepath.Join(root, "internal", "proofrun", "stub.go"), []byte("package proofrun\n"), 0o600)
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		writeTestingFixtureFile(t, filepath.Join(root, "scripts", "agents", name), []byte(`{"floors":{"internal/proofrun":1},"exempt":{}}`), 0o600)
	}
	testingFixtureGit(t, project, "init", "-q", "-b", "main")
	testingFixtureGit(t, project, "add", ".")
	testingFixtureGit(t, project, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-qm", "base")
	root, err := canonicalPath(root)
	if err != nil {
		t.Fatal(err)
	}
	project, err = canonicalPath(project)
	if err != nil {
		t.Fatal(err)
	}
	proofIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"),
		"full", "coverage-reuse-public", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	request := candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: root,
		ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2,
		ReservedMinutes: 5, Identity: proofIdentity, Launcher: launcher, Now: now})
	request = proofrun.WithTestHostAdmissionDirectory(request, filepath.Join(t.TempDir(), "host-admission"))
	attempt, decision, err := proofrun.ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	baselineName := "coverage-ratchet.json"
	if runtime.GOOS == "linux" {
		baselineName = "coverage-ratchet-linux.json"
	}
	baseline := filepath.Join(root, "scripts", "agents", baselineName)
	begin := proofrun.CoverageBeginOptions{ControlRoot: root, ExecutionRoot: root, AttemptID: attempt.AttemptID,
		BaselinePath: baseline, ProducerClass: "full", ProducerPID: int64(os.Getpid()), CallerPID: int64(os.Getpid())}
	if err := proofrun.BeginCoverage(begin); err != nil {
		t.Fatal(err)
	}
	evidenceRoot := t.TempDir()
	coverageLog, packages := filepath.Join(evidenceRoot, "coverage.log"), filepath.Join(evidenceRoot, "packages.txt")
	const module = "example.invalid/metasystem/"
	writeTestingFixtureFile(t, coverageLog, []byte("ok  "+module+"internal/proofrun 0.1s coverage: 85.0% of statements\n"), 0o600)
	writeTestingFixtureFile(t, packages, []byte(module+"internal/proofrun\n"), 0o600)
	if _, err := proofrun.CompleteCoverage(proofrun.CoverageCompleteOptions{CoverageBeginOptions: begin,
		CoverageLog: coverageLog, PackageInventory: packages, ModulePrefix: module}); err != nil {
		t.Fatal(err)
	}
	if _, err := proofrun.FinalizeAttempt(root, attempt.AttemptID, proofrun.TerminalSuccess, 0,
		"green", []byte(`{"receipt":true}`), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	args := []string{"--root", root, "--control-root", root, "--baseline", baseline, "--package", "internal/proofrun"}
	overlapOutput := newFreezeOverlapWriter()
	type freezeResult struct {
		code   int
		stderr string
	}
	freezeDone := make(chan freezeResult, 1)
	go func() {
		var stderr bytes.Buffer
		freezeDone <- freezeResult{runGateWitnessFreezeWithWriters([]string{"--root", root}, overlapOutput, &stderr), stderr.String()}
	}()
	select {
	case <-overlapOutput.started:
	case result := <-freezeDone:
		t.Fatalf("freeze CLI exited before controlled output overlap: code=%d stderr=%q", result.code, result.stderr)
	}
	coverageCode := runProofRunCoverageReuse(args)
	close(overlapOutput.release)
	result := <-freezeDone
	fields := strings.Split(strings.TrimSpace(overlapOutput.String()), "\t")
	if result.code != 0 || result.stderr != "" || len(fields) != 3 || strings.Contains(overlapOutput.String(), "coverage reuse:") {
		t.Fatalf("overlapped freeze CLI code=%d stdout=%q stderr=%q", result.code, overlapOutput.String(), result.stderr)
	}
	var cleanupOut, cleanupErr bytes.Buffer
	if code := runGateWitnessFreezeWithWriters([]string{"--cleanup", fields[2]}, &cleanupOut, &cleanupErr); code != 0 || cleanupOut.Len() != 0 || cleanupErr.Len() != 0 {
		t.Fatalf("overlapped freeze cleanup code=%d stdout=%q stderr=%q", code, cleanupOut.String(), cleanupErr.String())
	}
	if coverageCode != 0 {
		t.Fatalf("coverage-reuse before parent mutation exited %d", coverageCode)
	}
	writeTestingFixtureFile(t, filepath.Join(project, ".gitattributes"), []byte("changed parent input\n"), 0o644)
	if code := runProofRunCoverageReuse(args); code != 3 {
		t.Fatalf("coverage-reuse after parent mutation exited %d, want stale-proof status 3", code)
	}
}

func candidateProofAdmission(request proofrun.AdmissionRequest) proofrun.AdmissionRequest {
	request.CandidateGoalID = request.GoalID
	request.CandidateRevision = request.AccountingRevision
	request.CandidateBudgetEpoch = request.BudgetEpoch
	if request.Identity.CommandClass == "testing" && request.CandidateTree == "" {
		request.CandidateTree = strings.Repeat("b", 40)
	}
	return privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(request, "0"))
}

func requireProofReservationNotAdmissionRefused(t *testing.T, decision proofrun.LaunchResult) {
	t.Helper()
	if decision.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("proof reservation fixture was admission-refused: %+v", decision)
	}
}

func privateProofAdmissionRequest(request proofrun.AdmissionRequest) proofrun.AdmissionRequest {
	if os.Getenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR") == "" && fixtureauth.FixtureModeRoot(request.ControlRoot) {
		return proofrun.WithTestHostAdmissionDirectory(request,
			filepath.Join(request.ControlRoot, "artifacts", "agents", "host-admission-fixture"))
	}
	return request
}

func candidateProofLaunchAdmission(request proofLaunchAdmission) proofLaunchAdmission {
	if request.CommandClass == "testing" && request.CandidateTree == "" {
		request.CandidateTree = strings.Repeat("b", 40)
	}
	if request.BeforePublish == nil {
		request.BeforePublish = func(reservation *proofrun.AdmissionRequest) {
			*reservation = privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(*reservation, "0"))
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
		*reservation = privateProofAdmissionRequest(*reservation)
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
			attempt, result, noChild, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
				ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"),
				GoalID: candidate.Id, AuthorityGoalID: "standing-validation", CapMin: "1", ScopeClass: "full", CommandClass: "testing",
			})
			if test.after {
				if err != nil || noChild || result.Disposition != proofrun.DispositionExecuted || attempt.AttemptID == "" {
					t.Fatalf("admission before later rebudget: attempt=%+v result=%+v noChild=%t err=%v", attempt, result, noChild, err)
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
	if err := testexec.WriteFile(script, []byte("#!/usr/bin/env bash\n[[ \"$1\" == --witness-check-only && \"$METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE\" == ENGINE && \"$METASYSTEM_GATE_WITNESS\" == usable ]]\n"), 0o700); err != nil {
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

func workerAuthorizedAttemptFixture(t *testing.T, sectionWorktree ...bool) (string, string, proofrun.Attempt) {
	t.Helper()
	controlRoot, _ := proofExtensionGoalFixture(t)
	controlRoot, err := canonicalProofRoot(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	executionRoot := t.TempDir()
	candidateTree := ""
	commandClass := "worker-root"
	if len(sectionWorktree) != 0 && sectionWorktree[0] {
		executionRoot = controlRoot
		commandClass = "testing"
		// The production schema-1 worker admits the repository top while a
		// selected native group runs from an application subtree.
		application := filepath.Join(controlRoot, "application")
		if err := os.MkdirAll(application, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(application, "tracked.txt"), []byte("candidate\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit(t, controlRoot, "add", "application/tracked.txt")
		runGit(t, controlRoot, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "application subtree")
		candidateTree, err = (gittree.Workspace{Dir: controlRoot}).HeadTree()
		if err != nil {
			t.Fatal(err)
		}
	}
	proofIdentity, err := proofrun.BuildProofIdentity(controlRoot, filepath.Join(controlRoot, "metasystem.conf"), "full", commandClass, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.ProcessIdentityForPID(int64(os.Getppid()), nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt, decision, err := proofrun.ReserveLocked(privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(proofrun.AdmissionRequest{
		ControlRoot: controlRoot, ExecutionRoot: executionRoot, CandidateTree: candidateTree, GoalID: "standing-validation", GoalRevision: 2,
		AccountingRevision: 2, CandidateGoalID: "standing-validation", CandidateRevision: 2,
		ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher,
		Now: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC),
	}, "0")))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Disposition == proofrun.DispositionAdmissionRefused {
		t.Fatalf("worker-authorized attempt fixture was admission-refused: %+v", decision)
	}
	if len(sectionWorktree) != 0 && sectionWorktree[0] {
		// The installed generation-949 worker retained schema 3 during Stage A.
		// Keep this authority canary on that exact compatible reader surface.
		attempt.SchemaVersion = proofrun.CandidateAttemptSchemaVersion
		path, pathErr := proofrun.AttemptPath(controlRoot, attempt.AttemptID)
		encoded, encodeErr := json.Marshal(attempt)
		if pathErr != nil || encodeErr != nil {
			t.Fatalf("schema-3 attempt fixture: path=%v encode=%v", pathErr, encodeErr)
		}
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := proofrun.ReadAttempt(controlRoot, attempt.AttemptID); err != nil {
			t.Fatalf("schema-3 attempt fixture invalid: %v", err)
		}
	}
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_CONTROL_ROOT", controlRoot)
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_ATTEMPT", attempt.AttemptID)
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_RECORD_KEY", "")
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_CREATION_CLAIM", "")
	setOwnedGoGateProcessEnvironment(t, proofWitnessExecutionRootEnv, "")
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_GATE_WITNESS_WRITE", "")
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

func TestGoGateTestsRefusesUnauthenticatedInvocationBeforeNativeLaunch(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	root := t.TempDir()
	logRoot := filepath.Join(root, "native-logs")
	for _, name := range []string{"METASYSTEM_PROOF_CONTROL_ROOT", "METASYSTEM_PROOF_ATTEMPT", "METASYSTEM_PROOF_RECORD_KEY", "METASYSTEM_PROOF_CREATION_CLAIM", proofrun.TestWorkersEnvironment} {
		setOwnedGoGateProcessEnvironment(t, name, "")
	}
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runProofRunGoGateTests([]string{"--root", root, "--log-root", logRoot, "--workers", "1"})
	})
	if code != 3 || !strings.Contains(stderr, "no proof control root") {
		t.Fatalf("unauthenticated native gate refusal: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(logRoot); !os.IsNotExist(err) {
		t.Fatalf("unauthenticated invocation reached native launch: %v", err)
	}
}

func TestGoGateTestsRefusesWorkerRequestAboveInheritedAllowanceBeforeNativeLaunch(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	_, root, _ := workerAuthorizedAttemptFixture(t)
	setOwnedGoGateProcessEnvironment(t, proofrun.TestWorkersEnvironment, "1")
	logRoot := filepath.Join(root, "native-logs")
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runProofRunGoGateTests([]string{"--root", root, "--log-root", logRoot, "--workers", "8"})
	})
	if code != 3 || !strings.Contains(stderr, "requested workers 8 exceed inherited allowance 1") {
		t.Fatalf("inherited worker ceiling refusal: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(logRoot); !os.IsNotExist(err) {
		t.Fatalf("over-ceiling invocation reached native launch: %v", err)
	}
}

func TestGoGateTestsRefusesForeignRootBeforeNativeLaunch(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	foreign, admitted, _ := workerAuthorizedAttemptFixture(t)
	setOwnedGoGateProcessEnvironment(t, proofrun.TestWorkersEnvironment, "1")
	logRoot := filepath.Join(foreign, "native-logs")
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runProofRunGoGateTests([]string{"--root", foreign, "--log-root", logRoot, "--workers", "1"})
	})
	if code != 3 || !strings.Contains(stderr, foreign) || !strings.Contains(stderr, admitted) {
		t.Fatalf("foreign native root refusal: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(logRoot); !os.IsNotExist(err) {
		t.Fatalf("foreign-root invocation reached native launch: %v", err)
	}
}

func TestGoGateTestsAuthenticatedCancellationDrainsNativeChildAndBorrowedLease(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	controlRoot, root, attempt := workerAuthorizedAttemptFixture(t)
	for path, source := range map[string]string{
		"go.mod":              "module example.invalid/publicgate\n\ngo 1.27\n",
		"internal/app/app.go": "package app\n",
		"internal/app/app_test.go": `package app
import (
 "os"
 "strconv"
 "testing"
 "time"
)
func TestHeldUntilCancellation(t *testing.T) {
 if err := os.WriteFile(os.Getenv("NATIVE_PID"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil { t.Fatal(err) }
 if err := os.WriteFile(os.Getenv("NATIVE_READY"), []byte("ready"), 0600); err != nil { t.Fatal(err) }
 for { time.Sleep(time.Hour) }
}
`,
		"cmd/tool/main.go":      "package main\nfunc main() {}\n",
		"cmd/tool/main_test.go": "package main\nimport \"testing\"\nfunc TestCommand(t *testing.T) {}\n",
	} {
		writeReceiptFixture(t, root, path, source)
	}
	admissionDir := filepath.Join(t.TempDir(), "host-admission")
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_ADMISSION_TEST_DIR", admissionDir)
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", controlRoot)
	control, attemptID := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT")
	if err := os.Unsetenv("METASYSTEM_PROOF_CONTROL_ROOT"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("METASYSTEM_PROOF_ATTEMPT"); err != nil {
		t.Fatal(err)
	}
	hostProbeResource := []string{"go-gate-public-cancellation"}
	lease, err := proofrun.AcquireHostResources(t.Context(), controlRoot, filepath.Join(controlRoot, "metasystem.conf"), "heavy", hostProbeResource)
	if err != nil {
		t.Fatal(err)
	}
	leaseOpen := true
	defer func() {
		if leaseOpen {
			_ = lease.Close()
		}
	}()
	if err := os.Setenv("METASYSTEM_PROOF_CONTROL_ROOT", control); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("METASYSTEM_PROOF_ATTEMPT", attemptID); err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", engine, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build public native runner: %v\n%s", err, output)
	}
	ready, nativePIDPath := filepath.Join(t.TempDir(), "native.ready"), filepath.Join(t.TempDir(), "native.pid")
	logRoot := filepath.Join(t.TempDir(), "native-logs")
	environment := append(os.Environ(), proofrun.HostResourceFDEnvironment(lease.Files()), proofrun.TestWorkersEnvironment+"=1",
		"GOFLAGS=-buildvcs=false", "NATIVE_READY="+ready, "NATIVE_PID="+nativePIDPath)
	command := pinProofBinaryFixture(t, controlRoot).command(environment, engine, "proof-run", "go-gate-tests", "--root", root, "--log-root", logRoot, "--workers", "1")
	command.Dir = root
	command.ExtraFiles = append(command.ExtraFiles, lease.Files()...)
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	exited := make(chan error, 1)
	go func() {
		err := command.Wait()
		waited <- err
		exited <- err
	}()
	joined := false
	cancelAndJoin := func() error {
		if joined {
			return nil
		}
		if command.Process != nil {
			_ = command.Process.Signal(syscall.SIGTERM)
		}
		err := <-waited
		joined = true
		return err
	}
	t.Cleanup(func() { _ = cancelAndJoin() })
	waitForPublicRouteFileOrExit(t, ready, exited, func() string { return output.String() })
	nativePIDBytes, err := os.ReadFile(nativePIDPath)
	if err != nil {
		t.Fatal(err)
	}
	nativePID, err := strconv.Atoi(strings.TrimSpace(string(nativePIDBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	waitErr := <-waited
	joined = true
	if exit, ok := waitErr.(*exec.ExitError); !ok || exit.ExitCode() == 0 {
		t.Fatalf("cancelled public native runner exit=%v\n%s", waitErr, output.String())
	}
	if err := syscall.Kill(nativePID, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("public runner returned before exact native child %d drained: %v\n%s", nativePID, err, output.String())
	}
	if attempt.AttemptID == "" {
		t.Fatal("authenticated cancellation fixture has no admitted attempt")
	}
	if err := os.Unsetenv("METASYSTEM_PROOF_CONTROL_ROOT"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv("METASYSTEM_PROOF_ATTEMPT"); err != nil {
		t.Fatal(err)
	}
	type probeResult struct {
		lease *proofrun.HostResourceLease
		err   error
	}
	probeContext, cancelProbe := context.WithCancel(t.Context())
	queued := make(chan struct{})
	var queuedOnce sync.Once
	probeContext = proofrun.WithHostResourceWaitObserver(probeContext, func() { queuedOnce.Do(func() { close(queued) }) })
	probeDone := make(chan probeResult, 1)
	go func() {
		probe, probeErr := proofrun.AcquireHostResources(probeContext, controlRoot, filepath.Join(controlRoot, "metasystem.conf"), "heavy", hostProbeResource)
		probeDone <- probeResult{lease: probe, err: probeErr}
	}()
	probeJoined := false
	t.Cleanup(func() {
		cancelProbe()
		if !probeJoined {
			result := <-probeDone
			if result.lease != nil {
				_ = result.lease.Close()
			}
		}
	})
	select {
	case <-queued:
	case early := <-probeDone:
		probeJoined = true
		if early.lease != nil {
			_ = early.lease.Close()
		}
		t.Fatalf("host-capacity probe did not queue behind the held slot: %v", early.err)
	case <-t.Context().Done():
		t.Fatalf("host-capacity queue observation was cancelled: %v", context.Cause(t.Context()))
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	leaseOpen = false
	probeOutcome := <-probeDone
	probeJoined = true
	cancelProbe()
	if probeOutcome.err != nil {
		t.Fatalf("borrowed host slot remained held after public return: %v\n%s", probeOutcome.err, output.String())
	}
	_ = probeOutcome.lease.Close()
}

func runGoGateCommandTestInOwnedProcess(t *testing.T) bool {
	t.Helper()
	if os.Getenv("GO_WANT_GO_GATE_COMMAND_TEST") == "1" {
		return false
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
	command.Env = append(os.Environ(), "GO_WANT_GO_GATE_COMMAND_TEST=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("owned go-gate command test failed: %v\n%s", err, output)
	}
	return true
}

func setOwnedGoGateProcessEnvironment(t *testing.T, name, value string) {
	t.Helper()
	if os.Getenv("GO_WANT_GO_GATE_COMMAND_TEST") != "1" {
		t.Setenv(name, value)
		return
	}
	previous, present := os.LookupEnv(name)
	if err := os.Setenv(name, value); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if present {
			_ = os.Setenv(name, previous)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}

func TestWorkerAuthorizedAcceptsOnlyAttemptBoundSectionWorktree(t *testing.T) {
	sourceDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	controlRoot, _, attempt := workerAuthorizedAttemptFixture(t, true)
	detached, err := (gittree.Workspace{Dir: controlRoot}).NewDetachedWorktree(attempt.CandidateTree)
	if err != nil {
		t.Fatal(err)
	}
	defer detached.Close()
	sectionRoot := filepath.Join(detached.Workspace().Dir, "application")
	t.Chdir(sectionRoot)
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runProofRunWorkerAuthorized([]string{"--root", sectionRoot})
	})
	if code != 3 {
		t.Fatalf("section without its native process record was authorized: code=%d stderr=%q", code, stderr)
	}
	sectionProcess, err := proofrun.ProcessIdentityForPID(int64(os.Getppid()), nil)
	if err != nil {
		t.Fatal(err)
	}
	record := proofrun.Record{Suite: "application-checks", Root: sectionRoot, ControlRoot: controlRoot,
		AttemptID: attempt.AttemptID, LaunchID: "section-fixture", Launcher: sectionProcess,
		SuiteProcess: sectionProcess, Watchdog: sectionProcess, Status: proofrun.StatusRunning}
	recordPath, err := proofrun.ProcessRecordPath(controlRoot, record.AttemptID, record.LaunchID)
	if err != nil {
		t.Fatal(err)
	}
	writeRecord := func() {
		t.Helper()
		encoded, encodeErr := json.Marshal(record)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		if err := os.MkdirAll(filepath.Dir(recordPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(recordPath, encoded, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeRecord()
	code, _, stderr = captureCommandOutput(t, false, true, func() int {
		return runProofRunWorkerAuthorized([]string{"--root", sectionRoot})
	})
	if code != 3 {
		t.Fatalf("section record absent from retained attempt was authorized: code=%d stderr=%q", code, stderr)
	}
	attempt.ProcessKeys = append(attempt.ProcessKeys, record.Key())
	attemptPath, err := proofrun.AttemptPath(controlRoot, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	writeAttempt := func() {
		t.Helper()
		attemptBytes, encodeErr := json.Marshal(attempt)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		if err := os.WriteFile(attemptPath, attemptBytes, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeAttempt()
	code, _, stderr = captureCommandOutput(t, false, true, func() int {
		return runProofRunWorkerAuthorized([]string{"--root", sectionRoot})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("attempt-bound section worktree denied: code=%d stderr=%q", code, stderr)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	binaryFixture := pinProofBinaryFixture(t, controlRoot)
	build := exec.Command("go", "build", "-o", engine, ".")
	build.Dir = sourceDir
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build candidate section authorization CLI: %v\n%s", err, output)
	}
	cli := binaryFixture.command(os.Environ(), engine, "proof-run", "worker-authorized", "--root", sectionRoot)
	cli.Dir = sectionRoot
	if output, err := cli.CombinedOutput(); err != nil {
		t.Fatalf("new candidate CLI denied old schema-3 section worker: %v\n%s", err, output)
	}
	// The old worker runs command groups directly. They have no nested suite
	// record, but still descend from its live, attempt-registered testing suite.
	record.Root, record.Suite = controlRoot, "testing"
	writeRecord()
	commandCLI := binaryFixture.command(os.Environ(), engine, "proof-run", "worker-authorized", "--root", sectionRoot)
	commandCLI.Dir = sectionRoot
	if output, err := commandCLI.CombinedOutput(); err != nil {
		t.Fatalf("new candidate CLI denied old schema-3 direct command worker: %v\n%s", err, output)
	}
	record.Root, record.Suite = sectionRoot, "application-checks"
	writeRecord()
	denied := func(name, root string) {
		t.Helper()
		status, _, detail := captureCommandOutput(t, false, true, func() int {
			return runProofRunWorkerAuthorized([]string{"--root", root})
		})
		if status != 3 {
			t.Fatalf("%s section authorization = %d, want refusal; stderr=%q", name, status, detail)
		}
	}
	cliDenied := func(name, root string) {
		t.Helper()
		command := binaryFixture.command(os.Environ(), engine, "proof-run", "worker-authorized", "--root", root)
		command.Dir = root
		output, err := command.CombinedOutput()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 3 {
			t.Fatalf("%s candidate CLI authorization = %v, want refusal 3; output=%s", name, err, output)
		}
	}
	record.Status = proofrun.StatusDone
	writeRecord()
	denied("completed section process record", sectionRoot)
	outer := record
	outer.Root, outer.Suite, outer.LaunchID, outer.Status = controlRoot, "testing", "outer-fixture", proofrun.StatusRunning
	outerPath, err := proofrun.ProcessRecordPath(controlRoot, outer.AttemptID, outer.LaunchID)
	if err != nil {
		t.Fatal(err)
	}
	outerBytes, err := json.Marshal(outer)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outerPath, outerBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	attempt.ProcessKeys = append(attempt.ProcessKeys, outer.Key())
	writeAttempt()
	denied("completed section with live outer testing worker", sectionRoot)
	record.Status = proofrun.StatusRunning
	writeRecord()
	attempt.ProcessKeys = []string{outer.Key()}
	writeAttempt()
	denied("unregistered section with live outer testing worker", sectionRoot)
	attempt.ProcessKeys = append(attempt.ProcessKeys, record.Key())
	writeAttempt()
	if err := os.Remove(outerPath); err != nil {
		t.Fatal(err)
	}
	attempt.ProcessKeys = []string{record.Key()}
	writeAttempt()
	record.Root = controlRoot
	writeRecord()
	denied("section process record for another root", sectionRoot)
	record.Root = sectionRoot
	foreignReadyRead, foreignReadyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	foreignReleaseRead, foreignReleaseWrite, err := os.Pipe()
	if err != nil {
		_ = foreignReadyRead.Close()
		_ = foreignReadyWrite.Close()
		t.Fatal(err)
	}
	foreignProcess := exec.Command("/bin/sh", "-c", "printf 'ready\\n' >&3; IFS= read -r _ <&4")
	foreignProcess.ExtraFiles = []*os.File{foreignReadyWrite, foreignReleaseRead}
	if err := foreignProcess.Start(); err != nil {
		_ = foreignReadyRead.Close()
		_ = foreignReadyWrite.Close()
		_ = foreignReleaseRead.Close()
		_ = foreignReleaseWrite.Close()
		t.Fatal(err)
	}
	_ = foreignReadyWrite.Close()
	_ = foreignReleaseRead.Close()
	foreignDone := make(chan error, 1)
	go func() { foreignDone <- foreignProcess.Wait() }()
	foreignJoined := false
	t.Cleanup(func() {
		_ = foreignReleaseWrite.Close()
		if !foreignJoined {
			_ = foreignProcess.Process.Kill()
			<-foreignDone
		}
	})
	foreignReady, readyErr := bufio.NewReader(foreignReadyRead).ReadString('\n')
	_ = foreignReadyRead.Close()
	if readyErr != nil || foreignReady != "ready\n" {
		t.Fatalf("foreign process readiness=%q err=%v", foreignReady, readyErr)
	}
	record.SuiteProcess, err = proofrun.ProcessIdentityForPID(int64(foreignProcess.Process.Pid), nil)
	if err != nil {
		t.Fatal(err)
	}
	writeRecord()
	denied("section process record for unrelated process", sectionRoot)
	if _, err := foreignReleaseWrite.Write([]byte("release\n")); err != nil {
		t.Fatal(err)
	}
	if err := foreignReleaseWrite.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-foreignDone; err != nil {
		t.Fatalf("join foreign process: %v", err)
	}
	foreignJoined = true
	record.SuiteProcess = sectionProcess
	writeRecord()
	t.Chdir(controlRoot)
	denied("wrong cwd", sectionRoot)
	t.Chdir(sectionRoot)
	foreign := filepath.Join(t.TempDir(), "same-tree-checkout")
	runGit(t, controlRoot, "worktree", "add", "--detach", foreign, "HEAD")
	t.Cleanup(func() { runGit(t, controlRoot, "worktree", "remove", "--force", foreign) })
	t.Chdir(foreign)
	denied("ordinary same-tree linked checkout", foreign)
	cliDenied("ordinary same-tree linked checkout", foreign)
	t.Chdir(sectionRoot)
	t.Setenv("METASYSTEM_PROOF_ATTEMPT", "forged-attempt")
	denied("forged attempt locator", sectionRoot)
	t.Setenv("METASYSTEM_PROOF_ATTEMPT", attempt.AttemptID)
	conf := filepath.Join(detached.Workspace().Dir, "metasystem.conf")
	original, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, append(append([]byte(nil), original...), []byte("\n# dirty tracked\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	denied("dirty tracked file", sectionRoot)
	cliDenied("dirty tracked file", sectionRoot)
	runGit(t, detached.Workspace().Dir, "add", "metasystem.conf")
	denied("dirty index", sectionRoot)
	runGit(t, detached.Workspace().Dir, "reset", "--hard", "HEAD")
	untracked := filepath.Join(sectionRoot, "untracked-source.go")
	if err := os.WriteFile(untracked, []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	denied("untracked source", sectionRoot)
	if err := os.Remove(untracked); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, append(append([]byte(nil), original...), []byte("\n# different tree\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, detached.Workspace().Dir, "add", "metasystem.conf")
	runGit(t, detached.Workspace().Dir, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "different candidate tree")
	denied("wrong HEAD tree", sectionRoot)
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
  fixture) [[ "$2" == hidden ]] && printf 'hidden\tfixture-only section\n' ;;
  twice) printf 'repeat\n' ;;
  *) exit 2 ;;
esac
`
	if err := testexec.WriteFile(selector, []byte(script), 0o700); err != nil {
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
	sections, repeated, err = selectedSections(selector, "hidden", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0] != "hidden" || len(repeated) != 0 {
		t.Fatalf("fixture-only selected selector data = %v, %v", sections, repeated)
	}
	if _, _, err = selectedSections(selector, "undeclared", false); err == nil || !strings.Contains(err.Error(), "absent from the selector and bounded fixture declarations") {
		t.Fatalf("undeclared selected section error = %v", err)
	}
	sections, repeated, err = selectedSections(selector, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(sections, ",") != "first,repeat" || len(repeated) != 0 {
		t.Fatalf("enumerated selector data = %v, %v", sections, repeated)
	}
}

func TestValidationSelectorDeclaresGuardFixtureOutsideNormalSelection(t *testing.T) {
	t.Parallel()
	selector := filepath.Join("..", "..", "scripts", "agents", "validate-section-selector.sh")
	listed, err := exec.Command("bash", selector, "list").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(listed), "checkout-execution-guard-fixture") {
		t.Fatalf("normal validation selection contains the bounded guard fixture:\n%s", listed)
	}
	sections, repeated, err := selectedSections(selector, "checkout-execution-guard-fixture", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 || sections[0] != "checkout-execution-guard-fixture" || len(repeated) != 0 {
		t.Fatalf("guard fixture selection = %v, %v", sections, repeated)
	}
}

type authenticatedGuardInvocation struct {
	command       *exec.Cmd
	output        bytes.Buffer
	executionRoot string
	progress      string
	result        string
	admission     string
}

func newAuthenticatedGuardInvocation(t *testing.T, workspace gittree.Workspace, tree, engine string, now time.Time) *authenticatedGuardInvocation {
	t.Helper()
	detached, err := workspace.NewDetachedWorktree(tree)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := detached.Close(); err != nil {
			t.Errorf("close authenticated guard execution checkout: %v", err)
		}
	})
	executionRoot, err := filepath.EvalSymlinks(detached.Workspace().Dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(executionRoot, "metasystem.conf.local")); !os.IsNotExist(err) {
		t.Fatalf("private execution checkout contains ignored local configuration: %v", err)
	}
	pinProofBinaryFixture(t, executionRoot)

	controlRoot, _ := proofExtensionGoalFixtureAt(t, now)
	caller, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe authenticated guard fixture caller: state=%s err=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(controlRoot, "fixture-events-proof-main", caller.Pid,
		caller.StartedAt.Unix(), caller.StartTicks, caller.BootID, "fixture-events-proof", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	canonicalControl, err := filepath.EvalSymlinks(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	if canonicalControl == executionRoot {
		t.Fatal("authenticated guard control and execution roots are not independent")
	}

	private := filepath.Join(executionRoot, "artifacts", "fixture-events-authenticated-guard")
	tmp := filepath.Join(private, "tmp")
	progress := filepath.Join(executionRoot, "artifacts", "agents", "supervision", "suite-progress.jsonl")
	for _, directory := range []string{private, tmp, filepath.Dir(progress)} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	outerScript := filepath.Join(private, "authenticated-outer.sh")
	outerBody := `#!/usr/bin/env bash
set -euo pipefail
progress=$1
root=$2
engine=$3
work=$4
at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '{"suite":"validate-metasystem","section":"gate-fence-fixtures","event":"start","at":"%s","depth":0}\n' "$at" >>"$progress"
env METASYSTEM_SUITE_PROGRESS_ACTIVE=1 \
  METASYSTEM_SUITE_PROGRESS_ROOT="$root" \
  METASYSTEM_SUITE_PROGRESS_DEPTH=0 \
  METASYSTEM_SUITE_PROGRESS_TMP="$work" \
  METASYSTEM_SUITE_PROGRESS_TMP_OWNER= \
  METASYSTEM_BIN="$engine" \
  bash "$root/scripts/agents/checkout-execution-guard-fixtures.sh"
at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '{"suite":"validate-metasystem","section":"gate-fence-fixtures","event":"end","at":"%s","depth":0}\n' "$at" >>"$progress"
`
	if err := testexec.WriteFile(outerScript, []byte(outerBody), 0o700); err != nil {
		t.Fatal(err)
	}

	admissionDir := filepath.Join(private, "host-admission")
	resultPath := filepath.Join(private, "result.json")
	environment := append(receiptCanaryEnvironment(),
		"METASYSTEM_GOAL_NOW="+now.Format(time.RFC3339),
		"METASYSTEM_OWNER_LINEAGE=m1",
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR="+admissionDir,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+controlRoot)
	command := (proofBinaryFixture{t: t}).command(environment, engine, "proof-run", "launch",
		"--suite", "validate-metasystem", "--root", executionRoot, "--control-root", controlRoot,
		"--goal", "standing-validation", "--cap-min", "1", "--scope", "selected",
		"--command-class", "fixture-events-authenticated-guard", "--conf", filepath.Join(executionRoot, "metasystem.conf"),
		"--progress", progress, "--log", filepath.Join(private, "outer.log"),
		"--tmp", tmp, "--banner", "authenticated checkout guard fixture",
		"--selector", filepath.Join(executionRoot, "scripts", "agents", "validate-section-selector.sh"),
		"--selected", "gate-fence-fixtures", "--result", resultPath,
		"--", outerScript, progress, executionRoot, engine, tmp)
	run := &authenticatedGuardInvocation{
		command:       command,
		executionRoot: executionRoot,
		progress:      progress,
		result:        resultPath,
		admission:     admissionDir,
	}
	command.Stdout = &run.output
	command.Stderr = &run.output
	return run
}

func (run *authenticatedGuardInvocation) assertCompleted(t *testing.T) {
	t.Helper()
	if !bytes.Contains(run.output.Bytes(), []byte("checkout execution guard fixtures passed")) {
		t.Fatalf("authenticated public fixture omitted its terminal marker:\n%s", run.output.Bytes())
	}
	resultBytes, err := os.ReadFile(run.result)
	if err != nil {
		t.Fatal(err)
	}
	var result proofrun.LaunchResult
	if err := json.Unmarshal(resultBytes, &result); err != nil || result.Disposition != proofrun.DispositionExecuted || result.ExitStatus != 0 {
		t.Fatalf("authenticated outer result=%+v decode=%v bytes=%s", result, err, resultBytes)
	}
	progressRun, err := proofrun.ReadLatestProgressRun(run.progress)
	if err != nil {
		t.Fatal(err)
	}
	if err := proofrun.AssertSectionProgress(progressRun, "validate-metasystem", []string{"gate-fence-fixtures"}, nil); err != nil {
		t.Fatalf("outer progress certification: %v; events=%+v", err, progressRun.Events)
	}
	if len(progressRun.Events) != 2 {
		t.Fatalf("authenticated fixture appended events outside the outer section: %+v", progressRun.Events)
	}
	assertHostAdmissionClean(t, run.admission, 1)
}

func TestAuthenticatedOuterProofKeepsGuardFixtureControlOutOfOuterProgress(t *testing.T) {
	t.Parallel()
	sourceRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	engine := os.Getenv("METASYSTEM_FIXTURE_EVENTS_BINARY")
	if engine == "" {
		engine = filepath.Join(t.TempDir(), "metasystem")
		build := exec.Command("go", "build", "-p=1", "-o", engine, "./cmd/metasystem")
		build.Dir = sourceRoot
		if output, buildErr := build.CombinedOutput(); buildErr != nil {
			t.Fatalf("build authenticated guard fixture engine: %v\n%s", buildErr, output)
		}
	}
	if info, statErr := os.Stat(engine); statErr != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("authenticated guard fixture engine is not executable: %s: %v", engine, statErr)
	}

	callerProgress := filepath.Join(sourceRoot, "artifacts", "agents", "supervision", "suite-progress.jsonl")
	callerBytes, callerReadErr := os.ReadFile(callerProgress)
	callerExisted := callerReadErr == nil
	if callerReadErr != nil && !os.IsNotExist(callerReadErr) {
		t.Fatal(callerReadErr)
	}

	workspace := gittree.Workspace{Dir: sourceRoot}
	tree, err := workspace.Snapshot("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	runs := []*authenticatedGuardInvocation{
		newAuthenticatedGuardInvocation(t, workspace, tree, engine, now),
		newAuthenticatedGuardInvocation(t, workspace, tree, engine, now),
	}
	if runs[0].executionRoot == runs[1].executionRoot {
		t.Fatalf("concurrent execution roots are not distinct: %s", runs[0].executionRoot)
	}
	firstProgress, err := filepath.EvalSymlinks(filepath.Dir(runs[0].progress))
	if err != nil {
		t.Fatal(err)
	}
	secondProgress, err := filepath.EvalSymlinks(filepath.Dir(runs[1].progress))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Join(firstProgress, filepath.Base(runs[0].progress)) == filepath.Join(secondProgress, filepath.Base(runs[1].progress)) {
		t.Fatalf("concurrent progress journals are not distinct: %s", runs[0].progress)
	}

	started := 0
	for _, run := range runs {
		if err := run.command.Start(); err != nil {
			for _, active := range runs[:started] {
				_ = active.command.Process.Kill()
				_ = active.command.Wait()
			}
			t.Fatalf("start authenticated outer guard fixture: %v", err)
		}
		started++
	}
	runErrors := make([]error, len(runs))
	for i, run := range runs {
		runErrors[i] = run.command.Wait()
	}

	afterCallerBytes, afterCallerReadErr := os.ReadFile(callerProgress)
	afterCallerExisted := afterCallerReadErr == nil
	if afterCallerReadErr != nil && !os.IsNotExist(afterCallerReadErr) {
		t.Fatal(afterCallerReadErr)
	}
	if callerExisted != afterCallerExisted || !bytes.Equal(callerBytes, afterCallerBytes) {
		t.Fatalf("authenticated guard fixtures changed their caller's progress journal: existed %t -> %t, bytes %x -> %x",
			callerExisted, afterCallerExisted, sha256.Sum256(callerBytes), sha256.Sum256(afterCallerBytes))
	}
	for i, runErr := range runErrors {
		if runErr != nil {
			t.Fatalf("authenticated outer guard fixture %d: %v\n%s", i+1, runErr, runs[i].output.Bytes())
		}
		runs[i].assertCompleted(t)
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
	if err := testexec.WriteFile(watchdog, []byte(`#!/usr/bin/env bash
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

func TestProofDeadlineUsesAbsoluteSemanticBoundary(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)
	deadline := now.Add(time.Minute)
	parsed, check, err := proofDeadline(deadline.Format(time.RFC3339Nano), func() time.Time { return now })
	if err != nil || !parsed.Equal(deadline) || check == nil {
		t.Fatalf("before deadline parsed=%s check=%v err=%v", parsed, check != nil, err)
	}
	now = deadline.Add(-time.Nanosecond)
	if err := check(); err != nil {
		t.Fatalf("deadline-minus-one-nanosecond refused: %v", err)
	}
	for _, at := range []time.Time{deadline, deadline.Add(time.Nanosecond)} {
		now = at
		if err := check(); err == nil {
			t.Fatalf("semantic time %s crossed deadline %s without refusal", at, deadline)
		}
	}
}

func testProofResourceWaitUsesSemanticDeadline(t *testing.T) {
	t.Helper()
	for _, testCase := range []struct {
		name string
		at   time.Duration
		pass bool
	}{{"before", -time.Nanosecond, true}, {"at", 0, false}, {"after", time.Nanosecond, false}} {
		t.Run("resource-"+testCase.name, func(t *testing.T) {
			root := t.TempDir()
			conf := filepath.Join(root, "metasystem.conf")
			if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=1\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			admissionDir := filepath.Join(t.TempDir(), "host-admission")
			t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", admissionDir)
			t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root)
			holder, err := proofrun.AcquireHostResources(context.Background(), root, conf, "heavy", nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = holder.Close() })
			now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
			deadline := now.Add(time.Minute)
			_, check, err := proofDeadline(deadline.Format(time.RFC3339Nano), func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			ctx := proofrun.WithHostResourceWaitObserver(t.Context(), func() {
				now = deadline.Add(testCase.at)
				_ = holder.Close()
			})
			lease, err := proofrun.AcquireHostResourcesWithWaitCheck(ctx, root, conf, "heavy", nil, check)
			if testCase.pass {
				if err != nil || lease == nil {
					t.Fatalf("resource acquisition before deadline: lease=%v err=%v", lease, err)
				}
				_ = proofrun.MarkHostResourcesClean(lease.Files())
				_ = lease.Close()
			} else if err == nil || lease != nil || !strings.Contains(err.Error(), "has passed at semantic time") {
				t.Fatalf("resource acquisition at offset %s: lease=%v err=%v", testCase.at, lease, err)
			}
		})
	}
}

func proofExtensionGoalFixture(t *testing.T) (string, time.Time) {
	return proofExtensionGoalFixtureAt(t, time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC))
}

func proofExtensionGoalFixtureAt(t *testing.T, now time.Time) (string, time.Time) {
	t.Helper()
	root := syncedClaimedGoalFixtureAt(t, now)
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
		attempt.SchemaVersion != proofrun.IdentityAttemptSchemaVersion || attempt.CandidateGoalID != attempt.GoalID ||
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
	ambientAncestor, ok := lease.ParentPid(parent.Pid)
	if !ok {
		t.Fatal("native delegate fixture has no ambient ancestor")
	}
	identities := writeTemp(t, t.TempDir(), "native-proof-identities.json", map[string]any{
		strconv.FormatInt(ambientAncestor, 10): map[string]any{"terminal": true},
	})
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", identities)
	classified, err := classifyVerbCaller(root, parent.Pid)
	if err != nil || classified.Class != lease.ClassHuman {
		t.Fatalf("fixture ambient caller classification = %+v, %v; want HUMAN", classified, err)
	}
	verified, err := lease.HookDelegate(root, root, "native-proof", parent.Pid)
	if err != nil || !verified.Delegate || verified.JobID != "native-proof" {
		t.Fatalf("fixture native delegate custody = %+v, %v", verified, err)
	}
	t.Setenv("METASYSTEM_HOOK_DELEGATE_STATE_ROOT", root)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT", root)
	t.Setenv("METASYSTEM_HOOK_DELEGATE_JOB", "native-proof")
	t.Setenv("METASYSTEM_GOAL_NOW", now.Format(time.RFC3339))
	preBinding, err := dispatchcore.ResolveGoalBinding(root, "standing-validation", now)
	if err != nil {
		t.Fatalf("resolve pre-extension binding: %v", err)
	}
	preVerdict, verdictErr := dispatchcore.EvaluateProofAdmissionForDispatch(root, preBinding.GoalID, preBinding.Revision,
		preBinding.File, preBinding.Revision, 1, now, "implementer", "fresh", dispatchcore.HazardMechanical)
	if verdictErr != nil || !preVerdict.Refused() || preVerdict.Authority.Extension == nil ||
		!strings.Contains(strings.Join(dispatchcore.FormatProofAdmission(preVerdict), "; "), "attemptLimit used=1 limit=1") {
		t.Fatalf("fixture did not begin at exact extendable exhaustion: binding=%+v verdict=%+v err=%v", preBinding, preVerdict, verdictErr)
	}
	attempt, decision, joined, err := admitCandidateProofLaunch(t, proofLaunchAdmission{
		ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: "standing-validation",
		CapMin: "1", ScopeClass: "full", CommandClass: "testing",
	})

	if err != nil || joined || decision.Disposition != proofrun.DispositionExecuted || attempt.AttemptID == "" {
		t.Fatalf("native delegate proof did not extend and reserve: attempt=%+v decision=%+v joined=%v err=%v", attempt, decision, joined, err)
	}
	binding, err := dispatchcore.ResolveGoalBinding(root, "standing-validation", now)
	if err != nil || binding.File.Budget.AttemptLimit != 2 || binding.File.BudgetExtension == nil ||
		binding.File.BudgetExtension.AttemptLimitFrom != 1 || binding.File.BudgetExtension.AttemptLimitTo != 2 {
		t.Fatalf("authoritative post-extension binding = %+v, %v", binding, err)
	}
	postVerdict, err := dispatchcore.EvaluateProofAdmissionForDispatch(root, binding.GoalID, binding.Revision,
		binding.File, binding.Revision, 1, now, "implementer", "fresh", dispatchcore.HazardMechanical)
	if err != nil || !postVerdict.Refused() || postVerdict.Authority.Extension != nil ||
		!strings.Contains(strings.Join(dispatchcore.FormatProofAdmission(postVerdict), "; "), "attemptLimit used=2 limit=2") {
		t.Fatalf("consumed extension did not restore exact exhaustion: verdict=%+v err=%v", postVerdict, err)
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
	capacityConfig := filepath.Join(controlRoot, "metasystem.conf")
	capacityBytes, err := os.ReadFile(capacityConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(capacityBytes, []byte(proofrun.AdmissionCapKey+"=4")) {
		t.Fatal("proof fixture has no pinned host cap to narrow")
	}
	capacityBytes = bytes.Replace(capacityBytes, []byte(proofrun.AdmissionCapKey+"=4"), []byte(proofrun.AdmissionCapKey+"=1"), 1)
	if err := os.WriteFile(capacityConfig, capacityBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	controlRoot, err = filepath.EvalSymlinks(controlRoot)
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
	goalSyncMutationGit(t, controlRoot, "add", "plans/goals/standing-validation.md", "metasystem.conf")
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
	ready := filepath.Join(t.TempDir(), "retry-child-ready")
	release := filepath.Join(t.TempDir(), "retry-child-release")
	body := `count=0; test ! -f "$1" || count=$(cat "$1"); count=$((count+1)); printf '%d\n' "$count" >"$1"; if test "$count" -gt 1; then nested_engine=$4; nested_root=$5; nested_control=$6; nested_conf=$7; nested_result=$8; if test -z "$nested_engine"; then printf "nested command argc=%s\n" "$#" >&2; exit 97; fi; env -u METASYSTEM_HOST_RESOURCE_FDS "$nested_engine" proof-run launch --suite nested-command --root "$nested_root" --control-root "$nested_control" --goal standing-validation --cap-min 1 --command-class command-retry --conf "$nested_conf" --progress "$nested_result.progress" --log "$nested_result.log" --banner nested --result "$nested_result" -- sh -c "printf 'nested-legacy-tail\\n'" >"$nested_result.output" 2>&1 || { printf "nested engine=%s argc=%s\n" "$nested_engine" "$#" >&2; cat "$nested_result.output" >&2; exit 97; }; printf 'ready\n' >"$2"; while test ! -e "$3"; do sleep .05; done; fi; test "$count" -gt 1`
	admissionDir := filepath.Join(t.TempDir(), "host-admission")
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", admissionDir)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", controlRoot)
	environment := append(receiptCanaryEnvironment(), "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identities,
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR="+admissionDir,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+controlRoot)
	run := func(root, retry, result string) (int, proofrun.LaunchResult, string) {
		args := []string{"proof-run", "launch", "--suite", "command-retry", "--root", root, "--control-root", controlRoot,
			"--goal", "standing-validation", "--cap-min", "1", "--command-class", "command-retry", "--conf", filepath.Join(root, "metasystem.conf"),
			"--progress", result + ".progress.jsonl", "--log", result + ".log",
			"--banner", "command retry canary", "--result", result}
		if retry != "" {
			args = append(args, "--retry-decision", retry)
		}
		args = append(args, "--", "bash", "-c", body, "fixture", count, ready, release,
			engine, root, controlRoot, filepath.Join(root, "metasystem.conf"), result+".nested.json")
		command := proofFixture.command(environment, engine, args...)
		var output []byte
		if retry == "" {
			output, err = command.CombinedOutput()
		} else {
			var buffer bytes.Buffer
			command.Stdout, command.Stderr = &buffer, &buffer
			if err = command.Start(); err == nil {
				defer command.Process.Kill()
				finished := make(chan error, 1)
				go func() { finished <- command.Wait() }()
				waitForPublicRouteFileOrExit(t, ready, finished, func() string { return buffer.String() })
				probeContext, cancel := context.WithCancel(t.Context())
				observedWait := false
				probeContext = proofrun.WithHostResourceWaitObserver(probeContext, func() {
					observedWait = true
					cancel()
				})
				lease, acquireErr := proofrun.AcquireHostResources(probeContext, controlRoot,
					capacityConfig, "heavy", nil)
				cancel()
				if lease != nil {
					_ = lease.Close()
				}
				if !observedWait || !errors.Is(acquireErr, context.Canceled) {
					t.Fatalf("admitted proof did not reach a failed cap-1 scan: observed=%t err=%v", observedWait, acquireErr)
				}
				if writeErr := os.WriteFile(release, []byte("release\n"), 0o600); writeErr != nil {
					t.Fatal(writeErr)
				}
				err = <-finished
			}
			output = buffer.Bytes()
		}
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
	assertHostAdmissionClean(t, admissionDir, 1)
	secondResult := filepath.Join(t.TempDir(), "second.json")
	status, second, _ := run(renamedRoot, "", secondResult)
	if status != proofrun.ExitRetryRequired || second.Disposition != proofrun.DispositionRetryRequired || second.PriorAttempt != first.AttemptID {
		t.Fatalf("undiagnosed repeat status=%d result=%+v", status, second)
	}
	assertHostAdmissionClean(t, admissionDir, 1)
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
	nestedData, nestedErr := os.ReadFile(thirdResult + ".nested.json")
	var nestedResult proofrun.LaunchResult
	if nestedErr != nil || json.Unmarshal(nestedData, &nestedResult) != nil || nestedResult.Disposition != proofrun.DispositionExecuted || nestedResult.ExitStatus != 0 {
		t.Fatalf("nested public proof did not execute under the outer cap-1 lease: result=%+v readErr=%v bytes=%s", nestedResult, nestedErr, nestedData)
	}
	nestedRun, err := proofrun.ReadLatestProgressRun(thirdResult + ".nested.json.progress")
	if err != nil || len(nestedRun.Header.LogPaths) != 5 {
		t.Fatalf("legacy-borrowed public proof lacked durable custody outputs: paths=%v err=%v", nestedRun.Header.LogPaths, err)
	}
	nestedOutput, err := os.ReadFile(thirdResult + ".nested.json.output")
	if err != nil || !bytes.Contains(nestedOutput, []byte("nested-legacy-tail")) {
		t.Fatalf("legacy-borrowed public tail missing: output=%q err=%v", nestedOutput, err)
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
	assertHostAdmissionClean(t, admissionDir, 1)

}

func assertHostAdmissionClean(t *testing.T, directory string, wantLeases int, allowedStaleManagedPID ...int) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read isolated host admission: %v", err)
	}
	leases := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "managed-") {
			if len(allowedStaleManagedPID) != 1 || entry.Name() != fmt.Sprintf("managed-%d.json", allowedStaleManagedPID[0]) {
				t.Fatalf("unexpected managed proof process marker remained after exit: %s", entry.Name())
			}
			continue
		}
		if !strings.HasPrefix(entry.Name(), "lease-heavy-") {
			continue
		}
		leases++
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var marker struct {
			Cleared bool `json:"cleared"`
		}
		if json.Unmarshal(data, &marker) != nil || !marker.Cleared {
			t.Fatalf("resource custody remained dirty after exit: %s: %s", entry.Name(), data)
		}
	}
	if wantLeases > 0 && leases != wantLeases || wantLeases == 0 && leases == 0 {
		t.Fatalf("host resource leases = %d, want %d", leases, wantLeases)
	}
}

func waitForPublicRouteFile(t *testing.T, path string) {
	waitForPublicRouteFileOrExit(t, path, nil, nil)
}

func waitForPublicRouteFileOrExit(t *testing.T, path string, exited <-chan error, output func() string) {
	t.Helper()
	deadline, bounded := t.Deadline()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if bounded && !time.Now().Before(deadline) {
			t.Fatalf("public proof child did not publish %s before test deadline", filepath.Base(path))
		}
		select {
		case exitErr := <-exited:
			t.Fatalf("public proof exited before publishing %s: %v; output=%s", filepath.Base(path), exitErr, output())
		case <-t.Context().Done():
			t.Fatalf("public proof child did not publish %s before test cancellation: %v", filepath.Base(path), t.Context().Err())
		case <-ticker.C:
		}
	}
}

type publicCapacityWaitOutput struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	waiting chan struct{}
	once    sync.Once
}

func (w *publicCapacityWaitOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buffer.Write(data)
	if strings.Contains(w.buffer.String(), proofCapacityWaitLine) {
		w.once.Do(func() { close(w.waiting) })
	}
	return n, err
}

func (w *publicCapacityWaitOutput) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

func TestProofRunLegacyPublicLaunchUsesAndClearsHostPhase(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture := proofBinaryFixture{t: t}
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-o", engine, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build public legacy canary: %v\n%s", err, output)
	}
	admissionDir := filepath.Join(t.TempDir(), "host-admission")
	result := filepath.Join(t.TempDir(), "result.json")
	childOutput := filepath.Join(root, "child-ran")
	environment := append(receiptCanaryEnvironment(),
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR="+admissionDir,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+root)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", admissionDir)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root)
	holder, err := proofrun.AcquireHostResources(context.Background(), root, conf, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	// The observer must see a completed failed scan, with both the guard and
	// partially acquired named resource released before it runs.
	probeContext, cancelProbe := context.WithCancel(t.Context())
	observed := 0
	var probeErr error
	probeContext = proofrun.WithHostResourceWaitObserver(probeContext, func() {
		observed++
		for _, path := range []string{
			filepath.Join(admissionDir, "admission.lock"),
			filepath.Join(admissionDir, fmt.Sprintf("resource-%x", sha256.Sum256([]byte("observer-probe")))),
		} {
			file, openErr := os.OpenFile(path, os.O_RDWR, 0)
			if openErr != nil {
				probeErr = openErr
				break
			}
			lockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
			if lockErr == nil {
				lockErr = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
			}
			_ = file.Close()
			if lockErr != nil {
				probeErr = fmt.Errorf("observer saw %s locked: %w", filepath.Base(path), lockErr)
				break
			}
		}
		cancelProbe()
	})
	probe, err := proofrun.AcquireHostResources(probeContext, root, conf, "heavy", []string{"observer-probe"})
	cancelProbe()
	if probe != nil {
		_ = probe.Close()
	}
	if observed != 1 || probeErr != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("capacity wait observer was not called once after lock release: count=%d callback=%v acquire=%v", observed, probeErr, err)
	}
	unblockedCalls := 0
	unblockedContext := proofrun.WithHostResourceWaitObserver(t.Context(), func() { unblockedCalls++ })
	unblocked, err := proofrun.AcquireHostResources(unblockedContext, root, conf, "cheap", nil)
	if err != nil || unblockedCalls != 0 {
		t.Fatalf("unblocked resource acquisition reported a wait: calls=%d err=%v", unblockedCalls, err)
	}
	if err := proofrun.MarkHostResourcesClean(unblocked.Files()); err != nil {
		t.Fatal(err)
	}
	if err := unblocked.Close(); err != nil {
		t.Fatal(err)
	}
	command := fixture.command(environment, engine, "proof-run", "launch", "--suite", "legacy-canary",
		"--root", root, "--conf", conf, "--progress", result+".progress.jsonl", "--log", result+".log",
		"--banner", "legacy canary", "--result", result, "--", "bash", "-c", `printf 'ran\n' > "$1"`, "fixture", childOutput)
	output := &publicCapacityWaitOutput{waiting: make(chan struct{})}
	command.Stdout, command.Stderr = output, output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer command.Process.Kill()
	finished := make(chan error, 1)
	go func() { finished <- command.Wait() }()
	select {
	case err := <-finished:
		t.Fatalf("public legacy launch finished behind held cap-1 slot: %v\n%s", err, output.String())
	case <-output.waiting:
	case <-t.Context().Done():
		t.Fatalf("public legacy launch never reported the failed capacity scan: %v\n%s", t.Context().Err(), output.String())
	}
	if _, err := os.Stat(childOutput); !os.IsNotExist(err) {
		t.Fatalf("legacy child ran while host slot held: %v", err)
	}
	if err := proofrun.MarkHostResourcesClean(holder.Files()); err != nil {
		t.Fatal(err)
	}
	if err := holder.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("public legacy launch after release: %v\n%s", err, output.String())
		}
	case <-t.Context().Done():
		t.Fatalf("legacy public launch did not resume after slot release: %v", t.Context().Err())
	}
	if data, err := os.ReadFile(childOutput); err != nil || string(data) != "ran\n" {
		t.Fatalf("legacy child did not run: data=%q err=%v output=%s", data, err, output.String())
	}
	if count := strings.Count(output.String(), proofCapacityWaitLine); count != 1 {
		t.Fatalf("public capacity wait messages = %d, want one: %s", count, output.String())
	}
	assertHostAdmissionClean(t, admissionDir, 1)
	t.Run("closed_fence_before_admission", func(t *testing.T) {
		closedRoot := t.TempDir()
		closedConf := filepath.Join(closedRoot, "metasystem.conf")
		if err := os.WriteFile(closedConf, []byte("metasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(stopfence.TransitionPath(closedRoot)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := stopfence.Write(closedRoot, stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
			Generation: 1, ChangedAt: "2026-09-20T12:00:00Z", Checkout: closedRoot,
			By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 73}}}); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"METASYSTEM_PROOF_CONTROL_ROOT", "METASYSTEM_PROOF_ATTEMPT", "METASYSTEM_PROOF_RUN_ROOT",
			"METASYSTEM_PROOF_RUN_ID", "METASYSTEM_HOOK_DELEGATE_STATE_ROOT", "METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT",
			"METASYSTEM_HOOK_DELEGATE_JOB"} {
			t.Setenv(name, "")
		}
		// An invalid admission path proves the stopped refusal does not try to
		// acquire host capacity.
		t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", "/")
		resultPath := filepath.Join(closedRoot, "result.json")
		logPath := filepath.Join(closedRoot, "suite.log")
		status := runProofRunLaunch([]string{"--suite", "closed", "--root", closedRoot, "--control-root", closedRoot,
			"--conf", closedConf, "--progress", filepath.Join(closedRoot, "progress.jsonl"), "--log", logPath,
			"--banner", "closed", "--result", resultPath, "--", "/bin/true"})
		if status != 1 {
			t.Fatalf("closed fence status = %d, want failed refusal 1", status)
		}
		data, err := os.ReadFile(resultPath)
		if err != nil {
			t.Fatal(err)
		}
		var result proofrun.LaunchResult
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		if result.Disposition != proofrun.DispositionFailed || result.ExitStatus != 1 {
			t.Fatalf("closed fence result = %+v", result)
		}
		if _, err := os.Stat(logPath); !os.IsNotExist(err) {
			t.Fatalf("closed fence reached suite launch: %v", err)
		}
	})
	t.Run("rearmed_generation_cannot_resume_queued_launch", func(t *testing.T) {
		for _, name := range []string{"METASYSTEM_PROOF_CONTROL_ROOT", "METASYSTEM_PROOF_ATTEMPT", "METASYSTEM_PROOF_RUN_ROOT",
			"METASYSTEM_PROOF_RUN_ID", "METASYSTEM_HOOK_DELEGATE_STATE_ROOT", "METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT",
			"METASYSTEM_HOOK_DELEGATE_JOB"} {
			t.Setenv(name, "")
		}
		for _, scenario := range []struct {
			name     string
			changeAt int
			holdSlot bool
		}{
			{name: "while_waiting_for_slot", changeAt: 4, holdSlot: true},
			{name: "at_launcher_handoff", changeAt: 4},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				var holder *proofrun.HostResourceLease
				if scenario.holdSlot {
					var err error
					holder, err = proofrun.AcquireHostResources(context.Background(), root, conf, "heavy", nil)
					if err != nil {
						t.Fatal(err)
					}
					defer holder.Close()
				}
				previous := legacyProofFenceRead
				reads := 0
				legacyProofFenceRead = func(string) (stopfence.Record, error) {
					reads++
					generation := int64(7)
					if reads >= scenario.changeAt {
						generation = 9
						if holder != nil {
							if err := proofrun.MarkHostResourcesClean(holder.Files()); err != nil {
								t.Error(err)
							}
							if err := holder.Close(); err != nil {
								t.Error(err)
							}
							holder = nil
						}
					}
					return stopfence.Record{SchemaVersion: stopfence.SchemaVersion, State: stopfence.StateOpen,
						Phase: stopfence.PhaseArmed, Generation: generation}, nil
				}
				t.Cleanup(func() { legacyProofFenceRead = previous })
				resultPath := filepath.Join(t.TempDir(), "result.json")
				logPath := resultPath + ".log"
				status := runProofRunLaunch([]string{"--suite", "stale-epoch", "--root", root, "--control-root", root,
					"--conf", conf, "--progress", resultPath + ".progress", "--log", logPath,
					"--banner", "stale epoch", "--result", resultPath, "--", "/bin/true"})
				if status != 1 || reads != scenario.changeAt {
					t.Fatalf("stale epoch status=%d fence reads=%d, want failed at read %d", status, reads, scenario.changeAt)
				}
				data, err := os.ReadFile(resultPath)
				if err != nil {
					t.Fatal(err)
				}
				var result proofrun.LaunchResult
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatal(err)
				}
				if result.Disposition != proofrun.DispositionFailed || result.ExitStatus != 1 {
					t.Fatalf("stale epoch result = %+v", result)
				}
				if _, err := os.Stat(logPath); !os.IsNotExist(err) {
					t.Fatalf("stale epoch reached suite launch: %v", err)
				}
			})
		}
	})
}

func TestProofRunLegacyPublicLauncherLossHoldsSlotUntilCustodianDrainsChild(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	if output, err := exec.Command("go", "build", "-o", engine, ".").CombinedOutput(); err != nil {
		t.Fatalf("build public launcher-loss canary: %v\n%s", err, output)
	}
	admissionDir := filepath.Join(t.TempDir(), "host-admission")
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", admissionDir)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root)
	environment := append(receiptCanaryEnvironment(), "METASYSTEM_PROOF_ADMISSION_TEST_DIR="+admissionDir,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+root)
	childPIDPath := filepath.Join(root, "child-pid")
	result := filepath.Join(t.TempDir(), "result.json")
	command := (proofBinaryFixture{t: t}).command(environment, engine, "proof-run", "launch", "--suite", "launcher-loss",
		"--root", root, "--conf", conf, "--progress", result+".progress", "--log", result+".log", "--banner", "launcher-loss",
		"--result", result, "--", "bash", "-c", `printf '%d\n' "$$" > "$1"; while :; do sleep .05; done`, "fixture", childPIDPath)
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer command.Process.Kill()
	finished := make(chan error, 1)
	go func() { finished <- command.Wait() }()
	waitForPublicRouteFileOrExit(t, childPIDPath, finished, func() string { return output.String() })
	pidBytes, err := os.ReadFile(childPIDPath)
	if err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Fatal(err)
	}
	childLive := func() bool {
		actual, state, err := (identity.KernelProber{}).Probe(int64(childPID))
		return err == nil && state == identity.Alive && !actual.Zombie
	}
	if !childLive() {
		t.Fatalf("public child %d was not alive before launcher loss", childPID)
	}
	entries, err := os.ReadDir(admissionDir)
	if err != nil {
		t.Fatal(err)
	}
	var markerPath string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "lease-heavy-") {
			markerPath = filepath.Join(admissionDir, entry.Name())
			break
		}
	}
	if markerPath == "" {
		t.Fatal("public launcher made no heavy custody marker")
	}
	marker, err := os.ReadFile(markerPath)
	if err != nil || !bytes.Contains(marker, []byte(`"cleared":false`)) {
		t.Fatalf("live public child has no dirty marker: %v %s", err, marker)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type acquiredResource struct {
		lease *proofrun.HostResourceLease
		err   error
	}
	acquired := make(chan acquiredResource, 1)
	capacityRetried := make(chan struct{})
	go func() {
		observedContext := proofrun.WithHostResourceWaitObserver(ctx, func() {
			close(capacityRetried)
		})
		lease, err := proofrun.AcquireHostResources(observedContext, root, conf, "heavy", nil)
		acquired <- acquiredResource{lease, err}
	}()
	select {
	case result := <-acquired:
		if result.lease != nil {
			_ = result.lease.Close()
		}
		t.Fatalf("contender acquired before launcher loss while child alive: %v", result.err)
	case <-capacityRetried:
	case <-ctx.Done():
		t.Fatalf("contender never reached capacity retry: %v", ctx.Err())
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-finished
	select {
	case result := <-acquired:
		if result.err != nil {
			t.Fatalf("contender stayed blocked after custodian cleanup: %v; launcher output=%s", result.err, output.String())
		}
		if childLive() {
			_ = result.lease.Close()
			t.Fatal("contender acquired while public child still alive")
		}
		if err := proofrun.MarkHostResourcesClean(result.lease.Files()); err != nil {
			t.Fatal(err)
		}
		if err := result.lease.Close(); err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatalf("custodian did not drain launcher-loss child and release slot: %v; launcher output=%s", ctx.Err(), output.String())
	}
	if _, err := os.Stat(markerPath); err == nil {
		marker, readErr := os.ReadFile(markerPath)
		if readErr != nil || bytes.Contains(marker, []byte(`"cleared":false`)) {
			t.Fatalf("lost launcher left dirty marker: %v %s", readErr, marker)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	// A clean old marker can be reclaimed during the contender's retry, so
	// either one or two clean marker files may remain after the killed owner.
	assertHostAdmissionClean(t, admissionDir, 0, command.Process.Pid)
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
func terminalCommitFixture(t *testing.T, existing ...proofrun.Attempt) (string, proofrun.Attempt, func([]string, func(proofrun.CompletionContext, json.RawMessage) error) (int, string)) {
	t.Helper()
	var root string
	var attempt proofrun.Attempt
	if len(existing) != 0 {
		attempt, root = existing[0], existing[0].ControlRoot
	} else {
		root, _ = proofExtensionGoalFixture(t)
		proofIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "terminal-commit", nil, 2)
		if err != nil {
			t.Fatal(err)
		}
		launcher, err := proofrun.CurrentProcessIdentity(nil)
		if err != nil {
			t.Fatal(err)
		}
		var decision proofrun.LaunchResult
		attempt, decision, err = proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root,
			GoalID: "standing-validation", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 30,
			Identity: proofIdentity, Launcher: launcher, Now: time.Now().UTC()}))
		if err != nil || decision.Disposition != proofrun.DispositionExecuted {
			t.Fatalf("reserve = %+v, %+v, %v", attempt, decision, err)
		}
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
	if err := testexec.WriteFile(watchdog, []byte(`#!/usr/bin/env bash
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

func TestTestingWorkerRetainsDrainedOperationalErrorResultAtTerminal(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	processTable := filepath.Join(t.TempDir(), "processes.json")
	if err := os.WriteFile(processTable, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_CENSUS_PROCESS_FILE", processTable)
	controlRoot, root, attempt := workerAuthorizedAttemptFixture(t, true)
	progressDirectory := filepath.Join(root, "artifacts", "retained-progress")
	progressPath := filepath.Join(progressDirectory, "events.jsonl")
	if err := os.MkdirAll(progressDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	redA := `mkdir -p reports-red-a; printf '%s\n' '<testsuite><testcase classname="fixture" name="red-a"><failure message="red-a"/></testcase></testsuite>' > reports-red-a/tests.xml; printf 'RAW-RED-A\n' >&2; exit 7`
	redB := fmt.Sprintf(`mkdir -p reports-red-b; printf '%%s\n' '<testsuite><testcase classname="fixture" name="red-b"><failure message="red-b"/></testcase></testsuite>' > reports-red-b/tests.xml; printf 'RAW-RED-B\n' >&2; until grep -q '"section":"red-a".*"event":"end"' %s; do sleep 0.01; done; mv %s %s; printf 'closed\n' > %s; exit 8`,
		strconv.Quote(progressPath), strconv.Quote(progressDirectory), strconv.Quote(progressDirectory+"-closed"), strconv.Quote(progressDirectory))
	group := func(id, script string) testpolicy.Group {
		return testpolicy.Group{ID: id, Kind: "component", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit",
			Resources: testpolicy.GroupResources{Class: "cheap"}, Freshness: "reusable", Inputs: []string{"application/tracked.txt"},
			Outputs: []string{"reports-" + id}, Obligations: []string{id}, Platforms: []string{"any"}, TargetMS: 1000,
			Argv: []string{"sh", "-c", script}, Reports: []string{"reports-" + id}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports-" + id + "/tests.xml", Classname: "fixture", Name: id}}}
	}
	contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"application/**"}, Standard: []string{"red-a", "red-b", "later"}}},
		Groups:      []testpolicy.Group{group("red-a", redA), group("red-b", redB), group("later", "exit 99")},
		Always:      testpolicy.Always{Canary: []string{"red-a", "red-b"}, Standard: []string{"later"}}, Unknown: []string{"red-a"}}
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard,
		ExecutedMode: testpolicy.ModeStandard, RequiredGroups: []string{"red-a", "red-b", "later"}, SelectedGroups: []string{"red-a", "red-b", "later"},
		Stages: []testpolicy.Stage{{ID: "canary", Groups: []string{"red-a", "red-b"}}, {ID: "standard", Groups: []string{"later"}}}}
	engine, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	engineDigest, err := fileSHA256(engine)
	if err != nil {
		t.Fatal(err)
	}
	request := proofrun.TestRunRequest{ControlRoot: controlRoot, ProjectRoot: root, CandidateTree: attempt.CandidateTree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD",
		Contract: contract, Plan: plan, AttemptID: attempt.AttemptID, Environment: gittree.ScrubbedEnviron(), LogRoot: filepath.Join(root, "artifacts", "retained-logs"),
		ProgressPath: progressPath, PolicyEngine: engine, PolicyEngineDigest: engineDigest, CandidateEngine: engine, CandidateEngineDigest: engineDigest,
		CandidateEngineBuildIdentity: attempt.CandidateTree, Workers: 2, AdmissionMaximum: 0, Concurrency: 2}
	identities, prepared, launches, err := proofrun.PrepareGroupExecutionIdentities(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ComponentIdentities, request.PreparedGroups, request.PreparationLaunches = identities, prepared, launches
	packetPath, resultPath := filepath.Join(root, "artifacts", "retained-request.json"), filepath.Join(root, "artifacts", "retained-result.json")
	if err := writePrivateJSON(packetPath, request); err != nil {
		t.Fatal(err)
	}
	packetDigest, err := fileSHA256(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	_, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runTestWorker([]string{"--packet", packetPath, "--packet-sha256", packetDigest, "--result", resultPath})
	})
	result, err := readTestingWorkerResult(resultPath)
	if err != nil {
		t.Fatalf("worker did not persist its unsuccessful result: %v\nstderr=%s", err, stderr)
	}
	if !strings.Contains(stderr, "record testing group red-b end") || result.Delivery.Sufficient || result.LaunchCounts.Test != 2 || !result.LaunchCounts.CountsComplete {
		t.Fatalf("worker lost the operational error or physical accounting: stderr=%q counts=%+v delivery=%+v", stderr, result.LaunchCounts, result.Delivery)
	}
	byID := map[string]proofrun.GroupResult{}
	for _, observed := range result.Groups {
		byID[observed.ID] = observed
	}
	for _, id := range []string{"red-a", "red-b"} {
		observed := byID[id]
		if observed.Status != "failed" || !observed.NativeLaunched || observed.ExecutionIdentity != identities[id] || observed.InputDigest == "" ||
			len(observed.Observed) != 1 || observed.Observed[0].Name != id || observed.Observed[0].Status != "failed" {
			t.Fatalf("completed native failure %s was not retained: %+v", id, observed)
		}
		logged, readErr := os.ReadFile(observed.LogPath)
		if readErr != nil || !strings.Contains(string(logged), "RAW-RED-") {
			t.Fatalf("completed native failure %s lost raw output: err=%v output=%q", id, readErr, logged)
		}
	}
	if later := byID["later"]; later.Status != "not-run" || later.NativeLaunched || later.ExecutionIdentity != identities["later"] || later.NotRunReason == "" {
		t.Fatalf("later selected group invented coverage or lost identity: %+v", later)
	}

	greenProgressDirectory := filepath.Join(root, "artifacts", "retained-green-progress")
	greenProgressPath := filepath.Join(greenProgressDirectory, "events.jsonl")
	if err := os.MkdirAll(greenProgressDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	greenScript := fmt.Sprintf(`mkdir -p reports-green; printf '%%s\n' '<testsuite><testcase classname="fixture" name="green"/></testsuite>' > reports-green/tests.xml; printf 'RAW-GREEN\n' >&2; mv %s %s; printf 'closed\n' > %s`,
		strconv.Quote(greenProgressDirectory), strconv.Quote(greenProgressDirectory+"-closed"), strconv.Quote(greenProgressDirectory))
	greenContract := contract
	greenContract.Surfaces[0].Standard, greenContract.Groups = []string{"green"}, []testpolicy.Group{group("green", greenScript)}
	greenContract.Always, greenContract.Unknown = testpolicy.Always{Canary: []string{"green"}}, []string{"green"}
	if err := greenContract.Validate(); err != nil {
		t.Fatal(err)
	}
	greenPlan := plan
	greenPlan.RequiredGroups, greenPlan.SelectedGroups = []string{"green"}, []string{"green"}
	greenPlan.Stages = []testpolicy.Stage{{ID: "canary", Groups: []string{"green"}}}
	greenRequest := request
	greenRequest.Contract, greenRequest.Plan, greenRequest.ProgressPath = greenContract, greenPlan, greenProgressPath
	greenIdentities, greenPrepared, greenLaunches, err := proofrun.PrepareGroupExecutionIdentities(context.Background(), greenRequest)
	if err != nil {
		t.Fatal(err)
	}
	greenRequest.ComponentIdentities, greenRequest.PreparedGroups, greenRequest.PreparationLaunches = greenIdentities, greenPrepared, greenLaunches
	greenPacketPath, greenResultPath := filepath.Join(root, "artifacts", "retained-green-request.json"), filepath.Join(root, "artifacts", "retained-green-result.json")
	if err := writePrivateJSON(greenPacketPath, greenRequest); err != nil {
		t.Fatal(err)
	}
	greenPacketDigest, err := fileSHA256(greenPacketPath)
	if err != nil {
		t.Fatal(err)
	}
	greenCode, _, greenStderr := captureCommandOutput(t, false, true, func() int {
		return runTestWorker([]string{"--packet", greenPacketPath, "--packet-sha256", greenPacketDigest, "--result", greenResultPath})
	})
	greenResult, err := readTestingWorkerResult(greenResultPath)
	if err != nil {
		t.Fatalf("worker did not persist its all-pass operational result: %v\nstderr=%s", err, greenStderr)
	}
	if validationErr := proofrun.ValidateTestResult(greenResult); validationErr != nil || greenCode == 0 ||
		!strings.Contains(greenStderr, "record testing group green end") || greenResult.Delivery.Sufficient ||
		len(greenResult.Delivery.FailingGroups) != 0 || len(greenResult.Delivery.MissingGroups) != 0 || len(greenResult.Uncertainty) != 1 ||
		len(greenResult.Groups) != 1 || greenResult.Groups[0].Status != "passed" || !greenResult.Groups[0].NativeLaunched ||
		greenResult.Groups[0].ExecutionIdentity != greenIdentities["green"] || greenResult.LaunchCounts.Test != 1 {
		t.Fatalf("worker all-pass operational result is not exact: code=%d validation=%v stderr=%q result=%+v", greenCode, validationErr, greenStderr, greenResult)
	}

	producerPath, err := proofrun.AttemptPath(controlRoot, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	legacyGroup := func(id, script string) testpolicy.Group {
		return testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"application/tracked.txt"},
			Outputs: []string{"reports-" + id}, Obligations: []string{id}, Platforms: []string{"any"}, TargetMS: 1000,
			Argv: []string{"sh", "-c", script}, Reports: []string{"reports-" + id}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports-" + id + "/tests.xml", Classname: "fixture", Name: id}}}
	}
	firstScript := fmt.Sprintf(`mkdir -p reports-first; printf '%%s\n' '<testsuite><testcase classname="fixture" name="first"/></testsuite>' > reports-first/tests.xml; printf 'RETAINED-SOURCE-INVALIDATION-EVENT\n' >&2; rm %s`, strconv.Quote(producerPath))
	earlierScript := `mkdir -p reports-earlier; printf '%s\n' '<testsuite><testcase classname="fixture" name="earlier"/></testsuite>' > reports-earlier/tests.xml`
	laterScript := `mkdir -p reports-later; printf '%s\n' '<testsuite><testcase classname="fixture" name="later"/></testsuite>' > reports-later/tests.xml`
	legacyContract := testpolicy.Contract{SchemaVersion: testpolicy.SchemaVersion,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"application/**"}, Standard: []string{"earlier", "first", "later"}}},
		Groups:      []testpolicy.Group{legacyGroup("earlier", earlierScript), legacyGroup("first", firstScript), legacyGroup("later", laterScript)},
		Always:      testpolicy.Always{Canary: []string{"earlier"}, Standard: []string{"first", "later"}}, Unknown: []string{"earlier"}}
	if err := legacyContract.Validate(); err != nil {
		t.Fatal(err)
	}
	legacyPlan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard,
		ExecutedMode: testpolicy.ModeStandard, RequiredGroups: []string{"earlier", "first", "later"}, SelectedGroups: []string{"earlier", "first", "later"},
		Stages: []testpolicy.Stage{{ID: "canary", Groups: []string{"earlier"}}, {ID: "standard", Groups: []string{"first"}}, {ID: "retained", Groups: []string{"later"}}}}
	legacyRequest := request
	legacyRequest.Contract, legacyRequest.Plan, legacyRequest.ProgressPath = legacyContract, legacyPlan, ""
	legacyRequest.LogRoot = filepath.Join(root, "artifacts", "retained-source-logs")
	legacyIdentities, legacyPrepared, legacyLaunches, err := proofrun.PrepareGroupExecutionIdentities(context.Background(), legacyRequest)
	if err != nil {
		t.Fatal(err)
	}
	legacyRequest.ComponentIdentities, legacyRequest.PreparedGroups, legacyRequest.PreparationLaunches = legacyIdentities, legacyPrepared, legacyLaunches
	mutation, err := proofrun.AcquireMutation(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	producer, err := proofrun.BindJoinedTestOwnershipLocked(controlRoot, attempt.AttemptID, proofrun.AdmissionRequest{
		ControlRoot: controlRoot, ComponentIdentities: map[string]string{"earlier": legacyIdentities["earlier"], "later": legacyIdentities["later"]}, Now: time.Date(2026, 9, 18, 12, 0, 1, 0, time.UTC)})
	mutation.Release()
	if err != nil {
		t.Fatal(err)
	}
	sourceRequest := legacyRequest
	sourceRequest.AttemptID = producer.AttemptID
	sourceRequest.Plan.RequiredGroups, sourceRequest.Plan.SelectedGroups = []string{"earlier", "later"}, []string{"earlier", "later"}
	sourceRequest.Plan.Stages = []testpolicy.Stage{{ID: "canary", Groups: []string{"earlier"}}, {ID: "standard", Groups: []string{"later"}}}
	sourceRequest.ComponentIdentities = map[string]string{"earlier": legacyIdentities["earlier"], "later": legacyIdentities["later"]}
	sourceResult, sourceCode, sourceErr := proofrun.RunTestPlan(context.Background(), sourceRequest)
	if sourceErr != nil || sourceCode != 0 || !sourceResult.Delivery.Sufficient || len(sourceResult.Groups) != 2 ||
		!sourceResult.Groups[0].NativeLaunched || !sourceResult.Groups[1].NativeLaunched {
		t.Fatalf("retained source producer did not pass natively: code=%d err=%v result=%+v", sourceCode, sourceErr, sourceResult)
	}
	mutation, err = proofrun.AcquireMutation(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	_, err = proofrun.FinalizeAttemptWithTestResultLocked(controlRoot, producer.AttemptID, proofrun.TerminalSuccess, 0, "retained source fixture", nil,
		&sourceResult, time.Date(2026, 9, 18, 12, 0, 2, 0, time.UTC))
	mutation.Release()
	if err != nil {
		t.Fatal(err)
	}
	consumerNow := time.Now().UTC()
	consumer, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: controlRoot, ExecutionRoot: root,
		CandidateTree: attempt.CandidateTree, GoalID: "standing-validation", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2,
		Identity: attempt.ProofIdentity, Launcher: attempt.Launcher, Now: consumerNow,
		ComponentIdentities: legacyIdentities, SharedComponents: true}))
	if err != nil || decision.Disposition != proofrun.DispositionExecuted || consumer.TestOwned["first"] == "" ||
		consumer.TestSources["earlier"] != producer.AttemptID || consumer.TestSources["later"] != producer.AttemptID || len(consumer.TestInventory) != 3 {
		t.Fatalf("retained source consumer admission: attempt=%+v decision=%+v err=%v", consumer, decision, err)
	}
	legacyRequest.AttemptID = consumer.AttemptID
	legacyPacketPath := filepath.Join(root, "artifacts", "retained-source-request.json")
	legacyResultPath := filepath.Join(root, "artifacts", "retained-source-result.json")
	if err := writePrivateJSON(legacyPacketPath, legacyRequest); err != nil {
		t.Fatal(err)
	}
	legacyPacketDigest, err := fileSHA256(legacyPacketPath)
	if err != nil {
		t.Fatal(err)
	}
	setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_ATTEMPT", consumer.AttemptID)
	legacyCode, _, legacyStderr := captureCommandOutput(t, false, true, func() int {
		return runTestWorker([]string{"--packet", legacyPacketPath, "--packet-sha256", legacyPacketDigest, "--result", legacyResultPath})
	})
	legacyResult, err := readTestingWorkerResult(legacyResultPath)
	if err != nil {
		t.Fatalf("worker did not publish the later-stage retained-source failure: %v\nstderr=%s", err, legacyStderr)
	}
	if validationErr := proofrun.ValidateTestResult(legacyResult); validationErr != nil || legacyCode == 0 ||
		!strings.Contains(legacyStderr, "admitted source for later is incomplete") || strings.Contains(legacyStderr, "operational result was not retained") ||
		legacyResult.Delivery.Sufficient || len(legacyResult.Uncertainty) != 1 || legacyResult.LaunchCounts.Test != 1 || legacyResult.LaunchCounts.ReusedTest != 1 || len(legacyResult.Groups) != 3 ||
		legacyResult.Groups[0].ID != "earlier" || legacyResult.Groups[0].Status != "reused" || legacyResult.Groups[0].ReuseAttempt != producer.AttemptID ||
		legacyResult.Groups[1].ID != "first" || legacyResult.Groups[1].Status != "passed" || !legacyResult.Groups[1].NativeLaunched ||
		legacyResult.Groups[1].NativeExitStatus == nil || *legacyResult.Groups[1].NativeExitStatus != 0 || !legacyResult.Groups[1].CollectionComplete ||
		len(legacyResult.Groups[1].Observed) != 1 || legacyResult.Groups[1].Observed[0].Status != "passed" ||
		legacyResult.Groups[2].ID != "later" || legacyResult.Groups[2].Status != "not-run" || legacyResult.Groups[2].NativeLaunched ||
		legacyResult.Groups[2].ExecutionIdentity != legacyIdentities["later"] || !strings.Contains(legacyResult.Groups[2].NotRunReason, "admitted source for later is incomplete") {
		t.Fatalf("later-stage source failure lost native evidence or became green: code=%d validation=%v stderr=%q result=%+v", legacyCode, validationErr, legacyStderr, legacyResult)
	}
	legacyLog, err := os.ReadFile(legacyResult.Groups[1].LogPath)
	if err != nil || !strings.Contains(string(legacyLog), "RETAINED-SOURCE-INVALIDATION-EVENT") {
		t.Fatalf("earlier native pass lost its retained log: err=%v output=%q", err, legacyLog)
	}

	terminalRoot, terminalAttempt, launch := terminalCommitFixture(t)
	var retained *proofrun.TestResult
	status, launcherErrors := launch([]string{"false"}, testingTerminalCommit(resultPath, &retained))
	stored, readErr := proofrun.ReadAttempt(terminalRoot, terminalAttempt.AttemptID)
	if readErr != nil || retained == nil || stored.TestResult == nil {
		t.Fatalf("terminal result is absent: retained=%+v stored=%+v read=%v", retained, stored, readErr)
	}
	wantJSON, wantJSONErr := json.Marshal(result)
	storedWant := result
	storedWant.AttemptID = terminalAttempt.AttemptID
	storedWantJSON, storedWantJSONErr := json.Marshal(storedWant)
	retainedJSON, retainedJSONErr := json.Marshal(retained)
	storedJSON, storedJSONErr := json.Marshal(stored.TestResult)
	if wantJSONErr != nil || storedWantJSONErr != nil || retainedJSONErr != nil || storedJSONErr != nil {
		t.Fatalf("encode retained terminal result: want=%v stored-want=%v retained=%v stored=%v", wantJSONErr, storedWantJSONErr, retainedJSONErr, storedJSONErr)
	}
	if status != 1 || stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalFailed ||
		stored.TestResult == nil || stored.TestResult.Delivery.Sufficient || stored.TestResult.LaunchCounts.Test != 2 || len(stored.TestResult.Groups) != 3 ||
		!bytes.Equal(retainedJSON, wantJSON) || !bytes.Equal(storedJSON, storedWantJSON) ||
		strings.Contains(launcherErrors, "worker left no usable result") {
		t.Fatalf("terminal did not retain the worker result: status=%d errors=%q retained=%+v stored=%+v read=%v", status, launcherErrors, retained, stored, readErr)
	}

	_, _, legacyLaunch := terminalCommitFixture(t, consumer)
	var legacyRetained *proofrun.TestResult
	status, launcherErrors = legacyLaunch([]string{"false"}, testingTerminalCommit(legacyResultPath, &legacyRetained))
	legacyStored, readErr := proofrun.ReadAttempt(controlRoot, consumer.AttemptID)
	if status != 1 || readErr != nil || legacyRetained == nil || legacyStored.Terminal == nil || legacyStored.Terminal.Result != proofrun.TerminalFailed ||
		legacyStored.TestResult == nil || len(legacyStored.TestInventory) != 3 || legacyStored.TestOwned["first"] == "" || len(legacyStored.TestResult.Groups) != 3 ||
		legacyStored.TestResult.Groups[0].ID != "earlier" || legacyStored.TestResult.Groups[0].Status != "not-run" || legacyStored.TestResult.Groups[0].ReuseAttempt != "" ||
		legacyStored.TestResult.Groups[1].ID != "first" || legacyStored.TestResult.Groups[1].Status != "passed" || !legacyStored.TestResult.Groups[1].NativeLaunched ||
		legacyStored.TestResult.Groups[1].LogDigest != legacyResult.Groups[1].LogDigest || legacyStored.TestResult.Groups[1].NativeExitStatus == nil ||
		*legacyStored.TestResult.Groups[1].NativeExitStatus != 0 || legacyStored.TestResult.LaunchCounts.Test != 1 || legacyStored.TestResult.Delivery.Sufficient ||
		len(legacyStored.TestResult.Uncertainty) != 2 || strings.Contains(launcherErrors, "worker left no usable result") {
		t.Fatalf("terminal did not retain the later-stage source failure: status=%d errors=%q retained=%+v stored=%+v read=%v", status, launcherErrors, legacyRetained, legacyStored, readErr)
	}
}
