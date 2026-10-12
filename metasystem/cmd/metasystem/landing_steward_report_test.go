package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillLandingAgentLaneSteward(t *testing.T) {
	t.Parallel()
	module := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(module, "skills", "landing-agent", "SKILL.md"))
	helmMust(t, err)
	_, section, exists := strings.Cut(string(data), "## The lane steward\n")
	if !exists {
		t.Fatal("the landing agent has no steward instructions")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	if len(strings.Split(strings.TrimSpace(section), "\n")) > 25 {
		t.Fatal("steward section exceeds 25 lines")
	}
	for _, want := range []string{"landing health", "start of each host turn", "interventions.jsonl", "fresh proof of main", "stale questions", "silent agents", "unproven incidents", "redeploy", "settle", "never", "person's process", "person's verb", "pushes code"} {
		if !strings.Contains(section, want) {
			t.Fatalf("missing %q in steward instructions", want)
		}
	}
	result, err := audit.AuditMetasystem(module, audit.AuditOptions{AllowPlaceholders: true})
	helmMust(t, err)
	if len(result.Violations) > 0 {
		t.Fatal(result.Violations)
	}
}
