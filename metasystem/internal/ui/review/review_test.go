package review

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The desk's reads (g1-s65 D4) against a Git that is a table: which commit a
// ref names, which files each commit holds, what changed between two commits
// and which commits carry a trailer. Nothing here runs Git.

type fakeGit struct {
	refs     map[string]string
	bases    map[string]string
	files    map[string]map[string]string
	counts   map[string][]gittree.FileCount
	diffs    map[string]string
	carrying map[string][]string
	blame    map[string][]string
}

func commit(letter string) string { return strings.Repeat(letter, 40) }

func (f fakeGit) ResolveCommit(rev string) (string, error) {
	if id, known := f.refs[rev]; known {
		return id, nil
	}
	return "", fmt.Errorf("gittree commit %q is unreadable", rev)
}

func (f fakeGit) MergeBases(left, right string) ([]string, error) {
	if base, known := f.bases[left+" "+right]; known {
		return []string{base}, nil
	}
	return nil, errors.New("no merge base")
}

func (f fakeGit) TreeOf(rev string) (string, error) { return "tree-of-" + rev, nil }

func (f fakeGit) FileAt(tree, path string) ([]byte, bool, error) {
	held, known := f.files[strings.TrimPrefix(tree, "tree-of-")][path]
	return []byte(held), known, nil
}

func (f fakeGit) FileCounts(from, to string) ([]gittree.FileCount, error) {
	return f.counts[from+".."+to], nil
}

func (f fakeGit) PathDiff(from, to, path string) ([]byte, error) {
	return []byte(f.diffs[from+".."+to+" "+path]), nil
}

func (f fakeGit) CommitsCarrying(ref, line string) ([]string, error) {
	if found, known := f.carrying[ref+" "+line]; known {
		return found, nil
	}
	return nil, fmt.Errorf("unknown revision %s", ref)
}

func (f fakeGit) LineCommits(rev, path string) ([]string, error) {
	if found, known := f.blame[rev+" "+path]; known {
		return found, nil
	}
	return nil, fmt.Errorf("no blame for %s at %s", path, rev)
}

// A goal waiting to land: goal/g1-s64 at origin, its merge base with main, one
// changed file and one added binary.
func waiting() fakeGit {
	base, tip := commit("b"), commit("e")
	return fakeGit{
		refs: map[string]string{
			"origin/goal/g1-s64": tip, "goal/g1-s64": commit("d"), "origin/main": commit("a"),
		},
		bases: map[string]string{commit("a") + " " + tip: base},
		files: map[string]map[string]string{
			tip: {
				"internal/owner.go": "package owner\n\ntype owner struct{}\n\nfunc begin() {}\n",
				"docs/shot.png":     "\x89PNG\x00\x00binary",
			},
		},
		counts: map[string][]gittree.FileCount{
			base + ".." + tip: {{Path: "internal/owner.go", Added: 2, Deleted: 1}, {Path: "docs/shot.png", Binary: true}},
		},
		diffs: map[string]string{
			base + ".." + tip + " internal/owner.go": "diff --git a/internal/owner.go b/internal/owner.go\n" +
				"--- a/internal/owner.go\n+++ b/internal/owner.go\n" +
				"@@ -1,4 +1,5 @@\n package owner\n \n-type owner int\n+type owner struct{}\n+\n func begin() {}\n",
		},
	}
}

// A done goal: two Goal-Item commits on main, each against its first parent.
func done() fakeGit {
	first, second := commit("1"), commit("2")
	return fakeGit{
		refs: map[string]string{
			"origin/main": commit("a"), first + "^1": commit("0"), second + "^1": commit("9"),
		},
		files: map[string]map[string]string{second: {"a.go": "package a\n"}},
		counts: map[string][]gittree.FileCount{
			commit("0") + ".." + first:  {{Path: "a.go", Added: 1}},
			commit("9") + ".." + second: {{Path: "a.go", Added: 2, Deleted: 1}, {Path: "b.go", Added: 5}},
		},
		carrying: map[string][]string{"origin/main Goal-Item: g1-s50": {first, second}},
	}
}

func TestTheReviewedLineIsTheTipForAWaitingGoal(t *testing.T) {
	t.Parallel()
	owner := Owner{Git: waiting()}

	said := owner.ReviewedLine("g1-s64", Waiting)

	testutil.Expect(t, "origin's tip, named as the tip", said, commit("e")+" (the tip of goal/g1-s64)")
}

