// Package rootaudit finds every place where a state-root value reaches a
// run-state path. Run state (artifacts/, adapters/ and metasystem.conf)
// lives under the installation, so a path to it built from the state root is
// a crossing. The ratchet compares the crossings with run-state-audit.json at
// the module root, which only shrinks: each work deletes the entries it owns,
// and a new crossing, or a fixed one still listed, refuses the static gate.
//
// A source is a value that carries the state root: X.Path() with X typed
// roots.State; a string field named stateRoot or StateRoot; or a string
// variable or parameter of that name declared in a production file.
//
// A direct site is State.Path("artifacts"|"adapters"|"metasystem.conf", ...),
// a filepath.Join or path.Join of a source whose next segment is, or begins
// with, one of those names, or a source concatenated with such a segment. A
// flow site hands a source to a parameter or a string struct field that
// reaches such a site, found by a fixpoint over static calls, function
// literals, and func-typed struct fields and package variables.
// filepath.Clean, Abs, EvalSymlinks, ToSlash and FromSlash and
// strings.TrimSpace and TrimSuffix pass a value through, and a local
// variable assigned from a source or a parameter carries it.
//
// Known limits: calls through interface methods are not followed; paths
// built through stateroot.Steward are not seen; a func-typed parameter is not
// followed; and a value renamed into a variable not named stateRoot is
// followed only within its function.
package rootaudit

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
)

// runStateLiteral reports whether a path segment joined right after a root
// names run state.
func runStateLiteral(s string) bool {
	s = strings.TrimPrefix(s, "/")
	for _, p := range []string{"artifacts", "adapters", "metasystem.conf"} {
		if s == p || strings.HasPrefix(s, p+"/") {
			return true
		}
	}
	return false
}

// checkedPackage is one package variant type-checked from source.
type checkedPackage struct {
	path  string
	fset  *token.FileSet
	files []*ast.File
	info  *types.Info
}

type analyzer struct {
	dir   string // module directory
	trim  string // module path and slash, trimmed from function names
	roots string // import path of the roots package
	pkgs  []*checkedPackage
	// runParams maps "FullName#index" to a witness.
	runParams map[string]string
	// runFields maps "Type.Field" to a witness.
	runFields map[string]string
	// funcTargets maps a func-typed field or package variable to the
	// functions assigned to it ("FullName" or "lit@file:line:column").
	funcTargets map[string]map[string]bool
	changed     bool
	sites       []Site
	seen        map[string]bool
}

func newAnalyzer(dir, module string, pkgs []*checkedPackage) *analyzer {
	return &analyzer{dir: dir, trim: module + "/", roots: module + "/internal/roots", pkgs: pkgs,
		runParams: map[string]string{}, runFields: map[string]string{}, funcTargets: map[string]map[string]bool{}, seen: map[string]bool{}}
}

// run repeats the passes until one adds no run-state parameter, field or
// function target, so the last pass sees every flow.
func (a *analyzer) run() []Site {
	for round := 0; round < 50; round++ {
		a.changed = false
		a.pass()
		if !a.changed {
			break
		}
	}
	sort.SliceStable(a.sites, func(i, j int) bool {
		x, y := a.sites[i], a.sites[j]
		if x.File != y.File {
			return x.File < y.File
		}
		return x.Line < y.Line
	})
	return a.sites
}

func funcKey(f *types.Func, i int) string { return fmt.Sprintf("%s#%d", f.FullName(), i) }

