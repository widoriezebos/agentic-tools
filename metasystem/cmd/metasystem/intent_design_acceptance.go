package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
				if exit.Root != closedRoot || exit.State != "committed" || exit.Page != string(page) || file.CheckDesignAcceptance(exit.Operation, plan.recordID, exit.BodySHA256) != nil {
					continue
				}
				if entry.Exit != nil && entry.Exit.Operation == exit.Operation {
					entry.Exit = nil
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
	if !inv.input.has("dispositions") {
		return fail(fmt.Errorf("resume with --dispositions FILE for critique %s", chain.Root))
	}
	decisions, err := os.ReadFile(inv.flagPath("dispositions"))
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
		if read.Material != 0 {
			if plan.subject == read.Subject.ContentDigest {
				return nil
			}
			if err := dispatchcore.CritiqueRegisterApplyDecisions(inv.layout.InstallationRoot.Path(), chain.Root, inv.registerDecisions(chain.Root, chain.NewestRound)); err != nil {
				return fail(err)
			}
		}
		root, err := inv.jobRecord(chain.Root)
		if err != nil {
			return fail(err)
		}
		if clean, err := readsubject.CleanRegister(root["findingRegister"]); err != nil || !clean {
			return fail(fmt.Errorf("the critique has unresolved findings: %v", err))
		}
		if read.Material == 0 && plan.subject != read.Subject.ContentDigest {
			return fail(project.ErrDesignChanged)
		}
		page := []byte(read.Subject.DesignPage)
		if read.Material != 0 {
			page, err = os.ReadFile(plan.design)
			if err != nil || digestText(page) != plan.subject {
				return fail(project.ErrDesignChanged)
			}
		}
		units, err := launch.DeclaredUnits(plan.design)
		if err != nil || len(units) == 0 {
			return fail(fmt.Errorf("the accepted design needs declared source units: %v", err))
		}
		projection, _, problem := inv.projection()
		if problem != nil {
			return problem
		}
		file, _ := goalRecord(projection, plan.goalID)
		if file == nil {
			return fail(fmt.Errorf("goal %s is absent", plan.goalID))
		}
		operation, err := goal.NewOperationULID()
		if err != nil {
			return fail(err)
		}
		exit := goal.DesignExit{Operation: operation, DesignID: plan.recordID, Root: chain.Root, Round: chain.NewestRound, Revision: file.Revision,
			ExaminedSHA256: read.Subject.ContentDigest, DispositionsSHA256: digestText(decisions), Dispositions: string(decisions), Expected: string(page), State: "prepared"}
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
			head[len(head)-1] = fmt.Sprintf("- Critique: closed at round %d on %d material findings folded into the written Decisions (convergence %s)\n", exit.Round, read.Material, operation)
		}
		exit.Page = strings.Join(lines[:record.HeadLine-1], "") + strings.Join(head, "") + strings.Join(lines[record.Head[len(record.Head)-1].Line:], "")
		exit.BodySHA256, err = project.DesignBodyDigest(plan.design, []byte(exit.Page))
		if err == nil {
			entry.Exit = &exit
			err = inv.writeDesignReviewEntry(plan.recordID, entry)
		}
		if err != nil {
			return fail(err)
		}
	}
	exit := *entry.Exit
	guard.Release()
	if exit.Root != chain.Root || exit.DispositionsSHA256 != digestText(decisions) {
		return fail(fmt.Errorf("the prepared acceptance belongs to another critique or decisions file"))
	}
	req, err := inv.requestBuilder(nil, false)("defer-findings", inv.stateRoot, "", inv.input.text("lineage"))
	if err != nil {
		return fail(err)
	}
	if _, err := goal.Recover(req.Endpoint); err != nil {
		return fail(err)
	}
	committed := func() (goal.DesignExit, error) {
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
					return recorded, file.CheckDesignAcceptance(exit.Operation, plan.recordID, exit.BodySHA256)
				}
			}
		}
		return goal.DesignExit{}, os.ErrNotExist
	}
	if _, err = committed(); os.IsNotExist(err) {
		result, publishErr := goal.PublishDesignExit(req, plan.goalID, exit, func() error {
			policy, err := inv.unitRunner().ReviewPolicy()
			if err != nil {
				return err
			}
			if policy == "person" {
				return fmt.Errorf("acceptance is prepared for the review policy's holder; release that hold to resume")
			}
			page, err := os.ReadFile(plan.design)
			if err == nil && string(page) != exit.Expected {
				err = project.ErrDesignChanged
			}
			return err
		})
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
	closed := inv.closeChain(chain.Root)
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
	if entry.Exit != nil && entry.Exit.Operation == exit.Operation {
		entry.Exit = nil
		if err := inv.writeDesignReviewEntry(plan.recordID, entry); err != nil {
			return fail(err)
		}
	}
	closed.Summary = "design " + plan.recordID + " is accepted; its committed page is projected and critique " + chain.Root + " is closed"
	closed.Data = map[string]any{"exit": exit.Operation, "bodySha256": exit.BodySHA256, "closedAt": exit.Round}
	return &closed
}
