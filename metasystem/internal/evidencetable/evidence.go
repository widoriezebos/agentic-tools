// Package evidence reads and judges the app-owned covenant evidence
// table (docs/covenant-evidence.md), born at inception and re-derived
// here on demand. The table is gate-defining input on the same trust
// tier as the covenant itself: it is guardrail-classed, its bytes are
// loaded through the same held-handle discipline, and nothing in this
// package ever writes it — the counselor proposes, the human lands.
//
// What this package judges is deliberately narrow: traceability
// (every covenant requirement has its table row, matched by stable
// ID), declared-dependency presence (the repo paths a row names exist
// in the tree, reached without following a single symlink), and
// status lawfulness (a covenant-backed row recorded as having no
// proof refuses). Recorded statuses are claims on file — "observed"
// is NOT re-verified here; that needs observation metadata a later
// slice records.
package evidencetable

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Statuses are the three honesty levels the inception interview
// assigns; anything else is a format refusal.
const (
	StatusObserved         = "observed"
	StatusReferencedNotRun = "referenced-not-run"
	StatusPlannedFloating  = "planned-floating"
)

// Kinds say where the row's evidence lives: executed from the
// repository, or recorded outside it (a dashboard, a paid bench).
const (
	KindRepo     = "repo"
	KindExternal = "external"
)

// PlannedCommand is the exact command cell every floating row carries:
// the row's whole meaning is that no executable proof exists yet.
const PlannedCommand = "(planned)"

// TableFilename is the table's one home relative to the app root,
// beside the covenant it evidences.
const TableFilename = "docs/covenant-evidence.md"

// canonHeader is the format law's header row. A table whose header
// does not match cell-for-cell is not the authoritative table.
var canonHeader = []string{
	"criterion id", "criterion", "proof id", "kind",
	"exact command", "repo deps", "evidence source", "status",
}

// criterionIDPattern is the shared identity grammar: requirement IDs
// in the covenant and criterion ids here are one namespace.
var criterionIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// countLinePattern pins the persisted count line's exact syntax; the
// arithmetic against the rows is evaluation, never parsing.
var countLinePattern = regexp.MustCompile(`^Wired: ([0-9]+)\. Floating: ([0-9]+)\.$`)

var separatorCell = regexp.MustCompile(`^:?-{3,}:?$`)

// Row is one criterion's evidence record.
type Row struct {
	CriterionID string
	Criterion   string
	ProofID     string
	Kind        string
	Command     string
	Deps        []string
	Source      string
	Status      string
}

// Table is the parsed evidence file: the rows and the recorded
// wired/floating bookkeeping line.
type Table struct {
	Rows             []Row
	RecordedWired    int64
	RecordedFloating int64
}

func fail(format string, args ...any) error {
	return fmt.Errorf("evidence refused: "+format, args...)
}

// splitCells splits one markdown table line into trimmed cell texts,
// honoring \| as a literal pipe inside a cell.
func splitCells(line string) []string {
	var cells []string
	var cell strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			if r != '|' {
				cell.WriteRune('\\')
			}
			cell.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteRune(r)
		}
	}
	if escaped {
		cell.WriteRune('\\')
	}
	cells = append(cells, strings.TrimSpace(cell.String()))
	// A canonical row is "| a | b |": the split's leading and trailing
	// empty fragments are the outer pipes, not cells.
	if len(cells) > 0 && cells[0] == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

func isCanonHeader(cells []string) bool {
	if len(cells) != len(canonHeader) {
		return false
	}
	for i, want := range canonHeader {
		if !strings.EqualFold(cells[i], want) {
			return false
		}
	}
	return true
}

func isSeparator(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if !separatorCell.MatchString(c) {
			return false
		}
	}
	return true
}

