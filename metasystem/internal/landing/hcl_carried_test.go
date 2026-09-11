package landing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestHCL37EvaluatorUnavailablePrecedesUnneeded(t *testing.T) {
	ordinaryPass := Observation{Verdict: "pass"}
	sufficient := proofrun.DeliveryJudgment{Sufficient: true}
	tests := []struct {
		name        string
		past        string
		liveFailure string
		want        carriedMatchDecision
	}{
		{name: "word names evaluator failure", past: "evaluator-unavailable", liveFailure: "exit=9", want: carriedMatched},
		{name: "word names another defect", past: "missing-declaration", liveFailure: "exit=9", want: carriedMismatch},
		{name: "no live failure was recorded", past: "evaluator-unavailable", want: carriedUnneeded},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := decideCarriedMatch(test.past, "base", test.liveFailure, ordinaryPass, sufficient, nil)
			if got != test.want {
				t.Fatalf("decision = %s; want %s", got, test.want)
			}
		})
	}
}

func TestHCL51CarriedCounselorAppendBelongsToItsRow(t *testing.T) {
	t.Run("carried landing register", func(t *testing.T) {
		fixture := newObserveFixture(t)
		ref := "01ARZ3NDEKTSV4RRFFQ69G5FAY-seat-a-1a2b3c4d"
		row := goal.HistoryLine{
			At: "2026-09-11T10:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAX-seat-a-1a2b3c4d",
			Verb: "carried", Actor: "seat-a+lineage", Targets: []string{"g"}, Keep: -1, ApprovedRef: ref,
			Reason: "landed commit=" + strings.Repeat("1", 40) + " workspace=" + strings.Repeat("2", 40) +
				" project=" + strings.Repeat("3", 40) + " past=missing-declaration battery=green missing=- failing=- judge=base" +
				" judgeTree=" + strings.Repeat("4", 40) + " judgeDigest=" + strings.Repeat("5", 64) +
				" liveFailure=- ledger=" + strings.Repeat("6", 40) + " by=human:Wido",
		}
		line, err := counselor.CarriedLandingLine(row)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		fixture.write("records/counselor/carried-landings.jsonl", string(encoded)+"\n")
		tree := &goal.TreeGoals{Root: &goal.RootRecord{FormatVersion: "2"}, Live: map[string]*goal.GoalFile{"g": {Id: "g", History: []goal.HistoryLine{row}}}, Done: map[string]*goal.GoalFile{}}
		params := ObserveParams{RepoRoot: fixture.root, CandidateTree: fixture.tree(), Carried: ref}
		if err := candidatePathPolicy(params, tree); err != nil {
			t.Fatalf("equal carried counselor line refused: %v", err)
		}
		fixture.write("records/counselor/unowned.jsonl", "{}\n")
		if err := candidatePathPolicy(ObserveParams{RepoRoot: fixture.root, CandidateTree: fixture.tree(), Carried: ref}, tree); carriageRefusalCode(err) != "record-not-owned" {
			t.Fatalf("new unrelated counselor record = %v; want record-not-owned", err)
		}
	})

	t.Run("human carried accepted risk register", func(t *testing.T) {
		fixture := newObserveFixture(t)
		fixture.git("commit", "--allow-empty", "-qm", "carried fixture",
			"--trailer", "Carry: fixture-word", "--trailer", "Carried-By: human:Wido",
			"--trailer", "Carried-Tree: workspace="+strings.Repeat("1", 40)+" project="+strings.Repeat("2", 40),
			"--trailer", "Carried-Past: missing-declaration", "--trailer", "Carried-Battery: green",
			"--trailer", "Carried-Judge: base tree="+strings.Repeat("3", 40)+" sha256="+strings.Repeat("4", 64),
			"--trailer", "Carried-Ledger: "+strings.Repeat("5", 40),
			"--trailer", "Landing-Provenance: carried opid=fixture-word")
		commit := fixture.git("rev-parse", "HEAD")
		finding := "carried:" + commit
		opid := "01ARZ3NDEKTSV4RRFFQ69G5FAW-seat-a-1a2b3c4d"
		recordedAt := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
		why := "fixture accepts the carried review debt"
		if err := counselor.AppendCarriedAcceptedRisk(fixture.root, counselor.CarriedAcceptedRiskAppend{
			Goal: "g", Finding: finding, By: "Wido", Why: why, OpID: opid, Commit: commit, RecordedAt: recordedAt,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(fixture.root, "records/counselor/accepted-risk-register.jsonl.lock")); !os.IsNotExist(err) {
			t.Fatalf("accepted-risk register lock survived release: %v", err)
		}
		accepted := goal.HistoryLine{At: recordedAt.Format(time.RFC3339), Opid: opid, Verb: "accept-risk", Actor: "seat-a+lineage", Targets: []string{"g"}, Keep: -1, Reason: why}
		tree := &goal.TreeGoals{Root: &goal.RootRecord{FormatVersion: "2"}, Live: map[string]*goal.GoalFile{"g": {
			Id: "g", History: []goal.HistoryLine{accepted}, AcceptedRisks: []goal.AcceptedRiskRecord{{Finding: finding, Chain: goal.HumanCarriedChain, By: "Wido", Opid: opid}},
		}}, Done: map[string]*goal.GoalFile{}}
		if err := candidatePathPolicy(ObserveParams{RepoRoot: fixture.root, CandidateTree: fixture.tree(), Carried: "fixture-word"}, tree); err != nil {
			t.Fatalf("equal human-carried accepted-risk line refused: %v", err)
		}
	})
}

func TestHCL58BaseJudgeFenceOwners(t *testing.T) {
	paths := []string{
		"internal/landing/probe.txt", "internal/goal/probe.txt", "internal/proofrun/probe.txt",
		"internal/testpolicy/probe.txt", "internal/behaviorsurface/probe.txt", "internal/config/probe.txt",
		"internal/refusal/probe.txt", "internal/governance/probe.txt", "internal/humanauthority/probe.txt",
		"internal/fixtureauth/probe.txt", "metasystem.conf", "testing.json",
		"scripts/agents/landing-classes.json", "scripts/agents/path-classes.txt", "scripts/agents/landing-promotion.json",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			fixture := newObserveFixture(t)
			fixture.write(path, "candidate change\n")
			fixture.git("add", path)
			params := ObserveParams{RepoRoot: fixture.root, ProjectTree: fixture.git("write-tree"), Judge: "base"}
			got := baseJudgeFenceRefusal(params, "carried opid=fixture")
			if got == nil || got.Code != "carry-base-judge-blind" || !strings.Contains(got.Refusal, path) {
				t.Fatalf("base judge fence for %s = %+v; want carry-base-judge-blind naming the path", path, got)
			}
		})
	}
}

