package goal

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func openInboxQuestion(id, goalID, wants string) *ChannelQuestion {
	return &ChannelQuestion{
		ID: id, Goal: goalID, Kind: "other", Machine: "mac-a", Lineage: "lineage-a", Opid: "question-opid",
		OpenedAt: channelTestTime, Facts: []string{}, Options: []ChannelOption{}, Recommendation: "", Wants: wants,
		Destination: "team", Thread: &ChannelRef{Provider: "telegram", ID: "question-post", ThreadID: ""},
		OrphanPosts: []ChannelRef{}, Posting: nil, State: "open", Answer: nil, Rejected: []ChannelRejection{},
		FactsDigest: strings.Repeat("ab", 32),
	}
}

func TestReadChannelTree(t *testing.T) {
	t.Run("missing directory is empty", func(t *testing.T) {
		_, root := oneClone(t)
		tip := mustGit(t, root, "rev-parse", "HEAD")
		tree, err := ReadChannelTree(endpointFor(root), tip)
		if err != nil || len(tree.Questions) != 0 || len(tree.Inbox) != 0 || len(tree.Listeners) != 0 {
			t.Fatalf("missing directory: tree=%+v err=%v", tree, err)
		}
	})

	t.Run("all record kinds use path keys", func(t *testing.T) {
		_, root := oneClone(t)
		tip := commitChannelFiles(t, root, validChannelFixture().files(t))
		tree, err := ReadChannelTree(endpointFor(root), tip)
		if err != nil {
			t.Fatal(err)
		}
		if tree.Questions[channelTestQuestionID] == nil || tree.Inbox["team/telegram-42"] == nil || tree.Listeners["mac-a"] == nil {
			t.Fatalf("record keys are wrong: questions=%v inbox=%v listeners=%v", reflect.ValueOf(tree.Questions).MapKeys(), reflect.ValueOf(tree.Inbox).MapKeys(), reflect.ValueOf(tree.Listeners).MapKeys())
		}
	})

	t.Run("decode refusal names the path", func(t *testing.T) {
		_, root := oneClone(t)
		path := ChannelPrefix + "questions/" + channelTestQuestionID + ".json"
		files := vTree(vRoot(), []*GoalFile{vGoal(channelTestGoalID, StateQueued)}, nil)
		files[path] = []byte("{\"unexpected\":true}\n")
		tip := commitChannelFiles(t, root, files)
		_, err := ReadChannelTree(endpointFor(root), tip)
		if err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("decode error must name %s: %v", path, err)
		}
	})
}

func TestMatchChannelInboundThreadReferences(t *testing.T) {
	question := openInboxQuestion(channelTestQuestionID, channelTestGoalID, "token")
	postRef := &ChannelRef{Provider: "telegram", ID: "rejection-post", ThreadID: "question-post"}
	question.Rejected = []ChannelRejection{{Ref: ChannelRef{Provider: "telegram", ID: "reply", ThreadID: "question-post"}, PostRef: postRef}}
	question.OrphanPosts = []ChannelRef{{Provider: "telegram", ID: "orphan-post", ThreadID: ""}}
	receiptRef := &ChannelRef{Provider: "telegram", ID: "receipt-post", ThreadID: "question-post"}
	question.Answer = &ChannelAnswer{ReceiptRef: receiptRef}
	tree := &ChannelTree{Questions: map[string]*ChannelQuestion{question.ID: question}, Inbox: map[string]*ChannelInbound{}, Listeners: map[string]*ChannelListener{}}

	for _, reply := range []string{"question-post", "rejection-post", "orphan-post", "receipt-post"} {
		t.Run(reply, func(t *testing.T) {
			record := ChannelInbound{Destination: "team", ReplyTo: channelString(reply), Outcome: "verified"}
			got, bound := MatchChannelInbound(tree, record, func(*ChannelQuestion) bool { return false })
			if got != question.ID || !bound {
				t.Fatalf("reply %q matched %q bound=%v", reply, got, bound)
			}
		})
	}
	record := ChannelInbound{Destination: "team", ReplyTo: channelString("question-post"), Outcome: "wrong-user"}
	if got, bound := MatchChannelInbound(tree, record, func(*ChannelQuestion) bool { return false }); got != question.ID || bound {
		t.Fatalf("unverified thread matched %q bound=%v", got, bound)
	}
	question.State = "answered"
	record.Outcome = "verified"
	if got, bound := MatchChannelInbound(tree, record, func(*ChannelQuestion) bool { return false }); got != question.ID || bound {
		t.Fatalf("answered thread matched %q bound=%v", got, bound)
	}
}

