package main

import (
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport/stopreporttest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportStopResponseResolvesUnderChangedWording(t *testing.T) {
	forStopResponseCases(t, func(t *testing.T, runtime string, blocked bool) {
		root := stopResponseCommandRoot(t, runtime)
		published := stopreporttest.Publish(t, stopreporttest.Options{
			Root: root, Runtime: runtime, Session: "command", Attempt: strings.Repeat("c", 32),
			ShouldBlock: blocked, HumanLine: stopreporttest.ChangedHumanLine,
		})
		payloadPath := filepath.Join(t.TempDir(), "payload.json")
		if err := os.WriteFile(payloadPath, append(published.Payload, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
			return runReportStopResponse([]string{"--root", root, "--payload-file", payloadPath, "--runtime", runtime, "--session", "command"})
		})
		if code != 0 || stderr != "" || stdout != string(published.Report) {
			t.Fatalf("stop-response returned code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
			return runReportStopResponse([]string{"--root", root, "--payload-file", payloadPath, "--json"})
		})
		var response stopreport.Response
		if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &response) != nil || response != published.Response {
			t.Fatalf("stop-response --json returned code=%d response=%+v stderr=%q", code, response, stderr)
		}
	})
}
func TestReportStopResponseRefusesAnUnreadableResponse(t *testing.T) {
	forStopResponseCases(t, func(t *testing.T, runtime string, blocked bool) {
		root := stopResponseCommandRoot(t, runtime)
		published := stopreporttest.Publish(t, stopreporttest.Options{
			Root: root, Runtime: runtime, Session: "unreadable", Attempt: strings.Repeat("d", 32),
			ShouldBlock: blocked, HumanLine: stopreporttest.ChangedHumanLine,
		})
		var record map[string]any
		data, err := os.ReadFile(published.ResponsePath)
		if err != nil || json.Unmarshal(data, &record) != nil {
			t.Fatalf("read response fixture: %v", err)
		}
		delete(record, "report")
		data, _ = json.Marshal(record)
		if err := os.WriteFile(published.ResponsePath, data, 0o600); err != nil {
			t.Fatal(err)
		}
		payloadPath := filepath.Join(t.TempDir(), "payload.json")
		if err := os.WriteFile(payloadPath, published.Payload, 0o600); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
			return runReportStopResponse([]string{"--root", root, "--payload-file", payloadPath})
		})
		if code == 0 || stdout != "" || strings.Count(stderr, "\n") != 1 || !strings.Contains(stderr, "Stop response is unreadable") || !strings.Contains(stderr, "report") {
			t.Fatalf("unreadable response returned code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
}
func TestBedsResolveStopReportsThroughTheEngine(t *testing.T) {
	resolverPath := filepath.Join("..", "..", "scripts", "agents", "fixture-stop-report.sh")
	resolver, err := os.ReadFile(resolverPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, wording := range []string{"Stop allowed", "Stop blocked", "Report: ", "Do not stop"} {
		if strings.Contains(string(resolver), wording) {
			t.Errorf("fixture resolver still interprets wording %q", wording)
		}
	}
	if !strings.Contains(string(resolver), "report stop-response") {
		t.Error("fixture resolver does not call report stop-response")
	}

	// These lines assert rendered wording or construct fixture bytes. None
	// derives a report reference from the visible text.
	allowed := map[string]string{
		`scripts/agents/supervision-hook-fixtures.sh` + "\x00" + `&& [[ "$name_block_task" == 'Task: stop refusal fits on one screen; Stop blocked; needs your decision and supervision repair; Do not stop. Run this command; read and act on its report: metasystem report stop-status --id '* ]] \`: "wording assertion",
		`scripts/agents/supervision-hook-fixtures.sh` + "\x00" + `&& [[ "$name_allow_task" == 'Task: stop refusal fits on one screen; Stop allowed; needs supervision repair; Report: metasystem report stop-status --id '* ]] \`:                                                                      "wording assertion",
		`scripts/agents/health-fixtures.sh` + "\x00" + `hook_visible=$(printf 'Just completed: unknown for this turn.\nNo task in flight; Stop allowed; Report: metasystem report stop-status --id %s' "$hook_report_alias")`:                                                                          "fixture report text",
		`scripts/agents/health-fixtures.sh` + "\x00" + `hook_payload=$(printf '{"systemMessage":"Just completed: unknown for this turn.\\nNo task in flight; Stop allowed; Report: metasystem report stop-status --id %s"}' "$hook_report_alias")`:                                                     "fixture payload text",
		`scripts/agents/supervision-fixtures.sh` + "\x00" + `&& grep -Fq '; Stop allowed; needs supervision repair; Report: metasystem report stop-status --id ' <<<"$degraded" \`:                                                                                                                     "wording assertion",
	}
	paths, err := filepath.Glob(filepath.Join("..", "..", "scripts", "agents", "*.sh"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, sourceLine := range strings.Split(string(data), "\n") {
			line := strings.TrimSpace(sourceLine)
			if !strings.Contains(line, "stop-status --id") {
				continue
			}
			key := filepath.ToSlash(filepath.Join("scripts", "agents", filepath.Base(path))) + "\x00" + line
			if _, ok := allowed[key]; !ok {
				t.Errorf("%s extracts or newly consumes a report reference from console text", key)
			} else {
				seen[key] = true
			}
		}
	}
	for line, reason := range allowed {
		if !seen[line] {
			t.Errorf("allowed %s line is missing: %s", reason, line)
		}
	}
}

func stopResponseCommandRoot(t *testing.T, runtime string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "installation")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes="+runtime+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
func forStopResponseCases(t *testing.T, run func(*testing.T, string, bool)) {
	t.Helper()
	for _, runtime := range []string{"claude", "codex", "devin"} {
		for _, blocked := range []bool{false, true} {
			t.Run(runtime+map[bool]string{false: "/allowed", true: "/blocked"}[blocked], func(t *testing.T) {
				run(t, runtime, blocked)
			})
		}
	}
}
