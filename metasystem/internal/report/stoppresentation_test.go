package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func stopPresentationRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "installation with spaces")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func stopPresentationFixture(root, attempt string, shouldBlock bool) StopPresentationInput {
	source := "idle-backlog"
	control := StopControl{ShouldBlock: shouldBlock, Class: "idle-with-backlog", JudgmentAvailable: true}
	if shouldBlock {
		control.BlockSource = &source
	}
	session := "session-safe"
	observed := "2026-09-13T12:00:00Z"
	verdict := goal.Verdict{SchemaVersion: 1, Class: control.Class, ShouldBlock: shouldBlock, BlockSource: control.BlockSource, LedgerStatus: "ok", Display: "bounded display"}
	judgment := &goal.TurnVerdictFacts{
		SchemaVersion: 1,
		Identity:      goal.TurnFactsIdentity{Installation: root, Session: session, MainId: "main-->one", ObservedAt: observed},
		Verdict:       verdict,
		FullDisplay:   "uncut verdict detail",
		Scan: goal.ScanResult{
			Open:             []goal.Item{{Kind: "plan", Id: "plans/work.md", Detail: "short", FullDetail: strings.Repeat("complete-plan-step ", 80), SourcePath: "plans/work.md", RequestedAction: "perform the complete plan step"}},
			TemplateUnfilled: []goal.Item{}, OpenWorkWarnings: []string{}, WaitingOnHuman: []goal.Item{},
			StalePlans: []goal.Item{}, Busy: []goal.Item{}, Questions: []goal.Item{}, Drafts: []goal.Item{},
			Unreadable: []string{}, Jobs: []goal.JobFact{}, Runs: []goal.RunFact{}, RunUnreadable: []string{},
		},
		Work: goal.TurnWorkFacts{
			ReadSucceeded: true, Claimed: []goal.GoalFacts{{Id: "held", Intent: strings.Repeat("é", 160) + "\ncontrolled", NextStep: "continue it", Revision: "4"}},
			Landing: []goal.GoalFacts{}, Claimable: []goal.GoalFacts{}, Selected: &goal.GoalFacts{Id: "held", Intent: strings.Repeat("é", 160) + "\ncontrolled", NextStep: "continue it", Revision: "4"},
			Selection: "held", Refused: []goal.AdmissionRefusal{}, InFlight: []string{}, NonTerminalJobs: []string{},
		},
		Ownership: goal.TurnOwnershipFacts{State: "owned", GoalId: "held", Evidence: "joined holder evidence"},
		Actions:   []goal.TurnAction{{Kind: "continue-goal", TargetId: "held", Instruction: "continue it", Command: "metasystem run watch --id run-1 --root " + root, Owner: "seat"}},
		Refusal:   goal.TurnRefusalFacts{Class: control.Class, BlockSource: source, Occurrence: 1, CountSpent: true, IdleRefusal: true},
	}
	healthVerdict := steward.HealthVerdict{Schema: 1, ObservedAt: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), Aggregate: "healthy", Roles: []steward.RoleVerdict{{Role: steward.RoleStewardRunner, Status: steward.HealthAlive, Reason: "runner alive"}}}
	health := steward.NewHookHealthPreview(healthVerdict)
	digestText := "complete digest line\n"
	digestHash := sha256.Sum256([]byte(digestText))
	return StopPresentationInput{
		SchemaVersion: StopPresentationSchemaVersion,
		Identity: StopIdentity{Installation: root, Runtime: "claude", Session: session, SessionKey: StopSessionKey("claude", session), Attempt: attempt,
			MainId: "main-->one", Machine: "bed-m1", Lineage: "lineage-1", ObservedAt: observed, ClaimEpoch: 7},
		Judgment: judgment, Control: control,
		CompletionObservation: &StopCompletionObservation{
			SchemaVersion: StopCompletionObservationSchemaVersion,
			Identity:      StopCompletionIdentity{Installation: root, Session: session, MainId: "main-->one"},
			CollectedAt:   observed, Records: []StopCompletionRecord{}, Unavailable: []string{},
		},
		Health:  &health,
		Digest:  &StopDigest{Mode: "pending", Text: digestText, SourcePath: "records/narrator-digest.log", CursorPrefix: strings.Repeat("a", 64), SHA256: fmt.Sprintf("%x", digestHash)},
		Receipt: &StopCommandResult{ExitCode: 0, Stdout: "receipt current\n"},
		Arming:  &StopArmingResult{ExitCode: 0, Stdout: "up healthy\n", Components: []StopArmingComponent{}, Aggregate: "up healthy"},
		Notices: []StopNotice{}, Unavailable: []StopUnavailable{},
	}
}

