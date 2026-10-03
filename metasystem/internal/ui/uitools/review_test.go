package uitools

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
)

// What the review sitting adds to this server (g1-s65): the finding deposit
// with its bounds (D8), the desk's display suggestion (D5), and the changes
// read over the reviewed tree through the same owner the desk reads (D4).

// findingArgs is a whole finding as a review asks for it (review-findings-
// read-as-decisions §4): the evidence as the reviewer wrote it, its anchor and
// consequence, and the plain layers a person reads first.
func findingArgs(over Args) Args {
	args := Args{
		"kind": "finding", "text": "nothing recorded covers a press that dies between publish and reconcile",
		"anchor": "internal/owner.go:60-72", "consequence": "a dead press leaves the lock held until restart",
		"severity": "blocks", "title": "A press that dies halfway leaves the ledger locked.",
		"why":       "The next press waits forever, and nobody is told why.",
		"recommend": "must-fix", "reason": "Release the lock on every way out.",
	}
	for key, value := range over {
		args[key] = value
	}
	return args
}

func TestAFindingCarriesItsAnchorItsConsequenceAndItsPlainLayers(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpDeposit, findingArgs(nil))
	testutil.Expect(t, "it is prepared", result.Failed(), false)
	lines := strings.Split(strings.TrimRight(result.Text(), "\n"), "\n")
	testutil.Require(t, "eleven lines", len(lines), 11)
	testutil.Expect(t, "the kind", lines[1], DepositHeader+DepositFinding)
	testutil.Expect(t, "the anchor", lines[2], DepositAnchor+"internal/owner.go:60-72")
	testutil.Expect(t, "the consequence", lines[3], DepositConsequence+"a dead press leaves the lock held until restart")
	testutil.Expect(t, "the severity", lines[4], DepositSeverity+"blocks")
	testutil.Expect(t, "the title", lines[5], DepositTitle+"A press that dies halfway leaves the ledger locked.")
	testutil.Expect(t, "why it matters", lines[6], DepositWhy+"The next press waits forever, and nobody is told why.")
	testutil.Expect(t, "the recommendation", lines[7], DepositRecommends+"must-fix")
	testutil.Expect(t, "and its reason", lines[8], DepositReason+"Release the lock on every way out.")
	testutil.Expect(t, "the words, as the reviewer wrote them", lines[10],
		"nothing recorded covers a press that dies between publish and reconcile")
}

// A finding a person cannot read as a decision is refused, naming what is
// missing, so the Partner can offer it again whole.
func TestAFindingWithoutItsPlainLayersIsRefusedNamingTheField(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	for _, missing := range []struct{ field, says string }{
		{"severity", "a finding says how much it matters as its severity: blocks, fix or note"},
		{"title", "a finding carries a title: the problem in one plain sentence"},
		{"why", "a finding carries why it matters: what happens if it is ignored"},
		{"recommend", "a finding carries the one decision you recommend: must-fix, fix-later, not-a-problem or accept"},
		{"reason", "a finding carries the reason for the decision you recommend, in one sentence"},
	} {
		result := readers.Answer(OpDeposit, findingArgs(Args{missing.field: " "}))
		testutil.Expect(t, "without its "+missing.field, result.Prepared, refusedWords+missing.says+"\n")
	}
	odd := readers.Answer(OpDeposit, findingArgs(Args{"severity": "critical"}))
	testutil.Expect(t, "a severity of its own", odd.Prepared,
		refusedWords+"a finding's severity is blocks, fix or note; \"critical\" is none of them\n")
	other := readers.Answer(OpDeposit, findingArgs(Args{"recommend": "ignore"}))
	testutil.Expect(t, "a decision of its own", other.Prepared,
		refusedWords+"the decision you recommend is must-fix, fix-later, not-a-problem or accept; \"ignore\" is none of them\n")
	for _, heavy := range []string{"must-fix", "accept"} {
		note := readers.Answer(OpDeposit, findingArgs(Args{"severity": "note", "recommend": heavy}))
		testutil.Expect(t, "a note recommending "+heavy, note.Prepared,
			refusedWords+"a note is fixed after landing or is not a problem; recommend fix-later or not-a-problem\n")
	}
}

// The title is what a person reads first, so it carries no commit id, no path
// and no timestamp; those belong in the evidence.
func TestAFindingsTitleCarriesNoIdNoPathAndNoTimestamp(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	for _, title := range []string{
		"The review looked at e2f9c92 and not the version that lands.",
		"The verdict strip in panel.ts:284 is not tested.",
		"It was marked ready at 12:19:54Z, seventeen seconds after the push.",
		"origin/goal/fleet-page-redesign was last fetched long ago.",
		"Marked ready at 2026-10-02T12:19 without a check.",
	} {
		result := readers.Answer(OpDeposit, findingArgs(Args{"title": title}))
		testutil.Expect(t, "refused: "+title, result.Prepared, refusedWords+
			"a finding's title is the problem in a person's words, with no commit id, file path or timestamp; "+
			"put those in the finding's own words, which the person reads as its evidence\n")
	}
	for _, plain := range []string{
		"The version of 2 October, 14:19, is not the one that would land.",
		"Nothing checks the fix-after-landing and/or follow-up path.",
		"The facade was defaced by 1234567 people.",
	} {
		testutil.Expect(t, "admitted: "+plain, readers.Answer(OpDeposit, findingArgs(Args{"title": plain})).Failed(), false)
	}
}

