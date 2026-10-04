package launch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type causeAdapter struct{ fakeAdapter }

func (causeAdapter) StopCause(Record, string) string { return "provider-limit" }

func TestLaunchNamesItsEnvironmentCause(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, want string }{
		{"supervisor-lost", "process-lost"}, {"supervisor-lost-before-child", "process-lost"},
		{"output-busy", "output-busy"}, {"provider-limit", "provider-limit"}, {"legacy", ""}, {"completed", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			m, processes, probe, _ := manager(t)
			var got Record
			var err error
			switch row.name {
			case "supervisor-lost-before-child":
				probe.states[10] = identity.Dead
				m.Supervisor = fakeStarter{func(string) {}}
				got, err = m.Start(StartSpec{ID: "cause", Kind: "build", Brief: brief(t), WorkingDirectory: t.TempDir()})
			case "supervisor-lost":
				seed(t, m, "cause", Running)
				probe.states[10], probe.states[20] = identity.Dead, identity.Dead
				got, err = m.Status("cause")
			case "output-busy":
				record := seed(t, m, "cause", Starting)
				setStrings(record.AdapterData, "declaredOutputs", []string{filepath.Join(t.TempDir(), "read.md")})
				_, err = m.Store.Update(record.ID, func(r *Record) error { r.AdapterData = record.AdapterData; return nil })
				require(t, err != nil, "update: %v", err)
				stateDir, _ := m.Store.StateDir(record.ID)
				release, lockErr := m.prepareDeclaredOutputs(record, stateDir)
				require(t, lockErr != nil, "lock: %v", lockErr)
				defer release()
				got, err = m.Supervise(record.ID)
			default:
				seed(t, m, "cause", Starting)
				processes.exit = 1
				if row.name != "legacy" {
					m.Adapters["codex-exec"] = causeAdapter{}
				}
				if row.name == "completed" {
					processes.exit = 0
				}
				got, err = m.Supervise("cause")
			}
			wantState := Failed
			if row.name == "completed" {
				wantState = Completed
			}
			require(t, got.Cause != row.want || got.State != wantState, "record=%+v err=%v", got, err)
			stored, readErr := m.Store.Read(got.ID)
			require(t, readErr != nil || stored.Cause != row.want, "stored=%+v err=%v", stored, readErr)
		})
	}
}

func TestClaudeStopCauseNamesProviderLimit(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	for _, name := range []string{"stderr.log", "result.json"} {
		data := "Claude AI usage limit reached|1759262400"
		if name == "result.json" {
			data = `{"is_error":true,"result":"API Error: 529 overloaded_error"}`
		}
		require(t, os.WriteFile(filepath.Join(state, name), []byte(data), 0o600) != nil, "write %s", name)
		require(t, (ClaudeHeadless{}).StopCause(Record{}, state) != "provider-limit", "no cause from %s", name)
		require(t, os.Remove(filepath.Join(state, name)) != nil, "remove %s", name)
	}
}

func TestCodexStopCauseNamesProviderLimit(t *testing.T) {
	t.Parallel()
	state := t.TempDir()
	require(t, os.WriteFile(filepath.Join(state, "exec.log"), []byte("You've hit your usage limit"), 0o600) != nil, "write log")
	require(t, (CodexExec{}).StopCause(Record{}, state) != "provider-limit", "no cause from log")
}
