package main

import (
	"os"
	"strings"
	"testing"
)

// TestAuditNoGoSourceSpellsTheLowercaseStampSentinel holds the Go tree at zero
// lowercase spellings of the build-stamp record sentinel (disk-lifetimes rule
// A3). The sentinel exists in source only as enginebuild's uppercase constant,
// lowered at runtime, so the only lowercase sentinel a compiled engine carries
// is the record the linker placed there and ReadStamp cannot be misled by code.
func TestAuditNoGoSourceSpellsTheLowercaseStampSentinel(t *testing.T) {
	t.Parallel()
	repository, _ := verbRatchetRoots(t)
	sentinel := strings.ToLower("METASYSTEM-BUILD-STAMP")
	scanned := 0
	var sites []string
	walkRatchetFiles(t, repository, []string{".git", "node_modules", "artifacts"}, nil, func(path, rel string) {
		if !strings.HasSuffix(rel, ".go") {
			return
		}
		scanned++
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), sentinel) {
			sites = append(sites, rel)
		}
	})
	if scanned == 0 {
		t.Fatal("scanned no Go files; the walk no longer reaches the tree")
	}
	if len(sites) != 0 {
		t.Fatalf("Go sources spell the lowercase stamp sentinel (use enginebuild's uppercase constant): %v", sites)
	}
}
