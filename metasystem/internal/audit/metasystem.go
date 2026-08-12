package audit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The metasystem audit (ported whole from scripts/audit-metasystem.sh under
// plans/kill-shell.md Phase A): required instruction files, the
// outside-reference scan over the explicit metasystem-owned list, the
// instruction inventories, placeholder checks, the always-loaded word budget,
// and the report-only common-path bundle.

var auditRequiredFiles = []string{
	"AGENTS.md", "wow.md", "metasystem.conf", "docs/project-rules.md",
	"docs/orchestration.md", "docs/collaboration.md",
	"docs/design/design-principles.md", "docs/design/design-obligation-gate.md",
}

// The outside-reference scan roots: explicit metasystem-owned files only.
// docs/project-rules.md is project-owned and deliberately absent, and so are
// the audit's own shim and adopt.sh (both legitimately contain dot-dot
// segments for root resolution).
var auditScanRoots = []string{
	"AGENTS.md", "CLAUDE.md", "wow.md",
	"docs/orchestration.md", "docs/collaboration.md", "docs/working-modes.md",
	"docs/working-with-agents.md", "docs/project-adaptation.md", "docs/metasystem-reconciliation.md",
	"docs/design/design-principles.md", "docs/design/design-obligation-gate.md", "docs/examples",
	"skills", "optional-skills", "meta",
	"scripts/validate-metasystem.sh", "scripts/validate-skill.sh",
	"scripts/assert-design-obligation-gate.sh", "scripts/refactor-baseline.sh", "scripts/frontier.sh",
	"scripts/receipt.sh", "scripts/assert-stop-loss.sh", "scripts/enforcement",
	"plans/README.md", "plans/instruction-ledger.md", "plans/known-issues.md",
}

var (
	// The path pattern anchors on a non-word, non-slash character before the
	// leading slash so prose like "rule/home/owner" does not false-positive.
	auditOutsideRe     = regexp.MustCompile(`(^|[^[:alnum:]/])/(Users|home|root|tmp|var|opt|etc|private|workspace)/|\.\./`)
	auditActiveTODORe  = regexp.MustCompile(`TODO|TBD|<one paragraph>|<command>|<paths`)
	auditPlaceholderRe = regexp.MustCompile(`<one paragraph>|<command>|<paths|<policy>|<list them here>|<sources and handling>|<forbidden list>|<location>|<path outside the repository>|<amount and period>|<warning threshold>|<who approves>|<usage source>|<template sha>|<durable evidence root, outside the repository>|<cheapest model class>|<middle model class>|<costliest model class>|<model>`)
)

// AuditResult carries the audit's verdict: refusals, and the informational
// report lines the callers print.
type AuditResult struct {
	Violations []string
	Report     []string
}

// AuditOptions mirror the shim's environment knobs.
type AuditOptions struct {
	MaxAlwaysLoadedWords int  // 0 means the 1400 default
	AllowPlaceholders    bool // adopt.sh's structural pass, pre-fill
}

// AuditMetasystem runs the full audit under root.
func AuditMetasystem(root string, opts AuditOptions) (*AuditResult, error) {
	result := &AuditResult{}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	for _, file := range auditRequiredFiles {
		if _, err := os.Stat(filepath.Join(absRoot, file)); err != nil {
			result.Violations = append(result.Violations, "missing required file: "+file)
			return result, nil
		}
	}

	hits, err := auditScanFiles(absRoot, auditScanRoots, auditOutsideRe)
	if err != nil {
		return nil, err
	}
	if len(hits) > 0 {
		result.Report = append(result.Report, hits...)
		result.Violations = append(result.Violations,
			"references outside the metasystem are forbidden in metasystem-owned files")
		return result, nil
	}

	alwaysWords, perFile, err := auditWordCounts(absRoot, []string{"AGENTS.md", "wow.md"})
	if err != nil {
		return nil, err
	}
	result.Report = append(result.Report, "Always-loaded words")
	result.Report = append(result.Report, perFile...)
	result.Report = append(result.Report, fmt.Sprintf("%8d total", alwaysWords))

	skillRoots := []string{"skills"}
	if _, err := os.Stat(filepath.Join(absRoot, "optional-skills")); err == nil {
		skillRoots = append(skillRoots, "optional-skills")
	}
	result.Report = append(result.Report, "Skill inventory")
	skills, err := auditFindNamed(absRoot, skillRoots, map[string]bool{"SKILL.md": true}, false)
	if err != nil {
		return nil, err
	}
	result.Report = append(result.Report, skills...)

	result.Report = append(result.Report, "Instruction inventory")
	instructions, err := auditFindNamed(absRoot, []string{"."},
		map[string]bool{"AGENTS.md": true, "CLAUDE.md": true, "wow.md": true, "SKILL.md": true, "AGENT.md": true}, true)
	if err != nil {
		return nil, err
	}
	result.Report = append(result.Report, instructions...)

	activeScan := append([]string{"AGENTS.md", "wow.md"}, skillRoots...)
	todoHits, err := auditScanFiles(absRoot, activeScan, auditActiveTODORe)
	if err != nil {
		return nil, err
	}
	if len(todoHits) > 0 {
		result.Report = append(result.Report, todoHits...)
		result.Violations = append(result.Violations, "unresolved placeholders in active instructions")
		return result, nil
	}

	// Template detection uses a positive marker: only the template checkout
	// is a folder literally named metasystem with the development docs
	// beside it. Everywhere else project-rules.md must be filled in.
	isTemplate := filepath.Base(absRoot) == "metasystem" &&
		fileExists(filepath.Join(filepath.Dir(absRoot), "development", "metasystem-design.md"))
	if !isTemplate && !opts.AllowPlaceholders {
		placeholderHits, err := auditScanFiles(absRoot,
			[]string{"docs/project-rules.md", "metasystem.conf"}, auditPlaceholderRe)
		if err != nil {
			return nil, err
		}
		if len(placeholderHits) > 0 {
			result.Report = append(result.Report, placeholderHits...)
			result.Violations = append(result.Violations,
				"adopted repository has unreplaced placeholders in docs/project-rules.md or metasystem.conf")
			return result, nil
		}
	}

	maxWords := opts.MaxAlwaysLoadedWords
	if maxWords == 0 {
		maxWords = 1400
	}
	if alwaysWords > maxWords {
		result.Violations = append(result.Violations,
			fmt.Sprintf("always-loaded instructions exceed %d words", maxWords))
		return result, nil
	}

	// Report only, uncapped: the effective per-task footprint.
	bundle := []string{"AGENTS.md", "wow.md", "docs/project-rules.md",
		"docs/collaboration.md", "docs/design/design-obligation-gate.md", "docs/design/design-principles.md"}
	var present []string
	for _, file := range bundle {
		if fileExists(filepath.Join(absRoot, file)) {
			present = append(present, file)
		}
	}
	bundleWords, _, err := auditWordCounts(absRoot, present)
	if err != nil {
		return nil, err
	}
	result.Report = append(result.Report,
		fmt.Sprintf("Effective common-path bundle: %d words (report only)", bundleWords))
	return result, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// auditScanFiles greps the pattern across files and directory trees rooted at
// the named paths, skipping .git and tolerating absent roots (the shim's
// existence filter). Hits come back as file:line:text report lines.
func auditScanFiles(root string, paths []string, pattern *regexp.Regexp) ([]string, error) {
	var hits []string
	for _, rel := range paths {
		full := filepath.Join(root, rel)
		info, err := os.Stat(full)
		if err != nil {
			// An absent scan root is legitimate — not every repository
			// carries every optional tree. Anything else (a permission
			// denial, an I/O error) means the audit cannot see a file it
			// is supposed to police, and an audit that cannot read its
			// subject must refuse rather than report clean
			// (go-production-grade B7).
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("audit cannot stat scan root %s: %w", rel, err)
		}
		var files []string
		if info.IsDir() {
			err := filepath.WalkDir(full, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					// A runtime path that vanished mid-walk (a lock
					// directory, a temp file) is not a policy failure; any
					// other walk error is (B7).
					if os.IsNotExist(err) {
						return nil
					}
					return fmt.Errorf("audit cannot walk %s: %w", path, err)
				}
				if entry.IsDir() {
					if entry.Name() == ".git" {
						return filepath.SkipDir
					}
					return nil
				}
				files = append(files, path)
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			files = []string{full}
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				// Same rule as the walk: gone is fine, unreadable is not.
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("audit cannot read %s: %w", file, err)
			}
			if !isTextLike(data) {
				continue
			}
			relFile, _ := filepath.Rel(root, file)
			for number, line := range strings.Split(string(data), "\n") {
				if pattern.MatchString(line) {
					hits = append(hits, fmt.Sprintf("%s:%d:%s", relFile, number+1, line))
				}
			}
		}
	}
	return hits, nil
}

func isTextLike(data []byte) bool {
	return !strings.Contains(string(data[:min(len(data), 1024)]), "\x00")
}

func auditWordCounts(root string, files []string) (int, []string, error) {
	total := 0
	var perFile []string
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return 0, nil, fmt.Errorf("word count input unreadable: %s: %w", rel, err)
		}
		words := len(strings.Fields(string(data)))
		total += words
		perFile = append(perFile, fmt.Sprintf("%8d %s", words, rel))
	}
	return total, perFile, nil
}

// auditFindNamed lists files with the given basenames under the roots, sorted,
// pruning artifacts/ (runtime state whose lock directories vanish mid-walk).
func auditFindNamed(root string, dirs []string, names map[string]bool, pruneArtifacts bool) ([]string, error) {
	var found []string
	for _, dir := range dirs {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if entry.Name() == ".git" || (pruneArtifacts && entry.Name() == "artifacts" &&
					filepath.Dir(path) == root) {
					return filepath.SkipDir
				}
				return nil
			}
			if names[entry.Name()] {
				rel, _ := filepath.Rel(root, path)
				if dir == "." {
					rel = "./" + rel
				}
				found = append(found, rel)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(found)
	return found, nil
}
