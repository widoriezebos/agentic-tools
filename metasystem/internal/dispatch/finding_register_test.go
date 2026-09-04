package dispatch

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

func registerFacts() map[string]any {
	return map[string]any{
		"local": true, "recoverable": true,
		"proofBoundaryCrossed": false, "authorityBoundaryCrossed": false,
		"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false,
		"externalSideEffectBoundaryCrossed": false,
	}
}

func registerRigor(id, class string) map[string]any {
	return map[string]any{
		"findingId": id, "rigorClass": class, "facts": registerFacts(),
		"reopeningTrigger": "reopen when the finding recurs",
	}
}

func registerFindingValue(id string, material bool, evidence string) map[string]any {
	return map[string]any{
		"id": id, "severity": "high", "material": material,
		"claim": "the claimed defect", "evidence": evidence,
	}
}

func writeCriticRound(t *testing.T, repo, root, job string, round int, findings, rigor []any) {
	t.Helper()
	agents := filepath.Join(repo, "artifacts", "agents")
	parent := any(nil)
	if job != root {
		parent = root
	}
	record := map[string]any{
		"jobId": job, "role": "code-critic", "round": round,
		"parentJob": parent, "status": "completed",
	}
	if job == root {
		record[findingRegisterField] = []any{}
		record[findingRegisterRoundField] = 0
	}
	writeJSONFile(t, filepath.Join(agents, "jobs"), job+".json", record)
	writeJSONFile(t, filepath.Join(agents, root, "rounds", fmt.Sprint(round)), "return.json", map[string]any{
		"schemaVersion": 3, "jobId": job, "round": round,
		"findings": findings, "rigor": rigor,
	})
}

func setCriticSubject(t *testing.T, repo, root, reviews, reviewedTree string) {
	t.Helper()
	recordPath := filepath.Join(repo, "artifacts", "agents", "jobs", root+".json")
	record := readJSONFile(t, recordPath)
	record["reviews"] = reviews
	if err := writeRecord(recordPath, record); err != nil {
		t.Fatal(err)
	}
	round, ok := numInt(record["round"])
	if !ok {
		t.Fatalf("critic root %s has no round number", root)
	}
	returnPath := filepath.Join(repo, "artifacts", "agents", root, "rounds", fmt.Sprint(round), "return.json")
	result := readJSONFile(t, returnPath)
	result["reviewedTree"] = reviewedTree
	if err := writeRecord(returnPath, result); err != nil {
		t.Fatal(err)
	}
}

func readRegister(t *testing.T, repo, root string) []any {
	t.Helper()
	record := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", root+".json"))
	items, ok := record[findingRegisterField].([]any)
	if !ok {
		t.Fatalf("finding register is not an array: %v", record[findingRegisterField])
	}
	return items
}

func readRegisterRound(t *testing.T, repo, root string) int64 {
	t.Helper()
	record := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", root+".json"))
	round, ok := numInt(record[findingRegisterRoundField])
	if !ok {
		t.Fatalf("finding register round is not an integer: %v", record[findingRegisterRoundField])
	}
	return round
}

func registerEntryByID(t *testing.T, items []any, id string) map[string]any {
	t.Helper()
	for _, raw := range items {
		entry := raw.(map[string]any)
		if entry["findingId"] == id {
			return entry
		}
	}
	t.Fatalf("finding %s is absent from register %v", id, items)
	return nil
}

func TestCritiqueRegisterAdvanceIsIdempotent(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1,
		[]any{registerFindingValue("F-1", true, "line 10 proves it")},
		[]any{registerRigor("F-1", "bounded")})

	first, err := CritiqueRegisterAdvance(repo, "critic", "critic")
	if err != nil || first != "advanced" {
		t.Fatalf("first advance = %q, %v", first, err)
	}
	want := readRegister(t, repo, "critic")
	second, err := CritiqueRegisterAdvance(repo, "critic", "critic")
	if err != nil || second != "unchanged" {
		t.Fatalf("retry = %q, %v", second, err)
	}
	got := readRegister(t, repo, "critic")
	if string(canonicalJSON(got)) != string(canonicalJSON(want)) {
		t.Fatalf("idempotent retry changed register: got %v want %v", got, want)
	}
}

