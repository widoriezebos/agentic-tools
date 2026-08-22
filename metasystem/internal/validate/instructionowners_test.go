package validate

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The G-5 instruction-owner coverage lint: every document that
// AGENTS.md, the role preambles' quote-source markers, or the host-turn
// template names as a rule owner must appear in
// instruction-bearing-paths.txt — a missed owner silently un-protects a
// canonical instruction file. Adopted repositories receive the suite as
// source and run this under their own full-suite gate, so the lint keeps
// enforcing after delivery.
func TestInstructionOwnersAreInstructionBearing(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(data)
	}
	var entries []string
	for _, line := range strings.Split(read("scripts/agents/instruction-bearing-paths.txt"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			entries = append(entries, line)
		}
	}
	covered := func(path string) bool {
		for _, entry := range entries {
			if strings.HasSuffix(entry, "/") {
				if path == strings.TrimSuffix(entry, "/") || strings.HasPrefix(path, entry) {
					return true
				}
			} else if path == entry {
				return true
			}
		}
		return false
	}
	ownerPattern := regexp.MustCompile("`((?:docs|skills)/[^`]+\\.md|(?:AGENTS|CLAUDE|wow)\\.md)`")
	quotePattern := regexp.MustCompile(`<!-- quote source="([^"]+)" -->`)
	owners := map[string]bool{"AGENTS.md": true}
	for _, line := range strings.Split(read("AGENTS.md"), "\n") {
		lowered := strings.ToLower(line)
		if strings.Contains(lowered, "owns") || strings.Contains(lowered, "only routing index") ||
			strings.Contains(lowered, "lists the project") {
			for _, match := range ownerPattern.FindAllStringSubmatch(line, -1) {
				owners[match[1]] = true
			}
		}
	}
	roles, err := filepath.Glob(filepath.Join(root, "scripts/agents/roles/*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rolePath := range roles {
		data, err := os.ReadFile(rolePath)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range quotePattern.FindAllStringSubmatch(string(data), -1) {
			owners[match[1]] = true
		}
	}
	for _, match := range ownerPattern.FindAllStringSubmatch(read("scripts/agents/templates/host-turn-instruction.md"), -1) {
		owners[match[1]] = true
	}
	var missing []string
	for owner := range owners {
		if !covered(owner) {
			missing = append(missing, owner)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("rule-owning documents missing from instruction-bearing path list: %s", strings.Join(missing, ", "))
	}
}
