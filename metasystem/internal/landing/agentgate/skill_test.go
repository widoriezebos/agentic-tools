package agentgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The landing-agent skill is the agent's brief (design section 3): it names
// every landing verb the gate admits and only those, the judgement steps,
// the aggregate cap, the typed returns, when to stop and ask, and the
// residual risk the gate does not cover.
func TestLandingAgentSkillMatchesTheGate(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "skills", "landing-agent", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	skill := string(data)
	if !strings.HasPrefix(skill, "---\nname: landing-agent\ndescription: ") {
		t.Fatalf("frontmatter: %q", skill[:min(len(skill), 80)])
	}
	entries, err := Entries()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Kind == "verb" && strings.HasPrefix(entry.Name, "landing ") && !strings.Contains(skill, "metasystem "+entry.Name) {
			t.Errorf("the skill never names metasystem %s", entry.Name)
		}
	}
	for _, forbidden := range []string{"metasystem landing set", "metasystem landing unset", "metasystem landing start", "metasystem landing restart", "git push", "--no-verify", "metasystem test run"} {
		for _, line := range strings.Split(skill, "\n") {
			if strings.Contains(line, forbidden) && !strings.Contains(strings.ToLower(line), "never") && !strings.Contains(strings.ToLower(line), "denied") {
				t.Errorf("the skill names %q outside a prohibition: %q", forbidden, line)
			}
		}
	}
	for _, required := range []string{
		"--subject batch", "--subject base", "--subject member:", "could not run", "unavailable",
		"40 lines", "Lane-Resolved", "Lane-Integration", "seam-too-large",
		"--disposition red", "--disposition conflict", "--record-conflict",
		"metasystem landing stop --reason", "metasystem agent ask", "residual risk", "lane-agent-tools.json",
	} {
		if !strings.Contains(skill, required) {
			t.Errorf("the skill does not say %q", required)
		}
	}
}
