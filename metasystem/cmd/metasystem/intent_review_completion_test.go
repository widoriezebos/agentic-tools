package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// TestIntentGoalReviewCompletion drives build G, review G and revise G on
// the physical-Git connection bed: the unit runner, branch commit, push,
// read, collection and publication owners are the real ones, and so is the
// disposition join validator. Model launches, critic dispatch and the whole
// close owner (dispatch.sh close, which stamps the chain's closure) are the
// bed's fakes; the real close owner is exercised by
// TestIntentGoalReviewEmptyJoinRealClose.
func TestIntentGoalReviewCompletion(t *testing.T) {
	c := newConnectionBed(t)
	c.adapterFixture()
	c.edits = map[string]string{"connect.txt": "the built result\n"}
	code, result := c.do(append([]string{"build", c.id, "--work", "connect", "--brief", c.brief("brief.md", "Build it.\n"), "--lines", "5"}, workCheck...)...)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("build: code=%d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	install := c.worktree

	// The first review commits, publishes and requests the examination.
	if _, result = c.do("review", c.id); result.Outcome != intentInProgress || len(c.delegates) != 1 {
		t.Fatalf("review requests one examination: %+v", result)
	}
	first := c.runRecord(run).Subjects[0].Commit

	// A material finding stops at the author's decision with a bound file.
	c.writeCritic(install, "crit1", first, "completed", false)
	returnPath := filepath.Join(install, "artifacts", "agents", "crit1", "rounds", "1", "return.json")
	c.writeJSON(returnPath, map[string]any{"jobId": "crit1", "round": 1, "verdict": "1 finding",
		"findings": []any{map[string]any{"id": "F1", "material": true, "title": "the proof misses a case"}}})
	_, result = c.do("review", c.id, "--work", "connect")
	template, _ := resultData(t, result)["template"].(string)
	body, _ := os.ReadFile(template)
	if result.Outcome != intentInProgress || !strings.Contains(string(body), "Review binding: goal="+c.id+" work=connect attempt=1 subject="+first) ||
		!strings.Contains(string(body), "| F1 | DECIDE |") || !strings.Contains(result.Decision, "--dispositions") || len(c.closes) != 0 {
		t.Fatalf("findings stop at the author's bound decision: %+v %q", result, body)
	}
	if subjects := c.runRecord(run).Subjects; subjects[0].Examination != "crit1" || subjects[0].ExaminationRound != 1 {
		t.Fatalf("the examination is recorded with the reviewed subject: %+v", subjects)
	}

	// A file bound to another subject is refused before any close.
	stale := filepath.Join(c.root(), "stale.md")
	os.WriteFile(stale, []byte(strings.Replace(strings.Replace(string(body), "subject="+first, "subject="+strings.Repeat("0", 40), 1), "DECIDE", "noted", 1)), 0o600)
	if _, result = c.do("review", c.id, "--dispositions", stale); result.Outcome != intentRefused || !strings.Contains(result.Summary, "not the current examination") || len(c.closes) != 0 {
		t.Fatalf("a stale decisions file: %+v", result)
	}
	// An accepted material finding requires a correction, never a close.
	decided := filepath.Join(c.root(), "decided.md")
	os.WriteFile(decided, []byte(strings.Replace(string(body), "| F1 | DECIDE | | |", "| F1 | accepted | the case is real | add it |", 1)), 0o600)
	_, result = c.do("review", c.id, "--dispositions", decided)
	if result.Outcome != intentRefused || result.Next == nil || !slices.Contains(result.Next.Argv, "revise") ||
		!slices.Contains(result.Next.Argv, "--dispositions") || len(c.closes) != 0 || c.commitReads != 0 {
		t.Fatalf("accepted material finding: %+v", result)
	}

	// The correction carries the reviewed findings and decisions; repeating
	// it rejoins the same attempt without another launch.
	c.edits = map[string]string{"connect.txt": "the built result, fixed\n"}
	fix := c.brief("fix.md", "Fix F1.\n")
	code, result = c.do("revise", c.id, "--after", "1", "--brief", fix, "--dispositions", decided)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("revise: code=%d %+v", code, result)
	}
	record := c.runRecord(run)
	if len(record.Rounds) != 2 || len(record.Revisions) != 1 || record.Revisions[0].Decisions == "" {
		t.Fatalf("one retained correction with frozen decisions: %+v", record.Revisions)
	}
	frozen, _ := os.ReadFile(record.Revisions[0].Decisions)
	if !strings.Contains(string(frozen), "| F1 | accepted |") || !strings.Contains(string(frozen), "the proof misses a case") {
		t.Fatalf("the frozen document carries the decisions and the findings: %q", frozen)
	}
	launched := len(c.starter.launched())
	if _, result = c.do("revise", c.id, "--after", "1", "--brief", fix, "--dispositions", decided); result.Outcome != intentConfirmed ||
		len(c.starter.launched()) != launched || resultData(t, result)["revision"].(map[string]any)["rejoined"] != true {
		t.Fatalf("a repeated correction rejoins: %+v", result)
	}

	// The corrected attempt is reviewed again; its examination has no
	// findings, so the review closes, collects and publishes by itself.
	if _, result = c.do("review", c.id); result.Outcome != intentInProgress || len(c.delegates) != 2 {
		t.Fatalf("the new attempt needs its own examination: %+v", result)
	}
	second := c.runRecord(run).Subjects[1].Commit
	c.writeCritic(install, "crit2", second, "completed", false)
	c.writeJSON(filepath.Join(install, "artifacts", "agents", "crit2", "rounds", "1", "return.json"),
		map[string]any{"jobId": "crit2", "round": 1, "verdict": "clean", "findings": []any{}})
	// The whole close fails once (its mirror is missing): nothing is
	// collected, and repair review replays the close with the retained
	// decisions.
	c.failCloses = 1
	_, result = c.do("review", c.id)
	if result.Outcome != intentRefused || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "repair", "review", c.id, "--work", "connect"}) ||
		len(c.closes) != 1 || c.commitReads != 0 {
		t.Fatalf("a failed whole close: %+v", result)
	}
	code, result = c.do("repair", "review", c.id)
	if code != 0 || result.Outcome != intentConfirmed || len(c.closes) != 2 || c.commitReads != 1 || c.publications == 0 ||
		result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "land", c.id}) {
		t.Fatalf("the repaired close completes the review: code=%d %+v closes=%d reads=%d", code, result, len(c.closes), c.commitReads)
	}
	if generated, err := os.ReadFile(filepath.Join(install, "artifacts", "agents", "crit2", "rounds", "1", "decisions.md")); err != nil ||
		!strings.Contains(string(generated), "attempt=2 subject="+second) {
		t.Fatalf("the empty join is bound to the reviewed attempt: %q %v", generated, err)
	}
	// Selection reads the read owner: the work is reviewed, its read
	// collected and published; a later built item is the one review picks.
	if _, result = c.do("status", c.id); !strings.Contains(result.Summary, "its read is collected and published (attestation ") || result.Next == nil || result.Next.Argv[1] != "land" {
		t.Fatalf("status after collection: %+v", result)
	}
	c.edits = map[string]string{"later.txt": "a later unit\n"}
	if _, result = c.do(append([]string{"build", c.id, "--work", "later", "--brief", c.brief("later.md", "Later.\n"), "--lines", "5"}, workCheck...)...); result.Outcome != intentConfirmed {
		t.Fatalf("later build: %+v", result)
	}
	if _, result = c.do("review", c.id); result.Outcome != intentInProgress || len(c.delegates) != 3 {
		t.Fatalf("review without --work picks the one unreviewed item: %+v delegates=%v", result, c.delegates)
	}
	// The earlier decisions are now superseded by attempt 2's examination.
	if _, result = c.do("revise", c.id, "--work", "connect", "--brief", fix, "--dispositions", decided); result.Outcome != intentRefused || !strings.Contains(result.Summary, "supersedes") {
		t.Fatalf("superseded decisions: %+v", result)
	}
	// Repeating the completed review changes nothing.
	if code, result = c.do("review", c.id, "--work", "connect"); code != 0 || len(c.closes) != 2 || c.commitReads != 1 {
		t.Fatalf("repeat: code=%d %+v", code, result)
	}
}

