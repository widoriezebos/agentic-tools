package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestFreshDiagnosticEpisodeBindsTheDecision(t *testing.T) {
	t.Parallel()
	first, err := newTestingFreshEpisode()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newTestingFreshEpisode()
	if err != nil || first == second || len(first) != 64 || len(second) != 64 {
		t.Fatalf("fresh diagnostic episodes were not distinct: %q %q err=%v", first, second, err)
	}
	request := proofrun.TestRunRequest{CandidateTree: strings.Repeat("a", 40), BaseCommit: "base-one", PolicyBaseCommit: "base-one",
		ContractDigest: strings.Repeat("b", 64), BaseContractDigest: strings.Repeat("c", 64),
		Plan: testpolicy.Plan{Purpose: testpolicy.PurposeDiagnostic, SelectedGroups: []string{"native"}}}
	groups := map[string]string{"native": strings.Repeat("d", 64)}
	base := testingFreshnessBinding(request, groups, first)
	if base == "" || base != testingFreshnessBinding(request, groups, first) {
		t.Fatal("unchanged decision did not retain its binding")
	}
	changed := request
	changed.BaseCommit = "base-two"
	if testingFreshnessBinding(changed, groups, first) == base {
		t.Fatal("changed base retained the earlier episode binding")
	}
	changed = request
	changed.CandidateTree = strings.Repeat("e", 40)
	if testingFreshnessBinding(changed, groups, first) == base {
		t.Fatal("changed candidate retained the earlier episode binding")
	}
	changed = request
	changed.Plan.SelectedGroups = []string{"native", "added"}
	if testingFreshnessBinding(changed, groups, first) == base {
		t.Fatal("changed plan retained the earlier episode binding")
	}
	changed = request
	changed.FreshnessExpiresAt = "2026-09-21T00:00:00Z"
	if testingFreshnessBinding(changed, groups, first) == base {
		t.Fatal("changed expiry retained the earlier episode binding")
	}
	if testingFreshnessBinding(request, groups, "") != "" {
		t.Fatal("ordinary proof acquired a freshness binding")
	}
}
