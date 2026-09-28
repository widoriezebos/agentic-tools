package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The goal CLI shell bed's carry scenarios at the command edge
// (goal-cli-fixtures.sh carry-word's two asks and carried-discharge), on the
// Git-free goal CLI bed. The carry word's own lifecycle and the carried record
// are TestGoalCLICarryWord and TestGoalCLICarryRecord in internal/goal, whose
// fake repository answers the code-origin trailer queries the carry cap and
// the carried record read.

// gcliCarryAsk runs the carry owner as goal carry does, from the person's
// fixture proof through the request to goal.Carry, and maps its outcome to
// the command's exit code (printCarryMutation: an ask exits 3).
func gcliCarryAsk(t *testing.T, bed *goalCLIBed, why string, raise bool) (int, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	dependencies := bed.dependencies(&stdout, &stderr)
	f := &syncFlags{root: bed.root, id: "ship-widget", by: "Wido", fixtureHumanAuthority: true}
	classification, err := classifyGoalAuthorityFirstWithFacts("carry", f, dependencies.authorityFacts)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := proveGoalHumanAuthorityAt("carry", f, proveEnrolledGoalHumanAuthority, bed.commandNow)
	if err != nil {
		t.Fatal(err)
	}
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(bed.root, "Wido", "", &proof, classification, false, bed.commandNow, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	result, err := goal.Carry(req, goal.CarryArgs{Goal: "ship-widget", Workspace: strings.Repeat("a", 40), Past: "missing-declaration",
		Why: why, Expires: req.Now.Add(2 * time.Hour), RaiseFormat: raise}, &proof)
	if err == nil && result.Outcome == goal.OutcomeConfirmed {
		return 0, nil
	}
	return printCarryMutation(result, "", err), err
}

// TestGoalCLICarryAsks is carry-word's two asks: a single-machine checkout
// (goal.sync-remote local) is asked for a code remote, and a format-1 ledger
// for the one-way raise, each with exit 3 and nothing published.
func TestGoalCLICarryAsks(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	before := bed.tip()

	bed.remote = "local"
	code, err := gcliCarryAsk(t, bed, "fixture remote fence", false)
	var ask *goal.CarryAskError
	if code != 3 || !errors.As(err, &ask) ||
		!strings.Contains(err.Error(), "carry-remote-required: a carried landing needs a code remote: set goal.sync-remote") {
		t.Fatalf("single-machine carry did not ask for a code remote: rc=%d %v", code, err)
	}

	bed.remote = "origin"
	code, err = gcliCarryAsk(t, bed, "fixture format fence", false)
	if code != 3 || !errors.As(err, &ask) || ask.Code != "carry-format-required" || !strings.Contains(err.Error(), "carry-format-required") {
		t.Fatalf("format-1 carry did not ask for the one-way raise: rc=%d %v", code, err)
	}
	if bed.tip() != before {
		t.Fatal("a carry ask published to the accepted ledger")
	}
}

// TestGoalCLICarryDischarge is the carried-discharge scenario: the person's
// accept-risk on the human-carried chain discharges the carried commit's
// review obligation on the goal and appends the counselor's accepted-risk
// register exactly once.
func TestGoalCLICarryDischarge(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("6", 40)
	finding := "carried:" + commit
	bed := newGoalCLIBed(t, goalCLISeed{
		rootRecord: func(record *goal.RootRecord) { record.FormatVersion = "2" },
		amend: func(goals map[string]*goal.GoalFile) {
			// The open obligation goal carried writes for the carried commit.
			goals["ship-widget"].ReviewObligations = []goal.ReviewObligation{{
				Finding: finding, Chain: goal.HumanCarriedChain, Artifact: "commit:" + commit, Test: "pending", State: "open",
			}}
		},
	})
	// The carried commit's trailers, as prepare_carried_record_fixture
	// committed them.
	message := strings.Join([]string{
		"carried fixture", "", "Goal-Item: ship-widget", "Carry: 01ARZ3NDEKTSV4RRFFQ69G5FB0-fixture-machine-1a2b3c4d", "Carried-By: human:Wido",
		"Carried-Tree: workspace=" + strings.Repeat("a", 40) + " project=" + strings.Repeat("b", 40),
		"Carried-Past: missing-declaration", "Carried-Battery: green",
		"Carried-Judge: live sha256=" + strings.Repeat("d", 64),
		"Carried-Ledger: " + strings.Repeat("e", 40),
		"Landing-Provenance: carried opid=01ARZ3NDEKTSV4RRFFQ69G5FB0-fixture-machine-1a2b3c4d past=missing-declaration", "",
	}, "\n")
	readCommit := func(root, got string) ([]byte, error) {
		if root != bed.root || got != commit {
			t.Fatalf("commit message requested for root=%q commit=%q", root, got)
		}
		return []byte(message), nil
	}
	code, stdout, stderr := bed.owner(func(dependencies syncRequestDependencies) int {
		return runGoalAcceptRiskWithFacts([]string{"--root", bed.root, "--id", "ship-widget", "--finding", finding,
			"--chain", goal.HumanCarriedChain, "--by", "Wido", "--why", "fixture accepts the deferred review", "--fixture-human-authority"},
			bed.prove, bed.commandNow, dependencies, readCommit)
	})
	if code != 0 || !strings.Contains(stdout, `"outcome":"confirmed"`) {
		t.Fatalf("human-carried accept-risk did not confirm: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	record := bed.goalRecord("ship-widget")
	if !strings.Contains(record, "finding="+finding+" chain=human-carried artifact=\"commit:"+commit+"\" test=\"accepted-risk:") {
		t.Fatalf("accepted risk did not discharge the carried obligation:\n%s", record)
	}
	if !strings.Contains(record, "state=discharged") {
		t.Fatalf("accepted risk left the carried obligation open:\n%s", record)
	}
	register, err := os.ReadFile(filepath.Join(bed.root, "records", "counselor", "accepted-risk-register.jsonl"))
	if err != nil || len(register) == 0 || bytes.Count(register, []byte("\n")) != 1 {
		t.Fatalf("accepted-risk counselor register was not append-once: err=%v\n%s", err, register)
	}
	// R-129: the same decision again is success with no second record.
	tip := bed.tip()
	code, stdout, stderr = bed.owner(func(dependencies syncRequestDependencies) int {
		return runGoalAcceptRiskWithFacts([]string{"--root", bed.root, "--id", "ship-widget", "--finding", finding,
			"--chain", goal.HumanCarriedChain, "--by", "Wido", "--why", "fixture accepts the deferred review", "--fixture-human-authority"},
			bed.prove, bed.commandNow, dependencies, readCommit)
	})
	register, err = os.ReadFile(filepath.Join(bed.root, "records", "counselor", "accepted-risk-register.jsonl"))
	if code != 0 || !strings.Contains(stdout, "is already accepted by Wido") || bed.tip() != tip || err != nil || bytes.Count(register, []byte("\n")) != 1 || stderr != "" {
		t.Fatalf("accept-risk replay was not idempotent: code=%d stdout=%q stderr=%q err=%v\n%s", code, stdout, stderr, err, register)
	}
}
