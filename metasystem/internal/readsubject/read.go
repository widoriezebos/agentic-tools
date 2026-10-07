package readsubject

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Read is one immutable examination, independent of its transport or runner.
type Read struct {
	ID          string      `json:"id"`
	Subject     ReadSubject `json:"subject"`
	Engine      string      `json:"engine"`
	Model       string      `json:"model"`
	Material    int         `json:"material"`
	Findings    []Finding   `json:"findings"`
	Output      string      `json:"output"`
	CarriedFrom string      `json:"carriedFrom,omitempty"`
}

type Finding struct {
	ID       string `json:"id"`
	Class    string `json:"class"`
	Severity string `json:"severity"`
	Material bool   `json:"material"`
	Claim    string `json:"claim"`
	Evidence string `json:"evidence"`
	Where    string `json:"where"`
	Change   string `json:"change"`
	Resolves string `json:"resolves,omitempty"`
	Relation string `json:"relation,omitempty"`
}

var FindingClasses = []string{"regression", "weakened-test", "incomplete-item", "false-premise", "faked-seam", "missing-reader", "scope", "other"}

// Collect validates structured evidence before assigning examination-qualified
// identities. A prose verdict can contradict evidence, but cannot replace it.
func Collect(id string, subject ReadSubject, engine, model, output string, data []byte, verdict string) (Read, error) {
	r := Read{ID: id, Subject: subject, Engine: engine, Model: model, Output: output}
	var raw struct {
		Findings json.RawMessage `json:"findings"`
		Count    *int            `json:"verdictMaterialCount"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return r, err
	}
	if raw.Count == nil || len(raw.Findings) == 0 || string(raw.Findings) == "null" {
		return r, fmt.Errorf("structured findings and their material count are required")
	}
	var fields []map[string]json.RawMessage
	if err := json.Unmarshal(raw.Findings, &fields); err != nil {
		return r, err
	}
	if err := json.Unmarshal(raw.Findings, &r.Findings); err != nil {
		return r, err
	}
	for i := range r.Findings {
		f := &r.Findings[i]
		if (string(fields[i]["material"]) != "true" && string(fields[i]["material"]) != "false") || !slices.Contains(FindingClasses, f.Class) || !slices.Contains([]string{"critical", "high", "medium", "low"}, f.Severity) || strings.TrimSpace(f.Claim) == "" || strings.TrimSpace(f.Evidence) == "" || strings.TrimSpace(f.Change) == "" {
			return r, fmt.Errorf("finding %d has incomplete stop evidence", i+1)
		}
		f.Where = path.Clean(f.Where)
		if f.Where == "" || path.IsAbs(f.Where) || strings.Contains(f.Where, "\\") || slices.Contains(strings.Split(f.Where, "/"), "..") || f.Where == "." || regexp.MustCompile(`(^[A-Za-z]:|:[0-9]+(?:-[0-9]+)?$)`).MatchString(f.Where) {
			return r, fmt.Errorf("finding %d needs a repository-relative path", i+1)
		}
		if f.Class == "other" && strings.TrimSpace(f.Relation) == "" {
			return r, fmt.Errorf("finding %d of class other must name its rule", i+1)
		}
		f.ID = id + ":" + strconv.Itoa(i+1)
		if f.Material {
			r.Material++
		}
	}
	if *raw.Count != r.Material {
		return r, fmt.Errorf("structured material count %d disagrees with %d material findings", *raw.Count, r.Material)
	}
	if verdict != "" && verdict != "none" {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(verdict), "VERDICT:"))
		count := 0
		if !strings.EqualFold(text, "LAND") {
			patterns := []*regexp.Regexp{regexp.MustCompile(`(?i)material=([0-9]+)`), regexp.MustCompile(`(?i)([0-9]+) material findings?`)}
			matched := false
			for _, pattern := range patterns {
				match := pattern.FindStringSubmatch(text)
				if len(match) == 2 {
					count, _ = strconv.Atoi(match[1])
					matched = true
					break
				}
			}
			if !matched {
				return r, fmt.Errorf("prose verdict has no material count")
			}
		}
		if count != r.Material {
			return r, fmt.Errorf("prose verdict disagrees with the structured findings")
		}
	}
	return r, nil
}

func (r Read) Canonical() ([]byte, string) {
	data, _ := json.Marshal(r)
	return data, fmt.Sprintf("%x", sha256.Sum256(data))
}
