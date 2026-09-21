package parallelratchet

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// ParallelRatchet records the maximum serial-test count for each package.
// Exemptions name individual tests whose documented constraint prevents them
// from contributing to that count.
type ParallelRatchet struct {
	Packages map[string]int      `json:"packages"`
	Exempt   []ParallelExemption `json:"exempt"`
}

type ParallelExemption struct {
	Package string `json:"package"`
	Test    string `json:"test"`
	Reason  string `json:"reason"`
}

// ParallelTest is one top-level Go test discovered directly from source.
type ParallelTest struct {
	Package  string
	Test     string
	File     string
	Line     int
	Parallel bool
}

// ParallelInventory includes packages even when all their tests are parallel.
type ParallelInventory struct {
	Packages []string
	Tests    []ParallelTest
}

type ParallelViolation struct {
	Package  string
	Test     string
	File     string
	Line     int
	Recorded int
	Actual   int
}

func (violation ParallelViolation) String() string {
	return fmt.Sprintf("package %s test %s at %s:%d: recorded serial count %d, actual %d",
		violation.Package, violation.Test, violation.File, violation.Line, violation.Recorded, violation.Actual)
}

type ParallelDrop struct {
	Package string
	From    int
	To      int
}

// ReadParallelRatchet loads and validates the checked-in serial-test ceiling.
func ReadParallelRatchet(path string) (ParallelRatchet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ParallelRatchet{}, fmt.Errorf("parallel ratchet baseline unreadable: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var ratchet ParallelRatchet
	if err := decoder.Decode(&ratchet); err != nil {
		return ParallelRatchet{}, fmt.Errorf("parallel ratchet baseline unparsable: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return ParallelRatchet{}, fmt.Errorf("parallel ratchet baseline unparsable: %w", err)
	}
	if ratchet.Packages == nil {
		return ParallelRatchet{}, fmt.Errorf("parallel ratchet baseline has no packages object")
	}
	for pkg, count := range ratchet.Packages {
		if strings.TrimSpace(pkg) == "" || count < 0 {
			return ParallelRatchet{}, fmt.Errorf("parallel ratchet package count is invalid: %q=%d", pkg, count)
		}
	}
	return ratchet, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("multiple JSON values")
}

// ScanParallelTests parses every test source without applying build tags.
func ScanParallelTests(root string) (ParallelInventory, error) {
	modulePath, err := readModulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return ParallelInventory{}, err
	}
	fset := token.NewFileSet()
	packageSet := map[string]bool{}
	var tests []ParallelTest
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			// node_modules joins vendor by name: an installed dependency
			// tree is content on disk, never repository content, and its
			// test files are no part of this ratchet (g1-s8 revision 5).
			if path != root && (entry.Name() == ".git" || entry.Name() == "artifacts" || entry.Name() == "vendor" || entry.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}
		relativeDir, relErr := filepath.Rel(root, filepath.Dir(path))
		if relErr != nil {
			return relErr
		}
		pkg := modulePath
		if relativeDir != "." {
			pkg += "/" + filepath.ToSlash(relativeDir)
		}
		packageSet[pkg] = true
		relativeFile, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || function.Body == nil || function.Name.Name == "TestMain" || !strings.HasPrefix(function.Name.Name, "Test") {
				continue
			}
			tests = append(tests, ParallelTest{
				Package:  pkg,
				Test:     function.Name.Name,
				File:     filepath.ToSlash(relativeFile),
				Line:     fset.Position(function.Pos()).Line,
				Parallel: firstStatementCallsParallel(function),
			})
		}
		return nil
	})
	if err != nil {
		return ParallelInventory{}, fmt.Errorf("parallel ratchet scan failed: %w", err)
	}
	packages := make([]string, 0, len(packageSet))
	for pkg := range packageSet {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].Package != tests[j].Package {
			return tests[i].Package < tests[j].Package
		}
		if tests[i].File != tests[j].File {
			return tests[i].File < tests[j].File
		}
		if tests[i].Line != tests[j].Line {
			return tests[i].Line < tests[j].Line
		}
		return tests[i].Test < tests[j].Test
	})
	return ParallelInventory{Packages: packages, Tests: tests}, nil
}

func readModulePath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("parallel ratchet module path unreadable: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || fields[0] != "module" {
			continue
		}
		modulePath := fields[1]
		if unquoted, unquoteErr := strconv.Unquote(modulePath); unquoteErr == nil {
			modulePath = unquoted
		}
		if modulePath == "" {
			break
		}
		return modulePath, nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("parallel ratchet module path unreadable: %w", err)
	}
	return "", fmt.Errorf("parallel ratchet module path missing from %s", path)
}

