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
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
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
		if err := testexec.WriteFile(path, data, info.Mode().Perm()); err != nil {
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

	startAdmissionBlocker(t, engine, "ambient-admission-blocker")

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
	// A second live launcher makes the host count differ on an otherwise idle
	// machine, proving that this assertion ignores only that ambient count.
	startAdmissionBlocker(t, engine, "ambient-admission-second-blocker")
	withAmbient, withStatus, withOutput := run("with-ambient-host-load", true)
	if withoutStatus != proofrun.ExitAdmissionRefused || withStatus != proofrun.ExitAdmissionRefused ||
		withoutAmbient.Disposition != proofrun.DispositionAdmissionRefused ||
		!strings.HasPrefix(withoutAmbient.Reason, "ADMISSION_") || !strings.HasPrefix(withAmbient.Reason, "ADMISSION_") {
		t.Fatalf("ambient variable changed built-binary admission:\nwithout=%+v status=%d output=%s\nwith=%+v status=%d output=%s",
			withoutAmbient, withoutStatus, withoutOutput, withAmbient, withStatus, withOutput)
	}
	compareLaunchResultsIgnoringObserved(t, withoutAmbient, withAmbient)
}

func startAdmissionBlocker(t *testing.T, engine, name string) {
	t.Helper()
	root := t.TempDir()
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte(proofrun.AdmissionCapKey+"=0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(root, "release.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	banner := name + "-starting"
	blocker := exec.Command(engine, "proof-run", "launch", "--suite", name,
		"--root", root, "--conf", conf, "--progress", filepath.Join(root, "progress.jsonl"),
		"--log", filepath.Join(root, "proof.log"), "--banner", banner, "--",
		"sh", "-c", `printf '%s-ready\n' "$1"; read -r _ <"$2"`, "sh", name, fifo)
	blocker.Env = proofFixtureEnvironmentWithoutHostLoad(os.Environ())
	stdout, err := blocker.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	blocker.Stderr = &stderr
	if err := blocker.Start(); err != nil {
		t.Fatal(err)
	}
	ready := false
	defer func() {
		if !ready {
			_ = blocker.Process.Kill()
			_ = blocker.Wait()
		}
	}()
	reader := bufio.NewReader(stdout)
	if line, err := reader.ReadString('\n'); err != nil || line != banner+"\n" {
		t.Fatalf("%s banner=%q error=%v stderr=%s", name, line, err, stderr.String())
	}
	if line, err := reader.ReadString('\n'); err != nil || line != name+"-ready\n" {
		t.Fatalf("%s readiness=%q error=%v stderr=%s", name, line, err, stderr.String())
	}
	ready = true
	t.Cleanup(func() {
		release, openErr := os.OpenFile(fifo, os.O_WRONLY, 0)
		if openErr == nil {
			_, _ = release.WriteString("release\n")
			_ = release.Close()
		}
		if waitErr := blocker.Wait(); openErr != nil || waitErr != nil {
			t.Errorf("release proof blocker %s: open=%v wait=%v stderr=%s", name, openErr, waitErr, stderr.String())
		}
	})
}

func compareLaunchResultsIgnoringObserved(t *testing.T, withoutAmbient, withAmbient proofrun.LaunchResult) {
	t.Helper()
	fields := []struct {
		name                    string
		withoutValue, withValue string
	}{
		{"SchemaVersion", strconv.Itoa(withoutAmbient.SchemaVersion), strconv.Itoa(withAmbient.SchemaVersion)},
		{"Disposition", withoutAmbient.Disposition, withAmbient.Disposition},
		{"AttemptID", withoutAmbient.AttemptID, withAmbient.AttemptID},
		{"PriorAttempt", withoutAmbient.PriorAttempt, withAmbient.PriorAttempt},
		{"EvidencePath", withoutAmbient.EvidencePath, withAmbient.EvidencePath},
		{"ExitStatus", strconv.Itoa(withoutAmbient.ExitStatus), strconv.Itoa(withAmbient.ExitStatus)},
	}
	for _, field := range fields {
		if field.withoutValue != field.withValue {
			t.Fatalf("launch result field %s differs: without=%q with=%q", field.name, field.withoutValue, field.withValue)
		}
	}

	withoutTokens := admissionReasonTokens(t, withoutAmbient.Reason)
	withTokens := admissionReasonTokens(t, withAmbient.Reason)
	for _, leg := range []struct {
		name   string
		tokens map[string]string
	}{{"without", withoutTokens}, {"with", withTokens}} {
		observed, ok := leg.tokens["observed"]
		value, err := strconv.Atoi(observed)
		if !ok || err != nil || value < 1 {
			t.Fatalf("%s reason token observed=%q, want an integer of at least 1", leg.name, observed)
		}
	}
	tokenNames := make([]string, 0, len(withoutTokens)+len(withTokens))
	seen := map[string]bool{}
	for name := range withoutTokens {
		tokenNames = append(tokenNames, name)
		seen[name] = true
	}
	for name := range withTokens {
		if !seen[name] {
			tokenNames = append(tokenNames, name)
		}
	}
	sort.Strings(tokenNames)
	for _, name := range tokenNames {
		withoutValue, withoutOK := withoutTokens[name]
		withValue, withOK := withTokens[name]
		if name != "observed" && (withoutOK != withOK || withoutValue != withValue) {
			t.Fatalf("launch result reason token %s differs: without=%q present=%t with=%q present=%t",
				name, withoutValue, withoutOK, withValue, withOK)
		}
	}
}

func admissionReasonTokens(t *testing.T, reason string) map[string]string {
	t.Helper()
	fields := strings.Fields(reason)
	if len(fields) == 0 {
		t.Fatal("admission reason has no tokens")
	}
	tokens := map[string]string{"code": fields[0]}
	for _, field := range fields[1:] {
		name, value, found := strings.Cut(field, "=")
		if !found {
			name, value = field, field
		}
		if _, exists := tokens[name]; exists {
			t.Fatalf("admission reason repeats token %s", name)
		}
		tokens[name] = value
	}
	return tokens
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

func TestLandFixtureScenarioRegistryMatchesCount(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "land-fixtures.sh"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	const countPrefix = `run_fixture_bed_scenarios land "land fixtures passed (`
	start := strings.Index(source, countPrefix)
	if start < 0 {
		t.Fatal("land fixture scenario registry is absent")
	}
	registry := source[start+len(countPrefix):]
	const countSuffix = ` isolated legs)" \`
	countEnd := strings.Index(registry, countSuffix)
	if countEnd < 0 {
		t.Fatal("land fixture scenario count is absent")
	}
	declared, err := strconv.Atoi(registry[:countEnd])
	if err != nil {
		t.Fatalf("land fixture scenario count %q is not numeric: %v", registry[:countEnd], err)
	}
	registry = registry[countEnd+len(countSuffix):]
	const scriptArgument = `"$fixture_bed_script"`
	scriptEnd := strings.Index(registry, scriptArgument)
	if scriptEnd < 0 {
		t.Fatal("land fixture scenario registry has no script argument")
	}
	registry = registry[scriptEnd+len(scriptArgument):]
	registryEnd := strings.Index(registry, "\nfi")
	if registryEnd < 0 {
		t.Fatal("land fixture scenario registry has no closing branch")
	}
	scenarios := strings.Fields(strings.ReplaceAll(registry[:registryEnd], "\\\n", " "))
	if declared != len(scenarios) {
		t.Fatalf("land fixture success count = %d, registered scenarios = %d", declared, len(scenarios))
	}

	want := []string{
		"early-reader-large-producer",
		"push-retry",
		"step-failure",
		"new-plan",
		"goal",
		"receipt-line",
		"tier-one",
		"full-width-chain",
		"build-stamp",
		"brain-land-refuses",
		"brain-absent-node-proceeds",
		"ledger-move-lands",
		"records-move-lands",
		"input-move-refuses",
		"receipt-cutover",
		"carried-fresh",
		"carried-prefixed",
		"carried-second",
		"carried-red-battery",
		"carried-intent-failure",
		"carried-crash-local",
		"carried-asks",
		"carried-ledger-path",
		"carried-crash",
		"carried-two-seat",
		"carried-debt-abandoned",
		"carried-debt-expired",
		"abandonment-route-normal",
		"abandonment-route-retry",
		"abandonment-route-wrapper",
		"abandonment-route-commit-push-range",
		"abandonment-route-commit-push-rejected",
		"abandonment-route-stack",
		"abandonment-route-positive",
		"abandonment-route-recertified",
		"batch-owner-holds-lease",
		"batch-lands-by-agent-commit",
		"batch-two-units-disjoint-groups",
		"batch-land-trunk-moved",
		"batch-land-resumes",
		"batch-red-ejects-owner-and-lands-survivors",
		"batch-conflicting-join-refused",
		"batch-join-static-red-refused",
		"batch-join-dropped-test-refused",
		"batch-withdraw-before-and-after-seal",
	}
	if strings.Join(scenarios, "\n") != strings.Join(want, "\n") {
		t.Fatalf("land fixture scenarios = %q, want %q", scenarios, want)
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
