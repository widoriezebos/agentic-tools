package main

import (
	"crypto/sha256"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"os"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func (owners laneVerbOwners) incidentRequest(installation string) (goal.VerbRequest, error) {
	endpoint := owners.mainEndpoint
	if endpoint == nil {
		endpoint = mainPublicationEndpoint
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
	full := proof.Scope == "full" && proof.FullTree == proof.Tree && proof.FullAt == proof.At
	if !proof.Executed() || proof.Result != plain.Green || !(full || proof.Trunk && proof.Scope == "scoped") {
		return nil
	}
	r, err := owners.incidentRequest(installation)
	if err != nil {
		return err
	}
	seams := owners.proveSeams(installation)
	read := func(commit string) ([]goal.TrunkRedEntry, error) {
		if r.Endpoint.Repository != nil {
			files, err := r.Endpoint.Repository.Files(commit, "plans/goals/trunk-red.json")
			if err != nil {
				return nil, err
			}
			if data, present := files["plans/goals/trunk-red.json"]; present {
				entries, problems := goal.ParseTrunkRed(data)
				if len(problems) > 0 {
					return nil, fmt.Errorf("main's incident register cannot be read: %v", problems)
				}
				return entries, nil
			}
			return nil, nil
		}
		checkout, err := seams.Git(r.Endpoint.Root, "rev-parse", "--show-toplevel")
		if err != nil {
			return nil, err
		}
		return plain.ReadIncidents(installation, strings.TrimSpace(checkout), commit, seams.Git)
	}
	entries, err := read(proof.Commit)
	if err != nil || len(entries) == 0 {
		return err
	}
	currentRef := "refs/remotes/origin/main"
	if r.Endpoint.LocalMode() {
		currentRef = goal.LocalLedgerBranch
	}
	main, err := seams.Git(r.Endpoint.Root, "rev-parse", "--verify", "--quiet", currentRef+"^{commit}")
	if err != nil {
		return err
	}
	closed := map[string]bool{}
	if main = strings.TrimSpace(main); main != proof.Commit {
		current, err := read(main)
		if err != nil {
			return err
		}
		for _, entry := range current {
			if entry.Closed != nil {
				closed[entry.ID] = true
			}
		}
	}
	clear := owners.clearMain
	if clear == nil {
		clear = goal.ClearTrunkReds
	}
	var clears []goal.TrunkRedClearArgs
	applicable := func(tip string) error {
		if tip == proof.Commit {
			return nil
		}
		if full {
			main, err := seams.Git(installation, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}")
			if err != nil {
				return err
			}
			if strings.TrimSpace(main) == proof.Commit {
				return nil
			}
		}
		checkout, err := seams.Git(r.Endpoint.Root, "rev-parse", "--show-toplevel")
		if err != nil {
			return err
		}
		applies, err := plain.ProofStillApplies(installation, strings.TrimSpace(checkout), proof.Commit, tip, seams)
		if err == nil && !applies {
			err = goal.Coded("TRUNK_RED_PROOF_STALE", fmt.Errorf("the green no longer applies to main; run metasystem landing prove"))
		}
		return err
	}
	for _, entry := range entries {
		if entry.Closed != nil || closed[entry.ID] || !strings.HasPrefix(entry.Identity, "red:") || entry.EntryClass() != goal.TrunkRedClassTrunkRed || !full && !slices.Contains(proof.Ran, entry.Group) {
			continue
		}
		clears = append(clears, goal.TrunkRedClearArgs{Entry: entry.ID, Attempt: proof.Attempt, BaseCommit: proof.Commit, BaseTree: proof.Tree, Group: entry.Group, ExpectedEntry: entry, Executed: true, Applicable: applicable})
	}
	if len(clears) == 0 {
		return nil
	}
	result, err := clear(r, clears)
	if result.Code == "TRUNK_RED_PROOF_STALE" || goal.RefusalCode(err) == "TRUNK_RED_PROOF_STALE" {
		fmt.Fprintln(os.Stderr, "the green no longer applies to main; run metasystem landing prove")
		return nil
	}
	if err := confirmedIncidentWrite(result, err); err != nil {
		return err
	}
	for _, entry := range clears {
		if err := plain.CloseIncidentStop(installation, entry.ExpectedEntry.Identity, owners.now()); err != nil {
			return err
		}
	}
	return nil
}

func mainPublicationEndpoint(root string) (goal.Endpoint, error) {
	return project.CheckedEndpoint(branch.MainEndpoint(root))
}
