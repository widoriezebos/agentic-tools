package state

import (
	"strings"
	"testing"
	"time"
)

func TestDescribeMainNonRunExplainsRecordedRetryOrPersonAction(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"wake", "time", "spent", "after done", "after closed"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			r := Record{Agent: Agent{Word: "idle"}, Main: Main{Fact: Fact{Word: "unproven"}, CheckNotRun: true, RetryAllowed: true, Reason: "storage unavailable"}}
			want := "at its next wake"
			if mode == "after done" {
				r.Resolution.Word = "done"
			}
			if mode == "after closed" {
				r.Resolution.Word = "closed"
			}
			if mode == "time" {
				r.Main.RetryAt = time.Date(2026, 10, 15, 11, 30, 0, 0, time.UTC)
				want = r.Main.RetryAt.Local().Format("2006-01-02 15:04 MST")
			}
			if mode == "spent" {
				r.Main.RetryAllowed = false
				want = "metasystem landing prove --trunk"
			}
			got := Describe(r, RenderInputs{})
			if !strings.Contains(got.Headline, "main's last check could not run (storage unavailable)") || !strings.Contains(got.Headline, want) || strings.Contains(got.Person, "land a fix on main") || mode == "spent" && got.Agent != "wait for the person's check" {
				t.Fatalf("non-run advice: %+v", got)
			}
		})
	}
}
