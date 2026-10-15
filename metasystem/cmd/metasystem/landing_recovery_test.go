package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingPersonReturnKeepsMainDisagreementGitAdapter(t *testing.T) {
	t.Parallel()
	for _, person := range []bool{false, true} {
		t.Run(map[bool]string{false: "agent", true: "person"}[person], func(t *testing.T) {
			t.Parallel()
			b := newPlainVerbBed(t)
			b.cwd = b.checkout
			id := "lane-rehearsal-before-landing"
			sha := b.seat(t, id)
			b.merge(t, sha)
			raw, err := os.ReadFile("testdata/landing-main-attribution.json")
			helmMust(t, err)
			var proof plain.Result
			helmMust(t, json.Unmarshal(raw, &proof))
			if proof.Cause.Kind != "main" || proof.Attempt != "20261011T235626.846906000Z" {
				t.Fatalf("saved attribution: %+v", proof)
			}
			writeCauseProof(t, b.installation, "results.jsonl", proof)
			stop := plain.Stop{Loop: "lane-return", Subject: id, Tree: sha, Decision: "stop", Handoff: "ask lane", At: laneTestNow.Format(time.RFC3339), Required: []string{"metasystem", "landing", "return", id, "--cause", "own"}}
			data, err := json.Marshal(stop)
			helmMust(t, err)
			helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.installation), "stops.jsonl"), append(data, '\n'), 0600))
			fix := &plain.Fix{Goal: id, Attempt: "fix", Round: 1, State: "building", Job: "job", Units: []string{"cmd/metasystem"}}
			helmMust(t, plain.WriteFix(b.installation, fix))
			if person {
				b.owners.prove = enrolledPersonProver(t, b.installation, laneTestNow)
			}
			code, out := b.run(t, "landing", "return", id, "--cause", "own", "--reason", "repair at the seat")
			entry, ok, err := plain.Latest(b.installation, id)
			helmMust(t, err)
			if !person {
				if code == 0 || !ok || entry.State != plain.StateWaiting || !strings.Contains(out, "main itself is red") {
					t.Fatalf("agent return: %d %s %+v", code, out, entry)
				}
				return
			}
			want := "the lane's last check attributed main at "
			if code != 0 || !ok || entry.State != plain.StateReturned || entry.Cause.Kind != "own" || entry.ReturnOrigin.Person == nil || !strings.Contains(entry.Reason, want) || !strings.Contains(entry.Reason, "the person returned the goal as own") {
				t.Fatalf("person return: %d %s %+v", code, out, entry)
			}
			stops, err := plain.OpenStops(b.installation)
			helmMust(t, err)
			active, err := plain.ReadFix(b.installation)
			helmMust(t, err)
			pending, err := plain.Pending(b.installation, b.checkout)
			helmMust(t, err)
			if len(stops) != 0 || active != nil || len(pending) != 0 {
				t.Fatalf("return left work active: stops=%+v fix=%+v pending=%+v", stops, active, pending)
			}
			code, out = b.run(t, "landing", "status", "--verbose")
			if code != 0 || !strings.Contains(oneSpaced(out), want) {
				t.Fatalf("status lost disagreement: %d %s", code, out)
			}
			before := idemTreeDigest(t, plain.Dir(b.installation))
			code, out = b.run(t, "landing", "return", id, "--cause", "own", "--reason", "repeat")
			if code != 0 {
				t.Fatalf("repeat: %d %s", code, out)
			}
			idemSameTree(t, "repeat person's return", before, idemTreeDigest(t, plain.Dir(b.installation)))
		})
	}
}

