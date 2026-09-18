package testutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

type busyLoopFinding struct {
	path string
	line int
	kind string
}

func (finding busyLoopFinding) site() string {
	return finding.path + ":" + strconv.Itoa(finding.line)
}

func TestFixtureSourcesHaveNoBareBusyLoop(t *testing.T) {
	positive := "while :; do " + ":" + "; done"
	if findings := shellBusyLoops(positive); len(findings) != 1 {
		t.Fatalf("positive parser specimen findings = %#v, want one", findings)
	}
	if findings := shellBusyLoops("while true; do sleep 1; done"); len(findings) != 0 {
		t.Fatalf("negative parser specimen findings = %#v, want none", findings)
	}
	for _, positive := range []string{
		"while [[ 1 ]]; do " + ":" + "; done",
		"until ready; do " + ":" + "; done",
		"for ((;;)); do " + ":" + "; done",
	} {
		if findings := shellBusyLoops(positive); len(findings) != 1 {
			t.Fatalf("shell parser findings for %q = %#v, want one", positive, findings)
		}
	}
	if findings := shellBusyLoops("while (($#)); do shift; done"); len(findings) != 0 {
		t.Fatalf("finite shell parser specimen findings = %#v, want none", findings)
	}
	goPositive := "package fixture\nfunc spin() { for {} }\n"
	if findings, err := goBusyLoops("positive_test.go", "positive_test.go", goPositive); err != nil || len(findings) != 1 {
		t.Fatalf("positive Go parser specimen findings = %#v, %v; want one", findings, err)
	}
	goNegative := "package fixture\nfunc wait() { for { select {} } }\n"
	if findings, err := goBusyLoops("negative_test.go", "negative_test.go", goNegative); err != nil || len(findings) != 0 {
		t.Fatalf("negative Go parser specimen findings = %#v, %v; want none", findings, err)
	}
	goFinite := "package fixture\nfunc finite() { n := 1; for { n--; if n == 0 { break } } }\n"
	if findings, err := goBusyLoops("finite_test.go", "finite_test.go", goFinite); err != nil || len(findings) != 0 {
		t.Fatalf("finite Go parser specimen findings = %#v, %v; want none", findings, err)
	}

	root := fixtureSourceRoot(t)
	var findings []busyLoopFinding
	walk := func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if strings.HasPrefix(relative, "scripts/agents/") && strings.HasSuffix(relative, ".sh") {
			contents, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, finding := range shellBusyLoops(string(contents)) {
				finding.path = relative
				findings = append(findings, finding)
			}
			return nil
		}
		if !strings.HasSuffix(relative, "_test.go") {
			return nil
		}
		goFindings, parseErr := goTestBusyLoops(relative, path)
		if parseErr != nil {
			return parseErr
		}
		findings = append(findings, goFindings...)
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatal(err)
	}

	allowed := map[string]string{
		"scripts/agents/supervision-fixtures.sh:3664": "the loop advances a completed-invocation counter, exits when output appears, and refuses at its fixed budget",
	}
	used := make(map[string]bool, len(allowed))
	var unexpected []string
	for _, finding := range findings {
		site := finding.site()
		if _, ok := allowed[site]; ok {
			used[site] = true
			continue
		}
		unexpected = append(unexpected, site+" "+finding.kind)
	}
	for site, reason := range allowed {
		if reason == "" {
			t.Errorf("bare-loop allow-list entry %s needs a reason", site)
		}
		if !used[site] {
			t.Errorf("stale bare-loop allow-list entry %s: %s", site, reason)
		}
	}
	if len(unexpected) != 0 {
		sort.Strings(unexpected)
		t.Fatalf("fixture sources contain bare busy loops:\n%s", strings.Join(unexpected, "\n"))
	}
}

