package launch

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// TestUnitRunCarriesRoundsAndEndsInJudgement (R24, U10a-2, unit runner):
// every launch of a round carries the round and the run's ceiling, and the
// finished round puts the goal on the board as judgement with no owner; the
// run's step states write no card of their own.
func TestUnitRunCarriesRoundsAndEndsInJudgement(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "")
	machine := "m1-unit-run-card"
	fixture.manager.Seat = board.Seat{Machine: machine, Installation: "/checkouts/m1-unit-run-card/metasystem"}
	fixture.runner.options.MaxRounds = 3
	fixture.manager.Settings.UnitCountedRounds = 2
	var rounds [][2]int
	var cardsDuringSteps int
	fixture.starter.onStart = func(record Record) error {
		rounds = append(rounds, [2]int{record.Round, record.MaxRounds})
		if _, ok := cardOf(t, machine, "goal"); ok {
			cardsDuringSteps++
		}
		return nil
	}
	result, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || result.Record.State != "awaiting-judgement" || result.Record.MaxRounds != 3 || result.Record.CountedCap != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, round := range rounds {
		if round != [2]int{1, 3} {
			t.Fatalf("launch rounds %v, want every launch at round 1 of 3", rounds)
		}
	}
	if len(rounds) != 3 || cardsDuringSteps != 0 {
		t.Fatalf("launches %d, cards written by step states %d", len(rounds), cardsDuringSteps)
	}
	card, ok := cardOf(t, machine, "goal")
	if !ok || card.Stage != board.StageJudgement || card.Owner != nil || card.Round == nil || card.Round.N != 1 || card.Round.Max == nil || *card.Round.Max != 2 {
		t.Fatalf("judgement card = %+v (%v)", card, ok)
	}
}
