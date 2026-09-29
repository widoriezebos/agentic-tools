package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/evidence"
)

// evidenceVerbBed is a disk bed whose checkout has an evidence root with one
// old, closed chain of a concluded goal, and a person's environment over it
// with a fake ledger observer and no git.
type evidenceVerbBed struct {
	*diskBed
	root, gitRoot, chain, export string
	fetches                      *int
}

const evidenceIdentity = "01J9LEDGER0000000000000000"

func newEvidenceVerbBed(t *testing.T) *evidenceVerbBed {
	t.Helper()
	disk := newDiskBed(t)
	bed := &evidenceVerbBed{diskBed: disk, root: filepath.Join(disk.root, "evidence"), gitRoot: disk.root, export: filepath.Join(t.TempDir(), "exports"), fetches: new(int)}
	segment := diskstore.Segment(bed.gitRoot)
	bed.chain = filepath.Join(bed.root, "agents", segment, "old-chain")
	files := map[string]string{
		"jobs/old-chain.json": `{"jobId":"old-chain","round":1,"role":"implementer","status":"completed","chainClosed":true,"goalId":"g-done","endedAt":"2026-01-01T00:00:00Z"}`,
		"jobs/old-chain.log":  strings.Repeat("a log line\n", 2000),
		"brief.md":            "the brief",
	}
	for rel, data := range files {
		path := filepath.Join(bed.chain, filepath.FromSlash(rel))
		helmMust(t, os.MkdirAll(filepath.Dir(path), 0o755), os.WriteFile(path, []byte(data), 0o644))
	}
	facts := diskstore.CheckoutFacts{GitRoot: bed.gitRoot, Installation: disk.inst, RootCommit: "e83c5163316f89bfbde7d9ab23ca2e25604af290", LedgerIdentity: evidenceIdentity}
	if _, err := evidence.RecordSegmentIndex(bed.root, facts, diskNow, diskstore.Syncer{}); err != nil {
		t.Fatal(err)
	}
	this := evidence.HostCheckout{Installation: disk.inst, Facts: facts,
		Settings: diskstore.Settings{Values: map[string]string{}, EvidenceRoot: config.EvidenceRoot{Path: bed.root, Origin: "conf-local"}}}
	citations := &evidence.Citations{Dir: filepath.Join(disk.home, "stores", "citations"), Now: diskNow, Roots: func() ([]string, error) { return nil, nil }}
	if _, err := citations.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	fetches := bed.fetches
	disk.owners.disk.evidenceEnv = func(_ context.Context, _ string, by string) (evidence.Env, error) {
		return evidence.Env{UserHome: filepath.Join(disk.root, "user"), HomeStateRoot: disk.home, Checkouts: []evidence.HostCheckout{this}, This: this,
			Now: diskNow, Entropy: rand.Reader, By: by, Citations: citations, Blobs: diskstore.BlobStore{Dir: filepath.Join(disk.root, "user", "metasystem-evidence", ".blobs")},
			Observe: func(_ context.Context, _ string, fetch bool) (evidence.LedgerView, error) {
				if fetch {
					*fetches++
				}
				return evidence.LedgerView{Tip: "9498700a9", Identity: evidenceIdentity, States: map[string]string{"g-done": evidence.GoalDone}}, nil
			},
			Locks: func(string, []string, time.Duration) (func(), string, error) { return func() {}, "", nil }}, nil
	}
	return bed
}

func TestEvidenceIsAPublicObjectWithThreeActions(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"show", "export", "dispose"} {
		code, stdout, _ := routeWith(nil, "evidence", action, "--help")
		if code != 0 || !strings.Contains(stdout, "metasystem evidence "+action) {
			t.Errorf("evidence %s help = %d %q", action, code, stdout)
		}
	}
}