func matchByToken(tree *ChannelTree, record ChannelInbound) (string, bool) {
	return MatchChannelInbound(tree, record, func(q *ChannelQuestion) bool {
		return channelTokenAppearsOnce(record.Text, q.Wants)
	})
}

func TestMatchChannelInboundUnthreadedTokens(t *testing.T) {
	q1 := openInboxQuestion("01J5X0000000000000000000Q1", "goal-one", "goal=one resume elapsed=1d attempts=10 minutes=1200 active=1")
	q2 := openInboxQuestion("01J5X0000000000000000000Q2", "goal-two", "goal=two start")
	record := ChannelInbound{Destination: "team", Outcome: "verified"}

	assert := func(t *testing.T, tree *ChannelTree, text, want string, bound bool) {
		t.Helper()
		record.Text = text
		got, gotBound := matchByToken(tree, record)
		if got != want || gotBound != bound {
			t.Fatalf("text %q matched %q bound=%v, want %q bound=%v", text, got, gotBound, want, bound)
		}
	}
	t.Run("zero tokens with one open", func(t *testing.T) {
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{q1.ID: q1}}, "no token", "unbound", false)
	})
	t.Run("zero tokens with two open", func(t *testing.T) {
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{q1.ID: q1, q2.ID: q2}}, "no token", "unmatched", false)
	})
	t.Run("one token", func(t *testing.T) {
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{q1.ID: q1, q2.ID: q2}}, q2.Wants, q2.ID, true)
	})
	t.Run("two tokens", func(t *testing.T) {
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{q1.ID: q1, q2.ID: q2}}, q1.Wants+" "+q2.Wants, "unmatched", false)
	})
	t.Run("one token repeated", func(t *testing.T) {
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{q2.ID: q2}}, q2.Wants+" "+q2.Wants, "unmatched", false)
	})
	t.Run("last token field accepts punctuation", func(t *testing.T) {
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{q1.ID: q1}}, q1.Wants+".", q1.ID, true)
	})
	t.Run("partial token does not match", func(t *testing.T) {
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{q1.ID: q1}}, "goal=one resume", "unbound", false)
	})
	t.Run("unposted question does not bind", func(t *testing.T) {
		copy := *q1
		copy.Thread = nil
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{copy.ID: &copy}}, copy.Wants, "unbound", false)
	})
	t.Run("case differs", func(t *testing.T) {
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{q2.ID: q2}}, strings.ToUpper(q2.Wants), "unbound", false)
	})
	t.Run("unverified unthreaded", func(t *testing.T) {
		record.Outcome = "wrong-user"
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{q2.ID: q2}}, q2.Wants, "unmatched", false)
		record.Outcome = "verified"
	})
	t.Run("no open questions", func(t *testing.T) {
		copy := *q2
		copy.State = "answered"
		assert(t, &ChannelTree{Questions: map[string]*ChannelQuestion{copy.ID: &copy}}, copy.Wants, "unmatched", false)
	})
}

func channelPublishBed(t *testing.T, question *ChannelQuestion) (a, b string) {
	t.Helper()
	_, a, b = twoClones(t)
	files := vTree(vRoot(), []*GoalFile{vGoal(question.Goal, StateQueued)}, nil)
	files[ChannelPrefix+"questions/"+question.ID+".json"] = mustMarshalChannel(t, question)
	commitChannelFiles(t, a, files)
	mustGit(t, a, "push", "-q", "origin", "HEAD:main")
	return a, b
}

func publishRecord(messageID string, step int64) ChannelInbound {
	return ChannelInbound{
		Provider: "telegram", Destination: "team", MessageID: messageID, UpdateID: messageID,
		ReplyTo: channelString("question-post"), UserID: "human", SentAt: channelTestTime,
		Text: "human answer", Step: channelStep(step), Outcome: "verified", ReceivedBy: "mac-a", ReceivedAt: channelTestTime,
	}
}

func channelPublishNow() time.Time {
	return time.Date(2026, time.September, 4, 12, 40, 0, 0, time.UTC)
}

