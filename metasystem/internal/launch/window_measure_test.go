package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexCommandAppliesRecordedAutoCompactWindow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	briefPath := filepath.Join(root, "brief.md")
	if err := os.WriteFile(briefPath, []byte("build it\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data := map[string]json.RawMessage{}
	setString(data, "brief", briefPath)
	setInt64(data, "window", 187654)
	command, err := (CodexExec{Binary: "codex"}).Command(Record{
		Kind: "build", WorkingDirectory: root, AdapterData: data,
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	wantTail := []string{"-c", "model_auto_compact_token_limit=187654", "-"}
	if len(command.Args) < len(wantTail) || strings.Join(command.Args[len(command.Args)-len(wantTail):], "\x00") != strings.Join(wantTail, "\x00") {
		t.Fatalf("argv=%q", command.Args)
	}
	joinedArgs := strings.Join(command.Args, " ")
	joinedEnvironment := strings.Join(command.Environment, " ")
	if strings.Contains(joinedArgs, "model_context_window") || strings.Contains(joinedEnvironment, "CODEX_CONTEXT_WINDOW") {
		t.Fatalf("argv=%q environment=%q", command.Args, command.Environment)
	}
}

func TestCommandsRequirePositiveRecordedWindow(t *testing.T) {
	t.Parallel()

	for _, adapter := range []struct {
		name    string
		command func(Record, string) (Command, error)
	}{
		{name: "codex", command: (CodexExec{}).Command},
		{name: "claude", command: (ClaudeHeadless{}).Command},
	} {
		adapter := adapter
		t.Run(adapter.name, func(t *testing.T) {
			t.Parallel()
			briefPath := filepath.Join(t.TempDir(), "brief.md")
			if err := os.WriteFile(briefPath, []byte("work\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, window := range []int64{0, -1} {
				data := map[string]json.RawMessage{}
				setString(data, "brief", briefPath)
				if window < 0 {
					setInt64(data, "window", window)
				}
				_, err := adapter.command(Record{Kind: "build", WorkingDirectory: t.TempDir(), AdapterData: data}, t.TempDir())
				if err == nil || !strings.Contains(err.Error(), `"window"`) {
					t.Fatalf("window=%d error=%v", window, err)
				}
			}
		})
	}
}

func TestCodexMeasureFindsRolloutOnNextLocalDay(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("fixture", 2*60*60)
	started := time.Date(2026, 9, 18, 23, 50, 0, 0, location)
	now := time.Date(2026, 9, 19, 0, 10, 0, 0, location)
	state, sessions := t.TempDir(), t.TempDir()
	sessionID := "cross-midnight"
	if err := os.WriteFile(filepath.Join(state, "exec.log"), []byte("session id: "+sessionID+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rolloutDirectory := filepath.Join(sessions, "2026", "09", "19")
	if err := os.MkdirAll(rolloutDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	row := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":42},"total_token_usage":{"input_tokens":42}}}}` + "\n"
	if err := os.WriteFile(filepath.Join(rolloutDirectory, "rollout-"+sessionID+".jsonl"), []byte(row), 0o600); err != nil {
		t.Fatal(err)
	}
	measurement, _, _, err := (CodexExec{
		SessionsRoot: sessions,
		Now:          func() time.Time { return now },
		Location:     location,
	}).Measure(Record{Kind: "build", StartedAt: started.Format(time.RFC3339Nano)}, state)
	if err != nil || measurement.Calls != 1 || measurement.PeakContext != 42 {
		t.Fatalf("measurement=%+v error=%v", measurement, err)
	}
}

func TestMissingRolloutIsVisible(t *testing.T) {
	t.Parallel()

	m, _, _, _ := manager(t)
	record := seed(t, m, "missing-rollout", Starting)
	state, _ := m.Store.StateDir(record.ID)
	briefPath := filepath.Join(record.WorkingDirectory, "brief.md")
	if err := os.WriteFile(briefPath, []byte("build\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Store.Update(record.ID, func(current *Record) error {
		current.Measured = true
		setString(current.AdapterData, "brief", briefPath)
		setInt64(current.AdapterData, "window", 200000)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "exec.log"), []byte("session id: absent\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.Adapters["codex-exec"] = CodexExec{
		SessionsRoot: t.TempDir(), Now: m.Now, Location: time.UTC,
	}
	got, err := m.Supervise(record.ID)
	if err != nil || got.State != Completed || got.Measured || !strings.Contains(got.Reason, "found 0") || strings.Contains(got.Reason, "%!w") {
		t.Fatalf("record=%+v error=%v", got, err)
	}
	if line := RecordLine(got, m.Store.Root); !strings.Contains(line, "measured=false") {
		t.Fatalf("status line=%q", line)
	}
	report, err := m.Report("")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Kinds) == 0 || report.Kinds[0].Kind != "build" || report.Kinds[0].Unmeasured != 1 || !strings.Contains(strings.Join(report.Lines(), "\n"), "unmeasured=1") {
		t.Fatalf("report=%+v lines=%q", report, report.Lines())
	}
}

func TestMissingClaudeTranscriptIsVisible(t *testing.T) {
	t.Parallel()

	m, _, _, _ := manager(t)
	record, state := seedClaude(t, m, "missing-transcript")
	if _, err := m.Store.Update(record.ID, func(current *Record) error {
		current.Measured = true
		setInt64(current.AdapterData, "window", 400000)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	writeClaudeResult(t, state, `{"session_id":"absent","is_error":false,"result":"done"}`)
	m.Adapters["claude-headless"] = ClaudeHeadless{ProjectsRoot: t.TempDir()}
	got, err := m.Supervise(record.ID)
	if err != nil || got.State != Completed || got.Measured || !strings.Contains(got.Reason, "transcript") {
		t.Fatalf("record=%+v error=%v", got, err)
	}
}

func TestRecordWithoutMeasuredFieldDefaultsToMeasured(t *testing.T) {
	t.Parallel()

	var legacy Record
	if err := json.Unmarshal([]byte(`{"id":"legacy"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if !legacy.Measured {
		t.Fatalf("legacy record=%+v", legacy)
	}
	var unmeasured Record
	if err := json.Unmarshal([]byte(`{"id":"new","measured":false}`), &unmeasured); err != nil {
		t.Fatal(err)
	}
	if unmeasured.Measured {
		t.Fatalf("unmeasured record=%+v", unmeasured)
	}
}

func TestBriefAdmissionSettingExplainsReservedWorkingRoom(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# The 200000-token seat window reserves about 80000 tokens for tool output and the diff, leaving 120000 for the brief and its inputs.\n" +
		"launch.brief.admitted.tokens=120000"
	if !strings.Contains(string(data), want) {
		t.Fatalf("metasystem.conf does not contain the setting basis directly above the setting")
	}
}
