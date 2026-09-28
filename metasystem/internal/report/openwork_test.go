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
	lines := openWorkWithoutGoal(t, root)
	if !hasLine(lines, "OPEN-WORK plans/a.md: Finish the port") {
		t.Fatalf("expected an OPEN-WORK line, got %v", lines)
	}
}

func TestOpenWorkIgnoresNextStepInsideFence(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "example.md", "# Example\n\n```text\n- Next step: Do not report this example\n```\n")
	if lines := openWorkWithoutGoal(t, root); hasLine(lines, "example.md") {
		t.Fatalf("a fenced example became open work: %v", lines)
	}
}

func TestOpenWorkUsesRealNextStepBesideFencedExample(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "real.md", "```text\n- Next step: Ignore this example\n```\n\n- Next step: Ship the real change\n- In flight right now: none\n")
	lines := openWorkWithoutGoal(t, root)
	if !hasLine(lines, "OPEN-WORK plans/real.md: Ship the real change") || hasLine(lines, "Ignore this example") {
		t.Fatalf("the real field was not selected outside the fence: %v", lines)
	}
}

// TestOpenWorkUnclosedFenceRunsToEndAndIsNamed: under CommonMark an unclosed
// fence runs to the end of the file, so a field after it is fenced text; the
// plan does not vanish silently, a diagnostics line names it and the line of
// the unclosed fence (OSR-08).
func TestOpenWorkUnclosedFenceRunsToEndAndIsNamed(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "unclosed.md", "```text\nexample without a closing fence\n- Next step: FAKE field inside the unclosed fence\n- In flight right now: none\n")
	lines := openWorkWithoutGoal(t, root)
	if hasLine(lines, "FAKE field inside the unclosed fence") || !hasLine(lines, "PLAN-FENCE-UNCLOSED plans/unclosed.md: the code fence opened at line 1") {
		t.Fatalf("an unclosed fence was read as closed or went unnamed: %v", lines)
	}
}

// TestOpenWorkStrayFirstAndStrayLastFences are the OSR-08 critic's plans: a
// stray opener before a closed example leaves nothing to report (the pairing
// no longer slides) and names the fence that stays open; a stray fence after
// the fields keeps them and is named.
func TestOpenWorkStrayFirstAndStrayLastFences(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "first.md", "```\n- Next step: FAKE after the stray opener\n```text\n- Next step: FAKE inside\n```\n\n```text\n- Next step: FAKE in the example\n")
	writePlan(t, root, "trap.md", "```text\n- Next step: FAKE example from a closed fence\n```\n\n```text\n- Next step: FAKE after an unclosed fence\n- In flight right now: none\n")
	writePlan(t, root, "last.md", "- Next step: REAL work before a stray fence\n- In flight right now: none\n```\n")
	lines := openWorkWithoutGoal(t, root)
	if hasLine(lines, "FAKE") {
		t.Fatalf("a fenced field became open work: %v", lines)
	}
	for _, want := range []string{
		"PLAN-FENCE-UNCLOSED plans/first.md: the code fence opened at line 7",
		"PLAN-FENCE-UNCLOSED plans/trap.md: the code fence opened at line 5",
		"PLAN-FENCE-UNCLOSED plans/last.md: the code fence opened at line 3",
		"OPEN-WORK plans/last.md: REAL work before a stray fence",
	} {
		if !hasLine(lines, want) {
			t.Fatalf("missing %q in %v", want, lines)
		}
	}
}

