package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
)

var (
	appFixtureOnce   sync.Once
	appFixtureBinary string
	appEngineBinary  string
	appFixtureError  error
)

// appFixtureApp builds the launch contract's own application fixture: a
// small server that answers a health URL, can be told to ignore TERM, and
// can write a readiness line to its log.
func appFixtureApp(t *testing.T) string {
	t.Helper()
	appFixtureOnce.Do(func() {
		dir, err := os.MkdirTemp("", "app-verb-fixture-")
		if err != nil {
			appFixtureError = err
			return
		}
		appFixtureBinary = filepath.Join(dir, "fixtureapp")
		build := exec.Command("go", "build", "-o", appFixtureBinary, "../../internal/applaunch/testdata/fixtureapp")
		if out, err := build.CombinedOutput(); err != nil {
			appFixtureError = fmt.Errorf("build fixture application: %v\n%s", err, out)
			return
		}
		// A verb's start launches the engine's own `app serve`. Under `go
		// test` this process is the test binary, so the tests supervise with
		// a real engine built for the purpose.
		appEngineBinary = filepath.Join(dir, "metasystem")
		engine := exec.Command("go", "build", "-o", appEngineBinary, ".")
		if out, err := engine.CombinedOutput(); err != nil {
			appFixtureError = fmt.Errorf("build the engine: %v\n%s", err, out)
		}
	})
	if appFixtureError != nil {
		t.Fatal(appFixtureError)
	}
	previous := appEngine
	appEngine = func() (string, error) { return appEngineBinary, nil }
	t.Cleanup(func() { appEngine = previous })
	return appFixtureBinary
}

// appBed is one fixture project: a git checkout with an installation, a
// launch contract and whatever the test wants the application to do.
type appBed struct {
	t            *testing.T
	root         string
	installation string
	app          string
	reaping      map[int]bool
	// delivery replaces the engine-verb owner, so that a test can see the
	// argument vector a verb hands the testing runner and answer for it.
	delivery *intentDeliveryOwners
}