func TestEvidenceShowIsAShortSummaryThatChangesNothing(t *testing.T) {
	t.Parallel()
	bed := newEvidenceVerbBed(t)
	before := evidenceSnapshot(t, bed.root)
	code, out := bed.run("evidence", "show")
	if code != 0 || !strings.Contains(out, "evidence of "+bed.gitRoot) || !strings.Contains(out, "1 item(s): ") {
		t.Fatalf("evidence show = %d:\n%s", code, out)
	}
	if strings.Contains(out, "old-chain,") || len(strings.Split(strings.TrimSpace(out), "\n")) > 6 {
		t.Fatalf("the default is a short summary; --verbose lists items:\n%s", out)
	}
	code, out = bed.run("evidence", "show", filepath.Join(bed.chain, "jobs", "old-chain.log"))
	if code != 0 || !strings.Contains(out, "a live file") {
		t.Fatalf("show PATH = %d:\n%s", code, out)
	}
	if after := evidenceSnapshot(t, bed.root); after != before {
		t.Fatal("evidence show changes nothing")
	}
}

func evidenceSnapshot(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			data, _ := os.ReadFile(path)
			lines = append(lines, path+"="+string(data))
		}
		return nil
	})
	return strings.Join(lines, "\n")
}

func TestEvidenceExportWithoutADestinationNamesBothWays(t *testing.T) {
	t.Parallel()
	bed := newEvidenceVerbBed(t)
	code, out := bed.run("evidence", "export", "old-chain")
	if code == 0 || !strings.Contains(out, "--to DIR") || !strings.Contains(out, "evidence.export-dir") {
		t.Fatalf("no --to and no evidence.export-dir = %d:\n%s", code, out)
	}
	code, out = bed.run("evidence", "export", "old-chain", "--to", filepath.Join(bed.root, "exports"))
	if code == 0 || !strings.Contains(out, "an export must live outside every evidence root and checkout") {
		t.Fatalf("a destination inside the evidence root = %d:\n%s", code, out)
	}
}

// witnessEvidenceExportRepeat: an export twice reports already exported,
// verified, and writes nothing the second time.
func witnessEvidenceExportRepeat(t *testing.T, bed *evidenceVerbBed) {
	t.Helper()
	code, out := bed.run("evidence", "export", "old-chain", "--to", bed.export)
	if code != 0 || !strings.Contains(out, "exported 1 of 1 item(s)") {
		t.Fatalf("first export = %d:\n%s", code, out)
	}
	before := evidenceSnapshot(t, bed.export)
	code, out = bed.run("evidence", "export", "old-chain", "--to", bed.export)
	if code != 0 || !strings.Contains(out, "1 already exported and verified") {
		t.Fatalf("second export = %d:\n%s", code, out)
	}
	if after := evidenceSnapshot(t, bed.export); after != before {
		t.Fatal("a repeat export writes nothing")
	}
}

// witnessEvidenceDisposeRepeat: a person's plan executed twice removes the
// item once and writes nothing the second time; an agent may preview but
// not execute.
func witnessEvidenceDisposeRepeat(t *testing.T, bed *evidenceVerbBed) {
	t.Helper()
	code, out := bed.run("evidence", "dispose", "old-chain", "--export", bed.export, "--preview")
	if code != 0 || !strings.Contains(out, "preview: nothing was changed") || !strings.Contains(out, "1 clear") || *bed.fetches != 0 {
		t.Fatalf("preview = %d (fetches %d):\n%s", code, *bed.fetches, out)
	}
	if _, err := os.Stat(bed.chain); err != nil {
		t.Fatal("the preview removed nothing")
	}
	bed.person = errors.New("not the enrolled terminal")
	if code, out := bed.run("evidence", "dispose"); code == 0 || !strings.Contains(out, "metasystem system enroll --name NAME") {
		t.Fatalf("an agent does not execute a plan = %d:\n%s", code, out)
	}
	bed.person = nil
	code, out = bed.run("evidence", "dispose")
	if code != 0 || !strings.Contains(out, "1 of 1 item(s) disposed") {
		t.Fatalf("execute = %d:\n%s", code, out)
	}
	if _, err := os.Stat(bed.chain); !os.IsNotExist(err) {
		t.Fatalf("the item is removed: %v", err)
	}
	ledger := filepath.Join(bed.root, "disposals", diskstore.Segment(bed.gitRoot)+".jsonl")
	receipts, _ := diskstore.ReadReceipts(ledger)
	if len(receipts) != 1 || receipts[0].Export == nil || receipts[0].Rule != diskstore.RulePerson || receipts[0].By != "Wido" {
		t.Fatalf("one person's receipt with the export: %+v", receipts)
	}
	code, out = bed.run("evidence", "dispose")
	if code != 0 || !strings.Contains(out, "1 already disposed") {
		t.Fatalf("a repeat = %d:\n%s", code, out)
	}
	if again, _ := diskstore.ReadReceipts(ledger); len(again) != 1 {
		t.Fatal("a repeat writes no second receipt")
	}
	code, out = bed.run("evidence", "show", filepath.Join(bed.chain, "jobs", "old-chain.log"))
	if code != 0 || !strings.Contains(out, "removed on") || !strings.Contains(out, "exported to") {
		t.Fatalf("the pointer answers with the archive = %d:\n%s", code, out)
	}
}

