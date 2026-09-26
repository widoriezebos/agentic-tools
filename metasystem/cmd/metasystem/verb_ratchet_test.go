package main

// Ratchet witnesses for the object-action verb redesign
// (plans/designs/verbs-object-action.md, section 4, unit U0). Each test
// measures one number the redesign must drive down and holds it at a ceiling
// equal to the value measured when the witness landed. A change that raises a
// number fails and names the sites; a change that lowers one also fails until
// the same commit lowers the ceiling to the new value, so the ratchet never
// carries slack. The tests walk the filesystem and never call Git.

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	// R1/R3 residue: (family, verb) pairs plus dispatchInternal's top-level forms.
	verbRatchetInternalVerbCeiling = 473
	// R4: shell lines that reference the engine.
	verbRatchetShellEngineCeiling = 3119
	// R4 second ceiling: all lines of shell files under metasystem/scripts.
	verbRatchetScriptLinesCeiling = 52181
	// R5: non-test Go sites that run or build an argv for the engine itself.
	verbRatchetSelfSubprocessCeiling = 71
	// R6: instruction text naming a form whose first word is not public.
	verbRatchetInstructionCeiling = 426
)

type ratchetSite struct {
	path string
	line int
	text string
}

func (site ratchetSite) String() string {
	return fmt.Sprintf("%s:%d: %s", site.path, site.line, site.text)
}

// verbRatchetRoots returns the repository root and the metasystem module root.
func verbRatchetRoots(t *testing.T) (string, string) {
	t.Helper()
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(module, "go.mod")); err != nil {
		t.Fatalf("module root %s has no go.mod: %v", module, err)
	}
	return filepath.Dir(module), module
}

// checkVerbRatchet fails when the measured number differs from its ceiling:
// above it names every site, below it asks for the ceiling to be lowered.
func checkVerbRatchet(t *testing.T, number, constant string, measured, ceiling int, sites []ratchetSite) {
	t.Helper()
	t.Logf("%s: measured %d, ceiling %d", number, measured, ceiling)
	if measured > ceiling {
		every := ratchetSiteList(sites)
		if len(sites) > ratchetSiteListLimit {
			every = fmt.Sprintf("  (%d sites; the per-file counts above locate the growth)\n", len(sites))
		}
		t.Errorf("%s is %d, above its ceiling %d (%s). Per file:\n%s\nEvery site:\n%s",
			number, measured, ceiling, constant, ratchetPerFile(sites), every)
	}
	if measured < ceiling {
		t.Errorf("%s is %d, below its ceiling %d: lower %s to %d in this commit", number, measured, ceiling, constant, measured)
	}
}

// ratchetSiteListLimit bounds the failure message: above it only the per-file
// counts are printed.
const ratchetSiteListLimit = 5000

func ratchetPerFile(sites []ratchetSite) string {
	counts := map[string]int{}
	for _, site := range sites {
		counts[site.path]++
	}
	paths := make([]string, 0, len(counts))
	for path := range counts {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		if counts[paths[i]] != counts[paths[j]] {
			return counts[paths[i]] > counts[paths[j]]
		}
		return paths[i] < paths[j]
	})
	var b strings.Builder
	for _, path := range paths {
		fmt.Fprintf(&b, "  %6d %s\n", counts[path], path)
	}
	return b.String()
}

func ratchetSiteList(sites []ratchetSite) string {
	var b strings.Builder
	for _, site := range sites {
		fmt.Fprintf(&b, "  %s\n", site)
	}
	return b.String()
}

