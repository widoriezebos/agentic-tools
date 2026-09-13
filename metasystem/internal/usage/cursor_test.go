package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestUnchangedTranscriptReadsNothingAndReturnsTheLatest(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	writeCallRows(t, transcript, claudeAssistant("only", 40, 2, 3, false, "2026-09-13T14:00:00Z"))
	opts := ReadOptions{Capability: PerCall, Transcript: transcript}
	first, err := LatestCall(stateRoot, "claude", "unchanged", opts)
	if err != nil {
		t.Fatal(err)
	}

	bytesRead := 0
	previous := callBytesRead
	callBytesRead = func(count int) { bytesRead += count }
	t.Cleanup(func() { callBytesRead = previous })
	second, err := LatestCall(stateRoot, "claude", "unchanged", opts)
	if err != nil {
		t.Fatal(err)
	}
	if bytesRead != 0 {
		t.Fatalf("second read consumed %d bytes", bytesRead)
	}
	firstJSON, _ := json.Marshal(first.Latest)
	secondJSON, _ := json.Marshal(second.Latest)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("latest changed: first=%s second=%s", firstJSON, secondJSON)
	}
	if second.NewSamples != 0 || second.NewMarkers != 0 {
		t.Fatalf("second read added samples=%d markers=%d", second.NewSamples, second.NewMarkers)
	}
}

