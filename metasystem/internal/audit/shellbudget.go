package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The shell complexity budget (plans/kill-shell.md Phase A): a fence that
// refuses regressions the way the word-budget audit fences prompt growth.
// Its jurisdiction is exactly the scripts registered in
// scripts/agents/shell-dispositions.json — metasystem-owned means
// registered; an adopted project's own files are never registered and never
// judged. The budget file's numbers only ratchet down. Syntax counts do not
// prove semantics: the zero-decisions invariant stays with the registry's
// per-script verdict under review discipline.

const metasystemModule = "github.com/widoriezebos/agentic-tools/metasystem"

var (
	constructHead = regexp.MustCompile(`^[ \t]*(if|while|case|for)[ \t(]`)
	constructMid  = regexp.MustCompile(`(;|&&|\|\|)[ \t]*(if|while|case|for)[ \t(]`)
	functionDef   = regexp.MustCompile(`^[ \t]*(function[ \t]+[A-Za-z_][A-Za-z0-9_]*|[A-Za-z_][A-Za-z0-9_-]*[ \t]*\(\))`)
	pythonWord    = regexp.MustCompile(`\bpython3?\b`)
	heredocMark   = regexp.MustCompile(`<<-?`)
	shellWord     = regexp.MustCompile(`\b(bash|sh|zsh)\b`)
	moduleLine    = regexp.MustCompile(`(?m)^module ` + regexp.QuoteMeta(metasystemModule) + `$`)
)

// ShellMetrics is one script's measured complexity.
type ShellMetrics struct {
	Lines         int `json:"lines"`
	Constructs    int `json:"constructs"`
	Functions     int `json:"functions"`
	Python        int `json:"python"`
	ShellHeredocs int `json:"shellHeredocs"`
}

type shellBudgetFile struct {
	Note       string                  `json:"note"`
	TotalLines int                     `json:"totalLines"`
	Files      map[string]ShellMetrics `json:"files"`
}

type registryScript struct {
	Path     string `json:"path"`
	Shape    string `json:"shape"`
	Verdict  string `json:"verdict"`
	Verified any    `json:"verified"`
}

type dispositionRegistry struct {
	Scripts    []registryScript `json:"scripts"`
	Tombstones []any            `json:"tombstones"`
	GoPackages []struct {
		ImportPath string `json:"importPath"`
		Plan       string `json:"plan"`
	} `json:"go-packages"`
}

// MeasureShellFile computes the budget counters for one script's content.
// The counters are deliberately line-based and syntactic: comment lines are
// excluded, and the numbers exist to be ratcheted, not to prove semantics.
func MeasureShellFile(content string) ShellMetrics {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	metrics := ShellMetrics{Lines: len(lines)}
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		if constructHead.MatchString(line) {
			metrics.Constructs++
		}
		metrics.Constructs += len(constructMid.FindAllString(line, -1))
		if functionDef.MatchString(line) {
			metrics.Functions++
		}
		if pythonWord.MatchString(line) {
			metrics.Python++
		}
		if heredocMark.MatchString(line) && shellWord.MatchString(line) {
			metrics.ShellHeredocs++
		}
	}
	return metrics
}

