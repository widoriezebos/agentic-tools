package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// eventWaitRoot is the remote-ledger goal fixture prepared for the real wait
// owner: this process is its announced lease holder, and the holder's
// runtime adapter is the repository's own fake adapter, whose wait delivery
// answers for the fixture runtime.
func eventWaitRoot(t *testing.T) (string, string) {
	t.Helper()
	root, upstream, _ := goalBranchMainCLIFixtureBelow(t, "m1", ".")
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "m1")
	// The wait owner registers only for the holder's main session, which is
	// this test's parent; the fixture's own announcement names this process.
	announcements, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "mains", "*.json"))
	for _, path := range announcements {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	announceProofFixtureHolder(t, root)
	adapters := filepath.Join(root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fake.sh", "runtime-common.sh"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "adapters", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(filepath.Join(adapters, name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root, upstream
}

func runIntentEventWait(t *testing.T, root string, args ...string) (int, intentResult, string) {
	t.Helper()
	owners := defaultIntentOwners()
	owners.resolver = stateroot.NewResolver(fakeTop(root), noExecutable)
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, args[0]), append(args[1:], "--json"), &stdout, &stderr, root, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
	}
	return code, result, stderr.String()
}

// TestIntentWaitGoalEventGitAdapterObservesAPersonsAct: wait G --for
// human-act --verb V registers through the real wait owner and its default
// goal-event observer, which fetches the Git ledger: with no act yet the
// wait reaches its deadline with a durable record, wait resume continues
// that same record, and once a person's set-priority act is published by
// the real goal owner (existing fixture enrolled-human proof in this exact
// fake-runtime root) the resumed wait confirms on that act's ledger commit.
// Git is the claim because the observer fetches and validates the ledger
// history itself.
func TestIntentWaitGoalEventGitAdapterObservesAPersonsAct(t *testing.T) {
	root, _ := eventWaitRoot(t)
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	code, result, stderr := runIntentEventWait(t, root, "wait", "standing-validation", "--for", "human-act", "--verb", "set-priority", "--timeout", "7s")
	data, _ := result.Data.(map[string]any)
	id, _ := data["waitId"].(string)
	if result.Outcome != intentInProgress || id == "" || result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem wait resume "+id {
		t.Fatalf("human-act wait registration: code=%d %+v stderr=%q", code, result, stderr)
	}

	writeFixtureEnrollment(t, root, "Wido")
	args := []string{"--root", root, "--id", "standing-validation", "--by", "Wido", "--lineage", "m1", "--priority", "1", "--fixture-human-authority"}
	var stdout string
	actErr, actCode := captureStderr(t, func() int {
		var inner int
		stdout, inner = captureStdout(t, func() int {
			return runGoalSetPriorityWithAuthorityAndInputs(args, proveEnrolledGoalHumanAuthority, defaultSyncRequestDependencies())
		})
		return inner
	})
	if actCode != 0 || !strings.Contains(stdout, `"outcome":"confirmed"`) {
		t.Fatalf("the person's act: code=%d stdout=%q stderr=%q", actCode, stdout, actErr)
	}
	projection, err := goal.Project(endpoint, true, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	act := projection.Tree.Live["standing-validation"].History
	if last := act[len(act)-1]; last.Verb != "set-priority" || last.Actor != "human:Wido" {
		t.Fatalf("published act: %+v", last)
	}

	code, done, stderr := runIntentEventWait(t, root, "wait", "resume", id, "--timeout", "20s")
	doneData, _ := done.Data.(map[string]any)
	evidence, _ := doneData["sourceEvidence"].(string)
	if code != 0 || done.Outcome != intentConfirmed || doneData["waitId"] != id || doneData["exitCode"] != float64(0) ||
		!strings.Contains(evidence, projection.Tip) {
		t.Fatalf("the resumed wait after the act: code=%d %+v stderr=%q (act tip %s)", code, done, stderr, projection.Tip)
	}
}

