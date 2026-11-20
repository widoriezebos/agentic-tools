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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

type namedActFailureRepository struct {
	goal.Repository
	root           string
	readFailure    bool
	captureFailure bool
	captures       int
	t              *testing.T
}

func (r *namedActFailureRepository) Files(tip string, prefixes ...string) (map[string][]byte, error) {
	if r.readFailure && len(prefixes) == 1 && prefixes[0] == namedActPrefix {
		return nil, errors.New("shared records unavailable")
	}
	return r.Repository.Files(tip, prefixes...)
}
func (r *namedActFailureRepository) Capture(opid string) (string, error) {
	r.captures++
	if strings.Contains(opid, "-acting-") {
		if err := withPollLock(r.root, func() error { return nil }); err != nil {
			r.t.Errorf("answer publication holds the channel lock: %v", err)
		}
		if r.captureFailure {
			return "", errors.New("remote unreachable")
		}
	}
	return r.Repository.Capture(opid)
}

func TestQ1MatchingFailuresLeaveOtherQuestionsUsable(t *testing.T) {
	t.Parallel()
	for _, trigger := range []string{"ledger unreadable", "publication unavailable", "incomplete", "newer format", "malformed", "closed goal"} {
		t.Run(trigger, func(t *testing.T) {
			t.Parallel()
			b := newNamedActBed(t)
			q := b.ask(t, "Accept?", nil)
			if trigger == "closed goal" || trigger == "publication unavailable" {
				b.record(t, b.act, b.now.Add(time.Second))
			}
			if trigger == "closed goal" {
				tip, _, err := b.ep.Repository.Accepted()
				if err != nil {
					t.Fatal(err)
				}
				files, err := b.ep.Repository.Files(tip, "plans/", "records/")
				if err != nil {
					t.Fatal(err)
				}
				f, problems := goal.ParseFile(files["plans/goals/g.md"])
				if len(problems) > 0 {
					t.Fatal(problems)
				}
				f.State = goal.StateDone
				f.Conclude = "The work finished."
				files["records/goals/g.md"] = goal.RenderFile(f)
				delete(files, "plans/goals/g.md")
				b.ep.Repository = testgoal.New(files, b.now, strings.Repeat("2", 40))
			}
			// This ordinary question must still receive and settle its own reply.
			ordinary := b.ask(t, "Ordinary question", func(r *AskRequest) { r.Goal = ""; r.About = "machine"; r.Options[0].NamedAct = nil })
			legacy := b.ask(t, "Start the work?", func(r *AskRequest) {
				r.Options[0].NamedAct = nil
				r.PersonAct = &PersonAct{Verb: "work build", Goal: "g"}
			})
			var healthy Question
			if trigger == "incomplete" || trigger == "newer format" || trigger == "malformed" || trigger == "ledger unreadable" {
				act := NamedAct{Verb: "work stop", Goal: "g", Target: "healthy-run"}
				healthy = b.ask(t, "Stop the other run?", func(r *AskRequest) { r.Options[0].NamedAct = &act })
				b.record(t, act, b.now.Add(time.Second))
			}
			repository := &namedActFailureRepository{Repository: b.ep.Repository, root: b.root, t: t, readFailure: trigger == "ledger unreadable", captureFailure: trigger == "publication unavailable"}
			b.ep.Repository = repository
			bad := ""
			switch trigger {
			case "incomplete":
				bad = `{"id":"incomplete"}`
			case "newer format":
				bad = `{"id":"future","act":{"verb":"future verb","goal":"g","target":"x"},"at":"2026-09-03T01:00:01Z","person":"Wido"}`
			case "malformed":
				bad = `{`
			}
			if bad != "" {
				dir := filepath.Join(channelRoot(b.root), "named-acts")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(bad), 0600); err != nil {
					t.Fatal(err)
				}
			}
			provider := &testProvider{}
			c := pollBedConfig(&pollRepository{root: b.root}, provider, b.now.Add(time.Minute))
			code, err := TOTPCode(c.TOTPSecret, c.Now)
			if err != nil {
				t.Fatal(err)
			}
			provider.inbound = []Inbound{{Ref: MessageRef{ID: "ordinary-reply", ThreadID: ordinary.ID}, ThreadID: ordinary.ID, UserID: c.HumanUserID, Text: "Wait " + code, SentAt: c.Now}}
			for range 2 {
				if _, err := pollWithEndpoint(context.Background(), c, b.resolve); err != nil {
					t.Fatal(err)
				}
			}
			got := b.read(t, ordinary)
			if got.State != "closed" || got.Answer == nil || got.Answer.Text != "Wait" {
				t.Fatalf("unrelated reply did not settle: %+v", got)
			}
			got = b.read(t, q)
			if trigger == "closed goal" {
				if got.State != "closed" || got.Answer == nil || got.Answer.Phase != "closed" {
					t.Fatalf("closed goal trapped question: %+v", got)
				}
			} else if got.State != "open" || !strings.Contains(got.ActCheck, "could not be checked now:") {
				t.Fatalf("missing question-local diagnostic: %+v", got)
			}
			if healthy.ID != "" && b.read(t, healthy).State != "closed" {
				t.Fatal("one failed match blocked a healthy named act")
			}
			if err := ClosePersonActs(b.root, *legacy.PersonAct, b.now.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			if b.read(t, legacy).State != "closed" {
				t.Fatal("matching failure blocked the existing person-act route")
			}
			// The tolerant walk must not turn a matching failure into a damaged file.
			if _, bad := WalkOpenQuestions(b.root); len(bad) > 0 {
				t.Fatalf("matching failure poisoned open-question reads: %v", bad)
			}
		})
	}
}

