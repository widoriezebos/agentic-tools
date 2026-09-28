package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The evidence root's key is spelled in one file: its owner. Every other
// production Go file and shell script says "the evidence root" and reads it
// through ResolveEvidenceRoot. Comments count, deliberately.
func TestTheEvidenceRootKeyIsSpelledOnlyByItsOwner(t *testing.T) {
	t.Parallel()
	module := filepath.Join("..", "..")
	owner := filepath.Join("internal", "config", "evidenceroot.go")
	spelling := "evidence" + ".root"
	var offenders []string
	for _, top := range []string{"cmd", "internal", "scripts"} {
		err := filepath.WalkDir(filepath.Join(module, top), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) && path == filepath.Join(module, top) {
					return filepath.SkipDir
				}
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			name := entry.Name()
			goSource := strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
			if !goSource && !strings.HasSuffix(name, ".sh") {
				return nil
			}
			relative, err := filepath.Rel(module, path)
			if err != nil {
				return err
			}
			if relative == owner {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(content), spelling) {
				offenders = append(offenders, relative)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("files other than %s spell the evidence root's key; say \"the evidence root\" and use EvidenceRootKey:\n%s", owner, strings.Join(offenders, "\n"))
	}
}
