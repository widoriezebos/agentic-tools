package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("%s is not a JSON object: %v", path, err)
	}
	return value
}

// --- root job walk ---

// --- effective permissions handshake ---

const requestedRecord = `{
  "permissions": {
    "requested": {
      "readRoots": ["/a", "/b"],
      "writeRoots": ["/w"],
      "network": "deny",
      "approvals": "ask",
      "tools": "read-only"
    }
  }
}`

func TestMaterializeAndCompareAccept(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	effective := filepath.Join(dir, "effective.json")
	writeFile(t, record, requestedRecord)

	if err := MaterializeEffective(record, effective); err != nil {
		t.Fatal(err)
	}
	// The materialized file is exactly the requested envelope.
	got := readJSONFile(t, effective)
	if got["network"] != "deny" || got["tools"] != "read-only" {
		t.Fatalf("materialized effective lost fields: %v", got)
	}

	mismatch, err := ComparePermissions(record, effective)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch != "" {
		t.Fatalf("an exact copy of the request must not be wider, got %q", mismatch)
	}
}

func TestRewriteWriteScopePinsWorkspace(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	effective := filepath.Join(dir, "effective.json")
	workspace := filepath.Join(dir, "repo")
	writeFile(t, record, requestedRecord)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MaterializeEffective(record, effective); err != nil {
		t.Fatal(err)
	}
	if err := RewriteWriteScope(effective, workspace); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, effective)
	roots, ok := got["writeRoots"].([]any)
	if !ok || len(roots) != 1 || roots[0] != resolve(workspace) {
		t.Fatalf("writeRoots was not pinned to the resolved workspace: %v", got["writeRoots"])
	}
	// A narrower request pinned to the wider real boundary now reads as wider.
	mismatch, err := ComparePermissions(record, effective)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch != "writeRoots" {
		t.Fatalf("pinned workspace should widen writeRoots, got %q", mismatch)
	}
}

func TestComparePermissionsRefuse(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	effective := filepath.Join(dir, "effective.json")
	writeFile(t, record, requestedRecord)
	// Wider on both root sets, on network (allow > deny) and tools
	// (runtime-default > read-only); approvals is unchanged and must not appear.
	writeFile(t, effective, `{
      "readRoots": ["/a", "/c"],
      "writeRoots": ["/w", "/x"],
      "network": "allow",
      "approvals": "ask",
      "tools": "runtime-default"
    }`)
	mismatch, err := ComparePermissions(record, effective)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch != "readRoots,writeRoots,network,tools" {
		t.Fatalf("unexpected widening set %q", mismatch)
	}
}

func TestComparePermissionsAbsentOrdinalIgnored(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	effective := filepath.Join(dir, "effective.json")
	writeFile(t, record, requestedRecord)
	// An effective file that omits an ordinal field entirely asserts nothing
	// about it, so it never widens.
	writeFile(t, effective, `{"readRoots": ["/a"], "writeRoots": ["/w"]}`)
	mismatch, err := ComparePermissions(record, effective)
	if err != nil {
		t.Fatal(err)
	}
	if mismatch != "" {
		t.Fatalf("absent ordinals must not widen, got %q", mismatch)
	}
}

// --- patch writers ---

func TestWriteModelPatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model.json")
	if err := WriteModelPatch(path, "claude-opus-4-8"); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, path)
	if got["effectiveModel"] != "claude-opus-4-8" || len(got) != 1 {
		t.Fatalf("unexpected model patch %v", got)
	}
	if err := WriteModelPatch(path, ""); err == nil {
		t.Fatal("an empty model must be refused")
	}
}

func TestWriteRepairsPatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repairs.json")
	if err := WriteRepairsPatch(path, 1); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, path)
	if got["returnRepairs"] != float64(1) || len(got) != 1 {
		t.Fatalf("unexpected repairs patch %v", got)
	}
}

