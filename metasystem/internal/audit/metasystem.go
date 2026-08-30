package audit

import (
	"fmt"
	runtimereg "github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The metasystem audit, the decision engine behind the
// `metasystem audit metasystem` verb: required instruction files, the
// outside-reference scan over the explicit metasystem-owned list, the
// instruction inventories, placeholder checks, the always-loaded word budget,
// and the report-only common-path bundle.

var auditRequiredFiles = []string{
	"AGENTS.md", "wow.md", "metasystem.conf", "docs/project-rules.md",
	"docs/orchestration.md", "docs/collaboration.md",
	"docs/design/design-principles.md", "docs/design/design-obligation-gate.md",
	"memory/README.md", "memory/instruction-ledger.md", "memory/known-issues.md",
}

// The outside-reference scan roots: explicit metasystem-owned files only.
// docs/project-rules.md is project-owned and deliberately absent, and so are
// the audit's own shim and adopt.sh (both legitimately contain dot-dot
// segments for root resolution).
func auditScanRoots() []string {
	return append(runtimereg.InstructionFiles(), []string{
		"wow.md",
		"docs/orchestration.md", "docs/collaboration.md", "docs/working-modes.md",
		"docs/working-with-agents.md", "docs/project-adaptation.md", "docs/metasystem-reconciliation.md",
		"docs/design/design-principles.md", "docs/design/design-obligation-gate.md", "docs/examples",
		"skills", "optional-skills", "meta",
		"scripts/validate-metasystem.sh", "scripts/validate-skill.sh",
		"scripts/refactor-baseline.sh",
		"scripts/receipt.sh", "scripts/enforcement",
		"plans/README.md", "memory/README.md", "memory/instruction-ledger.md", "memory/known-issues.md",
	}...)
}

var (
	// The path pattern anchors on a non-word, non-slash character before the
	// leading slash so prose like "rule/home/owner" does not false-positive.
	auditOutsideRe    = regexp.MustCompile(`(^|[^[:alnum:]/])/(Users|home|root|tmp|var|opt|etc|private|workspace)/|\.\./`)
	auditActiveTODORe = regexp.MustCompile(`TODO|TBD|<one paragraph>|<command>|<paths`)
	// The residue marker grammar (residue-demands-a-token, R-4): a line
	// declaring residue must link the open backlog item that schedules it.
	auditResidueMarkerRe = regexp.MustCompile(`(?m)^\s*RESIDUE:`)
	auditResidueLinkRe   = regexp.MustCompile(`goal:([a-z0-9][a-z0-9-]*)`)
	auditPlaceholderRe   = regexp.MustCompile(`<one paragraph>|<command>|<paths|<policy>|<list them here>|<sources and handling>|<forbidden list>|<location>|<path outside the repository>|<amount and period>|<warning threshold>|<who approves>|<usage source>|<template sha>|<durable evidence root, outside the repository>|<cheapest model class>|<middle model class>|<costliest model class>|<model>`)
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

	hits, err := auditScanFiles(absRoot, auditScanRoots(), auditOutsideRe)
	if err != nil {
		return nil, err
	}
	if len(hits) > 0 {
		result.Report = append(result.Report, hits...)
		result.Violations = append(result.Violations,
			"references outside the metasystem are forbidden in metasystem-owned files")
		return result, nil
	}

	residueHits, err := auditResidueMarkers(absRoot)
	if err != nil {
		return nil, err
	}
	if len(residueHits) > 0 {
		result.Report = append(result.Report, residueHits...)
		result.Violations = append(result.Violations,
			"a RESIDUE: marker must link its open backlog item as goal:<id> (R-4: residue is a scheduled debt, not a prose note)")
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
	instructions, err := auditFindNamed(absRoot, []string{"."}, instructionInventoryNames(), true)
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

	result.Violations = append(result.Violations, auditGoalSystem(absRoot)...)
	qualified, qualErr := auditQualifiedNames(absRoot)
	if qualErr != nil {
		return nil, qualErr
	}
	result.Violations = append(result.Violations, qualified...)
	return result, nil
}

// auditQualifiedNames guards the TRAVELING agent-facing surfaces —
// dispatch templates, role briefs, and skills, which all end up read
// inside project workspaces — against bare words a project may own:
// metasystem prose there says "delegate job", "mission runner",
// "host turn", "mission gate", "recorder event". A bare use with no
// qualifying word in front cannot tell a delegate WHICH system's
// noun it is reading. Self-defined terms qualify themselves (the
// refactor skill's "acceptance gate").
var bareCollision = regexp.MustCompile(`(?i)\b(?:the|a|an|this|each|every|its) (job|runner|turn|gate|event)s?\b`)
var collisionQualifier = regexp.MustCompile(`(?i)(?:delegate|mission|host|recorder|stale|dispatch|goal|go.?gate|start.?gate|quality|acceptance|completion) (?:job|runner|turn|gate|event)s?\b|inside it\b`)

func auditQualifiedNames(root string) ([]string, error) {
	var violations []string
	for _, rel := range []string{
		filepath.Join("scripts", "agents", "templates"),
		filepath.Join("scripts", "agents", "roles"),
		"skills",
		"optional-skills",
	} {
		dir := filepath.Join(root, rel)
		walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			// A file that qualifies a word ONCE has told its reader
			// which system owns it; later bare uses read under that
			// definition (the refactor skill's "acceptance gate",
			// then "the gate" throughout).
			qualifiedAbove := map[string]bool{}
			for i, line := range strings.Split(string(data), "\n") {
				lower := strings.ToLower(line)
				if collisionQualifier.MatchString(line) {
					for _, w := range []string{"job", "runner", "turn", "gate", "event"} {
						if strings.Contains(lower, w) {
							qualifiedAbove[w] = true
						}
					}
					continue
				}
				for _, hit := range bareCollision.FindAllStringSubmatch(line, -1) {
					word := strings.ToLower(strings.TrimSuffix(hit[1], "s"))
					if qualifiedAbove[word] {
						continue
					}
					relPath, _ := filepath.Rel(root, path)
					violations = append(violations, fmt.Sprintf("%s:%d speaks the bare %q on a traveling surface; qualify it (delegate job, mission runner, host turn, ...)", relPath, i+1, strings.TrimSpace(hit[0])))
					break
				}
			}
			return nil
		})
		if walkErr != nil && !os.IsNotExist(walkErr) {
			return nil, walkErr
		}
	}
	return violations, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// auditScanFiles greps the pattern across files and directory trees rooted at
// the named paths, skipping .git and tolerating absent roots (the shim's
// existence filter). Hits come back as file:line:text report lines.
// auditResidueMarkers sweeps the design docs for RESIDUE: markers whose
// line carries no goal:<id> link resolving to a live backlog item
// (plans/goals/<id>.md). Free-prose residue words are not policed — the
// MARKER is the law's grammar, applied forward, nothing migrated.
func auditResidueMarkers(root string) ([]string, error) {
	designs, err := filepath.Glob(filepath.Join(root, "plans", "*design*.md"))
	if err != nil {
		return nil, err
	}
	var hits []string
	for _, path := range designs {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("audit cannot read design doc %s: %w", path, err)
		}
		rel, _ := filepath.Rel(root, path)
		for number, line := range strings.Split(string(data), "\n") {
			if !auditResidueMarkerRe.MatchString(line) {
				continue
			}
			links := auditResidueLinkRe.FindAllStringSubmatch(line, -1)
			if len(links) == 0 {
				hits = append(hits, fmt.Sprintf("%s:%d: RESIDUE marker carries no goal:<id> link", rel, number+1))
				continue
			}
			for _, link := range links {
				if !fileExists(filepath.Join(root, "plans", "goals", link[1]+".md")) {
					hits = append(hits, fmt.Sprintf("%s:%d: RESIDUE link goal:%s does not resolve to an open backlog item", rel, number+1, link[1]))
				}
			}
		}
	}
	return hits, nil
}

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
			// subject must refuse rather than report clean.
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
					// other walk error is.
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

