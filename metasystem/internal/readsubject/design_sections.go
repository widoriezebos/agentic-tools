package readsubject

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// DesignSections joins exact frozen anchors, with explicit renames bound to
// both page versions. New anchors receive new identities, never a fuzzy match.
func DesignSections(root, recordID, page string, previous *Read, decisions []byte) (map[string]string, error) {
	sections := map[string]string{}
	identity := root + "\n" + recordID
	if previous != nil {
		identity += "\n" + page
	}
	fenced := false
	for _, line := range strings.Split(page, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced || !strings.HasPrefix(line, "#") || !strings.HasPrefix(strings.TrimLeft(line, "#"), " ") || strings.Index(line, " ") > 6 {
			continue
		}
		if _, duplicate := sections[line]; duplicate {
			return nil, fmt.Errorf("ambiguous design heading %q", line)
		}
		sections[line] = fmt.Sprintf("%x", sha256.Sum256([]byte(identity+"\n"+line)))
	}
	if previous == nil {
		return sections, nil
	}
	if previous.Design == nil || previous.Design.Root != root || previous.Design.RecordID != recordID {
		return nil, fmt.Errorf("section history names another design or critique")
	}
	var mapping struct {
		From, To string
		Headings []struct{ From, To string }
	}
	count, fenced := 0, false
	for _, line := range strings.Split(string(decisions), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if !fenced && strings.HasPrefix(line, "Section mapping: ") {
			count++
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "Section mapping: ")), &mapping); err != nil {
				return nil, fmt.Errorf("unreadable section mapping: %w", err)
			}
		}
	}
	if count > 1 || count == 1 && (mapping.From != previous.Subject.ContentDigest || mapping.To != fmt.Sprintf("%x", sha256.Sum256([]byte(page)))) {
		return nil, fmt.Errorf("section mapping does not bind these two frozen pages")
	}
	from, to := map[string]bool{}, map[string]string{}
	for _, pair := range mapping.Headings {
		id := previous.Design.Sections[pair.From]
		if id == "" || sections[pair.To] == "" || from[pair.From] || to[pair.To] != "" {
			return nil, fmt.Errorf("ambiguous or unresolvable section mapping %q to %q", pair.From, pair.To)
		}
		from[pair.From], to[pair.To] = true, id
	}
	missing := false
	for heading := range previous.Design.Sections {
		if sections[heading] == "" && !from[heading] {
			missing = true
		}
	}
	for heading := range sections {
		if missing && to[heading] == "" && previous.Design.Sections[heading] == "" {
			return nil, fmt.Errorf("section identity is unknown: a removed heading and new heading %q need an explicit section mapping", heading)
		}
	}
	ids := map[string]bool{}
	for heading, id := range sections {
		if mapped := to[heading]; mapped != "" {
			id = mapped
		} else if prior := previous.Design.Sections[heading]; prior != "" && !from[heading] {
			id = prior
		}
		if ids[id] {
			return nil, fmt.Errorf("ambiguous section identity at %q", heading)
		}
		ids[id], sections[heading] = true, id
	}
	return sections, nil
}
