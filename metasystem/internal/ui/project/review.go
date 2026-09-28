package project

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// The review record (g1-s65 D1).
//
// A review is a record of its own, kind review, beside the designs. It is not
// one of the kinds the Project page creates: the server creates it when a human
// starts a review sitting on a goal, because its head carries what only the
// server can resolve — which commits were reviewed — and a head a human typed
// would be a claim about the candidate nobody read.
//
// The head names the goal, the reviewed commits and, where a design of that
// goal names one, the evidence path. The six sections are the room's: the four
// piles a review writes, the drawings it keeps, and the Outcome that carries the
// verdict line.

// ReviewSections are the review's own sections, in the order the page opens
// with them.
var ReviewSections = []string{"Facts", "Findings", "Decisions", "Open questions", "Drawings", "Outcome"}

// NoneFound is what the Reviewed line says where the server found no commits
// to name: a goal that neither waits on a branch nor carries a landed trailer.
// It says so and asks for them, rather than naming commits nobody resolved.
const NoneFound = "none found; write them here"

// The head keys a review carries beside the ones every record has.
const (
	ReviewedKey   = "Reviewed"
	PreviouslyKey = "Previously"
	EvidenceKey   = "Evidence"
)

// NewReview is what the server resolved for one review: the goal, and what the
// Reviewed line names — the branch tip, the trailer commits space-separated, or
// NoneFound.
type NewReview struct {
	Goal     string
	Reviewed string
}

// CreateReview writes one review record in the review home at the state root
// and answers it as the pane will read it.
//
// A second review of one goal is a second record, never a write over the first:
// what was reviewed before is a record somebody may be held to.
func CreateReview(roots Roots, asked NewReview, now time.Time) (Written, error) {
	goal := strings.TrimSpace(asked.Goal)
	if goal == "" {
		return Written{}, refuse(RefusalBad, "a review names the goal it reviews")
	}
	reviewed := normalizeSpace(asked.Reviewed)
	if reviewed == "" {
		reviewed = NoneFound
	}
	read, err := resolver.Read(resolver.Roots(roots))
	if err != nil {
		return Written{}, err
	}
	if _, refusal := ledgerGoals(read, []string{goal}); refusal != nil {
		return Written{}, refusal
	}
	home, found := homeFor(roots, resolver.KindReview)
	if !found {
		return Written{}, fmt.Errorf("this project has no home for a %s record", resolver.KindReview)
	}
	title := "Review of " + goal
	name, err := freeName(home, slug(title))
	if err != nil {
		return Written{}, err
	}
	absolute := filepath.Join(home.Path, name+".md")
	relative := path.Join(home.Rel, name+".md")
	if err := within(home.Path, absolute); err != nil {
		return Written{}, err
	}
	if err := within(roots.StateRoot, absolute); err != nil {
		return Written{}, err
	}
	id, err := resolver.NewID()
	if err != nil {
		return Written{}, fmt.Errorf("cannot mint an id: %w", err)
	}
	if read.Record(id) != nil {
		return Written{}, refuse(RefusalExists, "the id "+id+" is already declared")
	}
	text := reviewPage(title, id, goal, reviewed, evidenceFor(read, goal))
	if problems := wouldRefuse(read, relative, text); len(problems) > 0 {
		return Written{}, &Refusal{
			Kind:     RefusalBad,
			Message:  "the record this would write is one the project refuses",
			Problems: problems,
		}
	}
	if _, err := atomicfile.WriteText(absolute, text, roots.Checkout); err != nil {
		return Written{}, fmt.Errorf("cannot write %s: %w", relative, err)
	}
	return writtenRecord(roots, id, relative, absolute)
}

// freeName is the first file name in the home that nothing holds: the slug, then
// the slug with -2, -3 and so on after it.
func freeName(home resolver.Home, base string) (string, error) {
	for attempt := 1; attempt < 1000; attempt++ {
		name := base
		if attempt > 1 {
			name = base + "-" + strconv.Itoa(attempt)
		}
		if _, err := os.Lstat(filepath.Join(home.Path, name+".md")); os.IsNotExist(err) {
			return name, nil
		} else if err != nil {
			return "", fmt.Errorf("cannot look at %s: %w", path.Join(home.Rel, name+".md"), err)
		}
	}
	return "", refuse(RefusalExists, "every review name for this goal is taken")
}

// evidenceFor is the path a design of this goal names on its Evidence line, or
// "" where none names one. The first design in the resolver's own order wins,
// which is the order the pane lists them in.
func evidenceFor(read *resolver.Project, goal string) string {
	for _, record := range read.Records {
		if record.Kind != resolver.KindDesign || !includes(record.Goals, goal) {
			continue
		}
		if field, found := headField(record, EvidenceKey); found && strings.TrimSpace(field.Value) != "" {
			return strings.TrimSpace(field.Value)
		}
	}
	return ""
}

// reviewPage is the whole file: the head, then the six empty sections.
func reviewPage(title, id, goal, reviewed, evidence string) string {
	var out strings.Builder
	out.WriteString("# " + title + "\n\n")
	out.WriteString("- Kind: " + resolver.KindReview + "\n")
	out.WriteString("- Id: " + id + "\n")
	out.WriteString("- Status: " + resolver.StatusDraft + "\n")
	out.WriteString("- Goals: " + goal + "\n")
	out.WriteString("- " + ReviewedKey + ": " + reviewed + "\n")
	if evidence != "" {
		out.WriteString("- " + EvidenceKey + ": " + evidence + "\n")
	}
	for _, section := range ReviewSections {
		out.WriteString("\n## " + section + "\n")
	}
	return out.String()
}
