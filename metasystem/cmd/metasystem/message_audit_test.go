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
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// "Messages a Person Reads" (docs/design/design-principles.md): the default
// text of every message a person reads is line 1, what happened and why in
// plain words, and line 2, the one command that resolves it. This audit reads
// every production message source in the module and checks what can be
// checked from the source: the first line's length, a resolving command on a
// refusal, internal terms and refusal codes, and placeholders. The mode table
// decides, per package, file or function, whether a violation is reported
// (the test passes and the inventory lists it) or enforced (the test fails).

// messageModeEnforce fails the audit on any violation of the path; every
// other path is reported only. The per-path modes are the messages column
// of auditModes, the table this audit shares with the layout audit.
const messageModeEnforce = auditEnforce

// enforceMessages moves paths to enforce mode in the messages column of
// auditModes; a package-level "var _ = enforceMessages(...)" in each rewrite
// group's file (message_modes_<group>_test.go) calls it, so parallel
// builders never edit one table.
func enforceMessages(paths ...string) bool {
	for _, path := range paths {
		mode := auditModes[path]
		mode.messages = messageModeEnforce
		auditModes[path] = mode
	}
	return true
}

// The person, helm and grant family: the reference rewrite.
var _ = enforceMessages(
	"cmd/metasystem/admission_notice.go",
	"cmd/metasystem/attorney_admits.go",
	"cmd/metasystem/grant_everything.go",
	"cmd/metasystem/helm_admits.go",
	"cmd/metasystem/helm_force.go",
	"cmd/metasystem/helm_return.go",
	"cmd/metasystem/intent_grant_everything.go",
	"cmd/metasystem/intent_helm.go",
	"cmd/metasystem/person_refusal.go",
	"cmd/metasystem/intent_goals.go#intentInvocation.actorArgs",
	"cmd/metasystem/intent_planning.go#intentInvocation.actingAs",
	"cmd/metasystem/intent_planning.go#runIntentGrant",
	"cmd/metasystem/intent_planning.go#runIntentRevoke",
	"cmd/metasystem/intent_planning.go#runIntentGrantList",
	"cmd/metasystem/intent_process.go#intentInvocation.startRefusal",
	"cmd/metasystem/intent_process.go#processRefusalResult",
	"cmd/metasystem/intent_process.go#runIntentEnroll",
	"cmd/metasystem/intent_process.go#runIntentSystemRestart",
	"cmd/metasystem/intent_process.go#runIntentSystemStart",
	"cmd/metasystem/intent_process.go#systemStopResult",
	"cmd/metasystem/process_verbs.go#classificationDataRefusal",
	"cmd/metasystem/process_verbs.go#humanTerminalCheck",
	"cmd/metasystem/process_verbs.go#processOwners.arm",
	"cmd/metasystem/process_verbs.go#processOwners.stop",
	"cmd/metasystem/goalsync_mutations.go#runGoalEnrollTerminalWithDependencies",
	"internal/helm",
	"internal/humanauthority/remedy.go",
	"internal/humanauthority/authority.go#Enroll",
	"internal/humanauthority/authority.go#Prove",
)

// messageLineBudget is the first line's length the rule aims at.
const messageLineBudget = 100

// messageKind names where a text is shown.
const (
	messageSummary  = "summary"     // intentResult.Summary, a refusal's sentence: line 1
	messageDecision = "decision"    // intentResult.Decision, a refusal's remedy words: line 2
	messageReason   = "next-reason" // intentResult.nextReason, shown beside the run line
	messageText     = "text"        // intentResult.text, printed by default after line 1
	messageError    = "error"       // fmt.Errorf, errors.New, complain: often a refusal's line 1
	messagePrint    = "print"       // a line written straight to standard output or error
	messageProse    = "register"    // a refusal register prose row
)

// messageSource is one message a person may read by default.
type messageSource struct {
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Package    string   `json:"package"`
	Function   string   `json:"function"`
	Kind       string   `json:"kind"`
	Text       string   `json:"text"`
	Violations []string `json:"violations,omitempty"`
	Mode       string   `json:"mode"`
}

