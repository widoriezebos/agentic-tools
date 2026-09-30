package main

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// TestMessageProofAdmissionRefusalsArePlain: the eight admission refusals a
// test run's parent branches on (structured-output U1) are ordinary
// messages: line 1 plain words with no code or key=value fact, line 2 a
// "run:" command when one resolves it. The code travels in the envelope's
// code field, the candidate's state in its data.
func TestMessageProofAdmissionRefusalsArePlain(t *testing.T) {
	t.Parallel()
	codeWord := regexp.MustCompile(`\b[A-Z][A-Z0-9]*_[A-Z0-9_]+\b|\b[a-zA-Z]+=`)
	cases := []struct {
		code string
		err  error
		run  string
	}{
		{"CANDIDATE_GOAL_MOVED", proofAdmissionMoved(proofAdmissionSnapshots{}, proofAdmissionSnapshots{Candidate: proofAdmissionGoalSnapshot{Revision: 2}}, "goal-a", "goal-b"), ""},
		{"CANDIDATE_GOAL_REFUSED", candidateGoalRefusal("goal-a", "fenced", "stopId=s1"), "metasystem goal show goal-a"},
		{"PROOF_AUTHORITY_REQUIRED", proofAuthorityRefusal("goal-b", "is not a live claim"), ""},
		{"PROOF_AUTHORITY_ARC_MATE_REFUSED", arcMateRefusal("goal-a", "goal-b", "arc-1"), "metasystem goal claim goal-a"},
		{"GOAL_REVISION_MOVED", goalRevisionMoved(1, 2, 3, 4), ""},
		{"BATCH_MEMBER_BUDGET_REFUSED", batchMemberBudgetRefused("goal-a", 90), "metasystem goal budget goal-a BOX"},
		{"LANE_ACCOUNT_UNRESOLVED", laneAccountUnresolved("%q is not a lane accounting identity", "x"), ""},
	}
	for _, test := range cases {
		var coded *refusal.Coded
		if !errors.As(test.err, &coded) || coded.Code != test.code {
			t.Errorf("%s: not a coded refusal: %#v", test.code, test.err)
			continue
		}
		lines := strings.Split(test.err.Error(), "\n")
		if codeWord.MatchString(lines[0]) {
			t.Errorf("%s: line 1 carries a code or fact: %q", test.code, lines[0])
		}
		if test.run == "" && len(lines) != 1 || test.run != "" && (len(lines) != 2 || lines[1] != "run: "+test.run) {
			t.Errorf("%s: lines = %q, want run %q", test.code, lines, test.run)
		}
	}
	result := verbresult.FromError("internal test run", 78, candidateGoalRefusal("goal-a", "fenced", "stopId=s1"), nil)
	if result.Code != "CANDIDATE_GOAL_REFUSED" || string(result.Data) != `{"state":"fenced"}` {
		t.Errorf("the fenced refusal's envelope = %+v data=%s", result, result.Data)
	}
}
