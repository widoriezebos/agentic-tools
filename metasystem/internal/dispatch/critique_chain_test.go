package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chainState(records map[string]map[string]any) critiqueState {
	return critiqueState{agents: "unused", records: records}
}

func TestChainRootVerdicts(t *testing.T) {
	state := chainState(map[string]map[string]any{
		"root": {"jobId": "root"}, "child": {"jobId": "child", "parentJob": "root"},
		"grand": {"jobId": "grand", "parentJob": "child"},
		"x":     {"jobId": "x", "parentJob": "y"}, "y": {"jobId": "y", "parentJob": "x"},
		"dangling": {"jobId": "dangling", "parentJob": "missing"},
		"bad":      {"jobId": "bad", "parentJob": 42},
	})
	if got := state.chainRoot("grand"); got != "root" {
		t.Fatalf("grandchild root = %q", got)
	}
	if got := state.chainRoot("root"); got != "root" {
		t.Fatalf("root = %q", got)
	}
	for _, id := range []string{"x", "dangling", "bad", "absent"} {
		if got := state.chainRoot(id); got != "" {
			t.Fatalf("invalid chain %s resolved to %q", id, got)
		}
	}
}

func TestLatestMemberSelection(t *testing.T) {
	state := chainState(map[string]map[string]any{
		"root": {"jobId": "root", "round": json.Number("1")},
		"r3":   {"jobId": "r3", "parentJob": "root", "round": json.Number("3")},
		"r2":   {"jobId": "r2", "parentJob": "root", "round": json.Number("2")},
	})
	if got := state.latestMember("root"); got == nil || got["jobId"] != "r3" {
		t.Fatalf("latest = %v", got)
	}
	if got := state.latestMember("absent"); got != nil {
		t.Fatalf("absent latest = %v", got)
	}
}

func writeCapRound(t *testing.T, repo, root, role string, round int, protocolError bool, findings, rigor []any) string {
	t.Helper()
	job := root
	parent := any(nil)
	if round > 1 {
		job = fmt.Sprintf("%s-r%d", root, round)
		if round == 2 {
			parent = root
		} else {
			parent = fmt.Sprintf("%s-r%d", root, round-1)
		}
	}
	record := map[string]any{"jobId": job, "role": role, "round": round, "parentJob": parent, "status": "completed"}
	if round == 1 {
		record[findingRegisterField] = []any{}
		record[findingRegisterRoundField] = 0
		record[reviewRoundLimitField] = 3
		record[criticRoundsConsumedField] = 0
		record["demotions"] = []any{}
		record["critiqueExhaustions"] = []any{}
	}
	if protocolError {
		record["status"] = "failed"
		record["error"] = "protocol_error"
		record["protocolError"] = map[string]any{"key": fmt.Sprintf("key-%d", round), "violation": "malformed critic return"}
	}
	agents := filepath.Join(repo, "artifacts", "agents")
	writeJSONFile(t, filepath.Join(agents, "jobs"), job+".json", record)
	if !protocolError {
		writeJSONFile(t, filepath.Join(agents, root, "rounds", fmt.Sprint(round)), "return.json", map[string]any{
			"schemaVersion": 4, "jobId": job, "round": round, "findings": findings, "rigor": rigor,
		})
	}
	if outcome, err := CritiqueRegisterAdvance(repo, root, job); err != nil || outcome != "advanced" {
		t.Fatalf("advance %s = %q, %v", job, outcome, err)
	}
	return job
}

func TestBoundaryAtThreeRounds(t *testing.T) {
	repo := t.TempDir()
	writeCapRound(t, repo, "critic", "design-critic", 1, false,
		[]any{registerFindingValue("B-1", true, "bounded evidence")}, []any{registerRigor("B-1", "bounded")})
	writeCapRound(t, repo, "critic", "design-critic", 2, false, []any{}, []any{})
	writeCapRound(t, repo, "critic", "design-critic", 3, false, []any{}, []any{})
	root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
	state, err := readCritiqueCapState(repo, loadCritiqueState(repo), "critic", root)
	if err != nil {
		t.Fatal(err)
	}
	if state.boundary != boundedTerminalExhaustion || state.round != 3 {
		t.Fatalf("round-three state = %+v", state)
	}
}