func deref(t types.Type) types.Type {
	if p, ok := t.(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

func typeKey(t types.Type) string {
	t = deref(t)
	if n, ok := t.(*types.Named); ok {
		obj := n.Obj()
		if obj.Pkg() != nil {
			return obj.Pkg().Path() + "." + obj.Name()
		}
		return obj.Name()
	}
	return types.TypeString(t, nil)
}

func isString(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

func (a *analyzer) isStateType(t types.Type) bool {
	n, ok := deref(t).(*types.Named)
	if !ok {
		return false
	}
	o := n.Obj()
	return o.Pkg() != nil && o.Pkg().Path() == a.roots && o.Name() == "State"
}

// statePath reports whether f is the method roots.State.Path.
func (a *analyzer) statePath(f *types.Func) bool {
	if f == nil || f.Name() != "Path" {
		return false
	}
	sig, ok := f.Type().(*types.Signature)
	return ok && sig.Recv() != nil && a.isStateType(sig.Recv().Type())
}

func stringConst(info *types.Info, e ast.Expr) (string, bool) {
	tv, ok := info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

// calleeFunc returns the static callee of a call, if any.
func calleeFunc(info *types.Info, call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	case *ast.IndexExpr:
		if sel, ok := fun.X.(*ast.SelectorExpr); ok {
			id = sel.Sel
		} else if i, ok := fun.X.(*ast.Ident); ok {
			id = i
		}
	}
	if id == nil {
		return nil
	}
	if f, ok := info.Uses[id].(*types.Func); ok {
		return f.Origin()
	}
	return nil
}

func isJoin(f *types.Func) bool {
	if f == nil || f.Pkg() == nil {
		return false
	}
	p := f.Pkg().Path()
	return (p == "path/filepath" || p == "path") && f.Name() == "Join"
}

func isPassThrough(f *types.Func) bool {
	if f == nil || f.Pkg() == nil {
		return false
	}
	p, n := f.Pkg().Path(), f.Name()
	if p == "path/filepath" && (n == "Clean" || n == "Abs" || n == "EvalSymlinks" || n == "ToSlash" || n == "FromSlash") {
		return true
	}
	return p == "strings" && (n == "TrimSpace" || n == "TrimSuffix")
}

func stateRootName(name string) bool { return name == "stateRoot" || name == "StateRoot" }

// packageVar reports a package-level variable.
func packageVar(obj types.Object) (*types.Var, bool) {
	v, ok := obj.(*types.Var)
	return v, ok && !v.IsField() && v.Pkg() != nil && v.Parent() == v.Pkg().Scope()
}

// origin describes what a string expression carries inside one function:
// a parameter of that function, a struct field, or a state-root source.
type origin struct {
	param  int    // index of the enclosing function's parameter, or -1
	field  string // field key when the expression reads a string field
	source string // non-empty when the expression carries a state root
}

type funcCtx struct {
	a      *analyzer
	pkg    *checkedPackage
	fn     *types.Func // nil for function literals
	litKey string      // set for function literals
	name   string
	params map[types.Object]int
	alias  map[types.Object]origin
}

func (a *analyzer) newCtx(pkg *checkedPackage, name string) *funcCtx {
	return &funcCtx{a: a, pkg: pkg, name: name, params: map[types.Object]int{}, alias: map[types.Object]origin{}}
}

func (c *funcCtx) info() *types.Info { return c.pkg.info }

func (c *funcCtx) paramKey(i int) string {
	if c.fn != nil {
		return funcKey(c.fn, i)
	}
	return fmt.Sprintf("%s#%d", c.litKey, i)
}

func (c *funcCtx) bindParams(list *ast.FieldList) {
	i := 0
	for _, field := range list.List {
		if len(field.Names) == 0 {
			i++
			continue
		}
		for _, name := range field.Names {
			if o := c.info().Defs[name]; o != nil {
				c.params[o] = i
			}
			i++
		}
	}
}

// production reports an object declared outside a test file: a test-local
// variable named stateRoot carries a fixture directory, not the state root.
func (c *funcCtx) production(obj types.Object) bool {
	return !strings.HasSuffix(c.pkg.fset.Position(obj.Pos()).Filename, "_test.go")
}

func litKey(fset *token.FileSet, lit *ast.FuncLit) string {
	p := fset.Position(lit.Pos())
	return fmt.Sprintf("lit@%s:%d:%d", p.Filename, p.Line, p.Column)
}

// funcValueKey names the function a func-valued expression denotes.
func (c *funcCtx) funcValueKey(e ast.Expr) string {
	var id *ast.Ident
	switch x := ast.Unparen(e).(type) {
	case *ast.FuncLit:
		return litKey(c.pkg.fset, x)
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	default:
		return ""
	}
	if f, ok := c.info().Uses[id].(*types.Func); ok {
		return f.Origin().FullName()
	}
	if v, ok := packageVar(c.info().Uses[id]); ok {
		return "var:" + v.Pkg().Path() + "." + v.Name()
	}
	return ""
}

func (c *funcCtx) addTarget(slot, target string) {
	if slot == "" || target == "" {
		return
	}
	m := c.a.funcTargets[slot]
	if m == nil {
		m = map[string]bool{}
		c.a.funcTargets[slot] = m
	}
	if !m[target] {
		m[target] = true
		c.a.changed = true
	}
}

// resolveTargets expands a slot to the concrete function keys it can call.
func (a *analyzer) resolveTargets(slot string, depth int) []string {
	if depth > 6 {
		return nil
	}
	var out []string
	for t := range a.funcTargets[slot] {
		if strings.HasPrefix(t, "var:") || strings.HasPrefix(t, "field:") {
			out = append(out, a.resolveTargets(t, depth+1)...)
			continue
		}
		out = append(out, t)
	}
	return out
}

// dynamicSlot names the func-typed field or package variable a call goes
// through, when its callee is not static.
func (c *funcCtx) dynamicSlot(call *ast.CallExpr) string {
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		if sel, ok := c.info().Selections[fun]; ok && sel.Kind() == types.FieldVal {
			return "field:" + typeKey(sel.Recv()) + "." + fun.Sel.Name
		}
		id = fun.Sel
	case *ast.Ident:
		id = fun
	default:
		return ""
	}
	if v, ok := packageVar(c.info().Uses[id]); ok {
		return "var:" + v.Pkg().Path() + "." + v.Name()
	}
	return ""
}

// originOf classifies a string-valued expression.
func (c *funcCtx) originOf(e ast.Expr) origin {
	none := origin{param: -1}
	info := c.info()
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		obj := info.Uses[x]
		if obj == nil {
			obj = info.Defs[x]
		}
		if obj == nil {
			return none
		}
		named := isString(obj.Type()) && stateRootName(x.Name) && c.production(obj)
		if i, ok := c.params[obj]; ok {
			o := origin{param: i}
			if named {
				o.source = "variable " + x.Name
			}
			return o
		}
		if o, ok := c.alias[obj]; ok {
			return o
		}
		if named {
			return origin{param: -1, source: "variable " + x.Name}
		}
	case *ast.SelectorExpr:
		if sel, ok := info.Selections[x]; ok && sel.Kind() == types.FieldVal && isString(sel.Obj().Type()) {
			key := typeKey(sel.Recv()) + "." + x.Sel.Name
			o := origin{param: -1, field: key}
			if stateRootName(x.Sel.Name) {
				o.source = "field " + key
			}
			return o
		}
	case *ast.CallExpr:
		f := calleeFunc(info, x)
		if c.a.statePath(f) && len(x.Args) == 0 {
			return origin{param: -1, source: "State.Path() of " + compact(x.Fun)}
		}
		if isPassThrough(f) && len(x.Args) > 0 {
			return c.originOf(x.Args[0])
		}
	}
	return none
}

func compact(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return compact(x.X) + "." + x.Sel.Name
	case *ast.CallExpr:
		return compact(x.Fun) + "(...)"
	case *ast.IndexExpr:
		return compact(x.X) + "[...]"
	case *ast.StarExpr:
		return "*" + compact(x.X)
	case *ast.ParenExpr:
		return compact(x.X)
	}
	return fmt.Sprintf("%T", e)
}

