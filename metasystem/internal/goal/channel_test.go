package goal

import (
	"encoding/json"
	"strings"
	"testing"
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
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func commitChannelFilesForEndpoint(t *testing.T, e Endpoint, files map[string][]byte) string {
	t.Helper()
	tip, err := e.Repository.Capture("channel-fixture")
	if err != nil {
		t.Fatal(err)
	}
	changes := make([]Change, 0, len(files))
	for _, path := range sortedKeys(files) {
		changes = append(changes, Change{Path: path, Content: files[path]})
	}
	commit, err := e.Repository.Build("channel-fixture", tip, changes, "channel fixture")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := e.Repository.Publish(tip, commit); err != nil || outcome != CASLanded {
		t.Fatalf("publish channel fixture: outcome=%s err=%v", outcome, err)
	}
	return commit
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

func TestValidateChannelTreeAndCommitAcceptValidTree(t *testing.T) {
	t.Parallel()
	e, _ := fakeGoalEndpoint(t)
	tip := commitChannelFilesForEndpoint(t, e, validChannelFixture().files(t))
	if problems := validateChannelTreeFor(e, tip); len(problems) != 0 {
		t.Fatalf("valid channel tree was refused: %v", problems)
	}
	if err := validateCommitFor(e, tip); err != nil {
		t.Fatalf("ValidateCommit must accept the same tree: %v", err)
	}
}

func TestValidateChannelTreeRefusalTable(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			e, _ := fakeGoalEndpoint(t)
			fixture := validChannelFixture()
			test.mutate(&fixture)
			tip := commitChannelFilesForEndpoint(t, e, fixture.files(t))
			expectChannelProblem(t, validateChannelTreeFor(e, tip), test.code)
			if err := validateCommitFor(e, tip); err == nil || !strings.Contains(err.Error(), test.code+": ") {
				t.Fatalf("ValidateCommit did not carry %s in its normal refusal: %v", test.code, err)
			}
		})
	}
}

func TestValidateChannelTreeSecretAndClosedNullEdges(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			e, _ := fakeGoalEndpoint(t)
			fixture := validChannelFixture()
			test.mutate(&fixture)
			tip := commitChannelFilesForEndpoint(t, e, fixture.files(t))
			problems := validateChannelTreeFor(e, tip)
			if test.code == "" {
				if len(problems) != 0 {
					t.Fatalf("lawful edge was refused: %v", problems)
				}
				if err := validateCommitFor(e, tip); err != nil {
					t.Fatalf("ValidateCommit refused a lawful edge: %v", err)
				}
			} else {
				expectChannelProblem(t, problems, test.code)
				if err := validateCommitFor(e, tip); err == nil || !strings.Contains(err.Error(), test.code+": ") {
					t.Fatalf("ValidateCommit did not carry %s in its normal refusal: %v", test.code, err)
				}
			}
		})
	}
}

func TestValidateChannelTreeJSONEdges(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			e, _ := fakeGoalEndpoint(t)
			fixture := validChannelFixture()
			test.mutate(&fixture, t)
			tip := commitChannelFilesForEndpoint(t, e, fixture.files(t))
			expectChannelProblem(t, validateChannelTreeFor(e, tip), "channel-json")
			if err := validateCommitFor(e, tip); err == nil || !strings.Contains(err.Error(), "channel-json: ") {
				t.Fatalf("ValidateCommit did not carry channel-json in its normal refusal: %v", err)
			}
		})
	}
}

func TestChannelTimeRequiresCanonicalSecondPrecision(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			err := channelTime(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("channelTime(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
		})
	}
}

func TestValidateChannelTreeAbsentIsSilent(t *testing.T) {
	t.Parallel()
	e, _ := fakeGoalEndpoint(t)
	tip := commitChannelFilesForEndpoint(t, e, vTree(vRoot(), []*GoalFile{vGoal(channelTestGoalID, StateQueued)}, nil))
	if problems := validateChannelTreeFor(e, tip); problems != nil {
		t.Fatalf("absent channel directory must return nil, got %v", problems)
	}
	if err := validateCommitFor(e, tip); err != nil {
		t.Fatalf("absent channel directory must not change ValidateCommit: %v", err)
	}
}
