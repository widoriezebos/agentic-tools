package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

func TestGitAdapterCarrierInputsReflectRefIndexCommitAndORIGHEAD(t *testing.T) {
	root := wallRepo(t)
	workspace := gittree.Workspace{Dir: root}
	head, unborn, err := workspace.HeadCommit()
	if err != nil || unborn {
		t.Fatalf("baseline HEAD: %s %t %v", head, unborn, err)
	}
	ref := mission.MissionRefNamespace("alpha") + "state-anchors"
	verificationGit(t, root, "update-ref", ref, head)
	refs, err := workspace.RefMap()
	if err != nil || refs[ref] != head {
		t.Fatalf("created state-anchor ref: %v %v", refs, err)
	}
	verificationGit(t, root, "update-ref", "-d", ref)
	refs, err = workspace.RefMap()
	if err != nil {
		t.Fatal(err)
	}
	if _, present := refs[ref]; present {
		t.Fatalf("deleted state-anchor ref remains visible: %v", refs)
	}

	ledgerRel := missionLedgerRel("alpha")
	ledgerPath := filepath.Join(root, ledgerRel)
	writeText(t, ledgerPath, "authenticated ledger bytes\n")
	foreignPath := filepath.Join(t.TempDir(), "foreign-ledger")
	writeText(t, foreignPath, "arbitrary unreviewed bytes\n")
	out, code := runIn(t, root, "hash-object", "-w", foreignPath)
	if code != 0 {
		t.Fatalf("foreign blob write: %s", out)
	}
	foreignOID := strings.TrimSpace(out)
	if len(foreignOID) != 40 || foreignOID == carrierBlobOID([]byte("authenticated ledger bytes\n")) {
		t.Fatalf("foreign blob id %q is invalid or equals the ledger", foreignOID)
	}
	verificationGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+foreignOID+","+ledgerRel)
	staged, err := workspace.StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	stagedEntries, err := workspace.Entries(staged, []string{ledgerRel})
	want := gittree.Entry{OID: foreignOID, Mode: "100644"}
	if err != nil || stagedEntries[ledgerRel] != want || len(stagedEntries) != 1 {
		t.Fatalf("raw staged ledger entry = %v, %v", stagedEntries, err)
	}
	if current, err := os.ReadFile(ledgerPath); err != nil || string(current) != "authenticated ledger bytes\n" {
		t.Fatalf("worktree ledger changed: %q %v", current, err)
	}

	verificationGit(t, root, "commit", "-qm", "foreign ledger carrier")
	commit, unborn, err := workspace.HeadCommit()
	if err != nil || unborn || commit == head {
		t.Fatalf("carrier commit: %s %t %v", commit, unborn, err)
	}
	raw, err := workspace.TreeOf(commit)
	if err != nil {
		t.Fatal(err)
	}
	committedEntries, err := workspace.Entries(raw, []string{ledgerRel})
	if err != nil || committedEntries[ledgerRel] != want || len(committedEntries) != 1 {
		t.Fatalf("raw committed ledger entry = %v, %v", committedEntries, err)
	}
	verificationGit(t, root, "reset", "-q", "--hard", head)
	pseudorefs, err := workspace.PseudorefCensus()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pseudoref := range pseudorefs {
		if pseudoref.Name == "ORIG_HEAD" {
			found = pseudoref.Parseable && len(pseudoref.OIDs) == 1 && pseudoref.OIDs[0] == commit
		}
	}
	if !found {
		t.Fatalf("reset did not expose carrier commit through ORIG_HEAD: %v", pseudorefs)
	}
}
