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

func TestLinkedWorktreeMainInstallationPreservesInstallationSubdirectory(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "main")
	landedGit(t, filepath.Dir(root), "init", "-q", "-b", "main", root)
	module := filepath.Join(root, "metasystem")
	if err := os.Mkdir(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepath.Join(module, "go.mod"), "module fixture\n"); err != nil {
		t.Fatal(err)
	}
	landedGit(t, root, "add", ".")
	landedGit(t, root, "commit", "-qm", "seed nested module")
	linked := filepath.Join(filepath.Dir(root), "linked")
	landedGit(t, root, "worktree", "add", "-q", "-b", "linked-nested", linked)
	got, ok := linkedWorktreeMainInstallation(filepath.Join(linked, "metasystem"))
	want, err := canonicalPath(module)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got != want {
		t.Fatalf("linked nested installation resolved to %q, linked=%t, want %q", got, ok, want)
	}
}

func TestBatchPrefixProofControlRootMustOwnLinkedExecution(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	main := filepath.Join(parent, "main")
	landedGit(t, parent, "init", "-q", "-b", "main", main)
	module := filepath.Join(main, "metasystem")
	if err := os.Mkdir(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepath.Join(module, "go.mod"), "module fixture\n"); err != nil {
		t.Fatal(err)
	}
	landedGit(t, main, "add", ".")
	landedGit(t, main, "commit", "-qm", "seed nested module")
	linked := filepath.Join(parent, "linked")
	landedGit(t, main, "worktree", "add", "-q", "-b", "linked-prefix", linked)
	execution := filepath.Join(linked, "metasystem")
	want, err := canonicalProofRoot(module)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := batchPrefixProofControlRoot(execution, module); err != nil || got != want {
		t.Fatalf("owned batch prefix control root=%q error=%v, want %q", got, err, want)
	}
	unrelated := filepath.Join(parent, "unrelated")
	if err := os.Mkdir(unrelated, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := batchPrefixProofControlRoot(execution, unrelated); err == nil || !strings.Contains(err.Error(), "does not own") {
		t.Fatalf("unrelated batch prefix control root was accepted: %v", err)
	}
}

func TestBatchPrefixProofControlRootAcceptsALinkedLandingWorktree(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	main := filepath.Join(parent, "main")
	landedGit(t, parent, "init", "-q", "-b", "main", main)
	module := filepath.Join(main, "metasystem")
	if err := os.Mkdir(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepath.Join(module, "go.mod"), "module fixture\n"); err != nil {
		t.Fatal(err)
	}
	landedGit(t, main, "add", ".")
	landedGit(t, main, "commit", "-qm", "seed nested module")
	landing := filepath.Join(parent, "landing")
	landedGit(t, main, "worktree", "add", "-q", "-b", "landing", landing)
	proof := filepath.Join(parent, "proof")
	landedGit(t, main, "worktree", "add", "-q", "--detach", proof)
	controlRoot := filepath.Join(landing, "metasystem")
	execution := filepath.Join(proof, "metasystem")
	want, err := canonicalProofRoot(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := batchPrefixProofControlRoot(execution, controlRoot); err != nil || got != want {
		t.Fatalf("linked landing control root=%q error=%v, want %q", got, err, want)
	}
	if _, err := batchPrefixProofControlRoot(execution, landing); err == nil || !strings.Contains(err.Error(), "does not own") {
		t.Fatalf("linked landing root with another prefix was accepted: %v", err)
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
