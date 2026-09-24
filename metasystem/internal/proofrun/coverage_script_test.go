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

func TestCoverageDeltaProductionConsumer(t *testing.T) {
	for _, test := range []struct {
		name           string
		reuseStatus    int
		wantLaunches   int
		wantDiagnostic string
	}{
		{name: "matching retained evidence", reuseStatus: 0, wantLaunches: 0, wantDiagnostic: "no coverage test launched"},
		{name: "mismatch runs existing coverage", reuseStatus: 3, wantLaunches: 1, wantDiagnostic: "passed (1 package(s) considered)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := coverageDeltaScriptFixture(t)
			count := filepath.Join(root, "test-launch-count")
			engine := filepath.Join(root, "helpers", "metasystem")
			command := exec.Command("bash", filepath.Join(root, "scripts", "agents", "coverage-delta.sh"), "internal/proofrun")
			command.Dir = root
			command.Env = append(filteredCoverageScriptEnvironment(),
				"GO_WANT_COVERAGE_SCRIPT_HELPER=1",
				"COVERAGE_SCRIPT_HELPER="+coverageScriptExecutable(t),
				"COVERAGE_SCRIPT_REUSE_STATUS="+strconv.Itoa(test.reuseStatus),
				"COVERAGE_SCRIPT_LAUNCH_COUNT="+count,
				"METASYSTEM_BIN="+engine,
				"METASYSTEM_PROOFRUN_TEST_CANDIDATE_ENGINE=1",
				"PATH="+filepath.Join(root, "helpers")+string(os.PathListSeparator)+os.Getenv("PATH"),
			)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("coverage consumer failed: %v\n%s", err, output)
			}
			launches := 0
			if raw, readErr := os.ReadFile(count); readErr == nil {
				launches, readErr = strconv.Atoi(strings.TrimSpace(string(raw)))
				if readErr != nil {
					t.Fatal(readErr)
				}
			} else if !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			if launches != test.wantLaunches || !strings.Contains(string(output), test.wantDiagnostic) {
				t.Fatalf("coverage launches=%d want=%d output:\n%s", launches, test.wantLaunches, output)
			}
		})
	}
}

