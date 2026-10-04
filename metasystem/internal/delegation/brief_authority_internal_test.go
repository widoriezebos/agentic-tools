package delegation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeGoalReadBrief writes a goal read's composed brief citing a file the
// feature creates at runtime, which no tree or disk holds, and its record,
// marked as carrying the unit's build brief when marked is set.
func writeGoalReadBrief(t *testing.T, marked bool) (brief, commit string) {
	t.Helper()
	commit = strings.Repeat("b", 40)
	dir := filepath.Join(t.TempDir(), ".git", "metasystem", "goal-reads", "goal-a")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief = filepath.Join(dir, commit+".md")
	body := "Working Mode: implement\n\n# Supplied accepted implementation brief (frozen at dispatch)\n\n" +
		"Write `plans/handoff-project-partner.md` when the partner hands off.\n"
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

// TestBriefAuthorityAdmitsAGoalReadsBuildBrief: a code critic of a commit
// whose brief a goal read recorded as carrying the unit's build brief is
// admitted without its cited paths being read against any tree; the same
// brief unmarked, or under another role or review, is checked as before.
func TestBriefAuthorityAdmitsAGoalReadsBuildBrief(t *testing.T) {
	t.Parallel()
	marked, commit := writeGoalReadBrief(t, true)
	reviews := "commit:" + commit
	// The base tree is no repository: only a skipped path check admits.
	s := &session{root: t.TempDir(), repoScope: t.TempDir(), stderr: &bytes.Buffer{}}
	if err := s.briefAuthority(marked, filepath.Join(t.TempDir(), "absent"), "code-critic", reviews); err != nil {
		t.Fatalf("the build brief of a goal read was refused: %v", err)
	}
	unmarked, _ := writeGoalReadBrief(t, false)
	for name, c := range map[string]struct{ brief, role, reviews string }{
		"unmarked brief":      {unmarked, "code-critic", reviews},
		"implementer role":    {marked, "implementer", reviews},
		"review of a job":     {marked, "code-critic", "implementer-job"},
		"brief outside reads": {filepath.Join(t.TempDir(), commit+".md"), "code-critic", reviews},
	} {
		if buildBriefAdmitted(c.brief, c.role, c.reviews) {
			t.Errorf("%s: the path check is skipped", name)
		}
	}
	if !buildBriefAdmitted(marked, "code-critic", reviews) {
		t.Fatal("the marked brief's path check is not skipped")
	}
}
