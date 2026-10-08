package loopstop

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func TestUnitStopDecisionOrdersEvidenceBeforeContinuation(t *testing.T) {
	t.Parallel()
	f := readsubject.Finding{ID: "one:1", Class: "regression", Where: "internal/a.go", Material: true}
	prior := readsubject.Read{Material: 3, Findings: []readsubject.Finding{f}}
	for _, row := range []struct {
		name                      string
		policy                    string
		material, attempt, budget int
		prior                     []readsubject.Read
		findings                  []readsubject.Finding
		unknown, want, class      string
	}{
		{name: "clean invalid resolution stays unknown", policy: "auto", material: 0, attempt: 2, budget: 3, prior: []readsubject.Read{prior}, findings: []readsubject.Finding{{Class: "regression", Where: "wrong.go", Resolves: "one:1"}}, want: "stop", class: "invalid"},
		{name: "clean closes at exhausted person cap", policy: "person", material: 0, attempt: 3, budget: 3, want: "close"},
		{name: "first material", policy: "auto", material: 3, attempt: 1, budget: 3, want: "continue"},
		{name: "falling new class", policy: "auto", material: 2, attempt: 2, budget: 3, prior: []readsubject.Read{prior}, findings: []readsubject.Finding{{Class: "scope", Where: "b.go", Material: true}}, want: "continue"},
		{name: "same class new file", policy: "auto", material: 2, attempt: 2, budget: 3, prior: []readsubject.Read{prior}, findings: []readsubject.Finding{{Class: "regression", Where: "b.go", Material: true}}, want: "stop", class: "repeated"},
		{name: "same class same file unresolved", policy: "auto", material: 2, attempt: 2, budget: 3, prior: []readsubject.Read{prior}, findings: []readsubject.Finding{f}, want: "stop", class: "repeated"},
		{name: "equal material", policy: "auto", material: 3, attempt: 2, budget: 3, prior: []readsubject.Read{prior}, want: "stop", class: "did not fall"},
		{name: "two corrections", policy: "auto", material: 1, attempt: 3, budget: 3, want: "stop", class: "allowance"},
		{name: "person", policy: "person", material: 1, attempt: 1, budget: 3, want: "stop", class: "person"},
		{name: "unreadable policy", unknown: "unreadable-policy", material: 1, attempt: 1, budget: 3, want: "stop", class: "unknown"},
		{name: "unknown never zero", policy: "auto", unknown: "missing-classes", material: 0, attempt: 1, budget: 3, want: "stop", class: "unknown"},
		{name: "same class twice first read", policy: "auto", material: 2, attempt: 1, budget: 3, findings: []readsubject.Finding{f, f}, want: "continue"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			s := Decide(Input{Stop: Stop{Loop: "unit-round", Subject: "g/u/run", Attempt: row.attempt, Budget: row.budget, Handoff: "split u-next"}, Prior: row.prior, Read: &readsubject.Read{Material: row.material, Findings: row.findings}, Policy: row.policy, Unknown: row.unknown})
			if s.Decision != row.want || !strings.Contains(s.Class, row.class) {
				t.Fatalf("decision %+v", s)
			}
			if row.unknown != "" && s.Handoff != "stopped "+row.unknown {
				t.Fatalf("unknown handoff %+v", s)
			}
		})
	}
}

func TestUnitCleanReadResolvesPublishedInheritedFinding(t *testing.T) {
	t.Parallel()
	original := readsubject.Finding{ID: "source-read:1", Class: "scope", Where: "source.go", Material: true}
	resolved := readsubject.Finding{ID: "destination-read:1", Class: "scope", Where: "source.go", Resolves: original.ID}
	decision := Decide(Input{Stop: Stop{Loop: "unit-round", Attempt: 1, Budget: 3}, Inherited: []readsubject.Finding{original}, Read: &readsubject.Read{Findings: []readsubject.Finding{resolved}}, Policy: "auto"})
	if decision.Decision != "close" {
		t.Fatalf("inherited resolution %+v", decision)
	}
	resolved.Where = "unrelated.go"
	decision = Decide(Input{Stop: Stop{Loop: "unit-round", Attempt: 1, Budget: 3}, Inherited: []readsubject.Finding{original}, Read: &readsubject.Read{Findings: []readsubject.Finding{resolved}}, Policy: "auto"})
	if decision.Decision != "stop" || decision.Handoff != "stopped invalid-resolves" {
		t.Fatalf("unrelated resolution %+v", decision)
	}
}
