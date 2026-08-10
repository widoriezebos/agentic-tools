package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readObject(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return value
}

func TestResultWriteShapeAndNulls(t *testing.T) {
	dir := t.TempDir()
	usage := filepath.Join(dir, "usage.json")
	write(t, usage, `{"availability":"native","inputTokens":5}`)
	result := filepath.Join(dir, "result.json")
	if err := ResultWrite(result, "", "unresumable", usage, "/raw", ""); err != nil {
		t.Fatal(err)
	}
	got := readObject(t, result)
	keys := make([]string, 0, len(got))
	for key := range got {
		keys = append(keys, key)
	}
	want := map[string]bool{"sessionId": true, "outcome": true, "usage": true, "rawPath": true, "returnPath": true}
	if len(keys) != len(want) {
		t.Fatalf("unexpected keys %v", keys)
	}
	for _, key := range keys {
		if !want[key] {
			t.Fatalf("unexpected key %q", key)
		}
	}
	if got["sessionId"] != nil {
		t.Fatalf("empty session should be null, got %v", got["sessionId"])
	}
	if got["returnPath"] != nil {
		t.Fatalf("empty return path should be null, got %v", got["returnPath"])
	}
	usageObject, ok := got["usage"].(map[string]any)
	if !ok || usageObject["availability"] != "native" {
		t.Fatalf("usage not embedded: %v", got["usage"])
	}
}

func TestResultWriteUnavailableUsageWhenMissing(t *testing.T) {
	dir := t.TempDir()
	result := filepath.Join(dir, "result.json")
	if err := ResultWrite(result, "sess-1", "failed", filepath.Join(dir, "absent.json"), "/raw", ""); err != nil {
		t.Fatal(err)
	}
	got := readObject(t, result)
	usage, ok := got["usage"].(map[string]any)
	if !ok || usage["availability"] != "unavailable" {
		t.Fatalf("missing usage should be unavailable, got %v", got["usage"])
	}
	if got["sessionId"] != "sess-1" {
		t.Fatalf("session not recorded: %v", got["sessionId"])
	}
}

func TestClaudeResultStructuredOutput(t *testing.T) {
	dir := t.TempDir()
	provider := filepath.Join(dir, "claude.json")
	write(t, provider, `{
		"structured_output": {"gaps": [], "mode": "design"},
		"usage": {"input_tokens": 10, "cache_read_input_tokens": 3, "output_tokens": 7, "reasoning_tokens": 2},
		"total_cost_usd": 0.25,
		"session_id": "abc"
	}`)
	returnPath := filepath.Join(dir, "return.json")
	usagePath := filepath.Join(dir, "usage.json")
	if err := ClaudeResult(provider, returnPath, usagePath); err != nil {
		t.Fatal(err)
	}
	ret := readObject(t, returnPath)
	if ret["mode"] != "design" {
		t.Fatalf("return not extracted: %v", ret)
	}
	usage := readObject(t, usagePath)
	if usage["availability"] != "native" || usage["inputTokens"].(float64) != 10 || usage["cachedInputTokens"].(float64) != 3 {
		t.Fatalf("usage wrong: %v", usage)
	}
	cost, ok := usage["cost"].(map[string]any)
	if !ok || cost["currency"] != "USD" || cost["amount"].(float64) != 0.25 {
		t.Fatalf("cost wrong: %v", usage["cost"])
	}
}

func TestClaudeResultFromResultString(t *testing.T) {
	dir := t.TempDir()
	provider := filepath.Join(dir, "claude.json")
	write(t, provider, `{"result": "{\"gaps\": [], \"mode\": \"code\"}", "usage": {}}`)
	returnPath := filepath.Join(dir, "return.json")
	usagePath := filepath.Join(dir, "usage.json")
	if err := ClaudeResult(provider, returnPath, usagePath); err != nil {
		t.Fatal(err)
	}
	ret := readObject(t, returnPath)
	if ret["mode"] != "code" {
		t.Fatalf("return not parsed from result string: %v", ret)
	}
	usage := readObject(t, usagePath)
	if usage["cost"] != nil || usage["inputTokens"] != nil {
		t.Fatalf("absent counts should be null: %v", usage)
	}
}

