package main

// Ratchet witnesses for the object-action verb redesign
// (plans/designs/verbs-object-action.md, section 4, unit U0). Each test
// measures one number the redesign must drive down and holds it at a ceiling
// equal to the value measured when the witness landed. A change that raises a
// number fails and names the sites; a change that lowers one passes and logs
// the constant to lower, so the next commit can tighten the ratchet. The tests
// walk the filesystem and never call Git.

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
	// The launch contract raised it by one: `app serve` is the supervisor that
	// owns one run of the project's application for its life, in the shape of
	// `ui serve`, and like it it is a process entrypoint a person never types.
	verbRatchetInternalVerbCeiling = 289
	// R4: shell lines that reference the engine.
	verbRatchetShellEngineCeiling = 131
	// R4 second ceiling: all lines of shell files under metasystem/scripts.
	verbRatchetScriptLinesCeiling = 2133
	// R5: non-test Go sites that run or build an argv for the engine itself,
	// and every call of a launcher helper (see section 4 for what is followed).
	// U6a raised it by the process boundaries that were shell before: the
	// mission host turn's delegate-supervisor entry launch, the fake runtime's
	// fixture holds (its CLI stand-in children and the host hold exec, which
	// counts twice: its argv and its exec), and the fake self-test's delegate
	// children, launched through engineDelegate with the running binary.
	// Batch 2 (U6a+U6b) raised it again: with dispatch.sh gone, the
	// supervisor's lifecycle callbacks (__record-cas, __handshake,
	// __register-custody, __protocol-error, __repair-claim, __cancel-owned,
	// the self-test's status and reap) exec the engine's delegate entry
	// directly, where they used to exec dispatch.sh, and the lifecycle
	// launches the delegate-supervisor entry where it ran an adapter script.
	// The hops existed before; the witness now sees them.
	// The launch contract raised it by two, both real process boundaries of
	// its design: `app start` launches the engine's own `app serve`
	// supervisor detached (app.go), and `app check` bridges to the testing
	// contract's own runner instead of running a test itself (intent_app.go).
	// U5 added one: the Go landing path runs the engine's landing workspace
	// (landing_path.go), a hop commit.sh made from shell.
	verbRatchetSelfSubprocessCeiling = 128
	// R6: instruction text naming a form whose first word is not public, over
	// the vocabulary frozen when the witness landed.
	verbRatchetInstructionCeiling = 46
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

// checkVerbRatchet fails only when the measured number is above its ceiling,
// naming the sites; below it, it logs which constant to lower.
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
		t.Logf("%s is %d, below its ceiling %d: lower %s to %d to tighten the ratchet", number, measured, ceiling, constant, measured)
	}
}

// ratchetSiteListLimit bounds the failure message: above it only the per-file
// counts are printed.
const ratchetSiteListLimit = 200

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
	ratchetShellLiteralRE = regexp.MustCompile(`bin/metasystem(?:$|[^A-Za-z0-9_-])|\bMETASYSTEM_BIN\b|\bMETASYSTEM_PROOF_AUTH_BIN\b|go run \./cmd/metasystem\b`)
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

// R5 scope. The scan is structural and per package. It follows: local
// assignments within one top-level declaration; package-level var and const
// initialisers (var self = os.Args[0], var launch = os.Executable); package
// functions every return of which is the engine; and launcher helpers, found
// by a package-level fixpoint (ratchetLaunchers), so a launch through
// engineVerb, a subprocess field, batchDiagnosticExecute,
// newBrainBootInputsCommand or a local run closure counts at every call
// whatever the shape of its argv. It also counts a call of a function-typed
// parameter handed the engine (execute(binary, args, ...)).
//
// It does not follow: calls into a launcher defined in another package
// (only the launcher's own body counts there); launchers stored in maps,
// slices or interfaces, or reached through method values of another type
// that share a launcher's name only by accident (names are matched per
// package, so such a collision over-counts rather than hides); a launcher
// whose argv is built in a statement other than the one that names the
// engine; a function-typed parameter called without the engine among its
// arguments; engines found through PATH (a bare "metasystem" program name);
// and wrappers whose program is decided at run time from data.

// Directory names the Go and instruction scans skip under the module root.
var ratchetModuleSkipNames = []string{".git", "node_modules", "artifacts", "records", "plans", "memory", "testdata"}

