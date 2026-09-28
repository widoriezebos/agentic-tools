package testutil

// Ports of the fixture-bed-scenarios-fixtures.sh bed. The subjects are the
// Bash fixture libraries scripts/agents/fixture-budget.sh and
// scripts/agents/fixture-bed-scenarios.sh, which other beds still source, so
// these tests drive the libraries themselves through Bash. The engine the
// libraries call (METASYSTEM_BIN) is this test binary in engine-stub mode
// (see TestMain), so no bin/metasystem build is needed. Every wait is a
// handshake: descendants publish readiness on a pipe or a file, and the
// library's own Bash clock (SECONDS) is advanced by function wrappers in the
// generated inner bed instead of waiting out its ceilings and grace periods.

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"golang.org/x/sys/unix"
)

// fixtureBedEngineStubEnv switches this test binary into the engine the
// fixture libraries call. It is set only in the environment of the Bash
// processes these tests start.
const fixtureBedEngineStubEnv = "FIXTURE_BED_ENGINE_STUB"

// fixtureBedInnerBed is the Bash bed the tests drive: it sources both
// libraries and runs its scenarios through run_fixture_bed_scenarios. The
// kill and sleep wrappers are the controlled clocks: the scheduler clock
// jumps to the live scenario deadline once the hang scenario's grandchild has
// published its pid (only when FIXTURE_BED_SCHEDULER_READY_FILE is set), and
// the reaper clock jumps once to fixture_bed_reap_group's TERM deadline.
const fixtureBedInnerBed = `#!/usr/bin/env bash
set -euo pipefail

if [ -n "${METASYSTEM_FIXTURE_OWNER-}" ]; then
  tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"
  attempt_tag=
  [ -z "${METASYSTEM_FIXTURE_ATTEMPT-}" ] || attempt_tag="METASYSTEM_FIXTURE_ATTEMPT=$METASYSTEM_FIXTURE_ATTEMPT"
  if [ "${1-}" != "$tag" ]; then
    [ -z "$attempt_tag" ] || exec /bin/bash "$0" "$tag" "$attempt_tag" "$@"
    exec /bin/bash "$0" "$tag" "$@"
  fi
  if [ -n "$attempt_tag" ] && [ "${2-}" != "$attempt_tag" ]; then
    shift
    exec /bin/bash "$0" "$tag" "$attempt_tag" "$@"
  fi
  shift
  [ -z "$attempt_tag" ] || shift
fi

root=${FIXTURE_BED_SOURCE_ROOT:?}
source "$root/scripts/agents/fixture-budget.sh"
source "$root/scripts/agents/fixture-bed-scenarios.sh"

fixture_bed_scheduler_clock_advanced=0
fixture_bed_reaper_clock_advanced=0
kill() {
  if [[ "${FUNCNAME[1]:-}" == run_fixture_bed_scenarios && -n "${FIXTURE_BED_SCHEDULER_READY_FILE:-}" &&
      $fixture_bed_scheduler_clock_advanced -eq 0 && -s "${FIXTURE_BED_GRANDCHILD_PID_FILE:?}" ]]; then
    [[ ${#live_deadlines[@]} -eq 1 ]] \
      || { echo "controlled fixture-bed scheduler expected one live deadline" >&2; return 1; }
    printf '%s %s\n' "$SECONDS" "${live_deadlines[0]}" >"$FIXTURE_BED_SCHEDULER_READY_FILE"
    SECONDS=${live_deadlines[0]}
    fixture_bed_scheduler_clock_advanced=1
  fi
  builtin kill "$@"
}
sleep() {
  if [[ "${FUNCNAME[1]:-}" == fixture_bed_reap_group && $fixture_bed_reaper_clock_advanced -eq 0 ]]; then
    [[ ${deadline:-} =~ ^[0-9]+$ ]] \
      || { echo "controlled fixture-bed reaper found no initialized TERM deadline" >&2; return 1; }
    SECONDS=$deadline
    fixture_bed_reaper_clock_advanced=1
    return 0
  fi
  command sleep "$@"
}

fixture_bed_child=0
fixture_scenario=
if fixture_scenario=$(harness_fixture_bed_child_scenario fixture-bed-inner "$@"); then
  fixture_bed_child=1
else
  fixture_bed_child_rc=$?
  [[ $fixture_bed_child_rc -eq 1 ]] || exit "$fixture_bed_child_rc"
fi
unset METASYSTEM_FIXTURE_SCENARIO
if (( ! fixture_bed_child )); then
  run_fixture_bed_scenarios fixture-bed-inner "fixture-bed inner scenario passed" \
    "$0" ${FIXTURE_BED_INNER_SCENARIOS:?}
fi

case "$fixture_scenario" in
  print-scale)
    printf '%s\n' "$METASYSTEM_FIXTURE_CAP_SCALE_MILLI"
    ;;
  hang)
    harness_fixture_bed_leg hang
    trap 'exit 143' TERM
    printf '%s\n' "$$" >"${FIXTURE_BED_CHILD_PID_FILE:?}.next"
    mv "$FIXTURE_BED_CHILD_PID_FILE.next" "$FIXTURE_BED_CHILD_PID_FILE"
    bash -c 'trap "" TERM
      exec 3<"$METASYSTEM_FIXTURE_LEASH"
      printf "%s\n" "$$" >"$FIXTURE_BED_GRANDCHILD_PID_FILE.next"
      mv "$FIXTURE_BED_GRANDCHILD_PID_FILE.next" "$FIXTURE_BED_GRANDCHILD_PID_FILE"
      [ -z "${FIXTURE_BED_READY_FD:-}" ] || printf "ready\n" >&"$FIXTURE_BED_READY_FD"
      read -r _ <&3' \
      bash "METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER" &
    exec 3<"$METASYSTEM_FIXTURE_LEASH"
    read -r _ <&3
    ;;
  json-read-fails | exit-fails) ;;
  pass)
    harness_fixture_bed_leg passing-leg
    ;;
  fail-one)
    harness_fixture_bed_leg first-failing-leg
    false
    ;;
  fail-two)
    harness_fixture_bed_leg second-failing-leg
    false
    ;;
  *)
    echo "fixture-bed inner scenario is unknown: $fixture_scenario" >&2
    exit 64
    ;;
esac

# Keep the failing substitution in the same top-level if compound used by
# converted beds. The failure guard must not invent a Bash 3.2 source line.
if [[ "$fixture_scenario" == json-read-fails ]]; then
  harness_fixture_bed_leg read-broken-json
  read_fixture_json() { [[ "$1" == '{}' ]]; }
  json_value=$(read_fixture_json '{broken')
  printf '%s\n' "$json_value"
fi

if [[ "$fixture_scenario" == exit-fails ]]; then
  harness_fixture_bed_leg explicit-exit
  exit 7
fi
`

