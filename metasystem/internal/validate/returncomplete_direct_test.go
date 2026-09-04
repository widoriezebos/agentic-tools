package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Direct tests for the return checkers: the suite's shell fixtures
// exercise them end to end but are invisible to package coverage.

func returnRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// The checkers read role schemas from the checkout; give them the real
	// ones so the shape validation runs exactly as in production.
	source, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "schemas", "design-critic.schema.json"))
	if err != nil {
		t.Skip("role schema not readable from the test working directory")
	}
	dir := filepath.Join(root, "scripts", "agents", "schemas")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "design-critic.schema.json"), source, 0o644)
	return root
}

func v3Facts() string {
	return `{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false}`
}

func v3CriticReturn(findings, rigor string, count int) string {
	return fmt.Sprintf(`{
  "schemaVersion": 3,
  "claimed": {"sessionId": null, "model": null},
  "jobId": "rc-v3", "round": 1, "runtime": "fake", "sessionId": "fake-session",
  "model": {"requested": "fixture-model", "effective": "fixture-model"},
  "evidence": [], "gaps": [], "mode": "design", "reviewedCommit": "abc1234",
  "findings": %s, "verdictMaterialCount": %d, "rigor": %s
}`, findings, count, rigor)
}

func TestReturnCompleteRoleUnknownRole(t *testing.T) {
	violations := ReturnCompleteRole(t.TempDir(), "no-such-role", "")
	if len(violations) == 0 || !strings.Contains(violations[0], "unknown role") {
		t.Fatalf("unknown role not refused: %v", violations)
	}
}

func TestReturnCompleteRoleMissingAndMalformedFiles(t *testing.T) {
	root := returnRoot(t)
	violations := ReturnCompleteRole(root, "design-critic", filepath.Join(root, "absent.json"))
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "return file does not exist") {
		t.Fatalf("missing return not named: %v", violations)
	}
	bad := filepath.Join(root, "bad.json")
	os.WriteFile(bad, []byte("{not json"), 0o644)
	violations = ReturnCompleteRole(root, "design-critic", bad)
	if !strings.Contains(strings.Join(violations, "\n"), "not valid JSON") {
		t.Fatalf("malformed return not named: %v", violations)
	}
}

func TestReturnCompleteJobChainWalk(t *testing.T) {
	root := returnRoot(t)
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	os.MkdirAll(jobs, 0o755)
	write := func(name, content string) {
		os.WriteFile(filepath.Join(jobs, name+".json"), []byte(content), 0o644)
	}
	// A cycle: a's parent is b, b's parent is a.
	write("job-a", `{"jobId":"job-a","parentJob":"job-b"}`)
	write("job-b", `{"jobId":"job-b","parentJob":"job-a"}`)
	violations := ReturnCompleteJob(root, "job-a")
	if !strings.Contains(strings.Join(violations, "\n"), "cycle") {
		t.Fatalf("parent cycle not caught: %v", violations)
	}
	// A mismatched id.
	write("job-c", `{"jobId":"other"}`)
	violations = ReturnCompleteJob(root, "job-c")
	if !strings.Contains(strings.Join(violations, "\n"), "jobId must equal") {
		t.Fatalf("id mismatch not caught: %v", violations)
	}
	// A missing record.
	violations = ReturnCompleteJob(root, "job-none")
	if !strings.Contains(strings.Join(violations, "\n"), "does not exist") {
		t.Fatalf("missing record not caught: %v", violations)
	}
	// A parent that is not a valid id.
	write("job-d", `{"jobId":"job-d","parentJob":"NOT VALID"}`)
	violations = ReturnCompleteJob(root, "job-d")
	if !strings.Contains(strings.Join(violations, "\n"), "not a valid job id") {
		t.Fatalf("invalid parent id not caught: %v", violations)
	}
}