// ratchetGoSkipPrefixes are module-relative packages compiled only into test
// binaries; their os.Args[0] is the test binary, not the engine.
var ratchetGoSkipPrefixes = []string{"internal/testenv", "internal/testutil"}

// ratchetBootstrapPrefixes is the Go bootstrap (design 3.3): a program of its
// own, run as `go run ./cmd/devgate` from the tree it builds. It is not the
// engine, so R5 does not count it: its engine calls cross a binary boundary by
// design (it builds the engine, launches the proof owner from that build, and
// asks the trusted engine the `proof-run worker-authorized` entry).
var ratchetBootstrapPrefixes = []string{"cmd/devgate"}

// ratchetEngineNameRE matches identifiers, fields and functions that name the
// engine executable when the scan cannot follow a local assignment.
var ratchetEngineNameRE = regexp.MustCompile(`(?i)^(self|exe|execpath)$|(executable|binary|engine)$`)

// ratchetForeignToolRE matches the owner of a field or method that names
// another program's executable (adapter.Binary, tool.Executable), which is
// not the engine even though the field name is.
var ratchetForeignToolRE = regexp.MustCompile(`(?i)(adapter|tool|runtime)$`)

// ratchetGoScan holds what the structural scan knows about one package: the
// functions and methods that return the engine path (by name), the
// package-level var and const initialisers, and, per top-level declaration,
// the local definitions.
type ratchetGoScan struct {
	funcs map[string]bool
	pkg   map[string][]ast.Expr
	defs  map[string][]ast.Expr
}

// ratchetPackageDefinitions maps each package-level var or const name to its
// initialisers.
func ratchetPackageDefinitions(files []*ast.File) map[string][]ast.Expr {
	defs := map[string][]ast.Expr{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR && gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range value.Names {
					switch {
					case len(value.Values) == len(value.Names):
						defs[name.Name] = append(defs[name.Name], value.Values[i])
					case len(value.Values) == 1 && i == 0:
						defs[name.Name] = append(defs[name.Name], value.Values[0])
					default:
						defs[name.Name] = append(defs[name.Name], nil)
					}
				}
			}
		}
	}
	return defs
}

// ratchetEngineFuncs returns the names of a package's functions and methods
// every return of which is the engine, iterated to a fixpoint so a function
// returning another such function's result is found too.
func ratchetEngineFuncs(files []*ast.File, pkg map[string][]ast.Expr) map[string]bool {
	funcs := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, file := range files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || funcs[fn.Name.Name] || fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
					continue
				}
				scan := ratchetGoScan{funcs: funcs, pkg: pkg, defs: ratchetLocalDefinitions(fn)}
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

// ratchetLauncher is a helper whose calls launch a process. A negative
// program means the helper supplies the engine itself and forwards a
// caller's argv, so every call counts; otherwise program is the index of the
// argument carrying the program (or an argv led by it), and a call counts
// when that argument is the engine.
type ratchetLauncher struct{ program int }

// ratchetUnit is one named function body the launcher fixpoint examines: a
// function or method, a package-level var bound to a function literal, a
// field assigned or keyed to one (subprocess, Rebind), or a closure bound
// inside one top-level declaration (local, visible only there).
type ratchetUnit struct {
	name  string
	typ   *ast.FuncType
	body  *ast.BlockStmt
	decl  ast.Decl
	local bool
}

// ratchetLaunchers holds one package's launchers: package-scope names,
// closures per declaration, and aliases (var batchDiagnosticRunner =
// runBatchDiagnostic) resolved to their targets.
type ratchetLaunchers struct {
	pkg     map[string]ratchetLauncher
	local   map[ast.Decl]map[string]ratchetLauncher
	shadow  map[ast.Decl]map[string]bool
	aliases map[string]string
}

func (l *ratchetLaunchers) resolve(fun ast.Expr, decl ast.Decl) (ratchetLauncher, bool) {
	var name string
	switch fun := fun.(type) {
	case *ast.Ident:
		name = fun.Name
		if l.shadow[decl][name] {
			launcher, ok := l.local[decl][name]
			return launcher, ok
		}
	case *ast.SelectorExpr:
		name = fun.Sel.Name
	case *ast.ParenExpr:
		return l.resolve(fun.X, decl)
	default:
		return ratchetLauncher{}, false
	}
	for hops := 0; hops < 8; hops++ {
		if launcher, ok := l.pkg[name]; ok {
			return launcher, true
		}
		target, ok := l.aliases[name]
		if !ok {
			break
		}
		name = target
	}
	return ratchetLauncher{}, false
}

