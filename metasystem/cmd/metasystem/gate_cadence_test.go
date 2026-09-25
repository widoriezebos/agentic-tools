package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGateCadenceTickRefusesEveryNonOwnerAndPrintsOneOwnerResult(t *testing.T) {
	originalAcquire, originalTick, originalClock := batchOwnerAcquire, cadenceTick, cadenceProductionClock
	t.Cleanup(func() {
		batchOwnerAcquire, cadenceTick, cadenceProductionClock = originalAcquire, originalTick, originalClock
	})
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	cadenceProductionClock = func() time.Time { return now }
	owner, seat := t.TempDir(), t.TempDir()
	for _, root := range []string{owner, seat} {
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("landing.batch-root="+owner+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runGateCadenceTick([]string{"--root", seat})
	})
	if code != 3 || !strings.Contains(stderr, cadenceNotLandingOwner) || !strings.Contains(stderr, owner) {
		t.Fatalf("non-owner code=%d stderr=%q", code, stderr)
	}
	acquires := 0
	batchOwnerAcquire = func(root string) (batchOwnerLease, error) {
		acquires++
		return batchOwnerLease{root: root, epoch: 7}, nil
	}
	cadenceTick = func(root string, held batchOwnerLease, clock func() time.Time) (cadenceTickOutput, error) {
		return cadenceTickOutput{Trunk: gaterun.CadenceTrunk{Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40)},
			Tick: gaterun.CadenceTickResult{Executed: true, Status: &goal.CadenceStatus{Trigger: goal.CadenceTriggerIdentityChanged, AttemptID: "attempt-1"}}}, nil
	}
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runGateCadenceTick([]string{"--root", owner})
	})
	if code != 0 || stderr != "" || acquires != 1 || strings.Count(strings.TrimSpace(stdout), "\n") != 0 ||
		!strings.Contains(stdout, "trigger=identity happened=executed") || !strings.Contains(stdout, "attempt=attempt-1") {
		t.Fatalf("owner code=%d stdout=%q stderr=%q acquires=%d", code, stdout, stderr, acquires)
	}
}

