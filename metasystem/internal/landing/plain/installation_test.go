package plain

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestLaneProofCarriesNativeHistoryInstallation(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	install := filepath.Join(checkout, "metasystem")
	fixtureEngine(t, install)
	git := stubGit{commit: "main", tree: "main-tree", main: "main", installPrefix: "metasystem"}
	called := false
	result, err := Run(install, checkout, "proof", "", io.Discard, ProveSeams{Git: git.run, Now: func() time.Time { return bedNow }, Trunk: true, Command: func(command *exec.Cmd) error {
		called = true
		found := false
		for _, entry := range command.Env {
			if value, ok := strings.CutPrefix(entry, proofrun.NativeHistoryRootEnvironment+"="); ok {
				found = value == install
			}
		}
		if !found || command.Dir == install {
			t.Fatal("detached proof did not retain the installation history owner")
		}
		_, err := io.WriteString(command.Stdout, "LANDING-CHECKED\t0\n")
		return err
	}})
	if err != nil || result.Result != Green || !called {
		t.Fatalf("lane proof: result=%s called=%t err=%v", result.Result, called, err)
	}
}
