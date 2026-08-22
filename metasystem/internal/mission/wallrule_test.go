package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The host-implementer wall's interim rule. The prompt's two live
// authorities must carry this text verbatim: a paraphrase in either
// file would let the assembled prompt drift from the doctrine the
// wall enforces.
const wallRule = "Inside a mission created by the mission runner, the host never authors product bytes, regardless of size or urgency. A mechanically small change may omit a separate design artifact only when the existing contract permits it; implementation still requires an implementer job, critic closure, conformance-issued integration authorization, and exact authorized-patch integration. Until small-change-lane ships, use that ordinary path. A fence refusal parks through the mission runner; it never authorizes host implementation. Interactive work outside the mission runner is unaffected."

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "scripts", "agents", "roles", "orchestrator.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root with scripts/agents/roles/orchestrator.md not found above the package directory")
		}
		dir = parent
	}
}

func TestWallRuleVerbatimInBothLiveAuthorities(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		filepath.Join("scripts", "agents", "templates", "host-turn-instruction.md"),
		filepath.Join("scripts", "agents", "roles", "orchestrator.md"),
	} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), wallRule) {
			t.Errorf("%s does not carry the wall rule verbatim", rel)
		}
	}
}

func TestWallRuleScopesToRunnerMissionsOnly(t *testing.T) {
	// The interactive boundary lives inside the rule itself: work
	// outside missionrunner keeps its ordinary authority, so the
	// rule must open on the runner-created scope and close on the
	// interactive exemption.
	if !strings.HasPrefix(wallRule, "Inside a mission created by the mission runner, ") {
		t.Error("the wall rule does not open on the mission-runner scope")
	}
	if !strings.HasSuffix(wallRule, "Interactive work outside the mission runner is unaffected.") {
		t.Error("the wall rule does not close on the interactive exemption")
	}
}

func TestAssembledPromptCarriesWallRuleBytes(t *testing.T) {
	repo := promptSandbox(t)
	instr := filepath.Join(repo, "scripts", "agents", "templates", "host-turn-instruction.md")
	data, err := os.ReadFile(instr)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(instr, append(data, []byte("\n"+wallRule+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "prompt.md")
	if err := AssemblePrompt(repo, "m1", "t1", out); err != nil {
		t.Fatal(err)
	}
	assembled, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(assembled), wallRule) {
		t.Error("the assembled host-turn prompt does not carry the wall rule byte-exact")
	}
}
