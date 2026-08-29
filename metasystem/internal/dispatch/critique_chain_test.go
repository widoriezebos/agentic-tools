package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The critique chain helpers, driven directly: the lineage walk's
// verdicts and the highest-round member selection.

func chainState(records map[string]map[string]any) critiqueState {
	return critiqueState{agents: "unused", records: records}
}

func TestChainRootVerdicts(t *testing.T) {
	state := chainState(map[string]map[string]any{
		"root-a":  {"jobId": "root-a"},
		"child-b": {"jobId": "child-b", "parentJob": "root-a"},
		"grand-c": {"jobId": "grand-c", "parentJob": "child-b"},
		"cyc-x":   {"jobId": "cyc-x", "parentJob": "cyc-y"},
		"cyc-y":   {"jobId": "cyc-y", "parentJob": "cyc-x"},
		"dangler": {"jobId": "dangler", "parentJob": "not-loaded"},
		"badtype": {"jobId": "badtype", "parentJob": 42},
	})
	if got := state.chainRoot("grand-c"); got != "root-a" {
		t.Fatalf("three-deep walk: %q", got)
	}
	if got := state.chainRoot("root-a"); got != "root-a" {
		t.Fatalf("a root is its own root: %q", got)
	}
	if got := state.chainRoot("cyc-x"); got != "" {
		t.Fatalf("a cycle must resolve to nothing: %q", got)
	}
	if got := state.chainRoot("dangler"); got != "" {
		t.Fatalf("a walk leaving the table must resolve to nothing: %q", got)
	}
	if got := state.chainRoot("badtype"); got != "" {
		t.Fatalf("a malformed parent must resolve to nothing: %q", got)
	}
	if got := state.chainRoot("absent"); got != "" {
		t.Fatalf("an unloaded job must resolve to nothing: %q", got)
	}
}

func TestLatestMemberSelection(t *testing.T) {
	state := chainState(map[string]map[string]any{
		"root-a":  {"jobId": "root-a", "round": json.Number("1")},
		"child-b": {"jobId": "child-b", "parentJob": "root-a", "round": json.Number("3")},
		"child-c": {"jobId": "child-c", "parentJob": "root-a", "round": json.Number("2")},
		"noround": {"jobId": "noround", "parentJob": "root-a"},
		"other":   {"jobId": "other", "round": json.Number("9")},
	})
	best := state.latestMember("root-a")
	if best == nil || best["jobId"] != "child-b" {
		t.Fatalf("highest round not selected: %v", best)
	}
	// A chain whose members all lack integer rounds has no latest.
	empty := chainState(map[string]map[string]any{
		"r": {"jobId": "r"},
	})
	if got := empty.latestMember("r"); got != nil {
		t.Fatalf("roundless chain returned a member: %v", got)
	}
	// A chain that does not exist has no latest.
	if got := state.latestMember("nope"); got != nil {
		t.Fatalf("absent chain returned a member: %v", got)
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
	record := map[string]any{
		"jobId": job, "role": role, "round": round, "parentJob": parent,
		"status": "completed",
	}
	if round == 1 {
		record[findingRegisterField] = []any{}
		record[findingRegisterRoundField] = 0
		record[boundedCritiqueStartField] = nil
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
			"schemaVersion": 3, "jobId": job, "round": round,
			"findings": findings, "rigor": rigor,
		})
	}
	if outcome, err := CritiqueRegisterAdvance(repo, root, job); err != nil || outcome != "advanced" {
		t.Fatalf("advance %s = %q, %v", job, outcome, err)
	}
	return job
}

func capMessage(t *testing.T, repo, root string) string {
	t.Helper()
	items := readRegister(t, repo, root)
	var ids []string
	for _, raw := range items {
		entry := raw.(map[string]any)
		if entry["status"] == "open" || entry["status"] == "disputed" {
			ids = append(ids, entry["findingId"].(string))
		}
	}
	path := filepath.Join(t.TempDir(), "message.md")
	if err := os.WriteFile(path, []byte("Address every open finding: "+strings.Join(ids, " ")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBoundedCapStartsAtRoundsOneTwoAndThreeAndAllowsExactlyTwoFurther(t *testing.T) {
	for _, start := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("start-round-%d", start), func(t *testing.T) {
			repo := t.TempDir()
			for round := 1; round < start; round++ {
				writeCapRound(t, repo, "critic", "design-critic", round, false, []any{}, []any{})
			}
			writeCapRound(t, repo, "critic", "design-critic", start, false,
				[]any{registerFindingValue("B-1", true, "bounded evidence")},
				[]any{registerRigor("B-1", "bounded")})
			root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
			startRound, _, err := boundedCritiqueStart(root)
			if err != nil || startRound != int64(start) {
				t.Fatalf("bounded start = %d, %v; want %d", startRound, err, start)
			}
			message := capMessage(t, repo, "critic")
			if outcome, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, fmt.Sprintf("critic-r%d", start+1)); err != nil || outcome != "none" {
				t.Fatalf("start round exhausted early: %q, %v", outcome, err)
			}
			writeCapRound(t, repo, "critic", "design-critic", start+1, false, []any{}, []any{})
			if outcome, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, fmt.Sprintf("critic-r%d", start+2)); err != nil || outcome != "none" {
				t.Fatalf("first further round exhausted early: %q, %v", outcome, err)
			}
			writeCapRound(t, repo, "critic", "design-critic", start+2, false, []any{}, []any{})
			if _, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, fmt.Sprintf("critic-r%d", start+3)); err == nil || !strings.Contains(err.Error(), "bounded critique cap is exhausted") {
				t.Fatalf("second further round did not refuse: %v", err)
			}
		})
	}
}

