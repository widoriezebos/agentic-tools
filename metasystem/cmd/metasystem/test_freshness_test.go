package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

func TestFreshDiagnosticEpisodeBindsTheDecision(t *testing.T) {
	t.Parallel()
	first, err := testrun.NewFreshEpisode()
	if err != nil {
		t.Fatal(err)
	}
	second, err := testrun.NewFreshEpisode()
	if err != nil || first == second || len(first) != 64 || len(second) != 64 {
		t.Fatalf("fresh diagnostic episodes were not distinct: %q %q err=%v", first, second, err)
	}
	request := proofrun.TestRunRequest{CandidateTree: strings.Repeat("a", 40), BaseCommit: "base-one", PolicyBaseCommit: "base-one",
		ContractDigest: strings.Repeat("b", 64), BaseContractDigest: strings.Repeat("c", 64),
		Plan: testpolicy.Plan{Purpose: testpolicy.PurposeDiagnostic, SelectedGroups: []string{"native"}}}
	groups := map[string]string{"native": strings.Repeat("d", 64)}
	base := testrun.FreshnessBinding(request, groups, first)
	if base == "" || base != testrun.FreshnessBinding(request, groups, first) {
		t.Fatal("unchanged decision did not retain its binding")
	}
	changed := request
	changed.BaseCommit = "base-two"
	if testrun.FreshnessBinding(changed, groups, first) == base {
		t.Fatal("changed base retained the earlier episode binding")
	}
	changed = request
	changed.CandidateTree = strings.Repeat("e", 40)
	if testrun.FreshnessBinding(changed, groups, first) == base {
		t.Fatal("changed candidate retained the earlier episode binding")
	}
	changed = request
	changed.Plan.SelectedGroups = []string{"native", "added"}
	if testrun.FreshnessBinding(changed, groups, first) == base {
		t.Fatal("changed plan retained the earlier episode binding")
	}
	changed = request
	changed.FreshnessExpiresAt = "2026-09-21T00:00:00Z"
	if testrun.FreshnessBinding(changed, groups, first) == base {
		t.Fatal("changed expiry retained the earlier episode binding")
	}
	if testrun.FreshnessBinding(request, groups, "") != "" {
		t.Fatal("ordinary proof acquired a freshness binding")
	}
}
