package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const lanePredicatePackage = "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"

var lanePredicateAllowlist = map[string]bool{}
var lanePredicateName = regexp.MustCompile(`(?i)abandoned|owned|admissib|eligib|isPerson|premise|stopOwner`)
var lanePredicateDoc = regexp.MustCompile(`(?is)(decid|judg|determin|classif)[^\n]*(merge ownership|admissib|eligib|person.*agent|premise|stop.?owner)`)

type lanePredicateFunction struct {
	name, path string
	decl       *ast.FuncDecl
	file       *ast.File
}

// auditLanePredicates accepts source roots so its own mutations use fixtures.
func auditLanePredicates(roots []string, required map[string]string) ([]string, error) {
	set := token.NewFileSet()
	functions := map[string]lanePredicateFunction{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			// The lane's predicates live in the lane: the command package is
			// walked for its landing files only, so a goal verb's own words
			// ("owned", "abandoned") stay its own.
			if root == "." && !strings.Contains(filepath.Base(path), "landing") {
				return nil
			}
			file, err := parser.ParseFile(set, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				key := filepath.Dir(path) + "/" + fn.Name.Name
				if fn.Recv != nil {
					key += ":" + set.Position(fn.Pos()).String()
				}
				functions[key] = lanePredicateFunction{fn.Name.Name, path, fn, file}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	predicateCalls := func(fn lanePredicateFunction) map[string]bool {
		aliases := map[string]bool{}
		for _, imp := range fn.file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if path == lanePredicatePackage {
				alias := "state"
				if imp.Name != nil {
					alias = imp.Name.Name
				}
				aliases[alias] = true
			}
		}
		predicates := map[string]bool{}
		ast.Inspect(fn.decl.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && aliases[id.Name] {
					predicates[sel.Sel.Name] = true
				}
			}
			return true
		})
		return predicates
	}
	var failures []string
	found := map[string]bool{}
	for _, fn := range functions {
		if filepath.Clean(filepath.Dir(fn.path)) == filepath.Clean(filepath.Join(roots[0], "state")) {
			continue
		}
		where := set.Position(fn.decl.Pos()).String()
		calls := predicateCalls(fn)
		if predicate, requiredCaller := required[fn.name]; requiredCaller {
			found[fn.name] = true
			if !calls[predicate] {
				failures = append(failures, fmt.Sprintf("%s: %s must call %s", where, fn.name, predicate))
			}
		}
		if !lanePredicateName.MatchString(fn.name) && !lanePredicateDoc.MatchString(fn.decl.Doc.Text()) {
			continue
		}
		delegates := false
		for _, predicate := range []string{"MergeOwnership", "Admissibility", "Actor", "QuestionPremise", "StopOwner"} {
			delegates = delegates || calls[predicate]
		}
		localDecision := false
		ast.Inspect(fn.decl.Body, func(node ast.Node) bool {
			ret, ok := node.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			for _, result := range ret.Results {
				literal, ok := result.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				word, _ := strconv.Unquote(literal.Value)
				switch word {
				case "abandoned", "owned", "settled", "unknown", "admissible", "not admissible", "person", "agent", "current", "gone", "loop":
					localDecision = true
				}
			}
			return true
		})
		if (!delegates || localDecision) && !lanePredicateAllowlist[fn.name] {
			failures = append(failures, where+": second lane predicate "+fn.name)
		}
	}
	for name := range required {
		if !found[name] {
			failures = append(failures, "missing lane caller "+name)
		}
	}
	return failures, nil
}

func TestAuditLanePredicates(t *testing.T) {
	t.Parallel()
	if len(lanePredicateAllowlist) != 0 {
		t.Fatal("lane predicate allow-list must remain empty")
	}
	failures, err := auditLanePredicates([]string{"../../internal/landing", "."}, map[string]string{
		"Resume": "MergeOwnership", "deployMerge": "MergeOwnership", "stewardStep": "StopOwner",
		"batchAdmission": "Admissibility", "checkBatchLocked": "Admissibility", "admissionAtSeal": "Admissibility",
		"LaneQuestionPremise": "QuestionPremise", "WithdrawStaleLaneQuestion": "QuestionPremise",
		"returnOrigin": "Actor", "PersonSelection": "Actor", "closeGoneLoopStops": "StopOwner",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range failures {
		t.Error(failure)
	}
}

func TestAuditLanePredicatesFixtures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		required     map[string]string
		want         string
	}{
		{"delegation", `package plain
import shared "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
func LaneQuestionPremise() { shared.QuestionPremise(shared.PremiseFacts{}) }`, map[string]string{"LaneQuestionPremise": "QuestionPremise"}, ""},
		{"missing call", `package plain
func Resume() {}`, map[string]string{"Resume": "MergeOwnership"}, "must call MergeOwnership"},
		{"steward direct call", `package plain
import shared "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
func stewardStep() { shared.StopOwner(shared.StopFacts{}) }`, map[string]string{"stewardStep": "StopOwner"}, ""},
		{"steward inline copy", `package plain
func stewardStep(paused bool) bool { return paused }`, map[string]string{"stewardStep": "StopOwner"}, "stewardStep must call StopOwner"},
		{"steward closure parameter", `package plain
import shared "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
func stewardStep(owner func(shared.StopFacts) shared.Owner) { owner(shared.StopFacts{}) }
func StewardStep() { stewardStep(func(f shared.StopFacts) shared.Owner { return shared.StopOwner(f) }) }`, map[string]string{"stewardStep": "StopOwner"}, "stewardStep must call StopOwner"},
		{"indirect call", `package plain
import shared "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
func Resume() { helper() }
func helper() { shared.MergeOwnership(shared.MergeFacts{}) }`, map[string]string{"Resume": "MergeOwnership"}, "must call MergeOwnership"},
		{"wrong predicate", `package plain
import shared "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
func Resume() { shared.Actor(shared.ActorFacts{}) }`, map[string]string{"Resume": "MergeOwnership"}, "must call MergeOwnership"},
		{"wrong import", `package plain
import state "other/state"
func Resume() { state.MergeOwnership() }`, map[string]string{"Resume": "MergeOwnership"}, "must call MergeOwnership"},
		{"missing caller", `package plain`, map[string]string{"Resume": "MergeOwnership"}, "missing lane caller Resume"},
		{"copied name", `package plain
func mergeAbandoned() bool { return true }`, nil, "second lane predicate mergeAbandoned"},
		{"copy with token call", `package plain
import shared "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
func mergeAbandoned() string { shared.MergeOwnership(shared.MergeFacts{}); return "abandoned" }`, nil, "second lane predicate mergeAbandoned"},
		{"copied doc", `package plain
// pick decides merge ownership.
func pick() bool { return true }`, nil, "second lane predicate pick"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			failures, err := auditLanePredicates([]string{root}, tc.required)
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Join(failures, "\n")
			if tc.want == "" && len(failures) != 0 || tc.want != "" && !strings.Contains(text, tc.want) {
				t.Fatalf("failures=%v, want %q", failures, tc.want)
			}
			if tc.want != "" && tc.name != "missing caller" && !strings.Contains(text, "sample.go:2:") && !strings.Contains(text, "sample.go:3:") {
				t.Fatalf("missing file:line: %s", text)
			}
		})
	}
}
