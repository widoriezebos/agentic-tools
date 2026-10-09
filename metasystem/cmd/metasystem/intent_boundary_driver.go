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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
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
	if err := steward.RetainBoundary(inv.layout.InstallationRoot.Path(), supervise.BuildStamp, nil); err != nil {
		return err
	}
	err = steward.AdvanceBoundary(inv.layout.InstallationRoot.Path(), policy == "person", identity.KernelProber{}, inv.prepareBoundary)
	events, readErr := steward.ReadUnitBoundaries(inv.layout.InstallationRoot.Path())
	if readErr != nil {
		return errors.Join(err, readErr)
	}
	seen := map[string]bool{}
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Seat != inv.stateRoot || seen[event.Goal] {
			continue
		}
		seen[event.Goal] = true
		act := event.Next
		if act == nil {
			continue
		}
		if act.Unit == "" && len(act.Command) > 2 && act.Command[2] == "land" {
			err = errors.Join(err, inv.boundaryHandIn(&event))
		} else if policy == "auto" && act.Effect != "" {
			_, nextErr := inv.unitRunner().Continue(launch.UnitRequest{Resume: act.Effect, NonBlocking: true})
			if !errors.Is(nextErr, launch.ErrUnitObserving) {
				err = errors.Join(err, nextErr)
			}
		}
	}
	return err
}

func (inv *intentInvocation) prepareBoundary(event steward.UnitBoundary) (steward.BoundaryAct, error) {
	projection, _, problem := inv.projection()
	if problem != nil {
		return steward.BoundaryAct{}, errors.New(problem.Summary)
	}
	file, where := goalRecord(projection, event.Goal)
	if file == nil || where != "live" {
		return steward.BoundaryAct{}, steward.ErrBoundaryClosed
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
	directory, err := inv.unitRunner().NamedInputDirectory(inv.layout.InstallationRoot.Path(), event.Goal, fmt.Sprintf("%s/%s/%v/%x/%s/%s/%s", progress.Unit, act.Tip, files, claim, event.Session, event.Unit, event.Outcome))
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
			submitted := ""
			if operation := one.Record.Operation; operation != nil {
				submitted, _, _ = takeIntentFlag(operation.Argv, "brief", true)
				submitted = (&intentInvocation{cwd: operation.CallerDirectory}).inputPath(submitted)
			}
			if one.Record.State == "cancelled" || one.Record.Base != act.Tip || submitted != brief {
				act.Command, act.Summary = nil, "the required unit was cancelled or its preparation changed; a person must choose its next work"
				return act, nil
			}
			act.Effect, act.Summary = one.Run, "the worker's submitted build is retained; continue that run"
		}
	}
	return act, nil
}

func (inv *intentInvocation) boundaryBuildAdmission(id, unit, base, brief string) error {
	if inv.checkDirectPersonProof("work build", false) == nil {
		return nil
	}
	if problem := inv.selectRoot(); problem != nil {
		return errors.New(problem.Summary)
	}
	policy, _, code, err := inv.work().config("seat.driver", intentConfPath(inv.layout))
	if err != nil || code != 0 || (policy != "auto" && policy != "person") {
		return fmt.Errorf("the seat driver policy is unavailable (seat.driver=%q, exit %d): %v", policy, code, err)
	}
	event, err := steward.BoundaryAdmission(inv.layout.InstallationRoot.Path(), id, supervise.BuildStamp, policy == "person", identity.KernelProber{})
	if err != nil || event.Goal == "" {
		return err
	}
	if policy == "auto" {
		if err := inv.automaticReviewAdmission(); err != nil {
			return err
		}
	}
	act, err := inv.prepareBoundary(event)
	if err != nil {
		return err
	}
	if act.Unit != unit || act.Tip != base || actorValue(act.Command, "--brief") != brief {
		return fmt.Errorf("boundary preparation changed: %s; inspect metasystem work status %s", act.Summary, id)
	}
	return nil
}

// boundaryHandIn observes the prepared public command's effect on the lane.
func (inv *intentInvocation) boundaryHandIn(event *steward.UnitBoundary) error {
	root, configured, problem := inv.laneCheck(inv.targets(event.Goal))
	if problem != nil {
		return errors.New(problem.Summary)
	}
	if !configured {
		return nil
	}
	install, err := inv.laneInstallOf(root)
	if err != nil {
		return err
	}
	entry, found, err := inv.latestLaneGoalEntry(install, event.Goal, "")
	if err != nil || !found || entry.SHA != event.Next.Tip {
		return err
	}
	event.Next.Effect, event.Next.Summary = entry.SHA, "the whole goal is handed to the landing lane; the goal remains open"
	return steward.RetainBoundary(inv.layout.InstallationRoot.Path(), "", event)
}