// A hole is a value only known at run time; it counts as one character.
const messageHole = "…"

var (
	messageFormatVerb  = regexp.MustCompile(`%[-+# 0-9.*\[\]]*[a-zA-Z%]`)
	messageCommandSpan = regexp.MustCompile("metasystem [^;:,()\"`\n]*")
	messageFlagSpan    = regexp.MustCompile(`--[a-z][a-z0-9-]*`)
	messagePublicTerms = regexp.MustCompile(`HUMAN AT THE HELM|POWER OF ATTORNEY`)
	messagePlaceholder = regexp.MustCompile(`\b(NAME|PATH|ID|G|J)\b`)
	messageCodeToken   = regexp.MustCompile(`\b[A-Z][A-Z0-9]+(_[A-Z0-9]+)+\b`)
	// messageBanned are internal terms: each is the word a person never
	// reads by default, and why.
	messageBanned = []struct {
		pattern *regexp.Regexp
		term    string
	}{
		{regexp.MustCompile(`(?i)\bproofs?\b`), "proof"},
		{regexp.MustCompile(`(?i)\byield(s|ed|ing)?\b`), "yield"},
		{regexp.MustCompile(`(?i)\bcallers?\b`), "caller"},
		{regexp.MustCompile(`\b(HUMAN|MAIN|DELEGATE|UNTRUSTED|SUPERVISION|STEWARD)\b`), "caller class"},
		{regexp.MustCompile(`(?i)\blineages?\b`), "lineage"},
		{regexp.MustCompile(`(?i)\bepochs?\b`), "epoch"},
		{regexp.MustCompile(`(?i)\bgeneration\b`), "generation"},
		{regexp.MustCompile(`(?i)\bopids?\b`), "opid"},
		{regexp.MustCompile(`(?i)\bdigests?\b`), "digest"},
		{regexp.MustCompile(`(?i)ledger tip|accepted tree|\bjournal(ed)?\b`), "ledger internals"},
		{regexp.MustCompile(`would-refuse|\bcode=`), "refusal code"},
		{regexp.MustCompile(`== STEP`), "step banner"},
	}
)

// messageModeFor is the mode of one source: the longest key naming its
// function, file or a directory above it.
func messageModeFor(file, function string) string {
	candidates := []string{file + "#" + function, file}
	for dir := filepath.ToSlash(filepath.Dir(file)); dir != "." && dir != "/"; dir = filepath.ToSlash(filepath.Dir(dir)) {
		candidates = append(candidates, dir)
	}
	for _, key := range candidates {
		if mode, ok := auditModes[key]; ok && mode.messages != "" {
			return mode.messages
		}
	}
	return "report"
}

// messageTextOf is an expression's text as far as the source says it:
// literals and their concatenations, Sprintf formats with their verbs as
// holes, and a hole for anything else. complete is false when any part was a
// hole.
func messageTextOf(expr ast.Expr, format bool) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return messageHole, false
		}
		text, err := strconv.Unquote(e.Value)
		if err != nil {
			return messageHole, false
		}
		if format {
			holes := messageFormatVerb.ReplaceAllStringFunc(text, func(verb string) string {
				if verb == "%%" {
					return "%"
				}
				return messageHole
			})
			return holes, holes == text
		}
		return text, true
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return messageHole, false
		}
		left, leftComplete := messageTextOf(e.X, format)
		right, rightComplete := messageTextOf(e.Y, format)
		return left + right, leftComplete && rightComplete
	case *ast.ParenExpr:
		return messageTextOf(e.X, format)
	case *ast.CallExpr:
		if name := messageCallName(e); (name == "fmt.Sprintf" || name == "fmt.Errorf") && len(e.Args) > 0 {
			text, _ := messageTextOf(e.Args[0], true)
			return text, false
		}
	}
	return messageHole, false
}

// messageCallName is a call's callee as written: pkg.Func, recv.Method's
// "Method", or a plain function name.
func messageCallName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		if pkg, ok := fun.X.(*ast.Ident); ok && (pkg.Name == "fmt" || pkg.Name == "errors") {
			return pkg.Name + "." + fun.Sel.Name
		}
		return fun.Sel.Name
	}
	return ""
}

