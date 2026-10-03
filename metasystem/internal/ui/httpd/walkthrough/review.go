package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The walkthrough's review room (g1-s65 slice A): two goals waiting to land,
// their candidates as a table of commits and files, and a Partner that answers
// a review's opening in five parts, offers a finding, and walks through Built by
// putting a file on the desk.
//
// The candidate is a Git of this fixture's own, because the fixture checkout is
// not a repository with goal branches in it: the review owner is the shipped
// one, and only what it reads is invented. Which tip each goal's branch stands
// at is read from a file beside the fixture on every read, so a walkthrough can
// move a branch while the human is stepped out — a push, as the room sees one —
// by writing that file.

// The two goals waiting to land.
const (
	reviewedGoal = "g1-s21"
	secondGoal   = "g1-s90"
)

// The commits: main, the merge base, the first tip, and the tip after a push.
var (
	commitMain  = strings.Repeat("d3", 20)
	commitBase  = strings.Repeat("b0", 20)
	commitTip   = strings.Repeat("a1", 20)
	commitMoved = strings.Repeat("c2", 20)
)

// ownerBefore, ownerAfter and ownerPushed are the one file the change is about,
// at the merge base, at the reviewed tip, and after the push.
var ownerBefore = strings.Join([]string{
	"package owner",
	"",
	"import \"sync\"",
	"",
	"// owner publishes the ledger and reconciles it.",
	"type owner struct {",
	"\tmu sync.Mutex",
	"}",
	"",
	"func (o *owner) publish() error {",
	"\to.mu.Lock()",
	"\tdefer o.mu.Unlock()",
	"\treturn write()",
	"}",
	"",
	"func (o *owner) reconcile() error {",
	"\to.mu.Lock()",
	"\tdefer o.mu.Unlock()",
	"\treturn read()",
	"}",
	"",
}, "\n")

var ownerAfter = strings.Join([]string{
	"package owner",
	"",
	"import \"sync\"",
	"",
	"// owner publishes the ledger and reconciles it, under one lock held",
	"// across both, so a reconcile never reads a half-published ledger.",
	"type owner struct {",
	"\tmu sync.Mutex",
	"\t// held is the press that holds the lock, or \"\".",
	"\theld string",
	"}",
	"",
	"// begin takes the lock for one press and publishes, then reconciles.",
	"func (o *owner) begin(press string) error {",
	"\to.mu.Lock()",
	"\to.held = press",
	"\tif err := write(); err != nil {",
	"\t\to.release()",
	"\t\treturn err",
	"\t}",
	"\tif err := read(); err != nil {",
	"\t\to.release()",
	"\t\treturn err",
	"\t}",
	"\to.release()",
	"\treturn nil",
	"}",
	"",
	"func (o *owner) release() {",
	"\to.held = \"\"",
	"\to.mu.Unlock()",
	"}",
	"",
}, "\n")

var ownerPushed = strings.Replace(ownerAfter,
	"\tif err := read(); err != nil {",
	"\t// A press that dies here leaves the lock to the next begin.\n\tif err := read(); err != nil {", 1)

var ownerTest = strings.Join([]string{
	"package owner",
	"",
	"import \"testing\"",
	"",
	"func TestBeginPublishesThenReconciles(t *testing.T) {",
	"\to := &owner{}",
	"\tif err := o.begin(\"m1e\"); err != nil {",
	"\t\tt.Fatal(err)",
	"\t}",
	"\tif o.held != \"\" {",
	"\t\tt.Fatalf(\"the lock is still held by %q\", o.held)",
	"\t}",
	"}",
	"",
}, "\n")

// candidateFiles is every commit's tree, by path.
var candidateFiles = map[string]map[string]string{
	commitBase:  {"internal/owner/owner.go": ownerBefore},
	commitTip:   {"internal/owner/owner.go": ownerAfter, "internal/owner/owner_test.go": ownerTest},
	commitMoved: {"internal/owner/owner.go": ownerPushed, "internal/owner/owner_test.go": ownerTest},
}

