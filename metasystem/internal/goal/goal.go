// Package goal owns the sixth standing ledger: plans/goals.md, the thread
// of intent that survives every turn boundary (goal-system design, D69).
// It holds the grammar, the parser, the verbs, the turn verdict, and the
// verdict's input contract (ScanResult); internal/report's scanner fills
// that contract across the declared report→goal edge, and the verdict
// never imports report.
//
// The parser has two consumers' postures behind one function: the WRITE
// path refuses any grammar or bound violation outright, and the READ path
// carries the same violations as degraded-state problems, because a
// present-but-malformed ledger must warn loudly without fabricating
// blocks.
package goal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Byte bounds, complete by design (r4 finding 12). Everything projected
// anywhere is bounded at its source.
const (
	MaxIdBytes        = 64
	MaxIntentBytes    = 160
	MaxNextStepBytes  = 240
	MaxEvidenceLines  = 3
	MaxEvidenceBytes  = 200
	MaxParkedBytes    = 240
	MaxConcludedBytes = 240
	// DoneKept is the prune floor: prune drops Done blocks beyond the
	// newest ten.
	DoneKept = 10
)

// Origin says who opened a goal. Human-origin goals carry an advisory
// authority gate on done/park (D66 constraint 3).
const (
	OriginHuman = "human"
	OriginMain  = "main"
)

// Goal is one ledger block. Fields not applicable to a section stay empty.
type Goal struct {
	Id       string
	Intent   string
	Origin   string
	NextStep string   // Current, Queued, Parked
	Evidence []string // Current only, optional
	Parked   string   // Parked: the required "Parked because"
	Conclude string   // Done: the required "Concluded"
}

// Free is the Goal-free declaration: declared absence of intent, pinned to
// the plans-stream world it was declared over.
type Free struct {
	Declared string // ISO time as written
	Origin   string
	Digest   string // the plans-stream scan digest at declaration
}

// Ledger is the parsed file plus the exact byte slices the verdict hashes.
type Ledger struct {
	Current *Goal
	Queued  []Goal
	Parked  []Goal
	Done    []Goal
	Free    *Free

	// CurrentBlock is the Current goal's exact bytes, heading through its
	// last field line — the revision hashes these and nothing else.
	CurrentBlock []byte
	// QueuedSection is every Queued block's exact bytes in file order —
	// the queued-only verdict hashes these.
	QueuedSection []byte
}

// HasGoals reports whether the ledger carries any real goal (current,
// queued, parked, or done). A genesis skeleton is goal-free; an
// initialized project is not — the distinction blocks the
// deleted-baseline downgrade (genesis authority review F2).
func (l *Ledger) HasGoals() bool {
	return l.Current != nil || len(l.Queued) > 0 || len(l.Parked) > 0 || len(l.Done) > 0
}

// Revision is the Current goal's revision: sha256 of its exact block
// bytes. Empty when there is no Current goal.
func (l *Ledger) Revision() string {
	if l.Current == nil {
		return ""
	}
	return sha256Hex(l.CurrentBlock)
}

// QueuedDigest keys the queued-only block-once state.
func (l *Ledger) QueuedDigest() string {
	if len(l.Queued) == 0 {
		return ""
	}
	return sha256Hex(l.QueuedSection)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// LedgerPath and BaselinePath are the pair adoption seeds together.
func LedgerPath(root string) string {
	return filepath.Join(root, "plans", "goals.md")
}

func BaselinePath(root string) string {
	return filepath.Join(root, "plans", "goals-accepted.json")
}

// ScanDigest is the plans-stream set digest a Goal-free declaration pins:
// sha256 over the sorted base names of plans/*.md, excluding goals.md
// itself (scanner disjointness: only the goal parser reads the ledger).
// The SET is the fact: declared absence expires when the stream world
// gains or loses a member.
func ScanDigest(root string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "plans"))
	if err != nil {
		if os.IsNotExist(err) {
			return sha256Hex(nil), nil
		}
		return "", err
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || name == "goals.md" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return sha256Hex([]byte(strings.Join(names, "\n"))), nil
}

// Problem is one grammar or bound violation, human-readable.
type Problem string