func fixtureSourceRoot(t *testing.T) string {
	t.Helper()
	if override := os.Getenv("FIXTURE_SOURCE_ROOT"); override != "" {
		return override
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate fixture source test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
}

func goTestBusyLoops(relative, path string) ([]busyLoopFinding, error) {
	return goBusyLoops(relative, path, nil)
}

func goBusyLoops(relative, filename string, source any) ([]busyLoopFinding, error) {
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, filename, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var findings []busyLoopFinding
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.ForStmt:
			if typed.Init == nil && typed.Cond == nil && typed.Post == nil && !goLoopBlocks(typed.Body) && !goLoopHasFiniteProgress(typed.Body) {
				findings = append(findings, busyLoopFinding{
					path: relative, line: files.Position(typed.For).Line, kind: "Go for {} loop has no blocking operation",
				})
			}
		case *ast.BasicLit:
			if typed.Kind != token.STRING {
				break
			}
			value, unquoteErr := strconv.Unquote(typed.Value)
			if unquoteErr != nil {
				break
			}
			for _, finding := range shellBusyLoops(value) {
				findings = append(findings, busyLoopFinding{
					path: relative, line: files.Position(typed.Pos()).Line + finding.line - 1,
					kind: "shell literal " + finding.kind,
				})
			}
		}
		return true
	})
	return findings, nil
}

func goLoopBlocks(body *ast.BlockStmt) bool {
	blocks := false
	ast.Inspect(body, func(node ast.Node) bool {
		if blocks {
			return false
		}
		if literal, ok := node.(*ast.FuncLit); ok && literal.Body != body {
			return false
		}
		switch typed := node.(type) {
		case *ast.SelectStmt:
			blocks = selectBlocks(typed)
		case *ast.UnaryExpr:
			blocks = typed.Op == token.ARROW
		case *ast.CallExpr:
			blocks = blockingCallName(callName(typed.Fun))
		}
		return !blocks
	})
	return blocks
}

func goLoopHasFiniteProgress(body *ast.BlockStmt) bool {
	progress, exits := false, false
	ast.Inspect(body, func(node ast.Node) bool {
		if literal, ok := node.(*ast.FuncLit); ok && literal.Body != body {
			return false
		}
		switch typed := node.(type) {
		case *ast.AssignStmt, *ast.IncDecStmt:
			progress = true
		case *ast.BranchStmt:
			exits = exits || typed.Tok == token.BREAK
		case *ast.ReturnStmt:
			exits = true
		}
		return !(progress && exits)
	})
	return progress && exits
}

func selectBlocks(statement *ast.SelectStmt) bool {
	for _, item := range statement.Body.List {
		clause, ok := item.(*ast.CommClause)
		if ok && clause.Comm == nil {
			return false
		}
	}
	return true
}

func callName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	default:
		return ""
	}
}

func blockingCallName(name string) bool {
	lower := strings.ToLower(name)
	return lower == "sleep" || lower == "wait" || strings.HasPrefix(lower, "wait") ||
		lower == "read" || strings.HasPrefix(lower, "read") || lower == "recv" || lower == "receive" ||
		lower == "next" || lower == "after" || lower == "newtimer" || lower == "newticker"
}

type shellToken struct {
	text string
	line int
}

func shellBusyLoops(source string) []busyLoopFinding {
	tokens, fragments := shellTokens(source)
	var findings []busyLoopFinding
	for index := 0; index < len(tokens); index++ {
		candidate, unconditional := shellLoopStart(tokens, index)
		if !candidate {
			continue
		}
		do := nextShellToken(tokens, index+1, "do")
		if do < 0 {
			continue
		}
		depth, done := 1, -1
		for cursor := do + 1; cursor < len(tokens); cursor++ {
			switch tokens[cursor].text {
			case "do":
				depth++
			case "done":
				depth--
				if depth == 0 {
					done = cursor
				}
			}
			if done >= 0 {
				break
			}
		}
		if done < 0 || shellRangeBlocks(tokens[index:done+1]) || !unconditional && shellRangeProgresses(tokens[do+1:done]) {
			continue
		}
		findings = append(findings, busyLoopFinding{line: tokens[index].line, kind: tokens[index].text + " loop has no blocking operation"})
	}
	for _, fragment := range fragments {
		for _, finding := range shellBusyLoops(fragment.text) {
			finding.line += fragment.line - 1
			findings = append(findings, finding)
		}
	}
	return findings
}

