package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
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

// The traced reading of "Messages a Person Reads" (round 2). The direct scan
// of TestAuditMessagesAPersonReads reads a message where it is written; this
// one follows a message to where its words are: into the helper that builds a
// refusal from a format string, into the function or local that assembles a
// line and hands it on, into a constant, onto streams the direct scan does
// not name, into hook responses, browser refusals and an error type's Error
// text. Everything it finds is reported; a group enforces its paths with
// enforceTracedMessages once it has rewritten them.

// The trace classes: which blind spot of the direct scan a source was in.
const (
	messageTraceHelper      = "helper-format" // a literal handed to a helper that builds a refusal or error from it
	messageTraceAssembled   = "assembled"     // a text a function returns or a local assembles, then shown
	messageTraceConstant    = "constant"      // a string constant written into a message
	messageTraceNotice      = "notice"        // a progress or notice line on a stream the direct scan does not name
	messageTraceHook        = "hook"          // a Stop or SessionStart systemMessage, stopReason or block reason
	messageTraceUI          = "ui"            // a refusal or error the browser interface shows
	messageTraceErrorMethod = "error-method"  // an error type's Error text
)

var messageTraceClasses = []string{messageTraceHelper, messageTraceAssembled, messageTraceConstant, messageTraceNotice,
	messageTraceHook, messageTraceUI, messageTraceErrorMethod}

// messageTraced is one source the traced reading found.
type messageTraced struct {
	messageSource
	Trace string `json:"trace"`
}

// messageTracedModes holds the traced paths a group has rewritten; like the
// messages column of auditModes, the longest key naming a source wins.
var messageTracedModes = map[string]string{}

// enforceTracedMessages moves paths to enforce mode for the traced reading.
func enforceTracedMessages(paths ...string) bool {
	for _, path := range paths {
		messageTracedModes[path] = auditEnforce
	}
	return true
}

// messageTraceKept are the files (or file#Function) whose texts are machine
// protocol a parent process reads by its leading code (the kept exceptions
// of the direct scan), or records a parser reads back; their sources are
// listed as kept, never reported.
var messageTraceKept = map[string]string{
	"cmd/metasystem/proof_run_protocol.go": "proof-run protocol codes a parent reads",
	"internal/proofrun/protocol.go":        "proof-run protocol codes a parent reads",
	"internal/testrun/protocol.go":         "test-run protocol codes a parent reads",
	// A mission ledger's cycle and reset lines are the record its parser
	// (ParseLedger, the prompt's cycle table) reads back, not a message.
	"internal/mission/ledger.go#AppendCycle": "mission ledger record lines its parser reads",
	"internal/mission/ledger.go#AppendReset": "mission ledger record lines its parser reads",
}

// messageTraceKeptCodes are protocol codes kept in a person's message by
// decision: the code is not reported, the rest of the text is.
// BUDGET_REFUSED leads a test run's cold-build refusal because the landing
// batch owner reads it to tell a budget refusal from the others;
// BRIEF_BOUNDS_UNREADABLE is the refusal register's anchor at its Error.
var messageTraceKeptCodes = map[string]string{"CONTEXT_CONFIG_INVALID": "internal/config/context.go",
	"BUDGET_REFUSED": "cmd/metasystem/test_cold_budget.go", "BRIEF_BOUNDS_UNREADABLE": "internal/validate/brief_bounds_source.go"}

// messageTraceFixtureDir names the packages that serve tests (fakes,
// fixtures, the interface walkthrough): nothing a person reads in use.
var messageTraceFixtureDir = regexp.MustCompile(`(^|/)(fake[a-z]*|fixture[a-z]*|test(util|git|goal)|[a-z]+test|walkthrough)$`)

// messageTraceWords is the shape of a message: two words. An id, a path, a
// code or a key has none, so the traced reading skips it.
var messageTraceWords = regexp.MustCompile(`[A-Za-z]{2,}[,:;]? +[A-Za-z(…'"]`)

// messageTraceWriter names the person's streams the direct scan's
// messageWriter misses: notes, answers and error outputs.
var messageTraceWriter = regexp.MustCompile(`noteStream|answer(Errors|Output)\(\)|combinedErr|errOut|[Ee]rrorOutput|\bnotes\b|notice\.w|seams\.Out`)

// messageTraceField is an internal key=value pair in default text
// ("ours=… engine=…"): what --verbose or --json shows, never line 1.
var messageTraceField = regexp.MustCompile(`\b[a-z][a-z0-9_-]*=` + messageHole)

// messageTraceFields are the fields of an error or refusal type that hold
// its words; messageTraceHookFields hold a hook's words on any type.
var (
	messageTraceFields     = map[string]bool{"Message": true, "Reason": true, "Summary": true, "Sentence": true, "Words": true, "Detail": true, "Details": true, "Remedy": true, "Hint": true, "Why": true, "Decision": true, "Problem": true}
	messageTraceHookFields = map[string]bool{"SystemMessage": true, "HumanLine": true, "StopReason": true}
	messageTraceHookKeys   = map[string]bool{"systemMessage": true, "stopReason": true}
	messageTraceHookDirs   = regexp.MustCompile(`^internal/(hooks|report|stopreport|adapter)(/|$)`)
	messageTraceJSONHook   = regexp.MustCompile(`"(systemMessage|stopReason)":"((?:[^"\\]|\\.)*)"?|^(systemMessage)=(.*)$`)
)

// traceFile is one parsed production file.
type traceFile struct {
	rel, dir string
	fset     *token.FileSet
	file     *ast.File
	data     []byte
	imports  map[string]string // local package name -> module-relative dir
}

