package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func (inv *intentInvocation) finishDesignAcceptance(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, prepare bool) *intentResult {
	fail := func(err error) *intentResult {
		return &intentResult{Targets: plan.targets, Outcome: intentFailed, code: 1, Summary: "design acceptance remains pending: " + err.Error(), next: inv.sameCommand(), nextReason: "resume this publication after repairing the reported cause"}
	}
	guard, err := diskstore.BoundExclusive(inv.designReviewEntryPath(plan.recordID) + ".exit-lock")
	if err != nil {
		return fail(err)
	}
	defer guard.Release()
	var entry designReviewEntry
	data, err := os.ReadFile(inv.designReviewEntryPath(plan.recordID))
	if err == nil {
		err = json.Unmarshal(data, &entry)
	}
	if err != nil && !os.IsNotExist(err) {
		return fail(err)
	}
	closedRoot, isClosed := chain.Root, chain.Closed
	if entry.Exit != nil && entry.Exit.Root != chain.Root {
		root, err := inv.jobRecord(entry.Exit.Root)
		if err != nil {
			return fail(err)
		}
		closedRoot = entry.Exit.Root
		isClosed, _ = root["chainClosed"].(bool)
	}
	if isClosed && inv.input.has("retry") {
		return nil
	}
	if isClosed {
		projection, _, problem := inv.projection()
		if problem != nil {
			return problem
		}
		file, _ := goalRecord(projection, plan.goalID)
		if file != nil {
			page, err := os.ReadFile(plan.design)
			if err != nil {
				return fail(err)
			}
			for i := len(file.DesignExits) - 1; i >= 0; i-- {
				exit := file.DesignExits[i]
				if exit.Root != closedRoot || exit.State != "committed" || exit.Page != string(page) || exit.Destination == "" && file.CheckDesignAcceptance(exit.Operation, plan.recordID, exit.BodySHA256) != nil {
					continue
				}
				if err := inv.recordDesignEffect(plan, dispatchcore.DesignCritiqueChain{Root: exit.Root, NewestRound: exit.Round}, "acceptance", exit.Operation); err != nil {
					return fail(fmt.Errorf("acceptance committed; its held ask needs repair: %w", err))
				}
				if exit.Destination != "" {
					guard.Release()
					if err := inv.finishDesignSplit(plan, exit); err != nil {
						return fail(err)
					}
					return &intentResult{Targets: plan.targets, Outcome: intentUnchanged, Summary: "design " + plan.recordID + " is transferred to " + exit.Destination + "; its follow-up is open"}
				}
				if entry.Exit != nil && entry.Exit.Operation == exit.Operation {
					entry.Exit, entry.Fold = nil, nil
					if err := inv.writeDesignReviewEntry(plan.recordID, entry); err != nil {
						return fail(err)
					}
				}
				if closedRoot == chain.Root {
					return &intentResult{Targets: append(plan.targets, jobTarget(chain.Root)), Outcome: intentUnchanged,
						Summary: "design " + plan.recordID + " is accepted; its committed page is projected and critique " + chain.Root + " is closed",
						Data:    map[string]any{"exit": exit.Operation, "bodySha256": exit.BodySHA256, "closedAt": exit.Round}}
				}
				break
			}
		}
	}
	if entry.Exit == nil && !prepare {
		return nil
	}
	var decisions []byte
	if inv.input.has("dispositions") {
		decisions, err = os.ReadFile(inv.flagPath("dispositions"))
	} else if entry.Exit != nil {
		decisions = []byte(entry.Exit.Dispositions)
	} else {
		returnPath := inv.returnPathAt(inv.layout.InstallationRoot.Path(), chain.Root, chain.NewestRound)
		digest, _, readErr := reviewReturnDigest(returnPath)
		if readErr != nil {
			return fail(readErr)
		}
		findings, _, readErr := readIntentFindings(returnPath)
		if readErr != nil {
			return fail(readErr)
		}
		decisions = []byte(strings.ReplaceAll(decisionsDocument(reviewBinding{Goal: plan.goalID, Work: "design:" + plan.recordID, Attempt: int(chain.NewestRound), Subject: plan.subject, Examination: chain.Root, Round: chain.NewestRound, Return: digest}, findings), "| DECIDE | | |", "| noted | non-material finding retained | |"))
	}
	if err != nil {
		return fail(err)
	}
	if entry.Exit == nil {
		required, err := dispatchcore.DesignEvidenceRequired(inv.layout.InstallationRoot.Path(), chain.Root, chain.NewestRound)
		if err != nil {
			return fail(err)
		}
		if !required {
			return nil
		}
		read, readErr := inv.delivery().examinationRead(inv.layout.InstallationRoot.Path(), chain.NewestJob)
		if readErr != nil {
			return fail(readErr)
		}
		if read.Material != 0 && entry.Fold == nil {
			return fail(fmt.Errorf("supply the bound final Decision revision and acceptance mappings"))
		}
		if read.Material == 0 {
			root, err := inv.jobRecord(chain.Root)
			if err != nil {
				return fail(err)
			}
			if clean, err := readsubject.CleanDesignRegister(root); err != nil || !clean {
				return fail(fmt.Errorf("the critique has unresolved findings: %v", err))
			}
		}
		if read.Material == 0 && plan.subject != read.Subject.ContentDigest {
			return fail(project.ErrDesignChanged)
		}
		page := []byte(read.Subject.DesignPage)
		if read.Material != 0 {
			page = []byte(entry.Fold.Page)
			if digestText([]byte(entry.Fold.Expected)) != plan.subject {
				return fail(project.ErrDesignChanged)
			}
		}
		units, err := launch.CheckDesignSize(string(page))
		split := entry.Fold != nil && entry.Fold.Destination != ""
		if !split && (err != nil || len(units) == 0) {
			return fail(fmt.Errorf("the accepted design needs declared source units: %v", err))
		}
		projection, _, problem := inv.projection()
		if problem != nil {
			return problem
		}
		file, _ := goalRecord(projection, plan.goalID)
		if file == nil && plan.goalID != "" {
			return fail(fmt.Errorf("goal %s is absent", plan.goalID))
		}
		if file == nil {
			file = &goal.GoalFile{}
		}
		operation, err := goal.NewOperationULID()
		if err != nil {
			return fail(err)
		}
		exit := goal.DesignExit{Operation: operation, DesignID: plan.recordID, Root: chain.Root, Round: chain.NewestRound, Revision: file.Revision,
			ExaminedSHA256: read.Subject.ContentDigest, DispositionsSHA256: digestText(decisions), Dispositions: string(decisions), Expected: string(page), State: "prepared"}
		if read.Material != 0 {
			exit.Obligations, exit.Expected = entry.Fold.Obligations, entry.Fold.Expected
			for _, item := range exit.Obligations {
				exit.Items = append(exit.Items, item.Finding)
			}
		}
		if split {
			exit.AuthorPrior = read.Subject.DesignPage
			exit.Destination, exit.DestinationBrief, exit.OpenCommand = entry.Fold.Destination, entry.Fold.DestinationBrief, entry.Fold.OpenCommand
			exit.DestinationBriefPath = entry.Fold.DestinationBriefPath
			exit.TransferUnits, exit.TransferObligations, exit.Expected = entry.Fold.TransferUnits, entry.Fold.TransferObligations, entry.Fold.Expected
			units = nil
		}
		for _, unit := range units {
			exit.Units = append(exit.Units, unit.Name)
		}
		record, _, _ := project.ParseRecord(plan.design, string(page))
		lines := strings.SplitAfter(string(page), "\n")
		var head []string
		for _, field := range record.Head {
			if field.Key != "Status" && field.Key != "Critique" {
				head = append(head, lines[field.Line-1])
			}
		}
		head = append(head, "- Status: accepted\n", fmt.Sprintf("- Critique: closed at round %d on 0 material findings folded as 0 unit acceptance items (convergence %s)\n", exit.Round, operation))
		if read.Material != 0 {
			head[len(head)-1] = fmt.Sprintf("- Critique: closed at round %d on %d material findings folded as %d unit acceptance items (convergence %s)\n", exit.Round, read.Material, len(exit.Items), operation)
		}
		if split {
			head[len(head)-2], head[len(head)-1] = "- Status: draft\n", fmt.Sprintf("- Critique: split to %s at round %d (convergence %s)\n", exit.Destination, exit.Round, operation)
		}
		exit.Page = strings.Join(lines[:record.HeadLine-1], "") + strings.Join(head, "") + strings.Join(lines[record.Head[len(record.Head)-1].Line:], "")
		exit.BodySHA256, err = project.DesignBodyDigest(plan.design, []byte(exit.Page))
		if err == nil {
			for _, item := range exit.Obligations {
				item.DesignItem.BodySHA256, item.DesignItem.Exit = exit.BodySHA256, exit.Operation
			}
			entry.Exit = &exit
			err = inv.writeDesignReviewEntry(plan.recordID, entry)
		}
		if err != nil {
			return fail(err)
		}
	}
	if plan.goalID == "" && entry.Exit.State != "committed" {
		if _, err := inv.delivery().examinationRead(inv.layout.InstallationRoot.Path(), chain.NewestJob); err != nil {
			return fail(err)
		}
		if held := inv.designEffectPolicy(plan, chain, "acceptance"); held != nil {
			return held
		}
		page, err := os.ReadFile(plan.design)
		if err != nil {
			return fail(err)
		}
		if string(page) != entry.Exit.Expected {
			return fail(project.ErrDesignChanged)
		}
		entry.Exit.State = "committed"
		if err := inv.writeDesignReviewEntry(plan.recordID, entry); err != nil {
			return fail(err)
		}
	}
	exit := *entry.Exit
	guard.Release()
	closer := *inv
	if !inv.input.has("dispositions") {
		path := inv.designReviewEntryPath(plan.recordID) + ".exit-decisions.md"
		if _, err := atomicfile.WriteText(path, exit.Dispositions, inv.layout.InstallationRoot.Path()); err != nil {
			return fail(err)
		}
		closer.input.values = maps.Clone(inv.input.values)
		closer.input.values["dispositions"] = []string{path}
	}
	if exit.Destination != "" {
		attempt, err := inv.designManager().RecordSuppliedDesign(launch.DesignRequest{Goal: plan.goalID, RecordID: plan.recordID, Destination: plan.design}, exit.Operation, []byte(exit.AuthorPrior), []byte(exit.Expected))
		if err != nil {
			return fail(err)
		}
		exit.AuthorAttempt = attempt.Attempt
	}
	if exit.Root != chain.Root || exit.DispositionsSHA256 != digestText(decisions) {
		return fail(fmt.Errorf("the prepared acceptance belongs to another critique or decisions file"))
	}
	var req goal.VerbRequest
	if plan.goalID != "" {
		req, err = inv.requestBuilder(nil, false)("defer-findings", inv.stateRoot, "", inv.input.text("lineage"))
		if err != nil {
			return fail(err)
		}
		if _, err := goal.Recover(req.Endpoint); err != nil {
			return fail(err)
		}
	}
	committed := func() (goal.DesignExit, error) {
		if plan.goalID == "" {
			actual, err := inv.readDesignReviewEntry(plan.recordID)
			if err != nil {
				return goal.DesignExit{}, err
			}
			if actual.Exit == nil || actual.Exit.Operation != exit.Operation || actual.Exit.Page != exit.Page {
				return goal.DesignExit{}, fmt.Errorf("the retained goal-free exit disagrees with this acceptance")
			}
			if actual.Exit.State != "committed" {
				return goal.DesignExit{}, os.ErrNotExist
			}
			return *actual.Exit, nil
		}
		projection, err := goal.Project(req.Endpoint, false, req.Now)
		if err != nil {
			return goal.DesignExit{}, err
		}
		file, _ := goalRecord(projection, plan.goalID)
		if file != nil {
			for _, recorded := range file.DesignExits {
				if recorded.Operation == exit.Operation {
					if recorded.State != "committed" || recorded.Page != exit.Page || recorded.BodySHA256 != exit.BodySHA256 {
						return recorded, fmt.Errorf("the committed exit disagrees with its prepared page")
					}
					if recorded.Destination != "" && len(recorded.Units) == 0 {
						return recorded, nil
					}
					return recorded, file.CheckDesignAcceptance(exit.Operation, plan.recordID, exit.BodySHA256)
				}
			}
		}
		return goal.DesignExit{}, os.ErrNotExist
	}
	if _, err = committed(); os.IsNotExist(err) {
		if held := inv.designEffectPolicy(plan, chain, "acceptance"); held != nil {
			return held
		}
		admit := func() error {
			if held := inv.designEffectPolicy(plan, chain, "acceptance"); held != nil {
				return fmt.Errorf("%s; %s", held.Summary, held.nextReason)
			}
			if _, err := inv.delivery().examinationRead(inv.layout.InstallationRoot.Path(), chain.NewestJob); err != nil {
				return err
			}
			if len(exit.Items) == 0 && exit.Destination == "" {
				root, err := inv.jobRecord(chain.Root)
				if err != nil {
					return err
				}
				if clean, err := readsubject.CleanDesignRegister(root); err != nil || !clean {
					return fmt.Errorf("the critique has unresolved findings")
				}
			}
			page, err := os.ReadFile(plan.design)
			if err == nil && string(page) != exit.Expected {
				err = project.ErrDesignChanged
			}
			return err
		}
		result, publishErr := goal.PublishDesignExit(req, plan.goalID, exit, admit)
		if publishErr != nil {
			return fail(publishErr)
		}
		if result.Outcome != goal.OutcomeConfirmed && !result.Unchanged {
			return fail(fmt.Errorf("goal publication %s: %s", result.Outcome, result.Detail))
		}
	} else if err != nil {
		return fail(err)
	}
	_, err = project.PublishDesign(project.DesignPublication{Destination: plan.design, Root: inv.layout.GitRoot, StateRoot: inv.stateRoot, ExpectedPresent: true,
		Expected: []byte(exit.Expected), Draft: []byte(exit.Page), RecordID: plan.recordID, Goal: plan.goalID,
		Commitment: func() error { _, err := committed(); return err }})
	if err != nil {
		return fail(err)
	}
	if exit.Destination != "" {
		if err := inv.finishDesignSplit(plan, exit); err != nil {
			return fail(err)
		}
	}
	decided := inv.registerDecisions(chain.Root, exit.Round)
	for _, items := range [][]goal.ReviewObligation{exit.Obligations, exit.TransferObligations} {
		for _, item := range items {
			decided[item.DesignItem.Finding] = "accepted"
		}
	}
	if err := dispatchcore.CritiqueRegisterApplyDecisions(inv.layout.InstallationRoot.Path(), chain.Root, decided); err != nil {
		return fail(err)
	}
	closed := closer.closeChainRecords(chain.Root)
	if closed.Outcome != intentConfirmed && closed.Outcome != intentUnchanged {
		return &closed
	}
	clearGuard, err := diskstore.BoundExclusive(inv.designReviewEntryPath(plan.recordID) + ".exit-lock")
	if err != nil {
		return fail(err)
	}
	defer clearGuard.Release()
	data, err = os.ReadFile(inv.designReviewEntryPath(plan.recordID))
	if err == nil {
		entry = designReviewEntry{}
		err = json.Unmarshal(data, &entry)
	}
	if err != nil {
		return fail(err)
	}
	if plan.goalID != "" && entry.Exit != nil && entry.Exit.Operation == exit.Operation {
		entry.Exit, entry.Fold = nil, nil
		if err := inv.writeDesignReviewEntry(plan.recordID, entry); err != nil {
			return fail(err)
		}
	}
	closed.Summary = "design " + plan.recordID + " is accepted; its committed page is projected and critique " + chain.Root + " is closed"
	if exit.Destination != "" {
		closed.Summary = "design " + plan.recordID + " is transferred to " + exit.Destination + "; its follow-up is open"
	}
	if err := inv.recordDesignEffect(plan, chain, "acceptance", exit.Operation); err != nil {
		closed.Details = append(closed.Details, "acceptance committed; its held ask needs repair: "+err.Error())
	}
	closed.Data = map[string]any{"exit": exit.Operation, "bodySha256": exit.BodySHA256, "closedAt": exit.Round}
	return &closed
}

