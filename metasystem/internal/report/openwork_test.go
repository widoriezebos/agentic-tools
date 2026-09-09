package report

import (
	"encoding/json"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestOpenWorkIgnoresNextStepInsideFence(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "example.md", "# Example\n\n```text\n- Next step: Do not report this example\n```\n")
	if lines := OpenWork(root); hasLine(lines, "example.md") {
		t.Fatalf("a fenced example became open work: %v", lines)
	}
}

func TestOpenWorkUsesRealNextStepBesideFencedExample(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "real.md", "```text\n- Next step: Ignore this example\n```\n\n- Next step: Ship the real change\n- In flight right now: none\n")
	lines := OpenWork(root)
	if !hasLine(lines, "OPEN-WORK plans/real.md: Ship the real change") || hasLine(lines, "Ignore this example") {
		t.Fatalf("the real field was not selected outside the fence: %v", lines)
	}
}

func TestOpenWorkFallsBackWhenFenceIsUnclosed(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "unclosed.md", "```text\nexample without a closing fence\n- Next step: Ship the real change after the broken example\n- In flight right now: none\n")
	lines := OpenWork(root)
	if !hasLine(lines, "OPEN-WORK plans/unclosed.md: Ship the real change after the broken example") {
		t.Fatalf("an unclosed fence swallowed the real field: %v", lines)
	}
}

func TestOpenWorkKeepsClosedFenceExcludedBeforeUnclosedFence(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "trap.md", "```text\n- Next step: FAKE example from a closed fence\n```\n\n```text\n- Next step: REAL work to do\n- In flight right now: none\n")
	lines := OpenWork(root)
	if !hasLine(lines, "OPEN-WORK plans/trap.md: REAL work to do") || hasLine(lines, "FAKE example from a closed fence") {
		t.Fatalf("the unpaired fence exposed a field from a closed fence: %v", lines)
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

func TestOpenWorkSilentWhenOpenChainNewestRoundIsNonTerminal(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "a.md", "- Next step: Finish the port\n- In flight right now: none\n")
	writeJob(t, root, "chain.json", `{"jobId":"chain","status":"pending-setup","chainClosed":false}`)
	if lines := OpenWork(root); hasLine(lines, "OPEN-WORK") {
		t.Fatalf("an open chain with a non-terminal newest round is work in flight: %v", lines)
	}
	scan := Scan(root)
	if len(scan.Busy) == 0 {
		t.Fatalf("turn-verdict scan did not count the open chain as work in flight: %+v", scan)
	}
	verdict, err := (&goal.Store{Root: root}).TurnVerdict(scan, "open-chain-session", "", "")
	if err != nil || verdict.ShouldBlock || verdict.BlockSource != nil || !strings.Contains(verdict.Display, "STILL WORKING") || strings.Contains(verdict.Display, "OPEN WORK") {
		t.Fatalf("pending-setup changed the undeclared checkout's trunk open-chain verdict: %+v %v", verdict, err)
	}
}

func TestTemplatePlaceholderHasItsOwnClassification(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "template.md", "- Next step: <one line, required>\n- In flight right now: none\n")
	lines := OpenWork(root)
	if !hasLine(lines, "TEMPLATE-UNFILLED plans/template.md: <one line, required>") || hasLine(lines, "OPEN-WORK") {
		t.Fatalf("template placeholder was not classified separately: %v", lines)
	}
	scan := Scan(root)
	if len(scan.Open) != 0 || len(scan.TemplateUnfilled) != 1 || !strings.Contains(scan.TemplateUnfilled[0].Detail, "TEMPLATE-UNFILLED") {
		t.Fatalf("turn scan did not preserve the template classification: %+v", scan)
	}
}

func TestOpenWorkSeenStateIsDurablePerPlanAndLine(t *testing.T) {
	root := newPlanRoot(t)
	at := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	firstLine := goal.Item{Kind: "plan", Id: "plans/a.md", Detail: "OPEN-WORK plans/a.md: Finish the port"}
	first, warning, err := MarkOpenWorkSeen(root, []goal.Item{firstLine}, at)
	if err != nil || warning != "" || len(first) != 1 || first[0].PreviouslyRefused {
		t.Fatalf("first plan line was not recorded as new: %+v %q %v", first, warning, err)
	}
	second, warning, err := MarkOpenWorkSeen(root, []goal.Item{firstLine}, at.Add(time.Minute))
	if err != nil || warning != "" || !second[0].PreviouslyRefused {
		t.Fatalf("the same plan line was not remembered: %+v %q %v", second, warning, err)
	}
	changed := firstLine
	changed.Detail = "OPEN-WORK plans/a.md: Finish the changed port"
	third, warning, err := MarkOpenWorkSeen(root, []goal.Item{changed}, at.Add(2*time.Minute))
	if err != nil || third[0].PreviouslyRefused {
		t.Fatalf("changed line text did not receive a fresh refusal: %+v %q %v", third, warning, err)
	}
	data, err := os.ReadFile(openWorkSeenPath(root))
	if err != nil {
		t.Fatal(err)
	}
	var record openWorkSeenRecord
	if err := json.Unmarshal(data, &record); err != nil || record.SchemaVersion != 1 || len(record.Plans["plans/a.md"]) != 1 {
		t.Fatalf("unexpected durable seen record: %+v %v", record, err)
	}
}

