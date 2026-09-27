package proofrun

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestValidationRunSectionDoesNotInheritParentExitCleanup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	functions := filepath.Join(root, "functions.sh")
	engine := filepath.Join(root, "engine")
	wrapper := filepath.Join(root, "wrapper.sh")
	validationSource := filepath.Join(packageRoot(t), "scripts", "validate-metasystem.sh")
	budgetSource := filepath.Join(packageRoot(t), "scripts", "agents", "fixture-budget.sh")

	engineBody := `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == util && "$2" == now-ns ]]; then
  if [[ -e "$NOW_MARKER" ]]; then printf '1251000000\n'; else : >"$NOW_MARKER"; printf '1000000000\n'; fi
elif [[ "$1" == proc && "$2" == census ]]; then
  output=
  while (( $# )); do
    if [[ "$1" == --output ]]; then output=$2; break; fi
    shift
  done
  [[ -n "$output" ]]
  printf '{}\n' >"$output"
  printf '%s\n' "$output" >"$CALIBRATION_PATH"
else
  exit 97
fi
`
	if err := testexec.WriteFile(engine, []byte(engineBody), 0o700); err != nil {
		t.Fatal(err)
	}

	wrapperBody := `#!/usr/bin/env bash
set -eo pipefail
sed -n -e '/^stage_tail_for_record()/,/^}$/p' -e '/^record_stage_result()/,/^}$/p' \
  -e '/^section_dependency_ready()/,/^}$/p' -e '/^section_dependency_reason()/,/^}$/p' \
  -e '/^run_section()/,/^}$/p' "$VALIDATION_SOURCE" >"$FUNCTIONS"
sed -n -e '/^harness_fixture_warn_if_engine_stale()/,/^}$/p' \
  -e '/^harness_fixture_base_cap()/,/^}$/p' \
  -e '/^harness_fixture_milliseconds_to_seconds()/,/^}$/p' \
  -e '/^harness_fixture_budget_init()/,/^}$/p' "$BUDGET_SOURCE" >>"$FUNCTIONS"
source "$FUNCTIONS"
unset METASYSTEM_FIXTURE_CAP_SCALE METASYSTEM_FIXTURE_CAP_SCALE_MILLI
root=$TEST_ROOT
stage_work=$TEST_ROOT/stage
stage_results_file=$TEST_ROOT/results.tsv
fixture_budget_state_file=$stage_work/fixture-budget-state.sh
engine_dependency=ready
fixture_budget_dependency=uninitialized
validation_red_sections=()
validation_red_rcs=()
mkdir -p "$stage_work"
: >"$GUARD_SENTINEL"
suite_progress_finish() { printf 'finished\n' >>"$PROGRESS_FINISH"; }
checkout_execution_guard_release() { printf 'released\n' >>"$GUARD_RELEASE"; rm -f "$GUARD_SENTINEL"; }
parent_cleanup() {
  printf 'cleanup\n' >>"$PARENT_CLEANUP"
  suite_progress_finish
  rm -rf "$stage_work"
  checkout_execution_guard_release
}
arm_child_cleanup() {
  if (( BASH_SUBSHELL > 0 )); then trap parent_cleanup EXIT; trap - DEBUG; fi
}
trap arm_child_cleanup DEBUG
set -T
trap parent_cleanup EXIT
fixture_budget_section() {
  harness_fixture_budget_init "$root"
  printf 'ready\n' >"$fixture_budget_state_file"
}
run_section fixture-budget-initialization needs-engine fixture_budget_section
[[ "$last_section_status" == pass ]]
[[ -f "$fixture_budget_state_file" && -d "$stage_work" ]]
[[ -e "$GUARD_SENTINEL" && ! -e "$GUARD_RELEASE" ]]
[[ ! -e "$PROGRESS_FINISH" && ! -e "$PARENT_CLEANUP" ]]
: >"$BEFORE_EXIT_OK"
`
	if err := testexec.WriteFile(wrapper, []byte(wrapperBody), 0o700); err != nil {
		t.Fatal(err)
	}

	paths := map[string]string{
		"CALIBRATION_PATH": filepath.Join(root, "calibration-path"), "NOW_MARKER": filepath.Join(root, "now-marker"),
		"GUARD_SENTINEL": filepath.Join(root, "guard-sentinel"), "GUARD_RELEASE": filepath.Join(root, "guard-release"),
		"PROGRESS_FINISH": filepath.Join(root, "progress-finish"), "PARENT_CLEANUP": filepath.Join(root, "parent-cleanup"),
		"BEFORE_EXIT_OK": filepath.Join(root, "before-exit-ok"),
	}
	command := exec.Command("bash", wrapper)
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "TMPDIR=" + root, "TEST_ROOT=" + root,
		"FUNCTIONS=" + functions, "VALIDATION_SOURCE=" + validationSource, "BUDGET_SOURCE=" + budgetSource,
		"METASYSTEM_BIN=" + engine}
	for name, path := range paths {
		command.Env = append(command.Env, name+"="+path)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("section wrapper failed: %v\n%s", err, output)
	}
	for name, want := range map[string]string{"PARENT_CLEANUP": "cleanup\n", "PROGRESS_FINISH": "finished\n", "GUARD_RELEASE": "released\n"} {
		got, readErr := os.ReadFile(paths[name])
		if readErr != nil || string(got) != want {
			t.Fatalf("%s after wrapper exit = %q, want exactly %q; err=%v", name, got, want, readErr)
		}
	}
	for _, path := range []string{filepath.Join(root, "stage"), paths["GUARD_SENTINEL"]} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("parent cleanup left %s: %v", path, statErr)
		}
	}
	calibrationPath, err := os.ReadFile(paths["CALIBRATION_PATH"])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(strings.TrimSpace(string(calibrationPath))); !os.IsNotExist(err) {
		t.Fatalf("calibration probe cleanup did not remove its directory: %v", err)
	}
}

