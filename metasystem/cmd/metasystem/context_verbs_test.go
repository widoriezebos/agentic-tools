package main

import (
	"bytes"
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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

func TestContextReportVerbPublishesTheWeek(t *testing.T) {
	root := contextCommandRoot(t)
	transcript := writeContextCommandTranscript(t, root, "report", 120000, 1, true)
	if _, err := usagepkg.LatestCall(root, "claude", "report", usagepkg.ReadOptions{
		Capability: usagepkg.PerCall, Transcript: transcript,
	}); err != nil {
		t.Fatal(err)
	}
	if err := usagepkg.RegisterSession(root, "claude", "report", 701, 7001); err != nil {
		t.Fatal(err)
	}
	code, output, problem := captureChannelOutput(t, func() int {
		return dispatch([]string{"context", "report", "--root", root, "--week", "2026-09-13"})
	})
	if code != 0 || problem != "" || !strings.Contains(output, "verdict=PASS") ||
		!strings.Contains(output, filepath.Join(root, "artifacts", "reports", "coordinator-context", "2026-09-13", "calls.jsonl")) ||
		!strings.Contains(output, filepath.Join(root, "artifacts", "reports", "coordinator-context", "2026-09-13", "report.md")) {
		t.Fatalf("report command = code %d stdout %q stderr %q", code, output, problem)
	}

	file, err := os.OpenFile(transcript, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(contextCommandLine("over-ceiling", 200001, 2)); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := usagepkg.LatestCall(root, "claude", "report", usagepkg.ReadOptions{
		Capability: usagepkg.PerCall, Transcript: transcript,
	}); err != nil {
		t.Fatal(err)
	}
	code, output, problem = captureChannelOutput(t, func() int {
		return runContextReport([]string{"--root", root, "--week", "2026-09-13"})
	})
	if code != 0 || problem != "" || !strings.Contains(output, "verdict=FAIL") {
		t.Fatalf("measured FAIL command = code %d stdout %q stderr %q", code, output, problem)
	}

	for _, args := range [][]string{
		{"--root", root},
		{"--root", root, "--week", "2026-9-13"},
		{"--root", root, "--week", "2026-02-30"},
	} {
		code, _, problem = captureChannelOutput(t, func() int { return runContextReport(args) })
		if code != 2 || problem == "" {
			t.Fatalf("bad arguments %v = code %d stderr %q", args, code, problem)
		}
	}
}

func TestContextReportVerbRefusesRetiredEvidence(t *testing.T) {
	root := contextCommandRoot(t)
	retentionPath := filepath.Join(root, "artifacts", "agents", "context", "retention.json")
	if err := os.MkdirAll(filepath.Dir(retentionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(retentionPath, []byte("{\"schemaVersion\":1,\"retainedSince\":\"2026-09-14T00:00:00Z\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "artifacts", "reports", "coordinator-context", "2026-09-13")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	callsPath, reportPath := filepath.Join(directory, "calls.jsonl"), filepath.Join(directory, "report.md")
	if err := os.WriteFile(callsPath, []byte("prior calls\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, []byte("prior report\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, output, problem := captureChannelOutput(t, func() int {
		return runContextReport([]string{"--root", root, "--week", "2026-09-13"})
	})
	if code != 9 || output != "" || problem != "CONTEXT_EVIDENCE_RETIRED requested=2026-09-13 retained-since=2026-09-14\n" {
		t.Fatalf("retired report command = code %d stdout %q stderr %q", code, output, problem)
	}
	for path, want := range map[string]string{callsPath: "prior calls\n", reportPath: "prior report\n"} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("retired report changed %s: got %q err=%v", path, got, err)
		}
	}
}

func TestContextReportPropagatesRecoveryError(t *testing.T) {
	root := contextCommandRoot(t)
	transcript := writeContextCommandTranscript(t, root, "report-error", 120000, 1, true)
	if _, err := usagepkg.LatestCall(root, "claude", "report-error", usagepkg.ReadOptions{
		Capability: usagepkg.PerCall, Transcript: transcript,
	}); err != nil {
		t.Fatal(err)
	}
	if err := usagepkg.RegisterSession(root, "claude", "report-error", 702, 7002); err != nil {
		t.Fatal(err)
	}
	if code, _, problem := captureChannelOutput(t, func() int {
		return runContextReport([]string{"--root", root, "--week", "2026-09-13"})
	}); code != 0 || problem != "" {
		t.Fatalf("seed report = code %d stderr %q", code, problem)
	}
	directory := filepath.Join(root, "artifacts", "reports", "coordinator-context", "2026-09-13")
	callsPath := filepath.Join(directory, "calls.jsonl")
	reportPath := filepath.Join(directory, "report.md")
	callsBefore, err := os.ReadFile(callsPath)
	if err != nil {
		t.Fatal(err)
	}
	reportBefore, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	cursorPath := usagepkg.CursorPath(root, "claude", "report-error")
	samplesPath := usagepkg.SamplesPath(root, "claude", "report-error")
	corrupt := []byte("{corrupt\n")
	if err := os.WriteFile(cursorPath, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	code, output, problem := captureChannelOutput(t, func() int {
		return runContextReport([]string{"--root", root, "--week", "2026-09-13"})
	})
	if code != 1 || output != "" || !strings.Contains(problem, cursorPath) || !strings.Contains(problem, samplesPath) {
		t.Fatalf("recovery command = code %d stdout %q stderr %q", code, output, problem)
	}
	if got, err := os.ReadFile(callsPath); err != nil || !bytes.Equal(got, callsBefore) {
		t.Fatalf("calls changed after recovery error: err=%v", err)
	}
	if got, err := os.ReadFile(reportPath); err != nil || !bytes.Equal(got, reportBefore) {
		t.Fatalf("report changed after recovery error: err=%v", err)
	}
	if got, err := os.ReadFile(cursorPath); err != nil || !bytes.Equal(got, corrupt) {
		t.Fatalf("corrupt evidence changed: err=%v", err)
	}
}

func TestContextStatusVerbPrintsTheRoleLine(t *testing.T) {
	root := contextCommandRoot(t)
	writeDerivedContextCommandTranscript(t, root, "inferred", 150001, 1, true)
	writeContextCommandHolder(t, root, "claude", "inferred")

	code, text, problem := captureChannelOutput(t, func() int {
		return dispatch([]string{"context", "status", "--root", root})
	})
	if code != 0 || problem != "" || !strings.HasPrefix(text, "context-budget=alive (150 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the trigger:") {
		t.Fatalf("inferred text status = code %d stdout %q stderr %q", code, text, problem)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "context", "sessions.jsonl")); err != nil {
		t.Fatalf("inferred holder was not registered: %v", err)
	}

	code, structured, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--json"})
	})
	if code != 0 || problem != "" {
		t.Fatalf("JSON status = code %d stdout %q stderr %q", code, structured, problem)
	}
	var decoded contextStatusOutput
	if err := json.Unmarshal([]byte(structured), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Diagnostic || !strings.HasPrefix(text, decoded.Role.Line()+"\n") || decoded.Reading.Latest == nil || decoded.Reading.Latest.PromptTokens != 150001 {
		t.Fatalf("text/JSON evaluations diverged: text=%q JSON=%+v", text, decoded)
	}

	explicitRoot := contextCommandRoot(t)
	explicitTranscript := writeContextCommandTranscript(t, explicitRoot, "explicit", 120000, 1, true)
	code, explicit, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", explicitRoot, "--runtime", "claude", "--session", "explicit", "--transcript", explicitTranscript})
	})
	if code != 0 || problem != "" || !strings.HasPrefix(explicit, "context-budget=alive (diagnostic transcript override; 120 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the trigger)\n") {
		t.Fatalf("explicit status = code %d stdout %q stderr %q", code, explicit, problem)
	}
	if _, err := os.Stat(filepath.Join(explicitRoot, "artifacts", "agents", "context", "sessions.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("explicit identity invented a process registration: %v", err)
	}
	if _, err := os.Stat(usagepkg.CursorPath(explicitRoot, "claude", "explicit")); !os.IsNotExist(err) {
		t.Fatalf("explicit diagnostic left a live cursor: %v", err)
	}

	code, _, problem = captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", explicitRoot, "--runtime", "claude"})
	})
	if code != 2 || !strings.Contains(problem, "--runtime R --session S") {
		t.Fatalf("unpaired identity = code %d stderr %q", code, problem)
	}
	code, _, problem = captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", explicitRoot, "--runtime", "unknown", "--session", "session"})
	})
	if code != 1 || !strings.Contains(problem, "unknown runtime: unknown") {
		t.Fatalf("unregistered explicit runtime = code %d stderr %q", code, problem)
	}

	templateRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(templateRoot, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(templateRoot, "development"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateRoot, "development", "metasystem-design.md"), []byte("template\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(templateRoot, "metasystem")
	if err := os.MkdirAll(installation, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeDerivedContextCommandTranscript(t, templateRoot, "template", 125000, 1, true)
	code, templateText, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", templateRoot, "--runtime", "claude", "--session", "template"})
	})
	if code != 0 || problem != "" || !strings.Contains(templateText, "125 thousand tokens") {
		t.Fatalf("containing template root = code %d stdout %q stderr %q", code, templateText, problem)
	}
	if _, err := os.Stat(usagepkg.CursorPath(installation, "claude", "template")); err != nil {
		t.Fatalf("template status did not use resolved state root: %v", err)
	}
}

func TestContextStatusNamesTheWindowAndItsSource(t *testing.T) {
	root := contextCommandRoot(t)
	conf := "launch.seat.window.tokens=210000\ncontext.ceiling.tokens=250000\ncontext.handoff.margin.tokens=145000\n"
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	writeShippedSeatWindow(t, root, 210000, true)
	code, output, problem := captureChannelOutput(t, func() int { return runContextStatus([]string{"--root", root}) })
	if code != 0 || problem != "" || !strings.Contains(output, "\nwindow: 210000 tokens (launch.seat.window.tokens, conf); ceiling: 250000 tokens (context.ceiling.tokens, conf); ceiling-above-window; shipped: 210000 (claude-code-hooks.json)\n") {
		t.Fatalf("code=%d output=%q problem=%q", code, output, problem)
	}
	code, structured, problem := captureChannelOutput(t, func() int { return runContextStatus([]string{"--root", root, "--json"}) })
	var decoded contextStatusOutput
	if err := json.Unmarshal([]byte(structured), &decoded); err != nil {
		t.Fatal(err)
	}
	if code != 0 || problem != "" || decoded.Window.Tokens != 210000 || decoded.Window.Source != "conf" || !decoded.Window.CeilingAboveWindow || decoded.Window.Shipped != 210000 || decoded.Window.ShippedSource != "scripts/enforcement/claude-code-hooks.json" || decoded.Window.ShippedDiffersFromConf {
		t.Fatalf("decoded=%+v code=%d problem=%q", decoded.Window, code, problem)
	}
	writeShippedSeatWindow(t, root, 200000, true)
	code, output, problem = captureChannelOutput(t, func() int { return runContextStatus([]string{"--root", root}) })
	if code != 0 || problem != "" || !strings.Contains(output, "; shipped: 200000 (claude-code-hooks.json) shipped-differs-from-conf\n") {
		t.Fatalf("code=%d output=%q problem=%q", code, output, problem)
	}
	code, structured, problem = captureChannelOutput(t, func() int { return runContextStatus([]string{"--root", root, "--json"}) })
	decoded = contextStatusOutput{}
	if err := json.Unmarshal([]byte(structured), &decoded); err != nil {
		t.Fatal(err)
	}
	if code != 0 || problem != "" || decoded.Window.Shipped != 200000 || decoded.Window.ShippedSource != "scripts/enforcement/claude-code-hooks.json" || !decoded.Window.ShippedDiffersFromConf {
		t.Fatalf("decoded=%+v code=%d problem=%q", decoded.Window, code, problem)
	}
}

func TestContextStatusPrintsTheBudgetLine(t *testing.T) {
	t.Setenv("METASYSTEM_CONTEXT_CEILING_TOKENS", "")
	_ = os.Unsetenv("METASYSTEM_CONTEXT_CEILING_TOKENS")
	t.Setenv("METASYSTEM_CONTEXT_HANDOFF_MARGIN_TOKENS", "")
	_ = os.Unsetenv("METASYSTEM_CONTEXT_HANDOFF_MARGIN_TOKENS")
	root := contextCommandRoot(t)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("context.ceiling.tokens=200000\ncontext.handoff.margin.tokens=120000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeDerivedContextCommandTranscript(t, root, "configured", 120000, 1, true)
	writeContextCommandHolder(t, root, "claude", "configured")
	code, output, problem := captureChannelOutput(t, func() int {
		return dispatch([]string{"context", "status", "--root", root})
	})
	if code != 0 || problem != "" || !strings.Contains(output, "trigger 80, proof line 150, proof maximum 200, ceiling 200") {
		t.Fatalf("configured status = code %d stdout %q stderr %q", code, output, problem)
	}
}

func TestContextStatusExitCodesForReadableUnknownAndCeilingBreach(t *testing.T) {
	root := contextCommandRoot(t)
	missingSession := "missing-" + filepath.Base(root)
	code, output, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", missingSession})
	})
	if code != 0 || problem != "" || !strings.HasPrefix(output, "context-budget=unknown (unknown (no transcript at ") {
		t.Fatalf("readable unknown = code %d stdout %q stderr %q", code, output, problem)
	}

	transcript := writeContextCommandTranscript(t, root, "ceiling", 200001, 1, true)
	code, output, problem = captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", "ceiling", "--transcript", transcript})
	})
	if code != 0 || problem != "" || !strings.HasPrefix(output, "context-budget=dead (diagnostic transcript override; 200 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the proof maximum") {
		t.Fatalf("ceiling breach = code %d stdout %q stderr %q", code, output, problem)
	}
}

