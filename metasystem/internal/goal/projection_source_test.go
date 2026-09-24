package goal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type projectionReadFailure struct {
	Repository
	tip      string
	filesErr error
}

func (r projectionReadFailure) Accepted() (string, bool, error) { return r.tip, true, nil }

func (r projectionReadFailure) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	if r.filesErr != nil {
		return nil, r.filesErr
	}
	return r.Repository.Files(commit, prefixes...)
}

func projectionTestNow() time.Time {
	return time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
}

func writeProjectionTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectionSourceAcceptedWorldStates(t *testing.T) {
	t.Run("absent accepted ref is legacy only without a materialized root", func(t *testing.T) {
		store, client, _ := fakeServingFixture(t, "bed-m1", nil)
		client.accepted = ""
		converted, err := store.projectionWorld()
		if err != nil || converted {
			t.Fatalf("absent ref without root: converted=%t err=%v", converted, err)
		}
		if work, err := readClaimableBudgetedWork(store.Root, projectionTestNow(), identity.KernelProber{}, store.projectionDeps); err != nil || len(work.Claimable) != 0 {
			t.Fatalf("absent ref without root did not use legacy read: work=%+v err=%v", work, err)
		}
		writeProjectionTestFile(t, filepath.Join(store.Root, filepath.FromSlash(goalsPrefix+"backlog.md")), []byte("materialized"))
		converted, err = store.projectionWorld()
		if converted || err == nil || !strings.Contains(err.Error(), "canonical goal root exists") {
			t.Fatalf("materialized root must make absent ref uncertain: converted=%t err=%v", converted, err)
		}
	})
	t.Run("unreadable accepted ref never falls back to legacy", func(t *testing.T) {
		store, client, _ := fakeServingFixture(t, "bed-m1", nil)
		client.brokenAccepted = errors.New("unreadable ref")
		converted, err := store.projectionWorld()
		if converted || err == nil || !strings.Contains(err.Error(), "unreadable ref") {
			t.Fatalf("unreadable ref: converted=%t err=%v", converted, err)
		}
		if _, _, ok := store.ServingProjection(); ok {
			t.Fatal("serving projection exposed a corrupt accepted world")
		}
		if facts, status, _ := store.goalFacts(); facts != nil || status != "degraded" {
			t.Fatalf("goal facts did not degrade on corrupt ref: facts=%+v status=%q", facts, status)
		}
		if first, digest := store.queuedFrontier(); first != "" || digest != "" {
			t.Fatalf("queued frontier escaped corrupt ref: first=%q digest=%q", first, digest)
		}
		if fresh, digest, declared := store.freeState(); fresh || digest != "" || declared != "" {
			t.Fatalf("free state escaped corrupt ref: fresh=%t digest=%q declared=%q", fresh, digest, declared)
		}
		if _, err := readClaimableBudgetedWork(store.Root, projectionTestNow(), identity.KernelProber{}, store.projectionDeps); err == nil || !strings.Contains(err.Error(), "unreadable ref") {
			t.Fatalf("claimable read fell back on unreadable ref: %v", err)
		}
	})
	t.Run("present accepted tip needs committed root bytes", func(t *testing.T) {
		store, client, _ := fakeServingFixture(t, "bed-m1", nil)
		converted, err := store.projectionWorld()
		if err != nil || !converted {
			t.Fatalf("committed root: converted=%t err=%v", converted, err)
		}
		client.store.mu.Lock()
		seed := client.store.commits[client.accepted]
		seed.files = copyFakeFiles(seed.files)
		delete(seed.files, goalsPrefix+"backlog.md")
		client.store.commits[client.accepted] = seed
		client.store.mu.Unlock()
		converted, err = store.projectionWorld()
		if converted || err == nil || !strings.Contains(err.Error(), "readable canonical goal root") {
			t.Fatalf("missing committed root: converted=%t err=%v", converted, err)
		}
	})
	t.Run("empty tip and unreadable committed bytes are errors", func(t *testing.T) {
		store, client, _ := fakeServingFixture(t, "bed-m1", nil)
		source := *store.projectionDeps.source
		source.endpoint.Repository = projectionReadFailure{Repository: client, tip: ""}
		store.projectionDeps.source = &source
		if converted, err := store.projectionWorld(); converted || err == nil || !strings.Contains(err.Error(), "without a commit") {
			t.Fatalf("empty accepted tip: converted=%t err=%v", converted, err)
		}
		source.endpoint.Repository = projectionReadFailure{Repository: client, tip: client.accepted, filesErr: errors.New("blob unreadable")}
		if converted, err := store.projectionWorld(); converted || err == nil || !strings.Contains(err.Error(), "blob unreadable") {
			t.Fatalf("unreadable committed root: converted=%t err=%v", converted, err)
		}
	})
	t.Run("endpoint root must match resolved state root", func(t *testing.T) {
		store, _, _ := fakeServingFixture(t, "bed-m1", nil)
		source := *store.projectionDeps.source
		source.endpoint.Root = t.TempDir()
		store.projectionDeps.source = &source
		converted, err := store.projectionWorld()
		if converted || err == nil || !strings.Contains(err.Error(), "does not match resolved state root") {
			t.Fatalf("mismatched source: converted=%t err=%v", converted, err)
		}
		if _, err := readClaimableBudgetedWork(store.Root, projectionTestNow(), identity.KernelProber{}, store.projectionDeps); err == nil || !strings.Contains(err.Error(), "does not match resolved state root") {
			t.Fatalf("claimable read accepted mismatched source: %v", err)
		}
	})
	t.Run("injected source requires an explicit repository", func(t *testing.T) {
		store, _, _ := fakeServingFixture(t, "bed-m1", nil)
		source := *store.projectionDeps.source
		source.endpoint.Repository = nil
		store.projectionDeps.source = &source
		if converted, err := store.projectionWorld(); converted || err == nil || !strings.Contains(err.Error(), "has no repository") {
			t.Fatalf("repository-less source: converted=%t err=%v", converted, err)
		}
	})
	t.Run("injected source requires a machine identity", func(t *testing.T) {
		store, _, _ := fakeServingFixture(t, "bed-m1", nil)
		source := *store.projectionDeps.source
		source.machine = ""
		store.projectionDeps.source = &source
		if converted, err := store.projectionWorld(); converted || err == nil || !strings.Contains(err.Error(), "no usable machine") {
			t.Fatalf("machine-less source: converted=%t err=%v", converted, err)
		}
	})
}

