package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ported from supervision-fixtures.sh operator-layout: in the nested operator
// layout the Git toplevel is the application repository, not the vendored
// installation, so no shipped runtime hook configuration may resolve the
// supervision hook from the Git toplevel. The hook files are the subject.
func TestSupervisionBedAShippedHooksNeverResolveFromTheGitToplevel(t *testing.T) {
	t.Parallel()
	shipped, err := filepath.Glob(filepath.Join("..", "..", "scripts", "enforcement", "*hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(shipped) == 0 {
		t.Fatal("no shipped hook configuration under scripts/enforcement")
	}
	const toplevel = "$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh"
	for _, path := range shipped {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), toplevel) {
			t.Errorf("%s still resolves metasystem scripts from the Git toplevel", filepath.Base(path))
		}
		if !strings.Contains(string(data), "scripts/agents/supervision-hook.sh") {
			t.Errorf("%s carries no supervision hook command, so the check above proves nothing", filepath.Base(path))
		}
	}
}
