package goal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	channelTestQuestionID = "01J5X0000000000000000000Q1"
	channelTestGoalID     = "channel-goal"
	channelTestTime       = "2026-09-04T12:34:56Z"
)

type channelFixture struct {
	question    ChannelQuestion
	inbound     ChannelInbound
	listener    ChannelListener
	questionRaw []byte
	extra       map[string][]byte
}

func channelString(value string) *string { return &value }
func channelStep(value int64) *int64     { return &value }

func validChannelFixture() channelFixture {
	step := channelStep(123)
	receipt := channelString("answer recorded")
	thread := &ChannelRef{Provider: "telegram", ID: "question-post", ThreadID: ""}
	answerRef := ChannelRef{Provider: "telegram", ID: "42", ThreadID: "question-post"}
	return channelFixture{
		question: ChannelQuestion{
			ID: channelTestQuestionID, Goal: channelTestGoalID, Kind: "other",
			Machine: "mac-a", Lineage: "lineage-a", Opid: "question-opid",
			OpenedAt: channelTestTime, Facts: []string{}, Options: []ChannelOption{},
			Recommendation: "", Wants: "", Destination: "team", Thread: thread,
			OrphanPosts: []ChannelRef{}, Posting: nil, State: "answered",
			Answer: &ChannelAnswer{
				Text: "approved", UserID: "human", Ref: answerRef, At: channelTestTime,
				Step: step, InboxID: "telegram-42", Opid: "answer-opid", Phase: "approved",
				ApprovalULID: nil, Receipt: receipt, ReceiptRef: nil,
			},
			Rejected: []ChannelRejection{}, FactsDigest: strings.Repeat("ab", 32),
		},
		inbound: ChannelInbound{
			Provider: "telegram", Destination: "team", MessageID: "42", UpdateID: "100",
			ReplyTo: nil, UserID: "human", SentAt: channelTestTime, Text: "approved",
			Step: step, Outcome: "verified", Question: channelTestQuestionID,
			Opid: "answer-opid", ReceivedBy: "mac-a", ReceivedAt: channelTestTime,
		},
		listener: ChannelListener{
			Machine: "mac-a", Engine: "sha256:engine", LastReceiveAt: channelTestTime,
			LastConfirmAt: channelString(channelTestTime), ConflictsLastHour: 0,
			UpdatedAt: channelTestTime, Opid: "listener-opid",
		},
	}
}

func (f channelFixture) files(t *testing.T) map[string][]byte {
	t.Helper()
	files := vTree(vRoot(), []*GoalFile{vGoal(channelTestGoalID, StateQueued)}, nil)
	questionPath := ChannelPrefix + "questions/" + channelTestQuestionID + ".json"
	if f.questionRaw != nil {
		files[questionPath] = f.questionRaw
	} else {
		files[questionPath] = mustMarshalChannel(t, f.question)
	}
	files[ChannelPrefix+"inbox/team/telegram-42.json"] = mustMarshalChannel(t, f.inbound)
	files[ChannelPrefix+"listeners/mac-a.json"] = mustMarshalChannel(t, f.listener)
	for path, content := range f.extra {
		files[path] = content
	}
	return files
}

