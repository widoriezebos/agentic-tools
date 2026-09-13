package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

func TestContextStatusVerbPrintsTheRoleLine(t *testing.T) {
	root := contextCommandRoot(t)
	writeDerivedContextCommandTranscript(t, root, "inferred", 150001, 1, true)
	writeContextCommandHolder(t, root, "claude", "inferred")

	code, text, problem := captureChannelOutput(t, func() int {
		return dispatch([]string{"context", "status", "--root", root})
	})
	if code != 0 || problem != "" || !strings.HasPrefix(text, "context-budget=alive (150 thousand tokens this call, bound 150, ceiling 200; over the bound:") {
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
	if decoded.Diagnostic || decoded.Role.Line()+"\n" != text || decoded.Reading.Latest == nil || decoded.Reading.Latest.PromptTokens != 150001 {
		t.Fatalf("text/JSON evaluations diverged: text=%q JSON=%+v", text, decoded)
	}

	explicitRoot := contextCommandRoot(t)
	explicitTranscript := writeContextCommandTranscript(t, explicitRoot, "explicit", 120000, 1, true)
	code, explicit, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", explicitRoot, "--runtime", "claude", "--session", "explicit", "--transcript", explicitTranscript})
	})
	if code != 0 || problem != "" || explicit != "context-budget=alive (diagnostic transcript override; 120 thousand tokens this call, bound 150, ceiling 200)\n" {
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
	if code != 0 || problem != "" || !strings.HasPrefix(output, "context-budget=dead (diagnostic transcript override; 200 thousand tokens this call is over the ceiling 200") {
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
	if code != 0 || problem != "" || emptyText != "context-budget=alive (diagnostic transcript override; unknown (no call recorded yet))\n" {
		t.Fatalf("empty diagnostic = code %d stdout %q stderr %q", code, emptyText, problem)
	}

	overBound := writeContextCommandTranscript(t, root, "over-bound", 160000, 1, true)
	code, overBoundText, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", "over-bound", "--transcript", overBound})
	})
	wantOverBound := "context-budget=alive (diagnostic transcript override; 160 thousand tokens this call, bound 150, ceiling 200; over the bound)\n"
	if code != 0 || problem != "" || overBoundText != wantOverBound || strings.Contains(overBoundText, "handoff") {
		t.Fatalf("over-bound diagnostic = code %d stdout %q stderr %q", code, overBoundText, problem)
	}
	code, structured, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", "over-bound", "--transcript", overBound, "--json"})
	})
	var decoded contextStatusOutput
	if err := json.Unmarshal([]byte(structured), &decoded); err != nil {
		t.Fatal(err)
	}
	if code != 0 || problem != "" || !decoded.Diagnostic || decoded.Role.Line()+"\n" != overBoundText ||
		strings.Contains(structured, `"seen"`) || strings.Contains(structured, `"tail"`) {
		t.Fatalf("structured diagnostic = code %d stdout %q stderr %q decoded=%+v", code, structured, problem, decoded)
	}

	overCeiling := writeContextCommandTranscript(t, root, "over-ceiling", 200001, 1, true)
	code, ceilingText, problem := captureChannelOutput(t, func() int {
		return runContextStatus([]string{"--root", root, "--runtime", "claude", "--session", "over-ceiling", "--transcript", overCeiling})
	})
	if code != 0 || problem != "" || !strings.HasPrefix(ceilingText, "context-budget=dead (diagnostic transcript override; 200 thousand tokens this call is over the ceiling 200") ||
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
