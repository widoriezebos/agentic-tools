package validate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var dispositionsHeader = []string{"Finding id", "Disposition", "Reasoning and evidence", "Amendment"}

var (
	fenceMarkerRe   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	separatorCellRe = regexp.MustCompile(`^:?-{3,}:?$`)
)

// CritiqueClosed joins the canonical findings array from a critic
// return JSON against the Markdown dispositions table on finding id and
// returns every violation: each finding must have a disposition row, a
// material finding may not be dispositioned 'noted', and no disposition
// may name an unknown finding id. Structural problems on either side
// make that side unjoinable, and the join runs only when both sides
// join cleanly.
func CritiqueClosed(findingsPath, dispositionsPath string) []string {
	var violations []string
	violation := func(format string, args ...any) {
		violations = append(violations, fmt.Sprintf(format, args...))
	}

	findingIDs, materialByID, findingsJoinable := readFindings(findingsPath, violation)
	dispositionIDs, dispositionByID, dispositionsJoinable := readDispositions(dispositionsPath, violation)

	if findingsJoinable && dispositionsJoinable {
		for _, id := range findingIDs {
			disposition, present := dispositionByID[id]
			if !present {
				violation("finding id '%s' has no disposition row", id)
			} else if materialByID[id] && disposition == "noted" {
				violation("material finding id '%s' cannot use disposition 'noted'", id)
			}
		}
		for _, id := range dispositionIDs {
			if _, present := materialByID[id]; !present {
				violation("disposition names unknown finding id: '%s'", id)
			}
		}
	}
	return violations
}

// readFindings parses the return JSON down to a first-wins id-to-material
// map, reporting structural violations and whether the side is joinable.
func readFindings(path string, violation func(string, ...any)) ([]string, map[string]bool, bool) {
	materialByID := map[string]bool{}
	data, exists, err := readFileIfExists(path)
	if !exists {
		violation("return JSON is unjoinable: file does not exist: %s", path)
		return nil, materialByID, false
	}
	if err != nil {
		violation("return JSON is unjoinable: could not read %s: %v", path, err)
		return nil, materialByID, false
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		line, column := jsonErrorPosition(data, err)
		violation("return JSON is unjoinable: invalid JSON at line %d, column %d: %v", line, column, err)
		return nil, materialByID, false
	}
	object, ok := parsed.(map[string]any)
	if !ok {
		violation("return JSON is unjoinable: root must be an object")
		return nil, materialByID, false
	}
	raw, present := object["findings"]
	if !present {
		violation("return JSON is unjoinable: $.findings array is missing")
		return nil, materialByID, false
	}
	list, ok := raw.([]any)
	if !ok {
		violation("return JSON is unjoinable: $.findings must be an array")
		return nil, materialByID, false
	}

	var ids []string
	seen := map[string]bool{}
	joinable := true
	for index, item := range list {
		fieldPath := fmt.Sprintf("$.findings[%d]", index)
		finding, ok := item.(map[string]any)
		if !ok {
			violation("return JSON is unjoinable: %s must be an object", fieldPath)
			joinable = false
			continue
		}
		id, ok := finding["id"].(string)
		if !ok || strings.TrimSpace(id) == "" || id != strings.TrimSpace(id) {
			violation("return JSON is unjoinable: %s.id must be a non-empty string without surrounding whitespace", fieldPath)
			joinable = false
			continue
		}
		if seen[id] {
			violation("duplicate finding id: '%s'", id)
		} else {
			seen[id] = true
		}
		material, ok := finding["material"].(bool)
		if !ok {
			violation("return JSON is unjoinable: %s.material must be boolean", fieldPath)
			joinable = false
			continue
		}
		if _, recorded := materialByID[id]; !recorded {
			materialByID[id] = material
			ids = append(ids, id)
		}
	}
	return ids, materialByID, joinable
}