func approvedProjectionCandidate(id string) *GoalFile {
	budget := testBudget()
	budget.ActiveJobLimit = 1
	f := budgetedQueuedGoal(id, "2026-08-23T00:00:00Z")
	f.Budget = &budget
	f.Tier = 3
	return f
}

func TestProjectionSourceClaimableReadUsesConfigAndIsolatedSources(t *testing.T) {
	first, firstClient, firstEndpoint := fakeServingFixture(t, "bed-m1", map[string]*GoalFile{
		"first": approvedProjectionCandidate("first"),
	})
	writeProjectionTestFile(t, filepath.Join(first.Root, "metasystem.conf"), []byte("metasystem.budget.tier-3=8h/10/1200m/1/3\n"))
	read := func(store *Store) ClaimableBudgetedWork {
		t.Helper()
		work, err := readClaimableBudgetedWork(store.Root, projectionTestNow(), identity.KernelProber{}, store.projectionDeps)
		if err != nil {
			t.Fatal(err)
		}
		return work
	}
	work := read(first)
	if len(work.Claimable) != 1 || work.Claimable[0] != "first" || len(work.Refused) != 0 {
		t.Fatalf("approved candidate was not claimable: %+v", work)
	}
	if len(firstClient.captures) == 0 || firstClient.captures[0] != firstClient.accepted {
		t.Fatalf("fresh read did not use real FetchAdvance: captures=%v accepted=%s", firstClient.captures, firstClient.accepted)
	}
	writeProjectionTestFile(t, filepath.Join(first.Root, "metasystem.conf"), []byte("metasystem.budget.tier-3=malformed\n"))
	if _, err := readClaimableBudgetedWork(first.Root, projectionTestNow(), identity.KernelProber{}, first.projectionDeps); err == nil || !strings.Contains(err.Error(), "metasystem.budget.tier-3") {
		t.Fatalf("malformed real tier budget did not refuse: %v", err)
	}

	second, secondClient, secondEndpoint := fakeServingFixture(t, "bed-m2", map[string]*GoalFile{
		"second": approvedProjectionCandidate("second"),
	})
	writeProjectionTestFile(t, filepath.Join(second.Root, "metasystem.conf"), []byte("metasystem.budget.tier-3=8h/10/1200m/1/3\n"))
	parent := secondClient.accepted
	next, err := secondClient.Build("second-tip", parent, []Change{{Path: "notes/second-tip", Content: []byte("different accepted commit")}}, "advance second fixture")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := secondClient.Publish(parent, next); err != nil || outcome != CASLanded {
		t.Fatalf("publish second accepted tree: outcome=%v err=%v", outcome, err)
	}
	if err := secondClient.AcceptedCAS(parent, next); err != nil {
		t.Fatal(err)
	}
	if firstClient.accepted == secondClient.accepted || firstEndpoint.Root == secondEndpoint.Root {
		t.Fatal("independent sources did not keep separate roots and accepted tips")
	}
	secondWork := read(second)
	if len(secondWork.Claimable) != 1 || secondWork.Claimable[0] != "second" || len(secondWork.Refused) != 0 {
		t.Fatalf("second source did not select its own accepted work: %+v", secondWork)
	}
	writeProjectionTestFile(t, filepath.Join(first.Root, "metasystem.conf"), []byte("metasystem.budget.tier-3=8h/10/1200m/1/3\n"))
	if firstWork := read(first); len(firstWork.Claimable) != 1 || firstWork.Claimable[0] != "first" {
		t.Fatalf("second source changed first source's frontier: %+v", firstWork)
	}
}

