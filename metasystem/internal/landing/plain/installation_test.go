package plain

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func fixtureInstallation(t *testing.T) string {
	t.Helper()
	install := t.TempDir()
	fixtureEngine(t, install)
	return install
}

func fixtureEngine(t *testing.T, install string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(install, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(install, "bin", "metasystem"), []byte("#!/bin/sh\nexit 93\n"), 0700); err != nil {
		t.Fatal(err)
	}
}