func TestEvidenceExportTwiceSecondWritesNothing(t *testing.T) {
	t.Parallel()
	witnessEvidenceExportRepeat(t, newEvidenceVerbBed(t))
}

func TestEvidenceDisposeFromAPreviewedPlanOnceAndOnlyByAPerson(t *testing.T) {
	t.Parallel()
	witnessEvidenceDisposeRepeat(t, newEvidenceVerbBed(t))
}

// Round B2, F-8: execution without --plan takes only this terminal
// session's newest preview; another session's preview runs only when a
// person names it.
func TestEvidenceDisposeWithoutAPlanTakesOnlyThisSessionsPreview(t *testing.T) {
	t.Parallel()
	bed := newEvidenceVerbBed(t)
	code, out := bed.run("evidence", "dispose", "old-chain", "--preview")
	if code != 0 {
		t.Fatalf("preview = %d:\n%s", code, out)
	}
	plans, _ := filepath.Glob(filepath.Join(bed.home, "stores", "plans", "*.json"))
	if len(plans) != 1 {
		t.Fatalf("one plan: %v", plans)
	}
	data, err := os.ReadFile(plans[0])
	if err != nil {
		t.Fatal(err)
	}
	var plan map[string]any
	if err := json.Unmarshal(data, &plan); err != nil || plan["session"] == "" || plan["session"] == nil {
		t.Fatalf("the plan names its session: %v %v", plan["session"], err)
	}
	plan["session"] = "another-terminal"
	data, _ = json.Marshal(plan)
	helmMust(t, os.WriteFile(plans[0], data, 0o644))
	code, out = bed.run("evidence", "dispose")
	if code == 0 || !strings.Contains(out, "--preview") || !strings.Contains(out, "--plan ID") {
		t.Fatalf("another session's preview is not executed by default = %d:\n%s", code, out)
	}
	if _, err := os.Stat(bed.chain); err != nil {
		t.Fatal("nothing was removed")
	}
	id := strings.TrimSuffix(filepath.Base(plans[0]), ".json")
	if code, out := bed.run("evidence", "dispose", "--plan", id); code != 0 || !strings.Contains(out, "1 of 1 item(s) disposed") {
		t.Fatalf("a named plan runs = %d:\n%s", code, out)
	}
}

// Round B2-2, R1: --all --verbose lists every enumerated item; a word that
// is not one of them is refused, pointing at that listing.
func TestEvidenceShowAllVerboseListsTheItemsDisposeAccepts(t *testing.T) {
	t.Parallel()
	bed := newEvidenceVerbBed(t)
	code, out := bed.run("evidence", "show", "--all", "--verbose")
	if code != 0 || !strings.Contains(out, "chain "+bed.chain) {
		t.Fatalf("--all --verbose = %d:\n%s", code, out)
	}
	code, out = bed.run("evidence", "dispose", filepath.Join(bed.root, "AGENTS"), "--preview")
	if code == 0 || !strings.Contains(out, "not an item; metasystem evidence show --verbose") {
		t.Fatalf("a structure directory in another case is refused = %d:\n%s", code, out)
	}
}