func TestQ1OpenQuestionReadsUseAcceptedRecordsWithoutFetching(t *testing.T) {
	t.Parallel()
	b := newNamedActBed(t)
	q := b.ask(t, "Accept?", nil)
	repository := &namedActFailureRepository{Repository: b.ep.Repository, root: b.root, t: t, captureFailure: true}
	b.ep.Repository = repository
	for range 3 {
		b.settle(t)
	}
	if repository.captures != 0 || b.read(t, q).State != "open" {
		t.Fatalf("reading unmatched question fetched %d times", repository.captures)
	}
}

func TestQ1RunCompletionRecordsOnlyOpenMatchingQuestions(t *testing.T) {
	t.Parallel()
	b := newNamedActBed(t)
	act := NamedAct{Verb: "work stop", Goal: "g", Target: "run-x"}
	if a := b.record(t, act, b.now); a.ID != "" {
		t.Fatal("recorded a stop without a question")
	}
	q := b.ask(t, "Cancel?", func(r *AskRequest) { r.Options[0].NamedAct = &act })
	first := b.record(t, act, b.now.Add(time.Second))
	repeat := b.record(t, act, b.now.Add(2*time.Second))
	if first.ID == "" || repeat.ID != first.ID {
		t.Fatal("repeat created another completion record")
	}
	b.settle(t)
	if b.read(t, q).State != "closed" {
		t.Fatal("unchanged stop did not answer")
	}
	if a := b.record(t, act, b.now.Add(3*time.Second)); a.ID != "" {
		t.Fatal("recorded stop after its question closed")
	}
	paths, err := filepath.Glob(filepath.Join(channelRoot(b.root), "named-acts", "*.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("completion files %d: %v", len(paths), err)
	}
}

func TestQ1RepeatedStopAnswersANewerQuestionBesideAPendingAnswer(t *testing.T) {
	t.Parallel()
	b := newNamedActBed(t)
	act := NamedAct{Verb: "work stop", Goal: "g", Target: "run-x"}
	old := b.ask(t, "Cancel the run?", func(r *AskRequest) { r.Options[0].NamedAct = &act })
	first := b.record(t, act, b.now.Add(time.Second))
	failed := func(string) (goal.Endpoint, error) { return goal.Endpoint{}, errors.New("unavailable") }
	if err := ReconcileNamedActs(b.root, failed); err == nil {
		t.Fatal("fixture did not interrupt publication")
	}
	fresh := b.ask(t, "Cancel the run again?", func(r *AskRequest) { r.Options[0].NamedAct = &act })
	repeat := b.record(t, act, b.now.Add(2*time.Second))
	if repeat.ID == "" || repeat.ID == first.ID {
		t.Fatal("older pending answer swallowed the new invocation")
	}
	b.settle(t)
	if b.read(t, old).State != "closed" || b.read(t, fresh).State != "closed" {
		t.Fatal("the repeated stop did not answer both questions")
	}
}
