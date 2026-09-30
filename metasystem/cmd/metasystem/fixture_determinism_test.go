package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func makeFixtureFIFO(t *testing.T, path string) {
	t.Helper()
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeFixtureEvent(ctx context.Context, path string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	descriptor, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(descriptor), path)
	defer file.Close()
	if _, err := file.WriteString("release\n"); err != nil {
		return err
	}
	return nil
}

func commandTestExecutable(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return executable
}

func fixtureCommandEnvironment(t *testing.T, values ...string) []string {
	t.Helper()
	replacements := map[string]bool{"GO_WANT_BATCH_E2E_COMMAND": true}
	for _, value := range values {
		name, _, ok := strings.Cut(value, "=")
		if !ok {
			t.Fatalf("fixture environment entry has no equals sign: %q", value)
		}
		replacements[name] = true
	}
	environment := make([]string, 0, len(os.Environ())+len(values)+1)
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if !replacements[name] {
			environment = append(environment, value)
		}
	}
	environment = append(environment, "GO_WANT_BATCH_E2E_COMMAND=1")
	return append(environment, values...)
}

func syntheticFixtureOwnerKey(t *testing.T, testName string) string {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe synthetic fixture owner: state=%s error=%v", state, err)
	}
	key, err := identity.EncodeKey(identity.FixtureKey{Owner: exact.Ref(), Test: testName, Nonce: "1234abcd"})
	if err != nil {
		t.Fatal(err)
	}
	return key
}

type fixtureProcess struct {
	command *exec.Cmd
	cancel  context.CancelFunc
	output  bytes.Buffer
	done    chan struct{}
	err     error
}

func launchFixtureProcess(command *exec.Cmd, cancel context.CancelFunc) (*fixtureProcess, error) {
	process := &fixtureProcess{command: command, cancel: cancel, done: make(chan struct{})}
	command.Stdout, command.Stderr = &process.output, &process.output
	if err := command.Start(); err != nil {
		cancel()
		return nil, err
	}
	go func() {
		process.err = command.Wait()
		close(process.done)
	}()
	return process, nil
}

func (process *fixtureProcess) wait() (string, error) {
	<-process.done
	return process.output.String(), process.err
}

func (process *fixtureProcess) stopAndWait() (string, error) {
	process.cancel()
	return process.wait()
}

type fixtureLineReader struct {
	file  *os.File
	lines chan string
	done  chan struct{}
	err   error
}

func startFixtureLineReader(file *os.File, count int) *fixtureLineReader {
	reader := &fixtureLineReader{file: file, lines: make(chan string, count), done: make(chan struct{})}
	go func() {
		defer close(reader.done)
		scanner := bufio.NewScanner(file)
		for index := 0; index < count; index++ {
			if !scanner.Scan() {
				reader.err = scanner.Err()
				if reader.err == nil {
					reader.err = io.EOF
				}
				return
			}
			reader.lines <- scanner.Text()
		}
	}()
	return reader
}

func (reader *fixtureLineReader) closeAndWait() {
	_ = reader.file.Close()
	<-reader.done
}

func waitFixtureLine(ctx context.Context, reader *fixtureLineReader, process *fixtureProcess) (string, error) {
	select {
	case line := <-reader.lines:
		return line, nil
	case <-reader.done:
		select {
		case line := <-reader.lines:
			return line, nil
		default:
		}
		return "", fmt.Errorf("readiness producer closed without an event: %w", reader.err)
	case <-process.done:
		output, err := process.wait()
		return "", fmt.Errorf("readiness producer exited early: %v\n%s", err, output)
	case <-ctx.Done():
		output, err := process.stopAndWait()
		return "", fmt.Errorf("readiness cancelled: %v; process exit: %v\n%s", ctx.Err(), err, output)
	}
}

func TestFixtureEventReleaseDoesNotBlockWithoutReader(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "event")
	makeFixtureFIFO(t, path)
	err := writeFixtureEvent(t.Context(), path)
	if err == nil || !errors.Is(err, syscall.ENXIO) {
		t.Fatalf("release without a reader error = %v, want ENXIO", err)
	}
}