func TestTheReviewedLineFallsBackToTheLocalBranch(t *testing.T) {
	t.Parallel()
	git := waiting()
	delete(git.refs, "origin/goal/g1-s64")
	owner := Owner{Git: git}

	testutil.Expect(t, "the local tip", owner.ReviewedLine("g1-s64", Waiting), commit("d")+" (the tip of goal/g1-s64)")
}

func TestTheReviewedLineIsTheTrailerCommitsForADoneGoal(t *testing.T) {
	t.Parallel()
	owner := Owner{Git: done()}

	testutil.Expect(t, "the landed commits", owner.ReviewedLine("g1-s50", Done),
		commit("1")+" "+commit("2")+" (landed with Goal-Item: g1-s50)")
}

func TestTheReviewedLineSaysNoneFoundOtherwise(t *testing.T) {
	t.Parallel()
	owner := Owner{Git: done()}

	testutil.Expect(t, "a done goal with no trailer", owner.ReviewedLine("g1-s99", Done), NoneFound)
	testutil.Expect(t, "a waiting goal with no branch", owner.ReviewedLine("g1-s99", Waiting), NoneFound)
	testutil.Expect(t, "any other goal", owner.ReviewedLine("g1-s64", Elsewhere), NoneFound)
}

func TestReviewedInReadsTheHead(t *testing.T) {
	t.Parallel()
	source := "# Review of g1-s64\n\n- Kind: review\n- Goals: g1-s64\n- Reviewed: " + commit("e") +
		" (the tip of goal/g1-s64)\n- Previously: " + commit("8") + "\n\n## Findings\n\n- Reviewed: not the head\n"

	read, err := ReviewedIn(source)

	testutil.Require(t, "read the head", err, nil)
	testutil.Expect(t, "the goal", read.Goal, "g1-s64")
	testutil.Expect(t, "the tip", read.Tip, commit("e"))
	testutil.Expect(t, "no landed commits", read.Landed, []string(nil))
	testutil.Expect(t, "the earlier tip", read.Previously, []string{commit("8")})

	landed, err := ReviewedIn("- Goals: g1-s50\n- Reviewed: " + commit("1") + " " + commit("2") + " (landed with Goal-Item: g1-s50)\n")
	testutil.Require(t, "read a done goal's head", err, nil)
	testutil.Expect(t, "the landed commits", landed.Landed, []string{commit("1"), commit("2")})

	byHand, err := ReviewedIn("- Goals: g1-s50\n- Reviewed: " + commit("3") + "\n")
	testutil.Require(t, "read commits a human wrote", err, nil)
	testutil.Expect(t, "written by hand, read as landed", byHand.Landed, []string{commit("3")})

	_, err = ReviewedIn("- Goals: g1-s50\n- Reviewed: " + NoneFound + "\n")
	testutil.Expect(t, "nothing named", errors.Is(err, ErrNothingReviewed), true)
}

func TestTheChangeIndexOfAWaitingGoalIsTheMergeBaseAgainstTheTip(t *testing.T) {
	t.Parallel()
	owner := Owner{Git: waiting()}

	index, err := owner.Changes(Reviewed{Goal: "g1-s64", Tip: commit("e")})

	testutil.Require(t, "the index", err, nil)
	testutil.Expect(t, "one comparison", index.Comparisons, []Comparison{{From: commit("b"), To: commit("e")}})
	testutil.Expect(t, "the files", index.Files, []File{
		{Path: "internal/owner.go", Added: 2, Deleted: 1},
		{Path: "docs/shot.png", Binary: true},
	})
	testutil.Expect(t, "the branch now", index.Current, commit("e"))
	testutil.Expect(t, "not moved", index.Moved, false)
}

// Astra S65-02: a done goal's commits are on main, so a merge base would compare
// a commit with itself. Each Goal-Item commit is compared with its first parent,
// and the index is their combined changes — never empty for changed work.
func TestTheChangeIndexOfADoneGoalIsEachCommitAgainstItsParent(t *testing.T) {
	t.Parallel()
	owner := Owner{Git: done()}

	index, err := owner.Changes(Reviewed{Goal: "g1-s50", Landed: []string{commit("1"), commit("2")}})

	testutil.Require(t, "the index", err, nil)
	testutil.Expect(t, "one comparison per commit", index.Comparisons, []Comparison{
		{From: commit("0"), To: commit("1")}, {From: commit("9"), To: commit("2")},
	})
	testutil.Expect(t, "combined, in first-seen order", index.Files, []File{
		{Path: "a.go", Added: 3, Deleted: 1}, {Path: "b.go", Added: 5},
	})
	testutil.Expect(t, "no branch to compare", index.Current, "")
}

