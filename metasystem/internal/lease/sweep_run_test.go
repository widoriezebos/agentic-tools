package lease

import (
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// MON-06 (sweep side): a stale-epoch run without a provable group makes
// the sweep REFUSE loudly — it never signals blind and never silently
// terminalizes; a fresh-epoch or human (null-epoch) run is untouched.
func TestRunSweepProofOrRefuse(t *testing.T) {
	root := t.TempDir()
	c, _ := newClaimer(root)

	nonce := strings.Repeat("ef", 16)
	writeRun := func(id string, epoch string, status string) {
		record := `{"schemaVersion":1,"runId":"` + id + `","kind":"suite","display":"x","custody":"wrapped",` +
			`"generation":1,"pid":424242,"pidStartedAt":5000,"pgid":424242,"launchNonce":"` + nonce + `",` +
			`"log":"/tmp/x.log","startedAt":"2026-08-15T10:00:00Z","mainId":"main-old","ownerLineage":"main-old",` +
			`"claimEpoch":` + epoch + `,"sessionId":"s","goalId":"","staleAfterMin":30,"windDownMin":10,` +
			`"evidence":{"mode":"exit-sidecar"},"expect":{"green":"","red":"","hung":"","unknown":""},` +
			`"status":"` + status + `","acked":false}`
		os.MkdirAll(run.Dir(root), 0o755)
		os.WriteFile(run.RecordPath(root, id), []byte(record), 0o644)
	}

	// A stale-epoch running run: pgid 424242 has no live member carrying
	// the nonce, so ownership is disproven — the sweep refuses loudly.
	writeRun("stale-run", "1", "running")
	err := c.cleanupStaleRuns(5)
	if err == nil || !strings.Contains(err.Error(), "stale-run") {
		t.Fatalf("unproven stale run did not refuse loudly: %v", err)
	}
	// The record was NOT silently terminalized.
	record, _ := (&run.Store{Root: root}).Read("stale-run")
	if record.Status != "running" {
		t.Fatalf("the sweep terminalized without proof: %s", record.Status)
	}

	// A fresh-epoch run is out of the sweep's scope entirely.
	writeRun("stale-run", "9", "running")
	if err := c.cleanupStaleRuns(5); err != nil {
		t.Fatalf("fresh-epoch run swept: %v", err)
	}
	// A human run (null epoch) likewise.
	os.WriteFile(run.RecordPath(root, "stale-run"), []byte(strings.Replace(
		func() string { data, _ := os.ReadFile(run.RecordPath(root, "stale-run")); return string(data) }(),
		`"claimEpoch":9`, `"claimEpoch":null`, 1)), 0o644)
	if err := c.cleanupStaleRuns(5); err != nil {
		t.Fatalf("human run swept: %v", err)
	}
}
