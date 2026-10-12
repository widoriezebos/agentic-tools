package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func checkLandAnywayRepeat(t *testing.T) {
	t.Helper()
	b, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	holdIncidentFixture(t, b.intentBed)
	args := []string{"landing", "land-anyway", bedGoal, "--reason", "Ship the independent fix"}
	code, result := b.do(args...)
	expectOutcome(t, "land anyway", code, result, intentConfirmed)
	before := idemTreeDigest(t, install)
	code, result = b.do(args...)
	expectOutcome(t, "repeat land anyway", code, result, intentUnchanged)
	if !strings.Contains(result.Summary, "already admitted") {
		t.Fatalf("repeat: %+v", result)
	}
	idemSameTree(t, "repeated land-anyway", before, idemTreeDigest(t, install))
}

func TestLandingLandAnywayAliasAndDecision(t *testing.T) {
	t.Parallel()
	var decisions []*plain.Exception
	for _, alias := range []bool{false, true} {
		b, state, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
		holdIncidentFixture(t, b.intentBed)
		args := []string{"landing", "land-anyway", bedGoal, "--reason", "Ship the independent fix"}
		if alias {
			args = []string{"work", "land", bedGoal, "--exception", goal.LandTrunkRedCode, "--reason", "Ship the independent fix", "--by", "Wido"}
		}
		code, result := b.do(args...)
		expectOutcome(t, "admit despite main red", code, result, intentConfirmed)
		entry, known, err := plain.Latest(install, bedGoal)
		if err != nil || !known || entry.State != plain.StateWaiting || entry.Exception == nil {
			t.Fatalf("hand-in: %+v %v", entry, err)
		}
		e := entry.Exception
		impact := "This lands " + bedGoal + " while main fails " + holdIncidentID + "; those failures stay open and later goals are still held by them; undo with `landing give-back " + bedGoal + "` before the push, or a revert after."
		if !strings.Contains(result.Summary, impact) {
			t.Fatalf("impact missing from result: %+v", result)
		}
		data, err := json.Marshal(e)
		var record map[string]any
		if err != nil || json.Unmarshal(data, &record) != nil || record["impact"] != impact {
			t.Fatalf("impact missing from exception: %s %v", data, err)
		}
		if e.Reason != "Ship the independent fix" || e.By != "Wido" || e.Goal != bedGoal || e.SHA != state.status.BranchTip || e.Person == nil || e.Person.CheckedAt != syncRequestTestNow || e.Person.Person != "Wido" || e.Incidents == nil || len(e.Incidents.Open) != 1 || e.Incidents.Open[0].Identity != holdIncidentID {
			t.Fatalf("decision lost: %+v", e)
		}
		queue := filepath.Join(plain.Dir(install), "queue.jsonl")
		before, err := os.ReadFile(queue)
		helmMust(t, err)
		code, result = b.do(args...)
		expectOutcome(t, "repeated decision", code, result, intentUnchanged)
		after, err := os.ReadFile(queue)
		helmMust(t, err)
		if string(before) != string(after) {
			t.Fatal("repeat wrote queue")
		}
		copy := *e
		person := *copy.Person
		person.Root, person.Destination.Root, person.Destination.Install = "fixture-seat", "fixture-lane", "fixture-install"
		copy.Person = &person
		decisions = append(decisions, &copy)
	}
	if !reflect.DeepEqual(decisions[0], decisions[1]) {
		t.Fatalf("alias differs: %+v %+v", decisions[0], decisions[1])
	}
}

func TestLandingLandAnywayReasonWhitespace(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{" \t\n ", "  Ship the independent fix \t"} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			b, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
			holdIncidentFixture(t, b.intentBed)
			code, result := b.do("landing", "land-anyway", bedGoal, "--reason", reason)
			entry, known, err := plain.Latest(install, bedGoal)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(reason) == "" {
				if code != 2 || result.Summary != "landing land-anyway needs one goal and --reason TEXT" || known {
					t.Fatalf("empty reason admitted: %d %+v %+v", code, result, entry)
				}
			} else if code != 0 || !known || entry.Exception == nil || entry.Exception.Reason != "Ship the independent fix" {
				t.Fatalf("reason not trimmed: %d %+v %+v", code, result, entry)
			}
		})
	}
}

func TestLandingLandAnywayRemedyAndAuthority(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	holdIncidentFixture(t, b.intentBed)
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "main red remedy", code, result, intentRefused)
	if result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "landing land-anyway "+bedGoal+" --reason TEXT") {
		t.Fatalf("remedy: %+v", result)
	}
	code, result = b.do("landing", "land-anyway", bedGoal, "--reason", "Ship", "--by", "someone else")
	expectOutcome(t, "mismatched person", code, result, intentRefused)
	entries, err := plain.Entries(install)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unproved admission: %+v %v", entries, err)
	}
}