// messageIsKind says whether an expression names a refusal kind
// (KindRequest, act.KindEngine): the interface's refuse(kind, code, message).
func messageIsKind(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return strings.HasPrefix(e.Name, "Kind")
	case *ast.SelectorExpr:
		return strings.HasPrefix(e.Sel.Name, "Kind")
	}
	return false
}

// messageWriter says whether a writer expression is a person's stream.
var messageWriter = regexp.MustCompile(`(?i)std(out|err)|errStream|outStream|noteWriter`)

func messageExprString(fset *token.FileSet, source []byte, expr ast.Expr) string {
	start, end := fset.Position(expr.Pos()).Offset, fset.Position(expr.End()).Offset
	if start < 0 || end > len(source) || start > end {
		return ""
	}
	return string(source[start:end])
}

// messageViolations is what the rule finds wrong in one default text.
func messageViolations(kind, text string) []string {
	var found []string
	first, _, _ := strings.Cut(text, "\n")
	if kind != messageText && kind != messageReason {
		if length := utf8.RuneCountInString(first); length > messageLineBudget {
			found = append(found, fmt.Sprintf("long:%d", length))
		}
	}
	plain := messagePublicTerms.ReplaceAllString(text, "")
	plain = messageCommandSpan.ReplaceAllString(plain, "")
	plain = messageFlagSpan.ReplaceAllString(plain, "")
	for _, banned := range messageBanned {
		if banned.pattern.MatchString(plain) {
			found = append(found, "term:"+banned.term)
		}
	}
	if code := messageCodeToken.FindString(plain); code != "" {
		found = append(found, "code:"+code)
	}
	if placeholder := messagePlaceholder.FindString(messagePublicTerms.ReplaceAllString(text, "")); placeholder != "" {
		found = append(found, "placeholder:"+placeholder)
	}
	return found
}

// messageResolves says whether a refusal's second line resolves it: a
// metasystem command, or an explicit nothing to do.
func messageResolves(text string) bool {
	return strings.Contains(text, "metasystem ") || strings.Contains(text, "nothing to do")
}

// messageScan reads every production Go file of the module.
func messageScan(t *testing.T, module string) []messageSource {
	t.Helper()
	var sources []messageSource
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(module, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			found, err := messageScanFile(module, path)
			sources = append(sources, found...)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return sources
}

func messageScanFile(module, path string) ([]messageSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, data, 0)
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(module, path)
	rel = filepath.ToSlash(rel)
	var sources []messageSource
	for _, decl := range file.Decls {
		function := ""
		if fn, ok := decl.(*ast.FuncDecl); ok {
			function = fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) == 1 {
				recv := fn.Recv.List[0].Type
				if star, ok := recv.(*ast.StarExpr); ok {
					recv = star.X
				}
				if ident, ok := recv.(*ast.Ident); ok {
					function = ident.Name + "." + function
				}
			}
		}
		add := func(node ast.Node, kind string, expr ast.Expr, format bool, extra ...string) {
			text, _ := messageTextOf(expr, format)
			if strings.Trim(text, messageHole+" ") == "" && len(extra) == 0 {
				return
			}
			source := messageSource{File: rel, Line: fset.Position(node.Pos()).Line, Package: filepath.ToSlash(filepath.Dir(rel)),
				Function: function, Kind: kind, Text: text, Mode: messageModeFor(rel, function)}
			source.Violations = append(messageViolations(kind, text), extra...)
			sources = append(sources, source)
		}
		ast.Inspect(decl, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.CompositeLit:
				messageScanLiteral(n, add)
			case *ast.CallExpr:
				messageScanCall(fset, data, n, add)
			}
			return true
		})
	}
	return sources, nil
}

