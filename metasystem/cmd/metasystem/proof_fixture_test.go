package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

const proofAdmissionFixtureMax = 4

type proofBinaryFixture struct {
	t *testing.T
}

// pinProofBinaryFixture is the one writer of proof admission isolation for
// command fixtures. Four seats are available to a fixture, while its process
// sees a zero-load host and no unrelated launchers.
func pinProofBinaryFixture(t *testing.T, root string) proofBinaryFixture {
	t.Helper()
	path := filepath.Join(root, "metasystem.conf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read proof fixture configuration: %v", err)
	}
	want := proofrun.AdmissionCapKey + "=" + strconv.Itoa(proofAdmissionFixtureMax)
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if key, _, found := strings.Cut(strings.TrimSpace(line), "="); found && strings.TrimSpace(key) == proofrun.AdmissionCapKey {
			count++
			if strings.TrimSpace(line) != want {
				t.Fatalf("proof fixture admission pin = %q, want %q", strings.TrimSpace(line), want)
			}
		}
	}
	if count > 1 {
		t.Fatalf("proof fixture configuration repeats %s", proofrun.AdmissionCapKey)
	}
	if count == 0 {
		if len(data) != 0 && data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		data = append(data, want...)
		data = append(data, '\n')
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("stat proof fixture configuration: %v", statErr)
		}
		if err := os.WriteFile(path, data, info.Mode().Perm()); err != nil {
			t.Fatalf("write proof fixture admission pin: %v", err)
		}
	}
	return proofBinaryFixture{t: t}
}

func (fixture proofBinaryFixture) command(environment []string, executable string, args ...string) *exec.Cmd {
	fixture.t.Helper()
	if environment == nil {
		environment = os.Environ()
	}
	prefix := proofrun.TestHostLoadEnvironment + "="
	isolated := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			isolated = append(isolated, entry)
		}
	}
	command := exec.Command(executable, args...)
	command.Args[0] = proofrun.TestHostLoadCommandName("0")
	command.Env = isolated
	return command
}

func TestProofAttemptBinaryFixturePinsAdmissionInputs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	fixture := pinProofBinaryFixture(t, root)
	data, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	want := proofrun.AdmissionCapKey + "=" + strconv.Itoa(proofAdmissionFixtureMax)
	if strings.Count(string(data), want) != 1 {
		t.Fatalf("proof fixture configuration = %q, want one %q", data, want)
	}
	command := fixture.command([]string{"HOME=/fixture", proofrun.TestHostLoadEnvironment + "=3"}, "metasystem", "test", "run")
	if len(command.Args) != 3 || command.Args[1] != "test" || command.Args[2] != "run" {
		t.Fatalf("fixture command args = %q", command.Args)
	}
	if command.Args[0] != proofrun.TestHostLoadCommandName("0") {
		t.Fatalf("fixture command sampler option = %q", command.Args[0])
	}
	for _, entry := range command.Env {
		if strings.HasPrefix(entry, proofrun.TestHostLoadEnvironment+"=") {
			t.Fatalf("fixture command retained ambient host load input %q", entry)
		}
	}
}

func TestProofAttemptBinaryLaunchesUseSharedIsolation(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"proof_fixture_test.go": true}
	for _, path := range files {
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isExecCommandCall(call) || len(call.Args) == 0 || !looksLikeFixtureEngine(call.Args[0], call.Args[1:]) {
				return true
			}
			if !allowed[filepath.Base(path)] {
				t.Errorf("%s launches a metasystem fixture outside proofBinaryFixture.command", path)
			}
			return true
		})
	}
}

