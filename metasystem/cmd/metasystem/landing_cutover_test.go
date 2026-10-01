package main

// The hard cutover from the old batch owner to the lane's kernel (lane
// design r10 §5, migration step 0, Astra R9-01; unit D): the old owner and
// its authority are deleted, so nothing acts under its lineage any more.
// While the ledger shows any goal claimed under that lineage the new claim
// identity is not activated (landing set refuses, naming each goal and a
// person's release), no lane act moves such a claim, and no process holding
// the lane checkout under that lineage is admitted as the lane's caller.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// oldOwnerLineage is the deleted batch owner's lineage, spelled as the
// ledger records it.
const oldOwnerLineage = "landing-m1l"

func TestCutoverRefusedWithOldOwnerClaims(t *testing.T) {
	// landing set refuses while the old owner holds claims, naming each,
	// and even while an older engine's lane is still recorded it points at
	// a person's release: no return goes through the deleted owner.
	t.Run("set", func(t *testing.T) {
		verbs := newLaneVerbBed(t)
		verbs.oldClaims = []string{"goal-a", "goal-b"}
		if err := os.MkdirAll(lane.HostDir(verbs.home), 0o700); err != nil {
			t.Fatal(err)
		}
		older := `{"root": "` + verbs.landingB + `", "registeredBy": "Wido", "at": "2026-09-29T10:00:00Z"}` + "\n"
		if err := os.WriteFile(lane.RecordPath(verbs.home), []byte(older), 0o600); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := verbs.run(t, "landing", "set", verbs.landingA, "--json")
		var refused intentResult
		if err := json.Unmarshal([]byte(stdout+stderr), &refused); err != nil {
			t.Fatal(err)
		}
		if code == 0 || refused.Outcome != intentRefused || !strings.Contains(refused.Summary, "goal-a") || !strings.Contains(refused.Summary, "goal-b") ||
			refused.Next == nil || !slices.Equal(refused.Next.Argv[:4], []string{"metasystem", "goal", "release", "goal-a"}) {
			t.Fatalf("set while the old owner holds claims, an older lane recorded = %d %+v; want refused, naming each and a person's release", code, refused)
		}
		if record, _, _ := lane.Read(verbs.home); record.Root != verbs.landingB || record.CustodyEpoch != 0 {
			t.Fatalf("the refused set changed the lane record: %+v", record)
		}
	})

	// A process holding the lane checkout under the old lineage is not the
	// lane's caller: no proof is charged to the lane for it.
	t.Run("proof", func(t *testing.T) {
		t.Setenv("METASYSTEM_OWNER_LINEAGE", "")
		t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", t.TempDir())
		controlRoot := holdNestedLaneCheckout(t, oldOwnerLineage)
		if err := proveLaneOwnerCaller(controlRoot, int64(os.Getpid())); err == nil || !strings.Contains(err.Error(), "not by its landing agent") {
			t.Fatalf("a lane checkout held under the old owner's lineage charged the lane: %v", err)
		}
	})
}

// holdNestedLaneCheckout registers a nested lane checkout on the test's
// host home and holds its installation's lease as this process under lineage; it returns
// the checkout's control root (its module).
func holdNestedLaneCheckout(t *testing.T, lineage string) string {
	t.Helper()
	home, err := board.Home()
	if err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe: %s %v", state, err)
	}
	checkout := t.TempDir()
	if err := os.MkdirAll(filepath.Join(checkout, "metasystem"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "metasystem", "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	registerLane(t, home, checkout, "test", time.Now())
	if batch.ModuleRoot(checkout) == checkout {
		t.Fatalf("fixture is not nested: %s", checkout)
	}
	if _, err := lease.AnnounceWithPair(batch.ModuleRoot(checkout), "session-"+lineage, int64(os.Getpid()), exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "lane-cutover-test", "metasystem", lineage); err != nil {
		t.Fatal(err)
	}
	if holder, err := lease.RequireHolder(batch.ModuleRoot(checkout), int64(os.Getpid()), nil); err != nil || !holder.Holder {
		t.Fatalf("hold the lane installation as %s: %+v %v", lineage, holder, err)
	}
	return batch.ModuleRoot(checkout)
}
