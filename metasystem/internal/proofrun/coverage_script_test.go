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