func TestWriteResultPatch(t *testing.T) {
	dir := t.TempDir()

	// Success carries a null error and, with no usage file, a null usage.
	success := filepath.Join(dir, "success.json")
	if err := WriteResultPatch(success, "null", "completed", ""); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, success)
	if got["error"] != nil || got["phase"] != "completed" || got["usage"] != nil {
		t.Fatalf("unexpected success patch %v", got)
	}
	if _, present := got["usage"]; !present {
		t.Fatal("usage must be present as an explicit null")
	}

	// A failure keeps its code and embeds the usage file's JSON verbatim.
	usage := filepath.Join(dir, "usage.json")
	writeFile(t, usage, `{"availability": "native", "inputTokens": 12}`)
	failure := filepath.Join(dir, "failure.json")
	if err := WriteResultPatch(failure, "runtime_error", "runtime", usage); err != nil {
		t.Fatal(err)
	}
	got = readJSONFile(t, failure)
	if got["error"] != "runtime_error" || got["phase"] != "runtime" {
		t.Fatalf("unexpected failure patch %v", got)
	}
	embedded, ok := got["usage"].(map[string]any)
	if !ok || embedded["availability"] != "native" {
		t.Fatalf("usage was not embedded: %v", got["usage"])
	}

	// A usage path that is not a regular file records a null usage.
	missing := filepath.Join(dir, "missing.json")
	if err := WriteResultPatch(missing, "null", "handshake", filepath.Join(dir, "nope.json")); err != nil {
		t.Fatal(err)
	}
	if readJSONFile(t, missing)["usage"] != nil {
		t.Fatal("a missing usage file must record a null usage")
	}
}

// --- capability snapshot writer ---

const (
	validEnvelope = `{"writeRoots": "mapped", "readRoots": "notEnforced", "network": "mapped"}`
	validHashes   = `{"model.default": "` +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" + `"}`
)

func TestWriteCapabilitySnapshot(t *testing.T) {
	restore := now
	now = func() time.Time { return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC) }
	defer func() { now = restore }()

	dir := t.TempDir()
	path, err := WriteCapabilitySnapshot(dir, "claude", "1.2.3", "abcd",
		`["file", "stdout"]`, `{"resume": true}`, `{"unverified": []}`, validEnvelope, validHashes)
	if err != nil {
		t.Fatal(err)
	}
	if base := filepath.Base(path); base != "claude-1.2.3-abcd-20260810-001.json" {
		t.Fatalf("unexpected snapshot name %q", base)
	}
	got := readJSONFile(t, path)
	if got["runtime"] != "claude" || got["cliVersion"] != "1.2.3" || got["configHash"] != "abcd" {
		t.Fatalf("snapshot identity fields wrong: %v", got)
	}
	if got["capturedAt"] != "2026-08-10T12:00:00Z" || got["sequence"] != float64(1) {
		t.Fatalf("snapshot capture fields wrong: %v", got)
	}
	if _, ok := got["envelopeEnforcement"].(map[string]any); !ok {
		t.Fatalf("envelope enforcement missing: %v", got)
	}

	// A second capture the same day takes the next sequence.
	next, err := WriteCapabilitySnapshot(dir, "claude", "1.2.3", "abcd",
		`["file"]`, `{"resume": true}`, `{}`, validEnvelope, validHashes)
	if err != nil {
		t.Fatal(err)
	}
	if base := filepath.Base(next); base != "claude-1.2.3-abcd-20260810-002.json" {
		t.Fatalf("second snapshot did not advance the sequence: %q", base)
	}
}

func TestWriteCapabilitySnapshotRejects(t *testing.T) {
	dir := t.TempDir()

	// Envelope missing the network field.
	_, err := WriteCapabilitySnapshot(dir, "claude", "1", "h",
		`[]`, `{}`, `{}`, `{"writeRoots": "mapped", "readRoots": "mapped"}`, validHashes)
	if err == nil || !strings.Contains(err.Error(), "envelope enforcement") {
		t.Fatalf("expected an envelope-enforcement refusal, got %v", err)
	}

	// Envelope with an unknown value.
	_, err = WriteCapabilitySnapshot(dir, "claude", "1", "h",
		`[]`, `{}`, `{}`, `{"writeRoots": "mapped", "readRoots": "mapped", "network": "maybe"}`, validHashes)
	if err == nil || !strings.Contains(err.Error(), "envelope enforcement") {
		t.Fatalf("expected an envelope-enforcement refusal, got %v", err)
	}

	// A key hash that is not a SHA-256 digest.
	_, err = WriteCapabilitySnapshot(dir, "claude", "1", "h",
		`[]`, `{}`, `{}`, validEnvelope, `{"model.default": "short"}`)
	if err == nil || !strings.Contains(err.Error(), "configuration key hashes") {
		t.Fatalf("expected a key-hash refusal, got %v", err)
	}
}
