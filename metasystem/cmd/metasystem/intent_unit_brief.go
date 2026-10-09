package main

import (
	"crypto/sha256"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"os"
	"path/filepath"
	"strings"
)

func (inv *intentInvocation) composeUnitBrief(plan launch.UnitPlan, directory string, revision *launch.UnitRevisionRequest) (launch.UnitPlan, error) {
	data, err := os.ReadFile(plan.Build.Brief)
	if err != nil {
		return plan, err
	}
	command := "metasystem test run --unit-run " + filepath.Base(filepath.Dir(directory))
	prefix := "Before returning, run: " + command + "\n\n"
	supplied := strings.TrimPrefix(string(data), prefix)
	person := inv.directPersonProof("retain supplementary brief text") == nil || plan.Check.SelectedBy != "" || revision != nil && revision.Person != ""
	originalPersonBrief := revision != nil && revision.Rebase != nil && plan.BriefSuppliedByPerson
	if revision == nil || revision.Rebase == nil {
		plan.BriefSuppliedByPerson = person
	}
	var unchecked []string
	fault := func(err error) error {
		if !person {
			return err
		}
		if revision == nil || revision.Person == "" {
			actor, _, problem := inv.actingAs("retain unchecked brief", plan.Goal, actorHuman)
			if problem != nil {
				return fmt.Errorf("%s", problem.Summary)
			}
			impact := "Impact: launch with unknown composition evidence.\nThis proves no accepted items. Cancel the run to stop it."
			if writeErr := inv.recordUnitStopOverride(plan.Goal, "work-brief-unchecked", "retain supplied specification", impact, unitStopActor(actor)); writeErr != nil {
				return writeErr
			}
		}
		unchecked = append(unchecked, "Unchecked composition evidence: "+strings.ReplaceAll(err.Error(), "\n", "; "))
		return nil
	}
	for _, check := range suppliedBriefChecks(supplied) {
		if check != command && check != "Pending: committed proof.cheap, proof.audits and proof.deadline; the unit runner allocates the run id." && !person && !originalPersonBrief {
			return plan, fmt.Errorf("the supplied Check conflicts with the frozen declaration; regenerate supplementary input with metasystem work brief %s --work %s --out FILE, then repeat this build or revision; a person changes execution through work build --reason TEXT --by NAME --check COMMAND", plan.Goal, plan.Unit)
		}
	}
	head, err := inv.work().git(plan.Worktree, "rev-parse", "HEAD")
	if err != nil {
		return plan, err
	}
	referenceBase := strings.TrimSpace(string(head))
	designs, problem := inv.acceptedDesignPaths(plan.Goal)
	if problem != nil {
		if err := fault(fmt.Errorf("%s", problem.Summary)); err != nil {
			return plan, err
		}
	}
	var decision, readers, limits, acceptance, returns, sources string
	owners := 0
	for _, path := range designs {
		body, err := os.ReadFile(path)
		if err != nil {
			return plan, err
		}
		bodyDigest := sha256.Sum256(body)
		base := referenceBase
		if problem := inv.checkBriefCitations(&body, path, &base); problem != nil {
			if err := fault(fmt.Errorf("%s", problem.Summary)); err != nil {
				return plan, err
			}
		}
		if strings.HasPrefix(string(body), "## Citation constraints") {
			if err := fault(fmt.Errorf("citation evidence is unchecked")); err != nil {
				return plan, err
			}
		}
		spec, selectErr := project.SelectUnitBrief(map[string][]byte{path: body}, []string{path}, plan.Unit)
		if spec.Decision != "" && len(spec.Missing) > 0 {
			selectErr = fmt.Errorf("%s", strings.Join(spec.Missing, "; "))
		}
		if selectErr != nil {
			if err := fault(selectErr); err != nil {
				return plan, err
			}
		}
		d := spec.Decision
		r := readerSpec(body, plan.Unit)
		if spec.Size.Production != nil {
			sources += fmt.Sprintf("Production estimate: %d.\n", *spec.Size.Production)
		}
		acceptance += spec.Acceptance
		if d == "" {
			continue
		}
		owners++
		decision, readers = decision+d, readers+r
		sources += fmt.Sprintf("Design: %s; body sha256: %x\n", path, bodyDigest)
		c, ret, a := designSections(path, body, spec)
		for _, section := range c {
			limits += section.text + "\n"
		}
		for _, section := range a {
			acceptance += section.text + "\n"
		}
		for _, section := range ret {
			returns += section.text + "\n"
		}
	}
	if owners > 1 || len(designs) > 0 && owners == 0 && inv.designGateFacts(inv.layout.InstallationRoot.Path(), plan.Goal).Mode == "refuse" {
		if err := fault(fmt.Errorf("unit %s has %d accepted Decision owners; repair the unit's Decision mapping in its accepted design", plan.Unit, owners)); err != nil {
			return plan, err
		}
	}
	if owners == 0 {
		decision = "No selected accepted Decision is available; the supplied specification is quoted below and claims no accepted items.\n"
	}
	after, dispositions, correction, evidenceTree := 0, []byte(nil), []byte(nil), plan.Worktree
	if revision != nil {
		if revision.Rebase == nil {
			after, dispositions, correction = revision.After, revision.Decisions, revision.Brief
		}
		if predecessor, err := inv.work().units(inv.layout).Status(revision.Run); err == nil {
			evidenceTree = predecessor.Worktree
		}
	}
	evidence, err := inv.work().units(inv.layout).BriefEvidence(inv.layout.GitRoot, evidenceTree, plan.Goal, plan.Unit, after, dispositions, correction)
	if err != nil {
		if err := fault(err); err != nil {
			return plan, err
		}
		evidence, _ = inv.work().units(inv.layout).BriefEvidence(inv.layout.GitRoot, evidenceTree, plan.Goal, plan.Unit, 0, nil)
	}
	var decisions string
	if after > 0 {
		parts := strings.SplitN(evidence, "# Avoid these recurring defects", 2)
		if len(parts) == 2 {
			decisions, evidence = parts[0], "# Avoid these recurring defects"+parts[1]
		}
	}
	var out strings.Builder
	out.WriteString(prefix)
	fmt.Fprintf(&out, "# Goal\n\nUnit %s of goal %s.\n\n%s\n# Workspace\n\nBranch goal/%s, worktree %s; base %s. Leave changes uncommitted.\n\n# Units\n\n", plan.Unit, plan.Goal, decisions, plan.Goal, plan.Worktree, referenceBase)
	if lines, err := launch.DeclaredUnitLines(plan.Build.UnitsPage, plan.Unit); err == nil {
		fmt.Fprintf(&out, "| Unit | Changed lines |\n| --- | ---: |\n| %s | %d |\n", plan.Unit, lines)
	}
	fmt.Fprintf(&out, "\n# What this unit builds\n\n%s\n%s\n## Supplementary brief (non-executable quoted text)\n\n[begin supplementary brief]\n> %s\n[end supplementary brief]\n\n# Not in this unit\n\n%s\n", sources, decision, strings.ReplaceAll(supplied, "\n", "\n> "), limits)
	readerSections := strings.SplitN(inv.briefReaderSections(decision, readers, limits, referenceBase), "# Deletion rules", 2)
	out.WriteString(readerSections[0])
	fmt.Fprintf(&out, "\n# A test through the public verb\n\n%s\n# Check\n\n%s\n", acceptance, command)
	if plan.Check.SelectedBy != "" {
		fmt.Fprintf(&out, "Manual check selected by %s: %s.\n", plan.Check.SelectedBy, plan.Check.Reason)
	}
	out.WriteString("\n# Deletion rules" + readerSections[1])
	fmt.Fprintf(&out, "\n# Constraints\n\n%s\n%s\n%s\n# Expected Return\n\n%s\nThe change, uncommitted; report proof and remaining risk.\n\n# Acceptance Criteria\n\n%s\n", limits, strings.Join(unchecked, "\n"), evidence, returns, acceptance)
	plan.Build.Brief = filepath.Join(directory, "composed-build.md")
	_, err = atomicfile.WriteText(plan.Build.Brief, out.String(), directory)
	return plan, err
}
func suppliedBriefChecks(text string) []string {
	var checks []string
	active, fenced := false, false
	var check strings.Builder
	flush := func() {
		if active && strings.TrimSpace(check.String()) != "" {
			checks = append(checks, strings.Trim(strings.TrimSpace(check.String()), "`"))
		}
		check.Reset()
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
			fenced = !fenced
			continue
		}
		if !fenced && strings.HasPrefix(line, "#") {
			flush()
			active = strings.EqualFold(strings.TrimSpace(strings.TrimLeft(line, "#")), "Check")
		} else if !fenced && active {
			check.WriteString(line + "\n")
		}
	}
	flush()
	return checks
}
