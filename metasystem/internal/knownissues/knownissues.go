// Package knownissues reads the known-issues register, memory/known-issues.md.
//
// One row per defect or limitation, in the project's own words, and this
// package widens nothing and repairs nothing. It is the rulings reader's
// manner over a second register: read the file once, carry every row it can
// read whole, and name the rows it could not rather than dropping them.
//
// Two facts about this register decide the whole reader.
//
// The columns are named by the file. The kit's own register writes `| Id |
// Date | Symptom and evidence | Cost when it bites | Fix direction or lever |
// Status |`, and the one adoption ships writes `| Id | Date | Issue |
// Consequence | Reopen when | Status |` (scripts/adopt.sh). Both are six
// columns in the same order of meaning, so the reader takes the header row as
// the names of its columns, keeps them exactly as supplied — the fifth differs
// between the two sets and no reader may pretend otherwise — and reads either
// set by position.
//
// A row is open unless its status says it is concluded. The status column is
// prose that BEGINS with a word: FIXED, RESOLVED, RETIRED, CLOSED, ACCEPTED,
// OPEN, or a date. The five that conclude are listed below and nothing else is
// interpreted: an accepted limitation still exists, so its status word travels
// with the row and a reader shows it. There is no prose interpreter here and
// there will not be one.
//
// Cells are split on unescaped pipes only. This register carries a cell
// holding `\|` inside a code span, and a reader that split on every pipe would
// read that row as having eight columns and refuse it. A row whose cell count
// is still wrong after that is not repaired either: it is counted and named,
// because five of this register's own open rows are such rows today and a
// block that silently skipped them would be a page claiming a project knows
// about fewer problems than it does.
package knownissues

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Columns is how many columns both column sets carry.
const Columns = 6

// The status words that say a row is concluded.
//
// RETIRED and RESOLVED are here because both are written in this register
// today and a reader that knew only FIXED, CLOSED and ACCEPTED would list them
// as open problems this project still has. The word itself stays on the row:
// "accepted" is not "gone".
var concludedWords = []string{"FIXED", "RESOLVED", "RETIRED", "CLOSED", "ACCEPTED"}

// Row is one row of the register, by position, whatever the header called the
// columns.
//
// Open is this package's one judgement, and it is made from the status word
// alone. Everything else is the register's own text, trimmed and unescaped.
type Row struct {
	ID   string `json:"id"`
	Date string `json:"date"`
	// What is the third column: the symptom and its evidence, or the issue.
	What string `json:"what"`
	// Consequence is the fourth: what it costs when it bites.
	Consequence string `json:"consequence"`
	// Lever is the fifth, whose title differs between the two column sets: a
	// fix direction, or the condition that reopens the row.
	Lever string `json:"lever"`
	// Status is the sixth, whole, with its opening word intact.
	Status string `json:"status"`
	Open   bool   `json:"open"`
}

// Register is one read of the file: the column names the header supplied, the
// rows that could be read, split into open and concluded in register order,
// and the rows that could not.
type Register struct {
	// Columns are the header's own six names, or empty where the file carries
	// no table at all.
	Columns   []string `json:"columns"`
	Open      []Row    `json:"open"`
	Concluded []Row    `json:"concluded"`
	// Unread is how many rows this reader refused, which is what the block's
	// one line counts.
	Unread int `json:"unread"`
	// Defects are those rows in this reader's own words, named by their
	// position in the register, because the id is exactly what could not be
	// trusted.
	Defects []string `json:"defects"`
}

// Path is where the register lives in a checkout or an installation.
func Path(root string) string {
	return filepath.Join(root, "memory", "known-issues.md")
}

// Read reads the register once.
//
// A root with no register is not an error: a project that records no known
// issues has an empty register, which is what it means. Neither is a file with
// no table in it. A header that does not name six columns is: the reader reads
// either column set BY POSITION, so a header of another width is a file this
// reader cannot place columns in, and it says so rather than guessing.
func Read(root string) (Register, error) {
	file, err := os.Open(Path(root))
	if os.IsNotExist(err) {
		return empty(), nil
	}
	if err != nil {
		return empty(), err
	}
	defer func() { _ = file.Close() }()

	register := empty()
	headed := false
	scanner := bufio.NewScanner(file)
	// The register's longest row is a few thousand bytes of prose; the ceiling
	// is the rulings reader's, for the same file shape and the same reason.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	position := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := Cells(line)
		if !headed {
			if len(cells) != Columns {
				register.Defects = append(register.Defects,
					fmt.Sprintf("header: wrong column count: got %d, want %d", len(cells), Columns))
				return register, nil
			}
			register.Columns, headed = cells, true
			continue
		}
		if delimiter(cells) {
			continue
		}
		position++
		if len(cells) != Columns {
			register.Unread++
			register.Defects = append(register.Defects,
				fmt.Sprintf("row=%d: wrong column count: got %d, want %d", position, len(cells), Columns))
			continue
		}
		row := Row{
			ID: cells[0], Date: cells[1], What: cells[2],
			Consequence: cells[3], Lever: cells[4], Status: cells[5],
		}
		row.Open = !Concluded(row.Status)
		if row.Open {
			register.Open = append(register.Open, row)
			continue
		}
		register.Concluded = append(register.Concluded, row)
	}
	if err := scanner.Err(); err != nil {
		return empty(), err
	}
	return register, nil
}

// Cells is one table row split into its cells, on unescaped pipes only.
//
// A pipe written `\|` is a pipe inside a cell — Markdown's own escape, which
// this register uses inside a code span — and it arrives in the cell as a
// plain pipe. The leading and trailing pipes of the row delimit rather than
// separate, so they contribute no cells.
func Cells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return nil
	}
	cells := []string{}
	var cell strings.Builder
	escaped := false
	for _, character := range trimmed[1:] {
		if escaped {
			// Only a pipe is unescaped here: every other backslash in this
			// register is prose or a path, and a reader that swallowed them
			// would be rewriting the project's own words.
			if character != '|' {
				cell.WriteRune('\\')
			}
			cell.WriteRune(character)
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == '|' {
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
			continue
		}
		cell.WriteRune(character)
	}
	// What follows the last pipe is the row's closing delimiter where it is
	// blank, and a final cell where the row forgot to close itself.
	if rest := strings.TrimSpace(cell.String()); rest != "" {
		cells = append(cells, rest)
	}
	if escaped {
		cells = append(cells, "\\")
	}
	return cells
}

// Concluded reports whether a status says the row is concluded.
//
// The test is the opening word and nothing else. A word boundary is required,
// so a status opening "FIX SHIPPED, PROOF INCOMPLETE" is the open row it says
// it is rather than a fixed one.
func Concluded(status string) bool {
	opening := strings.TrimSpace(status)
	for _, word := range concludedWords {
		if !strings.HasPrefix(opening, word) {
			continue
		}
		if len(opening) == len(word) {
			return true
		}
		next := opening[len(word)]
		if (next >= 'A' && next <= 'Z') || (next >= 'a' && next <= 'z') || (next >= '0' && next <= '9') {
			continue
		}
		return true
	}
	return false
}

// delimiter reports the `| --- | --- |` line beneath a header, which is table
// punctuation rather than a row.
func delimiter(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		if strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return true
}

// empty is a register nothing could be read out of, written so that every list
// is a list rather than a null a reader has to test for.
func empty() Register {
	return Register{Columns: []string{}, Open: []Row{}, Concluded: []Row{}, Defects: []string{}}
}
