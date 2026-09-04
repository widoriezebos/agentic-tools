package validate

import (
	"encoding/json"
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

func TestCritiqueClosedRefusesEvidenceFreeRefutation(t *testing.T) {
	// A refutation without evidence is not a refutation: the chain
	// closes on zero UNREFUTED material findings, and an empty or
	// placeholder evidence cell leaves the finding standing.
	table := `# Dispositions

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| f-1 | refuted | none | none |
`
	findings, dispositions := critiqueFixture(t,
		`{"findings":[{"id":"f-1","material":true}]}`, table)
	violations := CritiqueClosed(findings, dispositions)
	if len(violations) == 0 {
		t.Fatal("an evidence-free refutation closed the chain")
	}
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "without evidence") {
		t.Fatalf("the refusal names the rule: %v", violations)
	}
	// The SAME finding with real evidence closes.
	evidenced := `# Dispositions

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| f-1 | refuted | ran the probe at HEAD; the claimed path does not execute (exit 1, log attached) | none |
`
	findings2, dispositions2 := critiqueFixture(t,
		`{"findings":[{"id":"f-1","material":true}]}`, evidenced)
	if violations := CritiqueClosed(findings2, dispositions2); len(violations) != 0 {
		t.Fatalf("an evidenced refutation closes: %v", violations)
	}
}

func TestCritiqueClosedOutOfScopeCitesTheBrief(t *testing.T) {
	// A true finding outside the declared threat model closes as
	// out-of-scope — but only by CITING the scope; a bare dismissal
	// leaves it standing.
	bare := `# Dispositions

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| f-1 | out-of-scope | not relevant | none |
`
	findings, dispositions := critiqueFixture(t,
		`{"findings":[{"id":"f-1","material":true}]}`, bare)
	violations := CritiqueClosed(findings, dispositions)
	if len(violations) == 0 || !strings.Contains(strings.Join(violations, "\n"), "without citing") {
		t.Fatalf("a bare out-of-scope is refused by name: %v", violations)
	}
	cited := `# Dispositions

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| f-1 | out-of-scope | true, but the brief's threat model declares no attackers; hostile-env hardening is out of scope | none |
`
	findings2, dispositions2 := critiqueFixture(t,
		`{"findings":[{"id":"f-1","material":true}]}`, cited)
	if violations := CritiqueClosed(findings2, dispositions2); len(violations) != 0 {
		t.Fatalf("a scope-citing out-of-scope closes a material finding: %v", violations)
	}
}

func TestCritiqueClosedWithRegisterPersistsOutOfScope(t *testing.T) {
	findings, dispositions := critiqueFixture(t,
		`{"findings":[{"id":"f-1","material":true}]}`,
		`# Dispositions

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| f-1 | out-of-scope | the brief's scope excludes this platform | none |
`)
	repo := t.TempDir()
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
	digest := strings.Repeat("a", 64)
	record := map[string]any{
		"jobId": "critic", "role": "code-critic",
		"findingRegister": []any{map[string]any{
			"findingId": "f-1", "critic": "critic", "rigorClass": "bounded",
			"factsDigest": digest, "facts": map[string]any{"local": true},
			"artifact": "metasystem/test.go", "title": "outside supported platform",
			"status": "open", "resolution": "", "decisionOpid": "",
			"evidence": "platform evidence", "evidenceDigest": digest, "multiplicity": 1,
		}},
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, rootPath, string(data))

	if violations := CritiqueClosedWithRegister(findings, dispositions, repo, "critic"); len(violations) != 0 {
		t.Fatalf("register-backed close = %v", violations)
	}
	var persisted map[string]any
	if err := json.Unmarshal([]byte(readFile(t, rootPath)), &persisted); err != nil {
		t.Fatal(err)
	}
	entry := persisted["findingRegister"].([]any)[0].(map[string]any)
	if entry["status"] != "resolved" || entry["resolution"] != "out-of-scope" {
		t.Fatalf("out-of-scope disposition was not persisted: %v", entry)
	}

	invalidFindings, invalidDispositions := critiqueFixture(t,
		`{"findings":[{"id":"f-2","material":true}]}`,
		`# Dispositions

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| f-2 | noted | unresolved | none |
`)
	violations := CritiqueClosedWithRegister(invalidFindings, invalidDispositions, repo, "critic")
	if len(violations) == 0 || !strings.Contains(strings.Join(violations, "\n"), "cannot use disposition 'noted'") {
		t.Fatalf("ordinary join violation was not preserved: %v", violations)
	}
}
