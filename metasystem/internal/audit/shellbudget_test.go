package audit

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeasureShellFile(t *testing.T) {
	content := `#!/usr/bin/env bash
# a comment mentioning python3 and if while for
set -euo pipefail
helper() {
  if [[ -f x ]]; then
    for item in a b; do echo "$item"; done
  fi
}
true && if [[ -d y ]]; then case $1 in a) ;; esac; fi
python3 -c 'print(1)'
bash <<'EOF'
echo hi
EOF
cat <<'DOC'
prose here-doc without a shell word
DOC
`
	metrics := MeasureShellFile(content)
	// The `case` after `then` sits mid-statement, not at a head or after a
	// separator, so the deliberately syntactic counter does not see it.
	want := ShellMetrics{Lines: 16, Constructs: 3, Functions: 1, Python: 1, ShellHeredocs: 1}
	if metrics != want {
		t.Fatalf("metrics %+v, want %+v", metrics, want)
	}
}

// budgetFixture builds a template-shaped checkout: a git repository whose
// go.mod declares the metasystem module, one engine sentinel, a registry,
// and a budget covering the registered scripts.
type budgetFixture struct {
	t    *testing.T
	root string
}

func newBudgetFixture(t *testing.T) *budgetFixture {
	t.Helper()
	f := &budgetFixture{t: t, root: t.TempDir()}
	os.MkdirAll(filepath.Join(f.root, "internal", "missionrunner"), 0o755)
	os.WriteFile(filepath.Join(f.root, "internal", "missionrunner", "stoploss.go"), []byte("package missionrunner\n"), 0o644)
	os.WriteFile(filepath.Join(f.root, "go.mod"),
		[]byte("module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.22\n"), 0o644)
	os.MkdirAll(filepath.Join(f.root, "scripts", "agents"), 0o755)
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "m"}, {"config", "user.email", "m@x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = f.root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return f
}

func (f *budgetFixture) writeScript(relative, content string) {
	f.t.Helper()
	path := filepath.Join(f.root, relative)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(content), 0o755)
	cmd := exec.Command("git", "add", relative)
	cmd.Dir = f.root
	if out, err := cmd.CombinedOutput(); err != nil {
		f.t.Fatalf("git add: %v %s", err, out)
	}
}

func (f *budgetFixture) writeRegistry(scripts []map[string]any) {
	f.t.Helper()
	registry := map[string]any{"scripts": scripts, "tombstones": []any{}, "go-packages": []any{}}
	data, _ := json.MarshalIndent(registry, "", "  ")
	os.WriteFile(filepath.Join(f.root, "scripts", "agents", "shell-dispositions.json"), data, 0o644)
}

func (f *budgetFixture) writeBudget(total int, files map[string]ShellMetrics) string {
	f.t.Helper()
	budget := map[string]any{"note": "test", "totalLines": total, "files": files}
	data, _ := json.MarshalIndent(budget, "", "  ")
	path := filepath.Join(f.root, "scripts", "agents", "shell-budget.json")
	os.WriteFile(path, data, 0o644)
	return path
}

func entry(path, shape string) map[string]any {
	return map[string]any{"path": path, "shape": shape, "verdict": "port+shim", "verified": nil}
}

const shimText = "#!/usr/bin/env bash\nset -euo pipefail\nexec bin/metasystem noop\n"

