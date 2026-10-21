package channel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

var questionTextNow = time.Date(2031, 1, 1, 9, 0, 0, 0, time.UTC)

// attemptProvider observes the durable words before accepting or rejecting a post.
type attemptProvider struct {
	testProvider
	t       *testing.T
	root    string
	fail    int
	attempt []string
	durable []string
}

func (p *attemptProvider) Post(_ context.Context, _ DestinationConfig, text string, _ *MessageRef) (MessageRef, error) {
	p.t.Helper()
	questions, unread := WalkQuestions(p.root)
	if len(unread) != 0 || len(questions) != 1 {
		p.t.Fatalf("before post: questions=%+v unread=%v", questions, unread)
	}
	p.attempt = append(p.attempt, text)
	p.durable = append(p.durable, questions[0].Text)
	if p.fail > 0 {
		p.fail--
		return MessageRef{}, errors.New("post refused")
	}
	return MessageRef{ID: "posted", ThreadID: "posted"}, nil
}

func questionTextBed(t *testing.T) *pollRepository {
	t.Helper()
	bed, _, q, _ := pollLedgerBed(t)
	if err := os.Remove(questionPath(bed.root, q.ID)); err != nil {
		t.Fatal(err)
	}
	return bed
}

func pollQuestionText(t *testing.T, bed *pollRepository, p Provider) error {
	t.Helper()
	_, err := bed.poll(context.Background(), PollConfig{RepoRoot: bed.root, Provider: p, Now: questionTextNow})
	return err
}

func TestPostedQuestionTextIsD3(t *testing.T) {
	t.Parallel()
	box, err := goal.NewBudget("2h", 5, 600, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	revive := `metasystem machine revive "m" --after "01JABCDEFGHJKMNPQRSTVWXYZ0"`
	revise := "metasystem work revise g --brief FILE --reason TEXT --by NAME"
	hash := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name string
		ask  AskRequest
		want string
	}{
		{"goal", AskRequest{Goal: "g", Facts: []string{"Keep main's version?"}, Options: []Option{{"yes", "keep it"}, {"no", "write a third"}}, Recommendation: "no", Plain: &Plain{Decision: "Keep main's version?", IfNothing: "nothing lands until you answer"}}, "Keep main's version?\nTo answer: metasystem question answer %s\nIf you do nothing: nothing lands until you answer.\n- yes: keep it.\n- no: write a third."},
		{"recovery", AskRequest{About: "machine", Machine: "m", Recovery: &RecoverySubject{Host: "host", Seat: "m", Launch: "01JABCDEFGHJKMNPQRSTVWXYZ0", Provider: "p"}, Wants: revive, Facts: []string{revive, "Seat m still holds approved work after provider p recovered."}}, "The machine m needs your answer.\nTo go on: " + revive + "\nIf you do nothing: this question stays open."},
		{"unit-stop", AskRequest{Goal: "g", UnitStop: &UnitStopQuestion{Loop: "unit-round", Subject: "g/u/run", Attempt: 1, Finding: "F-1", Needs: revise, AcceptableActs: []string{"work-revise"}}}, "Goal g needs your answer.\nTo go on: " + revise + "\nIf you do nothing: this question stays open."},
		{"process-act", AskRequest{Goal: "g", ProcessAct: hash, Wants: "person check " + hash}, "Goal g needs your answer.\nTo go on: person check " + hash + "\nIf you do nothing: this question stays open."},
		{"budget", AskRequest{Goal: "g", Kind: "budget-above-norm", Budget: &box, Wants: "yes"}, "Goal g needs your answer.\nTo answer: metasystem question answer %s\nIf you do nothing: this question stays open."},
		{"carry", AskRequest{Goal: "g", Kind: "carry", Wants: "carry workspace=" + strings.Repeat("b", 40) + " goal=g past=p"}, "Goal g needs your answer.\nTo answer: metasystem question answer %s\nIf you do nothing: this question stays open."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bed := questionTextBed(t)
			root := bed.root
			r := tc.ask
			r.RepoRoot, r.Now = root, questionTextNow
			if r.Kind == "" {
				r.Kind = "other"
			}
			if len(r.Facts) == 0 {
				r.Facts = []string{"finding remains"}
			}
			q, _, err := AskOrFind(r)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.ReplaceAll(tc.want, "%s", q.ID)
			p := &attemptProvider{t: t, root: root}
			if err := pollQuestionText(t, bed, p); err != nil {
				t.Fatal(err)
			}
			if len(p.attempt) != 1 || p.attempt[0] != want || q.Text != want {
				t.Fatalf("post=%q stored=%q want=%q", p.attempt, q.Text, want)
			}
		})
	}
}

