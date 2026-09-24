package landing

import (
	"path/filepath"
	"strings"
	"testing"
)

const receiptLineFixtureGoal = "fx-receipt-line"

const receiptLineBaseLedger = "receipt=existing\n"

func receiptLineAppended(goalID string) string {
	line := "1789000000|2026-09-12T00:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=verify|verify=clean|corrections=0|stop_loss=no|delegate=none"
	if goalID != "" {
		line += "|goal=" + goalID
	}
	return receiptLineBaseLedger + line + "|built_by=coordinator|critique_waived=none|waiver_stream=none|note=fixture\n"
}

func receiptLineCodeRun(f *receiptLineFixture, base, candidate, candidateLedger string) *receiptLineRun {
	r := f.newRun(base, candidate)
	r.expectPresentLedgerChange(receiptLineBaseLedger, candidateLedger, "metasystem/cmd/fixture.go")
	r.expectManifest()
	r.expectOwner()
	return r
}

func TestReceiptLineRefusesCodeWithoutItsLineAndNamesTheCommand(t *testing.T) {
	f := newReceiptLineFixture(t, false)
	f.writeInstall("cmd/fixture.go", "package main\n")
	decision := receiptLineCodeRun(f, "base", "candidate", receiptLineBaseLedger).decide(receiptLineFixtureGoal, "")
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
	f := newReceiptLineFixture(t, false)
	f.writeInstall("cmd/fixture.go", "package main\n")
	ledger := receiptLineAppended(receiptLineFixtureGoal)
	f.writeTop(f.ledger, ledger)
	decision := receiptLineCodeRun(f, "base", "candidate", ledger).decide(receiptLineFixtureGoal, "")
	if decision.Outcome != ReceiptLineOutcomePass || decision.Reason != "receipt-line-appended" {
		t.Fatalf("code with its receipt line did not pass: %+v", decision)
	}
	if !strings.Contains(decision.Detail, "epoch 1789000000") {
		t.Fatalf("pass detail does not name the appended line: %s", decision.Detail)
	}
}

func TestReceiptLineRefusesALineForAnotherGoal(t *testing.T) {
	f := newReceiptLineFixture(t, false)
	f.writeInstall("cmd/fixture.go", "package main\n")
	ledger := receiptLineAppended("other-goal")
	f.writeTop(f.ledger, ledger)
	decision := receiptLineCodeRun(f, "base", "candidate", ledger).decide(receiptLineFixtureGoal, "")
	if decision.Outcome != ReceiptLineOutcomeRefused {
		t.Fatalf("a receipt line for another goal satisfied the check: %+v", decision)
	}
	if !strings.Contains(decision.Detail, "the RECEIPT lines it appends name goal other-goal") {
		t.Fatalf("refusal does not name the other goal: %s", decision.Detail)
	}
}

func TestReceiptLineWithoutAGoalAcceptsAnyAppendedReceipt(t *testing.T) {
	f := newReceiptLineFixture(t, false)
	f.writeInstall("cmd/fixture.go", "package main\n")
	if decision := receiptLineCodeRun(f, "base", "candidate-without-line", receiptLineBaseLedger).decide("", ""); decision.Outcome != ReceiptLineOutcomeRefused || !strings.Contains(decision.Detail, "--goal <goal id>") {
		t.Fatalf("code without any receipt line was not refused with the goal placeholder: %+v", decision)
	}
	ledger := receiptLineAppended("")
	f.writeTop(f.ledger, ledger)
	if decision := receiptLineCodeRun(f, "base", "candidate-with-line", ledger).decide("", ""); decision.Outcome != ReceiptLineOutcomePass {
		t.Fatalf("an appended receipt line did not satisfy a goal-less landing: %+v", decision)
	}
}

