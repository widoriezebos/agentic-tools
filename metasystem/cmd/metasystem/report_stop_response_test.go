package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport/stopreporttest"
)

func TestStopResponseResolvesUnderChangedWording(t *testing.T) {
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
		resolved, err := resolveStopResponseFile(root, payloadPath, runtime, "command")
		if err != nil || string(resolved.Report) != string(published.Report) {
			t.Fatalf("stop response resolved report=%q err=%v", resolved.Report, err)
		}
		resolved, err = resolveStopResponseFile(root, payloadPath, "", "")
		if err != nil || resolved.Response != published.Response {
			t.Fatalf("stop response resolved record=%+v err=%v", resolved.Response, err)
		}
	})
}
func TestStopResponseRefusesAnUnreadableResponse(t *testing.T) {
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
		resolved, err := resolveStopResponseFile(root, payloadPath, "", "")
		if err == nil || resolved.Report != nil || !strings.Contains(err.Error(), "Stop response is unreadable") || !strings.Contains(err.Error(), "report") {
			t.Fatalf("unreadable response resolved report=%q err=%v", resolved.Report, err)
		}
	})
}

// TestBedsResolveStopReportsThroughTheEngine: no script derives a report
// reference from console text. The fixture resolver it also pinned retired
// to the stopreport owner (verbs-object-action U7c).
func TestBedsResolveStopReportsThroughTheEngine(t *testing.T) {
	// These lines assert rendered wording or construct fixture bytes. None
	// derives a report reference from the visible text.
	allowed := map[string]string{}
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
			if !strings.Contains(line, "session status --id") && !strings.Contains(line, "stop-status --id") {
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

// resolveStopResponseFile is the owner path the retired `report
// stop-response` verb relayed: the installation's state root, then the
// immutable report the payload names (verbs-object-action U7c).
func resolveStopResponseFile(installation, payloadPath, runtime, session string) (stopreport.ResolvedResponse, error) {
	root, err := report.StopStatusRoot(installation)
	if err != nil {
		return stopreport.ResolvedResponse{}, err
	}
	payload, err := os.ReadFile(payloadPath)
	if err != nil {
		return stopreport.ResolvedResponse{}, err
	}
	return stopreport.ResolveResponse(root, payload, runtime, session)
}
