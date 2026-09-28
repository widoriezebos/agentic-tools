package validate

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// skillAt validates the skill at path, naming it shown in every message.
func skillAt(path, shown string) (string, error) {
	file := shown + "/SKILL.md"
	data, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
	if err != nil {
		return "", fmt.Errorf("missing %s", file)
	}
	lines := strings.Split(string(data), "\n")
	if lines[0] != "---" {
		return "", fmt.Errorf("%s: frontmatter must start on line 1", file)
	}
	closing := -1
	for index := 1; index < len(lines); index++ {
		if lines[index] == "---" {
			closing = index
			break
		}
	}
	if closing < 0 {
		return "", fmt.Errorf("%s: frontmatter is not closed", file)
	}
	name := frontmatterValue(lines[1:closing], "name")
	description := frontmatterValue(lines[1:closing], "description")
	switch {
	case name == "":
		return "", fmt.Errorf("%s: missing name", file)
	case description == "":
		return "", fmt.Errorf("%s: missing description", file)
	case !skillNamePattern.MatchString(name):
		return "", fmt.Errorf("%s: invalid skill name", file)
	case len(name) > 64:
		return "", fmt.Errorf("%s: skill name exceeds 64 characters", file)
	case filepath.Base(path) != name:
		return "", fmt.Errorf("%s: folder and skill name differ", file)
	}
	return name, nil
}

// frontmatterValue returns the first value of key: the line whose text before
// its first colon is exactly the key, less the key, the colon and following
// spaces.
func frontmatterValue(lines []string, key string) string {
	for _, line := range lines {
		before, after, found := strings.Cut(line, ":")
		if found && before == key {
			return strings.TrimLeft(after, " ")
		}
	}
	return ""
}

// SkillInventory validates every skill present under root's skills and
// optional-skills: each skill directory holds a SKILL.md (an empty skill
// directory is an error rather than silently disappearing from the walk), and
// every SKILL.md below them validates. It prints "<name> is valid" per skill
// to out and stops at the first violation.
func SkillInventory(root string, out io.Writer) error {
	for _, tree := range []string{"skills", "optional-skills"} {
		base := filepath.Join(root, tree)
		entries, err := os.ReadDir(base)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			info, statErr := os.Stat(filepath.Join(base, entry.Name()))
			if statErr != nil || !info.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(base, entry.Name(), "SKILL.md")); err != nil {
				return fmt.Errorf("skill directory without SKILL.md: %s/%s", tree, entry.Name())
			}
		}
		var skills []string
		err = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Name() == "SKILL.md" && !entry.IsDir() {
				relative, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}
				skills = append(skills, filepath.ToSlash(relative))
			}
			return nil
		})
		if err != nil {
			return err
		}
		sort.Strings(skills)
		for _, skill := range skills {
			name, err := skillAt(filepath.Join(root, filepath.Dir(skill)), filepath.Dir(skill))
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "%s is valid\n", name)
		}
	}
	return nil
}