func TestCritiqueRegisterFoldsEveryFailedCriticAttempt(t *testing.T) {
	for _, failure := range []string{"empty_reply", "runtime_error"} {
		t.Run(failure, func(t *testing.T) {
			repo := t.TempDir()
			writeCriticRound(t, repo, "critic", "critic", 1, nil, nil)
			path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
			record := readJSONFile(t, path)
			record["status"] = "failed"
			record["error"] = failure
			record["phase"] = "delivery"
			if err := writeRecord(path, record); err != nil {
				t.Fatal(err)
			}
			outcome, err := CritiqueRegisterAdvance(repo, "critic", "critic")
			if err != nil || outcome != "advanced" {
				t.Fatalf("failed critic fold = %q, %v", outcome, err)
			}
			items := readRegister(t, repo, "critic")
			if len(items) != 1 {
				t.Fatalf("failed attempt did not create one synthetic finding: %v", items)
			}
			entry := items[0].(map[string]any)
			if entry["rigorClass"] != "unproven" || !strings.HasPrefix(entry["findingId"].(string), "synthetic-") {
				t.Fatalf("failed attempt did not become synthetic unproven evidence: %v", entry)
			}
		})
	}
}

func TestCritiqueRegisterConcurrentRetryHasOneWinner(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1,
		[]any{registerFindingValue("F-1", true, "evidence")},
		[]any{registerRigor("F-1", "severe")})

	start := make(chan struct{})
	outcomes := make(chan string, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			outcome, err := CritiqueRegisterAdvance(repo, "critic", "critic")
			outcomes <- outcome
			errors <- err
		}()
	}
	close(start)
	wait.Wait()
	close(outcomes)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for outcome := range outcomes {
		got = append(got, outcome)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != "advanced,unchanged" {
		t.Fatalf("concurrent outcomes = %v", got)
	}
	if items := readRegister(t, repo, "critic"); len(items) != 1 {
		t.Fatalf("concurrent retry lost or duplicated findings: %v", items)
	}
}

func TestCritiqueRegisterOutOfOrderRoundIsRetryableAndCannotLoseResolution(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1,
		[]any{registerFindingValue("F-1", true, "open")},
		[]any{registerRigor("F-1", "severe")})
	writeCriticRound(t, repo, "critic", "critic-r2", 2,
		[]any{registerFindingValue("F-1", false, "resolved")}, nil)

	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic-r2"); err == nil {
		t.Fatal("round two advanced before round one")
	} else {
		op, ok := err.(*OpError)
		if !ok || op.Code != 3 {
			t.Fatalf("out-of-order refusal is not typed retryable: %T %v", err, err)
		}
	}
	if items := readRegister(t, repo, "critic"); len(items) != 0 {
		t.Fatalf("out-of-order refusal mutated the register: %v", items)
	}
	if got := readRegisterRound(t, repo, "critic"); got != 0 {
		t.Fatalf("out-of-order refusal recorded round %d, want zero", got)
	}
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}
	if got := readRegisterRound(t, repo, "critic"); got != 1 {
		t.Fatalf("first advance recorded round %d, want one", got)
	}
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic-r2"); err != nil {
		t.Fatal(err)
	}
	if got := readRegisterRound(t, repo, "critic"); got != 2 {
		t.Fatalf("second advance recorded round %d, want two", got)
	}
	if got := registerEntryByID(t, readRegister(t, repo, "critic"), "F-1")["status"]; got != "resolved" {
		t.Fatalf("round two did not resolve the finding: %v", got)
	}
	if outcome, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil || outcome != "unchanged" {
		t.Fatalf("late round-one retry = %q, %v", outcome, err)
	}
	if got := registerEntryByID(t, readRegister(t, repo, "critic"), "F-1")["status"]; got != "resolved" {
		t.Fatalf("late retry lost the later resolution: %v", got)
	}
}

func TestCritiqueRegisterDowngradeIsDisputed(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1,
		[]any{registerFindingValue("F-1", true, "severe evidence")},
		[]any{registerRigor("F-1", "severe")})
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}
	writeCriticRound(t, repo, "critic", "critic-r2", 2,
		[]any{registerFindingValue("F-1", true, "bounded claim")},
		[]any{registerRigor("F-1", "bounded")})
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic-r2"); err != nil {
		t.Fatal(err)
	}
	entry := registerEntryByID(t, readRegister(t, repo, "critic"), "F-1")
	if entry["status"] != "disputed" || entry["rigorClass"] != "severe" {
		t.Fatalf("downgrade did not preserve the severe open dispute: %v", entry)
	}
}

