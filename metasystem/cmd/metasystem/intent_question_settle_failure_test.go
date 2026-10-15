package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testchannel"
)

func TestIntentQuestionSettlementIsAdvisory(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"fetch", "timeout", "unknown", "unconfigured", "busy"} {
		t.Run(cause, func(t *testing.T) {
			t.Parallel()
			for _, verb := range []string{"list", "show", "mission-answer", "failed-resume", "channel-answer"} {
				t.Run(verb, func(t *testing.T) {
					t.Parallel()
					b := newProcessBed(t)
					replies := testchannel.New(t, b.root())
					askPath := parkedHostFailureMission(t, b.root())
					owners := b.owners()
					owners.processes.question = channel.ReadQuestion
					owners.processes.mission = func(_ string, _ stateroot.Installation, id string) (*missionrunner.Engine, error) {
						engine := missionrunner.NewEngineAt(b.root(), b.root(), id)
						engine.AnchorEffect = func(string, string, string) error { return nil }
						return engine, nil
					}
					owners.dependencies.endpoint = replies.Resolve
					warning := ""
					raw := ""
					switch cause {
					case "fetch":
						raw = "ledger remote unreachable: inbox record /private/asking-seat"
						warning = "the reply service cannot be reached"
					case "timeout":
						testchannel.Save(t, filepath.Join(b.root(), "metasystem.conf"), []byte("channel.destination.fleet.adapter=fake\nchannel.poll-timeout-sec=bad\n"))
						warning = "the reply check timeout setting is invalid"
					case "unknown":
						raw = "adapter failure at /private/seat/inbox record"
						warning = "an unexpected error (details with --verbose)"
					case "unconfigured":
						testchannel.Save(t, filepath.Join(b.root(), "metasystem.conf"), []byte(""))
					case "busy":
						held, err := lock.File(filepath.Join(b.root(), "artifacts/agents/channel/lock"), 0600, lock.TryExclusive)
						if err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { held.Release() })
					}
					if cause == "fetch" {
						owners.dependencies.endpoint = func(string) (goal.Endpoint, error) {
							ep := replies.Endpoint
							ep.Repository = replyFetchFailure{Repository: ep.Repository, err: errors.New(raw)}
							return ep, nil
						}
					} else if raw != "" {
						owners.dependencies.endpoint = func(string) (goal.Endpoint, error) { return goal.Endpoint{}, errors.New(raw) }
					}
					resumes := 0
					owners.delivery = processBackedDelivery(&intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil }, process: func(p intentProcess) intentProcessResult {
						resumes++
						if !slices.Equal(p.argv[1:4], []string{"internal", "mission", "resume"}) {
							t.Fatalf("wrong resume: %v", p.argv)
						}
						if verb == "failed-resume" {
							return intentProcessResult{code: 1, stderr: []byte("resume unavailable")}
						}
						return intentProcessResult{stdout: []byte(`{"outcome":"resumed"}`)}
					}})
					args := []string{"question", "list"}
					wantCode := 0
					wantText := "Proceed?"
					switch verb {
					case "show":
						args = []string{"question", "show", replies.Question.ID}
					case "mission-answer", "failed-resume":
						args = []string{"question", "answer", "demo/host-down", "retry: the host is back"}
						wantText = "resumed"
						if verb == "failed-resume" {
							wantCode = 1
							wantText = "did not resume"
						}
					case "channel-answer":
						args = []string{"question", "answer", replies.Question.ID, "yes"}
						wantCode = 1
						wantText = "nothing was recorded here"
					}
					code, out, errout := b.run(owners, args...)
					text := strings.Join(strings.Fields(out+errout), " ")
					if code != wantCode || !strings.Contains(text, wantText) {
						t.Fatalf("exit %d want %d: %s", code, wantCode, text)
					}
					warnings := strings.Count(text, "Replies could not be checked just now:")
					if warning == "" && warnings != 0 || warning != "" && (warnings != 1 || !strings.Contains(text, warning)) {
						t.Fatalf("warnings=%d: %s", warnings, text)
					}

					if warning != "" {
						for _, line := range strings.Split(out+errout, "\n") {
							if strings.Contains(line, "Replies could not be checked just now:") && (!strings.Contains(line, warning) || !strings.HasSuffix(line, "questions are as last known.")) {
								t.Fatalf("warning is not one plain line: %q", line)
							}
						}
					}
					for _, hidden := range []string{"ledger", "inbox record", "adapter failure", "/private/", "asking-seat"} {
						if strings.Contains(text, hidden) {
							t.Fatalf("raw detail %q: %s", hidden, text)
						}
					}
					missionAnswered := verb == "mission-answer" || verb == "failed-resume"
					if missionAskAnswered(askPath) != missionAnswered || resumes != map[bool]int{true: 1, false: 0}[missionAnswered] {
						t.Fatalf("answer recorded=%v resumes=%d", missionAskAnswered(askPath), resumes)
					}
					q, err := channel.ReadQuestion(b.root(), replies.Question.ID)
					if err != nil || q.Answer != nil || q.State != "open" {
						t.Fatalf("terminal changed channel answer: %+v %v", q, err)
					}
				})
			}
		})
	}
}

