package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const (
	publicApplicationInstalledBinary  = "METASYSTEM_PUBLIC_APPLICATION_INSTALLED_BINARY"
	publicApplicationInstalledDigest  = "METASYSTEM_PUBLIC_APPLICATION_INSTALLED_SHA256"
	publicApplicationOrdinaryIsolated = "METASYSTEM_PUBLIC_APPLICATION_ORDINARY_ISOLATED"
	publicApplicationOrdinaryResult   = "METASYSTEM_PUBLIC_APPLICATION_ORDINARY_RESULT"
)

type publicApplicationRun struct {
	label      string
	configured int
	workers    int
	status     int
	output     string
	tree       string
	result     *proofrun.TestResult
	children   []identity.Ref
	maximum    int
	processes  int
	resultPath string
	attemptID  string
	nativeLog  string
	wantDigest string
	nativeCWD  string
}

// The process boundary keeps Git denial local while the parallel parent and
// other command tests use their own subprocess environments.
func testOrdinaryPublicApplicationCancellation(t *testing.T) {
	if os.Getenv(publicApplicationOrdinaryIsolated) != "1" {
		deny, err := filepath.Abs(filepath.Join("..", "..", "internal", "testgit", "testdata", "deny-bin"))
		if err != nil {
			t.Fatal(err)
		}
		log := filepath.Join(t.TempDir(), "git-denied.log")
		if err := os.WriteFile(log, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		resultPath := filepath.Join(t.TempDir(), "ordinary-result.json")
		command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCommandApplicationWorkerAllowancePublicDelivery/OrdinaryCancellationBridge$", "-test.v")
		command.Env = append(os.Environ(), "PATH="+deny+string(os.PathListSeparator)+os.Getenv("PATH"),
			"METASYSTEM_TEST_GIT_DENIED_LOG="+log, publicApplicationOrdinaryIsolated+"=1", publicApplicationOrdinaryResult+"="+resultPath)
		output, runErr := command.CombinedOutput()
		denied, readErr := os.ReadFile(log)
		if readErr != nil && !os.IsNotExist(readErr) {
			t.Fatal(readErr)
		}
		data, resultErr := os.ReadFile(resultPath)
		if runErr != nil || len(denied) != 0 || resultErr != nil {
			t.Fatalf("ordinary cancellation isolation exit=%v denial-log=%q result-read=%v output=%s", runErr, denied, resultErr, output)
		}
		var result proofrun.TestResult
		if err := json.Unmarshal(data, &result); err != nil || proofrun.ValidateTestResult(result) != nil || result.Delivery.Sufficient {
			t.Fatalf("ordinary cancellation JSON is not valid insufficient evidence: err=%v result=%+v", err, result)
		}
		t.Logf("ORDINARY_CANCELLATION_BRIDGE subprocess-exit=0 denial-log=%q result-json=%s\n%s", denied, data, output)
		return
	}
	testOrdinaryPublicApplicationCancellationIsolated(t)
}

func testOrdinaryPublicApplicationCancellationIsolated(t *testing.T) {
	denialLog := os.Getenv("METASYSTEM_TEST_GIT_DENIED_LOG")
	if denialLog == "" || os.Getenv(publicApplicationOrdinaryResult) == "" {
		t.Fatal("ordinary cancellation has no isolated Git denial or result path")
	}
	repository := newProofAdmissionRepositoryFixture(t, time.Now().UTC(), true)
	root := repository.root
	state := t.TempDir()
	readyPath := filepath.Join(state, "public-ready.fifo")
	internalReady := filepath.Join(state, "internal-ready.fifo")
	release := filepath.Join(state, "public-release.fifo")
	for _, path := range []string{readyPath, internalReady, release,
		filepath.Join(state, "release-1.fifo"), filepath.Join(state, "release-2.fifo")} {
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ready, err := os.OpenFile(readyPath, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer ready.Close()
	internal, err := os.OpenFile(internalReady, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer internal.Close()
	releasePipe, err := os.OpenFile(release, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer releasePipe.Close()

	workers := 2
	contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{{ID: "application", Paths: []string{"metasystem/app/**", "metasystem/scripts/check.sh", "metasystem/testing.json"},
			Standard: []string{"application-workers"}, Critical: []string{"application-workers-observed"}}},
		Groups: []testpolicy.Group{{ID: "application-workers", Phase: "acceptance", EnvironmentMode: "inherit", Kind: "component", Adapter: "command", CWD: "metasystem",
			Inputs: []string{"metasystem/app/worker-mode.txt", "metasystem/scripts/check.sh"}, Outputs: []string{"metasystem/reports-workers"},
			Tools:       []testpolicy.Tool{{ID: "shell", Executable: "sh", VersionArgs: []string{"-c", "printf public-application-shell"}}},
			Obligations: []string{"application-workers-observed"}, Platforms: []string{"any"}, TargetMS: 1000,
			Resources: testpolicy.GroupResources{Workers: new(int)},
			Env: map[string]string{"PUBLIC_APPLICATION_STATE": state, "PUBLIC_APPLICATION_INTERNAL_READY": internalReady,
				"PUBLIC_APPLICATION_READY": readyPath, "PUBLIC_APPLICATION_RELEASE": release,
				"PUBLIC_APPLICATION_NATIVE_COUNTER": filepath.Join(state, "native.log"),
				"PUBLIC_APPLICATION_NATIVE_CWD":     filepath.Join(state, "native.cwd")},
			Argv: []string{"sh", "scripts/check.sh", "app/worker-mode.txt", "reports-workers"}, Reports: []string{"metasystem/reports-workers"}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: "metasystem/reports-workers/result.xml", Classname: "public-application", Name: "allowance"}}}},
		Always:  testpolicy.Always{Canary: []string{"application-workers"}, Standard: []string{"application-workers"}},
		Unknown: []string{"application-workers"}, Cadence: []string{"application-workers"}}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "testing.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := testpolicy.Load(filepath.Join(root, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := testpolicy.Select(parsed, testpolicy.SelectionRequest{ChangedPaths: []string{"metasystem/app/worker-mode.txt"}, RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery})
	if err != nil || len(plan.SelectedGroups) != 1 || plan.SelectedGroups[0] != "application-workers" {
		t.Fatalf("ordinary public plan=%+v err=%v", plan, err)
	}

	candidate := newOrdinaryCandidateFixture(t, ordinaryCandidateFixtureOptions{externalDenialLog: denialLog, tracked: []byte("ordinary candidate\n")})
	candidate.root, candidate.installation = root, root
	candidate.writeFiles()
	openCandidate := func(projectRoot, tree string) (proofrun.CandidateWorkspace, error) {
		if projectRoot != root || tree != ordinaryProjectTree {
			return nil, fmt.Errorf("candidate opener root=%q tree=%q", projectRoot, tree)
		}
		candidate.prepareDetached(tree)
		for _, file := range []struct {
			path, body string
			mode       os.FileMode
		}{
			{"app/worker-mode.txt", "cancel\n", 0o644}, {"scripts/check.sh", publicApplicationWorkerScript, 0o755},
		} {
			path := filepath.Join(candidate.detachedRoot, "metasystem", file.path)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			if err := testexec.WriteFile(path, []byte(file.body), file.mode); err != nil {
				return nil, err
			}
		}
		candidate.queueBed(tree)
		return candidate.openBed(projectRoot, tree)
	}
	proofIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "testing", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, CandidateTree: ordinaryProjectTree,
		GoalID: "standing-validation", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 30,
		Identity: proofIdentity, Launcher: launcher, Now: time.Now().UTC()}))
	if err != nil || decision.Disposition != proofrun.DispositionExecuted {
		t.Fatalf("ordinary proof admission=%+v err=%v", decision, err)
	}
	request := proofrun.TestRunRequest{ControlRoot: root, ProjectRoot: root, CandidateTree: ordinaryProjectTree,
		BaseCommit: ordinaryBaseCommit, PolicyBaseCommit: ordinaryBaseCommit, Contract: parsed, Plan: plan,
		AttemptID: attempt.AttemptID, Environment: gittree.ScrubbedEnviron(), LogRoot: filepath.Join(root, "artifacts", "ordinary-public-logs"),
		ContractDigest: bytesSHA256(data), BaseContractDigest: bytesSHA256(data),
		PolicyEngineDigest: candidate.policyDigest, CandidateEngineDigest: candidate.policyDigest,
		CandidateEngineBuildIdentity: ordinaryEngineTree, BehaviorPolicyDigest: strings.Repeat("b", 64),
		Workers: workers, AdmissionMaximum: proofAdmissionFixtureMax}
	request.WithCandidateOpener(openCandidate)
	ids, prepared, launches, err := proofrun.PrepareGroupExecutionIdentities(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ComponentIdentities, request.PreparedGroups, request.PreparationLaunches = ids, prepared, launches
	resultPath := os.Getenv(publicApplicationOrdinaryResult)
	type outcome struct {
		result proofrun.TestResult
		status int
		err    error
	}
	runnerDone := make(chan outcome, 1)
	runnerFinished := make(chan struct{})
	var retained *proofrun.TestResult
	reads := repository.reads()
	_, _, launch := terminalCommitFixtureAt(t, root, attempt)
	terminalRelease := filepath.Join(state, "terminal-release.fifo")
	if err := syscall.Mkfifo(terminalRelease, 0o600); err != nil {
		t.Fatal(err)
	}
	terminalPipe, err := os.OpenFile(terminalRelease, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer terminalPipe.Close()
	terminalDone := make(chan struct {
		status int
		output string
	}, 1)
	go func() {
		commit := testingTerminalCommitWithReads(resultPath, &retained, &reads)
		status, output := launch([]string{"sh", "-c", "IFS= read -r signal < \"$1\"; exit 1", "ordinary-terminal", terminalRelease}, func(completion proofrun.CompletionContext, receipt json.RawMessage) error {
			<-runnerFinished
			return commit(completion, receipt)
		})
		terminalDone <- struct {
			status int
			output string
		}{status, output}
	}()
	workerContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cancelOnRecordedIntent(workerContext, cancel, root, attempt.AttemptID)
	go func() {
		result, status, runErr := proofrun.RunTestPlan(workerContext, request)
		if validationErr := proofrun.ValidateTestResult(result); validationErr == nil {
			if writeErr := writePrivateJSON(resultPath, result); writeErr != nil {
				runErr = errors.Join(runErr, writeErr)
			}
		} else {
			runErr = errors.Join(runErr, validationErr)
		}
		runnerDone <- outcome{result, status, runErr}
		close(runnerFinished)
	}()

	readyByte := make(chan error, 1)
	go func() {
		one := make([]byte, 1)
		_, readErr := io.ReadFull(ready, one)
		if readErr == nil && one[0] != 'x' {
			readErr = fmt.Errorf("readiness byte %q", one)
		}
		readyByte <- readErr
	}()
	select {
	case err := <-readyByte:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("ordinary public command did not become ready")
	}
	children := publicApplicationChildren(t, state, workers)
	launcherDeadline := time.After(10 * time.Second)
	for {
		liveAttempt, readErr := proofrun.ReadAttempt(root, attempt.AttemptID)
		if readErr == nil && len(liveAttempt.ProcessKeys) != 0 {
			record, recordErr := proofrun.ReadProcessRecord(root, liveAttempt.ProcessKeys[0])
			if recordErr == nil && record.Status == proofrun.StatusRunning {
				break
			}
			readErr = recordErr
		}
		select {
		case <-time.After(10 * time.Millisecond):
		case <-launcherDeadline:
			t.Fatalf("proof launcher did not start: attempt=%+v err=%v", liveAttempt, readErr)
		}
	}
	if active := activePublicApplicationAttempt(t, root, ordinaryProjectTree); active != attempt.AttemptID {
		t.Fatalf("ready worker attempt=%s admitted=%s", active, attempt.AttemptID)
	}
	if err := proofrun.RequestCancellation(root, attempt.AttemptID, "ordinary public application cancellation"); err != nil {
		t.Fatal(err)
	}
	var finished outcome
	select {
	case finished = <-runnerDone:
	case <-time.After(60 * time.Second):
		t.Fatal("ordinary public cancellation was not observed by the runner")
	}
	if finished.status == 0 || finished.result.Delivery.Sufficient || len(finished.result.Groups) != 1 || finished.result.Groups[0].Status != "cancelled" || !finished.result.Groups[0].NativeLaunched {
		t.Fatalf("ordinary cancelled result status=%d err=%v result=%+v", finished.status, finished.err, finished.result)
	}
	if err := proofrun.ValidateTestResult(finished.result); err != nil {
		t.Fatalf("ordinary cancelled result invalid: %v", err)
	}
	if _, err := terminalPipe.Write([]byte("x\n")); err != nil {
		t.Fatal(err)
	}
	for _, child := range children {
		exact, state, err := (identity.KernelProber{}).Probe(child.Pid)
		if err != nil || state == identity.Alive && identity.SameIdentity(exact, child) && !exact.Zombie {
			t.Fatalf("cancelled child remains executable: ref=%+v exact=%+v state=%s err=%v", child, exact, state, err)
		}
	}
	terminal := <-terminalDone
	if terminal.status == 0 || retained == nil || retained.AttemptID != attempt.AttemptID {
		t.Fatalf("ordinary terminal did not retain the cancelled worker result: status=%d output=%s retained=%+v", terminal.status, terminal.output, retained)
	}
	stored, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalCancelled || stored.TestResult == nil || len(proofrun.CommittedDeliveryReceipt(stored)) != 0 {
		t.Fatalf("ordinary cancelled attempt committed delivery: %+v err=%v", stored, err)
	}
	candidate.assertDrained()
	t.Logf("ORDINARY_CANCELLATION worker-status=%d launcher-status=%d workers=%d attempt=%s result-json=%s", finished.status, terminal.status, workers, attempt.AttemptID, resultPath)
}

func TestCommandApplicationWorkerAllowancePublicDelivery(t *testing.T) {
	t.Parallel()

	t.Run("OrdinaryCancellationBridge", testOrdinaryPublicApplicationCancellation)
	t.Run("NativeGitCommittedCandidateDelivery", func(t *testing.T) {
		fixture := newPortableProofFixture(t)
		preparePublicApplicationWorkerBase(t, fixture)
		observed := runPublicApplicationPhase(t, fixture, "committed-candidate-workers-2", "green", 2, false, true)
		requirePublicApplicationGreen(t, fixture, observed)
		controlRoot, err := canonicalProofRoot(fixture.root)
		if err != nil {
			t.Fatal(err)
		}
		attemptPath, err := proofrun.AttemptPath(controlRoot, observed.attemptID)
		if err != nil {
			t.Fatal(err)
		}
		beforeAttempt, err := os.ReadFile(attemptPath)
		if err != nil {
			t.Fatal(err)
		}
		beforeAttempts, err := proofrun.ReadAttempts(controlRoot)
		if err != nil {
			t.Fatal(err)
		}
		beforeBuilds, beforeNative := fixture.counts()
		forecast, err := forecastTestingSelection(fixture.root, costSelection{
			ID: "tip:" + observed.tree, Kind: "tip", Tree: observed.tree, GoalID: "portable",
			Requirements: []string{"application-workers"}}, 2)
		if err != nil || len(forecast.Groups) != 1 || forecast.Groups[0].GroupID != "application-workers" ||
			forecast.Groups[0].Status != "reusable" || !forecast.Groups[0].IdentityKnown || forecast.Groups[0].Reason != "" {
			t.Fatalf("committed public result forecast=%+v err=%v", forecast, err)
		}
		afterBuilds, afterNative := fixture.counts()
		afterAttempts, err := proofrun.ReadAttempts(controlRoot)
		if err != nil {
			t.Fatal(err)
		}
		afterAttempt, err := os.ReadFile(attemptPath)
		if err != nil {
			t.Fatal(err)
		}
		if afterBuilds != beforeBuilds || !maps.Equal(afterNative, beforeNative) ||
			len(afterAttempts) != len(beforeAttempts) || !bytes.Equal(afterAttempt, beforeAttempt) {
			t.Fatalf("forecast changed retained proof: builds=%d/%d native=%v/%v attempts=%d/%d bytes-equal=%t",
				beforeBuilds, afterBuilds, beforeNative, afterNative, len(beforeAttempts), len(afterAttempts), bytes.Equal(beforeAttempt, afterAttempt))
		}
		t.Logf("PUBLIC_APPLICATION_FORECAST status=%s identityKnown=%t reason=%q builds=%d native=%v attempts=%d retainedBytes=%d",
			forecast.Groups[0].Status, forecast.Groups[0].IdentityKnown, forecast.Groups[0].Reason,
			beforeBuilds, beforeNative, len(beforeAttempts), len(beforeAttempt))
	})
	installed, digest := os.Getenv(publicApplicationInstalledBinary), os.Getenv(publicApplicationInstalledDigest)
	if installed == "" && digest == "" {
		t.Logf("INSTALLED_PUBLIC_APPLICATION_REPLAY enabled=false inputs=%s,%s", publicApplicationInstalledBinary, publicApplicationInstalledDigest)
		return
	}
	if installed == "" || digest == "" {
		t.Fatalf("installed replay requires both %s and %s", publicApplicationInstalledBinary, publicApplicationInstalledDigest)
	}
	t.Run("exact-installed-binary-private-synthetic-authority", func(t *testing.T) {
		fixture := newInstalledPublicApplicationFixture(t, installed, digest)
		preparePublicApplicationWorkerBase(t, fixture)

		for _, workers := range []int{1, 2, 3} {
			observed := runPublicApplicationPhase(t, fixture, fmt.Sprintf("green-workers-%d", workers), "green", workers, false, false)
			requirePublicApplicationGreen(t, fixture, observed)
		}

		red := runPublicApplicationPhase(t, fixture, "retained-red-workers-2", "red", 2, false, false)
		requirePublicApplicationRed(t, fixture, red)
		repaired := runPublicApplicationPhase(t, fixture, "repaired-red-workers-2", "repaired", 2, false, false)
		requirePublicApplicationGreen(t, fixture, repaired)

		cancelled := runPublicApplicationPhase(t, fixture, "cancel-drain-workers-2-via-request-api", "cancel", 2, true, false)
		requirePublicApplicationCancelled(t, fixture, cancelled)

		recovered := runPublicApplicationPhase(t, fixture, "green-after-cancel-workers-2", "recovered", 2, false, false)
		requirePublicApplicationGreen(t, fixture, recovered)
	})
}

func preparePublicApplicationWorkerBase(t *testing.T, fixture *portableProofFixture) {
	t.Helper()
	prefix, err := fixtureProjectPrefix(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	wholeAllowance := 0
	input := filepath.ToSlash(filepath.Join(prefix, "app", "worker-mode.txt"))
	script := filepath.ToSlash(filepath.Join(prefix, "scripts", "check.sh"))
	candidateBuildScript := filepath.ToSlash(filepath.Join(prefix, "scripts", "agents", "go-build.sh"))
	contract := filepath.ToSlash(filepath.Join(prefix, "testing.json"))
	configuration := filepath.ToSlash(filepath.Join(prefix, "metasystem.conf"))
	reportRoot := filepath.ToSlash(filepath.Join(prefix, "reports-workers"))
	cwd := prefix
	if cwd == "" {
		cwd = "."
	}
	fixture.contract = testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{{ID: "application", Paths: []string{filepath.ToSlash(filepath.Join(prefix, "app/**")), script, candidateBuildScript, contract, configuration},
			Standard: []string{"application-workers"}, Critical: []string{"application-workers-observed"}}},
		Groups: []testpolicy.Group{{ID: "application-workers", Phase: "acceptance", EnvironmentMode: "inherit", Kind: "component", Adapter: "command", CWD: cwd,
			Inputs: []string{input, script}, Outputs: []string{reportRoot},
			Tools:       []testpolicy.Tool{{ID: "shell", Executable: "sh", VersionArgs: []string{"-c", "printf public-application-shell"}}},
			Obligations: []string{"application-workers-observed"}, Platforms: []string{"any"}, TargetMS: 1000,
			Resources: testpolicy.GroupResources{Workers: &wholeAllowance},
			Env:       map[string]string{"PUBLIC_APPLICATION_NATIVE_COUNTER": fixture.nativeCounter},
			Argv:      []string{"sh", "scripts/check.sh", "app/worker-mode.txt", "reports-workers"}, Reports: []string{reportRoot}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: filepath.ToSlash(filepath.Join(reportRoot, "result.xml")), Classname: "public-application", Name: "allowance"}}}},
		Always:  testpolicy.Always{Canary: []string{"application-workers"}, Standard: []string{"application-workers"}},
		Unknown: []string{"application-workers"}, Cadence: []string{"application-workers"}}
	fixture.writeContract()
	fixture.write("app/worker-mode.txt", "seed\n", 0o644)
	fixture.write("scripts/check.sh", publicApplicationWorkerScript, 0o755)
	fixture.write("metasystem.conf", "metasystem.runtimes=fake\ntesting.contract=testing.json\ntesting.workers=1\n"+
		proofrun.AdmissionCapKey+"="+strconv.Itoa(proofAdmissionFixtureMax)+"\n", 0o644)
	fixture.commit("establish private public application worker authority")
	base := fixture.git("rev-parse", "HEAD")
	fixture.git("update-ref", "refs/remotes/origin/main", base)
	fixture.git("update-ref", goal.LocalLedgerBranch, base)
	fixture.git("update-ref", goal.AcceptedRef, base)
	if fixture.contract.Groups[0].Resources.Workers == nil || *fixture.contract.Groups[0].Resources.Workers != 0 {
		t.Fatal("public application group does not reserve the complete attempt allowance")
	}
	if fixture.installedSnapshot != "" {
		fixture.write("scripts/agents/go-build.sh", publicApplicationInstalledCandidateScript(fixture.installedSnapshot, fixture.installedDigest), 0o755)
	}
}