func TestCritiqueRegisterOmissionStaysOpenAndNonMaterialResolves(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1,
		[]any{
			registerFindingValue("F-keep", true, "keep"),
			registerFindingValue("F-resolve", true, "resolve"),
		},
		[]any{registerRigor("F-keep", "severe"), registerRigor("F-resolve", "severe")})
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}
	writeCriticRound(t, repo, "critic", "critic-r2", 2,
		[]any{registerFindingValue("F-resolve", false, "fixed")}, nil)
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic-r2"); err != nil {
		t.Fatal(err)
	}
	items := readRegister(t, repo, "critic")
	if got := registerEntryByID(t, items, "F-keep")["status"]; got != "open" {
		t.Fatalf("omitted finding status = %v, want open", got)
	}
	if got := registerEntryByID(t, items, "F-resolve")["status"]; got != "resolved" {
		t.Fatalf("non-material finding status = %v, want resolved", got)
	}
}

func TestCritiqueRegisterSyntheticIDsAreStableAndEnumerated(t *testing.T) {
	repo := t.TempDir()
	findings := []any{
		registerFindingValue("", true, "empty"),
		registerFindingValue("DUP", true, "same duplicate"),
		registerFindingValue("DUP", true, "same duplicate"),
	}
	rigor := []any{registerRigor("", "severe"), registerRigor("DUP", "severe"), registerRigor("DUP", "severe")}
	writeCriticRound(t, repo, "critic", "critic", 1, findings, rigor)
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}
	first := readRegister(t, repo, "critic")
	if len(first) != 2 {
		t.Fatalf("empty or duplicate identifiers vanished: %v", first)
	}
	var duplicateID string
	for _, raw := range first {
		entry := raw.(map[string]any)
		if multiplicity, ok := numInt(entry["multiplicity"]); ok && multiplicity == 2 {
			duplicateID = entry["findingId"].(string)
		}
	}
	if !strings.HasPrefix(duplicateID, "synthetic-") {
		t.Fatalf("duplicate pair did not collapse with multiplicity two: %v", first)
	}
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}
	second := readRegister(t, repo, "critic")
	if string(canonicalJSON(first)) != string(canonicalJSON(second)) {
		t.Fatalf("synthetic identifiers changed on retry: first %v second %v", first, second)
	}
	writeCriticRound(t, repo, "critic", "critic-r2", 2,
		[]any{findings[2], findings[0], findings[1]},
		[]any{rigor[1], rigor[0], rigor[2]})
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic-r2"); err != nil {
		t.Fatal(err)
	}
	reordered := readRegister(t, repo, "critic")
	duplicateMultiplicity, multiplicityOK := numInt(registerEntryByID(t, reordered, duplicateID)["multiplicity"])
	if len(reordered) != 2 || !multiplicityOK || duplicateMultiplicity != 2 {
		t.Fatalf("reordered later round changed synthetic identity: %v", reordered)
	}
	register, err := decodeFindingRegister(readRegister(t, repo, "critic"))
	if err != nil {
		t.Fatal(err)
	}
	ids := openRegisterFindingIDs(register)
	if len(ids) != 2 || ids[0] == "" || ids[0] == ids[1] {
		t.Fatalf("open material identifiers = %v", ids)
	}
}

func TestCritiqueRegisterCrossRootClassConflictBlocks(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic-a", "critic-a", 1,
		[]any{registerFindingValue("SHARED", true, "severe")},
		[]any{registerRigor("SHARED", "severe")})
	setCriticSubject(t, repo, "critic-a", "implementer", "tree-a")
	if _, err := CritiqueRegisterAdvance(repo, "critic-a", "critic-a"); err != nil {
		t.Fatal(err)
	}
	writeCriticRound(t, repo, "critic-b", "critic-b", 1,
		[]any{registerFindingValue("SHARED", true, "bounded")},
		[]any{registerRigor("SHARED", "bounded")})
	setCriticSubject(t, repo, "critic-b", "implementer", "tree-b")
	if _, err := CritiqueRegisterAdvance(repo, "critic-b", "critic-b"); err == nil ||
		!strings.Contains(err.Error(), "conflicting rigor classes") ||
		!strings.Contains(err.Error(), `reviews="implementer" reviewedTree="tree-a"`) ||
		!strings.Contains(err.Error(), `reviews="implementer" reviewedTree="tree-b"`) ||
		!strings.Contains(err.Error(), "waiting on the original critic or the human is the only remedy") {
		t.Fatalf("cross-root conflict = %v", err)
	}
	if items := readRegister(t, repo, "critic-b"); len(items) != 0 {
		t.Fatalf("blocked root was mutated: %v", items)
	}
}

