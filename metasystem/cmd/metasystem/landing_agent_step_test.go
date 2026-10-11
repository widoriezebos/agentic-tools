package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingAgentStepEveryVerbAndPersonExclusion(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"prove", "push", "resolve", "return", "run", "status"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			for _, agent := range []bool{true, false} {
				bed := newLaneVerbBed(t)
				if _, _, err := lane.Register(bed.home, lane.Layout{Checkout: lane.CheckoutRoot(bed.landingA), Install: lane.InstallRoot(bed.landingA)}, "fixture", laneTestNow); err != nil {
					t.Fatal(err)
				}
				owners := bed.owners()
				owners.landing.stepActor = func(string, int64) bool { return agent }
				command, _ := findIntentAction("landing", action)
				command.run = func(inv *intentInvocation) int {
					data, err := os.ReadFile(filepath.Join(plain.Dir(bed.landingA), "agent-step.json"))
					if agent && action != "status" {
						var step plain.AgentStep
						if err != nil || json.Unmarshal(data, &step) != nil || step.Verb != action || step.StartedAt != laneTestNow.Format("2006-01-02T15:04:05Z07:00") {
							t.Fatalf("during %s: %s %v", action, data, err)
						}
					} else if !os.IsNotExist(err) {
						t.Fatalf("excluded call recorded: %s %v", data, err)
					}
					return 7
				}
				var out, problem bytes.Buffer
				if code := runIntentIn(command, nil, &out, &problem, bed.cwd, owners); code != 7 {
					t.Fatalf("exit=%d", code)
				}
				if _, err := os.Stat(filepath.Join(plain.Dir(bed.landingA), "agent-step.json")); !os.IsNotExist(err) {
					t.Fatalf("exit left step: %v", err)
				}
			}
		})
	}
}

func TestLandingAgentSkillEndsAnEmptyBatchAtOnce(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "skills", "landing-agent", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	sentence := "When `landing status --json` shows no batch member left waiting or running (every member landed, returned or superseded), end your turn at once — unless you were woken with `full-due` (case 8) or main owes a proof with no active batch (step 1): run that proof first. The keeper wakes the agent when there is work; a person's return of the last member ends your turn."
	if strings.Count(string(data), sentence) != 2 {
		t.Fatal("the batch section and the loop must each carry the empty-batch sentence")
	}
}
