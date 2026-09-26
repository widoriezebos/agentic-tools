package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The design journeys below run against the dispatcher fixture's armed,
// enrolled bed (scripts/agents/dispatch-fixtures.sh, cluster b), which
// wires it in and hands over its environment. Every agent act is a public
// command of the bed's installed engine; dispatch, follow-up, claim, close,
// launch supervision and cancellation are the real owners. The fakes are the
// critic runtime (fake), findings the fixture writes into a round's return
// (the bed's established practice) and a design author that answers the
// Claude protocol from its prompt alone. Outside the bed the tests skip.

const designBedEnv = "METASYSTEM_INTENT_DESIGN_BED"

// designBed is the handed-over dispatcher fixture bed.
type designBed struct {
	t                               *testing.T
	repo, engine, dispatch, lineage string
	supervision, follow, log        string
	env                             []string
}

func newDesignBed(t *testing.T) *designBed {
	t.Helper()
	repo := os.Getenv(designBedEnv)
	if repo == "" {
		t.Skip("runs inside the dispatcher fixture bed (dispatch-fixtures.sh, cluster b)")
	}
	// The command-test harness isolates this process's registry and
	// ambient controls; the bed's own environment is what its commands run
	// with, exactly as the fixture's shell would run them.
	raw, err := os.ReadFile(os.Getenv(designBedEnv + "_ENV"))
	if err != nil {
		t.Fatalf("the bed environment: %v", err)
	}
	var env []string
	for _, entry := range bytes.Split(raw, []byte{0}) {
		if len(entry) > 0 {
			env = append(env, string(entry))
		}
	}
	b := &designBed{t: t, repo: repo, engine: filepath.Join(repo, "bin", "metasystem"), dispatch: filepath.Join(repo, "scripts", "agents", "dispatch.sh"),
		lineage: os.Getenv(designBedEnv + "_LINEAGE"), supervision: os.Getenv(designBedEnv + "_SUPERVISION"),
		follow: os.Getenv(designBedEnv + "_FOLLOW"), log: t.TempDir(), env: env}
	if b.lineage == "" || b.follow == "" {
		t.Fatal("the bed did not hand over its lease holder main and follow-up message")
	}
	return b
}

// run runs a command in dir with the bed's environment plus extra.
func (b *designBed) run(dir string, extra []string, name string, args ...string) (string, int) {
	b.t.Helper()
	command := exec.Command(name, args...)
	command.Dir, command.Env = dir, append(append([]string{}, b.env...), extra...)
	output, err := command.CombinedOutput()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		b.t.Fatalf("%s %v: %v", name, args, err)
	}
	return string(output), code
}

// cli runs a public command as the lease holder's main and decodes its
// JSON result.
func (b *designBed) cli(dir string, extra []string, args ...string) map[string]any {
	b.t.Helper()
	b.censusFresh()
	command := exec.Command(b.engine, append(args, "--json")...)
	command.Dir, command.Env = dir, append(append([]string{}, b.env...), append([]string{"METASYSTEM_OWNER_LINEAGE=" + b.lineage}, extra...)...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	code := 0
	if err := command.Run(); err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			b.t.Fatalf("%v did not run: %v", args, err)
		}
		code = exit.ExitCode()
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		b.t.Fatalf("%v printed no result (exit %d): %v\nstdout=%s\nstderr=%s", args, code, err, stdout.String(), stderr.String())
	}
	// The command's own exit code travels with its result.
	result[cliExitKey] = code
	b.t.Logf("%v: exit %d: %s: %s", args, code, result["outcome"], result["summary"])
	return result
}

const cliExitKey = "fixtureCommandExit"

// cliExit is the exit code the command itself returned.
func cliExit(result map[string]any) int {
	code, _ := result[cliExitKey].(int)
	return code
}

// expectExit fails unless the command itself exited with want or, for
// want < 0, with any nonzero code. The intent contract exits 0 for a
// confirmed result and 1 for work still in progress.
func expectExit(t *testing.T, what string, result map[string]any, want int) {
	t.Helper()
	if code := cliExit(result); want >= 0 && code != want || want < 0 && code == 0 {
		t.Fatalf("%s: the command exited %d (want %d): %v", what, code, want, result)
	}
}

