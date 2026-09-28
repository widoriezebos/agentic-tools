package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFakeHostDelegatesItsLifetimeToUtilHold(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate prologue source test")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	if override := os.Getenv("FIXTURE_SOURCE_ROOT"); override != "" {
		root = override
	}
	// The fake host's hold (formerly hosts/fake.sh's `exec "$ms" util hold`)
	// replaces the host process with the engine's leash-bound hold.
	fake := readFixtureSource(t, filepath.Join(root, "internal", "adapter", "supervisor", "fake_host.go"))
	if !strings.Contains(fake, `syscall.Exec(d.Engine, append([]string{d.Engine, "util", "hold"}, flags...)`) || !strings.Contains(fake, `"--tag", t.Tag`) {
		t.Fatal("fake host does not delegate its leash-bound lifetime to util hold")
	}
}

func readFixtureSource(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