// fixtureBedOwnerScript sources fixture-budget.sh as a fixture owner and
// starts one child tagged with the owner's fixture key. The child publishes
// its pid on descriptor 5 after it holds the leash (or, with the leash
// disabled, blocks on descriptor 6); the owner then records the child as held
// and publishes the record file on descriptor 5.
const fixtureBedOwnerScript = `set -euo pipefail
source "$1/scripts/agents/fixture-budget.sh"
harness_fixture_owner "$1"
harness_fixture_key "$2"
if [[ "$3" == leashed ]]; then
  METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" \
    bash -c 'exec 3<"$METASYSTEM_FIXTURE_LEASH"; printf "%s\n" "$$" >&5; read -r _ <&3' "$harness_fixture_tag" 9>&- &
else
  METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" METASYSTEM_FIXTURE_LEASH= \
    bash -c 'printf "%s\n" "$$" >&5; read -r _ <&6 || :' "$harness_fixture_tag" 9>&- &
fi
child_pid=$!
harness_fixture_hold_pid "$child_pid"
printf 'held %s\n' "$harness_fixture_record_file" >&5
wait "$child_pid"
`

// runFixtureBedEngineStub serves the engine commands the fixture libraries
// call. Identity answers come from the real identity package; the custodian
// performs only its launch contract (session leader, ready handshake, alive
// until its owner exits) and records how it was launched. Custodian reaping decisions are
// the identity package's and are tested there.
func runFixtureBedEngineStub(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintf(stderr, "fixture bed engine stub: command is required: %q\n", args)
		return 2
	}
	command, rest := args[0]+" "+args[1], args[2:]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	pid := flags.Int64("pid", 0, "process id")
	owner := flags.String("owner", "", "exact owner reference")
	test := flags.String("test", "", "fixture scenario")
	logPath := flags.String("log", "", "custodian log")
	flags.String("key", "", "fixture key")
	flags.String("root", "", "metasystem root")
	flags.Bool("reap", false, "reap survivors")
	switch command {
	case "util now-ns":
		fmt.Fprintln(stdout, time.Now().UnixNano())
		return 0
	case "proc census", "proc fixture-survivors":
		return 0
	case "proc default-signals":
		if len(rest) > 0 && rest[0] == "--" {
			rest = rest[1:]
		}
		if len(rest) == 0 {
			return 2
		}
		path, err := exec.LookPath(rest[0])
		if err != nil {
			fmt.Fprintln(stderr, "fixture bed engine stub:", err)
			return 127
		}
		signal.Notify(make(chan os.Signal, 3), syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP)
		fmt.Fprintln(stderr, "fixture bed engine stub: exec:", syscall.Exec(path, rest, os.Environ()))
		return 126
	}
	if flags.Parse(rest) != nil || flags.NArg() != 0 {
		return 2
	}
	switch command {
	case "proc ref":
		exact, state, err := identity.KernelProber{}.Probe(*pid)
		if err != nil || state != identity.Alive {
			return 1
		}
		encoded, err := identity.EncodeRef(exact.Ref())
		if err != nil {
			return 1
		}
		fmt.Fprintln(stdout, encoded)
		return 0
	case "proc probe":
		exact, state, _ := identity.KernelProber{}.Probe(*pid)
		fields := []string{fmt.Sprintf(`"liveness":%q`, state.String()), fmt.Sprintf(`"pid":%d`, *pid)}
		if state == identity.Alive {
			fields = append(fields, fmt.Sprintf(`"zombie":%t`, exact.Zombie))
			if leader, err := unix.Getsid(int(*pid)); err == nil {
				fields = append(fields, fmt.Sprintf(`"sessionLeaderPid":%d`, leader))
			}
		}
		fmt.Fprintf(stdout, "{%s}\n", strings.Join(fields, ","))
		return 0
	case "proc fixture-key":
		ownerRef, err := identity.ParseRef(*owner)
		if err != nil || *test == "" {
			return 2
		}
		nonce := make([]byte, 4)
		if _, err := rand.Read(nonce); err != nil {
			return 1
		}
		encoded, err := identity.EncodeKey(identity.FixtureKey{Owner: ownerRef, Test: *test, Nonce: hex.EncodeToString(nonce)})
		if err != nil {
			return 2
		}
		fmt.Fprintln(stdout, encoded)
		return 0
	case "proc custodian":
		if record := os.Getenv("FIXTURE_BED_CUSTODIAN_RECORD"); record != "" {
			launch := fmt.Sprintf("owner=%s\nlog=%s\nrecords=%s\nleash=%s\nenv-owner=%s\n", *owner, *logPath,
				os.Getenv(identity.FixtureCustodianRecordsEnv), os.Getenv(identity.FixtureCustodianLeashEnv),
				os.Getenv(identity.FixtureCustodianOwnerEnv))
			if err := os.WriteFile(record, []byte(launch), 0o600); err != nil {
				return 2
			}
		}
		if _, present := os.LookupEnv(identity.FixtureOwnerEnv); present {
			return 2
		}
		if _, err := syscall.Setsid(); err != nil {
			return 2
		}
		null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			return 2
		}
		for _, descriptor := range []int{0, 1, 2} {
			if unix.Dup2(int(null.Fd()), descriptor) != nil {
				return 2
			}
		}
		ownerRef, err := identity.ParseRef(*owner)
		if err != nil {
			return 2
		}
		ready := os.NewFile(4, "fixture-custodian-ready")
		_, _ = io.WriteString(ready, "ready\n")
		_ = ready.Close()
		// The stub keeps the launch contract until its owner exits. It waits
		// on the owner's kernel exit event rather than on end of file at the
		// leash FIFO, which Darwin does not reliably deliver to every reader.
		waitFixtureBedOwnerExit(int(ownerRef.Pid))
		return 0
	}
	fmt.Fprintf(stderr, "fixture bed engine stub: unsupported command %q\n", args)
	return 2
}