func TestAuditShellBudget(t *testing.T) {
	f := newBudgetFixture(t)
	f.writeScript("scripts/shim.sh", shimText)
	f.writeScript("scripts/agents/fixture-driver.sh", "#!/usr/bin/env bash\npython3 -c 'print(1)'\n")
	f.writeRegistry([]map[string]any{
		entry("scripts/shim.sh", "exec"),
		entry("scripts/agents/fixture-driver.sh", "sequencer"),
	})
	budgetPath := f.writeBudget(5, map[string]ShellMetrics{
		"scripts/shim.sh":                  {Lines: 3, Constructs: 0, Functions: 0, Python: 0, ShellHeredocs: 0},
		"scripts/agents/fixture-driver.sh": {Lines: 2, Constructs: 0, Functions: 0, Python: 1, ShellHeredocs: 0},
	})
	violations, report := AuditShellBudget(f.root, budgetPath)
	if len(violations) != 0 {
		t.Fatalf("green fixture refused: %v", violations)
	}
	if len(report) != 1 || !strings.Contains(report[0], "2 scripts measured, 5/5 total lines, template=true") {
		t.Fatalf("report wrong: %v", report)
	}

	// A tracked but unregistered script fails the closed world.
	f.writeScript("scripts/rogue.sh", shimText)
	violations, _ = AuditShellBudget(f.root, budgetPath)
	if len(violations) != 1 || !strings.Contains(violations[0], "tracked shell file is not registered: scripts/rogue.sh") {
		t.Fatalf("rogue script not refused: %v", violations)
	}
	exec.Command("git", "-C", f.root, "rm", "-q", "--cached", "scripts/rogue.sh").Run()
	os.Remove(filepath.Join(f.root, "scripts", "rogue.sh"))

	// Production python refuses regardless of any budget allowance.
	f.writeScript("scripts/shim.sh", "#!/usr/bin/env bash\npython3 -c 'print(1)'\nexec true\n")
	violations, _ = AuditShellBudget(f.root, budgetPath)
	if len(violations) != 1 || !strings.Contains(violations[0], "production scripts carry zero python") {
		t.Fatalf("production python not refused: %v", violations)
	}
	f.writeScript("scripts/shim.sh", shimText)

	// Growth beyond a per-file cap refuses.
	f.writeScript("scripts/agents/fixture-driver.sh", "#!/usr/bin/env bash\npython3 -c 'print(1)'\nif true; then echo grow; fi\n")
	violations, _ = AuditShellBudget(f.root, budgetPath)
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "lines 3 exceeds the budget of 2") ||
		!strings.Contains(joined, "control-flow constructs 1 exceeds the budget of 0") ||
		!strings.Contains(joined, "total registered shell lines 6 exceed the budget of 5") {
		t.Fatalf("growth not refused: %v", violations)
	}
	f.writeScript("scripts/agents/fixture-driver.sh", "#!/usr/bin/env bash\npython3 -c 'print(1)'\n")

	// A registered script without a budget entry, and a budget entry for an
	// unregistered script, both refuse.
	f.writeScript("scripts/orphan.sh", shimText)
	f.writeRegistry([]map[string]any{
		entry("scripts/shim.sh", "exec"),
		entry("scripts/agents/fixture-driver.sh", "sequencer"),
		entry("scripts/orphan.sh", "exec"),
	})
	stale := f.writeBudget(20, map[string]ShellMetrics{
		"scripts/shim.sh":                  {Lines: 3},
		"scripts/agents/fixture-driver.sh": {Lines: 2, Python: 1},
		"scripts/gone.sh":                  {Lines: 1},
	})
	violations, _ = AuditShellBudget(f.root, stale)
	joined = strings.Join(violations, "\n")
	if !strings.Contains(joined, "registered script has no budget entry: scripts/orphan.sh") ||
		!strings.Contains(joined, "budget entry names an unregistered script: scripts/gone.sh") {
		t.Fatalf("budget/registry mismatch not refused: %v", violations)
	}
}

func TestAuditShellBudgetDamagedTemplate(t *testing.T) {
	f := newBudgetFixture(t)
	os.WriteFile(filepath.Join(f.root, "go.mod"), []byte("module example.com/other\n"), 0o644)
	f.writeRegistry([]map[string]any{})
	budgetPath := f.writeBudget(0, map[string]ShellMetrics{})
	violations, _ := AuditShellBudget(f.root, budgetPath)
	if len(violations) != 1 || !strings.Contains(violations[0], "the template is damaged") {
		t.Fatalf("damaged template not refused: %v", violations)
	}
}

func TestAuditShellBudgetAdopted(t *testing.T) {
	f := newBudgetFixture(t)
	// No engine sentinel: an adopted checkout. The project's own scripts are
	// never judged, and a pruned registered script is tolerated.
	os.RemoveAll(filepath.Join(f.root, "internal"))
	f.writeScript("scripts/own-tool.sh", shimText) // adopted project's own file
	f.writeRegistry([]map[string]any{
		entry("scripts/shim.sh", "exec"),
		entry("scripts/pruned.sh", "exec"),
	})
	f.writeScript("scripts/shim.sh", shimText)
	budgetPath := f.writeBudget(3, map[string]ShellMetrics{
		"scripts/shim.sh":   {Lines: 3},
		"scripts/pruned.sh": {Lines: 1},
	})
	violations, report := AuditShellBudget(f.root, budgetPath)
	if len(violations) != 0 {
		t.Fatalf("adopted checkout refused: %v", violations)
	}
	if !strings.Contains(report[0], "1 scripts measured") || !strings.Contains(report[0], "template=false") {
		t.Fatalf("adopted report wrong: %v", report)
	}
}

