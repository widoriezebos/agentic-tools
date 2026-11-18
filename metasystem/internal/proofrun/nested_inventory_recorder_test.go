package proofrun

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNestedInventoryRecorderStreamsBoundedEvidence(t *testing.T) {
	var record bytes.Buffer
	r := newNestedInventoryRecorder(func(format string, args ...any) {
		fmt.Fprintf(&record, format+"\n", args...)
	})
	emit := func(action, name, output string, elapsed float64) {
		t.Helper()
		event := nestedInventoryEvent{goEvent: goEvent{Action: action, Package: "example/nested", Test: name, Output: output}, Time: "2026-11-18T10:00:00Z", Elapsed: elapsed}
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		// os/exec's writes need not align with JSON records.
		data = append(data, '\n')
		for len(data) > 0 {
			n := min(len(data), 317)
			if written, err := r.Write(data[:n]); err != nil || written != n {
				t.Fatalf("write=%d err=%v", written, err)
			}
			data = data[n:]
		}
	}
	emit("start", "", "", 0)
	emit("run", "TestPass", "", 0)
	emit("pass", "TestPass", "", .1)
	emit("run", "TestSlow", "", 0)
	emit("pass", "TestSlow", "", 42)
	emit("run", "TestFailed", "", 0)
	emit("output", "TestFailed", "child stderr: synthetic helper diagnostic\n"+strings.Repeat("x", nestedOutputLimit)+"ordinary tail omitted", 0)
	emit("output", "TestFailed", "panic: test timed out after 3m0s\n\ngoroutine 17 [chan receive]:\nexample/helper.wait()\n"+strings.Repeat("s", nestedStackLimit)+"stack tail omitted", 0)
	emit("fail", "TestFailed", "", 180)
	emit("run", "TestSkipped", "", 0)
	emit("skip", "TestSkipped", "", 0)
	emit("run", "TestIncomplete", "", 0)
	emit("output", "TestIncomplete", "helper stderr before termination\npanic: incomplete helper\ngoroutine 9 [select]:\n", 0)

	// These assertions precede finish: a killed outer binary has no finalizer.
	live := record.String()
	for _, want := range []string{"time=2026-11-18T10:00:00Z", "test=TestIncomplete action=run", "child stderr: synthetic helper diagnostic", "panic: test timed out after 3m0s", "goroutine 17 [chan receive]", "example/helper.wait()", "helper stderr before termination", "panic: incomplete helper", "action=fail elapsed=180.000s", "output TRUNCATED", "stack TRUNCATED"} {
		if !strings.Contains(live, want) {
			t.Errorf("live record lacks %q", want)
		}
	}
	for _, unwanted := range []string{"ordinary tail omitted", "stack tail omitted"} {
		if strings.Contains(live, unwanted) {
			t.Errorf("record exceeded bound: contains %q", unwanted)
		}
	}
	if strings.Count(live, "TRUNCATED") != 2 || len(live) > nestedOutputLimit+nestedStackLimit+8000 {
		t.Errorf("unexpected record bounds: bytes=%d truncations=%d", len(live), strings.Count(live, "TRUNCATED"))
	}
	r.finish(errors.New("synthetic termination"))
	for _, want := range []string{"test=TestFailed status=fail", "test=TestSkipped status=skip", "test=TestIncomplete status=incomplete", "slowest=1 package=example/nested test=TestFailed status=fail elapsed=180.000s", "slowest=2 package=example/nested test=TestSlow status=pass elapsed=42.000s", "finished exit=synthetic termination"} {
		if !strings.Contains(record.String(), want) {
			t.Errorf("finished record lacks %q", want)
		}
	}
	_, missing, _, complete := parseGoJSON(r.inventory.Bytes(), nil)
	if complete || len(missing) != 1 || missing[0].Name != "TestIncomplete" {
		t.Fatalf("inventory lost incomplete identity: complete=%v missing=%v", complete, missing)
	}
	if bytes.Contains(r.inventory.Bytes(), []byte("helper stderr")) {
		t.Error("inventory retained diagnostic payload")
	}
}

func TestNestedInventoryRecorderPreservesInventoryVerdictAndSlowLimit(t *testing.T) {
	var record bytes.Buffer
	r := newNestedInventoryRecorder(func(format string, args ...any) { fmt.Fprintf(&record, format+"\n", args...) })
	var original bytes.Buffer
	encoder := json.NewEncoder(&original)
	var expected []NativeTestIdentity
	for i := 0; i < nestedSlowLimit+3; i++ {
		name := fmt.Sprintf("Test%d", i)
		expected = append(expected, NativeTestIdentity{Classname: "example/nested", Name: name})
		for _, action := range []string{"run", "output", "pass"} {
			if i == 0 && action == "pass" {
				action = "fail"
			}
			if err := encoder.Encode(nestedInventoryEvent{goEvent: goEvent{Action: action, Package: "example/nested", Test: name, Output: "failure detail\n"}, Elapsed: float64(i)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A final JSON event without a newline and plain stderr are also retained.
	_, _ = r.Write([]byte("go: synthetic stderr\n"))
	_, _ = r.Write(bytes.TrimSuffix(original.Bytes(), []byte{'\n'}))
	r.finish(nil)
	observed, notPassed, err := standardInventoryVerdict(r.inventory.Bytes(), nil, expected)
	if err != nil || notPassed != 1 || len(observed) != len(expected) {
		t.Fatalf("complete nested red changed inventory verdict: observed=%v notPassed=%d err=%v", observed, notPassed, err)
	}
	if strings.Count(record.String(), "slowest=") != nestedSlowLimit {
		t.Error("slowest summary did not respect its bound")
	}
	for _, want := range []string{"go: synthetic stderr", "failure detail", "test=Test0 status=fail", "test=Test12 status=pass elapsed=12.000s"} {
		if !strings.Contains(record.String(), want) {
			t.Errorf("passing inventory record lacks %q", want)
		}
	}
}
