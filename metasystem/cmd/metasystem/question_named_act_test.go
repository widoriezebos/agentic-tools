package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// The public parser, ask owner, human verb and answer publisher all run;
// only the goal repository and process ancestry are fixture boundaries.
func TestQ1AcceptRiskAnswersDeclaredOption(t *testing.T) {
	t.Parallel()
	b, owners := newQ1CommandBed(t)
	code, out, errout := b.run(owners, "question", "ask", bedGoal, "--question", "Accept this risk?", "--option", "Accept: keep the risk recorded", "--option-act", `Accept: metasystem goal accept-risk `+bedGoal+` --finding S-1 --review critic-r2`)
	if code != 0 {
		t.Fatalf("ask: %d %s %s", code, out, errout)
	}
	questions, bad := channel.WalkQuestions(b.root())
	if len(questions) != 1 || len(bad) != 0 {
		t.Fatalf("questions %v %v", questions, bad)
	}
	q := questions[0]
	code, out, errout = b.run(owners, "goal", "accept-risk", bedGoal, "--review", "critic-r2", "--reason", "bounded exposure", "--finding", "S-1")
	if code != 0 {
		t.Fatalf("accept: %d %s %s", code, out, errout)
	}
	q, err := channel.ReadQuestion(b.root(), q.ID)
	if err != nil || q.Answer == nil || q.Answer.Text != "Accept" || q.State != "closed" {
		t.Fatalf("answer: %+v %v", q, err)
	}
	if !strings.Contains(out, "answered") {
		t.Fatalf("missing confirmation: %s", out)
	}
}

func newQ1CommandBed(t *testing.T) (*intentBed, intentOwners) {
	t.Helper()
	b := newIntentBed(t, false, nil)
	writeCriticChain(t, b.root())
	owners := b.owners()
	now, _ := b.commandNow(b.root())
	owners.prove = enrolledPersonProver(t, b.root(), now)
	owners.processes = defaultProcessIntentOwners()
	owners.processes.ask = func(root string, in channelAskInput) (channel.Question, []string, int, error) {
		return askChannelQuestionVia(root, in, channelAskSurface{endpoint: owners.dependencies.endpoint, now: b.commandNow,
			identity: func(string) (string, string, error) { return "asking", "seat", nil },
			load:     func(string) (phase.Loaded, error) { return phase.Loaded{Provider: &questionProvider{}}, nil },
			cursor:   func(string) (string, bool, error) { return "", false, nil },
		})
	}

	return b, owners
}

func q1Ask(t *testing.T, b *intentBed, owners intentOwners, label, text string) channel.Question {
	t.Helper()
	code, out, errout := b.run(owners, "question", "ask", bedGoal, "--question", text, "--option", label+": keep the risk recorded", "--option", "Wait: wait for another day", "--option-act", label+": metasystem goal accept-risk "+bedGoal+" --finding S-1 --review critic-r2")
	if code != 0 {
		t.Fatalf("ask %d %s %s", code, out, errout)
	}
	qs, _ := channel.WalkQuestions(b.root())
	for _, q := range qs {
		if len(q.Facts) > 0 && q.Facts[0] == text {
			return q
		}
	}
	t.Fatal("asked question missing")
	return channel.Question{}
}
func q1Accept(b *intentBed, owners intentOwners) (int, string, string) {
	return b.run(owners, "goal", "accept-risk", bedGoal, "--finding", "S-1", "--review", "critic-r2", "--reason", "a different reason", "--by", "Wido")
}

func TestQ1UnchangedActWaitAndSteward(t *testing.T) {
	t.Parallel()
	b, owners := newQ1CommandBed(t)
	if code, out, errout := q1Accept(b, owners); code != 0 {
		t.Fatalf("first act %d %s %s", code, out, errout)
	}
	q := q1Ask(t, b, owners, "Accept", "Accept again?")
	channel.WalkOpenQuestions(b.root())
	current, _ := channel.ReadQuestion(b.root(), q.ID)
	if current.Answer != nil {
		t.Fatal("old act answered a newer question")
	}
	work := func(open []goal.OpenQuestion) goal.ClaimableBudgetedWork {
		w := goal.ClaimableBudgetedWork{Claimed: []string{bedGoal}, GoalFacts: map[string]goal.GoalFacts{bedGoal: {Id: bedGoal}}, Eligibility: map[string]goal.ClaimEligibility{bedGoal: {Ready: true}}}
		w.MarkAskedOpen(open)
		return w
	}
	now, _ := b.commandNow(b.root())
	before := steward.SeatWorldFrom(work([]goal.OpenQuestion{{ID: q.ID, Goal: bedGoal}}), nil, goal.GateSettings{}, nil, now)
	if before.Held[0].StepDue {
		t.Fatal("fixture was not held")
	}
	if code, out, errout := q1Accept(b, owners); code != 0 {
		t.Fatalf("repeat act %d %s %s", code, out, errout)
	}
	after := steward.SeatWorldFrom(work(channel.GoalOpenQuestions(b.root())), nil, goal.GateSettings{}, nil, now)
	if !after.Held[0].StepDue {
		t.Fatalf("steward still held: %+v", after)
	}
	for _, verb := range []string{"show", "wait"} {
		code, out, errout := b.run(owners, "question", verb, "channel:"+q.ID)
		if code != 0 || !strings.Contains(out, "Accept") || !strings.Contains(out, "Wido") {
			t.Fatalf("%s %d %s %s", verb, code, out, errout)
		}
	}
	current, _ = channel.ReadQuestion(b.root(), q.ID)
	if current.Answer == nil || current.Answer.Act == nil || current.Answer.Act.Person != "Wido" {
		t.Fatalf("evidence %+v", current)
	}
	if code, out, errout := q1Accept(b, owners); code != 0 || strings.Contains(out, "Question ") {
		t.Fatalf("repeated close %d %s %s", code, out, errout)
	}
}

