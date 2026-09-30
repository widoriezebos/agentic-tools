package main

import (
	"os"
	"path"
	"slices"
	"strings"
)

// Group 4 of the message rewrite: proofs, tests and gates. Each path below
// speaks in the two lines of "Messages a Person Reads". The files left out
// hold machine protocol a parent process reads by its leading code (the
// landing owner reads internal/testrun/protocol.go's and
// internal/proofrun/protocol.go's from a child's output); each says so at
// its top.
var _ = enforceMessages(append(append(
	packageFilesExcept("internal/testrun", "protocol.go"),
	packageFilesExcept("internal/proofrun", "protocol.go")...),
	"internal/audit",
	"internal/candidateengine",
	"internal/enginecause",
	"internal/gaterun",
	"internal/testenv",
	"internal/testpolicy",
	"internal/validate",
)...)

// packageFilesExcept is every production Go file of a package directory
// (module-relative) but the named ones, so a file added later is enforced
// too.
func packageFilesExcept(dir string, except ...string) []string {
	entries, err := os.ReadDir(path.Join("..", "..", dir))
	if err != nil {
		panic(err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || slices.Contains(except, name) {
			continue
		}
		files = append(files, dir+"/"+name)
	}
	return files
}
