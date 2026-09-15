package main

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestPathFlagParsesEverySupportedPosition(t *testing.T) {
	checkout := t.TempDir()
	realDir := filepath.Join(checkout, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(checkout, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}
	t.Chdir(checkout)
	absoluteRoot := absolutePath(t, ".")

	tests := []struct {
		name, defaultValue string
		args               []string
		want               string
		remaining          []string
	}{
		{"after value flag", "", []string{"--id", "foo", "--root", "."}, absoluteRoot, nil},
		{"equals form", "", []string{"--root=."}, absoluteRoot, nil},
		{"single dash", "", []string{"-root", "."}, absoluteRoot, nil},
		{"after separator", ".", []string{"--", "--root", link}, absoluteRoot, []string{"--root", link}},
		{"symlinked directory", "", []string{"--root", link}, absolutePath(t, link), nil},
		{"path does not exist", "", []string{"--root", "future"}, absolutePath(t, "future"), nil},
		{"empty value", ".", []string{"--root="}, "", nil},
		{"absolute default", ".", nil, absoluteRoot, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flags := flag.NewFlagSet(test.name, flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			flags.String("id", "", "value flag")
			root := pathFlag(flags, "root", test.defaultValue, "checkout root")
			if err := flags.Parse(test.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if *root != test.want || !slices.Equal(flags.Args(), test.remaining) {
				t.Fatalf("root = %q, args = %q; want %q, %q", *root, flags.Args(), test.want, test.remaining)
			}
		})
	}

	var repo string
	flags := flag.NewFlagSet("path var", flag.ContinueOnError)
	pathFlagVar(flags, &repo, "repo", ".", "checkout root")
	if repo != absoluteRoot {
		t.Fatalf("pathFlagVar default = %q, want %q", repo, absoluteRoot)
	}
}

func absolutePath(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}

func TestPathValuedFlagsUsePathHelpers(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := parser.ParseFile(files, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(source, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			nameArg, usageArg := 0, 2
			if selector.Sel.Name == "StringVar" {
				nameArg, usageArg = 1, 3
			} else if selector.Sel.Name != "String" {
				return true
			}
			if len(call.Args) <= usageArg {
				return true
			}
			flagName, named := goString(call.Args[nameArg])
			usage, described := goString(call.Args[usageArg])
			if named && (flagName == "root" || flagName == "repo") && (!described || usage != "chain root job id") {
				t.Errorf("%s: path-valued --%s uses plain %s", files.Position(call.Pos()), flagName, selector.Sel.Name)
			}
			return true
		})
	}
}

func goString(expression ast.Expr) (string, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func TestReadOnlyVerbsReportTheSamePathForRelativeAndAbsoluteFlags(t *testing.T) {
	checkout := t.TempDir()
	contract, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "testing.json"), contract, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(checkout)
	absolute := absolutePath(t, ".")

	tests := []struct {
		name, flagName string
		command        []string
	}{
		{"supervise status", "repo", []string{"supervise", "status"}},
		{"goal list", "root", []string{"goal", "list", "--json"}},
		{"test list", "root", []string{"test", "list", "--json"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := func(value string) (string, int) {
				args := append(append([]string{}, test.command...), "--"+test.flagName, value)
				return captureStdout(t, func() int { return dispatch(args) })
			}
			relativeOutput, relativeCode := run(".")
			absoluteOutput, absoluteCode := run(absolute)
			if relativeCode != 0 || absoluteCode != 0 || relativeOutput != absoluteOutput {
				t.Fatalf("relative: code=%d output=%q\nabsolute: code=%d output=%q", relativeCode, relativeOutput, absoluteCode, absoluteOutput)
			}
			if !strings.Contains(relativeOutput, absolute) {
				t.Fatalf("output does not contain resolved path %q: %s", absolute, relativeOutput)
			}
		})
	}
}
