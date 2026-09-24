package goal

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestWaitGoalCursorHistory(t *testing.T) {
	t.Parallel()
	before := &TreeGoals{Live: map[string]*GoalFile{"goal-a": {Id: "goal-a", History: []HistoryLine{{At: "2026-09-13T10:00:00Z", Opid: "old-op", Verb: "approve", Actor: "human:wido", Targets: []string{"goal-a"}, Keep: -1}}}}, Done: map[string]*GoalFile{}}
	after := &TreeGoals{Live: map[string]*GoalFile{"goal-a": {Id: "goal-a", History: append(append([]HistoryLine{}, before.Live["goal-a"].History...), HistoryLine{At: "2026-09-13T11:00:00Z", Opid: "answer-op", Verb: "answer", Actor: "human:wido", Targets: []string{"goal-a"}, Keep: -1, AuthorityOutcome: AuthorityOutcomeAuthenticatedChannelWord, ChannelProvider: "fake", ChannelUser: "u", ChannelRef: "r", ChannelStep: 1, Question: "question-a"})}}, Done: map[string]*GoalFile{}}
	rows, err := appendedGoalHistory(before, after, "goal-a")
	if err != nil || len(rows) != 1 || rows[0].Question != "question-a" || !acceptedHumanAct(rows[0]) {
		t.Fatalf("cursor history rows=%+v err=%v", rows, err)
	}
	rewritten := &TreeGoals{Live: map[string]*GoalFile{"goal-a": {Id: "goal-a", History: []HistoryLine{{At: "2026-09-13T10:00:00Z", Opid: "changed", Verb: "approve", Actor: "human:wido", Targets: []string{"goal-a"}, Keep: -1}}}}, Done: map[string]*GoalFile{}}
	if _, err := appendedGoalHistory(before, rewritten, "goal-a"); err == nil || !strings.Contains(err.Error(), "rewritten") {
		t.Fatalf("rewritten cursor history accepted: %v", err)
	}
	seatAct := HistoryLine{Actor: "mac+lineage", Verb: "approve", AuthorityOutcome: AuthorityOutcomePowerOfAttorney}
	if acceptedHumanAct(seatAct) {
		t.Fatal("a seat acting under power of attorney was classified as a human act")
	}
}

func TestWaitGoalLandingDestinationMatchesLedgerBranch(t *testing.T) {
	t.Parallel()
	endpoint, client := fakeGoalEndpoint(t)
	repo := endpoint.Root
	baseTip := acceptedTipForEndpoint(t, endpoint)
	opened, err := Open(verbReqFor(endpoint, "01J5X0000000000000000000D1", "mac-a"), "goal-a", "Wait target.", "main", "Wait for landing.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal-a: %+v %v", opened, err)
	}
	transcript := newWaitObservationTranscript(t, endpoint, client)
	transcript.declare(opened.Tip, baseTip)
	selector := metarun.WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "landing", After: opened.Tip}
	transcript.endpoint("", "")
	transcript.expect("refs/remotes/origin/trunk\n", "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref")
	ctx := withWaitGitDependencies(context.Background(), transcript.dependencies())
	refused, observeErr := ObserveLedger(ctx, repo, selector, metarun.WaiterTarget{}, opened.Tip)
	if observeErr != nil || refused.ExitCode != metarun.ExitNoRecord || refused.Reason != "landings go to refs/heads/trunk; this ledger endpoint watches refs/heads/main" {
		t.Fatalf("configured mismatched landing destination = %+v err=%v", refused, observeErr)
	}
	transcript.done()
	transcript.endpoint("", "")
	transcript.expect("refs/remotes/origin/main\n", "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref")
	transcript.capture(opened.Tip)
	transcript.acceptance(opened.Tip, opened.Tip)
	transcript.files(opened.Tip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	transcript.cleanup()
	accepted, observeErr := ObserveLedger(ctx, repo, selector, metarun.WaiterTarget{}, opened.Tip)
	if observeErr != nil || !accepted.Pending || accepted.ExitCode != 0 {
		t.Fatalf("matching remote landing destination = %+v err=%v", accepted, observeErr)
	}
	transcript.done()
	transcript.endpoint("local", LocalLedgerBranch)
	transcript.expect("", "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref")
	transcript.expect("refs/heads/main\n", "symbolic-ref", "--quiet", "HEAD")
	refused, observeErr = ObserveLedger(ctx, repo, selector, metarun.WaiterTarget{}, opened.Tip)
	wantReason := "landings go to refs/heads/main; this ledger endpoint watches " + LocalLedgerBranch
	if observeErr != nil || refused.ExitCode != metarun.ExitNoRecord || refused.Reason != wantReason {
		t.Fatalf("local mismatched landing destination = %+v err=%v", refused, observeErr)
	}
	transcript.done()
}

func TestWaitGoalFetchDeadline(t *testing.T) {
	t.Parallel()
	if _, err := CaptureTipBounded(Endpoint{Root: t.TempDir(), Remote: "local", Branch: LocalLedgerBranch}, 0); err == nil || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("zero fetch budget was accepted: %v", err)
	}
	endpoint, client := fakeGoalEndpoint(t)
	repo := endpoint.Root
	baseTip := acceptedTipForEndpoint(t, endpoint)
	opened, err := Open(verbReqFor(endpoint, "01J5X0000000000000000000W1", "mac-a"), "goal-a", "Wait target.", "main", "Wait.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal-a: %+v %v", opened, err)
	}
	cursor := opened.Tip
	advanced, err := Open(verbReqFor(endpoint, "01J5X0000000000000000000W2", "mac-a"), "goal-b", "Advance ledger.", "main", "Advance.")
	if err != nil || advanced.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal-b: %+v %v", advanced, err)
	}
	selector := metarun.WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "human-act", After: cursor}
	stages := []string{"endpoint resolution", "acceptance gates", "commit validation", "change inspection"}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			transcript := newWaitObservationTranscript(t, endpoint, client)
			transcript.declare(cursor, baseTip)
			transcript.declare(advanced.Tip, cursor)
			if stage == "endpoint resolution" {
				transcript.cancel("config", "--get", "goal.sync-remote")
			} else {
				transcript.endpoint("", "")
				transcript.capture(advanced.Tip)
				switch stage {
				case "acceptance gates":
					transcript.cancel("cat-file", "-p", cursor+":./"+goalsPrefix+"backlog.md")
				case "commit validation":
					transcript.acceptance(cursor, advanced.Tip)
					transcript.cancel("ls-tree", "-r", "--name-only", advanced.Tip, "--", goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
				case "change inspection":
					transcript.acceptance(cursor, advanced.Tip)
					transcript.files(advanced.Tip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
					transcript.cancel("rev-list", "--reverse", "--first-parent", cursor+".."+advanced.Tip)
				}
				transcript.cleanup()
			}
			ctx := withWaitGitDependencies(context.Background(), transcript.dependencies())
			result := (&metarun.Store{Root: repo}).Wait(ctx, metarun.WaitRequest{
				Selector: selector, Owner: metarun.Caller{Class: "MAIN", MainId: "main-bound", OwnerLineage: "lineage-bound", SessionId: "session-bound"},
				RuntimeSession: "runtime-bound", Timeout: time.Hour,
			}, metarun.WaitOptions{Observe: func(readCtx context.Context, selected metarun.WaitSelector, pinned metarun.WaiterTarget, tip string) (metarun.SourceObservation, error) {
				return ObserveLedger(readCtx, repo, selected, pinned, tip)
			}})
			if transcript.cancelled != 1 || result.ExitCode != metarun.ExitWaiterIO {
				t.Fatalf("hung %s: reached=%t result=%+v", stage, transcript.cancelled == 1, result)
			}
			transcript.done()
		})
	}
}

