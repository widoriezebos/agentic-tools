package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestComponentPathsAndHeartbeatWriteFailure(t *testing.T) {
	root := t.TempDir()
	if got, want := JobsDir(root), filepath.Join(root, "artifacts", "agents", "jobs"); got != want {
		t.Fatalf("jobs directory = %q, want %q", got, want)
	}
	blocked := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocked, "watcher.heartbeat.json")
	if err := WriteHeartbeat(path, "watcher", identity.Ref{Pid: 41, StartedAtSec: 100}, "watcher-tag", 5, 60); err == nil {
		t.Fatal("heartbeat publication through a file succeeded")
	}
}

func TestLedgerEncoderAndDefaultClockProduceARegistryRecord(t *testing.T) {
	var captured map[string]any
	ledger := &RegistryLedger{
		CheckoutPath: "/repo", OwnerTag: "owner-tag",
		Append: func(record map[string]any) error { captured = record; return nil },
	}
	if err := ledger.AppendLaunched(Held{Component: Watcher, Tag: "watcher-tag", Generation: 3, Identity: identity.Ref{Pid: 41, StartedAtSec: 100}}); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeRecord(captured)
	if err != nil || !json.Valid(encoded) || captured["at"] == "" {
		t.Fatalf("ledger record was not encodable and timestamped: record=%+v err=%v", captured, err)
	}
}
