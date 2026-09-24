package proofrun

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type relaunchScriptSite struct {
	name       string
	relative   string
	marker     string
	diagnostic string
	arguments  []string
}

var relaunchScriptSites = []relaunchScriptSite{
	{name: "go gate", relative: "scripts/agents/go-gate.sh", marker: "METASYSTEM_GO_GATE_RELAUNCHED", diagnostic: "go gate: relaunched child is not an authorized proof worker"},
	{name: "validator", relative: "scripts/validate-metasystem.sh", marker: "METASYSTEM_VALIDATE_RELAUNCHED", diagnostic: "validate metasystem: relaunched child is not an authorized proof worker"},
	{name: "adoption fixtures", relative: "scripts/adopt-fixtures.sh", marker: "METASYSTEM_ADOPT_FIXTURES_RELAUNCHED", diagnostic: "adopt fixtures: relaunched child is not an authorized proof worker"},
	{name: "coverage delta", relative: "scripts/agents/coverage-delta.sh", marker: "METASYSTEM_COVERAGE_DELTA_RELAUNCHED", diagnostic: "coverage delta: relaunched child is not an authorized proof worker", arguments: []string{"internal/proofrun"}},
}

type relaunchScriptFixture struct {
	root, engine, launches, launchArguments, workerChecks, progressed string
}

func TestProofScriptsRefuseUnauthorizedRelaunchedChildren(t *testing.T) {
	t.Parallel()
	for _, site := range relaunchScriptSites {
		site := site
		t.Run(site.name, func(t *testing.T) {
			t.Parallel()
			fixture := newRelaunchScriptFixture(t, site)
			environment := fixture.environment(site, 3, false)
			environment = append(environment, site.marker+"=1")
			output, status := runRelaunchScript(t, fixture.root, site, environment)
			if status != 1 || !strings.Contains(output, site.diagnostic) {
				t.Fatalf("status=%d want=1 diagnostic=%q output:\n%s", status, site.diagnostic, output)
			}
			assertRelaunchCount(t, fixture.launches, 0)
		})
	}
}

func TestProofScriptsExportDepthAndAuthenticationEngine(t *testing.T) {
	t.Parallel()
	for _, site := range relaunchScriptSites {
		site := site
		t.Run(site.name, func(t *testing.T) {
			t.Parallel()
			fixture := newRelaunchScriptFixture(t, site)
			output, status := runRelaunchScript(t, fixture.root, site, fixture.environment(site, 3, false))
			if status != 23 {
				t.Fatalf("status=%d want=23 output:\n%s", status, output)
			}
			assertRelaunchCount(t, fixture.launches, 1)
			record, err := os.ReadFile(fixture.launchArguments)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(record)), "\n")
			if len(lines) == 0 || !strings.HasPrefix(lines[0], "self=") {
				t.Fatalf("launch record has no engine identity:\n%s", record)
			}
			engine := strings.TrimPrefix(lines[0], "self=")
			want := []string{"arg=env", "arg=" + site.marker + "=1", "arg=METASYSTEM_PROOF_AUTH_BIN=" + engine}
			for _, entry := range want {
				if !stringLinesContain(lines, entry) {
					t.Errorf("launch record lacks %q:\n%s", entry, record)
				}
			}
		})
	}
}

func TestGoGateCopiedRootStopsAfterOneUnauthorizedRelaunch(t *testing.T) {
	t.Parallel()
	site := relaunchScriptSites[0]
	fixture := newRelaunchScriptFixture(t, site)
	environment := fixture.environment(site, 3, true)
	environment = append(environment, "METASYSTEM_PROOF_AUTH_BIN=")
	output, status := runRelaunchScript(t, fixture.root, site, environment)
	if status != 1 || !strings.Contains(output, site.diagnostic) {
		t.Fatalf("status=%d want=1 diagnostic=%q output:\n%s", status, site.diagnostic, output)
	}
	assertRelaunchCount(t, fixture.launches, 1)
	assertRelaunchCount(t, fixture.workerChecks, 1)
	if _, err := os.Stat(fixture.progressed); !os.IsNotExist(err) {
		t.Fatalf("unauthorized child progressed past its worker check: %v", err)
	}
}

