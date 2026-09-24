package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The name is a stable selector in retained testing contracts; the cases
// check routing by the lane's runtime and model settings.
func TestDesignAdapterFollowsTheResolvedModel(t *testing.T) {
	t.Parallel()

	// A lane runs on the agent its own runtime setting names. The model never
	// routes on its own: more than one agent can serve the same model, so a
	// Codex model on a Claude lane is a Claude launch of that model, and a
	// runtime this engine cannot launch is refused rather than guessed.
	cases := []struct {
		name        string
		kind        string
		useSettings bool
		runtime     string
		settings    string
		adapterData string
		model       string
		want        string
		refused     bool
	}{
		{name: "Codex from settings", kind: "design", useSettings: true, runtime: "codex", settings: "gpt-6-astra", want: "codex-exec"},
		{name: "Claude from settings", kind: "design", useSettings: true, runtime: "claude", settings: "claude-opus-5-5[1m]", want: "claude-headless"},
		{name: "the model alone never routes", kind: "design", useSettings: true, runtime: "claude", settings: "gpt-6-astra", want: "claude-headless"},
		{name: "adapter data changes the model, not the agent", kind: "design", useSettings: true, runtime: "claude", settings: "claude-opus-5-5[1m]", adapterData: "gpt-6-astra", want: "claude-headless"},
		{name: "spec changes the model, not the agent", kind: "design", useSettings: true, runtime: "codex", settings: "gpt-6-astra", model: "claude-opus-5-5[1m]", want: "codex-exec"},
		{name: "default design", kind: "design", want: "claude-headless"},
		{name: "read defaults to Codex", kind: "read", want: "codex-exec"},
		{name: "read on Claude by its setting", kind: "read", useSettings: true, runtime: "claude", settings: "claude-opus-5-5[1m]", want: "claude-headless"},
		{name: "an agent this engine cannot launch is refused", kind: "design", useSettings: true, runtime: "devin", settings: "claude-opus-5-5", refused: true},
	}
	for index, row := range cases {
		row := row
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			m, _, _, _ := manager(t)
			m.Supervisor = childStarter(m)
			m.Adapters["claude-headless"] = fakeAdapter{}
			if row.useSettings {
				m.Settings = DefaultSettings()
				if row.kind == "read" {
					m.Settings.ReadModel, m.Settings.ReadRuntime = row.settings, row.runtime
				} else {
					m.Settings.DesignModel, m.Settings.DesignRuntime = row.settings, row.runtime
				}
			}
			data := map[string]json.RawMessage{}
			if row.adapterData != "" {
				setString(data, "model", row.adapterData)
			}
			spec := StartSpec{
				ID:               "design-runtime-" + string(rune('a'+index)),
				Kind:             row.kind,
				Brief:            brief(t),
				WorkingDirectory: t.TempDir(),
				Model:            row.model,
				AdapterData:      data,
			}
			if row.kind == "read" {
				spec.DiffFile = writeLaunchFile(t, "change.diff", "")
			}
			record, err := m.Start(spec)
			if row.refused {
				if err == nil {
					t.Fatalf("a lane on an agent this engine cannot launch started: %+v", record)
				}
				return
			}
			if err != nil || record.Adapter != row.want {
				t.Fatalf("adapter=%q, want %q, err=%v", record.Adapter, row.want, err)
			}
		})
	}
}