func TestGitAdapterWaitGoalFetchDeadlineInstalledCommand(t *testing.T) {
	t.Parallel()
	if os.Getenv("METASYSTEM_WAIT_BINARY") == "" {
		t.Skip("installed wait binary is not configured")
	}
	_, repo := oneClone(t)
	mustGit(t, repo, "config", "metasystem.goal.machine", "mac-a")
	seedLedger(t, repo)
	opened, err := Open(verbReq(repo, "01J5X0000000000000000000W1", "mac-a"), "goal-a", "Wait target.", "main", "Wait.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal-a: %+v %v", opened, err)
	}
	cursor := opened.Tip
	advanced, err := Open(verbReq(repo, "01J5X0000000000000000000W2", "mac-a"), "goal-b", "Advance ledger.", "main", "Advance.")
	if err != nil || advanced.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal-b: %+v %v", advanced, err)
	}
	if binary := os.Getenv("METASYSTEM_WAIT_BINARY"); binary != "" {
		t.Run("installed hanging git", func(t *testing.T) {
			t.Parallel()
			binary := testutil.InstalledWaitBinary(t, binary)
			self := int64(os.Getpid())
			exact, state, probeErr := (identity.KernelProber{}).Probe(self)
			if probeErr != nil || state != identity.Alive {
				t.Fatalf("current process identity=%+v state=%s err=%v", exact, state, probeErr)
			}
			announce := exec.Command(binary, "lease", "announce", "--root", repo, "--session", "wait-bounds-session", "--pid", strconv.FormatInt(self, 10), "--start", strconv.FormatInt(exact.StartedAt.Unix(), 10), "--start-ticks", strconv.FormatInt(exact.StartTicks, 10), "--boot-id", exact.BootID, "--tag", "wait-bounds-test", "--runtime", "fake", "--owner-lineage", "wait-bounds-lineage")
			if output, announceErr := announce.CombinedOutput(); announceErr != nil {
				t.Fatalf("announce installed bounds holder: %v %s", announceErr, output)
			}
			dir := t.TempDir()
			fixture := testutil.Fixture(t)
			wrapper := filepath.Join(dir, "git")
			realGit, lookupErr := exec.LookPath("git")
			if lookupErr != nil {
				t.Fatal(lookupErr)
			}
			marker := filepath.Join(dir, "fetch-started")
			groupMarker := filepath.Join(dir, "fetch-group")
			childMarker := filepath.Join(dir, "fetch-child")
			groupFIFO, childFIFO := openHangingGitPIDFIFOs(t, groupMarker, childMarker)
			wrapperSource := "#!/bin/sh\n" + testutil.ShellPrologue + `case " $* " in
  *" fetch "*)
    printf reached >"${LEDGER_HANG_MARKER:?}"
	printf '%s %s\n' "$$" "$(ps -o pgid= -p $$ | tr -d ' ')" >"${LEDGER_HANG_GROUP:?}"
    trap '' TERM
	sh -c 'trap "" TERM; echo $$ >"${LEDGER_HANG_CHILD:?}"; read -r _ <"${METASYSTEM_FIXTURE_LEASH:?}"' sh "$tag" &
    wait
    ;;
esac
exec "${LEDGER_REAL_GIT:?}" "$@"
`
			if writeErr := testexec.WriteFile(wrapper, []byte(wrapperSource), 0o755); writeErr != nil {
				t.Fatal(writeErr)
			}
			cmd := exec.Command(binary, "wait", "--root", repo, "--goal", "goal-a", "--event", "human-act", "--verb", "deny", "--after", cursor, "--timeout", "30s", "--json")
			childEnvironment := environWithoutGitSteering()
			pathValue := "PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH")
			pathReplaced := false
			for i, value := range childEnvironment {
				if strings.HasPrefix(value, "PATH=") {
					childEnvironment[i] = pathValue
					pathReplaced = true
				}
			}
			if !pathReplaced {
				childEnvironment = append(childEnvironment, pathValue)
			}
			childEnvironment = append(childEnvironment, "LEDGER_REAL_GIT="+realGit, "LEDGER_HANG_MARKER="+marker, "LEDGER_HANG_GROUP="+groupMarker, "LEDGER_HANG_CHILD="+childMarker)
			cmd.Env = fixture.Env(childEnvironment)
			type result struct {
				output []byte
				err    error
			}
			var commandOutput bytes.Buffer
			cmd.Stdout = &commandOutput
			cmd.Stderr = &commandOutput
			if startErr := cmd.Start(); startErr != nil {
				t.Fatalf("start installed hanging Git wait: %v", startErr)
			}
			fixture.Record(cmd.Process.Pid)
			done := make(chan result, 1)
			go func() {
				commandErr := cmd.Wait()
				done <- result{output: append([]byte(nil), commandOutput.Bytes()...), err: commandErr}
			}()
			var wrapperID, groupID, childID int
			select {
			case identities := <-waitForHangingGitPIDs(groupFIFO, childFIFO):
				if identities.err != nil {
					t.Fatal(identities.err)
				}
				wrapperID, groupID, childID = identities.wrapperID, identities.groupID, identities.childID
			case commandResult := <-done:
				t.Fatalf("installed hanging Git wait returned before publishing transport identities: exit=%v output=%s", commandResult.err, commandResult.output)
			}
			fixture.Record(wrapperID)
			fixture.Record(childID)
			if wrapperID != groupID {
				t.Fatalf("installed hanging Git wrapper pid %d did not lead process group %d", wrapperID, groupID)
			}
			commandResult := <-done
			exit, ok := commandResult.err.(*exec.ExitError)
			if !ok || exit.ExitCode() != metarun.ExitWaiterIO {
				t.Fatalf("installed hanging Git wait exit=%v output=%s", commandResult.err, commandResult.output)
			}
			for _, pid := range []int{wrapperID, childID} {
				exited, exitErr := transportMemberExited(pid)
				if exitErr != nil {
					t.Fatalf("probe installed hanging Git transport member %d: %v", pid, exitErr)
				}
				if !exited {
					t.Fatalf("installed hanging Git transport member %d survived after wait returned", pid)
				}
			}
			if stateOut, stateErr := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(childID)).CombinedOutput(); stateErr == nil && !strings.HasPrefix(strings.TrimSpace(string(stateOut)), "Z") {
				t.Fatalf("installed hanging Git spinner %d survived with state %q", childID, stateOut)
			}
		})
	}
}

func TestWaitGoalObservationUsesBoundedBatchReads(t *testing.T) {
	t.Parallel()
	endpoint, client := fakeGoalEndpoint(t)
	repo := endpoint.Root
	baseTip := acceptedTipForEndpoint(t, endpoint)
	opened, err := Open(verbReqFor(endpoint, "01J5X0000000000000000000B1", "mac-a"), "goal-a", "Wait target.", "main", "Wait.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal-a: %+v %v", opened, err)
	}
	transcript := newWaitObservationTranscript(t, endpoint, client)
	transcript.declare(opened.Tip, baseTip)
	selector := metarun.WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "human-act", After: opened.Tip}
	transcript.endpoint("", "")
	transcript.capture(opened.Tip)
	transcript.acceptance(opened.Tip, opened.Tip)
	transcript.files(opened.Tip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	transcript.expect("mac-a\n", "config", "--get", "metasystem.goal.machine")
	transcript.cleanup()
	observe := func(pinned metarun.WaiterTarget, lastTip string) (metarun.SourceObservation, [][]string) {
		callStart := len(transcript.calls)
		ctx := withWaitGitDependencies(context.Background(), transcript.dependencies())
		observation, observeErr := ObserveLedgerForWait(ctx, repo, selector, pinned, lastTip, "lineage-a")
		if observeErr != nil || !observation.Pending {
			t.Fatalf("observation after %s = %+v err=%v", lastTip, observation, observeErr)
		}
		transcript.done()
		return observation, transcript.calls[callStart:]
	}
	firstObservation, firstCalls := observe(metarun.WaiterTarget{}, "")
	const newChanges = 3
	previousTip := opened.Tip
	var newTips []string
	for i, goalID := range []string{"goal-b", "goal-c", "goal-d"} {
		advanced, openErr := Open(verbReqFor(endpoint, fmt.Sprintf("01J5X0000000000000000000B%d", i+2), "mac-a"), goalID, "More files.", "main", "Wait.")
		if openErr != nil || advanced.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", goalID, advanced, openErr)
		}
		transcript.declare(advanced.Tip, previousTip)
		newTips = append(newTips, advanced.Tip)
		previousTip = advanced.Tip
	}
	transcript.endpoint("", "")
	transcript.capture(previousTip)
	transcript.acceptance(opened.Tip, previousTip)
	transcript.files(previousTip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	transcript.changes(opened.Tip, newTips...)
	transcript.expect("mac-a\n", "config", "--get", "metasystem.goal.machine")
	transcript.files(opened.Tip, goalsPrefix, recordsGoalsPrefix)
	for _, tip := range newTips[:len(newTips)-1] {
		transcript.files(tip, goalsPrefix, recordsGoalsPrefix)
	}
	transcript.cleanup()
	secondObservation, secondCalls := observe(firstObservation.Incarnation, firstObservation.LedgerTip)
	for name, calls := range map[string][][]string{"first": firstCalls, "incremental": secondCalls} {
		batch := 0
		for _, args := range calls {
			if len(args) >= 2 && args[0] == "cat-file" && args[1] == "--batch" {
				batch++
			}
			if len(args) > 0 && args[0] == "show" {
				t.Fatalf("%s observation used one process per file: %v", name, calls)
			}
		}
		wantMaximum := 1
		if name == "incremental" {
			wantMaximum = newChanges + 1
		}
		if batch < 1 || batch > wantMaximum {
			t.Fatalf("%s observation batch calls=%d, maximum=%d, all calls=%v", name, batch, wantMaximum, calls)
		}
	}
	var incrementalRange bool
	for _, args := range secondCalls {
		if len(args) > 2 && args[0] == "rev-list" && slices.Contains(args, firstObservation.LedgerTip+".."+secondObservation.LedgerTip) {
			incrementalRange = true
		}
	}
	if !incrementalRange {
		t.Fatalf("incremental observation did not start at the last checked tip: %v", secondCalls)
	}
	transcript.endpoint("", "")
	transcript.capture(previousTip)
	transcript.cleanup()
	_, repeatedCalls := observe(secondObservation.Incarnation, secondObservation.LedgerTip)
	for _, args := range repeatedCalls {
		if len(args) == 0 {
			continue
		}
		switch args[0] {
		case "cat-file", "ls-tree", "rev-list", "diff", "merge-base":
			t.Fatalf("an unchanged tip re-read a previously inspected ledger state: %v", repeatedCalls)
		}
	}
}

