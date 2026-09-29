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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
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

func TestHCL03EveryRowedSiteNamesAnEmission(t *testing.T) {
	root := moduleRoot(t)
	identifiers := refusalCodeIdentifiers(t, root)
	for _, row := range Rows {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Owner), filepath.FromSlash(SiteFile(row.Site))))
		if err != nil {
			t.Errorf("row %s site %q cannot be read from owner %q: %v", row.Code, row.Site, row.Owner, err)
			continue
		}
		if problem := rowSiteProblem(t, row, data, identifiers[row.Code]); problem != "" {
			t.Errorf("row %s site %q %s", row.Code, row.Site, problem)
		}
	}
}

// rowSiteProblem judges one row's site against its file's source: empty when
// the site resolves and an emission of the row's code (its string or a
// constant naming it) lies inside it.
func rowSiteProblem(t *testing.T, row Row, source []byte, identifiers []string) string {
	t.Helper()
	first, last, err := siteSpan(row.Site, source)
	if err != nil {
		return err.Error()
	}
	emissionLines := refusalEmissionLines(t, SiteFile(row.Site), source, row.Code, identifiers)
	if !windowContainsEmission(emissionLines, first, last) {
		return "does not name an emitted string or identifier inside its anchor"
	}
	return ""
}

func refusalCodeIdentifiers(t *testing.T, root string) map[string][]string {
	t.Helper()
	identifiers := map[string][]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, specification := range general.Specs {
				values, ok := specification.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, expression := range values.Values {
					if index >= len(values.Names) {
						break
					}
					literal, ok := expression.(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						continue
					}
					value, err := strconv.Unquote(literal.Value)
					if err == nil {
						identifiers[value] = append(identifiers[value], values.Names[index].Name)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("collect refusal identifiers: %v", err)
	}
	return identifiers
}

func refusalEmissionLines(t *testing.T, path string, source []byte, code string, identifiers []string) map[int]bool {
	t.Helper()
	if filepath.Ext(path) != ".go" {
		emissions := map[int]bool{}
		for index, line := range strings.Split(string(source), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "#") && strings.Contains(line, code) {
				emissions[index+1] = true
			}
		}
		return emissions
	}
	identifierSet := map[string]bool{}
	for _, identifier := range identifiers {
		identifierSet[identifier] = true
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "site.go", source, 0)
	if err != nil {
		t.Fatalf("parse refusal owner: %v", err)
	}
	emissions := map[int]bool{}
	var stack []ast.Node
	var walk func(ast.Node)
	walk = func(node ast.Node) {
		if node == nil {
			return
		}
		candidate := false
		switch value := node.(type) {
		case *ast.BasicLit:
			text, unquoteErr := strconv.Unquote(value.Value)
			candidate = value.Kind == token.STRING && unquoteErr == nil && strings.Contains(text, code)
		case *ast.Ident:
			candidate = identifierSet[value.Name]
		}
		if candidate && refusalOccurrenceIsEmitted(node, stack) {
			emissions[fset.Position(node.Pos()).Line] = true
		}
		stack = append(stack, node)
		ast.Inspect(node, func(child ast.Node) bool {
			if child == node {
				return true
			}
			if child != nil {
				walk(child)
			}
			return false
		})
		stack = stack[:len(stack)-1]
	}
	walk(file)
	return emissions
}

func refusalOccurrenceIsEmitted(node ast.Node, stack []ast.Node) bool {
	position := node.Pos()
	for index := len(stack) - 1; index >= 0; index-- {
		switch parent := stack[index].(type) {
		case *ast.CallExpr:
			return positionInExpressions(position, parent.Args)
		case *ast.ReturnStmt:
			return positionInExpressions(position, parent.Results)
		case *ast.AssignStmt:
			return positionInExpressions(position, parent.Rhs)
		case *ast.CompositeLit:
			return true
		case *ast.BinaryExpr:
			if parent.Op == token.EQL || parent.Op == token.NEQ || parent.Op == token.LSS || parent.Op == token.LEQ ||
				parent.Op == token.GTR || parent.Op == token.GEQ {
				return false
			}
		case *ast.ValueSpec:
			return false
		}
	}
	return false
}

func positionInExpressions(position token.Pos, expressions []ast.Expr) bool {
	for _, expression := range expressions {
		if expression.Pos() <= position && position <= expression.End() {
			return true
		}
	}
	return false
}

func windowContainsEmission(lines map[int]bool, first, last int) bool {
	for line := first; line <= last; line++ {
		if lines[line] {
			return true
		}
	}
	return false
}

func TestHCL03SiteValidatorRejectsNonEmissionReference(t *testing.T) {
	source := []byte("package fixture\nconst refusalCode = \"SITE_DRIFT\"\nfunc emit(observed string) error {\n\tif observed == refusalCode { return nil }\n\tassigned := observed == refusalCode\n\tconsume(observed == refusalCode)\n\tvalues := []bool{observed == refusalCode}\n\tif assigned || values[0] { return observed == refusalCode }\n\treturn fmt.Errorf(\"SITE_DRIFT: emitted\")\n}\n")
	lines := refusalEmissionLines(t, "fixture.go", source, "SITE_DRIFT", []string{"refusalCode"})
	for _, rejected := range []int{2, 4, 5, 6, 7, 8} {
		if lines[rejected] {
			t.Fatalf("emission lines = %v, comparison-only line %d was accepted", lines, rejected)
		}
	}
	if !lines[9] {
		t.Fatalf("emission lines = %v, want the returned error on line 9", lines)
	}
}

func TestHCL03EngineCausesEachCarryARemedy(t *testing.T) {
	maximum := 0
	for _, cause := range enginecause.Table {
		if cause.Commands > maximum {
			maximum = cause.Commands
		}
	}
	for _, row := range Rows {
		if row.Code == "TEST_POLICY_ENGINE_REQUIRED" {
			if row.Override != "the run: command of the line's cause (internal/enginecause.Table)" || row.Commands != maximum {
				t.Fatalf("engine refusal register row does not name the cause table and its maximum command count: %+v maximum=%d", row, maximum)
			}
			return
		}
	}
	t.Fatal("TEST_POLICY_ENGINE_REQUIRED has no refusal register row")
}

func TestHCL03EveryRowReal(t *testing.T) {
	root := moduleRoot(t)
	goalVerbs := collectGoalVerbs(t, filepath.Join(root, "cmd", "metasystem", "main.go"))
	for action := range collectPublicGoalActions(t, filepath.Join(root, "cmd", "metasystem")) {
		goalVerbs[action] = struct{}{}
	}
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
			if _, ok := goalVerbs[strings.TrimRight(words[1], ",")]; !ok {
				t.Errorf("row %q names unknown goal verb %q in override %q", row.Code, words[1], row.Override)
			}
		}
	}
}

