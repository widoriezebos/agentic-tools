package batchowner

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

type zombieProber struct{ exact identity.Exact }

func (p zombieProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return p.exact, identity.Alive, nil
}

// A landing owner that exited but whose parent has not reaped it is a zombie:
// it holds nothing, so the lane reads its owner as gone (landing restart and
// the ensure path can replace it) whether or not an announcement names it.
func TestBatchOwnerInspectionReadsAZombieOwnerAsDead(t *testing.T) {
	t.Parallel()
	holder := lease.CurrentHolderView{MainId: "main-a", OwnerLineage: LandingOwnerLineage, ClaimEpoch: 1, Pid: 41}
	read := func(string) (lease.CurrentHolderView, error) { return holder, nil }
	announced := func(string, int64) []lease.Announcement {
		return []lease.Announcement{{MainId: "main-a", Pid: 41, PidStartedAt: 2}}
	}
	none := func(string, int64) []lease.Announcement { return nil }
	zombie := zombieProber{exact: identity.Exact{Pid: 41, StartedAt: time.Unix(2, 0), Zombie: true, Exiting: true}}
	if pid, state, err := InspectBatchOwnerWith("root", zombie, read, announced); err != nil || pid != 41 || state != identity.Dead {
		t.Fatalf("announced zombie owner pid=%d state=%s error=%v, want dead", pid, state, err)
	}
	if _, state, err := InspectBatchOwnerWith("root", zombie, read, none); err != nil || state != identity.Dead {
		t.Fatalf("unannounced zombie owner state=%s error=%v, want dead", state, err)
	}
}
