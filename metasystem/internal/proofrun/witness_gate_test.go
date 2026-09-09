package proofrun

import (
	"bytes"
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
	"testing"
)

const witnessGateInvocation = `set +e
root=$PWD
delivery_contract=0
source scripts/agents/witness-gate.sh
source_status=$?
export WITNESS_GATE_SOURCE_STATUS="$source_status"
export WITNESS_GATE_LOCAL_STATE="${witness_state:-}"
export WITNESS_GATE_LOCAL_SNAPSHOT="${witness_snap:-}"
exec "$WITNESS_GATE_HELPER" -test.run=^TestWitnessGateHelperProcess$ -- observe
`

type witnessGateFixture struct {
	t          *testing.T
	repo       string
	scratch    string
	helperBin  string
	helperPath string
	realTar    string
	countPath  string
	reportPath string
}

type witnessGateRunOptions struct {
	fallback      string
	gateStatuses  string
	writeWitness  bool
	writeBinary   bool
	force         bool
	seed          bool
	archiveStatus int
}

type witnessGateObservation struct {
	SourceStatus  int    `json:"sourceStatus"`
	Witness       string `json:"witness"`
	WitnessRoot   string `json:"witnessRoot"`
	WitnessRun    string `json:"witnessRun"`
	WitnessExport string `json:"witnessExport"`
	LocalState    string `json:"localState"`
	LocalSnapshot string `json:"localSnapshot"`
}

type witnessGateRunResult struct {
	status      int
	launches    int
	output      string
	observation witnessGateObservation
}

func TestWitnessGateHistoricalSourceRelaunchedAfterFailure(t *testing.T) {
	source := historicalWitnessGateSource(t)
	fixture := newWitnessGateFixtureWithSource(t, source)
	result := fixture.run(witnessGateRunOptions{fallback: "plain", gateStatuses: "23,0"})
	t.Logf("historical source exit status %d after %d gate launches", result.status, result.launches)
	if result.status != 0 || result.launches != 2 {
		t.Fatalf("historical source status = %d, gate launches = %d; want fallback status 0 after two launches\noutput:\n%s",
			result.status, result.launches, result.output)
	}
}

func TestWitnessGateExecutedFailureIsTerminal(t *testing.T) {
	for _, fallback := range []string{"plain", "none"} {
		t.Run(fallback, func(t *testing.T) {
			fixture := newWitnessGateFixture(t)
			result := fixture.run(witnessGateRunOptions{
				fallback:     fallback,
				gateStatuses: "23,0",
			})
			t.Logf("corrected source exit status %d after %d gate launch", result.status, result.launches)

			if result.status != 23 || result.launches != 1 {
				t.Errorf("source status = %d, gate launches = %d; want status 23 and one launch\noutput:\n%s", result.status, result.launches, result.output)
			}
			assertNoPublishedWitness(t, result.observation)
			fixture.assertScratchEntries()
		})
	}
}

func TestWitnessGateForceModeStaysIsolatedAndSingleLaunch(t *testing.T) {
	fixture := newWitnessGateFixture(t)
	result := fixture.run(witnessGateRunOptions{fallback: "plain", gateStatuses: "23,0", force: true})
	if result.status != 23 || result.launches != 1 {
		t.Fatalf("force-mode status=%d launches=%d; want executed status 23 and one launch\noutput:\n%s", result.status, result.launches, result.output)
	}
	if result.observation.Witness != "" || result.observation.WitnessRoot != "" || result.observation.WitnessRun != "" ||
		result.observation.WitnessExport != "" || result.observation.LocalState != "" || result.observation.LocalSnapshot != "" {
		t.Fatalf("force mode retained witness state: %+v", result.observation)
	}
	fixture.assertScratchEntries()
}