func TestAFindingIsBoundedLikeAnEntry(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	long := readers.Answer(OpDeposit, Args{"kind": "finding", "text": strings.Repeat("x", 2001)})
	testutil.Expect(t, "a finding past an entry's bound", long.Prepared,
		refusedWords+"the finding carries at most 2000 characters and this one is 2001; offer the entry itself, shorter\n")
	empty := readers.Answer(OpDeposit, Args{"kind": "finding", "text": "  "})
	testutil.Expect(t, "a finding that says nothing", strings.Contains(empty.Prepared, "needs the words of the finding"), true)
	anchor := readers.Answer(OpDeposit, Args{"kind": "finding", "text": "x", "anchor": strings.Repeat("a", 501)})
	testutil.Expect(t, "an anchor past its bound", strings.Contains(anchor.Prepared, "the anchor on a deposit carries at most 500"), true)
}

func TestPresentPreparesADeskItemOrRefusesInWords(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	source := readers.Answer(OpPresent, Args{"kind": "source", "path": "internal/owner.go", "from": "41", "to": "88"})
	testutil.Expect(t, "a file at a range", source.Prepared,
		PresentedLine+"\n"+PresentHeader+"source\n"+PresentPath+"internal/owner.go\n"+PresentFrom+"41\n"+PresentTo+"88\n")
	section := readers.Answer(OpPresent, Args{"kind": "section", "path": "plans/designs/d.md", "section": "D3"})
	testutil.Expect(t, "a record's section", section.Prepared,
		PresentedLine+"\n"+PresentHeader+"section\n"+PresentPath+"plans/designs/d.md\n"+PresentSection+"D3\n")
	evidence := readers.Answer(OpPresent, Args{"kind": "evidence", "path": "shots/room-1280-light.png"})
	testutil.Expect(t, "an evidence file, by its evidence-relative path (g1-s71 D4)", evidence.Prepared,
		PresentedLine+"\n"+PresentHeader+"evidence\n"+PresentPath+"shots/room-1280-light.png\n")
	changes := readers.Answer(OpPresent, Args{"kind": "changes"})
	testutil.Expect(t, "the change index", changes.Prepared, PresentedLine+"\n"+PresentHeader+"changes\n")

	for _, refused := range []struct {
		args Args
		says string
	}{
		{Args{"kind": "window"}, "a desk item is source, changes, diff, section or evidence; \"window\" is none of them"},
		{Args{"kind": "evidence"}, "an evidence item names the file by its path relative to the evidence, as the listing names it"},
		{Args{"kind": "evidence", "path": "../secret.txt"}, "\"../secret.txt\" is not a path inside the evidence"},
		{Args{"kind": "source"}, "a source item names the path of a file of the reviewed tree"},
		{Args{"kind": "diff", "path": "../x"}, "\"../x\" is not a path inside the reviewed tree"},
		{Args{"kind": "source", "path": "a.go", "from": "9", "to": "2"}, "a range runs forwards: line 9 to line 2 is not one"},
		{Args{"kind": "source", "path": "a.go", "from": "one"}, "from is a line number, and \"one\" is not one"},
		{Args{"kind": "section", "path": "d.md"}, "a section item names the record and the heading"},
	} {
		result := readers.Answer(OpPresent, refused.args)
		testutil.Expect(t, "refused: "+refused.says, result.Prepared, refusedWords+refused.says+"\n")
	}
}

// The changes read reads the review record's head and answers from the review
// owner: the change index, or one file's hunks.
func TestTheChangesReadIsTheDesksOwn(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	record := "metasystem/plans/reviews/review-of-g1-s64.md"
	tip := strings.Repeat("e", 40)
	readers.Document = func(id string) (project.Document, error) {
		if id != record {
			return project.Document{}, errors.New("no document at " + id)
		}
		return project.Document{ID: id, Revision: "r1", Source: "# Review of g1-s64\n\n- Kind: review\n- Goals: g1-s64\n" +
			"- Reviewed: " + tip + " (the tip of goal/g1-s64)\n\n## Findings\n", Record: &project.Head{Kind: "review"}}, nil
	}
	readers.Review = func() (Reviewing, error) { return fakeReviewing{tip: tip}, nil }

	index := readers.Answer(OpChanges, Args{"record": record})
	testutil.Expect(t, "it read", index.Failed(), false)
	testutil.Expect(t, "the source names the tip", strings.Contains(index.Source, "goal/g1-s64 at "+tip[:12]), true)
	testutil.Expect(t, "a file with its counts", strings.Contains(index.Body, "- internal/owner.go +2 -1"), true)

	diff := readers.Answer(OpChanges, Args{"record": record, "path": "internal/owner.go"})
	testutil.Expect(t, "the hunks", strings.Contains(diff.Body, "@@ -1 +1 @@\n-old\n+new"), true)

	refused := readers.Answer(OpChanges, Args{"record": "plans/designs/d1.md"})
	testutil.Expect(t, "not a review", refused.Problem, "no document at plans/designs/d1.md")
	none := readers.Answer(OpChanges, Args{})
	testutil.Expect(t, "no record", none.Problem, "this tool needs the review record's checkout-relative path")
}

type fakeReviewing struct{ tip string }

func (f fakeReviewing) Changes(review.Reviewed) (review.Index, error) {
	return review.Index{Goal: "g1-s64", Files: []review.File{{Path: "internal/owner.go", Added: 2, Deleted: 1}},
		Supplied: 1, Total: 1, Current: f.tip}, nil
}

func (f fakeReviewing) Diff(_ review.Reviewed, path string, _ bool) (review.Diff, error) {
	return review.Diff{Path: path, Parts: []review.Part{{Hunks: []review.Hunk{{Header: "@@ -1 +1 @@",
		Lines: []review.DiffLine{{Kind: review.LineDeleted, Old: 1, Text: "old"}, {Kind: review.LineAdded, New: 1, Text: "new"}}}}}},
		Supplied: 2, Total: 2}, nil
}

const refusedWords = "Outcome: this call was refused — "
