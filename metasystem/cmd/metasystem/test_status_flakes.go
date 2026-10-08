package main

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// testStatusInputs binds the proof and accepted ledger readers to one invocation.
type testStatusInputs struct {
	verify   func(testrun.SelectionRequest) (proofrun.TestResult, error)
	endpoint func(string) (goal.Endpoint, error)
	now      func(string) (time.Time, error)
}

func (inputs testStatusInputs) flakes(request testrun.SelectionRequest) ([]goal.FlakeFact, error) {
	endpoint, now := inputs.endpoint, inputs.now
	if endpoint == nil {
		endpoint = goal.ResolveEndpoint
	}
	if now == nil {
		now = goalCommandNow
	}
	end, err := endpoint(request.Root)
	if err != nil {
		return nil, err
	}
	at, err := now(request.Root)
	if err != nil {
		return nil, err
	}
	projection, err := goal.Project(end, false, at)
	if err != nil {
		return nil, err
	}
	facts := []goal.FlakeFact{}
	for _, fact := range goal.FlakeFacts(projection.Tree.TrunkRed) {
		fact.Sightings = slices.DeleteFunc(slices.Clone(fact.Sightings), func(s goal.TrunkRedSighting) bool {
			return s.Tree != request.Tree && (s.Tree != "" || s.BaseTree != request.Tree)
		})
		if len(fact.Sightings) > 0 {
			facts = append(facts, fact)
		}
	}
	return facts, nil
}

// testStatusTo keeps flake observations separate from the retained proof verdict.
func testStatusTo(stdout, stderr io.Writer, request testrun.SelectionRequest, jsonOutput bool, inputs testStatusInputs) int {
	verify := inputs.verify
	if verify == nil {
		verify = verifyRetainedTesting
	}
	result, proofErr := verify(request)
	facts, flakeErr := inputs.flakes(request)
	information := "available"
	if flakeErr != nil {
		information = "flake information unavailable"
	}
	if jsonOutput && proofErr == nil {
		// An unreadable proof writes no result: an empty one would read as "no proof yet".
		writeJSONLine(stdout, stderr, struct {
			proofrun.TestResult
			Flakes           []goal.FlakeFact `json:"flakes"`
			FlakeInformation string           `json:"flakeInformation"`
		}{result, facts, information})
	} else if jsonOutput {
		// The refusal below is the whole answer.
	} else if flakeErr != nil {
		fmt.Fprintln(stdout, information+": "+flakeErr.Error())
	} else if len(facts) > 0 {
		page := passthroughPage(stdout, request.Root, request.Verbose)
		section := page.Section("Flakes observed on tree "+textui.SHA(request.Tree), "a passing repeat records intermittence; it does not fix the test")
		for _, fact := range facts {
			state := "open"
			if fact.Closed != nil {
				state = "closed"
			}
			if fact.FixGoal != "" {
				state += "; goal " + fact.FixGoal + " fixes it"
			} else {
				state += "; fix unowned"
			}
			section.KV(fact.TestUnit+" "+fact.TestName, textui.Plain(state))
			for _, sighting := range fact.Sightings {
				section.KV("observed", textui.Plain(flakeSightingEvidence(sighting)))
			}
		}
		printPage(stdout, page)
	}
	if proofErr != nil {
		printMovedProofInputsWithoutCandidateEngine(stderr, request)
		printTestingRefusal(stderr, proofErr, request)
		return 1
	}
	if !result.Delivery.Sufficient {
		printMovedProofInputs(stderr, request, result)
		retry := []string{"metasystem", "test", "run", "--root", request.Root}
		if request.GoalID != "" {
			retry = append(retry, "--goal", request.GoalID)
		}
		page := passthroughPage(stderr, request.Root, request.Verbose)
		page.Refusal(fmt.Sprintf("no passing test run covers %s on tree %s: %s", textui.Count(len(result.Delivery.MissingGroups), "group", "groups"),
			textui.SHA(result.CandidateTree), strings.Join(result.Delivery.MissingGroups, ", ")),
			textui.Hint{Argv: append(retry, "--tree", result.CandidateTree, "--mode", "auto"), Reason: "runs them"})
		printPage(stderr, page)
		return 1
	}
	if !jsonOutput {
		page := passthroughPage(stdout, request.Root, request.Verbose)
		page.Mark(textui.Done, fmt.Sprintf("Tree %s is proven for delivery", textui.SHA(result.CandidateTree)))
		page.Facts(textui.KV{Key: "groups", Value: []textui.Span{textui.Plain(strings.Join(result.SelectedGroups, ", "))}})
		printPage(stdout, page)
	}
	return 0
}

func flakeSightingEvidence(s goal.TrunkRedSighting) string {
	original, repeat := "failed", "unrecorded"
	evidence := s.LogPath
	if s.Output != nil {
		original = s.Output.Outcome
		evidence += "; test output: " + s.Output.Path + " (" + s.Output.Digest + ")"
	}
	words := "original " + original + " " + s.Attempt + "; evidence: " + evidence
	if s.Rerun != nil {
		repeat = "passed"
		if s.Rerun.Output != nil {
			repeat = s.Rerun.Output.Outcome
		}
		words += "; repeat " + repeat + " " + s.Rerun.Attempt + "; evidence: " + s.Rerun.LogPath
		if s.Rerun.Output != nil {
			words += "; test output: " + s.Rerun.Output.Path + " (" + s.Rerun.Output.Digest + ")"
		}
	} else {
		words += "; repeat " + repeat
	}
	return words
}