func TestFullScriptWrappersPreserveOwnerStatusAndIgnoreAmbientProgress(t *testing.T) {
	for _, test := range []struct {
		name, relative string
		status         int
		fixtureBudget  bool
	}{
		{name: "validator reusable success", relative: "scripts/validate-metasystem.sh", status: ExitReusableSuccess},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
				t.Fatal(err)
			}
			source := filepath.Clean(filepath.Join(packageRoot(t), test.relative))
			copyScriptFile(t, source, filepath.Join(root, test.relative))
			if test.relative == "scripts/validate-metasystem.sh" {
				// The validator resolves its run context through the selector before it relaunches.
				copyScriptFile(t, filepath.Join(packageRoot(t), "scripts", "agents", "validate-section-selector.sh"), filepath.Join(root, "scripts", "agents", "validate-section-selector.sh"))
			}
			if test.fixtureBudget {
				copyScriptFile(t, filepath.Join(packageRoot(t), "scripts", "agents", "fixture-budget.sh"), filepath.Join(root, "scripts", "agents", "fixture-budget.sh"))
			}
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/widoriezebos/agentic-tools/metasystem\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			helperDir := filepath.Join(root, "helpers")
			if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(helperDir, 0o755); err != nil {
				t.Fatal(err)
			}
			engineWrapper := filepath.Join(root, "bin", "metasystem")
			writeEntrypointHelperWrapper(t, engineWrapper, "engine")
			writeEntrypointHelperWrapper(t, filepath.Join(helperDir, "go"), "go")
			count := filepath.Join(root, "launch-count")
			admittedFlags := filepath.Join(root, "admitted-go-flags")
			command := exec.Command("bash", filepath.Join(root, test.relative))
			command.Dir = root
			command.Env = append(filteredCoverageScriptEnvironment(),
				"GO_WANT_PROOF_ENTRYPOINT_HELPER=1", "PROOF_ENTRYPOINT_HELPER="+coverageScriptExecutable(t),
				"PROOF_ENTRYPOINT_STATUS="+strconv.Itoa(test.status), "PROOF_ENTRYPOINT_LAUNCH_COUNT="+count,
				"PROOF_ENTRYPOINT_GOFLAGS_OUT="+admittedFlags, "GOFLAGS=-mod=mod",
				"METASYSTEM_SUITE_PROGRESS_ACTIVE=1", "METASYSTEM_SUITE_PROGRESS_ROOT="+root,
				"PATH="+helperDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			output, err := command.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != test.status {
				t.Fatalf("wrapper exit=%v want=%d output:\n%s", err, test.status, output)
			}
			raw, err := os.ReadFile(count)
			if err != nil || strings.TrimSpace(string(raw)) != "1" {
				t.Fatalf("ambient progress bypassed or recursively relaunched wrapper: launches=%q err=%v output:\n%s", raw, err, output)
			}
			if !test.fixtureBudget {
				flags, readErr := os.ReadFile(admittedFlags)
				if readErr != nil || string(flags) != "-mod=readonly" {
					t.Fatalf("proof admitted Go flags %q, want the full gate's -mod=readonly; err=%v", flags, readErr)
				}
			}
		})
	}
}

func writeDependencyGateFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

type recordedGoInvocation struct {
	gomaxprocs string
	argv       []string
}

type recordingGoFixture struct {
	root      string
	goLog     string
	nativeLog string
}

func TestGoBuildStubRunsTheBootstrapBuildOfItsOwnInstallation(t *testing.T) {
	t.Parallel()

	// Worker validation, the fence, the stamp and the -p/GOMAXPROCS allowance
	// are cmd/devgate's, proved in its package; the stub's whole contract is
	// to run that build for its own installation from any directory.
	fixture := newRecordingGoFixture(t)
	root, err := filepath.EvalSymlinks(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--trimpath", "--out", filepath.Join(root, "proof-engine")}, nil} {
		command := exec.Command("bash", append([]string{filepath.Join(fixture.root, "scripts", "agents", "go-build.sh")}, args...)...)
		command.Dir = t.TempDir()
		command.Env = append(recordingGoEnvironment(),
			"PATH="+filepath.Join(fixture.root, "helpers")+string(os.PathListSeparator)+os.Getenv("PATH"),
			"RECORDING_GO_LOG="+fixture.goLog, "RECORDING_NATIVE_LOG="+fixture.nativeLog,
			"RECORDING_ENGINE_STUB="+filepath.Join(fixture.root, "helpers", "engine-stub"))
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("go-build %v from an unrelated directory failed: %v\n%s", args, err, output)
		}
	}
	records := readRecordedGoInvocations(t, fixture.goLog)
	want := [][]string{
		{"-C", root, "run", "./cmd/devgate", "build", "--trimpath", "--out", filepath.Join(root, "proof-engine")},
		{"-C", root, "run", "./cmd/devgate", "build"},
	}
	if len(records) != len(want) {
		t.Fatalf("go invocations = %+v, want %q", records, want)
	}
	for index, record := range records {
		if !reflect.DeepEqual(record.argv, want[index]) {
			t.Fatalf("go invocation %d = %q, want %q", index, record.argv, want[index])
		}
	}
	for _, produced := range []string{filepath.Join(root, "proof-engine"), filepath.Join(root, "bin", "metasystem")} {
		if _, err := os.Stat(produced); err != nil {
			t.Fatalf("stub build did not produce %s: %v", produced, err)
		}
	}
}

