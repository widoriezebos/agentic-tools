package metrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestO3GoalTransactionAuthorNeverCountsAsLanding(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	w, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, commit := range w.Landings {
		if commit.SHA == w.Identity.AcceptedTip {
			t.Fatalf("goal transaction commit counted as landing: %s", commit.SHA)
		}
	}
}

func TestO4ThresholdCrossingOnlyChangesReportContent(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	result, err := Report(weeklyOptions(f))
	if err != nil {
		t.Fatalf("crossed threshold changed report outcome: %v", err)
	}
	report := detailedReport(t, result)
	if !strings.Contains(report, "threshold=density range=[0.5,10]; crossed") ||
		!strings.Contains(report, "threshold=days since green max=7; crossed") {
		t.Fatalf("crossings were not report content:\n%s", report)
	}
	if _, err := os.Stat(filepath.Join(f.root, "artifacts", "agents", "metrics.lock")); !os.IsNotExist(err) {
		t.Fatalf("metrics created a gate or lock: %v", err)
	}
}

func TestO5EveryNamedGapLineAppearsWhenInputIsAbsent(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	file := &goal.GoalFile{
		Id: "no-budget", State: goal.StateDone, Intent: "No budget fixture.", Origin: goal.OriginMain,
		NextStep: "Do the work.", Conclude: "Done.", OpenedAt: "2026-08-17T00:00:00Z", Revision: 3,
		History: []goal.HistoryLine{
			{At: "2026-08-17T00:00:00Z", Opid: "01J5X00000000000000000P080-machine-a-11111111", Verb: "open", Actor: "machine-a+fixture", Targets: []string{"no-budget"}, Keep: -1},
			{At: "2026-08-18T00:00:00Z", Opid: "01J5X00000000000000000P090-machine-a-11111111", Verb: "claim", Actor: "machine-a+fixture", Targets: []string{"no-budget"}, Keep: -1},
			{At: "2026-08-22T00:00:00Z", Opid: "01J5X00000000000000000P100-machine-a-11111111", Verb: "done", Actor: "machine-a+fixture", Targets: []string{"no-budget"}, Keep: -1},
		},
	}
	path := filepath.Join(f.root, "plans", "goals", "done", "no-budget.md")
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	accepted := f.commit("2026-08-27T00:00:00Z", "add no budget goal", true)
	f.run("update-ref", goal.AcceptedRef, accepted)
	periodResult, err := Report(weeklyOptions(f))
	if err != nil {
		t.Fatal(err)
	}
	periodReport := detailedReport(t, periodResult)
	goalResult, err := Report(Options{Root: f.root, GoalID: "no-budget", PeriodEnd: "2026-08-24T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	goalBytes, err := os.ReadFile(goalResult.Target)
	if err != nil {
		t.Fatal(err)
	}
	combined := periodReport + string(goalBytes)
	for _, want := range []string{
		"no structured elapsed budget", "no per-leg run-history record exists", "no classification surface exists",
		"no residue register exists", "builder unrecorded", "partial cost coverage", "no transport-failure record exists",
	} {
		if !strings.Contains(combined, want) {
			t.Errorf("named gap %q missing:\n%s", want, combined)
		}
	}
}

func TestO7FleetValuesFollowPinnedInputIdentityAcrossMachines(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	cloneRepo := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", "-q", f.repo, cloneRepo)
	cmd.Env = withoutGitSteering(os.Environ())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v: %s", err, output)
	}
	cloneRoot := filepath.Join(cloneRepo, "metasystem")
	runGitAt(t, cloneRepo, "config", "metasystem.goal.machine", "machine-b")
	runGitAt(t, cloneRepo, "update-ref", goal.AcceptedRef, f.run("rev-parse", goal.AcceptedRef))

	first, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadWorld(cloneRoot)
	if err != nil {
		t.Fatal(err)
	}
	if first.Identity != second.Identity {
		t.Fatalf("same tips produced different identity: %+v %+v", first.Identity, second.Identity)
	}
	period := mustPeriod(t, weeklyOptions(f))
	firstRows := fleetValues(computeRows(first, period, "", loadThresholds(f.root)))
	secondRows := fleetValues(computeRows(second, period, "", loadThresholds(cloneRoot)))
	if !reflect.DeepEqual(firstRows, secondRows) {
		t.Fatalf("fleet values differ at the same tips:\n%v\n%v", firstRows, secondRows)
	}

	lagTip := runGitAt(t, cloneRepo, "rev-parse", "refs/heads/main~2")
	runGitAt(t, cloneRepo, "update-ref", "refs/heads/main", lagTip)
	lagging, err := loadWorld(cloneRoot)
	if err != nil {
		t.Fatal(err)
	}
	if lagging.Identity == first.Identity || lagging.Identity.MainTip == first.Identity.MainTip || lagging.Identity.ReceiptBlob == first.Identity.ReceiptBlob {
		t.Fatalf("lagging fleet input was not named: current=%+v lagging=%+v", first.Identity, lagging.Identity)
	}
	currentReport := renderReport(first, period, "", computeRows(first, period, "", loadThresholds(f.root)), true)
	laggingReport := renderReport(lagging, period, "", computeRows(lagging, period, "", loadThresholds(cloneRoot)), true)
	if !strings.Contains(currentReport, "source_identity="+first.Identity.String()) ||
		!strings.Contains(laggingReport, "source_identity="+lagging.Identity.String()) ||
		first.Identity.String() == lagging.Identity.String() {
		t.Fatalf("reports did not print their distinct as-of identities:\ncurrent=%s\nlagging=%s", currentReport, laggingReport)
	}
}

func fleetValues(rows []metricRow) map[string]string {
	values := map[string]string{}
	for _, row := range rows {
		if row.Scope == "fleet-synced" || row.Key == "cross_machine_collisions" {
			values[row.Key] = row.Value
		}
	}
	return values
}

func runGitAt(t *testing.T, directory string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = directory
	cmd.Env = withoutGitSteering(os.Environ())
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func TestO8MalformedSourcesRejectByNameAndFallbackIsUsable(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	f.write("metasystem/artifacts/agents/jobs/truncated.json", "{")
	f.write("metasystem/artifacts/agents/jobs/trailing.json", `{"jobId":"trailing","role":"implementer","status":"completed","startedAt":"2026-08-19T00:00:00Z","endedAt":"2026-08-19T01:00:00Z"} trailing`)
	f.writeJSON("metasystem/artifacts/agents/jobs/bad-time.json", map[string]any{
		"jobId": "bad-time", "role": "implementer", "status": "completed",
		"startedAt": "not-a-time", "endedAt": "2026-08-20T00:00:00Z",
	})
	f.writeJSON("metasystem/artifacts/agents/jobs/running.json", map[string]any{
		"jobId": "running", "role": "implementer", "status": "running",
		"startedAt": "2026-08-20T00:00:00Z",
	})
	torn := filepath.Join(f.evidence, "suite-failures", "torn-envelope")
	if err := os.MkdirAll(torn, 0o755); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(f.evidence, "suite-failures", "fallback-envelope")
	if err := os.MkdirAll(fallback, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fallback, "outcome.json"), []byte(`{"verdict":"green"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(fallback, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	w, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	jobDetails := strings.Join(w.JobCoverage.Details, "\n")
	proofDetails := strings.Join(w.ProofCoverage.Details, "\n")
	if w.JobCoverage.Rejected != 3 || !strings.Contains(jobDetails, "truncated.json") || !strings.Contains(jobDetails, "trailing.json") || !strings.Contains(jobDetails, "bad-time.json") {
		t.Fatalf("malformed jobs not rejected by name: %+v", w.JobCoverage)
	}
	if strings.Contains(jobDetails, "running.json") {
		t.Fatalf("lawful in-flight job was rejected as timing rot: %+v", w.JobCoverage)
	}
	foundRunning := false
	for _, job := range w.Jobs {
		if job.JobID == "running" && job.TimingError == "" {
			foundRunning = true
		}
	}
	if !foundRunning {
		t.Fatalf("lawful in-flight job was not retained cleanly: %+v", w.Jobs)
	}
	if w.ProofCoverage.Rejected < 1 || !strings.Contains(proofDetails, "torn-envelope") {
		t.Fatalf("torn envelope not rejected by name: %+v", w.ProofCoverage)
	}
	foundFallback := false
	for _, proof := range w.Proofs {
		if proof.Path == fallback && proof.Fallback {
			foundFallback = true
		}
	}
	if !foundFallback {
		t.Fatalf("outcome-only envelope did not use mtime fallback: %+v", w.Proofs)
	}
	if row := computeCost(w, mustPeriod(t, weeklyOptions(f)), ""); row.Value == "unavailable" || row.Coverage[0].Usable != 3 {
		t.Fatalf("metric did not compute over the usable job remainder: %+v", row)
	}
}

func TestPerGoalAttributionIsExactAndCoveredBySource(t *testing.T) {
	opened := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	claim := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	preDoneLanding := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	done := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	postDone := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	file := &goal.GoalFile{
		Id: "g", State: goal.StateDone, OpenedAt: opened.Format(time.RFC3339),
		NextStep: "Verify exact attribution.",
		History: []goal.HistoryLine{
			{At: opened.Format(time.RFC3339), Opid: "open", Verb: "open"},
			{At: claim.Format(time.RFC3339), Opid: "claim", Verb: "claim"},
			{At: done.Format(time.RFC3339), Opid: "done", Verb: "done"},
		},
	}
	w := world{
		Goals: map[string]goalRecord{"g": {File: file}},
		Jobs: []jobRecord{
			{JobID: "post-done", GoalID: "g", Role: "implementer", Status: "completed", StartedAt: postDone.Add(-time.Hour), EndedAt: postDone},
			{JobID: "unattributed", Role: "implementer", Status: "completed", StartedAt: postDone.Add(-time.Hour), EndedAt: postDone},
		},
		Receipts: []*receiptRecord{
			{At: postDone, Fields: map[string]string{"goal": "g", "corrections": "2"}},
			{At: postDone, Fields: map[string]string{"corrections": "0"}},
		},
		Journals: []journalRecord{
			{Path: "post-done-rejected.json", Verb: "claim", Targets: []string{"g"}, Outcome: "rejected", TerminalAt: postDone},
			{Path: "post-done-lost.json", Verb: "claim", Targets: []string{"g"}, Outcome: "lost", TerminalAt: postDone},
			{Path: "wrong-goal.json", Verb: "claim", Targets: []string{"g-other"}, Outcome: "rejected", TerminalAt: postDone},
		},
		Landings: []landingCommit{
			{At: preDoneLanding, ChangedLines: 5, Goals: map[string]bool{"g": true}},
			{At: postDone, ChangedLines: 7, Goals: map[string]bool{"g": true}},
			{At: postDone, ChangedLines: 3, Goals: map[string]bool{}},
		},
		GoalCoverage:    Coverage{Source: "goals", Found: 1},
		JobCoverage:     Coverage{Source: "jobs", Found: 2},
		ReceiptCoverage: Coverage{Source: "receipts", Found: 2},
		LandingCoverage: Coverage{Source: "landings", Found: 3},
		JournalCoverage: Coverage{Source: "goal-journal", Found: 3},
	}
	period := Period{Start: opened, End: postDone.Add(24 * time.Hour), Instant: postDone.Add(24 * time.Hour)}

	overhead := computeOverhead(w, period, "g", thresholds{})
	if !strings.Contains(overhead.Value, "wall_hours=1.000") || !strings.Contains(overhead.Value, "corrections=2 landed_lines=12") {
		t.Fatalf("overhead time-bounded declared records: %+v", overhead)
	}
	waiting := computeWaiting(w, period, "g", thresholds{})
	if !strings.Contains(waiting.Value, "building_hours=24.000 proving_hours=24.000") ||
		waiting.Coverage[1].Extra != "goal=g attributed=2 total=3" || !detailsContain(waiting, "source=landings bucket=UNATTRIBUTED records=1") {
		t.Fatalf("waiting attribution coverage or lifecycle split drifted: %+v", waiting)
	}
	cost := computeCost(w, period, "g")
	if !strings.Contains(cost.Value, "wall_hours=1.000; results=1") || cost.Coverage[0].Usable != 1 ||
		cost.Coverage[0].Extra != "goal=g attributed=1 total=2" || !detailsContain(cost, "source=jobs bucket=UNATTRIBUTED records=1") {
		t.Fatalf("cost attribution coverage drifted: %+v", cost)
	}
	friction := computeFriction(w, period, "g")
	if friction.Value != "verb=claim rejected=1 terminal=2 rate=0.500" {
		t.Fatalf("post-conclusion journal declarations were lifecycle-bounded: %+v", friction)
	}
	collisions := computeCollisions(w, period, "g", thresholds{})
	if collisions.Coverage[1].Usable != 1 || !detailsContain(collisions, "post-done-lost.json") || detailsContain(collisions, "wrong-goal.json") {
		t.Fatalf("collision context did not use exact journal target declarations: %+v", collisions)
	}
}

func TestCostWallClockIsUnavailableWithoutTimedJobs(t *testing.T) {
	done := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	ended := done.Add(24 * time.Hour)
	w := world{
		Goals: map[string]goalRecord{"g": {File: &goal.GoalFile{Id: "g", State: goal.StateDone}}},
		Jobs: []jobRecord{{
			JobID: "timing-less", GoalID: "g", Role: "implementer", Status: "completed",
			EndedAt: ended, TimingError: "missing startedAt",
		}},
		JobCoverage: Coverage{Source: "jobs", Found: 1, Rejected: 1},
	}
	row := computeCost(w, Period{Start: done, End: ended.Add(24 * time.Hour), Instant: ended.Add(24 * time.Hour)}, "g")
	if !strings.Contains(row.Value, "wall_hours=unavailable; results=1") || strings.Contains(row.Value, "wall_hours=0.000") ||
		row.Coverage[0].Usable != 0 || row.Coverage[0].Extra != "goal=g attributed=1 total=1" {
		t.Fatalf("timing-less job was presented as usable wall-clock evidence: %+v", row)
	}
}

func TestO9JobUnionDeduplicatesAndLocalWinsTerminalConflict(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	local := filepath.Join(f.root, "artifacts", "agents", "jobs", "j1.json")
	data, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(f.evidence, "agents", "segment", "chain", "jobs", "j1.json")
	legacy := filepath.Join(f.evidence, "agents", "legacy-chain", "jobs", "j1.json")
	for _, path := range []string{current, legacy} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var conflicting map[string]any
	if err := json.Unmarshal(data, &conflicting); err != nil {
		t.Fatal(err)
	}
	conflicting["status"] = "failed"
	changed, _ := json.Marshal(conflicting)
	if err := os.WriteFile(legacy, changed, 0o644); err != nil {
		t.Fatal(err)
	}
	records, coverage := loadJobs(f.root)
	count := 0
	for _, record := range records {
		if record.JobID == "j1" {
			count++
			if record.Status != "completed" || !record.Local {
				t.Fatalf("local record did not win: %+v", record)
			}
		}
	}
	if count != 1 || coverage.Rejected != 1 || !strings.Contains(strings.Join(coverage.Details, "\n"), "terminal status failed conflicts with completed") {
		t.Fatalf("dedup contract failed: count=%d coverage=%+v", count, coverage)
	}
}

func TestO10LifecycleEdgesStayIncompleteOrNameEpochs(t *testing.T) {
	period := Period{
		Start:   time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		Instant: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
	}
	makeRecord := func(id string, history []goal.HistoryLine) goalRecord {
		return goalRecord{File: &goal.GoalFile{Id: id, State: goal.StateDone, OpenedAt: "2026-08-01T00:00:00Z", History: history}}
	}
	queuedDone := makeRecord("queued-done", []goal.HistoryLine{
		{At: "2026-08-01T00:00:00Z", Opid: "a", Verb: "open"},
		{At: "2026-08-20T00:00:00Z", Opid: "b", Verb: "done"},
	})
	multi := makeRecord("multi", []goal.HistoryLine{
		{At: "2026-08-02T00:00:00Z", Opid: "c", Verb: "claim"},
		{At: "2026-08-10T00:00:00Z", Opid: "d", Verb: "claim"},
		{At: "2026-08-20T00:00:00Z", Opid: "e", Verb: "done"},
	})
	noDone := makeRecord("no-done", []goal.HistoryLine{{At: "2026-08-02T00:00:00Z", Opid: "f", Verb: "claim"}})
	w := world{
		Goals:        map[string]goalRecord{"queued-done": queuedDone, "multi": multi, "no-done": noDone},
		Landings:     []landingCommit{{At: time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), Goals: map[string]bool{"multi": true}}},
		GoalCoverage: Coverage{Source: "goals", Found: 3}, LandingCoverage: Coverage{Source: "landings", Found: 1},
	}
	limits := thresholds{Waiting: shareLimit{Raw: "0.5", Value: 0.5}}
	queuedRow := computeWaiting(w, period, "queued-done", limits)
	multiRow := computeWaiting(w, period, "multi", limits)
	noDoneRow := computeWaiting(w, period, "no-done", limits)
	aggregate := computeWaiting(w, period, "", limits)
	if !detailsContain(queuedRow, "lifecycle incomplete: goal=queued-done epochs=0") ||
		!strings.Contains(multiRow.Value, "epochs=2") ||
		!detailsContain(noDoneRow, "lifecycle incomplete: goal=no-done epochs=0") ||
		strings.Contains(aggregate.Value, "queued-done") || strings.Contains(aggregate.Value, "no-done") || !strings.Contains(aggregate.Value, "multi") {
		t.Fatalf("lifecycle edges drifted:\nqueued=%+v\nmulti=%+v\nno-done=%+v\naggregate=%+v", queuedRow, multiRow, noDoneRow, aggregate)
	}
}

func TestWaitingZeroDurationLifecycleIsLabelledWithoutJudgment(t *testing.T) {
	stamp := "2026-08-20T12:00:00Z"
	at, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		t.Fatal(err)
	}
	file := &goal.GoalFile{
		Id: "instant", State: goal.StateDone, OpenedAt: stamp,
		History: []goal.HistoryLine{
			{At: stamp, Opid: "open", Verb: "open"},
			{At: stamp, Opid: "claim", Verb: "claim"},
			{At: stamp, Opid: "done", Verb: "done"},
		},
	}
	w := world{
		Goals:           map[string]goalRecord{"instant": {File: file}},
		Landings:        []landingCommit{{At: at, Goals: map[string]bool{"instant": true}}},
		GoalCoverage:    Coverage{Source: "goals", Found: 1},
		LandingCoverage: Coverage{Source: "landings", Found: 1},
		ProofCoverage:   Coverage{Source: "proof-evidence", Missing: 1},
	}
	row := computeWaiting(w, Period{Instant: at}, "instant", thresholds{Waiting: shareLimit{Raw: "0.5", Value: 0.5}})
	if row.Value != "instant building_hours=0.000 proving_hours=0.000 waiting_share=unavailable (zero-duration lifecycle) epochs=1" ||
		!detailsContain(row, "degenerate lifecycle: goal=instant") ||
		row.Thresholds[0] != "proving share max=0.5; not evaluated" ||
		strings.Contains(row.Value, "NaN") {
		t.Fatalf("zero-duration lifecycle was not reported as degenerate: %+v", row)
	}
}

func detailsContain(row metricRow, value string) bool {
	for _, item := range row.Details {
		if strings.Contains(item.Text, value) {
			return true
		}
	}
	return false
}

func TestO11CollisionSemanticsSeparateTrueEventsFromContext(t *testing.T) {
	period := Period{Start: time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), Instant: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)}
	file := &goal.GoalFile{Id: "g", History: []goal.HistoryLine{{At: "2026-08-20T00:00:00Z", Opid: "true", Verb: "steal", Displaced: "m+l@at"}}}
	w := world{
		Goals: map[string]goalRecord{"g": {File: file}}, GoalCoverage: Coverage{Source: "goals", Found: 1},
		Journals: []journalRecord{
			{Path: "lost.json", Verb: "claim", Outcome: "lost", Attempts: 1, TerminalAt: time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC)},
			{Path: "late.json", Verb: "claim", Outcome: "confirmed-late", Attempts: 2, TerminalAt: time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC)},
			{Path: "rejected.json", Verb: "claim", Outcome: "rejected", Attempts: 1, TerminalAt: time.Date(2026, 8, 20, 3, 0, 0, 0, time.UTC)},
			{Path: "retry.json", Verb: "claim", Outcome: "confirmed", Attempts: 2, TerminalAt: time.Date(2026, 8, 20, 4, 0, 0, 0, time.UTC)},
		}, JournalCoverage: Coverage{Source: "goal-journal", Found: 4},
	}
	row := computeCollisions(w, period, "", thresholds{Collisions: intMaximum{Raw: "0", Value: 0}})
	if row.Value != "true_cross_machine_events=1 displaced=1 steals=1" || !strings.Contains(row.Thresholds[0], "crossed") {
		t.Fatalf("true collision did not fire: %+v", row)
	}
	if !detailsContain(row, "contested transaction") || !detailsContain(row, "retry.json attempts=2 confirmed") || detailsContain(row, "late.json") || detailsContain(row, "rejected.json") {
		t.Fatalf("journal context classes drifted: %+v", row.Details)
	}
	if row.Coverage[1].Usable != 2 {
		t.Fatalf("excluded journal outcomes entered collision coverage: %+v", row.Coverage[1])
	}
}

func TestO12AtomicWriteFailureKeepsPriorGoalReport(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	target := GoalReportTarget(f.root, "g1")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	prior := []byte("prior report bytes\n")
	if err := os.WriteFile(target, prior, 0o644); err != nil {
		t.Fatal(err)
	}
	standing := writeReport
	writeReport = func(path, content, anchor string) error { return errors.New("simulated pre-publication failure") }
	t.Cleanup(func() { writeReport = standing })
	result, err := Report(Options{Root: f.root, GoalID: "g1", PeriodEnd: "2026-08-24T00:00:00Z"})
	if err == nil || result.Target != target || !strings.Contains(err.Error(), target) {
		t.Fatalf("failed write did not name exact target: result=%+v err=%v", result, err)
	}
	after, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !reflect.DeepEqual(after, prior) {
		t.Fatalf("failed atomic publication changed prior bytes: %q", after)
	}
}

func TestUnknownGoalPublishesUnavailableReportWithoutGlobalValues(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	result, err := Report(Options{Root: f.root, GoalID: "missing-goal", PeriodEnd: "2026-08-24T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(result.Target)
	if err != nil {
		t.Fatal(err)
	}
	report := string(data)
	for _, want := range []string{
		"report_kind=goal\ngoal=missing-goal\nreport_status=UNAVAILABLE\n",
		"coverage=source=goals found=4 usable=0 rejected=0 missing=1 goal=missing-goal status=missing-goal",
		"detail=requested goal missing-goal is absent from the accepted goal ledger",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("unknown-goal report missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "\nmetric=") || strings.Contains(report, "\nvalue=") || strings.Contains(report, "days_since_green=") {
		t.Fatalf("unknown-goal report leaked global metric values:\n%s", report)
	}
}

func TestO13MetricsAttributeOnlyExactGoalID(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	unattributed := filepath.Join(f.root, "artifacts", "agents", "critiques", "unattributed-chain")
	if err := os.MkdirAll(unattributed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unattributed, "r1-output.md"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.job("wrong-goal", map[string]any{
		"jobId": "wrong-goal", "role": "implementer", "status": "completed", "goalId": "g10", "round": 1,
		"runtime": "codex", "startedAt": "2026-08-18T10:00:00Z", "endedAt": "2026-08-18T11:00:00Z",
	})
	w, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	row := computeOverhead(w, mustPeriod(t, weeklyOptions(f)), "g1", loadThresholds(f.root))
	if !strings.Contains(row.Value, "wall_hours=4.000") || !strings.Contains(row.Value, "critique_rounds=2") ||
		!strings.Contains(row.Coverage[1].Extra, "attributed=3 total=4") || !detailsContain(row, "critique chain unattributed: unattributed-chain") {
		t.Fatalf("prefix-like goal id was attributed: value=%s coverage=%+v", row.Value, row.Coverage[1])
	}
	foundUnattributed := false
	for _, chain := range w.Critiques {
		if chain.Name == "unattributed-chain" && chain.GoalID == "" {
			foundUnattributed = true
		}
	}
	if !foundUnattributed {
		t.Fatalf("chain created without goal provenance was not reported unattributed: %+v", w.Critiques)
	}
}

func TestO14MetricsOnlyCommitAndReceiptAreSelfExcluded(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	// Keep the local evidence files out of the commit whose path set proves
	// metrics-only self-exclusion.
	f.commit("2026-08-20T23:00:00Z", "anchor fixture evidence", true)
	before, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	period := mustPeriod(t, weeklyOptions(f))
	beforeValues := allValues(computeRows(before, period, "", loadThresholds(f.root)))
	data, err := os.ReadFile(filepath.Join(f.root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	selfReceipt := "1772000000|2026-08-21T00:00:00Z|RECEIPT|type=metrics-report|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|goal=g1|built_by=coordinator|critique_waived=none|waiver_stream=none|note=report"
	f.write("metasystem/memory/receipts.log", string(data)+selfReceipt+"\n")
	f.write("metasystem/plans/metrics/machine-a/2026-W34.md", "report\n")
	f.commit("2026-08-21T00:00:00Z", "land metrics report", false)
	after, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	afterValues := allValues(computeRows(after, period, "", loadThresholds(f.root)))
	if !reflect.DeepEqual(beforeValues, afterValues) {
		t.Fatalf("self landing changed metric values:\nbefore=%v\nafter=%v", beforeValues, afterValues)
	}
}

func allValues(rows []metricRow) map[string]string {
	result := map[string]string{}
	for _, row := range rows {
		result[row.Key] = row.Value
	}
	return result
}

func TestO15LandingReceiptJoinHandlesUnattributedAndSharedCommits(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	f.write("metasystem/unattributed.txt", "work\n")
	unattributed := f.commit("2026-08-20T00:00:00Z", "unattributed landing", false)
	data, err := os.ReadFile(filepath.Join(f.root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	rows := "1773000000|2026-08-20T01:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|goal=g1|built_by=delegate|critique_waived=none|waiver_stream=none|note=shared\n" +
		"1773000001|2026-08-20T01:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|goal=g2|built_by=delegate|critique_waived=none|waiver_stream=none|note=shared\n"
	f.write("metasystem/memory/receipts.log", string(data)+rows)
	f.write("metasystem/shared.txt", "shared\n")
	sharedSHA := f.commit("2026-08-20T01:00:00Z", "shared landing", false)
	g2 := &goal.GoalFile{
		Id: "g2", State: goal.StateDone, Intent: "Share one landing.", Origin: goal.OriginMain,
		NextStep: "Share it.", Conclude: "Shared.", OpenedAt: "2026-08-01T00:00:00Z", Revision: 3,
		Budget: &goal.Budget{ElapsedLimit: "2h", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1},
		History: []goal.HistoryLine{
			{At: "2026-08-01T00:00:00Z", Opid: "01J5X00000000000000000S000-machine-a-11111111", Verb: "open", Actor: "machine-a+fixture", Targets: []string{"g2"}, Keep: -1},
			{At: "2026-08-10T00:00:00Z", Opid: "01J5X00000000000000000S010-machine-a-11111111", Verb: "claim", Actor: "machine-a+fixture", Targets: []string{"g2"}, Keep: -1},
			{At: "2026-08-21T00:00:00Z", Opid: "01J5X00000000000000000S020-machine-a-11111111", Verb: "done", Actor: "machine-a+fixture", Targets: []string{"g2"}, Keep: -1},
		},
	}
	if err := os.WriteFile(filepath.Join(f.root, "plans", "goals", "done", "g2.md"), goal.RenderFile(g2), 0o644); err != nil {
		t.Fatal(err)
	}
	accepted := f.commit("2026-08-27T00:00:00Z", "publish shared goal facts", true)
	f.run("update-ref", goal.AcceptedRef, accepted)
	w, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	foundUnattributed, foundShared := false, false
	sharedPayload, g1Payload := 0, 0
	for _, commit := range w.Landings {
		if commit.SHA == unattributed && len(commit.Goals) == 0 {
			foundUnattributed = true
		}
		if commit.SHA == sharedSHA && commit.Goals["g1"] && commit.Goals["g2"] && commit.Shared {
			foundShared = true
			sharedPayload = commit.ChangedLines
		}
		if commit.Goals["g1"] {
			g1Payload += commit.ChangedLines
		}
	}
	if !foundUnattributed || !foundShared || sharedPayload == 0 {
		t.Fatalf("landing join failed: unattributed=%v shared=%v shared_payload=%d landings=%+v", foundUnattributed, foundShared, sharedPayload, w.Landings)
	}
	for _, test := range []struct {
		id      string
		payload int
	}{
		{id: "g1", payload: g1Payload},
		{id: "g2", payload: sharedPayload},
	} {
		result, err := Report(Options{Root: f.root, GoalID: test.id, PeriodEnd: "2026-08-24T00:00:00Z"})
		if err != nil {
			t.Fatalf("goal %s report: %v", test.id, err)
		}
		data, err := os.ReadFile(result.Target)
		if err != nil {
			t.Fatal(err)
		}
		report := string(data)
		for _, want := range []string{fmt.Sprintf("landed_lines=%d", test.payload), "shared_commits=1"} {
			if !strings.Contains(report, want) {
				t.Fatalf("goal %s report missing %q:\n%s", test.id, want, report)
			}
		}
	}
}

func TestO16AgeIgnoresWindowEventsDoNotAndGoalUsesWholeLifecycle(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	f.job("old-job", map[string]any{
		"jobId": "old-job", "role": "implementer", "status": "completed", "goalId": "g1", "round": 1,
		"runtime": "codex", "startedAt": "2026-08-03T00:00:00Z", "endedAt": "2026-08-03T01:00:00Z", "usage": map[string]any{"inputTokens": 1},
	})
	f.job("middle-job", map[string]any{
		"jobId": "middle-job", "role": "implementer", "status": "completed", "goalId": "g1", "round": 1,
		"runtime": "codex", "startedAt": "2026-08-10T00:00:00Z", "endedAt": "2026-08-10T01:00:00Z", "usage": map[string]any{"inputTokens": 1},
	})
	w, err := loadWorld(f.root)
	if err != nil {
		t.Fatal(err)
	}
	week := mustPeriod(t, weeklyOptions(f))
	stale := computeStaleChecks(w, week, loadThresholds(f.root))
	debt := computeDebt(w, week, "", loadThresholds(f.root))
	if !strings.Contains(stale.Value, "days_since_green=9.000") || !strings.Contains(stale.Thresholds[0], "crossed") ||
		!strings.Contains(debt.Value, "age_days=54.000") || !strings.Contains(debt.Thresholds[0], "crossed") {
		t.Fatalf("age metrics were window-bound: stale=%+v debt=%+v", stale, debt)
	}
	nextWeek := Period{Start: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), Instant: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}
	if row := computeRework(w, nextWeek, "", loadThresholds(f.root)); row.Value != "unavailable" {
		t.Fatalf("original receipt leaked into another event week: %s", row.Value)
	}
	goalCost := computeCost(w, Period{Instant: week.Instant}, "g1")
	if goalCost.Coverage[0].Usable != 5 {
		t.Fatalf("per-goal cost did not read all three weeks: %+v", goalCost.Coverage[0])
	}
}

func TestO17InvalidThresholdsDisableWithoutFiring(t *testing.T) {
	f := newFixtureRepo(t)
	f.seedFullWorld()
	conf := "evidence.root=" + f.evidence + "\n" +
		"metrics.stale-checks.max-days=-1\n" +
		"metrics.rework.max-share=1.2\n" +
		"metrics.overhead.spend-min=4\nmetrics.overhead.spend-max=3\n" +
		"metrics.overhead.density-min=bogus\n" +
		"metrics.waiting.max-share=not-a-share\n" +
		"metrics.delegates.min-share=NaN\n"
	f.write("metasystem/metasystem.conf", conf)
	result, err := Report(weeklyOptions(f))
	if err != nil {
		t.Fatalf("invalid threshold aborted report: %v", err)
	}
	report := detailedReport(t, result)
	for _, want := range []string{
		"threshold invalid: metrics.stale-checks.max-days=-1; threshold disabled",
		"threshold invalid: metrics.rework.max-share=1.2; threshold disabled",
		"threshold invalid: metrics.overhead.spend-min=4,metrics.overhead.spend-max=3; threshold disabled",
		"threshold invalid: metrics.overhead.density-min=bogus; threshold disabled",
		"threshold invalid: metrics.waiting.max-share=not-a-share; threshold disabled",
		"threshold invalid: metrics.delegates.min-share=NaN; threshold disabled",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("invalid threshold line missing: %q\n%s", want, report)
		}
	}
}
