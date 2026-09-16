package validate

import (
	"fmt"
	"regexp"
	"strings"
)

type MovedEffectsReport struct {
	Inventory string
	Rows      []MovedEffectRow
	Problems  []MovedEffectProblem
}
type MovedEffectRow struct {
	Line                   int
	Effect, From, To, Code string
	Paths                  []string
}
type MovedEffectProblem struct {
	Code, Detail string
	Line         int
}

var movedEffectsHeading = regexp.MustCompile(`(?i)^(#{2,4})[ \t]+(?:[0-9]+\.[ \t]+)?Moved effects[ \t]*$`)
var markdownHeading = regexp.MustCompile(`^(#{1,6})[ \t]+`)

func movedEffectWeak(value string) bool {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "`", ""))
	return value == "" || value == "none" || value == "tbd" || value == "todo" || value == "unknown" ||
		value == "unowned" || value == "nobody" || value == "-" || value == "?" || value == "n/a"
}

func movedEffectCells(line string) ([]string, bool) {
	parts := strings.Split(strings.TrimSpace(line), "|")
	if len(parts) < 2 || parts[0] != "" || parts[len(parts)-1] != "" {
		return nil, false
	}
	return parts[1 : len(parts)-1], true
}

func movedEffectPaths(code string) (paths []string) {
	for _, token := range backtickToken.FindAllString(code, -1) {
		path := strings.Trim(token, "`")
		if cut := strings.IndexAny(path, ":# "); cut >= 0 {
			path = path[:cut]
		}
		if strings.Contains(path, "/") {
			paths = append(paths, path)
		}
	}
	return paths
}

func movedEffectSeparator(cells []string) bool {
	if len(cells) != 4 {
		return false
	}
	for _, cell := range cells {
		if !strings.Contains(cell, "-") || strings.Trim(cell, "- \t") != "" {
			return false
		}
	}
	return true
}

func CheckMovedEffects(page []byte, exists func(repoPath string) bool) MovedEffectsReport {
	report := MovedEffectsReport{Inventory: "absent"}
	inCode, section, inTable, none, level, headingLine := false, false, false, false, 0, 0
	add := func(code string, line int, detail string) {
		report.Problems = append(report.Problems, MovedEffectProblem{code, detail, line})
	}
	for index, line := range strings.Split(string(page), "\n") {
		lineNumber := index + 1
		if fenceLine.MatchString(line) {
			inCode, inTable = !inCode, false
			continue
		}
		if inCode {
			continue
		}
		if !section {
			match := movedEffectsHeading.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			section, level, headingLine, report.Inventory = true, len(match[1]), lineNumber, "present"
			continue
		}
		if match := markdownHeading.FindStringSubmatch(line); match != nil && len(match[1]) <= level {
			break
		}
		if strings.TrimSpace(line) == "No owner moves." {
			none = true
		}
		cells, tableLine := movedEffectCells(line)
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if !inTable {
			inTable = tableLine && len(cells) == 4 && strings.EqualFold(strings.Join(cells, "|"), "Effect|From|To|Code")
			continue
		}
		if separatorRow.MatchString(strings.TrimSpace(line)) && movedEffectSeparator(cells) {
			continue
		}
		if !tableLine {
			inTable = false
			continue
		}
		if len(cells) != 4 {
			add("MOVED-EFFECT-MALFORMED-ROW", lineNumber, "inventory row must have exactly four cells")
			continue
		}
		row := MovedEffectRow{lineNumber, cells[0], cells[1], cells[2], cells[3], movedEffectPaths(cells[3])}
		report.Rows = append(report.Rows, row)
		checks := []struct {
			failed       bool
			code, detail string
		}{
			{row.Effect == "", "MOVED-EFFECT-NO-EFFECT", "effect is empty"},
			{movedEffectWeak(row.From), "MOVED-EFFECT-NO-OLD-OWNER", "old owner is empty or weak"},
			{movedEffectWeak(row.To), "MOVED-EFFECT-NO-NEW-OWNER", "new owner is empty or weak"},
			{strings.EqualFold(row.From, row.To), "MOVED-EFFECT-SAME-OWNER", "old and new owner are the same"},
			{len(row.Paths) == 0, "MOVED-EFFECT-NO-CODE", "code names no backticked repository path"},
		}
		for _, check := range checks {
			if check.failed {
				add(check.code, lineNumber, check.detail)
			}
		}
		for _, path := range row.Paths {
			if !exists(path) {
				add("MOVED-EFFECT-CODE-ABSENT", lineNumber, fmt.Sprintf("code path %q does not exist", path))
			}
		}
	}
	if !section {
		return report
	}
	if len(report.Rows) == 0 && none {
		report.Inventory = "none-declared"
	} else if len(report.Rows) == 0 {
		add("MOVED-EFFECTS-NO-ROWS", headingLine, "Moved effects section has no inventory rows")
	} else if none {
		add("MOVED-EFFECTS-CONTRADICTORY", headingLine, "inventory rows contradict No owner moves.")
	}
	return report
}
