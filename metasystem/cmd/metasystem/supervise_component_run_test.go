package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestLandingOwnerComponentKeepsHeartbeatLoopOnSetupErrors(t *testing.T) {
	repo := t.TempDir()
	metasystemRoot := filepath.Join(repo, "metasystem")
	if err := os.Mkdir(metasystemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	release, pass, ok := setupLandingOwner(metasystemRoot, repo)
	if !ok {
		t.Fatal("missing configuration stopped the component before its heartbeat loop")
	}
	defer release()
	pass = landingOwnerReportedPass(repo, pass)
	if err := pass(); err != nil {
		t.Fatalf("missing configuration should mean no batch root: %v", err)
	}

	conf := filepath.Join(metasystemRoot, "metasystem.conf")
	contents := "landing.batch-root=" + repo + "\nlanding.batch-max-wait=invalid\n"
	if err := os.WriteFile(conf, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pass(); err == nil || !strings.Contains(err.Error(), "invalid batch wait") {
		t.Fatalf("setup error=%v, want invalid batch wait from configuration below the checkout root", err)
	}
	errorPath := landingOwnerErrorPath(repo)
	if data, err := os.ReadFile(errorPath); err != nil || !strings.Contains(string(data), "invalid batch wait") {
		t.Fatalf("durable setup error=%q error=%v", data, err)
	}
	changed, err := writeLandingOwnerError(repo, errors.New("invalid batch wait: fixture"))
	if err != nil || !changed {
		t.Fatalf("changed setup error record: changed=%t error=%v", changed, err)
	}
	changed, err = writeLandingOwnerError(repo, errors.New("invalid batch wait: fixture"))
	if err != nil || changed {
		t.Fatalf("unchanged setup error rewrote its record: changed=%t error=%v", changed, err)
	}
	if err := os.WriteFile(conf, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pass(); err != nil {
		t.Fatalf("later setup pass did not retry corrected configuration: %v", err)
	}
	if _, err := os.Stat(errorPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful setup left a stale error record: %v", err)
	}
}

func landingOwnerRetryFixture(t *testing.T) (string, func() error, func() error) {
	t.Helper()
	isolateGlobalGitConfig(t)
	originalLineage, hadLineage := os.LookupEnv("METASYSTEM_OWNER_LINEAGE")
	t.Cleanup(func() {
		if hadLineage {
			_ = os.Setenv("METASYSTEM_OWNER_LINEAGE", originalLineage)
			return
		}
		_ = os.Unsetenv("METASYSTEM_OWNER_LINEAGE")
	})
	root := syncedClaimedGoalFixture(t)
	goalSyncMutationGit(t, root, "config", "--unset", "metasystem.goal.machine")
	conf := "metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\n" +
		"landing.batch-root=" + root + "\nlanding.batch-max-wait=45m\n"
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	release, pass, ok := setupLandingOwner(root, root)
	if !ok {
		t.Fatal("landing owner setup stopped the component")
	}
	return root, pass, release
}

func TestLandingOwnerComponentRetriesAfterEnrollmentAppears(t *testing.T) {
	root, pass, release := landingOwnerRetryFixture(t)
	defer release()
	if err := pass(); err == nil || !strings.Contains(err.Error(), "no machine nickname is enrolled") {
		t.Fatalf("first setup error=%v, want missing machine enrollment", err)
	}
	if _, err := lease.CurrentHolder(root); !errors.Is(err, lease.ErrLeaseAbsent) {
		t.Fatalf("configuration failure left a checkout lease: %v", err)
	}
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "mac-cli")
	if err := pass(); err != nil {
		t.Fatalf("setup did not recover after machine enrollment: %v", err)
	}
}

func TestLandingOwnerComponentSetupFailureDoesNotReannounce(t *testing.T) {
	root, pass, release := landingOwnerRetryFixture(t)
	defer release()
	for attempt := 1; attempt <= 3; attempt++ {
		if err := pass(); err == nil || !strings.Contains(err.Error(), "no machine nickname is enrolled") {
			t.Fatalf("setup attempt %d error=%v, want missing machine enrollment", attempt, err)
		}
	}
	cursors, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "mains", "*.protocol-cursor.json"))
	if err != nil {
		t.Fatal(err)
	}
	fence, err := steward.ReadEnrollmentFence(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cursors) > 1 || fence > 1 {
		t.Fatalf("three failing setup passes wrote %d protocol cursors and fence %d, want at most one announcement", len(cursors), fence)
	}
}

