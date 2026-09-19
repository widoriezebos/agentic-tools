package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDesignAdapterFollowsTheResolvedModel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		kind        string
		useSettings bool
		settings    string
		adapterData string
		model       string
		want        string
	}{
		{name: "Codex from settings", kind: "design", useSettings: true, settings: "gpt-6-astra", want: "codex-exec"},
		{name: "Claude from settings", kind: "design", useSettings: true, settings: "claude-opus-5", want: "claude-headless"},
		{name: "empty model", kind: "design", useSettings: true, want: "claude-headless"},
		{name: "default design", kind: "design", want: "claude-headless"},
		{name: "adapter data overrides settings", kind: "design", useSettings: true, settings: "claude-opus-5", adapterData: "gpt-6-astra", want: "codex-exec"},
		{name: "spec overrides adapter data", kind: "design", useSettings: true, settings: "gpt-6-astra", adapterData: "gpt-6-astra", model: "claude-opus-5", want: "claude-headless"},
		{name: "read stays Claude", kind: "read", model: "gpt-6-astra", want: "claude-headless"},
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
				m.Settings.DesignModel = row.settings
			}
			data := map[string]json.RawMessage{}
			if row.adapterData != "" {
				setString(data, "model", row.adapterData)
			}
			record, err := m.Start(StartSpec{
				ID:               "design-runtime-" + string(rune('a'+index)),
				Kind:             row.kind,
				Brief:            brief(t),
				WorkingDirectory: t.TempDir(),
				Model:            row.model,
				AdapterData:      data,
			})
			if err != nil || record.Adapter != row.want {
				t.Fatalf("adapter=%q, want %q, err=%v", record.Adapter, row.want, err)
			}
		})
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
