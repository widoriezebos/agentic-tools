package proofrun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
			command := exec.Command("bash", filepath.Join(root, test.relative))
			command.Dir = root
			command.Env = append(filteredCoverageScriptEnvironment(),
				"GO_WANT_PROOF_ENTRYPOINT_HELPER=1", "PROOF_ENTRYPOINT_HELPER="+coverageScriptExecutable(t),
				"PROOF_ENTRYPOINT_STATUS="+strconv.Itoa(test.status), "PROOF_ENTRYPOINT_LAUNCH_COUNT="+count,
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
	goHelper := `#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  version) echo 'go version fixture' ;;
  env) echo local ;;
  vet|test) ;;
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
    printf 'one collected engine\n' >"$out"
    ;;
  *) exit 97 ;;
esac
`
	if err := os.WriteFile(filepath.Join(root, "helpers", "go"), []byte(goHelper), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "helpers", "gofmt"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "proof-engine")
	command := exec.Command("bash", filepath.Join(root, "scripts", "agents", "go-gate.sh"), "--fast", "--proof-out", output)
	command.Dir = root
	command.Env = append(filteredCoverageScriptEnvironment(), "FAST_GATE_BUILD_COUNT="+count,
		"METASYSTEM_BUILD_STAMP=fixture", "PATH="+filepath.Join(root, "helpers")+string(os.PathListSeparator)+os.Getenv("PATH"))
	if diagnostic, err := command.CombinedOutput(); err != nil {
		t.Fatalf("focused fast gate failed: %v\n%s", err, diagnostic)
	}
	builds, countErr := os.ReadFile(count)
	engine, engineErr := os.ReadFile(output)
	if countErr != nil || strings.TrimSpace(string(builds)) != "1" || engineErr != nil || string(engine) != "one collected engine\n" {
		t.Fatalf("fast gate builds=%q countErr=%v engine=%q engineErr=%v", builds, countErr, engine, engineErr)
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
	}{
		{name: "legitimate legacy worker", worker: 0, eligibility: 97, wantStatus: 1, want: "gofmt itself failed (status 79)"},
		{name: "authenticated foreign descendant", attempt: "proof-parent", worker: 0, eligibility: 3, wantStatus: 1, want: "gofmt itself failed (status 79)"},
		{name: "eligible gate refuses before measurement", attempt: "proof-parent", worker: 0, eligibility: 0, wantStatus: 1, want: "gofmt itself failed (status 79)"},
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
			if err := os.WriteFile(filepath.Join(helperDir, "gofmt"), []byte("#!/usr/bin/env bash\nexit 79\n"), 0o700); err != nil {
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
	if err := os.WriteFile(filepath.Join(root, "scripts", "agents", "coverage-delta.sh"), source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/metasystem\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "agents", "coverage-ratchet.json"),
		[]byte(`{"floors":{"internal/proofrun":80},"exempt":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	writeCoverageScriptWrapper(t, filepath.Join(root, "bin", "metasystem"), "engine")
	// The normal wrapper-provided override must win over the installed
	// generation. Make the installed path unusable so retained reuse proves
	// it reached the candidate engine selected by METASYSTEM_BIN.
	if err := os.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte("#!/usr/bin/env bash\nexit 97\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeCoverageScriptWrapper(t, filepath.Join(root, "helpers", "metasystem"), "engine")
	writeCoverageScriptWrapper(t, filepath.Join(root, "helpers", "go"), "go")
	return root
}

func writeCoverageScriptWrapper(t *testing.T, path, mode string) {
	t.Helper()
	wrapper := "#!/usr/bin/env bash\nexec \"$COVERAGE_SCRIPT_HELPER\" -test.run=^TestCoverageDeltaProductionConsumerHelper$ -- " + mode + " \"$@\"\n"
	if err := os.WriteFile(path, []byte(wrapper), 0o700); err != nil {
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
	if err := os.WriteFile(destination, data, 0o755); err != nil {
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
	return os.WriteFile(path, []byte(wrapper), 0o755)
}

func filteredCoverageScriptEnvironment() []string {
	var result []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "METASYSTEM_PROOF_") || strings.HasPrefix(key, "METASYSTEM_SUITE_PROGRESS_") {
			continue
		}
		switch key {
		case "GO_WANT_COVERAGE_SCRIPT_HELPER", "COVERAGE_SCRIPT_HELPER", "COVERAGE_SCRIPT_REUSE_STATUS", "COVERAGE_SCRIPT_LAUNCH_COUNT", "METASYSTEM_BIN", "PATH":
			continue
		}
		result = append(result, entry)
	}
	return result
}
