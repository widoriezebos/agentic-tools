package launch

import (
	"strings"
	"testing"
)

// TestLandingRunsOnlyOnTheGatedAdapter (unit A-b, F-2): the landing agent's
// tool gate is a Claude PreToolUse hook, so a landing session on any other
// adapter would run ungated. Every other adapter refuses a landing record,
// and Start refuses a landing lane whose runtime is not claude, in two plain
// lines naming the roster key.
func TestLandingRunsOnlyOnTheGatedAdapter(t *testing.T) {
	t.Parallel()
	record, _ := claudeRecord(t, LandingKind)
	for name, adapter := range map[string]Adapter{"codex-exec": CodexExec{Binary: "codex"}, "devin-print": DevinPrint{Binary: "devin"}, "plain-exec": PlainExec{}} {
		if _, err := adapter.Command(record, t.TempDir()); err == nil || !twoLineLandingRefusal(err.Error()) {
			t.Errorf("%s built a landing command: %v", name, err)
		}
	}
	for _, runtime := range []string{"codex", "devin", "auto", ""} {
		if err := landingRuntimeRefusal(LandingKind, runtime); err == nil || !twoLineLandingRefusal(err.Error()) {
			t.Errorf("landing on runtime %q admitted: %v", runtime, err)
		}
	}
	if err := landingRuntimeRefusal(LandingKind, "claude"); err != nil {
		t.Fatalf("landing on claude refused: %v", err)
	}
	if err := landingRuntimeRefusal("build", "codex"); err != nil {
		t.Fatalf("a build lane met the landing refusal: %v", err)
	}
	m, _, _, _ := manager(t)
	m.Supervisor = childStarter(m)
	if _, err := m.Start(StartSpec{Kind: LandingKind, Tag: "b1", WorkingDirectory: t.TempDir(), Brief: brief(t), Model: "claude-opus-5-5"}); err == nil || !twoLineLandingRefusal(err.Error()) {
		t.Fatalf("Start admitted an ungated landing lane: %v", err)
	}
}

func twoLineLandingRefusal(message string) bool {
	lines := strings.Split(message, "\n")
	return len(lines) == 2 && strings.Contains(lines[0], LandingRuntimeKey) && strings.HasPrefix(lines[1], "run: ")
}
