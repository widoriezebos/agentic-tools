package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// reviewStartRow is one way the start of a review can be refused before
// any critic is dispatched. block puts the cause in place, clear removes it
// the way a person would; cause is what the refusal's first line must name.
type reviewStartRow struct {
	name  string
	cite  string
	setup func(c *connectionBed)
	block *delegateOutcome
	cause string
	clear func(c *connectionBed)
	// corrected: the remedy is a corrected brief, so the request after the
	// cause is removed names that brief (the review with another brief).
	corrected bool
	// admitted: nothing refuses; the first request goes through.
	admitted bool
	// workAdmitted: the review of the work starts. Its critic's brief carries
	// the build brief, which the build ran with, so the paths it cites are
	// not checked again. A person's brief to the review of a commit still is.
	workAdmitted bool
	// fromPrimary: the critic is dispatched from the seat's checkout, whose
	// HEAD is main, not from the goal worktree.
	fromPrimary bool
	// edits are the unit's files besides connect.txt.
	edits map[string]string
}

// reviewStartDelegate is the delegate boundary of the matrix: every
// dispatch runs the real brief-authority admission against the tree and
// disk the critic would get (the goal worktree), then the row's refusal,
// if any is in place, as the delegate reports it; otherwise it starts one
// critic. It goes through readDelegate, so outcomes are classified as in
// production. The admission is the delegate's own call: the brief is read
// against the commit under review and the dispatching checkout's HEAD,
// except a goal read's brief recorded as carrying the build brief, whose
// headers and bounds alone are checked.
func reviewStartDelegate(c *connectionBed, install string) func(string, string, string, string, string) (string, error) {
	return func(brief, goalID, commit, runtime, model string) (string, error) {
		caller := func(_ delegateRequest, stdout, _ io.Writer) int {
			emit := func(outcome delegateOutcome) {
				data, _ := json.Marshal(outcome)
				fmt.Fprintln(stdout, string(data))
			}
			base := install
			if c.fromPrimary {
				base = c.root()
			}
			tree := base
			if branch.BuildBriefAdmitted(brief) {
				tree = ""
			}
			if _, err := dispatchcore.ReadReviewBriefAdmission(brief, base, tree, base, "commit:"+commit); err != nil {
				emit(delegateOutcome{Outcome: "REFUSED-INTERNAL", Headline: "refused", Detail: "brief authority admission refused: " + err.Error()})
				return 1
			}
			c.mu.Lock()
			blocked := c.refusal
			c.mu.Unlock()
			if blocked != nil {
				emit(*blocked)
				return 1
			}
			c.mu.Lock()
			job := fmt.Sprintf("crit%d", len(c.delegates)+1)
			c.delegates = append(c.delegates, commit)
			c.briefs = append(c.briefs, string(mustRead(c.t, brief)))
			c.mu.Unlock()
			c.writeCritic(install, job, commit, "running", false)
			emit(delegateOutcome{Outcome: "WON", Headline: "started", JobID: job})
			return 0
		}
		return readDelegate(caller, install, brief, goalID, commit, runtime, model)
	}
}