func TestIntentQuestionSettlementDetailsStayVerbose(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"--verbose", "--json"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			b := newIntentBed(t, false, nil)
			testchannel.New(t, b.root())
			owners := b.owners()
			raw := "unexpected adapter /private/inbox record on asking-seat"
			owners.dependencies.endpoint = func(string) (goal.Endpoint, error) { return goal.Endpoint{}, errors.New(raw) }
			code, out, errout := b.run(owners, "question", "list", format)
			if code != 0 || !strings.Contains(out+errout, raw) {
				t.Fatalf("exit %d: %s %s", code, out, errout)
			}
		})
	}
}

func pendingReply(t *testing.T, r *testchannel.Replies, id, goalID string) channel.Question {
	t.Helper()
	q := r.Question
	q.ID, q.Goal = id, goalID
	q.Thread = &channel.MessageRef{ID: id, ThreadID: id}
	q.Facts = []string{"Should " + id + " proceed?"}
	ulid, err := goal.NewOperationULID()
	if err != nil {
		t.Fatal(err)
	}
	q.State = "answered"
	q.Answer = &channel.Answer{Phase: "matched", Text: "yes", ULID: ulid, Opid: goal.Opid(ulid, "original", "session"), Machine: "original", Lineage: "session", Provider: "fake", UserID: "person", Ref: channel.MessageRef{ID: "reply-" + id, ThreadID: id}, At: r.Sent, Step: 42}
	r.SaveQuestion(t, q)
	return q
}

func freshReply(t *testing.T, r *testchannel.Replies, id, goalID string) channel.Question {
	t.Helper()
	q := r.Question
	q.ID, q.Goal = id, goalID
	q.Thread = &channel.MessageRef{ID: id, ThreadID: id}
	q.Facts = []string{"Should " + id + " proceed?"}
	r.SaveQuestion(t, q)
	r.CommitReply(t, q)
	return q
}

func closedReplyGoal(t *testing.T, r *testchannel.Replies) {
	t.Helper()
	ulid, err := goal.NewOperationULID()
	if err != nil {
		t.Fatal(err)
	}
	result, err := goal.Publish(r.Endpoint, goal.PublishRequest{Opid: goal.Opid(ulid, "fixture", "seed"), Machine: "fixture", Lineage: "seed", Intent: goal.Intent{Verb: "seed"}, Message: "closed goal fixture", Mutate: func(string) ([]goal.Change, error) {
		f := &goal.GoalFile{Id: "closed-goal", State: goal.StateDone, Tier: 1, Intent: "Already finished.", Origin: "main", Conclude: "Finished.", Revision: 1, OpenedAt: r.Sent.Format(time.RFC3339), History: []goal.HistoryLine{{At: r.Sent.Format(time.RFC3339), Opid: goal.Opid(ulid, "fixture", "seed"), Verb: "done", Actor: "fixture+seed", Targets: []string{"closed-goal"}, Keep: -1}}}
		return []goal.Change{{Path: "records/goals/closed-goal.md", Content: goal.RenderFile(f)}}, nil
	}})
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("closed goal: %+v %v", result, err)
	}
}

func assertReplyPublications(t *testing.T, r *testchannel.Replies, completed ...channel.Question) {
	t.Helper()
	tip, err := r.Endpoint.Repository.Capture("assert-publications")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Endpoint.Repository.Release("assert-publications")
	files, err := r.Endpoint.Repository.Files(tip, "plans/goals/")
	if err != nil {
		t.Fatal(err)
	}
	f, problems := goal.ParseFile(files["plans/goals/reply-goal.md"])
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	counts := map[string]int{}
	for _, h := range f.History {
		if h.Verb == "answer" {
			counts[h.Opid]++
		}
	}
	if len(counts) != len(completed) {
		t.Fatalf("published=%v want %d", counts, len(completed))
	}
	for _, before := range completed {
		q, err := channel.ReadQuestion(r.Root, before.ID)
		if err != nil || q.Answer == nil || q.Answer.Phase != "closed" || q.State != "closed" {
			t.Fatalf("unfinished success: %+v %v", q, err)
		}
		if counts[q.Answer.Opid] != 1 || strings.Count(f.NextStep, "ANSWERED "+q.ID) != 1 {
			t.Fatalf("answer not exactly once: %+v counts=%v next=%s", q, counts, f.NextStep)
		}
		if before.Answer != nil && before.Answer.Opid != q.Answer.Opid {
			t.Fatalf("retry identity changed: %+v %+v", before, q)
		}
	}
}

