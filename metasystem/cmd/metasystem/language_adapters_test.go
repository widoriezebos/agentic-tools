package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestEngineLinksItsLanguageAdapters: the landing batch's join admission
// detects a candidate's language through the adapter registry, and the Go
// adapter registers itself only when a production file of the engine imports
// it. A test file importing it would hide its absence from every test, so the
// witness reads the engine's own non-test imports.
func TestEngineLinksItsLanguageAdapters(t *testing.T) {
	t.Parallel()
	const goAdapter = "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range parsed.Imports {
			if path, _ := strconv.Unquote(spec.Path.Value); path == goAdapter {
				return
			}
		}
	}
	t.Fatalf("no production file of the engine imports %s; the landing batch would find no Go adapter", goAdapter)
}
