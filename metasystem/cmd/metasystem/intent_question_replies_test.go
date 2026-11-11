package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testchannel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

func TestIntentQuestionSettlesReplies(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"list", "show", "answer"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			for _, phase := range []string{"fresh", "matched", "recorded-commit", "recorded", "closed"} {
				t.Run(phase, func(t *testing.T) {
					t.Parallel()
					b := newIntentBed(t, false, nil)
					replies := testchannel.New(t, b.root())
					replies.CommitReply(t, replies.Question)
					if phase != "fresh" {
						replies.Interrupt(t, phase)
					}
					owners := b.owners()
					owners.processes = defaultProcessIntentOwners()
					owners.dependencies.endpoint = replies.Resolve
					// Use the production identity path with no supplied owner lineage.
					owners.dependencies.ownerLineage = nil
					args := []string{"question", verb}
					if verb != "list" {
						args = append(args, replies.Question.ID)
					}
					for i := 0; i < 2; i++ {
						code, stdout, stderr := b.run(owners, args...)
						if code != 0 || !strings.Contains(strings.Join(strings.Fields(stdout), " "), "answered 'yes' at 09:13") || strings.Contains(stdout, "withdrawn") {
							t.Fatalf("%v exit %d: %s %s", args, code, stdout, stderr)
						}
					}
					replies.AssertAnswer(t)
				})
			}
		})
	}
}

func TestIntentQuestionActionReplyStaysOutstanding(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	replies := testchannel.New(t, b.root())
	// One synthetic saved stop, reconciled by the real landing request owner.
	stopPath := filepath.Join(plain.Dir(b.root()), "stops.jsonl")
	testchannel.Save(t, stopPath, []byte(`{"loop":"lane-return","subject":"lane","attempt":2,"decision":"stop","handoff":"ask lane","at":"2026-10-15T08:00:00Z","tree":"candidate","proof-attempt":"attempt"}`+"\n"))
	if err := plain.SyncPolicyQuestion(b.root(), func(string) (string, error) { return "asking", nil }, replies.Sent); err != nil {
		t.Fatal(err)
	}
	questions, unreadable := channel.WalkOpenQuestions(b.root())
	var request channel.Question
	for _, q := range questions {
		if channel.LaneStopCommand(q) != "" {
			request = q
		}
	}
	if request.ID == "" || len(unreadable) > 0 {
		t.Fatalf("landing request: %v %v", questions, unreadable)
	}
	request.Thread = &channel.MessageRef{ID: "10", ThreadID: "10"}
	replies.SaveQuestion(t, request)
	before := map[string]string{}
	_ = filepath.WalkDir(plain.Dir(b.root()), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			before[path] = string(data)
		}
		return nil
	})
	replies.CommitReply(t, request)
	owners := b.owners()
	owners.processes = defaultProcessIntentOwners()
	owners.dependencies.endpoint = replies.Resolve
	for _, verb := range []string{"list", "show"} {
		args := []string{"question", verb}
		if verb == "show" {
			args = append(args, request.ID)
		}
		code, out, errout := b.run(owners, args...)
		want := "replied 'yes' at 09:13; the act is still needed: " + request.Facts[0]
		if code != 0 || !strings.Contains(strings.Join(strings.Fields(out), " "), want) || strings.Contains(out, "lane stop:") || strings.Contains(out, "evidence:") {
			t.Fatalf("%v exit %d: %s %s; want %q", args, code, out, errout, want)
		}
	}
	got, err := channel.ReadQuestion(b.root(), request.ID)
	if err != nil || got.State != "open" || got.Answer == nil || got.Answer.Phase != "recorded" || got.Answer.Text != "yes" || got.Answer.Ref.ID != "2-10" || got.Answer.Ref.ThreadID != "10" {
		t.Fatalf("request: %+v %v", got, err)
	}
	after := map[string]string{}
	_ = filepath.WalkDir(plain.Dir(b.root()), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			after[path] = string(data)
		}
		return nil
	})
	if len(before) != len(after) {
		t.Fatalf("landing records changed: %v %v", before, after)
	}
	for path, data := range before {
		if after[path] != data {
			t.Fatalf("landing record changed: %s", path)
		}
	}
	// Five complete replies must not exhaust the allowance for a fresh reply.
	for i := 0; i < 4; i++ {
		copy := got
		copy.ID = string(rune('a'+i)) + "-action"
		replies.SaveQuestion(t, copy)
	}
	// Replace the shared message with a new committed reply to the goal question.
	replies.CommitReply(t, replies.Question)
	for i := 0; i < 3; i++ {
		result, err := channel.SettleReplies(context.Background(), replies.Config(), replies.Resolve)
		want := 0
		if i == 0 {
			want = 1
		}
		if err != nil || result.Dispositions != want {
			t.Fatalf("settlement %d: %+v %v", i, result, err)
		}
	}
	replies.AssertAnswer(t)
}