// auditGoalSystem holds the goal system's two instruction checks
// (goal-system GOAL-18 + GOAL-19). The program-start rule is the SOLE
// compensating control for the design's accepted blind spot (intent
// recorded where no sensor reads), so its presence is audited by
// CONTENT, not just file existence; and the delivery contract's
// conformance rows are checked against the enforcement configs the
// distribution actually ships, so the table can never overclaim.
func auditGoalSystem(root string) []string {
	var violations []string

	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil || !strings.Contains(string(agents), "goal open") || !strings.Contains(string(agents), "goal next") {
		violations = append(violations, "AGENTS.md must carry the goal-thread doctrine: programs start with `goal open`, turn ends read `goal next`")
	}
	adaptation, err := os.ReadFile(filepath.Join(root, "docs", "project-adaptation.md"))
	if err != nil || !strings.Contains(string(adaptation), "goal open") {
		violations = append(violations, "docs/project-adaptation.md must carry the goal-open convention")
	}

	// The registry-pointer audit is positive, not just negative: the
	// named operational documents must carry registry-derived pointers,
	// not hand-maintained universes.
	pointerDocs := map[string]string{
		"docs/project-adaptation.md": "runtime registration",
		"docs/orchestration.md":      "runtime\nlist",
		"docs/glossary.md":           "runtime list",
	}
	// README ships only with the TEMPLATE (adoption's payload excludes
	// it — an adopted project's README is the project's own and owes
	// the metasystem nothing). Same marker the registration presence
	// checks ride.
	if fileExists(filepath.Join(root, "development", "metasystem-design.md")) {
		pointerDocs["README.md"] = "runtime list"
	}
	for doc, marker := range pointerDocs {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doc)))
		if err != nil || !strings.Contains(strings.ReplaceAll(string(body), "\n", " "), strings.ReplaceAll(marker, "\n", " ")) {
			violations = append(violations, doc+" must point at the runtime registry ("+strings.ReplaceAll(marker, "\n", " ")+")")
		}
	}

	contractPath := filepath.Join(root, "docs", "design", "turn-verdict-delivery-contract.md")
	contract, err := os.ReadFile(contractPath)
	if err != nil {
		violations = append(violations, "missing required file: docs/design/turn-verdict-delivery-contract.md")
		return violations
	}
	table := string(contract)
	for _, declaration := range runtimereg.All() {
		if declaration.ShippedEnforcementConfig == "" {
			continue
		}
		runtime := declaration.Name
		rowRe := regexp.MustCompile(`(?m)^\| ` + runtime + ` \|`)
		if !rowRe.MatchString(table) {
			violations = append(violations, "delivery contract lacks a conformance row for "+runtime)
			continue
		}
		config := filepath.Join(root, "scripts", "enforcement", declaration.ShippedEnforcementConfig)
		if _, err := os.Stat(config); err != nil {
			violations = append(violations,
				"delivery contract claims a shipped Stop config for "+runtime+" but "+enforcementConfigFor(runtime)+" is absent")
		}
	}
	return violations
}

func enforcementConfigFor(runtime string) string {
	declaration, ok := runtimereg.Lookup(runtime)
	if !ok {
		return ""
	}
	return declaration.ShippedEnforcementConfig
}

// instructionInventoryNames is the audit's instruction filename set:
// the generic names plus every registry-declared instruction file,
// computed when the audit RUNS — an init-time freeze would miss any
// runtime declared after package load.
func instructionInventoryNames() map[string]bool {
	names := map[string]bool{"wow.md": true, "SKILL.md": true, "AGENT.md": true}
	for _, name := range runtimereg.InstructionFiles() {
		names[name] = true
	}
	return names
}