func TestWitnessGateSeedModeStaysIsolatedAndSingleLaunch(t *testing.T) {
	fixture := newWitnessGateFixture(t)
	result := fixture.run(witnessGateRunOptions{fallback: "plain", gateStatuses: "23,0", seed: true})
	if result.status != 23 || result.launches != 1 {
		t.Fatalf("seed-mode status=%d launches=%d; want executed status 23 and one launch\noutput:\n%s", result.status, result.launches, result.output)
	}
	if result.observation != (witnessGateObservation{SourceStatus: 23}) {
		t.Fatalf("seed mode retained witness state: %+v", result.observation)
	}
	fixture.assertScratchEntries()
}

func TestWitnessGateSuccessWithoutWitnessIsRefused(t *testing.T) {
	fixture := newWitnessGateFixture(t)
	result := fixture.run(witnessGateRunOptions{
		fallback:     "plain",
		gateStatuses: "0",
		writeBinary:  true,
	})

	if result.status != 1 || result.launches != 1 {
		t.Errorf("source status = %d, gate launches = %d; want status 1 and one launch\noutput:\n%s", result.status, result.launches, result.output)
	}
	if !strings.Contains(result.output, "without publishing witness evidence") {
		t.Errorf("missing evidence-publication diagnostic in output:\n%s", result.output)
	}
	assertNoPublishedWitness(t, result.observation)
	fixture.assertScratchEntries()
}

func TestWitnessGateSuccessArmsWitness(t *testing.T) {
	fixture := newWitnessGateFixture(t)
	result := fixture.run(witnessGateRunOptions{
		fallback:     "plain",
		gateStatuses: "0",
		writeWitness: true,
		writeBinary:  true,
	})

	if result.status != 0 || result.launches != 1 {
		t.Fatalf("source status = %d, gate launches = %d; want status 0 and one launch\noutput:\n%s", result.status, result.launches, result.output)
	}
	observation := result.observation
	if observation.Witness == "" || observation.WitnessRoot == "" || observation.WitnessRun == "" {
		t.Fatalf("published witness observation is incomplete: %+v", observation)
	}
	if observation.Witness != filepath.Join(observation.WitnessRoot, "witness.json") {
		t.Errorf("witness path = %q, root = %q", observation.Witness, observation.WitnessRoot)
	}
	if observation.WitnessExport != "" {
		t.Errorf("clean witness unexpectedly exported a frozen tree: %q", observation.WitnessExport)
	}
	if observation.LocalState != observation.WitnessRoot {
		t.Errorf("local witness state = %q, exported root = %q", observation.LocalState, observation.WitnessRoot)
	}
	if _, err := os.Stat(observation.Witness); err != nil {
		t.Errorf("published witness is absent: %v", err)
	}
	if observation.LocalSnapshot == "" {
		t.Error("clean snapshot path was not reported by the sourced script")
	} else if _, err := os.Stat(observation.LocalSnapshot); !os.IsNotExist(err) {
		t.Errorf("clean snapshot was not removed: %v", err)
	}
	gotBinary, err := os.ReadFile(filepath.Join(fixture.repo, "bin", "metasystem"))
	if err != nil {
		t.Fatalf("read staged binary: %v", err)
	}
	if string(gotBinary) != "proven test binary\n" {
		t.Errorf("staged binary = %q", gotBinary)
	}
	fixture.assertScratchEntries(observation.WitnessRoot)
	if !strings.Contains(result.output, "gate witness armed for this run's nested validations") {
		t.Errorf("missing arming diagnostic in output:\n%s", result.output)
	}
	if err := os.RemoveAll(observation.WitnessRoot); err != nil {
		t.Errorf("remove successful witness state: %v", err)
	}
}

