package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func holdAnswerQuestion(t *testing.T, root, id, label string, act []string) channel.Question {
	t.Helper()
	q, err := channel.Ask(channel.AskRequest{RepoRoot: root, About: "machine", Machine: "mac-cli", Kind: "other", Contract: true, Now: boxStop,
		Decision: &channel.Decision{Kind: "steward-hold", Subject: id, State: "restart cap"}, Facts: []string{"the seat is held"},
		Plain:   &channel.Plain{Decision: "The seat is held until you choose.", IfNothing: "it stays down."},
		Options: []channel.Option{{Label: label, Act: act, Consequence: "the seat can start again."}}})
	if err != nil {
		t.Fatal(err)
	}
	return recordChoice(t, root, q, func(q *channel.Question) { q.Answer.Text, q.Answer.Lineage = strings.ToLower(label), "steward" })
}

func TestHoldAnswerApproveKeepsTheClaimBox(t *testing.T) {
	t.Parallel()
	for _, terminal := range []bool{false, true} {
		t.Run(map[bool]string{false: "channel", true: "terminal"}[terminal], func(t *testing.T) {
			t.Parallel()
			bed := newIntentBed(t, false, func(f *goal.GoalFile) {
				f.Tier = 2
				f.Budget = &goal.Budget{ElapsedLimit: "5d", AttemptLimit: 12, ReservedJobMinutesLimit: 2400, ActiveJobLimit: 1, ReviewRoundLimit: 20}
				f.Approved.EpisodeRevision = f.Approved.Revision
				f.Approved.Digest = goal.ApprovalDigest(f.Intent, f.Tier, *f.Budget, f.Risk)
				f.Claimed.EpisodeAt, f.Claimed.EpisodeRevision = f.Claimed.At, f.Claimed.Revision
			})
			channelFor(t, bed.root())
			before := bed.goalFile(bedGoal)
			q := holdAnswerQuestion(t, bed.root(), bedGoal, "Approve it again", []string{"metasystem", "goal", "approve", bedGoal})
			var code int
			var result intentResult
			if terminal {
				code, result = bed.runJSON(bed.owners(), "goal", "approve", bedGoal, "--fixture-human-authority", "--lineage", "m1")
			} else {
				code, result = boxAnswer(t, bed, q.ID, "goal", "approve", bedGoal)
				if state, _ := answerState(t, result); state != "started" {
					t.Fatalf("the channel approval did not start: %d %+v", code, result)
				}
			}
			after := bed.goalFile(bedGoal)
			if code != 0 || after.Approved.Opid == before.Approved.Opid || after.Claimed == nil {
				t.Fatalf("the claim did not get a fresh approval: %d %+v", code, result)
			}
			if !terminal && (*after.Budget != *before.Budget || !reflect.DeepEqual(after.Claimed, before.Claimed) || goal.BudgetEpisodeRevision(after) != goal.BudgetEpisodeRevision(before) || after.Approved.Authority != goal.ApprovalAuthorityChannel) {
				t.Fatalf("channel approval refunded the box or episode: before %+v after %+v", before, after)
			}
			if terminal && (after.Budget.ElapsedLimit != "4h" || after.Episode != nil) {
				t.Fatalf("terminal approval did not take the tier box afresh: %+v", after)
			}
			records := []steward.SeatRecord{{Goal: bedGoal, ApprovalOpid: before.Approved.Opid, Outcome: steward.SeatNoProgress}}
			if steward.SeatNoProgressCount(records, bedGoal, after.Approved.Opid) != 0 {
				t.Fatal("fresh approval still counts the old seats")
			}
		})
	}
	t.Run("another goal", func(t *testing.T) {
		t.Parallel()
		bed := newIntentBed(t, false, nil)
		channelFor(t, bed.root())
		q := holdAnswerQuestion(t, bed.root(), "another-goal", "Approve it again", []string{"metasystem", "goal", "approve", "another-goal"})
		code, result := boxAnswer(t, bed, q.ID, "goal", "approve", bedGoal)
		if code == 0 || !strings.Contains(result.Summary, "this answer authorizes") {
			t.Fatalf("another goal's answer acted: %d %+v", code, result)
		}
	})
}

func TestHoldAnswerMachineRevive(t *testing.T) {
	t.Parallel()
	machineRevivePublicAct(t, []string{"answer-stranded-continuation", "answer-other-seat", "answer-closed", "agent"})
}

func TestHoldAnswerFlagsAreAdvanced(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"machine revive", "goal approve"} {
		command, ok := findIntentCommand(name)
		if !ok {
			t.Fatal(name)
		}
		flag, ok := command.lookupFlag("answer")
		if !ok || flag.value != "Q" || !flag.advanced {
			t.Fatalf("%s answer flag: %+v", name, flag)
		}
		if _, problem := parseIntentArgs(command, []string{"target", "--answer", "question"}); problem != nil {
			t.Fatalf("%s answer: %+v", name, problem)
		}
	}
}