func mustMarshalChannel(t *testing.T, value any) []byte {
	t.Helper()
	b, err := MarshalChannel(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func commitChannelFiles(t *testing.T, root string, files map[string][]byte) string {
	t.Helper()
	for path, content := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustGit(t, root, "add", ".")
	mustGit(t, root, "commit", "-qm", "channel fixture")
	return mustGit(t, root, "rev-parse", "HEAD")
}

func expectChannelProblem(t *testing.T, problems []Problem, code string) {
	t.Helper()
	for _, problem := range problems {
		if strings.HasPrefix(string(problem), code+": ") {
			return
		}
	}
	t.Fatalf("no problem has code %s; got %v", code, problems)
}

func TestMarshalChannelRoundTripsEveryStruct(t *testing.T) {
	fixture := validChannelFixture()
	ref := ChannelRef{Provider: "telegram", ID: "1", ThreadID: ""}
	posting := ChannelPosting{Kind: "question", By: "mac-a", At: channelTestTime}
	values := []any{
		ref,
		ChannelOption{Label: "yes", Consequence: "continue"},
		posting,
		ChannelRejection{Ref: ref, Reason: "late", At: channelTestTime, PostRef: &ref, By: "mac-a"},
		*fixture.question.Answer,
		fixture.question,
		fixture.inbound,
		fixture.listener,
	}
	for _, value := range values {
		value := value
		t.Run(reflect.TypeOf(value).Name(), func(t *testing.T) {
			data := mustMarshalChannel(t, value)
			if len(data) == 0 || data[len(data)-1] != '\n' {
				t.Fatal("canonical channel JSON must end in one newline")
			}
			decoded := reflect.New(reflect.TypeOf(value))
			if err := json.Unmarshal(data, decoded.Interface()); err != nil {
				t.Fatal(err)
			}
			if got := decoded.Elem().Interface(); !reflect.DeepEqual(got, value) {
				t.Fatalf("round trip changed the record:\n got %#v\nwant %#v", got, value)
			}
		})
	}
}

func TestValidateChannelTreeAndCommitAcceptValidTree(t *testing.T) {
	_, root := oneClone(t)
	tip := commitChannelFiles(t, root, validChannelFixture().files(t))
	if problems := ValidateChannelTree(root, tip); len(problems) != 0 {
		t.Fatalf("valid channel tree was refused: %v", problems)
	}
	if err := ValidateCommit(root, tip); err != nil {
		t.Fatalf("ValidateCommit must accept the same tree: %v", err)
	}
}

func TestValidateChannelTreeRefusalTable(t *testing.T) {
	postedRejection := func() ChannelRejection {
		ref := ChannelRef{Provider: "telegram", ID: "rejection", ThreadID: "question-post"}
		return ChannelRejection{Ref: ref, Reason: "late", At: channelTestTime, PostRef: &ref, By: "mac-a"}
	}
	tests := []struct {
		name   string
		code   string
		mutate func(*channelFixture)
	}{
		{"unknown path", "channel-unknown-path", func(f *channelFixture) {
			f.extra = map[string][]byte{ChannelPrefix + "other.json": []byte("{}\n")}
		}},
		{"invalid json schema", "channel-json", func(f *channelFixture) {
			f.questionRaw = []byte("{\"unexpected\":true}\n")
		}},
		{"id mismatch", "channel-id-mismatch", func(f *channelFixture) { f.question.ID = "01J5X0000000000000000000Q2" }},
		{"missing goal", "channel-goal-missing", func(f *channelFixture) { f.question.Goal = "missing" }},
		{"unknown kind", "channel-kind", func(f *channelFixture) { f.question.Kind = "mystery" }},
		{"missing token", "channel-token-missing", func(f *channelFixture) { f.question.Kind = "stop" }},
		{"invalid budget", "channel-budget", func(f *channelFixture) {
			f.question.Kind = "budget-above-norm"
			f.question.Wants = "approve"
		}},
		{"answer state", "channel-answer-state", func(f *channelFixture) { f.question.State = "open" }},
		{"rejection cap", "channel-rejection-cap", func(f *channelFixture) {
			f.question.Rejected = []ChannelRejection{postedRejection(), postedRejection(), postedRejection(), postedRejection()}
		}},
		{"secret", "channel-secret", func(f *channelFixture) { f.question.Answer.Text = "approved 123456." }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, root := oneClone(t)
			fixture := validChannelFixture()
			test.mutate(&fixture)
			tip := commitChannelFiles(t, root, fixture.files(t))
			expectChannelProblem(t, ValidateChannelTree(root, tip), test.code)
			if err := ValidateCommit(root, tip); err == nil || !strings.Contains(err.Error(), test.code+": ") {
				t.Fatalf("ValidateCommit did not carry %s in its normal refusal: %v", test.code, err)
			}
		})
	}
}

func TestValidateChannelTreeSecretAndClosedNullEdges(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		mutate func(*channelFixture)
	}{
		{"mid-sentence digits accepted", "", func(f *channelFixture) { f.question.Answer.Text = "order 123456 now" }},
		{"six digit fact refused", "channel-secret", func(f *channelFixture) { f.question.Facts = []string{"machine wrote 123456 here"} }},
		{"closed without answer accepted", "", func(f *channelFixture) {
			f.question.State = "closed"
			f.question.Answer = nil
			f.question.ClosedAt = channelTestTime
			f.question.ClosedBy = "mac-a"
			f.question.ClosedBecause = "closed before the ledger inbox"
			f.inbound.Outcome = "late"
		}},
		{"closed as answered without answer refused", "channel-answer-state", func(f *channelFixture) {
			f.question.State = "closed"
			f.question.Answer = nil
			f.question.ClosedAt = channelTestTime
			f.question.ClosedBy = "mac-a"
			f.question.ClosedBecause = "answered"
			f.inbound.Outcome = "late"
		}},
		{"migrated verified join skipped", "", func(f *channelFixture) {
			f.question.Lineage = "migrated"
			f.question.State = "open"
			f.question.Answer = nil
		}},
		{"own lineage verified join required", "channel-answer-state", func(f *channelFixture) {
			f.question.Lineage = "own"
			f.question.State = "open"
			f.question.Answer = nil
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, root := oneClone(t)
			fixture := validChannelFixture()
			test.mutate(&fixture)
			tip := commitChannelFiles(t, root, fixture.files(t))
			problems := ValidateChannelTree(root, tip)
			if test.code == "" {
				if len(problems) != 0 {
					t.Fatalf("lawful edge was refused: %v", problems)
				}
			} else {
				expectChannelProblem(t, problems, test.code)
			}
		})
	}
}

