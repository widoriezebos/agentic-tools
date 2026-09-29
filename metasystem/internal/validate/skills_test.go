package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, root, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Every frontmatter rule the retired validate-skill.sh enforced, each with
// its message.
func TestSkillFrontmatterRules(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, dir, body, want string }{
		{"valid with extra keys", "verify", "---\nname: verify\ndescription: Prove it.\nmodel: any\n---\nbody\n", ""},
		{"no opening", "verify", "name: verify\n---\n", "skills/verify/SKILL.md: frontmatter must start on line 1"},
		{"not closed", "verify", "---\nname: verify\ndescription: d\n", "frontmatter is not closed"},
		{"missing name", "verify", "---\ndescription: d\n---\n", "skills/verify/SKILL.md: missing name"},
		{"empty name", "verify", "---\nname:\ndescription: d\n---\n", "missing name"},
		{"missing description", "verify", "---\nname: verify\n---\n", "missing description"},
		{"invalid name", "Verify", "---\nname: Verify\ndescription: d\n---\n", "invalid skill name"},
		{"double hyphen", "a--b", "---\nname: a--b\ndescription: d\n---\n", "invalid skill name"},
		{"too long", strings.Repeat("a", 65), "---\nname: " + strings.Repeat("a", 65) + "\ndescription: d\n---\n", "exceeds 64 characters"},
		{"folder differs", "verify", "---\nname: refactor\ndescription: d\n---\n", "folder and skill name differ"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeSkill(t, root, filepath.Join("skills", test.dir), test.body)
			name, err := skillAt(filepath.Join(root, "skills", test.dir), "skills/"+test.dir)
			if test.want == "" {
				if err != nil || name != test.dir {
					t.Fatalf("name=%q err=%v", name, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want %q", err, test.want)
			}
		})
	}
	if _, err := skillAt(filepath.Join(t.TempDir(), "absent"), "absent"); err == nil || !strings.Contains(err.Error(), "missing ") {
		t.Fatalf("absent skill: %v", err)
	}
}

func TestSkillInventoryValidatesEveryPresentSkill(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSkill(t, root, "skills/verify", "---\nname: verify\ndescription: d\n---\n")
	writeSkill(t, root, "skills/refactor", "---\nname: refactor\ndescription: d\n---\n")
	writeSkill(t, root, "optional-skills/debug-java", "---\nname: debug-java\ndescription: d\n---\n")
	var out strings.Builder
	if err := SkillInventory(root, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "refactor is valid\nverify is valid\ndebug-java is valid\n" {
		t.Fatalf("output=%q", out.String())
	}

	if err := os.Mkdir(filepath.Join(root, "skills", "hollow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := SkillInventory(root, &out); err == nil || err.Error() != "skill directory without SKILL.md: skills/hollow" {
		t.Fatalf("hollow: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "skills", "hollow")); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, root, "skills/verify", "---\ndescription: d\n---\n")
	if err := SkillInventory(root, &out); err == nil || err.Error() != "skills/verify/SKILL.md: missing name" {
		t.Fatalf("broken: %v", err)
	}
}