// traceFunc is one function or method with a body.
type traceFunc struct {
	file    *traceFile
	decl    *ast.FuncDecl
	name    string   // Name, or Recv.Name
	params  []string // flattened parameter names
	strings []bool   // whether each parameter is a string
	result  int      // index of the one string result, or -1
	locals  map[string][]ast.Expr
	// messageParams are the parameters whose value reaches a message.
	messageParams map[int]traceParam
}

type traceParam struct {
	format  bool
	class   string
	kind    string
	refusal bool
}

// tracePackage is what the traced reading knows of one package directory.
type tracePackage struct {
	consts   map[string]ast.Expr
	funcs    map[string]*traceFunc
	methods  map[string][]*traceFunc
	structs  map[string][]string
	refusals map[string]bool // types with a RefusalCode method
}

// traceSink is one expression whose value a person reads.
type traceSink struct {
	fn     *traceFunc
	file   *traceFile
	expr   ast.Expr
	kind   string
	format bool
	class  string
	// refusal: a refusal type with no field for its remedy carries these
	// words, so line 2 must be in them.
	refusal bool
	// pick transforms the text before it is judged (a hook's JSON value).
	pick func(string) (string, bool)
}

type traceIndex struct {
	modulePath string
	files      []*traceFile
	packages   map[string]*tracePackage
	funcOf     map[ast.Node]*traceFunc
	sites      map[string]bool // call-site arguments already made sinks
}

// messageTraceScan reads the module's production files and returns the
// messages the direct scan (direct) did not already hold.
func messageTraceScan(t *testing.T, module string, direct []messageSource) []messageTraced {
	t.Helper()
	index, err := messageTraceIndex(module)
	if err != nil {
		t.Fatal(err)
	}
	sinks := index.directSinks()
	sinks = index.close(sinks)
	seen := map[string]bool{}
	for _, source := range direct {
		seen[fmt.Sprintf("%s:%d:%s", source.File, source.Line, source.Text)] = true
	}
	var traced []messageTraced
	for _, sink := range sinks {
		for _, found := range index.emit(sink) {
			key := fmt.Sprintf("%s:%d:%s", found.File, found.Line, found.Text)
			if seen[key] {
				continue
			}
			seen[key] = true
			traced = append(traced, found)
		}
	}
	sort.Slice(traced, func(i, j int) bool {
		if traced[i].File != traced[j].File {
			return traced[i].File < traced[j].File
		}
		return traced[i].Line < traced[j].Line
	})
	return traced
}

func messageTraceIndex(module string) (*traceIndex, error) {
	index := &traceIndex{packages: map[string]*tracePackage{}, funcOf: map[ast.Node]*traceFunc{}, sites: map[string]bool{}}
	if data, err := os.ReadFile(filepath.Join(module, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if path, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
				index.modulePath = strings.TrimSpace(path)
			}
		}
	}
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(module, root), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			return index.add(module, path)
		})
		if err != nil {
			return nil, err
		}
	}
	return index, nil
}

func (index *traceIndex) pkg(dir string) *tracePackage {
	pkg := index.packages[dir]
	if pkg == nil {
		pkg = &tracePackage{consts: map[string]ast.Expr{}, funcs: map[string]*traceFunc{}, methods: map[string][]*traceFunc{},
			structs: map[string][]string{}, refusals: map[string]bool{}}
		index.packages[dir] = pkg
	}
	return pkg
}

func (index *traceIndex) add(module, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, data, 0)
	if err != nil {
		return err
	}
	rel, _ := filepath.Rel(module, path)
	rel = filepath.ToSlash(rel)
	file := &traceFile{rel: rel, dir: filepath.ToSlash(filepath.Dir(rel)), fset: fset, file: parsed, data: data, imports: map[string]string{}}
	for _, spec := range parsed.Imports {
		importPath, _ := strconv.Unquote(spec.Path.Value)
		dir, ok := strings.CutPrefix(importPath, index.modulePath+"/")
		if !ok || index.modulePath == "" {
			continue
		}
		name := filepath.Base(dir)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		file.imports[name] = dir
	}
	index.files = append(index.files, file)
	pkg := index.pkg(file.dir)
	for _, decl := range parsed.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.ValueSpec:
					if len(s.Names) == len(s.Values) {
						for i, name := range s.Names {
							pkg.consts[name.Name] = s.Values[i]
						}
					}
				case *ast.TypeSpec:
					if st, ok := s.Type.(*ast.StructType); ok {
						var fields []string
						for _, field := range st.Fields.List {
							if len(field.Names) == 0 {
								fields = append(fields, traceTypeName(field.Type))
							}
							for _, name := range field.Names {
								fields = append(fields, name.Name)
							}
						}
						pkg.structs[s.Name.Name] = fields
					}
				}
			}
		case *ast.FuncDecl:
			if d.Body == nil {
				continue
			}
			fn := &traceFunc{file: file, decl: d, name: d.Name.Name, result: -1, messageParams: map[int]traceParam{}}
			if d.Recv != nil && len(d.Recv.List) == 1 {
				receiver := traceTypeName(d.Recv.List[0].Type)
				fn.name = receiver + "." + d.Name.Name
				pkg.methods[d.Name.Name] = append(pkg.methods[d.Name.Name], fn)
				if d.Name.Name == "RefusalCode" {
					pkg.refusals[receiver] = true
				}
			} else {
				pkg.funcs[d.Name.Name] = fn
			}
			for _, field := range d.Type.Params.List {
				isString := traceTypeName(field.Type) == "string"
				for _, name := range field.Names {
					fn.params = append(fn.params, name.Name)
					fn.strings = append(fn.strings, isString)
				}
				if len(field.Names) == 0 {
					fn.params = append(fn.params, "_")
					fn.strings = append(fn.strings, false)
				}
			}
			if d.Type.Results != nil {
				position := 0
				for _, field := range d.Type.Results.List {
					count := max(len(field.Names), 1)
					if traceTypeName(field.Type) == "string" {
						if fn.result >= 0 || count > 1 {
							fn.result = -2
						} else {
							fn.result = position
						}
					}
					position += count
				}
				if fn.result == -2 {
					fn.result = -1
				}
			}
			fn.locals = traceLocals(d.Body)
			index.funcOf[d] = fn
		}
	}
	return nil
}