type spentDeadlineContext struct {
	context.Context
	done <-chan struct{}
}

func newSpentDeadlineContext(parent context.Context) context.Context {
	done := make(chan struct{})
	close(done)
	return spentDeadlineContext{Context: parent, done: done}
}

func (c spentDeadlineContext) Done() <-chan struct{} {
	return c.done
}

func (spentDeadlineContext) Err() error {
	return context.DeadlineExceeded
}

func TestWaitGoalSavedResultReplayThroughAcceptedLedger(t *testing.T) {
	t.Parallel()
	publisher, waiter := fakeGoalEndpointPair(t)
	client := publisher.Repository.(*fakeGoalRepository)
	baseTip := acceptedTipForEndpoint(t, publisher)
	waiterRoot := waiter.Root
	target := "goal-replay-observer"
	opened, err := Open(verbReqFor(publisher, "01J5X0000000000000000000R0", "mac-a"), target, "Replay a recorded wait result.", "main", "Wait for an authenticated answer.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open replay target: %+v %v", opened, err)
	}
	cursor := opened.Tip
	preActTips := map[string]bool{cursor: true}
	var preActOrder []string
	for index, goalID := range []string{"goal-replay-before-a", "goal-replay-before-b", "goal-replay-before-c", "goal-replay-before-d", "goal-replay-before-e", "goal-replay-before-f"} {
		advanced, openErr := Open(verbReqFor(publisher, fmt.Sprintf("01J5X0000000000000000000R%d", index+1), "mac-a"), goalID, "Advance before the answer.", "main", "Remain queued.")
		if openErr != nil || advanced.Outcome != OutcomeConfirmed {
			t.Fatalf("open pre-answer goal %s: %+v %v", goalID, advanced, openErr)
		}
		preActTips[advanced.Tip] = true
		preActOrder = append(preActOrder, advanced.Tip)
	}
	answerRequest := verbReqFor(publisher, "01J5X0000000000000000000R7", "mac-a")
	answered, err := Answer(answerRequest, target, "replay-question", "recorded answer", "", AnswerProof{Provider: "fake", User: "human-wido", Ref: "replay/answer", Step: 1})
	if err != nil || answered.Outcome != OutcomeConfirmed {
		t.Fatalf("answer replay target: %+v %v", answered, err)
	}
	owner := metarun.Caller{Class: "MAIN", MainId: "main-replay-observer", OwnerLineage: "lineage-replay-observer", SessionId: "session-replay-observer"}
	selector := metarun.WaitSelector{Kind: "goal", TargetID: target, GoalID: target, Event: "human-act", Verb: "answer", Question: "replay-question", After: cursor}
	store := &metarun.Store{Root: waiterRoot}
	options := metarun.WaitOptions{
		Observe: func(readCtx context.Context, selected metarun.WaitSelector, pinned metarun.WaiterTarget, floor string) (metarun.SourceObservation, error) {
			return ObserveLedgerForWait(readCtx, waiterRoot, selected, pinned, floor, owner.OwnerLineage)
		},
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		},
	}
	withoutGitOperationDeadline := func(parent context.Context, _ time.Duration, _ []string) (context.Context, context.CancelFunc) {
		return context.WithCancel(parent)
	}
	declare := func(t *testing.T) *waitObservationTranscript {
		tr := newWaitObservationTranscript(t, waiter, client)
		tr.declare(cursor, baseTip)
		parent := cursor
		for _, tip := range preActOrder {
			tr.declare(tip, parent)
			parent = tip
		}
		tr.declare(answered.Tip, parent)
		return tr
	}
	initial := declare(t)
	if got := initial.commits[answered.Tip].trailer; got != answerRequest.opid() {
		t.Fatalf("authenticated answer transaction trailer = %q, want %q", got, answerRequest.opid())
	}
	initial.endpoint("", "")
	initial.capture(answered.Tip)
	initial.acceptance(cursor, answered.Tip)
	initial.files(answered.Tip, goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
	initial.changes(cursor, append(append([]string(nil), preActOrder...), answered.Tip)...)
	initial.expect("mac-waiter\n", "config", "--get", "metasystem.goal.machine")
	initial.files(cursor, goalsPrefix, recordsGoalsPrefix)
	for _, tip := range preActOrder {
		initial.files(tip, goalsPrefix, recordsGoalsPrefix)
	}
	initial.cleanup()
	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	released := false
	savedResult := make(chan metarun.WaitResult, 1)
	joined := false
	waitCtx, cancelWait := context.WithCancel(t.Context())
	defer func() {
		if !released {
			close(releaseFetch)
		}
		cancelWait()
		if !joined {
			<-savedResult
		}
	}()
	delayedDependencies := initial.dependencies()
	runner := delayedDependencies.run
	fetches := 0
	delayedDependencies.run = func(gitCtx context.Context, root string, stdin []byte, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "fetch" {
			fetches++
			if fetches != 1 {
				return "", fmt.Errorf("unexpected extra wait fetch: %q", args)
			}
			close(fetchStarted)
			select {
			case <-releaseFetch:
			case <-gitCtx.Done():
				return "", gitCtx.Err()
			}
		}
		return runner(gitCtx, root, stdin, args...)
	}
	go func() {
		savedResult <- store.Wait(withWaitGitDependencies(waitCtx, delayedDependencies), metarun.WaitRequest{Selector: selector, Owner: owner, RuntimeSession: "runtime-replay-observer", Timeout: time.Hour}, options)
	}()
	select {
	case <-fetchStarted:
	case result := <-savedResult:
		joined = true
		t.Fatalf("saved replay returned before delayed fetch: %+v", result)
	}
	select {
	case result := <-savedResult:
		joined = true
		t.Fatalf("saved replay returned before delayed fetch release: %+v", result)
	default:
	}
	close(releaseFetch)
	released = true
	saved := <-savedResult
	joined = true
	if fetches != 1 {
		t.Fatalf("initial wait made %d fetches, want one", fetches)
	}
	if saved.ExitCode != metarun.ExitGreen || saved.LedgerTip != answered.Tip {
		t.Fatalf("save real answer result: %+v", saved)
	}
	row, _, err := metarun.LoadWaiterByID(waiterRoot, saved.WaitID)
	if err != nil || row.Result == nil || row.LastCheckedTip != answered.Tip {
		t.Fatalf("saved answer row=%+v err=%v", row, err)
	}

	const laterChanges = 2
	var laterTips []string
	for index, goalID := range []string{"goal-replay-after-a", "goal-replay-after-b"} {
		advanced, openErr := Open(verbReqFor(publisher, fmt.Sprintf("01J5X0000000000000000000R%d", index+8), "mac-a"), goalID, "Advance after the answer.", "main", "Remain queued.")
		if openErr != nil || advanced.Outcome != OutcomeConfirmed {
			t.Fatalf("open post-answer goal %s: %+v %v", goalID, advanced, openErr)
		}
		laterTips = append(laterTips, advanced.Tip)
	}
	newReplayTranscript := func(t *testing.T) *waitObservationTranscript {
		tr := declare(t)
		parent := answered.Tip
		for _, tip := range laterTips {
			tr.declare(tip, parent)
			parent = tip
		}
		tr.endpoint("", "")
		tr.capture(laterTips[1])
		tr.acceptance(answered.Tip, laterTips[1])
		tr.files(laterTips[1], goalsPrefix, recordsGoalsPrefix, ChannelPrefix)
		tr.changes(answered.Tip, laterTips...)
		tr.expect("mac-waiter\n", "config", "--get", "metasystem.goal.machine")
		tr.files(answered.Tip, goalsPrefix, recordsGoalsPrefix)
		return tr
	}

	t.Run("before rewinding the ledger", func(t *testing.T) {
		t.Run("later accepted changes replay the saved answer from its event floor", func(t *testing.T) {
			t.Parallel()
			tr := newReplayTranscript(t)
			tr.files(laterTips[0], goalsPrefix, recordsGoalsPrefix)
			tr.cleanup()
			var projectedTips []string
			dependencies := tr.dependencies()
			runner := dependencies.run
			dependencies.run = func(gitCtx context.Context, root string, stdin []byte, args ...string) (string, error) {
				if len(args) >= 4 && args[0] == "ls-tree" {
					projectedTips = append(projectedTips, args[3])
				}
				return runner(gitCtx, root, stdin, args...)
			}
			replayed := store.ResumeWait(withWaitGitDependencies(t.Context(), dependencies), saved.WaitID, owner, owner.SessionId, 0, options)
			if replayed.ExitCode != metarun.ExitGreen {
				t.Fatalf("replay after later accepted changes: %+v", replayed)
			}
			if len(projectedTips) == 0 || len(projectedTips) > laterChanges+2 {
				t.Fatalf("replay projected %d ledger states, want at most %d: %v", len(projectedTips), laterChanges+2, projectedTips)
			}
			sawAnswerFloor := false
			for _, tip := range projectedTips {
				if tip == answered.Tip {
					sawAnswerFloor = true
				}
				if preActTips[tip] {
					t.Fatalf("replay reprojected a ledger state before the saved answer: %s in %v", tip, projectedTips)
				}
			}
			if !sawAnswerFloor {
				t.Fatalf("replay did not project its saved answer floor: %v", projectedTips)
			}
		})

		t.Run("an exhausted intervening read is an operational failure", func(t *testing.T) {
			t.Parallel()
			tr := newReplayTranscript(t)
			tr.expectError(context.DeadlineExceeded, "ls-tree", "-r", "--name-only", laterTips[0], "--", goalsPrefix, recordsGoalsPrefix)
			tr.cleanup()
			contextReads := 0
			dependencies := tr.dependencies()
			dependencies.withTimeout = func(parent context.Context, budget time.Duration, args []string) (context.Context, context.CancelFunc) {
				if len(args) > 0 && args[0] == "ls-tree" {
					contextReads++
					if contextReads == 3 {
						return newSpentDeadlineContext(parent), func() {}
					}
				}
				return withoutGitOperationDeadline(parent, budget, args)
			}
			dependencies.withFetchTimeout = withoutGitOperationDeadline
			runnerReads := 0
			runner := dependencies.run
			dependencies.run = func(gitCtx context.Context, root string, stdin []byte, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "ls-tree" {
					runnerReads++
				}
				return runner(gitCtx, root, stdin, args...)
			}
			replayed := store.ResumeWait(withWaitGitDependencies(t.Context(), dependencies), saved.WaitID, owner, owner.SessionId, 0, options)
			if replayed.ExitCode != metarun.ExitWaiterIO || replayed.SourceOutcome != "transport-failure" || contextReads != 3 || runnerReads != 3 {
				t.Fatalf("budget-exhausted replay=%+v context reads=%d intervening reads=%d", replayed, contextReads, runnerReads)
			}
		})

		t.Run("cancelling an intervening read interrupts replay", func(t *testing.T) {
			t.Parallel()
			tr := newReplayTranscript(t)
			tr.cancel("ls-tree", "-r", "--name-only", laterTips[0], "--", goalsPrefix, recordsGoalsPrefix)
			tr.cleanup()
			reached := make(chan struct{})
			runnerReads := 0
			dependencies := tr.dependencies()
			dependencies.withTimeout = withoutGitOperationDeadline
			dependencies.withFetchTimeout = withoutGitOperationDeadline
			runner := dependencies.run
			dependencies.run = func(gitCtx context.Context, root string, stdin []byte, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "ls-tree" {
					runnerReads++
					if runnerReads == 3 {
						close(reached)
						<-gitCtx.Done()
					}
				}
				return runner(gitCtx, root, stdin, args...)
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancelled := make(chan struct{})
			defer func() { cancel(); <-cancelled }()
			go func() {
				select {
				case <-reached:
					cancel()
				case <-ctx.Done():
				}
				close(cancelled)
			}()
			replayed := store.ResumeWait(withWaitGitDependencies(ctx, dependencies), saved.WaitID, owner, owner.SessionId, 0, options)
			cancel()
			<-cancelled
			if replayed.ExitCode != metarun.ExitInterrupted || replayed.SourceOutcome != "interrupted" || runnerReads != 3 || tr.cancelled != 1 {
				t.Fatalf("cancelled replay=%+v intervening reads=%d", replayed, runnerReads)
			}
		})
	})

	t.Run("rewinding away the answer invalidates the saved evidence", func(t *testing.T) {
		client.store.mu.Lock()
		client.store.canonical = cursor
		client.store.mu.Unlock()
		tr := declare(t)
		tr.endpoint("", "")
		tr.capture(cursor)
		tr.rewind(answered.Tip, cursor)
		tr.cleanup()
		replayed := store.ResumeWait(withWaitGitDependencies(t.Context(), tr.dependencies()), saved.WaitID, owner, owner.SessionId, 0, options)
		if replayed.ExitCode != metarun.ExitNoRecord || replayed.SourceOutcome != "invalid-source" {
			t.Fatalf("rewound saved evidence replay=%+v", replayed)
		}
	})
}

func TestWaitGoalCancellationCleansPrivateFetchRef(t *testing.T) {
	t.Parallel()
	_, repo := oneClone(t)
	mustGit(t, repo, "config", "metasystem.goal.machine", "mac-a")
	seedLedger(t, repo)
	opened, err := Open(verbReq(repo, "01J5X0000000000000000000C1", "mac-a"), "goal-a", "Wait target.", "main", "Wait.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal-a: %+v %v", opened, err)
	}
	realRunner := attentionGitRun(runAttentionGit)
	cancelled := false
	dependencies := waitGitDependencies{run: func(ctx context.Context, root string, stdin []byte, args ...string) (string, error) {
		if !cancelled && len(args) >= 2 && args[0] == "cat-file" && args[1] == "-p" {
			cancelled = true
			return "", context.Canceled
		}
		return realRunner(ctx, root, stdin, args...)
	}}
	selector := metarun.WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "human-act", After: opened.Tip}
	ctx := withWaitGitDependencies(context.Background(), dependencies)
	observation, observeErr := ObserveLedger(ctx, repo, selector, metarun.WaiterTarget{}, "")
	if !cancelled || observeErr == nil || !observation.Temporary {
		t.Fatalf("cancelled observation=%+v err=%v reached=%t", observation, observeErr, cancelled)
	}
	if refs := mustGit(t, repo, "for-each-ref", "--format=%(refname)", "refs/metasystem/goals/fetch/read-"); refs != "" {
		t.Fatalf("cancelled observation leaked private fetch refs: %q", refs)
	}
}