func TestOpenWorkSeenStateReadFailuresResetVisiblyAndFailOpen(t *testing.T) {
	cases := map[string]string{
		"malformed JSON": `{not json`,
		"foreign schema": `{"schemaVersion":2,"plans":{}}`,
		"null plans":     `{"schemaVersion":1,"plans":null}`,
		"digest mismatch": `{"schemaVersion":1,"plans":{"plans/a.md":{"wrong":` +
			`{"line":"OPEN-WORK plans/a.md: old","firstAt":"2026-09-07T08:00:00Z"}}}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			root := newPlanRoot(t)
			path := openWorkSeenPath(root)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			item := goal.Item{Kind: "plan", Id: "plans/a.md", Detail: "OPEN-WORK plans/a.md: current"}
			marked, warning, err := MarkOpenWorkSeen(root, []goal.Item{item}, time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC))
			if err != nil || warning == "" || !strings.Contains(warning, path) || marked[0].PreviouslyRefused {
				t.Fatalf("bad state did not reset visibly and fail open: marked=%+v warning=%q err=%v", marked, warning, err)
			}
			again, secondWarning, err := MarkOpenWorkSeen(root, []goal.Item{item}, time.Date(2026, 9, 7, 8, 1, 0, 0, time.UTC))
			if err != nil || secondWarning != "" || !again[0].PreviouslyRefused {
				t.Fatalf("fresh rewrite was not readable: marked=%+v warning=%q err=%v", again, secondWarning, err)
			}
		})
	}
}

func TestOpenWorkSeenStateUsesFullLineAndPrunesCurrentScan(t *testing.T) {
	root := newPlanRoot(t)
	at := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	prefix := "OPEN-WORK plans/a.md: " + strings.Repeat("a", 240)
	first := goal.Item{Kind: "plan", Id: "plans/a.md", Detail: prefix[:200], FullDetail: prefix + "first-tail"}
	removed := goal.Item{Kind: "plan", Id: "plans/removed.md", Detail: "OPEN-WORK plans/removed.md: old"}
	if _, _, err := MarkOpenWorkSeen(root, []goal.Item{first, removed}, at); err != nil {
		t.Fatal(err)
	}
	changed := first
	changed.FullDetail = prefix + "changed-tail"
	marked, warning, err := MarkOpenWorkSeen(root, []goal.Item{changed}, at.Add(time.Minute))
	if err != nil || warning != "" || marked[0].PreviouslyRefused {
		t.Fatalf("a full-line tail edit was not new: %+v %q %v", marked, warning, err)
	}
	store := &goal.Store{Root: root, Now: func() time.Time { return at.Add(time.Minute) }}
	firstMarked, _, err := MarkOpenWorkSeen(root, []goal.Item{first}, at)
	if err != nil {
		t.Fatal(err)
	}
	firstVerdict, err := store.TurnVerdict(goal.ScanResult{Open: firstMarked}, "long-line-session", "", "")
	if err != nil || !firstVerdict.ShouldBlock {
		t.Fatalf("the first long line did not block: %+v %v", firstVerdict, err)
	}
	marked, _, err = MarkOpenWorkSeen(root, []goal.Item{changed}, at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	changedVerdict, err := store.TurnVerdict(goal.ScanResult{Open: marked}, "long-line-session", "", "")
	if err != nil || !changedVerdict.ShouldBlock {
		t.Fatalf("a tail edit past the display clip did not block the same session: %+v %v", changedVerdict, err)
	}
	data, err := os.ReadFile(openWorkSeenPath(root))
	if err != nil {
		t.Fatal(err)
	}
	var record openWorkSeenRecord
	if err := json.Unmarshal(data, &record); err != nil || len(record.Plans) != 1 || len(record.Plans["plans/a.md"]) != 1 {
		t.Fatalf("stale plan or stale digest survived pruning: %+v %v", record, err)
	}
	if _, present := record.Plans["plans/removed.md"]; present {
		t.Fatalf("removed plan survived pruning: %+v", record.Plans)
	}
	if _, _, err := MarkOpenWorkSeen(root, nil, at.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(openWorkSeenPath(root))
	record = openWorkSeenRecord{}
	if err := json.Unmarshal(data, &record); err != nil || len(record.Plans) != 0 {
		t.Fatalf("an empty current scan retained removed plans: %+v %v", record, err)
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
