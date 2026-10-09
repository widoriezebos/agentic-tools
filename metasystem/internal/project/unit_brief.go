package project

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// UnitBriefSpec projects accepted text without granting acceptance or scope authority.
type UnitBriefSpec struct {
	Unit, Path, Source, Decision, Acceptance string
	Headings, Missing                        []string
	Size                                     launch.UnitSize
}

var unitTableSection = regexp.MustCompile(`(?mi)^#{1,6}[ \t]+(?:[0-9]+[.)][ \t]+)?Units\b[^\n]*\n`)
var decisionReference = regexp.MustCompile(`(?i)Decisions?\s+([0-9]+)(?:\s*[–−-]\s*([0-9]+))?`)
var decisionNumber = regexp.MustCompile(`(?i)^Decision\s+([0-9]+)(?:\D|$)`)
var decisionsSection = regexp.MustCompile(`(?i)^(?:[0-9]+[.)]\s+)?Decisions(?:\b|$)`)
var numberedDecision = regexp.MustCompile(`^([0-9]+)[.)](?:\s|$)`)
var decisionLink = regexp.MustCompile(`\]\([^)]*#([^)]*)\)`)

// SelectUnitBrief reads the request's accepted pages; duplicate ownership is a contradiction.
func SelectUnitBrief(pages map[string][]byte, paths []string, unit string) (UnitBriefSpec, error) {
	unit = strings.Trim(unit, "`")
	spec := UnitBriefSpec{Unit: unit}
	var names []string
	sizes := map[string][]launch.UnitSize{}
	for _, path := range paths {
		table := string(pages[path])
		for _, heading := range unitTableSection.FindAllStringIndex(table, -1) {
			section := table[heading[1]:]
			if end := strings.Index("\n"+section, "\n#"); end >= 0 {
				section = section[:end]
			}
			if len(launch.ParseUnitSizes(section)) > 0 {
				table = section
				break
			}
		}
		sizes[path] = launch.ParseUnitSizes(table)
		for _, row := range sizes[path] {
			names = append(names, strings.Trim(row.Name, "`"))
		}
	}
	if unit == "" && len(names) == 1 {
		unit, spec.Unit = names[0], names[0]
	}
	if unit == "" {
		spec.Missing = append(spec.Missing, "select one declared unit with --work NAME: "+strings.Join(names, ", "))
		return spec, nil
	}
	for _, path := range paths {
		data := pages[path]
		rows := sizes[path]
		owned := 0
		row := launch.UnitSize{Name: unit, Lines: -1}
		for _, candidate := range rows {
			if strings.Trim(candidate.Name, "`") == unit {
				owned++
				row = candidate
			}
		}
		if owned == 0 && regexp.MustCompile(`(?m)^#{1,6} `+regexp.QuoteMeta(unit)+`(?:\s|:|$)`).Match(data) {
			owned++
		}
		if owned == 0 {
			continue
		}
		if spec.Path != "" || owned > 1 {
			return spec, fmt.Errorf("unit %s has duplicate accepted ownership in %s and %s; repair the Units rows in those designs", unit, spec.Path, path)
		}
		spec.Path, spec.Size = path, row
		record, _, _ := ParseRecord(path, string(data))
		spec.Source = fmt.Sprintf("Design: %s; id: %s; body sha256: %x; unit: %s\n", path, record.ID, sha256.Sum256(data), unit)
		for _, field := range record.Head {
			if strings.Contains(strings.ToLower(field.Key), "convergence") || strings.EqualFold(field.Key, "DesignExit") {
				spec.Missing = append(spec.Missing, "published convergence body/items for "+path+"; the convergence owner must publish and validate them before this unit claims accepted items")
			}
		}
		refs, anchors := map[string]bool{}, map[string]bool{}
		for _, ref := range decisionReference.FindAllStringSubmatch(spec.Size.Row, -1) {
			first, _ := strconv.Atoi(ref[1])
			last := first
			if ref[2] != "" {
				last, _ = strconv.Atoi(ref[2])
			}
			if first < 1 || last < first || last-first >= strings.Count(string(data), "\n") {
				spec.Missing = append(spec.Missing, "the Decision range "+ref[0]+" in "+path)
				continue
			}
			for n := first; n > 0 && n <= last; n++ {
				refs[strconv.Itoa(n)] = true
			}
		}
		for _, link := range decisionLink.FindAllStringSubmatch(spec.Size.Row, -1) {
			anchors[link[1]] = true
		}
		unmatched := maps.Clone(refs)
		heading, selected, fenced := "", false, false
		depth, decisionsDepth := 0, 0
		for _, line := range strings.SplitAfter(string(data), "\n") {
			plain := strings.TrimSuffix(line, "\n")
			if strings.HasPrefix(plain, "```") || strings.HasPrefix(plain, "~~~") {
				fenced = !fenced
			}
			if !fenced && strings.HasPrefix(plain, "#") {
				heading = strings.TrimSpace(strings.TrimLeft(plain, "#"))
				number := decisionNumber.FindStringSubmatch(heading)
				level := len(plain) - len(strings.TrimLeft(plain, "#"))
				if level <= decisionsDepth {
					decisionsDepth = 0
				}
				if decisionsSection.MatchString(heading) {
					decisionsDepth = level
				} else if decisionsDepth > 0 && len(number) == 0 {
					number = numberedDecision.FindStringSubmatch(heading)
				}
				name := strings.ReplaceAll(heading, "`", "")
				if !selected || level <= depth {
					selected = len(number) > 1 && refs[number[1]] || len(refs) == 0 && len(anchors) == 0 && (strings.HasPrefix(name, unit+" ") || strings.HasPrefix(name, unit+":") || name == unit) || anchors[regexp.MustCompile(`[^\pL\pN_-]`).ReplaceAllString(strings.ReplaceAll(strings.ToLower(heading), " ", "-"), "")]
					if selected {
						depth = level
						duplicate := slices.Contains(spec.Headings, heading) || len(number) > 1 && refs[number[1]] && !unmatched[number[1]]
						if len(number) > 1 {
							delete(unmatched, number[1])
						}
						if duplicate {
							spec.Missing = append(spec.Missing, "ambiguous Decision heading "+heading+" in "+path)
						}
						spec.Headings = append(spec.Headings, heading)
					}
				}
			}
			if selected {
				spec.Decision += line
			}
			cells := strings.Split(strings.Trim(plain, " |\t"), "|")
			if len(cells) < 2 {
				continue
			}
			if strings.Trim(strings.TrimSpace(cells[0]), "`") == unit || strings.HasPrefix(strings.TrimSpace(cells[0]), unit+" `") {
				if strings.Contains(strings.ToLower(heading), "test") || strings.Contains(strings.ToLower(heading), "accept") || strings.Contains(strings.ToLower(heading), "item") {
					spec.Acceptance += line
				}
			}
		}
		if spec.Decision == "" {
			spec.Missing = append(spec.Missing, "the exact Decision mapping for unit "+unit+" in "+path)
		}
		for _, ref := range slices.Sorted(maps.Keys(unmatched)) {
			spec.Missing = append(spec.Missing, "Decision "+ref+" for unit "+unit+" in "+path)
		}
	}
	if spec.Path == "" {
		spec.Missing = append(spec.Missing, "the owning Units row and exact Decision for unit "+unit)
	}
	return spec, nil
}