func TestAcceptedLedgerTipDistinguishesBootstrapFromBreakage(t *testing.T) {
	t.Parallel()
	_, clone := oneClone(t)
	if tip, exists, err := AcceptedLedgerTip(clone); err != nil || exists || tip != "" {
		t.Fatalf("pre-bootstrap accepted tip: tip=%q exists=%t err=%v", tip, exists, err)
	}
	seedLedger(t, clone)
	tip, exists, err := AcceptedLedgerTip(clone)
	if err != nil || !exists || tip == "" {
		t.Fatalf("migrated accepted tip: tip=%q exists=%t err=%v", tip, exists, err)
	}
	common := mustGit(t, clone, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err := os.WriteFile(filepath.Join(common, filepath.FromSlash(AcceptedRef)), []byte("not-an-oid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := AcceptedLedgerTip(clone); err == nil || !strings.Contains(err.Error(), "accepted ref") {
		t.Fatalf("broken accepted ref was read as bootstrap: %v", err)
	}
}

func TestLedgerChangesFallsBackWhenAcceptedIsASecondParent(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	accepted := strings.Repeat("a", 40)
	otherParent := strings.Repeat("b", 40)
	merged := strings.Repeat("c", 40)
	next := strings.Repeat("d", 40)
	transcript := []struct {
		args []string
		out  string
	}{
		{[]string{"rev-list", "--reverse", "--first-parent", accepted + ".." + merged}, merged + "\n"},
		{[]string{"rev-parse", "--verify", merged + "^1"}, otherParent + "\n"},
		{[]string{"rev-list", "--reverse", "--first-parent", merged + ".." + next}, next + "\n"},
		{[]string{"rev-parse", "--verify", next + "^1"}, merged + "\n"},
	}
	read := 0
	reader := func(root string, extraEnv []string, args ...string) (string, error) {
		t.Helper()
		if read >= len(transcript) {
			t.Fatalf("unexpected ledger history read: %v", args)
		}
		step := transcript[read]
		if root != repo || extraEnv != nil || !slices.Equal(args, step.args) {
			t.Fatalf("ledger history read %d = root %q env %v args %v; want root %q nil env args %v", read, root, extraEnv, args, repo, step.args)
		}
		read++
		return step.out, nil
	}
	changes, err := ledgerChangesWithReader(repo, accepted, merged, reader)
	if err != nil || len(changes) != 1 || changes[0].Tip != merged || changes[0].Consecutive {
		t.Fatalf("second-parent ancestry invented intermediate canonical states: %+v %v", changes, err)
	}
	changes, err = ledgerChangesWithReader(repo, merged, next, reader)
	if err != nil || len(changes) != 1 || changes[0].Tip != next || !changes[0].Consecutive {
		t.Fatalf("ordinary first-parent movement was not consecutive: %+v %v", changes, err)
	}
	if read != len(transcript) {
		t.Fatalf("ledger history read %d of %d expected commands", read, len(transcript))
	}
}

func TestLedgerChangesDoesNotDisguiseUnreadableHistoryAsARewind(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	before := strings.Repeat("a", 40)
	after := strings.Repeat("b", 40)
	readErr := errors.New("declared unreadable history")
	wantArgs := []string{"rev-list", "--reverse", "--first-parent", before + ".." + after}
	reads := 0
	reader := func(root string, extraEnv []string, args ...string) (string, error) {
		t.Helper()
		if reads != 0 || root != repo || extraEnv != nil || !slices.Equal(args, wantArgs) {
			t.Fatalf("unexpected ledger history read %d = root %q env %v args %v; want root %q nil env args %v", reads, root, extraEnv, args, repo, wantArgs)
		}
		reads++
		return "", readErr
	}
	if _, err := ledgerChangesWithReader(repo, before, after, reader); err == nil || !strings.Contains(err.Error(), "walk accepted ledger changes") || !errors.Is(err, readErr) {
		t.Fatalf("unreadable object history was accepted as a direct transition: %v", err)
	}
	if reads != 1 {
		t.Fatalf("ledger history read %d of 1 expected commands", reads)
	}
}

func TestRunAttentionGitCancellationUsesInjectedTimers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := t.TempDir()
	startedPath := filepath.Join(dir, "started")
	blockedPath := filepath.Join(dir, "blocked")
	callsPath := filepath.Join(dir, "calls")
	for _, path := range []string{startedPath, blockedPath} {
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	started, err := os.OpenFile(startedPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = started.Close() })

	timers := newFakeAttentionTimerSource()
	timers.fireTimers = true
	ctx, cancel := context.WithCancel(withWaitGitDependencies(context.Background(), waitGitDependencies{timers: timers}))
	defer cancel()
	result := make(chan error, 1)
	alias := "!trap '' TERM; printf S > " + strconv.Quote(startedPath) + "; read -r _ < " + strconv.Quote(blockedPath)
	script := `#!/bin/sh
if [ "$#" -eq 7 ] && [ "$1" = -C ] && [ "$2" = "$LEDGER_EXPECT_ROOT" ] &&
   [ "$3" = -c ] && [ "$4" = core.logAllRefUpdates=false ] &&
   [ "$5" = -c ] && [ "$6" = "alias.attention-hang=$LEDGER_EXPECT_ALIAS" ] &&
   [ "$7" = attention-hang ]; then
  printf 'accepted\n' >> "$LEDGER_CALLS_FILE"
  trap '' TERM
  printf S > "$LEDGER_HANG_STARTED"
  read -r _ < "$LEDGER_HANG_BLOCKED"
  exit 0
fi
printf 'unexpected' >> "$LEDGER_CALLS_FILE"
printf ' <%s>' "$@" >> "$LEDGER_CALLS_FILE"
printf '\n' >> "$LEDGER_CALLS_FILE"
exit 97
`
	if err := testexec.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	environment := testEnvironment(os.Environ(), "PATH="+dir,
		"LEDGER_EXPECT_ROOT="+root, "LEDGER_EXPECT_ALIAS="+alias, "LEDGER_HANG_STARTED="+startedPath,
		"LEDGER_HANG_BLOCKED="+blockedPath, "LEDGER_CALLS_FILE="+callsPath)
	go func() {
		_, runErr := runAttentionGitWithEnvironment(ctx, root, nil, environment, "-c", "alias.attention-hang="+alias, "attention-hang")
		result <- runErr
	}()
	startedByte := make([]byte, 1)
	startedResult := make(chan error, 1)
	go func() {
		_, readErr := started.Read(startedByte)
		startedResult <- readErr
	}()
	select {
	case err := <-startedResult:
		if err != nil || string(startedByte) != "S" {
			t.Fatalf("hanging git start signal=%q err=%v", startedByte, err)
		}
	case err := <-result:
		t.Fatalf("strict git executable returned before the start signal: %v", err)
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled git returned %v", err)
	}
	if calls, err := os.ReadFile(callsPath); err != nil || string(calls) != "accepted\n" {
		t.Fatalf("strict git calls=%q err=%v", calls, err)
	}
	created := timers.Created()
	if len(created) != 2 || created[0].kind != fakeAttentionTimer || created[0].duration != boundedCaptureGrace || created[1].kind != fakeAttentionTicker || created[1].duration != 10*time.Millisecond {
		t.Fatalf("cancelled git timers=%v, want grace %s then 10ms poll", created, boundedCaptureGrace)
	}
}

type strictCaptureCleanupRepository struct {
	Repository
	root        string
	environment []string
}

func (r strictCaptureCleanupRepository) Release(opid string) error {
	cmd := commandWithEnvironment(r.environment, "git", "-C", r.root, "update-ref", "-d", fetchRefFor(opid))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("strict capture cleanup: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func strictCaptureGitScript(body string) string {
	return "#!/bin/sh\n" + testutil.ShellPrologue + `prefix='+refs/heads/main:refs/metasystem/goals/fetch/read-'
if [ "$#" -eq 9 ] && [ "$1" = -C ] && [ "$2" = "$LEDGER_EXPECT_ROOT" ] &&
   [ "$3" = -c ] && [ "$4" = core.logAllRefUpdates=false ] &&
   [ "$5" = fetch ] && [ "$6" = --no-tags ] && [ "$7" = --refmap= ] &&
   [ "$8" = blocked ]; then
  case "$9" in
    "$prefix"*)
      suffix=${9#"$prefix"}
      if [ "${#suffix}" -eq 26 ]; then
        case "$suffix" in
          *[!0123456789ABCDEFGHJKMNPQRSTVWXYZ]*) ;;
          *)
            destination=${9#*:}
            printf '%s\n' "$destination" > "$LEDGER_CAPTURE_REF_FILE"
            printf 'fetch %s\n' "$destination" >> "$LEDGER_CAPTURE_CALLS_FILE"
` + body + `
            exit 0
            ;;
        esac
      fi
      ;;
  esac
fi
if [ "$#" -eq 5 ] && [ "$1" = -C ] && [ "$2" = "$LEDGER_EXPECT_ROOT" ] &&
   [ "$3" = update-ref ] && [ "$4" = -d ] &&
   [ -s "$LEDGER_CAPTURE_REF_FILE" ] && [ "$5" = "$(cat "$LEDGER_CAPTURE_REF_FILE")" ]; then
  printf 'cleanup %s\n' "$5" >> "$LEDGER_CAPTURE_CALLS_FILE"
  exit 0
fi
printf 'unexpected' >> "$LEDGER_CAPTURE_CALLS_FILE"
printf ' <%s>' "$@" >> "$LEDGER_CAPTURE_CALLS_FILE"
printf '\n' >> "$LEDGER_CAPTURE_CALLS_FILE"
exit 97
`
}

func strictCaptureGitFixture(t *testing.T, dir, root, body string, fixture *testutil.ProcessFixture, entries ...string) ([]string, Repository, func()) {
	t.Helper()
	refFile := filepath.Join(dir, "capture-ref")
	callsFile := filepath.Join(dir, "capture-calls")
	for _, name := range []string{"cat", "ps", "sh", "tr"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(path, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := testexec.WriteFile(filepath.Join(dir, "git"), []byte(strictCaptureGitScript(body)), 0o755); err != nil {
		t.Fatal(err)
	}
	environment := testEnvironment(os.Environ(), append(entries,
		"LEDGER_EXPECT_ROOT="+root, "LEDGER_CAPTURE_REF_FILE="+refFile, "LEDGER_CAPTURE_CALLS_FILE="+callsFile,
		"PATH="+dir)...)
	environment = fixture.Env(environment)
	repository := strictCaptureCleanupRepository{root: root, environment: environment}
	checkCalls := func() {
		t.Helper()
		refData, refErr := os.ReadFile(refFile)
		if refErr != nil {
			t.Fatalf("strict fetch destination was not recorded: %v", refErr)
		}
		ref := strings.TrimSpace(string(refData))
		calls, callsErr := os.ReadFile(callsFile)
		if callsErr != nil || string(calls) != "fetch "+ref+"\ncleanup "+ref+"\n" {
			t.Fatalf("strict git calls=%q, want fetch and cleanup for %q: %v", calls, ref, callsErr)
		}
	}
	return environment, repository, checkCalls
}

func TestCaptureTipBoundedKillsTheWholeTransportGroup(t *testing.T) {
	t.Parallel()
	timers := newFakeAttentionTimerSource()
	dir := t.TempDir()
	root := t.TempDir()
	fixture := testutil.Fixture(t)
	groupFile := filepath.Join(dir, "group")
	childFile := filepath.Join(dir, "child")
	groupFIFO, childFIFO := openHangingGitPIDFIFOs(t, groupFile, childFile)
	body := `
    trap '' TERM
	printf '%s %s\n' "$$" "$(ps -o pgid= -p $$ | tr -d ' ')" > "$LEDGER_FETCH_GROUP_FILE"
	sh -c 'trap "" TERM; echo $$ > "$LEDGER_FETCH_CHILD_FILE"; read -r _ <"${METASYSTEM_FIXTURE_LEASH:?}"' sh "$tag" &
    wait
`
	environment, repository, checkCalls := strictCaptureGitFixture(t, dir, root, body, fixture,
		"LEDGER_FETCH_GROUP_FILE="+groupFile, "LEDGER_FETCH_CHILD_FILE="+childFile)
	captured := make(chan error, 1)
	go func() {
		_, captureErr := CaptureTipBounded(Endpoint{Root: root, Remote: "blocked", Branch: "refs/heads/main", Repository: repository, commandEnv: environment, captureTimers: timers}, 300*time.Millisecond)
		captured <- captureErr
	}()
	var wrapperID, groupID, childID int
	select {
	case identities := <-waitForHangingGitPIDs(groupFIFO, childFIFO):
		if identities.err != nil {
			t.Fatal(identities.err)
		}
		wrapperID, groupID, childID = identities.wrapperID, identities.groupID, identities.childID
	case result := <-captured:
		t.Fatalf("capture returned before publishing transport identities: %v", result)
	}
	fixture.Record(wrapperID)
	fixture.Record(childID)
	if wrapperID != groupID || groupID == childID {
		t.Fatalf("invalid transport identities: wrapper=%d group=%d child=%d", wrapperID, groupID, childID)
	}
	timers.Next(t, captured, fakeAttentionTimer, 300*time.Millisecond).Fire()
	grace := timers.Next(t, captured, fakeAttentionTimer, boundedCaptureGrace)
	_ = timers.Next(t, captured, fakeAttentionTicker, 10*time.Millisecond)
	if groupErr := syscall.Kill(-groupID, 0); groupErr != nil && !errors.Is(groupErr, syscall.EPERM) {
		t.Fatalf("transport process group %d did not remain during TERM grace: %v", groupID, groupErr)
	}
	grace.Fire()
	err := <-captured
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("blocking transport did not time out: %v", err)
	}
	checkCalls()
	for _, pid := range []int{wrapperID, childID} {
		exited, exitErr := transportMemberExited(pid)
		if exitErr != nil {
			t.Fatalf("probe blocking transport member %d: %v", pid, exitErr)
		}
		if !exited {
			t.Fatalf("blocking transport member %d survived after capture returned", pid)
		}
	}
	if err := syscall.Kill(childID, 0); err == nil {
		// A killed child may remain briefly as a zombie. Its process state,
		// not kill(0), decides whether any transport work survived.
		stateOut, stateErr := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(childID)).CombinedOutput()
		if stateErr == nil && !strings.HasPrefix(strings.TrimSpace(string(stateOut)), "Z") {
			t.Fatalf("transport descendant %d survived group termination with state %q", childID, stateOut)
		}
	}
}

func TestCaptureTipBoundedLetsCooperativeTransportExitDuringGrace(t *testing.T) {
	t.Parallel()
	timers := newFakeAttentionTimerSource()
	if boundedCaptureGrace != 5*time.Second {
		t.Fatalf("bounded capture grace=%s, want 5s", boundedCaptureGrace)
	}
	dir := t.TempDir()
	root := t.TempDir()
	fixture := testutil.Fixture(t)
	groupFile := filepath.Join(dir, "group")
	childFile := filepath.Join(dir, "child")
	groupFIFO, childFIFO := openHangingGitPIDFIFOs(t, groupFile, childFile)
	termFile := filepath.Join(dir, "term")
	body := `
    trap 'echo TERM > "$LEDGER_GRACE_TERM_FILE"; exit 0' TERM
	printf '%s %s\n' "$$" "$(ps -o pgid= -p $$ | tr -d ' ')" > "$LEDGER_GRACE_GROUP_FILE"
	sh -c 'echo $$ > "$LEDGER_GRACE_CHILD_FILE"; read -r _ <"${METASYSTEM_FIXTURE_LEASH:?}"' sh "$tag" &
	wait
`
	environment, repository, checkCalls := strictCaptureGitFixture(t, dir, root, body, fixture,
		"LEDGER_GRACE_GROUP_FILE="+groupFile, "LEDGER_GRACE_CHILD_FILE="+childFile, "LEDGER_GRACE_TERM_FILE="+termFile)
	captured := make(chan error, 1)
	go func() {
		_, captureErr := CaptureTipBounded(Endpoint{Root: root, Remote: "blocked", Branch: "refs/heads/main", Repository: repository, commandEnv: environment, captureTimers: timers}, 300*time.Millisecond)
		captured <- captureErr
	}()
	var wrapperID, groupID, childID int
	select {
	case identities := <-waitForHangingGitPIDs(groupFIFO, childFIFO):
		if identities.err != nil {
			t.Fatal(identities.err)
		}
		wrapperID, groupID, childID = identities.wrapperID, identities.groupID, identities.childID
	case result := <-captured:
		t.Fatalf("capture returned before publishing transport identities: %v", result)
	}
	fixture.Record(wrapperID)
	fixture.Record(childID)
	if wrapperID != groupID {
		t.Fatalf("cooperative transport wrapper pid %d did not lead process group %d", wrapperID, groupID)
	}
	timers.Next(t, captured, fakeAttentionTimer, 300*time.Millisecond).Fire()
	grace := timers.Next(t, captured, fakeAttentionTimer, boundedCaptureGrace)
	poll := timers.Next(t, captured, fakeAttentionTicker, 10*time.Millisecond)
	var err error
	for {
		select {
		case err = <-captured:
			goto captureReturned
		default:
			poll.Fire()
			runtime.Gosched()
		}
	}

captureReturned:
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("cooperative transport did not time out: %v", err)
	}
	checkCalls()
	if grace.Fired() {
		t.Fatal("cooperative transport spent the grace timer")
	}
	if data, readErr := os.ReadFile(termFile); readErr != nil || strings.TrimSpace(string(data)) != "TERM" {
		t.Fatalf("transport received no graceful TERM opportunity: %q %v", data, readErr)
	}
	for _, pid := range []int{wrapperID, childID} {
		exited, exitErr := transportMemberExited(pid)
		if exitErr != nil {
			t.Fatalf("probe cooperative transport member %d: %v", pid, exitErr)
		}
		if !exited {
			t.Fatalf("cooperative transport member %d survived after capture returned", pid)
		}
	}
}

func TestTransportMemberExitedCountsAReapedGroupLeader(t *testing.T) {
	t.Parallel()
	fixture := testutil.Fixture(t)
	script := filepath.Join(t.TempDir(), "reaped-group-leader")
	source := "#!/bin/sh\n" + testutil.ShellPrologue + `sh -c 'read -r _ <"${METASYSTEM_FIXTURE_LEASH:?}"' sh "$tag" >/dev/null 2>&1 &
printf '%s\n' "$!"
`
	if err := testexec.WriteFile(script, []byte(source), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(script)
	cmd.Env = fixture.Env(os.Environ())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	leaderID := cmd.Process.Pid
	var descendantID int
	if _, err := fmt.Fscan(stdout, &descendantID); err != nil {
		t.Fatalf("read descendant pid: %v", err)
	}
	fixture.Record(leaderID)
	fixture.Record(descendantID)
	fixture.Hold(descendantID)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("reap process-group leader %d: %v", leaderID, err)
	}
	if err := syscall.Kill(-leaderID, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		t.Fatalf("process group %d has no live descendant: %v", leaderID, err)
	}
	leaderExited, err := transportMemberExited(leaderID)
	if err != nil || !leaderExited {
		t.Fatalf("reaped process-group leader %d probe: exited=%t err=%v", leaderID, leaderExited, err)
	}
	descendantExited, err := transportMemberExited(descendantID)
	if err != nil || descendantExited {
		t.Fatalf("live descendant %d probe: exited=%t err=%v", descendantID, descendantExited, err)
	}
}

func TestCaptureTipBoundedKillsADescendantThatOutlivesTheTransport(t *testing.T) {
	t.Parallel()
	timers := newFakeAttentionTimerSource()
	timers.noticeCCalls = true
	dir := t.TempDir()
	root := t.TempDir()
	fixture := testutil.Fixture(t)
	groupFile := filepath.Join(dir, "group")
	childFile := filepath.Join(dir, "child")
	groupFIFO, childFIFO := openHangingGitPIDFIFOs(t, groupFile, childFile)
	exitFile := filepath.Join(dir, "exit")
	if err := syscall.Mkfifo(exitFile, 0o600); err != nil {
		t.Fatal(err)
	}
	exitFIFO, err := os.OpenFile(exitFile, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = exitFIFO.Close() })
	termFile := filepath.Join(dir, "term")
	body := `
    trap 'echo TERM > "$LEDGER_GRACE_TERM_FILE"; exit 0' TERM
	printf '%s %s\n' "$$" "$$" > "$LEDGER_GRACE_GROUP_FILE"
	# Closing these pipes lets capture wait for the transport without waiting for its descendant.
	sh -c 'trap "" TERM; exec 3>"$LEDGER_GRACE_EXIT_FILE"; echo $$ > "$LEDGER_GRACE_CHILD_FILE"; read -r _ <"${METASYSTEM_FIXTURE_LEASH:?}"' sh "$tag" >/dev/null 2>&1 &
	wait
`
	environment, repository, checkCalls := strictCaptureGitFixture(t, dir, root, body, fixture,
		"LEDGER_GRACE_GROUP_FILE="+groupFile, "LEDGER_GRACE_CHILD_FILE="+childFile,
		"LEDGER_GRACE_EXIT_FILE="+exitFile, "LEDGER_GRACE_TERM_FILE="+termFile)
	captured := make(chan error, 1)
	go func() {
		_, captureErr := CaptureTipBounded(Endpoint{Root: root, Remote: "blocked", Branch: "refs/heads/main", Repository: repository, commandEnv: environment, captureTimers: timers}, 300*time.Millisecond)
		captured <- captureErr
	}()
	var wrapperID, groupID, childID int
	select {
	case identities := <-waitForHangingGitPIDs(groupFIFO, childFIFO):
		if identities.err != nil {
			t.Fatal(identities.err)
		}
		wrapperID, groupID, childID = identities.wrapperID, identities.groupID, identities.childID
	case result := <-captured:
		t.Fatalf("capture returned before publishing transport identities: %v", result)
	}
	fixture.Record(wrapperID)
	fixture.Record(childID)
	if wrapperID != groupID || groupID == childID {
		t.Fatalf("invalid transport identities: wrapper=%d group=%d child=%d", wrapperID, groupID, childID)
	}
	if err := syscall.SetNonblock(int(exitFIFO.Fd()), false); err != nil {
		t.Fatal(err)
	}
	timers.Next(t, captured, fakeAttentionTimer, 300*time.Millisecond).Fire()
	grace := timers.Next(t, captured, fakeAttentionTimer, boundedCaptureGrace)
	poll := timers.Next(t, captured, fakeAttentionTicker, 10*time.Millisecond)
	failEarly := func(result error) {
		exited, exitErr := transportMemberExited(childID)
		if exitErr == nil && !exited {
			t.Fatalf("transport descendant %d survived after capture returned before grace: %v", childID, result)
		}
		t.Fatalf("capture returned before grace: %v (descendant probe: %v)", result, exitErr)
	}
	// Capture evaluates poll.C on each select entry. The second call follows only
	// after the leader's wait result woke the first select and the probe found
	// the descendant still in the process group.
	for entries := 0; entries < 2; entries++ {
		select {
		case <-poll.cCalls:
		case result := <-captured:
			failEarly(result)
		}
	}
	select {
	case result := <-captured:
		failEarly(result)
	default:
	}
	leaderExited, leaderErr := transportMemberExited(wrapperID)
	if leaderErr != nil || !leaderExited {
		t.Fatalf("transport leader %d still running at second select entry: %v", wrapperID, leaderErr)
	}
	if data, readErr := os.ReadFile(termFile); readErr != nil || strings.TrimSpace(string(data)) != "TERM" {
		t.Fatalf("transport received no graceful TERM opportunity: %q %v", data, readErr)
	}
	childExited, childErr := transportMemberExited(childID)
	if childErr != nil || childExited {
		t.Fatalf("transport descendant %d exited before grace: %v", childID, childErr)
	}
	grace.Fire()
	err = <-captured
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("transport did not time out: %v", err)
	}
	checkCalls()
	if !grace.Fired() {
		t.Fatal("transport did not spend the grace timer")
	}
	if n, readErr := exitFIFO.Read(make([]byte, 1)); n != 0 || !errors.Is(readErr, io.EOF) {
		t.Fatalf("read descendant exit witness: bytes=%d err=%v", n, readErr)
	}
	for _, pid := range []int{wrapperID, childID} {
		exited, exitErr := transportMemberExited(pid)
		if exitErr != nil || !exited {
			t.Fatalf("transport member %d survived after exit witness: exited=%t err=%v", pid, exited, exitErr)
		}
	}
}

