package goal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	_, repo := oneClone(t)
	mustGit(t, repo, "config", "metasystem.goal.machine", "mac-a")
	seedLedger(t, repo)
	opened, err := Open(verbReq(repo, "01J5X0000000000000000000D1", "mac-a"), "goal-a", "Wait target.", "main", "Wait for landing.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal-a: %+v %v", opened, err)
	}
	selector := metarun.WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "landing", After: opened.Tip}
	mustGit(t, repo, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/trunk")
	refused, observeErr := ObserveLedger(context.Background(), repo, selector, metarun.WaiterTarget{}, opened.Tip)
	if observeErr != nil || refused.ExitCode != metarun.ExitNoRecord || refused.Reason != "landings go to refs/heads/trunk; this ledger endpoint watches refs/heads/main" {
		t.Fatalf("configured mismatched landing destination = %+v err=%v", refused, observeErr)
	}
	mustGit(t, repo, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	accepted, observeErr := ObserveLedger(context.Background(), repo, selector, metarun.WaiterTarget{}, opened.Tip)
	if observeErr != nil || !accepted.Pending || accepted.ExitCode != 0 {
		t.Fatalf("matching remote landing destination = %+v err=%v", accepted, observeErr)
	}
	mustGit(t, repo, "config", "--unset", "metasystem.steward.landing-ref")
	mustGit(t, repo, "config", "goal.sync-remote", "local")
	mustGit(t, repo, "config", "goal.sync-branch", LocalLedgerBranch)
	refused, observeErr = ObserveLedger(context.Background(), repo, selector, metarun.WaiterTarget{}, opened.Tip)
	wantReason := "landings go to refs/heads/main; this ledger endpoint watches " + LocalLedgerBranch
	if observeErr != nil || refused.ExitCode != metarun.ExitNoRecord || refused.Reason != wantReason {
		t.Fatalf("local mismatched landing destination = %+v err=%v", refused, observeErr)
	}
}

func TestWaitGoalFetchDeadline(t *testing.T) {
	t.Parallel()
	if _, err := CaptureTipBounded(Endpoint{Root: t.TempDir(), Remote: "local", Branch: LocalLedgerBranch}, 0); err == nil || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("zero fetch budget was accepted: %v", err)
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
	selector := metarun.WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "human-act", After: cursor}
	realRunner := attentionGitRun(runAttentionGit)
	stages := []struct {
		name  string
		match func([]string) bool
	}{
		{"endpoint resolution", func(args []string) bool {
			return len(args) >= 3 && args[0] == "config" && args[2] == "goal.sync-remote"
		}},
		{"acceptance gates", func(args []string) bool { return len(args) >= 1 && args[0] == "cat-file" }},
		{"commit validation", func(args []string) bool { return len(args) >= 1 && args[0] == "ls-tree" }},
		{"change inspection", func(args []string) bool { return len(args) >= 1 && args[0] == "rev-list" }},
	}
	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			dependencies := waitGitDependencies{}
			reached := false
			dependencies.withTimeout = func(parent context.Context, budget time.Duration, args []string) (context.Context, context.CancelFunc) {
				stageCtx, cancel := context.WithTimeout(parent, budget)
				if !reached && stage.match(args) {
					cancel()
				}
				return stageCtx, cancel
			}
			dependencies.run = func(gitCtx context.Context, root string, stdin []byte, args ...string) (string, error) {
				if !reached && stage.match(args) {
					reached = true
					<-gitCtx.Done()
					return "", gitCtx.Err()
				}
				return realRunner(gitCtx, root, stdin, args...)
			}
			ctx := withWaitGitDependencies(context.Background(), dependencies)
			result := (&metarun.Store{Root: repo}).Wait(ctx, metarun.WaitRequest{
				Selector: selector, Owner: metarun.Caller{Class: "MAIN", MainId: "main-bound", OwnerLineage: "lineage-bound", SessionId: "session-bound"},
				RuntimeSession: "runtime-bound", Timeout: time.Hour,
			}, metarun.WaitOptions{Observe: func(readCtx context.Context, selected metarun.WaitSelector, pinned metarun.WaiterTarget, tip string) (metarun.SourceObservation, error) {
				return ObserveLedger(readCtx, repo, selected, pinned, tip)
			}})
			if !reached || result.ExitCode != metarun.ExitWaiterIO {
				t.Fatalf("hung %s: reached=%t result=%+v", stage.name, reached, result)
			}
		})
	}
	if binary := os.Getenv("METASYSTEM_WAIT_BINARY"); binary != "" {
		t.Run("installed hanging git", func(t *testing.T) {
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
			if writeErr := os.WriteFile(wrapper, []byte(wrapperSource), 0o755); writeErr != nil {
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
			started := time.Now()
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
			wrapperID, groupID, childID := waitForHangingGitPIDs(t, groupMarker, childMarker, 25*time.Second)
			fixture.Record(wrapperID)
			fixture.Record(childID)
			if wrapperID != groupID {
				t.Fatalf("installed hanging Git wrapper pid %d did not lead process group %d", wrapperID, groupID)
			}
			select {
			case commandResult := <-done:
				exit, ok := commandResult.err.(*exec.ExitError)
				if !ok || exit.ExitCode() != metarun.ExitWaiterIO || time.Since(started) > 20*time.Second {
					t.Fatalf("installed hanging Git wait exit=%v elapsed=%s output=%s", commandResult.err, time.Since(started), commandResult.output)
				}
				if groupErr := waitForGroupAbsence(groupID, 2*time.Second); groupErr != nil {
					t.Fatalf("installed wait returned before its hanging Git process group was gone: %v", groupErr)
				}
				if stateOut, stateErr := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(childID)).CombinedOutput(); stateErr == nil && !strings.HasPrefix(strings.TrimSpace(string(stateOut)), "Z") {
					t.Fatalf("installed hanging Git spinner %d survived with state %q", childID, stateOut)
				}
			case <-time.After(25 * time.Second):
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				<-done
				if _, markerErr := os.Stat(marker); markerErr != nil {
					t.Fatalf("installed hanging Git wait exceeded its 25-second fixture ceiling before reaching the PATH wrapper: %v", markerErr)
				}
				t.Fatal("installed hanging Git wait exceeded its 25-second fixture ceiling after reaching the PATH wrapper")
			}
		})
	}
}

