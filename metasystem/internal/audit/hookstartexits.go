package audit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The SessionStart boundary is the runtime hook's Go start path
// (internal/hooks/runtime_hook_start.go). Every start termination goes through
// one outcome owner, finish, which publishes a declared notice or a validated
// intentional response. This audit refuses a start construct that can bypass
// that owner and joins the declared outcome catalog to the executed start
// cases in the hook's tests.

const (
	hookStartSourcePath    = "internal/hooks/runtime_hook_start.go"
	hookStartAssertionPath = "internal/hooks/runtime_hook_start_test.go"
)

// HookStartExitFinding names a start construct that can bypass the SessionStart
// outcome owner or an outcome that has no executed case.
type HookStartExitFinding struct {
	Path      string
	Line      int
	Invariant string
	Detail    string
}

func (finding HookStartExitFinding) String() string {
	return fmt.Sprintf("%s: %s:%d: %s", finding.Invariant, finding.Path, finding.Line, finding.Detail)
}

// hookStartOwners are the functions that may end the start: the outcome
// owner and its last resort.
var hookStartOwners = map[string]bool{"finish": true, "emergency": true}

// hookStartCases are the executed start tests the catalog joins: the
// declared-outcome matrix and the full, failure, brain, custody,
// post-publication and signal paths.
var hookStartCases = []string{
	"TestHookStartDeclaredOutcomeMatrix",
	"TestHookStartIntentionalFullPathFixtures",
	"TestHookStartFailureBranchFixtures",
	"TestHookStartBrainFailureKeepsWaitLine",
	"TestHookStartForgedDelegateHintRefuses",
	"TestHookStartPostPreparationFixtures",
	"TestHookStartSignalsUseOwnedStatuses",
	"TestHookStartUnexpectedTerminationPublishesTheLastResort",
	"TestHookStartLastResortUsesStderrWhenStdoutIsClosed",
}

// AuditHookStartExits audits the SessionStart path of the runtime hook.
func AuditHookStartExits(root string) ([]HookStartExitFinding, error) {
	sourcePath := filepath.Join(root, filepath.FromSlash(hookStartSourcePath))
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("hook start exit audit could not read %s: %w", sourcePath, err)
	}
	assertionPath := filepath.Join(root, filepath.FromSlash(hookStartAssertionPath))
	assertions, err := os.ReadFile(assertionPath)
	if err != nil {
		return nil, fmt.Errorf("hook start exit audit could not read %s: %w", assertionPath, err)
	}
	return auditHookStartSource(hookStartSourcePath, string(source), string(assertions)), nil
}