// openEvidenceDisposal leaves a person's removal of the bed's chain as a
// crash after its set-aside leaves it.
func openEvidenceDisposal(t *testing.T, bed *evidenceVerbBed) {
	t.Helper()
	files, err := diskstore.Inventory(context.Background(), bed.chain)
	if err != nil {
		t.Fatal(err)
	}
	tombstone := diskstore.Tombstone{Schema: diskstore.TombstoneSchema, Item: "old-chain", Kind: diskstore.KindChain, Segment: diskstore.Segment(bed.gitRoot),
		Files: files, History: []diskstore.HistoryEntry{}, InventoryDigest: diskstore.InventoryDigest(files), Step: diskstore.StepRemove,
		Rule: diskstore.RulePerson, By: "Wido", At: diskNow, Receipt: "01RECEIPTSHOW", State: diskstore.StateBegun, Disposing: "old-chain.disposing-01S"}
	data, _ := json.Marshal(tombstone)
	helmMust(t, os.WriteFile(diskstore.RemovedTombstonePath(bed.chain), data, 0o644), os.Rename(bed.chain, bed.chain+".disposing-01S"))
}

// Round B2-3 amendment: show shows. On a tree with an open disposal, show
// and show --all change no byte and name the verb that settles it.
func TestEvidenceShowReportsAnOpenDisposalAndChangesNothing(t *testing.T) {
	t.Parallel()
	bed := newEvidenceVerbBed(t)
	openEvidenceDisposal(t, bed)
	before := evidenceSnapshot(t, bed.root)
	for _, args := range [][]string{{"evidence", "show"}, {"evidence", "show", "--all"}} {
		code, out := bed.run(args...)
		if code != 0 || !strings.Contains(out, "an interrupted removal of "+bed.chain+" is open: metasystem evidence dispose settles it (rolls it back), then preview again") {
			t.Fatalf("%v reports the open disposal = %d:\n%s", args, code, out)
		}
		if after := evidenceSnapshot(t, bed.root); after != before {
			t.Fatalf("%v changes no byte", args)
		}
	}
}

// Round B2-3 amendment: evidence dispose, in any form, settles an open
// disposal before anything else: a preview rolls it back, then plans.
func TestEvidenceDisposeSettlesAnOpenDisposalFirst(t *testing.T) {
	t.Parallel()
	bed := newEvidenceVerbBed(t)
	openEvidenceDisposal(t, bed)
	code, out := bed.run("evidence", "dispose", "old-chain", "--preview")
	if code != 0 || !strings.Contains(out, "rolled back, the item is back") || !strings.Contains(out, "1 clear") {
		t.Fatalf("the preview settles first, then plans = %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(bed.chain, "jobs", "old-chain.log")); err != nil {
		t.Fatalf("the item is back: %v", err)
	}
}

// Round B2-3, rule 3: an entry of the root outside every segment is listed
// as not managed and refused by dispose.
func TestEvidenceShowListsNotManagedEntriesAndDisposeRefusesThem(t *testing.T) {
	t.Parallel()
	bed := newEvidenceVerbBed(t)
	legacy := filepath.Join(bed.root, "agents", "old-layout-chain")
	helmMust(t, os.MkdirAll(filepath.Join(legacy, "jobs"), 0o755), os.WriteFile(filepath.Join(legacy, "jobs", "x.json"), []byte("{}"), 0o644))
	code, out := bed.run("evidence", "show", "--verbose")
	if code != 0 || !strings.Contains(out, legacy+", ") || !strings.Contains(out, "not managed: remove by hand if unneeded") {
		t.Fatalf("show names the entry = %d:\n%s", code, out)
	}
	code, out = bed.run("evidence", "dispose", legacy, "--preview")
	if code == 0 || !strings.Contains(out, "not managed: remove by hand if unneeded") {
		t.Fatalf("dispose refuses it = %d:\n%s", code, out)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal("nothing was removed")
	}
}