func TestContextStatusLabelsTranscriptDiagnostics(t *testing.T) {
	root := contextCommandRoot(t)
	empty := filepath.Join(root, "empty.jsonl")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	code, emptyText, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", "empty", "--transcript", empty})
	})
	if code != 0 || problem != "" || !strings.HasPrefix(emptyText, "context-budget=alive (diagnostic transcript override; unknown (no call recorded yet))\n") {
		t.Fatalf("empty diagnostic = code %d stdout %q stderr %q", code, emptyText, problem)
	}

	overBound := writeContextCommandTranscript(t, root, "over-bound", 160000, 1, true)
	code, overBoundText, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", "over-bound", "--transcript", overBound})
	})
	wantOverBound := "context-budget=alive (diagnostic transcript override; 160 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the trigger)\n"
	if code != 0 || problem != "" || !strings.HasPrefix(overBoundText, wantOverBound) || strings.Contains(overBoundText, "handoff") {
		t.Fatalf("over-bound diagnostic = code %d stdout %q stderr %q", code, overBoundText, problem)
	}
	code, structured, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", "over-bound", "--transcript", overBound, "--json"})
	})
	var decoded contextStatusOutput
	if err := json.Unmarshal([]byte(structured), &decoded); err != nil {
		t.Fatal(err)
	}
	if code != 0 || problem != "" || !decoded.Diagnostic || !strings.HasPrefix(overBoundText, decoded.Role.Line()+"\n") ||
		strings.Contains(structured, `"seen"`) || strings.Contains(structured, `"tail"`) {
		t.Fatalf("structured diagnostic = code %d stdout %q stderr %q decoded=%+v", code, structured, problem, decoded)
	}

	overCeiling := writeContextCommandTranscript(t, root, "over-ceiling", 200001, 1, true)
	code, ceilingText, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", "over-ceiling", "--transcript", overCeiling})
	})
	if code != 0 || problem != "" || !strings.HasPrefix(ceilingText, "context-budget=dead (diagnostic transcript override; 200 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the proof maximum") ||
		!strings.Contains(ceilingText, "; remedy: metasystem context status --root "+root+")") || strings.Contains(ceilingText, "handoff") {
		t.Fatalf("over-ceiling diagnostic = code %d stdout %q stderr %q", code, ceilingText, problem)
	}

	earlyRoot := contextCommandRoot(t)
	lease := filepath.Join(earlyRoot, "artifacts", "agents", "mains", "worktree-lease.json")
	if err := os.MkdirAll(filepath.Dir(lease), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lease, []byte("null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, earlyText, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", earlyRoot, "--transcript", overBound})
	})
	if code != 1 || !strings.HasPrefix(earlyText, "context-budget=unknown (diagnostic transcript override; ") || !strings.Contains(earlyText, lease) || !strings.Contains(problem, lease) {
		t.Fatalf("early diagnostic error = code %d stdout %q stderr %q", code, earlyText, problem)
	}

	nonregular := filepath.Join(root, "nonregular")
	if err := os.Mkdir(nonregular, 0o700); err != nil {
		t.Fatal(err)
	}
	code, readErrorText, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", "read-error", "--transcript", nonregular})
	})
	if code != 1 || !strings.HasPrefix(readErrorText, "context-budget=unknown (diagnostic transcript override; ") ||
		!strings.Contains(readErrorText, "not a regular file") || !strings.Contains(problem, "not a regular file") {
		t.Fatalf("diagnostic read error = code %d stdout %q stderr %q", code, readErrorText, problem)
	}

	code, _, problem = captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--transcript", ""})
	})
	if code != 2 || !strings.Contains(problem, "usage: metasystem context status") {
		t.Fatalf("empty transcript flag = code %d stderr %q", code, problem)
	}
}