func TestUnchangedEmptyTranscriptDoesNotRepublishCursor(t *testing.T) {
	originalWriter := writeCallCursor
	t.Cleanup(func() { writeCallCursor = originalWriter })

	for _, nonempty := range []bool{false, true} {
		name := "empty"
		if nonempty {
			name = "nonempty"
		}
		t.Run(name, func(t *testing.T) {
			stateRoot := t.TempDir()
			transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
			if nonempty {
				writeCallRows(t, transcript, claudeAssistant("only", 1, 0, 0, false, "2026-09-13T14:00:00Z"))
			} else if err := os.WriteFile(transcript, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			opts := ReadOptions{Capability: PerCall, Transcript: transcript}
			first, err := LatestCall(stateRoot, "claude", "unchanged-"+name, opts)
			if err != nil {
				t.Fatal(err)
			}
			if first.Cursor.SchemaVersion != 2 {
				t.Fatalf("first read did not establish a valid cursor: %#v", first.Cursor)
			}

			bytesRead := 0
			transcriptOpens := 0
			cursorWrites := 0
			previousBytes := callBytesRead
			previousOpens := callFileOpens
			callBytesRead = func(count int) { bytesRead += count }
			callFileOpens = func(path string) {
				if path == transcript {
					transcriptOpens++
				}
			}
			writeCallCursor = func(path string, value any) error {
				cursorWrites++
				return originalWriter(path, value)
			}
			defer func() {
				callBytesRead = previousBytes
				callFileOpens = previousOpens
				writeCallCursor = originalWriter
			}()

			second, err := LatestCall(stateRoot, "claude", "unchanged-"+name, opts)
			if err != nil {
				t.Fatal(err)
			}
			if bytesRead != 0 || transcriptOpens != 0 || cursorWrites != 0 {
				t.Fatalf("unchanged read used bytes=%d transcript-opens=%d cursor-writes=%d", bytesRead, transcriptOpens, cursorWrites)
			}
			if second.NewSamples != 0 || second.NewMarkers != 0 {
				t.Fatalf("unchanged read added samples=%d markers=%d", second.NewSamples, second.NewMarkers)
			}
		})
	}
}

func TestLatestCallReturnsThePreviousReadTime(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	writeCallRows(t, transcript, claudeAssistant("first", 1, 0, 0, false, "2026-09-13T14:00:00Z"))
	session := "previous-read-at"
	firstAt := time.Date(2026, 9, 13, 14, 1, 0, 0, time.UTC)
	secondAt := firstAt.Add(time.Minute)
	thirdAt := secondAt.Add(time.Minute)
	opts := ReadOptions{Capability: PerCall, Transcript: transcript, Now: firstAt}

	first, err := LatestCall(stateRoot, "claude", session, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !first.PreviousReadAt.IsZero() || !first.Cursor.LastReadAt.Equal(firstAt) {
		t.Fatalf("first reading times previous=%s current=%s", first.PreviousReadAt, first.Cursor.LastReadAt)
	}

	opts.Now = secondAt
	unchanged, err := LatestCall(stateRoot, "claude", session, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !unchanged.PreviousReadAt.Equal(firstAt) || !unchanged.Cursor.LastReadAt.Equal(firstAt) {
		t.Fatalf("unchanged reading times previous=%s current=%s", unchanged.PreviousReadAt, unchanged.Cursor.LastReadAt)
	}

	appendCallTestRow(t, transcript, claudeAssistant("second", 2, 0, 0, false, "2026-09-13T14:02:00Z"))
	appended, err := LatestCall(stateRoot, "claude", session, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !appended.PreviousReadAt.Equal(firstAt) || !appended.Cursor.LastReadAt.Equal(secondAt) {
		t.Fatalf("appended reading times previous=%s current=%s", appended.PreviousReadAt, appended.Cursor.LastReadAt)
	}

	replacement := filepath.Join(t.TempDir(), "replacement.jsonl")
	writeCallRows(t, replacement, claudeAssistant("replacement", 3, 0, 0, false, "2026-09-13T14:03:00Z"))
	opts.Transcript = replacement
	opts.Now = thirdAt
	restarted, err := LatestCall(stateRoot, "claude", session, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !restarted.PreviousReadAt.Equal(secondAt) || !restarted.Cursor.LastReadAt.Equal(thirdAt) {
		t.Fatalf("restarted reading times previous=%s current=%s", restarted.PreviousReadAt, restarted.Cursor.LastReadAt)
	}

	unsupported, err := LatestCall(t.TempDir(), "devin", "unsupported", ReadOptions{Capability: PerInvocation, Now: thirdAt})
	if err != nil {
		t.Fatal(err)
	}
	if !unsupported.PreviousReadAt.IsZero() {
		t.Fatalf("unsupported capability returned previous read time %s", unsupported.PreviousReadAt)
	}
}

func TestRotatedTranscriptRestartsTheCursor(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	opts := ReadOptions{Capability: PerCall, Transcript: transcript}
	writeCallRows(t, transcript,
		claudeAssistant("one", 1, 0, 0, false, "2026-09-13T15:00:00Z"),
		claudeAssistant("two", 2, 0, 0, false, "2026-09-13T15:01:00Z"),
		claudeAssistant("three", 3, 0, 0, false, "2026-09-13T15:02:00Z"),
	)
	if _, err := LatestCall(stateRoot, "claude", "rotated", opts); err != nil {
		t.Fatal(err)
	}
	old, err := os.Open(transcript)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if err := os.Remove(transcript); err != nil {
		t.Fatal(err)
	}
	large := claudeAssistant("replacement", 10, 0, 0, false, "2026-09-13T15:03:00Z")
	large = strings.TrimSuffix(large, "}") + `,"padding":"` + strings.Repeat("x", 4096) + `"}`
	writeCallRows(t, transcript, large)
	replaced, err := LatestCall(stateRoot, "claude", "rotated", opts)
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Cursor.SampleCount != 1 || replaced.Cursor.Offset != fileSize(t, transcript) {
		t.Fatalf("replacement cursor = %#v", replaced.Cursor)
	}
	if replaced.Latest == nil || !strings.Contains(replaced.Latest.Source, "restarted: inode changed") {
		t.Fatalf("replacement latest = %#v", replaced.Latest)
	}

	if err := os.Truncate(transcript, 0); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(transcript, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString(claudeAssistant("after-truncate", 20, 0, 0, false, "2026-09-13T15:04:00Z") + "\n")
	closeErr := file.Close()
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	truncated, err := LatestCall(stateRoot, "claude", "rotated", opts)
	if err != nil {
		t.Fatal(err)
	}
	if truncated.Cursor.SampleCount != 1 || truncated.Latest == nil || !strings.Contains(truncated.Latest.Source, "restarted: truncated") {
		t.Fatalf("truncated reading = %#v", truncated)
	}
}

func TestConcurrentReadersCountEachSampleOnce(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(transcript, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	opts := ReadOptions{Capability: PerCall, Transcript: transcript}
	writerDone := make(chan struct{})
	writerErr := make(chan error, 1)
	go func() {
		defer close(writerDone)
		file, err := os.OpenFile(transcript, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			writerErr <- err
			return
		}
		defer file.Close()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for index := 0; index < 200; index++ {
			<-ticker.C
			row := claudeAssistant("request-"+decimal(index), int64(index), 0, 0, false, "2026-09-13T16:00:00Z") + "\n"
			if _, err := file.WriteString(row); err != nil {
				writerErr <- err
				return
			}
		}
	}()

	var readers sync.WaitGroup
	readerErrors := make(chan error, 8)
	for index := 0; index < 8; index++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-writerDone:
					_, err := LatestCall(stateRoot, "claude", "concurrent", opts)
					if err != nil {
						readerErrors <- err
					}
					return
				default:
					if _, err := LatestCall(stateRoot, "claude", "concurrent", opts); err != nil {
						readerErrors <- err
						return
					}
				}
			}
		}()
	}
	readers.Wait()
	close(readerErrors)
	for err := range readerErrors {
		t.Error(err)
	}
	select {
	case err := <-writerErr:
		t.Fatal(err)
	default:
	}
	final, err := LatestCall(stateRoot, "claude", "concurrent", opts)
	if err != nil {
		t.Fatal(err)
	}
	samples, _, err := Calls(stateRoot, "claude", "concurrent", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 200 || final.Cursor.SampleCount != 200 {
		t.Fatalf("sample rows=%d cursor count=%d, want 200", len(samples), final.Cursor.SampleCount)
	}
	if final.Cursor.Offset != fileSize(t, transcript) {
		t.Fatalf("offset=%d size=%d", final.Cursor.Offset, fileSize(t, transcript))
	}
}

func TestCursorKeepsAnUnterminatedTail(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	row := claudeAssistant("tail", 12, 3, 4, false, "2026-09-13T17:00:00Z")
	if err := os.WriteFile(transcript, []byte(row[:len(row)/2]), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := ReadOptions{Capability: PerCall, Transcript: transcript}
	first, err := LatestCall(stateRoot, "claude", "tail", opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.Latest != nil || len(first.Cursor.Tail) == 0 {
		t.Fatalf("first reading = %#v", first)
	}
	file, err := os.OpenFile(transcript, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString(row[len(row)/2:] + "\n")
	closeErr := file.Close()
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	second, err := LatestCall(stateRoot, "claude", "tail", opts)
	if err != nil {
		t.Fatal(err)
	}
	if second.Latest == nil || second.Latest.PromptTokens != 19 || len(second.Cursor.Tail) != 0 {
		t.Fatalf("second reading = %#v", second)
	}
}

func TestMalformedCursorStartsFresh(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	writeCallRows(t, transcript, claudeAssistant("fresh", 7, 0, 0, false, "2026-09-13T18:00:00Z"))
	cursorPath := CursorPath(stateRoot, "claude", "malformed")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cursorPath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	reading, err := LatestCall(stateRoot, "claude", "malformed", ReadOptions{Capability: PerCall, Transcript: transcript})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Cursor.SchemaVersion != 2 || reading.Cursor.SampleCount != 1 || reading.Latest == nil {
		t.Fatalf("reading = %#v", reading)
	}
}

func TestSemanticallyForeignCursorStartsFresh(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	writeCallRows(t, transcript, claudeAssistant("actual", 7, 0, 0, false, "2026-09-13T18:30:00Z"))
	info, err := os.Stat(transcript)
	if err != nil {
		t.Fatal(err)
	}
	stat := info.Sys().(*syscall.Stat_t)
	foreign := CursorState{
		SchemaVersion: 1,
		Runtime:       "codex",
		Session:       "someone-else",
		Path:          transcript,
		Dev:           uint64(stat.Dev),
		Inode:         uint64(stat.Ino),
		Size:          info.Size(),
		Offset:        info.Size(),
		Latest: &CallSample{
			Runtime: "codex", Session: "someone-else", InvocationID: "foreign",
			PromptTokens: 999, InputTokens: 999, Source: "codex-rollout",
		},
		SampleCount: 1,
		Seen:        map[string]bool{"foreign": true},
	}
	cursorPath := CursorPath(stateRoot, "claude", "semantic")
	if err := os.MkdirAll(filepath.Dir(cursorPath), 0o755); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cursorPath, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	reading, err := LatestCall(stateRoot, "claude", "semantic", ReadOptions{Capability: PerCall, Transcript: transcript})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Latest == nil || reading.Latest.InvocationID != "actual" || reading.Cursor.SampleCount != 1 {
		t.Fatalf("reading = %#v", reading)
	}
}

func TestScanErrorDoesNotAppendBeforeCursorAdvances(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	writeCallRows(t, transcript,
		claudeAssistant("valid", 1, 0, 0, false, "2026-09-13T18:45:00Z"),
		strings.Repeat("x", maxCallLineBytes+1),
	)
	opts := ReadOptions{Capability: PerCall, Transcript: transcript}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := LatestCall(stateRoot, "claude", "overlong", opts); err == nil || !strings.Contains(err.Error(), "token too long") {
			t.Fatalf("attempt %d error = %v", attempt+1, err)
		}
		if got := jsonlLineCountIfPresent(t, SamplesPath(stateRoot, "claude", "overlong")); got != 0 {
			t.Fatalf("attempt %d left %d sample rows", attempt+1, got)
		}
	}
}

func TestOnlySidechainsRemainUnknown(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	writeCallRows(t, transcript, claudeAssistant("side", 100, 0, 0, true, "2026-09-13T19:00:00Z"))
	reading, err := LatestCall(stateRoot, "claude", "sidechain", ReadOptions{Capability: PerCall, Transcript: transcript})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Latest != nil || reading.Reason != "unknown (only sidechain records so far)" {
		t.Fatalf("reading = %#v", reading)
	}
}

func TestSidechainCountSurvivesIncrementalReads(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	writeCallRows(t, transcript, claudeAssistant("first", 10, 0, 0, false, "2026-09-13T19:10:00Z"))
	opts := ReadOptions{Capability: PerCall, Transcript: transcript}
	if _, err := LatestCall(stateRoot, "claude", "sidechain-count", opts); err != nil {
		t.Fatal(err)
	}
	appendCallTestRow(t, transcript, claudeAssistant("side", 100, 0, 0, true, "2026-09-13T19:11:00Z"))
	second, err := LatestCall(stateRoot, "claude", "sidechain-count", opts)
	if err != nil {
		t.Fatal(err)
	}
	if second.Latest == nil || !strings.Contains(second.Latest.Source, "sidechain records skipped: 1") {
		t.Fatalf("second reading = %#v", second)
	}
	appendCallTestRow(t, transcript, claudeAssistant("third", 30, 0, 0, false, "2026-09-13T19:12:00Z"))
	third, err := LatestCall(stateRoot, "claude", "sidechain-count", opts)
	if err != nil {
		t.Fatal(err)
	}
	if third.Latest == nil || third.Latest.InvocationID != "third" || !strings.Contains(third.Latest.Source, "sidechain records skipped: 1") {
		t.Fatalf("third reading = %#v", third)
	}
}

func TestClaudePhysicalLineIdentitySurvivesIncrementalReads(t *testing.T) {
	for _, splitTail := range []bool{false, true} {
		name := "complete-append"
		if splitTail {
			name = "split-tail"
		}
		t.Run(name, func(t *testing.T) {
			stateRoot := t.TempDir()
			transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
			requestless := claudeAssistant("", 6, 0, 0, false, "2026-09-13T21:00:00Z")
			prefix := strings.Join([]string{
				claudeAssistant("", 1, 0, 0, false, "2026-09-13T20:55:00Z"),
				`{"type":"user"}`,
				"",
				"not json",
				claudeAssistant("side", 99, 0, 0, true, "2026-09-13T20:59:00Z"),
			}, "\n") + "\n"
			firstBytes := []byte(prefix)
			if splitTail {
				firstBytes = append(firstBytes, requestless[:len(requestless)/2]...)
			}
			if err := os.WriteFile(transcript, firstBytes, 0o644); err != nil {
				t.Fatal(err)
			}
			opts := ReadOptions{Capability: PerCall, Transcript: transcript}
			first, err := LatestCall(stateRoot, "claude", "physical-lines-"+name, opts)
			if err != nil {
				t.Fatal(err)
			}
			if first.Cursor.Line != 5 || first.Cursor.SidechainCount != 1 {
				t.Fatalf("first progress = line %d, sidechains %d", first.Cursor.Line, first.Cursor.SidechainCount)
			}

			marker := `{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":6}}`
			appendText := requestless + "\n" + marker + "\n"
			if splitTail {
				appendText = requestless[len(requestless)/2:] + "\n" + marker + "\n"
			}
			appendCallTestBytes(t, transcript, []byte(appendText))
			bytesRead := 0
			previous := callBytesRead
			callBytesRead = func(count int) { bytesRead += count }
			t.Cleanup(func() { callBytesRead = previous })
			second, err := LatestCall(stateRoot, "claude", "physical-lines-"+name, opts)
			if err != nil {
				t.Fatal(err)
			}
			if bytesRead != len(appendText) {
				t.Fatalf("incremental bytes = %d, want %d", bytesRead, len(appendText))
			}
			if second.Latest == nil || second.Latest.InvocationID != "line:6" || second.Latest.Ordinal != 6 {
				t.Fatalf("latest = %#v", second.Latest)
			}
			if second.Cursor.Line != 7 || second.Cursor.SidechainCount != 1 || second.Cursor.LastMarker == nil || second.Cursor.LastMarker.Ordinal != 7 {
				t.Fatalf("second progress = %#v", second.Cursor)
			}
			bytesRead = 0
			unchanged, err := LatestCall(stateRoot, "claude", "physical-lines-"+name, opts)
			if err != nil {
				t.Fatal(err)
			}
			if bytesRead != 0 || unchanged.Cursor.Line != 7 {
				t.Fatalf("unchanged bytes=%d cursor=%#v", bytesRead, unchanged.Cursor)
			}
		})
	}
}

func TestCursorSchemaRequiresExplicitProgressFields(t *testing.T) {
	t.Run("schema-1-rebuilds-the-prefix", func(t *testing.T) {
		stateRoot := t.TempDir()
		transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
		prefix := "not json\n" + claudeAssistant("side", 90, 0, 0, true, "2026-09-13T21:01:00Z") + "\n"
		if err := os.WriteFile(transcript, []byte(prefix), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(transcript)
		if err != nil {
			t.Fatal(err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		legacy := CursorState{
			SchemaVersion: 1, Runtime: "claude", Session: "schema-one", Path: transcript,
			Dev: uint64(stat.Dev), Inode: uint64(stat.Ino), Size: info.Size(), Offset: info.Size(), Seen: map[string]bool{},
		}
		writeCallCursorFixture(t, CursorPath(stateRoot, "claude", "schema-one"), legacy)
		appendCallTestRow(t, transcript, claudeAssistant("", 7, 0, 0, false, "2026-09-13T21:02:00Z"))
		reading, err := LatestCall(stateRoot, "claude", "schema-one", ReadOptions{Capability: PerCall, Transcript: transcript})
		if err != nil {
			t.Fatal(err)
		}
		if reading.Cursor.SchemaVersion != 2 || reading.Cursor.Line != 3 || reading.Cursor.SidechainCount != 1 || reading.Latest == nil || reading.Latest.InvocationID != "line:3" {
			t.Fatalf("rebuilt reading = %#v", reading)
		}
		if !strings.Contains(reading.Latest.Source, "sidechain records skipped: 1") {
			t.Fatalf("rebuilt source = %q", reading.Latest.Source)
		}
	})

	invalidCases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing-line", func(raw map[string]any) { delete(raw, "line") }},
		{"missing-sidechain-count", func(raw map[string]any) { delete(raw, "sidechainCount") }},
		{"missing-samples-bytes", func(raw map[string]any) { delete(raw, "samplesBytes") }},
		{"null-line", func(raw map[string]any) { raw["line"] = nil }},
		{"null-sidechain-count", func(raw map[string]any) { raw["sidechainCount"] = nil }},
		{"null-samples-bytes", func(raw map[string]any) { raw["samplesBytes"] = nil }},
		{"negative-line", func(raw map[string]any) { raw["line"] = -1 }},
		{"negative-sidechain-count", func(raw map[string]any) { raw["sidechainCount"] = -1 }},
		{"negative-samples-bytes", func(raw map[string]any) { raw["samplesBytes"] = -1 }},
		{"sidechains-exceed-lines", func(raw map[string]any) { raw["line"], raw["sidechainCount"] = 0, 1 }},
		{"lines-exceed-consumed-bytes", func(raw map[string]any) { raw["line"] = int64(1 << 30) }},
	}
	for _, test := range invalidCases {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
			writeCallRows(t, transcript, claudeAssistant("rebuilt", 1, 0, 0, false, "2026-09-13T21:03:00Z"))
			info, err := os.Stat(transcript)
			if err != nil {
				t.Fatal(err)
			}
			stat := info.Sys().(*syscall.Stat_t)
			fixture := CursorState{
				SchemaVersion: 2, Runtime: "claude", Session: test.name, Path: transcript,
				Dev: uint64(stat.Dev), Inode: uint64(stat.Ino), Size: info.Size(), Offset: info.Size(), Line: 1,
				Seen: map[string]bool{},
			}
			raw := cursorMap(t, fixture)
			test.mutate(raw)
			writeCallCursorFixture(t, CursorPath(stateRoot, "claude", test.name), raw)
			reading, err := LatestCall(stateRoot, "claude", test.name, ReadOptions{Capability: PerCall, Transcript: transcript})
			if err != nil {
				t.Fatal(err)
			}
			if reading.NewSamples != 1 || reading.Latest == nil || reading.Latest.InvocationID != "rebuilt" || reading.Cursor.Line != 1 {
				t.Fatalf("invalid cursor was reused: %#v", reading)
			}
		})
	}

	t.Run("codex-provider-ordinal-is-independent", func(t *testing.T) {
		stateRoot := t.TempDir()
		transcript := filepath.Join(t.TempDir(), "rollout.jsonl")
		writeCallRows(t, transcript, codexUsage("first", 1, 0, 0, 9001, "2026-09-13T21:04:00Z"))
		opts := ReadOptions{Capability: PerCall, Transcript: transcript}
		first, err := LatestCall(stateRoot, "codex", "large-provider-ordinal", opts)
		if err != nil {
			t.Fatal(err)
		}
		if first.Cursor.Line != 1 || first.Latest == nil || first.Latest.Ordinal != 9001 {
			t.Fatalf("first reading = %#v", first)
		}
		row := codexUsage("second", 2, 0, 0, 9002, "2026-09-13T21:05:00Z") + "\n"
		appendCallTestBytes(t, transcript, []byte(row))
		bytesRead := 0
		previous := callBytesRead
		callBytesRead = func(count int) { bytesRead += count }
		t.Cleanup(func() { callBytesRead = previous })
		second, err := LatestCall(stateRoot, "codex", "large-provider-ordinal", opts)
		if err != nil {
			t.Fatal(err)
		}
		if bytesRead != len(row) || second.Cursor.Line != 2 || second.Latest == nil || second.Latest.Ordinal != 9002 {
			t.Fatalf("incremental reading bytes=%d reading=%#v", bytesRead, second)
		}
	})
}

func TestCursorWriteFailureRetriesWithoutDuplicateRows(t *testing.T) {
	originalWriter := writeCallCursor
	t.Cleanup(func() { writeCallCursor = originalWriter })

	t.Run("initial-checkpoint-failure-appends-nothing", func(t *testing.T) {
		stateRoot := t.TempDir()
		transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
		writeCallRows(t, transcript, claudeAssistant("main", 1, 0, 0, false, "2026-09-13T21:06:00Z"))
		writeCallCursor = func(string, any) error { return errors.New("injected cursor failure") }
		if _, err := LatestCall(stateRoot, "claude", "checkpoint-failure", ReadOptions{Capability: PerCall, Transcript: transcript}); err == nil {
			t.Fatal("initial checkpoint failure was accepted")
		}
		if _, err := os.Stat(CursorPath(stateRoot, "claude", "checkpoint-failure")); !os.IsNotExist(err) {
			t.Fatalf("cursor exists after checkpoint failure: %v", err)
		}
		if _, err := os.Stat(SamplesPath(stateRoot, "claude", "checkpoint-failure")); !os.IsNotExist(err) {
			t.Fatalf("samples exist after checkpoint failure: %v", err)
		}
	})

	for _, existing := range []bool{false, true} {
		name := "first-read"
		if existing {
			name = "existing-checkpoint"
		}
		t.Run(name, func(t *testing.T) {
			writeCallCursor = originalWriter
			stateRoot := t.TempDir()
			transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
			session := "cursor-retry-" + name
			opts := ReadOptions{Capability: PerCall, Transcript: transcript}
			if existing {
				writeCallRows(t, transcript, claudeAssistant("old", 1, 0, 0, false, "2026-09-13T21:07:00Z"))
				if _, err := LatestCall(stateRoot, "claude", session, opts); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(transcript, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			appendCallTestRow(t, transcript, claudeAssistant("main", 2, 0, 0, false, "2026-09-13T21:08:00Z"))
			appendCallTestRow(t, transcript, claudeAssistant("", 3, 0, 0, false, "2026-09-13T21:09:00Z"))
			appendCallTestRow(t, transcript, `{"type":"system","subtype":"compact_boundary","timestamp":"2026-09-13T21:10:00Z","compactMetadata":{"trigger":"auto","preTokens":3}}`)

			cursorPath := CursorPath(stateRoot, "claude", session)
			oldCursor := readCallTestFileIfPresent(t, cursorPath)
			oldBoundary := int64(0)
			oldOffset := int64(0)
			if existing {
				var cursor CursorState
				if err := json.Unmarshal(oldCursor, &cursor); err != nil {
					t.Fatal(err)
				}
				oldBoundary, oldOffset = cursor.SamplesBytes, cursor.Offset
			}
			writeCallCursor = func(path string, value any) error {
				candidate := value.(CursorState)
				if candidate.Offset > oldOffset {
					return errors.New("injected final cursor failure")
				}
				return originalWriter(path, value)
			}
			if _, err := LatestCall(stateRoot, "claude", session, opts); err == nil {
				t.Fatal("final cursor failure was accepted")
			}
			failedCursorBytes := readCallTestFile(t, cursorPath)
			var failedCursor CursorState
			if err := json.Unmarshal(failedCursorBytes, &failedCursor); err != nil {
				t.Fatal(err)
			}
			if failedCursor.Offset != oldOffset || failedCursor.SamplesBytes != oldBoundary {
				t.Fatalf("checkpoint advanced after failure: %#v", failedCursor)
			}
			if existing && !bytes.Equal(failedCursorBytes, oldCursor) {
				t.Fatal("existing cursor bytes changed after failed publication")
			}
			if fileSize(t, SamplesPath(stateRoot, "claude", session)) <= oldBoundary {
				t.Fatal("failed append did not leave the expected uncommitted suffix")
			}

			writeCallCursor = originalWriter
			final, err := LatestCall(stateRoot, "claude", session, opts)
			if err != nil {
				t.Fatal(err)
			}
			samples, markers, err := Calls(stateRoot, "claude", session, time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			wantSamples := 2
			if existing {
				wantSamples++
			}
			if len(samples) != wantSamples || len(markers) != 1 || final.Cursor.SampleCount != int64(wantSamples) || final.Cursor.CompactionCount != 1 {
				t.Fatalf("samples=%d markers=%d cursor=%#v", len(samples), len(markers), final.Cursor)
			}
			assertJSONLKindCount(t, SamplesPath(stateRoot, "claude", session), "sample", wantSamples)
			assertJSONLKindCount(t, SamplesPath(stateRoot, "claude", session), "marker", 1)
			if final.Cursor.Offset != fileSize(t, transcript) || final.Cursor.SamplesBytes != fileSize(t, SamplesPath(stateRoot, "claude", session)) {
				t.Fatalf("final boundaries = %#v", final.Cursor)
			}
		})
	}
}

func TestReadersRecoverUncommittedSampleSuffix(t *testing.T) {
	for _, reader := range []string{"calls", "latest"} {
		t.Run(reader, func(t *testing.T) {
			stateRoot := t.TempDir()
			transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
			session := "recover-" + reader
			writeCallRows(t, transcript, claudeAssistant("committed", 1, 0, 0, false, "2026-09-13T21:11:00Z"))
			opts := ReadOptions{Capability: PerCall, Transcript: transcript}
			if _, err := LatestCall(stateRoot, "claude", session, opts); err != nil {
				t.Fatal(err)
			}
			cursorPath := CursorPath(stateRoot, "claude", session)
			samplesPath := SamplesPath(stateRoot, "claude", session)
			committedCursor := readCallTestFile(t, cursorPath)
			committedSamples := readCallTestFile(t, samplesPath)
			extraSample := encodedCallRow(t, sampleRow{Kind: "sample", CallSample: CallSample{Runtime: "claude", Session: session, InvocationID: "uncommitted", PromptTokens: 99, InputTokens: 99, At: time.Now().UTC(), Ordinal: 2, Source: "claude-transcript"}})
			extraMarker := encodedCallRow(t, markerRow{Kind: "marker", Marker: Marker{Runtime: "claude", Session: session, Kind: "compaction", At: time.Now().UTC(), Ordinal: 3}})
			appendCallTestBytes(t, samplesPath, append(append(append(extraSample, '\n'), append(extraMarker, '\n')...), []byte(`{"kind":"sample"`)...))

			if reader == "calls" {
				samples, markers, err := Calls(stateRoot, "claude", session, time.Time{})
				if err != nil {
					t.Fatal(err)
				}
				if len(samples) != 1 || len(markers) != 0 || samples[0].InvocationID != "committed" {
					t.Fatalf("recovered rows samples=%#v markers=%#v", samples, markers)
				}
			} else {
				bytesRead := 0
				previous := callBytesRead
				callBytesRead = func(count int) { bytesRead += count }
				t.Cleanup(func() { callBytesRead = previous })
				reading, err := LatestCall(stateRoot, "claude", session, opts)
				if err != nil {
					t.Fatal(err)
				}
				if bytesRead != 0 || reading.Latest == nil || reading.Latest.InvocationID != "committed" {
					t.Fatalf("unchanged recovery bytes=%d reading=%#v", bytesRead, reading)
				}
			}
			if got := readCallTestFile(t, cursorPath); !bytes.Equal(got, committedCursor) {
				t.Fatal("recovery rewrote the committed cursor")
			}
			if got := readCallTestFile(t, samplesPath); !bytes.Equal(got, committedSamples) {
				t.Fatalf("recovered samples differ:\n got %q\nwant %q", got, committedSamples)
			}
		})
	}
}

func TestPartialSampleAppendRetriesFromCommittedBoundary(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	session := "partial-sample"
	writeCallRows(t, transcript, claudeAssistant("first", 1, 0, 0, false, "2026-09-13T21:12:00Z"))
	opts := ReadOptions{Capability: PerCall, Transcript: transcript}
	if _, err := LatestCall(stateRoot, "claude", session, opts); err != nil {
		t.Fatal(err)
	}
	samplesPath := SamplesPath(stateRoot, "claude", session)
	committed := readCallTestFile(t, samplesPath)
	partialRow := encodedCallRow(t, sampleRow{Kind: "sample", CallSample: CallSample{Runtime: "claude", Session: session, InvocationID: "second", PromptTokens: 2, InputTokens: 2, At: time.Now().UTC(), Ordinal: 2, Source: "claude-transcript"}})
	appendCallTestBytes(t, samplesPath, partialRow[:len(partialRow)/2])
	appendCallTestRow(t, transcript, claudeAssistant("second", 2, 0, 0, false, "2026-09-13T21:13:00Z"))
	reading, err := LatestCall(stateRoot, "claude", session, opts)
	if err != nil {
		t.Fatal(err)
	}
	if reading.Latest == nil || reading.Latest.InvocationID != "second" {
		t.Fatalf("reading = %#v", reading)
	}
	finalBytes := readCallTestFile(t, samplesPath)
	if !bytes.Equal(finalBytes[:len(committed)], committed) {
		t.Fatal("the committed sample prefix changed")
	}
	lines := bytes.Split(bytes.TrimSuffix(finalBytes, []byte{'\n'}), []byte{'\n'})
	if len(lines) != 2 {
		t.Fatalf("raw sample lines = %d, want 2", len(lines))
	}
	for index, line := range lines {
		var row map[string]any
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("raw line %d is torn: %v: %q", index+1, err, line)
		}
	}
	samples, _, err := Calls(stateRoot, "claude", session, time.Time{})
	if err != nil || len(samples) != 2 {
		t.Fatalf("samples=%#v err=%v", samples, err)
	}
}

func TestUntrustedOrShortSampleLogIsPreserved(t *testing.T) {
	type fixture struct {
		stateRoot, transcript, cursorPath, samplesPath, session string
	}
	newFixture := func(t *testing.T, name string) fixture {
		t.Helper()
		stateRoot := t.TempDir()
		transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
		writeCallRows(t, transcript, claudeAssistant("committed", 1, 0, 0, false, "2026-09-13T21:14:00Z"))
		if _, err := LatestCall(stateRoot, "claude", name, ReadOptions{Capability: PerCall, Transcript: transcript}); err != nil {
			t.Fatal(err)
		}
		return fixture{stateRoot, transcript, CursorPath(stateRoot, "claude", name), SamplesPath(stateRoot, "claude", name), name}
	}
	tests := []struct {
		name                string
		boundaryUnavailable bool
		mutate              func(*testing.T, fixture)
	}{
		{"missing-cursor", true, func(t *testing.T, f fixture) {
			t.Helper()
			if err := os.Remove(f.cursorPath); err != nil {
				t.Fatal(err)
			}
		}},
		{"corrupt-cursor", true, func(t *testing.T, f fixture) {
			t.Helper()
			if err := os.WriteFile(f.cursorPath, []byte("not json"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"schema-one-cursor", true, func(t *testing.T, f fixture) {
			t.Helper()
			raw := cursorMapFromFile(t, f.cursorPath)
			raw["schemaVersion"] = 1
			writeCallCursorFixture(t, f.cursorPath, raw)
		}},
		{"missing-progress-field", true, func(t *testing.T, f fixture) {
			t.Helper()
			raw := cursorMapFromFile(t, f.cursorPath)
			delete(raw, "samplesBytes")
			writeCallCursorFixture(t, f.cursorPath, raw)
		}},
		{"short-samples", false, func(t *testing.T, f fixture) {
			t.Helper()
			data := readCallTestFile(t, f.samplesPath)
			if err := os.WriteFile(f.samplesPath, data[:len(data)-1], 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing-samples", false, func(t *testing.T, f fixture) {
			t.Helper()
			if err := os.Remove(f.samplesPath); err != nil {
				t.Fatal(err)
			}
		}},
		{"cursor-read-error", false, func(t *testing.T, f fixture) {
			t.Helper()
			if err := os.Remove(f.cursorPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(f.cursorPath, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"non-regular-samples", false, func(t *testing.T, f fixture) {
			t.Helper()
			if err := os.Remove(f.samplesPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(f.samplesPath, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t, test.name)
			test.mutate(t, f)
			cursorBefore := snapshotCallPath(t, f.cursorPath)
			samplesBefore := snapshotCallPath(t, f.samplesPath)
			_, _, callsErr := Calls(f.stateRoot, "claude", f.session, time.Time{})
			if callsErr == nil {
				t.Fatal("Calls accepted unavailable committed evidence")
			}
			assertCallPathSnapshot(t, f.cursorPath, cursorBefore)
			assertCallPathSnapshot(t, f.samplesPath, samplesBefore)
			_, latestErr := LatestCall(f.stateRoot, "claude", f.session, ReadOptions{Capability: PerCall, Transcript: f.transcript})
			if latestErr == nil {
				t.Fatal("LatestCall accepted unavailable committed evidence")
			}
			assertCallPathSnapshot(t, f.cursorPath, cursorBefore)
			assertCallPathSnapshot(t, f.samplesPath, samplesBefore)
			if test.boundaryUnavailable {
				for _, err := range []error{callsErr, latestErr} {
					if !strings.Contains(err.Error(), f.cursorPath) || !strings.Contains(err.Error(), f.samplesPath) || !strings.Contains(err.Error(), "committed boundary") {
						t.Fatalf("boundary error does not name both files: %v", err)
					}
				}
			}
		})
	}
}

func TestTranscriptRestartKeepsCommittedSampleHistory(t *testing.T) {
	originalWriter := writeCallCursor
	t.Cleanup(func() { writeCallCursor = originalWriter })
	for _, cause := range []string{"inode changed", "truncated", "path changed"} {
		t.Run(cause, func(t *testing.T) {
			writeCallCursor = originalWriter
			stateRoot := t.TempDir()
			transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
			session := "restart-" + strings.ReplaceAll(cause, " ", "-")
			writeCallRows(t, transcript,
				claudeAssistant("old", 1, 0, 0, false, "2026-09-13T21:15:00Z"),
				claudeAssistant("side", 99, 0, 0, true, "2026-09-13T21:16:00Z"),
				strings.Repeat("x", 4096),
			)
			opts := ReadOptions{Capability: PerCall, Transcript: transcript}
			first, err := LatestCall(stateRoot, "claude", session, opts)
			if err != nil {
				t.Fatal(err)
			}
			if first.Cursor.Line != 3 || first.Cursor.SidechainCount != 1 {
				t.Fatalf("initial cursor = %#v", first.Cursor)
			}
			cursorPath := CursorPath(stateRoot, "claude", session)
			samplesPath := SamplesPath(stateRoot, "claude", session)
			oldCursorBytes := readCallTestFile(t, cursorPath)
			oldSamplesBytes := readCallTestFile(t, samplesPath)
			newPath := transcript
			newRow := claudeAssistant("", 2, 0, 0, false, "2026-09-13T21:17:00Z")
			switch cause {
			case "inode changed":
				oldFile, err := os.Open(transcript)
				if err != nil {
					t.Fatal(err)
				}
				defer oldFile.Close()
				if err := os.Remove(transcript); err != nil {
					t.Fatal(err)
				}
				writeCallRows(t, transcript, newRow)
			case "truncated":
				writeCallRows(t, transcript, newRow)
			case "path changed":
				newPath = filepath.Join(t.TempDir(), "replacement.jsonl")
				writeCallRows(t, newPath, newRow)
				opts.Transcript = newPath
			}

			failed := false
			writeCallCursor = func(string, any) error {
				if !failed {
					failed = true
					return errors.New("injected restart publication failure")
				}
				return errors.New("unexpected second cursor write")
			}
			if _, err := LatestCall(stateRoot, "claude", session, opts); err == nil {
				t.Fatal("restart publication failure was accepted")
			}
			if got := readCallTestFile(t, cursorPath); !bytes.Equal(got, oldCursorBytes) {
				t.Fatal("restart failure changed the committed cursor")
			}
			if got := fileSize(t, samplesPath); got <= int64(len(oldSamplesBytes)) {
				t.Fatalf("restart failure did not leave a suffix: %d", got)
			}

			writeCallCursor = originalWriter
			final, err := LatestCall(stateRoot, "claude", session, opts)
			if err != nil {
				t.Fatal(err)
			}
			if final.Cursor.Path != newPath || final.Cursor.Line != 1 || final.Cursor.SidechainCount != 0 || final.Cursor.SampleCount != 1 || final.Latest == nil || final.Latest.InvocationID != "line:1" || final.Latest.Ordinal != 1 {
				t.Fatalf("final restart = %#v", final)
			}
			if !strings.Contains(final.Latest.Source, "restarted: "+cause) || strings.Contains(final.Latest.Source, sidechainSourceLabel) {
				t.Fatalf("restart source = %q", final.Latest.Source)
			}
			samples, _, err := Calls(stateRoot, "claude", session, time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			if len(samples) != 2 || samples[0].InvocationID != "old" || samples[1].InvocationID != "line:1" {
				t.Fatalf("history = %#v", samples)
			}
			if final.Cursor.SamplesBytes <= int64(len(oldSamplesBytes)) || final.Cursor.SamplesBytes != fileSize(t, samplesPath) {
				t.Fatalf("sample boundary = %d, old=%d", final.Cursor.SamplesBytes, len(oldSamplesBytes))
			}
		})
	}
}

func TestSidechainOnlyProgressSurvivesBeforeFirstSample(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	session := "sidechain-before-sample"
	writeCallRows(t, transcript,
		claudeAssistant("side-one", 90, 0, 0, true, "2026-09-13T21:18:00Z"),
		claudeAssistant("side-two", 91, 0, 0, true, "2026-09-13T21:19:00Z"),
	)
	opts := ReadOptions{Capability: PerCall, Transcript: transcript}
	first, err := LatestCall(stateRoot, "claude", session, opts)
	if err != nil {
		t.Fatal(err)
	}
	wantReason := "unknown (only sidechain records so far)"
	if first.Latest != nil || first.Reason != wantReason || first.Cursor.Line != 2 || first.Cursor.SidechainCount != 2 {
		t.Fatalf("first sidechain reading = %#v", first)
	}
	bytesRead := 0
	previous := callBytesRead
	callBytesRead = func(count int) { bytesRead += count }
	t.Cleanup(func() { callBytesRead = previous })
	for attempt := 0; attempt < 2; attempt++ {
		unchanged, err := LatestCall(stateRoot, "claude", session, opts)
		if err != nil {
			t.Fatal(err)
		}
		if unchanged.Latest != nil || unchanged.Reason != wantReason || unchanged.Cursor.SidechainCount != 2 {
			t.Fatalf("unchanged sidechain reading = %#v", unchanged)
		}
	}
	if bytesRead != 0 {
		t.Fatalf("unchanged reads consumed %d transcript bytes", bytesRead)
	}
	appendCallTestRow(t, transcript, `{"type":"user"}`)
	appendCallTestRow(t, transcript, claudeAssistant("main-one", 1, 0, 0, false, "2026-09-13T21:20:00Z"))
	mainOne, err := LatestCall(stateRoot, "claude", session, opts)
	if err != nil {
		t.Fatal(err)
	}
	if mainOne.Latest == nil || !strings.Contains(mainOne.Latest.Source, "sidechain records skipped: 2") || mainOne.Cursor.Line != 4 {
		t.Fatalf("first main reading = %#v", mainOne)
	}
	samplesPath := SamplesPath(stateRoot, "claude", session)
	firstSampleBytes := readCallTestFile(t, samplesPath)
	appendCallTestRow(t, transcript, claudeAssistant("side-three", 92, 0, 0, true, "2026-09-13T21:21:00Z"))
	sidechainOnly, err := LatestCall(stateRoot, "claude", session, opts)
	if err != nil {
		t.Fatal(err)
	}
	if sidechainOnly.Latest == nil || !strings.Contains(sidechainOnly.Latest.Source, "sidechain records skipped: 3") || sidechainOnly.Cursor.SidechainCount != 3 {
		t.Fatalf("sidechain-only increment = %#v", sidechainOnly)
	}
	if got := readCallTestFile(t, samplesPath); !bytes.Equal(got, firstSampleBytes) {
		t.Fatal("sidechain-only read rewrote historical sample rows")
	}
	appendCallTestRow(t, transcript, claudeAssistant("main-two", 2, 0, 0, false, "2026-09-13T21:22:00Z"))
	mainTwo, err := LatestCall(stateRoot, "claude", session, opts)
	if err != nil {
		t.Fatal(err)
	}
	if mainTwo.Latest == nil || !strings.Contains(mainTwo.Latest.Source, "sidechain records skipped: 3") || mainTwo.Cursor.Line != 6 {
		t.Fatalf("second main reading = %#v", mainTwo)
	}
	samples, _, err := Calls(stateRoot, "claude", session, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 2 || !strings.Contains(samples[0].Source, "sidechain records skipped: 2") || !strings.Contains(samples[1].Source, "sidechain records skipped: 3") {
		t.Fatalf("persisted samples = %#v", samples)
	}
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func decimal(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[position:])
}

func jsonlLineCountIfPresent(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0
	}
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

func appendCallTestRow(t *testing.T, path, row string) {
	t.Helper()
	appendCallTestBytes(t, path, []byte(row+"\n"))
}

func appendCallTestBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}

func writeCallCursorFixture(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
}

func cursorMap(t *testing.T, cursor CursorState) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func cursorMapFromFile(t *testing.T, path string) map[string]any {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(readCallTestFile(t, path), &raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func encodedCallRow(t *testing.T, row any) []byte {
	t.Helper()
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func readCallTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readCallTestFileIfPresent(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type callPathSnapshot struct {
	exists bool
	mode   os.FileMode
	data   []byte
}

func snapshotCallPath(t *testing.T, path string) callPathSnapshot {
	t.Helper()
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return callPathSnapshot{}
	}
	if err != nil {
		t.Fatal(err)
	}
	snapshot := callPathSnapshot{exists: true, mode: info.Mode()}
	if info.Mode().IsRegular() {
		snapshot.data = readCallTestFile(t, path)
	}
	return snapshot
}

func assertCallPathSnapshot(t *testing.T, path string, want callPathSnapshot) {
	t.Helper()
	got := snapshotCallPath(t, path)
	if got.exists != want.exists || got.mode != want.mode || !bytes.Equal(got.data, want.data) {
		t.Fatalf("path %s changed: got=%#v want=%#v", path, got, want)
	}
}
