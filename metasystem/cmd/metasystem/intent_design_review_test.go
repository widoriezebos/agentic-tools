package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

type designReviewGit struct{ root string }

func (g designReviewGit) Run(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
	if len(args) == 2 && args[0] == "rev-parse" && args[1] == "--show-toplevel" {
		return []byte(g.root + "\n"), nil, nil
	}
	return nil, nil, fmt.Errorf("unexpected Git query: %v", args)
}

// designReviewBed is a delivery bed with one draft design and a fake
// delegate boundary that records design-critic rounds the way dispatch does:
// a fresh root naming the design, and follow-up rounds carrying the
// operation id they were asked for.
type designReviewBed struct {
	*deliveryBed
	design    string
	followUps [][]string
	fresh     int
	lose      bool
}

func newDesignReviewBed(t *testing.T) *designReviewBed {
	return newDesignReviewBedAmended(t, nil)
}

// newDesignReviewBedAmended is newDesignReviewBed over an amended goal file.
func newDesignReviewBedAmended(t *testing.T, amend func(*goal.GoalFile)) *designReviewBed {
	b := &designReviewBed{deliveryBed: &deliveryBed{intentBed: newIntentBed(t, false, amend)}}
	if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	layout, err := b.intentBed.owners().resolver.ResolveLayout(b.root())
	if err != nil {
		t.Fatal(err)
	}
	b.install = layout.InstallationRoot.Path()
	state, err := b.intentBed.owners().resolver.RootForInstallation(layout.InstallationRoot)
	if err != nil {
		t.Fatal(err)
	}
	roots := project.Roots{Checkout: layout.GitRoot, Installation: layout.InstallationRoot, StateRoot: state}
	b.owners = &intentDeliveryOwners{
		draftPaths:   func([]byte, string) ([]string, error) { return nil, nil },
		recordWriter: humanRecordWriter,
		process: func(process intentProcess) intentProcessResult {
			b.calls = append(b.calls, process.argv)
			return b.handler(process)
		},
		closeOwner: func(root string, args []string) intentProcessResult {
			return b.handler(intentProcess{argv: append([]string{"close-owner"}, args...), dir: root})
		},
		executable: func() (string, error) { return "/fake/bin/metasystem", nil },
		now:        func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) },
		// Claim admission reads lane membership; this fixture has no lane.
		laneRoot: func(string, time.Time) (string, bool, error) { return "", false, nil },
	}
	b.owners.calls = processBackedOwnerCalls(b.owners.executable, b.owners.process)
	var home string
	for _, candidate := range project.Homes(roots) {
		if candidate.Kind == project.KindDesign && candidate.Glob == "" {
			home = candidate.Path
		}
	}
	b.design = filepath.Join(home, "reader.md")
	b.writeFile(b.design, "# Reader\n\n- Kind: design\n- Id: 01DESIGNREADER\n- Status: draft\n- Goals: standing-validation\n\nFirst version.\n")
	canonical, _ := filepath.EvalSymlinks(b.design)
	b.handler = func(process intentProcess) intentProcessResult {
		argv := process.argv
		if root := flagValue(argv, "--follow-up"); root != "" {
			b.followUps = append(b.followUps, argv)
			op := flagValue(argv, "--op")
			child := root + "-r" + string(rune('1'+len(b.followUps)))
			b.writeJob(map[string]any{"jobId": child, "role": "design-critic", "status": "running", "round": len(b.followUps) + 1,
				"parentJob": root, "operationId": op, "goalId": "standing-validation", "design": canonical})
			if b.lose {
				return intentProcessResult{stdout: []byte("lost"), code: 1}
			}
			return intentProcessResult{stdout: []byte(`{"outcome":"WON","jobId":"` + child + `"}`)}
		}
		if _, err := os.Stat(filepath.Join(b.install, "artifacts", "agents", "jobs", "rev1.json")); err == nil {
			// The same dispatch identity replays its job.
			return intentProcessResult{stdout: []byte(`{"outcome":"REPLAYED-COMPLETED","jobId":"rev1"}`)}
		}
		b.fresh++
		b.writeJob(map[string]any{"jobId": "rev1", "role": "design-critic", "status": "running", "round": 1, "goalId": "standing-validation", "design": canonical})
		return intentProcessResult{stdout: []byte(`{"outcome":"WON","jobId":"rev1"}`)}
	}
	return b
}