// ratchetLauncherUnits collects a package's named function bodies and its
// package-level aliases of function names.
func ratchetLauncherUnits(files []*ast.File) ([]ratchetUnit, map[string]string, map[ast.Decl]map[string]bool) {
	packageVars := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				for _, spec := range gen.Specs {
					for _, name := range spec.(*ast.ValueSpec).Names {
						packageVars[name.Name] = true
					}
				}
			}
		}
	}
	var units []ratchetUnit
	aliases := map[string]string{}
	shadow := map[ast.Decl]map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			decl := decl
			topSpecs := map[ast.Spec]bool{}
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Body != nil {
					units = append(units, ratchetUnit{name: decl.Name.Name, typ: decl.Type, body: decl.Body, decl: decl})
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					topSpecs[spec] = true
				}
			}
			bindLocal := func(name string, lit *ast.FuncLit) {
				if shadow[decl] == nil {
					shadow[decl] = map[string]bool{}
				}
				shadow[decl][name] = true
				units = append(units, ratchetUnit{name: name, typ: lit.Type, body: lit.Body, decl: decl, local: true})
			}
			bind := func(target ast.Expr, value ast.Expr, local bool) {
				lit, isLit := value.(*ast.FuncLit)
				switch target := target.(type) {
				case *ast.Ident:
					if target.Name == "_" {
						return
					}
					if isLit && local {
						bindLocal(target.Name, lit)
					} else if isLit {
						units = append(units, ratchetUnit{name: target.Name, typ: lit.Type, body: lit.Body, decl: decl})
					} else if !local {
						if alias := ratchetQualifiedName(value); alias != "" && !strings.Contains(alias, ".") {
							aliases[target.Name] = alias
						}
					}
				case *ast.SelectorExpr:
					if isLit {
						units = append(units, ratchetUnit{name: target.Sel.Name, typ: lit.Type, body: lit.Body, decl: decl})
					}
				}
			}
			ast.Inspect(decl, func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.ValueSpec:
					if len(node.Values) == len(node.Names) {
						for i, name := range node.Names {
							bind(name, node.Values[i], !topSpecs[node])
						}
					}
				case *ast.AssignStmt:
					if len(node.Lhs) == len(node.Rhs) {
						for i, target := range node.Lhs {
							ident, isIdent := target.(*ast.Ident)
							local := isIdent && (node.Tok == token.DEFINE || !packageVars[ident.Name])
							bind(target, node.Rhs[i], local)
						}
					}
				case *ast.KeyValueExpr:
					if key, ok := node.Key.(*ast.Ident); ok {
						if lit, ok := node.Value.(*ast.FuncLit); ok {
							units = append(units, ratchetUnit{name: key.Name, typ: lit.Type, body: lit.Body, decl: decl})
						}
					}
				}
				return true
			})
		}
	}
	return units, aliases, shadow
}

// ratchetInspect walks root like ast.Inspect, handing each node its
// ancestors (outermost first).
func ratchetInspect(root ast.Node, visit func(node ast.Node, stack []ast.Node)) {
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		visit(node, stack)
		stack = append(stack, node)
		return true
	})
}

// ratchetSiteScanner decides, for one package, which nodes are engine
// self-subprocess sites.
type ratchetSiteScanner struct {
	funcs, routed, pairs map[string]bool
	pkg                  map[string][]ast.Expr
	launchers            *ratchetLaunchers
}

// ratchetProgramArgument returns the index of a call's program argument when
// the call launches a process: an exec-family call or a launcher that takes
// its program from the caller.
func (s *ratchetSiteScanner) ratchetProgramArgument(call *ast.CallExpr, decl ast.Decl) int {
	switch ratchetQualifiedName(call.Fun) {
	case "exec.Command", "syscall.Exec", "os.StartProcess":
		return 0
	case "exec.CommandContext":
		return 1
	}
	if launcher, ok := s.launchers.resolve(call.Fun, decl); ok && launcher.program >= 0 {
		return launcher.program
	}
	return -1
}

