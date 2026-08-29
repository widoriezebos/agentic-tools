package adapter

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

func writeRound(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Proof 1: per-runtime projection — kinds from the fields the
// artifacts really carry, seq as the projected ordinal, raw-byte
// params, counted skips, ceilings, and absent artifacts.
func TestEventsProjectionPerRuntime(t *testing.T) {
	t.Run("codex kinds from type", func(t *testing.T) {
		dir := writeRound(t, map[string]string{"events.jsonl": strings.Join([]string{
			`{"type":"thread.started","thread_id":"th-1"}`,
			`{"type":"turn.completed","usage":{"input_tokens":3}}`,
			`{"noType":true}`,
		}, "\n")})
		events, err := RuntimeEvents("codex")(dir)
		if err != nil {
			t.Fatal(err)
		}
		kinds := eventKinds(events)
		want := []string{"codex/thread.started", "codex/turn.completed", "codex/unlabeled"}
		if strings.Join(kinds, ",") != strings.Join(want, ",") {
			t.Fatalf("kinds %v want %v", kinds, want)
		}
		for i, ev := range events {
			if ev.Seq != uint64(i+1) {
				t.Fatalf("seq law broken at %d: %d", i, ev.Seq)
			}
		}
		if !bytes.Equal(events[0].Params, []byte(`{"type":"thread.started","thread_id":"th-1"}`)) {
			t.Fatalf("params not raw bytes: %s", events[0].Params)
		}
	})
	t.Run("claude subtype refinement", func(t *testing.T) {
		dir := writeRound(t, map[string]string{"events.jsonl": strings.Join([]string{
			`{"type":"system","subtype":"init","session_id":"s"}`,
			`{"type":"assistant","message":{}}`,
			`{"type":"result","subtype":"success","usage":{}}`,
		}, "\n")})
		events, err := RuntimeEvents("claude")(dir)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"claude/system.init", "claude/assistant", "claude/result.success"}
		if strings.Join(eventKinds(events), ",") != strings.Join(want, ",") {
			t.Fatalf("kinds %v want %v", eventKinds(events), want)
		}
	})
	t.Run("fake falls back to event field", func(t *testing.T) {
		dir := writeRound(t, map[string]string{"events.jsonl": strings.Join([]string{
			`{"event":"session-established","topLevel":true}`,
			`{"type":"turn.completed","topLevel":false}`,
			`{"neither":1}`,
		}, "\n")})
		events, err := RuntimeEvents("fake")(dir)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"fake/session-established", "fake/turn.completed", "fake/unlabeled"}
		if strings.Join(eventKinds(events), ",") != strings.Join(want, ",") {
			t.Fatalf("kinds %v", eventKinds(events))
		}
	})
	t.Run("malformed tail counted and surfaced", func(t *testing.T) {
		dir := writeRound(t, map[string]string{"events.jsonl": strings.Join([]string{
			`{"type":"session-correlated"}`,
			`{"torn...`,
			`not json at all`,
		}, "\n")})
		events, err := RuntimeEvents("devin")(dir)
		if err != nil {
			t.Fatal(err)
		}
		last := events[len(events)-1]
		if last.Kind != "devin/lines-skipped" {
			t.Fatalf("no lines-skipped event: %v", eventKinds(events))
		}
		var body struct {
			Skipped int `json:"skipped"`
		}
		if err := json.Unmarshal(last.Params, &body); err != nil || body.Skipped != 2 {
			t.Fatalf("skip params %s", last.Params)
		}
		if last.Seq != uint64(len(events)) {
			t.Fatalf("skip event seq %d of %d", last.Seq, len(events))
		}
	})
	t.Run("padded line rides verbatim; null and array are counted skips", func(t *testing.T) {
		padded := `  {"type":"thread.started"}  `
		dir := writeRound(t, map[string]string{"events.jsonl": strings.Join([]string{
			padded,
			`null`,
			`[1,2]`,
		}, "\n")})
		events, err := RuntimeEvents("codex")(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 2 {
			t.Fatalf("projected %d events: %v", len(events), eventKinds(events))
		}
		if !bytes.Equal(events[0].Params, []byte(padded)) {
			t.Fatalf("padding not verbatim: %q", events[0].Params)
		}
		var body struct {
			Skipped int `json:"skipped"`
		}
		if err := json.Unmarshal(events[1].Params, &body); err != nil || body.Skipped != 2 {
			t.Fatalf("null/array not counted: %s", events[1].Params)
		}
	})
	t.Run("missing artifact is empty and nil", func(t *testing.T) {
		events, err := RuntimeEvents("codex")(t.TempDir())
		if err != nil || events != nil {
			t.Fatalf("missing artifact: %v %v", events, err)
		}
	})
	t.Run("over-ceiling errors, never truncates", func(t *testing.T) {
		big := strings.Repeat(`{"type":"x"}`+"\n", MaxEventArtifactBytes/12)
		dir := writeRound(t, map[string]string{"events.jsonl": big})
		if _, err := RuntimeEvents("codex")(dir); err == nil || !strings.Contains(err.Error(), "ceiling") {
			t.Fatalf("ceiling not enforced: %v", err)
		}
	})
	t.Run("claude artifact selection: stream is sole authority", func(t *testing.T) {
		dir := writeRound(t, map[string]string{
			"claude-stream.jsonl": `{"type":"result","subtype":"success"}`,
			"events.jsonl":        `{"type":"system","subtype":"init"}` + "\n" + `{"type":"result","subtype":"success"}`,
		})
		events, err := RuntimeEvents("claude")(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].Kind != "claude/result.success" {
			t.Fatalf("selection law broken: %v", eventKinds(events))
		}
	})
}