func TestWaitGoalObservationUsesBoundedBatchReads(t *testing.T) {
	t.Parallel()
	_, repo := oneClone(t)
	mustGit(t, repo, "config", "metasystem.goal.machine", "mac-a")
	seedLedger(t, repo)
	opened, err := Open(verbReq(repo, "01J5X0000000000000000000B1", "mac-a"), "goal-a", "Wait target.", "main", "Wait.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open goal-a: %+v %v", opened, err)
	}
	realRunner := attentionGitRun(runAttentionGit)
	selector := metarun.WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "human-act", After: opened.Tip}
	observe := func(pinned metarun.WaiterTarget, lastTip string) (metarun.SourceObservation, [][]string) {
		var calls [][]string
		dependencies := waitGitDependencies{run: func(ctx context.Context, root string, stdin []byte, args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return realRunner(ctx, root, stdin, args...)
		}}
		ctx := withWaitGitDependencies(context.Background(), dependencies)
		observation, observeErr := ObserveLedgerForWait(ctx, repo, selector, pinned, lastTip, "lineage-a")
		if observeErr != nil || !observation.Pending {
			t.Fatalf("observation after %s = %+v err=%v", lastTip, observation, observeErr)
		}
		return observation, calls
	}
	firstObservation, firstCalls := observe(metarun.WaiterTarget{}, "")
	const newChanges = 3
	for i, goalID := range []string{"goal-b", "goal-c", "goal-d"} {
		advanced, openErr := Open(verbReq(repo, fmt.Sprintf("01J5X0000000000000000000B%d", i+2), "mac-a"), goalID, "More files.", "main", "Wait.")
		if openErr != nil || advanced.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", goalID, advanced, openErr)
		}
	}
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

