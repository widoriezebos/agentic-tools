package testutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// InstalledWaitBinary places the candidate in a temporary installation whose
// adapter directory is the source tree's real adapter directory. Wait adapter
// discovery follows the executable installation even when --root names a
// separate state fixture.
func InstalledWaitBinary(t testing.TB, candidate string) string {
	t.Helper()
	sourceRoot := sourceMetasystemRoot(t)
	installation := t.TempDir()
	binDir := filepath.Join(installation, "bin")
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatalf("create temporary wait installation: %v", err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create temporary wait binary directory: %v", err)
	}
	if err := os.Symlink(filepath.Join(sourceRoot, "metasystem.conf"), filepath.Join(installation, "metasystem.conf")); err != nil {
		t.Fatalf("link source configuration into temporary wait installation: %v", err)
	}
	if err := os.Symlink(filepath.Join(sourceRoot, "scripts", "agents", "adapters"), filepath.Join(installation, "scripts", "agents", "adapters")); err != nil {
		t.Fatalf("link source adapters into temporary wait installation: %v", err)
	}

	source, err := os.Open(candidate)
	if err != nil {
		t.Fatalf("open wait candidate: %v", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		t.Fatalf("inspect wait candidate: %v", err)
	}
	installed := filepath.Join(binDir, "metasystem")
	if err := testexec.Locked(func() error {
		destination, err := os.OpenFile(installed, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return fmt.Errorf("create installed wait candidate: %w", err)
		}
		if _, err := io.Copy(destination, source); err != nil {
			_ = destination.Close()
			return fmt.Errorf("copy wait candidate into temporary installation: %w", err)
		}
		if err := destination.Close(); err != nil {
			return fmt.Errorf("close installed wait candidate: %w", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return installed
}

func sourceMetasystemRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("read test working directory: %v", err)
	}
	for {
		configuration := filepath.Join(dir, "metasystem.conf")
		adapter := filepath.Join(dir, "scripts", "agents", "adapters", "fake.sh")
		if _, configurationErr := os.Stat(configuration); configurationErr == nil {
			if _, adapterErr := os.Stat(adapter); adapterErr == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("find metasystem source root from %s", dir)
		}
		dir = parent
	}
}
