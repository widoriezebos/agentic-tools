package steward_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func writeU6bPortFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Port of dispatch-fixtures.sh steward-continuation (lines 5807-5836): a
// launched continuation that ended completed with the fake runtime's
// steward-continuation return is reaped by the tick as "ended completed
// with a valid return", the chain is closed, and the record stays
// completed. The return comes from the fake runtime's own writer and the
// validity from the standing return checker, as in the end-to-end leg.
func TestU6bPortReapCertifiesTheFakeContinuationsValidReturnAndClosesTheChain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	schema, err := os.ReadFile(filepath.Join("..", "..", "internal", "protocol", "schemas", "steward-continuation.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeU6bPortFile(t, filepath.Join(root, "internal", "protocol", "schemas", "steward-continuation.schema.json"), string(schema))

	intent := steward.Intent{
		Nonce: "n-valid", RepoIdentity: "repo", InstallGen: 1, Goal: "fix-it",
		RoleDigest: "r", BriefDigest: "b", PermsDigest: "p",
		Runtime: "fake", Model: "fake-model", JobId: "steward-n-valid", MintedAtTick: 7,
		Notified: true, LaunchStamped: true,
	}
	encoded, _ := json.Marshal(intent)
	writeU6bPortFile(t, filepath.Join(root, "artifacts", "agents", "steward", "consumed", intent.Nonce+".json"), string(encoded))

	recordPath := filepath.Join(root, "artifacts", "agents", "jobs", intent.JobId+".json")
	writeU6bPortFile(t, recordPath, `{"jobId":"steward-n-valid","role":"steward-continuation","round":1,"parentJob":null,`+
		`"runtime":"fake","sessionId":"fake-session-steward-n-valid","requestedModel":"fake-model","effectiveModel":"fake-model",`+
		`"status":"completed","endedAt":"2026-09-27T10:00:00Z"}`)
	prompt := filepath.Join(root, "artifacts", "agents", intent.JobId, "rounds", "1", "prompt.md")
	writeU6bPortFile(t, prompt, "Working Mode: implement\n\nRepair the thing.\n")
	returnPath := filepath.Join(filepath.Dir(prompt), "return.json")
	if err := adapter.WriteFakeReturn(recordPath, prompt, returnPath); err != nil {
		t.Fatal(err)
	}

	reports, err := steward.ReapContinuations(root)
	if err != nil || len(reports) != 1 || reports[0].JobId != intent.JobId || !strings.Contains(reports[0].Outcome, "ended completed with a valid return") {
		t.Fatalf("the tick must certify the fake continuation's valid return: %+v %v", reports, err)
	}
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record["chainClosed"] != true || record["status"] != "completed" {
		t.Fatalf("the reaped continuation must be closed and completed: %v", record)
	}
}
