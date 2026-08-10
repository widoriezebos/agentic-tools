package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newPlanRoot(t *testing.T) string {
	t.Helper()
	t.Setenv("METASYSTEM_GATES_RUNNING", "0") // no gate-marker interference
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "artifacts/agents/jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writePlan(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "plans", name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeJob(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "artifacts/agents/jobs", name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasLine(lines []string, substr string) bool {
	for _, l := range lines {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

func TestOpenWorkReportsUnblockedNextStep(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "a.md", "- Next step: Finish the port\n- Waiting on the human: none\n- In flight right now: none\n")
	lines := OpenWork(root)
	if !hasLine(lines, "OPEN-WORK plans/a.md: Finish the port") {
		t.Fatalf("expected an OPEN-WORK line, got %v", lines)
	}
}

func TestOpenWorkSilentWhenSettledOrWaiting(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "settled.md", "- Next step: none\n- In flight right now: none\n")
	writePlan(t, root, "waiting.md", "- Next step: Ship it\n- Waiting on the human: approval to deploy\n- In flight right now: none\n")
	lines := OpenWork(root)
	if hasLine(lines, "settled.md") {
		t.Fatalf("a settled plan should not be open work: %v", lines)
	}
	if hasLine(lines, "OPEN-WORK plans/waiting.md") {
		t.Fatalf("a plan waiting on the human should not be open work: %v", lines)
	}
}

func TestOpenWorkSilentWhenJobInFlight(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "a.md", "- Next step: Finish the port\n- In flight right now: none\n")
	writeJob(t, root, "job-1.json", `{"jobId":"job-1","status":"running"}`)
	if lines := OpenWork(root); hasLine(lines, "OPEN-WORK") {
		t.Fatalf("no open-work should be reported while a job is in flight: %v", lines)
	}
}

func TestStalePlanWhenClaimingIdleWork(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "a.md", "- Next step: none\n- In flight right now: job-abc is churning\n")
	lines := OpenWork(root)
	if !hasLine(lines, "STALE-PLAN plans/a.md: claims work in flight while no job is running") {
		t.Fatalf("expected a stale-plan line, got %v", lines)
	}
}

func TestNoStaleWhenClaimNamesRunningJob(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "a.md", "- Next step: none\n- In flight right now: job-abc\n")
	writeJob(t, root, "job-abc.json", `{"jobId":"job-abc","status":"running"}`)
	if lines := OpenWork(root); hasLine(lines, "STALE-PLAN") {
		t.Fatalf("a claim naming a running job is accurate, not stale: %v", lines)
	}
}
