package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Port of dispatch-fixtures.sh brain-delegate-refuses (lines 414-425) and
// the `internal delegate --cancel` leg of brain-cancel-close-reap-refuse
// (line 431): the operator boundary fences a brain checkout before it mints
// a claim capability or composes the lifecycle. An unreadable declaration
// refuses --role, --follow-up and --cancel with one typed BRAIN_REFUSED
// line, exit 2, naming the human repair; nothing is written under
// artifacts/agents beyond the declaration itself.
func TestU6bPortDelegateBoundaryFencesACorruptBrainDeclaration(t *testing.T) {
	t.Parallel()
	root, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	agents := filepath.Join(root, "artifacts", "agents")
	if err := os.MkdirAll(filepath.Join(agents, "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	pending := filepath.Join(agents, "jobs", "pending-job.json")
	if err := os.WriteFile(pending, []byte(`{"jobId":"pending-job","status":"pending"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agents, "brain.json"), []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "missing")
	for _, args := range [][]string{
		{"--role", "implementer", "--brief", missing, "--goal", "none-explicit", "--destructive-reach", "MECHANICAL"},
		{"--follow-up", "missing-job", "--brief", missing},
		{"--cancel", "pending-job"},
	} {
		var stdout, stderr bytes.Buffer
		code := runDelegateAt(root, args, &stdout, &stderr)
		var outcome delegateOutcome
		if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &outcome); err != nil {
			t.Fatalf("%v: stdout %q is not one typed line: %v", args, stdout.String(), err)
		}
		if code != 2 || outcome.Outcome != "BRAIN_REFUSED" || outcome.Headline != "refused" || !strings.Contains(outcome.Detail, "so nothing here dispatches, lands or claims") {
			t.Fatalf("%v: code %d outcome %+v stderr %q", args, code, outcome, stderr.String())
		}
	}
	entries, err := os.ReadDir(agents)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "jobs" && entry.Name() != "brain.json" {
			t.Fatalf("a fenced delegate call wrote %s", entry.Name())
		}
	}
	if jobs, _ := os.ReadDir(filepath.Join(agents, "jobs")); len(jobs) != 1 {
		t.Fatalf("a fenced delegate call wrote a job record: %v", jobs)
	}
	if content, _ := os.ReadFile(pending); string(content) != `{"jobId":"pending-job","status":"pending"}`+"\n" {
		t.Fatalf("the fenced cancel mutated its record: %q", content)
	}
}