// Parse reads ledger bytes. Problems are returned, never panicked over:
// the write path refuses on any, the read path degrades on any. A
// structurally intact ledger with problems still returns its parsed
// content so displays can name what they saw.
func Parse(data []byte) (*Ledger, []Problem) {
	ledger := &Ledger{}
	var problems []Problem
	addProblem := func(format string, args ...any) {
		problems = append(problems, Problem(fmt.Sprintf(format, args...)))
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "# Goals" {
		addProblem("the ledger must start with '# Goals'")
	}

	type section struct {
		kind  string // current, queued, parked, done
		goal  Goal
		start int
		end   int // exclusive, line index after the last field line
	}
	var sections []section
	seen := map[string]int{}

	i := 0
	for i < len(lines) {
		line := lines[i]
		if !strings.HasPrefix(line, "## ") {
			i++
			continue
		}
		heading := strings.TrimPrefix(line, "## ")
		switch {
		case strings.HasPrefix(heading, "Goal-free:"):
			if ledger.Free != nil {
				addProblem("duplicate Goal-free declaration")
			}
			free, ok := parseFree(heading)
			if !ok {
				addProblem("malformed Goal-free declaration: %q", line)
			} else {
				ledger.Free = &free
			}
			i++
		case strings.HasPrefix(heading, "Current goal: "),
			strings.HasPrefix(heading, "Queued goal: "),
			strings.HasPrefix(heading, "Parked goal: "),
			strings.HasPrefix(heading, "Done goal: "):
			kind := strings.ToLower(strings.SplitN(heading, " ", 2)[0])
			title := heading[strings.Index(heading, ": ")+2:]
			goal, ok := parseTitle(title)
			if !ok {
				addProblem("malformed %s goal heading: %q", kind, line)
				i++
				continue
			}
			start := i
			i++
			for i < len(lines) && strings.HasPrefix(lines[i], "- ") {
				parseField(&goal, lines[i], addProblem)
				i++
			}
			sections = append(sections, section{kind: kind, goal: goal, start: start, end: i})
			seen[goal.Id]++
		default:
			addProblem("unknown section heading: %q", line)
			i++
		}
	}

	for id, count := range seen {
		if count > 1 {
			addProblem("goal id %q appears %d times; ids are unique across all sections", id, count)
		}
	}

	var queuedBlocks []string
	for _, s := range sections {
		boundGoal(&s.goal, s.kind, addProblem)
		block := strings.Join(lines[s.start:s.end], "\n")
		switch s.kind {
		case "current":
			if ledger.Current != nil {
				addProblem("more than one Current goal")
				continue
			}
			goalCopy := s.goal
			ledger.Current = &goalCopy
			ledger.CurrentBlock = []byte(block)
		case "queued":
			ledger.Queued = append(ledger.Queued, s.goal)
			queuedBlocks = append(queuedBlocks, block)
		case "parked":
			ledger.Parked = append(ledger.Parked, s.goal)
		case "done":
			ledger.Done = append(ledger.Done, s.goal)
		}
	}
	ledger.QueuedSection = []byte(strings.Join(queuedBlocks, "\n"))

	// Zero-current legality: a queue or a declaration must stand in.
	if ledger.Current == nil && len(ledger.Queued) == 0 && ledger.Free == nil {
		addProblem("zero Current goals with neither a queue nor a Goal-free declaration; undeclared absence is unrepresentable")
	}
	if ledger.Free != nil && (ledger.Current != nil || len(ledger.Queued) > 0) {
		addProblem("a Goal-free declaration is legal only at zero Current and zero Queued goals")
	}

	return ledger, problems
}

// parseTitle splits "<kebab-id> — <intent>".
func parseTitle(title string) (Goal, bool) {
	parts := strings.SplitN(title, " — ", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Goal{}, false
	}
	return Goal{Id: strings.TrimSpace(parts[0]), Intent: strings.TrimSpace(parts[1])}, true
}

// parseField reads one "- Key: value" line into the goal.
func parseField(goal *Goal, line string, addProblem func(string, ...any)) {
	body := strings.TrimPrefix(line, "- ")
	parts := strings.SplitN(body, ": ", 2)
	if len(parts) != 2 {
		addProblem("malformed field line: %q", line)
		return
	}
	value := parts[1]
	switch parts[0] {
	case "Origin":
		goal.Origin = value
	case "Next step":
		goal.NextStep = value
	case "Evidence":
		goal.Evidence = append(goal.Evidence, value)
	case "Parked because":
		goal.Parked = value
	case "Concluded":
		goal.Conclude = value
	default:
		addProblem("unknown field %q in %q", parts[0], line)
	}
}

// parseFree reads "Goal-free: declared <ISO> by <origin> over <digest>".
func parseFree(heading string) (Free, bool) {
	rest := strings.TrimPrefix(heading, "Goal-free: declared ")
	if rest == heading {
		return Free{}, false
	}
	byIdx := strings.Index(rest, " by ")
	overIdx := strings.Index(rest, " over ")
	if byIdx < 0 || overIdx < 0 || overIdx < byIdx {
		return Free{}, false
	}
	free := Free{
		Declared: rest[:byIdx],
		Origin:   rest[byIdx+4 : overIdx],
		Digest:   rest[overIdx+6:],
	}
	if free.Declared == "" || free.Origin == "" || free.Digest == "" {
		return Free{}, false
	}
	return free, true
}

// boundGoal enforces every byte bound and per-section field requirement.
func boundGoal(goal *Goal, kind string, addProblem func(string, ...any)) {
	if len(goal.Id) > MaxIdBytes {
		addProblem("goal %q: id exceeds %d bytes", goal.Id, MaxIdBytes)
	}
	if !validId(goal.Id) {
		addProblem("goal %q: id must be kebab-case [a-z0-9-]", goal.Id)
	}
	if len(goal.Intent) > MaxIntentBytes {
		addProblem("goal %q: intent exceeds %d bytes", goal.Id, MaxIntentBytes)
	}
	if goal.Origin != OriginHuman && goal.Origin != OriginMain {
		addProblem("goal %q: Origin must be human or main, got %q", goal.Id, goal.Origin)
	}
	if singleLineViolation(goal.NextStep) {
		addProblem("goal %q: next step carries control characters", goal.Id)
	}
	if len(goal.NextStep) > MaxNextStepBytes {
		addProblem("goal %q: next step exceeds %d bytes", goal.Id, MaxNextStepBytes)
	}
	if len(goal.Evidence) > MaxEvidenceLines {
		addProblem("goal %q: more than %d evidence lines", goal.Id, MaxEvidenceLines)
	}
	for _, line := range goal.Evidence {
		if len(line) > MaxEvidenceBytes {
			addProblem("goal %q: evidence line exceeds %d bytes", goal.Id, MaxEvidenceBytes)
		}
	}
	if len(goal.Parked) > MaxParkedBytes {
		addProblem("goal %q: parked-because exceeds %d bytes", goal.Id, MaxParkedBytes)
	}
	if len(goal.Conclude) > MaxConcludedBytes {
		addProblem("goal %q: concluded exceeds %d bytes", goal.Id, MaxConcludedBytes)
	}
	switch kind {
	case "current", "queued":
		if goal.NextStep == "" {
			addProblem("goal %q: %s goals require a next step", goal.Id, kind)
		}
	case "parked":
		if goal.Parked == "" {
			addProblem("goal %q: parked goals require a parked-because", goal.Id)
		}
		if goal.NextStep == "" {
			addProblem("goal %q: parked goals keep their next step for unpark", goal.Id)
		}
	case "done":
		if goal.Conclude == "" {
			addProblem("goal %q: done goals require a concluded line", goal.Id)
		}
		if goal.NextStep != "" {
			addProblem("goal %q: done goals carry no next step", goal.Id)
		}
	}
}

func validId(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

// singleLineViolation reports control characters in a one-line field.
func singleLineViolation(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// Serialize writes the canonical ledger bytes the verbs publish. Parse and
// Serialize round-trip: Serialize(Parse(x)) is byte-stable for canonical
// input.
func Serialize(ledger *Ledger) []byte {
	var b strings.Builder
	b.WriteString("# Goals\n")
	writeGoal := func(kind string, goal Goal) {
		b.WriteString("\n## " + kind + " goal: " + goal.Id + " — " + goal.Intent + "\n")
		b.WriteString("- Origin: " + goal.Origin + "\n")
		switch kind {
		case "Current":
			b.WriteString("- Next step: " + goal.NextStep + "\n")
			for _, line := range goal.Evidence {
				b.WriteString("- Evidence: " + line + "\n")
			}
		case "Queued":
			b.WriteString("- Next step: " + goal.NextStep + "\n")
		case "Parked":
			b.WriteString("- Parked because: " + goal.Parked + "\n")
			b.WriteString("- Next step: " + goal.NextStep + "\n")
		case "Done":
			b.WriteString("- Concluded: " + goal.Conclude + "\n")
		}
	}
	if ledger.Current != nil {
		writeGoal("Current", *ledger.Current)
	}
	for _, goal := range ledger.Queued {
		writeGoal("Queued", goal)
	}
	for _, goal := range ledger.Parked {
		writeGoal("Parked", goal)
	}
	for _, goal := range ledger.Done {
		writeGoal("Done", goal)
	}
	if ledger.Free != nil {
		b.WriteString("\n## Goal-free: declared " + ledger.Free.Declared +
			" by " + ledger.Free.Origin + " over " + ledger.Free.Digest + "\n")
	}
	return []byte(b.String())
}