func TestValidateChannelTreeJSONEdges(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*channelFixture, *testing.T)
	}{
		{"non UTC time", func(f *channelFixture, _ *testing.T) { f.listener.LastReceiveAt = "2026-09-04T14:34:56+02:00" }},
		{"null required slice", func(f *channelFixture, t *testing.T) {
			raw := string(mustMarshalChannel(t, f.question))
			f.questionRaw = []byte(strings.Replace(raw, "\"facts\": []", "\"facts\": null", 1))
		}},
		{"unknown nested key", func(f *channelFixture, t *testing.T) {
			raw := string(mustMarshalChannel(t, f.question))
			f.questionRaw = []byte(strings.Replace(raw, "\"threadId\": \"\"", "\"threadId\": \"\", \"unknown\": true", 1))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, root := oneClone(t)
			fixture := validChannelFixture()
			test.mutate(&fixture, t)
			tip := commitChannelFiles(t, root, fixture.files(t))
			expectChannelProblem(t, ValidateChannelTree(root, tip), "channel-json")
		})
	}
}

func TestChannelTimeRequiresCanonicalSecondPrecision(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"canonical", channelTestTime, false},
		{"fractional second", "2026-09-04T12:34:56.123Z", true},
		{"numeric UTC offset", "2026-09-04T12:34:56+00:00", true},
		{"lowercase z", "2026-09-04T12:34:56z", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := channelTime(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("channelTime(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
		})
	}
}

func TestValidateChannelTreeAbsentIsSilent(t *testing.T) {
	_, root := oneClone(t)
	tip := commitChannelFiles(t, root, vTree(vRoot(), []*GoalFile{vGoal(channelTestGoalID, StateQueued)}, nil))
	if problems := ValidateChannelTree(root, tip); problems != nil {
		t.Fatalf("absent channel directory must return nil, got %v", problems)
	}
	if err := ValidateCommit(root, tip); err != nil {
		t.Fatalf("absent channel directory must not change ValidateCommit: %v", err)
	}
}

func channelPosting(kind, by string) *ChannelPosting {
	return &ChannelPosting{Kind: kind, By: by, At: channelTestTime}
}

func TestChannelQuestionTuple(t *testing.T) {
	question := validChannelFixture().question
	question.Posting = channelPosting("receipt", "mac-a")
	got := question.Tuple()
	want := ChannelTuple{State: "answered", Phase: "approved", Posting: question.Posting, ThreadNull: false, ReceiptRefNull: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("question tuple mismatch: got %#v want %#v", got, want)
	}
}

