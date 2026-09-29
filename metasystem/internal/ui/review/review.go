// Package review is the owner of what a human's review of a goal reads: the
// candidate's own tree, not the seat's checkout (g1-s65 D4).
//
// Three reads over a review record's Reviewed line, and one resolution that
// writes that line at Start. What is compared depends on the subject. A goal
// waiting to land is its branch at origin: the merge base of the canonical
// branch and the tip, against the tip, and the tip's tree for source reads. A
// done goal is already on the canonical branch, so a merge base would compare a
// commit with itself and show nothing (Astra S65-02): each Goal-Item commit is
// compared with its first parent, in order, combined, and the last commit's
// tree is what the source reads.
//
// The same owner serves the Partner's changes operation, so the colleague and
// the human read one tree. Nothing here writes, and every read is bounded and
// says how much of the whole it carried.
package review

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// Git is the reads this owner makes. gittree.Workspace over the checkout is the
// one every run uses; a test hands in a table.
type Git interface {
	ResolveCommit(rev string) (string, error)
	MergeBases(left, right string) ([]string, error)
	TreeOf(rev string) (string, error)
	FileAt(tree, path string) ([]byte, bool, error)
	FileCounts(from, to string) ([]gittree.FileCount, error)
	PathDiff(from, to, path string) ([]byte, error)
	CommitsCarrying(ref, line string) ([]string, error)
	LineCommits(rev, path string) ([]string, error)
	// FetchBranch brings one branch at origin into origin/BRANCH.
	FetchBranch(branch string) error
}

// Owner reads one checkout's candidates, and the checkout as it stands for a
// sitting that shapes a record (g1-s67 D2).
type Owner struct {
	Git Git
	// Canonical is the branch a goal lands on. Empty is main.
	Canonical string
	// Checkout is the root the document reader opens: a shaping desk's source
	// reads are opened beneath it and nowhere else.
	Checkout string
	// Home is where a review's Evidence line's ~ leads (g1-s71 D4). Empty is
	// the account's own home directory.
	Home string
}

// The bounds. A source read carries at most four hundred lines, the change
// index at most five hundred files, and one file's diff at most three thousand
// lines; each answer says what the whole was.
const (
	MaxSourceLines = 400
	MaxFiles       = 500
	MaxDiffLines   = 3000
	// binaryProbe is how much of a file is looked at for a NUL, which is
	// Git's own rule for calling a file binary.
	binaryProbe = 8000
	// maxCheckoutBytes bounds what one read of the checkout takes in: a source
	// file past it is not one a desk shows.
	maxCheckoutBytes = 8 << 20
)

// NoneFound is the Reviewed line of a review whose subject names no commits.
// It is the record package's own words, repeated here because this package
// reads the line the record creator writes.
const NoneFound = "none found; write them here"

// Where a goal stands, which decides what a review of it reads.
type Standing int

const (
	// Elsewhere is every lane but the two below: nothing is resolved.
	Elsewhere Standing = iota
	// Waiting is a goal built and waiting to land, on its branch.
	Waiting
	// Done is a goal whose work landed, carrying Goal-Item trailers.
	Done
)

// ErrNothingReviewed is a review whose Reviewed line names no commit yet.
var ErrNothingReviewed error = &Refusal{Reason: "this review names no commits yet; write them on its Reviewed line"}

// Refusal is a read refused on what it asked for — a path outside the tree, a
// range past the file, a binary file — as distinct from a read Git could not
// make. The route answers one as a bad request in its own words.
type Refusal struct{ Reason string }

func (r *Refusal) Error() string { return r.Reason }

func refused(format string, args ...any) error {
	return &Refusal{Reason: fmt.Sprintf(format, args...)}
}

var commitID = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

func (o Owner) canonical() string {
	if strings.TrimSpace(o.Canonical) == "" {
		return "main"
	}
	return o.Canonical
}

// resolved is the commit a ref names, origin first: a goal's candidate is what
// its branch holds at origin, and the local branch is read only where origin
// has none (the launch contract's own rule, cmd/metasystem/app.go).
func (o Owner) resolved(ref string) (string, string, error) {
	for _, candidate := range []string{"origin/" + ref, ref} {
		if commit, err := o.Git.ResolveCommit(candidate); err == nil {
			return commit, candidate, nil
		}
	}
	return "", "", fmt.Errorf("no commit is named by %s", ref)
}

