package evidence

// U5g (engine-owns-disk-lifetimes Part B, 3.11): the collector and the
// sweeper share artifacts/agents. The collector never prunes an empty
// directory inside a registered store, never collects a chain's payload or
// prunes its job records while the chain's workspace store is registered
// and unreleased, and holds all three whenever the registry cannot be read
// whole (fail-closed rule 1).

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// registerStore registers and accepts a marker store at path, owned by
// owner, in the checkout registry of root.
func registerStore(t *testing.T, root, path string, owner diskstore.Owner) diskstore.Record {
	t.Helper()
	registry := diskstore.CheckoutRegistry(root)
	record, err := registry.Register(diskstore.Registration{Path: path, Class: "fixture store", Owner: owner,
		Lifetime: diskstore.LifetimeOwner, CapKind: diskstore.CapNone}, testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := diskstore.WriteMarker(record); err != nil {
		t.Fatal(err)
	}
	if record, err = registry.Accept(record.ID); err != nil {
		t.Fatal(err)
	}
	return record
}

// runGCAt is one collection pass at testNow, read by the pass itself and
// never through the package clock, so these tests run in parallel.
func runGCAt(t *testing.T, root, evidenceRoot string) string {
	t.Helper()
	var out strings.Builder
	if err := gcWithGoalEndpoint(root, evidenceRoot, 5400, &out, nil, testNow); err != nil {
		t.Fatalf("GC: %v", err)
	}
	return out.String()
}

func ageDir(t *testing.T, dirs ...string) {
	t.Helper()
	old := testNow.Add(-3 * time.Hour)
	for _, dir := range dirs {
		if err := os.Chtimes(dir, old, old); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGCKeepsOldEmptyDirectoriesInsideARegisteredStore(t *testing.T) {
	t.Parallel()
	root, evidenceRoot, agents, _ := checkout(t)
	store := filepath.Join(agents, "workspaces", "goal-G", "default")
	registerStore(t, root, store, diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: "G"})
	output := filepath.Join(store, "out", "empty")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	ageDir(t, output, filepath.Dir(output))
	// An old empty directory outside every store still goes.
	loose := filepath.Join(agents, "loose", "empty")
	if err := os.MkdirAll(loose, 0o755); err != nil {
		t.Fatal(err)
	}
	ageDir(t, loose, filepath.Dir(loose))

	runGCAt(t, root, evidenceRoot)

	if _, err := os.Stat(output); err != nil {
		t.Fatalf("a live run's old empty output directory inside a registered store must survive GC: %v", err)
	}
	if _, err := os.Stat(loose); !os.IsNotExist(err) {
		t.Fatalf("an old empty directory outside every store should still be pruned: %v", err)
	}
}

// Rule 2: the store is found by file identity, not by the spelling its
// record carries: a record naming the store through a symlinked parent
// still protects the directory the collector walks by its real path.
func TestGCProtectsARegisteredStoreByIdentityNotSpelling(t *testing.T) {
	t.Parallel()
	root, evidenceRoot, agents, _ := checkout(t)
	link := filepath.Join(filepath.Dir(root), "spelled")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(agents, "workspaces", "goal-G", "default")
	registerStore(t, root, filepath.Join(link, "artifacts", "agents", "workspaces", "goal-G", "default"),
		diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: "G"})
	output := filepath.Join(real, "out")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	ageDir(t, output)

	runGCAt(t, root, evidenceRoot)

	if _, err := os.Stat(output); err != nil {
		t.Fatalf("the store spelled through a symlink is the same directory and must be protected: %v", err)
	}
}

// closedMirroredChain lays out a closed chain whose payload and records the
// mirror accounts for, mirrored well past the grace.
func closedMirroredChain(t *testing.T, root, evidenceRoot, agents, jobs, chain string) {
	t.Helper()
	payload := "the round result\n"
	writeFile(t, filepath.Join(agents, chain, "rounds", "1", "return.json"), payload)
	writeFile(t, filepath.Join(jobs, chain+".json"), fmt.Sprintf(`{"jobId": %q, "status": "completed", "chainClosed": true}`, chain))
	writeFile(t, filepath.Join(evidenceRoot, "agents", dispatch.CheckoutSegment(root), chain, "manifest.json"),
		fmt.Sprintf(`{"updatedAt": "2026-08-10T08:00:00Z", "files": {"rounds/1/return.json": {"sha256": %q}, "jobs/%s.json": %s}}`,
			digestOf(payload), chain, recordEntry(t, jobs, chain+".json")))
}

func TestGCDefersToAnUnreleasedDelegateWorkspaceInBothOrders(t *testing.T) {
	t.Parallel()
	for _, order := range []string{"gc-then-release", "release-then-gc"} {
		t.Run(order, func(t *testing.T) {
			t.Parallel()
			root, evidenceRoot, agents, jobs := checkout(t)
			closedMirroredChain(t, root, evidenceRoot, agents, jobs, "chain-a")
			store := registerStore(t, root, filepath.Join(agents, "worktrees", "chain-a"),
				diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: "chain-a"})
			release := func() {
				if _, err := diskstore.CheckoutRegistry(root).Transition(store.ID,
					[]diskstore.State{diskstore.StateAccepted}, diskstore.StateReleased, nil); err != nil {
					t.Fatal(err)
				}
			}
			if order == "gc-then-release" {
				out := runGCAt(t, root, evidenceRoot)
				if !strings.Contains(out, "kept      chain-a:") || !strings.Contains(out, store.ID) {
					t.Fatalf("the chain must be kept naming its unreleased workspace store: %s", out)
				}
				for _, path := range []string{filepath.Join(agents, "chain-a"), filepath.Join(jobs, "chain-a.json")} {
					if _, err := os.Stat(path); err != nil {
						t.Fatalf("%s must survive GC while the workspace is unreleased: %v", path, err)
					}
				}
				// The record prune defers too: the payload is already
				// gone in this layout, and the record still stays.
				collected, reason, err := CollectChain(root, evidenceRoot, "chain-a")
				if err != nil || collected || !strings.Contains(reason, store.ID) {
					t.Fatalf("CollectChain must decline naming the store: %v %v %q", collected, err, reason)
				}
			}
			release()
			out := runGCAt(t, root, evidenceRoot)
			if !strings.Contains(out, "collected chain-a\n") {
				t.Fatalf("once released the chain is collected by the next GC: %s", out)
			}
			if _, err := os.Stat(filepath.Join(agents, "chain-a")); !os.IsNotExist(err) {
				t.Fatalf("payload should be gone: %v", err)
			}
		})
	}
}

