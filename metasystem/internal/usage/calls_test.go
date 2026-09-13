package usage

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLatestCallReadsAClaudeTranscriptToTheFigure(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	toplevel := t.TempDir()
	session := "claude-session"
	transcript := writeClaudeCallTranscript(t, home, toplevel, session,
		claudeAssistant("A", 100, 20, 30, false, "2026-09-13T10:00:00Z"),
		claudeAssistant("A", 1, 1, 1, false, "2026-09-13T10:01:00Z"),
		claudeAssistant("side", 999999, 0, 0, true, "2026-09-13T10:02:00Z"),
		claudeAssistant("B", 5000, 0, 45000, false, "2026-09-13T10:03:00Z"),
	)

	reading, err := LatestCall(stateRoot, "claude", session, ReadOptions{
		Capability: PerCall,
		Home:       home,
		Toplevel:   toplevel,
		Now:        time.Date(2026, 9, 13, 10, 4, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Latest == nil || reading.Latest.PromptTokens != 50000 {
		t.Fatalf("latest = %#v, want 50000 prompt tokens", reading.Latest)
	}
	if !strings.Contains(reading.Latest.Source, "sidechain records skipped: 1") {
		t.Fatalf("latest source = %q, want the skipped sidechain count", reading.Latest.Source)
	}
	if reading.NewSamples != 2 || reading.Cursor.SampleCount != 2 {
		t.Fatalf("new=%d count=%d, want 2 and 2", reading.NewSamples, reading.Cursor.SampleCount)
	}
	if reading.Reason != "" {
		t.Fatalf("reason = %q, want empty", reading.Reason)
	}
	samples, markers, err := Calls(stateRoot, "claude", session, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 2 || len(markers) != 0 {
		t.Fatalf("samples=%d markers=%d, want 2 and 0; transcript=%s", len(samples), len(markers), transcript)
	}
}

func TestLatestCallKeysCodexSamplesByResponseID(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	session := "codex-session"
	var rows []string
	rows = append(rows, `{"type":"session_meta"}`)
	for ordinal := 1; ordinal <= 5; ordinal++ {
		rows = append(rows, `{"type":"event_msg","payload":{"type":"token_count"}}`)
	}
	rows = append(rows,
		codexUsage("R1", 100, 80, 0, 11, "2026-09-13T11:00:00Z"),
		codexUsage("R1", 101, 81, 0, 12, "2026-09-13T11:01:00Z"),
		codexUsage("R2", 27866, 27392, 0, 13, "2026-09-13T11:02:00Z"),
	)
	writeCodexCallRollout(t, home, session, rows...)

	reading, err := LatestCall(stateRoot, "codex", session, ReadOptions{Capability: PerCall, Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if reading.NewSamples != 2 || reading.Cursor.SampleCount != 2 {
		t.Fatalf("new=%d count=%d, want 2 and 2", reading.NewSamples, reading.Cursor.SampleCount)
	}
	if reading.Latest == nil {
		t.Fatal("latest is nil")
	}
	if reading.Latest.InvocationID != "R2" || reading.Latest.PromptTokens != 27866 || reading.Latest.CacheRead != 27392 {
		t.Fatalf("latest = %#v", reading.Latest)
	}
}

func TestCodexRolloutWithoutUsageRecordsIsUnknown(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	session := "old-codex-session"
	writeCodexCallRollout(t, home, session,
		`{"type":"event_msg","payload":{"type":"token_count"}}`,
		`{"type":"event_msg","payload":{"type":"token_count"}}`,
	)

	reading, err := LatestCall(stateRoot, "codex", session, ReadOptions{Capability: PerCall, Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Latest != nil {
		t.Fatalf("latest = %#v, want nil", reading.Latest)
	}
	want := "unknown (rollout carries no token_usage_record (codex CLI before 0.153))"
	if reading.Reason != want {
		t.Fatalf("reason = %q, want %q", reading.Reason, want)
	}
	if reading.Cursor.SampleCount != 0 {
		t.Fatalf("sample count = %d, want 0", reading.Cursor.SampleCount)
	}
	if _, err := os.Stat(CursorPath(stateRoot, "codex", session)); err != nil {
		t.Fatalf("cursor was not written: %v", err)
	}
}

func TestMarkersCountCompactionsPerRuntime(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	toplevel := t.TempDir()
	claudeSession := "claude-marker"
	codexSession := "codex-marker"
	writeClaudeCallTranscript(t, home, toplevel, claudeSession,
		`{"type":"system","subtype":"compact_boundary","timestamp":"2026-09-13T12:00:00Z","compactMetadata":{"trigger":"auto","preTokens":967668}}`,
	)
	writeCodexCallRollout(t, home, codexSession,
		`{"type":"compacted","timestamp":"2026-09-13T12:01:00Z","ordinal":17}`,
	)

	claudeReading, err := LatestCall(stateRoot, "claude", claudeSession, ReadOptions{Capability: PerCall, Home: home, Toplevel: toplevel})
	if err != nil {
		t.Fatal(err)
	}
	codexReading, err := LatestCall(stateRoot, "codex", codexSession, ReadOptions{Capability: PerCall, Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if claudeReading.Cursor.CompactionCount != 1 || codexReading.Cursor.CompactionCount != 1 {
		t.Fatalf("compactions claude=%d codex=%d", claudeReading.Cursor.CompactionCount, codexReading.Cursor.CompactionCount)
	}
	if claudeReading.NewMarkers != 1 || codexReading.NewMarkers != 1 {
		t.Fatalf("new markers claude=%d codex=%d", claudeReading.NewMarkers, codexReading.NewMarkers)
	}
	_, markers, err := Calls(stateRoot, "claude", claudeSession, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 1 || markers[0].Kind != "compaction" || markers[0].Detail != "trigger=auto preTokens=967668" {
		t.Fatalf("markers = %#v", markers)
	}
	assertJSONLKindCount(t, SamplesPath(stateRoot, "claude", claudeSession), "marker", 1)
	assertJSONLKindCount(t, SamplesPath(stateRoot, "codex", codexSession), "marker", 1)
}

func TestPerInvocationRuntimeAnswersUnknown(t *testing.T) {
	stateRoot := t.TempDir()
	var opened []string
	previous := callFileOpens
	callFileOpens = func(path string) { opened = append(opened, path) }
	t.Cleanup(func() { callFileOpens = previous })

	for _, test := range []struct {
		capability Capability
		reason     string
	}{
		{PerInvocation, "unknown (per-invocation usage only)"},
		{NoStream, "unknown (no per-call stream)"},
	} {
		reading, err := LatestCall(stateRoot, "devin", "session", ReadOptions{Capability: test.capability})
		if err != nil {
			t.Fatal(err)
		}
		if reading.Latest != nil || reading.Reason != test.reason {
			t.Fatalf("capability %s: reading=%#v", test.capability, reading)
		}
	}
	if len(opened) != 0 {
		t.Fatalf("opened files: %v", opened)
	}
	if _, err := os.Stat(filepath.Dir(CursorPath(stateRoot, "devin", "session"))); !os.IsNotExist(err) {
		t.Fatalf("cursor directory exists or stat failed unexpectedly: %v", err)
	}
}

func TestMissingTranscriptAnswersUnknownWithThePath(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	toplevel := t.TempDir()
	installation := t.TempDir()
	session := "missing"
	reading, err := LatestCall(stateRoot, "claude", session, ReadOptions{
		Capability:   PerCall,
		Home:         home,
		Toplevel:     toplevel,
		Installation: installation,
	})
	if err != nil {
		t.Fatal(err)
	}
	path1 := filepath.Join(home, ".claude", "projects", claudeSlug(toplevel), session+".jsonl")
	path2 := filepath.Join(home, ".claude", "projects", claudeSlug(installation), session+".jsonl")
	want := "unknown (no transcript at " + path1 + " or " + path2 + ")"
	if reading.Reason != want {
		t.Fatalf("reason = %q, want %q", reading.Reason, want)
	}
	if _, err := os.Stat(CursorPath(stateRoot, "claude", session)); !os.IsNotExist(err) {
		t.Fatalf("cursor exists or stat failed unexpectedly: %v", err)
	}

	unreadableHome := t.TempDir()
	if os.Geteuid() == 0 {
		t.Skip("root can traverse an unreadable home")
	}
	if err := os.Chmod(unreadableHome, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadableHome, 0o755) })
	unreadableState := t.TempDir()
	reading, err = LatestCall(unreadableState, "claude", session, ReadOptions{
		Capability:   PerCall,
		Home:         unreadableHome,
		Toplevel:     toplevel,
		Installation: installation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(reading.Reason, "unknown (home directory unreadable") && !strings.Contains(reading.Reason, "unknown (no transcript at ") {
		t.Fatalf("unexpected unreadable-home reason: %q", reading.Reason)
	}
	if _, err := os.Stat(CursorPath(unreadableState, "claude", session)); !os.IsNotExist(err) {
		t.Fatalf("cursor exists for unreadable home: %v", err)
	}
}

func TestCallsFiltersBySinceAndRegistersSessionsOnce(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	toplevel := t.TempDir()
	session := "filter-session"
	writeClaudeCallTranscript(t, home, toplevel, session,
		claudeAssistant("one", 1, 0, 0, false, "2026-09-13T13:00:00Z"),
		claudeAssistant("two", 2, 0, 0, false, "2026-09-13T13:01:00Z"),
		claudeAssistant("three", 3, 0, 0, false, "2026-09-13T13:02:00Z"),
	)
	if _, err := LatestCall(stateRoot, "claude", session, ReadOptions{Capability: PerCall, Home: home, Toplevel: toplevel}); err != nil {
		t.Fatal(err)
	}
	since := time.Date(2026, 9, 13, 13, 1, 0, 0, time.UTC)
	samples, markers, err := Calls(stateRoot, "claude", session, since)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 2 || len(markers) != 0 || samples[0].InvocationID != "two" || samples[1].InvocationID != "three" {
		t.Fatalf("samples=%#v markers=%#v", samples, markers)
	}

	if err := RegisterSession(stateRoot, "claude", session, 101, 202); err != nil {
		t.Fatal(err)
	}
	if err := RegisterSession(stateRoot, "claude", session, 101, 202); err != nil {
		t.Fatal(err)
	}
	if err := RegisterSession(stateRoot, "claude", session, 303, 404); err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(stateRoot, "artifacts", "agents", "context", "sessions.jsonl")
	if got := jsonlLineCount(t, registry); got != 2 {
		t.Fatalf("registry rows = %d, want 2", got)
	}
}

func TestCallPathRulesAndRuntimeValidation(t *testing.T) {
	if got, want := claudeSlug("/Users/wido/LocalStorage/GitHub/agentic-tools-m1e"), "-Users-wido-LocalStorage-GitHub-agentic-tools-m1e"; got != want {
		t.Fatalf("claude slug = %q, want %q", got, want)
	}
	if got := SessionSlug("safe.Session-1"); got != "safe.Session-1" {
		t.Fatalf("safe session slug = %q", got)
	}
	if got := SessionSlug("unsafe/session"); len(got) != 64 || strings.Contains(got, "/") {
		t.Fatalf("hashed session slug = %q", got)
	}
	if _, err := LatestCall("relative", "claude", "session", ReadOptions{Capability: PerCall}); err == nil {
		t.Fatal("relative state root was accepted")
	}
	if _, err := LatestCall(t.TempDir(), "Bad/runtime", "session", ReadOptions{Capability: PerCall}); err == nil {
		t.Fatal("invalid runtime was accepted")
	}
	reading, err := LatestCall(t.TempDir(), "fake", "session", ReadOptions{Capability: PerCall})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Reason != "unknown (no reader for runtime fake)" {
		t.Fatalf("unknown-runtime reason = %q", reading.Reason)
	}
}

func TestCallParsersIgnoreInvalidAndUnrelatedRows(t *testing.T) {
	for _, line := range []string{
		`not json`,
		`{"type":"user"}`,
		`{"type":"assistant"}`,
		`{"type":"assistant","message":{"usage":null}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":-1}}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":9223372036854775807,"cache_creation_input_tokens":1}}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":1,"cache_creation_input_tokens":null}}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":1,"cache_read_input_tokens":false}}}`,
	} {
		sample, marker, sidechain := parseClaudeLine([]byte(line), 4, "claude", "session")
		if sample != nil || marker != nil || sidechain {
			t.Fatalf("Claude line %q produced sample=%#v marker=%#v sidechain=%v", line, sample, marker, sidechain)
		}
	}
	sample, marker, sidechain := parseClaudeLine([]byte(`{"type":"assistant","timestamp":"bad","message":{"usage":{"input_tokens":4}}}`), 9, "claude", "session")
	if marker != nil || sidechain || sample == nil || sample.InvocationID != "line:9" || sample.PromptTokens != 4 || !sample.At.IsZero() {
		t.Fatalf("Claude fallback sample=%#v marker=%#v sidechain=%v", sample, marker, sidechain)
	}
	_, marker, _ = parseClaudeLine([]byte(`{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"manual","preTokens":12.5}}`), 10, "claude", "session")
	if marker == nil || marker.Detail != "trigger=manual preTokens=12.5" || !marker.At.IsZero() {
		t.Fatalf("Claude marker = %#v", marker)
	}
	_, _, sidechain = parseClaudeLine([]byte(`{"type":"assistant","isSidechain":true}`), 11, "claude", "session")
	if !sidechain {
		t.Fatal("Claude sidechain was not identified")
	}

	for _, line := range []string{
		`not json`,
		`{"type":"event_msg","payload":{"type":"token_count"}}`,
		`{"type":"token_usage_record","payload":{"usage":{"input_tokens":1}}}`,
		`{"type":"token_usage_record","payload":{"response_id":"R"}}`,
		`{"type":"token_usage_record","payload":{"response_id":"R","usage":{"input_tokens":-1}}}`,
		`{"type":"token_usage_record","payload":{"response_id":"R","usage":{"input_tokens":1,"cached_input_tokens":null}}}`,
	} {
		sample, marker := parseCodexLine([]byte(line), "codex", "session")
		if sample != nil || marker != nil {
			t.Fatalf("Codex line %q produced sample=%#v marker=%#v", line, sample, marker)
		}
	}
	sample, marker = parseCodexLine([]byte(`{"type":"token_usage_record","timestamp":"bad","payload":{"response_id":"R","usage":{"input_tokens":8}}}`), "codex", "session")
	if marker != nil || sample == nil || sample.PromptTokens != 8 || sample.CacheRead != 0 || sample.CacheCreation != 0 || !sample.At.IsZero() {
		t.Fatalf("Codex optional-cache sample=%#v marker=%#v", sample, marker)
	}
	_, marker = parseCodexLine([]byte(`{"type":"compacted"}`), "codex", "session")
	if marker == nil || marker.Kind != "compaction" || marker.Detail != "compacted" || marker.Ordinal != 0 {
		t.Fatalf("Codex marker = %#v", marker)
	}
}

func TestCallResolutionAndEmptySamples(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	if samples, markers, err := Calls(stateRoot, "claude", "absent", time.Time{}); err != nil || samples != nil || markers != nil {
		t.Fatalf("missing samples: samples=%#v markers=%#v err=%v", samples, markers, err)
	}
	reading, err := LatestCall(stateRoot, "codex", "absent", ReadOptions{Capability: PerCall, Home: home})
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := filepath.Join(home, ".codex", "sessions")
	if reading.Reason != "unknown (no rollout for session absent under "+wantRoot+")" {
		t.Fatalf("missing-rollout reason = %q", reading.Reason)
	}
	writeCodexCallRollout(t, home, "duplicate", `{"type":"session_meta"}`)
	second := filepath.Join(home, ".codex", "sessions", "2026", "09", "14", "rollout-second-duplicate.jsonl")
	writeCallRows(t, second, `{"type":"session_meta"}`)
	reading, err = LatestCall(stateRoot, "codex", "duplicate", ReadOptions{Capability: PerCall, Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Reason != "unknown (2 rollouts match session duplicate)" {
		t.Fatalf("duplicate-rollout reason = %q", reading.Reason)
	}
	if _, err := LatestCall(stateRoot, "claude", "session", ReadOptions{Capability: "surprise"}); err == nil {
		t.Fatal("invalid capability was accepted")
	}
}

func TestCallNumberHelpers(t *testing.T) {
	for _, test := range []struct {
		value any
		want  int64
		ok    bool
	}{
		{json.Number("12"), 12, true},
		{json.Number("12.75"), 12, true},
		{json.Number("bad"), 0, false},
		{float64(9), 9, true},
		{float64(-1), 0, false},
		{math.Inf(1), 0, false},
		{int(7), 7, true},
		{int64(8), 8, true},
		{true, 0, false},
		{"9", 0, false},
		{nil, 0, true},
	} {
		got, ok := callToken(test.value, test.value == nil)
		if got != test.want || ok != test.ok {
			t.Fatalf("callToken(%#v) = %d,%v, want %d,%v", test.value, got, ok, test.want, test.ok)
		}
	}
	if got := numberText(float64(3.5)); got != "3.5" {
		t.Fatalf("float number text = %q", got)
	}
	if got := numberText(int(4)); got != "4" {
		t.Fatalf("int number text = %q", got)
	}
	if got := numberText(int64(5)); got != "5" {
		t.Fatalf("int64 number text = %q", got)
	}
	if got := numberText("no"); got != "" {
		t.Fatalf("invalid number text = %q", got)
	}
}

func TestCallResolutionTreatsSessionAsLiteralText(t *testing.T) {
	stateRoot := t.TempDir()
	home := t.TempDir()
	toplevel := t.TempDir()
	directory := filepath.Join(home, ".claude", "projects", claudeSlug(toplevel))
	writeCallRows(t, filepath.Join(home, ".claude", "projects", "escaped.jsonl"), claudeAssistant("wrong", 99, 0, 0, false, "2026-09-13T20:00:00Z"))
	reading, err := LatestCall(stateRoot, "claude", "../escaped", ReadOptions{Capability: PerCall, Home: home, Toplevel: toplevel})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Latest != nil || !strings.HasPrefix(reading.Reason, "unknown (no transcript at ") {
		t.Fatalf("escaped Claude session reading = %#v (project directory %s)", reading, directory)
	}

	writeCodexCallRollout(t, home, "real-session", codexUsage("wrong", 88, 0, 0, 1, "2026-09-13T20:01:00Z"))
	reading, err = LatestCall(stateRoot, "codex", "*", ReadOptions{Capability: PerCall, Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Latest != nil || !strings.Contains(reading.Reason, "no rollout for session *") {
		t.Fatalf("glob-like Codex session reading = %#v", reading)
	}
}

func TestCodexRolloutTreatsHomeAsLiteralPath(t *testing.T) {
	for index, suffix := range []string{"[", "]", "*", "?", `\`, `[mix]*?\`} {
		name := "literal-" + decimal(index)
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			home := filepath.Join(parent, "home"+suffix)
			session := "literal-home-session"
			writeCodexCallRollout(t, home, session, codexUsage("right", 1, 0, 0, 1, "2026-09-13T22:00:00Z"))
			misleadingHome := filepath.Join(parent, "home-misleading-"+decimal(index))
			writeCodexCallRollout(t, misleadingHome, session, codexUsage("wrong", 99, 0, 0, 1, "2026-09-13T22:01:00Z"))
			reading, err := LatestCall(t.TempDir(), "codex", session, ReadOptions{Capability: PerCall, Home: home})
			if err != nil {
				t.Fatal(err)
			}
			if reading.Latest == nil || reading.Latest.InvocationID != "right" {
				t.Fatalf("literal home %q selected %#v", home, reading)
			}
		})
	}

	t.Run("zero-matches", func(t *testing.T) {
		home := t.TempDir()
		reading, err := LatestCall(t.TempDir(), "codex", "missing", ReadOptions{Capability: PerCall, Home: home})
		if err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(home, ".codex", "sessions")
		if reading.Latest != nil || reading.Reason != "unknown (no rollout for session missing under "+root+")" {
			t.Fatalf("zero-match reading = %#v", reading)
		}
	})

	t.Run("two-actual-matches", func(t *testing.T) {
		home := t.TempDir()
		session := "duplicate-literal"
		writeCodexCallRollout(t, home, session, codexUsage("one", 1, 0, 0, 1, "2026-09-13T22:02:00Z"))
		writeCallRows(t, filepath.Join(home, ".codex", "sessions", "2026", "09", "14", "rollout-other-"+session+".jsonl"), codexUsage("two", 2, 0, 0, 2, "2026-09-13T22:03:00Z"))
		reading, err := LatestCall(t.TempDir(), "codex", session, ReadOptions{Capability: PerCall, Home: home})
		if err != nil {
			t.Fatal(err)
		}
		if reading.Latest != nil || reading.Reason != "unknown (2 rollouts match session "+session+")" {
			t.Fatalf("multiple-match reading = %#v", reading)
		}
	})

	t.Run("literal-session-metacharacters", func(t *testing.T) {
		home := t.TempDir()
		session := `session[*]?\`
		writeCodexCallRollout(t, home, session, codexUsage("literal-session", 3, 0, 0, 3, "2026-09-13T22:04:00Z"))
		reading, err := LatestCall(t.TempDir(), "codex", session, ReadOptions{Capability: PerCall, Home: home})
		if err != nil {
			t.Fatal(err)
		}
		if reading.Latest == nil || reading.Latest.InvocationID != "literal-session" {
			t.Fatalf("literal session reading = %#v", reading)
		}
	})

	t.Run("explicit-transcript-override", func(t *testing.T) {
		transcript := filepath.Join(t.TempDir(), "explicit.jsonl")
		writeCallRows(t, transcript, codexUsage("explicit", 4, 0, 0, 4, "2026-09-13T22:05:00Z"))
		reading, err := LatestCall(t.TempDir(), "codex", "not-discovered", ReadOptions{Capability: PerCall, Home: filepath.Join(t.TempDir(), "[*]"), Transcript: transcript})
		if err != nil {
			t.Fatal(err)
		}
		if reading.Latest == nil || reading.Latest.InvocationID != "explicit" {
			t.Fatalf("override reading = %#v", reading)
		}
	})

	t.Run("directory-symlink-is-followed", func(t *testing.T) {
		home := t.TempDir()
		targetYear := t.TempDir()
		session := "directory-link"
		writeCallRows(t, filepath.Join(targetYear, "09", "13", "rollout-linked-"+session+".jsonl"), codexUsage("linked", 5, 0, 0, 5, "2026-09-13T22:06:00Z"))
		root := filepath.Join(home, ".codex", "sessions")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(targetYear, filepath.Join(root, "2026")); err != nil {
			t.Fatal(err)
		}
		reading, err := LatestCall(t.TempDir(), "codex", session, ReadOptions{Capability: PerCall, Home: home})
		if err != nil {
			t.Fatal(err)
		}
		if reading.Latest == nil || reading.Latest.InvocationID != "linked" {
			t.Fatalf("directory symlink reading = %#v", reading)
		}
	})

	t.Run("leaf-symlink-and-wrong-depth-are-ignored", func(t *testing.T) {
		home := t.TempDir()
		session := "ignored-shapes"
		target := filepath.Join(t.TempDir(), "target.jsonl")
		writeCallRows(t, target, codexUsage("wrong-link", 6, 0, 0, 6, "2026-09-13T22:07:00Z"))
		day := filepath.Join(home, ".codex", "sessions", "2026", "09", "13")
		if err := os.MkdirAll(day, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(day, "rollout-linked-"+session+".jsonl")); err != nil {
			t.Fatal(err)
		}
		writeCallRows(t, filepath.Join(home, ".codex", "sessions", "2026", "09", "rollout-shallow-"+session+".jsonl"), codexUsage("wrong-shallow", 7, 0, 0, 7, "2026-09-13T22:08:00Z"))
		writeCallRows(t, filepath.Join(day, "extra", "rollout-deep-"+session+".jsonl"), codexUsage("wrong-deep", 8, 0, 0, 8, "2026-09-13T22:09:00Z"))
		reading, err := LatestCall(t.TempDir(), "codex", session, ReadOptions{Capability: PerCall, Home: home})
		if err != nil {
			t.Fatal(err)
		}
		if reading.Latest != nil || !strings.Contains(reading.Reason, "no rollout for session "+session) {
			t.Fatalf("ignored-shape reading = %#v", reading)
		}
	})

	t.Run("unreadable-branch-prevents-unique-selection", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can traverse an unreadable directory")
		}
		home := t.TempDir()
		session := "unreadable-branch"
		writeCodexCallRollout(t, home, session, codexUsage("otherwise-unique", 9, 0, 0, 9, "2026-09-13T22:10:00Z"))
		unreadable := filepath.Join(home, ".codex", "sessions", "blocked")
		if err := os.MkdirAll(unreadable, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(unreadable, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(unreadable, 0o755) })
		if _, err := os.ReadDir(unreadable); err == nil {
			t.Skip("directory permissions are not enforceable")
		}
		reading, err := LatestCall(t.TempDir(), "codex", session, ReadOptions{Capability: PerCall, Home: home})
		if err != nil {
			t.Fatal(err)
		}
		if reading.Latest != nil || !strings.Contains(reading.Reason, "unknown (cannot read rollout directory "+unreadable+":") {
			t.Fatalf("unreadable-branch reading = %#v", reading)
		}
	})
}

func TestRegisterSessionSeparatesATruncatedTail(t *testing.T) {
	stateRoot := t.TempDir()
	registry := filepath.Join(stateRoot, "artifacts", "agents", "context", "sessions.jsonl")
	if err := os.MkdirAll(filepath.Dir(registry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registry, []byte(`{"runtime":"claude"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RegisterSession(stateRoot, "claude", "after-tail", 10, 20); err != nil {
		t.Fatal(err)
	}
	if err := RegisterSession(stateRoot, "claude", "after-tail", 10, 20); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(registry)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	valid := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row map[string]any
		if json.Unmarshal(scanner.Bytes(), &row) == nil && row["session"] == "after-tail" {
			valid++
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if valid != 1 {
		t.Fatalf("valid registrations after truncated tail = %d, want 1", valid)
	}
}

func writeClaudeCallTranscript(t *testing.T, home, cwd, session string, rows ...string) string {
	t.Helper()
	path := filepath.Join(home, ".claude", "projects", claudeSlug(cwd), session+".jsonl")
	writeCallRows(t, path, rows...)
	return path
}

func writeCodexCallRollout(t *testing.T, home, session string, rows ...string) string {
	t.Helper()
	path := filepath.Join(home, ".codex", "sessions", "2026", "09", "13", "rollout-2026-09-13T00-00-00-"+session+".jsonl")
	writeCallRows(t, path, rows...)
	return path
}

func writeCallRows(t *testing.T, path string, rows ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join(rows, "\n")
	if len(rows) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func claudeAssistant(id string, input, creation, read int64, sidechain bool, at string) string {
	row := map[string]any{
		"type":        "assistant",
		"requestId":   id,
		"isSidechain": sidechain,
		"timestamp":   at,
		"message": map[string]any{"usage": map[string]any{
			"input_tokens":                input,
			"cache_creation_input_tokens": creation,
			"cache_read_input_tokens":     read,
		}},
	}
	encoded, _ := json.Marshal(row)
	return string(encoded)
}

func codexUsage(id string, input, cached, creation, ordinal int64, at string) string {
	row := map[string]any{
		"type":      "token_usage_record",
		"timestamp": at,
		"ordinal":   ordinal,
		"payload": map[string]any{
			"response_id": id,
			"usage": map[string]any{
				"input_tokens":             input,
				"cached_input_tokens":      cached,
				"cache_write_input_tokens": creation,
			},
		},
	}
	encoded, _ := json.Marshal(row)
	return string(encoded)
}

func assertJSONLKindCount(t *testing.T, path, kind string, want int) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	got := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal(scanner.Bytes(), &row) == nil && row.Kind == kind {
			got++
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s kind %q count = %d, want %d", path, kind, got, want)
	}
}

func jsonlLineCount(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return count
}
