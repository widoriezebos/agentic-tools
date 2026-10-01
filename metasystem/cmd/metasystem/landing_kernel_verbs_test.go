package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

const kernelVerbBatch = "01j5x00000000000000000vb01"

func (bed *kernelBed) runWith(t *testing.T, owners intentOwners, words ...string) (int, string) {
	t.Helper()
	command, ok := findIntentAction(words[0], words[1])
	if !ok {
		t.Fatalf("no public command %s %s", words[0], words[1])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, words[2:], &stdout, &stderr, bed.cwd, owners)
	return code, stdout.String() + stderr.String()
}

// landing begin and landing prove (K4, K6) act on the admitted lane: the
// registered nested layout, the host home and the lane's claim identity
// reach the kernel, and each outcome reads as two plain lines.
func TestLandingBeginAndProveVerbs(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	bed.enroll(t, runningTestBinary(t))
	owners := bed.owners()
	var begun kernel.BeginRequest
	owners.landing.begin = func(request kernel.BeginRequest) (kernel.BeginOutcome, error) {
		begun = request
		return kernel.BeginOutcome{Changed: true, Opening: batch.Opening{Base: "1111111111111111111111111111111111111111",
			Candidate: "2222222222222222222222222222222222222222", Series: make([]batch.SeriesCommit, 2), Deviation: 3}}, nil
	}
	code, text := bed.runWith(t, owners, "landing", "begin", "--batch", kernelVerbBatch, "--members", "goal-a, change:0123456789ab", "--base", "origin/main", "--head", "HEAD")
	if code != 0 || !strings.Contains(text, "recorded batch "+kernelVerbBatch+"'s series of 2 commits") {
		t.Fatalf("landing begin = %d\n%s", code, text)
	}
	if begun.Home != bed.home || string(begun.Layout.Checkout) != bed.checkout || string(begun.Layout.Install) != bed.installation ||
		!strings.HasSuffix(begun.Actor, "+"+lane.ClaimLineage) || strings.Join(begun.Members, ",") != "goal-a,change:0123456789ab" ||
		begun.Base != "origin/main" || begun.Head != "HEAD" {
		t.Fatalf("landing begin asked the kernel %+v; want the admitted nested lane, its home and claim identity", begun)
	}
	if code, text := bed.runWith(t, owners, "landing", "begin", "--batch", kernelVerbBatch, "--base", "origin/main"); code != 2 || !strings.Contains(text, "nothing was recorded") {
		t.Fatalf("landing begin without members = %d\n%s", code, text)
	}

	var proved kernel.ProveRequest
	owners.landing.prove = func(request kernel.ProveRequest) (batch.ProofAttempt, error) {
		proved = request
		return batch.ProofAttempt{ID: "a1", Subject: batch.SubjectMember, Member: "goal-a", Status: batch.AttemptRed, RedGroups: []string{"app-standard"}}, nil
	}
	code, text = bed.runWith(t, owners, "landing", "prove", "--batch", kernelVerbBatch, "--subject", "member:goal-a")
	if code != 1 || !strings.Contains(text, "tests of goal-a on the base of batch "+kernelVerbBatch+" failed: app-standard") ||
		!strings.Contains(text, "landing status --verbose") || proved.Subject != "member:goal-a" || string(proved.Layout.Checkout) != bed.checkout {
		t.Fatalf("a red member = %d (asked %+v)\n%s", code, proved, text)
	}
	owners.landing.prove = func(request kernel.ProveRequest) (batch.ProofAttempt, error) {
		return batch.ProofAttempt{ID: "a2", Subject: batch.SubjectBatch, Status: batch.AttemptUnavailable, Reason: "the test run was refused: the lane cannot be named"}, nil
	}
	code, text = bed.runWith(t, owners, "landing", "prove", "--batch", kernelVerbBatch, "--subject", "batch", "--json")
	var result intentResult
	if err := json.Unmarshal([]byte(text), &result); err != nil || code != 1 || result.Outcome != intentFailed ||
		!strings.Contains(result.Summary, "could not run, which says nothing about the work") || result.Next == nil {
		t.Fatalf("an unavailable run = %d %+v %v\n%s", code, result, err, text)
	}
	owners.landing.prove = func(request kernel.ProveRequest) (batch.ProofAttempt, error) {
		return batch.ProofAttempt{}, &lane.Refusal{Code: lane.CodePaused, Message: "the landing lane is stopped by Wido, so prove was not started",
			Fix: "a person resumes it: metasystem landing start", Argv: []string{"metasystem", "landing", "start"}}
	}
	code, text = bed.runWith(t, owners, "landing", "prove", "--batch", kernelVerbBatch, "--subject", "batch")
	if code != 1 || !strings.Contains(text, "stopped by Wido") || !strings.Contains(text, "landing start") || strings.Contains(text, lane.CodePaused) {
		t.Fatalf("a paused prove = %d\n%s", code, text)
	}

	// The custody barrier (K9) holds a prove while landing work runs: in
	// progress, not failed, and line 2 shows what runs.
	owners.landing.prove = func(request kernel.ProveRequest) (batch.ProofAttempt, error) {
		return batch.ProofAttempt{}, &custody.Held{Live: []string{"validate validate-01 (pid 7)"}}
	}
	code, text = bed.runWith(t, owners, "landing", "prove", "--batch", kernelVerbBatch, "--subject", "batch", "--json")
	result = intentResult{}
	if err := json.Unmarshal([]byte(text), &result); err != nil || code != 1 || result.Outcome != intentInProgress ||
		!strings.Contains(result.Summary, "other landing work still runs") || result.Next == nil || !strings.Contains(strings.Join(result.Details, " "), "validate-01") {
		t.Fatalf("a prove held by live custody = %d %+v %v\n%s", code, result, err, text)
	}
	owners.landing.prove = func(request kernel.ProveRequest) (batch.ProofAttempt, error) {
		return batch.ProofAttempt{}, &custody.Held{Unknown: []string{"prove prove-02: the child can't be probed"}}
	}
	code, text = bed.runWith(t, owners, "landing", "prove", "--batch", kernelVerbBatch, "--subject", "batch", "--json")
	result = intentResult{}
	if err := json.Unmarshal([]byte(text), &result); err != nil || code != 1 || result.Outcome != intentRefused ||
		!strings.Contains(result.Summary, "can't be read") || result.Next == nil {
		t.Fatalf("a prove held by unknown custody = %d %+v %v\n%s", code, result, err, text)
	}

	// The production kernel behind the verb: a batch the lane does not hold
	// is refused in two lines, and nothing is recorded.
	code, text = bed.runWith(t, bed.owners(), "landing", "begin", "--batch", kernelVerbBatch, "--members", "goal-a", "--base", "origin/main", "--head", "HEAD")
	if code != 1 || !strings.Contains(text, "batch "+kernelVerbBatch+" can't be read") || strings.Contains(text, batch.CodeBeginRefused) {
		t.Fatalf("begin of an unknown batch = %d\n%s", code, text)
	}
	code, text = bed.runWith(t, bed.owners(), "landing", "prove", "--batch", kernelVerbBatch, "--subject", "batch")
	if code != 1 || !strings.Contains(text, "can't be read") || strings.Contains(text, kernel.CodeProveRefused) {
		t.Fatalf("prove of an unknown batch = %d\n%s", code, text)
	}
}