type fakeAttentionTimerKind string

const (
	fakeAttentionTimer  fakeAttentionTimerKind = "timer"
	fakeAttentionTicker fakeAttentionTimerKind = "ticker"
)

type fakeAttentionTimerSource struct {
	created chan *fakeAttentionTimerInstance
	mu      sync.Mutex
	all     []*fakeAttentionTimerInstance

	fireTimers   bool
	noticeCCalls bool
}

type fakeAttentionTimerInstance struct {
	kind     fakeAttentionTimerKind
	duration time.Duration
	c        chan time.Time
	mu       sync.Mutex
	fired    bool
	cCalls   chan struct{}
}

func newFakeAttentionTimerSource() *fakeAttentionTimerSource {
	return &fakeAttentionTimerSource{created: make(chan *fakeAttentionTimerInstance, 3)}
}

func (s *fakeAttentionTimerSource) NewTimer(after time.Duration) attentionTimer {
	return s.newTimer(fakeAttentionTimer, after)
}

func (s *fakeAttentionTimerSource) NewTicker(every time.Duration) attentionTimer {
	return s.newTimer(fakeAttentionTicker, every)
}

func (s *fakeAttentionTimerSource) newTimer(kind fakeAttentionTimerKind, duration time.Duration) attentionTimer {
	timer := &fakeAttentionTimerInstance{kind: kind, duration: duration, c: make(chan time.Time, 1)}
	s.mu.Lock()
	if s.noticeCCalls {
		timer.cCalls = make(chan struct{}, 2)
	}
	s.all = append(s.all, timer)
	s.mu.Unlock()
	if s.fireTimers && kind == fakeAttentionTimer {
		timer.Fire()
	}
	s.created <- timer
	return timer
}