func TestWitnessGatePreparationFallbackChoices(t *testing.T) {
	t.Run("plain preserves gate failure", func(t *testing.T) {
		fixture := newWitnessGateFixture(t)
		result := fixture.run(witnessGateRunOptions{
			fallback:      "plain",
			gateStatuses:  "29",
			archiveStatus: 41,
		})

		if result.status != 29 || result.launches != 1 {
			t.Errorf("source status = %d, gate launches = %d; want plain gate status 29 and one launch\noutput:\n%s", result.status, result.launches, result.output)
		}
		assertNoPublishedWitness(t, result.observation)
		fixture.assertScratchEntries()
	})

	t.Run("none runs no gate", func(t *testing.T) {
		fixture := newWitnessGateFixture(t)
		result := fixture.run(witnessGateRunOptions{
			fallback:      "none",
			gateStatuses:  "29",
			archiveStatus: 41,
		})

		if result.status != 0 || result.launches != 0 {
			t.Errorf("source status = %d, gate launches = %d; want status 0 and no launch\noutput:\n%s", result.status, result.launches, result.output)
		}
		assertNoPublishedWitness(t, result.observation)
		fixture.assertScratchEntries()
	})
}

func TestWitnessGateStagingFailureDoesNotPublishWitness(t *testing.T) {
	fixture := newWitnessGateFixture(t)
	result := fixture.run(witnessGateRunOptions{
		fallback:     "plain",
		gateStatuses: "0",
		writeWitness: true,
	})

	if result.status != 1 || result.launches != 1 {
		t.Errorf("source status = %d, gate launches = %d; want staging status 1 and one launch\noutput:\n%s", result.status, result.launches, result.output)
	}
	if !strings.Contains(result.output, "proven binary could not be published") {
		t.Errorf("missing binary-publication diagnostic in output:\n%s", result.output)
	}
	assertNoPublishedWitness(t, result.observation)
	fixture.assertScratchEntries()
}

func TestWitnessGateHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_WITNESS_GATE_HELPER") != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		fmt.Fprintln(os.Stderr, "witness gate test helper received no mode")
		os.Exit(97)
	}

	arguments := os.Args[separator+1:]
	var status int
	switch arguments[0] {
	case "go":
		status = witnessGateGoHelper(arguments[1:])
	case "gate":
		status = witnessGateCommandHelper()
	case "observe":
		status = witnessGateObservationHelper()
	case "mktemp":
		status = witnessGateMktempHelper(arguments[1:])
	case "tar":
		status = witnessGateTarHelper(arguments[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown witness gate test helper mode %q\n", arguments[0])
		status = 97
	}
	os.Exit(status)
}

func newWitnessGateFixture(t *testing.T) *witnessGateFixture {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate witness gate test source")
	}
	witnessGateSource := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "scripts", "agents", "witness-gate.sh"))
	source, err := os.ReadFile(witnessGateSource)
	if err != nil {
		t.Fatalf("read actual witness gate: %v", err)
	}
	return newWitnessGateFixtureWithSource(t, source)
}

func newWitnessGateFixtureWithSource(t *testing.T, source []byte) *witnessGateFixture {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	scratch := filepath.Join(root, "tmp")
	helperBin := filepath.Join(root, "helpers")
	for _, directory := range []string{
		filepath.Join(repo, "scripts", "agents"),
		scratch,
		helperBin,
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(repo, "scripts", "agents", "witness-gate.sh"), source, 0o600); err != nil {
		t.Fatal(err)
	}

	helperPath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	realTar, err := exec.LookPath("tar")
	if err != nil {
		t.Fatal(err)
	}
	writeWitnessGateWrapper(t, filepath.Join(repo, "scripts", "agents", "go-gate.sh"), "gate")
	writeWitnessGateWrapper(t, filepath.Join(helperBin, "go"), "go")
	writeWitnessGateWrapper(t, filepath.Join(helperBin, "mktemp"), "mktemp")
	writeWitnessGateWrapper(t, filepath.Join(helperBin, "tar"), "tar")

	runWitnessGateSetupCommand(t, repo, "git", "init", "-q")
	runWitnessGateSetupCommand(t, repo, "git", "config", "user.email", "witness-gate-test@example.invalid")
	runWitnessGateSetupCommand(t, repo, "git", "config", "user.name", "Witness Gate Test")
	runWitnessGateSetupCommand(t, repo, "git", "add", ".")
	runWitnessGateSetupCommand(t, repo, "git", "commit", "-qm", "fixture")

	return &witnessGateFixture{
		t:          t,
		repo:       repo,
		scratch:    scratch,
		helperBin:  helperBin,
		helperPath: helperPath,
		realTar:    realTar,
		countPath:  filepath.Join(root, "gate-count"),
		reportPath: filepath.Join(root, "observation.json"),
	}
}