func TestIntentQuestionSettlementRecoveryIsolation(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	r := testchannel.New(t, b.root())
	closedReplyGoal(t, r)
	a := pendingReply(t, r, "a", "closed-goal")
	bq := pendingReply(t, r, "b", r.Question.Goal)
	c := freshReply(t, r, "c", r.Question.Goal)
	owners := b.owners()
	owners.dependencies.endpoint = r.Resolve
	for i := 0; i < 2; i++ {
		code, out, errout := b.run(owners, "question", "list")
		text := strings.Join(strings.Fields(out+errout), " ")
		if code != 0 || strings.Count(text, "Replies could not be checked just now:") != 1 || strings.Count(text, a.Facts[0]) != 2 || !strings.Contains(text, "the goal is no longer open") {
			t.Fatalf("call %d exit %d: %s", i, code, text)
		}
		got, err := channel.ReadQuestion(r.Root, a.ID)
		if err != nil || !reflect.DeepEqual(got, a) {
			t.Fatalf("failed answer changed: %+v want %+v err=%v", got, a, err)
		}
		assertReplyPublications(t, r, bq, c)
	}
}

func TestIntentQuestionSettlementFreshIsolation(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	r := testchannel.New(t, b.root())
	d := freshReply(t, r, "d", r.Question.Goal)
	e := freshReply(t, r, "e", r.Question.Goal)
	// Only D's publication fails; reading the shared committed replies still works.
	failing := true
	owners := b.owners()
	owners.dependencies.endpoint = func(string) (goal.Endpoint, error) {
		ep := r.Endpoint
		ep.Repository = &replyPublicationFailure{Repository: ep.Repository, fail: &failing}
		return ep, nil
	}
	code, out, errout := b.run(owners, "question", "list")
	if code != 0 || !strings.Contains(strings.Join(strings.Fields(out+errout), " "), d.Facts[0]+`": an unexpected error`) {
		t.Fatalf("exit %d: %s %s", code, out, errout)
	}
	before, err := channel.ReadQuestion(r.Root, d.ID)
	if err != nil || before.Answer == nil || before.Answer.Phase != "matched" {
		t.Fatalf("pending: %+v %v", before, err)
	}
	assertReplyPublications(t, r, e)
	failing = false
	code, out, errout = b.run(owners, "question", "list")
	if code != 0 || strings.Contains(out+errout, "Replies could not") {
		t.Fatalf("retry exit %d: %s %s", code, out, errout)
	}
	assertReplyPublications(t, r, before, e)
}

type replyPublicationFailure struct {
	goal.Repository
	fail *bool
}

func (r *replyPublicationFailure) Build(opid, parent string, changes []goal.Change, message string) (string, error) {
	for _, c := range changes {
		if *r.fail && strings.Contains(string(c.Content), "ANSWERED d:") {
			return "", errors.New("adapter publication /private/inbox record failed")
		}
	}
	return r.Repository.Build(opid, parent, changes, message)
}

func TestIntentQuestionSettlementAllowance(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"recovery-failures", "fresh-failure"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newIntentBed(t, false, nil)
			r := testchannel.New(t, b.root())
			closedReplyGoal(t, r)
			var failures, successes []channel.Question
			if mode == "recovery-failures" {
				for i := 0; i < 5; i++ {
					failures = append(failures, pendingReply(t, r, fmt.Sprintf("a%d", i), "closed-goal"))
				}
				successes = append(successes, freshReply(t, r, "z", r.Question.Goal))
			} else {
				failures = append(failures, freshReply(t, r, "a", "closed-goal"))
				for i := 0; i < 6; i++ {
					q := freshReply(t, r, fmt.Sprintf("z%d", i), r.Question.Goal)
					if i < 5 {
						successes = append(successes, q)
					}
				}
			}
			owners := b.owners()
			owners.dependencies.endpoint = r.Resolve
			code, out, errout := b.run(owners, "question", "list")
			text := strings.Join(strings.Fields(out+errout), " ")
			if code != 0 || strings.Count(text, "Replies could not be checked just now:") != 1 {
				t.Fatalf("exit %d: %s", code, text)
			}
			warning := text[strings.Index(text, "Replies could not"):]
			for _, q := range failures {
				if strings.Count(warning, q.Facts[0]) != 1 {
					t.Fatalf("lost failure %s: %s", q.ID, warning)
				}
				got, err := channel.ReadQuestion(r.Root, q.ID)
				if err != nil || got.Answer == nil || got.Answer.Phase != "matched" || got.Answer.Opid == "" || got.Answer.Text != "yes" {
					t.Fatalf("lost retry: %+v %v", got, err)
				}
				if q.Answer != nil && !reflect.DeepEqual(got.Answer, q.Answer) {
					t.Fatalf("retry changed: %+v %+v", got, q)
				}
			}
			assertReplyPublications(t, r, successes...)
			if mode == "fresh-failure" {
				q, err := channel.ReadQuestion(r.Root, "z5")
				if err != nil || q.Answer != nil {
					t.Fatalf("allowance not respected: %+v %v", q, err)
				}
			}
		})
	}
}