func shellLoopStart(tokens []shellToken, index int) (candidate, unconditional bool) {
	switch tokens[index].text {
	case "while":
		if shellConditionIs(tokens, index+1, "false") {
			return false, false
		}
		return true, shellConditionIs(tokens, index+1, "true") || shellConditionIs(tokens, index+1, ":")
	case "until":
		if shellConditionIs(tokens, index+1, "true") || shellConditionIs(tokens, index+1, ":") {
			return false, false
		}
		return true, shellConditionIs(tokens, index+1, "false")
	case "for":
		do := nextShellToken(tokens, index+1, "do")
		if do < 0 {
			return false, false
		}
		header := strings.ReplaceAll(joinShellTokens(tokens[index+1:do]), " ", "")
		start, end := strings.Index(header, "(("), strings.LastIndex(header, "))")
		if start < 0 || end <= start+2 {
			return false, false
		}
		parts := strings.Split(header[start+2:end], ";")
		unbounded := len(parts) == 3 && parts[1] == ""
		return unbounded, unbounded
	default:
		return false, false
	}
}

func shellConditionIs(tokens []shellToken, index int, condition string) bool {
	return index < len(tokens) && tokens[index].text == condition
}

func nextShellToken(tokens []shellToken, start int, wanted string) int {
	for index := start; index < len(tokens); index++ {
		if tokens[index].text == wanted {
			return index
		}
		if strings.HasSuffix(tokens[index].text, ":") && tokens[index].text != ":" {
			return -1
		}
		if tokens[index].text == "done" {
			return -1
		}
	}
	return -1
}

func joinShellTokens(tokens []shellToken) string {
	words := make([]string, len(tokens))
	for index, token := range tokens {
		words[index] = token.text
	}
	return strings.Join(words, " ")
}

func shellRangeBlocks(tokens []shellToken) bool {
	for _, token := range tokens {
		switch token.text {
		case "sleep", "wait", "read", "select":
			return true
		}
	}
	return false
}

func shellRangeProgresses(tokens []shellToken) bool {
	for _, token := range tokens {
		switch token.text {
		case "shift", "break", "return", "exit":
			return true
		}
		if strings.Contains(token.text, "++") || strings.Contains(token.text, "--") ||
			strings.Contains(token.text, "=") && !strings.Contains(token.text, "==") && !strings.Contains(token.text, "=~") {
			return true
		}
	}
	return false
}

type shellFragment struct {
	text string
	line int
}

func shellTokens(source string) ([]shellToken, []shellFragment) {
	var tokens []shellToken
	var fragments []shellFragment
	line := 1
	for index := 0; index < len(source); {
		character := source[index]
		if character == '\n' {
			line++
			index++
			continue
		}
		if unicode.IsSpace(rune(character)) {
			index++
			continue
		}
		if character == '#' {
			for index < len(source) && source[index] != '\n' {
				index++
			}
			continue
		}
		if character == '\'' || character == '"' || character == '`' {
			quote, quoteLine := character, line
			index++
			start := index
			for index < len(source) && source[index] != quote {
				if source[index] == '\\' && quote != '\'' && index+1 < len(source) {
					if source[index+1] == '\n' {
						line++
					}
					index += 2
					continue
				}
				if source[index] == '\n' {
					line++
				}
				index++
			}
			fragments = append(fragments, shellFragment{text: source[start:index], line: quoteLine})
			if index < len(source) {
				index++
			}
			continue
		}
		if strings.ContainsRune(";(){}&|", rune(character)) {
			text := string(character)
			if index+1 < len(source) && source[index+1] == character && strings.ContainsRune("()&|", rune(character)) {
				text += string(character)
				index++
			}
			tokens = append(tokens, shellToken{text: text, line: line})
			index++
			continue
		}
		start, wordLine := index, line
		for index < len(source) && !unicode.IsSpace(rune(source[index])) && !strings.ContainsRune(";'\"`(){}&|#", rune(source[index])) {
			index++
		}
		if start != index {
			tokens = append(tokens, shellToken{text: source[start:index], line: wordLine})
			continue
		}
		index++
	}
	return tokens, fragments
}
