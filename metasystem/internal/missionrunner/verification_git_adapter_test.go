package missionrunner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

func verificationGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=host", "GIT_AUTHOR_EMAIL=host@test",
		"GIT_COMMITTER_NAME=host", "GIT_COMMITTER_EMAIL=host@test")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestGitAdapterCommittedLedgerTamperDoesNotReplaceAnchorTruth(t *testing.T) {
	t.Parallel()
	root := wallRepo(t)
	e := &Engine{Root: root, Mission: "alpha"}
	ledger := filepath.Join(e.missionDir(), "ledger.md")
	state := filepath.Join(e.missionDir(), "state.json")
	contract := e.approvedContractPath()
	writeText(t, contract, "```mission\ncandidate.branch=main\nstream.solo=Do solo\n```\n```mission-seal\ncandidate.branch=main\n```\n")
	if err := mission.InitLedger(ledger, 5, 3); err != nil {
		t.Fatal(err)
	}
	if err := mission.InitStateWithBaseline(state, contract, ledger, "", "main", recoveryPre,
		map[string]any{"headCommit": recoveryHead, "topTree": nil, "topStaged": nil,
			"refMap":         map[string]any{"refs/heads/main": recoveryHead},
			"worktreeCensus": []any{}, "capturedAt": "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := e.anchor(state, ledger, "open"); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, ledger, string(original)+"- Stop-loss reset: ask=forged\n")
	verificationGit(t, root, "add", "-f", "--", missionLedgerRel(e.Mission))
	verificationGit(t, root, "commit", "-qm", "host launders its ledger edit")
	doc := readTestDoc(t, state)
	anchored, current, err := (defaultWallReads{}).LedgerTruth(root, doc, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if anchored != string(original) || current != string(original)+"- Stop-loss reset: ask=forged\n" {
		t.Fatalf("committed ledger bytes replaced the anchor truth: anchored=%q current=%q", anchored, current)
	}
}

func TestGitAdapterPostAcceptanceCommitMovesCapturedPosture(t *testing.T) {
	t.Parallel()
	root := wallRepo(t)
	e := &Engine{Root: root, Mission: "alpha"}
	before, err := e.captureWallPosture("", nil)
	if err != nil {
		t.Fatal(err)
	}
	verificationGit(t, root, "commit", "-q", "--allow-empty", "-m", "post-acceptance motion")
	after, err := e.captureWallPosture("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if before.Head == after.Head || before.RefMap["refs/heads/main"] == after.RefMap["refs/heads/main"] {
		t.Fatalf("commit did not move captured HEAD and branch ref: before=%s after=%s", before.Head, after.Head)
	}
}

func TestGitAdapterRefMapSeesReplacementNamespace(t *testing.T) {
	t.Parallel()
	root := wallRepo(t)
	workspace := gittree.Workspace{Dir: root}
	head, _, err := workspace.HeadCommit()
	if err != nil {
		t.Fatal(err)
	}
	verificationGit(t, root, "commit", "-q", "--allow-empty", "-m", "replacement target")
	other, _, err := workspace.HeadCommit()
	if err != nil {
		t.Fatal(err)
	}
	verificationGit(t, root, "reset", "-q", "--hard", head)
	verificationGit(t, root, "update-ref", "refs/replace/"+head, other)
	refs, err := workspace.RefMap()
	if err != nil {
		t.Fatal(err)
	}
	if refs["refs/replace/"+head] != other {
		t.Fatalf("default ref map omitted replacement ref: %v", refs)
	}
}
