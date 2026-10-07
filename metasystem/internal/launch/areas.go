package launch

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

// DeclaredAreas reads pinned page bytes and unions page and unit-table edit
// areas. A missing declaration is unknown; an empty array declares no edits.
func DeclaredAreas(page string) ([]string, bool, error) {
	var areas []string
	known := false
	column := -1
	inTable := false
	for _, line := range strings.Split(page, "\n") {
		trimmed := strings.TrimSpace(line)
		declaration := strings.TrimPrefix(trimmed, "- ")
		if strings.HasPrefix(declaration, "Areas:") {
			values, err := areaValues(strings.TrimSpace(strings.TrimPrefix(declaration, "Areas:")))
			if err != nil {
				return nil, false, err
			}
			areas = append(areas, values...)
			known = true
		}
		cells := tableCells(line)
		if len(cells) > 0 && strings.EqualFold(strings.TrimSpace(cells[0]), "unit") {
			inTable = true
			column = -1
			for i, cell := range cells {
				if strings.EqualFold(strings.TrimSpace(cell), "areas") {
					column = i
				}
			}
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			inTable = false
		}
		if !inTable || column < 0 || separatorRow(cells) {
			continue
		}
		if len(cells) <= column || strings.TrimSpace(cells[column]) == "" {
			return nil, false, fmt.Errorf("unit areas are missing")
		}
		values, err := areaValues(strings.TrimSpace(cells[column]))
		if err != nil {
			return nil, false, err
		}
		areas = append(areas, values...)
		known = true
	}
	normalized, err := NormalizeAreas(areas)
	return normalized, known && err == nil, err
}

func areaValues(value string) ([]string, error) {
	if strings.HasPrefix(value, "[") && (strings.HasPrefix(strings.TrimSpace(value[1:]), "\"") || strings.TrimSpace(value[1:]) == "]") {
		var values []string
		if err := json.Unmarshal([]byte(value), &values); err != nil {
			return nil, err
		}
		return values, nil
	}
	if value == "" {
		return nil, fmt.Errorf("empty area declaration")
	}
	return []string{value}, nil
}

// NormalizeAreas rejects repository escapes and malformed glob syntax.
func NormalizeAreas(values []string) ([]string, error) {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.ReplaceAll(value, "\\", "/")
		if value == "" || strings.HasPrefix(value, "/") || (len(value) >= 2 && value[1] == ':' && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z'))) || strings.ContainsAny(value, "\x00\r\n") {
			return nil, fmt.Errorf("invalid repository area %q", value)
		}
		directory := strings.HasSuffix(value, "/")
		depth := 0
		for _, segment := range strings.Split(value, "/") {
			switch segment {
			case "", ".":
				continue
			case "..":
				depth--
				if depth < 0 {
					return nil, fmt.Errorf("area %q escapes the repository", value)
				}
			default:
				depth++
			}
		}
		value = path.Clean(value)
		if _, err := path.Match(value, ""); err != nil {
			return nil, fmt.Errorf("malformed area %q: %w", value, err)
		}
		for _, segment := range strings.Split(value, "/") {
			if strings.Contains(segment, "**") && segment != "**" {
				return nil, fmt.Errorf("** must occupy a full segment in %q", value)
			}
		}
		if directory && value != "." {
			value += "/"
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result, nil
}