func (inv *intentInvocation) finishDesignSplit(plan designReviewPlan, exit goal.DesignExit) error {
	path := filepath.Join(inv.stateRoot, exit.DestinationBriefPath)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(path), 0755); err == nil {
			_, err = atomicfile.WriteText(path, exit.DestinationBrief, inv.stateRoot)
		}
	} else if err == nil && string(data) != exit.DestinationBrief {
		err = fmt.Errorf("destination brief %s changed; restore the retained split before replay", path)
	}
	if err != nil {
		return err
	}
	req, err := inv.requestBuilder(nil, false)("open", inv.stateRoot, "", inv.input.text("lineage"))
	if err != nil {
		return err
	}
	opened, err := goal.OpenDesignDestination(req, plan.goalID, exit.Operation)
	if err != nil || opened.Outcome != goal.OutcomeConfirmed && !opened.Unchanged {
		return fmt.Errorf("destination opening remains pending (%s): %s %v; recovery: %s", opened.Outcome, opened.Detail, err, exit.OpenCommand)
	}
	text := fmt.Sprintf("Goal %s was split from %s because the design is too large.\nApprove: metasystem goal approve %s", exit.Destination, plan.goalID, exit.Destination)
	if err := phase.NotifyLanded(context.Background(), inv.stateRoot, text, "design-split:"+exit.Destination, req.Now); err != nil {
		fmt.Fprintln(inv.stderr, "follow-up is open; its approval message is retained for one retry: "+err.Error())
	}
	return nil
}
