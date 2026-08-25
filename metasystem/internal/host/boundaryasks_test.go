package host

import (
	"bytes"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"os"
	"path/filepath"
	"testing"
)

// Proof 2: boundary-ask candidates project verbatim with ordinal
// seqs; absence yields nothing; malformation errors, never invents.
func TestHostBoundaryAskEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reply-accepted.json")
	doc := `{"turnId":"t1","askCandidates":[` +
		`{"streamId":"build","reasonClass":"decision","question":"which db?"},` +
		`{"streamId":"build","reasonClass":"blocked","question":"creds?","supersedes":"a-1"}]}`
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	events, err := HostBoundaryAskEvents("claude")(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("candidates: %d", len(events))
	}
	for i, ev := range events {
		if ev.Kind != "claude/boundary-ask-candidate" || ev.Seq != uint64(i+1) {
			t.Fatalf("event %d: %+v", i, ev)
		}
	}
	if !bytes.Contains(events[1].Params, []byte(`"supersedes":"a-1"`)) {
		t.Fatalf("candidate not verbatim: %s", events[1].Params)
	}

	// No candidates → nothing; missing file → nothing.
	if err := os.WriteFile(path, []byte(`{"turnId":"t1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if events, err := HostBoundaryAskEvents("devin")(path); err != nil || len(events) != 0 {
		t.Fatalf("candidate-less return: %v %v", events, err)
	}
	if events, err := HostBoundaryAskEvents("devin")(filepath.Join(dir, "absent.json")); err != nil || events != nil {
		t.Fatalf("missing return: %v %v", events, err)
	}

	// Live registrations serve the same projection (S3-C1-003).
	for _, runtime := range []string{"claude", "codex", "devin", "fake"} {
		ports, err := delegate.PortsFor(runtime)
		if err != nil || ports.HostBoundaryAskEvents == nil {
			t.Fatalf("%s HostBoundaryAskEvents unregistered: %v", runtime, err)
		}
	}

	// Malformed → error, never invention.
	if err := os.WriteFile(path, []byte(`{torn`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := HostBoundaryAskEvents("devin")(path); err == nil {
		t.Fatal("malformed return did not error")
	}
}