func TestQ1DeclarationRefusalsAndAliases(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, act string
		valid     bool
	}{
		{"missing target", "metasystem goal accept-risk", false},
		{"wrong goal", "metasystem goal accept-risk another --finding S-1 --review critic-r2", false},
		{"wrong finding", "metasystem goal accept-risk " + bedGoal + " --finding absent --review critic-r2", false},
		{"unclosed quote", "metasystem goal accept-risk " + bedGoal + " --finding S-1 --review 'critic-r2", false},
		{"unknown verb", "metasystem goal fly " + bedGoal, false},
		{"withdraw", "metasystem question withdraw q", false},
		{"extra target", "metasystem goal accept-risk " + bedGoal + " other --finding S-1 --review critic-r2", false},
		{"human flag", "metasystem goal accept-risk " + bedGoal + " --finding S-1 --review critic-r2 --by Wido", false},
		{"no review to infer", "metasystem goal accept-risk " + bedGoal + " --finding S-1", false},
		{"alias", "metasystem goal accept-risk " + bedGoal + " --finding S-1 --chain critic-r2 --reason words --json", true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b, owners := newQ1CommandBed(t)
			code, out, errout := b.run(owners, "question", "ask", bedGoal, "--question", "Choose?", "--option", "Accept: accept", "--option-act", "Accept: "+row.act)
			qs, _ := channel.WalkQuestions(b.root())
			if (code == 0) != row.valid || (len(qs) > 0) != row.valid {
				t.Fatalf("declaration %d %s %s questions=%d", code, out, errout, len(qs))
			}
		})
	}
	t.Run("duplicate acts", func(t *testing.T) {
		b, owners := newQ1CommandBed(t)
		command := "metasystem goal accept-risk " + bedGoal + " --finding S-1 --review critic-r2"
		code, _, _ := b.run(owners, "question", "ask", bedGoal, "--question", "Choose?", "--option", "A: accept", "--option", "B: accept too", "--option-act", "A: "+command, "--option-act", "B: "+command)
		qs, _ := channel.WalkQuestions(b.root())
		if code == 0 || len(qs) != 0 {
			t.Fatal("duplicate declarations accepted")
		}
	})
}

func TestQ1AgentAndFailedActsLeaveQuestionOpen(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"agent", "unenrolled", "wrong name", "failed", "partial"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, owners := newQ1CommandBed(t)
			q := q1Ask(t, b, owners, "Accept", "Choose?")
			switch name {
			case "agent", "unenrolled":
				owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, errors.New(name + " has no authority")
				}
			case "wrong name":
				writeFixtureEnrollment(t, b.root(), "Another")
			case "failed":
				code, _, _ := b.run(owners, "goal", "accept-risk", bedGoal, "--finding", "missing", "--review", "critic-r2", "--reason", "no")
				if code == 0 {
					t.Fatal("missing finding accepted")
				}
			case "partial":
				// Fail the counselor append after the goal decision lands.
				path := filepath.Join(b.root(), "records", "counselor", "accepted-risk-register.jsonl")
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatal(err)
				}
				code, result := b.runJSON(owners, "goal", "accept-risk", bedGoal, "--finding", "S-1", "--review", "critic-r2", "--reason", "bounded exposure", "--by", "Wido")
				if code == 0 || result.Outcome != intentPartial {
					t.Fatalf("partial act %d %+v", code, result)
				}

			}
			if name != "failed" && name != "partial" {
				if code, _, _ := q1Accept(b, owners); code == 0 {
					t.Fatal("unproven act accepted")
				}
			}
			got, _ := channel.ReadQuestion(b.root(), q.ID)
			if got.Answer != nil || got.State != "open" {
				t.Fatalf("question changed %+v", got)
			}
		})
	}
}