func TestClaudeResultMissingProviderWritesUsageOnly(t *testing.T) {
	dir := t.TempDir()
	returnPath := filepath.Join(dir, "return.json")
	usagePath := filepath.Join(dir, "usage.json")
	if err := ClaudeResult(filepath.Join(dir, "absent.json"), returnPath, usagePath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(returnPath); !os.IsNotExist(err) {
		t.Fatalf("return should be absent when no object found")
	}
	usage := readObject(t, usagePath)
	if usage["availability"] != "native" {
		t.Fatalf("usage should still land: %v", usage)
	}
}

func TestDevinReturnExtractsFromNoise(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.out")
	write(t, raw, "welcome banner\n{\"gaps\": [], \"mode\": \"design\"}\ngoodbye")
	out := filepath.Join(dir, "return.json")
	if err := DevinReturn(raw, out); err != nil {
		t.Fatal(err)
	}
	got := readObject(t, out)
	if got["mode"] != "design" {
		t.Fatalf("return not extracted: %v", got)
	}
}

func TestDevinReturnNoObjectLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.out")
	write(t, raw, "no json here")
	out := filepath.Join(dir, "return.json")
	if err := DevinReturn(raw, out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("no object should leave the return absent")
	}
}

func TestDevinConfig(t *testing.T) {
	dir := t.TempDir()
	configHome := filepath.Join(dir, "xdg")
	write(t, filepath.Join(configHome, "devin", "config.json"),
		`{"organizationId": "org-1", "onboardingComplete": true, "sandbox": {"mode": "strict"}}`)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	root := filepath.Join(dir, "checkout")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "devin-config.json")
	if err := DevinConfig(root, out); err != nil {
		t.Fatal(err)
	}
	got := readObject(t, out)
	if got["organizationId"] != "org-1" || got["onboardingComplete"] != true {
		t.Fatalf("user config not preserved: %v", got)
	}
	if _, ok := got["sandbox"]; ok {
		t.Fatalf("sandbox should be dropped: %v", got)
	}
	perms, ok := got["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("permissions missing: %v", got)
	}
	if !reflect.DeepEqual(perms["deny"], []any{"mcp__*"}) {
		t.Fatalf("deny wrong: %v", perms["deny"])
	}
	allow, ok := perms["allow"].([]any)
	if !ok || len(allow) != 7 {
		t.Fatalf("allow wrong: %v", perms["allow"])
	}
}

func TestDevinUsageDelta(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "transcript.json")
	write(t, transcript, `{"final_metrics": {"total_prompt_tokens": 100, "total_completion_tokens": 40, "total_cached_tokens": 10, "total_steps": 6}}`)
	previous := filepath.Join(dir, "previous.json")
	write(t, previous, `{"total_prompt_tokens": 60, "total_completion_tokens": 25, "total_cached_tokens": 4, "total_steps": 4}`)
	usage := filepath.Join(dir, "usage.json")
	cumulative := filepath.Join(dir, "cumulative.json")
	if err := DevinUsage(usage, transcript, cumulative, previous, true); err != nil {
		t.Fatal(err)
	}
	got := readObject(t, usage)
	if got["availability"] != "native" || got["inputTokens"].(float64) != 40 || got["outputTokens"].(float64) != 15 || got["cachedInputTokens"].(float64) != 6 {
		t.Fatalf("delta wrong: %v", got)
	}
	units, ok := got["providerUnits"].(map[string]any)
	if !ok || units["name"] != "devin-steps" || units["value"].(float64) != 2 {
		t.Fatalf("steps delta wrong: %v", got["providerUnits"])
	}
	cum := readObject(t, cumulative)
	if cum["total_steps"].(float64) != 6 {
		t.Fatalf("cumulative should hold this turn's totals: %v", cum)
	}
}

func TestDevinUsagePredecessorMissing(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "transcript.json")
	write(t, transcript, `{"final_metrics": {"total_prompt_tokens": 100, "total_completion_tokens": 40, "total_cached_tokens": 10, "total_steps": 6}}`)
	usage := filepath.Join(dir, "usage.json")
	cumulative := filepath.Join(dir, "cumulative.json")
	// expectPrevious with no predecessor file: publishing session totals would
	// double-count, so usage is unavailable.
	if err := DevinUsage(usage, transcript, cumulative, "", true); err != nil {
		t.Fatal(err)
	}
	got := readObject(t, usage)
	if got["availability"] != "unavailable" {
		t.Fatalf("missing predecessor should be unavailable: %v", got)
	}
}