// sites reports every site under root (a top-level declaration or a part of
// decl) with its ancestors. A site is: an exec-family call, a call to a
// launcher, or any other call shaped as a process helper f(E, "word", ...)
// whose word the engine routes, whose program E is the engine; every call of
// a launcher that supplies the engine itself; a call of a function-typed
// parameter handed the engine; an exec.Cmd literal's Path, an argv literal
// []string{E, ...}, or a keyed Name/Program/Executable field that is the
// engine. The engine is os.Executable(), os.Args[0], a path ending in
// bin/metasystem, a name the engine regexp matches, a package function
// returning only the engine, or a local or package-level variable assigned
// from one of those. A call also counts, once, when two adjacent string
// arguments are a registered (family, verb) pair followed by a --flag, a
// non-literal, or nothing; path joins are not argv. String-slice literals are
// not read that way, because argv recognizers (killproof, toolgate,
// testpolicy) hold the same words without launching anything.
func (s *ratchetSiteScanner) sites(root ast.Node, decl ast.Decl, visit func(node ast.Node, stack []ast.Node)) {
	scan := ratchetGoScan{funcs: s.funcs, pkg: s.pkg, defs: ratchetLocalDefinitions(decl)}
	engine := func(expr ast.Expr) bool { return scan.engine(expr, 4) }
	ratchetInspect(root, func(node ast.Node, stack []ast.Node) {
		switch node := node.(type) {
		case *ast.CallExpr:
			site := false
			if program := s.ratchetProgramArgument(node, decl); program >= 0 {
				site = len(node.Args) > program && engine(node.Args[program])
			} else if launcher, ok := s.launchers.resolve(node.Fun, decl); ok && launcher.program < 0 {
				site = true
			} else if len(node.Args) >= 2 && s.routed[ratchetStringLiteral(node.Args[1])] {
				site = engine(node.Args[0])
			}
			if !site && ratchetFuncParameter(node.Fun, stack) {
				for _, arg := range node.Args {
					if engine(arg) {
						site = true
						break
					}
				}
			}
			joins := ratchetQualifiedName(node.Fun) == "filepath.Join" || ratchetQualifiedName(node.Fun) == "path.Join"
			if site || !joins && ratchetCarriesPair(node.Args, s.pairs) {
				visit(node, stack)
			}
		case *ast.CompositeLit:
			if array, ok := node.Type.(*ast.ArrayType); ok && ratchetQualifiedName(array.Elt) == "string" && len(node.Elts) > 0 {
				if _, keyed := node.Elts[0].(*ast.KeyValueExpr); !keyed && engine(node.Elts[0]) {
					visit(node, stack)
				}
				return
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
					visit(field, append(stack, node))
				}
			}
		}
	})
}

// ratchetFuncParameter reports whether fun names a function-typed parameter
// of an enclosing function or function literal.
func ratchetFuncParameter(fun ast.Expr, stack []ast.Node) bool {
	ident, ok := fun.(*ast.Ident)
	if !ok {
		return false
	}
	for i := len(stack) - 1; i >= 0; i-- {
		var typ *ast.FuncType
		switch node := stack[i].(type) {
		case *ast.FuncDecl:
			typ = node.Type
		case *ast.FuncLit:
			typ = node.Type
		default:
			continue
		}
		for _, field := range typ.Params.List {
			for _, name := range field.Names {
				if name.Name == ident.Name {
					_, isFunc := field.Type.(*ast.FuncType)
					return isFunc
				}
			}
		}
	}
	return false
}

// ratchetParameterIndex returns the index of the parameter an expression
// reads (p, p[0], p.field, p.field[0]).
func ratchetParameterIndex(expr ast.Expr, params map[string]int) (int, bool) {
	switch expr := expr.(type) {
	case *ast.Ident:
		index, ok := params[expr.Name]
		return index, ok
	case *ast.IndexExpr:
		return ratchetParameterIndex(expr.X, params)
	case *ast.SelectorExpr:
		return ratchetParameterIndex(expr.X, params)
	case *ast.ParenExpr:
		return ratchetParameterIndex(expr.X, params)
	case *ast.StarExpr:
		return ratchetParameterIndex(expr.X, params)
	}
	return 0, false
}