// traceRefusal says whether a type of the package is a refusal (it has a
// RefusalCode method, or its name says so) with no field for its remedy, so
// its words must carry line 2 themselves.
func (pkg *tracePackage) traceRefusal(name string) bool {
	if !pkg.refusals[name] && !strings.HasSuffix(name, "Refusal") {
		return false
	}
	for _, field := range pkg.structs[name] {
		if messageTraceRemedyField.MatchString(field) {
			return false
		}
	}
	return true
}

var messageTraceRemedyField = regexp.MustCompile(`(?i)remedy|next|decision|command|^run|hint|resolve|second`)

// traceTypeName is a type expression's base name: T for T, *T, pkg.T.
func traceTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return traceTypeName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.IndexExpr:
		return traceTypeName(e.X)
	}
	return ""
}

// traceLocals maps each local name of a body to the values assigned to it:
// :=, =, +=, var, and the elements of x = append(x, ...).
func traceLocals(body *ast.BlockStmt) map[string][]ast.Expr {
	locals := map[string][]ast.Expr{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			return true
		case *ast.AssignStmt:
			if len(n.Lhs) != len(n.Rhs) {
				return true
			}
			for i, lhs := range n.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || ident.Name == "_" {
					continue
				}
				value := n.Rhs[i]
				if call, ok := value.(*ast.CallExpr); ok && messageCallName(call) == "append" && len(call.Args) > 1 {
					if first, ok := call.Args[0].(*ast.Ident); ok && first.Name == ident.Name {
						locals[ident.Name] = append(locals[ident.Name], call.Args[1:]...)
						continue
					}
				}
				locals[ident.Name] = append(locals[ident.Name], value)
			}
		case *ast.ValueSpec:
			if len(n.Names) == len(n.Values) {
				for i, name := range n.Names {
					locals[name.Name] = append(locals[name.Name], n.Values[i])
				}
			}
		case *ast.CallExpr:
			// A builder's writes: b.WriteString(x), fmt.Fprint*(&b, ...).
			if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WriteString" && len(n.Args) == 1 {
				if ident, ok := sel.X.(*ast.Ident); ok {
					locals[ident.Name+".String"] = append(locals[ident.Name+".String"], n.Args[0])
				}
			}
			if name := messageCallName(n); strings.HasPrefix(name, "fmt.Fprint") && len(n.Args) > 1 {
				if unary, ok := n.Args[0].(*ast.UnaryExpr); ok && unary.Op == token.AND {
					if ident, ok := unary.X.(*ast.Ident); ok {
						locals[ident.Name+".String"] = append(locals[ident.Name+".String"], n.Args[1:]...)
					}
				}
			}
		}
		return true
	})
	return locals
}

// directSinks are the message expressions of every function: the direct
// scan's sinks (with every argument of a print or error, not only the
// first) and the sinks it does not read.
func (index *traceIndex) directSinks() []traceSink {
	var sinks []traceSink
	for _, file := range index.files {
		pkg := index.packages[file.dir]
		for _, decl := range file.file.Decls {
			fn := index.funcOf[decl]
			// Package-level declarations: a hook's fixed JSON responses.
			ast.Inspect(decl, func(node ast.Node) bool {
				add := func(expr ast.Expr, kind string, format bool, class string, refusal ...bool) {
					sinks = append(sinks, traceSink{fn: fn, file: file, expr: expr, kind: kind, format: format, class: class, refusal: len(refusal) == 1 && refusal[0]})
				}
				switch n := node.(type) {
				case *ast.CompositeLit:
					messageScanLiteral(n, func(_ ast.Node, kind string, expr ast.Expr, format bool, _ ...string) { add(expr, kind, format, "") })
					index.literalSinks(file, pkg, n, add)
				case *ast.CallExpr:
					messageScanCall(file.fset, file.data, n, func(_ ast.Node, kind string, expr ast.Expr, format bool, _ ...string) { add(expr, kind, format, "") })
					index.callSinks(file, n, add)
				case *ast.BinaryExpr, *ast.BasicLit:
					if text, _ := messageTextOf(n.(ast.Expr), false); strings.Contains(text, `"systemMessage":"`) || strings.HasPrefix(text, "systemMessage=") {
						sinks = append(sinks, traceSink{fn: fn, file: file, expr: n.(ast.Expr), kind: messagePrint, class: messageTraceHook, pick: traceJSONHook})
						return false
					}
				case *ast.FuncDecl:
					// An error type's Error text.
					if f := index.funcOf[n]; f != nil && n.Recv != nil && n.Name.Name == "Error" && f.result == 0 {
						index.returns(f, func(expr ast.Expr) {
							refusal := pkg.traceRefusal(strings.Split(f.name, ".")[0])
							sinks = append(sinks, traceSink{fn: f, file: file, expr: expr, kind: messageError, class: messageTraceErrorMethod, refusal: refusal})
						})
					}
				}
				return true
			})
		}
	}
	return sinks
}

