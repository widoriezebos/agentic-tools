package main

// U5f (engine-owns-disk-lifetimes Part B, 3.2 "Delegate", Round B3-3 rule 2,
// fail-closed rule 5): a person's discard releases the chain's workspace in
// that invocation, through the delegate proof, and leaves no authority a
// later pass could release under.

import (
	"context"
	"crypto/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

type fakeDiscarder struct {
	calls   *[]diskstore.Discard
	verdict diskstore.Verdict
}

func (f fakeDiscarder) ReleaseDiscarded(_ context.Context, _ diskstore.Registry, _ string, _ *diskstore.UseCensus, discard diskstore.Discard) (diskstore.Verdict, error) {
	*f.calls = append(*f.calls, discard)
	return f.verdict, nil
}

func TestDiskDiscardReleasesNowAndRecordsNoAuthority(t *testing.T) {
	t.Parallel()
	bed := newDiskBed(t)
	registry := diskstore.CheckoutRegistry(bed.inst)
	record, err := registry.Register(diskstore.Registration{Path: filepath.Join(bed.inst, "artifacts", "agents", "worktrees", "chain-7"), Git: true,
		Class: diskstore.DelegateClass, Owner: diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: "chain-7"}, Lifetime: diskstore.LifetimeOwner,
		CapKind: diskstore.CapTarget, Layout: diskstore.LayoutCopy}, diskNow, rand.Reader)
	helmMust(t, err)
	var calls []diskstore.Discard
	bed.owners.disk.discarder = func(string, time.Time) (diskstore.Discarder, error) {
		return fakeDiscarder{calls: &calls, verdict: diskstore.Verdict{Decision: diskstore.Release, Reason: "chain closed; its workspace released"}}, nil
	}
	code, out := bed.run("disk", "clean", "--discard", "j2:chain-7", "--reason", "abandoned bed")
	if code != 0 || len(calls) != 1 || calls[0].By != "Wido" || calls[0].Reason != "abandoned bed" || !strings.Contains(out, "discard: 1 done") {
		t.Fatalf("the discard releases now through the delegate proof = %d %+v:\n%s", code, calls, out)
	}
	if loaded, err := registry.Load(record.ID); err != nil || loaded.AuthorizedDiscard != nil {
		t.Fatalf("no authority is left on the record: %+v %v", loaded, err)
	}
	if code, out := bed.run("disk", "clean", "--discard", "j2:no-such-chain", "--reason", "x"); code != 2 || !strings.Contains(out, "no registered workspace") {
		t.Fatalf("a chain without a workspace = %d:\n%s", code, out)
	}
}