// sink records that origin o reaches a run-state use described by why.
func (c *funcCtx) sink(o origin, at token.Pos, sinkDesc, why, kind string) {
	c.sinkAs(o, at, sinkDesc, sinkDesc, why, kind)
}

// sinkAs is sink for a use that distinct tells apart from others on its line
// that read the same, such as two function literals in one file.
func (c *funcCtx) sinkAs(o origin, at token.Pos, distinct, sinkDesc, why, kind string) {
	a := c.a
	if o.param >= 0 && (c.fn != nil || c.litKey != "") {
		k := c.paramKey(o.param)
		if _, ok := a.runParams[k]; !ok {
			a.runParams[k] = why
			a.changed = true
		}
	}
	if o.field != "" {
		if _, ok := a.runFields[o.field]; !ok {
			a.runFields[o.field] = why
			a.changed = true
		}
	}
	if o.source != "" {
		pos := c.pkg.fset.Position(at)
		key := fmt.Sprintf("%s:%d:%s:%s", pos.Filename, pos.Line, o.source, distinct)
		if !a.seen[key] {
			a.seen[key] = true
			a.sites = append(a.sites, Site{File: a.rel(pos.Filename), Line: pos.Line, Test: strings.HasSuffix(pos.Filename, "_test.go"),
				Kind: kind, Function: c.name, Source: o.source, Sink: sinkDesc, Witness: why})
		}
	}
}

