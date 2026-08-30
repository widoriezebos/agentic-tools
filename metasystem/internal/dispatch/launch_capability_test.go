package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestLaunchCapabilityIsBoundAndSingleUse(t *testing.T) {
	root := t.TempDir()
	job := "cap-job"
	raw := "one-shot-launch-word"
	digest := sha256.Sum256([]byte(raw))
	tag := "metasystem-job-cap-job-nonce"
	record := map[string]any{
		"jobId": job, "operationId": "operation-a", "status": "pending", "instanceTag": tag,
		"launchCapability": map[string]any{
			"digest": hex.EncodeToString(digest[:]), "jobId": job, "operationId": "operation-a",
			"instanceTag": tag, "adapterVerb": "dispatch", "status": "minted",
			"mintedAt": "2026-08-30T10:00:00Z",
		},
	}
	path := filepath.Join(root, "artifacts", "agents", "jobs", job+".json")
	if err := writeRecord(path, record); err != nil {
		t.Fatal(err)
	}
	supervisor := nativeTestExact(7001, 3)
	prober := fixedStartReader{exact: supervisor, state: identity.Alive}
	if err := ConsumeLaunchCapability(root, job, raw, "follow-up", tag, supervisor.Pid, prober); err == nil || !strings.Contains(err.Error(), "does not bind") {
		t.Fatalf("wrong adapter verb consumed the capability: %v", err)
	}
	if err := ConsumeLaunchCapability(root, job, raw, "dispatch", tag, supervisor.Pid, prober); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeLaunchCapability(root, job, raw, "dispatch", tag, supervisor.Pid, prober); err == nil || !strings.Contains(err.Error(), "already consumed") {
		t.Fatalf("capability replay was accepted: %v", err)
	}
}
