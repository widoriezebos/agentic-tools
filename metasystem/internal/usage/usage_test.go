package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWaitMeasurementAccounting(t *testing.T) {
	second := int64(time.Second)
	base := waitMeasureReturn{WaitID: "w", Nonce: "n", Kind: "attempt", TargetID: "a", Runtime: "codex", Mode: "register", State: "ready", SourceEvidence: "attempt:a:d", RegisteredBootNanos: 10 * second, PrevObservedBootNanos: 20 * second, ObservedBootNanos: 30 * second, ReturnedBootNanos: 40 * second, RegisteredBootID: "boot", PrevObservedBootID: "boot", ObservedBootID: "boot", ReturnedBootID: "boot", StampsFrom: "event"}
	hint := waitMeasureHint{Kind: "attempt", TargetID: "a", PublicationID: "attempt:a:d", BeganBootNanos: 15 * second, PublishedBootNanos: 25 * second, BeganBootID: "boot", PublishedBootID: "boot"}
	check := func(name string, result waitMeasureReturn, hints []waitMeasureHint, verdict, unavailable, loose string) {
		t.Helper()
		report := WaitRuntimeMeasurement{Counts: map[string]int{}, Defects: map[string]int{}}
		sample := evaluateWaitSample(result, hints, &report)
		if sample.Verdict != verdict || sample.UnavailableReason != unavailable || sample.LooseEdgeReason != loose || strings.Contains(strings.ToLower(fmt.Sprint(sample)), "pass") {
			t.Fatalf("%s: %+v defects=%v", name, sample, report.Defects)
		}
	}
	check("hinted", base, []waitMeasureHint{hint}, "returned-within-60", "", "")
	foreign := hint
	foreign.PublicationID = "attempt:a:other"
	check("foreign publication", base, []waitMeasureHint{foreign}, "returned-within-60", "", "no-matching-publication")
	refuted := base
	refuted.ReturnedBootNanos, refuted.ObservedBootNanos = 100*second, 90*second
	check("refuted on boot clock", refuted, []waitMeasureHint{hint}, "refuted", "", "")
	suspect := refuted
	suspect.ReturnedBootNanos, suspect.ObservedBootNanos = 80*second, 40*second
	check("straddles bound", suspect, []waitMeasureHint{hint}, "suspect", "", "")
	atEntry := base
	atEntry.AtEntry = true
	check("at entry without hint", atEntry, nil, "", "no-lower-edge", "no-matching-publication")
	foreignBootHint := hint
	foreignBootHint.PublishedBootID = "another-boot"
	report := WaitRuntimeMeasurement{Counts: map[string]int{}, Defects: map[string]int{}}
	foreignBootSample := evaluateWaitSample(base, []waitMeasureHint{foreignBootHint}, &report)
	if foreignBootSample.Verdict != "returned-within-60" || foreignBootSample.UnavailableReason != "" || foreignBootSample.LooseEdgeReason != "no-matching-publication" || foreignBootSample.LowerEdge != "prev-observation" || foreignBootSample.UpperEdge != "observation" {
		t.Fatalf("foreign-boot hint changed a self-comparable sample: %+v", foreignBootSample)
	}
	mismatch := base
	mismatch.ReturnedBootID = "another-boot"
	check("return boot mismatch", mismatch, []waitMeasureHint{hint}, "", "clock-not-comparable", "no-matching-publication")
	foreignBegan := hint
	foreignBegan.BeganBootID = "another-boot"
	check("joined foreign began stamp", base, []waitMeasureHint{foreignBegan}, "", "clock-not-comparable", "")
	ordered := base
	ordered.PrevObservedBootNanos = 35 * second
	check("stamp order", ordered, []waitMeasureHint{hint}, "", "stamp-order", "")
	check("ambiguous publication", base, []waitMeasureHint{hint, hint}, "returned-within-60", "", "publication-ambiguous")
	check("hinted at entry", atEntry, []waitMeasureHint{hint}, "returned-within-60", "", "")
	missing := base
	missing.SourceEvidence = ""
	check("missing join key", missing, nil, "returned-within-60", "", "join-key-missing")

	root := t.TempDir()
	stamp := "2026-09-16T10:00:00Z"
	emitter := &events.Emitter{Component: "run", Pid: 10, PidStartedAt: 20}
	emit := func(name string, fields map[string]string) {
		t.Helper()
		if err := emitter.EmitChecked(root, name, "wait measurement fixture", fields); err != nil {
			t.Fatal(err)
		}
	}
	returnFields := func(waitID, nonce, mode, state string, registered, previous, observed, returned int64) map[string]string {
		return map[string]string{
			"waitId": waitID, "nonce": nonce, "kind": "attempt", "targetId": waitID, "runtime": "codex", "mode": mode, "state": state,
			"sourceEvidence": "attempt:" + waitID + ":d", "registeredAt": stamp, "returnedAt": stamp,
			"registeredBootNanos": strconv.FormatInt(registered, 10), "prevObservedBootNanos": strconv.FormatInt(previous, 10),
			"observedBootNanos": strconv.FormatInt(observed, 10), "returnedBootNanos": strconv.FormatInt(returned, 10),
			"registeredBootId": "boot", "prevObservedBootId": "boot", "observedBootId": "boot", "returnedBootId": "boot", "atEntry": "false",
		}
	}
	emit("wait-returned", returnFields("event", "n", "register", "ready", 10*second, 20*second, 90*second, 100*second))
	emit("wait-published", map[string]string{"kind": "attempt", "targetId": "event", "publicationId": "attempt:event:d", "beganBootNanos": strconv.FormatInt(15*second, 10), "publishedBootNanos": strconv.FormatInt(25*second, 10), "beganBootId": "boot", "publishedBootId": "boot"})
	// A replay with the same identity must not erase the original sample.
	emit("wait-returned", returnFields("event", "n", "replay", "ready", 10*second, 20*second, 90*second, 101*second))
	emit("wait-returned", returnFields("replay-only", "n", "replay", "ready", 10*second, 20*second, 30*second, 40*second))
	emit("wait-returned", returnFields("interrupted", "n", "register", "interrupted", 10*second, 20*second, 30*second, 40*second))
	rowResult := map[string]any{"waitId": "row", "nonce": "n", "runtime": "codex", "mode": "register", "state": "ready", "sourceEvidence": "attempt:row:d", "registeredAt": stamp, "returnedAt": stamp, "registeredBootNanos": 10 * second, "prevObservedBootNanos": 20 * second, "observedBootNanos": 30 * second, "returnedBootNanos": 40 * second, "registeredBootId": "boot", "prevObservedBootId": "boot", "observedBootId": "boot", "returnedBootId": "boot", "selector": map[string]any{"kind": "attempt", "targetId": "row"}}
	writeFile(t, filepath.Join(root, "artifacts", "agents", "waiters", "row.json"), string(mustJSON(t, map[string]any{"waitId": "row", "nonce": "n", "kind": "attempt", "targetId": "row", "runtime": "codex", "mode": "register", "state": "ready", "registeredAt": stamp, "result": rowResult})))
	writeFile(t, filepath.Join(root, "artifacts", "agents", "waiters", "replaced.json"), string(mustJSON(t, map[string]any{"waitId": "replaced", "nonce": "n", "kind": "attempt", "targetId": "replaced", "runtime": "codex", "mode": "register", "state": "ready", "registeredAt": stamp})))
	writeFile(t, filepath.Join(root, "artifacts", "agents", "waiters", "pending.json"), string(mustJSON(t, map[string]any{"waitId": "pending", "nonce": "n", "kind": "attempt", "targetId": "pending", "runtime": "codex", "mode": "register", "state": "pending", "registeredAt": stamp})))
	measurement, err := MeasureWaits(root, WaitMeasureOptions{})
	if err != nil || len(measurement.Runtimes) != 1 {
		t.Fatalf("measurement=%+v err=%v", measurement, err)
	}
	report = measurement.Runtimes[0]
	if report.Counts["pending-at-cut"] != 1 || report.Counts["not-sampled"] != 2 || report.Counts["unavailable"] != 1 || report.Counts["refuted"] != 1 || report.Counts["returned-within-60"] != 1 || len(report.Samples) != 2 || report.Samples[0].StampsFrom != "event" || report.Samples[1].StampsFrom != "row" || report.Unavailable[0].Reason != "return-unrecorded" {
		t.Fatalf("row/event accounting=%+v", report)
	}
	if rendered, encoded := FormatWaitMeasurement(measurement), string(mustJSON(t, measurement)); strings.Contains(strings.ToLower(rendered+encoded), "pass") {
		t.Fatalf("measurement vocabulary carried a pass: %s %s", rendered, encoded)
	}

	cutRoot := t.TempDir()
	cutResult := map[string]any{"waitId": "cut", "nonce": "n", "runtime": "codex", "mode": "register", "state": "ready", "registeredAt": stamp, "returnedAt": "2026-09-16T10:05:00Z", "selector": map[string]any{"kind": "attempt", "targetId": "cut"}}
	writeFile(t, filepath.Join(cutRoot, "artifacts", "agents", "waiters", "cut.json"), string(mustJSON(t, map[string]any{"waitId": "cut", "nonce": "n", "kind": "attempt", "targetId": "cut", "runtime": "codex", "mode": "register", "state": "ready", "registeredAt": stamp, "result": cutResult})))
	cut, err := MeasureWaits(cutRoot, WaitMeasureOptions{Until: time.Date(2026, 9, 16, 10, 2, 0, 0, time.UTC)})
	if err != nil || len(cut.Runtimes) != 1 || cut.Runtimes[0].Counts["pending-at-cut"] != 1 || cut.Runtimes[0].Counts["unavailable"] != 0 {
		t.Fatalf("cut accounting=%+v err=%v", cut, err)
	}

	malformedRoot := t.TempDir()
	malformed := &events.Emitter{Component: "run", Pid: 11, PidStartedAt: 21}
	bad := returnFields("malformed", "n", "register", "ready", second, 2*second, 3*second, 4*second)
	bad["returnedBootNanos"] = "not-a-number"
	if err := malformed.EmitChecked(malformedRoot, "wait-returned", "malformed wait fixture", bad); err != nil {
		t.Fatal(err)
	}
	if _, err := MeasureWaits(malformedRoot, WaitMeasureOptions{}); err == nil || !strings.Contains(err.Error(), "returnedBootNanos") {
		t.Fatalf("malformed wire value was not a named defect: %v", err)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("unparseable %s: %v", path, err)
	}
	return value
}