// traceJSONHook takes a hook response's visible words out of its JSON.
func traceJSONHook(text string) (string, bool) {
	match := messageTraceJSONHook.FindStringSubmatch(text)
	if match == nil {
		return "", false
	}
	if match[3] != "" {
		return match[4], true
	}
	value, err := strconv.Unquote(`"` + strings.TrimSuffix(match[2], `\`) + `"`)
	if err != nil {
		value = match[2]
	}
	return value, true
}

// literalSinks reads the words of an error or refusal type's literal, a
// hook's fields on any type, and a hook's map keys.
func (index *traceIndex) literalSinks(file *traceFile, pkg *tracePackage, lit *ast.CompositeLit, add func(ast.Expr, string, bool, string, ...bool)) {
	switch typ := lit.Type.(type) {
	case *ast.MapType:
		for _, element := range lit.Elts {
			kv, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.BasicLit); ok {
				name, _ := strconv.Unquote(key.Value)
				if messageTraceHookKeys[name] || name == "reason" && messageTraceHookDirs.MatchString(file.dir) {
					add(kv.Value, messagePrint, false, messageTraceHook)
				}
			}
		}
		return
	case nil:
		return
	default:
		name := traceTypeName(typ)
		errorType := strings.HasSuffix(name, "Error") || strings.HasSuffix(name, "Refusal") || pkg.refusals[name]
		fields := pkg.structs[name]
		refusal := pkg.traceRefusal(name)
		for position, element := range lit.Elts {
			field := ""
			value := element
			if kv, ok := element.(*ast.KeyValueExpr); ok {
				if key, ok := kv.Key.(*ast.Ident); ok {
					field = key.Name
				}
				value = kv.Value
			} else if position < len(fields) {
				field = fields[position]
			}
			switch {
			case messageTraceHookFields[field]:
				add(value, messagePrint, false, messageTraceHook)
			case errorType && messageTraceFields[field]:
				add(value, messageError, false, "", refusal)
			}
		}
	}
}

// callSinks reads what the direct scan's messageScanCall leaves: every
// argument of a print or error after the first, the streams it does not
// name, and http.Error.
func (index *traceIndex) callSinks(file *traceFile, call *ast.CallExpr, add func(ast.Expr, string, bool, string, ...bool)) {
	name := messageCallName(call)
	switch {
	case (name == "fmt.Fprintf" || name == "fmt.Fprintln" || name == "fmt.Fprint") && len(call.Args) > 1:
		stream := messageExprString(file.fset, file.data, call.Args[0])
		switch {
		case messageWriter.MatchString(stream):
			for _, arg := range call.Args[2:] {
				add(arg, messagePrint, false, "")
			}
		case messageTraceWriter.MatchString(stream):
			for i, arg := range call.Args[1:] {
				add(arg, messagePrint, i == 0 && name == "fmt.Fprintf", messageTraceNotice)
			}
		}
	case (name == "fmt.Printf" || name == "fmt.Println" || name == "fmt.Print") && len(call.Args) > 1:
		for _, arg := range call.Args[1:] {
			add(arg, messagePrint, false, "")
		}
	case name == "fmt.Errorf" && len(call.Args) > 1:
		for _, arg := range call.Args[1:] {
			add(arg, messageError, false, "")
		}
	case name == "Error" && len(call.Args) == 3:
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "http" {
				add(call.Args[1], messageError, false, messageTraceUI)
			}
		}
	}
}

// returns calls visit with the string result of each return statement of
// fn's own body (not of closures inside it).
func (index *traceIndex) returns(fn *traceFunc, visit func(ast.Expr)) {
	ast.Inspect(fn.decl.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if fn.result >= 0 && fn.result < len(n.Results) {
				visit(n.Results[fn.result])
			}
		}
		return true
	})
}

// resolve is the module function a call names, if the traced reading can
// tell: a function of the file's package or of an imported module package,
// or a method whose name is unique in the file's package.
func (index *traceIndex) resolve(file *traceFile, call *ast.CallExpr) *traceFunc {
	pkg := index.packages[file.dir]
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return pkg.funcs[fun.Name]
	case *ast.SelectorExpr:
		if ident, ok := fun.X.(*ast.Ident); ok {
			if dir, ok := file.imports[ident.Name]; ok {
				if other := index.packages[dir]; other != nil {
					return other.funcs[fun.Sel.Name]
				}
				return nil
			}
		}
		if methods := pkg.methods[fun.Sel.Name]; len(methods) == 1 && fun.Sel.Name != "Error" && fun.Sel.Name != "String" {
			return methods[0]
		}
	}
	return nil
}

// values are the expressions a sink's value comes from: the expression
// itself, then, followed, the values of the locals it names (a builder's
// writes, a joined slice's elements).
func (index *traceIndex) values(sink traceSink) []ast.Expr {
	out := []ast.Expr{sink.expr}
	if sink.fn == nil {
		return out
	}
	seen := map[ast.Expr]bool{sink.expr: true}
	var follow func(expr ast.Expr, depth int)
	follow = func(expr ast.Expr, depth int) {
		if depth > 4 {
			return
		}
		ast.Inspect(expr, func(node ast.Node) bool {
			var key string
			switch n := node.(type) {
			case *ast.FuncLit:
				return false
			case *ast.Ident:
				key = n.Name
			case *ast.CallExpr:
				if lit, ok := traceJoined(n); ok {
					for _, value := range lit.Elts {
						if !seen[value] {
							seen[value] = true
							out = append(out, value)
							follow(value, depth+1)
						}
					}
				}
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "String" && len(n.Args) == 0 {
					if ident, ok := sel.X.(*ast.Ident); ok {
						key = ident.Name + ".String"
					}
				}
			}
			for _, value := range sink.fn.locals[key] {
				values := []ast.Expr{value}
				if lit, ok := value.(*ast.CompositeLit); ok {
					values = lit.Elts
				}
				for _, value := range values {
					if !seen[value] {
						seen[value] = true
						out = append(out, value)
						follow(value, depth+1)
					}
				}
			}
			return true
		})
	}
	follow(sink.expr, 0)
	return out
}

