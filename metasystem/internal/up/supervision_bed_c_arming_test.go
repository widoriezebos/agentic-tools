package up

// Ported from scripts/agents/supervision-fixtures.sh, scenario
// census-lifecycle: S4-8 (omitted identity arguments are inferred from the
// caller's runtime-signature ancestry and the session environment) and the
// arming-log rule that the session announcement is written before the first
// census, so the arming session is never labelled UNTRACKED.

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestSupCOmittedIdentityIsInferredFromTheRuntimeAncestor(t *testing.T) {
	t.Setenv("METASYSTEM_SESSION_ID", "inferred-session")
	t.Setenv("METASYSTEM_AGENT_RUNTIME", "fake")
	t.Setenv("METASYSTEM_INSTANCE_TAG", "")
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "")
	root := t.TempDir()
	var askedPid int64
	var askedRuntime, askedRoot string
	options := Options{Root: root, Scope: root, Binary: root + "/bin/metasystem",
		FindSessionAncestor: func(metasystemRoot string, pid int64, runtime string) (census.AgentAncestor, error) {
			askedRoot, askedPid, askedRuntime = metasystemRoot, pid, runtime
			return census.AgentAncestor{Pid: 4242, PidStartedAt: 1786000000, Pgid: 4242, Runtime: "fake",
				Argv: "metasystem-fake-agent"}, nil
		}}
	session, err := resolveSessionIdentity(options)
	if err != nil {
		t.Fatal(err)
	}
	if askedRoot != root || askedPid != int64(os.Getppid()) || askedRuntime != "fake" {
		t.Fatalf("the ancestry walk did not start at the caller's parent for the environment runtime: root=%q pid=%d runtime=%q",
			askedRoot, askedPid, askedRuntime)
	}
	if session.Session != "inferred-session" || session.Pid != 4242 || session.StartTime != 1786000000 ||
		session.Runtime != "fake" || session.Tag != "metasystem-main-fake-inferred-session" ||
		session.Provenance.Source != "runtime-signature-ancestry" {
		t.Fatalf("S4-8: inferred session identity = %+v", session)
	}

	options.FindSessionAncestor = func(string, int64, string) (census.AgentAncestor, error) {
		return census.AgentAncestor{}, errors.New("no agent-signature ancestor")
	}
	if _, err := resolveSessionIdentity(options); err == nil || !strings.Contains(err.Error(), "runtime-signature ancestry proof failed") {
		t.Fatalf("an absent ancestor was not refused by name: %v", err)
	}
}

// The ordinary arming transaction announces the session (and logs
// announcement-written) before it ensures supervision (and logs
// first-census-complete). The order is the transaction's own code order, so
// it is read from the shipped source of ordinaryBody.
func TestSupCArmingAnnouncesBeforeTheFirstCensus(t *testing.T) {
	t.Parallel()
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, "up.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.FuncDecl
	for _, declaration := range parsed.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Name.Name == "ordinaryBody" {
			body = function
		}
	}
	if body == nil {
		t.Fatal("up.go no longer defines ordinaryBody")
	}
	positions := map[string]token.Pos{}
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := ""
		switch function := call.Fun.(type) {
		case *ast.Ident:
			name = function.Name
		case *ast.SelectorExpr:
			name = function.Sel.Name
		}
		switch name {
		case "AnnounceWithProofAt", "ensureSupervision":
			if _, seen := positions[name]; !seen {
				positions[name] = call.Pos()
			}
		case "appendArmingLog":
			ast.Inspect(call, func(inner ast.Node) bool {
				literal, ok := inner.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				text, err := strconv.Unquote(literal.Value)
				if err != nil {
					return true
				}
				for _, event := range []string{"announcement-written", "first-census-complete"} {
					if strings.HasPrefix(text, event) {
						if _, seen := positions[event]; !seen {
							positions[event] = call.Pos()
						}
					}
				}
				return true
			})
		}
		return true
	})
	for _, name := range []string{"AnnounceWithProofAt", "announcement-written", "ensureSupervision", "first-census-complete"} {
		if _, ok := positions[name]; !ok {
			t.Fatalf("ordinaryBody no longer carries %s: %v", name, positions)
		}
	}
	if !(positions["AnnounceWithProofAt"] < positions["announcement-written"] &&
		positions["announcement-written"] < positions["ensureSupervision"] &&
		positions["ensureSupervision"] < positions["first-census-complete"]) {
		t.Fatalf("arming does not announce before the first census: %v", positions)
	}
}

// stop-hook-monitor's arming notice: an enrolled engine that needs no re-arm
// reports its accepted-engine component as verified, never as a rejection.
func TestSupCAnEnrolledEngineIsReportedVerified(t *testing.T) {
	t.Parallel()
	enrolled := &steward.EnrolledBinary{Install: steward.InstallIdentity{Generation: 1, InstallPath: "/fixture/bin/metasystem"}}
	for _, rearmed := range []steward.ReArmOutcome{{}, {Status: "already-current"}} {
		outcome := acceptedEngine(enrolled, rearmed)
		if outcome.Component != "accepted-engine" || outcome.Outcome != "verified" ||
			!strings.Contains(outcome.Detail, "generation=1 path=/fixture/bin/metasystem") {
			t.Fatalf("status %q: accepted engine = %+v", rearmed.Status, outcome)
		}
		lines := Result{Components: []ComponentOutcome{outcome}, Outcome: "armed", Authority: "writer"}.Lines()
		if len(lines) != 2 || !strings.HasPrefix(lines[0], "component=accepted-engine outcome=verified ") ||
			lines[1] != "up outcome=armed authority=writer" {
			t.Fatalf("armed lines = %q", lines)
		}
	}
}