// TestIntentWaitGoalLandingGitAdapterObservesTheRealLanding: wait G --for
// landing registers through the real wait owner and its default goal-event
// observer before the goal lands, reaching its deadline with a durable
// record. The public land command then lands the goal through the unchanged
// hand-landing owners of the whole-owner fixture (only the proof run is a
// fake effect), and wait resume continues the same record to the landing
// the observer finds on the fetched ledger. Git is the claim because both
// the landing and its observation are physical ledger history.
func TestIntentWaitGoalLandingGitAdapterObservesTheRealLanding(t *testing.T) {
	f := newWholeOwnerLanding(t)
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "m1")
	announcements, _ := filepath.Glob(filepath.Join(f.mainRoot, "artifacts", "agents", "mains", "*.json"))
	for _, path := range announcements {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	announceProofFixtureHolder(t, f.mainRoot)
	adapters := filepath.Join(f.mainRoot, "scripts", "agents", "adapters")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fake.sh", "runtime-common.sh"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "adapters", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(filepath.Join(adapters, name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(args ...string) (int, intentResult, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, "wait"), append(append(args, "--repo", f.mainRoot), "--json"), &stdout, &stderr, f.mainRoot, defaultIntentOwners())
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result, stderr.String()
	}
	code, result, stderr := wait("standing-validation", "--for", "landing", "--timeout", "7s")
	data, _ := result.Data.(map[string]any)
	id, _ := data["waitId"].(string)
	if result.Outcome != intentInProgress || id == "" || result.Next == nil || !slicesHasPrefix(result.Next.Argv, []string{"metasystem", "wait", "resume", id}) {
		t.Fatalf("landing wait registration: code=%d %+v stderr=%q", code, result, stderr)
	}

	code, landed := f.land(t)
	if code != 0 || landed.Outcome != intentConfirmed {
		t.Fatalf("land = %d %+v", code, landed)
	}
	_, prepared := f.retained(t, landed)
	f.assertLanded(t, prepared)

	code, done, stderr := wait("resume", id, "--timeout", "8s")
	doneData, _ := done.Data.(map[string]any)
	evidence, _ := doneData["sourceEvidence"].(string)
	if code != 0 || done.Outcome != intentConfirmed || doneData["waitId"] != id || doneData["sourceOutcome"] != "landing" ||
		!strings.Contains(evidence, prepared.Landing) {
		t.Fatalf("the resumed wait after the landing: code=%d %+v stderr=%q (landing %s)", code, done, stderr, prepared.Landing)
	}
}