func (s *fakeAttentionTimerSource) Created() []*fakeAttentionTimerInstance {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*fakeAttentionTimerInstance(nil), s.all...)
}

func (s *fakeAttentionTimerSource) Next(t *testing.T, captured <-chan error, kind fakeAttentionTimerKind, duration time.Duration) *fakeAttentionTimerInstance {
	t.Helper()
	var timer *fakeAttentionTimerInstance
	select {
	case timer = <-s.created:
	case result := <-captured:
		t.Fatalf("capture returned before creating attention %s for %s: %v", kind, duration, result)
	}
	if timer.kind != kind || timer.duration != duration {
		t.Fatalf("created attention %s for %s, want %s for %s", timer.kind, timer.duration, kind, duration)
	}
	return timer
}

func (t *fakeAttentionTimerInstance) C() <-chan time.Time {
	if t.cCalls != nil {
		t.cCalls <- struct{}{}
	}
	return t.c
}

func (t *fakeAttentionTimerInstance) Stop() {}

func (t *fakeAttentionTimerInstance) Fire() {
	t.mu.Lock()
	t.fired = true
	t.mu.Unlock()
	select {
	case t.c <- time.Time{}:
	default:
	}
}

func (t *fakeAttentionTimerInstance) Fired() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.fired
}

type hangingGitPIDs struct {
	wrapperID int
	groupID   int
	childID   int
	err       error
}