func writeStopInput(t *testing.T, path string, input StopPresentationInput) {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

const stopTaskNameDatedIntent = `Wido, 2026-09-06: 'the stop message is still insanely long.' The Stop hook's refusal text has no bound: on m1's first Stop after c1525b90a (the refusal carries the turn verdict) it printed about two hundred run records from August ('no continuation recorded') and one actionable line (an unwatched job) in a single refusal. The Telegram ask got its bound last night (renderQuestion, 1600 runes, the token first, the rest trimmed with a notice); the hook's refusal and its systemMessage get the same discipline. DONE means: the refusal reads, in order, the verdict in one line, the actionable items (each with the command that clears it), then at most a few lines of everything else summarized by class and count ('244 runs without a recorded continuation, oldest 2026-08-16; full list: <path>'), the whole thing bounded to roughly a screen; the full unbounded text is written to a file under the checkout's supervision evidence and the refusal names it. Not a change to what is judged - only to what is printed.`

func setStopSelectedGoal(input *StopPresentationInput, selection, id, intent string) {
	fact := goal.GoalFacts{Id: id, Intent: intent, NextStep: "continue the named work", Revision: "5"}
	input.Judgment.Work.Selected = &fact
	input.Judgment.Work.Selection = selection
	input.Judgment.Work.Claimed = []goal.GoalFacts{}
	input.Judgment.Work.Claimable = []goal.GoalFacts{}
	if selection == "held" {
		input.Judgment.Work.Claimed = []goal.GoalFacts{fact}
		input.Judgment.Ownership = goal.TurnOwnershipFacts{State: "owned", GoalId: id, Evidence: "joined holder evidence"}
	} else {
		input.Judgment.Work.Claimable = []goal.GoalFacts{fact}
		input.Judgment.Ownership = goal.TurnOwnershipFacts{State: "none", Evidence: "no held claim"}
	}
}

func presentStopTaskName(t *testing.T, input StopPresentationInput, wantTask, wantOutcome string, wantFullTask ...string) (StopPresentationResult, string) {
	t.Helper()
	work := t.TempDir()
	inputPath := filepath.Join(work, "input.json")
	outputPath := filepath.Join(work, "output.json")
	writeStopInput(t, inputPath, input)
	result, err := PresentStop(input.Identity.Installation, inputPath, outputPath, time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	reportSuffix := "; Report: "
	if input.Control.ShouldBlock {
		reportSuffix = "; Do not stop. Run this command; read and act on its report: "
	}
	wantLine := "Just completed: unknown for this turn.\n" + wantTask + "; " + wantOutcome + reportSuffix + "metasystem report stop-status --id " + input.Identity.Attempt[:1]
	if result.HumanLine != wantLine {
		t.Fatalf("human line = %q, want %q", result.HumanLine, wantLine)
	}
	if err := ValidateStopHumanLine(result.HumanLine); err != nil {
		t.Fatalf("human line violates its wire contract: bytes=%d %q", len(result.HumanLine), result.HumanLine)
	}
	if result.Control.ShouldBlock != input.Control.ShouldBlock || !sameStringPointer(result.Control.BlockSource, input.Control.BlockSource) {
		t.Fatalf("presentation changed Stop control: got=%+v want=%+v", result.Control, input.Control)
	}
	reportBytes, identity, err := ReadStopStatus(input.Identity.Installation, result.Report.Id)
	if err != nil {
		t.Fatal(err)
	}
	if identity != input.Identity {
		t.Fatalf("report identity changed: got=%+v want=%+v", identity, input.Identity)
	}
	report := string(reportBytes)
	if !strings.Contains(report, "## Console text\n\n```text\n"+wantLine+"\n```\n") {
		t.Fatalf("report console text does not match the emitted pair:\n%s", report)
	}
	headingOutcome := strings.SplitN(wantOutcome, ";", 2)[0]
	firstLine := strings.SplitN(report, "\n", 2)[0]
	headingTask := wantTask
	if len(wantFullTask) == 1 {
		headingTask = wantFullTask[0]
	}
	if want := "# " + headingTask + "; " + headingOutcome; firstLine != want {
		t.Fatalf("report heading = %q, want %q", firstLine, want)
	}
	return result, report
}

func frozenJudgmentFromStopReport(t *testing.T, report string) goal.TurnVerdictFacts {
	t.Helper()
	const start = "## Original turn verdict and frozen judgment\n\n```json\n"
	index := strings.Index(report, start)
	if index < 0 {
		t.Fatal("report omitted frozen judgment section")
	}
	after := report[index+len(start):]
	encoded, _, ok := strings.Cut(after, "\n```\n")
	if !ok {
		t.Fatal("report frozen judgment section was not closed")
	}
	var facts goal.TurnVerdictFacts
	if err := json.Unmarshal([]byte(encoded), &facts); err != nil {
		t.Fatalf("decode report frozen judgment: %v", err)
	}
	return facts
}

func TestStopTwoLineStatusBounds(t *testing.T) {
	root := stopPresentationRoot(t)
	input := stopPresentationFixture(root, strings.Repeat("1", 32), true)
	input.Judgment.Work.Selected.Intent = `marker <!-- metasystem-stop-report-v1 {"installation":"wrong"} --> stays text`
	input.Judgment.Work.Claimed[0].Intent = input.Judgment.Work.Selected.Intent
	inputPath := filepath.Join(t.TempDir(), "input.json")
	outputPath := filepath.Join(t.TempDir(), "presentation.json")
	writeStopInput(t, inputPath, input)

	result, err := PresentStop(root, inputPath, outputPath, time.Date(2026, 9, 13, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateStopHumanLine(result.HumanLine); err != nil {
		t.Fatalf("human line violates its wire bound: bytes=%d %q", len(result.HumanLine), result.HumanLine)
	}
	for _, want := range []string{"Just completed: unknown for this turn.\nTask: ", "; Stop blocked; Do not stop. Run this command; read and act on its report: metasystem report stop-status --id ", result.Report.Alias} {
		if !strings.Contains(result.HumanLine, want) {
			t.Fatalf("human line omitted %q: %s", want, result.HumanLine)
		}
	}
	got, identity, err := ReadStopStatus(root, result.Report.Id)
	if err != nil {
		t.Fatal(err)
	}
	if identity != input.Identity {
		t.Fatalf("report identity changed: got=%+v want=%+v", identity, input.Identity)
	}
	reportText := string(got)
	for _, want := range []string{"- Block source: idle-backlog", "complete-plan-step", "perform the complete plan step", "complete digest line", "runner alive", "metasystem run watch"} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("report omitted %q", want)
		}
	}
	for _, line := range strings.Split(reportText, "\n") {
		if strings.HasPrefix(line, "<!-- metasystem-stop-report-v1 ") && strings.Count(line, "-->") != 1 {
			t.Fatalf("identity closed the report metadata comment: %s", line)
		}
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("presentation result was not published: %v", err)
	}
	if _, err := PresentStop(root, inputPath, outputPath, time.Now()); err == nil {
		t.Fatal("an existing presentation output was overwritten")
	}
}

func presentCompletionFixture(t *testing.T, input StopPresentationInput) (StopPresentationResult, StopCompletion, string) {
	t.Helper()
	work := t.TempDir()
	inputPath := filepath.Join(work, "input.json")
	outputPath := filepath.Join(work, "presentation.json")
	writeStopInput(t, inputPath, input)
	result, err := PresentStop(input.Identity.Installation, inputPath, outputPath, time.Date(2026, 9, 14, 12, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	reportBytes, _, err := ReadStopStatus(input.Identity.Installation, result.Report.Alias)
	if err != nil {
		t.Fatal(err)
	}
	var completion StopCompletion
	if err := decodeStopReportSection(reportBytes, "Completion", &completion); err != nil {
		t.Fatal(err)
	}
	return result, completion, string(reportBytes)
}

func stopCompletionInput(root, attempt, observed string, records []StopCompletionRecord) StopPresentationInput {
	input := stopPresentationFixture(root, attempt, false)
	input.Identity.ObservedAt = observed
	input.Judgment.Identity.ObservedAt = observed
	input.CompletionObservation.CollectedAt = observed
	input.CompletionObservation.Records = records
	return input
}

func ownedCompletionRecord(kind, id, status string) StopCompletionRecord {
	record := StopCompletionRecord{
		Kind: kind, Id: id, GoalId: "review-widget", MainId: "main-->one", Machine: "bed-m1",
		Lineage: "lineage-1", ClaimEpoch: 7, Status: status, StartedAt: "2026-09-14T11:00:00Z",
		Title: "Validation run", Role: "implementer", SourcePath: "artifacts/agents/" + kind + "s/" + id + ".json",
		SourceDigest: strings.Repeat("d", 64), Ownership: "owned",
	}
	if kind == "job" {
		record.OperationId = "operation-" + id
	}
	return record
}

func TestStopCompletionUsesNewOwnedTerminalRecords(t *testing.T) {
	root := stopPresentationRoot(t)
	active := ownedCompletionRecord("job", "active-job", "running")
	baselineInput := stopCompletionInput(root, strings.Repeat("a", 32), "2026-09-14T12:00:00Z", []StopCompletionRecord{active})
	baselineResult, baselineCompletion, _ := presentCompletionFixture(t, baselineInput)
	if baselineCompletion.State != "unknown" || baselineResult.HumanLine != "Just completed: unknown for this turn.\nTask: held; Stop allowed; Report: metasystem report stop-status --id a" {
		t.Fatalf("first-use completion = %+v; line=%q", baselineCompletion, baselineResult.HumanLine)
	}

	returned := active
	returned.Status = "completed"
	returned.EndedAt = "2026-09-14T12:04:00Z"
	earlier := ownedCompletionRecord("job", "earlier-job", "completed")
	earlier.EndedAt = "2026-09-14T12:03:00Z"
	passed := ownedCompletionRecord("run", "passed-run", "green")
	passed.GoalId = "validation-run"
	passed.Generation = 2
	passed.Nonce = strings.Repeat("e", 32)
	passed.TerminalSeq = 9
	currentInput := stopCompletionInput(root, strings.Repeat("b", 32), "2026-09-14T12:05:00Z", []StopCompletionRecord{returned, earlier, passed})
	currentResult, completion, reportText := presentCompletionFixture(t, currentInput)
	if currentResult.HumanLine != "Just completed: review widget (delegate returned).\nTask: held; Stop allowed; Report: metasystem report stop-status --id b" {
		t.Fatalf("job completion line = %q", currentResult.HumanLine)
	}
	if completion.State != "observed" || completion.BaselineReportId != baselineResult.Report.Id || completion.Selected == nil || completion.Selected.Id != "active-job" || len(completion.Events) != 3 {
		t.Fatalf("derived completion = %+v", completion)
	}
	for _, want := range []string{"operation-active-job", "operation-earlier-job", "passed-run", "artifacts/agents/jobs/active-job.json", baselineResult.Report.Id} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("completion report omitted %q", want)
		}
	}

	repeatedInput := stopCompletionInput(root, strings.Repeat("c", 32), "2026-09-14T12:06:00Z", []StopCompletionRecord{returned, earlier, passed})
	repeatedResult, repeated, _ := presentCompletionFixture(t, repeatedInput)
	if repeated.State != "none" || repeatedResult.HumanLine != "Just completed: none recorded this turn.\nTask: held; Stop allowed; Report: metasystem report stop-status --id c" {
		t.Fatalf("repeated completion = %+v; line=%q", repeated, repeatedResult.HumanLine)
	}

	runRoot := stopPresentationRoot(t)
	runBaseline := stopCompletionInput(runRoot, strings.Repeat("d", 32), "2026-09-14T12:00:00Z", []StopCompletionRecord{})
	presentCompletionFixture(t, runBaseline)
	runInput := stopCompletionInput(runRoot, strings.Repeat("e", 32), "2026-09-14T12:02:00Z", []StopCompletionRecord{passed})
	runResult, runCompletion, _ := presentCompletionFixture(t, runInput)
	if runCompletion.State != "observed" || runCompletion.Selected == nil || runCompletion.Selected.Kind != "run" ||
		!strings.HasPrefix(runResult.HumanLine, "Just completed: validation run (run passed).\n") {
		t.Fatalf("run completion = %+v; line=%q", runCompletion, runResult.HumanLine)
	}
}

func TestStopCompletionRejectsUnprovedAttribution(t *testing.T) {
	t.Run("missing baseline", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopCompletionInput(root, strings.Repeat("1", 32), "2026-09-14T12:00:00Z", []StopCompletionRecord{})
		result, completion, _ := presentCompletionFixture(t, input)
		if completion.State != "unknown" || !strings.HasPrefix(result.HumanLine, "Just completed: unknown for this turn.\n") {
			t.Fatalf("first-use completion = %+v; line=%q", completion, result.HumanLine)
		}
	})

	t.Run("unknown ownership", func(t *testing.T) {
		root := stopPresentationRoot(t)
		baseline := stopCompletionInput(root, strings.Repeat("2", 32), "2026-09-14T12:00:00Z", []StopCompletionRecord{})
		presentCompletionFixture(t, baseline)
		record := ownedCompletionRecord("job", "unproved", "completed")
		record.MainId = ""
		record.Ownership = "unknown"
		record.EndedAt = "2026-09-14T12:01:00Z"
		input := stopCompletionInput(root, strings.Repeat("3", 32), "2026-09-14T12:02:00Z", []StopCompletionRecord{record})
		result, completion, _ := presentCompletionFixture(t, input)
		if completion.State != "unknown" || !strings.HasPrefix(result.HumanLine, "Just completed: unknown for this turn.\n") {
			t.Fatalf("unproved completion = %+v; line=%q", completion, result.HumanLine)
		}
	})

	t.Run("other seat and failed work", func(t *testing.T) {
		root := stopPresentationRoot(t)
		baseline := stopCompletionInput(root, strings.Repeat("4", 32), "2026-09-14T12:00:00Z", []StopCompletionRecord{})
		presentCompletionFixture(t, baseline)
		other := ownedCompletionRecord("job", "foreign", "completed")
		other.MainId = "foreign-main"
		other.Lineage = "foreign-lineage"
		other.Ownership = "other"
		other.EndedAt = "2026-09-14T12:01:00Z"
		failed := ownedCompletionRecord("job", "failed", "failed")
		failed.EndedAt = "2026-09-14T12:01:30Z"
		input := stopCompletionInput(root, strings.Repeat("5", 32), "2026-09-14T12:02:00Z", []StopCompletionRecord{other, failed})
		result, completion, _ := presentCompletionFixture(t, input)
		if completion.State != "none" || !strings.HasPrefix(result.HumanLine, "Just completed: none recorded this turn.\n") || strings.Contains(result.HumanLine, "foreign") {
			t.Fatalf("foreign or failed work supplied completion = %+v; line=%q", completion, result.HumanLine)
		}
	})

	t.Run("unreadable newer report", func(t *testing.T) {
		root := stopPresentationRoot(t)
		baseline := stopCompletionInput(root, strings.Repeat("6", 32), "2026-09-14T12:00:00Z", []StopCompletionRecord{})
		presentCompletionFixture(t, baseline)
		corrupt := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", baseline.Identity.SessionKey+"-"+strings.Repeat("7", 32)+".md")
		if err := os.WriteFile(corrupt, []byte("corrupt newer report\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		input := stopCompletionInput(root, strings.Repeat("8", 32), "2026-09-14T12:02:00Z", []StopCompletionRecord{})
		result, completion, _ := presentCompletionFixture(t, input)
		if completion.State != "unknown" || !strings.Contains(strings.Join(completion.Unavailable, " "), "unreadable") || !strings.HasPrefix(result.HumanLine, "Just completed: unknown for this turn.\n") {
			t.Fatalf("unreadable baseline completion = %+v; line=%q", completion, result.HumanLine)
		}
	})

	t.Run("only newest holder identity controls the baseline", func(t *testing.T) {
		root := stopPresentationRoot(t)
		oldHolder := stopCompletionInput(root, strings.Repeat("9", 32), "2026-09-14T11:00:00Z", []StopCompletionRecord{})
		oldHolder.Identity.ClaimEpoch = 6
		presentCompletionFixture(t, oldHolder)

		currentHolder := stopCompletionInput(root, strings.Repeat("a", 32), "2026-09-14T12:00:00Z", []StopCompletionRecord{})
		presentCompletionFixture(t, currentHolder)
		next := stopCompletionInput(root, strings.Repeat("b", 32), "2026-09-14T12:01:00Z", []StopCompletionRecord{})
		result, completion, _ := presentCompletionFixture(t, next)
		if completion.State != "none" || !strings.HasPrefix(result.HumanLine, "Just completed: none recorded this turn.\n") {
			t.Fatalf("older holder poisoned a newer compatible baseline: %+v; line=%q", completion, result.HumanLine)
		}

		changedHolder := stopCompletionInput(root, strings.Repeat("c", 32), "2026-09-14T12:02:00Z", []StopCompletionRecord{})
		changedHolder.Identity.ClaimEpoch = 8
		changedResult, changed, _ := presentCompletionFixture(t, changedHolder)
		if changed.State != "unknown" || !strings.Contains(strings.Join(changed.Unavailable, " "), "holder identity") ||
			!strings.HasPrefix(changedResult.HumanLine, "Just completed: unknown for this turn.\n") {
			t.Fatalf("newest incompatible holder supplied a baseline: %+v; line=%q", changed, changedResult.HumanLine)
		}
	})
}

func TestStopTaskNamesUseGoalIDsInsteadOfIntent(t *testing.T) {
	tests := []struct {
		name, selection, id, intent, wantTask, wantOutcome string
		block                                              bool
	}{
		{"held dated intent", "held", "stop-refusal-fits-on-one-screen", stopTaskNameDatedIntent, "Task: stop refusal fits on one screen", "Stop blocked", true},
		{"claimable dated intent", "claimable", "stop-refusal-fits-on-one-screen", stopTaskNameDatedIntent, "No task in flight; next: stop refusal fits on one screen", "Stop allowed", false},
		{"empty intent", "held", "stop-refusal-fits-on-one-screen", "", "Task: stop refusal fits on one screen", "Stop blocked", true},
		{"short intent", "held", "stop-refusal-fits-on-one-screen", "short explanation", "Task: stop refusal fits on one screen", "Stop blocked", true},
		{"different slug", "held", "report-heading-stays-readable", stopTaskNameDatedIntent, "Task: report heading stays readable", "Stop blocked", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := stopPresentationRoot(t)
			input := stopPresentationFixture(root, strings.Repeat("1", 32), test.block)
			setStopSelectedGoal(&input, test.selection, test.id, test.intent)
			_, report := presentStopTaskName(t, input, test.wantTask, test.wantOutcome)
			firstLine := strings.SplitN(report, "\n", 2)[0]
			if strings.Contains(firstLine, "Wido, 2026-09-06") || strings.Contains(firstLine, "insanely long") {
				t.Fatalf("intent escaped into the task heading: %s", firstLine)
			}
			facts := frozenJudgmentFromStopReport(t, report)
			if facts.Work.Selected == nil || facts.Work.Selected.Intent != test.intent {
				t.Fatalf("complete intent did not survive in frozen report facts: %+v", facts.Work.Selected)
			}
			if !strings.Contains(report, "used goal ID "+test.id) {
				t.Fatalf("report summary omitted the chosen goal-ID source:\n%s", report)
			}
		})
	}
}

func TestStopTaskNamesForOwnedJobsAndRuns(t *testing.T) {
	tests := []struct {
		name, wantTask, summary string
		job                     *goal.JobFact
		run                     *goal.RunFact
	}{
		{
			name: "job goal ID", wantTask: "Task: stop refusal fits on one screen", summary: "used goal ID stop-refusal-fits-on-one-screen",
			job: &goal.JobFact{Id: "job-goal", StartedAt: "2026-09-14T08:00:00Z", Status: "running", Ownership: "owned", GoalId: "stop-refusal-fits-on-one-screen", Role: "design-review", Title: stopTaskNameDatedIntent, SourcePath: "artifacts/agents/jobs/job-goal.json"},
		},
		{
			name: "run goal ID", wantTask: "Task: stop refusal fits on one screen", summary: "used goal ID stop-refusal-fits-on-one-screen",
			run: &goal.RunFact{Id: "run-goal", StartedAt: "2026-09-14T08:00:00Z", Status: "running", Ownership: "owned", GoalId: "stop-refusal-fits-on-one-screen", Title: "A conflicting prose display. It must stay detail.", SourcePath: "artifacts/agents/runs/run-goal.json"},
		},
		{
			name: "job role fallback", wantTask: "Task: design review", summary: "goal ID was unusable; used role design-review",
			job: &goal.JobFact{Id: "job-role", Status: "pending", Ownership: "owned", Role: "design-review", Title: strings.Repeat("long prose title ", 20), SourcePath: "artifacts/agents/jobs/job-role.json"},
		},
		{
			name: "run display fallback", wantTask: "Task: Stop report checks", summary: "goal ID was unusable; used run display Stop report checks",
			run: &goal.RunFact{Id: "run-display", Status: "launching", Ownership: "owned", Title: "Stop report checks", SourcePath: "artifacts/agents/runs/run-display.json"},
		},
		{
			name: "quoted run prose rejected", wantTask: "Task: task name unavailable", summary: "goal ID and run display were unusable",
			run: &goal.RunFact{Id: "run-quoted", Status: "draining", Ownership: "owned", Title: `Wido, 2026-09-06: "fix the Stop line"`, SourcePath: "artifacts/agents/runs/run-quoted.json"},
		},
		{
			name: "multi sentence run prose rejected", wantTask: "Task: task name unavailable", summary: "goal ID and run display were unusable",
			run: &goal.RunFact{Id: "run-sentences", Status: "running", Ownership: "owned", Title: "First sentence. Second sentence.", SourcePath: "artifacts/agents/runs/run-sentences.json"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := stopPresentationRoot(t)
			input := stopPresentationFixture(root, strings.Repeat("2", 32), true)
			if test.job != nil {
				input.Judgment.Scan.Jobs = []goal.JobFact{*test.job}
				input.Judgment.Work.NonTerminalJobs = []string{test.job.Id}
			}
			if test.run != nil {
				input.Judgment.Scan.Runs = []goal.RunFact{*test.run}
			}
			_, report := presentStopTaskName(t, input, test.wantTask, "Stop blocked")
			if !strings.Contains(report, test.summary) {
				t.Fatalf("report summary omitted %q:\n%s", test.summary, report)
			}
			facts := frozenJudgmentFromStopReport(t, report)
			if test.job != nil && (len(facts.Scan.Jobs) != 1 || facts.Scan.Jobs[0].Title != test.job.Title || facts.Scan.Jobs[0].SourcePath != test.job.SourcePath) {
				t.Fatalf("job source facts changed in report: %+v", facts.Scan.Jobs)
			}
			if test.run != nil && (len(facts.Scan.Runs) != 1 || facts.Scan.Runs[0].Title != test.run.Title || facts.Scan.Runs[0].SourcePath != test.run.SourcePath) {
				t.Fatalf("run source facts changed in report: %+v", facts.Scan.Runs)
			}
		})
	}
}

func TestStopTaskNamePrecedence(t *testing.T) {
	t.Run("tier order ignores unowned and terminal records", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("3", 32), true)
		setStopSelectedGoal(&input, "held", "held-goal", "held detail")
		input.Judgment.Work.Claimable = []goal.GoalFacts{{Id: "claimable-goal", Intent: "claimable detail"}}
		input.Judgment.Scan.Jobs = []goal.JobFact{
			{Id: "other-job", StartedAt: "2026-09-01T00:00:00Z", Status: "running", Ownership: "other", GoalId: "other-goal"},
			{Id: "unknown-job", StartedAt: "2026-09-01T00:00:00Z", Status: "running", Ownership: "unknown", GoalId: "unknown-goal"},
			{Id: "terminal-job", StartedAt: "2026-09-01T00:00:00Z", Status: "completed", Ownership: "owned", GoalId: "terminal-goal"},
			{Id: "owned-job", StartedAt: "2026-09-03T00:00:00Z", Status: "running", Ownership: "owned", GoalId: "job-goal"},
		}
		input.Judgment.Scan.Runs = []goal.RunFact{
			{Id: "terminal-run", StartedAt: "2026-09-01T00:00:00Z", Status: "green", Ownership: "owned", GoalId: "terminal-run-goal"},
			{Id: "owned-run", StartedAt: "2026-09-02T00:00:00Z", Status: "running", Ownership: "owned", GoalId: "run-goal"},
		}
		input.Judgment.Work.NonTerminalJobs = []string{"other-job", "unknown-job", "owned-job"}
		_, report := presentStopTaskName(t, input, "Task: job goal", "Stop blocked")
		for _, want := range []string{"other-job", "unknown-job", "terminal-job", "owned-job", "terminal-run", "owned-run", "Claimable goals: 1", "non-terminal jobs: 3"} {
			if !strings.Contains(report, want) {
				t.Fatalf("precedence report omitted %q", want)
			}
		}
	})

	t.Run("run precedes held and selected claimable", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("4", 32), true)
		setStopSelectedGoal(&input, "held", "held-goal", "held detail")
		input.Judgment.Work.Claimable = []goal.GoalFacts{{Id: "claimable-goal"}}
		input.Judgment.Scan.Runs = []goal.RunFact{{Id: "owned-run", Status: "running", Ownership: "owned", GoalId: "run-goal"}}
		presentStopTaskName(t, input, "Task: run goal", "Stop blocked")
	})

	t.Run("held precedes claimable", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("5", 32), true)
		setStopSelectedGoal(&input, "held", "held-goal", "held detail")
		input.Judgment.Work.Claimable = []goal.GoalFacts{{Id: "claimable-goal"}}
		presentStopTaskName(t, input, "Task: held goal", "Stop blocked")
	})

	t.Run("claimable is labelled next", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("6", 32), false)
		setStopSelectedGoal(&input, "claimable", "claimable-goal", "claimable detail")
		presentStopTaskName(t, input, "No task in flight; next: claimable goal", "Stop allowed")
	})

	t.Run("earliest date wins despite reverse order and missing dates", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("7", 32), true)
		input.Judgment.Scan.Jobs = []goal.JobFact{
			{Id: "missing", Status: "running", Ownership: "owned", GoalId: "missing-date"},
			{Id: "later", StartedAt: "2026-09-14T09:00:00Z", Status: "running", Ownership: "owned", GoalId: "later-date"},
			{Id: "earlier", StartedAt: "2026-09-14T08:00:00Z", Status: "running", Ownership: "owned", GoalId: "earlier-date"},
		}
		presentStopTaskName(t, input, "Task: earlier date", "Stop blocked")
	})

	t.Run("equal dates use record ID", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("8", 32), true)
		input.Judgment.Scan.Runs = []goal.RunFact{
			{Id: "z-run", StartedAt: "2026-09-14T08:00:00Z", Status: "running", Ownership: "owned", GoalId: "zulu-goal"},
			{Id: "a-run", StartedAt: "2026-09-14T08:00:00Z", Status: "running", Ownership: "owned", GoalId: "alpha-goal"},
		}
		presentStopTaskName(t, input, "Task: alpha goal", "Stop blocked")
	})

	t.Run("winning unnamed job does not fall through", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("9", 32), true)
		input.Judgment.Scan.Jobs = []goal.JobFact{{Id: "unnamed", Status: "running", Ownership: "owned", GoalId: "550e8400-e29b-41d4-a716-446655440000", Role: "Review."}}
		input.Judgment.Scan.Runs = []goal.RunFact{{Id: "named-run", Status: "running", Ownership: "owned", GoalId: "named-run-goal"}}
		_, report := presentStopTaskName(t, input, "Task: task name unavailable", "Stop blocked")
		if !strings.Contains(report, "goal ID and role were unusable") || !strings.Contains(report, "named-run") {
			t.Fatalf("unnamed winning job or lower-tier report evidence was lost:\n%s", report)
		}
	})

	t.Run("winning unnamed run does not fall through", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("0", 32), true)
		setStopSelectedGoal(&input, "held", "named-held-goal", "held detail")
		input.Judgment.Scan.Runs = []goal.RunFact{{Id: "unnamed-run", Status: "running", Ownership: "owned", GoalId: strings.Repeat("d", 40), Title: "A dated run, 2026-09-06."}}
		_, report := presentStopTaskName(t, input, "Task: task name unavailable", "Stop blocked")
		if !strings.Contains(report, "goal ID and run display were unusable") || !strings.Contains(report, "named-held-goal") {
			t.Fatalf("unnamed winning run or lower-tier report evidence was lost:\n%s", report)
		}
	})

	t.Run("empty unknown and stopped states remain distinct", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("a", 32), false)
		input.Judgment.Work.Claimed = []goal.GoalFacts{}
		input.Judgment.Work.Claimable = []goal.GoalFacts{}
		input.Judgment.Work.Selected = nil
		input.Judgment.Work.Selection = "none"
		input.Judgment.Ownership = goal.TurnOwnershipFacts{State: "none"}
		presentStopTaskName(t, input, "No task in flight", "Stop allowed")

		root = stopPresentationRoot(t)
		input = stopPresentationFixture(root, strings.Repeat("b", 32), true)
		input.Judgment.Work.ReadSucceeded = false
		input.Judgment.Work.Selected = nil
		input.Judgment.Work.Selection = "unknown"
		input.Judgment.Ownership = goal.TurnOwnershipFacts{State: "unknown"}
		presentStopTaskName(t, input, "Task unknown", "Stop blocked")

		root = stopPresentationRoot(t)
		input = stopPresentationFixture(root, strings.Repeat("c", 32), false)
		input.Judgment.Verdict.LedgerStatus = "stopped"
		input.Judgment.Work.ReadSucceeded = false
		input.Judgment.Work.Selected = nil
		input.Judgment.Work.Selection = "unknown"
		input.Judgment.Ownership = goal.TurnOwnershipFacts{State: "unknown"}
		presentStopTaskName(t, input, "No task in flight", "Stop allowed")
	})
}

