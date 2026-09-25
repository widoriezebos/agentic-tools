// Package rulings reads the standing rulings register, memory/rulings.md.
//
// One reader, two views. The register is one row per human ruling — id, date,
// the ruling as close to verbatim as the session allowed, its context, the
// accountable owner, and an optional review condition — and two readers want
// two different things out of it. The steward's sweep wants the valid
// scheduled subset, so that it can surface a review that is due; a page that
// shows a human what they have decided wants every row whole, in the human's
// own words. Both are answered from one read, so the two can never disagree
// about what the register says.
//
// Nothing here widens the grammar and nothing here repairs the register. The
// acceptance rules and the defect wording are the steward's own, lifted
// unchanged: a row the parser refuses is a defect in the steward's words, and
// the sweep's digest says exactly what it said before. A row whose review
// condition is prose rather than the scheduled form is a row this package
// carries whole with no class, and a defect beside it, because that is what
// the parser already said about it.
//
// It reads one file and decides nothing else. Whether a review is due, what a
// ruling mentions, and what any of it means are questions for the callers.
package rulings

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The four classes a scheduled review may declare. `standing` is prose, not a
// class: a ruling that stands until further notice carries no schedule, and
// the parser refuses a condition that names one of these and nothing else.
const (
	ClassTemporary           = "temporary"
	ClassExperimental        = "experimental"
	ClassDelegatedAuthority  = "delegated-authority"
	ClassAssumptionDependent = "assumption-dependent"
)

// DateLayout is how the register writes a due date, and the only form the
// parser accepts.
const DateLayout = "2006-01-02"

// Row is one row of the register, whole: what the human decided, in the words
// the register keeps, with the raw review condition beside them.
//
// Class, Due and Event are filled only where the condition parses as a
// scheduled review. A blank condition and a condition the parser refuses both
// leave them empty, and Condition still carries what the register wrote, so a
// reader shows the row as it stands rather than as this package could read it.
type Row struct {
	ID    string `json:"id"`
	Date  string `json:"date"`
	Words string `json:"words"`
	// Context is the row's own fifth column: why the ruling was given.
	Context string `json:"context"`
	Owner   string `json:"owner"`
	// Condition is the review column exactly as written: blank, prose, or the
	// scheduled form.
	Condition string `json:"condition"`
	Class     string `json:"class"`
	Due       string `json:"due"`
	Event     string `json:"event"`
}

// Review is one valid scheduled review: the subset the steward's sweep
// consumes, and nothing more of the row than the sweep reads.
type Review struct {
	ID    string
	Owner string
	Class string
	Due   string
	Event string
}

// Defect is one row the reader refused, in the words the steward's digest
// prints. Ownerless says the row named no accountable owner, which is the one
// defect the digest offers a choice for.
type Defect struct {
	Label     string
	Reason    string
	Ownerless bool
}

// Register is one read of the file: every row it could read whole, the valid
// scheduled subset, and every defect, in register order.
type Register struct {
	Rows    []Row
	Reviews []Review
	Defects []Defect
}

// Path is where the register lives in a checkout.
func Path(repoRoot string) string {
	return filepath.Join(repoRoot, "memory", "rulings.md")
}

// ParseReviewCondition reads one review condition into its scheduled parts.
//
// An empty condition is not an error and is not a schedule: most rulings stand
// until a human says otherwise. Everything else must be key=value tokens from
// the three keys, must name one of the four schedulable classes, must carry a
// due date or an event, and must write any due date as a date. Every refusal
// is worded as the steward worded it, because the digest prints these
// sentences.
func ParseReviewCondition(value string) (class, due, event string, err error) {
	if value == "" {
		return "", "", "", nil
	}
	for _, token := range strings.Fields(value) {
		key, val, found := strings.Cut(token, "=")
		if !found || val == "" {
			return "", "", "", fmt.Errorf("review condition token %q is not key=value", token)
		}
		switch key {
		case "class":
			class = val
		case "due":
			due = val
		case "event":
			event = val
		default:
			return "", "", "", fmt.Errorf("unknown review condition key %q", key)
		}
	}
	if class != ClassTemporary && class != ClassExperimental && class != ClassDelegatedAuthority && class != ClassAssumptionDependent {
		return "", "", "", fmt.Errorf("review class %q is not schedulable", class)
	}
	if due == "" && event == "" {
		return "", "", "", fmt.Errorf("review condition needs due= or event=")
	}
	if due != "" {
		if _, parseErr := time.Parse(DateLayout, due); parseErr != nil {
			return "", "", "", fmt.Errorf("review due date %q is invalid", due)
		}
	}
	return class, due, event, nil
}