func TestContextStatusJSONOmitsCursorHistory(t *testing.T) {
	root := contextCommandRoot(t)
	transcript := filepath.Join(root, "large.jsonl")
	var rows strings.Builder
	firstID := contextResponseID(0)
	lastID := contextResponseID(6637)
	for index := 0; index < 6638; index++ {
		rows.WriteString(contextCommandLine(contextResponseID(index), 100000+int64(index), int64(index+1)))
	}
	const unfinished = "UNFINISHED-TAIL-MUST-NOT-LEAK"
	rows.WriteString(unfinished)
	if err := os.WriteFile(transcript, []byte(rows.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	code, output, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", "large", "--transcript", transcript, "--json"})
	})
	if code != 0 || problem != "" {
		t.Fatalf("large JSON status = code %d stderr %q", code, problem)
	}
	if strings.Contains(output, firstID) || strings.Contains(output, unfinished) || strings.Contains(output, `"seen"`) || strings.Contains(output, `"tail"`) {
		t.Fatalf("status exposed cursor history or tail: bytes=%d", len(output))
	}
	if !strings.Contains(output, lastID) || len(output) > 4096 {
		t.Fatalf("status did not stay bounded around the latest sample: bytes=%d latest=%t", len(output), strings.Contains(output, lastID))
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatal(err)
	}
	reading := decoded["reading"].(map[string]any)
	cursor := reading["cursor"].(map[string]any)
	if len(cursor) != 5 || cursor["offset"] == nil || cursor["line"] == nil || cursor["sidechainCount"] == nil || cursor["samplesBytes"] == nil || cursor["lastReadAt"] == nil {
		t.Fatalf("cursor summary has the wrong surface: %+v", cursor)
	}
}