func TestStopTaskNameFallbacks(t *testing.T) {
	invalidGoalIDs := []struct {
		name, id string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"numeric", "123456"},
		{"punctuation", "---"},
		{"malformed uppercase", "Not-kebab"},
		{"malformed underscore", "not_kebab"},
		{"overlong", strings.Repeat("a", 101)},
		{"UUID", "550e8400-e29b-41d4-a716-446655440000"},
		{"hex 32", strings.Repeat("a", 32)},
		{"hex 40", strings.Repeat("b", 40)},
		{"hex 64", strings.Repeat("c", 64)},
	}
	for _, test := range invalidGoalIDs {
		t.Run("goal "+test.name, func(t *testing.T) {
			root := stopPresentationRoot(t)
			input := stopPresentationFixture(root, strings.Repeat("d", 32), true)
			setStopSelectedGoal(&input, "held", test.id, stopTaskNameDatedIntent)
			_, report := presentStopTaskName(t, input, "Task: task name unavailable", "Stop blocked")
			if !strings.Contains(report, "goal ID was unusable") {
				t.Fatalf("report did not identify the rejected goal-ID source:\n%s", report)
			}
		})
	}

	t.Run("invalid job role does not use prose title", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("e", 32), true)
		input.Judgment.Scan.Jobs = []goal.JobFact{{Id: "job-bad-role", Status: "running", Ownership: "owned", Role: "Design review.", Title: "Short lawful title", SourcePath: "jobs/job-bad-role.json"}}
		presentStopTaskName(t, input, "Task: task name unavailable", "Stop blocked")
	})

	t.Run("short dated run prose is rejected whole", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("f", 32), true)
		input.Judgment.Scan.Runs = []goal.RunFact{{Id: "run-dated", Status: "running", Ownership: "owned", Title: `Wido, 2026-09-06 "Stop report"`}}
		presentStopTaskName(t, input, "Task: task name unavailable", "Stop blocked")
	})

	t.Run("long unpunctuated run prose is not clipped into eligibility", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("0", 32), true)
		input.Judgment.Scan.Runs = []goal.RunFact{{Id: "run-long", Status: "running", Ownership: "owned", Title: strings.Repeat("a", 101)}}
		presentStopTaskName(t, input, "Task: task name unavailable", "Stop blocked")
	})

	t.Run("missing name preserves control and intervention flags", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("1", 32), true)
		setStopSelectedGoal(&input, "held", "550e8400-e29b-41d4-a716-446655440000", stopTaskNameDatedIntent)
		input.Notices = []StopNotice{{HumanRequired: true, SupervisionRepair: true}}
		result, _ := presentStopTaskName(t, input, "Task: task name unavailable", "Stop blocked; needs your decision and supervision repair")
		if !result.NeedsYourDecision || !result.NeedsSupervisionRepair {
			t.Fatalf("missing name changed intervention flags: %+v", result)
		}
	})

	t.Run("Unicode role and benign separators are usable", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("2", 32), true)
		input.Judgment.Scan.Jobs = []goal.JobFact{{Id: "job-unicode", Status: "running", Ownership: "owned", Role: "design_review-équipe/2"}}
		presentStopTaskName(t, input, "Task: design review équipe/2", "Stop blocked")
	})

	t.Run("Unicode run label is usable", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("3", 32), true)
		input.Judgment.Scan.Runs = []goal.RunFact{{Id: "run-unicode", Status: "running", Ownership: "owned", Title: "Révision β2 + QA"}}
		presentStopTaskName(t, input, "Task: Révision β2 + QA", "Stop blocked")
	})
}

