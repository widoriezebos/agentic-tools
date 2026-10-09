package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func (inv *intentInvocation) advanceBoundary() error {
	defer inv.leaveStores()
	if problem := inv.selectRoot(); problem != nil {
		return errors.New(problem.Summary)
	}
	policy, _, code, err := inv.work().config("seat.driver", intentConfPath(inv.layout))
	if err != nil || code != 0 || (policy != "auto" && policy != "person") {
		return fmt.Errorf("the seat driver policy is unavailable (seat.driver=%q, exit %d): %v", policy, code, err)
	}
	return steward.AdvanceBoundary(inv.stateRoot, policy == "person", identity.KernelProber{}, inv.prepareBoundary)
}

func (inv *intentInvocation) prepareBoundary(event steward.UnitBoundary) (steward.BoundaryAct, error) {
	projection, _, problem := inv.projection()
	if problem != nil {
		return steward.BoundaryAct{}, errors.New(problem.Summary)
	}
	file, where := goalRecord(projection, event.Goal)
	if file == nil || where != "live" {
		return steward.BoundaryAct{}, errors.New("the boundary's goal is no longer open; no next work was prepared")
	}
	state, err := inv.delivery().branchState(inv.layout.InstallationRoot.Path(), event.Goal)
	if err != nil {
		return steward.BoundaryAct{}, err
	}
	designs, problem := inv.acceptedDesignPaths(event.Goal)
	if problem != nil {
		return steward.BoundaryAct{}, errors.New(problem.Summary)
	}
	progress, err := goalProgress(designs, state)
	if err != nil {
		return steward.BoundaryAct{}, err
	}
	act := steward.BoundaryAct{Tip: state.BranchTip, Unit: progress.Unit}
	if act.Tip == "" {
		return act, errors.New("the published goal tip is unavailable; no next work was prepared")
	}
	if progress.Unit == "" {
		if progress.NoEnd {
			act.Summary = "the goal has no declared end; a person must declare its remaining work"
			return act, nil
		}
		act.Command, act.Summary = inv.publicArgv("work", "land", event.Goal), "all required work is reviewed; the goal is ready for hand-in"
		return act, nil
	}
	if !progress.NeedsBuild {
		act.Command, act.Summary = inv.publicArgv("work", "review", event.Goal, "--work", progress.Unit), "the required unit still needs its read"
		return act, nil
	}
	// A changed tip or design selects a new input directory. Filled worker
	// content is preserved, and is never silently reused for a different subject.
	files := []unitRequestFile{}
	for _, path := range designs {
		file, err := fileIdentity(path)
		if err != nil {
			return act, err
		}
		files = append(files, file)
	}
	claim := sha256.Sum256(goal.RenderFile(file))
	directory, err := inv.unitRunner().NamedInputDirectory(inv.stateRoot, event.Goal, fmt.Sprintf("%s/%s/%v/%x", progress.Unit, act.Tip, files, claim))
	if err != nil {
		return act, err
	}
	brief := filepath.Join(directory, "next-brief.md")
	act.Command, act.Summary = inv.publicArgv("work", "build", event.Goal, "--work", progress.Unit, "--brief", brief), "the worker must fill the required unit's brief and submit this build"
	data, err := os.ReadFile(brief)
	if os.IsNotExist(err) {
		preparation := *inv
		preparation.command, _ = findIntentCommand("work brief")
		preparation.input = intentInput{args: []string{event.Goal}, values: map[string][]string{"out": {brief}}}
		preparation.raw, preparation.stdout, preparation.stderr = []string{event.Goal, "--out", brief}, io.Discard, io.Discard
		defer preparation.leaveStores()
		if code := runIntentBrief(&preparation); code != 0 {
			return act, fmt.Errorf("the next unit's brief could not be prepared (exit %d)", code)
		}
		data, err = os.ReadFile(brief)
	}
	if err != nil {
		return act, err
	}
	if missing := missingDecisionLines(data); len(missing) > 0 {
		act.Summary += ": " + strings.Join(missing, "; ")
	}
	work, problem := inv.goalWork(event.Goal)
	if problem != nil {
		return act, errors.New(problem.Summary)
	}
	for _, one := range work {
		if one.Unit == progress.Unit && one.Record != nil {
			if one.Record.State == "cancelled" {
				act.Command, act.Summary = nil, "the required unit was cancelled; a person must choose its next work"
				return act, nil
			}
			act.Effect, act.Command, act.Summary = one.Run, nil, "the worker's submitted build is retained; continue that run"
		}
	}
	return act, nil
}
