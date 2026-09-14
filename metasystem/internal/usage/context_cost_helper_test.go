package usage

import (
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

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

	ready := os.NewFile(3, "context-cost-reader-ready")
	release := os.NewFile(4, "context-cost-reader-release")
	if ready == nil || release == nil {
		t.Fatal("context cost reader helper received no coordination pipes")
	}
	defer ready.Close()
	defer release.Close()

	var pause sync.Once
	callBytesRead = func(count int) {
		if count < 1 {
			return
		}
		pause.Do(func() {
			if _, err := ready.Write([]byte{'x'}); err != nil {
				t.Fatal(err)
			}
			var signal [1]byte
			if _, err := io.ReadFull(release, signal[:]); err != nil {
				t.Fatal(err)
			}
		})
	}
	defer func() { callBytesRead = nil }()

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
}
