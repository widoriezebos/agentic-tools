package diskstore

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// Round D3 N4: a unit read's findings store is released when its unit
// record is released (its named entry gone), never while the unit is
// recorded or while a build or review holds the unit's name.
func TestAUnitsFindingsStoreEndsWithItsUnit(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	unitRoot := filepath.Join(root, "home", "unit")
	named := filepath.Join(unitRoot, ".named")
	for _, dir := range []string{named, filepath.Join(root, "tmp")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	registry := MachineRegistry(filepath.Join(root, "home"))
	path, err := createTempStore(filepath.Join(root, "tmp"), registry, "metasystem-unit-read.key10", UnitReadFindingsClass,
		Owner{Kind: OwnerUnit, Ref: "key10"}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "read-findings.md"), []byte("findings"), 0o600); err != nil {
		t.Fatal(err)
	}
	pass := func() Report {
		report, err := RunPass(context.Background(), PassOptions{Kind: "machine", Name: "machine", Registry: registry, Mode: ModeApply,
			Now: testNow, Clock: func() time.Time { return testNow },
			Classes: []Class{RegisteredStores{Registry: registry, Proofs: map[OwnerKind]OwnerProof{OwnerUnit: UnitFindingsProof{UnitRoot: unitRoot}}}}})
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	entry := filepath.Join(named, "key10.json")
	if err := os.WriteFile(entry, []byte(`{"run":"r1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if report := pass(); len(report.Actions) != 0 {
		t.Fatalf("a recorded unit's findings are released: %+v", report)
	}
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	held, err := lock.File(filepath.Join(named, "key10.lock"), 0o600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	if report := pass(); len(report.Actions) != 0 || len(report.Pending) != 1 {
		t.Fatalf("a held named lock keeps it pending: %+v", report)
	}
	_ = held.Release()
	if report := pass(); len(report.Actions) != 1 {
		t.Fatalf("a released unit's findings go: %+v", report)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the findings store survived: %v", err)
	}
}
