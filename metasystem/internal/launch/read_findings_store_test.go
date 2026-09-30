package launch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// Round D3 (batch 28): a unit read's findings store the launcher prepares
// between rounds stays the recorded store: emptied in place (same
// directory, marker kept), made again through the registry when a cleaner
// removed it, and released by its unit's proof when the unit ends.
func TestAFindingsStoreCleanedBetweenRoundsIsStillReleasedWhenItsUnitEnds(t *testing.T) {
	t.Parallel()
	key := "d3findings" + filepath.Base(t.TempDir())
	owner := diskstore.Owner{Kind: diskstore.OwnerUnit, Ref: key}
	dir, err := diskstore.CreateTempStore(diskstore.UnitReadFindingsName(key), diskstore.UnitReadFindingsClass, owner)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "read-findings.md")
	if err := os.WriteFile(output, []byte("round one"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareReadOutputDirectories([]string{output}); err != nil {
		t.Fatal(err)
	}
	after, err := os.Lstat(dir)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("round two's findings directory is not the same directory: %v", err)
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatalf("round one's findings survived the cleaning: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, diskstore.MarkerName)); err != nil {
		t.Fatalf("the store's marker went with the cleaning: %v", err)
	}
	// A temporary-directory cleaner removes it; the next round makes it
	// again through the registry.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := prepareReadOutputDirectories([]string{output}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(dir); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("round three's findings directory = %v, %v", info, err)
	}
	path, err := registry.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Dir(path)
	machine := diskstore.MachineRegistry(home)
	unitRoot := filepath.Join(t.TempDir(), "unit")
	if err := os.MkdirAll(filepath.Join(unitRoot, ".named"), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "machine", Name: "machine", Registry: machine, Mode: diskstore.ModeApply,
		Now: now, Clock: func() time.Time { return now }, LockPath: filepath.Join(t.TempDir(), ".sweep.flock"),
		Classes: []diskstore.Class{diskstore.RegisteredStores{Registry: machine, Proofs: map[diskstore.OwnerKind]diskstore.OwnerProof{
			diskstore.OwnerUnit: diskstore.UnitFindingsProof{UnitRoot: unitRoot}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("the ended unit's findings store was not released: %v; pending %+v kept %+v", err, report.Pending, report.Kept)
	}
}
