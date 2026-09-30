package main

import (
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// The commit boundary's unrecorded-bytes check never counts an agent
// runtime's declared local configuration: a seat with
// .claude/settings.local.json commits a records change.
func TestLandingPathSelectLeavesRuntimeLocalConfigurationOut(t *testing.T) {
	t.Parallel()
	selected, err := landingPathSelect([]string{".claude/settings.local.json", ".codex/config.toml", ".devin/config.local.json",
		".claude/agents/verify.md", "metasystem/cmd/metasystem/main.go"}, "metasystem/")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selected, []string{".claude/agents/verify.md", "metasystem/cmd/metasystem/main.go"}) {
		t.Fatalf("LANDING selection = %v", selected)
	}
}

// A lane change is code when a staged path is in the ENGINE or PAYLOAD
// projection; records, memory and plans are not.
func TestLandingPathSelectCodeIsEngineOrPayload(t *testing.T) {
	t.Parallel()
	code, err := landingPathSelectCode([]string{"metasystem/memory/receipts.log", "metasystem/records/narrator-digest.log",
		"metasystem/plans/x-design.md", "metasystem/internal/gittree/tree.go", "metasystem/skills/verify/SKILL.md"}, "metasystem/")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(code, []string{"metasystem/internal/gittree/tree.go", "metasystem/skills/verify/SKILL.md"}) {
		t.Fatalf("code paths = %v", code)
	}
}

// The admission scope asks test status for the plan's admission subset.
func TestLandingPathVerifyRequestFollowsTheProofScope(t *testing.T) {
	t.Parallel()
	if landingPathScopedVerifyRequest(landpath.VerifyRequest{Root: "r", Tree: "t", Scope: landpath.ProofFull}).BatchAdmission ||
		!landingPathScopedVerifyRequest(landpath.VerifyRequest{Root: "r", Tree: "t", Scope: landpath.ProofAdmission}).BatchAdmission {
		t.Fatal("only the admission scope verifies the admission subset")
	}
}