func TestDevinUsageDeltaAndUnavailable(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "t.json")
	usage := filepath.Join(dir, "usage.json")
	cumulative := filepath.Join(dir, "cum.json")
	writeFile(t, transcript, `{"final_metrics":{"total_prompt_tokens":25799,"total_completion_tokens":1200,"total_cached_tokens":900,"total_steps":40}}`)

	// First turn (no predecessor) records the totals as the delta.
	if err := DevinUsage(usage, transcript, "", cumulative, "", false); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, usage)
	if got["availability"] != "native" || got["inputTokens"] != float64(25799) {
		t.Fatalf("first turn should publish totals: %v", got)
	}
	if pu := got["providerUnits"].(map[string]any); pu["name"] != "devin-steps" || pu["value"] != float64(40) {
		t.Fatalf("unexpected provider units: %v", got["providerUnits"])
	}
	cum := readJSONFile(t, cumulative)
	if cum["total_prompt_tokens"] != float64(25799) {
		t.Fatalf("cumulative totals not recorded: %v", cum)
	}

	// A resumed turn subtracts its predecessor's cumulative totals.
	previous := filepath.Join(dir, "prev.json")
	writeFile(t, previous, `{"total_prompt_tokens":12833,"total_completion_tokens":700,"total_cached_tokens":400,"total_steps":22}`)
	if err := DevinUsage(usage, transcript, "", cumulative, previous, true); err != nil {
		t.Fatal(err)
	}
	got = readJSONFile(t, usage)
	if got["inputTokens"] != float64(25799-12833) || got["outputTokens"] != float64(1200-700) {
		t.Fatalf("resumed turn should publish the delta: %v", got)
	}

	// A resumed turn whose predecessor is missing is unavailable.
	if err := DevinUsage(usage, transcript, "", cumulative, "", true); err != nil {
		t.Fatal(err)
	}
	got = readJSONFile(t, usage)
	if got["availability"] != "unavailable" || got["inputTokens"] != nil {
		t.Fatalf("a missing predecessor should be unavailable: %v", got)
	}
}

