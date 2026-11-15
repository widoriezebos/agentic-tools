package steward

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func TestEveryDecisionIsReportedOnce(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"delivered", "retry", "no channel", "unreadable set"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			fixture := newUnservedChannelFixture(t)
			home := t.TempDir()
			loc := time.FixedZone("local", 2*3600)
			start := time.Date(2026, 11, 14, 23, 50, 0, 0, loc)
			cfg := TickConfig{AskHome: home, Now: time.Date(2026, 11, 16, 0, 5, 0, 0, loc), narrationLocation: loc}
			var decisions []humanauthority.Decision
			for i := 24; i >= 0; i-- {
				d := humanauthority.Decision{ID: fmt.Sprintf("D%02d", i), At: start.Add(time.Duration(i) * time.Minute),
					Grant: "G1", Class: "lane-recovery", Argv: []string{"landing", "run", "--goals", fmt.Sprintf("goal-%02d", i)}, Outcome: "confirmed"}
				decisions = append(decisions, d)
				if err := humanauthority.AppendDecision(home, d); err != nil {
					t.Fatal(err)
				}
			}
			for _, d := range []humanauthority.Decision{
				{ID: "today", Grant: "G1", At: cfg.Now.Add(-time.Minute), Argv: []string{"waiting"}},
				{ID: "person", At: start, Argv: []string{"person"}},
			} {
				if err := humanauthority.AppendDecision(home, d); err != nil {
					t.Fatal(err)
				}
			}
			pass := func() error { return runTickReports(fixture.root, cfg, func() error { return nil }) }
			if mode == "no channel" {
				fixture.root = t.TempDir()
				if err := pass(); err != nil {
					t.Fatal(err)
				}
			} else if mode == "unreadable set" {
				if err := os.WriteFile(decisionsDeliveredPath(home), []byte("broken"), 0o600); err != nil {
					t.Fatal(err)
				}
				fixture.serve(t)
				if err := pass(); err == nil || !strings.Contains(err.Error(), "read delivered decisions") {
					t.Fatalf("an unreadable delivered set was ignored: %v", err)
				}
				data, err := os.ReadFile(decisionsDeliveredPath(home))
				if err != nil || string(data) != "broken" || len(fixture.posts(t)) != 0 {
					t.Fatalf("an unreadable set was replaced or posted: %q %v", data, err)
				}
				return
			} else {
				if mode == "retry" {
					if err := pass(); err == nil {
						t.Fatal("an unavailable channel reported delivery")
					}
					if _, err := os.Stat(decisionsDeliveredPath(home)); !os.IsNotExist(err) {
						t.Fatalf("a failed post marked decisions delivered: %v", err)
					}
				}
				fixture.serve(t)
				if err := pass(); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "no channel" {
				if _, err := os.Stat(decisionsDeliveredPath(home)); !os.IsNotExist(err) || len(fixture.posts(t)) != 0 {
					t.Fatalf("no channel still submitted or marked decisions: %v", err)
				}
				return
			}
			posts := fixture.posts(t)
			if len(posts) != 1 || !strings.HasPrefix(posts[0], "Decisions taken in your name under a grant:\n2026-11-14\n") ||
				!strings.Contains(posts[0], "\n2026-11-15\n") || strings.Contains(posts[0], "waiting") || strings.Contains(posts[0], "person") {
				t.Fatalf("wrong daily report: %q", posts)
			}
			for _, d := range decisions {
				overrule, _, _ := humanauthority.OverruleLine(d)
				line := fmt.Sprintf("%s lane-recovery: %s -> confirmed; overrule: %s", d.At.In(loc).Format("15:04"), strings.Join(d.Argv, " "), overrule)
				if strings.Count(posts[0], line) != 1 {
					t.Fatalf("decision %s was not delivered once with its overrule: %q", d.ID, posts[0])
				}
			}
			if strings.Index(posts[0], "23:50 lane-recovery:") > strings.Index(posts[0], "00:14 lane-recovery:") {
				t.Fatal("days were not delivered oldest first")
			}
			before, err := os.ReadFile(decisionsDeliveredPath(home))
			if err != nil {
				t.Fatal(err)
			}
			marked, err := readDeliveredDecisions(home)
			if err != nil || len(marked) != 25 || marked["today"] || marked["person"] {
				t.Fatalf("wrong delivered set: %v %v", marked, err)
			}
			cfg.Now = cfg.Now.Add(time.Hour)
			if err := pass(); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(decisionsDeliveredPath(home))
			if err != nil || !bytes.Equal(before, after) || len(fixture.posts(t)) != 1 {
				t.Fatalf("a quiet pass or restart redelivered decisions: %q %v", fixture.posts(t), err)
			}
			t.Log("tick delivered all 25 completed-day decisions once; the unfinished day waits")
		})
	}
}