func TestCoverageDeltaProductionConsumerHelper(t *testing.T) {
	if os.Getenv("GO_WANT_COVERAGE_SCRIPT_HELPER") != "1" {
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
	arguments := os.Args[separator+1:]
	switch arguments[0] {
	case "engine":
		os.Exit(coverageScriptEngineHelper(arguments[1:]))
	case "go":
		os.Exit(coverageScriptGoHelper(arguments[1:]))
	default:
		os.Exit(97)
	}
}

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
		{name: "go gate child failure", relative: "scripts/agents/go-gate.sh", status: 23},
		{name: "validator reusable success", relative: "scripts/validate-metasystem.sh", status: ExitReusableSuccess},
		{name: "adoption reusable success", relative: "scripts/adopt-fixtures.sh", status: ExitReusableSuccess, fixtureBudget: true},
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

func TestFastGoGatePublishesCollectedBuildWithoutRecompiling(t *testing.T) {
	root := t.TempDir()
	copyScriptFile(t, filepath.Join(packageRoot(t), "scripts", "agents", "go-gate.sh"), filepath.Join(root, "scripts", "agents", "go-gate.sh"))
	copyScriptFile(t, filepath.Join(packageRoot(t), "scripts", "agents", "go-build.sh"), filepath.Join(root, "scripts", "agents", "go-build.sh"))
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/widoriezebos/agentic-tools/metasystem\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"internal", "cmd", "helpers"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	count := filepath.Join(root, "build-count")
	runOwnerAnswer := filepath.Join(root, "run-owner-answer")
	runOwnerSeen := filepath.Join(root, "run-owner-seen")
	refAssignment := `ref="pid=$4;micro=100000123"`
	if runtime.GOOS == "linux" {
		refAssignment = `ref="pid=$4;ticks=7001;boot=fixture-boot"`
	}
	collectedEngine := `#!/usr/bin/env bash
set -euo pipefail
if [[ "$#" == 4 && "$1" == proc && "$2" == ref && "$3" == --pid && "$4" =~ ^[1-9][0-9]*$ ]]; then
  ` + refAssignment + `
  printf '%s\n' "$ref" >"$FAST_GATE_RUN_OWNER_ANSWER"
  printf '%s\n' "$ref"
  exit 0
fi
printf 'unexpected collected engine invocation:' >&2
printf ' %q' "$@" >&2
printf '\n' >&2
exit 97
`
	goHelper := `#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  version) echo 'go version fixture' ;;
  env) echo local ;;
  vet) ;;
  test) printf '%s\n' "${METASYSTEM_RUN_OWNER:-}" >"$FAST_GATE_RUN_OWNER_SEEN" ;;
  run) ;;
  build)
    count=0
    [[ ! -f "$FAST_GATE_BUILD_COUNT" ]] || count=$(cat "$FAST_GATE_BUILD_COUNT")
    printf '%d\n' "$((count + 1))" >"$FAST_GATE_BUILD_COUNT"
    out=
    while (($#)); do
      [[ "$1" != -o ]] || { out=$2; break; }
      shift
    done
    [[ -n "$out" ]]
    cat >"$out" <<'FAST_GATE_COLLECTED_ENGINE'
` + collectedEngine + `FAST_GATE_COLLECTED_ENGINE
    chmod 700 "$out"
    ;;
  *) exit 97 ;;
esac
`
	if err := testexec.WriteFile(filepath.Join(root, "helpers", "go"), []byte(goHelper), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "helpers", "gofmt"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	gateEnvironment := func() []string {
		return append(filteredCoverageScriptEnvironment(), "FAST_GATE_BUILD_COUNT="+count,
			"FAST_GATE_RUN_OWNER_ANSWER="+runOwnerAnswer, "FAST_GATE_RUN_OWNER_SEEN="+runOwnerSeen,
			"METASYSTEM_BUILD_STAMP=fixture", "PATH="+filepath.Join(root, "helpers")+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	output := filepath.Join(root, "proof-engine")
	command := exec.Command("bash", filepath.Join(root, "scripts", "agents", "go-gate.sh"), "--fast", "--proof-out", output)
	command.Dir = root
	command.Env = gateEnvironment()
	diagnostic, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("focused fast gate failed: %v\n%s", err, diagnostic)
	}
	if count := strings.Count(string(diagnostic), "SessionStart exit audit not applicable to this script fixture"); count != 1 {
		t.Fatalf("partial gate fixture reported the SessionStart audit scope %d times, want 1:\n%s", count, diagnostic)
	}
	builds, countErr := os.ReadFile(count)
	engine, engineErr := os.ReadFile(output)
	if countErr != nil || strings.TrimSpace(string(builds)) != "1" || engineErr != nil || string(engine) != collectedEngine {
		t.Fatalf("fast gate builds=%q countErr=%v engine=%q engineErr=%v", builds, countErr, engine, engineErr)
	}
	answeredOwner, answerErr := os.ReadFile(runOwnerAnswer)
	seenOwner, seenErr := os.ReadFile(runOwnerSeen)
	if answerErr != nil || seenErr != nil || strings.Count(string(answeredOwner), "\n") != 1 || strings.TrimSpace(string(answeredOwner)) == "" || string(seenOwner) != string(answeredOwner) {
		t.Fatalf("go test run owner=%q err=%v, want collected stub answer %q err=%v", seenOwner, seenErr, answeredOwner, answerErr)
	}
	unexpectedOutput, unexpectedErr := exec.Command(output, "unexpected").CombinedOutput()
	if unexpectedErr == nil || !strings.Contains(string(unexpectedOutput), "unexpected collected engine invocation: unexpected") {
		t.Fatalf("collected stub accepted unexpected arguments: err=%v output=%q", unexpectedErr, unexpectedOutput)
	}

	// Any wow.md filesystem entry makes this an installation. A dangling
	// marker must therefore attempt the candidate audit and fail rather than
	// reclassifying corruption as a script-only fixture.
	if err := os.Symlink("missing-wow-target", filepath.Join(root, "wow.md")); err != nil {
		t.Fatal(err)
	}
	governedOutput := filepath.Join(root, "governed-proof-engine")
	governed := exec.Command("bash", filepath.Join(root, "scripts", "agents", "go-gate.sh"), "--fast", "--proof-out", governedOutput)
	governed.Dir = root
	governed.Env = gateEnvironment()
	governedDiagnostic, governedErr := governed.CombinedOutput()
	if governedErr == nil || !strings.Contains(string(governedDiagnostic), "SessionStart exit audit failed") || strings.Contains(string(governedDiagnostic), "run owner export failed") {
		t.Fatalf("dangling installation marker did not fail through the candidate SessionStart audit: %v\n%s", governedErr, governedDiagnostic)
	}

	if err := os.Remove(filepath.Join(root, "wow.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "audit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing-hookstartexits-target", filepath.Join(root, "internal", "audit", "hookstartexits.go")); err != nil {
		t.Fatal(err)
	}
	auditSignalOutput := filepath.Join(root, "audit-signal-proof-engine")
	auditSignal := exec.Command("bash", filepath.Join(root, "scripts", "agents", "go-gate.sh"), "--fast", "--proof-out", auditSignalOutput)
	auditSignal.Dir = root
	auditSignal.Env = gateEnvironment()
	auditSignalDiagnostic, auditSignalErr := auditSignal.CombinedOutput()
	if auditSignalErr == nil || !strings.Contains(string(auditSignalDiagnostic), "SessionStart exit audit failed") || strings.Contains(string(auditSignalDiagnostic), "run owner export failed") {
		t.Fatalf("dangling audit-source signal did not fail through the candidate SessionStart audit: %v\n%s", auditSignalErr, auditSignalDiagnostic)
	}
}

func TestFastGoGateRunsDependencyRatchetBeforeTests(t *testing.T) {
	root := t.TempDir()
	copyScriptFile(t, filepath.Join(packageRoot(t), "scripts", "agents", "go-gate.sh"), filepath.Join(root, "scripts", "agents", "go-gate.sh"))
	copyScriptFile(t, filepath.Join(packageRoot(t), "scripts", "agents", "go-build.sh"), filepath.Join(root, "scripts", "agents", "go-build.sh"))
	writeDependencyGateFile(t, filepath.Join(root, "go.mod"), "module github.com/widoriezebos/agentic-tools/metasystem\n", 0o600)
	writeDependencyGateFile(t, filepath.Join(root, "scripts", "banned.sh"), "#!/usr/bin/env bash\nnode x.js\n", 0o700)
	for _, directory := range []string{"internal", "cmd", "helpers"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	trace := filepath.Join(root, "trace")
	goHelper := `#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  version) echo 'go version fixture' ;;
  env) echo local ;;
  vet) printf 'vet\n' >>"$FAST_GATE_TRACE" ;;
  test) printf 'test\n' >>"$FAST_GATE_TRACE" ;;
  run)
    if [[ " $* " == *" ./cmd/metasystem audit dependency-ratchet "* ]]; then
      printf 'ratchet\n' >>"$FAST_GATE_TRACE"
      echo 'dependency ratchet: banned interpreter node: scripts/banned.sh:2' >&2
      exit 19
    fi
    printf 'staticcheck\n' >>"$FAST_GATE_TRACE"
    ;;
  build) exit 97 ;;
  *) exit 97 ;;
esac
`
	writeDependencyGateFile(t, filepath.Join(root, "helpers", "go"), goHelper, 0o700)
	writeDependencyGateFile(t, filepath.Join(root, "helpers", "gofmt"), "#!/usr/bin/env bash\nexit 0\n", 0o700)
	command := exec.Command("bash", filepath.Join(root, "scripts", "agents", "go-gate.sh"), "--fast")
	command.Dir = root
	command.Env = append(filteredCoverageScriptEnvironment(), "FAST_GATE_TRACE="+trace,
		"PATH="+filepath.Join(root, "helpers")+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := command.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 19 {
		t.Fatalf("fast gate exit=%v, want dependency ratchet status 19; output:\n%s", err, output)
	}
	events, readErr := os.ReadFile(trace)
	if readErr != nil || string(events) != "ratchet\n" {
		t.Fatalf("gate events=%q err=%v, want ratchet before any test; output:\n%s", events, readErr, output)
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

func TestGoGateWorkerContextsReachOwnedStageBoundary(t *testing.T) {
	for _, test := range []struct {
		name, attempt       string
		worker, eligibility int
		launchStatus        int
		wantStatus          int
		want                string
		wantBegins          int
		wantAuditSkip       int
	}{
		{name: "legitimate legacy worker", worker: 0, eligibility: 97, wantStatus: 1, want: "gofmt itself failed (status 79)", wantAuditSkip: 1},
		{name: "authenticated foreign descendant", attempt: "proof-parent", worker: 0, eligibility: 3, wantStatus: 1, want: "gofmt itself failed (status 79)", wantAuditSkip: 1},
		{name: "eligible gate refuses before measurement", attempt: "proof-parent", worker: 0, eligibility: 0, wantStatus: 1, want: "gofmt itself failed (status 79)", wantAuditSkip: 1},
		{name: "invalid custody", attempt: "proof-forged", worker: 3, eligibility: 3, launchStatus: ExitAdmissionRefused, wantStatus: ExitAdmissionRefused},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			copyScriptFile(t, filepath.Join(packageRoot(t), "scripts", "agents", "go-gate.sh"), filepath.Join(root, "scripts", "agents", "go-gate.sh"))
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/widoriezebos/agentic-tools/metasystem\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			helperDir := filepath.Join(root, "helpers")
			if err := os.MkdirAll(helperDir, 0o755); err != nil {
				t.Fatal(err)
			}
			auth := filepath.Join(helperDir, "metasystem")
			beginCount := filepath.Join(root, "coverage-begin-count")
			writeEntrypointHelperWrapper(t, auth, "engine")
			writeEntrypointHelperWrapper(t, filepath.Join(helperDir, "go"), "go")
			if err := testexec.WriteFile(filepath.Join(helperDir, "gofmt"), []byte("#!/usr/bin/env bash\nexit 79\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			command := exec.Command("bash", filepath.Join(root, "scripts", "agents", "go-gate.sh"))
			command.Dir = root
			command.Env = append(filteredCoverageScriptEnvironment(),
				"GO_WANT_PROOF_ENTRYPOINT_HELPER=1", "PROOF_ENTRYPOINT_HELPER="+coverageScriptExecutable(t),
				"PROOF_ENTRYPOINT_STATUS="+strconv.Itoa(test.launchStatus), "PROOF_ENTRYPOINT_WORKER_STATUS="+strconv.Itoa(test.worker),
				"PROOF_ENTRYPOINT_ELIGIBILITY_STATUS="+strconv.Itoa(test.eligibility),
				"PROOF_ENTRYPOINT_COVERAGE_BEGIN_COUNT="+beginCount,
				"METASYSTEM_PROOF_AUTH_BIN="+auth, "METASYSTEM_PROOF_CONTROL_ROOT="+root,
				"METASYSTEM_PROOF_ATTEMPT="+test.attempt,
				"PATH="+helperDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			output, err := command.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != test.wantStatus || test.want != "" && !strings.Contains(string(output), test.want) {
				t.Fatalf("worker boundary exit=%v want status=%d diagnostic=%q output:\n%s", err, test.wantStatus, test.want, output)
			}
			if count := strings.Count(string(output), "SessionStart exit audit not applicable to this script fixture"); count != test.wantAuditSkip {
				t.Fatalf("worker boundary reported the SessionStart audit scope %d times, want %d; output:\n%s", count, test.wantAuditSkip, output)
			}
			begins := 0
			if raw, readErr := os.ReadFile(beginCount); readErr == nil {
				begins, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			} else if !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			if begins != test.wantBegins {
				t.Fatalf("coverage producer began %d times before the measurement boundary, want %d; output:\n%s", begins, test.wantBegins, output)
			}
		})
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

func TestFastGoGatePropagatesInheritedWorkersToEveryGoPhase(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, workers string
		setWorkers    bool
	}{
		{name: "explicit", workers: "3", setWorkers: true},
		{name: "direct caller default", workers: "1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRecordingGoFixture(t)
			output := filepath.Join(fixture.root, "proof-engine")
			diagnostic, err := fixture.run(t, "scripts/agents/go-gate.sh", test.workers, test.setWorkers, "--fast", "--proof-out", output)
			if err != nil {
				t.Fatalf("fast gate failed: %v\n%s", err, diagnostic)
			}
			records := readRecordedGoInvocations(t, fixture.goLog)
			assertRecordedGoAllowance(t, records, test.workers)
			for _, phase := range []string{
				"run -p=" + test.workers + " ./cmd/metasystem audit dependency-ratchet",
				"run -p=" + test.workers + " ./cmd/metasystem audit parallel-ratchet",
				"vet -p=" + test.workers + " ./...",
				"run -p=" + test.workers + " honnef.co/go/tools/cmd/staticcheck@v0.8.0 ./...",
				"run -p=" + test.workers + " ./cmd/metasystem behavior-surface select",
				"test -p=" + test.workers + " -count=1 ./internal/refusal",
			} {
				requireRecordedGoPhase(t, records, phase)
			}
		})
	}

	fixture := newRecordingGoFixture(t)
	diagnostic, err := fixture.run(t, "scripts/agents/go-gate.sh", "invalid", true, "--fast")
	if err == nil || !strings.Contains(string(diagnostic), "METASYSTEM_TEST_WORKERS must be a positive integer") {
		t.Fatalf("invalid allowance err=%v output:\n%s", err, diagnostic)
	}
	if records := readRecordedGoInvocations(t, fixture.goLog); len(records) != 0 {
		t.Fatalf("invalid allowance reached Go: %+v", records)
	}
}

func TestFullGoGatePropagatesInheritedWorkersToEveryGoPhase(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, workers string
		setWorkers    bool
	}{
		{name: "explicit", workers: "3", setWorkers: true},
		{name: "direct caller default", workers: "1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRecordingGoFixture(t)
			diagnostic, err := fixture.run(t, "scripts/agents/go-gate.sh", test.workers, test.setWorkers)
			if err != nil {
				t.Fatalf("full gate failed: %v\n%s", err, diagnostic)
			}
			records := readRecordedGoInvocations(t, fixture.goLog)
			assertRecordedGoAllowance(t, records, test.workers)
			for _, phase := range []string{
				"build -p=" + test.workers + " -o ",
				"vet -p=" + test.workers + " ./...",
				"run -p=" + test.workers + " honnef.co/go/tools/cmd/staticcheck@v0.8.0 ./...",
				"run -p=" + test.workers + " ./cmd/metasystem behavior-surface select",
				"run -p=" + test.workers + " golang.org/x/vuln/cmd/govulncheck@v1.2.0 ./...",
				"list -p=" + test.workers + " ./internal/...",
			} {
				requireRecordedGoPhase(t, records, phase)
			}
			if countRecordedGoPhase(records, "build -p="+test.workers+" ./...") != 2 {
				t.Errorf("full gate did not record both Linux cross-builds: %+v", records)
			}
			native := readNULFields(t, fixture.nativeLog)
			wantNative := []string{test.workers, test.workers, "proof-run", "go-gate-tests"}
			if len(native) < len(wantNative) || !reflect.DeepEqual(native[:len(wantNative)], wantNative) || !containsAdjacentFields(native, "--workers", test.workers) {
				t.Fatalf("native boundary fields=%q, want GOMAXPROCS and METASYSTEM_TEST_WORKERS %s with --workers %s", native, test.workers, test.workers)
			}
		})
	}

	t.Run("frozen witness re-exec", func(t *testing.T) {
		fixture := newRecordingGoFixture(t)
		stateRoot := filepath.Join(fixture.root, "witness-state")
		freezeSnapshot := filepath.Join(fixture.root, "freeze-snapshot")
		if err := os.MkdirAll(stateRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		witness := filepath.Join(stateRoot, "gate-witness.json")
		witnessBody := `{"runId":"recording-run","policyVersion":1,"engineDigest":"0000000000000000000000000000000000000000000000000000000000000000","manifestDigest":"2222222222222222222222222222222222222222222222222222222222222222","payloadDigest":"","payloadManifest":"","toolchainIdentity":"1111111111111111111111111111111111111111111111111111111111111111","controller":{"pid":1,"startedAtSec":1,"startTicks":0,"bootId":""}}`
		if err := os.WriteFile(witness, []byte(witnessBody+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		diagnostic, err := fixture.runWithEnvironment(t, "scripts/agents/go-gate.sh", "3", true, []string{
			"METASYSTEM_GATE_WITNESS=" + witness,
			"METASYSTEM_GATE_WITNESS_ROOT=" + stateRoot,
			"METASYSTEM_GATE_WITNESS_RUN=recording-run",
			"METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE",
			"RECORDING_FREEZE_SNAPSHOT=" + freezeSnapshot,
		})
		if err != nil {
			t.Fatalf("frozen witness gate failed: %v\n%s", err, diagnostic)
		}
		records := readRecordedGoInvocations(t, fixture.goLog)
		assertRecordedGoAllowance(t, records, "3")
		for _, phase := range []string{
			"run -p=3 ./cmd/metasystem gate witness-freeze --root",
			"list -p=3 -f {{.Dir}} ./...",
			"run -p=3 ./cmd/metasystem gate controller-descendant",
			"run -p=3 ./cmd/metasystem gate witness-verify",
			"run -p=3 ./cmd/metasystem behavior-surface digest",
			"run -p=3 ./cmd/metasystem gate witness-freeze --cleanup",
		} {
			requireRecordedGoPhase(t, records, phase)
		}
		skipAllowed := "run -p=3 ./cmd/metasystem behavior-surface skip-allowed"
		if countRecordedGoPhase(records, skipAllowed) > 0 {
			requireRecordedGoPhase(t, records, skipAllowed)
		} else {
			if !strings.Contains(string(diagnostic), "go gate: witness not accepted; running the full gate") {
				t.Fatalf("frozen witness neither used the accepted witness nor diagnosed the full-gate fallback:\n%s", diagnostic)
			}
			for _, phase := range []string{
				"vet -p=3 ./...",
				"run -p=3 honnef.co/go/tools/cmd/staticcheck@v0.8.0 ./...",
				"run -p=3 ./cmd/metasystem behavior-surface select",
				"run -p=3 golang.org/x/vuln/cmd/govulncheck@v1.2.0 ./...",
				"list -p=3 ./internal/...",
			} {
				requireRecordedGoPhase(t, records, phase)
			}
			native := readNULFields(t, fixture.nativeLog)
			wantNative := []string{"3", "3", "proof-run", "go-gate-tests"}
			if len(native) < len(wantNative) || !reflect.DeepEqual(native[:len(wantNative)], wantNative) || !containsAdjacentFields(native, "--workers", "3") {
				t.Fatalf("full-gate fallback native boundary fields=%q, want GOMAXPROCS and METASYSTEM_TEST_WORKERS 3 with --workers 3", native)
			}
		}
	})

	fixture := newRecordingGoFixture(t)
	diagnostic, err := fixture.run(t, "scripts/agents/go-gate.sh", "0", true)
	if err == nil || !strings.Contains(string(diagnostic), "METASYSTEM_TEST_WORKERS must be a positive integer") {
		t.Fatalf("invalid allowance err=%v output:\n%s", err, diagnostic)
	}
	if records := readRecordedGoInvocations(t, fixture.goLog); len(records) != 0 {
		t.Fatalf("invalid allowance reached Go: %+v", records)
	}
}

func TestGoBuildPropagatesInheritedWorkersToClassificationAndBuild(t *testing.T) {
	t.Parallel()

	fixture := newRecordingGoFixture(t)
	for _, args := range [][]string{{"--out", filepath.Join(fixture.root, "proof-engine")}, nil} {
		diagnostic, err := fixture.run(t, "scripts/agents/go-build.sh", "3", true, args...)
		if err != nil {
			t.Fatalf("go-build %v failed: %v\n%s", args, err, diagnostic)
		}
	}
	records := readRecordedGoInvocations(t, fixture.goLog)
	assertRecordedGoAllowance(t, records, "3")
	if countRecordedGoPhase(records, "run -p=3 ./cmd/metasystem behavior-surface select") != 2 || countRecordedGoPhase(records, "build -p=3") != 2 {
		t.Fatalf("go-build did not classify and build both branches: %+v", records)
	}

	defaultFixture := newRecordingGoFixture(t)
	if diagnostic, err := defaultFixture.run(t, "scripts/agents/go-build.sh", "1", false, "--out", filepath.Join(defaultFixture.root, "proof-engine")); err != nil {
		t.Fatalf("default go-build failed: %v\n%s", err, diagnostic)
	}
	assertRecordedGoAllowance(t, readRecordedGoInvocations(t, defaultFixture.goLog), "1")

	invalidFixture := newRecordingGoFixture(t)
	diagnostic, err := invalidFixture.run(t, "scripts/agents/go-build.sh", "nope", true, "--out", filepath.Join(invalidFixture.root, "proof-engine"))
	if err == nil || !strings.Contains(string(diagnostic), "METASYSTEM_TEST_WORKERS must be a positive integer") {
		t.Fatalf("invalid allowance err=%v output:\n%s", err, diagnostic)
	}
	if records := readRecordedGoInvocations(t, invalidFixture.goLog); len(records) != 0 {
		t.Fatalf("invalid allowance reached Go: %+v", records)
	}
}

func newRecordingGoFixture(t *testing.T) recordingGoFixture {
	t.Helper()
	root := t.TempDir()
	for _, relative := range []string{"scripts/agents/go-gate.sh", "scripts/agents/go-build.sh"} {
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

func TestFullGoGateRetainsCoverageEvidenceOutsideFrozenWitnessOnConsumerFailure(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name              string
		environment       []string
		removeBaseline    bool
		wantFailure       string
		wantBundle        bool
		wantInventory     bool
		inventoryNonempty bool
		wantStderrOnly    bool
	}{
		{name: "post-native build failure", environment: []string{"RECORDING_POST_NATIVE_BUILD_FAIL=1"}, wantFailure: "build failed", wantBundle: true},
		{name: "post-native implicit exit", environment: []string{"RECORDING_NATIVE_CAT_FAIL=1"}, wantFailure: "post-native exit before coverage consumers completed", wantBundle: true},
		{name: "inventory failure", environment: []string{"RECORDING_INVENTORY_FAIL=1"}, wantFailure: "go list failed", wantBundle: true, wantInventory: true},
		{name: "missing baseline", removeBaseline: true, wantFailure: "no coverage baseline", wantBundle: true, wantInventory: true, inventoryNonempty: true},
		{name: "legacy ratchet failure", environment: []string{"RECORDING_COVERAGE_CONSUMER_FAIL=1"}, wantFailure: "coverage ratchet refused", wantBundle: true, wantInventory: true, inventoryNonempty: true},
		{name: "native stderr is diagnostic only", environment: []string{"RECORDING_NATIVE_COVERAGE_STDERR=1", "RECORDING_COVERAGE_CONSUMER_FAIL=1"}, wantFailure: "coverage ratchet refused", wantBundle: true, wantInventory: true, inventoryNonempty: true, wantStderrOnly: true},
		{name: "publication failure", environment: []string{"METASYSTEM_PROOF_ATTEMPT=retention-attempt", "RECORDING_COVERAGE_PUBLICATION_FAIL=1"}, wantFailure: "authenticated coverage publication refused", wantBundle: true, wantInventory: true, inventoryNonempty: true},
		{name: "seed success", environment: []string{"METASYSTEM_COVERAGE_RATCHET_SEED=1"}, wantFailure: "floors were not enforced", wantBundle: true, wantInventory: true, inventoryNonempty: true},
		{name: "enforcing success cleanup", wantInventory: true, inventoryNonempty: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRecordingGoFixture(t)
			stateRoot := filepath.Join(fixture.root, "witness-state")
			freezeSnapshot := filepath.Join(fixture.root, "freeze-snapshot")
			freezeExport := filepath.Join(t.TempDir(), "frozen-export")
			durable := filepath.Join(t.TempDir(), "durable-proof")
			if err := os.MkdirAll(stateRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			if test.removeBaseline {
				for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
					if err := os.Remove(filepath.Join(fixture.root, "scripts", "agents", name)); err != nil {
						t.Fatal(err)
					}
				}
			}
			witness := filepath.Join(stateRoot, "gate-witness.json")
			body := `{"runId":"retention-run","policyVersion":1,"engineDigest":"0000000000000000000000000000000000000000000000000000000000000000","manifestDigest":"2222222222222222222222222222222222222222222222222222222222222222","payloadDigest":"","payloadManifest":"","toolchainIdentity":"1111111111111111111111111111111111111111111111111111111111111111","controller":{"pid":1,"startedAtSec":1,"startTicks":0,"bootId":""}}`
			if err := os.WriteFile(witness, []byte(body+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			environment := []string{
				"METASYSTEM_GATE_WITNESS=" + witness, "METASYSTEM_GATE_WITNESS_ROOT=" + stateRoot,
				"METASYSTEM_GATE_WITNESS_RUN=retention-run", "METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE",
				"RECORDING_FREEZE_SNAPSHOT=" + freezeSnapshot, "RECORDING_FREEZE_EXPORT=" + freezeExport,
				"METASYSTEM_PROOF_CONTROL_ROOT=" + durable,
				"METASYSTEM_PROOF_AUTH_BIN=" + filepath.Join(fixture.root, "helpers", "engine-stub"),
			}
			environment = append(environment, test.environment...)
			diagnostic, runErr := fixture.runWithEnvironment(t, "scripts/agents/go-gate.sh", "1", true, environment)
			if test.wantFailure == "" {
				if runErr != nil {
					t.Fatalf("enforcing success failed: %v\n%s", runErr, diagnostic)
				}
			} else if strings.Contains(test.name, "success") {
				if runErr != nil || !strings.Contains(string(diagnostic), test.wantFailure) {
					t.Fatalf("seed success missing: %v\n%s", runErr, diagnostic)
				}
			} else if runErr == nil || !strings.Contains(string(diagnostic), test.wantFailure) {
				t.Fatalf("expected failure %q missing: %v\n%s", test.wantFailure, runErr, diagnostic)
			}
			if _, err := os.Stat(freezeExport); !os.IsNotExist(err) {
				t.Fatalf("frozen export survived cleanup: %v", err)
			}
			matches, globErr := filepath.Glob(filepath.Join(durable, "gate-failures", "*-coverage"))
			if globErr != nil || len(matches) != map[bool]int{false: 0, true: 1}[test.wantBundle] {
				t.Fatalf("durable bundles=%v err=%v\n%s", matches, globErr, diagnostic)
			}
			if !test.wantBundle {
				return
			}
			for _, relative := range []string{"coverage.jsonl", "native/go-gate-native.log", "native/counters"} {
				if data, readErr := os.ReadFile(filepath.Join(matches[0], relative)); readErr != nil || len(data) == 0 {
					t.Fatalf("durable %s missing: %v", relative, readErr)
				}
			}
			if test.wantStderrOnly {
				const stderrCoverage = "example.invalid/internal/stderr-only"
				coverage, coverageErr := os.ReadFile(filepath.Join(matches[0], "coverage.jsonl"))
				stderr, stderrErr := os.ReadFile(filepath.Join(matches[0], "native", "go-gate-tests.stderr.log"))
				if coverageErr != nil || strings.Contains(string(coverage), stderrCoverage) {
					t.Fatalf("stderr entered retained coverage input: err=%v coverage=%q", coverageErr, coverage)
				}
				if stderrErr != nil || !strings.Contains(string(stderr), stderrCoverage) || !strings.Contains(string(diagnostic), stderrCoverage) {
					t.Fatalf("stderr was not retained and reported as diagnostics: err=%v stderr=%q output=%q", stderrErr, stderr, diagnostic)
				}
			}
			inventory, inventoryErr := os.ReadFile(filepath.Join(matches[0], "package-inventory.txt"))
			if test.wantInventory && (inventoryErr != nil || test.inventoryNonempty && len(inventory) == 0) {
				t.Fatalf("durable package inventory missing or empty: bytes=%d err=%v", len(inventory), inventoryErr)
			}
			if !test.wantInventory && !os.IsNotExist(inventoryErr) {
				t.Fatalf("build failure unexpectedly created an inventory: bytes=%d err=%v", len(inventory), inventoryErr)
			}
		})
	}
}

func (fixture recordingGoFixture) run(t *testing.T, relative, workers string, setWorkers bool, arguments ...string) ([]byte, error) {
	t.Helper()
	return fixture.runWithEnvironment(t, relative, workers, setWorkers, []string{"METASYSTEM_COVERAGE_RATCHET_SEED=1"}, arguments...)
}

func (fixture recordingGoFixture) runWithEnvironment(t *testing.T, relative, workers string, setWorkers bool, extraEnvironment []string, arguments ...string) ([]byte, error) {
	t.Helper()
	command := exec.Command("bash", append([]string{filepath.Join(fixture.root, relative)}, arguments...)...)
	command.Dir = fixture.root
	environment := recordingGoEnvironment()
	environment = append(environment,
		"PATH="+filepath.Join(fixture.root, "helpers")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"RECORDING_GO_LOG="+fixture.goLog,
		"RECORDING_NATIVE_LOG="+fixture.nativeLog,
		"RECORDING_ENGINE_STUB="+filepath.Join(fixture.root, "helpers", "engine-stub"),
		"METASYSTEM_ALLOW_CONCURRENT_GATE=1",
	)
	environment = append(environment, extraEnvironment...)
	if setWorkers {
		environment = append(environment, "METASYSTEM_TEST_WORKERS="+workers)
	}
	command.Env = environment
	return command.CombinedOutput()
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

func assertRecordedGoAllowance(t *testing.T, records []recordedGoInvocation, workers string) {
	t.Helper()
	if len(records) == 0 {
		t.Fatal("no Go invocations were recorded")
	}
	for _, record := range records {
		if record.gomaxprocs != workers {
			t.Errorf("go %q GOMAXPROCS=%q, want %q", record.argv, record.gomaxprocs, workers)
		}
		wantParallelFlag := false
		switch record.argv[0] {
		case "build", "vet", "test", "list", "run":
			wantParallelFlag = true
		}
		hasParallelFlag := false
		for _, argument := range record.argv[1:] {
			if argument == "-p="+workers {
				hasParallelFlag = true
			}
		}
		if hasParallelFlag != wantParallelFlag {
			t.Errorf("go %q -p presence=%v, want %v", record.argv, hasParallelFlag, wantParallelFlag)
		}
	}
}

func requireRecordedGoPhase(t *testing.T, records []recordedGoInvocation, phase string) {
	t.Helper()
	if countRecordedGoPhase(records, phase) == 0 {
		t.Errorf("missing Go phase %q in %+v", phase, records)
	}
}

func countRecordedGoPhase(records []recordedGoInvocation, phase string) int {
	count := 0
	for _, record := range records {
		if strings.Contains(strings.Join(record.argv, " "), phase) {
			count++
		}
	}
	return count
}

func containsAdjacentFields(fields []string, first, second string) bool {
	for index := 0; index+1 < len(fields); index++ {
		if fields[index] == first && fields[index+1] == second {
			return true
		}
	}
	return false
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

func coverageScriptEngineHelper(arguments []string) int {
	joined := strings.Join(arguments, " ")
	switch {
	case strings.HasPrefix(joined, "proof-run coverage-reuse "):
		status, _ := strconv.Atoi(os.Getenv("COVERAGE_SCRIPT_REUSE_STATUS"))
		if status == 0 {
			fmt.Println("coverage evidence: retained full proof matched")
		}
		return status
	case strings.HasPrefix(joined, "proof-run worker-authorized "):
		if os.Getenv("COVERAGE_SCRIPT_PROOF_WORKER") == "1" {
			return 0
		}
		return 3
	case strings.HasPrefix(joined, "proof-run banner "):
		fmt.Println("coverage delta fixture banner")
		return 0
	case strings.HasPrefix(joined, "util sha256 "):
		fmt.Println(strings.Repeat("a", 64))
		return 0
	case strings.HasPrefix(joined, "proof-run launch "):
		separator := -1
		for index, argument := range arguments {
			if argument == "--" {
				separator = index
				break
			}
		}
		if separator < 0 || separator+1 >= len(arguments) {
			return 97
		}
		command := exec.Command(arguments[separator+1], arguments[separator+2:]...)
		command.Env = append(os.Environ(), "COVERAGE_SCRIPT_PROOF_WORKER=1")
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				return exit.ExitCode()
			}
			return 97
		}
		return 0
	case strings.Contains(joined, "json get") && strings.Contains(joined, "--field floors.internal/proofrun"):
		fmt.Println("80")
		return 0
	case strings.Contains(joined, "json get") && strings.Contains(joined, "--field floors"):
		fmt.Println(`{"internal/proofrun":80}`)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unexpected engine arguments: %q\n", arguments)
		return 97
	}
}

func coverageScriptGoHelper(arguments []string) int {
	if len(arguments) == 0 || arguments[0] != "test" {
		fmt.Fprintf(os.Stderr, "unexpected go arguments: %q\n", arguments)
		return 97
	}
	path := os.Getenv("COVERAGE_SCRIPT_LAUNCH_COUNT")
	launches := 1
	if raw, err := os.ReadFile(path); err == nil {
		launches, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
		launches++
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(launches)+"\n"), 0o600); err != nil {
		return 97
	}
	fmt.Println("ok  example.invalid/metasystem/internal/proofrun 0.1s coverage: 85.0% of statements")
	return 0
}

func coverageDeltaScriptFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{filepath.Join(root, "scripts", "agents"), filepath.Join(root, "bin"), filepath.Join(root, "helpers")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate coverage consumer source")
	}
	sourcePath := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "scripts", "agents", "coverage-delta.sh"))
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(root, "scripts", "agents", "coverage-delta.sh"), source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/metasystem\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ratchet := []byte(`{"floors":{"internal/proofrun":80},"exempt":{}}`)
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		if err := os.WriteFile(filepath.Join(root, "scripts", "agents", name), ratchet, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeCoverageScriptWrapper(t, filepath.Join(root, "bin", "metasystem"), "engine")
	// The normal wrapper-provided override must win over the installed
	// generation. Make the installed path unusable so retained reuse proves
	// it reached the candidate engine selected by METASYSTEM_BIN.
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte("#!/usr/bin/env bash\nexit 97\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeCoverageScriptWrapper(t, filepath.Join(root, "helpers", "metasystem"), "engine")
	writeCoverageScriptWrapper(t, filepath.Join(root, "helpers", "go"), "go")
	return root
}

func writeCoverageScriptWrapper(t *testing.T, path, mode string) {
	t.Helper()
	wrapper := "#!/usr/bin/env bash\nexec \"$COVERAGE_SCRIPT_HELPER\" -test.run=^TestCoverageDeltaProductionConsumerHelper$ -- " + mode + " \"$@\"\n"
	if err := testexec.WriteFile(path, []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
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