// messageScanLiteral reads an intentResult, a humanVerbRemedy or a refusal
// register prose row.
func messageScanLiteral(lit *ast.CompositeLit, add func(ast.Node, string, ast.Expr, bool, ...string)) {
	ident, ok := lit.Type.(*ast.Ident)
	if !ok {
		return
	}
	fields := map[string]ast.Expr{}
	for _, element := range lit.Elts {
		if kv, ok := element.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok {
				fields[key.Name] = kv.Value
			}
		}
	}
	switch ident.Name {
	case "intentResult":
		refused := false
		if outcome, ok := fields["Outcome"].(*ast.Ident); ok {
			refused = outcome.Name == "intentRefused" || outcome.Name == "intentFailed"
		}
		if summary, ok := fields["Summary"]; ok {
			var extra []string
			if refused {
				_, hasNext := fields["next"]
				_, hasRetry := fields["retry"] // line 2 is the typed command again
				decision, hasDecision := fields["Decision"]
				switch {
				case hasNext, hasRetry:
				case !hasDecision:
					extra = append(extra, "no-command")
				default:
					if text, complete := messageTextOf(decision, false); complete && !messageResolves(text) {
						extra = append(extra, "no-command")
					}
				}
			}
			add(lit, messageSummary, summary, false, extra...)
		}
		if decision, ok := fields["Decision"]; ok {
			add(lit, messageDecision, decision, false)
		}
		if reason, ok := fields["nextReason"]; ok {
			add(lit, messageReason, reason, false)
		}
		if text, ok := fields["text"].(*ast.CompositeLit); ok {
			for _, line := range text.Elts {
				add(line, messageText, line, false)
			}
		}
	case "humanVerbRemedy":
		if words, ok := fields["words"]; ok {
			add(lit, messageDecision, words, false)
		}
	case "processRefusal":
		if sentence, ok := fields["sentence"]; ok {
			add(lit, messageSummary, sentence, false)
		}
		if second, ok := fields["second"]; ok {
			add(lit, messageDecision, second, false)
		}
		if plain, ok := fields["plain"]; ok {
			add(lit, messagePrint, plain, false)
		}
	case "Prose":
		if prose, ok := fields["Prose"]; ok {
			add(lit, messageProse, prose, false)
		}
	}
}

// messageScanCall reads the calls that print or return a message.
func messageScanCall(fset *token.FileSet, source []byte, call *ast.CallExpr, add func(ast.Node, string, ast.Expr, bool, ...string)) {
	name := messageCallName(call)
	switch {
	case name == "refuse" && len(call.Args) == 3 && messageIsKind(call.Args[0]):
		// The interface's refuse(kind, code, message): the message is what
		// the page shows, the code routes it; the page offers the action.
		add(call, messageError, call.Args[2], false)
	case name == "refuse" && len(call.Args) == 3:
		var extra []string
		if text, complete := messageTextOf(call.Args[2], false); complete && !messageResolves(text) {
			extra = append(extra, "no-command")
		}
		add(call, messageSummary, call.Args[1], false, extra...)
		add(call, messageDecision, call.Args[2], false)
	case name == "refuseAgain" && len(call.Args) == 3:
		// Line 2 is the same command again; the third argument says when.
		add(call, messageSummary, call.Args[1], false)
		add(call, messageReason, call.Args[2], false)
	case name == "refuseHumanVerb" && len(call.Args) == 4:
		add(call, messageSummary, call.Args[2], false)
	case name == "fmt.Errorf" && len(call.Args) > 0:
		add(call, messageError, call.Args[0], true)
	case name == "errors.New" && len(call.Args) == 1:
		add(call, messageError, call.Args[0], false)
	case (name == "complain" || name == "complainf") && len(call.Args) > 0:
		add(call, messageError, call.Args[0], name == "complainf")
	case name == "note" && len(call.Args) == 2:
		add(call, messagePrint, call.Args[1], false)
	case (name == "fmt.Fprintf" || name == "fmt.Fprintln" || name == "fmt.Fprint") && len(call.Args) > 1:
		if messageWriter.MatchString(messageExprString(fset, source, call.Args[0])) {
			add(call, messagePrint, call.Args[1], name == "fmt.Fprintf")
		}
	case (name == "fmt.Printf" || name == "fmt.Println" || name == "fmt.Print") && len(call.Args) > 0:
		add(call, messagePrint, call.Args[0], name == "fmt.Printf")
	case name == "append" && len(call.Args) > 1:
		// Lines appended to a result's text are printed by default.
		if sel, ok := call.Args[0].(*ast.SelectorExpr); ok && sel.Sel.Name == "text" {
			for _, line := range call.Args[1:] {
				add(line, messageText, line, false)
			}
		}
	}
}

