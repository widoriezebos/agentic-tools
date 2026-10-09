package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// writeGoalReadBrief writes a goal read's composed brief citing a file the
// feature creates at runtime, which no tree or disk holds, and its record,
// marked as carrying the unit's build brief when marked is set.
func writeGoalReadBrief(t *testing.T, marked, before bool) (brief, commit string) {
	t.Helper()
	commit = strings.Repeat("b", 40)
	dir := filepath.Join(t.TempDir(), ".git", "metasystem", "goal-reads", "goal-a")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief = filepath.Join(dir, commit+".md")
	body := "Working Mode: implement\n\n# Supplied accepted implementation brief (frozen at dispatch)\n\n" +
		"Read `plans/handoff-project-partner.md` when the partner hands off.\n"
	if before {
		body = "Read `plans/handoff-project-partner.md`.\n" + body
	}
	if err := os.WriteFile(brief, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(body))
	record, err := json.Marshal(map[string]any{"schemaVersion": 1, "goal": "goal-a", "unitCommit": commit,
		"tree": strings.Repeat("d", 40), "brief": brief, "frozenBriefSha256": hex.EncodeToString(sum[:]), "briefFromBuild": marked})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, commit+".json"), record, 0o644); err != nil {
		t.Fatal(err)
	}
	return brief, commit
}

func admittedBuildBriefSession(t *testing.T, before bool) (*session, string, string, *scriptedGit) {
	t.Helper()
	brief, commit := writeGoalReadBrief(t, true, before)
	if !branch.BuildBriefAdmitted(brief) {
		t.Fatal("fixture is not an admitted build brief")
	}
	git := &scriptedGit{answers: map[string]scriptedAnswer{
		"rev-parse --verify HEAD^{commit}":                            {stdout: "head"},
		"rev-parse --verify --end-of-options " + commit + "^{commit}": {stdout: commit},
		"ls-tree -d --name-only head":                                 {stdout: "plans"},
		"ls-tree -d --name-only " + commit:                            {stdout: "plans"},
		"rev-parse --show-prefix":                                     {},
		"ls-tree -rtz --name-only --full-tree " + commit:              {},
	}}
	s := rebaseSession(git)
	s.ctx = context.Background()
	s.root, s.repoScope = t.TempDir(), t.TempDir()
	return s, brief, commit, git
}

func TestBriefAuthorityRefusesMissingPathBeforeTheFrozenBuildBrief(t *testing.T) {
	t.Parallel()
	s, brief, commit, git := admittedBuildBriefSession(t, true)
	err := s.briefAuthority(brief, s.repoScope, "code-critic", "commit:"+commit)
	var refusal *dispatch.BriefAuthorityRefusal
	if !errors.As(err, &refusal) || len(refusal.MissingPaths) != 1 || refusal.MissingPaths[0] != "plans/handoff-project-partner.md" {
		t.Fatalf("citation before frozen heading must refuse: %v; Git calls %v", err, git.calls)
	}
}

func TestBriefAuthorityAdmitsMissingPathOnlyInTheFrozenBuildBrief(t *testing.T) {
	t.Parallel()
	s, brief, commit, git := admittedBuildBriefSession(t, false)
	if err := s.briefAuthority(brief, s.repoScope, "code-critic", "commit:"+commit); err != nil {
		t.Fatalf("frozen citation refused: %v", err)
	}
	for _, call := range git.calls {
		if strings.HasPrefix(call, "cat-file") {
			t.Fatalf("checked frozen citation: %v", git.calls)
		}
	}
	if len(git.calls) == 0 {
		t.Fatal("admission did not inspect the reviewed tree")
	}
}
