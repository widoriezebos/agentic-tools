package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

const blockedQuestionSecret = "JBSWY3DPEHPK3PXP"

// blockedQuestionRoot is the remote-ledger seat fixture with the
// repository's fake channel provider configured and a synthetic TOTP
// secret; it returns the root and the provider's directory.
func blockedQuestionRoot(t *testing.T) (string, string) {
	t.Helper()
	root, _ := eventWaitRoot(t)
	providerDir, _ := commandFakeBed(t)
	conf := filepath.Join(root, "metasystem.conf")
	confBytes, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, conf, append(confBytes, []byte("\nchannel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir="+providerDir+"\nchannel.human.slack.user-id=human-a\n")...), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf.local"), []byte("channel.human.totp-secret="+blockedQuestionSecret+"\n"), 0o600)
	return root, providerDir
}

// postedQuestion waits for the wait's own poll to deliver the question to
// the provider, and returns the delivered record. The wait ending first is
// a failure; the only time bound is the test binary's deadline.
func postedQuestion(t *testing.T, root, id string, done <-chan struct{}) channel.Question {
	t.Helper()
	var posted channel.Question
	testenv.Await(t, "the wait's poll to deliver question "+id, func() bool {
		if q, err := channel.ReadQuestion(root, id); err == nil && q.Thread != nil {
			posted = q
			return true
		}
		select {
		case <-done:
			t.Fatal("the wait ended before its poll delivered the question")
		default:
		}
		return false
	})
	return posted
}