func TestDevinUsageACU(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "t.json")
	usage := filepath.Join(dir, "usage.json")
	cumulative := filepath.Join(dir, "cum.json")
	// An enterprise account reports ACU and no token totals.
	writeFile(t, transcript, `{"final_metrics":{"total_acu_used": 12.5}}`)
	if err := DevinUsage(usage, transcript, "", cumulative, "", false); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, usage)
	if got["availability"] != "unavailable" {
		t.Fatalf("a tokenless account is unavailable for tokens: %v", got)
	}
	pu := got["providerUnits"].(map[string]any)
	if pu["name"] != "acu" || pu["value"] != float64(12.5) {
		t.Fatalf("ACU should ride in provider units: %v", got["providerUnits"])
	}
	cum := readJSONFile(t, cumulative)
	if cum["total_acu_used"] != float64(12.5) {
		t.Fatalf("the raw ACU key should be stored for the successor: %v", cum)
	}
}

func TestRootJobIDChain(t *testing.T) {
	jobs := t.TempDir()
	writeFile(t, filepath.Join(jobs, "a.json"), `{"parentJob": null}`)
	writeFile(t, filepath.Join(jobs, "b.json"), `{"parentJob": "a"}`)
	writeFile(t, filepath.Join(jobs, "c.json"), `{"parentJob": "b"}`)
	// A record with no parentJob key at all is its own root.
	writeFile(t, filepath.Join(jobs, "lone.json"), `{"round": 1}`)

	for job, want := range map[string]string{"c": "a", "b": "a", "a": "a", "lone": "lone"} {
		got, err := RootJobID(jobs, job)
		if err != nil {
			t.Fatalf("RootJobID(%q): %v", job, err)
		}
		if got != want {
			t.Fatalf("RootJobID(%q) = %q, want %q", job, got, want)
		}
	}
}

