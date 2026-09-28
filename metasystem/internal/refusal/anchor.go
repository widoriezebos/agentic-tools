package refusal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// A register site is an anchor, not a line: "file.go#Symbol" names the
// top-level declaration that emits the row's refusal, so an edit above the
// refusal never breaks the register (it broke a dozen times on 2026-09-28
// when sites were file:line). Symbol is a function name, a method as
// Receiver.Method (a pointer receiver drops its star, a generic one its type
// parameters), or a package-level var, const or type name. The row's code
// (or, for a prose row, its prose) finishes the anchor: the static audit
// requires the emission inside the declaration.

// SiteFile is the owner-relative file a site anchor names.
func SiteFile(site string) string {
	file, _, _ := strings.Cut(site, "#")
	return file
}

// SiteSymbol is the declaration a site anchor names.
func SiteSymbol(site string) string {
	_, symbol, _ := strings.Cut(site, "#")
	return symbol
}

// siteSpan resolves a site anchor against its file's source to the first and
// last line of the declaration it names.
func siteSpan(site string, source []byte) (first, last int, err error) {
	file, symbol, ok := strings.Cut(site, "#")
	if !ok || file == "" || symbol == "" {
		return 0, 0, fmt.Errorf("is not a file#Symbol anchor")
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, source, parser.SkipObjectResolution)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot be parsed: %v", err)
	}
	span := func(node ast.Node) (int, int, error) {
		return fset.Position(node.Pos()).Line, fset.Position(node.End()).Line, nil
	}
	for _, declaration := range parsed.Decls {
		switch declared := declaration.(type) {
		case *ast.FuncDecl:
			if declarationName(declared) == symbol {
				return span(declared)
			}
		case *ast.GenDecl:
			for _, specification := range declared.Specs {
				switch spec := specification.(type) {
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if name.Name == symbol {
							if len(declared.Specs) == 1 {
								return span(declared)
							}
							return span(spec)
						}
					}
				case *ast.TypeSpec:
					if spec.Name.Name == symbol {
						return span(spec)
					}
				}
			}
		}
	}
	return 0, 0, fmt.Errorf("names no top-level declaration %s in %s", symbol, file)
}

// declarationName is a function's anchor name: Name, or Receiver.Name for a
// method.
func declarationName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return function.Name.Name
	}
	receiver := function.Recv.List[0].Type
	for {
		switch typed := receiver.(type) {
		case *ast.StarExpr:
			receiver = typed.X
			continue
		case *ast.IndexExpr:
			receiver = typed.X
			continue
		case *ast.IndexListExpr:
			receiver = typed.X
			continue
		case *ast.Ident:
			return typed.Name + "." + function.Name.Name
		}
		return function.Name.Name
	}
}
