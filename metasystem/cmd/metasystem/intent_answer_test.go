package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestUnitStopQuestionsOfferSolutions(t *testing.T) {
	t.Parallel()
	t.Run("a stopped build", func(t *testing.T) {
		t.Parallel()
		b := failedBuildQuestionBed(t)
		enrol(t, b, "Wido")
		q, r := partialQuestion(t, b)
		round := r.Rounds[0]
		act := withoutOption(shellWords(q.UnitStop.Needs), "by")
		if round.Cause == "environment" || round.Cause == "deadline" || !q.Contract || q.Recommendation != "Continue" || len(q.Options) != 2 {
			t.Fatalf("the correction does not offer solutions: cause=%s %+v", round.Cause, q)
		}
		decision := fmt.Sprintf("%s: the build of unit outcome stopped at %s (%s).", strings.ReplaceAll(b.id, "-", " "), q.OpenedAt.Local().Format("15:04"), round.Stop.Class)
		if q.Plain.Decision != decision || q.Plain.IfNothing != "it waits; nothing builds or lands until you choose." {
			t.Fatalf("words = %+v, want %q", q.Plain, decision)
		}
		continueOption, stop := q.Options[0], q.Options[1]
		if continueOption.Label != "Continue" || !slices.Equal(continueOption.Act, act) || !strings.HasSuffix(continueOption.Consequence, "Undo: metasystem work stop "+b.id+".") || stop.Label != "Stop here" || stop.Act != nil {
			t.Fatalf("options = %+v, want Continue with %q", q.Options, act)
		}
		if brief := act[slices.Index(act, "--brief")+1]; !regularFile(brief) || brief != filepath.Join(round.Directory, "correction-brief.md") {
			t.Fatalf("the act's brief is not the prepared one: %s", brief)
		}
		for _, word := range []string{"FILE", "TEXT", "NAME", "--by"} {
			if slices.Contains(continueOption.Act, word) {
				t.Fatalf("the act keeps %s: %q", word, continueOption.Act)
			}
		}
	})
	for _, row := range []struct{ name, diff string }{
		{"a gap", ""},
		{"a size stop", "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n-old\n+new\ndiff --git a/a_test.go b/a_test.go\n--- /dev/null\n+++ b/a_test.go\n+test\n"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := outcomeBed(t, row.diff)
			if code, result, _ := outcomeBuild(t, b); code != 1 {
				t.Fatalf("build: %d %+v", code, result)
			}
			if qs, _ := channel.WalkOpenQuestions(b.root()); len(qs) != 1 || qs[0].Contract || len(qs[0].Options) != 0 {
				t.Fatalf("%s asks differently than before: %+v", row.name, qs)
			}
		})
	}
	t.Run("a check deadline", func(t *testing.T) {
		t.Parallel()
		b, _, _ := deadlineCheckBed(t, 4, everyCheckDeadline)
		agentBuild(t, b)
		if qs, _ := channel.WalkOpenQuestions(b.root()); len(qs) != 1 || qs[0].Contract || !slices.Contains(qs[0].UnitStop.AcceptableActs, "work-review-retry") {
			t.Fatalf("the retry asks differently than before: %+v", qs)
		}
	})
}

const answerUser = "UWIDO"

// channelFor configures the bed's channel for the person whose user id is
// answerUser, with the one-time code off.
func channelFor(t *testing.T, root string) {
	t.Helper()
	conf := filepath.Join(root, "metasystem.conf")
	data, _ := os.ReadFile(conf)
	writeTestingFixtureFile(t, conf, append(data, []byte("\nchannel.destination.fleet.adapter=slack\nchannel.destination.fleet.slack.channel-id=CTEST\nchannel.human.slack.user-id="+answerUser+"\nchannel.human.answer-code=off\n")...), 0o644)
	writeTestingFixtureFile(t, conf+".local", []byte("channel.destination.fleet.slack.bot-token=xoxb-test\n"), 0o600)
}

