package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// TestSystemCheckReportsSkillProblems: skills are where users extend the
// metasystem, and system check reads them (the former validate skills): a
// skill directory without its SKILL.md is a problem system check names,
// and a sound inventory is reported as such.
func TestSystemCheckReportsSkillProblems(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	owners.processes.health = func(string, string, time.Time) steward.HealthVerdict {
		return steward.HealthVerdict{Aggregate: "healthy"}
	}
	skill := filepath.Join(b.root(), "skills", "verify")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: verify\ndescription: Prove a change works end to end.\n---\n\nBody.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, result := b.runJSON(owners, "system", "check")
	data, _ := result.Data.(map[string]any)
	// The bed is not set up (system check reports that drift too), so the
	// exit code is not the skills' verdict; the skills' own entry is.
	if code > 1 || !strings.Contains(fmt.Sprint(data["skills"]), "valid:true") || strings.Contains(fmt.Sprint(result.text), "skills invalid") {
		t.Fatalf("sound skills: system check = %d %+v", code, result)
	}
	if err := os.MkdirAll(filepath.Join(b.root(), "skills", "hollow"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, result = b.runJSON(owners, "system", "check")
	data, _ = result.Data.(map[string]any)
	if code != 1 || !strings.Contains(fmt.Sprint(data["skills"]), "skill directory without SKILL.md: skills/hollow") {
		t.Fatalf("a hollow skill: system check = %d %+v", code, result)
	}
}
