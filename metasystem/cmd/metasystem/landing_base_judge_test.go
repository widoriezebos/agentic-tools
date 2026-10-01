package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The base judge reads the engine it built through each child's --json
// envelope (structured-output U2, T7/T11). Driven against this tree's real
// engine, the workspace and the observation arrive as typed data equal to
// the in-process answers; an engine that answers in bare words is read as
// no answer.
func TestBaseJudgeReadsTheRealEnginesEnvelopes(t *testing.T) {
	root := t.TempDir()
	runReceiptGit(t, root, "init", "-q", "-b", "main")
	writeReceiptFixture(t, root, "application.txt", "stable application input\n")
	runReceiptGit(t, root, "add", "-A")
	tree := runReceiptGit(t, root, "write-tree")

	judge := landingEngineJudge(intentTestEngine(t), "digest")
	workspace, err := judge.Workspace(root, tree)
	want, wantErr := landing.ProjectWorkspaceTree(root, tree)
	if err != nil || wantErr != nil || workspace != want {
		t.Fatalf("base judge workspace = %q, %v; in process %q, %v", workspace, err, want, wantErr)
	}
	observed, status := judge.Observe(landpath.ObserveRequest{Root: root, Tree: tree})
	if status != 0 || observed.Mode == "" || observed.VerdictTrailer == "" {
		t.Fatalf("base judge observation = %+v (exit %d)", observed, status)
	}

	words := filepath.Join(t.TempDir(), "metasystem")
	if err := testexec.WriteFile(words, []byte("#!/bin/sh\nprintf '%s\\n' "+strings.Repeat("a", 40)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	wordJudge := landingEngineJudge(words, "digest")
	if got, err := wordJudge.Workspace(root, tree); err == nil {
		t.Fatalf("a bare-word workspace answer was accepted: %q", got)
	}
	if got, status := wordJudge.Observe(landpath.ObserveRequest{Root: root, Tree: tree}); status == 0 || got.Mode != "" {
		t.Fatalf("a bare-word observation was accepted: %+v (exit %d)", got, status)
	}
	if payload, status := wordJudge.VerifyCarried(root, tree, ""); status == 0 || len(payload) != 0 {
		t.Fatalf("a bare-word verify answer was accepted: %q (exit %d)", payload, status)
	}
}
