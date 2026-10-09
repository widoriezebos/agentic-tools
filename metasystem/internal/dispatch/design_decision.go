package dispatch

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// recordDesignDecision runs inside the register transition. Immutable reads
// and the retained revision answer establish the completed count and repairs.
func recordDesignDecision(state critiqueState, rootID string, root, record map[string]any, read readsubject.Read) error {
	var history []readsubject.Read
	var fixed []readsubject.Finding
	for key, destination := range map[string]any{"designExaminations": &history, "designFixedSections": &fixed} {
		if value := root[key]; value != nil {
			data, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(data, destination); err != nil {
				return err
			}
		}
	}
	for _, prior := range history {
		if prior.Design == nil || prior.Design.Root != rootID || prior.Design.RecordID != read.Design.RecordID {
			return fmt.Errorf("design examination history has no matching canonical identity")
		}
	}
	if len(history) > 0 {
		previous := history[len(history)-1]
		owner := record
		if source := asString(record["examinationRetryOf"]); source != "" {
			owner = state.records[source]
		}
		parent := state.records[asString(owner["parentJob"])]
		after, _ := numInt(parent["round"])
		answer, err := frozenDesignDecisions(state.agents, rootID, read.Design.RecordID, asString(owner["jobId"]), asString(owner["operationId"]), after)
		if err != nil {
			return err
		}
		accepted, table, fenced := map[string]bool{}, false, false
		for _, line := range strings.Split(string(answer), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
				fenced = !fenced
			}
			if fenced {
				continue
			}
			cells := strings.SplitN(strings.TrimPrefix(line, "|"), "|", 3)
			for i := range cells {
				cells[i] = strings.TrimSpace(cells[i])
			}
			if !strings.HasPrefix(line, "|") || len(cells) != 3 {
				table = false
				continue
			}
			if cells[0] == "Finding id" {
				table = true
			}
			if table && cells[1] == "accepted" {
				accepted[cells[0]] = true
			}
		}
		old, current := designSectionBytes(previous), designSectionBytes(read)
		for _, finding := range previous.Findings {
			section := previous.Design.Sections[finding.Where]
			if finding.Material && accepted[finding.ID] && section != "" && current[section] != "" && old[section] != current[section] {
				finding.Where = section
				finding.Evidence = fmt.Sprintf("finding %s accepted; revision %s -> %s", finding.ID, previous.Subject.ContentDigest, read.Subject.ContentDigest)
				fixed = append(fixed, finding)
			}
		}
	}
	limit, valid := numInt(root["designExaminationLimit"])
	if _, present := root["designExaminationLimit"]; present && (!valid || limit < 0 || limit > 4) {
		return fmt.Errorf("the frozen design examination allowance is unreadable")
	}
	if !valid {
		limit = DesignRoundLimit(filepath.Dir(filepath.Dir(state.agents)), rootID, 4)
		root["designExaminationLimit"] = limit
	}
	normalized := read
	normalized.Findings = append([]readsubject.Finding(nil), read.Findings...)
	for i := range normalized.Findings {
		normalized.Findings[i].Where = read.Design.Sections[normalized.Findings[i].Where]
	}
	path := filepath.Join(checkoutTop(filepath.Dir(filepath.Dir(state.agents))), filepath.FromSlash(read.Subject.DesignPath))
	decision := loopstop.Decide(loopstop.Input{Stop: loopstop.Stop{Loop: "design-round", Subject: rootID, Scope: path, Tree: read.Subject.ContentDigest,
		Attempt: len(history) + 1, Budget: int(limit), Handoff: "fold or split", Evidence: read.ID}, Prior: history, Fixed: fixed, Read: &normalized, Policy: "auto"})
	root["designExaminations"], root["designFixedSections"], root["designDecision"] = append(history, read), fixed, decision
	if decision.Decision == "stop" {
		decision.Required = []string{fmt.Sprintf("metasystem design review '%s' --dispositions FILE", strings.ReplaceAll(path, "'", "'\\''"))}
		root["designDecision"], root["designStop"], root["findingRegisterStop"] = decision, decision, decision
	}
	return nil
}

func designSectionBytes(read readsubject.Read) map[string]string {
	sections, section, fenced := map[string]string{}, "", false
	for _, line := range strings.SplitAfter(read.Subject.DesignPage, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
			fenced = !fenced
		}
		if !fenced {
			if id := read.Design.Sections[strings.TrimSuffix(line, "\n")]; id != "" {
				section = id
			}
		}
		sections[section] += line
	}
	return sections
}

// LimitDesignCorrections retains a lower correction bound without changing
// the frozen examination allowance or reopening a stopped decision.
func LimitDesignCorrections(repoRoot, rootID string, corrections int) error {
	_, err := withFindingRegisterLock(repoRoot, func() (string, error) {
		return "", withRecordLock(repoRoot, rootID, func(path string) error {
			root, err := readObject(path)
			if err != nil {
				return err
			}
			data, err := json.Marshal(root["designDecision"])
			if err != nil {
				return err
			}
			var decision loopstop.Stop
			if err := json.Unmarshal(data, &decision); err != nil {
				return err
			}
			if decision.Decision != "continue" || decision.Attempt <= corrections {
				return nil
			}
			decision.Decision, decision.Class, decision.Handoff = "stop", "review.stop correction allowance spent", "fold or split"
			decision.Required = []string{fmt.Sprintf("metasystem design review %q --dispositions FILE", decision.Scope)}
			root["designDecision"], root["designStop"], root["findingRegisterStop"] = decision, decision, decision
			return writeRecord(path, root)
		})
	})
	return err
}
