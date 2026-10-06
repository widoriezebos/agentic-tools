package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func TestCritiqueRegisterRetrySupersedesPlaceholders(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, commit, tree string
		failedRounds       int
		wantClosed         bool
		legacy             bool
	}{
		{"same commit", "commit-a", "tree-a", 1, true, false},
		{"every earlier placeholder", "commit-a", "tree-a", 2, true, false},
		{"other commit with same tree", "commit-b", "tree-a", 1, false, false},
		{"unbound return", "commit-a", "other-tree", 1, false, false},
		{"historical placeholder", "commit-a", "tree-a", 1, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			subject := ReadSubject{Kind: SubjectCommit, Commit: "commit-a", Tree: "tree-a", DiffDigest: "diff"}
			for round := 1; round <= tc.failedRounds+1; round++ {
				job := "critic"
				if round > 1 {
					job = fmt.Sprintf("critic-r%d", round)
				}
				writeCriticRound(t, repo, "critic", job, round, []any{}, []any{})
				dir := filepath.Join(repo, "artifacts", "agents", "critic", "rounds", fmt.Sprint(round))
				if round <= tc.failedRounds {
					path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
					record := readJSONFile(t, path)
					record["status"], record["error"] = "failed", "provider limit"
					if err := writeRecord(path, record); err != nil {
						t.Fatal(err)
					}
					if err := os.Remove(filepath.Join(dir, "return.json")); err != nil {
						t.Fatal(err)
					}
				} else {
					subject.Commit = tc.commit
					result := readJSONFile(t, filepath.Join(dir, "return.json"))
					result["reviewedTree"] = tc.tree
					if err := writeRecord(filepath.Join(dir, "return.json"), result); err != nil {
						t.Fatal(err)
					}
				}
				if err := WriteReadSubject(filepath.Join(dir, "subject.json"), subject); err != nil {
					t.Fatal(err)
				}
				if _, err := CritiqueRegisterAdvance(repo, "critic", job); err != nil {
					t.Fatal(err)
				}
				for _, raw := range readRegister(t, repo, "critic")[:min(tc.failedRounds, round)] {
					entry := raw.(map[string]any)
					if n, ok := numInt(entry["placeholderRound"]); !ok || n < 1 || n > int64(tc.failedRounds) {
						t.Fatalf("placeholder has no source round: %v", entry)
					}
					wantStatus, wantResolution := "open", ""
					if round > tc.failedRounds && tc.wantClosed {
						wantStatus, wantResolution = "resolved", fmt.Sprintf("superseded by round %d", round)
					}
					if entry["status"] != wantStatus || entry["resolution"] != wantResolution {
						t.Fatalf("placeholder after round %d: %v", round, entry)
					}
				}
				if tc.legacy && round <= tc.failedRounds {
					path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
					record := readJSONFile(t, path)
					delete(record[findingRegisterField].([]any)[0].(map[string]any), "placeholderRound")
					if err := writeRecord(path, record); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := CritiqueRegisterClose(repo, "critic"); (err == nil) != tc.wantClosed {
				t.Fatalf("close: %v; want closed %v", err, tc.wantClosed)
			}
			clean, err := readsubject.CleanRegister(readRegister(t, repo, "critic"))
			if err != nil || clean != tc.wantClosed {
				t.Fatalf("clean register: %v, %v", clean, err)
			}
			landable, risks, err := readsubject.LandableRegister(readRegister(t, repo, "critic"))
			if err != nil || landable != tc.wantClosed || len(risks) != 0 {
				t.Fatalf("landable register: %v, %v, %v", landable, risks, err)
			}
		})
	}
}
