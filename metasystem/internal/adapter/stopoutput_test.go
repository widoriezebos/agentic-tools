package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
)

func stopOutputFixture(t *testing.T, runtime string, blocked bool, attempt string) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "installation")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes="+runtime+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, _ = filepath.EvalSymlinks(root)
	source := "idle-backlog"
	var blockSource *string
	if blocked {
		blockSource = &source
	}
	verdict := goal.Verdict{SchemaVersion: 1, Class: "idle-with-backlog", ShouldBlock: blocked, BlockSource: blockSource, LedgerStatus: "ok"}
	facts := &goal.TurnVerdictFacts{
		SchemaVersion: 1,
		Identity:      goal.TurnFactsIdentity{Installation: root, Session: "session", ObservedAt: "2026-09-13T12:00:00Z"},
		Verdict:       verdict,
		Scan: goal.ScanResult{
			Open: []goal.Item{}, TemplateUnfilled: []goal.Item{}, OpenWorkWarnings: []string{}, WaitingOnHuman: []goal.Item{},
			StalePlans: []goal.Item{}, Busy: []goal.Item{}, Questions: []goal.Item{}, Drafts: []goal.Item{}, Unreadable: []string{},
			Jobs: []goal.JobFact{}, Runs: []goal.RunFact{}, RunUnreadable: []string{},
		},
		Work:    goal.TurnWorkFacts{ReadSucceeded: true, Claimed: []goal.GoalFacts{}, Landing: []goal.GoalFacts{}, Claimable: []goal.GoalFacts{}, Refused: []goal.AdmissionRefusal{}, InFlight: []string{}, NonTerminalJobs: []string{}},
		Actions: []goal.TurnAction{},
	}
	input := report.StopPresentationInput{
		SchemaVersion: 1,
		Identity:      report.StopIdentity{Installation: root, Runtime: runtime, Session: "session", SessionKey: report.StopSessionKey(runtime, "session"), Attempt: attempt, ObservedAt: "2026-09-13T12:00:00Z"},
		Judgment:      facts,
		Control:       report.StopControl{ShouldBlock: blocked, BlockSource: blockSource, Class: verdict.Class, JudgmentAvailable: true},
		Notices:       []report.StopNotice{},
		Unavailable: []report.StopUnavailable{
			{Section: "health"}, {Section: "digest"}, {Section: "receipt"}, {Section: "arming"},
		},
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(t.TempDir(), "input.json")
	presentationPath := filepath.Join(t.TempDir(), "presentation.json")
	if err := os.WriteFile(inputPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := report.PresentStop(root, inputPath, presentationPath, time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	return root, presentationPath
}

func TestMapStopOutputUsesOnePublicFieldAndExactReport(t *testing.T) {
	for _, test := range []struct {
		name    string
		blocked bool
		field   string
		count   int
	}{
		{name: "block", blocked: true, field: "reason", count: 2},
		{name: "allow", blocked: false, field: "systemMessage", count: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, runtime := range []string{"claude", "codex", "fake"} {
				t.Run(runtime, func(t *testing.T) {
					root, presentation := stopOutputFixture(t, runtime, test.blocked, strings.Repeat(test.name[:1], 32))
					output := filepath.Join(t.TempDir(), "provider.json")
					if err := MapStopOutput(runtime, presentation, output); err != nil {
						t.Fatal(err)
					}
					data, err := os.ReadFile(output)
					if err != nil {
						t.Fatal(err)
					}
					var payload map[string]any
					if err := json.Unmarshal(data, &payload); err != nil || len(payload) != test.count {
						t.Fatalf("mapped payload = %s: %v", data, err)
					}
					line, ok := payload[test.field].(string)
					if !ok || !strings.HasSuffix(line, "-"+strings.Repeat(test.name[:1], 32)) {
						t.Fatalf("mapped human field = %#v", payload)
					}
					if test.blocked && payload["decision"] != "block" {
						t.Fatalf("mapped block = %#v", payload)
					}
					if err := MapStopOutput("devin", presentation, filepath.Join(root, "devin.json")); err == nil || !strings.Contains(err.Error(), "no declared Stop output mapping") {
						t.Fatalf("unknown Devin mapping was accepted: %v", err)
					}
				})
			}
		})
	}
}

func TestMapStopOutputRejectsChangedReportBeforePublication(t *testing.T) {
	_, presentationPath := stopOutputFixture(t, "codex", true, strings.Repeat("f", 32))
	var presentation report.StopPresentationResult
	data, _ := os.ReadFile(presentationPath)
	if err := json.Unmarshal(data, &presentation); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(presentation.Report.Path, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "provider.json")
	if err := MapStopOutput("codex", presentationPath, output); err == nil {
		t.Fatal("changed immutable report was accepted")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatalf("failed mapping published an output: %v", err)
	}
}

func TestMapStopOutputRejectsDecisionOrRequiredBooleanChangedAfterPresentation(t *testing.T) {
	_, presentationPath := stopOutputFixture(t, "codex", true, strings.Repeat("e", 32))
	data, err := os.ReadFile(presentationPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	control := raw["control"].(map[string]any)
	control["shouldBlock"] = false
	control["blockSource"] = nil
	raw["humanLine"] = strings.Replace(raw["humanLine"].(string), "; Stop blocked;", "; Stop allowed;", 1)
	data, _ = json.Marshal(raw)
	if err := os.WriteFile(presentationPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "provider.json")
	if err := MapStopOutput("codex", presentationPath, output); err == nil || !strings.Contains(err.Error(), "immutable report") {
		t.Fatalf("decision changed after presentation was accepted: %v", err)
	}

	_, presentationPath = stopOutputFixture(t, "codex", false, strings.Repeat("d", 32))
	data, _ = os.ReadFile(presentationPath)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "needsSupervisionRepair")
	data, _ = json.Marshal(raw)
	if err := os.WriteFile(presentationPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MapStopOutput("codex", presentationPath, filepath.Join(t.TempDir(), "missing-bool.json")); err == nil || !strings.Contains(err.Error(), "required boolean needsSupervisionRepair") {
		t.Fatalf("missing required presentation boolean was accepted: %v", err)
	}
}
