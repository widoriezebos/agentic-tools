package proofrun

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

const (
	nestedOutputLimit = 256 << 10
	nestedStackLimit  = 256 << 10
	nestedSlowLimit   = 10
)

type nestedInventoryEvent struct {
	goEvent
	Time    string
	Elapsed float64
}

type nestedInventoryTest struct {
	pkg, name, status string
	elapsed           float64
	output, stack     int
	stackStarted      bool
	outputTruncated   bool
	stackTruncated    bool
}

// nestedInventoryRecorder streams evidence before knowing the child's verdict:
// waiting for a failure would lose output from a child or parent that is killed.
// Each identity gets an output head and a separate stack head, so noisy output
// before a timeout cannot consume the stack allowance. Package and non-JSON
// stderr output have their own identities and the same bounds.
// os/exec serializes writes when Stdout and Stderr are the same writer.
// Only inventory events are retained in memory for the existing verdict owner.
// The sink must stream (testing.T.Logf does so under the proof's -json mode).
// No test namespace file or final cleanup is needed for retention.
type nestedInventoryRecorder struct {
	logf      func(string, ...any)
	pending   []byte
	inventory bytes.Buffer
	tests     map[string]*nestedInventoryTest
}

func newNestedInventoryRecorder(logf func(string, ...any)) *nestedInventoryRecorder {
	return &nestedInventoryRecorder{logf: logf, tests: map[string]*nestedInventoryTest{}}
}

func (r *nestedInventoryRecorder) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			r.pending = append(r.pending, p...)
			break
		}
		r.pending = append(r.pending, p[:i]...)
		r.line(r.pending)
		r.pending = r.pending[:0]
		p = p[i+1:]
	}
	return n, nil
}

func (r *nestedInventoryRecorder) line(line []byte) {
	var event nestedInventoryEvent
	if json.Unmarshal(line, &event) != nil {
		event = nestedInventoryEvent{goEvent: goEvent{Action: "output", Package: "stderr", Output: string(line) + "\n"}}
	}
	key := event.Package + "/" + event.Test
	item := r.tests[key]
	if item == nil {
		item = &nestedInventoryTest{pkg: event.Package, name: event.Test}
		r.tests[key] = item
	}
	if event.Action == "output" {
		r.output(item, event.Output)
		return
	}
	// Output is irrelevant to parseGoJSON. Keep every other event unchanged
	// in meaning, including package events and missing-terminal evidence.
	event.Output = ""
	encoded, _ := json.Marshal(event.goEvent)
	r.inventory.Write(encoded)
	r.inventory.WriteByte('\n')
	if event.Action == "run" || event.Action == "start" {
		item.status = "incomplete"
	}
	if event.Action == "pass" || event.Action == "fail" || event.Action == "skip" {
		item.status, item.elapsed = event.Action, event.Elapsed
	}
	r.logf("nested inventory: time=%s package=%s test=%s action=%s elapsed=%.3fs", event.Time, event.Package, event.Test, event.Action, event.Elapsed)
}

func (r *nestedInventoryRecorder) output(item *nestedInventoryTest, output string) {
	if !item.stackStarted {
		// Go timeout/panic output can follow a large amount of ordinary test
		// output. Start the reserved allowance at its first stack header.
		start := -1
		for _, marker := range []string{"panic:", "fatal error:", "goroutine "} {
			if i := strings.Index(output, marker); i >= 0 && (start < 0 || i < start) {
				start = i
			}
		}
		if start >= 0 {
			r.boundedOutput(item, output[:start], false)
			item.stackStarted = true
			output = output[start:]
		}
	}
	r.boundedOutput(item, output, item.stackStarted)
}

func (r *nestedInventoryRecorder) boundedOutput(item *nestedInventoryTest, output string, stack bool) {
	used, truncated, limit, kind := &item.output, &item.outputTruncated, nestedOutputLimit, "output"
	if stack {
		used, truncated, limit, kind = &item.stack, &item.stackTruncated, nestedStackLimit, "stack"
	}
	n := min(len(output), limit-*used)
	if n > 0 {
		r.logf("nested inventory: package=%s test=%s %s: %s", item.pkg, item.name, kind, output[:n])
		*used += n
	}
	if n < len(output) && !*truncated {
		r.logf("nested inventory: package=%s test=%s %s TRUNCATED after %d bytes; head retained", item.pkg, item.name, kind, limit)
		*truncated = true
	}
}

func (r *nestedInventoryRecorder) finish(runErr error) {
	if len(r.pending) > 0 {
		r.line(r.pending)
		r.pending = nil
	}
	keys := make([]string, 0, len(r.tests))
	for key := range r.tests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var slow []*nestedInventoryTest
	for _, key := range keys {
		item := r.tests[key]
		if item.status != "" && item.status != "pass" {
			r.logf("nested inventory: non-pass package=%s test=%s status=%s elapsed=%.3fs (terminal duration; unavailable if incomplete)", item.pkg, item.name, item.status, item.elapsed)
		}
		if item.name != "" && (item.status == "pass" || item.status == "fail" || item.status == "skip") {
			slow = append(slow, item)
		}
	}
	sort.SliceStable(slow, func(i, j int) bool { return slow[i].elapsed > slow[j].elapsed })
	for i, item := range slow[:min(len(slow), nestedSlowLimit)] {
		r.logf("nested inventory: slowest=%d package=%s test=%s status=%s elapsed=%.3fs", i+1, item.pkg, item.name, item.status, item.elapsed)
	}
	r.logf("nested inventory: finished exit=%v", runErr)
}
