package act

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	if os.Getenv("METASYSTEM_TEST_MACHINE") == "" {
		humanauthority.EnrollmentLocation = func(root string) (string, string, error) {
			return filepath.Join(root, "artifacts", "agents", "authority", "human-terminal.json"), root, nil
		}
	}
	os.Exit(testenv.Main(m))
}
