package refusal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	upperSnakeToken = regexp.MustCompile(`[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+`)
	hyphenCode      = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*-(?:refused|unreadable|malformed|unavailable)$`)
)

func TestHCL03EveryCodeRowed(t *testing.T) {
	collected := collectRefusalTokens(t, moduleRoot(t))
	rowed := make(map[string]struct{}, len(Rows))
	for _, row := range Rows {
		rowed[row.Code] = struct{}{}
	}

	exclusionMatches := make([]int, len(Exclusions))
	for code := range collected {
		if _, ok := rowed[code]; ok {
			continue
		}
		matched := false
		for i, exclusion := range Exclusions {
			if exclusionMatchesCode(exclusion.Pattern, code) {
				exclusionMatches[i]++
				matched = true
			}
		}
		if !matched {
			t.Errorf("collected refusal token %q has no row or exclusion", code)
		}
	}
	for i, matches := range exclusionMatches {
		if matches == 0 {
			t.Errorf("exclusion %q is dead: it matches no collected token", Exclusions[i].Pattern)
		}
	}
	t.Logf("collected %d refusal-shaped tokens", len(collected))
}

func TestHCL03EveryRowReal(t *testing.T) {
	root := moduleRoot(t)
	goalVerbs := collectGoalVerbs(t, filepath.Join(root, "cmd", "metasystem", "main.go"))
	for _, row := range Rows {
		if row.Pending == "human-carried-landing" || row.Override == "" {
			continue
		}
		words := strings.Fields(row.Override)
		if len(words) > 0 && words[0] == "goal" {
			if len(words) < 2 {
				t.Errorf("row %q has incomplete goal override %q", row.Code, row.Override)
				continue
			}
			if _, ok := goalVerbs[words[1]]; !ok {
				t.Errorf("row %q names unknown goal verb %q in override %q", row.Code, words[1], row.Override)
			}
		}
	}
}

func TestHCL03PendingRowsNamed(t *testing.T) {
	rowCodes := make(map[string]struct{}, len(Rows))
	defectCodes := make(map[string]struct{}, len(Defects))
	for _, defect := range Defects {
		defectCodes[defect.Code] = struct{}{}
	}
	for _, row := range Rows {
		rowCodes[row.Code] = struct{}{}
		if row.Pending != "" && (row.Pending != "human-carried-landing" || !strings.HasPrefix(row.Override, "land.sh --carried")) {
			t.Errorf("row %q has invalid pending marker %q or override %q", row.Code, row.Pending, row.Override)
		}
		if row.Shape == Agent && row.Override == "" {
			if _, ok := defectCodes[row.Code]; !ok {
				t.Errorf("agent refusal row %q has no override and no defect entry", row.Code)
			}
		}
	}
	for _, defect := range Defects {
		if _, ok := rowCodes[defect.Code]; !ok {
			t.Errorf("defect %q has no refusal row", defect.Code)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find module root containing go.mod")
		}
		dir = parent
	}
}

func collectRefusalTokens(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	directories := []string{
		"internal/dispatch",
		"internal/goal",
		"internal/goalbudget",
		"internal/landing",
		"internal/steward",
		"internal/channel",
		"internal/humanauthority",
		"cmd/metasystem",
	}
	collected := make(map[string]struct{})
	for _, directory := range directories {
		directory := directory
		err := filepath.Walk(filepath.Join(root, filepath.FromSlash(directory)), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			collectLiteralTokens(t, file, collected)
			if directory == "internal/landing" {
				collectLandingTokens(t, file, collected)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", directory, err)
		}
	}
	return collected
}

func collectLiteralTokens(t *testing.T, file *ast.File, collected map[string]struct{}) {
	t.Helper()
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value := unquote(t, literal)
		for _, match := range upperSnakeToken.FindAllStringIndex(value, -1) {
			if (match[0] == 0 || !identifierByte(value[match[0]-1])) &&
				(match[1] == len(value) || !identifierByte(value[match[1]])) {
				collected[value[match[0]:match[1]]] = struct{}{}
			}
		}
		if hyphenCode.MatchString(value) {
			collected[value] = struct{}{}
		}
		return true
	})
}

func collectLandingTokens(t *testing.T, file *ast.File, collected map[string]struct{}) {
	t.Helper()
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.CallExpr:
			name, ok := value.Fun.(*ast.Ident)
			if ok && name.Name == "wouldRefuse" && len(value.Args) > 0 {
				addStringLiteral(t, value.Args[0], collected)
			}
		case *ast.CompositeLit:
			name, ok := value.Type.(*ast.Ident)
			if !ok || name.Name != "carriageError" {
				break
			}
			for _, element := range value.Elts {
				pair, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := pair.Key.(*ast.Ident)
				if ok && key.Name == "code" {
					addStringLiteral(t, pair.Value, collected)
				}
			}
		}
		return true
	})
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "knownRefusalCode" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			clause, ok := node.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expression := range clause.List {
				addStringLiteral(t, expression, collected)
			}
			return true
		})
	}
}

func addStringLiteral(t *testing.T, expression ast.Expr, collected map[string]struct{}) {
	t.Helper()
	literal, ok := expression.(*ast.BasicLit)
	if ok && literal.Kind == token.STRING {
		collected[unquote(t, literal)] = struct{}{}
	}
}

func unquote(t *testing.T, literal *ast.BasicLit) string {
	t.Helper()
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		t.Fatalf("unquote string literal %s: %v", literal.Value, err)
	}
	return value
}

func identifierByte(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || value == '_'
}

func exclusionMatchesCode(pattern, code string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(code, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == code
}

func collectGoalVerbs(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	verbs := make(map[string]struct{})
	ast.Inspect(file, func(node ast.Node) bool {
		family, ok := node.(*ast.CompositeLit)
		if !ok || !compositeHasStringField(t, family, "name", "goal") {
			return true
		}
		for _, element := range family.Elts {
			pair, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := pair.Key.(*ast.Ident)
			list, listOK := pair.Value.(*ast.CompositeLit)
			if !ok || key.Name != "verbs" || !listOK {
				continue
			}
			for _, item := range list.Elts {
				verb, ok := item.(*ast.CompositeLit)
				if !ok || len(verb.Elts) == 0 {
					continue
				}
				literal, ok := verb.Elts[0].(*ast.BasicLit)
				if ok && literal.Kind == token.STRING {
					verbs[unquote(t, literal)] = struct{}{}
				}
			}
		}
		return false
	})
	return verbs
}

func compositeHasStringField(t *testing.T, literal *ast.CompositeLit, field, want string) bool {
	t.Helper()
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := pair.Key.(*ast.Ident)
		value, valueOK := pair.Value.(*ast.BasicLit)
		if ok && key.Name == field && valueOK && value.Kind == token.STRING {
			return unquote(t, value) == want
		}
	}
	return false
}
