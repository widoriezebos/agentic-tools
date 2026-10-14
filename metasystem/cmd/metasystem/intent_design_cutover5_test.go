package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestDesignReviewAutomaticallyPublishesZero(t *testing.T) {
	t.Parallel()
	b, _, _ := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
	b.lineage = b.goalFile(bedGoal).Claimed.Lineage
	result := b.review()
	if result.Outcome != intentConfirmed || len(b.goalFile(bedGoal).DesignExits) != 1 || b.closes != 1 || !strings.Contains(string(mustRead(t, b.design)), "Status: accepted") {
		t.Fatalf("zero did not publish automatically: %+v", result)
	}
	if again := b.review(); again.Outcome != intentUnchanged || b.closes != 1 || len(b.goalFile(bedGoal).DesignExits) != 1 {
		t.Fatalf("replay duplicated acceptance: %+v", again)
	}
}

func TestDesignReviewHeldAskMatchesSuccessfulAct(t *testing.T) {
	t.Parallel()
	for _, effect := range []string{"continue", "acceptance", "acceptance-notify"} {
		t.Run(effect, func(t *testing.T) {
			t.Parallel()
			var b *designLoopBed
			var answer string
			if effect == "continue" {
				b, answer, _ = cutoverPositiveRead(t)
			} else {
				b, _, _ = designEvidenceBed(t, evidenceInventory, acceptanceUnits)
				b.lineage = b.goalFile(bedGoal).Claimed.Lineage
			}
			conf := filepath.Join(b.install, "metasystem.conf")
			b.writeFile(conf, string(mustRead(t, conf))+"\nreview.stop=person\n")
			args := []string{"design", "review", b.design, "--tool-calls", "30"}
			if answer != "" {
				args = append(args, "--dispositions", answer)
			}
			_, held := b.do(args...)
			if !strings.Contains(held.Summary, "holds") || b.closes != 0 || len(b.followUps) != 0 {
				t.Fatalf("hold crossed: %+v", held)
			}
			questions, unreadable := channel.WalkOpenQuestions(b.install)
			if len(unreadable) != 0 || len(questions) != 1 {
				t.Fatalf("missing exact ask: %v %v", questions, unreadable)
			}
			q := questions[0]
			if q.UnitStop == nil || q.UnitStop.Loop != "design-round" || q.UnitStop.Subject != "rev1" || q.UnitStop.Attempt != 1 || q.UnitStop.Needs == "" {
				t.Fatalf("unbound ask: %+v", q)
			}
			b.do(args...)
			questions, _ = channel.WalkOpenQuestions(b.install)
			if len(questions) != 1 {
				t.Fatal("replay duplicated ask")
			}
			if err := channel.RecordUnitStopAct(b.install, channel.UnitStopAct{ID: "wrong", Goal: bedGoal, Loop: "design-round", Subject: "another-root", Attempt: 1, Findings: []string{q.UnitStop.Finding}, Kind: q.UnitStop.AcceptableActs[0], Reason: "unrelated", At: q.OpenedAt}); err != nil {
				t.Fatal(err)
			}
			questions, _ = channel.WalkOpenQuestions(b.install)
			if len(questions) != 1 {
				t.Fatal("unmatched act closed ask")
			}
			owners := b.intentBed.owners()
			owners.delivery = b.owners
			now, _ := owners.commandNow(b.root())
			owners.prove = enrolledPersonProver(t, b.root(), now)
			if effect == "acceptance" {
				endpoint := owners.dependencies.endpoint
				owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
					e, err := endpoint(root)
					e.Repository = &designAcceptanceRepository{Repository: e.Repository, capture: func() error { return errors.New("publication storage unavailable") }}
					return e, err
				}
				code, failed := b.runJSON(owners, args...)
				questions, unreadable = channel.WalkOpenQuestions(b.install)
				if code == 0 || failed.Outcome != intentFailed || len(questions) != 1 || len(unreadable) != 0 || b.closes != 0 || len(b.goalFile(bedGoal).DesignExits) != 0 {
					t.Fatalf("failed act closed ask or published: %+v %v", failed, questions)
				}
				owners.dependencies.endpoint = endpoint
			}
			actsPath := filepath.Join(b.install, "artifacts", "agents", "channel", "unit-stop-acts")
			if effect == "acceptance-notify" {
				if err := os.Rename(actsPath, actsPath+".prior"); err != nil {
					t.Fatal(err)
				}
				b.writeFile(actsPath, "act storage unavailable")
			}
			_, acted := b.runJSON(owners, args...)
			if acted.Outcome != intentConfirmed && acted.Outcome != intentInProgress {
				t.Fatalf("holder act refused: %+v", acted)
			}
			if effect == "acceptance-notify" {
				questions, unreadable = channel.WalkOpenQuestions(b.install)
				if len(questions) != 1 || len(unreadable) != 0 || len(acted.Details) == 0 || len(b.goalFile(bedGoal).DesignExits) != 1 || b.closes != 1 {
					t.Fatalf("notification failure undid acceptance or hid the retained ask: %+v %v", acted, questions)
				}
				if err := os.Remove(actsPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(actsPath+".prior", actsPath); err != nil {
					t.Fatal(err)
				}
				_, replay := b.runJSON(owners, args...)
				if replay.Outcome != intentUnchanged || b.closes != 1 || len(b.goalFile(bedGoal).DesignExits) != 1 {
					t.Fatalf("notification repair duplicated acceptance: %+v", replay)
				}
			}
			questions, unreadable = channel.WalkOpenQuestions(b.install)
			if len(unreadable) != 0 || len(questions) != 0 {
				t.Fatalf("successful act left ask: %v %v", questions, unreadable)
			}
		})
	}
}

func TestDesignReviewNumericPolicyStopsBeforeCorrection(t *testing.T) {
	t.Parallel()
	for _, policy := range []string{"0", "1", "999999999999999999999999999999999"} {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			b, answer, _ := cutoverPositiveRead(t)
			conf := filepath.Join(b.install, "metasystem.conf")
			b.writeFile(conf, string(mustRead(t, conf))+"\nreview.stop="+policy+"\n")
			if policy == "0" {
				mappings := concreteFoldMapping()
				mappings.Finding, mappings.Decision = "A", "## Publication"
				page := strings.Replace(string(mustRead(t, b.design)), "Second version.", "First version.", 1)
				b.writeFile(b.design, suppliedFoldPage(t, page, mappings))
			}
			result := b.review("--dispositions", answer)
			if policy == "0" {
				if len(b.followUps) != 0 || b.job("rev1")["designStop"] == nil || result.Outcome != intentConfirmed || len(b.goalFile(bedGoal).DesignExits) != 1 {
					t.Fatalf("zero correction cap did not fold: %+v", result)
				}
			} else if len(b.followUps) != 1 || result.Outcome != intentInProgress {
				t.Fatalf("positive correction cap stopped early: %+v", result)
			}
		})
	}
}

func (b *designLoopBed) collectHeld() intentResult {
	b.t.Helper()
	conf := filepath.Join(b.install, "metasystem.conf")
	original := mustRead(b.t, conf)
	b.writeFile(conf, string(original)+"\nreview.stop=person\n")
	result := b.review()
	b.writeFile(conf, string(original))
	return result
}
