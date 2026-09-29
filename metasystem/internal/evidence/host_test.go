package evidence

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// hostBed is two armed checkouts on one host, each with its own evidence
// root, and a user home.
type hostBed struct {
	one, two  *boundBed
	userHome  string
	homeState string
}

func newHostBed(t *testing.T) hostBed {
	t.Helper()
	one, two := newBoundBed(t), newBoundBed(t)
	userHome := filepath.Join(filepath.Dir(one.root), "user")
	if err := os.MkdirAll(userHome, 0o755); err != nil {
		t.Fatal(err)
	}
	return hostBed{one: one, two: two, userHome: userHome, homeState: one.home}
}

func (bed hostBed) checkouts() []HostCheckout {
	var checkouts []HostCheckout
	for _, b := range []*boundBed{bed.one, bed.two} {
		settings := diskstore.Settings{Values: map[string]string{config.DiskEvidenceSegmentCapKey: "10", config.DiskEvidenceAgeFloorKey: "90"},
			EvidenceRoot: config.EvidenceRoot{Path: b.root, Origin: "conf-local"}}
		checkouts = append(checkouts, HostCheckout{Installation: b.installation, Facts: b.segment.Context.Facts, Settings: settings})
	}
	return checkouts
}

func (bed hostBed) class(capBytes int64) *BoundClass {
	citations := &Citations{Dir: filepath.Join(bed.homeState, "stores", "citations"), Now: boundNow,
		Roots: func() ([]string, error) { return nil, nil }}
	return &BoundClass{UserHome: bed.userHome, HomeStateRoot: bed.homeState, Checkouts: bed.checkouts(), MachineCap: capBytes,
		BlobGrace: 24 * time.Hour, AgeFloor: 90 * 24 * time.Hour, Citations: citations, By: "steward m1e",
		Bound: Bound{Now: boundNow, Blobs: diskstore.BlobStore{Dir: diskstore.BlobStoreDir(bed.userHome)}}}
}

func (bed hostBed) run(t *testing.T, class *BoundClass, mode diskstore.Mode) diskstore.Report {
	t.Helper()
	report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "machine", Name: "machine", Registry: diskstore.MachineRegistry(bed.homeState),
		LockPath: filepath.Join(bed.homeState, "stores", ".sweep.flock"), ReportPath: diskstore.MachineReportPath(bed.homeState),
		PlanDir: filepath.Join(bed.homeState, "stores", "plans"), Mode: mode, Now: boundNow, Clock: func() time.Time { return boundNow },
		Entropy: rand.Reader, Classes: []diskstore.Class{class}})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// Round B2-3, rule 1: past the machine cap and past a segment's cap the
// machine pass only reports, with the command pair; it changes nothing but
// its bookkeeping and writes no receipt.
func TestTheMachinePassOnlyReportsPastEitherCapAndRemovesNothing(t *testing.T) {
	t.Parallel()
	bed := newHostBed(t)
	older := bed.two.chain(t, "older", 400, 400, "")
	newer := bed.one.chain(t, "newer", 300, 400, "")
	bed.run(t, bed.class(1<<40), diskstore.ModeApply)
	before := snapshot(t, filepath.Dir(bed.one.root))
	before2 := snapshot(t, filepath.Dir(bed.two.root))
	class := bed.class(1)
	class.SegmentSettings = func(Segment) (PassSettings, error) { return settingsOf(1), nil }
	report := bed.run(t, class, diskstore.ModeApply)
	text := strings.Join(report.Lines(), "\n")
	for _, want := range []string{"machine cap: the host's evidence holds", "over the bound: " + bed.one.segment.Git, "over the bound: " + bed.two.segment.Git,
		"metasystem evidence dispose --over-bound --export DIR --preview"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the report names %q:\n%s", want, text)
		}
	}
	for _, dir := range []string{older, newer} {
		if _, err := os.Stat(filepath.Join(dir, "jobs", filepath.Base(dir)+".log")); err != nil {
			t.Fatalf("nothing is dropped or removed: %v", err)
		}
	}
	for _, b := range []*boundBed{bed.one, bed.two} {
		if len(receipts(t, b.segment)) != 0 {
			t.Fatal("the machine writes no receipt")
		}
	}
	for _, pair := range [][2]map[string]string{{before, snapshot(t, filepath.Dir(bed.one.root))}, {before2, snapshot(t, filepath.Dir(bed.two.root))}} {
		for path, content := range pair[0] {
			if strings.Contains(path, "/evidence/") && pair[1][path] != content {
				t.Fatalf("an evidence file changed: %s", path)
			}
		}
	}
}

