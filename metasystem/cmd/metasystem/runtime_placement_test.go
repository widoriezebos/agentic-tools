package main

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
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// The placement convention, mechanically checked (backlog item 17,
// D78; rebuilt after the item's own critique showed the first cut
// could not match runtime names INSIDE identifiers): in the seam
// packages and the runtime-named shell files, a file owned by one
// runtime must not reference another runtime in code. Go files are
// parsed — identifiers are split into camelCase/snake_case tokens and
// compared, comments are exempt by construction, and a string literal
// counts only when it IS a bare runtime name (a selector like
// RegisterRecoverer("claude", ...)), never fixture data. Shell files
// use word matching on non-comment lines, which shell's delimiting
// makes sound.
func TestRuntimeFilePlacement(t *testing.T) {
	names := runtimes.Names()
	var violations []string

	goGlobs := []string{
		"../../internal/adapter/*.go",
		"../../internal/host/*.go",
		"../../internal/usage/*.go",
		"*_verbs.go",
	}
	for _, glob := range goGlobs {
		paths, err := filepath.Glob(glob)
		if err != nil || len(paths) == 0 {
			t.Fatalf("go glob %s matched nothing", glob)
		}
		for _, path := range paths {
			owner := fileOwner(path, names)
			if owner == "" {
				continue
			}
			violations = append(violations, goPlacementViolations(t, path, owner, names)...)
		}
	}

	shellGlobs := []string{
		"../../scripts/agents/adapters/*.sh",
		"../../scripts/agents/hosts/*.sh",
	}
	for _, glob := range shellGlobs {
		paths, err := filepath.Glob(glob)
		if err != nil || len(paths) == 0 {
			t.Fatalf("shell glob %s matched nothing", glob)
		}
		for _, path := range paths {
			owner := fileOwner(path, names)
			if owner == "" {
				continue
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			for lineNumber, line := range strings.Split(string(body), "\n") {
				code := strings.SplitN(line, "#", 2)[0]
				for _, other := range names {
					if other == owner {
						continue
					}
					if regexp.MustCompile(`(?i)\b` + other + `\b`).MatchString(code) {
						violations = append(violations,
							filepath.Base(path)+":"+strconv.Itoa(lineNumber+1)+": ["+owner+" file] "+strings.TrimSpace(line))
					}
				}
			}
		}
	}

	if len(violations) > 0 {
		t.Fatalf("cross-runtime code in per-runtime files:\n%s", strings.Join(violations, "\n"))
	}
}

// The tokenizer catches what word boundaries cannot: the check must
// flag DevinPermissionMode-style identifiers in a foreign file — the
// exact shape of the original stray.
func TestPlacementTokenizerCatchesIdentifiers(t *testing.T) {
	for identifier, runtime := range map[string]string{
		"DevinPermissionMode": "devin",
		"TestDevinSettle":     "devin",
		"WriteFakeReturn":     "fake",
		"codexEventField":     "codex",
		"claude_budget":       "claude",
	} {
		if !identifierNamesRuntime(identifier, runtime) {
			t.Fatalf("tokenizer missed %s in %s", runtime, identifier)
		}
	}
	for identifier, runtime := range map[string]string{
		"stubEnv":   "fake",
		"fakerData": "fake",
		"clauses":   "claude",
		"devindex":  "devin",
	} {
		if identifierNamesRuntime(identifier, runtime) {
			t.Fatalf("tokenizer over-fired: %s claimed to name %s", identifier, runtime)
		}
	}
}

func fileOwner(path string, names []string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".go")
	base = strings.TrimSuffix(base, ".sh")
	base = strings.TrimSuffix(base, "_test")
	for _, part := range strings.FieldsFunc(base, func(r rune) bool { return r == '-' || r == '_' || r == '.' }) {
		for _, name := range names {
			// Prefix ownership covers concatenated names like
			// devincollect; over-claiming ownership only tightens.
			if strings.HasPrefix(part, name) {
				return name
			}
		}
	}
	return ""
}

func goPlacementViolations(t *testing.T, path, owner string, names []string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var violations []string
	flag := func(pos token.Pos, what string) {
		position := fset.Position(pos)
		violations = append(violations,
			filepath.Base(path)+":"+strconv.Itoa(position.Line)+": ["+owner+" file] "+what)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.Ident:
			for _, other := range names {
				if other != owner && identifierNamesRuntime(typed.Name, other) {
					flag(typed.Pos(), "identifier "+typed.Name+" names "+other)
				}
			}
		case *ast.BasicLit:
			if typed.Kind == token.STRING {
				value, unquoteErr := strconv.Unquote(typed.Value)
				if unquoteErr != nil {
					return true
				}
				for _, other := range names {
					// Only a BARE runtime name is a selector; fixture
					// blobs never equal one.
					if other != owner && value == other {
						flag(typed.Pos(), "string literal selects "+other)
					}
				}
			}
		}
		return true
	})
	return violations
}

// identifierNamesRuntime splits an identifier into camelCase and
// snake_case tokens and reports whether any token IS the runtime name.
func identifierNamesRuntime(identifier, runtime string) bool {
	var tokens []string
	current := strings.Builder{}
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, strings.ToLower(current.String()))
			current.Reset()
		}
	}
	runes := []rune(identifier)
	for index, r := range runes {
		switch {
		case r == '_' || r == '-':
			flush()
		case unicode.IsUpper(r) && index > 0 && (unicode.IsLower(runes[index-1]) ||
			(index+1 < len(runes) && unicode.IsLower(runes[index+1]))):
			flush()
			current.WriteRune(r)
		default:
			current.WriteRune(r)
		}
	}
	flush()
	for _, part := range tokens {
		if part == runtime {
			return true
		}
	}
	return false
}