// Branch is a goal's branch.
func Branch(goal string) string { return "goal/" + goal }

// ReviewedLine is what a new review of one goal names on its Reviewed line.
func (o Owner) ReviewedLine(goal string, standing Standing) string {
	switch standing {
	case Waiting:
		if tip, _, err := o.resolved(Branch(goal)); err == nil {
			return tip + " (the tip of " + Branch(goal) + ")"
		}
	case Done:
		_, ref, err := o.resolved(o.canonical())
		if err != nil {
			return NoneFound
		}
		commits, err := o.Git.CommitsCarrying(ref, trailer(goal))
		if err == nil && len(commits) > 0 {
			return strings.Join(commits, " ") + " (landed with " + trailer(goal) + ")"
		}
	}
	return NoneFound
}

func trailer(goal string) string { return "Goal-Item: " + goal }

// Reviewed is a review record's head, as this owner reads it.
type Reviewed struct {
	Goal string
	// Tip is the branch tip a waiting goal was reviewed at, or "".
	Tip string
	// Landed is a done goal's commits, oldest first, or nil.
	Landed []string
	// Previously is the tips reviewed before the one on the Reviewed line.
	Previously []string
}

// ReviewedIn reads a review record's head out of its whole source. The head is
// the list before the first section; a line of the same shape inside a section
// is somebody's words and not the head.
func ReviewedIn(source string) (Reviewed, error) {
	read := Reviewed{}
	said := ""
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(line, "## ") {
			break
		}
		key, value, isHead := headLine(line)
		if !isHead {
			continue
		}
		switch strings.ToLower(key) {
		case "goals":
			if fields := strings.Fields(value); len(fields) > 0 {
				read.Goal = fields[0]
			}
		case "reviewed":
			said = value
		case "previously":
			read.Previously = commitsIn(value)
		}
	}
	commits := commitsIn(said)
	switch {
	case len(commits) == 0:
		return read, ErrNothingReviewed
	case strings.Contains(said, "(the tip of"):
		read.Tip = commits[0]
	default:
		read.Landed = commits
	}
	return read, nil
}

func headLine(line string) (string, string, bool) {
	rest, listed := strings.CutPrefix(line, "- ")
	if !listed {
		return "", "", false
	}
	key, value, found := strings.Cut(rest, ":")
	if !found || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	return key, strings.TrimSpace(value), true
}

func commitsIn(said string) []string {
	var found []string
	for _, field := range strings.Fields(said) {
		if commitID.MatchString(field) {
			found = append(found, field)
		}
	}
	return found
}

/* ----------------------------------------------------------- comparisons -- */

