package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func (owners laneVerbOwners) incidentRequest(installation string) (goal.VerbRequest, error) {
	endpoint := owners.mainEndpoint
	if endpoint == nil {
		endpoint = branch.MainEndpoint
	}
	e, err := endpoint(installation)
	if err != nil {
		return goal.VerbRequest{}, err
	}
	machine, err := owners.machine(installation)
	if err != nil {
		return goal.VerbRequest{}, err
	}
	ulid, err := goalUlid()
	return goal.VerbRequest{Endpoint: e, Actor: goal.Actor{Machine: machine, Lineage: lane.AgentLineage}, Ulid: ulid, Now: owners.now()}, err
}

func (owners laneVerbOwners) landingIncidentRecorder(installation string) func([]plain.Result) error {
	return func(checks []plain.Result) error {
		if len(checks) == 0 {
			return nil
		}
		r, err := owners.incidentRequest(installation)
		if err != nil {
			return err
		}
		first := checks[0]
		args := goal.TrunkRedRecordArgs{Batch: first.Attempt, Attempt: first.Attempt, BaseCommit: first.Commit, BaseTree: first.Tree, SeenAt: r.Now.UTC().Format("2006-01-02T15:04:05Z"), Class: goal.TrunkRedClassTrunkRed}
		for _, check := range checks {
			if check.Result != plain.Red || check.Commit != first.Commit || check.Tree != first.Tree {
				return fmt.Errorf("main's failed checks must name the same commit and tree")
			}
			data, err := os.ReadFile(check.Log)
			if err != nil {
				return err
			}
			for _, unit := range check.Failed {
				tests := unit.Tests
				if len(tests) == 0 {
					tests = []string{""}
				}
				for _, test := range tests {
					group := goal.TrunkRedRecordGroup{Identity: "red:" + unit.Unit, Group: unit.Unit, Status: "failed", LogPath: check.Log, LogDigest: fmt.Sprintf("%x", sha256.Sum256(data)), Failures: []goal.TrunkRedFailure{}}
					if test != "" {
						group.Identity += ":" + test
						group.Failures = append(group.Failures, goal.TrunkRedFailure{Report: check.Log, Classname: unit.Unit, Name: test, Status: "failed"})
					}
					args.Groups = append(args.Groups, group)
				}
			}
		}
		record := owners.recordMain
		if record == nil {
			record = goal.RecordTrunkRed
		}
		result, err := record(r, args)
		return confirmedIncidentWrite(result, err)
	}
}

func confirmedIncidentWrite(result goal.PublishResult, err error) error {
	if err == nil && result.Outcome != goal.OutcomeConfirmed {
		err = fmt.Errorf("main's incident register was not updated (%s): %s", result.Outcome, result.Detail)
	}
	return err
}

func (owners laneVerbOwners) clearLandingIncidents(installation string, proof plain.Result) error {
	if proof.Result != plain.Green || proof.Scope != "full" || proof.FullTree != proof.Tree || proof.FullAt != proof.At || strings.HasPrefix(proof.Reason, "inherits green from tree ") {
		return nil
	}
	r, err := owners.incidentRequest(installation)
	if err != nil {
		return err
	}
	var files map[string][]byte
	if r.Endpoint.Repository != nil {
		files, err = r.Endpoint.Repository.Files(proof.Commit, "plans/goals/trunk-red.json")
	} else {
		files, err = goal.ReadCommitGoals(r.Endpoint.Root, proof.Commit)
	}
	if err != nil {
		return err
	}
	data, present := files["plans/goals/trunk-red.json"]
	if !present {
		return nil
	}
	entries, problems := goal.ParseTrunkRed(data)
	if len(problems) > 0 {
		return fmt.Errorf("main's incident register cannot be read: %v", problems)
	}
	clear := owners.clearMain
	if clear == nil {
		clear = goal.ClearTrunkRed
	}
	for _, entry := range entries {
		if entry.Closed != nil || !strings.HasPrefix(entry.Identity, "red:") || entry.EntryClass() != goal.TrunkRedClassTrunkRed {
			continue
		}
		r.Ulid, err = goalUlid()
		if err != nil {
			return err
		}
		result, err := clear(r, goal.TrunkRedClearArgs{Entry: entry.ID, Attempt: proof.Attempt, BaseCommit: proof.Commit, BaseTree: proof.Tree, Group: entry.Group, ExpectedEntry: entry, Executed: true})
		if err := confirmedIncidentWrite(result, err); err != nil {
			return err
		}
		if err := plain.CloseIncidentStop(installation, entry.Identity, owners.now()); err != nil {
			return err
		}
	}
	return nil
}