func firstStatementCallsParallel(function *ast.FuncDecl) bool {
	if len(function.Body.List) == 0 || function.Type.Params == nil || len(function.Type.Params.List) == 0 || len(function.Type.Params.List[0].Names) == 0 {
		return false
	}
	receiver := function.Type.Params.List[0].Names[0].Name
	statement, ok := function.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := statement.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Parallel" {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name == receiver
}

// CheckParallelRatchet reports every non-exempt serial test in a package whose
// count exceeds its recorded ceiling. An absent package has a ceiling of zero.
func CheckParallelRatchet(ratchet ParallelRatchet, inventory ParallelInventory) (map[string]int, []ParallelViolation) {
	exempt := map[string]bool{}
	for _, entry := range ratchet.Exempt {
		if entry.Reason != "" {
			exempt[entry.Package+"\x00"+entry.Test] = true
		}
	}
	counts := make(map[string]int, len(inventory.Packages))
	for _, pkg := range inventory.Packages {
		counts[pkg] = 0
	}
	serial := map[string][]ParallelTest{}
	for _, test := range inventory.Tests {
		if test.Parallel || exempt[test.Package+"\x00"+test.Test] {
			continue
		}
		counts[test.Package]++
		serial[test.Package] = append(serial[test.Package], test)
	}
	var violations []ParallelViolation
	for _, pkg := range inventory.Packages {
		actual := counts[pkg]
		recorded := ratchet.Packages[pkg]
		if actual <= recorded {
			continue
		}
		for _, test := range serial[pkg] {
			violations = append(violations, ParallelViolation{
				Package: pkg, Test: test.Test, File: test.File, Line: test.Line,
				Recorded: recorded, Actual: actual,
			})
		}
	}
	return counts, violations
}

// LowerParallelRatchet returns a baseline containing the current package
// inventory. It never raises a ceiling and returns the blocking violations
// without changing the supplied baseline when a raise would be required.
func LowerParallelRatchet(ratchet ParallelRatchet, inventory ParallelInventory) (ParallelRatchet, []ParallelDrop, []ParallelViolation) {
	counts, violations := CheckParallelRatchet(ratchet, inventory)
	if len(violations) != 0 {
		return ratchet, nil, violations
	}
	updated := ParallelRatchet{
		Packages: make(map[string]int, len(ratchet.Packages)+len(inventory.Packages)),
		Exempt:   append([]ParallelExemption(nil), ratchet.Exempt...),
	}
	for pkg, recorded := range ratchet.Packages {
		updated.Packages[pkg] = recorded
	}
	for _, pkg := range inventory.Packages {
		updated.Packages[pkg] = counts[pkg]
	}
	var drops []ParallelDrop
	for pkg, recorded := range ratchet.Packages {
		actual, present := counts[pkg]
		if !present {
			actual = 0
			updated.Packages[pkg] = 0
		}
		if actual < recorded {
			drops = append(drops, ParallelDrop{Package: pkg, From: recorded, To: actual})
		}
	}
	sort.Slice(drops, func(i, j int) bool { return drops[i].Package < drops[j].Package })
	return updated, drops, nil
}

// WriteParallelRatchet atomically replaces the checked-in baseline.
func WriteParallelRatchet(path, root string, ratchet ParallelRatchet) error {
	data, err := renderParallelRatchet(ratchet)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(path, string(data), root)
	if err != nil {
		return fmt.Errorf("parallel ratchet baseline write failed: %w", err)
	}
	return nil
}

func renderParallelRatchet(ratchet ParallelRatchet) ([]byte, error) {
	packages := make([]string, 0, len(ratchet.Packages))
	for pkg := range ratchet.Packages {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	var output bytes.Buffer
	output.WriteString("{\n  \"packages\": {\n")
	for index, pkg := range packages {
		encodedPackage, err := json.Marshal(pkg)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "    %s: %d", encodedPackage, ratchet.Packages[pkg])
		if index+1 < len(packages) {
			output.WriteByte(',')
		}
		output.WriteByte('\n')
	}
	output.WriteString("  },\n  \"exempt\": [\n")
	for index, exemption := range ratchet.Exempt {
		encoded, err := json.Marshal(exemption)
		if err != nil {
			return nil, err
		}
		output.WriteString("    ")
		output.Write(encoded)
		if index+1 < len(ratchet.Exempt) {
			output.WriteByte(',')
		}
		output.WriteByte('\n')
	}
	output.WriteString("  ]\n}\n")
	return output.Bytes(), nil
}
