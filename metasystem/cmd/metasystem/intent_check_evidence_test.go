package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func evidenceSelectionCommand(t *testing.T) string {
	t.Helper()
	fixture := filepath.Join(t.TempDir(), "selection.json")
	if err := (launch.CheckSelection{Version: 1, Base: "base-commit", BaseTree: "base-tree", Workers: 9}).Write(fixture); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "cheap")
	if err := testexec.WriteFile(script, []byte("#!/bin/sh\n/bin/cp "+shellCommand([]string{fixture})+" \"$METASYSTEM_CHECK_SELECTION\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return shellCommand([]string{script})
}

func checkTreeEvidence(t *testing.T, path, tree string) {
	t.Helper()
	data, err := os.ReadFile(path)
	var evidence launch.CheckEvidence
	if err != nil || json.Unmarshal(data, &evidence) != nil {
		t.Fatalf("evidence: %s %v", data, err)
	}
	if evidence.Content.Before != tree || evidence.Content.After != tree || !evidence.Strength.Complete || !evidence.Strength.Audited || !evidence.Strength.Cheap || evidence.Source != filepath.Base(filepath.Dir(path)) {
		t.Fatalf("check did not bind the observed subject tree: %+v", evidence)
	}
	if evidence.RunDirectory != "." {
		t.Fatalf("run directory is not relative to the worktree: %q", evidence.RunDirectory)
	}
}

func TestCarryCheckFillsTheTreeSeam(t *testing.T) {
	t.Parallel()
	b, owners, _, _ := unitCarryIntentBed(t, true)
	subject := branch.AttestationSubject{Commit: "corrected-commit", Tree: strings.Repeat("d", 40), Parent: "base-commit"}
	owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
	owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
	b.declaredCheap, b.declaredAudits, b.declaredDeadline = evidenceSelectionCommand(t), "printf audited", "15"
	original := owners.work.git
	owners.work.git = func(root string, args ...string) ([]byte, error) {
		switch {
		case slices.Equal(args, []string{"show", "-s", "--format=%(trailers:only,unfold=true)", subject.Commit}):
			return []byte("Goal-Unit: " + b.id + "/u1"), nil
		case slices.Equal(args, []string{"rev-parse", "HEAD"}):
			return []byte(subject.Commit), nil
		case len(args) == 2 && args[0] == "rev-parse" && strings.HasSuffix(args[1], "^{tree}"):
			return []byte(subject.Tree), nil
		case slices.Equal(args, []string{"status", "--porcelain", "--untracked-files=all"}):
			return nil, nil
		}
		return original(root, args...)
	}
	code, result := b.runJSON(owners, "work", "review", "--commit", subject.Commit, "--goal", b.id, "--check-only")
	if code != 0 {
		t.Fatalf("carry check: %d %+v", code, result)
	}
	paths, err := filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", subject.Commit, "check-*", "evidence.json"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("carry evidence: %v %v", paths, err)
	}
	checkTreeEvidence(t, paths[0], subject.Tree)
}

func TestIntentUnitCheckFillsTheTreeSeam(t *testing.T) {
	t.Parallel()
	cheap := evidenceSelectionCommand(t)
	b := declaredCheckBed(t, fmt.Sprintf("proof.cheap=%s\nproof.audits=printf audited\nproof.deadline=15\n", cheap))
	starter := &declaredCheckStarter{bed: b, t: t}
	b.manager.Supervisor = starter
	code, built, _ := b.work("work", "build", b.id, "tree-evidence", "--brief", b.brief("tree.md", "Build.\n"), "--lines", "100")
	if code != 0 || len(starter.results) != 2 {
		t.Fatalf("unit check: %d %+v", code, built)
	}
	for _, result := range starter.results {
		path := filepath.Join(resultData(t, result)["execution"].(string), "evidence.json")
		checkTreeEvidence(t, path, "previous-tree")
	}
}
