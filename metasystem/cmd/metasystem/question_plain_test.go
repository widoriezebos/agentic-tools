package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

var plainQuestionNow = time.Date(2031, 1, 1, 9, 0, 0, 0, time.UTC)

func plainAskSurface() channelAskSurface {
	return channelAskSurface{
		identity: func(string) (string, string, error) { return "fixture", "", nil },
		load:     func(string) (phase.Loaded, error) { return phase.Loaded{}, nil },
		cursor:   func(string) (string, bool, error) { return "", false, nil },
	}
}

func TestQuestionShowCarriesWhatThePostDrops(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"budget", "ordinary", "process", "lane-policy", "open-unit-stop", "answered-awaiting-act", "answered-lane"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newProcessBed(t)
			owners := channelQuestionOwners(b, &questionProvider{})
			owners.commandNow = func(string) (time.Time, error) { return plainQuestionNow, nil }
			root := b.root()
			var q channel.Question
			switch name {
			case "budget":
				budget, err := goal.NewBudget("2h", 5, 600, 1, 0)
				if err != nil {
					t.Fatal(err)
				}
				var code int
				q, _, code, err = askChannelQuestionVia(root, channelAskInput{Goal: bedGoal, Kind: "budget-above-norm", Facts: []string{"Raise the box?", "the box doubles"}, Options: []string{"yes: raise it", "no: wait"}, Recommendation: "raise it once and stop there", IfNothing: "the release waits", Budget: &budget}, plainAskSurface())
				if err != nil || code != 0 {
					t.Fatalf("ask: %d %v", code, err)
				}
			case "ordinary":
				code, _, stderr := b.run(owners, "question", "ask", bedGoal, "--question", "Keep the release?", "--option", "yes: keep it", "--if-nothing", "the release waits")
				if code != 0 {
					t.Fatalf("ask: %d %s", code, stderr)
				}
				qs, unread := channel.WalkQuestions(root)
				if len(unread) != 0 || len(qs) != 1 {
					t.Fatalf("questions: %+v %v", qs, unread)
				}
				q = qs[0]
			case "process":
				c := processchange.Check{Root: t.TempDir(), Installation: roots.Installation(root), ProcessAct: processchange.ProcessAct{Goal: bedGoal, Unit: "unit", Operation: "unit-check", Checkout: "checkout", AfterArgv: []string{"make", "check"}}, Observation: true, Now: plainQuestionNow, Remedy: func(string) string { return "person check" }}
				if _, err := processchange.AdmitCheck(c); err != nil {
					t.Fatal(err)
				}
				c.Observation, c.AfterArgv = false, []string{"make", "audit"}
				act, err := processchange.AdmitCheck(c)
				if err != nil {
					t.Fatal(err)
				}
				q, err = channel.ReadQuestion(root, act.Question)
				if err != nil {
					t.Fatal(err)
				}
			case "lane-policy", "answered-lane":
				f := newTrunkPermissionFixture(t)
				_, _, handInErr := plain.HandIn(f.l.install, plain.Line{At: laneTestNow.Format(time.RFC3339Nano), Goal: bedGoal, SHA: f.tip.status.BranchTip, Seat: f.b.root()})
				if handInErr != nil {
					t.Fatal(handInErr)
				}
				q = f.question(t)
				root, owners = f.l.install, f.owners
				if name == "answered-lane" {
					q.Answer = &channel.Answer{Text: "ok", Phase: "recorded", At: plainQuestionNow}
					q.Recommendation = "run the matching act"
					writeRunRootJSON(t, filepath.Join(root, "artifacts", "agents", "channel", "questions", q.ID+".json"), q)
				}
			case "answered-awaiting-act", "open-unit-stop":
				var err error
				q, err = channel.Ask(channel.AskRequest{RepoRoot: root, Goal: bedGoal, Kind: "other", Now: plainQuestionNow, Facts: []string{"finding remains unresolved", "S-1: missing behaviour"}, Recommendation: "accept the risk only if bounded", Plain: &channel.Plain{Decision: "Continue this unit?", IfNothing: "the unit waits"}, UnitStop: &channel.UnitStopQuestion{Loop: "unit-round", Subject: bedGoal + "/unit/run", Attempt: 1, Finding: "S-1", Needs: "metasystem work revise " + bedGoal + " --brief FILE --reason TEXT --by NAME", AcceptableActs: []string{"work-revise"}}})
				if err != nil {
					t.Fatal(err)
				}
				if name == "answered-awaiting-act" {
					q.Answer, q.State = &channel.Answer{Text: "ok", Phase: "recorded", At: plainQuestionNow}, "open"
				}
				writeRunRootJSON(t, filepath.Join(root, "artifacts", "agents", "channel", "questions", q.ID+".json"), q)
			}
			code, stdout, stderr := b.run(owners, "question", "show", "channel:"+q.ID, "--repo", root)
			if code != 0 {
				t.Fatalf("show: %d %s%s", code, stdout, stderr)
			}
			text := stdout + stderr
			wants := append([]string{}, q.Facts...)
			if name == "answered-lane" {
				wants = nil
				if strings.Contains(text, "lane policy:") || strings.Contains(text, "evidence:") {
					t.Fatalf("answered lane exposes its stop record: %s", text)
				}
			}
			if q.Plain == nil {
				t.Fatal("question has no plain words")
			}
			wants = append(wants, "Decision: "+q.Plain.Decision, "If you do nothing: "+q.Plain.IfNothing)
			if q.Recommendation != "" {
				wants = append(wants, "Recommendation: "+q.Recommendation)
			}
			if name == "budget" {
				wants = append(wants, "Proposed box: 2h, 5 attempts, 600 reserved minutes, 1 active job, 0 review rounds")
			}
			if name == "ordinary" {
				wants = append(wants, "Decision: Keep the release?", "If you do nothing: the release waits")
			}
			if name == "answered-awaiting-act" || name == "answered-lane" {
				wants = append(wants, "the act is still needed", "replied")
			}
			for _, want := range wants {
				if !strings.Contains(oneSpaced(text), oneSpaced(want)) {
					t.Errorf("show lacks %q: %s", want, text)
				}
			}
		})
	}
}

