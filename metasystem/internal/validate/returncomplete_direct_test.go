package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Direct tests for the return checkers (Phase 6): previously exercised only
// through the suite's shell fixtures, invisible to package coverage.

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
