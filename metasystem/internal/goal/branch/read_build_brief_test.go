package branch_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func writeBuildBrief(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "build-brief.md")
	if err := os.WriteFile(path, []byte("Build brief: cites plans/handoff-created-at-runtime.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestBuildBriefAdmittedMarksOnlyTheWorkFormsBuildBrief: the brief a work-form
// read composes around the unit's build brief is admitted while its bytes are
// the recorded ones, including on the saved start a refused dispatch repeats.
func TestBuildBriefAdmittedMarksOnlyTheWorkFormsBuildBrief(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	var frozen string
	admitted := []bool{}
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: r.unit,
		GoalID: "goal-a", UnitCommit: r.unit, Repository: r, BriefPath: writeBuildBrief(t), Join: true, BuildBriefSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("Build brief: cites plans/handoff-created-at-runtime.md\n"))),
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, _, _, _, _ string) (string, error) {
			frozen = brief
			admitted = append(admitted, branch.BuildBriefAdmitted(brief))
			if len(admitted) == 1 {
				return "", &branch.ReadNeverLaunchedError{Err: errors.New("refused before launch")}
			}
			writeReadJobWithSubject(t, r.root, "critic-build-brief", r.unit, "running", false, r.readSubject())
			return "critic-build-brief", nil
		},
	}
	if _, err := branch.RunBranchRead(request); err == nil || len(admitted) != 1 {
		t.Fatalf("refused dispatch err=%v admitted=%v", err, admitted)
	}
	request.BriefPath = ""
	if result, err := branch.RunBranchRead(request); err != nil || result.State != "dispatched" {
		t.Fatalf("saved start=%+v err=%v", result, err)
	}
	if len(admitted) != 2 || !admitted[0] || !admitted[1] {
		t.Fatalf("work-form brief admitted=%v", admitted)
	}
	if !strings.Contains(gleBranchReadRecord(t, r.root, r.unit), `"briefFromBuild": true`) {
		t.Fatalf("record=%s", gleBranchReadRecord(t, r.root, r.unit))
	}

	// The same brief and record outside goal-reads are not a goal read's.
	elsewhere := filepath.Join(t.TempDir(), "metasystem", "elsewhere", "goal-a")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(frozen)
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(elsewhere, r.unit+".md")
	record := strings.Replace(gleBranchReadRecord(t, r.root, r.unit), `"brief": "`+frozen+`"`, `"brief": "`+moved+`"`, 1)
	if err := os.WriteFile(moved, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(elsewhere, r.unit+".json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	if branch.BuildBriefAdmitted(moved) {
		t.Fatalf("a brief outside goal-reads was admitted: %s", moved)
	}

	if err := os.WriteFile(frozen, append(body, "Cites plans/another.md\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if branch.BuildBriefAdmitted(frozen) {
		t.Fatalf("a brief changed after recording was admitted: %s", frozen)
	}
}

// TestBuildBriefAdmittedRefusesTheCommitFormsBrief: a brief a person hands to
// the commit form as a correction is never marked, even when it restarts a refused work-form
// start.
func TestBuildBriefAdmittedRefusesTheCommitFormsBrief(t *testing.T) {
	t.Parallel()
	r := newReadFactRepository(t, false)
	r.expectStart()
	r.expectGateAndBrief()
	r.expectStart()
	r.expect(readFactCall{method: "Range", args: []string{r.root, r.base, r.unit, "goal-a"}, commits: r.rangeFacts()})
	admitted := []bool{}
	request := branch.BranchReadRequest{Repo: r.root, Remote: "origin", EndpointTip: r.base, BranchTip: r.unit,
		GoalID: "goal-a", UnitCommit: r.unit, Repository: r, BriefPath: writeBuildBrief(t), Join: true, BuildBriefSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("Build brief: cites plans/handoff-created-at-runtime.md\n"))),
		CheckClaim: claimAllowed, Gate: func(string) (string, error) { return "green", nil },
		Delegate: func(brief, _, _, _, _ string) (string, error) {
			admitted = append(admitted, branch.BuildBriefAdmitted(brief))
			if len(admitted) == 1 {
				return "", &branch.ReadNeverLaunchedError{Err: errors.New("refused before launch")}
			}
			writeReadJobWithSubject(t, r.root, "critic-hand-brief", r.unit, "running", false, r.readSubject())
			return "critic-hand-brief", nil
		},
	}
	if _, err := branch.RunBranchRead(request); err == nil || len(admitted) != 1 {
		t.Fatalf("refused dispatch err=%v admitted=%v", err, admitted)
	}
	hand := filepath.Join(t.TempDir(), "hand-brief.md")
	if err := os.WriteFile(hand, []byte("Hand brief: cites plans/nowhere.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	request.Join, request.BriefPath = false, hand
	if result, err := branch.RunBranchRead(request); err != nil || result.State != "dispatched" {
		t.Fatalf("commit-form start=%+v err=%v", result, err)
	}
	if len(admitted) != 2 || !admitted[0] || admitted[1] {
		t.Fatalf("admitted=%v, want the work-form start marked and the commit form's not", admitted)
	}
	if record := gleBranchReadRecord(t, r.root, r.unit); strings.Contains(record, "briefFromBuild") {
		t.Fatalf("commit-form record=%s", record)
	}
}