// fixtureGit is the candidate as the review owner reads it.
type fixtureGit struct {
	// branches is the file beside the fixture that says which tip each goal's
	// branch stands at; a goal it does not name is at the first tip.
	branches string
}

func (f fixtureGit) tipOf(goal string) string {
	body, err := os.ReadFile(f.branches)
	if err == nil {
		held := map[string]string{}
		if json.Unmarshal(body, &held) == nil {
			if moved := held[goal]; moved == "moved" {
				return commitMoved
			}
		}
	}
	return commitTip
}

func (f fixtureGit) ResolveCommit(rev string) (string, error) {
	switch {
	case rev == "origin/main", rev == "HEAD":
		// HEAD is the checkout's, which a shaping remark keeps as provenance.
		return commitMain, nil
	case rev == "origin/"+review.Branch(reviewedGoal):
		return f.tipOf(reviewedGoal), nil
	case rev == "origin/"+review.Branch(secondGoal):
		return f.tipOf(secondGoal), nil
	}
	return "", fmt.Errorf("gittree commit %q is unreadable", rev)
}

// MergeBases is the fixture's history: main and each tip meet at the base, and
// the push sits on the first tip.
func (fixtureGit) MergeBases(left, right string) ([]string, error) {
	if (left == commitTip && right == commitMoved) || (left == commitMoved && right == commitTip) {
		return []string{commitTip}, nil
	}
	if left == right {
		return []string{left}, nil
	}
	return []string{commitBase}, nil
}

func (fixtureGit) TreeOf(rev string) (string, error) { return rev, nil }

func (fixtureGit) FileAt(tree, path string) ([]byte, bool, error) {
	held, found := candidateFiles[tree][path]
	return []byte(held), found, nil
}

func (fixtureGit) FileCounts(from, to string) ([]gittree.FileCount, error) {
	counts := []gittree.FileCount{}
	for _, path := range pathsOf(from, to) {
		added, deleted := 0, 0
		for _, line := range diffLines(candidateFiles[from][path], candidateFiles[to][path]) {
			switch line[0] {
			case '+':
				added++
			case '-':
				deleted++
			}
		}
		if added+deleted > 0 {
			counts = append(counts, gittree.FileCount{Path: path, Added: int64(added), Deleted: int64(deleted)})
		}
	}
	return counts, nil
}

func (fixtureGit) PathDiff(from, to, path string) ([]byte, error) {
	return []byte(unified(candidateFiles[from][path], candidateFiles[to][path])), nil
}

func (fixtureGit) CommitsCarrying(string, string) ([]string, error) { return nil, nil }

// FetchBranch fetches nothing: the fixture's branches are its table.
func (fixtureGit) FetchBranch(string) error { return nil }

// CommitFacts says each fixture commit by a time and a builder, as a person is
// told a version (review-findings-read-as-decisions §3): the first tip on 2
// October and the push on 3 October.
func (fixtureGit) CommitFacts(rev string) (gittree.CommitFacts, error) {
	switch rev {
	case commitTip:
		return gittree.CommitFacts{At: time.Date(2026, 10, 2, 12, 19, 37, 0, time.UTC), Author: "m1e"}, nil
	case commitMoved:
		return gittree.CommitFacts{At: time.Date(2026, 10, 3, 7, 55, 2, 0, time.UTC), Author: "m1e"}, nil
	}
	return gittree.CommitFacts{}, fmt.Errorf("the fixture has no facts for %s", rev)
}

// LineCommits is never asked: the fixture's goals are reviewed at a tip.
func (fixtureGit) LineCommits(string, string) ([]string, error) {
	return nil, fmt.Errorf("the fixture has no blame")
}

