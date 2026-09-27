package dispatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// dispatch-fixtures.sh L2480-2554 (U6b port, slice p4): a set-up but never
// launched reservation (pending, fingerprinted, no process identity) waits
// out its handshake window; once the window ends, its next reap action is
// nonce-wide reconciliation, and a complete census absence with a dead
// creator fails it as creator-abandoned without a process-loss or
// group-death claim.
func TestPendingIdentitylessReservationReconcilesOnlyAfterItsHandshakeWindow(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_790_000_000, 0).UTC()
	creator := stage4Exact(911, 1_789_999_000_000_001)
	record := map[string]any{
		"jobId": "launch-window", "status": "pending", "phase": "handshake",
		"fingerprintVersion": 2, "fingerprint": "launch-window-fingerprint", "instanceTag": "metasystem-job-launch-window-tag",
		"pid": nil, "pgid": nil, "sessionId": nil, "sessionEstablishedTimeoutSec": 60,
		"startedAt": now.Format(time.RFC3339), "createdAt": now.Format(time.RFC3339),
		"creatorLiveness": stage4RefObject(creator),
	}
	if facts := ComputeReapFactsForRecord(record, HandshakeBackstopGraceSec, now); !facts.HandshakeWaiting || facts.ReconciliationDue {
		t.Fatalf("a young reservation = %+v, want waiting and no reconciliation", facts)
	}
	record["startedAt"] = "2000-01-01T00:00:00Z"
	if facts := ComputeReapFactsForRecord(record, HandshakeBackstopGraceSec, now); facts.HandshakeWaiting || !facts.ReconciliationDue || facts.BudgetExpired {
		t.Fatalf("an out-of-window reservation = %+v, want reconciliation due", facts)
	}

	root := t.TempDir()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, jobs, "launch-window.json", record)
	result, err := ReconcileReservation(root, "launch-window", ReconciliationDependencies{
		Now:     func() time.Time { return now },
		Scanner: fixedAdoptionScanner{result: census.TaggedProcessCensus{}},
		Creator: &stage4ProcessTable{startStates: map[int64]identity.Liveness{creator.Pid: identity.Dead}},
	})
	if err != nil || result.Outcome != ReconciliationCreatorAbandoned {
		t.Fatalf("reconciliation = %+v err=%v", result, err)
	}
	got := readRecord(t, root, "launch-window")
	if asString(got["status"]) != "failed" || asString(got["error"]) != "creator-abandoned" || asString(got["phase"]) != "reconciliation" {
		t.Fatalf("an abandoned reservation = %+v", got)
	}
	if _, has := got["groupDeathProvenAt"]; has {
		t.Fatalf("creator abandonment claimed a process-group death: %+v", got)
	}
}