// TestIntentWaitQuestionGitAdapterContinuesToTheRealAnswer: wait question Q
// runs the real channel wait owner (the freshly built engine, as its own
// process) with the adapter's exact argv. The channel provider is the
// repository's fake provider served for this test, configured in the
// synthetic root with a synthetic TOTP secret. With no reply the owner's
// first poll posts the question and its one controlled one-minute wait ends
// pending; the public result shows the continuation wait question Q. A
// person's coded reply in the question's thread is then scripted, and the
// shown continuation runs the owner again: its poll verifies the code,
// publishes the goal ledger's answer act through goal answer, and the wait
// owner matches that act. Once answered, wait question Q returns the answer
// immediately without an owner run. Git is the claim because the answer act
// is published to, and observed on, the fetched ledger.
func TestIntentWaitQuestionGitAdapterContinuesToTheRealAnswer(t *testing.T) {
	root, _ := eventWaitRoot(t)
	engine := intentTestEngine(t)
	providerDir, _ := commandFakeBed(t)
	const secret = "JBSWY3DPEHPK3PXP"
	conf := filepath.Join(root, "metasystem.conf")
	confBytes, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, conf, append(confBytes, []byte("\nchannel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir="+providerDir+"\nchannel.human.slack.user-id=human-a\n")...), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "metasystem.conf.local"), []byte("channel.human.totp-secret="+secret+"\n"), 0o600)
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := goal.Project(endpoint, true, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		t.Fatal(err)
	}
	question, err := channel.Ask(channel.AskRequest{RepoRoot: root, Goal: "standing-validation", Kind: "other", Machine: machine, Lineage: "m1",
		Facts: []string{"Proceed with the fixture?"}, LedgerCursor: before.Tip})
	if err != nil {
		t.Fatal(err)
	}

	var calls [][]string
	owners := defaultIntentOwners()
	owners.resolver = stateroot.NewResolver(fakeTop(root), noExecutable)
	owners.delivery = &intentDeliveryOwners{
		process: func(process intentProcess) intentProcessResult {
			calls = append(calls, append([]string(nil), process.argv...))
			return runIntentOwnerProcess(process)
		},
		executable: func() (string, error) { return engine, nil },
	}
	run := func(args ...string) (int, intentResult) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runIntentIn(mustIntentCommand(t, "wait"), append(args, "--json"), &stdout, &stderr, root, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("%v: %v %q %q", args, err, stdout.String(), stderr.String())
		}
		return code, result
	}

	code, pending := run("question", question.ID, "--timeout", "1m")
	want := []string{engine, "channel", "wait", "--root", root, "--question", question.ID, "--timeout", "1"}
	if len(calls) != 1 || !slicesEqual(calls[0], want) {
		t.Fatalf("owner argv = %v, want %v", calls, want)
	}
	if pending.Outcome != intentInProgress || code == 0 || pending.Next == nil ||
		!slicesEqual(pending.Next.Argv, []string{"metasystem", "wait", "question", "channel:" + question.ID}) {
		t.Fatalf("pending question wait: %d %+v", code, pending)
	}
	posted, err := channel.ReadQuestion(root, question.ID)
	if err != nil || posted.Thread == nil || posted.Answer != nil {
		t.Fatalf("the owner's poll did not post the question: %+v %v", posted, err)
	}

	thread := posted.Thread.ThreadID
	if thread == "" {
		thread = posted.Thread.ID
	}
	totp, err := channel.TOTPCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	reply, _ := json.Marshal(map[string]any{"thread_ts": thread, "user": "human-a", "text": "yes proceed " + totp})
	writeTestingFixtureFile(t, filepath.Join(providerDir, "replies.jsonl"), append(reply, '\n'), 0o644)

	// The public caller acts through the continuation it was shown.
	code, answered := run(pending.Next.Argv[2:]...)
	if len(calls) != 2 || !slicesEqual(calls[1], []string{engine, "channel", "wait", "--root", root, "--question", question.ID}) {
		t.Fatalf("continuation owner argv = %v", calls)
	}
	expectOutcome(t, "answered question wait", code, answered, intentConfirmed)
	ownerLines, _ := answered.Data.(map[string]any)["owner"].([]any)
	if answered.Summary != "channel question "+question.ID+" is answered" || len(ownerLines) == 0 ||
		fmt.Sprint(ownerLines[len(ownerLines)-1]) != "yes proceed" || !strings.Contains(fmt.Sprint(ownerLines...), `outcome="answer"`) {
		t.Fatalf("answered question wait: %+v", answered)
	}
	recorded, err := channel.ReadQuestion(root, question.ID)
	if err != nil || recorded.Answer == nil || recorded.Answer.Text != "yes proceed" || recorded.Answer.UserID != "human-a" {
		t.Fatalf("the owner's recorded answer: %+v %v", recorded, err)
	}
	after, err := goal.Project(endpoint, true, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	history := after.Tree.Live["standing-validation"].History
	last := history[len(history)-1]
	if after.Tip == before.Tip || last.Verb != "answer" || last.Question != question.ID || last.ChannelProvider == "" || last.ChannelUser != "human-a" ||
		last.Opid != recorded.Answer.Opid {
		t.Fatalf("the ledger's answer act: tip %s -> %s, %+v", before.Tip, after.Tip, last)
	}

	// Answered: the same public wait returns at once from the owner's record.
	code, immediate := run("question", question.ID)
	if len(calls) != 2 || code != 0 || immediate.Outcome != intentConfirmed {
		t.Fatalf("immediate answered wait: %d %+v (owner runs %d)", code, immediate, len(calls))
	}
}
