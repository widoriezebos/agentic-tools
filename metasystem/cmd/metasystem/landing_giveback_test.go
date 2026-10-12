package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func init() {
	registerIdempotency("landing give-back", idemStateful, "an already returned or pending goal changes nothing", checkGiveBackRepeat)
	registerIdempotency("landing land-anyway", idemStateful, "an identical incident decision changes nothing", checkLandAnywayRepeat)
}

func checkGiveBackRepeat(t *testing.T) {
	t.Helper()
	b, batch := batchVerbFixture(t, "sealed")
	args := []string{"landing", "give-back", batch.Members[0].Goal, "--reason", "continue at the seat", "--json"}
	if code, out := b.run(t, b.seat, args...); code != 0 {
		t.Fatalf("first: %d %s", code, out)
	}
	before := idemTreeDigest(t, b.lane)
	b.now = b.now.Add(time.Minute)
	if code, out := b.run(t, b.seat, args...); code != 0 || !strings.Contains(out, "already returned") {
		t.Fatalf("repeat: %d %s", code, out)
	}
	idemSameTree(t, "repeated give-back", before, idemTreeDigest(t, b.lane))
}

func TestLandingGiveBackStates(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"waiting", "admitted", "merged", "proving-by-lane", "proving-by-person", "returned-pending", "not-in-lane"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			fixtureState := "sealed"
			if strings.HasPrefix(state, "proving") || state == "merged" {
				fixtureState = "running"
			}
			b, original := batchVerbFixture(t, fixtureState)
			goal := original.Members[0].Goal
			if state == "waiting" {
				goal = "shared-state-in-the-ledger"
			}
			if state == "not-in-lane" {
				goal = "absent"
			}
			if state == "merged" {
				helmMust(t, os.Remove(filepath.Join(plain.Dir(b.lane), "running.json")))
				b.owners.landing.plainProve.Alive = func(plain.Running) bool { return false }
			}
			ended := 0
			helmMust(t, os.MkdirAll(filepath.Join(plain.Dir(b.lane), "proofs"), 0700))
			b.owners.landing.plainProve.EndProof = func(r plain.Running) error {
				if state == "proving-by-person" {
					t.Fatal("cancelled a person's proof")
				}
				if r.Attempt != "proof" {
					t.Fatalf("wrong proof: %+v", r)
				}
				data, err := os.ReadFile(filepath.Join(plain.Dir(b.lane), "proofs", "proof.cancel.json"))
				helmMust(t, err)
				if !strings.Contains(string(data), "cancelled: give-back") {
					t.Fatalf("cancel marker: %s", data)
				}
				ended++
				return nil
			}
			if state == "proving-by-person" {
				writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "running.json"), plain.Running{Attempt: "proof", BatchID: original.ID, BatchMembers: original.Members, Person: &plain.ActProvenance{Person: "Wido"}, Pid: 42, Tree: "tree", Since: b.now.Format(time.RFC3339Nano)})
			}
			if state == "returned-pending" {
				_, _, err := plain.ReturnProven(b.lane, goal, "unclassified", "continue at the seat", true, "Wido", b.now, plain.ProveSeams{BudgetHome: t.TempDir(), Person: &plain.ActProvenance{Person: "Wido", CheckedAt: b.now}})
				helmMust(t, err)
				original.Actions = []plain.BatchAction{{Kind: "give-back", Pending: true, With: original.Members, By: "Wido", Reason: "continue at the seat", Person: &plain.ActProvenance{Person: "Wido", CheckedAt: b.now}}}
				writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "batch.json"), original)
			}
			beforeBatch := b.batch(t)
			args := []string{"landing", "give-back", goal, "--reason", "continue at the seat", "--json"}
			code, out := b.run(t, b.seat, args...)
			if code != 0 {
				t.Fatalf("give-back: %d %s", code, out)
			}
			entry, known, err := plain.Latest(b.lane, goal)
			helmMust(t, err)
			if state == "not-in-lane" {
				if known || !strings.Contains(out, "nothing to give back: absent is not in the lane") {
					t.Fatalf("absent: %+v %s", entry, out)
				}
			} else if state == "proving-by-person" {
				if entry.State != plain.StateWaiting || !slices.Equal(beforeBatch.Members, b.batch(t).Members) || ended != 0 || !strings.Contains(out, "proof continues") {
					t.Fatalf("person's proof changed: %+v %s", entry, out)
				}
				before := idemTreeDigest(t, b.lane)
				if code, out := b.run(t, b.seat, args...); code != 0 {
					t.Fatalf("pending repeat: %d %s", code, out)
				}
				idemSameTree(t, "pending give-back", before, idemTreeDigest(t, b.lane))
				b.owners.landing.plainProve.Alive = func(plain.Running) bool { return false }
				// Selection consumes the saved request without another give-back.
				seams := b.owners.landing.plainProve
				seams.Now = func() time.Time { return b.now }
				_, _ = plain.SelectBatch(b.lane, b.lane, b.record, seams)
				entry, _, err = plain.Latest(b.lane, goal)
				helmMust(t, err)
			}
			if state != "not-in-lane" && (entry.State != plain.StateReturned || entry.ReturnOrigin == nil || entry.ReturnOrigin.Person == nil || entry.ReturnOrigin.Person.Person != "Wido" || entry.Reason != "continue at the seat") {
				t.Fatalf("return record: %+v %s", entry, out)
			}
			if state != "waiting" && state != "not-in-lane" && (slices.ContainsFunc(b.batch(t).Members, func(m plain.GoalSHA) bool { return m.Goal == goal }) || b.batch(t).State != plain.BatchPrepared || !b.batch(t).Open) {
				t.Fatalf("goal still selected: %+v", b.batch(t))
			}
			if state == "proving-by-lane" && ended != 1 {
				t.Fatalf("proof not cancelled: %d", ended)
			}
			before := idemTreeDigest(t, b.lane)
			if code, out := b.run(t, b.seat, args...); code != 0 {
				t.Fatalf("repeat: %d %s", code, out)
			}
			idemSameTree(t, "repeated give-back", before, idemTreeDigest(t, b.lane))
		})
	}
}