// walkRatchetFiles visits regular files under root in lexical order, skipping
// the named directories, root-relative directory prefixes, and nested
// checkouts (any directory other than root that carries a .git entry).
func walkRatchetFiles(t *testing.T, root string, skipNames, skipPrefixes []string, visit func(path, rel string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if path == root {
				return nil
			}
			for _, name := range skipNames {
				if entry.Name() == name {
					return filepath.SkipDir
				}
			}
			for _, prefix := range skipPrefixes {
				if rel == prefix {
					return filepath.SkipDir
				}
			}
			if _, statErr := os.Lstat(filepath.Join(path, ".git")); statErr == nil {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() {
			visit(path, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func readRatchetLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return lines
}

/* ---------------------------------------------------- 1 internal verbs -- */

// TestVerbRatchetInternalVerbCount counts every (family, verb) pair the
// registered families route, plus the top-level forms dispatchInternal routes
// before its family loop, read from main.go's `args[0] == "WORD"` tests.
func TestVerbRatchetInternalVerbCount(t *testing.T) {
	t.Parallel()
	var sites []ratchetSite
	for _, fam := range families() {
		for _, v := range fam.verbs {
			sites = append(sites, ratchetSite{path: "family " + fam.name, text: fam.name + " " + v.name})
		}
	}
	forms := dispatchInternalTopLevelForms(t)
	if len(forms) == 0 {
		t.Fatal("found no top-level forms in dispatchInternal; the scan no longer reads main.go")
	}
	for _, form := range forms {
		sites = append(sites, ratchetSite{path: "main.go dispatchInternal", text: form})
	}
	checkVerbRatchet(t, "internal verb count", "verbRatchetInternalVerbCeiling", len(sites), verbRatchetInternalVerbCeiling, sites)
}

// ratchetRoutedWords are the first words the engine's internal router
// answers today: the family names and dispatchInternal's top-level forms.
func ratchetRoutedWords(t *testing.T) map[string]bool {
	t.Helper()
	routed := map[string]bool{}
	for _, fam := range families() {
		routed[fam.name] = true
	}
	for _, form := range dispatchInternalTopLevelForms(t) {
		routed[form] = true
	}
	return routed
}

func dispatchInternalTopLevelForms(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var forms []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "dispatchInternal" {
			continue
		}
		for _, stmt := range fn.Body.List {
			ifStmt, ok := stmt.(*ast.IfStmt)
			if !ok {
				continue
			}
			cond, ok := ifStmt.Cond.(*ast.BinaryExpr)
			if !ok || cond.Op != token.EQL {
				continue
			}
			index, ok := cond.X.(*ast.IndexExpr)
			if !ok {
				continue
			}
			if name, ok := index.X.(*ast.Ident); !ok || name.Name != "args" {
				continue
			}
			if lit, ok := index.Index.(*ast.BasicLit); !ok || lit.Value != "0" {
				continue
			}
			word, ok := cond.Y.(*ast.BasicLit)
			if !ok || word.Kind != token.STRING {
				continue
			}
			value, err := strconv.Unquote(word.Value)
			if err != nil {
				t.Fatal(err)
			}
			forms = append(forms, value)
		}
	}
	return forms
}

/* ------------------------------------------ 2, 3 shell and the engine -- */

// Directory names and repository-relative prefixes the shell scans skip.
var (
	ratchetShellSkipNames    = []string{".git", "node_modules", "artifacts", "redesign", "records"}
	ratchetShellSkipPrefixes = []string{"plans"}
)

func ratchetShellFiles(t *testing.T, root string, skipPrefixes []string) []string {
	t.Helper()
	var files []string
	walkRatchetFiles(t, root, ratchetShellSkipNames, skipPrefixes, func(path, rel string) {
		if strings.HasSuffix(rel, ".sh") || strings.HasSuffix(rel, ".bash") {
			files = append(files, path)
		}
	})
	return files
}

// ratchetEngineSeedVariables hold the engine path by name in today's scripts.
// Every name ending in "engine" (any case) except the flags no_engine and
// *_from_engine is also an engine variable, and a
// variable whose whole assignment is an engine path or another engine
// variable (with or without a default) joins the set by discovery.
var ratchetEngineSeedVariables = []string{
	"ms", "engine", "deadline_engine", "gate_build_scratch", "checkout_execution_guard_engine",
}

var (
	ratchetShellNameRE    = regexp.MustCompile(`(?i)^[a-z_][a-z0-9_]*engine$`)
	ratchetShellNotNameRE = regexp.MustCompile(`(?i)(^no|from)_engine$`)
	ratchetShellAssignRE  = regexp.MustCompile(`^\s*(?:(?:local|export|readonly|declare(?:\s+-[A-Za-z]+)?)\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	ratchetShellWordRE    = regexp.MustCompile(`^(?:"[^"]*"|'[^']*'|[^\s"';&|<>()]|\$\{[^}]*\})*$`)
	ratchetShellPureRE    = regexp.MustCompile(`^\s*(?:(?:local|export|readonly|declare(?:\s+-[A-Za-z]+)?)\s+)?(?:[A-Za-z_][A-Za-z0-9_]*=(?:"[^"]*"|'[^']*'|[^\s"';&|<>()]|\$\{[^}]*\})*\s*)+(?:#.*)?$`)
	ratchetShellVarRefRE  = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)`)
	ratchetShellLiteralRE = regexp.MustCompile(`bin/metasystem\b|\bMETASYSTEM_BIN\b|\bMETASYSTEM_PROOF_AUTH_BIN\b|go run \./cmd/metasystem\b`)
)

// ratchetShellEngineAssignment reports whether an assignment's value is only
// an engine path: bin/metasystem at its end, METASYSTEM_BIN or
// METASYSTEM_PROOF_AUTH_BIN, or one engine variable, possibly with a default.
func ratchetShellEngineAssignment(value string, engines map[string]bool) bool {
	value = strings.TrimSpace(value)
	if !ratchetShellWordRE.MatchString(value) || strings.Contains(value, "$(") || strings.Contains(value, "`") {
		return false
	}
	stripped := strings.NewReplacer(`"`, "", `'`, "").Replace(value)
	if strings.HasSuffix(strings.TrimRight(stripped, "}"), "bin/metasystem") {
		return true
	}
	match := regexp.MustCompile(`^\$\{?([A-Za-z_][A-Za-z0-9_]*)(?:\}|:-(.*)\})?$`).FindStringSubmatch(stripped)
	if match == nil {
		return false
	}
	if match[1] == "METASYSTEM_BIN" || match[1] == "METASYSTEM_PROOF_AUTH_BIN" || engines[match[1]] {
		return true
	}
	return match[2] != "" && ratchetShellEngineAssignment(match[2], engines)
}

// ratchetEngineVariables returns the engine variables of one file: the seeds,
// every engine-suffixed name, and the fixpoint of that file's assignments.
// Discovery stays inside the file because scripts reuse short names (bin,
// canonical) for unrelated values.
func ratchetEngineVariables(lines []string) map[string]bool {
	engines := map[string]bool{}
	for _, name := range ratchetEngineSeedVariables {
		engines[name] = true
	}
	for _, line := range lines {
		for _, match := range ratchetShellVarRefRE.FindAllStringSubmatch(line, -1) {
			if ratchetShellNameRE.MatchString(match[1]) && !ratchetShellNotNameRE.MatchString(match[1]) {
				engines[match[1]] = true
			}
		}
		if match := ratchetShellAssignRE.FindStringSubmatch(line); match != nil && ratchetShellNameRE.MatchString(match[1]) && !ratchetShellNotNameRE.MatchString(match[1]) {
			engines[match[1]] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, line := range lines {
			match := ratchetShellAssignRE.FindStringSubmatch(line)
			if match == nil || engines[match[1]] || match[1] == "METASYSTEM_BIN" || match[1] == "METASYSTEM_PROOF_AUTH_BIN" {
				continue
			}
			if ratchetShellEngineAssignment(match[2], engines) {
				engines[match[1]] = true
				changed = true
			}
		}
	}
	return engines
}

// ratchetShellEngineLine reports whether a shell line references the engine:
// a comment or a line of only assignments does not; any other line that names
// an engine variable or an engine literal does.
func ratchetShellEngineLine(line string, engines map[string]bool) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}
	if ratchetShellPureRE.MatchString(line) && !strings.Contains(line, "$(") && !strings.Contains(line, "`") {
		return false
	}
	if ratchetShellLiteralRE.MatchString(line) {
		return true
	}
	for _, match := range ratchetShellVarRefRE.FindAllStringSubmatch(line, -1) {
		if engines[match[1]] {
			return true
		}
	}
	return false
}

// TestVerbRatchetShellEngineInvocations counts shell lines outside comments
// and pure assignments that reference the engine (R4).
func TestVerbRatchetShellEngineInvocations(t *testing.T) {
	t.Parallel()
	repository, _ := verbRatchetRoots(t)
	files := map[string][]string{}
	for _, path := range ratchetShellFiles(t, repository, ratchetShellSkipPrefixes) {
		rel, _ := filepath.Rel(repository, path)
		files[filepath.ToSlash(rel)] = readRatchetLines(t, path)
	}
	if len(files) == 0 {
		t.Fatal("found no shell files; the walk no longer reaches the scripts")
	}
	seen := map[string]bool{}
	var sites []ratchetSite
	for _, rel := range sortedKeys(files) {
		engines := ratchetEngineVariables(files[rel])
		for name := range engines {
			seen[name] = true
		}
		for i, line := range files[rel] {
			if ratchetShellEngineLine(line, engines) {
				sites = append(sites, ratchetSite{path: rel, line: i + 1, text: strings.TrimSpace(line)})
			}
		}
	}
	t.Logf("engine variables (%d): %s", len(seen), strings.Join(sortedKeys(seen), " "))
	checkVerbRatchet(t, "shell lines invoking the engine", "verbRatchetShellEngineCeiling", len(sites), verbRatchetShellEngineCeiling, sites)
}

// TestVerbRatchetShellScriptLines counts every line of the shell files under
// metasystem/scripts (R4 second ceiling).
func TestVerbRatchetShellScriptLines(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	scripts := filepath.Join(module, "scripts")
	var sites []ratchetSite
	total := 0
	for _, path := range ratchetShellFiles(t, scripts, nil) {
		rel, _ := filepath.Rel(module, path)
		count := len(readRatchetLines(t, path))
		total += count
		for i := 0; i < count; i++ {
			sites = append(sites, ratchetSite{path: filepath.ToSlash(rel), line: i + 1})
		}
	}
	if total == 0 {
		t.Fatal("found no shell lines under metasystem/scripts; the walk no longer reaches them")
	}
	checkVerbRatchet(t, "shell lines under metasystem/scripts", "verbRatchetScriptLinesCeiling", total, verbRatchetScriptLinesCeiling, sites)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

/* ----------------------------------------- 4 engine self-subprocesses -- */

// Directory names the Go and instruction scans skip under the module root.
var ratchetModuleSkipNames = []string{".git", "node_modules", "artifacts", "records", "plans", "memory", "testdata"}

// ratchetGoSkipPrefixes are module-relative packages compiled only into test
// binaries; their os.Args[0] is the test binary, not the engine.
var ratchetGoSkipPrefixes = []string{"internal/testenv", "internal/testutil"}

// ratchetEngineNameRE matches identifiers, fields and functions that name the
// engine executable when the scan cannot follow a local assignment.
var ratchetEngineNameRE = regexp.MustCompile(`(?i)^(self|exe|execpath)$|(executable|binary|engine)$`)

// ratchetForeignToolRE matches the owner of a field or method that names
// another program's executable (adapter.Binary, tool.Executable), which is
// not the engine even though the field name is.
var ratchetForeignToolRE = regexp.MustCompile(`(?i)(adapter|tool|runtime)$`)

// ratchetGoScan holds what the structural scan knows about one package: the
// functions and methods that return the engine path (by name), and, per
// top-level declaration, the local definitions.
type ratchetGoScan struct {
	funcs map[string]bool
	defs  map[string][]ast.Expr
}

// ratchetEngineFuncs returns the names of a package's functions and methods
// every return of which is the engine, iterated to a fixpoint so a function
// returning another such function's result is found too.
func ratchetEngineFuncs(files []*ast.File) map[string]bool {
	funcs := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, file := range files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || funcs[fn.Name.Name] || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
					continue
				}
				scan := ratchetGoScan{funcs: funcs, defs: ratchetLocalDefinitions(fn)}
				returns, engine := 0, true
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					if _, nested := node.(*ast.FuncLit); nested {
						return false
					}
					if ret, ok := node.(*ast.ReturnStmt); ok {
						returns++
						if len(ret.Results) == 0 || !scan.engine(ret.Results[0], 4) {
							engine = false
						}
					}
					return true
				})
				if returns > 0 && engine {
					funcs[fn.Name.Name] = true
					changed = true
				}
			}
		}
	}
	return funcs
}