func reviewStartRows() []reviewStartRow {
	refused := func(outcome, detail string) *delegateOutcome {
		return &delegateOutcome{Outcome: outcome, Headline: "refused", Detail: detail}
	}
	budget := strings.Join(dispatchcore.FormatGoalAdmission(dispatchcore.GoalAdmissionVerdict{Refusals: []dispatchcore.GoalAdmissionRefusal{{
		GoalID: "standing-validation", GoalRevision: 1, Breaches: []dispatchcore.BudgetBreach{{Field: "activeJobLimit", Used: "1", Limit: "1"}}}}}), "\n")
	unblock := func(c *connectionBed) { c.mu.Lock(); c.refusal = nil; c.mu.Unlock() }
	return []reviewStartRow{
		// The review of the work starts: the cited file is the build brief's,
		// written by the feature at runtime.
		{name: "ignored-path", cite: "plans/scratch/notes.md", cause: "plans/scratch/notes.md", workAdmitted: true,
			setup: func(c *connectionBed) {
				c.commitToMain(map[string]string{".gitignore": "artifacts/\n.claude/settings.local.json\nmetasystem.conf.local\nplans/scratch/\n", "plans/README.md": "plans\n"})
			},
			// The runtime file appears where the critic runs: the goal
			// worktree. Git ignores it, so no tree can ever hold it.
			clear: func(c *connectionBed) {
				c.writeFile(filepath.Join(c.worktree, "plans", "scratch", "notes.md"), "runtime notes\n")
			}},
		// The review of the work starts: the cited file is the build brief's.
		{name: "untracked-seat-file", cite: "plans/trace.md", cause: "plans/trace.md", workAdmitted: true,
			setup: func(c *connectionBed) { c.commitToMain(map[string]string{"plans/README.md": "plans\n"}) },
			// The seat writes the file in its own checkout and does not
			// commit it.
			clear: func(c *connectionBed) {
				c.writeFile(filepath.Join(c.root(), "plans", "trace.md"), "the seat's trace\n")
			}},
		// The review of the work starts: the cited file is the build brief's,
		// so only the review of a commit needs a corrected brief.
		{name: "corrected-brief", cite: "plans/never.md", cause: "plans/never.md", corrected: true, workAdmitted: true,
			setup: func(c *connectionBed) { c.commitToMain(map[string]string{"plans/README.md": "plans\n"}) },
			clear: func(c *connectionBed) {}},
		// A file new in the unit under review, absent from main, cited by a
		// critic dispatched from the seat's checkout (HEAD main): the
		// critic reads the unit commit, so it is admitted.
		{name: "unit-only-file", cite: "plans/unit-notes.md", admitted: true, fromPrimary: true,
			setup: func(c *connectionBed) { c.commitToMain(map[string]string{"plans/README.md": "plans\n"}) },
			edits: map[string]string{"plans/unit-notes.md": "notes the unit adds\n"}},
		{name: "budget-full", block: refused("REFUSED-BUDGET", budget), cause: "activeJobLimit", clear: unblock},
		{name: "tier-refuses-critic", block: refused("REFUSED-INTERNAL", "tier 1 goal standing-validation refuses critic roles and --reviews; raise it first with goal edit --tier 2"),
			cause: "refuses critic roles", clear: unblock},
		{name: "claim-stale", block: refused("REFUSED-INTERNAL", "goal standing-validation revision 1 was bound under claim 3, but the goal's current claim is 4\nnothing was dispatched; dispatch again under the current claim"),
			cause: "bound under claim 3", clear: unblock},
		{name: "census-absent", block: refused("REFUSED-INTERNAL", "dispatch refused: census verdict is absent; run metasystem system start --repo /seat"),
			cause: "census verdict is absent", clear: unblock},
		{name: "holder-busy", block: refused("REFUSED-INTERNAL", "goal standing-validation revision 1 is busy: pid 4242 holds it (waited 30s)\nnothing was done; run the same command again once it is released"),
			cause: "is busy: pid 4242 holds it", clear: unblock},
		{name: "engine-stale", block: refused("REFUSED-INTERNAL", "dispatch refused: the engine (abc1234) is older than this checkout (def5678), and engine scripts changed\nrebuild with go run ./cmd/devgate build, then arm the steward again"),
			cause: "the engine (abc1234) is older than this checkout", clear: unblock},
	}
}

// TestReviewStartMatrix holds the class of defects in starting a review of
// built work: every admission refusal names its cause and a next step;
// nothing is bound by a refused admission, so once the cause is removed the
// same request (or, for a refused brief, a corrected one) goes through; and
// a request that really dispatched stays bound to its brief. Each row runs
// through the public review of a work item from the seat's checkout, and
// through the review of a commit from the seat's checkout and from the
// goal worktree.
//
// Not parallel: the connection bed isolates Git with t.Setenv.
func TestReviewStartMatrix(t *testing.T) {
	for _, row := range reviewStartRows() {
		for _, via := range []string{"work", "commit", "commit-in-worktree"} {
			t.Run(row.name+"/"+via, func(t *testing.T) { runReviewStartRow(t, row, via) })
		}
	}
}

