package diskstore

// U5i (engine-owns-disk-lifetimes Part B, 3.12 placement rules): caches and
// source copies are recognized by their shape, never counted as evidence;
// a rebuildable store never enters an evidence root.

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var placementNow = time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

func TestPlacementRecognizesCachesAndSourceCopiesByShape(t *testing.T) {
	t.Parallel()
	base := realDir(t)
	write := func(rel, content string) {
		t.Helper()
		writeBedFile(t, filepath.Join(base, filepath.FromSlash(rel)), []byte(content))
	}
	write("go-cache/README", "This directory holds cached build artifacts from the Go build system.\nRun \"go clean -cache\" if the directory is getting too large.\n")
	write("go-cache/trim.txt", "1727600000\n")
	write("banner-only/README", "This directory holds cached build artifacts from the Go build system.\n")
	write("mod-cache/cache/download/example.com/lib/@v/list", "v1.0.0\n")
	write("staticcheck-cache/README", "This directory holds cached build artifacts from staticcheck.\n")
	write("gocache-x/00/a", "x")
	write("gomodcache-2/a", "x")
	write("clone/.git/HEAD", "ref: refs/heads/main\n")
	write("checkout-copy/metasystem/metasystem.conf", "metasystem.runtimes=fake\n")
	write("chain-a/jobs/chain-a.json", "{}")
	for name, want := range map[string]string{
		"go-cache": PlacementCache, "mod-cache": PlacementCache, "staticcheck-cache": PlacementCache, "gocache-x": PlacementCache,
		"gomodcache-2": PlacementCache, "clone": PlacementSourceCopy, "checkout-copy": PlacementSourceCopy,
		"banner-only": "", "chain-a": "",
	} {
		got := PlacementOf(filepath.Join(base, name))
		if got.Kind != want {
			t.Fatalf("%s: kind %q, want %q (%+v)", name, got.Kind, want, got)
		}
		if want != "" && got.Shape == "" {
			t.Fatalf("%s: a recognized tree says what shape it has", name)
		}
	}
}

