package uitools

import (
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
)

// What the review sitting adds to this server (g1-s65).
//
// One read and one offer. The changes read is the desk's own: it reads a review
// record's head and answers from the same review owner the human's desk reads
// through, so the colleague and the human read one tree — the candidate's, not
// this checkout (D4). The present offer puts one thing on a sitting's desk while
// the Partner explains it (D5). It reads nothing and navigates nowhere: it says
// what it prepared, and the human's own server decides whether there is a desk to
// put it on.

// Reviewing is the review owner's two reads this server answers from.
type Reviewing interface {
	Changes(review.Reviewed) (review.Index, error)
	Diff(review.Reviewed, string, bool) (review.Diff, error)
}

// The fixed form a desk item travels back in, read once by the interface's
// host.
const (
	PresentHeader  = "Desk: "
	PresentPath    = "Path: "
	PresentFrom    = "From: "
	PresentTo      = "To: "
	PresentSection = "Section: "
)

// PresentedLine says what preparing a desk item is and is not.
const PresentedLine = "prepared for the desk; the human sees it there beside your answer unless they stopped " +
	"the walk, and nothing else on their screen moves"

// The four things a desk shows.
var presentKinds = []string{"source", "changes", "diff", "section"}

// present prepares one desk item, or refuses the call in words.
func present(args Args) Result {
	kind := strings.ToLower(oneLine(args.Text("kind")))
	path := oneLine(args.Text("path"))
	section := oneLine(args.Text("section"))
	known := false
	for _, one := range presentKinds {
		known = known || one == kind
	}
	if !known {
		return refusedCall("a desk item is source, changes, diff or section; " + strconv.Quote(kind) +
			" is none of them")
	}
	built := PresentHeader + kind + "\n"
	switch kind {
	case "section":
		if path == "" || section == "" {
			return refusedCall("a section item names the record and the heading")
		}
		built += PresentPath + path + "\n" + PresentSection + section + "\n"
	case "source", "diff":
		if path == "" {
			return refusedCall("a " + kind + " item names the path of a file of the reviewed tree")
		}
		if !insideTree(path) {
			return refusedCall(strconv.Quote(path) + " is not a path inside the reviewed tree")
		}
		built += PresentPath + path + "\n"
		if kind == "source" {
			from, refusal := lineArg(args, "from")
			if refusal != "" {
				return refusedCall(refusal)
			}
			to, refusal := lineArg(args, "to")
			if refusal != "" {
				return refusedCall(refusal)
			}
			if from > 0 && to > 0 && to < from {
				return refusedCall("a range runs forwards: line " + strconv.Itoa(from) + " to line " +
					strconv.Itoa(to) + " is not one")
			}
			if from > 0 {
				built += PresentFrom + strconv.Itoa(from) + "\n"
			}
			if to > 0 {
				built += PresentTo + strconv.Itoa(to) + "\n"
			}
		}
	}
	return Result{Prepared: PresentedLine + "\n" + built}
}

func lineArg(args Args, name string) (int, string) {
	said := oneLine(args.Text(name))
	if said == "" {
		return 0, ""
	}
	number, err := strconv.Atoi(said)
	if err != nil || number < 1 {
		return 0, name + " is a line number, and " + strconv.Quote(said) + " is not one"
	}
	return number, ""
}

func insideTree(path string) bool {
	return !strings.HasPrefix(path, "/") && path != ".." && !strings.HasPrefix(path, "../") &&
		!strings.Contains(path, "/../") && !strings.HasSuffix(path, "/..")
}

// changes answers the changes read: the index of what one review record reviews,
// or one file's hunks, or with since the change between the reviewed tip and the
// branch now.
func (r Readers) changes(args Args) Result {
	record := oneLine(args.Text("record"))
	if record == "" {
		return Result{Problem: "this tool needs the review record's checkout-relative path"}
	}
	if r.Document == nil || r.Review == nil {
		return Result{Problem: "this build has no review reader"}
	}
	document, err := r.Document(record)
	if err != nil {
		return Result{Problem: err.Error()}
	}
	if document.Record == nil || document.Record.Kind != "review" {
		return Result{Problem: record + " is not a review record"}
	}
	reviewed, err := review.ReviewedIn(document.Source)
	if err != nil {
		return Result{Problem: err.Error()}
	}
	owner, err := r.Review()
	if err != nil {
		return Result{Problem: err.Error()}
	}
	source := reviewedSource(reviewed)
	since := strings.EqualFold(oneLine(args.Text("since")), "true")
	if path := oneLine(args.Text("path")); path != "" {
		diff, err := owner.Diff(reviewed, path, since)
		if err != nil {
			return Result{Source: source, Problem: err.Error()}
		}
		return Result{Source: source, Supplied: diff.Supplied, Total: diff.Total, Body: diffText(diff)}
	}
	index, err := owner.Changes(reviewed)
	if err != nil {
		return Result{Source: source, Problem: err.Error()}
	}
	lines := []string{}
	if index.Moved {
		lines = append(lines, "- The branch has moved to "+short(index.Current)+" since the tip this review names.")
	}
	for _, file := range index.Files {
		if file.Binary {
			lines = append(lines, "- "+file.Path+" (binary)")
			continue
		}
		lines = append(lines, "- "+file.Path+" +"+strconv.FormatInt(file.Added, 10)+" -"+strconv.FormatInt(file.Deleted, 10))
	}
	return Result{Source: source, Supplied: index.Supplied, Total: index.Total, Body: strings.Join(lines, "\n") + "\n"}
}

func reviewedSource(reviewed review.Reviewed) string {
	if reviewed.Tip != "" {
		return review.Branch(reviewed.Goal) + " at " + short(reviewed.Tip) + ", the tip this review names"
	}
	named := make([]string, 0, len(reviewed.Landed))
	for _, one := range reviewed.Landed {
		named = append(named, short(one))
	}
	return "the landed commits " + strings.Join(named, " ") + ", each against its first parent"
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func diffText(diff review.Diff) string {
	var built strings.Builder
	if diff.Binary {
		built.WriteString(diff.Path + " is a binary file; its change is not shown as lines.\n")
	}
	for _, part := range diff.Parts {
		for _, hunk := range part.Hunks {
			built.WriteString(hunk.Header + "\n")
			for _, line := range hunk.Lines {
				switch line.Kind {
				case review.LineAdded:
					built.WriteString("+" + line.Text + "\n")
				case review.LineDeleted:
					built.WriteString("-" + line.Text + "\n")
				default:
					built.WriteString(" " + line.Text + "\n")
				}
			}
		}
	}
	return built.String()
}
