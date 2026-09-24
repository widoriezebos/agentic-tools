package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
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

func releaseFixtureFIFO(ctx context.Context, t *testing.T, path string) {
	t.Helper()
	if err := writeFixtureEvent(ctx, path); err != nil {
		t.Fatalf("release event %s: %v", path, err)
	}
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

func readFixtureExactRef(t *testing.T, path string) identity.Ref {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := identity.ParseRef(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse fixture process reference %q: %v", data, err)
	}
	return ref
}

func identityString(ref identity.Ref) string {
	value, _ := identity.EncodeRef(ref)
	return value
}

func assertFixtureExactDeath(t *testing.T, ref identity.Ref) {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(ref.Pid)
	if err == nil && (state == identity.Dead || state == identity.Alive && (!identity.SameIdentity(exact, ref) || exact.Zombie)) {
		return
	}
	t.Fatalf("exact fixture process remained alive at %s: state=%s zombie=%t error=%v", identityString(ref), state, exact.Zombie, err)
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

type fixtureDriver struct {
	*fixtureProcess
	descendants       *os.File
	descendantsJoined bool
	descendantErr     error
}

func startFixtureDriver(t *testing.T, driver string, environment ...string) *fixtureDriver {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	command := exec.CommandContext(ctx, "bash", driver)
	command.Cancel = func() error {
		err := command.Process.Signal(syscall.SIGTERM)
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}
	command.Env = fixtureCommandEnvironment(t, environment...)
	deathReader, deathWriter, err := os.Pipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	command.ExtraFiles = append(command.ExtraFiles, deathWriter)
	process, err := launchFixtureProcess(command, cancel)
	if err != nil {
		_ = deathReader.Close()
		_ = deathWriter.Close()
		t.Fatal(err)
	}
	if err := deathWriter.Close(); err != nil {
		process.stopAndWait()
		_ = deathReader.Close()
		t.Fatal(err)
	}
	fixture := &fixtureDriver{fixtureProcess: process, descendants: deathReader}
	t.Cleanup(func() {
		fixture.stopAndWait()
		_ = fixture.joinDescendants()
	})
	return fixture
}

func (driver *fixtureDriver) joinDescendants() error {
	if !driver.descendantsJoined {
		_, driver.descendantErr = io.Copy(io.Discard, driver.descendants)
		closeErr := driver.descendants.Close()
		if driver.descendantErr == nil {
			driver.descendantErr = closeErr
		}
		driver.descendantsJoined = true
	}
	return driver.descendantErr
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

func TestFixtureBedConsumesWorkersNamespacesStateAndCleansConcurrentRed(t *testing.T) {
	t.Parallel()
	testRoot := t.TempDir()
	observed := filepath.Join(testRoot, "observed")
	if err := os.Mkdir(observed, 0o700); err != nil {
		t.Fatal(err)
	}
	readyPath := filepath.Join(testRoot, "ready")
	survivorBlock := filepath.Join(testRoot, "survivor-block")
	survivorDelay := filepath.Join(testRoot, "survivor-delay")
	survivorReady := filepath.Join(testRoot, "survivor-ready")
	makeFixtureFIFO(t, readyPath)
	makeFixtureFIFO(t, survivorBlock)
	makeFixtureFIFO(t, survivorDelay)
	makeFixtureFIFO(t, survivorReady)
	for _, scenario := range []string{"positive", "red", "late"} {
		makeFixtureFIFO(t, filepath.Join(testRoot, "release-"+scenario))
	}

	budget, err := filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "fixture-budget.sh"))
	if err != nil {
		t.Fatal(err)
	}
	runner, err := filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "fixture-bed-scenarios.sh"))
	if err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(testRoot, "scenario.sh")
	childSource := `#!/usr/bin/env bash
set -euo pipefail
source "$FIXTURE_BUDGET"
scenario=$(harness_fixture_bed_child_scenario controlled "$@")
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
  "$scenario" "${METASYSTEM_TEST_WORKERS:-}" "$HOME" \
  "$METASYSTEM_SUPERVISION_REGISTRY_HOME" "$METASYSTEM_PROOF_ADMISSION_TEST_DIR" \
  "$METASYSTEM_FIXTURE_NAMESPACE" "$TMPDIR" >"$FIXTURE_OBSERVED/$scenario"
if [[ "$scenario" == red ]]; then
  /bin/sh -c '
    exec 6<"$1"
    IFS= read -r _ <&6
    trap "exit 0" TERM INT
    set +e
    ref_output=$("$2" proc ref --pid "$$" 2>&1)
    ref_status=$?
    set -e
    printf "status=%s\n%s\n" "$ref_status" "$ref_output" >"$6"
    if [ "$ref_status" -ne 0 ]; then
      cat "$6" >&2
      exit "$ref_status"
    fi
    printf "%s\n" "$ref_output" >"$3"
    printf "ready\n" >"$4"
    exec 7<"$5"
    IFS= read -r _ <&7
  ' fixture-survivor "$FIXTURE_SURVIVOR_DELAY" "$METASYSTEM_BIN" \
    "$FIXTURE_SURVIVOR_REF" "$FIXTURE_SURVIVOR_READY" "$FIXTURE_SURVIVOR_BLOCK" \
    "$FIXTURE_SURVIVOR_REF_STATUS" &
  survivor_pid=$!
  IFS= read -r _ <"$FIXTURE_SURVIVOR_READY"
  harness_fixture_record_pid "$survivor_pid"
fi
exec 8<>"$FIXTURE_CONTROL/release-$scenario"
printf '%s\n' "$scenario" >"$FIXTURE_READY"
IFS= read -r _ <&8
[[ "$scenario" != red ]] || exit 7
`
	if err := testexec.WriteFile(child, []byte(childSource), 0o755); err != nil {
		t.Fatal(err)
	}
	driver := filepath.Join(testRoot, "driver.sh")
	driverSource := `#!/usr/bin/env bash
set -euo pipefail
source "$FIXTURE_BUDGET"
source "$FIXTURE_RUNNER"
harness_fixture_cap() { printf '30\n'; }
run_fixture_bed_scenarios controlled "controlled fixtures passed" "$FIXTURE_CHILD" positive red late
`
	if err := testexec.WriteFile(driver, []byte(driverSource), 0o755); err != nil {
		t.Fatal(err)
	}

	ready, err := os.OpenFile(readyPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	readyEvents := startFixtureLineReader(ready, 3)
	t.Cleanup(readyEvents.closeAndWait)
	delay, err := os.OpenFile(survivorDelay, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer delay.Close()
	refStatusPath := filepath.Join(testRoot, "survivor-ref-status")
	fixture := startFixtureDriver(t, driver,
		"FIXTURE_BUDGET="+budget,
		"FIXTURE_RUNNER="+runner,
		"FIXTURE_CHILD="+child,
		"FIXTURE_CONTROL="+testRoot,
		"FIXTURE_OBSERVED="+observed,
		"FIXTURE_READY="+readyPath,
		"FIXTURE_SURVIVOR_DELAY="+survivorDelay,
		"FIXTURE_SURVIVOR_READY="+survivorReady,
		"FIXTURE_SURVIVOR_REF="+filepath.Join(testRoot, "survivor.ref"),
		"FIXTURE_SURVIVOR_REF_STATUS="+refStatusPath,
		"FIXTURE_SURVIVOR_BLOCK="+survivorBlock,
		"METASYSTEM_BIN="+commandTestExecutable(t),
		"METASYSTEM_TEST_WORKERS=2",
		"METASYSTEM_FIXTURE_SCENARIO_CONCURRENCY=9",
	)
	nextReady := func() string {
		t.Helper()
		name, err := waitFixtureLine(t.Context(), readyEvents, fixture.fixtureProcess)
		if err != nil {
			refStatus, _ := os.ReadFile(refStatusPath)
			fixture.stopAndWait()
			t.Fatalf("fixture readiness event was not published: %v\nproc-ref witness:\n%s", err, refStatus)
		}
		return name
	}
	if first := nextReady(); first != "positive" {
		t.Fatalf("delayed survivor allowed %q to acknowledge before positive", first)
	}
	if _, err := os.Stat(filepath.Join(observed, "late")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("third scenario crossed the two-worker boundary before a release: %v", err)
	}
	if _, err := delay.WriteString("release\n"); err != nil {
		t.Fatal(err)
	}
	if second := nextReady(); second != "red" {
		t.Fatalf("survivor acknowledgement released %q, want red", second)
	}
	survivorRef := readFixtureExactRef(t, filepath.Join(testRoot, "survivor.ref"))
	releaseFixtureFIFO(t.Context(), t, filepath.Join(testRoot, "release-positive"))
	releaseFixtureFIFO(t.Context(), t, filepath.Join(testRoot, "release-red"))
	if third := nextReady(); third != "late" {
		t.Fatalf("released worker started %q, want late", third)
	}
	releaseFixtureFIFO(t.Context(), t, filepath.Join(testRoot, "release-late"))
	output, err := fixture.wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("bed with a retained red exited %v, want status 1\n%s", err, output)
	}
	for _, want := range []string{
		"controlled fixture scenario passed: positive",
		"controlled fixture scenario failed: red (rc=7",
		"controlled fixture scenario passed: late",
		"=== controlled failed scenarios ===",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("bed output lacks %q:\n%s", want, output)
		}
	}

	namespaces := map[string]bool{}
	for _, scenario := range []string{"positive", "red", "late"} {
		data, err := os.ReadFile(filepath.Join(observed, scenario))
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Split(strings.TrimSpace(string(data)), "\t")
		if len(fields) != 7 || fields[0] != scenario || fields[1] != "1" {
			t.Fatalf("%s scenario worker record = %q", scenario, data)
		}
		namespace := fields[5]
		if namespaces[namespace] {
			t.Fatalf("scenario namespace reused: %s", namespace)
		}
		namespaces[namespace] = true
		for _, path := range fields[2:] {
			if path != namespace && !strings.HasPrefix(path, namespace+string(os.PathSeparator)) {
				t.Fatalf("%s mutable path %q is outside namespace %q", scenario, path, namespace)
			}
		}
	}

	if err := fixture.joinDescendants(); err != nil {
		t.Fatalf("join red scenario descendants: %v\n%s", err, output)
	}
	assertFixtureExactDeath(t, survivorRef)
}