// censusFresh is the bed's census wait before a dispatching command.
func (b *designBed) censusFresh() {
	if b.supervision == "" {
		return
	}
	state := filepath.Join(b.supervision, "artifacts", "agents", "supervision")
	poll := os.Getenv("METASYSTEM_FIXTURE_POLL_INTERVAL_MS")
	for _, entry := range b.env {
		if value, ok := strings.CutPrefix(entry, "METASYSTEM_FIXTURE_POLL_INTERVAL_MS="); ok {
			poll = value
		}
	}
	if poll == "" {
		poll = "100"
	}
	if output, code := b.run(b.supervision, nil, filepath.Join(b.supervision, "bin", "metasystem"), "job", "census-wait", "--root", b.supervision, "--repo", b.supervision,
		"--arm", "rearm", "--verdict", filepath.Join(state, "last-census.json"), "--state", filepath.Join(state, "state.json"),
		"--attempt-budget", "2", "--max-regressions", "5", "--poll-ms", poll); code != 0 {
		b.t.Fatalf("no fresh census: %s", output)
	}
}

// release gives a goal's claim back once its journey is done: the bed
// allows one claim per machine.
func (b *designBed) release(id string) {
	b.t.Helper()
	if output, code := b.run(b.repo, []string{"METASYSTEM_OWNER_LINEAGE=" + b.lineage}, b.engine, "goal", "release", "--root", b.repo, "--id", id); code != 0 {
		b.t.Fatalf("goal release %s: %s", id, output)
	}
}

// releaseOnFailure gives the claim back when a journey fails early, so the
// next journey is judged on its own.
func (b *designBed) releaseOnFailure(id string) {
	b.t.Cleanup(func() {
		if b.t.Failed() {
			b.run(b.repo, []string{"METASYSTEM_OWNER_LINEAGE=" + b.lineage}, b.engine, "goal", "release", "--root", b.repo, "--id", id)
		}
	})
}

func (b *designBed) fixtureGoal(id, intent string) {
	b.t.Helper()
	if output, code := b.run(b.repo, []string{"METASYSTEM_OWNER_LINEAGE=agent-fixture"}, b.engine, "goal", "open", "--root", b.repo, "--id", id, "--origin", "human", "--by", "Wido",
		"--fixture-human-authority", "--intent", intent, "--next", "Drive it through intent.", "--risk", "severity=3,novelty=1,exposure=1,accumulation=1",
		"--basis", "The fixture drives a public design route over the real owners."); code != 0 {
		b.t.Fatalf("goal open: %s", output)
	}
	if output, code := b.run(b.repo, nil, b.engine, "goal", "approve", "--root", b.repo, "--id", id, "--by", "Wido", "--lineage", "agent-fixture",
		"--elapsed-limit", "8h", "--attempt-limit", "10", "--reserved-job-minutes-limit", "1200", "--active-job-limit", "1",
		"--review-round-limit", "3", "--fixture-human-authority"); code != 0 {
		b.t.Fatalf("goal approve: %s", output)
	}
}

// goalRecord is the goal's structured state and revision on the accepted
// tree.
func (b *designBed) goalRecord(id string) (string, float64) {
	b.t.Helper()
	output, code := b.run(b.repo, nil, b.engine, "goal", "show", "--root", b.repo, "--id", id)
	var page struct {
		Goal struct {
			State    string
			Revision float64
		} `json:"goal"`
	}
	if code != 0 || json.Unmarshal([]byte(output), &page) != nil || page.Goal.State == "" {
		b.t.Fatalf("goal show %s: %s", id, output)
	}
	return page.Goal.State, page.Goal.Revision
}

func (b *designBed) goalState(id string) string {
	b.t.Helper()
	state, _ := b.goalRecord(id)
	return state
}

func (b *designBed) job(id string) map[string]any {
	b.t.Helper()
	var record map[string]any
	data, err := os.ReadFile(filepath.Join(b.repo, "artifacts", "agents", "jobs", id+".json"))
	if err != nil || json.Unmarshal(data, &record) != nil {
		b.t.Fatalf("job %s: %v", id, err)
	}
	return record
}

