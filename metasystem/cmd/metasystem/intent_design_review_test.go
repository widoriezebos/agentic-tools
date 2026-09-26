package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

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
	b := &designReviewBed{deliveryBed: newDeliveryBed(t)}
	roots, err := project.ResolveRoots(b.install)
	if err != nil {
		t.Fatal(err)
	}
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
		_, result := b.do(append([]string{"review", "design", b.design, "--tool-calls", "30"}, extra...)...)
		return result
	}
	if result := review(); result.Outcome != intentInProgress || b.fresh != 1 {
		t.Fatalf("first examination: %+v", result)
	}
	b.finish("rev1", 1, "completed", map[string]any{"id": "F1", "material": true})
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
	b.finish("rev1-r2", 2, "timeout")
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
	if result = review("--retry", "3"); result.Outcome != intentFailed || !strings.Contains(result.Summary, "nothing was adopted") || len(b.followUps) != 2 {
		t.Fatalf("a foreign operation record: %+v", result)
	}
}

// TestIntentDesignCritiqueAdmission: the first paid critique of an
// approved goal nobody holds claims it through the real claim owner, and
// refuses without starting any critique when that owner refuses.
func TestIntentDesignCritiqueAdmission(t *testing.T) {
	t.Parallel()
	b := newDesignReviewBed(t)
	file := b.goalFile(bedGoal)
	workApprovedBox(file)
	file.State, file.Claimed, file.StopCapability, file.StopFence = goal.StateApproved, nil, nil, nil
	b.addGoal(file)
	_, result := b.do("review", "design", b.design, "--tool-calls", "30")
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
		_, result := b.do(append([]string{"review", "design", b.design, "--tool-calls", "30"}, extra...)...)
		return result
	}
	review()
	b.finish("rev1", 1, "completed", map[string]any{"id": "F1", "material": true})
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
