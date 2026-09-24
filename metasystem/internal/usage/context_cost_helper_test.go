package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

type contextCostReaderWork struct {
	BytesRead       int64 `json:"bytesRead"`
	LinesParsed     int   `json:"linesParsed"`
	TranscriptOpens int   `json:"transcriptOpens"`
	CursorWrites    int   `json:"cursorWrites"`
	SampleWrites    int   `json:"sampleWrites"`
}

// TestContextCostColdReaderHelper is a subprocess helper for the opt-in
// full-Stop cost bed. It pauses only after the real reader holds its cursor
// lock, so the competing non-blocking Stop has a deterministic busy outcome.
func TestContextCostColdReaderHelper(t *testing.T) {
	if os.Getenv("METASYSTEM_CONTEXT_COST_READER_HELPER") != "1" {
		return
	}
	root := os.Getenv("METASYSTEM_CONTEXT_COST_READER_ROOT")
	runtimeName := os.Getenv("METASYSTEM_CONTEXT_COST_READER_RUNTIME")
	session := os.Getenv("METASYSTEM_CONTEXT_COST_READER_SESSION")
	transcript := os.Getenv("METASYSTEM_CONTEXT_COST_READER_TRANSCRIPT")
	home := os.Getenv("METASYSTEM_CONTEXT_COST_READER_HOME")
	toplevel := os.Getenv("METASYSTEM_CONTEXT_COST_READER_TOPLEVEL")
	if root == "" || runtimeName == "" || session == "" || transcript == "" || home == "" || toplevel == "" {
		t.Fatal("context cost reader helper received incomplete coordinates")
	}

	work := contextCostReaderWork{}
	var pause sync.Once
	callBytesRead = func(count int) {
		if count < 1 {
			return
		}
		work.BytesRead += int64(count)
		if os.Getenv("METASYSTEM_CONTEXT_COST_READER_PAUSE") == "1" {
			pause.Do(func() {
				ready := os.NewFile(3, "context-cost-reader-ready")
				release := os.NewFile(4, "context-cost-reader-release")
				if ready == nil || release == nil {
					t.Fatal("context cost reader helper received no coordination pipes")
				}
				defer ready.Close()
				defer release.Close()
				if _, err := ready.Write([]byte{'x'}); err != nil {
					t.Fatal(err)
				}
				var signal [1]byte
				if _, err := io.ReadFull(release, signal[:]); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	callJSONDecodes = func() { work.LinesParsed++ }
	callFileOpens = func(path string) {
		switch path {
		case transcript:
			work.TranscriptOpens++
		case SamplesPath(root, runtimeName, session):
			work.SampleWrites++
		}
	}
	priorCursorWriter := writeCallCursor
	writeCallCursor = func(path string, value any) error {
		work.CursorWrites++
		return priorCursorWriter(path, value)
	}
	defer func() {
		callBytesRead = nil
		callJSONDecodes = nil
		callFileOpens = nil
		writeCallCursor = priorCursorWriter
	}()

	reading, err := LatestCall(root, runtimeName, session, ReadOptions{
		Capability: PerCall, Transcript: transcript, Home: home, Toplevel: toplevel,
		Now: time.Now().UTC(), NonBlocking: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Latest == nil || reading.Latest.PromptTokens != 120000 {
		t.Fatalf("cold reader latest=%+v, want 120000 prompt tokens", reading.Latest)
	}
	fmt.Printf("context-cost-cold-reader=live prompt-tokens=%d\n", reading.Latest.PromptTokens)
	encoded, err := json.Marshal(work)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("context-cost-reader-work=%s\n", encoded)
}
