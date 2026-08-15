package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleOutput = `ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/adapter	(cached)	coverage: 85.9% of statements
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch	1.2s	coverage: 66.8% of statements
?   	github.com/widoriezebos/agentic-tools/metasystem/internal/empty	[no test files]
`

const modulePrefix = "github.com/widoriezebos/agentic-tools/metasystem/"

func testBaseline(t *testing.T, floors map[string]float64) *CoverageBaseline {
	t.Helper()
	return &CoverageBaseline{Floors: floors, Exempt: map[string]string{"cmd/metasystem": "fixture-covered"}}
}

func TestParseCoverage(t *testing.T) {
	measured := ParseCoverage(sampleOutput, modulePrefix)
	if len(measured) != 2 || measured["internal/adapter"] != 85.9 || measured["internal/dispatch"] != 66.8 {
		t.Fatalf("parse wrong: %v", measured)
	}
}

// The ratchet can fail — a deliberately lowered number is caught (the
// prove-the-check-can-fail requirement of production-grade Phase 0c).
func TestCheckCoverageFailsOnDrop(t *testing.T) {
	baseline := testBaseline(t, map[string]float64{"internal/adapter": 85.9, "internal/dispatch": 66.8})
	measured := map[string]float64{"internal/adapter": 85.9, "internal/dispatch": 60.0}
	violations := CheckCoverage(baseline, measured, nil)
	if len(violations) != 1 || !strings.Contains(violations[0], "internal/dispatch coverage 60.0% is below") {
		t.Fatalf("drop not caught: %v", violations)
	}
}

func TestCheckCoveragePassesAtFloor(t *testing.T) {
	baseline := testBaseline(t, map[string]float64{"internal/adapter": 85.9})
	if violations := CheckCoverage(baseline, map[string]float64{"internal/adapter": 85.9}, nil); len(violations) != 0 {
		t.Fatalf("floor equality must pass: %v", violations)
	}
}

// A new package without a floor fails until registered; exempt packages do
// not; a floored package that vanishes from the measurement fails — losing
// sight never passes.
func TestCheckCoverageClosedWorld(t *testing.T) {
	baseline := testBaseline(t, map[string]float64{"internal/adapter": 85.9})
	violations := CheckCoverage(baseline, map[string]float64{
		"internal/adapter": 86.0, "internal/newpkg": 99.0, "cmd/metasystem": 3.5,
	}, nil)
	if len(violations) != 1 || !strings.Contains(violations[0], "internal/newpkg") {
		t.Fatalf("unregistered package not caught: %v", violations)
	}
	violations = CheckCoverage(baseline, map[string]float64{}, nil)
	if len(violations) != 1 || !strings.Contains(violations[0], "was not measured") {
		t.Fatalf("vanished package not caught: %v", violations)
	}
}

// The inventory join (go-production-grade B8): a package the module carries
// that produced no coverage line — a testless package is invisible to
// `go test -cover` output — refuses the ratchet unless exempt.
func TestCheckCoverageInventoryJoin(t *testing.T) {
	baseline := testBaseline(t, map[string]float64{"internal/adapter": 85.9})
	measured := map[string]float64{"internal/adapter": 86.0, "cmd/metasystem": 3.5}
	inventory := []string{"internal/adapter", "internal/testless", "cmd/metasystem"}
	violations := CheckCoverage(baseline, measured, inventory)
	if len(violations) != 1 || !strings.Contains(violations[0], "internal/testless") ||
		!strings.Contains(violations[0], "no coverage") {
		t.Fatalf("testless package not refused: %v", violations)
	}
	// Exemption clears it; a fully measured inventory is silent.
	baseline.Exempt["internal/testless"] = "fixture-only package, exercised end to end by the suite"
	if violations := CheckCoverage(baseline, measured, inventory); len(violations) != 0 {
		t.Fatalf("exempt testless package still refused: %v", violations)
	}
}

func TestReadCoverageBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ratchet.json")
	if err := os.WriteFile(path, []byte(`{"floors":{"internal/x":50.0},"exempt":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline, err := ReadCoverageBaseline(path)
	if err != nil || baseline.Floors["internal/x"] != 50.0 {
		t.Fatalf("read failed: %v %v", err, baseline)
	}
	for name, bad := range map[string]string{
		"empty":      `{"floors":{}}`,
		"range":      `{"floors":{"internal/x":140}}`,
		"unparsable": `nope`,
	} {
		if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadCoverageBaseline(path); err == nil {
			t.Fatalf("%s baseline accepted", name)
		}
	}
}

// The metasystem audit's refusal paths, each a distinct fixture tree.
func TestAuditMetasystemRefusals(t *testing.T) {
	build := func(t *testing.T) string {
		root := t.TempDir()
		for _, dir := range []string{"docs/design", "skills", "scripts/enforcement"} {
			if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		for _, file := range []string{"AGENTS.md", "wow.md", "metasystem.conf",
			"docs/project-rules.md", "docs/orchestration.md", "docs/collaboration.md",
			"docs/design/design-principles.md", "docs/design/design-obligation-gate.md"} {
			if err := os.WriteFile(filepath.Join(root, file), []byte("clean instruction text\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		// The goal-system doctrine and delivery contract (GOAL-18/19):
		// content the audit now requires, not just files.
		os.WriteFile(filepath.Join(root, "AGENTS.md"),
			[]byte("clean instruction text\nprograms start with `goal open`; at turn end read `goal next`\n"), 0o644)
		os.WriteFile(filepath.Join(root, "docs", "project-adaptation.md"),
			[]byte("programs start with `goal open`\n"), 0o644)
		os.WriteFile(filepath.Join(root, "docs", "design", "turn-verdict-delivery-contract.md"),
			[]byte("| claude |\n| codex |\n| devin |\n"), 0o644)
		for _, config := range []string{"claude-code-hooks.json", "codex-hooks.json", "devin-hooks.json"} {
			os.WriteFile(filepath.Join(root, "scripts", "enforcement", config), []byte("{}\n"), 0o644)
		}
		return root
	}
	t.Run("clean passes", func(t *testing.T) {
		result, err := AuditMetasystem(build(t), AuditOptions{})
		if err != nil || len(result.Violations) != 0 {
			t.Fatalf("clean tree refused: %v %v", err, result.Violations)
		}
	})
	t.Run("missing required file", func(t *testing.T) {
		root := build(t)
		os.Remove(filepath.Join(root, "wow.md"))
		result, _ := AuditMetasystem(root, AuditOptions{})
		if len(result.Violations) != 1 || !strings.Contains(result.Violations[0], "missing required file: wow.md") {
			t.Fatalf("missing file not caught: %v", result.Violations)
		}
	})
	t.Run("outside reference", func(t *testing.T) {
		root := build(t)
		os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("see /Users/someone/notes\n"), 0o644)
		result, _ := AuditMetasystem(root, AuditOptions{})
		if len(result.Violations) != 1 || !strings.Contains(result.Violations[0], "outside the metasystem") {
			t.Fatalf("outside reference not caught: %v", result.Violations)
		}
	})
	t.Run("active placeholder", func(t *testing.T) {
		root := build(t)
		os.WriteFile(filepath.Join(root, "wow.md"), []byte("TODO fill this\n"), 0o644)
		result, _ := AuditMetasystem(root, AuditOptions{})
		if len(result.Violations) != 1 || !strings.Contains(result.Violations[0], "unresolved placeholders") {
			t.Fatalf("active placeholder not caught: %v", result.Violations)
		}
	})
	t.Run("adopted placeholder refused unless allowed", func(t *testing.T) {
		root := build(t)
		os.WriteFile(filepath.Join(root, "docs/project-rules.md"), []byte("budget: <amount and period>\n"), 0o644)
		result, _ := AuditMetasystem(root, AuditOptions{})
		if len(result.Violations) != 1 || !strings.Contains(result.Violations[0], "unreplaced placeholders") {
			t.Fatalf("adopted placeholder not caught: %v", result.Violations)
		}
		result, _ = AuditMetasystem(root, AuditOptions{AllowPlaceholders: true})
		if len(result.Violations) != 0 {
			t.Fatalf("allow-placeholders did not tolerate: %v", result.Violations)
		}
	})
	t.Run("word budget", func(t *testing.T) {
		root := build(t)
		result, _ := AuditMetasystem(root, AuditOptions{MaxAlwaysLoadedWords: 3})
		if len(result.Violations) != 1 || !strings.Contains(result.Violations[0], "exceed 3 words") {
			t.Fatalf("budget breach not caught: %v", result.Violations)
		}
	})
}

// The report paths: inventories, budgets, the bundle line, template
// detection, and the binary-file skip.
func TestAuditMetasystemReport(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "metasystem")
	for _, dir := range []string{"docs/design", "skills/demo", "optional-skills/x", "meta", "artifacts/agents"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The template marker beside the checkout: placeholders tolerated.
	if err := os.MkdirAll(filepath.Join(parent, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(parent, "development", "metasystem-design.md"), []byte("design\n"), 0o644)
	for _, file := range []string{"AGENTS.md", "wow.md", "metasystem.conf",
		"docs/project-rules.md", "docs/orchestration.md", "docs/collaboration.md",
		"docs/design/design-principles.md", "docs/design/design-obligation-gate.md"} {
		os.WriteFile(filepath.Join(root, file), []byte("two words\n"), 0o644)
	}
	// Template placeholders in project-rules: tolerated because the marker
	// identifies the template.
	os.WriteFile(filepath.Join(root, "docs/project-rules.md"), []byte("budget: <amount and period>\n"), 0o644)
	os.WriteFile(filepath.Join(root, "skills/demo/SKILL.md"), []byte("a skill\n"), 0o644)
	os.WriteFile(filepath.Join(root, "optional-skills/x/SKILL.md"), []byte("optional\n"), 0o644)
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("programs start with `goal open`; read `goal next`\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs/project-adaptation.md"), []byte("the `goal open` convention\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs/design/turn-verdict-delivery-contract.md"), []byte("| claude |\n| codex |\n| devin |\n"), 0o644)
	for _, config := range []string{"claude-code-hooks.json", "codex-hooks.json", "devin-hooks.json"} {
		os.MkdirAll(filepath.Join(root, "scripts", "enforcement"), 0o755)
		os.WriteFile(filepath.Join(root, "scripts", "enforcement", config), []byte("{}\n"), 0o644)
	}
	os.WriteFile(filepath.Join(root, "artifacts/agents/AGENTS.md"), []byte("runtime state, pruned\n"), 0o644)
	os.WriteFile(filepath.Join(root, "meta/binary.bin"), append([]byte("bin"), 0), 0o644)

	result, err := AuditMetasystem(root, AuditOptions{})
	if err != nil || len(result.Violations) != 0 {
		t.Fatalf("template tree refused: %v %v", err, result.Violations)
	}
	joined := strings.Join(result.Report, "\n")
	for _, want := range []string{
		"Always-loaded words", "Skill inventory", "skills/demo/SKILL.md",
		"optional-skills/x/SKILL.md", "Instruction inventory", "./AGENTS.md",
		"Effective common-path bundle:",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("report missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "artifacts/agents/AGENTS.md") {
		t.Fatalf("artifacts/ was not pruned:\n%s", joined)
	}
}

func TestAuditMetasystemWordCountErrors(t *testing.T) {
	if _, _, err := auditWordCounts(t.TempDir(), []string{"absent.md"}); err == nil {
		t.Fatal("unreadable word-count input accepted")
	}
	if _, err := AuditMetasystem(filepath.Join(t.TempDir(), "nowhere"), AuditOptions{}); err != nil {
		t.Fatal("missing root should refuse via violations, not error:", err)
	}
}

// Remaining branch coverage: symlinked scan roots, unreadable scan files,
// baseline edge shapes, and parse tolerance for malformed percentages.
func TestAuditEdgeBranches(t *testing.T) {
	if got := ParseCoverage("ok pkg coverage: notanumber% of statements\n", ""); len(got) != 0 {
		t.Fatalf("malformed percentage parsed: %v", got)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "r.json")
	os.WriteFile(path, []byte(`{"floors":{"internal/x":-1}}`), 0o644)
	if _, err := ReadCoverageBaseline(path); err == nil {
		t.Fatal("negative floor accepted")
	}
	if _, err := ReadCoverageBaseline(filepath.Join(dir, "absent.json")); err == nil {
		t.Fatal("absent baseline accepted")
	}

	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "skills", ".git"), 0o755)
	os.WriteFile(filepath.Join(root, "skills", ".git", "config"), []byte("/Users/x\n"), 0o644)
	os.WriteFile(filepath.Join(root, "skills", "ok.md"), []byte("fine\n"), 0o644)
	// An absent scan root is legitimate and stays skippable; .git content
	// is pruned. Both must scan clean.
	hits, err := auditScanFiles(root, []string{"skills", "absent-root"}, auditOutsideRe)
	if err != nil || len(hits) != 0 {
		t.Fatalf(".git content or an absent root disturbed the scan: %v %v", hits, err)
	}
	// B7: a policy file that EXISTS but cannot be read must REFUSE the
	// audit. Reporting clean over a file it could not open is the fail-open
	// hole this replaces — the forbidden reference inside it would pass.
	unreadable := filepath.Join(root, "skills", "sealed.md")
	os.WriteFile(unreadable, []byte("/Users/secret\n"), 0o644)
	os.Chmod(unreadable, 0o000)
	defer os.Chmod(unreadable, 0o644)
	if os.Geteuid() != 0 {
		hits, err = auditScanFiles(root, []string{"skills"}, auditOutsideRe)
		if err == nil {
			t.Fatalf("an unreadable policy file scanned clean (fail-open): hits=%v", hits)
		}
		if !strings.Contains(err.Error(), "cannot read") {
			t.Fatalf("refusal does not name the cause: %v", err)
		}
	}
	found, err := auditFindNamed(root, []string{"skills", "gone"}, map[string]bool{"ok.md": true}, false)
	if err != nil || len(found) != 1 {
		t.Fatalf("find misbehaved: %v %v", found, err)
	}
}

// Error propagation through the audit's own steps: a bundle file readable at
// stat time but not at read time, and always-loaded files vanishing between
// the required check and the count.
func TestAuditMetasystemErrorPropagation(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"docs/design", "skills"} {
		os.MkdirAll(filepath.Join(root, dir), 0o755)
	}
	for _, file := range []string{"AGENTS.md", "wow.md", "metasystem.conf",
		"docs/project-rules.md", "docs/orchestration.md", "docs/collaboration.md",
		"docs/design/design-principles.md", "docs/design/design-obligation-gate.md"} {
		os.WriteFile(filepath.Join(root, file), []byte("clean text\n"), 0o644)
	}
	sealed := filepath.Join(root, "AGENTS.md")
	os.Chmod(sealed, 0o000)
	defer os.Chmod(sealed, 0o644)
	if _, err := AuditMetasystem(root, AuditOptions{}); err == nil {
		t.Fatal("unreadable always-loaded file must error")
	}
}

// GOAL-19: the program-start rule is audited by CONTENT — a doctrine file
// present but silent on `goal open` refuses.
func TestDoctrineProgramStartRule(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "design"), 0o755)
	os.MkdirAll(filepath.Join(root, "scripts", "enforcement"), 0o755)
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("no doctrine here\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs", "project-adaptation.md"), []byte("nothing\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs", "design", "turn-verdict-delivery-contract.md"),
		[]byte("| claude |\n| codex |\n| devin |\n"), 0o644)
	violations := auditGoalSystem(root)
	if len(violations) < 2 {
		t.Fatalf("silent doctrine passed the content check: %v", violations)
	}
}

// GOAL-18: a conformance row claiming a shipped config that is absent
// refuses; the full shipped set passes.
func TestConformanceTableAudit(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "design"), 0o755)
	os.MkdirAll(filepath.Join(root, "scripts", "enforcement"), 0o755)
	os.WriteFile(filepath.Join(root, "AGENTS.md"),
		[]byte("programs start with `goal open`; read `goal next`\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs", "project-adaptation.md"),
		[]byte("`goal open` starts programs\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs", "design", "turn-verdict-delivery-contract.md"),
		[]byte("| claude |\n| codex |\n| devin |\n"), 0o644)
	os.WriteFile(filepath.Join(root, "scripts", "enforcement", "claude-code-hooks.json"), []byte("{}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "scripts", "enforcement", "codex-hooks.json"), []byte("{}\n"), 0o644)
	// devin's config deliberately absent: the row overclaims.
	violations := auditGoalSystem(root)
	if len(violations) != 1 || !strings.Contains(violations[0], "devin") {
		t.Fatalf("overclaiming row not caught: %v", violations)
	}
	os.WriteFile(filepath.Join(root, "scripts", "enforcement", "devin-hooks.json"), []byte("{}\n"), 0o644)
	if violations := auditGoalSystem(root); len(violations) != 0 {
		t.Fatalf("full shipped set refused: %v", violations)
	}
}

// The remaining goal-audit branches: a contract missing a runtime's row,
// and a missing contract file naming itself.
func TestConformanceTableMissingRow(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "design"), 0o755)
	os.MkdirAll(filepath.Join(root, "scripts", "enforcement"), 0o755)
	os.WriteFile(filepath.Join(root, "AGENTS.md"),
		[]byte("programs start with `goal open`; read `goal next`\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs", "project-adaptation.md"),
		[]byte("`goal open` starts programs\n"), 0o644)
	// The contract exists but names only claude.
	os.WriteFile(filepath.Join(root, "docs", "design", "turn-verdict-delivery-contract.md"),
		[]byte("| claude |\n"), 0o644)
	os.WriteFile(filepath.Join(root, "scripts", "enforcement", "claude-code-hooks.json"), []byte("{}\n"), 0o644)
	violations := auditGoalSystem(root)
	if len(violations) != 2 {
		t.Fatalf("missing rows not caught pairwise: %v", violations)
	}
	for _, v := range violations {
		if !strings.Contains(v, "conformance row") {
			t.Fatalf("wrong violation shape: %v", violations)
		}
	}

	// No contract file at all names itself and stops there.
	os.Remove(filepath.Join(root, "docs", "design", "turn-verdict-delivery-contract.md"))
	violations = auditGoalSystem(root)
	if len(violations) != 1 || !strings.Contains(violations[0], "turn-verdict-delivery-contract.md") {
		t.Fatalf("missing contract not named: %v", violations)
	}
}