type fixtureBedHarness struct {
	dir    string
	root   string
	inner  string
	engine string
}

func newFixtureBedHarness(t *testing.T) *fixtureBedHarness {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate fixture bed test source")
	}
	sourceRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	if override := os.Getenv("FIXTURE_SOURCE_ROOT"); override != "" {
		sourceRoot = override
	}
	engine, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	harness := &fixtureBedHarness{dir: dir, root: filepath.Join(dir, "root"), inner: filepath.Join(dir, "inner-bed.sh"), engine: engine}
	for _, name := range []string{"home", "tmp", "go", filepath.Join("root", "scripts", "agents")} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, library := range []string{"fixture-budget.sh", "fixture-bed-scenarios.sh"} {
		contents, err := os.ReadFile(filepath.Join(sourceRoot, "scripts", "agents", library))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(harness.root, "scripts", "agents", library), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := testexec.WriteFile(harness.inner, []byte(fixtureBedInnerBed), 0o755); err != nil {
		t.Fatal(err)
	}
	return harness
}

// env is the complete environment of a bed: nothing from the test process
// leaks in except PATH.
func (harness *fixtureBedHarness) env(extra ...string) []string {
	return append([]string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + filepath.Join(harness.dir, "home"),
		"TMPDIR=" + filepath.Join(harness.dir, "tmp"),
		"GOCACHE=" + filepath.Join(harness.dir, "go"),
		"GOMODCACHE=" + filepath.Join(harness.dir, "go"),
		"GOPATH=" + filepath.Join(harness.dir, "go"),
		"METASYSTEM_BIN=" + harness.engine,
		"METASYSTEM_TEST_WORKERS=1",
		"FIXTURE_BED_SOURCE_ROOT=" + harness.root,
		fixtureBedEngineStubEnv + "=1",
	}, extra...)
}

