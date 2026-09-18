package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
