package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

var errDesignRevisionPending = errors.New("the prepared design revision is waiting")

// finalDesignRevision retains the request before starting the author. The visible
// page remains unchanged until the goal commits the validated proposal.
func (inv *intentInvocation) finalDesignRevision(plan designReviewPlan, chain dispatchcore.DesignCritiqueChain, read readsubject.Read, page []byte) ([]byte, *goal.DesignExit, error) {
	var decisions []byte
	if inv.input.has("dispositions") {
		var err error
		decisions, err = os.ReadFile(inv.flagPath("dispositions"))
		if err != nil {
			return nil, nil, err
		}
	}
	guard, err := diskstore.BoundExclusive(inv.designReviewEntryPath(plan.recordID) + ".exit-lock")
	if err != nil {
		return nil, nil, err
	}
	entry, err := inv.readDesignReviewEntry(plan.recordID)
	if err == nil && entry.Fold == nil {
		operation, problem := goal.NewOperationULID()
		err = problem
		if err == nil {
			entry.Fold = &goal.DesignExit{Operation: operation, Root: chain.Root, Round: chain.NewestRound, Expected: string(page), AuthorPrior: read.Subject.DesignPage}
			if inv.input.has("dispositions") {
				entry.Fold.Dispositions, entry.Fold.DispositionsSHA256 = string(decisions), digestText(decisions)
			}
			if err == nil {
				err = inv.writeDesignReviewEntry(plan.recordID, entry)
			}
		}
	}
	if err == nil && entry.Fold != nil && entry.Fold.Revision == 0 && entry.Fold.AuthorAttempt == 0 && entry.Fold.Page == "" && entry.Fold.Root == chain.Root && entry.Fold.Round == chain.NewestRound && entry.Fold.DispositionsSHA256 == digestText(decisions) && entry.Fold.Expected != string(page) {
		entry.Fold.Expected = string(page)
		err = inv.writeDesignReviewEntry(plan.recordID, entry)
	}
	guard.Release()
	if err != nil {
		return nil, nil, err
	}
	fold := entry.Fold
	if fold.Root != chain.Root || fold.Round != chain.NewestRound || fold.Expected != string(page) {
		return nil, nil, project.ErrDesignChanged
	}
	if inv.input.has("dispositions") {
		if fold.DispositionsSHA256 != digestText(decisions) {
			return nil, nil, fmt.Errorf("the final revision belongs to another decisions file")
		}
	}
	if fold.AuthorAttempt == 0 && string(page) != read.Subject.DesignPage {
		return page, fold, nil
	}
	if read.Material == 0 {
		return page, fold, nil
	}
	if plan.goalID == "" {
		return nil, nil, fmt.Errorf("%w; goal-free scope retains a draft proposal without launching a goal author", errDesignRevisionPending)
	}
	if held := inv.designEffectPolicy(plan, chain, "revision"); held != nil {
		return nil, nil, fmt.Errorf("%w: %s", errDesignRevisionPending, held.Summary)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return nil, nil, fmt.Errorf("%s", problem.Summary)
	}
	file, _ := goalRecord(projection, plan.goalID)
	req, err := inv.requestBuilder(nil, false)("defer-findings", inv.stateRoot, "", inv.input.text("lineage"))
	if err != nil {
		return nil, nil, err
	}
	if file == nil || file.Approved == nil || file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine != req.Actor.Machine || file.Claimed.Lineage != req.Actor.Lineage || file.StopFence != nil || file.Budget == nil || file.Budget.ReviewRoundLimit <= 0 {
		return nil, nil, fmt.Errorf("the final author needs the source's existing approved author allowance")
	}
	if fold.Revision != 0 && fold.Revision != file.Revision {
		return nil, nil, fmt.Errorf("the source revision changed while its final author was pending")
	}
	fold.Revision = file.Revision
	guard, err = diskstore.BoundExclusive(inv.designReviewEntryPath(plan.recordID) + ".exit-lock")
	if err != nil {
		return nil, nil, err
	}
	entry, err = inv.readDesignReviewEntry(plan.recordID)
	if err == nil && entry.Fold != nil && entry.Fold.Operation == fold.Operation {
		entry.Fold.Revision = fold.Revision
		err = inv.writeDesignReviewEntry(plan.recordID, entry)
	} else if err == nil {
		err = project.ErrDesignChanged
	}
	guard.Release()
	if err != nil {
		return nil, nil, err
	}
	root, err := inv.jobRecord(chain.Root)
	if err != nil {
		return nil, nil, err
	}
	open, err := dispatchcore.CritiqueOpenFindingIDs(inv.layout.InstallationRoot.Path(), chain.Root)
	if err != nil {
		return nil, nil, err
	}
	historyData, err := json.Marshal(root["designExaminations"])
	if err != nil {
		return nil, nil, err
	}
	var history []readsubject.Read
	if err := json.Unmarshal(historyData, &history); err != nil {
		return nil, nil, err
	}
	// The attempt's frozen prior supplies the page once; history supplies findings
	// and section identities without charging the author for old page copies.
	read.Subject.DesignPage = ""
	for i := range history {
		history[i].Subject.DesignPage = ""
	}
	evidence, err := json.Marshal([]any{read, history, open})
	if err != nil {
		return nil, nil, err
	}
	brief := fmt.Sprintf("Exit operation: %s\nFold every unresolved concrete requirement into its owning Decision, removing conflicting requirements. Add its public tests and code-read checklist to its unit row. Change only those Decisions, Units and Acceptance items. Never accept the page or run a critic.\nUnder ## Acceptance items write one Fold item: JSON row per finding with Finding, Unit, Decision, Passage, Checklist, Fixture (group:ID), Tests (package/TestName). For an unresolved choice use Unresolved:true, its Unit and Decision; preserve the choice. Also write Scope item: JSON rows for EVERY unit, with Unit, Decisions (exact headings), DependsOn (unit names, including an empty list for independent units). Preserve every existing dependency; never cut one to create a remainder.\nFrozen examination: %s\nBound dispositions:\n%s", fold.Operation, evidence, fold.Dispositions)
	manager := inv.designManager()
	result, err := manager.RequestDesign(launch.DesignRequest{Operation: fold.Operation, Goal: plan.goalID, RecordID: plan.recordID, Destination: plan.design, WorkingDirectory: inv.layout.GitRoot, Brief: []byte(brief), Contract: []byte("Write one complete draft proposal to the declared output. Never edit the project document.\n")})
	if err != nil {
		return nil, nil, fmt.Errorf("final author request: %w (%s)", err, launch.ErrorDetail(err))
	}
	fold.AuthorAttempt = result.Attempt.Attempt
	guard, err = diskstore.BoundExclusive(inv.designReviewEntryPath(plan.recordID) + ".exit-lock")
	if err != nil {
		return nil, nil, err
	}
	defer guard.Release()
	entry, err = inv.readDesignReviewEntry(plan.recordID)
	if err != nil {
		return nil, nil, err
	}
	if entry.Fold == nil || entry.Fold.Operation != fold.Operation || entry.Fold.Expected != fold.Expected {
		return nil, nil, project.ErrDesignChanged
	}
	entry.Fold.AuthorAttempt = fold.AuthorAttempt
	if err := inv.writeDesignReviewEntry(plan.recordID, entry); err != nil {
		return nil, nil, err
	}
	if result.Record.State != launch.Completed {
		return nil, nil, fmt.Errorf("%w: attempt %d is %s; collect the same attempt or repair its proposal at %s", errDesignRevisionPending, fold.AuthorAttempt, result.Record.State, result.Attempt.Draft)
	}
	proposal, err := os.ReadFile(result.Attempt.Draft)
	if err != nil {
		return nil, nil, err
	}
	if result.Attempt.ExpectedSHA256 != digestText(page) {
		return nil, nil, project.ErrDesignChanged
	}
	return proposal, fold, nil
}
