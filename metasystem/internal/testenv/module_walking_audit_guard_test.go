package testenv

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// walkCall is a directory walk; with a climb to the module root it walks it.
var walkCall = regexp.MustCompile(`filepath\.Walk(Dir)?\(`)

// walkCarried predate this guard and await their move; none is added.
var walkCarried = map[string]bool{
	"internal/adopt/adopt_git_integration_test.go": true,
	"internal/pathclass/path_class_bed_test.go":    true,
}

// A test that walks the module runs in a group the plan always selects, so a
// change anywhere it reads selects it (goal a-unit-check-runs-the-audits,
// F-1). The always groups and their packages come from testing.json.
func TestAModuleWalkingTestLivesInAnAuditGroup(t *testing.T) {
	t.Parallel()
	module := filepath.Join("..", "..")
	var plan struct {
		Groups []struct {
			ID       string   `json:"id"`
			Packages []string `json:"packages"`
		} `json:"groups"`
		Always map[string][]string `json:"always"`
	}
	data, err := os.ReadFile(filepath.Join(module, "testing.json"))
	if err == nil {
		err = json.Unmarshal(data, &plan)
	}
	if err != nil {
		t.Fatal(err)
	}
	always := map[string]bool{}
	for _, ids := range plan.Always {
		for _, id := range ids {
			always[id] = true
		}
	}
	audited := map[string]bool{}
	for _, group := range plan.Groups {
		for _, pkg := range group.Packages {
			audited[pkg] = audited[pkg] || always[group.ID]
		}
	}
	var offenders []string
	err = filepath.WalkDir(module, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case "vendor", "testdata", "node_modules", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(module, path)
		if relative = filepath.ToSlash(relative); err != nil || !strings.HasSuffix(relative, "_test.go") {
			return err
		}
		dir := filepath.ToSlash(filepath.Dir(relative))
		if audited[dir] || walkCarried[relative] {
			return nil
		}
		source, err := os.ReadFile(path)
		// The climb to the module root: one ".." per directory below it.
		dots := strings.TrimSuffix(strings.Repeat(`"\.\.",\s*`, strings.Count(dir, "/")+1), `,\s*`)
		climb := regexp.MustCompile(`filepath\.Join\(` + dots + `\)`)
		if err == nil && climb.Match(source) && walkCall.Match(source) {
			offenders = append(offenders, relative)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, offender := range offenders {
		t.Errorf("%s walks the module, so it belongs in an audit group package (internal/testenv) whose inputs cover what it reads", offender)
	}
}