// ratchetGoSelfSubprocessSites scans one parsed file structurally. A site is
// an exec.Command, exec.CommandContext, syscall.Exec or os.StartProcess call,
// or any other call shaped as a process helper f(E, "word", ...) whose word
// the engine routes (a family or a dispatchInternal top-level form), an
// exec.Cmd literal's Path, an argv literal []string{E, ...}, or a keyed
// Name/Program/Executable field, whose program expression E is the engine:
// os.Executable(), os.Args[0], a path ending in bin/metasystem, a name the
// engine regexp matches, a package function returning only the engine, or a
// local variable assigned from one of those in the same declaration. A call
// also counts, once, when two adjacent string arguments are a registered
// (family, verb) pair followed by a --flag, a non-literal, or nothing: that
// is an engine argv handed to a helper (engineVerb, batchChildRunner,
// runCaptured) whose program the scan cannot follow; path joins are not
// argv. String-slice literals
// are not read this way, because argv recognizers (killproof, toolgate,
// testpolicy) hold the same words without launching anything.
func ratchetGoSelfSubprocessSites(fset *token.FileSet, file *ast.File, rel string, funcs, routed, pairs map[string]bool) []ratchetSite {
	var sites []ratchetSite
	add := func(node ast.Node) {
		position := fset.Position(node.Pos())
		sites = append(sites, ratchetSite{path: rel, line: position.Line, text: ratchetNodeText(fset, node)})
	}
	for _, decl := range file.Decls {
		// A top-level function or a package-level declaration (whose values
		// may be function literals) is one unit for local definitions.
		scan := ratchetGoScan{funcs: funcs, defs: ratchetLocalDefinitions(decl)}
		engine := func(expr ast.Expr) bool { return scan.engine(expr, 4) }
		ast.Inspect(decl, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.CallExpr:
				program := -1
				switch ratchetQualifiedName(node.Fun) {
				case "exec.Command", "syscall.Exec", "os.StartProcess":
					program = 0
				case "exec.CommandContext":
					program = 1
				default:
					if len(node.Args) >= 2 && routed[ratchetStringLiteral(node.Args[1])] {
						program = 0
					}
				}
				joins := ratchetQualifiedName(node.Fun) == "filepath.Join" || ratchetQualifiedName(node.Fun) == "path.Join"
				if program >= 0 && len(node.Args) > program && engine(node.Args[program]) || !joins && ratchetCarriesPair(node.Args, pairs) {
					add(node)
				}
			case *ast.CompositeLit:
				if array, ok := node.Type.(*ast.ArrayType); ok && ratchetQualifiedName(array.Elt) == "string" && len(node.Elts) > 0 {
					if _, keyed := node.Elts[0].(*ast.KeyValueExpr); !keyed && engine(node.Elts[0]) {
						add(node)
					}
					return true
				}
				isCmd := ratchetQualifiedName(node.Type) == "exec.Cmd"
				for _, element := range node.Elts {
					field, ok := element.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := field.Key.(*ast.Ident)
					if !ok {
						continue
					}
					if (isCmd && key.Name == "Path" || !isCmd && (key.Name == "Name" || key.Name == "Program" || key.Name == "Executable")) && engine(field.Value) {
						add(field)
					}
				}
			}
			return true
		})
	}
	return sites
}