// TestAuditMessagesAPersonReads checks every message source against the
// rule; a violation fails the test only where the mode table enforces it.
// METASYSTEM_MESSAGE_INVENTORY=DIR also writes the inventory there.
func TestAuditMessagesAPersonReads(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sources := messageScan(t, module)
	if len(sources) < 1000 {
		t.Fatalf("the scan found %d message sources; the module holds thousands, so the scan is broken", len(sources))
	}
	reported := 0
	for _, source := range sources {
		if len(source.Violations) == 0 {
			continue
		}
		if source.Mode == messageModeEnforce {
			t.Errorf("%s:%d (%s) %s %q: %s", source.File, source.Line, source.Function, source.Kind, source.Text, strings.Join(source.Violations, ", "))
			continue
		}
		reported++
	}
	t.Logf("%d message sources, %d with violations in report mode", len(sources), reported)
	if dir := os.Getenv("METASYSTEM_MESSAGE_INVENTORY"); dir != "" {
		writeMessageInventory(t, dir, sources)
	}
}

// TestAuditAdmissionNoticesWaitForTheOutcome: the helm and grant admitters
// never print; their notice goes through the admission board, which prints it
// only when the act proceeds (no helm notice before a refusal it does not
// cover).
func TestAuditAdmissionNoticesWaitForTheOutcome(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"helm_admits.go", "attorney_admits.go"} {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok && strings.HasPrefix(messageCallName(call), "fmt.Fprint") {
				t.Errorf("%s:%d prints directly; an admission notice goes through admissionNotices.say", file, fset.Position(call.Pos()).Line)
			}
			return true
		})
	}
}

// TestAuditMessageModesNameRealPaths keeps the mode table honest: every key
// names a directory, file or function that exists.
func TestAuditMessageModesNameRealPaths(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sources := messageScan(t, module)
	functions := map[string]bool{}
	for _, source := range sources {
		functions[source.File+"#"+source.Function] = true
	}
	for key, modes := range auditModes {
		mode := modes.messages
		if mode == "" {
			continue
		}
		if mode != messageModeEnforce {
			t.Errorf("%s: mode %q; the table holds only enforced paths", key, mode)
		}
		path, function, isFunction := strings.Cut(key, "#")
		if _, err := os.Stat(filepath.Join(module, path)); err != nil {
			t.Errorf("%s names no path: %v", key, err)
		}
		if isFunction && !functions[path+"#"+function] {
			t.Errorf("%s names no function with a message", key)
		}
	}
}

// TestAuditMessageRuleCatchesItsTrigger holds the checks themselves to the
// 2026-09-30 grant add refusal that started the rule, and to its rewrite.
func TestAuditMessageRuleCatchesItsTrigger(t *testing.T) {
	t.Parallel()
	before := []struct{ kind, text, want string }{
		{messagePrint, "HUMAN AT THE HELM (wido): the person proof yields to the helm for grant add; caller HUMAN; recorded in /x/helm-yields.log", "term:yield"},
		{messageSummary, "a general power of attorney is the person's own act at the enrolled terminal, never at the helm or under a grant; nothing was done", "long:"},
		{messageDecision, "a person, at a terminal no agent started, enrolls it once with metasystem system enroll --name NAME, then runs metasystem grant add --acts everything --for 24h there", "placeholder:NAME"},
		{messageSummary, "stop refused: system stop is a human act at a terminal; this caller is DELEGATE", "term:caller class"},
		{messageError, "TERMINAL_NOT_REACHED: human authority has no readable terminal enrollment", "code:TERMINAL_NOT_REACHED"},
	}
	for _, test := range before {
		if found := strings.Join(messageViolations(test.kind, test.text), ","); !strings.Contains(found, test.want) {
			t.Errorf("%q: found %q, want %q", test.text, found, test.want)
		}
	}
	for _, after := range []string{
		"this terminal isn't enrolled yet, so nothing was done",
		"metasystem system enroll --name wido",
		"HUMAN AT THE HELM (wido): goal done g1 is recorded as wido's act",
	} {
		if found := messageViolations(messageSummary, after); len(found) != 0 {
			t.Errorf("%q: %v", after, found)
		}
	}
	if messageResolves("run it at your enrolled terminal") || !messageResolves("metasystem system enroll --name wido") || !messageResolves("nothing to do") {
		t.Error("messageResolves misreads a second line")
	}
}