func TestContextStatusPropagatesEvidenceErrors(t *testing.T) {
	t.Run("corrupt cursor beside retained samples", func(t *testing.T) {
		root, transcript := seedContextCommandReading(t, "corrupt")
		samplesPath := usagepkg.SamplesPath(root, "claude", "corrupt")
		beforeSamples, err := os.ReadFile(samplesPath)
		if err != nil {
			t.Fatal(err)
		}
		cursorPath := usagepkg.CursorPath(root, "claude", "corrupt")
		corruption := []byte("{not-json\n")
		if err := os.WriteFile(cursorPath, corruption, 0o600); err != nil {
			t.Fatal(err)
		}
		assertContextCommandError(t, root, "corrupt", transcript, "committed boundary is unavailable", samplesPath, beforeSamples)
		if got, err := os.ReadFile(cursorPath); err != nil || string(got) != string(corruption) {
			t.Fatalf("corrupt cursor changed: %q err=%v", got, err)
		}
	})

	t.Run("short samples", func(t *testing.T) {
		root, transcript := seedContextCommandReading(t, "short")
		samplesPath := usagepkg.SamplesPath(root, "claude", "short")
		committed, err := os.ReadFile(samplesPath)
		if err != nil {
			t.Fatal(err)
		}
		short := append([]byte(nil), committed[:len(committed)-1]...)
		if err := os.WriteFile(samplesPath, short, 0o600); err != nil {
			t.Fatal(err)
		}
		assertContextCommandError(t, root, "short", transcript, "below committed boundary", samplesPath, short)
	})

	t.Run("malformed announcement", func(t *testing.T) {
		root := contextCommandRoot(t)
		directory := filepath.Join(root, "artifacts", "agents", "mains")
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "worktree-lease.json"), []byte(`{"holderMainId":"main-context"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		announcement := filepath.Join(directory, "broken-1.json")
		before := []byte(`{"mainId":"main-context","runtime":"claude","pid":123,"pidStartedAt":456}`)
		if err := os.WriteFile(announcement, before, 0o600); err != nil {
			t.Fatal(err)
		}
		code, output, problem := captureChannelOutput(t, func() int { return runContextStatus([]string{"--root", root}) })
		if code != 1 || !strings.Contains(output, "context-budget=unknown") || !strings.Contains(output, announcement) || !strings.Contains(problem, announcement) {
			t.Fatalf("malformed announcement = code %d stdout %q stderr %q", code, output, problem)
		}
		if got, err := os.ReadFile(announcement); err != nil || string(got) != string(before) {
			t.Fatalf("malformed announcement changed: %q err=%v", got, err)
		}
	})

	t.Run("registry failure", func(t *testing.T) {
		root := contextCommandRoot(t)
		transcript := writeDerivedContextCommandTranscript(t, root, "registry", 120000, 1, true)
		writeContextCommandHolder(t, root, "claude", "registry")
		lock := filepath.Join(root, "artifacts", "agents", "context", "sessions.jsonl.lock")
		if err := os.MkdirAll(lock, 0o700); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(transcript)
		code, output, problem := captureChannelOutput(t, func() int {
			return runContextStatus([]string{"--root", root})
		})
		if code != 1 || !strings.Contains(output, "context-budget=unknown") || !strings.Contains(output, "sessions.jsonl.lock") || !strings.Contains(problem, "sessions.jsonl.lock") {
			t.Fatalf("registry failure = code %d stdout %q stderr %q", code, output, problem)
		}
		if got, err := os.ReadFile(transcript); err != nil || string(got) != string(before) {
			t.Fatalf("registry failure changed transcript: bytes=%d err=%v", len(got), err)
		}
		if _, err := os.Stat(usagepkg.CursorPath(root, "claude", "registry")); !os.IsNotExist(err) {
			t.Fatalf("registry failure still published a cursor: %v", err)
		}
	})
}

func TestContextTestingContractSelectsProof(t *testing.T) {
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := testpolicy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	groups := make(map[string]testpolicy.Group, len(contract.Groups))
	for _, group := range contract.Groups {
		groups[group.ID] = group
	}
	surfaces := make(map[string]testpolicy.Surface, len(contract.Surfaces))
	for _, surface := range contract.Surfaces {
		surfaces[surface.ID] = surface
	}
	contextSurface, ok := surfaces["context-budget"]
	if !ok {
		t.Fatal("context-budget surface is absent")
	}
	fallbackSurface, ok := surfaces[contract.Fallback]
	if !ok {
		t.Fatalf("fallback surface %q is absent", contract.Fallback)
	}
	if !reflect.DeepEqual(contextSurface.Standard, fallbackSurface.Standard) ||
		!reflect.DeepEqual(contextSurface.Deep, fallbackSurface.Deep) ||
		!reflect.DeepEqual(contextSurface.Critical, fallbackSurface.Critical) ||
		!reflect.DeepEqual(contextSurface.Risk, fallbackSurface.Risk) {
		t.Fatalf("context-budget lowers the fallback breadth: context=%+v fallback=%+v", contextSurface, fallbackSurface)
	}
	standard, ok := groups["context-standard"]
	if !ok {
		t.Fatal("context-standard group is absent")
	}
	foundations, ok := groups["context-foundations-standard"]
	if !ok {
		t.Fatal("context-foundations-standard group is absent")
	}
	allFoundations, foundationNames, err := testpolicy.GoTests(foundations)
	if err != nil || !allFoundations || len(foundationNames) != 0 ||
		!reflect.DeepEqual(foundations.Packages, []string{"internal/usage", "internal/output", "internal/runtimes"}) {
		t.Fatalf("context-foundations-standard does not run all three foundation packages: group=%+v all=%t names=%v err=%v",
			foundations, allFoundations, foundationNames, err)
	}
	cost, ok := groups["context-stop-cost"]
	if !ok {
		t.Fatal("context-stop-cost group is absent")
	}
	if cost.Env["METASYSTEM_CONTEXT_COST_PROOF"] != "1" {
		t.Fatalf("context-stop-cost does not enable its explicit proof: %+v", cost.Env)
	}
	all, names, err := testpolicy.GoTests(standard)
	if err != nil || all || len(names) == 0 {
		t.Fatalf("context-standard named tests = all %t names %v err %v", all, names, err)
	}
	wantedTests := []string{
		"TestCodexReaderDecodesEachLineOnce", "TestUnchangedEmptyTranscriptDoesNotRepublishCursor",
		"TestLatestCallReturnsThePreviousReadTime", "TestLatestCallNonBlockingReturnsBusy",
		"TestRegisterSessionNonBlockingReturnsBusy", "TestEveryDeclarationDeclaresContextSample",
		"TestRuntimeContextSampleVerb", "TestRoleContextRendersBoundCeilingAndUnknowns",
		"TestHealthLineCarriesContextBudget", "TestRoleContextNamesTheNewestSpill",
		"TestContextBudgetDoesNotAttributeUsageToAStaleHolder", "TestContextHolderResolutionSkipsForeignMalformedAnnouncements",
		"TestContextBudgetReturnsBusyUnknownWithoutWaiting", "TestContextBudgetReturnsRegistryBusyUnknownWithoutWaiting",
		"TestNewestSinceReturnsOnlyANewerRegularFile", "TestContextStatusVerbPrintsTheRoleLine",
		"TestContextStatusExitCodesForReadableUnknownAndCeilingBreach",
		"TestContextStatusJSONOmitsCursorHistory", "TestContextStatusPropagatesEvidenceErrors",
		"TestContextTranscriptOverrideUsesPrivateEvidence", "TestContextTranscriptOverridePreservesTheNextHealthRead",
		"TestContextTranscriptOverrideIgnoresLiveStoreFailures", "TestContextTranscriptOverrideDisposesPrivateCursor",
		"TestContextStatusLabelsTranscriptDiagnostics", "TestCallRegistrationsReadsStrictSnapshot",
		"TestCallSessionsDiscoversPairsWithoutReadingSamples", "TestContextReportVerbPublishesTheWeek",
		"TestContextReportComputesTheWeek", "TestContextReportPropagatesRecoveryError",
		"TestContextReportDeduplicatesTranscriptReplays", "TestContextReportHandlesFallbackAndConflictingIdentities",
		"TestContextReportWindowAndCoverage", "TestContextReportExcludesTranscriptDiagnostics",
		"TestContextCostRoleFromHealthLine",
		"TestContextTestingContractSelectsProof",
	}
	for _, name := range wantedTests {
		if !containsString(names, name) {
			t.Errorf("context-standard omitted %s", name)
		}
	}
	if containsString(names, "TestContextStopFitsDurationBudget") {
		t.Fatal("context-standard enables the large duration proof")
	}
	all, costNames, err := testpolicy.GoTests(cost)
	if err != nil || all || len(costNames) != 1 || costNames[0] != "TestContextStopFitsDurationBudget" {
		t.Fatalf("context-stop-cost tests = all %t names %v err %v", all, costNames, err)
	}

	for _, changed := range []string{
		"metasystem/internal/usage/cursor.go",
		"metasystem/internal/steward/context.go",
		"metasystem/scripts/agents/health-fixtures.sh",
		"metasystem/scripts/agents/supervision-hook-fixtures.sh",
	} {
		standardPlan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{
			ChangedPaths: []string{changed}, RequestedMode: testpolicy.ModeStandard, Purpose: testpolicy.PurposeDiagnostic,
		})
		if err != nil {
			t.Fatalf("standard selection for %s: %v", changed, err)
		}
		if !containsString(standardPlan.SelectedGroups, "context-standard") ||
			!containsString(standardPlan.SelectedGroups, "context-foundations-standard") ||
			!containsString(standardPlan.SelectedGroups, "section/supervision-and-census-fixtures") ||
			containsString(standardPlan.SelectedGroups, "context-stop-cost") {
			t.Fatalf("standard context selection for %s = %+v", changed, standardPlan)
		}
		deepPlan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{
			ChangedPaths: []string{changed}, RequestedMode: testpolicy.ModeDeep, Purpose: testpolicy.PurposeDiagnostic,
		})
		if err != nil {
			t.Fatalf("deep selection for %s: %v", changed, err)
		}
		for _, group := range []string{"context-standard", "context-foundations-standard", "section/supervision-and-census-fixtures", "context-stop-cost"} {
			if !containsString(deepPlan.SelectedGroups, group) {
				t.Errorf("deep context selection for %s omitted %s: %+v", changed, group, deepPlan)
			}
		}
	}
	usagePlan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{
		ChangedPaths: []string{"metasystem/internal/usage/cursor.go"}, RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDiagnostic,
	})
	if err != nil {
		t.Fatal(err)
	}
	fallbackPlan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{
		ChangedPaths: []string{"metasystem/residual-selection-probe"}, RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDiagnostic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if usagePlan.ExecutedMode != fallbackPlan.ExecutedMode || !reflect.DeepEqual(usagePlan.SelectedGroups, fallbackPlan.SelectedGroups) {
		t.Fatalf("internal/usage selection lowers the fallback plan: usage=%+v fallback=%+v", usagePlan, fallbackPlan)
	}
	t.Logf("internal/usage auto selection: %d surface-specific groups plus %d always-run canaries",
		len(usagePlan.SelectedGroups)-len(contract.Always.Canary), len(contract.Always.Canary))

	validateSource, err := os.ReadFile("../../scripts/validate-metasystem.sh")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(validateSource), "supervision_and_census_section() {")
	if start < 0 {
		t.Fatal("supervision and census section plumbing is not recognizable")
	}
	endMarker := "\nif section_selected supervisor-fingerprint-heal-harness"
	endOffset := strings.Index(string(validateSource[start:]), endMarker)
	if endOffset < 0 {
		t.Fatal("supervision and census section plumbing is not recognizable")
	}
	sectionBlock := string(validateSource[start : start+endOffset])
	fixtureRoot := t.TempDir()
	logPath := filepath.Join(fixtureRoot, "order.log")
	for _, name := range []string{"runtime-hook-fixtures.sh", "supervision-hook-fixtures.sh", "health-fixtures.sh", "supervision-fixtures.sh"} {
		path := filepath.Join(fixtureRoot, "scripts", "agents", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf("#!/usr/bin/env bash\nprintf '%%s\\n' %q >>\"${CONTEXT_SECTION_RECORD:?}\"\n", name)
		if err := testexec.WriteFile(path, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	driver := `set -euo pipefail
section_selected() { [[ "$1" == supervision-and-census-fixtures ]]; }
delegate_process_section() { return 0; }
delivery_contract_skip() { return 1; }
run_section() { shift 2; "$@"; }
` + sectionBlock
	command := exec.Command("bash", "-c", driver)
	command.Dir = fixtureRoot
	command.Env = append(os.Environ(), "CONTEXT_SECTION_RECORD="+logPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("recording section driver failed: %v\n%s", err, output)
	}
	order, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := "runtime-hook-fixtures.sh\nsupervision-hook-fixtures.sh\nsupervision-fixtures.sh\nhealth-fixtures.sh\n"
	if string(order) != wantOrder {
		t.Fatalf("supervision and census fixture order = %q, want %q", order, wantOrder)
	}
}

func TestContextHandoffVerb(t *testing.T) {
	container, root := contextHandoffCommandRoot(t)
	link := filepath.Join(t.TempDir(), "container")
	contextMust(t, os.Symlink(container, link))
	container = link
	useContextHandoffIdentity(t, filepath.Join(link, "metasystem"))
	contextMust(t, os.WriteFile(filepath.Join(root, "proof.txt"), []byte("bounded proof\n"), 0o644))
	code, output, problem := captureContextVerb(t, dispatch, "context", "handoff", "--root", container,
		"--note", contextHandoffNotePath(container), "--no-delegates",
		"--scratch", "purpose=proof,path=proof.txt,required=true", "--scratch", "purpose=second,path=proof.txt,required=false")
	if code != 0 || problem != "" || !strings.HasPrefix(output, "handoff recorded: ") || !strings.Contains(output, " state=") || !strings.Contains(output, " sha256=") {
		t.Fatalf("text handoff = code %d stdout %q stderr %q", code, output, problem)
	}
	fields := strings.Fields(output)
	first := contextHandoffRecord{fields[2], strings.TrimPrefix(fields[3], "state="), strings.TrimPrefix(fields[4], "sha256=")}
	stateData, err := os.ReadFile(first.state)
	var state steward.HandoffState
	if err != nil || json.Unmarshal(stateData, &state) != nil || len(state.Scratch) != 2 || !state.Scratch[0].Required || state.Scratch[1].Required || state.Seat.Machine != "m-test" || state.Seat.Runtime != "fake" || state.Seat.Session != "seat-session" || state.Seat.MainID != "main-context" || state.Seat.Tag != "main-tag" || state.Seat.Identity.StartTicks != 700 || state.Seat.Identity.BootID != "boot-fixture" {
		t.Fatalf("captured command state = %+v err=%v", state, err)
	}
	_, err = steward.ConsumeIntent(root, first.nonce)
	contextMust(t, err)
	jobID := "steward-" + first.nonce
	job, _ := json.Marshal(map[string]any{"jobId": jobID, "goalId": "goal-a", "role": "implementer", "status": "running", "phase": "work", "mainId": "delegate-main", "runtime": "fake", "sessionId": "delegate-session", "instanceTag": "delegate-tag", "pid": 4343, "pidStartedAt": 101})
	writeTestingFixtureFile(t, filepath.Join(root, "artifacts", "agents", "jobs", jobID+".json"), job, 0o644)
	classifyContextHandoffCaller = func(string, string, int64) (lease.Classification, error) {
		return lease.Classification{Class: lease.ClassDelegate, Pid: 4343}, nil
	}
	code, output, problem = captureContextVerb(t, runContextHandoff, "--root", container, "--note", contextHandoffNotePath(container), "--no-delegates", "--json")
	var projection map[string]any
	if err := json.Unmarshal([]byte(output), &projection); err != nil || code != 0 || problem != "" || len(projection) != 4 || projection["nonce"] == "" || projection["statePath"] == "" ||
		projection["digest"] == "" || projection["intentPath"] == "" || strings.Contains(output, "disposable") {
		t.Fatalf("JSON handoff = code %d stdout %q stderr %q", code, output, problem)
	}
	nonce, statePath, intentPath := projection["nonce"].(string), projection["statePath"].(string), projection["intentPath"].(string)
	stateSuffix := string(filepath.Separator) + filepath.Join("artifacts", "agents", "context", "handoffs", nonce, "state.json")
	canonical := strings.TrimSuffix(statePath, stateSuffix)
	wantIntentPath := filepath.Join(canonical, "artifacts", "agents", "steward", "intents", nonce+".json")
	if intentPath != wantIntentPath || strings.HasPrefix(intentPath, link) {
		t.Fatalf("intent path=%q want canonical path=%q link=%q", intentPath, wantIntentPath, link)
	}
	if _, err := os.Stat(intentPath); err != nil {
		t.Fatalf("intent path is not live: %v", err)
	}
	code, _, problem = captureContextVerb(t, runContextHandoff, "--root", container, "--note", contextHandoffNotePath(container), "--no-delegates", "--scratch", "purpose=required,path=missing\nfile,required=true")
	if code != 9 || !strings.HasPrefix(problem, "HANDOFF_REFERENCE ") || strings.Count(problem, "\n") != 1 {
		t.Fatalf("refused handoff = code %d stderr %q", code, problem)
	}
	hookContextHandoffDelegate = func(string, string, string, int64) (lease.HookDelegateResult, error) {
		return lease.HookDelegateResult{}, nil
	}
	code, _, problem = captureContextVerb(t, runContextHandoff, "--root", container, "--note", contextHandoffNotePath(container), "--no-delegates")
	if code != 9 || !strings.HasPrefix(problem, "HANDOFF_NOT_HOLDER") {
		t.Fatalf("wrong delegate = code %d stderr %q", code, problem)
	}
}

const (
	contextAgentTaskID     = "ab3e7e704e393e416"
	contextAgentTaskOutput = "/private/tmp/claude-501/<slug>/<session>/tasks/ab3e7e704e393e416.output"
	contextBashTaskID      = "b2rs8m65o"
	contextBashTaskOutput  = "/private/tmp/claude-501/<slug>/<session>/tasks/b2rs8m65o.output"
)

func TestHandoffRefusals(t *testing.T) {
	t.Run("tasks-in-flight", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		note := contextClaudeHandoffFixture(t, root, "tasks-in-flight", []string{"agent-running"}, contextAgentTaskOutput+"\n")
		useContextHandoffIdentityFor(t, root, "claude", "tasks-in-flight", 100)
		code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note, "--no-delegates")
		if code != 9 || problem != "HANDOFF_TASKS_IN_FLIGHT ids="+contextAgentTaskID+"\n" {
			t.Fatalf("tasks-in-flight refusal = code %d stderr %q", code, problem)
		}
	})
	t.Run("delegate-undeclared", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		note := contextClaudeHandoffFixture(t, root, "delegate-undeclared", []string{"agent-running", "bash-running"}, contextAgentTaskOutput+"\n")
		useContextHandoffIdentityFor(t, root, "claude", "delegate-undeclared", 100)
		code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note, "--delegate", "id="+contextAgentTaskID)
		if code != 9 || problem != "HANDOFF_DELEGATE_UNDECLARED id="+contextBashTaskID+"\n" {
			t.Fatalf("delegate-undeclared refusal = code %d stderr %q", code, problem)
		}
	})
	t.Run("delegate-unknown", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		note := contextClaudeHandoffFixture(t, root, "delegate-unknown", []string{"agent-running"}, "/tmp/unknown.out\n")
		useContextHandoffIdentityFor(t, root, "claude", "delegate-unknown", 100)
		code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note, "--delegate", "id=unknown,asked=work,output=/tmp/unknown.out")
		if code != 9 || problem != "HANDOFF_DELEGATE_UNKNOWN id=unknown\n" {
			t.Fatalf("delegate-unknown refusal = code %d stderr %q", code, problem)
		}
	})
	t.Run("delegate-output-mismatch", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		note := contextClaudeHandoffFixture(t, root, "delegate-output-mismatch", []string{"agent-running"}, "/tmp/wrong.out\n")
		useContextHandoffIdentityFor(t, root, "claude", "delegate-output-mismatch", 100)
		code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note, "--delegate", "id="+contextAgentTaskID+",output=/tmp/wrong.out")
		if code != 9 || problem != "HANDOFF_DELEGATE_OUTPUT_MISMATCH id="+contextAgentTaskID+"\n" {
			t.Fatalf("delegate-output-mismatch refusal = code %d stderr %q", code, problem)
		}
	})
	t.Run("delegates-undeclared", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		useContextHandoffIdentity(t, root)
		code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", contextHandoffNotePath(container))
		if code != 9 || problem != "HANDOFF_DELEGATES_UNDECLARED\n" {
			t.Fatalf("delegates-undeclared refusal = code %d stderr %q", code, problem)
		}
	})
	t.Run("delegate-incomplete-without-transcript", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		useContextHandoffIdentity(t, root)
		note := contextHandoffNotePath(container)
		for _, test := range []struct {
			declaration string
			want        string
		}{
			{"id=manual", "HANDOFF_DELEGATE_INCOMPLETE id=manual missing=asked,output\n"},
			{"id=manual,asked=finish", "HANDOFF_DELEGATE_INCOMPLETE id=manual missing=output\n"},
		} {
			code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note, "--delegate", test.declaration)
			if code != 9 || problem != test.want {
				t.Fatalf("declaration %q = code %d stderr %q", test.declaration, code, problem)
			}
		}
		writeContextHandoffNote(t, note, "manual output: /tmp/manual.out\n", time.Date(2026, 9, 18, 11, 59, 0, 0, time.UTC))
		code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note,
			"--delegate", "id=manual,asked=finish,output=/tmp/manual.out")
		if code != 0 || problem != "" {
			t.Fatalf("complete transcript-free declaration = code %d stderr %q", code, problem)
		}
	})
	for _, test := range []struct {
		name string
		run  func(*testing.T, string, string)
	}{
		{"note-missing", func(t *testing.T, container, _ string) {
			code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--no-delegates")
			if code != 9 || problem != "HANDOFF_NOTE_MISSING\n" {
				t.Fatalf("note-missing refusal = code %d stderr %q", code, problem)
			}
		}},
		{"note-outside-memory", func(t *testing.T, container, _ string) {
			outside := filepath.Join(t.TempDir(), "lessons.md")
			writeContextHandoffNote(t, outside, "lessons\n", time.Date(2026, 9, 18, 11, 59, 0, 0, time.UTC))
			code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", outside, "--no-delegates")
			if code != 9 || problem != "HANDOFF_NOTE_OUTSIDE_MEMORY path="+outside+"\n" {
				t.Fatalf("note-outside-memory refusal = code %d stderr %q", code, problem)
			}
		}},
		{"note-symlink", func(t *testing.T, container, _ string) {
			target := filepath.Join(t.TempDir(), "lessons.md")
			writeContextHandoffNote(t, target, "lessons\n", time.Date(2026, 9, 18, 11, 59, 0, 0, time.UTC))
			link := filepath.Join(filepath.Dir(contextHandoffNotePath(container)), "linked.md")
			contextMust(t, os.Symlink(target, link))
			code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", link, "--no-delegates")
			if code != 9 || problem != "HANDOFF_NOTE_OUTSIDE_MEMORY path="+link+"\n" {
				t.Fatalf("note-symlink refusal = code %d stderr %q", code, problem)
			}
		}},
		{"note-lacks-output", func(t *testing.T, container, _ string) {
			note := contextHandoffNotePath(container)
			code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note,
				"--delegate", "id=manual,asked=finish,output=/tmp/missing.out")
			if code != 9 || problem != "HANDOFF_NOTE_LACKS_OUTPUT path="+note+"\n" {
				t.Fatalf("note-lacks-output refusal = code %d stderr %q", code, problem)
			}
		}},
		{"note-stale", func(t *testing.T, container, root string) {
			started := time.Date(2026, 9, 18, 11, 0, 0, 0, time.UTC)
			useContextHandoffIdentityFor(t, root, "fake", "seat-session", started.Unix())
			note := contextHandoffNotePath(container)
			writeContextHandoffNote(t, note, "lessons\n", started.Add(-time.Second))
			code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note, "--no-delegates")
			if code != 9 || !strings.HasPrefix(problem, "HANDOFF_NOTE_STALE note="+note+" ") {
				t.Fatalf("note-stale refusal = code %d stderr %q", code, problem)
			}
		}},
		{"note-same-second", func(t *testing.T, container, root string) {
			started := time.Date(2026, 9, 18, 11, 0, 0, 0, time.UTC)
			useContextHandoffIdentityFor(t, root, "fake", "seat-session", started.Unix())
			note := contextHandoffNotePath(container)
			writeContextHandoffNote(t, note, "same lessons\n", started)
			first := recordContextHandoff(t, container)
			if first.nonce == "" {
				t.Fatal("first same-second note did not record")
			}
			code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note, "--no-delegates")
			if code != 9 || !strings.HasPrefix(problem, "HANDOFF_NOTE_STALE note="+note+" ") {
				t.Fatalf("note-same-second refusal = code %d stderr %q", code, problem)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			container, root := contextHandoffCommandRoot(t)
			useContextHandoffIdentity(t, root)
			test.run(t, container, root)
		})
	}
}

func TestHandoffRecordsDeclaredTasksFromTheTranscript(t *testing.T) {
	container, root := contextHandoffCommandRoot(t)
	session := "declared-tasks"
	note := contextClaudeHandoffFixture(t, root, session, []string{"agent-running", "bash-completed"},
		contextAgentTaskOutput+"\n"+contextBashTaskOutput+"\n")
	useContextHandoffIdentityFor(t, root, "claude", session, 100)
	code, output, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note,
		"--delegate", "id="+contextAgentTaskID, "--delegate", "id="+contextBashTaskID)
	if code != 0 || problem != "" {
		t.Fatalf("declared task handoff = code %d stdout %q stderr %q", code, output, problem)
	}
	fields := strings.Fields(output)
	state := readContextHandoffState(t, strings.TrimPrefix(fields[3], "state="))
	if len(state.Delegates) != 2 || state.Delegates[0].ID != contextAgentTaskID || state.Delegates[0].Kind != "agent" ||
		state.Delegates[0].Asked != "Token diagnosis for 2026-09-15" || state.Delegates[0].Output != contextAgentTaskOutput || state.Delegates[0].Terminal ||
		state.Delegates[1].ID != contextBashTaskID || state.Delegates[1].Kind != "bash" || state.Delegates[1].Output != contextBashTaskOutput || !state.Delegates[1].Terminal {
		t.Fatalf("recorded delegates = %+v", state.Delegates)
	}
	canonicalNote, err := filepath.EvalSymlinks(note)
	if err != nil {
		t.Fatal(err)
	}
	if state.LessonsNote == nil || state.LessonsNote.Path != canonicalNote || state.LessonsNote.SHA256 == "" {
		t.Fatalf("recorded lessons note = %+v", state.LessonsNote)
	}

	t.Run("unknown status notice", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		note := contextClaudeHandoffFixture(t, root, "unknown-status", []string{"unknown-status"}, contextAgentTaskOutput+"\n")
		useContextHandoffIdentityFor(t, root, "claude", "unknown-status", 100)
		code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note, "--delegate", "id="+contextAgentTaskID)
		want := "task " + contextAgentTaskID + " status paused: treated as in flight\n"
		if code != 0 || problem != want {
			t.Fatalf("unknown-status notice = code %d stderr %q", code, problem)
		}
	})
}

func TestHandoffRecordsNoEmptyDelegateField(t *testing.T) {
	container, root := contextHandoffCommandRoot(t)
	useContextHandoffIdentity(t, root)
	note := contextHandoffNotePath(container)
	writeContextHandoffNote(t, note, "/tmp/manual.out\n", time.Date(2026, 9, 18, 11, 59, 0, 0, time.UTC))
	code, output, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note,
		"--delegate", "id=manual,asked=finish,output=/tmp/manual.out")
	if code != 0 || problem != "" {
		t.Fatalf("complete delegate handoff = code %d stderr %q", code, problem)
	}
	fields := strings.Fields(output)
	state := readContextHandoffState(t, strings.TrimPrefix(fields[3], "state="))
	if len(state.Delegates) != 1 || state.Delegates[0].Asked == "" || state.Delegates[0].Output == "" {
		t.Fatalf("recorded delegate has an empty field: %+v", state.Delegates)
	}
}

func TestHandoffNoteSameSecondPassesWithANewDigest(t *testing.T) {
	container, root := contextHandoffCommandRoot(t)
	started := time.Date(2026, 9, 18, 11, 0, 0, 0, time.UTC)
	useContextHandoffIdentityFor(t, root, "fake", "seat-session", started.Unix())
	note := contextHandoffNotePath(container)
	writeContextHandoffNote(t, note, "first lessons\n", started)
	first := recordContextHandoff(t, container)
	writeContextHandoffNote(t, note, "changed lessons\n", started)
	second := recordContextHandoff(t, container)
	if first.nonce == second.nonce {
		t.Fatalf("same-second note digest did not produce a replacement: %+v %+v", first, second)
	}
}

func TestHandoffNoteDirectoryIsPerRuntime(t *testing.T) {
	container, root := contextHandoffCommandRoot(t)
	directory := filepath.Join(root, ".codex-handoff-memory")
	note := filepath.Join(directory, "lessons.md")
	writeContextHandoffNote(t, note, "output /tmp/codex-task.out\n", time.Date(2026, 9, 18, 11, 59, 0, 0, time.UTC))
	conf := filepath.Join(root, "metasystem.conf")
	handle, err := os.OpenFile(conf, os.O_APPEND|os.O_WRONLY, 0o644)
	contextMust(t, err)
	_, err = fmt.Fprintln(handle, "context.handoff.note-directory.codex="+directory)
	contextMust(t, errors.Join(err, handle.Close()))
	useContextHandoffIdentityFor(t, root, "codex", "codex-session", 100)
	code, output, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--note", note,
		"--delegate", "id=codex-task,asked=finish,output=/tmp/codex-task.out")
	if code != 0 || problem != "" {
		t.Fatalf("Codex note handoff = code %d stderr %q", code, problem)
	}
	fields := strings.Fields(output)
	state := readContextHandoffState(t, strings.TrimPrefix(fields[3], "state="))
	canonicalNote, err := filepath.EvalSymlinks(note)
	if err != nil {
		t.Fatal(err)
	}
	if state.Seat.Runtime != "codex" || state.LessonsNote == nil || state.LessonsNote.Path != canonicalNote {
		t.Fatalf("Codex handoff state = %+v", state)
	}
}

func TestContextVerifyAndCancel(t *testing.T) {
	container, root := contextHandoffCommandRoot(t)
	useContextHandoffIdentity(t, root)
	first := recordContextHandoff(t, container)
	code, output, problem := captureContextVerb(t, dispatch, "context", "verify", "--root", container, "--nonce", first.nonce)
	if code != 0 || output != "ok sha256="+first.digest+"\n" || problem != "" {
		t.Fatalf("verify = code %d stdout %q stderr %q", code, output, problem)
	}
	code, output, problem = captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", first.nonce)
	if code != 0 || output != "handoff cancelled: "+first.nonce+"\n" || problem != "" {
		t.Fatalf("cancel = code %d stdout %q stderr %q", code, output, problem)
	}
	foreign := recordContextHandoff(t, container)
	identityClassifier := classifyContextHandoffCaller
	classifyContextHandoffCaller = func(string, string, int64) (lease.Classification, error) {
		announcement := &lease.Announcement{SessionId: "other-session", MainId: "main-context", Pid: 4242,
			PidStartedAt: 100, PidStartTicks: 700, BootID: "boot-fixture", Runtime: "fake", InstanceTag: "main-tag"}
		return lease.Classification{Class: lease.ClassMain, MainId: announcement.MainId, Announcement: announcement}, nil
	}
	code, _, problem = captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", foreign.nonce)
	if code != 9 || problem != "HANDOFF_OTHER_SESSION nonce="+foreign.nonce+" session=seat-session caller=MAIN human=none\n" {
		t.Fatalf("foreign cancel = code %d stderr %q", code, problem)
	}
	classifyContextHandoffCaller = func(string, string, int64) (lease.Classification, error) {
		return lease.Classification{}, errors.New("injected classification failure")
	}
	code, _, problem = captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", foreign.nonce)
	if code != 1 || !strings.HasPrefix(problem, "metasystem context handoff: classify caller:") {
		t.Fatalf("unclassified cancel = code %d stderr %q", code, problem)
	}
	classifyContextHandoffCaller = identityClassifier
	if code, _, problem = captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", foreign.nonce); code != 0 || problem != "" {
		t.Fatalf("restored recorder cancel = code %d stderr %q", code, problem)
	}
	second := recordContextHandoff(t, container)
	_, err := steward.ConsumeIntent(root, second.nonce)
	contextMust(t, err)
	contextMust(t, os.WriteFile(second.state, []byte("{}\n"), 0o644))
	code, _, problem = captureContextVerb(t, runContextVerify, "--root", container, "--nonce", second.nonce)
	if code != 9 || !strings.HasPrefix(problem, "HANDOFF_STATE_MISMATCH ") {
		t.Fatalf("drifted verify = code %d stderr %q", code, problem)
	}
	code, _, problem = captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", second.nonce)
	if code != 9 || !strings.HasPrefix(problem, "HANDOFF_NOT_LIVE ") {
		t.Fatalf("consumed cancel = code %d stderr %q", code, problem)
	}
}

func TestContextHandoffCancelByHuman(t *testing.T) {
	t.Run("attended human", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		useContextHandoffIdentity(t, root)
		record := recordContextHandoff(t, container)
		installSessionStopLease(t, root, map[string]any{
			"holderMainId": "main-context", "pid": 4242, "pidStartedAt": 100, "claimEpoch": 7, "revision": 1,
		})
		now := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
		proof := sessionStopCommandProof(t, root, now)
		classifyContextHandoffCaller = func(string, string, int64) (lease.Classification, error) {
			return lease.Classification{Class: lease.ClassHuman}, nil
		}
		order := []string{}
		currentContextHandoffHolder = func(gotRoot string) (lease.CurrentHolderView, error) {
			if gotRoot != root || !reflect.DeepEqual(order, []string{"proof"}) {
				t.Fatalf("holder read root=%q order=%v", gotRoot, order)
			}
			order = append(order, "holder")
			return lease.CurrentHolderView{MainId: "main-context", SessionId: "seat-session", ClaimEpoch: 7}, nil
		}
		calls := 0
		proveSessionStopHuman = func(gotRoot string, pid int64, gotNow time.Time) (humanauthority.Proof, error) {
			calls++
			order = append(order, "proof")
			if gotRoot != root || pid != int64(os.Getppid()) || gotNow.IsZero() {
				t.Fatalf("human proof arguments root=%q pid=%d now=%s", gotRoot, pid, gotNow)
			}
			return proof, nil
		}
		code, output, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", record.nonce, "--by", "Wido")
		if code != 0 || output != "handoff cancelled: "+record.nonce+"\n" || problem != "" || calls != 1 || !reflect.DeepEqual(order, []string{"proof", "holder"}) {
			t.Fatalf("human cancel = code %d stdout %q stderr %q proof-calls=%d", code, output, problem, calls)
		}
		boot := proof.InvokerRef.BootID
		if boot == "" {
			boot = "none"
		}
		want := fmt.Sprintf("cancelled: cancelled by human by=Wido holder=main-context epoch=7 session=seat-session human-pid=%d human-started=%d human-ticks=%d human-boot=%s",
			proof.InvokerRef.PID, proof.InvokerRef.PIDStartedAt, proof.InvokerRef.StartTicks, boot)
		data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "cancelled", record.nonce+".json"))
		var intent steward.Intent
		if err != nil || json.Unmarshal(data, &intent) != nil || intent.Outcome != want {
			t.Fatalf("human cancellation outcome=%q want=%q err=%v", intent.Outcome, want, err)
		}
	})

	t.Run("holder read failure", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		useContextHandoffIdentity(t, root)
		classifyContextHandoffCaller = func(string, string, int64) (lease.Classification, error) {
			return lease.Classification{Class: lease.ClassHuman}, nil
		}
		currentContextHandoffHolder = func(string) (lease.CurrentHolderView, error) {
			return lease.CurrentHolderView{}, errors.New("holder read failed")
		}
		code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", "0000000000000000", "--by", "Wido")
		if code != 1 || problem != "metasystem context handoff: holder read failed\n" {
			t.Fatalf("holder failure = code %d stderr %q", code, problem)
		}
	})

	t.Run("agent never reaches the prover", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		useContextHandoffIdentity(t, root)
		record := recordContextHandoff(t, container)
		identityClassifier := classifyContextHandoffCaller
		identityHolder := currentContextHandoffHolder
		classifyContextHandoffCaller = func(string, string, int64) (lease.Classification, error) {
			return lease.Classification{Class: lease.ClassDelegate, Pid: 4343}, nil
		}
		proveSessionStopHuman = func(string, int64, time.Time) (humanauthority.Proof, error) {
			t.Fatal("agent reached the human prover")
			return humanauthority.Proof{}, nil
		}
		currentContextHandoffHolder = func(string) (lease.CurrentHolderView, error) {
			t.Fatal("agent reached the human holder read")
			return lease.CurrentHolderView{}, nil
		}
		code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", record.nonce, "--by", "Agent")
		if code != 9 || problem != "HANDOFF_HUMAN_UNPROVEN nonce="+record.nonce+" by=Agent caller=DELEGATE human=unattempted\n" {
			t.Fatalf("agent cancel = code %d stderr %q", code, problem)
		}
		classifyContextHandoffCaller, currentContextHandoffHolder = identityClassifier, identityHolder
		if code, _, problem = captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", record.nonce); code != 0 || problem != "" {
			t.Fatalf("recorder cancel after refused agent = code %d stderr %q", code, problem)
		}
	})

	t.Run("refused proof", func(t *testing.T) {
		container, root := contextHandoffCommandRoot(t)
		useContextHandoffIdentity(t, root)
		record := recordContextHandoff(t, container)
		classifyContextHandoffCaller = func(string, string, int64) (lease.Classification, error) {
			return lease.Classification{Class: lease.ClassHuman}, nil
		}
		proveSessionStopHuman = func(string, int64, time.Time) (humanauthority.Proof, error) {
			return humanauthority.Proof{Outcome: humanauthority.OutcomeAgent}, errors.New("refused walk")
		}
		code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", record.nonce, "--by", "Wido")
		if code != 9 || problem != "HANDOFF_HUMAN_UNPROVEN nonce="+record.nonce+" by=Wido caller=HUMAN human=AGENT_IN_AUTHORITY_CHAIN\n" {
			t.Fatalf("refused proof cancel = code %d stderr %q", code, problem)
		}
	})
}

func TestContextPruneVerb(t *testing.T) {
	container, root := contextHandoffCommandRoot(t)
	useContextHandoffIdentity(t, root)
	record := recordContextHandoff(t, container)
	if code, _, problem := captureContextVerb(t, runContextHandoff, "--root", container, "--cancel", record.nonce); code != 0 || problem != "" {
		t.Fatalf("cancel before prune = code %d stderr %q", code, problem)
	}
	contextMust(t, os.Chtimes(record.state, time.Now().Add(-15*24*time.Hour), time.Now().Add(-15*24*time.Hour)))
	code, output, problem := captureContextVerb(t, dispatch, "context", "prune", "--root", container)
	if code != 0 || problem != "" || output != "pruned call-sessions=0\npruned handoff="+filepath.Dir(record.state)+"\n" {
		t.Fatalf("prune = code %d stdout %q stderr %q", code, output, problem)
	}
	code, output, problem = captureContextVerb(t, runContextPrune, "--root", container, "--older-than", "14d")
	if code != 0 || output != "pruned call-sessions=0\n" || problem != "" {
		t.Fatalf("day duration = code %d stdout %q stderr %q", code, output, problem)
	}
	code, output, problem = captureContextVerb(t, runContextPrune, "--root", container, "--older-than", "336h")
	if code != 0 || output != "pruned call-sessions=0\n" || problem != "" {
		t.Fatalf("Go duration = code %d stdout %q stderr %q", code, output, problem)
	}
	contextMust(t, os.MkdirAll(filepath.Join(root, "artifacts", "agents", "context", "handoffs", "malformed"), 0o755))
	code, output, problem = captureContextVerb(t, runContextPrune, "--root", container, "--older-than", "14d")
	if code != 1 || output != "pruned call-sessions=0\n" || !strings.Contains(problem, "malformed nonce") {
		t.Fatalf("partial prune = code %d stdout %q stderr %q", code, output, problem)
	}
}

func TestContextVerbUsage(t *testing.T) {
	root := contextCommandRoot(t)
	const handoffUsage = contextHandoffUsage + "\n"
	for _, test := range []struct {
		run  func([]string) int
		args []string
	}{
		{runContextVerify, []string{"--root", root}},
		{runContextVerify, []string{"--nonce", "0000000000000000"}},
		{runContextHandoff, nil},
		{runContextHandoff, []string{"--root", root, "extra"}},
		{runContextPrune, []string{"--root", root, "extra"}},
		{runContextPrune, nil},
		{runContextHandoff, []string{"--root", root, "--scratch", "purpose=p,path=x,foo=bar"}},
		{runContextHandoff, []string{"--root", root, "--scratch", "purpose=p,path=x,required=1"}},
		{runContextHandoff, []string{"--root", root, "--scratch", "purpose=p,path=x,required="}},
		{runContextHandoff, []string{"--root", root, "--scratch", "purpose=,purpose=p,path=x"}},
		{runContextHandoff, []string{"--root", root, "--delegate", "id="}},
		{runContextHandoff, []string{"--root", root, "--delegate", "id=a,unknown=value"}},
		{runContextHandoff, []string{"--root", root, "--note", "/tmp/note", "--delegate", "id=a,asked=x,output=y", "--no-delegates"}},
		{runContextHandoff, []string{"--root", root, "--cancel", "0000000000000000", "--scratch", "purpose=p,path=x"}},
		{runContextHandoff, []string{"--root", root, "--cancel="}},
		{runContextHandoff, []string{"--root", root, "--by", "Wido"}},
		{runContextHandoff, []string{"--root", root, "--cancel", "0000000000000000", "--by="}},
		{runContextHandoff, []string{"--root", root, "--cancel", "0000000000000000", "--by", " "}},
		{runContextPrune, []string{"--root", root, "--older-than", "0"}},
		{runContextPrune, []string{"--root", root, "--older-than", "-1h"}},
		{runContextPrune, []string{"--root", root, "--older-than", "106752d"}},
		{runContextPrune, []string{"--root", root, "--older-than", "999999999999999999999d"}},
	} {
		code, _, problem := captureContextVerb(t, test.run, test.args...)
		byCase := strings.Contains(strings.Join(test.args, "\x00"), "--by")
		if code != 2 || (byCase && problem != handoffUsage) || (!byCase && !(strings.HasPrefix(problem, "usage: metasystem context ") || strings.Contains(problem, "--scratch must be") || strings.Contains(problem, "--delegate"))) {
			t.Fatalf("args %q = code %d stderr %q", test.args, code, problem)
		}
	}
}

type contextHandoffRecord struct{ nonce, state, digest string }

func recordContextHandoff(t *testing.T, root string) contextHandoffRecord {
	t.Helper()
	code, output, problem := captureContextVerb(t, runContextHandoff, "--root", root, "--note", contextHandoffNotePath(root), "--no-delegates")
	fields := strings.Fields(output)
	if code != 0 || problem != "" || len(fields) != 5 {
		t.Fatalf("record handoff = code %d stdout %q stderr %q", code, output, problem)
	}
	return contextHandoffRecord{fields[2], strings.TrimPrefix(fields[3], "state="), strings.TrimPrefix(fields[4], "sha256=")}
}

func captureContextVerb(t *testing.T, run func([]string) int, args ...string) (int, string, string) {
	t.Helper()
	return captureChannelOutput(t, func() int { return run(args) })
}

func contextHandoffCommandRoot(t *testing.T) (string, string) {
	t.Helper()
	container := t.TempDir()
	root := filepath.Join(container, "metasystem")
	writeTestingFixtureFile(t, filepath.Join(container, "development", "metasystem-design.md"), []byte("template\n"), 0o644)
	contextMust(t, os.MkdirAll(root, 0o755))
	seedClaimLaunchGoal(t, root)
	goalSyncMutationGit(t, root, "config", "metasystem.goal.machine", "m-test")
	noteDirectory := filepath.Join(root, ".handoff-memory")
	files := map[string]string{
		"metasystem.conf": "metasystem.runtimes=fake,claude,codex\ncontext.handoff.note-directory.fake=" + noteDirectory + "\nrole.steward-continuation.runtime=fake\nrole.steward-continuation.model.fake=fixture\n",
		"scripts/agents/roles/steward-continuation.md":                "# Role\n",
		"scripts/agents/roles/steward-continuation.requirements.json": "{\"required\":[]}\n",
		"scripts/agents/schemas/steward-continuation.schema.json":     "{\"type\":\"object\"}\n",
		"scripts/agents/permissions/workspace.json":                   "{\"write\":[\"workspace\"]}\n",
		"memory/receipts.log":                                         "",
		".handoff-memory/lessons.md":                                  "# Lessons\n\nContinue from the recorded state.\n",
	}
	for relative, body := range files {
		writeTestingFixtureFile(t, filepath.Join(root, filepath.FromSlash(relative)), []byte(body), 0o644)
	}
	modified := time.Date(2026, 9, 18, 11, 59, 0, 0, time.UTC)
	contextMust(t, os.Chtimes(filepath.Join(noteDirectory, "lessons.md"), modified, modified))
	canonicalRoot, err := filepath.EvalSymlinks(root)
	contextMust(t, err)
	contextMust(t, steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{
		RepoIdentity: canonicalRoot, Generation: 1, InstallPath: "/bin/true", MintedAt: time.Now().UTC().Format(time.RFC3339),
	}))
	return container, root
}

func contextHandoffNotePath(root string) string {
	if filepath.Base(filepath.Clean(root)) != "metasystem" {
		root = filepath.Join(root, "metasystem")
	}
	return filepath.Join(root, ".handoff-memory", "lessons.md")
}

func contextClaudeHandoffFixture(t *testing.T, root, session string, fixtures []string, noteBody string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	toplevel := contextHandoffToplevel(root)
	project := filepath.Join(home, ".claude", "projects", contextClaudeSlug(toplevel))
	if err := os.MkdirAll(filepath.Join(project, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	var transcript bytes.Buffer
	for _, fixture := range fixtures {
		data, err := os.ReadFile(filepath.Join("..", "..", "internal", "usage", "testdata", "tasks", fixture+".jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		transcript.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			transcript.WriteByte('\n')
		}
	}
	writeTestingFixtureFile(t, filepath.Join(project, session+".jsonl"), transcript.Bytes(), 0o600)
	note := filepath.Join(project, "memory", "lessons.md")
	writeContextHandoffNote(t, note, noteBody, time.Date(2026, 9, 18, 11, 59, 0, 0, time.UTC))
	return note
}

func contextClaudeSlug(path string) string {
	value := []byte(path)
	for index, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			continue
		}
		value[index] = '-'
	}
	return string(value)
}

func writeContextHandoffNote(t *testing.T, path, body string, modified time.Time) {
	t.Helper()
	writeTestingFixtureFile(t, path, []byte(body), 0o600)
	contextMust(t, os.Chtimes(path, modified, modified))
}

func readContextHandoffState(t *testing.T, path string) steward.HandoffState {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state steward.HandoffState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func contextMust(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func useContextHandoffIdentity(t *testing.T, root string) {
	useContextHandoffIdentityFor(t, root, "fake", "seat-session", 100)
}

func useContextHandoffIdentityFor(t *testing.T, root, runtimeName, session string, startedAt int64) {
	t.Helper()
	oldClassify, oldHolder, oldHook := classifyContextHandoffCaller, currentContextHandoffHolder, hookContextHandoffDelegate
	oldProof := proveSessionStopHuman
	oldNow := contextHandoffNow
	contextHandoffNow = func() time.Time { return time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC) }
	classifyContextHandoffCaller = func(stateRoot, installation string, _ int64) (lease.Classification, error) {
		if stateRoot != root || installation != root {
			t.Fatalf("handoff classified roots (%q, %q), want %q", stateRoot, installation, root)
		}
		announcement := &lease.Announcement{SessionId: session, MainId: "main-context", Pid: 4242,
			PidStartedAt: startedAt, PidStartTicks: 700, BootID: "boot-fixture", Runtime: runtimeName, InstanceTag: "main-tag"}
		return lease.Classification{Class: lease.ClassMain, MainId: announcement.MainId, Announcement: announcement}, nil
	}
	currentContextHandoffHolder = func(string) (lease.CurrentHolderView, error) {
		return lease.CurrentHolderView{MainId: "main-context", SessionId: session, ClaimEpoch: 7}, nil
	}
	hookContextHandoffDelegate = func(_, _ string, jobID string, _ int64) (lease.HookDelegateResult, error) {
		return lease.HookDelegateResult{Delegate: true, JobID: jobID}, nil
	}
	proveSessionStopHuman = func(string, int64, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{Outcome: humanauthority.OutcomeAgent}, errors.New("unexpected human proof")
	}
	t.Cleanup(func() {
		classifyContextHandoffCaller, currentContextHandoffHolder, hookContextHandoffDelegate = oldClassify, oldHolder, oldHook
		proveSessionStopHuman = oldProof
		contextHandoffNow = oldNow
	})
}

func contextCommandRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeContextCommandHolder(t *testing.T, root, runtimeName, session string) {
	t.Helper()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("cannot probe the context command test process: state=%s err=%v", state, err)
	}
	directory := filepath.Join(root, "artifacts", "agents", "mains")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "worktree-lease.json"), []byte(`{"holderMainId":"main-context"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	announcement := fmt.Sprintf(`{"sessionId":%q,"mainId":"main-context","runtime":%q,"pid":%d,"pidStartedAt":%d}`,
		session, runtimeName, exact.Pid, exact.StartedAt.Unix())
	if err := os.WriteFile(filepath.Join(directory, session+"-123.json"), []byte(announcement), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeContextCommandTranscript(t *testing.T, root, id string, tokens, ordinal int64, newline bool) string {
	t.Helper()
	path := filepath.Join(root, id+".jsonl")
	line := contextCommandLine(id, tokens, ordinal)
	if !newline {
		line = strings.TrimSuffix(line, "\n")
	}
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func contextCommandLine(id string, tokens, ordinal int64) string {
	return fmt.Sprintf(`{"type":"assistant","requestId":%q,"timestamp":"2026-09-13T11:00:00Z","message":{"usage":{"input_tokens":%d,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"ordinal":%d}`+"\n", id, tokens, ordinal)
}

func contextResponseID(index int) string {
	prefix := fmt.Sprintf("response-%04d-", index)
	return prefix + strings.Repeat("x", 55-len(prefix))
}

func seedContextCommandReading(t *testing.T, session string) (string, string) {
	t.Helper()
	root := contextCommandRoot(t)
	transcript := writeDerivedContextCommandTranscript(t, root, session, 120000, 1, true)
	code, _, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", session})
	})
	if code != 0 || problem != "" {
		t.Fatalf("seed status = code %d stderr %q", code, problem)
	}
	return root, transcript
}

func assertContextCommandError(t *testing.T, root, session, transcript, want string, evidencePath string, evidence []byte) {
	t.Helper()
	code, output, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", session})
	})
	if code != 1 || !strings.Contains(output, "context-budget=unknown") || !strings.Contains(output, want) || !strings.Contains(problem, want) {
		t.Fatalf("evidence failure = code %d stdout %q stderr %q", code, output, problem)
	}
	if got, err := os.ReadFile(evidencePath); err != nil || string(got) != string(evidence) {
		t.Fatalf("evidence changed at %s: bytes=%d err=%v", evidencePath, len(got), err)
	}
}

func writeDerivedContextCommandTranscript(t *testing.T, toplevel, session string, tokens, ordinal int64, newline bool) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	pathBytes := []byte(toplevel)
	for index, value := range pathBytes {
		if (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z') || (value >= '0' && value <= '9') {
			continue
		}
		pathBytes[index] = '-'
	}
	directory := filepath.Join(home, ".claude", "projects", string(pathBytes))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	return writeContextCommandTranscript(t, directory, session, tokens, ordinal, newline)
}