// Decision 2 admits ordinary and critical findings under the same frozen cap.
func TestSecondDesignRoundRefusedWithoutCritical(t *testing.T) {
	t.Parallel()
	for _, critical := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "critical"}[critical], func(t *testing.T) {
			t.Parallel()
			b := newDesignLoopBed(t)
			b.review()
			f := finding("F1", true, "a gap")
			if critical {
				f["severity"] = "critical"
			}
			b.finish("rev1", 1, "completed", f)
			b.register(1, 20, []int64{1}, map[string]any{"findingId": "F1"})
			answer := b.decide(b.review(), map[string]string{"F1": "accepted | specified the gap | section 2"})
			b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "First version.", "Second version.", 1))
			result := b.review("--dispositions", answer, "--after", "1")
			if result.Outcome != intentInProgress || len(b.followUps) != 1 || b.closes != 0 || strings.Contains(result.Summary, "final round") {
				t.Fatalf("severity changed the four-examination allowance: %+v", result)
			}
		})
	}
}

// Decision 2 removes the one-examination fold for every severity.
func TestOneRoundCloseFoldsAccepted(t *testing.T) {
	t.Parallel()
	for _, rigor := range []string{"unproven", "severe"} {
		t.Run(rigor, func(t *testing.T) {
			t.Parallel()
			b := newDesignLoopBed(t)
			b.review()
			b.finish("rev1", 1, "completed", finding("F1", true, "a material gap"))
			b.register(1, 20, []int64{1}, map[string]any{"findingId": "F1", "rigorClass": rigor})
			answer := b.decide(b.review(), map[string]string{"F1": "accepted | specified the gap | section 2"})
			result := b.review("--dispositions", answer)
			entry := b.job("rev1")["findingRegister"].([]any)[0].(map[string]any)
			if result.Outcome != intentRefused || entry["status"] != "open" || b.closes != 0 || len(b.followUps) != 0 || result.Next == nil || !strings.Contains(result.Next.Reason, "after changing the design") {
				t.Fatalf("unexamined amendment folded: %+v entry=%v", result, entry)
			}
		})
	}
}

func TestDesignReviewRetriesFailedFirstExamination(t *testing.T) {
	t.Parallel()
	b := newDesignLoopBed(t)
	b.writeFile(filepath.Join(b.install, "metasystem.conf"), "evidence.root="+t.TempDir()+"\n")
	realCloseOwner(t, b.deliveryBed, func(ports *delegation.Ports) { ports.Git = designReviewGit{b.root()} })
	close := b.owners.closeOwner
	b.owners.closeOwner = func(root string, args []string) intentProcessResult {
		b.closes++
		return close(root, args)
	}
	b.review()
	b.finish("rev1", 1, "failed")
	b.register(1, 20, nil)
	dispatch := b.handler
	b.handler = func(p intentProcess) intentProcessResult {
		if root := flagValue(p.argv, "--follow-up"); root != "" {
			// Decision 3 reserves the retry before dispatch admits that source.
			if err := dispatchcore.ReservedUnknownExaminationRetry(b.install, root); err != nil {
				return intentProcessResult{stderr: []byte(err.Error()), code: 1}
			}
			if _, err := dispatchcore.CritiqueExhaustionAdvance(b.install, root, "design-critic", flagValue(p.argv, "--brief"), "child"); err != nil {
				return intentProcessResult{stderr: []byte(err.Error()), code: 1}
			}
		}
		return dispatch(p)
	}
	if result := b.review("--retry", "1"); result.Outcome != intentInProgress || len(b.followUps) != 1 {
		t.Fatalf("retry of the failed first examination: %+v", result)
	}
	b.finish("rev1-r2", 2, "completed", finding("F1", true, "a material gap"))
	b.register(2, 20, []int64{0, 1}, map[string]any{"findingId": "F1", "critic": "rev1-r2", "rigorClass": "unproven"})
	if limit := dispatchcore.DesignRoundLimit(b.install, "rev1", 20); limit != 4 {
		t.Fatalf("first returned examination cap = %d, want 4", limit)
	}
	snapshot := "artifacts/agents/capabilities/close.json"
	b.writeFile(filepath.Join(b.install, snapshot), `{"ok":true}`)
	for _, job := range []string{"rev1", "rev1-r2"} {
		record := b.job(job)
		record["capabilitySnapshot"] = snapshot
		b.writeJob(record)
	}
	decided := b.decide(b.review(), map[string]string{"F1": "accepted | folded the gap | section 2"})
	answer := filepath.Join(b.root(), "retry-decisions.md")
	if err := os.Rename(decided, answer); err != nil {
		t.Fatal(err)
	}
	// Decision 2: the failed execution consumes no completed examination;
	// its first material return needs a revision, rather than a legacy fold.
	b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "First version.", "Second version.", 1))
	result := b.review("--dispositions", answer, "--after", "2")
	entry := b.job("rev1")["findingRegister"].([]any)[0].(map[string]any)
	if result.Outcome != intentInProgress || b.closes != 0 || len(b.followUps) != 2 || b.job("rev1")["chainClosed"] == true || entry["status"] != "open" || strings.Contains(result.Summary, "final round") {
		t.Fatalf("retry bought a fold or spent the four-examination allowance: %+v entry=%v", result, entry)
	}
	if clean, err := readsubject.CleanRegister(b.job("rev1")["findingRegister"]); err != nil || clean {
		t.Fatalf("unexamined amendment register = clean %v, error %v", clean, err)
	}
	if saved := string(mustRead(t, decided)); !strings.Contains(saved, "| F1 | accepted |") || strings.Contains(string(mustRead(t, b.design)), "## Dispositions (critique rev1)\n") {
		t.Fatal("the round lost its decisions or the unclosed page claimed a close")
	}
}

