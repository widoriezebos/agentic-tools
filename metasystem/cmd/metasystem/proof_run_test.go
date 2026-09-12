package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

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
	attempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root,
		GoalID: "standing-validation", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 30,
		Identity: proofIdentity, Launcher: launcher, Now: started})
	if err != nil {
		t.Fatal(err)
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

func TestProofRunCommandTopLevelRetryAcrossRenamedRoots(t *testing.T) {
	controlRoot := syncedClaimedGoalFixture(t)
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
		command := exec.Command(engine, args...)
		command.Env = environment
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
		if err := os.WriteFile(os.Getenv("GOVERNED_COMMAND_READY"), []byte("ready\n"), 0o600); err != nil {
			os.Exit(97)
		}
		deadline := time.Now().Add(wiringBound)
		for {
			if _, err := os.Stat(os.Getenv("GOVERNED_COMMAND_RELEASE")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				os.Exit(97)
			}
			time.Sleep(10 * time.Millisecond)
		}
		root := os.Getenv("GOVERNED_COMMAND_ROOT")
		command := exec.Command(os.Getenv("GOVERNED_COMMAND_ENGINE"), "proof-run", "launch",
			"--suite", "governed-command", "--root", root, "--control-root", root,
			"--conf", filepath.Join(root, "metasystem.conf"), "--progress", filepath.Join(root, "artifacts", "governed.progress.jsonl"),
			"--log", filepath.Join(root, "artifacts", "governed.log"), "--banner", "governed command canary", "--", "true")
		// The governed locators travel under the fixture's own names: the
		// package's TestMain clears every METASYSTEM_PROOF_* control before
		// a test runs, so the engine receives them here, not by inheritance.
		command.Env = append(os.Environ(), "METASYSTEM_PROOF_RUN_ROOT="+os.Getenv("GOVERNED_COMMAND_PROOF_RUN_ROOT"),
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
	ready, release := filepath.Join(t.TempDir(), "ready"), filepath.Join(t.TempDir(), "release")
	child := exec.Command(os.Args[0], "-test.run=^TestProofRunCommandGovernedParentSharesOneCharge$")
	child.Env = append(receiptCanaryEnvironment(), "GO_WANT_GOVERNED_COMMAND_PARENT=1", "GOVERNED_COMMAND_READY="+ready,
		"GOVERNED_COMMAND_RELEASE="+release, "GOVERNED_COMMAND_ROOT="+root, "GOVERNED_COMMAND_ENGINE="+engine,
		"GOVERNED_COMMAND_PROOF_RUN_ROOT="+root, "GOVERNED_COMMAND_PROOF_RUN_ID=governed-command")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	child.Stdout, child.Stderr = &output, &output
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	t.Cleanup(func() {
		if !finished {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	deadline := time.Now().Add(wiringBound)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("governed parent did not become ready: %s", output.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	pgid, err := syscall.Getpgid(child.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind("governed-command", nonce, int64(child.Process.Pid), int64(pgid)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(release, []byte("go\n"), 0o600); err != nil {
		t.Fatal(err)
	}
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