func TestChannelInboundPublishAtomicAnswerReplayLateAndLoss(t *testing.T) {
	question := openInboxQuestion(channelTestQuestionID, channelTestGoalID, "required token")
	a, b := channelPublishBed(t, question)
	eA := endpointFor(a)
	record := publishRecord("42", 123)
	opid := Opid("01J5X0000000000000000000H1", "mac-a", "lineage-a")
	var decided ChannelInbound
	result, err := Publish(eA, ChannelInboundRequest(eA, "mac-a", "lineage-a", opid, record, time.Date(2026, 9, 4, 12, 35, 0, 0, time.UTC), &decided))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("bound publish: result=%+v err=%v", result, err)
	}
	if err := ValidateCommit(a, result.Commit); err != nil {
		t.Fatalf("landed commit did not validate: %v", err)
	}
	tree, err := ReadChannelTree(eA, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	landedRecord := tree.Inbox["team/telegram-42"]
	landedQuestion := tree.Questions[question.ID]
	goals, err := loadTree(a, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	history := goals.Live[question.Goal].History[len(goals.Live[question.Goal].History)-1]
	if landedRecord == nil || landedRecord.Opid != opid || landedRecord.Question != question.ID || !reflect.DeepEqual(*landedRecord, decided) {
		t.Fatalf("record did not land under the answer opid: landed=%+v decided=%+v", landedRecord, decided)
	}
	if landedQuestion.State != "answered" || landedQuestion.Answer == nil || landedQuestion.Answer.Opid != opid || landedQuestion.Answer.InboxID != "telegram-42" || landedQuestion.Answer.Phase != "approved" {
		t.Fatalf("question answer is wrong: %+v", landedQuestion)
	}
	if history.Opid != opid || history.Reason != record.Text || history.ChannelStep != 123 || history.ChannelRef != "question-post/42" || strings.Contains(history.Reason, question.Wants) {
		t.Fatalf("goal answer row is not the verbatim one-opid proof: %+v", history)
	}

	eB := endpointFor(b)
	loserOpid := Opid("01J5X0000000000000000000H2", "mac-b", "lineage-b")
	before := mustGit(t, a, "ls-remote", "origin", "refs/heads/main")
	loser, err := Publish(eB, ChannelInboundRequest(eB, "mac-b", "lineage-b", loserOpid, record, time.Date(2026, 9, 4, 12, 36, 0, 0, time.UTC), nil))
	after := mustGit(t, a, "ls-remote", "origin", "refs/heads/main")
	if err != nil || loser.Outcome != OutcomeLost || !strings.Contains(loser.Detail, opid) || before != after {
		t.Fatalf("second clone must lose without a write: result=%+v err=%v before=%q after=%q", loser, err, before, after)
	}

	replay := publishRecord("43", 123)
	replayOpid := Opid("01J5X0000000000000000000H3", "mac-a", "lineage-a")
	var replayed ChannelInbound
	replayResult, err := Publish(eA, ChannelInboundRequest(eA, "mac-a", "lineage-a", replayOpid, replay, time.Date(2026, 9, 4, 12, 37, 0, 0, time.UTC), &replayed))
	if err != nil || replayResult.Outcome != OutcomeConfirmed || replayed.Outcome != "replayed" || replayed.Question != question.ID {
		t.Fatalf("replay result=%+v decided=%+v err=%v", replayResult, replayed, err)
	}
	tree, _ = ReadChannelTree(eA, replayResult.Tip)
	if tree.Questions[question.ID].Answer.Opid != opid {
		t.Fatalf("replayed message advanced the question: %+v", tree.Questions[question.ID].Answer)
	}

	late := publishRecord("44", 124)
	lateOpid := Opid("01J5X0000000000000000000H4", "mac-a", "lineage-a")
	var lateDecided ChannelInbound
	lateResult, err := Publish(eA, ChannelInboundRequest(eA, "mac-a", "lineage-a", lateOpid, late, time.Date(2026, 9, 4, 12, 38, 0, 0, time.UTC), &lateDecided))
	if err != nil || lateResult.Outcome != OutcomeConfirmed || lateDecided.Outcome != "late" || lateDecided.Question != question.ID {
		t.Fatalf("late result=%+v decided=%+v err=%v", lateResult, lateDecided, err)
	}
	tree, _ = ReadChannelTree(eA, lateResult.Tip)
	if tree.Questions[question.ID].Answer.Opid != opid {
		t.Fatalf("late message advanced the question: %+v", tree.Questions[question.ID].Answer)
	}
}

func TestChannelInboundPublishBudgetAnswerRows(t *testing.T) {
	tests := []struct {
		name         string
		messageID    string
		step         int64
		ulid         string
		text         string
		row          string
		phase        string
		approvalULID *string
		receipt      *string
	}{
		{
			name:         "matching token records budget approval",
			messageID:    "52",
			step:         223,
			ulid:         "01J5X0000000000000000000R1",
			text:         "approve exactly",
			row:          "answer budget",
			phase:        "recorded",
			approvalULID: channelString("01M1P6MEC0DTSKCJCZ05C283YB"),
		},
		{
			name:      "different text records box not raised",
			messageID: "53",
			step:      224,
			ulid:      "01J5X0000000000000000000R2",
			text:      "different reply",
			row:       "answer",
			phase:     "approved",
			receipt:   channelString("recorded: channel-goal box not raised; the reply did not carry the token"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			question := openInboxQuestion(channelTestQuestionID, channelTestGoalID, "approve exactly")
			question.Kind = "budget-above-norm"
			question.Budget = &Budget{ElapsedLimit: "1d", AttemptLimit: 10, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3}
			a, _ := channelPublishBed(t, question)
			e := endpointFor(a)
			record := publishRecord(test.messageID, test.step)
			record.Text = test.text
			opid := Opid(test.ulid, "mac-a", "lineage-a")
			budgetRow := ChannelMatrix["answer budget"]
			answerRow := ChannelMatrix["answer"]
			budgetCalls, answerCalls := 0, 0
			budgetProbe := budgetRow
			budgetProbe.From = func(tuple ChannelTuple, me string) bool {
				budgetCalls++
				return budgetRow.From(tuple, me)
			}
			answerProbe := answerRow
			answerProbe.From = func(tuple ChannelTuple, me string) bool {
				answerCalls++
				return answerRow.From(tuple, me)
			}
			ChannelMatrix["answer budget"] = budgetProbe
			ChannelMatrix["answer"] = answerProbe
			defer func() {
				ChannelMatrix["answer budget"] = budgetRow
				ChannelMatrix["answer"] = answerRow
			}()

			result, err := Publish(e, ChannelInboundRequest(e, "mac-a", "lineage-a", opid, record, channelPublishNow(), nil))
			if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("budget answer publish: result=%+v err=%v", result, err)
			}
			if test.row == "answer budget" && (budgetCalls == 0 || answerCalls != 0) || test.row == "answer" && (answerCalls == 0 || budgetCalls != 0) {
				t.Fatalf("publish selected the wrong matrix row: want=%q answer-budget calls=%d answer calls=%d", test.row, budgetCalls, answerCalls)
			}
			if err := ValidateCommit(a, result.Commit); err != nil {
				t.Fatalf("budget answer commit did not validate: %v", err)
			}

			tree, err := ReadChannelTree(e, result.Tip)
			if err != nil {
				t.Fatal(err)
			}
			landed := tree.Questions[question.ID]
			if landed == nil || landed.State != "answered" || landed.Answer == nil {
				t.Fatalf("budget question was not answered: %+v", landed)
			}
			if landed.Answer.Phase != test.phase || landed.Answer.Opid != opid || !reflect.DeepEqual(landed.Answer.ApprovalULID, test.approvalULID) || !reflect.DeepEqual(landed.Answer.Receipt, test.receipt) {
				t.Fatalf("budget answer row is wrong: %+v", landed.Answer)
			}

			goals, err := loadTree(a, result.Tip)
			if err != nil {
				t.Fatal(err)
			}
			history := goals.Live[question.Goal].History[len(goals.Live[question.Goal].History)-1]
			if history.Opid != opid || history.Reason != record.Text || history.ChannelStep != test.step {
				t.Fatalf("budget goal answer row is not verbatim one-opid proof: %+v", history)
			}
		})
	}
}