func (b *designBed) jobExists(id string) bool {
	_, err := os.Stat(filepath.Join(b.repo, "artifacts", "agents", "jobs", id+".json"))
	return err == nil
}

// jobs counts the records of a chain root and its rounds.
func (b *designBed) jobs(root string) int {
	matches, _ := filepath.Glob(filepath.Join(b.repo, "artifacts", "agents", "jobs", root+"*.json"))
	return len(matches)
}

func (b *designBed) waitStatus(id, status string) {
	b.t.Helper()
	// The engine's durable wait owner returns when the job's record is
	// terminal (or at its bounded deadline); the status is then read.
	output, _ := b.run(b.repo, nil, b.engine, "wait", "--root", b.repo, "--job", id, "--timeout", "3m")
	if b.jobExists(id) && fmt.Sprint(b.job(id)["status"]) == status {
		return
	}
	b.t.Fatalf("job %s never reached %s: %s", id, status, output)
}

// writeFindings is the fake critic's return for a round: the bed's
// established practice, followed by the real register advance.
func (b *designBed) writeFindings(root string, round int, member string, findings, rigor []map[string]any, material int) {
	b.t.Helper()
	path := filepath.Join(b.repo, "artifacts", "agents", root, "rounds", strconv.Itoa(round), "return.json")
	var result map[string]any
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &result) != nil {
		b.t.Fatalf("return %s: %v", path, err)
	}
	result["schemaVersion"], result["findings"], result["rigor"], result["verdictMaterialCount"] = 5, findings, rigor, material
	encoded, _ := json.MarshalIndent(result, "", "  ")
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		b.t.Fatal(err)
	}
	if output, code := b.run(b.repo, nil, b.engine, "job", "critique-register-advance", "--repo", b.repo, "--root-job", root, "--round-job", member); code != 0 {
		b.t.Fatalf("register advance: %s", output)
	}
}

func designResultData(result map[string]any, field string) string {
	data, _ := result["data"].(map[string]any)
	value, _ := data[field].(string)
	return value
}

