package launch

import (
	"regexp"
	"strconv"
	"strings"
)

var combinedCell = regexp.MustCompile(`^\s*([0-9]+)\s*\(\s*([0-9]+)\s*\)\s*$`)

// ParseUnitSizes retains sized unit rows, including conflicts, across tables.
// A production estimate is unknown unless the cell states one unambiguously.
func ParseUnitSizes(page string) []UnitSize {
	return parseUnitSizes(page, false)
}

// Build sizing uses the first sized table; design admission counts every table.
func parseUnitSizes(page string, firstTableOnly bool) []UnitSize {
	var rows []UnitSize
	unit, total, production := -1, -1, -1
	for _, line := range strings.Split(page, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			if firstTableOnly && unit >= 0 && (total >= 0 || production >= 0) {
				break
			}
			unit, total, production = -1, -1, -1
			continue
		}
		cells := tableCells(line)
		u, n, p := -1, -1, -1
		for i, cell := range cells {
			header := strings.ToLower(strings.Join(strings.Fields(cell), " "))
			switch {
			case header == "unit":
				u = i
			case header == "production lines":
				if p == -1 {
					p = i
				} else {
					p = -2
				}
			case strings.Contains(header, "line") || header == "size" || header == "alloc" || header == "cap":
				n = i
			}
		}
		if u >= 0 {
			unit, total, production = u, n, p
			continue
		}
		if unit < 0 || total < 0 && production < 0 || separatorRow(cells) || unit >= len(cells) {
			continue
		}
		row := UnitSize{Name: strings.TrimSpace(cells[unit]), Lines: -1, Row: line}
		if name := strings.ToLower(strings.Trim(row.Name, "*`_ ")); name == "total" || name == "sum" {
			continue
		}
		if total >= 0 && total < len(cells) {
			cell := cells[total]
			if first := firstInteger.FindString(cell); first != "" {
				row.Lines, _ = strconv.ParseInt(first, 10, 64)
			}
			if witness := witnessInteger.FindStringSubmatch(cell); len(witness) == 2 {
				extra, _ := strconv.ParseInt(witness[1], 10, 64)
				row.Lines += extra
			}
			if combined := combinedCell.FindStringSubmatch(cell); len(combined) == 3 {
				value, err := strconv.ParseInt(combined[2], 10, 64)
				if err == nil {
					row.Production = &value
				}
			}
		}
		if production >= 0 {
			var explicit *int64
			if production < len(cells) {
				value, err := strconv.ParseInt(strings.TrimSpace(cells[production]), 10, 64)
				if err == nil && value >= 0 {
					explicit = &value
				}
			}
			if explicit == nil || row.Production != nil && *row.Production != *explicit {
				row.Production = nil
			} else {
				row.Production = explicit
			}
		}
		if production == -2 {
			row.Production = nil
		}
		if total < 0 && row.Production != nil {
			row.Lines = *row.Production
		}
		rows = append(rows, row)
	}
	return rows
}