func TestSevereFindingOverridesBoundedDeadline(t *testing.T) {
	repo := t.TempDir()
	writeCapRound(t, repo, "critic", "design-critic", 1, false,
		[]any{registerFindingValue("B-1", true, "bounded")}, []any{registerRigor("B-1", "bounded")})
	writeCapRound(t, repo, "critic", "design-critic", 2, false,
		[]any{registerFindingValue("S-1", true, "severe")}, []any{registerRigor("S-1", "severe")})
	writeCapRound(t, repo, "critic", "design-critic", 3, false, []any{}, []any{})
	message := capMessage(t, repo, "critic")
	outcome, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r4")
	if err != nil || outcome != "recorded" {
		t.Fatalf("severe override did not use first 3-round exhaustion: %q, %v", outcome, err)
	}
}

func TestLateSevereEscalationRecordsFirstExhaustionFromState(t *testing.T) {
	repo := t.TempDir()
	writeCapRound(t, repo, "critic", "design-critic", 1, false, []any{}, []any{})
	writeCapRound(t, repo, "critic", "design-critic", 2, false,
		[]any{registerFindingValue("B-1", true, "bounded")}, []any{registerRigor("B-1", "bounded")})
	writeCapRound(t, repo, "critic", "design-critic", 3, false, []any{}, []any{})
	writeCapRound(t, repo, "critic", "design-critic", 4, false,
		[]any{registerFindingValue("S-1", true, "severe escalation")}, []any{registerRigor("S-1", "severe")})
	message := capMessage(t, repo, "critic")
	outcome, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r5")
	if err != nil || outcome != "recorded" {
		t.Fatalf("round-four severe escalation did not record the missing first exhaustion: %q, %v", outcome, err)
	}
	root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
	previous, err := exhaustions(root)
	if err != nil || len(previous) != 1 || roundOf(previous[0]) != 4 {
		t.Fatalf("first exhaustion state = %v, %v; want one entry at round four", previous, err)
	}
}

func TestProtocolErrorUnprovenFindingOverridesBoundedDeadline(t *testing.T) {
	repo := t.TempDir()
	writeCapRound(t, repo, "critic", "design-critic", 1, false,
		[]any{registerFindingValue("B-1", true, "bounded")}, []any{registerRigor("B-1", "bounded")})
	writeCapRound(t, repo, "critic", "design-critic", 2, true, nil, nil)
	writeCapRound(t, repo, "critic", "design-critic", 3, false, []any{}, []any{})
	message := capMessage(t, repo, "critic")
	outcome, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r4")
	if err != nil || outcome != "recorded" {
		t.Fatalf("protocol-error unproven override did not use first 3-round exhaustion: %q, %v", outcome, err)
	}
}

func TestRoundThreeFirstExhaustionAndRoundSixTerminalExhaustion(t *testing.T) {
	repo := t.TempDir()
	writeCapRound(t, repo, "critic", "design-critic", 1, false,
		[]any{registerFindingValue("S-1", true, "severe")}, []any{registerRigor("S-1", "severe")})
	writeCapRound(t, repo, "critic", "design-critic", 2, false, []any{}, []any{})
	writeCapRound(t, repo, "critic", "design-critic", 3, false, []any{}, []any{})
	message := capMessage(t, repo, "critic")
	if outcome, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r4"); err != nil || outcome != "recorded" {
		t.Fatalf("round three first exhaustion = %q, %v", outcome, err)
	}
	if outcome, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r4"); err != nil || outcome != "unchanged" {
		t.Fatalf("round three retry = %q, %v", outcome, err)
	}
	for round := 4; round <= 6; round++ {
		writeCapRound(t, repo, "critic", "design-critic", round, false, []any{}, []any{})
	}
	if _, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r7"); err == nil || !strings.Contains(err.Error(), "terminal round 6") {
		t.Fatalf("round six did not refuse terminally: %v", err)
	} else if op, ok := err.(*OpError); !ok || op.Code != CritiqueCapExhaustedExitCode || op.Reason != CritiqueCapExhaustedReason {
		t.Fatalf("round six refusal is not the typed human raise: %T %v", err, err)
	}
}

