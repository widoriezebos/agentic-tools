package goal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"golang.org/x/sys/unix"
)

func TestLandingMessageAcceptsAttestedBranchProvenance(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("a", 40)
	message := "landed\n\nGoal-Item: goal-a\nLanding-Provenance: attested=" + commit + " goal=goal-a unit=u1 critic=critic-a/1 change=" + strings.Repeat("d", 64) + "\nLanding-Provenance-Verdict: pass bar=e\n"
	matched, provenance, err := matchLandingMessage(message, "goal-a", "")
	if err != nil || !matched {
		t.Fatalf("match=%t provenance=%q err=%v", matched, provenance, err)
	}
	if !strings.Contains(provenance, "attested="+commit) {
		t.Fatalf("provenance=%q", provenance)
	}
}

func TestGitAdapterLandingAtReadsCommitMessage(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", "README.md")
	mustGit(t, repo, "commit", "-qm", "seed")
	change := strings.Repeat("a", 64)
	mustGit(t, repo, "commit", "--allow-empty", "-qm", "land wait fixture", "--trailer", "Goal-Item: goal-a", "--trailer", "Landing-Provenance: chain=root-job change="+change, "--trailer", "Landing-Provenance-Verdict: pass bar=a")
	tip := mustGit(t, repo, "rev-parse", "HEAD")
	matched, provenance, err := landingAt(context.Background(), repo, tip, "goal-a", "root-job")
	if err != nil || !matched || provenance != "chain=root-job change="+change {
		t.Fatalf("landing match=%t provenance=%q err=%v", matched, provenance, err)
	}
	if matched, _, _ := landingAt(context.Background(), repo, tip, "goal-a", "other-chain"); matched {
		t.Fatal("landing from another chain matched")
	}
}

func TestGitAdapterInstalledGoalWaits(t *testing.T) {
	t.Parallel()
	binary := os.Getenv("METASYSTEM_WAIT_BINARY")
	if binary == "" {
		t.Skip("installed CLI adapter proof pending final independently built CLI")
	}
	t.Run("installed two-clone ledger waits", func(t *testing.T) {
		t.Parallel()
		binary := testutil.InstalledWaitBinary(t, binary)
		testInstalledGoalWaits(t, binary)
	})
}

