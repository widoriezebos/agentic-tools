package main

// The room's candidate (g1-s69 D3, §8): a project with no launch contract is
// refused in words before anything runs, and status carries the running
// commit and since when.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

func TestTheCandidateOfAProjectWithNoLaunchContractIsRefusedInWords(t *testing.T) {
	t.Parallel()
	installation := t.TempDir()
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	roots := lifecycle.Roots{Checkout: installation, Installation: installation, StateRoot: installation}
	for _, action := range []string{"status", "start", "stop"} {
		_, err := candidateRunWith(roots, "g", "", action, intentOwners{})
		refusal, ok := err.(*httpd.CandidateRefusal)
		if !ok || refusal.Code != httpd.CodeNoContract || !strings.Contains(refusal.Message, "this goal's candidate cannot run from here") {
			t.Fatalf("%s without a contract = %v", action, err)
		}
	}
}

func TestAppStatusCarriesTheRunningCommitAndSince(t *testing.T) {
	t.Parallel()
	data := appData(appRun{key: "goal-g", goal: "g"}, applaunch.Status{
		State: applaunch.Running, Readiness: applaunch.Answering, Since: "2026-09-29T09:00:00Z",
		Record: &applaunch.Record{Address: "127.0.0.1:7981", Commit: "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2"},
	})
	if data["commit"] != "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2" || data["since"] != "2026-09-29T09:00:00Z" || data["address"] != "127.0.0.1:7981" {
		t.Fatalf("app status data = %v", data)
	}
}

// RF-04: a review's candidate runs at the commit the review names, through the
// app verbs' own --at; with no commit named it is the goal's branch, as before.
// Anything but a whole commit id is refused before anything runs.
func TestTheCandidateOfAReviewRunsAtTheReviewedCommit(t *testing.T) {
	t.Parallel()
	commit := "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2"
	if got := strings.Join(candidateTarget("g", commit), " "); got != "--at "+commit {
		t.Fatalf("the reviewed commit's run = %q", got)
	}
	if got := strings.Join(candidateTarget("g", ""), " "); got != "--goal g" {
		t.Fatalf("the goal's run = %q", got)
	}
	roots := lifecycle.Roots{Checkout: t.TempDir(), Installation: t.TempDir(), StateRoot: t.TempDir()}
	_, err := candidateRunWith(roots, "g", "main", "start", intentOwners{})
	if refusal, ok := err.(*httpd.CandidateRefusal); !ok || !strings.Contains(refusal.Message, "main is not one") {
		t.Fatalf("a ref that is not a commit = %v", err)
	}
}
