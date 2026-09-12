package landing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const receiptLineFixtureGoal = "fx-receipt-line"

// stagedProjectTree stages the fixture's working tree and returns the
// whole-project index tree, the tree land.sh hands the check.
func (f *observeFixture) stagedProjectTree() string {
	f.t.Helper()
	f.git("add", "-A", "--", ".")
	return f.git("write-tree")
}

func (f *observeFixture) appendReceipt(goalID string) {
	f.t.Helper()
	line := "1789000000|2026-09-12T00:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=verify|verify=clean|corrections=0|stop_loss=no|delegate=none"
	if goalID != "" {
		line += "|goal=" + goalID
	}
	line += "|built_by=coordinator|critique_waived=none|waiver_stream=none|note=fixture\n"
	f.write("memory/receipts.log", "receipt=existing\n"+line)
}

func receiptLineDecision(t *testing.T, f *observeFixture, goalID, directFix string) ReceiptLineDecision {
	t.Helper()
	decision, err := ObserveReceiptLine(ReceiptLineParams{
		RepoRoot: f.root, CandidateTree: f.stagedProjectTree(), Goal: goalID, DirectFix: directFix,
	})
	if err != nil {
		t.Fatalf("receipt-line check failed: %v", err)
	}
	return decision
}

func TestReceiptLineRefusesCodeWithoutItsLineAndNamesTheCommand(t *testing.T) {
	f := newObserveFixture(t)
	f.write("cmd/fixture.go", "package main\n")
	decision := receiptLineDecision(t, f, receiptLineFixtureGoal, "")
	if decision.Outcome != ReceiptLineOutcomeRefused || decision.Reason != "receipt-line-missing" {
		t.Fatalf("code without a receipt line was not refused: %+v", decision)
	}
	for _, want := range []string{
		"the landing changes code (cmd/fixture.go)",
		"appends no RECEIPT line for goal " + receiptLineFixtureGoal,
		`scripts/receipt.sh add --type implement --outcome shipped --goal ` + receiptLineFixtureGoal + ` --built-by coordinator --note "<what landed and how it was verified>"`,
		"include memory/receipts.log in the landing",
	} {
		if !strings.Contains(decision.Detail, want) {
			t.Fatalf("refusal detail lacks %q: %s", want, decision.Detail)
		}
	}
	if decision.Ledger != "memory/receipts.log" || decision.Command == "" || len(decision.CodePaths) != 1 {
		t.Fatalf("refusal facts are incomplete: %+v", decision)
	}
}

func TestReceiptLinePassesWhenTheLineForItsGoalIsAppended(t *testing.T) {
	f := newObserveFixture(t)
	f.write("cmd/fixture.go", "package main\n")
	f.appendReceipt(receiptLineFixtureGoal)
	decision := receiptLineDecision(t, f, receiptLineFixtureGoal, "")
	if decision.Outcome != ReceiptLineOutcomePass || decision.Reason != "receipt-line-appended" {
		t.Fatalf("code with its receipt line did not pass: %+v", decision)
	}
	if !strings.Contains(decision.Detail, "epoch 1789000000") {
		t.Fatalf("pass detail does not name the appended line: %s", decision.Detail)
	}
}

func TestReceiptLineRefusesALineForAnotherGoal(t *testing.T) {
	f := newObserveFixture(t)
	f.write("cmd/fixture.go", "package main\n")
	f.appendReceipt("other-goal")
	decision := receiptLineDecision(t, f, receiptLineFixtureGoal, "")
	if decision.Outcome != ReceiptLineOutcomeRefused {
		t.Fatalf("a receipt line for another goal satisfied the check: %+v", decision)
	}
	if !strings.Contains(decision.Detail, "the RECEIPT lines it appends name goal other-goal") {
		t.Fatalf("refusal does not name the other goal: %s", decision.Detail)
	}
}