func TestGoGateRelaunchUsesBuiltEngineForWorkerAuthorization(t *testing.T) {
	t.Parallel()
	site := relaunchScriptSites[0]
	fixture := newRelaunchScriptFixture(t, site)
	environment := fixture.environment(site, 0, true)
	environment = append(environment, "METASYSTEM_PROOF_AUTH_BIN=")
	output, status := runRelaunchScript(t, fixture.root, site, environment)
	if status == 0 {
		t.Fatalf("bounded gate fixture unexpectedly completed:\n%s", output)
	}
	assertRelaunchCount(t, fixture.launches, 1)
	assertRelaunchCount(t, fixture.workerChecks, 1)
	if _, err := os.Stat(fixture.progressed); err != nil {
		t.Fatalf("authorized child did not reach the bounded Go stub: %v\n%s", err, output)
	}
}

func TestValidatorGuardFixtureRequiresAndSelectsExplicitBoundedInvocation(t *testing.T) {
	t.Parallel()
	site := relaunchScriptSites[1]
	control := filepath.Join(t.TempDir(), "guard-control.json")

	ambientFixture := newRelaunchScriptFixture(t, site)
	ambientEnvironment := append(ambientFixture.environment(site, 3, false),
		"METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE="+control)
	output, status := runRelaunchScript(t, ambientFixture.root, site, ambientEnvironment)
	if status != 2 || !strings.Contains(output, "requires the explicit bounded fixture argument") {
		t.Fatalf("ambient fixture status=%d output=%q", status, output)
	}
	assertRelaunchCount(t, ambientFixture.launches, 0)

	site.arguments = []string{"--checkout-execution-guard-fixture", control}
	explicitFixture := newRelaunchScriptFixture(t, site)
	output, status = runRelaunchScript(t, explicitFixture.root, site, explicitFixture.environment(site, 3, false))
	if status != 23 {
		t.Fatalf("explicit fixture status=%d want=23 output=%q", status, output)
	}
	assertRelaunchCount(t, explicitFixture.launches, 1)
	record, err := os.ReadFile(explicitFixture.launchArguments)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(record)), "\n")
	for _, want := range []string{
		"arg=--selected",
		"arg=checkout-execution-guard-fixture",
		"arg=METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE=" + control,
		"arg=--checkout-execution-guard-fixture",
		"arg=" + control,
	} {
		if !stringLinesContain(lines, want) {
			t.Errorf("launch record lacks %q:\n%s", want, record)
		}
	}
}