func TestChannelInboundFreshRetryLosesAfterAbandonedAttempt(t *testing.T) {
	question := openInboxQuestion(channelTestQuestionID, channelTestGoalID, "token")
	a, b := channelPublishBed(t, question)
	record := publishRecord("42", 123)
	abandonedOpid := Opid("01J5X0000000000000000000J1", "mac-b", "lineage-b")
	badEndpoint := endpointFor(b)
	badEndpoint.Remote = "missing-remote"
	if _, err := Publish(badEndpoint, ChannelInboundRequest(badEndpoint, "mac-b", "lineage-b", abandonedOpid, record, channelPublishNow(), nil)); err == nil {
		t.Fatal("missing remote must fail capture")
	}
	abandoned, err := ReadEntry(b, abandonedOpid)
	if err != nil || abandoned.Phase != PhaseTerminal || abandoned.Outcome != OutcomeAbandoned {
		t.Fatalf("capture failure was not journaled abandoned: entry=%+v err=%v", abandoned, err)
	}

	winnerOpid := Opid("01J5X0000000000000000000J2", "mac-a", "lineage-a")
	eA := endpointFor(a)
	if result, err := Publish(eA, ChannelInboundRequest(eA, "mac-a", "lineage-a", winnerOpid, record, channelPublishNow(), nil)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("winner did not land: result=%+v err=%v", result, err)
	}
	retryOpid := Opid("01J5X0000000000000000000J3", "mac-b", "lineage-b")
	eB := endpointFor(b)
	retry, err := Publish(eB, ChannelInboundRequest(eB, "mac-b", "lineage-b", retryOpid, record, channelPublishNow(), nil))
	if err != nil || retry.Outcome != OutcomeLost || !strings.Contains(retry.Detail, winnerOpid) {
		t.Fatalf("fresh retry did not lose to the committed record: result=%+v err=%v", retry, err)
	}
	if retryEntry, readErr := ReadEntry(b, retryOpid); readErr != nil || retryEntry.Outcome != OutcomeLost {
		t.Fatalf("retry journal entry is wrong: entry=%+v err=%v", retryEntry, readErr)
	}
}