// rebuildableBed registers an accepted rebuildable store owned by a goal.
func rebuildableBed(t *testing.T) (Registry, Record, string) {
	t.Helper()
	base := realDir(t)
	registry := CheckoutRegistry(filepath.Join(base, "checkout"))
	store := filepath.Join(base, "checkout", "artifacts", "agents", "workspaces", "goal-G", "engines")
	record, err := registry.Register(Registration{Path: store, Class: "candidate engines", Owner: Owner{Kind: OwnerGoal, Ref: "G"},
		Lifetime: LifetimeRebuildable, RebuildFrom: &RebuildFrom{Commit: "e83c5163316f89bfbde7d9ab23ca2e25604af290", Command: "go build ./cmd/metasystem"},
		CapKind: CapTarget}, placementNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	writeBedFile(t, filepath.Join(store, "bin", "metasystem"), []byte("\xcf\xfa\xed\xfeengine"))
	if err := WriteMarker(record); err != nil {
		t.Fatal(err)
	}
	if record, err = registry.Accept(record.ID); err != nil {
		t.Fatal(err)
	}
	return registry, record, store
}

func TestRebuildableHoldingFindsTheStoreByIdentity(t *testing.T) {
	t.Parallel()
	registry, record, store := rebuildableBed(t)
	link := filepath.Join(filepath.Dir(filepath.Dir(store)), "spelled")
	if err := os.Symlink(store, link); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(filepath.Dir(filepath.Dir(store)), "into-bin")
	if err := os.Symlink(filepath.Join(store, "bin"), inner); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{store, filepath.Join(store, "bin", "metasystem"), filepath.Join(link, "bin"), filepath.Join(inner, "metasystem")} {
		found, held, err := registry.RebuildableHolding(path)
		if err != nil || !held || found.ID != record.ID {
			t.Fatalf("%s lies in the rebuildable store: %+v %v %v", path, found, held, err)
		}
	}
	if _, held, err := registry.RebuildableHolding(filepath.Dir(store)); err != nil || held {
		t.Fatalf("a parent of the store is not inside it: %v %v", held, err)
	}
	// Released, it holds nothing.
	if _, err := registry.Transition(record.ID, []State{StateAccepted}, StateReleased, nil); err != nil {
		t.Fatal(err)
	}
	if _, held, err := registry.RebuildableHolding(store); err != nil || held {
		t.Fatalf("a released store holds nothing: %v %v", held, err)
	}
}

// Rule 1: a registry that cannot be read whole cannot say a path is free
// of rebuildable stores; the answer is an error, never "not held".
func TestRebuildableHoldingFailsClosedOnAnUnreadableRegistry(t *testing.T) {
	t.Parallel()
	registry, _, store := rebuildableBed(t)
	if err := os.WriteFile(registry.RecordPath("01K2Z7Q3M8XW1V0P9D4J6S5R2T"), []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := registry.RebuildableHolding(filepath.Join(filepath.Dir(store), "elsewhere")); err == nil {
		t.Fatal("an unreadable record must make the answer an error")
	}
}

func TestTheMoveRefusesARebuildableStore(t *testing.T) {
	t.Parallel()
	registry, _, store := rebuildableBed(t)
	evidence := filepath.Join(filepath.Dir(registry.Dir), "evidence")
	if err := os.MkdirAll(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := MoveBundle(context.Background(), store, MoveRules{SegmentDir: filepath.Join(evidence, "suite-failures", "107e72c67539"),
		Blobs: BlobStore{Dir: filepath.Join(filepath.Dir(registry.Dir), "blobs")}, Referrer: "r", Stage: "01K2Z7Q3M8XW1V0P9D4J6S5R2T",
		Sync: Syncer{}, Stores: registry})
	if err != nil || !strings.Contains(result.Kept, "rebuildable") || result.Moved {
		t.Fatalf("the move keeps a rebuildable store out of the evidence root: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(evidence, "suite-failures", "107e72c67539", filepath.Base(store))); !os.IsNotExist(err) {
		t.Fatalf("nothing entered the evidence root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store, "bin", "metasystem")); err != nil {
		t.Fatalf("the store is untouched: %v", err)
	}
}

// A recorded candidate engine in a bundle is kept as bytes, and a second
// bundle holding the same engine shares its blob (DL4C-07). This witnesses
// the distiller U5c built; U5i adds no behaviour here.
func TestAnEngineInTwoBundlesIsBytesInOneBlob(t *testing.T) {
	t.Parallel()
	base := realDir(t)
	blobs := BlobStore{Dir: filepath.Join(base, "home", "metasystem-evidence", ".blobs")}
	engine := append([]byte("\xcf\xfa\xed\xfe"), bytes.Repeat([]byte("engine"), testMiB/4)...)
	var bundles []string
	for _, name := range []string{"20260926T101010Z-detached-a-1", "20260926T111010Z-detached-b-2"} {
		bundle := filepath.Join(base, "checkout", "suite-failures", name)
		writeBedFile(t, filepath.Join(bundle, "candidate-engines", "v2", "metasystem"), engine)
		if err := os.Chmod(filepath.Join(bundle, "candidate-engines", "v2", "metasystem"), 0o755); err != nil {
			t.Fatal(err)
		}
		rules := DistillRules{CompressAbove: testMiB, Blobs: blobs, Referrer: "107e72c67539-" + name, Installation: filepath.Join(base, "checkout"),
			Segment: "107e72c67539", Stage: "01K2Z7Q3M8XW1V0P9D4J6S5R2" + string(rune('A'+len(bundles)))}
		if _, err := Distill(context.Background(), bundle, rules, placementNow.Add(48*time.Hour)); err != nil {
			t.Fatal(err)
		}
		bundles = append(bundles, bundle)
	}
	entries, err := os.ReadDir(blobs.Dir)
	if err != nil {
		t.Fatal(err)
	}
	blobCount := 0
	for _, entry := range entries {
		if entry.Type().IsRegular() && len(entry.Name()) == 64 {
			blobCount++
		}
	}
	if blobCount != 1 {
		t.Fatalf("the same engine in two bundles is one blob, found %d", blobCount)
	}
	for _, bundle := range bundles {
		restored := restoreBundle(t, bundle, blobs)
		if restored["candidate-engines/v2/metasystem"] != string(engine) {
			t.Fatalf("%s: the engine is restorable byte for byte", bundle)
		}
	}
}

func TestMisplacedTreesRaiseTheDiskRole(t *testing.T) {
	t.Parallel()
	report := Report{Misplaced: []Line{{Class: PlacementCache, Path: "/evidence/gocache-x", Reason: "a directory named like a cache, 3 GiB", Command: "rm -rf -- '/evidence/gocache-x'"}}}
	health := healthOf(report)
	if health.Status != HealthAttention || !strings.Contains(health.Reason, "/evidence/gocache-x") {
		t.Fatalf("a cache under an evidence root raises the disk role: %+v", health)
	}
	if text := strings.Join(report.Lines(), "\n"); !strings.Contains(text, PlacementCache) || !strings.Contains(text, "rm -rf -- '/evidence/gocache-x'") {
		t.Fatalf("the report names the tree and what removes it:\n%s", text)
	}
}
