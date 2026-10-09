package goal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestDesignItemDischargeRequiresMatchingReadAndPublicTest(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"matching", "text", "person citation", "wrong tree", "generic clean read", "unretained read", "same model", "missing model", "malformed model", "missing test", "skipped test"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			endpoint := obligationAuthorityLocalEndpoint(t, "review-proof")
			req := verbReqFor(endpoint, "01J5X00000000000000000SP01", "mac-a")
			item := &DesignItem{Exit: "exit-one", DesignID: "design-one", BodySHA256: strings.Repeat("a", 64), Unit: "gate", Decision: "6", Tests: []string{"reader/TestPublicGate"}}
			o := ReviewObligation{Finding: "F-1", Chain: "design-critic", Artifact: "a.go", Test: "TestPublicGate", Fixture: "group:section/a", DesignItem: item}
			result, err := DeferFindings(req, "review-proof", []ReviewObligation{o})
			if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("defer: %+v %v", result, err)
			}
			evidence := fixtureEvidence(t, endpoint.Root)
			path, _ := proofrun.AttemptPath(endpoint.Root, "attempt-passed")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var attempt proofrun.Attempt
			if err := json.Unmarshal(data, &attempt); err != nil {
				t.Fatal(err)
			}
			attempt.TestResult.Groups[0].Observed = []proofrun.NativeTestIdentity{{Classname: "reader", Name: "TestPublicGate", Status: "passed"}}
			attempt.TestResult.Groups[0].Expected = []proofrun.NativeTestIdentity{{Classname: "reader", Name: "TestPublicGate", Status: "expected"}}
			returned := map[string]any{"jobId": "critic-root", "round": 1, "reviewedTree": strings.Repeat("b", 40), "coversFindings": []string{"F-1"}}
			switch scenario {
			case "wrong tree":
				attempt.TestResult.CandidateTree = strings.Repeat("c", 40)
			case "generic clean read":
				returned["coversFindings"] = []string{}
			case "unretained read":
				if err := os.Remove(filepath.Join(endpoint.Root, "artifacts", "agents", "critic-root", "rounds", "1", "subject.json")); err != nil {
					t.Fatal(err)
				}
			case "same model":
				writeProofJSON(t, filepath.Join(endpoint.Root, "artifacts", "agents", "jobs", "implementation-chain.json"), map[string]any{"jobId": "implementation-chain", "effectiveModel": "critic-model"})
			case "missing model":
				writeProofJSON(t, filepath.Join(endpoint.Root, "artifacts", "agents", "jobs", "implementation-chain.json"), map[string]any{"jobId": "implementation-chain", "effectiveModel": ""})
			case "malformed model":
				writeProofJSON(t, filepath.Join(endpoint.Root, "artifacts", "agents", "jobs", "implementation-chain.json"), map[string]any{"jobId": "implementation-chain", "effectiveModel": 1})
			case "missing test":
				attempt.TestResult.Groups[0].Observed = nil
				attempt.TestResult.Groups[0].Expected = nil
			case "skipped test":
				attempt.TestResult.Groups[0].Observed[0].Status = "skipped"
			}
			attempt.TestResult.RecomputeDelivery()
			writeProofJSON(t, path, attempt)
			writeProofJSON(t, filepath.Join(endpoint.Root, "artifacts", "agents", "critic-root", "rounds", "1", "return.json"), returned)
			req.Ulid = "01J5X00000000000000000SP02"
			before, _ := Project(endpoint, false, req.Now)
			stored := before.Tree.Live["review-proof"].ReviewObligations[0]
			if !reflect.DeepEqual(stored.DesignItem, item) {
				t.Fatalf("shared metadata lost: %+v", stored)
			}
			if scenario == "text" || scenario == "person citation" {
				if scenario == "person citation" {
					req.Actor.Human = "Wido"
				}
				result, err = DischargeReviewObligation(req, "review-proof", "F-1", "design-critic", "Wido", "TestPublicGate")
			} else {
				result, err = DischargeReviewObligation(req, "review-proof", "F-1", "design-critic", "mac-a", "", evidence)
			}
			if err != nil {
				t.Fatal(err)
			}
			after, _ := Project(endpoint, false, req.Now)
			proved := after.Tree.Live["review-proof"].ReviewObligations[0]
			if scenario == "matching" {
				if result.Outcome != OutcomeConfirmed || proved.State != "discharged" {
					t.Fatalf("matching proof: %+v %+v", result, proved)
				}
			} else if result.Outcome != OutcomeRejected || proved.State != "open" {
				t.Fatalf("%s minted proof: %+v %+v", scenario, result, proved)
			}
			t.Logf("%s: %s (%s)", scenario, result.Outcome, result.Detail)
		})
	}
}

func TestDesignExitReloadAndUnopenedDestinationBlockDone(t *testing.T) {
	t.Parallel()
	endpoint := obligationAuthorityLocalEndpoint(t, "split-source")
	req := verbReqFor(endpoint, "01J5X00000000000000000SP05", "mac-a")
	result, err := Publish(endpoint, PublishRequest{Opid: req.opid(), Machine: req.Actor.Machine, Lineage: req.Actor.Lineage, Intent: Intent{Verb: "defer-findings", Targets: []string{"split-source"}}, Message: "Retain split acceptance", Mutate: func(tip string) ([]Change, error) {
		tree, err := loadTreeFor(endpoint, tip)
		if err != nil {
			return nil, err
		}
		f := tree.Live["split-source"]
		f.DesignExits = []DesignExit{{Operation: "exit-one", DesignID: "design-one", BodySHA256: strings.Repeat("a", 64), Units: []string{"gate"}, Destination: "split-followup", OpenCommand: "metasystem goal open split-followup --brief followup.md --blocked-by split-source"}}
		touch(f, req, "defer-findings", []string{f.Id})
		return []Change{{Path: livePath(f.Id), Content: RenderFile(f)}}, nil
	}, Validate: func(commit string) error { return validateCommitFor(endpoint, commit) }})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("publication: %+v %v", result, err)
	}
	projected, err := Project(endpoint, false, req.Now)
	if err != nil {
		t.Fatal(err)
	}
	f := projected.Tree.Live["split-source"]
	if len(f.DesignExits) != 1 || f.DesignExits[0].Operation != "exit-one" {
		t.Fatalf("clone lost exit: %+v", f.DesignExits)
	}
	req.Ulid = "01J5X00000000000000000SP06"
	result, err = Done(req, "split-source", "The accepted remainder is complete.")
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, f.DesignExits[0].OpenCommand) {
		t.Fatalf("unopened source completed: %+v %v", result, err)
	}
	t.Logf("done: %s; %s", result.Outcome, result.Detail)
}
