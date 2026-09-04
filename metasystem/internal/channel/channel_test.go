package channel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

type testProvider struct {
	inbound     []Inbound
	cursor      Cursor
	posts       []string
	postThreads []*MessageRef
	threads     []MessageRef
	after       Cursor
	receives    int
	confirms    int
	beforePost  func()
	failPosts   int
}

func (p *testProvider) Post(_ context.Context, _ DestinationConfig, text string, thread *MessageRef) (MessageRef, error) {
	if p.beforePost != nil {
		p.beforePost()
	}
	if p.failPosts > 0 {
		p.failPosts--
		return MessageRef{}, errors.New("post failed")
	}
	p.posts = append(p.posts, text)
	p.postThreads = append(p.postThreads, thread)
	id := fmt.Sprintf("%d", len(p.posts))
	root := id
	if thread != nil {
		root = thread.ThreadID
		if root == "" {
			root = thread.ID
		}
	}
	return MessageRef{ID: id, ThreadID: root}, nil
}
func (p *testProvider) Receive(_ context.Context, _ DestinationConfig, threads []MessageRef, after Cursor) ([]Inbound, Cursor, error) {
	p.receives++
	p.threads = append([]MessageRef(nil), threads...)
	p.after = after
	return p.inbound, p.cursor, nil
}
func (p *testProvider) Confirm(context.Context, DestinationConfig, Cursor) error {
	p.confirms++
	return nil
}
func (p *testProvider) Credential(context.Context, DestinationConfig) (CredentialIdentity, error) {
	return CredentialIdentity{UserID: "UBOT"}, nil
}

func TestTOTPVerifiesRFC6238Vectors(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code, err := TOTPCode(secret, time.Unix(59, 0))
	if err != nil || code != "287082" {
		t.Fatalf("RFC 6238 SHA-1 vector suffix: %q %v", code, err)
	}
}
func TestTOTPWindowAndReplay(t *testing.T) {
	now := time.Unix(1234567890, 0)
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now.Add(-30*time.Second))
	step, ok := VerifyTOTP("JBSWY3DPEHPK3PXP", code, now)
	if !ok {
		t.Fatal("previous step rejected")
	}
	root := t.TempDir()
	row := consumedRow{Step: step, Destination: "fleet", Provider: "slack", ThreadID: "1", Ref: MessageRef{ID: "2"}, QID: "q"}
	if fresh, err := consume(root, row, now); err != nil || !fresh {
		t.Fatal(err)
	}
	other := row
	other.Ref.ID = "3"
	if fresh, _ := consume(root, other, now); fresh {
		t.Fatal("replay accepted")
	}
}

func TestDelayedTOTPReplayRemainsConsumed(t *testing.T) {
	now := time.Unix(1234567890, 0)
	delayed := consumedRow{Step: now.Add(-100*time.Second).Unix() / TOTPStep, Destination: "fleet", Provider: "slack", ThreadID: "1", Ref: MessageRef{ID: "2"}, QID: "q"}
	root := t.TempDir()
	if fresh, err := consume(root, delayed, now); err != nil || !fresh {
		t.Fatal(err)
	}
	current := delayed
	current.Step = now.Unix() / TOTPStep
	current.Ref.ID = "3"
	if fresh, err := consume(root, current, now); err != nil || !fresh {
		t.Fatal(err)
	}
	replay := delayed
	replay.Ref.ID = "4"
	if fresh, err := consume(root, replay, now); err != nil || fresh {
		t.Fatalf("delayed replay accepted: fresh=%t err=%v", fresh, err)
	}
}

func TestTOTPResumeExceptionIsEnvelopeScoped(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(1234567890, 0)
	row := consumedRow{Step: now.Unix() / 30, Destination: "fleet", Provider: "slack", ThreadID: "1", Ref: MessageRef{ID: "2"}, QID: "q"}
	if ok, err := consume(root, row, now); !ok || err != nil {
		t.Fatal(err)
	}
	same := row
	if ok, _ := consume(root, same, now); !ok {
		t.Fatal("same envelope cannot resume")
	}
	same.Provider = "fake"
	if ok, _ := consume(root, same, now); ok {
		t.Fatal("another provider reused the exception")
	}
}
func TestPollAtomicallyConsumesTOTP(t *testing.T) {
	root, p, _, now := pollLedgerBed(t)
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "first " + code}}
	if _, err := Poll(context.Background(), pollBedConfig(root, p, now)); err != nil {
		t.Fatal(err)
	}
	q2 := Question{ID: "01J5X0000000000000000000Q2", Goal: "g", Kind: "other", Machine: "machine", OpenedAt: now, Facts: []string{"second"}, Thread: &MessageRef{ID: "3", ThreadID: "3"}, State: "open"}
	if err := writeJSON(questionPath(root, q2.ID), q2); err != nil {
		t.Fatal(err)
	}
	p.inbound = []Inbound{{Ref: MessageRef{ID: "4", ThreadID: "3"}, ThreadID: "3", UserID: "UWIDO", Text: "second " + code}}
	if _, err := Poll(context.Background(), pollBedConfig(root, p, now)); err != nil {
		t.Fatal(err)
	}
	g := projectGoal(t, root, "g")
	answers := 0
	for _, h := range g.History {
		if h.Verb == "answer" {
			answers++
		}
	}
	got, _ := ReadQuestion(root, q2.ID)
	b, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "channel", "totp-consumed.json"))
	if err != nil || answers != 1 || len(got.Rejected) != 1 || got.Rejected[0].Reason != "replayed code" || strings.Count(string(b), `"step"`) != 1 {
		t.Fatalf("answers=%d rejected=%+v consumption=%s err=%v", answers, got.Rejected, b, err)
	}
}
func TestSecretsScrubbedFromErrors(t *testing.T) {
	got := Scrub("token hush leaked", "token", "hush")
	if strings.Contains(got, "token") || strings.Contains(got, "hush") {
		t.Fatal(got)
	}
}
func TestCredentialIsTokenIdentity(t *testing.T) {
	p := &testProvider{}
	id, err := p.Credential(context.Background(), DestinationConfig{})
	if err != nil || id.UserID != "UBOT" {
		t.Fatal(id, err)
	}
}

func TestAskWritesRecordBeforePosting(t *testing.T) {
	root := t.TempDir()
	p := &testProvider{}
	p.beforePost = func() {
		matches, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "channel", "questions", "*.json"))
		if len(matches) != 1 {
			t.Fatalf("post preceded durable record: %v", matches)
		}
	}
	_, err := Ask(AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Machine: "m", Facts: []string{"fact"}, Provider: p, Now: time.Unix(1, 0)})
	if err != nil {
		t.Fatal(err)
	}
}
func TestAskDedupsOpenQuestion(t *testing.T) {
	root := t.TempDir()
	p := &testProvider{}
	r := AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Machine: "m", Facts: []string{"fact"}, Provider: p, Now: time.Unix(1, 0)}
	a, err := Ask(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Ask(r)
	if err != nil || a.ID != b.ID || len(p.posts) != 1 {
		t.Fatalf("dedup failed: %s %s %d %v", a.ID, b.ID, len(p.posts), err)
	}
}
func TestReportOmitsEmptyParts(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "empty fleet", text: mustComposeReport(t, ReportConfig{RepoRoot: t.TempDir(), Machine: "m", Now: now}), want: "m status 2026-09-04 12:00Z"},
		{name: "needs you only", text: func() string {
			root := t.TempDir()
			if err := writeJSON(questionPath(root, "question"), Question{ID: "question", Goal: "choose-colour", State: "open", Facts: []string{"Choose the launch colour"}}); err != nil {
				t.Fatal(err)
			}
			return mustComposeReport(t, ReportConfig{RepoRoot: root, Machine: "m", Now: now})
		}(), want: "m status 2026-09-04 12:00Z\nNeeds you: choose colour — Choose the launch colour."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.text != test.want {
				t.Fatalf("got:\n%s\nwant:\n%s", test.text, test.want)
			}
			for _, absent := range []string{"Delivered:", "Next up:", "Spend today:"} {
				if strings.Contains(test.text, absent) {
					t.Fatalf("empty part %q was rendered:\n%s", absent, test.text)
				}
			}
		})
	}
}