func TestHCL03GoalDoneReadItemsOpenRow(t *testing.T) {
	for _, row := range Rows {
		if row.Code == "GOAL_DONE_READ_ITEMS_OPEN" {
			if row.Owner != "internal/goal" || row.Shape != Question || row.Override != "" {
				t.Fatalf("GOAL_DONE_READ_ITEMS_OPEN row = %+v", row)
			}
			return
		}
	}
	t.Fatal("GOAL_DONE_READ_ITEMS_OPEN has no refusal-register row")
}

func TestHCL03ProofAdmissionRowsNameEmissions(t *testing.T) {
	wants := map[string]Row{
		"CANDIDATE_GOAL_REFUSED":           {Owner: "cmd/metasystem", Site: "proof_run.go#candidateGoalRefusal", Shape: Question},
		"CANDIDATE_GOAL_MOVED":             {Owner: "cmd/metasystem", Site: "proof_run.go#proofAdmissionMoved", Shape: Question},
		"PROOF_AUTHORITY_REQUIRED":         {Owner: "cmd/metasystem", Site: "proof_run.go#proofAuthorityRefusal", Shape: Question},
		"PROOF_AUTHORITY_ARC_MATE_REFUSED": {Owner: "cmd/metasystem", Site: "proof_run.go#resolveProofGoalRolesWithReads", Shape: Question},
		"CANDIDATE_EXTENSION_REFUSED":      {Owner: "internal/dispatch", Site: "admission.go#evaluateCandidateConsumptionAdmissionWithReads", Shape: Question},
		"RETRY_PRIOR_OUTSIDE_TREE":         {Owner: "internal/proofrun", Site: "attempt.go#readRetryDecision", Shape: Question},
		"SET_BUDGET_FENCED_SAME_TUPLE":     {Owner: "internal/goal", Site: "verbs.go#setBudgetRequest", Shape: Question},
		"REBIND_EPOCH_UNAUTHENTICATED":     {Owner: "internal/goal", Site: "verbs.go#ClaimEpochForRebind", Shape: Identity},
	}
	root := moduleRoot(t)
	for code, want := range wants {
		var got Row
		for _, row := range Rows {
			if row.Code == code {
				got = row
				break
			}
		}
		if got.Code != code || got.Owner != want.Owner || got.Site != want.Site || got.Shape != want.Shape {
			t.Errorf("row %s = %+v, want %+v", code, got, want)
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, want.Owner, SiteFile(want.Site)))
		if err != nil {
			t.Errorf("read row %s site: %v", code, err)
			continue
		}
		first, last, err := siteSpan(want.Site, data)
		if err != nil {
			t.Errorf("row %s site %s %v", code, want.Site, err)
			continue
		}
		lines := strings.Split(string(data), "\n")
		found := false
		for index := first - 1; index < min(len(lines), last); index++ {
			found = found || strings.Contains(lines[index], code)
		}
		if !found {
			t.Errorf("row %s site %s has no emission inside its anchor", code, want.Site)
		}
	}
}