func ratchetNodeText(fset *token.FileSet, node ast.Node) string {
	start, end := fset.Position(node.Pos()), fset.Position(node.End())
	data, err := os.ReadFile(start.Filename)
	if err != nil || start.Offset >= len(data) || end.Offset > len(data) {
		return ""
	}
	text := string(data[start.Offset:end.Offset])
	if newline := strings.IndexByte(text, '\n'); newline >= 0 {
		text = text[:newline] + " ..."
	}
	return strings.TrimSpace(text)
}

func ratchetQualifiedName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.SelectorExpr:
		if x, ok := expr.X.(*ast.Ident); ok {
			return x.Name + "." + expr.Sel.Name
		}
	}
	return ""
}

func ratchetLastName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.SelectorExpr:
		return expr.Sel.Name
	case *ast.CallExpr:
		return ratchetLastName(expr.Fun)
	case *ast.IndexExpr:
		return ratchetLastName(expr.X)
	}
	return ""
}

// ratchetCarriesPair reports whether two adjacent expressions are string
// literals naming a registered (family, verb) pair that ends the list or is
// followed by a --flag literal or a non-literal, as an argv's verb is.
func ratchetCarriesPair(exprs []ast.Expr, pairs map[string]bool) bool {
	for i := 0; i+1 < len(exprs); i++ {
		first := ratchetStringLiteral(exprs[i])
		if first == "" || !pairs[first+" "+ratchetStringLiteral(exprs[i+1])] {
			continue
		}
		if i+2 == len(exprs) || !ratchetIsStringLit(exprs[i+2]) || strings.HasPrefix(ratchetStringLiteral(exprs[i+2]), "--") {
			return true
		}
	}
	return false
}