func TestChannelAnswerDispositionAndApprovalULID(t *testing.T) {
	question := openInboxQuestion(channelTestQuestionID, channelTestGoalID, "approve exactly")
	record := publishRecord("42", 123)
	record.Text = question.Wants
	question.Kind = "budget-above-norm"
	question.Budget = &Budget{ElapsedLimit: "1d", AttemptLimit: 10, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3}
	phase, approval, receipt, err := channelAnswerDisposition(question, record)
	if err != nil || phase != "recorded" || approval == nil || *approval != "01M1P6MEC0DTSKCJCZ05C283YB" || receipt != nil {
		t.Fatalf("budget disposition: phase=%q approval=%v receipt=%v err=%v", phase, approval, receipt, err)
	}
	record.Text = "no"
	phase, approval, receipt, err = channelAnswerDisposition(question, record)
	if err != nil || phase != "approved" || approval != nil || receipt == nil || *receipt != "recorded: channel-goal box not raised; the reply did not carry the token" {
		t.Fatalf("budget refusal disposition: phase=%q approval=%v receipt=%v err=%v", phase, approval, receipt, err)
	}
	question.Budget = nil
	_, _, receipt, _ = channelAnswerDisposition(question, record)
	if receipt == nil || *receipt != "recorded: channel-goal has no proposed box on this question; nothing raised" {
		t.Fatalf("missing tuple receipt=%v", receipt)
	}
	question.Kind = "stop"
	record.Text = question.Wants + "."
	_, _, receipt, _ = channelAnswerDisposition(question, record)
	if receipt == nil || *receipt != "recorded: channel-goal approved for execution" {
		t.Fatalf("stop receipt=%v", receipt)
	}
}

func TestChannelInboundPresentWithoutTrailerRefuses(t *testing.T) {
	question := openInboxQuestion(channelTestQuestionID, channelTestGoalID, "token")
	a, _ := channelPublishBed(t, question)
	record := publishRecord("42", 123)
	record.Opid = "missing-transaction"
	record.Question = "unmatched"
	path := ChannelPrefix + "inbox/team/telegram-42.json"
	fullPath := filepath.Join(a, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, mustMarshalChannel(t, record), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, a, "add", path)
	mustGit(t, a, "commit", "-qm", "unproven inbox record")
	mustGit(t, a, "push", "-q", "origin", "HEAD:main")
	e := endpointFor(a)
	opid := Opid("01J5X0000000000000000000K1", "mac-a", "lineage-a")
	result, err := Publish(e, ChannelInboundRequest(e, "mac-a", "lineage-a", opid, publishRecord("42", 123), channelPublishNow(), nil))
	if err != nil || result.Outcome != OutcomeRejected || result.Detail != "inbox record present without its transaction" {
		t.Fatalf("unproven record result=%+v err=%v", result, err)
	}
	entry, readErr := ReadEntry(a, opid)
	if readErr != nil || entry.Outcome != OutcomeRejected {
		t.Fatalf("refusal journal entry=%+v err=%v", entry, readErr)
	}
	var lost LostToCompetitor
	if errors.As(err, &lost) {
		t.Fatalf("unproven record was misclassified as a competitor loss: %v", err)
	}
}