// writeMessageInventory writes inventory.json (every source) and
// inventory.md (per-package counts and the proposed rewrite partition).
func writeMessageInventory(t *testing.T, dir string, sources []messageSource) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(sources, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inventory.json"), append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inventory.md"), []byte(messageInventoryMarkdown(sources)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// messageInventoryMarkdown summarizes the inventory per package and
// proposes the partition of the reported violations among rewrite builders.
func messageInventoryMarkdown(sources []messageSource) string {
	type tally struct {
		sources, violating, enforced int
		byKind                       map[string]int
	}
	packages := map[string]*tally{}
	kinds := map[string]int{}
	totalViolating := 0
	for _, source := range sources {
		entry := packages[source.Package]
		if entry == nil {
			entry = &tally{byKind: map[string]int{}}
			packages[source.Package] = entry
		}
		entry.sources++
		if source.Mode == messageModeEnforce {
			entry.enforced++
		}
		if len(source.Violations) == 0 {
			continue
		}
		entry.violating++
		totalViolating++
		for _, violation := range source.Violations {
			kind, _, _ := strings.Cut(violation, ":")
			entry.byKind[kind]++
			kinds[kind]++
		}
	}
	names := make([]string, 0, len(packages))
	for name := range packages {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if packages[names[i]].violating != packages[names[j]].violating {
			return packages[names[i]].violating > packages[names[j]].violating
		}
		return names[i] < names[j]
	})
	kindNames := []string{"long", "term", "code", "placeholder", "no-command"}
	var b strings.Builder
	fmt.Fprintf(&b, "# Message inventory\n\nGenerated by `METASYSTEM_MESSAGE_INVENTORY=DIR go test ./cmd/metasystem -run TestAuditMessagesAPersonReads`.\n")
	fmt.Fprintf(&b, "Rule: \"Messages a Person Reads\" in docs/design/design-principles.md. Each source is one message site (inventory.json has file:line, kind, text, violations).\n\n")
	fmt.Fprintf(&b, "%d message sources; %d with at least one violation.\n\n", len(sources), totalViolating)
	b.WriteString("Limits of the scan: it reads each message where it is written. A line a helper builds and returns, or appends to a local slice, " +
		"is not seen; no-command is judged only where the Decision is a literal; a placeholder is flagged even where the engine cannot know the value, " +
		"so each needs a look; an error is counted although some never reach a person. The helm-notice contradiction is held by " +
		"TestAuditAdmissionNoticesWaitForTheOutcome and the end-to-end TestMessageGrantAddAtTheHelmIsOneRefusal, not by this scan.\n\n")
	fmt.Fprintf(&b, "| violation | count |\n|---|---|\n")
	for _, kind := range kindNames {
		fmt.Fprintf(&b, "| %s | %d |\n", kind, kinds[kind])
	}
	fmt.Fprintf(&b, "\n## Per package\n\n| package | sources | violating | enforced | long | term | code | placeholder | no-command |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, name := range names {
		entry := packages[name]
		fmt.Fprintf(&b, "| %s | %d | %d | %d |", name, entry.sources, entry.violating, entry.enforced)
		for _, kind := range kindNames {
			fmt.Fprintf(&b, " %d |", entry.byKind[kind])
		}
		b.WriteString("\n")
	}
	b.WriteString(messagePartitionMarkdown(sources))
	return b.String()
}

