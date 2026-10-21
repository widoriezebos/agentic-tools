package processchange

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
)

func TestProcessQuestionsSayWhatAndIfNothing(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"check", "declaration", "setting-machine", "drift", "drift-process-change", "drift-missing-act"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			project, install := t.TempDir(), roots.Installation(t.TempDir())
			now := time.Date(2031, 1, 1, 9, 0, 0, 0, time.UTC)
			act := ProcessAct{Goal: "example", Unit: "unit", Operation: "unit-check", Checkout: project, AfterArgv: []string{"make", "check"}}
			c := Check{Root: project, Installation: install, ProcessAct: act, Observation: true, Now: now, Remedy: func(string) string { return "person check" }}
			want := channel.Plain{}
			switch name {
			case "check":
				if _, err := AdmitCheck(c); err != nil {
					t.Fatal(err)
				}
				c.Observation, c.AfterArgv = false, []string{"make", "audit"}
				if _, err := AdmitCheck(c); err != nil {
					t.Fatal(err)
				}
				want = channel.Plain{Decision: "Goal example wants to change a check it runs", IfNothing: "the check stays as it is"}
			case "declaration":
				main := Declaration{Commit: "main", Values: [4]string{"cheap", "audit", "15", "full"}}
				candidate := main
				candidate.Commit, candidate.Values[0] = "candidate", "different"
				_, err := AdmitDeclaration(DeclarationAdmission{Check: c, Source: func() (Declaration, Declaration, Declaration, error) { return candidate, main, main, nil }})
				if err != nil {
					t.Fatal(err)
				}
				want = channel.Plain{Decision: "Goal example wants to change its committed checks", IfNothing: "the committed checks stay as they are"}
			case "setting-machine":
				if err := os.MkdirAll(filepath.Join(project, ".git"), 0700); err != nil {
					t.Fatal(err)
				}
				_, err := ApplySetting(Setting{Root: install.Path(), Conf: filepath.Join(project, "metasystem.conf"), Now: now, ProcessAct: ProcessAct{Checkout: project, Key: "launch.max", After: "2"}, Remedy: func(string) string { return "person setting" }})
				if err != nil {
					t.Fatal(err)
				}
				want = channel.Plain{Decision: "This machine wants launch.max set to 2", IfNothing: "the setting stays as it is"}
			default:
				if err := os.WriteFile(install.Path("metasystem.conf"), nil, 0600); err != nil {
					t.Fatal(err)
				}
				elapsed, estimate := 1.0, 1.0
				drift := processmeasure.Decide("example/unit/run", processmeasure.Measures{ObservedAt: now, ElapsedHours: &elapsed, EstimateMinutes: &estimate}, nil)
				want = channel.Plain{Decision: "Goal example went past its process allowance: " + drift.Stops[0].Class, Act: "decide the next scoped process change for goal example", IfNothing: "ordinary admitted work goes on; the next process change waits for you"}
				if name != "drift" {
					drift.Stops[0].Cause = &loopstop.Cause{Kind: "process-change", Name: "original-act"}
				}
				if name == "drift-process-change" {
					act.ID, act.Status, act.Key, act.After = "original-act", "applied", "launch.max", "2"
					if err := save(install, filepath.Join(project, "process", "acts", act.ID+".json"), act); err != nil {
						t.Fatal(err)
					}
					want.Act, want.IfNothing = fmt.Sprintf("metasystem settings unset launch.max --repo %s --undo original-act", project), "the intervention hold stays"
				}
				if err := Observe(project, install, "example", "episode", drift); err != nil {
					t.Fatal(err)
				}
			}
			qs, unread := channel.WalkQuestions(install.Path())
			if len(unread) != 0 || len(qs) != 1 {
				t.Fatalf("questions: %+v %v", qs, unread)
			}
			q, err := channel.ReadQuestion(install.Path(), qs[0].ID)
			if err != nil || q.Plain == nil || *q.Plain != want {
				t.Fatalf("process words: %+v, want %+v, %v", q.Plain, want, err)
			}
		})
	}
}
