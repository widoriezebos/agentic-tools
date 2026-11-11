package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

func TestUnitCheckManualDirectoryFollowsExecutionTree(t *testing.T) {
	t.Parallel()
	for _, folder := range []string{".", "module"} {
		t.Run(folder, func(t *testing.T) {
			t.Parallel()
			candidate, base := realpath.ResolveExisting(t.TempDir()), realpath.ResolveExisting(t.TempDir())
			for _, tree := range []string{candidate, base} {
				for _, relative := range []string{"metasystem", folder} {
					if err := os.MkdirAll(filepath.Join(tree, relative), 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			check := UnitCheck{Directory: filepath.Join(candidate, "metasystem"), ManualDirectory: filepath.Join(candidate, folder),
				Cheap: "pwd", Audits: "pwd", Minutes: 1, Environment: []string{}}
			now := func() time.Time { return time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC) }
			for _, tree := range []string{candidate, base} {
				directory := filepath.Join(tree, "metasystem")
				_, exits, err := check.Run(directory, t.TempDir(), CheckContext{Now: now})
				if err != nil || len(exits) != 2 || strings.TrimSpace(exits[0].Output) != filepath.Join(tree, folder) || strings.TrimSpace(exits[1].Output) != directory {
					t.Fatalf("execution in %s: %+v %v", directory, exits, err)
				}
			}
			if check.ManualDirectory != filepath.Join(candidate, folder) {
				t.Fatal("executing the base changed the frozen manual folder")
			}
		})
	}
}