// witnessLandingBeginRepeat runs landing begin twice with the production
// kernel on a real nested lane whose enrolled engine is this test binary: a
// batch of one change, composed as its plain replay on landed main. Both
// runs are success; the second records nothing and leaves the batch, the
// candidate ref and the host home as they were.
func witnessLandingBeginRepeat(t *testing.T) {
	bed := newKernelBed(t)
	// The landed ledger's root: begin reads every member's authority there.
	root := goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1})
	if err := os.MkdirAll(filepath.Join(bed.installation, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.installation, "plans", "goals", "backlog.md"), root, 0o644); err != nil {
		t.Fatal(err)
	}
	main := bed.landMain(t)
	bed.enroll(t, runningTestBinary(t))
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", bed.checkout, "-c", "user.name=seat", "-c", "user.email=seat@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("fetch", "--quiet", "origin")
	if err := os.WriteFile(filepath.Join(bed.installation, "change.txt"), []byte("a seat's change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "metasystem/change.txt")
	git("commit", "--quiet", "-m", "record: a seat's change\n\nMachine: m1e+human")
	change := git("rev-parse", "HEAD")
	git("reset", "--quiet", "--hard", main)
	baseTree := git("rev-parse", main+"^{tree}")
	unit := batch.NewChangeUnit(batch.ChangeMember{Commit: change, Parent: main, AskedBy: "m1e+human", Subject: "record: a seat's change"},
		"/seat", "m1e", "human", []string{"metasystem/change.txt"}, nil)
	unit.State = batch.UnitJoined
	store := batch.NewStore(bed.checkout, nil)
	if err := store.Create(batch.Record{Schema: 1, BatchID: kernelVerbBatch, BaseTree: baseTree, TipTree: baseTree, State: batch.StateOpen, Units: []batch.Unit{unit}}); err != nil {
		t.Fatal(err)
	}
	words := []string{"landing", "begin", "--batch", kernelVerbBatch, "--members", unit.GoalID, "--base", main, "--head", change}
	if code, text := bed.runWith(t, bed.owners(), words...); code != 0 || !strings.Contains(text, "recorded batch "+kernelVerbBatch+"'s series of 1 commits") {
		t.Fatalf("first begin = %d\n%s", code, text)
	}
	record, home := idemTreeDigest(t, filepath.Join(bed.checkout, "artifacts")), idemTreeDigest(t, bed.home)
	candidate := git("rev-parse", kernel.CandidateRef(kernelVerbBatch))
	if code, text := bed.runWith(t, bed.owners(), words...); code != 0 || !strings.Contains(text, "already recorded") {
		t.Fatalf("repeated begin = %d\n%s", code, text)
	}
	idemSameTree(t, "a repeated landing begin (batch records)", record, idemTreeDigest(t, filepath.Join(bed.checkout, "artifacts")))
	idemSameTree(t, "a repeated landing begin (home)", home, idemTreeDigest(t, bed.home))
	if again := git("rev-parse", kernel.CandidateRef(kernelVerbBatch)); again != candidate {
		t.Fatalf("a repeated begin moved the candidate %s to %s", candidate, again)
	}
}

// The kernel verbs' layout goldens join G1b through the group hook.
var _ = func() bool {
	layoutGroupCases = append(layoutGroupCases, landingKernelLayoutCases)
	return true
}()

func landingKernelLayoutCases() []layoutCase {
	return []layoutCase{
		{name: "landing-begin", args: []string{"landing", "begin", "--batch", kernelVerbBatch, "--members", "goal-a,change:0123456789ab", "--base", "origin/main", "--head", "HEAD"},
			bed: landingKernelLayoutBed()},
		{name: "landing-prove-red", args: []string{"landing", "prove", "--batch", kernelVerbBatch, "--subject", "member:goal-a"}, bed: landingKernelLayoutBed()},
	}
}

// landingKernelLayoutBed is the running lane's bed whose engine is admitted,
// with a begin that records a two-commit series and a prove whose member
// subject is red.
func landingKernelLayoutBed() func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		bed := landingEngineLayoutBed(false)(t)
		bed.owners.landing.begin = func(kernel.BeginRequest) (kernel.BeginOutcome, error) {
			return kernel.BeginOutcome{Changed: true, Opening: batch.Opening{Base: "1111111111111111111111111111111111111111",
				Candidate: "2222222222222222222222222222222222222222", Series: make([]batch.SeriesCommit, 2), Deviation: 3}}, nil
		}
		bed.owners.landing.prove = func(kernel.ProveRequest) (batch.ProofAttempt, error) {
			return batch.ProofAttempt{ID: "a1", Subject: batch.SubjectMember, Member: "goal-a", Status: batch.AttemptRed, RedGroups: []string{"app-standard"}}, nil
		}
		return bed
	}
}