func TestFixtureBedEarlyCancellationReclaimsOwnedScenarioDescendant(t *testing.T) {
	t.Parallel()
	testRoot := t.TempDir()
	readyPath := filepath.Join(testRoot, "ready")
	survivorReady := filepath.Join(testRoot, "survivor-ready")
	survivorBlock := filepath.Join(testRoot, "survivor-block")
	for _, path := range []string{readyPath, survivorReady, survivorBlock} {
		makeFixtureFIFO(t, path)
	}
	ready, err := os.OpenFile(readyPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ready.Close()

	budget, _ := filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "fixture-budget.sh"))
	runner, _ := filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "fixture-bed-scenarios.sh"))
	child := filepath.Join(testRoot, "scenario.sh")
	childSource := `#!/usr/bin/env bash
set -euo pipefail
source "$FIXTURE_BUDGET"
scenario=$(harness_fixture_bed_child_scenario cancellation "$@")
/bin/sh -c '
  trap "exit 0" TERM INT
  set +e
  ref_output=$("$1" proc ref --pid "$$" 2>&1)
  ref_status=$?
  set -e
  printf "status=%s\n%s\n" "$ref_status" "$ref_output" >"$5"
  if [ "$ref_status" -ne 0 ]; then
    cat "$5" >&2
    exit "$ref_status"
  fi
  printf "%s\n" "$ref_output" >"$2"
  printf "ready\n" >"$3"
  exec 7<"$4"
  IFS= read -r _ <&7
' fixture-survivor "$METASYSTEM_BIN" "$FIXTURE_SURVIVOR_REF" "$FIXTURE_SURVIVOR_READY" \
  "$FIXTURE_SURVIVOR_BLOCK" "$FIXTURE_SURVIVOR_REF_STATUS" &
survivor_pid=$!
IFS= read -r _ <"$FIXTURE_SURVIVOR_READY"
harness_fixture_record_pid "$survivor_pid"
printf '%s\n' "$scenario" >"$FIXTURE_READY"
exec 8<"$FIXTURE_SURVIVOR_BLOCK"
IFS= read -r _ <&8
`
	if err := testexec.WriteFile(child, []byte(childSource), 0o755); err != nil {
		t.Fatal(err)
	}
	driver := filepath.Join(testRoot, "driver.sh")
	driverSource := `#!/usr/bin/env bash
set -euo pipefail
source "$FIXTURE_BUDGET"
source "$FIXTURE_RUNNER"
harness_fixture_cap() { printf '30\n'; }
run_fixture_bed_scenarios cancellation "cancellation fixtures passed" "$FIXTURE_CHILD" cancellation
`
	if err := testexec.WriteFile(driver, []byte(driverSource), 0o755); err != nil {
		t.Fatal(err)
	}

	refStatusPath := filepath.Join(testRoot, "survivor-ref-status")
	fixture := startFixtureDriver(t, driver,
		"FIXTURE_BUDGET="+budget,
		"FIXTURE_RUNNER="+runner,
		"FIXTURE_CHILD="+child,
		"FIXTURE_READY="+readyPath,
		"FIXTURE_SURVIVOR_READY="+survivorReady,
		"FIXTURE_SURVIVOR_REF="+filepath.Join(testRoot, "survivor.ref"),
		"FIXTURE_SURVIVOR_REF_STATUS="+refStatusPath,
		"FIXTURE_SURVIVOR_BLOCK="+survivorBlock,
		"METASYSTEM_BIN="+commandTestExecutable(t),
		"METASYSTEM_TEST_WORKERS=1",
	)
	readyEvents := startFixtureLineReader(ready, 1)
	t.Cleanup(readyEvents.closeAndWait)
	event, err := waitFixtureLine(t.Context(), readyEvents, fixture.fixtureProcess)
	if err != nil {
		refStatus, _ := os.ReadFile(refStatusPath)
		fixture.stopAndWait()
		t.Fatalf("cancellation fixture did not become ready: %v\nproc-ref witness:\n%s", err, refStatus)
	}
	if event != "cancellation" {
		t.Fatalf("cancellation fixture event = %q", event)
	}
	survivorRef := readFixtureExactRef(t, filepath.Join(testRoot, "survivor.ref"))
	fixture.cancel()
	output, waitErr := fixture.wait()
	if waitErr == nil {
		t.Fatal("cancelled fixture driver exited successfully")
	}
	if err := fixture.joinDescendants(); err != nil {
		t.Fatalf("join cancelled fixture descendants: %v\n%s", err, output)
	}
	assertFixtureExactDeath(t, survivorRef)
}