func TestQuestionTextIsDurableBeforeEveryAttempt(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"new", "legacy", "legacy-retry", "stored-wins-poll", "stored-wins-retry", "legacy-write-fails-poll", "legacy-write-fails-retry"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := questionTextBed(t)
			root := bed.root
			q := Question{ID: "q-legacy", Goal: "g", Kind: "other", State: "open", OpenedAt: questionTextNow, Facts: []string{"legacy fact"}}
			p := &attemptProvider{t: t, root: root, fail: 1}
			if name == "new" {
				var err error
				q, err = Ask(AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Facts: []string{"fact"}, Plain: &Plain{Decision: "Continue?", IfNothing: "the release waits"}, Now: questionTextNow, Provider: p})
				if err != nil || q.Undelivered != 1 || q.Thread != nil {
					t.Fatalf("new failed delivery: %+v %v", q, err)
				}
			} else {
				if strings.HasPrefix(name, "stored-wins") {
					q.Text, p.fail = "stored words", 0
					if renderPlain(q) == q.Text {
						t.Fatal("stored words must differ from current rendering")
					}
				}
				if err := writeJSON(questionPath(root, q.ID), q); err != nil {
					t.Fatal(err)
				}
			}
			writeFails := strings.Contains(name, "write-fails")
			if writeFails {
				if os.Geteuid() == 0 {
					t.Skip("root can write a read-only directory")
				}
				dir := filepath.Dir(questionPath(root, q.ID))
				if err := os.Chmod(dir, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			}
			attempt := func() error {
				if strings.HasSuffix(name, "retry") {
					_, _, err := RetryDelivery(context.Background(), root, q.ID, p, DestinationConfig{})
					return err
				}
				return pollQuestionText(t, bed, p)
			}
			err := attempt()
			if writeFails {
				stored, readErr := ReadQuestion(root, q.ID)
				if err == nil || len(p.attempt) != 0 || readErr != nil || stored.Text != "" {
					t.Fatalf("failed pre-post write: err=%v attempts=%q stored=%q read=%v", err, p.attempt, stored.Text, readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(name, "stored-wins") {
				if len(p.attempt) != 1 || p.attempt[0] != "stored words" {
					t.Fatalf("stored words replaced: attempts=%q", p.attempt)
				}
				return
			}
			// A new ask already failed once; its next poll delivered it above.
			if name != "new" {
				if err := attempt(); err != nil {
					t.Fatal(err)
				}
			}
			stored, err := ReadQuestion(root, q.ID)
			if err != nil || stored.Thread == nil || stored.Undelivered != 1 || len(p.attempt) != 2 {
				t.Fatalf("retry state: %+v err=%v attempts=%q", stored, err, p.attempt)
			}
			for i, text := range p.attempt {
				if text == "" || text != p.durable[i] || text != p.attempt[0] || text != stored.Text {
					t.Errorf("attempt %d: payload=%q durable=%q final=%q", i+1, text, p.durable[i], stored.Text)
				}
			}
		})
	}
}

func TestClosureLineIsPlain(t *testing.T) {
	t.Parallel()
	root, p := t.TempDir(), &testProvider{}
	q, err := Ask(AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Facts: []string{"fact"}, Now: questionTextNow, Provider: p})
	if err != nil || q.Thread == nil {
		t.Fatalf("ask: %+v %v", q, err)
	}
	for i := 0; i < 2; i++ {
		if _, err := Withdraw(root, q.ID, "the goal was parked", p, DestinationConfig{}); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.posts) != 2 || p.posts[1] != "Closed: the goal was parked." || p.postThreads[1] == nil || *p.postThreads[1] != *q.Thread {
		t.Fatalf("closure: posts=%q threads=%+v", p.posts, p.postThreads)
	}
}

func TestOtherWordsAreTheSameQuestion(t *testing.T) {
	t.Parallel()
	root, p := t.TempDir(), &testProvider{}
	r := AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Facts: []string{"fact"}, Plain: &Plain{Decision: "Continue?", IfNothing: "the release waits"}, Now: questionTextNow, Provider: p}
	first, _, err := AskOrFind(r)
	if err != nil {
		t.Fatal(err)
	}
	r.Plain = &Plain{Decision: "Continue?", IfNothing: "a differently worded wait"}
	second, existing, err := AskOrFind(r)
	qs, unread := WalkQuestions(root)
	if err != nil || !existing || second.ID != first.ID || second.Text != first.Text || len(p.posts) != 1 || len(qs) != 1 || len(unread) != 0 {
		t.Fatalf("repeat ask: existing=%t err=%v questions=%d unread=%v posts=%q", existing, err, len(qs), unread, p.posts)
	}
}

func TestPlainSentenceAndDecisionBound(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		text    string
		capital bool
		want    string
	}{
		{" \n\t ", true, ""}, {" élan\n  goes\ton ", true, "Élan goes on."},
		{"leave this!", false, "leave this!"}, {"keep it?", true, "Keep it?"}, {"done.", true, "Done."},
	} {
		if got := plainSentence(tc.text, tc.capital); got != tc.want {
			t.Errorf("sentence(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
	q := Question{Plain: &Plain{Decision: strings.Repeat("é", 401), Act: "explicit act", IfNothing: "nothing\n  changes"}, UnitStop: &UnitStopQuestion{Needs: "fallback act"}}
	want := strings.Repeat("é", 399) + "….\nTo go on: explicit act\nIf you do nothing: nothing changes."
	want = "É" + strings.TrimPrefix(want, "é")
	if got := renderPlain(q); got != want {
		t.Fatalf("bounded post = %q, want %q", got, want)
	}
}
