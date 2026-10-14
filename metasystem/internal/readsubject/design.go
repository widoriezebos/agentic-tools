package readsubject

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const DesignInventoryHeading = "## The five questions, for every new owner and record"

type DesignCoverage struct {
	Row     string         `json:"row"`
	Where   string         `json:"where"`
	Answers []DesignAnswer `json:"answers"`
}

type DesignAnswer struct {
	Question   int    `json:"question"`
	Answer     string `json:"answer"`
	Evidence   string `json:"evidence"`
	Unanswered bool   `json:"unanswered"`
}

type DesignRead struct {
	RecordID    string            `json:"recordId"`
	Goals       []string          `json:"goals"`
	Root        string            `json:"root"`
	RawMaterial int               `json:"rawMaterial"`
	Sections    map[string]string `json:"sections"`
	Inventory   []string          `json:"inventory"`
	Coverage    []DesignCoverage  `json:"coverage"`
	Synthetic   []string          `json:"synthetic"`
}

// CollectDesignRead derives whole-page evidence without changing the critic's return.
func CollectDesignRead(id, root, recordID string, goals []string, subject ReadSubject, engine, model, output string, data []byte, verdict string) (Read, error) {
	r, err := Collect(id, subject, engine, model, output, data, verdict)
	if err != nil {
		return r, err
	}
	if subject.DesignPage == "" || recordID == "" || verdict == "" {
		return r, fmt.Errorf("design page or prose evidence is unavailable")
	}
	var raw struct {
		Digest   string           `json:"wholePageDigest"`
		Coverage []DesignCoverage `json:"coverage"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return r, err
	}
	if raw.Digest != subject.ContentDigest || fmt.Sprintf("%x", sha256.Sum256([]byte(subject.DesignPage))) != subject.ContentDigest {
		return r, fmt.Errorf("design return does not bind the frozen full page")
	}
	d := &DesignRead{RecordID: recordID, Goals: append([]string{}, goals...), Root: root, RawMaterial: r.Material,
		Sections: map[string]string{}, Inventory: []string{}, Coverage: raw.Coverage, Synthetic: []string{}}
	d.Sections, err = DesignSections(root, recordID, subject.DesignPage, nil, nil)
	if err != nil {
		return r, err
	}
	r.Design = d
	table, header, malformed, fenced := false, false, false, false
	for _, line := range strings.Split(subject.DesignPage, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if strings.HasPrefix(line, "#") && strings.HasPrefix(strings.TrimLeft(line, "#"), " ") && strings.Index(line, " ") <= 6 {
			table = line == DesignInventoryHeading
		}
		if !table || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if len(cells) != 6 {
			malformed = true
			continue
		}
		if !header {
			header = slices.Equal(cells, []string{"Function / record / act", "Production caller", "Freshness at the decision", "Person or agent", "Remedy that can succeed", "Unreadable input"})
			malformed = !header
			continue
		}
		if strings.HasPrefix(cells[0], "---") {
			continue
		}
		if cells[0] == "" || slices.Contains(d.Inventory, cells[0]) {
			malformed = true
			continue
		}
		d.Inventory = append(d.Inventory, cells[0])
	}
	add := func(key, where, change string) {
		for _, f := range r.Findings {
			if f.Material && f.Class == "incomplete-item" && f.Coverage == key && f.Where == where {
				return
			}
		}
		f := Finding{ID: id + ":inventory:" + key, Class: "incomplete-item", Severity: "high", Material: true,
			Claim: "The design leaves " + key + " unanswered", Evidence: "Frozen design page and returned coverage", Where: where, Change: change, Coverage: key}
		r.Findings = append(r.Findings, f)
		r.Material++
		d.Synthetic = append(d.Synthetic, f.ID)
	}
	if !header || malformed || len(d.Inventory) == 0 {
		add("page-inventory", strings.Split(subject.DesignPage, "\n")[0], "Supply the six-column inventory table beneath "+DesignInventoryHeading)
	}
	ids := map[string]bool{}
	for _, f := range r.Findings {
		if f.ID == "" || ids[f.ID] {
			return r, fmt.Errorf("design finding identifiers are missing or duplicate")
		}
		ids[f.ID] = true
		if _, exists := d.Sections[f.Where]; !exists && (f.Material || f.Where != "") {
			return r, fmt.Errorf("unresolvable design heading %q", f.Where)
		}
	}
	covered := map[string]bool{}
	for _, row := range raw.Coverage {
		if row.Row == "" || covered[row.Row] || d.Sections[row.Where] == "" || (slices.Contains(d.Inventory, row.Row) && row.Where != DesignInventoryHeading) {
			return r, fmt.Errorf("duplicate or unresolvable inventory coverage %q", row.Row)
		}
		covered[row.Row] = true
		questions := map[int]bool{}
		for _, cell := range row.Answers {
			if cell.Question < 1 || cell.Question > 5 || questions[cell.Question] || (!cell.Unanswered && (strings.TrimSpace(cell.Answer) == "" || strings.TrimSpace(cell.Evidence) == "")) {
				return r, fmt.Errorf("invalid answer for inventory row %q", row.Row)
			}
			questions[cell.Question] = true
			if cell.Unanswered {
				add(row.Row+":"+strconv.Itoa(cell.Question), row.Where, "Specify question "+strconv.Itoa(cell.Question)+" for "+row.Row+" in its owning Decision and acceptance test")
			}
		}
		if len(questions) != 5 {
			return r, fmt.Errorf("silently absent coverage for inventory row %q", row.Row)
		}
	}
	for _, row := range d.Inventory {
		if !covered[row] {
			return r, fmt.Errorf("silently absent inventory row %q", row)
		}
	}
	return r, nil
}
