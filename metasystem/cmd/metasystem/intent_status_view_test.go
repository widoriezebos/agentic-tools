package main

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stoptransition"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func TestStatusShowsTheStewardOperation(t *testing.T) {
	t.Parallel()
	zone := time.FixedZone("fixture", 3600)
	since := time.Date(2026, 11, 17, 8, 1, 0, 0, time.UTC)
	for _, activity := range []string{"situation (working, 12 done)", ""} {
		page := textui.New(textui.Env{Now: since.Add(time.Minute), Zone: zone, Width: 120})
		inv := &intentInvocation{}
		report := stoptransition.Report{Items: []stoptransition.StatusItem{{Family: "steward", Activity: activity, ActivitySince: since}}}
		inv.statusView("/fixture/seat", report, statusBoard{})(page)
		got := oneSpaced(page.String())
		if activity != "" && !strings.Contains(got, "steward runner: "+activity+", since 09:01") {
			t.Fatalf("page: %s", got)
		}
		if activity == "" && strings.Contains(got, "steward runner:") {
			t.Fatalf("activity without a record: %s", got)
		}
	}
}
