package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The SessionStart audit reads the runtime hook's Go start path and its
// executed start cases. The start behavior itself is tested in
// internal/hooks (runtime_hook_start_test.go); these tests prove the audit
// accepts the production boundary and refuses each bypass.

func readHookStartInputs(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Join("..", "..")
	source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(hookStartSourcePath)))
	if err != nil {
		t.Fatal(err)
	}
	assertions, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(hookStartAssertionPath)))
	if err != nil {
		t.Fatal(err)
	}
	return string(source), string(assertions)
}

func requireHookStartFinding(t *testing.T, source, assertions, want string) {
	t.Helper()
	findings := auditHookStartSource(hookStartSourcePath, source, assertions)
	for _, finding := range findings {
		if strings.Contains(finding.Invariant+" "+finding.Detail, want) {
			return
		}
	}
	t.Fatalf("missing finding containing %q: %#v", want, findings)
}

func TestAuditHookStartExitsAcceptsProductionHook(t *testing.T) {
	findings, err := AuditHookStartExits(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("production start boundary findings: %#v", findings)
	}
}

func TestAuditHookStartExitsFailsClosedWhenProofInputIsMissing(t *testing.T) {
	source, _ := readHookStartInputs(t)
	root := t.TempDir()
	sourcePath := filepath.Join(root, filepath.FromSlash(hookStartSourcePath))
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := AuditHookStartExits(root)
	assertion := filepath.Join(root, filepath.FromSlash(hookStartAssertionPath))
	if err == nil || !strings.Contains(err.Error(), "hook start exit audit could not read "+assertion) {
		t.Fatalf("missing assertion source = %v", err)
	}
	if _, err := AuditHookStartExits(t.TempDir()); err == nil || !strings.Contains(err.Error(), "could not read") {
		t.Fatalf("missing start source = %v", err)
	}
}

// Each way around the outcome owner is refused.
func TestAuditHookStartExitsRejectsBypasses(t *testing.T) {
	source, assertions := readHookStartInputs(t)
	const anchor = "func (s *startRun) main() {\n"
	if !strings.Contains(source, anchor) {
		t.Fatal("the production start has no main")
	}
	inject := func(statement string) string { return strings.Replace(source, anchor, anchor+"\t"+statement+"\n", 1) }
	for _, test := range []struct{ name, source, want string }{
		{"direct exit", inject("exitHook(0)"), "start exit"},
		{"process exit", inject("os.Exit(0)"), "exits the process around the outcome owner"},
		{"undeclared notice", inject(`s.finish("notice", "undeclared")`), `notice outcome "undeclared" is not declared`},
		{"undeclared intentional", inject(`s.finish("intentional", "undeclared")`), `intentional outcome "undeclared" is not declared`},
		{"runtime outcome", inject(`s.finish("notice", s.session)`), "names its outcome at run time"},
		{"wrong arity", inject(`s.finish("notice")`), "names its outcome at run time"},
		{"unknown family", inject(`s.finish("other", "arming")`), `family "other"`},
		{"direct publication", inject(`_ = writeLine(s.inv.Stdout, "{}")`), "reaches stdout outside the outcome owner"},
		{"no recovery", strings.Replace(source, `s.finish("notice", "unexpected-termination")`+"\n\t\t}", "_ = 0\n\t\t}", 1), "start unexpected termination"},
		{"unparseable", source + "\nfunc (", "does not parse"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.source == source {
				t.Fatal("the mutation did not change the source")
			}
			requireHookStartFinding(t, test.source, assertions, test.want)
		})
	}
	catalogless := strings.Replace(source, "var StartOutcomeNotices = map[string]startNotice{", "var startOutcomeNoticesRenamed = map[string]startNotice{", 1)
	requireHookStartFinding(t, catalogless, assertions, "start catalog")
}

// A start outcome without its executed case is refused.
func TestAuditHookStartExitsRejectsMissingCoverage(t *testing.T) {
	source, assertions := readHookStartInputs(t)
	for _, name := range hookStartCases {
		mutated := strings.Replace(assertions, "func "+name+"(", "func renamed"+name+"(", 1)
		if mutated == assertions {
			t.Fatalf("assertions do not declare %s", name)
		}
		requireHookStartFinding(t, source, mutated, "the executed start case "+name+" is missing")
	}
	requireHookStartFinding(t, source, strings.Replace(assertions, "range StartOutcomeNotices", "range []string{}", 1), "declared outcome matrix")
	requireHookStartFinding(t, source, strings.Replace(assertions, `{"foreign-runtime",`, `{"renamed-runtime",`, 1), `intentional outcome "foreign-runtime" has no executed case`)
}

// The command prints each finding as one line: invariant, then the position,
// then the detail.
func TestHookStartExitFindingPrintsOneLine(t *testing.T) {
	t.Parallel()
	finding := HookStartExitFinding{Path: hookStartSourcePath, Line: 12, Invariant: "start exit", Detail: "main ends the start outside the outcome owner"}
	if got, want := finding.String(), "start exit: "+hookStartSourcePath+":12: main ends the start outside the outcome owner"; got != want {
		t.Fatalf("finding line = %q, want %q", got, want)
	}
}