func (b *designReviewBed) finish(job string, round int, status string, findings ...map[string]any) {
	record := b.job(job)
	record["status"] = status
	b.writeJob(record)
	if status == "completed" {
		b.writeReturn("rev1", round, job, findings...)
	}
}

// TestDesignCritiqueReplayAndCap: a changed design continues its one
// critique chain only with the author's bound decisions; the follow-up is
// requested once under its frozen operation, a replay rejoins the retained
// child without another request, a lost response is recovered from the
// operation's own record, a record of another chain is never adopted, no
// fresh root is bought for the same design, and a failed examination is
// retried once and rejoined.
func TestDesignCritiqueReplayAndCap(t *testing.T) {
	t.Parallel()
	b := newDesignReviewBed(t)
	review := func(extra ...string) intentResult {
		t.Helper()
		_, result := b.do(append([]string{"design", "review", b.design, "--tool-calls", "30"}, extra...)...)
		return result
	}
	if result := review(); result.Outcome != intentInProgress || b.fresh != 1 {
		t.Fatalf("first examination: %+v", result)
	}
	b.finish("rev1", 1, "completed", map[string]any{"id": "F1", "severity": "critical", "material": true})
	if result := review(); result.Outcome != intentConfirmed || b.fresh != 1 {
		t.Fatalf("the first examination's findings: %+v fresh=%d", result, b.fresh)
	}
	// The author changes the design: no fresh root, the bound template.
	b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "First version.", "Second version, F1 addressed.", 1))
	result := review()
	template, _ := result.Data.(map[string]any)["template"].(string)
	body := string(mustRead(t, template))
	if result.Outcome != intentInProgress || b.fresh != 1 || len(b.followUps) != 0 || !strings.Contains(body, "work=design:01DESIGNREADER attempt=1") {
		t.Fatalf("changed design without decisions: %+v\n%s", result, body)
	}
	decided := filepath.Join(b.root(), "decided.md")
	b.writeFile(decided, strings.Replace(body, "| F1 | DECIDE | | |", "| F1 | accepted | the design now covers it | section 2 |", 1))
	if result = review("--dispositions", decided); b.fresh != 1 || len(b.followUps) != 1 || flagValue(b.followUps[0], "--follow-up") != "rev1" {
		t.Fatalf("continuation: %+v followUps=%v", result, b.followUps)
	}
	operation := flagValue(b.followUps[0], "--op")
	if brief := string(mustRead(t, flagValue(b.followUps[0], "--brief"))); !strings.Contains(brief, "| F1 | accepted |") || !strings.Contains(brief, "decisions on examination 1") {
		t.Fatalf("the follow-up brief lacks the decisions: %s", brief)
	}
	// Replay after completion, and after a later goal revision: the same
	// retained child, no new request.
	b.finish("rev1-r2", 2, "completed")
	for range 2 {
		if result = review("--dispositions", decided); len(b.followUps) != 1 || b.fresh != 1 || !strings.Contains(result.Summary, "rev1-r2") {
			t.Fatalf("replay of the continuation: %+v", result)
		}
	}
	if operation == "" || !strings.HasPrefix(operation, "design-01designreader-after-1-") {
		t.Fatalf("frozen operation id: %q", operation)
	}
	// A failed examination: retried once; a lost response is recovered from
	// the operation's own record; the replay rejoins even after it failed.
	// Decision 3 permits one failed-execution retry; deadlines never retry.
	b.finish("rev1-r2", 2, "failed")
	if err := os.Remove(filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", "2", "return.json")); err != nil {
		t.Fatal(err)
	}
	b.lose = true
	if result = review("--retry", "2"); len(b.followUps) != 2 {
		t.Fatalf("retry request: %+v", result)
	}
	b.lose = false
	if result = review("--retry", "2"); len(b.followUps) != 2 || !strings.Contains(result.Summary, "rev1-r3") {
		t.Fatalf("lost response recovered from the operation record: %+v followUps=%d", result, len(b.followUps))
	}
	b.finish("rev1-r3", 3, "failed")
	if result = review("--retry", "2"); len(b.followUps) != 2 || !strings.Contains(result.Summary, "rev1-r3") {
		t.Fatalf("a repeated retry after the retry failed: %+v", result)
	}
	if result = review("--retry", "1"); result.Outcome != intentRefused || len(b.followUps) != 2 {
		t.Fatalf("a retry of an older examination: %+v", result)
	}
	// A record carrying a continuation's operation on another chain is
	// never adopted.
	b.writeJob(map[string]any{"jobId": "other-r2", "role": "design-critic", "status": "completed", "round": 2, "parentJob": "other", "operationId": "design-01designreader-retry-3"})
	b.writeJob(map[string]any{"jobId": "other", "role": "code-critic", "status": "completed", "round": 1})
	// Decision 3 stops before any third-execution operation can be adopted.
	if result = review("--retry", "3"); result.Outcome != intentFailed || !strings.Contains(result.Summary, "stopped") || len(b.followUps) != 2 || b.job("rev1")["unknownExaminationRetryFrom"] != "rev1-r2" {
		t.Fatalf("a foreign operation record: %+v", result)
	}
}

