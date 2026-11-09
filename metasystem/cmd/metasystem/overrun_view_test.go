package main

import (
	"encoding/json"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func TestOverrunWordsOnlyWhileTheBoxIsClosed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, run, want string
		state           dispatchcore.ElapsedBudgetState
	}{
		{"closed", "R", "past its box by 15m for run R", dispatchcore.AdmissionClosedElapsed},
		{"breached", "R", "past its box by 15m for run R", dispatchcore.ElapsedBreach},
		{"open", "R", "", ""},
		{"no overrun", "", "", dispatchcore.AdmissionClosedElapsed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown, ElapsedOverrunMinutes: 15, ElapsedOverrunRun: tc.run, ElapsedState: tc.state}
			if got := p.OverrunWords(); got != tc.want {
				t.Fatalf("overrun words = %q, want %q", got, tc.want)
			}
			view := intentBudgetView{Box: "1h/4/240m/1/2", Lens: "claim", Projection: p}
			page := textui.New(textui.Env{})
			goalShown{budget: view}.budgetSection(page)
			for _, text := range []string{strings.Join(view.lines(), "\n"), page.String()} {
				if strings.Contains(text, "past its box by") != (tc.want != "") || (tc.want != "" && !strings.Contains(text, tc.want)) {
					t.Fatalf("overrun display = %q, want %q", text, tc.want)
				}
			}
			body, err := json.Marshal(p)
			if err != nil || strings.Contains(string(body), "ElapsedOverrun") {
				t.Fatalf("private overrun fields reached JSON: %s %v", body, err)
			}
		})
	}
}