func eventKinds(events []delegate.Event) []string {
	kinds := make([]string, len(events))
	for i, ev := range events {
		kinds[i] = ev.Kind
	}
	return kinds
}

// Proofs 3+4: the derivation writes the last result-typed line
// verbatim; a stream without one is the missing-result failure.
func TestClaudeDeriveResult(t *testing.T) {
	dir := t.TempDir()
	stream := filepath.Join(dir, "claude-stream.jsonl")
	result := filepath.Join(dir, "claude-result.json")
	resultDoc := `{"type":"result","subtype":"success","session_id":"s-1","result":"{}","usage":{"input_tokens":2,"output_tokens":1}}`
	content := strings.Join([]string{
		`{"type":"system","subtype":"init","session_id":"s-1"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`,
		resultDoc,
	}, "\n")
	if err := os.WriteFile(stream, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ClaudeDeriveResult(stream, result); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(result)
	if string(got) != resultDoc+"\n" {
		t.Fatalf("derived document diverged:\n got %s\nwant %s", got, resultDoc)
	}
	// Every existing consumer reads the derived document as the
	// blocking one: usage extraction and result fields agree.
	usagePath := filepath.Join(dir, "usage.json")
	if err := ClaudeUsage(result, usagePath); err != nil {
		t.Fatal(err)
	}
	if value, present, err := ClaudeResultField(result, "session_id"); err != nil || !present || value != "s-1" {
		t.Fatalf("result field through derived doc: %q %v %v", value, present, err)
	}

	// Missing result-typed line: an error, no document invented.
	if err := os.WriteFile(stream, []byte(`{"type":"assistant"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ClaudeDeriveResult(stream, filepath.Join(dir, "never.json")); err == nil {
		t.Fatal("a result was invented from a resultless stream")
	}
}

// Proof 7: the host argv golden — full-vector comparison; json
// output, no --verbose, no stream flag.
func TestBuildClaudeCommandHostGolden(t *testing.T) {
	command, err := BuildClaudeCommand("", "opus", "{}", "", "", "5.00", "50", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"claude", "-p", "--output-format", "json", "--model", "opus",
		"--json-schema", "{}",
		"--permission-mode", "acceptEdits",
		"--tools", claudeFullTools,
		"--allowedTools", claudeFullTools,
		"--max-budget-usd", "5.00", "--max-turns", "50",
	}
	if strings.Join(command, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("host argv drifted:\n got %q\nwant %q", command, want)
	}
}

// Proof 6 (builder half): the nativeEvents implication at the joints
// that exist — claude's dispatch argv streams with --verbose; an
// unknown mode refuses.
func TestClaudeOutputModeImplication(t *testing.T) {
	command, err := BuildClaudeCommand("", "opus", "{}", "", "", "5.00", "50", "stream-json")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(command, " ")
	if !strings.Contains(joined, "--output-format stream-json") || !strings.Contains(joined, "--verbose") {
		t.Fatalf("stream argv incomplete: %s", joined)
	}
	if _, err := BuildClaudeCommand("", "opus", "{}", "", "", "5.00", "50", "yaml"); err == nil {
		t.Fatal("unknown output mode accepted")
	}
}

// Proof 5's live half (S3-C1-003): every live runtime registration
// serves the SAME projection as the direct call — deleting or
// swapping a registration in delegateports.go fails here, not
// silently.
func TestEventsPortLiveRegistrations(t *testing.T) {
	dir := writeRound(t, map[string]string{"events.jsonl": `{"type":"probe"}`})
	for _, runtime := range []string{"claude", "codex", "devin", "fake"} {
		ports, err := delegate.PortsFor(runtime)
		if err != nil || ports.Events == nil {
			t.Fatalf("%s Events port unregistered: %v", runtime, err)
		}
		direct, err1 := RuntimeEvents(runtime)(dir)
		ported, err2 := ports.Events(dir)
		if err1 != nil || err2 != nil || len(direct) != len(ported) ||
			direct[0].Kind != ported[0].Kind || !bytes.Equal(direct[0].Params, ported[0].Params) {
			t.Fatalf("%s port diverges from the direct call: %v %v %v %v", runtime, direct, ported, err1, err2)
		}
	}
}

// The devin half of the implication (design: honestly weaker,
// shell-side): devin's probe declares nativeEvents false and its
// inline argv must carry no stream output mode. The wall-rule
// verbatim-pin precedent: a static read of the live authority.
func TestDevinArgvCarriesNoStreamMode(t *testing.T) {
	data, err := os.ReadFile("../../scripts/agents/adapters/devin.sh")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "stream-json") {
		t.Fatal("devin.sh mentions a stream output mode its probe declares false")
	}
	if !strings.Contains(content, `"nativeEvents": false`) {
		t.Fatal("devin.sh no longer declares nativeEvents false; revisit the implication pin")
	}
	if !strings.Contains(content, `config_file="$round_dir/$instance_tag"`) {
		t.Fatal("Devin's CLI config argument does not carry the reservation instance tag")
	}
}