func TestProtocolErrorRoundsCountAtOffCapRoundThreeAndRoundSix(t *testing.T) {
	repo := t.TempDir()
	writeCapRound(t, repo, "critic", "design-critic", 1, true, nil, nil)
	writeCapRound(t, repo, "critic", "design-critic", 2, true, nil, nil)
	message := capMessage(t, repo, "critic")
	if outcome, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r3"); err != nil || outcome != "none" {
		t.Fatalf("off-cap protocol error = %q, %v", outcome, err)
	}
	writeCapRound(t, repo, "critic", "design-critic", 3, true, nil, nil)
	message = capMessage(t, repo, "critic")
	if outcome, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r4"); err != nil || outcome != "recorded" {
		t.Fatalf("round-three protocol exhaustion = %q, %v", outcome, err)
	}
	for round := 4; round <= 6; round++ {
		writeCapRound(t, repo, "critic", "design-critic", round, true, nil, nil)
	}
	message = capMessage(t, repo, "critic")
	if _, err := CritiqueExhaustionAdvance(repo, "critic", "design-critic", message, "critic-r7"); err == nil || !strings.Contains(err.Error(), "terminal round 6") {
		t.Fatalf("round-six protocol error did not terminate: %v", err)
	}
	if got := readRegisterRound(t, repo, "critic"); got != 6 {
		t.Fatalf("protocol errors advanced through round %d, want six", got)
	}
	for _, raw := range readRegister(t, repo, "critic") {
		entry := raw.(map[string]any)
		if !strings.HasPrefix(entry["findingId"].(string), "synthetic-") || entry["rigorClass"] != "unproven" {
			t.Fatalf("protocol failure was not retained as synthetic unproven evidence: %v", entry)
		}
	}
}

func TestZeroMaterialRoundsNeverTriggerCritiqueExhaustion(t *testing.T) {
	repo := t.TempDir()
	message := filepath.Join(t.TempDir(), "message.md")
	os.WriteFile(message, []byte("no findings\n"), 0o644)
	for round := 1; round <= 6; round++ {
		writeCapRound(t, repo, "critic", "code-critic", round, false, []any{}, []any{})
		if outcome, err := CritiqueExhaustionAdvance(repo, "critic", "code-critic", message, fmt.Sprintf("critic-r%d", round+1)); err != nil || outcome != "none" {
			t.Fatalf("zero-material round %d = %q, %v", round, outcome, err)
		}
	}
}

func TestImplementationSuccessorOwnsCodeCriticAndWardenFirstExhaustion(t *testing.T) {
	for _, role := range []string{"code-critic", "warden"} {
		t.Run(role, func(t *testing.T) {
			repo := t.TempDir()
			agents := filepath.Join(repo, "artifacts", "agents")
			writeJSONFile(t, filepath.Join(agents, "jobs"), "impl.json", map[string]any{
				"jobId": "impl", "role": "implementer", "round": 1,
				"parentJob": nil, "status": "completed",
			})
			writeCapRound(t, repo, "critic", role, 1, false,
				[]any{registerFindingValue("S-1", true, "severe")}, []any{registerRigor("S-1", "severe")})
			rootPath := filepath.Join(agents, "jobs", "critic.json")
			root := readJSONFile(t, rootPath)
			root["reviews"] = "impl"
			if err := writeRecord(rootPath, root); err != nil {
				t.Fatal(err)
			}
			writeCapRound(t, repo, "critic", role, 2, false, []any{}, []any{})
			writeCapRound(t, repo, "critic", role, 3, false, []any{}, []any{})
			message := capMessage(t, repo, "critic")
			if _, err := CritiqueExhaustionAdvance(repo, "critic", role, message, "critic-r4"); err == nil || !strings.Contains(err.Error(), "implementer follow-up") {
				t.Fatalf("critic owned its first exhaustion: %v", err)
			}
			if outcome, err := CritiqueExhaustionAdvance(repo, "impl", "implementer", message, "impl-r2"); err != nil || outcome != "recorded" {
				t.Fatalf("implementer exhaustion advance = %q, %v", outcome, err)
			}
			if outcome, err := CritiqueExhaustionAdvance(repo, "critic", role, message, "critic-r4"); err != nil || outcome != "none" {
				t.Fatalf("recorded correction did not reopen critic budget: %q, %v", outcome, err)
			}
		})
	}
}