func ratchetIsStringLit(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}

func ratchetStringLiteral(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return value
}

func ratchetIsLiteral(expr ast.Expr, want string) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		return false
	}
	value, err := strconv.Unquote(lit.Value)
	return err == nil && value == want
}

// ratchetLocalDefinitions maps each name assigned in one top-level
// declaration (closures included) to the expressions it is assigned from.
func ratchetLocalDefinitions(decl ast.Node) map[string][]ast.Expr {
	defs := map[string][]ast.Expr{}
	bind := func(lhs []ast.Expr, rhs []ast.Expr) {
		for i, target := range lhs {
			name, ok := target.(*ast.Ident)
			if !ok || name.Name == "_" {
				continue
			}
			switch {
			case len(rhs) == len(lhs):
				defs[name.Name] = append(defs[name.Name], rhs[i])
			case len(rhs) == 1 && i == 0:
				defs[name.Name] = append(defs[name.Name], rhs[0])
			default:
				defs[name.Name] = append(defs[name.Name], nil)
			}
		}
	}
	ast.Inspect(decl, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			bind(node.Lhs, node.Rhs)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, len(node.Names))
			for i, name := range node.Names {
				lhs[i] = name
			}
			bind(lhs, node.Values)
		case *ast.RangeStmt:
			bind([]ast.Expr{node.Key, node.Value}, nil)
		}
		return true
	})
	return defs
}

