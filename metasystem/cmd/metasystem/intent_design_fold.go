package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

type designFoldMapping struct {
	Finding, Unit, Decision, Passage, Checklist, Fixture string
	Tests                                                []string
	Unresolved                                           bool
}

func (inv *intentInvocation) prepareDesignFold(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, decided map[string]string) error {
	read, err := inv.delivery().examinationRead(inv.layout.InstallationRoot.Path(), chain.NewestJob)
	if err != nil || read.Design == nil {
		return err
	}
	current, err := os.ReadFile(plan.design)
	if err != nil {
		return err
	}
	page, prepared, err := inv.finalDesignRevision(plan, chain, read, current)
	if err != nil {
		return fmt.Errorf("author revision: %w", err)
	}
	guard, err := diskstore.BoundExclusive(inv.designReviewEntryPath(plan.recordID) + ".exit-lock")
	if err != nil {
		return err
	}
	defer guard.Release()
	entry, err := inv.readDesignReviewEntry(plan.recordID)
	if err != nil {
		return err
	}
	if entry.Fold == nil || entry.Fold.Operation != prepared.Operation {
		return fmt.Errorf("the prepared revision changed; rejoin it before publication")
	}
	fold := *prepared
	fold.Page, fold.ExaminedSHA256 = string(page), read.Subject.ContentDigest
	fold.Obligations, fold.TransferObligations, fold.TransferUnits, fold.Scope = nil, nil, nil, nil
	units, err := launch.DeclaredDesignUnits(string(page))
	if err != nil {
		return err
	}
	if len(units) == 0 {
		return fmt.Errorf("the revision needs declared source units")
	}
	priorUnits, err := launch.DeclaredDesignUnits(read.Subject.DesignPage)
	if err != nil {
		return err
	}
	for _, prior := range priorUnits {
		if !slices.ContainsFunc(units, func(u launch.UnitSize) bool { return u.Name == prior.Name }) {
			return fmt.Errorf("the revision loses source unit %s before transfer", prior.Name)
		}
	}
	pageUnits := slices.Clone(units)
	sections, allowed, mappings := designFoldSections(string(page)), map[string]bool{"## Units": true, "## Acceptance items": true}, map[string]designFoldMapping{}
	for _, line := range strings.Split(sections["## Acceptance items"], "\n") {
		if raw, present := strings.CutPrefix(line, "Scope item: "); present {
			var scope goal.DesignUnitScope
			if err := json.Unmarshal([]byte(raw), &scope); err != nil {
				return err
			}
			fold.Scope = append(fold.Scope, scope)
		}
		if raw, present := strings.CutPrefix(line, "Fold item: "); present {
			var mapping designFoldMapping
			if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
				return err
			}
			if _, duplicate := mappings[mapping.Finding]; duplicate {
				return fmt.Errorf("finding %s has duplicate Decision mappings", mapping.Finding)
			}
			mappings[mapping.Finding] = mapping
		}
	}
	open, err := dispatchcore.CritiqueOpenFindingIDs(inv.layout.InstallationRoot.Path(), chain.Root)
	if err != nil {
		return err
	}
	root, err := inv.jobRecord(chain.Root)
	if err != nil {
		return err
	}
	historyBytes, err := json.Marshal(root["designExaminations"])
	if err != nil {
		return err
	}
	var history []readsubject.Read
	if err := json.Unmarshal(historyBytes, &history); err != nil {
		return err
	}
	seen, origins := map[string]bool{}, map[string]string{}
	findings := slices.Clone(read.Findings)
	for _, prior := range history {
		for _, f := range prior.Findings {
			if !f.Material || !slices.Contains(open, f.ID) || seen[f.ID] {
				continue
			}
			for heading, stable := range read.Design.Sections {
				if stable == f.Where {
					f.Where = heading
					break
				}
			}
			if !slices.ContainsFunc(findings, func(current readsubject.Finding) bool { return current.ID == f.ID }) {
				findings = append(findings, f)
			}
			origins[f.ID], seen[f.ID] = prior.ID, true
		}
	}
	for _, f := range findings {
		decision, answered := decided[f.ID]
		if !f.Material || answered && decision != "accepted" || !answered && !slices.Contains(open, f.ID) {
			continue
		}
		m, present := mappings[f.ID]
		if !present {
			return fmt.Errorf("finding %s needs a Fold item: JSON row under ## Acceptance items", f.ID)
		}
		if !slices.ContainsFunc(pageUnits, func(u launch.UnitSize) bool { return u.Name == m.Unit }) || m.Decision != f.Where || sections[m.Decision] == "" {
			return fmt.Errorf("finding %s has no owning unit or Decision", f.ID)
		}
		if !m.Unresolved && (m.Passage == "" || !strings.Contains(sections[m.Decision], m.Passage) || !strings.Contains(m.Passage, f.Change) || m.Checklist == "" || !strings.HasPrefix(m.Fixture, "group:") || len(m.Fixture) == 6 || len(m.Tests) == 0) {
			return fmt.Errorf("finding %s is not written in its owning Decision", f.ID)
		}
		unitRow := ""
		for _, line := range strings.Split(sections["## Units"], "\n") {
			if cells := strings.Split(line, "|"); len(cells) > 2 && strings.TrimSpace(cells[1]) == m.Unit {
				unitRow = line
			}
		}
		if !m.Unresolved && (unitRow == "" || !strings.Contains(unitRow, m.Checklist)) {
			return fmt.Errorf("finding %s lacks its unit checklist", f.ID)
		}
		for _, test := range m.Tests {
			if !strings.Contains(test, "/Test") || !strings.Contains(unitRow, test) {
				return fmt.Errorf("finding %s lacks its public test mapping", f.ID)
			}
		}
		origin := origins[f.ID]
		if origin == "" {
			origin = read.ID
		}
		item := &goal.DesignItem{Exit: fold.Operation, DesignID: plan.recordID, Unit: m.Unit, Decision: m.Decision, Tests: m.Tests, Passage: m.Passage, Checklist: m.Checklist, Requirement: f.Change, Class: f.Class, Read: origin, Finding: f.ID, Evidence: f}
		fold.Obligations = append(fold.Obligations, goal.ReviewObligation{Finding: url.PathEscape(origin) + ":" + url.PathEscape(f.ID) + ":" + url.PathEscape(m.Unit), Chain: chain.Root, Artifact: relativeOrSame(inv.layout.GitRoot, plan.design), Test: strings.Join(m.Tests, ","), Fixture: m.Fixture, State: "open", DesignItem: item})
		allowed[m.Decision] = true
	}
	for _, id := range open {
		if !slices.ContainsFunc(findings, func(item readsubject.Finding) bool { return item.Material && item.ID == id }) {
			return fmt.Errorf("earlier finding %s still needs a bound revision; acceptance remains pending", id)
		}
	}
	original := designFoldSections(read.Subject.DesignPage)
	for _, set := range []map[string]string{original, sections} {
		for heading := range set {
			if !allowed[heading] && sections[heading] != original[heading] {
				return fmt.Errorf("the revision changes unrelated section %s", heading)
			}
		}
	}
	designs, problem := inv.linkedDesigns(plan.goalID)
	if problem != "" {
		return fmt.Errorf("linked designs: %s", problem)
	}
	for _, other := range designs {
		if other.ID == plan.recordID || other.Status == "superseded" || other.Status == "done" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(inv.layout.GitRoot, other.Path))
		if err != nil {
			return err
		}
		declared, err := launch.CheckDesignSize(string(data))
		if err != nil {
			return fmt.Errorf("linked design %s still needs a bound size revision: %w", other.ID, err)
		}
		units = append(units, declared...)
	}
	slices.SortFunc(units, func(a, b launch.UnitSize) int { return strings.Compare(a.Name, b.Name) })
	units = slices.CompactFunc(units, func(a, b launch.UnitSize) bool { return a.Name == b.Name })
	if err := partitionDesignFold(&fold, pageUnits, mappings, 5-(len(units)-len(pageUnits))); err != nil {
		return err
	}
	if len(fold.TransferUnits) > 0 {
		if plan.goalID == "" {
			fold.DestinationBrief = "Retained goal-free transfer proposal for units " + strings.Join(fold.TransferUnits, ", ") + ".\n\n" + string(page)
			entry.Fold = &fold
			if err := inv.writeDesignReviewEntry(plan.recordID, entry); err != nil {
				return err
			}
			return fmt.Errorf("%w: the goal-free transfer remains a draft proposal; no goal is opened", errDesignRevisionPending)
		}
		projection, _, problem := inv.projection()
		if problem != nil {
			return fmt.Errorf("%s", problem.Summary)
		}
		source, _ := goalRecord(projection, plan.goalID)
		if !source.CanOpenDesignFollowUp() {
			return fmt.Errorf("only a person-created source authorizes this follow-up")
		}
		if fold.Destination == "" {
			fold.Destination, err = goal.NewOperationULID()
			if err != nil {
				return err
			}
			fold.Destination = "design-split-" + strings.ToLower(fold.Destination)
		}
		fold.DestinationBriefPath = "plans/design-splits/" + fold.Destination + ".md"
		fold.DestinationBrief = fmt.Sprintf("Transferred from goal %s, design %s, critique %s. Queued immediately after %s.\n\nReshape only these transferred units: %s. Preserve their Decision sections, dependencies and finding history from this complete bound revision:\n\n%s", plan.goalID, plan.recordID, chain.Root, plan.goalID, strings.Join(fold.TransferUnits, ", "), page)
		if source.Risk == nil {
			return fmt.Errorf("the source needs risk answers before its follow-up can open; classify it with metasystem goal edit %s --risk severity=S,novelty=N,exposure=E,accumulation=A --basis TEXT", plan.goalID)
		}
		risk := fmt.Sprintf("severity=%d,novelty=%d,exposure=%d,accumulation=%d", source.Risk.Severity, source.Risk.Novelty, source.Risk.Exposure, source.Risk.Accumulation)
		fold.OpenCommand = shellCommand(inv.publicArgv("goal", "open", fold.Destination, "--intent", "Reshape transferred design scope from "+plan.goalID, "--tier", fmt.Sprint(source.Tier), "--blocked-by", plan.goalID, "--next", "Reshape scope in "+fold.DestinationBriefPath, "--risk", risk, "--basis", source.Risk.Basis, "--why", "Retain the source goal's tier for its transferred scope.", "--origin", "main", "--by", "NAME"))
	}
	entry.Fold = &fold
	return inv.writeDesignReviewEntry(plan.recordID, entry)
}
func designFoldSections(page string) map[string]string {
	sections, heading, fenced := map[string]string{}, "", false
	for _, line := range strings.SplitAfter(page, "\n") {
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(line, "#") {
			heading = strings.TrimSuffix(line, "\n")
		}
		sections[heading] += line
	}
	return sections
}