func TestGateCadenceTickClassifiesOnlyTickRefusalsNonZero(t *testing.T) {
	root := t.TempDir()
	code, _, stderr := captureCommandOutput(t, false, true, func() int {
		return runGateCadenceTick([]string{"--root", root})
	})
	if code != 3 || !strings.Contains(stderr, cadenceOwnerUnconfigured) {
		t.Fatalf("unconfigured code=%d stderr=%q", code, stderr)
	}
	if got := (cadenceRefusal{cadenceFetchRefused, errors.New("fetch").Error()}).Error(); got != "CADENCE_FETCH_REFUSED: fetch" {
		t.Fatalf("fetch refusal=%q", got)
	}
	if got := (cadenceRefusal{cadenceLedgerUnreadable, errors.New("ledger").Error()}).Error(); got != "CADENCE_LEDGER_UNREADABLE: ledger" {
		t.Fatalf("ledger refusal=%q", got)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("landing.batch-root="+root+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalAcquire, originalTick := batchOwnerAcquire, cadenceTick
	t.Cleanup(func() { batchOwnerAcquire, cadenceTick = originalAcquire, originalTick })
	batchOwnerAcquire = func(string) (batchOwnerLease, error) { return batchOwnerLease{}, nil }
	cadenceTick = func(string, batchOwnerLease, func() time.Time) (cadenceTickOutput, error) {
		return cadenceTickOutput{}, errors.New("raw ledger failure")
	}
	code, _, stderr = captureCommandOutput(t, false, true, func() int { return runGateCadenceTick([]string{"--root", root}) })
	if code != 3 || !strings.Contains(stderr, cadenceLedgerUnreadable) || strings.Contains(stderr, "\nraw ledger failure") {
		t.Fatalf("raw tick failure code=%d stderr=%q", code, stderr)
	}
}

func TestCadencePreparationDoesNotRequireClaimedGoal(t *testing.T) {
	request := cadencePreparationRequest("landing", "tree")
	if request.Root != "landing" || request.Tree != "tree" || request.GoalID != "" || request.Purpose != testpolicy.PurposeCadence ||
		request.Mode != testpolicy.ModeDeep || !request.CadencePreflight {
		t.Fatalf("cadence preparation request=%+v", request)
	}
	accounts, err := testingPreparationAccountsToGoal(request)
	if err != nil || accounts {
		t.Fatalf("cadence preflight accounts-to-goal=%t err=%v", accounts, err)
	}
}

func TestCadenceResultNamesTriggerForJoinedAndOccupiedClaims(t *testing.T) {
	for _, outcome := range []goal.CadenceClaimOutcome{goal.CadenceClaimJoined, goal.CadenceClaimOccupied} {
		trigger, happened, attempt := cadenceResultWords(gaterun.CadenceTickResult{ClaimOutcome: outcome, Trigger: goal.CadenceTriggerWeightDue})
		if trigger != "weight" || happened != "joined" || attempt != "" {
			t.Fatalf("outcome=%s trigger=%q happened=%q attempt=%q", outcome, trigger, happened, attempt)
		}
	}
	trigger, happened, attempt := cadenceResultWords(gaterun.CadenceTickResult{ClaimOutcome: goal.CadenceClaimComplete,
		Status: &goal.CadenceStatus{Trigger: goal.CadenceTriggerForcedWindow, AttemptID: "attempt-terminal"}})
	if trigger != "six-hour" || happened != "terminal-read" || attempt != "attempt-terminal" {
		t.Fatalf("complete trigger=%q happened=%q attempt=%q", trigger, happened, attempt)
	}
}

func TestCadenceRunStoreRereadsLandingOwnerEpoch(t *testing.T) {
	originalRequire := batchOwnerRequire
	t.Cleanup(func() { batchOwnerRequire = originalRequire })
	live := true
	batchOwnerRequire = func(batchOwnerLease) error {
		if live {
			return nil
		}
		return errors.New("owner lease moved")
	}
	store := cadenceRunStore("landing-root", batchOwnerLease{epoch: 9}, func() time.Time { return time.Unix(1, 0) })
	if epoch, ok := store.CurrentEpoch(); !ok || epoch == nil || *epoch != 9 {
		t.Fatalf("live epoch=%v ok=%t", epoch, ok)
	}
	live = false
	if epoch, ok := store.CurrentEpoch(); ok || epoch != nil {
		t.Fatalf("stale epoch=%v ok=%t", epoch, ok)
	}
}

func TestCadenceRevalidationDefersMissingEngineBuildUntilClaimedRun(t *testing.T) {
	originalIdentity, originalRetained := cadenceBuildIdentity, cadenceRetainedEngineDigest
	t.Cleanup(func() {
		cadenceBuildIdentity, cadenceRetainedEngineDigest = originalIdentity, originalRetained
	})
	cadenceBuildIdentity = func(context.Context, gittree.Workspace, string, string, []string) (string, error) {
		return "build-identity", nil
	}
	cadenceRetainedEngineDigest = func(testingPreparation, []proofrun.Attempt, string, bool) (string, error) {
		return "", errors.New("no retained engine")
	}
	identity, digest, exact, err := cadenceCandidateEngineIdentity(testingPreparation{ProjectRoot: "root"}, gaterun.CadenceTrunk{Tree: strings.Repeat("a", 40)}, nil)
	if err != nil || identity != "build-identity" || exact || digest != bytesSHA256([]byte("cadence-missing-engine-evidence\x00build-identity")) {
		t.Fatalf("identity=%q digest=%q exact=%t err=%v", identity, digest, exact, err)
	}
}

func TestCadenceRevalidationDoesNotBuildBeforeClaim(t *testing.T) {
	data, err := os.ReadFile("gate_cadence.go")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(data), "func revalidateCadence(")
	end := strings.Index(string(data), "func cadencePreparationRequest(")
	if start < 0 || end <= start || strings.Contains(string(data[start:end]), "buildCandidateEngine(") {
		t.Fatal("cadence pre-claim revalidation invokes the candidate-engine build")
	}
}

type cadenceDeclaredWorkspace struct {
	workspace gittree.Workspace
	closed    *int
}

func (candidate *cadenceDeclaredWorkspace) Workspace() gittree.Workspace { return candidate.workspace }
func (candidate *cadenceDeclaredWorkspace) Close() error {
	*candidate.closed++
	return nil
}

func TestCadenceRevalidationRetainsCurrentWorkerPolicyAcrossRepreparation(t *testing.T) {
	const childEnvironment = "GO_WANT_CADENCE_WORKER_POLICY_CHILD"
	if os.Getenv(childEnvironment) != "1" {
		t.Parallel()
		shim := t.TempDir()
		denialLog := filepath.Join(t.TempDir(), "denied-git.log")
		if err := os.WriteFile(denialLog, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(filepath.Join(shim, "git"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$CADENCE_GIT_DENIAL_LOG\"\nexit 97\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		child := exec.Command(os.Args[0], "-test.run=^TestCadenceRevalidationRetainsCurrentWorkerPolicyAcrossRepreparation$", "-test.count=1")
		child.Env = append(os.Environ(), "PATH="+shim+string(os.PathListSeparator)+os.Getenv("PATH"),
			"CADENCE_GIT_DENIAL_LOG="+denialLog, childEnvironment+"=1")
		output, err := child.CombinedOutput()
		if err != nil {
			t.Fatalf("Git-denied cadence child: %v\n%s", err, output)
		}
		if !strings.Contains("\n"+string(output), "\nPASS\n") {
			t.Fatalf("Git-denied cadence child did not report PASS: %q", output)
		}
		denied, err := os.ReadFile(denialLog)
		if err != nil || string(denied) != "--version\n" {
			t.Fatalf("unexpected Git calls in cadence child: %q, %v", denied, err)
		}
		t.Log("Git-denied cadence child passed with only its denial probe")
		return
	}
	probe := exec.Command("git", "--version")
	if err := probe.Run(); err == nil || probe.ProcessState == nil || probe.ProcessState.ExitCode() != 97 {
		t.Fatalf("Git denial probe did not exit 97: %v", err)
	}
	root := t.TempDir()
	source := []byte("current\n")
	configuration := []byte("testing.workers=4\n" + proofrun.AdmissionCapKey + "=2\n")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), configuration, 0o600); err != nil {
		t.Fatal(err)
	}
	wantWorkers := 4
	if inherited, found := os.LookupEnv(proofrun.TestWorkersEnvironment); found && inherited != "" {
		ceiling, err := strconv.Atoi(inherited)
		if err != nil || ceiling < 1 {
			t.Fatalf("inherited %s must be a positive integer, got %q", proofrun.TestWorkersEnvironment, inherited)
		}
		wantWorkers = min(wantWorkers, ceiling)
	}
	tree := strings.Repeat("e", 40)
	var beds []string
	closed := 0
	openCandidate := func(projectRoot, candidateTree string) (proofrun.CandidateWorkspace, error) {
		if projectRoot != root || candidateTree != tree {
			return nil, errors.New("unexpected cadence candidate tree or root")
		}
		bed := t.TempDir()
		for name, data := range map[string][]byte{"source.txt": source, "metasystem.conf": configuration} {
			if err := os.WriteFile(filepath.Join(bed, name), data, 0o600); err != nil {
				return nil, err
			}
		}
		beds = append(beds, bed)
		return &cadenceDeclaredWorkspace{workspace: gittree.Workspace{Dir: bed, RawSource: func(call gittree.RawRequest) gittree.RawResult {
			t.Errorf("Git requested in declared candidate bed: %v", call.Args)
			return gittree.RawResult{Err: errors.New("Git denied"), ExitCode: 97}
		}}, closed: &closed}, nil
	}
	share := min(2, wantWorkers)
	group := testpolicy.Group{ID: "section/deep", Kind: "section", Adapter: "command", CWD: ".", Inputs: []string{"source.txt"},
		Obligations: []string{"cadence-deep"}, Platforms: []string{"any"}, TargetMS: 1,
		Resources: testpolicy.GroupResources{Workers: &share}, Argv: []string{"sh", "-c", "printf launched > native-launched"}, Format: "exit-status"}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeCadence, RequestedMode: testpolicy.ModeDeep, RequiredMode: testpolicy.ModeDeep,
		ExecutedMode: testpolicy.ModeDeep, RequiredGroups: []string{group.ID}, SelectedGroups: []string{group.ID},
		Stages: []testpolicy.Stage{{ID: "deep", Groups: []string{group.ID}}}}
	digest := strings.Repeat("a", 64)
	buildIdentity, engineDigest := strings.Repeat("b", 40), strings.Repeat("c", 64)
	prepared := testingPreparation{Installation: root, ControlRoot: root, ProjectRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"),
		BaseCommit: "HEAD", PolicyBaseCommit: "HEAD", CandidateTree: tree, EffectiveContract: contract, Plan: plan,
		ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, BehaviorPolicyDigest: digest, JudgeKey: "cadence-worker-policy",
		Environment: os.Environ()}
	resolved := prepared
	if _, err := resolveTestingPreparationWorkerPolicy(&resolved); err != nil || resolved.Workers != wantWorkers || resolved.AdmissionMaximum != 2 {
		t.Fatalf("resolved cadence policy workers=%d admission=%d err=%v", resolved.Workers, resolved.AdmissionMaximum, err)
	}
	request := testingRunRequest(resolved, "cadence-retained", "", "", engineDigest, buildIdentity)
	request.WithCandidateOpener(openCandidate)
	identities, metadata, launches, err := proofrun.PrepareGroupExecutionIdentities(context.Background(), request)
	if err != nil || launches != 0 {
		t.Fatalf("prepare retained cadence metadata launches=%d err=%v", launches, err)
	}
	zero := 0
	preparedGroup := metadata[group.ID]
	result := proofrun.NewTestResult(request)
	if result.WorkerPolicyVersion != proofrun.TestWorkerPolicyVersion || result.Workers != wantWorkers ||
		result.AdmissionMaximum == nil || *result.AdmissionMaximum != 2 {
		t.Fatalf("retained cadence policy=%+v", result)
	}
	result.Groups = []proofrun.GroupResult{{ID: group.ID, Kind: group.Kind, Obligations: group.Obligations,
		IdentityVersion: proofrun.GroupExecutionIdentityVersion, ExecutionIdentity: identities[group.ID],
		InputDigest: preparedGroup.InputDigest, InputManifest: []string{"source.txt"}, EnvironmentDigest: preparedGroup.EnvironmentDigest,
		ToolIdentities: preparedGroup.ToolIdentities, ExecutableDigests: preparedGroup.ExecutableDigests, Argv: preparedGroup.Argv,
		Expected: preparedGroup.Expected, Status: "passed", NativeLaunched: true, NativeExitStatus: &zero,
		CollectionComplete: true, EndedAt: time.Now().UTC().Format(time.RFC3339Nano)}}
	result.RecomputeDelivery()
	attempt := proofrun.Attempt{SchemaVersion: proofrun.IdentityAttemptSchemaVersion, AttemptID: request.AttemptID,
		StartedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), Terminal: &proofrun.AttemptTerminal{Result: proofrun.TerminalSuccess},
		PendingTestGroups: map[string]string{group.ID: identities[group.ID]}, TestResult: &result}
	repreparations := 0
	dependencies := cadenceRevalidationDependencies{
		openCandidate: openCandidate,
		readAttempts:  func(string) ([]proofrun.Attempt, error) { return []proofrun.Attempt{attempt}, nil },
		buildIdentity: func(context.Context, gittree.Workspace, string, string, []string) (string, error) {
			return buildIdentity, nil
		},
		retainedDigest: func(testingPreparation, []proofrun.Attempt, string, bool) (string, error) { return engineDigest, nil },
		prepare: func(request testingSelectionRequest) (testingPreparation, error) {
			repreparations++
			if request.Tree != tree || request.Purpose != testpolicy.PurposeCadence || !request.CadencePreflight {
				return testingPreparation{}, errors.New("unexpected cadence re-preparation request")
			}
			return prepared, nil
		},
	}

	assertReused := func(name string, input testingPreparation, wantRepreparations int) {
		t.Helper()
		revalidation, err := revalidateCadenceWith(root, input, gaterun.CadenceTrunk{Commit: "HEAD", Tree: tree}, []string{group.ID}, dependencies)
		if err != nil || len(revalidation.Groups) != 1 || revalidation.Groups[0].Status != "reused" ||
			revalidation.Groups[0].ReuseAttempt != attempt.AttemptID || repreparations != wantRepreparations {
			t.Fatalf("%s revalidation=%+v repreparations=%d err=%v", name, revalidation, repreparations, err)
		}
		if _, err := os.Stat(filepath.Join(root, "native-launched")); !os.IsNotExist(err) {
			t.Fatalf("%s revalidation launched the retained native group: %v", name, err)
		}
		if closed != len(beds) {
			t.Fatalf("%s detached candidate beds opened=%d closed=%d", name, len(beds), closed)
		}
		for _, bed := range beds {
			if _, err := os.Stat(filepath.Join(bed, "native-launched")); !os.IsNotExist(err) {
				t.Fatalf("%s revalidation launched the retained native group in %s: %v", name, bed, err)
			}
		}
	}
	assertReused("same-tree", prepared, 0)
	changed := prepared
	changed.CandidateTree = strings.Repeat("d", 40)
	assertReused("changed-tree", changed, 1)
}
