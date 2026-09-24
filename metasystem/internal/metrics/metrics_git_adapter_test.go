package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMetricsGitAdapterReadsNestedReceiptHistory(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	f := &fixtureRepo{
		t: t, repo: filepath.Join(base, "repository"), evidence: filepath.Join(base, "evidence"),
	}
	f.root = filepath.Join(f.repo, "metasystem")
	if err := os.MkdirAll(f.root, 0o755); err != nil {
		t.Fatal(err)
	}
	f.run("init", "-q", "-b", "main")
	f.run("config", "user.name", "Fixture")
	f.run("config", "user.email", "fixture@example.invalid")
	f.write("metasystem/metasystem.conf", "evidence.root="+f.evidence+"\n")

	legacyReceipt := "1770000000|2026-08-01T00:00:00Z|RECEIPT|type=implement|outcome=shipped|goal=legacy|built_by=coordinator"
	f.write("metasystem/plans/receipts.log", legacyReceipt+"\n")
	legacyCommit := f.commit("2026-08-01T00:00:00Z", "legacy receipt landing", false)

	// The move copies the ledger; copied rows are not new landing attribution.
	f.write("metasystem/memory/receipts.log", legacyReceipt+"\n")
	if err := os.Remove(filepath.Join(f.root, "plans", "receipts.log")); err != nil {
		t.Fatal(err)
	}
	moveCommit := f.commit("2026-08-02T00:00:00Z", "move receipt register", false)

	currentReceipt := "1770000001|2026-08-03T00:00:00Z|RECEIPT|type=implement|outcome=shipped|goal=current|built_by=coordinator"
	f.write("metasystem/memory/receipts.log", legacyReceipt+"\n"+currentReceipt+"\n")
	currentCommit := f.commit("2026-08-03T00:00:00Z", "current receipt landing", false)

	facts, err := loadGitFacts(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if facts.receiptPath != "metasystem/memory/receipts.log" {
		t.Fatalf("current receipt path = %q", facts.receiptPath)
	}
	if facts.mainTip != currentCommit || facts.receiptBlob == "" {
		t.Fatalf("nested Git identity was not read: tip=%q receipt blob=%q", facts.mainTip, facts.receiptBlob)
	}
	wantKeys := map[string]string{
		legacyCommit:  receiptOriginalKey(legacyReceipt),
		currentCommit: receiptOriginalKey(currentReceipt),
	}
	wantLines := map[string]int{legacyCommit: 2, moveCommit: 0, currentCommit: 1}
	foundMove := false
	for _, landing := range facts.landings {
		if want, exists := wantLines[landing.SHA]; exists && landing.ChangedLines != want {
			t.Fatalf("commit %s changed lines = %d, want %d", landing.SHA, landing.ChangedLines, want)
		}
		if landing.SHA == moveCommit && len(landing.ReceiptKeys) != 0 {
			t.Fatalf("receipt copy was attributed as a new landing: %+v", landing)
		}
		if landing.SHA == moveCommit {
			foundMove = true
		}
		if want, exists := wantKeys[landing.SHA]; exists {
			if len(landing.ReceiptKeys) != 1 || landing.ReceiptKeys[0] != want {
				t.Fatalf("commit %s receipt keys = %v, want %s", landing.SHA, landing.ReceiptKeys, want)
			}
			delete(wantKeys, landing.SHA)
		}
	}
	if len(wantKeys) != 0 || !foundMove {
		t.Fatalf("receipt history was incomplete: missing keys=%v move present=%v", wantKeys, foundMove)
	}
}
