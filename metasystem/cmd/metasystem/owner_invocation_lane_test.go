package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// Only the landing lane's own calls publish through the lane's boundary
// (design r10 K3): a seat's invocation carries none, and the synced-ledger
// request an invocation builds resolves its endpoint through the boundary
// it carries, so a lane write the boundary refuses leaves main where it was
// while a seat's write pushes as it always did.
func TestOnlyTheLanesOwnerCallsTakeTheBoundary(t *testing.T) {
	t.Parallel()
	if ownercall.FromThisProcess("seat-lineage").Ledger != nil {
		t.Fatal("a seat's invocation carries a publication boundary")
	}
	base := t.TempDir()
	origin, checkout := filepath.Join(base, "origin.git"), filepath.Join(base, "checkout")
	git := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=seat", "-c", "user.email=seat@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(base, "init", "--quiet", "--bare", "-b", "main", origin)
	git(base, "clone", "--quiet", origin, checkout)
	if err := os.WriteFile(filepath.Join(checkout, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(checkout, "add", "README.md")
	git(checkout, "commit", "--quiet", "-m", "seed")
	git(checkout, "push", "--quiet", "origin", "main")
	tip := git(checkout, "rev-parse", "HEAD")
	next := git(checkout, "commit-tree", git(checkout, "rev-parse", "HEAD^{tree}"), "-p", tip, "-m", "goal write\n\nGoal-Transaction: op-1")

	boundary := batchowner.LaneInvocation(lane.ClaimIdentity{Machine: "lane-host", Lineage: lane.ClaimLineage, Epoch: 1})
	boundary.Ledger = func(endpoint goal.Endpoint) goal.Endpoint {
		return endpoint.WithCASPublisher(func(goal.Endpoint, string, string) (goal.CASOutcome, error) {
			return goal.CASRefused, &lane.PublishError{Code: lane.CodeBaseMoved, Message: "main moved"}
		})
	}
	laneEndpoint, err := ownerSyncDependencies(boundary).endpoint(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := goal.PublishCAS(laneEndpoint, tip, next); outcome != goal.CASRefused || !lane.IsBaseMoved(err) {
		t.Fatalf("a lane goal write through its boundary = %s %v; want the boundary's refusal", outcome, err)
	}
	if main := git(origin, "rev-parse", "refs/heads/main"); main != tip {
		t.Fatalf("the refused lane write moved main to %s", main)
	}
	seatEndpoint, err := ownerSyncDependencies(ownercall.FromThisProcess("seat-lineage")).endpoint(checkout)
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := goal.PublishCAS(seatEndpoint, tip, next); outcome != goal.CASLanded || err != nil {
		t.Fatalf("a seat's goal write = %s %v; want it pushed as before", outcome, err)
	}
	if main := git(origin, "rev-parse", "refs/heads/main"); main != next {
		t.Fatalf("main is %s after the seat's write, want %s", main, next)
	}
}