// TestOpenWorkFencesFollowCommonMark: a fence closes only on a bare fence of
// the same character at least as long, with up to three spaces of indent, so
// a fence inside a longer fence, an info-string line and a tilde block's
// backticks are all fenced text; a backtick line whose info string holds a
// backtick opens nothing.
func TestOpenWorkFencesFollowCommonMark(t *testing.T) {
	for name, body := range map[string]string{
		"fence inside a fence":       "````markdown\n```text\n- Next step: FAKE nested\n```\n- Next step: FAKE outer\n````\n\n- Next step: REAL after the outer fence\n",
		"indented fences":            "   ```\n- Next step: FAKE indented\n   ```\n- Next step: REAL after the outer fence\n",
		"info string does not close": "```\n- Next step: FAKE\n```go\n- Next step: FAKE still\n```\n- Next step: REAL after the outer fence\n",
		"tilde fence":                "~~~\n```\n- Next step: FAKE\n~~~\n- Next step: REAL after the outer fence\n",
		"backtick in the info":       "``` a`b\n- Next step: REAL after the outer fence\n",
		"four spaces is no fence":    "    ```\n- Next step: REAL after the outer fence\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := newPlanRoot(t)
			writePlan(t, root, "plan.md", body)
			lines := openWorkWithoutGoal(t, root)
			if !hasLine(lines, "OPEN-WORK plans/plan.md: REAL after the outer fence") || hasLine(lines, "FAKE") || hasLine(lines, "PLAN-FENCE-UNCLOSED") {
				t.Fatalf("fences misread: %v", lines)
			}
		})
	}
}

// TestScanNamesAnUnclosedPlanFenceInItsWarnings: the stop verdict's scan
// carries the same diagnostics line, so the refusal never drops a plan
// silently.
func TestScanNamesAnUnclosedPlanFenceInItsWarnings(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "unclosed.md", "# Plan\n\n```text\n- Next step: FAKE\n")
	scan := scanWithoutGoal(t, root)
	if !hasLine(scan.OpenWorkWarnings, "PLAN-FENCE-UNCLOSED plans/unclosed.md: the code fence opened at line 3") {
		t.Fatalf("scan warnings = %v", scan.OpenWorkWarnings)
	}
	for _, item := range scan.Open {
		if strings.Contains(item.Detail, "FAKE") {
			t.Fatalf("a fenced field became open work: %+v", item)
		}
	}
}

func TestOpenWorkSilentWhenSettledOrWaiting(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "settled.md", "- Next step: none\n- In flight right now: none\n")
	writePlan(t, root, "waiting.md", "- Next step: Ship it\n- Waiting on the human: approval to deploy\n- In flight right now: none\n")
	lines := openWorkWithoutGoal(t, root)
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
	if lines := openWorkWithoutGoal(t, root); hasLine(lines, "OPEN-WORK") {
		t.Fatalf("no open-work should be reported while a job is in flight: %v", lines)
	}
}

func TestOpenWorkSilentWhenOpenChainNewestRoundIsNonTerminal(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "a.md", "- Next step: Finish the port\n- In flight right now: none\n")
	writeJob(t, root, "chain.json", `{"jobId":"chain","status":"pending-setup","chainClosed":false}`)
	if lines := openWorkWithoutGoal(t, root); hasLine(lines, "OPEN-WORK") {
		t.Fatalf("an open chain with a non-terminal newest round is work in flight: %v", lines)
	}
	scan := scanWithoutGoal(t, root)
	if len(scan.Busy) == 0 {
		t.Fatalf("turn-verdict scan did not count the open chain as work in flight: %+v", scan)
	}
	reads := newScanReadFixture(t, root)
	reads.absent = true
	reads.expect("accepted", "accepted", "accepted")
	verdict, err := (&goal.Store{Root: root}).TurnVerdictAtEndpoint(goal.Endpoint{Root: reads.root, Remote: "local", Repository: reads}, reads.machine, scan, "open-chain-session", "", "")
	reads.checked(0)
	if err != nil || verdict.ShouldBlock || verdict.BlockSource != nil || !strings.Contains(verdict.Display, "STILL WORKING") || strings.Contains(verdict.Display, "OPEN WORK") {
		t.Fatalf("pending-setup changed the undeclared checkout's trunk open-chain verdict: %+v %v", verdict, err)
	}
}

