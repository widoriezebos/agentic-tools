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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport/stopreporttest"
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
		SchemaVersion: report.StopPresentationSchemaVersion,
		Identity:      report.StopIdentity{Installation: root, Runtime: runtime, Session: "session", SessionKey: report.StopSessionKey(runtime, "session"), Attempt: attempt, ObservedAt: "2026-09-13T12:00:00Z"},
		Judgment:      facts,
		Control:       report.StopControl{ShouldBlock: blocked, BlockSource: blockSource, Class: verdict.Class, JudgmentAvailable: true},
		CompletionObservation: &report.StopCompletionObservation{
			SchemaVersion: report.StopCompletionObservationSchemaVersion,
			Identity:      report.StopCompletionIdentity{Installation: root, Session: "session"},
			CollectedAt:   "2026-09-13T12:00:00Z", Records: []report.StopCompletionRecord{}, Unavailable: []string{},
		},
		Notices: []report.StopNotice{},
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
			for _, runtime := range []string{"claude", "codex", "devin", "fake"} {
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
					if !ok || strings.Count(line, "\n") != 1 {
						t.Fatalf("mapped human field = %#v", payload)
					}
					var presentationResult report.StopPresentationResult
					presentationBytes, readErr := os.ReadFile(presentation)
					if readErr != nil || json.Unmarshal(presentationBytes, &presentationResult) != nil ||
						!strings.HasSuffix(line, "--id "+presentationResult.Report.Alias) {
						t.Fatalf("mapped human field does not use its exact alias: %#v (%v)", payload, readErr)
					}
					wantLine := "Just completed: unknown for this turn.\nNo task in flight; Stop allowed; Report: metasystem report stop-status --id " + presentationResult.Report.Alias
					if test.blocked {
						wantLine = "Just completed: unknown for this turn.\nNo task in flight; Stop blocked; Do not stop. Run this command; read and act on its report: metasystem report stop-status --id " + presentationResult.Report.Alias
					}
					if line != wantLine {
						t.Fatalf("mapped human field = %q, want %q", line, wantLine)
					}
					if test.blocked && payload["decision"] != "block" {
						t.Fatalf("mapped block = %#v", payload)
					}
					if err := MapStopOutput("unknown", presentation, filepath.Join(root, "unknown.json")); err == nil || !strings.Contains(err.Error(), "no declared Stop output mapping") {
						t.Fatalf("unknown runtime mapping was accepted: %v", err)
					}
				})
			}
		})
	}
}

