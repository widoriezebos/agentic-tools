package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	case rev == "origin/main":
		return commitMain, nil
	case rev == "origin/"+review.Branch(reviewedGoal):
		return f.tipOf(reviewedGoal), nil
	case rev == "origin/"+review.Branch(secondGoal):
		return f.tipOf(secondGoal), nil
	}
	return "", fmt.Errorf("gittree commit %q is unreadable", rev)
}

func (fixtureGit) MergeBases(_, tip string) ([]string, error) {
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
const (
	reviewOpening = "Open this review"
	reviewWalk    = "Walk me through Built"
	reviewClose   = "Close this review"
)

// reviewAnswers are the review's canned answers: the opening in five parts,
// each claim anchored, and the Built walk.
var reviewAnswers = []fakeacp.Answer{
	{When: reviewOpening, Chunks: []string{
		"## Asked\n\nThe goal asks that one lock be held across publish and reconcile, so a reconcile never reads a " +
			"half-published ledger (plans/designs/reading.md is the design it names).\n\n",
		"## Built\n\nThe lock is taken once in `internal/owner/owner.go:14-27` and released on every return path " +
			"through `internal/owner/owner.go:29-32`.\n\n",
		"## Examined\n\nNo critique of this goal is recorded. Nothing recorded tries a press that dies between the " +
			"publish and the reconcile.\n\n",
		"## Proven\n\nOne test, `internal/owner/owner_test.go:5-13`, proves the happy path and assumes both calls " +
			"return.\n\n",
		"## Behaves\n\nNothing records it running; there is no evidence path on the design.\n",
	}},
	{When: reviewWalk, Chunks: []string{
		"Built, in the order I am putting it on the desk.\n\n",
		"First the owner itself, `internal/owner/owner.go:14-27`: begin takes the lock, records the press, " +
			"publishes, then reconciles.\n\n",
		"Then the change as a diff, so you can see the old two locks become one.\n",
	}},
	{When: reviewClose, Chunks: []string{"Here is the Outcome as drafted, with your verdict first.\n"}},
}

// reviewReads are the review's canned tool calls: the finding the opening
// offers, the desk items the Built walk puts up, and the Outcome of the close.
var reviewReads = []fakeacp.Read{
	{
		When:  reviewOpening,
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpDeposit,
		Title: "deposit(finding)",
		Result: uitools.DepositedLine + "\n" + uitools.DepositHeader + uitools.DepositFinding + "\n" +
			uitools.DepositAnchor + "internal/owner/owner.go:21-24\n" +
			uitools.DepositConsequence + "a press that dies after publishing holds the lock until the process ends\n" +
			uitools.DepositSeparator + "\n" +
			"Nothing recorded tries a press that dies between the publish and the reconcile.\n",
	},
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