func TestWaitGoalSavedResultReplayThroughAcceptedLedger(t *testing.T) {
	t.Parallel()
	_, publisher, waiterClone := twoClones(t)
	seedLedger(t, publisher)
	mustGit(t, waiterClone, "config", "metasystem.goal.machine", "mac-waiter")
	target := "goal-replay-observer"
	opened, err := Open(verbReq(publisher, "01J5X0000000000000000000R0", "mac-a"), target, "Replay a recorded wait result.", "main", "Wait for an authenticated answer.")
	if err != nil || opened.Outcome != OutcomeConfirmed {
		t.Fatalf("open replay target: %+v %v", opened, err)
	}
	cursor := opened.Tip
	preActTips := map[string]bool{cursor: true}
	for index, goalID := range []string{"goal-replay-before-a", "goal-replay-before-b", "goal-replay-before-c", "goal-replay-before-d", "goal-replay-before-e", "goal-replay-before-f"} {
		advanced, openErr := Open(verbReq(publisher, fmt.Sprintf("01J5X0000000000000000000R%d", index+1), "mac-a"), goalID, "Advance before the answer.", "main", "Remain queued.")
		if openErr != nil || advanced.Outcome != OutcomeConfirmed {
			t.Fatalf("open pre-answer goal %s: %+v %v", goalID, advanced, openErr)
		}
		preActTips[advanced.Tip] = true
	}
	answered, err := Answer(verbReq(publisher, "01J5X0000000000000000000R7", "mac-a"), target, "replay-question", "recorded answer", "", AnswerProof{Provider: "fake", User: "human-wido", Ref: "replay/answer", Step: 1})
	if err != nil || answered.Outcome != OutcomeConfirmed {
		t.Fatalf("answer replay target: %+v %v", answered, err)
	}
	owner := metarun.Caller{Class: "MAIN", MainId: "main-replay-observer", OwnerLineage: "lineage-replay-observer", SessionId: "session-replay-observer"}
	selector := metarun.WaitSelector{Kind: "goal", TargetID: target, GoalID: target, Event: "human-act", Verb: "answer", Question: "replay-question", After: cursor}
	store := &metarun.Store{Root: waiterClone}
	options := metarun.WaitOptions{
		Observe: func(readCtx context.Context, selected metarun.WaitSelector, pinned metarun.WaiterTarget, floor string) (metarun.SourceObservation, error) {
			return ObserveLedgerForWait(readCtx, waiterClone, selected, pinned, floor, owner.OwnerLineage)
		},
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		},
	}
	saved := store.Wait(context.Background(), metarun.WaitRequest{Selector: selector, Owner: owner, RuntimeSession: "runtime-replay-observer", Timeout: time.Hour}, options)
	if saved.ExitCode != metarun.ExitGreen || saved.LedgerTip != answered.Tip {
		t.Fatalf("save real answer result: %+v", saved)
	}
	row, _, err := metarun.LoadWaiterByID(waiterClone, saved.WaitID)
	if err != nil || row.Result == nil || row.LastCheckedTip != answered.Tip {
		t.Fatalf("saved answer row=%+v err=%v", row, err)
	}

	const laterChanges = 2
	for index, goalID := range []string{"goal-replay-after-a", "goal-replay-after-b"} {
		advanced, openErr := Open(verbReq(publisher, fmt.Sprintf("01J5X0000000000000000000R%d", index+8), "mac-a"), goalID, "Advance after the answer.", "main", "Remain queued.")
		if openErr != nil || advanced.Outcome != OutcomeConfirmed {
			t.Fatalf("open post-answer goal %s: %+v %v", goalID, advanced, openErr)
		}
	}

	realRunner := attentionGitRun(runAttentionGit)
	realContext := waitGitContextFunc(func(parent context.Context, budget time.Duration, _ []string) (context.Context, context.CancelFunc) {
		return context.WithTimeout(parent, budget)
	})

	t.Run("later accepted changes replay the saved answer from its event floor", func(t *testing.T) {
		var projectedTips []string
		dependencies := waitGitDependencies{run: func(gitCtx context.Context, root string, stdin []byte, args ...string) (string, error) {
			if len(args) >= 4 && args[0] == "ls-tree" {
				projectedTips = append(projectedTips, args[3])
			}
			return realRunner(gitCtx, root, stdin, args...)
		}}
		replayed := store.ResumeWait(withWaitGitDependencies(context.Background(), dependencies), saved.WaitID, owner, owner.SessionId, 0, options)
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
		contextReads := 0
		dependencies := waitGitDependencies{}
		dependencies.withTimeout = func(parent context.Context, budget time.Duration, args []string) (context.Context, context.CancelFunc) {
			if len(args) > 0 && args[0] == "ls-tree" {
				contextReads++
				if contextReads == 3 {
					return context.WithTimeout(parent, 50*time.Millisecond)
				}
			}
			return realContext(parent, budget, args)
		}
		runnerReads := 0
		dependencies.run = func(gitCtx context.Context, root string, stdin []byte, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "ls-tree" {
				runnerReads++
				if runnerReads == 3 {
					<-gitCtx.Done()
					return "", gitCtx.Err()
				}
			}
			return realRunner(gitCtx, root, stdin, args...)
		}
		replayed := store.ResumeWait(withWaitGitDependencies(context.Background(), dependencies), saved.WaitID, owner, owner.SessionId, 0, options)
		if replayed.ExitCode != metarun.ExitWaiterIO || replayed.SourceOutcome != "transport-failure" || runnerReads != 3 {
			t.Fatalf("budget-exhausted replay=%+v intervening reads=%d", replayed, runnerReads)
		}
	})

	t.Run("cancelling an intervening read interrupts replay", func(t *testing.T) {
		reached := make(chan struct{})
		runnerReads := 0
		dependencies := waitGitDependencies{run: func(gitCtx context.Context, root string, stdin []byte, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "ls-tree" {
				runnerReads++
				if runnerReads == 3 {
					close(reached)
					<-gitCtx.Done()
					return "", gitCtx.Err()
				}
			}
			return realRunner(gitCtx, root, stdin, args...)
		}}
		ctx, cancel := context.WithCancel(context.Background())
		cancelled := make(chan struct{})
		go func() {
			<-reached
			cancel()
			close(cancelled)
		}()
		replayed := store.ResumeWait(withWaitGitDependencies(ctx, dependencies), saved.WaitID, owner, owner.SessionId, 0, options)
		<-cancelled
		if replayed.ExitCode != metarun.ExitInterrupted || replayed.SourceOutcome != "interrupted" || runnerReads != 3 {
			t.Fatalf("cancelled replay=%+v intervening reads=%d", replayed, runnerReads)
		}
	})

	t.Run("rewinding away the answer invalidates the saved evidence", func(t *testing.T) {
		mustGit(t, publisher, "push", "-q", "--force", "origin", cursor+":refs/heads/main")
		replayed := store.ResumeWait(context.Background(), saved.WaitID, owner, owner.SessionId, 0, options)
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
	mustGit(t, repo, "init", "-q", "-b", "main")
	write := func(name, body, message string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		mustGit(t, repo, "add", name)
		mustGit(t, repo, "commit", "-qm", message)
		return mustGit(t, repo, "rev-parse", "HEAD")
	}
	base := write("base", "base\n", "base")
	accepted := write("accepted", "accepted\n", "accepted")
	mustGit(t, repo, "checkout", "-qb", "other", base)
	_ = write("other", "other\n", "other")
	mustGit(t, repo, "merge", "--no-ff", "-qm", "merge accepted as second parent", accepted)
	merged := mustGit(t, repo, "rev-parse", "HEAD")
	changes, err := LedgerChanges(repo, accepted, merged)
	if err != nil || len(changes) != 1 || changes[0].Tip != merged || changes[0].Consecutive {
		t.Fatalf("second-parent ancestry invented intermediate canonical states: %+v %v", changes, err)
	}
	next := write("next", "next\n", "next")
	changes, err = LedgerChanges(repo, merged, next)
	if err != nil || len(changes) != 1 || changes[0].Tip != next || !changes[0].Consecutive {
		t.Fatalf("ordinary first-parent movement was not consecutive: %+v %v", changes, err)
	}
}

func TestLedgerChangesDoesNotDisguiseUnreadableHistoryAsARewind(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main")
	if _, err := LedgerChanges(repo, strings.Repeat("a", 40), strings.Repeat("b", 40)); err == nil || !strings.Contains(err.Error(), "walk accepted ledger changes") {
		t.Fatalf("unreadable object history was accepted as a direct transition: %v", err)
	}
}

func TestRunAttentionGitCancellationUsesInjectedTimers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustGit(t, root, "init", "-q")
	startedPath := filepath.Join(t.TempDir(), "started")
	blockedPath := filepath.Join(t.TempDir(), "blocked")
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
	go func() {
		_, runErr := runAttentionGit(ctx, root, nil, "-c", "alias.attention-hang="+alias, "attention-hang")
		result <- runErr
	}()
	startedByte := make([]byte, 1)
	if _, err := started.Read(startedByte); err != nil || string(startedByte) != "S" {
		t.Fatalf("hanging git start signal=%q err=%v", startedByte, err)
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled git returned %v", err)
	}
	created := timers.Created()
	if len(created) != 2 || created[0].kind != fakeAttentionTimer || created[0].duration != boundedCaptureGrace || created[1].kind != fakeAttentionTicker || created[1].duration != 10*time.Millisecond {
		t.Fatalf("cancelled git timers=%v, want grace %s then 10ms poll", created, boundedCaptureGrace)
	}
}