func newRecordingGoFixture(t *testing.T) recordingGoFixture {
	t.Helper()
	root := t.TempDir()
	for _, relative := range []string{"scripts/agents/go-build.sh"} {
		copyScriptFile(t, filepath.Join(packageRoot(t), relative), filepath.Join(root, relative))
	}
	for _, directory := range []string{"cmd", "internal", "helpers", "bin"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeDependencyGateFile(t, filepath.Join(root, "go.mod"), "module github.com/widoriezebos/agentic-tools/metasystem\n", 0o600)
	writeDependencyGateFile(t, filepath.Join(root, "scripts", "agents", "coverage-ratchet.json"), "{}\n", 0o600)
	writeDependencyGateFile(t, filepath.Join(root, "scripts", "agents", "coverage-ratchet-linux.json"), "{}\n", 0o600)
	writeDependencyGateFile(t, filepath.Join(root, "helpers", "gofmt"), "#!/usr/bin/env bash\nexit 0\n", 0o700)
	writeDependencyGateFile(t, filepath.Join(root, "helpers", "shasum"), "#!/usr/bin/env bash\nprintf '1111111111111111111111111111111111111111111111111111111111111111  -\\n'\n", 0o700)
	writeDependencyGateFile(t, filepath.Join(root, "helpers", "git"), `#!/usr/bin/env bash
set -euo pipefail
case " $* " in
  *" rev-parse HEAD "*) printf '%040d\n' 1 ;;
  *" rev-parse --show-prefix "*) printf '\n' ;;
  *" diff --name-only "*) printf 'internal/dirty.go\0' ;;
  *" ls-files "*) ;;
  *" rev-parse --short HEAD "*) printf 'fixture\n' ;;
  *) exit 0 ;;
esac
`, 0o700)
	engineStub := filepath.Join(root, "helpers", "engine-stub")
	writeDependencyGateFile(t, engineStub, `#!/usr/bin/env bash
set -euo pipefail
joined=" $* "
case "$joined" in
  *" proof-run banner "*) echo 'recording gate banner' ;;
  *" proof-run launch "*)
    while [[ "$1" != -- ]]; do shift; done
    shift
    METASYSTEM_TEST_WORKERS="${METASYSTEM_TEST_WORKERS:-6}" exec "$@"
    ;;
  *" proof-run worker-authorized "*) exit 0 ;;
  *" proc ref --pid "*) echo 'pid=1;micro=1' ;;
  *" proof-run go-gate-tests "*)
    printf '%s\0%s\0' "${GOMAXPROCS:-}" "${METASYSTEM_TEST_WORKERS:-}" >>"$RECORDING_NATIVE_LOG"
    printf '%s\0' "$@" >>"$RECORDING_NATIVE_LOG"
    native_root=
    while (($#)); do
      [[ "$1" != --log-root ]] || { native_root=$2; break; }
      shift
    done
    mkdir -p "$native_root"
    printf 'native census and diagnostic\n' >"$native_root/go-gate-native.log"
    printf 'counter evidence\n' >"$native_root/counters"
	if [[ "${RECORDING_NATIVE_COVERAGE_STDERR:-0}" == 1 ]]; then
	  printf 'ok  \texample.invalid/internal/stderr-only\t0.000s\tcoverage: 100.0%% of statements\n' >&2
	fi
    printf '{"Action":"output","Package":"example.invalid/internal/proofrun","Output":"ok  \\texample.invalid/internal/proofrun\\t0.000s\\tcoverage: 100.0%% of statements\\n"}\n'
    ;;
  *" audit coverage-ratchet "*)
    [[ "${RECORDING_COVERAGE_CONSUMER_FAIL:-0}" != 1 ]]
    ;;
  *" proof-run coverage-complete "*)
    [[ "${RECORDING_COVERAGE_PUBLICATION_FAIL:-0}" != 1 ]]
    ;;
  *) exit 0 ;;
esac
`, 0o700)
	writeDependencyGateFile(t, filepath.Join(root, "helpers", "cat"), `#!/usr/bin/env bash
set -euo pipefail
if [[ "${RECORDING_NATIVE_CAT_FAIL:-0}" == 1 && "$#" == 1 && "$1" == */go-gate-native.log ]]; then
  exit 41
fi
exec /bin/cat "$@"
`, 0o700)
	writeDependencyGateFile(t, filepath.Join(root, "helpers", "go"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\0%s\0' "${GOMAXPROCS:-}" "$#" >>"$RECORDING_GO_LOG"
printf '%s\0' "$@" >>"$RECORDING_GO_LOG"
if [[ "${1:-}" == -C ]]; then
  # The go-build.sh stub's bootstrap build (cmd/devgate, unit-tested in its
  # own package): produce the engine where it would.
  [[ "${3:-} ${4:-} ${5:-}" == "run ./cmd/devgate build" ]] || exit 97
  cd "$2"
  shift 5
  out=
  while (($#)); do
    [[ "$1" != --out ]] || { out=$2; break; }
    shift
  done
  if [[ -z "$out" ]]; then
    if [[ "${RECORDING_POST_NATIVE_BUILD_FAIL:-0}" == 1 && -s "$RECORDING_NATIVE_LOG" ]]; then
      exit 1
    fi
    mkdir -p bin
    out=bin/metasystem
  fi
  cp "$RECORDING_ENGINE_STUB" "$out"
  chmod 700 "$out"
  exit 0
fi
case "${1:-}" in
  version) echo 'go version recording-fixture' ;;
  env) echo local ;;
  list)
    if [[ " $* " == *" ./internal/... "* && "${RECORDING_INVENTORY_FAIL:-0}" == 1 ]]; then
      exit 1
    fi
    if [[ " $* " == *" -f "* ]]; then
      echo "$PWD/internal/proofrun"
    else
      echo 'example.invalid/internal/proofrun'
    fi
    ;;
  run)
    case " $* " in
      *" behavior-surface select "*) cat ;;
      *" behavior-surface digest "*) printf '{"surfaceDigest":"0000000000000000000000000000000000000000000000000000000000000000","policyVersion":1}\n' ;;
      *" gate witness-freeze --root "*)
        export_root=${RECORDING_FREEZE_EXPORT:-$PWD}
        mkdir -p "$RECORDING_FREEZE_SNAPSHOT"
        if [[ "$export_root" != "$PWD" ]]; then
          mkdir -p "$export_root"
          cp -R "$PWD/." "$export_root/"
        fi
        printf '2222222222222222222222222222222222222222222222222222222222222222\t%s\t%s\n' "$export_root" "$RECORDING_FREEZE_SNAPSHOT"
        ;;
      *" gate witness-freeze --cleanup "*)
        rm -rf "$RECORDING_FREEZE_SNAPSHOT" "${RECORDING_FREEZE_EXPORT:-}"
        ;;
    esac
    ;;
  build)
    out=
    while (($#)); do
      [[ "$1" != -o ]] || { out=$2; break; }
      shift
    done
    if [[ -n "$out" ]]; then
      if [[ "$out" == bin/.metasystem.build.* && "${RECORDING_POST_NATIVE_BUILD_FAIL:-0}" == 1 && -s "$RECORDING_NATIVE_LOG" ]]; then
        exit 1
      fi
      cp "$RECORDING_ENGINE_STUB" "$out"
      chmod 700 "$out"
    fi
    ;;
  vet|test) ;;
  *) exit 97 ;;