func TestLandingReceiptRefusesSemanticBudgetBeforeCommand(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	var err error
	top, err = filepath.EvalSymlinks(top)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(top, "metasystem")
	writeReceiptFixture(t, top, "development/metasystem-design.md", "fixture template marker\n")
	writeReceiptFixture(t, root, "metasystem.conf", "metasystem.version=1\nmetasystem.template=true\nmetasystem.runtimes=fake\n"+proofrun.AdmissionCapKey+"=4\n")
	writeReceiptFixture(t, root, ".gitignore", "artifacts/\n")
	writeReceiptFixture(t, root, "payload.txt", "semantic budget fixture\n")

	now := time.Date(2026, time.September, 21, 12, 0, 0, 0, time.UTC)
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 12, ActiveJobLimit: 1}
	risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture proves pre-launch semantic accounting."}
	intent := "Refuse a receipt proof beyond its recorded semantic budget."
	goalFile := &goal.GoalFile{
		Id: "fixture-budget", State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent,
		Origin: goal.OriginMain, NextStep: "Request one over-budget proof.", OpenedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Revision: 3,
		Budget:  budget,
		Claimed: &goal.ClaimRecord{Machine: "fixture-machine", Lineage: "fixture-budget", At: now.Add(-time.Minute).Format(time.RFC3339), Revision: 2, AccountingRevision: 2},
		Approved: &goal.ApprovalRecord{
			By: "human:fixture", At: now.Add(-30 * time.Second).Format(time.RFC3339), Revision: 3,
			Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FCZ", "fixture-machine", "fixture-budget"), Authority: goal.ApprovalAuthorityProven,
			Digest: goal.ApprovalDigest(intent, 1, *budget, risk),
		},
		StopCapability: &goal.StopCapability{Generation: 3, Revision: 2, Machine: "fixture-machine", ClaimEpoch: 1},
		History: []goal.HistoryLine{
			{At: now.Add(-2 * time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FCA", "fixture-machine", "fixture-budget"), Verb: "open", Actor: "fixture-machine+fixture-budget", Keep: -1},
			{At: now.Add(-time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FCB", "fixture-machine", "fixture-budget"), Verb: "claim", Actor: "fixture-machine+fixture-budget", Keep: -1},
			{At: now.Add(-30 * time.Second).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FCZ", "fixture-machine", "fixture-budget"), Verb: "approve", Actor: "human:fixture", Keep: -1},
		},
	}
	rootRecord := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FCV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	for path, data := range map[string][]byte{
		filepath.Join(root, "plans", "goals", "backlog.md"):        goal.RenderRoot(rootRecord),
		filepath.Join(root, "plans", "goals", "fixture-budget.md"): goal.RenderFile(goalFile),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(top, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	repository := &proofAdmissionRepository{top: top, root: root, commits: map[string]proofAdmissionCommit{}, operations: map[string]string{}}
	repository.seed(map[string][]byte{
		"development/metasystem-design.md":         read("development/metasystem-design.md"),
		"metasystem/metasystem.conf":               read("metasystem/metasystem.conf"),
		"metasystem/plans/goals/backlog.md":        read("metasystem/plans/goals/backlog.md"),
		"metasystem/plans/goals/fixture-budget.md": read("metasystem/plans/goals/fixture-budget.md"),
	})
	fixture := newDeadlineReceiptTreeFixture(t, repository)
	claimed := repository.goalFile(t, "fixture-budget").Claimed
	if claimed == nil {
		t.Fatal("accepted budget goal has no machine claim")
	}
	snapshot := writeProofCommandFixtureSnapshot(t, repository, claimed.Machine)
	wrapper := proofCommandFixtureWrapper(t)
	tempRoot := t.TempDir()
	admissionDir := filepath.Join(tempRoot, "host-admission")
	processes := filepath.Join(t.TempDir(), "processes.json")
	if err := os.WriteFile(processes, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "command-ran")
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe receipt command parent: state=%s error=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "batch-proof-main", exact.Pid, exact.StartedAt.Unix(),
		exact.StartTicks, exact.BootID, "batch-proof-main", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(wrapper, "landing", "test-receipt", "--root", root,
		"--tree", fixture.tree, "--command", "touch "+marker,
		"--goal", "fixture-budget", "--cap-min", "13")
	command.Env = append(testenv.WithoutInheritedControls(os.Environ()),
		proofCommandFixtureMarker+"=1",
		proofCommandFixtureSnapshotEnv+"="+snapshot,
		proofCommandFixtureTempRootEnv+"="+tempRoot,
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR="+admissionDir,
		"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+root,
		"METASYSTEM_CENSUS_PROCESS_FILE="+processes,
		"METASYSTEM_GOAL_NOW="+now.Add(2*time.Minute).Format(time.RFC3339))
	output, commandErr := command.CombinedOutput()
	code := 0
	if commandErr != nil {
		var exit *exec.ExitError
		if !errors.As(commandErr, &exit) {
			t.Fatalf("run receipt refusal: %v\n%s", commandErr, output)
		}
		code = exit.ExitCode()
	}
	if code != proofrun.ExitAdmissionRefused || !bytes.Contains(output, []byte("BUDGET_REFUSED")) {
		t.Fatalf("over-budget receipt exit=%d, output=%q", code, output)
	}
	t.Logf("receipt exit=%d output=%s", code, output)
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("budget-refused command ran: %v", err)
	}
}

type receiptClockFixtureCase struct {
	name          string
	capMinutes    int
	advance       time.Duration
	cancelParent  bool
	wantLaunches  int
	wantExit      int
	wantReason    string
	wantObserved  uint64
	refusesExpiry bool
}

func receiptClockFixtureCases() []receiptClockFixtureCase {
	return []receiptClockFixtureCase{
		{name: "one-minute-before-expiry", capMinutes: 1, advance: -time.Nanosecond, wantLaunches: 1},
		{name: "one-minute-exact-expiry", capMinutes: 1, wantExit: proofrun.ExitAdmissionRefused, wantReason: "has passed (now ", refusesExpiry: true},
		{name: "one-minute-after-expiry", capMinutes: 1, advance: time.Nanosecond, wantExit: proofrun.ExitAdmissionRefused, wantReason: "has passed (now ", wantObserved: 2, refusesExpiry: true},
		{name: "three-minute-event-delayed", capMinutes: 3, advance: -time.Nanosecond, wantLaunches: 1},
		{name: "outer-cancellation", capMinutes: 1, advance: -time.Second, cancelParent: true, wantExit: proofrun.ExitAdmissionRefused, wantReason: context.Canceled.Error()},
	}
}

func TestLandingReceiptPublicSemanticClockBoundariesAndDelayedCompletion(t *testing.T) {
	if selected := os.Getenv("GO_WANT_FIXTURE_RECEIPT_CLOCK_CHILD"); selected != "" {
		for _, testCase := range receiptClockFixtureCases() {
			if testCase.name == selected {
				runLandingReceiptPublicSemanticClockCase(t, testCase)
				return
			}
		}
		t.Fatalf("unknown receipt clock fixture case %q", selected)
	}
	t.Parallel()
	for _, testCase := range receiptClockFixtureCases() {
		t.Run(testCase.name, func(t *testing.T) {
			startedAt := time.Date(2026, time.September, 21, 8, 0, 0, 0, time.UTC)
			command := exec.Command(commandTestExecutable(t), "-test.run=^TestLandingReceiptPublicSemanticClockBoundariesAndDelayedCompletion$", "-test.v")
			command.Env = append(testenv.WithoutInheritedControls(os.Environ()),
				"GO_WANT_FIXTURE_RECEIPT_CLOCK_CHILD="+testCase.name,
				"METASYSTEM_GOAL_NOW="+startedAt.Format(time.RFC3339Nano))
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("owned public receipt clock fixture: %v\n%s", err, output)
			}
			if !bytes.Contains(output, []byte("--- PASS: TestLandingReceiptPublicSemanticClockBoundariesAndDelayedCompletion")) || bytes.Contains(output, []byte("--- SKIP:")) {
				t.Fatalf("owned public receipt clock fixture did not report an unskipped pass:\n%s", output)
			}
		})
	}
}

func runLandingReceiptPublicSemanticClockCase(t *testing.T, testCase receiptClockFixtureCase) {
	startedAt := time.Date(2026, time.September, 21, 8, 0, 0, 0, time.UTC)
	repository := newProofAdmissionRepositoryFixture(t, startedAt, true)
	root, err := filepath.EvalSymlinks(repository.root)
	if err != nil {
		t.Fatal(err)
	}
	repository.root = root
	repository.top, err = filepath.EvalSymlinks(repository.top)
	if err != nil {
		t.Fatal(err)
	}
	confPath := filepath.Join(root, "metasystem.conf")
	configuration := repository.rawFile(t, "metasystem/metasystem.conf")
	configuration = bytes.Replace(configuration, []byte(proofrun.AdmissionCapKey+"=4"), []byte(proofrun.AdmissionCapKey+"=1"), 1)
	if !bytes.Contains(configuration, []byte(proofrun.AdmissionCapKey+"=1")) {
		t.Fatal("public receipt fixture did not narrow host capacity to one")
	}
	if err := os.WriteFile(confPath, configuration, 0o644); err != nil {
		t.Fatal(err)
	}
	repository.seed(map[string][]byte{"metasystem/metasystem.conf": configuration})
	fixture := newDeadlineReceiptTreeFixture(t, repository)
	admissionDir := filepath.Join(t.TempDir(), "host-admission")
	t.Setenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR", admissionDir)
	t.Setenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT", root)
	processes := filepath.Join(t.TempDir(), "processes.json")
	if err := os.WriteFile(processes, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	announceProofFixtureHolder(t, root)
	holder, err := proofrun.AcquireHostResources(context.Background(), root, confPath, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Close() })
	tree := fixture.tree
	launchCount := filepath.Join(t.TempDir(), "native-launches")
	t.Setenv("LANDING_DEADLINE_LAUNCH_COUNT", launchCount)
	resultPath := filepath.Join(t.TempDir(), "launch-result.json")
	semanticNow := startedAt
	t.Setenv(goalNowEnvironment, startedAt.Format(time.RFC3339Nano))
	setSemanticNow := func(at time.Time) {
		semanticNow = at
		// This exact-run child is the sole process owner of the declared clock.
		// Keep terminal accounting on the same instant as the injected resolver.
		if err := os.Setenv(goalNowEnvironment, at.Format(time.RFC3339Nano)); err != nil {
			t.Fatalf("set receipt fixture clock: %v", err)
		}
	}
	deadline := startedAt.Add(time.Duration(testCase.capMinutes) * time.Minute)
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	observedWait := 0
	parent = proofrun.WithHostResourceWaitObserver(parent, func() {
		observedWait++
		setSemanticNow(deadline.Add(testCase.advance))
		if testCase.cancelParent {
			cancelParent()
		}
		_ = holder.Close()
	})
	resolveClock := func(requestRoot string) (func() time.Time, bool, error) {
		if requestRoot != root {
			return nil, false, fmt.Errorf("clock resolver received root %q, want %q", requestRoot, root)
		}
		return func() time.Time { return semanticNow }, true, nil
	}
	args := []string{"--root", root, "--tree", tree,
		"--command", "printf 'launch\\n' >> " + strconv.Quote(launchCount),
		"--goal", "standing-validation", "--cap-min", strconv.Itoa(testCase.capMinutes), "--result", resultPath}
	code, _, problem := captureChannelOutput(t, func(stdout, stderr io.Writer) int {
		reads := repository.reads()
		return landingTestReceiptTo(stdout, stderr, parent, resolveClock, nil, landingReceiptTestRun, args,
			func(receiptRoot, receiptTree, command string) (*landing.ReceiptPreparation, error) {
				return landing.PrepareTestReceiptWithWorkspace(receiptRoot, receiptTree, command, fixture.workspace(),
					func(freezeRoot, freezeTree string) (proofrun.FrozenExport, error) {
						if freezeRoot != root || freezeTree != tree {
							t.Fatalf("freeze coordinates = %q %q", freezeRoot, freezeTree)
						}
						frozen, err := proofrun.Freeze(freezeRoot)
						if err == nil {
							fixture.frozen = frozen.Root
							fixture.checkFiles(frozen.Root)
						}
						return frozen, err
					})
			},
			func(request proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
				return admitCandidateProofLaunchWithRepository(t, repository, request)
			},
			func(receiptRoot, attemptID, receiptTree string, now time.Time) (landing.TestReceipt, error) {
				return landing.PublishCommittedReceiptAtWithWorkspace(receiptRoot, attemptID, receiptTree, now, fixture.workspace())
			},
			func(completion proofrun.CompletionContext, receipt json.RawMessage) error {
				return commitProofTerminalWithReasonAndReads(completion, receipt, nil, "proof launcher completed", &reads)
			})
	})
	if code != testCase.wantExit || observedWait != 1 || !strings.Contains(problem, testCase.wantReason) {
		t.Fatalf("public boundary exit=%d want=%d waits=%d stderr=%q", code, testCase.wantExit, observedWait, problem)
	}
	if testCase.cancelParent && strings.Contains(problem, "has passed (now ") {
		t.Fatalf("outer cancellation was mislabeled semantic expiry: %s", problem)
	}
	launches := 0
	if data, readErr := os.ReadFile(launchCount); readErr == nil {
		launches = strings.Count(string(data), "launch\n")
	} else if !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	if launches != testCase.wantLaunches {
		t.Fatalf("native launches=%d want=%d", launches, testCase.wantLaunches)
	}
	var launchResult proofrun.LaunchResult
	encodedResult, readErr := os.ReadFile(resultPath)
	if readErr != nil || json.Unmarshal(encodedResult, &launchResult) != nil || launchResult.ExitStatus != code {
		t.Fatalf("structured result=%+v readErr=%v bytes=%s", launchResult, readErr, encodedResult)
	}
	if !strings.Contains(launchResult.Reason, testCase.wantReason) {
		t.Fatalf("structured refusal reason=%q want substring %q", launchResult.Reason, testCase.wantReason)
	}
	wantDisposition := proofrun.DispositionExecuted
	if testCase.wantLaunches == 0 {
		wantDisposition = proofrun.DispositionAdmissionRefused
	}
	if launchResult.Disposition != wantDisposition {
		t.Fatalf("semantic boundary disposition=%+v want=%s", launchResult, wantDisposition)
	}
	attempts, err := proofrun.ReadAttempts(root)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("public boundary attempts=%d err=%v", len(attempts), err)
	}
	attempt := attempts[0]
	wantObserved := testCase.wantObserved
	if wantObserved == 0 {
		wantObserved = uint64(testCase.capMinutes)
	}
	if attempt.ReservedMinutes != uint64(testCase.capMinutes) || attempt.StartedAt != startedAt.Format(time.RFC3339Nano) ||
		attempt.Deadline != deadline.Format(time.RFC3339Nano) || attempt.ObservedMinutes != wantObserved || attempt.Terminal == nil {
		t.Fatalf("public boundary changed reservation/accounting: %+v", attempt)
	}
	if testCase.wantLaunches == 1 {
		if attempt.Terminal.Result != proofrun.TerminalSuccess || len(proofrun.CommittedDeliveryReceipt(attempt)) == 0 {
			t.Fatalf("accepted boundary lost custody or receipt: %+v", attempt)
		}
		if _, err := os.Stat(landing.TestReceiptPath(root, tree)); err != nil {
			t.Fatalf("accepted boundary did not publish receipt: %v", err)
		}
	} else {
		if attempt.Terminal.Result == proofrun.TerminalSuccess || len(proofrun.CommittedDeliveryReceipt(attempt)) != 0 {
			t.Fatalf("refused boundary published success: %+v", attempt)
		}
		if _, err := os.Stat(landing.TestReceiptPath(root, tree)); !os.IsNotExist(err) {
			t.Fatalf("refused boundary projected a receipt: %v", err)
		}
	}
	probe, acquireErr := proofrun.AcquireHostResources(t.Context(), root, confPath, "heavy", nil)
	if acquireErr != nil || probe == nil {
		t.Fatalf("released cap-one ownership was not reacquirable: lease=%v err=%v", probe, acquireErr)
	}
	if err := proofrun.MarkHostResourcesClean(probe.Files()); err != nil {
		t.Fatal(err)
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	assertHostAdmissionClean(t, admissionDir, 1)
}

func TestNestedPublicProofRunConsumesInheritedWorkerCeiling(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("metasystem.runtimes=fake\ntesting.workers=4\n"+proofrun.AdmissionCapKey+"=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "worker-state")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	script := `set -eu
state=$1
ready=$state/ready.fifo
mkfifo "$ready"
i=0
pids=
while [ "$i" -lt "$METASYSTEM_TEST_WORKERS" ]; do
  i=$((i+1))
  mkfifo "$state/release-$i.fifo"
  (printf x >"$ready"; IFS= read -r released <"$state/release-$i.fifo") &
  pids="$pids $!"
done
dd if="$ready" bs=1 count="$METASYSTEM_TEST_WORKERS" of=/dev/null 2>/dev/null
maximum=0
for pid in $pids; do kill -0 "$pid" 2>/dev/null && maximum=$((maximum+1)); done
printf '%s\n' "$METASYSTEM_TEST_WORKERS" >"$state/allowance"
printf '%s\n' "$maximum" >"$state/maximum"
printf '%s\n' $pids >"$state/pids"
i=1
while [ "$i" -le "$METASYSTEM_TEST_WORKERS" ]; do
  printf 'release\n' >"$state/release-$i.fifo"
  i=$((i+1))
done
for pid in $pids; do wait "$pid"; done
`
	result := filepath.Join(root, "launch-result.json")
	admission := filepath.Join(root, "host-admission")
	identities := filepath.Join(root, "process-identities.json")
	processes := filepath.Join(root, "processes.json")
	if err := os.WriteFile(identities, []byte(fmt.Sprintf(`{"%d":{"terminal":true}}`, os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(processes, []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := fixtureCommandEnvironment(t, proofrun.TestWorkersEnvironment+"=1",
		"METASYSTEM_PROOF_ADMISSION_TEST_DIR="+admission, "METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+root,
		"METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+identities, "METASYSTEM_CENSUS_PROCESS_FILE="+processes)
	command := (proofBinaryFixture{t: t}).command(environment, commandTestExecutable(t), "proof-run", "launch", "--suite", "nested-worker-ceiling",
		"--root", root, "--conf", conf, "--progress", result+".progress", "--log", result+".log", "--banner", "nested workers",
		"--result", result, "--", "/bin/sh", "-c", script, "fixture", state)
	output, commandErr := command.CombinedOutput()
	if commandErr != nil {
		t.Fatalf("nested public proof run: %v\n%s", commandErr, output)
	}
	allowance, allowanceErr := os.ReadFile(filepath.Join(state, "allowance"))
	maximum, maximumErr := os.ReadFile(filepath.Join(state, "maximum"))
	pids, pidsErr := os.ReadFile(filepath.Join(state, "pids"))
	if allowanceErr != nil || maximumErr != nil || pidsErr != nil || string(allowance) != "1\n" || string(maximum) != "1\n" || len(strings.Fields(string(pids))) != 1 {
		t.Fatalf("nested public allowance=%q maximum=%q pids=%q errors=%v/%v/%v", allowance, maximum, pids, allowanceErr, maximumErr, pidsErr)
	}
	var launch proofrun.LaunchResult
	data, err := os.ReadFile(result)
	if err != nil || json.Unmarshal(data, &launch) != nil || launch.Disposition != proofrun.DispositionExecuted || launch.ExitStatus != 0 {
		t.Fatalf("nested public launch result=%+v data=%q err=%v", launch, data, err)
	}
}

func TestTermResistantFakeHostAcknowledgesTermAndOwnerLeashReclaimsIt(t *testing.T) {
	t.Parallel()
	runFakeHostLifetimeWitness(t, true)
}

func TestOrdinaryFakeHostStopsOnTermAfterReadiness(t *testing.T) {
	t.Parallel()
	runFakeHostLifetimeWitness(t, false)
}

func runFakeHostLifetimeWitness(t *testing.T, resistTerm bool) {
	t.Helper()
	fixtureRoot := t.TempDir()
	if err := testexec.WriteFile(filepath.Join(fixtureRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	turn := filepath.Join(fixtureRoot, "turn")
	if err := os.Mkdir(turn, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(turn, "turn.json"), []byte("{\"missionId\":\"fixture-host\",\"turnId\":\"fixture-turn\",\"cycle\":1}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(turn, "prompt.md"), []byte("FAKEHOST:no-return\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	leashPath := filepath.Join(fixtureRoot, "leash")
	makeFixtureFIFO(t, leashPath)
	leash, err := os.OpenFile(leashPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = leash.Close() })
	hostReady := filepath.Join(turn, "host-ready")
	makeFixtureFIFO(t, hostReady)
	hostReadyFile, err := os.OpenFile(hostReady, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	hostReadyEvent := startFixtureLineReader(hostReadyFile, 1)
	t.Cleanup(hostReadyEvent.closeAndWait)
	var hostTermEvent *fixtureLineReader
	if resistTerm {
		hostTermObserved := filepath.Join(turn, "host-term-observed")
		makeFixtureFIFO(t, hostTermObserved)
		hostTermFile, err := os.OpenFile(hostTermObserved, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		hostTermEvent = startFixtureLineReader(hostTermFile, 1)
		t.Cleanup(hostTermEvent.closeAndWait)
	}
	witnessName := "ordinary-host"
	if resistTerm {
		witnessName = "term-resistant-host"
	}
	owner := syntheticFixtureOwnerKey(t, witnessName)
	ctx, cancel := context.WithCancel(t.Context())
	command := exec.CommandContext(ctx, commandTestExecutable(t), "delegate-supervisor",
		identity.FixtureOwnerEnv+"="+owner,
		"fake", "start-turn", "--root", fixtureRoot, "--mission", "fixture-host", "--turn-id", "fixture-turn",
		"--prompt", filepath.Join(turn, "prompt.md"), "--result", filepath.Join(turn, "result.json"),
		"--instance-tag", "fixture-host",
	)
	environment := []string{
		"METASYSTEM_BIN=" + commandTestExecutable(t),
		identity.FixtureOwnerEnv + "=" + owner,
		fixtureLeashEnvironment + "=" + leashPath,
		"METASYSTEM_FAKE_HOST_HOLD=1",
		"GO_WANT_BATCH_E2E_COMMAND=1",
	}
	if resistTerm {
		environment = append(environment, "METASYSTEM_FAKE_HOST_IGNORE_TERM=1")
	}
	command.Env = fixtureCommandEnvironment(t, environment...)
	process, err := launchFixtureProcess(command, cancel)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = leash.Close()
		process.stopAndWait()
	})
	readyLine, err := waitFixtureLine(t.Context(), hostReadyEvent, process)
	if err != nil || readyLine != "ready" {
		output, _ := process.stopAndWait()
		t.Fatalf("fake host readiness = %q, error %v\n%s", readyLine, err, output)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe ready fake host: state=%s error=%v", state, err)
	}
	hostRef := exact.Ref()
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if resistTerm {
		termLine, err := waitFixtureLine(t.Context(), hostTermEvent, process)
		if err != nil || termLine != "term-observed" {
			output, _ := process.stopAndWait()
			t.Fatalf("fake host resisted-SIGTERM acknowledgement = %q, error %v\n%s", termLine, err, output)
		}
		after, state, err := (identity.KernelProber{}).Probe(hostRef.Pid)
		if err != nil || state != identity.Alive || !identity.SameIdentity(after, hostRef) {
			t.Fatalf("fake host changed identity after resisted SIGTERM: state=%s error=%v", state, err)
		}
		if err := leash.Close(); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-process.done:
		output, err := process.wait()
		if err != nil {
			t.Fatalf("fake host lifetime cleanup failed: %v\n%s", err, output)
		}
	case <-t.Context().Done():
		output, err := process.stopAndWait()
		t.Fatalf("fake host outlived its terminal event: %v\n%s", err, output)
	}
	if !resistTerm {
		_ = leash.Close()
	}
	stopped, err := os.ReadFile(filepath.Join(turn, "host-stopped"))
	if err != nil || string(stopped) != "stopped\n" {
		t.Fatalf("fake host orderly-stop acknowledgement = %q, %v", stopped, err)
	}
}