func TestQ1RunStopAnswersItsExactQuestion(t *testing.T) {
	t.Parallel()
	for _, trigger := range []string{"damaged question", "busy channel", "publication failure"} {
		t.Run(trigger, func(t *testing.T) {
			b := newWorkBed(t)
			interrupted := new(int)
			var run string
			b.workOwnersHook = func(owners *intentWorkOwners) {
				units := owners.units
				owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
					r := units(layout)
					r.AfterWrite = func(record launch.UnitRunRecord) error {
						if len(record.Rounds) > 0 && record.Rounds[0].Steps[0].State == launch.StepStarting {
							run = record.ID
							panic(interrupted)
						}
						return nil
					}
					return r
				}
			}
			func() {
				defer func() {
					if p := recover(); p != interrupted {
						t.Fatalf("interruption %v", p)
					}
				}()
				treeBuild(t, b, "q1", false)
			}()
			b.workOwnersHook = nil
			owners := b.workOwners()
			owners.processes = defaultProcessIntentOwners()
			owners.processes.process.repositoryTop = fakeTop(b.root())
			now, _ := b.commandNow(b.root())
			owners.prove = enrolledPersonProver(t, b.root(), now)
			q, err := channel.Ask(channel.AskRequest{RepoRoot: b.root(), Goal: b.id, Kind: "other", Machine: "asking", Now: now, Facts: []string{"Cancel?"}, Options: []channel.Option{{Label: "Cancel", Consequence: "stop this run", NamedAct: &channel.NamedAct{Verb: "work stop", Goal: b.id, Target: run}}}})
			if err != nil {
				t.Fatal(err)
			}
			// An unrelated damaged question must not turn this successful stop partial.
			damaged := filepath.Join(b.root(), "artifacts", "agents", "channel", "questions", "damaged.json")
			if err := os.WriteFile(damaged, []byte("{"), 0600); err != nil {
				t.Fatal(err)
			}
			release := func() {}
			endpoint := owners.dependencies.endpoint
			if trigger == "busy channel" {
				held, err := lock.File(filepath.Join(b.root(), "artifacts", "agents", "channel", "lock"), 0644, lock.TryExclusive)
				if err != nil {
					t.Fatal(err)
				}
				release = func() { held.Release() }
				defer release()
			}
			if trigger == "publication failure" {
				owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
					ep, err := endpoint(root)
					ep.Repository = questionAnswerCaptureFailure{ep.Repository}
					return ep, err
				}
			}
			code, result := b.runJSON(owners, "work", "stop", "run:"+run)
			if code != 0 {
				t.Fatalf("stop %d %+v", code, result)
			}
			if result.Outcome != intentConfirmed {
				t.Fatalf("stop outcome changed: %+v", result)
			}
			if trigger != "damaged question" {
				if !strings.Contains(strings.Join(result.Details, " "), "Question "+q.ID+" stays open") {
					t.Fatalf("missing failed-close notice: %+v", result)
				}
				got, err := channel.ReadQuestion(b.root(), q.ID)
				if err != nil || got.State != "open" {
					t.Fatal("failed close ended question")
				}
				release()
				owners.dependencies.endpoint = endpoint
				code, result = b.runJSON(owners, "work", "stop", "run:"+run)
				if code != 0 || result.Outcome != intentConfirmed {
					t.Fatalf("repeat %d %+v", code, result)
				}
			}
			got, err := channel.ReadQuestion(b.root(), q.ID)
			if err != nil || got.Answer == nil || got.Answer.Text != "Cancel" {
				t.Fatalf("answer %+v %v", got, err)
			}
			if err := os.Remove(damaged); err != nil {
				t.Fatal(err)
			}
			// The terminal result is immediately visible to the waiting public command.
			code, result = b.runJSON(owners, "question", "wait", "channel:"+q.ID)
			if code != 0 || !strings.Contains(result.Summary, "Cancel") {
				t.Fatalf("wait %d %+v", code, result)
			}
		})
	}
}

func TestQ1QuestionReadsDoNotRepairAMissedClose(t *testing.T) {
	t.Parallel()
	b, owners := newQ1CommandBed(t)
	q := q1Ask(t, b, owners, "Accept", "Choose after restart?")
	// Act succeeds without the named close, as when the process exits in between.
	actOwners := owners
	actOwners.prove = fixedFixtureGoalAuthority
	if code, out, errout := q1Accept(b, actOwners); code != 0 {
		t.Fatalf("act %d %s %s", code, out, errout)
	}
	for range 2 {
		channel.WalkOpenQuestions(b.root())
		if code, out, errout := b.run(owners, "question", "show", q.ID); code != 0 {
			t.Fatalf("show %d %s %s", code, out, errout)
		}
	}
	got, err := channel.ReadQuestion(b.root(), q.ID)
	if err != nil || got.Answer != nil || got.State != "open" {
		t.Fatalf("read repaired missed close %+v %v", got, err)
	}
	if code, out, errout := q1Accept(b, owners); code != 0 {
		t.Fatalf("repeat %d %s %s", code, out, errout)
	}
	code, result := b.runJSON(owners, "question", "wait", "channel:"+q.ID)
	if code != 0 || !strings.Contains(result.Summary, "Accept") {
		t.Fatalf("wait %d %+v", code, result)
	}
}