esac
`, 0o700)
	return recordingGoFixture{root: root, goLog: filepath.Join(root, "go-records"), nativeLog: filepath.Join(root, "native-record")}
}

func recordingGoEnvironment() []string {
	var environment []string
	for _, entry := range filteredCoverageScriptEnvironment() {
		key, _, _ := strings.Cut(entry, "=")
		if key == "GOMAXPROCS" || key == "METASYSTEM_TEST_WORKERS" || key == "METASYSTEM_COVERAGE_RATCHET_SEED" || key == "METASYSTEM_BUILD_STAMP" || strings.HasPrefix(key, "RECORDING_") {
			continue
		}
		environment = append(environment, entry)
	}
	return environment
}

func readRecordedGoInvocations(t *testing.T, path string) []recordedGoInvocation {
	t.Helper()
	fields := readNULFields(t, path)
	var records []recordedGoInvocation
	for len(fields) > 0 {
		if len(fields) < 2 {
			t.Fatalf("truncated Go record: %q", fields)
		}
		argumentCount, err := strconv.Atoi(fields[1])
		if err != nil || argumentCount < 1 || len(fields) < argumentCount+2 {
			t.Fatalf("invalid Go record: %q", fields)
		}
		records = append(records, recordedGoInvocation{gomaxprocs: fields[0], argv: append([]string(nil), fields[2:argumentCount+2]...)})
		fields = fields[argumentCount+2:]
	}
	return records
}

func readNULFields(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Split(string(data), "\x00")
	return fields[:len(fields)-1]
}

func TestProofScriptEntrypointHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PROOF_ENTRYPOINT_HELPER") != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+2 > len(os.Args) {
		os.Exit(97)
	}
	mode, arguments := os.Args[separator+1], os.Args[separator+2:]
	status, _ := strconv.Atoi(os.Getenv("PROOF_ENTRYPOINT_STATUS"))
	if mode == "go" {
		if len(arguments) > 0 && arguments[0] == "build" {
			output := ""
			for index := range arguments {
				if arguments[index] == "-o" && index+1 < len(arguments) {
					output = arguments[index+1]
				}
			}
			if output == "" {
				os.Exit(97)
			}
			writeEntrypointWrapperNow(output, "engine")
			os.Exit(0)
		}
		os.Exit(status)
	}
	joined := strings.Join(arguments, " ")
	switch {
	case strings.HasPrefix(joined, "proof-run worker-authorized "):
		status, found := os.LookupEnv("PROOF_ENTRYPOINT_WORKER_STATUS")
		if !found {
			os.Exit(3)
		}
		parsed, _ := strconv.Atoi(status)
		os.Exit(parsed)
	case strings.HasPrefix(joined, "proof-run coverage-eligible "):
		status, _ := strconv.Atoi(os.Getenv("PROOF_ENTRYPOINT_ELIGIBILITY_STATUS"))
		os.Exit(status)
	case strings.HasPrefix(joined, "proof-run coverage-begin "):
		path := os.Getenv("PROOF_ENTRYPOINT_COVERAGE_BEGIN_COUNT")
		begins := 1
		if raw, err := os.ReadFile(path); err == nil {
			begins, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			begins++
		}
		if err := os.WriteFile(path, []byte(strconv.Itoa(begins)+"\n"), 0o600); err != nil {
			os.Exit(97)
		}
		os.Exit(0)
	case strings.HasPrefix(joined, "proof-run banner "):
		fmt.Println("entrypoint fixture banner")
		os.Exit(0)
	case strings.HasPrefix(joined, "proof-run launch "):
		if path := os.Getenv("PROOF_ENTRYPOINT_GOFLAGS_OUT"); path != "" {
			if err := os.WriteFile(path, []byte(os.Getenv("GOFLAGS")), 0o600); err != nil {
				os.Exit(97)
			}
		}
		path := os.Getenv("PROOF_ENTRYPOINT_LAUNCH_COUNT")
		launches := 1
		if raw, err := os.ReadFile(path); err == nil {
			launches, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			launches++
		}
		_ = os.WriteFile(path, []byte(strconv.Itoa(launches)+"\n"), 0o600)
		os.Exit(status)
	default:
		os.Exit(97)
	}
}

func coverageScriptExecutable(t *testing.T) string {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func packageRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate proofrun package")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func copyScriptFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(destination, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeEntrypointHelperWrapper(t *testing.T, path, mode string) {
	t.Helper()
	if err := writeEntrypointWrapperNow(path, mode); err != nil {
		t.Fatal(err)
	}
}

func writeEntrypointWrapperNow(path, mode string) error {
	wrapper := "#!/usr/bin/env bash\nexec \"$PROOF_ENTRYPOINT_HELPER\" -test.run=^TestProofScriptEntrypointHelper$ -- " + mode + " \"$@\"\n"
	return testexec.WriteFile(path, []byte(wrapper), 0o755)
}

func filteredCoverageScriptEnvironment() []string {
	var result []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		// The full gate arms a witness for its nested validations and then runs
		// this package's tests; a focused fast gate spawned here must not inherit
		// that handoff, which fast mode refuses by law.
		if strings.HasPrefix(key, "METASYSTEM_PROOF_") || strings.HasPrefix(key, "METASYSTEM_SUITE_PROGRESS_") || strings.HasPrefix(key, "METASYSTEM_GATE_WITNESS") {
			continue
		}
		switch key {
		case "GO_WANT_COVERAGE_SCRIPT_HELPER", "COVERAGE_SCRIPT_HELPER", "COVERAGE_SCRIPT_REUSE_STATUS", "COVERAGE_SCRIPT_LAUNCH_COUNT", "FAST_GATE_BUILD_COUNT", "FAST_GATE_RUN_OWNER_ANSWER", "FAST_GATE_RUN_OWNER_SEEN", "METASYSTEM_BIN", "METASYSTEM_PROOFRUN_TEST_CANDIDATE_ENGINE", "METASYSTEM_RUN_OWNER", "METASYSTEM_GO_GATE_RELAUNCHED", "METASYSTEM_VALIDATE_RELAUNCHED", "METASYSTEM_ADOPT_FIXTURES_RELAUNCHED", "METASYSTEM_COVERAGE_DELTA_RELAUNCHED", "PATH":
			continue
		}
		result = append(result, entry)
	}
	return result
}