// classify decides whether one unit is a launcher under the launchers known
// so far. A unit whose process launch takes its program from a parameter is
// a program launcher at that parameter. A unit holding an engine site whose
// statement also reads one of its []string or ...string parameters forwards
// a caller's argv to the engine, so every call of it is a launch.
func (s *ratchetSiteScanner) classify(unit ratchetUnit) (ratchetLauncher, bool) {
	params := map[string]int{}
	slices := map[string]bool{}
	index := 0
	for _, field := range unit.typ.Params.List {
		_, variadic := field.Type.(*ast.Ellipsis)
		array, isArray := field.Type.(*ast.ArrayType)
		slice := variadic && ratchetQualifiedName(field.Type.(*ast.Ellipsis).Elt) == "string" ||
			isArray && array.Len == nil && ratchetQualifiedName(array.Elt) == "string"
		if len(field.Names) == 0 {
			index++
			continue
		}
		for _, name := range field.Names {
			params[name.Name] = index
			slices[name.Name] = slice
			index++
		}
	}
	program := -1
	ast.Inspect(unit.body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested || program >= 0 {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if argument := s.ratchetProgramArgument(call, unit.decl); argument >= 0 && len(call.Args) > argument {
			if parameter, ok := ratchetParameterIndex(call.Args[argument], params); ok {
				program = parameter
			}
		}
		return true
	})
	if program >= 0 {
		return ratchetLauncher{program: program}, true
	}
	forwards := false
	s.sites(unit.body, unit.decl, func(node ast.Node, stack []ast.Node) {
		if forwards {
			return
		}
		statement := node
		for i := len(stack) - 1; i >= 0; i-- {
			if _, ok := stack[i].(ast.Stmt); ok {
				if _, block := stack[i].(*ast.BlockStmt); !block {
					statement = stack[i]
					break
				}
			}
		}
		ast.Inspect(statement, func(inner ast.Node) bool {
			if ident, ok := inner.(*ast.Ident); ok && slices[ident.Name] {
				forwards = true
			}
			return !forwards
		})
	})
	return ratchetLauncher{program: -1}, forwards
}