func TestStopReadImperativeBounds(t *testing.T) {
	const longSlug = "alpha-alpha-alpha-alpha-alpha-alpha-alpha-alpha-alpha-alpha-alpha-alpha-alpha-alpha-alpha-alpha-beta"
	const fullGoalName = "alpha alpha alpha alpha alpha alpha alpha alpha alpha alpha alpha alpha alpha alpha alpha alpha beta"
	tests := []struct {
		name, selection, wantTask, wantOutcome string
		block, human, repair                   bool
	}{
		{"held block without intervention", "held", "Task: " + fullGoalName, "Stop blocked", true, false, false},
		{"next allowance with decision", "claimable", "No task in flight; next: " + fullGoalName, "Stop allowed; needs your decision", false, true, false},
		{"held allowance with repair", "held", "Task: " + fullGoalName, "Stop allowed; needs supervision repair", false, false, true},
		{"next block with both", "claimable", "No task in flight; next: alpha alpha alpha alpha alpha alpha alpha alpha alpha alpha", "Stop blocked; needs your decision and supervision repair", true, true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := stopPresentationRoot(t)
			input := stopPresentationFixture(root, strings.Repeat("4", 32), test.block)
			setStopSelectedGoal(&input, test.selection, longSlug, "goal detail remains complete")
			input.Notices = []StopNotice{{HumanRequired: test.human, SupervisionRepair: test.repair}}
			headingTask := "Task: " + fullGoalName
			if test.selection == "claimable" {
				headingTask = "No task in flight; next: " + fullGoalName
			}
			result, report := presentStopTaskName(t, input, test.wantTask, test.wantOutcome, headingTask)
			if first := strings.SplitN(report, "\n", 2)[0]; !strings.Contains(first, fullGoalName) {
				t.Fatalf("report heading did not retain the full accepted name: %s", first)
			}
			reportSuffix := "; Report: "
			if test.block {
				reportSuffix = "; Do not stop. Run this command; read and act on its report: "
			}
			if !strings.HasSuffix(result.HumanLine, "; "+test.wantOutcome+reportSuffix+"metasystem report stop-status --id "+result.Report.Alias) {
				t.Fatalf("reserved Stop suffix changed: %s", result.HumanLine)
			}
		})
	}

	t.Run("Unicode run cut stays on a UTF-8 boundary", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("5", 32), true)
		input.Judgment.Scan.Runs = []goal.RunFact{{Id: "run-boundary", Status: "running", Ownership: "owned", Title: strings.Repeat("é", 50)}}
		input.Notices = []StopNotice{{HumanRequired: true}}
		result, report := presentStopTaskName(t, input, "Task: "+strings.Repeat("é", 50), "Stop blocked; needs your decision", "Task: "+strings.Repeat("é", 50))
		if err := ValidateStopHumanLine(result.HumanLine); err != nil || !utf8.ValidString(result.HumanLine) {
			t.Fatalf("Unicode cut broke the line: bytes=%d %q", len(result.HumanLine), result.HumanLine)
		}
		if first := strings.SplitN(report, "\n", 2)[0]; first != "# Task: "+strings.Repeat("é", 50)+"; Stop blocked" {
			t.Fatalf("report heading shortened the accepted run label: %s", first)
		}
	})

	t.Run("overlong fallback label is rejected whole", func(t *testing.T) {
		root := stopPresentationRoot(t)
		input := stopPresentationFixture(root, strings.Repeat("6", 32), false)
		input.Judgment.Scan.Runs = []goal.RunFact{{Id: "run-overlong", Status: "running", Ownership: "owned", Title: strings.Repeat("é", 51)}}
		presentStopTaskName(t, input, "Task: task name unavailable", "Stop allowed")
	})

	t.Run("longest-realistic-next-both-flags", func(t *testing.T) {
		const fullName = "preserve stop report instructions across claude codex and devin after compaction and session restart"
		const goalID = "preserve-stop-report-instructions-across-claude-codex-and-devin-after-compaction-and-session-restart"
		const blockedLine = "No task in flight; next: preserve stop report instruct; Stop blocked; needs your decision and supervision repair; Do not stop. Run this command; read and act on its report: metasystem report stop-status --id aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		const allowedLine = "No task in flight; next: preserve stop report instructions across claude codex and devin after compaction; Stop allowed; needs your decision and supervision repair; Report: metasystem report stop-status --id aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		if len(fullName) != 100 || len(blockedLine) != stopTaskLineTargetByteLimit || len(allowedLine) != stopTaskLineTargetByteLimit {
			t.Fatalf("fixture limits changed: name=%d block=%d allow=%d", len(fullName), len(blockedLine), len(allowedLine))
		}
		for _, test := range []struct {
			name, wantLine string
			blocked        bool
		}{
			{name: "block", wantLine: blockedLine, blocked: true},
			{name: "allow", wantLine: allowedLine},
		} {
			t.Run(test.name, func(t *testing.T) {
				root := stopPresentationRoot(t)
				baseline := stopCompletionInput(root, strings.Repeat("b", 32), "2026-09-14T12:00:00Z", []StopCompletionRecord{})
				presentCompletionFixture(t, baseline)
				attempt := strings.Repeat("a", 32)
				aliasDir := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts", "aliases")
				for length := 1; length < len(attempt); length++ {
					if err := os.WriteFile(filepath.Join(aliasDir, attempt[:length]+".json"), []byte("occupied\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				role := strings.Repeat("r", 100)
				completed := ownedCompletionRecord("job", "long-name", "completed")
				completed.GoalId = ""
				completed.Role = role
				completed.EndedAt = "2026-09-14T12:01:00Z"
				input := stopPresentationFixture(root, attempt, test.blocked)
				input.Identity.ObservedAt = "2026-09-14T12:02:00Z"
				input.Judgment.Identity.ObservedAt = input.Identity.ObservedAt
				input.CompletionObservation.CollectedAt = input.Identity.ObservedAt
				input.CompletionObservation.Records = []StopCompletionRecord{completed}
				setStopSelectedGoal(&input, "claimable", goalID, "full heading detail")
				input.Notices = []StopNotice{{HumanRequired: true, SupervisionRepair: true}}
				inputPath := filepath.Join(t.TempDir(), "input.json")
				outputPath := filepath.Join(t.TempDir(), "output.json")
				writeStopInput(t, inputPath, input)
				result, err := PresentStop(root, inputPath, outputPath, time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC))
				if err != nil {
					t.Fatal(err)
				}
				lines := strings.Split(result.HumanLine, "\n")
				if result.Report.Alias != attempt || len(result.Report.ReadCommand) != 67 || len(lines) != 2 || len(lines[0]) != 137 || lines[0] != "Just completed: "+role+" (delegate returned)." || lines[1] != test.wantLine || len(lines[1]) != 240 || len(result.HumanLine) != 378 {
					t.Fatalf("bounded pair: alias=%q command=%d lines=%d line1=%d line2=%d pair=%d text=%q", result.Report.Alias, len(result.Report.ReadCommand), len(lines), len(lines[0]), len(lines[1]), len(result.HumanLine), result.HumanLine)
				}
				if StopTaskLineByteLimit-len(lines[1]) != 16 {
					t.Fatalf("line 2 headroom = %d, want 16", StopTaskLineByteLimit-len(lines[1]))
				}
				t.Logf("emitted %s boundary line 1 (%d bytes): %s\nemitted %s boundary line 2 (%d bytes): %s", test.name, len(lines[0]), lines[0], test.name, len(lines[1]), lines[1])
				reportBytes, _, err := ReadStopStatus(root, result.Report.Alias)
				if err != nil {
					t.Fatal(err)
				}
				wantHeading := "# No task in flight; next: " + fullName
				if first := strings.SplitN(string(reportBytes), "\n", 2)[0]; !strings.HasPrefix(first, wantHeading+"; Stop ") {
					t.Fatalf("report heading shortened the accepted name: %s", first)
				}
			})
		}
	})

	t.Run("maximum-alias-combined-flag-name-budgets", func(t *testing.T) {
		command := "metasystem report stop-status --id " + strings.Repeat("a", 32)
		for _, test := range []struct {
			name, kind string
			blocked    bool
			nameBytes  int
		}{
			{name: "block-held", kind: "task", blocked: true, nameBytes: 48},
			{name: "block-next", kind: "next", blocked: true, nameBytes: 29},
			{name: "allow-held", kind: "task", nameBytes: 99},
			{name: "allow-next", kind: "next", nameBytes: 80},
		} {
			t.Run(test.name, func(t *testing.T) {
				line := compactStopLine(strings.Repeat("g", 100), test.kind, test.blocked, true, true, command)
				prefix := "Task: "
				if test.kind == "next" {
					prefix = "No task in flight; next: "
				}
				if len(line) != 240 || !strings.HasPrefix(line, prefix+strings.Repeat("g", test.nameBytes)+"; Stop ") {
					t.Fatalf("%s name budget: line=%d text=%q", test.name, len(line), line)
				}
			})
		}
	})

	t.Run("validity-cap-remains-256", func(t *testing.T) {
		lineOne := "Just completed: unknown for this turn."
		for _, size := range []int{241, 256} {
			line := lineOne + "\n" + strings.Repeat("x", size)
			if err := ValidateStopHumanLine(line); err != nil {
				t.Fatalf("%d-byte line 2 was rejected below the validity cap: %v", size, err)
			}
		}
		if err := ValidateStopHumanLine(lineOne + "\n" + strings.Repeat("x", 257)); err == nil {
			t.Fatal("257-byte line 2 was accepted above the validity cap")
		}
	})
}

func TestPresentStopReportRetainsInfrastructureControlWithoutFrozenFacts(t *testing.T) {
	root := stopPresentationRoot(t)
	input := stopPresentationFixture(root, strings.Repeat("2", 32), false)
	input.Control = StopControl{Class: "infrastructure", CauseCode: "turn-verdict-state", JudgmentAvailable: true}
	input.Judgment = nil
	input.Unavailable = append(input.Unavailable, StopUnavailable{
		Section: "judgment-facts", Cause: "the frozen judgment facts were unavailable",
		Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true,
	})
	inputPath := filepath.Join(t.TempDir(), "input.json")
	outputPath := filepath.Join(t.TempDir(), "presentation.json")
	writeStopInput(t, inputPath, input)

	result, err := PresentStop(root, inputPath, outputPath, time.Date(2026, 9, 13, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	reportBytes, _, err := ReadStopStatus(root, result.Report.Id)
	if err != nil {
		t.Fatal(err)
	}
	reportText := string(reportBytes)
	for _, want := range []string{
		"## Retained Stop control", `"class": "infrastructure"`, `"causeCode": "turn-verdict-state"`,
		`"judgmentAvailable": true`, `"cause": "the frozen judgment facts were unavailable"`,
	} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("report omitted retained unavailable control %q:\n%s", want, reportText)
		}
	}
}

func TestStopReadImperativeByOutcome(t *testing.T) {
	const blockedSuffix = "; Do not stop. Run this command; read and act on its report: "
	const allowedSuffix = "; Report: "
	if len(blockedSuffix) != 61 || len(allowedSuffix) != 10 {
		t.Fatalf("outcome suffix limits changed: block=%d allow=%d", len(blockedSuffix), len(allowedSuffix))
	}
	for _, test := range []struct {
		name, attempt, want string
		blocked, human      bool
		repair              bool
	}{
		{"block-exact", strings.Repeat("a", 32), "Just completed: unknown for this turn.\nTask: stop refusal fits on one screen; Stop blocked; Do not stop. Run this command; read and act on its report: metasystem report stop-status --id a", true, false, false},
		{"block-decision", strings.Repeat("b", 32), "Just completed: unknown for this turn.\nTask: stop refusal fits on one screen; Stop blocked; needs your decision; Do not stop. Run this command; read and act on its report: metasystem report stop-status --id b", true, true, false},
		{"block-repair", strings.Repeat("c", 32), "Just completed: unknown for this turn.\nTask: stop refusal fits on one screen; Stop blocked; needs supervision repair; Do not stop. Run this command; read and act on its report: metasystem report stop-status --id c", true, false, true},
		{"block-both", strings.Repeat("d", 32), "Just completed: unknown for this turn.\nTask: stop refusal fits on one screen; Stop blocked; needs your decision and supervision repair; Do not stop. Run this command; read and act on its report: metasystem report stop-status --id d", true, true, true},
		{"allow-exact", strings.Repeat("2", 32), "Just completed: unknown for this turn.\nTask: stop refusal fits on one screen; Stop allowed; Report: metasystem report stop-status --id 2", false, false, false},
		{"allow-decision", strings.Repeat("3", 32), "Just completed: unknown for this turn.\nTask: stop refusal fits on one screen; Stop allowed; needs your decision; Report: metasystem report stop-status --id 3", false, true, false},
		{"allow-repair", strings.Repeat("4", 32), "Just completed: unknown for this turn.\nTask: stop refusal fits on one screen; Stop allowed; needs supervision repair; Report: metasystem report stop-status --id 4", false, false, true},
		{"allow-both", strings.Repeat("5", 32), "Just completed: unknown for this turn.\nTask: stop refusal fits on one screen; Stop allowed; needs your decision and supervision repair; Report: metasystem report stop-status --id 5", false, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := stopPresentationRoot(t)
			input := stopPresentationFixture(root, test.attempt, test.blocked)
			setStopSelectedGoal(&input, "held", "stop-refusal-fits-on-one-screen", stopTaskNameDatedIntent)
			input.Notices = []StopNotice{{HumanRequired: test.human, SupervisionRepair: test.repair}}
			human, repair := stopInterventions(input)
			if human != test.human || repair != test.repair {
				t.Fatalf("derived flags = %t,%t", human, repair)
			}
			inputPath := filepath.Join(t.TempDir(), "input.json")
			outputPath := filepath.Join(t.TempDir(), "output.json")
			writeStopInput(t, inputPath, input)
			result, err := PresentStop(root, inputPath, outputPath, time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatal(err)
			}
			if result.HumanLine != test.want {
				t.Fatalf("decoded pair = %q, want %q", result.HumanLine, test.want)
			}
			if test.name == "block-exact" || test.name == "allow-exact" {
				lines := strings.Split(result.HumanLine, "\n")
				t.Logf("emitted %s line 1 (%d bytes): %s\nemitted %s line 2 (%d bytes): %s", test.name, len(lines[0]), lines[0], test.name, len(lines[1]), lines[1])
			}
			reportBytes, _, err := ReadStopStatus(root, result.Report.Alias)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(reportBytes), "## Console text\n\n```text\n"+test.want+"\n```\n") {
				t.Fatalf("report Console text differs from emitted pair:\n%s", reportBytes)
			}
		})
	}

	verdict := steward.HealthVerdict{Roles: []steward.RoleVerdict{
		{Role: steward.RoleRetroDebt, Status: steward.HealthDead, Remedy: "run the retro", NoAutomaticRemedy: true},
		{Role: steward.RoleSpendFence, Status: steward.HealthAlive, Remedy: "raise the ceiling"},
	}}
	preview := steward.NewHookHealthPreview(verdict)
	input := StopPresentationInput{Health: &preview}
	if human, repair := stopInterventions(input); human || repair {
		t.Fatalf("retro debt and spend advice became an intervention: %t,%t", human, repair)
	}
}