func TestChannelQuestionTupleAtMarksOnlyStaleCanonicalPosting(t *testing.T) {
	question := validChannelFixture().question
	question.Posting = channelPosting("question", "mac-b")
	postedAt := time.Date(2026, time.September, 4, 12, 34, 56, 0, time.UTC)
	if question.Tuple().PostingStale {
		t.Fatal("Tuple must leave clock-relative posting staleness false")
	}
	if question.TupleAt(postedAt.Add(5*time.Minute), 10*time.Minute).PostingStale {
		t.Fatal("fresh posting must not be stale")
	}
	if !question.TupleAt(postedAt.Add(11*time.Minute), 10*time.Minute).PostingStale {
		t.Fatal("posting older than the threshold must be stale")
	}
	question.Posting.At = "2026-09-04T12:34:56.123Z"
	if question.TupleAt(postedAt.Add(11*time.Minute), 10*time.Minute).PostingStale {
		t.Fatal("non-canonical posting time must not be stale")
	}
}

func TestClassifyChannelTransitionMatrix(t *testing.T) {
	_, root := oneClone(t)
	tip := mustGit(t, root, "rev-parse", "HEAD")
	e := endpointFor(root)
	const me = "mac-a"
	tests := []struct {
		name    string
		present bool
		from    ChannelTuple
		to      *ChannelTuple
	}{
		{"ask", false, ChannelTuple{}, &ChannelTuple{State: "open", Posting: channelPosting("question", "winner"), ThreadNull: true, ReceiptRefNull: true}},
		{"migrate", false, ChannelTuple{}, &ChannelTuple{State: "closed", Posting: nil, ThreadNull: false, ReceiptRefNull: true}},
		{"post-ref question", true, ChannelTuple{State: "open", Posting: channelPosting("question", me), ThreadNull: true, ReceiptRefNull: true}, &ChannelTuple{State: "open", ThreadNull: false, ReceiptRefNull: true}},
		{"answer budget", true, ChannelTuple{State: "open", Posting: channelPosting("list", "mac-b"), ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "answered", Phase: "recorded", Posting: channelPosting("list", "mac-b"), ThreadNull: false, ReceiptRefNull: true}},
		{"answer", true, ChannelTuple{State: "open", ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "answered", Phase: "approved", ThreadNull: false, ReceiptRefNull: true}},
		{"approve-intent", true, ChannelTuple{State: "answered", Phase: "recorded", ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "answered", Phase: "recorded", Posting: channelPosting("approval", "winner"), ThreadNull: false, ReceiptRefNull: true}},
		{"approved", true, ChannelTuple{State: "answered", Phase: "recorded", Posting: channelPosting("approval", me), ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "answered", Phase: "approved", ThreadNull: false, ReceiptRefNull: true}},
		{"receipt-intent", true, ChannelTuple{State: "answered", Phase: "approved", ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "answered", Phase: "approved", Posting: channelPosting("receipt", "winner"), ThreadNull: false, ReceiptRefNull: true}},
		{"receipted", true, ChannelTuple{State: "answered", Phase: "approved", Posting: channelPosting("receipt", me), ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "closed", Phase: "receipted", ThreadNull: false, ReceiptRefNull: false}},
		{"rejection intent", true, ChannelTuple{State: "open", ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "open", Posting: channelPosting("rejection", "winner"), ThreadNull: false, ReceiptRefNull: true}},
		{"list intent", true, ChannelTuple{State: "open", ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "open", Posting: channelPosting("list", "winner"), ThreadNull: false, ReceiptRefNull: true}},
		{"silence intent", true, ChannelTuple{State: "open", ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "open", Posting: channelPosting("silence", "winner"), ThreadNull: false, ReceiptRefNull: true}},
		{"rejection ref", true, ChannelTuple{State: "open", Posting: channelPosting("rejection", me), ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "open", ThreadNull: false, ReceiptRefNull: true}},
		{"list ref", true, ChannelTuple{State: "open", Posting: channelPosting("list", me), ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "open", ThreadNull: false, ReceiptRefNull: true}},
		{"silence ref", true, ChannelTuple{State: "open", Posting: channelPosting("silence", me), ThreadNull: false, ReceiptRefNull: true}, &ChannelTuple{State: "open", ThreadNull: false, ReceiptRefNull: true}},
		{"take-over", true, ChannelTuple{State: "open", Posting: channelPosting("question", "mac-b"), PostingStale: true, ThreadNull: true, ReceiptRefNull: true}, &ChannelTuple{State: "open", Posting: channelPosting("question", "winner"), ThreadNull: true, ReceiptRefNull: true}},
		{"orphan-post", true, ChannelTuple{State: "closed", Phase: "receipted", ThreadNull: false, ReceiptRefNull: false}, nil},
		{"close", true, ChannelTuple{State: "open", ThreadNull: true, ReceiptRefNull: true}, &ChannelTuple{State: "closed", ThreadNull: true, ReceiptRefNull: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row, ok := ChannelMatrix[test.name]
			if !ok {
				t.Fatalf("matrix row %q is missing", test.name)
			}
			apply, err := ClassifyChannelTransition(e, tip, channelTestQuestionID, "operation", me, "", test.present, test.from, row)
			if err != nil || !apply {
				t.Fatalf("FROM tuple must apply: apply=%v err=%v", apply, err)
			}
			if test.to != nil {
				apply, err = ClassifyChannelTransition(e, tip, channelTestQuestionID, "operation", me, "winner", true, *test.to, row)
				var lost LostToCompetitor
				if apply || !errors.As(err, &lost) || lost.Winner != "winner" {
					t.Fatalf("TO tuple must name the other writer: apply=%v err=%v", apply, err)
				}
			}
		})
	}
}

