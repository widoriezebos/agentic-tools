package lifecycle

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// listenNetAllowed is everything ValidateListen may use from net. All three
// decide an address from its own bytes; none of them consults a resolver.
var listenNetAllowed = map[string]bool{"SplitHostPort": true, "ParseIP": true, "JoinHostPort": true}

// A refusal that first resolved the name would satisfy every assertion about
// the refusals themselves, so the no-lookup property is proven from the code
// rather than from behaviour: a name must be refused from its own bytes, with
// no resolver reached, no network touched, and nothing to wait for. The check
// reads the function's own body instead of a seam, because a seam that exists
// only to be asserted against has no other consumer.
func TestValidateListenReachesNoResolver(t *testing.T) {
	t.Parallel()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "serve.go", nil, 0)
	testutil.Require(t, "parse serve.go", err, nil)

	var body *ast.BlockStmt
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Name.Name == "ValidateListen" {
			body = function.Body
		}
	}
	testutil.Require(t, "ValidateListen found in serve.go", body != nil, true)

	var used []string
	ast.Inspect(body, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "net" {
			used = append(used, selector.Sel.Name)
		}
		if strings.Contains(selector.Sel.Name, "Lookup") || strings.Contains(selector.Sel.Name, "Resolver") {
			used = append(used, selector.Sel.Name)
		}
		return true
	})

	var forbidden []string
	for _, name := range used {
		if !listenNetAllowed[name] {
			forbidden = append(forbidden, name)
		}
	}
	sort.Strings(forbidden)
	testutil.Expect(t, "resolver or other net call in ValidateListen", forbidden, []string(nil))
}
