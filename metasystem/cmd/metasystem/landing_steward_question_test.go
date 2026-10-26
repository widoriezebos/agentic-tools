package main

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestLandingAgentStewardQuestionHoldsNothing(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	now := time.Date(2026, 10, 1, 22, 0, 0, 0, time.UTC)
	a := landingAgent{machine: func(string) (string, error) { return "this-machine", nil }}
	ask := func(facts []string) channel.Question {
		t.Helper()
		q, err := channel.Ask(channel.AskRequest{RepoRoot: install, About: "lane", Kind: "other", Machine: "this-machine", Lineage: lane.AgentLineage, Now: now, Facts: facts})
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	q := ask([]string{"The steward stopped acting", `lane steward: {"ask":"steward-act-failed","row":"unproven-incidents","subject":""}`, "tried: prove main"})
	if hold, err := a.questionHold(install); err != nil || hold != "" {
		t.Fatalf("steward question held the agent: %q %v", hold, err)
	}
	asked := ask([]string{"The checkout is not on main. May I return it to main?"})
	if hold, err := a.questionHold(install); err != nil || hold == "" {
		t.Fatalf("agent's own question no longer holds: %q %v", hold, err)
	}
	for _, id := range []string{q.ID, asked.ID} {
		got, err := channel.ReadQuestion(install, id)
		if err != nil || got.State != "open" {
			t.Fatalf("hold reader changed the question: %+v %v", got, err)
		}
	}
}