func TestCompletedAndFailedRoundsCountButCancelledDoesNot(t *testing.T) {
	repo := t.TempDir()
	writeCapRound(t, repo, "critic", "design-critic", 1, false, []any{}, []any{})
	writeCapRound(t, repo, "critic", "design-critic", 2, true, nil, nil)
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
	root := readJSONFile(t, rootPath)
	if got, _ := numInt(root[criticRoundsConsumedField]); got != 2 {
		t.Fatalf("consumed = %d", got)
	}
	writeJSONFile(t, filepath.Dir(rootPath), "critic-r3.json", map[string]any{
		"jobId": "critic-r3", "role": "design-critic", "round": 3, "parentJob": "critic-r2", "status": "cancelled",
	})
	if outcome, err := CritiqueRegisterAdvance(repo, "critic", "critic-r3"); err != nil || outcome != "advanced" {
		t.Fatalf("cancelled fold = %q, %v", outcome, err)
	}
	root = readJSONFile(t, rootPath)
	if got, _ := numInt(root[criticRoundsConsumedField]); got != 2 {
		t.Fatalf("cancel changed consumed to %d", got)
	}
}

func TestCritiqueExhaustionAdvanceBackfillsLegacyRoundAccounting(t *testing.T) {
	repo := t.TempDir()
	writeCapRound(t, repo, "critic", "design-critic", 1, false,
		[]any{registerFindingValue("S-legacy", true, "severe evidence")}, []any{registerRigor("S-legacy", "severe")})
	writeCapRound(t, repo, "critic", "design-critic", 2, false, []any{}, []any{})
	writeCapRound(t, repo, "critic", "design-critic", 3, false, []any{}, []any{})
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
	root := readJSONFile(t, rootPath)
	delete(root, reviewRoundLimitField)
	delete(root, criticRoundsConsumedField)
	if err := writeRecord(rootPath, root); err != nil {
		t.Fatal(err)
	}
	message := filepath.Join(t.TempDir(), "message.md")
	if err := os.WriteFile(message, []byte("Address S-legacy.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r4")
	if err == nil || !strings.Contains(err.Error(), "review-round limit is exhausted") || strings.Contains(err.Error(), "malformed round accounting") {
		t.Fatalf("legacy exhaustion advance = %v", err)
	}
}

func TestCritiqueExhaustionRefusesMalformedHistory(t *testing.T) {
	repo := t.TempDir()
	writeCapRound(t, repo, "critic", "design-critic", 1, false, []any{}, []any{})
	message := filepath.Join(t.TempDir(), "message.md")
	if err := os.WriteFile(message, []byte("continue\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")

	root := readJSONFile(t, rootPath)
	root["critiqueExhaustions"] = "malformed"
	if err := writeRecord(rootPath, root); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r2"); err == nil ||
		!strings.Contains(err.Error(), "critiqueExhaustions is malformed") {
		t.Fatalf("malformed exhaustion history = %v", err)
	}

	root["critiqueExhaustions"] = []any{map[string]any{}, map[string]any{}}
	if err := writeRecord(rootPath, root); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r2"); err == nil ||
		!strings.Contains(err.Error(), secondExhaustionRefused) {
		t.Fatalf("second exhaustion history = %v", err)
	}

	root["critiqueExhaustions"] = []any{"malformed"}
	if err := writeRecord(rootPath, root); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r2"); err == nil ||
		!strings.Contains(err.Error(), secondExhaustionRefused) {
		t.Fatalf("non-object exhaustion history = %v", err)
	}

	root["critiqueExhaustions"] = []any{}
	if err := writeRecord(rootPath, root); err != nil {
		t.Fatal(err)
	}
	if _, err := CritiqueExhaustionAdvance(repo, "critic", "observer", message, "critic-r2"); err == nil ||
		!strings.Contains(err.Error(), "no rule for role observer") {
		t.Fatalf("unknown critique role = %v", err)
	}
}