func TestAMovedTipIsSaidAndItsChangesAreReadBetweenTheTwoTips(t *testing.T) {
	t.Parallel()
	git := waiting()
	git.refs["origin/goal/g1-s64"] = commit("f")
	git.counts[commit("e")+".."+commit("f")] = []gittree.FileCount{{Path: "internal/owner.go", Added: 4}}
	owner := Owner{Git: git}
	reviewed := Reviewed{Goal: "g1-s64", Tip: commit("e")}

	index, err := owner.Changes(reviewed)
	testutil.Require(t, "the index", err, nil)
	testutil.Expect(t, "the branch now", index.Current, commit("f"))
	testutil.Expect(t, "moved", index.Moved, true)

	since, err := owner.ChangesSince(reviewed)
	testutil.Require(t, "what changed", err, nil)
	testutil.Expect(t, "the two tips", since.Comparisons, []Comparison{{From: commit("e"), To: commit("f")}})
	testutil.Expect(t, "the file that moved", since.Files, []File{{Path: "internal/owner.go", Added: 4}})

	_, err = owner.ChangesSince(Reviewed{Goal: "g1-s64", Tip: commit("f")})
	testutil.Expect(t, "nothing moved", err.Error(), "goal/g1-s64 has not moved since it was reviewed")
}

func TestAFilesDiffIsItsHunksWithLineNumbers(t *testing.T) {
	t.Parallel()
	owner := Owner{Git: waiting()}

	diff, err := owner.Diff(Reviewed{Goal: "g1-s64", Tip: commit("e")}, "internal/owner.go", false)

	testutil.Require(t, "the diff", err, nil)
	testutil.Require(t, "one part", len(diff.Parts), 1)
	testutil.Require(t, "one hunk", len(diff.Parts[0].Hunks), 1)
	hunk := diff.Parts[0].Hunks[0]
	testutil.Expect(t, "the header", hunk.Header, "@@ -1,4 +1,5 @@")
	testutil.Expect(t, "the lines", hunk.Lines, []DiffLine{
		{Kind: LineContext, Old: 1, New: 1, Text: "package owner"},
		{Kind: LineContext, Old: 2, New: 2, Text: ""},
		{Kind: LineDeleted, Old: 3, Text: "type owner int"},
		{Kind: LineAdded, New: 3, Text: "type owner struct{}"},
		{Kind: LineAdded, New: 4, Text: ""},
		{Kind: LineContext, Old: 4, New: 5, Text: "func begin() {}"},
	})

	binary, err := owner.Diff(Reviewed{Goal: "g1-s64", Tip: commit("e")}, "docs/shot.png", false)
	testutil.Require(t, "a binary's diff", err, nil)
	testutil.Expect(t, "said to be binary", binary.Binary, true)

	_, err = owner.Diff(Reviewed{Goal: "g1-s64", Tip: commit("e")}, "internal/other.go", false)
	testutil.Expect(t, "a path the change does not touch", err.Error(),
		"internal/other.go did not change in what this review reads")
}

func TestTheSourceReadIsBoundedAndMarksTouchedLines(t *testing.T) {
	t.Parallel()
	owner := Owner{Git: waiting()}
	reviewed := Reviewed{Goal: "g1-s64", Tip: commit("e")}

	read, err := owner.Source(reviewed, "internal/owner.go", 2, 4)

	testutil.Require(t, "the source", err, nil)
	testutil.Expect(t, "at the tip", read.Commit, commit("e"))
	testutil.Expect(t, "the whole file's length", read.Total, 5)
	testutil.Expect(t, "the range", []int{read.From, read.To}, []int{2, 4})
	testutil.Expect(t, "the lines, the changed ones marked", read.Lines, []SourceLine{
		{Number: 2, Text: ""},
		{Number: 3, Text: "type owner struct{}", Touched: true},
		{Number: 4, Text: "", Touched: true},
	})

	whole, err := owner.Source(reviewed, "internal/owner.go", 0, 0)
	testutil.Require(t, "the whole file", err, nil)
	testutil.Expect(t, "from the first line to the last", []int{whole.From, whole.To}, []int{1, 5})

	for _, refused := range []struct {
		what, path string
		from, to   int
		reason     string
	}{
		{"a path outside the tree", "../secrets", 1, 2, `"../secrets" is not a path inside the reviewed tree`},
		{"an absolute path", "/etc/passwd", 1, 2, `"/etc/passwd" is not a path inside the reviewed tree`},
		{"a file the tree does not hold", "nowhere.go", 1, 2, "nowhere.go is not in the reviewed tree at " + commit("e")[:12]},
		{"a range past the file", "internal/owner.go", 9, 12, "internal/owner.go has 5 lines; line 9 is past its end"},
		{"a range backwards", "internal/owner.go", 4, 2, "a range runs forwards: line 4 to line 2 is not one"},
		{"a binary file", "docs/shot.png", 1, 2, "docs/shot.png is a binary file, and the desk shows text"},
	} {
		_, err := owner.Source(reviewed, refused.path, refused.from, refused.to)
		if err == nil || err.Error() != refused.reason {
			t.Fatalf("%s: %v, want %q", refused.what, err, refused.reason)
		}
	}
}