func TestClassifyChannelTransitionAlreadyAppliedAndForeignTuple(t *testing.T) {
	_, root := oneClone(t)
	mustGit(t, root, "commit", "--allow-empty", "-q", "-m", "own transaction", "-m", "Goal-Transaction: own-opid")
	tip := mustGit(t, root, "rev-parse", "HEAD")
	e := endpointFor(root)
	foreign := ChannelTuple{State: "closed", Posting: channelPosting("list", "mac-b"), ThreadNull: false, ReceiptRefNull: false}
	apply, err := ClassifyChannelTransition(e, tip, channelTestQuestionID, "own-opid", "mac-a", "", true, foreign, ChannelMatrix["post-ref question"])
	if err != nil || apply {
		t.Fatalf("own trailer must classify AlreadyApplied: apply=%v err=%v", apply, err)
	}
	apply, err = ClassifyChannelTransition(e, tip, channelTestQuestionID, "foreign-opid", "mac-a", "", true, foreign, ChannelMatrix["post-ref question"])
	want := "channel-transition: " + channelTestQuestionID + " is (closed, null, posting list mac-b, thread set, receiptRef set), expected post-ref question"
	if apply || err == nil || err.Error() != want {
		t.Fatalf("foreign tuple mismatch:\n got apply=%v err=%v\nwant %s", apply, err, want)
	}
}

func TestRejectionIntentClosedRequiresLateReason(t *testing.T) {
	_, root := oneClone(t)
	tip := mustGit(t, root, "rev-parse", "HEAD")
	row := ChannelMatrix["rejection intent"]
	closed := ChannelTuple{State: "closed", ThreadNull: false, ReceiptRefNull: true}
	if apply, err := ClassifyChannelTransition(endpointFor(root), tip, channelTestQuestionID, "opid", "mac-a", "", true, closed, row); apply || err == nil {
		t.Fatalf("closed rejection without late reason must refuse: apply=%v err=%v", apply, err)
	}
	row.RejectionReason = "late"
	if apply, err := ClassifyChannelTransition(endpointFor(root), tip, channelTestQuestionID, "opid", "mac-a", "", true, closed, row); !apply || err != nil {
		t.Fatalf("late rejection on closed question must apply: apply=%v err=%v", apply, err)
	}
}

func TestListAndSilenceIntentsRejectClosedQuestions(t *testing.T) {
	_, root := oneClone(t)
	tip := mustGit(t, root, "rev-parse", "HEAD")
	closed := ChannelTuple{State: "closed", ThreadNull: false, ReceiptRefNull: true}
	for _, name := range []string{"list intent", "silence intent"} {
		t.Run(name, func(t *testing.T) {
			apply, err := ClassifyChannelTransition(endpointFor(root), tip, channelTestQuestionID, "opid", "mac-a", "", true, closed, ChannelMatrix[name])
			if apply || err == nil || !strings.HasPrefix(err.Error(), "channel-transition: ") {
				t.Fatalf("closed %s must refuse with a channel-transition error: apply=%v err=%v", name, apply, err)
			}
		})
	}
}