// ratchetFindLaunchers iterates classify over a package's units to a
// fixpoint, so a helper calling another launcher with its caller's argv is a
// launcher too.
func ratchetFindLaunchers(files []*ast.File, scanner *ratchetSiteScanner) {
	units, aliases, shadow := ratchetLauncherUnits(files)
	scanner.launchers = &ratchetLaunchers{
		pkg:     map[string]ratchetLauncher{},
		local:   map[ast.Decl]map[string]ratchetLauncher{},
		shadow:  shadow,
		aliases: aliases,
	}
	found := map[int]bool{}
	for changed := true; changed; {
		changed = false
		for i, unit := range units {
			if found[i] {
				continue
			}
			launcher, ok := scanner.classify(unit)
			if !ok {
				continue
			}
			found[i], changed = true, true
			if unit.local {
				if scanner.launchers.local[unit.decl] == nil {
					scanner.launchers.local[unit.decl] = map[string]ratchetLauncher{}
				}
				scanner.launchers.local[unit.decl][unit.name] = launcher
			} else if _, known := scanner.launchers.pkg[unit.name]; !known {
				scanner.launchers.pkg[unit.name] = launcher
			}
		}
	}
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
			case len(rhs) == 1 && i == 0 && len(lhs) == 2:
				// v, err := f(): the first result is f's value.
				defs[name.Name] = append(defs[name.Name], rhs[0])
			case len(rhs) == 1 && len(lhs) > 2:
				// A wider tuple (argv, _, _, err :=
				// procArgsAndExecutable(pid)) is not one path; its names
				// are judged by name alone.
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
		assigned, ok := scan.defs[expr.Name]
		if !ok {
			assigned, ok = scan.pkg[expr.Name]
		}
		if ok {
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
		case "append":
			return len(expr.Args) > 0 && scan.engine(expr.Args[0], depth)
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
			if scan.funcs[fun.Name] || ratchetEngineNameRE.MatchString(fun.Name) {
				return true
			}
			// A package variable holding an engine function value
			// (var launchExecutable = os.Executable).
			if _, local := scan.defs[fun.Name]; !local {
				for _, value := range scan.pkg[fun.Name] {
					if name := ratchetQualifiedName(value); name == "os.Executable" || name != "" && scan.funcs[name] {
						return true
					}
				}
			}
			return false
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
// the engine itself, build an argv led by it, or call a launcher helper (R5).
func TestVerbRatchetEngineSelfSubprocess(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	fset := token.NewFileSet()
	type parsed struct {
		rel  string
		file *ast.File
	}
	packages := map[string][]parsed{}
	walkRatchetFiles(t, module, ratchetModuleSkipNames, append(append([]string(nil), ratchetGoSkipPrefixes...), ratchetBootstrapPrefixes...), func(path, rel string) {
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
	var launchers []string
	for _, dir := range sortedKeys(packages) {
		files := make([]*ast.File, 0, len(packages[dir]))
		for _, p := range packages[dir] {
			files = append(files, p.file)
		}
		pkg := ratchetPackageDefinitions(files)
		scanner := &ratchetSiteScanner{funcs: ratchetEngineFuncs(files, pkg), routed: routed, pairs: pairs, pkg: pkg}
		ratchetFindLaunchers(files, scanner)
		for name, launcher := range scanner.launchers.pkg {
			launchers = append(launchers, fmt.Sprintf("%s.%s(%d)", dir, name, launcher.program))
		}
		for _, closures := range scanner.launchers.local {
			for name, launcher := range closures {
				launchers = append(launchers, fmt.Sprintf("%s.<closure %s>(%d)", dir, name, launcher.program))
			}
		}
		for _, p := range packages[dir] {
			for _, decl := range p.file.Decls {
				scanner.sites(decl, decl, func(node ast.Node, _ []ast.Node) {
					position := fset.Position(node.Pos())
					sites = append(sites, ratchetSite{path: p.rel, line: position.Line, text: ratchetNodeText(fset, node)})
				})
			}
		}
	}
	sort.Strings(launchers)
	t.Logf("launchers (%d; -1 supplies the engine, n takes the program at argument n): %s", len(launchers), strings.Join(launchers, " "))
	checkVerbRatchet(t, "engine self-subprocess sites", "verbRatchetSelfSubprocessCeiling", len(sites), verbRatchetSelfSubprocessCeiling, sites)
}

/* ------------------------------------ 5 instructions naming non-public -- */

// ratchetFrozenFirstWords is the internal vocabulary routed when the witness
// landed: the 45 family names and dispatchInternal's 8 top-level forms. R6
// counts these whether or not they are still routed, so deleting a family
// never relaxes the ratchet by making its stale instructions invisible.
var ratchetFrozenFirstWords = []string{
	// families, in alphabetical order
	"acp", "adapter", "audit", "behavior-surface", "brain", "channel", "config", "context",
	"counselor", "covenant", "event", "evidence", "gate", "goal", "hooks", "host", "janitor",
	"job", "json", "landing", "launch", "lease", "metrics", "mission", "output", "path", "proc",
	"project", "proof-run", "receipt", "report", "run", "runtime", "schema", "seat", "session",
	"steward", "stopfence", "supervise", "test", "testing", "ui", "unit", "util", "validate",
	// dispatchInternal top-level forms, in alphabetical order
	"arm", "delegate", "health", "status", "stop", "up", "wait", "watch",
}

// ratchetAgentInstructionDirs are repository-relative directories of agent
// instruction text outside the module; every regular file under them is read.
var ratchetAgentInstructionDirs = []string{".claude/agents", ".devin/agents"}

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
// names `metasystem W W` whose first word is in the frozen vocabulary or the
// engine routes today (a family or a dispatchInternal top-level form) and is
// not a public command, help, or internal (R6).
func TestVerbRatchetInstructionsNameNonPublicForms(t *testing.T) {
	t.Parallel()
	repository, module := verbRatchetRoots(t)
	allowed := map[string]bool{"help": true, "internal": true, "status": true}
	publicPairs := map[string]bool{}
	for _, command := range publicIntentCommands() {
		publicPairs[command.name] = true
	}
	// Only a word the engine routes, or routed when the witness landed, makes
	// a form; "the metasystem is ..." and other prose never reach the router.
	routed := ratchetRoutedWords(t)
	for _, word := range ratchetFrozenFirstWords {
		routed[word] = true
	}
	var sites []ratchetSite
	record := func(rel string, line int, text string) {
		for _, match := range ratchetInstructionRE.FindAllStringSubmatch(text, -1) {
			if routed[match[1]] && !allowed[match[1]] && !publicPairs[match[1]+" "+match[2]] {
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
	for _, dir := range ratchetAgentInstructionDirs {
		root := filepath.Join(repository, filepath.FromSlash(dir))
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("agent instruction directory %s is unreadable: %v", dir, err)
		}
		walkRatchetFiles(t, root, nil, nil, func(path, rel string) {
			for i, line := range readRatchetLines(t, path) {
				record(dir+"/"+rel, i+1, line)
			}
		})
	}
	checkVerbRatchet(t, "instructions naming non-public forms", "verbRatchetInstructionCeiling", len(sites), verbRatchetInstructionCeiling, sites)
}