func TestCaptureTipBoundedKillsTheWholeTransportGroup(t *testing.T) {
	t.Parallel()
	timers := newFakeAttentionTimerSource()
	useCaptureTipTimerSource(t, timers)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fixture := testutil.Fixture(t)
	groupFile := filepath.Join(dir, "group")
	childFile := filepath.Join(dir, "child")
	wrapper := filepath.Join(dir, "git")
	script := "#!/bin/sh\n" + testutil.ShellPrologue + `case " $* " in
  *" fetch "*)
    trap '' TERM
	printf '%s %s\n' "$$" "$(ps -o pgid= -p $$ | tr -d ' ')" > "$LEDGER_FETCH_GROUP_FILE"
	sh -c 'trap "" TERM; echo $$ > "$LEDGER_FETCH_CHILD_FILE"; read -r _ <"${METASYSTEM_FIXTURE_LEASH:?}"' sh "$tag" &
    wait
    ;;
esac
exec "$LEDGER_REAL_GIT" "$@"
`
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	environment := testEnvironment(os.Environ(), "LEDGER_REAL_GIT="+realGit, "LEDGER_FETCH_GROUP_FILE="+groupFile,
		"LEDGER_FETCH_CHILD_FILE="+childFile, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	environment = fixture.Env(environment)

	root := t.TempDir()
	mustGit(t, root, "init", "-q")
	captured := make(chan error, 1)
	go func() {
		_, captureErr := CaptureTipBounded(Endpoint{Root: root, Remote: "blocked", Branch: "refs/heads/main", commandEnv: environment}, 300*time.Millisecond)
		captured <- captureErr
	}()
	wrapperID, groupID, childID := waitForHangingGitPIDs(t, groupFile, childFile, 25*time.Second)
	fixture.Record(wrapperID)
	fixture.Record(childID)
	if wrapperID != groupID || groupID == childID {
		t.Fatalf("invalid transport identities: wrapper=%d group=%d child=%d", wrapperID, groupID, childID)
	}
	timers.Next(t, fakeAttentionTimer, 300*time.Millisecond).Fire()
	grace := timers.Next(t, fakeAttentionTimer, boundedCaptureGrace)
	_ = timers.Next(t, fakeAttentionTicker, 10*time.Millisecond)
	if groupErr := syscall.Kill(-groupID, 0); groupErr != nil && !errors.Is(groupErr, syscall.EPERM) {
		t.Fatalf("transport process group %d did not remain during TERM grace: %v", groupID, groupErr)
	}
	grace.Fire()
	err = <-captured
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("blocking transport did not time out: %v", err)
	}
	if err := waitForGroupAbsence(groupID, 30*time.Second); err != nil {
		t.Fatal(err)
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
	useCaptureTipTimerSource(t, timers)
	if boundedCaptureGrace != 5*time.Second {
		t.Fatalf("bounded capture grace=%s, want 5s", boundedCaptureGrace)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fixture := testutil.Fixture(t)
	groupFile := filepath.Join(dir, "group")
	childFile := filepath.Join(dir, "child")
	termFile := filepath.Join(dir, "term")
	wrapper := filepath.Join(dir, "git")
	script := "#!/bin/sh\n" + testutil.ShellPrologue + `case " $* " in
  *" fetch "*)
    trap 'echo TERM > "$LEDGER_GRACE_TERM_FILE"; exit 0' TERM
	printf '%s %s\n' "$$" "$(ps -o pgid= -p $$ | tr -d ' ')" > "$LEDGER_GRACE_GROUP_FILE"
	sh -c 'echo $$ > "$LEDGER_GRACE_CHILD_FILE"; read -r _ <"${METASYSTEM_FIXTURE_LEASH:?}"' sh "$tag" &
	wait
    ;;
esac
exec "$LEDGER_GRACE_REAL_GIT" "$@"
`
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	environment := testEnvironment(os.Environ(), "LEDGER_GRACE_REAL_GIT="+realGit, "LEDGER_GRACE_GROUP_FILE="+groupFile,
		"LEDGER_GRACE_CHILD_FILE="+childFile, "LEDGER_GRACE_TERM_FILE="+termFile,
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	environment = fixture.Env(environment)
	root := t.TempDir()
	mustGit(t, root, "init", "-q")
	captured := make(chan error, 1)
	go func() {
		_, captureErr := CaptureTipBounded(Endpoint{Root: root, Remote: "blocked", Branch: "refs/heads/main", commandEnv: environment}, 300*time.Millisecond)
		captured <- captureErr
	}()
	wrapperID, groupID, childID := waitForHangingGitPIDs(t, groupFile, childFile, 25*time.Second)
	fixture.Record(wrapperID)
	fixture.Record(childID)
	if wrapperID != groupID {
		t.Fatalf("cooperative transport wrapper pid %d did not lead process group %d", wrapperID, groupID)
	}
	timers.Next(t, fakeAttentionTimer, 300*time.Millisecond).Fire()
	grace := timers.Next(t, fakeAttentionTimer, boundedCaptureGrace)
	poll := timers.Next(t, fakeAttentionTicker, 10*time.Millisecond)
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
	if grace.Fired() {
		t.Fatal("cooperative transport spent the grace timer")
	}
	if data, readErr := os.ReadFile(termFile); readErr != nil || strings.TrimSpace(string(data)) != "TERM" {
		t.Fatalf("transport received no graceful TERM opportunity: %q %v", data, readErr)
	}
	if err := waitForGroupAbsence(groupID, 30*time.Second); err != nil {
		t.Fatal(err)
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

	fireTimers bool
}

type fakeAttentionTimerInstance struct {
	kind     fakeAttentionTimerKind
	duration time.Duration
	c        chan time.Time
	mu       sync.Mutex
	fired    bool
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

func (s *fakeAttentionTimerSource) Next(t *testing.T, kind fakeAttentionTimerKind, duration time.Duration) *fakeAttentionTimerInstance {
	t.Helper()
	timer := <-s.created
	if timer.kind != kind || timer.duration != duration {
		t.Fatalf("created attention %s for %s, want %s for %s", timer.kind, timer.duration, kind, duration)
	}
	return timer
}

func (t *fakeAttentionTimerInstance) C() <-chan time.Time {
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

var captureTipTimerSourceTestMu sync.Mutex

func useCaptureTipTimerSource(t *testing.T, source attentionTimerSource) {
	t.Helper()
	captureTipTimerSourceTestMu.Lock()
	previous := replaceCaptureTipTimerSource(source)
	t.Cleanup(func() {
		replaceCaptureTipTimerSource(previous)
		captureTipTimerSourceTestMu.Unlock()
	})
}

func waitForHangingGitPIDs(t *testing.T, groupFile, childFile string, ceiling time.Duration) (int, int, int) {
	t.Helper()
	deadline := time.Now().Add(ceiling)
	for {
		groupData, groupErr := os.ReadFile(groupFile)
		childData, childErr := os.ReadFile(childFile)
		if groupErr == nil && childErr == nil && hangingGitPIDsReady(groupData, childData) {
			return parseHangingGitPIDs(t, groupData, childData)
		}
		if time.Now().After(deadline) {
			t.Fatalf("transport did not publish its process identities: group=%q err=%v child=%q err=%v", groupData, groupErr, childData, childErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
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

func parseHangingGitPIDs(t *testing.T, groupData, childData []byte) (int, int, int) {
	t.Helper()
	fields := strings.Fields(string(groupData))
	if len(fields) != 2 {
		t.Fatalf("transport wrote invalid wrapper and process-group identities: %q", groupData)
	}
	wrapperID, wrapperErr := strconv.Atoi(fields[0])
	groupID, groupErr := strconv.Atoi(fields[1])
	childID, childErr := strconv.Atoi(strings.TrimSpace(string(childData)))
	if wrapperErr != nil || groupErr != nil || childErr != nil || wrapperID < 1 || groupID < 1 || childID < 1 {
		t.Fatalf("transport wrote invalid process identities: wrapper=%q err=%v group=%q err=%v child=%q err=%v", fields[0], wrapperErr, fields[1], groupErr, childData, childErr)
	}
	return wrapperID, groupID, childID
}

func waitForGroupAbsence(pgid int, failsafe time.Duration) error {
	deadline := time.Now().Add(failsafe)
	for {
		err := syscall.Kill(-pgid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("transport process group %d still existed after the 30-second hang failsafe", pgid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestProjectAtReadsACommitWithoutMovingAnyRef(t *testing.T) {
	t.Parallel()
	_, clone := oneClone(t)
	seedLedger(t, clone)
	tip, exists, err := AcceptedLedgerTip(clone)
	if err != nil || !exists {
		t.Fatalf("accepted tip: %q %t %v", tip, exists, err)
	}
	before := mustGit(t, clone, "rev-parse", AcceptedRef)
	projection, err := ProjectAt(clone, tip)
	if err != nil || projection.Tip != tip || projection.Tree == nil {
		t.Fatalf("projection at the accepted tip: %+v err=%v", projection, err)
	}
	if after := mustGit(t, clone, "rev-parse", AcceptedRef); after != before {
		t.Fatalf("a read moved the accepted ref: %q -> %q", before, after)
	}
	if _, err := ProjectAt(clone, "0000000000000000000000000000000000000000"); err == nil {
		t.Fatal("an unreadable commit must refuse, not project")
	}
}

func TestIsAncestorAnswersAllThreeShapes(t *testing.T) {
	t.Parallel()
	_, clone := oneClone(t)
	seedLedger(t, clone)
	tip := strings.TrimSpace(mustGit(t, clone, "rev-parse", AcceptedRef))
	if ok, err := IsAncestor(clone, tip, tip); err != nil || !ok {
		t.Fatalf("a commit is its own ancestor: %t %v", ok, err)
	}
	parent := strings.TrimSpace(mustGit(t, clone, "rev-parse", tip+"^1"))
	if ok, err := IsAncestor(clone, parent, tip); err != nil || !ok {
		t.Fatalf("a first parent precedes its child: %t %v", ok, err)
	}
	if ok, err := IsAncestor(clone, tip, parent); err != nil || ok {
		t.Fatalf("a child does not precede its parent: %t %v", ok, err)
	}
}