func TestIntentQuestionWithdrawnStillRendersWithdrawn(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	replies := testchannel.New(t, b.root())
	q := replies.Question
	q.State = "closed"
	replies.SaveQuestion(t, q)
	owners := b.owners()
	owners.processes = defaultProcessIntentOwners()
	owners.dependencies.endpoint = replies.Resolve
	code, out, errout := b.run(owners, "question", "show", q.ID)
	if code != 0 || !strings.Contains(out, "withdrawn") || strings.Contains(out, "answered") {
		t.Fatalf("exit %d: %s %s", code, out, errout)
	}
}

func TestIntentQuestionSettlementUsesInstallation(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	replies := testchannel.New(t, t.TempDir())
	replies.CommitReply(t, replies.Question)
	owners := b.owners()
	owners.processes = defaultProcessIntentOwners()
	owners.dependencies.endpoint = replies.Resolve
	inv := &intentInvocation{stateRoot: b.root(), owners: owners, layout: stateroot.Layout{InstallationRoot: stateroot.Installation(replies.Root)}}
	if problem := inv.settleQuestionReplies(); problem != nil {
		t.Fatalf("settlement: %+v", problem)
	}
	q, problem := inv.resolveQuestion(replies.Question.ID)
	if problem != nil {
		t.Fatalf("question read outside installation: %+v", problem)
	}
	if result := inv.questionView(q); !strings.Contains(result.Summary, "answered 'yes' at 09:13") {
		t.Fatal(result.Summary)
	}
	replies.AssertAnswer(t)
	if _, err := channel.ReadQuestion(b.root(), q.id); !os.IsNotExist(err) {
		t.Fatalf("question appeared in project state: %v", err)
	}
}