func newAppBed(t *testing.T, contract map[string]any) *appBed {
	t.Helper()
	app := appFixtureApp(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "project")
	installation := filepath.Join(root, "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The template layout this repository itself has: the installation is the
	// nested metasystem directory and keeps its own state beneath it.
	if err := os.WriteFile(filepath.Join(root, "development", "metasystem-design.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("metasystem/artifacts/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := "metasystem.version=1\nmetasystem.engine-delivery=source\ntesting.contract=testing.json\n"
	if contract != nil {
		conf += "launch.contract=launch.json\n"
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	if contract != nil {
		contract["schemaVersion"] = 1
		body, err := json.MarshalIndent(contract, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(installation, "launch.json"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bed := &appBed{t: t, root: root, installation: installation, app: app, reaping: map[int]bool{}}
	bed.git("init", "--quiet", "--initial-branch=main")
	bed.git("config", "user.email", "fixture@invalid")
	bed.git("config", "user.name", "Fixture")
	bed.git("add", "-A")
	bed.git("commit", "--quiet", "-m", "the fixture project")
	t.Cleanup(func() { bed.run("app", "stop", "--clean") })
	return bed
}

func (b *appBed) git(args ...string) {
	b.t.Helper()
	command := exec.Command("git", args...)
	command.Dir = b.root
	if out, err := command.CombinedOutput(); err != nil {
		b.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// run drives one public app verb exactly as a person's terminal would.
func (b *appBed) run(args ...string) (int, string) {
	b.t.Helper()
	command, ok := findIntentAction(args[0], args[1])
	if !ok {
		b.t.Fatalf("no public verb %s %s", args[0], args[1])
	}
	rest := append(append([]string(nil), args[2:]...), "--repo", b.root)
	var stdout, stderr bytes.Buffer
	owners := defaultIntentOwners()
	if b.delivery != nil {
		owners.delivery = b.delivery
	}
	code := runIntentIn(command, rest, &stdout, &stderr, b.root, owners)
	b.reapSupervisors()
	return code, stdout.String() + stderr.String()
}

// reapSupervisors stands in for init. The engine's own start is a short-lived
// process, so a supervisor it launched is reparented and reaped the moment it
// exits; a test process outlives its launches, and an unreaped supervisor
// stays in the process table as a zombie that reads as a living owner.
func (b *appBed) reapSupervisors() {
	keys, err := applaunch.Keys(b.installation)
	if err != nil {
		return
	}
	for _, key := range keys {
		record, err := applaunch.ReadRecord(b.installation, key)
		if err != nil {
			continue
		}
		ref, err := record.SupervisorRef()
		if err != nil || b.reaping[int(ref.Pid)] {
			continue
		}
		b.reaping[int(ref.Pid)] = true
		go applaunch.Reap(int(ref.Pid))
	}
}

func (b *appBed) runJSON(args ...string) (int, map[string]any) {
	b.t.Helper()
	code, out := b.run(append(args, "--json")...)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		b.t.Fatalf("the result must be one JSON object: %v\n%s", err, out)
	}
	return code, parsed
}

func appFreePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func appHTTPContract(app, address string, extra ...string) map[string]any {
	return map[string]any{
		"name":      "fixture",
		"address":   address,
		"portRange": portRangeBeside(address),
		"start":     map[string]any{"argv": append([]string{app, "--listen", "${address}"}, extra...)},
		"ready":     map[string]any{"kind": "http", "url": "http://${address}/-/health"},
		"readyMs":   20000,
		"stopMs":    5000,
	}
}

// portRangeBeside is a candidate range that cannot overlap the standing
// address, which the contract check would refuse.
func portRangeBeside(address string) string {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return ""
	}
	number := 0
	for _, digit := range port {
		number = number*10 + int(digit-'0')
	}
	if number < 2000 {
		return ""
	}
	return strconv.Itoa(number-200) + "-" + strconv.Itoa(number-1)
}

// A project with no launch contract has no app verbs, and the refusal says
// which file to write.
func TestAppVerbsRefuseWithoutAContract(t *testing.T) {
	bed := newAppBed(t, nil)
	for _, verb := range []string{"status", "start", "stop", "restart", "log", "reset", "check"} {
		code, out := bed.run("app", verb)
		if code == 0 {
			t.Fatalf("app %s must be refused without a contract:\n%s", verb, out)
		}
		if !strings.Contains(out, "launch.json") || !strings.Contains(out, "launch.contract=launch.json") {
			t.Fatalf("app %s must name the file to write:\n%s", verb, out)
		}
	}
}

// Start waits for readiness and records; status says liveness and readiness
// separately and carries the data word; log reads what the engine captured;
// stop proves death and the next status says stopped.
func TestAppStartStatusLogAndStop(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address, "--ready-after", "400ms"))
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	code, out := bed.run("app", "status")
	if code != 0 || !strings.Contains(out, "state: running") || !strings.Contains(out, "readiness: answering") {
		t.Fatalf("app status must say running and answering:\n%s", out)
	}
	if !strings.Contains(out, "data: shared with the standing run") {
		t.Fatalf("a contract with no prepare says the data word:\n%s", out)
	}
	if code, out := bed.run("app", "log"); code != 0 || !strings.Contains(out, "fixtureapp: listening on") {
		t.Fatalf("app log must read the engine's capture:\n%s", out)
	}
	code, out = bed.run("app", "stop")
	if code != 0 {
		t.Fatalf("app stop: %d\n%s", code, out)
	}
	if !strings.Contains(out, "every recorded process is dead") {
		t.Fatalf("stop must prove death in words:\n%s", out)
	}
	if code, out := bed.run("app", "status"); code != 0 || !strings.Contains(out, "state: stopped") {
		t.Fatalf("after a proven stop the next status says stopped:\n%s", out)
	}
	if answered(address) {
		t.Fatal("the application still answers after a proven stop")
	}
}

// A second start rejoins the run that is live rather than starting a second
// application, and still waits for readiness before it says so.
func TestAppSecondStartRejoins(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	code, result := bed.runJSON("app", "start")
	if code != 0 {
		t.Fatalf("a second start must rejoin: %v", result)
	}
	if summary, _ := result["summary"].(string); !strings.Contains(summary, "already running at "+address) {
		t.Fatalf("a rejoin says so: %q", summary)
	}
	data, _ := result["data"].(map[string]any)
	if readiness, _ := data["readiness"].(string); readiness != "answering" {
		t.Fatalf("a rejoin still waits for readiness: %v", data)
	}
}

// --at and --goal are two spellings of one thing.
func TestAppAtAndGoalAreOneThing(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	code, out := bed.run("app", "status", "--at", "main", "--goal", "g1")
	if code == 0 || !strings.Contains(out, "use one of them") {
		t.Fatalf("naming both must be refused, not guessed:\n%s", out)
	}
}

// A run at a commit gets a tree, an address and a state root of its own, and
// the standing run's record is never touched by it.
func TestAppRunAtARefRunsBesideTheStandingRun(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("standing start: %d\n%s", code, out)
	}
	t.Cleanup(func() { bed.run("app", "stop", "--at", "main", "--clean") })
	code, candidate := bed.runJSON("app", "start", "--at", "main")
	if code != 0 {
		t.Fatalf("a run at a ref must start beside the standing run: %v", candidate)
	}
	candidateData, _ := candidate["data"].(map[string]any)
	candidateAddress, _ := candidateData["address"].(string)
	if candidateAddress == "" || candidateAddress == address {
		t.Fatalf("a candidate takes an address of its own, got %q beside %q", candidateAddress, address)
	}
	if !answered(candidateAddress) || !answered(address) {
		t.Fatal("both runs must be answering")
	}
	_, standing := bed.runJSON("app", "status")
	standingData, _ := standing["data"].(map[string]any)
	if got, _ := standingData["address"].(string); got != address {
		t.Fatalf("the standing run's record was touched: %v", standingData)
	}
	if got, _ := standingData["state"].(string); got != "running" {
		t.Fatalf("the standing run must be untouched and running: %v", standingData)
	}
	if got, _ := candidateData["commit"].(string); got == "" {
		t.Fatalf("a run at a ref records the commit it runs: %v", candidateData)
	}
}

// reset re-runs prepare; a contract with no prepare says reset is a restart.
func TestAppResetRunsPrepareAgain(t *testing.T) {
	address := appFreePort(t)
	counter := filepath.Join(t.TempDir(), "prepared")
	contract := appHTTPContract(appFixtureApp(t), address)
	contract["prepare"] = map[string]any{"argv": []string{"sh", "-c", "echo run >> " + counter}}
	contract["data"] = "own"
	bed := newAppBed(t, contract)
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	if runs := countLines(t, counter); runs != 1 {
		t.Fatalf("prepare runs once before the first start, ran %d time(s)", runs)
	}
	if code, out := bed.run("app", "reset"); code != 0 {
		t.Fatalf("app reset: %d\n%s", code, out)
	}
	if runs := countLines(t, counter); runs != 2 {
		t.Fatalf("reset re-runs prepare, ran %d time(s)", runs)
	}
	code, out := bed.run("app", "status")
	if code != 0 || !strings.Contains(out, "data: own, made by prepare") {
		t.Fatalf("a contract with prepare gives the run its own data:\n%s", out)
	}
}

func TestAppResetWithoutPrepareIsARestart(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	code, out := bed.run("app", "reset")
	if code != 0 || !strings.Contains(out, "no prepare declared: reset is a restart") {
		t.Fatalf("reset must say what it did instead:\n%s", out)
	}
}

// check answers that none is declared, and refuses a run that is not
// answering rather than proving nothing.
func TestAppCheckAnswersAndRefuses(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	code, out := bed.run("app", "check")
	if code != 0 || !strings.Contains(out, "no check is declared") {
		t.Fatalf("a contract with no check answers so:\n%s", out)
	}

	withCheck := appHTTPContract(appFixtureApp(t), appFreePort(t))
	withCheck["check"] = "app-smoke"
	checked := newAppBed(t, withCheck)
	code, out = checked.run("app", "check")
	if code == 0 || !strings.Contains(out, "not live and answering") {
		t.Fatalf("a check against a run that is not live must be refused in words:\n%s", out)
	}
}

// The check bridges to the testing contract's own runner: the one form that
// runs a group by name, executed freshly, with the run's address as the
// runner's declared input.
func TestAppCheckBridgesToTheTestingRunner(t *testing.T) {
	argv := appCheckArgv("/installation", "app-smoke", "127.0.0.1:7981")
	joined := strings.Join(argv, " ")
	for _, want := range []string{"test run", "--root /installation", "--mode canary",
		"--groups app-smoke", "--no-reuse", "--app-address 127.0.0.1:7981"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the check must run %q; got %q", want, joined)
		}
	}
}

// An application that exits by itself leaves an ended record; log still
// reads its log, and the next start closes that run before it begins.
func TestAppRunThatEndsByItself(t *testing.T) {
	bed := newAppBed(t, map[string]any{
		"name":   "fixture",
		"start":  map[string]any{"argv": []string{appFixtureApp(t), "--no-listen", "--exit-after", "600ms", "--exit-code", "4"}},
		"stopMs": 4000,
	})
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	eventuallyTrue(t, "the run to end into an ended record", func() bool {
		_, out := bed.run("app", "status")
		return strings.Contains(out, "state: ended")
	})
	_, out := bed.run("app", "status")
	if !strings.Contains(out, "exit 4") {
		t.Fatalf("the ended record carries the exit status:\n%s", out)
	}
	if code, logOut := bed.run("app", "log"); code != 0 || !strings.Contains(logOut, "fixtureapp: exiting by itself") {
		t.Fatalf("log still reads an ended run's log:\n%s", logOut)
	}
	if code, startOut := bed.run("app", "start"); code != 0 {
		t.Fatalf("the next start closes the ended run and begins again: %d\n%s", code, startOut)
	}
}

func answered(address string) bool {
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		return false
	}
	_ = connection.Close()
	return true
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return len(strings.Fields(string(data)))
}

func eventuallyTrue(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// --goal G is sugar for the goal branch's tip, and a moved tip makes the
// next start replace that run.
func TestAppGoalRunFollowsItsBranchTip(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	bed.git("branch", "goal/g1")
	t.Cleanup(func() { bed.run("app", "stop", "--goal", "g1", "--clean") })
	code, first := bed.runJSON("app", "start", "--goal", "g1")
	if code != 0 {
		t.Fatalf("a goal's run must start from its branch tip: %v", first)
	}
	firstData, _ := first["data"].(map[string]any)
	if got, _ := firstData["goal"].(string); got != "g1" {
		t.Fatalf("the record names the goal: %v", firstData)
	}
	firstCommit, _ := firstData["commit"].(string)
	if firstCommit == "" {
		t.Fatalf("the record names the commit it runs: %v", firstData)
	}
	if code, out := bed.run("app", "stop", "--goal", "g1"); code != 0 {
		t.Fatalf("app stop --goal: %d\n%s", code, out)
	}
	// The tip moves; the next start of that ref runs the new commit.
	if err := os.WriteFile(filepath.Join(bed.root, "moved.txt"), []byte("moved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git("checkout", "--quiet", "goal/g1")
	bed.git("add", "-A")
	bed.git("commit", "--quiet", "-m", "move the goal branch")
	bed.git("checkout", "--quiet", "main")
	code, second := bed.runJSON("app", "start", "--goal", "g1")
	if code != 0 {
		t.Fatalf("a moved tip must replace the run: %v", second)
	}
	secondData, _ := second["data"].(map[string]any)
	if got, _ := secondData["commit"].(string); got == firstCommit || got == "" {
		t.Fatalf("the replaced run must be at the new tip: %v", secondData)
	}
}

// --follow prints a stream, and a stream is not one JSON result.
func TestAppLogFollowRefusesJSON(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	code, out := bed.run("app", "log", "--follow", "--json")
	if code == 0 || !strings.Contains(out, "not one JSON result") {
		t.Fatalf("--follow with --json must be refused:\n%s", out)
	}
}

// The launch contract is read and validated where the settings are, with its
// declared tools resolved in the environment its commands will run in.
func TestLaunchContractReadyValidatesWithItsTools(t *testing.T) {
	absent := newAppBed(t, nil)
	if _, _, _, err := launchContractReady(absent.installation); err == nil || !errors.Is(err, errNoLaunchContract) {
		t.Fatalf("a project with no contract is not a project with a problem: %v", err)
	}

	address := appFreePort(t)
	valid := appHTTPContract(appFixtureApp(t), address)
	valid["tools"] = []any{map[string]any{"id": "git", "executable": "git", "versionArgs": []string{"--version"}}}
	bed := newAppBed(t, valid)
	path, contract, tools, err := launchContractReady(bed.installation)
	if err != nil {
		t.Fatalf("a valid contract with an available tool: %v", err)
	}
	if len(tools) != 1 || !strings.HasPrefix(tools[0].Line(), "git: ") || !strings.Contains(tools[0].Line(), "(git version") {
		t.Fatalf("an available tool is named with the executable found and its version line: %+v", tools)
	}
	if !strings.HasSuffix(path, "launch.json") || contract.Name != "fixture" {
		t.Fatalf("unexpected contract %s %+v", path, contract)
	}

	missing := appHTTPContract(appFixtureApp(t), appFreePort(t))
	missing["tools"] = []any{map[string]any{"id": "nosuchjdk", "executable": "nosuchjdk-9999"}}
	absentTool := newAppBed(t, missing)
	if _, _, _, err := launchContractReady(absentTool.installation); err == nil || !strings.Contains(err.Error(), "nosuchjdk") {
		t.Fatalf("a declared tool that is absent must be named: %v", err)
	}

	broken := newAppBed(t, map[string]any{"start": map[string]any{"argv": []string{"./app"}}, "data": "own"})
	if _, _, _, err := launchContractReady(broken.installation); err == nil ||
		!strings.Contains(err.Error(), "data: own is declared with no prepare to make it") {
		t.Fatalf("an invalid contract must name its fault: %v", err)
	}
}

// Declared tools are the start's preflight: a missing one is named before
// anything is prepared, built or started, and an available one has the
// executable found and its version line printed into the run's record.
func TestAppStartPreflightsDeclaredTools(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "prepared")
	missing := appHTTPContract(appFixtureApp(t), appFreePort(t))
	missing["prepare"] = map[string]any{"argv": []string{"sh", "-c", "echo run >> " + marker}}
	missing["tools"] = []any{map[string]any{"id": "nosuchjdk", "executable": "nosuchjdk-9999", "versionArgs": []string{"-version"}}}
	refused := newAppBed(t, missing)
	code, out := refused.run("app", "start")
	if code == 0 || !strings.Contains(out, "nosuchjdk") {
		t.Fatalf("a missing declared tool must be named and nothing started:\n%s", out)
	}
	if countLines(t, marker) != 0 {
		t.Fatal("nothing is prepared before the preflight passes")
	}
	if _, status := refused.run("app", "status"); !strings.Contains(status, "state: stopped") {
		t.Fatalf("a refused preflight starts nothing:\n%s", status)
	}

	address := appFreePort(t)
	declared := appHTTPContract(appFixtureApp(t), address)
	declared["tools"] = []any{map[string]any{"id": "git", "executable": "git", "versionArgs": []string{"--version"}}}
	bed := newAppBed(t, declared)
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	record, err := applaunch.ReadRecord(bed.installation, applaunch.StandingKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Tools) != 1 || record.Tools[0].ID != "git" || !filepath.IsAbs(record.Tools[0].Executable) ||
		!strings.HasPrefix(record.Tools[0].Version, "git version") {
		t.Fatalf("the record carries the executable found and its version line: %+v", record.Tools)
	}
	if _, status := bed.run("app", "status"); !strings.Contains(status, "tool git: "+record.Tools[0].Executable+" (git version") {
		t.Fatalf("status says which tool the run was started with:\n%s", status)
	}
}

// Every optional field left out leaves a contract that validates and verbs
// that work: a contract of start alone starts, reads, logs, restarts, resets,
// checks and stops.
func TestAppContractOfStartAloneWorksWithEveryVerb(t *testing.T) {
	bed := newAppBed(t, map[string]any{"start": map[string]any{"argv": []string{appFixtureApp(t), "--no-listen", "--ready-line", "started"}}})
	code, out := bed.run("app", "start")
	if code != 0 || !strings.Contains(out, "with no address to listen on") {
		t.Fatalf("app start with start alone: %d\n%s", code, out)
	}
	code, out = bed.run("app", "status")
	if code != 0 || !strings.Contains(out, "state: running") || !strings.Contains(out, "readiness: ready, observed at startup") ||
		!strings.Contains(out, "data: shared with the standing run") {
		t.Fatalf("no ready means alive is ready, and no prepare means shared data:\n%s", out)
	}
	eventuallyTrue(t, "the application's first line in the engine's capture", func() bool {
		_, out := bed.run("app", "log")
		return strings.Contains(out, "started")
	})
	if code, out := bed.run("app", "restart"); code != 0 || !strings.Contains(out, "every recorded process is dead") {
		t.Fatalf("restart: no stop command means TERM then KILL, proven: %d\n%s", code, out)
	}
	if code, out := bed.run("app", "reset"); code != 0 || !strings.Contains(out, "no prepare declared: reset is a restart") {
		t.Fatalf("reset with no prepare: %d\n%s", code, out)
	}
	if code, out := bed.run("app", "check"); code != 0 || !strings.Contains(out, "no check is declared") {
		t.Fatalf("check with none declared: %d\n%s", code, out)
	}
	if code, out := bed.run("app", "stop"); code != 0 || !strings.Contains(out, "every recorded process is dead") {
		t.Fatalf("stop: %d\n%s", code, out)
	}
	if code, out := bed.run("app", "status"); code != 0 || !strings.Contains(out, "state: stopped") {
		t.Fatalf("after stop:\n%s", out)
	}
}

// `--at main` and `--goal G` run side by side: each builds in its own tree,
// each gets its own address from the range and its own state root, prepare
// runs in each with that run's state root and address in its environment,
// and the standing run's record is never touched.
func TestAppAtMainBesideGoal(t *testing.T) {
	address := appFreePort(t)
	contract := appHTTPContract(appFixtureApp(t), address)
	contract["build"] = map[string]any{"argv": []string{"sh", "-c", "git rev-parse HEAD > built.txt"}}
	contract["prepare"] = map[string]any{"argv": []string{"sh", "-c",
		`mkdir -p "$METASYSTEM_APP_STATE_ROOT" && echo "$METASYSTEM_APP_ADDRESS" >> "$METASYSTEM_APP_STATE_ROOT/prepared.txt"`}}
	contract["data"] = "own"
	bed := newAppBed(t, contract)
	bed.git("branch", "goal/g1")
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("standing start: %d\n%s", code, out)
	}
	standingBefore, err := os.ReadFile(applaunch.RecordPath(bed.installation, applaunch.StandingKey))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bed.run("app", "stop", "--at", "main", "--clean") })
	t.Cleanup(func() { bed.run("app", "stop", "--goal", "g1", "--clean") })
	if code, out := bed.run("app", "start", "--at", "main"); code != 0 {
		t.Fatalf("app start --at main: %d\n%s", code, out)
	}
	if code, out := bed.run("app", "start", "--goal", "g1"); code != 0 {
		t.Fatalf("app start --goal g1: %d\n%s", code, out)
	}
	main, err := applaunch.ReadRecord(bed.installation, applaunch.KeyFor("main"))
	if err != nil {
		t.Fatal(err)
	}
	goal, err := applaunch.ReadRecord(bed.installation, applaunch.KeyFor("goal/g1"))
	if err != nil {
		t.Fatal(err)
	}
	if main.Tree == goal.Tree || main.Tree == bed.root || goal.Tree == bed.root {
		t.Fatalf("each run has a tree of its own: %s %s", main.Tree, goal.Tree)
	}
	for _, record := range []*applaunch.Record{main, goal} {
		if built, err := os.ReadFile(filepath.Join(record.Tree, "built.txt")); err != nil || strings.TrimSpace(string(built)) != record.Commit {
			t.Fatalf("the build ran in the run's own tree at its commit: %q %v (%s)", built, err, record.Commit)
		}
		prepared, err := os.ReadFile(filepath.Join(record.StateRoot, "prepared.txt"))
		if err != nil || strings.TrimSpace(string(prepared)) != record.Address {
			t.Fatalf("prepare ran once with the run's own state root and address: %q %v (%s)", prepared, err, record.Address)
		}
		if !answered(record.Address) {
			t.Fatalf("run %s must answer at %s", record.Key, record.Address)
		}
	}
	if main.Address == goal.Address || main.Address == address || goal.Address == address {
		t.Fatalf("three runs, three addresses: standing %s, main %s, goal %s", address, main.Address, goal.Address)
	}
	if main.StateRoot == goal.StateRoot || main.StateRoot == bed.installation || goal.StateRoot == bed.installation {
		t.Fatalf("each run has a state root of its own: %s %s", main.StateRoot, goal.StateRoot)
	}
	if goal.Goal != "g1" || goal.Ref != "goal/g1" || main.Ref != "main" {
		t.Fatalf("the records name the ref and the goal: %+v %+v", main, goal)
	}
	standingAfter, err := os.ReadFile(applaunch.RecordPath(bed.installation, applaunch.StandingKey))
	if err != nil || !bytes.Equal(standingBefore, standingAfter) {
		t.Fatalf("the standing record was touched by another run's start:\n%s\n%s", standingBefore, standingAfter)
	}
	if !answered(address) {
		t.Fatal("the standing run still answers")
	}
}

// log --follow prints the captured tail and then what the application writes
// after it, until it is interrupted.
func TestAppLogFollow(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	previous := appFollowContext
	appFollowContext = func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 1500*time.Millisecond)
	}
	t.Cleanup(func() { appFollowContext = previous })
	go func() {
		time.Sleep(500 * time.Millisecond)
		log, err := os.OpenFile(applaunch.DefaultLogPath(bed.installation, applaunch.StandingKey), os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprintln(log, "written after the follow began")
			_ = log.Close()
		}
	}()
	code, out := bed.run("app", "log", "--follow")
	if code != 0 {
		t.Fatalf("app log --follow: %d\n%s", code, out)
	}
	if !strings.Contains(out, "fixtureapp: listening on") || !strings.Contains(out, "written after the follow began") {
		t.Fatalf("follow prints the tail and then what is written after it:\n%s", out)
	}
}

