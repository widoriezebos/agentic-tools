package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatchproc"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// criticCustody reads every examination round, including a dispatch whose
// command ended before retaining its result. Commit and goal are immutable
// dispatch identities; a later round cannot hide an earlier live process.
func (inv *intentInvocation) criticCustody(run launch.UnitRunRecord, cancel bool) (bool, error) {
	root := inv.goalWorktreeInstallation(run.Worktree)
	var paths []string
	for _, store := range []string{root, inv.layout.InstallationRoot.Path()} {
		if store == "" {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(store, "artifacts", "agents", "jobs"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".json" {
				path := filepath.Join(store, "artifacts", "agents", "jobs", entry.Name())
				if !slices.Contains(paths, path) {
					paths = append(paths, path)
				}
			}
		}
	}
	ended := true
	for _, path := range paths {
		record, err := dispatchcore.ReadRecordObject(path)
		if err != nil {
			return false, err
		}
		if record["role"] != "code-critic" || record["goalId"] != run.Goal {
			continue
		}
		owned := false
		for _, subject := range run.Subjects {
			owned = owned || subject.Commit != "" && record["reviews"] == "commit:"+subject.Commit
		}
		if !owned {
			continue
		}
		job, ok := record["jobId"].(string)
		if !ok || filepath.Base(path) != job+".json" {
			return false, fmt.Errorf("the critic record has no exact job identity: %s", path)
		}
		store := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(path))))
		if cancel {
			_, code, err := inv.work().cancelCritic(store, job)
			if err != nil || code != 0 {
				return false, fmt.Errorf("critic %s could not be cancelled (exit %d): %v", job, code, err)
			}
			record, err = dispatchcore.ReadRecordObject(path)
			if err != nil {
				return false, err
			}
		}
		status, _ := record["status"].(string)
		if !dispatchcore.TerminalStatus(status) {
			ended = false
			continue
		}
		if dispatchcore.NeverLaunched(record) || status == "cancelled" && record["phase"] == "cancelled" && record["pid"] == nil && record["pgid"] == nil {
			continue
		}
		dependencies := inv.work().criticDeath
		if dependencies.MatchesTag == nil {
			dependencies.MatchesTag = dispatchproc.PositionedJobTagAt(store)
		}
		death := dispatchcore.ProveCustodyDeath(store, record, dependencies)
		if death.Process != nil {
			return false, fmt.Errorf("critic %s process pid %d, start %d still holds the worktree", job, death.Process.Pid, death.Process.StartedAtSec)
		}
		if death.Outcome != dispatchcore.CustodyDeathProven {
			ended = false
		}
	}
	return ended, nil
}

func (inv *intentInvocation) treeFailure(err error) intentResult {
	if launch.IsCode(err, "UNIT_RUN_BUSY") {
		plain, details := launchAccount(err)
		return intentResult{Outcome: intentInProgress, code: 3, Summary: plain, Details: details,
			next: inv.sameCommand(), nextReason: "continues when the command changing the worktree finishes"}
	}
	var waiting *launch.TreeWaitingError
	if errors.As(err, &waiting) {
		return intentResult{Outcome: intentInProgress, code: 3, Summary: waiting.Error(), next: inv.publicArgv("work", "wait", "run:"+waiting.Run)}
	}
	var damaged *launch.TreeOwnershipError
	if errors.As(err, &damaged) {
		result := intentResult{Outcome: intentFailed, code: 1, Summary: "the worktree ownership cannot be read; nothing was started", Details: []string{err.Error()}}
		if damaged.Run != "" {
			result.next = inv.publicArgv("work", "stop", "run:"+damaged.Run)
			result.nextReason = "a person stops this tree's run and releases it after its children end; then repeat this command"
		} else {
			result.next = inv.publicArgv("question", "ask", "--about", "machine", "--question", "The worktree ownership is damaged and no run can be named. A person needs to recover its ownership.", "--option", "Recover: inspect this tree's runs and stop its exact owner")
			result.nextReason = "ask a person to recover this tree; its owner could not be identified"
		}
		return result
	}
	return intentResult{Outcome: intentFailed, code: 1, Summary: "the worktree ownership cannot be read; nothing was started", Details: []string{err.Error()}, next: inv.sameCommand()}
}

func (inv *intentInvocation) stopUnitRun(id string) int {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.directPersonProof("cancellation of a unit run"); problem != nil {
		return inv.render(*problem)
	}
	record, err := inv.unitRunner().CancelRun(id)
	targets := []intentTarget{{Kind: "unit", ID: id}}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the run could not be fully stopped; its worktree remains reserved: " + err.Error(), Details: []string{err.Error()}, next: inv.sameCommand(), Data: record})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "run " + id + " cancelled; its children ended and its worktree is released", Data: record})
}
