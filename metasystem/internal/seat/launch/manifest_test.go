package launch

// The manifest is a copy of the adapters' declarations, so the copy is
// checked against them: every shipped adapter answers `local-config-paths`,
// and their union, sorted and deduplicated, is exactly LocalConfigPaths.
// This is an adapter integration test: the adapters are the shell scripts
// that own the declaration, so it runs them; it runs no git.

import (
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestTheManifestIsTheAdaptersDeclaredContract(t *testing.T) {
	t.Parallel()
	adapters, err := filepath.Glob(filepath.Join("..", "..", "..", "scripts", "agents", "adapters", "*.sh"))
	if err != nil || len(adapters) == 0 {
		t.Fatalf("no shipped adapters found: %v", err)
	}
	seen := map[string]bool{}
	for _, adapter := range adapters {
		if filepath.Base(adapter) == "runtime-common.sh" {
			continue
		}
		output, err := exec.Command("bash", adapter, "local-config-paths").Output()
		if err != nil {
			t.Fatalf("%s local-config-paths: %v", filepath.Base(adapter), err)
		}
		for _, line := range strings.Split(string(output), "\n") {
			if line != "" {
				seen[line] = true
			}
		}
	}
	declared := make([]string, 0, len(seen))
	for path := range seen {
		declared = append(declared, path)
	}
	sort.Strings(declared)
	if strings.Join(declared, "\n") != strings.Join(LocalConfigPaths, "\n") {
		t.Fatalf("the adapters declare\n%s\nand this package carries\n%s",
			strings.Join(declared, "\n"), strings.Join(LocalConfigPaths, "\n"))
	}
	if Manifest() != strings.Join(LocalConfigPaths, "\n")+"\n" {
		t.Fatalf("manifest = %q", Manifest())
	}
}
