package evidence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// The record prune treats a chain whose mirror a person removed as current
// (engine-owns-disk-lifetimes 3.12, DL4D-13): the records go after the
// grace; a removal that is not done keeps them.
func TestTheRecordPruneHonoursAPersonsRemoval(t *testing.T) {
	t.Parallel()
	root, evidenceRoot, _, jobs := checkout(t)
	segment := filepath.Join(evidenceRoot, "agents", dispatch.CheckoutSegment(root))
	tombstone := func(chain, state string) {
		writeFile(t, filepath.Join(jobs, chain+".json"), `{"jobId": "`+chain+`", "status": "completed"}`)
		writeFile(t, diskstore.RemovedTombstonePath(filepath.Join(segment, chain)),
			`{"schema":"`+diskstore.TombstoneSchema+`","item":"`+chain+`","kind":"chain","history":[],"inventoryDigest":"x","step":"remove","rule":"person","by":"wido","at":"2020-01-01T00:00:00Z","receipt":"01R","bytesBefore":1,"bytesAfter":0,"state":"`+state+`"}`)
	}
	tombstone("removed", diskstore.StateDone)
	tombstone("uncommitted", diskstore.StateBegun)
	runGC(t, root, evidenceRoot)
	if _, err := os.Stat(filepath.Join(jobs, "removed.json")); !os.IsNotExist(err) {
		t.Fatalf("the record of a removed chain goes after the grace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(jobs, "uncommitted.json")); err != nil {
		t.Fatalf("an unfinished removal keeps the record: %v", err)
	}
}
