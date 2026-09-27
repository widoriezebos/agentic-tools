package report

// Ported from scripts/agents/brain-fixtures.sh scenario
// brain-declare-quiescence (verb redesign U7b part 3): the in-flight facts a
// brain declaration must refuse on, one at a time, through the declaration
// scan with stubbed goal reads. The refusal wording is proved by
// cmd/metasystem TestBrainBedDeclareQuiescenceRefusals.

import (
	"fmt"
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestBrainBedDeclarationScanFindsEachInFlightKind(t *testing.T) {
	t.Parallel()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("test process identity unreadable: %v %v", state, err)
	}
	for _, test := range []struct {
		name, path, body, kind, id string
	}{
		{
			name: "pending job", path: "artifacts/agents/jobs/pending-job.json",
			body: `{"jobId":"pending-job","status":"pending","role":"implementer","runtime":"fake"}`,
			kind: "job", id: "pending-job",
		},
		{
			name: "pending-setup job", path: "artifacts/agents/jobs/pending-setup-job.json",
			body: `{"jobId":"pending-setup-job","status":"pending-setup","role":"implementer","runtime":"fake"}`,
			kind: "job", id: "pending-setup-job",
		},
		{
			name: "launching run", path: "artifacts/agents/runs/live-run.json",
			body: `{"schemaVersion":1,"runId":"live-run","kind":"custom","display":"fixture","custody":"wrapped","generation":1,"launchNonce":"0123456789abcdef0123456789abcdef","log":"","startedAt":"2026-09-07T00:00:00Z","sessionId":"fixture","goalId":"","staleAfterMin":10,"windDownMin":1,"evidence":{"mode":"none"},"expect":{"green":"","red":"","hung":"","unknown":""},"status":"launching","acked":false}`,
			kind: "run", id: "live-run",
		},
		{
			name: "running mission", path: "artifacts/agents/missions/runners/fixture-mission.json",
			body: fmt.Sprintf(`{"missionId":"fixture-mission","status":"running","pid":%d,"pidStartedAt":%d}`, os.Getpid(), exact.StartedAt.Unix()),
			kind: "mission", id: "fixture-mission",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := resolveRepo(t.TempDir())
			writeFile(t, root, test.path, test.body+"\n")
			reads := absentScanReads(t, root)
			reads.expect("endpoint", "accepted", "world")
			scan := scanForBrainDeclarationWithReads(root, reads.reads())
			reads.checked(0)
			if len(scan.Unreadable) != 0 || len(scan.RunUnreadable) != 0 {
				t.Fatalf("scan inputs were unreadable: %v %v", scan.Unreadable, scan.RunUnreadable)
			}
			found := false
			for _, item := range scan.Busy {
				if item.Kind == test.kind && item.Id == test.id {
					found = true
				}
			}
			if !found {
				t.Fatalf("brain declaration scan missed %s %s: %+v", test.kind, test.id, scan.Busy)
			}
		})
	}

	t.Run("quiescent", func(t *testing.T) {
		t.Parallel()
		root := resolveRepo(t.TempDir())
		reads := absentScanReads(t, root)
		reads.expect("endpoint", "accepted", "world")
		scan := scanForBrainDeclarationWithReads(root, reads.reads())
		reads.checked(0)
		if len(scan.Busy) != 0 {
			t.Fatalf("an empty checkout was not quiescent: %+v", scan.Busy)
		}
	})
}