// traceJoined is the slice literal a strings.Join call joins, if written
// in place.
func traceJoined(call *ast.CallExpr) (*ast.CompositeLit, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Join" || len(call.Args) != 2 {
		return nil, false
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "strings" {
		return nil, false
	}
	lit, ok := call.Args[0].(*ast.CompositeLit)
	return lit, ok
}

// close adds the sinks the traced reading derives until none is new: the
// string results of the functions a sink's value calls, and the arguments
// a call passes to a parameter that reaches a message.
func (index *traceIndex) close(sinks []traceSink) []traceSink {
	producers := map[*traceFunc]bool{}
	derived := func(sink traceSink) string {
		switch sink.class {
		case messageTraceNotice, messageTraceHook, messageTraceUI, messageTraceErrorMethod:
			return sink.class
		}
		return messageTraceAssembled
	}
	for round := 0; round < 12; round++ {
		changed := false
		// String results of the functions a message's value calls.
		for i := 0; i < len(sinks); i++ {
			sink := sinks[i]
			for _, value := range index.values(sink) {
				ast.Inspect(value, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					f := index.resolve(sink.file, call)
					if f == nil || f.result < 0 || producers[f] {
						return true
					}
					producers[f] = true
					changed = true
					index.returns(f, func(expr ast.Expr) {
						sinks = append(sinks, traceSink{fn: f, file: f.file, expr: expr, kind: sink.kind, class: derived(sink)})
					})
					return true
				})
			}
		}
		// Parameters whose value reaches a message.
		byFunc := map[*traceFunc][]traceSink{}
		for _, sink := range sinks {
			if sink.fn != nil {
				byFunc[sink.fn] = append(byFunc[sink.fn], sink)
			}
		}
		for fn, own := range byFunc {
			for position, name := range fn.params {
				if !fn.strings[position] || name == "_" {
					continue
				}
				if _, known := fn.messageParams[position]; known {
					continue
				}
				tainted := traceTaint(fn, name)
				for _, sink := range own {
					format, reaches := traceReaches(sink.expr, tainted)
					if !reaches {
						continue
					}
					class := sink.class
					if class == "" || class == messageTraceAssembled || class == messageTraceErrorMethod {
						class = messageTraceHelper
					}
					fn.messageParams[position] = traceParam{format: format || sink.format && traceIsIdentOf(sink.expr, tainted), class: class, kind: sink.kind,
						refusal: sink.refusal && traceIsWhole(sink.expr, tainted)}
					changed = true
					break
				}
			}
		}
		// The arguments passed to those parameters, at every call site.
		for _, file := range index.files {
			for _, decl := range file.file.Decls {
				caller := index.funcOf[decl]
				ast.Inspect(decl, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					f := index.resolve(file, call)
					if f == nil {
						return true
					}
					for position, param := range f.messageParams {
						if position >= len(call.Args) {
							continue
						}
						arg := call.Args[position]
						key := fmt.Sprintf("%s:%d", file.rel, arg.Pos())
						if index.sites[key] {
							continue
						}
						index.sites[key] = true
						changed = true
						sinks = append(sinks, traceSink{fn: caller, file: file, expr: arg, kind: param.kind, format: param.format, class: param.class, refusal: param.refusal})
					}
					return true
				})
			}
		}
		if !changed {
			break
		}
	}
	return sinks
}

// traceTaint is a parameter and the locals assigned from it.
func traceTaint(fn *traceFunc, param string) map[string]bool {
	tainted := map[string]bool{param: true}
	for range 3 {
		for name, values := range fn.locals {
			for _, value := range values {
				if _, reaches := traceReaches(value, tainted); reaches {
					tainted[name] = true
				}
			}
		}
	}
	return tainted
}

// traceReaches says whether a message expression carries a tainted name as
// its words: the expression itself, an operand of a concatenation, or the
// format of a Sprintf; an argument filled into a format's hole does not
// count, so an id a message names is not taken for the message.
func traceReaches(expr ast.Expr, tainted map[string]bool) (format, reaches bool) {
	switch e := expr.(type) {
	case *ast.Ident:
		return false, tainted[e.Name]
	case *ast.ParenExpr:
		return traceReaches(e.X, tainted)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return false, false
		}
		// A name written after "metasystem " is a command a person
		// types, as the direct scan's command span holds: not words.
		if prefix, ok := e.X.(*ast.BasicLit); ok && prefix.Kind == token.STRING && traceIsIdentOf(e.Y, tainted) {
			if text, err := strconv.Unquote(prefix.Value); err == nil && (strings.HasSuffix(text, "metasystem ") || strings.HasSuffix(text, "metasystem internal ")) {
				return false, false
			}
		}
		_, left := traceReaches(e.X, tainted)
		_, right := traceReaches(e.Y, tainted)
		return false, left || right
	case *ast.UnaryExpr:
		return traceReaches(e.X, tainted)
	case *ast.CallExpr:
		if name := messageCallName(e); (name == "fmt.Sprintf" || name == "fmt.Errorf") && len(e.Args) > 0 {
			if ident, ok := e.Args[0].(*ast.Ident); ok && tainted[ident.Name] {
				return true, true
			}
		}
	}
	return false, false
}

