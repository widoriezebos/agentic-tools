package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

func TestIntentUnitCheckDeadlineRetryNamesNextStep(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"build", "proof"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newStopWorkBed(t)
			b.starter.hold = kind
			brief := b.brief("check-retry.md", "Build the unit.\n")
			_, first, _ := stopPublic(t, b, "auto", append([]string{"work", "build", b.id, "retry-check", "--brief", brief, "--lines", "5"}, workCheck...)...)
			run, ok := resultData(t, first)["run"].(string)
			if !ok {
				t.Fatalf("no run: %+v", first)
			}
			runner := &launch.UnitRunner{Root: b.unitRoot}
			record, err := runner.Status(run)
			if err != nil {
				t.Fatal(err)
			}
			round := record.Rounds[0]
			index := 0
			if kind == "proof" {
				index = 1
			}
			step := round.Steps[index]
			state, err := b.manager.Store.StateDir(step.LaunchID)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "build" {
				if err := os.WriteFile(filepath.Join(state, "last-message.txt"), []byte("RESULT: PARTIAL\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			directory := filepath.Join(round.Directory, "builder-check-retry")
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			observation := launch.CheckExecution{ExecutionID: "check-retry", Role: "builder", Steps: []processmeasure.Step{{Parent: step.LaunchID, Start: "2026-10-23T09:00:00Z", Outcome: "deadline"}, {Outcome: "passed"}}}
			data, err := json.Marshal(observation)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "observation.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := b.manager.Store.Update(step.LaunchID, func(r *launch.Record) error {
				code := 0
				r.State = launch.Completed
				if kind == "proof" {
					code = 1
					r.State, r.Reason = launch.Failed, "exit-1"
				}
				r.ExitCode = &code
				r.FinishedAt = b.manager.Now().UTC().Format(time.RFC3339Nano)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if kind == "proof" {
				// The machine retries a unit check's first deadline once; its
				// second deadline is the person's.
				_, retried, _ := stopPublic(t, b, "auto", "work", "wait", "run:"+run)
				record, err = runner.Status(run)
				if err != nil || len(record.Rounds[0].Steps[1].LaunchIDs) != 2 || record.Rounds[0].Steps[1].RetryBy != "machine" || record.Rounds[0].Stop != nil {
					t.Fatalf("no machine retry: %+v record=%+v err=%v", retried, record, err)
				}
				retry := record.Rounds[0].Steps[1].LaunchID
				second := launch.CheckExecution{ExecutionID: "check-machine", Role: "builder", Steps: []processmeasure.Step{{Parent: retry, Start: "2026-10-23T09:30:00Z", Outcome: "deadline"}, {Outcome: "passed"}}}
				data, _ := json.Marshal(second)
				if err := os.MkdirAll(filepath.Join(round.Directory, "builder-check-machine"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(round.Directory, "builder-check-machine", "observation.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := b.manager.Store.Update(retry, func(r *launch.Record) error {
					code := 1
					r.State, r.Reason, r.ExitCode, r.FinishedAt = launch.Failed, "exit-1", &code, b.manager.Now().UTC().Format(time.RFC3339Nano)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			b.starter.hold = ""
			_, held, _ := stopPublic(t, b, "auto", "work", "wait", "run:"+run)
			record, err = runner.Status(run)
			if err != nil || record.Rounds[0].Stop == nil || record.Rounds[0].Stop.Handoff != "stopped deadline" {
				t.Fatalf("hold=%+v record=%+v err=%v", held, record, err)
			}
			questions, unknown := channel.WalkOpenQuestions(b.root())
			retryQuestion := false
			for _, q := range questions {
				if q.UnitStop != nil && slices.Contains(q.UnitStop.AcceptableActs, "work-review-retry") {
					retryQuestion = true
				}
			}
			if len(unknown) > 0 || !retryQuestion {
				t.Fatalf("questions=%+v unknown=%v", questions, unknown)
			}
			before := len(b.starter.launched())
			code, result, stderr := stopPublic(t, b, "auto", "work", "review", "run:"+run, "--reason", "Retry the stopped check", "--by", "Wido")
			sentence := "rerun the retained check with the same inputs and fresh output, without another build"
			if kind == "build" {
				sentence = "start another builder turn with the same brief and inputs, keeping its worktree changes"
			}
			launches := b.starter.launched()
			if code != 0 || len(launches) <= before || launches[before] != kind || !strings.Contains(stderr, "Impact: "+sentence+".") {
				t.Fatalf("retry exit=%d result=%+v stderr=%q launches=%v", code, result, stderr, launches)
			}
			paths, err := filepath.Glob(filepath.Join(b.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("impacts=%v err=%v", paths, err)
			}
			data, err = os.ReadFile(paths[0])
			var impact struct{ Kind, Impact string }
			if err != nil || json.Unmarshal(data, &impact) != nil || impact.Kind != "work-review-retry" || !strings.HasPrefix(impact.Impact, "Impact: "+sentence+".") {
				t.Fatalf("impact=%s err=%v", data, err)
			}
		})
	}
}
