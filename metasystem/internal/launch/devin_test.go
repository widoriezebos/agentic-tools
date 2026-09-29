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

func devinRecord(t *testing.T, kind string) (Record, string) {
	t.Helper()
	root := t.TempDir()
	briefPath := filepath.Join(root, "brief.md")
	if err := os.WriteFile(briefPath, []byte("brief text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data := map[string]json.RawMessage{"brief": rawString(briefPath), "model": rawString("claude-opus-5-5"), "effort": rawString("xhigh")}
	setInt64(data, "window", 0)
	return Record{ID: "devin-" + kind, Kind: kind, Tag: "alpha", WorkingDirectory: root, AdapterData: data}, root
}

// A Devin lane runs print mode from a prompt file holding the brief and the
// read packet, with the lane's effort folded into the model id.
func TestDevinPrintCommand(t *testing.T) {
	t.Parallel()
	record, root := devinRecord(t, "read")
	setString(record.AdapterData, "readDiff", "/diffs/read.diff")
	setStrings(record.AdapterData, "declaredOutputs", []string{"read.md"})
	setString(record.AdapterData, "resumeSession", "mica-friction")
	state := t.TempDir()
	command, err := (DevinPrint{Binary: "fake-devin"}).Command(record, state)
	if err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(state, "prompt.md")
	want := []string{"-p", "--prompt-file", prompt, "--respect-workspace-trust", "false", "--model", "claude-opus-5-5-xhigh",
		"--permission-mode", "dangerous", "--export", filepath.Join(state, "transcript.json"), "-r", "mica-friction"}
	require(t, command.Program != "fake-devin" || command.Directory != root || command.Stdin != "" || !reflect.DeepEqual(command.Args, want), "command=%+v", command)
	require(t, command.StdoutPath != filepath.Join(state, "result.txt") || command.LogPath != filepath.Join(state, "stderr.log"), "output paths=%+v", command)
	data, err := os.ReadFile(prompt)
	text := string(data)
	require(t, err != nil || !strings.HasPrefix(text, "brief text\n") || !strings.Contains(text, "Diff: /diffs/read.diff") || !strings.Contains(text, "to exactly this file: read.md"), "prompt=%q err=%v", text, err)
}

func TestDevinModelCarriesTheEffort(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ model, effort, want string }{
		{"claude-opus-5-5", "xhigh", "claude-opus-5-5-xhigh"},
		{"gpt-6-astra-xhigh", "xhigh", "gpt-6-astra-xhigh"},
		{"gpt-6-astra", "", "gpt-6-astra"},
		{"gpt-6-astra-high", "xhigh", "gpt-6-astra-high-xhigh"},
	} {
		if got := devinModel(row.model, row.effort); got != row.want {
			t.Errorf("devinModel(%q, %q)=%q, want %q", row.model, row.effort, got, row.want)
		}
	}
}

// Devin cannot cap its context window, and it has no lane default model:
// both are refused before a child starts.
func TestDevinPrintRefusesWhatItCannotHonour(t *testing.T) {
	t.Parallel()
	record, _ := devinRecord(t, "design")
	setInt64(record.AdapterData, "window", 400000)
	_, err := (DevinPrint{Binary: "devin"}).Command(record, t.TempDir())
	require(t, err == nil || !strings.Contains(err.Error(), "launch.design.window.tokens=0"), "window err=%v", err)
	record, _ = devinRecord(t, "critique")
	setInt64(record.AdapterData, "window", 1)
	_, err = (DevinPrint{Binary: "devin"}).Command(record, t.TempDir())
	require(t, err == nil || !strings.Contains(err.Error(), "launch.build.window.tokens=0"), "critique window err=%v", err)
	record, _ = devinRecord(t, "build")
	delete(record.AdapterData, "model")
	_, err = (DevinPrint{Binary: "devin"}).Command(record, t.TempDir())
	require(t, err == nil || !strings.Contains(err.Error(), "launch.build.model"), "model err=%v", err)
}

const devinTranscriptFixture = `{"schema_version":"1","session_id":"mica-friction","agent":{"name":"devin","model_name":"GPT-6 Astra XHigh Thinking"},"steps":[
{"step_id":1,"source":"system","message":"system prompt","extra":{"telemetry":{"source":"sysprompt","operation":"normal"}}},
{"step_id":2,"source":"user","message":"brief text","extra":{"telemetry":{"source":"user","operation":"unknown"}}},
{"step_id":3,"source":"agent","model_name":"gpt-6-astra-xhigh","tool_calls":[{"tool_call_id":"a","function_name":"read"},{"tool_call_id":"b","function_name":"exec"}],
 "metrics":{"prompt_tokens":11014,"completion_tokens":30,"extra":{"cache_creation_input_tokens":11011}}},
{"step_id":4,"source":"system","message":"summary","extra":{"telemetry":{"source":"system","operation":"compaction"}}},
{"step_id":5,"source":"agent","model_name":"gpt-6-astra-xhigh","tool_calls":[{"tool_call_id":"c","function_name":"write"}],
 "metrics":{"prompt_tokens":250000,"completion_tokens":40,"cached_tokens":249000,"extra":{"cache_creation_input_tokens":900}}},
{"step_id":6,"source":"agent","model_name":"gpt-6-astra-xhigh","message":"done",
 "metrics":{"prompt_tokens":11259,"completion_tokens":5,"cached_tokens":11200,"extra":{"cache_creation_input_tokens":56}}}
],"final_metrics":{"total_prompt_tokens":1,"total_completion_tokens":1,"total_cached_tokens":1,"total_steps":6}}`

// A finished Devin launch is measured from its own export, step by step,
// and a read's verdict counts only when the session never compacted.
func TestDevinPrintSuperviseMeasuresTheExport(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, transcript string
		exit             int
		state            State
		reason           string
	}{
		{name: "measured", transcript: devinTranscriptFixture, state: Completed},
		{name: "no export", state: Failed, reason: "result-unreadable"},
		{name: "unknown model exports no turn", transcript: `{"session_id":"s","steps":[{"step_id":1,"source":"user"}]}`, state: Failed, reason: "result-unreadable"},
		{name: "nonzero exit", transcript: devinTranscriptFixture, exit: 3, state: Failed, reason: "exit-3"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			m, processes, _, _ := manager(t)
			m.Adapters["devin-print"] = DevinPrint{Binary: "devin"}
			record, root := devinRecord(t, "read")
			record.Adapter, record.State, record.StartedAt = "devin-print", Starting, m.Now().Format(time.RFC3339Nano)
			report := filepath.Join(root, "read.md")
			setStrings(record.AdapterData, "declaredOutputs", []string{report})
			if err := m.Store.Create(record); err != nil {
				t.Fatal(err)
			}
			processes.exit = row.exit
			processes.onStart = func(command Command) {
				if row.transcript != "" {
					if err := os.WriteFile(flagValue(command.Args, "--export"), []byte(row.transcript), 0o600); err != nil {
						t.Error(err)
					}
				}
				if err := os.WriteFile(command.StdoutPath, []byte("Read finished.\ndone\n"), 0o600); err != nil {
					t.Error(err)
				}
				if err := os.WriteFile(report, []byte("findings\nVERDICT: land\n"), 0o600); err != nil {
					t.Error(err)
				}
			}
			got, err := m.Supervise(record.ID)
			if err != nil || got.State != row.state || got.Reason != row.reason {
				t.Fatalf("state=%s reason=%q err=%v", got.State, got.Reason, err)
			}
			if row.name != "measured" {
				require(t, got.VerdictIsCounting(), "a failed read counted its verdict: %+v", got)
				return
			}
			want := Measurement{Calls: 3, ToolCalls: 3, Turns: 3, Compactions: 1,
				InputTokens: 3 + 100 + 3, CacheReadTokens: 249000 + 11200, CacheCreationTokens: 11011 + 900 + 56, OutputTokens: 75,
				PeakContext: 250000, CallsAbove200: 1, ResultLines: 2, ResultWords: 3, ResultTail: "Read finished. | done", Verdict: "VERDICT: land"}
			require(t, !reflect.DeepEqual(got.Measurement, want), "measurement=%+v\nwant       %+v", got.Measurement, want)
			require(t, readString(got.AdapterData, "sessionID") != "mica-friction" || readString(got.AdapterData, "observedModel") != "gpt-6-astra-xhigh", "patch=%s", got.AdapterData)
			require(t, got.VerdictIsCounting(), "a compacted read counted its verdict")
		})
	}
}

// A print-mode Devin process exporting into a launch state directory that no
// running record owns is reported; an owned one and an unrelated one are not.
func TestDevinPrintReportsStrays(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	export := func(id string) string { return filepath.Join(root, id, "transcript.json") }
	owner := ref(31)
	adapter := DevinPrint{StateRoot: root, Scanner: scan{
		{Ref: ref(30), Argv: []string{"/opt/devin", "-p", "--export", export("lost-read")}},
		{Ref: owner, Argv: []string{"devin", "-p", "--export", export("owned-read")}},
		{Ref: ref(32), Argv: []string{"devin", "-p", "--export", "/elsewhere/transcript.json"}},
		{Ref: ref(33), Argv: []string{"devin", "acp"}},
	}}
	lines, err := adapter.StraysFor([]Record{{ID: "owned-read", Adapter: "devin-print", State: Running, Child: &owner}})
	require(t, err != nil || !reflect.DeepEqual(lines, []string{"stray-devin-print pid=30 id=lost-read"}), "strays=%v err=%v", lines, err)
}
