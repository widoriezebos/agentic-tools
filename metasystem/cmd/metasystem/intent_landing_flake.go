package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func landingFlakeJudge(installation string, git func(string, ...string) (string, error)) func(string, string, []plain.FailedUnit) (map[string]plain.UnitJudgement, error) {
	return landingFlakeJudgeFor(installation, git, false)
}

func landingFlakeJudgeFor(installation string, git func(string, ...string) (string, error), gate bool) func(string, string, []plain.FailedUnit) (map[string]plain.UnitJudgement, error) {
	return func(checkout, commit string, failed []plain.FailedUnit) (map[string]plain.UnitJudgement, error) {
		out := map[string]plain.UnitJudgement{}
		for _, unit := range failed {
			out[unit.Unit] = plain.UnitJudgement{Affected: true}
		}
		relative, _, err := config.CommittedLookup(filepath.Join(installation, "metasystem.conf"), "testing.contract")
		if err != nil || relative == "" {
			return out, err
		}
		prefix, err := filepath.Rel(checkout, installation)
		if err != nil {
			return out, err
		}
		data, err := git(checkout, "show", "origin/main:"+filepath.ToSlash(filepath.Join(prefix, relative)))
		if err != nil {
			return out, err
		}
		contract, err := testpolicy.Decode([]byte(data))
		if err != nil {
			return out, err
		}
		comparison := "origin/main..." + commit
		if gate {
			comparison = commit + "^1.." + commit
		}
		changed, err := git(checkout, "diff", "--name-only", comparison)
		if err != nil {
			return out, err
		}
		var changedPaths []string
		if changed != "" {
			changedPaths = strings.Split(strings.TrimSuffix(changed, "\n"), "\n")
		}
		plan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: changedPaths})
		if err != nil {
			return out, err
		}
		data, err = git(checkout, "show", "origin/main:"+filepath.ToSlash(filepath.Join(prefix, "plans/goals/trunk-red.json")))
		if err != nil {
			return out, err
		}
		entries, problems := goal.ParseTrunkRed([]byte(data))
		if len(problems) != 0 {
			return out, fmt.Errorf("main's flaky test register cannot be read: %v", problems)
		}
		filesByUnit := map[string][]string{}
		for _, unit := range failed {
			listing, err := git(checkout, "ls-tree", "-z", "origin/main:"+unit.Unit)
			if err != nil {
				return out, err
			}
			for _, line := range strings.Split(listing, "\x00") {
				if line == "" {
					continue
				}
				header, name, ok := strings.Cut(line, "\t")
				parts := strings.Fields(header)
				if !ok || len(parts) != 3 {
					return out, fmt.Errorf("main's files for %s cannot be read", unit.Unit)
				}
				if parts[1] == "blob" {
					filesByUnit[unit.Unit] = append(filesByUnit[unit.Unit], filepath.ToSlash(filepath.Join(unit.Unit, name)))
				}
			}
		}
		for _, unit := range failed {
			files := filesByUnit[unit.Unit]
			judgement := plain.UnitJudgement{Surfaces: testpolicy.HoldingSurfaces(contract, files), Affected: len(files) == 0 || len(plan.Uncertainty) != 0}
			for _, surface := range judgement.Surfaces {
				judgement.Affected = judgement.Affected || slices.Contains(plan.AffectedSurfaces, surface)
			}
			for _, entry := range entries {
				if entry.Identity != "flaky:"+unit.Unit || entry.Closed != nil || entry.EntryClass() != goal.TrunkRedClassPendingFlake {
					continue
				}
				judgement.Known = len(unit.Tests) > 0
				for _, test := range unit.Tests {
					judgement.Known = judgement.Known && slices.ContainsFunc(entry.Failures, func(f goal.TrunkRedFailure) bool { return f.Name == test })
				}
			}
			out[unit.Unit] = judgement
		}
		return out, nil
	}
}

func (owners laneVerbOwners) landingFlakeRecorder(installation string) func(plain.FlakeRecord) (plain.FlakeRecorded, error) {
	endpoint, record := owners.mainEndpoint, owners.recordFlake
	if endpoint == nil {
		endpoint = branch.MainEndpoint
	}
	if record == nil {
		record = goal.RecordFlake
	}
	return func(f plain.FlakeRecord) (plain.FlakeRecorded, error) {
		e, err := endpoint(installation)
		if err != nil {
			return plain.FlakeRecorded{}, err
		}
		machine, err := owners.machine(installation)
		if err != nil {
			return plain.FlakeRecorded{}, err
		}
		ulid, err := goalUlid()
		if err != nil {
			return plain.FlakeRecorded{}, err
		}
		result, err := record(goal.VerbRequest{Endpoint: e, Actor: goal.Actor{Machine: machine, Lineage: lane.AgentLineage}, Ulid: ulid, Now: owners.now()},
			goal.FlakeRecordArgs{Unit: f.Unit, Tests: f.Tests, Surfaces: f.Surfaces, Commit: f.Commit, Tree: f.Tree, Attempt: f.Attempt, LogPath: f.Log, Load: f.Load, Repeat: f.Repeat,
				Rerun: goal.TrunkRedRerun{Attempt: f.RepeatAttempt, LogPath: f.RepeatLog}})
		if err != nil {
			return plain.FlakeRecorded{}, err
		}
		if result.Publish.Outcome != goal.OutcomeConfirmed {
			return plain.FlakeRecorded{}, fmt.Errorf("the flaky test was not recorded in the shared goal list (%s): %s", result.Publish.Outcome, result.Publish.Detail)
		}
		return plain.FlakeRecorded{Goal: result.FixGoal, Seen: result.Sightings}, nil
	}
}
