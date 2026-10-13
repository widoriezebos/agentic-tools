package plain

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

var laneRemedyVerbs = regexp.MustCompile(`metasystem landing ([a-z-]+)([^;\n]*)`)
var laneMechanismWords = []string{"barren", "recorded selection", "fingerprint", "sealed window", "admission", "exception code", "lane-return", "attempt ids", "selection recorded", "recorded selection changed", "goal_land_trunk_red", "--exception", "old-launch-1", "old-launch-2"}

func laneRemedyProblem(text string) error {
	for _, word := range laneMechanismWords {
		if strings.Contains(strings.ToLower(text), word) {
			return fmt.Errorf("mechanism %q in %q", word, text)
		}
	}
	// A proof/classification act retains the attempt it addresses.
	inspected := regexp.MustCompile(`metasystem landing prove(?: [^;\n]*)?`).ReplaceAllString(text, "metasystem landing prove")
	if regexp.MustCompile(`\d{8}T\d{6}`).MatchString(inspected) {
		return fmt.Errorf("attempt id in %q", text)
	}
	matches := laneRemedyVerbs.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if match[1] == "land-anyway" {
			words := strings.Fields(match[2])
			if len(words) == 0 || !strings.Contains(text, "lets "+words[0]+" land while main's incident ") || !strings.Contains(text, "undo: metasystem landing give-back "+words[0]+" --reason TEXT") {
				return fmt.Errorf("permission without impact and undo in %q", text)
			}
		}
	}
	for _, match := range matches {
		switch match[1] {
		case "resume", "give-back", "land-anyway", "deploy", "stop", "start":
		case "prove", "set", "unset", "drain", "status":
		default:
			return fmt.Errorf("mechanism verb %q in %q", match[1], text)
		}
	}

	return nil
}

func TestLaneRemediesNameIntentVerbs(t *testing.T) {
	t.Parallel()
	for key, row := range state.Words {
		if row.Verb != "none" && !slices.Contains([]string{"landing resume", "landing give-back", "landing land-anyway", "landing deploy", "landing stop", "landing prove", "landing run --goals"}, row.Verb) {
			t.Errorf("%v: unknown verb %q", key, row.Verb)
		}
		if !strings.Contains(row.Phrase, " ") || row.Person == "" || row.Agent == "" {
			t.Errorf("%v is not actionable: %+v", key, row)
		}
	}
}

func TestLaneRemedyAuditRejectsMutations(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"recorded selection; metasystem landing resume", "selection recorded", "recorded selection changed", "admission", "GOAL_LAND_TRUNK_RED", "--exception", "20261012T123456", "metasystem landing land-anyway g --reason TEXT"} {
		if err := laneRemedyProblem(text); err == nil {
			t.Fatalf("audit accepted mutation %q", text)
		}
	}
	for _, text := range []string{"metasystem landing prove --trunk", "metasystem landing prove --gate", "metasystem landing prove --classify 20261012T123456"} {
		if err := laneRemedyProblem(text); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLaneRemedyKeepsDecisionFactsAndImpact(t *testing.T) {
	t.Parallel()
	stop := Stop{Subject: "g", Cause: &Cause{Kind: "main", Name: "incident-x", Tests: []string{"TestBroken"}}}
	for _, text := range []string{StopHeadline(stop), (PolicyRequest{Required: []string{"metasystem", "landing", "land-anyway", "g", "--reason", "TEXT"}, Subject: PolicySubject{Incidents: []IncidentIdentity{{Identity: "incident-x"}}}}).Remedy()} {
		for _, want := range []string{"g", "incident-x", "lets g land", "undo: metasystem landing give-back g --reason TEXT"} {
			if !strings.Contains(text, want) {
				t.Errorf("lost %q: %s", want, text)
			}
		}
	}
	if text := StopHeadline(stop); !strings.Contains(text, "TestBroken") || !strings.Contains(text, "land a fix on main, then metasystem landing prove --trunk") {
		t.Fatal(text)
	}
	stop.Subject = "g, h"
	for _, subject := range []string{"g", "h"} {
		if text := StopHeadline(stop); !strings.Contains(text, "landing land-anyway "+subject+" --reason TEXT") || !strings.Contains(text, "undo: metasystem landing give-back "+subject+" --reason TEXT") {
			t.Fatal("batch member lost its act: " + text)
		}
	}
	for _, state := range []string{"resolving", "resolved", "running", "reviewing"} {
		headline := fixHeadline(&Fix{Goal: "g", State: state, CheckResult: Red})
		if !strings.HasSuffix(headline, "; nothing to do") || LaneStatusRemedy(lane.View{Owner: lane.OwnerView{State: lane.OwnerHeld}}, false, headline) != "" {
			t.Fatal(headline)
		}
	}
}

func TestLaneRemedyFlakeHeadlinesKeepTests(t *testing.T) {
	t.Parallel()
	for _, published := range []bool{false, true} {
		b := newStatusBed(t)
		b.lines("results.jsonl", Result{Trunk: true, Commit: "main", Result: Green, FlakePublished: published, Reason: "flake on main: TestFlaky", Flakes: []FlakeRecord{{FailedUnit: FailedUnit{Unit: "unit", Tests: []string{"TestFlaky"}}}}})
		status := b.read(ProveSeams{}, b.git(nil, nil))
		if !strings.Contains(status.ProofHeadline, "TestFlaky") {
			t.Fatal(status.ProofHeadline)
		}
	}
}
