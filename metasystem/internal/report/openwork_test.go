package report

import (
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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

// Staleness edge cases: chain-root claims, per-stream verdicts, and the
// plans README, which is documentation rather than a work stream.

func TestNoStaleWhenClaimNamesTheChainRootOfALiveRound(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "a.md",
		"- Next step: none\n- In flight right now: job design-critic-20260101t000000z-aaaa\n")
	writeJob(t, root, "live.json",
		`{"jobId":"design-critic-20260101t000000z-aaaa-r3","status":"running"}`)
	if lines := OpenWork(root); hasLine(lines, "STALE-PLAN") {
		t.Fatalf("a claim naming the chain root of a live round is accurate: %v", lines)
	}
}

func TestStalenessIsPerStream(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "busy.md",
		"- Next step: none\n- In flight right now: job design-critic-20260101t000000z-aaaa\n")
	writePlan(t, root, "other.md",
		"- In flight right now: nothing\n- Waiting on the human: nothing blocking\n- Next step: none\n")
	writeJob(t, root, "live.json",
		`{"jobId":"design-critic-20260101t000000z-aaaa","status":"running"}`)
	if lines := OpenWork(root); hasLine(lines, "STALE-PLAN plans/other.md") {
		t.Fatalf("an idle stream was called stale because another stream had a job: %v", lines)
	}
}

func TestPlansReadmeIsNotAStream(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "README.md",
		"Standing conventions for plans in this directory.\n")
	if lines := OpenWork(root); len(lines) != 0 {
		t.Fatalf("the plans README was mistaken for a stream: %v", lines)
	}
}

// The reporter-to-gate-marker integration the package tests never drove:
// a LIVE gate marker counts as work in flight (open work silenced); a
// marker whose process is dead is ignored AND pruned by the reporting
// pass itself.
func TestOpenWorkGateMarkerIntegration(t *testing.T) {
	root := t.TempDir() // no METASYSTEM_GATES_RUNNING override here
	for _, dir := range []string{"plans", "artifacts/agents/jobs"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writePlan(t, root, "a.md", "- Next step: Finish the port\n- In flight right now: none\n")
	markers := filepath.Join(root, "artifacts", "agents", "supervision", "gate-runs")
	if err := os.MkdirAll(markers, 0o755); err != nil {
		t.Fatal(err)
	}
	self := os.Getpid()
	live := `{"gate":"fixture-gate.sh","pid":` + itoa(self) + `,"pidStartedAt":` + itoa(int(mustSelfStart(t))) + `}`
	if err := os.WriteFile(filepath.Join(markers, itoa(self)+".json"), []byte(live), 0o644); err != nil {
		t.Fatal(err)
	}
	if lines := OpenWork(root); hasLine(lines, "OPEN-WORK") {
		t.Fatalf("a live gate run was not counted as work in flight: %v", lines)
	}
	if err := os.Remove(filepath.Join(markers, itoa(self)+".json")); err != nil {
		t.Fatal(err)
	}
	dead := `{"gate":"fixture-gate.sh","pid":999999,"pidStartedAt":1}`
	if err := os.WriteFile(filepath.Join(markers, "999999.json"), []byte(dead), 0o644); err != nil {
		t.Fatal(err)
	}
	if lines := OpenWork(root); !hasLine(lines, "OPEN-WORK") {
		t.Fatalf("a gate marker whose process is dead still hid open work: %v", lines)
	}
	left, _ := os.ReadDir(markers)
	if len(left) != 0 {
		t.Fatalf("a dead gate marker was not pruned: %v", left)
	}
}

func itoa(v int) string { return strconv.Itoa(v) }

func mustSelfStart(t *testing.T) int64 {
	t.Helper()
	exact, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("cannot read own start: %v %v", state, err)
	}
	return exact.StartedAt.Unix()
}