func TestHCL03HandoffCancelRows(t *testing.T) {
	wants := map[string]Row{
		"HANDOFF_HUMAN_UNPROVEN": {Owner: "internal/steward", Shape: Identity},
		"HANDOFF_OTHER_SESSION":  {Owner: "internal/steward", Shape: Agent, Override: "metasystem session handoff --root <installation> --cancel <nonce> --by <human>, typed at an enrolled or agent-free terminal", Commands: 1},
	}
	for code, want := range wants {
		var got Row
		for _, row := range Rows {
			if row.Code == code {
				got = row
				break
			}
		}
		if got.Code != code || got.Owner != want.Owner || got.Shape != want.Shape || got.Override != want.Override || got.Commands != want.Commands {
			t.Errorf("row %s = %+v, want owner=%q shape=%q override=%q commands=%d", code, got, want.Owner, want.Shape, want.Override, want.Commands)
		}
	}
}

func TestHCL03HandoffCaptureSitesAreEmissionLines(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(moduleRoot(t), "internal", "steward", "handoff_capture.go"))
	if err != nil {
		t.Fatal(err)
	}
	const file = "handoff_capture.go"
	for _, row := range Rows {
		if row.Owner != "internal/steward" || SiteFile(row.Site) != file {
			continue
		}
		first, last, err := siteSpan(row.Site, data)
		if err != nil || !strings.Contains(strings.Join(strings.Split(string(data), "\n")[first-1:last], "\n"), `"`+row.Code+`"`) {
			t.Errorf("row %s site %q does not anchor an emission: %v", row.Code, row.Site, err)
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
		if row.Pending != "" && (row.Pending != "human-carried-landing" || row.Override != CarriedLanding) {
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
		"internal/testpolicy",
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

// publicGoalActionRE reads a public goal action's registration from the
// command table's source.
var publicGoalActionRE = regexp.MustCompile(`object: "goal", action: "([a-z][a-z-]*)"`)

// collectPublicGoalActions returns the public goal actions the command table
// registers, so an override may name the public form a person runs.
func collectPublicGoalActions(t *testing.T, dir string) map[string]struct{} {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	actions := make(map[string]struct{})
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "intent") || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range publicGoalActionRE.FindAllStringSubmatch(string(data), -1) {
			actions[match[1]] = struct{}{}
		}
	}
	if len(actions) == 0 {
		t.Fatal("found no public goal action in the command table's source")
	}
	return actions
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

func TestHCL03NoPendingAfterSlice2(t *testing.T) {
	root := moduleRoot(t)
	carriedRows := 0
	for _, row := range Rows {
		if row.Pending != "" {
			t.Errorf("refusal %s still has pending marker %q", row.Code, row.Pending)
		}
		if row.Override == CarriedLanding {
			carriedRows++
		}
		if strings.HasPrefix(row.Code, "carry-") && (row.Shape != Question || row.Override != "") {
			t.Errorf("carried ask %s is not a Question row with no override", row.Code)
		}
	}
	if carriedRows == 0 {
		t.Fatal("the register has no carried refusal rows")
	}
	for _, row := range ProseRows {
		if row.Override == "" && !row.Record {
			t.Errorf("prose row %s/%s has neither override nor record-failure marker", row.Owner, row.Site)
		}
		if row.Record && row.Override != "" {
			t.Errorf("prose row %s/%s is both overridable and a record failure", row.Owner, row.Site)
		}
		if !proseRowSiteEmits(t, root, row) {
			t.Errorf("prose row %s/%s does not name the line that emits %q", row.Owner, row.Site, row.Prose)
		}
	}
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	checks := []struct{ name, path, needle string }{
		{"carry question", "cmd/metasystem/intent_process.go", `if kind == "carry" && !goal.ValidCarryToken(in.Wants)`},
		{"carry reservation owner", "cmd/metasystem/landing_path.go", `GoalCarrying:    landingPathCarrying,`},
		{"carried landing record owner", "cmd/metasystem/landing_path.go", `GoalCarried:     landingPathCarried,`},
		{"carry status owner", "cmd/metasystem/landing_path.go", `CarryStatus:     landingPathCarryStatus,`},
		{"work land exception option", "cmd/metasystem/intent_delivery.go", `{name: "exception", value: "CODE"`},
		{"work land using-exception option", "cmd/metasystem/intent_delivery.go", `{name: "using-exception", value: "ID"`},
		{"commit boundary carried declaration", "internal/landing/landpath/commit.go", `if request.Carried != "" && (!request.GoalSet`},
	}
	sources := map[string]string{}
	for _, check := range checks {
		if _, ok := sources[check.path]; !ok {
			sources[check.path] = read(check.path)
		}
		if !strings.Contains(sources[check.path], check.needle) {
			t.Errorf("entry point %s is absent from %s", check.name, check.path)
		}
	}
	for _, removed := range checks {
		mutated := make(map[string]string, len(sources))
		for path, source := range sources {
			mutated[path] = source
		}
		mutated[removed.path] = strings.Replace(mutated[removed.path], removed.needle, "", 1)
		missing := false
		for _, check := range checks {
			if !strings.Contains(mutated[check.path], check.needle) {
				missing = true
				break
			}
		}
		if !missing {
			t.Errorf("removing %s did not fail the entry-point check", removed.name)
		}
	}
}

// proseRowSiteEmits reports whether the declaration the row's site anchors
// carries the prose's fixed opening (the text before its first placeholder).
func proseRowSiteEmits(t *testing.T, root string, row Prose) bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Owner), filepath.FromSlash(SiteFile(row.Site))))
	if err != nil {
		return false
	}
	first, last, err := siteSpan(row.Site, data)
	if err != nil {
		return false
	}
	opening := row.Prose
	if cut := strings.IndexAny(opening, "<"); cut >= 0 {
		opening = opening[:cut]
	}
	lines := strings.Split(string(data), "\n")
	for index := first; index <= min(len(lines), last); index++ {
		if strings.Contains(lines[index-1], opening) {
			return true
		}
	}
	return false
}

