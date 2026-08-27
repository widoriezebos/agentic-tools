package metrics

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type fixtureRepo struct {
	t               *testing.T
	repo            string
	root            string
	evidence        string
	originalReceipt string
}

func newFixtureRepo(t *testing.T) *fixtureRepo {
	t.Helper()
	base := t.TempDir()
	f := &fixtureRepo{t: t, repo: filepath.Join(base, "repository"), evidence: filepath.Join(base, "evidence")}
	f.root = filepath.Join(f.repo, "metasystem")
	for _, path := range []string{
		filepath.Join(f.root, "plans"), filepath.Join(f.root, "artifacts", "agents"),
		filepath.Join(f.evidence, "suite-failures"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f.run("init", "-q", "-b", "main")
	f.run("config", "user.name", "Fixture")
	f.run("config", "user.email", "fixture@example.invalid")
	f.run("config", "metasystem.goal.machine", "machine-a")
	f.write("metasystem/metasystem.conf", "evidence.root="+f.evidence+"\n")
	f.write("metasystem/plans/receipts.log", "")
	f.commit("2026-08-01T00:00:00Z", "fixture baseline", false)
	return f
}

func (f *fixtureRepo) run(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.repo
	cmd.Env = withoutGitSteering(os.Environ())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		f.t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

func (f *fixtureRepo) write(relative, content string) {
	f.t.Helper()
	path := filepath.Join(f.repo, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixtureRepo) writeJSON(path string, value any) {
	f.t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(path, string(data)+"\n")
}

func (f *fixtureRepo) commit(at, message string, goalTransaction bool) string {
	f.t.Helper()
	f.run("add", "-A")
	cmd := exec.Command("git", "commit", "-q", "--no-gpg-sign", "-m", message)
	cmd.Dir = f.repo
	cmd.Env = append(withoutGitSteering(os.Environ()), "GIT_AUTHOR_DATE="+at, "GIT_COMMITTER_DATE="+at)
	if goalTransaction {
		cmd.Args = append(cmd.Args, "--author", "Goals <goals@metasystem.invalid>")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		f.t.Fatalf("commit %s: %v: %s", message, err, stderr.String())
	}
	return f.run("rev-parse", "HEAD")
}

func history(at, opid, verb string) goal.HistoryLine {
	return goal.HistoryLine{At: at, Opid: opid, Verb: verb, Actor: "machine-a+fixture", Targets: []string{"g1"}, Keep: -1}
}

func (f *fixtureRepo) seedFullWorld() {
	f.t.Helper()
	f.originalReceipt = "1770000000|2026-08-19T12:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=1|stop_loss=no|delegate=fake:model:j1|goal=g1|built_by=delegate|critique_waived=none|waiver_stream=none|note=landed"
	oldReceipt := "1770000001|2026-08-19T13:00:00Z|RECEIPT|type=review|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|critique_waived=none|waiver_stream=none|note=old row"
	f.write("metasystem/plans/receipts.log", f.originalReceipt+"\n"+oldReceipt+"\n")
	f.write("metasystem/payload.txt", "one\ntwo\nthree\nfour\n")
	f.commit("2026-08-19T12:00:00Z", "land g1", false)

	digest := fmt.Sprintf("%x", sha1.Sum([]byte(f.originalReceipt)))
	correction1 := "1771000000|2026-08-25T00:00:00Z|CORRECTION|ref_epoch=1770000000|ref_sha1=" + digest + "|field=corrections|was=1|now=2|reason=first"
	correction2 := "1771000001|2026-08-25T00:01:00Z|CORRECTION|ref_epoch=1770000000|ref_sha1=" + digest + "|field=corrections|was=1|now=3|reason=last"
	f.write("metasystem/plans/receipts.log", f.originalReceipt+"\n"+oldReceipt+"\n"+correction1+"\n"+correction2+"\n")
	f.commit("2026-08-25T00:02:00Z", "project receipt corrections", false)

	goalsDir := filepath.Join(f.root, "plans", "goals")
	if err := os.MkdirAll(filepath.Join(goalsDir, "done"), 0o755); err != nil {
		f.t.Fatal(err)
	}
	g1 := &goal.GoalFile{
		Id: "g1", State: goal.StateDone, Intent: "Ship fixture work.", Origin: goal.OriginMain,
		NextStep: "Appetite: 8h ship it.", Conclude: "Shipped.", OpenedAt: "2026-08-01T00:00:00Z", Revision: 3,
		History: []goal.HistoryLine{
			history("2026-08-01T00:00:00Z", "01J5X00000000000000000P000-machine-a-11111111", "open"),
			history("2026-08-05T00:00:00Z", "01J5X00000000000000000P010-machine-a-11111111", "claim"),
			history("2026-08-20T12:00:00Z", "01J5X00000000000000000P020-machine-a-11111111", "done"),
		},
	}
	if err := os.WriteFile(filepath.Join(goalsDir, "done", "g1.md"), goal.RenderFile(g1), 0o644); err != nil {
		f.t.Fatal(err)
	}
	parked := &goal.GoalFile{
		Id: "parked-old", State: goal.StateParked, Intent: "Parked debt.", Origin: goal.OriginMain,
		NextStep: "Appetite: 1h revisit.", OpenedAt: "2026-06-01T00:00:00Z", Revision: 2,
		Parked: &goal.ParkRecord{By: "machine-a+fixture", At: "2026-07-01T00:00:00Z", Because: "waiting"},
		History: []goal.HistoryLine{
			{At: "2026-06-01T00:00:00Z", Opid: "01J5X00000000000000000P030-machine-a-11111111", Verb: "open", Actor: "machine-a+fixture", Targets: []string{"parked-old"}, Keep: -1},
			{At: "2026-07-01T00:00:00Z", Opid: "01J5X00000000000000000P040-machine-a-11111111", Verb: "park", Actor: "machine-a+fixture", Targets: []string{"parked-old"}, Keep: -1},
		},
	}
	queued := &goal.GoalFile{
		Id: "queued-unsized", State: goal.StateQueued, Intent: "Unsized debt.", Origin: goal.OriginMain,
		NextStep: "Investigate later.", OpenedAt: "2026-07-01T00:00:00Z", Revision: 1,
		History: []goal.HistoryLine{{At: "2026-07-01T00:00:00Z", Opid: "01J5X00000000000000000P050-machine-a-11111111", Verb: "open", Actor: "machine-a+fixture", Targets: []string{"queued-unsized"}, Keep: -1}},
	}
	collision := &goal.GoalFile{
		Id: "collision", State: goal.StateClaimed, Intent: "Collision fixture.", Origin: goal.OriginMain,
		NextStep: "Appetite: 4h continue.", OpenedAt: "2026-08-01T00:00:00Z", Revision: 2,
		Claimed: &goal.ClaimRecord{Machine: "machine-a", Lineage: "fixture", At: "2026-08-21T00:00:00Z"},
		History: []goal.HistoryLine{
			{At: "2026-08-01T00:00:00Z", Opid: "01J5X00000000000000000P060-machine-a-11111111", Verb: "open", Actor: "machine-a+fixture", Targets: []string{"collision"}, Keep: -1},
			{At: "2026-08-21T00:00:00Z", Opid: "01J5X00000000000000000P070-machine-a-11111111", Verb: "steal", Actor: "machine-a+fixture", Targets: []string{"collision"}, Displaced: "machine-b+other@2026-08-20T00:00:00Z", Keep: -1},
		},
	}
	for _, file := range []*goal.GoalFile{parked, queued, collision} {
		if err := os.WriteFile(filepath.Join(goalsDir, file.Id+".md"), goal.RenderFile(file), 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
	accepted := f.commit("2026-08-26T00:00:00Z", "publish goal facts", true)
	f.run("update-ref", goal.AcceptedRef, accepted)

	f.job("j1", map[string]any{
		"jobId": "j1", "role": "implementer", "status": "completed", "goalId": "g1", "round": 1,
		"runtime": "codex", "startedAt": "2026-08-18T10:00:00Z", "endedAt": "2026-08-18T12:00:00Z",
		"usage": map[string]any{"inputTokens": 10, "cost": map[string]any{"amount": 1, "currency": "USD"}, "providerUnits": map[string]any{"name": "credits", "value": 2}},
	})
	f.job("critic", map[string]any{
		"jobId": "critic", "role": "design-critic", "status": "completed", "goalId": "g1", "round": 1,
		"runtime": "claude", "startedAt": "2026-08-18T12:00:00Z", "endedAt": "2026-08-18T13:00:00Z",
		"usage": map[string]any{"outputTokens": 5, "cost": map[string]any{"amount": 2, "currency": "EUR"}},
	})
	f.job("critic-r2", map[string]any{
		"jobId": "critic-r2", "parentJob": "critic", "role": "design-critic", "status": "completed", "goalId": "g1", "round": 2,
		"runtime": "codex", "startedAt": "2026-08-18T13:00:00Z", "endedAt": "2026-08-18T14:00:00Z",
		"usage": map[string]any{"inputTokens": 5, "providerUnits": map[string]any{"name": "credits", "value": 1}},
	})

	chain := filepath.Join(f.root, "artifacts", "agents", "critiques", "fixture-chain")
	if err := os.MkdirAll(chain, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chain, "attribution"), []byte("goal g1\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	for _, round := range []string{"1", "2"} {
		if err := os.WriteFile(filepath.Join(chain, "r"+round+"-output.md"), []byte("ok\n"), 0o644); err != nil {
			f.t.Fatal(err)
		}
	}

	envelope := filepath.Join(f.evidence, "suite-failures", "green-run")
	if err := os.MkdirAll(envelope, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envelope, "outcome.json"), []byte(`{"verdict":"green"}`+"\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envelope, "timings.json"), []byte(`{"startedAt":"2026-08-14T00:00:00Z","endedAt":"2026-08-15T00:00:00Z"}`+"\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}

	f.writeJSON("metasystem/artifacts/agents/goal-transactions/rejected.json", map[string]any{
		"intent": map[string]any{"verb": "claim", "targets": []string{"g1"}},
		"phase":  "terminal", "outcome": "rejected", "attempts": 1,
		"terminalAt": "2026-08-21T00:00:00Z", "evidence": "claim refused",
	})
}

func (f *fixtureRepo) job(id string, value map[string]any) {
	f.t.Helper()
	f.writeJSON("metasystem/artifacts/agents/jobs/"+id+".json", value)
}

func weeklyOptions(f *fixtureRepo) Options {
	return Options{Root: f.root, PeriodEnd: "2026-08-24T00:00:00Z", Now: func() time.Time { return time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC) }}
}

func detailedReport(t *testing.T, result Result) string {
	t.Helper()
	for _, path := range result.Paths {
		if strings.Contains(path, string(filepath.Separator)+"period-") {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
	}
	t.Fatal("detailed period report not found")
	return ""
}

func TestO1EachMetricComputesValueAndCoverageFromCannedTree(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	result, err := Report(weeklyOptions(f))
	if err != nil {
		t.Fatal(err)
	}
	report := detailedReport(t, result)
	for _, key := range []string{
		"overhead_ratio", "stale_checks", "rework_rate", "friction_rate", "time_waiting_on_checks",
		"debt_age", "built_by_delegates", "cross_machine_collisions", "cost_per_result",
	} {
		if !strings.Contains(report, "metric="+key+"\n") {
			t.Fatalf("metric %s missing:\n%s", key, report)
		}
	}
	tests := []struct {
		key      string
		value    string
		coverage []string
	}{
		{
			key:   "overhead_ratio",
			value: "g1 wall_hours=4.000 spend=0.500 density=83.333 critique_rounds=2 corrections=3 landed_lines=6",
			coverage: []string{
				"source=goals found=4 usable=1 rejected=0 missing=0",
				"source=jobs found=3 usable=3 rejected=0 missing=0 goal=g1 attributed=3 total=3",
				"source=landings found=3 usable=1 rejected=0 missing=0 goal=g1 attributed=1 total=3",
				"source=receipts found=4 usable=1 rejected=0 missing=0 goal=g1 attributed=1 total=2",
				"source=critique-chains found=1 usable=1 rejected=0 missing=0 goal=g1 attributed=1 total=1",
			},
		},
		{
			key:      "stale_checks",
			value:    "milestone-battery days_since_green=9.000",
			coverage: []string{"source=proof-evidence found=1 usable=1 rejected=0 missing=0"},
		},
		{
			key:      "rework_rate",
			value:    "corrected_items=1 receipted_items=2 share=0.500 max_corrections=3",
			coverage: []string{"source=receipts found=4 usable=2 rejected=0 missing=0"},
		},
		{
			key:      "friction_rate",
			value:    "verb=claim rejected=1 terminal=1 rate=1.000",
			coverage: []string{"source=goal-journal found=1 usable=1 rejected=0 missing=0"},
		},
		{
			key:   "time_waiting_on_checks",
			value: "g1 building_hours=348.000 proving_hours=24.000 waiting_share=0.065 epochs=1",
			coverage: []string{
				"source=goals found=4 usable=1 rejected=0 missing=0",
				"source=landings found=3 usable=1 rejected=0 missing=0",
			},
		},
		{
			key:      "debt_age",
			value:    "parked-old kind=parked age_days=54.000; queued-unsized kind=queued-unsized opened-at anchor age_days=54.000",
			coverage: []string{"source=goals found=4 usable=3 rejected=0 missing=0"},
		},
		{
			key:      "built_by_delegates",
			value:    "delegate_items=1 builder_recorded_items=1 share=1.000 mixed=0 unrecorded=1",
			coverage: []string{"source=receipts found=4 usable=2 rejected=0 missing=0"},
		},
		{
			key:   "cross_machine_collisions",
			value: "true_cross_machine_events=1 displaced=1 steals=1",
			coverage: []string{
				"source=goal-history found=4 usable=2 rejected=0 missing=0",
				"source=goal-journal found=1 usable=0 rejected=0 missing=0",
				"source=transport-push-failures found=0 usable=0 rejected=0 missing=1",
			},
		},
		{
			key:      "cost_per_result",
			value:    "wall_hours=4.000; results=1; tokens[claude,outputTokens]=5.000 records=1 per_result=5.000; tokens[codex,inputTokens]=15.000 records=2 per_result=15.000; cost[EUR]=2.000 records=1 per_result=2.000; cost[USD]=1.000 records=1 per_result=1.000; provider_units[codex,credits]=3.000 records=2 per_result=3.000",
			coverage: []string{"source=jobs found=3 usable=3 rejected=0 missing=0"},
		},
	}
	for _, test := range tests {
		value, coverage := reportMetricValueAndCoverage(t, report, test.key)
		if value != test.value {
			t.Errorf("metric %s value mismatch:\nwant %q\n got %q", test.key, test.value, value)
		}
		if strings.Join(coverage, "\n") != strings.Join(test.coverage, "\n") {
			t.Errorf("metric %s coverage mismatch:\nwant %q\n got %q", test.key, test.coverage, coverage)
		}
	}
}

func reportMetricValueAndCoverage(t *testing.T, report, key string) (string, []string) {
	t.Helper()
	marker := "\nmetric=" + key + "\n"
	start := strings.Index(report, marker)
	if start < 0 {
		t.Fatalf("metric %s missing:\n%s", key, report)
	}
	block := report[start+len(marker):]
	if end := strings.Index(block, "\nmetric="); end >= 0 {
		block = block[:end]
	}
	value := ""
	var coverage []string
	for _, line := range strings.Split(block, "\n") {
		switch {
		case strings.HasPrefix(line, "value="):
			value = strings.TrimPrefix(line, "value=")
		case strings.HasPrefix(line, "coverage="):
			coverage = append(coverage, strings.TrimPrefix(line, "coverage="))
		}
	}
	return value, coverage
}

func TestO2JobsGapIsLoudAndNeverPrintsZero(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	if err := os.RemoveAll(filepath.Join(f.root, "artifacts", "agents", "jobs")); err != nil {
		t.Fatal(err)
	}
	result, err := Report(weeklyOptions(f))
	if err != nil {
		t.Fatal(err)
	}
	report := detailedReport(t, result)
	if !strings.Contains(report, "coverage=source=jobs found=0 usable=0 rejected=0 missing=1") ||
		!strings.Contains(report, "metric=overhead_ratio\nname=Overhead ratio\nscope=this-machine\nvalue=g1 wall_hours=unavailable spend=unavailable (no timed attributed jobs)") ||
		!strings.Contains(report, "metric=cost_per_result\nname=Cost per result\nscope=this-machine context-only\nvalue=unavailable") {
		t.Fatalf("jobs gap was not loud:\n%s", report)
	}
	if strings.Contains(report, "wall_hours=0.000") || strings.Contains(report, "spend=0.000") {
		t.Fatalf("jobs gap printed a false zero:\n%s", report)
	}
}

func TestO6InjectedPeriodRerunIsByteIdentical(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	first, err := Report(weeklyOptions(f))
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := os.ReadFile(first.Target)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Report(weeklyOptions(f))
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(second.Target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("same records and injected period produced different bytes")
	}
}

func TestO18CorrectionProjectionIsLastWinsAtOriginalPeriod(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	w, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	var original *receiptRecord
	for _, record := range w.Receipts {
		if record.Epoch == "1770000000" {
			original = record
		}
	}
	if original == nil || original.Fields["corrections"] != "3" || original.At.Format(time.RFC3339) != "2026-08-19T12:00:00Z" {
		t.Fatalf("projection did not stay on the original event: %+v", original)
	}
	row := computeRework(w, mustPeriod(t, weeklyOptions(f)), "", loadThresholds(f.root))
	if row.Value != "corrected_items=1 receipted_items=2 share=0.500 max_corrections=3" {
		t.Fatalf("last correction did not win: %s", row.Value)
	}
}

func TestInvalidEffectiveReceiptProvenanceIsRejectedByOriginalRow(t *testing.T) {
	f := newFixtureRepo(t)
	goalReceipt := "1770000100|2026-08-19T12:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|goal=g1|built_by=delegate|critique_waived=none|waiver_stream=none|note=goal"
	builderReceipt := "1770000101|2026-08-19T12:01:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|goal=g1|built_by=delegate|critique_waived=none|waiver_stream=none|note=builder"
	goalCorrection := "1770000200|2026-08-20T12:00:00Z|CORRECTION|ref_epoch=1770000100|ref_sha1=" + fmt.Sprintf("%x", sha1.Sum([]byte(goalReceipt))) + "|field=goal|was=g1|now=Invalid_goal|reason=corrupt"
	builderCorrection := "1770000201|2026-08-20T12:01:00Z|CORRECTION|ref_epoch=1770000101|ref_sha1=" + fmt.Sprintf("%x", sha1.Sum([]byte(builderReceipt))) + "|field=built_by|was=delegate|now=critic|reason=corrupt"
	f.write("metasystem/plans/receipts.log", strings.Join([]string{goalReceipt, builderReceipt, goalCorrection, builderCorrection}, "\n")+"\n")
	invalidLanding := f.commit("2026-08-20T12:02:00Z", "hand-corrupt receipt provenance", false)

	w, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	details := strings.Join(w.ReceiptCoverage.Details, "\n")
	if w.ReceiptCoverage.Rejected != 2 || !strings.Contains(details, "line=1 invalid effective goal") ||
		!strings.Contains(details, "line=2 invalid effective built_by") {
		t.Fatalf("invalid effective provenance was not rejected by original row: %+v", w.ReceiptCoverage)
	}
	if strings.Count(details, "line=1 invalid effective goal") != 1 || strings.Count(details, "line=2 invalid effective built_by") != 1 {
		t.Fatalf("invalid effective provenance was named more than once in coverage: %+v", w.ReceiptCoverage)
	}
	period := Period{Start: time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)}
	invalidLandingFound := false
	for _, landing := range w.Landings {
		if landing.SHA != invalidLanding {
			continue
		}
		invalidLandingFound = true
		if len(landing.Goals) != 0 || landing.Shared {
			t.Fatalf("invalid effective provenance attributed its landing: %+v", landing)
		}
	}
	if !invalidLandingFound {
		t.Fatalf("invalid-provenance landing %s was not loaded", invalidLanding)
	}
	landingCounts := landingAttribution(w, period, "")
	if landingCounts.Total != 1 || landingCounts.Attributed != 0 || landingCounts.Unattributed != 1 || landingCounts.Rejected != 0 {
		t.Fatalf("invalid-provenance period landing was not unattributed: %+v", landingCounts)
	}
	counts := receiptAttribution(w, period, "g1")
	if counts.Total != 2 || counts.Attributed != 0 || counts.Unattributed != 0 || counts.Rejected != 2 {
		t.Fatalf("invalid goal escaped the rejected attribution bucket: %+v", counts)
	}
	rework := computeRework(w, period, "g1", thresholds{})
	if rework.Value != "unavailable" || rework.Coverage[0].Usable != 0 || !detailsContain(rework, "attribution source=receipts bucket=REJECTED records=2") {
		t.Fatalf("rework hid rejected attribution: %+v", rework)
	}
	delegates := computeDelegates(w, period, "g1", thresholds{})
	if delegates.Value != "unavailable" || delegates.Coverage[0].Usable != 0 {
		t.Fatalf("invalid effective builder remained usable: %+v", delegates)
	}
	periodRework := computeRework(w, period, "", thresholds{})
	if periodRework.Value != "unavailable" || periodRework.Coverage[0].Usable != 0 {
		t.Fatalf("period rework used rejected effective provenance: %+v", periodRework)
	}
	periodDelegates := computeDelegates(w, period, "", thresholds{})
	if periodDelegates.Value != "unavailable" || periodDelegates.Coverage[0].Usable != 0 {
		t.Fatalf("period delegates used rejected effective provenance: %+v", periodDelegates)
	}
}

func TestCritiqueAttributionDecisionReader(t *testing.T) {
	f := newFixtureRepo(t)
	root := filepath.Join(f.root, "artifacts", "agents", "critiques")
	makeChain := func(name string) string {
		t.Helper()
		directory := filepath.Join(root, name)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "r1-output.md"), []byte("ok\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return directory
	}
	goalChain := makeChain("goal-chain")
	if err := os.WriteFile(filepath.Join(goalChain, "attribution"), []byte("goal g1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	explicitChain := makeChain("explicit-unattributed")
	if err := os.WriteFile(filepath.Join(explicitChain, "attribution"), []byte("unattributed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	historicalChain := makeChain("historical-unattributed")
	if err := os.WriteFile(filepath.Join(historicalChain, ".attribution.abandoned"), []byte("partial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	malformedChain := makeChain("malformed")
	if err := os.WriteFile(filepath.Join(malformedChain, "attribution"), []byte("goal Invalid_goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unreadableChain := makeChain("unreadable")
	if err := os.Mkdir(filepath.Join(unreadableChain, "attribution"), 0o755); err != nil {
		t.Fatal(err)
	}

	chains, coverage := loadCritiques(f.root)
	if coverage.Found != 5 || coverage.Rejected != 2 || len(chains) != 3 {
		t.Fatalf("critique attribution decisions produced wrong coverage: chains=%+v coverage=%+v", chains, coverage)
	}
	goals := map[string]string{}
	for _, chain := range chains {
		goals[chain.Name] = chain.GoalID
	}
	if goals["goal-chain"] != "g1" || goals["explicit-unattributed"] != "" || goals["historical-unattributed"] != "" {
		t.Fatalf("critique attribution decisions were read incorrectly: %+v", goals)
	}
	details := strings.Join(coverage.Details, "\n")
	if !strings.Contains(details, filepath.Join(malformedChain, "attribution")+" invalid attribution decision") ||
		!strings.Contains(details, filepath.Join(unreadableChain, "attribution")+" unreadable:") {
		t.Fatalf("rejected critique attribution was not named: %+v", coverage)
	}
}

func TestO19CostDimensionsNeverCollapseRuntimeOrCurrency(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	w, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	row := computeCost(w, mustPeriod(t, weeklyOptions(f)), "")
	for _, want := range []string{"tokens[codex,inputTokens]", "tokens[claude,outputTokens]", "cost[USD]", "cost[EUR]"} {
		if !strings.Contains(row.Value, want) {
			t.Fatalf("missing dimension %s: %s", want, row.Value)
		}
	}
	if strings.Contains(row.Value, "total_cost") || strings.Contains(row.Value, "tokens[total") {
		t.Fatalf("collapsed total appeared: %s", row.Value)
	}
}

func TestO20PeriodSweepCreatesMissingNonCLIGoalReport(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	target := filepath.Join(f.root, "artifacts", "agents", "metrics", "goal-g1.md")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("goal report unexpectedly exists before sweep: %v", err)
	}
	if _, err := Report(weeklyOptions(f)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "report_kind=goal\ngoal=g1\n") {
		t.Fatalf("sweep wrote the wrong report: %s", data)
	}
}

func mustPeriod(t *testing.T, opts Options) Period {
	t.Helper()
	period, err := resolvePeriod(opts)
	if err != nil {
		t.Fatal(err)
	}
	return period
}