func TestAuditShellBudgetRegistryHygiene(t *testing.T) {
	f := newBudgetFixture(t)
	if violations, _ := AuditShellBudget(f.root, filepath.Join(f.root, "absent-budget.json")); len(violations) != 1 ||
		!strings.Contains(violations[0], "disposition registry is unreadable") {
		t.Fatalf("missing registry tolerated: %v", violations)
	}
	os.WriteFile(filepath.Join(f.root, "scripts", "agents", "shell-dispositions.json"), []byte("not json"), 0o644)
	if violations, _ := AuditShellBudget(f.root, filepath.Join(f.root, "absent-budget.json")); len(violations) != 1 ||
		!strings.Contains(violations[0], "disposition registry is malformed") {
		t.Fatalf("malformed registry tolerated: %v", violations)
	}
	f.writeRegistry([]map[string]any{})
	if violations, _ := AuditShellBudget(f.root, filepath.Join(f.root, "absent-budget.json")); len(violations) != 1 ||
		!strings.Contains(violations[0], "shell budget file is unreadable") {
		t.Fatalf("missing budget tolerated: %v", violations)
	}
	badBudget := filepath.Join(f.root, "bad-budget.json")
	os.WriteFile(badBudget, []byte("not json"), 0o644)
	if violations, _ := AuditShellBudget(f.root, badBudget); len(violations) != 1 ||
		!strings.Contains(violations[0], "shell budget file is malformed") {
		t.Fatalf("malformed budget tolerated: %v", violations)
	}

	// Registry entries missing fields, duplicated, or naming untracked
	// scripts all refuse, and go-package entries must name real paths.
	f.writeRegistry([]map[string]any{
		{"path": "scripts/ghost.sh", "shape": "exec", "verdict": "port+shim"},
		{"path": "scripts/ghost.sh", "shape": "exec", "verdict": "port+shim"},
		{"path": "scripts/broken.sh"},
	})
	registryPath := filepath.Join(f.root, "scripts", "agents", "shell-dispositions.json")
	raw, _ := os.ReadFile(registryPath)
	var parsed map[string]any
	json.Unmarshal(raw, &parsed)
	parsed["go-packages"] = []any{map[string]any{"importPath": "internal/ghostpkg", "plan": "plans/ghost.md"}}
	data, _ := json.MarshalIndent(parsed, "", "  ")
	os.WriteFile(registryPath, data, 0o644)
	budgetPath := f.writeBudget(0, map[string]ShellMetrics{})
	violations, _ := AuditShellBudget(f.root, budgetPath)
	joined := strings.Join(violations, "\n")
	for _, want := range []string{
		"lacks path, shape, or verdict",
		"registry lists scripts/ghost.sh twice",
		"registered script is not tracked: scripts/ghost.sh",
		"registered go package directory is missing: internal/ghostpkg",
		"registered go package plan is missing: plans/ghost.md",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("hygiene violation %q missing from: %v", want, violations)
		}
	}

	// A sequencer's python cap is a ratchet, not a free pass.
	f.writeScript("scripts/agents/fixture-driver.sh", "#!/usr/bin/env bash\npython3 -c 1\npython3 -c 2\n")
	f.writeRegistry([]map[string]any{entry("scripts/agents/fixture-driver.sh", "sequencer")})
	budgetPath = f.writeBudget(10, map[string]ShellMetrics{
		"scripts/agents/fixture-driver.sh": {Lines: 10, Python: 1},
	})
	violations, _ = AuditShellBudget(f.root, budgetPath)
	if len(violations) != 1 || !strings.Contains(violations[0], "python lines 2 exceeds the budget of 1") {
		t.Fatalf("sequencer python ratchet not enforced: %v", violations)
	}
}