func newRelaunchScriptFixture(t *testing.T, site relaunchScriptSite) relaunchScriptFixture {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{"helpers", "bin", "scripts/agents"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	copyScriptFile(t, filepath.Join(packageRoot(t), site.relative), filepath.Join(root, site.relative))
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/widoriezebos/agentic-tools/metasystem\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ratchet := []byte(`{"floors":{"internal/proofrun":80},"exempt":{}}`)
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		if err := os.WriteFile(filepath.Join(root, "scripts", "agents", name), ratchet, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	selector := "#!/usr/bin/env bash\n[[ ${1:-} == context ]] || exit 97\nprintf 'template\\n'\n"
	if err := testexec.WriteFile(filepath.Join(root, "scripts", "agents", "validate-section-selector.sh"), []byte(selector), 0o700); err != nil {
		t.Fatal(err)
	}
	fixtureBudget := "#!/usr/bin/env bash\nharness_fixture_bed_child_scenario() { return 1; }\n"
	if err := testexec.WriteFile(filepath.Join(root, "scripts", "agents", "fixture-budget.sh"), []byte(fixtureBudget), 0o700); err != nil {
		t.Fatal(err)
	}

	fixture := relaunchScriptFixture{
		root:            root,
		engine:          filepath.Join(root, "helpers", "recording-engine"),
		launches:        filepath.Join(root, "launch-count"),
		launchArguments: filepath.Join(root, "launch-arguments"),
		workerChecks:    filepath.Join(root, "worker-check-count"),
		progressed:      filepath.Join(root, "child-progressed"),
	}
	engine := `#!/usr/bin/env bash
set -u
increment() {
  local path=$1 count=0
  [[ ! -f "$path" ]] || count=$(cat "$path")
  printf '%d\n' "$((count + 1))" >"$path"
}
case "${1:-}/${2:-}" in
  proof-run/coverage-reuse) exit 3 ;;
  proof-run/worker-authorized)
    increment "$PROOF_SCRIPT_WORKER_CHECKS"
    exit "$PROOF_SCRIPT_WORKER_STATUS"
    ;;
  proof-run/banner) printf 'relaunch fixture banner\n' ;;
  proof-run/launch)
    increment "$PROOF_SCRIPT_LAUNCH_COUNT"
    { printf 'self=%s\n' "$0"; printf 'arg=%s\n' "$@"; } >"$PROOF_SCRIPT_LAUNCH_ARGUMENTS"
    if [[ "$PROOF_SCRIPT_RUN_CHILD" == 1 ]]; then
      [[ $(cat "$PROOF_SCRIPT_LAUNCH_COUNT") -le 1 ]] || exit 88
      while (($#)) && [[ "$1" != -- ]]; do shift; done
      (($#)) || exit 97
      shift
      "$@"
      exit $?
    fi
    exit "$PROOF_SCRIPT_LAUNCH_STATUS"
    ;;
  json/get)
    if [[ " $* " == *" --field floors "* ]]; then
      printf '{"internal/proofrun":80}\n'
    else
      printf '80\n'
    fi
    ;;
  util/sha256) printf '%064d\n' 0 ;;
  *) printf 'unexpected recording engine invocation: %q\n' "$*" >&2; exit 97 ;;
esac
`
	if err := testexec.WriteFile(fixture.engine, []byte(engine), 0o700); err != nil {
		t.Fatal(err)
	}
	goStub := `#!/usr/bin/env bash
set -u
if [[ "${1:-}" == build ]]; then
  output=
  while (($#)); do
    [[ "$1" != -o ]] || { output=$2; break; }
    shift
  done
  [[ -n "$output" ]] || exit 97
  cp "$PROOF_SCRIPT_ENGINE_TEMPLATE" "$output"
  chmod 700 "$output"
  exit 0
fi
: >"$PROOF_SCRIPT_PROGRESSED"
exit 79
`
	if err := testexec.WriteFile(filepath.Join(root, "helpers", "go"), []byte(goStub), 0o700); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (fixture relaunchScriptFixture) environment(site relaunchScriptSite, workerStatus int, runChild bool) []string {
	runChildValue := "0"
	if runChild {
		runChildValue = "1"
	}
	return append(filteredCoverageScriptEnvironment(),
		"PATH="+filepath.Join(fixture.root, "helpers")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"METASYSTEM_BIN="+fixture.engine,
		"METASYSTEM_PROOF_AUTH_BIN="+fixture.engine,
		"METASYSTEM_SUITE_PROGRESS_ACTIVE=1",
		"METASYSTEM_SUITE_PROGRESS_ROOT="+fixture.root,
		"PROOF_SCRIPT_ENGINE_TEMPLATE="+fixture.engine,
		"PROOF_SCRIPT_LAUNCH_COUNT="+fixture.launches,
		"PROOF_SCRIPT_LAUNCH_ARGUMENTS="+fixture.launchArguments,
		"PROOF_SCRIPT_WORKER_CHECKS="+fixture.workerChecks,
		"PROOF_SCRIPT_PROGRESSED="+fixture.progressed,
		"PROOF_SCRIPT_WORKER_STATUS="+strconv.Itoa(workerStatus),
		"PROOF_SCRIPT_LAUNCH_STATUS=23",
		"PROOF_SCRIPT_RUN_CHILD="+runChildValue,
	)
}

func runRelaunchScript(t *testing.T, root string, site relaunchScriptSite, environment []string) (string, int) {
	t.Helper()
	arguments := append([]string{filepath.Join(root, site.relative)}, site.arguments...)
	command := exec.Command("bash", arguments...)
	command.Dir = root
	command.Env = environment
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return string(output), exit.ExitCode()
	}
	t.Fatalf("run %s: %v", site.name, err)
	return "", -1
}

func assertRelaunchCount(t *testing.T, path string, want int) {
	t.Helper()
	got := 0
	if raw, err := os.ReadFile(path); err == nil {
		var scanErr error
		if got, scanErr = strconv.Atoi(strings.TrimSpace(string(raw))); scanErr != nil {
			t.Fatal(scanErr)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s count=%d want=%d", filepath.Base(path), got, want)
	}
}

func stringLinesContain(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}