func TestEveryRootIsNamedWithItsOwnerAndOrphansAreUntouched(t *testing.T) {
	t.Parallel()
	bed := newHostBed(t)
	evidenceParent := diskstore.EvidenceParent(bed.userHome)
	retired := filepath.Join(evidenceParent, "retired-root")
	unclaimed := filepath.Join(evidenceParent, "unclaimed-root")
	custom := filepath.Join(filepath.Dir(bed.userHome), "volumes", "evidence", "project")
	orphan := filepath.Join(bed.one.root, "agents", "0123456789ab", "orphan-chain")
	unsegmented := filepath.Join(bed.one.root, "hand-named-20260901")
	for _, dir := range []string{retired, unclaimed, filepath.Join(custom, "agents"), orphan, unsegmented} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(retired, "RETIRED.json"), []byte(`{"schemaVersion":1,"retiredAt":"2026-09-01T00:00:00Z","checkouts":["/gone/checkout"],"successor":"per-checkout default","rule":"evidence-root-default"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphan, "payload"), make([]byte, 100*kib), 0o644); err != nil {
		t.Fatal(err)
	}
	// A custom root outside $HOME is found through the registry its arming
	// wrote, after its checkout is gone.
	if err := os.MkdirAll(filepath.Join(bed.homeState, "stores"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RootsRegistryPath(bed.homeState), []byte(`[{"root":"`+custom+`","checkout":"/gone/checkout","gitRoot":"/gone/checkout","installation":"/gone/checkout/metasystem","firstSeen":"2026-09-01T00:00:00Z","lastSeen":"2026-09-02T00:00:00Z"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	report := bed.run(t, bed.class(1), diskstore.ModeApply)
	text := strings.Join(report.Lines(), "\n")
	for _, want := range []string{bed.one.root + ": root of " + bed.one.gitRoot, retired + ": retired root of /gone/checkout, successor per-checkout default",
		unclaimed + ": unclaimed root", custom + ": unclaimed root", "1 entry not managed", "orphan segment 0123456789ab"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the report names %q:\n%s", want, text)
		}
	}
	if _, err := os.Stat(filepath.Join(orphan, "payload")); err != nil {
		t.Fatalf("an orphan segment is untouched: %v", err)
	}
	entries, err := ReadRootsRegistry(RootsRegistryPath(bed.homeState))
	if err != nil || len(entries) != 3 {
		t.Fatalf("the pass records both armed roots and keeps the custom one: %+v %v", entries, err)
	}
	if _, err := ReadSegmentIndex(bed.one.root, bed.one.segment.Git); err != nil {
		t.Fatalf("the pass records the segment index: %v", err)
	}
}

// A preview plans nothing and changes nothing but its plan (R15).
func TestAPreviewOfTheMachinePassChangesNothing(t *testing.T) {
	t.Parallel()
	bed := newHostBed(t)
	bed.one.chain(t, "old", 300, 400, "")
	bed.run(t, bed.class(1<<40), diskstore.ModeApply)
	class := bed.class(1 << 40)
	class.SegmentSettings = func(Segment) (PassSettings, error) { return settingsOf(1), nil }
	before := snapshot(t, filepath.Dir(bed.one.root))
	report := bed.run(t, class, diskstore.ModePreview)
	after := snapshot(t, filepath.Dir(bed.one.root))
	delete(after, strings.TrimPrefix(filepath.Join(bed.homeState, "stores", "plans", report.Plan+".json"), filepath.Dir(bed.one.root)))
	if !equalSnapshots(before, after) || len(report.Planned) != 0 {
		t.Fatalf("a preview changes nothing but its plan and plans no step: %+v", report.Planned)
	}
	if !strings.Contains(strings.Join(report.Lines(), "\n"), "over the bound: "+bed.one.segment.Git) {
		t.Fatalf("it reports the segment over its bound:\n%s", strings.Join(report.Lines(), "\n"))
	}
}