func (scan ratchetGoScan) engine(expr ast.Expr, depth int) bool {
	if expr == nil || depth < 0 {
		return false
	}
	switch expr := expr.(type) {
	case *ast.ParenExpr:
		return scan.engine(expr.X, depth)
	case *ast.StarExpr:
		return scan.engine(expr.X, depth)
	case *ast.BasicLit:
		value, err := strconv.Unquote(expr.Value)
		return err == nil && strings.HasSuffix(value, "bin/metasystem")
	case *ast.BinaryExpr:
		return expr.Op == token.ADD && (scan.engine(expr.X, depth) || scan.engine(expr.Y, depth))
	case *ast.IndexExpr:
		if ratchetQualifiedName(expr.X) == "os.Args" {
			return true
		}
		return scan.engine(expr.X, depth)
	case *ast.CompositeLit:
		return len(expr.Elts) > 0 && scan.engine(expr.Elts[0], depth)
	case *ast.SelectorExpr:
		return ratchetEngineNameRE.MatchString(expr.Sel.Name) && !ratchetForeignToolRE.MatchString(ratchetLastName(expr.X))
	case *ast.Ident:
		if assigned, ok := scan.defs[expr.Name]; ok {
			for _, value := range assigned {
				if scan.engine(value, depth-1) {
					return true
				}
			}
			return false
		}
		return ratchetEngineNameRE.MatchString(expr.Name)
	case *ast.CallExpr:
		switch name := ratchetQualifiedName(expr.Fun); name {
		case "os.Executable":
			return true
		case "filepath.Join", "path.Join":
			if n := len(expr.Args); n > 0 {
				if last, ok := expr.Args[n-1].(*ast.BasicLit); ok {
					if value, err := strconv.Unquote(last.Value); err == nil && (value == "metasystem" && n >= 2 && ratchetIsLiteral(expr.Args[n-2], "bin") || strings.HasSuffix(value, "bin/metasystem")) {
						return true
					}
				}
			}
			return false
		}
		switch fun := expr.Fun.(type) {
		case *ast.Ident:
			return scan.funcs[fun.Name] || ratchetEngineNameRE.MatchString(fun.Name)
		case *ast.SelectorExpr:
			if scan.funcs[fun.Sel.Name] {
				return true
			}
			return ratchetEngineNameRE.MatchString(fun.Sel.Name) && !ratchetForeignToolRE.MatchString(ratchetLastName(fun.X))
		}
	}
	return false
}

// TestVerbRatchetEngineSelfSubprocess counts non-test Go sites that execute
// the engine itself or build an argv led by it (R5).
func TestVerbRatchetEngineSelfSubprocess(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	fset := token.NewFileSet()
	type parsed struct {
		rel  string
		file *ast.File
	}
	packages := map[string][]parsed{}
	walkRatchetFiles(t, module, ratchetModuleSkipNames, ratchetGoSkipPrefixes, func(path, rel string) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		packages[filepath.Dir(rel)] = append(packages[filepath.Dir(rel)], parsed{rel: rel, file: file})
	})
	if len(packages) == 0 {
		t.Fatal("found no Go files; the walk no longer reaches the module")
	}
	routed := ratchetRoutedWords(t)
	pairs := map[string]bool{}
	for _, fam := range families() {
		for _, v := range fam.verbs {
			pairs[fam.name+" "+v.name] = true
		}
	}
	var sites []ratchetSite
	for _, dir := range sortedKeys(packages) {
		files := make([]*ast.File, 0, len(packages[dir]))
		for _, p := range packages[dir] {
			files = append(files, p.file)
		}
		funcs := ratchetEngineFuncs(files)
		for _, p := range packages[dir] {
			sites = append(sites, ratchetGoSelfSubprocessSites(fset, p.file, p.rel, funcs, routed, pairs)...)
		}
	}
	checkVerbRatchet(t, "engine self-subprocess sites", "verbRatchetSelfSubprocessCeiling", len(sites), verbRatchetSelfSubprocessCeiling, sites)
}