// Parse validates evidence bytes against the canonical format law:
// exactly one authoritative table (header, separator, data rows),
// exactly one count line, and per-row grammar including the
// status-by-kind matrix. Competing candidates refuse — silently
// selecting one is the shadowing the covenant parser also forbids.
func Parse(data []byte, label string) (*Table, error) {
	if strings.ContainsRune(string(data), 0) {
		return nil, fail("%s contains a NUL byte", label)
	}
	lines := strings.Split(string(data), "\n")
	table := &Table{}
	seenIDs := map[string]int{}
	headerAt := -1
	countAt := -1
	inTable := false
	expectSeparator := false
	for n, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if m := countLinePattern.FindStringSubmatch(trimmed); m != nil {
			// A count line is NOT the separator: nothing may sit
			// between the header and its separator row.
			if expectSeparator {
				return nil, fail("%s line %d: the canonical header must be followed immediately by its separator row, not the count line", label, n+1)
			}
			if countAt >= 0 {
				return nil, fail("%s carries competing count lines (lines %d and %d); exactly one records the bookkeeping", label, countAt+1, n+1)
			}
			countAt = n
			var err error
			if table.RecordedWired, err = strconv.ParseInt(m[1], 10, 64); err != nil {
				return nil, fail("%s line %d: the wired count %s does not fit a number", label, n+1, m[1])
			}
			if table.RecordedFloating, err = strconv.ParseInt(m[2], 10, 64); err != nil {
				return nil, fail("%s line %d: the floating count %s does not fit a number", label, n+1, m[2])
			}
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			if expectSeparator {
				return nil, fail("%s line %d: the canonical header must be followed immediately by its separator row", label, headerAt+1)
			}
			inTable = false
			continue
		}
		cells := splitCells(trimmed)
		if isCanonHeader(cells) {
			if headerAt >= 0 {
				return nil, fail("%s carries competing evidence tables (headers at lines %d and %d); exactly one is authoritative", label, headerAt+1, n+1)
			}
			headerAt = n
			inTable = true
			expectSeparator = true
			continue
		}
		if !inTable {
			// A canonical-width data row DETACHED from the table (a
			// stray blank line above it) must refuse, never silently
			// vanish from judgment; narrower pipe lines stay prose.
			if headerAt >= 0 && len(cells) == len(canonHeader) && !isSeparator(cells) {
				return nil, fail("%s line %d: an evidence row outside the authoritative table — a blank line detached it from judgment", label, n+1)
			}
			continue
		}
		if expectSeparator {
			if !isSeparator(cells) || len(cells) != len(canonHeader) {
				return nil, fail("%s line %d: the canonical header must be followed immediately by an %d-cell separator row", label, n+1, len(canonHeader))
			}
			expectSeparator = false
			continue
		}
		if isSeparator(cells) {
			return nil, fail("%s line %d: a second separator row inside the table", label, n+1)
		}
		row, err := parseRow(cells, label, n+1)
		if err != nil {
			return nil, err
		}
		if prev, dup := seenIDs[row.CriterionID]; dup {
			return nil, fail("%s line %d: criterion id %q already has a row at line %d — two rows for one criterion are contradictory, never additive", label, n+1, row.CriterionID, prev)
		}
		seenIDs[row.CriterionID] = n + 1
		table.Rows = append(table.Rows, *row)
	}
	if headerAt < 0 {
		return nil, fail("%s has no evidence table with the canonical header", label)
	}
	if expectSeparator {
		return nil, fail("%s line %d: the canonical header must be followed immediately by its separator row", label, headerAt+1)
	}
	if countAt < 0 {
		return nil, fail("%s has no count line (\"Wired: N. Floating: M.\")", label)
	}
	if len(table.Rows) == 0 {
		return nil, fail("%s has a header but no evidence rows", label)
	}
	return table, nil
}

func parseRow(cells []string, label string, line int) (*Row, error) {
	if len(cells) != len(canonHeader) {
		return nil, fail("%s line %d: %d cells where the canon has %d", label, line, len(cells), len(canonHeader))
	}
	row := &Row{
		CriterionID: cells[0],
		Criterion:   cells[1],
		ProofID:     cells[2],
		Kind:        cells[3],
		Command:     cells[4],
		Source:      cells[6],
		Status:      cells[7],
	}
	if !criterionIDPattern.MatchString(row.CriterionID) {
		return nil, fail("%s line %d: criterion id %q is outside the identity grammar [A-Za-z0-9._-]+", label, line, row.CriterionID)
	}
	if row.ProofID == "" {
		return nil, fail("%s line %d: the proof id cell is empty", label, line)
	}
	if row.Kind != KindRepo && row.Kind != KindExternal {
		return nil, fail("%s line %d: kind %q is neither %q nor %q", label, line, row.Kind, KindRepo, KindExternal)
	}
	if deps := cells[5]; deps != "" {
		for _, dep := range strings.Split(deps, ",") {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				return nil, fail("%s line %d: the repo deps list has an empty entry", label, line)
			}
			row.Deps = append(row.Deps, dep)
		}
	}
	switch row.Status {
	case StatusObserved, StatusReferencedNotRun:
		// The no-proof marker is reserved for floating rows: a wired
		// row wearing it would launder "no proof exists" into bound.
		if row.Command == "" || row.Command == PlannedCommand {
			return nil, fail("%s line %d: a %s row must carry its executable command — %q belongs to planned-floating alone", label, line, row.Status, PlannedCommand)
		}
		if row.Kind == KindRepo && len(row.Deps) == 0 {
			return nil, fail("%s line %d: a %s repo row must declare its repo deps, first entry the proof's entrypoint file", label, line, row.Status)
		}
		if row.Kind == KindExternal && row.Source == "" {
			return nil, fail("%s line %d: a %s external row must name its evidence source", label, line, row.Status)
		}
	case StatusPlannedFloating:
		if len(row.Deps) != 0 {
			return nil, fail("%s line %d: a planned-floating row declares no deps — its whole meaning is that no proof exists yet", label, line)
		}
		if row.Command != PlannedCommand {
			return nil, fail("%s line %d: a planned-floating row's command cell is exactly %q", label, line, PlannedCommand)
		}
		if row.Kind == KindExternal && row.Source == "" {
			return nil, fail("%s line %d: a planned-floating external row names its INTENDED source — it is the row's only content", label, line)
		}
	default:
		return nil, fail("%s line %d: status %q is not one of %s, %s, %s", label, line, row.Status, StatusObserved, StatusReferencedNotRun, StatusPlannedFloating)
	}
	// Dep path grammar is format law: unclean paths die here, before
	// any filesystem walk gets a chance to be confused by them.
	for _, dep := range row.Deps {
		if err := ValidateDepPath(dep); err != nil {
			return nil, fail("%s line %d: repo dep %q: %v", label, line, dep, err)
		}
	}
	return row, nil
}
