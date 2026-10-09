package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

func TestDesignReviewGoalFreeHeldAskMatchesSuccessfulAct(t *testing.T) {
	t.Parallel()
	for _, effect := range []string{"continuation", "acceptance"} {
		t.Run(effect, func(t *testing.T) {
			t.Parallel()
			b, dir, raw := designEvidenceBedConfigured(t, evidenceInventory, func(b *designLoopBed) {
				b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "- Goals: standing-validation\n", "", 1))
			}, acceptanceUnits)
			var answer string
			if effect == "continuation" {
				raw["findings"], raw["rigor"], raw["verdictMaterialCount"] = []any{evidenceFinding("A", "## Publication", "")}, []any{evidenceRigor("A")}, 1
				b.writeJSON(filepath.Join(dir, "return.json"), raw)
				b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=1\n")
				answer = b.decide(b.review(), map[string]string{"A": "accepted | specified the requirement | ## Publication"})
				b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "First version.", "Second version.", 1))
			}
			conf := filepath.Join(b.install, "metasystem.conf")
			b.writeFile(conf, string(mustRead(t, conf))+"\nreview.stop=person\n")
			args := []string{"design", "review", b.design, "--tool-calls", "30"}
			if answer != "" {
				args = append(args, "--dispositions", answer)
			}
			_, held := b.do(args...)
			questions, unreadable := channel.WalkOpenQuestions(b.install)
			if !strings.Contains(held.Summary, "holds") || len(questions) != 1 || len(unreadable) != 0 || b.closes != 0 || len(b.followUps) != 0 {
				t.Fatalf("goal-free hold lost its ask or crossed the policy: %+v questions=%v unreadable=%v", held, questions, unreadable)
			}
			q := questions[0]
			if q.Goal != "" || q.About != "machine" || q.UnitStop == nil || q.UnitStop.Loop != "design-round" || q.UnitStop.Subject != "rev1" || q.UnitStop.Attempt != 1 || q.UnitStop.Finding != effect || held.Next == nil || q.UnitStop.Needs != shellCommand(held.Next.Argv) {
				t.Fatalf("goal-free ask is not bound to the prepared act: %+v", q)
			}
			b.do(args...)
			questions, _ = channel.WalkOpenQuestions(b.install)
			if len(questions) != 1 {
				t.Fatal("replay duplicated the held ask")
			}
			if err := channel.RecordUnitStopAct(b.install, channel.UnitStopAct{ID: "unrelated", Loop: "design-round", Subject: "another-root", Attempt: 1, Findings: []string{effect}, Kind: "design-" + effect, Reason: "unrelated act", At: q.OpenedAt}); err != nil {
				t.Fatal(err)
			}
			questions, _ = channel.WalkOpenQuestions(b.install)
			if len(questions) != 1 {
				t.Fatal("another design's act closed the ask")
			}
			owners := b.intentBed.owners()
			owners.delivery = b.owners
			now, _ := owners.commandNow(b.root())
			owners.prove = enrolledPersonProver(t, b.root(), now)
			code, acted := b.runJSON(owners, held.Next.Argv[1:]...)
			questions, unreadable = channel.WalkOpenQuestions(b.install)
			if (code != 0 && acted.Outcome != intentInProgress) || (acted.Outcome != intentConfirmed && acted.Outcome != intentInProgress) || len(questions) != 0 || len(unreadable) != 0 {
				t.Fatalf("holder's successful goal-free act left its ask open: exit=%d result=%+v questions=%v unreadable=%v", code, acted, questions, unreadable)
			}
			if len(b.goalFile(bedGoal).DesignExits) != 0 || b.fresh != 1 {
				t.Fatal("goal-free holder act gained program authority or a fresh chain")
			}
			if effect == "acceptance" && b.closes != 1 || effect == "continuation" && len(b.followUps) != 1 {
				t.Fatal("successful act did not produce its prepared effect")
			}
		})
	}
}