// replyInThread scripts the person's coded reply in the question's thread.
func replyInThread(t *testing.T, providerDir string, q channel.Question, text string) {
	t.Helper()
	thread := q.Thread.ThreadID
	if thread == "" {
		thread = q.Thread.ID
	}
	code, err := channel.TOTPCode(blockedQuestionSecret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	reply, _ := json.Marshal(map[string]any{"thread_ts": thread, "user": "human-a", "text": text + " " + code})
	writeTestingFixtureFile(t, filepath.Join(providerDir, "replies.jsonl"), append(reply, '\n'), 0o644)
}

// stopVerdict runs the Stop judgment the seat's hook runs, for one session
// of the fixture's announced main.
func stopVerdict(t *testing.T, root, session string) goal.Verdict {
	t.Helper()
	holder, err := lease.CurrentHolder(root)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := reportTurnVerdict(hooks.TurnVerdictRequest{Root: root, Session: session, MainID: holder.MainId}, &stdout, &stderr, nil, nil); code != 0 {
		t.Fatalf("turn verdict: code=%d stderr=%q", code, stderr.String())
	}
	var verdict goal.Verdict
	if err := json.Unmarshal(stdout.Bytes(), &verdict); err != nil {
		t.Fatalf("turn verdict: %v %q", err, stdout.String())
	}
	return verdict
}

// TestBlockedAgentAsksAndTheStopLetsItWait is the agent side end to end in
// a fixture seat: the session's Stop refuses on its claimed goal; the
// session asks through question ask and runs question wait in the
// background; the Stop now lets the turn end and names the question; the
// fake provider delivers the person's TOTP-valid reply; the wait exits 0
// printing the answer; and question list --answered shows it under its
// refusal.
func TestBlockedAgentAsksAndTheStopLetsItWait(t *testing.T) {
	root, providerDir := blockedQuestionRoot(t)
	owners := defaultIntentOwners()
	owners.resolver = stateroot.NewResolver(fakeTop(root), noExecutable)
	run := func(args ...string) (int, intentResult) {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentArgvCommand(t, args), append(intentArgvRest(args), "--json"), &stdout, &stderr, root, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Errorf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}

	if control := stopVerdict(t, root, "control-session"); !control.ShouldBlock {
		t.Fatalf("before the ask the Stop refuses on the claimed goal: %+v", control)
	}

	const refusal = "landing refused: the review hold stands"
	code, asked := run("question", "ask", "standing-validation", "--question", "Lift the review hold?", "--fact", refusal, "--fact", "tried: landing retry",
		"--option", "lift: land now", "--option", "keep: wait for the review")
	data, _ := asked.Data.(map[string]any)
	question, _ := data["question"].(map[string]any)
	id, _ := question["id"].(string)
	if id == "" || asked.Next == nil || strings.Join(asked.Next.Argv, " ") != "metasystem question wait channel:"+id {
		t.Fatalf("the ask: code=%d %+v", code, asked)
	}

	type waited struct {
		code   int
		result intentResult
	}
	finished := make(chan waited, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		code, result := run(append(asked.Next.Argv[1:], "--timeout", "4m")...)
		finished <- waited{code, result}
	}()
	posted := postedQuestion(t, root, id, done)

	verdict := stopVerdict(t, root, "asked-session")
	if verdict.ShouldBlock || !strings.Contains(verdict.Display, "WAITING: question "+id) {
		t.Fatalf("with the question open the Stop lets the turn end: %+v", verdict)
	}

	replyInThread(t, providerDir, posted, "lift")
	var answered waited
	testenv.Await(t, "the background wait to return the answer", func() bool {
		select {
		case answered = <-finished:
			return true
		default:
			return false
		}
	})
	ownerLines, _ := answered.result.Data.(map[string]any)["owner"].([]any)
	if answered.code != 0 || answered.result.Outcome != intentConfirmed || len(ownerLines) == 0 || fmt.Sprint(ownerLines[len(ownerLines)-1]) != "lift" {
		t.Fatalf("the wait returns the answer: code=%d %+v", answered.code, answered.result)
	}
	if after := stopVerdict(t, root, "answered-session"); !after.ShouldBlock || strings.Contains(after.Display, "WAITING: question") {
		t.Fatalf("answered, the Stop judges as before: %+v", after)
	}

	code, listed := run("question", "list", "--answered")
	groups := fmt.Sprint(listed.Data.(map[string]any)["groups"])
	if code != 0 || !strings.Contains(groups, refusal) || !strings.Contains(groups, "answered:1") {
		t.Fatalf("question list --answered shows the answer under its refusal: code=%d %s", code, groups)
	}
}

// TestGoallessQuestionWaitReturnsTheRecordedAnswer: question ask --about
// lane records a question with no goal; its wait registers the human wait
// of the session that waits, polls the channel, and returns the answer
// once it is recorded on the question record, ending its registration.
func TestGoallessQuestionWaitReturnsTheRecordedAnswer(t *testing.T) {
	root, providerDir := blockedQuestionRoot(t)
	owners := defaultIntentOwners()
	owners.resolver = stateroot.NewResolver(fakeTop(root), noExecutable)
	var askOut, askErr bytes.Buffer
	args := []string{"question", "ask", "--about", "lane", "--question", "Return the conflicting branch?", "--fact", "merge conflict in internal/goal", "--option", "return: its seat fixes it"}
	if code := runIntentIn(mustIntentArgvCommand(t, args), append(intentArgvRest(args), "--json"), &askOut, &askErr, root, owners); code != 0 {
		t.Fatalf("goal-less ask: code=%d %s %s", code, askOut.String(), askErr.String())
	}
	var asked intentResult
	if err := json.Unmarshal(askOut.Bytes(), &asked); err != nil {
		t.Fatal(err)
	}
	id, _ := asked.Data.(map[string]any)["question"].(map[string]any)["id"].(string)
	if q, err := channel.ReadQuestion(root, id); err != nil || q.Goal != "" || q.About != "lane" || q.Lineage != "m1" {
		t.Fatalf("the goal-less record: %+v %v", q, err)
	}

	var stdout, stderr bytes.Buffer
	exit := make(chan int, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		exit <- channelWaitWith(waitCallerPID(), "m1", &stdout, &stderr, []string{"--root", root, "--question", id, "--timeout", "4", "--poll-seconds", "1"}, nil)
	}()
	posted := postedQuestion(t, root, id, done)
	stateRoot, err := goal.ResolveStateRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	var humanWait func() (metarun.Waiter, bool)
	registered := func() bool {
		row, ok := humanWait()
		return ok && row.State == metarun.WaiterStatePending
	}
	humanWait = func() (metarun.Waiter, bool) {
		paths, _ := filepath.Glob(filepath.Join(metarun.WaitersDir(stateRoot), "*.json"))
		for _, path := range paths {
			var row metarun.Waiter
			if body, readErr := os.ReadFile(path); readErr == nil && json.Unmarshal(body, &row) == nil && row.Kind == "human" && row.Question == id {
				return row, true
			}
		}
		return metarun.Waiter{}, false
	}
	testenv.Await(t, "the waiting session's human wait to be registered", func() bool {
		if registered() {
			return true
		}
		select {
		case <-done:
			t.Fatalf("the wait ended before registering its human wait: stdout=%q stderr=%q", stdout.String(), stderr.String())
		default:
		}
		return false
	})
	replyInThread(t, providerDir, posted, "return it")
	var code int
	testenv.Await(t, "the goal-less wait to return", func() bool {
		select {
		case code = <-exit:
			return true
		default:
			return false
		}
	})
	if code != 0 || strings.TrimSpace(stdout.String()) != "return it" {
		t.Fatalf("goal-less wait: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if row, ok := humanWait(); !ok || row.State == metarun.WaiterStatePending {
		t.Fatalf("the human wait ends with the wait: %+v %t", row, ok)
	}
	if q, err := channel.ReadQuestion(root, id); err != nil || q.Answer == nil || q.Answer.Phase == "matched" {
		t.Fatalf("the answer is recorded on the record: %+v %v", q, err)
	}
}
