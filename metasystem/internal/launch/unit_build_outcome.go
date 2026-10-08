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
	gap := false
	if round.DeclaredLines == 0 {
		diff, err := runner.diffSince(UnitRunRecord{Worktree: plan.Worktree, Base: plan.Base}, UnitRound{Directory: filepath.Join(round.Directory, "build-before")}, *round, "records", "metasystem/records")
		if err != nil {
			return false, err
		}
		gap = len(strings.TrimSpace(string(diff))) == 0
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
		last, err := runner.Manager.Store.Read(round.Steps[builds-1].LaunchID)
		if err != nil {
			return true, err
		}
		state, err := runner.Manager.Store.StateDir(last.ID)
		if err != nil {
			return true, err
		}
		name := "last-message.txt"
		if last.Adapter == "claude-headless" {
			name = "result.json"
		}
		round.GapMessage = filepath.Join(state, name)
	}
	s.Cause = &loopstop.Cause{Kind: "unclassified", Goal: record.Goal, Evidence: path}
	round.Stop = &s
	if err := runner.saveDecision(record, round); err != nil {
		return true, err
	}
	runner.publishJudgement(*record, round.Number)
	return true, nil
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