func TestHCL58GenerationZeroNeedsFixtureAuthority(t *testing.T) {
	word := goal.CarryWord{History: goal.HistoryLine{Verb: "carry", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAZ-mac-a-1a2b3c4d"}}
	production := t.TempDir()
	if got := carryWordAuthorityRefusal(production, word, "carried opid=fixture"); got == nil || got.Code != "carry-word-unproven" {
		t.Fatalf("generation-zero production word = %+v; want carry-word-unproven", got)
	}
	fixture := t.TempDir()
	if err := os.WriteFile(filepath.Join(fixture, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := carryWordAuthorityRefusal(fixture, word, "carried opid=fixture"); got != nil {
		t.Fatalf("fixture-authorized generation-zero word refused: %+v", got)
	}
}

func TestHCL57CarriedMatchAndRedBattery(t *testing.T) {
	refusal := Observation{Verdict: "would-refuse", Code: "missing-declaration"}
	pass := Observation{Verdict: "pass"}
	closedChain := Observation{Verdict: "would-refuse", Code: "chain-test-receipt-refused"}
	tests := []struct {
		name      string
		past      string
		ordinary  Observation
		delivery  proofrun.DeliveryJudgment
		verifyErr error
		want      carriedMatchDecision
	}{
		{name: "code word and sufficient result land", past: "missing-declaration", ordinary: refusal, delivery: proofrun.DeliveryJudgment{Sufficient: true}, want: carriedMatched},
		{name: "code word and insufficient result ask", past: "missing-declaration", ordinary: refusal, delivery: proofrun.DeliveryJudgment{MissingGroups: []string{"alpha"}}, want: carriedMismatch},
		{name: "two failures under one group ask", past: "group:alpha", ordinary: pass, delivery: proofrun.DeliveryJudgment{FailingGroups: []string{"alpha", "beta"}}, want: carriedMismatch},
		{name: "uncovered obligation beside group asks", past: "group:alpha", ordinary: pass, delivery: proofrun.DeliveryJudgment{MissingGroups: []string{"alpha"}, UncoveredObligations: []string{"proof"}}, want: carriedMismatch},
		{name: "discrepancy beside group asks", past: "group:alpha", ordinary: pass, delivery: proofrun.DeliveryJudgment{MissingGroups: []string{"alpha"}, Discrepancies: []string{"tree"}}, want: carriedMismatch},
		{name: "verify error asks battery unverified", past: "group:alpha", ordinary: pass, verifyErr: fmt.Errorf("result unreadable"), want: carriedBatteryUnverified},
		{name: "group word lands when it is the only insufficiency", past: "group:alpha", ordinary: pass, delivery: proofrun.DeliveryJudgment{MissingGroups: []string{"alpha"}}, want: carriedMatched},
		{name: "closed chain lands when named group is its only red", past: "group:alpha", ordinary: closedChain, delivery: proofrun.DeliveryJudgment{FailingGroups: []string{"alpha"}}, want: carriedMatched},
		{name: "group word cannot carry an ordinary missing declaration", past: "group:G", ordinary: refusal, delivery: proofrun.DeliveryJudgment{MissingGroups: []string{"G"}}, want: carriedMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := decideCarriedMatch(test.past, "live", "", test.ordinary, test.delivery, test.verifyErr)
			if got != test.want {
				t.Fatalf("carried match outcome = %s; want %s", got, test.want)
			}
		})
	}
}

func TestHCL33ExpiredReservationNotDebt(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	ref := "01ARZ3NDEKTSV4RRFFQ69G5FAZ-mac-a-1a2b3c4d"
	open := goal.HistoryLine{At: now.Add(-2 * time.Hour).Format(time.RFC3339), Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-mac-a-1a2b3c4d", Verb: "carrying", ApprovedRef: ref,
		Reason: "open workspace=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa project=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb expires=" + now.Add(-time.Hour).Format(time.RFC3339) + " by=human:Wido"}
	tests := []struct {
		name string
		rows []goal.HistoryLine
	}{
		{name: "expired", rows: []goal.HistoryLine{open}},
		{name: "abandoned", rows: []goal.HistoryLine{open, {Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAX-mac-a-1a2b3c4d", Verb: "carrying", ApprovedRef: ref, Reason: "abandoned of=" + open.Opid + ` why="stopped"`}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tree := &goal.TreeGoals{Live: map[string]*goal.GoalFile{"g": {Id: "g", History: test.rows}}, Done: map[string]*goal.GoalFile{}}
			if debt, found, err := goal.CarryDebtAt("", tree, "", "", now); err != nil || found {
				t.Fatalf("closed reservation counted as debt: debt=%+v found=%t err=%v", debt, found, err)
			}
		})
	}
}