func pathsOf(from, to string) []string {
	seen := map[string]bool{}
	paths := []string{}
	for _, tree := range []string{from, to} {
		for path := range candidateFiles[tree] {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	sortStrings(paths)
	return paths
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// diffLines is a line diff of two texts by their longest common subsequence,
// each line prefixed with ' ', '+' or '-'. The fixture's files are short.
func diffLines(before, after string) []string {
	a, b := splitLines(before), splitLines(after)
	table := make([][]int, len(a)+1)
	for i := range table {
		table[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	lines := []string{}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			lines = append(lines, " "+a[i])
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			lines = append(lines, "-"+a[i])
			i++
		default:
			lines = append(lines, "+"+b[j])
			j++
		}
	}
	for ; i < len(a); i++ {
		lines = append(lines, "-"+a[i])
	}
	for ; j < len(b); j++ {
		lines = append(lines, "+"+b[j])
	}
	return lines
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// unified is the diff as one hunk over the whole file, which is what a short
// fixture file's change is.
func unified(before, after string) string {
	lines := diffLines(before, after)
	old, fresh := len(splitLines(before)), len(splitLines(after))
	return fmt.Sprintf("@@ -1,%d +1,%d @@\n%s\n", old, fresh, strings.Join(lines, "\n"))
}

// branchesFile is where this fixture keeps which tip each goal stands at.
func branchesFile(checkout string) string {
	return filepath.Join(filepath.Dir(checkout), filepath.Base(checkout)+"-branches.json")
}

/* --------------------------------------------------- the canned review -- */

// The phrases of the review's fixed requests the canned answers are narrowed to.
// The opening of g1-s21's review offers three findings; g1-s90's is clean
// (review-findings-read-as-decisions, the mocks blocking and clean).
const (
	reviewOpening = "Open this review"
	reviewWalk    = "Walk me through Built"
	reviewClose   = "Close this review"
	findingsAsked = "recorded in plans/reviews/review-of-" + reviewedGoal + ".md"
	cleanAsked    = "recorded in plans/reviews/review-of-" + secondGoal + ".md"
)

// reviewAnswers are the review's canned answers: the opening, a plain
// paragraph first and then five parts, each claim anchored, and the Built walk.
var reviewAnswers = []fakeacp.Answer{
	{When: cleanAsked, Chunks: []string{
		"The goal: the board reads every goal once, however many seats hold it. This version reads the " +
			"ledger once per page and changes 2 files, both in the board's reader.\n\n",
		"## Asked\n\nOne read per page (plans/designs/reading.md).\n\n",
		"## Built\n\nThe reader in `internal/owner/owner.go:14-27`.\n\n",
		"## Examined\n\nOne critique round, closed with nothing material.\n\n",
		"## Proven\n\n`internal/owner/owner_test.go:5-13` proves it; 212 tests pass.\n\n",
		"## Behaves\n\nThe board was opened at 1280 wide and read each goal once.\n",
	}},
	{When: reviewOpening, Chunks: []string{
		"The goal: one lock held across publish and reconcile, so a reconcile never reads a half-published " +
			"ledger. This version replaces the two locks with one and adds a test of the happy path. It changes " +
			"2 files, both in the owner.\n\n",
		"## Asked\n\nThe goal asks that one lock be held across publish and reconcile, so a reconcile never reads a " +
			"half-published ledger (plans/designs/reading.md is the design it names).\n\n",
		"## Built\n\nThe lock is taken once in `internal/owner/owner.go:14-27` and released on every return path " +
			"through `internal/owner/owner.go:29-32`.\n\n",
		"## Examined\n\nNo critique of this goal is recorded. Nothing recorded tries a press that dies between the " +
			"publish and the reconcile.\n\n",
		"## Proven\n\nOne test, `internal/owner/owner_test.go:5-13`, proves the happy path and assumes both calls " +
			"return.\n\n",
		"## Behaves\n\nThe build recorded evidence of it running; the Behaves walk puts it on the desk.\n",
	}},
	{When: reviewWalk, Chunks: []string{
		"Built, in the order I am putting it on the desk.\n\n",
		"First the owner itself, `internal/owner/owner.go:14-27`: begin takes the lock, records the press, " +
			"publishes, then reconciles.\n\n",
		"Then the change as a diff, so you can see the old two locks become one.\n",
	}},
	{When: reviewClose, Chunks: []string{"Here is the Outcome as drafted, with your verdict first.\n"}},
}

// findingRead is one finding the opening of g1-s21's review offers, in the
// deposit tool's own framing, with its plain layers.
func findingRead(anchor, consequence, severity, title, why, recommend, reason, words string) fakeacp.Read {
	return fakeacp.Read{
		When:  findingsAsked,
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpDeposit,
		Title: "deposit(finding)",
		Result: uitools.DepositedLine + "\n" + uitools.DepositHeader + uitools.DepositFinding + "\n" +
			uitools.DepositAnchor + anchor + "\n" + uitools.DepositConsequence + consequence + "\n" +
			uitools.DepositSeverity + severity + "\n" + uitools.DepositTitle + title + "\n" +
			uitools.DepositWhy + why + "\n" + uitools.DepositRecommends + recommend + "\n" +
			uitools.DepositReason + reason + "\n" + uitools.DepositSeparator + "\n" + words + "\n",
	}
}

// reviewReads are the review's canned tool calls: the finding the opening
// offers, the desk items the Built walk puts up, and the Outcome of the close.
var reviewReads = []fakeacp.Read{
	findingRead("internal/owner/owner.go:21-24", "a press that dies after publishing holds the lock until the process ends",
		"blocks", "A press that dies halfway through leaves the ledger locked.",
		"The next press waits for a lock nobody will release, and nothing tells anyone why. Every later press stalls until the process restarts.",
		"must-fix", "Release the lock on every way out, and test a press that dies after publishing.",
		"Nothing recorded tries a press that dies between the publish and the reconcile: owner.go:21-24 releases the lock only on the error paths it names, and owner_test.go:5-13 assumes both calls return."),
	findingRead("internal/owner/owner_test.go:5-13", "a later change to the reconcile could break the lock without a failing test",
		"fix", "Only the happy path is tested.",
		"The test proves that one press publishes and reconciles. A change that breaks the failure paths would pass every test.",
		"fix-later", "Open a follow-up goal to test each way a press can fail.",
		"owner_test.go:5-13 is the only test; it calls begin once and asserts the lock is free afterwards. No test makes write() or read() fail."),
	findingRead("plans/designs/reading.md", "nothing breaks",
		"note", "The design still describes two locks.",
		"The design was written before the change and says the owner holds two locks. Nothing breaks; a reader of the design may be confused.",
		"not-a-problem", "The design is history; the code and its test say what holds now.",
		"plans/designs/reading.md (Part 2) names a publish lock and a reconcile lock; the change at a1a1a1a removed the second."),
	{
		When:   reviewWalk,
		Name:   "mcp__" + uitools.ServerName + "__" + uitools.OpPresent,
		Title:  "present(source)",
		Result: uitools.PresentedLine + "\n" + uitools.PresentHeader + "source\n" + uitools.PresentPath + "internal/owner/owner.go\n" + uitools.PresentFrom + "14\n" + uitools.PresentTo + "27\n",
	},
	{
		When:   reviewWalk,
		Name:   "mcp__" + uitools.ServerName + "__" + uitools.OpPresent,
		Title:  "present(diff)",
		Result: uitools.PresentedLine + "\n" + uitools.PresentHeader + "diff\n" + uitools.PresentPath + "internal/owner/owner.go\n",
	},
	{
		When:  reviewClose,
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpDeposit,
		Title: "deposit(outcome)",
		Result: uitools.DepositedLine + "\n" + uitools.DepositHeader + uitools.DepositOutcome + "\n" +
			uitools.DepositSeparator + "\n" +
			"The owner holds one lock across publish and reconcile (internal/owner/owner.go:14-27).\n\n" +
			"Findings: a press that dies after publishing — answered as recorded.\n",
	},
}
