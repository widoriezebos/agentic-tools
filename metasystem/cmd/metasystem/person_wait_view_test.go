package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func TestGoalPersonWaitWords(t *testing.T) {
	t.Parallel()
	since := time.Date(2026, 8, 28, 15, 4, 0, 0, time.FixedZone("fixture", 2*3600))
	p := &dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown, PersonWaitSince: since, PersonPaused: 90 * time.Minute, Limits: goal.Budget{ElapsedLimit: "8h"}}
	want := "waiting for a person since " + since.Local().Format("15:04") + ", clock paused (1.5h so far)"
	if p.PersonWords() != want {
		t.Fatalf("person words = %q, want %q", p.PersonWords(), want)
	}
	view := intentBudgetView{Box: "8h/6/360m/1/2", Lens: "claim", Projection: p}
	if !strings.Contains(strings.Join(view.lines(), "\n"), want) {
		t.Fatalf("goal status lost person wait: %v", view.lines())
	}
	page := textui.New(textui.Env{})
	goalShown{budget: view}.budgetSection(page)
	if !strings.Contains(page.String(), want) {
		t.Fatalf("goal show lost person wait: %s", page.String())
	}
	data, err := json.Marshal(p)
	if err != nil || strings.Contains(string(data), "PersonWaitSince") || strings.Contains(string(data), "PersonPaused") {
		t.Fatalf("private wait fields reached JSON: %s %v", data, err)
	}
	p.PersonWaitSince = time.Time{}
	if p.PersonWords() != "" {
		t.Fatalf("ended wait still printed: %q", p.PersonWords())
	}
}