func (c *funcCtx) witness(at token.Pos, joined string) string {
	pos := c.pkg.fset.Position(at)
	return fmt.Sprintf("%s:%d joins %q", c.a.rel(pos.Filename), pos.Line, joined)
}

func (c *funcCtx) walk(body ast.Node) {
	info := c.info()
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i, lhs := range x.Lhs {
					c.assign(lhs, x.Rhs[i], x.Pos())
				}
			}
		case *ast.ValueSpec:
			if len(x.Names) == len(x.Values) {
				for i, id := range x.Names {
					o := c.originOf(x.Values[i])
					if obj := info.Defs[id]; obj != nil && (o.param >= 0 || o.source != "" || o.field != "") {
						c.alias[obj] = o
					}
				}
			}
		case *ast.CompositeLit:
			c.composite(x)
		case *ast.BinaryExpr:
			if x.Op == token.ADD {
				if s, ok := stringConst(info, x.Y); ok && runStateLiteral(s) && (strings.HasPrefix(s, "/") || strings.HasPrefix(s, string(filepath.Separator))) {
					c.sink(c.originOf(x.X), x.Pos(), "concatenation", c.witness(x.Pos(), s), "concat")
				}
			}
		case *ast.CallExpr:
			c.call(x)
		}
		// Closures see the enclosing aliases, so they are walked in place
		// as well as on their own.
		return true
	})
}

func (c *funcCtx) assign(lhs, rhs ast.Expr, at token.Pos) {
	info := c.info()
	o := c.originOf(rhs)
	if id, ok := lhs.(*ast.Ident); ok {
		obj := info.Defs[id]
		if obj == nil {
			obj = info.Uses[id]
		}
		if v, ok := packageVar(obj); ok {
			c.addTarget("var:"+v.Pkg().Path()+"."+v.Name(), c.funcValueKey(rhs))
		}
		if obj != nil && (o.param >= 0 || o.source != "" || o.field != "") {
			if _, isParam := c.params[obj]; !isParam {
				c.alias[obj] = o
			}
		}
	}
	sel, ok := lhs.(*ast.SelectorExpr)
	if !ok {
		return
	}
	if s, ok := info.Selections[sel]; ok && s.Kind() == types.FieldVal {
		if _, isFunc := s.Obj().Type().Underlying().(*types.Signature); isFunc {
			c.addTarget("field:"+typeKey(s.Recv())+"."+sel.Sel.Name, c.funcValueKey(rhs))
		}
	}
	if v, ok := packageVar(info.Uses[sel.Sel]); ok {
		c.addTarget("var:"+v.Pkg().Path()+"."+v.Name(), c.funcValueKey(rhs))
	}
	if s, ok := info.Selections[sel]; ok && s.Kind() == types.FieldVal && isString(s.Obj().Type()) {
		key := typeKey(s.Recv()) + "." + sel.Sel.Name
		if why, ok := c.a.runFields[key]; ok {
			c.sink(o, at, "field "+key, why, "flow")
		}
	}
}

func (c *funcCtx) composite(x *ast.CompositeLit) {
	tv, ok := c.info().Types[x]
	if !ok {
		return
	}
	st, ok := deref(tv.Type).Underlying().(*types.Struct)
	if !ok {
		return
	}
	for _, elt := range x.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		kid, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			field := st.Field(i)
			if field.Name() != kid.Name {
				continue
			}
			if _, isFunc := field.Type().Underlying().(*types.Signature); isFunc {
				c.addTarget("field:"+typeKey(tv.Type)+"."+kid.Name, c.funcValueKey(kv.Value))
			}
			if isString(field.Type()) {
				key := typeKey(tv.Type) + "." + kid.Name
				if why, ok := c.a.runFields[key]; ok {
					c.sink(c.originOf(kv.Value), kv.Pos(), "field "+key, why, "flow")
				}
			}
		}
	}
}