// Read reads the register once and answers all three views.
//
// A checkout with no register is not an error: an adopted repository that
// records no rulings has an empty register, which is what it means. A row this
// reader cannot split into six columns is a defect named by its position in
// the register, because its id is exactly what could not be trusted.
func Read(repoRoot string) (Register, error) {
	file, err := os.Open(Path(repoRoot))
	if os.IsNotExist(err) {
		return Register{}, nil
	}
	if err != nil {
		return Register{}, err
	}
	defer file.Close()

	var register Register
	// The scanner is the steward's, with its own line ceiling, because this
	// reader accepts and refuses exactly what the sweep accepted and refused
	// before it was lifted. The longest row the register carries is a few
	// thousand bytes.
	scanner := bufio.NewScanner(file)
	rowPosition := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "| R-") {
			continue
		}
		rowPosition++
		fields := strings.Split(line, "|")
		if len(fields) != 8 {
			register.Defects = append(register.Defects, Defect{
				Label:  fmt.Sprintf("row=%d", rowPosition),
				Reason: fmt.Sprintf("wrong column count: got %d, want 6", len(fields)-2),
			})
			continue
		}
		row := Row{
			ID:        strings.TrimSpace(fields[1]),
			Date:      strings.TrimSpace(fields[2]),
			Words:     strings.TrimSpace(fields[3]),
			Context:   strings.TrimSpace(fields[4]),
			Owner:     strings.TrimSpace(fields[5]),
			Condition: strings.TrimSpace(fields[6]),
		}
		class, due, event, parseErr := ParseReviewCondition(row.Condition)
		if parseErr == nil {
			row.Class, row.Due, row.Event = class, due, event
		}
		// The row is carried whole whatever the parser made of its condition:
		// a ruling with a prose review condition is still a ruling, and the
		// page shows the condition as written. The defects below are the
		// steward's own judgement of the same row, in the same order it made
		// them, so the digest is unchanged.
		register.Rows = append(register.Rows, row)
		if row.Owner == "" {
			register.Defects = append(register.Defects, Defect{Label: row.ID, Reason: "no accountable owner", Ownerless: true})
			continue
		}
		if parseErr != nil {
			register.Defects = append(register.Defects, Defect{Label: row.ID, Reason: parseErr.Error()})
			continue
		}
		if class != "" {
			register.Reviews = append(register.Reviews, Review{ID: row.ID, Owner: row.Owner, Class: class, Due: due, Event: event})
		}
	}
	if err := scanner.Err(); err != nil {
		return Register{}, err
	}
	return register, nil
}

// ReadReviews is the steward's own view: the valid scheduled subset and the
// defects, exactly as its sweep has always read them.
func ReadReviews(repoRoot string) ([]Review, []Defect, error) {
	register, err := Read(repoRoot)
	if err != nil {
		return nil, nil, err
	}
	return register.Reviews, register.Defects, nil
}

// DuePassed says whether a valid due date is today or earlier, against the
// observing clock's own UTC day.
//
// An event condition is not judged here and never will be: what an event
// means is an evaluation with observed and unobservable outcomes, which is
// the steward's, and a page that guessed at one would be claiming a ruling is
// overdue on no evidence.
func DuePassed(due string, now time.Time) bool {
	if due == "" {
		return false
	}
	if _, err := time.Parse(DateLayout, due); err != nil {
		return false
	}
	return due <= now.UTC().Format(DateLayout)
}
