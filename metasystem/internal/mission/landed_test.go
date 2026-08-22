package mission

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// landedRepo lays out a repo with a jobs tree and a stub return checker: a
// job listed in the repo's invalid-jobs file fails validation, everything
// else passes. The stub stands in for scripts/assert-return-complete.sh so
// derivation exercises the real subprocess seam.
func landedRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	script := filepath.Join(repo, "scripts", "assert-return-complete.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	stub := "#!/bin/sh\n" +
		"if [ -f invalid-jobs ] && grep -qx -- \"$2\" invalid-jobs; then exit 1; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(script, []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo
}

func landedWriteJob(t *testing.T, repo, id string, doc map[string]any) {
	t.Helper()
	doc["jobId"] = id
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "artifacts", "agents", "jobs", id+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func landedWriteReturn(t *testing.T, repo, root string, round int) {
	t.Helper()
	path := filepath.Join(repo, "artifacts", "agents", root, "rounds", fmt.Sprint(round), "return.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"jobId":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLandedReturnsQualificationMatrix(t *testing.T) {
	repo := landedRepo(t)
	terminal := func(extra map[string]any) map[string]any {
		doc := map[string]any{"mission": "m1", "status": "completed", "round": 1, "parentJob": nil}
		for k, v := range extra {
			doc[k] = v
		}
		return doc
	}

	// a: landed, validates, unacted -> a ready row.
	landedWriteJob(t, repo, "a1", terminal(nil))
	landedWriteReturn(t, repo, "a1", 1)
	// b: landed but certified by a concluded turn -> retired.
	landedWriteJob(t, repo, "b1", terminal(nil))
	landedWriteReturn(t, repo, "b1", 1)
	// c: landed round 1, but a concluded turn's accepted dispatch claim
	// names round 2 of the chain -> superseded; round 2 never landed.
	landedWriteJob(t, repo, "c1", terminal(nil))
	landedWriteReturn(t, repo, "c1", 1)
	landedWriteJob(t, repo, "c2", map[string]any{"mission": "m1", "status": "running", "round": 2, "parentJob": "c1"})
	// d: landed but the checker refuses it -> invalid marker, path kept.
	landedWriteJob(t, repo, "d1", terminal(nil))
	landedWriteReturn(t, repo, "d1", 1)
	if err := os.WriteFile(filepath.Join(repo, "invalid-jobs"), []byte("d1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// e: the chain's rounds tree is unreadable -> unreadable marker.
	landedWriteJob(t, repo, "e1", terminal(nil))
	if err := os.MkdirAll(filepath.Join(repo, "artifacts", "agents", "e1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "artifacts", "agents", "e1", "rounds"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	// f: a foreign mission's chain never lists.
	landedWriteJob(t, repo, "f1", map[string]any{"mission": "other", "status": "completed", "round": 1})
	landedWriteReturn(t, repo, "f1", 1)
	// g: a runner-closed chain with an uncertified landed round KEEPS its
	// row - closure never excludes (the park that orphans a return must
	// not also hide it).
	landedWriteJob(t, repo, "g1", terminal(map[string]any{"chainClosed": true, "runnerClosed": true}))
	landedWriteReturn(t, repo, "g1", 1)
	// h: no landed return at all -> no row.
	landedWriteJob(t, repo, "h1", terminal(nil))
	// i: round 2 landed AND was the dispatched claim -> its own dispatch is
	// not a successor, so the chain lists its latest landed round.
	landedWriteJob(t, repo, "i1", terminal(nil))
	landedWriteReturn(t, repo, "i1", 1)
	landedWriteJob(t, repo, "i2", map[string]any{"mission": "m1", "status": "completed", "round": 2, "parentJob": "i1"})
	landedWriteReturn(t, repo, "i1", 2)

	turnLog := []any{
		map[string]any{
			"outcome":   "completed",
			"certified": []any{map[string]any{"jobId": "b1", "verdict": "pass", "evidence": "seen"}},
			"accepted": []any{
				map[string]any{"kind": "dispatched", "value": map[string]any{"jobId": "c2"}},
				map[string]any{"kind": "dispatched", "value": map[string]any{"jobId": "i2"}},
			},
		},
	}
	rows := LandedReturns(repo, "m1", turnLog)
	want := [][]string{
		{"a1", "1", "artifacts/agents/a1/rounds/1/return.json"},
		{"d1", "invalid", "artifacts/agents/d1/rounds/1/return.json"},
		{"e1", "unreadable", "none"},
		{"g1", "1", "artifacts/agents/g1/rounds/1/return.json"},
		{"i1", "2", "artifacts/agents/i1/rounds/2/return.json"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("qualification matrix rows:\n got %v\nwant %v", rows, want)
	}
}

func TestLandedReturnsOverflowBoundaryAtTwentyOneChains(t *testing.T) {
	repo := landedRepo(t)
	for i := 1; i <= 21; i++ {
		id := fmt.Sprintf("ch-%02d", i)
		landedWriteJob(t, repo, id, map[string]any{"mission": "m1", "status": "completed", "round": 1})
		landedWriteReturn(t, repo, id, 1)
	}
	rows := LandedReturns(repo, "m1", nil)
	if len(rows) != 20 {
		t.Fatalf("21 qualifying chains must emit exactly 20 rows, got %d", len(rows))
	}
	for i := 0; i < 19; i++ {
		wantRoot := fmt.Sprintf("ch-%02d", i+1)
		if rows[i][0] != wantRoot {
			t.Fatalf("row %d must carry chain %s in sort order, got %v", i, wantRoot, rows[i])
		}
	}
	last := rows[19]
	if last[0] != "overflow" || last[1] != "2" || last[2] != "none" {
		t.Fatalf("the final row must be the overflow summary for 2 chains: %v", last)
	}

	// Exactly 20 chains: every qualifying row is emitted, no overflow.
	if err := os.Remove(filepath.Join(repo, "artifacts", "agents", "jobs", "ch-21.json")); err != nil {
		t.Fatal(err)
	}
	rows = LandedReturns(repo, "m1", nil)
	if len(rows) != 20 || rows[19][0] != "ch-20" {
		t.Fatalf("20 qualifying chains must all list without overflow: %v", rows)
	}
}

func TestLandedReturnsIsAPureFunctionAcrossParks(t *testing.T) {
	repo := landedRepo(t)
	// Two chains with equal round numbers sort deterministically by root.
	for _, id := range []string{"z-chain", "a-chain"} {
		landedWriteJob(t, repo, id, map[string]any{"mission": "m1", "status": "completed", "round": 1})
		landedWriteReturn(t, repo, id, 1)
	}
	first := LandedReturns(repo, "m1", nil)
	if len(first) != 2 || first[0][0] != "a-chain" || first[1][0] != "z-chain" {
		t.Fatalf("rows must sort by chain root: %v", first)
	}
	// A park, a failed turn, or a drain-stalled cycle records no consumption:
	// turn-log entries without certified or accepted claims change nothing,
	// and re-derivation after resume lists the same rows.
	parkedLog := []any{
		map[string]any{"outcome": "drain-stalled", "detail": "deadline passed"},
		map[string]any{"outcome": "failed", "detail": "host crashed"},
	}
	second := LandedReturns(repo, "m1", parkedLog)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("the list must be a pure function of tree and host action:\n%v\nvs\n%v", first, second)
	}
}
