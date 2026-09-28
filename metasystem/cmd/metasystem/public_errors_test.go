package main

import (
	"strings"
	"testing"
)

// The public refusals of the 2026-09-28 error-message audit
// (agentic-tools-evidence/error-messages-20260928/public-audit.md): each
// witness names its audit row. A refusal names what is wrong in the words the
// caller typed and the public command that helps; it never crashes, never
// names an internal command, never exits 0 for work it did not do.

// EM-01: question answer with no question refused by name, never a panic.
func TestQuestionAnswerWithoutAQuestionNamesIt(t *testing.T) {
	b := newIntentBed(t, false, nil)
	code, stdout, stderr := b.run(b.owners(), "question", "answer")
	text := stdout + stderr
	if code != 2 || !strings.Contains(text, "needs the question") || !strings.Contains(text, "metasystem question list") {
		t.Fatalf("question answer without Q: code %d stdout %q stderr %q", code, stdout, stderr)
	}
}