// TestDesignCritiqueRejoinKeepsTheBrief: a second Send while the
// examination runs rejoins it and writes nothing, so the brief that states
// the running read's budget, which a follow-up is built from, keeps its bytes
// even when the second Send names another reader budget.
func TestDesignCritiqueRejoinKeepsTheBrief(t *testing.T) {
	t.Parallel()
	b := newDesignReviewBed(t)
	if _, result := b.do("design", "review", b.design, "--tool-calls", "30"); result.Outcome != intentInProgress || b.fresh != 1 {
		t.Fatalf("first examination: %+v", result)
	}
	briefs, _ := filepath.Glob(filepath.Join(b.install, "artifacts", "agents", "intent-review", "design-01designreader-*", "brief.md"))
	if len(briefs) != 1 {
		t.Fatalf("the admitted brief: %v", briefs)
	}
	admitted := string(mustRead(t, briefs[0]))
	_, result := b.do("design", "review", b.design, "--tool-calls", "45")
	if result.Outcome != intentInProgress || b.fresh != 1 {
		t.Fatalf("the rejoin: %+v fresh=%d", result, b.fresh)
	}
	if after := string(mustRead(t, briefs[0])); after != admitted {
		t.Fatalf("the rejoin rewrote the admitted brief:\n--- admitted\n%s\n--- after\n%s", admitted, after)
	}
}