func TestHarnessFixtureGoTestConsumesInheritedWorkerAllowance(t *testing.T) {
	t.Parallel()
	module := t.TempDir()
	if err := testexec.WriteFile(filepath.Join(module, "go.mod"), []byte("module fixtureworkers\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testSource := `package fixtureworkers

import (
  "flag"
  "runtime"
  "testing"
)

func TestInheritedWorkerAllowance(t *testing.T) {
  if got := runtime.GOMAXPROCS(0); got != 2 {
    t.Fatalf("GOMAXPROCS = %d, want 2", got)
  }
  parallel := flag.Lookup("test.parallel")
  if parallel == nil || parallel.Value.String() != "2" {
    t.Fatalf("test.parallel = %v, want 2", parallel)
  }
}
`
	if err := testexec.WriteFile(filepath.Join(module, "worker_test.go"), []byte(testSource), 0o644); err != nil {
		t.Fatal(err)
	}
	budget, _ := filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "fixture-budget.sh"))
	command := exec.Command("bash", "-c", `source "$1"; harness_fixture_go_test "$2" -count=1 ./...`, "bash", budget, module)
	command.Env = append(os.Environ(), "METASYSTEM_TEST_WORKERS=2")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("shared fixture Go consumer did not apply inherited workers: %v\n%s", err, output)
	}
}

func TestFixtureWithoutOuterProofScrubsWitnessHandoff(t *testing.T) {
	t.Parallel()
	budget, _ := filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "fixture-budget.sh"))
	command := exec.Command("bash", "-c", `
source "$1"
harness_fixture_without_outer_proof bash -c 'printf "%s|%s|%s\n" "${METASYSTEM_GATE_WITNESS+x}" "${METASYSTEM_GATE_WITNESS_WRITE+x}" "$FIXTURE_SENTINEL"'
printf '%s|%s|%s\n' "$METASYSTEM_GATE_WITNESS" "$METASYSTEM_GATE_WITNESS_WRITE" "$FIXTURE_SENTINEL"
`, "bash", budget)
	command.Env = append(os.Environ(),
		"METASYSTEM_GATE_WITNESS=outer-read",
		"METASYSTEM_GATE_WITNESS_WRITE=outer-write",
		"FIXTURE_SENTINEL=retained",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture proof scrub failed: %v\n%s", err, output)
	}
	if want := "||retained\nouter-read|outer-write|retained\n"; string(output) != want {
		t.Fatalf("fixture proof scrub output = %q, want %q", output, want)
	}
}

func TestFixtureGoConsumersUseSharedWorkerBoundary(t *testing.T) {
	t.Parallel()
	scripts := map[string]int{
		"dispatch-fixtures.sh":         1,
		"land-fixtures.sh":             14,
		"return-schema-fixtures.sh":    3,
		"supervision-fixtures.sh":      10,
		"supervision-hook-fixtures.sh": 1,
	}
	for name, minimum := range scripts {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", name))
		if err != nil {
			t.Fatal(err)
		}
		if count := strings.Count(string(data), "harness_fixture_go_test"); count < minimum {
			t.Errorf("%s has %d shared Go consumer calls, want at least %d", name, count, minimum)
		}
	}
	budgetData, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "fixture-budget.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`GOMAXPROCS=$workers go test -p="$workers" -parallel="$workers"`,
		`METASYSTEM_TEST_WORKERS must be a positive integer`,
	} {
		if !strings.Contains(string(budgetData), required) {
			t.Errorf("shared fixture Go consumer lacks %q", required)
		}
	}
	landData, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "land-fixtures.sh"))
	if err != nil {
		t.Fatal(err)
	}
	land := string(landData)
	if !strings.Contains(land, "METASYSTEM_TEST_WORKERS PATH") {
		t.Error("scrubbed receipt environment does not preserve the scenario worker allocation")
	}
	for _, required := range []string{
		`fixture_minimum_cap_min=$(harness_fixture_semantic_cap minimum-minutes)`,
		`fixture_budget_refusal_cap_min=$(harness_fixture_semantic_cap landing-budget-refusal-minutes)`,
		`[[ ! -e "$full_chain_budget_command_marker" ]]`,
	} {
		if !strings.Contains(land, required) {
			t.Errorf("land fixture deterministic contract lacks %q", required)
		}
	}
	if strings.Contains(land, "--cap-min 1") || strings.Contains(land, "--cap-min 3") {
		t.Fatal("land fixture still hard-codes a proof cap instead of its semantic budget input")
	}
}

func TestLandingReceiptRunnerObservationKeepsUnknownTracked(t *testing.T) {
	t.Parallel()
	observer := filepath.Join(t.TempDir(), "observer")
	observerSource := `#!/usr/bin/env bash
set -euo pipefail
case "${MODE:?}:$1:$2" in
  exact:proc:ref) printf '%s\n' 'pid=4242;started=10' ;;
  replaced:proc:ref) printf '%s\n' 'pid=4242;started=11' ;;
  dead:proc:ref|unknown:proc:ref) exit 1 ;;
  dead:proc:probe) printf '%s\n' '{"pid":4242,"liveness":"dead"}' ;;
  unknown:proc:probe) printf '%s\n' '{"pid":4242,"liveness":"unknown","error":"permission denied"}'; exit 1 ;;
  *) exit 64 ;;
