package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingAgentWithdrawsOnlyItsStaleLaneQuestion(t *testing.T) {
	t.Parallel()
	brief := landingAgentBrief("fixture", lane.Wake{})
	for _, premise := range []string{"The checkout is not on main.", "Main is at commit <full SHA>."} {
		if !strings.Contains(brief, premise) {
			t.Fatalf("agent cannot record a withdrawable premise: %s", brief)
		}
	}
	for _, scenario := range []string{"back on main", "main advanced", "still off main", "same main", "other agent", "unrelated question"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			lineage := lane.AgentLineage
			facts := []string{"The checkout is not on main. May I return it to main?"}
			if strings.Contains(scenario, "main advanced") || scenario == "same main" {
				facts = []string{"Main is at commit aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa. May I rebuild the batch?"}
			}
			if scenario == "other agent" {
				lineage = "another-agent"
			}
			if scenario == "unrelated question" {
				facts = []string{"May I return a conflicting goal?"}
			}
			q, err := channel.Ask(channel.AskRequest{RepoRoot: install, About: "lane", Kind: "other", Machine: "this-machine", Lineage: lineage, Facts: facts})
			if err != nil {
				t.Fatal(err)
			}
			a := landingAgent{machine: func(string) (string, error) { return "this-machine", nil }, proofEffects: plain.ProveSeams{Git: func(_ string, args ...string) (string, error) {
				switch args[0] {
				case "symbolic-ref":
					if scenario == "still off main" {
						return "goal/g", nil
					}
					return "main", nil
				case "rev-parse":
					if scenario == "same main" {
						return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil
					}
					return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", nil
				case "cat-file", "merge-base":
					return "", nil
				}
				return "", fmt.Errorf("unexpected git %v", args)
			}}}
			hold, err := a.questionHold(install)
			if err != nil {
				t.Fatal(err)
			}
			got, err := channel.ReadQuestion(install, q.ID)
			if err != nil {
				t.Fatal(err)
			}
			stale := scenario == "back on main" || scenario == "main advanced"
			if stale {
				if hold != "" || got.State != "closed" {
					t.Fatalf("stale premise still holds: %q %+v", hold, got)
				}
			} else if hold == "" || got.State != "open" {
				t.Fatalf("live or unowned question withdrawn: %q %+v", hold, got)
			}
		})
	}
}