// recordChoice is the person's verified reply recorded on q, as the poll
// records it before it runs the act.
func recordChoice(t *testing.T, root string, q channel.Question, amend func(*channel.Question)) channel.Question {
	t.Helper()
	q.Thread = &channel.MessageRef{ID: "m1", ThreadID: "m1"}
	q.Answer = &channel.Answer{Provider: "slack", Text: "continue", UserID: answerUser, Ref: channel.MessageRef{ID: "m2", ThreadID: "m1"}, At: q.OpenedAt, Step: q.OpenedAt.Unix(), ULID: "01J5X0000000000000000000A1", Phase: "recorded"}
	if amend != nil {
		amend(&q)
	}
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "artifacts", "agents", "channel", "questions", q.ID+".json"), data, 0o600)
	return q
}

// answerAct runs argv as the executor does, with --answer, from a process
// that is not the person's terminal.
func answerAct(t *testing.T, b *workBed, argv []string, id string) (int, intentResult) {
	t.Helper()
	owners := b.workOwners()
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("an agent process is an ancestor of this shell")
	}
	command, rest, ok := resolveIntentArgv(append(slices.Clone(argv[1:]), "--answer", id))
	if !ok {
		t.Fatalf("no public command %q", argv)
	}
	var out, stderr bytes.Buffer
	code := runIntentIn(command, append(rest, "--json"), &out, &stderr, b.root(), owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("%v printed no JSON result: %v; %s %s", argv, err, out.String(), stderr.String())
	}
	return code, result
}

func answerState(t *testing.T, result intentResult) (string, string) {
	t.Helper()
	data, _ := result.Data.(map[string]any)
	answer, _ := data["answer"].(map[string]any)
	state, _ := answer["state"].(string)
	hold, _ := answer["hold"].(string)
	return state, hold
}

// answeredCorrection is a stopped build's correction question with the
// person's Continue recorded on it.
func answeredCorrection(t *testing.T, b *workBed, amend func(*channel.Question)) (channel.Question, []string) {
	t.Helper()
	enrol(t, b, "Wido")
	channelFor(t, b.root())
	q, _ := partialQuestion(t, b)
	if !q.Contract {
		t.Fatalf("the correction is no contract question: %+v", q)
	}
	return recordChoice(t, b.root(), q, amend), q.Options[0].Act
}

func TestChannelAnswerBindsExactlyTheChosenAct(t *testing.T) {
	t.Parallel()
	other := func(act []string, flag, value string) []string {
		changed := slices.Clone(act)
		changed[slices.Index(changed, flag)+1] = value
		return changed
	}
	for _, row := range []struct {
		name  string
		amend func(*channel.Question)
		argv  func(act []string, b *workBed) []string
		id    string
		want  string
		setup func(*testing.T, *workBed)
	}{
		{name: "another user", amend: func(q *channel.Question) { q.Answer.UserID = "USOMEONE" }},
		{name: "nobody enrolled", setup: unenrol, want: "nobody is enrolled on this machine"},
		{name: "an answer that does not prove", amend: func(q *channel.Question) { q.Answer.Provider = "" }, want: "could not be proved as the person's word"},
		{name: "an answer not yet recorded", amend: func(q *channel.Question) { q.Answer.Phase = "matched" }},
		{name: "a closed question", amend: func(q *channel.Question) { q.State = "closed" }},
		{name: "a question outside the contract", amend: func(q *channel.Question) { q.Contract = false }},
		{name: "an id with a path's shape", id: "../questions/x"},
		{name: "an id holding L", id: "01J5X000000000000000000L00"},
		{name: "another brief", argv: func(act []string, b *workBed) []string {
			return other(act, "--brief", filepath.Join(b.root(), b.brief("other.md", "Correct something else.\n")))
		}},
		{name: "another attempt", argv: func(act []string, _ *workBed) []string { return other(act, "--after", "2") }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := failedBuildQuestionBed(t)
			q, act := answeredCorrection(t, b, row.amend)
			argv, id := act, q.ID
			if row.argv != nil {
				argv = row.argv(act, b)
			}
			if row.id != "" {
				id = row.id
			}
			if row.setup != nil {
				row.setup(t, b)
			}
			code, refused := answerAct(t, b, argv, id)
			r := onlyRun(t, b)
			if code == 0 || refused.Outcome != intentRefused || !strings.Contains(refused.Summary, "nothing was done") || !strings.Contains(refused.Summary, row.want) || len(r.Revisions) != 0 || len(r.Rounds) != 1 {
				t.Fatalf("a foreign answer was admitted: %d %+v revisions=%+v", code, refused, r.Revisions)
			}
			if state, _ := answerState(t, refused); state != "refused" {
				t.Fatalf("the refused act reads %q", state)
			}
		})
	}
	t.Run("the chosen act", func(t *testing.T) {
		t.Parallel()
		b := failedBuildQuestionBed(t)
		q, act := answeredCorrection(t, b, nil)
		_, result := answerAct(t, b, act, q.ID)
		r := onlyRun(t, b)
		closed, _ := channel.ReadQuestion(b.root(), q.ID)
		if state, _ := answerState(t, result); state != "started" || len(r.Revisions) != 1 || r.Revisions[0].Person != "Wido" || len(r.Rounds) != 2 || closed.State != "closed" {
			t.Fatalf("the chosen act did not start as the person: %+v run=%+v question=%s", result, r.Revisions, closed.State)
		}
	})
}