func TestWaitGoalLandingAndHumanAct(t *testing.T) {
	t.Parallel()
	publisher, waiter := fakeGoalEndpointPair(t)
	client := publisher.Repository.(*fakeGoalRepository)
	repo := waiter.Root
	baseTip := acceptedTipForEndpoint(t, publisher)
	observe := func(tr *waitObservationTranscript, selector metarun.WaitSelector, pinned metarun.WaiterTarget, lastTip string) (metarun.SourceObservation, error) {
		t.Helper()
		ctx := withWaitGitDependencies(context.Background(), tr.dependencies())
		result, err := ObserveLedger(ctx, repo, selector, pinned, lastTip)
		tr.done()
		return result, err
	}
	open := func(id, ulid string) string {
		t.Helper()
		result, err := Open(verbReqFor(publisher, ulid, "mac-a"), id, "Wait fixture.", "main", "Wait for the recorded event.")
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
		return result.Tip
	}
	answer := func(id, question, ulid string) string {
		t.Helper()
		result, err := Answer(verbReqFor(publisher, ulid, "mac-a"), id, question, "yes", "", AnswerProof{Provider: "fake", User: "wido", Ref: "thread/message", Step: 1})
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("answer %s/%s: %+v %v", id, question, result, err)
		}
		return result.Tip
	}

	human := HistoryLine{Actor: "human:wido", Verb: "deny"}
	channel := HistoryLine{Actor: "human:wido", Verb: "answer", AuthorityOutcome: AuthorityOutcomeAuthenticatedChannelWord, Question: "question-a"}
	if !acceptedHumanAct(human) || !acceptedHumanAct(channel) || channel.Question != "question-a" {
		t.Fatal("accepted human history predicates did not retain the actual act")
	}

	answerCursor := open("answer-at-entry", "01J5X0000000000000000000X1")
	answerTip := answer("answer-at-entry", "question-entry", "01J5X0000000000000000000X2")
	client.store.mu.Lock()
	answerCommit := client.store.commits[answerTip]
	answerCommit.trailer = ""
	client.store.commits[answerTip] = answerCommit
	client.store.mu.Unlock()
	answerSelector := metarun.WaitSelector{Kind: "goal", TargetID: "answer-at-entry", GoalID: "answer-at-entry", Event: "human-act", Verb: "answer", Question: "question-entry", After: answerCursor}
	answerTranscript := newWaitObservationTranscript(t, waiter, client)
	answerTranscript.declare(answerCursor, baseTip)
	answerTranscript.declare(answerTip, answerCursor)
	if answerTranscript.commits[answerTip].trailer != "" {
		t.Fatal("answer fixture retained a Goal-Transaction trailer")
	}
	answerTranscript.endpoint("", "")
	answerTranscript.capture(answerTip)
	answerTranscript.acceptance(answerCursor, answerTip)
	answerTranscript.files(answerTip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	answerTranscript.changes(answerCursor, answerTip)
	answerTranscript.expect("mac-waiter\n", "config", "--get", "metasystem.goal.machine")
	answerTranscript.files(answerCursor, goalsPrefix, recordsGoalsPrefix)
	answerTranscript.cleanup()
	answerObservation, err := observe(answerTranscript, answerSelector, metarun.WaiterTarget{}, answerCursor)
	if err != nil || answerObservation.Pending || answerObservation.ExitCode != metarun.ExitGreen {
		t.Fatalf("answer recorded before registration was missed: %+v %v", answerObservation, err)
	}

	wrongCursor := open("wrong-then-right", "01J5X0000000000000000000X3")
	wrongTip := answer("wrong-then-right", "question-wrong", "01J5X0000000000000000000X4")
	rightSelector := metarun.WaitSelector{Kind: "goal", TargetID: "wrong-then-right", GoalID: "wrong-then-right", Event: "human-act", Verb: "answer", Question: "question-right", After: wrongCursor}
	wrongTranscript := newWaitObservationTranscript(t, waiter, client)
	wrongTranscript.declare(wrongCursor, answerTip)
	wrongTranscript.declare(wrongTip, wrongCursor)
	wrongTranscript.endpoint("", "")
	wrongTranscript.capture(wrongTip)
	wrongTranscript.acceptance(wrongCursor, wrongTip)
	wrongTranscript.files(wrongTip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	wrongTranscript.changes(wrongCursor, wrongTip)
	wrongTranscript.expect("mac-waiter\n", "config", "--get", "metasystem.goal.machine")
	wrongTranscript.files(wrongCursor, goalsPrefix, recordsGoalsPrefix)
	wrongTranscript.cleanup()
	wrongObservation, err := observe(wrongTranscript, rightSelector, metarun.WaiterTarget{}, wrongCursor)
	if err != nil || !wrongObservation.Pending {
		t.Fatalf("wrong question ended the wait: %+v %v", wrongObservation, err)
	}

	rightTip := answer("wrong-then-right", "question-right", "01J5X0000000000000000000X5")
	rightTranscript := newWaitObservationTranscript(t, waiter, client)
	rightTranscript.declare(wrongTip, wrongCursor)
	rightTranscript.declare(rightTip, wrongTip)
	rightTranscript.endpoint("", "")
	rightTranscript.capture(rightTip)
	rightTranscript.acceptance(wrongTip, rightTip)
	rightTranscript.files(rightTip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	rightTranscript.changes(wrongTip, rightTip)
	rightTranscript.expect("mac-waiter\n", "config", "--get", "metasystem.goal.machine")
	rightTranscript.files(wrongTip, goalsPrefix, recordsGoalsPrefix)
	rightTranscript.cleanup()
	rightObservation, err := observe(rightTranscript, rightSelector, wrongObservation.Incarnation, wrongObservation.LedgerTip)
	if err != nil || rightObservation.Pending || rightObservation.ExitCode != metarun.ExitGreen {
		t.Fatalf("right question did not end the wait: %+v %v", rightObservation, err)
	}

	landingCursor := open("landing-at-entry", "01J5X0000000000000000000X6")
	landingChange := strings.Repeat("b", 64)
	landingMessage := "land wait fixture\n\nGoal-Item: landing-at-entry\nLanding-Provenance: chain=root-job change=" + landingChange + "\nLanding-Provenance-Verdict: pass fixture\n"
	rootFiles, err := client.Files(landingCursor, goalsPrefix+"backlog.md")
	if err != nil || len(rootFiles[goalsPrefix+"backlog.md"]) == 0 {
		t.Fatalf("landing fixture root: %v", err)
	}
	landingTip, err := client.Build("landing-fixture", landingCursor, []Change{{Path: goalsPrefix + "backlog.md", Content: rootFiles[goalsPrefix+"backlog.md"]}}, landingMessage)
	if err != nil {
		t.Fatalf("build landing fixture: %v", err)
	}
	if outcome, err := client.Publish(landingCursor, landingTip); err != nil || outcome != CASLanded {
		t.Fatalf("publish landing fixture: %s %v", outcome, err)
	}
	landingSelector := metarun.WaitSelector{Kind: "goal", TargetID: "landing-at-entry", GoalID: "landing-at-entry", Event: "landing", Chain: "root-job", After: landingCursor}
	landingTranscript := newWaitObservationTranscript(t, waiter, client)
	landingTranscript.declare(landingCursor, rightTip)
	landingTranscript.declare(landingTip, landingCursor)
	landingTranscript.endpoint("", "")
	landingTranscript.expect("refs/remotes/origin/main\n", "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref")
	landingTranscript.capture(landingTip)
	landingTranscript.acceptance(landingCursor, landingTip)
	landingTranscript.files(landingTip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	landingTranscript.changes(landingCursor, landingTip)
	landingTranscript.files(landingCursor, goalsPrefix, recordsGoalsPrefix)
	landingTranscript.message(landingTip, landingMessage)
	landingTranscript.cleanup()
	landingObservation, err := observe(landingTranscript, landingSelector, metarun.WaiterTarget{}, landingCursor)
	if err != nil || landingObservation.Pending || landingObservation.ExitCode != metarun.ExitGreen ||
		landingObservation.Reason != "matching goal landing became reachable" ||
		landingObservation.Evidence != "commit:"+landingTip+":chain=root-job change="+landingChange {
		t.Fatalf("landing recorded before registration was missed: %+v %v", landingObservation, err)
	}

	localTip, err := client.Build("local-only", landingTip, []Change{{Path: goalsPrefix + "backlog.md", Content: rootFiles[goalsPrefix+"backlog.md"]}}, "local-only non-event")
	if err != nil {
		t.Fatalf("build local-only fixture: %v", err)
	}
	if localTip == landingTip || canonicalTipForTransactionTest(t, publisher) != landingTip {
		t.Fatal("unpublished local-only commit moved the canonical tip")
	}
	localSelector := metarun.WaitSelector{Kind: "goal", TargetID: "landing-at-entry", GoalID: "landing-at-entry", Event: "human-act", Verb: "edit", After: landingTip}
	localTranscript := newWaitObservationTranscript(t, waiter, client)
	localTranscript.declare(landingTip, landingCursor)
	localTranscript.endpoint("", "")
	localTranscript.capture(landingTip)
	localTranscript.acceptance(landingTip, landingTip)
	localTranscript.files(landingTip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	localTranscript.expect("mac-waiter\n", "config", "--get", "metasystem.goal.machine")
	localTranscript.cleanup()
	localObservation, err := observe(localTranscript, localSelector, metarun.WaiterTarget{}, landingTip)
	if err != nil || !localObservation.Pending {
		t.Fatalf("local-only commit affected the wait: %+v %v", localObservation, err)
	}

	next := "agent edit is not human authority"
	editResult, err := Edit(verbReqFor(publisher, "01J5X0000000000000000000X7", "mac-a"), "landing-at-entry", EditFields{NextStep: &next})
	if err != nil || editResult.Outcome != OutcomeConfirmed {
		t.Fatalf("agent edit: %+v %v", editResult, err)
	}
	editTranscript := newWaitObservationTranscript(t, waiter, client)
	editTranscript.declare(landingTip, landingCursor)
	editTranscript.declare(editResult.Tip, landingTip)
	editTranscript.endpoint("", "")
	editTranscript.capture(editResult.Tip)
	editTranscript.acceptance(landingTip, editResult.Tip)
	editTranscript.files(editResult.Tip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	editTranscript.changes(landingTip, editResult.Tip)
	editTranscript.expect("mac-waiter\n", "config", "--get", "metasystem.goal.machine")
	editTranscript.files(landingTip, goalsPrefix, recordsGoalsPrefix)
	editTranscript.cleanup()
	wrongAuthority, err := observe(editTranscript, localSelector, metarun.WaiterTarget{}, landingTip)
	if err != nil || !wrongAuthority.Pending {
		t.Fatalf("non-human authority ended the wait: %+v %v", wrongAuthority, err)
	}

	client.store.mu.Lock()
	client.store.canonical = landingTip
	client.store.mu.Unlock()
	rewindSelector := localSelector
	rewindSelector.After = editResult.Tip
	rewindTranscript := newWaitObservationTranscript(t, waiter, client)
	rewindTranscript.declare(editResult.Tip, landingTip)
	rewindTranscript.declare(landingTip, landingCursor)
	rewindTranscript.endpoint("", "")
	rewindTranscript.capture(landingTip)
	rewindTranscript.rewind(editResult.Tip, landingTip)
	rewindTranscript.cleanup()
	rewind, err := observe(rewindTranscript, rewindSelector, metarun.WaiterTarget{}, editResult.Tip)
	if err != nil || rewind.ExitCode != metarun.ExitNoRecord || !strings.Contains(rewind.Reason, "rewound") {
		t.Fatalf("rewind source failure = %+v %v", rewind, err)
	}
}

type installedGoalWaitResult struct {
	output string
	err    error
}

type installedGoalWaitProcess struct {
	command    *exec.Cmd
	done       chan installedGoalWaitResult
	registered chan error
}

func startInstalledGoalWait(t *testing.T, binary, root string, args ...string) *installedGoalWaitProcess {
	t.Helper()
	eventsPath := filepath.Join(root, "artifacts", "agents", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(eventsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(eventsPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(eventsPath, 0o600); err != nil {
		t.Fatal(err)
	}
	eventPipe, err := os.OpenFile(eventsPath, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	allArgs := append([]string{"wait", "--root", root}, args...)
	command := exec.Command(binary, allArgs...)
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		_ = eventPipe.Close()
		t.Fatal(err)
	}
	process := &installedGoalWaitProcess{
		command: command, done: make(chan installedGoalWaitResult, 1),
		registered: make(chan error, 1),
	}
	go func() {
		err := command.Wait()
		process.done <- installedGoalWaitResult{output: output.String(), err: err}
	}()
	go func() {
		decoder := json.NewDecoder(bufio.NewReader(eventPipe))
		for {
			var event struct {
				Event string `json:"event"`
			}
			if err := decoder.Decode(&event); err != nil {
				process.registered <- err
				return
			}
			if event.Event == "wait-registered" {
				process.registered <- nil
				return
			}
		}
	}()
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			<-process.done
		}
		_ = eventPipe.Close()
	})
	return process
}

func waitForInstalledGoalRow(t *testing.T, root, lineage, target string, process *installedGoalWaitProcess) metarun.Waiter {
	t.Helper()
	select {
	case result := <-process.done:
		t.Fatalf("installed goal waiter exited before it became pending: err=%v output=%s", result.err, result.output)
	case err := <-process.registered:
		if err != nil {
			t.Fatalf("read installed goal waiter registration signal: %v", err)
		}
	}
	rows, failures := metarun.PendingWaitersForLineages(root, []string{lineage})
	if len(failures) != 0 {
		t.Fatalf("read installed goal waiter: %v", failures)
	}
	for _, row := range rows {
		if row.TargetID == target {
			return row
		}
	}
	t.Fatalf("installed goal waiter signalled registration without a pending row for %s", target)
	return metarun.Waiter{}
}

func awaitInstalledGoalWait(t *testing.T, process *installedGoalWaitProcess, wantExit int) string {
	t.Helper()
	result := <-process.done
	exit := 0
	if result.err != nil {
		status, ok := result.err.(*exec.ExitError)
		if !ok {
			t.Fatalf("installed goal waiter failed without an exit status: %v output=%s", result.err, result.output)
		}
		exit = status.ExitCode()
	}
	if exit != wantExit {
		t.Fatalf("installed goal waiter exit=%d want=%d output=%s", exit, wantExit, result.output)
	}
	return result.output
}

func testInstalledGoalWaits(t *testing.T, binary string) {
	_, publisher, waiterClone := twoClones(t)
	seedLedger(t, publisher)
	mustGit(t, waiterClone, "config", "metasystem.goal.machine", "mac-waiter")
	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatalf("current process identity=%+v state=%s err=%v", exact, state, err)
	}
	const lineage = "installed-ledger-lineage"
	announce := exec.Command(binary, "lease", "announce", "--root", waiterClone, "--session", "installed-ledger-session", "--pid", strconv.FormatInt(self, 10), "--start", strconv.FormatInt(exact.StartedAt.Unix(), 10), "--start-ticks", strconv.FormatInt(exact.StartTicks, 10), "--boot-id", exact.BootID, "--tag", "installed-ledger-test", "--runtime", "fake", "--owner-lineage", lineage)
	if output, announceErr := announce.CombinedOutput(); announceErr != nil {
		t.Fatalf("announce installed ledger holder: %v %s", announceErr, output)
	}
	syncCursor := func(tip string) {
		t.Helper()
		mustGit(t, waiterClone, "fetch", "-q", "origin")
		mustGit(t, waiterClone, "update-ref", AcceptedRef, tip)
	}
	openGoal := func(id, ulid string) string {
		t.Helper()
		result, openErr := Open(verbReq(publisher, ulid, "mac-a"), id, "Installed ledger wait target.", "main", "Wait for a recorded event.")
		if openErr != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open installed wait target %s: %+v %v", id, result, openErr)
		}
		return result.Tip
	}

	actionTarget := "installed-actionable-target"
	actionCursor := openGoal(actionTarget, "01J5X0000000000000000000Y1")
	syncCursor(actionCursor)
	actionWait := startInstalledGoalWait(t, binary, waiterClone, "--goal", actionTarget, "--event", "human-act", "--verb", "deny", "--after", actionCursor, "--timeout", "1m", "--json")
	_ = waitForInstalledGoalRow(t, waiterClone, lineage, actionTarget, actionWait)
	// Reaching pending is the bounded proof that an unchanged frontier did
	// not end registration before the newly-ready goal is published.
	ready := "installed-newly-ready"
	_ = openGoal(ready, "01J5X0000000000000000000Y2")
	approveGoalForTest(t, verbReq(publisher, "01J5X0000000000000000000Y3", "mac-a"), ready, testBudget())
	if output := awaitInstalledGoalWait(t, actionWait, 6); !strings.Contains(output, "became claimable") {
		t.Fatalf("installed actionable result omitted the changed frontier: %s", output)
	}

	answerTarget := "installed-answer-target"
	answerCursor := openGoal(answerTarget, "01J5X0000000000000000000Y4")
	syncCursor(answerCursor)
	answerWait := startInstalledGoalWait(t, binary, waiterClone, "--goal", answerTarget, "--event", "human-act", "--verb", "answer", "--question", "installed-question", "--after", answerCursor, "--timeout", "1m", "--json")
	_ = waitForInstalledGoalRow(t, waiterClone, lineage, answerTarget, answerWait)
	answerResult, answerErr := Answer(verbReq(publisher, "01J5X0000000000000000000Y5", "mac-a"), answerTarget, "installed-question", "continue", "", AnswerProof{Provider: "fake", User: "wido", Ref: "thread/installed", Step: 1})
	if answerErr != nil || answerResult.Outcome != OutcomeConfirmed {
		t.Fatalf("publish installed answer: %+v %v", answerResult, answerErr)
	}
	if output := awaitInstalledGoalWait(t, answerWait, metarun.ExitGreen); !strings.Contains(output, `"exitCode":0`) {
		t.Fatalf("installed answer result was not green: %s", output)
	}
}

// mustGit runs git in dir or fails the test with the full output.
func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir,
		"-c", "user.name=t", "-c", "user.email=t@t",
		"-c", "protocol.file.allow=always"}, args...)...)
	cmd.Env = environWithoutGitSteering()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// cloneBed builds the fixture spine with one required clone and an optional
