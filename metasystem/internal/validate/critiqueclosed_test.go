package validate

import (
	"path/filepath"
	"strings"
	"testing"
)

func critiqueFixture(t *testing.T, findingsJSON, dispositionsMarkdown string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	findings := filepath.Join(dir, "return.json")
	dispositions := filepath.Join(dir, "dispositions.md")
	writeFile(t, findings, findingsJSON)
	writeFile(t, dispositions, dispositionsMarkdown)
	return findings, dispositions
}

const closedTable = `# Dispositions

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| f-1 | accepted | fixed in place | tightened the check |
| f-2 | noted | stylistic only | none |
`

func TestCritiqueClosedAccepts(t *testing.T) {
	findings, dispositions := critiqueFixture(t,
		`{"findings":[{"id":"f-1","material":true},{"id":"f-2","material":false}]}`,
		closedTable)
	if violations := CritiqueClosed(findings, dispositions); len(violations) != 0 {
		t.Fatalf("closed critique flagged: %v", violations)
	}
}

func TestCritiqueClosedRejectsOpenJoin(t *testing.T) {
	findings, dispositions := critiqueFixture(t,
		`{"findings":[{"id":"f-1","material":true},{"id":"f-3","material":false}]}`,
		strings.Replace(closedTable, "| f-1 | accepted |", "| f-1 | noted |", 1))
	violations := CritiqueClosed(findings, dispositions)
	want := []string{
		"material finding id 'f-1' cannot use disposition 'noted'",
		"finding id 'f-3' has no disposition row",
		"disposition names unknown finding id: 'f-2'",
	}
	if !equalStrings(violations, want) {
		t.Fatalf("violations = %v, want %v", violations, want)
	}
}

func TestCritiqueClosedRejectsUnjoinableSides(t *testing.T) {
	findings, dispositions := critiqueFixture(t,
		`{"findings":[{"id":" f-1","material":true}]}`,
		"```\n"+closedTable+"```\n")
	violations := CritiqueClosed(findings, dispositions)
	if len(violations) != 2 {
		t.Fatalf("violations = %v, want an unjoinable finding and a missing header", violations)
	}
	if !strings.Contains(violations[0], "$.findings[0].id must be a non-empty string") {
		t.Fatalf("unexpected findings violation: %s", violations[0])
	}
	if !strings.Contains(violations[1], "required header not found") {
		t.Fatalf("a fenced table must not count as a header: %s", violations[1])
	}
}