func assertLandingOwnerAnnouncementCount(t *testing.T, root string, wantCursors int, wantFence int64) {
	t.Helper()
	cursors, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "mains", "*.protocol-cursor.json"))
	if err != nil {
		t.Fatal(err)
	}
	fence, err := steward.ReadEnrollmentFence(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cursors) != wantCursors || fence != wantFence {
		t.Fatalf("landing owner announcements wrote %d protocol cursors and fence %d, want %d and %d", len(cursors), fence, wantCursors, wantFence)
	}
}

func TestLandingOwnerComponentRecoversFromHolderProofFailure(t *testing.T) {
	root, pass, release := landingOwnerRetryFixture(t)
	defer release()
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "mac-cli")
	mains := filepath.Join(root, "artifacts", "agents", "mains")
	if err := os.MkdirAll(mains, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(mains, "zz-invalid-announcement.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pass(); err == nil || !strings.Contains(err.Error(), "holder proof failed") {
		t.Fatalf("first pass error=%v, want holder proof failure after the lease claim", err)
	}
	if err := os.Remove(bad); err != nil {
		t.Fatal(err)
	}
	if err := pass(); err != nil {
		t.Fatalf("cause was removed but the later pass remained stuck: %v", err)
	}
	assertLandingOwnerAnnouncementCount(t, root, 1, 1)
}

func TestLandingOwnerComponentAnnounceErrorReleasesOnStop(t *testing.T) {
	root, pass, release := landingOwnerRetryFixture(t)
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "mac-cli")
	original := batchOwnerAnnounce
	t.Cleanup(func() { batchOwnerAnnounce = original })
	batchOwnerAnnounce = func(root, session string, pid, start, startTicks int64, bootID, tag, runtime, lineage string) (string, error) {
		if _, err := original(root, session, pid, start, startTicks, bootID, tag, runtime, lineage); err != nil {
			return "", err
		}
		return "", errors.New("injected failure after announcement")
	}
	if err := pass(); err == nil || !strings.Contains(err.Error(), "injected failure after announcement") {
		t.Fatalf("first pass error=%v, want injected post-announcement failure", err)
	}
	release()
	if announcements := lease.AnnouncementsFor(root, int64(os.Getpid())); len(announcements) != 0 {
		t.Fatalf("component stop after announcement failure leaked %d announcements", len(announcements))
	}
}

func TestLandingOwnerComponentForeignHolderDoesNotReannounce(t *testing.T) {
	if root := os.Getenv("GO_WANT_LANDING_OWNER_FOREIGN_HOLDER"); root != "" {
		exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
		if err != nil || state != identity.Alive {
			t.Fatalf("foreign holder identity state=%s error=%v", state, err)
		}
		session := fmt.Sprintf("foreign-holder-%d", os.Getpid())
		if _, err := lease.AnnounceWithPair(root, session, exact.Pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID,
			"foreign-holder", "metasystem", "foreign-lineage"); err != nil {
			t.Fatal(err)
		}
		defer lease.Retire(root, session, exact.Pid, exact.StartedAt.Unix())
		fmt.Println("READY")
		_, _ = os.Stdin.Read(make([]byte, 1))
		return
	}

	root, pass, release := landingOwnerRetryFixture(t)
	defer release()
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "mac-cli")
	child := exec.Command(os.Args[0], "-test.run=^TestLandingOwnerComponentForeignHolderDoesNotReannounce$")
	child.Env = append(os.Environ(), "GO_WANT_LANDING_OWNER_FOREIGN_HOLDER="+root)
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	t.Cleanup(func() {
		if !finished {
			_ = stdin.Close()
			_ = child.Wait()
		}
	})
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "READY" {
		t.Fatalf("foreign holder did not acquire the fixture lease: %q (%v)", scanner.Text(), scanner.Err())
	}
	baseCursors, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "mains", "*.protocol-cursor.json"))
	if err != nil {
		t.Fatal(err)
	}
	baseFence, err := steward.ReadEnrollmentFence(root)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		if err := pass(); err == nil || !strings.Contains(err.Error(), "OWNED-ELSEWHERE") {
			t.Fatalf("contended pass %d error=%v, want live foreign holder refusal", attempt, err)
		}
	}
	assertLandingOwnerAnnouncementCount(t, root, len(baseCursors)+1, baseFence+1)
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	finished = true
	if err := pass(); err != nil {
		t.Fatalf("foreign holder exited but the next pass remained stuck: %v", err)
	}
}