// messagePartitionThemes are the rewrite groups, each a theme a builder can
// hold in mind: a unit (an internal package whole, or one cmd/metasystem
// file) joins the first theme whose pattern matches it, and the rest is the
// system group. The sizes were balanced on the 2026-09-30 inventory.
var messagePartitionThemes = []struct {
	name     string
	patterns []*regexp.Regexp
}{
	{"goal ledger and goal verbs", []*regexp.Regexp{regexp.MustCompile(`^internal/goal`), regexp.MustCompile(`^cmd/metasystem/(goal|goalsync|intent_goal)`)}},
	{"work, design and review verbs", []*regexp.Regexp{regexp.MustCompile(`^cmd/metasystem/intent_(delivery|work|selection|unit_review|design|review_binding|sent_back|references|worktree|table|planning)`)}},
	{"landing, exceptions and manual reviews", []*regexp.Regexp{regexp.MustCompile(`^internal/(landing|ledgerfence|gittree)`),
		regexp.MustCompile(`^cmd/metasystem/(intent_land|landing|hold|holder_step|intent_exception|intent_evidence|intent_manual)`)}},
	{"proofs, tests and gates", []*regexp.Regexp{regexp.MustCompile(`^internal/(proofrun|testrun|gaterun|testenv|candidateengine|enginecause|audit|validate|testpolicy)$`),
		regexp.MustCompile(`^cmd/metasystem/(proof_run|test|testing|validate_verbs)`), regexp.MustCompile(`^cmd/devgate`)}},
	{"machinery: dispatch, launch, steward, supervision, agents", []*regexp.Regexp{
		regexp.MustCompile(`^internal/(dispatch|launch|steward|supervise|run|delegation|missionrunner|brain|seat|adapter|acp|board|stopreport|stoptransition|up|lease|hooks)(/|$)`),
		regexp.MustCompile(`^cmd/metasystem/(steward|supervise|delegate|launch|dispatch|brain|seat|adapter|channel|up\.go|wait|session|context|hook|mission|intent_agent|intent_questions)`)}},
}

// messagePartitionMarkdown proposes the rewrite partition: the reported
// violations in units that stay whole (an internal package, or one
// cmd/metasystem file), grouped by theme, so no file is in two groups.
func messagePartitionMarkdown(sources []messageSource) string {
	units := map[string]int{}
	for _, source := range sources {
		if len(source.Violations) == 0 || source.Mode == messageModeEnforce {
			continue
		}
		unit := source.Package
		if source.Package == "cmd/metasystem" {
			unit = source.File
		}
		units[unit]++
	}
	groups := make([][]string, len(messagePartitionThemes)+1)
	weights := make([]int, len(groups))
	names := make([]string, len(groups))
	for index, theme := range messagePartitionThemes {
		names[index] = theme.name
	}
	names[len(names)-1] = "system, app, disk, evidence, interface and the rest"
	for unit, weight := range units {
		group := len(messagePartitionThemes)
		for index, theme := range messagePartitionThemes {
			if slices.ContainsFunc(theme.patterns, func(pattern *regexp.Regexp) bool { return pattern.MatchString(unit) }) {
				group = index
				break
			}
		}
		groups[group] = append(groups[group], unit)
		weights[group] += weight
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Proposed partition for %d rewrite builders\n\n", len(groups))
	b.WriteString("Each group lists its units (an internal package whole, or one cmd/metasystem file) with the violating sources in it; no file is in two groups. " +
		"Paths already enforced are done and left out; a file partly enforced lists its remaining sources. " +
		"A group enforces its paths from cmd/metasystem/message_modes_<group>_test.go with enforceMessages.\n")
	for index, group := range groups {
		sort.Strings(group)
		fmt.Fprintf(&b, "\n### Group %d, %s: %d violating sources\n\n", index+1, names[index], weights[index])
		for _, unit := range group {
			fmt.Fprintf(&b, "- %s (%d)\n", unit, units[unit])
		}
	}
	return b.String()
}