func historicalWitnessGateSource(t *testing.T) []byte {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate witness gate test source")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "scripts", "agents", "witness-gate.sh"))
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read current witness gate source: %v", err)
	}
	corrected := `  if (( witness_gate_rc != 0 )); then
    echo "witness gate failed in its clean snapshot" >&2
    rm -rf "$witness_state" || true
    rm -rf "$witness_snap" || true
    witness_state=
    return "$witness_gate_rc" 2>/dev/null || exit "$witness_gate_rc"
  fi
`
	historical := `  if (( witness_gate_rc != 0 )); then
    echo "witness gate did not complete; falling back to the plain gate" >&2
    rm -rf "$witness_state" || true
    witness_state=
    if [[ "${WITNESS_GATE_FALLBACK:-plain}" == plain ]]; then
      bash scripts/agents/go-gate.sh
    fi
    rm -rf "$witness_snap" || true
    return 0 2>/dev/null || exit 0
  fi
`
	if bytes.Count(source, []byte(corrected)) != 1 {
		t.Fatal("current witness source no longer carries the executed-failure terminal block")
	}
	return bytes.Replace(source, []byte(corrected), []byte(historical), 1)
}

func (fixture *witnessGateFixture) run(options witnessGateRunOptions) witnessGateRunResult {
	fixture.t.Helper()
	command := exec.Command("bash", "-c", witnessGateInvocation)
	command.Dir = fixture.repo
	command.Env = witnessGateEnvironment(fixture, options)
	output, err := command.CombinedOutput()
	status := witnessGateProcessStatus(err)

	var observation witnessGateObservation
	report, reportErr := os.ReadFile(fixture.reportPath)
	if reportErr != nil {
		fixture.t.Fatalf("read witness gate observation after status %d: %v\noutput:\n%s", status, reportErr, output)
	}
	if err := json.Unmarshal(report, &observation); err != nil {
		fixture.t.Fatalf("decode witness gate observation: %v", err)
	}
	if observation.SourceStatus != status {
		fixture.t.Errorf("observer recorded source status %d but process exited %d", observation.SourceStatus, status)
	}

	launches := 0
	if count, err := os.ReadFile(fixture.countPath); err == nil {
		launches, err = strconv.Atoi(strings.TrimSpace(string(count)))
		if err != nil {
			fixture.t.Fatalf("parse gate launch count %q: %v", count, err)
		}
	} else if !os.IsNotExist(err) {
		fixture.t.Fatalf("read gate launch count: %v", err)
	}
	return witnessGateRunResult{
		status:      status,
		launches:    launches,
		output:      string(output),
		observation: observation,
	}
}