// traceIsWhole says whether the tainted name is the whole of a message
// (itself, or the format of a Sprintf), not one piece of a concatenation.
func traceIsWhole(expr ast.Expr, tainted map[string]bool) bool {
	if traceIsIdentOf(expr, tainted) {
		return true
	}
	format, _ := traceReaches(expr, tainted)
	return format
}

func traceIsIdentOf(expr ast.Expr, tainted map[string]bool) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && tainted[ident.Name]
}

// traceTextOf is messageTextOf with constants followed: a name of the
// package's or an imported package's string constant is its text.
func (index *traceIndex) traceTextOf(file *traceFile, expr ast.Expr, format bool, depth int) (string, bool) {
	if depth > 6 {
		return messageHole, false
	}
	var value ast.Expr
	var owner *traceFile = file
	switch e := expr.(type) {
	case *ast.Ident:
		value = index.packages[file.dir].consts[e.Name]
	case *ast.SelectorExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			if dir, ok := file.imports[ident.Name]; ok && index.packages[dir] != nil {
				value = index.packages[dir].consts[e.Sel.Name]
				owner = index.fileIn(dir)
			}
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			left, lc := index.traceTextOf(file, e.X, format, depth+1)
			right, rc := index.traceTextOf(file, e.Y, format, depth+1)
			return left + right, lc && rc
		}
	case *ast.ParenExpr:
		return index.traceTextOf(file, e.X, format, depth+1)
	case *ast.CallExpr:
		if name := messageCallName(e); (name == "fmt.Sprintf" || name == "fmt.Errorf") && len(e.Args) > 0 {
			text, _ := index.traceTextOf(file, e.Args[0], true, depth+1)
			return text, false
		}
	}
	if value != nil && owner != nil {
		if _, isLit := value.(*ast.BasicLit); isLit || traceIsConcat(value) {
			return index.traceTextOf(owner, value, format, depth+1)
		}
	}
	return messageTextOf(expr, format)
}

func traceIsConcat(expr ast.Expr) bool {
	b, ok := expr.(*ast.BinaryExpr)
	return ok && b.Op == token.ADD
}

func (index *traceIndex) fileIn(dir string) *traceFile {
	for _, file := range index.files {
		if file.dir == dir {
			return file
		}
	}
	return nil
}

// emit turns one sink into the traced sources it holds.
func (index *traceIndex) emit(sink traceSink) []messageTraced {
	var out []messageTraced
	for i, value := range index.values(sink) {
		class := sink.class
		text, _ := index.traceTextOf(sink.file, value, sink.format && i == 0, 0)
		if i > 0 && class == "" {
			class = messageTraceAssembled
		}
		if class == "" {
			directText, _ := messageTextOf(value, sink.format)
			if directText == text {
				continue
			}
			class = messageTraceConstant
		}
		if sink.pick != nil {
			picked, ok := sink.pick(text)
			if !ok {
				continue
			}
			text = picked
		}
		if !messageTraceWords.MatchString(strings.ReplaceAll(text, messageHole, "x")) || strings.HasPrefix(text, "#!") {
			continue
		}
		if messageTraceFixtureDir.MatchString(sink.file.dir) {
			continue
		}
		if strings.HasPrefix(sink.file.dir, "internal/ui") && class != messageTraceHook {
			class = messageTraceUI
		}
		function := ""
		if sink.fn != nil {
			function = sink.fn.name
		}
		source := messageTraced{Trace: class, messageSource: messageSource{File: sink.file.rel, Line: sink.file.fset.Position(value.Pos()).Line,
			Package: sink.file.dir, Function: function, Kind: sink.kind, Text: text}}
		violations := messageViolations(sink.kind, text)
		// The browser page offers the action beside its refusal, as the
		// direct scan holds for the interface's refuse(kind, code, message).
		if sink.refusal && i == 0 && !messageResolves(text) && class != messageTraceUI {
			violations = append(violations, "no-command")
		}
		if field := messageTraceField.FindString(text); field != "" {
			violations = append(violations, "field:"+strings.TrimSuffix(field, "="+messageHole))
		}
		for _, violation := range violations {
			if code, isCode := strings.CutPrefix(violation, "code:"); isCode && messageTraceKeptCodes[code] == sink.file.rel {
				continue
			}
			source.Violations = append(source.Violations, violation)
		}
		source.Mode = messageTracedModeFor(sink.file.rel, function)
		out = append(out, source)
	}
	return out
}

// messageTracedModeFor is a traced source's mode: kept for machine
// protocol, enforce where a group enforced it, report otherwise.
func messageTracedModeFor(file, function string) string {
	for _, key := range []string{file + "#" + function, file} {
		if _, kept := messageTraceKept[key]; kept {
			return "kept"
		}
	}
	for _, key := range auditKeys(file, function) {
		if mode, ok := messageTracedModes[key]; ok {
			return mode
		}
	}
	return auditReport
}

