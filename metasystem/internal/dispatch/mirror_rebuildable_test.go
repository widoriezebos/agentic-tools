package dispatch

// U5i (engine-owns-disk-lifetimes Part B, 3.12 placement rule 2, R22): a
// rebuildable store never enters an evidence root, so the mirror refuses a
// chain whose payload lies in one, and holds when the store registry cannot
// be read (fail-closed rule 1).

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

func registerRebuildable(t *testing.T, repo, path string) diskstore.Record {
	t.Helper()
	registry := diskstore.CheckoutRegistry(repo)
	record, err := registry.Register(diskstore.Registration{Path: path, Class: "candidate engines", Owner: diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: "G"},
		Lifetime: diskstore.LifetimeRebuildable, RebuildFrom: &diskstore.RebuildFrom{Commit: "e83c5163316f89bfbde7d9ab23ca2e25604af290", Command: "go build ./cmd/metasystem"},
		CapKind: diskstore.CapTarget}, time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if record, err = registry.Accept(record.ID); err != nil {
		t.Fatal(err)
	}
	return record
}

func TestTheMirrorRefusesARebuildableStore(t *testing.T) {
	t.Parallel()
	repo, evidence, job := mirrorFixture(t)
	record := registerRebuildable(t, repo, filepath.Join(repo, "artifacts", "agents", job, "rounds"))
	err := Mirror(repo, repo, evidence, job, job, filepath.Join(t.TempDir(), "result.json"))
	if err == nil || !strings.Contains(err.Error(), record.ID) || !strings.Contains(err.Error(), "rebuildable") {
		t.Fatalf("the mirror refuses naming the rebuildable store: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(evidence, "agents", CheckoutSegment(repo), job, "rounds")); !os.IsNotExist(statErr) {
		t.Fatalf("nothing of the store entered the evidence root: %v", statErr)
	}
}

func TestTheMirrorHoldsWhenTheStoreRegistryCannotBeRead(t *testing.T) {
	t.Parallel()
	repo, evidence, job := mirrorFixture(t)
	registry := diskstore.CheckoutRegistry(repo)
	if err := os.MkdirAll(registry.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registry.RecordPath("01K2Z7Q3M8XW1V0P9D4J6S5R2T"), []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Mirror(repo, repo, evidence, job, job, filepath.Join(t.TempDir(), "result.json")); err == nil {
		t.Fatal("a mirror that cannot tell whether a source is rebuildable holds")
	}
}