// decide fills every DECIDE row of a decisions template with one decision.
func decide(t *testing.T, template, disposition, reasoning string) {
	t.Helper()
	data, err := os.ReadFile(template)
	if err != nil {
		t.Fatalf("decisions template: %v", err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "| ") && strings.Contains(line, " | DECIDE | | |") {
			line = strings.Replace(line, " | DECIDE | | |", " | "+disposition+" | "+reasoning+" | none |", 1)
		}
		lines = append(lines, line)
	}
	if err := os.WriteFile(template, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestIntentDesignReviewRealOwnerJourney drives review design FILE
// [--dispositions FILE] over the real dispatcher: the first critique claims
// the approved goal; a changed page cannot buy a fresh chain from the
// dispatcher; the author's bound decisions continue the same chain through
// the real follow-up, whose child examines the refreshed page; a replay after
// the goal's revision moved and after a lost response rejoins that child; the frozen cap
// refuses a further round; decisions on the unchanged page close the whole
// chain. Serial: it drives one shared armed bed whose goals, lease and
// supervision are process-wide.
func TestIntentDesignReviewRealOwnerJourney(t *testing.T) {
	b := newDesignBed(t)
	goalID, page := "intent-design-journey", "plans/designs/intent-design-journey.md"
	pagePath := filepath.Join(b.repo, page)
	os.MkdirAll(filepath.Dir(pagePath), 0o755)
	os.WriteFile(pagePath, []byte("# Intent design journey\n\n- Kind: design\n- Id: 01M3EFDSFTKWEMSDCP1BB7TJRN\n- Status: draft\n- Goals: "+goalID+"\n\nFirst version.\n"), 0o644)
	for _, args := range [][]string{{"add", "--", page}, {"-c", "core.hooksPath=/dev/null", "-c", "user.name=metasystem", "-c", "user.email=metasystem@example.invalid", "commit", "-qm", "add the intent design journey page"}} {
		if output, code := b.run(b.repo, nil, "git", append([]string{"-C", b.repo}, args...)...); code != 0 {
			t.Fatalf("git %v: %s", args, output)
		}
	}
	b.fixtureGoal(goalID, "Prove the public design critique journey")
	b.releaseOnFailure(goalID)
	review := func(extra ...string) map[string]any {
		return b.cli(b.repo, nil, append([]string{"review", "design", page, "--tool-calls", "30"}, extra...)...)
	}

	first := review()
	expectExit(t, "the first review", first, 1)
	root := designResultData(first, "job")
	if root == "" || fmt.Sprint(b.job(root)["goalId"]) != goalID {
		t.Fatalf("the first review started no goal-bound critic: %v", first)
	}
	if state := b.goalState(goalID); state != "claimed" {
		t.Fatalf("the first critique did not claim the goal: state %s", state)
	}
	b.waitStatus(root, "completed")
	safe := map[string]any{"local": true, "recoverable": true, "proofBoundaryCrossed": false, "authorityBoundaryCrossed": false, "secretsBoundaryCrossed": false,
		"irreversibleDataBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false}
	b.writeFindings(root, 1, root, []map[string]any{{"id": "JOURNEY-A", "severity": "medium", "material": true, "claim": "the page lacks a replay rule", "evidence": "read"}},
		[]map[string]any{{"findingId": "JOURNEY-A", "rigorClass": "bounded", "facts": safe, "reopeningTrigger": "if replay recurs",
			"artifact": "metasystem/internal/dispatch/build.go", "grain": "mechanical", "behaviour": "replay", "fixture": "go test ./replay"}}, 1)

	// The same page again shows the examination; nothing is dispatched.
	jobs := b.jobs(root)
	again := review()
	expectExit(t, "the same page again", again, 0)
	if designResultData(again, "job") != root || again["outcome"] == "refused" || b.jobs(root) != jobs {
		t.Fatalf("the same page did not rejoin its examination: %v", again)
	}

	// The author changes the page. The dispatcher itself refuses a fresh
	// root for the goal's open chain of this document.
	os.WriteFile(pagePath, append(designBedRead(os.ReadFile(pagePath)), []byte("\nReplays rejoin the retained child.\n")...), 0o644)
	briefs, _ := filepath.Glob(filepath.Join(b.repo, "artifacts", "agents", "intent-review", "design-*", "brief.md"))
	outputs, _ := filepath.Glob(filepath.Join(b.repo, "artifacts", "agents", "intent-review", "design-*", "outputs.md"))
	if len(briefs) == 0 || len(outputs) == 0 {
		t.Fatal("the review left no brief or outputs")
	}
	b.censusFresh()
	output, code := b.run(b.repo, []string{"METASYSTEM_OWNER_LINEAGE=" + b.lineage}, b.dispatch, "dispatch", "--role", "design-critic", "--outputs", outputs[0],
		"--design", page, "--brief", briefs[0], "--goal", goalID, "--job-id", "intent-design-fresh")
	if code == 0 || b.jobExists("intent-design-fresh") || !strings.Contains(output, "DESIGN_CHAIN_OPEN") {
		t.Fatalf("a fresh root for the goal's open design chain was not refused: code=%d %s", code, output)
	}

	changed := review()
	template := designResultData(changed, "template")
	decide(t, template, "accepted", "the replay rule was added in the changed page")

	continued := review("--dispositions", template)
	expectExit(t, "the continuation", continued, 1)
	child := designResultData(continued, "job")
	if child != root+"-r2" || fmt.Sprint(b.job(child)["parentJob"]) != root {
		t.Fatalf("the continuation is not the chain's second round: %v", continued)
	}
	b.waitStatus(child, "completed")
	var subject struct {
		ContentDigest string `json:"contentDigest"`
	}
	json.Unmarshal(designBedRead(os.ReadFile(filepath.Join(b.repo, "artifacts", "agents", root, "rounds", "2", "subject.json"))), &subject)
	if digest, _ := b.run(b.repo, nil, b.engine, "util", "sha256", "--file", pagePath); subject.ContentDigest != strings.TrimSpace(digest) {
		t.Fatalf("the follow-up did not examine the refreshed page: %s", subject.ContentDigest)
	}

	// The goal's revision moves (its holder rewrites the next step); the
	// replay still rejoins the same child.
	_, revision := b.goalRecord(goalID)
	if output, code := b.run(b.repo, []string{"METASYSTEM_OWNER_LINEAGE=" + b.lineage}, b.engine, "goal", "set-next", "--root", b.repo, "--id", goalID,
		"--next", "Decide the second examination."); code != 0 {
		t.Fatalf("goal set-next: %s", output)
	}
	if _, moved := b.goalRecord(goalID); moved <= revision {
		t.Fatalf("the goal revision did not move: %v -> %v", revision, moved)
	}
	jobs = b.jobs(root)
	replay := review("--dispositions", template)
	expectExit(t, "the replay", replay, 0)
	if designResultData(replay, "job") != child || b.jobs(root) != jobs {
		t.Fatalf("the replay after completion did not rejoin %s: %v", child, replay)
	}
	// A lost response: the retained request is gone; the operation's own
	// record names the child again.
	entries, _ := filepath.Glob(filepath.Join(b.repo, "artifacts", "agents", "intent-review", "design-*", "chain.json"))
	var entry map[string]any
	json.Unmarshal(designBedRead(os.ReadFile(entries[0])), &entry)
	delete(entry, "requests")
	encoded, _ := json.Marshal(entry)
	os.WriteFile(entries[0], encoded, 0o644)
	lost := review("--dispositions", template)
	expectExit(t, "the lost-response replay", lost, 0)
	if designResultData(lost, "job") != child || b.jobs(root) != jobs {
		t.Fatalf("the lost-response replay did not rejoin %s: %v", child, lost)
	}

	// The frozen cap: a direct third round is refused by the dispatcher.
	b.censusFresh()
	if output, code := b.run(b.repo, nil, b.dispatch, "follow-up", "--job", child, "--message", b.follow, "--wait"); code == 0 || b.jobExists(root+"-r3") {
		t.Fatalf("a third round was not refused at the cap: code=%d %s", code, output)
	}

	// Decisions on the last examination of the unchanged page close the
	// whole chain through review design itself.
	b.writeFindings(root, 2, child, []map[string]any{{"id": "JOURNEY-A", "severity": "low", "material": false, "claim": "replay rule added", "evidence": "read"}}, []map[string]any{}, 0)
	last := review()
	template = designResultData(last, "template")
	decide(t, template, "noted", "the second examination found it resolved and not material")
	expectExit(t, "the closing review", review("--dispositions", template), 0)
	if closed, _ := b.job(root)["chainClosed"].(bool); !closed {
		t.Fatalf("review design did not close the clean critique")
	}
	b.release(goalID)
}

func designBedRead(data []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return data
}

// designAuthorProtocol answers the Claude headless protocol as a design
// author when this test binary runs under the name claude: it reads only
// its prompt, writes the draft to the output file the prompt names, starting
// from the frozen prior or the reserved head the prompt gives, and prints a
// Claude JSON result. A prompt asking to hold waits to be stopped.
func designAuthorProtocol() {
	prompt, _ := io.ReadAll(os.Stdin)
	result := func(isError bool, text string) {
		fmt.Printf(`{"type":"result","is_error":%t,"num_turns":1,"result":%q,"session_id":"fixture-author-%d"}`+"\n", isError, text, os.Getpid())
		os.Exit(0)
	}
	if bytes.Contains(prompt, []byte("FIXTURE-AUTHOR-HOLD")) {
		// Held until the launch owner stops it: block on its signal.
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
		<-stop
		os.Exit(143)
	}
	output, head, err := authorPromptTask(string(prompt))
	if err != nil {
		result(true, err.Error())
	}
	if err := os.WriteFile(output, []byte(strings.TrimRight(head, "\n")+"\n\nWritten by the fixture design author from its prompt alone.\n"), 0o644); err != nil {
		result(true, err.Error())
	}
	result(false, "draft written")
}

func init() {
	if filepath.Base(os.Args[0]) == "claude" {
		designAuthorProtocol()
	}
}

// TestIntentDesignAuthorRealOwnerJourney drives design G from a second
// checkout through the real launch supervisor, with this binary as the
// claude executable: attempt 1 is published into that checkout's design
// home only and rejoined without a launch; the goal stays approved with no
// build claim; a running attempt is stopped through the launch owner and
// leaves the document as it was; the next attempt is published. Serial: it
// drives the shared armed bed.
func TestIntentDesignAuthorRealOwnerJourney(t *testing.T) {
	b := newDesignBed(t)
	goalID := "intent-design-author"
	b.fixtureGoal(goalID, "Prove the public design author journey")
	second := filepath.Join(t.TempDir(), "second-checkout")
	if output, code := b.run(b.repo, nil, "git", "-C", b.repo, "worktree", "add", "-q", "--detach", second, "HEAD"); code != 0 {
		t.Fatalf("second checkout: %s", output)
	}
	second, _ = filepath.EvalSymlinks(second)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	provider := t.TempDir()
	if err := os.Symlink(executable, filepath.Join(provider, "claude")); err != nil {
		t.Fatal(err)
	}
	path := provider
	for _, entry := range b.env {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			path += string(os.PathListSeparator) + value
		}
	}
	// The fake-runtime tailoring puts the author lane on runtime fake,
	// which it does not run; the environment puts it back on claude.
	extra := []string{"PATH=" + path, "METASYSTEM_LAUNCH_DESIGN_RUNTIME=claude"}
	brief := filepath.Join(b.log, "brief.md")
	design := func(text string, args ...string) map[string]any {
		os.WriteFile(brief, []byte(text), 0o644)
		return b.cli(second, extra, append([]string{"design", goalID, "--brief", brief}, args...)...)
	}
	published := func(text string, attempt float64) map[string]any {
		deadline := time.Now().Add(time.Minute)
		for {
			result := design(text)
			data, _ := result["data"].(map[string]any)
			if data["outcome"] == "published" && data["attempt"] == attempt && cliExit(result) == 0 {
				return result
			}
			if time.Now().After(deadline) {
				t.Fatalf("attempt %v was never published: %v", attempt, result)
			}
			// The public wait waits on the running author; the repeated
			// design then judges its publication.
			b.cli(second, extra, "wait", "goal", goalID, "--timeout", "1m")
		}
	}
	first := "Design the fixture reader.\n"
	if data, _ := design(first)["data"].(map[string]any); data["attempt"] != float64(1) {
		t.Fatalf("the request reached no attempt 1: %v", data)
	}
	document := designResultData(published(first, 1), "document")
	written, err := os.ReadFile(filepath.Join(second, document))
	if _, primary := os.Stat(filepath.Join(b.repo, document)); err != nil || primary == nil ||
		!strings.Contains(string(written), "- Status: draft\n") || !strings.Contains(string(written), "- Goals: "+goalID+"\n") ||
		!strings.Contains(string(written), "from its prompt alone") {
		t.Fatalf("the draft is not published into the invoking checkout only: %v %q", err, written)
	}
	if data, _ := design(first)["data"].(map[string]any); data["rejoined"] != true || data["attempt"] != float64(1) {
		t.Fatalf("the same request did not rejoin attempt 1: %v", data)
	}
	if state := b.goalState(goalID); state != "approved" {
		t.Fatalf("design changed the approved goal's state to %s", state)
	}

	held := design("Design it again. FIXTURE-AUTHOR-HOLD\n", "--after", "1")
	if data, _ := held["data"].(map[string]any); data["attempt"] != float64(2) || held["outcome"] != "in-progress" {
		t.Fatalf("the held attempt 2 is not running: %v", held)
	}
	if stopped := b.cli(second, nil, "stop", "design", goalID); stopped["outcome"] != "confirmed" || cliExit(stopped) != 0 {
		t.Fatalf("stop design did not stop the running author: %v", stopped)
	}
	if now, _ := os.ReadFile(filepath.Join(second, document)); !bytes.Equal(now, written) {
		t.Fatalf("the stopped author changed the document: %q", now)
	}
	third := "Design it a third time.\n"
	design(third, "--after", "2")
	published(third, 3)
}

// fakeBehaviour points the bed copy of the fake provider at one extra
// behaviour source, a marker file under the disposable bed's artifacts,
// through its existing behavior_sources(); the original bytes come back
// once the rounds it drove are dead. Prompts, subjects, briefs and
// terminal records are never touched.
func (b *designBed) fakeBehaviour() (marker string, restore func()) {
	b.t.Helper()
	provider := filepath.Join(b.repo, "scripts", "agents", "adapters", "fake.sh")
	original, err := os.ReadFile(provider)
	if err != nil {
		b.t.Fatal(err)
	}
	marker = filepath.Join(b.repo, "artifacts", "agents", "intent-design-fake-behaviour.md")
	anchor := "behavior_sources() {\n  printf '%s\\n' \"$prompt\"\n"
	if !bytes.Contains(original, []byte(anchor)) {
		b.t.Fatalf("the fake provider's behavior_sources() changed shape")
	}
	patched := bytes.Replace(original, []byte(anchor), []byte(anchor+"  [[ -f '"+marker+"' ]] && printf '%s\\n' '"+marker+"'\n"), 1)
	if err := testexec.WriteFile(provider, patched, 0o755); err != nil {
		b.t.Fatal(err)
	}
	return marker, func() {
		os.Remove(marker)
		if err := testexec.WriteFile(provider, original, 0o755); err != nil {
			b.t.Errorf("restore the fake provider: %v", err)
		}
		if now, _ := os.ReadFile(provider); !bytes.Equal(now, original) {
			b.t.Errorf("the fake provider was not restored byte for byte")
		}
	}
}

func (b *designBed) waitFile(path string) {
	b.t.Helper()
	// The engine's durable file wait returns once the path exists.
	output, _ := b.run(b.repo, nil, b.engine, "wait", "--root", b.repo, "--path", path, "--until", "present", "--timeout", "2m")
	if _, err := os.Stat(path); err == nil {
		return
	}
	b.t.Fatalf("%s never appeared: %s", path, output)
}

// reapDead runs the real reaper on a round until it is terminal with the
// reaper's recorded group-death proof, and its held child has stopped.
func (b *designBed) reapDead(job, roundDir string) map[string]any {
	b.t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		output, _ := b.run(b.repo, nil, b.dispatch, "reap", "--job", job)
		record := b.job(job)
		status := fmt.Sprint(record["status"])
		if status != "running" && status != "pending" && status != "pending-setup" {
			b.waitFile(filepath.Join(roundDir, "child.stopped"))
			record = b.job(job)
			if fmt.Sprint(record["groupDeathProvenAt"]) == "" || record["groupDeathProvenAt"] == nil {
				b.t.Fatalf("job %s ended %s without the reaper's group-death proof: %v", job, status, record)
			}
			b.t.Logf("job %s ended %s (%v), group death proven %v", job, status, record["error"], record["groupDeathProvenAt"])
			return record
		}
		if time.Now().After(deadline) {
			b.t.Fatalf("job %s was never reaped: %s", job, output)
		}
		// Wait on the job through the engine's durable wait owner before
		// the next reap attempt.
		b.run(b.repo, nil, b.engine, "wait", "--root", b.repo, "--job", job, "--timeout", "5s")
	}
}

// TestIntentDesignRetryRealOwnerJourney drives review design FILE --retry N
// over the real dispatcher. The fake provider holds round 1 at its cap
// (FAKE:cap-hold-round=1): while its recorded supervisor and child live, a
// retry is refused and launches nothing. The real reaper ends the round at
// its cap and proves the group dead; the retry is then the chain's own
// round 2 through the real follow-up and examination-retry owner. Round 2
// loses its process (FAKE:process-loss) and is reaped dead the same way; the
// same retry again rejoins round 2; a retry of round 2 is refused at the
// frozen cap of 2, with no round 3 and no fresh root. Serial: the shared
// armed bed.
func TestIntentDesignRetryRealOwnerJourney(t *testing.T) {
	b := newDesignBed(t)
	goalID, page := "intent-design-retry", "plans/designs/intent-design-retry.md"
	pagePath := filepath.Join(b.repo, page)
	os.MkdirAll(filepath.Dir(pagePath), 0o755)
	os.WriteFile(pagePath, []byte("# Retry journey\n\n- Kind: design\n- Id: 01M3EFDSFTKWEMSDCP1BB7TRTY\n- Status: draft\n- Goals: "+goalID+"\n\nA design whose first examination is cut off.\n"), 0o644)
	for _, args := range [][]string{{"add", "--", page}, {"-c", "core.hooksPath=/dev/null", "-c", "user.name=metasystem", "-c", "user.email=metasystem@example.invalid", "commit", "-qm", "add the retry journey page"}} {
		if output, code := b.run(b.repo, nil, "git", append([]string{"-C", b.repo}, args...)...); code != 0 {
			t.Fatalf("git %v: %s", args, output)
		}
	}
	b.fixtureGoal(goalID, "Prove the public design examination retry")
	b.releaseOnFailure(goalID)
	marker, restore := b.fakeBehaviour()
	restored := false
	defer func() {
		if !restored {
			restore()
		}
	}()
	os.WriteFile(marker, []byte("FAKE:cap-hold-round=1\n"), 0o644)
	review := func(extra ...string) map[string]any {
		return b.cli(b.repo, nil, append([]string{"review", "design", page, "--tool-calls", "30"}, extra...)...)
	}

	first := review()
	expectExit(t, "the first review", first, 1)
	root := designResultData(first, "job")
	if root == "" {
		t.Fatalf("the first review started no critic: %v", first)
	}
	round1 := filepath.Join(b.repo, "artifacts", "agents", root, "rounds", "1")
	b.waitStatus(root, "running")
	b.waitFile(filepath.Join(round1, "child.pid"))
	jobs := b.jobs(root)
	live := review("--retry", "1")
	expectExit(t, "a retry while the examination runs", live, -1)
	if b.jobExists(root+"-r2") || b.jobs(root) != jobs || fmt.Sprint(b.job(root)["status"]) != "running" {
		t.Fatalf("a retry of a live examination launched or changed something: %v", live)
	}

	// The cap deadline passes (the fixture's established fault injection);
	// the real reaper ends the round and proves its group dead.
	if output, code := b.run(b.repo, nil, b.engine, "json", "set", "--file", filepath.Join(b.repo, "artifacts", "agents", "jobs", root+".json"),
		"--field", "capDeadline=2000-01-01T00:01:00Z"); code != 0 {
		t.Fatalf("cap deadline: %s", output)
	}
	capped := b.reapDead(root, round1)
	if capped["status"] != "timeout" || capped["error"] != "budget-cap" {
		t.Fatalf("round 1 did not end at its cap: %v", capped)
	}
	offered := review()
	next, _ := offered["next"].(map[string]any)
	if offered["outcome"] != "failed" || !strings.Contains(fmt.Sprint(next["argv"]), "--retry 1") {
		t.Fatalf("the capped examination does not offer its retry: %v", offered)
	}

	os.WriteFile(marker, []byte("FAKE:process-loss\n"), 0o644)
	retried := review("--retry", "1")
	expectExit(t, "the retry", retried, 1)
	child := designResultData(retried, "job")
	if child != root+"-r2" || fmt.Sprint(b.job(child)["parentJob"]) != root || b.jobs(root) != jobs+1 {
		t.Fatalf("the retry is not the chain's own round 2: %v", retried)
	}
	lostRound := b.reapDead(child, filepath.Join(b.repo, "artifacts", "agents", root, "rounds", "2"))
	if lostRound["status"] == "completed" {
		t.Fatalf("round 2 did not lose its process: %v", lostRound)
	}
	restore()
	restored = true

	again := review("--retry", "1")
	expectExit(t, "the repeated retry", again, -1)
	if again["outcome"] != "failed" || designResultData(again, "job") != child || b.jobs(root) != jobs+1 {
		t.Fatalf("repeating the retry did not rejoin failed round 2: %v", again)
	}
	beyond := review("--retry", "2")
	expectExit(t, "a retry past the frozen cap", beyond, -1)
	if b.jobExists(root+"-r3") || b.jobs(root) != jobs+1 {
		t.Fatalf("a retry past the frozen cap launched a round: %v", beyond)
	}
	if chains, _ := filepath.Glob(filepath.Join(b.repo, "artifacts", "agents", "intent-review", "design-01m3efdsftkwemsdcp1bb7trty", "chain.json")); len(chains) != 1 {
		t.Fatalf("the retry chain's review entry is missing")
	}
	b.release(goalID)
}