esac
`
	if err := testexec.WriteFile(observer, []byte(observerSource), 0o755); err != nil {
		t.Fatal(err)
	}
	budget, err := filepath.Abs(filepath.Join("..", "..", "scripts", "agents", "fixture-budget.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		mode string
		rc   string
	}{{"exact", "1"}, {"replaced", "0"}, {"dead", "0"}, {"unknown", "2"}} {
		command := exec.Command("/bin/bash", "-c", `source "$1"; set +e; harness_fixture_exact_process_gone "$2" 4242 'pid=4242;started=10'; rc=$?; set -e; printf 'rc=%s\n' "$rc"`, "bash", budget, observer)
		command.Env = fixtureCommandEnvironment(t, "MODE="+test.mode)
		output, err := command.CombinedOutput()
		if err != nil || !strings.Contains(string(output), "rc="+test.rc+"\n") {
			t.Fatalf("%s process observation = %q, %v; want rc=%s", test.mode, output, err, test.rc)
		}
		if test.mode == "unknown" && !strings.Contains(string(output), "identity is indeterminate") {
			t.Fatalf("unknown process observation did not retain its diagnostic: %s", output)
		}
	}
	landData, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "land-fixtures.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`harness_fixture_exact_process_gone "$source_engine" "$runner_pid" "$runner_ref"`,
		`tracking retained: $runner_ref`,
	} {
		if !strings.Contains(string(landData), required) {
			t.Fatalf("landing teardown lacks fail-closed tracking behavior %q", required)
		}
	}
}

func TestLandingReceiptRefusesSemanticBudgetBeforeCommand(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	runReceiptGit(t, root, "config", "user.name", "fixture")
	runReceiptGit(t, root, "config", "user.email", "fixture@example.invalid")
	runReceiptGit(t, root, "config", "goal.sync-remote", "local")
	runReceiptGit(t, root, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	runReceiptGit(t, root, "config", "metasystem.goal.machine", "fixture-machine")
	writeReceiptFixture(t, root, "metasystem.conf", "metasystem.version=1\nmetasystem.runtimes=fake\n")
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
	runReceiptGit(t, root, "add", "-A")
	runReceiptGit(t, root, "commit", "-qm", "semantic budget fixture")
	runReceiptGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	runReceiptGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	tree := runReceiptGit(t, root, "write-tree")
	marker := filepath.Join(root, "command-ran")
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe receipt command parent: state=%s error=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "batch-proof-main", exact.Pid, exact.StartedAt.Unix(),
		exact.StartTicks, exact.BootID, "batch-proof-main", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(commandTestExecutable(t), "landing", "test-receipt",
		"--root", root, "--tree", tree, "--command", "touch "+marker,
		"--goal", "fixture-budget", "--cap-min", "13")
	command.Env = fixtureCommandEnvironment(t, "METASYSTEM_GOAL_NOW="+now.Add(2*time.Minute).Format(time.RFC3339))
	output, commandErr := command.CombinedOutput()
	code := 0
	if commandErr != nil {
		var exit *exec.ExitError
		if !errors.As(commandErr, &exit) {
			t.Fatalf("run public receipt refusal: %v", commandErr)
		}
		code = exit.ExitCode()
	}
	stderr := string(output)
	if code != proofrun.ExitAdmissionRefused || !strings.Contains(stderr, "BUDGET_REFUSED") {
		t.Fatalf("over-budget receipt exit=%d, stderr=%q", code, stderr)
	}
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
		{name: "one-minute-exact-expiry", capMinutes: 1, wantExit: proofrun.ExitAdmissionRefused, wantReason: "has passed at semantic time", refusesExpiry: true},
		{name: "one-minute-after-expiry", capMinutes: 1, advance: time.Nanosecond, wantExit: proofrun.ExitAdmissionRefused, wantReason: "has passed at semantic time", wantObserved: 2, refusesExpiry: true},
		{name: "three-minute-event-delayed", capMinutes: 3, advance: -time.Nanosecond, wantLaunches: 1},
		{name: "outer-cancellation", capMinutes: 1, advance: -time.Second, cancelParent: true, wantExit: proofrun.ExitAdmissionRefused, wantReason: context.Canceled.Error()},
	}
}

func TestLandingReceiptPublicSemanticClockBoundariesAndDelayedCompletion(t *testing.T) {
	t.Parallel()
	if selected := os.Getenv("GO_WANT_FIXTURE_RECEIPT_CLOCK_CHILD"); selected != "" {
		for _, testCase := range receiptClockFixtureCases() {
			if testCase.name == selected {
				runLandingReceiptPublicSemanticClockCase(t, testCase)
				return
			}
		}
		t.Fatalf("unknown receipt clock fixture case %q", selected)
	}
	for _, testCase := range receiptClockFixtureCases() {
		t.Run(testCase.name, func(t *testing.T) {
			startedAt := time.Date(2026, time.September, 21, 8, 0, 0, 0, time.UTC)
			root, _ := proofExtensionGoalFixtureAt(t, startedAt)
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			confPath := filepath.Join(root, "metasystem.conf")
			configuration, err := os.ReadFile(confPath)
			if err != nil || !bytes.Contains(configuration, []byte(proofrun.AdmissionCapKey+"=4")) {
				t.Fatalf("read pinned receipt host capacity: %v", err)
			}
			configuration = bytes.Replace(configuration, []byte(proofrun.AdmissionCapKey+"=4"), []byte(proofrun.AdmissionCapKey+"=1"), 1)
			if err := os.WriteFile(confPath, configuration, 0o644); err != nil {
				t.Fatal(err)
			}
			writeReceiptFixture(t, root, ".gitignore", "artifacts/\n")
			runReceiptGit(t, root, "add", "metasystem.conf", ".gitignore")
			runReceiptGit(t, root, "commit", "-qm", "public receipt clock fixture")
			runReceiptGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
			runReceiptGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
			command := exec.Command(commandTestExecutable(t), "-test.run=^TestLandingReceiptPublicSemanticClockBoundariesAndDelayedCompletion$", "-test.v")
			command.Env = append(testenv.WithoutInheritedControls(os.Environ()),
				"GO_WANT_FIXTURE_RECEIPT_CLOCK_CHILD="+testCase.name,
				"FIXTURE_RECEIPT_CLOCK_ROOT="+root,
				"METASYSTEM_GOAL_NOW="+startedAt.Format(time.RFC3339Nano),
				"METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+root)
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

func TestLandingReceiptShellCallerCarriesAuthorizedClock(t *testing.T) {
	t.Parallel()
	if arguments, private := receiptClockPrivateChildArguments(); private {
		if len(arguments) != 3 {
			t.Fatalf("private receipt clock child arguments=%q", arguments)
		}
		clock := os.Getenv(goalNowEnvironment)
		if clock == "" {
			t.Fatal("private receipt child received no authorized semantic clock")
		}
		if err := os.WriteFile(arguments[0], []byte("private-clock="+clock+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		ready, err := os.OpenFile(arguments[1], os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ready.WriteString("ready\n"); err != nil || ready.Close() != nil {
			t.Fatalf("publish private receipt readiness: %v", err)
		}
		release, err := os.Open(arguments[2])
		if err != nil {
			t.Fatal(err)
		}
		var event [1]byte
		if _, err := io.ReadFull(release, event[:]); err != nil {
			t.Fatal(err)
		}
		_ = release.Close()
		return
	}

	fixtureInstant := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, capMinutes := range []int{1, 3} {
		capMinutes := capMinutes
		t.Run(fmt.Sprintf("cap-%d-minute", capMinutes), func(t *testing.T) {
			root, _ := proofExtensionGoalFixtureAt(t, fixtureInstant)
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			writeReceiptFixture(t, root, ".gitignore", "artifacts/\n")
			runReceiptGit(t, root, "add", ".gitignore")
			runReceiptGit(t, root, "commit", "-qm", "ignore receipt artifacts")
			runReceiptGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
			runReceiptGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
			scriptRoot := t.TempDir()
			for _, directory := range []string{filepath.Join(scriptRoot, "scripts", "agents"), filepath.Join(scriptRoot, "bin")} {
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, relative := range []string{
				"scripts/agents/land-fixtures.sh",
				"scripts/agents/fixture-budget.sh",
				"scripts/agents/fixture-bed-scenarios.sh",
			} {
				data, err := os.ReadFile(filepath.Join("..", "..", relative))
				if err != nil {
					t.Fatal(err)
				}
				if err := testexec.WriteFile(filepath.Join(scriptRoot, relative), data, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			engine := filepath.Join(scriptRoot, "bin", "metasystem")
			wrapper := fmt.Sprintf("#!/usr/bin/env bash\nexport GO_WANT_BATCH_E2E_COMMAND=1\nexec %q \"$@\"\n", commandTestExecutable(t))
			if err := testexec.WriteFile(engine, []byte(wrapper), 0o755); err != nil {
				t.Fatal(err)
			}
			capability := filepath.Join(scriptRoot, "receipt-clock.capability")
			if err := testexec.WriteFile(capability, []byte("receipt-clock-boundary\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			readyPath, releasePath := filepath.Join(t.TempDir(), "ready"), filepath.Join(t.TempDir(), "release")
			makeFixtureFIFO(t, readyPath)
			makeFixtureFIFO(t, releasePath)
			ready, err := os.OpenFile(readyPath, os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			readyEvents := startFixtureLineReader(ready, 1)
			release, err := os.OpenFile(releasePath, os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer release.Close()
			observed := filepath.Join(t.TempDir(), "observed-clock")
			resultPath := filepath.Join(t.TempDir(), "launch-result.json")
			namespace := filepath.Join(t.TempDir(), "namespace")
			ctx, cancel := context.WithCancel(t.Context())
			command := exec.CommandContext(ctx, "bash", filepath.Join(scriptRoot, "scripts", "agents", "land-fixtures.sh"),
				"--fixture-bed-child", "receipt-clock-boundary", capability)
			command.Env = fixtureCommandEnvironment(t,
				"METASYSTEM_FIXTURE_NAMESPACE="+namespace,
				"METASYSTEM_TEST_WORKERS=1",
				"METASYSTEM_RECEIPT_CLOCK_FIXTURE_ROOT="+root,
				"METASYSTEM_RECEIPT_CLOCK_CAP_MIN="+strconv.Itoa(capMinutes),
				"METASYSTEM_RECEIPT_CLOCK_TEST_EXECUTABLE="+commandTestExecutable(t),
				"METASYSTEM_RECEIPT_CLOCK_READY="+readyPath,
				"METASYSTEM_RECEIPT_CLOCK_RELEASE="+releasePath,
				"METASYSTEM_RECEIPT_CLOCK_OBSERVED="+observed,
				"METASYSTEM_RECEIPT_CLOCK_RESULT="+resultPath)
			started := time.Now()
			process, err := launchFixtureProcess(command, cancel)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { process.stopAndWait() })
			event, waitErr := waitFixtureLine(t.Context(), readyEvents, process)
			if waitErr != nil || event != "ready" {
				_ = ready.Close()
				t.Fatalf("private receipt child did not reach the delayed event: event=%q err=%v", event, waitErr)
			}
			readyEvents.closeAndWait()
			if _, err := release.WriteString("release\n"); err != nil {
				t.Fatal(err)
			}
			output, processErr := process.wait()
			if processErr != nil {
				t.Fatalf("shell receipt boundary: %v\n%s", processErr, output)
			}
			var result proofrun.LaunchResult
			encoded, err := os.ReadFile(resultPath)
			if err != nil || json.Unmarshal(encoded, &result) != nil || result.ExitStatus != 0 || result.AttemptID == "" {
				t.Fatalf("shell receipt result=%+v read=%v bytes=%s", result, err, encoded)
			}
			attempt, err := proofrun.ReadAttempt(root, result.AttemptID)
			wantDeadline := fixtureInstant.Add(time.Duration(capMinutes) * time.Minute).Format(time.RFC3339Nano)
			if err != nil || attempt.StartedAt != fixtureInstant.Format(time.RFC3339Nano) || attempt.Deadline != wantDeadline || attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess {
				t.Fatalf("public command clock attempt=%+v err=%v", attempt, err)
			}
			for _, marker := range []string{
				"shell clock=" + fixtureInstant.Format(time.RFC3339),
				fmt.Sprintf("public receipt completed cap-min=%d", capMinutes),
				"private-clock=" + fixtureInstant.Format(time.RFC3339),
			} {
				if !strings.Contains(output, marker) {
					t.Fatalf("shell receipt output lacks %q:\n%s", marker, output)
				}
			}
			t.Logf("receipt shell/public/private clock witness cap=%dm physical=%s", capMinutes, time.Since(started))
		})
	}
}

func receiptClockPrivateChildArguments() ([]string, bool) {
	for index, argument := range os.Args {
		if argument == "--receipt-clock-private-child" {
			return os.Args[index+1:], true
		}
	}
	return nil, false
}

func runLandingReceiptPublicSemanticClockCase(t *testing.T, testCase receiptClockFixtureCase) {
	startedAt := time.Date(2026, time.September, 21, 8, 0, 0, 0, time.UTC)
	root, err := filepath.EvalSymlinks(os.Getenv("FIXTURE_RECEIPT_CLOCK_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	const admissionDirectoryEnvironment = "METASYSTEM_PROOF_ADMISSION_TEST_DIR"
	previousAdmissionDir, hadAdmissionDir := os.LookupEnv(admissionDirectoryEnvironment)
	if err := os.Setenv(admissionDirectoryEnvironment, filepath.Join(t.TempDir(), "host-admission")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadAdmissionDir {
			_ = os.Setenv(admissionDirectoryEnvironment, previousAdmissionDir)
		} else {
			_ = os.Unsetenv(admissionDirectoryEnvironment)
		}
	})
	confPath := filepath.Join(root, "metasystem.conf")
	announceProofFixtureHolder(t, root)
	holder, err := proofrun.AcquireHostResources(context.Background(), root, confPath, "heavy", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Close() })
	tree := runReceiptGit(t, root, "write-tree")
	launchCount := filepath.Join(t.TempDir(), "native-launches")
	resultPath := filepath.Join(t.TempDir(), "launch-result.json")
	semanticNow := startedAt
	previousGoalNow, hadGoalNow := os.LookupEnv(goalNowEnvironment)
	t.Cleanup(func() {
		if hadGoalNow {
			_ = os.Setenv(goalNowEnvironment, previousGoalNow)
		} else {
			_ = os.Unsetenv(goalNowEnvironment)
		}
	})
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
	code, _, problem := captureChannelOutput(t, func() int {
		return runLandingTestReceiptWithContext(parent, resolveClock, args)
	})
	if code != testCase.wantExit || observedWait != 1 || !strings.Contains(problem, testCase.wantReason) {
		t.Fatalf("public boundary exit=%d want=%d waits=%d stderr=%q", code, testCase.wantExit, observedWait, problem)
	}
	if testCase.cancelParent && strings.Contains(problem, "has passed at semantic time") {
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
	if testCase.refusesExpiry && launchResult.Disposition != proofrun.DispositionAdmissionRefused {
		t.Fatalf("semantic expiry disposition=%+v", launchResult)
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
	} else if attempt.Terminal.Result == proofrun.TerminalSuccess || len(proofrun.CommittedDeliveryReceipt(attempt)) != 0 {
		t.Fatalf("refused boundary published success: %+v", attempt)
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
	assertHostAdmissionClean(t, os.Getenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR"), 1)
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

func TestStandaloneChannelFixtureOwnsFakeServerAndMutableState(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "scripts", "agents", "channel-fixtures.sh")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	owner := strings.Index(source, `harness_fixture_owner "$source_root/metasystem"`)
	server := strings.Index(source, `METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value"`)
	if owner < 0 || server < 0 || owner > server {
		t.Fatal("standalone channel fixture does not bind its freshly minted owner before the fake server")
	}
	for _, required := range []string{
		`export HOME=$bed/home`,
		`export TMPDIR=$bed/tmp`,
		`export METASYSTEM_SUPERVISION_REGISTRY_HOME=$bed/registry`,
		`export METASYSTEM_PROOF_ADMISSION_TEST_DIR=$bed/proof-admission`,
		`export METASYSTEM_FIXTURE_NAMESPACE=$bed`,
		`harness_fixture_key channel-fake-server`,
		`harness_fixture_record_pid "$server_pid"`,
		`harness_fixture_reap`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("standalone channel fixture lacks private owner contract %q", required)
		}
	}
}