// A lawful return drives the schema-validation core end to end — shape
// walk, value walk, type matching, enums, and the material-count cross
// check — the paths the refusal cases above never reach.
func TestReturnCompleteRoleLawfulReturn(t *testing.T) {
	root := returnRoot(t)
	lawful := `{
  "jobId": "rc-happy",
  "round": 1,
  "runtime": "fake",
  "sessionId": null,
  "model": {"requested": "fixture-model", "effective": "fixture-model"},
  "evidence": [{"command": "go test ./...", "observed": "ok", "level": "ran"}],
  "gaps": [],
  "mode": "design",
  "reviewedCommit": "abc1234",
  "findings": [
    {"id": "F1", "severity": "high", "material": true, "claim": "a claim", "evidence": "seen directly"},
    {"id": "F2", "severity": "low", "material": false, "claim": "minor", "evidence": "read"}
  ],
  "verdictMaterialCount": 1
}`
	path := filepath.Join(root, "return.json")
	os.WriteFile(path, []byte(lawful), 0o644)
	violations := ReturnCompleteRole(root, "design-critic", path)
	if len(violations) != 0 {
		t.Fatalf("a lawful return was refused: %v", violations)
	}
	// One wrong enum and one wrong material count, to prove the deep walk
	// actually judges.
	broken := strings.Replace(lawful, `"level": "ran"`, `"level": "guessed"`, 1)
	os.WriteFile(path, []byte(broken), 0o644)
	violations = ReturnCompleteRole(root, "design-critic", path)
	if len(violations) == 0 {
		t.Fatal("a bad enum passed the value walk")
	}
	miscounted := strings.Replace(lawful, `"verdictMaterialCount": 1`, `"verdictMaterialCount": 3`, 1)
	os.WriteFile(path, []byte(miscounted), 0o644)
	violations = ReturnCompleteRole(root, "design-critic", path)
	if len(violations) == 0 {
		t.Fatal("a wrong material count passed")
	}
}

func TestReturnCompleteRoleVersionThreeRigorJoin(t *testing.T) {
	root := returnRoot(t)
	path := filepath.Join(root, "return.json")
	findings := `[
    {"id":"F1","severity":"high","material":true,"claim":"bounded","evidence":"read"},
    {"id":"F2","severity":"high","material":true,"claim":"severe","evidence":"read"},
    {"id":"F3","severity":"high","material":true,"claim":"unproven","evidence":"read"}
  ]`
	rigor := `[
    {"findingId":"F1","rigorClass":"bounded","facts":` + v3Facts() + `,"reopeningTrigger":"reopen if the finding recurs"},
    {"findingId":"F2","rigorClass":"severe","facts":` + v3Facts() + `,"reopeningTrigger":"reopen until the invariant is proved"},
    {"findingId":"F3","rigorClass":"unproven","facts":` + v3Facts() + `,"reopeningTrigger":"reopen when classification evidence exists"}
  ]`
	os.WriteFile(path, []byte(v3CriticReturn(findings, rigor, 3)), 0o644)
	if violations := ReturnCompleteRole(root, "design-critic", path); len(violations) != 0 {
		t.Fatalf("lawful version-three return was refused: %v", violations)
	}

	os.WriteFile(path, []byte(v3CriticReturn(`[]`, `[]`, 0)), 0o644)
	if violations := ReturnCompleteRole(root, "design-critic", path); len(violations) != 0 {
		t.Fatalf("zero-material return with empty rigor was refused: %v", violations)
	}
}

func TestReturnCompleteRoleVersionFourRequiresCanonicalArtifact(t *testing.T) {
	root := returnRoot(t)
	path := filepath.Join(root, "return.json")
	finding := `[{"id":"F1","severity":"high","material":true,"claim":"claim","evidence":"read"}]`
	row := `[{"findingId":"F1","rigorClass":"bounded","facts":` + v3Facts() + `,"reopeningTrigger":"reopen","artifact":"metasystem/a.go"}]`
	value := strings.Replace(v3CriticReturn(finding, row, 1), `"schemaVersion": 3`, `"schemaVersion": 4`, 1)
	os.WriteFile(path, []byte(value), 0o644)
	if violations := ReturnCompleteRole(root, "design-critic", path); len(violations) != 0 {
		t.Fatalf("lawful version-four return = %v", violations)
	}
	missing := strings.Replace(value, `,"artifact":"metasystem/a.go"`, "", 1)
	os.WriteFile(path, []byte(missing), 0o644)
	if violations := strings.Join(ReturnCompleteRole(root, "design-critic", path), "\n"); !strings.Contains(violations, "artifact is required") {
		t.Fatalf("missing artifact refusal = %s", violations)
	}
}