func TestLandingBatchBinaryIgnoresAmbientProofHostLoad(t *testing.T) {
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", engine, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build metasystem binary: %v\n%s", err, output)
	}

	blockerRoot := t.TempDir()
	blockerConf := filepath.Join(blockerRoot, "metasystem.conf")
	if err := os.WriteFile(blockerConf, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(blockerRoot, "release.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	blocker := exec.Command(engine, "proof-run", "launch", "--suite", "ambient-admission-blocker",
		"--root", blockerRoot, "--conf", blockerConf, "--progress", filepath.Join(blockerRoot, "progress.jsonl"),
		"--log", filepath.Join(blockerRoot, "proof.log"), "--banner", "blocker-starting", "--",
		"sh", "-c", `printf 'blocker-ready\n'; read -r _ <"$1"`, "sh", fifo)
	blocker.Env = proofFixtureEnvironmentWithoutHostLoad(os.Environ())
	stdout, err := blocker.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var blockerStderr bytes.Buffer
	blocker.Stderr = &blockerStderr
	if err := blocker.Start(); err != nil {
		t.Fatal(err)
	}
	blockerReader := bufio.NewReader(stdout)
	if line, err := blockerReader.ReadString('\n'); err != nil || line != "blocker-starting\n" {
		t.Fatalf("blocker banner=%q error=%v stderr=%s", line, err, blockerStderr.String())
	}
	if line, err := blockerReader.ReadString('\n'); err != nil || line != "blocker-ready\n" {
		t.Fatalf("blocker readiness=%q error=%v stderr=%s", line, err, blockerStderr.String())
	}
	t.Cleanup(func() {
		release, openErr := os.OpenFile(fifo, os.O_WRONLY, 0)
		if openErr == nil {
			_, _ = release.WriteString("release\n")
			_ = release.Close()
		}
		if waitErr := blocker.Wait(); openErr != nil || waitErr != nil {
			t.Errorf("release proof blocker: open=%v wait=%v stderr=%s", openErr, waitErr, blockerStderr.String())
		}
	})

	root, now := proofExtensionGoalFixture(t)
	caller, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe binary fixture caller: state=%s error=%v", state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "ambient-admission-main", caller.Pid, caller.StartedAt.Unix(), caller.StartTicks,
		caller.BootID, "mac-cli", "fake", "m1"); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(root, "metasystem.conf")
	data, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data,
		[]byte(proofrun.AdmissionCapKey+"="+strconv.Itoa(proofAdmissionFixtureMax)),
		[]byte(proofrun.AdmissionCapKey+"=1"))
	if err := os.WriteFile(conf, data, 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(name string, ambient bool) (proofrun.LaunchResult, int, string) {
		t.Helper()
		resultPath := filepath.Join(root, name+"-result.json")
		command := exec.Command(engine, "proof-run", "launch", "--suite", name, "--root", root,
			"--conf", conf, "--goal", "standing-validation", "--cap-min", "1", "--scope", "full",
			"--command-class", name, "--progress", filepath.Join(root, name+"-progress.jsonl"),
			"--log", filepath.Join(root, name+".log"), "--banner", name, "--result", resultPath, "--", "/usr/bin/true")
		command.Env = append(proofFixtureEnvironmentWithoutHostLoad(os.Environ()),
			"METASYSTEM_GOAL_NOW="+now.Format(time.RFC3339), "METASYSTEM_OWNER_LINEAGE=m1")
		if ambient {
			command.Env = append(command.Env, proofrun.TestHostLoadEnvironment+"=0")
		}
		output, runErr := command.CombinedOutput()
		status := 0
		if runErr != nil {
			var exit *exec.ExitError
			if !errors.As(runErr, &exit) {
				t.Fatalf("run %s: %v\n%s", name, runErr, output)
			}
			status = exit.ExitCode()
		}
		resultBytes, readErr := os.ReadFile(resultPath)
		if readErr != nil {
			t.Fatalf("read %s admission result: %v\n%s", name, readErr, output)
		}
		var result proofrun.LaunchResult
		if err := json.Unmarshal(resultBytes, &result); err != nil {
			t.Fatalf("decode %s admission result: %v\n%s", name, err, resultBytes)
		}
		return result, status, string(output)
	}

	withoutAmbient, withoutStatus, withoutOutput := run("without-ambient-host-load", false)
	withAmbient, withStatus, withOutput := run("with-ambient-host-load", true)
	if withoutStatus != proofrun.ExitAdmissionRefused || withStatus != proofrun.ExitAdmissionRefused ||
		withoutAmbient != withAmbient || withoutAmbient.Disposition != proofrun.DispositionAdmissionRefused ||
		!strings.HasPrefix(withAmbient.Reason, "ADMISSION_") {
		t.Fatalf("ambient variable changed built-binary admission:\nwithout=%+v status=%d output=%s\nwith=%+v status=%d output=%s",
			withoutAmbient, withoutStatus, withoutOutput, withAmbient, withStatus, withOutput)
	}
}

func proofFixtureEnvironmentWithoutHostLoad(environment []string) []string {
	prefix := proofrun.TestHostLoadEnvironment + "="
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func TestLandFixtureConfigurationsPinProofAdmission(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "land-fixtures.sh"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, want := range []string{
		"pin_land_fixture_proof_admission()",
		"pin_land_fixture_proof_admission \"$leg_seed/metasystem.conf\"",
		"proof.admission.top-level-max=4",
		`[[ "$fixture_scenario" == *admission* ]]`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("land fixture admission isolation is missing %q", want)
		}
	}
}

func isExecCommandCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	packageName, packageOK := selector.X.(*ast.Ident)
	return packageOK && packageName.Name == "exec" && (selector.Sel.Name == "Command" || selector.Sel.Name == "CommandContext")
}

func looksLikeFixtureEngine(executable ast.Expr, args []ast.Expr) bool {
	if identifier, ok := executable.(*ast.Ident); ok {
		if identifier.Name == "engine" || identifier.Name == "executable" {
			return true
		}
		if identifier.Name != "binary" {
			return false
		}
		return containsProofVerb(args)
	}
	source := exprText(executable)
	return strings.Contains(source, `"metasystem"`) || strings.Contains(source, "ENGINE") && containsProofVerb(args)
}

func containsProofVerb(args []ast.Expr) bool {
	var words []string
	for _, argument := range args {
		ast.Inspect(argument, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if ok && literal.Kind == token.STRING {
				if word, err := strconv.Unquote(literal.Value); err == nil {
					words = append(words, word)
				}
			}
			return true
		})
	}
	joined := " " + strings.Join(words, " ") + " "
	return strings.Contains(joined, " proof-run ") || strings.Contains(joined, " landing ") || strings.Contains(joined, " reaper ") || strings.Contains(joined, " test run ")
}

func exprText(expression ast.Expr) string {
	var parts []string
	ast.Inspect(expression, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.Ident:
			parts = append(parts, value.Name)
		case *ast.BasicLit:
			parts = append(parts, value.Value)
		}
		return true
	})
	return strings.Join(parts, " ")
}