func (c *funcCtx) call(x *ast.CallExpr) {
	info := c.info()
	f := calleeFunc(info, x)
	if f == nil {
		slot := c.dynamicSlot(x)
		if slot == "" {
			return
		}
		for _, target := range c.a.resolveTargets(slot, 0) {
			for i, arg := range x.Args {
				if why, ok := c.a.runParams[fmt.Sprintf("%s#%d", target, i)]; ok {
					c.sinkAs(c.originOf(arg), arg.Pos(), fmt.Sprintf("%s arg %d via %s", target, i, slot),
						fmt.Sprintf("%s arg %d via %s", c.a.display(target), i, c.a.display(slot)), why, "flow")
				}
			}
		}
		return
	}
	// A direct join onto a typed State: State.Path("artifacts", ...).
	if c.a.statePath(f) && len(x.Args) > 0 {
		if s, ok := stringConst(info, x.Args[0]); ok && runStateLiteral(s) {
			pos := c.pkg.fset.Position(x.Pos())
			c.sink(origin{param: -1, source: "State " + compact(x.Fun)}, x.Pos(), fmt.Sprintf("Path(%q)", s), fmt.Sprintf("%s:%d", c.a.rel(pos.Filename), pos.Line), "direct")
		}
	}
	if isJoin(f) && len(x.Args) >= 2 {
		if s, ok := stringConst(info, x.Args[1]); ok && runStateLiteral(s) {
			c.sink(c.originOf(x.Args[0]), x.Pos(), fmt.Sprintf("filepath.Join(_, %q)", s), c.witness(x.Pos(), s), "direct")
		}
		return
	}
	sig, ok := f.Type().(*types.Signature)
	if !ok {
		return
	}
	for i, arg := range x.Args {
		pi := min(i, sig.Params().Len()-1)
		if pi < 0 {
			continue
		}
		if why, ok := c.a.runParams[funcKey(f, pi)]; ok {
			c.sink(c.originOf(arg), arg.Pos(), fmt.Sprintf("%s arg %d (%s)", c.a.display(f.FullName()), pi, sig.Params().At(pi).Name()), why, "flow")
		}
	}
}

// display names a function or slot as a key shows it: without the module
// path, and a function literal by its file alone, so that a key survives
// edits above the literal and reads the same in every checkout.
func (a *analyzer) display(name string) string {
	if at, ok := strings.CutPrefix(name, "lit@"); ok {
		for range 2 {
			if i := strings.LastIndex(at, ":"); i >= 0 {
				at = at[:i]
			}
		}
		return "lit@" + a.rel(at)
	}
	return strings.ReplaceAll(name, a.trim, "")
}

func (a *analyzer) rel(p string) string {
	if r, err := filepath.Rel(a.dir, p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}

func (a *analyzer) pass() {
	for _, pkg := range a.pkgs {
		for _, file := range pkg.files {
			for _, decl := range file.Decls {
				if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
					c := a.newCtx(pkg, "package var")
					for _, spec := range gd.Specs {
						vs := spec.(*ast.ValueSpec)
						for i, id := range vs.Names {
							if i < len(vs.Values) {
								if v, ok := pkg.info.Defs[id].(*types.Var); ok {
									c.addTarget("var:"+pkg.path+"."+v.Name(), c.funcValueKey(vs.Values[i]))
								}
								c.walk(vs.Values[i])
							}
						}
					}
				}
				ast.Inspect(decl, func(n ast.Node) bool {
					if lit, ok := n.(*ast.FuncLit); ok {
						lc := a.newCtx(pkg, "func literal")
						lc.litKey = litKey(pkg.fset, lit)
						lc.bindParams(lit.Type.Params)
						lc.walk(lit.Body)
					}
					return true
				})
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				c := a.newCtx(pkg, fd.Name.Name)
				if obj, ok := pkg.info.Defs[fd.Name].(*types.Func); ok {
					c.fn = obj
					c.name = a.display(obj.FullName())
				}
				c.bindParams(fd.Type.Params)
				c.walk(fd.Body)
			}
		}
	}
}
