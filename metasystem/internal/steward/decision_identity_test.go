package steward

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func declaredCauses(t *testing.T, path, typeName string) (*token.FileSet, *ast.File, map[string]string) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	causes := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			typ, ok := value.Type.(*ast.Ident)
			if !ok || typ.Name != typeName {
				continue
			}
			for i, name := range value.Names {
				literal, ok := value.Values[i].(*ast.BasicLit)
				if !ok {
					t.Fatalf("%s: cause must have a string value", fset.Position(value.Pos()))
				}
				text, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				causes[name.Name] = text
			}
		}
	}
	if len(causes) == 0 {
		t.Fatalf("no %s constants found", typeName)
	}
	return fset, file, causes
}

func TestEveryRemedyCauseIsClassified(t *testing.T) {
	t.Parallel()
	_, _, declared := declaredCauses(t, "health.go", "RemedyCause")
	for name, value := range declared {
		if _, ok := remedyCauseNeedsAPerson[RemedyCause(value)]; !ok {
			t.Errorf("%s (%s) has no remedy classification", name, value)
		}
	}
	for cause := range remedyCauseNeedsAPerson {
		found := false
		for _, value := range declared {
			found = found || string(cause) == value
		}
		if !found {
			t.Errorf("classification names undeclared remedy cause %q", cause)
		}
	}
	// These are the causes whose remedy requires a person; the others are machine work.
	person := map[RemedyCause]bool{CauseUnreadable: true, CauseSettingsInvalid: true, CauseStopFenceClosed: true, CauseJobProcessDead: true, CauseTrunkRedUnowned: true, CauseBudgetMissing: true, CauseBudgetMalformed: true, CauseBudgetUnknown: true, CauseBreachStopUnresolved: true, CauseForeignLineage: true, CauseStopCapabilityMissing: true}
	for cause, got := range remedyCauseNeedsAPerson {
		if got != person[cause] {
			t.Errorf("%s needs a person = %t, want %t", cause, got, person[cause])
		}
	}
}

func TestDegradedTickNamesItsCause(t *testing.T) {
	t.Parallel()
	t.Run("torn-evidence", func(t *testing.T) {
		t.Parallel()
		bed := newAttentionPolicyBed(t)
		fixture := newTickContinuationFixture(t, bed, false)
		path := EvidencePath(bed.root)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{torn"), 0600); err != nil {
			t.Fatal(err)
		}
		_, loadErr := LoadEvidence(path)
		if loadErr == nil {
			t.Fatal("fixture evidence must be torn")
		}
		result, complete, err := fixture.run(t)
		if err != nil || complete || result.Decision.Verdict != VerdictDegraded {
			t.Fatalf("torn evidence result=%+v complete=%t error=%v", result, complete, err)
		}
		pending, err := PendingNotifications(bed.root)
		if err != nil || len(pending) != 1 {
			t.Fatalf("degraded pending=%+v error=%v", pending, err)
		}
		if pending[0].Nonce != "verdict-degraded" || pending[0].Cause != degradedEvidenceStore {
			t.Errorf("torn evidence queued cause=%q nonce=%q, want evidence-store and verdict-degraded", pending[0].Cause, pending[0].Nonce)
		}
		if want := fmt.Sprintf("steward: %s — %s", VerdictDegraded, loadErr); pending[0].Message != want {
			t.Errorf("degraded message=%q, want unchanged %q", pending[0].Message, want)
		}
	})
	t.Run("declared-distinct-callers", func(t *testing.T) {
		t.Parallel()
		fset, file, declared := declaredCauses(t, "tick.go", "degradedCause")
		used := map[string]bool{}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := call.Fun.(*ast.Ident)
			if !ok || fn.Name != "degradedTick" {
				return true
			}
			if len(call.Args) != 3 {
				t.Errorf("%s: degradedTick needs a cause argument", fset.Position(call.Pos()))
				return true
			}
			cause, ok := call.Args[1].(*ast.Ident)
			if !ok {
				t.Errorf("%s: degradedTick cause must be a declared constant", fset.Position(call.Pos()))
				return true
			}
			if _, ok := declared[cause.Name]; !ok {
				t.Errorf("%s: undeclared degraded cause %s", fset.Position(call.Pos()), cause.Name)
			}
			if used[cause.Name] {
				t.Errorf("%s: repeated degraded cause %s", fset.Position(call.Pos()), cause.Name)
			}
			used[cause.Name] = true
			return true
		})
		for name := range declared {
			if !used[name] {
				t.Errorf("degraded cause %s has no caller", name)
			}
		}
		values := map[string]bool{}
		for name, value := range declared {
			if values[value] {
				t.Errorf("%s repeats degraded value %q", name, value)
			}
			values[value] = true
		}
	})
}

func TestBreachStopNoticeUsesTickClock(t *testing.T) {
	t.Parallel()
	fixture := newChannelFixture(t)
	cfg := TickConfig{Now: funnelStart, BreachStop: func(string, uint64) (string, error) { return "stopped", nil }}
	reports := custodialBreachStops(fixture.root, cfg, func(string, time.Time) ([]dispatch.StopRoute, error) {
		return []dispatch.StopRoute{{GoalID: "budget-goal", Revision: 4, Budget: &breachBox}}, nil
	})
	asked, _ := channel.WalkOpenQuestions(fixture.root)
	if len(reports) != 1 || reports[0].State != "COMPLETE" || len(asked) != 1 {
		t.Fatalf("stop must complete and ask: %+v %+v", reports, asked)
	}
	if !asked[0].OpenedAt.Equal(funnelStart) || !strings.Contains(asked[0].Plain.Decision, "stopped at "+funnelStart.Local().Format("15:04")) {
		t.Errorf("the box question must use the tick clock: %s %q", asked[0].OpenedAt, asked[0].Plain.Decision)
	}
}

func TestProviderStaleNoticeUsesTickClock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := testprovider.Register(t, root)
	observed := funnelStart.Add(-outage.Horizon)
	if _, err := outage.Observe(home, "claude", "fixture-model", outage.ProviderLimit, "HTTP 429 Too Many Requests", "limited-call", observed); err != nil {
		t.Fatal(err)
	}
	// Stop after provider expiry: the remainder of this tick has a deliberately unreadable store.
	if err := os.MkdirAll(EvidencePath(root), 0755); err != nil {
		t.Fatal(err)
	}
	_, _ = RunTick(root, TickConfig{Now: funnelStart, ProviderHome: home, WorkStateRoot: t.TempDir(), BreachStopReady: func() bool { return false }}, fakeCensus{})
	var found int
	for _, record := range journalOf(t, root) {
		if record.Ref == "provider-stale-anthropic-"+observed.Format(time.RFC3339Nano) {
			found++
			if record.Delivered || record.Withheld != "provider" || record.At != funnelStart.Format(time.RFC3339) {
				t.Errorf("provider-stale journal must use tick clock and be kept from a person: %+v", record)
			}
		}
	}
	if found != 1 {
		t.Fatalf("provider-stale journal records=%d, want 1", found)
	}
}