func (harness *fixtureBedHarness) path(name string) string {
	return filepath.Join(harness.dir, name)
}

// command prepares the inner bed with its output in one file, so a
// descendant that outlives the bed cannot hold a pipe the test waits on.
func (harness *fixtureBedHarness) command(t *testing.T, output string, env []string, argv ...string) *exec.Cmd {
	t.Helper()
	file, err := os.Create(output)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if len(argv) == 0 {
		argv = []string{harness.inner}
	}
	command := exec.Command(argv[0], argv[1:]...)
	command.Env, command.Stdout, command.Stderr = env, file, file
	return command
}

func fixtureBedExitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("bed did not run: %v", err)
	}
	return exit.ExitCode()
}

func readFixtureBedOutput(t *testing.T, path string) (string, []string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents), strings.Split(string(contents), "\n")
}

func readFixtureBedPid(t *testing.T, path string) int {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil || pid < 2 {
		t.Fatalf("pid file %s holds %q", path, contents)
	}
	return pid
}

func requireFixtureBedLines(t *testing.T, output string, lines []string, want ...string) {
	t.Helper()
	for _, line := range want {
		if !slices.Contains(lines, line) {
			t.Fatalf("bed output lacks line %q:\n%s", line, output)
		}
	}
}

func assertNoFixtureBedGroupSurvivor(t *testing.T, child, grandchild int) {
	t.Helper()
	if err := unix.Kill(grandchild, 0); !errors.Is(err, unix.ESRCH) {
		t.Errorf("bed left grandchild %d alive (kill -0: %v)", grandchild, err)
	}
	if err := unix.Kill(-child, 0); !errors.Is(err, unix.ESRCH) {
		t.Errorf("bed left process group %d alive (kill -0: %v)", child, err)
	}
}