// budgetedPartialBed is the stopped-build bed whose goal allows attempts.
func budgetedPartialBed(t *testing.T, attempts uint64) *workBed {
	t.Helper()
	b := newWorkBedWith(t, func(file *goal.GoalFile) {
		file.Budget = &goal.Budget{ElapsedLimit: "4h", AttemptLimit: attempts, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}
		workApprovedBox(file)
	})
	b.workOwnersHook = func(o *intentWorkOwners) {
		units := o.units
		o.units = func(layout stateroot.Layout) *launch.UnitRunner {
			r := units(layout)
			r.Git = outcomeGit{workGit{b}, "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n+line 0\n+line 1\n+line 2\n"}
			return r
		}
	}
	b.manager.Settings.BuildLinesCap = 1500
	b.manager.Supervisor = partialStarter{bed: b, message: partialMessage}
	return b
}

func TestChannelAnswerActReadsItsOwnRecord(t *testing.T) {
	t.Parallel()
	t.Run("a loaded host", func(t *testing.T) {
		t.Parallel()
		b := failedBuildQuestionBed(t)
		q, act := answeredCorrection(t, b, nil)
		b.manager.CapacitySources.Load = loadedHost
		_, result := answerAct(t, b, act, q.ID)
		if state, hold := answerState(t, result); state != "started" || len(onlyRun(t, b).Rounds) != 2 {
			t.Fatalf("the person's answer was held on a loaded host: %s (%s) %+v", state, hold, result)
		}
	})
	t.Run("a spent budget and a second correction of the same stop", func(t *testing.T) {
		t.Parallel()
		b := budgetedPartialBed(t, 2)
		q, act := answeredCorrection(t, b, nil)
		if _, first := answerAct(t, b, act, q.ID); len(onlyRun(t, b).Rounds) != 2 {
			t.Fatalf("the first correction did not start: %+v", first)
		}
		open, _ := channel.WalkOpenQuestions(b.root())
		if len(open) != 1 || !open[0].Contract || open[0].UnitStop.Attempt != 2 {
			t.Fatalf("the second stop asked no correction: %+v", open)
		}
		second := recordChoice(t, b.root(), open[0], nil)
		_, result := answerAct(t, b, second.Options[0].Act, second.ID)
		r := onlyRun(t, b)
		state, hold := answerState(t, result)
		if state != "accepted" || hold != "the goal's budget is used up" || len(r.Revisions) != 2 || r.Revisions[0].Reason != r.Revisions[1].Reason || slices.Contains([]launch.UnitStepState{launch.StepRunning, launch.StepPassed, launch.StepFailed}, r.Rounds[len(r.Rounds)-1].Steps[0].State) {
			t.Fatalf("a retained correction under a spent budget reads %s (%s): %+v revisions=%+v", state, hold, result, r.Revisions)
		}
		again := recordChoice(t, b.root(), open[0], func(q *channel.Question) { q.Answer.Ref.ID, q.Answer.ULID = "m3", "01J5X0000000000000000000A2" })
		if _, replay := answerAct(t, b, again.Options[0].Act, again.ID); len(onlyRun(t, b).Revisions) != 2 {
			t.Fatalf("a fresh reply recorded a second correction: %+v", replay)
		}
	})
}

