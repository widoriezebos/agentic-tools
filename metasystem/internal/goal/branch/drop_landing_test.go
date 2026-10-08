package branch_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestPublishedDropLandingGitAdapter(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	f.base = f.commit(t, "metasystem/memory/receipts.log", "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n", "seed receipts")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	u := commitUnit(t, f, "U", "metasystem/unit.txt", "optional\n")
	readUnit(t, f, "U", u)
	inverse, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "U", OpID: "drop-U", Kind: branch.Drop, CheckClaim: claimAllowed, FrozenPatch: []byte{}, BeforeCommit: func(dir, _, _ string) error { git(t, dir, "revert", "--no-commit", u); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	// The inverse immediately before V must stay in U's landing group.
	v := commitUnit(t, f, "V", "metasystem/other.txt", "unrelated\n")
	if _, err := branch.Push(pushRequest(f, "drop-push")); err != nil {
		t.Fatal(err)
	}
	page := goal.GoalFile{Id: "goal-a", State: goal.StateQueued, Intent: "Build the declared requirement", Origin: goal.OriginMain, OpenedAt: "2026-08-20T00:31:00Z", Revision: 1, NextStep: "land through " + u, History: []goal.HistoryLine{{At: "2026-08-20T00:31:00Z", Opid: "01J5X0000000000000000000A0-mac-studio-1a2b3c4d", Verb: "open", Actor: "mac-studio+session-a", Targets: []string{"goal-a"}, Keep: -1}}}
	pendingPage := string(goal.RenderFile(&page))
	page.UnitDrops = []goal.UnitDrop{{Unit: "U", Operation: "drop-U", Loop: "unit-read", Subject: u, Attempt: 1, Covered: []string{u}, Findings: []string{"read:1"}, Commit: inverse, Tree: unitTree(t, f, inverse), Proof: "drop-check", Decisions: strings.Repeat("a", 64), Requirements: strings.Repeat("b", 64), Revision: 1, Actor: "machine+session", Reason: "Optional work removed", At: "2026-08-20T00:31:00Z"}}
	if _, problems := goal.ParseFile(goal.RenderFile(&page)); len(problems) != 0 {
		t.Fatal(problems)
	}
	git(t, f.root, "checkout", "-q", "main")
	f.base = f.commit(t, "metasystem/plans/goals/goal-a.md", string(goal.RenderFile(&page)), "published outcome")
	git(t, f.root, "push", "-q", "origin", "main")
	git(t, f.root, "update-ref", goal.AcceptedRef, f.base)
	fresh := filepath.Join(t.TempDir(), "fresh")
	git(t, f.root, "clone", "-q", f.origin, fresh)
	git(t, fresh, "config", "goal.human.Wido", "Wido <wido@example.invalid>")
	git(t, fresh, "update-ref", goal.AcceptedRef, f.base)
	status, err := branch.InspectStatus(fresh, f.base, v, "goal-a")
	if err != nil || len(status.Units) != 2 || status.Prefix != 1 || status.Units[0].ReadState != "dropped" || status.Units[0].PriorReadState != "read clean" || status.Units[1].ReadState != "built" {
		t.Fatalf("dropped U and unread V: %+v %v", status, err)
	}
	for _, commit := range []string{u, inverse} {
		read, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: fresh, Remote: "origin", EndpointTip: f.base, BranchTip: v, GoalID: "goal-a", UnitCommit: commit, CheckClaim: claimAllowed, Gate: func(string) (string, error) { t.Fatal("a dropped subject asked for a new check"); return "", nil }, Delegate: func(string, string, string, string, string) (string, error) {
			t.Fatal("a dropped subject asked for a read")
			return "", nil
		}})
		if err != nil || read.State != "dropped" || read.GateRunID != "drop-check" {
			t.Fatalf("read exemption: %+v %v", read, err)
		}
	}
	gate, err := branch.ResolveReadGate(branch.ReadGateRequest{Repo: fresh, GoalID: "goal-a", UnitCommit: inverse, Gate: func(string) (string, error) { t.Fatal("the inverse was checked again"); return "", nil }})
	if err != nil || gate.RunID != "drop-check" {
		t.Fatalf("inverse gate: %+v %v", gate, err)
	}
	req := branch.LandRequest{Repo: fresh, Remote: "origin", EndpointTip: f.base, BranchTip: v, GoalID: "goal-a", Last: true, LandingReady: true, CandidateOnly: true, GoalPage: string(goal.RenderFile(&page)), ApprovedBy: "human:Wido", Seat: "seat", CheckClaim: claimAllowed}
	if _, err := branch.PrepareLanding(req); err == nil || !strings.Contains(err.Error(), "isn't reviewed") {
		t.Fatalf("unread V admitted: %v", err)
	}
	req.GoalPage, req.ReadsWaived = pendingPage, true
	if _, err := branch.PrepareLanding(req); err == nil || !strings.Contains(err.Error(), "pending drop") || !strings.Contains(err.Error(), "work status goal-a --work U") {
		t.Fatalf("pending drop remedy: %v", err)
	}
	req.GoalPage, req.ReadsWaived = string(goal.RenderFile(&page)), false
	req.Last, req.Through = false, u
	partial, err := branch.PrepareLanding(req)
	if err != nil {
		t.Fatal(err)
	}
	req.CandidateOnly = false
	req.Out, req.TestReceipt = filepath.Join(t.TempDir(), "partial"), filepath.Join(t.TempDir(), "receipt.json")
	writeLandingReceipt(t, req.TestReceipt, partial.Candidate, "drop-check")
	landed, err := branch.PrepareLanding(req)
	if err != nil {
		t.Fatal(err)
	}
	message := git(t, fresh, "show", "-s", "--format=%B", landed.Landing)
	if !strings.Contains(message, "Goal-Drop: goal-a/U drop-U") || strings.Contains(message, "Goal-Unit:") || !strings.Contains(message, "Goal-Source: "+inverse) {
		t.Fatalf("U must land as dropped with its inverse: %s", message)
	}
	if paths := git(t, fresh, "diff", "--name-only", f.base, landed.Landing, "--", "metasystem/unit.txt", "metasystem/other.txt"); paths != "" {
		t.Fatal("partial drop changed main or included V")
	}
	count, err := plain.UnitsOnMain(fresh, landed.Landing, "goal-a")
	if err != nil || count != 0 {
		t.Fatalf("U counted as landed: %d %v", count, err)
	}
	if _, err := branch.VerifyLanded(fresh, landed.Landing); err != nil {
		t.Fatal(err)
	}
	paths, err := branch.LandingChangeSet(fresh, f.base, v, "goal-a", "U")
	if err != nil || !strings.Contains(strings.Join(paths, " "), "metasystem/unit.txt") || strings.Contains(strings.Join(paths, " "), "other.txt") {
		t.Fatalf("drop change set: %v %v", paths, err)
	}
	git(t, f.root, "checkout", "-q", "goal/goal-a")
	readUnit(t, f, "V", v)
	if _, err := branch.Push(pushRequest(f, "V-read-push")); err != nil {
		t.Fatal(err)
	}
	req.BranchTip = git(t, f.root, "rev-parse", "HEAD")
	git(t, fresh, "fetch", "-q", "origin")
	req.Last, req.Through, req.CandidateOnly = true, "", true
	full, err := branch.PrepareLanding(req)
	if err != nil {
		t.Fatal(err)
	}
	req.CandidateOnly = false
	req.Out, req.TestReceipt = filepath.Join(t.TempDir(), "full"), filepath.Join(t.TempDir(), "full-receipt.json")
	writeLandingReceipt(t, req.TestReceipt, full.Candidate, "full-check")
	full, err = branch.PrepareLanding(req)
	if err != nil {
		t.Fatal(err)
	}
	commits := strings.Fields(git(t, fresh, "rev-list", "--reverse", f.base+".."+full.Landing))
	if len(commits) != 2 {
		t.Fatalf("landing groups: %v", commits)
	}
	um := git(t, fresh, "show", "-s", "--format=%B", commits[0])
	vm := git(t, fresh, "show", "-s", "--format=%B", commits[1])
	if !strings.Contains(um, "Goal-Source: "+inverse) || strings.Contains(vm, "Goal-Source: "+inverse) || !strings.Contains(vm, "Goal-Unit: goal-a/V") {
		t.Fatalf("inverse folded into V instead of U: U=%s V=%s", um, vm)
	}
	count, err = plain.UnitsOnMain(fresh, full.Landing, "goal-a")
	if err != nil || count != 1 {
		t.Fatalf("only V counts as landed: %d %v", count, err)
	}
	if _, err := branch.VerifyLandedSeries(fresh, full.Landing); err != nil {
		t.Fatal(err)
	}

}
