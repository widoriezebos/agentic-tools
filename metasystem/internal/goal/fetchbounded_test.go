package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// singleMachineLedgerBed is a converted checkout whose ledger commit sits on
// the dedicated single-machine branch and whose accepted ref has never been
// created — the world a first advance walks into. Two beds built from the
// same bytes at the same instant carry the same commit, so two advances over
// them are comparable results rather than two unrelated oids.
func singleMachineLedgerBed(t *testing.T) string {
	t.Helper()
	root := validatedBed(t, "2026-08-23T09:00:00+00:00", bedRoot(), map[string][]byte{
		"alpha.md": RenderFile(bedGoal("alpha")),
	})
	mustGit(t, root, "update-ref", LocalLedgerBranch, "HEAD")
	mustGit(t, root, "update-ref", "-d", AcceptedRef)
	return root
}

// remoteLedgerBed publishes a ledger on a bare origin and hands back a clone
// of it. The clone's fetch refspec covers refs/heads/* alone, so it starts
// without an accepted ref exactly as a fresh clone does.
func remoteLedgerBed(t *testing.T) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	mustGit(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", origin)
	seed := filepath.Join(t.TempDir(), "seed")
	mustGit(t, t.TempDir(), "clone", "-q", origin, seed)
	write := func(rel string, data []byte) {
		t.Helper()
		abs := filepath.Join(seed, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, data, 0o644); err != nil {
			t.Fatal(err)
		}
		mustGit(t, seed, "add", rel)
	}
	remoteRoot := bedRoot()
	remoteRoot.SyncMode = SyncRemote
	write(goalsPrefix+"backlog.md", RenderRoot(remoteRoot))
	write(goalsPrefix+"alpha.md", RenderFile(bedGoal("alpha")))
	mustGit(t, seed, "commit", "-qm", "publish the ledger")
	mustGit(t, seed, "push", "-q", "origin", "main")
	clone := filepath.Join(t.TempDir(), "clone")
	mustGit(t, t.TempDir(), "clone", "-q", origin, clone)
	return clone
}

func metasystemRefs(t *testing.T, root string) string {
	t.Helper()
	return mustGit(t, root, "for-each-ref", "--format=%(refname) %(objectname)", "refs/metasystem/")
}

// TestFetchAdvanceBoundedIsTheSameAcceptanceSequence holds the bounded
// advance to the unbounded one's answer. Two beds carrying identical bytes
// carry an identical commit, so the two results compare directly.
func TestFetchAdvanceBoundedIsTheSameAcceptanceSequence(t *testing.T) {
	t.Parallel()
	unbounded := Endpoint{Root: singleMachineLedgerBed(t), Remote: "local", Branch: LocalLedgerBranch}
	bounded := Endpoint{Root: singleMachineLedgerBed(t), Remote: "local", Branch: LocalLedgerBranch}

	wanted, err := FetchAdvance(unbounded)
	testutil.Require(t, "the unbounded advance", err, nil)
	observed, err := FetchAdvanceBounded(bounded, time.Minute)
	testutil.Require(t, "the bounded advance", err, nil)
	testutil.Require(t, "the advance moved the ref", wanted.Advanced, true)
	testutil.Expect(t, "the bounded result", observed, wanted)

	wantedAgain, err := FetchAdvance(unbounded)
	testutil.Require(t, "the second unbounded advance", err, nil)
	observedAgain, err := FetchAdvanceBounded(bounded, time.Minute)
	testutil.Require(t, "the second bounded advance", err, nil)
	testutil.Expect(t, "the settled detail", wantedAgain.Detail, "already at the canonical tip")
	testutil.Expect(t, "the second bounded result", observedAgain, wantedAgain)
}

// TestFetchAdvanceBoundedAdvancesOverTheTransport drives the bounded path
// that a single-machine bed never reaches: the remote capture, its
// acceptance gates, and the compare-and-swap that creates the accepted ref.
func TestFetchAdvanceBoundedAdvancesOverTheTransport(t *testing.T) {
	t.Parallel()
	root := remoteLedgerBed(t)
	endpoint, err := ResolveEndpoint(root)
	testutil.Require(t, "the endpoint", err, nil)
	testutil.Require(t, "the clone starts without an accepted ref", metasystemRefs(t, root), "")

	result, err := FetchAdvanceBounded(endpoint, time.Minute)
	testutil.Require(t, "the first advance", err, nil)
	testutil.Expect(t, "the advance moved the ref", result.Advanced, true)
	published := mustGit(t, root, "rev-parse", "--verify", "refs/remotes/origin/main")
	testutil.Expect(t, "the accepted tip", result.Tip, published)
	testutil.Expect(t, "the accepted ref", metasystemRefs(t, root), AcceptedRef+" "+published)

	settled, err := FetchAdvanceBounded(endpoint, time.Minute)
	testutil.Require(t, "the second advance", err, nil)
	testutil.Expect(t, "the settled advance", settled, AdvanceResult{Tip: published, Detail: "already at the canonical tip"})
	testutil.Expect(t, "the refs after the second advance", metasystemRefs(t, root), AcceptedRef+" "+published)
}

// TestFetchAdvanceBoundedLeavesEveryRefWhenTheCaptureFails is the failure
// the scheduled caller sees most: a branch the remote does not carry. The
// pass must end as an error naming the cause, with the accepted ref where it
// was and no per-operation ref left behind for the next pass to trip over.
func TestFetchAdvanceBoundedLeavesEveryRefWhenTheCaptureFails(t *testing.T) {
	t.Parallel()
	root := remoteLedgerBed(t)
	endpoint, err := ResolveEndpoint(root)
	testutil.Require(t, "the endpoint", err, nil)
	_, err = FetchAdvanceBounded(endpoint, time.Minute)
	testutil.Require(t, "the first advance", err, nil)
	before := metasystemRefs(t, root)

	mustGit(t, root, "config", "goal.sync-branch", "refs/heads/no-such-branch")
	missing, err := ResolveEndpoint(root)
	testutil.Require(t, "the reconfigured endpoint", err, nil)
	result, err := FetchAdvanceBounded(missing, time.Minute)
	if err == nil {
		t.Fatalf("a branch the remote does not carry was accepted: %+v", result)
	}
	testutil.Expect(t, "the refusal names the fetch", strings.Contains(err.Error(), "fetch"), true)
	testutil.Expect(t, "the refs after the refusal", metasystemRefs(t, root), before)
}

// TestFetchAdvanceBoundedRunsSingleMachineCaptureUnbounded records where the
// bound applies. Single-machine capture is git against paths on this machine,
// with no transport to hang, so it runs whatever budget it is handed; the
// budget governs the remote fetch alone.
func TestFetchAdvanceBoundedRunsSingleMachineCaptureUnbounded(t *testing.T) {
	t.Parallel()
	endpoint := Endpoint{Root: singleMachineLedgerBed(t), Remote: "local", Branch: LocalLedgerBranch}

	result, err := FetchAdvanceBounded(endpoint, 0)
	testutil.Require(t, "the advance under a zero budget", err, nil)
	testutil.Expect(t, "the advance moved the ref", result.Advanced, true)
}
