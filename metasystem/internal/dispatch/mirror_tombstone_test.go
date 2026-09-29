package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

func writeTombstone(t *testing.T, path string, tombstone diskstore.Tombstone) {
	t.Helper()
	tombstone.Schema, tombstone.State = diskstore.TombstoneSchema, diskstore.StateDone
	data := []byte(`{"schema":"` + tombstone.Schema + `","item":"` + tombstone.Item + `","kind":"chain","history":[],"inventoryDigest":"x","step":"` +
		tombstone.Step + `","rule":"` + tombstone.Rule + `","by":"wido","at":"2026-12-29T02:00:00Z","receipt":"` + tombstone.Receipt + `","bytesBefore":1,"bytesAfter":0,"state":"done"}`)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// A mirror into a removed chain lands nothing and says so; into a
// compacted one it lands only the kept set (engine-owns-disk-lifetimes
// 3.12, DL4D-13).
func TestMirrorHonoursTheDisposalTombstone(t *testing.T) {
	t.Parallel()
	repo, evidence, job := mirrorFixture(t)
	destination := filepath.Join(evidence, "agents", CheckoutSegment(repo), job)
	writeTombstone(t, diskstore.RemovedTombstonePath(destination), diskstore.Tombstone{Item: job, Step: diskstore.StepRemove, Rule: diskstore.RulePerson, Receipt: "01RECEIPT"})
	result := filepath.Join(t.TempDir(), "result.json")
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("a removed chain is never re-created by a mirror: %v", err)
	}
	if got := asString(readJSONFile(t, result)["disposed"]); !strings.Contains(got, "receipt 01RECEIPT") {
		t.Fatalf("the result reports the tombstone: %q", got)
	}
	if err := os.Remove(diskstore.RemovedTombstonePath(destination)); err != nil {
		t.Fatal(err)
	}
	writeTombstone(t, filepath.Join(destination, diskstore.CompactTombstoneName), diskstore.Tombstone{Item: job, Step: diskstore.StepCompact, Rule: diskstore.RuleBound, Receipt: "01RECEIPTC"})
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{"jobs/" + job + ".json", "brief.md"} {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(kept))); err != nil {
			t.Fatalf("the kept set lands: %s %v", kept, err)
		}
	}
	for _, dropped := range []string{"rounds/1/diff.patch", "capabilities/snap.json"} {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(dropped))); !os.IsNotExist(err) {
			t.Fatalf("a dropped member never lands again: %s %v", dropped, err)
		}
	}
}