// second clone for tests that exercise concurrent publishers.
func cloneBed(t *testing.T, second bool) (origin, a, b string) {
	t.Helper()
	origin = filepath.Join(t.TempDir(), "origin.git")
	mustGit(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", origin)
	seed := filepath.Join(t.TempDir(), "seed")
	mustGit(t, t.TempDir(), "clone", "-q", origin, seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, seed, "add", "README.md", "metasystem.conf")
	mustGit(t, seed, "commit", "-qm", "seed")
	mustGit(t, seed, "push", "-q", "origin", "main")
	a = filepath.Join(t.TempDir(), "clone-a")
	mustGit(t, t.TempDir(), "clone", "-q", origin, a)
	if second {
		b = filepath.Join(t.TempDir(), "clone-b")
		mustGit(t, t.TempDir(), "clone", "-q", origin, b)
	}
	return origin, a, b
}

func oneClone(t *testing.T) (origin, clone string) {
	t.Helper()
	origin, clone, _ = cloneBed(t, false)
	return origin, clone
}

// twoClones builds two independent publishers against the same bare origin.
func twoClones(t *testing.T) (origin, a, b string) {
	t.Helper()
	return cloneBed(t, true)
}

func endpointFor(root string) Endpoint {
	return Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main"}
}

func goalChange(id, body string) Change {
	return Change{Path: "plans/goals/" + id + ".md", Content: []byte(body)}
}

func validGoalChange(id string) Change {
	return Change{Path: livePath(id), Content: RenderFile(vGoal(id, StateQueued))}
}

func claimedGoalChange(id, machine string) Change {
	goal := vGoal(id, StateClaimed)
	goal.Claimed.Machine = machine
	return Change{Path: livePath(id), Content: RenderFile(goal)}
}

func canonicalTipForTransactionTest(t *testing.T, e Endpoint) string {
	t.Helper()
	client, ok := e.Repository.(*fakeGoalRepository)
	if !ok {
		t.Fatal("canonical-tip check needs a fake goal endpoint")
	}
	client.store.mu.Lock()
	defer client.store.mu.Unlock()
	return client.store.canonical
}

func TestPublishLandsWithParentAndTrailer(t *testing.T) {
	t.Parallel()
	origin, a, _ := twoClones(t)
	e := endpointFor(a)
	oldTip := mustGit(t, a, "rev-parse", "origin/main")
	headBefore := mustGit(t, a, "rev-parse", "HEAD")

	res, err := Publish(e, PublishRequest{
		Opid: "op-land", Machine: "mac-a", Lineage: "l1",
		Intent:  testIntentFor("open"),
		Message: "goal open fix-it",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{goalChange("fix-it", "# fix-it\nState: queued\n")}, nil
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the publish must confirm: %+v %v", res, err)
	}

	remoteTip := mustGit(t, origin, "rev-parse", "refs/heads/main")
	if remoteTip != res.Commit {
		t.Fatalf("the remote branch advances to the transaction commit: %s vs %s", remoteTip, res.Commit)
	}
	parent := mustGit(t, origin, "rev-parse", remoteTip+"^")
	if parent != oldTip {
		t.Fatalf("the commit's parent is exactly the captured tip (R3-02): %s vs %s", parent, oldTip)
	}
	body := mustGit(t, origin, "log", "-1", "--format=%B", remoteTip)
	if !strings.Contains(body, "Goal-Transaction: op-land") {
		t.Fatalf("the provenance trailer rides the commit: %q", body)
	}
	// The user's world never moved.
	if mustGit(t, a, "rev-parse", "HEAD") != headBefore {
		t.Fatal("HEAD must never move")
	}
	// The journal writes its machine-local record under artifacts/
	// (gitignored in the real repository); TRACKED state is what
	// must never move.
	if status := mustGit(t, a, "status", "--porcelain", "--untracked-files=no"); status != "" {
		t.Fatalf("the tracked worktree must stay untouched: %q", status)
	}
	// Journal terminal, temp refs cleaned, accepted advanced.
	entry, err := ReadEntry(a, "op-land")
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeConfirmed {
		t.Fatalf("the journal closes confirmed: %+v %v", entry, err)
	}
	if _, err := gitIn(a, "rev-parse", "--verify", "--quiet", fetchRefFor("op-land")); err == nil {
		t.Fatal("the fetch ref dies at terminal")
	}
	if _, err := gitIn(a, "rev-parse", "--verify", "--quiet", txnRefFor("op-land")); err == nil {
		t.Fatal("the txn ref dies at terminal")
	}
	accepted := mustGit(t, a, "rev-parse", AcceptedRef)
	if accepted != res.Commit {
		t.Fatalf("the accepted ref advances on confirmation: %s vs %s", accepted, res.Commit)
	}
}

func TestWaitPublishedAtOwners(t *testing.T) {
	t.Parallel()
	e, _ := fakeGoalEndpoint(t)
	hinted := false
	res, err := Publish(e, PublishRequest{
		Opid: "op-hint", Machine: "mac-a", Lineage: "l1",
		Intent:  Intent{Verb: "edit", Targets: []string{"goal-a"}},
		Message: "goal edit goal-a",
		Mutate: func(string) ([]Change, error) {
			return []Change{validGoalChange("goal-a")}, nil
		},
		bootClock: func() (string, time.Duration, error) { return "goal-boot", 4 * time.Hour, nil },
		HintConfirmed: func(root string, targets []string, publicationID, bootID string, bootNanos int64) {
			if root != e.Root || strings.Join(targets, ",") != "goal-a" {
				t.Fatalf("hint target root=%q targets=%v", root, targets)
			}
			tip := canonicalTipForTransactionTest(t, e)
			present, trailerErr := TrailerPresent(e, tip, "op-hint")
			if trailerErr != nil || !present {
				t.Fatalf("hint preceded confirmed canonical publication: tip=%s present=%t err=%v", tip, present, trailerErr)
			}
			entry, entryErr := ReadEntry(e.Root, "op-hint")
			if entryErr != nil || entry.Phase != PhasePushed {
				t.Fatalf("hint moved or preceded the publication boundary: entry=%+v err=%v", entry, entryErr)
			}
			if publicationID != tip {
				t.Fatalf("publication identity = %q, want ledger tip %q", publicationID, tip)
			}
			if bootID != "goal-boot" || bootNanos != int64(4*time.Hour) {
				t.Fatalf("boot sample = %s/%d", bootID, bootNanos)
			}
			hinted = true
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed || !hinted {
		t.Fatalf("result=%+v err=%v hinted=%t", res, err, hinted)
	}
	if canonicalTipForTransactionTest(t, e) != res.Commit || acceptedTipForEndpoint(t, e) != res.Commit {
		t.Fatalf("confirmed hint did not advance canonical and accepted tips to %s", res.Commit)
	}

	first, competitor := fakeGoalEndpointPair(t)
	clockReads := 0
	bootClock := func() (string, time.Duration, error) {
		clockReads++
		return "goal-boot", time.Duration(clockReads+4) * time.Hour, nil
	}
	attempts := 0
	var retrySample int64
	retried, err := Publish(first, PublishRequest{
		Opid: "op-retry-hint", Machine: "mac-a", Lineage: "l1", Intent: testIntentFor("open"), Message: "goal open retry-hint",
		Mutate: func(string) ([]Change, error) {
			return []Change{validGoalChange("retry-hint")}, nil
		},
		bootClock: bootClock,
		BeforePush: func(attempt int) error {
			attempts = attempt
			if attempt != 1 {
				return nil
			}
			other, otherErr := Publish(competitor, PublishRequest{
				Opid: "op-retry-competitor", Machine: "mac-b", Lineage: "l1", Intent: testIntentFor("open"), Message: "goal open retry-other",
				bootClock: bootClock,
				Mutate: func(string) ([]Change, error) {
					return []Change{validGoalChange("retry-other")}, nil
				},
			})
			if otherErr != nil || other.Outcome != OutcomeConfirmed {
				return fmt.Errorf("competitor publish: %+v %v", other, otherErr)
			}
			return nil
		},
		HintConfirmed: func(_ string, _ []string, _ string, bootID string, bootNanos int64) {
			if bootID != "goal-boot" {
				t.Fatalf("retry boot identity = %q", bootID)
			}
			retrySample = bootNanos
		},
	})
	if err != nil || retried.Outcome != OutcomeConfirmed || attempts < 2 || retrySample != int64(5*time.Hour) || clockReads != 2 {
		t.Fatalf("retry result=%+v err=%v attempts=%d sample=%d clockReads=%d", retried, err, attempts, retrySample, clockReads)
	}
	if canonicalTipForTransactionTest(t, first) != retried.Commit || acceptedTipForEndpoint(t, first) != retried.Commit || acceptedTipForEndpoint(t, competitor) == retried.Commit {
		t.Fatalf("retry tips: canonical=%s first accepted=%s competitor accepted=%s result=%+v",
			canonicalTipForTransactionTest(t, first), acceptedTipForEndpoint(t, first), acceptedTipForEndpoint(t, competitor), retried)
	}

	zeroSample := int64(-1)
	alreadyEndpoint, _ := fakeGoalEndpoint(t)
	already, err := Publish(alreadyEndpoint, PublishRequest{
		Opid: "op-already-hint", Machine: "mac-a", Lineage: "l1", Intent: testIntentFor("open"), Message: "goal open already-hint",
		Mutate: func(string) ([]Change, error) { return nil, AlreadyApplied{} },
		HintConfirmed: func(_ string, _ []string, _ string, bootID string, bootNanos int64) {
			if bootID != "" {
				t.Fatalf("already-applied hint carried boot identity %q", bootID)
			}
			zeroSample = bootNanos
		},
	})
	if err != nil || already.Outcome != OutcomeConfirmed || already.Detail != "idempotent" || zeroSample != 0 {
		t.Fatalf("already-applied result=%+v err=%v sample=%d", already, err, zeroSample)
	}
	if already.Tip != canonicalTipForTransactionTest(t, alreadyEndpoint) || acceptedTipForEndpoint(t, alreadyEndpoint) != already.Tip {
		t.Fatalf("already-applied tips changed: result=%+v canonical=%s accepted=%s", already,
			canonicalTipForTransactionTest(t, alreadyEndpoint), acceptedTipForEndpoint(t, alreadyEndpoint))
	}
}

func TestSameTargetRaceExactlyOneWins(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)
	oldAcceptedB := acceptedTipForEndpoint(t, b)

	// A wins the target first.
	resA, err := Publish(a, PublishRequest{
		Opid: "op-winner", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("claim"), Message: "goal claim fix-it",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{claimedGoalChange("fix-it", "mac-a")}, nil
		},
	})
	if err != nil || resA.Outcome != OutcomeConfirmed {
		t.Fatalf("A confirms: %+v %v", resA, err)
	}
	if canonicalTipForTransactionTest(t, a) != resA.Commit || acceptedTipForEndpoint(t, a) != resA.Commit || acceptedTipForEndpoint(t, b) != oldAcceptedB {
		t.Fatalf("winner's canonical and accepted tips diverged: result=%+v A=%s B=%s", resA,
			acceptedTipForEndpoint(t, a), acceptedTipForEndpoint(t, b))
	}

	// B captures A's canonical commit while its own accepted ref still
	// points to the seed. Its mutation reads the committed winner.
	resB, err := Publish(b, PublishRequest{
		Opid: "op-loser", Machine: "mac-b", Lineage: "l1",
		Intent: testIntentFor("claim"), Message: "goal claim fix-it",
		Mutate: func(tip string) ([]Change, error) {
			files, readErr := readCommitFiles(b, tip, livePath("fix-it"))
			if readErr != nil {
				return nil, readErr
			}
			if data := files[livePath("fix-it")]; len(data) != 0 {
				goal, problems := ParseFile(data)
				if len(problems) != 0 {
					return nil, fmt.Errorf("committed winner goal: %v", problems)
				}
				if goal.Claimed != nil && goal.Claimed.Machine == "mac-a" {
					return nil, LostToCompetitor{Winner: "op-winner"}
				}
			}
			return []Change{claimedGoalChange("fix-it", "mac-b")}, nil
		},
	})
	if err != nil || resB.Outcome != OutcomeLost {
		t.Fatalf("B classifies the loss, reverting nothing: %+v %v", resB, err)
	}
	if !strings.Contains(resB.Detail, "op-winner") {
		t.Fatalf("the loser names the winner: %+v", resB)
	}
	entry, _ := ReadEntry(b.Root, "op-loser")
	if entry.Outcome != OutcomeLost || !strings.Contains(entry.Evidence, "op-winner") {
		t.Fatalf("the journal carries the winner's opid: %+v", entry)
	}
	if canonicalTipForTransactionTest(t, b) != resA.Commit || acceptedTipForEndpoint(t, b) != oldAcceptedB {
		t.Fatalf("loser moved a tip: canonical=%s accepted=%s winner=%s", canonicalTipForTransactionTest(t, b),
			acceptedTipForEndpoint(t, b), resA.Commit)
	}
}

func TestLeaseRefusalOnMidflightCompetitor(t *testing.T) {
	t.Parallel()
	e, b := fakeGoalEndpointPair(t)
	oldAccepted := acceptedTipForEndpoint(t, e)

	// Between A's capture and push, B lands a same-target commit.
	// A's compare-and-swap refuses; the rebuild classifies the loss.
	injected := false
	res, err := Publish(e, PublishRequest{
		Opid: "op-cas", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("claim"), Message: "goal claim fix-it",
		Mutate: func(tip string) ([]Change, error) {
			files, readErr := readCommitFiles(e, tip, livePath("fix-it"))
			if readErr != nil {
				return nil, readErr
			}
			if data := files[livePath("fix-it")]; len(data) != 0 {
				goal, problems := ParseFile(data)
				if len(problems) != 0 {
					return nil, fmt.Errorf("committed competitor goal: %v", problems)
				}
				if goal.Claimed != nil && goal.Claimed.Machine == "mac-b" {
					return nil, LostToCompetitor{Winner: "op-b"}
				}
			}
			return []Change{claimedGoalChange("fix-it", "mac-a")}, nil
		},
		BeforePush: func(attempt int) error {
			if injected {
				return nil
			}
			injected = true
			resB, bErr := Publish(b, PublishRequest{
				Opid: "op-b", Machine: "mac-b", Lineage: "l1",
				Intent: testIntentFor("claim"), Message: "goal claim fix-it",
				Mutate: func(tip string) ([]Change, error) {
					return []Change{claimedGoalChange("fix-it", "mac-b")}, nil
				},
			})
			if bErr != nil || resB.Outcome != OutcomeConfirmed {
				return fmt.Errorf("the competitor must land: %+v %v", resB, bErr)
			}
			return nil
		},
	})
	if err != nil || res.Outcome != OutcomeLost {
		t.Fatalf("the lease refuses and the rebuild classifies the loss: %+v %v", res, err)
	}
	if !injected || !strings.Contains(res.Detail, "op-b") {
		t.Fatalf("midflight loss omitted competitor identity: injected=%t result=%+v", injected, res)
	}
	entry, entryErr := ReadEntry(e.Root, "op-cas")
	if entryErr != nil || entry.Outcome != OutcomeLost || !strings.Contains(entry.Evidence, "op-b") {
		t.Fatalf("midflight loss was not journaled by winner: entry=%+v err=%v", entry, entryErr)
	}
	if canonicalTipForTransactionTest(t, e) != acceptedTipForEndpoint(t, b) || acceptedTipForEndpoint(t, e) != oldAccepted {
		t.Fatalf("midflight tips: canonical=%s A accepted=%s B accepted=%s", canonicalTipForTransactionTest(t, e),
			acceptedTipForEndpoint(t, e), acceptedTipForEndpoint(t, b))
	}
}

func TestBenignAdvancementRetriesWithinTheDeadline(t *testing.T) {
	t.Parallel()
	a, b := fakeGoalEndpointPair(t)

	// The competitor's commit touches a DIFFERENT goal: lawful work,
	// not failure. A's first push refuses on the lease, the
	// rebuild carries the same change, the second push lands.
	injected := false
	attempts := 0
	res, err := Publish(a, PublishRequest{
		Opid: "op-benign", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open mine",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{validGoalChange("mine")}, nil
		},
		BeforePush: func(attempt int) error {
			attempts = attempt
			if injected {
				return nil
			}
			injected = true
			resB, bErr := Publish(b, PublishRequest{
				Opid: "op-other", Machine: "mac-b", Lineage: "l1",
				Intent: testIntentFor("open"), Message: "goal open other",
				Mutate: func(tip string) ([]Change, error) {
					return []Change{validGoalChange("other")}, nil
				},
			})
			if bErr != nil || resB.Outcome != OutcomeConfirmed {
				return fmt.Errorf("the unrelated competitor must land: %+v %v", resB, bErr)
			}
			return nil
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("benign advancement retries and lands: %+v %v", res, err)
	}
	if attempts < 2 {
		t.Fatalf("the first lease must have refused: attempts=%d", attempts)
	}
	// Both goals live on the canonical branch, while B's accepted ref
	// remains at its own confirmed publication.
	tip := canonicalTipForTransactionTest(t, a)
	if tip != res.Commit || acceptedTipForEndpoint(t, a) != tip || acceptedTipForEndpoint(t, b) == tip {
		t.Fatalf("benign retry tips: canonical=%s A accepted=%s B accepted=%s result=%+v", tip,
			acceptedTipForEndpoint(t, a), acceptedTipForEndpoint(t, b), res)
	}
	for _, id := range []string{"mine", "other"} {
		files, readErr := readCommitFiles(a, tip, livePath(id))
		if readErr != nil || len(files[livePath(id)]) == 0 {
			t.Fatalf("goal %s must be on the canonical branch: %v", id, readErr)
		}
	}
}

func TestPublishRetryDeadlineUsesInjectedClock(t *testing.T) {
	t.Parallel()
	publisher, competitor := fakeGoalEndpointPair(t)
	start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	clockReads := 0
	beforePushCalls := 0
	deadline := time.Hour
	result, err := Publish(publisher, PublishRequest{
		Opid: "op-clock-deadline", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open clock-deadline",
		Mutate: func(string) ([]Change, error) {
			return []Change{validGoalChange("clock-deadline")}, nil
		},
		Deadline: deadline,
		now: func() time.Time {
			clockReads++
			if clockReads == 1 {
				return start
			}
			return start.Add(deadline + time.Nanosecond)
		},
		BeforePush: func(attempt int) error {
			beforePushCalls++
			if attempt != 1 {
				return fmt.Errorf("expired transaction reached attempt %d", attempt)
			}
			other, otherErr := Publish(competitor, PublishRequest{
				Opid: "op-clock-competitor", Machine: "mac-b", Lineage: "l1",
				Intent: testIntentFor("open"), Message: "goal open clock-competitor",
				Mutate: func(string) ([]Change, error) {
					return []Change{validGoalChange("clock-competitor")}, nil
				},
			})
			if otherErr != nil || other.Outcome != OutcomeConfirmed {
				return fmt.Errorf("competitor publish: %+v %v", other, otherErr)
			}
			return nil
		},
	})
	if err != nil || result.Outcome != OutcomeExpired || result.Detail != "1 attempts" {
		t.Fatalf("fake deadline result=%+v err=%v", result, err)
	}
	if clockReads != 2 || beforePushCalls != 1 {
		t.Fatalf("retry deadline clock reads=%d before-push calls=%d, want 2 and 1", clockReads, beforePushCalls)
	}
	if canonicalTipForTransactionTest(t, publisher) != acceptedTipForEndpoint(t, competitor) || acceptedTipForEndpoint(t, publisher) == canonicalTipForTransactionTest(t, publisher) {
		t.Fatalf("expired publisher moved an accepted tip: canonical=%s publisher=%s competitor=%s",
			canonicalTipForTransactionTest(t, publisher), acceptedTipForEndpoint(t, publisher), acceptedTipForEndpoint(t, competitor))
	}
}

func TestPushedEntryBlocksTheWholeClone(t *testing.T) {
	t.Parallel()
	a, client := fakeGoalEndpoint(t)
	tipBefore := canonicalTipForTransactionTest(t, a)
	// A stranded pushed entry (a crashed process's) blocks every new
	// mutation on this clone until classified — process-independent.
	if _, err := CreateEntry(a.Root, "op-stuck", "mac-a", "l1", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	if err := MarkPushed(a.Root, "op-stuck", "sometip", 1, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	mutated := false
	_, err := Publish(a, PublishRequest{
		Opid: "op-next", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open x",
		Mutate: func(tip string) ([]Change, error) {
			mutated = true
			return []Change{goalChange("x", "# x\n")}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "op-stuck") || mutated || canonicalTipForTransactionTest(t, a) != tipBefore {
		t.Fatalf("a pushed entry blocks own-clone mutations by name: err=%v mutated=%t", err, mutated)
	}
	if len(client.captures) != 0 {
		t.Fatalf("a pushed entry reached repository capture: captures=%v", client.captures)
	}
}

func TestValidationRefusalIsRejectedByName(t *testing.T) {
	t.Parallel()
	a, _ := fakeGoalEndpoint(t)
	tipBefore := canonicalTipForTransactionTest(t, a)
	res, err := Publish(a, PublishRequest{
		Opid: "op-invalid", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open bad",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{goalChange("bad", "not a goal file\n")}, nil
		},
		Validate: func(commit string) error {
			validationErr := validateCommitFor(a, commit)
			if validationErr == nil {
				return nil
			}
			files, readErr := readCommitFiles(a, commit, livePath("bad"))
			if readErr != nil {
				return readErr
			}
			if data, exists := files[livePath("bad")]; exists && !bytes.Contains(data, []byte("State:")) {
				return fmt.Errorf("%s: State missing: %w", livePath("bad"), validationErr)
			}
			return validationErr
		},
	})
	if err != nil || res.Outcome != OutcomeRejected {
		t.Fatalf("a validation refusal is a definite rejection: %+v %v", res, err)
	}
	entry, _ := ReadEntry(a.Root, "op-invalid")
	if entry.Outcome != OutcomeRejected || !strings.Contains(entry.Evidence, "State missing") {
		t.Fatalf("the rejection is journaled by name: %+v", entry)
	}
	if canonicalTipForTransactionTest(t, a) != tipBefore || acceptedTipForEndpoint(t, a) != tipBefore {
		t.Fatalf("rejected transaction moved canonical or accepted tip: before=%s canonical=%s accepted=%s",
			tipBefore, canonicalTipForTransactionTest(t, a), acceptedTipForEndpoint(t, a))
	}
	files, readErr := readCommitFiles(a, tipBefore, livePath("bad"))
	if readErr != nil || len(files) != 0 {
		t.Fatalf("rejected transaction published bad goal: files=%v err=%v", files, readErr)
	}
}

func TestSingleMachineModeCASNeverMovesHead(t *testing.T) {
	t.Parallel()
	repo := filepath.Join(t.TempDir(), "solo")
	mustGit(t, t.TempDir(), "init", "-q", "-b", "main", repo)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("solo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", "README.md")
	mustGit(t, repo, "commit", "-qm", "seed")
	// The dedicated ledger branch — never the user's checked-out
	// branch (migration/adoption owns real bootstrap; the fixture
	// seeds it directly).
	mustGit(t, repo, "update-ref", LocalLedgerBranch, "HEAD")
	headBefore := mustGit(t, repo, "rev-parse", "HEAD")

	e := Endpoint{Root: repo, Remote: "local", Branch: "refs/heads/main"}
	res, err := Publish(e, PublishRequest{
		Opid: "op-solo", Machine: "mac-solo", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open here",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{goalChange("here", "# here\nState: queued\n")}, nil
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("local mode confirms by update-ref CAS: %+v %v", res, err)
	}
	ledger := mustGit(t, repo, "rev-parse", LocalLedgerBranch)
	if ledger != res.Commit {
		t.Fatalf("the ledger branch advances: %s vs %s", ledger, res.Commit)
	}
	if mustGit(t, repo, "rev-parse", "HEAD") != headBefore {
		t.Fatal("HEAD provably never moves in either mode")
	}
	if ok, err := TrailerPresent(e, ledger, "op-solo"); err != nil || !ok {
		t.Fatalf("the trailer postcondition resolves on the ledger branch: %v %v", ok, err)
	}
}

func TestPushFailureClassifierSeparatesLeaseFromTransport(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		output string
		want   CASOutcome
	}{
		{"! [rejected] main -> main (stale info)", CASRefused},
		{"! [rejected] abc..def main -> main (non-fast-forward)", CASRefused},
		{"hint: Updates were rejected... fetch first", CASRefused},
		{"fatal: unable to access 'https://x/': Could not resolve host", CASUnknown},
		{"ssh: connect to host x port 22: Connection timed out", CASUnknown},
	} {
		if got := classifyPushFailure(c.output); got != c.want {
			t.Fatalf("%q: got %s want %s", c.output, got, c.want)
		}
	}
}

func TestTransactionsAddressTheTreeUnderASubdirectoryRoot(t *testing.T) {
	t.Parallel(
	// The REAL repository's shape: the goal root sits one level below
	// the git toplevel. Every rev:path read and raw index write must
	// address the tree under that prefix — the toplevel-rooted forms
	// pass every toplevel-rooted fixture and then refuse (or worse,
	// write beside the ledger) on the deployment layout itself.
	)

	origin, a, _ := twoClones(t)
	_ = origin
	sub := filepath.Join(a, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	seedLedger(t, sub)
	res, err := Open(verbReq(sub, "01J5X00000000000000000SD00", "mac-a"), "deep-goal", "Prefixed world.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open under a subdirectory root: %+v %v", res, err)
	}
	// The write landed under the PREFIX at the toplevel view.
	out := mustGit(t, a, "ls-tree", "-r", "--name-only", res.Tip)
	if !strings.Contains(out, "nested/plans/goals/deep-goal.md") {
		t.Fatalf("the tree entry is not under the root's prefix:\n%s", out)
	}
	// The read side resolves the same world back.
	tree, err := loadTree(sub, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["deep-goal"] == nil {
		t.Fatalf("the prefixed ledger does not read back: %+v", tree.Live)
	}
}

func TestABrokenAcceptedRefRefusesMutations(t *testing.T) {
	t.Parallel()
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// The accepted ref EXISTS but points at a blob: identity and
	// descent cannot be gated, and skipping them because the ref
	// "did not resolve" is exactly the fail-open the gates forbid.
	oid, err := hashObject(a, []byte("not a commit"))
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, a, "update-ref", AcceptedRef, oid)
	_, openErr := Open(verbReq(a, "01J5X00000000000000000BA00", "mac-a"), "gated-out", "Blocked.", "main", "Go.")
	if openErr == nil || !strings.Contains(openErr.Error(), "does not resolve to a commit") {
		t.Fatalf("a broken accepted ref refuses the mutation by name: %v", openErr)
	}
}

func TestAMalformedAcceptedRefFileRefusesMutations(t *testing.T) {
	t.Parallel()
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// git WARNS and ignores a broken loose ref (exit 1, same as
	// absent): the file's existence is the only tell, and reading it
	// as pre-bootstrap would skip identity and descent.
	refFile := filepath.Join(a, ".git", "refs", "metasystem", "goals", "accepted")
	if err := os.MkdirAll(filepath.Dir(refFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(refFile, []byte("garbagenothex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// End to end, remote mode already refuses — git's own fetch dies
	// enumerating the corrupt ref. The GATE's arm is what needs the
	// direct proof: absent and broken must answer differently.
	_, has, gateErr := acceptedTipForGates(a)
	if has || gateErr == nil || !strings.Contains(gateErr.Error(), "no valid ref") {
		t.Fatalf("a broken accepted-ref file refuses by name at the gate: %v (has=%v)", gateErr, has)
	}
}

func TestADanglingAcceptedRefSymlinkRefusesMutations(t *testing.T) {
	t.Parallel()
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	refFile := filepath.Join(a, ".git", "refs", "metasystem", "goals", "accepted")
	if err := os.MkdirAll(filepath.Dir(refFile), 0o755); err != nil {
		t.Fatal(err)
	}
	// The seed's own accepted ref steps aside first: the bed needs
	// the BROKEN shape at the path.
	_ = os.Remove(refFile)
	// A DANGLING symlink: git ignores it like an absent ref, a
	// following stat sees nothing — only Lstat tells the truth.
	if err := os.Symlink(filepath.Join(a, "no-such-target"), refFile); err != nil {
		t.Fatal(err)
	}
	_, has, gateErr := acceptedTipForGates(a)
	if has || gateErr == nil || !strings.Contains(gateErr.Error(), "no valid ref") {
		t.Fatalf("a dangling ref symlink refuses at the gate: %v (has=%v)", gateErr, has)
	}
}

func TestGoalGitParsesStdoutCleanOfStderrWarnings(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git-calls.log")
	wantTip := strings.Repeat("d", 40)
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
[ "$#" -eq 7 ] && [ "$1" = -C ] && [ "$2" = %q ] &&
    [ "$3" = -c ] && [ "$4" = core.logAllRefUpdates=false ] &&
    [ "$5" = rev-parse ] && [ "$6" = --verify ] && [ "$7" = HEAD ] || exit 97
printf '%%s\n' %q
printf '%%s\n' 'warning: stderr noise' >&2
`, logPath, repo, wantTip)
	if err := testexec.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	environment := testEnvironment(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := goalGitWithEnvironment(repo, environment, nil, "rev-parse", "--verify", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	tip := strings.TrimSpace(out)
	if len(tip) != 40 || strings.Contains(out, "warning") {
		t.Fatalf("parsed tip polluted by stderr: %q", out)
	}
	if out != wantTip+"\n" {
		t.Fatalf("parsed tip changed: %q", out)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	wantCall := fmt.Sprintf("-C %s -c core.logAllRefUpdates=false rev-parse --verify HEAD\n", repo)
	if string(logBytes) != wantCall {
		t.Fatalf("unexpected git calls: %s", logBytes)
	}
}