func TestBuildStartQuestionCarriesItsCommand(t *testing.T) {
	t.Parallel()
	installation := t.TempDir()
	writeTestingFixtureFile(t, filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0600)
	inv := &intentInvocation{stateRoot: t.TempDir(), layout: stateroot.Layout{InstallationRoot: stateroot.Installation(installation)}}
	var id string
	inv.owners.processes.ask = func(root string, in channelAskInput) (channel.Question, []string, int, error) {
		q, warnings, code, err := askChannelQuestionVia(root, in, plainAskSurface())
		id = q.ID
		return q, warnings, code, err
	}
	result := inv.unitOutcome(nil, launch.UnitResult{Record: launch.UnitRunRecord{Goal: "goal"}}, &launch.CodedError{Code: "LAUNCH_BUILD_PERSON"}, inv.targets("goal"), []string{"metasystem", "work", "build", "goal"})
	q, err := channel.ReadQuestion(installation, id)
	if err != nil || result.Outcome != intentRefused || q.Plain == nil || q.Plain.Act != "metasystem work build goal" || q.Plain.IfNothing != "the build waits" {
		t.Fatalf("build words: %+v %v, %+v", q, err, result)
	}
	want := "Start this build at your enrolled terminal?\nTo go on: metasystem work build goal\nIf you do nothing: the build waits.\n- start: run the exact build command at your enrolled terminal.\n- wait: leave this build waiting."
	if q.Text != want {
		t.Fatalf("build post = %q, want %q", q.Text, want)
	}
}

func TestRebaseQuestionNamesNoCommit(t *testing.T) {
	t.Parallel()
	b, owners, _ := rebaseIntentBed(t)
	owners.connection.rebase = func(branch.RebaseRequest) (branch.RebaseResult, error) {
		return branch.RebaseResult{}, rebaseJudgementFixture(t)
	}
	var ids []string
	owners.connection.askRebase = func(root string, in channelAskInput) (channel.Question, []string, int, error) {
		q, warnings, code, err := askChannelQuestionVia(root, in, plainAskSurface())
		ids = append(ids, q.ID)
		return q, warnings, code, err
	}
	code, _, _ := b.run(owners, "work", "rebase", b.id)
	if code == 0 || len(ids) != 2 {
		t.Fatalf("rebase: %d %v", code, ids)
	}
	for i, id := range ids {
		q, err := channel.ReadQuestion(b.root(), id)
		if err != nil {
			t.Fatal(err)
		}
		want := "the goal's undoes main's change there"
		if i == 0 {
			want += " (peer)"
		}
		if q.Options[1].Consequence != want || q.Plain == nil || q.Plain.IfNothing != "nothing lands until you answer" {
			t.Fatalf("rebase words: %+v", q)
		}
		decision := fmt.Sprintf("Goal %s and main both changed lines 2 to 3 of source.go. Keep main's, keep the goal's, or write a third.", b.id)
		if i == 1 {
			decision = fmt.Sprintf("Goal %s and main both changed added.go where no common version exists. Keep main's, keep the goal's, or write a third.", b.id)
		}
		post := decision + "\nTo answer: metasystem question answer " + q.ID + "\nIf you do nothing: nothing lands until you answer.\n- keep main's: main's drops what " + b.id + " did there and unit u1 loses its read.\n- keep the goal's: " + want + ".\n- write a third: a third is read again."
		if q.Text != post {
			t.Fatalf("rebase post = %q, want %q", q.Text, post)
		}
		if i == 1 && !strings.HasSuffix(q.Facts[2], "; main commit another-commit.") {
			t.Fatalf("lost commit evidence: %q", q.Facts[2])
		}
	}
}