func TestLandingKeeperProvesMovedMainBeforeExceptionGitAdapter(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"open question", "no question", "records only", "same main", "paused", "launch fails", "red", "green"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newPlainVerbBed(t)
			b.cwd = b.checkout
			b.setCommand(t, "printf 'LANDING-CHECKED\\t0\\n'")
			b.seat(t, "waiting")
			register := newIntentBed(t, false, nil)
			entries := holdIncidentFixture(t, register)
			entries[0].Identity = "red:unit:TestBroken"
			entries[0].ID = entries[0].Identity
			entries[0].Sightings[0].BaseCommit = b.main
			register.repo.commit(register.repo.canonical).files["plans/goals/trunk-red.json"] = goal.RenderTrunkRed(entries)
			helmMust(t, os.MkdirAll(filepath.Join(b.installation, "plans/goals"), 0755))
			helmMust(t, os.WriteFile(filepath.Join(b.installation, "plans/goals/trunk-red.json"), goal.RenderTrunkRed(entries), 0600))
			b.git(t, b.checkout, "add", "metasystem/plans/goals/trunk-red.json")
			b.git(t, b.checkout, "commit", "--quiet", "-m", "fixture incident")
			b.git(t, b.checkout, "push", "--quiet", "origin", "main")
			b.git(t, b.checkout, "fetch", "--quiet", "origin")
			b.main = b.git(t, b.checkout, "rev-parse", "HEAD")
			initial := b.main
			owners := incidentRecorderFixture(t, register, "lane-machine")
			b.owners.landing.mainEndpoint, b.owners.landing.machine = owners.mainEndpoint, owners.machine
			b.owners.landing.now = func() time.Time { return laneTestNow }
			seams := plain.ProveSeams{Now: func() time.Time { return laneTestNow }, BudgetHome: b.home,
				Lane: func() (lane.Record, error) { r, _, err := lane.Read(b.home); return r, err },
				Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) {
					es, problems := goal.ParseTrunkRed(register.repo.commit(register.repo.canonical).files["plans/goals/trunk-red.json"])
					if len(problems) > 0 {
						return nil, fmt.Errorf("bad incident fixture: %v", problems)
					}
					return es, nil
				},
				Alive: func(plain.Running) bool { return true }, Executable: func() (string, error) { return "fixture-engine", nil },
			}
			machine := func(string) (string, error) { return "lane-machine", nil }
			helmMust(t, plain.SyncPolicyQuestion(b.installation, machine, laneTestNow, seams))
			questions, _ := channel.WalkOpenQuestions(b.installation)
			if len(questions) != 1 {
				t.Fatalf("initial exception: %+v", questions)
			}
			old := questions[0]
			if scenario == "no question" {
				_, err := channel.Withdraw(b.installation, old.ID, "fixture no question", nil, channel.DestinationConfig{})
				helmMust(t, err)
			}
			if scenario == "records only" {
				b.advanceMain(t, "metasystem/plans/page.md", "record\n")
			} else if scenario != "same main" {
				b.setCommand(t, "printf 'LANDING-CHECKED\\t0\\n'; : fixed")
			}
			if scenario == "paused" {
				_, err := lane.SetPause(b.home, "Wido", laneTestNow)
				helmMust(t, err)
			}
			launches := 0
			seams.Launch = func(argv []string, dir, log string) (int64, error) {
				launches++
				if !strings.Contains(strings.Join(argv, " "), "landing prove --wait --attempt") || !strings.Contains(strings.Join(argv, " "), "--trunk") || dir != b.checkout {
					t.Fatalf("wrong detached request: %v %s", argv, dir)
				}
				if scenario == "launch fails" {
					return 0, errors.New("fixture launch failed")
				}
				return int64(os.Getpid()), nil
			}
			b.owners.landing.plainProve = seams
			keeper := newLandingAgentKeeper(b.checkout, b.home, newTestLandingAgent(func(a *landingAgent) { a.now = seams.Now; a.machine = machine; a.proofEffects = seams }))
			keeper.Prepare = nil
			keeper.Running = func() (string, bool, error) { return "", false, nil }
			keeper.Start = func(string, lane.Wake) (string, error) { t.Fatal("agent started before trunk result"); return "", nil }
			keeper.Fingerprint, keeper.BarrenStop, keeper.Waiting = nil, nil, nil
			run := keeper.Run()
			expected := scenario != "paused"
			if (launches == 1) != expected {
				t.Fatalf("tick: %+v launches=%d initial=%s main=%s", run, launches, initial, b.main)
			}
			if !expected {
				return
			}
			if scenario == "launch fails" {
				if !strings.Contains(strings.Join(run.Problems, " "), "fixture launch failed") {
					t.Fatalf("launch failure hidden: %+v", run)
				}
				return
			}
			current, err := channel.ReadQuestion(b.installation, old.ID)
			helmMust(t, err)
			wantClosure := "main moved to " + b.main + "; proving it again"
			if scenario == "records only" || scenario == "same main" {
				wantClosure = "superseded by current lane state"
			}
			if scenario != "no question" && (current.State != "closed" || current.ClosedBecause != wantClosure) {
				t.Fatalf("stale question: %+v", current)
			}
			if qs, _ := channel.WalkOpenQuestions(b.installation); len(qs) != 0 {
				t.Fatalf("asked before proof: %+v", qs)
			}
			running, recorded, _, err := plain.ReadRunning(b.installation, seams)
			helmMust(t, err)
			wantCommand := "printf 'LANDING-CHECKED\\t0\\n'; : fixed"
			if scenario == "records only" || scenario == "same main" {
				wantCommand = "printf 'LANDING-CHECKED\\t0\\n'"
			}
			if !recorded || !running.Trunk || running.Commit != b.main || running.Admission.Command != wantCommand {
				t.Fatalf("wrong admission: %+v", running)
			}
			if scenario != "red" && scenario != "green" {
				keeper.Run()
				if launches != 1 {
					t.Fatalf("repeat tick launched %d proofs", launches)
				}
				return
			}
			register.repo.commits[b.main] = register.repo.commit(register.repo.canonical)
			if scenario == "red" {
				writeCauseProof(t, b.installation, "results.jsonl", plain.Result{Trunk: true, Commit: b.main, Tree: running.Tree, Result: plain.Red, At: laneTestNow.Format(time.RFC3339), Cause: &plain.Cause{Kind: "main"}})
				helmMust(t, os.Remove(filepath.Join(plain.Dir(b.installation), "running.json")))
			} else {
				code, out := b.run(t, "landing", "prove", "--trunk", "--wait", "--attempt", running.Attempt, "--json")
				if code != 0 {
					t.Fatalf("complete recovery proof: %d %s", code, out)
				}
				es, problems := goal.ParseTrunkRed(register.repo.commit(register.repo.canonical).files["plans/goals/trunk-red.json"])
				if len(problems) > 0 || len(es) != 1 || es[0].Closed == nil {
					t.Fatalf("green did not clear incidents: %+v %v", es, problems)
				}
			}
			helmMust(t, plain.SyncPolicyQuestion(b.installation, machine, laneTestNow, seams))
			qs, _ := channel.WalkOpenQuestions(b.installation)
			if (len(qs) == 1) != (scenario == "red") {
				t.Fatalf("result %s questions: %+v", scenario, qs)
			}
		})
	}
}
