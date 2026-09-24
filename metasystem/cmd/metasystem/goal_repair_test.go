package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestGoalRepairRequiresAcceptRemoteAndNamesItInUsage(t *testing.T) {
	stderr, code := captureStderr(t, func() int { return runGoalRepair(nil) })
	if code != 2 || !strings.Contains(stderr,
		"usage: metasystem goal repair --accept-remote --by <human> --root <checkout>") {
		t.Fatalf("bare goal repair did not print usage naming --accept-remote: code=%d stderr=%q", code, stderr)
	}
}

func TestGoalRepairReachesAcceptRemoteAndRequiresBy(t *testing.T) {
	repository := newProofAdmissionRepositoryFixture(t, time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC), false)
	root := repository.root
	repository.mu.Lock()
	accepted := repository.accepted
	canonical := repository.next(accepted, "", repository.commits[accepted].files)
	repository.canonical = canonical
	repository.mu.Unlock()
	facts := goalAuthorityReadFacts{
		repositoryTop: repository.receiptTop,
		ledgerIdentity: func(got string) string {
			if got != root {
				t.Fatalf("ledger identity root = %q, want %q", got, root)
			}
			return goal.ExistingLedgerIdentityAtEndpoint(goal.Endpoint{Root: root, Repository: repository})
		},
	}
	resolveEndpoint := func(got string) (goal.Endpoint, error) {
		if got != root {
			t.Fatalf("repair endpoint root %q differs from %q", got, root)
		}
		var configReads []string
		endpoint, err := goal.ResolveEndpointWithConfig(got, func(configRoot, key string) (string, error) {
			if configRoot != root {
				t.Fatalf("repair config root %q differs from %q", configRoot, root)
			}
			configReads = append(configReads, key)
			switch key {
			case "goal.sync-remote":
				return "local", nil
			case "goal.sync-branch":
				return goal.LocalLedgerBranch, nil
			default:
				t.Fatalf("unexpected repair config key %q", key)
				return "", nil
			}
		})
		if err == nil && (endpoint.Remote != "local" || endpoint.Branch != goal.LocalLedgerBranch ||
			strings.Join(configReads, ",") != "goal.sync-remote,goal.sync-branch") {
			t.Fatalf("repair endpoint config: remote=%q branch=%q reads=%q", endpoint.Remote, endpoint.Branch, configReads)
		}
		endpoint.Repository = repository
		return endpoint, err
	}
	entries, err := goal.Entries(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("initial repair journal: entries=%+v err=%v", entries, err)
	}
	stdout, stderr, code := captureRelay(t, func() int {
		return runGoalRepairWithInputs([]string{"--root", root, "--accept-remote"}, facts, resolveEndpoint)
	})
	if code != 1 || stdout != "" || !strings.Contains(stderr, "repair --accept-remote is a human-reserved act") ||
		!strings.Contains(stderr, "--by") {
		t.Fatalf("goal repair did not relay RepairAcceptRemote's human attribution refusal: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	entries, err = goal.Entries(root)
	if err != nil || len(entries) != 0 || repository.accepted != accepted || repository.canonical != canonical {
		t.Fatalf("missing --by changed repair state: entries=%+v err=%v accepted=%s canonical=%s", entries, err, repository.accepted, repository.canonical)
	}

	stdout, stderr, code = captureRelay(t, func() int {
		return runGoalRepairWithInputs([]string{"--root", root, "--accept-remote", "--by", "Wido"}, facts, resolveEndpoint)
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "advanced=true tip=") ||
		!strings.Contains(stdout, "repair by Wido accepted") {
		t.Fatalf("goal repair did not print the accepted AdvanceResult: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	want := fmt.Sprintf("advanced=true tip=%s repair by Wido accepted %s (was %s)\n", canonical, canonical[:12], accepted[:12])
	if stdout != want || repository.accepted != canonical || repository.canonical != canonical {
		t.Fatalf("repair result or pointers differ: stdout=%q want=%q accepted=%s canonical=%s", stdout, want, repository.accepted, repository.canonical)
	}
	entries, err = goal.Entries(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("accepted repair journal: entries=%+v err=%v", entries, err)
	}
	entry := entries[0]
	if entry.Phase != goal.PhaseTerminal || entry.Outcome != goal.OutcomeConfirmed ||
		entry.Intent.Verb != "repair-accept-remote" || entry.Intent.Args["by"] != "Wido" ||
		entry.Intent.Args["oldTip"] != accepted || entry.Intent.Args["newTip"] != canonical {
		t.Fatalf("accepted repair journal entry = %+v", entry)
	}
}
