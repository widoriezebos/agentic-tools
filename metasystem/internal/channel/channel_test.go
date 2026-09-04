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
func TestReportOmitsEmptySectionsAndCaps(t *testing.T) {
	text, err := ComposeReport(ReportConfig{RepoRoot: t.TempDir(), Machine: "m", Now: time.Unix(0, 0)})
	if err != nil || strings.Contains(text, "Undelivered:") || len(text) > 3500 {
		t.Fatal(text, err)
	}
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
}
func TestReportHasDurableSpendSource(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "artifacts", "agents", "jobs")
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "a.json"), []byte(`{"startedAt":"1970-01-01T00:00:01Z","runtime":"codex","usage":{"inputTokens":3,"cost":99}}`), 0644)
	line := spendLine(ReportConfig{RepoRoot: root, Now: time.Unix(2, 0)})
	if !strings.Contains(line, "codex: 3 units") || strings.Contains(line, "99") {
		t.Fatal(line)
	}
}
func TestReportComposesFromLedgerJobsAndLandings(t *testing.T) {
	root, _, _, _ := pollLedgerBed(t)
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	g := projectGoal(t, root, "g")
	g.Intent, g.NextStep, g.State, g.Revision = "Ship the fleet conversation. Keep it authenticated.", "Finish the gate. Then land.", goal.StateClaimed, 2
	g.Budget = &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1}
	g.Claimed = &goal.ClaimRecord{Machine: "fleet-one", Lineage: "lineage", At: now.Add(-time.Hour).Format(time.RFC3339), Revision: 2}
	g.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "fleet-one", ClaimEpoch: 1}
	g.History = append(g.History, goal.HistoryLine{At: now.Add(-time.Hour).Format(time.RFC3339), Opid: goal.Opid("01J5X0000000000000000000C0", "fleet-one", "lineage"), Verb: "claim", Actor: "fleet-one+lineage", Targets: []string{"g"}, Keep: -1})
	planned := &goal.GoalFile{Id: "next-item", State: goal.StateQueued, Intent: "Queue the follow-up. Preserve the contract.", Origin: "main", NextStep: "Wait.", OpenedAt: now.Add(-2 * time.Hour).Format(time.RFC3339), Revision: 1, Pinned: "fleet-one", History: []goal.HistoryLine{{At: now.Add(-2 * time.Hour).Format(time.RFC3339), Opid: goal.Opid("01J5X0000000000000000000C1", "fleet-one", "lineage"), Verb: "open", Actor: "fleet-one+lineage", Targets: []string{"next-item"}, Keep: -1}}}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "g.md"), goal.RenderFile(g), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "next-item.md"), goal.RenderFile(planned), 0644); err != nil {
		t.Fatal(err)
	}
	reportGit(t, root, "add", "plans/goals")
	reportGit(t, root, "commit", "-q", "-m", "prepare report ledger")
	reportGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	reportGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	for _, subject := range []string{"Land one", "Land two", "Land three"} {
		reportGit(t, root, "commit", "-q", "--allow-empty", "-m", subject+"\n\nGoal-Item: g")
	}
	reportGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, "running.json"), []byte(`{"jobId":"running","goalId":"g","role":"implementer","status":"running","runtime":"codex","startedAt":"2026-09-03T11:30:00Z","usage":{"inputTokens":3,"outputTokens":7}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, "done.json"), []byte(`{"jobId":"done","goalId":"g","role":"critic","status":"completed","runtime":"devin","startedAt":"2026-09-03T10:00:00Z","usage":{}}`), 0644); err != nil {
		t.Fatal(err)
	}
	text, err := ComposeReport(ReportConfig{RepoRoot: root, Machine: "fleet-one", Now: now, WindowStart: now.Add(-4 * time.Hour)})
	want := strings.Join([]string{
		"fleet-one status 2026-09-03 12:00Z",
		"Landed since 2026-09-03T08:00:00Z: g — Ship the fleet conversation. — Land three (3 landings)",
		"Under way: g — Ship the fleet conversation.; Finish the gate.; job implementer running 30 min",
		"Planned: next item — Queue the follow-up. (queued, needs budget)",
		"Spend today: 2 jobs; codex: 10 units",
	}, "\n")
	if err != nil || text != want {
		t.Fatalf("report mismatch\n--- got ---\n%s\n--- want ---\n%s\nerr=%v", text, want, err)
	}
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
	f := &goal.GoalFile{Id: "g", State: goal.StateQueued, Intent: "Question target.", Origin: "main", NextStep: "Wait.", OpenedAt: "2026-09-03T00:00:00Z", Revision: 1, History: []goal.HistoryLine{opened}}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "g.md"), goal.RenderFile(f), 0644); err != nil {
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