func TestCritiqueRegisterSameFindingIDOnDifferentSubjectsAdvances(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic-a", "critic-a", 1,
		[]any{registerFindingValue("F-1", true, "severe")},
		[]any{registerRigor("F-1", "severe")})
	setCriticSubject(t, repo, "critic-a", "implementer-a", "tree-a")
	if _, err := CritiqueRegisterAdvance(repo, "critic-a", "critic-a"); err != nil {
		t.Fatal(err)
	}

	writeCriticRound(t, repo, "critic-b", "critic-b", 1,
		[]any{registerFindingValue("F-1", true, "bounded")},
		[]any{registerRigor("F-1", "bounded")})
	setCriticSubject(t, repo, "critic-b", "implementer-b", "tree-b")
	if outcome, err := CritiqueRegisterAdvance(repo, "critic-b", "critic-b"); err != nil || outcome != "advanced" {
		t.Fatalf("different-subject advance = %q, %v", outcome, err)
	}
}

func TestCritiqueRegisterSameReviewedTreeConflictsAcrossReviewTargets(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic-a", "critic-a", 1,
		[]any{registerFindingValue("F-1", true, "severe")},
		[]any{registerRigor("F-1", "severe")})
	setCriticSubject(t, repo, "critic-a", "implementer-a", "shared-tree")
	if _, err := CritiqueRegisterAdvance(repo, "critic-a", "critic-a"); err != nil {
		t.Fatal(err)
	}

	writeCriticRound(t, repo, "critic-b", "critic-b", 1,
		[]any{registerFindingValue("F-1", true, "bounded")},
		[]any{registerRigor("F-1", "bounded")})
	setCriticSubject(t, repo, "critic-b", "implementer-b", "shared-tree")
	if _, err := CritiqueRegisterAdvance(repo, "critic-b", "critic-b"); err == nil ||
		!strings.Contains(err.Error(), "conflicting rigor classes") {
		t.Fatalf("same-tree conflict = %v", err)
	}
}

func TestCritiqueRegisterReissuedRoundWithNewFindingIDsAdvances(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1,
		[]any{registerFindingValue("F-1", true, "first")},
		[]any{registerRigor("F-1", "severe")})
	setCriticSubject(t, repo, "critic", "implementer", "tree")
	if _, err := CritiqueRegisterAdvance(repo, "critic", "critic"); err != nil {
		t.Fatal(err)
	}
	writeCriticRound(t, repo, "critic", "critic-r2", 2,
		[]any{registerFindingValue("F-2", true, "reissued")},
		[]any{registerRigor("F-2", "bounded")})
	if outcome, err := CritiqueRegisterAdvance(repo, "critic", "critic-r2"); err != nil || outcome != "advanced" {
		t.Fatalf("reissued round advance = %q, %v", outcome, err)
	}
	if items := readRegister(t, repo, "critic"); len(items) != 2 {
		t.Fatalf("reissued round did not retain both findings: %v", items)
	}
}

func TestCancelledRoundFoldsNeutrally(t *testing.T) {
	repo := t.TempDir()
	writeCriticRound(t, repo, "critic", "critic", 1, nil, nil)
	path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
	record := readJSONFile(t, path)
	record["status"] = "cancelled"
	if err := writeRecord(path, record); err != nil {
		t.Fatal(err)
	}
	outcome, err := CritiqueRegisterAdvance(repo, "critic", "critic")
	if err != nil || outcome != "advanced" {
		t.Fatalf("cancelled round must fold and unwedge: %q, %v", outcome, err)
	}
	if items := readRegister(t, repo, "critic"); len(items) != 0 {
		t.Fatalf("a cancelled round is nobody's critique yet contributed findings: %v", items)
	}
	if got := readRegisterRound(t, repo, "critic"); got != 1 {
		t.Fatalf("the folded round did not advance: %d", got)
	}
}