func auditHookStartSource(path, source, assertions string) []HookStartExitFinding {
	var findings []HookStartExitFinding
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, path, source, 0)
	if err != nil {
		return []HookStartExitFinding{{path, 1, "start source", "the start source does not parse: " + err.Error()}}
	}
	add := func(position token.Pos, invariant, detail string) {
		findings = append(findings, HookStartExitFinding{path, files.Position(position).Line, invariant, detail})
	}
	notices, intentional := hookStartCatalog(file)
	if len(notices) == 0 || len(intentional) == 0 {
		add(file.Package, "start catalog", "StartOutcomeNotices and StartIntentionalOutcomes must declare the start outcomes")
	}

	recovers := false
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		owner := hookStartOwners[function.Name.Name]
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.CallExpr:
				name := hookStartCallName(node.Fun)
				switch name {
				case "exitHook":
					if !owner {
						add(node.Pos(), "start exit", fmt.Sprintf("%s ends the start outside the outcome owner", function.Name.Name))
					}
				case "os.Exit":
					add(node.Pos(), "start exit", fmt.Sprintf("%s exits the process around the outcome owner", function.Name.Name))
				case "finish":
					family, key, literal := hookStartFinishArguments(node)
					switch {
					case !literal:
						add(node.Pos(), "start outcome", "a start termination names its outcome at run time")
					case family == "notice" && !notices[key], family == "intentional" && !intentional[key]:
						add(node.Pos(), "start outcome", fmt.Sprintf("%s outcome %q is not declared in the start catalog", family, key))
					case family != "notice" && family != "intentional":
						add(node.Pos(), "start outcome", fmt.Sprintf("start outcome family %q is neither notice nor intentional", family))
					}
				}
			case *ast.FuncLit:
				if function.Name.Name == "runStart" && hookStartRecoversIntoOwner(node) {
					recovers = true
				}
			case *ast.SelectorExpr:
				if node.Sel.Name == "Stdout" && !owner {
					add(node.Pos(), "start publication", fmt.Sprintf("%s reaches stdout outside the outcome owner", function.Name.Name))
				}
			}
			return true
		})
	}
	if !recovers {
		add(file.Package, "start unexpected termination", "runStart must answer an unexpected failure through the outcome owner")
	}

	for _, name := range hookStartCases {
		if !regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(name) + `\(`).MatchString(assertions) {
			findings = append(findings, HookStartExitFinding{hookStartAssertionPath, 1, "start fixture coverage", fmt.Sprintf("the executed start case %s is missing", name)})
		}
	}
	if !strings.Contains(assertions, "range StartOutcomeNotices") || !strings.Contains(assertions, "len(StartIntentionalOutcomes)") {
		findings = append(findings, HookStartExitFinding{hookStartAssertionPath, 1, "start fixture coverage", "the declared outcome matrix must execute every catalog outcome"})
	}
	for _, key := range sortedStartKeys(intentional) {
		if !regexp.MustCompile(`(?m)^\s*\{"` + regexp.QuoteMeta(key) + `",`).MatchString(assertions) {
			findings = append(findings, HookStartExitFinding{hookStartAssertionPath, 1, "start fixture coverage", fmt.Sprintf("intentional outcome %q has no executed case", key)})
		}
	}
	return findings
}

func sortedStartKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// hookStartCatalog reads the declared notice keys and intentional outcomes.
func hookStartCatalog(file *ast.File) (map[string]bool, map[string]bool) {
	notices, intentional := map[string]bool{}, map[string]bool{}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
				continue
			}
			literal, ok := value.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, element := range literal.Elts {
				switch value.Names[0].Name {
				case "StartOutcomeNotices":
					if pair, ok := element.(*ast.KeyValueExpr); ok {
						if key, ok := stringLiteral(pair.Key); ok {
							notices[key] = true
						}
					}
				case "StartIntentionalOutcomes":
					if key, ok := stringLiteral(element); ok {
						intentional[key] = true
					}
				}
			}
		}
	}
	return notices, intentional
}

func hookStartCallName(function ast.Expr) string {
	switch function := function.(type) {
	case *ast.Ident:
		return function.Name
	case *ast.SelectorExpr:
		if receiver, ok := function.X.(*ast.Ident); ok && receiver.Name == "os" {
			return "os." + function.Sel.Name
		}
		return function.Sel.Name
	}
	return ""
}

func hookStartFinishArguments(call *ast.CallExpr) (string, string, bool) {
	if len(call.Args) != 2 {
		return "", "", false
	}
	family, familyLiteral := stringLiteral(call.Args[0])
	key, keyLiteral := stringLiteral(call.Args[1])
	return family, key, familyLiteral && keyLiteral
}

// hookStartRecoversIntoOwner reports whether a deferred function recovers a
// failure and answers it with the unexpected-termination outcome.
func hookStartRecoversIntoOwner(literal *ast.FuncLit) bool {
	recovered, answered := false, false
	ast.Inspect(literal.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch hookStartCallName(call.Fun) {
		case "recover":
			recovered = true
		case "finish":
			if family, key, literal := hookStartFinishArguments(call); literal && family == "notice" && key == "unexpected-termination" {
				answered = true
			}
		}
		return true
	})
	return recovered && answered
}