func firstFixtureBedNumber(lines []string) string {
	for _, line := range lines {
		if line != "" && strings.Trim(line, "0123456789") == "" {
			return line
		}
	}
	return ""
}

// budget-standalone: a bed started with no scale in its environment
// calibrates its own cap scale before its first scenario.
func TestFixtureBedBudgetSelfInitializes(t *testing.T) {
	t.Parallel()
	harness := newFixtureBedHarness(t)
	output := harness.path("standalone.out")
	err := harness.command(t, output, harness.env("FIXTURE_BED_INNER_SCENARIOS=print-scale")).Run()
	text, lines := readFixtureBedOutput(t, output)
	if code := fixtureBedExitCode(t, err); code != 0 {
		t.Fatalf("standalone bed exited %d:\n%s", code, text)
	}
	milli, convErr := strconv.Atoi(firstFixtureBedNumber(lines))
	if convErr != nil || milli < 8000 || milli > 48000 {
		t.Fatalf("standalone budget was not self-initialized to 8000..48000: %q\n%s", firstFixtureBedNumber(lines), text)
	}
	requireFixtureBedLines(t, text, lines, "fixture-bed inner scenario passed")
}

// budget-inherited: an operator scale and a parent's already-resolved scale
// both reach the scenario unchanged.
func TestFixtureBedBudgetInheritsScale(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		env  []string
	}{
		{name: "operator", env: []string{"METASYSTEM_FIXTURE_CAP_SCALE=3"}},
		{name: "parent", env: []string{"METASYSTEM_FIXTURE_CAP_SCALE=3", "METASYSTEM_FIXTURE_CAP_SCALE_MILLI=3000"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			harness := newFixtureBedHarness(t)
			output := harness.path(test.name + ".out")
			env := harness.env(append(test.env, "FIXTURE_BED_INNER_SCENARIOS=print-scale")...)
			err := harness.command(t, output, env).Run()
			text, lines := readFixtureBedOutput(t, output)
			if code := fixtureBedExitCode(t, err); code != 0 {
				t.Fatalf("%s bed exited %d:\n%s", test.name, code, text)
			}
			if got := firstFixtureBedNumber(lines); got != "3000" {
				t.Fatalf("%s budget scale = %q, want 3000\n%s", test.name, got, text)
			}
		})
	}
}

// ceiling-reaps-group and ceiling-expiry-mutation: a scenario still running
// at its ceiling is named, its whole process group is reaped (TERM, then
// KILL for the TERM-ignoring grandchild), and the bed fails with status 124.
// The cap is the 120-second maximum and the scheduler record proves the
// controlled clock was still before the deadline when it advanced, so the
// expiry is the library's reaction to its own clock, never a wall timeout.
func TestFixtureBedCeilingReapsGroup(t *testing.T) {
	t.Parallel()
	harness := newFixtureBedHarness(t)
	output, childFile, grandchildFile, schedulerFile := harness.path("ceiling.out"), harness.path("child.pid"),
		harness.path("grandchild.pid"), harness.path("scheduler-ready")
	env := harness.env("METASYSTEM_FIXTURE_CAP_SCALE=1", "METASYSTEM_FIXTURE_CAP_SCALE_MILLI=1000",
		"METASYSTEM_BED_SCENARIO_FIXTURE_TIMEOUT_SEC=120", "FIXTURE_BED_INNER_SCENARIOS=hang",
		"FIXTURE_BED_CHILD_PID_FILE="+childFile, "FIXTURE_BED_GRANDCHILD_PID_FILE="+grandchildFile,
		"FIXTURE_BED_SCHEDULER_READY_FILE="+schedulerFile)
	err := harness.command(t, output, env).Run()
	text, lines := readFixtureBedOutput(t, output)
	if code := fixtureBedExitCode(t, err); code != 1 {
		t.Fatalf("ceiling bed exited %d, want 1:\n%s", code, text)
	}
	child, grandchild := readFixtureBedPid(t, childFile), readFixtureBedPid(t, grandchildFile)
	scheduler, readErr := os.ReadFile(schedulerFile)
	if readErr != nil {
		t.Fatalf("the controlled scheduler never advanced after both descendants were ready: %v\n%s", readErr, text)
	}
	var before, deadline int
	if _, err := fmt.Sscanf(string(scheduler), "%d %d", &before, &deadline); err != nil || before >= deadline {
		t.Fatalf("scheduler record %q: the deadline was not still ahead when the controlled clock advanced (%v)", scheduler, err)
	}
	ceiling := regexp.MustCompile(`^fixture-bed-inner fixture scenario exceeded its ceiling: hang \(elapsed ([0-9]+)s, scaled cap 120s\)$`)
	matched := false
	for _, line := range lines {
		if match := ceiling.FindStringSubmatch(line); match != nil {
			elapsed, _ := strconv.Atoi(match[1])
			matched = elapsed >= 120
		}
	}
	if !matched {
		t.Fatalf("bed did not report the ceiling expiry at the controlled deadline:\n%s", text)
	}
	if !strings.Contains(text, "fixture-bed-inner fixture scenario hang failed while serving leg hang with status 124") {
		t.Fatalf("ceiling failure does not name its leg and status 124:\n%s", text)
	}
	group := strconv.Itoa(child)
	requireFixtureBedLines(t, text, lines, "- hang (rc=124)", "group "+group+": TERM sent",
		"group "+group+": alive after 5s grace; KILL sent", "group "+group+": empty")
	assertNoFixtureBedGroupSurvivor(t, child, grandchild)
}

