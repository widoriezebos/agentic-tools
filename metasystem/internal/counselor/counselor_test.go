package counselor

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func TestComputeSignalsFromFixtureRecords(t *testing.T) {
	stamp := func(value string) time.Time {
		t.Helper()
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	records := RecordSet{
		Runs: []RunObservation{
			{ID: "governed-proof", Kind: RunGoverned, CompletedAt: stamp("2026-08-12T12:00:00Z"), Duration: 90 * time.Minute, Outcome: OutcomeGreen},
			{ID: "tracked-suite", Kind: RunTracked, CompletedAt: stamp("2026-08-13T12:00:00Z"), Duration: 30 * time.Minute, Outcome: OutcomeRed},
		},
		Landings: []LandingObservation{
			{Commit: "product", CompletedAt: stamp("2026-08-14T10:00:00Z"), Files: 2, Insertions: 50, Paths: []string{"internal/app.go", "internal/app_test.go"}},
			{Commit: "records", CompletedAt: stamp("2026-08-14T11:00:00Z"), Files: 1, Insertions: 4, BinaryFiles: 1, Paths: []string{"plans/goals/work.md"}},
			{Commit: "unclassified", CompletedAt: stamp("2026-08-14T12:00:00Z")},
		},
		GoalEvents: []GoalEventObservation{
			{OperationID: "open", At: stamp("2026-08-11T08:00:00Z"), Class: GoalOpen},
			{OperationID: "edit", At: stamp("2026-08-11T09:00:00Z"), Class: GoalEdit},
			{OperationID: "claim", At: stamp("2026-08-11T10:00:00Z"), Class: GoalClaim},
			{OperationID: "budget", At: stamp("2026-08-11T11:00:00Z"), Class: GoalBudget},
			{OperationID: "done", At: stamp("2026-08-15T11:00:00Z"), Class: GoalDone},
		},
	}

	brief := Compute(records, stamp("2026-08-25T12:00:00Z"))
	if len(brief.SpendVsOutcome.Windows) != 2 || len(brief.ProcessVsProduct.Windows) != 2 {
		t.Fatalf("completed observed weeks plus latest week not preserved: %+v", brief)
	}
	spend := brief.SpendVsOutcome.Windows[0]
	if spend.Governed.Runs != 1 || spend.Governed.Duration != 90*time.Minute || spend.Governed.Outcomes.Green != 1 {
		t.Fatalf("governed spend mismatch: %+v", spend.Governed)
	}
	if spend.Tracked.Runs != 1 || spend.Tracked.Duration != 30*time.Minute || spend.Tracked.Outcomes.Red != 1 {
		t.Fatalf("tracked spend mismatch: %+v", spend.Tracked)
	}
	if spend.Landings.Commits != 3 || spend.Landings.Files != 3 || spend.Landings.Insertions != 54 || spend.Landings.BinaryFiles != 1 {
		t.Fatalf("landing outcome mismatch: %+v", spend.Landings)
	}

	activity := brief.ProcessVsProduct.Windows[0]
	if activity.GoalEvents != (GoalVerbCounts{Open: 1, Edit: 1, Claim: 1, Budget: 1, Done: 1}) {
		t.Fatalf("goal verb counts mismatch: %+v", activity.GoalEvents)
	}
	if activity.RecordOnlyLandings != 1 || activity.ProductLandings != 1 || activity.UnclassifiedLandings != 1 {
		t.Fatalf("landing classes mismatch: %+v", activity)
	}
	if activity.Ratio.Value == nil || *activity.Ratio.Value != 2.5 || activity.Ratio.ProcessActivities != 5 || activity.Ratio.ProductOutcomes != 2 {
		t.Fatalf("activity ratio mismatch: %+v", activity.Ratio)
	}
	if brief.ProcessVsProduct.Windows[1].Ratio.Value != nil {
		t.Fatalf("zero-denominator ratio must stay undefined: %+v", brief.ProcessVsProduct.Windows[1].Ratio)
	}
}

func TestProcessRatioDoesNotCountGoalOperationCarrierCommitsTwice(t *testing.T) {
	stamp := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	brief := Compute(RecordSet{
		GoalEvents: []GoalEventObservation{
			{OperationID: "edit-one", At: stamp, Class: GoalEdit},
			{OperationID: "edit-two", At: stamp, Class: GoalEdit},
			{OperationID: "done", At: stamp, Class: GoalDone},
		},
		Landings: []LandingObservation{
			{Commit: "edit-one-carrier", CompletedAt: stamp, Paths: []string{"plans/goals/a.md"}, GoalOperationID: "edit-one"},
			{Commit: "edit-two-carrier", CompletedAt: stamp, Paths: []string{"plans/goals/a.md"}, GoalOperationID: "edit-two"},
			{Commit: "done-carrier", CompletedAt: stamp, Paths: []string{"records/goals/a.md"}, GoalOperationID: "done"},
		},
	}, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	window := brief.ProcessVsProduct.Windows[0]
	if window.GoalCarrierLandings != 3 || window.RecordOnlyLandings != 0 {
		t.Fatalf("goal carrier classification mismatch: %+v", window)
	}
	if window.Ratio.Value == nil || *window.Ratio.Value != 2 || window.Ratio.ProcessActivities != 2 || window.Ratio.ProductOutcomes != 1 {
		t.Fatalf("goal carrier commits inflated the ratio: %+v", window.Ratio)
	}
	var rendered bytes.Buffer
	if err := Render(&rendered, brief); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Git landing classes were non-carrier record-only 0, matched goal-operation carrier 3",
		"activity ratio is 2.00, from 2 process activities divided by 1 product outcome",
	} {
		if !strings.Contains(rendered.String(), expected) {
			t.Fatalf("corrected carrier accounting missing %q:\n%s", expected, rendered.String())
		}
	}

	unmatched := Compute(RecordSet{Landings: []LandingObservation{{
		Commit: "unmatched", CompletedAt: stamp, Paths: []string{"plans/goals/a.md"}, GoalOperationID: "missing",
	}}}, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	if unmatched.ProcessVsProduct.Windows[0].RecordOnlyLandings != 1 ||
		!hasLimitation(unmatched.ProcessVsProduct.Limitations, "Goal-carrier reconciliation") {
		t.Fatalf("unmatched carrier was silently discarded: %+v", unmatched.ProcessVsProduct)
	}
}

func TestRenderCarriesNamedLimitationsInNarrative(t *testing.T) {
	stamp := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	brief := Compute(RecordSet{
		Runs: []RunObservation{
			{ID: "unknown", Kind: RunGoverned, CompletedAt: stamp, Duration: 90 * time.Second, Outcome: OutcomeEndedUnknown},
			{ID: "failed", Kind: RunTracked, CompletedAt: stamp, Duration: 2 * time.Minute, Outcome: OutcomeLaunchFailed},
		},
		Landings:   []LandingObservation{{Commit: "product", CompletedAt: stamp, Files: 1, Insertions: 1, Paths: []string{"cmd/app.go"}}},
		GoalEvents: []GoalEventObservation{{OperationID: "open", At: stamp, Class: GoalOpen}},
	}, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))
	var output bytes.Buffer
	if err := Render(&output, brief); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, expected := range []string{
		"This brief reads durable records only, offers advice, and makes no decision or refusal.",
		"Cost: During 2026-08-10 through 2026-08-17 (end excluded)",
		"1.5 minutes",
		"Limitation — Risk-class evidence:",
		"Limitation — Threshold history:",
		"Limitation — Causal attribution:",
		"Classification rule: Goal events are deduplicated by operation identifier.",
		"Every other landing is record-only",
		"activity ratio is 1.00",
		"ratio is undefined",
		"Limitation — Path classification:",
		"Limitation — Goal-verb coverage:",
		"Limitation — Register-edit classification:",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("narrative missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "|---") {
		t.Fatalf("brief rendered a dashboard table:\n%s", text)
	}
}

func TestParseGitLogCarriesTextAndBinaryFacts(t *testing.T) {
	log := "\x1eabc\x1f2026-08-12T12:00:00Z\x1foperation-one\n3\t1\tmetasystem/internal/app.go\n-\t-\tmetasystem/assets/image.png\n" +
		"\x1ebad\x1fheader\n"
	landings, rejected := parseGitLog(log, "metasystem/")
	if len(landings) != 1 || rejected != 1 {
		t.Fatalf("unexpected parse outcome: landings=%+v rejected=%d", landings, rejected)
	}
	landing := landings[0]
	if landing.Commit != "abc" || landing.GoalOperationID != "operation-one" || landing.Files != 2 || landing.Insertions != 3 || landing.BinaryFiles != 1 ||
		len(landing.Paths) != 2 || landing.Paths[0] != "internal/app.go" {
		t.Fatalf("numstat facts mismatch: %+v", landing)
	}
}

func TestClassifiersAreClosedAndAuditable(t *testing.T) {
	if got := classifyLanding(nil); got != landingUnclassified {
		t.Fatalf("empty landing classified as %v", got)
	}
	if got := classifyLanding([]string{"memory/register.md", "plans/goals/a.md", "records/goals/b.md"}); got != landingRecordOnly {
		t.Fatalf("record-only landing classified as %v", got)
	}
	if got := classifyLanding([]string{"plans/goals/a.md", "cmd/app.go"}); got != landingProduct {
		t.Fatalf("mixed landing classified as %v", got)
	}
	for verb, want := range map[string]GoalVerbClass{"open": GoalOpen, "edit": GoalEdit, "claim": GoalClaim, "set-budget": GoalBudget, "done": GoalDone} {
		if got, ok := goalVerbClass(verb); !ok || got != want {
			t.Fatalf("verb %q classified as %q, %v", verb, got, ok)
		}
	}
	if _, ok := goalVerbClass("open-claim"); ok {
		t.Fatal("combined verb silently entered an exact class")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestRenderSurfacesWriterFailure(t *testing.T) {
	if err := Render(failingWriter{}, Brief{}); err == nil {
		t.Fatal("writer failure was hidden")
	}
}

func TestBuildReadsDurableFixtureSources(t *testing.T) {
	root := t.TempDir()
	mustGit(t, root, "init", "-q")
	mustGit(t, root, "config", "user.name", "Fixture")
	mustGit(t, root, "config", "user.email", "fixture@example.invalid")
	mustGit(t, root, "config", "commit.gpgsign", "false")

	rootRecord := &goal.RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
	}
	goalRecord := &goal.GoalFile{
		Id: "fixture", State: goal.StateDone, Intent: "Prove the counselor sources.", Origin: goal.OriginMain,
		Conclude: "Fixture landed.", OpenedAt: "2026-08-11T08:00:00Z", Revision: 3,
		History: []goal.HistoryLine{
			{At: "2026-08-11T08:00:00Z", Opid: "01J5X00000000000000000P000-m1-11111111", Verb: "open", Actor: "m1+fixture", Targets: []string{"fixture"}, Keep: -1},
			{At: "2026-08-11T09:00:00Z", Opid: "01J5X00000000000000000P001-m1-11111111", Verb: "edit", Actor: "m1+fixture", Targets: []string{"fixture"}, Keep: -1},
			{At: "2026-08-11T10:00:00Z", Opid: "01J5X00000000000000000P002-m1-11111111", Verb: "done", Actor: "m1+fixture", Targets: []string{"fixture"}, Keep: -1},
		},
	}
	writeFixture(t, filepath.Join(root, "plans", "goals", "backlog.md"), goal.RenderRoot(rootRecord))
	writeFixture(t, filepath.Join(root, "records", "goals", "fixture.md"), goal.RenderFile(goalRecord))
	mustGit(t, root, "add", ".")
	mustGitAt(t, root, "2026-08-12T10:00:00Z", "commit", "-q", "-m", "goal records", "-m", "Goal-Transaction: 01J5X00000000000000000P000-m1-11111111")

	writeFixture(t, filepath.Join(root, "internal", "app.go"), []byte("package app\n"))
	mustGit(t, root, "add", ".")
	mustGitAt(t, root, "2026-08-13T10:00:00Z", "commit", "-q", "-m", "product")

	if err := obligationstate.RecordTerminal(root, "fixture", 3, 1, obligationstate.TerminalAttempt{
		RunID: "governed-proof", Status: run.StatusGreen, StartedAt: "2026-08-12T11:00:00Z", EndedAt: "2026-08-12T11:25:00Z",
		AttemptOrdinal: 1, ExecutionCostMinutes: 30, ObservedCostMinutes: 25, WeightGeneration: 1, Breaker: run.BreakerClosed,
	}); err != nil {
		t.Fatal(err)
	}
	writeRunFixture(t, root, governedRunFixture(
		"governed-proof", run.StatusGreen, "2026-08-12T11:00:00Z", "2026-08-12T11:25:00Z", 25,
	))
	writeRunFixture(t, root, run.Record{
		SchemaVersion: 1, RunId: "tracked-suite", Kind: "suite", Display: "tracked", Custody: run.CustodyWrapped,
		Generation: 1, LaunchNonce: strings.Repeat("a", 32), StartedAt: "2026-08-13T11:00:00Z", StaleAfterMin: 5,
		WindDownMin: run.DefaultWindDown, Evidence: run.Evidence{Mode: run.EvidenceNone}, Status: run.StatusRed,
		TerminalSeq: int64Pointer(1), EndedAt: stringPointer("2026-08-13T11:30:00Z"),
	})
	writeRunFixture(t, root, run.Record{
		SchemaVersion: 1, RunId: "active-suite", Kind: "suite", Display: "active", Custody: run.CustodyWrapped,
		Generation: 1, LaunchNonce: strings.Repeat("b", 32), StartedAt: "2026-08-14T11:00:00Z", StaleAfterMin: 5,
		WindDownMin: run.DefaultWindDown, Evidence: run.Evidence{Mode: run.EvidenceNone}, Status: run.StatusLaunching,
	})

	brief := Build(Options{Root: root, Now: func() time.Time {
		return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	}})
	window := brief.SpendVsOutcome.Windows[0]
	if window.Governed.Runs != 1 || window.Governed.Duration != 25*time.Minute || window.Tracked.Runs != 1 || window.Tracked.Duration != 30*time.Minute {
		t.Fatalf("durable run sources not projected: %+v", window)
	}
	if window.Landings.Commits != 2 || window.Landings.Files != 3 {
		t.Fatalf("current-branch history not projected: %+v limitations=%+v", window.Landings, brief.SpendVsOutcome.Limitations)
	}
	activity := brief.ProcessVsProduct.Windows[0]
	if activity.GoalEvents.Open != 1 || activity.GoalEvents.Edit != 1 || activity.GoalEvents.Done != 1 ||
		activity.RecordOnlyLandings != 0 || activity.GoalCarrierLandings != 1 || activity.ProductLandings != 1 {
		t.Fatalf("goal and path activity not projected: %+v", activity)
	}
	if !hasLimitation(brief.SpendVsOutcome.Limitations, "Active-run boundary") {
		t.Fatalf("active run omission was not carried as data: %+v", brief.SpendVsOutcome.Limitations)
	}
}

func TestGovernedRunReconciliationAndLaunchFailureBoundaries(t *testing.T) {
	root := t.TempDir()
	recordTerminalFixture(t, root, "match-goal", "governed-match", run.StatusGreen, "2026-08-11T10:00:00Z", "2026-08-11T10:10:00Z", 10)
	writeRunFixture(t, root, governedRunFixture(
		"governed-match", run.StatusGreen, "2026-08-11T10:00:00Z", "2026-08-11T10:10:00Z", 10,
	))

	recordTerminalFixture(t, root, "contradiction-goal", "governed-contradiction", run.StatusGreen, "2026-08-12T10:00:00Z", "2026-08-12T10:20:00Z", 20)
	writeRunFixture(t, root, governedRunFixture(
		"governed-contradiction", run.StatusRed, "2026-08-12T10:00:00Z", "2026-08-12T10:20:00Z", 20,
	))

	writeRunFixture(t, root, governedRunFixture(
		"governed-missing", run.StatusRed, "2026-08-13T10:00:00Z", "2026-08-13T10:15:00Z", 15,
	))

	recordTerminalFixture(t, root, "duplicate-one", "governed-duplicate", run.StatusGreen, "2026-08-14T10:00:00Z", "2026-08-14T10:05:00Z", 5)
	recordTerminalFixture(t, root, "duplicate-two", "governed-duplicate", run.StatusGreen, "2026-08-14T10:00:00Z", "2026-08-14T10:05:00Z", 5)
	writeRunFixture(t, root, governedRunFixture(
		"governed-duplicate", run.StatusGreen, "2026-08-14T10:00:00Z", "2026-08-14T10:05:00Z", 5,
	))

	writeRunFixture(t, root, run.Record{
		SchemaVersion: 1, RunId: "launch-failed", Kind: "suite", Display: "launch failure", Custody: run.CustodyWrapped,
		Generation: 1, LaunchNonce: strings.Repeat("c", 32), StartedAt: "2026-08-15T10:00:00Z", StaleAfterMin: 5,
		WindDownMin: run.DefaultWindDown, Evidence: run.Evidence{Mode: run.EvidenceNone}, Status: run.StatusLaunchFailed,
		TerminalSeq: int64Pointer(1),
	})

	brief := Build(Options{Root: root, Now: func() time.Time {
		return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	}})
	window := brief.SpendVsOutcome.Windows[0]
	if window.Governed.Runs != 3 || window.Governed.Duration != 45*time.Minute ||
		window.Governed.Outcomes.Green != 2 || window.Governed.Outcomes.Red != 1 {
		t.Fatalf("governed reconciliation did not count each authoritative run once: %+v", window.Governed)
	}
	if window.Tracked.Runs != 1 || window.Tracked.Duration != 0 || window.Tracked.Outcomes.LaunchFailed != 1 {
		t.Fatalf("launch failure without endedAt was not counted truthfully: %+v", window.Tracked)
	}
	for _, name := range []string{"Governed-run identity", "Governed-run durability", "Governed-run reconciliation", "Launch-failed timing"} {
		if !hasLimitation(brief.SpendVsOutcome.Limitations, name) {
			t.Fatalf("missing %s limitation: %+v", name, brief.SpendVsOutcome.Limitations)
		}
	}
	if hasLimitation(brief.SpendVsOutcome.Limitations, "Tracked-run evidence") {
		t.Fatalf("valid launch failure was mislabeled unreadable: %+v", brief.SpendVsOutcome.Limitations)
	}
}

func TestBuildTurnsUnavailableSourcesIntoCounselLimitations(t *testing.T) {
	brief := Build(Options{Root: t.TempDir(), Now: func() time.Time {
		return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	}})
	if !hasLimitation(brief.SpendVsOutcome.Limitations, "Git landing evidence") {
		t.Fatalf("missing Git evidence did not become spend limitation: %+v", brief.SpendVsOutcome.Limitations)
	}
	if !hasLimitation(brief.ProcessVsProduct.Limitations, "Goal-history evidence") ||
		!hasLimitation(brief.ProcessVsProduct.Limitations, "Git landing evidence") {
		t.Fatalf("missing sources did not remain visible in activity counsel: %+v", brief.ProcessVsProduct.Limitations)
	}
	if len(brief.SpendVsOutcome.Windows) != 1 {
		t.Fatalf("evidence loss prevented the current brief: %+v", brief.SpendVsOutcome.Windows)
	}
}

func writeRunFixture(t *testing.T, root string, record run.Record) {
	t.Helper()
	if problems := run.Validate(&record); len(problems) > 0 {
		t.Fatalf("invalid run fixture: %v", problems)
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, run.RecordPath(root, record.RunId), data)
}

func governedRunFixture(id, status, started, ended string, observedMinutes uint64) run.Record {
	weightGeneration := uint64(1)
	return run.Record{
		SchemaVersion: 1, RunId: id, Kind: "suite", Display: id, Custody: run.CustodyWrapped,
		Generation: 1, LaunchNonce: strings.Repeat("d", 32), StartedAt: started, GoalId: "fixture", StaleAfterMin: 5,
		WindDownMin: run.DefaultWindDown, Evidence: run.Evidence{Mode: run.EvidenceNone}, Status: status,
		TerminalSeq: int64Pointer(1), EndedAt: stringPointer(ended),
		Governed: &run.GovernedAttempt{
			GoalRevision: 3, ObligationRevision: 1, WeightGeneration: &weightGeneration,
			Recurrence: governance.SingleExperiment, ExecutionCostMinutes: 30, ObservedCostMinutes: &observedMinutes,
			AttemptOrdinal: 1, Budget: goalbudget.Budget{
				ElapsedLimit: "2h", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1,
			},
			BudgetStartedAt: started,
			ExpectedAssumptions: governance.ObligationAssumptions{
				Recurrence: governance.SingleExperiment, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
				SurfaceDigest: "fixture-digest", MaxActiveJobs: 1, TimingEnvelopeSeconds: 60, ObservationSource: "run-terminal-record",
			},
			AdmissionDecision: governance.ConsequenceDecision{Apply: true},
			Observation:       &run.AssumptionObservation{ObservedAt: ended, AssumptionState: run.AssumptionMatch},
			Breaker:           run.BreakerClosed,
		},
	}
}

func recordTerminalFixture(t *testing.T, root, goalID, runID, status, started, ended string, observedMinutes uint64) {
	t.Helper()
	if err := obligationstate.RecordTerminal(root, goalID, 3, 1, obligationstate.TerminalAttempt{
		RunID: runID, Status: status, StartedAt: started, EndedAt: ended, AttemptOrdinal: 1,
		ExecutionCostMinutes: 30, ObservedCostMinutes: observedMinutes, WeightGeneration: 1, Breaker: run.BreakerClosed,
	}); err != nil {
		t.Fatal(err)
	}
}

func writeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustGit(t *testing.T, root string, args ...string) {
	t.Helper()
	mustGitAt(t, root, "", args...)
}

func mustGitAt(t *testing.T, root, stamp string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	if stamp != "" {
		command.Env = append(command.Env, "GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp)
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func int64Pointer(value int64) *int64    { return &value }
func stringPointer(value string) *string { return &value }

func hasLimitation(limitations []Limitation, name string) bool {
	for _, limitation := range limitations {
		if limitation.Name == name {
			return true
		}
	}
	return false
}