func TestLandingOwnerComponentRecoversFromEnvironmentFailure(t *testing.T) {
	root, pass, release := landingOwnerRetryFixture(t)
	defer release()
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "mac-cli")
	original := batchOwnerSetenv
	t.Cleanup(func() { batchOwnerSetenv = original })
	failed := false
	batchOwnerSetenv = func(key, value string) error {
		if !failed {
			failed = true
			return errors.New("injected environment failure")
		}
		return original(key, value)
	}
	if err := pass(); err == nil || !strings.Contains(err.Error(), "injected environment failure") {
		t.Fatalf("first pass error=%v, want injected environment failure", err)
	}
	if err := pass(); err != nil {
		t.Fatalf("environment recovered but the later pass remained stuck: %v", err)
	}
	assertLandingOwnerAnnouncementCount(t, root, 1, 1)
}

func TestLandingOwnerComponentRetriesConstructionWithFreshInputs(t *testing.T) {
	root, pass, release := landingOwnerRetryFixture(t)
	defer release()
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "machine-one")
	original := batchOwnerConstruct
	t.Cleanup(func() { batchOwnerConstruct = original })
	var machines []string
	batchOwnerConstruct = func(settings config.BatchLanding, held batchOwnerLease, inputs productionBatchOwnerInputs, now func() time.Time) (*batch.Owner, error) {
		machines = append(machines, inputs.machine)
		if len(machines) == 1 {
			return nil, errors.New("injected construction failure")
		}
		return original(settings, held, inputs, now)
	}
	if err := pass(); err == nil || !strings.Contains(err.Error(), "injected construction failure") {
		t.Fatalf("first pass error=%v, want injected construction failure", err)
	}
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "machine-two")
	if err := pass(); err != nil {
		t.Fatalf("construction recovered but the later pass remained stuck: %v", err)
	}
	if got := strings.Join(machines, ","); got != "machine-one,machine-two" {
		t.Fatalf("construction inputs=%s, want each retry to re-read settings and inputs", got)
	}
	assertLandingOwnerAnnouncementCount(t, root, 1, 1)
}

func TestLandingOwnerComponentStopsActingAfterLeaseLoss(t *testing.T) {
	root, pass, release := landingOwnerRetryFixture(t)
	defer release()
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "mac-cli")
	originalResume := batchOwnerResume
	t.Cleanup(func() { batchOwnerResume = originalResume })
	resumes := 0
	batchOwnerResume = func(*batch.Owner) { resumes++ }
	if err := pass(); err != nil || resumes != 1 {
		t.Fatalf("initial pass resumes=%d error=%v", resumes, err)
	}
	leasePath := filepath.Join(root, "artifacts", "agents", "mains", "worktree-lease.json")
	originalLease, err := os.ReadFile(leasePath)
	if err != nil {
		t.Fatal(err)
	}
	var record lease.Lease
	if err := json.Unmarshal(originalLease, &record); err != nil {
		t.Fatal(err)
	}
	record.HolderMainId = "main-taken-over"
	changedLease, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(leasePath, changedLease, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pass(); err == nil || !strings.Contains(err.Error(), "OWNED-ELSEWHERE") {
		t.Fatalf("pass after lease loss error=%v, want holder refusal", err)
	}
	if resumes != 1 {
		t.Fatalf("lease was lost but the component kept acting as holder: resumes=%d", resumes)
	}
	if err := os.WriteFile(leasePath, originalLease, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pass(); err != nil || resumes != 2 {
		t.Fatalf("holder state recovered but the later pass stayed stuck: resumes=%d error=%v", resumes, err)
	}
}

func TestLandingOwnerComponentCadenceWiringBound(t *testing.T) {
	originalStart, originalTick, originalReport := batchOwnerCadenceStart, batchOwnerCadenceTick, batchOwnerCadenceReport
	t.Cleanup(func() {
		batchOwnerCadenceStart, batchOwnerCadenceTick, batchOwnerCadenceReport = originalStart, originalTick, originalReport
	})
	root, pass, release := landingOwnerRetryFixture(t)
	defer release()
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "mac-cli")
	starts, ticks, reports := 0, 0, 0
	batchOwnerCadenceStart = func(tick func()) { starts++; tick() }
	batchOwnerCadenceTick = func(gotRoot string, _ batchOwnerLease, _ func() time.Time) error {
		if gotRoot != root {
			t.Fatalf("cadence root=%q want %q", gotRoot, root)
		}
		ticks++
		return errors.New("injected cadence failure")
	}
	batchOwnerCadenceReport = func(err error) {
		if !strings.Contains(err.Error(), "injected cadence failure") {
			t.Fatalf("cadence report=%v", err)
		}
		reports++
	}
	if err := pass(); err != nil {
		t.Fatal(err)
	}
	if starts != 1 || ticks != 1 || reports != 1 {
		t.Fatalf("cadence starts=%d ticks=%d reports=%d", starts, ticks, reports)
	}
}