// messageTraceGroups are the rewrite groups of the traced inventory: the
// layout conversion groups of output-style.md §7, so one builder does the
// words and the shape of a file together. A file takes the group auditModes
// names for it; otherwise the first pattern matching its path decides.
var messageTraceGroups = []struct {
	group    string
	patterns []*regexp.Regexp
}{
	{"G1a", []*regexp.Regexp{regexp.MustCompile(`^internal/(helm|board|stoptransition|stopreport|humanauthority|hooks|report|adapter|session|context|up|seat|stopfence|protocol|refusal|authority|lock)(/|$)`),
		regexp.MustCompile(`^cmd/metasystem/(intent_process|intent_table|intent_helm|intent_status|process_|helm_|admission|person_|hook|session_start|runtime_hook|up\b|up_|intent\.go|helpers|main)`)}},
	{"G1b", []*regexp.Regexp{regexp.MustCompile(`^internal/(landing|ledgerfence|gittree|machine|host|lease|supervise|steward|engine|pins|batch)`),
		regexp.MustCompile(`^cmd/metasystem/(intent_machine|intent_landing|landing|machine|steward|supervise|engine|holder|hold)`)}},
	{"G2", []*regexp.Regexp{regexp.MustCompile(`^internal/(goal|goalsync|grant|attorney|incident|plan|backlog|ledger)`),
		regexp.MustCompile(`^cmd/metasystem/(intent_goal|intent_planning|goal|grant|attorney|incident)`)}},
	{"G3", []*regexp.Regexp{regexp.MustCompile(`^internal/(disk|evidence|retention|artifact|archive|prune|clean|usage|gocache)`),
		regexp.MustCompile(`^cmd/metasystem/(intent_disk|intent_evidence|disk|evidence|clean)`)}},
	{"G4", []*regexp.Regexp{regexp.MustCompile(`^internal/(dispatch|delegation|missionrunner|review|brief|work|design|proofrun|testrun|gaterun|testenv|testpolicy|candidateengine|enginecause|build|worktree|unit)`),
		regexp.MustCompile(`^cmd/metasystem/(intent_work|intent_delivery|intent_design|intent_sent_back|intent_land|intent_manual|intent_unit|intent_review|intent_selection|intent_references|work|delegate|dispatch|mission|proof_run|review|design)`)}},
	{"G5", []*regexp.Regexp{regexp.MustCompile(`^internal/(ui|adopt|app|applaunch|agent|channel|acp|brain|question|launch|exception|decision|run|runtimes|mission|covenant|counselor|capability|contract|registry)(/|$)`),
		regexp.MustCompile(`^cmd/metasystem/(intent_agent|intent_app|intent_adopt|intent_questions|intent_operations|intent_exception|completion|agent|app|adopt|question|launch|brain|channel|ui|seat_launch|adapter_runtime|run\.go)`)}},
	{"G6", []*regexp.Regexp{regexp.MustCompile(`^internal/(receipt|experiment|config|cliflags|validate|audit|testexec|narrator|records|memory|metrics)`),
		regexp.MustCompile(`^cmd/(devgate|metasystem/(test|receipt|report|context|validate|session|runtime_setup|experiment|testing|config_verbs))`)}},
}

func messageTraceGroupOf(file string) string {
	if group := auditGroupFor(file, ""); group != "" {
		return group
	}
	for _, entry := range messageTraceGroups {
		for _, pattern := range entry.patterns {
			if pattern.MatchString(file) {
				return entry.group
			}
		}
	}
	return "unassigned"
}