func TestParseStopArmingComponentsRetainsTypedFailure(t *testing.T) {
	input := "component=accepted-engine outcome=verified\n" +
		`component=steward-runner outcome=failed detail="ENROLLMENT_DRIFT" remedy="run 'metasystem steward restart' from an agent-free terminal"` + "\n" +
		`up outcome=failed component=steward-runner remedy="repair"`
	components, err := parseStopArmingComponents(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(components) != 2 || components[0].Name != "accepted-engine" || components[0].SupervisionRepair ||
		components[1].Name != "steward-runner" || !components[1].SupervisionRepair ||
		components[1].Detail != "ENROLLMENT_DRIFT" || !strings.Contains(components[1].Remedy, "agent-free terminal") {
		t.Fatalf("parsed arming components = %+v", components)
	}
	if _, err := parseStopArmingComponents("component=broken outcome=failed detail=not-quoted"); err == nil {
		t.Fatal("malformed arming component was accepted")
	}
}

func TestComposeStopPresentationInputPreservesCommandStreams(t *testing.T) {
	root := stopPresentationRoot(t)
	work := t.TempDir()
	write := func(name, value string) string {
		t.Helper()
		path := filepath.Join(work, name)
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	verdictBytes, err := json.Marshal(goal.Verdict{SchemaVersion: 1, Class: "seat-actionable", ShouldBlock: false})
	if err != nil {
		t.Fatal(err)
	}
	verdict := write("verdict.json", string(verdictBytes)+"\n")
	receipt := write("receipt.stdout", "receipt output\n")
	receiptStderr := write("receipt.stderr", "receipt diagnostic\n")
	arming := write("arming.stdout", "component=accepted-engine outcome=verified\nup outcome=healthy\n")
	armingStderr := write("arming.stderr", "arming diagnostic\n")
	output := filepath.Join(root, "presentation-input.json")
	if err := ComposeStopPresentationInput(StopPresentationCollection{
		Root: root, Runtime: "claude", Session: "streams", Attempt: strings.Repeat("a", 32),
		VerdictFile: verdict, ReceiptFile: receipt, ReceiptStderrFile: receiptStderr,
		ReceiptExit: 1, ArmingFile: arming, ArmingStderrFile: armingStderr,
		ArmingExit: 2, OutputFile: output,
	}, time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	input, err := ReadStopPresentationInput(output)
	if err != nil {
		t.Fatal(err)
	}
	if input.Receipt == nil || input.Receipt.ExitCode != 1 || input.Receipt.Stdout != "receipt output\n" || input.Receipt.Stderr != "receipt diagnostic\n" {
		t.Fatalf("receipt streams changed: %+v", input.Receipt)
	}
	if input.Arming == nil || input.Arming.ExitCode != 2 || input.Arming.Stdout != "component=accepted-engine outcome=verified\nup outcome=healthy\n" || input.Arming.Stderr != "arming diagnostic\n" {
		t.Fatalf("arming streams changed: %+v", input.Arming)
	}
}

func TestComposeStopPresentationInputPublishesAdvisorReportWithoutJudgment(t *testing.T) {
	root := stopPresentationRoot(t)
	work := t.TempDir()
	write := func(name, value string) string {
		t.Helper()
		path := filepath.Join(work, name)
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	health := steward.NewHookHealthPreview(steward.HealthVerdict{
		Schema: 1, ObservedAt: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), Aggregate: "healthy",
		Roles: []steward.RoleVerdict{{Role: steward.RoleStewardRunner, Status: steward.HealthAlive, Reason: "runner alive"}},
	})
	healthBytes, err := json.Marshal(health)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(root, "advisor-input.json")
	if err := ComposeStopPresentationInput(StopPresentationCollection{
		Root: root, Runtime: "claude", Session: "advisor", Attempt: strings.Repeat("9", 32),
		MainID: "advisor-main", Advisor: true,
		HealthFile: write("health.json", string(healthBytes)+"\n"),
		DigestFile: write("digest.txt", "advisor digest\n"), DigestCursorPrefix: strings.Repeat("d", 64),
		ReceiptFile: write("receipt.txt", "current\n"), ReceiptStderrFile: write("receipt.err", ""),
		ArmingFile:       write("arming.txt", "component=checkout-lease outcome=advisor\nup outcome=advisor authority=read-only\n"),
		ArmingStderrFile: write("arming.err", ""),
		NoticeFile:       write("notice.txt", "OWNED-ELSEWHERE: use scripts/agents/second-session.sh"),
		FailureFile:      write("failure.txt", ""), OutputFile: inputPath,
	}, time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	input, err := ReadStopPresentationInput(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	if input.Judgment != nil || input.Control.JudgmentAvailable || input.Control.ShouldBlock || input.Control.Class != "advisor" {
		t.Fatalf("advisor control was not judgment-free: %+v", input.Control)
	}
	outputPath := filepath.Join(root, "advisor-output.json")
	result, err := PresentStop(root, inputPath, outputPath, time.Date(2026, 9, 13, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if result.NeedsSupervisionRepair || strings.Contains(result.HumanLine, "needs supervision repair") {
		t.Fatalf("ordinary advisor allowance was misclassified as failed supervision: %q", result.HumanLine)
	}
	reportBytes, _, err := ReadStopStatus(root, result.Report.Id)
	if err != nil || !strings.Contains(string(reportBytes), "OWNED-ELSEWHERE") {
		t.Fatalf("advisor guidance was not delivered in its report: %v\n%s", err, reportBytes)
	}
}

func TestPresentStopTimesOutWhenSessionPresentationLockIsBusy(t *testing.T) {
	root := stopPresentationRoot(t)
	input := stopPresentationFixture(root, strings.Repeat("8", 32), true)
	inputPath := filepath.Join(t.TempDir(), "input.json")
	outputPath := filepath.Join(t.TempDir(), "output.json")
	writeStopInput(t, inputPath, input)
	reportDir := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(reportDir, input.Identity.SessionKey+".present.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lockFile.Close()
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN) }()
	previous := stopPresentationLockWait
	stopPresentationLockWait = 0
	t.Cleanup(func() { stopPresentationLockWait = previous })

	if _, err := PresentStop(root, inputPath, outputPath, time.Now()); err == nil || !strings.Contains(err.Error(), "busy after") {
		t.Fatalf("busy presentation lock did not fail within its bound: %v", err)
	}
	if _, err := os.Lstat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("busy presentation lock published an output: %v", err)
	}
}

func TestStopAllowanceLineRetainsThirdRefusalAndDelegateWork(t *testing.T) {
	for _, test := range []struct {
		name      string
		attempt   string
		configure func(*StopPresentationInput)
		want      string
	}{
		{
			name:    "third-refusal-release",
			attempt: strings.Repeat("a", 32),
			configure: func(input *StopPresentationInput) {
				input.Judgment.Refusal.Occurrence = 3
				input.Judgment.FullDisplay = "refusal 3 reached the bound of 3; the turn will end"
			},
			want: "refusal 3 reached the bound of 3",
		},
		{
			name:    "delegate-exemption",
			attempt: strings.Repeat("b", 32),
			configure: func(input *StopPresentationInput) {
				input.Judgment.Refusal = goal.TurnRefusalFacts{}
				input.Judgment.Work.InFlight = []string{"job:delegate-1"}
				input.Judgment.Work.Claimable = []goal.GoalFacts{{Id: "waiting", Intent: "claim shared work", NextStep: "claim it"}}
			},
			want: "delegate work is in flight; claimable backlog remains",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := stopPresentationRoot(t)
			input := stopPresentationFixture(root, test.attempt, false)
			test.configure(&input)
			inputPath := filepath.Join(t.TempDir(), "input.json")
			outputPath := filepath.Join(t.TempDir(), "output.json")
			writeStopInput(t, inputPath, input)
			result, err := PresentStop(root, inputPath, outputPath, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(result.HumanLine, "\nTask: ") || !strings.Contains(result.HumanLine, "; Stop allowed; Report:") || strings.Contains(result.HumanLine, "needs ") {
				t.Fatalf("allowance line changed its work or intervention meaning: %q", result.HumanLine)
			}
			reportBytes, _, err := ReadStopStatus(root, result.Report.Id)
			if err != nil || !strings.Contains(string(reportBytes), test.want) {
				t.Fatalf("allowance report omitted %q: %v\n%s", test.want, err, reportBytes)
			}
		})
	}
}

func TestPresentStopStoppedJudgmentIsAnOrdinaryAllowance(t *testing.T) {
	root := stopPresentationRoot(t)
	input := stopPresentationFixture(root, strings.Repeat("c", 32), false)
	input.Judgment.Verdict.LedgerStatus = "stopped"
	input.Judgment.FullDisplay = "the metasystem is stopped; run: metasystem arm"
	input.Judgment.Work.ReadSucceeded = false
	input.Judgment.Work.Selection = "unknown"
	input.Judgment.Work.Selected = nil
	input.Judgment.Ownership = goal.TurnOwnershipFacts{State: "unknown"}
	inputPath := filepath.Join(t.TempDir(), "stopped-input.json")
	outputPath := filepath.Join(t.TempDir(), "stopped-output.json")
	writeStopInput(t, inputPath, input)
	result, err := PresentStop(root, inputPath, outputPath, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.HumanLine, "Just completed: unknown for this turn.\nNo task in flight; Stop allowed; Report: ") ||
		strings.Contains(result.HumanLine, "needs supervision repair") {
		t.Fatalf("stopped allowance line = %q", result.HumanLine)
	}
	reportBytes, err := os.ReadFile(result.Report.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reportBytes), input.Judgment.FullDisplay) {
		t.Fatalf("stopped detail missing from report: %s", reportBytes)
	}
}

func TestReadStopPresentationInputRejectsMissingTypedBoolean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":2,"control":{"shouldBlock":false,"judgmentAvailable":false},"completionObservation":null,"notices":[{"humanRequired":false}],"unavailable":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadStopPresentationInput(path); err == nil || !strings.Contains(err.Error(), "supervisionRepair") {
		t.Fatalf("missing classification accepted: %v", err)
	}
}

func TestPresentStopRejectsMissingArraysAndUnknownHealthRoles(t *testing.T) {
	root := stopPresentationRoot(t)
	input := stopPresentationFixture(root, strings.Repeat("e", 32), true)
	input.Judgment.Scan.Open = nil
	inputPath := filepath.Join(t.TempDir(), "missing-array.json")
	writeStopInput(t, inputPath, input)
	if _, err := PresentStop(root, inputPath, filepath.Join(t.TempDir(), "output.json"), time.Now()); err == nil || !strings.Contains(err.Error(), "required array open") {
		t.Fatalf("missing judgment array was accepted: %v", err)
	}

	input = stopPresentationFixture(root, strings.Repeat("f", 32), true)
	data, _ := json.Marshal(input)
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["notices"] = []any{nil}
	data, _ = json.Marshal(raw)
	inputPath = filepath.Join(t.TempDir(), "null-notice.json")
	if err := os.WriteFile(inputPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PresentStop(root, inputPath, filepath.Join(t.TempDir(), "output.json"), time.Now()); err == nil || !strings.Contains(err.Error(), "notices item 0 must be an object") {
		t.Fatalf("null typed item was accepted: %v", err)
	}

	input = stopPresentationFixture(root, strings.Repeat("d", 32), true)
	input.Health.Verdict.Roles[0].Role = steward.HealthRole("invented-role")
	input.Health.Interventions[0].Role = steward.HealthRole("invented-role")
	input.Health.Line = input.Health.Verdict.Line()
	inputPath = filepath.Join(t.TempDir(), "unknown-role.json")
	writeStopInput(t, inputPath, input)
	if _, err := PresentStop(root, inputPath, filepath.Join(t.TempDir(), "output.json"), time.Now()); err == nil || !strings.Contains(err.Error(), "intervention role") {
		t.Fatalf("unknown health role was accepted: %v", err)
	}
}

func TestPresentStopRejectsMissingRequiredNestedObjectAndSymlinkedReportDirectory(t *testing.T) {
	root := stopPresentationRoot(t)
	input := stopPresentationFixture(root, strings.Repeat("c", 32), true)
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw["judgment"].(map[string]any), "refusal")
	data, _ = json.Marshal(raw)
	missing := filepath.Join(t.TempDir(), "missing-refusal.json")
	if err := os.WriteFile(missing, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PresentStop(root, missing, filepath.Join(t.TempDir(), "output.json"), time.Now()); err == nil || !strings.Contains(err.Error(), "required object refusal") {
		t.Fatalf("missing refusal object was accepted: %v", err)
	}

	root = stopPresentationRoot(t)
	outside := t.TempDir()
	reportParent := filepath.Join(root, "artifacts", "agents", "supervision")
	if err := os.MkdirAll(reportParent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(reportParent, "stop-verdicts")); err != nil {
		t.Fatal(err)
	}
	input = stopPresentationFixture(root, strings.Repeat("d", 32), true)
	inputPath := filepath.Join(t.TempDir(), "symlink-input.json")
	writeStopInput(t, inputPath, input)
	if _, err := PresentStop(root, inputPath, filepath.Join(t.TempDir(), "output.json"), time.Now()); err == nil || !strings.Contains(err.Error(), "must not contain symlinks") {
		t.Fatalf("symlinked report directory was accepted: %v", err)
	}
}

func TestPresentStopSupportsParallelReportsAndRetentionBoundary(t *testing.T) {
	root := stopPresentationRoot(t)
	old := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	reportDir := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionKey := StopSessionKey("claude", "session-safe")
	for index := 0; index < 25; index++ {
		path := filepath.Join(reportDir, sessionKey+"-"+fmt.Sprintf("%032x", index+10)+".md")
		if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old.Add(time.Duration(index)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}

	var wait sync.WaitGroup
	errors := make(chan error, 2)
	work := t.TempDir()
	for _, digit := range []string{"1", "2"} {
		input := stopPresentationFixture(root, strings.Repeat(digit, 32), false)
		inputPath := filepath.Join(work, digit+"-input.json")
		outputPath := filepath.Join(work, digit+"-output.json")
		writeStopInput(t, inputPath, input)
		wait.Add(1)
		go func(inputPath, outputPath string) {
			defer wait.Done()
			_, err := PresentStop(root, inputPath, outputPath, time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC))
			errors <- err
		}(inputPath, outputPath)
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	paths, err := filepath.Glob(filepath.Join(reportDir, sessionKey+"-*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != stopReportRetentionCount {
		t.Fatalf("retained %d reports, want %d", len(paths), stopReportRetentionCount)
	}
}