func TestChannelFakeServePublicCommandListensUnderSyntheticFixtureAuthority(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	state := filepath.Join(root, "state")
	leashPath := filepath.Join(root, "leash")
	makeFixtureFIFO(t, leashPath)
	leash, err := os.OpenFile(leashPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = leash.Close() })
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	readyEvents := startFixtureLineReader(readyRead, 1)
	t.Cleanup(readyEvents.closeAndWait)
	owner := syntheticFixtureOwnerKey(t, "channel-fake-server")
	ctx, cancel := context.WithCancel(t.Context())
	command := exec.CommandContext(ctx, commandTestExecutable(t), "channel", "fake", "serve", "--dir", state, "--ready-fd", "3")
	command.Env = fixtureCommandEnvironment(t,
		identity.FixtureOwnerEnv+"="+owner,
		fixtureLeashEnvironment+"="+leashPath,
	)
	command.ExtraFiles = []*os.File{readyWrite}
	process, err := launchFixtureProcess(command, cancel)
	if err != nil {
		_ = readyWrite.Close()
		t.Fatal(err)
	}
	if err := readyWrite.Close(); err != nil {
		process.stopAndWait()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = leash.Close()
		process.stopAndWait()
	})
	published, err := waitFixtureLine(t.Context(), readyEvents, process)
	if err != nil {
		output, _ := process.stopAndWait()
		t.Fatalf("public channel fake server exited before listener readiness: %v\n%s", err, output)
	}
	baseURLPath := filepath.Join(state, "base-url")
	baseURL, err := os.ReadFile(baseURLPath)
	if err != nil {
		t.Fatal(err)
	}
	if published != strings.TrimSpace(string(baseURL)) {
		t.Fatalf("inherited readiness address %q differs from public base-url %q", published, baseURL)
	}
	parsed, err := url.Parse(strings.TrimSpace(string(baseURL)))
	if err != nil {
		t.Fatal(err)
	}
	connection, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", parsed.Host)
	if err != nil {
		t.Fatalf("published channel listener %q did not accept a connection: %v", parsed.Host, err)
	}
	_ = connection.Close()
	if err := leash.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-process.done:
		output, err := process.wait()
		if err != nil {
			t.Fatalf("channel fake server did not stop with its owner leash: %v\n%s", err, output)
		}
	case <-t.Context().Done():
		output, err := process.stopAndWait()
		t.Fatalf("channel fake server outlived its owner leash: %v\n%s", err, output)
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
	hostDir := filepath.Join(fixtureRoot, "scripts", "agents", "hosts")
	if err := os.MkdirAll(hostDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fake.sh", "host-common.sh"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "hosts", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(filepath.Join(hostDir, name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
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
	command := exec.CommandContext(ctx, "/bin/bash", filepath.Join(hostDir, "fake.sh"),
		identity.FixtureOwnerEnv+"="+owner,
		"start-turn", "--mission", "fixture-host", "--turn-id", "fixture-turn",
		"--prompt", filepath.Join(turn, "prompt.md"), "--result", filepath.Join(turn, "result.json"),
		"--instance-tag", "fixture-host",
	)
	environment := []string{
		"METASYSTEM_BIN=" + commandTestExecutable(t),
		identity.FixtureOwnerEnv + "=" + owner,
		fixtureLeashEnvironment + "=" + leashPath,
		"METASYSTEM_FAKE_HOST_HOLD=1",
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

func TestSupervisionGoFixtureSuppliesCensusInputsAndRetainsFailures(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "supervision-go-fixtures.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, required := range []string{
		`cp "$bin" "$fixture_root/bin/metasystem"`,
		`scripts/agents/arm-supervision.sh`,
		`scripts/agents/dispatch.sh`,
		`scripts/agents/adapters/runtime-common.sh`,
		`scripts/agents/adapters/fake.sh`,
		`scripts/watch-background-jobs.sh`,
		`export METASYSTEM_CENSUS_PROCESS_FILE="$process_fixture"`,
		`--metasystem-root "$fixture_root" --scope "$repo"`,
		`--fingerprint "$fingerprint" --watcher-cap "$watcher_cap"`,
		`local status=$?`,
		`exit "$status"`,
		`cp "$registry" "$failure_dir/registry.jsonl"`,
		`"$repo/owner.out"`,
		`"$repo/artifacts/agents/supervision/owner.ndjson"`,
		`"$repo/artifacts/agents/supervision/state.json"`,
		`"$repo/artifacts/agents/supervision/last-census.json"`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("supervision Go fixture lacks %q", required)
		}
	}
	if strings.Contains(script, `census_generation_one "$census1" || true`) {
		t.Fatal("generation-one census assertion was weakened")
	}
}
