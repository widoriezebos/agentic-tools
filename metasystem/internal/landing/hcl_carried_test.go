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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func carriedFixtureMessage(word string) []byte {
	return []byte(strings.Join([]string{
		"carried fixture", "", "Carry: " + word, "Carried-By: human:Wido",
		"Carried-Tree: workspace=" + strings.Repeat("1", 40) + " project=" + strings.Repeat("2", 40),
		"Carried-Past: missing-declaration", "Carried-Battery: green",
		"Carried-Judge: base tree=" + strings.Repeat("3", 40) + " sha256=" + strings.Repeat("4", 64),
		"Carried-Ledger: " + strings.Repeat("5", 40),
		"Landing-Provenance: carried opid=" + word, "",
	}, "\n"))
}

func strictCarriedFixtureMessage(t *testing.T, root, commit string, message []byte) func(string, string) ([]byte, error) {
	t.Helper()
	return func(gotRoot, gotCommit string) ([]byte, error) {
		t.Helper()
		if gotRoot != root || gotCommit != commit {
			t.Fatalf("commit message requested for root=%q commit=%q; want root=%q commit=%q", gotRoot, gotCommit, root, commit)
		}
		return append([]byte(nil), message...), nil
	}
}

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
	policyFacts := func(c *observationCase) observationFacts {
		resolver := stateroot.NewResolver(func(path string) (string, error) {
			if path != c.fixture.root {
				c.fixture.t.Fatalf("unexpected repository-top lookup %q", path)
			}
			return c.fixture.repository, nil
		}, func() (string, error) {
			c.fixture.t.Fatal("owner resolver requested executable path")
			return "", fmt.Errorf("unexpected executable lookup")
		})
		return observationFacts{reader: c, installation: c.fixture.root, ownerForInstallation: resolver.OwnerForInstallation}
	}
	t.Run("carried landing register", func(t *testing.T) {
		fixture := newRepositoryObservationFixture(t)
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
		const path = "records/counselor/carried-landings.jsonl"
		appended := string(encoded) + "\n"
		caseOne := fixture.comparison(observeTreeB, "", path)
		caseOne.declare(path, nil, &appended)
		tree := &goal.TreeGoals{Root: &goal.RootRecord{FormatVersion: "2"}, Live: map[string]*goal.GoalFile{"g": {Id: "g", History: []goal.HistoryLine{row}}}, Done: map[string]*goal.GoalFile{}}
		params := ObserveParams{RepoRoot: fixture.root, CandidateTree: observeTreeB, Carried: ref}
		if err := candidatePathPolicyWithFacts(params, tree, policyFacts(caseOne), nil); err != nil {
			t.Fatalf("equal carried counselor line refused: %v", err)
		}
		caseTwo := fixture.comparison(observeTreeB, "", path, "records/counselor/unowned.jsonl")
		caseTwo.declare(path, nil, &appended)
		unowned := "{}\n"
		caseTwo.declare("records/counselor/unowned.jsonl", nil, &unowned)
		if err := candidatePathPolicyWithFacts(params, tree, policyFacts(caseTwo), nil); carriageRefusalCode(err) != "record-not-owned" {
			t.Fatalf("new unrelated counselor record = %v; want record-not-owned", err)
		}
	})

	t.Run("human carried accepted risk register", func(t *testing.T) {
		fixture := newRepositoryObservationFixture(t)
		commit := strings.Repeat("a", 40)
		commitMessage := strictCarriedFixtureMessage(t, fixture.root, commit, carriedFixtureMessage("fixture-word"))
		finding := "carried:" + commit
		opid := "01ARZ3NDEKTSV4RRFFQ69G5FAW-seat-a-1a2b3c4d"
		recordedAt := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
		why := "fixture accepts the carried review debt"
		if err := counselor.AppendCarriedAcceptedRisk(fixture.root, counselor.CarriedAcceptedRiskAppend{
			Goal: "g", Finding: finding, By: "Wido", Why: why, OpID: opid, Commit: commit, RecordedAt: recordedAt, CommitMessage: commitMessage,
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
		const path = "records/counselor/accepted-risk-register.jsonl"
		appended, err := os.ReadFile(filepath.Join(fixture.root, path))
		if err != nil {
			t.Fatal(err)
		}
		caseOne := fixture.comparison(observeTreeB, "", path)
		appendedText := string(appended)
		caseOne.declare(path, nil, &appendedText)
		if err := candidatePathPolicyWithFacts(ObserveParams{RepoRoot: fixture.root, CandidateTree: observeTreeB, Carried: "fixture-word"}, tree, policyFacts(caseOne), commitMessage); err != nil {
			t.Fatalf("equal human-carried accepted-risk line refused: %v", err)
		}
	})
}

func TestAbandonedCounselorAppendsKeepOwnership(t *testing.T) {
	const carriedPath = "records/counselor/carried-landings.jsonl"
	const riskPath = "records/counselor/accepted-risk-register.jsonl"
	type ownershipFixture struct {
		fixture     *repositoryObservationFixture
		reader      *observationCase
		params      ObserveParams
		tree        *goal.TreeGoals
		owner       *goal.GoalFile
		carriedLine counselor.CarriedLanding
		message     func(string, string) ([]byte, error)
	}
	newFixture := func(t *testing.T, carriedLine, riskLine bool) ownershipFixture {
		t.Helper()
		fixture := newRepositoryObservationFixture(t)
		commit := strings.Repeat("a", 40)
		message := strictCarriedFixtureMessage(t, fixture.root, commit, carriedFixtureMessage("earlier-word"))
		carried := goal.HistoryLine{
			At: "2026-09-13T10:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-seat-a-1a2b3c4d",
			Verb: "carried", Actor: "seat-a+lineage", Targets: []string{"owner"}, Keep: -1,
			ApprovedRef: "01ARZ3NDEKTSV4RRFFQ69G5FAU-seat-a-1a2b3c4d",
			Reason: "landed commit=" + strings.Repeat("6", 40) + " workspace=" + strings.Repeat("7", 40) +
				" project=" + strings.Repeat("8", 40) + " past=missing-declaration battery=green missing=- failing=- judge=base" +
				" judgeTree=" + strings.Repeat("9", 40) + " judgeDigest=" + strings.Repeat("a", 64) +
				" liveFailure=- ledger=" + strings.Repeat("b", 40) + " by=human:Wido",
		}
		encodedCarried, err := counselor.CarriedLandingLine(carried)
		if err != nil {
			t.Fatal(err)
		}
		acceptedAt := time.Date(2026, 9, 13, 10, 5, 0, 0, time.UTC)
		acceptOpID := "01ARZ3NDEKTSV4RRFFQ69G5FAT-seat-a-1a2b3c4d"
		finding := "carried:" + commit
		why := "the bounded carried risk is accepted"
		accepted := goal.HistoryLine{At: acceptedAt.Format(time.RFC3339), Opid: acceptOpID, Verb: "accept-risk", Actor: "seat-a+lineage", Targets: []string{"owner"}, Keep: -1, Reason: why}
		owner := &goal.GoalFile{
			Id: "owner", State: goal.StateAbandoned, History: []goal.HistoryLine{carried, accepted},
			AcceptedRisks: []goal.AcceptedRiskRecord{{Finding: finding, Chain: goal.HumanCarriedChain, By: "Wido", Opid: acceptOpID}},
		}
		laterWord := goal.HistoryLine{
			At: "2026-09-13T11:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAS-seat-b-2b3c4d5e", Verb: "carry",
			Actor: "seat-b+lineage", Targets: []string{"later"}, Keep: -1, ApprovedRef: "later-word",
			Reason: "workspace=" + strings.Repeat("c", 40) + " past=missing-declaration expires=2026-09-13T12:00:00Z why=fixture by=human:Wido",
		}
		tree := &goal.TreeGoals{
			Root: &goal.RootRecord{FormatVersion: "2"},
			Live: map[string]*goal.GoalFile{"later": {Id: "later", State: goal.StateClaimed, History: []goal.HistoryLine{laterWord}}},
			Done: map[string]*goal.GoalFile{}, Abandoned: map[string]*goal.GoalFile{"owner": owner},
		}
		changed := []string{}
		appendedCarried := ""
		if carriedLine {
			encoded, marshalErr := json.Marshal(encodedCarried)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			appendedCarried = string(encoded) + "\n"
			changed = append(changed, carriedPath)
		}
		appendedRisk := ""
		if riskLine {
			if err := counselor.AppendCarriedAcceptedRisk(fixture.root, counselor.CarriedAcceptedRiskAppend{
				Goal: owner.Id, Finding: finding, By: "Wido", Why: why, OpID: acceptOpID, Commit: commit, RecordedAt: acceptedAt, CommitMessage: message,
			}); err != nil {
				t.Fatal(err)
			}
			encoded, err := os.ReadFile(filepath.Join(fixture.root, riskPath))
			if err != nil {
				t.Fatal(err)
			}
			appendedRisk = string(encoded)
			changed = append(changed, riskPath)
		}
		reader := fixture.comparison(observeTreeB, "", changed...)
		if carriedLine {
			reader.declare(carriedPath, nil, &appendedCarried)
		}
		if riskLine {
			reader.declare(riskPath, nil, &appendedRisk)
		}
		return ownershipFixture{
			fixture: fixture, reader: reader, params: ObserveParams{RepoRoot: fixture.root, CandidateTree: observeTreeB, Carried: "later-word"},
			tree: tree, owner: owner, carriedLine: encodedCarried, message: message,
		}
	}
	validate := func(f ownershipFixture) error {
		resolver := stateroot.NewResolver(func(path string) (string, error) {
			if path != f.fixture.root {
				f.fixture.t.Fatalf("unexpected repository-top lookup %q", path)
			}
			return f.fixture.repository, nil
		}, func() (string, error) {
			f.fixture.t.Fatal("owner resolver requested executable path")
			return "", fmt.Errorf("unexpected executable lookup")
		})
		facts := observationFacts{reader: f.reader, installation: f.fixture.root, ownerForInstallation: resolver.OwnerForInstallation}
		return candidatePathPolicyWithFacts(f.params, f.tree, facts, f.message)
	}
	wantCode := func(t *testing.T, got error, code string) {
		t.Helper()
		if carriageRefusalCode(got) != code {
			t.Fatalf("carried candidate error = %v; want %s", got, code)
		}
	}

	for _, test := range []struct {
		name          string
		carried, risk bool
	}{{"carried register alone", true, false}, {"accepted-risk register alone", false, true}, {"both registers", true, true}} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFixture(t, test.carried, test.risk)
			if err := validate(fixture); err != nil {
				t.Fatalf("abandoned owner refused: %v", err)
			}
		})
	}

	for _, state := range []string{"live", "done", "abandoned"} {
		t.Run("ownership in "+state, func(t *testing.T) {
			fixture := newFixture(t, true, true)
			delete(fixture.tree.Abandoned, fixture.owner.Id)
			switch state {
			case "live":
				fixture.owner.State = goal.StateClaimed
				fixture.tree.Live[fixture.owner.Id] = fixture.owner
			case "done":
				fixture.owner.State = goal.StateDone
				fixture.tree.Done[fixture.owner.Id] = fixture.owner
			case "abandoned":
				fixture.owner.State = goal.StateAbandoned
				fixture.tree.Abandoned[fixture.owner.Id] = fixture.owner
			}
			if err := validate(fixture); err != nil {
				t.Fatalf("%s owner refused: %v", state, err)
			}
		})
	}

	t.Run("missing carried row", func(t *testing.T) {
		fixture := newFixture(t, true, false)
		fixture.owner.History = fixture.owner.History[1:]
		wantCode(t, validate(fixture), "record-not-owned")
	})
	t.Run("changed carried facts", func(t *testing.T) {
		fixture := newFixture(t, true, false)
		fixture.carriedLine.Goal = "later"
		encoded, err := json.Marshal(fixture.carriedLine)
		if err != nil {
			t.Fatal(err)
		}
		changed := string(encoded) + "\n"
		fixture.reader = fixture.fixture.comparison(observeTreeB, "", carriedPath)
		fixture.reader.declare(carriedPath, nil, &changed)
		wantCode(t, validate(fixture), "record-not-owned")
	})
	t.Run("non-human-carried waiver", func(t *testing.T) {
		fixture := newFixture(t, false, true)
		fixture.owner.AcceptedRisks[0].Chain = "critic-chain"
		wantCode(t, validate(fixture), "record-not-owned")
	})
	t.Run("existing line is rewritten", func(t *testing.T) {
		fixture := newFixture(t, true, false)
		original, err := json.Marshal(fixture.carriedLine)
		if err != nil {
			t.Fatal(err)
		}
		before := string(original) + "\n"
		fixture.carriedLine.Goal = "later"
		encoded, err := json.Marshal(fixture.carriedLine)
		if err != nil {
			t.Fatal(err)
		}
		after := string(encoded) + "\n"
		fixture.reader = fixture.fixture.comparison(observeTreeB, "", carriedPath)
		fixture.reader.declare(carriedPath, &before, &after)
		wantCode(t, validate(fixture), "register-carriage-not-append-only")
	})
}

