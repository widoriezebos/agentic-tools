package events

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// The shell emitter's caller-boundary legs of flight-recorder-fixtures.sh
// (verbs-object-action U6b retired emit-event.sh; every emitter is this Go
// one now): an unwritable stream or an absent root never fails or panics
// the caller, and the checked form names the failure.
func TestEmitNeverFailsItsCallerOnAnUnwritableStream(t *testing.T) {
	t.Parallel()
	root := sandbox(t, leaseRegistry)
	stream := filepath.Join(root, "artifacts", "agents", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(stream), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stream, nil, 0o000); err != nil {
		t.Fatal(err)
	}
	emitter := &Emitter{Component: "lease", Pid: 1, PidStartedAt: 2}
	emitter.Emit(root, "lease-claimed", "probe", map[string]string{"epoch": "1"})
	if err := emitter.EmitChecked(root, "lease-claimed", "probe", nil); err == nil && os.Geteuid() != 0 {
		t.Fatal("the checked emitter reported success into an unwritable stream")
	}
	(&Emitter{Component: "lease", Pid: 1}).Emit("", "lease-claimed", "probe", nil)
	(&Emitter{Component: "lease", Pid: 1}).Emit(filepath.Join(root, "absent", "root"), "lease-claimed", "probe", nil)
}

// Concurrent writers: framing keeps every writer's every event parseable,
// and each writer's sequence is gapless.
func TestConcurrentWritersKeepEveryEventParseableAndEachSequenceGapless(t *testing.T) {
	t.Parallel()
	root := sandbox(t, `{"events":{"job-created":{"emitters":["dispatch"]}}}`)
	const writers, events = 6, 30
	var group sync.WaitGroup
	for writer := 1; writer <= writers; writer++ {
		group.Add(1)
		go func(pid int64) {
			defer group.Done()
			emitter := &Emitter{Component: "dispatch", Pid: pid, PidStartedAt: 100}
			for index := 1; index <= events; index++ {
				emitter.Emit(root, "job-created", "s", map[string]string{"jobId": fmt.Sprintf("w%d-%d", pid, index)})
			}
		}(int64(writer))
	}
	group.Wait()
	lines := readLines(t, root)
	if len(lines) != writers*events {
		t.Fatalf("%d events, want %d", len(lines), writers*events)
	}
	sequences := map[float64][]float64{}
	for _, line := range lines {
		sequences[line["pid"].(float64)] = append(sequences[line["pid"].(float64)], line["seq"].(float64))
	}
	for pid, got := range sequences {
		sort.Float64s(got)
		for index, seq := range got {
			if seq != float64(index+1) {
				t.Fatalf("writer %v sequence has a gap: %v", pid, got)
			}
		}
	}
}

// A torn fragment (a short write without its framing) cannot poison the
// next writer: the next event starts on its own line and parses.
func TestATornFragmentDoesNotPoisonTheNextEvent(t *testing.T) {
	t.Parallel()
	root := sandbox(t, leaseRegistry)
	stream := filepath.Join(root, "artifacts", "agents", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(stream), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stream, []byte("\n{\"torn\": tru"), 0o644); err != nil {
		t.Fatal(err)
	}
	(&Emitter{Component: "lease", Pid: 3}).Emit(root, "lease-claimed", "after-torn", map[string]string{"epoch": "2"})
	data, err := os.ReadFile(stream)
	if err != nil {
		t.Fatal(err)
	}
	var healthy []string
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" && !strings.HasPrefix(line, `{"torn"`) {
			healthy = append(healthy, line)
		}
	}
	if len(healthy) != 1 || !strings.Contains(healthy[0], `"summary":"after-torn"`) {
		t.Fatalf("the event after the torn fragment did not survive whole: %q", data)
	}
}