// TestAuditMessagesTraced is the traced reading of the module: it fails
// only on a violation in a path a group has enforced for it, and with
// METASYSTEM_MESSAGE_INVENTORY_R2=DIR writes inventory-r2.json and
// inventory-r2.md there.
func TestAuditMessagesTraced(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	direct := messageScan(t, module)
	traced := messageTraceScan(t, module, direct)
	if len(traced) < 500 {
		t.Fatalf("the traced scan found %d sources; the module holds thousands of assembled messages, so the scan is broken", len(traced))
	}
	reported := 0
	for _, source := range traced {
		if len(source.Violations) == 0 {
			continue
		}
		if source.Mode == auditEnforce {
			t.Errorf("%s:%d (%s) %s/%s %q: %s", source.File, source.Line, source.Function, source.Trace, source.Kind, source.Text, strings.Join(source.Violations, ", "))
			continue
		}
		reported++
	}
	t.Logf("%d traced message sources, %d with violations in report mode", len(traced), reported)
	if dir := os.Getenv("METASYSTEM_MESSAGE_INVENTORY_R2"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.MarshalIndent(traced, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "inventory-r2.json"), append(encoded, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "inventory-r2.md"), []byte(messageTraceMarkdown(traced)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// messageTraceMarkdown summarizes the traced inventory: counts by trace
// class and violation, per group, per package and per file.
func messageTraceMarkdown(traced []messageTraced) string {
	type tally struct{ sources, violating int }
	byClass := map[string]*tally{}
	byViolation := map[string]int{}
	byGroup := map[string]*tally{}
	byGroupClass := map[string]map[string]int{}
	byPackage := map[string]*tally{}
	byFile := map[string]*tally{}
	fileGroup := map[string]string{}
	count := func(m map[string]*tally, key string, violating bool) {
		if m[key] == nil {
			m[key] = &tally{}
		}
		m[key].sources++
		if violating {
			m[key].violating++
		}
	}
	total, violatingTotal, kept := 0, 0, 0
	for _, source := range traced {
		if source.Mode == "kept" {
			kept++
			continue
		}
		total++
		violating := len(source.Violations) > 0
		if violating {
			violatingTotal++
		}
		group := messageTraceGroupOf(source.File)
		fileGroup[source.File] = group
		count(byClass, source.Trace, violating)
		count(byGroup, group, violating)
		count(byPackage, source.Package, violating)
		count(byFile, source.File, violating)
		if byGroupClass[group] == nil {
			byGroupClass[group] = map[string]int{}
		}
		byGroupClass[group][source.Trace]++
		for _, violation := range source.Violations {
			kind, _, _ := strings.Cut(violation, ":")
			byViolation[kind]++
		}
	}
	var b strings.Builder
	b.WriteString("# Message inventory, round 2: the traced reading\n\n")
	b.WriteString("Generated by `METASYSTEM_MESSAGE_INVENTORY_R2=DIR go test ./cmd/metasystem -run 'TestAuditMessagesTraced$'` " +
		"(cmd/metasystem/message_trace_test.go). Rule: \"Messages a Person Reads\" in docs/design/design-principles.md. " +
		"inventory-r2.json has every source: file:line, function, kind (where it shows), trace (the blind spot it was in), text, violations, mode.\n\n")
	fmt.Fprintf(&b, "%d message sources the direct scan does not hold; %d with at least one violation; %d kept as machine protocol.\n\n", total, violatingTotal, kept)
	b.WriteString("Every source here is in report mode: nothing turns red until a group rewrites its files and enforces them with " +
		"enforceTracedMessages in its message_modes_<group>_test.go.\n\n")
	b.WriteString("What it leaves out, so internal strings are not reported: an argument filled into a format's hole (an id, a path) is not " +
		"taken for the message; a text without two words (a code, a key, an id) is skipped; a Code field, a log line, a map key and " +
		"a JSON field other than a hook's systemMessage, stopReason or block reason are no sinks; fakes, fixtures and the interface " +
		"walkthrough are skipped; the protocol files the direct scan keeps are listed as kept, and CONTEXT_CONFIG_INVALID is not counted " +
		"in internal/config/context.go; a browser refusal is not judged for a command, since the page offers the action; a shell script " +
		"a function assembles is skipped. no-command is judged only for a refusal type (a RefusalCode method, or a name ending in Refusal) " +
		"with no remedy field. field is an internal key=value pair in default text. Limits: a method is followed only when its name is " +
		"unique in its package; a text written to a plain w, out or writer is not read; a producer's piece is judged alone, so a long " +
		"or short verdict on it may change once it is joined.\n\n")
	b.WriteString("## By trace class\n\n| class | what it is | sources | violating |\n|---|---|---|---|\n")
	what := map[string]string{
		messageTraceHelper:      "a literal handed to a helper that builds a refusal or error from it (OpError, RangeError, refused(format, ...))",
		messageTraceAssembled:   "a text a function returns, or a local, slice or builder assembles, and a message then shows",
		messageTraceConstant:    "a string constant written into a message",
		messageTraceNotice:      "a progress or notice line on a stream the direct scan does not name (noteStream, answerErrors, errOut)",
		messageTraceHook:        "a hook's systemMessage, stopReason or block text",
		messageTraceUI:          "a refusal or error the browser interface shows",
		messageTraceErrorMethod: "an error type's Error text",
	}
	for _, class := range messageTraceClasses {
		entry := byClass[class]
		if entry == nil {
			entry = &tally{}
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %d |\n", class, what[class], entry.sources, entry.violating)
	}
	b.WriteString("\n## By violation\n\n| violation | count |\n|---|---|\n")
	for _, kind := range []string{"long", "term", "code", "placeholder", "no-command", "field"} {
		fmt.Fprintf(&b, "| %s | %d |\n", kind, byViolation[kind])
	}
	groups := []string{"G1a", "G1b", "G2", "G3", "G4", "G5", "G6", "unassigned"}
	b.WriteString("\n## Proposed rewrite groups\n\nThe layout conversion groups of output-style.md §7: a cmd/metasystem file takes the group auditModes names for it, " +
		"an internal package the group of the verbs it serves; no file is in two groups, so one builder does the words and the shape of its files together.\n\n")
	b.WriteString("| group | files | sources | violating |")
	for _, class := range messageTraceClasses {
		fmt.Fprintf(&b, " %s |", class)
	}
	b.WriteString("\n|---|---|---|---|" + strings.Repeat("---|", len(messageTraceClasses)) + "\n")
	for _, group := range groups {
		entry := byGroup[group]
		if entry == nil {
			continue
		}
		files := 0
		for _, g := range fileGroup {
			if g == group {
				files++
			}
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d |", group, files, entry.sources, entry.violating)
		for _, class := range messageTraceClasses {
			fmt.Fprintf(&b, " %d |", byGroupClass[group][class])
		}
		b.WriteString("\n")
	}
	sortedKeys := func(m map[string]*tally) []string {
		keys := make([]string, 0, len(m))
		for key := range m {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			if m[keys[i]].violating != m[keys[j]].violating {
				return m[keys[i]].violating > m[keys[j]].violating
			}
			if m[keys[i]].sources != m[keys[j]].sources {
				return m[keys[i]].sources > m[keys[j]].sources
			}
			return keys[i] < keys[j]
		})
		return keys
	}
	b.WriteString("\n## Per package\n\n| package | sources | violating |\n|---|---|---|\n")
	for _, key := range sortedKeys(byPackage) {
		fmt.Fprintf(&b, "| %s | %d | %d |\n", key, byPackage[key].sources, byPackage[key].violating)
	}
	for _, group := range groups {
		if byGroup[group] == nil {
			continue
		}
		fmt.Fprintf(&b, "\n## Files of %s\n\n| file | sources | violating |\n|---|---|---|\n", group)
		for _, key := range sortedKeys(byFile) {
			if fileGroup[key] == group {
				fmt.Fprintf(&b, "| %s | %d | %d |\n", key, byFile[key].sources, byFile[key].violating)
			}
		}
	}
	return b.String()
}
