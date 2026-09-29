package main

import (
	"context"
	"crypto/rand"
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
	if code != 0 || !strings.Contains(out, "evidence of "+bed.gitRoot) || !strings.Contains(out, "1 item(s): 1 live, 0 compacted") {
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
