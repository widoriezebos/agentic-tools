package validate

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass"
)

// The G-5 instruction-owner coverage lint: every document that
// AGENTS.md, the role preambles' quote-source markers, or the host-turn
// template names as a rule owner must classify as behavior. Adopted
// repositories receive the suite as source and run this under their own
// full-suite gate, so the lint keeps enforcing after delivery.
func TestInstructionOwnersAreBehavior(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if os.IsNotExist(err) {
			// A frozen witness export carries only the ENGINE closure;
			// a repository-content audit is meaningful only where the
			// content lives (live trees and full copies), same skip
			// idiom as the migration-manifest test.
			t.Skipf("repository content absent here (%s); frozen exports carry only the engine closure", rel)
		}
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(data)
	}
	classes, err := pathclass.Load(root)
	if os.IsNotExist(err) {
		t.Skip("repository content absent here; frozen exports carry only the engine closure")
	}
	if err != nil {
		t.Fatalf("load path class manifest: %v", err)
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
		if classes.Class(owner) != pathclass.Behavior {
			missing = append(missing, owner)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("rule-owning documents not classified as behavior: %s", strings.Join(missing, ", "))
	}
}
