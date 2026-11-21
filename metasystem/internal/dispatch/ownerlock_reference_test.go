package dispatch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type workspaceHolder struct{ state identity.Liveness }

func (p *workspaceHolder) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartTicks: 57, BootID: "boot", ArgvKnown: true, Argv: []string{"other"}}, p.state, nil
}

func TestOwnerLockReferencesRoundTripAndOtherBindingsKeepTheirBytes(t *testing.T) {
	t.Parallel()
	now := func() time.Time { return time.Unix(1000, 0) }
	codec := ownerLockCodec{now: now}
	holder := lock.Identity{Pid: 12, Tag: "chain"}
	data, err := codec.Encode(holder)
	const legacy = "{\"acquiredAt\":\"1970-01-01T00:16:40Z\",\"instanceTag\":\"chain\",\"pid\":12}\n"
	if err != nil || string(data) != legacy {
		t.Fatalf("legacy encoding %q, %v", data, err)
	}
	holder.Process = identity.Ref{Pid: 12, StartTicks: 57, BootID: "boot"}
	data, err = codec.Encode(holder)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := codec.Decode(data); err != nil || got != holder {
		t.Fatalf("owner round trip: %+v, %v", got, err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	record := diskstore.Record{Owner: diskstore.Owner{Ref: "chain"}, Path: filepath.Join(root, "workspace")}
	dir := filepath.Join(root, "artifacts", "agents", "locks", "chain.d")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "owner.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	prober := &workspaceHolder{}
	proof := DelegateWorkspaceProof{Repo: root, lockOptions: OwnerLockOptions{Now: now, Prober: prober}}
	for _, state := range []identity.Liveness{identity.Alive, identity.Unknown, identity.Dead} {
		prober.state = state
		gone, err := proof.recordsGone("chain", record.Path)
		if gone || err == nil || errors.Is(err, ErrDispatchLockReclaim) != (state == identity.Dead) {
			t.Fatalf("%s: gone %v, %v", state, gone, err)
		}
	}
	if err := proof.Reclaim(record); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reclaimed lock: %v", err)
	}
}