func TestReceiptLineIgnoresNonReceiptLines(t *testing.T) {
	f := newReceiptLineFixture(t, false)
	f.writeInstall("cmd/fixture.go", "package main\n")
	ledger := receiptLineBaseLedger + "1789000000|2026-09-12T00:00:00Z|CORRECTION|ref_epoch=1|field=note|was=a|now=b|reason=fixture\n"
	f.writeTop(f.ledger, ledger)
	if decision := receiptLineCodeRun(f, "base", "candidate", ledger).decide(receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeRefused {
		t.Fatalf("a CORRECTION line passed as a receipt: %+v", decision)
	}
}

func TestReceiptLineExemptsRecordsOnlyAndReceiptOnlyLandings(t *testing.T) {
	f := newReceiptLineFixture(t, false)
	f.writeInstall("plans/note.md", "a record\n")
	f.writeInstall("records/narrator-digest.log", "digest=existing\ndigest=more\n")
	r := f.newRun("base", "candidate-records")
	r.expectPresentLedgerChange(receiptLineBaseLedger, receiptLineBaseLedger,
		"metasystem/plans/note.md", "metasystem/records/narrator-digest.log")
	r.expectManifest()
	r.expectOwner()
	r.expectOwner()
	if decision := r.decide(receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeExempt ||
		decision.Reason != "records-only" {
		t.Fatalf("a records-only landing was not exempt: %+v", decision)
	}
	ledger := receiptLineAppended(receiptLineFixtureGoal)
	f.writeTop(f.ledger, ledger)
	r = f.newRun("base-after-records", "candidate-receipt")
	r.expectPresentLedgerChange(receiptLineBaseLedger, ledger, f.ledger)
	r.expectManifest()
	r.expectOwner()
	if decision := r.decide(receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeExempt ||
		decision.Reason != "receipt-only" {
		t.Fatalf("a receipt-only landing was not exempt: %+v", decision)
	}
	t.Run("empty change list stops before candidate ledger", func(t *testing.T) {
		r := f.newRun("base-without-changes", "same-tree")
		r.expectLocation()
		r.expectHead()
		r.expectBaseLedger(ledger, true)
		r.expectChanged()
		if decision := r.decide(receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeExempt ||
			decision.Reason != "no-changes" {
			t.Fatalf("a landing with no changed paths was not exempt: %+v", decision)
		}
	})
}

func TestReceiptLineExemptsAnExactRevert(t *testing.T) {
	f := newReceiptLineFixture(t, false)
	f.writeInstall("cmd/fixture.go", "package main\n")
	r := f.newRun("base", "candidate")
	r.expectPresentLedgerChange(receiptLineBaseLedger, receiptLineBaseLedger, "metasystem/cmd/fixture.go")
	if decision := r.decide(receiptLineFixtureGoal, "exact-revert"); decision.Outcome != ReceiptLineOutcomeExempt ||
		decision.Reason != "exact-revert" {
		t.Fatalf("an exact revert was not exempt: %+v", decision)
	}
}

func TestReceiptLineExemptsACheckoutWithoutATrackedLedger(t *testing.T) {
	f := newReceiptLineFixture(t, false)
	f.removeTop(f.ledger)
	f.writeInstall("cmd/fixture.go", "package main\n")
	r := f.newRun("base-without-ledger", "candidate")
	r.expectLocation()
	r.expectHead()
	r.expectBaseLedger("", false)
	if decision := r.decide(receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeExempt ||
		decision.Reason != "no-receipt-ledger" {
		t.Fatalf("a checkout without a tracked ledger was not exempt: %+v", decision)
	}
}

func TestReceiptLineRefusesALandingThatRemovesTheLedger(t *testing.T) {
	f := newReceiptLineFixture(t, false)
	f.writeInstall("cmd/fixture.go", "package main\n")
	f.removeTop(f.ledger)
	r := f.newRun("base", "candidate-without-ledger")
	r.expectLocation()
	r.expectHead()
	r.expectBaseLedger(receiptLineBaseLedger, true)
	r.expectChanged("metasystem/cmd/fixture.go", f.ledger)
	r.expectCandidateLedger("", false)
	if decision := r.decide(receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeRefused ||
		decision.Reason != "receipt-ledger-removed" {
		t.Fatalf("a landing that removes the ledger was not refused: %+v", decision)
	}
}

func TestReceiptLineNamesTheLedgerFromTheInstallationRoot(t *testing.T) {
	f := newReceiptLineFixture(t, false)
	f.writeInstall("cmd/fixture.go", "package main\n")
	decision := receiptLineCodeRun(f, "base", "candidate", receiptLineBaseLedger).decide(receiptLineFixtureGoal, "")
	if decision.Ledger != filepath.ToSlash(filepath.Join("memory", "receipts.log")) {
		t.Fatalf("ledger path is not installation-relative: %+v", decision)
	}
}

func TestReceiptLineRefusesALandingThatRemovesTheLedgerAlone(t *testing.T) {
	f := newReceiptLineFixture(t, false)
	f.removeTop(f.ledger)
	r := f.newRun("base", "candidate-without-ledger")
	r.expectLocation()
	r.expectHead()
	r.expectBaseLedger(receiptLineBaseLedger, true)
	r.expectChanged(f.ledger)
	r.expectCandidateLedger("", false)
	if decision := r.decide(receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeRefused ||
		decision.Reason != "receipt-ledger-removed" {
		t.Fatalf("a landing that removes only the ledger was not refused: %+v", decision)
	}
}

// The application keeps state at the repository top while its installation
// lives beneath vendor/metasystem.
func TestReceiptLineVendoredAdoptedLayoutKeepsApplicationStateAsRecords(t *testing.T) {
	f := newReceiptLineFixture(t, true)
	f.writeTop("plans/goals/app-goal.md", "# app-goal\n")
	f.writeTop("records/misc/note.md", "note\n")
	f.writeTop("app/main.go", "package main\n")
	f.writeTop("plans/goals/app-goal.md", "# app-goal\n- State: claimed\n")
	f.writeTop("records/misc/note.md", "note\nmore\n")
	r := f.newRun("vendored-base", "candidate-records")
	r.expectPresentLedgerChange(receiptLineBaseLedger, receiptLineBaseLedger,
		"plans/goals/app-goal.md", "records/misc/note.md")
	r.expectManifest()
	if decision := r.decide(receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeExempt ||
		decision.Reason != "records-only" {
		t.Fatalf("application records were counted as code: %+v", decision)
	}
	otherLedger := receiptLineBaseLedger + "1|2026-09-12T00:00:00Z|RECEIPT|type=implement|outcome=shipped|goal=other|note=x\n"
	f.writeTop(f.ledger, otherLedger)
	r = f.newRun("base-after-records", "candidate-receipt")
	r.expectPresentLedgerChange(receiptLineBaseLedger, otherLedger, f.ledger)
	r.expectManifest()
	if decision := r.decide(receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomeExempt ||
		decision.Reason != "receipt-only" || decision.Ledger != "../../memory/receipts.log" {
		t.Fatalf("the application ledger alone was not receipt-only: %+v", decision)
	}
	f.writeInstall("cmd/vendored.go", "package main\n")
	r = f.newRun("base-after-receipt", "candidate-code")
	r.expectPresentLedgerChange(otherLedger, otherLedger, "vendor/metasystem/cmd/vendored.go")
	r.expectManifest()
	r.expectOwner()
	decision := r.decide(receiptLineFixtureGoal, "")
	if decision.Outcome != ReceiptLineOutcomeRefused || decision.Reason != "receipt-line-missing" ||
		!strings.Contains(decision.Detail, "the landing changes code (cmd/vendored.go)") ||
		!strings.Contains(decision.Detail, "include ../../memory/receipts.log in the landing") {
		t.Fatalf("vendored code without its line was not refused with application-root paths: %+v", decision)
	}
	matchingLedger := otherLedger + "2|2026-09-12T00:00:01Z|RECEIPT|type=implement|outcome=shipped|goal=" + receiptLineFixtureGoal + "|note=y\n"
	f.writeTop(f.ledger, matchingLedger)
	r = f.newRun("base-after-receipt", "candidate-code-and-receipt")
	r.expectPresentLedgerChange(otherLedger, matchingLedger, f.ledger, "vendor/metasystem/cmd/vendored.go")
	r.expectManifest()
	r.expectOwner()
	if decision := r.decide(receiptLineFixtureGoal, ""); decision.Outcome != ReceiptLineOutcomePass {
		t.Fatalf("vendored code with its line did not pass: %+v", decision)
	}
}
