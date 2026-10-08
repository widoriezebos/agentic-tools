package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

func (inv *intentInvocation) ruleDesign() intentResult {
	if problem := inv.directPersonProof("design review scope or ruling"); problem != nil {
		return *problem
	}
	actor, proof, problem := inv.actingAs("design review", "", actorHuman)
	if problem != nil {
		return *problem
	}
	fail := func(err error) intentResult {
		return intentResult{Outcome: intentFailed, code: 1, Summary: "the person's design act remains pending: " + err.Error(), next: inv.sameCommand(), nextReason: "resume the retained act after repairing the physical cause"}
	}
	if inv.input.has("scope") == inv.input.has("ruling") || strings.TrimSpace(inv.input.text("reason")) == "" || inv.input.text("by") == "" {
		return fail(fmt.Errorf("name exactly one --scope FILE or --ruling TEXT, with --reason TEXT and --by NAME"))
	}
	for _, flag := range []string{"dispositions", "retry", "after", "check-only"} {
		if inv.input.has(flag) {
			return fail(fmt.Errorf("the person's act cannot also request --%s", flag))
		}
	}
	path := canonicalDocument(inv.textPath(inv.input.args[0]))
	state, err := inv.owners.resolver.RootForInstallation(inv.layout.InstallationRoot)
	rel, relErr := filepath.Rel(inv.layout.GitRoot, path)
	home, inHome := project.HomeFor(project.Roots{Checkout: inv.layout.GitRoot, Installation: inv.layout.InstallationRoot, StateRoot: state}, filepath.ToSlash(rel))
	if err != nil || relErr != nil || !inHome || home.Kind != project.KindDesign || strings.HasPrefix(rel, "..") {
		return fail(fmt.Errorf("name a design in this project's design home"))
	}
	record, current, err := inv.delivery().designRecord(path)
	if err != nil {
		return fail(err)
	}
	if len(record.Goals) != 1 || inv.input.has("goal") && inv.input.text("goal") != record.Goals[0] {
		return fail(fmt.Errorf("the act must preserve the design's one identified goal"))
	}
	page, ruling := current, inv.input.text("ruling")
	if inv.input.has("scope") {
		page, err = os.ReadFile(inv.flagPath("scope"))
		ruling = "scope replacement " + inv.input.text("scope")
	}
	if err != nil || strings.TrimSpace(ruling) == "" {
		return fail(fmt.Errorf("scope must be readable and ruling must be nonempty: %v", err))
	}
	replacement, problems, ok := project.ParseRecord(path, string(page))
	if !ok || len(problems) != 0 || replacement.ID != record.ID || replacement.Kind != project.KindDesign || strings.Join(replacement.Goals, ",") != strings.Join(record.Goals, ",") {
		return fail(fmt.Errorf("replacement scope must retain this design's identity and goal; change goal intent through goal revise"))
	}
	pageDigest := digestText(current)
	projection, _, projectionProblem := inv.projection()
	if projectionProblem != nil {
		return fail(fmt.Errorf("goal state: %s", projectionProblem.Summary))
	}
	if file, _ := goalRecord(projection, record.Goals[0]); file != nil {
		for i := len(file.DesignExits) - 1; i >= 0; i-- {
			exit := file.DesignExits[i]
			if exit.DesignID != record.ID {
				continue
			}
			if exit.State == "committed" && exit.Page == string(current) && exit.Ruling == ruling && exit.Reason == inv.input.text("reason") && exit.Who == unitStopActor(actor) {
				body, bodyErr := project.DesignBodyDigest(path, page)
				if bodyErr == nil && body == exit.BodySHA256 {
					pageDigest = digestText([]byte(exit.Expected))
				}
			}
			break
		}
	}
	identity := []string{path, ruling, inv.input.text("reason"), unitStopActor(actor), pageDigest}
	if inv.input.has("scope") {
		identity = append(identity, string(page))
	}
	request, _ := json.Marshal(identity)
	key := digestText(request)
	intentPath := inv.layout.InstallationRoot.Path("artifacts", "design-person", key+".json")
	guard, err := diskstore.BoundExclusive(intentPath + ".lock")
	if err != nil {
		return fail(err)
	}
	defer guard.Release()
	var exit goal.DesignExit
	data, err := os.ReadFile(intentPath)
	if err == nil {
		err = json.Unmarshal(data, &exit)
	} else if os.IsNotExist(err) {
		exit.Operation, err = goal.NewOperationULID()
		if err == nil {
			exit.At = inv.unitStopNow().UTC().Format(time.RFC3339)
			exit.DesignID, exit.Expected, exit.Who, exit.Ruling, exit.Reason = record.ID, string(current), unitStopActor(actor), ruling, inv.input.text("reason")
			chains := inv.delivery().designChains(inv.layout.InstallationRoot.Path(), record.Goals[0], path)
			if len(chains) == 1 {
				exit.Root, exit.Round = chains[0].Root, chains[0].NewestRound
			}
			exit.Page = ruledDesignPage(path, page, exit, intentPath)
			exit.BodySHA256, err = project.DesignBodyDigest(path, []byte(exit.Page))
			unitPath := path
			if inv.input.has("scope") {
				unitPath = inv.flagPath("scope")
			}
			units, unitErr := launch.DeclaredUnits(unitPath)
			if unitErr != nil {
				return fail(fmt.Errorf("the design's units cannot be ruled: %w", unitErr))
			}
			if len(units) == 0 {
				return fail(fmt.Errorf("the design has no declared units to rule"))
			}
			for _, unit := range units {
				exit.Units = append(exit.Units, unit.Name)
			}
			// The record publisher validates identity before any page is replaced.
			exit.Impact = fmt.Sprintf("Impact: %s authorizes %s for design %s. Advisory evidence may be unknown; no clean examination or implementation proof is claimed. Subsequent work uses this ruled page; spent examinations and existing obligations remain. Undo with %s using the Expected page retained in %s. Any separately opened destination needs its own goal act; built code needs its own correction.", exit.Who, ruling, record.ID, shellCommand([]string{"metasystem", "design", "review", path, "--scope", intentPath + ".prior.md", "--reason", "TEXT", "--by", exit.Who}), intentPath)
			if err == nil {
				data, err = json.Marshal(exit)
			}
			if err == nil {
				_, err = atomicfile.WriteFile(intentPath, data, 0600, "")
			}
		}
	}
	if err != nil {
		return fail(err)
	}
	if _, err := atomicfile.WriteFile(intentPath+".prior.md", []byte(exit.Expected), 0600, ""); err != nil {
		return fail(err)
	}
	if err := inv.recordUnitStopOverride(record.Goals[0], "design-ruling", exit.Reason, exit.Impact, exit.Who, intentPath); err != nil {
		return fail(err)
	}
	req, err := inv.requestBuilder(proof, false)("design-exit", inv.stateRoot, exit.Who, "")
	if err != nil {
		return fail(err)
	}
	if _, err = goal.Recover(req.Endpoint); err != nil {
		return fail(err)
	}
	result, err := goal.PublishDesignExit(req, record.Goals[0], exit, func() error {
		actual, err := os.ReadFile(path)
		if err == nil && string(actual) != exit.Expected && string(actual) != exit.Page {
			err = project.ErrDesignChanged
		}
		return err
	})
	if err != nil || result.Outcome != goal.OutcomeConfirmed && !result.Unchanged {
		return fail(fmt.Errorf("goal publication: %s %s: %v", result.Outcome, result.Detail, err))
	}
	_, err = project.PublishDesign(project.DesignPublication{Destination: path, Root: inv.layout.GitRoot, StateRoot: inv.stateRoot, ExpectedPresent: true, Expected: []byte(exit.Expected), Draft: []byte(exit.Page), RecordID: record.ID, Goal: record.Goals[0], Commitment: func() error {
		projection, err := goal.Project(req.Endpoint, false, req.Now)
		if err != nil {
			return err
		}
		file, _ := goalRecord(projection, record.Goals[0])
		if file != nil {
			for i := len(file.DesignExits) - 1; i >= 0; i-- {
				committed := file.DesignExits[i]
				if committed.Operation == exit.Operation && committed.State == "committed" && committed.Page == exit.Page {
					return nil
				}
				if committed.DesignID == exit.DesignID {
					break
				}
			}
		}
		return fmt.Errorf("the ruled exit is not committed")
	}})
	if err != nil {
		return fail(err)
	}
	answer := intentResult{Outcome: intentConfirmed, Summary: "the person's design ruling is published; advisory findings and implementation obligations retain their evidence", Data: exit}
	if exit.Root != "" {
		if err := dispatchcore.CloseRuledDesign(inv.layout.InstallationRoot.Path(), exit.Root, exit.Operation); err != nil {
			answer.Details = append(answer.Details, "ruling committed; local closure needs repair: "+err.Error())
		}
		if err := channel.RecordUnitStopAct(inv.layout.InstallationRoot.Path(), channel.UnitStopAct{ID: exit.Operation, Goal: record.Goals[0], Loop: "design-round", Subject: exit.Root, Attempt: int(exit.Round), Kind: "design-ruling", Reason: exit.Reason, At: inv.unitStopNow(), UnitClosed: true}); err != nil {
			answer.Details = append(answer.Details, "ruling committed; notification needs repair: "+err.Error())
		}
	}
	return answer
}

func ruledDesignPage(path string, page []byte, exit goal.DesignExit, intentPath string) string {
	record, _, _ := project.ParseRecord(path, string(page))
	lines := strings.SplitAfter(string(page), "\n")
	var head []string
	for _, field := range record.Head {
		if field.Key != "Status" && field.Key != "Critique" {
			head = append(head, lines[field.Line-1])
		}
	}
	at, _ := time.Parse(time.RFC3339, exit.At)
	head = append(head, "- Status: accepted\n", fmt.Sprintf("- Critique: ruled by %s %s, %s; reason: %s; impact: %s (ruling %s)\n", exit.Who, at.Local().Format("2006-01-02 15:04 MST"), strings.Join(strings.Fields(exit.Ruling), " "), strings.Join(strings.Fields(exit.Reason), " "), intentPath, exit.Operation))
	return strings.Join(lines[:record.HeadLine-1], "") + strings.Join(head, "") + strings.Join(lines[record.Head[len(record.Head)-1].Line:], "")
}