func openHangingGitPIDFIFOs(t *testing.T, groupPath, childPath string) (*os.File, *os.File) {
	t.Helper()
	for _, path := range []string{groupPath, childPath} {
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	groupFIFO, err := os.OpenFile(groupPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	childFIFO, err := os.OpenFile(childPath, os.O_RDWR, 0)
	if err != nil {
		_ = groupFIFO.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = groupFIFO.Close()
		_ = childFIFO.Close()
	})
	return groupFIFO, childFIFO
}

func waitForHangingGitPIDs(groupFIFO, childFIFO *os.File) <-chan hangingGitPIDs {
	identities := make(chan hangingGitPIDs, 1)
	go func() {
		groupData, groupErr := bufio.NewReader(groupFIFO).ReadBytes('\n')
		if groupErr != nil {
			identities <- hangingGitPIDs{err: fmt.Errorf("read transport wrapper and group identities: %w", groupErr)}
			return
		}
		childData, childErr := bufio.NewReader(childFIFO).ReadBytes('\n')
		if childErr != nil {
			identities <- hangingGitPIDs{err: fmt.Errorf("read transport child identity: %w", childErr)}
			return
		}
		wrapperID, groupID, childID, parseErr := parseHangingGitPIDs(groupData, childData)
		identities <- hangingGitPIDs{wrapperID: wrapperID, groupID: groupID, childID: childID, err: parseErr}
	}()
	return identities
}

func hangingGitPIDsReady(groupData, childData []byte) bool {
	groupFields := strings.Fields(string(groupData))
	childFields := strings.Fields(string(childData))
	if len(groupFields) != 2 || len(childFields) != 1 {
		return false
	}
	for _, field := range append(groupFields, childFields[0]) {
		pid, err := strconv.Atoi(field)
		if err != nil || pid < 1 {
			return false
		}
	}
	return true
}

func parseHangingGitPIDs(groupData, childData []byte) (int, int, int, error) {
	if !hangingGitPIDsReady(groupData, childData) {
		return 0, 0, 0, fmt.Errorf("transport wrote invalid process identities: group=%q child=%q", groupData, childData)
	}
	fields := strings.Fields(string(groupData))
	wrapperID, wrapperErr := strconv.Atoi(fields[0])
	groupID, groupErr := strconv.Atoi(fields[1])
	childID, childErr := strconv.Atoi(strings.TrimSpace(string(childData)))
	if wrapperErr != nil || groupErr != nil || childErr != nil || wrapperID < 1 || groupID < 1 || childID < 1 {
		return 0, 0, 0, fmt.Errorf("transport wrote invalid process identities: wrapper=%q err=%v group=%q err=%v child=%q err=%v", fields[0], wrapperErr, fields[1], groupErr, childData, childErr)
	}
	return wrapperID, groupID, childID, nil
}

func TestProjectAtReadsACommitWithoutMovingAnyRef(t *testing.T) {
	t.Parallel()
	endpoint, repository := fakeGoalEndpoint(t, vGoal("seen", StateQueued))
	before := acceptedTipForEndpoint(t, endpoint)
	goal := vGoal("unseen", StateQueued)
	other := repository.store.client()
	other.accepted = before
	publisher := endpoint
	publisher.Repository = other
	result, err := Publish(publisher, PublishRequest{
		Opid: "publish-unseen", Machine: "mac-a", Lineage: "lin-1", Intent: testIntentFor("open"), Message: "publish unseen goal",
		Mutate: func(string) ([]Change, error) {
			return []Change{{Path: livePath(goal.Id), Content: RenderFile(goal)}}, nil
		},
		Validate: func(commit string) error { return validateCommitFor(publisher, commit) },
	})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("publish immutable goal commit: %+v %v", result, err)
	}
	projection, err := projectAtForEndpoint(endpoint, result.Tip)
	if err != nil || projection.Root != endpoint.Root || projection.Tip != result.Tip || projection.Tree == nil {
		t.Fatalf("projection at the published tip: %+v err=%v", projection, err)
	}
	if seen, unseen := projection.Tree.Live["seen"], projection.Tree.Live["unseen"]; seen == nil || seen.State != StateQueued || unseen == nil || unseen.State != StateQueued || unseen.Intent != goal.Intent {
		t.Fatalf("projection did not parse the committed goal files: %+v", projection.Tree.Live)
	}
	if after := acceptedTipForEndpoint(t, endpoint); after != before {
		t.Fatalf("a read moved the accepted ref: %q -> %q", before, after)
	}
	if _, err := projectAtForEndpoint(endpoint, "0000000000000000000000000000000000000000"); err == nil {
		t.Fatal("an unreadable commit must refuse, not project")
	}
}