func TestReturnCompleteRoleVersionThreeRefusesUnjoinableRigor(t *testing.T) {
	root := returnRoot(t)
	path := filepath.Join(root, "return.json")
	finding := `[{"id":"F1","severity":"high","material":true,"claim":"claim","evidence":"read"}]`
	row := `{"findingId":"F1","rigorClass":"bounded","facts":` + v3Facts() + `,"reopeningTrigger":"reopen if it recurs"}`
	tests := map[string]struct {
		findings string
		rigor    string
		want     string
	}{
		"missing row":       {finding, `[]`, "missing a classification row"},
		"extra row":         {`[]`, `[` + row + `]`, "not a material finding"},
		"blank finding id":  {strings.Replace(finding, `"F1"`, `"  "`, 1), `[]`, "without surrounding whitespace"},
		"duplicate finding": {`[` + strings.TrimPrefix(strings.TrimSuffix(finding, `]`), `[`) + `,{"id":"F1","severity":"low","material":false,"claim":"other","evidence":"read"}]`, `[` + row + `]`, "duplicates finding identifier"},
		"duplicate row":     {finding, `[` + row + `,` + row + `]`, "duplicates rigor row"},
		"blank trigger":     {finding, `[` + strings.Replace(row, `"reopen if it recurs"`, `"  "`, 1) + `]`, "must contain non-whitespace text"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			count := 1
			if test.findings == `[]` {
				count = 0
			}
			os.WriteFile(path, []byte(v3CriticReturn(test.findings, test.rigor, count)), 0o644)
			violations := strings.Join(ReturnCompleteRole(root, "design-critic", path), "\n")
			if !strings.Contains(violations, test.want) {
				t.Fatalf("missing %q refusal:\n%s", test.want, violations)
			}
		})
	}
}

func TestReturnCompleteRoleVersionThreeRefusesMalformedFacts(t *testing.T) {
	root := returnRoot(t)
	path := filepath.Join(root, "return.json")
	finding := `[{"id":"F1","severity":"high","material":true,"claim":"claim","evidence":"read"}]`
	facts := strings.Replace(v3Facts(), `,"externalSideEffectBoundaryCrossed":false`, "", 1)
	rigor := `[{"findingId":"F1","rigorClass":"bounded","facts":` + facts + `,"reopeningTrigger":"reopen if it recurs"}]`
	os.WriteFile(path, []byte(v3CriticReturn(finding, rigor, 1)), 0o644)
	violations := strings.Join(ReturnCompleteRole(root, "design-critic", path), "\n")
	if !strings.Contains(violations, "externalSideEffectBoundaryCrossed is required") {
		t.Fatalf("malformed facts were not refused: %s", violations)
	}
}

func TestFindingIDRefusesInteriorControlCharacters(t *testing.T) {
	root := returnRoot(t)
	path := filepath.Join(root, "return.json")
	findings := `[{"id":"F-1\nF-2","severity":"high","material":true,"claim":"x","evidence":"read"}]`
	rigor := `[{"findingId":"F-1\nF-2","rigorClass":"severe","facts":` + v3Facts() + `,"reopeningTrigger":"reopen"}]`
	os.WriteFile(path, []byte(v3CriticReturn(findings, rigor, 1)), 0o644)
	violations := ReturnCompleteRole(root, "design-critic", path)
	found := false
	for _, v := range violations {
		if strings.Contains(v, "control characters") {
			found = true
		}
	}
	if !found {
		t.Fatalf("an interior line feed survived id validation: %v", violations)
	}
}