func TestRootJobIDCycle(t *testing.T) {
	jobs := t.TempDir()
	writeFile(t, filepath.Join(jobs, "x.json"), `{"parentJob": "y"}`)
	writeFile(t, filepath.Join(jobs, "y.json"), `{"parentJob": "x"}`)

	if _, err := RootJobID(jobs, "x"); err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("expected a cyclic-chain error, got %v", err)
	}
}

// The snapshot path: usage reads the attempt snapshot when given
// one, the snapshot survives export mutation, and oversize propagates.
func TestDevinUsageThroughSnapshot(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "t.json")
	if err := os.WriteFile(transcript,
		[]byte(`{"final_metrics":{"total_prompt_tokens":10,"total_cached_tokens":4,"total_completion_tokens":3,"total_steps":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	usage := filepath.Join(dir, "usage.json")
	cumulative := filepath.Join(dir, "cumulative.json")
	snapshot := filepath.Join(dir, "snap.json")
	if err := DevinUsage(usage, transcript, snapshot, cumulative, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(snapshot); err != nil {
		t.Fatal("the snapshot must be materialized")
	}
	// Mutating the export does not change a re-read through the snapshot.
	if err := os.WriteFile(transcript, []byte(`{"final_metrics":{"total_prompt_tokens":999}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	usage2 := filepath.Join(dir, "usage2.json")
	cumulative2 := filepath.Join(dir, "cumulative2.json")
	if err := DevinUsage(usage2, transcript, snapshot, cumulative2, "", false); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(cumulative)
	second, _ := os.ReadFile(cumulative2)
	if string(first) != string(second) {
		t.Fatalf("snapshot-fed usage must be immutable across export mutation:\n%s\n%s", first, second)
	}
}
