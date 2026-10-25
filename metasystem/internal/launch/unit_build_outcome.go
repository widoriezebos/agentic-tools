package launch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

// The builder's diff is retained before any proof can run or change its files.
func (runner *UnitRunner) freezeBuildOutcome(record *UnitRunRecord, round *UnitRound, plan UnitPlan, builds int) (bool, error) {
	path := filepath.Join(round.Directory, "worktree.diff")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := runner.writeDiff(plan.Worktree, plan.Base, path); err != nil {
			return false, err
		}
	}
	if round.Result == nil {
		snapshot, err := runner.snapshotRepository(plan.Worktree)
		if err != nil {
			return false, err
		}
		patch, err := runner.WorktreeDiff(plan.Worktree, strings.TrimSpace(snapshot.Head))
		if err != nil {
			return false, err
		}
		if _, err := atomicfile.WriteText(filepath.Join(round.Directory, "result.patch"), string(patch), runner.root()); err != nil {
			return false, err
		}
		round.Result = &RoundResult{Tree: snapshot.TreeID, Parent: strings.TrimSpace(snapshot.Head), PatchDigest: digestHex(patch)}
	}
	gap := false
	if round.DeclaredLines == 0 {
		var err error
		if gap, err = runner.countBuildLines(round, plan, builds, round.Directory); err != nil {
			return false, err
		}
	}
	oversized := round.BuildLines > round.DeclaredLines && round.BuildLines-round.DeclaredLines > round.DeclaredLines
	if !gap && (!oversized || round.SizeAcceptedBy != "") {
		if len(record.Rounds) > 1 {
			previous := record.Rounds[len(record.Rounds)-2]
			if previous.Outcome == "build-gap" || previous.Outcome == "build-size" {
				entry, _ := json.Marshal(map[string]any{"kind": "stop", "status": "cleared", "stop": previous.Stop})
				_, err := atomicfile.WriteFile(filepath.Join(previous.Directory, "stop-register.json"), entry, 0600, runner.root())
				if err != nil {
					return false, err
				}
			}
		}
		return false, runner.save(*record)
	}
	round.Outcome, round.Cause, round.Material, record.State = "build-size", "unclassified", -1, "awaiting-judgement"
	s := loopstop.Stop{Loop: "unit-build", Subject: record.Goal + "/" + record.Unit + "/" + record.ID, Attempt: round.Number,
		Decision: "stop", Handoff: "hold size", Class: "the builder exceeded twice its declared size", Evidence: path,
		At: runner.Manager.Now().UTC().Format(time.RFC3339Nano)}
	s.Measure.Name, s.Measure.Now, s.Measure.Previous = "changed lines", []string{fmt.Sprint(round.BuildLines)}, []string{fmt.Sprint(round.DeclaredLines)}
	if gap {
		round.Outcome, s.Handoff, s.Class = "build-gap", "stopped (gap)", "the builder left no change"
		message, err := runner.builderMessage(*round, builds)
		if err != nil {
			return true, err
		}
		round.GapMessage = message
	}
	s.Cause = &loopstop.Cause{Kind: "unclassified", Goal: record.Goal, Evidence: path}
	round.Stop = &s
	if err := runner.saveDecision(record, round); err != nil {
		return true, err
	}
	runner.publishJudgement(*record, round.Number)
	return true, nil
}

// countBuildLines adds the builder's changed lines, from directory's
// worktree.diff against the round's starting diff, and its declared lines;
// gap is a change that is empty.
func (runner *UnitRunner) countBuildLines(round *UnitRound, plan UnitPlan, builds int, directory string) (bool, error) {
	diff, err := runner.diffSince(UnitRunRecord{Worktree: plan.Worktree, Base: plan.Base}, UnitRound{Directory: filepath.Join(round.Directory, "build-before")}, UnitRound{Number: round.Number, Directory: directory}, "records", "metasystem/records")
	if err != nil {
		return false, err
	}
	for _, step := range round.Steps[:builds] {
		launch, err := runner.Manager.Store.Read(step.LaunchID)
		if err != nil {
			return false, err
		}
		round.DeclaredLines += launch.DeclaredLines
	}
	for _, block := range parseDiff(diff) {
		round.BuildLines += block.lines
	}
	return len(strings.TrimSpace(string(diff))) == 0, nil
}

