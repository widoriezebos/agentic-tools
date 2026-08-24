package outage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)

// The mark's lifecycle: absent → recorded → fed → cleared, with the
// count growing only while the outage keeps being fed.
func TestMarkLifecycle(t *testing.T) {
	root := t.TempDir()
	if _, ok := Read(root); ok {
		t.Fatal("an absent mark must read as no outage")
	}
	m, err := Record(root, "overloaded", "API Error: 529", "mission-runner", t0)
	if err != nil {
		t.Fatal(err)
	}
	if m.ConsecutiveFailures != 1 || m.LastClass != "overloaded" || m.Since != m.LastAt {
		t.Fatalf("first failure starts the outage: %+v", m)
	}
	m, err = Record(root, "http-529", "still down", "delegate-adapter", t0.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if m.ConsecutiveFailures != 2 || m.Since == m.LastAt || m.Source != "delegate-adapter" {
		t.Fatalf("a fed outage keeps its start and grows its count: %+v", m)
	}
	if _, ok := StandingAt(root, t0.Add(3*time.Minute)); !ok {
		t.Fatal("a fed mark stands")
	}
	if err := Clear(root); err != nil {
		t.Fatal(err)
	}
	if _, ok := Read(root); ok {
		t.Fatal("a cleared mark is gone")
	}
	if err := Clear(root); err != nil {
		t.Fatal("clearing an absent mark is success")
	}
}

// The horizon: a mark nobody feeds lapses, so a forgotten mark can
// never permanently pause the steward's clocks; and a NEW failure
// after the lapse starts a fresh outage rather than resuming the old.
func TestHorizonLapse(t *testing.T) {
	root := t.TempDir()
	if _, err := Record(root, "overloaded", "529", "mission-runner", t0); err != nil {
		t.Fatal(err)
	}
	if _, ok := StandingAt(root, t0.Add(Horizon)); !ok {
		t.Fatal("a mark inside the horizon stands")
	}
	if _, ok := StandingAt(root, t0.Add(Horizon+time.Second)); ok {
		t.Fatal("a mark past the horizon has lapsed")
	}
	if _, ok := StandingAt(root, t0.Add(-Horizon-time.Second)); ok {
		t.Fatal("a future-dated mark lapses on the same bound; a clock correction must not pause the clocks indefinitely")
	}
	m, err := Record(root, "overloaded", "529 again", "mission-runner", t0.Add(2*Horizon))
	if err != nil {
		t.Fatal(err)
	}
	if m.ConsecutiveFailures != 1 || m.Since != t0.Add(2*Horizon).UTC().Format(time.RFC3339) {
		t.Fatalf("a failure after the lapse starts a new outage: %+v", m)
	}
}

// A torn mark fails toward normal operation: no outage, and the next
// Record starts clean.
func TestTornMarkReadsAsNoOutage(t *testing.T) {
	root := t.TempDir()
	path := Path(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := Read(root); ok {
		t.Fatal("a torn mark must not stand")
	}
	m, err := Record(root, "overloaded", "529", "mission-runner", t0)
	if err != nil || m.ConsecutiveFailures != 1 {
		t.Fatalf("recording over a torn mark starts clean: %+v %v", m, err)
	}
}

// The classifier's line rule: the provider's own words and status codes
// hit; ordinary diagnostics and near-miss numbers do not.
func TestClassifyLogs(t *testing.T) {
	cases := []struct {
		line  string
		class string
	}{
		{`API Error: 529 {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, "overloaded"},
		{"api error: status 503 service unavailable", "http-503"},
		{"HTTP 503 Service Unavailable", "http-503"},
		{"upstream returned error 502", "http-502"},
		{"request failed with status 500", "http-500"},
		{"API status 504 gateway timeout", "http-504"},
		{"server error (501)", "http-501"},
		{"stream error: code 529", "http-529"},
		{"unexpected http 507 from upstream", "http-507"},
		{"error after 500ms", ""},
		{"took 503 ms retrying", ""},
		{"processed 529 records", ""},
		{"error: took 5030ms", ""},
		{"error at line 15290", ""},
		{"failed to decode 500 records", ""},
		{"the local scheduler is overloaded", ""},
		{"the pipeline is overloadedly verbose", ""},
		{"all good", ""},
	}
	for _, c := range cases {
		dir := t.TempDir()
		log := filepath.Join(dir, "host.log")
		if err := os.WriteFile(log, []byte("benign line\n"+c.line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		class, evidence, ok := ClassifyLogs(log)
		if c.class == "" {
			if ok {
				t.Fatalf("%q must not classify (got %s: %s)", c.line, class, evidence)
			}
			continue
		}
		if !ok || class != c.class {
			t.Fatalf("%q: want %s, got %s (ok=%v)", c.line, c.class, class, ok)
		}
	}
	if _, _, ok := ClassifyLogs("", filepath.Join(t.TempDir(), "absent.log")); ok {
		t.Fatal("absent logs classify nothing")
	}
}

// The structured gate: a provider result classifies only when it
// declares itself an error. A successful result whose model output
// merely DISCUSSES a 529 must never mark an outage.
func TestClassifyProviderResultGate(t *testing.T) {
	dir := t.TempDir()
	talking := filepath.Join(dir, "success.json")
	if err := os.WriteFile(talking, []byte(
		`{"is_error":false,"result":"we fixed the 529 overloaded error handling"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := ClassifyProviderResult(talking); ok {
		t.Fatal("model output discussing an outage must not mark one")
	}
	erring := filepath.Join(dir, "error.json")
	if err := os.WriteFile(erring, []byte(
		`{"is_error":true,"result":"API Error: 529 Overloaded"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	class, _, ok := ClassifyProviderResult(erring)
	if !ok || class != "overloaded" {
		t.Fatalf("a declared error with overload words classifies: %s %v", class, ok)
	}
	if _, _, ok := ClassifyProviderResult(filepath.Join(dir, "absent.json")); ok {
		t.Fatal("an absent result classifies nothing")
	}
}

// Long evidence is clipped; a long log is tail-read without error.
func TestEvidenceClipAndTail(t *testing.T) {
	root := t.TempDir()
	m, err := Record(root, "overloaded", strings.Repeat("x", 1000), "mission-runner", t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.LastDetail) != evidenceClip {
		t.Fatalf("evidence must clip to %d, got %d", evidenceClip, len(m.LastDetail))
	}
	log := filepath.Join(t.TempDir(), "big.log")
	big := strings.Repeat("filler line\n", 10000) + "api error: status 529\n"
	if err := os.WriteFile(log, []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	if class, _, ok := ClassifyLogs(log); !ok || class != "http-529" {
		t.Fatalf("the tail of a large log still classifies: %s %v", class, ok)
	}
}