func runReviewStartRow(t *testing.T, row reviewStartRow, via string) {
	c := newConnectionBed(t)
	c.dispatcher = reviewStartDelegate
	c.fromPrimary = row.fromPrimary
	if row.setup != nil {
		row.setup(c)
	}
	text := "Working Mode: implement\n\nBuild the connection.\n"
	if row.cite != "" {
		text += "Read `" + row.cite + "` first.\n"
	}
	brief := c.brief("brief.md", text)
	c.edits = map[string]string{"connect.txt": "the built result\n"}
	for path, content := range row.edits {
		c.edits[path] = content
	}
	if code, result := c.do(append([]string{"work", "build", c.id, "connect", "--brief", brief, "--lines", "10"}, workCheck...)...); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("build: code=%d %+v", code, result)
	}
	work := []string{"work", "review", c.id, "--work", "connect"}
	var request []string
	if via == "work" {
		request = work
	} else {
		// The unit is committed and published by the work's review, whose
		// read owner is not reached, so no read is requested yet.
		c.skipRead = true
		c.do(work...)
		c.skipRead = false
		subjects := c.runRecord(c.runID(t)).Subjects
		if len(subjects) != 1 || subjects[0].Commit == "" {
			t.Fatalf("no unit commit to review: %+v", subjects)
		}
		request = []string{"work", "review", "--commit", subjects[0].Commit, "--goal", c.id, "--brief", filepath.Join(c.root(), brief)}
		if via == "commit-in-worktree" {
			request = append(request, "--repo", c.worktree)
		}
	}
	c.refusal = row.block
	code, result := c.do(request...)
	first, _, _ := strings.Cut(result.Summary, "\n")
	admitted := row.admitted || via == "work" && row.workAdmitted
	if admitted {
		if result.Outcome != intentInProgress || len(c.delegates) != 1 || strings.Contains(c.briefs[0], "Frozen input: ") {
			t.Fatalf("an admitted brief starts the review outright, never frozen: code=%d %+v delegates=%d", code, result, len(c.delegates))
		}
	} else if code == 0 || result.Outcome != intentRefused || !strings.Contains(first, row.cause) || result.Next == nil || len(c.delegates) != 0 {
		t.Errorf("(a) the refusal names its cause %q on its first line and a next step: code=%d outcome=%s first=%q next=%v delegates=%d",
			row.cause, code, result.Outcome, first, result.Next, len(c.delegates))
	}
	if row.clear != nil {
		row.clear(c)
	}
	again := request
	if row.corrected && !admitted {
		corrected := c.brief("corrected.md", "Working Mode: implement\n\nReview the connection against its result.\n")
		again = []string{"work", "review", "--commit", c.unitCommit(t), "--goal", c.id, "--brief", filepath.Join(c.root(), corrected)}
	}
	code, result = c.do(again...)
	if result.Outcome != intentInProgress || len(c.delegates) != 1 {
		t.Fatalf("(b) nothing is bound by a refused admission: after the cause is removed the request goes through: code=%d %+v delegates=%d",
			code, result, len(c.delegates))
	}
	if row.name == "untracked-seat-file" && !admitted && !strings.Contains(c.briefs[0], "Frozen input: plans/trace.md ") {
		t.Fatalf("the seat's untracked file reaches the critic only as a frozen copy:\n%s", c.briefs[0])
	}
	other := c.brief("other.md", "Working Mode: implement\n\nAnother brief.\n")
	code, result = c.do("work", "review", "--commit", c.unitCommit(t), "--goal", c.id, "--brief", filepath.Join(c.root(), other))
	if data, _ := result.Data.(map[string]any); result.Outcome != intentRefused || data["code"] != branch.ReadBriefChangedCode || len(c.delegates) != 1 {
		t.Fatalf("(c) a dispatched review stays bound to its brief: code=%d %+v delegates=%d", code, result, len(c.delegates))
	}
	if code, result = c.do(again...); result.Outcome != intentInProgress || len(c.delegates) != 1 {
		t.Fatalf("(c) the dispatched review continues and is never started twice: code=%d %+v delegates=%d", code, result, len(c.delegates))
	}
}

// commitToMain commits files on main and publishes it, before the goal
// worktree is made from it.
func (c *connectionBed) commitToMain(files map[string]string) {
	c.t.Helper()
	root := c.root()
	for name, content := range files {
		c.writeFile(filepath.Join(root, filepath.FromSlash(name)), content)
	}
	connectionGit(c.t, root, "add", "-A")
	connectionGit(c.t, root, "commit", "-q", "-m", "matrix base")
	connectionGit(c.t, root, "push", "-q", "origin", "main")
}

func (c *connectionBed) writeFile(path, content string) {
	c.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		c.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		c.t.Fatal(err)
	}
}

// runID is the bed's one unit run.
func (c *connectionBed) runID(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir(c.unitRoot)
	if err != nil {
		t.Fatal(err)
	}
	var runs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			runs = append(runs, entry.Name())
		}
	}
	if len(runs) != 1 {
		t.Fatalf("want one unit run, have %v", runs)
	}
	return runs[0]
}

func (c *connectionBed) unitCommit(t *testing.T) string {
	t.Helper()
	subjects := c.runRecord(c.runID(t)).Subjects
	if len(subjects) != 1 || subjects[0].Commit == "" {
		t.Fatalf("no unit commit: %+v", subjects)
	}
	return subjects[0].Commit
}