func witnessGateEnvironment(fixture *witnessGateFixture, options witnessGateRunOptions) []string {
	excluded := map[string]bool{
		"GO_WANT_WITNESS_GATE_HELPER":      true,
		"LC_ALL":                           true,
		"METASYSTEM_COVERAGE_RATCHET_SEED": true,
		"METASYSTEM_GATE_FORCE":            true,
		"PATH":                             true,
		"TMPDIR":                           true,
		"WITNESS_GATE_ARCHIVE_STATUS":      true,
		"WITNESS_GATE_COUNT_FILE":          true,
		"WITNESS_GATE_FALLBACK":            true,
		"WITNESS_GATE_HELPER":              true,
		"WITNESS_GATE_REPORT_FILE":         true,
		"WITNESS_GATE_STATUSES":            true,
		"WITNESS_GATE_TMP_ROOT":            true,
		"WITNESS_GATE_WRITE_BINARY":        true,
		"WITNESS_GATE_WRITE_WITNESS":       true,
		"WITNESS_REAL_TAR":                 true,
	}
	environment := make([]string, 0, len(os.Environ())+14)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if excluded[key] || strings.HasPrefix(key, "METASYSTEM_GATE_WITNESS") {
			continue
		}
		environment = append(environment, entry)
	}
	environment = append(environment,
		"GO_WANT_WITNESS_GATE_HELPER=1",
		"LC_ALL=C",
		"PATH="+fixture.helperBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TMPDIR="+fixture.scratch,
		"WITNESS_GATE_ARCHIVE_STATUS="+strconv.Itoa(options.archiveStatus),
		"WITNESS_GATE_COUNT_FILE="+fixture.countPath,
		"WITNESS_GATE_FALLBACK="+options.fallback,
		"WITNESS_GATE_HELPER="+fixture.helperPath,
		"WITNESS_GATE_REPORT_FILE="+fixture.reportPath,
		"WITNESS_GATE_STATUSES="+options.gateStatuses,
		"WITNESS_GATE_TMP_ROOT="+fixture.scratch,
		"WITNESS_REAL_TAR="+fixture.realTar,
	)
	if options.writeWitness {
		environment = append(environment, "WITNESS_GATE_WRITE_WITNESS=1")
	}
	if options.writeBinary {
		environment = append(environment, "WITNESS_GATE_WRITE_BINARY=1")
	}
	if options.seed {
		environment = append(environment, "METASYSTEM_COVERAGE_RATCHET_SEED=1")
	}
	if options.force {
		environment = append(environment, "METASYSTEM_GATE_FORCE=1")
	}
	return environment
}

func writeWitnessGateWrapper(t *testing.T, path, mode string) {
	t.Helper()
	content := "#!/usr/bin/env bash\nexec \"$WITNESS_GATE_HELPER\" -test.run=^TestWitnessGateHelperProcess$ -- " + mode + " \"$@\"\n"
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func runWitnessGateSetupCommand(t *testing.T, directory, name string, arguments ...string) {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
}

func (fixture *witnessGateFixture) assertScratchEntries(want ...string) {
	fixture.t.Helper()
	entries, err := os.ReadDir(fixture.scratch)
	if err != nil {
		fixture.t.Fatal(err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, filepath.Join(fixture.scratch, entry.Name()))
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		fixture.t.Errorf("temporary entries = %q, want %q", got, want)
	}
}

func assertNoPublishedWitness(t *testing.T, observation witnessGateObservation) {
	t.Helper()
	if observation.Witness != "" || observation.WitnessRoot != "" || observation.WitnessRun != "" || observation.WitnessExport != "" || observation.LocalState != "" {
		t.Errorf("unpublished witness state was retained: %+v", observation)
	}
	if observation.LocalSnapshot == "" {
		t.Error("clean snapshot path was not reported by the sourced script")
	} else if _, err := os.Stat(observation.LocalSnapshot); !os.IsNotExist(err) {
		t.Errorf("clean snapshot was not removed: %v", err)
	}
}

func witnessGateGoHelper(arguments []string) int {
	joined := strings.Join(arguments, " ")
	switch {
	case strings.Contains(joined, "behavior-surface select"):
		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 97
		}
		return 0
	case strings.Contains(joined, "proc started-at"):
		fmt.Println("1700000000 42 test-boot")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unexpected go helper arguments: %q\n", arguments)
		return 97
	}
}