func TestIsAncestorAnswersAllThreeShapes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git-calls.log")
	tip := strings.Repeat("a", 40)
	parent := strings.Repeat("b", 40)
	broken := strings.Repeat("c", 40)
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
[ "$#" -eq 8 ] && [ "$1" = -C ] && [ "$2" = %q ] &&
    [ "$3" = -c ] && [ "$4" = core.logAllRefUpdates=false ] &&
    [ "$5" = merge-base ] && [ "$6" = --is-ancestor ] || exit 97
if [ "$7" = %q ] && [ "$8" = %q ]; then exit 0; fi
if [ "$7" = %q ] && [ "$8" = %q ]; then exit 1; fi
if [ "$7" = %q ] && [ "$8" = %q ]; then exit 23; fi
exit 97
`, logPath, root, parent, tip, tip, parent, broken, tip)
	if err := testexec.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	environment := testEnvironment(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if ok, err := isAncestorWithEnvironment(root, tip, tip, environment); err != nil || !ok {
		t.Fatalf("a commit is its own ancestor: %t %v", ok, err)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("equality invoked git: %v", err)
	}
	if ok, err := isAncestorWithEnvironment(root, parent, tip, environment); err != nil || !ok {
		t.Fatalf("a first parent precedes its child: %t %v", ok, err)
	}
	if ok, err := isAncestorWithEnvironment(root, tip, parent, environment); err != nil || ok {
		t.Fatalf("a child does not precede its parent: %t %v", ok, err)
	}
	if ok, err := isAncestorWithEnvironment(root, broken, tip, environment); err == nil || ok {
		t.Fatalf("a merge-base failure other than exit 1 must remain an error: %t %v", ok, err)
	} else {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 23 {
			t.Fatalf("merge-base failure lost its exit status: %v", err)
		}
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("-C %s -c core.logAllRefUpdates=false merge-base --is-ancestor %s %s\n-C %s -c core.logAllRefUpdates=false merge-base --is-ancestor %s %s\n-C %s -c core.logAllRefUpdates=false merge-base --is-ancestor %s %s\n", root, parent, tip, root, tip, parent, root, broken, tip)
	if string(logBytes) != want {
		t.Fatalf("unexpected git calls:\n%s", logBytes)
	}
}
