package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/laneengine"
)

// renewalEvidence is landing publish's reader of the lane's own begin
// record, with the proof standing in for landing prove's green run of the
// series begin recorded (the test runner is not what this witness drives).
type renewalEvidence struct {
	kernel.PublishEvidence
	policy string
}

func (e renewalEvidence) Proof(id string) (lane.ProofAttempt, error) {
	begun, err := e.Begin(id)
	if err != nil {
		return lane.ProofAttempt{}, err
	}
	return lane.ProofAttempt{Batch: id, Attempt: "a1", Subject: lane.SubjectBatch, Outcome: lane.OutcomeGreen, Base: begun.Base,
		Commit: begun.Head, Tree: begun.Tree, PolicyEngineDigest: e.policy, CandidateEngineDigest: "sha256:" + strings.Repeat("c", 64)}, nil
}

func (renewalEvidence) Verify(lane.ProofAttempt) error { return nil }

// K7 end to end on a real nested lane with its ledger on origin's main: a
// goal joined while the lane was registered once is held at that custody
// epoch; the lane is registered again. landing begin renews the claim to
// the new epoch first, through the lane's own publication boundary, which
// moves main, so it records nothing and says to compose again. Composed on
// the new main, begin records the series with nothing left to renew, and
// landing publish puts it on main.
func TestBeginRenewsAStaleClaimThenBeginAndPublishSucceed(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	now := time.Now().UTC()
	git := func(dir string, args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=lane", "-c", "user.email=lane@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	ledgerConfig := func(dir, machine string) {
		git(dir, "config", "metasystem.goal.machine", machine)
		git(dir, "config", "goal.sync-remote", "origin")
		git(dir, "config", "goal.sync-branch", "refs/heads/main")
	}
	const goalID = "ship-widget"
	write := func(dir, path, content string) {
		t.Helper()
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	intent := "Ship the widget."
	budget := &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 1, ReviewRoundLimit: 2}
	risk := &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "The fixture changes one isolated text file."}
	opened, claimed, approved := now.Add(-30*time.Minute).Format(time.RFC3339), now.Add(-20*time.Minute).Format(time.RFC3339), now.Add(-10*time.Minute).Format(time.RFC3339)
	approval := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAC", "human", "terminal")
	file := &goal.GoalFile{Id: goalID, State: goal.StateClaimed, Tier: 1, Risk: risk, Intent: intent, Origin: goal.OriginMain,
		NextStep: "Land the widget.", OpenedAt: opened, Revision: 3, Budget: budget,
		Approved:       &goal.ApprovalRecord{By: "human:wido", At: approved, Revision: 3, Opid: approval, Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(intent, 1, *budget, risk)},
		Claimed:        &goal.ClaimRecord{Machine: "m9", Lineage: "L1", At: claimed, Revision: 2, AccountingRevision: 2},
		StopCapability: &goal.StopCapability{Generation: 2, Revision: 2, Machine: "m9", ClaimEpoch: 1},
		History: []goal.HistoryLine{{At: opened, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAA", "human", "terminal"), Verb: "open", Actor: "human:wido", Keep: -1},
			{At: claimed, Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAB", "m9", "L1"), Verb: "claim", Actor: "m9+L1", Keep: -1},
			{At: approved, Opid: approval, Verb: "approve", Actor: "human:wido", Keep: -1}}}
	write(bed.installation, "plans/goals/backlog.md", string(goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1})))
	write(bed.installation, "plans/goals/"+goalID+".md", string(goal.RenderFile(file)))
	write(bed.installation, "app/base.txt", "base\n")
	ledgerConfig(bed.checkout, "landing")
	landed := bed.landMain(t)
	bed.enroll(t, runningTestBinary(t))

	// The seat that holds the goal hands it to the lane's claim identity at
	// its first custody epoch, as a join does.
	seat := filepath.Join(filepath.Dir(bed.checkout), "seat")
	origin := filepath.Join(filepath.Dir(bed.checkout), "origin.git")
	git(filepath.Dir(bed.checkout), "clone", "--quiet", "file://"+origin, seat)
	ledgerConfig(seat, "m9")
	git(seat, "update-ref", goal.AcceptedRef, "origin/main")
	plantBatchE2EFenceEngine(t, filepath.Join(seat, "metasystem"))
	const batchID = "01j5x00000000000000000rn01"
	if err := batchowner.LaneForwardHandover(func() (string, error) { return bed.home, nil }, &batchowner.BatchOwnerCalls)(
		batchowner.BatchJoinRequest{SeatRoot: filepath.Join(seat, "metasystem"), LandingRoot: bed.checkout, GoalID: goalID}, batchID,
		batch.Claim{Machine: "m9", Lineage: "L1", Epoch: 1}); err != nil {
		t.Fatalf("the join's handover: %v", err)
	}
	ledgerAt := func(rev string) *goal.GoalFile {
		t.Helper()
		parsed, problems := goal.ParseFile([]byte(git(origin, "show", rev+":metasystem/plans/goals/"+goalID+".md")))
		if len(problems) != 0 {
			t.Fatalf("ledger at %s: %v", rev, problems)
		}
		return parsed
	}
	joined := git(origin, "rev-parse", "refs/heads/main")
	if held := ledgerAt(joined); held.Claimed.Lineage != lane.ClaimLineage || held.StopCapability.ClaimEpoch != 1 || joined == landed {
		t.Fatalf("after the join the ledger shows %+v %+v", held.Claimed, held.StopCapability)
	}
	// The lane is registered again: a new custody epoch.
	registerLane(t, bed.home, filepath.Join(filepath.Dir(bed.checkout), "interim-lane"), "Wido", laneTestNow)
	registerLane(t, bed.home, bed.checkout, "Wido", laneTestNow)
	record, _, err := lane.Read(bed.home)
	if err != nil || record.CustodyEpoch != 3 {
		t.Fatalf("re-registered lane = %+v %v; want custody epoch 3", record, err)
	}

	// The batch, and the agent's series composed on the main it fetched.
	git(bed.checkout, "fetch", "--quiet", "origin")
	git(bed.checkout, "checkout", "--quiet", "--detach", "origin/main")
	write(bed.installation, "app/widget.txt", "widget\n")
	git(bed.checkout, "add", "-A")
	git(bed.checkout, "commit", "--quiet", "-m", "ship the widget")
	build := git(bed.checkout, "rev-parse", "HEAD")
	digest, err := goalbranch.UnitDigest(bed.checkout, build)
	if err != nil {
		t.Fatal(err)
	}
	unit := batch.BindBranchMember(batch.Unit{GoalID: goalID, Chain: build, SeatRoot: filepath.Join(seat, "metasystem"), State: batch.UnitJoined, Approver: "Wido",
		AuthorName: "Wido Riezebos", AuthorEmail: "wido@example.invalid", SelectedGroups: []string{"app-standard"},
		Claim: batch.Claim{Machine: "m9", Lineage: "L1", Epoch: 1, Revision: 2, AccountingRevision: 2}},
		batch.BranchMember{GoalID: goalID, Tip: build, Last: true, Builds: []batch.BranchBuild{{Units: []string{"u1"}, Commit: build, Digest: digest}}})
	baseTree := git(bed.checkout, "rev-parse", joined+"^{tree}")
	if err := batch.NewStore(bed.checkout, nil).Create(batch.Record{Schema: 1, BatchID: batchID, BaseTree: baseTree, TipTree: baseTree, State: batch.StateOpen,
		Units: []batch.Unit{unit}}); err != nil {
		t.Fatal(err)
	}
	git(bed.checkout, "checkout", "--quiet", "-B", "lane/"+batchID, joined)
	git(bed.checkout, "cherry-pick", build)
	head := git(bed.checkout, "rev-parse", "HEAD")

	beginJSON := func(base, head string) (int, intentResult) {
		t.Helper()
		code, stdout, stderr := bed.run(t, "landing", "begin", "--batch", batchID, "--members", goalID, "--base", base, "--head", head, "--json")
		var result intentResult
		if err := json.Unmarshal([]byte(stdout+stderr), &result); err != nil {
			t.Fatalf("landing begin = %d %v\n%s%s", code, err, stdout, stderr)
		}
		return code, result
	}
	code, result := beginJSON(joined, head)
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(strings.Join(result.Details, " "), kernel.CodeBeginBaseMoved) || !strings.Contains(result.Summary, "renewed") {
		t.Fatalf("begin on a stale claim = %d %+v; want the renewal, then a refusal for the moved base", code, result)
	}
	renewed := git(origin, "rev-parse", "refs/heads/main")
	if renewed == joined || git(origin, "rev-parse", renewed+"^") != joined {
		t.Fatalf("main after the renewal is %s; want one ledger commit on %s", renewed, joined)
	}
	if held := ledgerAt(renewed); held.Claimed.Lineage != lane.ClaimLineage || held.StopCapability.ClaimEpoch != 3 || held.Claimed.Revision != 2 {
		t.Fatalf("the renewed claim = %+v %+v; want the lane at epoch 3, the claim's revision kept", held.Claimed, held.StopCapability)
	}
	if stored, err := batch.NewStore(bed.checkout, nil).Load(batchID); err != nil || len(stored.Openings) != 0 {
		t.Fatalf("the refused begin recorded %+v, %v", stored.Openings, err)
	}

	// The agent composes again on the new main.
	git(bed.checkout, "fetch", "--quiet", "origin")
	git(bed.checkout, "checkout", "--quiet", "-B", "lane/"+batchID, renewed)
	git(bed.checkout, "cherry-pick", build)
	head = git(bed.checkout, "rev-parse", "HEAD")
	code, result = beginJSON(renewed, head)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("begin on the new main = %d %+v", code, result)
	}
	if main := git(origin, "rev-parse", "refs/heads/main"); main != renewed {
		t.Fatalf("a begin with nothing to renew moved main to %s", main)
	}

	enrolled, err := laneengine.Enrollment(bed.installation)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := record.Layout()
	if err != nil {
		t.Fatal(err)
	}
	owners := bed.owners()
	owners.landing.evidence = renewalEvidence{PublishEvidence: kernel.PublishEvidence{Layout: layout, Home: bed.home}, policy: enrolled.InstallDigest}
	command, ok := findIntentAction("landing", "publish")
	if !ok {
		t.Fatal("no public command landing publish")
	}
	var stdout, stderr strings.Builder
	if code := runIntentIn(command, []string{"--batch", batchID}, &stdout, &stderr, bed.cwd, owners); code != 0 {
		t.Fatalf("publish after the renewal = %d\n%s%s", code, stdout.String(), stderr.String())
	}
	published := git(origin, "rev-parse", "refs/heads/main")
	if published == renewed || git(origin, "rev-parse", published+"^") != renewed || git(origin, "rev-parse", published+"^{tree}") != git(bed.checkout, "rev-parse", head+"^{tree}") {
		t.Fatalf("main after publish is %s; want the one-commit series on the renewed main %s", published, renewed)
	}
}