func witnessGateCommandHelper() int {
	countPath := os.Getenv("WITNESS_GATE_COUNT_FILE")
	launch := 1
	if raw, err := os.ReadFile(countPath); err == nil {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
		if parseErr != nil {
			fmt.Fprintln(os.Stderr, parseErr)
			return 97
		}
		launch = parsed + 1
	} else if !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
		return 97
	}
	if err := os.WriteFile(countPath, []byte(strconv.Itoa(launch)+"\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 97
	}

	status := witnessGateSelectedStatus(os.Getenv("WITNESS_GATE_STATUSES"), launch)
	if status != 0 {
		return status
	}
	if os.Getenv("WITNESS_GATE_WRITE_WITNESS") == "1" {
		path := os.Getenv("METASYSTEM_GATE_WITNESS_WRITE")
		if path == "" {
			fmt.Fprintln(os.Stderr, "gate helper was asked to write a witness without a destination")
			return 97
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 97
		}
	}
	if os.Getenv("WITNESS_GATE_WRITE_BINARY") == "1" {
		if err := os.MkdirAll("bin", 0o700); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 97
		}
		if err := os.WriteFile(filepath.Join("bin", "metasystem"), []byte("proven test binary\n"), 0o700); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 97
		}
	}
	return 0
}

func witnessGateSelectedStatus(raw string, launch int) int {
	statuses := strings.Split(raw, ",")
	index := launch - 1
	if index >= len(statuses) {
		index = len(statuses) - 1
	}
	if index < 0 || strings.TrimSpace(statuses[index]) == "" {
		return 0
	}
	status, err := strconv.Atoi(strings.TrimSpace(statuses[index]))
	if err != nil || status < 0 || status > 255 {
		fmt.Fprintf(os.Stderr, "invalid gate helper status %q\n", statuses[index])
		return 97
	}
	return status
}

func witnessGateObservationHelper() int {
	status, err := strconv.Atoi(os.Getenv("WITNESS_GATE_SOURCE_STATUS"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 97
	}
	observation := witnessGateObservation{
		SourceStatus:  status,
		Witness:       os.Getenv("METASYSTEM_GATE_WITNESS"),
		WitnessRoot:   os.Getenv("METASYSTEM_GATE_WITNESS_ROOT"),
		WitnessRun:    os.Getenv("METASYSTEM_GATE_WITNESS_RUN"),
		WitnessExport: os.Getenv("METASYSTEM_GATE_WITNESS_EXPORT"),
		LocalState:    os.Getenv("WITNESS_GATE_LOCAL_STATE"),
		LocalSnapshot: os.Getenv("WITNESS_GATE_LOCAL_SNAPSHOT"),
	}
	encoded, err := json.Marshal(observation)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 97
	}
	if err := os.WriteFile(os.Getenv("WITNESS_GATE_REPORT_FILE"), encoded, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 97
	}
	return status
}

func witnessGateMktempHelper(arguments []string) int {
	root := os.Getenv("WITNESS_GATE_TMP_ROOT")
	var (
		path string
		err  error
	)
	if len(arguments) == 1 && arguments[0] == "-d" {
		path, err = os.MkdirTemp(root, "witness-")
	} else if len(arguments) == 0 {
		var file *os.File
		file, err = os.CreateTemp(root, "witness-")
		if err == nil {
			path = file.Name()
			err = file.Close()
		}
	} else {
		fmt.Fprintf(os.Stderr, "unexpected mktemp helper arguments: %q\n", arguments)
		return 97
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 97
	}
	fmt.Println(path)
	return 0
}

func witnessGateTarHelper(arguments []string) int {
	command := exec.Command(os.Getenv("WITNESS_REAL_TAR"), arguments...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return witnessGateProcessStatus(err)
	}
	status, err := strconv.Atoi(os.Getenv("WITNESS_GATE_ARCHIVE_STATUS"))
	if err != nil || status < 0 || status > 255 {
		fmt.Fprintln(os.Stderr, "invalid archive helper status")
		return 97
	}
	return status
}

func witnessGateProcessStatus(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return 97
}