func TestAWithdrawnQuestionTellsItsAsker(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"reason", "repeat", "decision", "goal scope", "failed tell", "act questions", "recorded reply", "unknown decision"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newGrantEverythingBed(t)
			conf, err := os.ReadFile(filepath.Join(b.root(), "metasystem.conf"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			r := testchannel.New(t, b.root())
			replyConf, err := os.ReadFile(filepath.Join(b.root(), "metasystem.conf"))
			if err != nil {
				t.Fatal(err)
			}
			testchannel.Save(t, filepath.Join(b.root(), "metasystem.conf"), append(append(conf, '\n'), replyConf...))
			owners := b.owners()
			owners.processes.question = channel.ReadQuestion
			owners.processes.channelLink = func(string) (channel.Provider, channel.DestinationConfig) { return nil, channel.DestinationConfig{} }
			owners.dependencies.endpoint = r.Resolve
			home := t.TempDir()
			owners.decisions.home = func() (string, error) { return home, nil }
			owners.lookupEnv = func(string) (string, bool) { return "", false }
			q := r.Question
			now, err := b.commandNow(b.root())
			if err != nil {
				t.Fatal(err)
			}
			const reason = "decided PAGE in Wido's name:\nthe overdue-ask row is the first line of the situation page"
			folded := strings.Join(strings.Fields(reason), " ")
			read := func(id string) channel.Question {
				t.Helper()
				got, err := channel.ReadQuestion(b.root(), id)
				if err != nil {
					t.Fatal(err)
				}
				return got
			}
			ledger := func() (string, *goal.GoalFile) {
				t.Helper()
				tip, err := r.Endpoint.Repository.Capture("withdrawal-check")
				if err != nil {
					t.Fatal(err)
				}
				defer r.Endpoint.Repository.Release("withdrawal-check")
				files, err := r.Endpoint.Repository.Files(tip, "plans/goals/")
				if err != nil {
					t.Fatal(err)
				}
				f, problems := goal.ParseFile(files["plans/goals/reply-goal.md"])
				if len(problems) != 0 {
					t.Fatal(problems)
				}
				return tip, f
			}
			marker := func(id, why string) {
				t.Helper()
				_, f := ledger()
				count := 0
				for _, h := range f.History {
					if h.Verb == "withdraw-question" && h.Reason == "question "+id {
						count++
						if !slices.Equal(h.Targets, []string{q.Goal}) || h.At != now.UTC().Format(time.RFC3339) {
							t.Fatalf("history: %+v", h)
						}
					}
				}
				if count != 1 || strings.Count(f.NextStep, "WITHDRAWN "+id+": "+why) != 1 {
					t.Fatalf("next=%q history=%d", f.NextStep, count)
				}
			}
			withdraw := func(id, why, outcome string, code int) intentResult {
				t.Helper()
				gotCode, got := b.runJSON(owners, "question", "withdraw", id, "--reason", why)
				if gotCode != code || got.Outcome != outcome {
					t.Fatalf("withdraw: code=%d %+v", gotCode, got)
				}
				return got
			}
			show := func(id, summary, decision string) {
				t.Helper()
				code, got := b.runJSON(owners, "question", "show", id)
				data := got.Data.(map[string]any)
				if code != 0 || got.Summary != summary {
					t.Fatalf("show: %d %+v", code, got)
				}
				if decision == "" {
					if _, ok := data["withdrawalDecision"]; ok || got.Next != nil {
						t.Fatalf("unrecorded decision: %+v", got)
					}
				} else if data["withdrawalDecision"] != decision || got.Next == nil || !slices.Equal(got.Next.Argv, []string{"metasystem", "decision", "show", decision}) {
					t.Fatalf("decision: %+v", got)
				}
				code, out, errout := b.run(owners, "question", "show", id)
				words := strings.Join(strings.Fields(out), " ")
				if code != 0 || !strings.Contains(words, "because") || !strings.Contains(words, strings.Join(strings.Fields(read(id).Withdrawal.Reason), " ")) {
					t.Fatalf("text show: %d %s %s", code, out, errout)
				}
				if decision == "" && strings.Contains(words, "decided by ") || decision != "" && !strings.Contains(words, "by Wido in decision "+decision) {
					t.Fatalf("decided row: %s", out)
				}
			}
			switch name {
			case "reason":
				got := withdraw(q.ID, reason, intentConfirmed, 0)
				if got.Summary != "question "+q.ID+" is withdrawn" {
					t.Fatal(got.Summary)
				}
				closed := read(q.ID)
				if closed.State != "closed" || closed.ClosedBecause != "" || !reflect.DeepEqual(closed.Withdrawal, &channel.Withdrawal{Reason: reason, At: now.UTC()}) {
					t.Fatalf("record: %+v", closed)
				}
				marker(q.ID, folded)
				_, f := ledger()
				if f.NextStep != "Wait.; WITHDRAWN "+q.ID+": "+folded {
					t.Fatal(f.NextStep)
				}
				show(q.ID, "channel question "+q.ID+" is withdrawn: "+reason, "")
				_, out, _ := b.run(owners, "question", "show", q.ID)
				if !strings.Contains(out, "Channel question "+q.ID+" is withdrawn") {
					t.Fatal(out)
				}
			case "repeat":
				withdraw(q.ID, reason, intentConfirmed, 0)
				before := read(q.ID)
				_, f := ledger()
				ulid, err := goal.NewOperationULID()
				if err != nil {
					t.Fatal(err)
				}
				f.NextStep, f.Revision = "Build U2.", f.Revision+1
				f.History = append(f.History, goal.HistoryLine{At: now.UTC().Format(time.RFC3339), Opid: goal.Opid(ulid, "asking", "session"), Verb: "edit", Actor: "asking+session", Targets: []string{q.Goal}, Keep: -1})
				result, err := goal.Publish(r.Endpoint, goal.PublishRequest{Opid: goal.Opid(ulid, "asking", "session"), Machine: "asking", Lineage: "session", Intent: goal.Intent{Verb: "edit", Targets: []string{q.Goal}}, Message: "goal edit " + q.Goal, Mutate: func(string) ([]goal.Change, error) {
					return []goal.Change{{Path: "plans/goals/reply-goal.md", Content: goal.RenderFile(f)}}, nil
				}})
				if err != nil || result.Outcome != goal.OutcomeConfirmed {
					t.Fatalf("rewrite: %+v %v", result, err)
				}
				tip, _ := ledger()
				got := withdraw(q.ID, reason, intentUnchanged, 0)
				after, f := ledger()
				if got.Summary != "question "+q.ID+" is already withdrawn; nothing was changed" || after != tip || f.NextStep != "Build U2." || !reflect.DeepEqual(before, read(q.ID)) {
					t.Fatalf("repeat: %+v next=%q", got, f.NextStep)
				}
			case "decision":
				unreachable := true
				var environments [][]string
				owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "ENGINE", nil }}
				owners.delivery.ownerEnvelope = func(p intentProcess, verb string) (verbresult.Result, error) {
					environments = append(environments, slices.Clone(p.env))
					childOwners := owners
					childOwners.lookupEnv = func(key string) (string, bool) {
						for _, value := range p.env {
							if v, ok := strings.CutPrefix(value, key+"="); ok {
								return v, true
							}
						}
						return "", false
					}
					childOwners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
						if unreachable {
							return goal.Endpoint{}, errors.New("the remote cannot be reached")
						}
						return r.Resolve(root)
					}
					command, rest, ok := resolveIntentArgv(p.argv[1:])
					if !ok {
						t.Fatalf("child argv: %v", p.argv)
					}
					var out, errout bytes.Buffer
					code := runIntentIn(command, rest, &out, &errout, p.dir, childOwners)
					return verbresult.Read(out.Bytes(), verb, code, errout.String())
				}
				var ids []string
				for i, outcome := range []string{intentPartial, intentConfirmed, intentUnchanged} {
					why := folded
					if i == 2 {
						why = "a later reason"
					}
					code, got := b.runJSON(owners, "decision", "record", "--json", "--class", "values", "--text", "PAGE: the row is the first line", "--rejected", "a separate row", "--undo", "the seat asks again", "--", "question", "withdraw", q.ID, "--reason", why)
					wantCode := 0
					if i == 0 {
						wantCode = 1
					}
					if code != wantCode || got.Outcome != outcome {
						t.Fatalf("decision %d: %d %+v", i, code, got)
					}
					id := got.Data.(map[string]any)["decision"].(map[string]any)["id"].(string)
					ids = append(ids, id)
					if !slices.Equal(environments[i], []string{"METASYSTEM_DECISION_ID=" + id}) {
						t.Fatalf("environment: %v", environments)
					}
					if i == 0 {
						_, f := ledger()
						if f.NextStep != "Wait." {
							t.Fatal(f.NextStep)
						}
					}
					unreachable = false
				}
				log, bad, err := humanauthority.ReadDecisions(home)
				if err != nil || bad != 0 || len(log) != 3 {
					t.Fatalf("log: %+v %d %v", log, bad, err)
				}
				for i, outcome := range []string{intentPartial, intentConfirmed, intentUnchanged} {
					if log[i].ID != ids[i] || log[i].Outcome != outcome {
						t.Fatalf("log: %+v", log)
					}
				}
				if !reflect.DeepEqual(read(q.ID).Withdrawal, &channel.Withdrawal{Reason: folded, At: now.UTC(), Decision: ids[0]}) {
					t.Fatalf("record: %+v", read(q.ID).Withdrawal)
				}
				marker(q.ID, folded)
				show(q.ID, "channel question "+q.ID+" is withdrawn: "+folded, ids[0])
			case "goal scope":
				q.ID, q.Lineage = "question-no-lineage", ""
				r.SaveQuestion(t, q)
				withdraw(q.ID, reason, intentConfirmed, 0)
				marker(q.ID, folded)
				closedReplyGoal(t, r)
				q.ID, q.Goal = "question-finished-goal", "closed-goal"
				r.SaveQuestion(t, q)
				tip, _ := ledger()
				withdraw(q.ID, reason, intentConfirmed, 0)
				if after, _ := ledger(); after != tip {
					t.Fatal("finished goal was told")
				}
			case "failed tell":
				owners.dependencies.endpoint = func(string) (goal.Endpoint, error) {
					return goal.Endpoint{}, errors.New("the remote cannot be reached")
				}
				tip, _ := ledger()
				got := withdraw(q.ID, reason, intentPartial, 1)
				if got.Summary != "question "+q.ID+" is withdrawn; its goal does not show the reason yet" || !strings.Contains(strings.Join(got.Details, " "), "the remote cannot be reached") || got.Next == nil || !slices.Equal(got.Next.Argv, []string{"metasystem", "question", "withdraw", q.ID, "--reason", reason, "--json"}) {
					t.Fatalf("failed tell: %+v", got)
				}
				if read(q.ID).State != "closed" || read(q.ID).Withdrawal == nil {
					t.Fatal("withdrawal did not stand")
				}
				if after, _ := ledger(); after != tip {
					t.Fatal("failed tell changed the goal")
				}
				owners.dependencies.endpoint = r.Resolve
				got = withdraw(q.ID, reason, intentConfirmed, 0)
				if got.Summary != "question "+q.ID+" was already withdrawn; its goal now shows the reason" {
					t.Fatal(got.Summary)
				}
				marker(q.ID, folded)
				tip, _ = ledger()
				withdraw(q.ID, reason, intentUnchanged, 0)
				if after, _ := ledger(); after != tip {
					t.Fatal("repeat changed the goal")
				}
			case "act questions":
				for _, kind := range []string{"contract", "person-act"} {
					copy := q
					copy.ID = "question-" + kind
					if kind == "contract" {
						copy.Contract = true
					} else {
						copy.PersonAct = &channel.PersonAct{Verb: "work build", Goal: q.Goal, Work: "U"}
						copy.Plain = &channel.Plain{Act: "metasystem work build reply-goal --work U --brief brief.md"}
					}
					r.SaveQuestion(t, copy)
					withdraw(copy.ID, reason, intentConfirmed, 0)
					marker(copy.ID, folded)
				}
				q.ID, q.Contract, q.State, q.ClosedBecause, q.ClosedAt = "question-act-closed", true, "closed", "goal budget: the box was raised", now
				r.SaveQuestion(t, q)
				tip, _ := ledger()
				withdraw(q.ID, "noise", intentUnchanged, 0)
				if after, _ := ledger(); after != tip || read(q.ID).Withdrawal != nil {
					t.Fatal("closure by its act was told as withdrawal")
				}
				code, got := b.runJSON(owners, "question", "show", q.ID)
				if code != 0 || got.Summary != "channel question "+q.ID+" is withdrawn" {
					t.Fatalf("act closure: %+v", got)
				}
			case "recorded reply":
				q.ID = "question-recorded-reply"
				q.UnitStop = &channel.UnitStopQuestion{Loop: "unit-round", Subject: "reply-goal/U/run", Attempt: 1, Finding: "read:1", Needs: "metasystem goal accept-risk reply-goal --finding read:1"}
				q.Answer = &channel.Answer{Text: "yes", Phase: "recorded", At: r.Sent}
				r.SaveQuestion(t, q)
				got := withdraw(q.ID, "fixed another way", intentConfirmed, 0)
				if got.Summary != "question "+q.ID+" is withdrawn; it had already been answered, and that answer stays recorded" {
					t.Fatal(got.Summary)
				}
				show(q.ID, "question "+q.ID+" answered 'yes' at 09:13", "")
				_, out, _ := b.run(owners, "question", "show", q.ID)
				if !strings.Contains(out, "answered 'yes' at 09:13") {
					t.Fatal(out)
				}
				marker(q.ID, "fixed another way")
			case "unknown decision":
				const id = "01M7ZZZZZZZZZZZZZZZZZZZZZZ"
				owners.lookupEnv = func(key string) (string, bool) { return id, key == "METASYSTEM_DECISION_ID" }
				withdraw(q.ID, reason, intentConfirmed, 0)
				if read(q.ID).Withdrawal.Decision != id {
					t.Fatal(read(q.ID).Withdrawal)
				}
				show(q.ID, "channel question "+q.ID+" is withdrawn: "+reason, "")
			}
		})
	}
}
