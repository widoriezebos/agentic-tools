package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestLandingWordsNameCoveredIntentVerbs(t *testing.T) {
	t.Parallel()
	for key, row := range state.Words {
		if row.Verb == "none" {
			continue
		}
		words := strings.Fields(row.Verb)
		command, ok := findIntentAction(words[0], words[1])
		if !ok {
			t.Errorf("%v names absent intent %q", key, row.Verb)
			continue
		}
		if len(command.examples) == 0 {
			t.Errorf("%v: %s has no coverage examples", key, row.Verb)
		}
		if len(words) > 2 {
			if _, problem := parseIntentArgs(command, []string{words[2], "fixture"}); problem != nil {
				t.Errorf("%v: %s: %v", key, row.Verb, problem)
			}
		}
	}
}

func TestLandingHealthWordsPrintsTheWholeTable(t *testing.T) {
	t.Parallel()
	bed, _, _, _, _ := landingRestartBed(t)
	command, _ := findIntentAction("landing", "health")
	var out, errOut bytes.Buffer
	if code := runIntentIn(command, []string{"--words"}, &out, &errOut, bed.cwd, bed.owners()); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	for _, want := range []string{"section", "word", "phrase", "verb", "landing resume", "landing run --goals", "paused by"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q: %s", want, out.String())
		}
	}
	for key, row := range state.Words {
		if !strings.Contains(out.String(), row.Phrase) {
			t.Errorf("%v missing from words", key)
		}
	}
}

var landingStateWord = regexp.MustCompile(`(?i)\b(running|paused|stopped|proving|resolving|held|returned|landed)\b`)

// Person-facing state phrases throughout the lane have one wording owner.
func landingHeadlineLiterals(file *ast.File, positions *token.FileSet) []string {
	var found []string
	// A single state word at a headline sink is person-facing too.
	inspectHeadline := func(node ast.Node) {
		ast.Inspect(node, func(node ast.Node) bool {
			if comparison, ok := node.(*ast.BinaryExpr); ok && (comparison.Op == token.EQL || comparison.Op == token.NEQ) {
				return false
			}
			if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
				text, err := strconv.Unquote(literal.Value)
				if err == nil && landingStateWord.MatchString(text) && len(strings.Fields(text)) == 1 {
					found = append(found, fmt.Sprintf("%s: state headline %q", positions.Position(literal.Pos()), text))
				}
			}
			return true
		})
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			if strings.Contains(strings.ToLower(node.Name.Name), "headline") {
				inspectHeadline(node.Body)
			}
		case *ast.CallExpr:
			if selector, ok := node.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Headline" {
				for _, arg := range node.Args {
					inspectHeadline(arg)
				}
			}
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				if selector, ok := lhs.(*ast.SelectorExpr); ok && selector.Sel.Name == "ProofHeadline" {
					for _, rhs := range node.Rhs {
						inspectHeadline(rhs)
					}
				}
			}
		}
		return true
	})
	ast.Inspect(file, func(n ast.Node) bool {
		if comparison, ok := n.(*ast.BinaryExpr); ok && (comparison.Op == token.EQL || comparison.Op == token.NEQ) {
			return false
		}
		if kv, ok := n.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && (key.Name == "usage" || key.Name == "details" || key.Name == "examples" || key.Name == "flags") {
				return false
			}
		}
		literal, ok := n.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		text, err := strconv.Unquote(literal.Value)
		if err == nil && landingStateWord.MatchString(text) && len(strings.Fields(text)) > 1 && !strings.HasPrefix(text, "metasystem ") {
			found = append(found, fmt.Sprintf("%s: state phrase %q", positions.Position(literal.Pos()), text))
		}
		return true
	})
	return found
}

func TestLandingHeadlinesHaveOneProducer(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"../../internal/landing", "."} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.Base(path) == "describe.go" || filepath.Base(path) == "words.go" || root == "." && !strings.Contains(filepath.Base(path), "landing") {
				return nil
			}
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, path, nil, 0)
			if err != nil {
				return err
			}
			for _, finding := range landingHeadlineLiterals(file, positions) {
				t.Error(finding)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLandingHeadlineAuditRejectsAnotherProducer(t *testing.T) {
	t.Parallel()
	for _, source := range []string{`package fixture; func otherHeadline() string {return "paused"}`, `package fixture; func otherHeadline() string {return "proving again"}`, `package fixture; func f(){page.Headline("the lane is paused")}`, `package fixture; func f(){status.ProofHeadline="the lane stopped"}`, `package fixture; func keeper(){return fmt.Sprintf("lane paused by %s", by)}`} {
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, "landing_fixture.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(landingHeadlineLiterals(file, positions)) == 0 {
			t.Fatal("audit accepted another headline producer: " + source)
		}
	}
	file, err := parser.ParseFile(token.NewFileSet(), "landing_fixture.go", `package fixture; const Running="running"; func f(){r.Word="paused"}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if found := landingHeadlineLiterals(file, token.NewFileSet()); len(found) > 0 {
		t.Fatal("audit mistook machine state for a headline: " + strings.Join(found, "; "))
	}
}

func TestLandingHealthWordsCLI(t *testing.T) {
	t.Parallel()
	bed, _, _, _, _ := landingRestartBed(t)
	registry := t.TempDir()
	host := filepath.Join(registry, ".metasystem", "host")
	if err := os.MkdirAll(host, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(bed.home, "host", "landing-lane.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(host, "landing-lane.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	engine := os.Getenv("METASYSTEM_WAIT_BINARY")
	if engine == "" {
		t.Fatal("the package did not build the CLI")
	}
	for _, args := range [][]string{{"landing", "health", "--words"}, {"landing", "health", "--words", "unexpected"}} {
		command := exec.CommandContext(t.Context(), engine, args...)
		command.Dir = bed.cwd
		command.Env = fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+registry)
		output, err := command.CombinedOutput()
		if len(args) == 4 {
			if err == nil {
				t.Fatal("CLI accepted an unexpected argument")
			}
			continue
		}
		if err != nil || !strings.Contains(string(output), "batch.execution") || !strings.Contains(string(output), "landing run --goals") {
			t.Fatalf("CLI: %v: %s", err, output)
		}
	}
}