func TestSolReadCommandAndCollectedVerdict(t *testing.T) {
	t.Parallel()
	for _, compacted := range []bool{false, true} {
		t.Run(map[bool]string{false: "intact", true: "compacted"}[compacted], func(t *testing.T) {
			m, processes, _, _ := manager(t)
			workingDirectory, sessions := t.TempDir(), t.TempDir()
			briefPath := filepath.Join(workingDirectory, "brief.md")
			diffPath := filepath.Join(workingDirectory, "read.diff")
			reportPath := filepath.Join(workingDirectory, "read.md")
			for path, content := range map[string]string{
				briefPath: "Review the unit.\n", diffPath: "diff --git a/a b/a\n", reportPath: "VERDICT: stale\n",
			} {
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			data := map[string]json.RawMessage{}
			setString(data, "brief", briefPath)
			setString(data, "readDiff", diffPath)
			setString(data, "model", "gpt-6-sol")
			setString(data, "effort", "xhigh")
			setInt64(data, "window", 0)
			setStrings(data, "declaredOutputs", []string{"read.md"})
			record := Record{ID: "sol-read", Kind: "read", Adapter: "codex-exec", WorkingDirectory: workingDirectory,
				StartedAt: m.Now().Format(time.RFC3339Nano), State: Starting, AdapterData: data}
			if err := m.Store.Create(record); err != nil {
				t.Fatal(err)
			}
			stateDir, err := m.Store.StateDir(record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stateDir, "exec.log"), []byte("session id: sol-read-session\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stateDir, "last-message.txt"), []byte("Read finished.\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(sessions, "2026", "09", "17")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			rollout := `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100},"total_token_usage":{"input_tokens":100,"output_tokens":20}}}}` + "\n"
			if compacted {
				rollout += `{"type":"compacted","payload":{}}` + "\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "rollout-sol-read-session.jsonl"), []byte(rollout), 0o600); err != nil {
				t.Fatal(err)
			}
			m.Adapters["codex-exec"] = CodexExec{Binary: "codex", SessionsRoot: sessions, Now: m.Now, Location: time.UTC}
			processes.onStart = func(Command) {
				if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
					t.Errorf("prior report still present when child starts: %v", err)
				}
				if err := os.WriteFile(reportPath, []byte("VERDICT: pass\n"), 0o600); err != nil {
					t.Error(err)
				}
			}
			got, err := m.Supervise(record.ID)
			if err != nil || got.State != Completed || got.Measurement.Verdict != "VERDICT: pass" || got.VerdictCounts == nil || *got.VerdictCounts == compacted || got.Measurement.Compactions != map[bool]int{false: 0, true: 1}[compacted] {
				t.Fatalf("collected read=%+v err=%v", got, err)
			}
			if processes.command.Program != "codex" || !strings.Contains(processes.command.Stdin, "Diff: "+diffPath) || !strings.Contains(processes.command.Stdin, "Review the unit.") ||
				!reflect.DeepEqual(processes.command.Args[0:4], []string{"exec", "-m", "gpt-6-sol", "-c"}) || strings.Contains(strings.Join(processes.command.Args, " "), "model_auto_compact_token_limit") {
				t.Fatalf("Codex read command=%+v", processes.command)
			}
			if len(got.Outputs) != 1 || !strings.HasSuffix(got.Outputs[0].Path, "read.md") {
				t.Fatalf("read output was not collected: %+v", got.Outputs)
			}
		})
	}
}

func TestSolReadRejectsAnUnchangedPriorVerdict(t *testing.T) {
	t.Parallel()
	m, processes, _, _ := manager(t)
	workingDirectory, sessions := t.TempDir(), t.TempDir()
	reportPath := filepath.Join(workingDirectory, "read.md")
	if err := os.WriteFile(reportPath, []byte("VERDICT: land\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data := map[string]json.RawMessage{}
	setString(data, "brief", writeLaunchFile(t, "brief.md", "Review the unit.\n"))
	setString(data, "model", "gpt-6-sol")
	setStrings(data, "declaredOutputs", []string{reportPath})
	record := Record{ID: "stale-read", Kind: "read", Adapter: "codex-exec", WorkingDirectory: workingDirectory,
		StartedAt: m.Now().Format(time.RFC3339Nano), State: Starting, AdapterData: data}
	if err := m.Store.Create(record); err != nil {
		t.Fatal(err)
	}
	stateDir, err := m.Store.StateDir(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "exec.log"), []byte("session id: stale-read-session\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "last-message.txt"), []byte("Read finished.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(sessions, "2026", "09", "17")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-stale-read-session.jsonl"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	processes.onStart = func(Command) {
		if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
			t.Errorf("prior report still present when child starts: %v", err)
		}
	}
	m.Adapters["codex-exec"] = CodexExec{Binary: "codex", SessionsRoot: sessions, Now: m.Now, Location: time.UTC}
	got, err := m.Supervise(record.ID)
	if err != nil || got.State != Failed || got.VerdictIsCounting() || got.Measurement.Verdict == "VERDICT: land" || len(got.Outputs) != 0 || !strings.Contains(got.Reason, "declared-output") {
		t.Fatalf("stale report counted: record=%+v err=%v", got, err)
	}
	prior, readErr := os.ReadFile(filepath.Join(stateDir, "previous-outputs", "0-read.md"))
	if readErr != nil || string(prior) != "VERDICT: land\n" {
		t.Fatalf("prior report was not preserved: %q err=%v", prior, readErr)
	}
}

func TestDesignStartRefusesAMissingResolvedAdapterBeforeSpawning(t *testing.T) {
	t.Parallel()

	m, _, _, _ := manager(t)
	delete(m.Adapters, "codex-exec")
	starts := 0
	m.Supervisor = fakeStarter{start: func(string) { starts++ }}
	_, err := m.Start(StartSpec{
		ID:               "missing-design-adapter",
		Kind:             "design",
		Brief:            brief(t),
		WorkingDirectory: t.TempDir(),
		Model:            "gpt-6-astra",
	})
	if err == nil || err.Error() != "adapter-unavailable" || starts != 0 {
		t.Fatalf("error=%v supervisor starts=%d", err, starts)
	}
}

func TestCodexDesignCommandUsesTheWorkingDirectory(t *testing.T) {
	t.Parallel()

	workingDirectory := t.TempDir()
	briefPath := filepath.Join(workingDirectory, "brief.md")
	if err := os.WriteFile(briefPath, []byte("write the design\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data := map[string]json.RawMessage{}
	setString(data, "brief", briefPath)
	setInt64(data, "window", 200000)
	command, err := (CodexExec{Binary: "codex"}).Command(Record{
		Kind:             "design",
		WorkingDirectory: workingDirectory,
		AdapterData:      data,
	}, t.TempDir())
	if err != nil || command.Directory != workingDirectory {
		t.Fatalf("directory=%q, want %q, err=%v", command.Directory, workingDirectory, err)
	}
}

func TestCodexDesignMeasureCopiesThePage(t *testing.T) {
	t.Parallel()

	state, sessions, workingDirectory := t.TempDir(), t.TempDir(), t.TempDir()
	page := filepath.Join(workingDirectory, "design.md")
	want := "one two\nthree\n"
	if err := os.WriteFile(page, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	sessionID := "design-session"
	if err := os.WriteFile(filepath.Join(state, "exec.log"), []byte("session id: "+sessionID+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	rolloutDirectory := filepath.Join(sessions, "2026", "09", "19")
	if err := os.MkdirAll(rolloutDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rolloutDirectory, "rollout-"+sessionID+".jsonl"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	data := map[string]json.RawMessage{}
	setString(data, "page", "design.md")
	measurement, outputs, _, err := (CodexExec{
		SessionsRoot: sessions,
		Now:          func() time.Time { return day },
	}).Measure(Record{Kind: "design", WorkingDirectory: workingDirectory, AdapterData: data}, state)
	if err != nil || measurement.PageLines != 2 || measurement.PageWords != 3 || measurement.PageMissing || len(outputs) != 1 {
		t.Fatalf("measurement=%+v outputs=%+v err=%v", measurement, outputs, err)
	}
	dataOut, readErr := os.ReadFile(outputs[0].Path)
	if readErr != nil || string(dataOut) != want || !strings.HasPrefix(outputs[0].Path, filepath.Join(state, "page")+string(os.PathSeparator)) {
		t.Fatalf("output=%+v content=%q err=%v", outputs[0], dataOut, readErr)
	}
}