func TestReceiptLineWithoutAGoalAcceptsAnyAppendedReceipt(t *testing.T) {
	f := newObserveFixture(t)
	f.write("cmd/fixture.go", "package main\n")
	if decision := receiptLineDecision(t, f, "", ""); decision.Outcome != ReceiptLineOutcomeRefused ||
		!strings.Contains(decision.Detail, "--goal <goal id>") {
		t.Fatalf("code without any receipt line was not refused with the goal placeholder: %+v", decision)
	}
	f.appendReceipt("")
	if decision := receiptLineDecision(t, f, "", ""); decision.Outcome != ReceiptLineOutcomePass {
		t.Fatalf("an appended receipt line did not satisfy a goal-less landing: %+v", decision)
	}
}

func TestReceiptLineIgnoresNonReceiptLines(t *testing.T) {
	f := newObserveFixture(t)
	f.write("cmd/fixture.go", "package main\n")
	f.write("memory/receipts.log", "receipt=existing\n1789000000|2026-09-12T00:00:00Z|CORRECTION|ref_epoch=1|field=note|was=a|now=b|reason=fixture\n")
	if decision := receiptLineDecision(t, f, receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeRefused {
		t.Fatalf("a CORRECTION line passed as a receipt: %+v", decision)
	}
}

func TestReceiptLineExemptsRecordsOnlyAndReceiptOnlyLandings(t *testing.T) {
	f := newObserveFixture(t)
	f.write("plans/note.md", "a record\n")
	f.write("records/narrator-digest.log", "digest=existing\ndigest=more\n")
	if decision := receiptLineDecision(t, f, receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeExempt ||
		decision.Reason != "records-only" {
		t.Fatalf("a records-only landing was not exempt: %+v", decision)
	}
	f.git("commit", "-qm", "records")
	f.appendReceipt(receiptLineFixtureGoal)
	if decision := receiptLineDecision(t, f, receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeExempt ||
		decision.Reason != "receipt-only" {
		t.Fatalf("a receipt-only landing was not exempt: %+v", decision)
	}
}

func TestReceiptLineExemptsAnExactRevert(t *testing.T) {
	f := newObserveFixture(t)
	f.write("cmd/fixture.go", "package main\n")
	if decision := receiptLineDecision(t, f, receiptLineFixtureGoal, "exact-revert"); decision.Outcome != ReceiptLineOutcomeExempt ||
		decision.Reason != "exact-revert" {
		t.Fatalf("an exact revert was not exempt: %+v", decision)
	}
}

func TestReceiptLineExemptsACheckoutWithoutATrackedLedger(t *testing.T) {
	f := newObserveFixture(t)
	f.git("rm", "-q", "memory/receipts.log")
	f.git("commit", "-qm", "no ledger")
	f.write("cmd/fixture.go", "package main\n")
	if decision := receiptLineDecision(t, f, receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeExempt ||
		decision.Reason != "no-receipt-ledger" {
		t.Fatalf("a checkout without a tracked ledger was not exempt: %+v", decision)
	}
}

func TestReceiptLineRefusesALandingThatRemovesTheLedger(t *testing.T) {
	f := newObserveFixture(t)
	f.write("cmd/fixture.go", "package main\n")
	f.git("rm", "-q", "memory/receipts.log")
	if decision := receiptLineDecision(t, f, receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeRefused ||
		decision.Reason != "receipt-ledger-removed" {
		t.Fatalf("a landing that removes the ledger was not refused: %+v", decision)
	}
}

func TestReceiptLineNamesTheLedgerFromTheInstallationRoot(t *testing.T) {
	f := newObserveFixture(t)
	f.write("cmd/fixture.go", "package main\n")
	decision := receiptLineDecision(t, f, receiptLineFixtureGoal, "")
	if decision.Ledger != filepath.ToSlash(filepath.Join("memory", "receipts.log")) {
		t.Fatalf("ledger path is not installation-relative: %+v", decision)
	}
}

func TestReceiptLineRefusesALandingThatRemovesTheLedgerAlone(t *testing.T) {
	f := newObserveFixture(t)
	f.git("rm", "-q", "memory/receipts.log")
	if decision := receiptLineDecision(t, f, receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeRefused ||
		decision.Reason != "receipt-ledger-removed" {
		t.Fatalf("a landing that removes only the ledger was not refused: %+v", decision)
	}
}

// newVendoredAdoptedFixture lays out an application that vendors the
// installation at vendor/metasystem and keeps its registers, plans and
// records at its own root, the layout where the manifest has no row for
// the application's state.
func newVendoredAdoptedFixture(t *testing.T) (*observeFixture, string) {
	t.Helper()
	repository := t.TempDir()
	root := filepath.Join(repository, "vendor", "metasystem")
	f := newObserveFixtureAt(t, repository, root)
	f.git("rm", "-q", "memory/receipts.log")
	for _, entry := range []struct{ relative, content string }{
		{"memory/receipts.log", "receipt=existing\n"},
		{"plans/goals/app-goal.md", "# app-goal\n"},
		{"records/misc/note.md", "note\n"},
		{"app/main.go", "package main\n"},
	} {
		target := filepath.Join(repository, filepath.FromSlash(entry.relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(entry.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f.git("add", "-A", "--", "../..")
	f.git("commit", "-qm", "vendored base")
	return f, repository
}

func TestReceiptLineVendoredAdoptedLayoutKeepsApplicationStateAsRecords(t *testing.T) {
	f, repository := newVendoredAdoptedFixture(t)
	writeTop := func(relative, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repository, filepath.FromSlash(relative)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stage := func() string {
		t.Helper()
		f.git("add", "-A", "--", "../..")
		return f.git("write-tree")
	}
	decide := func() ReceiptLineDecision {
		t.Helper()
		decision, err := ObserveReceiptLine(ReceiptLineParams{RepoRoot: f.root, CandidateTree: stage(), Goal: receiptLineFixtureGoal})
		if err != nil {
			t.Fatalf("receipt-line check failed: %v", err)
		}
		return decision
	}
	writeTop("plans/goals/app-goal.md", "# app-goal\n- State: claimed\n")
	writeTop("records/misc/note.md", "note\nmore\n")
	if decision := decide(); decision.Outcome != ReceiptLineOutcomeExempt || decision.Reason != "records-only" {
		t.Fatalf("application records were counted as code: %+v", decision)
	}
	f.git("commit", "-qm", "records")
	writeTop("memory/receipts.log", "receipt=existing\n1|2026-09-12T00:00:00Z|RECEIPT|type=implement|outcome=shipped|goal=other|note=x\n")
	if decision := decide(); decision.Outcome != ReceiptLineOutcomeExempt || decision.Reason != "receipt-only" ||
		decision.Ledger != "../../memory/receipts.log" {
		t.Fatalf("the application ledger alone was not receipt-only: %+v", decision)
	}
	f.git("commit", "-qm", "receipt")
	f.write("cmd/vendored.go", "package main\n")
	decision := decide()
	if decision.Outcome != ReceiptLineOutcomeRefused || decision.Reason != "receipt-line-missing" ||
		!strings.Contains(decision.Detail, "the landing changes code (cmd/vendored.go)") ||
		!strings.Contains(decision.Detail, "include ../../memory/receipts.log in the landing") {
		t.Fatalf("vendored code without its line was not refused with application-root paths: %+v", decision)
	}
	writeTop("memory/receipts.log", "receipt=existing\n1|2026-09-12T00:00:00Z|RECEIPT|type=implement|outcome=shipped|goal=other|note=x\n2|2026-09-12T00:00:01Z|RECEIPT|type=implement|outcome=shipped|goal="+receiptLineFixtureGoal+"|note=y\n")
	if decision := decide(); decision.Outcome != ReceiptLineOutcomePass {
		t.Fatalf("vendored code with its line did not pass: %+v", decision)
	}
}
