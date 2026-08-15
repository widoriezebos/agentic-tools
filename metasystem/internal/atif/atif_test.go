package atif

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const sample = `{"session_id":"s-1","agent":{"model_name":"m"},"steps":[
 {"step_id":1,"tool_calls":[{"tool_call_id":"t1","function_name":"exec","arguments":{"command":"ls"}}]},
 {"step_id":2,"tool_calls":[{"tool_call_id":"t2","function_name":"write","arguments":{"file_path":"/x","content":"{}"}}]}
]}`

func TestReadBoundedDecodesStepsAndKeepsRaw(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "t.json", sample)
	transcript, err := ReadBounded(path)
	if err != nil {
		t.Fatal(err)
	}
	if transcript.SessionID != "s-1" || len(transcript.Steps) != 2 {
		t.Fatalf("decoded shape wrong: %+v", transcript)
	}
	if transcript.Steps[1].ToolCalls[0].FunctionName != "write" {
		t.Fatalf("tool call lost: %+v", transcript.Steps[1])
	}
	if string(transcript.Raw()) != sample {
		t.Fatal("raw bytes must be the exact file content")
	}
}

func TestReadBoundedRefusesOversizeWithNoPartialContent(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "big.json", `{"steps":[`+strings.Repeat(" ", MaxTranscriptBytes)+`]}`)
	if _, err := ReadBounded(path); !errors.Is(err, ErrOversize) {
		t.Fatalf("oversize must return ErrOversize, got %v", err)
	}
}

func TestReadBoundedDistinguishesUnparseable(t *testing.T) {
	dir := t.TempDir()
	path := write(t, dir, "torn.json", `{"steps":[{`)
	_, err := ReadBounded(path)
	if err == nil || errors.Is(err, ErrOversize) {
		t.Fatalf("torn transcript needs its own error, got %v", err)
	}
}

func TestSnapshotIsImmutableAcrossExportMutation(t *testing.T) {
	dir := t.TempDir()
	export := write(t, dir, "export.json", sample)
	snapshot := filepath.Join(dir, "snap.json")
	first, err := Snapshot(export, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	// The live export changes; every later consumer still sees the snapshot.
	if err := os.WriteFile(export, []byte(`{"session_id":"tampered","steps":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := Snapshot(export, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if second.SessionID != first.SessionID || string(second.Raw()) != string(first.Raw()) {
		t.Fatal("snapshot must be immutable once materialized")
	}
}

func TestSnapshotOversizePropagates(t *testing.T) {
	dir := t.TempDir()
	export := write(t, dir, "big.json", `{"steps":[`+strings.Repeat(" ", MaxTranscriptBytes)+`]}`)
	if _, err := Snapshot(export, filepath.Join(dir, "snap.json")); !errors.Is(err, ErrOversize) {
		t.Fatalf("oversize export must refuse snapshotting, got %v", err)
	}
}

func TestReadBoundedObjectAndSnapshotObject(t *testing.T) {
	dir := t.TempDir()
	export := write(t, dir, "export.json", sample)
	object, err := ReadBoundedObject(export)
	if err != nil || object["session_id"] != "s-1" {
		t.Fatalf("bounded object read = %v %v", object, err)
	}
	if _, err := ReadBoundedObject(write(t, dir, "arr.json", `[1,2]`)); err == nil {
		t.Fatal("a non-object transcript must refuse the object read")
	}
	if _, err := ReadBoundedObject(filepath.Join(dir, "absent.json")); err == nil {
		t.Fatal("an absent file must error")
	}

	snapshot := filepath.Join(dir, "snap.json")
	object, err = SnapshotObject(export, snapshot)
	if err != nil || object["session_id"] != "s-1" {
		t.Fatalf("snapshot object = %v %v", object, err)
	}
	// Export mutation after materialization changes nothing.
	if err := os.WriteFile(export, []byte(`{"session_id":"tampered"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	object, err = SnapshotObject(export, snapshot)
	if err != nil || object["session_id"] != "s-1" {
		t.Fatalf("snapshot must stay immutable: %v %v", object, err)
	}
}

func TestSnapshotUnparseableExportErrors(t *testing.T) {
	dir := t.TempDir()
	export := write(t, dir, "torn.json", `{"steps":[{`)
	if _, err := Snapshot(export, filepath.Join(dir, "snap.json")); err == nil {
		t.Fatal("a torn export must not snapshot")
	}
}