// countFailedBuild counts a stopped build's change for the person's question
// from its own diff; the round's worktree.diff stays the frozen result of a
// build that passes. A count that fails leaves no counts: the stop never
// waits on it.
func (runner *UnitRunner) countFailedBuild(round *UnitRound, plan UnitPlan, builds int) {
	directory := filepath.Join(round.Directory, "failed-build")
	round.BuildLines, round.DeclaredLines, round.GapMessage = 0, 0, ""
	err := runner.writeDiff(plan.Worktree, plan.Base, filepath.Join(directory, "worktree.diff"))
	if err == nil {
		_, err = runner.countBuildLines(round, plan, builds, directory)
	}
	if err == nil {
		round.GapMessage, err = runner.builderMessage(*round, builds)
	}
	if err != nil {
		round.BuildLines, round.DeclaredLines, round.GapMessage = 0, 0, ""
	}
}

// builderMessage is the path of the last builder turn's final message.
func (runner *UnitRunner) builderMessage(round UnitRound, builds int) (string, error) {
	last, err := runner.Manager.Store.Read(round.Steps[builds-1].LaunchID)
	if err != nil {
		return "", err
	}
	state, err := runner.Manager.Store.StateDir(last.ID)
	if err != nil {
		return "", err
	}
	name := "last-message.txt"
	if last.Adapter == "claude-headless" {
		name = "result.json"
	}
	return filepath.Join(state, name), nil
}

// AcceptBuildSize validates the retained builder change and records the
// person's impact under the run lock before clearing its hold.
func (runner *UnitRunner) AcceptBuildSize(id, person string, recordImpact func() error) (UnitResult, error) {
	record, err := runner.read(id)
	if err != nil {
		return UnitResult{}, err
	}
	if runner.tree == nil {
		return treeCall(runner, record.Worktree, func(r *UnitRunner) (UnitResult, error) { return r.AcceptBuildSize(id, person, recordImpact) })
	}
	held, err := runner.lock(id)
	if err != nil {
		return UnitResult{}, err
	}
	defer releaseUnitLock(held)
	record, err = runner.read(id)
	if err != nil {
		return UnitResult{}, err
	}
	if person == "" || recordImpact == nil || len(record.Rounds) == 0 {
		return UnitResult{Record: record}, fmt.Errorf("accepting the builder's size needs a proven person")
	}
	round := &record.Rounds[len(record.Rounds)-1]
	if round.Outcome != "build-size" && round.SizeAcceptedBy == "" {
		return UnitResult{Record: record}, fmt.Errorf("this round has no pending size decision")
	}
	plan, err := readUnitPlan(record.Plan, record.PlanDirectory)
	if err != nil {
		return UnitResult{Record: record}, err
	}
	before := UnitRound{Directory: filepath.Join(round.Directory, "build-before")}
	scope := UnitRunRecord{Worktree: plan.Worktree, Base: plan.Base}
	frozen, err := runner.diffSince(scope, before, *round, "records", "metasystem/records")
	if err != nil {
		return UnitResult{Record: record}, err
	}
	directory, done, err := diskstore.ScratchDir("metasystem-unit-size.")
	if err != nil {
		return UnitResult{Record: record}, err
	}
	defer done()
	if err := runner.writeDiff(plan.Worktree, plan.Base, filepath.Join(directory, "worktree.diff")); err != nil {
		return UnitResult{Record: record}, err
	}
	current, err := runner.diffSince(scope, before, UnitRound{Number: round.Number, Directory: directory}, "records", "metasystem/records")
	if err != nil {
		return UnitResult{Record: record}, err
	}
	if !bytes.Equal(current, frozen) {
		return UnitResult{Record: record}, fmt.Errorf("the builder's retained change moved; restore that change before accepting its size")
	}
	if err := recordImpact(); err != nil {
		return UnitResult{Record: record}, err
	}
	entry, _ := json.Marshal(map[string]any{"kind": "stop", "status": "cleared", "stop": round.Stop})
	if _, err := atomicfile.WriteFile(filepath.Join(round.Directory, "stop-register.json"), entry, 0600, runner.root()); err != nil {
		return UnitResult{Record: record}, err
	}
	round.SizeAcceptedBy, round.Stop, round.Outcome, round.Cause, record.State = person, nil, "", "", "running"
	err = runner.save(record)
	return UnitResult{Record: record, Round: round.Number}, err
}