func TestTemplatePlaceholderHasItsOwnClassification(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "template.md", "- Next step: <one line, required>\n- In flight right now: none\n")
	lines := openWorkWithoutGoal(t, root)
	if !hasLine(lines, "TEMPLATE-UNFILLED plans/template.md: <one line, required>") || hasLine(lines, "OPEN-WORK") {
		t.Fatalf("template placeholder was not classified separately: %v", lines)
	}
	scan := scanWithoutGoal(t, root)
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
	reads := newScanReadFixture(t, root)
	reads.absent = true
	reads.expect("accepted", "accepted", "accepted", "accepted", "accepted", "accepted")
	endpoint := goal.Endpoint{Root: reads.root, Remote: "local", Repository: reads}
	firstMarked, _, err := MarkOpenWorkSeen(root, []goal.Item{first}, at)
	if err != nil {
		t.Fatal(err)
	}
	firstVerdict, err := store.TurnVerdictAtEndpoint(endpoint, reads.machine, goal.ScanResult{Open: firstMarked}, "long-line-session", "", "")
	if err != nil || !firstVerdict.ShouldBlock {
		t.Fatalf("the first long line did not block: %+v %v", firstVerdict, err)
	}
	marked, _, err = MarkOpenWorkSeen(root, []goal.Item{changed}, at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	changedVerdict, err := store.TurnVerdictAtEndpoint(endpoint, reads.machine, goal.ScanResult{Open: marked}, "long-line-session", "", "")
	reads.checked(0)
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
	lines := openWorkWithoutGoal(t, root)
	if !hasLine(lines, "STALE-PLAN plans/a.md: claims work in flight while no job is running") {
		t.Fatalf("expected a stale-plan line, got %v", lines)
	}
}

func TestNoStaleWhenClaimNamesRunningJob(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "a.md", "- Next step: none\n- In flight right now: job-abc\n")
	writeJob(t, root, "job-abc.json", `{"jobId":"job-abc","status":"running"}`)
	if lines := openWorkWithoutGoal(t, root); hasLine(lines, "STALE-PLAN") {
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
	if lines := openWorkWithoutGoal(t, root); hasLine(lines, "STALE-PLAN") {
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
	if lines := openWorkWithoutGoal(t, root); hasLine(lines, "STALE-PLAN plans/other.md") {
		t.Fatalf("an idle stream was called stale because another stream had a job: %v", lines)
	}
}

func TestPlansReadmeIsNotAStream(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "README.md",
		"Standing conventions for plans in this directory.\n")
	if lines := openWorkWithoutGoal(t, root); len(lines) != 0 {
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
	if lines := openWorkWithoutGoal(t, root); hasLine(lines, "OPEN-WORK") {
		t.Fatalf("a live gate run was not counted as work in flight: %v", lines)
	}
	if err := os.Remove(filepath.Join(markers, itoa(self)+".json")); err != nil {
		t.Fatal(err)
	}
	dead := `{"gate":"fixture-gate.sh","pid":999999,"pidStartedAt":1}`
	if err := os.WriteFile(filepath.Join(markers, "999999.json"), []byte(dead), 0o644); err != nil {
		t.Fatal(err)
	}
	if lines := openWorkWithoutGoal(t, root); !hasLine(lines, "OPEN-WORK") {
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

// A plan naming an open chain between rounds is current for a bounded window
// after the newest round ends; an aged-out or closed chain no longer vouches
// for the claim.
func TestStalePlanChainBetweenRoundsIsCurrentOnlyWithinTheGraceWindow(t *testing.T) {
	root := newPlanRoot(t)
	writePlan(t, root, "stream.md", "- In flight right now: chain implementer-20260101t000000z-cccc (round 2 adjudicating)\n- Waiting on the human: nothing blocking\n- Next step: none\n")
	writeJob(t, root, "implementer-20260101t000000z-cccc.json", `{"jobId":"implementer-20260101t000000z-cccc","status":"completed"}`)
	if lines := openWorkWithoutGoal(t, root); hasLine(lines, "STALE-PLAN") {
		t.Fatalf("a plan naming an open chain between rounds was called stale: %v", lines)
	}
	t.Setenv("METASYSTEM_CHAIN_GRACE_SECONDS", "0")
	if lines := openWorkWithoutGoal(t, root); !hasLine(lines, "STALE-PLAN") {
		t.Fatalf("an aged-out chain still vouched for the plan: %v", lines)
	}
	t.Setenv("METASYSTEM_CHAIN_GRACE_SECONDS", "5400")
	writeJob(t, root, "implementer-20260101t000000z-cccc.json", `{"jobId":"implementer-20260101t000000z-cccc","status":"completed","chainClosed":true}`)
	if lines := openWorkWithoutGoal(t, root); !hasLine(lines, "STALE-PLAN") {
		t.Fatalf("a closed chain still vouched for the plan: %v", lines)
	}
}
