package plain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestLandingStewardAlreadyProvenAllowsLaterRowsAndNextPass(t *testing.T) {
	t.Parallel()
	b, scoped := changedMainStewardFixture(t)
	seedMainProofReuse(t, scoped, "landed batch")
	scoped.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{
			{ID: "first", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: "older-main"}}},
			{ID: "later", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: "older-main"}}},
		}, nil
	}
	calls := 0
	scoped.seams.Steward.Trunk = func(install, checkout string, s ProveSeams) (string, error) {
		calls++
		if calls == 1 {
			outcome, err := stewardTrunk(install, checkout, s)
			if outcome != stewardAlreadyProven || err != nil {
				t.Fatalf("applicable proof = %q, %v", outcome, err)
			}
			return outcome, err
		}
		return "proving main", nil
	}
	run, err := StewardStep(b.home, bedNow, scoped.seams)
	if err != nil || calls != 2 || len(run.Interventions) != 1 || run.Interventions[0].Subject != "later" {
		t.Fatalf("first pass=%+v calls=%d err=%v", run, calls, err)
	}
	entries, err := readLines[Intervention](interventionsPath(b.install))
	if err != nil || len(entries) != 1 || entries[0].Subject != "later" {
		t.Fatalf("invented an intervention: %+v %v", entries, err)
	}
	run, err = StewardStep(b.home, bedNow.Add(time.Minute), scoped.seams)
	if err != nil || calls != 3 || len(run.Interventions) != 1 || run.Interventions[0].Subject != "first" {
		t.Fatalf("next pass=%+v calls=%d err=%v", run, calls, err)
	}
}

func TestLandingStewardAlreadyProvenKeepsWithdrawalAndBound(t *testing.T) {
	t.Parallel()
	b, scoped := changedMainStewardFixture(t)
	seedMainProofReuse(t, scoped, "landed batch")
	b.staleQuestion()
	b.seams = scoped.seams
	b.seams.Steward.Trunk = stewardTrunk
	run := b.step(bedNow)
	if len(b.calls) != 1 || b.calls[0] != "withdraw" || len(run.Interventions) != 1 || run.Interventions[0].Row != "stale-question" || run.Interventions[0].Outcome != stewardAlreadyProven {
		t.Fatalf("withdrawal lost: %+v calls=%v", run, b.calls)
	}
	// Restore the same question to prove that the withdrawal still owns its bound.
	question := b.question
	question.State = "open"
	data, err := json.Marshal(question)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.install, "artifacts", "agents", "channel", "questions", question.ID+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	b.calls = nil
	run = b.step(bedNow.Add(time.Minute))
	if len(b.calls) != 0 || len(run.Interventions) != 0 || !strings.Contains(run.Line, "already acted within 60 minutes") {
		t.Fatalf("withdrawal repeated: %+v calls=%v", run, b.calls)
	}
	entries, err := readLines[Intervention](interventionsPath(b.install))
	if err != nil || len(entries) != 1 || entries[0].Row != "stale-question" {
		t.Fatalf("withdrawal record=%+v err=%v", entries, err)
	}
}

func TestLandingStewardAdmissionDistinguishesCompletionAndLiveJoin(t *testing.T) {
	t.Parallel()
	for _, completed := range []bool{true, false} {
		t.Run(fmt.Sprintf("completed=%v", completed), func(t *testing.T) {
			t.Parallel()
			b, scoped := changedMainStewardFixture(t)
			scoped.seams.ProcessRef = func(pid int64) string { return syntheticTrunkProcess(t, pid) }
			scoped.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
				return []goal.TrunkRedEntry{
					{ID: "first", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: "older-main"}}},
					{ID: "later", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: "older-main"}}},
				}, nil
			}
			calls, launches := 0, 0
			scoped.seams.Launch = func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }
			outcome := ""
			scoped.seams.Steward.Trunk = func(install, checkout string, s ProveSeams) (string, error) {
				calls++
				if calls > 1 {
					return "proving later row", nil
				}
				git, reads := s.Git, 0
				s.Git = func(dir string, args ...string) (string, error) {
					if strings.Join(args, " ") == "rev-parse --verify origin/main^{commit}" {
						reads++
						if reads == 2 {
							if completed {
								savedTrunkProof(t, scoped)
							} else {
								live := s
								live.Git = git
								if running, already, err := Start(install, checkout, live); err != nil || already || running.Reused != nil {
									t.Fatalf("live admission: %+v already=%v err=%v", running, already, err)
								}
							}
						}
					}
					return git(dir, args...)
				}
				var err error
				outcome, err = stewardTrunk(install, checkout, s)
				if reads != 3 {
					t.Fatalf("did not cross steward read and admission: reads=%d", reads)
				}
				return outcome, err
			}
			run, err := StewardStep(b.home, bedNow, scoped.seams)
			if err != nil {
				t.Fatal(err)
			}
			if completed {
				if outcome != stewardAlreadyProven || calls != 2 || launches != 0 || len(run.Interventions) != 1 || run.Interventions[0].Subject != "later" {
					t.Fatalf("completion blocked later row: %+v outcome=%q calls=%d launches=%d", run, outcome, calls, launches)
				}
			} else if outcome == stewardAlreadyProven || calls != 1 || launches != 1 || len(run.Interventions) != 1 || run.Interventions[0].Subject != "first" || !strings.Contains(run.Line, "later; bounded:") {
				t.Fatalf("live join lost proofRunning: %+v outcome=%q calls=%d launches=%d", run, outcome, calls, launches)
			}
		})
	}
}