func TestPolicyOverrideKeepsItsImpact(t *testing.T) {
	t.Parallel()
	f := newTrunkPermissionFixture(t)
	_, _, handInErr := plain.HandIn(f.l.install, plain.Line{At: laneTestNow.Format(time.RFC3339Nano), Goal: bedGoal, SHA: f.tip.status.BranchTip, Seat: f.b.root()})
	if handInErr != nil {
		t.Fatal(handInErr)
	}
	q := f.question(t)
	want := channel.LaneStopCommand(q) + " (lets " + bedGoal + " land while main's failing checks remain unresolved; undo: metasystem landing give-back " + bedGoal + " --reason TEXT)"
	if q.Plain == nil || q.Plain.Act != want || q.Plain.Decision != "The landing lane needs your word before it goes on" || q.Plain.IfNothing != "the lane does not do this; this question closes when the act is recorded" {
		t.Fatalf("policy words: %+v", q.Plain)
	}
	post := "The landing lane needs your word before it goes on.\nTo go on: " + want + "\nIf you do nothing: the lane does not do this; this question closes when the act is recorded."
	if q.Text != post {
		t.Fatalf("policy post = %q, want %q", q.Text, post)
	}
	if strings.Contains(q.Plain.Act, "tr-") || regexp.MustCompile(`[0-9a-f]{12}`).MatchString(q.Plain.Act) {
		t.Fatalf("identity in policy act: %s", q.Plain.Act)
	}
	for _, incident := range f.incidents {
		if !strings.Contains(q.Facts[0], incident.Identity) {
			t.Fatalf("lost incident %s: %q", incident.Identity, q.Facts)
		}
	}
}

func TestUnitAsksNameTheUnit(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"review-finding", "build-gap", "build-size"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var b *workBed
			var want string
			if name == "review-finding" {
				f := newTransferScenarioFixture(t, false)
				b = f.bed
				code, result := transferPublic(t, b, f.owners, "work", "review", b.id, "--work", "stopped")
				if code != 1 || result.Outcome != intentInProgress {
					t.Fatalf("review: %d %+v", code, result)
				}
				qs, unread := channel.WalkOpenQuestions(b.root())
				if len(unread) != 0 || len(qs) != len(f.findings) {
					t.Fatalf("finding questions: %+v %v", qs, unread)
				}
				for _, q := range qs {
					if q.UnitStop == nil {
						t.Fatalf("missing unit stop: %+v", q)
					}
					var claim string
					for _, finding := range f.findings {
						if finding.ID == q.UnitStop.Finding {
							claim = finding.Claim
						}
					}
					want = fmt.Sprintf("Unit stopped of goal %s stopped on a review finding: %s", b.id, claim)
					if claim == "" || q.Plain == nil || *q.Plain != (channel.Plain{Decision: want, IfNothing: "the unit stays stopped; this question closes when the act is recorded or the unit closes"}) {
						t.Fatalf("finding words: %+v, want %q", q.Plain, want)
					}
					post := want + ".\nTo go on: " + channel.ActCommand(q) + "\nIf you do nothing: the unit stays stopped; this question closes when the act is recorded or the unit closes."
					if q.Text != post {
						t.Fatalf("finding post = %q, want %q", q.Text, post)
					}
				}
				return
			} else {
				diff := ""
				if name == "build-size" {
					diff = "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n-old\n+new\ndiff --git a/a_test.go b/a_test.go\n--- /dev/null\n+++ b/a_test.go\n+test\n"
				}
				b = outcomeBed(t, diff)
				code, result, _ := outcomeBuild(t, b)
				if code != 1 {
					t.Fatalf("build: %d %+v", code, result)
				}
				want = fmt.Sprintf("Unit outcome of goal %s stopped after build round 1: the builder reported a gap", b.id)
			}
			qs, unread := channel.WalkOpenQuestions(b.root())
			if name == "build-size" {
				want = fmt.Sprintf("%s: the build of unit outcome grew to 3 changed lines against the 1 planned, at %s.", strings.ReplaceAll(b.id, "-", " "), qs[0].OpenedAt.Local().Format("15:04"))
				if len(unread) != 0 || len(qs) != 1 || !qs[0].Contract || qs[0].Plain.Decision != want || !strings.HasPrefix(qs[0].Text, "S"+want[1:]+"\n\n★ Accept the size — ") {
					t.Fatalf("the size ask names no solution for the unit: %+v", qs)
				}
				return
			}
			if len(unread) != 0 || len(qs) != 1 || qs[0].Plain == nil {
				t.Fatalf("unit questions: %+v %v", qs, unread)
			}
			nothing := "the unit stays held; this question closes when the act is recorded"
			if name == "review-finding" {
				nothing = "the unit stays stopped; this question closes when the act is recorded or the unit closes"
			}
			post := want + ".\nTo go on: " + channel.ActCommand(qs[0]) + "\nIf you do nothing: " + nothing + "."
			if qs[0].Text != post {
				t.Fatalf("unit post = %q, want %q", qs[0].Text, post)
			}
			if *qs[0].Plain != (channel.Plain{Decision: want, IfNothing: nothing}) {
				t.Fatalf("unit words: %+v, want %q", qs[0].Plain, want)
			}
		})
	}
}