func TestDevinUsageACUOnly(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "transcript.json")
	write(t, transcript, `{"final_metrics": {"total_acu": 12.5}}`)
	usage := filepath.Join(dir, "usage.json")
	cumulative := filepath.Join(dir, "cumulative.json")
	if err := DevinUsage(usage, transcript, cumulative, "", false); err != nil {
		t.Fatal(err)
	}
	got := readObject(t, usage)
	if got["availability"] != "unavailable" {
		t.Fatalf("acu-only is unavailable tokens: %v", got)
	}
	units, ok := got["providerUnits"].(map[string]any)
	if !ok || units["name"] != "acu" || units["value"].(float64) != 12.5 {
		t.Fatalf("acu not metered: %v", got["providerUnits"])
	}
	cum := readObject(t, cumulative)
	if cum["total_acu"].(float64) != 12.5 {
		t.Fatalf("acu cumulative not written: %v", cum)
	}
}

func TestFakeReturnDispatchTerminalWritesJobRecord(t *testing.T) {
	dir := t.TempDir()
	turn := filepath.Join(dir, "turn.json")
	write(t, turn, `{"turnId": "t-1", "missionId": "m-1", "cycle": 2, "model": "fable", "startedAt": "2026-08-10T00:00:00Z", "hostSession": null}`)
	state := filepath.Join(dir, "state.json")
	write(t, state, `{"streams": {"beta": {"state": "parked"}, "alpha": {"state": "active"}}}`)
	out := filepath.Join(dir, "return.json")
	if err := FakeReturn(turn, state, out, "dispatch-terminal", dir); err != nil {
		t.Fatal(err)
	}
	got := readObject(t, out)
	dispatched, ok := got["dispatched"].([]any)
	if !ok || len(dispatched) != 1 {
		t.Fatalf("dispatched wrong: %v", got["dispatched"])
	}
	job := dispatched[0].(map[string]any)
	if job["jobId"] != "verifier-m-1" || job["stream"] != "alpha" {
		t.Fatalf("dispatch entry wrong: %v", job)
	}
	record := readObject(t, filepath.Join(dir, "artifacts", "agents", "jobs", "verifier-m-1.json"))
	if record["status"] != "completed" || record["round"].(float64) != 1 || record["endedAt"] != "2026-08-10T00:00:00Z" {
		t.Fatalf("job record wrong: %v", record)
	}
	identity := got["identity"].(map[string]any)
	if identity["runtime"] != "fake" || identity["sessionId"] != nil {
		t.Fatalf("identity wrong: %v", identity)
	}
}

func TestFakeReturnActiveStreamFallback(t *testing.T) {
	dir := t.TempDir()
	turn := filepath.Join(dir, "turn.json")
	write(t, turn, `{"turnId": "t-1", "missionId": "m-1", "cycle": 1, "model": "fable"}`)
	state := filepath.Join(dir, "state.json")
	// No active stream: fall back to the first by name.
	write(t, state, `{"streams": {"gamma": {"state": "done"}, "beta": {"state": "parked"}}}`)
	out := filepath.Join(dir, "return.json")
	if err := FakeReturn(turn, state, out, "close-stream", dir); err != nil {
		t.Fatal(err)
	}
	got := readObject(t, out)
	updates := got["streamUpdatesRequested"].([]any)
	if len(updates) != 1 || updates[0].(map[string]any)["streamId"] != "beta" {
		t.Fatalf("fallback stream wrong: %v", updates)
	}
}

func TestFakeResultOutcomes(t *testing.T) {
	dir := t.TempDir()
	completed := filepath.Join(dir, "completed.json")
	if err := FakeResult(completed, "sess", "/raw", "/return", "completed"); err != nil {
		t.Fatal(err)
	}
	got := readObject(t, completed)
	if got["outcome"] != "completed" || got["returnPath"] != "/return" {
		t.Fatalf("completed envelope wrong: %v", got)
	}
	usage := got["usage"].(map[string]any)
	if usage["inputTokens"].(float64) != 11 {
		t.Fatalf("completed usage wrong: %v", usage)
	}
	failed := filepath.Join(dir, "failed.json")
	if err := FakeResult(failed, "sess", "/raw", "", "failed"); err != nil {
		t.Fatal(err)
	}
	got = readObject(t, failed)
	if got["outcome"] != "failed" || got["returnPath"] != nil {
		t.Fatalf("failed envelope wrong: %v", got)
	}
	if err := FakeResult(filepath.Join(dir, "bad.json"), "sess", "/raw", "", "weird"); err == nil {
		t.Fatalf("unknown outcome should error")
	}
}

func TestCompact(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "schema.json")
	write(t, file, "{\n  \"a\": 1,\n  \"b\": [1, 2]\n}\n")
	compact, err := Compact(file)
	if err != nil {
		t.Fatal(err)
	}
	if compact != `{"a":1,"b":[1,2]}` {
		t.Fatalf("compact wrong: %q", compact)
	}
}
