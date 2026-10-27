package launch

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type causeAdapter struct{ fakeAdapter }

func (causeAdapter) StopCause(Record, string) (string, string) {
	return "provider-limit", "fixture words"
}

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
			wantWords := ""
			if row.name == "provider-limit" {
				wantWords = "fixture words"
			}
			require(t, readErr != nil || stored.Cause != row.want || stored.CauseWords != wantWords, "stored=%+v err=%v", stored, readErr)
		})
	}
}

const capacityWords = "ERROR: Selected model is at capacity. Please try a different model."

func TestClaudeStopCauseNamesProviderLimit(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, data, words string }{
		{"stderr.log", "Claude AI usage limit reached|1759262400", "Claude AI usage limit reached|1759262400"},
		{"result.json", `{"is_error":true,"result":"API Error: 529 overloaded_error"}`, "API Error: 529 overloaded_error"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			state := t.TempDir()
			require(t, os.WriteFile(filepath.Join(state, row.name), []byte(row.data), 0600) != nil, "write %s", row.name)
			cause, words := (ClaudeHeadless{}).StopCause(Record{}, state)
			require(t, cause != "provider-limit" || words != row.words, "cause=%q words=%q, want %q", cause, words, row.words)
		})
	}
}

func TestCodexStopCauseNamesProviderLimit(t *testing.T) {
	t.Parallel()
	fixture, err := os.ReadFile("testdata/codex-capacity-exec.log")
	require(t, err != nil, "fixture: %v", err)
	for _, row := range []struct{ name, log, words string }{
		{"usage limit", "You've hit your usage limit", "You've hit your usage limit"},
		{"capacity", string(fixture), capacityWords},
		{"after work", strings.Repeat("tool output\n", 200) + "\n" + capacityWords + "\n" + capacityWords + "\ntokens used\n211,438\n", capacityWords},
		{"quoted near end", "tool output from a file:\n" + capacityWords + "\nERROR: failed to write final output: permission denied\ntokens used\n24,307\n", ""},
		{"echoed early", capacityWords + "\n" + strings.Repeat("tool output\n", 40) + "tokens used\n24,307\n", ""},
		{"other error", "ERROR: failed to write final output: permission denied\ntokens used\n24,307\n", ""},
		{"empty", "", ""},
		{"without footer", "tool output\n\n" + capacityWords + "\n", capacityWords},
		{"indented quote", " " + capacityWords + "\ntokens used\n24,307\n", ""},
		{"missing", "", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			state := t.TempDir()
			if row.name != "missing" {
				require(t, os.WriteFile(filepath.Join(state, "exec.log"), []byte(row.log), 0600) != nil, "write log")
			}
			cause, words := (CodexExec{}).StopCause(Record{}, state)
			want := ""
			if row.words != "" {
				want = "provider-limit"
			}
			require(t, cause != want || words != row.words, "cause=%q words=%q, want %q %q", cause, words, want, row.words)
		})
	}
}

func TestProviderMessageIsShortAndPlain(t *testing.T) {
	t.Parallel()
	// internal/channel/question.go's unplainWords rejects these in contract questions.
	unplain := regexp.MustCompile(`[0-9a-fA-F]{10}|/Users/`)
	for _, row := range []struct{ name, words, want string }{
		{"capacity", capacityWords, "Selected model is at capacity. Please try a different model."},
		{"reset", "Claude AI usage limit reached|1759262400", "Claude AI usage limit reached"},
		{"timestamp", "ERROR: unavailable at 2026-10-26T06:35:09.123+02:00 request 0123456789abcdef try later", "unavailable at request try later"},
		{"long", strings.Repeat("word ", 80), strings.TrimSpace(strings.Repeat("word ", 20))},
		{"unicode", strings.Repeat("café ", 80), strings.TrimSpace(strings.Repeat("café ", 20))},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got := providerMessage(row.words)
			require(t, got != row.want || utf8.RuneCountInString(got) > 100 || unplain.MatchString(got), "message=%q, want %q", got, row.want)
		})
	}
}
