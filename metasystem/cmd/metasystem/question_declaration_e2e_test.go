package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// The ledger adapter is part of this claim: a wait already registered on a
// physical ledger cursor must observe the answer publication made by the
// terminal act. An in-memory repository cannot exercise that fetch boundary.
func TestQ2DeclaredActWakesWaitingStewardGitAdapter(t *testing.T) {
	root, _ := blockedQuestionRoot(t)
	writeCriticChain(t, root)
	owners := defaultIntentOwners()
	owners.resolver = stateroot.NewResolver(fakeTop(root), noExecutable)
	// This test has no incoming channel replies. Disable only provider polling;
	// the real channel wait still registers and observes the goal's answer event.
	calls := defaultIntentOwnerCalls()
	calls.channelWait = func(caller ownercall.Process, lineage string, stdout, stderr io.Writer, args []string) int {
		return channelWaitWith(caller.Pid, lineage, stdout, stderr, append(args, "--poll-seconds", "0"), nil)
	}
	owners.delivery = &intentDeliveryOwners{calls: calls}
	human := owners
	human.dependencies.ownerLineage = func() string { return "" }
	human.prove = enrolledPersonProver(t, root, blockedQuestionNow())
	run := func(o intentOwners, args ...string) (int, intentResult) {
		var out, errout bytes.Buffer
		code := runIntentIn(mustIntentArgvCommand(t, args), append(intentArgvRest(args), "--json"), &out, &errout, root, o)
		var result intentResult
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Errorf("command result: %v %s %s", err, out.String(), errout.String())
		}
		return code, result
	}
	command := "metasystem goal accept-risk " + bedGoal + " --review critic-r2 --finding S-1"
	code, asked := run(owners, "question", "ask", bedGoal, "--question", "Accept this risk?", "--option", "Accept: keep the risk recorded", "--option-act", "Accept: "+command, "--option", "Wait: leave the work waiting", "--recommend", "Accept")
	if code != 0 {
		t.Fatalf("steward asks: %d %+v", code, asked)
	}
	questions, bad := channel.WalkQuestions(root)
	if len(questions) != 1 || len(bad) != 0 {
		t.Fatalf("questions=%d unreadable=%d", len(questions), len(bad))
	}
	q := questions[0]
	// The incident starts with a ready held goal and no living seat. The only
	// changing input to its next decision is the channel's open-question set.
	decide := func() (steward.Decision, *steward.SeatSelection) {
		work := goal.ClaimableBudgetedWork{Claimed: []string{bedGoal}, GoalFacts: map[string]goal.GoalFacts{bedGoal: {Id: bedGoal}}, Eligibility: map[string]goal.ClaimEligibility{bedGoal: {Ready: true}}}
		work.MarkAskedOpen(channel.GoalOpenQuestions(root))
		world := steward.SeatWorldFrom(work, nil, goal.GateSettings{}, nil, blockedQuestionNow())
		world.Held[0].Lineage = steward.SeatLineage
		return steward.PlanSeat(world, nil, 3, false)
	}
	if decision, selection := decide(); selection != nil || !strings.Contains(decision.Reason, "open question") {
		t.Fatalf("seat was not held: %+v %+v", decision, selection)
	}
	type waited struct {
		code   int
		result intentResult
	}
	finished := make(chan waited, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		code, result := run(owners, "question", "wait", "channel:"+q.ID, "--timeout", "1m")
		finished <- waited{code, result}
	}()
	t.Cleanup(func() { <-done })
	stateRoot, err := goal.ResolveStateRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	var waitID string
	testenv.Await(t, "the question wait to register before the terminal act", func() bool {
		rows, problems := metarun.PendingWaiters(stateRoot, "m1")
		if len(problems) != 0 {
			t.Fatalf("wait registrations: %v", problems)
		}
		for _, row := range rows {
			if row.Selector.Question == q.ID {
				waitID = row.WaitID
				return true
			}
		}
		select {
		case ended := <-finished:
			t.Fatalf("wait returned before the act: %d %+v", ended.code, ended.result)
		default:
		}
		return false
	})
	stillOpen, err := channel.ReadQuestion(root, q.ID)
	if err != nil || stillOpen.Answer != nil || stillOpen.State != "open" {
		t.Fatalf("not waiting on an open question: %+v %v", stillOpen, err)
	}
	// Run the canonical command the person is shown, including its required reason.
	display := q.Options[0].NamedAct.DisplayCommand()
	if !strings.Contains(q.Text, display) {
		t.Fatalf("question did not show its executable act: %s", q.Text)
	}
	code, accepted := run(human, shellWords(display)[1:]...)
	if code != 0 {
		t.Fatalf("terminal act: %d %+v", code, accepted)
	}
	var got waited
	testenv.Await(t, "the already waiting caller to receive Accept", func() bool {
		select {
		case got = <-finished:
			return true
		default:
			return false
		}
	})
	if got.code != 0 || got.result.Outcome != intentConfirmed || !strings.Contains(strings.Join(got.result.Details, " ")+stringifyQ2(got.result.Data), "Accept") {
		t.Fatalf("waiting result: %d %+v", got.code, got.result)
	}
	closed, err := channel.ReadQuestion(root, q.ID)
	if err != nil || closed.State != "closed" || closed.Answer == nil || closed.Answer.Text != "Accept" || closed.Answer.Act == nil || closed.Answer.Act.Person != "Wido" {
		t.Fatalf("closed question: %+v %v", closed, err)
	}
	row, _, err := metarun.FindWaiterByID(stateRoot, waitID)
	if err != nil || row.Result == nil || row.Result.ExitCode != 0 {
		t.Fatalf("durable waiting result: %+v %v", row, err)
	}
	if decision, selection := decide(); decision.Action != steward.ActRevive || selection == nil || selection.Goal != bedGoal {
		t.Fatalf("seat still held: %+v %+v", decision, selection)
	}
}

func stringifyQ2(value any) string { data, _ := json.Marshal(value); return string(data) }
