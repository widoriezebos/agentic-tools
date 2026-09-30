package testrun

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var (
	rearmUpEngineOnce sync.Once
	rearmUpEnginePath string
	rearmUpEngineErr  error
)

// rearmUpEngine is this tree's engine, built once for the package.
func rearmUpEngine(t *testing.T) []byte {
	t.Helper()
	rearmUpEngineOnce.Do(func() {
		dir, err := os.MkdirTemp("", "metasystem-rearm-up.")
		if err != nil {
			rearmUpEngineErr = err
			return
		}
		rearmUpEnginePath = filepath.Join(dir, "metasystem")
		build := exec.Command("go", "build", "-trimpath", "-o", rearmUpEnginePath, filepath.Join("..", "..", "cmd", "metasystem"))
		if output, err := build.CombinedOutput(); err != nil {
			rearmUpEngineErr = err
			t.Logf("%s", output)
		}
	})
	if rearmUpEngineErr != nil {
		t.Fatalf("build the engine: %v", rearmUpEngineErr)
	}
	data, err := os.ReadFile(rearmUpEnginePath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// rearmUpInstallation is an installation whose bin/metasystem is engine.
func rearmUpInstallation(t *testing.T, engine []byte) string {
	t.Helper()
	installation := t.TempDir()
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(installation, "bin", "metasystem"), engine, 0o755); err != nil {
		t.Fatal(err)
	}
	return installation
}

// The landed re-arm reads the rebuilt engine's up through its --json
// envelope (structured-output U2, T4): driven against the real up, a
// refusal arrives as the envelope's summary; an up that answers in words
// only reads as unknown, never as armed.
func TestLandedRearmUpReadsTheRealUpsEnvelope(t *testing.T) {
	installation := rearmUpInstallation(t, rearmUpEngine(t))
	notARepository := t.TempDir()
	outcome, err := landedRearmUp(context.Background(), installation, notARepository)
	if err == nil || !strings.Contains(err.Error(), "--repo is not inside a git repository") || outcome.Outcome != "" {
		t.Fatalf("the real up's refusal read as %+v, %v", outcome, err)
	}

	words := t.TempDir()
	if err := os.WriteFile(filepath.Join(words, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(words, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(words, "bin", "metasystem"), []byte("#!/bin/sh\nprintf 'up outcome=armed authority=writer\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	outcome, err = landedRearmUp(context.Background(), words, words)
	if err == nil || outcome.Outcome == "armed" {
		t.Fatalf("an up that answered only in words read as %+v, %v", outcome, err)
	}
}