func TestHCL58BaseJudgeFenceOwners(t *testing.T) {
	paths := []string{
		"internal/landing/probe.txt", "internal/goal/probe.txt", "internal/proofrun/probe.txt",
		"internal/testpolicy/probe.txt", "internal/behaviorsurface/probe.txt", "internal/config/probe.txt",
		"internal/refusal/probe.txt", "internal/governance/probe.txt", "internal/humanauthority/probe.txt",
		"internal/fixtureauth/probe.txt", "metasystem.conf", "testing.json",
		"scripts/agents/landing-classes.json", "scripts/agents/path-classes.txt",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			projectTree := strings.Repeat("b", 40)
			params := ObserveParams{RepoRoot: root, ProjectTree: projectTree, Judge: "base"}
			got := baseJudgeFenceRefusalWithSources(params, "carried opid=fixture", func(gotRoot, gotTree string) ([]byte, error) {
				if gotRoot != root || gotTree != projectTree {
					t.Fatalf("diff-tree requested for root=%q tree=%q", gotRoot, gotTree)
				}
				return []byte("metasystem/" + path + "\n"), nil
			}, func(gotRoot string) (string, error) {
				if gotRoot != root {
					t.Fatalf("prefix requested for root=%q", gotRoot)
				}
				return "metasystem/", nil
			})
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