func TestChannelAnswerPassesTheHolderGateWithGoalOwnership(t *testing.T) {
	t.Parallel()
	bed := seatGoalBed(t)
	bed.lineage = seatLaunchLineage(t)
	seat, caller := seatStandIn(t, bed.root, "seat-one", bed.lineage)
	bed.caller = caller
	gcliLedgerMust(t, bed, "goal", "claim", "fix-docs")
	channelFor(t, bed.root)
	act := []string{"metasystem", "work", "revise", "fix-docs", "--work", "u", "--after", "1", "--brief", "brief.md", "--reason", "the machinery stopped: a build step failed"}
	request := channel.AskRequest{RepoRoot: bed.root, Goal: "fix-docs", Kind: "other", Machine: "fixture-machine", Facts: []string{"the build stopped"}, Now: goalCLISeedNow, Contract: true, Recommendation: "Continue",
		Plain:   &channel.Plain{Decision: "fix docs: the build of unit u stopped", IfNothing: "it waits"},
		Options: []channel.Option{{Label: "Continue", Consequence: "one more round. Undo: metasystem work stop fix-docs.", Act: act}, {Label: "Stop here", Consequence: "nothing runs"}}}
	q, err := channel.Ask(request)
	if err != nil {
		t.Fatal(err)
	}
	recordChoice(t, bed.root, q, nil)
	endpoint, err := bed.endpoint(bed.root)
	if err != nil {
		t.Fatal(err)
	}
	machine := func(string, string) (string, error) { return "fixture-machine", nil }
	holder := func(string) string { return bed.root }
	conn := intentConnectionOwners{
		laneFix: func(string, string, string) *plain.Fix { return nil },
		claimCheck: func(root, id string, endpoint goal.Endpoint) func() error {
			return goalBranchClaimCheckWith(root, id, endpoint, machine, holder)
		},
		ownershipCheck: func(root, id string, endpoint goal.Endpoint) func() error {
			return goalBranchOwnershipCheckWith(root, id, endpoint, machine, holder)
		},
	}
	gate := func(answer bool) error {
		argv := act[1:]
		if answer {
			argv = append(slices.Clone(argv), "--answer", q.ID)
		}
		command, rest, _ := resolveIntentArgv(argv)
		input, problem := parseIntentArgs(command, rest)
		if problem != nil {
			t.Fatalf("parse %q: %+v", argv, problem)
		}
		inv := &intentInvocation{command: command, raw: rest, input: input, cwd: bed.root, owners: bed.owners(&bytes.Buffer{}, &bytes.Buffer{})}
		if answer {
			if _, _, problem := inv.actingAs("revise", "fix-docs", actorHuman); problem != nil {
				t.Fatalf("the answer was not admitted as the person's act: %+v", problem)
			}
		}
		return inv.laneClaimCheck(conn, bed.root, "fix-docs", endpoint)()
	}
	if err := gate(true); err != nil {
		t.Fatalf("the admitted answer was refused at the holder gate: %v", err)
	}
	if err := gate(false); err == nil {
		t.Fatal("a process that does not hold the checkout passed without an answer")
	}
	_ = seat.Kill()
	seatStandIn(t, bed.root, "seat-two", "person-terminal")
	if err := gate(true); err == nil {
		t.Fatal("the answer passed for a goal claimed by another lineage")
	}
}

func TestChannelAnswerActNeedsItsReason(t *testing.T) {
	t.Parallel()
	for _, form := range []string{"goal", "run"} {
		t.Run(form, func(t *testing.T) {
			t.Parallel()
			b := failedBuildQuestionBed(t)
			var argv []string
			q, _ := answeredCorrection(t, b, func(q *channel.Question) {
				argv = withoutOption(q.Options[0].Act, "reason")
				if form == "run" {
					argv = []string{"metasystem", "work", "revise", "run:" + onlyRun(t, b).ID, "--brief", argv[slices.Index(argv, "--brief")+1]}
				}
				q.Options[0].Act = argv
			})
			if slices.Contains(argv, "--reason") {
				t.Fatalf("the act still holds its reason: %q", argv)
			}
			code, refused := answerAct(t, b, argv, q.ID)
			r := onlyRun(t, b)
			if code == 0 || refused.Outcome != intentRefused || !strings.Contains(refused.Summary, "reason") || len(r.Revisions) != 0 || len(r.Rounds) != 1 {
				t.Fatalf("an answer act without its reason was not refused: %d %+v revisions=%+v", code, refused, r.Revisions)
			}
		})
	}
}