// signal-reaps-group and the hang-leash INT leg: a bed interrupted while a
// scenario hangs exits with the signal's status and its cleanup empties the
// scenario's process group. The INT leg starts the bed through the engine's
// default-signals launcher, as a caller that ignores INT must.
func TestFixtureBedSignalReapsGroup(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		signal   syscall.Signal
		status   int
		launcher bool
	}{
		{name: "TERM", signal: syscall.SIGTERM, status: 143},
		{name: "INT", signal: syscall.SIGINT, status: 130, launcher: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			harness := newFixtureBedHarness(t)
			output, childFile, grandchildFile := harness.path("signal.out"), harness.path("child.pid"), harness.path("grandchild.pid")
			env := harness.env("METASYSTEM_FIXTURE_CAP_SCALE=1", "METASYSTEM_FIXTURE_CAP_SCALE_MILLI=1000",
				"FIXTURE_BED_INNER_SCENARIOS=hang", "FIXTURE_BED_READY_FD=5",
				"FIXTURE_BED_CHILD_PID_FILE="+childFile, "FIXTURE_BED_GRANDCHILD_PID_FILE="+grandchildFile)
			argv := []string{harness.inner}
			if test.launcher {
				argv = []string{harness.engine, "proc", "default-signals", "--", harness.inner}
			}
			command := harness.command(t, output, env, argv...)
			readyReader, readyWriter, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer readyReader.Close()
			command.ExtraFiles = []*os.File{nil, nil, readyWriter}
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			_ = readyWriter.Close()
			exited := make(chan error, 1)
			go func() { exited <- command.Wait() }()
			ready := make(chan error, 1)
			go func() {
				line, err := bufio.NewReader(readyReader).ReadString('\n')
				if err == nil && line != "ready\n" {
					err = fmt.Errorf("ready line %q", line)
				}
				ready <- err
			}()
			select {
			case err := <-ready:
				if err != nil {
					_ = command.Process.Kill()
					<-exited
					text, _ := readFixtureBedOutput(t, output)
					t.Fatalf("hang grandchild did not publish readiness: %v\n%s", err, text)
				}
			case err := <-exited:
				text, _ := readFixtureBedOutput(t, output)
				t.Fatalf("bed exited before its hang scenario was ready: %v\n%s", err, text)
			}
			child, grandchild := readFixtureBedPid(t, childFile), readFixtureBedPid(t, grandchildFile)
			if err := command.Process.Signal(test.signal); err != nil {
				t.Fatal(err)
			}
			waitErr := <-exited
			text, lines := readFixtureBedOutput(t, output)
			if code := fixtureBedExitCode(t, waitErr); code != test.status {
				t.Fatalf("%s bed exited %d, want %d:\n%s", test.name, code, test.status, text)
			}
			group := strconv.Itoa(child)
			requireFixtureBedLines(t, text, lines, "group "+group+": TERM sent",
				"group "+group+": alive after 5s grace; KILL sent", "group "+group+": empty")
			assertNoFixtureBedGroupSurvivor(t, child, grandchild)
		})
	}
}

