package supervisor

// The observed-model wiring, ported from the retired
// scripts/agents/telemetry-census-fixtures.sh one-key case: a completed
// Claude result's modelUsage names the model that actually ran, the
// supervisor records it on the job record through the record CAS, and the
// normalized return carries it as the effective model.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
)

// patchingDispatcher answers __record-cas the way the dispatcher's CAS does
// for a matching expectation: the patch's fields merge into the record. It
// records every call; any other callback is answered 0.
type patchingDispatcher struct {
	t      *testing.T
	record string
	calls  [][]string
}

func (d *patchingDispatcher) Run(_, _ io.Writer, args ...string) int {
	d.calls = append(d.calls, append([]string(nil), args...))
	if len(args) == 0 || args[0] != "__record-cas" {
		return 0
	}
	flags := map[string]string{}
	for i := 1; i+1 < len(args); i += 2 {
		flags[args[i]] = args[i+1]
	}
	var record, patch map[string]any
	readJSONInto(d.t, d.record, &record)
	readJSONInto(d.t, flags["--patch"], &patch)
	if record["status"] != flags["--expect"] {
		return 3
	}
	for key, value := range patch {
		record[key] = value
	}
	record["status"] = flags["--status"]
	data, err := json.Marshal(record)
	if err != nil {
		d.t.Fatal(err)
	}
	if err := os.WriteFile(d.record, data, 0o644); err != nil {
		d.t.Fatal(err)
	}
	return 0
}

func readJSONInto(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func TestObservedResultModelIsRecordedAndNormalized(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "fixture-one-key.json")
	if err := os.WriteFile(record, []byte(`{"jobId":"fixture-one-key","round":1,"runtime":"claude","sessionId":"fixture-session","requestedModel":"requested-model","effectiveModel":"provisional-handshake-model","status":"running"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(dir, "result.json")
	if err := os.WriteFile(result, []byte(`{"type":"result","session_id":"fixture-session","modelUsage":{"actual-model": {}},"structured_output":{"jobId":"fixture-one-key","round":1,"runtime":"claude","sessionId":"fixture-session","model":{"requested":"requested-model","effective":"agent-claimed-model"},"evidence":[],"gaps":[],"mode":"implement","riskiestPart":"fixture","diffBoundary":[],"whatWasDone":"fixture"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dispatch := &patchingDispatcher{t: t, record: record}
	s := &Supervision{
		d:   Deps{Stdout: io.Discard, Stderr: io.Discard, Dispatch: dispatch, Clock: &deadlineClock{}},
		job: "fixture-one-key", record: record, roundDir: dir,
		handshakeDone: true, sessionID: "fixture-session",
		requestedModel: "requested-model", effectiveModel: "provisional-handshake-model",
	}

	observed, present, err := adapter.ClaudeResultField(result, "model")
	if err != nil || !present || observed != "actual-model" {
		t.Fatalf("one-key modelUsage produced %q (present=%v, err=%v) instead of actual-model", observed, present, err)
	}
	session, _, _ := adapter.ClaudeResultField(result, "session_id")
	if !s.settleResultIdentity(session, "", observed, observed, "") {
		t.Fatalf("settling the result identity failed: %q", dispatch.calls)
	}
	if len(dispatch.calls) != 1 || dispatch.calls[0][0] != "__record-cas" ||
		dispatch.calls[0][4] != "running" || dispatch.calls[0][6] != "running" {
		t.Fatalf("expected one running->running model CAS: %q", dispatch.calls)
	}
	var updated map[string]any
	readJSONInto(t, record, &updated)
	if updated["effectiveModel"] != "actual-model" || s.effectiveModel != "actual-model" {
		t.Fatalf("record did not adopt the observed effective model: %v", updated)
	}

	returnPath := filepath.Join(dir, "return.json")
	if err := adapter.NormalizeReturn(result, "", record, returnPath, filepath.Join(dir, "return.md"), s.sessionID); err != nil {
		t.Fatal(err)
	}
	var normalized map[string]any
	readJSONInto(t, returnPath, &normalized)
	model, _ := normalized["model"].(map[string]any)
	if model["requested"] != "requested-model" || model["effective"] != "actual-model" {
		t.Fatalf("normalized return did not carry the observed model: %v", normalized["model"])
	}
}