func TestMapStopOutputWritesTheResponseRecord(t *testing.T) {
	forStopOutputCases(t, func(t *testing.T, runtime string, blocked bool) {
		root, presentationPath := stopOutputFixture(t, runtime, blocked, strings.Repeat(runtime[:1], 32))
		var presentation report.StopPresentationResult
		data, err := os.ReadFile(presentationPath)
		if err != nil || json.Unmarshal(data, &presentation) != nil {
			t.Fatalf("read presentation: %v", err)
		}
		payloadObject := map[string]any{"systemMessage": presentation.HumanLine}
		if blocked {
			payloadObject = map[string]any{"decision": "block", "reason": presentation.HumanLine}
		}
		payload, err := json.Marshal(payloadObject)
		if err != nil {
			t.Fatal(err)
		}
		readOnly := filepath.Join(t.TempDir(), "read-only")
		if err := os.Mkdir(readOnly, 0o500); err != nil {
			t.Fatal(err)
		}
		if err := MapStopOutput(runtime, presentationPath, filepath.Join(readOnly, "provider.json")); err == nil {
			t.Fatal("mapping unexpectedly wrote into a read-only output directory")
		}
		response, err := stopreport.ReadResponse(root, payload)
		if err != nil {
			t.Fatalf("response was not written before the payload write: %v", err)
		}
		if response.Runtime != runtime || response.ShouldBlock != blocked || response.Report.Alias != presentation.Report.Alias {
			t.Fatalf("response = %+v", response)
		}
		output := filepath.Join(t.TempDir(), "provider.json")
		if err := MapStopOutput(runtime, presentationPath, output); err != nil {
			t.Fatal(err)
		}
		wire, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if string(wire) != string(payload)+"\n" {
			t.Fatalf("payload wire changed: got %q want %q", wire, append(payload, '\n'))
		}
	})
}
func TestMapStopOutputAcceptsChangedWording(t *testing.T) {
	forStopOutputCases(t, func(t *testing.T, runtime string, blocked bool) {
		root := filepath.Join(t.TempDir(), "installation")
		if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes="+runtime+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		root, _ = filepath.EvalSymlinks(root)
		published := stopreporttest.Publish(t, stopreporttest.Options{
			Root: root, Runtime: runtime, Session: "changed", Attempt: strings.Repeat(runtime[:1], 32),
			ShouldBlock: blocked, HumanLine: stopreporttest.ChangedHumanLine,
		})
		if err := os.Remove(published.ResponsePath); err != nil {
			t.Fatal(err)
		}
		presentationPath := writePublishedPresentation(t, published, blocked)
		output := filepath.Join(t.TempDir(), "provider.json")
		if err := MapStopOutput(runtime, presentationPath, output); err != nil {
			t.Fatalf("changed wording was refused: %v", err)
		}
		payload, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		response, err := stopreport.ReadResponse(published.Identity.Installation, payload)
		if err != nil || response.Report.Alias != published.Response.Report.Alias {
			t.Fatalf("changed wording response = %+v, %v", response, err)
		}
	})
}
func forStopOutputCases(t *testing.T, run func(*testing.T, string, bool)) {
	t.Helper()
	for _, runtime := range []string{"claude", "codex", "devin"} {
		for _, blocked := range []bool{false, true} {
			t.Run(runtime+map[bool]string{false: "/allowed", true: "/blocked"}[blocked], func(t *testing.T) {
				run(t, runtime, blocked)
			})
		}
	}
}
func writePublishedPresentation(t *testing.T, published stopreporttest.Published, blocked bool) string {
	t.Helper()
	var source *string
	if blocked {
		value := "fixture"
		source = &value
	}
	presentation := report.StopPresentationResult{
		SchemaVersion: report.StopPresentationSchemaVersion,
		Identity:      published.Identity,
		Control:       report.StopControl{ShouldBlock: blocked, BlockSource: source, JudgmentAvailable: true},
		HumanLine:     stopreporttest.ChangedHumanLine(published.Response.Report.Alias),
		Report: report.StopReportReference{
			Id: published.Response.Report.ID, Alias: published.Response.Report.Alias,
			Path: published.Response.Report.Path, SHA256: published.Response.Report.SHA256,
			ReadCommand: "metasystem report stop-status --id " + published.Response.Report.Alias,
		},
	}
	data, err := json.Marshal(presentation)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "presentation.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
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

	_, presentationPath = stopOutputFixture(t, "codex", false, strings.Repeat("a", 32))
	data, _ = os.ReadFile(presentationPath)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	raw["control"].(map[string]any)["blockSource"] = "unexpected"
	data, _ = json.Marshal(raw)
	if err := os.WriteFile(presentationPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MapStopOutput("codex", presentationPath, filepath.Join(t.TempDir(), "changed-source.json")); err == nil || !strings.Contains(err.Error(), "block source") {
		t.Fatalf("decision with an inconsistent block source was accepted: %v", err)
	}
}

func TestMapStopOutputRejectsUnboundReportReference(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*report.StopPresentationResult)
	}{
		{name: "trailing text", change: func(value *report.StopPresentationResult) {
			value.HumanLine += " now"
		}},
		{name: "substituted alias", change: func(value *report.StopPresentationResult) {
			value.Report.Alias = "f"
			value.Report.ReadCommand = "metasystem report stop-status --id f"
		}},
		{name: "unbound read command", change: func(value *report.StopPresentationResult) {
			value.Report.ReadCommand += "f"
		}},
		{name: "changed digest", change: func(value *report.StopPresentationResult) {
			value.Report.SHA256 = strings.Repeat("f", 64)
		}},
		{name: "changed path", change: func(value *report.StopPresentationResult) {
			value.Report.Path += ".other"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, presentationPath := stopOutputFixture(t, "codex", true, strings.Repeat("c", 32))
			data, err := os.ReadFile(presentationPath)
			if err != nil {
				t.Fatal(err)
			}
			var presentation report.StopPresentationResult
			if err := json.Unmarshal(data, &presentation); err != nil {
				t.Fatal(err)
			}
			test.change(&presentation)
			changed, _ := json.Marshal(presentation)
			if err := os.WriteFile(presentationPath, changed, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := MapStopOutput("codex", presentationPath, filepath.Join(t.TempDir(), "provider.json")); err == nil {
				t.Fatal("invalid read binding was accepted")
			}
		})
	}
}