func mustComposeReport(t *testing.T, config ReportConfig) string {
	t.Helper()
	text, err := ComposeReport(config)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestStatusCadenceAndDigestGate(t *testing.T) {
	now := time.Unix(10000, 0)
	s := StatusState{LastPost: now.Add(-5 * time.Hour), ContentDigest: Digest("same")}
	if ShouldPost(s, now, 4*time.Hour, "same", false) {
		t.Fatal("unchanged digest posted")
	}
	if !ShouldPost(s, now, 4*time.Hour, "new", false) {
		t.Fatal("changed due digest skipped")
	}
	old := "m status 2026-09-04 08:00Z\nNext up: first"
	s = StatusState{LastPost: now.Add(-5 * time.Hour), ContentDigest: Digest(old)}
	if ShouldPost(s, now, 4*time.Hour, "m status 2026-09-04 13:00Z\nNext up: first", false) {
		t.Fatal("a changed header timestamp defeated the content digest")
	}
	old = "m status 2026-09-04 08:00Z"
	s = StatusState{LastPost: now.Add(-5 * time.Hour), ContentDigest: Digest(old)}
	if ShouldPost(s, now, 4*time.Hour, "m status 2026-09-04 13:00Z", false) {
		t.Fatal("an empty fleet posted again because only its header time changed")
	}
}

func TestReportShowsOneQuestionTwoLandingsAndOnlyTwoNextItems(t *testing.T) {
	now := time.Now().UTC().Add(time.Minute)
	root := reportLedger(t,
		reportGoal("delivery-one", "Deliver one.", goal.StateApproved, "other-machine", now),
		reportGoal("delivery-two", "Deliver two.", goal.StateApproved, "other-machine", now),
		reportGoal("alpha-next", "Do alpha.", goal.StateApproved, "fleet-one", now),
		reportGoal("beta-next", "Do beta.", goal.StateApproved, "", now),
		reportGoal("gamma-next", "Do gamma.", goal.StateApproved, "fleet-one", now),
	)
	if err := writeJSON(questionPath(root, "question"), Question{ID: "question", Goal: "launch-choice", State: "open", Facts: []string{"Choose the launch colour"}}); err != nil {
		t.Fatal(err)
	}
	reportGit(t, root, "commit", "-q", "--allow-empty", "-m", "Ship delivery one\n\nGoal-Item: delivery-one")
	reportGit(t, root, "commit", "-q", "--allow-empty", "-m", "Ship delivery two\n\nGoal-Item: delivery-two")
	reportGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	text := mustComposeReport(t, ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: now, WindowStart: now.Add(-4 * time.Hour)})
	want := strings.Join([]string{
		"fleet-one status " + now.Format("2006-01-02 15:04") + "Z",
		"Needs you: launch choice — Choose the launch colour.",
		"Delivered: delivery one — Ship delivery one",
		"Delivered: delivery two — Ship delivery two",
		"Next up: alpha next",
		"Next up: beta next",
	}, "\n")
	if text != want || strings.Contains(text, "gamma next") {
		t.Fatalf("report mismatch\n--- got ---\n%s\n--- want ---\n%s", text, want)
	}
}

func TestReportShowsGoalApprovalGapOnceBesideItsOpenQuestion(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	waiting := reportGoal("waiting-feature", "Wait for approval.", goal.StateQueued, "", now)
	waiting.Budget = &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 30, ActiveJobLimit: 1}
	waiting.Pinned = "fleet-one"
	waiting.Labels = []string{"next"}
	root := reportLedger(t, waiting)
	withoutQuestion := mustComposeReport(t, ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: now})
	if !strings.Contains(withoutQuestion, "Needs you: waiting feature — Reply in this thread with this token verbatim, followed by your code: start waiting-feature") {
		t.Fatalf("approval gap was omitted:\n%s", withoutQuestion)
	}
	questionBudget, _ := goal.NewBudget("2h", 5, 600, 1, 0)
	if err := writeJSON(questionPath(root, "budget-question"), Question{ID: "budget-question", Goal: waiting.Id, Kind: "budget-above-norm", State: "open", Facts: []string{"The current allowance is exhausted"}, Budget: &questionBudget}); err != nil {
		t.Fatal(err)
	}
	withQuestion := mustComposeReport(t, ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: now})
	if strings.Count(withQuestion, "Needs you: waiting feature") != 1 || !strings.Contains(withQuestion, "approve the requested budget raise.") {
		t.Fatalf("the open budget question did not replace the generic approval gap:\n%s", withQuestion)
	}
}