func TestHCL11EntryPointsPresent(t *testing.T) {
	carriedOverrides := 0
	carryQuestions := map[string]bool{}
	for _, row := range Rows {
		if row.Override == CarriedLanding {
			carriedOverrides++
			if row.Pending != "" {
				t.Errorf("carried override %s is still pending: %s", row.Code, row.Pending)
			}
		}
		if strings.HasPrefix(row.Code, "carry-") {
			carryQuestions[row.Code] = true
			if row.Shape != Question || row.Override != "" {
				t.Errorf("carry ask %s must be a Question with no override", row.Code)
			}
		}
	}
	// The original 48 pending Rows lose three non-carry ledger-meaning
	// overrides and the two deleted promotion-reader failures, leaving 43
	// carryable refusal codes.
	if carriedOverrides != 43 {
		t.Fatalf("carried refusal override count = %d, want 43 carryable refusal rows", carriedOverrides)
	}
	want := []string{"carry-ledger-moved", "carry-goal-not-live", "carry-word-missing", "carry-word-unproven", "carry-seat-mismatch", "carry-tree-mismatch", "carry-not-carryable", "carry-word-expired", "carry-word-consumed", "carry-debt-unpaid", "carry-cap-reached", "carry-base-judge-blind", "carry-battery-unverified", "carry-unneeded", "carry-refusal-mismatch"}
	for _, code := range want {
		if !carryQuestions[code] {
			t.Errorf("carry ask row %s is absent", code)
		}
	}
}
