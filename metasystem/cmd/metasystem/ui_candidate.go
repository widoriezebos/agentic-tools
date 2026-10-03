package main

import (
	"bytes"
	"encoding/json"
	"regexp"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// candidateRun is one of the three app forms for a goal's candidate, as the
// room's pill performs it (g1-s69 D3): app status, start or stop --goal G, run
// in this process exactly as the public verb runs, and read back from its one
// JSON result. A project with no launch contract is refused in words before
// anything runs, and so is anything the verb itself refused.
func candidateRun(roots lifecycle.Roots, goal, at, action string) (httpd.Candidate, error) {
	return candidateRunWith(roots, goal, at, action, defaultIntentOwners())
}

// candidateTarget is how an app form names the candidate's run: the commit a
// review reviews, with --at, so a person tries the version they are deciding
// on (review-findings-read-as-decisions RF-04), or the goal's branch as it
// stands, with --goal, where no commit is named.
func candidateTarget(goal, at string) []string {
	if at != "" {
		return []string{"--at", at}
	}
	return []string{"--goal", goal}
}

func candidateRunWith(roots lifecycle.Roots, goal, at, action string, owners intentOwners) (httpd.Candidate, error) {
	if at != "" && !commitName.MatchString(at) {
		return httpd.Candidate{}, &httpd.CandidateRefusal{Code: "action", Message: "a candidate runs at a whole commit id, and " + at + " is not one"}
	}
	if _, _, _, err := loadPhysicalLaunchContract(roots.Installation); err != nil {
		return httpd.Candidate{}, &httpd.CandidateRefusal{Code: httpd.CodeNoContract,
			Message: "this goal's candidate cannot run from here: " + err.Error()}
	}
	command, found := findIntentCommand("app " + action)
	if !found || (action != "status" && action != "start" && action != "stop") {
		return httpd.Candidate{}, &httpd.CandidateRefusal{Code: "action", Message: "a candidate is read, started or stopped; " + action + " is none of them"}
	}
	var stdout, stderr bytes.Buffer
	runIntentIn(command, append(candidateTarget(goal, at), "--json", "--repo", roots.Checkout), &stdout, &stderr, roots.Checkout, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return httpd.Candidate{}, &httpd.CandidateRefusal{Code: "unreadable", Message: "app " + action + " answered nothing the room can read: " + string(bytes.TrimSpace(stderr.Bytes()))}
	}
	if result.Outcome != intentConfirmed && result.Outcome != intentUnchanged {
		return httpd.Candidate{}, &httpd.CandidateRefusal{Code: "refused", Message: result.Summary}
	}
	data, _ := result.Data.(map[string]any)
	text := func(key string) string {
		said, _ := data[key].(string)
		return said
	}
	return httpd.Candidate{Goal: goal, State: text("state"), Readiness: text("readiness"), Address: text("address"),
		Commit: text("commit"), Since: text("since"), Said: result.Summary}, nil
}

// commitName is a whole commit id, which is all a candidate's --at is given here.
var commitName = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