type replyFetchFailure struct {
	goal.Repository
	err error
}

func (r replyFetchFailure) Capture(string) (string, error) { return "", r.err }

func TestIntentQuestionSettlementPreservesMissionOutcome(t *testing.T) {
	t.Parallel()
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			t.Parallel()
			b := newProcessBed(t)
			testchannel.New(t, b.root())
			askPath := parkedHostFailureMission(t, b.root())
			owners := b.owners()
			owners.dependencies.endpoint = func(string) (goal.Endpoint, error) { return goal.Endpoint{}, errors.New("settlement unavailable") }
			owners.processes.mission = func(_ string, _ stateroot.Installation, id string) (*missionrunner.Engine, error) {
				engine := missionrunner.NewEngineAt(b.root(), b.root(), id)
				engine.AnchorEffect = func(string, string, string) error { return nil }
				return engine, nil
			}
			resumes := 0
			owners.delivery = processBackedDelivery(&intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil }, process: func(intentProcess) intentProcessResult {
				resumes++
				if failed {
					return intentProcessResult{code: 1, stderr: []byte("resume unavailable")}
				}
				return intentProcessResult{stdout: []byte(`{"outcome":"resumed"}`)}
			}})
			code, result := b.runJSON(owners, "question", "answer", "demo/host-down", "retry: the host is back")
			wantOutcome, wantCode := intentConfirmed, 0
			if failed {
				wantOutcome, wantCode = intentPartial, 1
			}
			if code != wantCode || result.Outcome != wantOutcome || !missionAskAnswered(askPath) || resumes != 1 || !strings.Contains(strings.Join(result.Details, " "), "settlement unavailable") {
				t.Fatalf("exit %d result=%+v recorded=%v resumes=%d", code, result, missionAskAnswered(askPath), resumes)
			}
		})
	}
}

func TestIntentQuestionSettlementPublicationFetchFailure(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	replies := testchannel.New(t, b.root())
	q := pendingReply(t, replies, "a", replies.Question.Goal)
	failing := true
	owners := b.owners()
	owners.dependencies.endpoint = func(string) (goal.Endpoint, error) {
		ep := replies.Endpoint
		ep.Repository = replyPublicationFetchFailure{Repository: ep.Repository, failing: &failing}
		return ep, nil
	}
	code, out, errout := b.run(owners, "question", "list")
	text := out + errout
	if code != 0 || strings.Count(text, "Replies could not be checked just now:") != 1 || !strings.Contains(text, `"Should a proceed?": the reply service cannot be reached`) {
		t.Fatalf("exit %d: %s", code, text)
	}
	if strings.Contains(text, "private publication") || strings.Contains(text, "remote ledger") {
		t.Fatalf("raw failure leaked: %s", text)
	}
	got, err := channel.ReadQuestion(b.root(), q.ID)
	if err != nil || !reflect.DeepEqual(got, q) {
		t.Fatalf("failed publication changed answer: %+v %v", got, err)
	}
	failing = false
	code, out, errout = b.run(owners, "question", "list")
	if code != 0 || strings.Contains(out+errout, "Replies could not") {
		t.Fatalf("retry exit %d: %s %s", code, out, errout)
	}
	assertReplyPublications(t, replies, q)
}

type replyPublicationFetchFailure struct {
	goal.Repository
	failing *bool
}

func (r replyPublicationFetchFailure) Capture(opid string) (string, error) {
	if *r.failing && !strings.HasPrefix(opid, "channel-match-") {
		return "", errors.New("private publication remote unreachable")
	}
	return r.Repository.Capture(opid)
}