func runPublicApplicationPhase(t *testing.T, fixture *portableProofFixture, label, mode string, configuredWorkers int, cancel, mutateLive bool) publicApplicationRun {
	t.Helper()
	state := t.TempDir()
	internalReady := filepath.Join(state, "internal-ready.fifo")
	publicReady := filepath.Join(state, "public-ready.fifo")
	publicRelease := filepath.Join(state, "public-release.fifo")
	for _, path := range []string{internalReady, publicReady, publicRelease} {
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fixture.write("app/worker-mode.txt", mode+"\n", 0o644)
	fixture.write("metasystem.conf", "metasystem.runtimes=fake\ntesting.contract=testing.json\ntesting.workers="+strconv.Itoa(configuredWorkers)+"\n"+
		proofrun.AdmissionCapKey+"="+strconv.Itoa(proofAdmissionFixtureMax)+"\n", 0o644)
	limits, err := resolveProofRunLimits(filepath.Join(fixture.root, "metasystem.conf"))
	if err != nil {
		t.Fatalf("%s resolve effective worker allowance: %v", label, err)
	}
	workers := limits.workers
	for index := 1; index <= workers; index++ {
		if err := syscall.Mkfifo(filepath.Join(state, "release-"+strconv.Itoa(index)+".fifo"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fixture.contract.Groups[0].Env = map[string]string{
		"PUBLIC_APPLICATION_STATE": state, "PUBLIC_APPLICATION_INTERNAL_READY": internalReady,
		"PUBLIC_APPLICATION_READY": publicReady, "PUBLIC_APPLICATION_RELEASE": publicRelease,
		"PUBLIC_APPLICATION_NATIVE_COUNTER": fixture.nativeCounter,
		"PUBLIC_APPLICATION_NATIVE_CWD":     fixture.nativeCounter + ".cwd",
	}
	fixture.writeContract()
	tree := fixture.commit(label)
	baseArgs := []string{"--root", fixture.root, "--goal", "portable", "--tree", tree, "--mode", "auto", "--purpose", "delivery"}
	planStatus, planOutput := fixture.command(append([]string{"test", "plan"}, baseArgs...)...)
	if planStatus != 0 || !strings.Contains(planOutput, "groups=application-workers") {
		t.Fatalf("%s public plan status=%d output=%s", label, planStatus, planOutput)
	}
	resultPath := filepath.Join(t.TempDir(), label+".json")
	runArgs := append(append([]string{"test", "run"}, baseArgs...), "--force-groups", "--result", resultPath)
	if mutateLive {
		controlRoot, err := canonicalProofRoot(fixture.root)
		if err != nil {
			t.Fatal(err)
		}
		beforeAttempts, err := proofrun.ReadAttempts(controlRoot)
		if err != nil {
			t.Fatal(err)
		}
		beforeBuilds, beforeNative := fixture.counts()
		fixture.write("scripts/check.sh", "#!/bin/sh\nexit 9\n", 0o755)
		fixture.write("app/worker-mode.txt", "red\n", 0o644)
		refusalStatus, refusalOutput := fixture.command(runArgs...)
		afterBuilds, afterNative := fixture.counts()
		afterAttempts, err := proofrun.ReadAttempts(controlRoot)
		if err != nil {
			t.Fatal(err)
		}
		if refusalStatus != 1 || !strings.Contains(refusalOutput, "delivery candidate differs from relevant working-tree inputs") ||
			afterBuilds != beforeBuilds || !maps.Equal(afterNative, beforeNative) || len(afterAttempts) != len(beforeAttempts) {
			t.Fatalf("%s dirty delivery exit=%d output=%s builds=%d/%d native=%v/%v attempts=%d/%d", label,
				refusalStatus, refusalOutput, beforeBuilds, afterBuilds, beforeNative, afterNative, len(beforeAttempts), len(afterAttempts))
		}
		if _, err := os.Stat(resultPath); !os.IsNotExist(err) {
			t.Fatalf("%s dirty delivery produced a result: %v", label, err)
		}
		t.Logf("PUBLIC_APPLICATION_PARITY_REFUSAL exit=%d output=%q builds=%d native=%v attempts=%d", refusalStatus,
			strings.TrimSpace(refusalOutput), beforeBuilds, beforeNative, len(beforeAttempts))
		fixture.write("scripts/check.sh", publicApplicationWorkerScript, 0o755)
		fixture.write("app/worker-mode.txt", mode+"\n", 0o644)
	}
	beforeGreenBuilds, beforeGreenNative := fixture.counts()

	internalReadyOwner, err := os.OpenFile(internalReady, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer internalReadyOwner.Close()
	readyPipe, err := os.OpenFile(publicReady, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer readyPipe.Close()
	releasePipe, err := os.OpenFile(publicRelease, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer releasePipe.Close()

	command := fixture.proofCommand.command(fixture.commandEnvironment(), fixture.engine, runArgs...)
	if mutateLive {
		// Fake-runtime fixtures use a finite process table for suite custody.
		// Exact native child identities are checked directly after the run.
		processTable := filepath.Join(state, "processes.json")
		if err := testexec.WriteFile(processTable, []byte("[]\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		command.Env = append(command.Env, "METASYSTEM_CENSUS_PROCESS_FILE="+processTable)
	}
	command.Dir = fixture.root
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	pid := 0
	testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{Verb: "public application " + label, Resolve: func() (int, bool, error) {
		return pid, pid != 0, nil
	}}})
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	pid = command.Process.Pid
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ready := make(chan error, 1)
	go func() {
		one := make([]byte, 1)
		_, readErr := io.ReadFull(readyPipe, one)
		if readErr == nil && string(one) != "x" {
			readErr = fmt.Errorf("readiness byte %q", one)
		}
		ready <- readErr
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("%s readiness: %v; output=%s", label, err, output.String())
		}
	case err := <-done:
		t.Fatalf("%s public run exited before readiness: %v; output=%s; native-log=%s", label, err, output.String(), publicApplicationNativeLog(resultPath))
	case <-t.Context().Done():
		t.Fatalf("%s readiness cancelled: %v", label, t.Context().Err())
	}

	observed := publicApplicationRun{label: label, configured: configuredWorkers, workers: workers, tree: tree, resultPath: resultPath, wantDigest: fixture.installedDigest}
	if mutateLive {
		cwdBytes, err := os.ReadFile(fixture.nativeCounter + ".cwd")
		if err != nil {
			t.Fatalf("%s native cwd: %v", label, err)
		}
		observed.nativeCWD = strings.TrimSpace(string(cwdBytes))
		liveRoot, err := filepath.EvalSymlinks(fixture.root)
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(observed.nativeCWD) || observed.nativeCWD == liveRoot {
			t.Fatalf("%s native cwd=%q live root=%q", label, observed.nativeCWD, liveRoot)
		}
		for _, path := range []string{"scripts/check.sh", "app/worker-mode.txt"} {
			committed := fixture.gitBytes("show", tree+":"+path)
			actual, err := os.ReadFile(filepath.Join(observed.nativeCWD, path))
			if err != nil || !bytes.Equal(actual, committed) {
				t.Fatalf("%s detached %s differs from committed candidate: read=%v actual=%q committed=%q", label, path, err, actual, committed)
			}
		}
		t.Logf("PUBLIC_APPLICATION_DETACHED_CWD cwd=%q live=%q committedTree=%s scriptBytes=%d inputBytes=%d", observed.nativeCWD,
			liveRoot, tree, len(fixture.gitBytes("show", tree+":scripts/check.sh")), len(fixture.gitBytes("show", tree+":app/worker-mode.txt")))
	}
	observed.children = publicApplicationChildren(t, state, workers)
	observed.processes = len(observed.children)
	maximum, err := os.ReadFile(filepath.Join(state, "maximum"))
	if err != nil {
		t.Fatal(err)
	}
	observed.maximum, err = strconv.Atoi(strings.TrimSpace(string(maximum)))
	if err != nil || observed.maximum != workers || observed.processes != workers {
		t.Fatalf("%s active maximum=%d process count=%d allowance=%d parse=%v", label, observed.maximum, observed.processes, workers, err)
	}
	if cancel {
		controlRoot, err := canonicalProofRoot(fixture.root)
		if err != nil {
			t.Fatal(err)
		}
		observed.attemptID = activePublicApplicationAttempt(t, controlRoot, tree)
		if err := proofrun.RequestCancellation(controlRoot, observed.attemptID, "public application cancellation drain"); err != nil {
			t.Fatal(err)
		}
	} else if _, err := releasePipe.Write([]byte("x\n")); err != nil {
		t.Fatal(err)
	}
	var waitErr error
	select {
	case waitErr = <-done:
	case <-t.Context().Done():
		t.Fatalf("%s public run did not terminate after its event: %v", label, t.Context().Err())
	}
	observed.output = output.String()
	if waitErr != nil {
		var exit *exec.ExitError
		if !errors.As(waitErr, &exit) {
			t.Fatalf("%s public run: %v; output=%s", label, waitErr, observed.output)
		}
		observed.status = exit.ExitCode()
	}
	if data, err := os.ReadFile(resultPath); err == nil {
		var result proofrun.TestResult
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("%s result decode: %v: %s", label, err, data)
		}
		if err := proofrun.ValidateTestResult(result); err != nil {
			t.Fatalf("%s result validation: %v: %s", label, err, data)
		}
		if observed.attemptID != "" && observed.attemptID != result.AttemptID {
			t.Fatalf("%s result attempt=%s differs from observed attempt=%s", label, result.AttemptID, observed.attemptID)
		}
		observed.attemptID = result.AttemptID
		observed.result = &result
	} else if !cancel || !os.IsNotExist(err) {
		t.Fatalf("%s result read: %v; output=%s", label, err, observed.output)
	}
	observed.nativeLog = publicApplicationNativeLog(resultPath)
	if mutateLive {
		for _, child := range observed.children {
			exact, state, err := (identity.KernelProber{}).Probe(child.Pid)
			if err != nil || state == identity.Alive && identity.SameIdentity(exact, child) && !exact.Zombie {
				t.Fatalf("%s retained executable native child: ref=%+v exact=%+v state=%s err=%v", label, child, exact, state, err)
			}
		}
		afterGreenBuilds, afterGreenNative := fixture.counts()
		if afterGreenBuilds != beforeGreenBuilds+1 || afterGreenNative["application-workers"] != beforeGreenNative["application-workers"]+1 {
			t.Fatalf("%s green launched unexpected build/native work: builds=%d/%d native=%v/%v", label,
				beforeGreenBuilds, afterGreenBuilds, beforeGreenNative, afterGreenNative)
		}
		t.Logf("PUBLIC_APPLICATION_GREEN_WORK builds=%d native=%v attempt=%s", afterGreenBuilds, afterGreenNative, observed.attemptID)
	}
	cancellationMethod := "none"
	if cancel {
		cancellationMethod = "production-request-api"
	}
	t.Logf("PUBLIC_APPLICATION_PHASE label=%s configuredWorkers=%d effectiveWorkers=%d maximum=%d processes=%d status=%d retained=%t cancellation=%s", label, configuredWorkers, workers, observed.maximum, observed.processes, observed.status, observed.result != nil, cancellationMethod)
	return observed
}

func requirePublicApplicationGreen(t *testing.T, fixture *portableProofFixture, observed publicApplicationRun) {
	t.Helper()
	group := requirePublicApplicationResult(t, observed, "passed", true)
	if observed.status != 0 || !observed.result.Delivery.Sufficient || group.NativeExitStatus == nil || *group.NativeExitStatus != 0 ||
		len(group.Observed) != 1 || group.Observed[0].Status != "passed" {
		t.Fatalf("%s green result status=%d delivery=%+v group=%+v output=%s native-log=%s", observed.label, observed.status, observed.result.Delivery, group, observed.output, observed.nativeLog)
	}
	verifyStatus, verifyOutput := fixture.command("test", "verify", "--root", fixture.root, "--goal", "portable", "--tree", observed.tree, "--mode", "auto", "--purpose", "delivery")
	if verifyStatus != 0 {
		t.Fatalf("%s public verify refused green: status=%d output=%s", observed.label, verifyStatus, verifyOutput)
	}
	requirePublicApplicationCommittedReceipt(t, fixture, observed)
}

func requirePublicApplicationCommittedReceipt(t *testing.T, fixture *portableProofFixture, observed publicApplicationRun) {
	t.Helper()
	controlRoot, err := canonicalProofRoot(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := proofrun.ReadAttempt(controlRoot, observed.result.AttemptID)
	payload := proofrun.CommittedDeliveryReceipt(attempt)
	if err != nil || attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess || attempt.Terminal.ExitStatus != 0 || len(payload) == 0 {
		t.Fatalf("%s committed receipt attempt=%s terminal=%+v payload=%d err=%v", observed.label, observed.result.AttemptID, attempt.Terminal, len(payload), err)
	}
	var receipt landing.TestReceipt
	if err := json.Unmarshal(payload, &receipt); err != nil {
		t.Fatalf("%s committed receipt decode: %v", observed.label, err)
	}
	if receipt.Tree != observed.tree || receipt.ProvedTree != observed.tree || receipt.ExitStatus != 0 || receipt.Testing == nil ||
		receipt.Testing.AttemptID != observed.result.AttemptID || receipt.Testing.CandidateTree != observed.tree || !receipt.Testing.Delivery.Sufficient {
		t.Fatalf("%s committed receipt does not match the sufficient attempt: %+v", observed.label, receipt)
	}
}

func requirePublicApplicationRed(t *testing.T, fixture *portableProofFixture, observed publicApplicationRun) {
	t.Helper()
	group := requirePublicApplicationResult(t, observed, "failed", true)
	if observed.status == 0 || observed.result.Delivery.Sufficient || group.NativeExitStatus == nil || *group.NativeExitStatus != 9 ||
		len(group.Observed) != 1 || group.Observed[0].Status != "failed" {
		t.Fatalf("retained red status=%d delivery=%+v group=%+v output=%s native-log=%s", observed.status, observed.result.Delivery, group, observed.output, observed.nativeLog)
	}
	requirePublicApplicationVerifyRefusal(t, fixture, observed)
}

func requirePublicApplicationCancelled(t *testing.T, fixture *portableProofFixture, observed publicApplicationRun) {
	t.Helper()
	if observed.status == 0 {
		t.Fatalf("cancelled public run exited green: %s", observed.output)
	}
	if observed.result != nil {
		group := requirePublicApplicationResult(t, observed, "cancelled", false)
		if observed.result.Delivery.Sufficient || !group.NativeLaunched {
			t.Fatalf("cancelled result authorized delivery or omitted native launch: %+v", observed.result)
		}
	}
	for _, child := range observed.children {
		exact, state, err := (identity.KernelProber{}).Probe(child.Pid)
		released := err == nil && (state == identity.Dead || state == identity.Alive && (!identity.SameIdentity(exact, child) || exact.Zombie))
		if !released {
			t.Fatalf("cancelled public application left exact child executable: ref=%+v exact=%+v state=%s err=%v", child, exact, state, err)
		}
	}
	requirePublicApplicationVerifyRefusal(t, fixture, observed)
}

func requirePublicApplicationResult(t *testing.T, observed publicApplicationRun, status string, complete bool) proofrun.GroupResult {
	t.Helper()
	if observed.result == nil || observed.result.WorkerPolicyVersion != proofrun.TestWorkerPolicyVersion || observed.result.Workers != observed.workers ||
		observed.result.AdmissionMaximum == nil || *observed.result.AdmissionMaximum != proofAdmissionFixtureMax || len(observed.result.Groups) != 1 {
		t.Fatalf("%s retained worker result=%+v native-log=%s", observed.label, observed.result, observed.nativeLog)
	}
	if status != "cancelled" && observed.wantDigest != "" &&
		(observed.result.CandidateEngineDigest != observed.wantDigest || observed.result.PolicyEngineDigest != observed.wantDigest) {
		t.Fatalf("%s retained result used different engine bytes: policy=%s candidate=%s want=%s native-log=%s",
			observed.label, observed.result.PolicyEngineDigest, observed.result.CandidateEngineDigest, observed.wantDigest, observed.nativeLog)
	}
	group := observed.result.Groups[0]
	if group.ID != "application-workers" || group.Status != status || group.CollectionComplete != complete {
		t.Fatalf("%s group status=%+v want status=%s complete=%t", observed.label, group, status, complete)
	}
	return group
}

func requirePublicApplicationVerifyRefusal(t *testing.T, fixture *portableProofFixture, observed publicApplicationRun) {
	t.Helper()
	status, output := fixture.command("test", "verify", "--root", fixture.root, "--goal", "portable", "--tree", observed.tree, "--mode", "auto", "--purpose", "delivery")
	if status == 0 {
		t.Fatalf("%s public verify accepted insufficient evidence: %s", observed.label, output)
	}
	controlRoot, err := canonicalProofRoot(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := proofrun.ReadAttempt(controlRoot, observed.attemptID)
	if err != nil || len(proofrun.CommittedDeliveryReceipt(attempt)) != 0 {
		t.Fatalf("%s insufficient attempt=%s has committed receipt payload: err=%v", observed.label, observed.attemptID, err)
	}
	if _, err := os.Stat(landing.TestReceiptPath(fixture.root, observed.tree)); !os.IsNotExist(err) {
		t.Fatalf("%s published a delivery receipt from insufficient evidence: %v", observed.label, err)
	}
}

func publicApplicationChildren(t *testing.T, state string, workers int) []identity.Ref {
	t.Helper()
	children := make([]identity.Ref, 0, workers)
	seen := map[int64]bool{}
	for index := 1; index <= workers; index++ {
		data, err := os.ReadFile(filepath.Join(state, "pid-"+strconv.Itoa(index)))
		pid, parseErr := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		exact, live, probeErr := (identity.KernelProber{}).Probe(pid)
		if err != nil || parseErr != nil || probeErr != nil || live != identity.Alive || !exact.Ref().NativeExact() || seen[pid] {
			t.Fatalf("worker %d identity data=%q read=%v parse=%v probe=%v live=%s exact=%+v duplicate=%t", index, data, err, parseErr, probeErr, live, exact, seen[pid])
		}
		seen[pid] = true
		children = append(children, exact.Ref())
	}
	return children
}

func activePublicApplicationAttempt(t *testing.T, root, tree string) string {
	t.Helper()
	attempts, err := proofrun.ReadAttempts(root)
	if err != nil {
		t.Fatal(err)
	}
	for index := len(attempts) - 1; index >= 0; index-- {
		if attempts[index].CandidateTree == tree && attempts[index].Terminal == nil && attempts[index].CancellationIntent == "" {
			return attempts[index].AttemptID
		}
	}
	t.Fatalf("public application tree %s has no active admitted attempt", tree)
	return ""
}

func fixtureProjectPrefix(root string) (string, error) {
	command := exec.Command("git", "-C", root, "rev-parse", "--show-prefix")
	output, err := command.CombinedOutput()
	return strings.Trim(strings.TrimSpace(string(output)), "/"), err
}

func newInstalledPublicApplicationFixture(t *testing.T, installed, expectedDigest string) *portableProofFixture {
	t.Helper()
	if !filepath.IsAbs(installed) {
		t.Fatalf("installed public application binary must be absolute: %q", installed)
	}
	canonicalEngine, err := canonicalPath(installed)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest := strings.TrimPrefix(expectedDigest, "sha256:")
	actualDigest, err := fileSHA256(canonicalEngine)
	if err != nil || len(wantDigest) != 64 || actualDigest != wantDigest {
		t.Fatalf("installed public application digest mismatch: got=%s want=%s err=%v", actualDigest, wantDigest, err)
	}
	snapshot := filepath.Join(t.TempDir(), "installed-engine")
	if err := copyCandidateEngineArtifact(canonicalEngine, snapshot); err != nil {
		t.Fatalf("copy immutable installed public application: %v", err)
	}
	if snapshotDigest, err := fileSHA256(snapshot); err != nil || snapshotDigest != wantDigest {
		t.Fatalf("immutable installed public application digest mismatch: got=%s want=%s err=%v", snapshotDigest, wantDigest, err)
	}
	t.Cleanup(func() {
		snapshotDigest, snapshotErr := fileSHA256(snapshot)
		sourceDigest, sourceErr := fileSHA256(canonicalEngine)
		if snapshotErr != nil || sourceErr != nil || snapshotDigest != wantDigest || sourceDigest != wantDigest {
			t.Errorf("installed public application bytes changed: snapshot=%s source=%s want=%s snapshotErr=%v sourceErr=%v", snapshotDigest, sourceDigest, wantDigest, snapshotErr, sourceErr)
		}
	})
	statusRoot := t.TempDir()
	status := exec.Command(canonicalEngine, "supervise", "status", "--repo", statusRoot)
	statusOutput, err := status.CombinedOutput()
	var engineStatus struct {
		EngineBuild string `json:"engineBuild"`
	}
	if err != nil || json.Unmarshal(statusOutput, &engineStatus) != nil || len(engineStatus.EngineBuild) != 40 {
		t.Fatalf("read installed public application build stamp: err=%v output=%s", err, statusOutput)
	}
	sourceRootCommand := exec.Command("git", "rev-parse", "--show-toplevel")
	sourceRootOutput, err := sourceRootCommand.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve installed replay source: %v: %s", err, sourceRootOutput)
	}
	sourceRoot := strings.TrimSpace(string(sourceRootOutput))
	head := strings.TrimSpace(runPublicApplicationGit(t, sourceRoot, "rev-parse", "HEAD"))
	if head != engineStatus.EngineBuild {
		t.Fatalf("installed fixture authority obstacle: binary build stamp %s is not the available source commit %s; replay requires the exact published source so ENGINE ancestry is genuine", engineStatus.EngineBuild, head)
	}
	clone := filepath.Join(t.TempDir(), "private-application-source")
	cloneCommand := exec.Command("git", "clone", "-q", "--no-local", sourceRoot, clone)
	if output, err := cloneCommand.CombinedOutput(); err != nil {
		t.Fatalf("clone private installed replay authority: %v: %s", err, output)
	}
	root := filepath.Join(clone, "metasystem")
	fixture := &portableProofFixture{t: t, root: root, engine: canonicalEngine,
		admissionDir: filepath.Join(t.TempDir(), "host-admission"), buildCounter: filepath.Join(t.TempDir(), "builds.log"), nativeCounter: filepath.Join(t.TempDir(), "native.log"),
		installedSnapshot: snapshot, installedDigest: wantDigest}
	fixture.git("remote", "remove", "origin")
	fixture.git("config", "user.name", "installed-public-application-fixture")
	fixture.git("config", "user.email", "installed-public-application@example.invalid")
	fixture.git("config", "metasystem.goal.machine", "portable")
	fixture.git("config", "goal.sync-remote", "local")
	fixture.git("config", "goal.sync-branch", goal.LocalLedgerBranch)
	fixture.git("config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	fixture.write("metasystem.conf", "metasystem.runtimes=fake\ntesting.contract=testing.json\ntesting.workers=1\n"+
		proofrun.AdmissionCapKey+"="+strconv.Itoa(proofAdmissionFixtureMax)+"\n", 0o644)
	fixture.writeGoal()
	fixture.proofCommand = pinProofBinaryFixture(t, root)
	fixture.git("add", "-A")
	fixture.git("commit", "-qm", "private synthetic application authority for installed engine")
	base := fixture.git("rev-parse", "HEAD")
	fixture.git("update-ref", "refs/remotes/origin/main", base)
	fixture.git("update-ref", goal.LocalLedgerBranch, base)
	fixture.git("update-ref", goal.AcceptedRef, base)
	canonicalRoot, err := canonicalProofRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(canonicalRoot)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(canonicalRoot), steward.InstallIdentity{RepoIdentity: canonicalRoot, Generation: 1,
		InstallPath: canonicalEngine, InstallDigest: "sha256:" + actualDigest, MintedAt: "2026-09-21T00:00:00Z",
		Enrollment: steward.EnrollmentFixture, EngineBuild: engineStatus.EngineBuild}); err != nil {
		t.Fatal(err)
	}
	batchFixture := &batchE2EFixture{t: t, landing: root}
	batchFixture.announce(root, "portable-lineage")
	holder, err := lease.RequireHolder(root, int64(os.Getpid()), nil)
	if err != nil || !holder.Holder {
		t.Fatalf("installed replay checkout has no active fixture lease holder: %+v %v", holder, err)
	}
	t.Logf("INSTALLED_PUBLIC_APPLICATION_REPLAY enabled=true privateSyntheticAuthority=true binary=%s sha256=%s build=%s", canonicalEngine, actualDigest, engineStatus.EngineBuild)
	return fixture
}

func runPublicApplicationGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func publicApplicationInstalledCandidateScript(snapshot, digest string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
[[ "$#" -eq 3 && "$1" == --trimpath && "$2" == --out && -n "$3" ]] || { echo "installed candidate build requires --trimpath --out PATH" >&2; exit 2; }
snapshot=%s
expected=%s
actual=$("$snapshot" util sha256 --file "$snapshot")
[[ "$actual" == "$expected" ]] || { echo "installed candidate snapshot digest mismatch: got=$actual want=$expected" >&2; exit 1; }
cp "$snapshot" "$3"
chmod 0500 "$3"
actual=$("$snapshot" util sha256 --file "$3")
[[ "$actual" == "$expected" ]] || { echo "installed candidate output digest mismatch: got=$actual want=$expected" >&2; exit 1; }
`, strconv.Quote(snapshot), strconv.Quote(digest))
}

func publicApplicationNativeLog(resultPath string) string {
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return fmt.Sprintf("unavailable: %v", err)
	}
	var result proofrun.TestResult
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Sprintf("result-decode: %v", err)
	}
	var logs strings.Builder
	for _, group := range result.Groups {
		content, readErr := os.ReadFile(group.LogPath)
		fmt.Fprintf(&logs, "group=%s path=%s read=%v\n%s", group.ID, group.LogPath, readErr, content)
	}
	return logs.String()
}

const publicApplicationWorkerScript = `#!/bin/sh
set -eu
input=$1
report=$2
: "${METASYSTEM_TEST_WORKERS:?}"
: "${PUBLIC_APPLICATION_STATE:?}"
: "${PUBLIC_APPLICATION_INTERNAL_READY:?}"
: "${PUBLIC_APPLICATION_READY:?}"
: "${PUBLIC_APPLICATION_RELEASE:?}"
: "${PUBLIC_APPLICATION_NATIVE_COUNTER:?}"
: "${PUBLIC_APPLICATION_NATIVE_CWD:?}"
printf 'application-workers\n' >>"$PUBLIC_APPLICATION_NATIVE_COUNTER"
pwd -P >"$PUBLIC_APPLICATION_NATIVE_CWD"
mode=$(cat "$input")
i=0
pids=
while [ "$i" -lt "$METASYSTEM_TEST_WORKERS" ]; do
  i=$((i+1))
  (
    printf x >"$PUBLIC_APPLICATION_INTERNAL_READY"
    IFS= read -r released <"$PUBLIC_APPLICATION_STATE/release-$i.fifo"
  ) </dev/null >/dev/null 2>&1 &
  child=$!
  printf '%s\n' "$child" >"$PUBLIC_APPLICATION_STATE/pid-$i"
  pids="$pids $child"
done
dd if="$PUBLIC_APPLICATION_INTERNAL_READY" bs=1 count="$METASYSTEM_TEST_WORKERS" of=/dev/null 2>/dev/null
maximum=0
for child in $pids; do
  kill -0 "$child" 2>/dev/null && maximum=$((maximum+1))
done
printf '%s\n' "$maximum" >"$PUBLIC_APPLICATION_STATE/maximum"
printf x >"$PUBLIC_APPLICATION_READY"
IFS= read -r release <"$PUBLIC_APPLICATION_RELEASE"
i=1
while [ "$i" -le "$METASYSTEM_TEST_WORKERS" ]; do
  printf 'release\n' >"$PUBLIC_APPLICATION_STATE/release-$i.fifo"
  i=$((i+1))
done
for child in $pids; do wait "$child"; done
mkdir -p "$report"
if [ "$mode" = red ]; then
  printf '<testsuite><testcase classname="public-application" name="allowance"><failure message="retained red"/></testcase></testsuite>\n' >"$report/result.xml"
  exit 9
fi
printf '<testsuite><testcase classname="public-application" name="allowance"/></testsuite>\n' >"$report/result.xml"
`
