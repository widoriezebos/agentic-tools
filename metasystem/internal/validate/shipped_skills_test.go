package validate

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// Every skill the installation ships, under skills/ and optional-skills/,
// validates; this was the static-contract-audits section's
// `internal validate skills --root` over the real installation.
func TestShippedSkillInventoryValidates(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := SkillInventory(filepath.Join("..", ".."), &out); err != nil {
		t.Fatalf("shipped skill inventory refused: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "valid") {
		t.Fatalf("shipped skill inventory validated nothing:\n%s", out.String())
	}
}
