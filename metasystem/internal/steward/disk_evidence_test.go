package steward

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// The checkout pass ages its suite-failure bundles (design
// engine-owns-disk-lifetimes 3.3 trigger 3, 3.5): a bundle idle a day is
// distilled with the host's blob store, and a distilled bundle seven days
// old by its creation stamp moves into the git root's segment of the
// checkout's evidence root. The fixture names its own evidence root and
// home, so no real root is touched.
func TestCheckoutPassDistilsThenMovesSuiteFailureBundles(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	evidence := filepath.Join(bed.root, "evidence")
	conf := "metasystem.template=true\ndisk.floor-gib=1\nevidence.root=" + evidence + "\n"
	if err := os.WriteFile(filepath.Join(bed.inst, "metasystem.conf"), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(bed.inst, "artifacts", "agents", "suite-failures", "20260927T010000Z-detached-group-standalone-7-1")
	if err := os.MkdirAll(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	log := bytes.Repeat([]byte("a failing run's log\n"), (2<<20)/20)
	if err := os.WriteFile(filepath.Join(bundle, "run.log"), log, 0o600); err != nil {
		t.Fatal(err)
	}
	old := staleNow.Add(-30 * time.Hour)
	for _, path := range []string{filepath.Join(bundle, "run.log"), bundle} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	pass := bed.pass(diskstore.ModeApply, []string{bed.inst}, false)
	pass.UserHome = filepath.Join(bed.root, "user")
	pass.SkipMachine = true
	pass.SuiteFailureSeams = func(class *diskstore.SuiteFailures) {
		class.Known = nil
		class.Facts = func(context.Context) (diskstore.CheckoutFacts, error) {
			return diskstore.CheckoutFacts{RootCommit: "e83c5163316f89bfbde7d9ab23ca2e25604af290", LedgerIdentity: "01J9LEDGER0000000000000000"}, nil
		}
	}
	result, err := SweepDiskStores(context.Background(), bed.inst, pass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "run.log.gz")); err != nil {
		t.Fatalf("the idle bundle must be distilled: %v\n%v", err, result.Checkout.Lines())
	}
	if _, err := os.Stat(filepath.Join(bundle, "run.log")); !os.IsNotExist(err) {
		t.Fatalf("the original goes once its gzip verified: %v", err)
	}
	owner, err := diskstore.ReadBundleOwner(bundle)
	if err != nil || owner.Goal != diskstore.GoalUnknown || owner.LedgerIdentity != "01J9LEDGER0000000000000000" {
		t.Fatalf("a legacy bundle gets its owner and the checkout's facts: %+v %v", owner, err)
	}
	pass.Now, pass.Clock = staleNow.Add(8*24*time.Hour), func() time.Time { return staleNow.Add(8 * 24 * time.Hour) }
	if _, err := SweepDiskStores(context.Background(), bed.inst, pass); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(evidence, "suite-failures", diskstore.Segment(filepath.Join(bed.root, "repo")), filepath.Base(bundle))
	if _, err := os.Stat(filepath.Join(moved, "run.log.gz")); err != nil {
		t.Fatalf("the distilled bundle must be in its segment: %v", err)
	}
	if _, err := os.Stat(bundle); !os.IsNotExist(err) {
		t.Fatalf("the source goes after the verified move: %v", err)
	}
}