func TestGCDefersRecordPruningWhileTheChainWorkspaceIsUnreleased(t *testing.T) {
	t.Parallel()
	root, evidenceRoot, agents, jobs := checkout(t)
	// The payload is gone already (collected before the store was
	// registered); the record is mirrored past the grace.
	writeFile(t, filepath.Join(jobs, "chain-b.json"), `{"jobId": "chain-b", "status": "completed", "chainClosed": true}`)
	writeFile(t, filepath.Join(evidenceRoot, "agents", "chain-b", "manifest.json"),
		fmt.Sprintf(`{"updatedAt": "2026-08-10T08:00:00Z", "files": {"jobs/chain-b.json": %s}}`, recordEntry(t, jobs, "chain-b.json")))
	registerStore(t, root, filepath.Join(agents, "worktrees", "chain-b"), diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: "chain-b"})

	runGCAt(t, root, evidenceRoot)

	if _, err := os.Stat(filepath.Join(jobs, "chain-b.json")); err != nil {
		t.Fatalf("CloseCheck's local records must survive until the workspace is released: %v", err)
	}
}

// Rule 1: an unreadable record (and, as its variant, a record of an unknown
// schema) holds every collection of the pass.
func TestGCHoldsEverythingWhenTheStoreRegistryCannotBeRead(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"garbage":        "{not json",
		"unknown schema": `{"schema": "metasystem.diskstore/9", "id": "01K2Z7Q3M8XW1V0P9D4J6S5R2T"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, evidenceRoot, agents, jobs := checkout(t)
			closedMirroredChain(t, root, evidenceRoot, agents, jobs, "chain-c")
			writeFile(t, filepath.Join(jobs, "old.json"), `{"jobId": "old", "status": "completed"}`)
			writeFile(t, filepath.Join(evidenceRoot, "agents", "old", "manifest.json"),
				fmt.Sprintf(`{"updatedAt": "2026-08-10T08:00:00Z", "files": {"jobs/old.json": %s}}`, recordEntry(t, jobs, "old.json")))
			loose := filepath.Join(agents, "loose")
			if err := os.MkdirAll(loose, 0o755); err != nil {
				t.Fatal(err)
			}
			ageDir(t, loose)
			writeFile(t, filepath.Join(diskstore.CheckoutRegistry(root).Dir, "01K2Z7Q3M8XW1V0P9D4J6S5R2T.json"), content)

			out := runGCAt(t, root, evidenceRoot)

			for _, path := range []string{filepath.Join(agents, "chain-c"), filepath.Join(jobs, "old.json"), loose} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("%s must survive a pass whose store registry cannot be read: %v", path, err)
				}
			}
			if !strings.Contains(out, "store registry") {
				t.Fatalf("the output must say why nothing was collected: %s", out)
			}
			collected, _, err := CollectChain(root, evidenceRoot, "chain-c")
			if err != nil || collected {
				t.Fatalf("CollectChain must hold too: %v %v", collected, err)
			}
		})
	}
}

func TestGCNeverTreatsStoreRootsAsChains(t *testing.T) {
	t.Parallel()
	root, evidenceRoot, agents, _ := checkout(t)
	for _, dir := range []string{"stores", "workspaces", "proof-runs", "suite-failures", "candidate-engines", "steward"} {
		writeFile(t, filepath.Join(agents, dir, "keep.txt"), "x")
	}
	out := runGCAt(t, root, evidenceRoot)
	for _, dir := range []string{"stores", "workspaces", "proof-runs", "suite-failures", "candidate-engines", "steward"} {
		if strings.Contains(out, "kept      "+dir+":") {
			t.Fatalf("%s is infrastructure, never a chain candidate: %s", dir, out)
		}
	}
}
