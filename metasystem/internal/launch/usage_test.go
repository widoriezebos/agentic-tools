package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// usageRows is a session transcript of two answered calls: 30 tokens.
const usageRows = `{"type":"assistant","message":{"id":"m1","usage":{"input_tokens":5,"output_tokens":7}}}
{"type":"assistant","message":{"id":"m2","usage":{"input_tokens":3,"cache_read_input_tokens":10,"output_tokens":5}}}
`

func writeTranscript(t *testing.T, projects, session string) {
	t.Helper()
	dir := filepath.Join(projects, "-lane-checkout")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, session+".jsonl"), []byte(usageRows), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCancelledLaunchIsMeasured (K10, R8-06): a launch cancelled while it
// runs is measured like any other when its supervisor sees the child end:
// its session's usage is in its record.
func TestCancelledLaunchIsMeasured(t *testing.T) {
	t.Parallel()
	m, processes, _, _ := manager(t)
	projects := t.TempDir()
	m.Adapters["claude-headless"] = ClaudeHeadless{ProjectsRoot: projects, Scanner: scan{}}
	_, _ = seedClaude(t, m, "landing-cancelled")
	if _, err := m.Store.Update("landing-cancelled", func(record *Record) error {
		record.AdapterData["sessionID"] = rawString("0f0e0d0c-0b0a-4908-8706-050403020100")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	writeTranscript(t, projects, "0f0e0d0c-0b0a-4908-8706-050403020100")
	processes.childWait = func() {
		// A cancellation arrives while the session runs; it leaves no
		// result behind.
		_, _ = m.Store.Update("landing-cancelled", func(record *Record) error { record.Reason = "cancel-requested"; return nil })
	}
	got, err := m.Supervise("landing-cancelled")
	if err != nil || got.State != Cancelled {
		t.Fatalf("supervise = %+v %v; want the launch cancelled", got, err)
	}
	if got.Measurement.TotalTokens() != 30 {
		t.Fatalf("the cancelled launch's measurement = %+v; want its session's 30 tokens", got.Measurement)
	}
}

// TestClaudeMeasureReadsTheTranscriptOfAFailedSession (K10, R8-06): a
// session that ended in an error result, or left no readable result, is
// still measured from its transcript when its session is known; the
// outcome stays failed.
func TestClaudeMeasureReadsTheTranscriptOfAFailedSession(t *testing.T) {
	t.Parallel()
	projects := t.TempDir()
	writeTranscript(t, projects, "sid")
	adapter := ClaudeHeadless{ProjectsRoot: projects}
	state := t.TempDir()
	writeClaudeResult(t, state, `{"session_id":"sid","is_error":true,"num_turns":1,"result":"API Error"}`)
	measurement, _, _, err := adapter.Measure(Record{Kind: "landing", AdapterData: map[string]json.RawMessage{}}, state)
	if err != errClaudeResultError || measurement.TotalTokens() != 30 {
		t.Fatalf("an error result: %+v %v; want result-error and the transcript's 30 tokens", measurement, err)
	}
	unread := t.TempDir()
	measurement, _, _, err = adapter.Measure(Record{Kind: "landing", AdapterData: map[string]json.RawMessage{"sessionID": rawString("sid")}}, unread)
	if err != errClaudeResultUnreadable || measurement.TotalTokens() != 30 {
		t.Fatalf("no result, a known session: %+v %v; want result-unreadable and the transcript's 30 tokens", measurement, err)
	}
}

// TestLandingSessionIDIsFixedBeforeItStarts (K10): a landing session runs
// under the session id its launch recorded, so its transcript is found
// however it ends; other kinds keep Claude's own id.
func TestLandingSessionIDIsFixedBeforeItStarts(t *testing.T) {
	t.Parallel()
	record, _ := claudeRecord(t, "landing")
	record.AdapterData["settings"] = rawString("/lane/state/landing-settings.json")
	record.AdapterData["sessionID"] = rawString("0f0e0d0c-0b0a-4908-8706-050403020100")
	command, err := (ClaudeHeadless{Binary: "claude"}).Command(record, t.TempDir())
	index := slices.Index(command.Args, "--session-id")
	if err != nil || index < 0 || command.Args[index+1] != "0f0e0d0c-0b0a-4908-8706-050403020100" {
		t.Fatalf("argv=%v err=%v; want --session-id with the recorded id", command.Args, err)
	}
	build, _ := claudeRecord(t, "build")
	build.AdapterData["sessionID"] = rawString("0f0e0d0c-0b0a-4908-8706-050403020100")
	if command, err := (ClaudeHeadless{Binary: "claude"}).Command(build, t.TempDir()); err != nil || slices.Contains(command.Args, "--session-id") {
		t.Fatalf("a build: argv=%v err=%v; want no fixed session", command.Args, err)
	}
}

// TestUsageFallsBackToTheTranscript (K10, R8-06): an ended launch's usage
// is its own measurement when that read the whole session; else the
// session's transcript (a launch cancelled where no supervisor measured
// it); else unknown, with why. A launch that has not ended has no final
// usage.
func TestUsageFallsBackToTheTranscript(t *testing.T) {
	t.Parallel()
	m, _, _, _ := manager(t)
	projects := t.TempDir()
	m.Adapters["claude-headless"] = ClaudeHeadless{ProjectsRoot: projects, Scanner: scan{}}
	measured, _ := seedClaude(t, m, "measured")
	_, _ = m.Store.Update(measured.ID, func(record *Record) error {
		record.State, record.Measurement = Completed, Measurement{InputTokens: 40, OutputTokens: 2, UsageRead: true}
		return nil
	})
	if usage, err := m.Usage(measured.ID); err != nil || usage != (Usage{Known: true, Tokens: 42, Source: "measure"}) {
		t.Fatalf("a measured launch: %+v %v", usage, err)
	}
	skipped, _ := seedClaude(t, m, "skipped")
	_, _ = m.Store.Update(skipped.ID, func(record *Record) error {
		record.State, record.Measured = Cancelled, true
		record.AdapterData["sessionID"] = rawString("sid-skipped")
		return nil
	})
	if usage, err := m.Usage(skipped.ID); err != nil || usage.Known || usage.Why == "" {
		t.Fatalf("no transcript: %+v %v; want unknown, with why", usage, err)
	}
	writeTranscript(t, projects, "sid-skipped")
	if usage, err := m.Usage(skipped.ID); err != nil || usage != (Usage{Known: true, Tokens: 30, Source: "transcript"}) {
		t.Fatalf("a skipped measure: %+v %v; want the transcript's 30 tokens", usage, err)
	}
	running, _ := seedClaude(t, m, "running")
	if _, err := m.Usage(running.ID); err == nil {
		t.Fatal("a launch that has not ended has a final usage")
	}
}