// TestIntentDesignCritiqueAdmission: the first paid critique of an
// approved goal nobody holds claims it through the real claim owner, and
// refuses without starting any critique when that owner refuses.
func TestIntentDesignCritiqueAdmission(t *testing.T) {
	t.Parallel()
	b := newDesignReviewBed(t)
	b.lineage = "m1"
	b.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, nil }
	file := b.goalFile(bedGoal)
	workApprovedBox(file)
	file.State, file.Claimed, file.StopCapability, file.StopFence = goal.StateApproved, nil, nil, nil
	b.addGoal(file)
	_, result := b.do("design", "review", b.design, "--tool-calls", "30")
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "claims goal standing-validation first") || b.fresh != 0 {
		t.Fatalf("an unclaimed goal without a lease holder: %+v fresh=%d", result, b.fresh)
	}
	after := b.goalFile(bedGoal)
	if after.State != goal.StateApproved || after.Claimed != nil {
		t.Fatalf("a refused critique claim changed the goal: %+v", after.Claimed)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestDesignCritiqueClosesOnUnchangedDesign: decisions that answer the
// design version still in place are judged by review design --dispositions
// itself: an accepted material finding on the unchanged design is refused,
// and nothing is closed or requested.
func TestDesignCritiqueClosesOnUnchangedDesign(t *testing.T) {
	t.Parallel()
	b := newDesignReviewBed(t)
	b.owners.recordWriter = humanRecordWriter
	dispatch := b.handler
	var closes [][]string
	b.handler = func(process intentProcess) intentProcessResult {
		if len(process.argv) > 1 && process.argv[1] == "close" {
			closes = append(closes, process.argv)
			record := b.job(flagValue(process.argv, "--job"))
			record["chainClosed"] = true
			b.writeJob(record)
			return intentProcessResult{}
		}
		return dispatch(process)
	}
	review := func(extra ...string) intentResult {
		t.Helper()
		_, result := b.do(append([]string{"design", "review", b.design, "--tool-calls", "30"}, extra...)...)
		return result
	}
	review()
	b.finish("rev1", 1, "completed", map[string]any{"id": "F1", "severity": "critical", "material": true})
	review()
	first := string(mustRead(t, b.design))
	b.writeFile(b.design, strings.Replace(first, "First version.", "Second version.", 1))
	template, _ := review().Data.(map[string]any)["template"].(string)
	body := string(mustRead(t, template))
	b.writeFile(b.design, first)
	accepted := filepath.Join(b.root(), "accepted.md")
	b.writeFile(accepted, strings.Replace(body, "| F1 | DECIDE | | |", "| F1 | accepted | a real gap | section 2 |", 1))
	if result := review("--dispositions", accepted); result.Outcome != intentRefused || len(closes) != 0 || len(b.followUps) != 0 || !strings.Contains(result.Summary, "F1") {
		t.Fatalf("an accepted material finding on the unchanged design: %+v closes=%d followUps=%d", result, len(closes), len(b.followUps))
	}
}

// TestDesignReviewBriefCarriesTheGoalsRounds: a design review's round budget
// is the goal's review-round member, as a code review's is; no fixed five
// clamps it (Wido 2026-10-02).
func TestDesignReviewBriefCarriesTheGoalsRounds(t *testing.T) {
	t.Parallel()
	b := newDesignReviewBedAmended(t, func(file *goal.GoalFile) {
		if file.Budget == nil {
			t.Fatal("the bed goal has no budget to amend")
		}
		file.Budget.ReviewRoundLimit = 12
		if file.Approved != nil {
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		}
	})
	_, result := b.do("design", "review", b.design, "--tool-calls", "30")
	if result.Outcome != intentInProgress {
		t.Fatalf("design review: %+v", result)
	}
	briefs, _ := filepath.Glob(filepath.Join(b.install, "artifacts", "agents", "intent-review", "design-*", "brief.md"))
	if len(briefs) != 1 {
		t.Fatalf("design review briefs = %v, want one", briefs)
	}
	if brief := string(mustRead(t, briefs[0])); !strings.Contains(brief, "Round budget: 12 focused rounds") {
		t.Fatalf("the brief does not carry the goal's 12 rounds:\n%s", brief)
	}
}
