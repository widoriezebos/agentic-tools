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
		SchemaVersion: 1,
		Identity: StopIdentity{Installation: root, Runtime: "claude", Session: session, SessionKey: StopSessionKey("claude", session), Attempt: attempt,
			MainId: "main-->one", Machine: "bed-m1", Lineage: "lineage-1", ObservedAt: observed, ClaimEpoch: 7},
		Judgment: judgment, Control: control, Health: &health,
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

func TestPresentStopPublishesOneBoundedLineAndCompleteImmutableReport(t *testing.T) {
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
	if len(result.HumanLine) > StopHumanLineByteLimit || !utf8.ValidString(result.HumanLine) || strings.ContainsAny(result.HumanLine, "\r\n") {
		t.Fatalf("human line violates its wire bound: bytes=%d %q", len(result.HumanLine), result.HumanLine)
	}
	for _, want := range []string{"Task: ", "; Stop blocked; status: metasystem report stop-status --id ", result.Report.Id} {
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

func TestStopInterventionPhrasesUseOnlyTypedFlags(t *testing.T) {
	for _, test := range []struct {
		name          string
		human, repair bool
		want          string
	}{
		{"neither", false, false, "; Stop allowed; status:"},
		{"decision", true, false, "; Stop allowed; needs your decision; status:"},
		{"repair", false, true, "; Stop allowed; needs supervision repair; status:"},
		{"both", true, true, "; Stop allowed; needs your decision and supervision repair; status:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := StopPresentationInput{Notices: []StopNotice{{HumanRequired: test.human, SupervisionRepair: test.repair}}}
			human, repair := stopInterventions(input)
			if human != test.human || repair != test.repair {
				t.Fatalf("derived flags = %t,%t", human, repair)
			}
			line := compactStopLine("", "none", false, human, repair, "cmd")
			if !strings.Contains(line, test.want) {
				t.Fatalf("line = %q, want %q", line, test.want)
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
			if !strings.Contains(result.HumanLine, "Task: ") || !strings.Contains(result.HumanLine, "; Stop allowed; status:") || strings.Contains(result.HumanLine, "needs ") {
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
	if !strings.HasPrefix(result.HumanLine, "No task in flight; Stop allowed; status: ") ||
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
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"control":{"shouldBlock":false,"judgmentAvailable":false},"notices":[{"humanRequired":false}],"unavailable":[]}`), 0o644); err != nil {
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