func TestTakeOverRequiresStaleForeignPosting(t *testing.T) {
	_, root := oneClone(t)
	tip := mustGit(t, root, "rev-parse", "HEAD")
	e := endpointFor(root)
	question := validChannelFixture().question
	question.State = "open"
	question.Answer = nil
	question.Posting = channelPosting("question", "mac-b")
	postedAt := time.Date(2026, time.September, 4, 12, 34, 56, 0, time.UTC)
	row := ChannelMatrix["take-over"]

	for _, test := range []struct {
		name  string
		tuple ChannelTuple
	}{
		{"fresh TupleAt", question.TupleAt(postedAt.Add(5*time.Minute), 10*time.Minute)},
		{"Tuple without a clock", question.Tuple()},
	} {
		t.Run(test.name, func(t *testing.T) {
			apply, err := ClassifyChannelTransition(e, tip, channelTestQuestionID, "opid", "mac-a", "", true, test.tuple, row)
			if apply || err == nil || !strings.HasPrefix(err.Error(), "channel-transition: ") {
				t.Fatalf("non-stale posting must refuse with a channel-transition error: apply=%v err=%v", apply, err)
			}
		})
	}

	stale := question.TupleAt(postedAt.Add(11*time.Minute), 10*time.Minute)
	if apply, err := ClassifyChannelTransition(e, tip, channelTestQuestionID, "opid", "mac-a", "", true, stale, row); !apply || err != nil {
		t.Fatalf("stale foreign posting must apply take-over: apply=%v err=%v", apply, err)
	}
}

func TestChannelInboxMutateThreeBranches(t *testing.T) {
	_, root := oneClone(t)
	e := endpointFor(root)
	recordPath := ChannelPrefix + "inbox/team/telegram-42.json"
	content := []byte("{\"opid\":\"winner\"}\n")
	tip := mustGit(t, root, "rev-parse", "HEAD")
	changes, err := ChannelInboxMutate(e, tip, recordPath, content)
	if err != nil || len(changes) != 1 || changes[0].Path != recordPath || !reflect.DeepEqual(changes[0].Content, content) {
		t.Fatalf("absent path must produce one write: changes=%v err=%v", changes, err)
	}
	fullPath := filepath.Join(root, filepath.FromSlash(recordPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "add", recordPath)
	mustGit(t, root, "commit", "-qm", "record without transaction")
	tip = mustGit(t, root, "rev-parse", "HEAD")
	if changes, err = ChannelInboxMutate(e, tip, recordPath, content); changes != nil || err == nil || err.Error() != "inbox record present without its transaction" {
		t.Fatalf("unproven record must refuse by name: changes=%v err=%v", changes, err)
	}
	mustGit(t, root, "commit", "--allow-empty", "-q", "-m", "winner transaction", "-m", "Goal-Transaction: winner")
	tip = mustGit(t, root, "rev-parse", "HEAD")
	changes, err = ChannelInboxMutate(e, tip, recordPath, content)
	var lost LostToCompetitor
	if changes != nil || !errors.As(err, &lost) || lost.Winner != "winner" {
		t.Fatalf("proven existing record must name its winner: changes=%v err=%v", changes, err)
	}
	changes, err = ChannelInboxMutate(e, "not-a-commit", recordPath, content)
	if changes != nil || err == nil {
		t.Fatalf("bogus tip must surface the Git error: changes=%v err=%v", changes, err)
	}
}

func TestChannelOpidHasGoalOperationShape(t *testing.T) {
	ulid, opid, err := ChannelOpid("mac-a", "lineage-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(ulid) != 26 || !strings.HasPrefix(opid, ulid+"-mac-a-") || !validOpidShape(opid) {
		t.Fatalf("channel operation identity has the wrong shape: ulid=%q opid=%q", ulid, opid)
	}
}