// hang-leash, leash leg: when a fixture owner dies, the leash it holds
// releases a leashed child without anyone signaling it. The descriptor 5
// pipe reaches end of file only when the owner, its custodian and the child
// have all exited, and the stub custodian never signals.
func TestFixtureBedLeashReleasesChildOnOwnerDeath(t *testing.T) {
	t.Parallel()
	harness := newFixtureBedHarness(t)
	owner, ready := startFixtureBedOwner(t, harness, "hang-leash-fast", "leashed", nil)
	child, _ := readFixtureBedOwnerReady(t, ready)
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = owner.Wait()
	if _, err := io.Copy(io.Discard, ready); err != nil {
		t.Fatal(err)
	}
	// End of file says every holder closed descriptor 5; a child closes its
	// descriptors on the way out before the kernel marks it exited, so the
	// probe waits for the child's own exit event first. A child that closed
	// the leash and kept running would never deliver it.
	waitFixtureBedOwnerExit(child)
	exact, state, err := identity.KernelProber{}.Probe(int64(child))
	if err == nil && state == identity.Alive && !exact.Zombie {
		t.Fatalf("leashed child %d is still running after its owner died", child)
	}
}

// hang-leash, disabled leg: with the leash removed, the owner's hold record
// is what leaves the custodian as the safety. The custodian is launched for
// the owner's exact ref with the record file, and the record file holds the
// child's exact ref. That the custodian kills such a running orphan is
// identity.TestCustodianKillsARunningOrphanWithoutAHeldLeashAfterSettledScans.
func TestFixtureBedOwnerRecordsHeldChildForCustodian(t *testing.T) {
	t.Parallel()
	harness := newFixtureBedHarness(t)
	releaseReader, releaseWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer releaseWriter.Close()
	launch := harness.path("custodian.launch")
	owner, ready := startFixtureBedOwner(t, harness, "hang-leash-disabled", "unleashed", releaseReader,
		"FIXTURE_BED_CUSTODIAN_RECORD="+launch)
	_ = releaseReader.Close()
	child, records := readFixtureBedOwnerReady(t, ready)
	prober := identity.KernelProber{}
	ownerRef, childRef := fixtureBedRef(t, prober, owner.Process.Pid), fixtureBedRef(t, prober, child)
	held, err := os.ReadFile(records)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Split(string(held), "\n"), "+"+childRef) {
		t.Fatalf("custodian record file %s = %q, want the child's exact ref %s", records, held, childRef)
	}
	launched, err := os.ReadFile(launch)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"owner=" + ownerRef, "env-owner=" + ownerRef, "records=" + records} {
		if !slices.Contains(strings.Split(string(launched), "\n"), want) {
			t.Fatalf("custodian launch %q lacks %q", launched, want)
		}
	}
	if strings.Contains(string(launched), "\nleash=\n") {
		t.Fatalf("custodian launch %q has no leash", launched)
	}
	_ = releaseWriter.Close()
	if code := fixtureBedExitCode(t, owner.Wait()); code != 0 {
		t.Fatalf("owner exited %d after its child was released", code)
	}
	if _, err := io.Copy(io.Discard, ready); err != nil {
		t.Fatal(err)
	}
}