/* ------------------------------------ 5 instructions naming non-public -- */

// ratchetInstructionRE finds `metasystem W W` and `bin/metasystem W W`.
// metasystem must stand alone (not the tail of a path or identifier) unless
// bin/ precedes it.
var ratchetInstructionRE = regexp.MustCompile(`(?:bin/|(?:^|[^A-Za-z0-9_./-]))metasystem ([a-z][a-z0-9-]*) ([a-z][a-z0-9-]*)`)

// ratchetInstructionSources lists the module-relative text the R6 scan reads:
// every .md under the module outside records/, plans/ and memory/; skills,
// optional skills and role packets whatever their text format; the UI source.
func ratchetInstructionSource(rel string) bool {
	switch {
	case strings.HasSuffix(rel, ".md"):
		return true
	case strings.HasPrefix(rel, "skills/"), strings.HasPrefix(rel, "optional-skills/"), strings.HasPrefix(rel, "scripts/agents/roles/"):
		return strings.HasSuffix(rel, ".json") || strings.HasSuffix(rel, ".yaml") || strings.HasSuffix(rel, ".yml")
	case strings.HasPrefix(rel, "internal/ui/web/_app/src/"):
		return strings.HasSuffix(rel, ".ts") || strings.HasSuffix(rel, ".tsx")
	}
	return false
}

// ratchetGoStringLiterals returns each string literal of a Go file with its line.
func ratchetGoStringLiterals(t *testing.T, path string) []ratchetSite {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file := fset.AddFile(path, -1, len(src))
	var s scanner.Scanner
	s.Init(file, src, func(pos token.Position, msg string) { t.Fatalf("%s: %s", pos, msg) }, 0)
	var literals []ratchetSite
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return literals
		}
		if tok != token.STRING {
			continue
		}
		value, err := strconv.Unquote(lit)
		if err != nil {
			t.Fatalf("%s: %v", fset.Position(pos), err)
		}
		literals = append(literals, ratchetSite{line: fset.Position(pos).Line, text: value})
	}
}

// TestVerbRatchetInstructionsNameNonPublicForms counts instruction text that
// names `metasystem W W` whose first word the engine routes today (a family
// or a dispatchInternal top-level form) and is not a public command, help, or
// internal (R6).
func TestVerbRatchetInstructionsNameNonPublicForms(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	allowed := map[string]bool{"help": true, "internal": true}
	for _, command := range publicIntentCommands() {
		allowed[command.name] = true
	}
	// Only a word the engine routes makes a form; "the metasystem is ..."
	// and other prose never reach the router.
	routed := ratchetRoutedWords(t)
	var sites []ratchetSite
	record := func(rel string, line int, text string) {
		for _, match := range ratchetInstructionRE.FindAllStringSubmatch(text, -1) {
			if routed[match[1]] && !allowed[match[1]] {
				sites = append(sites, ratchetSite{path: rel, line: line, text: strings.TrimLeft(match[0], " `'\"(")})
			}
		}
	}
	walkRatchetFiles(t, module, ratchetModuleSkipNames, nil, func(path, rel string) {
		switch {
		case strings.HasSuffix(rel, ".go"):
			if strings.HasSuffix(rel, "_test.go") {
				return
			}
			for _, literal := range ratchetGoStringLiterals(t, path) {
				for offset, line := range strings.Split(literal.text, "\n") {
					record(rel, literal.line+offset, line)
				}
			}
		case ratchetInstructionSource(rel):
			for i, line := range readRatchetLines(t, path) {
				record(rel, i+1, line)
			}
		}
	})
	checkVerbRatchet(t, "instructions naming non-public forms", "verbRatchetInstructionCeiling", len(sites), verbRatchetInstructionCeiling, sites)
}