// Comparison is one pair of commits the change is read between.
type Comparison struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// comparisons are what a review reads, and the commit its source reads come
// from.
func (o Owner) comparisons(reviewed Reviewed) ([]Comparison, string, error) {
	if reviewed.Tip != "" {
		main, ref, err := o.resolved(o.canonical())
		if err != nil {
			return nil, "", err
		}
		bases, err := o.Git.MergeBases(main, reviewed.Tip)
		if err != nil || len(bases) == 0 {
			return nil, "", fmt.Errorf("%s and %s share no history to compare", ref, short(reviewed.Tip))
		}
		return []Comparison{{From: bases[0], To: reviewed.Tip}}, reviewed.Tip, nil
	}
	if len(reviewed.Landed) == 0 {
		return nil, "", ErrNothingReviewed
	}
	pairs := make([]Comparison, 0, len(reviewed.Landed))
	for _, landed := range reviewed.Landed {
		parent, err := o.Git.ResolveCommit(landed + "^1")
		if err != nil {
			return nil, "", fmt.Errorf("%s has no first parent to compare it with", short(landed))
		}
		pairs = append(pairs, Comparison{From: parent, To: landed})
	}
	return pairs, reviewed.Landed[len(reviewed.Landed)-1], nil
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

/* ------------------------------------------------------------- the index -- */

// File is one path of the change with its counts.
type File struct {
	Path    string `json:"path"`
	Added   int64  `json:"added"`
	Deleted int64  `json:"deleted"`
	Binary  bool   `json:"binary,omitempty"`
}

// Index is the change as a whole: every file it touched, with counts, and the
// comparisons it was read from.
type Index struct {
	Goal        string       `json:"goal"`
	Comparisons []Comparison `json:"comparisons"`
	Files       []File       `json:"files"`
	Supplied    int          `json:"supplied"`
	Total       int          `json:"total"`
	// Current is the branch tip now, for a goal reviewed at a tip, and "" for
	// a done goal or a branch that cannot be read. Moved says it is not the
	// tip the record names.
	Current string `json:"current"`
	Moved   bool   `json:"moved"`
}

// Changes is the change index of what the record reviews.
func (o Owner) Changes(reviewed Reviewed) (Index, error) {
	pairs, _, err := o.comparisons(reviewed)
	if err != nil {
		return Index{}, err
	}
	index, err := o.indexOf(pairs)
	if err != nil {
		return Index{}, err
	}
	index.Goal = reviewed.Goal
	if reviewed.Tip != "" && reviewed.Goal != "" {
		if now, _, err := o.resolved(Branch(reviewed.Goal)); err == nil {
			index.Current = now
			index.Moved = now != reviewed.Tip
		}
	}
	return index, nil
}

// ChangesSince is what changed on the branch since the tip the record names:
// the diff between the two tips, which is what Show what changed puts on the
// desk. A branch that has not moved has nothing to show, and says so.
func (o Owner) ChangesSince(reviewed Reviewed) (Index, error) {
	pair, err := o.moved(reviewed)
	if err != nil {
		return Index{}, err
	}
	index, err := o.indexOf([]Comparison{pair})
	if err != nil {
		return Index{}, err
	}
	index.Goal, index.Current, index.Moved = reviewed.Goal, pair.To, true
	return index, nil
}

func (o Owner) moved(reviewed Reviewed) (Comparison, error) {
	if reviewed.Tip == "" {
		return Comparison{}, refused("a review of landed commits has no branch tip to move")
	}
	now, _, err := o.resolved(Branch(reviewed.Goal))
	if err != nil {
		return Comparison{}, err
	}
	if now == reviewed.Tip {
		return Comparison{}, refused("%s has not moved since it was reviewed", Branch(reviewed.Goal))
	}
	return Comparison{From: reviewed.Tip, To: now}, nil
}

func (o Owner) indexOf(pairs []Comparison) (Index, error) {
	files := []File{}
	at := map[string]int{}
	for _, pair := range pairs {
		counts, err := o.Git.FileCounts(pair.From, pair.To)
		if err != nil {
			return Index{}, err
		}
		for _, count := range counts {
			where, seen := at[count.Path]
			if !seen {
				at[count.Path] = len(files)
				files = append(files, File{Path: count.Path})
				where = len(files) - 1
			}
			files[where].Added += count.Added
			files[where].Deleted += count.Deleted
			files[where].Binary = files[where].Binary || count.Binary
		}
	}
	total := len(files)
	if len(files) > MaxFiles {
		files = files[:MaxFiles]
	}
	return Index{Comparisons: pairs, Files: files, Supplied: len(files), Total: total}, nil
}

/* -------------------------------------------------------------- the diff -- */

// The three kinds of line a hunk carries.
const (
	LineContext = "context"
	LineAdded   = "added"
	LineDeleted = "deleted"
)

// DiffLine is one line of a hunk, numbered on the side or sides it stands on.
type DiffLine struct {
	Kind string `json:"kind"`
	Old  int    `json:"old,omitempty"`
	New  int    `json:"new,omitempty"`
	Text string `json:"text"`
}

// Hunk is one hunk of a file's diff.
type Hunk struct {
	Header string     `json:"header"`
	Lines  []DiffLine `json:"lines"`
}

// Part is one comparison's hunks for the file.
type Part struct {
	Comparison
	Hunks []Hunk `json:"hunks"`
}

// Diff is one file's change, as hunks, across the comparisons that touched it.
type Diff struct {
	Path     string `json:"path"`
	Binary   bool   `json:"binary,omitempty"`
	Parts    []Part `json:"parts"`
	Supplied int    `json:"supplied"`
	Total    int    `json:"total"`
}

// Diff is one file's hunks: across what the record reviews, or, with since,
// between the reviewed tip and the branch now.
func (o Owner) Diff(reviewed Reviewed, file string, since bool) (Diff, error) {
	clean, err := inside(file)
	if err != nil {
		return Diff{}, err
	}
	pairs := []Comparison{}
	if since {
		pair, err := o.moved(reviewed)
		if err != nil {
			return Diff{}, err
		}
		pairs = append(pairs, pair)
	} else if pairs, _, err = o.comparisons(reviewed); err != nil {
		return Diff{}, err
	}
	diff := Diff{Path: clean, Parts: []Part{}}
	touched := false
	for _, pair := range pairs {
		counts, err := o.Git.FileCounts(pair.From, pair.To)
		if err != nil {
			return Diff{}, err
		}
		count, found := countOf(counts, clean)
		if !found {
			continue
		}
		touched = true
		if count.Binary {
			diff.Binary = true
			diff.Parts = append(diff.Parts, Part{Comparison: pair, Hunks: []Hunk{}})
			continue
		}
		patch, err := o.Git.PathDiff(pair.From, pair.To, clean)
		if err != nil {
			return Diff{}, err
		}
		hunks := hunksOf(string(patch))
		for _, hunk := range hunks {
			diff.Total += len(hunk.Lines)
		}
		diff.Parts = append(diff.Parts, Part{Comparison: pair, Hunks: hunks})
	}
	if !touched {
		return Diff{}, refused("%s did not change in what this review reads", clean)
	}
	diff.Supplied = bound(diff.Parts, MaxDiffLines)
	return diff, nil
}

// bound cuts the parts' hunks at a whole line once the bound is reached, and
// answers how many lines were kept.
func bound(parts []Part, most int) int {
	kept := 0
	for at := range parts {
		for index := range parts[at].Hunks {
			left := most - kept
			if left <= 0 {
				parts[at].Hunks = parts[at].Hunks[:index]
				break
			}
			if lines := parts[at].Hunks[index].Lines; len(lines) > left {
				parts[at].Hunks[index].Lines = lines[:left]
			}
			kept += len(parts[at].Hunks[index].Lines)
		}
	}
	return kept
}

func countOf(counts []gittree.FileCount, file string) (gittree.FileCount, bool) {
	for _, count := range counts {
		if count.Path == file {
			return count, true
		}
	}
	return gittree.FileCount{}, false
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// hunksOf reads a unified diff's hunks, numbering each line on its sides.
func hunksOf(patch string) []Hunk {
	hunks := []Hunk{}
	old, fresh := 0, 0
	for _, line := range strings.Split(patch, "\n") {
		if found := hunkHeader.FindStringSubmatch(line); found != nil {
			old, _ = strconv.Atoi(found[1])
			fresh, _ = strconv.Atoi(found[2])
			hunks = append(hunks, Hunk{Header: found[0], Lines: []DiffLine{}})
			continue
		}
		if len(hunks) == 0 || line == "" {
			continue
		}
		current := &hunks[len(hunks)-1]
		switch line[0] {
		case ' ':
			current.Lines = append(current.Lines, DiffLine{Kind: LineContext, Old: old, New: fresh, Text: line[1:]})
			old++
			fresh++
		case '+':
			current.Lines = append(current.Lines, DiffLine{Kind: LineAdded, New: fresh, Text: line[1:]})
			fresh++
		case '-':
			current.Lines = append(current.Lines, DiffLine{Kind: LineDeleted, Old: old, Text: line[1:]})
			old++
		}
	}
	return hunks
}

/* ------------------------------------------------------------ the source -- */

// SourceLine is one line of a source read, marked where the change touched it.
type SourceLine struct {
	Number  int    `json:"number"`
	Text    string `json:"text"`
	Touched bool   `json:"touched,omitempty"`
}

// Source is one text file of the reviewed tree at a range of lines, or of the
// checkout as it stands, where Checkout says so and Commit is "".
type Source struct {
	Path     string `json:"path"`
	Commit   string `json:"commit"`
	Checkout bool   `json:"checkout,omitempty"`
	// Head is the checkout's head when a checkout read was made, "" where it
	// could not be read: a remark made on these lines keeps it as provenance
	// only, never as a pin (g1-s71 D1).
	Head  string       `json:"head,omitempty"`
	From  int          `json:"from"`
	To    int          `json:"to"`
	Total int          `json:"total"`
	Lines []SourceLine `json:"lines"`
	// Unmarked says the touched lines could not be established and none are
	// marked; "" where the marks stand.
	Unmarked string `json:"unmarked,omitempty"`
}

// Source reads one file of the reviewed tree from line from to line to, both
// counted from one and both included. Zero for from is the first line, and
// zero for to is as far as the bound allows.
func (o Owner) Source(reviewed Reviewed, file string, from, to int) (Source, error) {
	clean, err := inside(file)
	if err != nil {
		return Source{}, err
	}
	pairs, at, err := o.comparisons(reviewed)
	if err != nil {
		return Source{}, err
	}
	tree, err := o.Git.TreeOf(at)
	if err != nil {
		return Source{}, err
	}
	body, found, err := o.Git.FileAt(tree, clean)
	if err != nil {
		return Source{}, err
	}
	if !found {
		return Source{}, refused("%s is not in the reviewed tree at %s", clean, short(at))
	}
	if binary(body) {
		return Source{}, refused("%s is a binary file, and the desk shows text", clean)
	}
	touched, unmarked := o.touched(reviewed, pairs, at, clean)
	read, err := ranged(clean, body, from, to, touched)
	if err != nil {
		return Source{}, err
	}
	read.Commit, read.Unmarked = at, unmarked
	return read, nil
}

// AsItStands reads one file of the checkout as it stands, uncommitted edits
// included, from line from to line to (g1-s67 D2): the desk of a sitting that
// shapes a record reads the same bytes the Partner's own reads see. The file is
// opened beneath the checkout root, so a name that leaves it — a step out or a
// link — is refused on the open and never followed; the path check, the binary
// refusal and the bounds are Source's own. What the interface never serves is
// refused by the document reader's own rule, on the name asked and on the path
// it resolves to, and bytes that are not UTF-8 are refused rather than shown as
// other characters. Nothing is marked, because nothing is
// compared.
func (o Owner) AsItStands(file string, from, to int) (Source, error) {
	clean, err := insideOf(file, "the checkout")
	if err != nil {
		return Source{}, err
	}
	if strings.TrimSpace(o.Checkout) == "" {
		return Source{}, errors.New("this reader was given no checkout to read")
	}
	if !project.Served(clean) {
		return Source{}, refused("%s is a file this interface never serves", clean)
	}
	resolved, ok := resolvedIn(o.Checkout, clean)
	if !ok {
		return Source{}, refused("%s is not a file of the checkout", clean)
	}
	if !project.Served(resolved) {
		return Source{}, refused("%s is a file this interface never serves", clean)
	}
	root, err := os.OpenRoot(o.Checkout)
	if err != nil {
		return Source{}, fmt.Errorf("cannot open the checkout at %s: %w", o.Checkout, err)
	}
	defer func() { _ = root.Close() }()
	// The resolved path is the one opened, and the descriptor must be the file
	// that path names, so a link put in its place after the judgment is refused.
	opened, err := root.Open(resolved)
	if err != nil {
		return Source{}, refused("%s is not a file of the checkout", clean)
	}
	defer func() { _ = opened.Close() }()
	info, err := opened.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Source{}, refused("%s is not a file of the checkout", clean)
	}
	if named, err := root.Lstat(resolved); err != nil || !os.SameFile(info, named) {
		return Source{}, refused("%s is not a file of the checkout", clean)
	}
	body, err := io.ReadAll(io.LimitReader(opened, maxCheckoutBytes+1))
	if err != nil {
		return Source{}, fmt.Errorf("cannot read %s: %w", clean, err)
	}
	if len(body) > maxCheckoutBytes {
		return Source{}, refused("%s is larger than the desk reads", clean)
	}
	if !binary(body) && !utf8.Valid(body) {
		return Source{}, refused("%s is not UTF-8 text, and the desk shows text as it is", clean)
	}
	read, err := ranged(clean, body, from, to, nil)
	if err != nil {
		return Source{}, err
	}
	read.Checkout = true
	if o.Git != nil {
		read.Head, _ = o.Git.ResolveCommit("HEAD")
	}
	return read, nil
}

// ranged is one text file's lines from line from to line to, with the marks
// given, held to the bounds: Source's and AsItStands' one reading of a body.
func ranged(clean string, body []byte, from, to int, touched map[int]bool) (Source, error) {
	if binary(body) {
		return Source{}, refused("%s is a binary file, and the desk shows text", clean)
	}
	lines := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
	total := len(lines)
	if from <= 0 {
		from = 1
	}
	if to <= 0 {
		to = from + MaxSourceLines - 1
	}
	switch {
	case to < from:
		return Source{}, refused("a range runs forwards: line %d to line %d is not one", from, to)
	case from > total:
		return Source{}, refused("%s has %d lines; line %d is past its end", clean, total, from)
	}
	if to-from+1 > MaxSourceLines {
		to = from + MaxSourceLines - 1
	}
	if to > total {
		to = total
	}
	read := Source{Path: clean, From: from, To: to, Total: total, Lines: make([]SourceLine, 0, to-from+1)}
	for number := from; number <= to; number++ {
		read.Lines = append(read.Lines, SourceLine{Number: number, Text: lines[number-1], Touched: touched[number]})
	}
	return read, nil
}

// Unmarked is what a source read says where its touched lines could not be
// established.
const Unmarked = "touched lines not marked"

// touched is the lines of the source commit's file that the change wrote. A
// goal waiting to land is its merge base against its tip. A done goal's commits
// may have another goal's between them, so its marks come from its own commits
// only (Sol SOL-A-04): a line is marked where the commit that last touched it at
// the last landed commit is one of the goal's. Where that cannot be read,
// nothing is marked and the read says so.
func (o Owner) touched(reviewed Reviewed, pairs []Comparison, at, file string) (map[int]bool, string) {
	marked := map[int]bool{}
	if len(reviewed.Landed) > 0 {
		blamed, err := o.Git.LineCommits(at, file)
		if err != nil {
			return marked, Unmarked
		}
		own := map[string]bool{}
		for _, landed := range reviewed.Landed {
			own[landed] = true
		}
		for index, commit := range blamed {
			if own[commit] {
				marked[index+1] = true
			}
		}
		return marked, ""
	}
	if len(pairs) == 0 {
		return marked, ""
	}
	patch, err := o.Git.PathDiff(pairs[0].From, at, file)
	if err != nil {
		return marked, ""
	}
	for _, hunk := range hunksOf(string(patch)) {
		for _, line := range hunk.Lines {
			if line.Kind == LineAdded {
				marked[line.New] = true
			}
		}
	}
	return marked, ""
}

// inside is a path of the reviewed tree, or the refusal that names it as not
// one: relative, with no step out of the tree.
func inside(file string) (string, error) {
	return insideOf(file, "the reviewed tree")
}

// insideOf is inside for the tree it names in its refusal.
func insideOf(file, tree string) (string, error) {
	trimmed := strings.TrimSpace(file)
	clean := path.Clean(trimmed)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") || clean == "." || clean == ".." ||
		strings.HasPrefix(clean, "../") || strings.ContainsRune(trimmed, 0) {
		return "", refused("%q is not a path inside %s", file, tree)
	}
	return clean, nil
}

// resolvedIn is the checkout-relative path a clean name reaches once every
// link on the way is followed, and false where it reaches nothing or leaves the
// checkout.
func resolvedIn(checkout, clean string) (string, bool) {
	base, err := filepath.EvalSymlinks(checkout)
	if err != nil {
		return "", false
	}
	target, err := filepath.EvalSymlinks(filepath.Join(base, filepath.FromSlash(clean)))
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(base, target)
	if err != nil {
		return "", false
	}
	relative = filepath.ToSlash(relative)
	if relative == "." || relative == ".." || strings.HasPrefix(relative, "../") {
		return "", false
	}
	return relative, true
}

func binary(body []byte) bool {
	probe := body
	if len(probe) > binaryProbe {
		probe = probe[:binaryProbe]
	}
	return strings.ContainsRune(string(probe), 0)
}

// The workspace every run reads through is a Git of this owner's shape.
var _ Git = gittree.Workspace{}

// BranchTip is the commit a goal's branch holds, origin first, as a review of
// it would record it: the tip a decision to land without a sitting binds
// (g1-s70 D4).
func (o Owner) BranchTip(goal string) (string, error) {
	tip, _, err := o.resolved(Branch(goal))
	return tip, err
}

// BranchTipAtOrigin is BranchTip read after fetching the branch from origin,
// for the gate's reading of a word to land (g1-s70 G5): a branch that moved at
// origin since this checkout last fetched reads as moved. A fetch that fails
// reads the local ref as BranchTip does.
func (o Owner) BranchTipAtOrigin(goal string) (string, error) {
	_ = o.Git.FetchBranch(Branch(goal))
	return o.BranchTip(goal)
}
