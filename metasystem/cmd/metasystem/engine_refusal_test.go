package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestEngineRefusalsNameTheirCause(t *testing.T) {
	cases := []struct {
		token string
		err   error
	}{
		{"not-enrolled", enrollmentRefusal(t.TempDir(), errors.New("absent"))},
		{"enrollment-drift", engineRefusal("enrollment-drift", nil, "engine changed")},
		{"engine-behind-tip", judgmentRefusal(steward.ErrProjectionDiffers, nil, "compare")},
		{"judgment-stalled", judgmentRefusal(steward.ErrJudgmentStalled, nil, "compare")},
		{"judgment-failed", judgmentRefusal(errors.New("git failed"), nil, "compare")},
		{enginecause.TokenEngineUnavailable, engineRefusal(enginecause.TokenEngineUnavailable, nil, "engine absent")},
		{"child-failed", engineRefusal("child-failed", nil, "child failed")},
		{"child-output", engineRefusal("child-output", nil, "child output malformed")},
		{"mutation-lock", engineRefusal("mutation-lock", nil, "lock busy")},
		{"rebuild-failed", engineRefusal("rebuild-failed", nil, "build failed")},
		{"rearm-failed", engineRefusal("rearm-failed", nil, "up failed")},
	}
	for _, tc := range cases {
		if tc.err == nil || !strings.Contains(tc.err.Error(), engineRefusalCode+": cause="+tc.token) || !strings.Contains(tc.err.Error(), "; run: ") {
			t.Errorf("cause %s was not rendered with a remedy: %v", tc.token, tc.err)
		}
	}
	for _, fact := range []string{"fetch-failed", "head-diverged", "dirty-engine-paths", "named-delivery-tree", "live-attempt"} {
		err := engineRefusal("engine-behind-tip", []enginecause.Fact{enginecause.Value("fact", fact)}, "behind")
		if !strings.Contains(err.Error(), "cause=engine-behind-tip fact="+fact) {
			t.Errorf("engine-behind-tip outcome %s lost its cause: %v", fact, err)
		}
	}
}

func TestUnenrolledLinkedWorktreeNamesItsMainCheckout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "main")
	landedGit(t, filepath.Dir(root), "init", "-q", "-b", "main", root)
	if err := osWriteFile(filepath.Join(root, "tracked"), "tracked\n"); err != nil {
		t.Fatal(err)
	}
	landedGit(t, root, "add", ".")
	landedGit(t, root, "commit", "-qm", "seed")
	linked := filepath.Join(filepath.Dir(root), "linked")
	landedGit(t, root, "worktree", "add", "-q", "-b", "linked", linked)
	root, err := canonicalPath(root)
	if err != nil {
		t.Fatal(err)
	}
	err = enrollmentRefusal(linked, errors.New("steward identity absent"))
	if !strings.Contains(err.Error(), "cause=not-enrolled") || !strings.Contains(err.Error(), "linked-worktree='"+root+"'") {
		t.Fatalf("linked worktree refusal did not name its main checkout: %v", err)
	}
}

func TestDecisionMismatchNamesTheField(t *testing.T) {
	err := decisionMismatchRefusal("candidate", "ours-base", "digest", testingPlanOutput{
		CandidateTree: "candidate", PolicyBaseCommit: "engine-base", BaseContractDigest: "digest",
	})
	if err == nil || !strings.Contains(err.Error(), "cause=decision-mismatch field=policy-base-commit ours=ours-base engine=engine-base") {
		t.Fatalf("decision mismatch did not name the field and both values: %v", err)
	}
}

func osWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