// readDispositions parses the Markdown dispositions table (outside code
// fences) down to a first-wins id-to-disposition map, reporting
// structural violations and whether the side is joinable.
func readDispositions(path string, violation func(string, ...any)) ([]string, map[string]string, bool) {
	dispositionByID := map[string]string{}
	data, exists, err := readFileIfExists(path)
	if !exists {
		violation("dispositions file is unjoinable: file does not exist: %s", path)
		return nil, dispositionByID, false
	}
	if err != nil {
		violation("dispositions file is unjoinable: could not read %s: %v", path, err)
		return nil, dispositionByID, false
	}
	lines := splitLines(string(data))
	visible := linesOutsideFences(lines)

	headerIndex := -1
	headerCount := 0
	for index, line := range lines {
		if visible[index] && equalStrings(markdownCells(line), dispositionsHeader) {
			headerCount++
			if headerIndex < 0 {
				headerIndex = index
			}
		}
	}
	if headerCount == 0 {
		violation("dispositions file is unjoinable: malformed dispositions table: required header not found")
		return nil, dispositionByID, false
	}
	if headerCount > 1 {
		violation("dispositions file is unjoinable: malformed dispositions table: multiple required headers found")
		return nil, dispositionByID, false
	}

	separatorIndex := headerIndex + 1
	if separatorIndex >= len(lines) || !visible[separatorIndex] {
		violation("dispositions file is unjoinable: malformed dispositions table: separator row is missing")
		return nil, dispositionByID, false
	}
	separator := markdownCells(lines[separatorIndex])
	separatorValid := separator != nil && len(separator) == len(dispositionsHeader)
	if separatorValid {
		for _, cell := range separator {
			if !separatorCellRe.MatchString(cell) {
				separatorValid = false
			}
		}
	}
	if !separatorValid {
		violation("dispositions file is unjoinable: malformed dispositions table: invalid separator row at line %d", separatorIndex+1)
		return nil, dispositionByID, false
	}

	type tableRow struct {
		line  int
		cells []string
	}
	var rows []tableRow
	joinable := true
	for index := separatorIndex + 1; index < len(lines); index++ {
		if !visible[index] || strings.TrimSpace(lines[index]) == "" {
			break
		}
		cells := markdownCells(lines[index])
		if cells == nil {
			break
		}
		if len(cells) != len(dispositionsHeader) {
			violation("dispositions file is unjoinable: malformed dispositions table: row at line %d has %d columns instead of %d",
				index+1, len(cells), len(dispositionsHeader))
			joinable = false
			continue
		}
		rows = append(rows, tableRow{index + 1, cells})
	}

	var ids []string
	seen := map[string]bool{}
	for _, row := range rows {
		findingID, disposition := row.cells[0], row.cells[1]
		if findingID == "" {
			violation("dispositions file is unjoinable: malformed dispositions table: row at line %d has an empty finding id", row.line)
			joinable = false
			continue
		}
		if seen[findingID] {
			violation("duplicate disposition id: '%s'", findingID)
		} else {
			seen[findingID] = true
		}
		if disposition != "accepted" && disposition != "refuted" && disposition != "noted" {
			violation("disposition for finding id '%s' has unknown value '%s'; allowed values are accepted, refuted, noted",
				findingID, disposition)
		}
		// A refutation without evidence is not a refutation: the chain
		// closes on ZERO unrefuted material findings, and an empty or
		// placeholder evidence cell leaves the finding standing.
		if disposition == "refuted" {
			evidence := ""
			if len(row.cells) > 2 {
				evidence = strings.TrimSpace(row.cells[2])
			}
			if evidence == "" || evidence == "..." || strings.EqualFold(evidence, "none") || strings.EqualFold(evidence, "n/a") {
				violation("finding id '%s' is refuted without evidence; a refutation carries the exact check and its observed result, or the finding stands", findingID)
			}
		}
		if _, recorded := dispositionByID[findingID]; !recorded {
			dispositionByID[findingID] = disposition
			ids = append(ids, findingID)
		}
	}
	return ids, dispositionByID, joinable
}

// markdownCells splits one table line into trimmed cells on unescaped
// pipes, dropping an empty leading and trailing cell. A line without a
// pipe is not a table line and yields nil.
func markdownCells(line string) []string {
	stripped := strings.TrimSpace(line)
	if !strings.Contains(stripped, "|") {
		return nil
	}
	cells := []string{}
	var cell []rune
	escaped := false
	for _, character := range stripped {
		switch {
		case escaped:
			cell = append(cell, character)
			escaped = false
		case character == '\\':
			cell = append(cell, character)
			escaped = true
		case character == '|':
			cells = append(cells, strings.TrimSpace(string(cell)))
			cell = cell[:0]
		default:
			cell = append(cell, character)
		}
	}
	cells = append(cells, strings.TrimSpace(string(cell)))
	if len(cells) > 0 && cells[0] == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// linesOutsideFences marks which lines sit outside Markdown code fences;
// fence markers themselves are not visible. A fence closes only on a
// marker of the same character at least as long as the opener.
func linesOutsideFences(lines []string) []bool {
	visible := make([]bool, len(lines))
	var fenceCharacter byte
	fenceLength := 0
	for index, line := range lines {
		marker := fenceMarkerRe.FindStringSubmatch(line)
		if fenceCharacter == 0 {
			if marker != nil {
				fenceCharacter = marker[1][0]
				fenceLength = len(marker[1])
			} else {
				visible[index] = true
			}
		} else if marker != nil && marker[1][0] == fenceCharacter && len(marker[1]) >= fenceLength {
			fenceCharacter = 0
			fenceLength = 0
		}
	}
	return visible
}

// jsonErrorPosition converts a JSON syntax error's byte offset into a
// one-based line and column for the violation message.
func jsonErrorPosition(data []byte, err error) (int, int) {
	var offset int64
	if syntax, ok := err.(*json.SyntaxError); ok {
		offset = syntax.Offset
	}
	if offset < 1 {
		offset = 1
	}
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	line, column := 1, 1
	for _, b := range data[:offset-1] {
		if b == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	return line, column
}