// check runs the named group through the testing runner's own form with the
// run's address, and the verdict and its time are written on the run's
// record; a run that is alive and not answering is refused in words.
func TestAppCheckRecordsItsVerdict(t *testing.T) {
	address := appFreePort(t)
	contract := appHTTPContract(appFixtureApp(t), address)
	contract["check"] = "app-smoke"
	bed := newAppBed(t, contract)
	var handed []string
	bed.delivery = &intentDeliveryOwners{
		executable: func() (string, error) { return "/engine", nil },
		process: func(process intentProcess) intentProcessResult {
			handed = process.argv
			return intentProcessResult{stdout: []byte(`{"verdict":"pass"}`)}
		}}
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	code, out := bed.run("app", "check")
	if code != 0 {
		t.Fatalf("app check: %d\n%s", code, out)
	}
	joined := strings.Join(handed, " ")
	if !strings.Contains(joined, "internal test run") || !strings.Contains(joined, "--mode canary --groups app-smoke --no-reuse --app-address "+address) {
		t.Fatalf("the check is the testing runner's named-group form with the run's address: %q", joined)
	}
	record, err := applaunch.ReadRecord(bed.installation, applaunch.StandingKey)
	if err != nil {
		t.Fatal(err)
	}
	if record.Check == nil || record.Check.Group != "app-smoke" || record.Check.Verdict != "pass" || record.Check.At == "" || record.Check.Address != address {
		t.Fatalf("the verdict and its time are written on the run record: %+v", record.Check)
	}
	if _, status := bed.run("app", "status"); !strings.Contains(status, "check app-smoke: pass at "+record.Check.At) {
		t.Fatalf("status says the last check:\n%s", status)
	}

	dark := appHTTPContract(appFixtureApp(t), appFreePort(t), "--dark-after", "1s")
	dark["check"] = "app-smoke"
	darkBed := newAppBed(t, dark)
	darkBed.delivery = bed.delivery
	handed = nil
	if code, out := darkBed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	eventuallyTrue(t, "the application to stop answering", func() bool {
		_, out := darkBed.run("app", "status")
		return strings.Contains(out, "readiness: not answering")
	})
	code, out = darkBed.run("app", "check")
	if code == 0 || !strings.Contains(out, "not live and answering, so its check was not run") || handed != nil {
		t.Fatalf("a check against a run that is not answering is refused in words and nothing is run:\n%s", out)
	}
}

