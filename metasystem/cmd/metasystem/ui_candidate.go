package main

import (
	"bytes"
	"encoding/json"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// candidateRun is one of the three app forms for a goal's candidate, as the
// room's pill performs it (g1-s69 D3): app status, start or stop --goal G, run
// in this process exactly as the public verb runs, and read back from its one
// JSON result. A project with no launch contract is refused in words before
// anything runs, and so is anything the verb itself refused.
func candidateRun(roots lifecycle.Roots, goal, action string) (httpd.Candidate, error) {
	return candidateRunWith(roots, goal, action, defaultIntentOwners())
}

func candidateRunWith(roots lifecycle.Roots, goal, action string, owners intentOwners) (httpd.Candidate, error) {
	if _, _, _, err := loadPhysicalLaunchContract(roots.Installation.Path()); err != nil {
		return httpd.Candidate{}, &httpd.CandidateRefusal{Code: httpd.CodeNoContract,
			Message: "this goal's candidate cannot run from here: " + err.Error()}
	}
	command, found := findIntentCommand("app " + action)
	if !found || (action != "status" && action != "start" && action != "stop") {
		return httpd.Candidate{}, &httpd.CandidateRefusal{Code: "action", Message: "a candidate is read, started or stopped; " + action + " is none of them"}
	}
	var stdout, stderr bytes.Buffer
	runIntentIn(command, []string{"--goal", goal, "--json", "--repo", roots.Checkout}, &stdout, &stderr, roots.Checkout, owners)
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
