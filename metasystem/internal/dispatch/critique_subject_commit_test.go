package dispatch

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestCritiqueRegisterCommitSubjectUsesDeclaredFacts(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, artifact, path        string
		changed                     []string
		absent                      bool
		pathsErr, treeErr           error
		wantFindings, wantDemotions int
		wantError                   string
	}{
		{name: "in-set", artifact: "metasystem/test.go", path: "metasystem/test.go", changed: []string{"metasystem/test.go"}, wantFindings: 1},
		{name: "outside-set", artifact: "metasystem/test.go", path: "metasystem/other.go", changed: []string{"metasystem/other.go"}, wantDemotions: 1},
		{name: "empty-changed-paths", artifact: "metasystem/test.go", wantError: "commit subject commit:" + commit + " has no changed paths"},
		{name: "unreadable-commit", artifact: "metasystem/test.go", pathsErr: errors.New("declared unreadable commit"), wantError: "commit subject commit:" + commit + " is not a readable non-root commit: declared unreadable commit"},
		{name: "unreadable-tree", artifact: "metasystem/test.go", changed: []string{"metasystem/test.go"}, treeErr: errors.New("declared unreadable tree"), wantError: "commit subject commit:" + commit + " has no readable tree: declared unreadable tree"},
		{name: "new-absent", artifact: "NEW metasystem/new.go", path: "metasystem/new.go", changed: []string{"metasystem/new.go"}, absent: true, wantFindings: 1},
		{name: "new-present", artifact: "NEW metasystem/new.go", path: "metasystem/new.go", changed: []string{"metasystem/new.go"}, wantDemotions: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			rigor := registerRigor("F-1", "bounded")
			rigor["artifact"] = tc.artifact
			writeCriticRound(t, repo, "critic", "critic", 1,
				[]any{registerFindingValue("F-1", true, "evidence")}, []any{rigor})
			rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
			root := readJSONFile(t, rootPath)
			root["reviews"] = "commit:" + commit
			if err := writeRecord(rootPath, root); err != nil {
				t.Fatal(err)
			}
			pathCall := declaredPaths(repo, commit, tc.changed...)
			pathCall.err = tc.pathsErr
			calls := []critiqueFactCall{pathCall}
			if len(tc.changed) != 0 {
				treeCall := declaredTree(repo, commit, "declared-tree")
				treeCall.err = tc.treeErr
				calls = append(calls, treeCall)
				if tc.treeErr == nil && strings.HasPrefix(tc.artifact, "NEW ") {
					calls = append(calls, declaredAbsence(repo, "declared-tree", tc.path, tc.absent))
				}
			}
			outcome, err := advanceWithFacts(t, repo, "critic", "critic", calls...)
			if tc.wantError != "" {
				if err == nil || err.Error() != tc.wantError {
					t.Fatalf("empty commit subject error = %v; want %q", err, tc.wantError)
				}
				if len(readRegister(t, repo, "critic")) != 0 {
					t.Fatal("refused commit subject mutated the register")
				}
				return
			}
			if err != nil || outcome != "advanced" {
				t.Fatalf("commit subject advance = %q, %v", outcome, err)
			}
			if got := len(readRegister(t, repo, "critic")); got != tc.wantFindings {
				t.Fatalf("register findings = %d; want %d", got, tc.wantFindings)
			}
			root = readJSONFile(t, rootPath)
			demotions, _ := root["demotions"].([]any)
			if len(demotions) != tc.wantDemotions {
				t.Fatalf("demotions = %v; want %d", demotions, tc.wantDemotions)
			}
		})
	}
}