func TestRunPassCarriesGovernedSpendProjection(t *testing.T) {
	root := t.TempDir()
	seedClaimLaunchGoal(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goalPath := filepath.Join(root, "plans", "goals", "goal-a.md")
	data, err := os.ReadFile(goalPath)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse fixture goal: %v", problems)
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	file.Revision++
	effects := []goal.GoverningEffect{goal.EffectAuthorizeSpend}
	file.Obligation = &goal.GovernedObligation{Revision: file.Revision, BudgetRevision: file.Claimed.Revision,
		State: goal.ObligationEnforced, Owner: "fixture", AuthorizedBy: "fixture", AuthorizedAt: time.Now().UTC().Format(time.RFC3339),
		AuthorityOperation: "01ARZ3NDEKTSV4RRFFQ69G5FAZ-m-test-run-pass", ReviewPolicy: "C", ReviewOutcome: "human-approved",
		Effects: effects, AuthorizedEffects: effects,
		Assumptions: goal.ObligationAssumptions{Recurrence: goal.StandingSharedProcess,
			Platform: stdruntime.GOOS + "/" + stdruntime.GOARCH, ToolchainIdentity: stdruntime.Version(), SurfaceDigest: digest,
			MaxActiveJobs: 1, TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record"},
		Triggers: goal.HumanReviewTriggers{ValueJudgment: "no", Reversibility: "reversible", SevereHarm: "no",
			UnfamiliarApproach: "no", TestDiscrimination: "strong", CorrelatedAssumptionRisk: "no",
			AuthorityScopeChange: "no", DestructiveReach: "none"}}
	if err := os.WriteFile(goalPath, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "metasystem.conf", "plans/goals/goal-a.md")
	goalSyncMutationGit(t, root, "commit", "-qm", "enforce run-pass obligation")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	started := time.Now().UTC().Add(-3 * time.Minute)
	weightGeneration := uint64(0)
	store := &run.Store{Root: root, Now: func() time.Time { return started },
		AdmitGoverned: func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
			return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{GoalRevision: 3, ObligationRevision: file.Revision,
				WeightGeneration: &weightGeneration, Recurrence: governance.StandingSharedProcess,
				ExecutionCostMinutes: 30, AttemptOrdinal: 1,
				Budget:          goalbudget.Budget{ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1},
				BudgetStartedAt: started.Format(time.RFC3339), ExpectedAssumptions: governance.ObligationAssumptions{
					Recurrence: governance.StandingSharedProcess, Platform: stdruntime.GOOS + "/" + stdruntime.GOARCH,
					ToolchainIdentity: stdruntime.Version(), SurfaceDigest: digest, MaxActiveJobs: 1,
					TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record",
				}, AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: run.BreakerClosed}}, nil
		}}
	if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: "governed-pass", Kind: "suite",
		Display: "governed pass", Log: "artifacts/governed-pass.log", GoalId: "goal-a", ObligationRevision: file.Revision, StandingShared: true}); err != nil {
		t.Fatal(err)
	}
	if err := runPass(root, identity.Ref{Pid: 71, StartedAtSec: 72}); err != nil {
		t.Fatal(err)
	}
	concluded, err := store.Read("governed-pass")
	state, found, stateErr := obligationstate.Load(root, "goal-a", 3, file.Revision)
	if err != nil || stateErr != nil || concluded == nil || concluded.Status != run.StatusLaunchFailed || !found || len(state.Attempts) != 1 ||
		concluded.Governed.Exhausted || concluded.Governed.ExhaustionReason != "" {
		t.Fatalf("watcher run pass did not durably carry its projection: run=%+v state=%+v found=%t err=%v stateErr=%v", concluded, state, found, err, stateErr)
	}
}