func TestBudgetQuestionRequiresPersistsAndRendersCompleteTuple(t *testing.T) {
	root := t.TempDir()
	request := AskRequest{RepoRoot: root, Goal: "g", Kind: "budget-above-norm", Machine: "machine", Facts: []string{"The current allowance is exhausted"}, Wants: "yes", Now: time.Unix(1, 0)}
	if _, err := Ask(request); err == nil || !strings.Contains(err.Error(), "requires a complete proposed budget tuple") {
		t.Fatalf("budget question without tuple was accepted: %v", err)
	}
	budget, err := goal.NewBudget("2h", 5, 600, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	request.Budget = &budget
	q, err := Ask(request)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := ReadQuestion(root, q.ID)
	if err != nil || stored.Budget == nil || *stored.Budget != budget {
		t.Fatalf("stored budget=%+v err=%v", stored.Budget, err)
	}
	want := "Proposed box: 2h, 5 attempts, 600 reserved minutes, 1 active job, 0 review rounds"
	if rendered := renderQuestion(stored); !strings.Contains(rendered, want) {
		t.Fatalf("rendered question %q does not contain %q", rendered, want)
	}
	other := request
	other.Kind = "other"
	if _, err := Ask(other); err == nil || !strings.Contains(err.Error(), "cannot carry a proposed budget tuple") {
		t.Fatalf("non-budget question stored a tuple: %v", err)
	}
}

func TestLegacyBudgetQuestionLoadsWithoutTuple(t *testing.T) {
	root := t.TempDir()
	legacyID := "01J5X0000000000000000000L1"
	questionsDir := filepath.Dir(questionPath(root, legacyID))
	if err := os.MkdirAll(questionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "id": "01J5X0000000000000000000L1",
  "goal": "g",
  "kind": "budget-above-norm",
  "machine": "machine",
  "state": "closed"
}`
	if err := os.WriteFile(questionPath(root, legacyID), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	open := Question{ID: "01J5X0000000000000000000L2", Goal: "g", Kind: "other", Machine: "machine", State: "open"}
	if err := writeJSON(questionPath(root, open.ID), open); err != nil {
		t.Fatal(err)
	}

	loaded, err := ReadQuestion(root, legacyID)
	if err != nil || loaded.Budget != nil {
		t.Fatalf("legacy budget question budget=%+v err=%v", loaded.Budget, err)
	}
	questions, err := listQuestions(root)
	if err != nil {
		t.Fatal(err)
	}
	foundOpen := false
	for _, question := range questions {
		if question.ID == open.ID && question.State == "open" {
			foundOpen = true
		}
	}
	if !foundOpen {
		t.Fatalf("open question %q missing from %+v", open.ID, questions)
	}
}

func TestReportOmitsApprovalForElevenUnmarkedQueuedGoals(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	goals := make([]*goal.GoalFile, 0, 11)
	for i := 1; i <= 11; i++ {
		f := reportGoal(fmt.Sprintf("feature-%02d", i), "Build the feature.", goal.StateQueued, "", now)
		f.Budget = &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 30, ActiveJobLimit: 1}
		goals = append(goals, f)
	}
	root := reportLedger(t, goals...)
	text := mustComposeReport(t, ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: now})
	want := "fleet-one status 2026-09-04 12:00Z"
	if text != want || strings.Contains(text, "Needs you:") {
		t.Fatalf("unmarked queued goals produced an approval decision\n--- got ---\n%s\n--- want ---\n%s", text, want)
	}
}

func TestReportNamesTheMarkedPinnedGoalAndReturnsItsBinding(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	marked := reportGoal("real-next-pick", "Build the selected feature.", goal.StateQueued, "fleet-one", now)
	marked.Labels = []string{"next"}
	root := reportLedger(t, marked)
	text, goalID, err := ComposeStatusReport(ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	want := "Needs you: real next pick — Reply in this thread with this token verbatim, followed by your code: start real-next-pick"
	if goalID != marked.Id || !strings.Contains(text, want) || strings.Count(text, "Needs you:") != 1 {
		t.Fatalf("goal=%q report=%q", goalID, text)
	}
}

func TestReportDoesNotRequestApprovalWhenFirstBudgetedGoalIsApproved(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	approved := reportGoal("alpha-approved", "Start approved work.", goal.StateApproved, "", now)
	waiting := reportGoal("beta-waiting", "Wait for approval.", goal.StateQueued, "", now)
	waiting.Budget = &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 30, ActiveJobLimit: 1}
	root := reportLedger(t, approved, waiting)
	text := mustComposeReport(t, ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: now})
	if strings.Contains(text, "Needs you:") || !strings.Contains(text, "Next up: alpha approved") {
		t.Fatalf("an approved first goal should need no approval:\n%s", text)
	}
}

func TestReportCapsAllOutputAtTwelveLines(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("question-%02d", i)
		if err := writeJSON(questionPath(root, id), Question{ID: id, Goal: id, State: "open", Facts: []string{"Make a decision"}}); err != nil {
			t.Fatal(err)
		}
	}
	text := mustComposeReport(t, ReportConfig{RepoRoot: root, Machine: "m", Now: time.Now(), Undelivered: 2})
	lines := strings.Split(text, "\n")
	if len(lines) != 12 || !strings.HasPrefix(lines[len(lines)-1], "Undelivered: 2 channel messages") {
		t.Fatalf("line count=%d, last=%q\n%s", len(lines), lines[len(lines)-1], text)
	}
}

func TestReportCapTrimsNextUpBeforeNeedsYou(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	goals := []*goal.GoalFile{
		reportGoal("next-alpha", "Do alpha next.", goal.StateApproved, "fleet-one", now),
		reportGoal("next-beta", "Do beta next.", goal.StateApproved, "fleet-one", now),
	}
	for i := 0; i < 11; i++ {
		goals = append(goals, reportGoal(fmt.Sprintf("delivered-%02d", i), "Deliver work.", goal.StateApproved, "other-machine", now))
	}
	root := reportLedger(t, goals...)
	if err := writeJSON(questionPath(root, "decision"), Question{ID: "decision", Goal: "launch-choice", State: "open", Facts: []string{"Choose the launch colour"}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 11; i++ {
		reportGit(t, root, "commit", "-q", "--allow-empty", "-m", fmt.Sprintf("Ship delivered %02d\n\nGoal-Item: delivered-%02d", i, i))
	}
	reportGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	text := mustComposeReport(t, ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: now, WindowStart: now.Add(-4 * time.Hour)})
	lines := strings.Split(text, "\n")
	if len(lines) != 12 || !strings.Contains(text, "Needs you: launch choice — Choose the launch colour.") || strings.Contains(text, "Next up:") {
		t.Fatalf("the cap did not preserve the decision while trimming Next up first:\n%s", text)
	}
}

func TestLandingIsReportedInExactlyOnePostWindow(t *testing.T) {
	landedAt := time.Now().UTC()
	root := reportLedger(t, reportGoal("one-landing", "Land once.", goal.StateApproved, "other-machine", landedAt))
	firstPost := landedAt.Add(time.Minute)
	if err := SaveStatusState(root, StatusState{LastPost: landedAt.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	reportGit(t, root, "commit", "-q", "--allow-empty", "-m", "Ship it once\n\nGoal-Item: one-landing")
	reportGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	first := mustComposeReport(t, ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: firstPost})
	if strings.Count(first, "Delivered: one landing — Ship it once") != 1 {
		t.Fatalf("first post did not report the landing once:\n%s", first)
	}
	if err := SaveStatusState(root, StatusState{LastPost: firstPost, ContentDigest: Digest(first)}); err != nil {
		t.Fatal(err)
	}
	second := mustComposeReport(t, ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: firstPost.Add(time.Hour)})
	if strings.Contains(second, "Delivered:") {
		t.Fatalf("second post repeated the landing:\n%s", second)
	}
}

func reportGoal(id, intent, state, pin string, now time.Time) *goal.GoalFile {
	openedAt := now.Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	opened := goal.HistoryLine{At: openedAt, Opid: goal.Opid("01J5X0000000000000000000R0", "machine", id), Verb: "open", Actor: "machine+" + id, Targets: []string{id}, Keep: -1}
	f := &goal.GoalFile{Id: id, State: state, Tier: 1, Intent: intent, Origin: "main", NextStep: "Work it.", OpenedAt: openedAt, Revision: 1, Pinned: pin, History: []goal.HistoryLine{opened}}
	if state == goal.StateApproved {
		budget := &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 1, ReservedJobMinutesLimit: 30, ActiveJobLimit: 1}
		approvedAt := now.Add(-time.Hour).UTC().Format(time.RFC3339)
		opid := goal.Opid("01J5X0000000000000000000R1", "human", id)
		f.Budget = budget
		f.Revision = 2
		f.History = append(f.History, goal.HistoryLine{At: approvedAt, Opid: opid, Verb: "approve", Actor: "human:wido", Targets: []string{id}, Keep: -1})
		f.Approved = &goal.ApprovalRecord{By: "human:wido", At: approvedAt, Revision: 2, Opid: opid, Authority: goal.ApprovalAuthorityProven, Digest: goal.ApprovalDigest(intent, 1, *budget)}
	}
	return f
}

func reportLedger(t *testing.T, goals ...*goal.GoalFile) string {
	t.Helper()
	root := t.TempDir()
	reportGit(t, root, "init", "-q", "-b", "main")
	reportGit(t, root, "config", "user.name", "channel-test")
	reportGit(t, root, "config", "user.email", "channel@example.invalid")
	reportGit(t, root, "config", "goal.sync-remote", "local")
	if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	backlog := goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1})
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "backlog.md"), backlog, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range goals {
		if err := os.WriteFile(filepath.Join(root, "plans", "goals", f.Id+".md"), goal.RenderFile(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reportGit(t, root, "add", "plans/goals")
	reportGit(t, root, "commit", "-q", "-m", "seed report ledger")
	reportGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	reportGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	reportGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	return root
}

func reportGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = testGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestPollRejectsWrongUserNoCodeBadCodeReplay(t *testing.T) {
	root := t.TempDir()
	p := &testProvider{}
	q := Question{ID: "q", Goal: "g", State: "open", Thread: &MessageRef{ID: "1", ThreadID: "1"}}
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "stranger", Text: "answer 000000"}, {Ref: MessageRef{ID: "3", ThreadID: "1"}, ThreadID: "1", UserID: "human", Text: "answer"}, {Ref: MessageRef{ID: "4", ThreadID: "1"}, ThreadID: "1", UserID: "human", Text: "answer 000000"}, {Ref: MessageRef{ID: "5", ThreadID: "1"}, ThreadID: "1", UserID: "stranger", Text: "answer 000000"}}
	_, err := Poll(context.Background(), PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: "fake", HumanUserID: "human", TOTPSecret: "JBSWY3DPEHPK3PXP", Machine: "m", Lineage: "l", Provider: p, Now: time.Unix(1234567890, 0)})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := ReadQuestion(root, "q")
	if len(got.Rejected) != 4 || len(p.posts) != 3 {
		t.Fatalf("rejections=%d posts=%d", len(got.Rejected), len(p.posts))
	}
	for _, tc := range []struct{ name, user, secret string }{{"empty-secret", "human", ""}, {"empty-user", "", "JBSWY3DPEHPK3PXP"}} {
		t.Run(tc.name, func(t *testing.T) {
			root, p, q, now := pollLedgerBed(t)
			code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
			p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "human", Text: "answer " + code}}
			cfg := pollBedConfig(root, p, now)
			cfg.HumanUserID, cfg.TOTPSecret = tc.user, tc.secret
			if _, err := Poll(context.Background(), cfg); err != nil {
				t.Fatal(err)
			}
			got, _ := ReadQuestion(root, q.ID)
			g := projectGoal(t, root, "g")
			if len(got.Rejected) != 1 || got.Rejected[0].Reason != "unconfigured" || len(p.posts) != 1 {
				t.Fatalf("question=%+v posts=%v", got, p.posts)
			}
			for _, h := range g.History {
				if h.Verb == "answer" {
					t.Fatal("unconfigured channel recorded an answer")
				}
			}
			if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "channel", "totp-consumed.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unconfigured channel wrote consumption: %v", err)
			}
		})
	}
}

func TestPollVerifiesCodeAtProviderSendTime(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	tests := []struct {
		name        string
		sentBefore  time.Duration
		includeTime bool
		wantAnswer  bool
		wantReason  string
	}{
		{name: "sent one hundred seconds before poll", sentBefore: 100 * time.Second, includeTime: true, wantAnswer: true},
		{name: "sent four hundred seconds before poll", sentBefore: 400 * time.Second, includeTime: true, wantReason: "code too old: sent 400s before the poll"},
		{name: "provider has no send time", wantAnswer: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, p, q, now := pollLedgerBed(t)
			sentAt := now.Add(-tc.sentBefore)
			codeAt := sentAt
			if !tc.includeTime {
				sentAt = time.Time{}
				codeAt = now
			}
			code, err := TOTPCode(secret, codeAt)
			if err != nil {
				t.Fatal(err)
			}
			p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved " + code, SentAt: sentAt}}
			if _, err = Poll(context.Background(), pollBedConfig(root, p, now)); err != nil {
				t.Fatal(err)
			}
			got, err := ReadQuestion(root, q.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantAnswer != (got.Answer != nil) {
				t.Fatalf("answer=%+v rejections=%+v", got.Answer, got.Rejected)
			}
			if tc.wantReason != "" && (len(got.Rejected) != 1 || got.Rejected[0].Reason != tc.wantReason) {
				t.Fatalf("rejections=%+v", got.Rejected)
			}
		})
	}
}

func TestInboundCheckpointSurvivesCrashAndDeduplicates(t *testing.T) {
	root, p, _, now := pollLedgerBed(t)
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "u"}, ThreadID: "stray"}, {Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved " + code}}
	cfg := pollBedConfig(root, p, now)
	cfg.FailurePoint = func(point string) error {
		if point == "before-cursor" {
			return errors.New("crash")
		}
		return nil
	}
	if _, err := Poll(context.Background(), cfg); err == nil {
		t.Fatal("crash before cursor write did not fire")
	}
	cfg.FailurePoint = nil
	if _, err := Poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "channel", "fleet", "cursor.json"))
	if err != nil || !strings.Contains(string(b), "done") {
		t.Fatal(string(b), err)
	}
	unmatched, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "channel", "fleet", "unmatched.jsonl"))
	g := projectGoal(t, root, "g")
	answers := 0
	for _, h := range g.History {
		if h.Verb == "answer" {
			answers++
		}
	}
	if err != nil || strings.Count(strings.TrimSpace(string(unmatched)), "\n") != 0 || answers != 1 {
		t.Fatalf("unmatched=%q answers=%d err=%v", unmatched, answers, err)
	}
}

func TestStatusThreadTokenWithValidCodeApprovesMarkedGoal(t *testing.T) {
	root, p, _, now := pollLedgerBed(t)
	statusRef := MessageRef{ID: "10", ThreadID: "10"}
	if err := SaveStatusState(root, StatusState{LastPost: now, Ref: statusRef, GoalID: "g"}); err != nil {
		t.Fatal(err)
	}
	sentAt := now.Add(-100 * time.Second)
	code, err := TOTPCode("JBSWY3DPEHPK3PXP", sentAt)
	if err != nil {
		t.Fatal(err)
	}
	p.inbound = []Inbound{{Ref: MessageRef{ID: "11", ThreadID: "10"}, ThreadID: "10", UserID: "UWIDO", Text: "start g " + code, SentAt: sentAt}}
	if _, err = Poll(context.Background(), pollBedConfig(root, p, now)); err != nil {
		t.Fatal(err)
	}
	approved := projectGoal(t, root, "g")
	last := approved.History[len(approved.History)-1]
	if approved.State != goal.StateApproved || approved.Approved == nil || last.Verb != "approve" ||
		last.AuthorityOutcome != goal.AuthorityOutcomeVerifiedChannelAnswer || last.ChannelUser != "UWIDO" ||
		last.ChannelRef != "10/11" || last.ChannelContext != "10" || last.ChannelStep != sentAt.Unix()/TOTPStep {
		t.Fatalf("goal=%+v history=%+v posts=%v", approved, last, p.posts)
	}
	if len(p.posts) != 1 || p.posts[0] != "recorded: g approved for execution" || p.postThreads[0] == nil || p.postThreads[0].ThreadID != "10" {
		t.Fatalf("posts=%v threads=%v", p.posts, p.postThreads)
	}
}

func TestStatusThreadReplyWithoutTokenIsAnsweredAndFiled(t *testing.T) {
	root, p, _, now := pollLedgerBed(t)
	if err := SaveStatusState(root, StatusState{LastPost: now, Ref: MessageRef{ID: "10", ThreadID: "10"}, GoalID: "g"}); err != nil {
		t.Fatal(err)
	}
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	in := Inbound{Ref: MessageRef{ID: "11", ThreadID: "10"}, ThreadID: "10", UserID: "UWIDO", Text: "approved " + code, SentAt: now}
	p.inbound = []Inbound{in}
	if _, err := Poll(context.Background(), pollBedConfig(root, p, now)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "channel", "fleet", "unmatched.jsonl"))
	if err != nil || !strings.Contains(string(b), `"id":"11"`) {
		t.Fatalf("unmatched=%s err=%v", b, err)
	}
	if len(p.posts) != 1 || p.posts[0] != "not recorded: wrong token; reply with the token and your code" || p.postThreads[0] == nil || p.postThreads[0].ThreadID != "10" {
		t.Fatalf("posts=%v threads=%v", p.posts, p.postThreads)
	}
	if got := projectGoal(t, root, "g"); got.State != goal.StateQueued {
		t.Fatalf("reply without token changed goal state: %s", got.State)
	}
}

func TestTickChannelPassBound(t *testing.T) {
	root := t.TempDir()
	p := &testProvider{cursor: "later"}
	for i := 0; i < 6; i++ {
		p.inbound = append(p.inbound, Inbound{Ref: MessageRef{ID: fmt.Sprint(i)}, ThreadID: "stray"})
	}
	r, err := Poll(context.Background(), PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: "fake", Provider: p, Now: time.Unix(1, 0)})
	if err != nil || r.Dispositions != 5 || p.receives != 1 {
		t.Fatal(r, p.receives, err)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "channel", "fleet", "cursor.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial pass advanced cursor")
	}
}

func TestPollPassesEveryPostedRefWithItsRoot(t *testing.T) {
	root := t.TempDir()
	q := Question{
		ID: "q", Goal: "g", State: "open", Thread: &MessageRef{ID: "10", ThreadID: "10"},
		Rejected: []Rejection{{Ref: MessageRef{ID: "11", ThreadID: "10"}, PostRef: &MessageRef{ID: "12", ThreadID: "wrong"}}},
	}
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	p := &testProvider{}
	if _, err := Poll(context.Background(), PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: "fake", Provider: p}); err != nil {
		t.Fatal(err)
	}
	want := []MessageRef{{ID: "10", ThreadID: "10"}, {ID: "12", ThreadID: "10"}}
	if fmt.Sprint(p.threads) != fmt.Sprint(want) {
		t.Fatalf("threads=%+v want=%+v", p.threads, want)
	}
}

func TestRejectionRecordsItsPostRef(t *testing.T) {
	root := t.TempDir()
	q := Question{ID: "q", Goal: "g", State: "open", Thread: &MessageRef{ID: "10", ThreadID: "10"}}
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	p := &testProvider{inbound: []Inbound{{Ref: MessageRef{ID: "11", ThreadID: "10"}, ThreadID: "10", UserID: "stranger"}}}
	if _, err := Poll(context.Background(), PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: "fake", HumanUserID: "human", TOTPSecret: "JBSWY3DPEHPK3PXP", Provider: p}); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadQuestion(root, q.ID)
	if len(got.Rejected) != 1 || got.Rejected[0].PostRef == nil || got.Rejected[0].PostRef.ThreadID != "10" {
		t.Fatalf("rejection=%+v", got.Rejected)
	}
}

func rejectionCrashCase(t *testing.T, point string, wantPosts int) {
	t.Helper()
	root := t.TempDir()
	q := Question{ID: "q", Goal: "g", State: "open", Thread: &MessageRef{ID: "10", ThreadID: "10"}}
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	p := &testProvider{inbound: []Inbound{{Ref: MessageRef{ID: "11", ThreadID: "10"}, ThreadID: "10", UserID: "stranger"}}}
	fired := false
	cfg := PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: "fake", HumanUserID: "human", TOTPSecret: "JBSWY3DPEHPK3PXP", Provider: p, FailurePoint: func(got string) error {
		if got == point && !fired {
			fired = true
			return errors.New("injected crash")
		}
		return nil
	}}
	if _, err := Poll(context.Background(), cfg); err == nil {
		t.Fatal("crash did not fire")
	}
	cfg.FailurePoint = nil
	if _, err := Poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadQuestion(root, q.ID)
	if len(got.Rejected) != 1 || len(p.posts) != wantPosts {
		t.Fatalf("rejections=%+v posts=%v", got.Rejected, p.posts)
	}
}

func TestRejectionReceiptPostRecordCrashIsDispositionSafe(t *testing.T) {
	t.Run("after record", func(t *testing.T) { rejectionCrashCase(t, "rejection-recorded", 0) })
	t.Run("after post", func(t *testing.T) { rejectionCrashCase(t, "rejection-posted", 1) })
}

func TestEveryInvalidInboundRefIsAnsweredAtMostOnceAcrossRecordBeforePostCrash(t *testing.T) {
	for _, tc := range []struct {
		point string
		posts int
	}{{"rejection-recorded", 0}, {"rejection-posted", 1}} {
		t.Run(tc.point, func(t *testing.T) { rejectionCrashCase(t, tc.point, tc.posts) })
	}
}

func TestTelegramCrashAfterMatchedDoesNotRedisposeReplyAsUnmatched(t *testing.T) {
	root, p, _, now := pollLedgerBed(t)
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "22", ThreadID: "20"}, ThreadID: "1", UserID: "UWIDO", Text: "yes " + code}}
	cfg := pollBedConfig(root, p, now)
	cfg.FailurePoint = func(point string) error {
		if point == "matched" {
			return errors.New("injected crash")
		}
		return nil
	}
	if _, err := Poll(context.Background(), cfg); err == nil {
		t.Fatal("matched crash did not fire")
	}
	cfg.FailurePoint = nil
	if _, err := Poll(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", "channel", "fleet", "unmatched.jsonl")
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) != 0 {
		t.Fatalf("matched reply became unmatched: %s", b)
	}
}

func TestCursorFromAnotherProviderIsIgnored(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "artifacts", "agents", "channel", "fleet", "cursor.json")
	if err := writeJSON(path, cursorRecord{Provider: "slack", Cursor: "slack-cursor"}); err != nil {
		t.Fatal(err)
	}
	p := &testProvider{cursor: "telegram-cursor"}
	if _, err := Poll(context.Background(), PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: "telegram", Provider: p}); err != nil {
		t.Fatal(err)
	}
	if p.after != "" {
		t.Fatalf("foreign cursor passed to provider: %q", p.after)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), `"provider": "telegram"`) {
		t.Fatal(string(b), err)
	}
}

func TestPollKeepsPassingSavedCursorWithoutConfirming(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "artifacts", "agents", "channel", "fleet", "cursor.json")
	if err := writeJSON(path, cursorRecord{Provider: "telegram", Cursor: "saved-cursor"}); err != nil {
		t.Fatal(err)
	}
	p := &testProvider{cursor: "next-cursor"}
	if _, err := Poll(context.Background(), PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: "telegram", Provider: p}); err != nil {
		t.Fatal(err)
	}
	if p.after != "saved-cursor" || p.confirms != 0 {
		t.Fatalf("Poll passed cursor %q and made %d confirmations", p.after, p.confirms)
	}
}

func TestAuthenticatedChannelHistoryRoundTrip(t *testing.T) {
	h := goal.HistoryLine{At: "2026-09-03T00:00:00Z", Opid: goal.Opid("01J5X0000000000000000000F8", "machine", "lineage"), Verb: "answer", Actor: "human:wido", Targets: []string{"g"}, Keep: -1, AuthorityOutcome: goal.AuthorityOutcomeAuthenticatedChannelWord, ChannelProvider: "slack", ChannelUser: "UWIDO", ChannelRef: "1/2", ChannelStep: 42, Reason: "yes"}
	line := goal.RenderHistoryLine(h)
	parsed, err := goal.ParseHistoryLine(line)
	if err != nil || goal.RenderHistoryLine(parsed) != line {
		t.Fatal(line, err)
	}
	want := "authorityOutcome=AUTHENTICATED_CHANNEL_WORD channelProvider=slack channelUser=UWIDO channelRef=1/2 channelStep=42"
	if !strings.Contains(line, want) {
		t.Fatal(line)
	}
}
func TestPollRecordsAuthenticatedReply(t *testing.T) {
	root, p, q, now := pollLedgerBed(t)
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved\nnow " + code}}
	r, err := Poll(context.Background(), pollBedConfig(root, p, now))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := ReadQuestion(root, q.ID)
	g := projectGoal(t, root, "g")
	answers := 0
	for _, h := range g.History {
		if h.Verb == "answer" {
			answers++
			if h.Actor != "human:wido" {
				t.Fatal(h)
			}
			if h.Reason != "approved now" {
				t.Fatalf("answer was not normalized to one line: %q", h.Reason)
			}
		}
	}
	if got.State != "closed" || got.Answer == nil || answers != 1 || strings.Count(g.NextStep, "ANSWERED "+q.ID) != 1 || r.Dispositions != 1 {
		t.Fatalf("question=%+v goal=%+v result=%+v", got, g, r)
	}
	t.Run("receipt post failure does not stop receiving", func(t *testing.T) {
		root, p, _, now := pollLedgerBed(t)
		code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
		p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved " + code}}
		cfg := pollBedConfig(root, p, now)
		cfg.FailurePoint = func(point string) error {
			if point == "recorded" {
				return errors.New("crash")
			}
			return nil
		}
		if _, err := Poll(context.Background(), cfg); err == nil {
			t.Fatal("recorded crash did not fire")
		}
		cfg.FailurePoint = nil
		p.failPosts = 1
		r, err := Poll(context.Background(), cfg)
		if err != nil || r.Received != 1 || p.receives != 2 {
			t.Fatalf("result=%+v receives=%d err=%v", r, p.receives, err)
		}
		if _, err := Poll(context.Background(), cfg); err != nil {
			t.Fatal(err)
		}
		got, _ := ReadQuestion(root, "01J5X0000000000000000000Q0")
		if got.State != "closed" {
			t.Fatalf("receipt was not retried: %+v", got)
		}
	})
}
func TestPollCrashRecoveryExactlyOnce(t *testing.T) {
	for _, phase := range []string{"matched", "recorded-commit", "recorded", "receipted", "closed"} {
		t.Run(phase, func(t *testing.T) {
			root, p, q, now := pollLedgerBed(t)
			code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
			p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved " + code}}
			cfg := pollBedConfig(root, p, now)
			fired := false
			cfg.FailurePoint = func(got string) error {
				if got == phase && !fired {
					fired = true
					return errors.New("crash")
				}
				return nil
			}
			if _, err := Poll(context.Background(), cfg); err == nil {
				t.Fatal("crash injection did not stop the pass")
			}
			cfg.FailurePoint = nil
			if _, err := Poll(context.Background(), cfg); err != nil {
				t.Fatal(err)
			}
			g := projectGoal(t, root, "g")
			answers := 0
			for _, h := range g.History {
				if h.Verb == "answer" {
					answers++
				}
			}
			record, _ := ReadQuestion(root, q.ID)
			if answers != 1 || strings.Count(g.NextStep, "ANSWERED "+q.ID) != 1 || record.State != "closed" {
				t.Fatalf("answers=%d next=%q record=%+v", answers, g.NextStep, record)
			}
			if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "channel", "fleet", "cursor.json")); err != nil {
				t.Fatal("cursor did not advance last", err)
			}
		})
	}
}
func TestAnswerCarryingStrictTokenSatisfiesNormApproval(t *testing.T) {
	root, p, q, now := pollLedgerBed(t)
	q.Wants = "goal=g minutes=60 reviewRounds=3 goalRevision=2"
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approved " + code}}
	if _, err := Poll(context.Background(), pollBedConfig(root, p, now)); err != nil {
		t.Fatal(err)
	}
	record, _ := ReadQuestion(root, q.ID)
	syncAccepted(t, root)
	ep, _ := goal.ResolveEndpoint(root)
	projection, err := goal.Project(ep, false, now)
	if err != nil {
		t.Fatal(err)
	}
	minutes, rounds, revision, exists, proven, err := goal.RecordedNormApproval(root, projection.Tree, record.Answer.Opid, "g")
	if err != nil || !exists || !proven || minutes != 60 || rounds != 3 || revision != 2 {
		t.Fatalf("approval=%d/%d/%d exists=%v proven=%v err=%v", minutes, rounds, revision, exists, proven, err)
	}
}

func TestVerifiedBudgetTokenRaisesBoxTwiceAndReopensAdmission(t *testing.T) {
	root, p, _, now := pollLedgerBed(t)
	firstBudget, _ := goal.NewBudget("2h", 5, 600, 1, 0)
	first := Question{ID: "01J5X0000000000000000000B1", Goal: "g", Kind: "budget-above-norm", Machine: "machine", OpenedAt: now, Facts: []string{"raise the box"}, Wants: "yes", Budget: &firstBudget, Thread: &MessageRef{ID: "10", ThreadID: "10"}, State: "open"}
	if err := writeJSON(questionPath(root, first.ID), first); err != nil {
		t.Fatal(err)
	}
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "11", ThreadID: "10"}, ThreadID: "10", UserID: "UWIDO", Text: "yes " + code, SentAt: now}}
	if _, err := Poll(context.Background(), pollBedConfig(root, p, now)); err != nil {
		t.Fatal(err)
	}
	afterFirst := projectGoal(t, root, "g")
	if afterFirst.Budget == nil || *afterFirst.Budget != firstBudget || afterFirst.Approved == nil || afterFirst.Approved.Authority != goal.ApprovalAuthorityChannel || afterFirst.NormApproval == nil || afterFirst.NormApproval.ApprovedRef != first.ID {
		t.Fatalf("first verified answer did not bind its box and norm proof: %+v", afterFirst)
	}
	if got := p.posts[len(p.posts)-1]; got != "recorded: g box raised to 2h, 5 attempts, 600 reserved minutes, 1 active job, 0 review rounds" {
		t.Fatalf("first receipt=%q", got)
	}

	secondNow := now.Add(30 * time.Second)
	secondBudget, _ := goal.NewBudget("3h", 6, 700, 1, 0)
	second := Question{ID: "01J5X0000000000000000000B2", Goal: "g", Kind: "budget-above-norm", Machine: "machine", OpenedAt: secondNow, Facts: []string{"raise the box again"}, Wants: "allow two more rounds", Budget: &secondBudget, Thread: &MessageRef{ID: "20", ThreadID: "20"}, State: "open"}
	if err := writeJSON(questionPath(root, second.ID), second); err != nil {
		t.Fatal(err)
	}
	code, _ = TOTPCode("JBSWY3DPEHPK3PXP", secondNow)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "21", ThreadID: "20"}, ThreadID: "20", UserID: "UWIDO", Text: "allow two more rounds " + code, SentAt: secondNow}}
	if _, err := Poll(context.Background(), pollBedConfig(root, p, secondNow)); err != nil {
		t.Fatal(err)
	}
	afterSecond := projectGoal(t, root, "g")
	if afterSecond.Budget == nil || *afterSecond.Budget != secondBudget || afterSecond.Approved == nil || afterSecond.Approved.Authority != goal.ApprovalAuthorityChannel || afterSecond.NormApproval == nil || afterSecond.NormApproval.ApprovedRef != second.ID {
		t.Fatalf("second verified answer did not replace the box: %+v", afterSecond)
	}
	syncAccepted(t, root)
	ep, err := goal.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := goal.Claim(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: "machine", Lineage: "lineage"}, Ulid: "01J5X0000000000000000000B3", Now: secondNow.Add(time.Minute), ClaimEpoch: 1}, "g")
	if err != nil || claim.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("raised box did not reopen admission: %+v %v", claim, err)
	}
}

func TestBudgetFreeTextAndOtherQuestionDoNotRaiseBox(t *testing.T) {
	for _, test := range []struct {
		name, kind, wants, answer string
		withBudget                bool
	}{
		{name: "free text budget answer", kind: "budget-above-norm", wants: "yes", answer: "please raise it", withBudget: true},
		{name: "other question token", kind: "other", wants: "yes", answer: "yes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, p, _, now := pollLedgerBed(t)
			budget, _ := goal.NewBudget("2h", 5, 600, 1, 0)
			q := Question{ID: "01J5X0000000000000000000N1", Goal: "g", Kind: test.kind, Machine: "machine", OpenedAt: now, Facts: []string{"question"}, Wants: test.wants, Thread: &MessageRef{ID: "10", ThreadID: "10"}, State: "open"}
			if test.withBudget {
				q.Budget = &budget
			}
			if err := writeJSON(questionPath(root, q.ID), q); err != nil {
				t.Fatal(err)
			}
			code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
			p.inbound = []Inbound{{Ref: MessageRef{ID: "11", ThreadID: "10"}, ThreadID: "10", UserID: "UWIDO", Text: test.answer + " " + code, SentAt: now}}
			if _, err := Poll(context.Background(), pollBedConfig(root, p, now)); err != nil {
				t.Fatal(err)
			}
			got := projectGoal(t, root, "g")
			if got.Budget != nil || got.Approved != nil || got.State != goal.StateQueued {
				t.Fatalf("non-token answer changed approval: %+v", got)
			}
		})
	}
}

func TestLegacyBudgetQuestionAnswerRaisesNothing(t *testing.T) {
	root, p, _, now := pollLedgerBed(t)
	q := Question{ID: "01J5X0000000000000000000L3", Goal: "g", Kind: "budget-above-norm", Machine: "machine", OpenedAt: now, Facts: []string{"raise the box"}, Wants: "yes", Thread: &MessageRef{ID: "10", ThreadID: "10"}, State: "open"}
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "11", ThreadID: "10"}, ThreadID: "10", UserID: "UWIDO", Text: "yes " + code, SentAt: now}}
	if _, err := Poll(context.Background(), pollBedConfig(root, p, now)); err != nil {
		t.Fatal(err)
	}

	record, err := ReadQuestion(root, q.ID)
	if err != nil {
		t.Fatal(err)
	}
	if record.State != "closed" || record.Answer == nil || record.Answer.Phase != "closed" || record.Answer.Text != "yes" {
		t.Fatalf("legacy answer was not recorded and closed: %+v", record)
	}
	if !strings.Contains(record.Answer.Receipt, "nothing raised") {
		t.Fatalf("legacy answer receipt=%q", record.Answer.Receipt)
	}
	got := projectGoal(t, root, "g")
	if got.Budget != nil || got.Approved != nil {
		t.Fatalf("legacy answer changed approval: %+v", got)
	}
	for _, history := range got.History {
		if history.Verb == "approve" {
			t.Fatalf("legacy answer wrote an approve row: %+v", history)
		}
	}
	if len(p.posts) == 0 || !strings.Contains(p.posts[len(p.posts)-1], "nothing raised") {
		t.Fatalf("legacy answer posts=%q", p.posts)
	}
}

func TestAuthenticatedChannelAuthorityAfterTemporaryHorizon(t *testing.T) {
	root, p, first, _ := pollLedgerBed(t)
	now := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	proof, err := humanauthority.AuthenticatedChannelProof(root, governance.RecordedChannelAuthority{Outcome: governance.AuthorityOutcomeAuthenticatedChannelWord, Provider: "slack", UserID: "UWIDO", MessageRef: "1/2", Step: 42}, now)
	if err != nil || !proof.AuthorizesResume(root) || !proof.AuthorizesSetObligation(root) || proof.TemporaryResumeFor(root) {
		t.Fatalf("channel authority did not survive the temporary horizon: %+v %v", proof, err)
	}
	budget, _ := goal.NewBudget("4h", 3, 90, 2, 0)
	resumeToken := goal.ResumeApprovalToken("g", budget)
	first.Wants = "goal=g minutes=90 goalRevision=1"
	if err := writeJSON(questionPath(root, first.ID), first); err != nil {
		t.Fatal(err)
	}
	code, _ := TOTPCode("JBSWY3DPEHPK3PXP", now)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "2", ThreadID: "1"}, ThreadID: "1", UserID: "UWIDO", Text: "approve " + code}}
	if _, err := Poll(context.Background(), pollBedConfig(root, p, now)); err != nil {
		t.Fatal(err)
	}
	answered, _ := ReadQuestion(root, first.ID)
	if _, err := goal.AuthenticatedChannelApproval(root, "g", answered.Answer.Opid, resumeToken, now); err == nil {
		t.Fatal("a budget-above-norm answer authorized a resume without the resume tuple")
	}

	resumeQ := Question{ID: "01J5X0000000000000000000Q2", Goal: "g", Kind: "stop", Machine: "machine", OpenedAt: now.Add(30 * time.Second), Facts: []string{"resume"}, Wants: resumeToken, Thread: &MessageRef{ID: "3", ThreadID: "3"}, State: "open"}
	if err := writeJSON(questionPath(root, resumeQ.ID), resumeQ); err != nil {
		t.Fatal(err)
	}
	code, _ = TOTPCode("JBSWY3DPEHPK3PXP", resumeQ.OpenedAt)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "4", ThreadID: "3"}, ThreadID: "3", UserID: "UWIDO", Text: resumeToken + " " + code}}
	if _, err := Poll(context.Background(), pollBedConfig(root, p, resumeQ.OpenedAt)); err != nil {
		t.Fatal(err)
	}
	resumeAnswer, _ := ReadQuestion(root, resumeQ.ID)
	if _, err := goal.AuthenticatedChannelApproval(root, "g", resumeAnswer.Answer.Opid, resumeToken, resumeQ.OpenedAt); err != nil {
		t.Fatal("resume token refused its first use:", err)
	}
	landApprovalUse(t, root, "resume", resumeAnswer.Answer.Opid, resumeQ.OpenedAt)
	if _, err := goal.AuthenticatedChannelApproval(root, "g", resumeAnswer.Answer.Opid, resumeToken, resumeQ.OpenedAt); err == nil {
		t.Fatal("resume token was accepted twice")
	}

	obligationToken := goal.SetObligationApprovalToken("g", goal.ObligationObserve, "machine")
	obligationQ := Question{ID: "01J5X0000000000000000000Q3", Goal: "g", Kind: "other", Machine: "machine", OpenedAt: now.Add(60 * time.Second), Facts: []string{"obligation"}, Wants: obligationToken, Thread: &MessageRef{ID: "5", ThreadID: "5"}, State: "open"}
	if err := writeJSON(questionPath(root, obligationQ.ID), obligationQ); err != nil {
		t.Fatal(err)
	}
	code, _ = TOTPCode("JBSWY3DPEHPK3PXP", obligationQ.OpenedAt)
	p.inbound = []Inbound{{Ref: MessageRef{ID: "6", ThreadID: "5"}, ThreadID: "5", UserID: "UWIDO", Text: obligationToken + " " + code}}
	if _, err := Poll(context.Background(), pollBedConfig(root, p, obligationQ.OpenedAt)); err != nil {
		t.Fatal(err)
	}
	obligationAnswer, _ := ReadQuestion(root, obligationQ.ID)
	if _, err := goal.AuthenticatedChannelApproval(root, "g", obligationAnswer.Answer.Opid, obligationToken, obligationQ.OpenedAt); err != nil {
		t.Fatal("set-obligation token refused its first use:", err)
	}
	landApprovalUse(t, root, "set-obligation", obligationAnswer.Answer.Opid, obligationQ.OpenedAt)
	if _, err := goal.AuthenticatedChannelApproval(root, "g", obligationAnswer.Answer.Opid, obligationToken, obligationQ.OpenedAt); err == nil {
		t.Fatal("set-obligation token was accepted twice")
	}
}

func landApprovalUse(t *testing.T, root, verb, approvedRef string, now time.Time) {
	t.Helper()
	g := projectGoal(t, root, "g")
	g.Revision++
	g.History = append(g.History, goal.HistoryLine{At: now.UTC().Format(time.RFC3339), Opid: goal.Opid("01J5X0000000000000000000A0", "machine", verb), Verb: verb, Actor: "human:wido", Targets: []string{"g"}, Keep: -1, ApprovedRef: approvedRef})
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "g.md"), goal.RenderFile(g), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", root, "add", "plans/goals/g.md")
	cmd.Env = testGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	cmd = exec.Command("git", "-C", root, "commit", "-q", "-m", "record approval use")
	cmd.Env = testGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
	cmd = exec.Command("git", "-C", root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	cmd.Env = testGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("update local ledger: %v %s", err, out)
	}
	syncAccepted(t, root)
}

func pollBedConfig(root string, p Provider, now time.Time) PollConfig {
	return PollConfig{RepoRoot: root, Destination: "fleet", ProviderName: "fake", HumanUserID: "UWIDO", TOTPSecret: "JBSWY3DPEHPK3PXP", Machine: "machine", Lineage: "lineage", Provider: p, Now: now}
}
func pollLedgerBed(t *testing.T) (string, *testProvider, Question, time.Time) {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = testGitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "channel-test")
	run("config", "user.email", "channel@example.invalid")
	run("config", "goal.sync-remote", "local")
	if endpoint, err := goal.ResolveEndpoint(root); err != nil || !endpoint.LocalMode() {
		t.Fatalf("local goal endpoint not resolved: %+v %v", endpoint, err)
	}
	_ = os.MkdirAll(filepath.Join(root, "plans", "goals"), 0755)
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "backlog.md"), goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}), 0644); err != nil {
		t.Fatal(err)
	}
	opened := goal.HistoryLine{At: "2026-09-03T00:00:00Z", Opid: goal.Opid("01J5X0000000000000000000F0", "machine", "lineage"), Verb: "open", Actor: "machine+lineage", Targets: []string{"g"}, Keep: -1}
	f := &goal.GoalFile{Id: "g", State: goal.StateQueued, Tier: 1, Intent: "Question target.", Origin: "main", NextStep: "Wait.", OpenedAt: "2026-09-03T00:00:00Z", Revision: 1, History: []goal.HistoryLine{opened}}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "g.md"), goal.RenderFile(f), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.tier-1=1h/3/360m/1/0\nmetasystem.budget.review-round-max=3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "plans/goals")
	run("commit", "-q", "-m", "seed channel ledger")
	run("update-ref", goal.LocalLedgerBranch, "HEAD")
	run("update-ref", goal.AcceptedRef, "HEAD")
	q := Question{ID: "01J5X0000000000000000000Q0", Goal: "g", Kind: "other", Machine: "machine", OpenedAt: time.Date(2026, 9, 3, 1, 0, 0, 0, time.UTC), Facts: []string{"fact"}, Thread: &MessageRef{ID: "1", ThreadID: "1"}, State: "open"}
	if err := writeJSON(questionPath(root, q.ID), q); err != nil {
		t.Fatal(err)
	}
	return root, &testProvider{cursor: "done"}, q, q.OpenedAt
}
func testGitEnv() []string {
	drop := map[string]bool{"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_COMMON_DIR": true, "GIT_INDEX_FILE": true, "GIT_CEILING_DIRECTORIES": true, "GIT_OBJECT_DIRECTORY": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true, "GIT_CONFIG": true, "GIT_CONFIG_PARAMETERS": true, "GIT_CONFIG_COUNT": true, "GIT_CONFIG_GLOBAL": true, "GIT_CONFIG_SYSTEM": true, "GIT_CONFIG_NOSYSTEM": true, "GIT_GRAFT_FILE": true, "GIT_SHALLOW_FILE": true, "GIT_REPLACE_REF_BASE": true}
	out := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !drop[name] && !strings.HasPrefix(name, "GIT_CONFIG_KEY_") && !strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			out = append(out, entry)
		}
	}
	return out
}
func projectGoal(t *testing.T, root, id string) *goal.GoalFile {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "show", goal.LocalLedgerBranch+":plans/goals/"+id+".md")
	cmd.Env = testGitEnv()
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	f, problems := goal.ParseFile(b)
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	return f
}
func syncAccepted(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "update-ref", goal.AcceptedRef, goal.LocalLedgerBranch)
	cmd.Env = testGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("advance accepted ref: %v %s", err, out)
	}
}