func TestLandingGiveBackDisagreement(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"main", "other", "own"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			attribution := "another goal"
			if kind == "main" {
				attribution = "main"
			}
			b, batch := batchVerbFixture(t, "sealed")
			member := batch.Members[0]
			writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "results.jsonl"), plain.Result{Attempt: "red", Result: plain.Red, At: b.now.Format(time.RFC3339Nano), Goals: []plain.GoalSHA{member}, Cause: &plain.Cause{Kind: kind, Goal: "another-fixture-goal"}})
			code, out := b.run(t, b.seat, "landing", "give-back", member.Goal, "--reason", "already inspected at the seat", "--by", "Wido", "--json")
			var result intentResult
			helmMust(t, json.Unmarshal([]byte(out), &result))
			expectOutcome(t, "person's return despite main attribution", code, result, intentConfirmed)
			entry, _, err := plain.Latest(b.lane, member.Goal)
			helmMust(t, err)
			if code != 0 || !strings.Contains(entry.Reason, "last check attributed "+attribution) || !strings.Contains(out, "last check attributed "+attribution) {
				t.Fatalf("disagreement missing: %d %+v %s", code, entry, out)
			}
		})
	}
}

func TestLandingGiveBackCLIAdapter(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	engine := os.Getenv("METASYSTEM_WAIT_BINARY")
	if engine == "" {
		t.Fatal("package setup did not rebuild the CLI")
	}
	before := idemTreeDigest(t, b.installation)
	for _, verb := range []string{"give-back", "land-anyway"} {
		for _, form := range []string{"help", "missing reason", "blank reason", "unproved caller"} {
			args := []string{"landing", verb, "fixture-goal"}
			if form == "help" {
				args = []string{"landing", verb, "--help"}
			}
			if form == "unproved caller" {
				args = append(args, "--reason", "fixture decision", "--by", "Wido")
			}
			if form == "blank reason" {
				args = append(args, "--reason", " \t\n ")
			}
			command := exec.CommandContext(t.Context(), engine, args...)
			command.Dir, command.Env = b.checkout, fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
			out, err := command.CombinedOutput()
			if form == "help" {
				if err != nil || !strings.Contains(string(out), "metasystem landing "+verb) {
					t.Fatalf("CLI help: %v %s", err, out)
				}
			} else {
				exit, ok := err.(*exec.ExitError)
				want := 2
				if form == "unproved caller" {
					want = 1
				}
				if !ok || exit.ExitCode() != want {
					t.Fatalf("CLI boundary %v: %v %s", args, err, out)
				}
			}
			t.Logf("rebuilt CLI %v: %v\n%s", args, err, out)
		}
	}
	helmMust(t, os.Remove(lane.RecordPath(b.home)))
	command := exec.CommandContext(t.Context(), engine, "landing", "give-back", "fixture-goal", "--reason", "fixture decision")
	command.Dir, command.Env = b.checkout, fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
	out, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "nothing to give back: fixture-goal is not in the lane") {
		t.Fatalf("CLI no-lane repeat: %v %s", err, out)
	}
	t.Logf("rebuilt CLI give-back with no lane: %v\n%s", err, out)
	idemSameTree(t, "CLI boundary leaves fixture untouched", before, idemTreeDigest(t, b.installation))
}
