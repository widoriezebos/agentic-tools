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
}

func (inv *intentInvocation) prepareDesignFold(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, decided map[string]string) error {
	read, err := inv.delivery().examinationRead(inv.layout.InstallationRoot.Path(), chain.NewestJob)
	if err != nil || read.Design == nil {
		return err
	}
	guard, err := diskstore.BoundExclusive(inv.designReviewEntryPath(plan.recordID) + ".exit-lock")
	if err != nil {
		return err
	}
	defer guard.Release()
	page, err := os.ReadFile(plan.design)
	if err != nil {
		return err
	}
	entry, err := inv.readDesignReviewEntry(plan.recordID)
	if err != nil {
		return err
	}
	fold := &goal.DesignExit{Expected: string(page), Page: string(page), ExaminedSHA256: read.Subject.ContentDigest}
	units, err := launch.DeclaredDesignUnits(string(page))
	if err != nil {
		return err
	}
	if len(units) == 0 {
		return fmt.Errorf("the revision needs declared source units")
	}
	pageUnits := slices.Clone(units)
	sections, allowed, mappings := designFoldSections(string(page)), map[string]bool{"## Units": true, "## Acceptance items": true}, map[string]designFoldMapping{}
	for _, line := range strings.Split(sections["## Acceptance items"], "\n") {
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
	for _, f := range read.Findings {
		if !f.Material || decided[f.ID] != "accepted" {
			continue
		}
		m, present := mappings[f.ID]
		if !present {
			return fmt.Errorf("finding %s needs a Fold item: JSON row under ## Acceptance items", f.ID)
		}
		if m.Decision != f.Where || m.Passage == "" || !strings.Contains(sections[m.Decision], m.Passage) || !strings.Contains(m.Passage, f.Change) || m.Checklist == "" || !strings.HasPrefix(m.Fixture, "group:") || len(m.Fixture) == 6 || len(m.Tests) == 0 {
			return fmt.Errorf("finding %s is not written in its owning Decision", f.ID)
		}
		unitRow := ""
		for _, line := range strings.Split(sections["## Units"], "\n") {
			if cells := strings.Split(line, "|"); len(cells) > 2 && strings.TrimSpace(cells[1]) == m.Unit {
				unitRow = line
			}
		}
		if unitRow == "" || !strings.Contains(unitRow, m.Checklist) {
			return fmt.Errorf("finding %s lacks its unit checklist", f.ID)
		}
		for _, test := range m.Tests {
			if !strings.Contains(test, "/Test") || !strings.Contains(unitRow, test) {
				return fmt.Errorf("finding %s lacks its public test mapping", f.ID)
			}
		}
		item := &goal.DesignItem{Exit: fold.Operation, DesignID: plan.recordID, Unit: m.Unit, Decision: m.Decision, Tests: m.Tests, Passage: m.Passage, Checklist: m.Checklist, Requirement: f.Change, Class: f.Class, Read: read.ID, Finding: f.ID, Evidence: f}
		fold.Obligations = append(fold.Obligations, goal.ReviewObligation{Finding: url.PathEscape(read.ID) + ":" + url.PathEscape(f.ID) + ":" + url.PathEscape(m.Unit), Chain: chain.Root, Artifact: relativeOrSame(inv.layout.GitRoot, plan.design), Test: strings.Join(m.Tests, ","), Fixture: m.Fixture, State: "open", DesignItem: item})
		allowed[m.Decision] = true
	}
	for _, id := range open {
		if !slices.ContainsFunc(read.Findings, func(item readsubject.Finding) bool { return item.Material && item.ID == id }) {
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
		return fmt.Errorf("%s", problem)
	}
	for _, other := range designs {
		if other.ID == plan.recordID || other.Status == "superseded" || other.Status == "done" {
			continue
		}
		declared, err := launch.DeclaredUnits(filepath.Join(inv.layout.GitRoot, other.Path))
		if err != nil {
			return err
		}
		if slices.ContainsFunc(declared, func(u launch.UnitSize) bool { return u.Lines > 250 }) {
			return fmt.Errorf("linked design %s still needs a bound size revision", other.ID)
		}
		units = append(units, declared...)
	}
	slices.SortFunc(units, func(a, b launch.UnitSize) int { return strings.Compare(a.Name, b.Name) })
	units = slices.CompactFunc(units, func(a, b launch.UnitSize) bool { return a.Name == b.Name })
	if len(units) > 5 && len(units) != len(pageUnits) {
		return fmt.Errorf("the resulting goal has %d units; excess scope needs a split before acceptance", len(units))
	}
	if _, sizeErr := launch.CheckDesignSize(string(page)); sizeErr != nil {
		projection, _, problem := inv.projection()
		if problem != nil {
			return fmt.Errorf("%s", problem.Summary)
		}
		source, _ := goalRecord(projection, plan.goalID)
		if source == nil || source.Origin != goal.OriginHuman {
			return fmt.Errorf("only a person-created source authorizes this follow-up")
		}
		fold.Destination, err = goal.NewOperationULID()
		if err != nil {
			return err
		}
		fold.Destination = "design-split-" + strings.ToLower(fold.Destination)
		fold.DestinationBriefPath = "plans/design-splits/" + fold.Destination + ".md"
		fold.DestinationBrief = fmt.Sprintf("Transferred from goal %s, design %s, critique %s: %s\n\nReshape this complete scope before implementation; preserve its dependencies and finding history.\n\n%s", plan.goalID, plan.recordID, chain.Root, sizeErr, page)
		for _, unit := range pageUnits {
			fold.TransferUnits = append(fold.TransferUnits, unit.Name)
		}
		fold.TransferObligations, fold.Obligations = fold.Obligations, nil
		if source.Risk == nil {
			return fmt.Errorf("the source needs risk answers before its follow-up can open; classify it with metasystem goal edit %s --risk severity=S,novelty=N,exposure=E,accumulation=A --basis TEXT", plan.goalID)
		}
		risk := fmt.Sprintf("severity=%d,novelty=%d,exposure=%d,accumulation=%d", source.Risk.Severity, source.Risk.Novelty, source.Risk.Exposure, source.Risk.Accumulation)
		fold.OpenCommand = shellCommand(inv.publicArgv("goal", "open", fold.Destination, "--intent", "Reshape transferred design scope from "+plan.goalID, "--tier", fmt.Sprint(source.Tier), "--blocked-by", plan.goalID, "--next", "Reshape scope in "+fold.DestinationBriefPath, "--risk", risk, "--basis", source.Risk.Basis, "--why", "Retain the source goal's tier for its transferred scope.", "--origin", "main", "--by", "NAME"))
	}
	entry.Fold = fold
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