func startFixtureBedOwner(t *testing.T, harness *fixtureBedHarness, key, mode string, release *os.File, extra ...string) (*exec.Cmd, *os.File) {
	t.Helper()
	readyReader, readyWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = readyReader.Close() })
	owner := exec.Command("bash", "-c", fixtureBedOwnerScript, "fixture-owner", harness.root, key, mode)
	owner.Env = harness.env(extra...)
	owner.Stderr = io.Discard
	owner.ExtraFiles = []*os.File{nil, nil, readyWriter, release}
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	_ = readyWriter.Close()
	t.Cleanup(func() {
		if owner.ProcessState == nil {
			_ = owner.Process.Kill()
			_ = owner.Wait()
		}
	})
	return owner, readyReader
}

// readFixtureBedOwnerReady reads the child's pid and the owner's record file
// path, in whichever order the two processes wrote them.
func readFixtureBedOwnerReady(t *testing.T, ready *os.File) (int, string) {
	t.Helper()
	reader := bufio.NewReader(ready)
	child, records := 0, ""
	for child == 0 || records == "" {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("fixture owner did not publish its child and record file: %v (child=%d records=%q)", err, child, records)
		}
		line = strings.TrimSuffix(line, "\n")
		if path, ok := strings.CutPrefix(line, "held "); ok {
			records = path
		} else if pid, err := strconv.Atoi(line); err == nil {
			child = pid
		} else {
			t.Fatalf("unexpected fixture owner line %q", line)
		}
	}
	return child, records
}

func fixtureBedRef(t *testing.T, prober identity.Prober, pid int) string {
	t.Helper()
	exact, state, err := prober.Probe(int64(pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe %d: %v %v", pid, state, err)
	}
	encoded, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

// command-substitution-failure: a failing command substitution and an
// explicit exit each name their scenario, leg and status, and neither claims
// a Bash 3.2 source line.
func TestFixtureBedCommandSubstitutionFailureNamesLeg(t *testing.T) {
	t.Parallel()
	harness := newFixtureBedHarness(t)
	output := harness.path("command-substitution.out")
	env := harness.env("METASYSTEM_FIXTURE_CAP_SCALE=1", "METASYSTEM_FIXTURE_CAP_SCALE_MILLI=1000",
		"FIXTURE_BED_INNER_SCENARIOS=json-read-fails exit-fails")
	err := harness.command(t, output, env).Run()
	text, _ := readFixtureBedOutput(t, output)
	if code := fixtureBedExitCode(t, err); code != 1 {
		t.Fatalf("bed exited %d, want 1:\n%s", code, text)
	}
	for _, want := range []string{
		"fixture-bed-inner fixture scenario json-read-fails failed while serving leg read-broken-json with status 1",
		"fixture-bed-inner fixture scenario exit-fails failed while serving leg explicit-exit with status 7",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("bed output lacks %q:\n%s", want, text)
		}
	}
	for _, refused := range []string{"failed while serving leg read-broken-json at line ", "failed while serving leg explicit-exit at line "} {
		if strings.Contains(text, refused) {
			t.Fatalf("bed output claims an unproven Bash 3.2 line %q:\n%s", refused, text)
		}
	}
}

// collects-every-failure: a bed runs every scenario past a red one and fails
// once at the end naming each failure.
func TestFixtureBedCollectsEveryFailure(t *testing.T) {
	t.Parallel()
	harness := newFixtureBedHarness(t)
	output := harness.path("collects-every-failure.out")
	env := harness.env("METASYSTEM_FIXTURE_CAP_SCALE=1", "METASYSTEM_FIXTURE_CAP_SCALE_MILLI=1000",
		"FIXTURE_BED_INNER_SCENARIOS=fail-one pass fail-two")
	err := harness.command(t, output, env).Run()
	text, lines := readFixtureBedOutput(t, output)
	if code := fixtureBedExitCode(t, err); code != 1 {
		t.Fatalf("bed exited %d, want 1:\n%s", code, text)
	}
	requireFixtureBedLines(t, text, lines, "- fail-one (rc=1)", "- fail-two (rc=1)")
	if !regexp.MustCompile(`(?m)^fixture-bed-inner fixture scenario passed: pass \([0-9]+s\)$`).MatchString(text) {
		t.Fatalf("bed did not run its passing scenario:\n%s", text)
	}
}
