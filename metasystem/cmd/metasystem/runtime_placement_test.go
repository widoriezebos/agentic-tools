package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// The placement convention, mechanically checked (backlog item 17,
// D78): inside the seam packages, a file named for one runtime must
// not reference another runtime in CODE — comments may cite critics
// and incidents, code may not drift. This is exactly the check that
// would have caught DevinPermissionMode landing in codex.go during
// the D25 consolidation.
func TestRuntimeFilePlacement(t *testing.T) {
	seamGlobs := []string{
		"../../internal/adapter/*.go",
		"../../internal/host/*.go",
		"../../internal/usage/*.go",
	}
	names := runtimes.Names()
	lineComment := regexp.MustCompile(`//.*$`)
	stringLiteral := regexp.MustCompile("`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\"")
	nameRes := map[string]*regexp.Regexp{}
	var violations []string
	for _, glob := range seamGlobs {
		paths, err := filepath.Glob(glob)
		if err != nil || len(paths) == 0 {
			t.Fatalf("seam glob %s matched nothing", glob)
		}
		for _, path := range paths {
			base := strings.TrimSuffix(filepath.Base(path), ".go")
			base = strings.TrimSuffix(base, "_test")
			var owner string
			for _, name := range names {
				if strings.Contains(base, name) {
					owner = name
					break
				}
			}
			if owner == "" {
				continue // a generic seam file may serve every runtime
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for lineNumber, line := range strings.Split(string(body), "\n") {
				code := lineComment.ReplaceAllString(line, "")
				code = stringLiteral.ReplaceAllString(code, `""`)
				for _, other := range names {
					if other == owner {
						continue
					}
					re, cached := nameRes[other]
					if !cached {
						re = regexp.MustCompile(`(?i)\b` + other + `\b`)
						nameRes[other] = re
					}
					if re.MatchString(code) {
						violations = append(violations,
							filepath.Base(path)+":"+strconv.Itoa(lineNumber+1)+": ["+owner+" file] "+strings.TrimSpace(line))
					}
				}
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("cross-runtime code in per-runtime seam files:\n%s", strings.Join(violations, "\n"))
	}
}