// templateState resolves the three-state module discriminator: the
// metasystem's own checkout runs every check, an adopted target without
// engine source skips the template-only ones, and engine source without the
// metasystem module line is a damaged template that fails loudly.
func templateState(root string) (template bool, violation string) {
	sentinel := false
	for _, path := range []string{
		filepath.Join(root, "internal", "missionrunner", "stoploss.go"),
		filepath.Join(root, "internal", "mission", "ledger.go"),
	} {
		if _, err := os.Stat(path); err == nil {
			sentinel = true
		}
	}
	if !sentinel {
		return false, ""
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !moduleLine.Match(data) {
		return false, "engine source is present but go.mod does not declare the metasystem module; the template is damaged"
	}
	return true, ""
}

// AuditShellBudget runs the complexity fence and returns the violations and
// a short report. Any violation means the fence refuses.
func AuditShellBudget(root, budgetPath string) (violations, report []string) {
	registryPath := filepath.Join(root, "scripts", "agents", "shell-dispositions.json")
	registryData, err := os.ReadFile(registryPath)
	if err != nil {
		return []string{fmt.Sprintf("disposition registry is unreadable: %v", err)}, nil
	}
	var registry dispositionRegistry
	if err := json.Unmarshal(registryData, &registry); err != nil {
		return []string{fmt.Sprintf("disposition registry is malformed: %v", err)}, nil
	}
	budgetData, err := os.ReadFile(budgetPath)
	if err != nil {
		return []string{fmt.Sprintf("shell budget file is unreadable: %v", err)}, nil
	}
	var budget shellBudgetFile
	if err := json.Unmarshal(budgetData, &budget); err != nil {
		return []string{fmt.Sprintf("shell budget file is malformed: %v", err)}, nil
	}

	registered := map[string]registryScript{}
	for _, script := range registry.Scripts {
		if script.Path == "" || script.Shape == "" || script.Verdict == "" {
			violations = append(violations, fmt.Sprintf("registry entry %q lacks path, shape, or verdict", script.Path))
			continue
		}
		if _, duplicate := registered[script.Path]; duplicate {
			violations = append(violations, "registry lists "+script.Path+" twice")
			continue
		}
		registered[script.Path] = script
	}

	template, damaged := templateState(root)
	if damaged != "" {
		return append(violations, damaged), nil
	}

	if template {
		// Closed world: every tracked shell file is registered and every
		// registered live script is tracked. One list, no globs.
		tracked := map[string]bool{}
		lsFiles := exec.Command("git", "-C", root, "ls-files", "*.sh")
		output, err := lsFiles.Output()
		if err != nil {
			return append(violations, fmt.Sprintf("git ls-files failed: %v", err)), nil
		}
		for _, path := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if path != "" {
				tracked[path] = true
			}
		}
		for path := range tracked {
			if _, ok := registered[path]; !ok {
				violations = append(violations, "tracked shell file is not registered: "+path)
			}
		}
		for path := range registered {
			if !tracked[path] {
				violations = append(violations, "registered script is not tracked: "+path)
			}
		}
		for _, entry := range registry.GoPackages {
			if info, err := os.Stat(filepath.Join(root, entry.ImportPath)); err != nil || !info.IsDir() {
				violations = append(violations, "registered go package directory is missing: "+entry.ImportPath)
			}
			if _, err := os.Stat(filepath.Join(root, entry.Plan)); err != nil {
				violations = append(violations, "registered go package plan is missing: "+entry.Plan)
			}
		}
	}

	paths := make([]string, 0, len(registered))
	for path := range registered {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	totalLines := 0
	measuredCount := 0
	for _, path := range paths {
		content, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			// In an adopted checkout an export condition may have pruned
			// the file; the template's closed-world check above already
			// refused a genuinely missing script.
			continue
		}
		measured := MeasureShellFile(string(content))
		measuredCount++
		totalLines += measured.Lines
		cap, budgeted := budget.Files[path]
		if !budgeted {
			violations = append(violations, "registered script has no budget entry: "+path)
			continue
		}
		check := func(name string, got, allowed int) {
			if got > allowed {
				violations = append(violations, fmt.Sprintf("%s: %s %d exceeds the budget of %d", path, name, got, allowed))
			}
		}
		check("lines", measured.Lines, cap.Lines)
		check("control-flow constructs", measured.Constructs, cap.Constructs)
		check("function definitions", measured.Functions, cap.Functions)
		check("shell-interpreter here-docs", measured.ShellHeredocs, cap.ShellHeredocs)
		if registered[path].Shape == "sequencer" {
			check("python lines", measured.Python, cap.Python)
		} else if measured.Python > 0 {
			violations = append(violations, fmt.Sprintf(
				"%s: production scripts carry zero python; found %d python line(s)", path, measured.Python))
		}
	}
	for path := range budget.Files {
		if _, ok := registered[path]; !ok {
			violations = append(violations, "budget entry names an unregistered script: "+path)
		}
	}
	if totalLines > budget.TotalLines {
		violations = append(violations, fmt.Sprintf(
			"total registered shell lines %d exceed the budget of %d", totalLines, budget.TotalLines))
	}
	report = append(report, fmt.Sprintf(
		"shell budget: %d scripts measured, %d/%d total lines, template=%t",
		measuredCount, totalLines, budget.TotalLines, template))
	return violations, report
}