func TestTheSourceReadCarriesAtMostFourHundredLines(t *testing.T) {
	t.Parallel()
	git := waiting()
	git.files[commit("e")]["long.go"] = strings.Repeat("line\n", 1000)
	owner := Owner{Git: git}

	read, err := owner.Source(Reviewed{Goal: "g1-s64", Tip: commit("e")}, "long.go", 10, 900)

	testutil.Require(t, "the source", err, nil)
	testutil.Expect(t, "four hundred lines", []int{read.From, read.To, len(read.Lines)}, []int{10, 409, 400})
	testutil.Expect(t, "the whole", read.Total, 1000)
}

func TestADoneGoalsSourceIsItsLastCommitsTree(t *testing.T) {
	t.Parallel()
	owner := Owner{Git: done()}

	read, err := owner.Source(Reviewed{Goal: "g1-s50", Landed: []string{commit("1"), commit("2")}}, "a.go", 1, 1)

	testutil.Require(t, "the source", err, nil)
	testutil.Expect(t, "the last commit", read.Commit, commit("2"))
}

// Sol SOL-A-04: a done goal's marks come from its own commits only. Another
// goal's commit landed between the two of this goal's, and the lines it wrote
// are not this goal's; where blame cannot be read nothing is marked, and the
// read says so.
func TestADoneGoalsMarksAreItsOwnCommitsLinesOnly(t *testing.T) {
	t.Parallel()
	first, foreign, second := commit("1"), commit("f"), commit("2")
	git := done()
	git.refs[second+"^1"] = foreign
	git.files[second]["a.go"] = "package a\n\nfunc mine() {}\nfunc theirs() {}\nfunc also() {}\n"
	// The old reading: the first commit's parent against the last commit, which
	// counts the foreign commit's line as added.
	git.diffs = map[string]string{commit("0") + ".." + second + " a.go": "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n" +
		"@@ -1 +1,5 @@\n package a\n+\n+func mine() {}\n+func theirs() {}\n+func also() {}\n"}
	git.blame = map[string][]string{second + " a.go": {commit("0"), first, first, foreign, second}}
	reviewed := Reviewed{Goal: "g1-s50", Landed: []string{first, second}}

	read, err := Owner{Git: git}.Source(reviewed, "a.go", 1, 5)

	testutil.Require(t, "the source", err, nil)
	marked := []int{}
	for _, line := range read.Lines {
		if line.Touched {
			marked = append(marked, line.Number)
		}
	}
	testutil.Expect(t, "the goal's own lines are marked, the foreign one is not", marked, []int{2, 3, 5})
	testutil.Expect(t, "nothing said about marks", read.Unmarked, "")

	delete(git.blame, second+" a.go")
	read, err = Owner{Git: git}.Source(reviewed, "a.go", 1, 5)
	testutil.Require(t, "the source without blame", err, nil)
	for _, line := range read.Lines {
		testutil.Expect(t, fmt.Sprintf("line %d is not marked", line.Number), line.Touched, false)
	}
	testutil.Expect(t, "the read says so", read.Unmarked, "touched lines not marked")
}

// A read refused on what it asked for is a Refusal, told apart from a read Git
// could not make, so the route can answer the one as the human's request and the
// other as the server's failure.
func TestARefusedReadIsARefusal(t *testing.T) {
	t.Parallel()
	owner := Owner{Git: waiting()}
	_, err := owner.Source(Reviewed{Goal: "g1-s64", Tip: commit("e")}, "../x", 1, 1)
	var refusal *Refusal
	testutil.Expect(t, "a path outside the tree is a refusal", errors.As(err, &refusal), true)
	_, err = Owner{Git: fakeGit{refs: map[string]string{}}}.Changes(Reviewed{Goal: "g1-s64", Tip: commit("e")})
	testutil.Expect(t, "a ref Git cannot read is not", errors.As(err, &refusal), false)
}