func TestProjectionSourceRoutesStoreConsumers(t *testing.T) {
	store, _, endpoint := fakeServingFixture(t, "bed-m1", map[string]*GoalFile{
		"held": {
			Id: "held", State: StateClaimed, Intent: "Carry the claim", Origin: OriginMain,
			NextStep: "Continue it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		},
		"waiting": {
			Id: "waiting", State: StateQueued, Intent: "Wait for approval", Origin: OriginMain,
			NextStep: "Approve it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 1,
		},
	})
	store.Now = projectionTestNow
	if id, intent, ok := store.ServingProjection(); !ok || id != "held" || intent != "Carry the claim" {
		t.Fatalf("serving projection: id=%q intent=%q ok=%t", id, intent, ok)
	}
	for name, read := range map[string]func() (*GoalFacts, string, string){
		"goalFacts":          store.goalFacts,
		"convertedGoalFacts": store.convertedGoalFacts,
	} {
		facts, status, detail := read()
		if status != "ok" || detail != "" || facts == nil || facts.Id != "held" || facts.NextStep != "Continue it." {
			t.Fatalf("%s: facts=%+v status=%q detail=%q", name, facts, status, detail)
		}
	}
	first, digest := store.queuedFrontier()
	if first != "waiting" || digest != sha256Hex([]byte("waiting@1")) {
		t.Fatalf("queued frontier: first=%q digest=%q", first, digest)
	}
	work, err := readClaimableBudgetedWork(store.Root, projectionTestNow(), identity.KernelProber{}, store.projectionDeps)
	if err != nil || len(work.Claimed) != 1 || work.Claimed[0] != "held" || work.Queued != 1 {
		t.Fatalf("the bound Store did not retain the real Next frontier: work=%+v err=%v", work, err)
	}
	projection, err := store.readProjection(endpoint, projectionTestNow())
	banners := strings.Join(projection.Banners, "\n")
	if err != nil || projection.Tree == nil || !strings.Contains(banners, "single-machine mode") || !strings.Contains(banners, "accepted tree is") {
		t.Fatalf("real Project lost local mode or freshness banners: projection=%+v err=%v", projection, err)
	}

	freeStore, client, _ := fakeServingFixture(t, "bed-m1", nil)
	freeStore.Now = projectionTestNow
	writeProjectionTestFile(t, filepath.Join(freeStore.Root, "plans", "one.md"), []byte("plan"))
	scan, err := ScanDigest(freeStore.Root)
	if err != nil {
		t.Fatal(err)
	}
	client.store.mu.Lock()
	seed := client.store.commits[client.accepted]
	seed.files = copyFakeFiles(seed.files)
	seed.files[goalsPrefix+"backlog.md"] = RenderRoot(&RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: SyncLocal, Revision: 1,
		Free: &FreeRecord{Declared: "2026-08-23T00:00:00Z", Origin: OriginHuman, Digest: scan},
	})
	client.store.commits[client.accepted] = seed
	client.store.mu.Unlock()
	if fresh, got, declared := freeStore.freeState(); !fresh || got != scan || declared != "2026-08-23T00:00:00Z" {
		t.Fatalf("fresh free state: fresh=%t digest=%q declared=%q", fresh, got, declared)
	}
	writeProjectionTestFile(t, filepath.Join(freeStore.Root, "plans", "two.md"), []byte("later plan"))
	if fresh, got, _ := freeStore.freeState(); fresh || got == scan {
		t.Fatalf("new plan did not stale free declaration: fresh=%t digest=%q", fresh, got)
	}
}