// A goal's run copies its evidence, the log tail and the last check, to the
// evidence root under the goal: at stop, and at the next start of that ref
// where the run had ended by itself, before the record is removed.
func TestAppGoalRunEvidenceIsCopiedBeforeItsRecordIsRemoved(t *testing.T) {
	bed := newAppBed(t, map[string]any{
		"name":   "fixture",
		"start":  map[string]any{"argv": []string{appFixtureApp(t), "--no-listen", "--exit-after", "600ms", "--exit-code", "5"}},
		"stopMs": 4000,
	})
	evidence := t.TempDir()
	conf := filepath.Join(bed.installation, "metasystem.conf")
	body, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, append(body, []byte("evidence.root="+evidence+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git("branch", "goal/g1")
	t.Cleanup(func() { bed.run("app", "stop", "--goal", "g1", "--clean") })
	if code, out := bed.run("app", "start", "--goal", "g1"); code != 0 {
		t.Fatalf("app start --goal g1: %d\n%s", code, out)
	}
	eventuallyTrue(t, "the goal's run to end by itself", func() bool {
		_, out := bed.run("app", "status", "--goal", "g1")
		return strings.Contains(out, "state: ended")
	})
	ended, err := applaunch.ReadRecord(bed.installation, applaunch.KeyFor("goal/g1"))
	if err != nil {
		t.Fatal(err)
	}
	code, out := bed.run("app", "start", "--goal", "g1")
	if code != 0 || !strings.Contains(out, "evidence:") || !strings.Contains(out, "copied to") {
		t.Fatalf("the next start copies the ended run's evidence: %d\n%s", code, out)
	}
	copies, _ := filepath.Glob(filepath.Join(evidence, "goals", "g1", "app", applaunch.KeyFor("goal/g1"), "*", "source-*-run.txt"))
	if len(copies) != 1 {
		t.Fatalf("one evidence copy under the goal, got %v\n%s", copies, out)
	}
	summary, _ := os.ReadFile(copies[0])
	if !strings.Contains(string(summary), "ended: "+ended.Ended.At+" (exit 5)") || !strings.Contains(string(summary), "check: none recorded") {
		t.Fatalf("the copy is the ended record, read before it was removed:\n%s", summary)
	}
	tails, _ := filepath.Glob(filepath.Join(filepath.Dir(copies[0]), "source-*-log-tail.txt"))
	if len(tails) != 1 {
		t.Fatalf("the copy carries the log tail, got %v", tails)
	}
	tail, _ := os.ReadFile(tails[0])
	if !strings.Contains(string(tail), "fixtureapp: exiting by itself") {
		t.Fatalf("the copy carries the log tail:\n%s", tail)
	}
	if record, err := applaunch.ReadRecord(bed.installation, applaunch.KeyFor("goal/g1")); err != nil || record.Ended != nil && record.Ended.At == ended.Ended.At && record.Supervisor == ended.Supervisor {
		t.Fatalf("the ended record was replaced by the new run's: %+v %v", record, err)
	}
	if code, out := bed.run("app", "stop", "--goal", "g1"); code != 0 || !strings.Contains(out, "copied to") {
		t.Fatalf("stop copies the goal run's evidence: %d\n%s", code, out)
	}
	copies, _ = filepath.Glob(filepath.Join(evidence, "goals", "g1", "app", applaunch.KeyFor("goal/g1"), "*", "source-*-run.txt"))
	if len(copies) != 2 {
		t.Fatalf("stop made a second evidence copy, got %v", copies)
	}
}

// The check's bridge through the testing runner's own verb: --app-address
// belongs to one named diagnostic group, and what the verb parses is what
// the run request carries to the runner that overlays it.
func TestAppAddressIsOneNamedDiagnosticGroupsInput(t *testing.T) {
	root := t.TempDir()
	parsed, _, status := parseTestingSelection("test run", appCheckArgv(root, "app-smoke", "127.0.0.1:7981")[2:], true)
	if status != 0 || parsed.AppAddress != "127.0.0.1:7981" || !parsed.NoReuse || parsed.Purpose != "diagnostic" ||
		len(parsed.Groups) != 1 || parsed.Groups[0] != "app-smoke" {
		t.Fatalf("the check's argv must parse as one fresh named diagnostic group with the address: %+v status=%d", parsed, status)
	}
	status, _, refusal := captureCommandOutput(t, false, true, func() int {
		_, _, status := parseTestingSelection("test run", []string{"--root", root, "--mode", "standard", "--app-address", "127.0.0.1:7981"}, true)
		return status
	})
	if status != 2 || !strings.Contains(refusal, "--app-address belongs to one named diagnostic group") {
		t.Fatalf("an address outside one named diagnostic group is refused: status=%d %q", status, refusal)
	}
	request := testingRunRequest(testingPreparation{AppAddress: "127.0.0.1:7981"}, "attempt", root, "", "", "")
	if request.AppAddress != "127.0.0.1:7981" {
		t.Fatalf("the run request carries the address to the runner: %+v", request.AppAddress)
	}
}

// A moved ref makes the next start replace the run even while the old one is
// live: a review must never be shown the commit before the one it asked for.
func TestAppStartReplacesALiveRunWhoseTipMoved(t *testing.T) {
	address := appFreePort(t)
	bed := newAppBed(t, appHTTPContract(appFixtureApp(t), address))
	bed.git("branch", "goal/g1")
	t.Cleanup(func() { bed.run("app", "stop", "--goal", "g1", "--clean") })
	code, first := bed.runJSON("app", "start", "--goal", "g1")
	if code != 0 {
		t.Fatalf("app start --goal g1: %v", first)
	}
	before, err := applaunch.ReadRecord(bed.installation, applaunch.KeyFor("goal/g1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.root, "moved.txt"), []byte("moved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git("checkout", "--quiet", "goal/g1")
	bed.git("add", "-A")
	bed.git("commit", "--quiet", "-m", "move the goal branch")
	bed.git("checkout", "--quiet", "main")
	code, out := bed.run("app", "start", "--goal", "g1")
	if code != 0 || strings.Contains(out, "already running") {
		t.Fatalf("a live run whose tip moved is replaced, not rejoined: %d\n%s", code, out)
	}
	after, err := applaunch.ReadRecord(bed.installation, applaunch.KeyFor("goal/g1"))
	if err != nil {
		t.Fatal(err)
	}
	if after.Commit == before.Commit || after.Supervisor == before.Supervisor {
		t.Fatalf("the replaced run is a new run at the new tip: before %s, after %s", before.Commit, after.Commit)
	}
	if identity := before.Supervisor; identity == after.Supervisor {
		t.Fatal("the old run's supervisor must be gone")
	}
	if code, out := bed.run("app", "status", "--goal", "g1"); code != 0 || !strings.Contains(out, "commit: "+after.Commit) {
		t.Fatalf("status names the new commit:\n%s", out)
	}
}

// The standing run's prepare is given a state root of the run's own, never
// the engine's installation: a prepare that clears its state root must not
// be able to clear the engine.
func TestAppStandingPrepareGetsAStateRootOfItsOwn(t *testing.T) {
	address := appFreePort(t)
	contract := appHTTPContract(appFixtureApp(t), address)
	contract["prepare"] = map[string]any{"argv": []string{"sh", "-c",
		`mkdir -p "$METASYSTEM_APP_STATE_ROOT" && echo "$METASYSTEM_APP_STATE_ROOT" > "$METASYSTEM_APP_STATE_ROOT/where.txt"`}}
	bed := newAppBed(t, contract)
	if code, out := bed.run("app", "start"); code != 0 {
		t.Fatalf("app start: %d\n%s", code, out)
	}
	record, err := applaunch.ReadRecord(bed.installation, applaunch.StandingKey)
	if err != nil {
		t.Fatal(err)
	}
	if record.StateRoot == bed.installation || !strings.HasPrefix(record.StateRoot, applaunch.Dir(bed.installation)+string(filepath.Separator)) {
		t.Fatalf("the standing run's state root must be its own, under the app directory, not %s", record.StateRoot)
	}
	if where, err := os.ReadFile(filepath.Join(record.StateRoot, "where.txt")); err != nil || strings.TrimSpace(string(where)) != record.StateRoot {
		t.Fatalf("prepare was given that state root: %q %v", where, err)
	}
}