// TestIntentGoalReviewEmptyJoinRealClose: an examination with no findings is
// decided by the generated, bound empty join, and the real whole close owner
// (dispatch.sh close with its register, mirror and close check) closes the
// chain. A stale binding refuses before the owner runs.
func TestIntentGoalReviewEmptyJoinRealClose(t *testing.T) {
	b := newDeliveryBed(t)
	realCloseOwner(t, b)
	b.writeFile(filepath.Join(b.install, "artifacts", "agents", "capabilities", "close.json"), `{"ok":true}`)
	if err := os.MkdirAll(filepath.Join(b.install, "artifacts", "agents", "record-locks"), 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(b.install, "metasystem.conf")
	existing, _ := os.ReadFile(conf)
	b.writeFile(conf, string(existing)+"\nevidence.root="+t.TempDir()+"\n")
	b.writeJob(map[string]any{"jobId": "crit9", "role": "design-critic", "status": "completed", "round": 1,
		"destructiveReach": "DESIGN-BEARING", "dispatchMode": "fresh", "sessionId": "crit9-session", "endedAt": "2026-09-25T11:00:00Z",
		"capabilitySnapshot": "artifacts/agents/capabilities/close.json", "parentJob": nil, "findingRegister": []any{}})
	b.writeJSON(filepath.Join(b.install, "artifacts", "agents", "crit9", "rounds", "1", "return.json"),
		map[string]any{"jobId": "crit9", "round": 1, "findings": []any{}, "verdict": "clean"})
	if advanced, err := dispatchcore.CritiqueRegisterAdvance(b.install, "crit9", "crit9"); err != nil || advanced != "advanced" {
		t.Fatalf("register advance = %q, %v", advanced, err)
	}
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	layout, err := stateroot.NewResolver(fakeTop(b.root()), noExecutable).ResolveLayout(b.root())
	if err != nil {
		t.Fatal(err)
	}
	subject := strings.Repeat("a", 40)
	invocation := func(values map[string][]string) *intentInvocation {
		return &intentInvocation{owners: owners, layout: layout, cwd: b.root(), input: intentInput{values: values},
			reviewWork: &reviewWorkContext{goal: "design", work: "main", attempt: 1}}
	}
	stale := filepath.Join(b.root(), "stale.md")
	b.writeFile(stale, "Review binding: goal=design work=main attempt=2 subject="+subject+" examination=crit9 round=1 return="+strings.Repeat("0", 64)+"\n\n"+deliveryDispositionsHeader)
	if pending := invocation(map[string][]string{"dispositions": {stale}}).closeWorkReview(nil, b.install, subject, "crit9"); pending == nil ||
		pending.Outcome != intentRefused || len(b.calls) != 0 {
		t.Fatalf("a stale binding reaches the close owner: %+v calls=%v", pending, b.calls)
	} else if !strings.Contains(pending.Decision, "metasystem review goal design --work main --dispositions ") {
		t.Fatalf("the decisions continuation changed the selected goal into a design subject: %+v", pending)
	}
	// Before the whole close mirrors it, the owner's own check calls the
	// chain unmirrored: a failed close then is repaired by replaying it.
	if err := dispatchcore.CloseCheck(b.install, "crit9"); err == nil || !strings.Contains(err.Error(), "unmirrored") {
		t.Fatalf("the close check before mirroring = %v", err)
	}
	if pending := invocation(map[string][]string{}).closeWorkReview(nil, b.install, subject, "crit9"); pending != nil {
		t.Fatalf("the empty join did not close through the real owner: %+v", pending)
	}
	critic := b.job("crit9")
	if closed, _ := critic["chainClosed"].(bool); !closed || critic["mirror"] == nil || len(b.calls) != 1 {
		t.Fatalf("the real owner closes and mirrors the chain: %v calls=%v", critic, b.calls)
	}
	generated, _ := os.ReadFile(filepath.Join(b.install, "artifacts", "agents", "crit9", "rounds", "1", "decisions.md"))
	if !strings.Contains(string(generated), "attempt=1 subject="+subject+" examination=crit9 round=1") {
		t.Fatalf("the generated join is bound: %q", generated)
	}
}

// TestIntentReviewAcceptedRiskException: an accepted material finding closes
// only when the goal ledger records a person's accepted risk for exactly
// that finding of that examination; the author's decision alone, or a risk
// recorded for another examination, sends it to revise.
func TestIntentReviewAcceptedRiskException(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	b.writeJob(map[string]any{"jobId": "crit7", "role": "code-critic", "status": "completed", "round": 1, "findingRegister": []any{}})
	b.writeJSON(filepath.Join(b.install, "artifacts", "agents", "crit7", "rounds", "1", "return.json"),
		map[string]any{"jobId": "crit7", "round": 1, "verdict": "1 finding", "findings": []any{map[string]any{"id": "S-1", "material": true}}})
	b.handler = func(process intentProcess) intentProcessResult {
		record := b.job("crit7")
		record["chainClosed"] = true
		b.writeJob(record)
		return intentProcessResult{}
	}
	layout, err := stateroot.NewResolver(fakeTop(b.root()), noExecutable).ResolveLayout(b.root())
	if err != nil {
		t.Fatal(err)
	}
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	subject := strings.Repeat("d", 40)
	digest, _, _ := reviewReturnDigest(filepath.Join(b.install, "artifacts", "agents", "crit7", "rounds", "1", "return.json"))
	decided := filepath.Join(b.root(), "decided.md")
	b.writeFile(decided, reviewBinding{Goal: bedGoal, Work: "main", Attempt: 1, Subject: subject, Examination: "crit7", Round: 1, Return: digest}.line()+"\n\n"+
		deliveryDispositionsHeader+"| S-1 | accepted | the exposure is local | none |\n")
	close := func() *intentResult {
		inv := &intentInvocation{owners: owners, layout: layout, cwd: b.root(), stateRoot: b.root(),
			input: intentInput{values: map[string][]string{"dispositions": {decided}}}, reviewWork: &reviewWorkContext{goal: bedGoal, work: "main", attempt: 1}}
		return inv.closeWorkReview(nil, b.install, subject, "crit7")
	}
	if pending := close(); pending == nil || pending.Outcome != intentRefused || !slices.Contains(pending.next, "revise") || len(b.calls) != 0 {
		t.Fatalf("the author's decision alone: %+v", pending)
	}
	file := b.goalFile(bedGoal)
	file.AcceptedRisks = []goal.AcceptedRiskRecord{{Finding: "S-1", Chain: "another-review", By: "Wido", Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FD0", "mac-cli", "m1")}}
	b.addGoal(file)
	if pending := close(); pending == nil || pending.Outcome != intentRefused || len(b.calls) != 0 {
		t.Fatalf("a risk recorded for another examination: %+v", pending)
	}
	file.AcceptedRisks[0].Chain = "crit7"
	b.addGoal(file)
	if pending := close(); pending != nil || len(b.calls) != 1 {
		t.Fatalf("the recorded accepted risk is the lawful exception: %+v calls=%v", pending, b.calls)
	}
}
